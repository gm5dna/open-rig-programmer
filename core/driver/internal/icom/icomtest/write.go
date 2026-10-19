// SPDX-License-Identifier: GPL-3.0-or-later

package icomtest

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

func (fx Fixture) GoodChannel(slot string) codeplug.Channel {
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
func (fx Fixture) writeSession(t *testing.T, img RadioImage) (Session, *ScriptedPort) {
	t.Helper()
	p := fx.port(t, img)
	return fx.openWith(t, p), p
}
func (fx Fixture) FramesAfterOpen(t *testing.T, p *ScriptedPort, openFrames int) [][]byte {
	t.Helper()
	all := p.Transcript()
	if len(all) < openFrames {
		t.Fatalf("the transcript has %d frames, fewer than Open's %d", len(all), openFrames)
	}
	return all[openFrames:]
}
func (fx Fixture) AckingRadio() RadioImage {
	img := fx.occupiedRadio()
	img.AckSets = true
	img.Records[42] = fx.Golden
	return img
}
func (fx Fixture) tMemorySetSpec_IsWriteWithAckAndNeverRetries(t *testing.T) {
	sp := civ.CIVWriteWithAckSpec(fx.Params.Profile().AcknowledgementMatcher())
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
func (fx Fixture) tWriteChannel_FBIsConfirmed(t *testing.T) {
	s, p := fx.writeSession(t, fx.AckingRadio())
	openFrames := len(p.Transcript())

	res, err := s.WriteChannel(t.Context(), fx.GoodChannel("042"))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	want := []driver.WriteStep{{Command: icom.MemorySetStep, Sent: true, Confirmed: true}}
	if !reflect.DeepEqual(res.Steps, want) {
		t.Errorf("Steps = %+v, want %+v", res.Steps, want)
	}
	frames := fx.FramesAfterOpen(t, p, openFrames)
	if len(frames) != 2 {
		t.Fatalf("the write put %d frames on the wire, want 2 (one read, one set):\n  %s", len(frames), HexFrames(frames))
	}
	if !bytes.Equal(frames[0], fx.memReadFrame(42)) {
		t.Errorf("the first frame was %s, want the preservation read", HexFrames(frames[:1]))
	}
	if len(frames[1]) != MemSetFrameLen {
		t.Errorf("the set frame is %d bytes, want %d (6 envelope + 2 address + 25 record + 1 terminator)", len(frames[1]), MemSetFrameLen)
	}
}
func (fx Fixture) tWriteChannel_AckTimeoutReportsAnUnknownOutcome(t *testing.T) {
	img := fx.AckingRadio()
	img.AckSets = false
	s, p := fx.writeSession(t, img)
	openFrames := len(p.Transcript())

	res, err := s.WriteChannel(t.Context(), fx.GoodChannel("042"))
	if err == nil {
		t.Fatal("WriteChannel succeeded with no acknowledgement; silence is not success on an acknowledged write")
	}
	want := []driver.WriteStep{{Command: icom.MemorySetStep, Sent: false, Confirmed: false}}
	if !reflect.DeepEqual(res.Steps, want) {
		t.Errorf("Steps = %+v, want %+v - an ack timeout leaves the outcome UNKNOWN", res.Steps, want)
	}
	var sets int
	for _, f := range fx.FramesAfterOpen(t, p, openFrames) {
		if len(f) >= MemSetFrameLen {
			sets++
		}
	}
	if sets != 1 {
		t.Errorf("%d set frames reached the port, want exactly 1 - an acknowledged write is still a WRITE and is never resent", sets)
	}
}
func (fx Fixture) tWriteChannel_FARejectionIsReported(t *testing.T) {
	t.Run(fmt.Sprintf("FA from 0x%02X is a rejection", fx.Addr), func(t *testing.T) {
		img := fx.AckingRadio()
		img.AckSets = false
		img.RejectSets = true
		s, _ := fx.writeSession(t, img)
		res, err := s.WriteChannel(t.Context(), fx.GoodChannel("042"))
		if !errors.Is(err, transport.ErrRejected) {
			t.Fatalf("err = %v, want one satisfying errors.Is(err, transport.ErrRejected)", err)
		}
		if !strings.Contains(err.Error(), icom.MemorySetStep) {
			t.Errorf("the rejection %q does not name the command", err)
		}
		want := []driver.WriteStep{{Command: icom.MemorySetStep, Sent: true, Confirmed: false}}
		if !reflect.DeepEqual(res.Steps, want) {
			t.Errorf("Steps = %+v, want %+v", res.Steps, want)
		}
	})

	t.Run("FA from another station is not a rejection", func(t *testing.T) {
		img := fx.AckingRadio()
		img.AckSets = false
		img.RejectSets = true
		img.SetFrom = 0x94
		s, _ := fx.writeSession(t, img)
		_, err := s.WriteChannel(t.Context(), fx.GoodChannel("042"))
		if err == nil {
			t.Fatal("the write succeeded on another station's FA")
		}
		if errors.Is(err, transport.ErrRejected) {
			t.Errorf("err = %v, want a TIMEOUT: E1's IsRejection is source-address-checked, so an FA from 0x94 leaves the wait running rather than ending it", err)
		}
	})
}
func (fx Fixture) tWriteChannel_UnverifiedRefusesBeforeAnyWire(t *testing.T) {
	p := fx.port(t, fx.AckingRadio())
	d := fx.New(true, false)
	sess, err := d.Open(t.Context(), p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = sess.Close() }()
	openFrames := len(p.Transcript())

	res, err := sess.WriteChannel(t.Context(), fx.GoodChannel("042"))
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("err = %v, want one satisfying errors.Is(err, driver.ErrWriteRefused)", err)
	}
	if len(res.Steps) != 0 || res.Steps == nil {
		t.Errorf("Steps = %+v, want an EMPTY non-nil slice - a refusal before any frame is built has no sequence to describe", res.Steps)
	}
	if frames := fx.FramesAfterOpen(t, p, openFrames); len(frames) != 0 {
		t.Errorf("%d frames reached the port:\n  %s\nwant NONE - a locally decidable refusal precedes ALL wire traffic (T5)", len(frames), HexFrames(frames))
	}
}
func (fx Fixture) tWriteChannel_KnownUnmappedFieldIsRefusedNotDropped(t *testing.T) {
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
			s, p := fx.writeSession(t, fx.AckingRadio())
			openFrames := len(p.Transcript())
			ch := fx.GoodChannel("042")
			tt.set(ch.Data)

			res, err := s.WriteChannel(t.Context(), ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("err = %v, want ErrWriteRefused - a Known unmapped field must be REFUSED, not dropped", err)
			}
			var refusal *driver.WriteRefusedError
			if !errors.As(err, &refusal) {
				t.Fatalf("err = %v, want a *driver.WriteRefusedError", err)
			}
			if !ContainsField(refusal.Fields, tt.field) {
				t.Errorf("the refusal names %v, want it to name %s", refusal.Fields, tt.field)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty", res.Steps)
			}
			if frames := fx.FramesAfterOpen(t, p, openFrames); len(frames) != 0 {
				t.Errorf("%d frames reached the port, want none", len(frames))
			}
		})
	}
}
func (fx Fixture) tWriteChannel_RefusesASlotWhoseUnmappedRegionsDiffer(t *testing.T) {
	for _, row := range fx.DifferRows {
		tt := struct {
			name   string
			prior  []byte
			offset int
			nibble string
			got    byte
		}{row.Name, fx.withRecord(row.Off, row.Val), row.Offset, row.Nibble, row.Got}
		t.Run(tt.name, func(t *testing.T) {
			img := fx.AckingRadio()
			img.Records[42] = tt.prior
			s, p := fx.writeSession(t, img)
			openFrames := len(p.Transcript())

			res, err := s.WriteChannel(t.Context(), fx.GoodChannel("042"))
			if !errors.Is(err, driver.ErrUnmappedRegion) {
				t.Fatalf("err = %v, want one satisfying errors.Is(err, driver.ErrUnmappedRegion)", err)
			}
			e := fx.AsUnmapped(err)
			if e == nil {
				t.Fatalf("err = %v, want an UnmappedRegionError", err)
			}
			if e.Offset != tt.offset || e.Nibble != tt.nibble || e.Want != 0 || e.Got != tt.got {
				t.Errorf("UnmappedRegionError = %+v, want {Offset: %d, Nibble: %q, Want: 0, Got: %#x}", e, tt.offset, tt.nibble, tt.got)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty - no frame was ever built", res.Steps)
			}

			frames := fx.FramesAfterOpen(t, p, openFrames)
			if len(frames) != 1 {
				t.Fatalf("the refused write put %d frames on the wire, want exactly 1 (the T5 preservation read):\n  %s", len(frames), HexFrames(frames))
			}
			if !bytes.Equal(frames[0], fx.memReadFrame(42)) {
				t.Errorf("the one frame was %s, want the preservation read", HexFrames(frames))
			}
		})
	}
}
func (fx Fixture) tWriteChannel_EmptySlotWritesAgainstTheTemplate(t *testing.T) {
	img := fx.AckingRadio()
	delete(img.Records, 42)
	s, p := fx.writeSession(t, img)
	openFrames := len(p.Transcript())

	res, err := s.WriteChannel(t.Context(), fx.GoodChannel("042"))
	if err != nil {
		t.Fatalf("WriteChannel into an empty slot: %v", err)
	}
	if !res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want a confirmed write", res.Steps)
	}
	frames := fx.FramesAfterOpen(t, p, openFrames)
	if len(frames) != 2 {
		t.Fatalf("the write put %d frames on the wire, want 2:\n  %s", len(frames), HexFrames(frames))
	}
	rec := frames[1][8 : len(frames[1])-1]
	tmpl := fx.Params.Template
	if tmpl == nil {
		tmpl = make([]byte, len(rec))
	}
	for _, r := range fx.Params.Unmapped {
		mask := map[string]byte{"whole": 0xFF, "high": 0xF0, "low": 0x0F}[r.Nibble]
		if rec[r.Offset]&mask != tmpl[r.Offset]&mask {
			t.Errorf("the set's byte %d (%s) is %#02x, want the template's %#02x", r.Offset, r.Nibble, rec[r.Offset]&mask, tmpl[r.Offset]&mask)
		}
	}
}
func (fx Fixture) tWriteChannel_ConsentedWritesTheFullRecord(t *testing.T) {
	p := fx.port(t, fx.AckingRadio())
	d := fx.New(true, true)
	sess, err := d.Open(t.Context(), p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = sess.Close() }()
	openFrames := len(p.Transcript())

	if _, err := sess.WriteChannel(t.Context(), fx.GoodChannel("042")); err != nil {
		t.Fatalf("WriteChannel under consent: %v", err)
	}
	frames := fx.FramesAfterOpen(t, p, openFrames)
	if len(frames) != 2 {
		t.Fatalf("the write put %d frames on the wire, want 2:\n  %s", len(frames), HexFrames(frames))
	}
	set := frames[1]
	if len(set) != MemSetFrameLen {
		t.Errorf("the set frame is %d bytes, want %d", len(set), MemSetFrameLen)
	}
	want := append([]byte{0xFE, 0xFE, fx.Addr, 0xE0, 0x1A, 0x00, 0x00, 0x42}, fx.Golden...)
	want = append(want, 0xFD)
	if !bytes.Equal(set, want) {
		t.Errorf("the set frame was\n  %s\nwant\n  %s", HexFrames([][]byte{set}), HexFrames([][]byte{want}))
	}
}
func (fx Fixture) tWriteChannel_EraseIsRefused(t *testing.T) {
	for _, tt := range []struct {
		name string
		mk   func() driver.Driver
	}{
		{"RealHardware", func() driver.Driver { return fx.New(true, false) }},
		{"RealHardware + consent", func() driver.Driver { return fx.New(true, true) }},
		{"Simulated", func() driver.Driver { return fx.New(false, false) }},
		{"Simulated + consent", func() driver.Driver { return fx.New(false, true) }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := fx.port(t, fx.AckingRadio())
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
			if errors.As(err, &refusal) && !ContainsField(refusal.Fields, spec.FieldErase) {
				t.Errorf("the refusal names %v, want it to name erase", refusal.Fields)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty", res.Steps)
			}
			if frames := fx.FramesAfterOpen(t, p, openFrames); len(frames) != 0 {
				t.Errorf("%d frames reached the port, want none", len(frames))
			}
		})
	}
}
func (fx Fixture) tWriteChannel_NoClearFrameIsReachable(t *testing.T) {
	isClearForm := func(f []byte) bool {
		if len(f) == 6 && f[4] == 0x0B {
			return true
		}
		return len(f) == 10 && f[4] == 0x1A && f[5] == 0x00 && f[8] == 0xFF
	}

	inputs := []codeplug.Channel{
		fx.GoodChannel("001"),
		fx.GoodChannel("P1"),
		{Slot: "001", Data: nil},
		{Slot: "042", Data: &codeplug.ChannelData{}},
		func() codeplug.Channel {
			ch := fx.GoodChannel("042")
			ch.Data.Tag = ""
			return ch
		}(),
		func() codeplug.Channel {
			ch := fx.GoodChannel("042")
			ch.Data.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
			ch.Data.ToneRx = codeplug.ToneField{State: codeplug.Unavailable}
			return ch
		}(),
	}
	for _, mk := range []func() driver.Driver{
		func() driver.Driver { return fx.New(true, false) },
		func() driver.Driver { return fx.New(true, true) },
		func() driver.Driver { return fx.New(false, false) },
		func() driver.Driver { return fx.New(false, true) },
	} {
		p := fx.port(t, fx.AckingRadio())
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
				t.Errorf("a clear frame reached the port: %s", HexFrames([][]byte{f}))
			}
		}
	}
}
func (fx Fixture) tWriteChannel_RefusesAFrequencyOutsideTheEncodableRange(t *testing.T) {
	for _, hz := range []uint64{70_000_000, 100_000_000, 1 << 40} {
		s, p := fx.writeSession(t, fx.AckingRadio())
		openFrames := len(p.Transcript())
		ch := fx.GoodChannel("042")
		ch.Data.FreqHz = hz

		res, err := s.WriteChannel(t.Context(), ch)
		e := fx.AsDomain(err)
		if e == nil {
			t.Fatalf("%d Hz gave %v, want an OutOfDomainError", hz, err)
		}
		if e.Field != spec.FieldFrequency || e.Value != hz || e.Max != fx.MaxFreqHz {
			t.Errorf("OutOfDomainError = %+v, want {frequency, %d, %d}", e, hz, fx.MaxFreqHz)
		}
		if len(res.Steps) != 0 {
			t.Errorf("Steps = %+v, want empty - the refusal precedes the frame being built", res.Steps)
		}
		if frames := fx.FramesAfterOpen(t, p, openFrames); len(frames) != 0 {
			t.Errorf("%d frames reached the port, want none", len(frames))
		}
	}
	s, _ := fx.writeSession(t, fx.AckingRadio())
	ch := fx.GoodChannel("042")
	ch.Data.FreqHz = fx.MaxFreqHz
	if _, err := s.WriteChannel(t.Context(), ch); err != nil {
		t.Errorf("the declared ceiling %d Hz was refused: %v", fx.MaxFreqHz, err)
	}
}
func (fx Fixture) tWriteChannel_RefusesAToneOutsideThePrintedDigitRange(t *testing.T) {
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
				s, p := fx.writeSession(t, fx.AckingRadio())
				openFrames := len(p.Transcript())
				ch := fx.GoodChannel("042")
				tt.set(ch.Data, v)

				_, err := s.WriteChannel(t.Context(), ch)
				e := fx.AsDomain(err)
				if e == nil {
					t.Fatalf("%s = %d gave %v, want an OutOfDomainError", tt.name, v, err)
				}
				if e.Field != tt.field || e.Value != uint64(v) || e.Max != fx.MaxToneDeciHz {
					t.Errorf("OutOfDomainError = %+v, want {%s, %d, %d}", e, tt.field, v, fx.MaxToneDeciHz)
				}
				if frames := fx.FramesAfterOpen(t, p, openFrames); len(frames) != 0 {
					t.Errorf("%d frames reached the port, want none", len(frames))
				}
			})
		}
	}
}
func (fx Fixture) tNumericRefusalIsDefenceInDepthNotTheGate(t *testing.T) {
	overRange := append([]byte(nil), fx.Golden...)
	copy(overRange[1:6], []byte{0x00, 0x00, 0x00, 0x70, 0x00})

	frame := append([]byte{0xFE, 0xFE, fx.Addr, 0xE0, 0x1A, 0x00, 0x00, 0x42}, overRange...)
	frame = append(frame, 0xFD)

	if !fx.Params.Profile().AllowedCommand(frame) {
		t.Fatalf("the gate REFUSED a set carrying 70 MHz.\n"+
			"That is a GOOD change and this test is the one that flips when it lands: civ.FieldSpan has\n"+
			"gained a numeric domain, or validateSpanValue has learned to bound one, and this driver's\n"+
			"pre-build refusals are no longer the only thing standing between an out-of-domain value and\n"+
			"the wire. Rewrite this test to assert the REFUSAL, and update doc.go's deferred-gap section.\n"+
			"frame: %s", HexFrames([][]byte{frame}))
	}
	s, p := fx.writeSession(t, fx.AckingRadio())
	openFrames := len(p.Transcript())
	ch := fx.GoodChannel("042")
	ch.Data.FreqHz = 70_000_000
	if _, err := s.WriteChannel(t.Context(), ch); !errors.Is(err, driver.ErrOutOfDomain) {
		t.Errorf("the DRIVER admitted 70 MHz too (%v); the gate does not bound it, so this refusal is the only one there is", err)
	}
	if frames := fx.FramesAfterOpen(t, p, openFrames); len(frames) != 0 {
		t.Errorf("%d frames reached the port, want none", len(frames))
	}
}
func (fx Fixture) tRequestedFields_BaseMembershipAndOrder(t *testing.T) {
	want := []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldTag,
		spec.FieldToneMode,
		spec.FieldToneTx,
		spec.FieldToneRx,
		spec.FieldFilter,
	}
	got := fx.requestedFields(*fx.GoodChannel("042").Data)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("requestedFields = %v, want %v", got, want)
	}
	for _, absent := range []spec.Field{spec.FieldClarifier, spec.FieldCTCSSState, spec.FieldShift} {
		if ContainsField(got, absent) {
			t.Errorf("the base set carries %s; this record has no such field", absent)
		}
	}
}
func (fx Fixture) tRequestedFields_EveryConditionalAppearsWhenKnown(t *testing.T) {
	base := len(fx.requestedFields(*fx.GoodChannel("042").Data))
	for _, tt := range fx.conditionalCases() {
		t.Run(string(tt.field), func(t *testing.T) {
			d := *fx.GoodChannel("042").Data
			tt.set(&d)
			got := fx.requestedFields(d)
			if !ContainsField(got, tt.field) {
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
	if n := len(fx.conditionalCases()); n != len(icom.ConditionalRequestedFields) {
		t.Errorf("this test covers %d conditionals and the table has %d - every entry must be exercised", n, len(icom.ConditionalRequestedFields))
	}
}
func (fx Fixture) tWriteChannel_EveryConditionalIsRefusedByTheGate(t *testing.T) {
	for _, tt := range fx.conditionalCases() {
		t.Run(string(tt.field), func(t *testing.T) {
			s, p := fx.writeSession(t, fx.AckingRadio())
			openFrames := len(p.Transcript())
			ch := fx.GoodChannel("042")
			tt.set(ch.Data)

			res, err := s.WriteChannel(t.Context(), ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("err = %v, want ErrWriteRefused for a Known %s", err, tt.field)
			}
			var refusal *driver.WriteRefusedError
			if !errors.As(err, &refusal) {
				t.Fatalf("err = %v, want a *driver.WriteRefusedError", err)
			}
			if !ContainsField(refusal.Fields, tt.field) {
				t.Errorf("the refusal names %v, want it to name %s", refusal.Fields, tt.field)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty", res.Steps)
			}
			if frames := fx.FramesAfterOpen(t, p, openFrames); len(frames) != 0 {
				t.Errorf("%d frames reached the port, want none:\n  %s", len(frames), HexFrames(frames))
			}
		})
	}
}
func (fx Fixture) conditionalCases() []struct {
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
func (fx Fixture) tWriteChannel_RefusesANonKnownMandatoryField(t *testing.T) {
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
			s, p := fx.writeSession(t, fx.AckingRadio())
			openFrames := len(p.Transcript())
			ch := fx.GoodChannel("042")
			tt.set(ch.Data)

			res, err := s.WriteChannel(t.Context(), ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("err = %v, want ErrWriteRefused", err)
			}
			var refusal *driver.WriteRefusedError
			if !errors.As(err, &refusal) || !ContainsField(refusal.Fields, tt.field) {
				t.Errorf("err = %v, want a *driver.WriteRefusedError naming %s", err, tt.field)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty", res.Steps)
			}
			if frames := fx.FramesAfterOpen(t, p, openFrames); len(frames) != 0 {
				t.Errorf("%d frames reached the port, want ZERO - this rung is locally decidable and precedes the preservation read (T5)", len(frames))
			}
		})
	}
}
func (fx Fixture) tWriteChannel_UpdateCarriesTheJustReadToneWhenNotKnown(t *testing.T) {
	for _, tt := range []struct {
		name  string
		prior []byte
		want  []byte
	}{
		{"the golden record's 885 and 1000", fx.Golden, []byte{0x00, 0x08, 0x85, 0x00, 0x10, 0x00}},
		{"an out-of-domain 0, preserved verbatim", func() []byte {
			r := fx.withRecord(9, 0x00, 0x00, 0x00)
			copy(r[12:], []byte{0x00, 0x00, 0x00})
			return r
		}(), []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}},
		{"an out-of-domain 2999 boundary neighbour", func() []byte {
			r := fx.withRecord(9, 0x00, 0x29, 0x99)
			copy(r[12:], []byte{0x00, 0x00, 0x01})
			return r
		}(), []byte{0x00, 0x29, 0x99, 0x00, 0x00, 0x01}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			img := fx.AckingRadio()
			img.Records[42] = tt.prior
			s, p := fx.writeSession(t, img)
			openFrames := len(p.Transcript())

			ch := fx.GoodChannel("042")
			ch.Data.ToneTx = codeplug.ToneField{State: codeplug.Unknown}
			ch.Data.ToneRx = codeplug.ToneField{State: codeplug.Unavailable}

			if _, err := s.WriteChannel(t.Context(), ch); err != nil {
				t.Fatalf("WriteChannel: %v", err)
			}
			frames := fx.FramesAfterOpen(t, p, openFrames)
			if len(frames) != 2 {
				t.Fatalf("the write put %d frames on the wire, want 2:\n  %s", len(frames), HexFrames(frames))
			}
			rec := frames[1][8 : len(frames[1])-1]
			if got := rec[9:15]; !bytes.Equal(got, tt.want) {
				t.Errorf("the set carried tone bytes % x, want % x verbatim from the just-read record", got, tt.want)
			}
		})
	}
}
func (fx Fixture) tWriteChannel_CreateWithoutToneIsRefused(t *testing.T) {
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
			img := fx.AckingRadio()
			delete(img.Records, 42)
			s, p := fx.writeSession(t, img)
			openFrames := len(p.Transcript())
			ch := fx.GoodChannel("042")
			tt.set(ch.Data)

			res, err := s.WriteChannel(t.Context(), ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("err = %v, want ErrWriteRefused", err)
			}
			var refusal *driver.WriteRefusedError
			if !errors.As(err, &refusal) || !ContainsField(refusal.Fields, tt.field) {
				t.Fatalf("err = %v, want a *driver.WriteRefusedError naming %s", err, tt.field)
			}
			if !strings.Contains(refusal.Reason, fx.DefaultToneEntry) {
				t.Errorf("the refusal does not cite its register entry: %q", refusal.Reason)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty", res.Steps)
			}
			frames := fx.FramesAfterOpen(t, p, openFrames)
			if len(frames) != 1 {
				t.Errorf("the refused CREATE put %d frames on the wire, want exactly 1 (the preservation read):\n  %s", len(frames), HexFrames(frames))
			}
		})
	}
}
func (fx Fixture) tWriteChannel_CreateWithEveryValueSucceeds(t *testing.T) {
	img := fx.AckingRadio()
	delete(img.Records, 42)
	s, p := fx.writeSession(t, img)
	openFrames := len(p.Transcript())

	res, err := s.WriteChannel(t.Context(), fx.GoodChannel("042"))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want a sent, confirmed write", res.Steps)
	}
	frames := fx.FramesAfterOpen(t, p, openFrames)
	if len(frames) != 2 {
		t.Fatalf("the write put %d frames on the wire, want 2:\n  %s", len(frames), HexFrames(frames))
	}
	if got := frames[1][8 : len(frames[1])-1]; !bytes.Equal(got, fx.Golden) {
		t.Errorf("the set carried % x, want the golden record % x", got, fx.Golden)
	}
}
func (fx Fixture) tWriteChannel_ScanEdgesWriteThroughTheSameForm(t *testing.T) {
	for _, tt := range []struct {
		slot string
		hi   byte
		lo   byte
	}{{"P1", 0x01, 0x00}, {"P2", 0x01, 0x01}} {
		t.Run(tt.slot, func(t *testing.T) {
			img := fx.AckingRadio()
			a, _, _ := fx.slotToAddress(tt.slot)
			img.Records[a.Channel] = fx.Golden
			s, p := fx.writeSession(t, img)
			openFrames := len(p.Transcript())

			if _, err := s.WriteChannel(t.Context(), fx.GoodChannel(tt.slot)); err != nil {
				t.Fatalf("WriteChannel %s: %v", tt.slot, err)
			}
			frames := fx.FramesAfterOpen(t, p, openFrames)
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

func ContainsField(fields []spec.Field, f spec.Field) bool {
	for _, x := range fields {
		if x == f {
			return true
		}
	}
	return false
}
func (fx Fixture) tWriteChannel_RefusesAValueOutsideThisRadiosVocabulary(t *testing.T) {
	for _, tt := range []struct {
		name  string
		field spec.Field
		set   func(*codeplug.ChannelData)
	}{
		{"mode FM-N is a Yaesu spelling this radio has no code for", spec.FieldMode, func(d *codeplug.ChannelData) { d.Mode = "FM-N" }},
		{"mode DV belongs to another Icom, not this one", spec.FieldMode, func(d *codeplug.ChannelData) { d.Mode = "DV" }},
		{"mode differs only in case", spec.FieldMode, func(d *codeplug.ChannelData) { d.Mode = "usb" }},
		{"filter FIL9 is outside the three printed values", spec.FieldFilter, func(d *codeplug.ChannelData) {
			d.Filter = codeplug.StringField{State: codeplug.Known, Value: "FIL9"}
		}},
		{"filter is Known but empty", spec.FieldFilter, func(d *codeplug.ChannelData) {
			d.Filter = codeplug.StringField{State: codeplug.Known, Value: ""}
		}},
		{"tone_mode DTCS is printed nowhere in this document", spec.FieldToneMode, func(d *codeplug.ChannelData) {
			d.ToneMode = codeplug.StringField{State: codeplug.Known, Value: "DTCS"}
		}},
		{"tone_mode TSQL-R is a vocabulary this radio does not carry", spec.FieldToneMode, func(d *codeplug.ChannelData) {
			d.ToneMode = codeplug.StringField{State: codeplug.Known, Value: "TSQL-R"}
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, p := fx.writeSession(t, fx.AckingRadio())
			openFrames := len(p.Transcript())
			ch := fx.GoodChannel("042")
			tt.set(ch.Data)

			res, err := s.WriteChannel(t.Context(), ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("err = %v, want ErrWriteRefused - a value this radio cannot express is a REFUSAL, and a locally decidable one", err)
			}
			var refusal *driver.WriteRefusedError
			if !errors.As(err, &refusal) || !ContainsField(refusal.Fields, tt.field) {
				t.Fatalf("err = %v, want a *driver.WriteRefusedError naming %s", err, tt.field)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty", res.Steps)
			}
			if frames := fx.FramesAfterOpen(t, p, openFrames); len(frames) != 0 {
				t.Errorf("%d frames reached the port:\n  %s\nwant ZERO - T5 names vocabularies among the refusals that precede ALL wire traffic, so the builder must never be the first detector", len(frames), HexFrames(frames))
			}
		})
	}
	t.Run("every printed value is admitted", func(t *testing.T) {
		caps := fx.Unverified()
		for _, mode := range caps.Modes {
			for _, filter := range caps.Filters {
				for _, tm := range caps.ToneModes {
					d := *fx.GoodChannel("042").Data
					d.Mode = mode
					d.Filter = codeplug.StringField{State: codeplug.Known, Value: filter}
					d.ToneMode = codeplug.StringField{State: codeplug.Known, Value: tm.Value}
					if f, _ := fx.outsideVocabulary(d, caps); f != "" {
						t.Errorf("mode %q / filter %q / tone_mode %q was refused on field %s, and all three are printed", mode, filter, tm.Value, f)
					}
				}
			}
		}
	})
}
func (fx Fixture) tUnreachableRefusalsTellTheTruth(t *testing.T) {
	t.Run("the E6 comparison, handed a short record", func(t *testing.T) {
		err := fx.unmappedRegionsDiffer(make([]byte, 3))
		if err == nil {
			t.Fatal("a 3-byte record passed the E6 comparison")
		}
		e := fx.AsUnmapped(err)
		if e != nil {
			t.Errorf("err = %+v, an UnmappedRegionError naming a nibble it never measured; the old form printed \"byte 0 carries 0x0 where the template carries 0x0\", which is a lie about a record that was never read", e)
		}
		for _, want := range []string{"3-byte", "25", "length fingerprint"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal %q does not mention %q", err, want)
			}
		}
		if err := fx.unmappedRegionsDiffer(fx.Golden); err != nil {
			t.Errorf("the golden record was refused: %v", err)
		}
	})

	t.Run("the tone refusal names the situation it is in", func(t *testing.T) {
		create := fx.toneRefusalReason(true)
		update := fx.toneRefusalReason(false)
		if create == update {
			t.Fatal("the CREATE and UPDATE arms share one sentence; they are different facts")
		}
		if !strings.Contains(create, "no prior record") || !strings.Contains(create, fx.DefaultToneEntry) {
			t.Errorf("the CREATE reason does not carry T1(5)'s fact and its register entry: %q", create)
		}
		if strings.Contains(update, "no prior record") {
			t.Errorf("the UPDATE reason claims there is no prior record; on an UPDATE there IS one: %q", update)
		}
		if !strings.Contains(update, "impossible") {
			t.Errorf("the UPDATE reason does not say the arm is unreachable on this single-layout profile: %q", update)
		}
	})
}
