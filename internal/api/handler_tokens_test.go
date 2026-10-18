package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/podiax/pdswarm/internal/api"
	"github.com/podiax/pdswarm/internal/auth"
	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/store"
)

func setupTestRouter(t *testing.T) (http.Handler, store.Store, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_api.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	hash, _ := auth.HashPassword("adminpass")
	admin := &store.User{
		Username:     "admin",
		PasswordHash: hash,
		Role:         "admin",
		CreatedAt:    time.Now(),
	}
	if err := st.SaveUser(admin); err != nil {
		t.Fatalf("failed to seed admin: %v", err)
	}

	cfg := &config.Config{
		JWTSecret: "test-jwt-secret-key-32-bytes!!",
		NoAuth:    false,
	}

	router := api.SetupRouter(cfg, st, nil)
	return router, st, cfg.JWTSecret
}

func getAdminAuthHeader(t *testing.T, secret string) string {
	t.Helper()
	token, err := auth.GenerateJWT("admin", "admin", secret, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}
	return "Bearer " + token
}

func TestTokens_Lifecycle(t *testing.T) {
	router, _, secret := setupTestRouter(t)
	authHeader := getAdminAuthHeader(t, secret)

	// 1. List initially empty
	req := httptest.NewRequest(http.MethodGet, "/api/tokens", nil)
	req.Header.Set("Authorization", authHeader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var tokens []api.TokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &tokens); err != nil {
		t.Fatalf("failed to decode list: %v", err)
	}
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens initially, got %d", len(tokens))
	}

	// 2. Create Token
	createBody := `{"name":"Deployment Worker","expires_days":30}`
	req = httptest.NewRequest(http.MethodPost, "/api/tokens", strings.NewReader(createBody))
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var createResp api.CreateTokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create resp: %v", err)
	}
	if !strings.HasPrefix(createResp.Token, "pds_") {
		t.Errorf("expected token to start with pds_, got %s", createResp.Token)
	}
	if createResp.TokenInfo.Name != "Deployment Worker" {
		t.Errorf("unexpected token name: %s", createResp.TokenInfo.Name)
	}
	if createResp.TokenInfo.CreatedBy != "admin" {
		t.Errorf("unexpected created_by: %s", createResp.TokenInfo.CreatedBy)
	}
	if createResp.TokenInfo.ExpiresAt == nil {
		t.Errorf("expected ExpiresAt to be set for 30 days")
	}

	rawAPIToken := createResp.Token
	tokenID := createResp.TokenInfo.ID

	// 3. Authenticate using the new API token on /api/auth/me
	apiReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	apiReq.Header.Set("Authorization", "Bearer "+rawAPIToken)
	apiRec := httptest.NewRecorder()
	router.ServeHTTP(apiRec, apiReq)

	if apiRec.Code != http.StatusOK {
		t.Fatalf("expected 200 with API token auth, got %d: %s", apiRec.Code, apiRec.Body.String())
	}
	var me api.UserInfoDTO
	if err := json.Unmarshal(apiRec.Body.Bytes(), &me); err != nil {
		t.Fatalf("failed to decode me resp: %v", err)
	}
	if me.Username != "admin" || me.Role != "admin" {
		t.Errorf("unexpected user info from API token: %+v", me)
	}

	// 4. List tokens to see updated last used
	req = httptest.NewRequest(http.MethodGet, "/api/tokens", nil)
	req.Header.Set("Authorization", authHeader)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	tokens = nil
	_ = json.Unmarshal(rec.Body.Bytes(), &tokens)
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].LastUsedAt == nil {
		t.Errorf("expected LastUsedAt to be updated after API token use")
	}

	// 5. Delete Token
	req = httptest.NewRequest(http.MethodDelete, "/api/tokens/"+tokenID, nil)
	req.Header.Set("Authorization", authHeader)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on delete, got %d", rec.Code)
	}

	// 6. Deleted token should no longer authenticate
	apiReq = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	apiReq.Header.Set("Authorization", "Bearer "+rawAPIToken)
	apiRec = httptest.NewRecorder()
	router.ServeHTTP(apiRec, apiReq)

	if apiRec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for revoked token, got %d", apiRec.Code)
	}
}

func TestAuth_ChangePassword(t *testing.T) {
	router, st, secret := setupTestRouter(t)
	authHeader := getAdminAuthHeader(t, secret)

	// Wrong current password
	badBody := `{"old_password":"wrong","new_password":"newpass123"}`
	req := httptest.NewRequest(http.MethodPut, "/api/auth/password", strings.NewReader(badBody))
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for wrong old password, got %d", rec.Code)
	}

	// Correct current password
	goodBody := `{"old_password":"adminpass","new_password":"newAdminPass456"}`
	req = httptest.NewRequest(http.MethodPut, "/api/auth/password", strings.NewReader(goodBody))
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid password change, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify login with new password
	user, err := st.GetUser("admin")
	if err != nil {
		t.Fatalf("failed to get admin user: %v", err)
	}
	if !auth.CheckPassword("newAdminPass456", user.PasswordHash) {
		t.Errorf("password in store was not updated correctly")
	}
}

func TestUsers_Management(t *testing.T) {
	router, _, secret := setupTestRouter(t)
	authHeader := getAdminAuthHeader(t, secret)

	// List users (should have admin)
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Authorization", authHeader)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var users []api.UserResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &users); err != nil {
		t.Fatalf("failed to decode users: %v", err)
	}
	if len(users) != 1 || users[0].Username != "admin" {
		t.Errorf("unexpected initial users: %+v", users)
	}

	// Create user
	createUserJSON := `{"username":"operator1","password":"opPassword!","role":"operator"}`
	req = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(createUserJSON))
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create user, got %d: %s", rec.Code, rec.Body.String())
	}

	// Duplicate user should fail
	req = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewBufferString(createUserJSON))
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 for duplicate user, got %d", rec.Code)
	}

	// Cannot delete self (admin)
	req = httptest.NewRequest(http.MethodDelete, "/api/users/admin", nil)
	req.Header.Set("Authorization", authHeader)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 when attempting to delete self, got %d", rec.Code)
	}

	// Delete operator1
	req = httptest.NewRequest(http.MethodDelete, "/api/users/operator1", nil)
	req.Header.Set("Authorization", authHeader)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 deleting operator1, got %d: %s", rec.Code, rec.Body.String())
	}
}
