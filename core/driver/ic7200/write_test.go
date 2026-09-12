// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

func legalChannelData() codeplug.ChannelData {
	return codeplug.ChannelData{
		FreqHz:   14_250_000,
		Mode:     "USB",
		Filter:   codeplug.StringField{State: codeplug.Known, Value: "Wide"},
		DataMode: codeplug.BoolField{State: codeplug.Known, Value: true},
	}
}

// TestWriteChannel_Erase pins rung 1: an empty channel is refused before
// any wire traffic, FieldErase named.
func TestWriteChannel_Erase(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio())
	s := openWith(t, p)

	_, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: "002", Data: nil})
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel(nil) = %v, want ErrWriteRefused", err)
	}
	if n := len(p.Transcript()); n != 2 { // Open's own 19 00 probe + one 1A 00 read of channel 1
		t.Errorf("transcript has %d frames after an erase refusal; want no extra wire traffic beyond Open's own probe", n)
	}
}

// TestWriteChannel_MandatoryFieldsMissing pins rung 3: mode, filter and
// data mode are mandatory Known fields with no "leave it alone" encoding.
func TestWriteChannel_MandatoryFieldsMissing(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio())
	s := openWith(t, p)

	for _, tc := range []struct {
		name   string
		mutate func(*codeplug.ChannelData)
	}{
		{"mode", func(d *codeplug.ChannelData) { d.Mode = "" }},
		{"filter", func(d *codeplug.ChannelData) { d.Filter = codeplug.StringField{} }},
		{"data mode", func(d *codeplug.ChannelData) { d.DataMode = codeplug.BoolField{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := legalChannelData()
			tc.mutate(&d)
			_, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: "003", Data: &d})
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("WriteChannel = %v, want ErrWriteRefused", err)
			}
		})
	}
}

// TestWriteChannel_Vocabulary pins rung 3b: a mode or filter this radio
// cannot express is refused, never dropped or mapped to a neighbour.
func TestWriteChannel_Vocabulary(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio())
	s := openWith(t, p)

	for _, tc := range []struct {
		name   string
		mutate func(*codeplug.ChannelData)
	}{
		{"mode", func(d *codeplug.ChannelData) { d.Mode = "FM" }},             // matrix §1 row 5: no FM code
		{"filter", func(d *codeplug.ChannelData) { d.Filter.Value = "FIL1" }}, // matrix §1b: this radio says Wide/Mid/Narrow, not FIL1
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := legalChannelData()
			tc.mutate(&d)
			_, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: "003", Data: &d})
			if !errors.Is(err, driver.ErrWriteRefused) {
				t.Fatalf("WriteChannel = %v, want ErrWriteRefused", err)
			}
		})
	}
}

// TestWriteChannel_OutOfDomainFrequency pins rung 4.
func TestWriteChannel_OutOfDomainFrequency(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio())
	s := openWith(t, p)

	d := legalChannelData()
	d.FreqHz = MaxRadioFreqHz + 1
	_, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: "003", Data: &d})
	var oor *OutOfDomainError
	if !errors.As(err, &oor) {
		t.Fatalf("WriteChannel = %v, want *OutOfDomainError", err)
	}
}

// TestWriteChannel_TxFrequencyDefaultsToMirrorRX pins rung 7's
// SimplexTxEqualsRx default: an unsupplied TxFreqHz on a CREATE is never
// refused — it mirrors the RX frequency, per the manual's own NOTE
// (matrix §3.11).
func TestWriteChannel_TxFrequencyDefaultsToMirrorRX(t *testing.T) {
	p := newScriptedPort(t, radioImage{idToken: []byte{0x76}, ackSets: true})
	s := openWith(t, p)

	d := legalChannelData()
	res, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: "003", Data: &d})
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Fatalf("WriteResult = %+v, want one sent+confirmed step", res)
	}
	transcript := p.Transcript()
	set := transcript[len(transcript)-1]
	if len(set) != memSetFrameLen {
		t.Fatalf("set frame is %d bytes, want %d: % X", len(set), memSetFrameLen, set)
	}
	// Record starts at frame index 8 (FE FE 76 E0 1A 00 <ch-hi> <ch-lo>);
	// the TX-duplicate frequency span is record offset 9..13 and must
	// equal the RX span at offset 1..5.
	rx := set[8+1 : 8+6]
	tx := set[8+9 : 8+14]
	if string(rx) != string(tx) {
		t.Errorf("TX frequency % X does not mirror RX frequency % X", tx, rx)
	}
}

