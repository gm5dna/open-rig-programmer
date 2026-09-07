// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft991a

// This file is fakeft991a's own, independent byte-level CAT parser and reply
// builder for the FT-991A. It is derived from that radio's own position charts
// in the FT-991A CAT Operation Reference Manual, revision 1711-D — the Control
// Command List on printed folio 3, whose rows are cited line by line beside
// each section below, and the per-command charts cited with them — and NOT
// from core/cat. See doc.go for why that independence matters and for the full
// ASSUMED register; individual assumed points are flagged inline, next to the
// code that implements them.
//
// The manual itself is gitignored (docs/fixtures-private/manuals/), so the
// line references here are citations in the sense core/cat/ft991a/doc.go uses
// them: they name where the chart is, they are not links.

import (
	"bytes"
	"strings"
)

// --- General framing ---

// rejection is the protocol's one and only NAK, "?;" — an unattributed
// generic command failure. Every refusal in this file answers with it and
// nothing else: an empty slot, an out-of-inventory slot, a malformed frame, an
// unknown command and an overflowed accumulator are indistinguishable to the
// host, which is the whole of the convention.
//
// THE CONVENTION IS THE DIALECT'S ASSUMPTION, CITED NOT RE-REGISTERED —
// core/cat/ft991a/doc.go's register entry "THE ACKNOWLEDGEMENT CONVENTIONS",
// which records that this manual "describes no ACK/NAK vocabulary beyond the
// '?;' every Yaesu CAT manual in this repository shows for a rejected command,
// and it never says whether an accepted Set answers at all". Both halves of
// this fake's behaviour — one "?;" on a refusal, silence on an accepted Set —
// are that one entry, and its Stage R lifting capture (one write session's raw
// transcript, every byte in both directions) moves this file with it.
var rejection = []byte("?;")

// maxAccumulatorBytes is the reassembler's byte cap — this package's own
// bounded-input policy, not a manual figure (doc.go's register entry THE FRAME
// ACCUMULATOR'S CAP AND RESYNC).
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
// THIS RADIO HAS ONE SLOT NUMBER LINE AND NOTHING ELSE. Every legend in this
// manual that names a slot prints the same span: MC's "001 - 117: Memory
// Channel Number" (ft991a_layout.txt:913), and the bare "001-117 (Memory
// Channel)" of MR (966), MT (999), MW (1037), IF (783) and OI (1117). MC's is
// the ONLY one that decomposes it — "001 - 099: Regular Memory Channel" and
// "100: P-1L 101: P-1U ~ 116: P-9L 117: P-9U" (915-916) — which is the
// difference core/cat/ft991a/testdata/provenance.md records and does not
// resolve.
//
// TWO CONSEQUENCES, AND BOTH BITE A COPY FROM internal/fakeft891:
//
//   - THE PMS SLOTS ARE DECIMAL CHANNEL NUMBERS, not the token "P1L". Every
//     registered sibling's legend spells them "P1L - P9U (PMS)"; this one
//     numbers them. So "P1L" is not a slot form here and is refused, which
//     TestParseSlotForm holds. Not an assumption: this is the legend,
//     transcribed, and it is why core/cat/ft991a/doc.go deliberately carries
//     NO register entry for the numbering.
//   - THERE IS NO 5 MHz BANK AND NO EMERGENCY CHANNEL. "5xx", "5 MHz" and
//     "EMG" appear in no slot legend of this manual — checked mechanically
//     over the whole extraction by core/cat/ft991a/dialect.go:114-123 and
//     :141-149 — where the FT-891's and the FTdx10's MR legends print both.
//     Transcribed absences, not guesses, so "501" and "EMG" are simply not
//     slots.

// slotNoneWire is the answer-only "no slot" form. The wire spelling is the
// DIALECT's ASSUMED NoneWire (core/cat/ft991a/doc.go's register entry
// `SlotSpace.NoneWire = "000"`): it appears in no FT-991A slot legend. It is
// never a valid REQUEST slot here — a read or recall naming it is malformed —
// and it appears only in the MC answer of a radio sitting on a VFO.
const slotNoneWire = "000"

