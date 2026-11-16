package store

import (
	"errors"

	"github.com/leoarkiteto/zelo/internal/shared/model"
)

// ErrNotFound and ErrDuplicate alias the domain-level sentinels so adapters
// return errors that services can compare with errors.Is.
var (
	ErrNotFound  = model.ErrNotFound
	ErrDuplicate = model.ErrDuplicate
)

// ErrLateFeeAboveBRLCap is returned by CondominiumStore.SetLateFeeSettings when
// the condominium's current currency is BRL and the requested multa exceeds
// maxBRLFeeBP.
var ErrLateFeeAboveBRLCap = errors.New("late fee above BRL cap")
