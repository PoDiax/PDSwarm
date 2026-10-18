package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/podiax/pdswarm/internal/api"
	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/docker"
)

func TestHandlerBuildImage_NilDockerService(t *testing.T) {
	st := newTestStore(t)
	cfg := &config.Config{NoAuth: true}
	handler := api.SetupRouter(cfg, st, nil)

	body, _ := json.Marshal(api.BuildImageRequest{
		Tag:        "my-app:latest",
		Dockerfile: "FROM alpine:latest",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/images/build", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerBuildImage_Validation(t *testing.T) {
	st := newTestStore(t)
	cfg := &config.Config{NoAuth: true}
	ds, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create docker service: %v", err)
	}
	defer ds.Close()

	handler := api.SetupRouter(cfg, st, ds)

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantError  string
	}{
		{
			name:       "nil / empty body",
			body:       "",
			wantStatus: http.StatusBadRequest,
			wantError:  "request body is required",
		},
		{
			name:       "invalid JSON",
			body:       `{not valid json}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "invalid request body",
		},
		{
			name:       "missing tag",
			body:       `{"tag": "", "dockerfile": "FROM alpine"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "image tag is required",
		},
		{
			name:       "whitespace tag",
			body:       `{"tag": "   ", "dockerfile": "FROM alpine"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "image tag is required",
		},
		{
			name:       "missing dockerfile",
			body:       `{"tag": "my-app:latest", "dockerfile": ""}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "dockerfile content is required",
		},
		{
			name:       "whitespace dockerfile",
			body:       `{"tag": "my-app:latest", "dockerfile": "   \n  \t "}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "dockerfile content is required",
		},
		{
			name:       "invalid context_dir",
			body:       `{"tag": "my-app:latest", "dockerfile": "FROM alpine", "context_dir": "/nonexistent/test/path"}`,
			wantStatus: http.StatusBadRequest,
			wantError:  "failed to start image build",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body == "" {
				req = httptest.NewRequest(http.MethodPost, "/api/images/build", nil)
			} else {
				req = httptest.NewRequest(http.MethodPost, "/api/images/build", strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}

			if !strings.Contains(rec.Body.String(), tt.wantError) {
				t.Errorf("expected body to contain %q, got: %s", tt.wantError, rec.Body.String())
			}
		})
	}
}

func TestHandlerBuildImage_LiveBuildStream(t *testing.T) {
	st := newTestStore(t)
	cfg := &config.Config{NoAuth: true}
	ds, err := docker.NewService("")
	if err != nil {
		t.Skip("docker not available")
	}
	defer ds.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ds.Ping(ctx); err != nil {
		t.Skipf("docker daemon ping failed: %v", err)
	}

	handler := api.SetupRouter(cfg, st, ds)

	testTag := "pdswarm-api-builder-test:latest"
	payload := api.BuildImageRequest{
		Tag:        testTag,
		Dockerfile: "FROM scratch\nLABEL org.opencontainers.image.title=\"pdswarm-api-test\"\n",
	}

	bodyBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/images/build", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/x-ndjson") {
		t.Errorf("expected Content-Type to contain application/x-ndjson, got %q", contentType)
	}

	out := rec.Body.String()
	if !strings.Contains(out, "stream") {
		t.Errorf("expected stream output in response, got: %s", out)
	}

	// Cleanup
	_ = ds.RemoveImage(context.Background(), testTag, true)
}
