package tests_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/podiax/pdswarm"
	"github.com/podiax/pdswarm/internal/api"
	"github.com/podiax/pdswarm/internal/auth"
	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
	"github.com/podiax/pdswarm/internal/templates"
)

// TestE2E_InProcessRouter runs full end-to-end integration tests on the assembled HTTP router.
func TestE2E_InProcessRouter(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pdswarm_e2e.db")

	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to initialize bbolt store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ds, err := docker.NewService("/var/run/docker.sock")
	if err != nil {
		t.Logf("Docker service initialized with warning: %v", err)
	}
	if ds != nil {
		t.Cleanup(func() { _ = ds.Close() })
	}

	distFS := pdswarm.WebFS()
	cfg := &config.Config{
		Host:      "127.0.0.1",
		Port:      9000,
		NoAuth:    true,
		DataDir:   tempDir,
		JWTSecret: "e2e-test-secret-key-12345",
	}

	router := api.SetupRouter(cfg, st, ds, distFS)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	client := ts.Client()

	// 1. Root / -> 200 OK with embedded HTML containing <!DOCTYPE html> and <div id="app">
	t.Run("GET / serves embedded SPA HTML", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/")
		if err != nil {
			t.Fatalf("GET / request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		bodyStr := string(body)

		if !strings.Contains(strings.ToLower(bodyStr), "<!doctype html") {
			t.Errorf("expected <!doctype html> in response, got: %s", bodyStr)
		}
		if !strings.Contains(bodyStr, `<div id="app">`) {
			t.Errorf("expected <div id=\"app\"> in response, got: %s", bodyStr)
		}
	})

	// 2. GET /assets/... -> 200 OK with embedded JS or CSS
	t.Run("GET /assets serves embedded static assets", func(t *testing.T) {
		entries, err := fs.ReadDir(distFS, "assets")
		if err != nil || len(entries) == 0 {
			t.Fatalf("failed to read assets from embedded distFS: %v (entries=%d)", err, len(entries))
		}

		tested := 0
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			assetPath := "/assets/" + entry.Name()
			resp, err := client.Get(ts.URL + assetPath)
			if err != nil {
				t.Fatalf("GET %s failed: %v", assetPath, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200 for %s, got %d", assetPath, resp.StatusCode)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed to read asset body %s: %v", assetPath, err)
			}
			if len(body) == 0 {
				t.Fatalf("asset %s was unexpectedly empty", assetPath)
			}

			cc := resp.Header.Get("Cache-Control")
			if !strings.Contains(cc, "max-age=31536000") || !strings.Contains(cc, "immutable") {
				t.Errorf("expected immutable cache-control for %s, got: %s", assetPath, cc)
			}

			ct := resp.Header.Get("Content-Type")
			if strings.HasSuffix(entry.Name(), ".js") && !strings.Contains(ct, "javascript") {
				t.Errorf("expected javascript content-type for %s, got: %s", assetPath, ct)
			}
			if strings.HasSuffix(entry.Name(), ".css") && !strings.Contains(ct, "text/css") {
				t.Errorf("expected text/css content-type for %s, got: %s", assetPath, ct)
			}
			tested++
		}

		if tested == 0 {
			t.Fatalf("no assets were tested")
		}
	})

	// 2b. GET /logo.svg and /favicon.svg -> 200 OK with SVG content type
	t.Run("GET /logo.svg and /favicon.svg serve SVG brand assets", func(t *testing.T) {
		for _, asset := range []string{"/logo.svg", "/favicon.svg", "/icon.svg"} {
			resp, err := client.Get(ts.URL + asset)
			if err != nil {
				t.Fatalf("GET %s failed: %v", asset, err)
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200 for %s, got %d", asset, resp.StatusCode)
			}
			if len(body) == 0 {
				t.Fatalf("expected non-empty body for %s", asset)
			}
		}
	})

	// 3. GET /api/system/ping -> 200 OK with {"status":"ok"}
	t.Run("GET /api/system/ping returns status ok", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/system/ping")
		if err != nil {
			t.Fatalf("GET /api/system/ping failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var payload struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode ping JSON: %v", err)
		}
		if payload.Status != "ok" {
			t.Errorf("expected status 'ok', got '%s'", payload.Status)
		}
	})

	// 4. GET /api/system/info -> 200 OK if Docker reachable, or 503 if unreachable
	t.Run("GET /api/system/info returns valid JSON", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/system/info")
		if err != nil {
			t.Fatalf("GET /api/system/info failed: %v", err)
		}
		defer resp.Body.Close()

		var raw map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
			t.Fatalf("expected valid JSON response from /api/system/info: %v", err)
		}

		if resp.StatusCode == http.StatusOK {
			if _, ok := raw["server_version"]; !ok {
				t.Logf("system/info payload keys: %+v", raw)
			}
		} else if resp.StatusCode == http.StatusServiceUnavailable {
			if _, ok := raw["error"]; !ok {
				t.Errorf("expected 'error' field on 503 response, got: %+v", raw)
			}
		} else {
			t.Fatalf("unexpected status code %d from /api/system/info", resp.StatusCode)
		}
	})

	// 5. GET /api/containers -> 200 OK with valid JSON array or 503
	t.Run("GET /api/containers returns valid JSON array or 503", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/containers")
		if err != nil {
			t.Fatalf("GET /api/containers failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			var containers []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
				t.Fatalf("expected JSON array response from /api/containers: %v", err)
			}
			t.Logf("retrieved %d containers", len(containers))
		} else if resp.StatusCode == http.StatusServiceUnavailable {
			var errResp map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
				t.Fatalf("failed to decode 503 error JSON: %v", err)
			}
		} else {
			t.Fatalf("unexpected status code %d from /api/containers", resp.StatusCode)
		}
	})

	// 6. GET /api/auth/status -> 200 OK
	t.Run("GET /api/auth/status returns 200 OK", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/auth/status")
		if err != nil {
			t.Fatalf("GET /api/auth/status failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var status api.AuthStatusResponse
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			t.Fatalf("failed to decode auth status JSON: %v", err)
		}

		if status.AuthEnabled {
			t.Errorf("expected AuthEnabled=false when NoAuth=true, got true")
		}
	})

	// 7. SPA client-side fallback: GET /containers and GET /stacks -> 200 OK with index.html
	t.Run("SPA fallback returns index.html for client-side routes", func(t *testing.T) {
		spaRoutes := []string{"/containers", "/stacks", "/images", "/volumes", "/networks", "/settings"}

		for _, route := range spaRoutes {
			resp, err := client.Get(ts.URL + route)
			if err != nil {
				t.Fatalf("GET %s failed: %v", route, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected status 200 for %s, got %d", route, resp.StatusCode)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed to read body for %s: %v", route, err)
			}
			bodyStr := string(body)

			if !strings.Contains(bodyStr, `<div id="app">`) {
				t.Errorf("expected SPA index.html containing <div id=\"app\"> for route %s", route)
			}
		}
	})

	// 8. API 404 behavior: GET /api/unknown-endpoint -> 404 JSON, NOT index.html
	t.Run("Unknown API routes return 404 JSON instead of index.html", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/unknown-endpoint-12345")
		if err != nil {
			t.Fatalf("GET /api/unknown-endpoint failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		bodyStr := string(body)

		if strings.Contains(bodyStr, `<!doctype html`) || strings.Contains(bodyStr, `<div id="app">`) {
			t.Errorf("expected 404 JSON, but received HTML index page: %s", bodyStr)
		}

		var errPayload map[string]interface{}
		if err := json.Unmarshal(body, &errPayload); err != nil {
			t.Fatalf("expected valid JSON error response, got: %s (err: %v)", bodyStr, err)
		}
		if _, ok := errPayload["error"]; !ok {
			t.Errorf("expected 'error' key in 404 JSON response, got: %+v", errPayload)
		}
	})

	// 9. Multi-Host Environments: GET /api/environments, GET /api/environments/{id}
	t.Run("GET /api/environments returns default local environment", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/environments")
		if err != nil {
			t.Fatalf("GET /api/environments failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var envs []api.EnvironmentResponse
		if err := json.NewDecoder(resp.Body).Decode(&envs); err != nil {
			t.Fatalf("failed to decode environments response: %v", err)
		}

		if len(envs) == 0 {
			t.Fatalf("expected at least 1 environment, got %d", len(envs))
		}

		var defaultEnv *api.EnvironmentResponse
		for i := range envs {
			if envs[i].ID == "local" || envs[i].IsDefault {
				defaultEnv = &envs[i]
				break
			}
		}

		if defaultEnv == nil {
			t.Fatalf("expected default 'local' environment, found none in %+v", envs)
		}

		if defaultEnv.ID != "local" {
			t.Errorf("expected default env ID 'local', got '%s'", defaultEnv.ID)
		}
		if defaultEnv.Type != "local" {
			t.Errorf("expected default env type 'local', got '%s'", defaultEnv.Type)
		}

		// GET /api/environments/{id}
		detailResp, err := client.Get(ts.URL + "/api/environments/" + defaultEnv.ID)
		if err != nil {
			t.Fatalf("GET /api/environments/%s failed: %v", defaultEnv.ID, err)
		}
		defer detailResp.Body.Close()

		if detailResp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", detailResp.StatusCode)
		}

		var env api.EnvironmentResponse
		if err := json.NewDecoder(detailResp.Body).Decode(&env); err != nil {
			t.Fatalf("failed to decode single environment response: %v", err)
		}
		if env.ID != defaultEnv.ID {
			t.Errorf("expected env ID '%s', got '%s'", defaultEnv.ID, env.ID)
		}
	})

	// 10. App Templates Catalog API
	t.Run("App Templates Catalog API returns curated templates and categories", func(t *testing.T) {
		// GET /api/templates
		resp, err := client.Get(ts.URL + "/api/templates")
		if err != nil {
			t.Fatalf("GET /api/templates failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var tmplResp api.ListTemplatesResponse
		if err := json.NewDecoder(resp.Body).Decode(&tmplResp); err != nil {
			t.Fatalf("failed to decode templates response: %v", err)
		}

		if tmplResp.Total != 35 || len(tmplResp.Templates) != 35 {
			t.Errorf("expected 35 curated templates, got total=%d, list=%d", tmplResp.Total, len(tmplResp.Templates))
		}

		// GET /api/templates/categories
		catResp, err := client.Get(ts.URL + "/api/templates/categories")
		if err != nil {
			t.Fatalf("GET /api/templates/categories failed: %v", err)
		}
		defer catResp.Body.Close()

		if catResp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", catResp.StatusCode)
		}

		var catList api.ListCategoriesResponse
		if err := json.NewDecoder(catResp.Body).Decode(&catList); err != nil {
			t.Fatalf("failed to decode categories response: %v", err)
		}

		if len(catList.Categories) == 0 {
			t.Errorf("expected non-empty categories list")
		}

		// GET /api/templates/{id}
		detailResp, err := client.Get(ts.URL + "/api/templates/postgres")
		if err != nil {
			t.Fatalf("GET /api/templates/postgres failed: %v", err)
		}
		defer detailResp.Body.Close()

		if detailResp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 for /api/templates/postgres, got %d", detailResp.StatusCode)
		}

		var tmpl templates.Template
		if err := json.NewDecoder(detailResp.Body).Decode(&tmpl); err != nil {
			t.Fatalf("failed to decode postgres template: %v", err)
		}

		if tmpl.ID != "postgres" {
			t.Errorf("expected template ID 'postgres', got '%s'", tmpl.ID)
		}
		if tmpl.ComposeYAML == "" {
			t.Errorf("expected template ComposeYAML to be populated")
		}
		if len(tmpl.Env) == 0 {
			t.Errorf("expected postgres template to define environment variables")
		}
	})

	// 10b. App Template Repos & Settings API: Custom Catalogs & Default Toggle
	t.Run("App Template Repos and Settings API manages custom feeds and default templates toggle", func(t *testing.T) {
		// 1. GET /api/templates/settings -> disable_default_templates: false
		resp, err := client.Get(ts.URL + "/api/templates/settings")
		if err != nil {
			t.Fatalf("GET /api/templates/settings failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var settings api.TemplateSettingsPayload
		if err := json.NewDecoder(resp.Body).Decode(&settings); err != nil {
			t.Fatalf("failed to decode settings: %v", err)
		}
		if settings.DisableDefaultTemplates {
			t.Errorf("expected DisableDefaultTemplates=false initially")
		}

		// 2. PUT /api/templates/settings -> disable_default_templates: true
		putBody, _ := json.Marshal(map[string]bool{"disable_default_templates": true})
		req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/templates/settings", bytes.NewReader(putBody))
		req.Header.Set("Content-Type", "application/json")
		putResp, err := client.Do(req)
		if err != nil {
			t.Fatalf("PUT /api/templates/settings failed: %v", err)
		}
		defer putResp.Body.Close()
		if putResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 on settings update, got %d", putResp.StatusCode)
		}

		// Verify /api/templates returns 0 templates now
		tmplResp, err := client.Get(ts.URL + "/api/templates")
		if err != nil {
			t.Fatalf("GET /api/templates failed: %v", err)
		}
		defer tmplResp.Body.Close()
		var listResp api.ListTemplatesResponse
		_ = json.NewDecoder(tmplResp.Body).Decode(&listResp)
		if listResp.Total != 0 {
			t.Errorf("expected 0 templates when defaults disabled and no custom repos, got %d", listResp.Total)
		}

		// 3. Create a custom template repository via POST /api/templates/repos
		repoPayload, _ := json.Marshal(map[string]any{
			"name":    "E2E Test Repo",
			"url":     "https://raw.githubusercontent.com/Lissy93/portainer-templates/main/templates.json",
			"type":    "portainer",
			"enabled": true,
		})
		repoResp, err := client.Post(ts.URL+"/api/templates/repos", "application/json", bytes.NewReader(repoPayload))
		if err != nil {
			t.Fatalf("POST /api/templates/repos failed: %v", err)
		}
		defer repoResp.Body.Close()
		if repoResp.StatusCode != http.StatusCreated {
			t.Fatalf("expected 201 on repo creation, got %d", repoResp.StatusCode)
		}
		var createdRepo store.TemplateRepo
		if err := json.NewDecoder(repoResp.Body).Decode(&createdRepo); err != nil {
			t.Fatalf("failed to decode created repo: %v", err)
		}
		if createdRepo.ID == "" {
			t.Errorf("expected non-empty repo ID")
		}

		// 4. GET /api/templates/repos
		listRepoResp, err := client.Get(ts.URL + "/api/templates/repos")
		if err != nil {
			t.Fatalf("GET /api/templates/repos failed: %v", err)
		}
		defer listRepoResp.Body.Close()
		var repos []store.TemplateRepo
		_ = json.NewDecoder(listRepoResp.Body).Decode(&repos)
		if len(repos) != 1 {
			t.Errorf("expected 1 repo, got %d", len(repos))
		}

		// 5. DELETE /api/templates/repos/{id}
		delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/templates/repos/"+createdRepo.ID, nil)
		delResp, err := client.Do(delReq)
		if err != nil {
			t.Fatalf("DELETE /api/templates/repos/%s failed: %v", createdRepo.ID, err)
		}
		defer delResp.Body.Close()
		if delResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 on repo deletion, got %d", delResp.StatusCode)
		}

		// 6. Restore default templates: PUT /api/templates/settings -> disable_default_templates: false
		restoreBody, _ := json.Marshal(map[string]bool{"disable_default_templates": false})
		restReq, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/templates/settings", bytes.NewReader(restoreBody))
		restReq.Header.Set("Content-Type", "application/json")
		restResp, err := client.Do(restReq)
		if err != nil {
			t.Fatalf("PUT /api/templates/settings restore failed: %v", err)
		}
		defer restResp.Body.Close()
		if restResp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 on settings restore, got %d", restResp.StatusCode)
		}

		// Verify 35 curated templates are restored
		finalTmplResp, err := client.Get(ts.URL + "/api/templates")
		if err != nil {
			t.Fatalf("GET /api/templates failed: %v", err)
		}
		defer finalTmplResp.Body.Close()
		var finalResp api.ListTemplatesResponse
		_ = json.NewDecoder(finalTmplResp.Body).Decode(&finalResp)
		if finalResp.Total != 35 {
			t.Errorf("expected 35 curated templates after restore, got %d", finalResp.Total)
		}
	})

	// 11. Private Registries API
	t.Run("Private Registries API creates, lists with masked password, and deletes", func(t *testing.T) {
		regPayload := map[string]string{
			"name":     "E2E Test Registry",
			"url":      "ghcr.io",
			"username": "e2e-user",
			"password": "e2e-password",
		}
		jsonBytes, _ := json.Marshal(regPayload)

		// POST /api/registries
		createResp, err := client.Post(ts.URL+"/api/registries", "application/json", bytes.NewReader(jsonBytes))
		if err != nil {
			t.Fatalf("POST /api/registries failed: %v", err)
		}
		defer createResp.Body.Close()

		if createResp.StatusCode != http.StatusCreated {
			t.Fatalf("expected status 201 Created, got %d", createResp.StatusCode)
		}

		var created api.RegistryResponse
		if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
			t.Fatalf("failed to decode create registry response: %v", err)
		}

		if created.ID == "" {
			t.Errorf("expected non-empty registry ID")
		}
		if created.Name != "E2E Test Registry" {
			t.Errorf("expected name 'E2E Test Registry', got '%s'", created.Name)
		}
		if created.Password != "" {
			t.Errorf("expected masked password in create response, got '%s'", created.Password)
		}
		if !created.HasPassword {
			t.Errorf("expected has_password to be true")
		}

		// GET /api/registries
		listResp, err := client.Get(ts.URL + "/api/registries")
		if err != nil {
			t.Fatalf("GET /api/registries failed: %v", err)
		}
		defer listResp.Body.Close()

		if listResp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", listResp.StatusCode)
		}

		var registries []api.RegistryResponse
		if err := json.NewDecoder(listResp.Body).Decode(&registries); err != nil {
			t.Fatalf("failed to decode registries list: %v", err)
		}

		found := false
		for _, reg := range registries {
			if reg.ID == created.ID {
				found = true
				if reg.Password != "" {
					t.Errorf("expected masked password in list response, got '%s'", reg.Password)
				}
				if !reg.HasPassword {
					t.Errorf("expected has_password=true in list response")
				}
				break
			}
		}
		if !found {
			t.Fatalf("created registry %s not found in list %+v", created.ID, registries)
		}

		// DELETE /api/registries/{id}
		delReq, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/registries/"+created.ID, nil)
		if err != nil {
			t.Fatalf("failed to create delete request: %v", err)
		}
		delResp, err := client.Do(delReq)
		if err != nil {
			t.Fatalf("DELETE /api/registries/%s failed: %v", created.ID, err)
		}
		defer delResp.Body.Close()

		if delResp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 on registry delete, got %d", delResp.StatusCode)
		}

		// Verify deletion
		getResp, err := client.Get(ts.URL + "/api/registries/" + created.ID)
		if err != nil {
			t.Fatalf("GET deleted registry failed: %v", err)
		}
		defer getResp.Body.Close()
		if getResp.StatusCode != http.StatusNotFound {
			t.Errorf("expected status 404 for deleted registry, got %d", getResp.StatusCode)
		}
	})

	// 12. Container Files API Routing
	t.Run("Container Files API routing correctly targets file handler", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/containers/nonexistent-container-xyz/files/list")
		if err != nil {
			t.Fatalf("GET /api/containers/.../files/list failed: %v", err)
		}
		defer resp.Body.Close()

		// If Docker is unavailable -> 503, if Docker is available but container not found -> 404
		if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("expected status 404 or 503 from file handler, got %d", resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		bodyStr := string(body)

		if strings.Contains(bodyStr, `<!doctype html`) || strings.Contains(bodyStr, `<div id="app">`) {
			t.Errorf("container files request fell back to HTML SPA: %s", bodyStr)
		}

		var errPayload map[string]interface{}
		if err := json.Unmarshal(body, &errPayload); err != nil {
			t.Fatalf("expected valid JSON error response, got: %s (err: %v)", bodyStr, err)
		}
		if _, ok := errPayload["error"]; !ok {
			t.Errorf("expected 'error' field in JSON response, got %+v", errPayload)
		}
	})

	// 13. System Ports & Conflict Monitoring API
	t.Run("System Ports API returns allocated ports and checks conflicts", func(t *testing.T) {
		// GET /api/system/ports
		resp, err := client.Get(ts.URL + "/api/system/ports")
		if err != nil {
			t.Fatalf("GET /api/system/ports failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200, got %d", resp.StatusCode)
		}

		var portsResp api.PortsListResponse
		if err := json.NewDecoder(resp.Body).Decode(&portsResp); err != nil {
			t.Fatalf("failed to decode allocated ports response: %v", err)
		}

		// POST /api/system/ports/check
		checkReq := api.CheckPortsRequest{
			Ports: []string{"80:80", "59123:80"},
		}
		checkBody, _ := json.Marshal(checkReq)
		cResp, err := client.Post(ts.URL+"/api/system/ports/check", "application/json", bytes.NewReader(checkBody))
		if err != nil {
			t.Fatalf("POST /api/system/ports/check failed: %v", err)
		}
		defer cResp.Body.Close()

		if cResp.StatusCode != http.StatusOK {
			t.Fatalf("expected status 200 from ports check, got %d", cResp.StatusCode)
		}

		var checkRes docker.CheckPortsResult
		if err := json.NewDecoder(cResp.Body).Decode(&checkRes); err != nil {
			t.Fatalf("failed to decode check ports response: %v", err)
		}
	})
}


