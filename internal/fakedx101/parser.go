// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx101

import (
	"fmt"
	"strings"
)

// This file is fakedx101's own, independent byte-level CAT parser and reply
// builder for the FTDX101D and the FTDX101MP. It is derived from those radios'
// own position charts in the FTDX101MP/FTDX101D CAT Operation Reference
// Manual, rev 2308-L — the availability table at layout 236-337 and the
// per-command charts cited beside each section below — and NOT from core/cat.
// See doc.go for why that independence matters and for the full ASSUMED
// register; individual assumed points are flagged inline, next to the code
// that implements them.
//
// ONE PARSER FOR TWO RADIOS. Every byte in this file is identical for the D
// and the MP except the four the ID answer carries, which is why the answer
// builder for that ONE command is a method on Radio (see buildIDAnswer) while
// every other builder here is a plain function. The manual prints the whole
// memory-channel surface once, unconditionally, for both models — the MT
// block and its legends at layout 1311-1330, MR at 1276-1294, MW at 1352-1367,
// MC at 1224-1233 — and carries no model qualifier anywhere in them
// (docs/superpowers/m9d2-capability-matrix.md §2.5 states the sweep that
// establishes this, §4 the whole of the model-distinguishing surface).
//
// The manual and its layout-preserved extraction are gitignored
// (docs/fixtures-private/manuals/), so the "layout N" references here are
// citations in the sense core/cat/ftdx101/doc.go uses them: they name where
// the chart is, they are not links.

// --- General framing ---

// rejection is the protocol's one and only NAK, "?;" — an unattributed
// generic command failure. Every refusal in this file answers with it and
// nothing else: an empty slot, an out-of-inventory slot, a malformed frame, an
// unknown command and an overflowed accumulator are indistinguishable to the
// host, which is the whole of the convention.
//
// THE CONVENTION ITSELF IS INHERITED AND IS NOT IN THIS MANUAL — doc.go
// register entry 16. It is core/cat's ErrRejected, adopted from the FT-710's
// reference; this manual never prints the character at all.
var rejection = []byte("?;")

// maxAccumulatorBytes is the reassembler's byte cap — this package's own
// bounded-input policy, not a manual figure (doc.go register entry 15).
const maxAccumulatorBytes = 256

// reassembler turns an arbitrary stream of Write() chunks into complete
// ';'-terminated frames. Framing only: it says nothing about what any frame
// means. The terminator is the manual's own ("To signal the end of a command,
// it is necessary to use a semicolon (;)", layout 227-229).
//
// Overflow behaviour (doc.go register entry 15): once more than
// maxAccumulatorBytes bytes have accumulated without completing a frame, push
// reports one overflow event — the caller replies "?;" for it — and discards
// every byte from that point up to and including the next ';', then resumes
// normal framing. The zero value is not usable; construct with newReassembler.
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

// --- Slot grammar ---
//
// The FTdx101's slot legends, printed beside MC (layout 1225-1227), MR
// (1278-1279), MT (1312-1313), IF (1082-1083) and OI (1436-1437) alike:
// "001-099 (Memory Channel), P1L -P9U (PMS), 5xx (5MHz BAND), EMG (EMERGENCY
// CH)". MW's legend (1353) is the restricted pair "001-099 (Memory Channel),
// P1L -P9U (PMS)" — no 5xx, no EMG.
//
// The "5xx" legend is taken here at its literal width: any 5 followed by two
// digits is a well-formed 5 MHz slot code. The dialect's 501..599 NUMBERING is
// its own ASSUMED register entry ("SlotSpace.SixtyLo/SixtyHi = 501/599",
// core/cat/ftdx101/doc.go, cited by name and not re-derived), and this
// grammar's slight extra breadth is invisible in behaviour — a 5xx code no
// image populates answers "?;" exactly as an out-of-inventory code does.

// slotNoneWire is the answer-only "no slot" form. The wire spelling is the
// DIALECT's ASSUMED NoneWire (core/cat/ftdx101/doc.go's register entry
// "SlotSpace.NoneWire = \"000\""): it appears in no FTdx101 slot legend. It is
// never a valid REQUEST slot here — a read or recall naming it is malformed,
// as fakeradio also holds — and it appears only in the MC answer of a radio
// sitting on a VFO.
const slotNoneWire = "000"

