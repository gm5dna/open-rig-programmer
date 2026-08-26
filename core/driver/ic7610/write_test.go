// SPDX-License-Identifier: GPL-3.0-or-later

package ic7610

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7610 "github.com/gm5dna/open-rig-programmer/core/civ/ic7610"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// goodChannel is a channel every rung of the ladder admits: the seven
// mapped fields all present and all in domain, the values of the golden
// record.
func goodChannel(slot string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz:   14_250_000,
			Mode:     "USB",
			Tag:      "HOME QTH01",
			Filter:   codeplug.StringField{State: codeplug.Known, Value: "FIL1"},
			ToneMode: codeplug.StringField{State: codeplug.Known, Value: "TONE"},
			ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: 885},
			ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: 1000},
		},
	}
}

// writeSession opens a Simulated session (whose seven mapped fields are
// Supported) against img.
func writeSession(t *testing.T, img radioImage, opts ...Option) (*Session, *scriptedPort) {
	t.Helper()
	p := newScriptedPort(t, img)
	return openWith(t, p, opts...), p
}

// framesAfterOpen returns the frames the port received after the ones Open
// itself wrote, so a write test asserts on ITS OWN traffic.
func framesAfterOpen(t *testing.T, p *scriptedPort, openFrames int) [][]byte {
	t.Helper()
	all := p.Transcript()
	if len(all) < openFrames {
		t.Fatalf("the transcript has %d frames, fewer than Open's %d", len(all), openFrames)
	}
	return all[openFrames:]
}

// ackingRadio answers every 1A 00 set with FB and holds the golden record
// at every channel a write test targets.
func ackingRadio() radioImage {
	img := occupiedRadio()
	img.ackSets = true
	img.records[42] = goldenRecord
	return img
}

// TestMemorySetSpec_IsWriteWithAckAndNeverRetries pins the contract of the
// spec this driver builds its write with.
//
// AN ACKNOWLEDGED WRITE IS STILL A WRITE: its ack tells you the outcome
// when one arrives and NOTHING AT ALL when one does not. RetryReads is
// zero because a timeout is never resolved by resending a write (safety
// obligation 2), and Engine.Do refuses a non-zero value on this class with
// ErrInvalidSpec before writing anything — so the engine enforces it too.
func TestMemorySetSpec_IsWriteWithAckAndNeverRetries(t *testing.T) {
	sp := civ.CIVWriteWithAckSpec(civic7610.Profile().AcknowledgementMatcher())
	if sp.Class != transport.ClassWriteWithAck {
		t.Errorf("Class = %v, want ClassWriteWithAck", sp.Class)
	}
	if sp.RetryReads != 0 {
		t.Errorf("RetryReads = %d, want 0 - a write is never resent", sp.RetryReads)
	}
	if sp.Match == nil {
		t.Error("Match is nil - the ack matcher must come from the codec, never from this package")
	}
}

// TestWriteChannel_FBIsConfirmed: an FB answer yields Sent and Confirmed.
//
// Register entry ic7610-1a00-set-ack, matrix lift R14: THE CODES ARE
// MANUAL-EVIDENCED BUT THAT A 1A 00 SET IS ANSWERED BY ONE OF THEM IS
// ASSUMED. The nearest printed statement is about command 29, which this
// driver does not read across.
func TestWriteChannel_FBIsConfirmed(t *testing.T) {
	s, p := writeSession(t, ackingRadio())
	openFrames := len(p.Transcript())

	res, err := s.WriteChannel(t.Context(), goodChannel("042"))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	want := []driver.WriteStep{{Command: memorySetStep, Sent: true, Confirmed: true}}
	if !reflect.DeepEqual(res.Steps, want) {
		t.Errorf("Steps = %+v, want %+v", res.Steps, want)
	}
	// One preservation read, then one set. Nothing else.
	frames := framesAfterOpen(t, p, openFrames)
	if len(frames) != 2 {
		t.Fatalf("the write put %d frames on the wire, want 2 (one read, one set):\n  %s", len(frames), hexFrames(frames))
	}
	if !bytes.Equal(frames[0], memReadFrame(42)) {
		t.Errorf("the first frame was %s, want the preservation read", hexFrames(frames[:1]))
	}
	if len(frames[1]) != memSetFrameLen {
		t.Errorf("the set frame is %d bytes, want %d (6 envelope + 2 address + 25 record + 1 terminator)", len(frames[1]), memSetFrameLen)
	}
}

// TestWriteChannel_AckTimeoutReportsAnUnknownOutcome: a port that swallows
// the set and never acks.
//
// THE WriteStep REPORTS Sent: false, Confirmed: false.
// core/driver/driver.go defines a false Sent as "the outcome is NOT
// known-clean … a transport-level failure left its outcome unknowable",
// which is exactly an ack timeout. Reporting Sent: true would put a false
// ATTRIBUTABLE outcome in the audit trail.
//
// WHAT PROVES THE NEVER-RETRANSMIT RULE IS NOT THE FLAG BUT THE BYTE LOG:
// exactly ONE set frame reached the port.
func TestWriteChannel_AckTimeoutReportsAnUnknownOutcome(t *testing.T) {
	img := ackingRadio()
	img.ackSets = false // silence
	s, p := writeSession(t, img)
	openFrames := len(p.Transcript())

	res, err := s.WriteChannel(t.Context(), goodChannel("042"))
	if err == nil {
		t.Fatal("WriteChannel succeeded with no acknowledgement; silence is not success on an acknowledged write")
	}
	want := []driver.WriteStep{{Command: memorySetStep, Sent: false, Confirmed: false}}
	if !reflect.DeepEqual(res.Steps, want) {
		t.Errorf("Steps = %+v, want %+v - an ack timeout leaves the outcome UNKNOWN", res.Steps, want)
	}
	var sets int
	for _, f := range framesAfterOpen(t, p, openFrames) {
		if len(f) >= memSetFrameLen {
			sets++
		}
	}
	if sets != 1 {
		t.Errorf("%d set frames reached the port, want exactly 1 - an acknowledged write is still a WRITE and is never resent", sets)
	}
}

