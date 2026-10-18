package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/api"
	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/docker"
)

func setupFilesRouter(ds *docker.Service) chi.Router {
	r := chi.NewRouter()
	r.Get("/api/containers/{id}/files/stat", api.HandleStatContainerFile(ds))
	r.Get("/api/containers/{id}/files/list", api.HandleListContainerFiles(ds))
	r.Get("/api/containers/{id}/files/read", api.HandleReadContainerFile(ds))
	r.Get("/api/containers/{id}/files/download", api.HandleDownloadContainerFile(ds))
	r.Post("/api/containers/{id}/files/upload", api.HandleUploadContainerFile(ds))
	r.Delete("/api/containers/{id}/files", api.HandleDeleteContainerFile(ds))
	return r
}

func TestHandleFiles_NilDockerService(t *testing.T) {
	r := setupFilesRouter(nil)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/containers/c123/files/list?path=/"},
		{http.MethodGet, "/api/containers/c123/files/stat?path=/etc"},
		{http.MethodGet, "/api/containers/c123/files/read?path=/etc/hosts"},
		{http.MethodGet, "/api/containers/c123/files/download?path=/etc/hosts"},
		{http.MethodPost, "/api/containers/c123/files/upload?path=/tmp&filename=a.txt"},
		{http.MethodDelete, "/api/containers/c123/files?path=/tmp/a.txt"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, strings.NewReader("body"))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("expected 503 Service Unavailable, got %d", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "docker service not available") {
				t.Errorf("expected error message in response, got %s", rec.Body.String())
			}
		})
	}
}

func TestHandleFiles_ValidationErrors(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create docker service: %v", err)
	}
	defer svc.Close()

	r := setupFilesRouter(svc)

	// Missing container id (blank spaces)
	req := httptest.NewRequest(http.MethodGet, "/api/containers/%20/files/list", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for whitespace container ID, got %d", rec.Code)
	}

	// Missing path on read
	req = httptest.NewRequest(http.MethodGet, "/api/containers/c123/files/read", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing path on read, got %d", rec.Code)
	}

	// Missing path on download
	req = httptest.NewRequest(http.MethodGet, "/api/containers/c123/files/download", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing path on download, got %d", rec.Code)
	}

	// Missing path on delete
	req = httptest.NewRequest(http.MethodDelete, "/api/containers/c123/files", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for missing path on delete, got %d", rec.Code)
	}

	// Missing filename on direct upload
	req = httptest.NewRequest(http.MethodPost, "/api/containers/c123/files/upload?path=/tmp", strings.NewReader("content"))
	req.Header.Set("Content-Type", "text/plain")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for direct upload without filename, got %d", rec.Code)
	}
}

func TestHandleFiles_LiveContainerFlow(t *testing.T) {
	svc, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create docker service: %v", err)
	}
	defer svc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := svc.Ping(ctx); err != nil {
		t.Skipf("Docker daemon not accessible: %v", err)
	}

	containers, err := svc.ListContainers(ctx, false)
	if err != nil || len(containers) == 0 {
		t.Skip("No running containers found for live API test")
	}

	cid := containers[0].ID
	r := setupFilesRouter(svc)

	// 1. List / directory
	req := httptest.NewRequest(http.MethodGet, "/api/containers/"+cid+"/files/list?path=/", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("failed to list files: %d %s", rec.Code, rec.Body.String())
	}
	var listResp []docker.FileInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to parse list JSON: %v", err)
	}
	if len(listResp) == 0 {
		t.Fatal("expected files in / listing")
	}

	// 2. Upload via multipart form
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	part, err := w.CreateFormFile("file", "api_upload_multipart.txt")
	if err != nil {
		t.Fatalf("failed creating form file: %v", err)
	}
	uploadContent := "Multipart test content from PDSwarm test suite\n"
	_, _ = io.WriteString(part, uploadContent)
	_ = w.Close()

	req = httptest.NewRequest(http.MethodPost, "/api/containers/"+cid+"/files/upload?path=/tmp", &b)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("multipart upload failed: %d %s", rec.Code, rec.Body.String())
	}

	// 3. Upload via direct raw body stream
	directContent := "Direct raw body upload content\n"
	req = httptest.NewRequest(http.MethodPost, "/api/containers/"+cid+"/files/upload?path=/tmp&filename=api_upload_direct.txt", strings.NewReader(directContent))
	req.Header.Set("Content-Type", "text/plain")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("direct upload failed: %d %s", rec.Code, rec.Body.String())
	}

	// 4. Stat the uploaded multipart file
	req = httptest.NewRequest(http.MethodGet, "/api/containers/"+cid+"/files/stat?path=/tmp/api_upload_multipart.txt", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("stat failed: %d %s", rec.Code, rec.Body.String())
	}

	// 5. Read the uploaded multipart file
	req = httptest.NewRequest(http.MethodGet, "/api/containers/"+cid+"/files/read?path=/tmp/api_upload_multipart.txt", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("read failed: %d %s", rec.Code, rec.Body.String())
	}
	var readResp api.ReadFileResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &readResp); err != nil {
		t.Fatalf("failed to decode read response: %v", err)
	}
	if readResp.Content != uploadContent {
		t.Errorf("read content mismatch: expected %q, got %q", uploadContent, readResp.Content)
	}

	// 6. Download the file and verify headers
	req = httptest.NewRequest(http.MethodGet, "/api/containers/"+cid+"/files/download?path=/tmp/api_upload_multipart.txt", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("download failed: %d %s", rec.Code, rec.Body.String())
	}
	if disp := rec.Header().Get("Content-Disposition"); !strings.Contains(disp, "api_upload_multipart.txt") {
		t.Errorf("expected Content-Disposition with filename, got %s", disp)
	}
	if ctype := rec.Header().Get("Content-Type"); ctype != "application/octet-stream" {
		t.Errorf("expected Content-Type application/octet-stream, got %s", ctype)
	}
	if rec.Body.String() != uploadContent {
		t.Errorf("downloaded content mismatch: expected %q, got %q", uploadContent, rec.Body.String())
	}

	// 7. Delete both uploaded files
	for _, f := range []string{"api_upload_multipart.txt", "api_upload_direct.txt"} {
		req = httptest.NewRequest(http.MethodDelete, "/api/containers/"+cid+"/files?path=/tmp/"+f, nil)
		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("delete of %s failed: %d %s", f, rec.Code, rec.Body.String())
		}
	}

	// 8. Verify 404 for deleted file
	req = httptest.NewRequest(http.MethodGet, "/api/containers/"+cid+"/files/read?path=/tmp/api_upload_multipart.txt", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for deleted file, got %d", rec.Code)
	}
}

func TestHandleFiles_RouterIntegration_Auth(t *testing.T) {
	st := newTestStore(t)
	cfg := &config.Config{
		NoAuth:    false,
		JWTSecret: "test-secret-key-32-bytes-long!!",
	}

	handler := api.SetupRouter(cfg, st, nil)

	// Unauthenticated request should be 401
	req := httptest.NewRequest(http.MethodGet, "/api/containers/any-id/files/list?path=/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for unauthenticated files route, got %d", rec.Code)
	}
}
