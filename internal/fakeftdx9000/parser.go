// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx9000

// This file is fakeftdx9000's own, independent byte-level CAT parser and
// reply builder for the FTdx9000. It is derived from this project's own
// capability matrix (docs/superpowers/ftdx9000-capability-matrix.md), and
// through it the FTdx9000 CAT Operation Reference Book's own position charts
// — cited "ftdx9000_layout.txt:NNNN" as the matrix cites them, since the
// manual is gitignored (docs/fixtures-private/manuals/) — and NOT from
// core/driver/ftdx9000, which this package is forbidden to read. See doc.go
// for why that independence matters and for the full ASSUMED register;
// individual assumed points are flagged inline, next to the code that
// implements them.

import "strconv"

// --- General framing ---

// rejection is the protocol's one and only NAK, "?;" — doc.go's register
// entry THE "?;" REJECTION CONVENTION, AND SILENCE ON AN ACCEPTED SET.
var rejection = []byte("?;")

// maxAccumulatorBytes is the reassembler's byte cap — doc.go's register entry
// THE FRAME ACCUMULATOR'S CAP AND RESYNC, this package's own bounded-input
// policy, not a manual figure.
const maxAccumulatorBytes = 256

// reassembler turns an arbitrary stream of Write() chunks into complete
// ';'-terminated frames. Framing only: it says nothing about what any frame
// means. Copied in shape from every sibling fake's identical reassembler.
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
// P1's own legend decomposes the whole span: "001 - 117: Memory Channel
// Number", "001 - 099: Regular Memory Channel", "100: P1L ... 117: P9U"
// (matrix §1.4, ftdx9000_layout.txt:950-957). DECIMAL CHANNEL NUMBERS, not a
// token form — there is no "P1L" slot here.

// slotNoneWire is the answer-only "no selection" form (New's own doc
// comment). Never a valid REQUEST slot.
const slotNoneWire = "000"

// slotWireLen is the width of every slot code on the wire: three ASCII
// digits, matching P1's cells in every chart.
const slotWireLen = 3

// The printed span's own bounds (matrix §1.4).
const (
	memoryLo = 1
	memoryHi = 99
	pmsLo    = 100
	pmsHi    = 117
)

// slotKind classifies a 3-byte slot code.
type slotKind int

const (
	slotInvalid slotKind = iota
	slotNone             // "000" — answer-only, never a valid request
	slotMemory           // 001-099
	slotPMS              // 100-117, the nine P1L..P9U pairs
)

// parseSlotForm classifies s per the P1 legend. Pure grammar check; says
// nothing about whether the slot is populated.
func parseSlotForm(s string) slotKind {
	if len(s) != slotWireLen {
		return slotInvalid
	}
	if s == slotNoneWire {
		return slotNone
	}
	if !isDigit(s[0]) || !isDigit(s[1]) || !isDigit(s[2]) {
		return slotInvalid
	}
	n, _ := strconv.Atoi(s)
	switch {
	case n >= memoryLo && n <= memoryHi:
		return slotMemory
	case n >= pmsLo && n <= pmsHi:
		return slotPMS
	}
	return slotInvalid
}

// mrReadableSlot reports whether kind is a slot an MR read may name — the
// whole 001-117 span (matrix §1.4/§2). "000" is answer-only.
func mrReadableSlot(kind slotKind) bool { return kind == slotMemory || kind == slotPMS }

// mcSelectableSlot reports whether kind is a slot MC may recall or recall-set.
// MCSelectsAll: this radio's MC legend prints the whole 001-117 span with no
// exclusion (matrix §1.4's own MC citation), matching the driver's own
// MCSelectsAll posture.
func mcSelectableSlot(kind slotKind) bool { return kind == slotMemory || kind == slotPMS }

// --- Field validators (wire level, doc.go's register entry SET-DIRECTION
// FIELD STRICTNESS) ---

// validModeByte reports whether b is one of the twelve mode nibbles this
// radio's legend prints identically on MR and MW: "1: LSB 2: USB 3: CW 4: FM
// 5: AM 6: FSK (RTTY-LSB) 7: CW-R 8: PKT-L 9: FSK-R (RTTY-USB) A: PKT-FM
// B: FM-N C: PKT-U" (matrix §1.5, ftdx9000_layout.txt:1010-1012, 1036-1038).
// 'D', 'E', 'F' are absent — printed nowhere.
func validModeByte(b byte) bool {
	switch {
	case b >= '1' && b <= '9':
		return true
	case b >= 'A' && b <= 'C':
		return true
	}
	return false
}

// validCTCSSByte reports whether b is one of P8's THREE printed values,
// "0: CTCSS OFF 1: CTCSS ENC/DEC 2: CTCSS ENC" (matrix §1.16,
// ftdx9000_layout.txt:1017, 1041). No DCS member on this radio.
func validCTCSSByte(b byte) bool { return b >= '0' && b <= '2' }

