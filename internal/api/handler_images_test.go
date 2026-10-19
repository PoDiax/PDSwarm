package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/podiax/pdswarm/internal/api"
)

func TestHandleCheckImageUpdatesBatch_Validation(t *testing.T) {
	// 1. Nil Docker service returns 503
	handler := api.HandleCheckImageUpdatesBatch(nil, nil)

	body, _ := json.Marshal(api.CheckImageUpdatesBatchRequest{
		Images: []string{"nginx:latest", "redis:alpine"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/images/check-updates", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}

	// 2. Invalid JSON payload
	reqBad := httptest.NewRequest(http.MethodPost, "/api/images/check-updates", bytes.NewReader([]byte("{invalid-json")))
	reqBad.Header.Set("Content-Type", "application/json")
	wBad := httptest.NewRecorder()

	// With a non-nil handler context error expectation
	handler(wBad, reqBad)
	// Still 503 because docker service is nil (checked first)
	if wBad.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", wBad.Code)
	}
}
