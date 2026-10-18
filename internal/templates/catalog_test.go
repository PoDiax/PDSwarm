package templates

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/podiax/pdswarm/internal/store"
)

func TestCuratedTemplatesCount(t *testing.T) {
	curated := GetCuratedTemplates()
	if len(curated) < 30 {
		t.Fatalf("expected at least 30 curated templates, got %d", len(curated))
	}

	cm := NewCatalogManager()
	all := cm.ListTemplates("", "", "")
	if len(all) < 30 {
		t.Fatalf("expected at least 30 templates in catalog manager, got %d", len(all))
	}

	// Verify required fields in all curated templates
	for _, tmpl := range curated {
		if tmpl.ID == "" {
			t.Errorf("template missing ID: %+v", tmpl)
		}
		if tmpl.Title == "" {
			t.Errorf("template %s missing Title", tmpl.ID)
		}
		if tmpl.Description == "" {
			t.Errorf("template %s missing Description", tmpl.ID)
		}
		if len(tmpl.Categories) == 0 {
			t.Errorf("template %s has no categories", tmpl.ID)
		}
		if tmpl.ComposeYAML == "" {
			t.Errorf("template %s has empty ComposeYAML", tmpl.ID)
		}
		if tmpl.Source != "curated" {
			t.Errorf("template %s has source %q, expected 'curated'", tmpl.ID, tmpl.Source)
		}
	}
}

func TestCatalogManager_ListAndFilter(t *testing.T) {
	cm := NewCatalogManager()

	// 1. Filter by query
	pgList := cm.ListTemplates("postgres", "", "")
	if len(pgList) == 0 {
		t.Fatalf("expected at least 1 template matching 'postgres'")
	}
	foundPg := false
	for _, tpl := range pgList {
		if tpl.ID == "postgres" {
			foundPg = true
			break
		}
	}
	if !foundPg {
		t.Errorf("expected 'postgres' in search results")
	}

	// 2. Filter by category
	dbList := cm.ListTemplates("", "Database", "")
	if len(dbList) == 0 {
		t.Fatalf("expected templates in 'Database' category")
	}
	for _, tpl := range dbList {
		hasCat := false
		for _, cat := range tpl.Categories {
			if cat == "Database" {
				hasCat = true
				break
			}
		}
		if !hasCat {
			t.Errorf("template %s does not contain 'Database' category", tpl.ID)
		}
	}

	// 3. Filter by source
	curatedList := cm.ListTemplates("", "", "curated")
	if len(curatedList) == 0 {
		t.Fatalf("expected templates with source 'curated'")
	}
	nonExistentSource := cm.ListTemplates("", "", "non-existent")
	if len(nonExistentSource) != 0 {
		t.Errorf("expected 0 templates for non-existent source, got %d", len(nonExistentSource))
	}
}

func TestCatalogManager_GetTemplate(t *testing.T) {
	cm := NewCatalogManager()

	tmpl, err := cm.GetTemplate("postgres")
	if err != nil {
		t.Fatalf("unexpected error getting postgres template: %v", err)
	}
	if tmpl.ID != "postgres" || tmpl.Title != "PostgreSQL 16" {
		t.Errorf("unexpected template data: %+v", tmpl)
	}

	_, err = cm.GetTemplate("does-not-exist")
	if err == nil {
		t.Fatalf("expected error for non-existent template, got nil")
	}
}

func TestCatalogManager_CategoriesAndSources(t *testing.T) {
	cm := NewCatalogManager()

	categories := cm.Categories()
	if len(categories) == 0 {
		t.Fatalf("expected non-empty categories list")
	}

	counts := cm.CategoryCounts()
	if counts["Database"] == 0 {
		t.Errorf("expected non-zero count for Database category")
	}

	sources := cm.Sources()
	if len(sources) == 0 || sources[0] != "curated" {
		t.Errorf("expected sources to include 'curated', got: %v", sources)
	}
}

