package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/podiax/pdswarm/internal/api"
)

func TestHandleSuggestHostPaths(t *testing.T) {
	handler := api.HandleSuggestHostPaths(nil)

	// 1. Test empty query returns root directories
	req1 := httptest.NewRequest(http.MethodGet, "/api/system/host-paths", nil)
	rr1 := httptest.NewRecorder()
	handler(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr1.Code)
	}

	var res1 api.HostPathResponse
	if err := json.NewDecoder(rr1.Body).Decode(&res1); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(res1.Suggestions) == 0 {
		t.Errorf("expected non-empty root suggestions for empty query")
	}

	// 2. Test querying root "/"
	req2 := httptest.NewRequest(http.MethodGet, "/api/system/host-paths?query=/", nil)
	rr2 := httptest.NewRecorder()
	handler(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr2.Code)
	}

	var res2 api.HostPathResponse
	if err := json.NewDecoder(rr2.Body).Decode(&res2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(res2.Suggestions) == 0 {
		t.Errorf("expected non-empty suggestions for root path query")
	}

	// 3. Test querying non-existent path
	req3 := httptest.NewRequest(http.MethodGet, "/api/system/host-paths?query=/nonexistent_dir_xyz_123/", nil)
	rr3 := httptest.NewRecorder()
	handler(rr3, req3)

	if rr3.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr3.Code)
	}

	var res3 api.HostPathResponse
	if err := json.NewDecoder(rr3.Body).Decode(&res3); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(res3.Suggestions) != 0 {
		t.Errorf("expected 0 suggestions for nonexistent path, got %d", len(res3.Suggestions))
	}
}
