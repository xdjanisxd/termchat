package httpapi

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/google/uuid"
)

const directInviteTTL = 60 * time.Second

var (
	errInvalidDirectTarget = errors.New("invalid direct target")
	errInvalidDirectInvite = errors.New("invalid direct invite")
	errDirectContextBusy   = errors.New("direct chat context is busy")
	errNotInDirectSession  = errors.New("not in a direct session")
)

type chatClient struct {
	conn     *websocket.Conn
	identity Identity
	clientIP string
	writeMu  sync.Mutex
	stateMu  sync.RWMutex
	roomID   string
	directID string
}

func (c *chatClient) write(event ServerEvent) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return wsjson.Write(ctx, c.conn, event)
}

func (c *chatClient) room() string   { c.stateMu.RLock(); defer c.stateMu.RUnlock(); return c.roomID }
func (c *chatClient) direct() string { c.stateMu.RLock(); defer c.stateMu.RUnlock(); return c.directID }
func (c *chatClient) setRoom(roomID string) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.roomID = roomID
}
func (c *chatClient) setDirect(sessionID string) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.directID = sessionID
}

type directInvite struct {
	id, senderID, recipientID string
	expiresAt                 time.Time
}

type directSession struct {
	id                string
	firstID, secondID string
}

type chatHub struct {
	mu       sync.RWMutex
	rooms    map[string]map[*chatClient]struct{}
	users    map[string]*chatClient
	byName   map[string]*chatClient
	invites  map[string]directInvite
	sessions map[string]directSession
}

func newChatHub() *chatHub {
	return &chatHub{
		rooms: make(map[string]map[*chatClient]struct{}), users: make(map[string]*chatClient), byName: make(map[string]*chatClient),
		invites: make(map[string]directInvite), sessions: make(map[string]directSession),
	}
}

func (h *chatHub) register(client *chatClient) {
	h.mu.RLock()
	previous := h.users[client.identity.UserID]
	h.mu.RUnlock()
	if previous != nil && previous != client {
		h.unregister(previous)
	}
	h.mu.Lock()
	h.users[client.identity.UserID] = client
	h.byName[client.identity.Username] = client
	h.mu.Unlock()
}

func (h *chatHub) unregister(client *chatClient) {
	h.cancelInvitesFor(client, "direct_invite_cancelled")
	h.endDirect(client, "connection_lost")
	h.leave(client)
	h.mu.Lock()
	if h.users[client.identity.UserID] == client {
		delete(h.users, client.identity.UserID)
	}
	if h.byName[client.identity.Username] == client {
		delete(h.byName, client.identity.Username)
	}
	h.mu.Unlock()
}

func (h *chatHub) disconnectUser(userID string) {
	h.mu.RLock()
	client := h.users[userID]
	h.mu.RUnlock()
	if client == nil {
		return
	}
	h.unregister(client)
	_ = client.write(ServerEvent{Type: "account_deleted"})
	_ = client.conn.Close(websocket.StatusNormalClosure, "account deleted")
}

