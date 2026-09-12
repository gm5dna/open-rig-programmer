// SPDX-License-Identifier: GPL-3.0-or-later

package ic9100

import (
	"bytes"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func knownRecord() civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:      civ.ChannelAddress{Group: 1, Channel: 1},
		RXFreqHz:     civ.Available(uint64(145_500_000)),
		OffsetHz:     civ.Available(uint64(600_000)),
		ToneTXDeciHz: civ.Available(uint64(885)),
		ToneRXDeciHz: civ.Available(uint64(885)),
		DTCSCode:     civ.Available(uint64(23)),
		Duplex:       civ.Available("OFF"),
		Mode:         civ.Available("FM"),
		Filter:       civ.Available("FIL1"),
		DataMode:     civ.Available("OFF"),
		ToneMode:     civ.Available("OFF"),
		DTCSPolarity: civ.Available("NN"),
		Name:         civ.Available("HOME BASE"),
		Select:       civ.Available("OFF"),
	}
}

func answerFrameFromSet(t *testing.T, set []byte) []byte {
	t.Helper()
	answer := append([]byte(nil), set...)
	answer[2], answer[3] = answer[3], answer[2]
	return answer
}

func TestRecordRoundTrip(t *testing.T) {
	rec := knownRecord()
	cmd, err := Profile().BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	frame := cmd.Bytes()
	if got, want := len(frame), 4+2+AddressBytes+RecordLength+1; got != want {
		t.Fatalf("set frame length = %d, want %d", got, want)
	}

	record := frame[6+AddressBytes : len(frame)-1]
	for _, check := range []struct {
		name   string
		lo, hi int
		want   []byte
	}{
		{"select", 0, 1, []byte{0x00}},
		{"RX frequency", 1, 6, []byte{0x00, 0x00, 0x50, 0x45, 0x01}},
		{"mode/filter", 6, 8, []byte{0x05, 0x01}},
		{"data mode", 8, 9, []byte{0x00}},
		{"duplex/tone_mode", 9, 10, []byte{0x00}},
		{"DSQL (unmapped)", 10, 11, []byte{0x00}},
		{"tone TX", 11, 14, []byte{0x00, 0x08, 0x85}},
		{"tone RX", 14, 17, []byte{0x00, 0x08, 0x85}},
		{"DTCS polarity", 17, 18, []byte{0x00}},
		{"DTCS code", 18, 20, []byte{0x00, 0x23}},
		{"digital code squelch (unmapped)", 20, 21, []byte{0x00}},
		{"offset", 21, 24, []byte{0x00, 0x60, 0x00}},
		{"name", 48, 57, []byte("HOME BASE")},
	} {
		if got := record[check.lo:check.hi]; !bytes.Equal(got, check.want) {
			t.Errorf("%s bytes = % X, want % X", check.name, got, check.want)
		}
	}

	back, err := Profile().ParseMemoryAnswer(answerFrameFromSet(t, frame))
	if err != nil {
		t.Fatalf("ParseMemoryAnswer: %v", err)
	}
	if back != rec {
		t.Fatalf("round trip changed record:\n got %+v\nwant %+v", back, rec)
	}
}

func TestRecordFixedTemplate(t *testing.T) {
	cmd, err := Profile().BuildMemorySet(knownRecord())
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	record := cmd.Bytes()[6+AddressBytes : 6+AddressBytes+RecordLength]
	for _, region := range []struct {
		name   string
		lo, hi int
	}{
		{"DSQL", DigitalSquelchOffset, DigitalSquelchOffset + 1},
		{"digital code squelch", DigitalCodeSquelchOffset, DigitalCodeSquelchOffset + 1},
		{"destination call sign", DestCallOffset, DestCallOffset + callLength},
		{"R1 call sign", R1CallOffset, R1CallOffset + callLength},
		{"R2 call sign", R2CallOffset, R2CallOffset + callLength},
	} {
		if got, want := record[region.lo:region.hi], fixedTemplateBytes[region.lo:region.hi]; !bytes.Equal(got, want) {
			t.Errorf("%s fixed bytes = % X, want % X", region.name, got, want)
		}
	}
}

