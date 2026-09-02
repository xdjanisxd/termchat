package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

type statusLevel uint8

const (
	statusInfo statusLevel = iota
	statusSuccess
	statusWarning
	statusError

	maxNotifications = 3
)

type notification struct {
	level statusLevel
	text  string
}

func (m *Model) setStatus(level statusLevel, text string) {
	m.statusLevel = level
	m.status = text
	if text == "" {
		return
	}
	m.notifications = append(m.notifications, notification{level: level, text: text})
	if len(m.notifications) > maxNotifications {
		m.notifications = m.notifications[len(m.notifications)-maxNotifications:]
	}
}

func (m *Model) clearStatus() {
	m.statusLevel = statusInfo
	m.status = ""
	m.notifications = nil
}

func (m *Model) renderStatus() string {
	return m.renderNotification(notification{level: m.statusLevel, text: m.status})
}

func (m *Model) renderNotification(note notification) string {
	label := "[INFO]"
	style := m.theme.statusInfo
	switch note.level {
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
	return style.Render(label + " " + note.text)
}

func (m *Model) notificationLines(width int) []string {
	lines := make([]string, 0, len(m.notifications)+2)
	if m.pendingDirectInvite != "" && m.pendingDirectSender != nil {
		lines = append(lines, m.theme.statusWarning.Render("[INVITE] Direct invitation from "+m.pendingDirectSender.Username))
		lines = append(lines, m.theme.muted.Render("         /accept to join · /decline to refuse"))
	}
	for _, note := range m.notifications {
		lines = append(lines, m.renderNotification(note))
	}
	for index := range lines {
		lines[index] = ansi.Truncate(lines[index], width, "")
	}
	return lines
}

func (m *Model) notificationTrayHeight(width int) int {
	return len(m.notificationLines(width))
}

func (m *Model) renderNotificationTray(width int) string {
	lines := m.notificationLines(width)
	if len(lines) == 0 {
		return ""
	}
	for index := range lines {
		lines[index] = m.theme.root.Width(width).Render(lines[index])
	}
	return strings.Join(lines, "\n")
}
