package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"

	"termchat.local/termchat/internal/app"
	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/protocol"
	"termchat.local/termchat/internal/store"
)

type ChatHandler struct {
	rooms    *app.RoomService
	messages *app.MessageService
	hub      *chatHub
	attempts *AttemptGuard
}

const maxWebSocketMessageBytes = 16 * 1024

func NewChatHandler(rooms *app.RoomService, messages *app.MessageService, attempts ...*AttemptGuard) *ChatHandler {
	handler := &ChatHandler{rooms: rooms, messages: messages, hub: newChatHub()}
	if len(attempts) > 0 {
		handler.attempts = attempts[0]
	}
	return handler
}

func (h *ChatHandler) DisconnectUser(userID string) {
	h.hub.disconnectUser(userID)
}

func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	identity, ok := IdentityFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication is required.")
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	conn.SetReadLimit(maxWebSocketMessageBytes)
	client := &chatClient{conn: conn, identity: identity}
	if h.attempts != nil {
		client.clientIP = h.attempts.clientIP(r)
	}
	h.hub.register(client)
	defer func() {
		h.hub.unregister(client)
		_ = conn.Close(websocket.StatusNormalClosure, "connection closed")
	}()

	for {
		event, err := readClientEvent(r.Context(), conn)
		if err != nil {
			return
		}
		h.handle(client, event)
	}
}

func readClientEvent(ctx context.Context, conn *websocket.Conn) (ClientEvent, error) {
	messageType, payload, err := conn.Read(ctx)
	if err != nil {
		return ClientEvent{}, err
	}
	return decodeWebSocketClientEvent(messageType, payload)
}

func decodeWebSocketClientEvent(messageType websocket.MessageType, payload []byte) (ClientEvent, error) {
	if messageType != websocket.MessageText {
		return ClientEvent{}, errors.New("client event must be a text message")
	}
	return decodeClientEventPayload(payload)
}

func decodeClientEventPayload(payload []byte) (ClientEvent, error) {
	var event ClientEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return ClientEvent{}, err
	}
	return event, nil
}

