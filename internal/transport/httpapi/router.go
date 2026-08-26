package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"termchat.local/termchat/internal/domain"
)

type HealthCheck func(context.Context) error

func NewRouter(
	authRoutes http.Handler,
	authMiddleware func(http.Handler) http.Handler,
	chat http.Handler,
	health HealthCheck,
) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := health(ctx); err != nil {
			writeError(w, http.StatusServiceUnavailable, "UNHEALTHY", "Database is unavailable.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Post("/v1/auth/register", authRoutes.ServeHTTP)
	router.Post("/v1/auth/login", authRoutes.ServeHTTP)

	router.Group(func(protected chi.Router) {
		protected.Use(authMiddleware)
		protected.Get("/v1/users/me", func(w http.ResponseWriter, r *http.Request) {
			identity, ok := IdentityFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required.")
				return
			}
			writeJSON(w, http.StatusOK, domain.PublicUser{ID: identity.UserID, Username: identity.Username})
		})
		protected.Get("/v1/ws", chat.ServeHTTP)
	})
	return router
}
