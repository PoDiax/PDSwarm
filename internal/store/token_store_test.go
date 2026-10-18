package store_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/podiax/pdswarm/internal/store"
)

func setupTestStore(t *testing.T) *store.BoltStore {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_tokens.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() {
		st.Close()
	})
	return st
}

func TestGenerateAPIToken(t *testing.T) {
	raw, hash, prefix, err := store.GenerateAPIToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(raw, "pds_") {
		t.Errorf("expected token to start with pds_, got %s", raw)
	}
	// pds_ + 32 hex characters = 36 characters total
	if len(raw) != 36 {
		t.Errorf("expected token length 36, got %d (%s)", len(raw), raw)
	}
	if len(hash) != 64 {
		t.Errorf("expected sha256 hex length 64, got %d (%s)", len(hash), hash)
	}
	if !strings.HasPrefix(prefix, "pds_") {
		t.Errorf("expected prefix to start with pds_, got %s", prefix)
	}
	if store.HashAPIToken(raw) != hash {
		t.Errorf("HashAPIToken did not match generated hash")
	}
}

func TestAPIToken_CRUD(t *testing.T) {
	st := setupTestStore(t)

	_, hash, prefix, err := store.GenerateAPIToken()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	tok := &store.APIToken{
		ID:        "tok-1",
		Name:      "CI Runner",
		TokenHash: hash,
		Prefix:    prefix,
		CreatedBy: "alice",
		CreatedAt: time.Now(),
	}

	// Save
	if err := st.SaveToken(tok); err != nil {
		t.Fatalf("failed to save token: %v", err)
	}

	// Get
	fetched, err := st.GetToken("tok-1")
	if err != nil {
		t.Fatalf("failed to get token: %v", err)
	}
	if fetched.Name != "CI Runner" || fetched.CreatedBy != "alice" || fetched.Prefix != prefix {
		t.Errorf("unexpected token data: %+v", fetched)
	}

	// List
	list, err := st.ListTokens()
	if err != nil {
		t.Fatalf("failed to list tokens: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 token, got %d", len(list))
	}

	// Delete
	if err := st.DeleteToken("tok-1"); err != nil {
		t.Fatalf("failed to delete token: %v", err)
	}

	// Get after delete
	_, err = st.GetToken("tok-1")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound after deletion, got %v", err)
	}
}

func TestAPIToken_Validate(t *testing.T) {
	st := setupTestStore(t)

	raw, hash, prefix, err := store.GenerateAPIToken()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	tok := &store.APIToken{
		ID:        "tok-valid",
		Name:      "Deploy Script",
		TokenHash: hash,
		Prefix:    prefix,
		CreatedBy: "bob",
		CreatedAt: time.Now(),
	}
	if err := st.SaveToken(tok); err != nil {
		t.Fatalf("failed to save token: %v", err)
	}

	// Validate valid token
	validated, err := st.ValidateToken(raw)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}
	if validated.ID != "tok-valid" || validated.CreatedBy != "bob" {
		t.Errorf("unexpected validated token: %+v", validated)
	}
	if validated.LastUsedAt == nil {
		t.Errorf("expected LastUsedAt to be updated, got nil")
	}

	// Validate invalid token
	_, err = st.ValidateToken("pds_invalid_token_1234567890abcdef")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound for invalid token, got %v", err)
	}
}

func TestAPIToken_Validate_Expired(t *testing.T) {
	st := setupTestStore(t)

	raw, hash, prefix, err := store.GenerateAPIToken()
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	past := time.Now().Add(-1 * time.Hour)
	tok := &store.APIToken{
		ID:        "tok-expired",
		Name:      "Old Token",
		TokenHash: hash,
		Prefix:    prefix,
		CreatedBy: "charlie",
		ExpiresAt: &past,
		CreatedAt: time.Now().Add(-24 * time.Hour),
	}
	if err := st.SaveToken(tok); err != nil {
		t.Fatalf("failed to save token: %v", err)
	}

	_, err = st.ValidateToken(raw)
	if !errors.Is(err, store.ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired for expired token, got %v", err)
	}
}

func TestUser_ListAndDelete(t *testing.T) {
	st := setupTestStore(t)

	u1 := &store.User{
		Username:     "alice",
		PasswordHash: "hash1",
		Role:         "admin",
		CreatedAt:    time.Now(),
	}
	u2 := &store.User{
		Username:     "bob",
		PasswordHash: "hash2",
		Role:         "operator",
		CreatedAt:    time.Now(),
	}

	if err := st.SaveUser(u1); err != nil {
		t.Fatalf("failed to save user 1: %v", err)
	}
	if err := st.SaveUser(u2); err != nil {
		t.Fatalf("failed to save user 2: %v", err)
	}

	users, err := st.ListUsers()
	if err != nil {
		t.Fatalf("failed to list users: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	if err := st.DeleteUser("bob"); err != nil {
		t.Fatalf("failed to delete user: %v", err)
	}

	usersAfter, err := st.ListUsers()
	if err != nil {
		t.Fatalf("failed to list users after delete: %v", err)
	}
	if len(usersAfter) != 1 || usersAfter[0].Username != "alice" {
		t.Errorf("unexpected users after delete: %+v", usersAfter)
	}

	// Deleting non-existent user
	err = st.DeleteUser("nonexistent")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound when deleting non-existent user, got %v", err)
	}
}