func TestSubstituteEnv(t *testing.T) {
	yamlInput := `
services:
  db:
    image: postgres:16
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-pdswarm_default}
      POSTGRES_USER: ${POSTGRES_USER}
      SECRET_KEY: ${SECRET_KEY}
    ports:
      - "${PORT:-5432}:5432"
`

	templateEnvs := []TemplateEnvVar{
		{Name: "POSTGRES_DB", Default: "pdswarm_default", Required: false},
		{Name: "POSTGRES_USER", Default: "postgres_default", Required: false},
		{Name: "SECRET_KEY", Default: "", Required: true},
		{Name: "PORT", Default: "5432", Required: false},
	}

	// Case 1: Missing required variable
	_, err := SubstituteEnv(yamlInput, map[string]string{
		"POSTGRES_DB": "custom_db",
	}, templateEnvs)
	if err == nil {
		t.Fatalf("expected error due to missing required SECRET_KEY, got nil")
	}

	// Case 2: Successful substitution with custom values and defaults
	out, err := SubstituteEnv(yamlInput, map[string]string{
		"POSTGRES_DB": "custom_db",
		"SECRET_KEY":  "supersecret",
	}, templateEnvs)
	if err != nil {
		t.Fatalf("unexpected error substituting env: %v", err)
	}

	if !testing.Short() {
		t.Logf("Substituted YAML:\n%s", out)
	}

	// Check custom value substituted
	if !containsStr(out, "POSTGRES_DB: custom_db") {
		t.Errorf("expected 'POSTGRES_DB: custom_db' in output: %s", out)
	}
	// Check fallback to template default
	if !containsStr(out, "POSTGRES_USER: postgres_default") {
		t.Errorf("expected 'POSTGRES_USER: postgres_default' in output: %s", out)
	}
	// Check required value substituted
	if !containsStr(out, "SECRET_KEY: supersecret") {
		t.Errorf("expected 'SECRET_KEY: supersecret' in output: %s", out)
	}
	// Check inline default substituted
	if !containsStr(out, `5432:5432`) {
		t.Errorf("expected '5432:5432' in output: %s", out)
	}
}

func containsStr(s, sub string) bool {
	return filepath.Clean(s) != "" && (len(s) >= len(sub)) && (stringContains(s, sub))
}

func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && searchSubstring(s, substr)))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestImportPortainerCatalog(t *testing.T) {
	cm := NewCatalogManager()

	portainerJSON := `{
		"version": "2",
		"templates": [
			{
				"type": 3,
				"title": "Custom Portainer App",
				"description": "A demo app imported from Portainer catalog",
				"categories": ["Utility"],
				"platform": "linux",
				"logo": "https://example.com/logo.png",
				"image": "custom/app:latest",
				"ports": ["8080:80"],
				"volumes": [
					{"container": "/app/data", "bind": "/data/app"}
				],
				"env": [
					{"name": "APP_ENV", "label": "Environment", "default": "production"}
				]
			}
		]
	}`

	imported, err := cm.ImportPortainerCatalog([]byte(portainerJSON))
	if err != nil {
		t.Fatalf("failed to import portainer catalog: %v", err)
	}
	if len(imported) != 1 {
		t.Fatalf("expected 1 imported template, got %d", len(imported))
	}

	tmpl := imported[0]
	if tmpl.Title != "Custom Portainer App" {
		t.Errorf("unexpected title: %s", tmpl.Title)
	}
	if tmpl.Source != "portainer" {
		t.Errorf("unexpected source: %s", tmpl.Source)
	}
	if len(tmpl.Ports) != 1 || tmpl.Ports[0] != "8080:80" {
		t.Errorf("unexpected ports: %v", tmpl.Ports)
	}

	// Verify it shows up in filtered query
	found := cm.ListTemplates("", "", "portainer")
	if len(found) != 1 {
		t.Errorf("expected 1 template with source portainer, got %d", len(found))
	}
}

func TestImportCasaOSTemplate(t *testing.T) {
	cm := NewCatalogManager()

	casaOSYAML := `
x-casaos:
  architectures:
    - amd64
    - arm64
  main: casaos-demo
  category: Utilities
  description:
    en_us: "A handy tool for CasaOS users."
  developer: "CasaOS Team"
  icon: "https://example.com/casaos-icon.png"
  port_map: "9095"
  title:
    en_us: "CasaOS Demo App"

services:
  casaos-demo:
    image: alpine:latest
    restart: unless-stopped
    ports:
      - "9095:80"
`

	tmpl, err := cm.ImportCasaOSTemplate([]byte(casaOSYAML))
	if err != nil {
		t.Fatalf("failed to import casaos template: %v", err)
	}

	if tmpl.Title != "CasaOS Demo App" {
		t.Errorf("expected title 'CasaOS Demo App', got %q", tmpl.Title)
	}
	if tmpl.Source != "casaos" {
		t.Errorf("expected source 'casaos', got %q", tmpl.Source)
	}
	if len(tmpl.Categories) == 0 || tmpl.Categories[0] != "Utilities" {
		t.Errorf("expected category 'Utilities', got %v", tmpl.Categories)
	}
}

