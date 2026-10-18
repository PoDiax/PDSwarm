package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/podiax/pdswarm/internal/api"
	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
)

func setupRegistriesTestServer(t *testing.T) (http.Handler, store.Store, *docker.Service) {
	t.Helper()
	st := newTestStore(t)
	cfg := &config.Config{NoAuth: true}

	ds, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create docker service: %v", err)
	}
	t.Cleanup(func() { _ = ds.Close() })

	handler := api.SetupRouter(cfg, st, ds)
	return handler, st, ds
}

func TestRegistries_CRUD(t *testing.T) {
	handler, st, _ := setupRegistriesTestServer(t)

	// 1. List registries when empty
	req := httptest.NewRequest(http.MethodGet, "/api/registries", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	var list []api.RegistryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to parse list response: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected 0 registries initially, got %d", len(list))
	}

	// 2. Create validation errors
	invalidBodies := []map[string]string{
		{"name": "", "url": "docker.io", "username": "user", "password": "pass"},
		{"name": "Docker", "url": "", "username": "user", "password": "pass"},
		{"name": "Docker", "url": "docker.io", "username": "", "password": "pass"},
		{"name": "Docker", "url": "docker.io", "username": "user", "password": ""},
	}
	for i, body := range invalidBodies {
		b, _ := json.Marshal(body)
		r := httptest.NewRequest(http.MethodPost, "/api/registries", bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("case %d: expected 400 Bad Request, got %d: %s", i, w.Code, w.Body.String())
		}
	}

	// 3. Create valid registry
	createReq := api.CreateRegistryRequest{
		Name:     "GitHub Container Registry",
		URL:      "ghcr.io",
		Username: "octocat",
		Password: "ghp_secret_token_12345",
	}
	b, _ := json.Marshal(createReq)
	r := httptest.NewRequest(http.MethodPost, "/api/registries", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var created api.RegistryResponse
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode created registry: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected generated UUID ID, got empty")
	}
	if created.Name != "GitHub Container Registry" || created.URL != "ghcr.io" || created.Username != "octocat" {
		t.Errorf("unexpected created fields: %+v", created)
	}
	if created.Password != "" {
		t.Errorf("expected password to be masked (empty), got %q", created.Password)
	}
	if !created.HasPassword {
		t.Error("expected has_password to be true")
	}

	regID := created.ID

	// Verify database stored the actual password
	stored, err := st.GetRegistry(regID)
	if err != nil {
		t.Fatalf("GetRegistry from store failed: %v", err)
	}
	if stored.Password != "ghp_secret_token_12345" {
		t.Errorf("expected store to hold secret password, got %q", stored.Password)
	}

	// 4. Get registry by ID
	req = httptest.NewRequest(http.MethodGet, "/api/registries/"+regID, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	var got api.RegistryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("failed to decode registry: %v", err)
	}
	if got.ID != regID || got.Password != "" || !got.HasPassword {
		t.Errorf("unexpected got registry: %+v", got)
	}

	// Get 404
	req = httptest.NewRequest(http.MethodGet, "/api/registries/nonexistent-id", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent registry, got %d", rec.Code)
	}

	// 5. Update registry with empty password -> should preserve existing password!
	updateReq := api.UpdateRegistryRequest{
		Name:     "GitHub Container Registry (Renamed)",
		Password: "", // empty means preserve existing password
	}
	b, _ = json.Marshal(updateReq)
	r = httptest.NewRequest(http.MethodPut, "/api/registries/"+regID, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on update, got %d: %s", w.Code, w.Body.String())
	}
	var updated api.RegistryResponse
	_ = json.Unmarshal(w.Body.Bytes(), &updated)
	if updated.Name != "GitHub Container Registry (Renamed)" {
		t.Errorf("expected updated name, got %q", updated.Name)
	}

	storedAfterUpdate, _ := st.GetRegistry(regID)
	if storedAfterUpdate.Password != "ghp_secret_token_12345" {
		t.Fatalf("expected existing password preserved, got %q", storedAfterUpdate.Password)
	}

	// 6. Update registry with new password
	updatePassReq := api.UpdateRegistryRequest{
		Password: "ghp_brand_new_token_67890",
	}
	b, _ = json.Marshal(updatePassReq)
	r = httptest.NewRequest(http.MethodPut, "/api/registries/"+regID, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on password update, got %d: %s", w.Code, w.Body.String())
	}
	storedNewPass, _ := st.GetRegistry(regID)
	if storedNewPass.Password != "ghp_brand_new_token_67890" {
		t.Fatalf("expected updated password, got %q", storedNewPass.Password)
	}

	// 7. List registries shows 1 entry
	req = httptest.NewRequest(http.MethodGet, "/api/registries", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("expected 1 registry in list, got %d", len(list))
	}

	// 8. Delete registry
	req = httptest.NewRequest(http.MethodDelete, "/api/registries/"+regID, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on delete, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify deleted
	req = httptest.NewRequest(http.MethodGet, "/api/registries/"+regID, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", rec.Code)
	}

	// Delete again returns 404
	req = httptest.NewRequest(http.MethodDelete, "/api/registries/"+regID, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 on deleting nonexistent registry, got %d", rec.Code)
	}
}

func TestRegistries_PingAndTest(t *testing.T) {
	handler, st, _ := setupRegistriesTestServer(t)

	// Ping nonexistent ID returns 404
	req := httptest.NewRequest(http.MethodPost, "/api/registries/nonexistent-id/ping", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 on pinging nonexistent registry, got %d", rec.Code)
	}

	// Test endpoint validation
	testReq := map[string]string{
		"url":      "",
		"username": "",
	}
	b, _ := json.Marshal(testReq)
	r := httptest.NewRequest(http.MethodPost, "/api/registries/test", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty test req, got %d", w.Code)
	}

	// Save test registry
	reg := &store.Registry{
		ID:       "test-reg-1",
		Name:     "Docker Hub",
		URL:      "docker.io",
		Username: "invalid_test_user_xyz",
		Password: "invalid_password_123",
	}
	_ = st.SaveRegistry(reg)

	// Test ping on saved registry (will test auth against registry endpoint)
	req = httptest.NewRequest(http.MethodPost, "/api/registries/test-reg-1/ping", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	// Response should be a valid JSON with status field (either success or error based on network/auth)
	var pingResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &pingResp); err != nil {
		t.Fatalf("failed to decode ping response: %v, body=%s", err, rec.Body.String())
	}
	if pingResp["status"] == nil {
		t.Errorf("expected response to have status field: %+v", pingResp)
	}
}

func TestImages_PullWithRegistryAuth(t *testing.T) {
	handler, st, _ := setupRegistriesTestServer(t)

	// Pre-seed a private registry
	reg := &store.Registry{
		ID:       "reg-mycompany",
		Name:     "Internal GHCR",
		URL:      "ghcr.io",
		Username: "mycompany-robot",
		Password: "ghp_company_robot_token",
	}
	if err := st.SaveRegistry(reg); err != nil {
		t.Fatalf("failed to save registry: %v", err)
	}

	// 1. Empty image ref returns 400
	req := httptest.NewRequest(http.MethodPost, "/api/images/pull", bytes.NewReader([]byte(`{"image": ""}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for empty image, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Request with explicit registry_id
	req = httptest.NewRequest(http.MethodPost, "/api/images/pull?registry_id=reg-mycompany", bytes.NewReader([]byte(`{"image": "ghcr.io/mycompany/private-app:v1.0"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	// Either 200 or 500 (if credentials unauthorized against actual remote GHCR), but NOT 400
	if rec.Code == http.StatusBadRequest {
		t.Errorf("unexpected 400 Bad Request: %s", rec.Body.String())
	}
}
