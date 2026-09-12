// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import "testing"

// lift_test.go is the Phase 2 Kenwood/Yaesu-wave lift's own runnable check
// (ponytail: the smallest one, not the fleet script — see the brief's own
// "byte-identity gate" section). It proves the RecordLen axis, the two
// ts570 axis values (P2Unused, ToneModesTwo), the TS-2000 lift's five
// field axes (P10/P12/P13, plus the Byte28Reverse/Byte41MemoryGroup
// reuses) and the ts870s second record type all round-trip, WITHOUT
// registering any of the three as a model package — that is Phase 3's
// job. None of these three synthetic layouts is a shipping row; they
// exist only to exercise the mechanism this lift added.

// ts570LikeLayout is a synthetic 28-byte row: RecordLen 28, P2Unused,
// ToneModesTwo, and no tail axes at all — the shape ts570-capability-
// matrix.md §1.4 describes, reusing Byte19Lockout and Mode's own nibbles
// unchanged, per the Phase 2 brief §K.2.
func ts570LikeLayout(t *testing.T) Layout {
	t.Helper()
	l, err := NewLayout(LayoutConfig{
		Book:         Book570,
		Model:        "LIFT-TS570",
		RecordLen:    28,
		P2:           P2Unused,
		Byte19:       Byte19Lockout,
		ToneModes:    ToneModesTwo,
		MaxEXAddress: 60,
		ModeNames:    modeNames480(),
		Slots:        []SlotRange{{Class: SlotMemory, Lo: 0, Hi: 99}},
	})
	if err != nil {
		t.Fatalf("NewLayout(ts570-like) = %v, want nil", err)
	}
	return l
}

func TestLift_TS570LikeLayout_RecordLenMovesTheTerminatorAndDropsTheTail(t *testing.T) {
	l := ts570LikeLayout(t)
	if got := l.RecordLen(); got != 28 {
		t.Fatalf("RecordLen() = %d, want 28", got)
	}

	slot, err := l.NewSlot(42, ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot: %v", err)
	}
	rec := Record{
		Slot:     slot,
		FreqHz:   14230000,
		Mode:     ModeCW,
		Byte19:   '0',
		ToneMode: ToneModeTone,
	}
	cmd, err := l.BuildMWSet(rec)
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	frame := cmd.Bytes()
	if len(frame) != 28 {
		t.Fatalf("built frame is %d bytes, want 28", len(frame))
	}
	if frame[27] != ';' {
		t.Errorf("frame[27] = %q, want ';' — RecordLen must move the terminator, not just the length check", frame[27])
	}

	answer := append([]byte(nil), frame...)
	answer[0], answer[1] = 'M', 'R'
	got, err := l.ParseMRAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMRAnswer: %v", err)
	}
	if got.Slot.Number() != 42 || got.FreqHz != 14230000 || got.Mode != ModeCW || got.ToneMode != ToneModeTone {
		t.Errorf("round trip = %+v, want slot 42 / 14230000 Hz / CW / tone", got)
	}
	// The tail is genuinely absent: a 50-byte frame is refused outright.
	if _, err := l.ParseMRAnswer(make([]byte, 50)); err == nil {
		t.Error("ParseMRAnswer accepted a 50-byte frame on a RecordLen:28 layout")
	}
}

func TestLift_TS570LikeLayout_ToneModesTwoAdmitsOnlyOffAndTone(t *testing.T) {
	l := ts570LikeLayout(t)
	if l.ValidToneMode(ToneModeCTCSS) {
		t.Error("ToneModesTwo admitted CTCSS — the TS-570's P7 prints only OFF and ON")
	}
	if !l.ValidToneMode(ToneModeOff) || !l.ValidToneMode(ToneModeTone) {
		t.Error("ToneModesTwo refused OFF or TONE, its own two printed values")
	}
}

func TestLift_TS570LikeLayout_RefusesATailAxisItHasNoByteFor(t *testing.T) {
	_, err := NewLayout(LayoutConfig{
		Book:         Book570,
		Model:        "LIFT-TS570-BAD",
		RecordLen:    28,
		P2:           P2Unused,
		Byte19:       Byte19Lockout,
		Byte28:       Byte28FixedZero, // this row has no byte 28 at all
		ToneModes:    ToneModesTwo,
		MaxEXAddress: 60,
		ModeNames:    modeNames480(),
		Slots:        []SlotRange{{Class: SlotMemory, Lo: 0, Hi: 99}},
	})
	if err == nil {
		t.Fatal("NewLayout accepted a Byte28 policy on a 28-byte row with no byte 28")
	}
}

