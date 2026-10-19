package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/store"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"0.2.0", "v0.2.1", -1},
		{"v0.2.0", "0.2.0", 0},
		{"0.2.1", "0.2.0", 1},
		{"v1.0.0", "0.9.9", 1},
		{"0.2", "0.2.0", 0},
		{"0.2.0.1", "0.2.0", 1},
		{"0.1.9", "0.2.0", -1},
	}

	for _, tt := range tests {
		got := compareVersions(tt.v1, tt.v2)
		if got != tt.expected {
			t.Errorf("compareVersions(%q, %q) = %d; want %d", tt.v1, tt.v2, got, tt.expected)
		}
	}
}

func TestHandleGetUpdateStatus(t *testing.T) {
	config.Version = "0.2.0"
	globalReleaseCache.Lock()
	globalReleaseCache.lastChecked = time.Now()
	globalReleaseCache.latestVersion = "v0.2.1"
	globalReleaseCache.releaseURL = "https://github.com/PoDiax/PDSwarm/releases/tag/v0.2.1"
	globalReleaseCache.releaseNotes = "New features"
	globalReleaseCache.publishedAt = "2026-10-19T00:00:00Z"
	globalReleaseCache.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/system/update", nil)
	w := httptest.NewRecorder()

	handler := HandleGetUpdateStatus(nil)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	var dto UpdateStatusDTO
	if err := json.NewDecoder(w.Body).Decode(&dto); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if dto.CurrentVersion != "v0.2.0" {
		t.Errorf("expected current version v0.2.0, got %s", dto.CurrentVersion)
	}
	if dto.LatestVersion != "v0.2.1" {
		t.Errorf("expected latest version v0.2.1, got %s", dto.LatestVersion)
	}
	if !dto.UpdateAvailable {
		t.Errorf("expected update_available to be true")
	}
}

func TestHandleSetAutoUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	st, err := store.New(tmpDir + "/test.db")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	defer st.Close()

	payload := SetAutoUpdateRequest{Enabled: true}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/api/system/update/auto", bytes.NewReader(b))
	w := httptest.NewRecorder()

	handler := HandleSetAutoUpdate(st)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	val, err := st.GetSetting("auto_update_enabled")
	if err != nil || val != "true" {
		t.Errorf("expected setting auto_update_enabled='true', got %s (err: %v)", val, err)
	}

	// Now disable
	payload.Enabled = false
	b, _ = json.Marshal(payload)
	req = httptest.NewRequest(http.MethodPut, "/api/system/update/auto", bytes.NewReader(b))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	val, err = st.GetSetting("auto_update_enabled")
	if err != nil || val != "false" {
		t.Errorf("expected setting auto_update_enabled='false', got %s (err: %v)", val, err)
	}
}
