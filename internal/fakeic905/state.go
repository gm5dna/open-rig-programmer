// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic905

import "fmt"

// ---------------------------------------------------------------------------
// What one channel holds
// ---------------------------------------------------------------------------

// MemState is this fake's in-memory representation of one memory channel's
// content, and it is RAW WIRE FORM AND NOTHING ELSE: the bytes that arrived, at
// the length they arrived, in the order they arrived.
//
// It is deliberately NOT a neutral struct of frequency, mode, tone and name.
// The fake does not know what any byte of a record means, must not learn, and
// must never "fix up" a value on the way through — that is the whole of its
// value as a test double (see doc.go, THE HARD RULE). Every rule this package
// applies to a record is a rule about its LENGTH, never about its content.
type MemState struct {
	// Record is the raw record: the bytes that follow the four address bytes
	// in a `1A 00` frame. For the record the printed diagram measures that is
	// bytes 5-68 of 68 — sixty-four of them — but this field holds ANY length,
	// because WithRecord seeds any length and a set to an unheld channel
	// accepts any length. See recordFields below for what the diagram prints
	// at each of those offsets, and note that nothing in this package consults
	// that table when handling a frame.
	Record []byte
}

// Length is how many bytes this channel's record holds.
//
// It is a METHOD, not a field. The brief asks MemState to hold "a []byte record
// and its length", and it does — but as one fact, not two: a stored length
// beside a stored slice is two spellings of one number that can disagree, and
// the record-length rejection rule is the single most load-bearing rule in this
// package. A method cannot disagree with the slice it measures.
func (s MemState) Length() int { return len(s.Record) }

// clone returns an independent copy, so neither an Option's caller nor a
// Record() caller shares the fake's own backing array.
func (s MemState) clone() MemState {
	if s.Record == nil {
		return MemState{}
	}
	out := make([]byte, len(s.Record))
	copy(out, s.Record)
	return MemState{Record: out}
}

// ---------------------------------------------------------------------------
// Addressing: the two printed two-byte fields
// ---------------------------------------------------------------------------

// chanAddr is one channel's address as it appears on the wire: the two-byte
// group field (printed indices 1, 2) and the two-byte channel field (printed
// indices 3, 4), verbatim. The fake keys its channel map on these RAW PAIRS
// rather than on a pair of Go ints, so a request addressed with bytes the fake
// never issued still finds — or misses — exactly the entry those bytes name.
type chanAddr struct {
	group   [2]byte
	channel [2]byte
}

func (a chanAddr) String() string {
	return fmt.Sprintf("%02X %02X / %02X %02X", a.group[0], a.group[1], a.channel[0], a.channel[1])
}

// bcd2 encodes n as the two-byte address field the diagram prints, high byte
// first, packed BCD.
//
// ASSUMED — doc.go register entry 6. Both address rows of
// ic905-transcription-b.csv record their `encoding` column as `unstated`: the
// page prints permitted VALUES and never says how to read them. The values it
// prints are the whole of the evidence for this mapping, and they read as BCD
// in both fields and in both directions:
//
//	①, ②  "00 00 ~ 00 99: Memory channel group" and "01 00: Call channel group"
//	③, ④  "00 00 ~ 00 99: 00 ~ 99", and "00 10, 00 11: 10G C1, C2"
//
// `00 10` for the tenth call channel rather than `00 0A` is the reading that
// settles it: a plain binary field would print 0A there. On that reading the
// call-channel GROUP, `01 00`, is group 100, which is why WithRecord(100, …)
// addresses it.
//
// It PANICS outside 0-9999, which no two-byte packed-BCD field can spell. Every
// caller is a test or a consumer's fixture, so a bad address is a programming
// error that must stop loudly rather than silently alias some other channel.
func bcd2(n int) [2]byte {
	if n < 0 || n > 9999 {
		panic(fmt.Sprintf("fakeic905: %d cannot be spelt in the printed two-byte address field (0-9999)", n))
	}
	return [2]byte{
		byte((n/1000%10)<<4 | (n / 100 % 10)),
		byte((n/10%10)<<4 | (n % 10)),
	}
}

// addrOf builds the wire address for a group and channel number.
func addrOf(group, channel int) chanAddr {
	return chanAddr{group: bcd2(group), channel: bcd2(channel)}
}

// ---------------------------------------------------------------------------
// The printed record, transcribed — NAMING ONLY
// ---------------------------------------------------------------------------

// recordField is one row of the memory-record diagram as the two artefacts
// print it: the index printed above the cells, the label printed in the legend,
// and the byte positions measured on the render.
//
// NOTHING IN THIS PACKAGE READS THIS TABLE WHEN HANDLING A FRAME. It exists so
// that a reader of this file can see what each offset of a stored record is,
// and so that the transcription can be pinned by a test — not so that the fake
// can interpret a record, which it must never do.
type recordField struct {
	// Index is the index printed above the field's cells, with the printed
	// separator preserved: "1, 2" for a comma, "6~10" for a swung dash.
	//
	// Written in PLAIN NUMERALS, following ic905-geometry-witness.md's own
	// stated convention: every index in the diagram is drawn as a numeral
	// inside a thin circle, but Unicode has no circled forms above 50 and the
	// band prints 52, 53 and 68, so writing some circled and some plain "would
	// falsely suggest two printed styles". The circle is uniform across all
	// eighteen indices and is recorded here, once.
	Index string
	// Label is the legend line, verbatim, transcribed as printed and not
	// repaired — including the two identical labels at 16~18 and 19~21 (STOP 1
	// in both artefacts) and the stray full stop inside "(8 characters,
	// fixed.)" at 37~44.
	Label string
	// First and Last are the byte positions measured on the render, 1-based,
	// counted left to right across the wrapped two-row block.
	First, Last int
	// Width is the printed width in bytes.
	Width int
}

