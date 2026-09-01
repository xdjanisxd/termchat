package tui

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"termchat.local/termchat/internal/client"
	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/protocol"
)

func TestHomeViewShowsOfflineBeforeChatConnection(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	model.session.User.Username = "alice"

	plain := ansi.Strip(model.View())
	if !strings.Contains(plain, "[OFFLINE]") {
		t.Fatalf("View() missing offline connection state:\n%s", plain)
	}
}

func TestAuthenticationSuccessShowsConnectingWhileChatConnects(t *testing.T) {
	t.Parallel()

	api, err := client.New("http://example.test")
	if err != nil {
		t.Fatalf("client.New() error = %v", err)
	}
	model := NewModel(api)
	session := client.Session{Token: "token"}
	session.User.ID = "user-1"
	session.User.Username = "alice"

	command := updateModel(t, model, authResultMsg{session: session})

	if command == nil {
		t.Fatal("authentication success did not start chat connection")
	}
	if model.connectionState != connectionConnecting {
		t.Fatalf("connectionState = %d, want %d", model.connectionState, connectionConnecting)
	}
	plain := ansi.Strip(model.View())
	if !strings.Contains(plain, "[CONNECTING]") {
		t.Fatalf("View() missing connecting state:\n%s", plain)
	}
}

func TestSuccessfulChatConnectionShowsOnlineAcrossViews(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	model.session.User.Username = "alice"
	model.connectionState = connectionConnecting

	updateModel(t, model, connectResultMsg{})

	if model.connectionState != connectionOnline {
		t.Fatalf("connectionState = %d, want %d", model.connectionState, connectionOnline)
	}
	if plain := ansi.Strip(model.View()); !strings.Contains(plain, "[ONLINE]") {
		t.Fatalf("Home View() missing online state:\n%s", plain)
	}
	model.screen = ScreenChat
	if plain := ansi.Strip(model.View()); !strings.Contains(plain, "[ONLINE] alice") {
		t.Fatalf("Chat View() missing online state:\n%s", plain)
	}
}

func TestFailedChatConnectionSchedulesReconnect(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	model.session.User.Username = "alice"
	model.connectionState = connectionConnecting

	command := updateModel(t, model, connectResultMsg{err: errors.New("unreachable")})

	if command == nil {
		t.Fatal("failed connection did not schedule a retry")
	}
	if model.connectionState != connectionReconnecting {
		t.Fatalf("connectionState = %d, want %d", model.connectionState, connectionReconnecting)
	}
	plain := ansi.Strip(model.View())
	if !strings.Contains(plain, "[RECONNECTING]") {
		t.Fatalf("View() missing reconnecting state after connection failure:\n%s", plain)
	}
}

func TestLostChatConnectionSchedulesReconnect(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenChat
	model.session.User.Username = "alice"
	model.connectionState = connectionOnline

	command := updateModel(t, model, serverEventMsg{err: errors.New("connection reset")})

	if command == nil {
		t.Fatal("lost connection did not schedule reconnect")
	}
	if model.connectionState != connectionReconnecting {
		t.Fatalf("connectionState = %d, want %d", model.connectionState, connectionReconnecting)
	}
	plain := ansi.Strip(model.View())
	if !strings.Contains(plain, "[RECONNECTING] alice") {
		t.Fatalf("Chat View() missing reconnecting state after disconnect:\n%s", plain)
	}
}

func TestReconnectRejoinsTheLastRoomWithProcessOnlyCredentials(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.room = &domain.PublicRoom{ID: "room-1", Name: "private_room"}
	model.rejoinRoom = roomRejoin{Name: "private_room", Password: "roompass"}

	event, ok := model.rejoinEvent()

	if !ok {
		t.Fatal("rejoinEvent() returned no event for the last room")
	}
	if event.Type != "join_room" || event.RoomName != "private_room" || event.Password != "roompass" {
		t.Fatalf("rejoin event = %#v", event)
	}
}

