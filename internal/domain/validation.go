package domain

import (
	"errors"
	"regexp"
)

var (
	usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,24}$`)
	roomNamePattern = regexp.MustCompile(`^[a-z0-9_-]{3,32}$`)
)

var (
	ErrInvalidUsername = errors.New("username must be 3-24 characters and contain only lowercase letters, digits, or underscore")
	ErrInvalidRoomName = errors.New("room name must be 3-32 characters and contain only lowercase letters, digits, hyphen, or underscore")
)

func ValidateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return ErrInvalidUsername
	}
	return nil
}

func ValidateRoomName(name string) error {
	if !roomNamePattern.MatchString(name) {
		return ErrInvalidRoomName
	}
	return nil
}
