package domain

import "errors"

// Sentinel errors shared across layers. Adapters translate driver/vendor errors
// into these so that services and handlers never depend on implementation
// details.
var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrInvalidInput = errors.New("invalid input")
)
