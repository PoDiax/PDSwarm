package templates

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
	"gopkg.in/yaml.v3"
)

var (
	// ErrTemplateNotFound is returned when a requested template does not exist.
	ErrTemplateNotFound = errors.New("template not found")

	// ErrStackConflict is returned when a stack with the same name already exists.
	ErrStackConflict = errors.New("stack already exists")

	// ErrInvalidInput is returned when input parameters are invalid.
	ErrInvalidInput = errors.New("invalid input")
)

var envVarPattern = regexp.MustCompile(`\$\{([a-zA-Z_][a-zA-Z0-9_]*)(?::?[-?]([^}]*))?\}`)

// CatalogManager manages curated templates and optional external Portainer / CasaOS template catalogs.
// It is fully thread-safe using an internal RWMutex.
type CatalogManager struct {
	mu        sync.RWMutex
	templates map[string]Template
}

// NewCatalogManager creates a new CatalogManager initialized with the built-in curated templates.
func NewCatalogManager() *CatalogManager {
	cm := &CatalogManager{
		templates: make(map[string]Template),
	}
	for _, t := range GetCuratedTemplates() {
		_ = cm.AddTemplate(t)
	}
	return cm
}

// AddTemplate registers or overwrites a template in the catalog.
func (cm *CatalogManager) AddTemplate(t Template) error {
	if strings.TrimSpace(t.ID) == "" {
		return fmt.Errorf("%w: template id cannot be empty", ErrInvalidInput)
	}
	if strings.TrimSpace(t.Title) == "" {
		return fmt.Errorf("%w: template title cannot be empty", ErrInvalidInput)
	}
	if t.Source == "" {
		t.Source = "curated"
	}
	if t.Platform == "" {
		t.Platform = "linux"
	}
	if t.Type == "" {
		t.Type = "compose"
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.templates[t.ID] = t
	return nil
}

// RegisterTemplate is an alias for AddTemplate.
func (cm *CatalogManager) RegisterTemplate(t Template) error {
	return cm.AddTemplate(t)
}

// GetTemplate retrieves a template by ID, dynamically resolving remote stackfiles and missing ports/volumes if needed.
func (cm *CatalogManager) GetTemplate(id string) (*Template, error) {
	cm.mu.RLock()
	tmpl, ok := cm.templates[id]
	cm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrTemplateNotFound, id)
	}

	needsUpdate := false

	// If template references a git repository and has no concrete services or image in its Compose YAML, resolve it
	if tmpl.Repository != nil && tmpl.Repository.StackFile != "" && (!strings.Contains(tmpl.ComposeYAML, "image:") && !strings.Contains(tmpl.ComposeYAML, "build:")) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		resolved, err := resolveGitStackFile(ctx, tmpl.Repository.URL, tmpl.Repository.StackFile)
		cancel()
		if err == nil && strings.TrimSpace(resolved) != "" {
			tmpl.ComposeYAML = resolved
			needsUpdate = true

			// Extract real ports from the fetched stackfile
			if len(tmpl.Ports) == 0 {
				extractedPorts := extractPortsFromCompose(resolved)
				if len(extractedPorts) > 0 {
					tmpl.Ports = extractedPorts
				}
			}
			// Extract real volumes from the fetched stackfile
			if len(tmpl.Volumes) == 0 {
				extractedVols := extractVolumesFromCompose(resolved)
				if len(extractedVols) > 0 {
					tmpl.Volumes = extractedVols
				}
			}
			// Extract real environment variables from the fetched stackfile
			if len(tmpl.Env) == 0 {
				extractedEnv := extractEnvFromCompose(resolved)
				if len(extractedEnv) > 0 {
					tmpl.Env = extractedEnv
				}
			}
		}
	}

	// Extract ports if empty
	if len(tmpl.Ports) == 0 && tmpl.ComposeYAML != "" {
		extractedPorts := extractPortsFromCompose(tmpl.ComposeYAML)
		if len(extractedPorts) > 0 {
			tmpl.Ports = extractedPorts
			needsUpdate = true
		}
	}

	// Extract volumes if empty
	if len(tmpl.Volumes) == 0 && tmpl.ComposeYAML != "" {
		extractedVolumes := extractVolumesFromCompose(tmpl.ComposeYAML)
		if len(extractedVolumes) > 0 {
			tmpl.Volumes = extractedVolumes
			needsUpdate = true
		}
	}

	// Sanitize any existing env vars to prune erroneously extracted compose directives or paths
	if len(tmpl.Env) > 0 {
		var validEnvs []TemplateEnvVar
		cleaned := false
		for _, e := range tmpl.Env {
			if isValidEnvVarName(e.Name) && !isReservedComposeKey(e.Name) {
				validEnvs = append(validEnvs, e)
			} else {
				cleaned = true
			}
		}
		if cleaned {
			tmpl.Env = validEnvs
			needsUpdate = true
		}
	}

	// Extract environment variables if empty
	if len(tmpl.Env) == 0 && tmpl.ComposeYAML != "" {
		extractedEnv := extractEnvFromCompose(tmpl.ComposeYAML)
		if len(extractedEnv) > 0 {
			tmpl.Env = extractedEnv
			needsUpdate = true
		}
	}

	if needsUpdate {
		cm.mu.Lock()
		cm.templates[id] = tmpl
		cm.mu.Unlock()
	}

	tCopy := tmpl

	// Provide a working baseline fallback ONLY on the returned copy for display,
	// without permanently overwriting cm.templates[id] so git repos can still be retried.
	if strings.TrimSpace(tCopy.ComposeYAML) == "" || (!strings.Contains(tCopy.ComposeYAML, "services:") && !strings.Contains(tCopy.ComposeYAML, "image:")) {
		img := strings.ToLower(strings.ReplaceAll(tCopy.Title, " ", ""))
		if !strings.Contains(img, "/") && !strings.Contains(img, ":") {
			img = img + ":latest"
		}
		tCopy.ComposeYAML = fmt.Sprintf("version: '3.8'\nservices:\n  app:\n    image: %s\n    restart: unless-stopped\n", img)
	}

	return &tCopy, nil
}

