// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
	"strings"
	"testing"
)

// --- BuildEXRead ---

// TestBuildEXRead_WireShape hand-derives two read frames from the manual's
// EX grammar block (manual extract line ~629: "E X P1 P1 P2 P2 P3 P3 ;", 9
// bytes) and Table 2 spot checks already pinned in exinventory_test.go:
// (01,01,01) AF TREBLE GAIN (manual line ~646) and (06,05,18) RPTT SELECT
// (manual line ~915).
func TestBuildEXRead_WireShape(t *testing.T) {
	tests := []struct {
		name string
		addr EXAddress
		want string
	}{
		{"(01,01,01) AF TREBLE GAIN", EXAddress{P1: 1, P2: 1, P3: 1}, "EX010101;"},
		{"(06,05,18) RPTT SELECT", EXAddress{P1: 6, P2: 5, P3: 18}, "EX060518;"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := FT710.BuildEXRead(tc.addr)
			if err != nil {
				t.Fatalf("BuildEXRead(%v): unexpected error: %v", tc.addr, err)
			}
			got := cmd.Bytes()
			if string(got) != tc.want {
				t.Errorf("BuildEXRead(%v) = %q, want %q", tc.addr, got, tc.want)
			}
			if len(got) != FT710.exReadLen() {
				t.Errorf("len(BuildEXRead(%v)) = %d, want %d", tc.addr, len(got), FT710.exReadLen())
			}
		})
	}
}

// TestBuildEXRead_RejectsNonMembers covers the zero value, the P1==05
// grammar/Table-2 anomaly address (M8c put two such addresses to a real
// radio and both were rejected — dialect.go's Dialect.KnownEXAddress doc
// comment), an
// address one past the SSB subgroup's last item ((01,01,19) CW AUTO MODE
// is the last P1=1,P2=1 item per exinventory_gen.go), and a wildly
// out-of-range triple. Membership is the only check — never a numeric
// range.
func TestBuildEXRead_RejectsNonMembers(t *testing.T) {
	tests := []struct {
		name string
		addr EXAddress
	}{
		{"zero value", EXAddress{}},
		{"P1==05 anomaly address (grammar says 05, Table 2 has none)", EXAddress{P1: 5, P2: 1, P3: 1}},
		{"one past SSB's last item (01,01,19)", EXAddress{P1: 1, P2: 1, P3: 20}},
		{"wildly out of range", EXAddress{P1: 99, P2: 99, P3: 99}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := FT710.BuildEXRead(tc.addr)
			if err == nil {
				t.Fatalf("BuildEXRead(%v): want error, got %q", tc.addr, cmd.Bytes())
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Errorf("BuildEXRead(%v): error is %T, want *ParseError", tc.addr, err)
			}
			if !cmd.IsZero() {
				t.Errorf("BuildEXRead(%v): returned error but non-zero Command %q", tc.addr, cmd.Bytes())
			}
		})
	}
}

// TestBuildEXRead_PropertyAll296 builds a read frame for every one of the
// 296 inventory addresses: every build must succeed, every frame must be
// exactly FT710.exReadLen() bytes, and all 296 frames must be pairwise distinct
// (Wire() is injective over the inventory, exinventory_test.go's
// TestEXInventory_NoDuplicatesSortedAndWireStable).
func TestBuildEXRead_PropertyAll296(t *testing.T) {
	items := FT710.EXItems()
	seen := make(map[string]bool, len(items))
	for _, it := range items {
		cmd, err := FT710.BuildEXRead(it.Addr)
		if err != nil {
			t.Fatalf("BuildEXRead(%v): unexpected error: %v", it.Addr, err)
		}
		got := cmd.Bytes()
		if len(got) != FT710.exReadLen() {
			t.Errorf("BuildEXRead(%v): len = %d, want %d", it.Addr, len(got), FT710.exReadLen())
		}
		frame := string(got)
		if seen[frame] {
			t.Errorf("BuildEXRead(%v) produced duplicate frame %q", it.Addr, frame)
		}
		seen[frame] = true
	}
	if len(seen) != 296 {
		t.Errorf("got %d distinct frames, want 296", len(seen))
	}
}

// --- ParseEXAnswer ---

