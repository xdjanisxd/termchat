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

type themePalette struct {
	background lipgloss.AdaptiveColor
	primary    lipgloss.AdaptiveColor
	bright     lipgloss.AdaptiveColor
	muted      lipgloss.AdaptiveColor
	border     lipgloss.AdaptiveColor
	success    lipgloss.AdaptiveColor
	warning    lipgloss.AdaptiveColor
	error      lipgloss.AdaptiveColor
}

func newAmberCRTTheme() tuiTheme {
	return newTheme("amber-crt", themePalette{
		background: lipgloss.AdaptiveColor{Light: "#FFF8E7", Dark: "#120B04"},
		primary:    lipgloss.AdaptiveColor{Light: "#8A4F00", Dark: "#FFB000"},
		bright:     lipgloss.AdaptiveColor{Light: "#663A00", Dark: "#FFD27A"},
		muted:      lipgloss.AdaptiveColor{Light: "#7A6850", Dark: "#8F6A2A"},
		border:     lipgloss.AdaptiveColor{Light: "#A77A37", Dark: "#5E4315"},
		success:    lipgloss.AdaptiveColor{Light: "#1F7A3A", Dark: "#7DFF91"},
		warning:    lipgloss.AdaptiveColor{Light: "#663A00", Dark: "#FFD27A"},
		error:      lipgloss.AdaptiveColor{Light: "#B42318", Dark: "#FF6B5E"},
	})
}

func newGreenCRTTheme() tuiTheme {
	return newTheme("green-crt", themePalette{
		background: lipgloss.AdaptiveColor{Light: "#EFFBF2", Dark: "#031209"},
		primary:    lipgloss.AdaptiveColor{Light: "#176B32", Dark: "#4AFF72"},
		bright:     lipgloss.AdaptiveColor{Light: "#0D4F24", Dark: "#B7FFC6"},
		muted:      lipgloss.AdaptiveColor{Light: "#66806D", Dark: "#4D8A5D"},
		border:     lipgloss.AdaptiveColor{Light: "#8DB99A", Dark: "#245C35"},
		success:    lipgloss.AdaptiveColor{Light: "#137333", Dark: "#62FF8A"},
		warning:    lipgloss.AdaptiveColor{Light: "#8A5A00", Dark: "#FFD166"},
		error:      lipgloss.AdaptiveColor{Light: "#B42318", Dark: "#FF6B6B"},
	})
}

func newIceBlueTheme() tuiTheme {
	return newTheme("ice-blue", themePalette{
		background: lipgloss.AdaptiveColor{Light: "#F2F8FC", Dark: "#07131F"},
		primary:    lipgloss.AdaptiveColor{Light: "#166A8A", Dark: "#4CC9F0"},
		bright:     lipgloss.AdaptiveColor{Light: "#16475D", Dark: "#BDEEFF"},
		muted:      lipgloss.AdaptiveColor{Light: "#657C87", Dark: "#63889B"},
		border:     lipgloss.AdaptiveColor{Light: "#91B4C5", Dark: "#24506A"},
		success:    lipgloss.AdaptiveColor{Light: "#18794E", Dark: "#65E6A5"},
		warning:    lipgloss.AdaptiveColor{Light: "#8A5A00", Dark: "#FFD166"},
		error:      lipgloss.AdaptiveColor{Light: "#B42318", Dark: "#FF6B6B"},
	})
}

func newSynthwaveTheme() tuiTheme {
	return newTheme("synthwave", themePalette{
		background: lipgloss.AdaptiveColor{Light: "#FBF4FF", Dark: "#150A24"},
		primary:    lipgloss.AdaptiveColor{Light: "#9A237E", Dark: "#FF4FD8"},
		bright:     lipgloss.AdaptiveColor{Light: "#275C73", Dark: "#8BE9FD"},
		muted:      lipgloss.AdaptiveColor{Light: "#76617F", Dark: "#9576A6"},
		border:     lipgloss.AdaptiveColor{Light: "#B89BC5", Dark: "#57306E"},
		success:    lipgloss.AdaptiveColor{Light: "#237A3B", Dark: "#50FA7B"},
		warning:    lipgloss.AdaptiveColor{Light: "#8A5A00", Dark: "#FFD166"},
		error:      lipgloss.AdaptiveColor{Light: "#B21F50", Dark: "#FF5C8A"},
	})
}