func TestFailedAutomaticRejoinReturnsToHomeAndForgetsRoomCredential(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenChat
	model.room = &domain.PublicRoom{ID: "room-1", Name: "private_room"}
	model.rejoinRoom = roomRejoin{Name: "private_room", Password: "roompass"}
	model.rejoinInFlight = true

	updateModel(t, model, serverEventMsg{event: protocol.ServerEvent{Type: "error", Error: &protocol.EventError{Message: "room password is no longer valid"}}})

	if model.screen != ScreenHome || model.room != nil {
		t.Fatalf("failed automatic rejoin left model on screen %v with room %#v", model.screen, model.room)
	}
	if model.rejoinRoom != (roomRejoin{}) {
		t.Fatalf("rejoin room credential remained in memory: %#v", model.rejoinRoom)
	}
}

func TestOnlineConnectionSchedulesA45SecondHeartbeat(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.connectionState = connectionOnline

	command := updateModel(t, model, heartbeatTickMsg{generation: model.connectionGeneration})

	if heartbeatInterval != 45*time.Second {
		t.Fatalf("heartbeat interval = %s, want 45s", heartbeatInterval)
	}
	if command == nil {
		t.Fatal("heartbeat tick did not send a ping")
	}
}

func TestReconnectDelayUsesBoundedExponentialBackoff(t *testing.T) {
	t.Parallel()

	for attempt, want := range map[int]time.Duration{1: time.Second, 2: 2 * time.Second, 3: 4 * time.Second, 10: 30 * time.Second} {
		if got := reconnectRetryDelay(attempt); got != want {
			t.Fatalf("reconnectRetryDelay(%d) = %s, want %s", attempt, got, want)
		}
	}
}

func TestHeartbeatSendsPingAndAcceptsMatchingPong(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "test complete")

		var event protocol.ClientEvent
		if err := wsjson.Read(r.Context(), conn, &event); err != nil {
			return
		}
		if event.Type != "ping" {
			t.Errorf("event type = %q, want ping", event.Type)
			return
		}
		_ = wsjson.Write(r.Context(), conn, protocol.ServerEvent{Type: "pong", RequestID: event.RequestID})
	}))
	defer server.Close()

	api, err := client.New(server.URL)
	if err != nil {
		t.Fatalf("client.New() error = %v", err)
	}
	api.SetToken("jwt-token")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := api.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer api.Disconnect()

	model := NewModel(api)
	model.connectionState = connectionOnline
	model.connectionGeneration = 1
	batchCommand := updateModel(t, model, heartbeatTickMsg{generation: 1})
	batch, ok := batchCommand().(tea.BatchMsg)
	if !ok || len(batch) != 2 {
		t.Fatalf("heartbeat batch = %#v, want send and next-tick commands", batchCommand())
	}
	heartbeatSent, ok := batch[0]().(heartbeatSentMsg)
	if !ok || heartbeatSent.err != nil {
		t.Fatalf("heartbeat send result = %#v", heartbeatSent)
	}
	updateModel(t, model, heartbeatSent)

	updateModel(t, model, model.listenCmd()())
	if model.pendingHeartbeatID != "" {
		t.Fatalf("pending heartbeat = %q after matching pong", model.pendingHeartbeatID)
	}
}

