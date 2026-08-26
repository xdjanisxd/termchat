package httpapi

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

type chatClient struct {
	conn     *websocket.Conn
	identity Identity
	writeMu  sync.Mutex
	roomMu   sync.RWMutex
	roomID   string
}

func (c *chatClient) write(event ServerEvent) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return wsjson.Write(ctx, c.conn, event)
}

func (c *chatClient) room() string {
	c.roomMu.RLock()
	defer c.roomMu.RUnlock()
	return c.roomID
}

func (c *chatClient) setRoom(roomID string) {
	c.roomMu.Lock()
	defer c.roomMu.Unlock()
	c.roomID = roomID
}

type chatHub struct {
	mu    sync.RWMutex
	rooms map[string]map[*chatClient]struct{}
}

func newChatHub() *chatHub {
	return &chatHub{rooms: make(map[string]map[*chatClient]struct{})}
}

func (h *chatHub) join(client *chatClient, roomID string) {
	h.leave(client)
	h.mu.Lock()
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*chatClient]struct{})
	}
	h.rooms[roomID][client] = struct{}{}
	client.setRoom(roomID)
	h.mu.Unlock()
	h.broadcast(roomID, ServerEvent{Type: "user_joined", Username: client.identity.Username})
}

func (h *chatHub) leave(client *chatClient) {
	roomID := client.room()
	if roomID == "" {
		return
	}
	h.mu.Lock()
	if clients := h.rooms[roomID]; clients != nil {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.rooms, roomID)
		}
	}
	client.setRoom("")
	h.mu.Unlock()
	h.broadcast(roomID, ServerEvent{Type: "user_left", Username: client.identity.Username})
}

func (h *chatHub) broadcast(roomID string, event ServerEvent) {
	h.mu.RLock()
	clients := make([]*chatClient, 0, len(h.rooms[roomID]))
	for client := range h.rooms[roomID] {
		clients = append(clients, client)
	}
	h.mu.RUnlock()
	for _, client := range clients {
		_ = client.write(event)
	}
}

func (h *chatHub) usernames(roomID string) []string {
	h.mu.RLock()
	users := make([]string, 0, len(h.rooms[roomID]))
	for client := range h.rooms[roomID] {
		users = append(users, client.identity.Username)
	}
	h.mu.RUnlock()
	sort.Strings(users)
	return users
}

func (h *chatHub) deleteRoom(roomID string) {
	h.mu.Lock()
	clients := make([]*chatClient, 0, len(h.rooms[roomID]))
	for client := range h.rooms[roomID] {
		client.setRoom("")
		clients = append(clients, client)
	}
	delete(h.rooms, roomID)
	h.mu.Unlock()
	for _, client := range clients {
		_ = client.write(ServerEvent{Type: "room_deleted"})
	}
}
