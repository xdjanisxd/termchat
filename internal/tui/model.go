package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/google/uuid"

	"termchat.local/termchat/internal/client"
	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/protocol"
)

type Screen uint8

const (
	ScreenWelcome Screen = iota
	ScreenLogin
	ScreenRegister
	ScreenHome
	ScreenRoomPassword
	ScreenChat
	ScreenHelp
)

type authResultMsg struct {
	session client.Session
	err     error
}

type sessionRestoreMsg struct {
	session    client.Session
	connectErr error
	loadErr    error
}

type connectResultMsg struct {
	err error
}

type eventSentMsg struct {
	err error
}

type serverEventMsg struct {
	event protocol.ServerEvent
	err   error
}

type Model struct {
	api      *client.Client
	sessions client.SessionStore
	theme    tuiTheme

	screen           Screen
	session          client.Session
	username         textinput.Model
	password         textinput.Model
	commandInput     textinput.Model
	roomPassword     textinput.Model
	viewport         viewport.Model
	helpViewport     viewport.Model
	helpReturnScreen Screen
	pendingRoomName  string
	room             *domain.PublicRoom
	messages         []domain.Message
	focus            int
	status           string
	statusLevel      statusLevel
	connectionState  connectionState
	loading          bool
	width            int
	height           int
}

func NewModel(api *client.Client, sessions client.SessionStore) *Model {
	theme := newAmberCRTTheme()

	username := textinput.New()
	username.Prompt = "Username: "
	username.Placeholder = "alice_01"
	username.CharLimit = 24
	username.Focus()

	password := textinput.New()
	password.Prompt = "Password: "
	password.Placeholder = "at least 10 characters"
	password.EchoMode = textinput.EchoPassword
	password.EchoCharacter = '*'
	password.CharLimit = 128

	commandInput := textinput.New()
	commandInput.Prompt = "> "
	commandInput.Placeholder = "/join private_room"
	commandInput.CharLimit = domain.MaxMessageLength
	commandInput.Focus()

	roomPassword := textinput.New()
	roomPassword.Prompt = "Room password: "
	roomPassword.Placeholder = "at least 8 characters"
	roomPassword.EchoMode = textinput.EchoPassword
	roomPassword.EchoCharacter = '*'
	roomPassword.CharLimit = 128

	theme.applyInput(&username)
	theme.applyInput(&password)
	theme.applyInput(&commandInput)
	theme.applyInput(&roomPassword)

	return &Model{
		api: api, sessions: sessions, theme: theme, screen: ScreenWelcome,
		username: username, password: password, commandInput: commandInput,
		roomPassword: roomPassword, viewport: viewport.New(80, 19), helpViewport: viewport.New(80, 21), width: 80, height: 24,
	}
}

func (m *Model) Init() tea.Cmd {
	return func() tea.Msg {
		session, err := m.sessions.Load()
		if err != nil {
			return sessionRestoreMsg{loadErr: err}
		}
		m.api.SetToken(session.Token)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return sessionRestoreMsg{session: session, connectErr: m.api.Connect(ctx)}
	}
}

func (m *Model) Screen() Screen {
	return m.screen
}

func (m *Model) Session() client.Session {
	return m.session
}

func (m *Model) CurrentRoom() *domain.PublicRoom {
	return m.room
}

