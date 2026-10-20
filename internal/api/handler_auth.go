package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/audit"
	"github.com/podiax/pdswarm/internal/auth"
	"github.com/podiax/pdswarm/internal/config"
	"github.com/podiax/pdswarm/internal/store"
)

type AuthStatusResponse struct {
	SetupRequired bool `json:"setup_required"`
	AuthEnabled   bool `json:"auth_enabled"`
}

type SetupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserInfoDTO struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

type AuthResponse struct {
	Token string      `json:"token"`
	User  UserInfoDTO `json:"user"`
}

const (
	tokenCookieName = "pdswarm_token"
	tokenTTL        = 24 * time.Hour
)

func isRequestHTTPS(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func setAuthCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   isRequestHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     tokenCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// HandleAuthStatus reports whether setup is required and if auth is enabled.
func HandleAuthStatus(cfg *config.Config, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		count, err := st.CountUsers()
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to check user count: "+err.Error())
			return
		}
		respondJSON(w, http.StatusOK, AuthStatusResponse{
			SetupRequired: count == 0,
			AuthEnabled:   !cfg.NoAuth,
		})
	}
}

// HandleAuthSetup provisions the initial administrator account.
func HandleAuthSetup(cfg *config.Config, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		count, err := st.CountUsers()
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to check user count: "+err.Error())
			return
		}
		if count > 0 {
			respondJSONError(w, http.StatusForbidden, "setup already completed")
			return
		}

		var req SetupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		req.Username = strings.TrimSpace(req.Username)
		if req.Username == "" {
			respondJSONError(w, http.StatusBadRequest, "username is required")
			return
		}
		if len(req.Password) == 0 {
			respondJSONError(w, http.StatusBadRequest, "password is required")
			return
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to hash password: "+err.Error())
			return
		}

		user := &store.User{
			Username:     req.Username,
			PasswordHash: hash,
			Role:         "admin",
			CreatedAt:    time.Now(),
		}
		if err := st.SaveUser(user); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to create admin user: "+err.Error())
			return
		}
		audit.Default().LogSetup(audit.ClientIP(r), user.Username)

		secret := cfg.JWTSecret
		if secret == "" {
			secret = "pdswarm-default-jwt-secret-key"
		}

		expires := time.Now().Add(tokenTTL)
		token, err := auth.GenerateJWT(user.Username, user.Role, secret, tokenTTL)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to generate token: "+err.Error())
			return
		}

		setAuthCookie(w, r, token, expires)
		respondJSON(w, http.StatusOK, AuthResponse{
			Token: token,
			User: UserInfoDTO{
				Username: user.Username,
				Role:     user.Role,
			},
		})
	}
}

// HandleAuthLogin authenticates a user and returns a token and cookie.
func HandleAuthLogin(cfg *config.Config, st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		req.Username = strings.TrimSpace(req.Username)
		if req.Username == "" || req.Password == "" {
			respondJSONError(w, http.StatusBadRequest, "username and password are required")
			return
		}

		clientIP := audit.ClientIP(r)
		user, err := st.GetUser(req.Username)
		if err != nil || !auth.CheckPassword(req.Password, user.PasswordHash) {
			audit.Default().LogLoginFailure(clientIP, req.Username, "invalid username or password", r.UserAgent())
			respondJSONError(w, http.StatusUnauthorized, "invalid username or password")
			return
		}
		audit.Default().LogLoginSuccess(clientIP, user.Username, r.UserAgent())

		secret := cfg.JWTSecret
		if secret == "" {
			secret = "pdswarm-default-jwt-secret-key"
		}

		expires := time.Now().Add(tokenTTL)
		token, err := auth.GenerateJWT(user.Username, user.Role, secret, tokenTTL)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to generate token: "+err.Error())
			return
		}

		setAuthCookie(w, r, token, expires)
		respondJSON(w, http.StatusOK, AuthResponse{
			Token: token,
			User: UserInfoDTO{
				Username: user.Username,
				Role:     user.Role,
			},
		})
	}
}

// HandleAuthLogout clears the auth cookie.
func HandleAuthLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clearAuthCookie(w)
		respondJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
	}
}

// HandleAuthMe returns the current authenticated user's info.
func HandleAuthMe(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if claims, ok := r.Context().Value(auth.UserContextKey).(*auth.Claims); ok && claims != nil {
			respondJSON(w, http.StatusOK, UserInfoDTO{
				Username: claims.Username,
				Role:     claims.Role,
			})
			return
		}

		if cfg.NoAuth {
			respondJSON(w, http.StatusOK, UserInfoDTO{
				Username: "anonymous",
				Role:     "admin",
			})
			return
		}

		respondJSONError(w, http.StatusUnauthorized, "unauthorized")
	}
}

