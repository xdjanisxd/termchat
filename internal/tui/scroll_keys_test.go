package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"termchat.local/termchat/internal/client"
	"termchat.local/termchat/internal/domain"
)

func TestUppercaseKIsNotScrollAlias(t *testing.T) {
	t.Parallel()

	t.Run("chat composer", func(t *testing.T) {
		model := newScrollableChatModel()
		updateModel(t, model, tea.WindowSizeMsg{Width: 60, Height: 10})
		_ = model.View()
		before := model.viewport.YOffset

		updateModel(t, model, keyRunes("K"))

		if model.viewport.YOffset != before {
			t.Fatalf("viewport offset after K = %d, want %d", model.viewport.YOffset, before)
		}
		if model.commandInput.Value() != "K" {
			t.Fatalf("composer value after K = %q, want %q", model.commandInput.Value(), "K")
		}
	})

	t.Run("help viewport", func(t *testing.T) {
		model := NewModel(nil, client.SessionStore{})
		model.screen = ScreenHelp
		updateModel(t, model, tea.WindowSizeMsg{Width: 30, Height: 8})
		_ = model.View()
		for range 3 {
			updateModel(t, model, tea.KeyMsg{Type: tea.KeyPgDown})
		}
		before := model.helpViewport.YOffset
		if before == 0 {
			t.Fatal("test setup did not scroll help viewport")
		}

		updateModel(t, model, keyRunes("K"))

		if model.helpViewport.YOffset != before {
			t.Fatalf("help viewport offset after K = %d, want %d", model.helpViewport.YOffset, before)
		}
	})
}

func TestUppercaseJIsNotScrollAlias(t *testing.T) {
	t.Parallel()

	t.Run("chat composer", func(t *testing.T) {
		model := newScrollableChatModel()
		updateModel(t, model, tea.WindowSizeMsg{Width: 60, Height: 10})
		_ = model.View()
		updateModel(t, model, tea.KeyMsg{Type: tea.KeyPgUp})
		before := model.viewport.YOffset

		updateModel(t, model, keyRunes("J"))

		if model.viewport.YOffset != before {
			t.Fatalf("viewport offset after J = %d, want %d", model.viewport.YOffset, before)
		}
		if model.commandInput.Value() != "J" {
			t.Fatalf("composer value after J = %q, want %q", model.commandInput.Value(), "J")
		}
	})

	t.Run("help viewport", func(t *testing.T) {
		model := NewModel(nil, client.SessionStore{})
		model.screen = ScreenHelp
		updateModel(t, model, tea.WindowSizeMsg{Width: 30, Height: 8})
		_ = model.View()
		before := model.helpViewport.YOffset

		updateModel(t, model, keyRunes("J"))

		if model.helpViewport.YOffset != before {
			t.Fatalf("help viewport offset after J = %d, want %d", model.helpViewport.YOffset, before)
		}
	})
}

func TestLowercaseJKRemainComposerInput(t *testing.T) {
	t.Parallel()

	model := newScrollableChatModel()
	updateModel(t, model, tea.WindowSizeMsg{Width: 60, Height: 10})
	_ = model.View()
	before := model.viewport.YOffset

	updateModel(t, model, keyRunes("j"))
	updateModel(t, model, keyRunes("k"))

	if model.commandInput.Value() != "jk" {
		t.Fatalf("composer value = %q, want %q", model.commandInput.Value(), "jk")
	}
	if model.viewport.YOffset != before {
		t.Fatalf("viewport offset after lowercase j/k = %d, want %d", model.viewport.YOffset, before)
	}
}

func TestPastedUppercaseJRemainsComposerInput(t *testing.T) {
	t.Parallel()

	model := newScrollableChatModel()
	updateModel(t, model, tea.WindowSizeMsg{Width: 60, Height: 10})
	_ = model.View()
	before := model.viewport.YOffset

	updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}, Paste: true})

	if model.commandInput.Value() != "J" {
		t.Fatalf("composer value after pasted J = %q, want %q", model.commandInput.Value(), "J")
	}
	if model.viewport.YOffset != before {
		t.Fatalf("viewport offset after pasted J = %d, want %d", model.viewport.YOffset, before)
	}
}

func TestModifiedUppercaseJDoesNotScroll(t *testing.T) {
	t.Parallel()

	model := newScrollableChatModel()
	updateModel(t, model, tea.WindowSizeMsg{Width: 60, Height: 10})
	_ = model.View()
	updateModel(t, model, keyRunes("K"))
	before := model.viewport.YOffset

	updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}, Alt: true})

	if model.viewport.YOffset != before {
		t.Fatalf("viewport offset after Alt+J = %d, want %d", model.viewport.YOffset, before)
	}
}

func newScrollableChatModel() *Model {
	model := NewModel(nil, client.SessionStore{})
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
	return model
}