// ListTemplates filters templates by search query, category, and source.
func (cm *CatalogManager) ListTemplates(query, category, source string) []Template {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	q := strings.ToLower(strings.TrimSpace(query))
	cat := strings.ToLower(strings.TrimSpace(category))
	src := strings.ToLower(strings.TrimSpace(source))

	results := make([]Template, 0, len(cm.templates))
	for _, t := range cm.templates {
		// Filter by query (ID, Title, Description)
		if q != "" {
			matchTitle := strings.Contains(strings.ToLower(t.Title), q)
			matchDesc := strings.Contains(strings.ToLower(t.Description), q)
			matchID := strings.Contains(strings.ToLower(t.ID), q)
			if !matchTitle && !matchDesc && !matchID {
				continue
			}
		}

		// Filter by category
		if cat != "" {
			catMatch := false
			for _, c := range t.Categories {
				if strings.EqualFold(c, cat) {
					catMatch = true
					break
				}
			}
			if !catMatch {
				continue
			}
		}

		// Filter by source
		if src != "" {
			if !strings.EqualFold(t.Source, src) {
				continue
			}
		}

		results = append(results, t)
	}

	// Sort deterministically by Title
	sort.Slice(results, func(i, j int) bool {
		return results[i].Title < results[j].Title
	})

	return results
}

// Categories returns a sorted slice of all distinct categories available across templates.
func (cm *CatalogManager) Categories() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	catMap := make(map[string]struct{})
	for _, t := range cm.templates {
		for _, c := range t.Categories {
			trimmed := strings.TrimSpace(c)
			if trimmed != "" {
				catMap[trimmed] = struct{}{}
			}
		}
	}

	categories := make([]string, 0, len(catMap))
	for c := range catMap {
		categories = append(categories, c)
	}
	sort.Strings(categories)
	return categories
}

// Sources returns a sorted slice of all distinct sources (e.g. curated, portainer, casaos).
func (cm *CatalogManager) Sources() []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	srcMap := make(map[string]struct{})
	for _, t := range cm.templates {
		if t.Source != "" {
			srcMap[t.Source] = struct{}{}
		}
	}

	sources := make([]string, 0, len(srcMap))
	for s := range srcMap {
		sources = append(sources, s)
	}
	sort.Strings(sources)
	return sources
}

// CategoryCounts returns a map of category names to the count of templates in each category.
func (cm *CatalogManager) CategoryCounts() map[string]int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	counts := make(map[string]int)
	for _, t := range cm.templates {
		for _, c := range t.Categories {
			trimmed := strings.TrimSpace(c)
			if trimmed != "" {
				counts[trimmed]++
			}
		}
	}
	return counts
}

