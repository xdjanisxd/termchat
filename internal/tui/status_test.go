package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

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
	if !strings.Contains(plain, "[WARN] Connection lost: offline") {
		t.Fatalf("View() missing semantic reconnect status:\n%s", plain)
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

func TestDirectInviteBannerSurvivesHeartbeatPong(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	model.session.User.Username = "alice"
	updateModel(t, model, tea.WindowSizeMsg{Width: 80, Height: 24})
	invite := protocol.ServerEvent{Type: "direct_invite_received", InviteID: "invite-1", Counterpart: &protocol.DirectIdentity{UserID: "bob-id", Username: "bob"}}

	updateModel(t, model, serverEventMsg{event: invite})
	updateModel(t, model, serverEventMsg{event: protocol.ServerEvent{Type: "pong", RequestID: "heartbeat-1"}})

	plain := ansi.Strip(model.View())
	if !strings.Contains(plain, "[INVITE] Direct invitation from bob") || !strings.Contains(plain, "/accept to join · /decline to refuse") {
		t.Fatalf("View() lost the direct invite action banner after pong:\n%s", plain)
	}
	if strings.Contains(plain, "[OK] Connected.") {
		t.Fatalf("View() surfaced a successful heartbeat as a notification:\n%s", plain)
	}
}

func TestStatusToastUsesUpperRightOverlayAndExpires(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	updateModel(t, model, tea.WindowSizeMsg{Width: 100, Height: 24})
	model.setStatus(statusInfo, "Contacting server...")

	plain := ansi.Strip(model.View())
	lines := strings.Split(plain, "\n")
	if got := strings.Index(lines[chatHeaderHeight], "[INFO] Contacting server..."); got < 55 {
		t.Fatalf("toast is not right aligned: offset=%d line=%q", got, lines[chatHeaderHeight])
	}
	if got := model.notificationTrayHeight(model.width); got != 1 {
		t.Fatalf("toast height = %d, want 1", got)
	}

	updateModel(t, model, notificationExpiredMsg{id: model.activeNotification.id})
	if strings.Contains(ansi.Strip(model.View()), "[INFO] Contacting server...") {
		t.Fatal("expired info toast remained visible")
	}
}

func TestStatusToastTickerExpiresElapsedToast(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	updateModel(t, model, tea.WindowSizeMsg{Width: 100, Height: 24})
	model.setStatus(statusSuccess, "Connected as test")
	model.activeNotification.expiresAt = time.Now().Add(-time.Second)

	command := updateModel(t, model, statusToastTickMsg{})
	if command == nil {
		t.Fatal("toast tick did not schedule the next expiry check")
	}
	if strings.Contains(ansi.Strip(model.View()), "[OK] Connected as test") {
		t.Fatal("elapsed success toast remained visible after ticker update")
	}
}

func TestStatusToastReplacesEarlierNormalToast(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	updateModel(t, model, tea.WindowSizeMsg{Width: 100, Height: 24})
	model.setStatus(statusInfo, "Contacting server...")
	model.setStatus(statusSuccess, "Authenticated as test")
	model.setStatus(statusSuccess, "Connected as test")

	plain := ansi.Strip(model.View())
	if strings.Contains(plain, "Contacting server...") || strings.Contains(plain, "Authenticated as test") || !strings.Contains(plain, "[OK] Connected as test") {
		t.Fatalf("normal toast was not replaced by the latest status:\n%s", plain)
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
			name: "connection failed", wantLevel: statusWarning, wantText: "Chat connection failed: offline Retrying shortly...",
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
			name: "event send failed", wantLevel: statusWarning, wantText: "Could not send chat event: send failed Retrying shortly...",
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
			name: "invalid command", wantLevel: statusError, wantText: "Unknown command \"/unknown\". Type /help to see available commands.",
			act: func(t *testing.T, model *Model) {
				model.screen = ScreenChat
				model.commandInput.SetValue("/unknown")
				updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
			},
		},
		{
			name: "message before room", wantLevel: statusWarning, wantText: "You are not in a room. Join one with /join <room-name>, or create one with /createroom <room-name> <password>.",
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