func (h *ChatHandler) handle(client *chatClient, event ClientEvent) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if h.attempts != nil {
		now := time.Now().UTC()
		if h.attempts.isIPBlocked(client.clientIP, now) || h.attempts.IsUserBlocked(client.identity.UserID, now) {
			client.write(namedError(event.RequestID, "RATE_LIMITED", attemptRateLimitMessage))
			return
		}
	}

	switch event.Type {
	case "create_room":
		room, err := h.rooms.Create(ctx, client.identity.UserID, event.RoomName, event.Password, time.Now().UTC())
		if err != nil {
			client.write(eventError(event.RequestID, err))
			return
		}
		history, err := h.messages.History(ctx, room.ID)
		if err != nil {
			client.write(eventError(event.RequestID, err))
			return
		}
		client.write(ServerEvent{Type: "room_joined", RequestID: event.RequestID, Room: &room, Messages: history.Messages, HasMore: history.HasMore})
		h.hub.join(client, room.ID)
	case "join_room":
		room, err := h.rooms.Join(ctx, client.identity.UserID, event.RoomName, event.Password)
		if err != nil {
			if h.attempts != nil && errors.Is(err, app.ErrInvalidRoomCredentials) {
				h.attempts.attempts.RecordFailure(userAttemptKey(client.identity.UserID), time.Now().UTC())
			}
			client.write(eventError(event.RequestID, err))
			return
		}
		if h.attempts != nil {
			h.attempts.attempts.Reset(userAttemptKey(client.identity.UserID))
		}
		history, err := h.messages.History(ctx, room.ID)
		if err != nil {
			client.write(eventError(event.RequestID, err))
			return
		}
		client.write(ServerEvent{Type: "room_joined", RequestID: event.RequestID, Room: &room, Messages: history.Messages, HasMore: history.HasMore})
		h.hub.join(client, room.ID)
	case "load_history":
		roomID := client.room()
		if roomID == "" {
			client.write(namedError(event.RequestID, "NOT_IN_ROOM", "Join a room before loading history."))
			return
		}
		page, err := h.messages.HistoryBefore(ctx, roomID, event.BeforeMessageID)
		if err != nil {
			client.write(eventError(event.RequestID, err))
			return
		}
		client.write(ServerEvent{Type: "message_history", RequestID: event.RequestID, Messages: page.Messages, HasMore: page.HasMore})
	case "leave_room":
		h.hub.leave(client)
		client.write(ServerEvent{Type: "room_left", RequestID: event.RequestID})
	case "send_message":
		roomID := client.room()
		if roomID == "" {
			client.write(namedError(event.RequestID, "NOT_IN_ROOM", "Join a room before sending messages."))
			return
		}
		message, err := h.messages.Send(ctx, roomID, client.identity.UserID, client.identity.Username, event.Content, time.Now().UTC())
		if err != nil {
			client.write(eventError(event.RequestID, err))
			return
		}
		h.hub.broadcast(roomID, ServerEvent{Type: "new_message", Message: &message})
	case "room_invite":
		roomID := client.room()
		if roomID == "" {
			client.write(namedError(event.RequestID, "NOT_IN_ROOM", "Join a room first."))
			return
		}
		room, err := h.rooms.InviteRoom(ctx, client.identity.UserID, roomID)
		if err != nil {
			client.write(eventError(event.RequestID, err))
			return
		}
		invite, target, err := h.hub.inviteRoom(client, event.TargetUsername, room, time.Now().UTC())
		if err != nil {
			client.write(namedError(event.RequestID, "ROOM_INVITE_UNAVAILABLE", "Room invitation could not be delivered."))
			return
		}
		expiresAt := invite.expiresAt
		client.write(ServerEvent{Type: "room_invite_sent", RequestID: event.RequestID, InviteID: invite.id, ExpiresAt: &expiresAt})
		target.write(ServerEvent{Type: "room_invite_received", InviteID: invite.id, ExpiresAt: &expiresAt, Room: &room, Counterpart: directIdentity(client)})
	case "room_invite_accept":
		room, _, err := h.hub.acceptRoomInvite(client, event.InviteID, time.Now().UTC())
		if err != nil {
			client.write(directError(event.RequestID, err))
			return
		}
		client.write(ServerEvent{Type: "room_joined", RequestID: event.RequestID, Room: &room})
	case "room_invite_decline":
		sender, err := h.hub.declineRoomInvite(client, event.InviteID)
		if err != nil {
			client.write(directError(event.RequestID, err))
			return
		}
		client.write(ServerEvent{Type: "room_invite_declined", RequestID: event.RequestID, InviteID: event.InviteID})
		if sender != nil {
			sender.write(ServerEvent{Type: "room_invite_declined", InviteID: event.InviteID})
		}
	case "direct_invite":
		invite, recipient, err := h.hub.inviteDirect(client, event.TargetUsername, time.Now().UTC())
		if err != nil {
			client.write(directInviteError(event.RequestID, err))
			return
		}
		expiresAt := invite.expiresAt
		client.write(ServerEvent{Type: "direct_invite_sent", RequestID: event.RequestID, InviteID: invite.id, ExpiresAt: &expiresAt, Counterpart: directIdentity(recipient)})
		recipient.write(ServerEvent{Type: "direct_invite_received", InviteID: invite.id, ExpiresAt: &expiresAt, Counterpart: directIdentity(client)})
	case "direct_invite_accept":
		session, sender, err := h.hub.acceptDirect(client, event.InviteID, time.Now().UTC())
		if err != nil {
			client.write(directError(event.RequestID, err))
			return
		}
		client.write(ServerEvent{Type: "direct_session_started", RequestID: event.RequestID, DirectSessionID: session.id, Counterpart: directIdentity(sender)})
		sender.write(ServerEvent{Type: "direct_session_started", DirectSessionID: session.id, Counterpart: directIdentity(client)})
	case "direct_invite_decline":
		sender, err := h.hub.declineDirect(client, event.InviteID)
		if err != nil {
			client.write(directError(event.RequestID, err))
			return
		}
		client.write(ServerEvent{Type: "direct_invite_declined", RequestID: event.RequestID, InviteID: event.InviteID})
		if sender != nil {
			sender.write(ServerEvent{Type: "direct_invite_declined", InviteID: event.InviteID, Counterpart: directIdentity(client)})
		}
	case "send_direct_message":
		peer, err := h.hub.directPeer(client)
		if err != nil {
			client.write(directError(event.RequestID, err))
			return
		}
		now := time.Now().UTC()
		if err := h.messages.ValidateAndAllow(client.identity.UserID, event.Content, now); err != nil {
			client.write(eventError(event.RequestID, err))
			return
		}
		message := protocol.DirectMessage{ID: uuid.NewString(), UserID: client.identity.UserID, Username: client.identity.Username, Content: event.Content, CreatedAt: now}
		client.write(ServerEvent{Type: "new_direct_message", DirectMessage: &message})
		peer.write(ServerEvent{Type: "new_direct_message", DirectMessage: &message})
	case "leave_direct":
		if client.direct() == "" {
			client.write(directError(event.RequestID, errNotInDirectSession))
			return
		}
		h.hub.endDirect(client, "participant_left")
	case "who":
		roomID := client.room()
		if roomID == "" {
			client.write(namedError(event.RequestID, "NOT_IN_ROOM", "Join a room first."))
			return
		}
		client.write(ServerEvent{Type: "user_list", RequestID: event.RequestID, Users: h.hub.usernames(roomID)})
	case "change_room_password":
		roomID := client.room()
		if roomID == "" {
			client.write(namedError(event.RequestID, "NOT_IN_ROOM", "Join a room first."))
			return
		}
		if err := h.rooms.ChangePassword(ctx, client.identity.UserID, roomID, event.NewPassword); err != nil {
			client.write(eventError(event.RequestID, err))
			return
		}
		client.write(ServerEvent{Type: "room_password_changed", RequestID: event.RequestID})
	case "delete_room":
		roomID := client.room()
		if roomID == "" {
			client.write(namedError(event.RequestID, "NOT_IN_ROOM", "Join a room first."))
			return
		}
		if err := h.rooms.Delete(ctx, client.identity.UserID, roomID); err != nil {
			client.write(eventError(event.RequestID, err))
			return
		}
		h.hub.deleteRoom(roomID)
	case "ping":
		client.write(ServerEvent{Type: "pong", RequestID: event.RequestID})
	default:
		client.write(namedError(event.RequestID, "UNKNOWN_EVENT", "Unknown event type."))
	}
}

