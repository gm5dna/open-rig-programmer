// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// TestIsCancelled_Canceled/DeadlineExceeded/Other pin isCancelled's
// classification: task-12 brief §1's Ctrl-C requirement needs read/diff
// to recognise BOTH context.Canceled (signal.NotifyContext firing) and
// context.DeadlineExceeded, wrapped by any depth of %w, as "the run was
// cancelled" — but not an unrelated error.
func TestIsCancelled_Canceled(t *testing.T) {
	err := fmt.Errorf("clone: ReadAll: %w", context.Canceled)
	if !isCancelled(err) {
		t.Errorf("isCancelled(%v) = false, want true", err)
	}
}

func TestIsCancelled_DeadlineExceeded(t *testing.T) {
	err := fmt.Errorf("clone: ReadAll: %w", context.DeadlineExceeded)
	if !isCancelled(err) {
		t.Errorf("isCancelled(%v) = false, want true", err)
	}
}

func TestIsCancelled_Other(t *testing.T) {
	err := errors.New("some unrelated failure")
	if isCancelled(err) {
		t.Errorf("isCancelled(%v) = true, want false", err)
	}
}

func TestIsCancelled_Nil(t *testing.T) {
	if isCancelled(nil) {
		t.Error("isCancelled(nil) = true, want false")
	}
}
