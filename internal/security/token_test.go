package security

import (
	"testing"
	"time"
)

func TestTokenManagerIssuesAndParsesToken(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 26, 15, 0, 0, 0, time.UTC)
	manager := NewTokenManager([]byte("01234567890123456789012345678901"), 24*time.Hour)
	token, err := manager.Issue("user-1", "alice", now)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	claims, err := manager.Parse(token, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if claims.UserID != "user-1" || claims.Username != "alice" {
		t.Fatalf("Parse() claims = %#v", claims)
	}
}

func TestTokenManagerRejectsExpiredToken(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 26, 15, 0, 0, 0, time.UTC)
	manager := NewTokenManager([]byte("01234567890123456789012345678901"), time.Hour)
	token, err := manager.Issue("user-1", "alice", now)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if _, err := manager.Parse(token, now.Add(2*time.Hour)); err == nil {
		t.Fatal("Parse() accepted an expired token")
	}
}
