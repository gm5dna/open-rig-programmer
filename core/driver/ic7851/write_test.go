// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7851 "github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func TestE6RefusesNonTemplateUnmappedBytes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		offset int
		value  byte
		nibble string
	}{
		{"a ★2 select-group marker", civic7851.SelectNibbleOffset, 0x02, "low"},
		{"a non-zero ③ high nibble", civic7851.SelectNibbleOffset, 0x10, "high"},
		{"a DATA 1 data mode", civic7851.DataModeNibbleOffset, 0x10, "high"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := make([]byte, civic7851.RecordOnlyLength)
			raw[tc.offset] = tc.value
			var e *UnmappedRegionError
			err := unmappedRegionsDiffer(raw)
			if !errors.As(err, &e) {
				t.Fatalf("E6 result = %v, want *UnmappedRegionError", err)
			}
			if e.Offset != tc.offset || e.Nibble != tc.nibble {
				t.Errorf("UnmappedRegionError = %+v, want offset %d nibble %s", e, tc.offset, tc.nibble)
			}
			if !errors.Is(err, ErrUnmappedRegion) {
				t.Error("the refusal does not satisfy errors.Is(err, ErrUnmappedRegion)")
			}
			if errors.Is(err, driver.ErrWriteRefused) {
				t.Error("the E6 refusal claims driver.ErrWriteRefused, whose contract says NOTHING went out — this one necessarily follows the preservation read")
			}
		})
	}
}

func TestE6AcceptsTemplate(t *testing.T) {
	if err := unmappedRegionsDiffer(make([]byte, civic7851.RecordOnlyLength)); err != nil {
		t.Fatal(err)
	}
	// The tone-mode nibble is MAPPED and must not be judged by E6: a
	// channel whose tone is TSQL is an ordinary writable channel.
	raw := make([]byte, civic7851.RecordOnlyLength)
	raw[civic7851.DataModeNibbleOffset] = 0x02
	if err := unmappedRegionsDiffer(raw); err != nil {
		t.Fatalf("E6 refused a TSQL channel: %v", err)
	}
}

// consentedSession is a Session with NO ENGINE, for the refusal rungs that
// must precede all wire traffic. Reaching the wire from one of these panics
// on a nil pointer, which is a stronger assertion than counting bytes.
func consentedSession() *Session {
	return &Session{caps: spec.ConsentUnverifiedWrites(capabilitiesUnverified())}
}

func validChannelData() codeplug.ChannelData {
	return codeplug.ChannelData{
		FreqHz:   14_250_000,
		Mode:     "USB",
		Filter:   codeplug.StringField{State: codeplug.Known, Value: "FIL1"},
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)},
		ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(1000)},
		Tag:      "ALPHA",
	}
}

// TestWriteChannel_LocalRefusalsPrecedeAllWireTraffic walks tier ruling
// T5's locally decidable rungs. Every one of them is reached with a nil
// engine, so a rung that had drifted BELOW the preservation read would
// panic here rather than pass quietly.
func TestWriteChannel_LocalRefusalsPrecedeAllWireTraffic(t *testing.T) {
	for _, tc := range []struct {
		name   string
		slot   string
		mutate func(*codeplug.ChannelData)
		fields []spec.Field
	}{
		{"an erase", "001", nil, []spec.Field{spec.FieldErase}},
		{"a Known scan_skip, which is four-valued SELECT-group membership", "001",
			func(d *codeplug.ChannelData) { d.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true} },
			[]spec.Field{spec.FieldScanSkip}},
		{"a Known data_mode, which is likewise four-valued", "001",
			func(d *codeplug.ChannelData) { d.DataMode = codeplug.BoolField{State: codeplug.Known, Value: true} },
			[]spec.Field{spec.FieldDataMode}},
		{"a Known split-TX frequency this record has no span for", "001",
			func(d *codeplug.ChannelData) {
				d.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: 14_260_000}
			},
			[]spec.Field{spec.FieldTxFrequency}},
		{"a mode outside the ten printed codes", "001",
			func(d *codeplug.ChannelData) { d.Mode = "C4FM" }, []spec.Field{spec.FieldMode}},
		{"a filter outside the three printed codes", "001",
			func(d *codeplug.ChannelData) {
				d.Filter = codeplug.StringField{State: codeplug.Known, Value: "FIL4"}
			}, []spec.Field{spec.FieldFilter}},
		{"a tone mode outside the three printed codes", "001",
			func(d *codeplug.ChannelData) {
				d.ToneMode = codeplug.StringField{State: codeplug.Known, Value: "DCS"}
			}, []spec.Field{spec.FieldToneMode}},
		{"an absent mode, which the record has no way to leave alone", "001",
			func(d *codeplug.ChannelData) { d.Mode = "" }, []spec.Field{spec.FieldMode}},
		{"an absent filter", "001",
			func(d *codeplug.ChannelData) { d.Filter = codeplug.StringField{} }, []spec.Field{spec.FieldFilter}},
		{"an absent tone mode", "001",
			func(d *codeplug.ChannelData) { d.ToneMode = codeplug.StringField{} }, []spec.Field{spec.FieldToneMode}},
		{"a tag longer than the ten-byte name span", "001",
			func(d *codeplug.ChannelData) { d.Tag = "ELEVENCHARS" }, []spec.Field{spec.FieldTag}},
		{"a tag byte outside the printed charset", "001",
			func(d *codeplug.ChannelData) { d.Tag = "A\x01B" }, []spec.Field{spec.FieldTag}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := consentedSession()
			ch := codeplug.Channel{Slot: tc.slot}
			if tc.mutate != nil {
				d := validChannelData()
				tc.mutate(&d)
				ch.Data = &d
			}
			res, err := s.WriteChannel(t.Context(), ch)
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("WriteChannel = %v, want driver.ErrWriteRefused", err)
			}
			var refusal *driver.WriteRefusedError
			if errors.As(err, &refusal) {
				if len(refusal.Fields) != len(tc.fields) || refusal.Fields[0] != tc.fields[0] {
					t.Errorf("the refusal named %v, want %v — a Known value the wire cannot say is REFUSED BY NAME, never dropped", refusal.Fields, tc.fields)
				}
			}
			if res.Steps == nil || len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want an empty (never nil) slice: there is no sequence to describe", res.Steps)
			}
		})
	}
}

