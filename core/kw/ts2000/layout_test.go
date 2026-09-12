// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000_test

import (
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ts2000"
)

// TestConformance runs core/kw's exported conformance suite over all three
// rows: zero byte difference (matrix §1-§2) means one suite pass per row is
// still three separate proofs, since kwtest.Run walks each Layout's own
// builders and gate independently.
//
// IT IS SKIPPED, AND THE REASON IS A GAP IN kwtest ITSELF, NOT IN THIS
// PACKAGE. kwtest.conformanceRecord (core/kw/kwtest/kwtest.go:957-980)
// builds its one shared fixture with NO Shift field at all — its own doc
// comment enumerates the axes it was written against (byte 19, byte 28,
// bytes 39-40, byte 41) and stops there, because P10/P12/P13 postdate it
// (lift K, this wave). DCSCode (int) and OffsetHz (uint64) default to Go's
// zero value harmlessly under P10DCSCode/P13OffsetLive — 0 is a valid DCS
// code and a valid offset — but Shift is a byte read as an ASCII digit
// (checkP12: '0'..'3'), and its Go zero value, 0x00, is not one; every
// row whose P12Policy is P12ShiftLive therefore fails kwtest.Run on its
// very first BuildMWSet, on a byte this package's own layout never
// populates.
//
// THIS PACKAGE MAY NOT FIX IT: kwtest.go is core/kw/kwtest, and this
// brief's own worktree discipline forbids touching any core/kw file but
// this package's new one. TestCrosscheck_FiveLiveAxesRoundTrip
// (crosscheck_test.go) is this package's own substitute proof that
// BuildMWSet/ParseMRAnswer round-trip a REAL Shift value (P12, alongside
// P10/P11/P13/P15) correctly; it is what stands in for kwtest.Run's walk
// until kwtest's own fixture is widened for the three axes this wave
// added — a follow-up outside this package's brief and this file's reach.
func TestConformance(t *testing.T) {
	t.Skip("kwtest.conformanceRecord has no Shift field (core/kw/kwtest/kwtest.go:957-980, predates lift K's P12ShiftLive); every P12ShiftLive row fails on BuildMWSet's zero-byte Shift, which this package cannot fix without editing core/kw/kwtest — see TestCrosscheck_FiveLiveAxesRoundTrip for this package's own round-trip proof of P10/P11/P12/P13/P15")
}

// TestLayout_IsConfiguredAndNamed is the vacuity guard the pins below rest
// on, for all three rows.
func TestLayout_IsConfiguredAndNamed(t *testing.T) {
	for _, tc := range []struct {
		l    kw.Layout
		want string
	}{
		{ts2000.TS2000, "TS-2000"},
		{ts2000.TS2000X, "TS-2000X"},
		{ts2000.TSB2000, "TS-B2000"},
	} {
		if !tc.l.Configured() {
			t.Fatalf("%s: Layout is unconfigured — it describes no radio and refuses everything", tc.want)
		}
		if got := tc.l.Model(); got != tc.want {
			t.Errorf("Model() = %q, want %q", got, tc.want)
		}
		// Book480, DELIBERATELY — see layout.go's own doc comment for why
		// this row cites the TS-480's book rather than the 590 pair's:
		// this document's own TY probe and its own "O;" cause sentence
		// (ts2000:9617-9618) both match Book480's, not Book590's.
		if got := tc.l.Book(); got != kw.Book480 {
			t.Errorf("Book() = %v, want %v (layout.go's own doc comment)", got, kw.Book480)
		}
	}
}

// wantModeNames is the MD legend all three rows share (ts2000:10611-10619).
func wantModeNames() map[kw.Mode]string {
	return map[kw.Mode]string{
		kw.ModeLSB: "LSB", kw.ModeUSB: "USB", kw.ModeCW: "CW", kw.ModeFM: "FM",
		kw.ModeAM: "AM", kw.ModeFSK: "FSK", kw.ModeCWR: "CW-R", kw.ModeFSKR: "FSK-R",
	}
}

// wantSlots is the slot space all three rows share: 290 ordinary memories,
// then ten Program Scan edge pairs (ts2000:5614, ts2000:10726-10727,
// ts2000:10797-10798) — see layout.go's own doc comment for why this is NOT
// the matrix's own summary line ("MEM 000-299") taken literally.
func wantSlots() []kw.SlotRange {
	return []kw.SlotRange{
		{Class: kw.SlotMemory, Lo: 0, Hi: 289},
		{Class: kw.SlotScan, Lo: 290, Hi: 299},
	}
}

