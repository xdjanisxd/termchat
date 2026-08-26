package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"termchat.local/termchat/internal/client"
)

type Screen uint8

const (
	ScreenWelcome Screen = iota
	ScreenLogin
	ScreenRegister
	ScreenHome
	ScreenRoomPassword
	ScreenChat
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

type Model struct {
	api      *client.Client
	sessions client.SessionStore

	screen   Screen
	session  client.Session
	username textinput.Model
	password textinput.Model
	focus    int
	status   string
	loading  bool
	width    int
	height   int
}

func NewModel(api *client.Client, sessions client.SessionStore) *Model {
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

	return &Model{
		api: api, sessions: sessions, screen: ScreenWelcome,
		username: username, password: password, width: 80, height: 24,
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

func (m *Model) Status() string {
	return m.status
}

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case sessionRestoreMsg:
		if msg.loadErr != nil {
			if !errors.Is(msg.loadErr, client.ErrNoSession) {
				m.status = "Could not load saved session: " + msg.loadErr.Error()
			}
			return m, nil
		}
		m.session = msg.session
		m.screen = ScreenHome
		if msg.connectErr != nil {
			m.status = "Session loaded, but chat connection failed: " + msg.connectErr.Error()
		} else {
			m.status = "Session restored for " + msg.session.User.Username
		}
		return m, nil
	case connectResultMsg:
		if msg.err != nil && !errors.Is(msg.err, client.ErrAlreadyConnected) {
			m.status = "Chat connection failed: " + msg.err.Error()
		} else {
			m.status = "Connected as " + m.session.User.Username
		}
		return m, nil
	case authResultMsg:
		m.loading = false
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.api.SetToken(msg.session.Token)
		if err := m.sessions.Save(msg.session); err != nil {
			m.status = "Could not save session: " + err.Error()
			return m, nil
		}
		m.session = msg.session
		m.password.SetValue("")
		m.status = "Authenticated as " + msg.session.User.Username
		m.screen = ScreenHome
		return m, m.connectCmd()
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
		}
		switch m.screen {
		case ScreenWelcome:
			return m.updateWelcome(msg)
		case ScreenLogin, ScreenRegister:
			return m.updateAuthentication(msg)
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
		m.status = ""
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
			m.status = "Username and password are required."
			return m, nil
		}
		register := m.screen == ScreenRegister
		m.loading = true
		m.status = "Contacting server..."
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

func (m *Model) openAuthentication(screen Screen) {
	m.screen = screen
	m.focus = 0
	m.status = ""
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

func (m *Model) View() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#5A56E0", Dark: "#7D7AFF"}).Render("TermChat")
	status := ""
	if m.status != "" {
		status = "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#8A3B12", Dark: "#FFB86C"}).Render(m.status)
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
		return fmt.Sprintf("%s\n\nLogged in as: %s\n\n/createroom <name> <password>\n/join <name>\n/quit%s\n",
			title, m.session.User.Username, status)
	default:
		return title + status + "\n"
	}
}
