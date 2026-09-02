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
	m.syncThemePicker()
	viewportHeight := height - chatHeaderHeight - chatComposerHeight - chatFooterHeight - m.notificationTrayHeight(width) - m.themePickerHeight(width)
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	wasAtBottom := m.viewport.AtBottom()
	m.viewport.Width = width
	m.viewport.Height = viewportHeight
	m.viewport.Style = m.theme.viewport.Width(width)
	m.sizeCommandInput(width)
	m.commandInput.Placeholder = "Type a message or /help"
	m.viewport.SetContent(m.renderMessages())
	if wasAtBottom {
		m.viewport.GotoBottom()
		m.pendingHistoryOffset = 0
	} else if m.pendingHistoryOffset > 0 {
		m.viewport.YOffset += m.pendingHistoryOffset
		m.pendingHistoryOffset = 0
	}
}

func (m *Model) renderChatView() string {
	width, height := m.terminalSize()
	if width < minimumChatWidth || height < minimumChatHeight {
		return m.renderSmallTerminalFallback(width, height)
	}
	m.syncChatLayout()

	sections := []string{m.renderChatHeader(width)}
	if tray := m.renderNotificationTray(width); tray != "" {
		sections = append(sections, tray)
	}
	sections = append(sections, m.viewport.View())
	if m.themePickerOpen {
		sections = append(sections, m.renderThemePicker(width))
	}
	sections = append(sections, m.renderChatComposer(width), m.renderChatFooter(width))
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return m.theme.root.Width(width).Height(height).Render(content)
}

func (m *Model) renderHomeView() string {
	width, height := m.terminalSize()
	m.syncThemePicker()
	m.sizeCommandInput(width)
	m.commandInput.Placeholder = "/join private_room"

	contentHeight := maxInt(1, height-chatHeaderHeight-chatComposerHeight-chatFooterHeight-m.notificationTrayHeight(width)-m.themePickerHeight(width))
	guide := strings.Join([]string{
		"WELCOME, " + strings.ToUpper(m.session.User.Username),
		"",
		m.theme.brand.Render("START HERE"),
		"1. Join a room you know",
		m.theme.muted.Render("   /join <room-name>"),
		"2. Create a new private room",
		m.theme.muted.Render("   /createroom <room-name> <password>"),
		"3. Start a temporary direct chat",
		m.theme.muted.Render("   /dm <username>"),
		"",
		m.theme.muted.Render("Room names are private; there is no public room list."),
		"",
		"Type /help for every command and example.",
	}, "\n")
	content := m.theme.viewport.Width(width).Height(contentHeight).Render(guide)

	sections := []string{m.renderHomeHeader(width)}
	if tray := m.renderNotificationTray(width); tray != "" {
		sections = append(sections, tray)
	}
	sections = append(sections, content)
	if m.themePickerOpen {
		sections = append(sections, m.renderThemePicker(width))
	}
	sections = append(sections, m.renderChatComposer(width), m.renderHomeFooter(width))
	return m.theme.root.Width(width).Height(height).Render(lipgloss.JoinVertical(lipgloss.Left, sections...))
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
	prefix := "#"
	if m.direct != nil {
		roomName = m.direct.Username
		prefix = "@"
	} else if m.room != nil {
		roomName = m.room.Name
	}
	username := m.session.User.Username
	if username == "" {
		username = "anonymous"
	}

	left := m.theme.brand.Render("TERMCHAT") + m.theme.muted.Render(" // ") + m.theme.room.Render(prefix+roomName)
	right := m.renderConnectionBadge() + m.theme.root.Render(" ") + m.theme.input.Render(username)
	line := joinSidesPreservingRight(left, right, width, m.theme.root)
	return m.theme.header.Width(width).Render(line)
}

func (m *Model) renderHomeHeader(width int) string {
	username := m.session.User.Username
	if username == "" {
		username = "anonymous"
	}

	left := m.theme.brand.Render("TERMCHAT") + m.theme.muted.Render(" // ") + m.theme.room.Render("HOME")
	right := m.renderConnectionBadge() + m.theme.root.Render(" ") + m.theme.input.Render(username)
	return m.theme.header.Width(width).Render(joinSidesPreservingRight(left, right, width, m.theme.root))
}

