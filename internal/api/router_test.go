package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/podiax/pdswarm/internal/api"
	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/docker"
	"github.com/podiax/pdswarm/internal/store"
)

func newTestStore(t *testing.T) store.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "router_test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestRouter_AuthStatus_EmptyStore(t *testing.T) {
	st := newTestStore(t)
	cfg := &config.Config{
		NoAuth:    false,
		JWTSecret: "test-secret-key-32-bytes-long!!",
	}

	handler := api.SetupRouter(cfg, st, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var status api.AuthStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("failed to parse auth status response: %v", err)
	}

	if !status.SetupRequired {
		t.Errorf("expected SetupRequired to be true for empty store, got false")
	}
	if !status.AuthEnabled {
		t.Errorf("expected AuthEnabled to be true when NoAuth is false, got false")
	}
}

func TestRouter_ProtectedEndpoints_Unauthenticated(t *testing.T) {
	st := newTestStore(t)
	cfg := &config.Config{
		NoAuth:    false,
		JWTSecret: "test-secret-key-32-bytes-long!!",
	}

	handler := api.SetupRouter(cfg, st, nil)

	protectedRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/auth/me"},
		{http.MethodGet, "/api/system/info"},
		{http.MethodGet, "/api/containers"},
		{http.MethodGet, "/api/images"},
		{http.MethodGet, "/api/volumes"},
		{http.MethodGet, "/api/networks"},
		{http.MethodGet, "/api/stacks"},
	}

	for _, route := range protectedRoutes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 Unauthorized for unauthenticated request to %s, got %d", route.path, rec.Code)
			}
		})
	}
}

func TestRouter_SetupWizard_Flow(t *testing.T) {
	st := newTestStore(t)
	cfg := &config.Config{
		NoAuth:    false,
		JWTSecret: "test-secret-key-32-bytes-long!!",
	}

	handler := api.SetupRouter(cfg, st, nil)

	// 1. Initial setup request with admin account
	setupPayload := api.SetupRequest{
		Username: "admin",
		Password: "SuperSecurePassword123!",
	}
	body, _ := json.Marshal(setupPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from setup, got %d: %s", rec.Code, rec.Body.String())
	}

	var authResp api.AuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &authResp); err != nil {
		t.Fatalf("failed to decode setup response: %v", err)
	}

	if authResp.Token == "" {
		t.Fatal("expected JWT token in setup response, got empty string")
	}
	if authResp.User.Username != "admin" || authResp.User.Role != "admin" {
		t.Errorf("expected user 'admin'/'admin', got '%s'/'%s'", authResp.User.Username, authResp.User.Role)
	}

	// Verify Set-Cookie header
	cookies := rec.Result().Cookies()
	var authCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "pdswarm_token" {
			authCookie = c
			break
		}
	}
	if authCookie == nil {
		t.Fatalf("expected pdswarm_token cookie to be set")
	}
	if authCookie.Value != authResp.Token {
		t.Errorf("expected cookie value to match token")
	}

	// 2. Check /api/auth/status now indicates setup is NOT required
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/auth/status", nil)
	recStatus := httptest.NewRecorder()
	handler.ServeHTTP(recStatus, reqStatus)

	var status api.AuthStatusResponse
	_ = json.Unmarshal(recStatus.Body.Bytes(), &status)
	if status.SetupRequired {
		t.Errorf("expected SetupRequired to be false after initial setup, got true")
	}

	// 3. Second call to /api/auth/setup must be rejected (403 Forbidden)
	reqDuplicate := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewReader(body))
	recDuplicate := httptest.NewRecorder()
	handler.ServeHTTP(recDuplicate, reqDuplicate)

	if recDuplicate.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for repeated setup call, got %d", recDuplicate.Code)
	}

	// 4. Access /api/auth/me using Bearer token
	reqMe := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	reqMe.Header.Set("Authorization", "Bearer "+authResp.Token)
	recMe := httptest.NewRecorder()
	handler.ServeHTTP(recMe, reqMe)

	if recMe.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/auth/me with Bearer token, got %d: %s", recMe.Code, recMe.Body.String())
	}

	var meResp api.UserInfoDTO
	if err := json.Unmarshal(recMe.Body.Bytes(), &meResp); err != nil {
		t.Fatalf("failed to decode me response: %v", err)
	}
	if meResp.Username != "admin" {
		t.Errorf("expected username 'admin', got %s", meResp.Username)
	}

	// 5. Access /api/auth/me using Cookie
	reqMeCookie := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	reqMeCookie.AddCookie(authCookie)
	recMeCookie := httptest.NewRecorder()
	handler.ServeHTTP(recMeCookie, reqMeCookie)

	if recMeCookie.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/auth/me with cookie, got %d: %s", recMeCookie.Code, recMeCookie.Body.String())
	}
}