// slotEMGWire is the emergency channel's wire form, from the slot legends
// above ("EMG (EMERGENCY CH)").
const slotEMGWire = "EMG"

// slotWireLen is the width of every slot code on the wire: three bytes for
// every form in every legend.
const slotWireLen = 3

// slotKind classifies a 3-byte slot code.
type slotKind int

const (
	slotInvalid slotKind = iota // malformed: none of the forms below
	slotNone                    // "000" — answer-only, never a valid request
	slotMemory                  // 001-099
	slotPMS                     // P1L-P9U
	slotFiveMHz                 // 5xx (which values are POPULATED is a state question — image.go)
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
		case s[0] == '5':
			return slotFiveMHz
		default:
			return slotInvalid // "100", "200".."499", "600".."999"
		}
	}
	if s[0] == 'P' && s[1] >= '1' && s[1] <= '9' && (s[2] == 'L' || s[2] == 'U') {
		return slotPMS
	}
	return slotInvalid
}

// readableSlot reports whether kind is a slot an MT-read, an MR-read or an
// MC-set may name: the four banks the MC, MR and MT legends list. "000" is
// answer-only.
func readableSlot(kind slotKind) bool {
	switch kind {
	case slotMemory, slotPMS, slotFiveMHz, slotEMG:
		return true
	}
	return false
}

// mtSettableSlot reports whether kind is a slot a combined MT Set may name.
//
// It is readableSlot — 5xx and EMG INCLUDED — and that is WIDER than core/cat's
// own MT write policy, which refuses both (mtcombined.go). doc.go register
// entry 8 records why: that refusal is a PROJECT DECISION taken by the layer
// that talks to real radios, this radio's own MT legend carries the full
// vocabulary (layout 1312-1313), and this fake models what the radio accepts
// rather than what this project permits itself to send. Kept as its own
// function, rather than calling readableSlot at the site, so that narrowing it
// when hardware speaks is a one-line change at the decision.
func mtSettableSlot(kind slotKind) bool { return readableSlot(kind) }