// validToneIndex reports whether s is a 2-digit ASCII tone-chart index in the
// live domain "00"-"49" — matrix §1.10, the SAME 50-entry chart as
// CTCSSTones, not the fixed placeholder some sibling radios carry.
func validToneIndex(s string) bool {
	if len(s) != 2 || !isDigit(s[0]) || !isDigit(s[1]) {
		return false
	}
	n, _ := strconv.Atoi(s)
	return n >= 0 && n <= 49
}

// validShiftByte reports whether b is one of P10's three printed values,
// "0: Simplex 1: Plus Shift 2: Minus Shift" (matrix §1.15,
// ftdx9000_layout.txt:1017, 1043).
func validShiftByte(b byte) bool { return b >= '0' && b <= '2' }

func validBoolFlagByte(b byte) bool { return b == '0' || b == '1' }

// validClarSign reports whether b is one of P3's two printed direction bytes,
// "+: Plus Shift, -: Minus Shift" (matrix §2, ftdx9000_layout.txt:1005-1006,
// 1031-1032). The wire carries exactly one byte for it — the frame's own
// counted width leaves no room for more.
func validClarSign(b byte) bool { return b == '+' || b == '-' }

// validClarMagDigits reports whether s is a 4-digit clarifier magnitude field
// inside this manual's printed range, "0000 - 9999 (Hz)" (matrix §1.7). No
// narrower step is enforced here — that is the shared dialect's own deduced
// policy, a different question from what this fake, modelling the radio,
// accepts off the wire.
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

// --- The shared 24-byte field block (MR-Answer / MW-Set) ---
//
// Matrix §2's own table, 0-indexed offsets into the block (i.e. after the
// two-byte command name, before the trailing ';'):
//
//	block offset  width  field
//	0-2           3      P1  slot
//	3-10          8      P2  frequency (Hz), ASCII decimal
//	11            1      P3  clarifier sign
//	12-15         4      P3  clarifier magnitude
//	16            1      P4  RX clarifier flag
//	17            1      P5  TX clarifier flag
//	18            1      P6  mode
//	19            1      P7  kind (Set: fixed '0'; Answer: '0' VFO / '1' Memory)
//	20            1      P8  CTCSS state
//	21-22         2      P9  tone-chart index, live
//	23            1      P10 shift

const blockLen = 24

const (
	blkSlotStart, blkSlotEnd       = 0, 3
	blkFreqStart, blkFreqEnd       = 3, 11
	blkClarSign                    = 11
	blkClarMagStart, blkClarMagEnd = 12, 16
	blkRXClar                      = 16
	blkTXClar                      = 17
	blkMode                        = 18
	blkKind                        = 19
	blkCTCSS                       = 20
	blkP9Start, blkP9End           = 21, 23
	blkShift                       = 23
)

// freqDigits is P2's fixed width — 8 ASCII digits, ONE NARROWER than the
// registered 9-digit Yaesu frame (matrix §2, the whole reason this radio's
// frame is 27 bytes rather than 28).
const freqDigits = 8

// kindMemory is the P7 byte every populated slot in this fake answers with —
// memory channels and PMS band-edges alike. Doc.go's register entry PMS SLOTS
// ANSWER P7 '1' covers the PMS half; the memory half is a MANUAL FACT (MR's
// own Read legend, "0: VFO 1: Memory").
const kindMemory = '1'

// kindSetFixed is P7's Set-direction byte, printed "0: (Fixed)" on MW's own
// chart (matrix §2, ftdx9000_layout.txt:1040-1041).
const kindSetFixed = '0'