// TestWriteChannel_FARejectionIsReported: an FA answer from THIS radio is
// a rejection; an FA from another address is not, and leaves the wait
// running (E1's source-address check).
func TestWriteChannel_FARejectionIsReported(t *testing.T) {
	t.Run("FA from 0x98 is a rejection", func(t *testing.T) {
		img := ackingRadio()
		img.ackSets = false
		img.rejectSets = true
		s, _ := writeSession(t, img)
		res, err := s.WriteChannel(t.Context(), goodChannel("042"))
		if !errors.Is(err, transport.ErrRejected) {
			t.Fatalf("err = %v, want one satisfying errors.Is(err, transport.ErrRejected)", err)
		}
		if !strings.Contains(err.Error(), memorySetStep) {
			t.Errorf("the rejection %q does not name the command", err)
		}
		// A rejection is an ATTRIBUTABLE outcome, so Sent is true.
		want := []driver.WriteStep{{Command: memorySetStep, Sent: true, Confirmed: false}}
		if !reflect.DeepEqual(res.Steps, want) {
			t.Errorf("Steps = %+v, want %+v", res.Steps, want)
		}
	})

	t.Run("FA from another station is not a rejection", func(t *testing.T) {
		img := ackingRadio()
		img.ackSets = false
		img.rejectSets = true
		img.setFrom = 0x94
		s, _ := writeSession(t, img)
		_, err := s.WriteChannel(t.Context(), goodChannel("042"))
		if err == nil {
			t.Fatal("the write succeeded on another station's FA")
		}
		if errors.Is(err, transport.ErrRejected) {
			t.Errorf("err = %v, want a TIMEOUT: E1's IsRejection is source-address-checked, so an FA from 0x94 leaves the wait running rather than ending it", err)
		}
	})
}

// TestWriteChannel_UnverifiedRefusesBeforeAnyWire: under RealHardware
// without consent, NOTHING is writable and ZERO BYTES reach the port.
func TestWriteChannel_UnverifiedRefusesBeforeAnyWire(t *testing.T) {
	p := newScriptedPort(t, ackingRadio())
	d := New(RealHardware)
	sess, err := d.Open(t.Context(), p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = sess.Close() }()
	openFrames := len(p.Transcript())

	res, err := sess.WriteChannel(t.Context(), goodChannel("042"))
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("err = %v, want one satisfying errors.Is(err, driver.ErrWriteRefused)", err)
	}
	if len(res.Steps) != 0 || res.Steps == nil {
		t.Errorf("Steps = %+v, want an EMPTY non-nil slice - a refusal before any frame is built has no sequence to describe", res.Steps)
	}
	if frames := framesAfterOpen(t, p, openFrames); len(frames) != 0 {
		t.Errorf("%d frames reached the port:\n  %s\nwant NONE - a locally decidable refusal precedes ALL wire traffic (T5)", len(frames), hexFrames(frames))
	}
}

// TestWriteChannel_KnownUnmappedFieldIsRefusedNotDropped: a channel whose
// ScanSkip or DataMode is KNOWN is refused before any wire traffic,
// because both carry the zero FieldSupport.
//
// NEVER DROPPED, NEVER COLLAPSED 4->2 (adjudication R6). A Known value is
// a REQUEST, and this project refuses a request it cannot honour rather
// than quietly writing a record that disagrees with what the user asked
// for. requestedFields appends both fields when Known precisely so the
// gate can SEE them.
func TestWriteChannel_KnownUnmappedFieldIsRefusedNotDropped(t *testing.T) {
	for _, tt := range []struct {
		name  string
		field spec.Field
		set   func(*codeplug.ChannelData)
	}{
		{"ScanSkip Known false", spec.FieldScanSkip, func(d *codeplug.ChannelData) {
			d.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: false}
		}},
		{"ScanSkip Known true", spec.FieldScanSkip, func(d *codeplug.ChannelData) {
			d.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
		}},
		{"DataMode Known false", spec.FieldDataMode, func(d *codeplug.ChannelData) {
			d.DataMode = codeplug.BoolField{State: codeplug.Known, Value: false}
		}},
		{"DataMode Known true", spec.FieldDataMode, func(d *codeplug.ChannelData) {
			d.DataMode = codeplug.BoolField{State: codeplug.Known, Value: true}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, p := writeSession(t, ackingRadio())
			openFrames := len(p.Transcript())
			ch := goodChannel("042")
			tt.set(ch.Data)

			res, err := s.WriteChannel(t.Context(), ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("err = %v, want ErrWriteRefused - a Known unmapped field must be REFUSED, not dropped", err)
			}
			var refusal *driver.WriteRefusedError
			if !errors.As(err, &refusal) {
				t.Fatalf("err = %v, want a *driver.WriteRefusedError", err)
			}
			if !containsField(refusal.Fields, tt.field) {
				t.Errorf("the refusal names %v, want it to name %s", refusal.Fields, tt.field)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty", res.Steps)
			}
			if frames := framesAfterOpen(t, p, openFrames); len(frames) != 0 {
				t.Errorf("%d frames reached the port, want none", len(frames))
			}
		})
	}
}

