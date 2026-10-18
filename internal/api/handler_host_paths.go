package api

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/podiax/pdswarm/internal/docker"
)

// HostPathSuggestion represents a suggested directory or Docker named volume.
type HostPathSuggestion struct {
	Path  string `json:"path"`
	Name  string `json:"name"`
	Type  string `json:"type"` // "dir" or "volume"
	IsDir bool   `json:"is_dir"`
}

// HostPathResponse is returned by GET /api/system/host-paths.
type HostPathResponse struct {
	CurrentDir  string               `json:"current_dir"`
	Suggestions []HostPathSuggestion `json:"suggestions"`
}

// HandleSuggestHostPaths returns host directory autocomplete suggestions and Docker named volumes.
func HandleSuggestHostPaths(defaultDS *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds := dockerFromContext(r.Context(), defaultDS)

		rawQuery := strings.TrimSpace(r.URL.Query().Get("query"))
		if rawQuery == "" {
			rawQuery = strings.TrimSpace(r.URL.Query().Get("path"))
		}

		// Expand ~ to user's home directory
		if strings.HasPrefix(rawQuery, "~") {
			if home, err := os.UserHomeDir(); err == nil && home != "" {
				rest := strings.TrimPrefix(rawQuery, "~")
				rest = strings.TrimPrefix(rest, "/")
				rawQuery = filepath.Join(home, rest)
			}
		}

		var suggestions []HostPathSuggestion
		currentDir := ""

		if rawQuery == "" {
			// 1. Common host directory roots
			if home, err := os.UserHomeDir(); err == nil && home != "" {
				suggestions = append(suggestions, HostPathSuggestion{
					Path:  filepath.ToSlash(home),
					Name:  "~ (" + filepath.Base(home) + ")",
					Type:  "dir",
					IsDir: true,
				})
			}

			commonRoots := []string{"/", "/home", "/var", "/opt", "/mnt", "/srv", "/media", "/data"}
			for _, root := range commonRoots {
				if fi, err := os.Stat(root); err == nil && fi.IsDir() {
					suggestions = append(suggestions, HostPathSuggestion{
						Path:  filepath.ToSlash(root),
						Name:  root,
						Type:  "dir",
						IsDir: true,
					})
				}
			}

			// 2. Docker named volumes
			if ds != nil {
				if volList, err := ds.ListVolumes(r.Context()); err == nil {
					for _, v := range volList.Volumes {
						suggestions = append(suggestions, HostPathSuggestion{
							Path:  v.Name,
							Name:  v.Name + " (Docker Volume)",
							Type:  "volume",
							IsDir: true,
						})
					}
				}
			}

			respondJSON(w, http.StatusOK, HostPathResponse{
				CurrentDir:  "",
				Suggestions: suggestions,
			})
			return
		}

		// Detect if this is a path query (starts with /, ., or drive letter)
		isFilePathQuery := strings.HasPrefix(rawQuery, "/") ||
			strings.HasPrefix(rawQuery, ".") ||
			(len(rawQuery) >= 2 && rawQuery[1] == ':')

		if isFilePathQuery {
			var dirToRead string
			var prefix string

			if strings.HasSuffix(rawQuery, "/") || strings.HasSuffix(rawQuery, "\\") {
				dirToRead = filepath.Clean(rawQuery)
				prefix = ""
			} else {
				dirToRead = filepath.Dir(rawQuery)
				prefix = strings.ToLower(filepath.Base(rawQuery))
			}

			currentDir = filepath.ToSlash(dirToRead)

			entries, err := os.ReadDir(dirToRead)
			if err == nil {
				var nonHidden []HostPathSuggestion
				var hidden []HostPathSuggestion

				for _, entry := range entries {
					isDir := entry.IsDir()
					if !isDir && (entry.Type()&os.ModeSymlink != 0) {
						targetPath := filepath.Join(dirToRead, entry.Name())
						if fi, err := os.Stat(targetPath); err == nil && fi.IsDir() {
							isDir = true
						}
					}

					if !isDir {
						continue
					}

					name := entry.Name()
					lowerName := strings.ToLower(name)
					if prefix != "" && !strings.HasPrefix(lowerName, prefix) {
						continue
					}

					fullPath := filepath.ToSlash(filepath.Join(dirToRead, name))
					sug := HostPathSuggestion{
						Path:  fullPath,
						Name:  name,
						Type:  "dir",
						IsDir: true,
					}

					if strings.HasPrefix(name, ".") {
						hidden = append(hidden, sug)
					} else {
						nonHidden = append(nonHidden, sug)
					}

					if len(nonHidden)+len(hidden) >= 60 {
						break
					}
				}

				sort.Slice(nonHidden, func(i, j int) bool {
					return strings.ToLower(nonHidden[i].Name) < strings.ToLower(nonHidden[j].Name)
				})
				sort.Slice(hidden, func(i, j int) bool {
					return strings.ToLower(hidden[i].Name) < strings.ToLower(hidden[j].Name)
				})

				suggestions = append(suggestions, nonHidden...)
				suggestions = append(suggestions, hidden...)
			}
		} else {
			// Query does not start with slash: Search Docker named volumes & matching roots
			lowerQ := strings.ToLower(rawQuery)
			if ds != nil {
				if volList, err := ds.ListVolumes(r.Context()); err == nil {
					for _, v := range volList.Volumes {
						if strings.Contains(strings.ToLower(v.Name), lowerQ) {
							suggestions = append(suggestions, HostPathSuggestion{
								Path:  v.Name,
								Name:  v.Name + " (Docker Volume)",
								Type:  "volume",
								IsDir: true,
							})
						}
					}
				}
			}

			// Also suggest matching common root directories like /home, /var, etc.
			for _, root := range []string{"home", "var", "opt", "mnt", "srv", "media", "data", "etc"} {
				if strings.HasPrefix(root, lowerQ) {
					full := "/" + root
					if fi, err := os.Stat(full); err == nil && fi.IsDir() {
						suggestions = append(suggestions, HostPathSuggestion{
							Path:  full,
							Name:  full,
							Type:  "dir",
							IsDir: true,
						})
					}
				}
			}
		}

		respondJSON(w, http.StatusOK, HostPathResponse{
			CurrentDir:  currentDir,
			Suggestions: suggestions,
		})
	}
}