// mwSettableSlot reports whether kind is a slot an MW Set may name: memory
// channels and PMS pairs only, per MW's own restricted P1 legend (layout 1353)
// — a MANUAL FACT of this radio, not a policy, and unambiguously the Set
// direction's parameter (MW has no other direction: availability O X X X,
// layout 336).
func mwSettableSlot(kind slotKind) bool {
	return kind == slotMemory || kind == slotPMS
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

// toUpperASCII folds one ASCII lower-case letter to upper case and leaves
// every other byte alone. Used on COMMAND NAMES ONLY — see handleFrame.
func toUpperASCII(b byte) byte {
	if b >= 'a' && b <= 'z' {
		return b - 'a' + 'A'
	}
	return b
}

// --- Field validators (wire level) ---
//
// Every one of them is enforced on the SET direction, and every one is ASSUMED
// to be what the radio itself enforces — doc.go register entry 7.

// validModeWireByte reports whether b is a mode nibble this radio's legend
// admits. The legend is printed beside five commands — MD's P2 (layout
// 1240-1243), IF's P6 (1089-1091), MR's P6 (1286-1288), MT's P6 (1321-1323)
// and MW's P6 (1361-1363) — all five identical, all five running 1 to F with
// no other member. (A sixth, OI's P6 at 1443-1446, MISNUMBERS its last two
// members and is recorded as a printing defect in core/cat/ftdx101/doc.go's
// chart-defect record; it is not sourced from here either.) '0' is accepted
// additionally as the "-" placeholder, which appears in NO FTdx101 legend and
// is the DIALECT's ASSUMED register entry ("the cat.ModeUnset member of the
// mode table", cited): parsers must accept a placeholder even where builders
// must never emit one.
func validModeWireByte(b byte) bool {
	switch {
	case b == '0':
		return true
	case b >= '1' && b <= '9':
		return true
	case b >= 'A' && b <= 'F':
		return true
	}
	return false
}

func validCTCSSByte(b byte) bool    { return b >= '0' && b <= '2' }
func validShiftByte(b byte) bool    { return b >= '0' && b <= '2' }
func validBoolFlagByte(b byte) bool { return b == '0' || b == '1' }

// validClarSign reports whether b is a P3 direction byte.
//
// '+' is this manual's own glyph. '-' IS NOT: the manual prints the minus
// direction as a TWO-HYPHEN glyph — "+: Plus Shift, --: Minus Shift" —
// identically in all five frame pages that carry it (layout 1085 IF, 1281 MR,
// 1316 MT, 1355 MW, 1439 OI), and the ASCII HYPHEN-MINUS 0x2D accepted here is
// INHERITED from the FT-710/FTdx10 convention. That is the DIALECT's own
// seventh ASSUMED register entry ("The CLARIFIER'S MINUS-DIRECTION BYTE, the
// ASCII HYPHEN-MINUS 0x2D ('-')", core/cat/ftdx101/doc.go), cited here by name
// and NOT re-registered: if a Stage W capture moves that byte, this validator
// and core/cat move together.
func validClarSign(b byte) bool { return b == '+' || b == '-' }

// validClarMagDigits reports whether s is a 4-digit clarifier magnitude field
// that is in range (0000-9990 Hz) and on a 10 Hz step.
//
// The RANGE is this manual's, agreed by the IF, MR, MT and MW field legends
// (layout 1085-1086, 1281-1282, 1316-1317, 1355-1356) and the RD/RU command
// pages (1602, 1700). The STEP is the DIALECT's ASSUMED register entry
// ("ClarifierPolicy.StepHz = 10") — no step is stated anywhere in this manual
// — and is cited here rather than re-registered. Enforcing either at the wire
// is register entry 7's assumption.
func validClarMagDigits(s string) bool {
	if len(s) != 4 {
		return false
	}
	n := 0
	for i := 0; i < 4; i++ {
		if !isDigit(s[i]) {
			return false
		}
		n = n*10 + int(s[i]-'0')
	}
	return n <= 9990 && n%10 == 0
}

// validTagField reports whether field is acceptable as a combined record's P12
// tag field. The manual's P12 legend says only "TAG Characters (up to 12
// characters) (ASCII)" (layout 1330), so the exact accepted charset is
// unknown; this check is printable ASCII 0x20-0x7E EXCLUDING ';'.
//
// It is NARROWER than the one charset rule this manual does state — that a
// parameter's unused digits be "filled using any character except the ASCII
// control codes (00 to 1Fh) and the terminator (;)" (layout 224-225), which
// permits 0x7F and every byte above it — and the narrowing is doc.go register
// entry 13's assumption. The ';' half of it is the one part this manual agrees
// with outright.
//
// SAFETY-CRITICAL, and it stays whatever hardware turns out to accept: ';' is
// the frame terminator, so a tag carrying one would make command injection
// possible. (The reassembler splits on ';' before a frame ever reaches here,
// which means the check is unreachable through Port() — it is kept because
// unreachable-today is not a security property, and buildMTAnswer's field is
// written from stored state that WithSlot can set directly.)
func validTagField(field []byte) bool {
	for _, b := range field {
		if b < 0x20 || b > 0x7E || b == ';' {
			return false
		}
	}
	return true
}

// --- Field encoders (semantic builders) ---
//
// Used by image.go to construct readable fixture data. NOT used on the
// reply-building path, which concatenates already-validated MemState fields.

// encodeFreqDigits converts hz to the 9-digit ASCII P2 field, refusing values
// that would need more than 9 digits.
func encodeFreqDigits(hz uint64) (string, error) {
	if hz > 999_999_999 {
		return "", fmt.Errorf("fakedx101: frequency %d Hz needs more than 9 digits", hz)
	}
	return fmt.Sprintf("%09d", hz), nil
}

// validModeBuildByte reports whether m is a mode nibble a BUILDER may emit:
// validModeWireByte without the '0' placeholder, which parsers accept and
// builders must not produce (the dialect's ModeUnset register entry, cited).
func validModeBuildByte(m byte) bool { return validModeWireByte(m) && m != '0' }

// --- The shared memory field block ---
//
// The FTdx101's MR Answer and MW Set are one 28-byte chart under two prefixes
// (layout 1277-1294 and 1352-1367), and the 41-byte combined MT Set/Answer
// record (1311-1330) carries that same block as its head. So there is one
// block layout in this file, used three times.
//
// Positions, from the charts' own 1-indexed numbering, expressed as 0-indexed
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
//	24          P8  CTCSS state        21
//	25-26       P9  fixed "00"         [22:24]
//	27          P10 shift              24
//
// The MR/MW frame's ';' sits at position 28, immediately after the block; the
// combined record puts P11 there instead and continues with the tag field.
// core/cat/ftdx101/testdata/geometry-witness.csv counted the same positions off
// 300 dpi raster renders of the charts, independently of this file.

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

// blkP9Fixed is P9's documented fixed value, positions 25-26 (layout 1326).
const blkP9Fixed = "00"

// parseMemoryBlock validates a memBlockLen-byte field block against every
// vocabulary the charts print (doc.go register entry 7) and returns the slot it
// names together with the state it encodes. ok is false on the first violation
// found; the caller answers "?;" and changes nothing.
//
// wantKind is the P7 byte the CALLING COMMAND's chart documents for its Set
// direction, passed in rather than assumed, because MT-Set P7 (layout 1324) and
// MW-Set P7 (1364) are two command-specific facts that happen to coincide on
// this radio (both "0: (Fixed)") and deriving either from the other is the
// conflation core/cat's CombinedMTSetKind doc comment records having paid for
// once.
//
// The returned MemState carries the ANSWER kind, kindMemory, NOT wantKind: a
// Set's P7 is a fixed placeholder carrying no channel information, so there is
// nothing in it to store (doc.go register entries 2 and 3, and MemState.Kind's
// own comment). Tag and P11 are left to the caller: MW has neither field, and
// MT's are outside this block.
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
	rx, tx := block[blkRXClar], block[blkTXClar]
	if !validBoolFlagByte(rx) || !validBoolFlagByte(tx) {
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
		// STORED, not zeroed — doc.go register entry 5, the deliberate
		// non-borrowing of the FT-710's clarifier hardware finding. This is
		// the line that makes the combined Set round-trip byte-faithfully,
		// and so the line that makes the driver's Simulated profile honest
		// in writing the clarifier Supported rather than Inert (matrix
		// §2.1's profile table).
		ClarSign: sign,
		ClarMag:  mag,
		RXClar:   rx == '1',
		TXClar:   tx == '1',
		Mode:     mode,
		// The ANSWER kind, never the Set's placeholder — register entries 2
		// and 3: memory, PMS, 5xx and EMG slots all answer '1'.
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
	out = append(out, boolFlagByte(s.TXClar))
	out = append(out, s.Mode)
	out = append(out, s.Kind)
	out = append(out, s.CTCSS)
	out = append(out, blkP9Fixed...)
	out = append(out, s.Shift)
	return out
}

// --- MR: MEMORY CHANNEL READ (layout 1277-1294) ---
//
// Availability X O O X (layout 331): no Set, a Read, an Answer, no AI push.
// Read frame (6 bytes) "MR" + 3-byte slot + ';'; Answer frame (28 bytes) "MR" +
// the field block + ';'.
//
// MR is answered here even though core/driver/ftdx101 NEVER SENDS IT — its read
// path is MT-only (matrix §3.5), and the combined answer is an atomic snapshot
// the two-frame MR+MT stitch cannot be. The command is on the radio, so it is
// on the fake: that is what keeps the dialect's own MR coverage (its golden
// vectors and dialecttest) meaningful, and what would let a later driver read
// MR without first teaching this package a command.

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
		// availability table gives MR no Set direction at all (a manual
		// fact, not an assumption), so such a frame is simply unknown.
		return rejection
	}
	slot := string(body)
	if !readableSlot(parseSlotForm(slot)) {
		return rejection
	}
	r.mu.Lock()
	s, ok := r.slots[slot]
	r.mu.Unlock()
	if !ok {
		return rejection // empty slot — ASSUMED, doc.go register entry 1
	}
	return buildMRAnswer(slot, s)
}

