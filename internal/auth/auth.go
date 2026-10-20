package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/podiax/pdswarm/internal/audit"
	"github.com/podiax/pdswarm/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const UserContextKey = contextKey("user")

// TokenValidator defines the contract for validating API tokens and looking up users.
type TokenValidator interface {
	ValidateToken(rawToken string) (*store.APIToken, error)
	GetUser(username string) (*store.User, error)
}

type Claims struct {
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func GenerateJWT(username, role, secret string, ttl time.Duration) (string, error) {
	claims := Claims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ValidateJWT(tokenStr, secret string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func AuthMiddleware(secret string, noAuth bool, tokenValidator ...TokenValidator) func(next http.Handler) http.Handler {
	var validator TokenValidator
	if len(tokenValidator) > 0 {
		validator = tokenValidator[0]
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if noAuth {
				next.ServeHTTP(w, r)
				return
			}

			var tokenStr string
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			} else {
				cookie, err := r.Cookie("pdswarm_token")
				if err == nil {
					tokenStr = cookie.Value
				}
			}

			if tokenStr == "" {
				tokenStr = r.URL.Query().Get("token")
			}

			if tokenStr == "" {
				audit.Default().LogUnauthorized(audit.ClientIP(r), r.URL.Path, "missing authentication token")
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			// Support API tokens (format: pds_...)
			if strings.HasPrefix(tokenStr, "pds_") {
				if validator == nil {
					audit.Default().LogUnauthorized(audit.ClientIP(r), r.URL.Path, "api token validator not configured")
					http.Error(w, `{"error":"api token authentication not configured"}`, http.StatusUnauthorized)
					return
				}
				tok, err := validator.ValidateToken(tokenStr)
				if err != nil {
					audit.Default().LogUnauthorized(audit.ClientIP(r), r.URL.Path, "invalid or expired api token")
					http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
					return
				}
				userRole := "admin"
				if validator != nil {
					if u, err := validator.GetUser(tok.CreatedBy); err == nil {
						userRole = u.Role
					}
				}
				
				claims := &Claims{
					Username: tok.CreatedBy,
					Role:     userRole,
				}
				ctx := context.WithValue(r.Context(), UserContextKey, claims)
				ctx = context.WithValue(ctx, contextKey("store_user"), &store.User{
					Username: tok.CreatedBy,
					Role:     userRole,
				})
				
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			claims, err := ValidateJWT(tokenStr, secret)
			if err != nil {
				audit.Default().LogUnauthorized(audit.ClientIP(r), r.URL.Path, "invalid or expired jwt token")
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
