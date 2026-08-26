package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"

	"termchat.local/termchat/internal/app"
	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/security"
	"termchat.local/termchat/internal/store"
)

type wsRoomRepository struct {
	mu     sync.Mutex
	byID   map[string]domain.Room
	byName map[string]string
}

func newWSRoomRepository() *wsRoomRepository {
	return &wsRoomRepository{byID: make(map[string]domain.Room), byName: make(map[string]string)}
}

func (r *wsRoomRepository) CreateRoom(_ context.Context, room domain.Room) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byName[room.Name]; exists {
		return store.ErrConflict
	}
	r.byID[room.ID] = room
	r.byName[room.Name] = room.ID
	return nil
}

func (r *wsRoomRepository) RoomByName(_ context.Context, name string) (domain.Room, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	id, exists := r.byName[name]
	if !exists {
		return domain.Room{}, store.ErrNotFound
	}
	return r.byID[id], nil
}

func (r *wsRoomRepository) UpdateRoomPassword(_ context.Context, roomID, ownerID, hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	room, exists := r.byID[roomID]
	if !exists {
		return store.ErrNotFound
	}
	if room.CreatedBy != ownerID {
		return store.ErrForbidden
	}
	room.PasswordHash = hash
	r.byID[roomID] = room
	return nil
}

func (r *wsRoomRepository) DeleteRoom(_ context.Context, roomID, ownerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	room, exists := r.byID[roomID]
	if !exists {
		return store.ErrNotFound
	}
	if room.CreatedBy != ownerID {
		return store.ErrForbidden
	}
	delete(r.byID, roomID)
	delete(r.byName, room.Name)
	return nil
}

type wsMessageRepository struct {
	mu       sync.Mutex
	messages []domain.Message
}

func (r *wsMessageRepository) SaveMessage(_ context.Context, message domain.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messages = append(r.messages, message)
	return nil
}

func (r *wsMessageRepository) RecentMessages(_ context.Context, roomID string, limit int) ([]domain.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	messages := make([]domain.Message, 0, limit)
	for _, message := range r.messages {
		if message.RoomID == roomID {
			messages = append(messages, message)
		}
	}
	return messages, nil
}

func TestChatHandlerCreatesJoinsAndBroadcasts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hasher := security.NewPasswordHasher(security.Argon2Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})
	roomRepository := newWSRoomRepository()
	messageRepository := &wsMessageRepository{}
	roomService := app.NewRoomService(roomRepository, hasher)
	messageService := app.NewMessageService(messageRepository)
	tokens := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	chat := NewChatHandler(roomService, messageService)
	router := chi.NewRouter()
	router.With(TokenMiddleware(tokens)).Get("/v1/ws", chat.ServeHTTP)
	server := httptest.NewServer(router)
	defer server.Close()
	websocketURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/ws"

	ownerToken, _ := tokens.Issue("owner-1", "alice", time.Now().UTC())
	memberToken, _ := tokens.Issue("user-2", "bob", time.Now().UTC())
	owner := dialTestWebSocket(t, ctx, websocketURL, ownerToken)
	defer owner.Close(websocket.StatusNormalClosure, "test complete")
	member := dialTestWebSocket(t, ctx, websocketURL, memberToken)
	defer member.Close(websocket.StatusNormalClosure, "test complete")

	if err := wsjson.Write(ctx, owner, ClientEvent{Type: "create_room", RequestID: "create-1", RoomName: "private_room", Password: "roompass"}); err != nil {
		t.Fatalf("owner create write: %v", err)
	}
	created := readUntilEvent(t, ctx, owner, "room_joined")
	if created.Room == nil || !created.Room.IsOwner || len(created.Messages) != 0 {
		t.Fatalf("room_joined event = %#v", created)
	}

	if err := wsjson.Write(ctx, member, ClientEvent{Type: "join_room", RequestID: "join-1", RoomName: "private_room", Password: "roompass"}); err != nil {
		t.Fatalf("member join write: %v", err)
	}
	joined := readUntilEvent(t, ctx, member, "room_joined")
	if joined.Room == nil || joined.Room.IsOwner || joined.Room.ID != created.Room.ID {
		t.Fatalf("member room_joined event = %#v", joined)
	}

	if err := wsjson.Write(ctx, member, ClientEvent{Type: "send_message", RequestID: "send-1", Content: "hello"}); err != nil {
		t.Fatalf("member send write: %v", err)
	}
	ownerMessage := readUntilEvent(t, ctx, owner, "new_message")
	memberMessage := readUntilEvent(t, ctx, member, "new_message")
	if ownerMessage.Message == nil || memberMessage.Message == nil || ownerMessage.Message.ID != memberMessage.Message.ID || ownerMessage.Message.Content != "hello" {
		t.Fatalf("broadcast mismatch: owner=%#v member=%#v", ownerMessage, memberMessage)
	}
	messageRepository.mu.Lock()
	defer messageRepository.mu.Unlock()
	if len(messageRepository.messages) != 1 {
		t.Fatalf("persisted messages = %d, want 1", len(messageRepository.messages))
	}
}

func dialTestWebSocket(t *testing.T, ctx context.Context, url, token string) *websocket.Conn {
	t.Helper()
	conn, response, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: http.Header{"Authorization": []string{"Bearer " + token}}})
	if err != nil {
		if response != nil {
			t.Fatalf("websocket.Dial() status = %d, error = %v", response.StatusCode, err)
		}
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	return conn
}

func readUntilEvent(t *testing.T, ctx context.Context, conn *websocket.Conn, eventType string) ServerEvent {
	t.Helper()
	for {
		var event ServerEvent
		if err := wsjson.Read(ctx, conn, &event); err != nil {
			t.Fatalf("read websocket event %q: %v", eventType, err)
		}
		if event.Type == eventType {
			return event
		}
	}
}
