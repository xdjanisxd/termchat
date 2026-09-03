package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	pgxmock "github.com/pashagolub/pgxmock/v4"

	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/store"
)

func TestStoreCreateUser(t *testing.T) {
	t.Parallel()

	db, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer db.Close()
	repository := New(db)
	user := domain.User{
		ID: "ca5d3e57-d720-4a82-bd01-607a9d2b0450", Username: "alice", PasswordHash: "argon-hash",
		CreatedAt: time.Date(2026, time.August, 26, 15, 0, 0, 0, time.UTC),
	}
	db.ExpectExec("INSERT INTO users").
		WithArgs(user.ID, user.Username, user.PasswordHash, user.CreatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := repository.CreateUser(context.Background(), user); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := db.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestStoreCreateUserMapsUniqueViolation(t *testing.T) {
	t.Parallel()

	db, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer db.Close()
	repository := New(db)
	user := domain.User{ID: "ca5d3e57-d720-4a82-bd01-607a9d2b0450", Username: "alice", PasswordHash: "hash", CreatedAt: time.Now()}
	db.ExpectExec("INSERT INTO users").
		WithArgs(user.ID, user.Username, user.PasswordHash, user.CreatedAt).
		WillReturnError(&pgconn.PgError{Code: "23505"})

	if err := repository.CreateUser(context.Background(), user); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("CreateUser() error = %v, want ErrConflict", err)
	}
}

func TestStoreUserByUsername(t *testing.T) {
	t.Parallel()

	db, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer db.Close()
	repository := New(db)
	createdAt := time.Date(2026, time.August, 26, 15, 0, 0, 0, time.UTC)
	db.ExpectQuery("SELECT id, username, password_hash, created_at FROM users").
		WithArgs("alice").
		WillReturnRows(pgxmock.NewRows([]string{"id", "username", "password_hash", "created_at"}).
			AddRow("user-1", "alice", "argon-hash", createdAt))

	user, err := repository.UserByUsername(context.Background(), "alice")
	if err != nil {
		t.Fatalf("UserByUsername() error = %v", err)
	}
	if user.ID != "user-1" || user.Username != "alice" || user.PasswordHash != "argon-hash" {
		t.Fatalf("UserByUsername() = %#v", user)
	}
}

func TestStoreCreateAndFindRoom(t *testing.T) {
	t.Parallel()

	db, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer db.Close()
	repository := New(db)
	createdAt := time.Date(2026, time.August, 26, 15, 0, 0, 0, time.UTC)
	room := domain.Room{ID: "room-1", Name: "private_room", PasswordHash: "argon-hash", CreatedBy: "owner-1", CreatedAt: createdAt}
	db.ExpectExec("INSERT INTO rooms").
		WithArgs(room.ID, room.Name, room.PasswordHash, room.CreatedBy, room.CreatedAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	if err := repository.CreateRoom(context.Background(), room); err != nil {
		t.Fatalf("CreateRoom() error = %v", err)
	}

	db.ExpectQuery("SELECT id, name, password_hash, created_by, created_at FROM rooms").
		WithArgs(room.Name).
		WillReturnRows(pgxmock.NewRows([]string{"id", "name", "password_hash", "created_by", "created_at"}).
			AddRow(room.ID, room.Name, room.PasswordHash, room.CreatedBy, room.CreatedAt))
	found, err := repository.RoomByName(context.Background(), room.Name)
	if err != nil {
		t.Fatalf("RoomByName() error = %v", err)
	}
	if found != room {
		t.Fatalf("RoomByName() = %#v, want %#v", found, room)
	}
}

func TestStoreRoomOwnerMutations(t *testing.T) {
	t.Parallel()

	db, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer db.Close()
	repository := New(db)

	db.ExpectExec("UPDATE rooms SET password_hash").
		WithArgs("new-hash", "room-1", "owner-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	if err := repository.UpdateRoomPassword(context.Background(), "room-1", "owner-1", "new-hash"); err != nil {
		t.Fatalf("UpdateRoomPassword() owner error = %v", err)
	}

	db.ExpectExec("UPDATE rooms SET password_hash").
		WithArgs("other-hash", "room-1", "user-2").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))
	if err := repository.UpdateRoomPassword(context.Background(), "room-1", "user-2", "other-hash"); !errors.Is(err, store.ErrForbidden) {
		t.Fatalf("UpdateRoomPassword() non-owner error = %v, want ErrForbidden", err)
	}

	db.ExpectExec("DELETE FROM rooms").
		WithArgs("room-1", "owner-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	if err := repository.DeleteRoom(context.Background(), "room-1", "owner-1"); err != nil {
		t.Fatalf("DeleteRoom() owner error = %v", err)
	}
}

func TestStoreSaveAndReadMessagesBefore(t *testing.T) {
	t.Parallel()

	db, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer db.Close()
	repository := New(db)
	createdAt := time.Date(2026, time.August, 26, 15, 0, 0, 0, time.UTC)
	expiresAt := createdAt.Add(7 * 24 * time.Hour)
	message := domain.Message{ID: "message-1", RoomID: "room-1", UserID: "user-1", Content: "hello", CreatedAt: createdAt, ExpiresAt: expiresAt}

	db.ExpectExec("INSERT INTO messages").
		WithArgs(message.ID, message.RoomID, message.UserID, message.Content, message.CreatedAt, message.ExpiresAt).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	if err := repository.SaveMessage(context.Background(), message); err != nil {
		t.Fatalf("SaveMessage() error = %v", err)
	}

	db.ExpectQuery("cursor.id = NULLIF\\(\\$2, ''\\)::uuid").
		WithArgs("room-1", "message-2", 51).
		WillReturnRows(pgxmock.NewRows([]string{"id", "room_id", "user_id", "username", "content", "created_at", "expires_at"}).
			AddRow("message-1", "room-1", "user-1", "alice", "hello", createdAt, expiresAt))
	messages, err := repository.MessagesBefore(context.Background(), "room-1", "message-2", 51)
	if err != nil {
		t.Fatalf("MessagesBefore() error = %v", err)
	}
	if len(messages) != 1 || messages[0].Username != "alice" || messages[0].ExpiresAt != expiresAt {
		t.Fatalf("MessagesBefore() = %#v", messages)
	}
}

func TestStoreDeleteExpiredMessages(t *testing.T) {
	t.Parallel()

	db, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock.NewPool() error = %v", err)
	}
	defer db.Close()
	repository := New(db)
	now := time.Date(2026, time.September, 2, 15, 0, 0, 0, time.UTC)
	db.ExpectExec("DELETE FROM messages WHERE expires_at <=").
		WithArgs(now).
		WillReturnResult(pgxmock.NewResult("DELETE", 3))

	deleted, err := repository.DeleteExpiredMessages(context.Background(), now)
	if err != nil {
		t.Fatalf("DeleteExpiredMessages() error = %v", err)
	}
	if deleted != 3 {
		t.Fatalf("DeleteExpiredMessages() = %d, want 3", deleted)
	}
}
