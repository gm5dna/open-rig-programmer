// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"errors"
	"io"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
)

func TestPortClosedError_IsCompatible(t *testing.T) {
	err := wrapClosedErr(io.EOF)
	if !errors.Is(err, ErrPortClosed) {
		t.Errorf("errors.Is(%v, ErrPortClosed) = false, want true", err)
	}
	if !errors.Is(err, io.EOF) {
		t.Errorf("errors.Is(%v, io.EOF) = false, want true (cause must be reachable)", err)
	}
}

func TestPortClosedError_NoCause(t *testing.T) {
	err := wrapClosedErr(nil)
	if err != ErrPortClosed {
		t.Errorf("wrapClosedErr(nil) = %v, want the bare ErrPortClosed sentinel", err)
	}
}

func TestContaminatedError_IsCompatible(t *testing.T) {
	cause := &cat.FrameTooLongError{DiscardedLen: 300}
	err := wrapContaminatedErr(cause)
	if !errors.Is(err, ErrContaminated) {
		t.Errorf("errors.Is(%v, ErrContaminated) = false, want true", err)
	}
	if !errors.Is(err, cat.ErrFrameTooLong) {
		t.Errorf("errors.Is(%v, cat.ErrFrameTooLong) = false, want true (chain must reach the cat sentinel too)", err)
	}
	var ftl *cat.FrameTooLongError
	if !errors.As(err, &ftl) {
		t.Fatalf("errors.As(%v, *cat.FrameTooLongError) = false, want true", err)
	}
	if ftl.DiscardedLen != 300 {
		t.Errorf("FrameTooLongError.DiscardedLen = %d, want 300", ftl.DiscardedLen)
	}
}

func TestContaminatedError_NilCauseFallsBackToSentinel(t *testing.T) {
	err := wrapContaminatedErr(nil)
	if err != ErrContaminated {
		t.Errorf("wrapContaminatedErr(nil) = %v, want the bare ErrContaminated sentinel", err)
	}
}

func TestQuarantineFailedError_IsCompatible(t *testing.T) {
	err := wrapQuarantineFailedErr(ErrPortClosed)
	if !errors.Is(err, ErrQuarantineFailed) {
		t.Errorf("errors.Is(%v, ErrQuarantineFailed) = false, want true", err)
	}
	if !errors.Is(err, ErrPortClosed) {
		t.Errorf("errors.Is(%v, ErrPortClosed) = false, want true (cause must be reachable)", err)
	}
}

func TestQuarantineFailedError_NilCauseFallsBackToSentinel(t *testing.T) {
	err := wrapQuarantineFailedErr(nil)
	if err != ErrQuarantineFailed {
		t.Errorf("wrapQuarantineFailedErr(nil) = %v, want the bare ErrQuarantineFailed sentinel", err)
	}
}