func TestSyncCatalogs_HTTP(t *testing.T) {
	cm := NewCatalogManager()

	// Mock server for Portainer catalog
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version": "2", "templates": [{"type": 1, "title": "Synced App", "image": "synced:latest"}]}`))
	}))
	defer ts.Close()

	err := cm.SyncPortainerCatalog(context.Background(), ts.Client(), ts.URL)
	if err != nil {
		t.Fatalf("failed to sync portainer catalog: %v", err)
	}

	tmpl, err := cm.GetTemplate("portainer-synced-app")
	if err != nil {
		t.Fatalf("failed to get synced template: %v", err)
	}
	if tmpl.Title != "Synced App" {
		t.Errorf("expected synced title 'Synced App', got %q", tmpl.Title)
	}
}

func TestDeployTemplate_StoreAndValidation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pdswarm_tmpl_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create bolt store: %v", err)
	}
	defer st.Close()

	cm := NewCatalogManager()

	// Deploy postgres
	req := DeployTemplateRequest{
		TemplateID: "postgres",
		StackName:  "my-postgres",
		Env: map[string]string{
			"POSTGRES_DB":       "testdb",
			"POSTGRES_PASSWORD": "secretpassword",
		},
	}

	stack, err := cm.Deploy(context.Background(), nil, st, req)
	if err != nil {
		t.Fatalf("failed to deploy template: %v", err)
	}

	if stack.Name != "my-postgres" {
		t.Errorf("expected stack name 'my-postgres', got %q", stack.Name)
	}
	if !stringContains(stack.ComposeYAML, "POSTGRES_DB: testdb") {
		t.Errorf("expected substituted POSTGRES_DB: testdb in compose YAML, got:\n%s", stack.ComposeYAML)
	}

	// Verify saved in store
	saved, err := st.GetStack("my-postgres")
	if err != nil {
		t.Fatalf("failed to get saved stack: %v", err)
	}
	if saved.Name != "my-postgres" {
		t.Errorf("mismatched stack name in store: %q", saved.Name)
	}

	// Try deploying again with same stack name -> conflict
	_, err = cm.Deploy(context.Background(), nil, st, req)
	if err == nil {
		t.Fatalf("expected conflict error when deploying existing stack name, got nil")
	}

	// Try deploying invalid template id
	_, err = cm.Deploy(context.Background(), nil, st, DeployTemplateRequest{
		TemplateID: "non-existent",
		StackName:  "stack2",
	})
	if err == nil {
		t.Fatalf("expected error for non-existent template, got nil")
	}
}

