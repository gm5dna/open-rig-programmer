// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civicr8600 "github.com/gm5dna/open-rig-programmer/core/civ/icr8600"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

func writableChannel(slot, mode string) codeplug.Channel {
	d := &codeplug.ChannelData{
		FreqHz: 145_600_000, Mode: mode, Tag: "WRITE TEST",
		CTCSSTone:           codeplug.ToneField{State: codeplug.Unavailable},
		TagDisplay:          codeplug.BoolField{State: codeplug.Unavailable},
		ScanSkip:            codeplug.BoolField{State: codeplug.Unavailable},
		TxFreqHz:            codeplug.FreqField{State: codeplug.Unavailable},
		Duplex:              codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		OffsetHz:            codeplug.FreqField{State: codeplug.Known},
		ToneMode:            codeplug.StringField{State: codeplug.Unavailable},
		ToneTx:              codeplug.ToneField{State: codeplug.Unavailable},
		ToneRx:              codeplug.ToneField{State: codeplug.Unavailable},
		DTCSCode:            codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity:        codeplug.StringField{State: codeplug.Unavailable},
		Filter:              codeplug.StringField{State: codeplug.Known, Value: "FIL2"},
		DataMode:            codeplug.BoolField{State: codeplug.Unavailable},
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Known, Value: true},
		TuningStep:          codeplug.StringField{State: codeplug.Known, Value: "5 kHz"},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Known, Value: 12_500},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Known, Value: 20},
		Preamp:              codeplug.StringField{State: codeplug.Known, Value: "ON"},
		Antenna:             codeplug.StringField{State: codeplug.Known, Value: "ANT3"},
		IPPlus:              codeplug.BoolField{State: codeplug.Known, Value: true},
	}
	if mode == "FM" {
		d.ToneMode = codeplug.StringField{State: codeplug.Known, Value: "TSQL"}
		d.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: 885}
		d.DTCSCode = codeplug.IntField{State: codeplug.Known, Value: 23}
		d.DTCSPolarity = codeplug.StringField{State: codeplug.Known, Value: "Reverse"}
	}
	return codeplug.Channel{Slot: slot, Data: d}
}

func openConsentedWriteSession(t *testing.T, image testRadioImage) (*respondingPort, *Session) {
	t.Helper()
	return openTestSession(t, image, WithConsentedUnverifiedWrites())
}

func setFrames(frames [][]byte) [][]byte {
	var out [][]byte
	for _, f := range frames {
		if len(f) > 11 && f[4] == 0x1A && f[5] == 0x00 {
			out = append(out, f)
		}
	}
	return out
}

func requireWriteRefused(t *testing.T, res driver.WriteResult, err error, fields ...spec.Field) *driver.WriteRefusedError {
	t.Helper()
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel error = %v, want ErrWriteRefused", err)
	}
	if res.Steps == nil || len(res.Steps) != 0 {
		t.Errorf("WriteResult.Steps = %+v, want an explicitly empty slice", res.Steps)
	}
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("WriteChannel error type = %T, want *driver.WriteRefusedError", err)
	}
	if len(fields) > 0 && !slices.Equal(refused.Fields, fields) {
		t.Errorf("refused fields = %v, want %v", refused.Fields, fields)
	}
	return refused
}

