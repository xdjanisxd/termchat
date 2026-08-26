package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"termchat.local/termchat/internal/app"
	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/security"
	"termchat.local/termchat/internal/store"
)

type testUserRepository struct {
	users map[string]domain.User
}

func (r *testUserRepository) CreateUser(_ context.Context, user domain.User) error {
	if _, exists := r.users[user.Username]; exists {
		return store.ErrConflict
	}
	r.users[user.Username] = user
	return nil
}

func (r *testUserRepository) UserByUsername(_ context.Context, username string) (domain.User, error) {
	user, exists := r.users[username]
	if !exists {
		return domain.User{}, store.ErrNotFound
	}
	return user, nil
}

func newTestAuthHandler() http.Handler {
	hasher := security.NewPasswordHasher(security.Argon2Params{
		Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})
	tokens := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	service := app.NewAuthService(&testUserRepository{users: make(map[string]domain.User)}, hasher, tokens)
	return NewAuthHandler(service).Routes()
}

func TestAuthHandlerRegisterAndLogin(t *testing.T) {
	t.Parallel()

	handler := newTestAuthHandler()
	registerBody := []byte(`{"username":"alice","password":"long-enough-password"}`)
	registerRequest := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(registerBody))
	registerRequest.Header.Set("Content-Type", "application/json")
	registerResponse := httptest.NewRecorder()
	handler.ServeHTTP(registerResponse, registerRequest)
	if registerResponse.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", registerResponse.Code, registerResponse.Body.String())
	}
	var registered app.AuthResult
	if err := json.NewDecoder(registerResponse.Body).Decode(&registered); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	if registered.Token == "" || registered.User.Username != "alice" {
		t.Fatalf("register response = %#v", registered)
	}

	loginRequest := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(registerBody))
	loginRequest.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginResponse.Code, loginResponse.Body.String())
	}
}

func TestAuthHandlerRejectsDuplicateAndWrongPassword(t *testing.T) {
	t.Parallel()

	handler := newTestAuthHandler()
	body := []byte(`{"username":"alice","password":"long-enough-password"}`)
	for i, want := range []int{http.StatusCreated, http.StatusConflict} {
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != want {
			t.Fatalf("register attempt %d status = %d, want %d", i+1, response.Code, want)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader([]byte(`{"username":"alice","password":"wrong-password"}`)))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
