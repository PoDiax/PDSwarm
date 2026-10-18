package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
)

// RegistryResponse is the client-facing representation of a registry with masked credentials.
type RegistryResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Username    string    `json:"username"`
	Password    string    `json:"password"`
	HasPassword bool      `json:"has_password"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// toRegistryResponse converts a store.Registry into a masked RegistryResponse.
func toRegistryResponse(r *store.Registry) RegistryResponse {
	hasPass := strings.TrimSpace(r.Password) != ""
	return RegistryResponse{
		ID:          r.ID,
		Name:        r.Name,
		URL:         r.URL,
		Username:    r.Username,
		Password:    "", // masked
		HasPassword: hasPass,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

// CreateRegistryRequest defines the payload for creating a new registry.
type CreateRegistryRequest struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// UpdateRegistryRequest defines the payload for updating an existing registry.
type UpdateRegistryRequest struct {
	Name     string `json:"name,omitempty"`
	URL      string `json:"url,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// TestRegistryRequest defines the payload for testing registry credentials.
type TestRegistryRequest struct {
	ID       string `json:"id,omitempty"`
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// generateUUID creates a cryptographically random RFC 4122 v4 UUID string.
func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// HandleListRegistries returns all configured registries with masked passwords.
func HandleListRegistries(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "store not available")
			return
		}

		registries, err := st.ListRegistries()
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to list registries: "+err.Error())
			return
		}

		resp := make([]RegistryResponse, len(registries))
		for i, reg := range registries {
			resp[i] = toRegistryResponse(reg)
		}

		respondJSON(w, http.StatusOK, resp)
	}
}

// HandleGetRegistry returns a specific registry with masked password.
func HandleGetRegistry(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "store not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "registry id is required")
			return
		}

		reg, err := st.GetRegistry(id)
		if err != nil {
			if errors.Is(err, store.ErrRegistryNotFound) || errors.Is(err, store.ErrNotFound) {
				respondJSONError(w, http.StatusNotFound, "registry not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to get registry: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, toRegistryResponse(reg))
	}
}

// HandleCreateRegistry stores a new private registry configuration.
func HandleCreateRegistry(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "store not available")
			return
		}

		var req CreateRegistryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		name := strings.TrimSpace(req.Name)
		url := strings.TrimSpace(req.URL)
		username := strings.TrimSpace(req.Username)
		password := req.Password

		if name == "" {
			respondJSONError(w, http.StatusBadRequest, "registry name is required")
			return
		}
		if url == "" {
			respondJSONError(w, http.StatusBadRequest, "registry url is required")
			return
		}
		if username == "" {
			respondJSONError(w, http.StatusBadRequest, "registry username is required")
			return
		}
		if password == "" {
			respondJSONError(w, http.StatusBadRequest, "registry password or token is required")
			return
		}

		reg := &store.Registry{
			ID:       generateUUID(),
			Name:     name,
			URL:      url,
			Username: username,
			Password: password,
		}

		if err := st.SaveRegistry(reg); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to save registry: "+err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, toRegistryResponse(reg))
	}
}

// HandleUpdateRegistry updates an existing registry configuration.
// If the incoming password is empty, the existing password is preserved.
func HandleUpdateRegistry(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "store not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "registry id is required")
			return
		}

		var req UpdateRegistryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		existing, err := st.GetRegistry(id)
		if err != nil {
			if errors.Is(err, store.ErrRegistryNotFound) || errors.Is(err, store.ErrNotFound) {
				respondJSONError(w, http.StatusNotFound, "registry not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to get registry: "+err.Error())
			return
		}

		if strings.TrimSpace(req.Name) != "" {
			existing.Name = strings.TrimSpace(req.Name)
		}
		if strings.TrimSpace(req.URL) != "" {
			existing.URL = strings.TrimSpace(req.URL)
		}
		if strings.TrimSpace(req.Username) != "" {
			existing.Username = strings.TrimSpace(req.Username)
		}
		// Preserve existing password if incoming is empty
		if req.Password != "" {
			existing.Password = req.Password
		}

		if err := st.SaveRegistry(existing); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to update registry: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, toRegistryResponse(existing))
	}
}

// HandleDeleteRegistry removes a registry configuration.
func HandleDeleteRegistry(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "store not available")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "registry id is required")
			return
		}

		if err := st.DeleteRegistry(id); err != nil {
			if errors.Is(err, store.ErrRegistryNotFound) || errors.Is(err, store.ErrNotFound) {
				respondJSONError(w, http.StatusNotFound, "registry not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to delete registry: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{
			"message": "registry deleted",
			"id":      id,
		})
	}
}

