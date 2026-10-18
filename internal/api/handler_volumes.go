package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/docker"
)

type CreateVolumeRequest struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
}

// HandleListVolumes returns the list of Docker volumes.
func HandleListVolumes(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		resp, err := ds.ListVolumes(r.Context())
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to list volumes: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, resp)
	}
}

// HandleCreateVolume creates a new Docker volume.
func HandleCreateVolume(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		var req CreateVolumeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			respondJSONError(w, http.StatusBadRequest, "volume name is required")
			return
		}

		vol, err := ds.CreateVolume(r.Context(), req.Name, req.Labels)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to create volume: "+err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, vol)
	}
}

// HandleDeleteVolume removes a Docker volume by name.
func HandleDeleteVolume(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		name := chi.URLParam(r, "name")
		if strings.TrimSpace(name) == "" {
			respondJSONError(w, http.StatusBadRequest, "volume name is required")
			return
		}

		force := r.URL.Query().Get("force") == "true" || r.URL.Query().Get("force") == "1"

		if err := ds.RemoveVolume(r.Context(), name, force); err != nil {
			if client.IsErrNotFound(err) {
				respondJSONError(w, http.StatusNotFound, "volume not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to remove volume: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"message": "volume removed"})
	}
}

// HandlePruneVolumes deletes all unused local volumes.
func HandlePruneVolumes(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		report, err := ds.PruneVolumes(r.Context())
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to prune volumes: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, report)
	}
}
