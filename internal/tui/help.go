package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

const (
	helpHeaderHeight = 2
	helpFooterHeight = 1
	wideHelpWidth    = 56
)

type helpCommand struct {
	usage       string
	description string
}

type helpSection struct {
	title    string
	commands []helpCommand
}

var helpSections = []helpSection{
	{title: "ROOMS", commands: []helpCommand{
		{usage: "/createroom <name> <password>", description: "Create a private room"},
		{usage: "/join <name>", description: "Join a private room"},
	}},
	{title: "CHAT", commands: []helpCommand{
		{usage: "/who", description: "Show users in the room"},
		{usage: "/l", description: "Leave the current room"},
	}},
	{title: "OWNER", commands: []helpCommand{
		{usage: "/roompasswd <password>", description: "Change the room password"},
		{usage: "/deleteroom", description: "Delete the current room"},
	}},
	{title: "APP", commands: []helpCommand{
		{usage: "/help", description: "Show this screen"},
		{usage: "/q", description: "Quit TermChat"},
	}},
}

func (m *Model) syncHelpLayout() {
	width, height := m.terminalSize()
	m.helpViewport.Width = width
	m.helpViewport.Height = maxInt(1, height-helpHeaderHeight-helpFooterHeight)
	m.helpViewport.Style = m.theme.viewport.Width(width)
	m.helpViewport.SetContent(m.renderHelpContent(width))
}

func (m *Model) renderHelpContent(width int) string {
	lines := make([]string, 0, len(helpSections)*4)
	for sectionIndex, section := range helpSections {
		if sectionIndex > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, m.theme.brand.Render(section.title))
		for _, command := range section.commands {
			if width >= wideHelpWidth {
				line := fmt.Sprintf("%-34s%s", command.usage, command.description)
				lines = append(lines, m.theme.messageBody.Render(line))
				continue
			}
			usage := ansi.Hardwrap(m.theme.prompt.Render(command.usage), width, false)
			descriptionWidth := maxInt(1, width-2)
			description := ansi.Wordwrap(m.theme.muted.Render(command.description), descriptionWidth, " ")
			description = ansi.Hardwrap(description, descriptionWidth, false)
			descriptionLines := strings.Split(description, "\n")
			for index := range descriptionLines {
				descriptionLines[index] = "  " + descriptionLines[index]
			}
			lines = append(lines, usage, strings.Join(descriptionLines, "\n"))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) updateHelp(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		m.screen = m.helpReturnScreen
		m.focusCommandInput()
		return m, nil
	case tea.KeyPgUp, tea.KeyPgDown:
		var command tea.Cmd
		m.helpViewport, command = m.helpViewport.Update(message)
		return m, command
	}
	return m, nil
}

func (m *Model) renderHelpView() string {
	width, height := m.terminalSize()
	m.syncHelpLayout()
	headerText := m.theme.brand.Render("TERMCHAT") + m.theme.muted.Render(" // ") + m.theme.room.Render("HELP")
	header := m.theme.header.Width(width).Render(ansi.Truncate(headerText, width, ""))
	footer := m.theme.footer.Width(width).Render(ansi.Truncate("PgUp/PgDn scroll • Esc close", width, ""))
	content := lipgloss.JoinVertical(lipgloss.Left, header, m.helpViewport.View(), footer)
	return m.theme.root.Width(width).Height(height).Render(content)
}
