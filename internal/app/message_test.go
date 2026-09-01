package app

import (
	"context"
	"errors"
	"fmt"
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

func (r *fakeMessageRepository) MessagesBefore(_ context.Context, roomID, beforeMessageID string, limit int) ([]domain.Message, error) {
	var found []domain.Message
	for _, message := range r.messages {
		if message.RoomID == roomID {
			found = append(found, message)
		}
	}
	end := len(found)
	if beforeMessageID != "" {
		end = 0
		for index, message := range found {
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
	return append([]domain.Message(nil), found[start:end]...), nil
}

func TestMessageServiceHistoryBeforeReturnsAnOrderedPageAndHasMore(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 26, 15, 0, 0, 0, time.UTC)
	repository := &fakeMessageRepository{}
	for i := 1; i <= 101; i++ {
		repository.messages = append(repository.messages, domain.Message{
			ID: fmt.Sprintf("message-%03d", i), RoomID: "room-1", UserID: "user-1", Username: "alice",
			Content: fmt.Sprintf("message %d", i), CreatedAt: now.Add(time.Duration(i) * time.Minute), ExpiresAt: now.Add(domain.MessageRetention),
		})
	}
	service := NewMessageService(repository)

	page, err := service.HistoryBefore(context.Background(), "room-1", "message-101")
	if err != nil {
		t.Fatalf("HistoryBefore() error = %v", err)
	}
	if !page.HasMore || len(page.Messages) != 50 {
		t.Fatalf("HistoryBefore() page = %#v", page)
	}
	if page.Messages[0].ID != "message-051" || page.Messages[len(page.Messages)-1].ID != "message-100" {
		t.Fatalf("HistoryBefore() messages = %#v", page.Messages)
	}
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