// TestWriteChannel_RefusesASlotWhoseUnmappedRegionsDiffer — THE E6 RULING.
//
// RULING E6, VERBATIM: a driver may write a slot ONLY when its unmapped
// regions equal the profile's Fixed template; anything else is REFUSED
// with the reason named, NEVER REWRITTEN.
//
// THE COST: a SELECT-group member (★1/★2/★3) or a DATA 1/2/3 channel
// CANNOT BE WRITTEN BY THIS PROGRAMME AT ALL. It is never silently
// downgraded to ★1/DATA 1 and never silently cleared to OFF.
//
// The assertion is on the byte log: exactly ONE read exchange, and NO SET
// FRAME. That one read is tier ruling T5's single recorded exception to
// "refusal before any wire traffic" — and it is the reason every eligible
// write on this model costs an extra exchange.
func TestWriteChannel_RefusesASlotWhoseUnmappedRegionsDiffer(t *testing.T) {
	for _, tt := range []struct {
		name   string
		prior  []byte
		offset int
		nibble string
		got    byte
	}{
		{"byte 0 low = 2, SELECT ★2", withRecord(0, 0x02), civic7610.SelectNibbleOffset, "low", 0x02},
		{"byte 0 low = 1, SELECT ★1", withRecord(0, 0x01), civic7610.SelectNibbleOffset, "low", 0x01},
		{"byte 0 high = 1, not the printed Fixed 0", withRecord(0, 0x10), civic7610.SelectNibbleOffset, "high", 0x01},
		{"byte 8 high = 2, DATA 2", withRecord(8, 0x21), civic7610.DataModeNibbleOffset, "high", 0x02},
		{"byte 8 high = 1, DATA 1", withRecord(8, 0x11), civic7610.DataModeNibbleOffset, "high", 0x01},
	} {
		t.Run(tt.name, func(t *testing.T) {
			img := ackingRadio()
			img.records[42] = tt.prior
			s, p := writeSession(t, img)
			openFrames := len(p.Transcript())

			res, err := s.WriteChannel(t.Context(), goodChannel("042"))
			if !errors.Is(err, ErrUnmappedRegion) {
				t.Fatalf("err = %v, want one satisfying errors.Is(err, ErrUnmappedRegion)", err)
			}
			var e *UnmappedRegionError
			if !errors.As(err, &e) {
				t.Fatalf("err = %v, want an *UnmappedRegionError", err)
			}
			if e.Offset != tt.offset || e.Nibble != tt.nibble || e.Want != 0 || e.Got != tt.got {
				t.Errorf("*UnmappedRegionError = %+v, want {Offset: %d, Nibble: %q, Want: 0, Got: %#x}", e, tt.offset, tt.nibble, tt.got)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty - no frame was ever built", res.Steps)
			}

			frames := framesAfterOpen(t, p, openFrames)
			if len(frames) != 1 {
				t.Fatalf("the refused write put %d frames on the wire, want exactly 1 (the T5 preservation read):\n  %s", len(frames), hexFrames(frames))
			}
			if !bytes.Equal(frames[0], memReadFrame(42)) {
				t.Errorf("the one frame was %s, want the preservation read", hexFrames(frames))
			}
		})
	}
}

// TestWriteChannel_EmptySlotWritesAgainstTheTemplate: the preservation
// read is rejected, so there are NO unmapped regions to compare and the
// write proceeds. The set carries the template's zeros at both regions.
func TestWriteChannel_EmptySlotWritesAgainstTheTemplate(t *testing.T) {
	img := ackingRadio()
	delete(img.records, 42) // answered FA
	s, p := writeSession(t, img)
	openFrames := len(p.Transcript())

	res, err := s.WriteChannel(t.Context(), goodChannel("042"))
	if err != nil {
		t.Fatalf("WriteChannel into an empty slot: %v", err)
	}
	if !res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want a confirmed write", res.Steps)
	}
	frames := framesAfterOpen(t, p, openFrames)
	if len(frames) != 2 {
		t.Fatalf("the write put %d frames on the wire, want 2:\n  %s", len(frames), hexFrames(frames))
	}
	rec := frames[1][8 : len(frames[1])-1]
	tmpl := civic7610.FixedTemplate()
	if rec[civic7610.SelectNibbleOffset] != tmpl[civic7610.SelectNibbleOffset] {
		t.Errorf("the set's byte %d is %#02x, want the template's %#02x", civic7610.SelectNibbleOffset, rec[civic7610.SelectNibbleOffset], tmpl[civic7610.SelectNibbleOffset])
	}
	if rec[civic7610.DataModeNibbleOffset]>>4 != tmpl[civic7610.DataModeNibbleOffset]>>4 {
		t.Errorf("the set's byte %d high nibble is %#x, want the template's %#x", civic7610.DataModeNibbleOffset, rec[civic7610.DataModeNibbleOffset]>>4, tmpl[civic7610.DataModeNibbleOffset]>>4)
	}
}

