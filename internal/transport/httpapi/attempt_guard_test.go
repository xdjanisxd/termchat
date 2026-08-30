package httpapi

import (
	"net/http/httptest"
	"testing"
	"time"

	"termchat.local/termchat/internal/app"
)

func TestAttemptGuardUsesForwardedIPOnlyWhenTrusted(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest("GET", "/", nil)
	request.RemoteAddr = "10.0.0.5:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.5")
	attempts := app.NewAttemptLimiter(4, 5*time.Minute)

	if got := NewAttemptGuard(attempts, true).clientIP(request); got != "203.0.113.10" {
		t.Fatalf("trusted client IP = %q, want 203.0.113.10", got)
	}
	if got := NewAttemptGuard(attempts, false).clientIP(request); got != "10.0.0.5" {
		t.Fatalf("untrusted client IP = %q, want 10.0.0.5", got)
	}
}