// ts2000LikeLayout is a synthetic 50-byte row exercising the TS-2000 lift's
// five field axes: P10 (DCS code), P12 (shift status), P13 (offset
// frequency), Byte28Reverse and Byte41MemoryGroup.
func ts2000LikeLayout(t *testing.T) Layout {
	t.Helper()
	l, err := NewLayout(LayoutConfig{
		Book:         Book590,
		Model:        "LIFT-TS2000",
		RecordLen:    RecordLen,
		P2:           P2HundredsDigit,
		Byte19:       Byte19Lockout,
		Byte28:       Byte28Reverse,
		Byte3940:     Byte3940StepIndex,
		Byte41:       Byte41MemoryGroup,
		ToneModes:    ToneModesFour,
		P10:          P10DCSCode,
		P12:          P12ShiftLive,
		P13:          P13OffsetLive,
		MaxEXAddress: 99,
		ModeNames:    modeNames480(),
		Slots:        []SlotRange{{Class: SlotMemory, Lo: 0, Hi: 99}},
	})
	if err != nil {
		t.Fatalf("NewLayout(ts2000-like) = %v, want nil", err)
	}
	return l
}

func TestLift_TS2000LikeLayout_FiveFieldAxesRoundTrip(t *testing.T) {
	l := ts2000LikeLayout(t)
	slot, err := l.NewSlot(7, ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot: %v", err)
	}
	rec := Record{
		Slot:     slot,
		FreqHz:   14230000,
		Mode:     ModeCW,
		Byte19:   '1',
		ToneMode: ToneModeOff,
		DCSCode:  731,
		Byte28:   '1', // REVERSE on
		Shift:    '1', // "+"
		OffsetHz: 600000,
		Byte3940: "03",
		Byte41:   '5', // Memory Group 5
	}
	cmd, err := l.BuildMWSet(rec)
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	frame := cmd.Bytes()
	answer := append([]byte(nil), frame...)
	answer[0], answer[1] = 'M', 'R'
	got, err := l.ParseMRAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMRAnswer: %v", err)
	}
	if got.DCSCode != 731 {
		t.Errorf("DCSCode = %d, want 731", got.DCSCode)
	}
	if got.Byte28 != '1' {
		t.Errorf("Byte28 (REVERSE) = %q, want '1'", got.Byte28)
	}
	if got.Shift != '1' {
		t.Errorf("Shift = %q, want '1'", got.Shift)
	}
	if got.OffsetHz != 600000 {
		t.Errorf("OffsetHz = %d, want 600000", got.OffsetHz)
	}
	if got.Byte41 != '5' {
		t.Errorf("Byte41 (Memory Group) = %q, want '5'", got.Byte41)
	}
}

func TestLift_TS2000LikeLayout_RefusesAnOutOfDomainMemoryGroup(t *testing.T) {
	l := ts2000LikeLayout(t)
	slot, _ := l.NewSlot(7, ScanHalfNone)
	rec := Record{
		Slot: slot, FreqHz: 14230000, Mode: ModeCW, Byte19: '1', ToneMode: ToneModeOff,
		Byte28: '0', Shift: '0', Byte3940: "00",
		Byte41: 'A', // not a digit — Memory Group prints '0'-'9' only
	}
	if _, err := l.BuildMWSet(rec); err == nil {
		t.Error("BuildMWSet accepted a non-digit Memory Group byte")
	}
}

func TestLift_Layout870_RoundTrips(t *testing.T) {
	l, err := NewLayout870(Layout870Config{
		Model:        "LIFT-TS870S",
		MaxEXAddress: 60,
		ModeNames:    modeNames480(),
		ChannelLo:    0,
		ChannelHi:    99,
	})
	if err != nil {
		t.Fatalf("NewLayout870 = %v, want nil", err)
	}
	rec := Record870{
		Channel:   17,
		FreqHz:    7100000,
		Mode:      ModeLSB,
		Lockout:   '0',
		ToneMode:  ToneModeTone,
		ToneIndex: 5,
	}
	cmd, err := l.BuildMWSet(rec)
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	frame := cmd.Bytes()
	if len(frame) != 22 {
		t.Fatalf("built frame is %d bytes, want 22", len(frame))
	}
	answer := append([]byte(nil), frame...)
	answer[0], answer[1] = 'M', 'R'
	got, err := l.ParseMRAnswer(answer)
	if err != nil {
		t.Fatalf("ParseMRAnswer: %v", err)
	}
	if got.Channel != 17 || got.FreqHz != 7100000 || got.Mode != ModeLSB || got.ToneIndex != 5 {
		t.Errorf("round trip = %+v, want channel 17 / 7100000 Hz / LSB / tone 5", got)
	}
	if _, err := l.ParseMRAnswer(make([]byte, 50)); err == nil {
		t.Error("ParseMRAnswer accepted a 50-byte frame on a Layout870")
	}
}
