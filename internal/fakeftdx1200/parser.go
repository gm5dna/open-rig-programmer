// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftdx1200

// This file is fakeftdx1200's own, independent byte-level CAT parser and
// reply builder. It is derived from
// docs/superpowers/ftdx1200-capability-matrix.md — NOT from core/cat or
// core/driver/ftdx1200 (doc.go, THE HARD RULE).

// --- General framing ---

// rejection is the protocol's one and only NAK, "?;". Every refusal in this
// file answers with it and nothing else — an empty slot, an out-of-domain
// slot, a malformed frame, an unknown command and an overflowed
// accumulator are indistinguishable to the host, the fleet-wide convention
// every registered Yaesu fake plays.
var rejection = []byte("?;")

// maxAccumulatorBytes is the reassembler's byte cap — this package's own
// bounded-input policy (doc.go's register entry THE FRAME ACCUMULATOR'S CAP
// AND RESYNC), not a manual figure.
const maxAccumulatorBytes = 256

// reassembler turns an arbitrary stream of Write() chunks into complete
// ';'-terminated frames. Framing only: it says nothing about what any frame
// means. Copied in shape from fakeft2000's own.
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
// (terminator included) or an overflow signal (frame == nil, overflow
// true).
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
// MR and MW's own P1 legends both print "(001 ～ 117)" (matrix §2.4) — one
// slot space, DIRECT DECIMAL CHANNEL NUMBERS, no letter-suffix token form
// anywhere on this radio. "000" is deliberately excluded — doc.go's
// register entry CHANNEL "000" IS OUT OF SCOPE.

const slotWireLen = 3

const (
	memoryLo = 1
	memoryHi = 99
	pmsLo    = 100
	pmsHi    = 117
)

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

func validClarSign(b byte) bool { return b == '+' || b == '-' }

// validClarMagDigits reports whether s is a 4-digit clarifier magnitude
// field inside this manual's printed range, "0000 - 9999 (Hz)".
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
// CTCSS OFF 1: CTCSS ENC/DEC 2: CTCSS ENC" (matrix §2.9) — no DCS member.
func validCTCSSByte(b byte) bool { return b >= '0' && b <= '2' }

// toneFixedBytes is the ONLY two-byte value P9 may carry, on either
// direction — "P10 00: (Fixed)" on MR, "P9 00: (Fixed)" on MW (matrix
// §1.3, doc.go's register entry P9 (TONE) IS PRINTED-FIXED).
const toneFixedBytes = "00"

// validToneBytes reports whether s is the literal fixed tone field. Unlike
// fakeft2000's live tone index, this is not a shape check: any two-digit
// value other than "00" is refused, since the matrix states this byte pair
// is fixed, not merely two ASCII digits wide.
func validToneBytes(s string) bool { return s == toneFixedBytes }

// validShiftByte reports whether b is one of P10's three printed values,
// "0: Simplex 1: Plus Shift 2: Minus Shift".
func validShiftByte(b byte) bool { return b >= '0' && b <= '2' }

// --- The shared 24-byte memory field block ---
//
// MW's Set and MR's Answer are one chart under two prefixes
// (docs/superpowers/ftdx1200-capability-matrix.md §1.1). Positions are
// 0-indexed offsets INTO THE BLOCK — i.e. into a frame's bytes after the
// two-byte command name and before the terminator:
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
//	21            2      P9  tone (fixed "00")
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

// mwSetKindFixed is MW's own Set-direction P7, "P7 0: (Fixed)" (matrix
// §1.4): the write direction carries no channel information at all.
const mwSetKindFixed = '0'

// kindMemory is the P7 byte every populated slot answers with, whichever of
// MR's Read legend's two members, "0: VFO 1: Memory" (matrix §1.4), a
// channel this fake actually stores could honestly be (doc.go's register
// entry AN ANSWER'S KIND BYTE IS ALWAYS '1').
const kindMemory = '1'

// parseMemoryBlock validates a memBlockLen-byte field block against every
// vocabulary this matrix prints (doc.go's register entry SET-DIRECTION
// FIELD STRICTNESS) and returns the
// slot it names together with the state it encodes. ok is false on the
// first violation found; the caller answers "?;" and changes nothing.
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
	if !validToneBytes(tone) {
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
// came from a validated Set or from an image constant. The tone field is
// ALWAYS the literal "00" (doc.go's register entry P9 (TONE) IS
// PRINTED-FIXED), never s's own state,
// since MemState carries no Tone value to read back.
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
	out = append(out, toneFixedBytes...)
	out = append(out, s.Shift)
	return out
}

// --- MW: MEMORY CHANNEL WRITE ---
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
	// Creates an absent channel as well as overwriting a present one —
	// nothing in this matrix's MW block names a precondition (mirrors
	// fakeft2000's own identical choice).
	r.slots[slot] = s
	return nil // fire-and-forget success
}

// --- MR: MEMORY CHANNEL READ ---
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

// --- ID (block: matrix §2.2) ---
//
// No Set. Read "ID;", a 7-byte Answer: "ID" + the 4-digit CATID + ';'. The
// VALUE is THIS Radio's own configured id — "0582" by default, or "0583"
// with WithFFT1NotFitted (options.go). Accepting either ID as a valid
// answer is the DRIVER's identify() concern (matrix §2.2's own note); this
// fake, like any one real radio, only ever gives ONE of the two.

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

// --- AI: AUTO INFORMATION ---
//
// Set frame (4 bytes) "AI" + P1 + ';', fire-and-forget; Read "AI;" (3
// bytes) answered by "AI" + P1 + ';' (4 bytes).
//
// core/transport.Engine.Init opens every CAT session with an unconditional
// "AI0;" (ClassWrite), so this handler's silent-accept path is on the
// critical path of every fake session — an unmodelled AI answering "?;"
// like any other unknown command would make Init's write draw a rejection
// and fail the open outright. Doc.go's register entry MC (CURRENT CHANNEL
// SELECTION) IS NOT IMPLEMENTED: AI is modelled for this
// reason, not because the matrix documents this radio's own AI legend.
//
// THIS FAKE NEVER PUSHES AN UNSOLICITED FRAME, WHATEVER AI IS SET TO. "AI1;"
// is accepted, stored and read back faithfully, and then nothing follows
// from it: this package writes to the port only in reply to a frame that
// arrived on it.

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
// reassembler.push) and returns the reply to send: nil for a
// fire-and-forget success, or a non-nil frame — a real answer, or
// rejection — otherwise.
//
// COMMAND NAMES ARE MATCHED IN EITHER CASE, the same fleet-wide convention
// every registered Yaesu fake plays. Field values remain case-sensitive.
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
	default:
		return rejection
	}
}
