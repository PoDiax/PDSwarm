package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/podiax/pdswarm/internal/api"
	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/tasks"
)

func TestTasksAPI(t *testing.T) {
	st := newTestStore(t)
	cfg := &config.Config{NoAuth: true}
	ds, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create docker service: %v", err)
	}
	defer ds.Close()

	handler := api.SetupRouter(cfg, st, ds)

	// 1. Initially list tasks
	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. Start a mock task
	taskMgr := tasks.DefaultManager()
	testTask := taskMgr.Start(tasks.TypeImagePull, "Pulling test:latest", "test:latest", func(ctx context.Context, t *tasks.Task) error {
		t.SetProgress(50, "Downloading")
		t.AppendLog("chunk 1")
		time.Sleep(50 * time.Millisecond)
		t.SetProgress(100, "Done")
		return nil
	})

	// 3. Get task by ID
	req = httptest.NewRequest(http.MethodGet, "/api/tasks/"+testTask.ID, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for get task, got %d", rec.Code)
	}

	var fetched tasks.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &fetched); err != nil {
		t.Fatalf("failed to parse task JSON: %v", err)
	}
	if fetched.ID != testTask.ID {
		t.Fatalf("expected ID %s, got %s", testTask.ID, fetched.ID)
	}

	// Wait for task to finish
	time.Sleep(70 * time.Millisecond)

	// 4. Delete finished task
	req = httptest.NewRequest(http.MethodDelete, "/api/tasks/"+testTask.ID, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for delete task, got %d: %s", rec.Code, rec.Body.String())
	}
}
