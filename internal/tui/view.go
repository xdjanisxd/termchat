package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	chatHeaderHeight   = 2
	chatComposerHeight = 2
	chatFooterHeight   = 1
	minimumChatWidth   = 40
	minimumChatHeight  = 8
)

func (m *Model) syncChatLayout() {
	width, height := m.terminalSize()
	viewportHeight := height - chatHeaderHeight - chatComposerHeight - chatFooterHeight
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	wasAtBottom := m.viewport.AtBottom()
	m.viewport.Width = width
	m.viewport.Height = viewportHeight
	m.viewport.Style = m.theme.viewport.Width(width)
	m.commandInput.Width = maxInt(1, width-4)
	m.commandInput.Placeholder = "Type a message or /help"
	m.viewport.SetContent(m.renderMessages())
	if wasAtBottom {
		m.viewport.GotoBottom()
	}
}

func (m *Model) renderChatView() string {
	width, height := m.terminalSize()
	if width < minimumChatWidth || height < minimumChatHeight {
		return m.renderSmallTerminalFallback(width, height)
	}
	m.syncChatLayout()

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderChatHeader(width),
		m.viewport.View(),
		m.renderChatComposer(width),
		m.renderChatFooter(width),
	)
	return m.theme.root.Width(width).Height(height).Render(content)
}

func (m *Model) renderHomeView() string {
	width, height := m.terminalSize()
	m.commandInput.Width = maxInt(1, width-4)
	m.commandInput.Placeholder = "/join private_room"

	contentHeight := maxInt(1, height-chatHeaderHeight-chatComposerHeight-chatFooterHeight)
	guide := strings.Join([]string{
		"WELCOME, " + strings.ToUpper(m.session.User.Username),
		"",
		m.theme.brand.Render("START HERE"),
		"1. Join a room you know",
		m.theme.muted.Render("   /join <room-name>"),
		"2. Create a new private room",
		m.theme.muted.Render("   /createroom <room-name> <password>"),
		"",
		m.theme.muted.Render("Room names are private; there is no public room list."),
		"",
		"Type /help for every command and example.",
	}, "\n")
	content := m.theme.viewport.Width(width).Height(contentHeight).Render(guide)

	return m.theme.root.Width(width).Height(height).Render(lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderHomeHeader(width),
		content,
		m.renderChatComposer(width),
		m.renderHomeFooter(width),
	))
}

func (m *Model) renderSmallTerminalFallback(width, height int) string {
	lines := []string{
		m.theme.brand.Render("TERMCHAT"),
		m.theme.room.Render("Terminal too small"),
		m.theme.muted.Render("Resize to 40x8 or larger"),
	}
	if height < len(lines) {
		lines = lines[:height]
	}
	for index := range lines {
		lines[index] = ansi.Truncate(lines[index], width, "")
	}
	return m.theme.root.Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func (m *Model) renderChatHeader(width int) string {
	roomName := "room"
	if m.room != nil {
		roomName = m.room.Name
	}
	username := m.session.User.Username
	if username == "" {
		username = "anonymous"
	}

	left := m.theme.brand.Render("TERMCHAT") + m.theme.muted.Render(" // ") + m.theme.room.Render("#"+roomName)
	right := m.renderConnectionBadge() + " " + m.theme.input.Render(username)
	line := joinSidesPreservingRight(left, right, width)
	return m.theme.header.Width(width).Render(line)
}

func (m *Model) renderHomeHeader(width int) string {
	username := m.session.User.Username
	if username == "" {
		username = "anonymous"
	}

	left := m.theme.brand.Render("TERMCHAT") + m.theme.muted.Render(" // ") + m.theme.room.Render("HOME")
	right := m.renderConnectionBadge() + " " + m.theme.input.Render(username)
	return m.theme.header.Width(width).Render(joinSidesPreservingRight(left, right, width))
}

func (m *Model) renderChatComposer(width int) string {
	contentWidth := maxInt(1, width-2)
	return m.theme.composer.Width(contentWidth).Render(m.commandInput.View())
}

func (m *Model) renderChatFooter(width int) string {
	hints := "PgUp/PgDn scroll • /help commands • Ctrl+C quit"
	if m.status == "" {
		return m.theme.footer.Width(width).Render(ansi.Truncate(hints, width, ""))
	}
	return m.theme.footer.Width(width).Render(joinSides(m.renderStatus(), hints, width))
}

func (m *Model) renderHomeFooter(width int) string {
	hints := "/help commands • Ctrl+C quit"
	if m.status == "" {
		return m.theme.footer.Width(width).Render(ansi.Truncate(hints, width, ""))
	}
	return m.theme.footer.Width(width).Render(joinSides(m.renderStatus(), hints, width))
}

func (m *Model) terminalSize() (int, int) {
	return maxInt(1, m.width), maxInt(1, m.height)
}

func joinSides(left, right string, width int) string {
	if width < 1 {
		return ""
	}
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	if leftWidth+rightWidth+1 > width {
		return ansi.Truncate(left, width, "")
	}
	return left + strings.Repeat(" ", width-leftWidth-rightWidth) + right
}

func joinSidesPreservingRight(left, right string, width int) string {
	if width < 1 {
		return ""
	}
	rightWidth := lipgloss.Width(right)
	if rightWidth >= width {
		return ansi.Truncate(right, width, "")
	}
	left = ansi.Truncate(left, width-rightWidth-1, "")
	leftWidth := lipgloss.Width(left)
	return left + strings.Repeat(" ", width-leftWidth-rightWidth) + right
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