func (m *Model) renderChatComposer(width int) string {
	contentWidth := maxInt(1, width-m.theme.composer.GetHorizontalFrameSize())
	input := strings.TrimRight(m.commandInput.View(), " ")
	content := m.theme.root.Width(contentWidth).Render(input)
	return m.theme.composer.Width(width).Render(content)
}

func (m *Model) renderThemePicker(width int) string {
	lines := m.themePickerLines(width)
	for index := range lines {
		lines[index] = m.theme.root.Width(width).Render(lines[index])
	}
	return strings.Join(lines, "\n")
}

func (m *Model) themePickerHeight(width int) int {
	if !m.themePickerOpen {
		return 0
	}
	return len(m.themePickerLines(width))
}

func (m *Model) themePickerLines(width int) []string {
	prefix := m.theme.brand.Render("THEMES ")
	lines := make([]string, 0, 2)
	line := prefix
	for index, name := range themeNames() {
		label := m.theme.input.Render(" " + name + " ")
		if name == m.theme.name {
			label = m.theme.brand.Render("[" + name + "]")
		}
		if index == m.themePickerIndex {
			if name == m.theme.name {
				label = m.theme.statusSuccess.Render(">[" + name + "]<")
			} else {
				label = m.theme.statusSuccess.Render(">" + name + "<")
			}
		}
		separator := ""
		if lipgloss.Width(line) > 0 {
			separator = m.theme.muted.Render(" ")
		}
		if lipgloss.Width(line)+lipgloss.Width(separator)+lipgloss.Width(label) > width && lipgloss.Width(line) > 0 {
			lines = append(lines, line)
			line = label
			continue
		}
		line += separator + label
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func (m *Model) renderChatFooter(width int) string {
	hints := "PgUp/PgDn scroll • /help commands • Ctrl+C quit"
	if m.themePickerOpen {
		hints = "Tab next • Enter apply • Esc cancel"
		return m.theme.footer.Width(width).Render(ansi.Truncate(hints, width, ""))
	}
	return m.theme.footer.Width(width).Render(ansi.Truncate(hints, width, ""))
}

func (m *Model) renderHomeFooter(width int) string {
	hints := "/help commands • Ctrl+C quit"
	if m.themePickerOpen {
		hints = "Tab next • Enter apply • Esc cancel"
		return m.theme.footer.Width(width).Render(ansi.Truncate(hints, width, ""))
	}
	return m.theme.footer.Width(width).Render(ansi.Truncate(hints, width, ""))
}

func (m *Model) terminalSize() (int, int) {
	return maxInt(1, m.width), maxInt(1, m.height)
}

func (m *Model) renderFullScreen(content string) string {
	width, height := m.terminalSize()
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		lines[index] = m.theme.viewport.Width(width).Render(line)
	}
	return m.theme.root.Width(width).Height(height).Render(strings.Join(lines, "\n"))
}

func joinSides(left, right string, width int, fill lipgloss.Style) string {
	if width < 1 {
		return ""
	}
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	if leftWidth+rightWidth+1 > width {
		return ansi.Truncate(left, width, "")
	}
	return left + fill.Render(strings.Repeat(" ", width-leftWidth-rightWidth)) + right
}

func joinSidesPreservingRight(left, right string, width int, fill lipgloss.Style) string {
	if width < 1 {
		return ""
	}
	rightWidth := lipgloss.Width(right)
	if rightWidth >= width {
		return ansi.Truncate(right, width, "")
	}
	left = ansi.Truncate(left, width-rightWidth-1, "")
	leftWidth := lipgloss.Width(left)
	return left + fill.Render(strings.Repeat(" ", width-leftWidth-rightWidth)) + right
}

func (m *Model) sizeCommandInput(width int) {
	contentWidth := maxInt(1, width-m.theme.composer.GetHorizontalFrameSize())
	promptWidth := lipgloss.Width(m.commandInput.Prompt)
	m.commandInput.Width = maxInt(1, contentWidth-promptWidth-1)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
