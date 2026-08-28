// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

import "strings"

// recordFields is one memory record's content in WIRE bytes, field by
// field, so a test states what the radio says rather than what a builder
// would produce for it.
//
// DELIBERATELY NOT civ.Profile.BuildMemorySet. A fixture built by the
// builder the driver calls would pin the driver's parser against that
// builder — the two would agree about a wrong offset just as happily as a
// right one — and would additionally refuse the malformed records these
// tests need, validation being exactly what makes it useless here.
//
// The layout is assembled BY OFFSET from PDF p.19 (folio 18), "• Memory
// content", Command: 1A 00, whose printed indices are named per field
// below. Offsets are 0-based from the start of the RECORD, i.e. after the
// four channel-address bytes (spec Erratum 1's convention), and the only
// field that moves between the two layouts is the frequency: five bytes
// in the shape the diagram draws, six in the 10 GHz form.
type recordFields struct {
	// select5 is byte ⑤, "Split and Select memory setting". 0x00 is the
	// template value, and the footnote "* Set 0 for Call channel."
	// applies to it. A non-zero value here is a SELECT ★ tag, which
	// write.go must REFUSE rather than clear.
	select5 byte
	// freqHz is ⑥~⑩ (or ⑥~⑪ in the wide form): packed BCD, least
	// significant pair first.
	freqHz uint64
	// freqBytes is 5 for the 64-byte record and 6 for the 65-byte one.
	freqBytes      int
	mode           byte   // ⑪, PDF p.17 folio 16 column ①
	filter         byte   // ⑫, same table column ②
	dataMode       byte   // ⑬
	duplexTone     byte   // ⑭: duplex in the high nibble, tone mode in the low
	digitalSquelch byte   // ⑮
	toneTX         uint64 // ⑯~⑱, big-endian BCD in TENTHS of a hertz
	toneRX         uint64 // ⑲~㉑, likewise
	dtcsPol        byte   // ㉒, transmit polarity high / receive low
	dtcsCode       uint64 // ㉓,㉔, big-endian BCD
	dvSquelch      byte   // ㉕
	offsetHz       uint64 // ㉖~㉘, little-endian BCD at 100 Hz resolution
	// urCall, r1Call and r2Call are ㉙~㊱, ㊲~㊹ and ㊺~52: three
	// eight-character D-STAR call-sign blocks with no home in the
	// neutral model. Empty means the template's eight spaces.
	urCall, r1Call, r2Call string
	name                   string // 53~68, sixteen characters fixed
}

// goldenRecord is the record BOTH golden set vectors carry, at the given
// frequency width — read by hand off
// core/civ/ic905/testdata/ic905-vectors.golden and its assumptions CSV: a
// 144.500 MHz FM channel, FIL1, data mode OFF, duplex OFF, tone mode OFF,
// 88.5 Hz in BOTH tone blocks (which is why no delivered byte depends on
// ic905.tone_block_assignment being right), DTCS polarity NN, code 023,
// zero offset, and the name "HIGHLAND BASE905".
func goldenRecord(freqHz uint64, freqBytes int) recordFields {
	return recordFields{
		freqHz:     freqHz,
		freqBytes:  freqBytes,
		mode:       0x05, // 05:FM
		filter:     0x01, // 01:FIL1
		dataMode:   0x00, // 00: Data mode OFF
		duplexTone: 0x00, // 0=Duplex OFF, 0=tone mode OFF
		toneTX:     885,
		toneRX:     885,
		dtcsPol:    0x00, // NN
		dtcsCode:   23,
		offsetHz:   0,
		name:       "HIGHLAND BASE905",
	}
}

// build assembles the record's bytes.
func (r recordFields) build() []byte {
	n := r.freqBytes
	if n == 0 {
		n = 5
	}
	rec := make([]byte, 64+n-5)
	off := func(afterFreq int) int { return afterFreq + n - 5 }

	rec[0] = r.select5
	copy(rec[1:1+n], bcdLittle(r.freqHz, n))
	rec[off(6)] = r.mode
	rec[off(7)] = r.filter
	rec[off(8)] = r.dataMode
	rec[off(9)] = r.duplexTone
	rec[off(10)] = r.digitalSquelch
	copy(rec[off(11):off(14)], bcdBig(r.toneTX, 3))
	copy(rec[off(14):off(17)], bcdBig(r.toneRX, 3))
	rec[off(17)] = r.dtcsPol
	copy(rec[off(18):off(20)], bcdBig(r.dtcsCode, 2))
	rec[off(20)] = r.dvSquelch
	copy(rec[off(21):off(24)], bcdLittle(r.offsetHz/100, 3))
	copy(rec[off(24):off(32)], callSign(r.urCall))
	copy(rec[off(32):off(40)], callSign(r.r1Call))
	copy(rec[off(40):off(48)], callSign(r.r2Call))
	copy(rec[off(48):off(64)], padName(r.name))
	return rec
}

// callSign renders one eight-character call-sign block, space-padded —
// the template value 0x20 this document's own call-sign character table
// prints for a space (PDF p.24, folio 23).
func callSign(s string) []byte {
	b := []byte(strings.Repeat(" ", 8))
	copy(b, s)
	return b
}

// padName renders the sixteen-character name field, space-padded. The pad
// byte is ASSUMED — register ic905.name_pad_byte, lift ic905-R-17.
func padName(s string) []byte {
	b := []byte(strings.Repeat(" ", 16))
	copy(b, s)
	return b
}

// bcdLittle packs v into n bytes of BCD, LEAST significant pair first:
// each byte's LOW nibble is the less significant of its two digits. It is
// the form PDF p.17 (folio 16)'s "• Operating frequency" diagram draws,
// whose rotated labels run "10 Hz digit", "1 Hz digit", "1 kHz", "100 Hz"
// and so on.
func bcdLittle(v uint64, n int) []byte {
	b := make([]byte, n)
	for i := 0; i < n; i++ {
		b[i] = byte((v % 10) | ((v / 10 % 10) << 4)) //nolint:gocritic // digit pair, low first
		v /= 100
	}
	return b
}

// bcdBig packs v into n bytes of BCD, MOST significant pair first — the
// form the tone and DTCS-code diagrams draw (PDF p.24, folio 23).
func bcdBig(v uint64, n int) []byte {
	b := make([]byte, n)
	for i := n - 1; i >= 0; i-- {
		b[i] = byte((v % 10) | ((v / 10 % 10) << 4)) //nolint:gocritic // digit pair, low first
		v /= 100
	}
	return b
}
