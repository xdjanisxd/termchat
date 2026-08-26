package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"termchat.local/termchat/internal/security"
)

type Identity struct {
	UserID   string
	Username string
}

type identityContextKey struct{}

func TokenMiddleware(tokens security.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parts := strings.Fields(r.Header.Get("Authorization"))
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "A valid Bearer token is required.")
				return
			}
			claims, err := tokens.Parse(parts[1], time.Now().UTC())
			if err != nil {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "A valid Bearer token is required.")
				return
			}
			identity := Identity{UserID: claims.UserID, Username: claims.Username}
			ctx := context.WithValue(r.Context(), identityContextKey{}, identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(Identity)
	return identity, ok
}
