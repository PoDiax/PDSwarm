package store_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/podiax/pdswarm/internal/store"
)

func setupTestDB(t *testing.T) (*store.BoltStore, func()) {
	tmpDir, err := os.MkdirTemp("", "pdswarm-store-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	dbPath := filepath.Join(tmpDir, "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to init store: %v", err)
	}
	cleanup := func() {
		st.Close()
		os.RemoveAll(tmpDir)
	}
	return st, cleanup
}

func TestStoreUsersAndStacks(t *testing.T) {
	st, cleanup := setupTestDB(t)
	defer cleanup()

	// Verify interface compliance
	var _ store.Store = st

	// Verify empty ListStacks returns non-nil empty slice (serializes to [])
	initialStacks, err := st.ListStacks()
	if err != nil {
		t.Fatalf("failed to list initial stacks: %v", err)
	}
	if initialStacks == nil {
		t.Fatalf("expected non-nil slice for initial stacks, got nil")
	}
	if len(initialStacks) != 0 {
		t.Fatalf("expected 0 initial stacks, got %d", len(initialStacks))
	}
	rawJSON, err := json.Marshal(initialStacks)
	if err != nil {
		t.Fatalf("failed to marshal empty stacks slice: %v", err)
	}
	if string(rawJSON) != "[]" {
		t.Fatalf("expected JSON '[]', got %q", string(rawJSON))
	}

	count, err := st.CountUsers()
	if err != nil {
		t.Fatalf("failed to count users: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 users initially, got %d", count)
	}

	u := &store.User{
		Username:     "admin",
		PasswordHash: "hashedsecret",
		Role:         "admin",
		CreatedAt:    time.Now().UTC(),
	}
	if err := st.SaveUser(u); err != nil {
		t.Fatalf("failed to save user: %v", err)
	}

	count, err = st.CountUsers()
	if err != nil {
		t.Fatalf("failed to count users after save: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 user, got %d", count)
	}

	got, err := st.GetUser("admin")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if got.Username != "admin" || got.PasswordHash != "hashedsecret" || got.Role != "admin" {
		t.Errorf("unexpected user data: %+v", got)
	}

	// Test non-existent user and sentinel error
	_, err = st.GetUser("nonexistent")
	if err == nil {
		t.Fatalf("expected error getting nonexistent user, got nil")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if !errors.Is(err, store.ErrUserNotFound) {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}

	stack := &store.Stack{
		Name:        "web-app",
		ComposeYAML: "version: '3'\nservices:\n  web:\n    image: nginx",
		Status:      "running",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	if err := st.SaveStack(stack); err != nil {
		t.Fatalf("failed to save stack: %v", err)
	}

	stacks, err := st.ListStacks()
	if err != nil || len(stacks) != 1 {
		t.Fatalf("expected 1 stack, got %d, err: %v", len(stacks), err)
	}

	gotStack, err := st.GetStack("web-app")
	if err != nil {
		t.Fatalf("failed to get stack: %v", err)
	}
	if gotStack.Name != "web-app" || gotStack.Status != "running" || gotStack.ComposeYAML != stack.ComposeYAML {
		t.Errorf("unexpected stack data: %+v", gotStack)
	}

	// Test non-existent stack and sentinel error
	_, err = st.GetStack("nonexistent")
	if err == nil {
		t.Fatalf("expected error getting nonexistent stack, got nil")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if !errors.Is(err, store.ErrStackNotFound) {
		t.Errorf("expected ErrStackNotFound, got %v", err)
	}

	// Test delete stack
	if err := st.DeleteStack("web-app"); err != nil {
		t.Fatalf("failed to delete stack: %v", err)
	}

	stacksAfterDelete, err := st.ListStacks()
	if err != nil || len(stacksAfterDelete) != 0 {
		t.Fatalf("expected 0 stacks after delete, got %d", len(stacksAfterDelete))
	}
	if stacksAfterDelete == nil {
		t.Fatalf("expected non-nil empty slice after delete, got nil")
	}

	_, err = st.GetStack("web-app")
	if err == nil {
		t.Fatalf("expected error getting deleted stack, got nil")
	}
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound on deleted stack, got %v", err)
	}
}

func TestInputValidation(t *testing.T) {
	st, cleanup := setupTestDB(t)
	defer cleanup()

	// SaveUser validation
	if err := st.SaveUser(nil); !errors.Is(err, store.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for nil user, got %v", err)
	}
	if err := st.SaveUser(&store.User{Username: ""}); !errors.Is(err, store.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for user with empty username, got %v", err)
	}
	if _, err := st.GetUser(""); !errors.Is(err, store.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for GetUser with empty username, got %v", err)
	}

	// SaveStack / GetStack / DeleteStack validation
	if err := st.SaveStack(nil); !errors.Is(err, store.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for nil stack, got %v", err)
	}
	if err := st.SaveStack(&store.Stack{Name: ""}); !errors.Is(err, store.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for stack with empty name, got %v", err)
	}
	if _, err := st.GetStack(""); !errors.Is(err, store.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for GetStack with empty name, got %v", err)
	}
	if err := st.DeleteStack(""); !errors.Is(err, store.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for DeleteStack with empty name, got %v", err)
	}

	// SaveSetting / GetSetting validation
	if err := st.SaveSetting("", "val"); !errors.Is(err, store.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for SaveSetting with empty key, got %v", err)
	}
	if _, err := st.GetSetting(""); !errors.Is(err, store.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for GetSetting with empty key, got %v", err)
	}
}

func TestStoreSettings(t *testing.T) {
	st, cleanup := setupTestDB(t)
	defer cleanup()

	// Missing setting
	_, err := st.GetSetting("missing_key")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing key, got %v", err)
	}

	// Save and retrieve
	if err := st.SaveSetting("jwt_secret", "super-secret-token"); err != nil {
		t.Fatalf("failed to save setting: %v", err)
	}

	val, err := st.GetSetting("jwt_secret")
	if err != nil {
		t.Fatalf("failed to get setting: %v", err)
	}
	if val != "super-secret-token" {
		t.Fatalf("expected 'super-secret-token', got %q", val)
	}

	// Update setting
	if err := st.SaveSetting("jwt_secret", "updated-secret"); err != nil {
		t.Fatalf("failed to update setting: %v", err)
	}
	val, err = st.GetSetting("jwt_secret")
	if err != nil {
		t.Fatalf("failed to get updated setting: %v", err)
	}
	if val != "updated-secret" {
		t.Fatalf("expected 'updated-secret', got %q", val)
	}
}

func TestStoreEnvironments(t *testing.T) {
	st, cleanup := setupTestDB(t)
	defer cleanup()

	// 1. Check auto-seeded default environment
	envs, err := st.ListEnvironments()
	if err != nil {
		t.Fatalf("failed to list environments: %v", err)
	}
	if len(envs) != 1 {
		t.Fatalf("expected 1 auto-seeded environment, got %d", len(envs))
	}
	defEnv, err := st.GetDefaultEnvironment()
	if err != nil {
		t.Fatalf("failed to get default environment: %v", err)
	}
	if defEnv.ID != "local" || !defEnv.IsDefault {
		t.Fatalf("expected default environment ID 'local' and IsDefault=true, got ID=%s, IsDefault=%v", defEnv.ID, defEnv.IsDefault)
	}

	// 2. Cannot delete the only remaining environment
	err = st.DeleteEnvironment("local")
	if !errors.Is(err, store.ErrCannotDeleteLastEnvironment) {
		t.Fatalf("expected ErrCannotDeleteLastEnvironment, got %v", err)
	}

	// 3. Save a new remote environment
	remote := &store.Environment{
		ID:        "remote-1",
		Name:      "Remote Host 1",
		Type:      "tcp",
		URL:       "tcp://192.168.1.100:2375",
		IsDefault: false,
	}
	if err := st.SaveEnvironment(remote); err != nil {
		t.Fatalf("failed to save remote environment: %v", err)
	}

	gotRemote, err := st.GetEnvironment("remote-1")
	if err != nil {
		t.Fatalf("failed to get remote environment: %v", err)
	}
	if gotRemote.Name != "Remote Host 1" || gotRemote.Type != "tcp" {
		t.Fatalf("unexpected remote env values: %+v", gotRemote)
	}

	// List should now have 2
	envs, err = st.ListEnvironments()
	if err != nil || len(envs) != 2 {
		t.Fatalf("expected 2 environments, got %d (err: %v)", len(envs), err)
	}

	// 4. Switch default environment
	if err := st.SetDefaultEnvironment("remote-1"); err != nil {
		t.Fatalf("failed to set default environment: %v", err)
	}
	defEnv, err = st.GetDefaultEnvironment()
	if err != nil || defEnv.ID != "remote-1" {
		t.Fatalf("expected default environment to be 'remote-1', got ID=%s (err: %v)", defEnv.ID, err)
	}

	// Local should no longer be default
	localEnv, err := st.GetEnvironment("local")
	if err != nil || localEnv.IsDefault {
		t.Fatalf("expected local environment to not be default, got IsDefault=%v", localEnv.IsDefault)
	}

	// 5. Delete remote-1 (which is default). Local should now be promoted to default!
	if err := st.DeleteEnvironment("remote-1"); err != nil {
		t.Fatalf("failed to delete remote-1: %v", err)
	}
	defEnv, err = st.GetDefaultEnvironment()
	if err != nil || defEnv.ID != "local" {
		t.Fatalf("expected promoted default to be 'local', got ID=%s (err: %v)", defEnv.ID, err)
	}
	if !defEnv.IsDefault {
		t.Fatalf("expected local to have IsDefault=true after promotion")
	}

	// 6. Non-existent environment
	_, err = st.GetEnvironment("non-existent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// 7. Input validation
	if err := st.SaveEnvironment(nil); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for nil env, got %v", err)
	}
	if err := st.SaveEnvironment(&store.Environment{ID: ""}); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty ID, got %v", err)
	}
	if _, err := st.GetEnvironment(""); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty ID, got %v", err)
	}
	if err := st.DeleteEnvironment(""); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty ID, got %v", err)
	}
	if err := st.SetDefaultEnvironment(""); !errors.Is(err, store.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty ID, got %v", err)
	}
}