// TestE2E_APIAccessTokens_BearerAuth tests the complete API token lifecycle with Bearer authentication.
func TestE2E_APIAccessTokens_BearerAuth(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "pdswarm_tokens_e2e.db")

	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to initialize bbolt store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	// Seed admin user
	adminPassword := "AdminSecretPass123!"
	passHash, err := auth.HashPassword(adminPassword)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	adminUser := &store.User{
		Username:     "e2e-admin",
		PasswordHash: passHash,
		Role:         "admin",
		CreatedAt:    time.Now(),
	}
	if err := st.SaveUser(adminUser); err != nil {
		t.Fatalf("failed to seed admin user: %v", err)
	}

	cfg := &config.Config{
		Host:      "127.0.0.1",
		Port:      9002,
		NoAuth:    false, // Enforce authentication!
		DataDir:   tempDir,
		JWTSecret: "e2e-token-jwt-secret-key-32bytes!",
	}

	router := api.SetupRouter(cfg, st, nil)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)

	client := ts.Client()

	// 1. Unauthenticated request to /api/tokens must fail with 401
	unauthResp, err := client.Get(ts.URL + "/api/tokens")
	if err != nil {
		t.Fatalf("unauthenticated GET /api/tokens failed: %v", err)
	}
	unauthResp.Body.Close()
	if unauthResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d", unauthResp.StatusCode)
	}

	// 2. Login via POST /api/auth/login to obtain JWT
	loginBody := map[string]string{
		"username": "e2e-admin",
		"password": adminPassword,
	}
	loginBytes, _ := json.Marshal(loginBody)
	loginResp, err := client.Post(ts.URL+"/api/auth/login", "application/json", bytes.NewReader(loginBytes))
	if err != nil {
		t.Fatalf("POST /api/auth/login failed: %v", err)
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected login status 200, got %d", loginResp.StatusCode)
	}

	var authResp api.AuthResponse
	if err := json.NewDecoder(loginResp.Body).Decode(&authResp); err != nil {
		t.Fatalf("failed to decode auth login response: %v", err)
	}
	if authResp.Token == "" {
		t.Fatalf("expected non-empty JWT token from login")
	}
	jwtToken := authResp.Token

	// 3. POST /api/tokens using JWT auth header
	createTokenBody := `{"name":"E2E CI Token","expires_days":30}`
	createReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/tokens", strings.NewReader(createTokenBody))
	if err != nil {
		t.Fatalf("failed to create token request: %v", err)
	}
	createReq.Header.Set("Authorization", "Bearer "+jwtToken)
	createReq.Header.Set("Content-Type", "application/json")

	createResp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("POST /api/tokens failed: %v", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201 Created for token creation, got %d", createResp.StatusCode)
	}

	var tokenCreated api.CreateTokenResponse
	if err := json.NewDecoder(createResp.Body).Decode(&tokenCreated); err != nil {
		t.Fatalf("failed to decode create token response: %v", err)
	}

	if !strings.HasPrefix(tokenCreated.Token, "pds_") {
		t.Errorf("expected token to start with 'pds_', got '%s'", tokenCreated.Token)
	}
	if tokenCreated.TokenInfo.Name != "E2E CI Token" {
		t.Errorf("expected token name 'E2E CI Token', got '%s'", tokenCreated.TokenInfo.Name)
	}
	if tokenCreated.TokenInfo.CreatedBy != "e2e-admin" {
		t.Errorf("expected created_by 'e2e-admin', got '%s'", tokenCreated.TokenInfo.CreatedBy)
	}
	rawAPIToken := tokenCreated.Token
	tokenID := tokenCreated.TokenInfo.ID

	// 4. Authenticate against protected API endpoint (/api/tokens) using the generated Bearer API token
	bearerReq, err := http.NewRequest(http.MethodGet, ts.URL+"/api/tokens", nil)
	if err != nil {
		t.Fatalf("failed to create bearer request: %v", err)
	}
	bearerReq.Header.Set("Authorization", "Bearer "+rawAPIToken)

	bearerResp, err := client.Do(bearerReq)
	if err != nil {
		t.Fatalf("GET /api/tokens with Bearer API token failed: %v", err)
	}
	defer bearerResp.Body.Close()

	if bearerResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 OK with Bearer API token auth, got %d", bearerResp.StatusCode)
	}

	var tokenList []api.TokenResponse
	if err := json.NewDecoder(bearerResp.Body).Decode(&tokenList); err != nil {
		t.Fatalf("failed to decode token list: %v", err)
	}
	if len(tokenList) != 1 || tokenList[0].ID != tokenID {
		t.Fatalf("expected 1 token with ID %s, got %+v", tokenID, tokenList)
	}

	// Also verify Bearer token works against /api/environments
	envReq, err := http.NewRequest(http.MethodGet, ts.URL+"/api/environments", nil)
	if err != nil {
		t.Fatalf("failed to create env request: %v", err)
	}
	envReq.Header.Set("Authorization", "Bearer "+rawAPIToken)
	envResp, err := client.Do(envReq)
	if err != nil {
		t.Fatalf("GET /api/environments with Bearer API token failed: %v", err)
	}
	defer envResp.Body.Close()
	if envResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 on /api/environments with Bearer token, got %d", envResp.StatusCode)
	}

	// 5. Delete token via DELETE /api/tokens/{id}
	delReq, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/tokens/"+tokenID, nil)
	if err != nil {
		t.Fatalf("failed to create delete token request: %v", err)
	}
	delReq.Header.Set("Authorization", "Bearer "+jwtToken)
	delResp, err := client.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE /api/tokens/%s failed: %v", tokenID, err)
	}
	defer delResp.Body.Close()

	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200 on token delete, got %d", delResp.StatusCode)
	}

	// 6. Verify that revoked Bearer token is now rejected with 401
	revokedReq, err := http.NewRequest(http.MethodGet, ts.URL+"/api/tokens", nil)
	if err != nil {
		t.Fatalf("failed to create revoked token request: %v", err)
	}
	revokedReq.Header.Set("Authorization", "Bearer "+rawAPIToken)
	revokedResp, err := client.Do(revokedReq)
	if err != nil {
		t.Fatalf("GET /api/tokens with revoked token failed: %v", err)
	}
	defer revokedResp.Body.Close()

	if revokedResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401 Unauthorized for revoked API token, got %d", revokedResp.StatusCode)
	}
}