func TestRequestedFields_OrderAndReceiveOnlyFieldsRemainVisibleToTheGate(t *testing.T) {
	wantUnconditional := []spec.Field{spec.FieldFrequency, spec.FieldMode, spec.FieldTag}
	if !slices.Equal(unconditionalFields, wantUnconditional) {
		t.Fatalf("unconditionalFields = %v, want %v", unconditionalFields, wantUnconditional)
	}
	wantTier := []spec.Field{
		spec.FieldClarifier, spec.FieldCTCSSState, spec.FieldCTCSSTone, spec.FieldShift,
		spec.FieldTagDisplay, spec.FieldScanSkip, spec.FieldTxFrequency,
		spec.FieldDuplex, spec.FieldOffset, spec.FieldToneMode, spec.FieldToneTx,
		spec.FieldToneRx, spec.FieldDTCSCode, spec.FieldDTCSPolarity,
		spec.FieldFilter, spec.FieldDataMode, spec.FieldTuningStepEnabled,
		spec.FieldTuningStep, spec.FieldProgramTuningStep, spec.FieldAttenuator,
		spec.FieldPreamp, spec.FieldAntenna, spec.FieldIPPlus,
	}
	var gotTier []spec.Field
	for _, entry := range tierRequestedFields {
		gotTier = append(gotTier, entry.field)
	}
	if !slices.Equal(gotTier, wantTier) {
		t.Errorf("tierRequestedFields = %v, want ChannelData declaration order %v", gotTier, wantTier)
	}

	d := *writableChannel("G00-000", "FM").Data
	d.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: 145_600_000}
	d.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: 885}
	got := requestedFields(d)
	if !slices.Contains(got, spec.FieldTxFrequency) || !slices.Contains(got, spec.FieldToneTx) {
		t.Errorf("requestedFields = %v: ReceiveOnly TX requests must reach the capability gate, never be dropped", got)
	}
}

func TestConsent_UnconsentedRefusesAndConsentedWritesOneFullRecord(t *testing.T) {
	addr := testWireAddress{0, 0}
	image := testRadioImage{idToken: []byte{0x01}, acknowledge: true, records: map[testWireAddress][]byte{addr: testRecord(t, addr, "FM")}}

	t.Run("unconsented", func(t *testing.T) {
		p, s := openTestSession(t, image)
		before := len(p.Transcript())
		res, err := s.WriteChannel(context.Background(), writableChannel("G00-000", "FM"))
		refused := requireWriteRefused(t, res, err)
		if len(refused.Fields) == 0 || len(p.Transcript()) != before {
			t.Errorf("unconsented refusal fields/transcript = %v/%d frames; want named fields and zero traffic", refused.Fields, len(p.Transcript())-before)
		}
	})

	t.Run("consented", func(t *testing.T) {
		p, s := openConsentedWriteSession(t, image)
		res, err := s.WriteChannel(context.Background(), writableChannel("G00-000", "FM"))
		if err != nil {
			t.Fatalf("WriteChannel: %v", err)
		}
		if len(res.Steps) != 1 || res.Steps[0] != (driver.WriteStep{Command: "1A 00", Sent: true, Confirmed: true}) {
			t.Errorf("Steps = %+v, want one acknowledged 1A 00", res.Steps)
		}
		sets := setFrames(p.Transcript())
		if len(sets) != 1 {
			t.Fatalf("set frames = %d, want one", len(sets))
		}
		if got, want := len(sets[0]), 6+4+44+1; got != want {
			t.Errorf("full FM set length = %d, want %d", got, want)
		}
		rec, err := s.parseRecord(civ.ChannelAddress{Group: 0, Channel: 0}, sets[0][10:len(sets[0])-1])
		if err != nil {
			t.Fatalf("parse written full record: %v", err)
		}
		if mode, _ := rec.Mode.Get(); mode != "FM" {
			t.Errorf("written mode = %q, want FM", mode)
		}
	})
}

func TestWrite_ReceiveOnlyTXFrequencyAndToneAreRefusedBeforeWire(t *testing.T) {
	addr := testWireAddress{0, 0}
	for _, tc := range []struct {
		field spec.Field
		set   func(*codeplug.ChannelData)
	}{
		{spec.FieldTxFrequency, func(d *codeplug.ChannelData) {
			d.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: 145_600_000}
		}},
		{spec.FieldToneTx, func(d *codeplug.ChannelData) { d.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: 885} }},
	} {
		t.Run(string(tc.field), func(t *testing.T) {
			p, s := openConsentedWriteSession(t, testRadioImage{idToken: []byte{0x01}, acknowledge: true, records: map[testWireAddress][]byte{addr: testRecord(t, addr, "FM")}})
			before := len(p.Transcript())
			ch := writableChannel("G00-000", "FM")
			tc.set(ch.Data)
			res, err := s.WriteChannel(context.Background(), ch)
			requireWriteRefused(t, res, err, tc.field)
			if got := len(p.Transcript()) - before; got != 0 {
				t.Errorf("refusal sent %d frames, want zero", got)
			}
		})
	}
}

