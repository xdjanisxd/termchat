package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"termchat.local/termchat/internal/app"
	"termchat.local/termchat/internal/config"
	"termchat.local/termchat/internal/observability"
	"termchat.local/termchat/internal/security"
	postgresstore "termchat.local/termchat/internal/store/postgres"
	"termchat.local/termchat/internal/transport/httpapi"
)

func main() {
	logger := slog.New(observability.NewRedactingHandler(slog.NewTextHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv, logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, logger *slog.Logger) error {
	serverConfig, err := config.LoadServer(getenv)
	if err != nil {
		return fmt.Errorf("load server config: %w", err)
	}

	pool, err := pgxpool.New(ctx, serverConfig.DatabaseURL)
	if err != nil {
		return fmt.Errorf("create postgres pool: %w", err)
	}
	defer pool.Close()
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	err = pool.Ping(pingCtx)
	pingCancel()
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}

	repository := postgresstore.New(pool)
	hasher := security.NewPasswordHasher(security.DefaultArgon2Params())
	tokens := security.NewTokenManager(serverConfig.JWTSecret, serverConfig.TokenTTL)
	authService := app.NewAuthService(repository, hasher, tokens)
	roomService := app.NewRoomService(repository, hasher)
	messageService := app.NewMessageService(repository)
	attempts := app.NewAttemptLimiter(4, 5*time.Minute)
	attemptGuard := httpapi.NewAttemptGuard(attempts, serverConfig.TrustProxyHeaders)
	chatHandler := httpapi.NewChatHandler(roomService, messageService, attemptGuard)
	authHandler := httpapi.NewAuthHandler(authService, attemptGuard, chatHandler.DisconnectUser)
	router := httpapi.NewRouter(authHandler, httpapi.TokenMiddleware(tokens), chatHandler, pool.Ping, attemptGuard)

	workerCtx, stopWorker := context.WithCancel(ctx)
	defer stopWorker()
	go app.RunCleanupWorker(workerCtx, repository, serverConfig.CleanupInterval, func(deleted int64, cleanupErr error) {
		if cleanupErr != nil {
			logger.Error("message cleanup failed", "error", cleanupErr)
		} else if deleted > 0 {
			logger.Info("expired messages deleted", "count", deleted)
		}
	})

	httpServer := &http.Server{
		Addr:              serverConfig.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}
	errCh := make(chan error, 1)
	go func() {
		logger.Info("termchat server listening", "address", serverConfig.ListenAddr)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case serveErr := <-errCh:
		if errors.Is(serveErr, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", serveErr)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		return nil
	}
}