// slotWireLen is the width of every slot code on the wire: three bytes, the
// three P1 cells every chart in this manual draws for the field.
const slotWireLen = 3

// The printed span's own bounds and its printed decomposition
// (ft991a_layout.txt:913-916).
const (
	memoryLo = 1
	memoryHi = 99
	pmsLo    = 100
	pmsHi    = 117
)

// slotKind classifies a 3-byte slot code.
type slotKind int

const (
	slotInvalid slotKind = iota // malformed: none of the forms below
	slotNone                    // "000" — answer-only, never a valid request
	slotMemory                  // 001-099
	slotPMS                     // 100-117, the nine P-1L..P-9U pairs
)

// parseSlotForm classifies s per the slot legends above. It is a pure grammar
// check and says nothing about whether the slot is populated.
func parseSlotForm(s string) slotKind {
	if len(s) != slotWireLen {
		return slotInvalid
	}
	if s == slotNoneWire {
		return slotNone
	}
	if !isDigit(s[0]) || !isDigit(s[1]) || !isDigit(s[2]) {
		// Includes the SIBLINGS' PMS token form, "P1L".."P9U", which this
		// radio's legend does not print.
		return slotInvalid
	}
	n := int(s[0]-'0')*100 + int(s[1]-'0')*10 + int(s[2]-'0')
	switch {
	case n >= memoryLo && n <= memoryHi:
		return slotMemory
	case n >= pmsLo && n <= pmsHi:
		return slotPMS
	}
	return slotInvalid // "118".."999"
}

// mrReadableSlot reports whether kind is a slot an MR read may name — the span
// MR's own legend prints, "P0/1 001-117 (Memory Channel)"
// (ft991a_layout.txt:966). "000" is answer-only.
func mrReadableSlot(kind slotKind) bool { return kind == slotMemory || kind == slotPMS }

// mtSlot reports whether kind is a slot MT may name, in EITHER direction, per
// MT's own legend (ft991a_layout.txt:999).
//
// Its own function rather than a call to mrReadableSlot: the two legends agree
// on this radio as two separate facts that happen to coincide, and a radio
// that ever separated them — as the FT-891 does, whose MT legend names two
// classes where its MR legend names four — would take one function with it.
func mtSlot(kind slotKind) bool { return kind == slotMemory || kind == slotPMS }

// mcSettableSlot reports whether kind is a slot an MC Set may name, per MC's
// own legend (ft991a_layout.txt:913-916) — its own function for the reason
// mtSlot gives.
func mcSettableSlot(kind slotKind) bool { return kind == slotMemory || kind == slotPMS }

// --- Field validators (wire level) ---
//
// Every one of them is enforced on the SET direction, and every one is ASSUMED
// to be what the radio itself enforces — doc.go's register entry
// SET-DIRECTION FIELD STRICTNESS.

// validModeWireByte reports whether b is a mode nibble this radio's legend
// admits. The legend is printed beside FIVE commands — MR's P6
// (ft991a_layout.txt:973-975), MT's (1006-1008), MW's (1044-1046), IF's
// (789-791) and OI's (1124-1126) — and all five are identical: 1..9 then
// A..E, with NO hole and no 'F'. '0' is accepted additionally as the "-"
// placeholder, which appears in NO FT-991A legend and is the DIALECT's ASSUMED
// register entry "THE cat.ModeUnset MEMBER OF THE MODE TABLE" (cited):
// parsers must accept a placeholder even where builders must never emit one.
//
// IT IS NOT THE PREDICATE AN INCOMING SET IS JUDGED BY. parseMemoryBlock uses
// image.go's validModeBuildByte — this predicate minus the placeholder —
// because a Set's P6 vocabulary is the printed legend alone (the closing
// review's C-M2). This one states the LEGEND-plus-placeholder domain, which is
// what the register entry is about, and is the base the build-direction
// predicate narrows.
//
// NOTE THE DIVERGENCES FROM BOTH SIBLINGS. internal/fakeft891 refuses 'A'
// (its legend prints "A: -", a hole) and 'E'; internal/fakedx10 accepts 'F'
// (its legend fills it with DATA-FM-N). This one takes 1..9 and A..E, and its
// 'E' is C4FM where the FTdx10's is PSK — one nibble, two different real
// modes, which is why core/cat/ft991a's mode table is transcribed afresh and
// why this one is too.
func validModeWireByte(b byte) bool {
	switch {
	case b == '0':
		return true
	case b >= '1' && b <= '9':
		return true
	case b >= 'A' && b <= 'E':
		return true
	}
	return false
}

