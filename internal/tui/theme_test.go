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