// SubstituteEnv replaces `${VAR_NAME}` and `${VAR_NAME:-default}` occurrences in composeYAML
// using values from userEnv or default values from templateEnv definitions.
// Returns an error if any required variable is empty/missing.
func SubstituteEnv(composeYAML string, userEnv map[string]string, templateEnv []TemplateEnvVar) (string, error) {
	// Build map of defaults and required flags from templateEnv
	defaults := make(map[string]string)
	required := make(map[string]bool)
	for _, v := range templateEnv {
		defaults[v.Name] = v.Default
		if v.Required {
			required[v.Name] = true
		}
	}

	var missingRequired []string

	replaced := envVarPattern.ReplaceAllStringFunc(composeYAML, func(match string) string {
		submatches := envVarPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		varName := submatches[1]
		inlineDefault := ""
		if len(submatches) >= 3 {
			inlineDefault = submatches[2]
		}

		// 1. Check user provided value
		if userEnv != nil {
			if val, ok := userEnv[varName]; ok && val != "" {
				return val
			}
		}

		// 2. Inline default in compose YAML (${VAR:-default})
		if inlineDefault != "" {
			return inlineDefault
		}

		// 3. Template defined default
		if defVal, ok := defaults[varName]; ok && defVal != "" {
			return defVal
		}

		// 4. If userEnv explicitly contains empty string, use empty
		if userEnv != nil {
			if val, ok := userEnv[varName]; ok {
				return val
			}
		}

		// 5. If marked required and still unresolved, record missing
		if required[varName] {
			missingRequired = append(missingRequired, varName)
		}

		return ""
	})

	if len(missingRequired) > 0 {
		return "", fmt.Errorf("%w: missing required environment variable(s): %s", ErrInvalidInput, strings.Join(missingRequired, ", "))
	}

	return replaced, nil
}

// Deploy prepares and deploys a Docker Compose stack from a template.
func (cm *CatalogManager) Deploy(ctx context.Context, ds *docker.Service, st store.Store, req DeployTemplateRequest) (*store.Stack, error) {
	return cm.DeployWithProgress(ctx, ds, st, req, nil)
}

// DeployWithProgress prepares and deploys a Docker Compose stack with streaming progress events.
func (cm *CatalogManager) DeployWithProgress(ctx context.Context, ds *docker.Service, st store.Store, req DeployTemplateRequest, onProgress docker.DeployProgressFunc) (*store.Stack, error) {
	req.TemplateID = strings.TrimSpace(req.TemplateID)
	req.StackName = strings.TrimSpace(req.StackName)

	if req.TemplateID == "" {
		return nil, fmt.Errorf("%w: template_id is required", ErrInvalidInput)
	}
	if req.StackName == "" {
		return nil, fmt.Errorf("%w: stack_name is required", ErrInvalidInput)
	}
	if st == nil {
		return nil, fmt.Errorf("%w: storage store is required", ErrInvalidInput)
	}

	// 1. Fetch template
	tmpl, err := cm.GetTemplate(req.TemplateID)
	if err != nil {
		return nil, err
	}

	// 2. Check if stack already exists in store
	existing, err := st.GetStack(req.StackName)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("%w: stack %q already exists", ErrStackConflict, req.StackName)
	}

	// 3. Substitute environment variables
	composeSource := tmpl.ComposeYAML
	if strings.TrimSpace(req.ComposeYAML) != "" {
		composeSource = req.ComposeYAML
	}
	finalYAML, err := SubstituteEnv(composeSource, req.Env, tmpl.Env)
	if err != nil {
		return nil, err
	}

	// 4. Validate compose YAML syntax
	if _, err := docker.ParseComposeYAML(finalYAML); err != nil {
		return nil, fmt.Errorf("%w: invalid substituted compose yaml: %v", ErrInvalidInput, err)
	}

	stack := &store.Stack{
		Name:        req.StackName,
		ComposeYAML: finalYAML,
		Status:      "created",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 5. Deploy to Docker engine if service is available
	if ds != nil {
		stack.Status = "deploying"
		if err := ds.DeployComposeStackWithProgress(ctx, req.StackName, finalYAML, onProgress); err != nil {
			return nil, fmt.Errorf("failed to deploy compose stack: %w", err)
		}
		stack.Status = "running"
	}

	// 6. Save stack in database
	if err := st.SaveStack(stack); err != nil {
		return nil, fmt.Errorf("failed to save stack: %w", err)
	}

	return stack, nil
}

