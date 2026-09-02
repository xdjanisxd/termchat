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

type reconnectMsg struct{}

type heartbeatTickMsg struct {
	generation uint64
}

type heartbeatSentMsg struct {
	requestID string
	err       error
}

type heartbeatTimeoutMsg struct {
	generation uint64
	requestID  string
}

type roomRejoin struct {
	Name     string
	Password string
}

const (
	heartbeatInterval        = 45 * time.Second
	heartbeatResponseTimeout = 15 * time.Second
	reconnectMaxDelay        = 30 * time.Second
)

type Model struct {
	api   *client.Client
	theme tuiTheme

	screen               Screen
	session              client.Session
	username             textinput.Model
	password             textinput.Model
	commandInput         textinput.Model
	themePickerOpen      bool
	themePickerIndex     int
	roomPassword         textinput.Model
	viewport             viewport.Model
	helpViewport         viewport.Model
	helpReturnScreen     Screen
	pendingRoomName      string
	pendingRoomPass      string
	rejoinRoom           roomRejoin
	rejoinInFlight       bool
	room                 *domain.PublicRoom
	direct               *protocol.DirectIdentity
	directSessionID      string
	pendingDirectInvite  string
	pendingDirectSender  *protocol.DirectIdentity
	messages             []domain.Message
	historyLoading       bool
	historyHasMore       bool
	pendingHistoryOffset int
	focus                int
	status               string
	statusLevel          statusLevel
	notifications        []notification
	connectionState      connectionState
	connectionGeneration uint64
	reconnectAttempts    int
	pendingHeartbeatID   string
	loading              bool
	width                int
	height               int
}

func NewModel(api *client.Client) *Model {
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
		api: api, theme: theme, screen: ScreenWelcome,
		username: username, password: password, commandInput: commandInput,
		roomPassword: roomPassword, viewport: viewport.New(80, 19), helpViewport: viewport.New(80, 21), width: 80, height: 24,
	}
}

