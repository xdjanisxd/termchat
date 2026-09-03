package httpapi

import (
	"context"
	"fmt"
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

func (r *wsRoomRepository) RoomByID(_ context.Context, id string) (domain.Room, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	room, exists := r.byID[id]
	if !exists {
		return domain.Room{}, store.ErrNotFound
	}
	return room, nil
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

func (r *wsMessageRepository) MessagesBefore(_ context.Context, roomID, beforeMessageID string, limit int) ([]domain.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	messages := make([]domain.Message, 0, limit)
	for _, message := range r.messages {
		if message.RoomID == roomID {
			messages = append(messages, message)
		}
	}
	end := len(messages)
	if beforeMessageID != "" {
		end = 0
		for index, message := range messages {
			if message.ID == beforeMessageID {
				end = index
				break
			}
		}
	}
	start := end - limit
	if start < 0 {
		start = 0
	}
	return append([]domain.Message(nil), messages[start:end]...), nil
}

func TestChatHandlerLoadsOlderMessageHistoryForTheCurrentRoom(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hasher := security.NewPasswordHasher(security.Argon2Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})
	messageRepository := &wsMessageRepository{}
	chat := NewChatHandler(app.NewRoomService(newWSRoomRepository(), hasher), app.NewMessageService(messageRepository))
	tokens := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	router := chi.NewRouter()
	router.With(TokenMiddleware(tokens)).Get("/v1/ws", chat.ServeHTTP)
	server := httptest.NewServer(router)
	defer server.Close()
	token, _ := tokens.Issue("owner-1", "alice", time.Now().UTC())
	conn := dialTestWebSocket(t, ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/ws", token)
	defer conn.Close(websocket.StatusNormalClosure, "test complete")

	if err := wsjson.Write(ctx, conn, ClientEvent{Type: "create_room", RequestID: "create-1", RoomName: "private_room", Password: "roompass"}); err != nil {
		t.Fatalf("create room write: %v", err)
	}
	joined := readUntilEvent(t, ctx, conn, "room_joined")
	for index := 1; index <= 101; index++ {
		messageRepository.messages = append(messageRepository.messages, domain.Message{ID: fmt.Sprintf("message-%03d", index), RoomID: joined.Room.ID, UserID: "owner-1", Username: "alice", Content: fmt.Sprintf("message %d", index), CreatedAt: time.Unix(int64(index), 0), ExpiresAt: time.Now().Add(time.Hour)})
	}

	if err := wsjson.Write(ctx, conn, ClientEvent{Type: "load_history", RequestID: "history-1", BeforeMessageID: "message-101"}); err != nil {
		t.Fatalf("load history write: %v", err)
	}
	page := readUntilEvent(t, ctx, conn, "message_history")
	if page.RequestID != "history-1" || !page.HasMore || len(page.Messages) != 50 || page.Messages[0].ID != "message-051" || page.Messages[49].ID != "message-100" {
		t.Fatalf("message_history = %#v", page)
	}
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

func TestChatHandlerBlocksAllEventsAfterFourFailedRoomPasswords(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hasher := security.NewPasswordHasher(security.Argon2Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})
	roomService := app.NewRoomService(newWSRoomRepository(), hasher)
	messageService := app.NewMessageService(&wsMessageRepository{})
	attempts := app.NewAttemptLimiter(4, 5*time.Minute)
	tokens := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	chat := NewChatHandler(roomService, messageService, NewAttemptGuard(attempts, false))
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
	readUntilEvent(t, ctx, owner, "room_joined")

	for attempt := 1; attempt <= 4; attempt++ {
		requestID := fmt.Sprintf("join-%d", attempt)
		if err := wsjson.Write(ctx, member, ClientEvent{Type: "join_room", RequestID: requestID, RoomName: "private_room", Password: "wrongpass"}); err != nil {
			t.Fatalf("member failed join %d write: %v", attempt, err)
		}
		response := readUntilEvent(t, ctx, member, "error")
		if response.Error == nil || response.Error.Code != "INVALID_ROOM_CREDENTIALS" {
			t.Fatalf("member failed join %d response = %#v", attempt, response)
		}
	}

	if err := wsjson.Write(ctx, member, ClientEvent{Type: "ping", RequestID: "ping-blocked"}); err != nil {
		t.Fatalf("member ping write: %v", err)
	}
	response := readUntilEvent(t, ctx, member, "error")
	if response.RequestID != "ping-blocked" || response.Error == nil || response.Error.Code != "RATE_LIMITED" {
		t.Fatalf("blocked ping response = %#v", response)
	}
}

func TestChatHandlerBlocksExistingConnectionAfterFourFailedLoginsFromSameIP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hasher := security.NewPasswordHasher(security.Argon2Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})
	tokens := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	attempts := app.NewAttemptLimiter(4, 5*time.Minute)
	guard := NewAttemptGuard(attempts, false)
	authService := app.NewAuthService(&testUserRepository{users: make(map[string]domain.User)}, hasher, tokens)
	chat := NewChatHandler(app.NewRoomService(newWSRoomRepository(), hasher), app.NewMessageService(&wsMessageRepository{}), guard)
	router := NewRouter(NewAuthHandler(authService, guard).Routes(), TokenMiddleware(tokens), chat, func(context.Context) error { return nil }, guard)
	server := httptest.NewServer(router)
	defer server.Close()

	token, err := tokens.Issue("user-1", "alice", time.Now().UTC())
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	conn := dialTestWebSocket(t, ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/ws", token)
	defer conn.Close(websocket.StatusNormalClosure, "test complete")

	for attempt := 1; attempt <= 4; attempt++ {
		response, err := http.Post(server.URL+"/v1/auth/login", "application/json", strings.NewReader(`{"username":"alice","password":"wrong-password"}`))
		if err != nil {
			t.Fatalf("failed login %d: %v", attempt, err)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("failed login %d status = %d, want %d", attempt, response.StatusCode, http.StatusUnauthorized)
		}
	}

	if err := wsjson.Write(ctx, conn, ClientEvent{Type: "ping", RequestID: "ping-blocked-by-ip"}); err != nil {
		t.Fatalf("ping write: %v", err)
	}
	response := readUntilEvent(t, ctx, conn, "error")
	if response.RequestID != "ping-blocked-by-ip" || response.Error == nil || response.Error.Code != "RATE_LIMITED" {
		t.Fatalf("blocked ping response = %#v", response)
	}
}

func TestChatHandlerSuccessfulRoomJoinResetsFailures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hasher := security.NewPasswordHasher(security.Argon2Params{Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})
	roomService := app.NewRoomService(newWSRoomRepository(), hasher)
	messageService := app.NewMessageService(&wsMessageRepository{})
	attempts := app.NewAttemptLimiter(4, 5*time.Minute)
	tokens := security.NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	chat := NewChatHandler(roomService, messageService, NewAttemptGuard(attempts, false))
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
	readUntilEvent(t, ctx, owner, "room_joined")
	for attempt := 1; attempt <= 3; attempt++ {
		if err := wsjson.Write(ctx, member, ClientEvent{Type: "join_room", RequestID: fmt.Sprintf("wrong-before-%d", attempt), RoomName: "private_room", Password: "wrongpass"}); err != nil {
			t.Fatalf("member pre-reset join %d write: %v", attempt, err)
		}
		readUntilEvent(t, ctx, member, "error")
	}
	if err := wsjson.Write(ctx, member, ClientEvent{Type: "join_room", RequestID: "join-valid", RoomName: "private_room", Password: "roompass"}); err != nil {
		t.Fatalf("member valid join write: %v", err)
	}
	readUntilEvent(t, ctx, member, "room_joined")
	if err := wsjson.Write(ctx, member, ClientEvent{Type: "leave_room", RequestID: "leave-1"}); err != nil {
		t.Fatalf("member leave write: %v", err)
	}
	readUntilEvent(t, ctx, member, "room_left")
	if err := wsjson.Write(ctx, member, ClientEvent{Type: "join_room", RequestID: "wrong-after", RoomName: "private_room", Password: "wrongpass"}); err != nil {
		t.Fatalf("member post-reset join write: %v", err)
	}
	readUntilEvent(t, ctx, member, "error")
	if err := wsjson.Write(ctx, member, ClientEvent{Type: "ping", RequestID: "ping-allowed"}); err != nil {
		t.Fatalf("member ping write: %v", err)
	}
	response := readUntilEvent(t, ctx, member, "pong")
	if response.RequestID != "ping-allowed" {
		t.Fatalf("pong response = %#v", response)
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
