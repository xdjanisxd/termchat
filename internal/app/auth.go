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

const MinUserPasswordLength = 10

var (
	ErrInvalidPassword    = errors.New("password must contain at least 10 characters")
	ErrInvalidCredentials = errors.New("invalid username or password")
)

type UserRepository interface {
	CreateUser(ctx context.Context, user domain.User) error
	UserByUsername(ctx context.Context, username string) (domain.User, error)
}

type AuthResult struct {
	User  domain.PublicUser `json:"user"`
	Token string            `json:"token"`
}

type AuthService struct {
	users  UserRepository
	hasher security.PasswordHasher
	tokens security.TokenManager
}

func NewAuthService(users UserRepository, hasher security.PasswordHasher, tokens security.TokenManager) *AuthService {
	return &AuthService{users: users, hasher: hasher, tokens: tokens}
}

func (s *AuthService) Register(ctx context.Context, username, password string, now time.Time) (AuthResult, error) {
	if err := domain.ValidateUsername(username); err != nil {
		return AuthResult{}, err
	}
	if utf8.RuneCountInString(password) < MinUserPasswordLength {
		return AuthResult{}, ErrInvalidPassword
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return AuthResult{}, fmt.Errorf("hash password: %w", err)
	}
	user := domain.User{
		ID: uuid.NewString(), Username: username, PasswordHash: hash, CreatedAt: now,
	}
	if err := s.users.CreateUser(ctx, user); err != nil {
		return AuthResult{}, err
	}
	return s.issue(user, now)
}

func (s *AuthService) Login(ctx context.Context, username, password string, now time.Time) (AuthResult, error) {
	user, err := s.users.UserByUsername(ctx, username)
	if err != nil || !s.hasher.Verify(user.PasswordHash, password) {
		return AuthResult{}, ErrInvalidCredentials
	}
	return s.issue(user, now)
}

func (s *AuthService) issue(user domain.User, now time.Time) (AuthResult, error) {
	token, err := s.tokens.Issue(user.ID, user.Username, now)
	if err != nil {
		return AuthResult{}, fmt.Errorf("issue token: %w", err)
	}
	return AuthResult{User: user.Public(), Token: token}, nil
}

func IsConflict(err error) bool {
	return errors.Is(err, store.ErrConflict)
}
