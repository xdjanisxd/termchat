package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/store"
)

type DB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Store struct {
	db DB
}

func New(db DB) *Store {
	return &Store{db: db}
}

func (s *Store) CreateUser(ctx context.Context, user domain.User) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO users (id, username, password_hash, created_at)
		VALUES ($1, $2, $3, $4)
	`, user.ID, user.Username, user.PasswordHash, user.CreatedAt)
	if err != nil {
		return mapError(err, "create user")
	}
	return nil
}

func (s *Store) UserByUsername(ctx context.Context, username string) (domain.User, error) {
	var user domain.User
	err := s.db.QueryRow(ctx, `
		SELECT id, username, password_hash, created_at
		FROM users
		WHERE username = $1
	`, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return domain.User{}, mapError(err, "find user by username")
	}
	return user, nil
}

func (s *Store) DeleteUser(ctx context.Context, userID string) error {
	result, err := s.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return mapError(err, "delete user")
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("delete user: %w", store.ErrNotFound)
	}
	return nil
}

func (s *Store) CreateRoom(ctx context.Context, room domain.Room) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO rooms (id, name, password_hash, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, room.ID, room.Name, room.PasswordHash, room.CreatedBy, room.CreatedAt)
	if err != nil {
		return mapError(err, "create room")
	}
	return nil
}

func (s *Store) RoomByName(ctx context.Context, name string) (domain.Room, error) {
	var room domain.Room
	err := s.db.QueryRow(ctx, `
		SELECT id, name, password_hash, created_by, created_at
		FROM rooms
		WHERE name = $1
	`, name).Scan(&room.ID, &room.Name, &room.PasswordHash, &room.CreatedBy, &room.CreatedAt)
	if err != nil {
		return domain.Room{}, mapError(err, "find room by name")
	}
	return room, nil
}

func (s *Store) RoomByID(ctx context.Context, id string) (domain.Room, error) {
	var room domain.Room
	err := s.db.QueryRow(ctx, `SELECT id, name, password_hash, created_by, created_at FROM rooms WHERE id = $1`, id).Scan(&room.ID, &room.Name, &room.PasswordHash, &room.CreatedBy, &room.CreatedAt)
	if err != nil {
		return domain.Room{}, mapError(err, "find room by id")
	}
	return room, nil
}

func (s *Store) UpdateRoomPassword(ctx context.Context, roomID, ownerID, passwordHash string) error {
	result, err := s.db.Exec(ctx, `
		UPDATE rooms
		SET password_hash = $1
		WHERE id = $2 AND created_by = $3
	`, passwordHash, roomID, ownerID)
	if err != nil {
		return mapError(err, "update room password")
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("update room password: %w", store.ErrForbidden)
	}
	return nil
}

func (s *Store) DeleteRoom(ctx context.Context, roomID, ownerID string) error {
	result, err := s.db.Exec(ctx, `
		DELETE FROM rooms
		WHERE id = $1 AND created_by = $2
	`, roomID, ownerID)
	if err != nil {
		return mapError(err, "delete room")
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("delete room: %w", store.ErrForbidden)
	}
	return nil
}

func (s *Store) SaveMessage(ctx context.Context, message domain.Message) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO messages (id, room_id, user_id, content, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, message.ID, message.RoomID, message.UserID, message.Content, message.CreatedAt, message.ExpiresAt)
	if err != nil {
		return mapError(err, "save message")
	}
	return nil
}

func (s *Store) MessagesBefore(ctx context.Context, roomID, beforeMessageID string, limit int) ([]domain.Message, error) {
	if limit < 1 || limit > 51 {
		limit = 51
	}
	rows, err := s.db.Query(ctx, `
		WITH recent_messages AS (
			SELECT m.id, m.room_id, m.user_id, u.username, m.content, m.created_at, m.expires_at
			FROM messages m
			JOIN users u ON u.id = m.user_id
			WHERE m.room_id = $1
				AND m.expires_at > NOW()
				AND ($2 = '' OR (m.created_at, m.id) < (
					SELECT cursor.created_at, cursor.id
					FROM messages cursor
					WHERE cursor.room_id = $1 AND cursor.id = NULLIF($2, '')::uuid
				))
			ORDER BY m.created_at DESC, m.id DESC
			LIMIT $3
		)
		SELECT id, room_id, user_id, username, content, created_at, expires_at
		FROM recent_messages
		ORDER BY created_at ASC, id ASC
	`, roomID, beforeMessageID, limit)
	if err != nil {
		return nil, mapError(err, "list recent messages")
	}
	defer rows.Close()

	messages := make([]domain.Message, 0, limit)
	for rows.Next() {
		var message domain.Message
		if err := rows.Scan(
			&message.ID, &message.RoomID, &message.UserID, &message.Username,
			&message.Content, &message.CreatedAt, &message.ExpiresAt,
		); err != nil {
			return nil, mapError(err, "scan recent message")
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err, "read recent messages")
	}
	return messages, nil
}

func (s *Store) RecentMessages(ctx context.Context, roomID string, limit int) ([]domain.Message, error) {
	return s.MessagesBefore(ctx, roomID, "", limit)
}

func (s *Store) DeleteExpiredMessages(ctx context.Context, now time.Time) (int64, error) {
	result, err := s.db.Exec(ctx, `DELETE FROM messages WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, mapError(err, "delete expired messages")
	}
	return result.RowsAffected(), nil
}

func mapError(err error, operation string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return fmt.Errorf("%s: %w", operation, store.ErrConflict)
		case "23503":
			return fmt.Errorf("%s: %w", operation, store.ErrNotFound)
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%s: %w", operation, store.ErrNotFound)
	}
	return fmt.Errorf("%s: %w", operation, err)
}
