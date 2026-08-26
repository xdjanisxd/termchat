package tui

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"termchat.local/termchat/internal/client"
)

func TestModelRegisterFlowPersistsSession(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/auth/register" {
			t.Fatalf("request path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"user":{"id":"user-1","username":"alice"},"token":"jwt-token"}`))
	}))
	defer server.Close()
	api, err := client.New(server.URL)
	if err != nil {
		t.Fatalf("client.New() error = %v", err)
	}
	store := client.NewSessionStore(filepath.Join(t.TempDir(), "session.json"))
	model := NewModel(api, store)

	updateModel(t, model, keyRunes("r"))
	if model.Screen() != ScreenRegister {
		t.Fatalf("screen = %v, want ScreenRegister", model.Screen())
	}
	updateModel(t, model, keyRunes("alice"))
	updateModel(t, model, tea.KeyMsg{Type: tea.KeyTab})
	updateModel(t, model, keyRunes("long-enough-password"))
	if strings.Contains(model.View(), "long-enough-password") {
		t.Fatal("View() exposed the plaintext password")
	}
	command := updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil {
		t.Fatal("pressing Enter did not start registration")
	}
	message := command()
	updateModel(t, model, message)
	if model.Screen() != ScreenHome || model.Session().User.Username != "alice" {
		t.Fatalf("model after register: screen=%v session=%#v status=%q", model.Screen(), model.Session(), model.Status())
	}
	saved, err := store.Load()
	if err != nil || saved.Token != "jwt-token" {
		t.Fatalf("saved session = %#v, %v", saved, err)
	}
}

func TestModelShowsAuthenticationError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"INVALID_CREDENTIALS","message":"Invalid username or password."}}`))
	}))
	defer server.Close()
	api, _ := client.New(server.URL)
	model := NewModel(api, client.NewSessionStore(filepath.Join(t.TempDir(), "session.json")))
	updateModel(t, model, keyRunes("l"))
	updateModel(t, model, keyRunes("alice"))
	updateModel(t, model, tea.KeyMsg{Type: tea.KeyTab})
	updateModel(t, model, keyRunes("wrong-password"))
	command := updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	updateModel(t, model, command())
	if model.Screen() != ScreenLogin || !strings.Contains(model.Status(), "Invalid username or password") {
		t.Fatalf("screen=%v status=%q", model.Screen(), model.Status())
	}
}

func updateModel(t *testing.T, model *Model, message tea.Msg) tea.Cmd {
	t.Helper()
	updated, command := model.Update(message)
	if updated != model {
		t.Fatal("Update() replaced the model pointer")
	}
	return command
}

func keyRunes(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}