// --- MW: MEMORY CHANNEL WRITE (layout 1352-1367) ---
//
// Availability O X X X (layout 336): a Set and nothing else. The Set frame is
// MR's 28-byte chart under an "MW" prefix, its P1 legend restricted to 001-099
// and P1L-P9U, and its P7 documented "0: (Fixed)".
//
// Accepted here although core/driver/ftdx101 never sends one — doc.go register
// entry 9 for why, and for what the modelling assumes.

// mwSetKindFixed is MW's P7 for THIS radio, "0: (Fixed)" (layout 1364).
// Deliberately its own constant rather than a reference to mtSetKindFixed: the
// two coincide as a fact of this radio (core/cat/ftdx101's dialect.go says
// exactly that about the same pair, in its MWWriteKind comment), not as a rule,
// and a radio that ever separated them would take one constant with it.
const mwSetKindFixed = '0'

func (r *Radio) handleMW(body []byte) []byte {
	slot, s, ok := parseMemoryBlock(body, mwSetKindFixed)
	if !ok {
		return rejection
	}
	if !mwSettableSlot(parseSlotForm(slot)) {
		return rejection
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	// MW has no tag field, so the stored tag survives the write untouched
	// (ASSUMED — register entry 9; "untouched" rather than "cleared" is a
	// choice, and nothing has watched either radio make it). The zero value
	// for an absent slot is exactly right: an empty tag, and the write CREATES
	// the channel (also entry 9).
	s.Tag = r.slots[slot].Tag
	r.slots[slot] = s
	// NOTE what is NOT here: r.currentChannel is not moved. fakeradio's MW
	// moves the FT-710's selection, on that radio's own M5b hardware finding;
	// borrowing it would invent a side effect for two radios nobody has
	// written to. doc.go register entry 10.
	return nil // fire-and-forget success — register entry 11
}

// --- MT: MEMORY CHANNEL WRITE/TAG, THE COMBINED FORM (layout 1311-1330) ---
//
// Availability O O O X (layout 333-335): a Set, a Read, an Answer. Read frame
// (6 bytes) "MT" + 3-byte slot + ';'. Set AND Answer are ONE 41-byte chart:
// "MT" + the 25-position shared field block (chart positions 3-27) + P11 + a
// 12-byte P12 tag field + ';', with P7 reading "Set: 0: (Fixed) / Read: 0: VFO
// 1: Memory". (25 is the BLOCK; 28 is the MR/MW FRAME length, which is the same
// block under a two-byte prefix with its own terminator.)
//
// The two directions are disambiguated purely by length, as fakeradio's short
// MT form also is: a Read body (after "MT", before ';') is exactly 3 bytes, a
// Set body exactly combinedBodyLen. There is no variable-width form and no
// display flag anywhere (the P11 position is the manual's fixed "0", which is
// matrix §3.7's manual-evidenced TagDisplay absence).

// mtSetKindFixed is the combined Set's P7, "Set: 0: (Fixed)" (layout 1324).
// See mwSetKindFixed for why the two are separate constants.
const mtSetKindFixed = '0'

// combinedP11Fixed is P11, documented "0: (Fixed)" (layout 1329) with no
// direction qualifier, so it is required of a Set arriving and emitted in every
// answer.
const combinedP11Fixed = '0'

// tagFieldLen is P12's fixed width, "up to 12 characters" (layout 1330), which
// the chart draws as positions 29-40.
const tagFieldLen = 12

// combinedBodyLen is a combined MT Set/Answer frame's body: everything after
// "MT" and before the trailing ';' — the shared field block, P11, and the tag
// field. 38, making the frame 41.
const combinedBodyLen = memBlockLen + 1 + tagFieldLen

// Body offsets past the shared field block.
const (
	cmbP11                 = memBlockLen
	cmbTagStart, cmbTagEnd = memBlockLen + 1, memBlockLen + 1 + tagFieldLen
)

// tagFill is the byte a short tag is padded to width with, and the byte trimmed
// from an answer's field to recover the tag.
//
// A SPACE because the DIALECT says so — core/cat/ftdx101's ASSUMED register
// entry "MTPolicy.TagFill = ' '", whose own note records that the manual's P12
// legend names no fill and that neither FTdx101's padding has ever been
// observed. Cited here, not re-derived: if that entry's Stage R capture moves
// the byte, this constant moves with it.
const tagFill = ' '

// buildMTAnswer builds the 41-byte combined MT answer for slot.
//
// ALWAYS THE FULL WIDTH, which is the DIALECT's ASSUMED register entry "The
// combined MT answer's EXACT length" seen from the other side: that entry
// records that the manual's grid draws the MAXIMAL frame and that a
// variable-width ANSWER is live, with a recorded contingency of a 30..41
// window. This fake answers at the width core/cat expects (a fake that answered
// short would fail the parser rather than exercise it); if that entry's Stage R
// capture takes the contingency, this builder and core/cat move together.
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
		if !readableSlot(parseSlotForm(slot)) {
			return rejection
		}
		r.mu.Lock()
		s, ok := r.slots[slot]
		r.mu.Unlock()
		if !ok {
			// Empty slot — ASSUMED, doc.go register entry 1. This is the
			// exchange core/driver/ftdx101's own register entry 8 reads as
			// "empty" and its entry 7 reads as "absent from this radio"
			// during 5xx/EMG discovery.
			return rejection
		}
		return buildMTAnswer(slot, s)

	case combinedBodyLen:
		slot, s, ok := parseMemoryBlock(body[:memBlockLen], mtSetKindFixed)
		if !ok {
			return rejection
		}
		if !mtSettableSlot(parseSlotForm(slot)) {
			return rejection
		}
		if body[cmbP11] != combinedP11Fixed {
			return rejection
		}
		field := body[cmbTagStart:cmbTagEnd]
		if !validTagField(field) {
			return rejection
		}
		// Stored TRIMMED, answered PADDED — doc.go register entry 13. An
		// all-fill field is no tag, by the same rule, with no branch of its
		// own. The FT-710's HW-confirmed rejection of a ZERO-BYTE tag Set has
		// no analogue: a 41-byte frame always carries the full 12-byte field,
		// so the shape does not exist on this radio to accept or refuse.
		s.Tag = strings.TrimRight(string(field), string(tagFill))

		r.mu.Lock()
		defer r.mu.Unlock()
		// The Set carries the WHOLE record, so it overwrites an existing
		// channel and CREATES an absent one, with no MW first — ASSUMED,
		// doc.go register entry 6, and the fake's half of the driver's own
		// entry 9. An MT-only driver against a fake that demanded an MW could
		// not write at all.
		r.slots[slot] = s
		// The selection is NOT moved — register entry 10, as for MW.
		return nil // fire-and-forget success — register entry 11
	}
	return rejection
}

