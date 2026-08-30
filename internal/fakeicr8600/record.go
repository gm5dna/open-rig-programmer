// SPDX-License-Identifier: GPL-3.0-or-later

package fakeicr8600

import "fmt"

// ---------------------------------------------------------------------------
// The printed record, transcribed
// ---------------------------------------------------------------------------

// field is one row of the memory-record diagrams as the two quarantined
// artefacts print and measure it: the index printed above the cells, the label
// printed in the legend, the byte positions measured on the render, and the
// printed width.
//
// UNLIKE ITS SIBLING FAKES, THIS PACKAGE DOES CONSULT ONE BYTE OF A RECORD.
// internal/fakeic905 and internal/fakeic7300 hold their records as opaque bytes
// and ask only how long they are, because on those radios the length names the
// layout. It cannot here: the IC-R8600's FM and DCR layouts are BOTH 44
// record-only bytes with different contents (geometry witness, "The six tails
// measured against one another"), so length alone names nothing and the mode
// byte must be read. Exactly one byte is read — modeByteOffset — and no other
// byte of any record is ever interpreted, compared or repaired.
type field struct {
	// Index is the index printed above the field's cells, with the printed
	// separator preserved: "(1), (2)" for a comma, "(6) ~ (10)" for a swung
	// dash.
	//
	// Written in PLAIN PARENTHESISED NUMERALS. The document draws every index
	// as a numeral inside a thin outlined circle, uniformly (both artefacts
	// record hazard (a) as NOT ENCOUNTERED), and Unicode's circled forms stop
	// at 50 while this record's indices run to 49 in some tails and would run
	// past 50 in none — but the geometry witness numbers positions to 49 and
	// the transcription uses the circled glyphs throughout. Rendering some
	// circled and some plain would falsely suggest two printed styles, so all
	// are plain here and the circle is recorded once, in this comment.
	Index string
	// Label is the legend line, verbatim, transcribed as printed and not
	// repaired.
	Label string
	// First and Last are the byte positions measured on the render, 1-based.
	// For headFields they are RECORD-ONLY positions (the printed data-area
	// position less the four address bytes); for a layout's Tail they are the
	// DATA-AREA positions the witness prints, which begin at 42 in every tail.
	First, Last int
	// Width is the printed width in bytes.
	Width int
}

// headFields is the common part of the record — diagram D1, the three linked
// strip rows under "● Memory channel content / Command: 1A 00" on PDF page 12
// (printed folio 11) of the IC-R8600 CI-V REFERENCE GUIDE, rev A7375-2EX-3a —
// as core/civ/icr8600/testdata/IC-R8600-transcription-b.csv transcribes it and
// IC-R8600-geometry-witness.csv measures it. The two agree on every offset,
// every width and every boundary.
//
// THE FOUR ADDRESS BYTES ARE NOT HERE. Printed indices (1),(2) (memory group
// number) and (3),(4) (memory channel number) are the channel's ADDRESS and
// travel ahead of the record in both directions; the witness measures the whole
// data area as 41 bytes and this table is the 37 that remain. The positions
// below are therefore the printed data-area position less four, which is what
// makes First run from 1 at printed index (5).
//
// ROW GRANULARITY follows the PRINTED HEADINGS, which is the geometry witness's
// grouping: "(18), (19) Tuning step (TS)" and "(20), (21) Programmable tuning
// step" are each one printed heading over two drawn cells, so each is one row
// here. The semantic transcription splits those pairs into a row per index —
// its stated convention, because each index carries its own annotation — and
// carries the pair's printed heading on both rows. The two artefacts are
// describing the same cells at different granularities; neither is wrong, and
// the byte positions they give are identical.
//
// The widths sum to 37 with no gap and no overlap, pinned by
// TestTheHeadTableIsGaplessAndSumsTo37.
var headFields = []field{
	{Index: "(5)", Label: "Skip/Select Memory scan setting", First: 1, Last: 1, Width: 1},
	{Index: "(6) ~ (10)", Label: "Receiving frequency", First: 2, Last: 6, Width: 5},
	// The transcription carries (11),(12) as ONE row, "Receiving mode", because
	// the record page prints one heading for the pair and defers to PDF page 10.
	// That page draws the pair as two separately tabled cells, "(1) Receiving
	// mode" then "(2) Filter setting", in that order — which is what makes the
	// FIRST of the two the mode byte. See modeByteOffset.
	{Index: "(11), (12)", Label: "Receiving mode", First: 7, Last: 8, Width: 2},
	{Index: "(13)", Label: "Duplex setting", First: 9, Last: 9, Width: 1},
	{Index: "(14) ~ (17)", Label: "Offset frequency", First: 10, Last: 13, Width: 4},
	{Index: "(18), (19)", Label: "Tuning step (TS)", First: 14, Last: 15, Width: 2},
	// The drawn nibble weights of this pair are 1 kHz, 100 Hz, 100 kHz, 10 kHz,
	// left to right — not monotonic, and the exact reverse of the printed label
	// order. Both artefacts record it as an observed disagreement and neither
	// reconciles it; nothing in this package depends on it, because the fake
	// stores these bytes without reading them.
	{Index: "(20), (21)", Label: "Programmable tuning step", First: 16, Last: 17, Width: 2},
	{Index: "(22)", Label: "Attenuator setting", First: 18, Last: 18, Width: 1},
	{Index: "(23)", Label: "Preamplifier setting", First: 19, Last: 19, Width: 1},
	{Index: "(24)", Label: "Antenna setting", First: 20, Last: 20, Width: 1},
	{Index: "(25)", Label: "IP plus (IP+) function", First: 21, Last: 21, Width: 1},
	{Index: "(26) ~ (41)", Label: "Memory name (Up to 16 characters)", First: 22, Last: 37, Width: 16},
}

