package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/docker"
)

type CreateNetworkRequest struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
}

// HandleListNetworks lists all networks configured in the Docker daemon.
func HandleListNetworks(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		nets, err := ds.ListNetworks(r.Context())
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to list networks: "+err.Error())
			return
		}

		if nets == nil {
			nets = []network.Summary{}
		}
		respondJSON(w, http.StatusOK, nets)
	}
}

// HandleCreateNetwork creates a new Docker network.
func HandleCreateNetwork(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		var req CreateNetworkRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			respondJSONError(w, http.StatusBadRequest, "network name is required")
			return
		}

		id, err := ds.CreateNetwork(r.Context(), req.Name, req.Driver)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to create network: "+err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, map[string]string{"id": id})
	}
}

// HandleDeleteNetwork removes a Docker network by ID or name.
func HandleDeleteNetwork(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "network id is required")
			return
		}

		if err := ds.RemoveNetwork(r.Context(), id); err != nil {
			if client.IsErrNotFound(err) {
				respondJSONError(w, http.StatusNotFound, "network not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to remove network: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"message": "network removed"})
	}
}

// HandlePruneNetworks removes all unused networks.
func HandlePruneNetworks(ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		report, err := ds.PruneNetworks(r.Context())
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to prune networks: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, report)
	}
}