// -----------------------------------------------------------------------------
// EXTERNAL CATALOG INGESTION (Portainer & CasaOS)
// -----------------------------------------------------------------------------

// ImportPortainerCatalog parses Portainer templates JSON format and adds them to the catalog.
func (cm *CatalogManager) ImportPortainerCatalog(data []byte) ([]Template, error) {
	var catalog PortainerTemplatesCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		// Try fallback array format
		var list []PortainerTemplate
		if errArray := json.Unmarshal(data, &list); errArray != nil {
			return nil, fmt.Errorf("failed to parse portainer templates: %w", err)
		}
		catalog.Templates = list
	}

	var imported []Template
	for i, pt := range catalog.Templates {
		slug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(pt.Title), " ", "-"))
		if slug == "" {
			slug = fmt.Sprintf("app-%d", i+1)
		}
		baseID := "portainer-" + slug
		id := baseID
		cm.mu.RLock()
		if _, exists := cm.templates[id]; exists {
			id = fmt.Sprintf("%s-%d", baseID, i+1)
		}
		cm.mu.RUnlock()

		// Convert env vars
		var envVars []TemplateEnvVar
		for _, e := range pt.Env {
			envVars = append(envVars, TemplateEnvVar{
				Name:        e.Name,
				Label:       e.Label,
				Default:     e.Default,
				Description: e.Description,
				Required:    false,
			})
		}

		// Convert volumes
		var volumes []string
		for _, v := range pt.Volumes {
			volumes = append(volumes, fmt.Sprintf("%s:%s", v.Bind, v.Container))
		}

		// Generate Compose YAML if not given
		composeYAML := ""
		if pt.Repository != nil && pt.Repository.StackFile != "" {
			composeYAML = fmt.Sprintf("# Imported from %s\n# Stackfile: %s\n", pt.Repository.URL, pt.Repository.StackFile)
		} else if pt.Image != "" {
			portsBlock := ""
			if len(pt.Ports) > 0 {
				portsBlock = "    ports:\n"
				for _, p := range pt.Ports {
					portsBlock += fmt.Sprintf("      - %q\n", p)
				}
			}
			volumesBlock := ""
			if len(pt.Volumes) > 0 {
				volumesBlock = "    volumes:\n"
				for _, v := range pt.Volumes {
					volumesBlock += fmt.Sprintf("      - %q\n", fmt.Sprintf("%s:%s", v.Bind, v.Container))
				}
			}
			envBlock := ""
			if len(envVars) > 0 {
				envBlock = "    environment:\n"
				for _, ev := range envVars {
					envBlock += fmt.Sprintf("      - %s=%s\n", ev.Name, ev.Default)
				}
			}
			restartPolicy := pt.RestartPolicy
			if restartPolicy == "" {
				restartPolicy = "unless-stopped"
			}
			composeYAML = fmt.Sprintf(`version: '3.8'
services:
  app:
    image: %s
    restart: %s
%s%s%s`, pt.Image, restartPolicy, portsBlock, volumesBlock, envBlock)
		}

		t := Template{
			ID:          id,
			Title:       pt.Title,
			Description: pt.Description,
			Categories:  pt.Categories,
			Logo:        pt.Logo,
			Platform:    pt.Platform,
			Type:        "compose",
			ComposeYAML: composeYAML,
			Env:         envVars,
			Volumes:     volumes,
			Ports:       pt.Ports,
			Source:      "portainer",
			Repository:  pt.Repository,
		}
		if t.Platform == "" {
			t.Platform = "linux"
		}
		_ = cm.AddTemplate(t)
		imported = append(imported, t)
	}

	return imported, nil
}

