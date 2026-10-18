package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/podiax/pdswarm/internal/auth"
	"github.com/podiax/pdswarm/internal/store"
)

// TokenResponse is the safe representation of an APIToken without its hash.
type TokenResponse struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	CreatedBy  string     `json:"created_by"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

func toTokenResponse(tok *store.APIToken) TokenResponse {
	return TokenResponse{
		ID:         tok.ID,
		Name:       tok.Name,
		Prefix:     tok.Prefix,
		CreatedBy:  tok.CreatedBy,
		ExpiresAt:  tok.ExpiresAt,
		CreatedAt:  tok.CreatedAt,
		LastUsedAt: tok.LastUsedAt,
	}
}

// CreateTokenRequest defines parameters for generating an API token.
type CreateTokenRequest struct {
	Name        string `json:"name"`
	ExpiresDays int    `json:"expires_days"` // 0 = never expires
}

// CreateTokenResponse returns the raw token string (only shown once) along with metadata.
type CreateTokenResponse struct {
	Token     string        `json:"token"`
	TokenInfo TokenResponse `json:"token_info"`
}

// HandleListTokens returns all active API tokens.
func HandleListTokens(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "store not available")
			return
		}

		tokens, err := st.ListTokens()
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to list tokens: "+err.Error())
			return
		}

		resp := make([]TokenResponse, 0, len(tokens))
		for _, tok := range tokens {
			resp = append(resp, toTokenResponse(tok))
		}
		respondJSON(w, http.StatusOK, resp)
	}
}

// HandleCreateToken creates a new API token.
func HandleCreateToken(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "store not available")
			return
		}

		var req CreateTokenRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
			return
		}

		req.Name = strings.TrimSpace(req.Name)
		if req.Name == "" {
			respondJSONError(w, http.StatusBadRequest, "token name is required")
			return
		}

		createdBy := "admin"
		if claims, ok := r.Context().Value(auth.UserContextKey).(*auth.Claims); ok && claims != nil && claims.Username != "" {
			createdBy = claims.Username
		}

		rawToken, tokenHash, prefix, err := store.GenerateAPIToken()
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to generate token: "+err.Error())
			return
		}

		var expiresAt *time.Time
		if req.ExpiresDays > 0 {
			exp := time.Now().Add(time.Duration(req.ExpiresDays) * 24 * time.Hour)
			expiresAt = &exp
		}

		id := generateUUID()
		tok := &store.APIToken{
			ID:        id,
			Name:      req.Name,
			TokenHash: tokenHash,
			Prefix:    prefix,
			CreatedBy: createdBy,
			ExpiresAt: expiresAt,
			CreatedAt: time.Now(),
		}

		if err := st.SaveToken(tok); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to save token: "+err.Error())
			return
		}

		respondJSON(w, http.StatusCreated, CreateTokenResponse{
			Token:     rawToken,
			TokenInfo: toTokenResponse(tok),
		})
	}
}

// HandleDeleteToken revokes an API token.
func HandleDeleteToken(st store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if st == nil {
			respondJSONError(w, http.StatusServiceUnavailable, "store not available")
			return
		}

		id := strings.TrimSpace(chi.URLParam(r, "id"))
		if id == "" {
			respondJSONError(w, http.StatusBadRequest, "token id is required")
			return
		}

		if err := st.DeleteToken(id); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				respondJSONError(w, http.StatusNotFound, "token not found")
				return
			}
			respondJSONError(w, http.StatusInternalServerError, "failed to revoke token: "+err.Error())
			return
		}

		respondJSON(w, http.StatusOK, map[string]string{"message": "token revoked successfully"})
	}
}
