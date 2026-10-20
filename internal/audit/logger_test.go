package audit_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/podiax/pdswarm/internal/audit"
)

func TestAuditLogger_FileCreationAndLevels(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pdswarm-audit-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	logPath := filepath.Join(tmpDir, "security.log")
	logger, err := audit.New(logPath, "auth", 9000)
	if err != nil {
		t.Fatalf("failed to create audit logger: %v", err)
	}
	defer logger.Close()

	// 1. Log a success and a failure
	logger.LogLoginSuccess("192.168.1.10", "admin", "Mozilla/5.0")
	logger.LogLoginFailure("10.0.0.99", "hacker", "invalid credentials", "curl/7.68.0")
	logger.LogUnauthorized("172.16.0.5", "/api/containers", "token expired")

	// 2. Read recent logs
	lines, err := logger.ReadRecent(50)
	if err != nil {
		t.Fatalf("failed to read recent logs: %v", err)
	}

	if len(lines) != 3 {
		t.Fatalf("expected 3 log lines, got %d", len(lines))
	}

	if !strings.Contains(lines[0], "[AUTH] [SUCCESS]") || !strings.Contains(lines[0], "ip=192.168.1.10") {
		t.Errorf("line 0 unexpected content: %s", lines[0])
	}
	if !strings.Contains(lines[1], "[AUTH] [FAIL]") || !strings.Contains(lines[1], "ip=10.0.0.99") {
		t.Errorf("line 1 unexpected content: %s", lines[1])
	}
	if !strings.Contains(lines[2], "[AUTH] [FAIL]") || !strings.Contains(lines[2], "ip=172.16.0.5") {
		t.Errorf("line 2 unexpected content: %s", lines[2])
	}

	// 3. Fail2ban configs
	filterCfg := logger.Fail2banFilterConfig()
	if !strings.Contains(filterCfg, "failregex =") {
		t.Errorf("filter config missing failregex: %s", filterCfg)
	}

	jailCfg := logger.Fail2banJailConfig()
	if !strings.Contains(jailCfg, logPath) || !strings.Contains(jailCfg, "port     = 9000") {
		t.Errorf("jail config missing log path or port: %s", jailCfg)
	}
}

func TestClientIP(t *testing.T) {
	// Direct remote addr with port
	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.RemoteAddr = "192.168.1.50:54321"
	if ip := audit.ClientIP(req1); ip != "192.168.1.50" {
		t.Errorf("expected 192.168.1.50, got %s", ip)
	}

	// Remote addr without port
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "203.0.113.195"
	if ip := audit.ClientIP(req2); ip != "203.0.113.195" {
		t.Errorf("expected 203.0.113.195, got %s", ip)
	}

	// Nil request
	if ip := audit.ClientIP(nil); ip != "unknown" {
		t.Errorf("expected unknown, got %s", ip)
	}
}
