package tui

import tea "github.com/charmbracelet/bubbletea"

func pageScrollMessage(message tea.KeyMsg) (tea.KeyMsg, bool) {
	if message.Type == tea.KeyPgUp || message.Type == tea.KeyPgDown {
		return message, true
	}
	if message.Type != tea.KeyRunes || message.Alt || message.Paste || len(message.Runes) != 1 {
		return tea.KeyMsg{}, false
	}
	switch message.Runes[0] {
	case 'K':
		return tea.KeyMsg{Type: tea.KeyPgUp}, true
	case 'J':
		return tea.KeyMsg{Type: tea.KeyPgDown}, true
	default:
		return tea.KeyMsg{}, false
	}
}
