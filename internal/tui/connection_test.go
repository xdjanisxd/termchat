package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"termchat.local/termchat/internal/client"
	"termchat.local/termchat/internal/domain"
)

func TestHomeViewShowsOfflineBeforeChatConnection(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, client.SessionStore{})
	model.screen = ScreenHome
	model.session.User.Username = "alice"

	plain := ansi.Strip(model.View())
	if !strings.Contains(plain, "[OFFLINE]") {
		t.Fatalf("View() missing offline connection state:\n%s", plain)
	}
}

func TestAuthenticationSuccessShowsConnectingWhileChatConnects(t *testing.T) {
	t.Parallel()

	api, err := client.New("http://example.test")
	if err != nil {
		t.Fatalf("client.New() error = %v", err)
	}
	store := client.NewSessionStore(filepath.Join(t.TempDir(), "session.json"))
	model := NewModel(api, store)
	session := client.Session{Token: "token"}
	session.User.ID = "user-1"
	session.User.Username = "alice"

	command := updateModel(t, model, authResultMsg{session: session})

	if command == nil {
		t.Fatal("authentication success did not start chat connection")
	}
	if model.connectionState != connectionConnecting {
		t.Fatalf("connectionState = %d, want %d", model.connectionState, connectionConnecting)
	}
	plain := ansi.Strip(model.View())
	if !strings.Contains(plain, "[CONNECTING]") {
		t.Fatalf("View() missing connecting state:\n%s", plain)
	}
}

func TestSuccessfulChatConnectionShowsOnlineAcrossViews(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, client.SessionStore{})
	model.screen = ScreenHome
	model.session.User.Username = "alice"
	model.connectionState = connectionConnecting

	updateModel(t, model, connectResultMsg{})

	if model.connectionState != connectionOnline {
		t.Fatalf("connectionState = %d, want %d", model.connectionState, connectionOnline)
	}
	if plain := ansi.Strip(model.View()); !strings.Contains(plain, "[ONLINE]") {
		t.Fatalf("Home View() missing online state:\n%s", plain)
	}
	model.screen = ScreenChat
	if plain := ansi.Strip(model.View()); !strings.Contains(plain, "[ONLINE] alice") {
		t.Fatalf("Chat View() missing online state:\n%s", plain)
	}
}

func TestFailedChatConnectionReturnsOfflineWithoutRetry(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, client.SessionStore{})
	model.screen = ScreenHome
	model.session.User.Username = "alice"
	model.connectionState = connectionConnecting

	command := updateModel(t, model, connectResultMsg{err: errors.New("unreachable")})

	if command != nil {
		t.Fatal("failed connection unexpectedly scheduled a retry")
	}
	if model.connectionState != connectionOffline {
		t.Fatalf("connectionState = %d, want %d", model.connectionState, connectionOffline)
	}
	plain := ansi.Strip(model.View())
	if !strings.Contains(plain, "[OFFLINE]") {
		t.Fatalf("View() missing offline state after connection failure:\n%s", plain)
	}
}

func TestLostChatConnectionShowsOfflineWithoutReconnect(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, client.SessionStore{})
	model.screen = ScreenChat
	model.session.User.Username = "alice"
	model.connectionState = connectionOnline

	command := updateModel(t, model, serverEventMsg{err: errors.New("connection reset")})

	if command != nil {
		t.Fatal("lost connection unexpectedly scheduled reconnect")
	}
	if model.connectionState != connectionOffline {
		t.Fatalf("connectionState = %d, want %d", model.connectionState, connectionOffline)
	}
	plain := ansi.Strip(model.View())
	if !strings.Contains(plain, "[OFFLINE] alice") {
		t.Fatalf("Chat View() missing offline state after disconnect:\n%s", plain)
	}
}

func TestSuccessfulSessionRestoreShowsOnline(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, client.SessionStore{})
	session := client.Session{Token: "token"}
	session.User.ID = "user-1"
	session.User.Username = "alice"

	command := updateModel(t, model, sessionRestoreMsg{session: session})

	if command == nil {
		t.Fatal("successful session restore did not start event listener")
	}
	if model.connectionState != connectionOnline {
		t.Fatalf("connectionState = %d, want %d", model.connectionState, connectionOnline)
	}
	plain := ansi.Strip(model.View())
	if !strings.Contains(plain, "[ONLINE]") {
		t.Fatalf("View() missing online state after session restore:\n%s", plain)
	}
}

func TestConnectionStatesUseSemanticColors(t *testing.T) {
	t.Parallel()

	theme := newAmberCRTTheme()
	tests := []struct {
		name string
		got  lipgloss.TerminalColor
		want lipgloss.AdaptiveColor
	}{
		{name: "online green", got: theme.connOnline.GetForeground(), want: lipgloss.AdaptiveColor{Light: "#1F7A3A", Dark: "#7DFF91"}},
		{name: "offline red", got: theme.connOffline.GetForeground(), want: lipgloss.AdaptiveColor{Light: "#B42318", Dark: "#FF6B5E"}},
		{name: "connecting amber", got: theme.connPending.GetForeground(), want: lipgloss.AdaptiveColor{Light: "#663A00", Dark: "#FFD27A"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got, want := fmt.Sprint(test.got), fmt.Sprint(test.want); got != want {
				t.Fatalf("foreground = %q, want %q", got, want)
			}
		})
	}
}

func TestChatHeaderPreservesConnectionStateAtSupportedWidths(t *testing.T) {
	t.Parallel()

	states := []struct {
		name  string
		state connectionState
		badge string
	}{
		{name: "offline", state: connectionOffline, badge: "[OFFLINE]"},
		{name: "connecting", state: connectionConnecting, badge: "[CONNECTING]"},
		{name: "online", state: connectionOnline, badge: "[ONLINE]"},
	}
	sizes := []struct {
		name     string
		width    int
		room     string
		username string
	}{
		{name: "minimum width", width: minimumChatWidth, room: "private_room", username: "alice"},
		{name: "maximum names at common width", width: 80, room: strings.Repeat("r", 32), username: strings.Repeat("u", 24)},
	}

	for _, state := range states {
		for _, size := range sizes {
			t.Run(state.name+"/"+size.name, func(t *testing.T) {
				model := NewModel(nil, client.SessionStore{})
				model.connectionState = state.state
				model.session.User.Username = size.username
				model.room = &domain.PublicRoom{Name: size.room}

				header := model.renderChatHeader(size.width)
				if plain := ansi.Strip(header); !strings.Contains(plain, state.badge) {
					t.Fatalf("renderChatHeader(%d) missing %s:\n%s", size.width, state.badge, plain)
				}
				if got := lipgloss.Width(header); got > size.width {
					t.Fatalf("renderChatHeader(%d) width = %d, want <= %d", size.width, got, size.width)
				}
			})
		}
	}
}