// ImportCasaOSTemplate parses a CasaOS compose file with x-casaos metadata and adds it to the catalog.
func (cm *CatalogManager) ImportCasaOSTemplate(yamlData []byte) (*Template, error) {
	var root struct {
		Name     string                 `yaml:"name"`
		XCasaOS  CasaOSMetadata         `yaml:"x-casaos"`
		Services map[string]interface{} `yaml:"services"`
	}

	if err := yaml.Unmarshal(yamlData, &root); err != nil {
		return nil, fmt.Errorf("failed to parse casaos compose yaml: %w", err)
	}

	// If x-casaos is nested inside a service, extract it
	if len(root.XCasaOS.Title) == 0 && len(root.Services) > 0 {
		for _, svc := range root.Services {
			svcMap, ok := svc.(map[string]interface{})
			if !ok {
				continue
			}
			if xVal, exists := svcMap["x-casaos"]; exists {
				var svcCasaOS CasaOSMetadata
				if rawBytes, err := yaml.Marshal(xVal); err == nil {
					if err := yaml.Unmarshal(rawBytes, &svcCasaOS); err == nil {
						if len(root.XCasaOS.Title) == 0 && len(svcCasaOS.Title) > 0 {
							root.XCasaOS.Title = svcCasaOS.Title
						}
						if root.XCasaOS.Icon == "" && svcCasaOS.Icon != "" {
							root.XCasaOS.Icon = svcCasaOS.Icon
						}
						if len(root.XCasaOS.Description) == 0 && len(svcCasaOS.Description) > 0 {
							root.XCasaOS.Description = svcCasaOS.Description
						}
						if len(root.XCasaOS.Tagline) == 0 && len(svcCasaOS.Tagline) > 0 {
							root.XCasaOS.Tagline = svcCasaOS.Tagline
						}
						if root.XCasaOS.Category == "" && svcCasaOS.Category != "" {
							root.XCasaOS.Category = svcCasaOS.Category
						}
						if root.XCasaOS.PortMap == "" && svcCasaOS.PortMap != "" {
							root.XCasaOS.PortMap = svcCasaOS.PortMap
						}
						if len(root.XCasaOS.Envs) == 0 && len(svcCasaOS.Envs) > 0 {
							root.XCasaOS.Envs = svcCasaOS.Envs
						}
					}
				}
			}
		}
	}

	title := root.XCasaOS.Title["en_us"]
	if title == "" {
		for _, v := range root.XCasaOS.Title {
			title = v
			break
		}
	}
	if title == "" && root.Name != "" {
		title = strings.Title(strings.ReplaceAll(root.Name, "-", " "))
	}
	if title == "" {
		title = root.XCasaOS.Main
	}
	if title == "" {
		title = "CasaOS App"
	}

	desc := root.XCasaOS.Description["en_us"]
	if desc == "" {
		desc = root.XCasaOS.Tagline["en_us"]
	}
	if desc == "" {
		for _, v := range root.XCasaOS.Description {
			desc = v
			break
		}
	}

	id := "casaos-" + strings.ToLower(strings.ReplaceAll(title, " ", "-"))
	categories := []string{}
	if root.XCasaOS.Category != "" {
		categories = append(categories, root.XCasaOS.Category)
	}

	var ports []string
	if root.XCasaOS.PortMap != "" {
		ports = append(ports, root.XCasaOS.PortMap)
	}

	// Convert CasaOS env declarations
	var envVars []TemplateEnvVar
	for _, env := range root.XCasaOS.Envs {
		if env.Container == "" {
			continue
		}
		descStr := env.Description["en_us"]
		if descStr == "" {
			for _, d := range env.Description {
				descStr = d
				break
			}
		}
		envVars = append(envVars, TemplateEnvVar{
			Name:        env.Container,
			Label:       env.Container,
			Description: descStr,
			Required:    false,
		})
	}

	tmpl := Template{
		ID:          id,
		Title:       title,
		Description: desc,
		Categories:  categories,
		Logo:        root.XCasaOS.Icon,
		Platform:    "linux",
		Type:        "compose",
		ComposeYAML: string(yamlData),
		Env:         envVars,
		Ports:       ports,
		Source:      "casaos",
	}

	if err := cm.AddTemplate(tmpl); err != nil {
		return nil, err
	}
	return &tmpl, nil
}

