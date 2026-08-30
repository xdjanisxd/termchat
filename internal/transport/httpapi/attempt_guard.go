package httpapi

import (
	"net"
	"net/http"
	"strings"
	"time"

	"termchat.local/termchat/internal/app"
)

const attemptRateLimitMessage = "Too many failed attempts. Try again later."

type AttemptGuard struct {
	attempts          *app.AttemptLimiter
	trustProxyHeaders bool
}

func NewAttemptGuard(attempts *app.AttemptLimiter, trustProxyHeaders bool) *AttemptGuard {
	return &AttemptGuard{attempts: attempts, trustProxyHeaders: trustProxyHeaders}
}

func (g *AttemptGuard) IPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if g.attempts.IsBlocked(g.ipKey(r), time.Now().UTC()) {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", attemptRateLimitMessage)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (g *AttemptGuard) UserMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := IdentityFromContext(r.Context())
		if ok && g.attempts.IsBlocked(userAttemptKey(identity.UserID), time.Now().UTC()) {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", attemptRateLimitMessage)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (g *AttemptGuard) RecordIPFailure(r *http.Request, now time.Time) {
	g.attempts.RecordFailure(g.ipKey(r), now)
}

func (g *AttemptGuard) ResetIP(r *http.Request) {
	g.attempts.Reset(g.ipKey(r))
}

func (g *AttemptGuard) IsUserBlocked(userID string, now time.Time) bool {
	return g.attempts.IsBlocked(userAttemptKey(userID), now)
}

func (g *AttemptGuard) isIPBlocked(clientIP string, now time.Time) bool {
	return g.attempts.IsBlocked("ip:"+clientIP, now)
}

func (g *AttemptGuard) ipKey(r *http.Request) string {
	return "ip:" + g.clientIP(r)
}

func userAttemptKey(userID string) string {
	return "user:" + userID
}

func (g *AttemptGuard) clientIP(r *http.Request) string {
	if g.trustProxyHeaders {
		forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
		if net.ParseIP(forwarded) != nil {
			return forwarded
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
