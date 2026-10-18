package store

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	bbolt "go.etcd.io/bbolt"
)

// Environment represents a target Docker host (local socket or remote TCP/TLS daemon).
type Environment struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"` // "local", "tcp", "tls"
	URL       string    `json:"url"`
	TLSCert   string    `json:"tls_cert,omitempty"`
	TLSKey    string    `json:"tls_key,omitempty"`
	TLSCA     string    `json:"tls_ca,omitempty"`
	IsDefault bool      `json:"is_default"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ensureDefaultEnvironment seeds the default "local" environment if no environments exist.
func (s *BoltStore) ensureDefaultEnvironment() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEnvironments)
		if b == nil {
			return nil
		}

		if b.Stats().KeyN == 0 {
			now := time.Now()
			dockerHost := os.Getenv("DOCKER_HOST")
			if dockerHost == "" {
				dockerHost = "unix:///var/run/docker.sock"
			}
			localEnv := &Environment{
				ID:        "local",
				Name:      "Local Daemon",
				Type:      "local",
				URL:       dockerHost,
				IsDefault: true,
				CreatedAt: now,
				UpdatedAt: now,
			}
			data, err := json.Marshal(localEnv)
			if err != nil {
				return err
			}
			return b.Put([]byte(localEnv.ID), data)
		}
		return nil
	})
}

// GetEnvironment retrieves an environment by its ID.
func (s *BoltStore) GetEnvironment(id string) (*Environment, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: environment id cannot be empty", ErrInvalidInput)
	}

	var env Environment
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEnvironments)
		if b == nil {
			return ErrEnvironmentNotFound
		}
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("%w: %s", ErrEnvironmentNotFound, id)
		}
		return json.Unmarshal(data, &env)
	})
	if err != nil {
		return nil, err
	}
	return &env, nil
}

// ListEnvironments returns all configured environments.
func (s *BoltStore) ListEnvironments() ([]*Environment, error) {
	envs := make([]*Environment, 0)
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEnvironments)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var env Environment
			if err := json.Unmarshal(v, &env); err != nil {
				return err
			}
			envs = append(envs, &env)
			return nil
		})
	})
	return envs, err
}

// SaveEnvironment creates or updates an environment. If env.IsDefault is true,
// other environments will have their default flag cleared.
func (s *BoltStore) SaveEnvironment(env *Environment) error {
	if env == nil || env.ID == "" {
		return fmt.Errorf("%w: environment or environment ID cannot be empty", ErrInvalidInput)
	}

	now := time.Now()
	if env.CreatedAt.IsZero() {
		env.CreatedAt = now
	}
	env.UpdatedAt = now

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEnvironments)
		if b == nil {
			return fmt.Errorf("bucket %s does not exist", bucketEnvironments)
		}

		type pendingPut struct {
			key []byte
			val []byte
		}
		var updates []pendingPut

		// If this environment is marked default, unset default on all other environments
		if env.IsDefault {
			err := b.ForEach(func(k, v []byte) error {
				if string(k) == env.ID {
					return nil
				}
				var other Environment
				if err := json.Unmarshal(v, &other); err != nil {
					return err
				}
				if other.IsDefault {
					other.IsDefault = false
					data, err := json.Marshal(&other)
					if err != nil {
						return err
					}
					kCopy := append([]byte(nil), k...)
					updates = append(updates, pendingPut{key: kCopy, val: data})
				}
				return nil
			})
			if err != nil {
				return err
			}

			for _, u := range updates {
				if err := b.Put(u.key, u.val); err != nil {
					return err
				}
			}
		}

		data, err := json.Marshal(env)
		if err != nil {
			return err
		}
		return b.Put([]byte(env.ID), data)
	})
}

// DeleteEnvironment deletes an environment by ID. Returns ErrCannotDeleteLastEnvironment
// if attempting to delete the only remaining environment.
func (s *BoltStore) DeleteEnvironment(id string) error {
	if id == "" {
		return fmt.Errorf("%w: environment id cannot be empty", ErrInvalidInput)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEnvironments)
		if b == nil {
			return ErrEnvironmentNotFound
		}

		if b.Stats().KeyN <= 1 {
			return ErrCannotDeleteLastEnvironment
		}

		targetData := b.Get([]byte(id))
		if targetData == nil {
			return fmt.Errorf("%w: %s", ErrEnvironmentNotFound, id)
		}

		var target Environment
		_ = json.Unmarshal(targetData, &target)

		if err := b.Delete([]byte(id)); err != nil {
			return err
		}

		// If we deleted the default environment, promote the first remaining one to default
		if target.IsDefault {
			c := b.Cursor()
			k, v := c.First()
			if k != nil && v != nil {
				kCopy := append([]byte(nil), k...)
				var promo Environment
				if err := json.Unmarshal(v, &promo); err == nil {
					promo.IsDefault = true
					promo.UpdatedAt = time.Now()
					if data, err := json.Marshal(&promo); err == nil {
						_ = b.Put(kCopy, data)
					}
				}
			}
		}

		return nil
	})
}

// GetDefaultEnvironment retrieves the default environment. If none is explicitly marked default,
// the first available environment is returned.
func (s *BoltStore) GetDefaultEnvironment() (*Environment, error) {
	var defaultEnv *Environment
	var firstEnv *Environment

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEnvironments)
		if b == nil {
			return ErrEnvironmentNotFound
		}

		return b.ForEach(func(k, v []byte) error {
			var env Environment
			if err := json.Unmarshal(v, &env); err != nil {
				return err
			}
			if firstEnv == nil {
				firstCopy := env
				firstEnv = &firstCopy
			}
			if env.IsDefault {
				envCopy := env
				defaultEnv = &envCopy
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	if defaultEnv != nil {
		return defaultEnv, nil
	}
	if firstEnv != nil {
		return firstEnv, nil
	}
	return nil, ErrEnvironmentNotFound
}

// SetDefaultEnvironment sets the environment with the given ID as the default,
// unsetting default on all others.
func (s *BoltStore) SetDefaultEnvironment(id string) error {
	if id == "" {
		return fmt.Errorf("%w: environment id cannot be empty", ErrInvalidInput)
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketEnvironments)
		if b == nil {
			return ErrEnvironmentNotFound
		}

		targetData := b.Get([]byte(id))
		if targetData == nil {
			return fmt.Errorf("%w: %s", ErrEnvironmentNotFound, id)
		}

		type pendingPut struct {
			key []byte
			val []byte
		}
		var updates []pendingPut

		err := b.ForEach(func(k, v []byte) error {
			var env Environment
			if err := json.Unmarshal(v, &env); err != nil {
				return err
			}

			shouldBeDefault := string(k) == id
			if env.IsDefault != shouldBeDefault {
				env.IsDefault = shouldBeDefault
				env.UpdatedAt = time.Now()
				data, err := json.Marshal(&env)
				if err != nil {
					return err
				}
				kCopy := append([]byte(nil), k...)
				updates = append(updates, pendingPut{key: kCopy, val: data})
			}
			return nil
		})
		if err != nil {
			return err
		}

		for _, u := range updates {
			if err := b.Put(u.key, u.val); err != nil {
				return err
			}
		}
		return nil
	})
}