// --- MC: MEMORY CHANNEL (recall) (layout 1224-1233) ---
//
// Availability O O O X (layout 327). Set frame (6 bytes) "MC" + 3-byte slot +
// ';', fire-and-forget, recalling the channel; Read frame "MC;" answered by
// "MC" + the 3-byte current channel + ';'. Disambiguated by length, as MT is.
//
// MC-set of a slot this fake holds no state for answers "?;" — paired with the
// read rule (doc.go register entry 1): a channel with no stored data cannot be
// recalled.

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
		if !readableSlot(parseSlotForm(slot)) {
			return rejection
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if _, ok := r.slots[slot]; !ok {
			return rejection // empty slot — register entry 1
		}
		r.currentChannel = slot
		return nil // fire-and-forget success — register entry 11
	}
	return rejection
}

// --- ID (layout 1069-1078) ---
//
// Availability X O O X (layout 304): no Set, Read "ID;", a 7-byte Answer. The
// VALUE is the one place in this whole package where the two models differ:
// P1's legend prints "0681: FTDX101D" (layout 1070) and "0682: FTDX101MP"
// (1072).
//
// THIS IS THE ONE DELIBERATE DIVERGENCE FROM internal/fakedx10's parser, whose
// equivalent is a zero-argument function returning a hardcoded "ID0761;"
// literal (internal/fakedx10/parser.go:698). That package fakes ONE radio and a
// literal is honest there. This one fakes TWO, so the answer is built from the
// Radio's own configured CAT ID and the builder is a METHOD — doc.go's
// divergence note states the reasoning in full, and TestID_AnswersTheModelsOwnCATID
// plus TestTheTwoModelsDifferOnlyInTheIDAnswer are what hold it to exactly one
// difference.
//
// r.catID is written once, by newRadio, before serve() starts, and never
// mutated afterwards — so this method may read it without r.mu, for the same
// reason rawWrite may read r.latency without it.

