// SPDX-License-Identifier: GPL-3.0-or-later

package ts480

import (
	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// mrAddr renders the four addressing bytes of an MR read of the canonical
// slot id — the key an image's mrAnswers and mrSilent maps use.
//
// P1 IS ALWAYS '0' ON THIS ROW, which is the whole of the read choreography's
// P1 story here: layout480 declares one flat SlotMemory range (480:955), so
// kw.Slot.P1 answers '0' for every slot this driver publishes and the P1=1
// half of channels 90-99 (480:943-944, 480:986-987) is never addressed.
// P2 IS ALWAYS '0' TOO, and for a different reason: it is a printed constant
// on this radio, "Always 0 for the TS-480." (480:953), where the 590 pair
// carry the channel's hundreds digit.
func mrAddr(id string) string {
	num, err := parseSlotID(id)
	if err != nil {
		panic("mrAddr: " + err.Error())
	}
	return string([]byte{'0', '0', byte('0' + num/10), byte('0' + num%10)})
}

// recordFields is the 50-byte MR answer's parameter block, field by field, in
// WIRE bytes, so a test states what the radio says rather than what a builder
// would produce for it.
//
// DELIBERATELY NOT kw.Layout.BuildMWSet — and on this row that is not merely
// preferable, it is the only route: A22 refuses every TS-480 channel write, so
// this driver builds no MW at all and a fixture drawn from the builder would
// pin the parser against a path the driver never takes. It would additionally
// refuse the malformed frames these tests need.
//
// THE SIXTEEN PRINTED-FIXED BYTES ARE FILLED HERE AS THE BOOK PRINTS THEM and
// are NOT fields of this struct — except byte4, which one test must be able
// to spoil deliberately (A24). The offsets are the 480 book's own printed
// positions (480:918-944 for the MR answer's chart), 1-indexed in the
// comments and 0-indexed in the code.
type recordFields struct {
	p1    byte   // position 3, "0: RX frequency, 1: TX frequency" (480:951)
	byte4 byte   // position 4, P2, "Always 0 for the TS-480." (480:953)
	p3    string // positions 5-6, "00 ~ 99: Memory channel number" (480:955)
	freq  string // positions 7-17, P4, 11 digits (480:957)
	mode  byte   // position 18, P5, via MD (480:959)
	b19   byte   // position 19, P6, the channel LOCKOUT on this book (480:962)
	tone  byte   // position 20, P7, three values (480:964)
	p8    string // positions 21-22, the TN index (480:966)
	p9    string // positions 23-24, the CN index (480:969)
	p14   string // positions 39-40, the STEP INDEX on this book (480:979)
	name  string // positions 42-49, P16, space-padded (A1) (480:984)
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
	b[3] = f.byte4
	copy(b[4:6], f.p3)
	copy(b[6:17], f.freq)
	b[17] = f.mode
	b[18] = f.b19
	b[19] = f.tone
	copy(b[20:22], f.p8)
	copy(b[22:24], f.p9)
	copy(b[24:27], "000") // P10, "Always 000 for the TS-480." (480:971)
	b[27] = '0'           // P11, "Always 0 for the TS-480." (480:973)
	b[28] = '0'           // P12, "Always 0 for the TS-480." (480:975)
	// P13, "Always 000000000 for the TS-480." (480:977).
	copy(b[29:38], "000000000")
	copy(b[38:40], f.p14)
	b[40] = '0' // P15, "Always 0 for the TS-480." (480:982)
	nameField := b[41:49]
	n := copy(nameField, f.name)
	for i := n; i < len(nameField); i++ {
		nameField[i] = ' '
	}
	b[49] = ';'
	return string(b)
}

// populatedFields is a valid, unremarkable channel for the canonical slot id:
// 145.500 MHz FM, lockout off, TONE on with TN index 08 and CN index 08, step
// index 00, named "SIMPLEX".
//
// P14 IS A STEP INDEX HERE AND NOT AN FM WIDTH FLAG, which is the divergence
// this fixture exists to keep visible: "00" is ST's own first index, 5 kHz in
// AM/FM and 0.5 kHz in SSB/CW/FSK (480:1494-1500). That the same two bytes
// mean FM Normal/Narrow on the 590 pair is why A22 refuses every write here
// and why this row publishes eight mode names rather than nine.
//
// THE TONE INDICES ARE WIRE VALUES THIS ROW CANNOT NAME. TN's and CN's charts
// are not in this book — "Refer to page 32 of the TS-480 instruction manual"
// (480:1559-1560) and page 33 for CN (480:339-340) — so the driver reads P8
// and P9 as bytes it must not translate into hertz (§1.9, Q2). The fixture
// carries printed-range values (00 ~ 42, 480:1557; 00 ~ 41, 480:337) because
// core/kw bounds both indices, not because 08 is known to mean anything.
func populatedFields(id string) recordFields {
	num, err := parseSlotID(id)
	if err != nil {
		panic("populatedFields: " + err.Error())
	}
	return recordFields{
		p1:    '0',
		byte4: '0',
		p3:    string([]byte{byte('0' + num/10), byte('0' + num%10)}),
		freq:  "00145500000",
		mode:  '4', // FM (480:847)
		b19:   '0', // lockout OFF (480:962)
		tone:  '1', // TONE (480:964)
		p8:    "08",
		p9:    "08",
		p14:   "00",
		name:  "SIMPLEX",
	}
}

// populatedMR is populatedFields' whole MR answer for id.
func populatedMR(id string) string { return populatedFields(id).frame() }

// emptyMR is the answer an empty channel would give: P4-P15 all zero and P16
// blank.
//
// THIS FIXTURE ASSERTS A4 AND DOES NOT OBSERVE IT. The rule it embodies is
// 590:1492-1493's, a TS-590SG sentence; the 480's book prints nothing about an
// empty channel anywhere, and whether a fresh TS-480 answers one at all is A4
// — the release gate for this row (L-HW-3). What the driver's own test can
// honestly claim is that IF such a frame arrives it is read as an empty
// channel rather than as a parse failure, which is a property of the codec and
// of this driver; it claims nothing about the radio.
func emptyMR(id string) string {
	f := populatedFields(id)
	f.freq = "00000000000"
	f.mode = '0'
	f.b19 = '0'
	f.tone = '0'
	f.p8 = "00"
	f.p9 = "00"
	f.p14 = "00"
	f.name = ""
	return f.frame()
}
