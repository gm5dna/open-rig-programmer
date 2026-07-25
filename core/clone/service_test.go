// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"testing"
	"time"
)

// TestNewService_Defaults: without any Option, a Service gets working
// no-op defaults — nopLogger, time.Now, nopProgress — and a distinct
// generation per call.
func TestNewService_Defaults(t *testing.T) {
	_, sess := openSimSession(t)
	store := newStore(t)

	s1 := NewService(sess, store)
	s2 := NewService(sess, store)

	if s1.generation == s2.generation {
		t.Errorf("two NewService calls got the same generation %d, want distinct", s1.generation)
	}
	if s1.logger == nil {
		t.Error("default logger is nil, want nopLogger")
	}
	if s1.progress == nil {
		t.Error("default progress is nil, want nopProgress")
	}
	if s1.now == nil {
		t.Fatal("default now is nil, want time.Now")
	}
	// Defaults must not panic when called.
	s1.logger.Printf("smoke test %d", 1)
	s1.progress("read", 1, 1, "001")
	if s1.now().IsZero() {
		t.Error("default now() returned the zero time")
	}
}

// TestNewService_Options: WithLogger/WithNow/WithProgress override the
// defaults; a nil argument to any of them is ignored, keeping the default.
func TestNewService_Options(t *testing.T) {
	_, sess := openSimSession(t)
	store := newStore(t)

	var loggedCalls int
	logger := testLogger{fn: func(string, ...any) { loggedCalls++ }}

	var progressCalls int
	progress := func(string, int, int, string) { progressCalls++ }

	fixed := func() time.Time { return fixedNow }

	s := NewService(sess, store, WithLogger(logger), WithNow(fixed), WithProgress(progress))

	s.logger.Printf("x")
	if loggedCalls != 1 {
		t.Errorf("WithLogger not wired: loggedCalls = %d, want 1", loggedCalls)
	}
	s.progress("read", 1, 1, "001")
	if progressCalls != 1 {
		t.Errorf("WithProgress not wired: progressCalls = %d, want 1", progressCalls)
	}
	if got := s.now(); !got.Equal(fixedNow) {
		t.Errorf("WithNow not wired: now() = %v, want %v", got, fixedNow)
	}

	// nil options are ignored, not applied.
	s2 := NewService(sess, store, WithLogger(nil), WithNow(nil), WithProgress(nil))
	if s2.logger == nil || s2.now == nil || s2.progress == nil {
		t.Error("a nil Option argument must be ignored, not leave the field nil")
	}
}

// testLogger adapts a func to the Logger interface for assertions.
type testLogger struct {
	fn func(format string, args ...any)
}

func (l testLogger) Printf(format string, args ...any) { l.fn(format, args...) }