func (m *Model) Status() string {
	return m.status
}

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.screen == ScreenChat {
			m.syncChatLayout()
		} else if m.screen == ScreenHelp {
			m.syncHelpLayout()
		}
		return m, nil
	case sessionRestoreMsg:
		if msg.loadErr != nil {
			if !errors.Is(msg.loadErr, client.ErrNoSession) {
				m.setStatus(statusError, "Could not load saved session: "+msg.loadErr.Error())
			}
			return m, nil
		}
		m.session = msg.session
		m.screen = ScreenHome
		m.focusCommandInput()
		if msg.connectErr != nil {
			m.connectionState = connectionOffline
			m.setStatus(statusError, "Session loaded, but chat connection failed: "+msg.connectErr.Error())
			return m, nil
		}
		m.connectionState = connectionOnline
		m.setStatus(statusSuccess, "Session restored for "+msg.session.User.Username)
		return m, m.listenCmd()
	case connectResultMsg:
		if msg.err != nil && !errors.Is(msg.err, client.ErrAlreadyConnected) {
			m.connectionState = connectionOffline
			m.setStatus(statusError, "Chat connection failed: "+msg.err.Error())
			return m, nil
		}
		m.connectionState = connectionOnline
		m.setStatus(statusSuccess, "Connected as "+m.session.User.Username)
		return m, m.listenCmd()
	case authResultMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatus(statusError, msg.err.Error())
			return m, nil
		}
		m.api.SetToken(msg.session.Token)
		if err := m.sessions.Save(msg.session); err != nil {
			m.setStatus(statusError, "Could not save session: "+err.Error())
			return m, nil
		}
		m.session = msg.session
		m.password.SetValue("")
		m.setStatus(statusSuccess, "Authenticated as "+msg.session.User.Username)
		m.screen = ScreenHome
		m.focusCommandInput()
		m.connectionState = connectionConnecting
		return m, m.connectCmd()
	case eventSentMsg:
		if msg.err != nil {
			m.setStatus(statusError, msg.err.Error())
		}
		return m, nil
	case serverEventMsg:
		if msg.err != nil {
			m.connectionState = connectionOffline
			m.setStatus(statusError, "Connection lost: "+msg.err.Error())
			return m, nil
		}
		m.applyServerEvent(msg.event)
		if m.screen == ScreenChat {
			m.syncChatLayout()
		}
		return m, m.listenCmd()
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			_ = m.api.Disconnect()
			return m, tea.Quit
		}
		switch m.screen {
		case ScreenWelcome:
			return m.updateWelcome(msg)
		case ScreenLogin, ScreenRegister:
			return m.updateAuthentication(msg)
		case ScreenHome, ScreenChat:
			return m.updateCommandInput(msg)
		case ScreenRoomPassword:
			return m.updateRoomPassword(msg)
		case ScreenHelp:
			return m.updateHelp(msg)
		}
	}
	return m, nil
}

func (m *Model) updateWelcome(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(message.String()) {
	case "l":
		m.openAuthentication(ScreenLogin)
	case "r":
		m.openAuthentication(ScreenRegister)
	case "q", "esc":
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) updateAuthentication(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		m.screen = ScreenWelcome
		m.password.SetValue("")
		m.clearStatus()
		return m, nil
	case tea.KeyTab, tea.KeyShiftTab, tea.KeyUp, tea.KeyDown:
		m.focus = (m.focus + 1) % 2
		m.syncInputFocus()
		return m, nil
	case tea.KeyEnter:
		if m.loading {
			return m, nil
		}
		username := strings.TrimSpace(m.username.Value())
		password := m.password.Value()
		if username == "" || password == "" {
			m.setStatus(statusWarning, "Username and password are required.")
			return m, nil
		}
		register := m.screen == ScreenRegister
		m.loading = true
		m.setStatus(statusInfo, "Contacting server...")
		return m, m.authenticateCmd(register, username, password)
	}

	var command tea.Cmd
	if m.focus == 0 {
		m.username, command = m.username.Update(message)
	} else {
		m.password, command = m.password.Update(message)
	}
	return m, command
}

func (m *Model) updateCommandInput(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.screen == ScreenChat {
		if scrollMessage, ok := pageScrollMessage(message); ok {
			var command tea.Cmd
			m.viewport, command = m.viewport.Update(scrollMessage)
			return m, command
		}
	}
	if message.Type != tea.KeyEnter {
		var command tea.Cmd
		m.commandInput, command = m.commandInput.Update(message)
		return m, command
	}

	parsed, err := ParseInput(m.commandInput.Value())
	if err != nil {
		m.setStatus(statusError, err.Error())
		return m, nil
	}
	if parsed.Kind != CommandJoinRoom {
		m.commandInput.SetValue("")
	}
	return m.dispatchCommand(parsed)
}