// TestWriteChannel_TxFrequencyExplicit pins that a caller-supplied
// TxFreqHz is used verbatim rather than mirrored.
func TestWriteChannel_TxFrequencyExplicit(t *testing.T) {
	p := newScriptedPort(t, radioImage{idToken: []byte{0x76}, ackSets: true})
	s := openWith(t, p)

	d := legalChannelData()
	d.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: 7_100_000}
	if _, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: "003", Data: &d}); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	transcript := p.Transcript()
	set := transcript[len(transcript)-1]
	rx := set[8+1 : 8+6]
	tx := set[8+9 : 8+14]
	if string(rx) == string(tx) {
		t.Error("TX frequency mirrored RX even though an explicit TxFreqHz was supplied")
	}
}

// TestWriteChannel_UnmappedRegionRefuses pins rung 6: a stored slot whose
// Split byte (or TX-mirror bytes) differ from the Fixed template cannot
// be written by this programme at all.
func TestWriteChannel_UnmappedRegionRefuses(t *testing.T) {
	splitOn := append([]byte(nil), goldenRecord...)
	splitOn[0] = 0x10 // Split ON
	p := newScriptedPort(t, radioImage{
		idToken: []byte{0x76},
		records: map[int][]byte{1: goldenRecord, 3: splitOn},
	})
	s := openWith(t, p)

	d := legalChannelData()
	_, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: "003", Data: &d})
	var unmapped *UnmappedRegionError
	if !errors.As(err, &unmapped) {
		t.Fatalf("WriteChannel = %v, want *UnmappedRegionError", err)
	}
	if unmapped.Offset != 0 {
		t.Errorf("UnmappedRegionError.Offset = %d, want 0 (Split)", unmapped.Offset)
	}
}

// TestWriteChannel_EmptySlotWritesCleanly pins that a CREATE (no prior
// record) proceeds directly against the Fixed template — no unmapped
// region to compare.
func TestWriteChannel_EmptySlotWritesCleanly(t *testing.T) {
	p := newScriptedPort(t, radioImage{idToken: []byte{0x76}, ackSets: true})
	s := openWith(t, p)

	d := legalChannelData()
	res, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: "005", Data: &d})
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Confirmed {
		t.Fatalf("WriteResult = %+v", res)
	}
}

// TestWriteChannel_RejectedSet pins the FA branch: an explicit rejection
// is an attributable outcome.
func TestWriteChannel_RejectedSet(t *testing.T) {
	p := newScriptedPort(t, radioImage{idToken: []byte{0x76}, rejectSets: true})
	s := openWith(t, p)

	d := legalChannelData()
	res, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: "005", Data: &d})
	if err == nil {
		t.Fatal("WriteChannel succeeded, want the radio's rejection surfaced")
	}
	if len(res.Steps) != 1 || !res.Steps[0].Sent || res.Steps[0].Confirmed {
		t.Fatalf("WriteResult = %+v, want Sent true, Confirmed false", res)
	}
}

// TestWriteChannel_CapabilityGateRefusesWithoutConsent pins rung 2 on a
// RealHardware (Unverified) profile: nothing is writable without
// recorded consent, and the refusal precedes all wire traffic.
func TestWriteChannel_CapabilityGateRefusesWithoutConsent(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio())
	d := New(RealHardware)
	sess, err := d.Open(t.Context(), p.Port(), driver.Identity{Port: "/dev/scripted"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = sess.Close() }()

	data := legalChannelData()
	_, err = sess.WriteChannel(t.Context(), codeplug.Channel{Slot: "003", Data: &data})
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel = %v, want ErrWriteRefused", err)
	}
}