// TestParseEXAnswer_Vectors hand-derives four answer frames of increasing
// P4 width from Table 2 (transcribed into exinventory_gen.go, itself
// derived from the manual): a 3-byte signed numeric field
// ((01,01,01) AF TREBLE GAIN, range "-20 - +10", manual line ~646), a
// 1-byte numeric field ((03,01,05) CAT-1 RATE, range "0-4", manual line
// ~801), a 4-byte zero-padded numeric field ((01,01,04) AGC FAST DELAY,
// range "0020-4000", manual line ~649), and a 12-byte Text field
// ((04,01,01) MY CALL, manual line ~879) padded with trailing spaces the
// way BuildMTSet's tag convention already does elsewhere in this package —
// which the M8c sweeps bore out on one radio: all six text items answered
// exactly 12 bytes, right-space-padded (docs/hardware-notes.md for the
// session's full scope). See ParseEXAnswer's
// doc comment for the verbatim-P4 policy this test also exercises.
//
// The text vector's value is deliberately synthetic. It used to be the
// operator's own callsign, which this repository does not commit now that
// it is heading for public release; the property under test is the 12-byte
// right-padded shape, and any six-character value exercises it identically.
func TestParseEXAnswer_Vectors(t *testing.T) {
	tests := []struct {
		name     string
		frame    string
		wantAddr EXAddress
		wantRaw  string
	}{
		{"3-byte signed: AF TREBLE GAIN -20", "EX010101-20;", EXAddress{1, 1, 1}, "-20"},
		{"1-byte: CAT-1 RATE 3", "EX0301053;", EXAddress{3, 1, 5}, "3"},
		{"4-byte zero-padded: AGC FAST DELAY 0020", "EX0101040020;", EXAddress{1, 1, 4}, "0020"},
		{"12-byte text: MY CALL TESTER", "EX040101TESTER      ;", EXAddress{4, 1, 1}, "TESTER      "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "12-byte text: MY CALL TESTER" && len(tc.frame) != 21 {
				t.Fatalf("test fixture bug: frame %q is %d bytes, want 21", tc.frame, len(tc.frame))
			}
			addr, raw, err := FT710.ParseEXAnswer([]byte(tc.frame))
			if err != nil {
				t.Fatalf("ParseEXAnswer(%q): unexpected error: %v", tc.frame, err)
			}
			if addr != tc.wantAddr {
				t.Errorf("ParseEXAnswer(%q) addr = %v, want %v", tc.frame, addr, tc.wantAddr)
			}
			if raw != tc.wantRaw {
				t.Errorf("ParseEXAnswer(%q) raw = %q, want %q", tc.frame, raw, tc.wantRaw)
			}
		})
	}
}

// TestParseEXAnswer_RejectTable exercises the reject/shape-boundary table
// from the brief: an exactly-9-byte, read-shaped frame (P4 would be 0
// bytes, one below exAnswerMinLen), a 22-byte frame (P4 would be 13 bytes,
// one above exP4MaxBytes), missing prefix, missing terminator, non-digit
// address bytes, a non-member address at two different P4 widths (the
// P1==05 anomaly, with M8c evidence — dialect.go's Dialect.KnownEXAddress
// doc comment), empty and nil input, and the radio's generic NAK frame "?;".
func TestParseEXAnswer_RejectTable(t *testing.T) {
	tests := []struct {
		name  string
		frame []byte
	}{
		{"9-byte read-shaped frame (empty P4)", []byte("EX010101;")},
		{"22-byte frame (13-byte P4)", []byte("EX010101ABCDEFGHIJKLM;")},
		{"missing prefix", []byte("XX0101013;")},
		{"missing terminator", []byte("EX0101013X")},
		{"non-digit address bytes", []byte("EX0A01013;")},
		{"non-member address (P1=05), 1-byte P4 'X'", []byte("EX050101X;")},
		{"non-member address (P1=05), 1-byte P4 '3'", []byte("EX0501013;")},
		{"empty input", []byte("")},
		{"nil input", nil},
		{"generic NAK", []byte("?;")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr, raw, err := FT710.ParseEXAnswer(tc.frame)
			if err == nil {
				t.Fatalf("ParseEXAnswer(%q): want error, got addr=%v raw=%q", tc.frame, addr, raw)
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Errorf("ParseEXAnswer(%q): error is %T, want *ParseError", tc.frame, err)
			}
		})
	}
}

