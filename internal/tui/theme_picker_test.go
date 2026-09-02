package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"termchat.local/termchat/internal/domain"
)

func TestThemePickerOpensAfterTypingThemeCommand(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	updateModel(t, model, keyRunes("/theme"))

	view := ansi.Strip(model.View())
	for _, name := range themeNames() {
		if !strings.Contains(view, name) {
			t.Fatalf("theme picker does not contain %q:\n%s", name, view)
		}
	}
	if !strings.Contains(view, ">[amber-crt]<") {
		t.Fatalf("theme picker does not mark amber-crt as both active and selected:\n%s", view)
	}
}

func TestThemePickerTabCyclesAndEnterAppliesSelection(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	updateModel(t, model, keyRunes("/theme"))
	updateModel(t, model, tea.KeyMsg{Type: tea.KeyTab})

	view := ansi.Strip(model.View())
	if !strings.Contains(view, "[amber-crt]") || !strings.Contains(view, ">green-crt<") {
		t.Fatalf("theme picker did not distinguish the active theme from the Tab selection:\n%s", view)
	}
	if model.commandInput.Value() != "/theme" {
		t.Fatalf("command input after Tab = %q, want /theme", model.commandInput.Value())
	}

	updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if model.theme.name != "green-crt" {
		t.Fatalf("theme after Enter = %q, want green-crt", model.theme.name)
	}
	if model.commandInput.Value() != "" {
		t.Fatalf("command input after selection = %q, want empty", model.commandInput.Value())
	}
	if model.Status() != "Theme changed to green-crt." {
		t.Fatalf("status = %q, want theme confirmation", model.Status())
	}
}

func TestThemePickerStartsFromCurrentThemeAndShiftTabMovesBackward(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	model.applyTheme(newSynthwaveTheme())
	updateModel(t, model, keyRunes("/theme"))

	if view := ansi.Strip(model.View()); !strings.Contains(view, ">[synthwave]<") {
		t.Fatalf("theme picker does not start from current theme:\n%s", view)
	}

	updateModel(t, model, tea.KeyMsg{Type: tea.KeyShiftTab})
	if view := ansi.Strip(model.View()); !strings.Contains(view, "[synthwave]") || !strings.Contains(view, ">ice-blue<") {
		t.Fatalf("theme picker did not move backward with Shift+Tab:\n%s", view)
	}
}

func TestThemePickerWrapsAndEscapeCancelsWithoutSending(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	updateModel(t, model, keyRunes("/theme"))
	if command := updateModel(t, model, tea.KeyMsg{Type: tea.KeyShiftTab}); command != nil {
		t.Fatal("Shift+Tab returned a server command")
	}
	if view := ansi.Strip(model.View()); !strings.Contains(view, ">cyberpunk<") {
		t.Fatalf("Shift+Tab did not wrap to cyberpunk:\n%s", view)
	}
	if command := updateModel(t, model, tea.KeyMsg{Type: tea.KeyTab}); command != nil {
		t.Fatal("Tab returned a server command")
	}
	if view := ansi.Strip(model.View()); !strings.Contains(view, ">[amber-crt]<") {
		t.Fatalf("Tab did not wrap to amber-crt:\n%s", view)
	}
	if command := updateModel(t, model, tea.KeyMsg{Type: tea.KeyEsc}); command != nil {
		t.Fatal("Escape returned a server command")
	}
	if model.theme.name != "amber-crt" || model.commandInput.Value() != "" {
		t.Fatalf("Escape changed picker state: theme=%q input=%q", model.theme.name, model.commandInput.Value())
	}
}

func TestThemePickerAndDirectThemeSelectionStayClientOnly(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	updateModel(t, model, keyRunes("/theme"))
	updateModel(t, model, tea.KeyMsg{Type: tea.KeyTab})
	if command := updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter}); command != nil {
		t.Fatal("picker theme selection returned a server command")
	}

	model.commandInput.SetValue("/theme cyberpunk")
	if command := updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter}); command != nil {
		t.Fatal("direct theme selection returned a server command")
	}
	if model.theme.name != "cyberpunk" {
		t.Fatalf("direct theme selection = %q, want cyberpunk", model.theme.name)
	}
}

func TestThemePickerPreservesChatSizeAndShowsAllThemesAtMinimumWidth(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenChat
	model.room = &domain.PublicRoom{ID: "room-1", Name: "private_room"}
	model.session.User.Username = "alice"
	updateModel(t, model, tea.WindowSizeMsg{Width: minimumChatWidth, Height: minimumChatHeight})
	beforeHeight := lipgloss.Height(model.View())

	updateModel(t, model, keyRunes("/theme"))
	view := model.View()
	plain := ansi.Strip(view)

	if got := lipgloss.Height(view); got != beforeHeight {
		t.Fatalf("view height with theme picker = %d, want %d", got, beforeHeight)
	}
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > minimumChatWidth {
			t.Fatalf("rendered line width = %d, want at most %d", got, minimumChatWidth)
		}
	}
	for _, name := range themeNames() {
		if !strings.Contains(plain, name) {
			t.Fatalf("minimum-width theme picker does not contain %q:\n%s", name, plain)
		}
	}
	if !strings.Contains(plain, "Tab next") || !strings.Contains(plain, "Enter apply") {
		t.Fatalf("theme picker footer does not explain controls:\n%s", plain)
	}
}

func TestThemePickerControlsTakePriorityOverStatusAtMinimumWidth(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenChat
	model.room = &domain.PublicRoom{ID: "room-1", Name: "private_room"}
	model.setStatus(statusSuccess, "Connected with a deliberately long status message")
	updateModel(t, model, tea.WindowSizeMsg{Width: minimumChatWidth, Height: minimumChatHeight})
	updateModel(t, model, keyRunes("/theme"))

	plain := ansi.Strip(model.View())
	for _, control := range []string{"Tab next", "Enter apply", "Esc cancel"} {
		if !strings.Contains(plain, control) {
			t.Fatalf("picker with status does not show %q:\n%s", control, plain)
		}
	}
}

func TestThemePickerClosesWhenCommandChanges(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	updateModel(t, model, keyRunes("/theme"))
	updateModel(t, model, tea.KeyMsg{Type: tea.KeyBackspace})

	view := ansi.Strip(model.View())
	if strings.Contains(view, ">[amber-crt]<") {
		t.Fatalf("theme picker remained open after command changed:\n%s", view)
	}
}
