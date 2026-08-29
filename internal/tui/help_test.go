package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"termchat.local/termchat/internal/client"
)

func TestHelpCommandOpensDedicatedScreenFromHome(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, client.SessionStore{})
	model.screen = ScreenHome
	model.commandInput.SetValue("/help")

	updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if model.Screen() != ScreenHelp {
		t.Fatalf("Screen() = %v, want ScreenHelp", model.Screen())
	}
}

func TestHelpScreenShowsScannableCommandGroups(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, client.SessionStore{})
	model.screen = ScreenHelp
	updateModel(t, model, tea.WindowSizeMsg{Width: 80, Height: 24})

	view := model.View()
	plain := ansi.Strip(view)
	for _, marker := range []string{
		"TERMCHAT // HELP",
		"ROOMS",
		"/createroom <name> <password>",
		"/join <name>",
		"CHAT",
		"/who",
		"/l",
		"OWNER",
		"/roompasswd <password>",
		"/deleteroom",
		"APP",
		"/help",
		"/q",
		"Esc close",
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

func TestHelpScreenScrollsWithinSmallTerminal(t *testing.T) {
	t.Parallel()

	model := NewModel(nil, client.SessionStore{})
	model.screen = ScreenHelp
	updateModel(t, model, tea.WindowSizeMsg{Width: 30, Height: 8})

	initial := model.View()
	initialPlain := ansi.Strip(initial)
	if !strings.Contains(initialPlain, "TERMCHAT // HELP") || !strings.Contains(initialPlain, "ROOMS") {
		t.Fatalf("initial help view missing fixed header or first section:\n%s", initialPlain)
	}
	if !strings.Contains(initialPlain, "PgUp/PgDn scroll") || !strings.Contains(initialPlain, "Esc close") {
		t.Fatalf("initial help view missing navigation hints:\n%s", initialPlain)
	}
	if got := lipgloss.Width(initial); got > 30 {
		t.Fatalf("small help width = %d, want <= 30", got)
	}
	if got := lipgloss.Height(initial); got != 8 {
		t.Fatalf("small help height = %d, want 8", got)
	}

	for range 10 {
		updateModel(t, model, tea.KeyMsg{Type: tea.KeyPgDown})
	}
	bottom := ansi.Strip(model.View())
	if !strings.Contains(bottom, "/q") {
		t.Fatalf("help did not scroll to final command:\n%s", bottom)
	}
}

func TestEscapeClosesHelpToPreviousScreen(t *testing.T) {
	t.Parallel()

	for _, start := range []Screen{ScreenHome, ScreenChat} {
		t.Run(fmt.Sprintf("screen_%d", start), func(t *testing.T) {
			model := NewModel(nil, client.SessionStore{})
			model.screen = start
			model.commandInput.SetValue("/help")
			updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
			if model.screen != ScreenHelp {
				t.Fatalf("help command screen = %v, want ScreenHelp", model.screen)
			}

			updateModel(t, model, tea.KeyMsg{Type: tea.KeyEsc})

			if model.screen != start {
				t.Fatalf("screen after Esc = %v, want %v", model.screen, start)
			}
			if !model.commandInput.Focused() {
				t.Fatal("command input is not focused after closing help")
			}
		})
	}
}