// TestWriteChannel_ConsentedWritesTheFullRecord: with
// WithConsentedUnverifiedWrites on a RealHardware driver, the frame on the
// wire is the full 34-byte set.
//
// Register entry ic7610-full-record-mandatory, matrix lift R15: the
// document prints one full-record form and one three-index clear form and
// NEVER SAYS whether a short record is accepted, so the driver always
// sends all 25 record bytes.
func TestWriteChannel_ConsentedWritesTheFullRecord(t *testing.T) {
	p := newScriptedPort(t, ackingRadio())
	d := New(RealHardware, WithConsentedUnverifiedWrites())
	sess, err := d.Open(t.Context(), p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = sess.Close() }()
	openFrames := len(p.Transcript())

	if _, err := sess.WriteChannel(t.Context(), goodChannel("042")); err != nil {
		t.Fatalf("WriteChannel under consent: %v", err)
	}
	frames := framesAfterOpen(t, p, openFrames)
	if len(frames) != 2 {
		t.Fatalf("the write put %d frames on the wire, want 2:\n  %s", len(frames), hexFrames(frames))
	}
	set := frames[1]
	if len(set) != memSetFrameLen {
		t.Errorf("the set frame is %d bytes, want %d", len(set), memSetFrameLen)
	}
	// The whole record, byte for byte: this is the golden vector's
	// set-record-name-with-space payload.
	want := append([]byte{0xFE, 0xFE, 0x98, 0xE0, 0x1A, 0x00, 0x00, 0x42}, goldenRecord...)
	want = append(want, 0xFD)
	// Channel 42's selector is BCD "00 42".
	if !bytes.Equal(set, want) {
		t.Errorf("the set frame was\n  %s\nwant\n  %s", hexFrames([][]byte{set}), hexFrames([][]byte{want}))
	}
}

// TestWriteChannel_EraseIsRefused — ChannelData HAS NO Erase MEMBER.
//
// Erase is represented solely by Channel.Data == nil, "the SOLE
// discriminator between empty and populated" (core/codeplug/channel.go).
// FieldErase carries the zero FieldSupport in both profiles and
// spec.ConsentUnverifiedWrites structurally never consents it, so the
// refusal stands under BOTH profiles AND with consent applied.
func TestWriteChannel_EraseIsRefused(t *testing.T) {
	for _, tt := range []struct {
		name string
		mk   func() driver.Driver
	}{
		{"RealHardware", func() driver.Driver { return New(RealHardware) }},
		{"RealHardware + consent", func() driver.Driver { return New(RealHardware, WithConsentedUnverifiedWrites()) }},
		{"Simulated", func() driver.Driver { return New(Simulated) }},
		{"Simulated + consent", func() driver.Driver { return New(Simulated, WithConsentedUnverifiedWrites()) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := newScriptedPort(t, ackingRadio())
			sess, err := tt.mk().Open(t.Context(), p.Port(), driver.Identity{})
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			defer func() { _ = sess.Close() }()
			openFrames := len(p.Transcript())

			res, err := sess.WriteChannel(t.Context(), codeplug.Channel{Slot: "001", Data: nil})
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("err = %v, want ErrWriteRefused", err)
			}
			var refusal *driver.WriteRefusedError
			if errors.As(err, &refusal) && !containsField(refusal.Fields, spec.FieldErase) {
				t.Errorf("the refusal names %v, want it to name erase", refusal.Fields)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty", res.Steps)
			}
			if frames := framesAfterOpen(t, p, openFrames); len(frames) != 0 {
				t.Errorf("%d frames reached the port, want none", len(frames))
			}
		})
	}
}

// TestWriteChannel_NoClearFrameIsReachable: NO input, under any profile,
// with or without consent, produces either of this radio's two printed
// clear forms.
//
// The gate would refuse one — core/civ's AllowedCommand admits only 19 00,
// a valid 1A 00 read and a re-validated 1A 00 set — and this asserts the
// driver never BUILDS one.
func TestWriteChannel_NoClearFrameIsReachable(t *testing.T) {
	// (a) FE FE 98 E0 1A 00 <ch-hi> <ch-lo> FF FD, and
	// (b) FE FE 98 E0 0B FD.
	isClearForm := func(f []byte) bool {
		if len(f) == 6 && f[4] == 0x0B {
			return true
		}
		return len(f) == 10 && f[4] == 0x1A && f[5] == 0x00 && f[8] == 0xFF
	}

	inputs := []codeplug.Channel{
		goodChannel("001"),
		goodChannel("P1"),
		{Slot: "001", Data: nil},
		{Slot: "042", Data: &codeplug.ChannelData{}},
		func() codeplug.Channel {
			ch := goodChannel("042")
			ch.Data.Tag = ""
			return ch
		}(),
		func() codeplug.Channel {
			ch := goodChannel("042")
			ch.Data.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
			ch.Data.ToneRx = codeplug.ToneField{State: codeplug.Unavailable}
			return ch
		}(),
	}
	for _, mk := range []func() driver.Driver{
		func() driver.Driver { return New(RealHardware) },
		func() driver.Driver { return New(RealHardware, WithConsentedUnverifiedWrites()) },
		func() driver.Driver { return New(Simulated) },
		func() driver.Driver { return New(Simulated, WithConsentedUnverifiedWrites()) },
	} {
		p := newScriptedPort(t, ackingRadio())
		sess, err := mk().Open(t.Context(), p.Port(), driver.Identity{})
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		for _, ch := range inputs {
			_, _ = sess.WriteChannel(t.Context(), ch)
		}
		_ = sess.Close()
		for _, f := range p.Transcript() {
			if isClearForm(f) {
				t.Errorf("a clear frame reached the port: %s", hexFrames([][]byte{f}))
			}
		}
	}
}