// validCTCSSByte reports whether b is one of P8's FIVE printed values,
// `0: CTCSS "OFF" 1: CTCSS ENC/DEC 2: CTCSS ENC 3: DCS ENC/DEC 4: DCS ENC`,
// printed identically on IF (ft991a_layout.txt:795-796), MR (977-978), MT
// (1010-1011), MW (1048-1049) and OI (1128-1129). Every registered sibling
// prints 0/1/2 and nothing else.
//
// THAT THE RADIO ACCEPTS THE TWO DCS STATES ON A SET IS THE DIALECT'S
// ASSUMPTION, cited here and not re-registered: its entry "THE DCS STATES' SET
// ACCEPTANCE" records that this manual never says whether a DCS state may be
// written into a memory without a DCS code having been set first, CN carrying
// the code as a separate command (364-374). This fake accepts what the legend
// prints, which is the honest default for a field nobody has watched a radio
// handle; if that entry's Stage R capture refuses it, this validator and the
// driver's write path move together.
//
// CT's and CN's own legends are NOT read across: they describe what the radio
// is doing now, not what the memory record can hold.
func validCTCSSByte(b byte) bool { return b >= '0' && b <= '4' }

// validShiftByte reports whether b is one of P10's three printed values,
// `0: Simplex 1: Plus Shift 2: Minus Shift` (ft991a_layout.txt:1014 and four
// more blocks).
func validShiftByte(b byte) bool { return b >= '0' && b <= '2' }

func validBoolFlagByte(b byte) bool { return b == '0' || b == '1' }

// validClarSign reports whether b is one of P3's two printed direction bytes,
// "Clarifier Direction +: Plus Shift, --: Minus Shift"
// (ft991a_layout.txt:1000 and four more blocks).
//
// THE MINUS BYTE IS THE ASCII HYPHEN-MINUS, 0x2D, and that is the DIALECT's
// ASSUMED register entry "THE CLARIFIER'S MINUS-DIRECTION BYTE, the ASCII
// HYPHEN-MINUS 0x2D ('-')" — cited here, never re-derived. THIS RADIO IS THE
// WORST OF THE FAMILY FOR IT: every block carrying the legend prints TWO
// hyphens (IF 784, MR 967, MT 1000, MW 1038, OI 1118) where the FT-891 prints
// one, and evidence leg G looked at the render at 1800 dpi and found the
// doubling REAL rather than an extraction artefact
// (core/cat/ft991a/testdata/provenance.md §Disagreements, item 1). P3 has five
// positions against a four-digit offset, so exactly one is left for the
// direction and a two-character minus cannot fit the counted frame. Which
// single byte the wire wants is still undecided by this manual; if that
// entry's Stage R capture moves it, this validator and core/cat move together.
func validClarSign(b byte) bool { return b == '+' || b == '-' }

// validClarMagDigits reports whether s is a 4-digit clarifier magnitude field
// inside this manual's printed range.
//
// THE RANGE IS THE PRINTED ONE, 0000-9999, AND THE DIALECT'S NARROWER POLICY
// IS DELIBERATELY NOT APPLIED HERE. The manual prints "Clarifier Offset: 0000
// - 9999 (Hz)" on every block carrying the field (IF 785, MR 968, MT 1001, MW
// 1039, OI 1119) and states NO STEP ANYWHERE. core/cat/ft991a/dialect.go's
// ClarifierPolicy takes 10 Hz and 9990 — a DEDUCTION FROM AN ASSUMPTION,
// registered as one ("ClarifierPolicy.StepHz = 10 AND
// ClarifierPolicy.MaxAbsHz = 9990") — and 9990 is what this project permits
// ITSELF to build, not what this radio's manual says it accepts. This fake
// models the radio, so it takes the printed range; the consequence is that it
// accepts a magnitude core/cat would refuse to build, which is recorded in
// doc.go's register entry SET-DIRECTION FIELD STRICTNESS.
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

