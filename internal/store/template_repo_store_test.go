package store_test

import (
	"path/filepath"
	"testing"

	"github.com/podiax/pdswarm/internal/store"
)

func TestBoltStore_TemplateRepos(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "repos_test.db")

	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	// 1. Initial list empty
	repos, err := st.ListTemplateRepos()
	if err != nil {
		t.Fatalf("failed to list repos: %v", err)
	}
	if len(repos) != 0 {
		t.Fatalf("expected 0 repos, got %d", len(repos))
	}

	// 2. Save repo
	r1 := &store.TemplateRepo{
		ID:      "lissy93-templates",
		Name:    "Lissy93 Portainer Templates",
		URL:     "https://raw.githubusercontent.com/Lissy93/portainer-templates/main/templates.json",
		Type:    "portainer",
		Enabled: true,
	}
	if err := st.SaveTemplateRepo(r1); err != nil {
		t.Fatalf("failed to save repo: %v", err)
	}

	// 3. Get repo
	fetched, err := st.GetTemplateRepo("lissy93-templates")
	if err != nil {
		t.Fatalf("failed to get repo: %v", err)
	}
	if fetched.Name != r1.Name || fetched.URL != r1.URL || !fetched.Enabled {
		t.Fatalf("unexpected repo data: %+v", fetched)
	}

	// 4. Update repo
	fetched.Enabled = false
	fetched.ItemCount = 85
	if err := st.SaveTemplateRepo(fetched); err != nil {
		t.Fatalf("failed to update repo: %v", err)
	}

	updated, err := st.GetTemplateRepo("lissy93-templates")
	if err != nil {
		t.Fatalf("failed to get updated repo: %v", err)
	}
	if updated.Enabled || updated.ItemCount != 85 {
		t.Fatalf("expected enabled=false and itemCount=85, got: %+v", updated)
	}

	// 5. Delete repo
	if err := st.DeleteTemplateRepo("lissy93-templates"); err != nil {
		t.Fatalf("failed to delete repo: %v", err)
	}

	_, err = st.GetTemplateRepo("lissy93-templates")
	if err == nil {
		t.Fatalf("expected error getting deleted repo, got nil")
	}
}
