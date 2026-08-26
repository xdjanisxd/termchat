package app

import (
	"context"
	"testing"
	"time"
)

type fakeExpiredMessageStore struct {
	calledAt time.Time
	deleted  int64
	err      error
}

func (s *fakeExpiredMessageStore) DeleteExpiredMessages(_ context.Context, now time.Time) (int64, error) {
	s.calledAt = now
	return s.deleted, s.err
}

func TestCleanupExpiredMessages(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.September, 2, 15, 0, 0, 0, time.UTC)
	store := &fakeExpiredMessageStore{deleted: 3}
	deleted, err := CleanupExpiredMessages(context.Background(), store, now)
	if err != nil {
		t.Fatalf("CleanupExpiredMessages() error = %v", err)
	}
	if deleted != 3 || store.calledAt != now {
		t.Fatalf("CleanupExpiredMessages() = %d, calledAt = %v", deleted, store.calledAt)
	}
}