// validTagField reports whether field is acceptable as a combined record's P12
// tag field.
//
// THIS MANUAL STATES ITS OWN BYTE RULE, and it is broader than core/cat's
// default charset: "the parameter digits should be filled using any character
// except the ASCII control codes (00 to 1Fh) and the terminator (;)"
// (ft991a_layout.txt:106-109, printed folio 2). P12's own legend says only
// "TAG Characters (up to 12 characters) (ASCII)" (1017) and names no set. So
// the check here is the folio-2 rule — not a control code, not ';' — where
// core/spec's Capabilities.TagByteOK default narrows to printable ASCII
// 0x20-0x7E for the capability table (the conservative direction for what this
// project PUTS ON THE WIRE, which is a different question from what the radio
// accepts off it).
//
// SAFETY-CRITICAL, and it stays whatever hardware turns out to accept: ';' is
// the frame terminator, so a tag carrying one would make command injection
// possible. (The reassembler splits on ';' before a frame ever reaches here,
// which means that half of the check is unreachable through Port() — it is
// kept because unreachable-today is not a security property, full stop.
// Separately: WithSlot's Tag is stored verbatim with no validation of its
// own (see WithSlot's doc), so a caller can still make buildMTAnswer emit a
// ';' mid-frame — that is the caller's own crafted frame, not one this check
// is meant to guard.)
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
// prefixes (ft991a_layout.txt:965-981 and 1036-1051), and the 41-position
// combined MT Set/Answer record (998-1033) carries that same block as its
// head. So there is one block layout in this file, used by every frame that
// carries channel data.
//
// Positions, from the charts' own 1-indexed numbering as counted twice by
// evidence leg G (core/cat/ft991a/testdata/mt-vectors.golden §POSITION-BY-
// POSITION FIELD MAP, mr-vectors.golden for the 28), expressed as 0-indexed
// offsets INTO THE BLOCK (i.e. into a frame's bytes after the two-byte command
// name):
//
//	pos(1-idx)  field                  block offset
//	3-5         P1  slot               [0:3]
//	6-14        P2  frequency, 9 dig   [3:12]
//	15          P3  clarifier sign     12
//	16-19       P3  clarifier mag      [13:17]
//	20          P4  RX clarifier       17
//	21          P5  TX clarifier       18
//	22          P6  mode nibble        19
//	23          P7  kind               20
//	24          P8  tone state         21
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
	blkTXClar                      = 18
	blkMode                        = 19
	blkKind                        = 20
	blkCTCSS                       = 21
	blkP9Start, blkP9End           = 22, 24
	blkShift                       = 24
)

// blkP9Fixed is P9's documented fixed value, positions 25-26 — "P9 00:
// (Fixed)" (ft991a_layout.txt:1012, 979, 1050, 797). OI's fifth printing of
// the same legend is one character wide against the two cells its chart draws
// (1130), a printed defect core/cat/ft991a/doc.go records; the GRID is what
// this fake follows, as that package does.
const blkP9Fixed = "00"

// kindMemory is the P7 byte every populated slot in this fake answers with —
// memory channels and PMS band-edges alike.
//
// FOR A MEMORY CHANNEL IT IS PRINTED, IN BOTH DIRECTIONS, IN ONE LEGEND:
// "P7 Set: 0: (Fixed) / Read: 0: VFO 1: Memory" (ft991a_layout.txt:1009), with
// MR's "P7 0: VFO 1: Memory" (976) and OI's (1127) saying the same. That is
// the difference from internal/fakeft891, whose MT block prints no read
// vocabulary at all and whose fake has to carry the answer's domain across
// from MR as an assumption.
//
// FOR A PMS SLOT IT IS ASSUMED — doc.go's register entry PMS SLOTS ANSWER P7
// '1'. The legend has exactly two members and no manual statement says which
// of them a band-edge answers with; '1' is the only one that is not plainly
// false, since a PMS slot is not a VFO. Note what this is NOT: IF's own P7
// legend runs to seven values and has a "5: PMS" member (792-793), and it is
// NOT read across — that parameter describes what the VFO is doing, and MR's
// and OI's P7 are the narrow pair this record's answers belong to. Inventing a
// third value here would produce a frame core/cat would rightly refuse to
// parse.
const kindMemory = '1'

