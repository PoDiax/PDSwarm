package store

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	bbolt "go.etcd.io/bbolt"
)

// Registry represents credential and connection metadata for a Docker registry.
type Registry struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"` // e.g. "docker.io", "ghcr.io", "harbor.internal.net"
	Username  string    `json:"username"`
	Password  string    `json:"password,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// normalizeRegistryHost extracts and normalizes the host component of a registry or image URL.
func normalizeRegistryHost(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimRight(u, "/")
	host := u
	if idx := strings.Index(u, "/"); idx != -1 {
		host = u[:idx]
	}
	host = strings.ToLower(host)
	if host == "index.docker.io" || host == "registry-1.docker.io" {
		host = "docker.io"
	}
	return host
}

// cleanRegistryURL returns lowercased URL without scheme or trailing slash.
func cleanRegistryURL(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return strings.ToLower(strings.TrimRight(u, "/"))
}

// SaveRegistry saves or updates a registry configuration in the database.
func (s *BoltStore) SaveRegistry(r *Registry) error {
	if r == nil {
		return fmt.Errorf("%w: registry cannot be nil", ErrInvalidInput)
	}
	if strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("%w: registry id cannot be empty", ErrInvalidInput)
	}
	if strings.TrimSpace(r.URL) == "" {
		return fmt.Errorf("%w: registry url cannot be empty", ErrInvalidInput)
	}

	now := time.Now()
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
	r.UpdatedAt = now

	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRegistries)
		if b == nil {
			return ErrRegistryNotFound
		}
		return b.Put([]byte(r.ID), data)
	})
}

// GetRegistry retrieves a registry by its unique ID.
func (s *BoltStore) GetRegistry(id string) (*Registry, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("%w: registry id cannot be empty", ErrInvalidInput)
	}

	var reg Registry
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRegistries)
		if b == nil {
			return ErrRegistryNotFound
		}
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("%w: %s", ErrRegistryNotFound, id)
		}
		return json.Unmarshal(data, &reg)
	})
	if err != nil {
		return nil, err
	}

	return &reg, nil
}

// GetRegistryByURL searches for a registry whose configured URL or host matches rawURL.
func (s *BoltStore) GetRegistryByURL(rawURL string) (*Registry, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("%w: registry url cannot be empty", ErrInvalidInput)
	}

	targetClean := cleanRegistryURL(rawURL)
	targetHost := normalizeRegistryHost(rawURL)

	var matched *Registry

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRegistries)
		if b == nil {
			return ErrRegistryNotFound
		}

		return b.ForEach(func(k, v []byte) error {
			if matched != nil {
				return nil
			}
			var reg Registry
			if err := json.Unmarshal(v, &reg); err != nil {
				return nil
			}

			regClean := cleanRegistryURL(reg.URL)
			regHost := normalizeRegistryHost(reg.URL)

			// 1. Direct clean match
			if regClean == targetClean {
				matched = &reg
				return nil
			}

			// 2. Prefix match (e.g. registry configured as 'harbor.domain/project' and target is 'harbor.domain/project/image')
			if strings.HasPrefix(targetClean, regClean+"/") {
				matched = &reg
				return nil
			}

			// 3. Host match (e.g. registry configured as 'docker.io' and target is 'registry-1.docker.io' or 'index.docker.io')
			if regHost != "" && regHost == targetHost {
				matched = &reg
				return nil
			}

			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	if matched == nil {
		return nil, fmt.Errorf("%w: %s", ErrRegistryNotFound, rawURL)
	}

	return matched, nil
}

// ListRegistries returns all configured registries sorted by CreatedAt ascending.
func (s *BoltStore) ListRegistries() ([]*Registry, error) {
	registries := make([]*Registry, 0)

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRegistries)
		if b == nil {
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			var reg Registry
			if err := json.Unmarshal(v, &reg); err != nil {
				return err
			}
			registries = append(registries, &reg)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(registries, func(i, j int) bool {
		return registries[i].CreatedAt.Before(registries[j].CreatedAt)
	})

	return registries, nil
}

// DeleteRegistry removes a registry configuration by its ID.
func (s *BoltStore) DeleteRegistry(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%w: registry id cannot be empty", ErrInvalidInput)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketRegistries)
		if b == nil {
			return ErrRegistryNotFound
		}

		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("%w: %s", ErrRegistryNotFound, id)
		}

		return b.Delete([]byte(id))
	})
}
