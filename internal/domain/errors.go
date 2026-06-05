// Package domain contains the core domain entities, interfaces, and error types.
package domain

import "errors"

// Sentinel errors — wrap with context using fmt.Errorf("...: %w", ErrX).
var (
	ErrNotFound        = errors.New("not found")
	ErrConflict        = errors.New("conflict")
	ErrBusinessRule    = errors.New("business rule violation")
	ErrValidation      = errors.New("validation error")
	ErrInternalFailure = errors.New("internal failure")
)
