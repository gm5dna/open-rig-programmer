// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
	"strings"
	"testing"
)

// ErrRejected represents the radio's one and only NAK, "?;" (reference:
// "General framing" — "The only NAK is ?; — an unattributed generic command
// failure"). It carries no cause; tests only check identity and message
// content, never a specific inferred cause.
func TestErrRejected(t *testing.T) {
	if ErrRejected == nil {
		t.Fatal("ErrRejected must not be nil")
	}
	if !strings.Contains(ErrRejected.Error(), "?;") {
		t.Errorf("ErrRejected.Error() = %q, want mention of the literal %q reply", ErrRejected.Error(), "?;")
	}
}

func TestParseError_Error(t *testing.T) {
	err := newParseError([]byte("ID12;"), "bad length")

	if err.Reason != "bad length" {
		t.Errorf("Reason = %q, want %q", err.Reason, "bad length")
	}
	if string(err.Frame) != "ID12;" {
		t.Errorf("Frame = %q, want %q", err.Frame, "ID12;")
	}
	msg := err.Error()
	if !strings.Contains(msg, "bad length") {
		t.Errorf("Error() = %q, want it to contain reason %q", msg, "bad length")
	}
	if !strings.Contains(msg, "ID12;") {
		t.Errorf("Error() = %q, want it to contain offending input %q", msg, "ID12;")
	}
}

// newParseError must copy the input, so mutating the caller's slice after
// the call must not change the error's recorded Frame.
func TestParseError_CopiesInput(t *testing.T) {
	input := []byte("MUTATE")
	err := newParseError(input, "test")
	input[0] = 'X'

	if string(err.Frame) != "MUTATE" {
		t.Errorf("Frame = %q, want copy unaffected by caller mutation, want %q", err.Frame, "MUTATE")
	}
}

// newParseError must truncate very long offending input to a sane length so
// a hostile or corrupt buffer cannot bloat error messages or logs.
func TestParseError_TruncatesLongInput(t *testing.T) {
	huge := strings.Repeat("A", 10_000)
	err := newParseError([]byte(huge), "too long")

	if len(err.Frame) > maxParseErrorFrameLen {
		t.Errorf("len(Frame) = %d, want <= %d (maxParseErrorFrameLen)", len(err.Frame), maxParseErrorFrameLen)
	}
	if len(err.Frame) == 0 {
		t.Error("Frame should not be empty for non-empty input")
	}
}

// ParseError must satisfy the error interface and be distinguishable via
// errors.As from other error types (e.g. ErrRejected).
func TestParseError_ErrorsAs(t *testing.T) {
	var target *ParseError
	err := error(newParseError([]byte("x"), "reason"))

	if !errors.As(err, &target) {
		t.Fatal("errors.As failed to match *ParseError")
	}
	if !errors.Is(err, err) {
		t.Fatal("errors.Is failed to match itself")
	}
}