func TestAddressBoundaryFrames(t *testing.T) {
	p := Profile()
	for _, tc := range []struct {
		name string
		addr civ.ChannelAddress
		want []byte
	}{
		{
			name: "band 00 (HF/50MHz) first channel",
			addr: civ.ChannelAddress{Group: 0, Channel: 1},
			want: []byte{0xfe, 0xfe, 0x7c, 0xe0, 0x1a, 0x00, 0x00, 0x00, 0x01, 0xfd},
		},
		{
			name: "band 02 (430MHz) last channel",
			addr: civ.ChannelAddress{Group: 2, Channel: 99},
			want: []byte{0xfe, 0xfe, 0x7c, 0xe0, 0x1a, 0x00, 0x02, 0x00, 0x99, 0xfd},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := p.BuildMemoryRead(tc.addr)
			if err != nil {
				t.Fatalf("BuildMemoryRead(%v): %v", tc.addr, err)
			}
			if got := cmd.Bytes(); !bytes.Equal(got, tc.want) {
				t.Errorf("frame = % X, want % X", got, tc.want)
			}
			if !p.AllowedCommand(cmd.Bytes()) {
				t.Errorf("profile gate refused its own valid boundary frame % X", cmd.Bytes())
			}
		})
	}
}

func TestAddressesOutsideBaseRectangleAreRefused(t *testing.T) {
	for _, tc := range []struct {
		name string
		addr civ.ChannelAddress
	}{
		{"band 03 (UX-9100 4th band, deferred — doc.go)", civ.ChannelAddress{Group: 3, Channel: 1}},
		{"channel 0000", civ.ChannelAddress{Group: 1, Channel: 0}},
		{"channel 0100 (scan edge, out of scope)", civ.ChannelAddress{Group: 1, Channel: 100}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd, err := Profile().BuildMemoryRead(tc.addr)
			if err == nil {
				t.Fatalf("BuildMemoryRead(%v) built out-of-scope frame % X", tc.addr, cmd.Bytes())
			}
		})
	}
}

// TestRecordToneRangeDecision replays every family-standard chart tone
// (PDF p.74, matrix §1 row 12) through the profile's own three-byte BCD
// tenths-of-a-hertz span, exactly as ic7100's identical test does for its
// sibling model.
func TestRecordToneRangeDecision(t *testing.T) {
	for i, tone := range spec.StandardCTCSSTones() {
		rec := knownRecord()
		rec.ToneTXDeciHz = civ.Available(uint64(tone))
		rec.ToneRXDeciHz = civ.Available(uint64(tone))
		cmd, err := Profile().BuildMemorySet(rec)
		if err != nil {
			t.Fatalf("BuildMemorySet(tone %v): %v", tone, err)
		}
		frame := cmd.Bytes()
		record := frame[6+AddressBytes : len(frame)-1]
		want := []byte{0x00, byte(tone/1000)<<4 | byte(tone/100%10), byte(tone/10%10)<<4 | byte(tone%10)}
		if got := record[11:14]; !bytes.Equal(got, want) {
			t.Errorf("chart tone %d (%v) TX bytes = % X, want % X", i, tone, got, want)
		}
		if got := record[14:17]; !bytes.Equal(got, want) {
			t.Errorf("chart tone %d (%v) RX bytes = % X, want % X", i, tone, got, want)
		}
		back, err := Profile().ParseMemoryAnswer(answerFrameFromSet(t, frame))
		if err != nil {
			t.Fatalf("ParseMemoryAnswer(tone %v): %v", tone, err)
		}
		if back != rec {
			t.Errorf("chart tone %d (%v) did not survive the round trip:\n got %+v\nwant %+v", i, tone, back, rec)
		}
	}
}
