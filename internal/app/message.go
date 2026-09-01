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
	historyPageSize   = 50
)

var ErrRateLimited = errors.New("message rate limit exceeded")

type MessageRepository interface {
	SaveMessage(ctx context.Context, message domain.Message) error
	MessagesBefore(ctx context.Context, roomID, beforeMessageID string, limit int) ([]domain.Message, error)
}

type MessagePage struct {
	Messages []domain.Message
	HasMore  bool
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

func (s *MessageService) History(ctx context.Context, roomID string) (MessagePage, error) {
	return s.HistoryBefore(ctx, roomID, "")
}

func (s *MessageService) HistoryBefore(ctx context.Context, roomID, beforeMessageID string) (MessagePage, error) {
	messages, err := s.messages.MessagesBefore(ctx, roomID, beforeMessageID, historyPageSize+1)
	if err != nil {
		return MessagePage{}, err
	}
	page := MessagePage{Messages: messages}
	if len(page.Messages) > historyPageSize {
		page.HasMore = true
		page.Messages = page.Messages[1:]
	}
	return page, nil
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
