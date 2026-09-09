// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import (
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestCommand_SatisfiesTransportCommand is the compiler assertion written as
// a test as well, because the interface is what makes an ma.Command usable
// by the engine at all.
func TestCommand_SatisfiesTransportCommand(t *testing.T) {
	var c transport.Command = newCommand([]byte("ID;"))
	if got := string(c.Bytes()); got != "ID;" {
		t.Errorf("Bytes() = %q, want %q", got, "ID;")
	}
}

// TestCommand_IsNotAKWCommand is decision 3's reader-facing claim turned
// into a check: ma.Command and kw.Command are DIFFERENT TYPES carrying
// different families' frames, and nothing may quietly convert one to the
// other. A conversion would compile if the two ever became type aliases,
// which is exactly the "tidy-up" this test refuses.
func TestCommand_IsNotAKWCommand(t *testing.T) {
	var c any = newCommand([]byte("ID;"))
	if _, ok := c.(kw.Command); ok {
		t.Error("an ma.Command satisfied a kw.Command type assertion — the two types carry different families' frames and must not be interchangeable")
	}
}

// TestCommand_BytesIsAnIndependentCopyEveryCall pins the check-then-write
// TOCTOU closure kw.Command states: the bytes the outbound gate judged and
// the bytes the transport writes must be the same bytes, so no caller-held
// slice may alias the command's own.
func TestCommand_BytesIsAnIndependentCopyEveryCall(t *testing.T) {
	c := newCommand([]byte("MA0000;"))
	first := c.Bytes()
	first[0] = 'X'
	second := c.Bytes()
	if string(second) != "MA0000;" {
		t.Errorf("after mutating the first copy, Bytes() = %q, want %q — the copies are not independent", second, "MA0000;")
	}
	if &first[0] == &second[0] {
		t.Error("two Bytes() calls returned the same backing array")
	}
}

// TestCommand_StringIsQuoted pins the log-safety property: frame content is
// radio-adjacent and a raw control byte in a log line can spoof its
// surroundings.
func TestCommand_StringIsQuoted(t *testing.T) {
	got := newCommand([]byte("AI0;")).String()
	if !strings.HasPrefix(got, `"`) || !strings.Contains(got, "AI0;") {
		t.Errorf("String() = %s, want a %%q-quoted rendering of the frame", got)
	}
}

// TestCommand_ZeroValueIsZero pins the sentinel every fallible builder
// returns alongside its error.
func TestCommand_ZeroValueIsZero(t *testing.T) {
	if !(Command{}).IsZero() {
		t.Error("the zero Command does not report IsZero")
	}
	if newCommand([]byte("ID;")).IsZero() {
		t.Error("a built Command reports IsZero")
	}
}
