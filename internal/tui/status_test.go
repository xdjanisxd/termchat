package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"termchat.local/termchat/internal/client"
	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/protocol"
)

func TestConnectionErrorStatusHasSemanticLabel(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenChat
	model.session.User.Username = "alice"
	model.room = &domain.PublicRoom{ID: "room-1", Name: "private_room"}
	updateModel(t, model, tea.WindowSizeMsg{Width: 80, Height: 24})

	updateModel(t, model, serverEventMsg{err: errors.New("offline")})

	plain := ansi.Strip(model.View())
	if !strings.Contains(plain, "[ERROR] Connection lost: offline") {
		t.Fatalf("View() missing semantic error status:\n%s", plain)
	}
}

func TestRoomJoinedStatusHasSemanticSuccessLabel(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.session.User.Username = "alice"
	updateModel(t, model, tea.WindowSizeMsg{Width: 80, Height: 24})
	room := domain.PublicRoom{ID: "room-1", Name: "private_room"}

	updateModel(t, model, serverEventMsg{event: protocol.ServerEvent{Type: "room_joined", Room: &room}})

	plain := ansi.Strip(model.View())
	if !strings.Contains(plain, "[OK] Joined private_room") {
		t.Fatalf("View() missing semantic success status:\n%s", plain)
	}
}

func TestNewStatusReplacesPreviousSemanticLevel(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenChat
	model.session.User.Username = "alice"
	model.room = &domain.PublicRoom{ID: "room-1", Name: "private_room"}
	updateModel(t, model, tea.WindowSizeMsg{Width: 100, Height: 24})
	updateModel(t, model, serverEventMsg{err: errors.New("offline")})

	updateModel(t, model, serverEventMsg{event: protocol.ServerEvent{Type: "user_joined", Username: "bob"}})

	plain := ansi.Strip(model.View())
	if !strings.Contains(plain, "[INFO] bob joined the room.") {
		t.Fatalf("View() did not replace prior error level with info:\n%s", plain)
	}
	if strings.Contains(plain, "[ERROR] bob joined the room.") {
		t.Fatalf("View() retained stale error level:\n%s", plain)
	}
}

func TestServerEventsAssignSemanticStatusLevels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		event     protocol.ServerEvent
		wantLevel statusLevel
		wantText  string
	}{
		{name: "error", event: protocol.ServerEvent{Type: "error", Error: &protocol.EventError{Message: "denied"}}, wantLevel: statusError, wantText: "denied"},
		{name: "room left", event: protocol.ServerEvent{Type: "room_left"}, wantLevel: statusSuccess, wantText: "Left the room."},
		{name: "room deleted", event: protocol.ServerEvent{Type: "room_deleted"}, wantLevel: statusWarning, wantText: "The room was deleted."},
		{name: "user list", event: protocol.ServerEvent{Type: "user_list", Users: []string{"alice", "bob"}}, wantLevel: statusInfo, wantText: "Online: alice, bob"},
		{name: "user joined", event: protocol.ServerEvent{Type: "user_joined", Username: "bob"}, wantLevel: statusInfo, wantText: "bob joined the room."},
		{name: "user left", event: protocol.ServerEvent{Type: "user_left", Username: "bob"}, wantLevel: statusWarning, wantText: "bob left the room."},
		{name: "password changed", event: protocol.ServerEvent{Type: "room_password_changed"}, wantLevel: statusSuccess, wantText: "Room password changed."},
		{name: "pong", event: protocol.ServerEvent{Type: "pong"}, wantLevel: statusSuccess, wantText: "Connected."},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := NewModel(nil)
			model.screen = ScreenChat
			model.room = &domain.PublicRoom{ID: "room-1", Name: "private_room"}
			model.setStatus(statusLevel(255), "stale status")

			model.applyServerEvent(test.event)

			if model.statusLevel != test.wantLevel {
				t.Fatalf("statusLevel = %d, want %d", model.statusLevel, test.wantLevel)
			}
			if model.status != test.wantText {
				t.Fatalf("status = %q, want %q", model.status, test.wantText)
			}
		})
	}
}

