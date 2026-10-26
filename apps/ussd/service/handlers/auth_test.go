package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pitabwire/frame/v3/security"
	"github.com/stretchr/testify/require"
)

type stubAuthenticator struct{ security.Authenticator }

func TestManagementAPIAdmitsServiceAccountsOnly(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	handler := AuthMiddleware(stubAuthenticator{}, ok)

	tests := []struct {
		name   string
		claims *security.AuthenticationClaims
		want   int
	}{
		{"anonymous", nil, http.StatusUnauthorized},
		{"a person", &security.AuthenticationClaims{
			Roles:            []string{"user"},
			RegisteredClaims: jwt.RegisteredClaims{Subject: "profile-1"},
		}, http.StatusForbidden},
		{"a service account", &security.AuthenticationClaims{
			Roles:            []string{security.ConstantSystemInternalRole},
			RegisteredClaims: jwt.RegisteredClaims{Subject: "service_seed"},
		}, http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/menus", nil)
			if tc.claims != nil {
				req = req.WithContext(tc.claims.ClaimsToContext(req.Context()))
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			require.Equal(t, tc.want, rec.Code)
		})
	}
}
