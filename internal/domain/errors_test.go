package domain

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrors_AreNonNil(t *testing.T) {
	sentinels := []struct {
		name string
		err  error
	}{
		{"ErrNotFound", ErrNotFound},
		{"ErrConflict", ErrConflict},
		{"ErrBusinessRule", ErrBusinessRule},
		{"ErrValidation", ErrValidation},
		{"ErrInternalFailure", ErrInternalFailure},
	}
	for _, s := range sentinels {
		if s.err == nil {
			t.Errorf("%s is nil", s.name)
		}
	}
}

func TestErrors_AreDistinct(t *testing.T) {
	all := []error{ErrNotFound, ErrConflict, ErrBusinessRule, ErrValidation, ErrInternalFailure}
	for i, a := range all {
		for j, b := range all {
			if i != j && errors.Is(a, b) {
				t.Errorf("sentinel %d and %d should be distinct but errors.Is matches them", i, j)
			}
		}
	}
}

func TestErrors_Wrap(t *testing.T) {
	cases := []struct {
		name     string
		sentinel error
	}{
		{"ErrNotFound", ErrNotFound},
		{"ErrConflict", ErrConflict},
		{"ErrBusinessRule", ErrBusinessRule},
		{"ErrValidation", ErrValidation},
		{"ErrInternalFailure", ErrInternalFailure},
	}
	for _, tc := range cases {
		wrapped := fmt.Errorf("some context: %w", tc.sentinel)
		if !errors.Is(wrapped, tc.sentinel) {
			t.Errorf("%s: errors.Is did not match through wrapping", tc.name)
		}
		doubleWrapped := fmt.Errorf("outer: %w", wrapped)
		if !errors.Is(doubleWrapped, tc.sentinel) {
			t.Errorf("%s: errors.Is did not match through double wrapping", tc.name)
		}
	}
}

func TestErrors_Messages(t *testing.T) {
	cases := []struct {
		err     error
		wantMsg string
	}{
		{ErrNotFound, "not found"},
		{ErrConflict, "conflict"},
		{ErrBusinessRule, "business rule violation"},
		{ErrValidation, "validation error"},
		{ErrInternalFailure, "internal failure"},
	}
	for _, tc := range cases {
		if tc.err.Error() != tc.wantMsg {
			t.Errorf("got %q, want %q", tc.err.Error(), tc.wantMsg)
		}
	}
}