func TestImportCasaOSZip(t *testing.T) {
	cm := NewCatalogManager()

	// Create test zip archive in memory
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	composeYAML := `
name: linuxserver-adguardhome-sync
services:
  app:
    image: linuxserver/adguardhome-sync:0.9.4
    restart: unless-stopped
    ports:
      - 8080:8080
    x-casaos:
      envs:
        - container: PUID
          description:
            en_US: User ID
x-casaos:
  title:
    en_us: Adguard Home Sync
  tagline:
    en_us: Sync AdGuard Home configurations
  category: Utilities
  icon: https://raw.githubusercontent.com/WisdomSky/LinuxServer-AppStore/main/Apps/Adguardhome-sync/icon.png
`
	f, err := zw.Create("Apps/Adguardhome-sync/docker-compose.yml")
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	if _, err := f.Write([]byte(composeYAML)); err != nil {
		t.Fatalf("failed to write zip content: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("failed to close zip writer: %v", err)
	}

	imported, err := cm.ImportCasaOSZip(buf.Bytes(), "LinuxServer CasaOS")
	if err != nil {
		t.Fatalf("failed to import CasaOS zip: %v", err)
	}
	if len(imported) != 1 {
		t.Fatalf("expected 1 imported template, got %d", len(imported))
	}

	tmpl := imported[0]
	if tmpl.Title != "Adguard Home Sync" {
		t.Errorf("expected title 'Adguard Home Sync', got %q", tmpl.Title)
	}
	if tmpl.Source != "LinuxServer CasaOS" {
		t.Errorf("expected source 'LinuxServer CasaOS', got %q", tmpl.Source)
	}
	if tmpl.Logo == "" {
		t.Errorf("expected logo URL from x-casaos")
	}
}

func TestCatalogManager_Rebuild_And_DisableDefaults(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "catalog_rebuild.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer st.Close()

	cm := NewCatalogManager()

	// 1. Initial state: curated templates present
	if len(cm.ListTemplates("", "", "")) < 30 {
		t.Fatalf("expected at least 30 curated templates initially")
	}

	// 2. Disable default templates
	if err := st.SaveSetting("disable_default_templates", "true"); err != nil {
		t.Fatalf("failed to save setting: %v", err)
	}

	if err := cm.Rebuild(context.Background(), nil, st); err != nil {
		t.Fatalf("failed to rebuild catalog: %v", err)
	}

	// Curated templates should now be gone!
	if len(cm.ListTemplates("", "", "")) != 0 {
		t.Fatalf("expected 0 templates when defaults disabled and no repos configured, got %d", len(cm.ListTemplates("", "", "")))
	}

	// 3. Re-enable default templates
	if err := st.SaveSetting("disable_default_templates", "false"); err != nil {
		t.Fatalf("failed to save setting: %v", err)
	}

	if err := cm.Rebuild(context.Background(), nil, st); err != nil {
		t.Fatalf("failed to rebuild catalog: %v", err)
	}

	if len(cm.ListTemplates("", "", "")) < 30 {
		t.Fatalf("expected curated templates restored after re-enabling defaults")
	}
}

func TestExtractEnvFromCompose_CrowdsecUI(t *testing.T) {
	composeYAML := `
version: '3.8'

services:
  crowdsec-web-ui:
    image: ghcr.io/theduffman85/crowdsec-web-ui:latest
    container_name: crowdsec-web-ui
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      - CROWDSEC_URL=${CROWDSEC_URL:-http://crowdsec:8080}
      - CROWDSEC_USER=${CROWDSEC_USER:-crowdsec-web-ui}
      - CROWDSEC_PASSWORD=${CROWDSEC_PASSWORD}
      - CROWDSEC_LOOKBACK_PERIOD=${CROWDSEC_LOOKBACK_PERIOD:-168h}
      - CROWDSEC_REFRESH_INTERVAL=${CROWDSEC_REFRESH_INTERVAL:-30s}
      - CROWDSEC_IDLE_REFRESH_INTERVAL=${CROWDSEC_IDLE_REFRESH_INTERVAL:-5m}
      - CROWDSEC_IDLE_THRESHOLD=${CROWDSEC_IDLE_THRESHOLD:-2m}
      - CROWDSEC_FULL_REFRESH_INTERVAL=${CROWDSEC_FULL_REFRESH_INTERVAL:-5m}
      - BASE_PATH=${BASE_PATH:-}
    volumes:
      - "/home/podiax/Downloads/test123/crowdsec-web-ui/data:/app/data"
    networks:
      - crowdsec-web-ui-network

networks:
  crowdsec-web-ui-network:
    driver: bridge
`

	envs := extractEnvFromCompose(composeYAML)
	t.Logf("Extracted %d environment variables:", len(envs))
	for _, e := range envs {
		t.Logf(" - Name: %s, Default: %q, Required: %v", e.Name, e.Default, e.Required)
	}

	// Must extract exactly the 9 environment variables
	expectedVars := []string{
		"CROWDSEC_URL",
		"CROWDSEC_USER",
		"CROWDSEC_PASSWORD",
		"CROWDSEC_LOOKBACK_PERIOD",
		"CROWDSEC_REFRESH_INTERVAL",
		"CROWDSEC_IDLE_REFRESH_INTERVAL",
		"CROWDSEC_IDLE_THRESHOLD",
		"CROWDSEC_FULL_REFRESH_INTERVAL",
		"BASE_PATH",
	}

	if len(envs) != len(expectedVars) {
		t.Fatalf("expected exactly %d environment variables, got %d", len(expectedVars), len(envs))
	}

	for i, expected := range expectedVars {
		if envs[i].Name != expected {
			t.Errorf("expected var %d to be %s, got %s", i, expected, envs[i].Name)
		}
	}

	// Ensure volumes and networks are NOT extracted as env vars
	for _, e := range envs {
		if strings.Contains(e.Name, "/") || strings.Contains(e.Name, ":") || strings.Contains(e.Name, "volumes") || strings.Contains(e.Name, "networks") {
			t.Errorf("forbidden non-env variable extracted: %s", e.Name)
		}
	}
}

