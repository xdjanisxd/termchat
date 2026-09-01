package tui

import (
	"fmt"
	"testing"
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