// TestWriteChannel_RefusesAFrequencyOutsideTheEncodableRange — the
// adjudication R11 pre-build typed refusal.
//
// Matrix §1 row 12: PDF p.11's five-cell frequency strip labels the 10 MHz
// digit 0-6 and prints cell 5 as a fixed "0 : 0", so 69 999 999 Hz is the
// largest encodable value.
func TestWriteChannel_RefusesAFrequencyOutsideTheEncodableRange(t *testing.T) {
	for _, hz := range []uint64{70_000_000, 100_000_000, 1 << 40} {
		s, p := writeSession(t, ackingRadio())
		openFrames := len(p.Transcript())
		ch := goodChannel("042")
		ch.Data.FreqHz = hz

		res, err := s.WriteChannel(t.Context(), ch)
		var e *OutOfDomainError
		if !errors.As(err, &e) {
			t.Fatalf("%d Hz gave %v, want an *OutOfDomainError", hz, err)
		}
		if e.Field != spec.FieldFrequency || e.Value != hz || e.Max != MaxEncodableFreqHz {
			t.Errorf("*OutOfDomainError = %+v, want {frequency, %d, %d}", e, hz, uint64(MaxEncodableFreqHz))
		}
		if len(res.Steps) != 0 {
			t.Errorf("Steps = %+v, want empty - the refusal precedes the frame being built", res.Steps)
		}
		if frames := framesAfterOpen(t, p, openFrames); len(frames) != 0 {
			t.Errorf("%d frames reached the port, want none", len(frames))
		}
	}
	// The ceiling itself is admissible.
	s, _ := writeSession(t, ackingRadio())
	ch := goodChannel("042")
	ch.Data.FreqHz = MaxEncodableFreqHz
	if _, err := s.WriteChannel(t.Context(), ch); err != nil {
		t.Errorf("the declared ceiling %d Hz was refused: %v", uint64(MaxEncodableFreqHz), err)
	}
}

// TestWriteChannel_RefusesAToneOutsideThePrintedDigitRange: matrix §1
// row 8 / PDF p.14's six rotated nibble labels (100Hz: 0-2, 10 Hz: 0-9,
// 1 Hz: 0-9, 0.1 Hz: 0-9).
func TestWriteChannel_RefusesAToneOutsideThePrintedDigitRange(t *testing.T) {
	for _, tt := range []struct {
		name  string
		field spec.Field
		set   func(*codeplug.ChannelData, spec.Tone)
	}{
		{"ToneTx", spec.FieldToneTx, func(d *codeplug.ChannelData, v spec.Tone) {
			d.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: v}
		}},
		{"ToneRx", spec.FieldToneRx, func(d *codeplug.ChannelData, v spec.Tone) {
			d.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: v}
		}},
	} {
		for _, v := range []spec.Tone{3000, 9999} {
			t.Run(tt.name, func(t *testing.T) {
				s, p := writeSession(t, ackingRadio())
				openFrames := len(p.Transcript())
				ch := goodChannel("042")
				tt.set(ch.Data, v)

				_, err := s.WriteChannel(t.Context(), ch)
				var e *OutOfDomainError
				if !errors.As(err, &e) {
					t.Fatalf("%s = %d gave %v, want an *OutOfDomainError", tt.name, v, err)
				}
				if e.Field != tt.field || e.Value != uint64(v) || e.Max != MaxToneDeciHz {
					t.Errorf("*OutOfDomainError = %+v, want {%s, %d, %d}", e, tt.field, v, uint64(MaxToneDeciHz))
				}
				if frames := framesAfterOpen(t, p, openFrames); len(frames) != 0 {
					t.Errorf("%d frames reached the port, want none", len(frames))
				}
			})
		}
	}
}

// TestNumericRefusalIsDefenceInDepthNotTheGate is a DOCUMENTATION PIN WITH
// A REAL ASSERTION: it builds a 1A 00 set frame by hand carrying
// 70 000 000 Hz and asserts that the outbound gate ADMITS it.
//
// THAT IS THE DEFERRED GATE-DOMAIN GAP. civ.FieldSpan has no numeric
// domain and civ's validateSpanValue checks only BCD width and scale, so
// the refusals in this file sit in the DRIVER rather than at the last
// defence. codeplug.Validate bounds the primary frequency and enabler E3
// bounds tones, so every path through the model layer is already covered
// — which is one of the three grounds the orchestrator's 24/08/2026
// deferral rests on.
//
// WHEN THE POST-WAVE-3 FOLLOW-UP CLOSES THE GAP, THIS TEST IS THE ONE THAT
// FLIPS — and flipping it is a visible, reviewable change, which is the
// second ground. DO NOT DELETE THIS TEST TO "FIX" IT.
func TestNumericRefusalIsDefenceInDepthNotTheGate(t *testing.T) {
	// 70 000 000 Hz in this record's five-byte LITTLE-endian packed BCD:
	// least significant pair first, so 00 00 00 70 00. The 10 MHz digit's
	// printed range is 0-6, so the 7 is exactly the out-of-domain digit.
	overRange := append([]byte(nil), goldenRecord...)
	copy(overRange[1:6], []byte{0x00, 0x00, 0x00, 0x70, 0x00})

	frame := append([]byte{0xFE, 0xFE, 0x98, 0xE0, 0x1A, 0x00, 0x00, 0x42}, overRange...)
	frame = append(frame, 0xFD)

	if !civic7610.Profile().AllowedCommand(frame) {
		t.Fatalf("the gate REFUSED a set carrying 70 MHz.\n"+
			"That is a GOOD change and this test is the one that flips when it lands: civ.FieldSpan has\n"+
			"gained a numeric domain, or validateSpanValue has learned to bound one, and this driver's\n"+
			"pre-build refusals are no longer the only thing standing between an out-of-domain value and\n"+
			"the wire. Rewrite this test to assert the REFUSAL, and update doc.go's deferred-gap section.\n"+
			"frame: %s", hexFrames([][]byte{frame}))
	}
	// And the driver refuses it anyway — which is the whole point of
	// defence in depth.
	s, p := writeSession(t, ackingRadio())
	openFrames := len(p.Transcript())
	ch := goodChannel("042")
	ch.Data.FreqHz = 70_000_000
	if _, err := s.WriteChannel(t.Context(), ch); !errors.Is(err, ErrOutOfDomain) {
		t.Errorf("the DRIVER admitted 70 MHz too (%v); the gate does not bound it, so this refusal is the only one there is", err)
	}
	if frames := framesAfterOpen(t, p, openFrames); len(frames) != 0 {
		t.Errorf("%d frames reached the port, want none", len(frames))
	}
}

