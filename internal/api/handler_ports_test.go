package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/podiax/pdswarm/internal/docker"
)

func TestHandleCheckPortConflicts_NilService(t *testing.T) {
	handler := HandleCheckPortConflicts(nil)

	body, _ := json.Marshal(CheckPortsRequest{
		Ports: []string{"8080:80"},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/system/ports/check", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable when docker service is nil, got %d", rec.Code)
	}
}

func TestHandleCheckPortConflicts_InvalidJSON(t *testing.T) {
	handler := HandleCheckPortConflicts(&docker.Service{})

	req := httptest.NewRequest(http.MethodPost, "/api/system/ports/check", bytes.NewReader([]byte("invalid json")))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", rec.Code)
	}
}