// TestParseEXAnswer_RawP4Verbatim proves the raw P4 field survives
// byte-for-byte with no trimming, sign stripping, or other normalisation:
// leading zeros, sign characters, and interior spaces all round-trip
// exactly. See ParseEXAnswer's doc comment: the M8c sweeps gave
// verbatim-return read-direction support on one radio — re-validating
// width here would have rejected that radio's own honest TONE FREQ answer,
// which is three bytes where the manual prints two.
func TestParseEXAnswer_RawP4Verbatim(t *testing.T) {
	tests := []struct {
		name    string
		frame   string
		wantRaw string
	}{
		{"leading zeros", "EX0101040007;", "0007"},
		{"sign character preserved", "EX010101-05;", "-05"},
		{"interior space preserved (text field)", "EX040101TES TER     ;", "TES TER     "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, raw, err := FT710.ParseEXAnswer([]byte(tc.frame))
			if err != nil {
				t.Fatalf("ParseEXAnswer(%q): unexpected error: %v", tc.frame, err)
			}
			if raw != tc.wantRaw {
				t.Errorf("ParseEXAnswer(%q) raw = %q, want %q", tc.frame, raw, tc.wantRaw)
			}
		})
	}
}

// TestParseEXAnswer_RoundTripAll296 builds a synthetic answer frame for
// every inventory item ("EX" + Wire() + a P4 body of exactly the item's
// Digits width, all zero bytes + ";") and checks it parses back to the same
// address and the exact raw P4 string, for all 296 items (Digits is
// already 12 for the six Text items, exinventory.go's EXItem.Digits doc
// comment).
func TestParseEXAnswer_RoundTripAll296(t *testing.T) {
	items := FT710.EXItems()
	for _, it := range items {
		rawWant := strings.Repeat("0", it.Digits)
		frame := "EX" + FT710.EXWire(it.Addr) + rawWant + ";"
		addr, raw, err := FT710.ParseEXAnswer([]byte(frame))
		if err != nil {
			t.Fatalf("ParseEXAnswer(%q) for %v: unexpected error: %v", frame, it.Addr, err)
		}
		if addr != it.Addr {
			t.Errorf("ParseEXAnswer(%q) addr = %v, want %v", frame, addr, it.Addr)
		}
		if raw != rawWant {
			t.Errorf("ParseEXAnswer(%q) raw = %q, want %q", frame, raw, rawWant)
		}
	}
	if len(items) != 296 {
		t.Fatalf("test fixture bug: EXItems() returned %d items, want 296", len(items))
	}
}

// FuzzParseEXAnswer requires ParseEXAnswer never panics, and that on
// success: the returned address is a known inventory member, the raw P4
// string is 1-12 bytes, and reassembling "EX"+FT710.EXWire(addr)+raw+";"
// reproduces the input byte-for-byte (proves the split points are exactly
// right and nothing was silently dropped, trimmed, or added).
func FuzzParseEXAnswer(f *testing.F) {
	seeds := [][]byte{
		nil,
		[]byte(""),
		[]byte("?;"),
		[]byte("EX010101-20;"),           // AF TREBLE GAIN
		[]byte("EX0301053;"),             // CAT-1 RATE
		[]byte("EX0101040020;"),          // AGC FAST DELAY
		[]byte("EX040101TESTER      ;"),  // MY CALL, 21 bytes
		[]byte("EX010101;"),              // 9-byte read-shaped, empty P4
		[]byte("EX010101ABCDEFGHIJKLM;"), // 22-byte, 13-byte P4
		[]byte("XX0101013;"),
		[]byte("EX0101013X"),
		[]byte("EX0A01013;"),
		[]byte("EX050101X;"),
		[]byte("EX0501013;"),
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, frame []byte) {
		addr, raw, err := FT710.ParseEXAnswer(frame)
		if err != nil {
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("ParseEXAnswer(%q) returned non-ParseError: %T (%v)", frame, err, err)
			}
			return
		}
		if !FT710.KnownEXAddress(addr) {
			t.Fatalf("ParseEXAnswer(%q) succeeded with unknown address %v", frame, addr)
		}
		if len(raw) < 1 || len(raw) > FT710.exP4MaxBytes() {
			t.Fatalf("ParseEXAnswer(%q) succeeded with raw %q of length %d, want 1-%d", frame, raw, len(raw), FT710.exP4MaxBytes())
		}
		reconstructed := "EX" + FT710.EXWire(addr) + raw + ";"
		if reconstructed != string(frame) {
			t.Fatalf("ParseEXAnswer(%q): reconstruction %q != input", frame, reconstructed)
		}
	})
}
