// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// populatedChannel is a channel every requestedFieldRules entry this row
// grades can be asked to write, in the SAME mode family FM/AM occupies
// (matching currentFMChannel below) so a same-family write test need not
// also cross registerP14Family.
func populatedChannel(slot string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz: 146520000, Mode: "FM",
			Tag:      "NEWTAG",
			ScanSkip: codeplug.BoolField{State: codeplug.Known, Value: false},
			Duplex:   codeplug.StringField{State: codeplug.Known, Value: "1"},
			OffsetHz: codeplug.FreqField{State: codeplug.Known, Value: 600000},
			ToneMode: codeplug.StringField{State: codeplug.Known, Value: "CTCSS"},
			ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: 825},
			ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: 825},
		},
	}
}

// currentFMChannel is the scripted radio's own record for number before the
// write: FM, with byte28/byte3940/byte41/DCSCode set to values this
// package models no field for, so a test can assert they SURVIVE the
// write unmutated.
func currentFMChannel(t *testing.T, l kw.Layout, number int) string {
	t.Helper()
	slot, err := l.NewSlot(number, kw.ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot(%d): %v", number, err)
	}
	return memoryFrame(t, l, kw.Record{
		Slot: slot, FreqHz: 145500000, Mode: kw.ModeFM,
		Byte19: '0', ToneMode: kw.ToneModeOff, Byte28: '1', Byte3940: "03", Byte41: '7',
		Shift: '0', OffsetHz: 0, DCSCode: 45, Name: "OLDTAG",
	})
}

// addrFor is the four-byte MR/MW addressing key (P1+P2+P3P3) a
// respondingPort image keys its answers by, built through BuildMRRead so
// this test can never drift from the codec's own addressing.
func addrFor(t *testing.T, l kw.Layout, number int) string {
	t.Helper()
	slot, err := l.NewSlot(number, kw.ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot(%d): %v", number, err)
	}
	cmd, err := l.BuildMRRead(slot)
	if err != nil {
		t.Fatalf("BuildMRRead(%d): %v", number, err)
	}
	return string(cmd.Bytes()[2:6])
}

// TestWriteChannel_PreservesUnmodeledBytes is the write-existing proof: a
// write that changes only fields this row models succeeds, and the MW
// frame that goes out carries byte28/byte3940/byte41/DCSCode UNCHANGED
// from the slot's own pre-write state.
func TestWriteChannel_PreservesUnmodeledBytes(t *testing.T) {
	l := paramsTS2000.layout
	addr := addrFor(t, l, 72)
	current := currentFMChannel(t, l, 72)

	sess, p := openSession(t, NewTS2000, "019", radioImage{
		mrAnswers: map[string]string{addr: current},
	}, WithSimulatedProfile())

	res, err := sess.WriteChannel(context.Background(), populatedChannel("072"))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || res.Steps[0].Command != "MW" || !res.Steps[0].Sent {
		t.Fatalf("WriteResult = %+v, want one Sent MW step", res)
	}

	var sentMW string
	for _, f := range p.Transcript() {
		if strings.HasPrefix(f, "MW") {
			sentMW = f
		}
	}
	if sentMW == "" {
		t.Fatal("no MW frame appears in the transcript")
	}
	answer := []byte(sentMW)
	answer[0], answer[1] = 'M', 'R'
	got, err := l.ParseMRAnswer(answer)
	if err != nil {
		t.Fatalf("parsing the sent MW as an MR answer: %v", err)
	}
	if got.Byte28 != '1' {
		t.Errorf("Byte28 (P11 REVERSE) = %q, want '1' (preserved from the pre-write read)", got.Byte28)
	}
	if got.Byte3940 != "03" {
		t.Errorf("Byte3940 (P14 step index) = %q, want \"03\" (preserved)", got.Byte3940)
	}
	if got.Byte41 != '7' {
		t.Errorf("Byte41 (P15 Memory Group) = %q, want '7' (preserved)", got.Byte41)
	}
	if got.DCSCode != 45 {
		t.Errorf("DCSCode (P10) = %d, want 45 (preserved)", got.DCSCode)
	}
	if got.FreqHz != 146520000 {
		t.Errorf("FreqHz = %d, want 146520000 (the requested value)", got.FreqHz)
	}
	if strings.TrimRight(got.Name, " ") != "NEWTAG" {
		t.Errorf("Name = %q, want \"NEWTAG\" (the requested value)", got.Name)
	}
	if got.Shift != '1' {
		t.Errorf("Shift (P12) = %q, want '1' (the requested duplex)", got.Shift)
	}
	if got.OffsetHz != 600000 {
		t.Errorf("OffsetHz (P13) = %d, want 600000 (the requested value)", got.OffsetHz)
	}
}

// TestWriteChannel_UnconsentedIsRefusedByTheGateBeforeAnyRead pins that an
// unconsented RealHardware session never reaches the pre-write read at
// all: the capability gate (all-Unverified, writeTrialsComplete false)
// answers first.
func TestWriteChannel_UnconsentedIsRefusedByTheGateBeforeAnyRead(t *testing.T) {
	l := paramsTS2000.layout
	addr := addrFor(t, l, 72)
	current := currentFMChannel(t, l, 72)

	sess, p := openSession(t, NewTS2000, "019", radioImage{
		mrAnswers: map[string]string{addr: current},
	})
	_, err := sess.WriteChannel(context.Background(), populatedChannel("072"))
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("err = %v, want errors.Is(_, driver.ErrWriteRefused)", err)
	}
	var re *RefusalError
	if errors.As(err, &re) {
		t.Errorf("err = %v, want the bare capability-gate refusal, not *RefusalError", err)
	}
	for _, f := range p.Transcript() {
		if f != "AI0;" && f != "ID;" && f != "TY;" {
			t.Errorf("a frame was sent before the capability gate refused: %q", f)
		}
	}
}

