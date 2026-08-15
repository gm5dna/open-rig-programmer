// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import "testing"

// TestSupportString covers String() for all five Support states.
func TestSupportString(t *testing.T) {
	cases := []struct {
		s    Support
		want string
	}{
		{Unsupported, "Unsupported"},
		{Unverified, "Unverified"},
		{Supported, "Supported"},
		{Inert, "Inert"},
		{ConsentedUnverified, "ConsentedUnverified"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.s.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestFieldSupportCanWrite is the full truth table for CanWrite() across
// all twenty-five (Read, Write) combinations. CanWrite() must depend only
// on Write, and must be true for exactly two write states (Supported:
// hardware evidence; ConsentedUnverified: the user's recorded acceptance).
//
// This is the hardware-write gate: this project hard-gates all real-radio
// writes behind hardware verification sessions, and CanWrite() returning
// true only for Supported and ConsentedUnverified is the mechanism that
// enforces it. In particular, Unverified.CanWrite() == false — a field
// that is merely documented or assumed, not yet proven on hardware and
// with no recorded consent, must never be written to a real radio — and
// Inert.CanWrite() == false too: a field the radio provably IGNORES on
// write (HW-CONFIRMED 2026-07-13, the FT-710's clarifier) must never be
// claimed writable, however freely its baseline value may be transmitted
// (see Inert's doc comment).
func TestFieldSupportCanWrite(t *testing.T) {
	states := []Support{Unsupported, Unverified, Supported, Inert, ConsentedUnverified}
	for _, read := range states {
		for _, write := range states {
			fs := FieldSupport{Read: read, Write: write}
			want := write == Supported || write == ConsentedUnverified
			t.Run(read.String()+"/"+write.String(), func(t *testing.T) {
				if got := fs.CanWrite(); got != want {
					t.Errorf("CanWrite() with Write=%v = %v, want %v", write, got, want)
				}
			})
		}
	}
}

// TestUnverifiedCanWrite_HardwareGate is an explicit, named regression test
// for the single most important property of this package: a field whose
// Write support is Unverified must never report CanWrite() == true. This is
// the hardware gate — the mechanism that stops the clone/write path from
// ever touching a real radio with a field that has not been proven safe on
// hardware.
func TestUnverifiedCanWrite_HardwareGate(t *testing.T) {
	fs := FieldSupport{Read: Supported, Write: Unverified}
	if fs.CanWrite() {
		t.Fatal("FieldSupport{Write: Unverified}.CanWrite() = true, want false: Unverified must not be writable (hardware gate)")
	}
}

// TestInertCanWrite_NotWritable is TestUnverifiedCanWrite_HardwareGate's
// sibling for the M5b-added state: a field whose Write support is Inert —
// transmitted on every write but IGNORED by the radio (HW-CONFIRMED
// 2026-07-13, the FT-710's clarifier) — must never report CanWrite() ==
// true. The changed-value blocking that makes Inert safe to transmit at
// all lives in codeplug.Diff, not here; this pins only that nothing may
// mistake Inert for genuinely writable.
func TestInertCanWrite_NotWritable(t *testing.T) {
	fs := FieldSupport{Read: Supported, Write: Inert}
	if fs.CanWrite() {
		t.Fatal("FieldSupport{Write: Inert}.CanWrite() = true, want false: an Inert field's transmitted value is ignored by the radio and must never be claimed writable")
	}
}

// TestFieldSupportCanWrite_ConsentedUnverified pins the second key to the
// hardware-write gate: a write label the user has explicitly consented to
// opens the gate, and the same state on the READ side never does —
// ConsentedUnverified is a write-side state, and CanWrite() looks at Write
// alone.
func TestFieldSupportCanWrite_ConsentedUnverified(t *testing.T) {
	if !(FieldSupport{Read: Supported, Write: ConsentedUnverified}).CanWrite() {
		t.Fatal("Write: ConsentedUnverified must satisfy CanWrite — consent unlocks the write gate")
	}
	if (FieldSupport{Read: ConsentedUnverified, Write: Unverified}).CanWrite() {
		t.Fatal("a read-side ConsentedUnverified must not unlock writes")
	}
}
