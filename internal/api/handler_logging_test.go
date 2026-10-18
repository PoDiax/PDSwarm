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
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
)

func TestLoggingConfigEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_pdswarm.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	cfg := &config.Config{
		NoAuth:    true,
		JWTSecret: "test-secret-must-be-very-long-and-secure",
	}

	ds, err := docker.NewService("unix:///var/run/docker.sock")
	if err != nil {
		t.Skip("docker not available, skipping")
	}

	handler := api.SetupRouter(cfg, st, ds)

	// 1. Test GET /api/system/logging
	req := httptest.NewRequest(http.MethodGet, "/api/system/logging", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET /api/system/logging, got %d: %s", rec.Code, rec.Body.String())
	}

	var dto api.LoggingConfigDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("failed to parse GET response: %v", err)
	}

	if dto.LogDriver == "" || dto.MaxSize == "" || dto.MaxFile == "" {
		t.Errorf("expected populated logging config fields, got %+v", dto)
	}

	// 2. Test PUT /api/system/logging
	payload := api.UpdateLoggingConfigRequest{
		LogDriver: "json-file",
		MaxSize:   "25m",
		MaxFile:   "5",
		Compress:  true,
	}
	body, _ := json.Marshal(payload)

	req = httptest.NewRequest(http.MethodPut, "/api/system/logging", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for PUT /api/system/logging, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated api.LoggingConfigDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to parse PUT response: %v", err)
	}

	if updated.MaxSize != "25m" || updated.MaxFile != "5" {
		t.Errorf("expected MaxSize 25m and MaxFile 5, got %s / %s", updated.MaxSize, updated.MaxFile)
	}

	// Verify persistence in store
	val, err := st.GetSetting("log_max_size")
	if err != nil || val != "25m" {
		t.Errorf("expected stored log_max_size to be 25m, got %s (err: %v)", val, err)
	}
}
