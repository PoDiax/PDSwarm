package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	bbolt "go.etcd.io/bbolt"
)

// TemplateRepo represents a custom external template catalog (Portainer JSON feed, CasaOS Zip archive, etc.).
type TemplateRepo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	URL        string     `json:"url"`
	Type       string     `json:"type"` // "auto", "portainer", "casaos_zip", "casaos_compose"
	Enabled    bool       `json:"enabled"`
	LastSyncAt *time.Time `json:"last_sync_at,omitempty"`
	SyncError  string     `json:"sync_error,omitempty"`
	ItemCount  int        `json:"item_count"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// SaveTemplateRepo creates or updates a custom template repository.
func (s *BoltStore) SaveTemplateRepo(repo *TemplateRepo) error {
	if repo == nil {
		return fmt.Errorf("%w: template repo cannot be nil", ErrInvalidInput)
	}
	repo.ID = strings.TrimSpace(repo.ID)
	repo.Name = strings.TrimSpace(repo.Name)
	repo.URL = strings.TrimSpace(repo.URL)
	repo.Type = strings.TrimSpace(repo.Type)

	if repo.ID == "" {
		return fmt.Errorf("%w: template repo id cannot be empty", ErrInvalidInput)
	}
	if repo.Name == "" {
		return fmt.Errorf("%w: template repo name cannot be empty", ErrInvalidInput)
	}
	if repo.URL == "" {
		return fmt.Errorf("%w: template repo url cannot be empty", ErrInvalidInput)
	}
	if repo.Type == "" {
		repo.Type = "auto"
	}

	now := time.Now()
	if repo.CreatedAt.IsZero() {
		repo.CreatedAt = now
	}
	repo.UpdatedAt = now

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketTemplateRepos)
		if b == nil {
			return ErrTemplateRepoNotFound
		}
		data, err := json.Marshal(repo)
		if err != nil {
			return fmt.Errorf("failed to marshal template repo: %w", err)
		}
		return b.Put([]byte(repo.ID), data)
	})
}

// GetTemplateRepo retrieves a template repository by ID.
func (s *BoltStore) GetTemplateRepo(id string) (*TemplateRepo, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("%w: template repo id cannot be empty", ErrInvalidInput)
	}

	var repo TemplateRepo
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketTemplateRepos)
		if b == nil {
			return ErrTemplateRepoNotFound
		}
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("%w: %s", ErrTemplateRepoNotFound, id)
		}
		return json.Unmarshal(data, &repo)
	})
	if err != nil {
		return nil, err
	}
	return &repo, nil
}

// ListTemplateRepos lists all registered template repositories sorted by CreatedAt.
func (s *BoltStore) ListTemplateRepos() ([]*TemplateRepo, error) {
	var repos []*TemplateRepo
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketTemplateRepos)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var r TemplateRepo
			if err := json.Unmarshal(v, &r); err != nil {
				return err
			}
			repos = append(repos, &r)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].CreatedAt.Before(repos[j].CreatedAt)
	})
	return repos, nil
}

// DeleteTemplateRepo removes a custom template repository by ID.
func (s *BoltStore) DeleteTemplateRepo(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%w: template repo id cannot be empty", ErrInvalidInput)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketTemplateRepos)
		if b == nil {
			return ErrTemplateRepoNotFound
		}
		if b.Get([]byte(id)) == nil {
			return fmt.Errorf("%w: %s", ErrTemplateRepoNotFound, id)
		}
		return b.Delete([]byte(id))
	})
}