// layout is one of the seven mode-selected record shapes: a neutral class name,
// the wire mode codes that select it, the tail the document draws for it, and
// the record-only length that results.
type layout struct {
	// Class is a NEUTRAL name for the mode class, not a printed string.
	Class string
	// Diagram is the transcription's own id for the block, and Heading its
	// printed heading, so a reader can find the page.
	Diagram string
	Heading string
	// Modes are the wire codes that select this layout. See modeClasses for the
	// assumption their VALUES rest on.
	Modes []byte
	// Tail is the drawn tail, in DATA-AREA positions beginning at 42.
	Tail []field
	// RecordLen is headRecordLen plus the tail's width.
	RecordLen int
}

// layouts is the seven shapes, one per drawn diagram plus the no-tail class the
// record page's note describes.
//
// The note beneath D1 on PDF page 12, printed in full: "In the modes other than
// FM and Digital, (42) and or later is not used. In the FM and Digital modes,
// entering (42) and or later can be omitted. The default value is applied to
// the omitted items." — "and or later" is printed twice, as shown; both
// artefacts record it as an observed disagreement and neither repairs it.
//
// FM and DCR are DELIBERATELY two layouts at one length. The geometry witness
// measures both tails as 7 drawn cells and divides them differently (FM 1+3+3,
// DCR 1+2+1+3), and records the coincidence of length rather than assuming it.
// Pinned by TestFMAndDCRAreTwoLayoutsAtOneLength.
var layouts = []layout{
	{
		Class:   "NONE",
		Diagram: "D1",
		Heading: "● Memory channel content (no mode tail)",
		// The eleven printed codes that are neither FM nor a digital mode.
		// WFM (06) is among them: the note names "FM and Digital" and the tail
		// page is headed "For receiving an FM signal", so WFM takes no tail.
		Modes: []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x06, 0x07, 0x08, 0x11, 0x14, 0x15},
		Tail:  nil,
	},
	{
		Class:   "D-STAR",
		Diagram: "D4",
		Heading: "For receiving a D-STAR signal",
		Modes:   []byte{0x17},
		Tail: []field{
			// This block prints 0=OFF and 2=CSQL and prints NO code 1, alone
			// among the six. Transcribed as printed, not repaired.
			{Index: "(42)", Label: "Digital squelch (D.SQL) type", First: 42, Last: 42, Width: 1},
			{Index: "(43)", Label: "Digital code squelch (CSQL) code", First: 43, Last: 43, Width: 1},
		},
	},
	{
		Class:   "P25",
		Diagram: "D3",
		Heading: "For receiving a P25 signal",
		Modes:   []byte{0x16},
		Tail: []field{
			{Index: "(42)", Label: "Digital squelch (D.SQL) type", First: 42, Last: 42, Width: 1},
			{Index: "(43) ~ (45)", Label: "NAC", First: 43, Last: 45, Width: 3},
		},
	},
	{
		Class:   "NXDN",
		Diagram: "D6",
		Heading: "For receiving an NXDN signal",
		// TWO wire codes, ONE layout: PDF page 10's mode table prints
		// "19: NXDN-VN" and "20: NXDN-N" separately, and PDF page 14 draws a
		// single diagram headed "For receiving an NXDN signal" for both.
		Modes: []byte{0x19, 0x20},
		Tail: []field{
			{Index: "(42)", Label: "Digital squelch (D.SQL) type", First: 42, Last: 42, Width: 1},
			{Index: "(43)", Label: "Radio Access Number (RAN) code", First: 43, Last: 43, Width: 1},
			{Index: "(44)", Label: "Encryption setting", First: 44, Last: 44, Width: 1},
			{Index: "(45) ~ (47)", Label: "Encryption key", First: 45, Last: 47, Width: 3},
		},
	},
	{
		Class:   "FM",
		Diagram: "D2",
		Heading: "For receiving an FM signal",
		Modes:   []byte{0x05},
		Tail: []field{
			// The only tail whose (42) is headed "Tone squelch type" rather
			// than "Digital squelch (D.SQL) type" — same position, different
			// printed name, recorded as an observed disagreement by the
			// geometry witness.
			{Index: "(42)", Label: "Tone squelch type", First: 42, Last: 42, Width: 1},
			{Index: "(43) ~ (45)", Label: "Tone squelch frequency", First: 43, Last: 45, Width: 3},
			{Index: "(46) ~ (48)", Label: "DTCS code", First: 46, Last: 48, Width: 3},
		},
	},
	{
		Class:   "DCR",
		Diagram: "D7",
		Heading: "For receiving a DCR signal",
		Modes:   []byte{0x21},
		Tail: []field{
			{Index: "(42)", Label: "Digital squelch (D.SQL) type", First: 42, Last: 42, Width: 1},
			{Index: "(43), (44)", Label: "UC code", First: 43, Last: 44, Width: 2},
			{Index: "(45)", Label: "Encryption setting", First: 45, Last: 45, Width: 1},
			{Index: "(46) ~ (48)", Label: "Encryption key", First: 46, Last: 48, Width: 3},
		},
	},
	{
		Class:   "dPMR",
		Diagram: "D5",
		Heading: "For receiving a dPMR signal",
		Modes:   []byte{0x18},
		Tail: []field{
			{Index: "(42)", Label: "Digital squelch (D.SQL) type", First: 42, Last: 42, Width: 1},
			{Index: "(43), (44)", Label: "COM ID", First: 43, Last: 44, Width: 2},
			{Index: "(45)", Label: "CC", First: 45, Last: 45, Width: 1},
			{Index: "(46)", Label: "Scrambler (SCRM) setting", First: 46, Last: 46, Width: 1},
			{Index: "(47) ~ (49)", Label: "Scrambler key", First: 47, Last: 49, Width: 3},
		},
	},
}

