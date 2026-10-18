package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/podiax/pdswarm/internal/store"
	"github.com/podiax/pdswarm/internal/templates"
)

// TemplateSettingsPayload represents configuration for application templates catalog.
type TemplateSettingsPayload struct {
	DisableDefaultTemplates bool `json:"disable_default_templates"`
}

// CreateTemplateRepoRequest represents the payload to register an external template repository.
type CreateTemplateRepoRequest struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Type    string `json:"type"` // "auto", "portainer", "casaos_zip"
	Enabled *bool  `json:"enabled"`
}

// HandleGetTemplateSettings returns template settings (e.g. whether default curated templates are disabled).
func HandleGetTemplateSettings(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		val, _ := st.GetSetting("disable_default_templates")
		respondJSON(w, http.StatusOK, TemplateSettingsPayload{
			DisableDefaultTemplates: val == "true",
		})
	}
}

// HandleUpdateTemplateSettings updates template settings and triggers catalog rebuild.
func HandleUpdateTemplateSettings(cm *templates.CatalogManager, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req TemplateSettingsPayload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		val := "false"
		if req.DisableDefaultTemplates {
			val = "true"
		}
		if err := st.SaveSetting("disable_default_templates", val); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to save setting: "+err.Error())
			return
		}

		if cm != nil {
			_ = cm.Rebuild(r.Context(), nil, st)
		}

		respondJSON(w, http.StatusOK, req)
	}
}

// HandleListTemplateRepos lists all configured template repositories.
func HandleListTemplateRepos(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		repos, err := st.ListTemplateRepos()
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to list template repositories: "+err.Error())
			return
		}
		if repos == nil {
			repos = []*store.TemplateRepo{}
		}
		respondJSON(w, http.StatusOK, repos)
	}
}

// HandleCreateTemplateRepo adds a new external template repository, attempts an initial sync, and persists it.
func HandleCreateTemplateRepo(cm *templates.CatalogManager, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateTemplateRepoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		req.Name = strings.TrimSpace(req.Name)
		req.URL = strings.TrimSpace(req.URL)
		req.Type = strings.TrimSpace(req.Type)

		if req.Name == "" {
			respondJSONError(w, http.StatusBadRequest, "repository name is required")
			return
		}
		if req.URL == "" {
			respondJSONError(w, http.StatusBadRequest, "repository URL is required")
			return
		}
		if req.Type == "" {
			req.Type = "auto"
		}

		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}

		now := time.Now()
		repo := &store.TemplateRepo{
			ID:        uuid.New().String(),
			Name:      req.Name,
			URL:       req.URL,
			Type:      req.Type,
			Enabled:   enabled,
			CreatedAt: now,
			UpdatedAt: now,
		}

		// Initial sync if enabled
		if enabled && cm != nil {
			tmpls, err := cm.SyncRepo(r.Context(), nil, repo)
			if err != nil {
				repo.SyncError = err.Error()
			} else {
				repo.ItemCount = len(tmpls)
				repo.LastSyncAt = &now
				repo.SyncError = ""
			}
		}

		if err := st.SaveTemplateRepo(repo); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to save repository: "+err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, repo)
	}
}

// HandleUpdateTemplateRepo updates an external template repository.
func HandleUpdateTemplateRepo(cm *templates.CatalogManager, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "repository ID is required")
			return
		}

		existing, err := st.GetTemplateRepo(id)
		if err != nil {
			if errors.Is(err, store.ErrTemplateRepoNotFound) {
				respondJSONError(w, http.StatusNotFound, "template repository not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var req CreateTemplateRepoRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		if strings.TrimSpace(req.Name) != "" {
			existing.Name = strings.TrimSpace(req.Name)
		}
		if strings.TrimSpace(req.URL) != "" {
			existing.URL = strings.TrimSpace(req.URL)
		}
		if strings.TrimSpace(req.Type) != "" {
			existing.Type = strings.TrimSpace(req.Type)
		}
		if req.Enabled != nil {
			existing.Enabled = *req.Enabled
		}
		existing.UpdatedAt = time.Now()

		if err := st.SaveTemplateRepo(existing); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to update repository: "+err.Error())
			return
		}

		if cm != nil {
			_ = cm.Rebuild(r.Context(), nil, st)
		}

		respondJSON(w, http.StatusOK, existing)
	}
}

// HandleDeleteTemplateRepo deletes an external template repository.
func HandleDeleteTemplateRepo(cm *templates.CatalogManager, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "repository ID is required")
			return
		}

		if err := st.DeleteTemplateRepo(id); err != nil {
			if errors.Is(err, store.ErrTemplateRepoNotFound) {
				respondJSONError(w, http.StatusNotFound, "template repository not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if cm != nil {
			_ = cm.Rebuild(r.Context(), nil, st)
		}

		respondJSON(w, http.StatusOK, map[string]string{
			"status": "deleted",
			"id":     id,
		})
	}
}

// HandleSyncTemplateRepo triggers on-demand sync of a specific repository.
func HandleSyncTemplateRepo(cm *templates.CatalogManager, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "repository ID is required")
			return
		}

		repo, err := st.GetTemplateRepo(id)
		if err != nil {
			if errors.Is(err, store.ErrTemplateRepoNotFound) {
				respondJSONError(w, http.StatusNotFound, "template repository not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if cm == nil {
			respondJSONError(w, http.StatusInternalServerError, "catalog manager not initialized")
			return
		}

		tmpls, err := cm.SyncRepo(r.Context(), nil, repo)
		now := time.Now()
		if err != nil {
			repo.SyncError = err.Error()
			_ = st.SaveTemplateRepo(repo)
			respondJSONError(w, http.StatusBadGateway, "failed to sync repository: "+err.Error())
			return
		}

		repo.SyncError = ""
		repo.ItemCount = len(tmpls)
		repo.LastSyncAt = &now
		if err := st.SaveTemplateRepo(repo); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to save repository: "+err.Error())
			return
		}

		_ = cm.Rebuild(r.Context(), nil, st)

		respondJSON(w, http.StatusOK, repo)
	}
}

// HandleSyncAllTemplateRepos syncs all enabled repositories and rebuilds the active catalog.
func HandleSyncAllTemplateRepos(cm *templates.CatalogManager, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cm == nil {
			respondJSONError(w, http.StatusInternalServerError, "catalog manager not initialized")
			return
		}

		if err := cm.Rebuild(r.Context(), nil, st); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to rebuild template catalog: "+err.Error())
			return
		}

		repos, _ := st.ListTemplateRepos()
		if repos == nil {
			repos = []*store.TemplateRepo{}
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":       "synchronized",
			"total_repos":  len(repos),
			"total_apps":   len(cm.ListTemplates("", "", "")),
			"repositories": repos,
		})
	}
}
