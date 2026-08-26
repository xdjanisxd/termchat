package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"termchat.local/termchat/internal/security"
)

func TestTokenMiddlewareAuthenticatesBearerToken(t *testing.T) {
	t.Parallel()

	manager := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	token, err := manager.Issue("user-1", "alice", time.Now().UTC())
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	protected := TokenMiddleware(manager)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := IdentityFromContext(r.Context())
		if !ok || identity.UserID != "user-1" || identity.Username != "alice" {
			t.Fatalf("IdentityFromContext() = %#v, %v", identity, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("authenticated status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestTokenMiddlewareRejectsMissingToken(t *testing.T) {
	t.Parallel()

	manager := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	protected := TokenMiddleware(manager)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("protected handler was called without a token")
	}))
	response := httptest.NewRecorder()
	protected.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("missing token status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