// parseFieldBlock validates a blockLen-byte field block against every
// vocabulary the matrix's §2 table prints (doc.go's register entry
// SET-DIRECTION FIELD STRICTNESS) and returns the slot it names together with
// the state it encodes. ok is false on the first violation found; the caller
// answers "?;" and changes nothing.
//
// The clarifier sign, magnitude and both flags are stored exactly as they
// arrived — doc.go's register entry THE CLARIFIER IS STORED — so the next MR
// read answers those same bytes.
func parseFieldBlock(block []byte) (slot string, s MemState, ok bool) {
	if len(block) != blockLen {
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
	if !validModeByte(mode) {
		return "", MemState{}, false
	}
	if block[blkKind] != kindSetFixed {
		return "", MemState{}, false
	}
	ctcss := block[blkCTCSS]
	if !validCTCSSByte(ctcss) {
		return "", MemState{}, false
	}
	tone := string(block[blkP9Start:blkP9End])
	if !validToneIndex(tone) {
		return "", MemState{}, false
	}
	shift := block[blkShift]
	if !validShiftByte(shift) {
		return "", MemState{}, false
	}

	return slot, MemState{
		Freq:     string(freq),
		ClarSign: sign,
		ClarMag:  mag,
		RXClar:   rx == '1',
		TXClar:   tx == '1',
		Mode:     mode,
		Kind:     kindMemory, // the ANSWER kind, never the Set's placeholder
		CTCSS:    ctcss,
		Tone:     tone,
		Shift:    shift,
	}, true
}

func boolFlagByte(b bool) byte {
	if b {
		return '1'
	}
	return '0'
}

// appendFieldBlock concatenates an already-validated MemState into its
// blockLen-byte field block. It trusts its input.
func appendFieldBlock(out []byte, slot string, s MemState) []byte {
	out = append(out, slot...)
	out = append(out, s.Freq...)
	out = append(out, s.ClarSign)
	out = append(out, s.ClarMag...)
	out = append(out, boolFlagByte(s.RXClar))
	out = append(out, boolFlagByte(s.TXClar))
	out = append(out, s.Mode)
	out = append(out, s.Kind)
	out = append(out, s.CTCSS)
	out = append(out, s.Tone...)
	out = append(out, s.Shift)
	return out
}

// --- MR: MEMORY CHANNEL READ (Read/Answer only, no Set — this radio's own
// command list gives MR "X O O X"; matrix §2) ---

func buildMRAnswer(slot string, s MemState) []byte {
	out := make([]byte, 0, 2+blockLen+1)
	out = append(out, 'M', 'R')
	out = appendFieldBlock(out, slot, s)
	out = append(out, ';')
	return out
}

func (r *Radio) handleMR(body []byte) []byte {
	if len(body) != slotWireLen {
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
		return rejection // doc.go's register entry EMPTY-SLOT ANSWERS "?;"
	}
	return buildMRAnswer(slot, s)
}

// --- MW: MEMORY CHANNEL WRITE (Set only, no Read, no Answer — this radio's
// own command list gives MW "O X X X"; matrix §2) ---

func (r *Radio) handleMW(body []byte) []byte {
	if len(body) != blockLen {
		return rejection
	}
	slot, s, ok := parseFieldBlock(body)
	if !ok {
		return rejection
	}
	// A Set must name a real slot (memory or PMS); the answer-only "000" and
	// anything outside 001-117 are refused.
	if kind := parseSlotForm(slot); kind != slotMemory && kind != slotPMS {
		return rejection
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	// A Set creates an absent channel and overwrites an existing one; the
	// selection is not moved — doc.go's register entry A SET DOES NOT MOVE THE
	// SELECTED CHANNEL.
	r.slots[slot] = s
	return nil // fire-and-forget success
}

// --- ID (Read/Answer only; matrix §1.2) ---

// catIDDefault is doc.go's register entry THE DEFAULT CAT-ID ANSWER's
// default of three legal answers.
const catIDDefault = "0101"

func buildIDAnswer(id string) []byte { return []byte("ID" + id + ";") }

func (r *Radio) handleID(body []byte) []byte {
	if len(body) != 0 {
		return rejection
	}
	r.mu.Lock()
	id := r.catID
	r.mu.Unlock()
	return buildIDAnswer(id)
}

// --- AI (Set/Read/Answer, no push — doc.go's register entry
// AUTOMATIC-INFORMATION SUPPRESSION) ---

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

// --- MC: MEMORY CHANNEL (recall) — Set/Read/Answer, MCSelectsAll (matrix
// §1.4's own MC citation covers the whole 001-117 span with no exclusion) ---

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
		if !mcSelectableSlot(parseSlotForm(slot)) {
			return rejection
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if _, ok := r.slots[slot]; !ok {
			return rejection // doc.go's register entry EMPTY-SLOT ANSWERS "?;"
		}
		r.currentChannel = slot
		return nil // fire-and-forget success
	}
	return rejection
}

// --- Top-level dispatch ---

// upperASCII folds the two ASCII bytes of a command name to upper case and
// leaves every other byte alone. NOT strings/bytes.ToUpper — see every
// sibling fake's identical function for why (Unicode folding can change a
// body's byte length).
func upperASCII(name [2]byte) [2]byte {
	for i, b := range name {
		if b >= 'a' && b <= 'z' {
			name[i] = b - 'a' + 'A'
		}
	}
	return name
}

// handleFrame parses one complete, ';'-terminated frame (as produced by
// reassembler.push) and returns the reply to send: nil for a fire-and-forget
// success, or a non-nil frame otherwise.
//
// COMMAND NAMES ARE MATCHED IN EITHER CASE — this radio's own manual fact,
// "A command consists of 2 alphabetical characters. You may use either lower
// or upper case characters." (ftdx9000_layout.txt:108-109), the same
// sentence every sibling Yaesu manual states.
func (r *Radio) handleFrame(frame []byte) []byte {
	if len(frame) == 0 || frame[len(frame)-1] != ';' {
		return rejection
	}
	body := frame[:len(frame)-1]
	if len(body) < 2 {
		return rejection
	}
	cmd := upperASCII([2]byte{body[0], body[1]})
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
	case [2]byte{'M', 'C'}:
		return r.handleMC(rest)
	default:
		return rejection
	}
}
