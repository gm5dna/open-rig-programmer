// SPDX-License-Identifier: GPL-3.0-or-later

package fakeyaesuclone

import (
	"encoding/binary"

	"github.com/gm5dna/open-rig-programmer/core/clonewire"
)

// unmappedFill is the fixed, deliberately non-zero byte every region
// outside the channel bank is filled with — see doc.go, "Fabricated image
// content". Never 0x00: a zero-filled unmapped region reads as "confirmed
// empty", which no capture has confirmed.
const unmappedFill = 0x5A

// emptyChannelFill is CHIRP's own "unused" convention re-derived here
// (informed-by ft817.py's _get_normal: freq 0 or 0xFFFFFFFF reads as
// empty) — an all-0xFF record makes DecodeRecord's FreqHz field read as
// 0xFFFFFFFF without this package needing to know DecodeRecord's exact
// byte offsets for every other field.
const emptyChannelFill = 0xFF

// populatedChannels is a handful of fabricated, internally-consistent
// sample channels this fake writes into every Profile's channel bank — real
// enough to exercise DecodeRecord's frequency/mode/name path, invented
// because no capture exists.
var populatedChannels = []struct {
	freqHz uint32 // CHIRP wire units: Hz / 10
	mode   byte   // index into clonewire's own yaesuModes ordering
	name   string
}{
	{freqHz: 1420000, mode: 1, name: "TEST1"},     // 14.200000 MHz, USB
	{freqHz: 707400, mode: 6, name: "FT8"},        // 7.074000 MHz, DIG
	{freqHz: 14550000, mode: 5, name: "REPEATER"}, // 145.500000 MHz, FM
}

// BuildImage fabricates a p.ImageLen-byte image: populatedChannels at the
// start of p's channel bank, an all-0xFF erased record for every remaining
// channel, and unmappedFill everywhere else. See doc.go.
func BuildImage(p clonewire.Profile) []byte {
	buf := make([]byte, p.ImageLen)
	for i := range buf {
		buf[i] = unmappedFill
	}
	if p.ChannelCount == 0 || p.RecordWidth <= 0 {
		return buf
	}
	for ch := 0; ch < p.ChannelCount; ch++ {
		off := p.RecordOffset + ch*p.RecordWidth
		if off < 0 || off+p.RecordWidth > len(buf) {
			continue
		}
		rec := buf[off : off+p.RecordWidth]
		if ch < len(populatedChannels) {
			writePopulatedRecord(rec, p, populatedChannels[ch])
		} else {
			for i := range rec {
				rec[i] = emptyChannelFill
			}
		}
	}
	return buf
}

// writePopulatedRecord fills one channel record with a fabricated sample:
// unmappedFill everywhere, then the mode/narrow/frequency/name fields
// clonewire.DecodeRecord actually reads, at the offsets p itself states
// (FreqOffset; mode/narrow/name are constant across this family per
// core/clonewire/record.go's own doc comment).
func writePopulatedRecord(rec []byte, p clonewire.Profile, ch struct {
	freqHz uint32
	mode   byte
	name   string
}) {
	for i := range rec {
		rec[i] = unmappedFill
	}
	const (
		modeOffset   = 0
		narrowOffset = 1
		nameLen      = 8
	)
	rec[modeOffset] = ch.mode
	rec[narrowOffset] = 0 // not narrow
	if p.FreqOffset >= 0 && p.FreqOffset+4 <= len(rec) {
		binary.BigEndian.PutUint32(rec[p.FreqOffset:p.FreqOffset+4], ch.freqHz)
	}
	// record.go's DecodeRecord places the name 8 bytes after the END of
	// the frequency field's own 4-byte gap (nameOffset := FreqOffset+8) —
	// the 4 bytes between them are unmapped and stay unmappedFill.
	nameOff := p.FreqOffset + 8
	if nameOff >= 0 && nameOff+nameLen <= len(rec) {
		name := rec[nameOff : nameOff+nameLen]
		for i := range name {
			name[i] = ' '
		}
		copy(name, ch.name)
	}
}