func TestE6_DigitalTailMismatchUsesExactReason(t *testing.T) {
	addr := testWireAddress{0, 0}
	record := testRecord(t, addr, "D-STAR")
	record[37] ^= 0x01
	p, s := openConsentedWriteSession(t, testRadioImage{idToken: []byte{0x01}, acknowledge: true, records: map[testWireAddress][]byte{addr: record}})
	before := len(p.Transcript())
	res, err := s.WriteChannel(context.Background(), writableChannel("G00-000", "D-STAR"))
	refused := requireWriteRefused(t, res, err)
	if refused.Reason != civicr8600.DigitalTailRefusalReason {
		t.Errorf("E6 reason = %q, want exact %q", refused.Reason, civicr8600.DigitalTailRefusalReason)
	}
	if got := len(p.Transcript()) - before; got != 1 {
		t.Errorf("E6 refusal sent %d frames, want the one preservation read and no set", got)
	}
	if got := len(setFrames(p.Transcript())); got != 0 {
		t.Errorf("E6 refusal sent %d set frames", got)
	}
}

func TestE6_EveryCommonUnmappedHighNibbleIsRefused(t *testing.T) {
	addr := testWireAddress{0, 0}
	// These bytes map only their low nibble: SELECT, duplex, preamp,
	// antenna and IP+. A set rebuilt by the codec writes ZERO into each
	// high nibble, so E6 must refuse before that can silently replace a
	// state the neutral model cannot carry. What each of those nibbles
	// actually prints is named by the refusal itself and pinned by
	// TestE6_TheHighNibbleRefusalNamesWhatEachNibbleActuallyIs; none of
	// them is an assumed template.
	for name, offset := range map[string]int{"select": 0, "duplex": 8, "preamp": 18, "antenna": 19, "ip_plus": 20} {
		t.Run(name, func(t *testing.T) {
			record := testRecord(t, addr, "AM")
			record[offset] |= 0x10
			p, s := openConsentedWriteSession(t, testRadioImage{idToken: []byte{0x01}, acknowledge: true, records: map[testWireAddress][]byte{addr: record}})
			before := len(p.Transcript())
			res, err := s.WriteChannel(context.Background(), writableChannel("G00-000", "AM"))
			refused := requireWriteRefused(t, res, err)
			if !strings.Contains(refused.Reason, "E6") || !strings.Contains(refused.Reason, "unmapped high nibble") {
				t.Errorf("reason = %q, want an E6 unmapped-high-nibble refusal", refused.Reason)
			}
			if got := len(p.Transcript()) - before; got != 1 {
				t.Errorf("E6 refusal sent %d frames, want one preservation read", got)
			}
			if got := len(setFrames(p.Transcript())); got != 0 {
				t.Errorf("E6 refusal sent %d set frames", got)
			}
		})
	}
}

func TestE6_EveryFMOnlyUnmappedHighNibbleIsRefused(t *testing.T) {
	addr := testWireAddress{0, 0}
	for name, offset := range map[string]int{"tone_mode": 37, "dtcs_polarity": 41} {
		t.Run(name, func(t *testing.T) {
			record := testRecord(t, addr, "FM")
			record[offset] |= 0x10
			p, s := openConsentedWriteSession(t, testRadioImage{idToken: []byte{0x01}, acknowledge: true, records: map[testWireAddress][]byte{addr: record}})
			before := len(p.Transcript())
			res, err := s.WriteChannel(context.Background(), writableChannel("G00-000", "FM"))
			refused := requireWriteRefused(t, res, err)
			if !strings.Contains(refused.Reason, "E6") || !strings.Contains(refused.Reason, "unmapped high nibble") {
				t.Errorf("reason = %q, want an E6 unmapped-high-nibble refusal", refused.Reason)
			}
			if got := len(p.Transcript()) - before; got != 1 {
				t.Errorf("E6 refusal sent %d frames, want one preservation read", got)
			}
			if got := len(setFrames(p.Transcript())); got != 0 {
				t.Errorf("E6 refusal sent %d set frames", got)
			}
		})
	}
}

