package api

import (
	"net/http"

	"github.com/podiax/pdswarm/internal/auth"
)

// RequireRole enforces that the authenticated user has one of the required roles.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(auth.UserContextKey).(*auth.Claims)
			if !ok || claims == nil {
				// If no authentication is provided (e.g. noAuth flag), assume full access.
				next.ServeHTTP(w, r)
				return
			}

			userRole := claims.Role
			// Treat empty role as viewer (safest default)
			if userRole == "" {
				userRole = "viewer"
			}

			hasAccess := false
			for _, allowedRole := range roles {
				if userRole == allowedRole {
					hasAccess = true
					break
				}
			}

			if !hasAccess {
				respondJSONError(w, http.StatusForbidden, "insufficient privileges for this action")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}