package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"termchat.local/termchat/internal/security"
)

func TestRouterHealthAndCurrentUser(t *testing.T) {
	t.Parallel()

	tokens := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	token, err := tokens.Issue("user-1", "alice", time.Now().UTC())
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	authRoutes := newTestAuthHandler()
	chat := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	router := NewRouter(authRoutes, TokenMiddleware(tokens), chat, func(context.Context) error { return nil })

	healthResponse := httptest.NewRecorder()
	router.ServeHTTP(healthResponse, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if healthResponse.Code != http.StatusOK {
		t.Fatalf("health status = %d, body = %s", healthResponse.Code, healthResponse.Body.String())
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	meRequest.Header.Set("Authorization", "Bearer "+token)
	meResponse := httptest.NewRecorder()
	router.ServeHTTP(meResponse, meRequest)
	if meResponse.Code != http.StatusOK || meResponse.Body.String() != "{\"id\":\"user-1\",\"username\":\"alice\"}\n" {
		t.Fatalf("me status = %d, body = %s", meResponse.Code, meResponse.Body.String())
	}
}

func TestRouterProtectsWebSocket(t *testing.T) {
	t.Parallel()

	tokens := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	chat := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("chat handler called without authentication")
	})
	router := NewRouter(newTestAuthHandler(), TokenMiddleware(tokens), chat, func(context.Context) error { return nil })
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/ws", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("websocket status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
