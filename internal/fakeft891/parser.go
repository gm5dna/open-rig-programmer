// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft891

// This file is fakeft891's own, independent byte-level CAT parser and reply
// builder for the FT-891. It is derived from that radio's own position charts
// in the FT-891 CAT Operation Reference Book, rev 1909-C — the Control
// Command List on printed folio 3, whose rows are cited line by line beside
// each section below, and the per-command charts cited with them — and NOT
// from core/cat. See doc.go for why that independence matters and for the
// full ASSUMED register; individual assumed points are flagged inline, next
// to the code that implements them.
//
// The manual itself is gitignored (docs/fixtures-private/manuals/), so the
// line references here are citations in the sense core/cat/ft891/doc.go uses
// them: they name where the chart is, they are not links.

// --- General framing ---

// rejection is the protocol's one and only NAK, "?;" — an unattributed
// generic command failure. Every refusal in this file answers with it and
// nothing else: an empty slot, an out-of-inventory slot, a malformed frame,
// an unknown command and an overflowed accumulator are indistinguishable to
// the host, which is the whole of the convention.
//
// THE CONVENTION ITSELF IS INHERITED AND NO FT-891 LINE IS CITED FOR IT
// ANYWHERE IN THIS REPOSITORY — doc.go's register entry THE "?;" REJECTION
// CONVENTION ITSELF IS INHERITED. It is core/cat's ErrRejected, adopted from
// the FT-710's reference (core/cat/errors.go:10-19).
var rejection = []byte("?;")

// maxAccumulatorBytes is the reassembler's byte cap — this package's own
// bounded-input policy, not a manual figure (doc.go's register entry THE
// FRAME ACCUMULATOR'S CAP AND RESYNC).
const maxAccumulatorBytes = 256

// reassembler turns an arbitrary stream of Write() chunks into complete
// ';'-terminated frames. Framing only: it says nothing about what any frame
// means.
//
// Overflow behaviour: once more than maxAccumulatorBytes bytes have
// accumulated without completing a frame, push reports one overflow event —
// the caller replies "?;" for it — and discards every byte from that point up
// to and including the next ';', then resumes normal framing
// (TestAccumulatorOverflowRejectsOnceAndResyncs). The zero value is not
// usable; construct with newReassembler.
type reassembler struct {
	buf       []byte
	max       int
	resyncing bool
}

func newReassembler(max int) *reassembler {
	if max <= 0 {
		max = maxAccumulatorBytes
	}
	return &reassembler{max: max}
}

// accEvent is one unit reassembler.push hands back: either a complete frame
// (terminator included) or an overflow signal (frame == nil, overflow true).
type accEvent struct {
	frame    []byte
	overflow bool
}

