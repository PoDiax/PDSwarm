package api_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/podiax/pdswarm/internal/api"
	"github.com/podiax/pdswarm/internal/config"
)

func TestStatic_ServeStatic(t *testing.T) {
	mockFS := fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte("<!DOCTYPE html><html><head><title>PDSwarm</title></head><body><div id='app'></div></body></html>"),
		},
		"assets/bundle.js": &fstest.MapFile{
			Data: []byte("console.log('pdswarm');"),
		},
		"assets/style.css": &fstest.MapFile{
			Data: []byte("body { margin: 0; }"),
		},
		"logo.svg": &fstest.MapFile{
			Data: []byte("<svg></svg>"),
		},
	}

	st := newTestStore(t)
	cfg := &config.Config{NoAuth: true}
	handler := api.SetupRouter(cfg, st, nil, mockFS)

	// 1. Root route serves index.html
	t.Run("Root path serves index.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			t.Errorf("expected text/html, got %s", rec.Header().Get("Content-Type"))
		}
		if !strings.Contains(rec.Body.String(), "<title>PDSwarm</title>") {
			t.Errorf("expected body to contain PDSwarm, got: %s", rec.Body.String())
		}
	})

	// 2. SPA client-side route fallback to index.html
	t.Run("SPA route falls back to index.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/containers", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
			t.Errorf("expected text/html, got %s", rec.Header().Get("Content-Type"))
		}
		if !strings.Contains(rec.Body.String(), "<title>PDSwarm</title>") {
			t.Errorf("expected body to contain PDSwarm, got: %s", rec.Body.String())
		}
	})

	// 3. Static asset JavaScript
	t.Run("Serve JS asset with content type and cache control", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/bundle.js", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "application/javascript") {
			t.Errorf("expected application/javascript, got %s", rec.Header().Get("Content-Type"))
		}
		if !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
			t.Errorf("expected immutable cache-control for asset, got %s", rec.Header().Get("Cache-Control"))
		}
		if !strings.Contains(rec.Body.String(), "console.log") {
			t.Errorf("expected body to contain console.log, got: %s", rec.Body.String())
		}
	})

	// 4. Static asset CSS
	t.Run("Serve CSS asset with text/css", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/assets/style.css", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "text/css") {
			t.Errorf("expected text/css, got %s", rec.Header().Get("Content-Type"))
		}
	})

	// 5. Unknown /api endpoint does NOT fall back to index.html
	t.Run("Unknown /api endpoint returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "<title>PDSwarm</title>") {
			t.Errorf("/api/does-not-exist should not return index.html")
		}
	})

	// 6. Non-GET/HEAD method to static route returns 405 Method Not Allowed
	t.Run("POST to static route returns 405", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/containers", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 for POST to static route, got %d", rec.Code)
		}
	})

	// 7. HEAD request returns 200 OK without body
	t.Run("HEAD request returns 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodHead, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for HEAD, got %d", rec.Code)
		}
		if rec.Body.Len() != 0 {
			t.Errorf("expected empty body for HEAD, got %d bytes", rec.Body.Len())
		}
	})
}