// parseMemoryBlock validates a memBlockLen-byte field block against every
// vocabulary the charts print (doc.go's register entry SET-DIRECTION FIELD
// STRICTNESS) and returns the slot it names together with the state it
// encodes. ok is false on the first violation found; the caller answers "?;"
// and changes nothing.
//
// wantKind is the P7 byte the CALLING COMMAND's chart documents for its Set
// direction, passed in rather than assumed, because MT-Set P7 (1009) and
// MW-Set P7 (1047) are two command-specific facts that happen to coincide on
// this radio — which is exactly what core/cat/ft991a/dialect.go's MWWriteKind
// comment says about the same pair, and no more.
//
// The returned MemState carries the ANSWER kind, kindMemory, NOT wantKind: a
// Set's P7 is a fixed placeholder carrying no channel information, so there is
// nothing in it to store (see MemState.Kind). P11 is left zero — the schema's
// fixed '0'. Tag is left to the caller: it is outside this block.
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
	// BOTH clarifier flags are live on this radio — the refusal
	// internal/fakeft891 cannot have on its second flag, because there the
	// same byte is the schema's fixed '0'.
	rx, tx := block[blkRXClar], block[blkTXClar]
	if !validBoolFlagByte(rx) || !validBoolFlagByte(tx) {
		return "", MemState{}, false
	}
	mode := block[blkMode]
	// THE BUILD-DIRECTION PREDICATE, not validModeWireByte, and the
	// distinction is the whole of the closing review's C-M2: this function
	// only ever sees an INCOMING SET (handleMT's and handleMW's Set arms
	// are its only callers), and a Set's P6 vocabulary is the printed legend
	// alone — `1`-`9`, `A`-`E` (ft991a_layout.txt:1006-1008). The '0'
	// placeholder appears in NO legend of this radio; it exists so a PARSER
	// can read an answer carrying it (the dialect's register entry "THE
	// cat.ModeUnset MEMBER OF THE MODE TABLE"), and core/cat's own builder
	// refuses to emit it in a Set (mtcombined.go's Set-frame check).
	// Accepting it here modelled a permissiveness no evidence supports, and
	// then stored a byte this fake would answer with. Pinned by
	// TestMTSet_RejectionsLeaveTheChannelUntouched's ModeUnset row.
	if !validModeBuildByte(mode) {
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
		// round-trip byte-faithfully, and on this radio it carries the TX half
		// as well.
		ClarSign: sign,
		ClarMag:  mag,
		RXClar:   rx == '1',
		TXClar:   tx == '1',
		Mode:     mode,
		// The ANSWER kind, never the Set's placeholder: memory and PMS slots
		// both answer '1'.
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
	// Position 21 is written from stored channel data, not from a schema
	// constant: P5 is the LIVE TX clarifier flag here.
	out = append(out, boolFlagByte(s.TXClar))
	out = append(out, s.Mode)
	out = append(out, s.Kind)
	out = append(out, s.CTCSS)
	out = append(out, blkP9Fixed...)
	out = append(out, s.Shift)
	return out
}

