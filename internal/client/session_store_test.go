package client

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestSessionStoreRoundTripAndClear(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "nested", "session.json")
	store := NewSessionStore(path)
	want := Session{Token: "jwt-token"}
	want.User.ID = "user-1"
	want.User.Username = "alice"
	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got != want {
		t.Fatalf("Load() = %#v, want %#v", got, want)
	}
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if _, err := store.Load(); !errors.Is(err, ErrNoSession) {
		t.Fatalf("Load() after Clear() error = %v, want ErrNoSession", err)
	}
}

func TestSessionStoreRejectsIncompleteSession(t *testing.T) {
	t.Parallel()

	store := NewSessionStore(filepath.Join(t.TempDir(), "session.json"))
	if err := store.Save(Session{}); !errors.Is(err, ErrInvalidSession) {
		t.Fatalf("Save() error = %v, want ErrInvalidSession", err)
	}
}