// TestLayout_EveryAxisByValue pins all thirteen axes outright, on every row,
// each against the line this document prints it on (matrix §2).
func TestLayout_EveryAxisByValue(t *testing.T) {
	for name, l := range map[string]kw.Layout{
		"TS-2000": ts2000.TS2000, "TS-2000X": ts2000.TS2000X, "TS-B2000": ts2000.TSB2000,
	} {
		t.Run(name, func(t *testing.T) {
			if got := l.P2Policy(); got != kw.P2HundredsDigit {
				t.Errorf("P2 policy = %v, want %v — MC's own bank digit (ts2000:10589-10600)", got, kw.P2HundredsDigit)
			}
			if got := l.Byte19(); got != kw.Byte19Lockout {
				t.Errorf("byte 19 meaning = %v, want %v (ts2000:10704)", got, kw.Byte19Lockout)
			}
			if got := l.Byte28(); got != kw.Byte28Reverse {
				t.Errorf("byte 28 policy = %v, want %v — P11 REVERSE status (ts2000:10713-10714)", got, kw.Byte28Reverse)
			}
			if got := l.Byte3940(); got != kw.Byte3940StepIndex {
				t.Errorf("bytes 39-40 meaning = %v, want %v (ts2000:10720-10722)", got, kw.Byte3940StepIndex)
			}
			if got := l.Byte41(); got != kw.Byte41MemoryGroup {
				t.Errorf("byte 41 meaning = %v, want %v — P15 Memory Group (ts2000:10723-10724)", got, kw.Byte41MemoryGroup)
			}
			if got := l.ToneModes(); got != kw.ToneModesFour {
				t.Errorf("tone-mode value set = %v, want %v — width/count match the 590 pair; the 4th value means DCS here, not Cross Tone (matrix §6 item 1)", got, kw.ToneModesFour)
			}
			if got := l.RecordLen(); got != kw.RecordLen {
				t.Errorf("record length = %d, want %d (matrix §2, width delta 0)", got, kw.RecordLen)
			}
			if got := l.P10Policy(); got != kw.P10DCSCode {
				t.Errorf("P10 policy = %v, want %v (ts2000:10711-10712)", got, kw.P10DCSCode)
			}
			if got := l.P12Policy(); got != kw.P12ShiftLive {
				t.Errorf("P12 policy = %v, want %v (ts2000:10715-10716)", got, kw.P12ShiftLive)
			}
			if got := l.P13Policy(); got != kw.P13OffsetLive {
				t.Errorf("P13 policy = %v, want %v (ts2000:10717-10718)", got, kw.P13OffsetLive)
			}
			if got := l.MaxEXAddress(); got != 62 {
				t.Errorf("MaxEXAddress = %d, want 62 — the highest menu number this document's Appendix prints (ts2000:10234-10246)", got)
			}
			if got := l.ModeNames(); !reflect.DeepEqual(got, wantModeNames()) {
				t.Errorf("ModeNames = %v, want %v", got, wantModeNames())
			}
			if got := l.Slots(); !reflect.DeepEqual(got, wantSlots()) {
				t.Errorf("Slots = %v, want %v", got, wantSlots())
			}
			if got := l.PrintedFixed(); len(got) != 0 {
				t.Errorf("PrintedFixed = %v, want empty — every one of the sixteen parameter fields carries a live meaning on this row", got)
			}
		})
	}
}

// TestLayout_ThreeRowsAreByteIdenticalExceptModel pins the matrix's central
// claim directly: nothing distinguishes the three rows' Layouts but the
// Model string each names in its own refusals.
func TestLayout_ThreeRowsAreByteIdenticalExceptModel(t *testing.T) {
	strip := func(l kw.Layout) kw.LayoutConfig {
		return kw.LayoutConfig{
			Book: l.Book(), RecordLen: l.RecordLen(),
			P2: l.P2Policy(), Byte19: l.Byte19(), Byte28: l.Byte28(),
			Byte3940: l.Byte3940(), Byte41: l.Byte41(), ToneModes: l.ToneModes(),
			P10: l.P10Policy(), P12: l.P12Policy(), P13: l.P13Policy(),
			MaxEXAddress: l.MaxEXAddress(), ModeNames: l.ModeNames(),
			Slots: l.Slots(), PrintedFixed: l.PrintedFixed(),
		}
	}
	base := strip(ts2000.TS2000)
	for name, l := range map[string]kw.Layout{"TS-2000X": ts2000.TS2000X, "TS-B2000": ts2000.TSB2000} {
		if got := strip(l); !reflect.DeepEqual(got, base) {
			t.Errorf("%s's Layout (Model aside) = %+v, want %+v (matrix §1-§2: zero byte difference)", name, got, base)
		}
	}
}
