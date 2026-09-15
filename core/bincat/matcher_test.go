// SPDX-License-Identifier: GPL-3.0-or-later

package bincat

import (
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

func TestExactLengthMatcherAcceptsAnything(t *testing.T) {
	m := ExactLengthMatcher()
	for _, frame := range [][]byte{nil, {}, {1, 2, 3}, make([]byte, 649)} {
		if !m(frame) {
			t.Errorf("ExactLengthMatcher()(% x) = false, want true", frame)
		}
	}
}

func TestReadSpec(t *testing.T) {
	spec := ReadSpec(2*time.Second, 1)
	if spec.Class != transport.ClassRead {
		t.Errorf("Class = %v, want ClassRead", spec.Class)
	}
	if spec.Match == nil {
		t.Error("Match is nil, want ExactLengthMatcher")
	}
	if spec.Timeout != 2*time.Second {
		t.Errorf("Timeout = %v, want 2s", spec.Timeout)
	}
	if spec.RetryReads != 1 {
		t.Errorf("RetryReads = %d, want 1", spec.RetryReads)
	}
}

func TestWriteSpec(t *testing.T) {
	spec := WriteSpec(150 * time.Millisecond)
	if spec.Class != transport.ClassWrite {
		t.Errorf("Class = %v, want ClassWrite", spec.Class)
	}
	if spec.Match != nil {
		t.Error("Match is non-nil, want nil for ClassWrite")
	}
	if spec.ErrorWindow != 150*time.Millisecond {
		t.Errorf("ErrorWindow = %v, want 150ms", spec.ErrorWindow)
	}
	if spec.RetryReads != 0 {
		t.Errorf("RetryReads = %d, want 0 for a write", spec.RetryReads)
	}
}
