package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type statusLevel uint8

const (
	statusInfo statusLevel = iota
	statusSuccess
	statusWarning
	statusError

	infoToastDuration    = 4 * time.Second
	successToastDuration = 4 * time.Second
	warningToastDuration = 8 * time.Second
	errorToastDuration   = 12 * time.Second
)

type notification struct {
	id        uint64
	level     statusLevel
	text      string
	expiresAt time.Time
}

func (m *Model) setStatus(level statusLevel, text string) {
	m.statusLevel = level
	m.status = text
	if text == "" {
		m.activeNotification = notification{}
		return
	}
	m.nextNotificationID++
	m.activeNotification = notification{id: m.nextNotificationID, level: level, text: text, expiresAt: time.Now().Add(notificationDuration(level))}
}

func (m *Model) clearStatus() {
	m.statusLevel = statusInfo
	m.status = ""
	m.activeNotification = notification{}
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

// notificationTrayHeight reserves only the currently-visible, timed toast plus
// the persistent direct-invite action. The height is released when the toast expires.
func (m *Model) notificationTrayHeight(width int) int {
	height := 0
	if m.activeNotification.id != 0 {
		height++
	}
	if m.pendingDirectInvite != "" && m.pendingDirectSender != nil {
		height += 2
	}
	return height
}

func (m *Model) renderNotificationTray(width int) string {
	lines := make([]string, 0, m.notificationTrayHeight(width))
	if m.activeNotification.id != 0 {
		lines = append(lines, m.renderStatusToast(width))
	}
	if m.pendingDirectInvite != "" && m.pendingDirectSender != nil {
		lines = append(lines,
			m.theme.statusWarning.Render("[INVITE] Direct invitation from "+m.pendingDirectSender.Username),
			m.theme.muted.Render("         /accept to join · /decline to refuse"),
		)
	}
	if len(lines) == 0 {
		return ""
	}
	for index := range lines {
		lines[index] = m.theme.root.Width(width).Render(ansi.Truncate(lines[index], width, ""))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderStatusToast(width int) string {
	if m.activeNotification.id == 0 || width < 1 {
		return ""
	}
	maxWidth := maxInt(1, width/2)
	toast := ansi.Truncate(m.renderNotification(m.activeNotification), maxWidth, "…")
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Right).Render(toast)
}

func notificationDuration(level statusLevel) time.Duration {
	switch level {
	case statusSuccess:
		return successToastDuration
	case statusWarning:
		return warningToastDuration
	case statusError:
		return errorToastDuration
	default:
		return infoToastDuration
	}
}
