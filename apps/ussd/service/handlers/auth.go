package handlers

import (
	"context"
	"net/http"

	"github.com/pitabwire/frame/v2/security"
)

type contextKey string

const claimsContextKey contextKey = "ussd_claims"

// AuthMiddleware wraps an http.Handler and rejects requests that do not carry
// valid authentication claims in the context (set by Frame's authenticator).
// Gateway endpoints (telco-facing) use their own auth-key mechanism and are
// NOT wrapped by this middleware.
func AuthMiddleware(authenticator security.Authenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// If no authenticator is configured (e.g. RunServiceSecurely=false),
		// allow requests through (development mode).
		if authenticator == nil {
			next.ServeHTTP(w, r)
			return
		}

		claims := security.ClaimsFromContext(ctx)
		if claims == nil || claims.Subject == "" {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		// Store claims for downstream handlers to extract owner/tenant context
		ctx = context.WithValue(ctx, claimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ClaimsFromRequest extracts authentication claims from the request context.
// Returns nil if unauthenticated (e.g. development mode).
func ClaimsFromRequest(r *http.Request) *security.AuthenticationClaims {
	claims, _ := r.Context().Value(claimsContextKey).(*security.AuthenticationClaims)
	if claims != nil {
		return claims
	}
	// Fallback: check Frame's native claims context
	return security.ClaimsFromContext(r.Context())
}

// OwnerIDFromRequest extracts the owner ID (profile/subject) from auth claims.
// Returns empty string if unauthenticated.
func OwnerIDFromRequest(r *http.Request) string {
	claims := ClaimsFromRequest(r)
	if claims == nil {
		return ""
	}
	return claims.Subject
}