func (m *Model) Init() tea.Cmd {
	return nil
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

func (m *Model) endDirectLocally(status string) {
	m.direct = nil
	m.directSessionID = ""
	m.messages = nil
	m.historyHasMore = false
	m.historyLoading = false
	m.screen = ScreenHome
	m.focusCommandInput()
	m.setStatus(statusInfo, status)
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
	case connectResultMsg:
		if msg.err != nil && !errors.Is(msg.err, client.ErrAlreadyConnected) {
			return m, m.handleConnectionLoss("Chat connection failed: " + msg.err.Error())
		}
		wasReconnecting := m.connectionState == connectionReconnecting
		m.connectionState = connectionOnline
		m.connectionGeneration++
		m.reconnectAttempts = 0
		m.pendingHeartbeatID = ""
		if wasReconnecting {
			m.setStatus(statusSuccess, "Reconnected as "+m.session.User.Username)
		} else {
			m.setStatus(statusSuccess, "Connected as "+m.session.User.Username)
		}
		commands := []tea.Cmd{m.listenCmd(), m.scheduleHeartbeat()}
		if event, ok := m.rejoinEvent(); ok {
			m.rejoinInFlight = true
			m.setStatus(statusInfo, "Reconnected. Rejoining "+event.RoomName+"...")
			commands = append(commands, m.sendEventCmd(event))
		}
		return m, tea.Batch(commands...)
	case authResultMsg:
		m.loading = false
		if msg.err != nil {
			m.setStatus(statusError, msg.err.Error())
			return m, nil
		}
		m.api.SetToken(msg.session.Token)
		m.session = msg.session
		m.password.SetValue("")
		m.setStatus(statusSuccess, "Authenticated as "+msg.session.User.Username)
		m.screen = ScreenHome
		m.focusCommandInput()
		m.connectionState = connectionConnecting
		return m, m.connectCmd()
	case eventSentMsg:
		if msg.err != nil {
			return m, m.handleConnectionLoss("Could not send chat event: " + msg.err.Error())
		}
		return m, nil
	case serverEventMsg:
		if msg.err != nil {
			return m, m.handleConnectionLoss("Connection lost: " + msg.err.Error())
		}
		if msg.event.Type == "pong" && msg.event.RequestID == m.pendingHeartbeatID {
			m.pendingHeartbeatID = ""
		}
		m.applyServerEvent(msg.event)
		if m.screen == ScreenChat {
			m.syncChatLayout()
		}
		return m, m.listenCmd()
	case reconnectMsg:
		if m.connectionState != connectionReconnecting {
			return m, nil
		}
		m.connectionState = connectionConnecting
		m.setStatus(statusInfo, "Reconnecting to chat...")
		return m, m.connectCmd()
	case heartbeatTickMsg:
		if m.connectionState != connectionOnline || msg.generation != m.connectionGeneration || m.pendingHeartbeatID != "" {
			return m, nil
		}
		requestID := uuid.NewString()
		return m, tea.Batch(m.sendHeartbeatCmd(requestID), m.scheduleHeartbeat())
	case heartbeatSentMsg:
		if msg.err != nil {
			return m, m.handleConnectionLoss("Heartbeat failed: " + msg.err.Error())
		}
		m.pendingHeartbeatID = msg.requestID
		return m, m.scheduleHeartbeatTimeout(msg.requestID)
	case heartbeatTimeoutMsg:
		if msg.generation == m.connectionGeneration && msg.requestID == m.pendingHeartbeatID {
			return m, m.handleConnectionLoss("Heartbeat timed out")
		}
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			if m.api != nil {
				_ = m.api.Disconnect()
			}
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
	if m.screen == ScreenChat && (message.Type == tea.KeyPgUp || message.Type == tea.KeyPgDown) {
		var command tea.Cmd
		m.viewport, command = m.viewport.Update(message)
		if message.Type == tea.KeyPgUp && m.direct == nil && m.viewport.AtTop() && m.historyHasMore && !m.historyLoading && len(m.messages) > 0 {
			m.historyLoading = true
			return m, tea.Batch(command, m.sendEventCmd(protocol.ClientEvent{Type: "load_history", RequestID: uuid.NewString(), BeforeMessageID: m.messages[0].ID}))
		}
		return m, command
	}
	m.syncThemePicker()
	if m.themePickerOpen {
		switch message.Type {
		case tea.KeyTab:
			m.themePickerIndex = (m.themePickerIndex + 1) % len(themeNames())
			return m, nil
		case tea.KeyShiftTab:
			m.themePickerIndex = (m.themePickerIndex - 1 + len(themeNames())) % len(themeNames())
			return m, nil
		case tea.KeyEsc:
			m.commandInput.SetValue("")
			m.syncThemePicker()
			return m, nil
		case tea.KeyEnter:
			m.commandInput.SetValue("/theme " + themeNames()[m.themePickerIndex])
			m.syncThemePicker()
		}
	}
	if message.Type != tea.KeyEnter {
		var command tea.Cmd
		m.commandInput, command = m.commandInput.Update(message)
		m.syncThemePicker()
		return m, command
	}

	parsed, err := ParseInput(m.commandInput.Value())
	if err != nil {
		m.setStatus(statusError, userFacingCommandError(err))
		return m, nil
	}
	if parsed.Kind != CommandJoinRoom {
		m.commandInput.SetValue("")
	}
	return m.dispatchCommand(parsed)
}

func (m *Model) syncThemePicker() {
	shouldOpen := strings.TrimSpace(m.commandInput.Value()) == "/theme"
	if shouldOpen && !m.themePickerOpen {
		m.themePickerIndex = themeIndex(m.theme.name)
	}
	m.themePickerOpen = shouldOpen
}

func themeIndex(name string) int {
	for index, candidate := range themeNames() {
		if candidate == name {
			return index
		}
	}
	return 0
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
		m.pendingRoomName = command.Args[0]
		m.pendingRoomPass = command.Args[1]
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
	case CommandDirectMessage:
		if m.screen != ScreenHome {
			m.setStatus(statusWarning, "Leave the current chat before starting a direct chat.")
			return m, nil
		}
		return m, m.sendEventCmd(protocol.ClientEvent{Type: "direct_invite", RequestID: uuid.NewString(), TargetUsername: command.Args[0]})
	case CommandAcceptDirect:
		if m.pendingDirectInvite == "" {
			m.setStatus(statusWarning, "There is no direct invitation to accept.")
			return m, nil
		}
		return m, m.sendEventCmd(protocol.ClientEvent{Type: "direct_invite_accept", RequestID: uuid.NewString(), InviteID: m.pendingDirectInvite})
	case CommandDeclineDirect:
		if m.pendingDirectInvite == "" {
			m.setStatus(statusWarning, "There is no direct invitation to decline.")
			return m, nil
		}
		return m, m.sendEventCmd(protocol.ClientEvent{Type: "direct_invite_decline", RequestID: uuid.NewString(), InviteID: m.pendingDirectInvite})
	case CommandMessage:
		if m.screen != ScreenChat {
			m.setStatus(statusWarning, "You are not in a room. Join one with /join <room-name>, or create one with /createroom <room-name> <password>.")
			return m, nil
		}
		if m.direct != nil {
			return m, m.sendEventCmd(protocol.ClientEvent{Type: "send_direct_message", RequestID: uuid.NewString(), Content: command.Args[0]})
		}
		if m.room == nil {
			m.setStatus(statusWarning, "You are not in a room. Join one with /join <room-name>, or create one with /createroom <room-name> <password>.")
			return m, nil
		}
		return m, m.sendEventCmd(protocol.ClientEvent{Type: "send_message", RequestID: uuid.NewString(), Content: command.Args[0]})
	case CommandLeaveRoom:
		if m.direct != nil {
			return m, m.sendEventCmd(protocol.ClientEvent{Type: "leave_direct", RequestID: uuid.NewString()})
		}
		return m, m.sendEventCmd(protocol.ClientEvent{Type: "leave_room", RequestID: uuid.NewString()})
	case CommandWho:
		return m, m.sendEventCmd(protocol.ClientEvent{Type: "who", RequestID: uuid.NewString()})
	case CommandChangeRoomPassword:
		m.pendingRoomPass = command.Args[0]
		return m, m.sendEventCmd(protocol.ClientEvent{
			Type: "change_room_password", RequestID: uuid.NewString(), NewPassword: command.Args[0],
		})
	case CommandDeleteRoom:
		return m, m.sendEventCmd(protocol.ClientEvent{Type: "delete_room", RequestID: uuid.NewString()})
	case CommandTheme:
		if len(command.Args) == 0 {
			m.commandInput.SetValue("/theme")
			m.syncThemePicker()
			return m, nil
		}
		theme, ok := themeByName(command.Args[0])
		if !ok {
			m.setStatus(statusError, "Unknown theme: "+command.Args[0]+". Available: "+strings.Join(themeNames(), ", ")+".")
			return m, nil
		}
		m.applyTheme(theme)
		m.setStatus(statusSuccess, "Theme changed to "+theme.name+".")
	case CommandQuit:
		_ = m.api.Disconnect()
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) applyTheme(theme tuiTheme) {
	m.theme = theme
	theme.applyInput(&m.username)
	theme.applyInput(&m.password)
	theme.applyInput(&m.commandInput)
	theme.applyInput(&m.roomPassword)
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
		m.pendingRoomPass = password
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
			m.historyLoading = false
			m.pendingRoomPass = ""
			if m.rejoinInFlight {
				m.rejoinInFlight = false
				m.rejoinRoom = roomRejoin{}
				m.room = nil
				m.messages = nil
				m.screen = ScreenHome
				m.focusCommandInput()
				m.setStatus(statusError, "Could not rejoin the last room. Use /join <room-name> to enter it again.")
				return
			}
			m.setStatus(statusError, event.Error.Message)
		}
	case "room_joined":
		m.rejoinInFlight = false
		m.room = event.Room
		m.messages = append([]domain.Message(nil), event.Messages...)
		m.historyLoading = false
		m.historyHasMore = event.HasMore
		if event.Room != nil && m.pendingRoomPass != "" {
			m.rejoinRoom = roomRejoin{Name: event.Room.Name, Password: m.pendingRoomPass}
		}
		m.pendingRoomName = ""
		m.pendingRoomPass = ""
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
	case "message_history":
		m.historyLoading = false
		m.historyHasMore = event.HasMore
		m.prependHistory(event.Messages)
	case "room_left":
		m.room = nil
		m.rejoinRoom = roomRejoin{}
		m.rejoinInFlight = false
		m.messages = nil
		m.screen = ScreenHome
		m.focusCommandInput()
		m.setStatus(statusSuccess, "Left the room.")
	case "room_deleted":
		m.room = nil
		m.rejoinRoom = roomRejoin{}
		m.rejoinInFlight = false
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
		if m.room != nil && m.pendingRoomPass != "" {
			m.rejoinRoom = roomRejoin{Name: m.room.Name, Password: m.pendingRoomPass}
		}
		m.pendingRoomPass = ""
		m.setStatus(statusSuccess, "Room password changed.")
	case "direct_invite_received":
		m.pendingDirectInvite = event.InviteID
		m.pendingDirectSender = event.Counterpart
	case "direct_invite_sent":
		if event.Counterpart != nil {
			m.setStatus(statusInfo, "Direct invitation sent to "+event.Counterpart.Username+". It expires in 60 seconds.")
		}
	case "direct_invite_declined", "direct_invite_expired", "direct_invite_cancelled":
		m.pendingDirectInvite = ""
		m.pendingDirectSender = nil
		m.setStatus(statusInfo, "The direct invitation is no longer active.")
	case "direct_session_started":
		m.pendingDirectInvite = ""
		m.pendingDirectSender = nil
		m.room = nil
		m.rejoinRoom = roomRejoin{}
		m.direct = event.Counterpart
		m.directSessionID = event.DirectSessionID
		m.messages = nil
		m.historyHasMore = false
		m.historyLoading = false
		m.screen = ScreenChat
		m.focusCommandInput()
		if event.Counterpart != nil {
			m.setStatus(statusSuccess, "Direct chat with "+event.Counterpart.Username+" is ephemeral.")
		}
	case "new_direct_message":
		if event.DirectMessage != nil && m.direct != nil {
			m.messages = append(m.messages, domain.Message{ID: event.DirectMessage.ID, UserID: event.DirectMessage.UserID, Username: event.DirectMessage.Username, Content: event.DirectMessage.Content, CreatedAt: event.DirectMessage.CreatedAt})
		}
	case "direct_session_ended":
		m.endDirectLocally("Direct chat ended: " + event.Reason)
	case "pong":
		// A successful heartbeat is reflected by the persistent connection badge.
		// It must not displace user-visible notifications or required actions.
	}
}