// TestWriteChannel_BadSlotIsRefusedBeforeAnything covers the slot map's
// negative half through WriteChannel, which is where a caller meets it.
func TestWriteChannel_BadSlotIsRefusedBeforeAnything(t *testing.T) {
	s := consentedSession()
	for _, slot := range []string{"000", "100", "101", "P0", "P3", "1", "", "CALL", "G01-001"} {
		d := validChannelData()
		if _, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: slot, Data: &d}); err == nil {
			t.Errorf("WriteChannel accepted the slot %q", slot)
		}
	}
}

// TestNumericRefusalIsDefenceInDepthNotTheGate asserts the GAP that
// remains, so that closing it later is a visible test change.
//
// civ.FieldSpan carries no numeric domain, so AllowedCommand — the last
// defence before a radio sees bytes — admits a set carrying a frequency
// this radio cannot receive, provided the digits fit the field. The
// driver's rung 4 is what refuses it. DO NOT DELETE THIS TEST TO "FIX" IT:
// the enabler that closes the gap should turn it red.
func TestNumericRefusalIsDefenceInDepthNotTheGate(t *testing.T) {
	p := civic7851.Profile()
	// 65 MHz: above the declared 60 MHz radio ceiling and below the
	// 99.999999 MHz the four variable frequency bytes can encode.
	const above = 65_000_000
	cmd, err := p.BuildMemorySet(civ.MemoryRecord{
		Address: civ.ChannelAddress{Channel: 1}, RXFreqHz: civ.Available(uint64(above)),
		Mode: civ.Available("USB"), Filter: civ.Available("FIL1"), ToneMode: civ.Available("OFF"),
		ToneTXDeciHz: civ.Available(uint64(885)), ToneRXDeciHz: civ.Available(uint64(1000)),
		Name: civ.Available("HIGH"),
	})
	if err != nil {
		t.Fatalf("the codec refused to build a frame it has no domain for: %v", err)
	}
	if !p.AllowedCommand(cmd.Bytes()) {
		t.Fatal("THE GAP HAS CLOSED: the gate now refuses an out-of-domain frame. That is the intended end state — update this test and doc.go rather than deleting either")
	}

	d := validChannelData()
	d.FreqHz = above
	s := consentedSession()
	if _, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: "001", Data: &d}); !errors.Is(err, ErrOutOfDomain) {
		t.Fatalf("the driver admitted %d Hz: %v", above, err)
	}
}

// TestWriteChannel_NoClearFrameIsReachable is the source-level half of the
// erase rule; TestE2E_BothEraseFormsAreRefused is the wire-level half.
//
// There is no builder for either printed clear form anywhere in this tier,
// and the gate admits only 19 00, a valid 1A 00 read and a re-validated
// 1A 00 set — so neither frame can be constructed, let alone sent.
func TestWriteChannel_NoClearFrameIsReachable(t *testing.T) {
	p := civic7851.Profile()
	for _, frame := range [][]byte{
		{0xfe, 0xfe, 0x8e, 0xe0, 0x1a, 0x00, 0x00, 0x01, 0xff, 0xfd},
		{0xfe, 0xfe, 0x8e, 0xe0, 0x0b, 0xfd},
		{0xfe, 0xfe, 0x8e, 0xe0, 0x09, 0xfd},
		{0xfe, 0xfe, 0x8e, 0xe0, 0x0a, 0xfd},
	} {
		if p.AllowedCommand(frame) {
			t.Errorf("the gate admitted a clear form: % X", frame)
		}
	}
	// And FieldErase is unreachable through consent, structurally.
	for _, caps := range []spec.Capabilities{
		capabilitiesUnverified(), capabilitiesSimulated(),
		spec.ConsentUnverifiedWrites(capabilitiesUnverified()),
		spec.ConsentUnverifiedWrites(capabilitiesSimulated()),
	} {
		for _, bank := range []spec.BankID{spec.BankMemory, spec.BankScan} {
			if caps.FieldSupport(bank, spec.FieldErase).CanWrite() {
				t.Errorf("FieldErase is writable on %s", bank)
			}
		}
	}
}
