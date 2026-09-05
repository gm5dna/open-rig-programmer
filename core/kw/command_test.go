// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// THERE IS NO TestCommand_SatisfiesTheNeutralSeam: that half of the contract
// is asserted by the compiler, in framing.go's var block, so there is nothing
// for a test to run. The two tests below are the behavioural halves a type
// assertion cannot see.

// TestCommand_BytesReturnsAFreshCopyEveryCall pins the TOCTOU closure that is
// the type's whole reason to exist: what the gate judges and what the port
// writes must be bytes nobody else can reach.
func TestCommand_BytesReturnsAFreshCopyEveryCall(t *testing.T) {
	c := newCommand([]byte("AI0;"))
	a := c.Bytes()
	b := c.Bytes()
	if string(a) != "AI0;" || string(b) != "AI0;" {
		t.Fatalf("Bytes() = %q / %q, want \"AI0;\" both times", a, b)
	}
	a[0] = 'X'
	if string(c.Bytes()) != "AI0;" {
		t.Errorf("mutating one copy changed the command: %q", c.Bytes())
	}
	if string(b) != "AI0;" {
		t.Errorf("mutating one copy changed another: %q", b)
	}
}

// TestCommand_StringIsQuoted pins the diagnostic rendering: frame content is
// radio-supplied, so it is %q-quoted before it can reach a log line.
func TestCommand_StringIsQuoted(t *testing.T) {
	c := newCommand([]byte("MC\x00 07;"))
	got := c.String()
	if !strings.HasPrefix(got, `"`) || strings.ContainsRune(got, 0) {
		t.Errorf("String() = %s, want a %%q-quoted rendering with no raw control bytes", got)
	}
}

// TestCommand_ZeroValueIsRecognisable pins the sentinel fallible builders
// return alongside their error.
func TestCommand_ZeroValueIsRecognisable(t *testing.T) {
	var zero Command
	if !zero.IsZero() {
		t.Error("the zero Command reports IsZero() = false")
	}
	if newCommand([]byte("ID;")).IsZero() {
		t.Error("a built Command reports IsZero() = true")
	}
	var _ transport.Command = Command{}
}
