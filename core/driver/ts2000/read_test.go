// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000

import (
	"context"
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// TestParseSlotID pins the three-digit, optional "L"/"U" syntax — this
// row's own MC prints a real hundreds digit (ts2000:10589-10600), so the
// width matches the 590 pair's rather than the TS-480's two digits.
func TestParseSlotID(t *testing.T) {
	for _, tc := range []struct {
		id       string
		wantN    int
		wantHalf kw.ScanHalf
		ok       bool
	}{
		{"000", 0, kw.ScanHalfNone, true},
		{"072", 72, kw.ScanHalfNone, true},
		{"289", 289, kw.ScanHalfNone, true},
		{"290L", 290, kw.ScanLower, true},
		{"299U", 299, kw.ScanUpper, true},
		{"", 0, 0, false},
		{"42", 0, 0, false},
		{"0042", 0, 0, false},
		{"29L", 0, 0, false},
		{"29X", 0, 0, false},
	} {
		n, half, err := parseSlotID(tc.id)
		if tc.ok {
			if err != nil {
				t.Errorf("parseSlotID(%q): %v", tc.id, err)
			} else if n != tc.wantN || half != tc.wantHalf {
				t.Errorf("parseSlotID(%q) = (%d, %v), want (%d, %v)", tc.id, n, half, tc.wantN, tc.wantHalf)
			}
			continue
		}
		if err == nil {
			t.Errorf("parseSlotID(%q) = (%d, %v), want a refusal", tc.id, n, half)
		}
	}
}

// memoryFrame builds a valid 50-byte MR answer frame through the codec's
// own BuildMWSet and relabels it "MR" — the core/driver/ts590 test-fixture
// shape, used here to script the responding radio's ANSWERS rather than to
// exercise this package's own write path (write_test.go does that).
func memoryFrame(t *testing.T, l kw.Layout, rec kw.Record) string {
	t.Helper()
	cmd, err := l.BuildMWSet(rec)
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	f := cmd.Bytes()
	f[0], f[1] = 'M', 'R'
	return string(f)
}

// TestReadChannel_PopulatedMemoryChannel round-trips a real channel through
// the scripted radio, exercising the five live axes' driver-level mapping
// (P12 Shift -> FieldDuplex, P13 Offset -> FieldOffset) alongside the
// ordinary fields.
func TestReadChannel_PopulatedMemoryChannel(t *testing.T) {
	l := paramsTS2000.layout
	slot, err := l.NewSlot(72, kw.ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot(72): %v", err)
	}
	rec := kw.Record{
		Slot: slot, FreqHz: 145500000, Mode: kw.ModeFM,
		Byte19: '1', ToneMode: kw.ToneModeCTCSS, ToneIndex: 8, CTCSSIndex: 8,
		DCSCode: 23, Byte28: '0', Byte3940: "00", Byte41: '3',
		Shift: '1', OffsetHz: 600000, Name: "REPEATER",
	}
	frame := memoryFrame(t, l, rec)

	sess, _ := openSession(t, NewTS2000, "019", radioImage{
		mrAnswers: map[string]string{"0072": frame},
	}, WithSimulatedProfile())

	ch, err := sess.ReadChannel(context.Background(), "072")
	if err != nil {
		t.Fatalf("ReadChannel(072): %v", err)
	}
	if ch.Data == nil {
		t.Fatal("ReadChannel(072) returned an empty channel")
	}
	if ch.Data.FreqHz != 145500000 {
		t.Errorf("FreqHz = %d, want 145500000", ch.Data.FreqHz)
	}
	if ch.Data.Mode != "FM" {
		t.Errorf("Mode = %q, want FM", ch.Data.Mode)
	}
	if !ch.Data.ScanSkip.Value {
		t.Error("ScanSkip = false, want true (byte 19 = '1', ts2000:10704)")
	}
	if ch.Data.Duplex.State != codeplug.Known || ch.Data.Duplex.Value != "1" {
		t.Errorf("Duplex = %+v, want Known \"1\" (P12 '+', ts2000:10935-10938)", ch.Data.Duplex)
	}
	if ch.Data.OffsetHz.State != codeplug.Known || ch.Data.OffsetHz.Value != 600000 {
		t.Errorf("OffsetHz = %+v, want Known 600000", ch.Data.OffsetHz)
	}
	if ch.Data.ToneMode.Value != "CTCSS" {
		t.Errorf("ToneMode = %q, want CTCSS", ch.Data.ToneMode.Value)
	}
	if ch.Data.ToneTx.State != codeplug.Known || ch.Data.ToneTx.Value != 915 {
		t.Errorf("ToneTx = %+v, want Known 91.5 Hz (index 8 -> chart's 9th entry, ts2000:3837-3847)", ch.Data.ToneTx)
	}
	if ch.Data.ToneRx.State != codeplug.Known || ch.Data.ToneRx.Value != 915 {
		t.Errorf("ToneRx = %+v, want Known 91.5 Hz", ch.Data.ToneRx)
	}
	// DCS (P10) and Memory Group (P15) are parsed and never published —
	// see channelData's own doc comment.
	if ch.Data.DTCSCode.State != codeplug.Unavailable {
		t.Errorf("DTCSCode = %+v, want Unavailable (matrix §6 item 4)", ch.Data.DTCSCode)
	}
}

// TestReadChannel_EmptyChannel pins that a well-formed all-zero record
// answers as an empty channel, not an error — core/kw's own structural
// rule (parse.go), applied here on the same unlifted-assumption footing
// core/driver/ts480's own A4 states (matrix §5: no empty-channel sentence
// is printed for this document, so this remains recorded rather than
// confirmed against hardware).
func TestReadChannel_EmptyChannel(t *testing.T) {
	l := paramsTS2000.layout
	slot, err := l.NewSlot(5, kw.ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot(5): %v", err)
	}
	frame := make([]byte, kw.RecordLen)
	for i := range frame {
		frame[i] = '0'
	}
	frame[0], frame[1] = 'M', 'R'
	copy(frame[3:6], "005")
	copy(frame[41:49], "        ")
	frame[49] = ';'
	// Cross-check: the frame this test hand-builds must be one the codec's
	// own gate accepts as an answer at all.
	if _, err := l.ParseMRAnswer(frame); err != nil {
		t.Fatalf("the hand-built empty frame was refused before ReadChannel even ran: %v", err)
	}
	_ = slot

	sess, _ := openSession(t, NewTS2000, "019", radioImage{
		mrAnswers: map[string]string{"0005": string(frame)},
	}, WithSimulatedProfile())

	ch, err := sess.ReadChannel(context.Background(), "005")
	if err != nil {
		t.Fatalf("ReadChannel(005): %v", err)
	}
	if ch.Data != nil {
		t.Errorf("ReadChannel(005) = %+v, want an empty channel (Data == nil)", ch.Data)
	}
}

// TestReadChannel_UnknownSlot pins that a slot outside this row's
// published banks is refused before any frame is sent.
func TestReadChannel_UnknownSlot(t *testing.T) {
	sess, p := openSession(t, NewTS2000, "019", radioImage{}, WithSimulatedProfile())
	_, err := sess.ReadChannel(context.Background(), "999")
	var use *UnknownSlotError
	if !errors.As(err, &use) {
		t.Fatalf("ReadChannel(999): %v, want *UnknownSlotError", err)
	}
	for _, f := range p.Transcript() {
		if f != "AI0;" && f != "ID;" && f != "TY;" {
			t.Errorf("a frame was sent for an unknown slot: %q", f)
		}
	}
}

// TestReadChannel_ScanHalf pins that a Program Scan channel's slot
// identifier round-trips its "L"/"U" suffix.
func TestReadChannel_ScanHalf(t *testing.T) {
	l := paramsTS2000.layout
	slot, err := l.NewSlot(290, kw.ScanLower)
	if err != nil {
		t.Fatalf("NewSlot(290, lower): %v", err)
	}
	rec := kw.Record{
		Slot: slot, FreqHz: 146000000, Mode: kw.ModeFM,
		Byte19: '0', ToneMode: kw.ToneModeOff, Byte28: '0', Byte3940: "00", Byte41: '0',
		Shift: '0', OffsetHz: 0, Name: "SCANLO",
	}
	frame := memoryFrame(t, l, rec)

	sess, _ := openSession(t, NewTS2000, "019", radioImage{
		mrAnswers: map[string]string{"0290": frame},
	}, WithSimulatedProfile())

	ch, err := sess.ReadChannel(context.Background(), "290L")
	if err != nil {
		t.Fatalf("ReadChannel(290L): %v", err)
	}
	if ch.Data == nil || ch.Data.FreqHz != 146000000 {
		t.Errorf("ReadChannel(290L) = %+v, want FreqHz 146000000", ch.Data)
	}
}

// TestReadChannel_P7Equals3IsDCSNotCrossTone pins that this row's fourth
// tone-mode wire value is refused rather than mislabelled — matrix §6 item
// 1.
func TestReadChannel_P7Equals3IsDCSNotCrossTone(t *testing.T) {
	l := paramsTS2000.layout
	slot, err := l.NewSlot(10, kw.ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot(10): %v", err)
	}
	rec := kw.Record{
		Slot: slot, FreqHz: 145000000, Mode: kw.ModeFM,
		Byte19: '0', ToneMode: kw.ToneModeCross, Byte28: '0', Byte3940: "00", Byte41: '0',
		Shift: '0', OffsetHz: 0, Name: "DCSTEST",
	}
	frame := memoryFrame(t, l, rec)

	sess, _ := openSession(t, NewTS2000, "019", radioImage{
		mrAnswers: map[string]string{"0010": frame},
	}, WithSimulatedProfile())

	if _, err := sess.ReadChannel(context.Background(), "010"); err == nil {
		t.Error("ReadChannel(010) with P7='3' succeeded; want a refusal naming matrix §6 item 1")
	}
}

// TestReadChannel_P12Equals3IsAllETypesUnpublished pins the OS fourth-value
// refusal — matrix §6 item 2.
func TestReadChannel_P12Equals3IsAllETypesUnpublished(t *testing.T) {
	l := paramsTS2000.layout
	slot, err := l.NewSlot(11, kw.ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot(11): %v", err)
	}
	rec := kw.Record{
		Slot: slot, FreqHz: 145000000, Mode: kw.ModeFM,
		Byte19: '0', ToneMode: kw.ToneModeOff, Byte28: '0', Byte3940: "00", Byte41: '0',
		Shift: '3', OffsetHz: 0, Name: "ETYPE",
	}
	frame := memoryFrame(t, l, rec)

	sess, _ := openSession(t, NewTS2000, "019", radioImage{
		mrAnswers: map[string]string{"0011": frame},
	}, WithSimulatedProfile())

	if _, err := sess.ReadChannel(context.Background(), "011"); err == nil {
		t.Error("ReadChannel(011) with P12='3' succeeded; want a refusal naming matrix §6 item 2")
	}
}
