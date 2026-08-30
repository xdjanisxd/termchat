package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"termchat.local/termchat/internal/app"
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

func TestRouterBlocksLockedIPExceptHealth(t *testing.T) {
	t.Parallel()

	tokens := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	token, err := tokens.Issue("user-1", "alice", time.Now().UTC())
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	attempts := app.NewAttemptLimiter(4, 5*time.Minute)
	guard := NewAttemptGuard(attempts, false)
	chat := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	router := NewRouter(newTestAuthHandler(), TokenMiddleware(tokens), chat, func(context.Context) error { return nil }, guard)

	remoteAddr := "192.0.2.30:1234"
	failedRequest := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
	failedRequest.RemoteAddr = remoteAddr
	for range 4 {
		guard.RecordIPFailure(failedRequest, time.Now().UTC())
	}

	meRequest := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	meRequest.RemoteAddr = remoteAddr
	meRequest.Header.Set("Authorization", "Bearer "+token)
	meResponse := httptest.NewRecorder()
	router.ServeHTTP(meResponse, meRequest)
	if meResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("locked IP current-user status = %d, want %d", meResponse.Code, http.StatusTooManyRequests)
	}

	healthRequest := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRequest.RemoteAddr = remoteAddr
	healthResponse := httptest.NewRecorder()
	router.ServeHTTP(healthResponse, healthRequest)
	if healthResponse.Code != http.StatusOK {
		t.Fatalf("locked IP health status = %d, want %d", healthResponse.Code, http.StatusOK)
	}
}

func TestRouterBlocksLockedIPOnUnmatchedRequests(t *testing.T) {
	t.Parallel()

	attempts := app.NewAttemptLimiter(4, 5*time.Minute)
	guard := NewAttemptGuard(attempts, false)
	remoteAddr := "192.0.2.31:1234"
	failedRequest := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
	failedRequest.RemoteAddr = remoteAddr
	for range 4 {
		guard.RecordIPFailure(failedRequest, time.Now().UTC())
	}
	router := NewRouter(newTestAuthHandler(), func(next http.Handler) http.Handler { return next }, http.NotFoundHandler(), func(context.Context) error { return nil }, guard)

	for _, test := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "unknown path", method: http.MethodGet, path: "/unknown"},
		{name: "unsupported method", method: http.MethodDelete, path: "/v1/auth/login"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			request.RemoteAddr = remoteAddr
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusTooManyRequests {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusTooManyRequests, response.Body.String())
			}
			var body errorResponse
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Error.Code != "RATE_LIMITED" || body.Error.Message != attemptRateLimitMessage {
				t.Fatalf("error = %#v", body.Error)
			}
		})
	}
}

func TestRouterBlocksHTTPRequestsForUserLockedByRoomFailures(t *testing.T) {
	t.Parallel()

	tokens := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	token, err := tokens.Issue("user-1", "alice", time.Now().UTC())
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	attempts := app.NewAttemptLimiter(4, 5*time.Minute)
	for range 4 {
		attempts.RecordFailure(userAttemptKey("user-1"), time.Now().UTC())
	}
	guard := NewAttemptGuard(attempts, false)
	chat := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	router := NewRouter(newTestAuthHandler(), TokenMiddleware(tokens), chat, func(context.Context) error { return nil }, guard)

	request := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	request.RemoteAddr = "192.0.2.40:1234"
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("locked user current-user status = %d, want %d", response.Code, http.StatusTooManyRequests)
	}
}