// The geometry the tables above fix, named once so nothing has to recount it.
const (
	// addressBytes is the width of the group and channel fields together —
	// printed indices (1)-(4), two bytes each, measured by both artefacts.
	addressBytes = 4
	// headRecordLen is the common head's record-only width: the witness's
	// 41-byte data area less the four address bytes.
	headRecordLen = 37
	// modeByteOffset is where the mode byte sits within a RECORD, zero-based.
	//
	// Printed index (11) is data-area byte 11; less the four address bytes it
	// is record byte 7, hence offset 6. That (11) is the MODE and (12) the
	// FILTER comes from the cross-reference the record page prints — "(11),
	// (12) Receiving mode / Refer to 'Receiving mode' (p. 9)" — and from the
	// referenced block on PDF page 10, which draws two cells and tables them
	// "(1) Receiving mode" and "(2) Filter setting" in that order.
	modeByteOffset = 6
	// clearFormLen and clearFormByte are the clear form printed at the foot of
	// PDF page 15's left column: the four address bytes, then a single "FF",
	// then "(6) ~ : None".
	clearFormLen  = addressBytes + 1
	clearFormByte = 0xFF
	// emptyReplyLen is how long an all-FF "this channel is empty" answer is
	// under WithEmptyReplyAllFF: the shortest accepted record. See doc.go
	// register entry 6 — the guide prints no such answer at all, so no length
	// is more evidenced than another, and the shortest invents the least.
	emptyReplyLen = headRecordLen
)

