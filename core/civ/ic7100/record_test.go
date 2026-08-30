// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func knownRecord() civ.MemoryRecord {
	return civ.MemoryRecord{
		Address:      civ.ChannelAddress{Group: 1, Channel: 1},
		RXFreqHz:     civ.Available(uint64(145_500_000)),
		TXFreqHz:     civ.Available(uint64(145_500_000)),
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
	if got, want := len(frame), 121; got != want {
		t.Fatalf("set frame length = %d, want %d", got, want)
	}

	// ASSUMED: ic7100-wire-order. These positions are pinned so changing
	// byte order or confusing record-only and data-area offsets is visible.
	record := frame[6+AddressBytes : len(frame)-1]
	for _, check := range []struct {
		name   string
		lo, hi int
		want   []byte
	}{
		{"RX frequency", 1, 6, []byte{0x00, 0x00, 0x50, 0x45, 0x01}},
		{"tone TX", 11, 14, []byte{0x00, 0x08, 0x85}},
		{"DTCS", 18, 20, []byte{0x00, 0x23}},
		{"offset", 21, 24, []byte{0x00, 0x60, 0x00}},
		{"name", 95, 111, []byte("HOME BASE       ")},
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

func TestRecordDuplicateMismatch(t *testing.T) {
	cmd, err := Profile().BuildMemorySet(knownRecord())
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	answer := answerFrameFromSet(t, cmd.Bytes())
	// ASSUMED: ic7100-tx-block-mandatory. Alter the duplicate mode at
	// record offset 53; the decoder must refuse rather than choose a copy.
	answer[6+AddressBytes+53] = 0x02 // AM, while the primary remains FM.
	_, err = Profile().ParseMemoryAnswer(answer)
	if err == nil || !strings.Contains(err.Error(), "appears twice") {
		t.Fatalf("duplicate mismatch error = %v, want disagreement refusal", err)
	}
}

func TestRecordDVModeCode(t *testing.T) {
	rec := knownRecord()
	rec.Mode = civ.Available("DV")
	cmd, err := Profile().BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet(DV): %v", err)
	}
	record := cmd.Bytes()[6+AddressBytes : 6+AddressBytes+RecordLength]
	// ASSUMED: ic7100-dv-mode-code; lift: store a DV channel at the front
	// panel and read byte ⑩. Until then both copies must carry 0x17.
	if record[6] != 0x17 || record[6+duplicateBlockShift] != 0x17 {
		t.Errorf("DV mode bytes = %#02x/%#02x, want 0x17/0x17", record[6], record[6+duplicateBlockShift])
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
		{"DSQL", 10, 11}, {"CSQL", 20, 21}, {"UR", 24, 32},
		{"R1", 32, 40}, {"R2", 40, 48}, {"duplicate DSQL", 57, 58},
		{"duplicate CSQL", 67, 68}, {"duplicate UR", 71, 79},
		{"duplicate R1", 79, 87}, {"duplicate R2", 87, 95},
	} {
		if got, want := record[region.lo:region.hi], fixedTemplate()[region.lo:region.hi]; !bytes.Equal(got, want) {
			t.Errorf("%s fixed bytes = % X, want % X", region.name, got, want)
		}
	}
}

func TestRecordToneRangeDecision(t *testing.T) {
	// THE WIRE FIELD IS A NUMBER, and this is the dialect's own evidence
	// for saying so: spans ⑮–⑰ and ⑱–⑳ are three-byte packed BCD in
	// TENTHS OF A HERTZ, so every one of the 50 tones printed on PDF p.91
	// — 67.0 to 254.1 Hz, the family-standard chart — is expressible here,
	// byte for byte, and comes back unchanged. Nothing about the chart's
	// irregular spacing is encoded in the record.
	//
	// The capability declaration that rests on this evidence belongs to
	// the driver, not to this package: core/driver/ic7100's
	// TestCTCSSToneDomainAdmitsEveryChartTone pins the domain, and
	// register entry ic7100-tone-range-step remains the open question of
	// whether the RADIO also accepts a tenth of a hertz off the printed
	// chart. The driver declares the span's whole BCD capacity, so the
	// answer no longer changes the declaration.
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
