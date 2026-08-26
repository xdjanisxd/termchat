package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/security"
	"termchat.local/termchat/internal/store"
)

type fakeRoomRepository struct {
	byID   map[string]domain.Room
	byName map[string]string
}

func newFakeRoomRepository() *fakeRoomRepository {
	return &fakeRoomRepository{byID: make(map[string]domain.Room), byName: make(map[string]string)}
}

func (r *fakeRoomRepository) CreateRoom(_ context.Context, room domain.Room) error {
	if _, exists := r.byName[room.Name]; exists {
		return store.ErrConflict
	}
	r.byID[room.ID] = room
	r.byName[room.Name] = room.ID
	return nil
}

func (r *fakeRoomRepository) RoomByName(_ context.Context, name string) (domain.Room, error) {
	id, exists := r.byName[name]
	if !exists {
		return domain.Room{}, store.ErrNotFound
	}
	return r.byID[id], nil
}

func (r *fakeRoomRepository) UpdateRoomPassword(_ context.Context, roomID, ownerID, passwordHash string) error {
	room, exists := r.byID[roomID]
	if !exists {
		return store.ErrNotFound
	}
	if room.CreatedBy != ownerID {
		return store.ErrForbidden
	}
	room.PasswordHash = passwordHash
	r.byID[roomID] = room
	return nil
}

func (r *fakeRoomRepository) DeleteRoom(_ context.Context, roomID, ownerID string) error {
	room, exists := r.byID[roomID]
	if !exists {
		return store.ErrNotFound
	}
	if room.CreatedBy != ownerID {
		return store.ErrForbidden
	}
	delete(r.byID, roomID)
	delete(r.byName, room.Name)
	return nil
}

func TestRoomServiceCreateAndJoinPrivateRoom(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 26, 15, 0, 0, 0, time.UTC)
	repo := newFakeRoomRepository()
	hasher := security.NewPasswordHasher(security.Argon2Params{
		Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})
	service := NewRoomService(repo, hasher)

	room, err := service.Create(context.Background(), "owner-1", "rust-devs_01", "roompass", now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	stored := repo.byID[room.ID]
	if stored.PasswordHash == "roompass" || stored.CreatedBy != "owner-1" {
		t.Fatalf("Create() stored unsafe or invalid room: %#v", stored)
	}
	joined, err := service.Join(context.Background(), "user-2", "rust-devs_01", "roompass")
	if err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	if joined.ID != room.ID || joined.Name != room.Name || joined.IsOwner {
		t.Fatalf("Join() room = %#v, want non-owner access to %#v", joined, room)
	}
	if _, err := service.Join(context.Background(), "user-2", "rust-devs_01", "wrongpass"); !errors.Is(err, ErrInvalidRoomCredentials) {
		t.Fatalf("Join() error = %v, want ErrInvalidRoomCredentials", err)
	}
}

func TestRoomServiceOwnerCanChangePasswordAndDeleteRoom(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 26, 15, 0, 0, 0, time.UTC)
	repo := newFakeRoomRepository()
	hasher := security.NewPasswordHasher(security.Argon2Params{
		Memory: 8 * 1024, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32,
	})
	service := NewRoomService(repo, hasher)
	room, err := service.Create(context.Background(), "owner-1", "private_room", "old-pass", now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := service.ChangePassword(context.Background(), "user-2", room.ID, "new-pass"); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("ChangePassword() non-owner error = %v, want ErrForbidden", err)
	}
	if err := service.ChangePassword(context.Background(), "owner-1", room.ID, "new-pass"); err != nil {
		t.Fatalf("ChangePassword() owner error = %v", err)
	}
	if _, err := service.Join(context.Background(), "user-2", room.Name, "old-pass"); !errors.Is(err, ErrInvalidRoomCredentials) {
		t.Fatalf("Join() accepted old room password: %v", err)
	}
	if _, err := service.Join(context.Background(), "user-2", room.Name, "new-pass"); err != nil {
		t.Fatalf("Join() rejected new room password: %v", err)
	}

	if err := service.Delete(context.Background(), "user-2", room.ID); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("Delete() non-owner error = %v, want ErrForbidden", err)
	}
	if err := service.Delete(context.Background(), "owner-1", room.ID); err != nil {
		t.Fatalf("Delete() owner error = %v", err)
	}
	if _, err := service.Join(context.Background(), "owner-1", room.Name, "new-pass"); !errors.Is(err, ErrInvalidRoomCredentials) {
		t.Fatalf("Join() found deleted room: %v", err)
	}
}
