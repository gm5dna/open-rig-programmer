// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"testing"
)

// --- Command: zero value ---

func TestCommand_ZeroValueIsZero(t *testing.T) {
	var c Command
	if !c.IsZero() {
		t.Error("zero Command.IsZero() = false, want true")
	}
}

func TestCommand_ZeroValueBytesIsEmpty(t *testing.T) {
	var c Command
	if got := c.Bytes(); len(got) != 0 {
		t.Errorf("zero Command.Bytes() = %q, want empty", got)
	}
}

// --- Command: construction via newCommand ---

func TestCommand_NewCommandIsNotZero(t *testing.T) {
	c := newCommand([]byte("ID;"))
	if c.IsZero() {
		t.Error("newCommand([]byte(\"ID;\")).IsZero() = true, want false")
	}
}

func TestCommand_BytesRoundTrips(t *testing.T) {
	c := newCommand([]byte("MC099;"))
	if got := string(c.Bytes()); got != "MC099;" {
		t.Errorf("Bytes() = %q, want %q", got, "MC099;")
	}
}

// --- Command.Bytes(): defensive copy semantics (the TOCTOU fix) ---

// TestCommand_BytesReturnsFreshCopyEachCall: every call to Bytes() must
// allocate an independent copy. This is what closes the TOCTOU window a
// raw []byte builder result left open: a caller-held slice could
// previously be mutated between AllowedCommand's check and the transport's
// actual write. With Command, the transport always writes bytes nobody
// else can reach.
func TestCommand_BytesReturnsFreshCopyEachCall(t *testing.T) {
	c := newCommand([]byte("ID;"))
	b1 := c.Bytes()
	b2 := c.Bytes()

	if len(b1) > 0 && len(b2) > 0 && &b1[0] == &b2[0] {
		t.Fatal("Bytes() returned the same backing array on two separate calls")
	}

	b1[0] = 'X' // mutate one copy after the fact
	if string(c.Bytes()) != "ID;" {
		t.Errorf("mutating one Bytes() copy corrupted a later call: got %q, want %q", c.Bytes(), "ID;")
	}
	if b2[0] == 'X' {
		t.Error("mutating b1 affected b2: the two copies are not independent")
	}
}

// TestNewCommand_DoesNotCopyOnConstruction documents newCommand's actual
// contract (see its doc comment): it does NOT defensively copy frame,
// because every builder in this package already hands it a freshly
// allocated, non-aliased slice — copying again here would just be wasted
// work. The safety property callers outside the package actually rely on
// is Bytes() isolating every read from every other read and from this
// internal buffer, which the tests above cover; this test exists so a
// future change to newCommand's contract is a deliberate, visible edit
// here, not a silent behaviour change.
func TestNewCommand_DoesNotCopyOnConstruction(t *testing.T) {
	frame := []byte("AI0;")
	c := newCommand(frame)
	frame[0] = 'X' // mutate the slice that was passed to newCommand
	if got := string(c.Bytes()); got != "XI0;" {
		t.Errorf("Bytes() = %q, want %q (newCommand aliases its input by design)", got, "XI0;")
	}
}

// --- Command.String(): safe for logs ---

func TestCommand_StringIsSafeQuoted(t *testing.T) {
	c := newCommand([]byte("ID;"))
	want := `"ID;"`
	if got := c.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

// TestCommand_StringEscapesControlBytes proves String() is safe even for
// bytes this package's own charset checks would never allow through a
// builder (defence in depth: %q-quoting must not assume its input is
// already clean).
func TestCommand_StringEscapesControlBytes(t *testing.T) {
	raw := []byte("MT0011AB\x00CD;")
	c := newCommand(raw)
	want := fmt.Sprintf("%q", raw)
	if got := c.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if c.String() == string(raw) {
		t.Fatal("String() returned raw unescaped bytes")
	}
}

// --- Builder integration: every builder must return a non-zero Command ---

func TestCommand_AllFallibleBuildersReturnZeroCommandOnError(t *testing.T) {
	if _, err := BuildMRRead(Slot{}); err == nil {
		t.Fatal("BuildMRRead(Slot{}): want error")
	} else if c, _ := BuildMRRead(Slot{}); !c.IsZero() {
		t.Errorf("BuildMRRead(Slot{}) on error: Command = %v, want zero value", c)
	}

	if _, err := BuildMWSet(MemoryData{}); err == nil {
		t.Fatal("BuildMWSet(MemoryData{}): want error")
	} else if c, _ := BuildMWSet(MemoryData{}); !c.IsZero() {
		t.Errorf("BuildMWSet(MemoryData{}) on error: Command = %v, want zero value", c)
	}

	if _, err := BuildMTSet(Slot{}, true, "x"); err == nil {
		t.Fatal("BuildMTSet(Slot{}, ...): want error")
	} else if c, _ := BuildMTSet(Slot{}, true, "x"); !c.IsZero() {
		t.Errorf("BuildMTSet(Slot{}, ...) on error: Command = %v, want zero value", c)
	}

	if _, err := BuildMTRead(Slot{}); err == nil {
		t.Fatal("BuildMTRead(Slot{}): want error")
	} else if c, _ := BuildMTRead(Slot{}); !c.IsZero() {
		t.Errorf("BuildMTRead(Slot{}) on error: Command = %v, want zero value", c)
	}

	if _, err := BuildMCSet(Slot{}); err == nil {
		t.Fatal("BuildMCSet(Slot{}): want error")
	} else if c, _ := BuildMCSet(Slot{}); !c.IsZero() {
		t.Errorf("BuildMCSet(Slot{}) on error: Command = %v, want zero value", c)
	}
}

func TestCommand_InfallibleBuildersReturnNonZeroCommand(t *testing.T) {
	if BuildIDRead().IsZero() {
		t.Error("BuildIDRead().IsZero() = true, want false")
	}
	if BuildAISet(true).IsZero() {
		t.Error("BuildAISet(true).IsZero() = true, want false")
	}
	if BuildAISet(false).IsZero() {
		t.Error("BuildAISet(false).IsZero() = true, want false")
	}
	if BuildMCRead().IsZero() {
		t.Error("BuildMCRead().IsZero() = true, want false")
	}
}
