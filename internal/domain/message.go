package domain

import (
	"errors"
	"time"
	"unicode"

	"github.com/google/uuid"
)

const (
	MaxMessageLength = 2000
	MessageRetention = 7 * 24 * time.Hour
)

var ErrInvalidMessage = errors.New("message must contain 1-2000 characters")

type Message struct {
	ID        string    `json:"id"`
	RoomID    string    `json:"room_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username,omitempty"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"-"`
}

func NewMessage(roomID, userID, content string, now time.Time) (Message, error) {
	runes := []rune(content)
	if len(runes) < 1 || len(runes) > MaxMessageLength {
		return Message{}, ErrInvalidMessage
	}
	for _, r := range runes {
		if unicode.IsControl(r) {
			return Message{}, ErrInvalidMessage
		}
	}
	return Message{
		ID:        uuid.NewString(),
		RoomID:    roomID,
		UserID:    userID,
		Content:   content,
		CreatedAt: now,
		ExpiresAt: now.Add(MessageRetention),
	}, nil
}
