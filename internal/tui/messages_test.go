package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/protocol"
)

func TestHistoryPagePrependsOlderMessagesAndUpdatesAvailability(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.messages = []domain.Message{{ID: "message-51"}}
	model.historyLoading = true
	model.applyServerEvent(protocol.ServerEvent{Type: "message_history", Messages: []domain.Message{{ID: "message-1"}, {ID: "message-50"}}, HasMore: false})

	if model.historyLoading || model.historyHasMore || len(model.messages) != 3 || model.messages[0].ID != "message-1" || model.messages[2].ID != "message-51" {
		t.Fatalf("history state = loading:%t hasMore:%t messages:%#v", model.historyLoading, model.historyHasMore, model.messages)
	}
}

func TestEmptyMessageStateFillsViewportWidthWithThemeBackground(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.width = 80

	rendered := model.renderMessages()
	if got := lipgloss.Width(rendered); got != model.width {
		t.Fatalf("empty message width = %d, want %d", got, model.width)
	}
	if !strings.Contains(ansi.Strip(rendered), "No messages yet.") {
		t.Fatalf("empty state text missing: %q", ansi.Strip(rendered))
	}
}

func TestMessagesDistinguishCurrentUserWithoutRelyingOnColor(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.session.User.ID = "user-1"
	model.session.User.Username = "alice"
	model.messages = []domain.Message{
		{ID: "message-1", UserID: "user-1", Username: "alice", Content: "hello from me", CreatedAt: time.Date(2026, 8, 28, 21, 4, 0, 0, time.Local)},
		{ID: "message-2", UserID: "user-2", Username: "bob", Content: "hello from bob", CreatedAt: time.Date(2026, 8, 28, 21, 5, 0, 0, time.Local)},
	}

	plain := ansi.Strip(model.renderMessages())
	for _, line := range []string{
		"21:04  YOU  hello from me",
		"21:05  bob  hello from bob",
	} {
		if !strings.Contains(plain, line) {
			t.Fatalf("renderMessages() missing %q:\n%s", line, plain)
		}
	}
}

func TestMessagesWrapWithinViewportWidthWithoutLosingContent(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.width = 40
	model.session.User.ID = "user-1"
	longRun := strings.Repeat("x", 80)
	model.messages = []domain.Message{{
		ID: "message-1", UserID: "user-2", Username: "bob",
		Content: "start " + longRun + " end", CreatedAt: time.Date(2026, 8, 28, 21, 4, 0, 0, time.Local),
	}}

	rendered := model.renderMessages()
	plain := ansi.Strip(rendered)
	for lineNumber, line := range strings.Split(rendered, "\n") {
		if got := lipgloss.Width(line); got > 40 {
			t.Fatalf("line %d width = %d, want <= 40:\n%s", lineNumber+1, got, plain)
		}
	}
	if strings.Count(plain, "x") != len(longRun) {
		t.Fatalf("renderMessages() lost long content: x count = %d, want %d\n%s", strings.Count(plain, "x"), len(longRun), plain)
	}
	for _, marker := range []string{"start", "end"} {
		if !strings.Contains(plain, marker) {
			t.Fatalf("renderMessages() lost %q:\n%s", marker, plain)
		}
	}
}
