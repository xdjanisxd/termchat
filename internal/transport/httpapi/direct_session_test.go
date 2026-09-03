package httpapi

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"

	"termchat.local/termchat/internal/app"
	"termchat.local/termchat/internal/security"
)

func TestDirectSessionRequiresRecipientConsentAndNeverPersistsMessages(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	chat, tokens, messages := newDirectTestChat()
	router := chi.NewRouter()
	router.With(TokenMiddleware(tokens)).Get("/v1/ws", chat.ServeHTTP)
	server := httptest.NewServer(router)
	defer server.Close()
	websocketURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/ws"

	aliceToken, _ := tokens.Issue("alice-id", "alice", time.Now().UTC())
	bobToken, _ := tokens.Issue("bob-id", "bob", time.Now().UTC())
	alice := dialTestWebSocket(t, ctx, websocketURL, aliceToken)
	defer alice.Close(websocket.StatusNormalClosure, "test complete")
	bob := dialTestWebSocket(t, ctx, websocketURL, bobToken)
	defer bob.Close(websocket.StatusNormalClosure, "test complete")

	mustWriteDirect(t, ctx, alice, ClientEvent{Type: "direct_invite", RequestID: "invite-1", TargetUsername: "bob"})
	sent := readUntilEvent(t, ctx, alice, "direct_invite_sent")
	received := readUntilEvent(t, ctx, bob, "direct_invite_received")
	if sent.InviteID == "" || sent.InviteID != received.InviteID || received.Counterpart == nil || received.Counterpart.Username != "alice" {
		t.Fatalf("invite events = sent=%#v received=%#v", sent, received)
	}

	mustWriteDirect(t, ctx, bob, ClientEvent{Type: "direct_invite_accept", RequestID: "accept-1", InviteID: received.InviteID})
	aliceStarted := readUntilEvent(t, ctx, alice, "direct_session_started")
	bobStarted := readUntilEvent(t, ctx, bob, "direct_session_started")
	if aliceStarted.DirectSessionID == "" || aliceStarted.DirectSessionID != bobStarted.DirectSessionID || aliceStarted.Counterpart == nil || aliceStarted.Counterpart.Username != "bob" {
		t.Fatalf("direct starts = alice=%#v bob=%#v", aliceStarted, bobStarted)
	}

	mustWriteDirect(t, ctx, alice, ClientEvent{Type: "send_direct_message", RequestID: "dm-1", Content: "this stays in memory"})
	fromAlice := readUntilEvent(t, ctx, alice, "new_direct_message")
	toBob := readUntilEvent(t, ctx, bob, "new_direct_message")
	if fromAlice.DirectMessage == nil || toBob.DirectMessage == nil || fromAlice.DirectMessage.ID != toBob.DirectMessage.ID || toBob.DirectMessage.Content != "this stays in memory" {
		t.Fatalf("direct message delivery = alice=%#v bob=%#v", fromAlice, toBob)
	}
	messages.mu.Lock()
	persisted := len(messages.messages)
	messages.mu.Unlock()
	if persisted != 0 {
		t.Fatalf("direct message unexpectedly persisted: %d rows", persisted)
	}

	mustWriteDirect(t, ctx, bob, ClientEvent{Type: "leave_direct", RequestID: "leave-1"})
	aliceEnded := readUntilEvent(t, ctx, alice, "direct_session_ended")
	bobEnded := readUntilEvent(t, ctx, bob, "direct_session_ended")
	if aliceEnded.Reason != "participant_left" || bobEnded.Reason != "participant_left" {
		t.Fatalf("direct end reasons = alice=%q bob=%q", aliceEnded.Reason, bobEnded.Reason)
	}
}

