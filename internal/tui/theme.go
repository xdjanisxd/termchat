package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type tuiTheme struct {
	name          string
	root          lipgloss.Style
	title         lipgloss.Style
	brand         lipgloss.Style
	room          lipgloss.Style
	connOnline    lipgloss.Style
	connOffline   lipgloss.Style
	connPending   lipgloss.Style
	header        lipgloss.Style
	viewport      lipgloss.Style
	composer      lipgloss.Style
	footer        lipgloss.Style
	statusInfo    lipgloss.Style
	statusSuccess lipgloss.Style
	statusWarning lipgloss.Style
	statusError   lipgloss.Style
	timestamp     lipgloss.Style
	messageUser   lipgloss.Style
	messageSelf   lipgloss.Style
	messageBody   lipgloss.Style
	emptyState    lipgloss.Style
	prompt        lipgloss.Style
	input         lipgloss.Style
	muted         lipgloss.Style
	cursor        lipgloss.Style
}

func newAmberCRTTheme() tuiTheme {
	amber := lipgloss.AdaptiveColor{Light: "#8A4F00", Dark: "#FFB000"}
	brightAmber := lipgloss.AdaptiveColor{Light: "#663A00", Dark: "#FFD27A"}
	mutedAmber := lipgloss.AdaptiveColor{Light: "#7A6850", Dark: "#8F6A2A"}
	borderAmber := lipgloss.AdaptiveColor{Light: "#A77A37", Dark: "#5E4315"}
	phosphorGreen := lipgloss.AdaptiveColor{Light: "#1F7A3A", Dark: "#7DFF91"}
	errorRed := lipgloss.AdaptiveColor{Light: "#B42318", Dark: "#FF6B5E"}
	background := lipgloss.AdaptiveColor{Light: "#FFF8E7", Dark: "#120B04"}

	return tuiTheme{
		name:          "amber-crt",
		root:          lipgloss.NewStyle().Background(background),
		title:         lipgloss.NewStyle().Bold(true).Foreground(brightAmber).Background(background),
		brand:         lipgloss.NewStyle().Bold(true).Foreground(brightAmber).Background(background),
		room:          lipgloss.NewStyle().Foreground(amber).Background(background),
		connOnline:    lipgloss.NewStyle().Bold(true).Foreground(phosphorGreen).Background(background),
		connOffline:   lipgloss.NewStyle().Bold(true).Foreground(errorRed).Background(background),
		connPending:   lipgloss.NewStyle().Bold(true).Foreground(brightAmber).Background(background),
		header:        lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(borderAmber).Background(background),
		viewport:      lipgloss.NewStyle().Foreground(brightAmber).Background(background),
		composer:      lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderTop(true).BorderForeground(borderAmber).Background(background).Padding(0, 1),
		footer:        lipgloss.NewStyle().Foreground(mutedAmber).Background(background),
		statusInfo:    lipgloss.NewStyle().Bold(true).Foreground(amber).Background(background),
		statusSuccess: lipgloss.NewStyle().Bold(true).Foreground(phosphorGreen).Background(background),
		statusWarning: lipgloss.NewStyle().Bold(true).Foreground(brightAmber).Background(background),
		statusError:   lipgloss.NewStyle().Bold(true).Foreground(errorRed).Background(background),
		timestamp:     lipgloss.NewStyle().Foreground(mutedAmber).Background(background),
		messageUser:   lipgloss.NewStyle().Bold(true).Foreground(amber).Background(background),
		messageSelf:   lipgloss.NewStyle().Bold(true).Foreground(phosphorGreen).Background(background),
		messageBody:   lipgloss.NewStyle().Foreground(brightAmber).Background(background),
		emptyState:    lipgloss.NewStyle().Faint(true).Foreground(mutedAmber).Background(background),
		prompt:        lipgloss.NewStyle().Bold(true).Foreground(amber).Background(background),
		input:         lipgloss.NewStyle().Foreground(brightAmber).Background(background),
		muted:         lipgloss.NewStyle().Faint(true).Foreground(mutedAmber).Background(background),
		cursor:        lipgloss.NewStyle().Foreground(brightAmber).Background(background),
	}
}

func (t tuiTheme) applyInput(input *textinput.Model) {
	input.PromptStyle = t.prompt
	input.TextStyle = t.input
	input.PlaceholderStyle = t.muted
	input.CompletionStyle = t.muted
	input.Cursor.Style = t.cursor
}