func TestRouter_LoginAndLogout(t *testing.T) {
	st := newTestStore(t)
	cfg := &config.Config{
		NoAuth:    false,
		JWTSecret: "test-secret-key-32-bytes-long!!",
	}

	handler := api.SetupRouter(cfg, st, nil)

	// Set up initial user
	setupBody, _ := json.Marshal(api.SetupRequest{
		Username: "operator",
		Password: "validPassword123",
	})
	reqSetup := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewReader(setupBody))
	recSetup := httptest.NewRecorder()
	handler.ServeHTTP(recSetup, reqSetup)
	if recSetup.Code != http.StatusOK {
		t.Fatalf("setup failed: %d", recSetup.Code)
	}

	// Invalid password
	loginFailBody, _ := json.Marshal(api.LoginRequest{
		Username: "operator",
		Password: "wrongPassword",
	})
	reqFail := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginFailBody))
	recFail := httptest.NewRecorder()
	handler.ServeHTTP(recFail, reqFail)

	if recFail.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong password, got %d", recFail.Code)
	}

	// Valid login
	loginSuccessBody, _ := json.Marshal(api.LoginRequest{
		Username: "operator",
		Password: "validPassword123",
	})
	reqSuccess := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginSuccessBody))
	recSuccess := httptest.NewRecorder()
	handler.ServeHTTP(recSuccess, reqSuccess)

	if recSuccess.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid login, got %d: %s", recSuccess.Code, recSuccess.Body.String())
	}

	var loginResp api.AuthResponse
	if err := json.Unmarshal(recSuccess.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if loginResp.Token == "" {
		t.Errorf("expected token in login response")
	}

	// Logout
	reqLogout := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	recLogout := httptest.NewRecorder()
	handler.ServeHTTP(recLogout, reqLogout)

	if recLogout.Code != http.StatusOK {
		t.Errorf("expected 200 for logout, got %d", recLogout.Code)
	}

	// Check cookie was cleared
	logoutCookies := recLogout.Result().Cookies()
	var clearedCookie *http.Cookie
	for _, c := range logoutCookies {
		if c.Name == "pdswarm_token" {
			clearedCookie = c
			break
		}
	}
	if clearedCookie == nil || clearedCookie.MaxAge >= 0 {
		t.Errorf("expected pdswarm_token cookie to be cleared with MaxAge < 0")
	}
}

func TestRouter_NoAuthMode(t *testing.T) {
	st := newTestStore(t)
	cfg := &config.Config{
		NoAuth:    true,
		JWTSecret: "test-secret-key-32-bytes-long!!",
	}

	handler := api.SetupRouter(cfg, st, nil)

	// In NoAuth mode, /api/stacks should succeed without any token
	req := httptest.NewRequest(http.MethodGet, "/api/stacks", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/stacks in NoAuth mode, got %d: %s", rec.Code, rec.Body.String())
	}

	// /api/auth/me should return anonymous user
	reqMe := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	recMe := httptest.NewRecorder()
	handler.ServeHTTP(recMe, reqMe)

	if recMe.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/auth/me in NoAuth mode, got %d: %s", recMe.Code, recMe.Body.String())
	}
}

func TestRouter_SystemPing(t *testing.T) {
	st := newTestStore(t)
	cfg := &config.Config{NoAuth: false}

	handler := api.SetupRouter(cfg, st, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/system/ping", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for ping, got %d: %s", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), `"status":"ok"`) {
		t.Errorf("expected status ok in ping response, got %s", rec.Body.String())
	}
}

func TestRouter_CORS(t *testing.T) {
	st := newTestStore(t)
	cfg := &config.Config{NoAuth: false}

	handler := api.SetupRouter(cfg, st, nil)

	req := httptest.NewRequest(http.MethodOptions, "/api/auth/status", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusNoContent {
		t.Errorf("expected 200 or 204 for CORS preflight, got %d", rec.Code)
	}

	allowOrigin := rec.Header().Get("Access-Control-Allow-Origin")
	if allowOrigin == "" {
		t.Errorf("expected Access-Control-Allow-Origin header to be present")
	}
}

func TestRouter_AuthInputValidation(t *testing.T) {
	st := newTestStore(t)
	cfg := &config.Config{NoAuth: false}
	handler := api.SetupRouter(cfg, st, nil)

	// Setup with empty username
	body, _ := json.Marshal(api.SetupRequest{Username: "", Password: "secret"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty username, got %d", rec.Code)
	}

	// Setup with empty password
	body, _ = json.Marshal(api.SetupRequest{Username: "admin", Password: ""})
	req = httptest.NewRequest(http.MethodPost, "/api/auth/setup", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty password, got %d", rec.Code)
	}

	// Login with empty username
	body, _ = json.Marshal(api.LoginRequest{Username: "", Password: "secret"})
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty username on login, got %d", rec.Code)
	}
}

func TestRouter_DockerEndpoints_NilService(t *testing.T) {
	st := newTestStore(t)
	cfg := &config.Config{NoAuth: true} // bypass auth to directly hit handler
	handler := api.SetupRouter(cfg, st, nil)

	endpoints := []string{
		"/api/system/info",
		"/api/containers",
		"/api/images",
		"/api/volumes",
		"/api/networks",
	}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("expected 503 Service Unavailable for nil docker service on %s, got %d", ep, rec.Code)
			}
		})
	}
}

func TestRouter_DockerEndpoints_Live(t *testing.T) {
	st := newTestStore(t)
	cfg := &config.Config{NoAuth: true}

	ds, err := docker.NewService("")
	if err != nil {
		t.Skipf("cannot create docker service: %v", err)
	}
	defer ds.Close()

	if err := ds.Ping(context.Background()); err != nil {
		t.Skipf("docker daemon unavailable: %v", err)
	}

	handler := api.SetupRouter(cfg, st, ds)

	// Test GET /api/system/info
	req := httptest.NewRequest(http.MethodGet, "/api/system/info", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for system info, got %d: %s", rec.Code, rec.Body.String())
	}

	// Test GET /api/containers
	req = httptest.NewRequest(http.MethodGet, "/api/containers", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for list containers, got %d: %s", rec.Code, rec.Body.String())
	}

	// Test GET /api/images
	req = httptest.NewRequest(http.MethodGet, "/api/images", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for list images, got %d: %s", rec.Code, rec.Body.String())
	}

	// Test GET /api/volumes
	req = httptest.NewRequest(http.MethodGet, "/api/volumes", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for list volumes, got %d: %s", rec.Code, rec.Body.String())
	}

	// Test GET /api/networks
	req = httptest.NewRequest(http.MethodGet, "/api/networks", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for list networks, got %d: %s", rec.Code, rec.Body.String())
	}
}

