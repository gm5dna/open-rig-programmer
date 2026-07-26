// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"strings"
	"testing"
)

// Tests for the M9c-0 milestone review's findings 1, 2 and 4. Each was
// reproduced before it was fixed; each test below fails against the
// pre-fix code.

// TestClarifierPolicy_LargeStepCannotPanicTheGate covers finding 1.
//
// {StepHz: 65536, MaxAbsHz: 0} passed every clause of V10 — StepHz >= 1,
// MaxAbsHz >= 0, MaxAbsHz <= 9999, and 0 % 65536 == 0 — and then panicked
// validClarHz, which narrowed the step to int16(65536) == 0 and executed
// v % 0. Reproduced as a genuine "integer divide by zero" reachable
// through BuildMWSet and AllowedCommand, i.e. a caller-supplied config
// could crash the outbound write gate.
//
// Both halves of the fix are asserted: the constructor now refuses such a
// policy, AND validClarHz does its arithmetic in int so no surviving
// narrowing can reproduce the fault.
func TestClarifierPolicy_LargeStepCannotPanicTheGate(t *testing.T) {
	// Half one: the constructor refuses it.
	cfg := validBaselineConfig()
	cfg.Clarifier = ClarifierPolicy{StepHz: 65536, MaxAbsHz: 0}
	if _, err := NewDialect(cfg); err == nil {
		t.Error("NewDialect accepted StepHz 65536 — a step wider than the 4-digit field cannot describe any legal value, and it used to panic validClarHz")
	} else if !strings.Contains(err.Error(), "StepHz") {
		t.Errorf("error %q does not name StepHz", err)
	}

	// Half two: even a dialect built past the validator must not panic.
	// Constructed by hand precisely because NewDialect now refuses it —
	// the arithmetic must be correct for any value the TYPE permits, not
	// merely for values some other function filtered.
	d := Dialect{clar: ClarifierPolicy{StepHz: 65536, MaxAbsHz: 0}}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("validClarHz panicked: %v — the arithmetic is still narrowing to int16 somewhere", r)
		}
	}()
	// NOT PANICKING is the whole property. The verdict itself is
	// unremarkable and deliberately not over-asserted: with MaxAbsHz 0 the
	// only in-range value is 0, and 0 is a multiple of any step, so `true`
	// here is arithmetically correct. An earlier draft of this test
	// asserted `false` and failed — the test was wrong, not the code.
	_ = d.validClarHz(0)
	// Anything outside the (degenerate) range must still be refused, so
	// this cannot pass by the function accepting everything.
	if d.validClarHz(10) {
		t.Error("validClarHz(10) = true with MaxAbsHz 0, want false")
	}
}

// TestClarifierPolicy_StepBoundsAreBothEnds pins that V10 rejects at both
// ends, so the new upper bound cannot be removed silently.
func TestClarifierPolicy_StepBoundsAreBothEnds(t *testing.T) {
	for _, tc := range []struct {
		name    string
		step    int
		maxAbs  int
		wantErr bool
	}{
		{"zero step", 0, 0, true},
		{"negative step", -1, 0, true},
		{"step one", 1, 9999, false},
		{"step at the field width", 9999, 0, false},
		{"step one past the field width", 10000, 0, true},
		{"the FT-710's own policy", 10, 9990, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validBaselineConfig()
			cfg.Clarifier = ClarifierPolicy{StepHz: tc.step, MaxAbsHz: tc.maxAbs}
			_, err := NewDialect(cfg)
			if tc.wantErr && err == nil {
				t.Errorf("NewDialect accepted StepHz %d", tc.step)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("NewDialect refused StepHz %d: %v", tc.step, err)
			}
		})
	}
}

// TestMWWriteKind_RejectsTheParserOnlyPlaceholder covers finding 2.
//
// V11 delegated to validKindByte, the READ-side P7 domain, which accepts
// KindUnset ('4') because ParseMRAnswer must. memdata.go's own declaration
// says builders must never emit it. So a dialect declaring
// MWWriteKind: KindUnset built "MW005007100000+000000240000;" carrying
// P7 '4', and its own gate admitted the frame.
func TestMWWriteKind_RejectsTheParserOnlyPlaceholder(t *testing.T) {
	cfg := validBaselineConfig()
	cfg.MWWriteKind = KindUnset

	if _, err := NewDialect(cfg); err == nil {
		t.Fatal("NewDialect accepted MWWriteKind: KindUnset — the '-' placeholder is read-side only and a builder must never emit it")
	} else if !strings.Contains(err.Error(), "MWWriteKind") {
		t.Errorf("error %q does not name MWWriteKind", err)
	}
}

