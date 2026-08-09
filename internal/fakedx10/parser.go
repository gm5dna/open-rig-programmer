// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx10

import (
	"fmt"
	"strings"
)

// This file is fakedx10's own, independent byte-level CAT parser and reply
// builder for the FTdx10. It is derived from that radio's own position charts
// in the FTdx10 CAT Operation Reference Manual, rev 2308-F — the availability
// table at manual lines 192-271 and the per-command charts cited beside each
// section below — and NOT from core/cat. See doc.go for why that independence
// matters and for the full ASSUMED register; individual assumed points are
// flagged inline, next to the code that implements them.
//
// The manual itself is gitignored (docs/fixtures-private/manuals/), so the
// line references here are citations in the sense core/cat/ftdx10/doc.go uses
// them: they name where the chart is, they are not links.

// --- General framing ---

// rejection is the protocol's one and only NAK, "?;" — an unattributed
// generic command failure. Every refusal in this file answers with it and
// nothing else: an empty slot, an out-of-inventory slot, a malformed frame,
// an unknown command and an overflowed accumulator are indistinguishable to
// the host, which is the whole of the convention.
//
// THE CONVENTION ITSELF IS INHERITED AND IS NOT IN THIS MANUAL — doc.go
// register entry 18. It is core/cat's ErrRejected, adopted from the FT-710's
// reference; this manual never prints the character at all.
var rejection = []byte("?;")

// maxAccumulatorBytes is the reassembler's byte cap — this package's own
// bounded-input policy, not a manual figure (doc.go register entry 16).
const maxAccumulatorBytes = 256

// reassembler turns an arbitrary stream of Write() chunks into complete
// ';'-terminated frames. Framing only: it says nothing about what any frame
// means.
//
// Overflow behaviour (doc.go register entry 16): once more than
// maxAccumulatorBytes bytes have accumulated without completing a frame, push
// reports one overflow event — the caller replies "?;" for it — and discards
// every byte from that point up to and including the next ';', then resumes
// normal framing. The zero value is not usable; construct with
// newReassembler.
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
// The FTdx10's slot legends, printed beside MC (manual lines 1131-1133) and
// MR (1184-1185) alike: "001-099 (Memory Channel), P1L-P9U (PMS), 5xx (5MHz
// BAND), EMG (EMERGENCY CH)". MW's legend (1259) is the restricted pair
// "001-099, P1L-P9U" — no 5xx, no EMG.
//
// The "5xx" legend is taken here at its literal width: any 5 followed by two
// digits is a well-formed 5 MHz slot code. The dialect's 501..599 NUMBERING
// is its own ASSUMED register entry 5 (cited, not re-derived), and this
// grammar's slight extra breadth is invisible in behaviour — a 5xx code no
// image populates answers "?;" exactly as an out-of-inventory code does.

// slotNoneWire is the answer-only "no slot" form. The wire spelling is the
// DIALECT's ASSUMED NoneWire (core/cat/ftdx10's register entry 3): it appears
// in no FTdx10 slot legend. It is never a valid REQUEST slot here — a read or
// recall naming it is malformed, as fakeradio also holds (its register item
// 15) — and it appears only in the MC answer of a radio sitting on a VFO.
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
	if len(s) != 3 {
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

// readableSlot reports whether kind is a slot MT-read, MR-read or MC-set may
// name: the four banks the MC and MR legends list. "000" is answer-only.
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
// entry 8 records why: that refusal is a project decision taken by the layer
// that talks to real radios, and this fake models what the radio accepts
// rather than what this project permits itself to send. Kept as its own
// function, rather than calling readableSlot at the site, so that narrowing it
// when hardware speaks is a one-line change at the decision.
func mtSettableSlot(kind slotKind) bool { return readableSlot(kind) }

// mwSettableSlot reports whether kind is a slot an MW Set may name: memory
// channels and PMS pairs only, per MW's own restricted P1 legend (manual line
// 1259) — a MANUAL FACT of this radio, not a policy.
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
// Every one of them is enforced on the SET direction, and every one is
// ASSUMED to be what the radio itself enforces — doc.go register entry 7.

// validModeWireByte reports whether b is a mode nibble this radio's legend
// admits. The legend is printed beside four commands — MD (manual lines
// 1146-1149), MR's P6 (1192-1194), MT's P6 (1227-1229), MW's P6 (1267-1269) —
// all four identical, all four running 1 to F with no other member. '0' is
// accepted additionally as the "-" placeholder, which appears in NO FTdx10
// legend and is the DIALECT's ASSUMED register entry 4 (cited): parsers must
// accept a placeholder even where builders must never emit one.
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
func validClarSign(b byte) bool     { return b == '+' || b == '-' }

// validClarMagDigits reports whether s is a 4-digit clarifier magnitude field
// that is in range (0000-9990 Hz) and on a 10 Hz step.
//
// The RANGE is this manual's, agreed by the MR/MT/MW field legends and the
// RD/RU command pages (manual lines 1507, 1605). The STEP is the DIALECT's
// ASSUMED register entry 2 — no step is stated anywhere in this manual — and
// is cited here rather than re-registered. Enforcing either at the wire is
// register entry 7's assumption.
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

// validTagField reports whether field is acceptable as a combined record's
// P12 tag field. The manual's P12 legend says only "TAG Characters (up to 12
// characters) (ASCII)" (manual line 1236), so the exact accepted charset is
// unknown; this check is printable ASCII 0x20-0x7E EXCLUDING ';'.
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
		return "", fmt.Errorf("fakedx10: frequency %d Hz needs more than 9 digits", hz)
	}
	return fmt.Sprintf("%09d", hz), nil
}

