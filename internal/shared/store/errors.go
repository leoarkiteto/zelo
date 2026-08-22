package store

import "github.com/leoarkiteto/zelo/internal/shared/model"

// ErrNotFound and ErrDuplicate alias the domain-level sentinels so adapters
// return errors that services can compare with errors.Is.
var (
	ErrNotFound  = model.ErrNotFound
	ErrDuplicate = model.ErrDuplicate
)
