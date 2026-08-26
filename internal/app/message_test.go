package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"termchat.local/termchat/internal/domain"
)

type fakeMessageRepository struct {
	messages []domain.Message
}

func (r *fakeMessageRepository) SaveMessage(_ context.Context, message domain.Message) error {
	r.messages = append(r.messages, message)
	return nil
}

func (r *fakeMessageRepository) RecentMessages(_ context.Context, roomID string, limit int) ([]domain.Message, error) {
	var found []domain.Message
	for _, message := range r.messages {
		if message.RoomID == roomID {
			found = append(found, message)
		}
	}
	if len(found) > limit {
		found = found[len(found)-limit:]
	}
	return found, nil
}

func TestMessageServicePersistsMessageWithRetention(t *testing.T) {
	t.Parallel()

	repo := &fakeMessageRepository{}
	service := NewMessageService(repo)
	now := time.Date(2026, time.August, 26, 15, 0, 0, 0, time.UTC)

	message, err := service.Send(context.Background(), "room-1", "user-1", "alice", "hello", now)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if len(repo.messages) != 1 || repo.messages[0].ID != message.ID {
		t.Fatalf("Send() did not persist message: %#v", repo.messages)
	}
	if message.Username != "alice" || !message.ExpiresAt.Equal(now.Add(7*24*time.Hour)) {
		t.Fatalf("Send() message = %#v", message)
	}
}

func TestMessageServiceLimitsUserToFiveMessagesPerTwoSeconds(t *testing.T) {
	t.Parallel()

	service := NewMessageService(&fakeMessageRepository{})
	now := time.Date(2026, time.August, 26, 15, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		if _, err := service.Send(context.Background(), "room-1", "user-1", "alice", "hello", now); err != nil {
			t.Fatalf("Send() message %d error = %v", i+1, err)
		}
	}
	if _, err := service.Send(context.Background(), "room-1", "user-1", "alice", "blocked", now); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("Send() sixth error = %v, want ErrRateLimited", err)
	}
	if _, err := service.Send(context.Background(), "room-1", "user-1", "alice", "allowed", now.Add(2*time.Second)); err != nil {
		t.Fatalf("Send() after window error = %v", err)
	}
}