// validModeBuildByte reports whether m is a mode nibble a BUILDER may emit:
// validModeWireByte without the '0' placeholder, which parsers accept and
// builders must not produce (the dialect's register entry 4, cited).
func validModeBuildByte(m byte) bool { return validModeWireByte(m) && m != '0' }

// --- The shared memory field block ---
//
// The FTdx10's MR Answer and MW Set are one 28-byte chart under two prefixes
// (manual lines 1183-1202 and 1258-1272), and the 41-byte combined MT
// Set/Answer record (1217-1256) carries that same block as its head. So there
// is one block layout in this file, used three times.
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

// blkP9Fixed is P9's documented fixed value, positions 25-26.
const blkP9Fixed = "00"

// parseMemoryBlock validates a memBlockLen-byte field block against every
// vocabulary the charts print (doc.go register entry 7) and returns the slot
// it names together with the state it encodes. ok is false on the first
// violation found; the caller answers "?;" and changes nothing.
//
// wantKind is the P7 byte the CALLING COMMAND's chart documents for its Set
// direction, passed in rather than assumed, because MT-Set P7 and MW-Set P7
// are two command-specific facts that happen to coincide on this radio (both
// "0: (Fixed)") and deriving either from the other is the conflation
// core/cat's CombinedMTSetKind doc comment records having paid for once.
//
// The returned MemState carries the ANSWER kind, kindMemory, NOT wantKind:
// a Set's P7 is a fixed placeholder carrying no channel information, so there
// is nothing in it to store (doc.go register entries 2 and 3, and
// MemState.Kind's own comment). Tag and P11 are left to the caller: MW has
// neither field, and MT's are outside this block.
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
		// the line that makes the combined Set round-trip byte-faithfully.
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

// --- MR: MEMORY CHANNEL READ (manual lines 1183-1202) ---
//
// Availability X O O X: no Set, a Read, an Answer, no AI push. Read frame (6
// bytes) "MR" + 3-byte slot + ';'; Answer frame (28 bytes) "MR" + the field
// block + ';'.
//
// MR is answered here even though core/driver/ftdx10 NEVER SENDS IT — its read
// path is MT-only, and the combined answer is an atomic snapshot the two-frame
// MR+MT stitch cannot be. The command is on the radio, so it is on the fake:
// that is what keeps the dialect's own MR coverage (its golden vectors and
// dialecttest) meaningful, and what would let a later driver read MR without
// first teaching this package a command.

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

// --- MW: MEMORY CHANNEL WRITE (manual lines 1258-1272) ---
//
// Availability O X X X: a Set and nothing else. The Set frame is MR's 28-byte
// chart under an "MW" prefix, its P1 legend restricted to 001-099 and P1L-P9U,
// and its P7 documented "0: (Fixed)".
//
// Accepted here although core/driver/ftdx10 never sends one — doc.go register
// entry 9 for why, and for what the modelling assumes.

// mwSetKindFixed is MW's P7 for THIS radio, "0: (Fixed)" (manual line ~1268).
// Deliberately its own constant rather than a reference to mtSetKindFixed:
// the two coincide as a fact of this radio (core/cat/ftdx10's dialect.go says
// exactly that about the same pair), not as a rule, and a radio that ever
// separated them would take one constant with it.
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
	// choice, and nothing has watched a radio make it). The zero value for an
	// absent slot is exactly right: an empty tag, and the write CREATES the
	// channel (also entry 9).
	s.Tag = r.slots[slot].Tag
	r.slots[slot] = s
	// NOTE what is NOT here: r.currentChannel is not moved. fakeradio's MW
	// moves the FT-710's selection, on that radio's own M5b hardware finding;
	// borrowing it would invent a side effect for a radio nobody has written
	// to. doc.go register entry 10.
	return nil // fire-and-forget success — register entry 11
}

// --- MT: MEMORY CHANNEL TAG, THE COMBINED FORM (manual lines 1217-1256) ---
//
// Availability O O O X: a Set, a Read, an Answer. Read frame (6 bytes) "MT" +
// 3-byte slot + ';'. Set AND Answer are ONE 41-byte chart: "MT" + the 28-position
// shared field block + P11 + a 12-byte P12 tag field + ';', with P7 reading
// "Set: 0: (Fixed) / Read: 0: VFO 1: Memory".
//
// The two directions are disambiguated purely by length, as fakeradio's short
// MT form also is: a Read body (after "MT", before ';') is exactly 3 bytes, a
// Set body exactly combinedBodyLen. There is no variable-width form and no
// display flag anywhere.

