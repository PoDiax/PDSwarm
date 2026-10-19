package docker

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
)

// ImageReleaseNote represents a release or changelog item from an upstream source (e.g. GitHub).
type ImageReleaseNote struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	PublishedAt string `json:"published_at"`
	HTMLURL     string `json:"html_url"`
	Body        string `json:"body"`
}

var (
	gitHubRepoRegex = regexp.MustCompile(`(?:https?://github\.com/|git@github\.com:)([^/\s]+)/([^/\s#?]+?)(?:\.git)?(?:/.*)?$`)
	ghcrRegex       = regexp.MustCompile(`^ghcr\.io/([^/]+)/([^/:]+)`)
)

// Well-known official Docker library images mapped to their upstream GitHub repositories
var knownOfficialUpstreams = map[string]string{
	"golang":                 "golang/go",
	"redis":                  "redis/redis",
	"postgres":               "postgres/postgres",
	"node":                   "nodejs/node",
	"nginx":                  "nginx/nginx",
	"caddy":                  "caddyserver/caddy",
	"traefik":                "traefik/traefik",
	"alpine":                 "alpinelinux/docker-alpine",
	"ubuntu":                 "canonical/ubuntu-docker-image",
	"python":                 "python/cpython",
	"mariadb":                "MariaDB/server",
	"mysql":                  "mysql/mysql-server",
	"memcached":              "memcached/memcached",
	"rabbitmq":               "rabbitmq/rabbitmq-server",
	"portainer/portainer-ce": "portainer/portainer",
	"grafana/grafana":        "grafana/grafana",
	"prometheus/prometheus":  "prometheus/prometheus",
	"timberio/vector":        "vectordotdev/vector",
}

// ExtractImageMetadataAndChangelog parses image labels, tags, registry endpoints, and optionally
// queries upstream changelogs/releases.
func ExtractImageMetadataAndChangelog(
	ctx context.Context,
	imageRef string,
	inspect types.ImageInspect,
) (localVersion, sourceURL, changelogURL, hubURL string, releases []ImageReleaseNote) {
	cleanRef := strings.TrimSpace(imageRef)

	// 1. Local Version
	if inspect.Config != nil && inspect.Config.Labels != nil {
		labels := inspect.Config.Labels
		for _, key := range []string{
			"org.opencontainers.image.version",
			"version",
			"build_version",
			"image.version",
		} {
			if v := strings.TrimSpace(labels[key]); v != "" {
				localVersion = v
				break
			}
		}

		for _, key := range []string{
			"org.opencontainers.image.source",
			"org.opencontainers.image.url",
		} {
			if v := strings.TrimSpace(labels[key]); v != "" {
				sourceURL = normalizeGitHubURL(v)
				break
			}
		}
	}

	// If no version label, infer from tag if not 'latest'
	tag := extractTagFromRef(cleanRef)
	if localVersion == "" && tag != "" && tag != "latest" {
		localVersion = tag
	}

	// 2. Hub URL
	hubURL = resolveHubURL(cleanRef)

	// 3. Fallback source URL from GHCR or known upstreams
	repoKey := extractRepoKey(cleanRef)
	if sourceURL == "" {
		if m := ghcrRegex.FindStringSubmatch(cleanRef); len(m) == 3 {
			sourceURL = "https://github.com/" + m[1] + "/" + m[2]
		} else if upstream, ok := knownOfficialUpstreams[repoKey]; ok {
			sourceURL = "https://github.com/" + upstream
		}
	}

	// 4. Changelog URL
	if sourceURL != "" && strings.Contains(sourceURL, "github.com") {
		changelogURL = strings.TrimSuffix(sourceURL, "/") + "/releases"
	} else if hubURL != "" {
		changelogURL = hubURL + "?tab=tags"
	}

	// 5. Query GitHub Releases if a GitHub repository was identified
	if sourceURL != "" {
		if owner, repo, ok := parseGitHubOwnerRepo(sourceURL); ok {
			releases = fetchGitHubReleases(ctx, owner, repo)
		}
	}

	return localVersion, sourceURL, changelogURL, hubURL, releases
}

