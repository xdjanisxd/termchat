package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type tuiTheme struct {
	name   string
	title  lipgloss.Style
	prompt lipgloss.Style
	input  lipgloss.Style
	muted  lipgloss.Style
	cursor lipgloss.Style
}

func newAmberCRTTheme() tuiTheme {
	amber := lipgloss.AdaptiveColor{Light: "#8A4F00", Dark: "#FFB000"}
	brightAmber := lipgloss.AdaptiveColor{Light: "#663A00", Dark: "#FFD27A"}
	mutedAmber := lipgloss.AdaptiveColor{Light: "#7A6850", Dark: "#8F6A2A"}

	return tuiTheme{
		name:   "amber-crt",
		title:  lipgloss.NewStyle().Bold(true).Foreground(brightAmber),
		prompt: lipgloss.NewStyle().Bold(true).Foreground(amber),
		input:  lipgloss.NewStyle().Foreground(brightAmber),
		muted:  lipgloss.NewStyle().Faint(true).Foreground(mutedAmber),
		cursor: lipgloss.NewStyle().Foreground(brightAmber),
	}
}

func (t tuiTheme) applyInput(input *textinput.Model) {
	input.PromptStyle = t.prompt
	input.TextStyle = t.input
	input.PlaceholderStyle = t.muted
	input.CompletionStyle = t.muted
	input.Cursor.Style = t.cursor
}
