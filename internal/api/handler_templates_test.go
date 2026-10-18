package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/api"
	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
	"github.com/podiax/pdswarm/internal/templates"
)

func setupTestTemplatesRouter(st store.Store, ds *docker.Service) (chi.Router, *templates.CatalogManager) {
	cm := templates.NewCatalogManager()
	r := chi.NewRouter()

	r.Get("/api/templates", api.HandleListTemplates(cm))
	r.Get("/api/templates/categories", api.HandleListTemplateCategories(cm))
	r.Get("/api/templates/{id}", api.HandleGetTemplate(cm))
	r.Post("/api/templates/deploy", api.HandleDeployTemplate(cm, st, ds))

	return r, cm
}

func TestHandleListTemplates(t *testing.T) {
	st := setupTestStore(t)
	r, _ := setupTestTemplatesRouter(st, nil)

	// 1. List all templates
	req := httptest.NewRequest(http.MethodGet, "/api/templates", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp api.ListTemplatesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total < 30 {
		t.Errorf("expected at least 30 templates, got %d", resp.Total)
	}
	if len(resp.Categories) == 0 {
		t.Errorf("expected categories to be populated")
	}
	if len(resp.Sources) == 0 {
		t.Errorf("expected sources to be populated")
	}
	if resp.CategoryCounts["Database"] == 0 {
		t.Errorf("expected Database category count > 0")
	}

	// 2. Query filter ?q=postgres
	req = httptest.NewRequest(http.MethodGet, "/api/templates?q=postgres", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var qResp api.ListTemplatesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &qResp); err != nil {
		t.Fatalf("failed to decode search response: %v", err)
	}
	if qResp.Total == 0 {
		t.Fatalf("expected at least 1 template matching 'postgres'")
	}

	// 3. Category filter ?category=Database
	req = httptest.NewRequest(http.MethodGet, "/api/templates?category=Database", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var catResp api.ListTemplatesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &catResp); err != nil {
		t.Fatalf("failed to decode category response: %v", err)
	}
	if catResp.Total == 0 {
		t.Fatalf("expected templates in Database category")
	}
	for _, tmpl := range catResp.Templates {
		matched := false
		for _, c := range tmpl.Categories {
			if c == "Database" {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("template %s returned without Database category: %v", tmpl.ID, tmpl.Categories)
		}
	}

	// 4. Non-matching filter
	req = httptest.NewRequest(http.MethodGet, "/api/templates?category=NonExistentCategory", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var emptyResp api.ListTemplatesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &emptyResp); err != nil {
		t.Fatalf("failed to decode empty response: %v", err)
	}
	if emptyResp.Total != 0 {
		t.Errorf("expected 0 templates for non-existent category, got %d", emptyResp.Total)
	}
}

func TestHandleListTemplateCategories(t *testing.T) {
	st := setupTestStore(t)
	r, _ := setupTestTemplatesRouter(st, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/templates/categories", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var resp api.ListCategoriesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse categories response: %v", err)
	}

	if len(resp.Categories) == 0 {
		t.Fatalf("expected non-empty categories list")
	}
	if resp.Counts["Database"] == 0 {
		t.Errorf("expected counts for Database category")
	}
}

func TestHandleGetTemplate(t *testing.T) {
	st := setupTestStore(t)
	r, _ := setupTestTemplatesRouter(st, nil)

	// Existing template
	req := httptest.NewRequest(http.MethodGet, "/api/templates/postgres", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var tmpl templates.Template
	if err := json.Unmarshal(rec.Body.Bytes(), &tmpl); err != nil {
		t.Fatalf("failed to decode template: %v", err)
	}
	if tmpl.ID != "postgres" || tmpl.Title != "PostgreSQL 16" {
		t.Errorf("unexpected template data: %+v", tmpl)
	}
	if tmpl.ComposeYAML == "" {
		t.Errorf("expected non-empty ComposeYAML")
	}

	// Non-existent template
	req = httptest.NewRequest(http.MethodGet, "/api/templates/unknown-template-xyz", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", rec.Code)
	}
}

func TestHandleDeployTemplate(t *testing.T) {
	st := setupTestStore(t)
	r, _ := setupTestTemplatesRouter(st, nil)

	// 1. Success deployment
	payload := templates.DeployTemplateRequest{
		TemplateID: "postgres",
		StackName:  "my-postgres-app",
		Env: map[string]string{
			"POSTGRES_DB":       "my_database",
			"POSTGRES_PASSWORD": "custom_password",
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/templates/deploy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp api.DeployTemplateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode deploy response: %v", err)
	}
	if resp.Status != "deployed" {
		t.Errorf("expected status 'deployed', got %q", resp.Status)
	}
	if resp.StackName != "my-postgres-app" {
		t.Errorf("expected stack_name 'my-postgres-app', got %q", resp.StackName)
	}
	if resp.Stack == nil || resp.Stack.Name != "my-postgres-app" {
		t.Errorf("expected stack object returned in response")
	}

	// Verify stack saved in store
	saved, err := st.GetStack("my-postgres-app")
	if err != nil {
		t.Fatalf("failed to retrieve stack from store: %v", err)
	}
	if saved.Name != "my-postgres-app" {
		t.Errorf("expected stack name 'my-postgres-app', got %q", saved.Name)
	}

	// 2. Conflict on duplicate stack name
	req = httptest.NewRequest(http.MethodPost, "/api/templates/deploy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict, got %d", rec.Code)
	}

	// 3. Missing template_id
	badReqBody, _ := json.Marshal(templates.DeployTemplateRequest{
		StackName: "some-stack",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/templates/deploy", bytes.NewReader(badReqBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing template_id, got %d", rec.Code)
	}

	// 4. Missing stack_name
	badReqBody2, _ := json.Marshal(templates.DeployTemplateRequest{
		TemplateID: "postgres",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/templates/deploy", bytes.NewReader(badReqBody2))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing stack_name, got %d", rec.Code)
	}

	// 5. Template not found
	unknownTmplBody, _ := json.Marshal(templates.DeployTemplateRequest{
		TemplateID: "unknown-tmpl-abc",
		StackName:  "stack-abc",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/templates/deploy", bytes.NewReader(unknownTmplBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found, got %d", rec.Code)
	}

	// 6. Invalid JSON body
	req = httptest.NewRequest(http.MethodPost, "/api/templates/deploy", bytes.NewReader([]byte("{invalid-json")))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid JSON, got %d", rec.Code)
	}
}

func TestRouterIntegration_TemplatesRoutes(t *testing.T) {
	st := setupTestStore(t)
	cfg := &config.Config{
		NoAuth: true,
	}
	router := api.SetupRouter(cfg, st, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/templates", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from full router /api/templates, got %d", rec.Code)
	}

	var resp api.ListTemplatesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode router response: %v", err)
	}
	if resp.Total < 30 {
		t.Errorf("expected at least 30 templates via router, got %d", resp.Total)
	}
}