// ChangePasswordRequest defines the payload for updating the current user's password.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// HandleChangePassword updates the authenticated user's password.
func HandleChangePassword(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(auth.UserContextKey).(*auth.Claims)
		if !ok || claims == nil || claims.Username == "" {
			respondJSONError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req ChangePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		if req.OldPassword == "" {
			respondJSONError(w, http.StatusBadRequest, "old_password is required")
			return
		}
		if len(req.NewPassword) < 1 {
			respondJSONError(w, http.StatusBadRequest, "new_password is required")
			return
		}

		user, err := st.GetUser(claims.Username)
		if err != nil {
			respondJSONError(w, http.StatusNotFound, "user not found")
			return
		}

		if !auth.CheckPassword(req.OldPassword, user.PasswordHash) {
			respondJSONError(w, http.StatusBadRequest, "invalid current password")
			return
		}

		newHash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to hash password: "+err.Error())
			return
		}

		user.PasswordHash = newHash
		if err := st.SaveUser(user); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to update password: "+err.Error())
			return
		}
		audit.Default().LogPasswordChange(audit.ClientIP(r), user.Username)

		respondJSON(w, http.StatusOK, map[string]string{"message": "password updated successfully"})
	}
}

// CreateUserRequest defines the payload for creating a new user account.
type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// UserResponse is the client-facing representation of a user without sensitive hashes.
type UserResponse struct {
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// HandleListUsers returns all registered users omitting password hashes.
func HandleListUsers(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := st.ListUsers()
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to list users: "+err.Error())
			return
		}

		resp := make([]UserResponse, 0, len(users))
		for _, u := range users {
			resp = append(resp, UserResponse{
				Username:  u.Username,
				Role:      u.Role,
				CreatedAt: u.CreatedAt,
			})
		}
		respondJSON(w, http.StatusOK, resp)
	}
}

// HandleCreateUser registers a new user account.
func HandleCreateUser(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req CreateUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		req.Username = strings.TrimSpace(req.Username)
		if req.Username == "" {
			respondJSONError(w, http.StatusBadRequest, "username is required")
			return
		}
		if len(req.Password) == 0 {
			respondJSONError(w, http.StatusBadRequest, "password is required")
			return
		}

		req.Role = strings.TrimSpace(req.Role)
		if req.Role == "" {
			req.Role = "operator"
		}
		if req.Role != "admin" && req.Role != "operator" && req.Role != "viewer" {
			respondJSONError(w, http.StatusBadRequest, "role must be one of: admin, operator, viewer")
			return
		}

		// Ensure user does not already exist
		existing, err := st.GetUser(req.Username)
		if err == nil && existing != nil {
			respondJSONError(w, http.StatusConflict, "username already exists")
			return
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to hash password: "+err.Error())
			return
		}

		user := &store.User{
			Username:     req.Username,
			PasswordHash: hash,
			Role:         req.Role,
			CreatedAt:    time.Now(),
		}
		if err := st.SaveUser(user); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to create user: "+err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, UserResponse{
			Username:  user.Username,
			Role:      user.Role,
			CreatedAt: user.CreatedAt,
		})
	}
}

// HandleDeleteUser deletes a user account, preventing deleting self or the last admin.
func HandleDeleteUser(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetUsername := strings.TrimSpace(chi.URLParam(r, "username"))
		if targetUsername == "" {
			respondJSONError(w, http.StatusBadRequest, "username is required")
			return
		}

		// Prevent deleting self
		if claims, ok := r.Context().Value(auth.UserContextKey).(*auth.Claims); ok && claims != nil {
			if claims.Username == targetUsername {
				respondJSONError(w, http.StatusBadRequest, "cannot delete your own account")
				return
			}
		}

		targetUser, err := st.GetUser(targetUsername)
		if err != nil {
			respondJSONError(w, http.StatusNotFound, "user not found")
			return
		}

		// If target is admin, ensure not the last remaining admin
		if targetUser.Role == "admin" {
			users, err := st.ListUsers()
			if err != nil {
				respondJSONError(w, http.StatusInternalServerError, "failed to verify admin accounts: "+err.Error())
				return
			}
			adminCount := 0
			for _, u := range users {
				if u.Role == "admin" {
					adminCount++
				}
			}
			if adminCount <= 1 {
				respondJSONError(w, http.StatusBadRequest, "cannot delete last remaining admin")
				return
			}
		}

		if err := st.DeleteUser(targetUsername); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to delete user: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"message": "user deleted successfully"})
	}
}