func (m *Model) dispatchCommand(command Command) (tea.Model, tea.Cmd) {
	switch command.Kind {
	case CommandHelp:
		m.helpReturnScreen = m.screen
		m.screen = ScreenHelp
		m.commandInput.Blur()
		m.helpViewport.GotoTop()
		m.syncHelpLayout()
	case CommandCreateRoom:
		return m, m.sendEventCmd(protocol.ClientEvent{
			Type: "create_room", RequestID: uuid.NewString(), RoomName: command.Args[0], Password: command.Args[1],
		})
	case CommandJoinRoom:
		m.pendingRoomName = command.Args[0]
		m.commandInput.SetValue("")
		m.commandInput.Blur()
		m.roomPassword.SetValue("")
		m.roomPassword.Focus()
		m.screen = ScreenRoomPassword
		m.setStatus(statusInfo, "Enter the password for "+command.Args[0])
	case CommandMessage:
		if m.screen != ScreenChat || m.room == nil {
			m.setStatus(statusWarning, "Join a room before sending messages.")
			return m, nil
		}
		return m, m.sendEventCmd(protocol.ClientEvent{Type: "send_message", RequestID: uuid.NewString(), Content: command.Args[0]})
	case CommandLeaveRoom:
		return m, m.sendEventCmd(protocol.ClientEvent{Type: "leave_room", RequestID: uuid.NewString()})
	case CommandWho:
		return m, m.sendEventCmd(protocol.ClientEvent{Type: "who", RequestID: uuid.NewString()})
	case CommandChangeRoomPassword:
		return m, m.sendEventCmd(protocol.ClientEvent{
			Type: "change_room_password", RequestID: uuid.NewString(), NewPassword: command.Args[0],
		})
	case CommandDeleteRoom:
		return m, m.sendEventCmd(protocol.ClientEvent{Type: "delete_room", RequestID: uuid.NewString()})
	case CommandQuit:
		_ = m.api.Disconnect()
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) updateRoomPassword(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.Type {
	case tea.KeyEsc:
		m.roomPassword.SetValue("")
		m.pendingRoomName = ""
		m.screen = ScreenHome
		m.focusCommandInput()
		return m, nil
	case tea.KeyEnter:
		password := m.roomPassword.Value()
		if password == "" {
			m.setStatus(statusWarning, "Room password is required.")
			return m, nil
		}
		event := protocol.ClientEvent{
			Type: "join_room", RequestID: uuid.NewString(), RoomName: m.pendingRoomName, Password: password,
		}
		m.roomPassword.SetValue("")
		m.setStatus(statusInfo, "Joining "+m.pendingRoomName+"...")
		return m, m.sendEventCmd(event)
	}
	var command tea.Cmd
	m.roomPassword, command = m.roomPassword.Update(message)
	return m, command
}

func (m *Model) applyServerEvent(event protocol.ServerEvent) {
	switch event.Type {
	case "error":
		if event.Error != nil {
			m.setStatus(statusError, event.Error.Message)
		}
	case "room_joined":
		m.room = event.Room
		m.messages = append([]domain.Message(nil), event.Messages...)
		m.pendingRoomName = ""
		m.roomPassword.SetValue("")
		m.screen = ScreenChat
		m.focusCommandInput()
		if event.Room != nil {
			m.setStatus(statusSuccess, "Joined "+event.Room.Name)
		}
	case "new_message":
		if event.Message != nil {
			m.messages = append(m.messages, *event.Message)
		}
	case "room_left":
		m.room = nil
		m.messages = nil
		m.screen = ScreenHome
		m.focusCommandInput()
		m.setStatus(statusSuccess, "Left the room.")
	case "room_deleted":
		m.room = nil
		m.messages = nil
		m.screen = ScreenHome
		m.focusCommandInput()
		m.setStatus(statusWarning, "The room was deleted.")
	case "user_list":
		m.setStatus(statusInfo, "Online: "+strings.Join(event.Users, ", "))
	case "user_joined":
		m.setStatus(statusInfo, event.Username+" joined the room.")
	case "user_left":
		m.setStatus(statusWarning, event.Username+" left the room.")
	case "room_password_changed":
		m.setStatus(statusSuccess, "Room password changed.")
	case "pong":
		m.setStatus(statusSuccess, "Connected.")
	}
}

func (m *Model) openAuthentication(screen Screen) {
	m.screen = screen
	m.focus = 0
	m.clearStatus()
	m.commandInput.Blur()
	m.syncInputFocus()
}

func (m *Model) syncInputFocus() {
	if m.focus == 0 {
		m.username.Focus()
		m.password.Blur()
	} else {
		m.username.Blur()
		m.password.Focus()
	}
}

func (m *Model) focusCommandInput() {
	m.username.Blur()
	m.password.Blur()
	m.roomPassword.Blur()
	if m.screen == ScreenChat {
		m.commandInput.Placeholder = "Type a message or /help"
	} else {
		m.commandInput.Placeholder = "/join private_room"
	}
	m.commandInput.Focus()
}

func (m *Model) authenticateCmd(register bool, username, password string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var (
			session client.Session
			err     error
		)
		if register {
			session, err = m.api.Register(ctx, username, password)
		} else {
			session, err = m.api.Login(ctx, username, password)
		}
		return authResultMsg{session: session, err: err}
	}
}

