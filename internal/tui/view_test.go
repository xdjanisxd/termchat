package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/protocol"
)

func TestChatViewUsesResponsiveContentFirstLayout(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenChat
	model.session.User.ID = "user-1"
	model.session.User.Username = "alice"
	model.connectionState = connectionOnline
	model.room = &domain.PublicRoom{ID: "room-1", Name: "private_room"}
	model.messages = []domain.Message{{
		ID: "message-1", RoomID: "room-1", UserID: "user-2", Username: "bob",
		Content: "hello from the viewport", CreatedAt: time.Date(2026, 8, 27, 21, 4, 0, 0, time.Local),
	}}
	updateModel(t, model, tea.WindowSizeMsg{Width: 80, Height: 24})

	view := model.View()
	plain := ansi.Strip(view)
	for _, marker := range []string{
		"TERMCHAT // #private_room",
		"[ONLINE] alice",
		"hello from the viewport",
		"Type a message or /help",
		"PgUp/PgDn scroll",
	} {
		if !strings.Contains(plain, marker) {
			t.Fatalf("View() missing %q:\n%s", marker, plain)
		}
	}
	if got := lipgloss.Width(view); got > 80 {
		t.Fatalf("View() width = %d, want <= 80", got)
	}
	if got := lipgloss.Height(view); got != 24 {
		t.Fatalf("View() height = %d, want 24", got)
	}
}

func TestChatViewportKeepsAmberBackgroundAfterRenderingMessages(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenChat
	model.room = &domain.PublicRoom{ID: "room-1", Name: "private_room"}
	model.messages = []domain.Message{{
		ID: "message-1", RoomID: "room-1", UserID: "user-2", Username: "bob",
		Content: "message on the amber background", CreatedAt: time.Date(2026, 8, 27, 21, 4, 0, 0, time.Local),
	}}
	updateModel(t, model, tea.WindowSizeMsg{Width: 80, Height: 24})
	_ = model.View()

	if got, want := fmt.Sprint(model.viewport.Style.GetBackground()), fmt.Sprint(model.theme.root.GetBackground()); got != want {
		t.Fatalf("viewport background = %q, want root background %q", got, want)
	}
}

func TestRenderedMessageLinesFillTheViewportWidth(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenChat
	model.room = &domain.PublicRoom{ID: "room-1", Name: "private_room"}
	model.messages = []domain.Message{{
		ID: "message-1", RoomID: "room-1", UserID: "user-2", Username: "bob",
		Content: "short message", CreatedAt: time.Date(2026, 8, 27, 21, 4, 0, 0, time.Local),
	}}
	updateModel(t, model, tea.WindowSizeMsg{Width: 80, Height: 24})

	for _, line := range strings.Split(model.renderMessages(), "\n") {
		if got := lipgloss.Width(line); got != 80 {
			t.Fatalf("rendered message line width = %d, want viewport width 80", got)
		}
	}
}

func TestNonChatScreenRowsFillTheTerminalWidth(t *testing.T) {
	t.Parallel()

	const width = 80
	for _, screen := range []Screen{ScreenWelcome, ScreenLogin, ScreenRegister, ScreenRoomPassword} {
		model := NewModel(nil)
		model.screen = screen
		model.pendingRoomName = "private_room"
		updateModel(t, model, tea.WindowSizeMsg{Width: width, Height: 24})

		for _, line := range strings.Split(model.View(), "\n") {
			if got := lipgloss.Width(line); got != width {
				t.Fatalf("screen %d rendered row width = %d, want terminal width %d", screen, got, width)
			}
		}
	}
}