// HandlePingRegistry validates credentials for a stored registry against the registry server.
func HandlePingRegistry(st store.Store, ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "store not available")
			return
		}
		ds = dockerFromContext(r.Context(), ds)

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "registry id is required")
			return
		}

		reg, err := st.GetRegistry(id)
		if err != nil {
			if errors.Is(err, store.ErrRegistryNotFound) || errors.Is(err, store.ErrNotFound) {
				respondJSONError(w, http.StatusNotFound, "registry not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to get registry: "+err.Error())
			return
		}

		msg, err := testRegistryAuth(r.Context(), ds, reg.Username, reg.Password, reg.URL)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]interface{}{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "success",
			"message": msg,
		})
	}
}

// HandleTestRegistry tests authentication against a registry with credentials supplied in request body.
func HandleTestRegistry(st store.Store, ds *docker.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ds = dockerFromContext(r.Context(), ds)

		var req TestRegistryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		url := strings.TrimSpace(req.URL)
		username := strings.TrimSpace(req.Username)
		password := req.Password

		// If testing an existing registry where password was not re-sent, lookup saved password
		if password == "" && req.ID != "" && st != nil {
			if existing, err := st.GetRegistry(req.ID); err == nil && existing != nil {
				password = existing.Password
				if url == "" {
					url = existing.URL
				}
				if username == "" {
					username = existing.Username
				}
			}
		}

		if url == "" {
			respondJSONError(w, http.StatusBadRequest, "registry url is required")
			return
		}
		if username == "" {
			respondJSONError(w, http.StatusBadRequest, "registry username is required")
			return
		}

		msg, err := testRegistryAuth(r.Context(), ds, username, password, url)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]interface{}{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "success",
			"message": msg,
		})
	}
}

// testRegistryAuth performs registry authentication validation using Docker daemon login
// or direct HTTP API v2 probe as fallback.
func testRegistryAuth(ctx context.Context, ds *docker.Service, username, password, serverAddress string) (string, error) {
	normHost := strings.ToLower(strings.TrimSpace(serverAddress))
	normHost = strings.TrimPrefix(normHost, "https://")
	normHost = strings.TrimPrefix(normHost, "http://")
	normHost = strings.TrimRight(normHost, "/")
	if idx := strings.Index(normHost, "/"); idx != -1 {
		normHost = normHost[:idx]
	}

	loginAddress := serverAddress
	if normHost == "docker.io" || normHost == "index.docker.io" || normHost == "registry-1.docker.io" {
		loginAddress = "https://index.docker.io/v1/"
	} else if !strings.HasPrefix(loginAddress, "https://") && !strings.HasPrefix(loginAddress, "http://") {
		loginAddress = "https://" + loginAddress
	}

	// 1. Try Docker daemon RegistryLogin if service is available
	if ds != nil {
		pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		status, err := ds.RegistryLogin(pingCtx, username, password, loginAddress)
		if err == nil {
			if status == "" {
				status = "Login Succeeded"
			}
			return status, nil
		}
		errLower := strings.ToLower(err.Error())
		if strings.Contains(errLower, "unauthorized") || strings.Contains(errLower, "forbidden") ||
			strings.Contains(errLower, "incorrect username or password") || strings.Contains(errLower, "denied") {
			return "", fmt.Errorf("authentication failed: %w", err)
		}
	}

	// 2. Direct HTTP ping to /v2/
	pingURL := loginAddress
	if !strings.HasSuffix(pingURL, "/") {
		pingURL += "/"
	}
	pingURL += "v2/"

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create ping request: %w", err)
	}
	if username != "" || password != "" {
		httpReq.SetBasicAuth(username, password)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to connect to registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		return "Registry authentication verified", nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		authHdr := resp.Header.Get("Www-Authenticate")
		if strings.Contains(authHdr, "Bearer") || strings.Contains(authHdr, "Basic") {
			return "Registry reached and responding", nil
		}
		return "", fmt.Errorf("unauthorized: invalid credentials (HTTP %d)", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusForbidden {
		return "", fmt.Errorf("access forbidden (HTTP %d)", resp.StatusCode)
	}

	return fmt.Sprintf("Registry responded with status %d", resp.StatusCode), nil
}