func TestDirectSessionEndsWhenParticipantDisconnects(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	chat, tokens, _ := newDirectTestChat()
	router := chi.NewRouter()
	router.With(TokenMiddleware(tokens)).Get("/v1/ws", chat.ServeHTTP)
	server := httptest.NewServer(router)
	defer server.Close()
	websocketURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/ws"
	aliceToken, _ := tokens.Issue("alice-id", "alice", time.Now().UTC())
	bobToken, _ := tokens.Issue("bob-id", "bob", time.Now().UTC())
	alice := dialTestWebSocket(t, ctx, websocketURL, aliceToken)
	defer alice.Close(websocket.StatusNormalClosure, "test complete")
	bob := dialTestWebSocket(t, ctx, websocketURL, bobToken)

	mustWriteDirect(t, ctx, alice, ClientEvent{Type: "direct_invite", RequestID: "invite-1", TargetUsername: "bob"})
	invite := readUntilEvent(t, ctx, bob, "direct_invite_received")
	readUntilEvent(t, ctx, alice, "direct_invite_sent")
	mustWriteDirect(t, ctx, bob, ClientEvent{Type: "direct_invite_accept", RequestID: "accept-1", InviteID: invite.InviteID})
	readUntilEvent(t, ctx, alice, "direct_session_started")
	readUntilEvent(t, ctx, bob, "direct_session_started")

	if err := bob.Close(websocket.StatusNormalClosure, "disconnect for test"); err != nil {
		t.Fatalf("close participant: %v", err)
	}
	ended := readUntilEvent(t, ctx, alice, "direct_session_ended")
	if ended.Reason != "connection_lost" {
		t.Fatalf("direct end reason = %q, want connection_lost", ended.Reason)
	}
}

func TestDirectInviteReachesRecipientInRoomAndAcceptLeavesRoom(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	chat, tokens, _ := newDirectTestChat()
	router := chi.NewRouter()
	router.With(TokenMiddleware(tokens)).Get("/v1/ws", chat.ServeHTTP)
	server := httptest.NewServer(router)
	defer server.Close()
	websocketURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/ws"
	aliceToken, _ := tokens.Issue("alice-id", "alice", time.Now().UTC())
	bobToken, _ := tokens.Issue("bob-id", "bob", time.Now().UTC())
	alice := dialTestWebSocket(t, ctx, websocketURL, aliceToken)
	defer alice.Close(websocket.StatusNormalClosure, "test complete")
	bob := dialTestWebSocket(t, ctx, websocketURL, bobToken)
	defer bob.Close(websocket.StatusNormalClosure, "test complete")

	mustWriteDirect(t, ctx, alice, ClientEvent{Type: "create_room", RequestID: "create-1", RoomName: "private_room", Password: "long-enough-password"})
	readUntilEvent(t, ctx, alice, "room_joined")
	mustWriteDirect(t, ctx, bob, ClientEvent{Type: "direct_invite", RequestID: "invite-1", TargetUsername: "alice"})
	invite := readUntilEvent(t, ctx, alice, "direct_invite_received")
	if invite.InviteID == "" || invite.Counterpart == nil || invite.Counterpart.Username != "bob" {
		t.Fatalf("invite received while in room = %#v", invite)
	}
	readUntilEvent(t, ctx, bob, "direct_invite_sent")
	mustWriteDirect(t, ctx, alice, ClientEvent{Type: "direct_invite_accept", RequestID: "accept-1", InviteID: invite.InviteID})
	startedByAlice := readUntilEvent(t, ctx, alice, "direct_session_started")
	startedByBob := readUntilEvent(t, ctx, bob, "direct_session_started")
	if startedByAlice.DirectSessionID == "" || startedByAlice.DirectSessionID != startedByBob.DirectSessionID {
		t.Fatalf("direct session after room leave = alice=%#v bob=%#v", startedByAlice, startedByBob)
	}
	if client := chat.hub.users["alice-id"]; client == nil || client.room() != "" || client.direct() != startedByAlice.DirectSessionID {
		t.Fatalf("alice server state after accepting invite = %#v", client)
	}
}