func extractTagFromRef(ref string) string {
	if idx := strings.LastIndex(ref, ":"); idx != -1 {
		t := ref[idx+1:]
		if !strings.Contains(t, "/") {
			return t
		}
	}
	return ""
}

func extractRepoKey(ref string) string {
	clean := ref
	if idx := strings.LastIndex(clean, ":"); idx != -1 {
		clean = clean[:idx]
	}
	// Strip domain if present
	parts := strings.Split(clean, "/")
	if len(parts) > 1 && (strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":")) {
		clean = strings.Join(parts[1:], "/")
	}
	return clean
}

func resolveHubURL(ref string) string {
	clean := ref
	if idx := strings.LastIndex(clean, ":"); idx != -1 {
		clean = clean[:idx]
	}

	if strings.HasPrefix(clean, "ghcr.io/") {
		parts := strings.TrimPrefix(clean, "ghcr.io/")
		return "https://github.com/" + parts + "/pkgs/container/" + strings.TrimPrefix(parts, strings.Split(parts, "/")[0]+"/")
	}
	if strings.HasPrefix(clean, "quay.io/") {
		parts := strings.TrimPrefix(clean, "quay.io/")
		return "https://quay.io/repository/" + parts
	}

	// Standard Docker Hub
	parts := strings.Split(clean, "/")
	if len(parts) == 1 || (len(parts) == 2 && parts[0] == "library") {
		name := parts[len(parts)-1]
		return "https://hub.docker.com/_/" + name
	}
	if len(parts) == 2 && !strings.Contains(parts[0], ".") {
		return "https://hub.docker.com/r/" + clean
	}

	return ""
}

func normalizeGitHubURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "git@github.com:") {
		path := strings.TrimPrefix(raw, "git@github.com:")
		path = strings.TrimSuffix(path, ".git")
		return "https://github.com/" + path
	}
	if strings.HasPrefix(raw, "http://github.com/") {
		return "https://" + strings.TrimPrefix(raw, "http://")
	}
	return strings.TrimSuffix(raw, ".git")
}

func parseGitHubOwnerRepo(ghURL string) (owner, repo string, ok bool) {
	matches := gitHubRepoRegex.FindStringSubmatch(ghURL)
	if len(matches) == 3 {
		return matches[1], strings.TrimSuffix(matches[2], ".git"), true
	}
	return "", "", false
}

func fetchGitHubReleases(parentCtx context.Context, owner, repo string) []ImageReleaseNote {
	ctx, cancel := context.WithTimeout(parentCtx, 3*time.Second)
	defer cancel()

	url := "https://api.github.com/repos/" + owner + "/" + repo + "/releases?per_page=3"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "PDSwarm-Docker-Manager")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return nil
	}
	defer resp.Body.Close()

	type ghRelease struct {
		TagName     string `json:"tag_name"`
		Name        string `json:"name"`
		PublishedAt string `json:"published_at"`
		HTMLURL     string `json:"html_url"`
		Body        string `json:"body"`
		Draft       bool   `json:"draft"`
		Prerelease  bool   `json:"prerelease"`
	}

	var rawReleases []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rawReleases); err != nil {
		return nil
	}

	var results []ImageReleaseNote
	for _, r := range rawReleases {
		if r.Draft {
			continue
		}
		body := r.Body
		if len(body) > 2000 {
			body = body[:2000] + "\n\n*(Changelog truncated for preview...)*"
		}
		results = append(results, ImageReleaseNote{
			TagName:     r.TagName,
			Name:        r.Name,
			PublishedAt: r.PublishedAt,
			HTMLURL:     r.HTMLURL,
			Body:        body,
		})
	}
	return results
}
