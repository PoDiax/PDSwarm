package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
	"github.com/podiax/pdswarm/internal/templates"
)

// ListTemplatesResponse is returned by GET /api/templates.
type ListTemplatesResponse struct {
	Templates      []templates.Template `json:"templates"`
	Total          int                  `json:"total"`
	Categories     []string             `json:"categories"`
	Sources        []string             `json:"sources"`
	CategoryCounts map[string]int       `json:"category_counts"`
}

// ListCategoriesResponse is returned by GET /api/templates/categories.
type ListCategoriesResponse struct {
	Categories []string       `json:"categories"`
	Counts     map[string]int `json:"counts"`
}

// DeployTemplateResponse is returned by POST /api/templates/deploy.
type DeployTemplateResponse struct {
	Status    string       `json:"status"`
	StackName string       `json:"stack_name"`
	Stack     *store.Stack `json:"stack,omitempty"`
}

// HandleListTemplates handles GET /api/templates with search, category, and source filters.
func HandleListTemplates(cm *templates.CatalogManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cm == nil {
			respondJSONError(w, http.StatusInternalServerError, "catalog manager not initialized")
			return
		}

		q := r.URL.Query().Get("q")
		category := r.URL.Query().Get("category")
		source := r.URL.Query().Get("source")

		results := cm.ListTemplates(q, category, source)
		if results == nil {
			results = []templates.Template{}
		}

		resp := ListTemplatesResponse{
			Templates:      results,
			Total:          len(results),
			Categories:     cm.Categories(),
			Sources:        cm.Sources(),
			CategoryCounts: cm.CategoryCounts(),
		}

		respondJSON(w, http.StatusOK, resp)
	}
}

// HandleListTemplateCategories handles GET /api/templates/categories.
func HandleListTemplateCategories(cm *templates.CatalogManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cm == nil {
			respondJSONError(w, http.StatusInternalServerError, "catalog manager not initialized")
			return
		}

		resp := ListCategoriesResponse{
			Categories: cm.Categories(),
			Counts:     cm.CategoryCounts(),
		}

		respondJSON(w, http.StatusOK, resp)
	}
}

// HandleGetTemplate handles GET /api/templates/{id}.
func HandleGetTemplate(cm *templates.CatalogManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cm == nil {
			respondJSONError(w, http.StatusInternalServerError, "catalog manager not initialized")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "template id is required")
			return
		}

		tmpl, err := cm.GetTemplate(id)
		if err != nil {
			if errors.Is(err, templates.ErrTemplateNotFound) {
				respondJSONError(w, http.StatusNotFound, "template not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		respondJSON(w, http.StatusOK, tmpl)
	}
}

// HandleDeployTemplate handles POST /api/templates/deploy.
func HandleDeployTemplate(cm *templates.CatalogManager, st store.Store, defaultDS *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cm == nil {
			respondJSONError(w, http.StatusInternalServerError, "catalog manager not initialized")
			return
		}
		if st == nil {
			respondJSONError(w, http.StatusInternalServerError, "storage store not available")
			return
		}

		var req templates.DeployTemplateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		req.TemplateID = strings.TrimSpace(req.TemplateID)
		req.StackName = strings.TrimSpace(req.StackName)

		if req.TemplateID == "" {
			respondJSONError(w, http.StatusBadRequest, "template_id is required")
			return
		}
		if req.StackName == "" {
			respondJSONError(w, http.StatusBadRequest, "stack_name is required")
			return
		}

		// Resolve docker service from context
		ds := dockerFromContext(r.Context(), defaultDS)

		// Allow up to 10 minutes for deployment (image pulling, container creation, startup)
		deployCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		isStreaming := r.URL.Query().Get("stream") == "true"
		var onProgress docker.DeployProgressFunc

		if isStreaming {
			w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("X-Accel-Buffering", "no")
			w.WriteHeader(http.StatusOK)

			flusher, hasFlusher := w.(http.Flusher)
			if hasFlusher {
				flusher.Flush()
			}

			onProgress = func(evt docker.DeployEvent) {
				data, err := json.Marshal(evt)
				if err == nil {
					_, _ = w.Write(append(data, '\n'))
					if hasFlusher {
						flusher.Flush()
					}
				}
			}
		}

		stack, err := cm.DeployWithProgress(deployCtx, ds, st, req, onProgress)
		if err != nil {
			if isStreaming {
				errEvt := docker.DeployEvent{
					Step:      "error",
					Status:    "Deployment failed",
					Detail:    err.Error(),
					Timestamp: time.Now().Unix(),
				}
				data, _ := json.Marshal(errEvt)
				_, _ = w.Write(append(data, '\n'))
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
				return
			}
			if errors.Is(err, templates.ErrTemplateNotFound) {
				respondJSONError(w, http.StatusNotFound, "template not found")
				return
			}
			if errors.Is(err, templates.ErrStackConflict) {
				respondJSONError(w, http.StatusConflict, err.Error())
				return
			}
			if errors.Is(err, templates.ErrInvalidInput) {
				respondJSONError(w, http.StatusBadRequest, err.Error())
				return
			}
			respondJSONError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if isStreaming {
			doneEvt := docker.DeployEvent{
				Step:      "done",
				Status:    "Application deployed successfully",
				Detail:    req.StackName,
				Timestamp: time.Now().Unix(),
			}
			data, _ := json.Marshal(doneEvt)
			_, _ = w.Write(append(data, '\n'))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			return
		}

		resp := DeployTemplateResponse{
			Status:    "deployed",
			StackName: req.StackName,
			Stack:     stack,
		}

		respondJSON(w, http.StatusCreated, resp)
	}
}
