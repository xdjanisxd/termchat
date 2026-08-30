package app

import (
	"testing"
	"time"
)

func TestAttemptLimiterBlocksAfterFourFailures(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 31, 0, 0, 0, 0, time.UTC)
	limiter := NewAttemptLimiter(4, 5*time.Minute)

	for attempt := 1; attempt <= 4; attempt++ {
		if limiter.IsBlocked("ip:192.0.2.1", now) {
			t.Fatalf("attempt %d blocked before failure was recorded", attempt)
		}
		limiter.RecordFailure("ip:192.0.2.1", now)
	}
	if !limiter.IsBlocked("ip:192.0.2.1", now) {
		t.Fatal("fifth attempt was not blocked")
	}
}

func TestAttemptLimiterUnblocksAfterLockDuration(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 31, 0, 0, 0, 0, time.UTC)
	limiter := NewAttemptLimiter(4, 5*time.Minute)
	for range 4 {
		limiter.RecordFailure("user:user-1", now)
	}

	if !limiter.IsBlocked("user:user-1", now.Add(5*time.Minute-time.Nanosecond)) {
		t.Fatal("user was unblocked before the lock duration elapsed")
	}
	if limiter.IsBlocked("user:user-1", now.Add(5*time.Minute)) {
		t.Fatal("user remained blocked after the lock duration elapsed")
	}
}

func TestAttemptLimiterResetClearsFailures(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 31, 0, 0, 0, 0, time.UTC)
	limiter := NewAttemptLimiter(4, 5*time.Minute)
	for range 3 {
		limiter.RecordFailure("ip:192.0.2.1", now)
	}
	limiter.Reset("ip:192.0.2.1")
	limiter.RecordFailure("ip:192.0.2.1", now)

	if limiter.IsBlocked("ip:192.0.2.1", now) {
		t.Fatal("failure count survived a successful-attempt reset")
	}
}

func TestAttemptLimiterDropsStalePartialFailures(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 31, 0, 0, 0, 0, time.UTC)
	limiter := NewAttemptLimiter(4, 5*time.Minute)
	for range 3 {
		limiter.RecordFailure("ip:192.0.2.1", now)
	}
	limiter.RecordFailure("ip:192.0.2.2", now.Add(5*time.Minute))

	if _, exists := limiter.states["ip:192.0.2.1"]; exists {
		t.Fatal("stale partial failure state was not cleaned up")
	}
	limiter.RecordFailure("ip:192.0.2.1", now.Add(5*time.Minute))
	if limiter.IsBlocked("ip:192.0.2.1", now.Add(5*time.Minute)) {
		t.Fatal("stale failures contributed to a new attempt window")
	}
}

func TestAttemptLimiterDropsStalePartialFailuresPerKey(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 31, 0, 0, 0, 0, time.UTC)
	limiter := NewAttemptLimiter(4, 5*time.Minute)

	limiter.RecordFailure("ip:192.0.2.2", now.Add(-time.Second))
	for range 3 {
		limiter.RecordFailure("ip:192.0.2.1", now)
	}
	limiter.RecordFailure("ip:192.0.2.2", now.Add(5*time.Minute-time.Second))
	limiter.RecordFailure("ip:192.0.2.1", now.Add(5*time.Minute))

	if limiter.IsBlocked("ip:192.0.2.1", now.Add(5*time.Minute)) {
		t.Fatal("stale per-key failures contributed to a new attempt window")
	}
}
