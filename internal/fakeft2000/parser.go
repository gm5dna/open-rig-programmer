// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft2000

// This file is fakeft2000's own, independent byte-level CAT parser and reply
// builder. It is derived from the FT-2000 SERIES CAT OPERATION REFERENCE
// BOOK's own Control Command Table (revision EH025H124) and from
// docs/superpowers/ft2000-capability-matrix.md — NOT from core/cat or
// core/driver/ft2000 (doc.go, THE HARD RULE). The manual is gitignored
// (docs/fixtures-private/manuals/), so "layout:N" citations name where the
// chart is, not a link.

// --- General framing ---

// rejection is the protocol's one and only NAK, "?;". Every refusal in this
// file answers with it and nothing else — an empty slot, an out-of-domain
// slot, a malformed frame, an unknown command and an overflowed accumulator
// are indistinguishable to the host, the fleet-wide convention every
// registered Yaesu fake plays (doc.go, "What is NOT in this register, and
// why").
var rejection = []byte("?;")

// maxAccumulatorBytes is the reassembler's byte cap — this package's own
// bounded-input policy (doc.go's register entry THE FRAME ACCUMULATOR'S CAP
// AND RESYNC), not a manual figure.
const maxAccumulatorBytes = 256

// reassembler turns an arbitrary stream of Write() chunks into complete
// ';'-terminated frames. Framing only: it says nothing about what any frame
// means. Copied in shape from internal/fakeft991a's own.
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
// MC's own legend prints the whole span and its decomposition in one place:
// "P1 001 - 117: Memory Channel Number / 001 - 099: Regular Memory Channel /
// 100: P1L 101: P1U ~ 116: P9L 117: P9U" (layout:844-847). MW and MR draw the
// same span implicitly ("P1 Memory Channel Number", layout:894, :924) — one
// slot space for all three commands, DIRECT DECIMAL CHANNEL NUMBERS
// throughout, no letter-suffix token form anywhere on this radio (matrix
// §2.4).

const slotWireLen = 3

const (
	memoryLo = 1
	memoryHi = 99
	pmsLo    = 100
	pmsHi    = 117
)

// slotNoneWire is the answer-only "no selection yet" form (doc.go's register
// entry 2) — never a valid REQUEST slot, since it falls outside 001-117.
const slotNoneWire = "000"

// slotKind classifies a 3-byte slot code. A pure grammar check: it says
// nothing about whether the slot is populated.
type slotKind int

const (
	slotInvalid slotKind = iota
	slotMemory           // 001-099
	slotPMS              // 100-117
)

func parseSlotForm(s string) slotKind {
	if len(s) != slotWireLen || !isDigit(s[0]) || !isDigit(s[1]) || !isDigit(s[2]) {
		return slotInvalid
	}
	n := int(s[0]-'0')*100 + int(s[1]-'0')*10 + int(s[2]-'0')
	switch {
	case n >= memoryLo && n <= memoryHi:
		return slotMemory
	case n >= pmsLo && n <= pmsHi:
		return slotPMS
	}
	return slotInvalid
}

func validSlot(kind slotKind) bool { return kind == slotMemory || kind == slotPMS }

// --- Field validators (wire level) ---
//
// Every one of them is enforced on MW's Set direction, and every one is
// ASSUMED to be what the radio itself enforces (doc.go's register entry
// SET-DIRECTION FIELD STRICTNESS).

// validClarSign reports whether b is one of P3's two printed direction bytes,
// "Clarifier Direction +: Plus Shift, --: Minus Shift" (layout:924-925).
func validClarSign(b byte) bool { return b == '+' || b == '-' }

// validClarMagDigits reports whether s is a 4-digit clarifier magnitude field
// inside this manual's printed range, "Clarifier Offset: 0000 - 9999 (Hz)"
// (layout:925).
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

func validBoolFlagByte(b byte) bool { return b == '0' || b == '1' }

// validCTCSSByte reports whether b is one of P8's THREE printed values, "0:
// CTCSS OFF 1: CTCSS ENC/DEC 2: CTCSS ENC" (layout:934) — the ordinary
// legacy domain, no DCS member (matrix §2.9).
func validCTCSSByte(b byte) bool { return b >= '0' && b <= '2' }