// TestMWWriteKind_AcceptsEveryEmittableKind is the other direction, so the
// rejection above cannot pass by refusing everything. Each of these is a
// documented P7 value a builder may legitimately write.
func TestMWWriteKind_AcceptsEveryEmittableKind(t *testing.T) {
	for _, k := range []byte{KindVFO, KindMemory, KindMemTune, KindQMB, KindPMS} {
		cfg := validBaselineConfig()
		cfg.MWWriteKind = k
		if _, err := NewDialect(cfg); err != nil {
			t.Errorf("NewDialect refused MWWriteKind %q: %v", k, err)
		}
	}
}

// TestMTClearTag_DecodingPreservesLegitimateTrailingBytes covers finding 4.
//
// Decoding trimmed EVERY trailing ClearTagByte rather than recognising the
// exact full-width clear form. That is indistinguishable from correct on
// the FT-710, whose clear byte IS a space, so the bug hid behind the very
// dialect the corpus pins. On a peer clearing with '-', the tag "CALL-"
// built, passed the gate, and came back as "CALL" — silent data loss on a
// round trip.
func TestMTClearTag_DecodingPreservesLegitimateTrailingBytes(t *testing.T) {
	peer, err := NewDialect(DialectConfig{
		CATID:       "6666",
		ModeNames:   map[Mode]string{Mode('2'): "USB"},
		Slots:       SlotSpace{MemoryLo: 1, MemoryHi: 9, NoneWire: "000"},
		MT:          MTPolicy{TagMaxBytes: 6, ClearTagByte: '-'},
		Clarifier:   ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		MWWriteKind: KindMemory,
	})
	if err != nil {
		t.Fatalf("building the '-'-clearing peer: %v", err)
	}
	slot, err := peer.MemorySlot(5)
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}

	for _, tc := range []struct {
		name string
		tag  string
		want string
	}{
		{"a tag ending in the clear byte survives", "CALL-", "CALL-"},
		{"a tag that is all clear bytes but short survives", "--", "--"},
		{"an ordinary tag survives", "CALL", "CALL"},
		{"an EMPTY tag round-trips as empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := peer.BuildMTSet(slot, false, tc.tag)
			if err != nil {
				t.Fatalf("BuildMTSet(%q): %v", tc.tag, err)
			}
			if !peer.AllowedCommand(cmd.Bytes()) {
				t.Errorf("the peer's own gate refused %q", cmd.Bytes())
			}
			_, _, got, err := peer.ParseMTAnswer(cmd.Bytes())
			if err != nil {
				t.Fatalf("ParseMTAnswer(%q): %v", cmd.Bytes(), err)
			}
			if got != tc.want {
				t.Errorf("round trip of %q gave %q, want %q — decoding is destroying legitimate bytes", tc.tag, got, tc.want)
			}
		})
	}
}

// TestMTClearTag_FT710RoundTripUnchanged pins that the fix did not move
// the FT-710, whose clear byte is a space and whose answers are padded
// with spaces — the two rules coincide there, which is exactly why the
// bug was invisible on it.
func TestMTClearTag_FT710RoundTripUnchanged(t *testing.T) {
	slot, err := FT710.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot: %v", err)
	}
	for _, tag := range []string{"", "CALLING", "CALLING FREQ", "A"} {
		cmd, err := FT710.BuildMTSet(slot, true, tag)
		if err != nil {
			t.Fatalf("BuildMTSet(%q): %v", tag, err)
		}
		_, _, got, err := FT710.ParseMTAnswer(cmd.Bytes())
		if err != nil {
			t.Fatalf("ParseMTAnswer: %v", err)
		}
		if got != tag {
			t.Errorf("FT-710 round trip of %q gave %q", tag, got)
		}
	}
	// And a space-padded answer still decodes with its padding removed.
	if _, _, got, err := FT710.ParseMTAnswer([]byte("MT0011CALL        ;")); err != nil || got != "CALL" {
		t.Errorf("space-padded answer decoded to %q (err %v), want \"CALL\"", got, err)
	}
}
