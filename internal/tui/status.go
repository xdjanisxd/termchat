package tui

type statusLevel uint8

const (
	statusInfo statusLevel = iota
	statusSuccess
	statusWarning
	statusError
)

func (m *Model) setStatus(level statusLevel, text string) {
	m.statusLevel = level
	m.status = text
}

func (m *Model) clearStatus() {
	m.setStatus(statusInfo, "")
}

func (m *Model) renderStatus() string {
	label := "[INFO]"
	style := m.theme.statusInfo
	switch m.statusLevel {
	case statusSuccess:
		label = "[OK]"
		style = m.theme.statusSuccess
	case statusWarning:
		label = "[WARN]"
		style = m.theme.statusWarning
	case statusError:
		label = "[ERROR]"
		style = m.theme.statusError
	}
	return style.Render(label + " " + m.status)
}
