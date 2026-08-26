package domain

import (
	"strings"
	"testing"
	"time"
)

func TestNewMessageExpiresAfterSevenDays(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 26, 15, 0, 0, 0, time.UTC)
	message, err := NewMessage("room-1", "user-1", "hello", now)
	if err != nil {
		t.Fatalf("NewMessage() error = %v", err)
	}
	if message.ID == "" {
		t.Fatal("NewMessage() returned an empty ID")
	}
	if got, want := message.ExpiresAt, now.Add(7*24*time.Hour); !got.Equal(want) {
		t.Fatalf("ExpiresAt = %v, want %v", got, want)
	}
	if message.CreatedAt != now || message.RoomID != "room-1" || message.UserID != "user-1" || message.Content != "hello" {
		t.Fatalf("NewMessage() returned unexpected fields: %#v", message)
	}
}

func TestNewMessageRejectsInvalidContent(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"", strings.Repeat("x", 2001), "hello\x1b[2J", "line\nfeed"} {
		if _, err := NewMessage("room-1", "user-1", content, time.Now()); err == nil {
			t.Fatalf("NewMessage() accepted invalid content %q", content)
		}
	}
}