// TestWriteChannel_TargetUnassignedIsRefused pins registerCreate: this
// programme does not create channels.
func TestWriteChannel_TargetUnassignedIsRefused(t *testing.T) {
	l := paramsTS2000.layout
	addr := addrFor(t, l, 72)
	empty := make([]byte, kw.RecordLen)
	for i := range empty {
		empty[i] = '0'
	}
	empty[0], empty[1] = 'M', 'R'
	copy(empty[3:6], "072")
	copy(empty[41:49], "        ")
	empty[49] = ';'

	sess, p := openSession(t, NewTS2000, "019", radioImage{
		mrAnswers: map[string]string{addr: string(empty)},
	}, WithSimulatedProfile())

	_, err := sess.WriteChannel(context.Background(), populatedChannel("072"))
	var re *RefusalError
	if !errors.As(err, &re) || re.Register != registerCreate {
		t.Fatalf("err = %v, want a *RefusalError naming registerCreate", err)
	}
	for _, f := range p.Transcript() {
		if strings.HasPrefix(f, "MW") {
			t.Errorf("an MW frame was sent to an unassigned target: %q", f)
		}
	}
}

// TestWriteChannel_ModeFamilyChangeIsRefused pins registerP14Family: the
// pre-write read is USB (SSB/CW/FSK family) and the write asks for FM
// (AM/FM family), so preserving P14 verbatim would silently reinterpret
// it.
func TestWriteChannel_ModeFamilyChangeIsRefused(t *testing.T) {
	l := paramsTS2000.layout
	addr := addrFor(t, l, 72)
	slot, err := l.NewSlot(72, kw.ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot(72): %v", err)
	}
	current := memoryFrame(t, l, kw.Record{
		Slot: slot, FreqHz: 14250000, Mode: kw.ModeUSB,
		Byte19: '0', ToneMode: kw.ToneModeOff, Byte28: '0', Byte3940: "02", Byte41: '0',
		Shift: '0', OffsetHz: 0, Name: "USBCHAN",
	})

	sess, p := openSession(t, NewTS2000, "019", radioImage{
		mrAnswers: map[string]string{addr: current},
	}, WithSimulatedProfile())

	_, err = sess.WriteChannel(context.Background(), populatedChannel("072"))
	var re *RefusalError
	if !errors.As(err, &re) || re.Register != registerP14Family {
		t.Fatalf("err = %v, want a *RefusalError naming registerP14Family", err)
	}
	for _, f := range p.Transcript() {
		if strings.HasPrefix(f, "MW") {
			t.Errorf("an MW frame was sent across a P14 family change: %q", f)
		}
	}
}

// TestWriteChannel_ToneRx1750HzIsRefused pins the M-E1 restatement: CN's
// own printed domain stops one entry short of TN's, so a Known tone_rx of
// 1750 Hz (the chart's 39th, TN-only entry) is refused rather than
// written.
func TestWriteChannel_ToneRx1750HzIsRefused(t *testing.T) {
	l := paramsTS2000.layout
	addr := addrFor(t, l, 72)
	current := currentFMChannel(t, l, 72)

	sess, p := openSession(t, NewTS2000, "019", radioImage{
		mrAnswers: map[string]string{addr: current},
	}, WithSimulatedProfile())

	ch := populatedChannel("072")
	ch.Data.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: 17500}
	_, err := sess.WriteChannel(context.Background(), ch)
	var wre *driver.WriteRefusedError
	if !errors.As(err, &wre) || len(wre.Fields) != 1 || wre.Fields[0] != "tone_rx" {
		t.Fatalf("err = %v, want a *driver.WriteRefusedError naming tone_rx", err)
	}
	for _, f := range p.Transcript() {
		if strings.HasPrefix(f, "MW") {
			t.Errorf("an MW frame was sent with tone_rx 1750 Hz: %q", f)
		}
	}
}

// TestWriteChannel_EmptyChannelIsRefusedAsErase pins the erase rung, ahead
// of the field checks and the pre-write read.
func TestWriteChannel_EmptyChannelIsRefusedAsErase(t *testing.T) {
	sess, _ := openSession(t, NewTS2000, "019", radioImage{}, WithSimulatedProfile())
	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "072"})
	var wre *driver.WriteRefusedError
	if !errors.As(err, &wre) {
		t.Fatalf("WriteChannel(empty): %v, want *driver.WriteRefusedError", err)
	}
	if len(wre.Fields) != 1 || wre.Fields[0] != "erase" {
		t.Errorf("WriteRefusedError.Fields = %v, want [erase]", wre.Fields)
	}
}

// TestWriteChannel_UnknownSlotIsRefusedBeforeAnyFieldCheck pins slot
// membership as the first rung, ahead of the empty-channel check.
func TestWriteChannel_UnknownSlotIsRefusedBeforeAnyFieldCheck(t *testing.T) {
	sess, _ := openSession(t, NewTS2000, "019", radioImage{}, WithSimulatedProfile())
	_, err := sess.WriteChannel(context.Background(), populatedChannel("999"))
	var use *UnknownSlotError
	if !errors.As(err, &use) {
		t.Fatalf("WriteChannel(999): %v, want *UnknownSlotError", err)
	}
}