// TestRequestedFields_BaseMembershipAndOrder: for a ChannelData with NO
// conditional Known, the result is exactly the seven base fields in that
// order.
//
// WHY EACH OMITTED FIELD IS OMITTED: clarifier, ctcss_state, ctcss_tone,
// shift, tag_display, tx_frequency, duplex, offset, dtcs_code and
// dtcs_polarity because THE RECORD HAS NO SUCH FIELD; scan_skip and
// data_mode because RULING E6 UNMAPS THEIR NIBBLES; erase because it is
// not a state-bearing member of ChannelData at all (Data == nil is the
// discriminator).
func TestRequestedFields_BaseMembershipAndOrder(t *testing.T) {
	want := []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldTag,
		spec.FieldToneMode,
		spec.FieldToneTx,
		spec.FieldToneRx,
		spec.FieldFilter,
	}
	got := requestedFields(*goodChannel("042").Data)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("requestedFields = %v, want %v", got, want)
	}
	// The Yaesu trio the FTdx101's base set carries appears nowhere.
	for _, absent := range []spec.Field{spec.FieldClarifier, spec.FieldCTCSSState, spec.FieldShift} {
		if containsField(got, absent) {
			t.Errorf("the base set carries %s; this record has no such field (R6-COMPLETION: the base set is THIS model's, from its own matrix §2)", absent)
		}
	}
}

// TestRequestedFields_EveryConditionalAppearsWhenKnown is table-driven
// over EVERY ONE of the twelve conditionals.
//
// A FIELD MISSING FROM THAT TABLE IS A FIELD THE GATE WOULD NEVER SEE, and
// therefore a Known value that would be silently dropped rather than
// refused.
func TestRequestedFields_EveryConditionalAppearsWhenKnown(t *testing.T) {
	base := len(requestedFields(*goodChannel("042").Data))
	for _, tt := range conditionalCases() {
		t.Run(string(tt.field), func(t *testing.T) {
			d := *goodChannel("042").Data
			tt.set(&d)
			got := requestedFields(d)
			if !containsField(got, tt.field) {
				t.Fatalf("requestedFields = %v, want it to include %s", got, tt.field)
			}
			if len(got) <= base {
				t.Errorf("requestedFields = %v, want the base set PLUS the conditional", got)
			}
			for i := 0; i < base; i++ {
				if got[i] == tt.field {
					t.Errorf("%s appears inside the base set; conditionals are appended AFTER it", tt.field)
				}
			}
		})
	}
	if n := len(conditionalCases()); n != len(conditionalRequestedFields) {
		t.Errorf("this test covers %d conditionals and the table has %d - every entry must be exercised", n, len(conditionalRequestedFields))
	}
}

// TestWriteChannel_EveryConditionalIsRefusedByTheGate is the Wave-1 C2
// class: a DIRECT Session.WriteChannel call carrying each conditional
// field Known is refused with ErrWriteRefused NAMING IT, with ZERO BYTES
// WRITTEN. This is what proves membership of requestedFields is not
// cosmetic.
func TestWriteChannel_EveryConditionalIsRefusedByTheGate(t *testing.T) {
	for _, tt := range conditionalCases() {
		t.Run(string(tt.field), func(t *testing.T) {
			s, p := writeSession(t, ackingRadio())
			openFrames := len(p.Transcript())
			ch := goodChannel("042")
			tt.set(ch.Data)

			res, err := s.WriteChannel(t.Context(), ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("err = %v, want ErrWriteRefused for a Known %s", err, tt.field)
			}
			var refusal *driver.WriteRefusedError
			if !errors.As(err, &refusal) {
				t.Fatalf("err = %v, want a *driver.WriteRefusedError", err)
			}
			if !containsField(refusal.Fields, tt.field) {
				t.Errorf("the refusal names %v, want it to name %s", refusal.Fields, tt.field)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty", res.Steps)
			}
			if frames := framesAfterOpen(t, p, openFrames); len(frames) != 0 {
				t.Errorf("%d frames reached the port, want none:\n  %s", len(frames), hexFrames(frames))
			}
		})
	}
}

