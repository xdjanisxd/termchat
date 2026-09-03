package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientRegisterStoresSession(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/auth/register" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var credentials map[string]string
		if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if credentials["username"] != "alice" || credentials["password"] != "long-enough-password" {
			t.Fatalf("credentials = %#v", credentials)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"user":{"id":"user-1","username":"alice"},"token":"jwt-token"}`))
	}))
	defer server.Close()

	api, err := New(server.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	session, err := api.Register(context.Background(), "alice", "long-enough-password")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if session.Token != "jwt-token" || session.User.Username != "alice" || api.Token() != "jwt-token" {
		t.Fatalf("Register() session = %#v, token = %q", session, api.Token())
	}
}

func TestClientDeleteAccountExplainsMissingServerEndpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	api, err := New(server.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	api.SetToken("jwt-token")

	err = api.DeleteAccount(context.Background())
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Status != http.StatusNotFound || apiErr.Code != "ACCOUNT_DELETE_UNAVAILABLE" {
		t.Fatalf("DeleteAccount() error = %#v", err)
	}
}

func TestClientReturnsStructuredAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"INVALID_CREDENTIALS","message":"Invalid username or password."}}`))
	}))
	defer server.Close()
	api, err := New(server.URL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = api.Login(context.Background(), "alice", "wrong-password")
	apiErr, ok := err.(*APIError)
	if !ok || apiErr.Code != "INVALID_CREDENTIALS" || apiErr.Status != http.StatusUnauthorized {
		t.Fatalf("Login() error = %#v", err)
	}
}