// d1RecordFields is diagram D1 — the section headed "• Memory content" /
// "Command: 1A 00" on PDF page 19 (printed folio 18) of the IC-905 CI-V
// REFERENCE GUIDE, rev A7711-9EX-2 — transcribed from
// core/civ/ic905/testdata/ic905-transcription-b.csv and cross-read against
// core/civ/ic905/testdata/ic905-geometry-witness.csv, which measured the same
// block independently. The two agree on every offset, every width and every
// boundary.
//
// The widths sum to 68, the highest printed index, with no gap and no overlap —
// the arithmetic both artefacts state for themselves, pinned by
// TestTheRecordFieldTableIsGaplessAndSumsTo68.
var d1RecordFields = []recordField{
	{Index: "1, 2", Label: "Memory group number", First: 1, Last: 2, Width: 2},
	{Index: "3, 4", Label: "Memory channel numbers", First: 3, Last: 4, Width: 2},
	{Index: "5", Label: "Split and Select memory setting", First: 5, Last: 5, Width: 1},
	{Index: "6~10", Label: "Operating frequency setting", First: 6, Last: 10, Width: 5},
	{Index: "11, 12", Label: "Operating mode setting", First: 11, Last: 12, Width: 2},
	{Index: "13", Label: "Data mode setting", First: 13, Last: 13, Width: 1},
	{Index: "14", Label: "Duplex and Tone settings", First: 14, Last: 14, Width: 1},
	{Index: "15", Label: "Digital squelch setting", First: 15, Last: 15, Width: 1},
	// STOP 1, carried by both artefacts: the legend prints these two lines
	// word for word the same, for two distinct three-byte ranges, whilst the
	// single pointer they share names TWO settings ("Repeater tone/tone
	// squelch frequency setting"). Neither artefact repairs it and neither
	// does this table.
	{Index: "16~18", Label: "Repeater tone frequency setting", First: 16, Last: 18, Width: 3},
	{Index: "19~21", Label: "Repeater tone frequency setting", First: 19, Last: 21, Width: 3},
	{Index: "22~24", Label: "DTCS code setting", First: 22, Last: 24, Width: 3},
	{Index: "25", Label: "DV Digital code squelch setting", First: 25, Last: 25, Width: 1},
	{Index: "26~28", Label: "Duplex offset frequency setting", First: 26, Last: 28, Width: 3},
	{Index: "29~36", Label: "UR (Destination) call sign setting (8 characters, fixed)", First: 29, Last: 36, Width: 8},
	// The stray full stop is printed. Observed disagreement 1 in both
	// artefacts; transcribed, not reconciled.
	{Index: "37~44", Label: "R1 (Access repeater) call sign setting (8 characters, fixed.)", First: 37, Last: 44, Width: 8},
	{Index: "45~52", Label: "R2 (Gateway/Link repeater) call sign setting (8 characters, fixed)", First: 45, Last: 52, Width: 8},
	{Index: "53~68", Label: "Memory name setting (16 characters, fixed)", First: 53, Last: 68, Width: 16},
}

// d2ClearFields is the D2 block — the four indexed lines printed at the foot of
// the same legend under "To clear the memory channel contents on 1A 00:" —
// transcribed independently of D1, as ic905-transcription-b.csv transcribes it.
// D2 has no drawn diagram, so it has no measured position and no printed width;
// those cells are empty there and the widths are zero here.
//
// It is what makes the clear form recognisable: four address bytes followed by
// a single FF, and nothing after it.
var d2ClearFields = []recordField{
	{Index: "1, 2", Label: "Memory channel group (00 00 ~ 00 99)"},
	{Index: "3, 4", Label: "Memory channel (00 00 ~ 00 99)"},
	// No label is printed for index 5 in this block, only the value `"FF,"`.
	{Index: "5", Label: ""},
	// Printed as an open-ended range — a circled 6 followed by a spaced tilde
	// with no upper bound — against the value "None".
	{Index: "6 ~", Label: ""},
}

// The geometry the two tables above fix, named once so nothing has to recount
// it. These ARE consulted at run time; the tables above are not.
const (
	// addressBytes is the width of the group and channel fields together —
	// printed indices 1-4, two bytes each.
	addressBytes = 4
	// recordFirstByte and recordLastByte bound what this fake calls "the
	// record": everything the printed block holds after the address.
	recordFirstByte = addressBytes + 1 // 5
	recordLastByte  = 68
	// printedRecordLen is how many bytes that is: 64.
	//
	// IT IS NOT A RULE OF THIS FAKE. The fake holds a record of any length at
	// all and rejects a set only for disagreeing with the length it already
	// holds. {64, 65} is the DRIVER's fingerprint, and this constant exists to
	// document where the 64 in it comes from and to size the default image —
	// nothing else consults it.
	printedRecordLen = recordLastByte - recordFirstByte + 1
	// clearFormLen and clearFormByte are the D2 form: four address bytes then
	// a single FF, five in all, with nothing after it ("⑥ ~ : None").
	clearFormLen  = addressBytes + 1
	clearFormByte = 0xFF
)