func (m *Model) prependHistory(messages []domain.Message) {
	oldHeight := lipgloss.Height(m.renderMessages())
	seen := make(map[string]struct{}, len(m.messages))
	for _, message := range m.messages {
		seen[message.ID] = struct{}{}
	}
	older := make([]domain.Message, 0, len(messages))
	for _, message := range messages {
		if _, exists := seen[message.ID]; !exists {
			older = append(older, message)
		}
	}
	m.messages = append(older, m.messages...)
	if len(older) > 0 {
		m.pendingHistoryOffset = lipgloss.Height(m.renderMessages()) - oldHeight
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

func (m *Model) handleConnectionLoss(message string) tea.Cmd {
	if m.connectionState == connectionReconnecting {
		return nil
	}
	if m.direct != nil {
		m.direct = nil
		m.directSessionID = ""
		m.messages = nil
		m.historyHasMore = false
		m.historyLoading = false
		m.screen = ScreenHome
		m.focusCommandInput()
		message += " Direct chat ended and cannot be reconnected."
	}
	if m.api != nil {
		_ = m.api.Disconnect()
	}
	m.pendingHeartbeatID = ""
	m.connectionState = connectionReconnecting
	m.reconnectAttempts++
	m.setStatus(statusWarning, message+" Retrying shortly...")
	return tea.Tick(reconnectRetryDelay(m.reconnectAttempts), func(time.Time) tea.Msg { return reconnectMsg{} })
}

func reconnectRetryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Second
	for retry := 1; retry < attempt && delay < reconnectMaxDelay; retry++ {
		delay *= 2
	}
	if delay > reconnectMaxDelay {
		return reconnectMaxDelay
	}
	return delay
}

func (m *Model) scheduleHeartbeat() tea.Cmd {
	generation := m.connectionGeneration
	return tea.Tick(heartbeatInterval, func(time.Time) tea.Msg {
		return heartbeatTickMsg{generation: generation}
	})
}

func (m *Model) scheduleHeartbeatTimeout(requestID string) tea.Cmd {
	generation := m.connectionGeneration
	return tea.Tick(heartbeatResponseTimeout, func(time.Time) tea.Msg {
		return heartbeatTimeoutMsg{generation: generation, requestID: requestID}
	})
}

func (m *Model) sendHeartbeatCmd(requestID string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return heartbeatSentMsg{requestID: requestID, err: m.api.Send(ctx, protocol.ClientEvent{Type: "ping", RequestID: requestID})}
	}
}

