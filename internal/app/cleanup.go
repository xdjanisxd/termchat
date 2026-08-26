package app

import (
	"context"
	"time"
)

type ExpiredMessageStore interface {
	DeleteExpiredMessages(ctx context.Context, now time.Time) (int64, error)
}

func CleanupExpiredMessages(ctx context.Context, messages ExpiredMessageStore, now time.Time) (int64, error) {
	return messages.DeleteExpiredMessages(ctx, now)
}

func RunCleanupWorker(ctx context.Context, messages ExpiredMessageStore, interval time.Duration, report func(int64, error)) {
	run := func() {
		deleted, err := CleanupExpiredMessages(ctx, messages, time.Now().UTC())
		if report != nil {
			report(deleted, err)
		}
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