// mtSetKindFixed is the combined Set's P7, "0: (Fixed)" (manual line ~1233).
// See mwSetKindFixed for why the two are separate constants.
const mtSetKindFixed = '0'

// combinedP11Fixed is P11, documented fixed "0" in BOTH directions (manual
// line ~1234) — required of a Set arriving and emitted in every answer.
const combinedP11Fixed = '0'

// tagFieldLen is P12's fixed width, "up to 12 characters" (manual line 1236).
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

// tagFill is the byte a short tag is padded to width with, and the byte
// trimmed from an answer's field to recover the tag.
//
// A SPACE because the DIALECT says so — core/cat/ftdx10's ASSUMED register
// entry 1, whose own note records that the manual's P12 legend names no fill
// and that the FTdx10's padding has never been observed. Cited here, not
// re-derived: if that entry's Stage R capture moves the byte, this constant
// moves with it.
const tagFill = ' '

// buildMTAnswer builds the 41-byte combined MT answer for slot.
//
// ALWAYS THE FULL WIDTH, which is the DIALECT's ASSUMED register entry 6 seen
// from the other side: that entry records that the manual's grid draws the
// MAXIMAL frame and that a variable-width ANSWER is live, with a recorded
// contingency of a 30..41 window. This fake answers at the width core/cat
// expects (a fake that answered short would fail the parser rather than
// exercise it); if that entry's Stage R capture takes the contingency, this
// builder and core/cat move together.
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
			// exchange core/driver/ftdx10's own register entry 8 reads as
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
		// own. The FT-710's HW-confirmed rejection of a ZERO-BYTE tag Set
		// (fakeradio's register item 7) has no analogue: a 41-byte frame
		// always carries the full 12-byte field, so the shape does not exist
		// on this radio to accept or refuse.
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

// --- MC: MEMORY CHANNEL (recall) (manual lines 1130-1140) ---
//
// Availability O O O X. Set frame (6 bytes) "MC" + 3-byte slot + ';',
// fire-and-forget, recalling the channel; Read frame "MC;" answered by "MC" +
// the 3-byte current channel + ';'. Disambiguated by length, as MT is.
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

// --- ID (manual lines 976-984) ---
//
// Availability X O O X: no Set, Read "ID;", a 7-byte Answer. The VALUE is this
// radio's own CAT ID, 0761 — the one byte-level difference from the FT-710's
// 0800 that a probe turns into a *driver.WrongRadioError.

func buildIDAnswer() []byte { return []byte("ID0761;") }

func (r *Radio) handleID(body []byte) []byte {
	if len(body) != 0 {
		return rejection
	}
	return buildIDAnswer()
}

// --- AI (manual lines 309-320) ---
//
// Availability O O O O. Set and Answer are 4 bytes, Read is "AI;". AI-set is
// fire-and-forget. This fake never PUSHES anything unsolicited whatever AI is
// set to: no FTdx10's AI behaviour has been observed, and the engine's
// drain-to-quiet discipline is already exercised against fakeradio, whose own
// AI-flood facts are the FT-710's.
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
		return nil // fire-and-forget success — register entry 11
	}
	return rejection
}

// --- EX (MENU) ---
//
// handleEX lives in ex.go, beside the generated inventory it answers from and
// the provenance note that explains where that inventory comes from. Only the
// dispatch arm below is here, with every other command's.

// --- Top-level dispatch ---

// handleFrame parses one complete, ';'-terminated frame (as produced by
// reassembler.push) and returns the reply to send: nil for a fire-and-forget
// success, or a non-nil frame — a real answer, or rejection — otherwise.
// Unknown and garbled commands fall through to rejection.
//
// COMMAND NAMES ARE MATCHED IN EITHER CASE, and that is a MANUAL FACT of this
// radio rather than a leniency inherited from fakeradio: "A command consists
// of 2 alphabetical characters. You may use either lower or upper case
// characters." (manual lines 160-161, under the "Alphabetical Commands"
// heading at 159). This arm matched upper case ONLY until the M9d follow-up
// wave, on a register entry that asserted no such statement about the FTdx10
// was cited anywhere in this repository; core/cat/ftdx10/testdata/
// provenance.md's note A4 had cited it since 29/07/2026 (M9c-4 task 7a), the
// day before that entry was written. See doc.go's "What is NOT in this
// register, and why".
//
// FIELD VALUES REMAIN CASE-SENSITIVE (the mode nibble's hex letters, the PMS
// L/U suffix, EMG): the manual's statement is about the two-character command
// NAME and says nothing about parameters, so extending it would be an
// invented leniency. That half is ASSUMED — doc.go register entry 12.
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