// ImportCasaOSZip parses a zip archive containing CasaOS app directories with docker-compose.yml files.
func (cm *CatalogManager) ImportCasaOSZip(zipData []byte, sourceName string) ([]Template, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("failed to open zip reader: %w", err)
	}

	if sourceName == "" {
		sourceName = "casaos"
	}

	var imported []Template
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		nameLower := strings.ToLower(f.Name)
		if strings.HasSuffix(nameLower, "docker-compose.yml") || strings.HasSuffix(nameLower, "docker-compose.yaml") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			content, err := io.ReadAll(io.LimitReader(rc, 10<<20))
			_ = rc.Close()
			if err != nil {
				continue
			}

			tmpl, err := cm.ImportCasaOSTemplate(content)
			if err != nil {
				continue
			}
			tmpl.Source = sourceName
			baseID := "casaos-" + strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tmpl.Title), " ", "-"))
			if baseID == "casaos-" {
				baseID = fmt.Sprintf("casaos-app-%d", len(imported)+1)
			}
			id := baseID
			cm.mu.RLock()
			if _, exists := cm.templates[id]; exists {
				id = fmt.Sprintf("%s-%d", baseID, len(imported)+1)
			}
			cm.mu.RUnlock()
			tmpl.ID = id
			_ = cm.AddTemplate(*tmpl)
			imported = append(imported, *tmpl)
		}
	}

	if len(imported) == 0 {
		return nil, errors.New("no valid docker-compose.yml app files found in zip archive")
	}
	return imported, nil
}

// SyncRepo fetches an external template repository (Portainer JSON, CasaOS Zip, CasaOS compose),
// parses all templates, tags them with repo.Name, and registers them.
func (cm *CatalogManager) SyncRepo(ctx context.Context, client *http.Client, repo *store.TemplateRepo) ([]Template, error) {
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, repo.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", repo.URL, err)
	}
	req.Header.Set("User-Agent", "PDSwarm/1.0 (+https://github.com/podiax/pdswarm)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", repo.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d error fetching %s", resp.StatusCode, repo.URL)
	}

	// Read body up to 100MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 100<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	sourceName := repo.Name
	if sourceName == "" {
		sourceName = "custom"
	}

	// Check if zip archive (by type, url suffix, content-type, or magic bytes)
	isZip := repo.Type == "casaos_zip" ||
		strings.HasSuffix(strings.ToLower(repo.URL), ".zip") ||
		strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "zip") ||
		(len(body) >= 4 && string(body[:4]) == "PK\x03\x04")

	if isZip {
		imported, err := cm.ImportCasaOSZip(body, sourceName)
		if err != nil {
			return nil, err
		}
		return imported, nil
	}

	// Try Portainer JSON format
	imported, err := cm.ImportPortainerCatalog(body)
	if err == nil && len(imported) > 0 {
		for i := range imported {
			imported[i].Source = sourceName
			_ = cm.AddTemplate(imported[i])
		}
		return imported, nil
	}

	// Fallback: try CasaOS single compose
	tmpl, errCasa := cm.ImportCasaOSTemplate(body)
	if errCasa == nil && tmpl != nil {
		tmpl.Source = sourceName
		_ = cm.AddTemplate(*tmpl)
		return []Template{*tmpl}, nil
	}

	return nil, fmt.Errorf("unrecognized template catalog format from %s", repo.URL)
}

// Rebuild reconstructs the entire in-memory catalog from scratch, honoring the
// "disable_default_templates" setting and syncing/loading all enabled external repositories.
func (cm *CatalogManager) Rebuild(ctx context.Context, client *http.Client, st store.Store) error {
	var disableDefaults bool
	if st != nil {
		val, _ := st.GetSetting("disable_default_templates")
		disableDefaults = (val == "true")
	}

	newTemplates := make(map[string]Template)

	// 1. Add curated templates if enabled
	if !disableDefaults {
		for _, t := range GetCuratedTemplates() {
			newTemplates[t.ID] = t
		}
	}

	// 2. Fetch and import custom repos if store provided
	if st != nil {
		repos, err := st.ListTemplateRepos()
		if err == nil {
			for _, repo := range repos {
				if !repo.Enabled {
					continue
				}
				templates, err := cm.SyncRepo(ctx, client, repo)
				now := time.Now()
				if err != nil {
					repo.SyncError = err.Error()
					_ = st.SaveTemplateRepo(repo)
					continue
				}
				repo.SyncError = ""
				repo.ItemCount = len(templates)
				repo.LastSyncAt = &now
				_ = st.SaveTemplateRepo(repo)

				for _, t := range templates {
					newTemplates[t.ID] = t
				}
			}
		}
	}

	// Swap under lock
	cm.mu.Lock()
	cm.templates = newTemplates
	cm.mu.Unlock()

	return nil
}