func TestE6_TheHighNibbleRefusalNamesWhatEachNibbleActuallyIs(t *testing.T) {
	// NONE OF THESE NIBBLES IS "ASSUMED". Byte 0's high nibble is the
	// three-valued printed enum 0=SKIP OFF / 1=SKIP / 2=PSKIP (matrix
	// section 2 row 9, section 3.16.4); bytes 8, 18, 19, 20 and FM 37
	// each print "0 (Fixed)" against their high nibble; FM 41 sits in
	// the DTCS span ㊻ ~ ㊽, against which the guide prints no value at
	// all. An operator reading the refusal has to be told which of those
	// it is, because what they must do at the radio differs: a SKIP or
	// PSKIP channel has a state to clear, a "0 (Fixed)" nibble that is
	// non-zero means the record is not what the document describes.
	addr := testWireAddress{0, 0}
	for _, tc := range []struct {
		name   string
		mode   string
		offset int
		want   []string
	}{
		{"select carries the SKIP/PSKIP enum", "AM", 0, []string{"SKIP OFF", "PSKIP"}},
		{"duplex is printed 0 (Fixed)", "AM", 8, []string{"0 (Fixed)"}},
		{"preamp is printed 0 (Fixed)", "AM", 18, []string{"0 (Fixed)"}},
		{"antenna is printed 0 (Fixed)", "AM", 19, []string{"0 (Fixed)"}},
		{"ip_plus is printed 0 (Fixed)", "AM", 20, []string{"0 (Fixed)"}},
		{"fm tone mode is printed 0 (Fixed)", "FM", 37, []string{"0 (Fixed)"}},
		{"fm dtcs polarity has no printed value", "FM", 41, []string{"DTCS"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := testRecord(t, addr, tc.mode)
			record[tc.offset] |= 0x10
			_, s := openConsentedWriteSession(t, testRadioImage{idToken: []byte{0x01}, acknowledge: true, records: map[testWireAddress][]byte{addr: record}})
			res, err := s.WriteChannel(context.Background(), writableChannel("G00-000", tc.mode))
			refused := requireWriteRefused(t, res, err)
			for _, want := range tc.want {
				if !strings.Contains(refused.Reason, want) {
					t.Errorf("reason = %q, want it to name %q", refused.Reason, want)
				}
			}
			if strings.Contains(refused.Reason, "assumed") {
				t.Errorf("reason = %q, but nothing about this nibble is assumed", refused.Reason)
			}
		})
	}
}

