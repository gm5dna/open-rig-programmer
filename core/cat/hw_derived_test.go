// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "testing"

// HW-DERIVED 2026-07-13, UK FT-710, firmware V01-12 — see
// docs/hardware-notes.md.
//
// These vectors were captured live against Stuart's physical FT-710
// (controller-driven, read-only session, M5a) and are ADDITIVE to the
// manual-derived G1-G12 golden vectors used throughout this package's
// other tests: they prove the codec's parse behaviour against REAL wire
// bytes, not just the reference manual's documented examples. Nothing
// below replaces an existing golden vector or weakens an existing
// assertion.
//
// Redaction (docs/fixtures.md, binding): the real radio's M-06 memory
// tag was a genuine 4-character amateur-radio callsign, space-padded by
// the radio itself to the full 12-byte tag field. The MT vector below
// substitutes the placeholder "MYCALL" (6 characters), padded the same
// way to 12 bytes, preserving the frame's exact byte SHAPE (display flag
// + 12-byte space-padded tag) while removing the real callsign
// entirely. M-06's frequency/mode/shift are NOT personal data (a memory
// channel's radio parameters reveal nothing about the operator) and are
// reproduced exactly as read.

// TestParseMRAnswer_HWDerived_M06 pins the live MR006 exchange: TX
// "MR006;" -> RX "MR006029620000+000000411002;" (11ms, one of 20
// back-to-back zero-settle reads — see docs/hardware-notes.md
// §Timing). Confirms field-for-field: P6=4 FM, P7=1 memory-kind, P8=1
// CTCSS ENC/DEC state, P10=2 minus shift — AND, byte-level, that P9
// (bytes 25-26, 1-indexed) is the fixed "00" DESPITE a CTCSS tone
// (146.2 Hz) being demonstrably SET and ACTIVE on the radio at capture
// time: this REFUTES the Hamlib live-tone-index theory. MemoryData
// carries no field for P9 (cat.ParseMRAnswer already validates it is
// the constant "00"), so there is nothing slot-specific to preserve —
// see core/driver/ft710/caps.go's FieldCTCSSTone doc comment for the
// capability-level consequence.
func TestParseMRAnswer_HWDerived_M06(t *testing.T) {
	frame := "MR006029620000+000000411002;"
	got, err := FT710.ParseMRAnswer([]byte(frame))
	if err != nil {
		t.Fatalf("ParseMRAnswer(%q): unexpected error: %v", frame, err)
	}
	wantSlot, _ := FT710.MemorySlot(6)
	want := MemoryData{
		Slot:   wantSlot,
		FreqHz: 29_620_000,
		ClarHz: 0,
		RxClar: false,
		TxClar: false,
		Mode:   ModeFM,
		Kind:   KindMemory,
		CTCSS:  CTCSSEncDec,
		Shift:  ShiftMinus,
	}
	if got != want {
		t.Errorf("ParseMRAnswer(%q) = %+v, want %+v", frame, got, want)
	}
	// Bytes 25-26 (1-indexed P9), 0-indexed [24:26]: fixed "00" — explicit
	// byte check since MemoryData has no field to carry it through.
	if got, want := frame[24:26], "00"; got != want {
		t.Fatalf("frame bytes 25-26 (P9) = %q, want %q (fixed, even with a live CTCSS tone set)", got, want)
	}
}

// TestParseMTAnswer_HWDerived_M06_ShortForm pins the live MT006 exchange
// (tag redacted per docs/fixtures.md — see this file's package comment):
// TX "MT006;" -> RX "MT0061MYCALL      ;". HW-CONFIRMED: the radio uses
// the Set-shaped SHORT FORM for its MT answer (display flag + 0-12 byte
// tag), not a Hamlib-style long combined form, and pads a short tag with
// trailing spaces to the full 12-byte field on read. The WIRE FRAME
// below is unchanged (still the real 12-byte space-padded bytes the
// radio sent) — only the EXPECTED PARSE RESULT changed, Fix (tag
// normalisation): ParseMTAnswer now trims the trailing padding, so the
// model-level tag is "MYCALL", not "MYCALL      " (see mt.go's doc
// comment and mt_test.go's TestParseMTAnswer_TrimsTrailingSpaces).
func TestParseMTAnswer_HWDerived_M06_ShortForm(t *testing.T) {
	frame := "MT0061MYCALL      ;" // redacted: real tag was a 4-char callsign + 8 spaces
	slot, display, tag, err := FT710.ParseMTAnswer([]byte(frame))
	if err != nil {
		t.Fatalf("ParseMTAnswer(%q): unexpected error: %v", frame, err)
	}
	wantSlot, _ := FT710.MemorySlot(6)
	wantTag := "MYCALL"
	if slot != wantSlot || !display || tag != wantTag {
		t.Errorf("ParseMTAnswer(%q) = (%q,%v,%q), want (%q,true,%q)", frame, slot.Wire(), display, tag, wantSlot.Wire(), wantTag)
	}
	if len(tag) != 6 {
		t.Fatalf("tag length = %d, want 6 (radio-side space padding TRIMMED at parse — the wire frame itself still carries the full 12 padded bytes)", len(tag))
	}
}

// TestParseMCAnswer_HWDerived_M06 pins the live MC query while the radio
// sat on M-06: TX "MC;" -> RX "MC006;" — a 3-digit memory slot, the same
// shape ParseMCAnswer already required (see mc.go's doc comment on what
// remains open about MC's "000" answer case).
func TestParseMCAnswer_HWDerived_M06(t *testing.T) {
	frame := "MC006;"
	got, err := FT710.ParseMCAnswer([]byte(frame))
	if err != nil {
		t.Fatalf("ParseMCAnswer(%q): unexpected error: %v", frame, err)
	}
	want, _ := FT710.MemorySlot(6)
	if got != want {
		t.Errorf("ParseMCAnswer(%q) = %q, want %q", frame, got.Wire(), want.Wire())
	}
}

// TestParseAIAnswer_HWDerived pins the live "AI;" -> "AI0;" query,
// matching G2's manual-derived form exactly — recorded here as the
// hardware-derived confirmation that the radio's AI-off answer is
// byte-identical to the documented one, not a new parse path.
func TestParseAIAnswer_HWDerived(t *testing.T) {
	on, err := FT710.ParseAIAnswer([]byte("AI0;"))
	if err != nil {
		t.Fatalf("ParseAIAnswer: unexpected error: %v", err)
	}
	if on {
		t.Error(`ParseAIAnswer("AI0;") = true, want false`)
	}
}

// TestIsRejection_HWDerived_NAKToken pins only the literal "?;" NAK
// token this package's IsRejection recognises — the live session's four
// rejection exchanges (docs/hardware-notes.md §Empty/out-of-range
// slots: "MR010;", "MT010;" both with and without a preceding "MR010;",
// and "MR100;") all produced this identical frame, matching the
// manual's "one unattributed NAK" claim (golden vector G12), but this
// test does not itself invoke or distinguish those four command-specific
// exchanges. For that command-level coverage, see
// internal/fakeradio/fakeradio_test.go's TestGolden_MR_EmptySlot,
// TestGolden_MT_ReadOnNeverTouchedSlot,
// TestGolden_MT_ReadOnNeverTouchedSlot_WithoutPrecedingMR, and
// TestRejection_MalformedSlot (which exercises "MR100;" directly).
func TestIsRejection_HWDerived_NAKToken(t *testing.T) {
	if !IsRejection([]byte("?;")) {
		t.Error(`IsRejection("?;") = false, want true`)
	}
}