func eventError(requestID string, err error) ServerEvent {
	switch {
	case errors.Is(err, store.ErrConflict):
		return namedError(requestID, "ROOM_EXISTS", "A room with this name already exists.")
	case errors.Is(err, store.ErrForbidden):
		return namedError(requestID, "FORBIDDEN", "Only the room owner can perform this action.")
	case errors.Is(err, app.ErrInvalidRoomCredentials):
		return namedError(requestID, "INVALID_ROOM_CREDENTIALS", "Invalid room name or password.")
	case errors.Is(err, app.ErrRateLimited):
		return namedError(requestID, "RATE_LIMITED", "You can send at most 5 messages every 2 seconds.")
	case errors.Is(err, domain.ErrInvalidRoomName), errors.Is(err, app.ErrInvalidRoomPassword), errors.Is(err, domain.ErrInvalidMessage):
		return namedError(requestID, "VALIDATION_ERROR", err.Error())
	default:
		return namedError(requestID, "INTERNAL_ERROR", "The server could not complete the request.")
	}
}

func namedError(requestID, code, message string) ServerEvent {
	return ServerEvent{Type: "error", RequestID: requestID, Error: &EventError{Code: code, Message: message}}
}

func directIdentity(client *chatClient) *protocol.DirectIdentity {
	return &protocol.DirectIdentity{UserID: client.identity.UserID, Username: client.identity.Username}
}

func directInviteError(requestID string, err error) ServerEvent {
	if errors.Is(err, errInvalidDirectTarget) || errors.Is(err, errDirectContextBusy) {
		return namedError(requestID, "DIRECT_UNAVAILABLE", "Direct invitation could not be delivered.")
	}
	return directError(requestID, err)
}

func directError(requestID string, err error) ServerEvent {
	switch {
	case errors.Is(err, errInvalidDirectTarget):
		return namedError(requestID, "INVALID_DIRECT_TARGET", "The target must be a different, currently connected username.")
	case errors.Is(err, errInvalidDirectInvite):
		return namedError(requestID, "INVALID_DIRECT_INVITE", "This direct invitation is no longer available to you.")
	case errors.Is(err, errDirectContextBusy):
		return namedError(requestID, "DIRECT_CONTEXT_BUSY", "One participant already has a pending direct invitation.")
	case errors.Is(err, errNotInDirectSession):
		return namedError(requestID, "NOT_IN_DIRECT_SESSION", "You are not in an active direct chat.")
	default:
		return namedError(requestID, "INTERNAL_ERROR", "The server could not complete the direct chat action.")
	}
}