func newCyberpunkTheme() tuiTheme {
	return newTheme("cyberpunk", themePalette{
		background: lipgloss.AdaptiveColor{Light: "#F8F7FF", Dark: "#050816"},
		primary:    lipgloss.AdaptiveColor{Light: "#007A85", Dark: "#00F5FF"},
		bright:     lipgloss.AdaptiveColor{Light: "#A10082", Dark: "#FF2BD6"},
		muted:      lipgloss.AdaptiveColor{Light: "#666A83", Dark: "#7A7FA8"},
		border:     lipgloss.AdaptiveColor{Light: "#A7A0C2", Dark: "#4B2A7A"},
		success:    lipgloss.AdaptiveColor{Light: "#4F7600", Dark: "#A8FF00"},
		warning:    lipgloss.AdaptiveColor{Light: "#7A6500", Dark: "#FFE600"},
		error:      lipgloss.AdaptiveColor{Light: "#B00035", Dark: "#FF3B6B"},
	})
}

func newTheme(name string, palette themePalette) tuiTheme {
	return tuiTheme{
		name:          name,
		root:          lipgloss.NewStyle().Background(palette.background),
		title:         lipgloss.NewStyle().Bold(true).Foreground(palette.bright).Background(palette.background),
		brand:         lipgloss.NewStyle().Bold(true).Foreground(palette.bright).Background(palette.background),
		room:          lipgloss.NewStyle().Foreground(palette.primary).Background(palette.background),
		connOnline:    lipgloss.NewStyle().Bold(true).Foreground(palette.success).Background(palette.background),
		connOffline:   lipgloss.NewStyle().Bold(true).Foreground(palette.error).Background(palette.background),
		connPending:   lipgloss.NewStyle().Bold(true).Foreground(palette.warning).Background(palette.background),
		header:        lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderBottom(true).BorderForeground(palette.border).Background(palette.background),
		viewport:      lipgloss.NewStyle().Foreground(palette.bright).Background(palette.background),
		composer:      lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderTop(true).BorderForeground(palette.border).Background(palette.background).Padding(0, 1),
		footer:        lipgloss.NewStyle().Foreground(palette.muted).Background(palette.background),
		statusInfo:    lipgloss.NewStyle().Bold(true).Foreground(palette.primary).Background(palette.background),
		statusSuccess: lipgloss.NewStyle().Bold(true).Foreground(palette.success).Background(palette.background),
		statusWarning: lipgloss.NewStyle().Bold(true).Foreground(palette.warning).Background(palette.background),
		statusError:   lipgloss.NewStyle().Bold(true).Foreground(palette.error).Background(palette.background),
		timestamp:     lipgloss.NewStyle().Foreground(palette.muted).Background(palette.background),
		messageUser:   lipgloss.NewStyle().Bold(true).Foreground(palette.primary).Background(palette.background),
		messageSelf:   lipgloss.NewStyle().Bold(true).Foreground(palette.success).Background(palette.background),
		messageBody:   lipgloss.NewStyle().Foreground(palette.bright).Background(palette.background),
		emptyState:    lipgloss.NewStyle().Faint(true).Foreground(palette.muted).Background(palette.background),
		prompt:        lipgloss.NewStyle().Bold(true).Foreground(palette.primary).Background(palette.background),
		input:         lipgloss.NewStyle().Foreground(palette.bright).Background(palette.background),
		muted:         lipgloss.NewStyle().Faint(true).Foreground(palette.muted).Background(palette.background),
		cursor:        lipgloss.NewStyle().Foreground(palette.bright).Background(palette.background),
	}
}

func themeByName(name string) (tuiTheme, bool) {
	switch name {
	case "amber-crt":
		return newAmberCRTTheme(), true
	case "green-crt":
		return newGreenCRTTheme(), true
	case "ice-blue":
		return newIceBlueTheme(), true
	case "synthwave":
		return newSynthwaveTheme(), true
	case "cyberpunk":
		return newCyberpunkTheme(), true
	default:
		return tuiTheme{}, false
	}
}

func themeNames() []string {
	return []string{"amber-crt", "green-crt", "ice-blue", "synthwave", "cyberpunk"}
}

func (t tuiTheme) applyInput(input *textinput.Model) {
	input.PromptStyle = t.prompt
	input.TextStyle = t.input
	input.PlaceholderStyle = t.muted
	input.CompletionStyle = t.muted
	input.Cursor.Style = t.cursor
}
