package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/api"
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
)

func setupTestStore(t *testing.T) store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_stacks.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func setupStackRouter(st store.Store, ds *docker.Service) chi.Router {
	r := chi.NewRouter()
	r.Get("/api/stacks", api.HandleListStacks(st, ds))
	r.Post("/api/stacks", api.HandleCreateStack(st, ds))
	r.Get("/api/stacks/{name}", api.HandleGetStack(st, ds))
	r.Put("/api/stacks/{name}", api.HandleUpdateStack(st, ds))
	r.Post("/api/stacks/{name}/adopt", api.HandleAdoptStack(st, ds))
	r.Post("/api/stacks/{name}/stop", api.HandleStopStack(st, ds))
	r.Post("/api/stacks/{name}/down", api.HandleDownStack(st, ds))
	r.Delete("/api/stacks/{name}", api.HandleDeleteStack(st, ds))
	return r
}

func TestHandleListStacks_EmptyAndPopulated(t *testing.T) {
	st := setupTestStore(t)
	r := setupStackRouter(st, nil)

	// Test empty list
	req := httptest.NewRequest(http.MethodGet, "/api/stacks", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var emptyList []*store.Stack
	if err := json.Unmarshal(rec.Body.Bytes(), &emptyList); err != nil {
		t.Fatalf("failed to parse empty list response: %v", err)
	}
	if len(emptyList) != 0 {
		t.Fatalf("expected 0 stacks, got %d", len(emptyList))
	}

	// Save a stack
	_ = st.SaveStack(&store.Stack{
		Name:        "demo",
		ComposeYAML: "version: '3'\nservices:\n  web:\n    image: nginx",
		Status:      "running",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})

	req = httptest.NewRequest(http.MethodGet, "/api/stacks", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var populatedList []*store.Stack
	if err := json.Unmarshal(rec.Body.Bytes(), &populatedList); err != nil {
		t.Fatalf("failed to parse list response: %v", err)
	}
	if len(populatedList) != 1 {
		t.Fatalf("expected 1 stack, got %d", len(populatedList))
	}
	if populatedList[0].Name != "demo" {
		t.Errorf("expected stack name 'demo', got %q", populatedList[0].Name)
	}
}

func TestHandleGetStack(t *testing.T) {
	st := setupTestStore(t)
	r := setupStackRouter(st, nil)

	// Non-existent stack
	req := httptest.NewRequest(http.MethodGet, "/api/stacks/nonexistent", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", rec.Code)
	}

	// Existing stack
	_ = st.SaveStack(&store.Stack{
		Name:        "web-stack",
		ComposeYAML: "version: '3'\nservices:\n  app:\n    image: alpine",
		Status:      "running",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})

	req = httptest.NewRequest(http.MethodGet, "/api/stacks/web-stack", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var res store.Stack
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.Name != "web-stack" {
		t.Errorf("expected stack name 'web-stack', got %q", res.Name)
	}
}

func TestHandleCreateStack_ValidationAndCreation(t *testing.T) {
	st := setupTestStore(t)
	r := setupStackRouter(st, nil)

	// Missing name
	body, _ := json.Marshal(map[string]interface{}{
		"name":         "",
		"compose_yaml": "services:\n  app:\n    image: nginx",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/stacks", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty name, got %d", rec.Code)
	}

	// Missing yaml
	body, _ = json.Marshal(map[string]interface{}{
		"name":         "test-stack",
		"compose_yaml": "",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/stacks", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty compose yaml, got %d", rec.Code)
	}

	// Malformed yaml
	body, _ = json.Marshal(map[string]interface{}{
		"name":         "test-stack",
		"compose_yaml": "::: malformed yaml :::",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/stacks", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed yaml, got %d", rec.Code)
	}

	// Valid stack creation with deploy=false (since docker service is nil)
	deployFalse := false
	body, _ = json.Marshal(map[string]interface{}{
		"name":         "test-stack",
		"compose_yaml": "services:\n  app:\n    image: nginx:alpine",
		"deploy":       deployFalse,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/stacks", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var created store.Stack
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if created.Name != "test-stack" {
		t.Errorf("expected name 'test-stack', got %q", created.Name)
	}

	// Duplicate stack creation should conflict
	req = httptest.NewRequest(http.MethodPost, "/api/stacks", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for duplicate stack, got %d", rec.Code)
	}
}

func TestHandleUpdateStack(t *testing.T) {
	st := setupTestStore(t)
	r := setupStackRouter(st, nil)

	_ = st.SaveStack(&store.Stack{
		Name:        "update-test",
		ComposeYAML: "services:\n  app:\n    image: alpine",
		Status:      "stopped",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})

	// Non-existent
	body, _ := json.Marshal(map[string]interface{}{
		"compose_yaml": "services:\n  app:\n    image: nginx",
		"deploy":       false,
	})
	req := httptest.NewRequest(http.MethodPut, "/api/stacks/not-found", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", rec.Code)
	}

	// Invalid yaml
	body, _ = json.Marshal(map[string]interface{}{
		"compose_yaml": "invalid: yaml: [",
	})
	req = httptest.NewRequest(http.MethodPut, "/api/stacks/update-test", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", rec.Code)
	}

	// Valid update
	body, _ = json.Marshal(map[string]interface{}{
		"compose_yaml": "services:\n  app:\n    image: nginx:alpine",
		"deploy":       false,
	})
	req = httptest.NewRequest(http.MethodPut, "/api/stacks/update-test", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	updated, err := st.GetStack("update-test")
	if err != nil {
		t.Fatalf("failed to retrieve updated stack: %v", err)
	}
	if updated.ComposeYAML != "services:\n  app:\n    image: nginx:alpine" {
		t.Errorf("stack YAML was not updated in store")
	}
}

func TestHandleStopAndDownStack(t *testing.T) {
	st := setupTestStore(t)
	r := setupStackRouter(st, nil)

	_ = st.SaveStack(&store.Stack{
		Name:        "running-stack",
		ComposeYAML: "services:\n  app:\n    image: alpine",
		Status:      "running",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})

	// Stop non-existent
	req := httptest.NewRequest(http.MethodPost, "/api/stacks/missing/stop", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing stack stop, got %d", rec.Code)
	}

	// Stop existing
	req = httptest.NewRequest(http.MethodPost, "/api/stacks/running-stack/stop", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for stop, got %d", rec.Code)
	}

	stk, _ := st.GetStack("running-stack")
	if stk.Status != "stopped" {
		t.Errorf("expected status 'stopped', got %q", stk.Status)
	}

	// Down non-existent
	req = httptest.NewRequest(http.MethodPost, "/api/stacks/missing/down", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing stack down, got %d", rec.Code)
	}

	// Down existing
	req = httptest.NewRequest(http.MethodPost, "/api/stacks/running-stack/down", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for down, got %d", rec.Code)
	}

	stk, _ = st.GetStack("running-stack")
	if stk.Status != "down" {
		t.Errorf("expected status 'down', got %q", stk.Status)
	}
}

func TestHandleDeleteStack(t *testing.T) {
	st := setupTestStore(t)
	r := setupStackRouter(st, nil)

	// Delete non-existent
	req := httptest.NewRequest(http.MethodDelete, "/api/stacks/ghost", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for deleting non-existent stack, got %d", rec.Code)
	}

	// Delete existing
	_ = st.SaveStack(&store.Stack{
		Name:        "to-delete",
		ComposeYAML: "services:\n  app:\n    image: alpine",
		Status:      "stopped",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	})

	req = httptest.NewRequest(http.MethodDelete, "/api/stacks/to-delete", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for delete, got %d", rec.Code)
	}

	_, err := st.GetStack("to-delete")
	if err == nil {
		t.Errorf("expected error getting deleted stack, got nil")
	}
}

func TestHandleCreateStack_FailedDeployCleanup(t *testing.T) {
	st := setupTestStore(t)
	// Create docker service pointing to non-existent socket so DeployStack fails
	svc, err := docker.NewService("/tmp/nonexistent-docker-socket.sock")
	if err != nil {
		t.Fatalf("failed to create docker service: %v", err)
	}
	defer svc.Close()

	r := setupStackRouter(st, svc)

	body, _ := json.Marshal(map[string]interface{}{
		"name":         "failing-deploy",
		"compose_yaml": "services:\n  app:\n    image: alpine:latest\n",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/stacks", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error when deploy fails, got %d", rec.Code)
	}

	// Verify the stack was cleaned up from store
	_, err = st.GetStack("failing-deploy")
	if err == nil {
		t.Errorf("expected failed stack to be removed from store, but it was found")
	}

	// Verify re-trying create does not fail with 409 Conflict
	req2 := httptest.NewRequest(http.MethodPost, "/api/stacks", bytes.NewReader(body))
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)
	if rec2.Code == http.StatusConflict {
		t.Errorf("subsequent create got 409 Conflict; stack was not cleanly purged on failure")
	}
}

func TestHandleAdoptStack(t *testing.T) {
	st := setupTestStore(t)
	r := setupStackRouter(st, nil)

	// 1. Adopt with provided Compose YAML
	yaml := "services:\n  web:\n    image: nginx:alpine\n"
	body, _ := json.Marshal(map[string]interface{}{
		"compose_yaml": yaml,
		"working_dir":  "/data/projects/my-stack",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/stacks/my-ext-stack/adopt", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var adopted store.Stack
	if err := json.Unmarshal(rec.Body.Bytes(), &adopted); err != nil {
		t.Fatalf("failed to parse adopted stack: %v", err)
	}
	if adopted.Name != "my-ext-stack" {
		t.Errorf("expected stack name 'my-ext-stack', got %q", adopted.Name)
	}
	if adopted.IsExternal {
		t.Errorf("expected IsExternal to be false after adoption, got true")
	}
	if adopted.WorkingDir != "/data/projects/my-stack" {
		t.Errorf("expected working_dir '/data/projects/my-stack', got %q", adopted.WorkingDir)
	}

	// Verify persisted in store
	saved, err := st.GetStack("my-ext-stack")
	if err != nil {
		t.Fatalf("failed to get adopted stack from store: %v", err)
	}
	if saved.IsExternal {
		t.Errorf("expected saved stack IsExternal to be false, got true")
	}

	// 2. Adopt an already stored external stack without providing body
	extStack := &store.Stack{
		Name:        "stored-ext",
		ComposeYAML: "services:\n  db:\n    image: postgres:16-alpine\n",
		IsExternal:  true,
		WorkingDir:  "/var/compose/db",
		CreatedAt:   time.Now(),
	}
	if err := st.SaveStack(extStack); err != nil {
		t.Fatalf("failed to save external stack: %v", err)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/stacks/stored-ext/adopt", bytes.NewReader([]byte("{}")))
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec2.Code, rec2.Body.String())
	}

	saved2, err := st.GetStack("stored-ext")
	if err != nil {
		t.Fatalf("failed to get adopted stack from store: %v", err)
	}
	if saved2.IsExternal {
		t.Errorf("expected IsExternal to be false after adoption, got true")
	}
	if saved2.WorkingDir != "/var/compose/db" {
		t.Errorf("expected WorkingDir to be preserved, got %q", saved2.WorkingDir)
	}

	// 3. Reject invalid compose YAML
	badBody, _ := json.Marshal(map[string]interface{}{
		"compose_yaml": "not: valid: yaml: [",
	})
	reqBad := httptest.NewRequest(http.MethodPost, "/api/stacks/bad-stack/adopt", bytes.NewReader(badBody))
	recBad := httptest.NewRecorder()
	r.ServeHTTP(recBad, reqBad)

	if recBad.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid yaml, got %d", recBad.Code)
	}
}


