package store

import "errors"

var (
	ErrNotFound  = errors.New("record not found")
	ErrConflict  = errors.New("record already exists")
	ErrForbidden = errors.New("operation forbidden")
)