// SyncPortainerCatalog fetches a remote Portainer templates JSON URL and imports it.
func (cm *CatalogManager) SyncPortainerCatalog(ctx context.Context, client *http.Client, catalogURL string) error {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, catalogURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch portainer catalog from %s: %w", catalogURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to fetch portainer catalog: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	_, err = cm.ImportPortainerCatalog(body)
	return err
}

// SyncCasaOSCatalog fetches a remote CasaOS compose YAML URL and imports it.
func (cm *CatalogManager) SyncCasaOSCatalog(ctx context.Context, client *http.Client, fileURL string) error {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch casaos compose from %s: %w", fileURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to fetch casaos template: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	_, err = cm.ImportCasaOSTemplate(body)
	return err
}

// resolveGitStackFile attempts to fetch a raw docker-compose file from a git repository URL (e.g. GitHub).
func resolveGitStackFile(ctx context.Context, repoURL, stackFile string) (string, error) {
	if repoURL == "" || stackFile == "" {
		return "", errors.New("empty repository url or stackfile")
	}

	url := strings.TrimRight(strings.TrimSpace(repoURL), "/")
	url = strings.TrimSuffix(url, ".git")
	sf := strings.TrimLeft(strings.TrimSpace(stackFile), "/")

	// Also prepare alternate extension if .yml or .yaml
	var altSF string
	if strings.HasSuffix(strings.ToLower(sf), ".yml") {
		altSF = sf[:len(sf)-4] + ".yaml"
	} else if strings.HasSuffix(strings.ToLower(sf), ".yaml") {
		altSF = sf[:len(sf)-5] + ".yml"
	}

	var candidateURLs []string
	if strings.Contains(url, "github.com/") {
		parts := strings.Split(url, "github.com/")
		if len(parts) == 2 {
			ownerRepo := strings.Trim(parts[1], "/")
			candidateURLs = append(candidateURLs,
				fmt.Sprintf("https://raw.githubusercontent.com/%s/master/%s", ownerRepo, sf),
				fmt.Sprintf("https://raw.githubusercontent.com/%s/main/%s", ownerRepo, sf),
				fmt.Sprintf("https://raw.githubusercontent.com/%s/HEAD/%s", ownerRepo, sf),
			)
			if altSF != "" {
				candidateURLs = append(candidateURLs,
					fmt.Sprintf("https://raw.githubusercontent.com/%s/master/%s", ownerRepo, altSF),
					fmt.Sprintf("https://raw.githubusercontent.com/%s/main/%s", ownerRepo, altSF),
				)
			}
		}
	} else if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		candidateURLs = append(candidateURLs, fmt.Sprintf("%s/%s", url, sf))
		if altSF != "" {
			candidateURLs = append(candidateURLs, fmt.Sprintf("%s/%s", url, altSF))
		}
	}

	client := &http.Client{Timeout: 15 * time.Second}
	for _, cURL := range candidateURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, cURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "PDSwarm/1.0 (+https://github.com/podiax/pdswarm)")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		if resp.StatusCode == http.StatusOK {
			b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
			resp.Body.Close()
			if err == nil && len(b) > 0 {
				return string(b), nil
			}
		} else {
			resp.Body.Close()
		}
	}

	return "", fmt.Errorf("failed to retrieve stackfile %s from %s", stackFile, repoURL)
}

// extractPortsFromCompose extracts port strings from a Compose YAML string.
func extractPortsFromCompose(yamlContent string) []string {
	var ports []string
	lines := strings.Split(yamlContent, "\n")
	inPorts := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ports:") {
			inPorts = true
			continue
		}
		if inPorts {
			if strings.HasPrefix(trimmed, "-") {
				p := strings.Trim(strings.TrimPrefix(trimmed, "-"), " \"'")
				if p != "" {
					ports = append(ports, p)
				}
			} else if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				inPorts = false
			}
		}
	}
	return ports
}

// extractVolumesFromCompose extracts volume strings from a Compose YAML string.
func extractVolumesFromCompose(yamlContent string) []string {
	var volumes []string
	lines := strings.Split(yamlContent, "\n")
	inVols := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "volumes:") {
			inVols = true
			continue
		}
		if inVols {
			if strings.HasPrefix(trimmed, "-") {
				v := strings.Trim(strings.TrimPrefix(trimmed, "-"), " \"'")
				if v != "" {
					volumes = append(volumes, v)
				}
			} else if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
				inVols = false
			}
		}
	}
	return volumes
}

var validEnvNameRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// isValidEnvVarName checks whether an environment variable name is a valid identifier.
func isValidEnvVarName(name string) bool {
	return validEnvNameRegex.MatchString(name)
}

// isReservedComposeKey returns true if name is a standard Docker Compose directive.
func isReservedComposeKey(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "version", "services", "volumes", "networks", "ports", "labels",
		"deploy", "restart", "image", "build", "container_name", "entrypoint",
		"command", "depends_on", "healthcheck", "logging", "devices",
		"cap_add", "cap_drop", "sysctls", "ulimits", "dns", "expose",
		"extra_hosts", "configs", "secrets", "environment", "user",
		"working_dir", "stdin_open", "tty", "privileged", "hostname":
		return true
	default:
		return false
	}
}

// formatEnvLabel converts a SCREAMING_SNAKE_CASE variable into a readable Label.
func formatEnvLabel(varName string) string {
	parts := strings.Split(varName, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
		}
	}
	return strings.Join(parts, " ")
}

// extractEnvFromCompose extracts environment variables declared under services.*.environment in Compose YAML.
func extractEnvFromCompose(yamlContent string) []TemplateEnvVar {
	var envs []TemplateEnvVar
	seen := make(map[string]bool)

	lines := strings.Split(yamlContent, "\n")
	inEnv := false
	envIndent := -1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		currIndent := len(line) - len(strings.TrimLeft(line, " "))

		if strings.HasPrefix(trimmed, "environment:") {
			inEnv = true
			envIndent = currIndent
			continue
		}

		if inEnv {
			// If we hit an unindented line or a line with <= envIndent, we left the environment block
			if envIndent >= 0 && currIndent <= envIndent {
				inEnv = false
				if strings.HasPrefix(trimmed, "environment:") {
					inEnv = true
					envIndent = currIndent
				}
				continue
			}

			// List syntax: - KEY=VAL or - KEY
			if strings.HasPrefix(trimmed, "-") {
				entry := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
				entry = strings.Trim(entry, "\"'")
				if entry == "" {
					continue
				}
				parts := strings.SplitN(entry, "=", 2)
				varName := strings.TrimSpace(parts[0])

				if !isValidEnvVarName(varName) || isReservedComposeKey(varName) || seen[varName] {
					continue
				}

				defaultVal := ""
				required := false
				if len(parts) > 1 {
					val := strings.TrimSpace(parts[1])
					val = strings.Trim(val, "\"'")
					subMatch := envVarPattern.FindStringSubmatch(val)
					if len(subMatch) > 0 {
						if len(subMatch) > 2 && subMatch[2] != "" {
							defaultVal = subMatch[2]
						} else {
							required = true
						}
					} else {
						defaultVal = val
					}
				} else {
					required = true
				}

				seen[varName] = true
				envs = append(envs, TemplateEnvVar{
					Name:        varName,
					Label:       formatEnvLabel(varName),
					Default:     defaultVal,
					Description: fmt.Sprintf("Configures %s", varName),
					Required:    required,
				})
			} else if strings.Contains(trimmed, ":") {
				// Map syntax: KEY: VAL
				parts := strings.SplitN(trimmed, ":", 2)
				varName := strings.TrimSpace(parts[0])

				if isReservedComposeKey(varName) {
					inEnv = false
					continue
				}

				if !isValidEnvVarName(varName) || seen[varName] {
					continue
				}

				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, "\"'")
				defaultVal := val
				required := false
				subMatch := envVarPattern.FindStringSubmatch(val)
				if len(subMatch) > 0 {
					if len(subMatch) > 2 && subMatch[2] != "" {
						defaultVal = subMatch[2]
					} else {
						required = true
						defaultVal = ""
					}
				}
				seen[varName] = true
				envs = append(envs, TemplateEnvVar{
					Name:        varName,
					Label:       formatEnvLabel(varName),
					Default:     defaultVal,
					Description: fmt.Sprintf("Configures %s", varName),
					Required:    required,
				})
			} else {
				inEnv = false
			}
		}
	}
	return envs
}
