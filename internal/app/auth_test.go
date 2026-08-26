package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/security"
	"termchat.local/termchat/internal/store"
)

type fakeUserRepository struct {
	users map[string]domain.User
}

func (r *fakeUserRepository) CreateUser(_ context.Context, user domain.User) error {
	if _, exists := r.users[user.Username]; exists {
		return store.ErrConflict
	}
	r.users[user.Username] = user
	return nil
}

func (r *fakeUserRepository) UserByUsername(_ context.Context, username string) (domain.User, error) {
	user, exists := r.users[username]
	if !exists {
		return domain.User{}, store.ErrNotFound
	}
	return user, nil
}

func TestAuthServiceRegisterAndLogin(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 26, 15, 0, 0, 0, time.UTC)
	repo := &fakeUserRepository{users: make(map[string]domain.User)}
	hasher := security.NewPasswordHasher(security.Argon2Params{
		Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})
	tokens := security.NewTokenManager([]byte("01234567890123456789012345678901"), 24*time.Hour)
	service := NewAuthService(repo, hasher, tokens)

	registered, err := service.Register(context.Background(), "alice_01", "long-enough-password", now)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	stored := repo.users["alice_01"]
	if stored.PasswordHash == "long-enough-password" || stored.PasswordHash == "" {
		t.Fatal("Register() did not safely hash the password")
	}
	if registered.Token == "" || registered.User.Username != "alice_01" {
		t.Fatalf("Register() result = %#v", registered)
	}

	loggedIn, err := service.Login(context.Background(), "alice_01", "long-enough-password", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if loggedIn.Token == "" || loggedIn.User.ID != registered.User.ID {
		t.Fatalf("Login() result = %#v", loggedIn)
	}
}

func TestAuthServiceRejectsInvalidCredentials(t *testing.T) {
	t.Parallel()

	repo := &fakeUserRepository{users: make(map[string]domain.User)}
	hasher := security.NewPasswordHasher(security.Argon2Params{
		Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})
	tokens := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	service := NewAuthService(repo, hasher, tokens)
	now := time.Now().UTC()
	if _, err := service.Register(context.Background(), "alice", "long-enough-password", now); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if _, err := service.Login(context.Background(), "alice", "wrong-password", now); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}
