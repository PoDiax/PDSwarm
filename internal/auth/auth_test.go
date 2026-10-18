package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/podiax/pdswarm/internal/auth"
	"github.com/podiax/pdswarm/internal/store"
)

func TestPasswordAndJWT(t *testing.T) {
	rawPass := "superSecret123"
	hash, err := auth.HashPassword(rawPass)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !auth.CheckPassword(rawPass, hash) {
		t.Fatalf("password check failed for correct password")
	}
	if auth.CheckPassword("wrongPass", hash) {
		t.Fatalf("password check succeeded for wrong password")
	}

	secret := "test-secret-key-32-bytes-long!!"
	token, err := auth.GenerateJWT("admin", "admin", secret, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	claims, err := auth.ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("failed to validate JWT: %v", err)
	}
	if claims.Username != "admin" || claims.Role != "admin" {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestValidateJWT_Expired(t *testing.T) {
	secret := "test-secret-key-32-bytes-long!!"
	token, err := auth.GenerateJWT("admin", "admin", secret, -time.Minute)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	_, err = auth.ValidateJWT(token, secret)
	if err == nil {
		t.Fatalf("expected error for expired JWT, got nil")
	}
}

func TestValidateJWT_InvalidSecret(t *testing.T) {
	secret1 := "test-secret-key-32-bytes-long!!"
	secret2 := "different-secret-key-32-bytes!"
	token, err := auth.GenerateJWT("admin", "admin", secret1, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	_, err = auth.ValidateJWT(token, secret2)
	if err == nil {
		t.Fatalf("expected error for JWT validated with wrong secret, got nil")
	}
}

func TestAuthMiddleware_NoAuth(t *testing.T) {
	secret := "test-secret-key-32-bytes-long!!"
	mw := auth.AuthMiddleware(secret, true)

	called := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatalf("expected inner handler to be called when noAuth=true")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_BearerToken(t *testing.T) {
	secret := "test-secret-key-32-bytes-long!!"
	mw := auth.AuthMiddleware(secret, false)

	token, err := auth.GenerateJWT("testuser", "operator", secret, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	var extractedClaims *auth.Claims
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, ok := r.Context().Value(auth.UserContextKey).(*auth.Claims); ok {
			extractedClaims = c
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if extractedClaims == nil {
		t.Fatalf("expected claims in request context, got nil")
	}
	if extractedClaims.Username != "testuser" || extractedClaims.Role != "operator" {
		t.Errorf("unexpected claims: %+v", extractedClaims)
	}
}

func TestAuthMiddleware_CookieToken(t *testing.T) {
	secret := "test-secret-key-32-bytes-long!!"
	mw := auth.AuthMiddleware(secret, false)

	token, err := auth.GenerateJWT("cookieuser", "viewer", secret, time.Hour)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	var extractedClaims *auth.Claims
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, ok := r.Context().Value(auth.UserContextKey).(*auth.Claims); ok {
			extractedClaims = c
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	req.AddCookie(&http.Cookie{
		Name:  "pdswarm_token",
		Value: token,
	})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if extractedClaims == nil {
		t.Fatalf("expected claims in request context, got nil")
	}
	if extractedClaims.Username != "cookieuser" || extractedClaims.Role != "viewer" {
		t.Errorf("unexpected claims: %+v", extractedClaims)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	secret := "test-secret-key-32-bytes-long!!"
	mw := auth.AuthMiddleware(secret, false)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	secret := "test-secret-key-32-bytes-long!!"
	mw := auth.AuthMiddleware(secret, false)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	req.Header.Set("Authorization", "Bearer invalid.jwt.token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

type mockValidator struct {
	token string
	tok   *store.APIToken
	err   error
}

func (m *mockValidator) ValidateToken(rawToken string) (*store.APIToken, error) {
	if rawToken == m.token {
		return m.tok, nil
	}
	if m.err != nil {
		return nil, m.err
	}
	return nil, store.ErrNotFound
}

func TestAuthMiddleware_APIToken(t *testing.T) {
	validator := &mockValidator{
		token: "pds_1234567890abcdef1234567890abcdef",
		tok: &store.APIToken{
			ID:        "tok-1",
			Name:      "CI Token",
			CreatedBy: "ci-agent",
		},
	}

	secret := "test-secret-key-32-bytes-long!!"
	mw := auth.AuthMiddleware(secret, false, validator)

	var extractedClaims *auth.Claims
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, ok := r.Context().Value(auth.UserContextKey).(*auth.Claims); ok {
			extractedClaims = c
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	req.Header.Set("Authorization", "Bearer pds_1234567890abcdef1234567890abcdef")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if extractedClaims == nil {
		t.Fatalf("expected claims in request context, got nil")
	}
	if extractedClaims.Username != "ci-agent" || extractedClaims.Role != "admin" {
		t.Errorf("unexpected claims: %+v", extractedClaims)
	}
}

func TestAuthMiddleware_APIToken_Invalid(t *testing.T) {
	validator := &mockValidator{
		token: "pds_valid",
		tok:   &store.APIToken{CreatedBy: "admin"},
	}

	secret := "test-secret-key-32-bytes-long!!"
	mw := auth.AuthMiddleware(secret, false, validator)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/nodes", nil)
	req.Header.Set("Authorization", "Bearer pds_wrong_token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}
}

