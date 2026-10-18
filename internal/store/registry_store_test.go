package store_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/podiax/pdswarm/internal/store"
)

func setupTestRegistryStore(t *testing.T) store.Store {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-registry.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})
	return st
}

func TestRegistryStore_CRUD(t *testing.T) {
	st := setupTestRegistryStore(t)

	// List when empty
	regs, err := st.ListRegistries()
	if err != nil {
		t.Fatalf("ListRegistries empty: unexpected error: %v", err)
	}
	if len(regs) != 0 {
		t.Fatalf("expected 0 registries, got %d", len(regs))
	}

	// Save invalid
	if err := st.SaveRegistry(nil); !errors.Is(err, store.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for nil registry, got %v", err)
	}
	if err := st.SaveRegistry(&store.Registry{URL: "docker.io"}); !errors.Is(err, store.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for empty ID, got %v", err)
	}
	if err := st.SaveRegistry(&store.Registry{ID: "reg-1"}); !errors.Is(err, store.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for empty URL, got %v", err)
	}

	// Save valid registry
	reg1 := &store.Registry{
		ID:       "reg-dockerhub",
		Name:     "Docker Hub Main",
		URL:      "docker.io",
		Username: "dockermaster",
		Password: "supersecrettoken",
	}
	if err := st.SaveRegistry(reg1); err != nil {
		t.Fatalf("SaveRegistry failed: %v", err)
	}

	// Get registry
	got, err := st.GetRegistry("reg-dockerhub")
	if err != nil {
		t.Fatalf("GetRegistry failed: %v", err)
	}
	if got.ID != "reg-dockerhub" || got.Name != "Docker Hub Main" || got.Username != "dockermaster" || got.Password != "supersecrettoken" {
		t.Errorf("unexpected registry data: %+v", got)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Errorf("expected non-zero timestamps: created=%v, updated=%v", got.CreatedAt, got.UpdatedAt)
	}

	// Update registry
	time.Sleep(10 * time.Millisecond)
	got.Name = "Docker Hub Updated"
	if err := st.SaveRegistry(got); err != nil {
		t.Fatalf("update SaveRegistry failed: %v", err)
	}
	updated, err := st.GetRegistry("reg-dockerhub")
	if err != nil {
		t.Fatalf("GetRegistry updated failed: %v", err)
	}
	if updated.Name != "Docker Hub Updated" {
		t.Errorf("expected Name to be 'Docker Hub Updated', got %q", updated.Name)
	}
	if !updated.UpdatedAt.After(got.CreatedAt) {
		t.Errorf("expected UpdatedAt > CreatedAt, got %v <= %v", updated.UpdatedAt, got.CreatedAt)
	}

	// Add second registry
	reg2 := &store.Registry{
		ID:       "reg-ghcr",
		Name:     "GitHub Container Registry",
		URL:      "https://ghcr.io/",
		Username: "octocat",
		Password: "ghp_tokendata",
	}
	if err := st.SaveRegistry(reg2); err != nil {
		t.Fatalf("SaveRegistry reg2 failed: %v", err)
	}

	// List registries
	all, err := st.ListRegistries()
	if err != nil {
		t.Fatalf("ListRegistries failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 registries, got %d", len(all))
	}
	if all[0].ID != "reg-dockerhub" || all[1].ID != "reg-ghcr" {
		t.Errorf("unexpected order or IDs: %+v", all)
	}

	// Delete registry
	if err := st.DeleteRegistry("reg-dockerhub"); err != nil {
		t.Fatalf("DeleteRegistry failed: %v", err)
	}
	_, err = st.GetRegistry("reg-dockerhub")
	if !errors.Is(err, store.ErrRegistryNotFound) {
		t.Errorf("expected ErrRegistryNotFound after delete, got %v", err)
	}

	// Delete nonexistent
	if err := st.DeleteRegistry("reg-dockerhub"); !errors.Is(err, store.ErrRegistryNotFound) {
		t.Errorf("expected ErrRegistryNotFound for nonexistent delete, got %v", err)
	}

	// Delete with empty ID
	if err := st.DeleteRegistry(""); !errors.Is(err, store.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for empty ID, got %v", err)
	}
}

func TestRegistryStore_GetByURL(t *testing.T) {
	st := setupTestRegistryStore(t)

	regHub := &store.Registry{
		ID:       "hub",
		Name:     "Docker Hub",
		URL:      "docker.io",
		Username: "hubuser",
		Password: "hubpassword",
	}
	regGHCR := &store.Registry{
		ID:       "ghcr",
		Name:     "GitHub",
		URL:      "ghcr.io",
		Username: "ghuser",
		Password: "ghpassword",
	}
	regHarbor := &store.Registry{
		ID:       "harbor",
		Name:     "Internal Harbor",
		URL:      "harbor.internal.net:8443",
		Username: "harboruser",
		Password: "harborpassword",
	}

	for _, r := range []*store.Registry{regHub, regGHCR, regHarbor} {
		if err := st.SaveRegistry(r); err != nil {
			t.Fatalf("failed saving %s: %v", r.ID, err)
		}
	}

	tests := []struct {
		rawURL      string
		expectedID  string
		expectedErr error
	}{
		{"", "", store.ErrInvalidInput},
		{"docker.io", "hub", nil},
		{"https://docker.io", "hub", nil},
		{"https://docker.io/", "hub", nil},
		{"index.docker.io", "hub", nil},
		{"registry-1.docker.io", "hub", nil},
		{"docker.io/library/alpine", "hub", nil},
		{"ghcr.io", "ghcr", nil},
		{"https://ghcr.io/", "ghcr", nil},
		{"ghcr.io/org/repo:v1.0", "ghcr", nil},
		{"harbor.internal.net:8443", "harbor", nil},
		{"https://harbor.internal.net:8443/team/app:latest", "harbor", nil},
		{"quay.io", "", store.ErrRegistryNotFound},
		{"unknown.registry.io", "", store.ErrRegistryNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.rawURL, func(t *testing.T) {
			matched, err := st.GetRegistryByURL(tt.rawURL)
			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Fatalf("GetRegistryByURL(%q): expected error %v, got %v", tt.rawURL, tt.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("GetRegistryByURL(%q): unexpected error: %v", tt.rawURL, err)
			}
			if matched.ID != tt.expectedID {
				t.Errorf("GetRegistryByURL(%q): expected ID %s, got %s", tt.rawURL, tt.expectedID, matched.ID)
			}
		})
	}
}