// TestE2E_CompiledBinary builds and runs the standalone pdswarm executable,
// verifying end-to-end operation through the compiled binary.
func TestE2E_CompiledBinary(t *testing.T) {
	tempDir := t.TempDir()
	binPath := filepath.Join(tempDir, "pdswarm_e2e_bin")

	// Compile pdswarm executable
	buildCmd := exec.Command("go", "build", "-o", binPath, "../cmd/pdswarm")
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to compile pdswarm: %v, output: %s", err, string(out))
	}

	// Find free port
	port, err := getFreePort()
	if err != nil {
		t.Fatalf("failed to find free TCP port: %v", err)
	}

	dataDir := filepath.Join(tempDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	runCmd := exec.Command(binPath,
		"-host", "127.0.0.1",
		"-port", strconv.Itoa(port),
		"-data-dir", dataDir,
		"-no-auth",
	)
	// Create new process group for clean termination
	runCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr strings.Builder
	runCmd.Stdout = &stdout
	runCmd.Stderr = &stderr

	if err := runCmd.Start(); err != nil {
		t.Fatalf("failed to start pdswarm binary: %v", err)
	}

	// Ensure cleanup on test exit
	t.Cleanup(func() {
		if runCmd.Process != nil {
			_ = syscall.Kill(-runCmd.Process.Pid, syscall.SIGKILL)
			_ = runCmd.Wait()
		}
	})

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := &http.Client{Timeout: 3 * time.Second}

	// Wait for server to become ready
	readyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ready := false
	for readyCtx.Err() == nil {
		resp, err := client.Get(baseURL + "/api/system/ping")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !ready {
		t.Fatalf("server failed to start within timeout. stdout:\n%s\nstderr:\n%s", stdout.String(), stderr.String())
	}

	// 1. GET / -> verify embedded SPA HTML returns 200 OK
	resp, err := client.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("binary GET / failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("binary GET / returned status %d, expected 200", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if !strings.Contains(string(body), `<div id="app">`) {
		t.Errorf("expected <div id=\"app\"> in binary GET / response")
	}

	// 2. GET /api/system/info -> verify 200 OK (or 503 if Docker daemon is unavailable)
	infoResp, err := client.Get(baseURL + "/api/system/info")
	if err != nil {
		t.Fatalf("binary GET /api/system/info failed: %v", err)
	}
	defer infoResp.Body.Close()

	if infoResp.StatusCode != http.StatusOK && infoResp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("binary GET /api/system/info returned unexpected status %d", infoResp.StatusCode)
	}

	// 3. GET /api/containers -> verify container list returns valid JSON array
	containersResp, err := client.Get(baseURL + "/api/containers")
	if err != nil {
		t.Fatalf("binary GET /api/containers failed: %v", err)
	}
	defer containersResp.Body.Close()

	if containersResp.StatusCode == http.StatusOK {
		var containers []map[string]interface{}
		if err := json.NewDecoder(containersResp.Body).Decode(&containers); err != nil {
			t.Errorf("failed to decode container list JSON: %v", err)
		}
	} else if containersResp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("binary GET /api/containers returned unexpected status %d", containersResp.StatusCode)
	}

	// 4. Graceful shutdown verification: send SIGTERM and verify process exits
	if err := syscall.Kill(-runCmd.Process.Pid, syscall.SIGTERM); err != nil {
		t.Logf("error sending SIGTERM: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- runCmd.Wait()
	}()

	select {
	case <-time.After(5 * time.Second):
		t.Errorf("pdswarm did not exit within 5s of SIGTERM")
		_ = syscall.Kill(-runCmd.Process.Pid, syscall.SIGKILL)
	case err := <-done:
		t.Logf("pdswarm process exited cleanly: %v", err)
	}
}

func getFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
