package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/pitabwire/frame/v3/security"
	"github.com/pitabwire/util"
)

type contextKey string

const (
	claimsContextKey contextKey = "ussd_claims"
	devModeKey       contextKey = "ussd_dev_mode"
)

// devMode reports whether the request was let through without an
// authenticator (RunServiceSecurely=false), where ownership is not enforced.
func devMode(r *http.Request) bool {
	dev, _ := r.Context().Value(devModeKey).(bool)
	return dev
}

// AuthMiddleware admits internal service accounts only: the management API
// is how services register their menus and push what callers see, never a
// person's API. Gateway endpoints (telco-facing) use their own auth-key
// mechanism and are NOT wrapped by this middleware.
func AuthMiddleware(authenticator security.Authenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// If no authenticator is configured (e.g. RunServiceSecurely=false),
		// allow requests through (development mode).
		if authenticator == nil {
			next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, devModeKey, true)))
			return
		}

		claims := security.ClaimsFromContext(ctx)
		if claims == nil {
			// Verify the bearer token here rather than with frame's HTTP
			// middleware, which logs the raw Authorization header on every
			// failure; a token must never reach the logs.
			var err error
			if ctx, err = authenticateBearer(ctx, authenticator, r); err != nil {
				util.Log(ctx).WithError(err).Info("ussd management API: token not accepted")
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			claims = security.ClaimsFromContext(ctx)
		}
		if claims == nil || claims.Subject == "" {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if !claims.IsInternalSystem() {
			writeError(w, http.StatusForbidden, "only service accounts manage USSD services")
			return
		}

		// Store claims for downstream handlers to extract owner/tenant context
		ctx = context.WithValue(ctx, claimsContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// errBearerMissing is returned for a request without a bearer token.
var errBearerMissing = errors.New("no bearer token")

// authenticateBearer verifies the request's bearer token and returns the
// context carrying its claims. Nothing about the token is logged.
func authenticateBearer(
	ctx context.Context,
	authenticator security.Authenticator,
	r *http.Request,
) (context.Context, error) {
	scheme, token, found := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " ")
	token = strings.TrimSpace(token)
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return ctx, errBearerMissing
	}
	return authenticator.Authenticate(ctx, token)
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