func (h *chatHub) join(client *chatClient, roomID string) {
	h.cancelInvitesFor(client, "direct_invite_cancelled")
	h.endDirect(client, "participant_left")
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

func (h *chatHub) inviteDirect(sender *chatClient, targetUsername string, now time.Time) (directInvite, *chatClient, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	target := h.byName[targetUsername]
	if target == nil || target == sender {
		return directInvite{}, nil, errInvalidDirectTarget
	}
	if h.hasInviteLocked(sender.identity.UserID) || h.hasInviteLocked(target.identity.UserID) {
		return directInvite{}, nil, errDirectContextBusy
	}
	invite := directInvite{id: uuid.NewString(), senderID: sender.identity.UserID, recipientID: target.identity.UserID, expiresAt: now.Add(directInviteTTL)}
	h.invites[invite.id] = invite
	go func(id string, expiry time.Time) {
		timer := time.NewTimer(time.Until(expiry))
		defer timer.Stop()
		<-timer.C
		h.expireInvite(id, expiry)
	}(invite.id, invite.expiresAt)
	return invite, target, nil
}

func (h *chatHub) acceptDirect(recipient *chatClient, inviteID string, now time.Time) (directSession, *chatClient, error) {
	h.mu.Lock()
	invite, ok := h.invites[inviteID]
	if !ok || invite.recipientID != recipient.identity.UserID || !now.Before(invite.expiresAt) {
		if ok {
			delete(h.invites, inviteID)
		}
		h.mu.Unlock()
		return directSession{}, nil, errInvalidDirectInvite
	}
	sender := h.users[invite.senderID]
	if sender == nil {
		delete(h.invites, inviteID)
		h.mu.Unlock()
		return directSession{}, nil, errInvalidDirectInvite
	}
	delete(h.invites, inviteID)

	// Consent starts a new exclusive direct session. Both consenting people leave
	// any current room or direct session atomically before the new session is live.
	endedParticipants := make(map[*chatClient]struct{})
	roomLeaves := make(map[string][]*chatClient)
	for _, participant := range []*chatClient{sender, recipient} {
		if session, exists := h.sessions[participant.direct()]; exists {
			delete(h.sessions, session.id)
			for _, userID := range []string{session.firstID, session.secondID} {
				if peer := h.users[userID]; peer != nil {
					peer.setDirect("")
					endedParticipants[peer] = struct{}{}
				}
			}
		}
		if roomID := participant.room(); roomID != "" {
			if clients := h.rooms[roomID]; clients != nil {
				delete(clients, participant)
				if len(clients) == 0 {
					delete(h.rooms, roomID)
				}
			}
			participant.setRoom("")
			roomLeaves[roomID] = append(roomLeaves[roomID], participant)
		}
	}

	session := directSession{id: uuid.NewString(), firstID: sender.identity.UserID, secondID: recipient.identity.UserID}
	h.sessions[session.id] = session
	sender.setDirect(session.id)
	recipient.setDirect(session.id)
	h.mu.Unlock()

	for participant := range endedParticipants {
		_ = participant.write(ServerEvent{Type: "direct_session_ended", Reason: "participant_left"})
	}
	for roomID, participants := range roomLeaves {
		for _, participant := range participants {
			h.broadcast(roomID, ServerEvent{Type: "user_left", Username: participant.identity.Username})
		}
	}
	return session, sender, nil
}

func (h *chatHub) declineDirect(recipient *chatClient, inviteID string) (*chatClient, error) {
	h.mu.Lock()
	invite, ok := h.invites[inviteID]
	if !ok || invite.recipientID != recipient.identity.UserID {
		h.mu.Unlock()
		return nil, errInvalidDirectInvite
	}
	delete(h.invites, inviteID)
	sender := h.users[invite.senderID]
	h.mu.Unlock()
	return sender, nil
}

func (h *chatHub) directPeer(client *chatClient) (*chatClient, error) {
	h.mu.RLock()
	session, ok := h.sessions[client.direct()]
	if !ok {
		h.mu.RUnlock()
		return nil, errNotInDirectSession
	}
	peerID := session.firstID
	if peerID == client.identity.UserID {
		peerID = session.secondID
	}
	peer := h.users[peerID]
	h.mu.RUnlock()
	if peer == nil {
		return nil, errNotInDirectSession
	}
	return peer, nil
}

func (h *chatHub) endDirect(client *chatClient, reason string) {
	h.mu.Lock()
	session, ok := h.sessions[client.direct()]
	if !ok {
		h.mu.Unlock()
		return
	}
	delete(h.sessions, session.id)
	first, second := h.users[session.firstID], h.users[session.secondID]
	if first != nil {
		first.setDirect("")
	}
	if second != nil {
		second.setDirect("")
	}
	h.mu.Unlock()
	for _, participant := range []*chatClient{first, second} {
		if participant != nil {
			_ = participant.write(ServerEvent{Type: "direct_session_ended", Reason: reason})
		}
	}
}

func (h *chatHub) cancelInvitesFor(client *chatClient, eventType string) {
	h.mu.Lock()
	notifications := make([]*chatClient, 0)
	for id, invite := range h.invites {
		if invite.senderID != client.identity.UserID && invite.recipientID != client.identity.UserID {
			continue
		}
		delete(h.invites, id)
		otherID := invite.senderID
		if otherID == client.identity.UserID {
			otherID = invite.recipientID
		}
		if other := h.users[otherID]; other != nil {
			notifications = append(notifications, other)
		}
	}
	h.mu.Unlock()
	for _, other := range notifications {
		_ = other.write(ServerEvent{Type: eventType})
	}
}

func (h *chatHub) expireInvite(inviteID string, expectedExpiry time.Time) {
	h.mu.Lock()
	invite, ok := h.invites[inviteID]
	if !ok || !invite.expiresAt.Equal(expectedExpiry) || time.Now().UTC().Before(invite.expiresAt) {
		h.mu.Unlock()
		return
	}
	delete(h.invites, inviteID)
	sender, recipient := h.users[invite.senderID], h.users[invite.recipientID]
	h.mu.Unlock()
	for _, client := range []*chatClient{sender, recipient} {
		if client != nil {
			_ = client.write(ServerEvent{Type: "direct_invite_expired", InviteID: inviteID})
		}
	}
}

func (h *chatHub) hasInviteLocked(userID string) bool {
	for _, invite := range h.invites {
		if invite.senderID == userID || invite.recipientID == userID {
			return true
		}
	}
	return false
}