func TestWrite_OffsetOutsideThePrintedDomainIsRefusedBeforeTheWire(t *testing.T) {
	// Matrix section 3.15.7 draws the four offset bytes as eight labelled
	// nibble leaders: [1 kHz | 100 Hz (0 Fixed)], [100 kHz | 10 kHz],
	// [10 MHz | 1 MHz], [1 GHz (0 Fixed) | 100 MHz (0~2)]. The effective
	// resolution is therefore 1 kHz and the ceiling 299,999,000 Hz.
	//
	// Every OTHER printed "0 (Fixed)" nibble in this record is refused by
	// the E6 gate on the stored bytes. These two are reachable only here,
	// because the codec packs all eight nibbles as ONE BCD number: a
	// caller's offset is the only thing that can drive a non-zero digit
	// into them, and the raw four-byte bound alone lets it.
	addr := testWireAddress{0, 0}
	for name, offset := range map[string]uint64{
		"above the printed 299,999,000 Hz ceiling": 300_000_000,
		"a non-zero 1 GHz (0 Fixed) nibble":        5_000_000_000,
		"a non-zero 100 Hz (0 Fixed) nibble":       123_456_100,
	} {
		t.Run(name, func(t *testing.T) {
			p, s := openConsentedWriteSession(t, testRadioImage{idToken: []byte{0x01}, acknowledge: true, records: map[testWireAddress][]byte{addr: testRecord(t, addr, "AM")}})
			ch := writableChannel("G00-000", "AM")
			ch.Data.OffsetHz = codeplug.FreqField{State: codeplug.Known, Value: offset}
			before := len(p.Transcript())
			res, err := s.WriteChannel(context.Background(), ch)
			refused := requireWriteRefused(t, res, err, spec.FieldOffset)
			if !strings.Contains(refused.Reason, "3.15.7") {
				t.Errorf("reason = %q, want it to cite matrix section 3.15.7", refused.Reason)
			}
			// The refusal is locally decidable, so it must precede the
			// preservation read as every other field check does.
			if got := len(p.Transcript()) - before; got != 0 {
				t.Errorf("an offset refusal put %d frames on the wire, want none", got)
			}
		})
	}
}

func TestWrite_PreservesSupportedFMToneAndDTCSBytes(t *testing.T) {
	addr := testWireAddress{0, 0}
	stored := testRecord(t, addr, "FM")
	p, s := openConsentedWriteSession(t, testRadioImage{idToken: []byte{0x01}, acknowledge: true, records: map[testWireAddress][]byte{addr: stored}})
	ch := writableChannel("G00-000", "FM")
	ch.Data.ToneMode = codeplug.StringField{State: codeplug.Unknown}
	ch.Data.ToneRx = codeplug.ToneField{State: codeplug.Unknown}
	ch.Data.DTCSCode = codeplug.IntField{State: codeplug.Unavailable}
	ch.Data.DTCSPolarity = codeplug.StringField{State: codeplug.Unknown}
	if _, err := s.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	sets := setFrames(p.Transcript())
	if len(sets) != 1 {
		t.Fatalf("set frames = %d, want one", len(sets))
	}
	got := sets[0][10 : len(sets[0])-1]
	if !bytes.Equal(got[37:44], stored[37:44]) {
		t.Errorf("FM receive tail = % X, want preserved % X", got[37:44], stored[37:44])
	}
}

func TestWrite_MemorySetSpecAndTimeoutQuarantineNeverRetransmit(t *testing.T) {
	addr := testWireAddress{0, 0}
	p, s := openConsentedWriteSession(t, testRadioImage{idToken: []byte{0x01}, records: map[testWireAddress][]byte{addr: testRecord(t, addr, "FM")}})
	sp := s.memorySetSpec()
	if sp.Class != transport.ClassWriteWithAck || sp.RetryReads != 0 || sp.Match == nil {
		t.Fatalf("memorySetSpec = %+v, want ClassWriteWithAck, RetryReads 0 and the codec ack matcher", sp)
	}

	res, err := s.WriteChannel(context.Background(), writableChannel("G00-000", "FM"))
	if !errors.Is(err, transport.ErrTimeout) {
		t.Fatalf("WriteChannel error = %v, want ErrTimeout", err)
	}
	if len(res.Steps) != 1 || res.Steps[0].Sent || res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want one unattributable step", res.Steps)
	}
	if !strings.Contains(err.Error(), "UNATTRIBUTABLE") || !strings.Contains(err.Error(), "will not be resent") {
		t.Errorf("timeout error = %q, want the operator-facing no-resend warning", err)
	}
	if got := len(setFrames(p.Transcript())); got != 1 {
		t.Errorf("set frames after timeout = %d, want exactly one", got)
	}

	start := time.Now()
	if _, err := s.ReadChannel(context.Background(), "G00-000"); err != nil {
		t.Fatalf("read after timed-out write: %v", err)
	}
	if elapsed := time.Since(start); elapsed < civ.DrainIdleGap {
		t.Errorf("read after timeout took %v, want at least quarantine idle gap %v", elapsed, civ.DrainIdleGap)
	}
	if got := len(setFrames(p.Transcript())); got != 1 {
		t.Errorf("set frames after quarantine = %d, want no retry", got)
	}
	for _, f := range p.Transcript() {
		if len(f) >= 7 && f[4] == 0x1A && f[5] == 0x05 {
			t.Errorf("transceive-setting command was sent: % X", f)
		}
		if len(f) == 12 && f[4] == 0x1A && f[5] == 0x00 && f[10] == 0xFF {
			t.Errorf("erase command was sent: % X", f)
		}
	}
}

