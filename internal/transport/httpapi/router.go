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

type AccountDeletionHandler interface {
	DeleteAccount(http.ResponseWriter, *http.Request)
}

func NewRouter(
	authRoutes http.Handler,
	authMiddleware func(http.Handler) http.Handler,
	chat http.Handler,
	health HealthCheck,
	attemptGuards ...*AttemptGuard,
) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	var attempts *AttemptGuard
	if len(attemptGuards) > 0 {
		attempts = attemptGuards[0]
	}

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := health(ctx); err != nil {
			writeError(w, http.StatusServiceUnavailable, "UNHEALTHY", "Database is unavailable.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Group(func(public chi.Router) {
		public.Post("/v1/auth/register", authRoutes.ServeHTTP)
		public.Post("/v1/auth/login", authRoutes.ServeHTTP)
	})

	router.Group(func(protected chi.Router) {
		protected.Use(authMiddleware)
		if attempts != nil {
			protected.Use(attempts.UserMiddleware)
		}
		if accountDeletion, ok := authRoutes.(AccountDeletionHandler); ok {
			protected.Delete("/v1/auth/account", accountDeletion.DeleteAccount)
		}
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
	if attempts != nil {
		guarded := attempts.IPMiddleware(router)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/healthz" {
				router.ServeHTTP(w, r)
				return
			}
			guarded.ServeHTTP(w, r)
		})
	}
	return router
}
