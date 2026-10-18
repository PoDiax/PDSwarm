package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/podiax/pdswarm/internal/api"
	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/docker"
)

func TestContainerInspectResponse_JSON(t *testing.T) {
	resp := api.ContainerInspectResponse{
		ContainerJSON: types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				ID:   "cont-123",
				Name: "/app_postgres",
			},
			Config: &container.Config{
				Image: "postgres:16-alpine",
				Env: []string{
					"PATH=/usr/local/bin:/usr/bin",
					"PG_VERSION=16.15",
					"POSTGRES_DB=app_db",
					"POSTGRES_USER=postgres",
				},
			},
		},
		ImageEnv: []string{
			"PATH=/usr/local/bin:/usr/bin",
			"PG_VERSION=16.15",
		},
		UserEnv: []string{
			"POSTGRES_DB=app_db",
			"POSTGRES_USER=postgres",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal ContainerInspectResponse: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if decoded["Id"] != "cont-123" {
		t.Errorf("expected Id 'cont-123', got %v", decoded["Id"])
	}

	imgEnvRaw, ok := decoded["ImageEnv"].([]interface{})
	if !ok || len(imgEnvRaw) != 2 {
		t.Fatalf("expected 2 ImageEnv items, got %v", decoded["ImageEnv"])
	}

	userEnvRaw, ok := decoded["UserEnv"].([]interface{})
	if !ok || len(userEnvRaw) != 2 {
		t.Fatalf("expected 2 UserEnv items, got %v", decoded["UserEnv"])
	}

	if userEnvRaw[0] != "POSTGRES_DB=app_db" {
		t.Errorf("expected POSTGRES_DB=app_db, got %v", userEnvRaw[0])
	}
}

func TestFilterUserEnv_Separation(t *testing.T) {
	containerEnv := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"GOSU_VERSION=1.19",
		"LANG=en_US.utf8",
		"PG_MAJOR=16",
		"PG_VERSION=16.15",
		"PG_SHA256=...",
		"DOCKER_PG_LLVM_DEPS=llvm21-dev clang21",
		"PGDATA=/var/lib/postgresql/data",
		"POSTGRES_DB=app_db",
		"POSTGRES_USER=postgres",
		"POSTGRES_PASSWORD=postgres",
	}

	imageEnv := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"GOSU_VERSION=1.19",
		"LANG=en_US.utf8",
		"PG_MAJOR=16",
		"PG_VERSION=16.15",
		"PG_SHA256=...",
		"DOCKER_PG_LLVM_DEPS=llvm21-dev clang21",
		"PGDATA=/var/lib/postgresql/data",
	}

	userEnv := docker.FilterUserEnv(containerEnv, imageEnv)
	if len(userEnv) != 3 {
		t.Fatalf("expected 3 user-defined env vars, got %d: %v", len(userEnv), userEnv)
	}

	expected := []string{"POSTGRES_DB=app_db", "POSTGRES_USER=postgres", "POSTGRES_PASSWORD=postgres"}
	for i, exp := range expected {
		if userEnv[i] != exp {
			t.Errorf("at index %d: expected %s, got %s", i, exp, userEnv[i])
		}
	}
}

func TestHandleUpdateContainerImage_NotFound(t *testing.T) {
	st := newTestStore(t)
	cfg := &config.Config{NoAuth: true}
	ds, err := docker.NewService("")
	if err != nil {
		t.Fatalf("failed to create docker service: %v", err)
	}
	defer ds.Close()

	handler := api.SetupRouter(cfg, st, ds)

	req := httptest.NewRequest(http.MethodPost, "/api/containers/nonexistent-container-xyz/update-image", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Since nonexistent container, expect 404 or 500 error, but never panic
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 404 or 500, got %d: %s", rec.Code, rec.Body.String())
	}
}
