package model

import "errors"

// Domain-level sentinels shared by all layers.
var (
	// ErrNotFound indicates a record does not exist.
	ErrNotFound = errors.New("not found")
	// ErrDuplicate indicates a uniqueness constraint was violated.
	ErrDuplicate = errors.New("duplicate")
)
