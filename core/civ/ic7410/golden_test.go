// SPDX-License-Identifier: GPL-3.0-or-later

package ic7410_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7410"
)

// bcdLE renders n as width bytes of little-endian packed BCD — the
// frequency spans' convention on this radio (matrix §1b).
func bcdLE(n uint64, width int) []byte {
	out := make([]byte, width)
	for i := 0; i < width; i++ {
		lo := byte(n % 10)
		n /= 10
		hi := byte(n % 10)
		n /= 10
		out[i] = hi<<4 | lo
	}
	return out
}

// bcdBE is bcdLE reversed — the tone spans' convention on this radio,
// the OPPOSITE byte order to the frequency spans on the same record.
func bcdBE(n uint64, width int) []byte {
	out := bcdLE(n, width)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// goldenVector is testdata/ic7410-golden-vectors.md's Vector 1 or 2,
// authored directly from the matrix; see that file for citations.
type goldenVector struct {
	addr                   civ.ChannelAddress
	rxFreqHz, txFreqHz     uint64
	mode, filter, dataMode string
	toneMode               string
	toneTx, toneRx         uint64
	name                   string
}

// record assembles the 40-byte record BY OFFSET, from this package's own
// reading of the matrix table — independently of core/civ/ic7410's
// profile, so agreement below is a genuine cross-check and not the codec
// agreeing with itself.
func (v goldenVector) record(t *testing.T) []byte {
	t.Helper()
	rec := make([]byte, ic7410.RecordOnlyLength)
	// byte 0: UNMAPPED, left 0x00.
	copy(rec[1:6], bcdLE(v.rxFreqHz, 5))
	rec[6] = enumCode(t, "mode", v.mode)
	rec[7] = enumCode(t, "filter", v.filter)
	rec[8] = enumCode(t, "data_mode", v.dataMode)
	rec[9] = enumCode(t, "tone_mode", v.toneMode) << 4 // HIGH nibble on this model
	copy(rec[10:13], bcdBE(v.toneTx, 3))
	copy(rec[13:16], bcdBE(v.toneRx, 3))
	copy(rec[16:21], bcdLE(v.txFreqHz, 5))
	// bytes 21-30: UNMAPPED, left 0x00.
	for i := 0; i < 9; i++ {
		rec[31+i] = 0x20
	}
	copy(rec[31:40], v.name)
	return rec
}

func enumCode(t *testing.T, kind, name string) byte {
	t.Helper()
	tables := map[string]map[string]byte{
		"mode":      {"LSB": 0x00, "USB": 0x01, "AM": 0x02, "CW": 0x03, "RTTY": 0x04, "FM": 0x05, "CW-R": 0x07, "RTTY-R": 0x08},
		"filter":    {"FIL1": 0x01, "FIL2": 0x02, "FIL3": 0x03},
		"data_mode": {"OFF": 0x00, "ON": 0x01},
		"tone_mode": {"OFF": 0x0, "TONE": 0x1, "TSQL": 0x2},
	}
	code, ok := tables[kind][name]
	if !ok {
		t.Fatalf("no %s code for %q", kind, name)
	}
	return code
}

var (
	vector1 = goldenVector{
		addr:     civ.ChannelAddress{Channel: 1},
		rxFreqHz: 14_250_000, txFreqHz: 14_750_000,
		mode: "USB", filter: "FIL1", dataMode: "ON",
		toneMode: "TONE", toneTx: 885, toneRx: 1000,
		name: "TEST 1",
	}
	vector2 = goldenVector{
		addr:     civ.ChannelAddress{Channel: 100}, // P1
		rxFreqHz: 7_000_000, txFreqHz: 7_000_000,
		mode: "LSB", filter: "FIL2", dataMode: "OFF",
		toneMode: "OFF", toneTx: 1000, toneRx: 1000,
		name: "",
	}
)

// TestGoldenVectorsBuildAndRoundTrip checks core/civ/ic7410's encoder
// against this package's own independent by-offset assembly, and that the
// resulting frame round-trips through the profile's own decoder.
func TestGoldenVectorsBuildAndRoundTrip(t *testing.T) {
	p := ic7410.Profile()
	for name, v := range map[string]goldenVector{"vector1": vector1, "vector2": vector2} {
		t.Run(name, func(t *testing.T) {
			want := v.record(t)
			if len(want) != ic7410.RecordOnlyLength {
				t.Fatalf("this test's own assembly is %d bytes, want %d", len(want), ic7410.RecordOnlyLength)
			}

			rec := civ.MemoryRecord{
				Address:      v.addr,
				RXFreqHz:     civ.Available(v.rxFreqHz),
				TXFreqHz:     civ.Available(v.txFreqHz),
				Mode:         civ.Available(v.mode),
				Filter:       civ.Available(v.filter),
				DataMode:     civ.Available(v.dataMode),
				ToneMode:     civ.Available(v.toneMode),
				ToneTXDeciHz: civ.Available(v.toneTx),
				ToneRXDeciHz: civ.Available(v.toneRx),
				Name:         civ.Available(v.name),
			}
			cmd, err := p.BuildMemorySet(rec)
			if err != nil {
				t.Fatalf("BuildMemorySet: %v", err)
			}
			frame := cmd.Bytes()
			got := frame[len(frame)-1-ic7410.RecordOnlyLength : len(frame)-1]
			if string(got) != string(want) {
				t.Errorf("record bytes disagree:\n got  % 02x\n want % 02x", got, want)
			}

			if !p.AllowedCommand(frame) {
				t.Fatalf("the profile's own gate REFUSED its own builder's frame: % 02x", frame)
			}

			// Round trip through the answer form.
			answer := append([]byte(nil), frame...)
			answer[2], answer[3] = answer[3], answer[2] // swap to/from
			back, err := p.ParseMemoryAnswer(answer)
			if err != nil {
				t.Fatalf("ParseMemoryAnswer: %v", err)
			}
			if back != rec {
				t.Errorf("record did not survive build -> parse:\n got  %+v\nwant %+v", back, rec)
			}
		})
	}
}

// TestGoldenVectorUnmappedBytesAreZero pins that the two UNMAPPED regions
// (record byte 0 and bytes 21-30) are zero in a built frame, which is what
// the Fixed template guarantees and what a future driver's write-time E6
// comparison is judged against.
func TestGoldenVectorUnmappedBytesAreZero(t *testing.T) {
	p := ic7410.Profile()
	cmd, err := p.BuildMemorySet(civ.MemoryRecord{
		Address: vector1.addr, RXFreqHz: civ.Available(vector1.rxFreqHz),
		TXFreqHz: civ.Available(vector1.txFreqHz), Mode: civ.Available(vector1.mode),
		Filter: civ.Available(vector1.filter), DataMode: civ.Available(vector1.dataMode),
		ToneMode: civ.Available(vector1.toneMode), ToneTXDeciHz: civ.Available(vector1.toneTx),
		ToneRXDeciHz: civ.Available(vector1.toneRx), Name: civ.Available(vector1.name),
	})
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	frame := cmd.Bytes()
	rec := frame[len(frame)-1-ic7410.RecordOnlyLength : len(frame)-1]
	if rec[ic7410.SelectSplitOffset] != 0x00 {
		t.Errorf("record[%d] (select/split byte) = %#02x, want 0x00", ic7410.SelectSplitOffset, rec[ic7410.SelectSplitOffset])
	}
	for i := 0; i < ic7410.TXDupUnmappedLength; i++ {
		off := ic7410.TXDupUnmappedOffset + i
		if rec[off] != 0x00 {
			t.Errorf("record[%d] (TX-duplicate-block unmapped byte) = %#02x, want 0x00", off, rec[off])
		}
	}
}