func TestWrite_AcknowledgementMatcherAndPreservationAddressAreExact(t *testing.T) {
	addr := testWireAddress{0, 0}
	p, s := openConsentedWriteSession(t, testRadioImage{idToken: []byte{0x01}, acknowledge: true, records: map[testWireAddress][]byte{addr: testRecord(t, addr, "AM")}})
	match := s.memorySetSpec().Match
	if !match(testFrame(civicr8600.ControllerAddress, civicr8600.RadioAddress, 0xFB)) {
		t.Error("ack matcher refused this receiver's exact FB")
	}
	if match(testFrame(civicr8600.ControllerAddress, 0x94, 0xFB)) || match(testFrame(civicr8600.ControllerAddress, civicr8600.RadioAddress, 0xFA)) {
		t.Error("ack matcher accepted a foreign-source FB or this receiver's FA")
	}

	p.misdirect(addr, testWireAddress{0, 1})
	before := len(p.Transcript())
	res, err := s.WriteChannel(context.Background(), writableChannel("G00-000", "AM"))
	if !errors.Is(err, ErrAnswerMismatch) {
		t.Fatalf("WriteChannel error = %v, want ErrAnswerMismatch from the preservation read", err)
	}
	if res.Steps == nil || len(res.Steps) != 0 {
		t.Errorf("Steps = %+v, want explicitly empty after the mismatched preservation read", res.Steps)
	}
	if got := len(p.Transcript()) - before; got != 1 {
		t.Errorf("address mismatch sent %d frames, want one preservation read", got)
	}
	if got := len(setFrames(p.Transcript())); got != 0 {
		t.Errorf("address mismatch sent %d set frames", got)
	}
}

func TestErase_RefusedWithoutWireAndGateAdmitsOnlyThreeGrammars(t *testing.T) {
	addr := testWireAddress{0, 0}
	p, s := openConsentedWriteSession(t, testRadioImage{idToken: []byte{0x01}, acknowledge: true, records: map[testWireAddress][]byte{addr: testRecord(t, addr, "AM")}})
	before := len(p.Transcript())
	res, err := s.WriteChannel(context.Background(), codeplug.Channel{Slot: "G00-000"})
	requireWriteRefused(t, res, err, spec.FieldErase)
	if len(p.Transcript()) != before {
		t.Error("erase refusal reached the wire")
	}

	profile := civicr8600.Profile()
	id, _ := profile.BuildTransceiverIDRead()
	read, _ := profile.BuildMemoryRead(civ.ChannelAddress{Group: 0, Channel: 0})
	set := testFrame(civicr8600.RadioAddress, civicr8600.ControllerAddress, append([]byte{0x1A, 0x00}, append(encodeTestAddress(addr), testRecord(t, addr, "AM")...)...)...)
	for name, frame := range map[string][]byte{"19 00 read": id.Bytes(), "1A 00 read": read.Bytes(), "1A 00 set": set} {
		if !profile.AllowedCommand(frame) {
			t.Errorf("AllowedCommand refused admitted grammar %s: % X", name, frame)
		}
	}
	for name, frame := range map[string][]byte{
		"clear":      testFrame(civicr8600.RadioAddress, civicr8600.ControllerAddress, 0x1A, 0x00, 0, 0, 0, 0, 0xFF),
		"transceive": testFrame(civicr8600.RadioAddress, civicr8600.ControllerAddress, 0x1A, 0x05, 0x00, 0x71, 0x00),
		"unknown":    testFrame(civicr8600.RadioAddress, civicr8600.ControllerAddress, 0x7F),
	} {
		if profile.AllowedCommand(frame) {
			t.Errorf("AllowedCommand admitted %s outside the three grammars: % X", name, frame)
		}
	}
}