func (r *Radio) buildIDAnswer() []byte {
	out := make([]byte, 0, 2+catIDLen+1)
	out = append(out, 'I', 'D')
	out = append(out, r.catID...)
	out = append(out, ';')
	return out
}

func (r *Radio) handleID(body []byte) []byte {
	if len(body) != 0 {
		return rejection
	}
	return r.buildIDAnswer()
}

// --- AI (layout 376-385) ---
//
// Availability O O O X (layout 244). Set and Answer are 4 bytes, Read is "AI;".
// AI-set is fire-and-forget. This fake never PUSHES anything unsolicited
// whatever AI is set to: no FTdx101's AI behaviour has been observed, and the
// engine's drain-to-quiet discipline is already exercised against fakeradio,
// whose own AI-flood facts are the FT-710's.
//
// core/transport.Engine.Init opens every session with an AI-off Set, so this
// handler's silent-accept path is on the critical path of every fake session.
//
// Two notes this manual prints beside the chart are worth having here, because
// neither is inherited: "The AI command is available only when PC is connected
// with USB cable" (layout 381 — matrix §3.12, the reason every hardware capture
// must record its port), and "This parameter is set to '0' (OFF) automatically
// when the transceiver is turned 'OFF'" (layout 384), which is what makes this
// fake's AI-off-at-construction a MANUAL FACT for these radios and so keeps it
// out of the ASSUMED register (doc.go's "What is NOT in this register").

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
		return nil // fire-and-forget success — register entry 11
	}
	return rejection
}