// validToneDigits reports whether s is a 2-digit ASCII tone-table index. Its
// SHAPE is the only thing enforced — doc.go's register entry TONE INDEX (P9)
// IS STORED AT ITS PRINTED WIDTH, NOT RANGE-CHECKED: CN's separate 0-49
// ceiling is not asserted against this byte pair.
func validToneDigits(s string) bool {
	return len(s) == 2 && isDigit(s[0]) && isDigit(s[1])
}

// validShiftByte reports whether b is one of P10's three printed values, "0:
// Simplex 1: Plus Shift 2: Minus Shift" (layout:936).
func validShiftByte(b byte) bool { return b >= '0' && b <= '2' }

// --- The shared 24-byte memory field block ---
//
// MW's Set and MR's Answer are one chart under two prefixes
// (docs/superpowers/ft2000-capability-matrix.md §1.1, layout:894-905 and
// :924-936). Positions are 0-indexed offsets INTO THE BLOCK — i.e. into a
// frame's bytes after the two-byte command name and before the terminator:
//
//	block offset  width  field
//	0             3      P1  slot
//	3             8      P2  frequency (8-digit ASCII Hz)
//	11            1      P3  clarifier sign
//	12            4      P3  clarifier magnitude
//	16            1      P4  RX clarifier flag
//	17            1      P5  TX clarifier flag
//	18            1      P6  mode nibble
//	19            1      P7  kind
//	20            1      P8  CTCSS state
//	21            2      P9  tone index
//	23            1      P10 shift
//
// memBlockLen (24) plus the two-byte opcode and the terminator makes the
// counted 27-byte MW/MR frame (matrix §1.1).
const memBlockLen = 24

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
	blkToneStart, blkToneEnd       = 21, 23
	blkShift                       = 23
)

// mwSetKindFixed is MW's own Set-direction P7, "P7 0: (Fixed)" (layout:934):
// the write direction carries no channel information at all.
const mwSetKindFixed = '0'

// kindMemory is the P7 byte every populated slot answers with, whichever of
// MR's Read legend's two members, "0: VFO 1: Memory" (layout:894), a channel
// this fake actually stores could honestly be (doc.go's register entry AN
// ANSWER'S KIND BYTE IS ALWAYS '1').
const kindMemory = '1'