func TestDirectInviteReachesRecipientInDirectChatAndAcceptReplacesSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	chat, tokens, _ := newDirectTestChat()
	router := chi.NewRouter()
	router.With(TokenMiddleware(tokens)).Get("/v1/ws", chat.ServeHTTP)
	server := httptest.NewServer(router)
	defer server.Close()
	websocketURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/ws"
	aliceToken, _ := tokens.Issue("alice-id", "alice", time.Now().UTC())
	bobToken, _ := tokens.Issue("bob-id", "bob", time.Now().UTC())
	malloryToken, _ := tokens.Issue("mallory-id", "mallory", time.Now().UTC())
	alice := dialTestWebSocket(t, ctx, websocketURL, aliceToken)
	defer alice.Close(websocket.StatusNormalClosure, "test complete")
	bob := dialTestWebSocket(t, ctx, websocketURL, bobToken)
	defer bob.Close(websocket.StatusNormalClosure, "test complete")
	mallory := dialTestWebSocket(t, ctx, websocketURL, malloryToken)
	defer mallory.Close(websocket.StatusNormalClosure, "test complete")

	mustWriteDirect(t, ctx, alice, ClientEvent{Type: "direct_invite", RequestID: "invite-ab", TargetUsername: "bob"})
	inviteAB := readUntilEvent(t, ctx, bob, "direct_invite_received")
	readUntilEvent(t, ctx, alice, "direct_invite_sent")
	mustWriteDirect(t, ctx, bob, ClientEvent{Type: "direct_invite_accept", RequestID: "accept-ab", InviteID: inviteAB.InviteID})
	readUntilEvent(t, ctx, alice, "direct_session_started")
	readUntilEvent(t, ctx, bob, "direct_session_started")

	mustWriteDirect(t, ctx, mallory, ClientEvent{Type: "direct_invite", RequestID: "invite-mb", TargetUsername: "bob"})
	inviteMB := readUntilEvent(t, ctx, bob, "direct_invite_received")
	readUntilEvent(t, ctx, mallory, "direct_invite_sent")
	mustWriteDirect(t, ctx, bob, ClientEvent{Type: "direct_invite_accept", RequestID: "accept-mb", InviteID: inviteMB.InviteID})
	ended := readUntilEvent(t, ctx, alice, "direct_session_ended")
	if ended.Reason != "participant_left" {
		t.Fatalf("replaced direct session end = %#v", ended)
	}
	readUntilEvent(t, ctx, bob, "direct_session_started")
	readUntilEvent(t, ctx, mallory, "direct_session_started")
	if client := chat.hub.users["bob-id"]; client == nil || client.direct() == "" {
		t.Fatal("bob did not transition into the new direct session")
	}
}

func TestDirectInviteRejectsSelfOfflineAndUnauthorizedAcceptance(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	chat, tokens, _ := newDirectTestChat()
	router := chi.NewRouter()
	router.With(TokenMiddleware(tokens)).Get("/v1/ws", chat.ServeHTTP)
	server := httptest.NewServer(router)
	defer server.Close()
	websocketURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/ws"
	aliceToken, _ := tokens.Issue("alice-id", "alice", time.Now().UTC())
	bobToken, _ := tokens.Issue("bob-id", "bob", time.Now().UTC())
	malloryToken, _ := tokens.Issue("mallory-id", "mallory", time.Now().UTC())
	alice := dialTestWebSocket(t, ctx, websocketURL, aliceToken)
	defer alice.Close(websocket.StatusNormalClosure, "test complete")
	bob := dialTestWebSocket(t, ctx, websocketURL, bobToken)
	defer bob.Close(websocket.StatusNormalClosure, "test complete")
	mallory := dialTestWebSocket(t, ctx, websocketURL, malloryToken)
	defer mallory.Close(websocket.StatusNormalClosure, "test complete")

	for _, target := range []string{"alice", "offline"} {
		mustWriteDirect(t, ctx, alice, ClientEvent{Type: "direct_invite", RequestID: "reject-" + target, TargetUsername: target})
		response := readUntilEvent(t, ctx, alice, "error")
		if response.Error == nil || response.Error.Code != "INVALID_DIRECT_TARGET" {
			t.Fatalf("target %q response = %#v", target, response)
		}
	}

	mustWriteDirect(t, ctx, alice, ClientEvent{Type: "direct_invite", RequestID: "invite-1", TargetUsername: "bob"})
	invite := readUntilEvent(t, ctx, bob, "direct_invite_received")
	readUntilEvent(t, ctx, alice, "direct_invite_sent")
	mustWriteDirect(t, ctx, mallory, ClientEvent{Type: "direct_invite_accept", RequestID: "steal-1", InviteID: invite.InviteID})
	response := readUntilEvent(t, ctx, mallory, "error")
	if response.Error == nil || response.Error.Code != "INVALID_DIRECT_INVITE" {
		t.Fatalf("unauthorized accept = %#v", response)
	}
}

func newDirectTestChat() (*ChatHandler, security.TokenManager, *wsMessageRepository) {
	hasher := security.NewPasswordHasher(security.Argon2Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})
	messages := &wsMessageRepository{}
	return NewChatHandler(app.NewRoomService(newWSRoomRepository(), hasher), app.NewMessageService(messages)), security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour), messages
}

func mustWriteDirect(t *testing.T, ctx context.Context, conn *websocket.Conn, event ClientEvent) {
	t.Helper()
	if err := wsjson.Write(ctx, conn, event); err != nil {
		t.Fatalf("write %s: %v", event.Type, err)
	}
}