func TestReconnectRejoinsLastRoomOverNewWebSocket(t *testing.T) {
	t.Parallel()

	firstConnected := make(chan struct{})
	rejoinEvents := make(chan protocol.ClientEvent, 1)
	var connectionsMu sync.Mutex
	connections := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "test complete")

		connectionsMu.Lock()
		connections++
		connectionNumber := connections
		connectionsMu.Unlock()
		if connectionNumber == 1 {
			close(firstConnected)
			_, _, _ = conn.Read(r.Context())
			return
		}
		var event protocol.ClientEvent
		if err := wsjson.Read(r.Context(), conn, &event); err == nil {
			rejoinEvents <- event
		}
	}))
	defer server.Close()

	api, err := client.New(server.URL)
	if err != nil {
		t.Fatalf("client.New() error = %v", err)
	}
	api.SetToken("jwt-token")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := api.Connect(ctx); err != nil {
		t.Fatalf("initial Connect() error = %v", err)
	}
	defer api.Disconnect()
	select {
	case <-firstConnected:
	case <-ctx.Done():
		t.Fatal("first websocket connection was not accepted")
	}

	model := NewModel(api)
	model.connectionState = connectionOnline
	model.connectionGeneration = 1
	model.room = &domain.PublicRoom{ID: "room-1", Name: "private_room"}
	model.rejoinRoom = roomRejoin{Name: "private_room", Password: "roompass"}
	updateModel(t, model, serverEventMsg{err: errors.New("connection reset")})

	connectCommand := updateModel(t, model, reconnectMsg{})
	connectResult, ok := connectCommand().(connectResultMsg)
	if !ok || connectResult.err != nil {
		t.Fatalf("reconnect result = %#v", connectResult)
	}
	postConnectCommand := updateModel(t, model, connectResult)
	batch, ok := postConnectCommand().(tea.BatchMsg)
	if !ok || len(batch) != 3 {
		t.Fatalf("post-reconnect batch = %#v, want listen, heartbeat, and rejoin", postConnectCommand())
	}
	if sent, ok := batch[2]().(eventSentMsg); !ok || sent.err != nil {
		t.Fatalf("rejoin send result = %#v", sent)
	}

	select {
	case event := <-rejoinEvents:
		if event.Type != "join_room" || event.RoomName != "private_room" || event.Password != "roompass" {
			t.Fatalf("rejoin event = %#v", event)
		}
	case <-ctx.Done():
		t.Fatal("reconnect did not rejoin the last room")
	}
}

func TestConnectionStatesUseSemanticColors(t *testing.T) {
	t.Parallel()

	theme := newAmberCRTTheme()
	tests := []struct {
		name string
		got  lipgloss.TerminalColor
		want lipgloss.AdaptiveColor
	}{
		{name: "online green", got: theme.connOnline.GetForeground(), want: lipgloss.AdaptiveColor{Light: "#1F7A3A", Dark: "#7DFF91"}},
		{name: "offline red", got: theme.connOffline.GetForeground(), want: lipgloss.AdaptiveColor{Light: "#B42318", Dark: "#FF6B5E"}},
		{name: "connecting amber", got: theme.connPending.GetForeground(), want: lipgloss.AdaptiveColor{Light: "#663A00", Dark: "#FFD27A"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got, want := fmt.Sprint(test.got), fmt.Sprint(test.want); got != want {
				t.Fatalf("foreground = %q, want %q", got, want)
			}
		})
	}
}

func TestChatHeaderPreservesConnectionStateAtSupportedWidths(t *testing.T) {
	t.Parallel()

	states := []struct {
		name  string
		state connectionState
		badge string
	}{
		{name: "offline", state: connectionOffline, badge: "[OFFLINE]"},
		{name: "connecting", state: connectionConnecting, badge: "[CONNECTING]"},
		{name: "online", state: connectionOnline, badge: "[ONLINE]"},
	}
	sizes := []struct {
		name     string
		width    int
		room     string
		username string
	}{
		{name: "minimum width", width: minimumChatWidth, room: "private_room", username: "alice"},
		{name: "maximum names at common width", width: 80, room: strings.Repeat("r", 32), username: strings.Repeat("u", 24)},
	}

	for _, state := range states {
		for _, size := range sizes {
			t.Run(state.name+"/"+size.name, func(t *testing.T) {
				model := NewModel(nil)
				model.connectionState = state.state
				model.session.User.Username = size.username
				model.room = &domain.PublicRoom{Name: size.room}

				header := model.renderChatHeader(size.width)
				if plain := ansi.Strip(header); !strings.Contains(plain, state.badge) {
					t.Fatalf("renderChatHeader(%d) missing %s:\n%s", size.width, state.badge, plain)
				}
				if got := lipgloss.Width(header); got > size.width {
					t.Fatalf("renderChatHeader(%d) width = %d, want <= %d", size.width, got, size.width)
				}
			})
		}
	}
}
