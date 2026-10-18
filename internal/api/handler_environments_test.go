package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/podiax/pdswarm/internal/api"
	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
)

func setupEnvironmentsTestServer(t *testing.T) (http.Handler, store.Store, *docker.DockerServiceRegistry, *docker.Service) {
	t.Helper()
	st := newTestStore(t)
	cfg := &config.Config{NoAuth: true}

	ds, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create local docker service: %v", err)
	}
	t.Cleanup(func() { _ = ds.Close() })

	reg := docker.NewDockerServiceRegistry()
	t.Cleanup(func() { _ = reg.Close() })

	// Pre-seed local service in registry
	reg.Set("local", ds)

	handler := api.SetupRouterWithRegistry(cfg, st, ds, reg)
	return handler, st, reg, ds
}

func TestEnvironments_List(t *testing.T) {
	handler, _, _, _ := setupEnvironmentsTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/environments", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var envs []api.EnvironmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &envs); err != nil {
		t.Fatalf("failed to parse environments response: %v", err)
	}

	if len(envs) != 1 {
		t.Fatalf("expected 1 seeded environment, got %d", len(envs))
	}

	if envs[0].ID != "local" || !envs[0].IsDefault {
		t.Fatalf("expected seeded environment to be 'local' and default, got ID=%s, default=%v", envs[0].ID, envs[0].IsDefault)
	}

	if envs[0].Status != "connected" {
		t.Fatalf("expected local environment status 'connected', got %q", envs[0].Status)
	}
}

func TestEnvironments_Get(t *testing.T) {
	handler, _, _, _ := setupEnvironmentsTestServer(t)

	// Existing environment
	req := httptest.NewRequest(http.MethodGet, "/api/environments/local", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var env api.EnvironmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to parse environment: %v", err)
	}
	if env.ID != "local" {
		t.Fatalf("expected ID 'local', got %q", env.ID)
	}

	// Non-existent environment
	req = httptest.NewRequest(http.MethodGet, "/api/environments/non-existent", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", rec.Code)
	}
}

func TestEnvironments_CreateValidation(t *testing.T) {
	handler, _, _, _ := setupEnvironmentsTestServer(t)

	// Missing name
	body, _ := json.Marshal(api.CreateEnvironmentRequest{
		Name: "",
		Type: "local",
		URL:  "unix:///var/run/docker.sock",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/environments", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty name, got %d: %s", rec.Code, rec.Body.String())
	}

	// Invalid type
	body, _ = json.Marshal(api.CreateEnvironmentRequest{
		Name: "Test Env",
		Type: "unknown-type",
		URL:  "tcp://localhost:2375",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/environments", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid type, got %d: %s", rec.Code, rec.Body.String())
	}

	// Missing URL for TCP
	body, _ = json.Marshal(api.CreateEnvironmentRequest{
		Name: "TCP Env",
		Type: "tcp",
		URL:  "",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/environments", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty TCP url, got %d: %s", rec.Code, rec.Body.String())
	}

	// Unreachable daemon connection probe failure
	body, _ = json.Marshal(api.CreateEnvironmentRequest{
		Name: "Unreachable Daemon",
		Type: "tcp",
		URL:  "tcp://127.0.0.1:54321",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/environments", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unreachable daemon ping probe, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEnvironments_CreateAndLifecycle(t *testing.T) {
	handler, st, _, _ := setupEnvironmentsTestServer(t)

	// Create a second valid environment pointing to local socket
	body, _ := json.Marshal(api.CreateEnvironmentRequest{
		Name:      "Secondary Local",
		Type:      "local",
		URL:       "/var/run/docker.sock",
		IsDefault: false,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/environments", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var created api.EnvironmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode created environment: %v", err)
	}
	if created.ID == "" || created.Name != "Secondary Local" {
		t.Fatalf("unexpected created environment fields: %+v", created)
	}

	// Verify list now returns 2
	envs, err := st.ListEnvironments()
	if err != nil || len(envs) != 2 {
		t.Fatalf("expected 2 environments in store, got %d (err: %v)", len(envs), err)
	}

	// Update environment name
	newName := "Renamed Secondary"
	updateBody, _ := json.Marshal(api.UpdateEnvironmentRequest{
		Name: &newName,
	})
	req = httptest.NewRequest(http.MethodPut, "/api/environments/"+created.ID, bytes.NewReader(updateBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK updating environment, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated api.EnvironmentResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &updated)
	if updated.Name != newName {
		t.Fatalf("expected name to be %q, got %q", newName, updated.Name)
	}

	// Ping environment
	req = httptest.NewRequest(http.MethodPost, "/api/environments/"+created.ID+"/ping", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for ping, got %d: %s", rec.Code, rec.Body.String())
	}

	var pingRes map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &pingRes)
	if connected, ok := pingRes["connected"].(bool); !ok || !connected {
		t.Fatalf("expected connected=true in ping response, got %+v", pingRes)
	}

	// Delete created environment (we have 2, so deletion is allowed)
	req = httptest.NewRequest(http.MethodDelete, "/api/environments/"+created.ID, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK deleting environment, got %d: %s", rec.Code, rec.Body.String())
	}

	// Now only 1 environment remains; attempt to delete the last one must fail
	req = httptest.NewRequest(http.MethodDelete, "/api/environments/local", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request deleting last remaining environment, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEnvironments_DynamicResolutionMiddleware(t *testing.T) {
	handler, st, reg, localDS := setupEnvironmentsTestServer(t)

	// Save a mock custom environment
	customEnv := &store.Environment{
		ID:        "env-custom",
		Name:      "Custom Host",
		Type:      "local",
		URL:       "/var/run/docker.sock",
		IsDefault: false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := st.SaveEnvironment(customEnv); err != nil {
		t.Fatalf("failed to save custom env: %v", err)
	}

	// Register localDS under "env-custom"
	reg.Set("env-custom", localDS)

	// 1. Hit /api/system/info with X-Environment-ID header
	req := httptest.NewRequest(http.MethodGet, "/api/system/info", nil)
	req.Header.Set("X-Environment-ID", "env-custom")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with X-Environment-ID, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Hit /api/containers with ?env= query parameter
	req = httptest.NewRequest(http.MethodGet, "/api/containers?env=env-custom", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with ?env= query param, got %d: %s", rec.Code, rec.Body.String())
	}

	// 3. Hit with non-existent environment ID should fall back to defaultDS and succeed
	req = httptest.NewRequest(http.MethodGet, "/api/containers?env=non-existent", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with fallback to defaultDS, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEnvironments_TestEndpoint(t *testing.T) {
	handler, _, _, _ := setupEnvironmentsTestServer(t)

	// Valid local environment probe
	body := map[string]interface{}{
		"name": "Probe Test",
		"type": "local",
		"url":  "unix:///var/run/docker.sock",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/environments/test", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		t.Fatalf("expected success true, got %v", result)
	}

	// Invalid connection probe (non-existent socket)
	badBody := map[string]interface{}{
		"name": "Bad Socket",
		"type": "local",
		"url":  "unix:///non/existent/docker.sock",
	}
	badBytes, _ := json.Marshal(badBody)

	req = httptest.NewRequest(http.MethodPost, "/api/environments/test", bytes.NewReader(badBytes))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var badResult map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &badResult); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if success, ok := badResult["success"].(bool); !ok || success {
		t.Fatalf("expected success false for bad socket, got %v", badResult)
	}
}

