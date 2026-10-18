package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
)

// EnvironmentResponse represents the public JSON view of an environment with live connection status.
type EnvironmentResponse struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Type            string    `json:"type"`
	URL             string    `json:"url"`
	TLSCert         string    `json:"tls_cert,omitempty"`
	TLSCA           string    `json:"tls_ca,omitempty"`
	IsDefault       bool      `json:"is_default"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	Status          string    `json:"status"` // "connected" or "disconnected"
	ContainersCount int       `json:"containers_count"`
	Error           string    `json:"error,omitempty"`
}

// CreateEnvironmentRequest defines the payload for creating a new environment.
type CreateEnvironmentRequest struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // "local", "tcp", "tls"
	URL       string `json:"url"`
	TLSCert   string `json:"tls_cert,omitempty"`
	TLSKey    string `json:"tls_key,omitempty"`
	TLSCA     string `json:"tls_ca,omitempty"`
	IsDefault bool   `json:"is_default,omitempty"`
}

// UpdateEnvironmentRequest defines the payload for updating an existing environment.
type UpdateEnvironmentRequest struct {
	Name      *string `json:"name,omitempty"`
	Type      *string `json:"type,omitempty"`
	URL       *string `json:"url,omitempty"`
	TLSCert   *string `json:"tls_cert,omitempty"`
	TLSKey    *string `json:"tls_key,omitempty"`
	TLSCA     *string `json:"tls_ca,omitempty"`
	IsDefault *bool   `json:"is_default,omitempty"`
}

func generateEnvID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "env-" + hex.EncodeToString(b)
}

func probeEnvironment(ctx context.Context, env *store.Environment, reg *docker.DockerServiceRegistry) EnvironmentResponse {
	resp := EnvironmentResponse{
		ID:              env.ID,
		Name:            env.Name,
		Type:            env.Type,
		URL:             env.URL,
		TLSCert:         env.TLSCert,
		TLSCA:           env.TLSCA,
		IsDefault:       env.IsDefault,
		CreatedAt:       env.CreatedAt,
		UpdatedAt:       env.UpdatedAt,
		Status:          "disconnected",
		ContainersCount: 0,
	}

	if reg == nil {
		resp.Error = "docker registry unavailable"
		return resp
	}

	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	svc, err := reg.Get(env.ID)
	if err != nil || svc == nil {
		svc, err = reg.GetByEnv(checkCtx, env)
	}
	if err != nil {
		resp.Status = "disconnected"
		resp.Error = err.Error()
		return resp
	}

	if err := svc.Ping(checkCtx); err != nil {
		resp.Status = "disconnected"
		resp.Error = err.Error()
		reg.Remove(env.ID)
		return resp
	}

	resp.Status = "connected"
	containers, err := svc.ListContainers(checkCtx, true)
	if err == nil {
		resp.ContainersCount = len(containers)
	}

	return resp
}

// HandleListEnvironments returns all environments with their live status and container count.
func HandleListEnvironments(st store.Store, reg *docker.DockerServiceRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "store unavailable")
			return
		}

		envs, err := st.ListEnvironments()
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to list environments: "+err.Error())
			return
		}

		results := make([]EnvironmentResponse, len(envs))
		var wg sync.WaitGroup
		wg.Add(len(envs))
		for i, env := range envs {
			go func(idx int, e *store.Environment) {
				defer wg.Done()
				results[idx] = probeEnvironment(r.Context(), e, reg)
			}(i, env)
		}
		wg.Wait()

		respondJSON(w, http.StatusOK, results)
	}
}

// HandleCreateEnvironment creates a new environment after validating its connection.
func HandleCreateEnvironment(st store.Store, reg *docker.DockerServiceRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "store unavailable")
			return
		}

		var req CreateEnvironmentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		name := strings.TrimSpace(req.Name)
		if name == "" {
			respondJSONError(w, http.StatusBadRequest, "environment name is required")
			return
		}

		envType := strings.TrimSpace(req.Type)
		if envType == "" {
			envType = "local"
		}
		if envType != "local" && envType != "tcp" && envType != "tls" {
			respondJSONError(w, http.StatusBadRequest, "invalid environment type: must be 'local', 'tcp', or 'tls'")
			return
		}

		url := strings.TrimSpace(req.URL)
		if url == "" {
			if envType == "local" {
				url = "unix:///var/run/docker.sock"
			} else {
				respondJSONError(w, http.StatusBadRequest, "environment url is required")
				return
			}
		}

		// Probe connection before persisting
		probeEnv := &store.Environment{
			ID:      "probe-env",
			Name:    name,
			Type:    envType,
			URL:     url,
			TLSCert: req.TLSCert,
			TLSKey:  req.TLSKey,
			TLSCA:   req.TLSCA,
		}

		testSvc, err := docker.NewServiceFromEnvironment(probeEnv)
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, "failed to configure docker client: "+err.Error())
			return
		}
		defer testSvc.Close()

		pingCtx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
		defer cancel()
		if err := testSvc.Ping(pingCtx); err != nil {
			respondJSONError(w, http.StatusBadRequest, "failed to connect to docker daemon: "+err.Error())
			return
		}

		now := time.Now()
		env := &store.Environment{
			ID:        generateEnvID(),
			Name:      name,
			Type:      envType,
			URL:       url,
			TLSCert:   req.TLSCert,
			TLSKey:    req.TLSKey,
			TLSCA:     req.TLSCA,
			IsDefault: req.IsDefault,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := st.SaveEnvironment(env); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to save environment: "+err.Error())
			return
		}

		if reg != nil {
			_, _ = reg.Register(env)
		}

		resp := probeEnvironment(r.Context(), env, reg)
		respondJSON(w, http.StatusCreated, resp)
	}
}

// HandleGetEnvironment returns information and live status for a specific environment.
func HandleGetEnvironment(st store.Store, reg *docker.DockerServiceRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "store unavailable")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "environment id is required")
			return
		}

		env, err := st.GetEnvironment(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				respondJSONError(w, http.StatusNotFound, "environment not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to get environment: "+err.Error())
			return
		}

		resp := probeEnvironment(r.Context(), env, reg)
		respondJSON(w, http.StatusOK, resp)
	}
}

// HandleUpdateEnvironment updates environment configuration.
func HandleUpdateEnvironment(st store.Store, reg *docker.DockerServiceRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "store unavailable")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "environment id is required")
			return
		}

		existing, err := st.GetEnvironment(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				respondJSONError(w, http.StatusNotFound, "environment not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to get environment: "+err.Error())
			return
		}

		var req UpdateEnvironmentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		connectionChanged := false

		if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
			existing.Name = strings.TrimSpace(*req.Name)
		}
		if req.Type != nil && *req.Type != existing.Type {
			if *req.Type != "local" && *req.Type != "tcp" && *req.Type != "tls" {
				respondJSONError(w, http.StatusBadRequest, "invalid environment type: must be 'local', 'tcp', or 'tls'")
				return
			}
			existing.Type = *req.Type
			connectionChanged = true
		}
		if req.URL != nil && *req.URL != existing.URL {
			existing.URL = strings.TrimSpace(*req.URL)
			connectionChanged = true
		}
		if req.TLSCert != nil {
			existing.TLSCert = *req.TLSCert
			connectionChanged = true
		}
		if req.TLSKey != nil {
			existing.TLSKey = *req.TLSKey
			connectionChanged = true
		}
		if req.TLSCA != nil {
			existing.TLSCA = *req.TLSCA
			connectionChanged = true
		}
		if req.IsDefault != nil {
			existing.IsDefault = *req.IsDefault
		}

		// If connection parameters changed, validate connection
		if connectionChanged {
			testSvc, err := docker.NewServiceFromEnvironment(existing)
			if err != nil {
				respondJSONError(w, http.StatusBadRequest, "failed to configure docker client: "+err.Error())
				return
			}
			defer testSvc.Close()

			pingCtx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
			defer cancel()
			if err := testSvc.Ping(pingCtx); err != nil {
				respondJSONError(w, http.StatusBadRequest, "failed to connect to docker daemon: "+err.Error())
				return
			}
		}

		existing.UpdatedAt = time.Now()
		if err := st.SaveEnvironment(existing); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to save environment: "+err.Error())
			return
		}

		if reg != nil {
			reg.Remove(existing.ID)
			_, _ = reg.Register(existing)
		}

		resp := probeEnvironment(r.Context(), existing, reg)
		respondJSON(w, http.StatusOK, resp)
	}
}

// HandleDeleteEnvironment deletes an environment if it is not the last remaining one.
func HandleDeleteEnvironment(st store.Store, reg *docker.DockerServiceRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "store unavailable")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "environment id is required")
			return
		}

		_, err := st.GetEnvironment(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				respondJSONError(w, http.StatusNotFound, "environment not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to get environment: "+err.Error())
			return
		}

		envs, err := st.ListEnvironments()
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to check environments: "+err.Error())
			return
		}
		if len(envs) <= 1 {
			respondJSONError(w, http.StatusBadRequest, "cannot delete the last remaining environment")
			return
		}

		if err := st.DeleteEnvironment(id); err != nil {
			if errors.Is(err, store.ErrCannotDeleteLastEnvironment) {
				respondJSONError(w, http.StatusBadRequest, "cannot delete the last remaining environment")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to delete environment: "+err.Error())
			return
		}

		if reg != nil {
			reg.Remove(id)
		}

		respondJSON(w, http.StatusOK, map[string]string{"message": "environment deleted"})
	}
}

// HandlePingEnvironment tests the daemon connection for a specific environment.
func HandlePingEnvironment(st store.Store, reg *docker.DockerServiceRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "store unavailable")
			return
		}

		id := chi.URLParam(r, "id")
		if strings.TrimSpace(id) == "" {
			respondJSONError(w, http.StatusBadRequest, "environment id is required")
			return
		}

		env, err := st.GetEnvironment(id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				respondJSONError(w, http.StatusNotFound, "environment not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to get environment: "+err.Error())
			return
		}

		if reg == nil {
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":    "disconnected",
				"connected": false,
				"error":     "docker registry unavailable",
			})
			return
		}

		start := time.Now()
		pingCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		svc, err := reg.Get(env.ID)
		if err != nil || svc == nil {
			svc, err = reg.GetByEnv(pingCtx, env)
		}
		if err != nil {
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":    "disconnected",
				"connected": false,
				"error":     err.Error(),
			})
			return
		}

		if err := svc.Ping(pingCtx); err != nil {
			reg.Remove(env.ID)
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":    "disconnected",
				"connected": false,
				"error":     err.Error(),
			})
			return
		}

		latencyMs := time.Since(start).Milliseconds()
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":     "connected",
			"connected":  true,
			"latency_ms": latencyMs,
			"message":    "Docker daemon ping successful",
		})
	}
}

// HandleTestEnvironment probes a prospective Docker daemon connection before saving.
func HandleTestEnvironment() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateEnvironmentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		envType := strings.ToLower(strings.TrimSpace(req.Type))
		if envType == "" {
			envType = "local"
		}
		if envType != "local" && envType != "tcp" && envType != "tls" {
			respondJSONError(w, http.StatusBadRequest, "invalid environment type: must be local, tcp, or tls")
			return
		}

		url := strings.TrimSpace(req.URL)
		if url == "" {
			if envType == "local" {
				url = "unix:///var/run/docker.sock"
			} else {
				respondJSONError(w, http.StatusBadRequest, "environment url is required")
				return
			}
		}

		probeEnv := &store.Environment{
			ID:      "test-probe",
			Name:    req.Name,
			Type:    envType,
			URL:     url,
			TLSCert: req.TLSCert,
			TLSKey:  req.TLSKey,
			TLSCA:   req.TLSCA,
		}

		testSvc, err := docker.NewServiceFromEnvironment(probeEnv)
		if err != nil {
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"success": false,
				"error":   "failed to configure docker client: " + err.Error(),
			})
			return
		}
		defer testSvc.Close()

		start := time.Now()
		pingCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		if err := testSvc.Ping(pingCtx); err != nil {
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"success": false,
				"error":   "connection failed: " + err.Error(),
			})
			return
		}

		latencyMs := time.Since(start).Milliseconds()
		containers, _ := testSvc.ListContainers(pingCtx, true)

		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success":    true,
			"latency_ms": latencyMs,
			"containers": len(containers),
			"message":    fmt.Sprintf("Docker daemon responsive (%d ms, %d container(s) detected)", latencyMs, len(containers)),
		})
	}
}