// parseMemoryBlock validates a memBlockLen-byte field block against every
// vocabulary MW's own chart prints (doc.go's register entry SET-DIRECTION
// FIELD STRICTNESS) and returns the slot it names together with the state it
// encodes. ok is false on the first violation found; the caller answers "?;"
// and changes nothing.
func parseMemoryBlock(block []byte) (slot string, s MemState, ok bool) {
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
	if !validModeByte(mode) {
		return "", MemState{}, false
	}
	if block[blkKind] != mwSetKindFixed {
		return "", MemState{}, false
	}
	ctcss := block[blkCTCSS]
	if !validCTCSSByte(ctcss) {
		return "", MemState{}, false
	}
	tone := string(block[blkToneStart:blkToneEnd])
	if !validToneDigits(tone) {
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
		// The ANSWER kind, never the Set's fixed placeholder.
		Kind:  kindMemory,
		CTCSS: ctcss,
		Tone:  tone,
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
// memBlockLen-byte field block. It trusts its input — state reaching here
// came from a validated Set or from an image constant.
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
	out = append(out, s.Tone...)
	out = append(out, s.Shift)
	return out
}

// --- MW: MEMORY CHANNEL WRITE (availability "O X X X", layout:113-165's
// index; block layout:924-936) ---
//
// Set only. Set frame (27 bytes) "MW" + the 24-byte field block + ';'.

func (r *Radio) handleMW(body []byte) []byte {
	if len(body) != memBlockLen {
		return rejection
	}
	slot, s, ok := parseMemoryBlock(body)
	if !ok {
		return rejection
	}
	if !validSlot(parseSlotForm(slot)) {
		return rejection
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// Creates an absent channel as well as overwriting a present one
	// (doc.go's register entry AN MW SET CREATES AN ABSENT CHANNEL). The
	// selection is NOT moved (doc.go's register entry A SET DOES NOT MOVE
	// THE SELECTED CHANNEL).
	r.slots[slot] = s
	return nil // fire-and-forget success
}

// --- MR: MEMORY CHANNEL READ (availability "X O O X"; block layout:894-905)
// ---
//
// Read/Answer only. Read frame (6 bytes) "MR" + 3-byte slot + ';'; Answer
// frame (27 bytes) "MR" + the 24-byte field block + ';'.

func buildMRAnswer(slot string, s MemState) []byte {
	out := make([]byte, 0, 2+memBlockLen+1)
	out = append(out, 'M', 'R')
	out = appendMemBlock(out, slot, s)
	out = append(out, ';')
	return out
}

func (r *Radio) handleMR(body []byte) []byte {
	if len(body) != slotWireLen {
		return rejection
	}
	slot := string(body)
	if !validSlot(parseSlotForm(slot)) {
		return rejection
	}
	r.mu.Lock()
	s, ok := r.slots[slot]
	r.mu.Unlock()
	if !ok {
		// Empty slot — ASSUMED, doc.go's register entry EMPTY-SLOT AND
		// OUT-OF-DOMAIN ANSWERS.
		return rejection
	}
	return buildMRAnswer(slot, s)
}

// --- MC: MEMORY CHANNEL (availability "O O O X"; block layout:844-847) ---
//
// Set frame (6 bytes) "MC" + 3-byte slot + ';', fire-and-forget, selecting
// the channel; Read frame "MC;" answered by "MC" + the 3-byte current
// channel + ';'.

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
		if !validSlot(parseSlotForm(slot)) {
			return rejection
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if _, ok := r.slots[slot]; !ok {
			// Empty slot — the EMPTY-SLOT AND OUT-OF-DOMAIN ANSWERS entry.
			return rejection
		}
		r.currentChannel = slot
		return nil // fire-and-forget success
	}
	return rejection
}

// --- ID (availability "X O O X"; block layout:725-732) ---
//
// No Set. Read "ID;", a 7-byte Answer: "ID" + the 4-digit CATID + ';'. The
// VALUE is this Radio's current row: "P1 0251: FT-2000 / 0252: FT-2000D"
// (layout:725-726) — the one byte-level difference either book states
// between the two rows.

func (r *Radio) handleID(body []byte) []byte {
	if len(body) != 0 {
		return rejection
	}
	r.mu.Lock()
	catID := r.catID
	r.mu.Unlock()
	out := make([]byte, 0, 2+4+1)
	out = append(out, 'I', 'D')
	out = append(out, catID...)
	out = append(out, ';')
	return out
}

// --- AI: AUTO INFORMATION (availability "O O O X"; block layout:201-206) ---
//
// Set frame (4 bytes) "AI" + P1 + ';', fire-and-forget; Read "AI;" (3 bytes)
// answered by "AI" + P1 + ';' (4 bytes). P1 is "0: Auto Information OFF /
// 1: Auto Information ON" (layout:202-203).
//
// core/transport.Engine.Init opens every CAT session with an unconditional
// "AI0;" (ClassWrite), so this handler's silent-accept path is on the
// critical path of every fake session — an unmodelled AI answering "?;" like
// any other unknown command would make Init's write draw a rejection and
// fail the open outright.
//
// THIS FAKE NEVER PUSHES AN UNSOLICITED FRAME, WHATEVER AI IS SET TO —
// doc.go's register entry AUTOMATIC-INFORMATION SUPPRESSION. "AI1;" is
// accepted, stored and read back faithfully, and then nothing follows from
// it: this package writes to the port only in reply to a frame that arrived
// on it.

func (r *Radio) handleAI(body []byte) []byte {
	switch len(body) {
	case 0:
		r.mu.Lock()
		ai := r.ai
		r.mu.Unlock()
		return []byte{'A', 'I', ai, ';'}
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

// --- Top-level dispatch ---

// upperASCII folds the two ASCII bytes of a command name to upper case and
// leaves every other byte alone. Not strings.ToUpper: this must be a
// byte-exact, length-preserving fold over arbitrary wire bytes, not a
// Unicode-aware one.
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
// success, or a non-nil frame — a real answer, or rejection — otherwise.
//
// COMMAND NAMES ARE MATCHED IN EITHER CASE, a MANUAL FACT: "A command
// consists of 2 alphabetical characters. You may use either lower or upper
// case characters." (layout:106-107). Field values remain case-sensitive:
// the manual's statement is about the two-character command name alone.
func (r *Radio) handleFrame(frame []byte) []byte {
	if len(frame) == 0 || frame[len(frame)-1] != ';' {
		return rejection // defensive: the reassembler never hands us this
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
	case [2]byte{'M', 'W'}:
		return r.handleMW(rest)
	case [2]byte{'M', 'R'}:
		return r.handleMR(rest)
	case [2]byte{'M', 'C'}:
		return r.handleMC(rest)
	default:
		return rejection
	}
}
