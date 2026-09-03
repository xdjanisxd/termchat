package app

import (
	"context"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"termchat.local/termchat/internal/domain"
	"termchat.local/termchat/internal/security"
	"termchat.local/termchat/internal/store"
)

const MinRoomPasswordLength = 8

var (
	ErrInvalidRoomPassword    = errors.New("room password must contain at least 8 characters")
	ErrInvalidRoomCredentials = errors.New("invalid room name or password")
)

type RoomRepository interface {
	CreateRoom(ctx context.Context, room domain.Room) error
	RoomByName(ctx context.Context, name string) (domain.Room, error)
	RoomByID(ctx context.Context, id string) (domain.Room, error)
	UpdateRoomPassword(ctx context.Context, roomID, ownerID, passwordHash string) error
	DeleteRoom(ctx context.Context, roomID, ownerID string) error
}

type RoomService struct {
	rooms  RoomRepository
	hasher security.PasswordHasher
}

func NewRoomService(rooms RoomRepository, hasher security.PasswordHasher) *RoomService {
	return &RoomService{rooms: rooms, hasher: hasher}
}

func (s *RoomService) Create(ctx context.Context, ownerID, name, password string, now time.Time) (domain.PublicRoom, error) {
	if err := domain.ValidateRoomName(name); err != nil {
		return domain.PublicRoom{}, err
	}
	if utf8.RuneCountInString(password) < MinRoomPasswordLength {
		return domain.PublicRoom{}, ErrInvalidRoomPassword
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return domain.PublicRoom{}, fmt.Errorf("hash room password: %w", err)
	}
	room := domain.Room{
		ID: uuid.NewString(), Name: name, PasswordHash: hash, CreatedBy: ownerID, CreatedAt: now,
	}
	if err := s.rooms.CreateRoom(ctx, room); err != nil {
		return domain.PublicRoom{}, err
	}
	return room.PublicFor(ownerID), nil
}

func (s *RoomService) Join(ctx context.Context, userID, name, password string) (domain.PublicRoom, error) {
	room, err := s.rooms.RoomByName(ctx, name)
	if err != nil || !s.hasher.Verify(room.PasswordHash, password) {
		return domain.PublicRoom{}, ErrInvalidRoomCredentials
	}
	return room.PublicFor(userID), nil
}

func (s *RoomService) InviteRoom(ctx context.Context, ownerID, roomID string) (domain.PublicRoom, error) {
	room, err := s.rooms.RoomByID(ctx, roomID)
	if err != nil {
		return domain.PublicRoom{}, err
	}
	if room.CreatedBy != ownerID {
		return domain.PublicRoom{}, fmt.Errorf("invite room: %w", store.ErrForbidden)
	}
	return room.PublicFor(ownerID), nil
}

func (s *RoomService) ChangePassword(ctx context.Context, ownerID, roomID, password string) error {
	if utf8.RuneCountInString(password) < MinRoomPasswordLength {
		return ErrInvalidRoomPassword
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return fmt.Errorf("hash room password: %w", err)
	}
	return s.rooms.UpdateRoomPassword(ctx, roomID, ownerID, hash)
}

func (s *RoomService) Delete(ctx context.Context, ownerID, roomID string) error {
	return s.rooms.DeleteRoom(ctx, roomID, ownerID)
}
