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
	var pce *PortClosedError
	if !errors.As(err, &pce) {
		t.Fatalf("errors.As(%v, *PortClosedError) = false, want true", err)
	}
	if pce.Cause != io.EOF {
		t.Errorf("PortClosedError.Cause = %v, want io.EOF", pce.Cause)
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
	var ce *ContaminatedError
	if !errors.As(err, &ce) {
		t.Fatalf("errors.As(%v, *ContaminatedError) = false, want true", err)
	}
	if ce.Cause.DiscardedLen != 300 {
		t.Errorf("ContaminatedError.Cause.DiscardedLen = %d, want 300", ce.Cause.DiscardedLen)
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
	var qfe *QuarantineFailedError
	if !errors.As(err, &qfe) {
		t.Fatalf("errors.As(%v, *QuarantineFailedError) = false, want true", err)
	}
	if qfe.Cause != ErrPortClosed {
		t.Errorf("QuarantineFailedError.Cause = %v, want ErrPortClosed", qfe.Cause)
	}
}

func TestQuarantineFailedError_NilCauseFallsBackToSentinel(t *testing.T) {
	err := wrapQuarantineFailedErr(nil)
	if err != ErrQuarantineFailed {
		t.Errorf("wrapQuarantineFailedErr(nil) = %v, want the bare ErrQuarantineFailed sentinel", err)
	}
}
