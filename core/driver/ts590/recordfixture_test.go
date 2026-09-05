// SPDX-License-Identifier: GPL-3.0-or-later

package ts590

import (
	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// mrAddr renders the four addressing bytes of an MR read of the canonical
// slot id — the key an image's mrAnswers and mrSilent maps use.
//
// It is built from THE BYTES THE DRIVER WILL SEND rather than from the
// builder under test: P1 is '1' only for a section channel's UPPER half
// (590:1529-1531), P2 is the hundreds digit this programme always emits as a
// digit (A10), and P3 is the two-digit remainder.
func mrAddr(id string) string {
	num, half, err := parseSlotID(id)
	if err != nil {
		panic("mrAddr: " + err.Error())
	}
	p1 := byte('0')
	if half == kw.ScanUpper {
		p1 = '1'
	}
	return string([]byte{p1, byte('0' + num/100), byte('0' + (num/10)%10), byte('0' + num%10)})
}

// recordFields is the 50-byte MR answer's parameter block, field by field, in
// WIRE bytes, so a test states what the radio says rather than what a builder
// would produce for it.
//
// DELIBERATELY NOT kw.Layout.BuildMWSet. A fixture built by the builder under
// test would pin the parser against the builder — the two would agree about a
// wrong offset just as happily as a right one — and would additionally refuse
// the malformed frames these tests need.
//
// The offsets are the 590 book's own printed positions (590:1440-1461 for the
// MR answer's chart), 1-indexed in the comments and 0-indexed in the code.
type recordFields struct {
	p1   byte   // position 3
	p2   byte   // position 4, the hundreds digit or MC's printed space
	p3   string // positions 5-6
	freq string // positions 7-17, P4, 11 digits
	mode byte   // position 18, P5
	b19  byte   // position 19, P6, the data mode on this book
	tone byte   // position 20, P7
	p8   string // positions 21-22
	p9   string // positions 23-24
	b28  byte   // position 28, P11, FILTER A/B
	p14  string // positions 39-40
	b41  byte   // position 41, P15, the channel lockout
	name string // positions 42-49, P16, space-padded (A1)
}

// frame assembles the 50-byte MR answer these fields describe.
func (f recordFields) frame() string {
	b := make([]byte, kw.RecordLen)
	for i := range b {
		// A VISIBLE SENTINEL, so a position a fixture forgets to fill shows
		// up as a parse failure naming the field rather than as an
		// accidental zero that happens to be valid somewhere.
		b[i] = '?'
	}
	copy(b[0:2], "MR")
	b[2] = f.p1
	b[3] = f.p2
	copy(b[4:6], f.p3)
	copy(b[6:17], f.freq)
	b[17] = f.mode
	b[18] = f.b19
	b[19] = f.tone
	copy(b[20:22], f.p8)
	copy(b[22:24], f.p9)
	copy(b[24:27], "000") // P10, "Always 000" (590:1558-1559)
	b[27] = f.b28
	b[28] = '0'                 // P12, "Always 0" (590:1565-1566)
	copy(b[29:38], "000000000") // P13 (590:1567-1568)
	copy(b[38:40], f.p14)
	b[40] = f.b41
	nameField := b[41:49]
	n := copy(nameField, f.name)
	for i := n; i < len(nameField); i++ {
		nameField[i] = ' '
	}
	b[49] = ';'
	return string(b)
}

// populatedFields is a valid, unremarkable FM channel for the canonical slot
// id: 145.500 MHz FM Normal, data mode off, TONE on with TN index 08 and CN
// index 08 (88.5 Hz on both charts), FILTER A, lockout off, named "SIMPLEX".
//
// FM is chosen deliberately: it is the one mode whose P14 the 590 book gives
// a printed meaning (590:1569-1571), so the read's mode name exercises the
// FM-N synthesis rather than sidestepping it.
func populatedFields(id string) recordFields {
	num, half, err := parseSlotID(id)
	if err != nil {
		panic("populatedFields: " + err.Error())
	}
	p1 := byte('0')
	if half == kw.ScanUpper {
		p1 = '1'
	}
	return recordFields{
		p1:   p1,
		p2:   byte('0' + num/100),
		p3:   string([]byte{byte('0' + (num/10)%10), byte('0' + num%10)}),
		freq: "00145500000",
		mode: '4', // FM (590:1358)
		b19:  '0',
		tone: '1', // TONE ON (590:1551)
		p8:   "08",
		p9:   "08",
		b28:  '0', // FILTER A (590:1560-1563)
		p14:  "00",
		b41:  '0',
		name: "SIMPLEX",
	}
}

// populatedMR is populatedFields' whole MR answer for id.
func populatedMR(id string) string { return populatedFields(id).frame() }

// emptyMR is the answer an EMPTY channel gives: "If the selected channel is
// empty, P4 ~ P15 will be 0 and P16 will be blank" (590:1492-1493) — A18a,
// documentary fact. Every byte of P4-P15 is '0' and P16 is eight spaces.
func emptyMR(id string) string {
	f := populatedFields(id)
	f.freq = "00000000000"
	f.mode = '0'
	f.b19 = '0'
	f.tone = '0'
	f.p8 = "00"
	f.p9 = "00"
	f.b28 = '0'
	f.p14 = "00"
	f.b41 = '0'
	f.name = ""
	return f.frame()
}
