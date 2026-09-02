package protocol

import (
	"time"

	"termchat.local/termchat/internal/domain"
)

type ClientEvent struct {
	Type            string `json:"type"`
	RequestID       string `json:"request_id,omitempty"`
	RoomID          string `json:"room_id,omitempty"`
	RoomName        string `json:"room_name,omitempty"`
	Password        string `json:"password,omitempty"`
	NewPassword     string `json:"new_password,omitempty"`
	BeforeMessageID string `json:"before_message_id,omitempty"`
	TargetUsername  string `json:"target_username,omitempty"`
	InviteID        string `json:"invite_id,omitempty"`

	Content string `json:"content,omitempty"`
}

type EventError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// DirectIdentity contains only the public identity needed to label an ephemeral direct chat.
type DirectIdentity struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

// DirectMessage is deliberately separate from domain.Message: it has no room or retention fields
// and is only ever sent on an active WebSocket direct session.
type DirectMessage struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type ServerEvent struct {
	Type      string             `json:"type"`
	RequestID string             `json:"request_id,omitempty"`
	Room      *domain.PublicRoom `json:"room,omitempty"`
	Messages  []domain.Message   `json:"messages,omitempty"`
	HasMore   bool               `json:"has_more,omitempty"`

	Message  *domain.Message `json:"message,omitempty"`
	Username string          `json:"username,omitempty"`
	Users    []string        `json:"users,omitempty"`
	Error    *EventError     `json:"error,omitempty"`

	InviteID        string          `json:"invite_id,omitempty"`
	ExpiresAt       *time.Time      `json:"expires_at,omitempty"`
	Counterpart     *DirectIdentity `json:"counterpart,omitempty"`
	DirectSessionID string          `json:"direct_session_id,omitempty"`
	DirectMessage   *DirectMessage  `json:"direct_message,omitempty"`
	Reason          string          `json:"reason,omitempty"`
}