// conditionalCases sets each of the twelve conditional fields Known in
// turn. Deliberately written out rather than derived from
// conditionalRequestedFields: a table generated from the code under test
// would agree with a missing entry as happily as a present one, and the
// count check above is what binds the two.
func conditionalCases() []struct {
	field spec.Field
	set   func(*codeplug.ChannelData)
} {
	return []struct {
		field spec.Field
		set   func(*codeplug.ChannelData)
	}{
		{spec.FieldClarifier, func(d *codeplug.ChannelData) { d.ClarHz = 100 }},
		{spec.FieldCTCSSState, func(d *codeplug.ChannelData) { d.CTCSS = "ENC" }},
		{spec.FieldCTCSSTone, func(d *codeplug.ChannelData) {
			d.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 885}
		}},
		{spec.FieldShift, func(d *codeplug.ChannelData) { d.Shift = "PLUS" }},
		{spec.FieldTagDisplay, func(d *codeplug.ChannelData) {
			d.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: true}
		}},
		{spec.FieldScanSkip, func(d *codeplug.ChannelData) {
			d.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
		}},
		{spec.FieldTxFrequency, func(d *codeplug.ChannelData) {
			d.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: 14_300_000}
		}},
		{spec.FieldDuplex, func(d *codeplug.ChannelData) {
			d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "DUP+"}
		}},
		{spec.FieldOffset, func(d *codeplug.ChannelData) {
			d.OffsetHz = codeplug.FreqField{State: codeplug.Known, Value: 600_000}
		}},
		{spec.FieldDTCSCode, func(d *codeplug.ChannelData) {
			d.DTCSCode = codeplug.IntField{State: codeplug.Known, Value: 23}
		}},
		{spec.FieldDTCSPolarity, func(d *codeplug.ChannelData) {
			d.DTCSPolarity = codeplug.StringField{State: codeplug.Known, Value: "NN"}
		}},
		{spec.FieldDataMode, func(d *codeplug.ChannelData) {
			d.DataMode = codeplug.BoolField{State: codeplug.Known, Value: true}
		}},
	}
}

// TestWriteChannel_RefusesANonKnownMandatoryField — Codex M5's fix.
//
// Left as REV 2 had it, the landed BUILDER would have been the first
// detector: civ's encodeRecord returns a codec error for an absent mapped
// field, and by then the preservation read has already put traffic on the
// wire. T5 forbids that, so this rung is LOCALLY DECIDABLE and fires
// FIRST — which is why the byte count below is ZERO and not "one read's
// worth". That is the T5 ordering made observable.
func TestWriteChannel_RefusesANonKnownMandatoryField(t *testing.T) {
	for _, tt := range []struct {
		name  string
		field spec.Field
		set   func(*codeplug.ChannelData)
	}{
		{"mode is empty", spec.FieldMode, func(d *codeplug.ChannelData) { d.Mode = "" }},
		{"filter is Unknown", spec.FieldFilter, func(d *codeplug.ChannelData) {
			d.Filter = codeplug.StringField{State: codeplug.Unknown}
		}},
		{"filter is Unavailable", spec.FieldFilter, func(d *codeplug.ChannelData) {
			d.Filter = codeplug.StringField{State: codeplug.Unavailable}
		}},
		{"tone_mode is Unknown", spec.FieldToneMode, func(d *codeplug.ChannelData) {
			d.ToneMode = codeplug.StringField{State: codeplug.Unknown}
		}},
		{"tone_mode is Unavailable", spec.FieldToneMode, func(d *codeplug.ChannelData) {
			d.ToneMode = codeplug.StringField{State: codeplug.Unavailable}
		}},
		{"tag carries a byte outside the charset", spec.FieldTag, func(d *codeplug.ChannelData) { d.Tag = "HOME\tQTH" }},
		{"tag is longer than the name span", spec.FieldTag, func(d *codeplug.ChannelData) { d.Tag = "ELEVENCHARS" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, p := writeSession(t, ackingRadio())
			openFrames := len(p.Transcript())
			ch := goodChannel("042")
			tt.set(ch.Data)

			res, err := s.WriteChannel(t.Context(), ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("err = %v, want ErrWriteRefused", err)
			}
			var refusal *driver.WriteRefusedError
			if !errors.As(err, &refusal) || !containsField(refusal.Fields, tt.field) {
				t.Errorf("err = %v, want a *driver.WriteRefusedError naming %s", err, tt.field)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty", res.Steps)
			}
			if frames := framesAfterOpen(t, p, openFrames); len(frames) != 0 {
				t.Errorf("%d frames reached the port, want ZERO - this rung is locally decidable and precedes the preservation read (T5)", len(frames))
			}
		})
	}
	// FREQUENCY has no "not Known" arm: ChannelData.FreqHz is a plain
	// uint64 that always carries a value, so its analogue is rung 4's
	// numeric domain — asserted by
	// TestWriteChannel_RefusesAFrequencyOutsideTheEncodableRange. Said out
	// loud so the omission reads as a decision.
}

// TestWriteChannel_UpdateCarriesTheJustReadToneWhenNotKnown — tier ruling
// T1(4).
//
// THIS IS PRESERVATION, NOT SYNTHESIS. The value came from the radio, in
// the read the E6/T5 rung already required, so nothing is invented — and
// it is the reason ReadChannel mapping an OUT-OF-DOMAIN tone to Unknown
// does not make that channel unwritable: the number goes back exactly as
// it came, 0 included.
func TestWriteChannel_UpdateCarriesTheJustReadToneWhenNotKnown(t *testing.T) {
	for _, tt := range []struct {
		name  string
		prior []byte
		want  []byte // the six tone bytes the set must carry
	}{
		{"the golden record's 885 and 1000", goldenRecord, []byte{0x00, 0x08, 0x85, 0x00, 0x10, 0x00}},
		{"an out-of-domain 0, preserved verbatim", func() []byte {
			r := withRecord(9, 0x00, 0x00, 0x00)
			copy(r[12:], []byte{0x00, 0x00, 0x00})
			return r
		}(), []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		{"an out-of-domain 2999 boundary neighbour", func() []byte {
			r := withRecord(9, 0x00, 0x29, 0x99)
			copy(r[12:], []byte{0x00, 0x00, 0x01})
			return r
		}(), []byte{0x00, 0x29, 0x99, 0x00, 0x00, 0x01}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			img := ackingRadio()
			img.records[42] = tt.prior
			s, p := writeSession(t, img)
			openFrames := len(p.Transcript())

			ch := goodChannel("042")
			ch.Data.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
			ch.Data.ToneRx = codeplug.ToneField{State: codeplug.Unavailable}

			if _, err := s.WriteChannel(t.Context(), ch); err != nil {
				t.Fatalf("WriteChannel: %v", err)
			}
			frames := framesAfterOpen(t, p, openFrames)
			if len(frames) != 2 {
				t.Fatalf("the write put %d frames on the wire, want 2:\n  %s", len(frames), hexFrames(frames))
			}
			rec := frames[1][8 : len(frames[1])-1]
			if got := rec[9:15]; !bytes.Equal(got, tt.want) {
				t.Errorf("the set carried tone bytes % x, want % x verbatim from the just-read record", got, tt.want)
			}
		})
	}
}

