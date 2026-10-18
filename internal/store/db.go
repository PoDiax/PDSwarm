package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	bbolt "go.etcd.io/bbolt"
)

var (
	bucketUsers        = []byte("users")
	bucketSettings     = []byte("settings")
	bucketStacks       = []byte("stacks")
	bucketEnvironments = []byte("environments")
	bucketRegistries   = []byte("registries")
	bucketTokens       = []byte("tokens")
	bucketTemplateRepos = []byte("template_repos")
)

var (
	// ErrNotFound is returned when a requested record is not found in the database.
	ErrNotFound = errors.New("not found")

	// ErrUserNotFound is returned when a user record does not exist.
	ErrUserNotFound = fmt.Errorf("user: %w", ErrNotFound)

	// ErrTokenNotFound is returned when an API token record does not exist.
	ErrTokenNotFound = fmt.Errorf("token: %w", ErrNotFound)

	// ErrTokenExpired is returned when an API token is past its expiration.
	ErrTokenExpired = errors.New("token expired")

	// ErrCannotDeleteLastAdmin is returned when attempting to delete the only admin.
	ErrCannotDeleteLastAdmin = errors.New("cannot delete last remaining admin")

	// ErrStackNotFound is returned when a stack record does not exist.
	ErrStackNotFound = fmt.Errorf("stack: %w", ErrNotFound)

	// ErrEnvironmentNotFound is returned when an environment record does not exist.
	ErrEnvironmentNotFound = fmt.Errorf("environment: %w", ErrNotFound)

	// ErrRegistryNotFound is returned when a registry record does not exist.
	ErrRegistryNotFound = fmt.Errorf("registry: %w", ErrNotFound)

	// ErrTemplateRepoNotFound is returned when a template repo record does not exist.
	ErrTemplateRepoNotFound = fmt.Errorf("template repo: %w", ErrNotFound)

	// ErrCannotDeleteLastEnvironment is returned when attempting to delete the only environment.
	ErrCannotDeleteLastEnvironment = errors.New("cannot delete last remaining environment")

	// ErrInvalidInput is returned when invalid arguments or empty keys are provided.
	ErrInvalidInput = errors.New("invalid input")
)

// Store defines the persistence interface for PDSwarm.
type Store interface {
	Close() error
	GetUser(username string) (*User, error)
	SaveUser(user *User) error
	CountUsers() (int, error)
	ListUsers() ([]*User, error)
	DeleteUser(username string) error
	GetStack(name string) (*Stack, error)
	ListStacks() ([]*Stack, error)
	SaveStack(stack *Stack) error
	DeleteStack(name string) error
	GetSetting(key string) (string, error)
	SaveSetting(key string, val string) error

	// Environments
	GetEnvironment(id string) (*Environment, error)
	ListEnvironments() ([]*Environment, error)
	SaveEnvironment(env *Environment) error
	DeleteEnvironment(id string) error
	GetDefaultEnvironment() (*Environment, error)
	SetDefaultEnvironment(id string) error

	// Registries
	SaveRegistry(r *Registry) error
	GetRegistry(id string) (*Registry, error)
	GetRegistryByURL(rawURL string) (*Registry, error)
	ListRegistries() ([]*Registry, error)
	DeleteRegistry(id string) error

	// API Tokens
	SaveToken(token *APIToken) error
	GetToken(id string) (*APIToken, error)
	ListTokens() ([]*APIToken, error)
	DeleteToken(id string) error
	ValidateToken(rawToken string) (*APIToken, error)

	// Template Repositories
	SaveTemplateRepo(repo *TemplateRepo) error
	GetTemplateRepo(id string) (*TemplateRepo, error)
	ListTemplateRepos() ([]*TemplateRepo, error)
	DeleteTemplateRepo(id string) error
}

// BoltStore implements Store using pure-Go bbolt embedded database.
type BoltStore struct {
	db *bbolt.DB
}

var _ Store = (*BoltStore)(nil)

// New initializes a new BoltStore at the specified path and ensures buckets exist.
func New(dbPath string) (*BoltStore, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open bbolt db: %w", err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		for _, b := range [][]byte{bucketUsers, bucketSettings, bucketStacks, bucketEnvironments, bucketRegistries, bucketTokens, bucketTemplateRepos} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create buckets: %w", err)
	}

	st := &BoltStore{db: db}
	if err := st.ensureDefaultEnvironment(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to seed default environment: %w", err)
	}

	return st, nil
}

// Close closes the underlying bbolt database.
func (s *BoltStore) Close() error {
	return s.db.Close()
}

// GetSetting retrieves a configuration value from the settings bucket.
func (s *BoltStore) GetSetting(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("%w: setting key cannot be empty", ErrInvalidInput)
	}
	var val string
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		data := b.Get([]byte(key))
		if data == nil {
			return ErrNotFound
		}
		val = string(data)
		return nil
	})
	if err != nil {
		return "", err
	}
	return val, nil
}

// SaveSetting stores a configuration value in the settings bucket.
func (s *BoltStore) SaveSetting(key, val string) error {
	if key == "" {
		return fmt.Errorf("%w: setting key cannot be empty", ErrInvalidInput)
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		return b.Put([]byte(key), []byte(val))
	})
}