// --- MT: MEMORY CHANNEL WRITE/TAG, THE COMBINED FORM (availability 181;
// frames 998-1033) ---
//
// O O O X: a Set, a Read, an Answer, no AI push. THIS MANUAL DOES NOT
// CONTRADICT ITSELF ABOUT MT, and that is worth stating because the FT-891's
// does: there the Control Command List gives MT "Set O, Read X, Ans. X" while
// MT's own detail block prints a filled Read chart and a filled Answer chart,
// and internal/fakeft891 carries a WithMTReadUnsupported() option to play both
// radios. Here the availability row (181) and the detail block (998-1033)
// AGREE, so there is no contradiction to simulate and NO SUCH OPTION EXISTS —
// this milestone's plan decision P14 says so in terms, and
// core/cat/ft991a/doc.go's reused-command verification adds that Stage 2 "must
// not invent" an ErrMTReadRejectedForOccupiedSlot analogue either.
//
// Set frame and Answer frame are ONE 41-position chart: "MT" + the 25-byte
// shared field block + P11 + a 12-byte P12 tag field + ';'. The two directions
// are disambiguated purely by length: a Read body (after "MT", before ';') is
// exactly 3 bytes, a Set body exactly combinedBodyLen.
//
// THE READ'S SLOT DOMAIN IS MT's OWN LEGEND, the whole 001-117 span. Its Read
// chart labels positions 3-5 "P0" (1018) and the legend column heads the field
// "P0/1" (999), so the one legend covers both labels — where the FT-891's Read
// chart says "P0" against a legend column that defines no P0 at all.

// mtSetKindFixed is the combined Set's P7, printed in the same legend as the
// read direction's: "P7 Set: 0: (Fixed) / Read: 0: VFO 1: Memory"
// (ft991a_layout.txt:1009). Deliberately its own constant rather than a
// reference to any other command's: MT-Set P7 and MW-Set P7 coincide as a fact
// of this radio, not as a rule.
const mtSetKindFixed = '0'

// combinedP11Fixed is P11, position 28, documented "0: (Fixed)"
// (ft991a_layout.txt:1015).
//
// THIS IS THE INVERSION OF internal/fakeft891's MOST DISTINCTIVE BYTE. There
// P11 prints `0: TAG "OFF" 1: TAG "ON"` and the fake stores a live
// per-channel flag; here the legend fixes it, so the byte is schema, it is
// required of a Set arriving, and it is emitted in every answer unless a
// crafted MemState.P11 says otherwise. core/cat/ft991a/dialect.go carries the
// same fact as MTPolicy.P11 = cat.P11Fixed, under which core/cat's
// display-BEARING combined pair refuses outright.
const combinedP11Fixed = '0'

// tagFieldLen is P12's fixed width, "TAG Characters (up to 12 characters)
// (ASCII)" (ft991a_layout.txt:1017), drawn over positions 29-40.
const tagFieldLen = 12

// combinedBodyLen is a combined MT Set/Answer frame's body: everything after
// "MT" and before the trailing ';' — the shared field block, P11, and the tag
// field. 38, making the frame the counted 41 (evidence leg G's two independent
// counts, core/cat/ft991a/testdata/mt-vectors.golden).
const combinedBodyLen = memBlockLen + 1 + tagFieldLen

// Body offsets past the shared field block.
const (
	cmbP11                 = memBlockLen
	cmbTagStart, cmbTagEnd = memBlockLen + 1, memBlockLen + 1 + tagFieldLen
)

// tagFill is the byte a short tag is padded to width with, and the byte
// trimmed from an answer's field to recover the tag.
//
// A SPACE because the DIALECT says so — core/cat/ft991a/doc.go's ASSUMED
// register entry "MTPolicy.TagFill = ' '", whose own note records that this
// manual's P12 legend names a width and an alphabet and no fill, and that no
// FT-991A has been asked. Cited here, not re-derived: if that entry's Stage R
// capture moves the byte, this constant moves with it.
const tagFill = ' '