func (m *Model) connectCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return connectResultMsg{err: m.api.Connect(ctx)}
	}
}

func (m *Model) sendEventCmd(event protocol.ClientEvent) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return eventSentMsg{err: m.api.Send(ctx, event)}
	}
}

func (m *Model) listenCmd() tea.Cmd {
	return func() tea.Msg {
		event, err := m.api.Receive(context.Background())
		return serverEventMsg{event: event, err: err}
	}
}

func (m *Model) View() string {
	title := m.theme.title.Render("TermChat")
	status := ""
	if m.status != "" {
		status = "\n\n" + m.renderStatus()
	}

	switch m.screen {
	case ScreenWelcome:
		return fmt.Sprintf("%s\n\n[L] Login\n[R] Register\n[Q] Quit%s\n", title, status)
	case ScreenLogin, ScreenRegister:
		action := "Login"
		if m.screen == ScreenRegister {
			action = "Register"
		}
		return fmt.Sprintf("%s — %s\n\n%s\n%s\n\nTab: switch field • Enter: submit • Esc: back%s\n",
			title, action, m.username.View(), m.password.View(), status)
	case ScreenHome:
		return fmt.Sprintf("%s\n\nLogged in as: %s  %s\n\n%s\n\n/createroom <name> <password> • /join <name> • /q%s\n",
			title, m.session.User.Username, m.renderConnectionBadge(), m.commandInput.View(), status)
	case ScreenRoomPassword:
		return fmt.Sprintf("%s\n\nJoin room: %s\n\n%s\n\nEnter: join • Esc: cancel%s\n",
			title, m.pendingRoomName, m.roomPassword.View(), status)
	case ScreenChat:
		return m.renderChatView()
	case ScreenHelp:
		return m.renderHelpView()
	default:
		return title + status + "\n"
	}
}

func (m *Model) renderMessages() string {
	if len(m.messages) == 0 {
		return m.theme.emptyState.Render("No messages yet.")
	}
	width, _ := m.terminalSize()
	lines := make([]string, 0, len(m.messages))
	for _, message := range m.messages {
		author := message.Username
		authorStyle := m.theme.messageUser
		if message.UserID == m.session.User.ID {
			author = "YOU"
			authorStyle = m.theme.messageSelf
		}
		prefix := m.theme.timestamp.Render(message.CreatedAt.Local().Format("15:04")) + "  " + authorStyle.Render(author) + "  "
		prefixWidth := lipgloss.Width(prefix)
		bodyWidth := maxInt(1, width-prefixWidth)
		body := ansi.Wordwrap(m.theme.messageBody.Render(message.Content), bodyWidth, " ")
		body = ansi.Hardwrap(body, bodyWidth, false)
		bodyLines := strings.Split(body, "\n")
		rendered := make([]string, 0, len(bodyLines))
		for index, bodyLine := range bodyLines {
			if index == 0 {
				rendered = append(rendered, prefix+bodyLine)
				continue
			}
			rendered = append(rendered, strings.Repeat(" ", prefixWidth)+bodyLine)
		}
		lines = append(lines, strings.Join(rendered, "\n"))
	}
	return strings.Join(lines, "\n")
}
