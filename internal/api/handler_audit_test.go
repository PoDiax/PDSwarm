package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/podiax/pdswarm/internal/api"
	"github.com/podiax/pdswarm/internal/audit"
	"github.com/podiax/pdswarm/internal/config"
)

func TestHandleGetAuditLogs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pdswarm-audit-handler-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "auth.log")
	cfg := &config.Config{
		AuthLogPath: logPath,
		LogLevel:    "auth",
		Port:        9000,
	}

	logger, err := audit.New(logPath, "auth", 9000)
	if err != nil {
		t.Fatalf("failed to init audit logger: %v", err)
	}
	defer logger.Close()
	audit.SetDefault(logger)

	logger.LogLoginFailure("192.168.1.100", "testuser", "bad password", "curl/8.0")

	handler := api.HandleGetAuditLogs(cfg)
	req := httptest.NewRequest(http.MethodGet, "/api/system/audit-logs", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var resp api.AuditLogsResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Path != logPath {
		t.Errorf("expected path %s, got %s", logPath, resp.Path)
	}
	if resp.Level != "auth" {
		t.Errorf("expected level 'auth', got %s", resp.Level)
	}
	if len(resp.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(resp.Entries))
	}
	if !strings.Contains(resp.Fail2banFilter, "failregex =") {
		t.Errorf("missing failregex in fail2ban filter")
	}
	if !strings.Contains(resp.Fail2banJail, "port     = 9000") {
		t.Errorf("missing port in fail2ban jail")
	}
}