// buildMTAnswer builds the 41-byte combined MT answer for slot.
//
// ALWAYS THE FULL WIDTH, which is the DIALECT's ASSUMED register entry "THE
// COMBINED MT ANSWER'S EXACT LENGTH, 41" seen from the other side: that entry
// records that the manual's Answer grid draws the MAXIMAL frame and that a
// variable-width ANSWER is live, on the FT-710's precedent. This fake answers
// at the width core/cat expects (a fake that answered short would fail the
// parser rather than exercise it); if that entry's Stage R capture takes the
// contingency, this builder and core/cat move together.
//
// The tag field is written by copying the stored tag into a fixed-width,
// fill-initialised field, so a tag longer than tagFieldLen (only reachable
// through WithSlot — the wire cannot deliver one) is truncated rather than
// overflowing the frame.
func buildMTAnswer(slot string, s MemState) []byte {
	out := make([]byte, 0, 2+combinedBodyLen+1)
	out = append(out, 'M', 'T')
	out = appendMemBlock(out, slot, s)

	// The zero value means the schema's fixed '0' — see MemState.P11.
	p11 := s.P11
	if p11 == 0 {
		p11 = combinedP11Fixed
	}
	out = append(out, p11)

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
		slot := string(body)
		if !mtSlot(parseSlotForm(slot)) {
			return rejection
		}
		r.mu.Lock()
		s, ok := r.slots[slot]
		r.mu.Unlock()
		if !ok {
			// Empty slot — ASSUMED, doc.go's register entry EMPTY-SLOT
			// ANSWERS. This milestone's plan has core/driver/ft991a treat
			// this as the ONE "?;" it interprets, and the only frame that
			// driver sends which could draw one from a slot.
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
		// P11 is SCHEMA on this radio, so the fixed '0' is the only byte a Set
		// may carry here — where internal/fakeft891 accepts both values
		// because its P11 is live. TestMTSet_RejectionsLeaveTheChannelUntouched
		// holds the refusal.
		if body[cmbP11] != combinedP11Fixed {
			return rejection
		}
		field := body[cmbTagStart:cmbTagEnd]
		if !validTagField(field) {
			return rejection
		}
		// Stored TRIMMED, answered PADDED — doc.go's register entry THE TAG IS
		// STORED TRIMMED AND ANSWERED PADDED. An all-fill field is no tag, by
		// the same rule, with no branch of its own. The FT-710's HW-confirmed
		// rejection of a ZERO-BYTE tag Set has no analogue: a 41-byte frame
		// always carries the full 12-byte field, so the shape does not exist on
		// this radio to accept or refuse.
		s.Tag = strings.TrimRight(string(field), string(tagFill))

		r.mu.Lock()
		defer r.mu.Unlock()
		// The Set carries the WHOLE record, so it overwrites an existing
		// channel and CREATES an absent one, with no MW first — ASSUMED,
		// doc.go's register entry AN MT SET CREATES AN ABSENT CHANNEL. An
		// MT-only driver against a fake that demanded an MW could not write at
		// all.
		r.slots[slot] = s
		// The selection is NOT moved — doc.go's register entry A SET DOES NOT
		// MOVE THE SELECTED CHANNEL.
		return nil // fire-and-forget success
	}
	return rejection
}

// --- MR: MEMORY CHANNEL READ (availability 178; frames 965-981) ---
//
// X O O X: no Set, a Read, an Answer, no AI push. Read frame (6 bytes) "MR" +
// 3-byte slot + ';'; Answer frame (28 bytes) "MR" + the field block + ';'.
// Both counted by evidence leg G (core/cat/ft991a/testdata/mr-vectors.golden).
//
// MR IS SERVED FOR EVERY SLOT ITS LEGEND NAMES even though core/driver/ft991a
// never sends one: this radio's read path is MT-only, atomic, one frame, with
// no cross-check and no discovery walk — the whole of the FT-891's MR
// machinery collapses here because there are no discovered banks to probe. The
// command is modelled because this radio HAS it (178) and a fake that answered
// "?;" to a documented read would be a claim about the radio rather than a
// scope decision.

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
		// Includes an MR frame in the 28-byte MW SET shape: this radio's
		// availability row gives MR no Set direction at all (a manual fact,
		// not an assumption — ft991a_layout.txt:178), so such a frame is
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
		return rejection
	}
	return buildMRAnswer(slot, s)
}

// --- ID (availability 161; frames 771-779) ---
//
// X O O X: no Set — the Set chart is printed as an empty grid — Read "ID;", a
// 7-byte Answer. The VALUE is this radio's own CAT ID: the ID block prints
// "P1 0670: FT-991A" (ft991a_layout.txt:772), and it is the one byte-level
// difference a probe turns into a *driver.WrongRadioError. The FT-891's is
// 0650, one digit away, which is exactly why the probe pin matters.

func buildIDAnswer() []byte { return []byte("ID0670;") }

func (r *Radio) handleID(body []byte) []byte {
	if len(body) != 0 {
		return rejection
	}
	return buildIDAnswer()
}