// TestWriteChannel_CreateWithoutToneIsRefused — tier ruling T1(5).
//
// The preservation read is rejected (an empty slot), so the tone spans
// have no source, and THIS MODEL HAS NO DOCUMENTED DEFAULT TONE TO FALL
// BACK ON — which is exactly why T1(5)'s other arm does not apply here.
// Register entry ic7610-default-tone-undocumented.
func TestWriteChannel_CreateWithoutToneIsRefused(t *testing.T) {
	for _, tt := range []struct {
		name  string
		field spec.Field
		set   func(*codeplug.ChannelData)
	}{
		{"ToneTx Unknown", spec.FieldToneTx, func(d *codeplug.ChannelData) {
			d.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
		}},
		{"ToneTx Unavailable", spec.FieldToneTx, func(d *codeplug.ChannelData) {
			d.ToneTx = codeplug.ToneField{State: codeplug.Unavailable}
		}},
		{"ToneRx Unknown", spec.FieldToneRx, func(d *codeplug.ChannelData) {
			d.ToneRx = codeplug.ToneField{State: codeplug.Unknown}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			img := ackingRadio()
			delete(img.records, 42) // an empty slot: answered FA
			s, p := writeSession(t, img)
			openFrames := len(p.Transcript())
			ch := goodChannel("042")
			tt.set(ch.Data)

			res, err := s.WriteChannel(t.Context(), ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("err = %v, want ErrWriteRefused", err)
			}
			var refusal *driver.WriteRefusedError
			if !errors.As(err, &refusal) || !containsField(refusal.Fields, tt.field) {
				t.Fatalf("err = %v, want a *driver.WriteRefusedError naming %s", err, tt.field)
			}
			if !strings.Contains(refusal.Reason, "ic7610-default-tone-undocumented") {
				t.Errorf("the refusal does not cite its register entry: %q", refusal.Reason)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty", res.Steps)
			}
			// ONE read's worth, and no set frame: this rung is
			// read-dependent, so it costs the T5 exception's exchange.
			frames := framesAfterOpen(t, p, openFrames)
			if len(frames) != 1 {
				t.Errorf("the refused CREATE put %d frames on the wire, want exactly 1 (the preservation read):\n  %s", len(frames), hexFrames(frames))
			}
		})
	}
}

// TestWriteChannel_CreateWithEveryValueSucceeds: the same empty slot, with
// all seven mapped fields Known, writes normally against the Fixed
// template.
func TestWriteChannel_CreateWithEveryValueSucceeds(t *testing.T) {
	img := ackingRadio()
	delete(img.records, 42)
	s, p := writeSession(t, img)
	openFrames := len(p.Transcript())

	res, err := s.WriteChannel(t.Context(), goodChannel("042"))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want a sent, confirmed write", res.Steps)
	}
	frames := framesAfterOpen(t, p, openFrames)
	if len(frames) != 2 {
		t.Fatalf("the write put %d frames on the wire, want 2:\n  %s", len(frames), hexFrames(frames))
	}
	if got := frames[1][8 : len(frames[1])-1]; !bytes.Equal(got, goldenRecord) {
		t.Errorf("the set carried % x, want the golden record % x", got, goldenRecord)
	}
}

// TestWriteChannel_ScanEdgesWriteThroughTheSameForm: P1 and P2 are two
// more values of the same selector (matrix §3.15(d)), so a write to one
// is the same 34-byte set at a different address.
func TestWriteChannel_ScanEdgesWriteThroughTheSameForm(t *testing.T) {
	for _, tt := range []struct {
		slot string
		hi   byte
		lo   byte
	}{{"P1", 0x01, 0x00}, {"P2", 0x01, 0x01}} {
		t.Run(tt.slot, func(t *testing.T) {
			img := ackingRadio()
			a, _, _ := slotToAddress(tt.slot)
			img.records[a.Channel] = goldenRecord
			s, p := writeSession(t, img)
			openFrames := len(p.Transcript())

			if _, err := s.WriteChannel(t.Context(), goodChannel(tt.slot)); err != nil {
				t.Fatalf("WriteChannel %s: %v", tt.slot, err)
			}
			frames := framesAfterOpen(t, p, openFrames)
			if len(frames) != 2 {
				t.Fatalf("the write put %d frames on the wire, want 2", len(frames))
			}
			set := frames[1]
			if set[6] != tt.hi || set[7] != tt.lo {
				t.Errorf("the set addressed %02x %02x, want %02x %02x", set[6], set[7], tt.hi, tt.lo)
			}
		})
	}
}

func containsField(fields []spec.Field, f spec.Field) bool {
	for _, x := range fields {
		if x == f {
			return true
		}
	}
	return false
}
