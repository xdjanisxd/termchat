package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"termchat.local/termchat/internal/domain"
)

func TestStorePostgresIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("pool.Ping() error = %v", err)
	}
	if _, err := pool.Exec(ctx, "TRUNCATE messages, rooms, users CASCADE"); err != nil {
		t.Fatalf("truncate test tables: %v", err)
	}

	repository := New(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	user := domain.User{ID: uuid.NewString(), Username: "alice", PasswordHash: "argon-hash", CreatedAt: now}
	if err := repository.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	foundUser, err := repository.UserByUsername(ctx, user.Username)
	if err != nil || foundUser.ID != user.ID {
		t.Fatalf("UserByUsername() = %#v, %v", foundUser, err)
	}

	room := domain.Room{ID: uuid.NewString(), Name: "private_room", PasswordHash: "room-hash", CreatedBy: user.ID, CreatedAt: now}
	if err := repository.CreateRoom(ctx, room); err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}
	foundRoom, err := repository.RoomByName(ctx, room.Name)
	if err != nil || foundRoom.ID != room.ID {
		t.Fatalf("RoomByName() = %#v, %v", foundRoom, err)
	}

	message, err := domain.NewMessage(room.ID, user.ID, "hello", now)
	if err != nil {
		t.Fatalf("NewMessage() error = %v", err)
	}
	if err := repository.SaveMessage(ctx, message); err != nil {
		t.Fatalf("SaveMessage() error = %v", err)
	}
	history, err := repository.RecentMessages(ctx, room.ID, 50)
	if err != nil || len(history) != 1 || history[0].Username != user.Username {
		t.Fatalf("RecentMessages() = %#v, %v", history, err)
	}

	deleted, err := repository.DeleteExpiredMessages(ctx, message.ExpiresAt)
	if err != nil || deleted != 1 {
		t.Fatalf("DeleteExpiredMessages() = %d, %v", deleted, err)
	}
}