// push appends chunk to the internal buffer, byte by byte, and returns, in
// arrival order, every complete frame and overflow event it produced.
func (a *reassembler) push(chunk []byte) []accEvent {
	var events []accEvent
	for _, b := range chunk {
		if a.resyncing {
			if b == ';' {
				a.resyncing = false
			}
			continue
		}
		a.buf = append(a.buf, b)
		if b == ';' {
			frame := make([]byte, len(a.buf))
			copy(frame, a.buf)
			events = append(events, accEvent{frame: frame})
			a.buf = a.buf[:0]
			continue
		}
		if len(a.buf) > a.max {
			events = append(events, accEvent{overflow: true})
			a.buf = a.buf[:0]
			a.resyncing = true
		}
	}
	return events
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// --- Slot grammar ---
//
// MR's slot legend is the WIDEST this manual prints, and it prints the 5 MHz
// bank's ACTUAL NUMBERS: "001 - 099 (Regular Memory Channel)", "P1L - P9U
// (PMS)", "501 - 510 (5 MHz, U.S. and U.K. version only)" and "EMG
// (Emergency)" (ft891_layout.txt:960-964), repeated by IF (776, 778) and OI
// (1122, 1123). MT's and MW's legends print the first two classes only
// (998-999, 1035-1036) and MC's the same (907-909) — the disagreement
// core/cat/ft891/testdata/provenance.md records as item 5 and does not
// resolve.
//
// THE 5xx BOUNDS ARE TRANSCRIBED, NOT INHERITED, and this radio is the first
// Yaesu dialect of which that can be said: the FTdx10's and the FT-710's
// manuals print only "5xx (5MHz BAND)" and their 501..599 sits on their own
// ASSUMED registers, which is why internal/fakedx10's grammar takes any 5xx
// and this one does not (core/cat/ft891/doc.go, "WHAT IS DELIBERATELY NOT AN
// ENTRY"; TestParseSlotForm holds 511 and 599 as invalid).

// slotNoneWire is the answer-only "no slot" form. The wire spelling is the
// DIALECT's ASSUMED NoneWire (core/cat/ft891/doc.go's register entry
// "SlotSpace.NoneWire = \"000\""): it appears in no FT-891 slot legend. It is
// never a valid REQUEST slot here — a read or recall naming it is malformed —
// and it appears only in the MC answer of a radio sitting on a VFO.
const slotNoneWire = "000"

// slotEMGWire is the emergency channel's wire form, from MR's slot legend
// ("EMG (Emergency)", ft891_layout.txt:964).
const slotEMGWire = "EMG"

// slotWireLen is the width of every slot code on the wire: three bytes for
// every form in every legend.
const slotWireLen = 3

// The 5 MHz bank's printed bounds (ft891_layout.txt:962).
const (
	fiveMHzLo = 501
	fiveMHzHi = 510
)

// slotKind classifies a 3-byte slot code.
type slotKind int

const (
	slotInvalid slotKind = iota // malformed: none of the forms below
	slotNone                    // "000" — answer-only, never a valid request
	slotMemory                  // 001-099
	slotPMS                     // P1L-P9U
	slotFiveMHz                 // 501-510 (which values are POPULATED is a state question — image.go)
	slotEMG                     // "EMG"
)

// parseSlotForm classifies s per the slot legends above. It is a pure grammar
// check and says nothing about whether the slot is populated.
func parseSlotForm(s string) slotKind {
	if len(s) != slotWireLen {
		return slotInvalid
	}
	if s == slotEMGWire {
		return slotEMG
	}
	if s == slotNoneWire {
		return slotNone
	}
	if isDigit(s[0]) && isDigit(s[1]) && isDigit(s[2]) {
		n := int(s[0]-'0')*100 + int(s[1]-'0')*10 + int(s[2]-'0')
		switch {
		case n >= 1 && n <= 99:
			return slotMemory
		case n >= fiveMHzLo && n <= fiveMHzHi:
			return slotFiveMHz
		default:
			return slotInvalid // "100".."500", "511".."999"
		}
	}
	if s[0] == 'P' && s[1] >= '1' && s[1] <= '9' && (s[2] == 'L' || s[2] == 'U') {
		return slotPMS
	}
	return slotInvalid
}

// mrReadableSlot reports whether kind is a slot an MR read may name: the four
// classes MR's own legend lists (ft891_layout.txt:960-964). "000" is
// answer-only.
func mrReadableSlot(kind slotKind) bool {
	switch kind {
	case slotMemory, slotPMS, slotFiveMHz, slotEMG:
		return true
	}
	return false
}

// --- Field validators (wire level) ---
//
// Every one of them is enforced on the SET direction, and every one is
// ASSUMED to be what the radio itself enforces — doc.go's register entry
// SET-DIRECTION FIELD STRICTNESS.

// validModeWireByte reports whether b is a mode nibble this radio's legend
// admits. The legend is printed beside three commands — MR's P6
// (ft891_layout.txt:972-974), MT's P6 (1007-1010) and MW's P6 (1043-1046) —
// and all three are identical: 1..9, then a PRINTED HOLE at 'A' ("A: -"),
// then B, C, D, with 'E' and 'F' not printed at all. '0' is accepted
// additionally as the "-" placeholder, which appears in NO FT-891 legend and
// is the DIALECT's ASSUMED register entry "THE cat.ModeUnset MEMBER OF THE
// MODE TABLE" (cited): parsers must accept a placeholder even where builders
// must never emit one.
//
// NOTE THE DIVERGENCE FROM internal/fakedx10, which accepts 1..F: that
// radio's legend fills 'A', 'E' and 'F' (DATA-FM, PSK, DATA-FM-N). Six of
// this radio's twelve names disagree with the FTdx10's at the same nibble
// besides, which is why core/cat/ft891's mode table is transcribed afresh and
// why this one is too.
func validModeWireByte(b byte) bool {
	switch {
	case b == '0':
		return true
	case b >= '1' && b <= '9':
		return true
	case b >= 'B' && b <= 'D':
		return true
	}
	return false
}

// validCTCSSByte reports whether b is one of P8's three printed values,
// `0: CTCSS "OFF" 1: CTCSS ENC/DEC 2: CTCSS ENC` (ft891_layout.txt:1012 and
// four more blocks). CT's own P2 legend runs to a fourth value (3: DCS "ON",
// 410-414) and is NOT read across: that describes what the radio is doing
// now, not what the memory record can hold.
func validCTCSSByte(b byte) bool { return b >= '0' && b <= '2' }

// validShiftByte reports whether b is one of P10's three printed values,
// `0: Simplex 1: Plus Shift 2: Minus Shift` (ft891_layout.txt:1015 and four
// more blocks).
func validShiftByte(b byte) bool { return b >= '0' && b <= '2' }

func validBoolFlagByte(b byte) bool { return b == '0' || b == '1' }

// validClarSign reports whether b is one of P3's two printed direction bytes,
// "+: Plus Shift, -: Minus Shift" (ft891_layout.txt:1002 and four more
// blocks).
//
// THE MINUS BYTE IS THE ASCII HYPHEN-MINUS, 0x2D, and that is the DIALECT's
// ASSUMED register entry "THE CLARIFIER'S MINUS-DIRECTION BYTE" — cited here,
// never re-derived. Its own note records why: a rendered glyph in a PDF is
// not a byte value, and the extraction that produced the layout file is
// exactly where an en-dash would have been flattened to an ASCII hyphen
// without trace. If that entry's Stage R capture moves the byte, this
// validator and core/cat move together.
func validClarSign(b byte) bool { return b == '+' || b == '-' }

// validClarMagDigits reports whether s is a 4-digit clarifier magnitude field
// inside this manual's printed range.
//
// THE RANGE IS THE PRINTED ONE, 0000-9999, AND THE DIALECT'S NARROWER POLICY
// IS DELIBERATELY NOT APPLIED HERE. The manual prints "Clarifier Offset:
// 0000 - 9999 (Hz)" on every block carrying the field (MR 967, MT 1003, MW
// 1040, IF 781, OI 1126) and states NO STEP ANYWHERE.
// core/cat/ft891/dialect.go's ClarifierPolicy takes 10 Hz and 9990 — a
// DEDUCTION FROM AN ASSUMPTION, registered as one ("ClarifierPolicy.StepHz =
// 10 AND ClarifierPolicy.MaxAbsHz = 9990") — and 9990 is what this project
// permits ITSELF to build, not what this radio's manual says it accepts.
// This fake models the radio, so it takes the printed range; the consequence
// is that it accepts a magnitude core/cat would refuse to build, which is the
// same direction as internal/fakedx10's MT-Set slot leniency and is recorded
// in doc.go's register entry SET-DIRECTION FIELD STRICTNESS.
func validClarMagDigits(s string) bool {
	if len(s) != 4 {
		return false
	}
	for i := 0; i < 4; i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

// validTagField reports whether field is acceptable as a combined record's
// P12 tag field.
//
// THIS MANUAL STATES ITS OWN BYTE RULE, and it is broader than core/cat's
// default charset: "the parameter digits should be filled using any character
// except the ASCII control codes (00 to 1Fh) and the terminator (;)"
// (ft891_layout.txt:93-96, printed folio 2). P12's own legend says only "TAG
// Characters (up to 12 characters) (ASCII)" (1017) and names no set. So the
// check here is the folio-2 rule — not a control code, not ';' — where
// core/spec's Capabilities.TagByteOK default narrows to printable ASCII
// 0x20-0x7E for the FT-891's capability table (the conservative direction for
// what this project PUTS ON THE WIRE, which is a different question from what
// the radio accepts off it).
//
// SAFETY-CRITICAL, and it stays whatever hardware turns out to accept: ';' is
// the frame terminator, so a tag carrying one would make command injection
// possible. (The reassembler splits on ';' before a frame ever reaches here,
// which means that half of the check is unreachable through Port() — it is
// kept because unreachable-today is not a security property, and
// buildMTAnswer's field is written from stored state that WithSlot can set
// directly.)
func validTagField(field []byte) bool {
	for _, b := range field {
		if b < 0x20 || b == ';' {
			return false
		}
	}
	return true
}

// --- The shared memory field block ---
//
// This radio's MR Answer and MW Set are one 28-position chart under two
// prefixes (ft891_layout.txt:968-975 and 1034-1050), and the 41-position
// combined MT Set/Answer record (996-1027) carries that same block as its
// head. So there is one block layout in this file, used by every frame that
// carries channel data.
//
// Positions, from the charts' own 1-indexed numbering as counted twice by the
// geometry witness (core/cat/ft891/testdata/provenance.md §MT, §MR),
// expressed as 0-indexed offsets INTO THE BLOCK (i.e. into a frame's bytes
// after the two-byte command name):
//
//	pos(1-idx)  field                  block offset
//	3-5         P1  slot               [0:3]
//	6-14        P2  frequency, 9 dig   [3:12]
//	15          P3  clarifier sign     12
//	16-19       P3  clarifier mag      [13:17]
//	20          P4  RX clarifier       17
//	21          P5  fixed '0'          18
//	22          P6  mode nibble        19
//	23          P7  kind               20
//	24          P8  CTCSS state        21
//	25-26       P9  fixed "00"         [22:24]
//	27          P10 shift              24
//
// The MR/MW frame's ';' sits at position 28, immediately after the block; the
// combined record puts P11 there instead and continues with the tag field.

const memBlockLen = 25 // 3+9+1+4+1+1+1+1+1+2+1

const (
	blkSlotStart, blkSlotEnd       = 0, 3
	blkFreqStart, blkFreqEnd       = 3, 12
	blkClarSign                    = 12
	blkClarMagStart, blkClarMagEnd = 13, 17
	blkRXClar                      = 17
	blkP5                          = 18
	blkMode                        = 19
	blkKind                        = 20
	blkCTCSS                       = 21
	blkP9Start, blkP9End           = 22, 24
	blkShift                       = 24
)

// blkP5Fixed is P5's documented fixed value, position 21 — "P5 0: (Fixed)"
// (ft891_layout.txt:971, 1006, 1042, 783, 1129). It is required of a Set
// arriving and emitted in every answer unless a crafted MemState.P5 says
// otherwise.
const blkP5Fixed = '0'

// blkP9Fixed is P9's documented fixed value, positions 25-26 — "P9 00:
// (Fixed)" (ft891_layout.txt:1013).
const blkP9Fixed = "00"

// kindMemory is the P7 byte every populated slot in this fake answers with —
// memory channels, PMS pair members, 5 MHz channels and EMG alike.
//
// MR's legend prints "P7 0: VFO 1: Memory" (ft891_layout.txt:976) and MT's
// prints no read vocabulary at all ("P7 0: (Fixed)", 1011), so for a memory
// channel this is MR's legend read plainly and carried across to MT — which
// is exactly the cross-command inference core/cat/ft891/doc.go's register
// entry "THE COMBINED ANSWER'S P7 READ DOMAIN" records for the parse side.
// For PMS, 5xx and EMG it is doc.go's own register entry PMS, 5 MHz AND EMG
// SLOTS ANSWER P7 '1', because the legend has exactly two members and no
// manual statement says which of them those slots answer with.
//
// Note what this fake does NOT do: internal/fakeradio serves '5' (PMS) on the
// FT-710's MR answer, whose own P7 table runs 0-5 and has a member for it.
// This radio's legend documents two values, so inventing a third would
// produce a frame core/cat would rightly refuse to parse.
const kindMemory = '1'

// parseMemoryBlock validates a memBlockLen-byte field block against every
// vocabulary the charts print (doc.go's register entry SET-DIRECTION FIELD
// STRICTNESS) and returns the slot it names together with the state it
// encodes. ok is false on the first violation found; the caller answers "?;"
// and changes nothing.
//
// wantKind is the P7 byte the CALLING COMMAND's chart documents for its Set
// direction, passed in rather than assumed, because MT-Set P7 (1011) and
// MW-Set P7 (1047) are two command-specific facts that happen to coincide on
// this radio and deriving either from the other is the conflation
// core/cat/ft891/dialect.go's MWWriteKind comment records at length.
//
// The returned MemState carries the ANSWER kind, kindMemory, NOT wantKind: a
// Set's P7 is a fixed placeholder carrying no channel information, so there
// is nothing in it to store (see MemState.Kind). P5 is left zero — the
// schema's fixed '0' — because a Set that reached here carried exactly that
// byte. Tag and TagDisplay are left to the caller: they are outside this
// block.
func parseMemoryBlock(block []byte, wantKind byte) (slot string, s MemState, ok bool) {
	if len(block) != memBlockLen {
		return "", MemState{}, false
	}
	slot = string(block[blkSlotStart:blkSlotEnd])

	freq := block[blkFreqStart:blkFreqEnd]
	for _, b := range freq {
		if !isDigit(b) {
			return "", MemState{}, false
		}
	}
	sign := block[blkClarSign]
	if !validClarSign(sign) {
		return "", MemState{}, false
	}
	mag := string(block[blkClarMagStart:blkClarMagEnd])
	if !validClarMagDigits(mag) {
		return "", MemState{}, false
	}
	rx := block[blkRXClar]
	if !validBoolFlagByte(rx) {
		return "", MemState{}, false
	}
	// P5 is schema on this radio, so a Set carrying anything else is not a
	// frame this radio's charts describe — the ONE refusal internal/fakedx10
	// cannot have, because there the same byte is a TX clarifier flag.
	if block[blkP5] != blkP5Fixed {
		return "", MemState{}, false
	}
	mode := block[blkMode]
	if !validModeWireByte(mode) {
		return "", MemState{}, false
	}
	if block[blkKind] != wantKind {
		return "", MemState{}, false
	}
	ctcss := block[blkCTCSS]
	if !validCTCSSByte(ctcss) {
		return "", MemState{}, false
	}
	if string(block[blkP9Start:blkP9End]) != blkP9Fixed {
		return "", MemState{}, false
	}
	shift := block[blkShift]
	if !validShiftByte(shift) {
		return "", MemState{}, false
	}

	return slot, MemState{
		Freq: string(freq),
		// STORED, not zeroed — doc.go's register entry THE CLARIFIER IS
		// STORED, the deliberate non-borrowing of the FT-710's clarifier
		// hardware finding. This is the line that makes the combined Set
		// round-trip byte-faithfully.
		ClarSign: sign,
		ClarMag:  mag,
		RXClar:   rx == '1',
		Mode:     mode,
		// The ANSWER kind, never the Set's placeholder: memory, PMS, 5xx and
		// EMG slots all answer '1'.
		Kind:  kindMemory,
		CTCSS: ctcss,
		Shift: shift,
	}, true
}

func boolFlagByte(b bool) byte {
	if b {
		return '1'
	}
	return '0'
}

// appendMemBlock concatenates an already-validated MemState into its
// memBlockLen-byte field block. It trusts its input — state reaching here came
// from a validated Set or from an image constant — and returns no error.
func appendMemBlock(out []byte, slot string, s MemState) []byte {
	out = append(out, slot...)
	out = append(out, s.Freq...)
	out = append(out, s.ClarSign)
	out = append(out, s.ClarMag...)
	out = append(out, boolFlagByte(s.RXClar))
	// The zero value means the schema's fixed '0' — see MemState.P5.
	p5 := s.P5
	if p5 == 0 {
		p5 = blkP5Fixed
	}
	out = append(out, p5)
	out = append(out, s.Mode)
	out = append(out, s.Kind)
	out = append(out, s.CTCSS)
	out = append(out, blkP9Fixed...)
	out = append(out, s.Shift)
	return out
}

// --- MT: MEMORY WRITE & TAG, THE COMBINED FORM (availability 166; frames
// 996-1027) ---
//
// THE AVAILABILITY ROW AND THE DETAIL BLOCK CONTRADICT EACH OTHER on this
// radio, and it is the only one of the four registered Yaesu models of which
// that is true. The Control Command List gives MT "Set O, Read X, Ans. X"
// (ft891_layout.txt:166); MT's own block, on the same printed page, prints a
// filled Read chart ("M T P0 P0 P0 ;", 1016) and a filled 41-position Answer
// chart (1018-1027). core/cat/ft891/doc.go records the disagreement and
// deliberately does not settle it.
//
// THIS FAKE PLAYS BOTH RADIOS. By default it honours the detail block —
// doc.go's register entry MT READ IS ANSWERED — and WithMTReadUnsupported()
// makes it honour the command list, which is what puts core/driver/ft891's
// typed refusal within reach of a test against a real fake.
//
// Set frame and Answer frame are ONE 41-position chart: "MT" + the 25-byte
// shared field block + P11 + a 12-byte P12 tag field + ';'. The two
// directions are disambiguated purely by length: a Read body (after "MT",
// before ';') is exactly 3 bytes, a Set body exactly combinedBodyLen.
//
// THE READ'S SLOT DOMAIN IS MT's OWN LEGEND, memory and PMS (998-999), and
// nothing else. Its Read chart labels positions 3-5 "P0" and MT's legend
// column defines no P0 at all — the defect
// core/cat/ft891/testdata/provenance.md records as item 2 — so there is no
// second legend to read the read's domain from: the block's one slot legend
// is it. That is what core/cat/ft891/dialect.go's MTReadsMemoryPMS carries,
// and internal/fakedx10's opposite leniency is that radio's legend, not a
// shared rule.

// mtSetKindFixed is the combined Set's P7, "0: (Fixed)" (ft891_layout.txt:
// 1011). Deliberately its own constant rather than a reference to any other
// command's: MT-Set P7 and MW-Set P7 coincide as a fact of this radio
// (core/cat/ft891/dialect.go's MWWriteKind comment says exactly that about
// the same pair), not as a rule, and a radio that ever separated them would
// take one constant with it.
const mtSetKindFixed = '0'

// tagFieldLen is P12's fixed width, "TAG Characters (up to 12 characters)
// (ASCII)" (ft891_layout.txt:1017), drawn over positions 29-40.
const tagFieldLen = 12

// combinedBodyLen is a combined MT Set/Answer frame's body: everything after
// "MT" and before the trailing ';' — the shared field block, P11, and the tag
// field. 38, making the frame the counted 41
// (core/cat/ft891/testdata/provenance.md §MT).
const combinedBodyLen = memBlockLen + 1 + tagFieldLen

// Body offsets past the shared field block.
const (
	cmbP11                 = memBlockLen
	cmbTagStart, cmbTagEnd = memBlockLen + 1, memBlockLen + 1 + tagFieldLen
)

// tagFill is the byte a short tag is padded to width with, and the byte
// trimmed from an answer's field to recover the tag.
//
// A SPACE because the DIALECT says so — core/cat/ft891/doc.go's ASSUMED
// register entry "MTPolicy.TagFill = ' '", whose own note records that this
// manual's P12 legend names a width and an alphabet and no fill, and that no
// FT-891 has been asked. Cited here, not re-derived: if that entry's Stage R
// capture moves the byte, this constant moves with it.
const tagFill = ' '

// mtSlot reports whether kind is a slot MT may name, in EITHER direction:
// memory channels and PMS pairs only, per MT's own P1 legend
// (ft891_layout.txt:998-999). A MANUAL FACT of this radio, not a policy —
// TestMTRead_RefusesTheSlotsItsLegendDoesNotName and
// TestMTSet_RefusedOnTheSlotsItsLegendDoesNotName hold both directions
// against a slot MR answers in the same session.
func mtSlot(kind slotKind) bool { return kind == slotMemory || kind == slotPMS }

// buildMTAnswer builds the 41-byte combined MT answer for slot.
//
// ALWAYS THE FULL WIDTH, which is the DIALECT's ASSUMED register entry "THE
// COMBINED MT ANSWER'S EXACT LENGTH, 41" seen from the other side: that entry
// records that the manual's grid draws the MAXIMAL frame and that a
// variable-width ANSWER is live, with a recorded contingency of a 30..41
// window. This fake answers at the width core/cat expects (a fake that
// answered short would fail the parser rather than exercise it); if that
// entry's Stage R capture takes the contingency, this builder and core/cat
// move together.
//
// The tag field is written by copying the stored tag into a fixed-width,
// fill-initialised field, so a tag longer than tagFieldLen (only reachable
// through WithSlot — the wire cannot deliver one) is truncated rather than
// overflowing the frame.
func buildMTAnswer(slot string, s MemState) []byte {
	out := make([]byte, 0, 2+combinedBodyLen+1)
	out = append(out, 'M', 'T')
	out = appendMemBlock(out, slot, s)

	// P11 is a LIVE flag here, so it is written from stored channel data and
	// has no schema constant to fall back on — the difference from P5 two
	// lines up in the block, and from every registered sibling's byte 28.
	out = append(out, boolFlagByte(s.TagDisplay))

	field := make([]byte, tagFieldLen)
	for i := range field {
		field[i] = tagFill
	}
	copy(field, s.Tag)
	out = append(out, field...)

	out = append(out, ';')
	return out
}

func (r *Radio) handleMT(body []byte) []byte {
	switch len(body) {
	case slotWireLen:
		if r.mtReadUnsupported {
			// The COMMAND LIST's radio: MT is Set-only (ft891_layout.txt:166).
			// The refusal is deliberately indistinguishable from every other
			// "?;" — that is the whole difficulty core/driver/ft891 reports
			// rather than diagnoses.
			return rejection
		}
		slot := string(body)
		if !mtSlot(parseSlotForm(slot)) {
			return rejection
		}
		r.mu.Lock()
		s, ok := r.slots[slot]
		r.mu.Unlock()
		if !ok {
			// Empty slot — ASSUMED, doc.go's register entry EMPTY-SLOT
			// ANSWERS. This is the first frame of core/driver/ft891's
			// cross-check, and the MR that follows it is the second.
			return rejection
		}
		return buildMTAnswer(slot, s)

	case combinedBodyLen:
		slot, s, ok := parseMemoryBlock(body[:memBlockLen], mtSetKindFixed)
		if !ok {
			return rejection
		}
		if !mtSlot(parseSlotForm(slot)) {
			return rejection
		}
		// P11 is the LIVE TAG display flag on this radio, so both values are
		// legal and nothing else is — where every registered sibling requires
		// the fixed '0' here. TestMTSet_RejectionsLeaveTheChannelUntouched
		// holds the refusal.
		if !validBoolFlagByte(body[cmbP11]) {
			return rejection
		}
		s.TagDisplay = body[cmbP11] == '1'
		field := body[cmbTagStart:cmbTagEnd]
		if !validTagField(field) {
			return rejection
		}
		// Stored TRIMMED, answered PADDED — doc.go's register entry THE TAG
		// IS STORED TRIMMED AND ANSWERED PADDED. An all-fill field is no tag,
		// by the same rule, with no branch of its own. The FT-710's
		// HW-confirmed rejection of a ZERO-BYTE tag Set has no analogue: a
		// 41-byte frame always carries the full 12-byte field, so the shape
		// does not exist on this radio to accept or refuse.
		s.Tag = trimRightBytes(string(field), tagFill)

		r.mu.Lock()
		defer r.mu.Unlock()
		// The Set carries the WHOLE record, so it overwrites an existing
		// channel and CREATES an absent one, with no MW first — ASSUMED,
		// doc.go's register entry AN MT SET CREATES AN ABSENT CHANNEL, and
		// the fake's half of the driver register's "A SINGLE COMBINED MT SET
		// SUFFICES ...". An MT-only driver against a fake that demanded an MW
		// could not write at all.
		r.slots[slot] = s
		// The selection is NOT moved — doc.go's register entry A SET DOES NOT
		// MOVE THE SELECTED CHANNEL.
		return nil // fire-and-forget success
	}
	return rejection
}

// trimRightBytes returns s without its trailing runs of cut.
//
// strings.TrimRight would do, and this package could import strings — the
// hard rule forbids project-internal imports, not the standard library. It is
// four lines here so that the tag's trimming rule is visible beside the
// storage decision it implements rather than behind a call whose second
// argument is a cutset rather than a byte.
func trimRightBytes(s string, cut byte) string {
	end := len(s)
	for end > 0 && s[end-1] == cut {
		end--
	}
	return s[:end]
}

// --- MR: MEMORY CHANNEL READ (availability 164; frames 959-979) ---
//
// X O O X: no Set, a Read, an Answer, no AI push. Read frame (6 bytes) "MR" +
// 3-byte slot + ';'; Answer frame (28 bytes) "MR" + the field block + ';'.
//
// MR IS THIS RADIO'S ONLY ROUTE TO THE DISCOVERED BANKS. MT's legend names
// memory and PMS only, so core/driver/ft891 probes 501..510 and EMG by MR and
// reads those banks by MR alone — which is why every readable slot class is
// served here and why the answer carries no tag: the block stops at position
// 27, and Tag and TagDisplay read Unavailable on those banks in consequence.

// buildMRAnswer builds the 28-byte MR answer for slot.
func buildMRAnswer(slot string, s MemState) []byte {
	out := make([]byte, 0, 2+memBlockLen+1)
	out = append(out, 'M', 'R')
	out = appendMemBlock(out, slot, s)
	out = append(out, ';')
	return out
}

func (r *Radio) handleMR(body []byte) []byte {
	if len(body) != slotWireLen {
		// Includes an MR frame in the 28-byte SET shape: this radio's
		// availability row gives MR no Set direction at all (a manual fact,
		// not an assumption — ft891_layout.txt:164), so such a frame is
		// simply unknown. TestMR_HasNoSetDirection.
		return rejection
	}
	slot := string(body)
	if !mrReadableSlot(parseSlotForm(slot)) {
		return rejection
	}
	r.mu.Lock()
	s, ok := r.slots[slot]
	r.mu.Unlock()
	if !ok {
		// Empty slot — ASSUMED, doc.go's register entry EMPTY-SLOT ANSWERS.
		// This is the exchange core/driver/ft891 reads as "the slot is empty"
		// in its MT/MR cross-check and as "absent from this radio" during
		// 5xx/EMG discovery.
		return rejection
	}
	return buildMRAnswer(slot, s)
}

// --- ID (availability 147; frames 762-770) ---
//
// X O O X: no Set, Read "ID;", a 7-byte Answer. The VALUE is this radio's own
// CAT ID — the ID block prints "P1 0650: FT-891" (ft891_layout.txt:763) —
// and it is the one byte-level difference a probe turns into a
// *driver.WrongRadioError.

func buildIDAnswer() []byte { return []byte("ID0650;") }

func (r *Radio) handleID(body []byte) []byte {
	if len(body) != 0 {
		return rejection
	}
	return buildIDAnswer()
}

// --- AI (availability 117; frames 226-235) ---
//
// O O O O. Set and Answer are 4 bytes, Read is "AI;". AI-set is
// fire-and-forget. This fake never PUSHES anything unsolicited whatever AI is
// set to: no FT-891's AI behaviour has been observed, and the engine's
// drain-to-quiet discipline is already exercised against internal/fakeradio,
// whose own AI-flood facts are the FT-710's. THAT SUPPRESSION IS AN
// ASSUMPTION AND IS REGISTERED — doc.go's entry AUTOMATIC-INFORMATION
// SUPPRESSION. It is modelling silence as the honest default, not a claim
// that this radio is silent.
//
// core/transport.Engine.Init opens every session with an AI-off Set, so this
// handler's silent-accept path is on the critical path of every fake session.

func buildAIAnswer(ai byte) []byte { return []byte{'A', 'I', ai, ';'} }

func (r *Radio) handleAI(body []byte) []byte {
	switch len(body) {
	case 0:
		r.mu.Lock()
		ai := r.ai
		r.mu.Unlock()
		return buildAIAnswer(ai)
	case 1:
		if !validBoolFlagByte(body[0]) {
			return rejection
		}
		r.mu.Lock()
		r.ai = body[0]
		r.mu.Unlock()
		return nil // fire-and-forget success
	}
	return rejection
}

// --- MC: MEMORY CHANNEL (recall) (availability 160; frames 906-915) ---
//
// O O O X. Set frame (6 bytes) "MC" + 3-byte slot + ';', fire-and-forget,
// recalling the channel; Read frame "MC;" answered by "MC" + the 3-byte
// current channel + ';'. Disambiguated by length, as MT is.
//
// THIS RADIO'S MC LEGEND PRINTS TWO SLOT CLASSES ONLY — "001 - 099: Regular
// Memory Channel" and "P1L - P9U (PMS)" (ft891_layout.txt:907-909) — where
// every registered sibling's prints 5xx and EMG as well. So a 5 MHz channel
// this fake answers over MR cannot be recalled by MC. Not an assumption: this
// is the legend, transcribed, and it is what core/cat/ft891/dialect.go's
// MCSelectsMemoryPMS carries on the send side.
//
// A NOTE ON THE ANSWER DIRECTION, so that a later reader does not read a gap
// as a decision: core/cat/ft891/doc.go's register entry "THE MC ANSWER DOMAIN
// BEYOND MEMORY AND PMS" records that the codec will ACCEPT an MC answer
// naming a 5xx or EMG channel, because a radio put on one from the front
// panel would report it however narrow its Set domain is. This fake never
// produces such an answer — nothing here models a front panel, and only an
// MC-set moves the selection — so it neither exercises that tolerance nor
// contradicts it.
//
// MC-set of a slot this fake holds no state for answers "?;", paired with the
// read rule (doc.go's register entry EMPTY-SLOT ANSWERS): a channel with no
// stored data cannot be recalled.

// mcSettableSlot reports whether kind is a slot an MC Set may name, per MC's
// own legend. Its own function rather than a call to mtSlot: the two legends
// agree on this radio (907-909 against 998-999) as two separate facts that
// happen to coincide, and a radio that ever separated them would take one
// function with it.
func mcSettableSlot(kind slotKind) bool { return kind == slotMemory || kind == slotPMS }

func buildMCAnswer(current string) []byte {
	out := make([]byte, 0, 2+slotWireLen+1)
	out = append(out, 'M', 'C')
	out = append(out, current...)
	out = append(out, ';')
	return out
}

func (r *Radio) handleMC(body []byte) []byte {
	switch len(body) {
	case 0:
		r.mu.Lock()
		cur := r.currentChannel
		r.mu.Unlock()
		return buildMCAnswer(cur)

	case slotWireLen:
		slot := string(body)
		if !mcSettableSlot(parseSlotForm(slot)) {
			return rejection
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if _, ok := r.slots[slot]; !ok {
			return rejection // empty slot — the EMPTY-SLOT ANSWERS entry
		}
		r.currentChannel = slot
		return nil // fire-and-forget success
	}
	return rejection
}

// --- Top-level dispatch ---

// toUpperASCII folds one ASCII lower-case byte to upper case and leaves every
// other byte alone. Used on COMMAND NAMES ONLY — see handleFrame.
func toUpperASCII(b byte) byte {
	if b >= 'a' && b <= 'z' {
		return b - 'a' + 'A'
	}
	return b
}

// handleFrame parses one complete, ';'-terminated frame (as produced by
// reassembler.push) and returns the reply to send: nil for a fire-and-forget
// success, or a non-nil frame — a real answer, or rejection — otherwise.
// Unknown and garbled commands fall through to rejection.
//
// COMMAND NAMES ARE MATCHED IN EITHER CASE, and that is a MANUAL FACT of this
// radio rather than a leniency inherited from fakeradio: "A command consists
// of 2 alphabetical characters. You may use either lower or upper case
// charac-/ters." (hyphenated across the column break, ft891_layout.txt:
// 100-102, under the "Alphabetical Commands" heading at 99) — the same
// sentence the FTdx10's manual states (ftdx10_layout.txt:160-161), which
// internal/fakedx10 already honours on this line. TestCommandNamesAre
// AcceptedInEitherCase pins it, including the mixed-case form: "either lower
// or upper" says nothing about mixing, so admitting it is a CONSEQUENCE of
// folding each byte independently, not a separate invented leniency. See
// doc.go's "What is NOT in this register, and why".
//
// FIELD VALUES REMAIN CASE-SENSITIVE (the mode nibble's hex letters, the PMS
// L/U suffix, "EMG"): the manual's statement is about the two-character
// command NAME and says nothing about parameters, so extending it would be an
// invented leniency.
func (r *Radio) handleFrame(frame []byte) []byte {
	if len(frame) == 0 || frame[len(frame)-1] != ';' {
		return rejection // defensive: the reassembler never hands us this
	}
	body := frame[:len(frame)-1]
	if len(body) < 2 {
		return rejection
	}
	cmd := [2]byte{toUpperASCII(body[0]), toUpperASCII(body[1])}
	rest := body[2:]

	switch cmd {
	case [2]byte{'I', 'D'}:
		return r.handleID(rest)
	case [2]byte{'A', 'I'}:
		return r.handleAI(rest)
	case [2]byte{'M', 'R'}:
		return r.handleMR(rest)
	case [2]byte{'M', 'T'}:
		return r.handleMT(rest)
	case [2]byte{'M', 'C'}:
		return r.handleMC(rest)
	case [2]byte{'E', 'X'}:
		// EX (MENU), READ ONLY — ex.go, which carries this radio's own
		// four-digit grammar (a SEVEN-byte read frame where every registered
		// sibling's is nine, ft891_layout.txt:513-522) and the inventory
		// generated from this package's own copy of transcription B. A Set
		// draws "?;" as a too-long body: a modelling gap, stated there.
		return r.handleEX(rest)
	default:
		return rejection
	}
}
