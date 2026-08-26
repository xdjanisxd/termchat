package tui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"termchat.local/termchat/internal/client"
	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/protocol"
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

func TestModelRestoresSavedSessionAndConnects(t *testing.T) {
	t.Parallel()

	connected := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/ws" {
			http.NotFound(w, r)
			return
		}
		connected <- r.Header.Get("Authorization")
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "test complete")
		_, _, _ = conn.Read(r.Context())
	}))
	defer server.Close()
	api, _ := client.New(server.URL)
	store := client.NewSessionStore(filepath.Join(t.TempDir(), "session.json"))
	session := client.Session{Token: "saved-token"}
	session.User.ID = "user-1"
	session.User.Username = "alice"
	if err := store.Save(session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	model := NewModel(api, store)
	command := model.Init()
	if command == nil {
		t.Fatal("Init() did not return a session restore command")
	}
	updateModel(t, model, command())
	defer api.Disconnect()
	if model.Screen() != ScreenHome || model.Session() != session {
		t.Fatalf("restored screen=%v session=%#v status=%q", model.Screen(), model.Session(), model.Status())
	}
	select {
	case authorization := <-connected:
		if authorization != "Bearer saved-token" {
			t.Fatalf("Authorization = %q", authorization)
		}
	default:
		t.Fatal("saved session did not establish a websocket connection")
	}
}

func TestModelJoinUsesMaskedPasswordPromptAndOpensChat(t *testing.T) {
	t.Parallel()

	received := make(chan protocol.ClientEvent, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "test complete")
		var event protocol.ClientEvent
		if err := wsjson.Read(r.Context(), conn, &event); err != nil {
			return
		}
		received <- event
		room := domain.PublicRoom{ID: "room-1", Name: "private_room"}
		_ = wsjson.Write(r.Context(), conn, protocol.ServerEvent{Type: "room_joined", RequestID: event.RequestID, Room: &room})
		_, _, _ = conn.Read(r.Context())
	}))
	defer server.Close()
	api, _ := client.New(server.URL)
	api.SetToken("jwt-token")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := api.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer api.Disconnect()

	model := NewModel(api, client.NewSessionStore(filepath.Join(t.TempDir(), "session.json")))
	model.screen = ScreenHome
	model.session.Token = "jwt-token"
	model.session.User.ID = "user-1"
	model.session.User.Username = "alice"
	updateModel(t, model, keyRunes("/join private_room"))
	updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	if model.Screen() != ScreenRoomPassword {
		t.Fatalf("screen = %v, want ScreenRoomPassword", model.Screen())
	}
	updateModel(t, model, keyRunes("roompass"))
	if strings.Contains(model.View(), "roompass") {
		t.Fatal("room password prompt exposed plaintext")
	}
	sendCommand := updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	if sendCommand == nil {
		t.Fatal("room password submit did not return a send command")
	}
	updateModel(t, model, sendCommand())
	select {
	case event := <-received:
		if event.Type != "join_room" || event.RoomName != "private_room" || event.Password != "roompass" {
			t.Fatalf("join event = %#v", event)
		}
	case <-ctx.Done():
		t.Fatal("server did not receive join_room event")
	}
	listenCommand := model.listenCmd()
	updateModel(t, model, listenCommand())
	if model.Screen() != ScreenChat || model.CurrentRoom() == nil || model.CurrentRoom().Name != "private_room" {
		t.Fatalf("screen=%v room=%#v status=%q", model.Screen(), model.CurrentRoom(), model.Status())
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
