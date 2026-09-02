package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/protocol"
)

func TestDirectChatIsDiscardedOnConnectionLoss(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenChat
	model.direct = &protocol.DirectIdentity{UserID: "user-bob", Username: "bob"}
	model.directSessionID = "session-1"
	model.messages = append(model.messages, domain.Message{ID: "dm-1", Content: "ephemeral"})

	model.handleConnectionLoss("Connection lost")
	if model.screen != ScreenHome || model.direct != nil || model.directSessionID != "" || len(model.messages) != 0 || !strings.Contains(model.Status(), "Direct chat ended and cannot be reconnected") {
		t.Fatalf("connection loss did not discard direct state: screen:%v direct:%#v session:%q messages:%#v status:%q", model.screen, model.direct, model.directSessionID, model.messages, model.Status())
	}
}

func TestDirectInviteAcceptanceOpensEphemeralChatAndRendersCounterpart(t *testing.T) {
	t.Parallel()

	model := NewModel(nil)
	model.screen = ScreenHome
	model.session.User.ID = "user-alice"
	model.session.User.Username = "alice"
	model.width, model.height = 80, 24

	model.applyServerEvent(protocol.ServerEvent{Type: "direct_invite_received", InviteID: "invite-1", Counterpart: &protocol.DirectIdentity{UserID: "user-bob", Username: "bob"}})
	if !strings.Contains(ansi.Strip(model.renderNotificationTray(model.width)), "[INVITE] Direct invitation from bob") {
		t.Fatalf("direct invite banner = %q", ansi.Strip(model.renderNotificationTray(model.width)))
	}
	model.applyServerEvent(protocol.ServerEvent{Type: "direct_session_started", DirectSessionID: "session-1", Counterpart: &protocol.DirectIdentity{UserID: "user-bob", Username: "bob"}})
	model.applyServerEvent(protocol.ServerEvent{Type: "new_direct_message", DirectMessage: &protocol.DirectMessage{ID: "dm-1", UserID: "user-bob", Username: "bob", Content: "not saved", CreatedAt: time.Date(2026, 9, 2, 12, 0, 0, 0, time.Local)}})

	if model.screen != ScreenChat || model.directSessionID != "session-1" || len(model.messages) != 1 {
		t.Fatalf("direct chat state = screen:%v session:%q messages:%#v", model.screen, model.directSessionID, model.messages)
	}
	plain := ansi.Strip(model.View())
	for _, want := range []string{"@bob", "bob  not saved"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("direct chat view missing %q:\n%s", want, plain)
		}
	}

	model.applyServerEvent(protocol.ServerEvent{Type: "direct_session_ended", Reason: "participant_left"})
	if model.screen != ScreenHome || model.direct != nil || len(model.messages) != 0 {
		t.Fatalf("direct session was not discarded: screen:%v direct:%#v messages:%#v", model.screen, model.direct, model.messages)
	}
}
