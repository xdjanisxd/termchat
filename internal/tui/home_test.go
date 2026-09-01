package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestHomeViewGuidesUsersToTheirNextRoomAction(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	model.session.User.Username = "alice"
	model.connectionState = connectionOnline
	updateModel(t, model, tea.WindowSizeMsg{Width: 80, Height: 24})

	plain := ansi.Strip(model.View())
	for _, want := range []string{
		"WELCOME, ALICE",
		"START HERE",
		"1. Join a room you know",
		"/join <room-name>",
		"2. Create a new private room",
		"/createroom <room-name> <password>",
		"Type /help for every command and example.",
	} {
		if !strings.Contains(plain, want) {
			t.Fatalf("Home View() missing %q:\n%s", want, plain)
		}
	}
}

func TestHomeViewPlacesCommandInputAboveFooter(t *testing.T) {
	t.Parallel()

	const height = 24
	model := NewModel(nil)
	model.screen = ScreenHome
	model.session.User.Username = "alice"
	updateModel(t, model, tea.WindowSizeMsg{Width: 80, Height: height})

	lines := strings.Split(ansi.Strip(model.View()), "\n")
	inputLine := -1
	for index, line := range lines {
		if strings.Contains(line, "> /join private_room") {
			inputLine = index
			break
		}
	}
	if inputLine != height-2 {
		t.Fatalf("home command input line = %d, want %d (immediately above footer); view:\n%s", inputLine, height-2, strings.Join(lines, "\n"))
	}
}

func TestHomeCommandErrorsExplainHowToRecover(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "unknown command",
			input: "/unknown",
			want:  "Unknown command \"/unknown\". Type /help to see available commands.",
		},
		{
			name:  "join missing room name",
			input: "/join",
			want:  "\"/join\" needs a room name. Try: /join <room-name>",
		},
		{
			name:  "message before joining",
			input: "hello",
			want:  "You are not in a room. Join one with /join <room-name>, or create one with /createroom <room-name> <password>.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := NewModel(nil)
			model.screen = ScreenHome
			model.commandInput.SetValue(test.input)

			updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

			if model.Status() != test.want {
				t.Fatalf("status = %q, want %q", model.Status(), test.want)
			}
		})
	}
}
