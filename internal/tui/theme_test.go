package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModelUsesAmberCRTThemeForInputs(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)

	if model.theme.name != "amber-crt" {
		t.Fatalf("theme name = %q, want amber-crt", model.theme.name)
	}
	if !model.theme.title.GetBold() {
		t.Fatal("theme title is not bold")
	}
	if model.theme.title.GetForeground() == nil {
		t.Fatal("theme title has no foreground color")
	}
	if got, want := fmt.Sprint(model.username.PromptStyle.GetForeground()), fmt.Sprint(model.theme.prompt.GetForeground()); got != want {
		t.Fatalf("username prompt color = %q, want %q", got, want)
	}
	if got, want := fmt.Sprint(model.password.PlaceholderStyle.GetForeground()), fmt.Sprint(model.theme.muted.GetForeground()); got != want {
		t.Fatalf("password placeholder color = %q, want %q", got, want)
	}
	if got, want := fmt.Sprint(model.commandInput.Cursor.Style.GetForeground()), fmt.Sprint(model.theme.cursor.GetForeground()); got != want {
		t.Fatalf("command cursor color = %q, want %q", got, want)
	}
}

func TestThemeCommandChangesThemeAndEveryInputForCurrentSession(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	model.commandInput.SetValue("/theme green-crt")

	updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if model.theme.name != "green-crt" {
		t.Fatalf("theme name = %q, want green-crt", model.theme.name)
	}
	if model.statusLevel != statusSuccess || model.Status() != "Theme changed to green-crt." {
		t.Fatalf("theme status = (%v, %q), want success confirmation", model.statusLevel, model.Status())
	}
	for name, input := range map[string]*textinput.Model{
		"username":      &model.username,
		"password":      &model.password,
		"command input": &model.commandInput,
		"room password": &model.roomPassword,
	} {
		if got, want := fmt.Sprint(input.PromptStyle.GetForeground()), fmt.Sprint(model.theme.prompt.GetForeground()); got != want {
			t.Errorf("%s prompt color = %q, want %q", name, got, want)
		}
	}
}

func TestThemeCommandSupportsEveryBuiltInPalette(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"amber-crt", "green-crt", "ice-blue", "synthwave", "cyberpunk"} {
		t.Run(name, func(t *testing.T) {
			model := NewModel(nil)
			model.screen = ScreenHome
			model.commandInput.SetValue("/theme " + name)

			updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

			if model.theme.name != name {
				t.Fatalf("theme name = %q, want %q", model.theme.name, name)
			}
			wantBackground := fmt.Sprint(model.theme.viewport.GetBackground())
			for styleName, background := range map[string]any{
				"root":         model.theme.root.GetBackground(),
				"header":       model.theme.header.GetBackground(),
				"composer":     model.theme.composer.GetBackground(),
				"message body": model.theme.messageBody.GetBackground(),
				"input":        model.theme.input.GetBackground(),
			} {
				if got := fmt.Sprint(background); got != wantBackground {
					t.Errorf("%s background = %q, want viewport background %q", styleName, got, wantBackground)
				}
			}
		})
	}
}

func TestUnknownThemeKeepsCurrentThemeAndListsValidNames(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	model.commandInput.SetValue("/theme unknown")

	updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if model.theme.name != "amber-crt" {
		t.Fatalf("theme name = %q, want unchanged amber-crt", model.theme.name)
	}
	if model.statusLevel != statusError {
		t.Fatalf("status level = %v, want error", model.statusLevel)
	}
	for _, text := range []string{"Unknown theme", "amber-crt", "green-crt", "ice-blue", "synthwave", "cyberpunk"} {
		if !strings.Contains(model.Status(), text) {
			t.Fatalf("status %q does not contain %q", model.Status(), text)
		}
	}
}

func TestAmberCRTMessageCellsUseViewportBackground(t *testing.T) {
	t.Parallel()

	theme := newAmberCRTTheme()
	want := fmt.Sprint(theme.viewport.GetBackground())
	for name, style := range map[string]struct{ background any }{
		"timestamp":    {theme.timestamp.GetBackground()},
		"message user": {theme.messageUser.GetBackground()},
		"message self": {theme.messageSelf.GetBackground()},
		"message body": {theme.messageBody.GetBackground()},
		"empty state":  {theme.emptyState.GetBackground()},
	} {
		if got := fmt.Sprint(style.background); got != want {
			t.Errorf("%s background = %q, want viewport background %q", name, got, want)
		}
	}
}

func TestAmberCRTHomeAndHelpCellsUseViewportBackground(t *testing.T) {
	t.Parallel()

	theme := newAmberCRTTheme()
	want := fmt.Sprint(theme.viewport.GetBackground())
	for name, style := range map[string]struct{ background any }{
		"root":     {theme.root.GetBackground()},
		"brand":    {theme.brand.GetBackground()},
		"room":     {theme.room.GetBackground()},
		"header":   {theme.header.GetBackground()},
		"composer": {theme.composer.GetBackground()},
		"footer":   {theme.footer.GetBackground()},
		"prompt":   {theme.prompt.GetBackground()},
		"input":    {theme.input.GetBackground()},
		"muted":    {theme.muted.GetBackground()},
		"cursor":   {theme.cursor.GetBackground()},
	} {
		if got := fmt.Sprint(style.background); got != want {
			t.Errorf("%s background = %q, want viewport background %q", name, got, want)
		}
	}
}