func (m *Model) rejoinEvent() (protocol.ClientEvent, bool) {
	if m.room == nil || m.rejoinRoom.Name == "" || m.rejoinRoom.Password == "" {
		return protocol.ClientEvent{}, false
	}
	return protocol.ClientEvent{Type: "join_room", RequestID: uuid.NewString(), RoomName: m.rejoinRoom.Name, Password: m.rejoinRoom.Password}, true
}

func (m *Model) View() string {
	title := m.theme.title.Render("TermChat")
	status := ""
	if m.status != "" {
		status = "\n\n" + m.renderStatus()
	}

	switch m.screen {
	case ScreenWelcome:
		return m.renderFullScreen(fmt.Sprintf("%s\n\n[L] Login\n[R] Register\n[Q] Quit%s", title, status))
	case ScreenLogin, ScreenRegister:
		action := "Login"
		if m.screen == ScreenRegister {
			action = "Register"
		}
		return m.renderFullScreen(fmt.Sprintf("%s — %s\n\n%s\n%s\n\nTab: switch field • Enter: submit • Esc: back%s",
			title, action, m.username.View(), m.password.View(), status))
	case ScreenHome:
		return m.renderHomeView()
	case ScreenRoomPassword:
		return m.renderFullScreen(fmt.Sprintf("%s\n\nJoin room: %s\n\n%s\n\nEnter: join • Esc: cancel%s",
			title, m.pendingRoomName, m.roomPassword.View(), status))
	case ScreenChat:
		return m.renderChatView()
	case ScreenHelp:
		return m.renderHelpView()
	default:
		return m.renderFullScreen(title + status)
	}
}