// --- AI (availability 130; frames 237-246) ---
//
// O O O X. Set and Answer are 4 bytes, Read is "AI;". AI-set is
// fire-and-forget. This fake never PUSHES anything unsolicited whatever AI is
// set to: no FT-991A's AI behaviour has been observed, and the engine's
// drain-to-quiet discipline is already exercised against internal/fakeradio,
// whose own AI-flood facts are the FT-710's. THAT SUPPRESSION IS AN ASSUMPTION
// AND IS REGISTERED — doc.go's entry AUTOMATIC-INFORMATION SUPPRESSION. It is
// modelling silence as the honest default, not a claim that this radio is
// silent.
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

// --- MC: MEMORY CHANNEL (recall) (availability 174; frames 912-921) ---
//
// O O O X. Set frame (6 bytes) "MC" + 3-byte slot + ';', fire-and-forget,
// recalling the channel; Read frame "MC;" answered by "MC" + the 3-byte
// current channel + ';'. Disambiguated by length, as MT is. All three counted
// by evidence leg G (core/cat/ft991a/testdata/mc-vectors.golden).
//
// THIS RADIO'S MC LEGEND PRINTS THE WHOLE SPAN — "P1 001 - 117: Memory Channel
// Number" (ft991a_layout.txt:913) — and it is the ONLY legend in this manual
// that decomposes it into memory and the nine PMS pairs (915-916). So every
// slot this radio has may be recalled, which is what
// core/cat/ft991a/dialect.go's MCSelects = cat.MCSelectsAll carries on the send
// side. Not an assumption: this is the legend, transcribed.
//
// MC-set of a slot this fake holds no state for answers "?;", paired with the
// read rule (doc.go's register entry EMPTY-SLOT ANSWERS): a channel with no
// stored data cannot be recalled.
//
// A NOTE ON THE ANSWER DIRECTION, so that a later reader does not read a gap as
// a decision: this fake never answers a slot it was not put on by an MC-set —
// nothing here models a front panel — so an "MC;" of a radio a user had moved
// by hand is outside what this package simulates.

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

// handleFrame parses one complete, ';'-terminated frame (as produced by
// reassembler.push) and returns the reply to send: nil for a fire-and-forget
// success, or a non-nil frame — a real answer, or rejection — otherwise.
// Unknown and garbled commands fall through to rejection.
//
// COMMAND NAMES ARE MATCHED IN EITHER CASE, and that is a MANUAL FACT of this
// radio: "A command consists of 2 alphabetical characters. You may use either
// lower or upper case characters." (ft991a_layout.txt:113-114, under the
// "Alphabetical Commands" heading at 112) — the same sentence the FT-891's and
// the FTdx10's manuals state, and printed whole here rather than hyphenated
// across a column break. TestCommandNamesAreAcceptedInEitherCase pins it,
// including the mixed-case form: "either lower or upper" says nothing about
// mixing, so admitting it is a CONSEQUENCE of folding each byte independently,
// not a separate invented leniency. See doc.go's "What is NOT in this
// register, and why".
//
// FIELD VALUES REMAIN CASE-SENSITIVE (the mode nibble's hex letters): the
// manual's statement is about the two-character command NAME and says nothing
// about parameters, so extending it would be an invented leniency.
func (r *Radio) handleFrame(frame []byte) []byte {
	if len(frame) == 0 || frame[len(frame)-1] != ';' {
		return rejection // defensive: the reassembler never hands us this
	}
	body := frame[:len(frame)-1]
	if len(body) < 2 {
		return rejection
	}
	// The fold is bytes.ToUpper, which is Unicode-aware: a body whose first two
	// bytes are not ASCII comes back re-encoded (U+FFFD) and possibly longer,
	// so the first two bytes are taken. Such a frame matches no command name
	// and falls to rejection below either way.
	cmd := [2]byte(bytes.ToUpper(body[:2])[:2])
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
		return r.handleEX(rest)
	default:
		// MW falls through here: this radio documents it (Set only,
		// availability 183) and this fake does not model it, so an MW frame
		// draws "?;". See doc.go, "What this fake deliberately does NOT
		// model".
		return rejection
	}
}