func TestChatViewportScrollsWithoutChangingComposer(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenChat
	model.session.User.ID = "user-1"
	model.session.User.Username = "alice"
	model.room = &domain.PublicRoom{ID: "room-1", Name: "private_room"}
	for index := range 40 {
		model.messages = append(model.messages, domain.Message{
			ID: "message", RoomID: "room-1", UserID: "user-2", Username: "bob",
			Content: "message line", CreatedAt: time.Date(2026, 8, 27, 21, index, 0, 0, time.Local),
		})
	}
	updateModel(t, model, tea.WindowSizeMsg{Width: 60, Height: 10})
	_ = model.View()
	before := model.viewport.YOffset
	if before == 0 {
		t.Fatal("test setup did not produce scrollable viewport content")
	}

	updateModel(t, model, tea.KeyMsg{Type: tea.KeyPgUp})

	if model.viewport.YOffset >= before {
		t.Fatalf("viewport offset after PageUp = %d, want less than %d", model.viewport.YOffset, before)
	}
	if model.commandInput.Value() != "" {
		t.Fatalf("composer changed during viewport scroll: %q", model.commandInput.Value())
	}
}

func TestChatViewportPageDownDoesNotChangeComposer(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenChat
	model.session.User.ID = "user-1"
	model.session.User.Username = "alice"
	model.room = &domain.PublicRoom{ID: "room-1", Name: "private_room"}
	for index := range 40 {
		model.messages = append(model.messages, domain.Message{
			ID: "message", RoomID: "room-1", UserID: "user-2", Username: "bob",
			Content: "message line", CreatedAt: time.Date(2026, 8, 27, 21, index, 0, 0, time.Local),
		})
	}
	updateModel(t, model, tea.WindowSizeMsg{Width: 60, Height: 10})
	_ = model.View()
	updateModel(t, model, tea.KeyMsg{Type: tea.KeyPgUp})
	before := model.viewport.YOffset

	updateModel(t, model, tea.KeyMsg{Type: tea.KeyPgDown})

	if model.viewport.YOffset <= before {
		t.Fatalf("viewport offset after PageDown = %d, want greater than %d", model.viewport.YOffset, before)
	}
	if model.commandInput.Value() != "" {
		t.Fatalf("composer changed during viewport scroll: %q", model.commandInput.Value())
	}
}

func TestChatViewUsesFallbackWhenTerminalIsTooSmall(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenChat
	model.session.User.Username = "alice"
	model.room = &domain.PublicRoom{ID: "room-1", Name: "private_room"}
	updateModel(t, model, tea.WindowSizeMsg{Width: 30, Height: 6})

	view := model.View()
	plain := ansi.Strip(view)
	if !strings.Contains(plain, "Terminal too small") {
		t.Fatalf("View() missing small-terminal fallback:\n%s", plain)
	}
	if !strings.Contains(plain, "40x8") {
		t.Fatalf("View() missing minimum dimensions:\n%s", plain)
	}
	if got := lipgloss.Width(view); got > 30 {
		t.Fatalf("View() width = %d, want <= 30", got)
	}
	if got := lipgloss.Height(view); got != 6 {
		t.Fatalf("View() height = %d, want 6", got)
	}
}

func TestNewMessagePreservesManualViewportPosition(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenChat
	model.session.User.ID = "user-1"
	model.session.User.Username = "alice"
	model.room = &domain.PublicRoom{ID: "room-1", Name: "private_room"}
	for index := range 40 {
		model.messages = append(model.messages, domain.Message{
			ID: "message", RoomID: "room-1", UserID: "user-2", Username: "bob",
			Content: "message line", CreatedAt: time.Date(2026, 8, 27, 21, index, 0, 0, time.Local),
		})
	}
	updateModel(t, model, tea.WindowSizeMsg{Width: 60, Height: 10})
	_ = model.View()
	updateModel(t, model, tea.KeyMsg{Type: tea.KeyPgUp})
	before := model.viewport.YOffset

	message := domain.Message{
		ID: "message-new", RoomID: "room-1", UserID: "user-2", Username: "bob",
		Content: "new message", CreatedAt: time.Date(2026, 8, 27, 22, 0, 0, 0, time.Local),
	}
	updateModel(t, model, serverEventMsg{event: protocol.ServerEvent{Type: "new_message", Message: &message}})

	if model.viewport.YOffset != before {
		t.Fatalf("viewport offset after new message = %d, want preserved at %d", model.viewport.YOffset, before)
	}
	if model.viewport.AtBottom() {
		t.Fatal("new message pulled manually scrolled viewport to bottom")
	}
}