// TestOccupiedSurprise_TheDiagnosticNamesTheWalkThisSessionRan pins the two
// halves of the occupied-surprise refusal, because a single message cannot be
// true of both walks and the wrong one sends the user somewhere useless.
//
// The closing review found the old single message asserting that "this build
// offers no setting that widens it" while icr8600.go:34 exports
// WithFullInventoryWalk. What is true is that no COMMAND-LINE or WINDOW
// control reaches it and no registered composition passes it, and that is what
// the bounded half now says. The full-walk half must not offer widening at
// all: there is nothing left to widen.
//
// This calls the rung directly rather than through a session, because after a
// FULL walk the fake cannot present the surprise — every address has been
// read, so producing one would mean the receiver gained a record mid-session,
// which the fake has no way to do. The condition is still exactly the one
// WriteChannel reaches: a slot absent from s.caps' MEM bank whose pre-write
// read returned a record.
func TestOccupiedSurprise_TheDiagnosticNamesTheWalkThisSessionRan(t *testing.T) {
	sessionWith := func(complete bool) *Session {
		return &Session{
			caps:   spec.Capabilities{Banks: []spec.Bank{{ID: spec.BankMemory, Slots: []string{"G00-000"}}}},
			report: OpenReport{InventoryComplete: complete},
		}
	}

	t.Run("a listed slot is not a surprise", func(t *testing.T) {
		if err := sessionWith(false).occupiedSurprise("G00-000", true); err != nil {
			t.Errorf("a slot the walk DID list was refused: %v", err)
		}
	})
	t.Run("an empty pre-write read is not a surprise", func(t *testing.T) {
		if err := sessionWith(false).occupiedSurprise("G05-037", false); err != nil {
			t.Errorf("a slot that read EMPTY was refused: %v", err)
		}
	})

	t.Run("after the bounded walk", func(t *testing.T) {
		var refused *driver.WriteRefusedError
		if err := sessionWith(false).occupiedSurprise("G05-037", true); !errors.As(err, &refused) {
			t.Fatalf("occupiedSurprise = %v, want a WriteRefusedError", err)
		}
		for _, want := range []string{
			"discovery walk never saw it",
			"BOUNDED walk",
			"Re-discover the receiver",
			// The honest form of the over-claim: no user-reachable
			// control widens it, and the Go option is named as one.
			"command line",
			"WithFullInventoryWalk",
		} {
			if !strings.Contains(refused.Reason, want) {
				t.Errorf("bounded refusal = %q, want it to contain %q", refused.Reason, want)
			}
		}
		if strings.Contains(refused.Reason, "no setting that widens it") {
			t.Errorf("the refusal still claims no setting widens the walk, which WithFullInventoryWalk falsifies: %q", refused.Reason)
		}
	})

	t.Run("after a full walk", func(t *testing.T) {
		var refused *driver.WriteRefusedError
		if err := sessionWith(true).occupiedSurprise("G05-037", true); !errors.As(err, &refused) {
			t.Fatalf("occupiedSurprise = %v, want a WriteRefusedError", err)
		}
		for _, want := range []string{"WHOLE 100x100", "AFTER this session opened", "front panel"} {
			if !strings.Contains(refused.Reason, want) {
				t.Errorf("full-walk refusal = %q, want it to contain %q", refused.Reason, want)
			}
		}
		for _, unwanted := range []string{"BOUNDED walk", "WithFullInventoryWalk"} {
			if strings.Contains(refused.Reason, unwanted) {
				t.Errorf("full-walk refusal offers %q, but there is nothing left to widen: %q", unwanted, refused.Reason)
			}
		}
	})
}
