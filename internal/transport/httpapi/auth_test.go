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

func (r *testUserRepository) DeleteUser(_ context.Context, userID string) error {
	for username, user := range r.users {
		if user.ID == userID {
			delete(r.users, username)
			return nil
		}
	}
	return store.ErrNotFound
}

func newTestAuthHandler() *AuthHandler {
	hasher := security.NewPasswordHasher(security.Argon2Params{
		Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})
	tokens := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	service := app.NewAuthService(&testUserRepository{users: make(map[string]domain.User)}, hasher, tokens)
	return NewAuthHandler(service)
}

func TestAuthHandlerDeleteAccountRemovesIdentityAndRunsDisconnect(t *testing.T) {
	t.Parallel()

	repository := &testUserRepository{users: map[string]domain.User{"alice": {ID: "user-1", Username: "alice"}}}
	service := app.NewAuthService(repository, security.NewPasswordHasher(security.Argon2Params{}), security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour))
	disconnected := ""
	handler := NewAuthHandler(service, func(userID string) { disconnected = userID })
	request := httptest.NewRequest(http.MethodDelete, "/v1/auth/account", nil)
	request = request.WithContext(context.WithValue(request.Context(), identityContextKey{}, Identity{UserID: "user-1", Username: "alice"}))
	response := httptest.NewRecorder()

	handler.DeleteAccount(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("delete account status = %d, body = %s", response.Code, response.Body.String())
	}
	if disconnected != "user-1" {
		t.Fatalf("disconnected user = %q, want user-1", disconnected)
	}
	if _, exists := repository.users["alice"]; exists {
		t.Fatal("account was not deleted")
	}
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

func TestAuthHandlerBlocksIPAfterFourFailedLogins(t *testing.T) {
	t.Parallel()

	hasher := security.NewPasswordHasher(security.Argon2Params{
		Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})
	tokens := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	service := app.NewAuthService(&testUserRepository{users: make(map[string]domain.User)}, hasher, tokens)
	attempts := app.NewAttemptLimiter(4, 5*time.Minute)
	guard := NewAttemptGuard(attempts, false)
	handler := guard.IPMiddleware(NewAuthHandler(service, guard).Routes())

	registerBody := []byte(`{"username":"alice","password":"long-enough-password"}`)
	registerRequest := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(registerBody))
	registerRequest.RemoteAddr = "192.0.2.10:1234"
	registerResponse := httptest.NewRecorder()
	handler.ServeHTTP(registerResponse, registerRequest)
	if registerResponse.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", registerResponse.Code, registerResponse.Body.String())
	}

	wrongBody := []byte(`{"username":"alice","password":"wrong-password"}`)
	for attempt := 1; attempt <= 4; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(wrongBody))
		request.RemoteAddr = "192.0.2.10:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("failed login %d status = %d, want %d", attempt, response.Code, http.StatusUnauthorized)
		}
	}

	blockedRequest := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(registerBody))
	blockedRequest.RemoteAddr = "192.0.2.10:1234"
	blockedResponse := httptest.NewRecorder()
	handler.ServeHTTP(blockedResponse, blockedRequest)
	if blockedResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked login status = %d, want %d", blockedResponse.Code, http.StatusTooManyRequests)
	}
	var response errorResponse
	if err := json.NewDecoder(blockedResponse.Body).Decode(&response); err != nil {
		t.Fatalf("decode blocked response: %v", err)
	}
	if response.Error.Code != "RATE_LIMITED" {
		t.Fatalf("blocked error code = %q, want RATE_LIMITED", response.Error.Code)
	}
}

func TestAuthHandlerSuccessfulLoginResetsIPFailures(t *testing.T) {
	t.Parallel()

	hasher := security.NewPasswordHasher(security.Argon2Params{
		Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})
	tokens := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	service := app.NewAuthService(&testUserRepository{users: make(map[string]domain.User)}, hasher, tokens)
	attempts := app.NewAttemptLimiter(4, 5*time.Minute)
	guard := NewAttemptGuard(attempts, false)
	handler := guard.IPMiddleware(NewAuthHandler(service, guard).Routes())
	remoteAddr := "192.0.2.20:1234"
	validBody := []byte(`{"username":"alice","password":"long-enough-password"}`)
	wrongBody := []byte(`{"username":"alice","password":"wrong-password"}`)

	request := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(validBody))
	request.RemoteAddr = remoteAddr
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("register status = %d", response.Code)
	}

	login := func(body []byte) int {
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(body))
		request.RemoteAddr = remoteAddr
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response.Code
	}
	for range 3 {
		if status := login(wrongBody); status != http.StatusUnauthorized {
			t.Fatalf("pre-reset failed login status = %d", status)
		}
	}
	if status := login(validBody); status != http.StatusOK {
		t.Fatalf("resetting login status = %d", status)
	}
	if status := login(wrongBody); status != http.StatusUnauthorized {
		t.Fatalf("post-reset failed login status = %d, want %d", status, http.StatusUnauthorized)
	}
	if status := login(validBody); status != http.StatusOK {
		t.Fatalf("post-reset valid login status = %d, want %d", status, http.StatusOK)
	}
}

func TestAuthHandlerRejectsLoginForUserLockedByRoomFailures(t *testing.T) {
	t.Parallel()

	hasher := security.NewPasswordHasher(security.Argon2Params{
		Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})
	tokens := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	service := app.NewAuthService(&testUserRepository{users: make(map[string]domain.User)}, hasher, tokens)
	attempts := app.NewAttemptLimiter(4, 5*time.Minute)
	guard := NewAttemptGuard(attempts, false)
	handler := guard.IPMiddleware(NewAuthHandler(service, guard).Routes())
	body := []byte(`{"username":"alice","password":"long-enough-password"}`)

	registerRequest := httptest.NewRequest(http.MethodPost, "/v1/auth/register", bytes.NewReader(body))
	registerRequest.RemoteAddr = "192.0.2.50:1234"
	registerResponse := httptest.NewRecorder()
	handler.ServeHTTP(registerResponse, registerRequest)
	if registerResponse.Code != http.StatusCreated {
		t.Fatalf("register status = %d", registerResponse.Code)
	}
	var registered app.AuthResult
	if err := json.NewDecoder(registerResponse.Body).Decode(&registered); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	for range 4 {
		attempts.RecordFailure(userAttemptKey(registered.User.ID), time.Now().UTC())
	}

	loginRequest := httptest.NewRequest(http.MethodPost, "/v1/auth/login", bytes.NewReader(body))
	loginRequest.RemoteAddr = "192.0.2.51:1234"
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("locked user login status = %d, want %d", loginResponse.Code, http.StatusTooManyRequests)
	}
}