// --- Top-level dispatch ---

// handleFrame parses one complete, ';'-terminated frame (as produced by
// reassembler.push) and returns the reply to send: nil for a fire-and-forget
// success, or a non-nil frame — a real answer, or rejection — otherwise.
// Unknown and garbled commands fall through to rejection.
//
// COMMAND NAMES ARE MATCHED IN EITHER CASE, and that is a MANUAL FACT of these
// radios rather than an inherited leniency: "A command consists of 2
// alphabetical characters. You may use either lower or upper case characters."
// (layout 204-205). internal/fakedx10 refuses lower case; why it does is a
// question about that package, recorded for milestone review, and it changes
// nothing about the line quoted here.
//
// FIELD VALUES REMAIN CASE-SENSITIVE (the mode nibble's hex letters, the PMS
// L/U suffix, EMG): the manual's statement is about the two-character command
// NAME and says nothing about parameters, so extending it would be an invented
// leniency. That half is ASSUMED — doc.go register entry 12.
//
// EX (MENU) is dispatched to handleEX (ex.go), which answers from this package's
// generated projection of transcription B. It is READ ONLY: a set-shaped body
// falls through the same length check as any other malformed one, to "?;" —
// doc.go register entry 17.
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
	case [2]byte{'M', 'W'}:
		return r.handleMW(rest)
	case [2]byte{'M', 'T'}:
		return r.handleMT(rest)
	case [2]byte{'M', 'C'}:
		return r.handleMC(rest)
	case [2]byte{'E', 'X'}:
		return r.handleEX(rest)
	default:
		return rejection
	}
}
