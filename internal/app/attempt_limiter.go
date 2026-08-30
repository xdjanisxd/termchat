package app

import (
	"sync"
	"time"
)

type attemptState struct {
	failures     int
	lastFailure  time.Time
	blockedUntil time.Time
}

type AttemptLimiter struct {
	mu           sync.Mutex
	states       map[string]attemptState
	maxFailures  int
	lockDuration time.Duration
	lastCleanup  time.Time
}

func NewAttemptLimiter(maxFailures int, lockDuration time.Duration) *AttemptLimiter {
	return &AttemptLimiter{
		states:       make(map[string]attemptState),
		maxFailures:  maxFailures,
		lockDuration: lockDuration,
	}
}

func (l *AttemptLimiter) IsBlocked(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.cleanupLocked(now)
	state, exists := l.states[key]
	if !exists || state.blockedUntil.IsZero() {
		return false
	}
	if !now.Before(state.blockedUntil) {
		delete(l.states, key)
		return false
	}
	return true
}

func (l *AttemptLimiter) RecordFailure(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.cleanupLocked(now)
	state := l.states[key]
	if !state.lastFailure.IsZero() && !now.Before(state.lastFailure.Add(l.lockDuration)) {
		state = attemptState{}
	}
	state.failures++
	state.lastFailure = now
	if state.failures >= l.maxFailures {
		state.blockedUntil = now.Add(l.lockDuration)
	}
	l.states[key] = state
}

func (l *AttemptLimiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.states, key)
}

func (l *AttemptLimiter) cleanupLocked(now time.Time) {
	if !l.lastCleanup.IsZero() && now.Sub(l.lastCleanup) < l.lockDuration {
		return
	}
	for key, state := range l.states {
		blockExpired := !state.blockedUntil.IsZero() && !now.Before(state.blockedUntil)
		partialExpired := state.blockedUntil.IsZero() && !state.lastFailure.IsZero() && !now.Before(state.lastFailure.Add(l.lockDuration))
		if blockExpired || partialExpired {
			delete(l.states, key)
		}
	}
	l.lastCleanup = now
}