func TestModelMessagesAssignSemanticStatusLevels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		act       func(*testing.T, *Model)
		wantLevel statusLevel
		wantText  string
	}{

		{
			name: "connection failed", wantLevel: statusError, wantText: "Chat connection failed: offline",
			act: func(t *testing.T, model *Model) {
				updateModel(t, model, connectResultMsg{err: errors.New("offline")})
			},
		},
		{
			name: "connected", wantLevel: statusSuccess, wantText: "Connected as alice",
			act: func(t *testing.T, model *Model) {
				model.session.User.Username = "alice"
				updateModel(t, model, connectResultMsg{})
			},
		},
		{
			name: "authentication failed", wantLevel: statusError, wantText: "invalid credentials",
			act: func(t *testing.T, model *Model) {
				updateModel(t, model, authResultMsg{err: errors.New("invalid credentials")})
			},
		},
		{
			name: "event send failed", wantLevel: statusError, wantText: "send failed",
			act: func(t *testing.T, model *Model) {
				updateModel(t, model, eventSentMsg{err: errors.New("send failed")})
			},
		},
		{
			name: "credentials required", wantLevel: statusWarning, wantText: "Username and password are required.",
			act: func(t *testing.T, model *Model) {
				model.screen = ScreenLogin
				updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
			},
		},
		{
			name: "contacting server", wantLevel: statusInfo, wantText: "Contacting server...",
			act: func(t *testing.T, model *Model) {
				model.screen = ScreenLogin
				model.username.SetValue("alice")
				model.password.SetValue("not-a-real-password")
				updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
			},
		},
		{
			name: "invalid command", wantLevel: statusError, wantText: "invalid command: unknown command \"/unknown\"",
			act: func(t *testing.T, model *Model) {
				model.screen = ScreenChat
				model.commandInput.SetValue("/unknown")
				updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
			},
		},
		{
			name: "message before room", wantLevel: statusWarning, wantText: "Join a room before sending messages.",
			act: func(t *testing.T, model *Model) {
				model.screen = ScreenHome
				model.commandInput.SetValue("hello")
				updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
			},
		},
		{
			name: "room password prompt", wantLevel: statusInfo, wantText: "Enter the password for private_room",
			act: func(t *testing.T, model *Model) {
				model.screen = ScreenHome
				model.commandInput.SetValue("/join private_room")
				updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
			},
		},
		{
			name: "room password required", wantLevel: statusWarning, wantText: "Room password is required.",
			act: func(t *testing.T, model *Model) {
				model.screen = ScreenRoomPassword
				updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
			},
		},
		{
			name: "joining room", wantLevel: statusInfo, wantText: "Joining private_room...",
			act: func(t *testing.T, model *Model) {
				model.screen = ScreenRoomPassword
				model.pendingRoomName = "private_room"
				model.roomPassword.SetValue("not-a-real-room-password")
				updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := NewModel(nil)
			model.setStatus(statusLevel(255), "stale status")

			test.act(t, model)

			if model.statusLevel != test.wantLevel {
				t.Fatalf("statusLevel = %d, want %d", model.statusLevel, test.wantLevel)
			}
			if model.status != test.wantText {
				t.Fatalf("status = %q, want %q", model.status, test.wantText)
			}
		})
	}
}

func TestAuthenticationSuccessAssignsSuccessStatus(t *testing.T) {
	t.Parallel()

	api, err := client.New("http://example.test")
	if err != nil {
		t.Fatalf("client.New() error = %v", err)
	}
	model := NewModel(api)
	model.setStatus(statusLevel(255), "stale status")
	session := client.Session{User: domain.PublicUser{ID: "user-1", Username: "alice"}, Token: "token"}

	updateModel(t, model, authResultMsg{session: session})

	if model.statusLevel != statusSuccess {
		t.Fatalf("statusLevel = %d, want %d", model.statusLevel, statusSuccess)
	}
	if model.status != "Authenticated as alice" {
		t.Fatalf("status = %q, want %q", model.status, "Authenticated as alice")
	}
}

func TestOpeningAuthenticationClearsSemanticStatus(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.setStatus(statusError, "stale error")

	updateModel(t, model, keyRunes("l"))

	if model.status != "" {
		t.Fatalf("status = %q, want empty", model.status)
	}
	if model.statusLevel != statusInfo {
		t.Fatalf("statusLevel = %d, want %d", model.statusLevel, statusInfo)
	}
}

func TestAuthenticationViewShowsSemanticStatusLabel(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenLogin
	model.setStatus(statusError, "Invalid username or password.")

	plain := ansi.Strip(model.View())
	if !strings.Contains(plain, "[ERROR] Invalid username or password.") {
		t.Fatalf("View() missing semantic auth status:\n%s", plain)
	}
}
