package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"termchat.local/termchat/internal/domain"
)

const (
	messageRateWindow = 2 * time.Second
	messageRateLimit  = 5
)

var ErrRateLimited = errors.New("message rate limit exceeded")

type MessageRepository interface {
	SaveMessage(ctx context.Context, message domain.Message) error
	RecentMessages(ctx context.Context, roomID string, limit int) ([]domain.Message, error)
}

type MessageService struct {
	messages MessageRepository
	mu       sync.Mutex
	sentAt   map[string][]time.Time
}

func NewMessageService(messages MessageRepository) *MessageService {
	return &MessageService{messages: messages, sentAt: make(map[string][]time.Time)}
}

func (s *MessageService) Send(ctx context.Context, roomID, userID, username, content string, now time.Time) (domain.Message, error) {
	message, err := domain.NewMessage(roomID, userID, content, now)
	if err != nil {
		return domain.Message{}, err
	}
	if !s.allow(userID, now) {
		return domain.Message{}, ErrRateLimited
	}
	message.Username = username
	if err := s.messages.SaveMessage(ctx, message); err != nil {
		return domain.Message{}, fmt.Errorf("save message: %w", err)
	}
	return message, nil
}

func (s *MessageService) History(ctx context.Context, roomID string) ([]domain.Message, error) {
	return s.messages.RecentMessages(ctx, roomID, 50)
}

func (s *MessageService) allow(userID string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := now.Add(-messageRateWindow)
	previous := s.sentAt[userID]
	kept := previous[:0]
	for _, sent := range previous {
		if sent.After(cutoff) {
			kept = append(kept, sent)
		}
	}
	if len(kept) >= messageRateLimit {
		s.sentAt[userID] = kept
		return false
	}
	s.sentAt[userID] = append(kept, now)
	return true
}
