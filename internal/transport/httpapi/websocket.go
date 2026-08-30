package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"termchat.local/termchat/internal/app"
	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/store"
)

type ChatHandler struct {
	rooms    *app.RoomService
	messages *app.MessageService
	hub      *chatHub
	attempts *AttemptGuard
}

func NewChatHandler(rooms *app.RoomService, messages *app.MessageService, attempts ...*AttemptGuard) *ChatHandler {
	handler := &ChatHandler{rooms: rooms, messages: messages, hub: newChatHub()}
	if len(attempts) > 0 {
		handler.attempts = attempts[0]
	}
	return handler
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
	conn.SetReadLimit(16 * 1024)
	client := &chatClient{conn: conn, identity: identity}
	if h.attempts != nil {
		client.clientIP = h.attempts.clientIP(r)
	}
	defer func() {
		h.hub.leave(client)
		_ = conn.Close(websocket.StatusNormalClosure, "connection closed")
	}()

	for {
		var event ClientEvent
		if err := wsjson.Read(r.Context(), conn, &event); err != nil {
			return
		}
		h.handle(client, event)
	}
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
		client.write(ServerEvent{Type: "room_joined", RequestID: event.RequestID, Room: &room, Messages: history})
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
		client.write(ServerEvent{Type: "room_joined", RequestID: event.RequestID, Room: &room, Messages: history})
		h.hub.join(client, room.ID)
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
