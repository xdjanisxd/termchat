package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maxSessionFileSize = 64 * 1024

var (
	ErrNoSession      = errors.New("no saved session")
	ErrInvalidSession = errors.New("invalid session")
)

type SessionStore struct {
	path string
}

func NewSessionStore(path string) SessionStore {
	return SessionStore{path: path}
}

func DefaultSessionStore() (SessionStore, error) {
	configDirectory, err := os.UserConfigDir()
	if err != nil {
		return SessionStore{}, fmt.Errorf("find user config directory: %w", err)
	}
	return NewSessionStore(filepath.Join(configDirectory, "termchat", "session.json")), nil
}

func (s SessionStore) Save(session Session) error {
	if err := validateSession(session); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}
	encoded, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.path), ".session-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary session file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("restrict session permissions: %w", err)
	}
	if _, err := temporary.Write(encoded); err != nil {
		temporary.Close()
		return fmt.Errorf("write session: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync session: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close session: %w", err)
	}
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("replace old session: %w", err)
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("install session file: %w", err)
	}
	return nil
}

func (s SessionStore) Load() (Session, error) {
	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Session{}, ErrNoSession
	}
	if err != nil {
		return Session{}, fmt.Errorf("open session: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(io.LimitReader(file, maxSessionFileSize))
	decoder.DisallowUnknownFields()
	var session Session
	if err := decoder.Decode(&session); err != nil {
		return Session{}, fmt.Errorf("%w: %v", ErrInvalidSession, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Session{}, ErrInvalidSession
	}
	if err := validateSession(session); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s SessionStore) Clear() error {
	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove session: %w", err)
	}
	return nil
}

func validateSession(session Session) error {
	if session.Token == "" || session.User.ID == "" || session.User.Username == "" {
		return ErrInvalidSession
	}
	return nil
}