// modeClasses maps every printed wire mode code to the layout class it selects.
//
// THE VALUES ARE ASSUMED — doc.go register entry 2. PDF page 10's "(1)
// Receiving mode" table prints eighteen two-character codes (00-08, 11, 14-21)
// and never says whether a code is to be read as packed BCD or as a binary
// number. Under BCD "21" is the byte 0x21; under binary it is 0x15. The bytes
// here are the printed characters read as BCD, which is what every other
// numeric field in the guide uses and what the frozen golden vectors spell —
// but "natural" is not evidence, and a single capture would settle it.
//
// Built from the layouts above so there is one source of truth; pinned against
// the printed list by TestTheModeTableCarriesTheEighteenPrintedCodesAndNoOthers.
var modeClasses map[byte]string

// layoutIndexByClass resolves a class name to its index in layouts.
var layoutIndexByClass map[string]int

// init derives each layout's record length from the widths its tail declares,
// and builds the two indices. Deriving rather than restating means a mistyped
// width shows up as a wrong length in the tests that pin the accepted set,
// instead of hiding behind a length typed out beside it.
//
// Two collisions panic here rather than being tolerated: one wire code
// selecting two layouts, and two layouts sharing a class name. Both would make
// the discriminator ambiguous, and neither can be true of the printed material.
func init() {
	layoutIndexByClass = make(map[string]int, len(layouts))
	modeClasses = make(map[byte]string)

	for i := range layouts {
		l := &layouts[i]
		total := 0
		for _, f := range l.Tail {
			total += f.Width
		}
		l.RecordLen = headRecordLen + total

		if _, dup := layoutIndexByClass[l.Class]; dup {
			panic("fakeicr8600: two layouts named " + l.Class)
		}
		layoutIndexByClass[l.Class] = i

		for _, code := range l.Modes {
			if prior, dup := modeClasses[code]; dup {
				panic(fmt.Sprintf("fakeicr8600: mode code %#02X selects both %s and %s", code, prior, l.Class))
			}
			modeClasses[code] = l.Class
		}
	}
}

// layoutForRecord reads a record's mode byte and returns the layout it selects.
//
// It is the ONLY place a record's contents are looked at, and it looks at one
// byte. A record too short to carry a mode byte selects nothing; so does a mode
// byte the printed table does not list.
func layoutForRecord(record []byte) (layout, bool) {
	if len(record) <= modeByteOffset {
		return layout{}, false
	}
	class, ok := modeClasses[record[modeByteOffset]]
	if !ok {
		return layout{}, false
	}
	return layouts[layoutIndexByClass[class]], true
}

// ---------------------------------------------------------------------------
// What one channel holds
// ---------------------------------------------------------------------------

// MemState is this fake's representation of one memory channel's content: RAW
// WIRE FORM, the bytes that arrived, at the length they arrived, in the order
// they arrived.
//
// It is deliberately NOT a neutral struct of frequency, mode, tone and name.
// The fake decodes nothing and must never "fix up" a value on the way through.
// The single byte it reads (see field) decides which layout a record claims to
// be; it is never rewritten and never validated field by field.
type MemState struct {
	Record []byte
}

// Length is how many bytes this channel's record holds.
//
// A METHOD, not a field: a stored length beside a stored slice is two spellings
// of one number that can disagree, and length is load-bearing here.
func (s MemState) Length() int { return len(s.Record) }

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
// group field (printed indices (1),(2)) and the two-byte channel field (printed
// indices (3),(4)), verbatim. The fake keys its channel map on these RAW PAIRS
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
// The transcription records both address rows' encoding as bcd_packed and gives
// the reason: "every printed value is a four decimal-digit string carried in
// two bytes". The printed lists are the whole of the evidence —
//
//	(1),(2)  0000 ~ 0099 Normal memory channel group | 0100 Auto Write Memory
//	         channel group | 0101 Scan Skip channel group | 0102 Programmable
//	         Scan Edge channel group
//	(3),(4)  0000 ~ 0099 Normal memory channel (00 ~ 99) | 0000 ~ 0199 Auto
//	         Write Memory channels (A000 ~ A199)
//
// — and both fields are 0-BASED: the first Normal group is 0000 and the first
// channel of it is 0000, not 0001.
//
// It PANICS outside 0-9999, which no two-byte packed-BCD field can spell. Every
// caller is a test or a consumer's fixture, so a bad address is a programming
// error that must stop loudly rather than silently alias some other channel.
func bcd2(n int) [2]byte {
	if n < 0 || n > 9999 {
		panic(fmt.Sprintf("fakeicr8600: %d cannot be spelt in the printed two-byte address field (0-9999)", n))
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