func userFacingCommandError(err error) string {
	if !errors.Is(err, ErrInvalidCommand) {
		return err.Error()
	}

	message := strings.TrimPrefix(err.Error(), ErrInvalidCommand.Error()+": ")
	if strings.HasPrefix(message, "unknown command ") {
		return strings.ToUpper(message[:1]) + message[1:]
	}
	if strings.HasPrefix(message, "/join needs the right arguments.") {
		return "\"/join\" needs a room name. Try: /join <room-name>"
	}
	return strings.ToUpper(message[:1]) + message[1:]
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
		separator := m.theme.viewport.Render("  ")
		prefix := m.theme.timestamp.Render(message.CreatedAt.Local().Format("15:04")) + separator + authorStyle.Render(author) + separator
		prefixWidth := lipgloss.Width(prefix)
		bodyWidth := maxInt(1, width-prefixWidth)
		body := ansi.Wordwrap(m.theme.messageBody.Render(message.Content), bodyWidth, " ")
		body = ansi.Hardwrap(body, bodyWidth, false)
		bodyLines := strings.Split(body, "\n")
		rendered := make([]string, 0, len(bodyLines))
		for index, bodyLine := range bodyLines {
			line := bodyLine
			if index == 0 {
				line = prefix + bodyLine
			} else {
				line = m.theme.viewport.Render(strings.Repeat(" ", prefixWidth)) + bodyLine
			}
			rendered = append(rendered, m.theme.viewport.Width(width).Render(line))
		}
		lines = append(lines, strings.Join(rendered, "\n"))
	}
	return strings.Join(lines, "\n")
}
