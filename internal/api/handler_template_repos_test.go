package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/podiax/pdswarm/internal/api"
	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/store"
	"github.com/podiax/pdswarm/internal/templates"
)

func TestHandlerTemplateRepos_CRUD(t *testing.T) {
	tempDir := t.TempDir()
	st, err := store.New(filepath.Join(tempDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer st.Close()

	cm := templates.NewCatalogManager()
	r := api.SetupRouter(&config.Config{NoAuth: true}, st, nil, nil)
	ts := httptest.NewServer(r)
	defer ts.Close()

	client := ts.Client()

	// 1. Initial settings
	resp, err := client.Get(ts.URL + "/api/templates/settings")
	if err != nil {
		t.Fatalf("failed to get settings: %v", err)
	}
	defer resp.Body.Close()

	var settings api.TemplateSettingsPayload
	if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
		t.Fatalf("failed to decode settings: %v", err)
	}
	if settings.DisableDefaultTemplates {
		t.Errorf("expected DisableDefaultTemplates=false initially")
	}

	// 2. Update settings: disable defaults
	settings.DisableDefaultTemplates = true
	body, _ := json.Marshal(settings)
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/templates/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("failed to put settings: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	// Verify templates list is empty now
	resp, err = client.Get(ts.URL + "/api/templates")
	if err != nil {
		t.Fatalf("failed to list templates: %v", err)
	}
	var tmplList api.ListTemplatesResponse
	_ = json.NewDecoder(resp.Body).Decode(&tmplList)
	resp.Body.Close()
	if tmplList.Total != 0 {
		t.Errorf("expected 0 templates when defaults disabled, got %d", tmplList.Total)
	}

	// 3. Create repo with invalid URL (offline test)
	enabled := true
	createReq := api.CreateTemplateRepoRequest{
		Name:    "Test Lissy93",
		URL:     "http://127.0.0.1:54321/offline-templates.json",
		Type:    "portainer",
		Enabled: &enabled,
	}
	body, _ = json.Marshal(createReq)
	resp, err = client.Post(ts.URL+"/api/templates/repos", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to post repo: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", resp.StatusCode)
	}

	var created store.TemplateRepo
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode created repo: %v", err)
	}
	if created.Name != "Test Lissy93" {
		t.Errorf("unexpected repo name %q", created.Name)
	}

	// 4. List repos
	resp, err = client.Get(ts.URL + "/api/templates/repos")
	if err != nil {
		t.Fatalf("failed to list repos: %v", err)
	}
	defer resp.Body.Close()
	var repos []*store.TemplateRepo
	_ = json.NewDecoder(resp.Body).Decode(&repos)
	if len(repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(repos))
	}

	// 5. Delete repo
	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/templates/repos/"+created.ID, nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("failed to delete repo: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	_ = cm
}
