package api

import (
	"encoding/json"
	"net/http"

	"github.com/podiax/pdswarm/internal/docker"
)

// PortsListResponse represents the response containing all allocated host ports.
type PortsListResponse struct {
	Allocated []docker.PortAllocation `json:"allocated"`
	UsedPorts []int                   `json:"used_ports"`
}

// CheckPortsRequest represents payload to check for port conflicts.
type CheckPortsRequest struct {
	Ports []string `json:"ports"`
}

// HandleGetAllocatedPorts returns all ports currently allocated by containers on the host.
func HandleGetAllocatedPorts(defaultDS *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds := dockerFromContext(r.Context(), defaultDS)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		allocations, err := ds.GetAllocatedPorts(r.Context())
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to get allocated ports: "+err.Error())
			return
		}

		usedMap := make(map[int]bool)
		for _, a := range allocations {
			if a.HostPort > 0 {
				usedMap[a.HostPort] = true
			}
		}

		usedList := make([]int, 0, len(usedMap))
		for p := range usedMap {
			usedList = append(usedList, p)
		}

		respondJSON(w, http.StatusOK, PortsListResponse{
			Allocated: allocations,
			UsedPorts: usedList,
		})
	}
}

// HandleCheckPortConflicts checks whether a set of requested ports conflicts with existing
// container bindings or host network listeners, and provides auto-suggested free ports.
func HandleCheckPortConflicts(defaultDS *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds := dockerFromContext(r.Context(), defaultDS)
		if ds == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "docker service not available")
			return
		}

		var req CheckPortsRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		result, err := ds.CheckPortConflicts(r.Context(), req.Ports)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to check port conflicts: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, result)
	}
}
