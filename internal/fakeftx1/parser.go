// SPDX-License-Identifier: GPL-3.0-or-later

package fakeftx1

// This file is fakeftx1's own, independent byte-level CAT parser and
// reply builder. It is derived from
// `.superpowers/sdd/2026-09-18-v1100-ftx1/reviews/spec.md` (the FTX-1
// manual's own reading) — NOT from core/cat or any future
// core/driver/ftx1 (doc.go, THE HARD RULE).

// --- General framing ---

// rejection is the protocol's one and only NAK, "?;". Every refusal in
// this file answers with it and nothing else — an empty slot, an
// out-of-domain address, a malformed frame, an unknown command (MC, VM,
// GT, EX included — doc.go) and an overflowed accumulator are
// indistinguishable to the host, the fleet-wide convention every
// registered Yaesu fake plays.
var rejection = []byte("?;")

// maxAccumulatorBytes is the reassembler's byte cap — this package's own
// bounded-input policy (doc.go's register entry THE FRAME ACCUMULATOR'S
// CAP AND RESYNC), not a manual figure.
const maxAccumulatorBytes = 256

// reassembler turns an arbitrary stream of Write() chunks into complete
// ';'-terminated frames. Framing only: it says nothing about what any
// frame means. Copied in shape from internal/fakeftdx3000's own.
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

// accEvent is one unit reassembler.push hands back: either a complete
// frame (terminator included) or an overflow signal (frame == nil,
// overflow true).
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

// --- Address grammar (spec.md §4) ---
//
// Every FTX-1 slot form is 5 bytes wide:
//
//	Memory  "00001"-"00999"   direct decimal channel number
//	PMS     "P-01L"-"P-50U"   a hyphen, a two-digit pair (01-50), L/U
//	5 MHz   "50001"-"50020"
//	EMGCH   "EMGCH"           the one fixed literal
//
// MR, MT and MW all draw from this same four-class list in this fake —
// doc.go's register entries EMPTY-SLOT AND OUT-OF-DOMAIN ANSWERS and MW
// FOLLOWS MR'S DOMAIN, NOT MW'S OWN NARROWER PRINTED P1 (MW's own printed
// P1 gap is not followed here). The "00000" none-form is not specially
// cased: it is simply out of every class's numeric range.

const addrWireLen = 5

// slotKind classifies a 5-byte address. A pure grammar check: it says
// nothing about whether the address is populated.
type slotKind int

const (
	slotInvalid slotKind = iota
	slotMemory           // 00001-00999
	slotPMS              // P-01L-P-50U
	slotFiveMHz          // 50001-50020
	slotEMG              // "EMGCH"
)

const (
	memoryLo, memoryHi   = 1, 999
	fiveMHzLo, fiveMHzHi = 50001, 50020
	pmsPairLo, pmsPairHi = 1, 50
)

// emgWire is the one fixed emergency-channel literal (spec.md §4).
const emgWire = "EMGCH"

// digitsToInt converts a run of ASCII digit bytes to an int. The caller
// must already have validated every byte is a digit.
func digitsToInt(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

// classifySlot classifies a 5-byte wire address per the grammar above.
func classifySlot(addr string) slotKind {
	if len(addr) != addrWireLen {
		return slotInvalid
	}
	if addr == emgWire {
		return slotEMG
	}
	if addr[0] == 'P' && addr[1] == '-' && isDigit(addr[2]) && isDigit(addr[3]) &&
		(addr[4] == 'L' || addr[4] == 'U') {
		pair := int(addr[2]-'0')*10 + int(addr[3]-'0')
		if pair >= pmsPairLo && pair <= pmsPairHi {
			return slotPMS
		}
		return slotInvalid
	}
	for i := 0; i < addrWireLen; i++ {
		if !isDigit(addr[i]) {
			return slotInvalid
		}
	}
	n := digitsToInt(addr)
	switch {
	case n >= memoryLo && n <= memoryHi:
		return slotMemory
	case n >= fiveMHzLo && n <= fiveMHzHi:
		return slotFiveMHz
	}
	return slotInvalid
}

func validAddr(addr string) bool { return classifySlot(addr) != slotInvalid }

// pmsWire renders PMS pair (1-50) and half ('L'/'U') as its dash-token
// wire form, e.g. pmsWire(1, 'L') == "P-01L" (spec.md §4).
func pmsWire(pair int, half byte) string {
	digits := [2]byte{byte('0' + pair/10), byte('0' + pair%10)}
	return "P-" + string(digits[:]) + string(half)
}

// --- Field validators (wire level, MW's Set direction) ---
//
// Every one of them is ASSUMED to be what the radio itself enforces
// (doc.go register entry 3): the manual prints the legends, never what a
// Set that leaves one does.

func validFreqDigits(s string) bool {
	if len(s) != freqWireLen {
		return false
	}
	for i := 0; i < freqWireLen; i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

// validClarSign reports whether b is one of P3's two printed direction
// bytes (spec.md §3.1 row P3, §7).
func validClarSign(b byte) bool { return b == '+' || b == '-' }

// validClarMagDigits reports whether s is a 4-digit clarifier magnitude
// field inside "0000-9990 (Hz)" (spec.md §7).
func validClarMagDigits(s string) bool {
	if len(s) != 4 {
		return false
	}
	for i := 0; i < 4; i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return digitsToInt(s) <= 9990
}

func validBoolFlagByte(b byte) bool { return b == '0' || b == '1' }

// validModeByte reports whether m is one of the 18-nibble legend's 16
// named members, "0"-"9","A"-"F" (the FT-710's own table) plus 'H'
// (C4FM-DN) and 'I' (C4FM-VW) — spec.md §5. 'G'/'J' are ASSUMED reserved
// and refused.
func validModeByte(m byte) bool {
	switch {
	case m >= '0' && m <= '9':
		return true
	case m >= 'A' && m <= 'F':
		return true
	case m == 'H' || m == 'I':
		return true
	}
	return false
}

// validKindByte reports whether b is one of P7's six legend bytes,
// "0"-"5" (spec.md §3.1 row P7: VFO/Memory/Memory Tune/QMB/-/PMS). Any
// legend byte is accepted and stored verbatim — doc.go's register entry
// KIND (P7) AND TONE (P8) ARE STORED VERBATIM, NOT HARDWARE-CONFIRMED.
func validKindByte(b byte) bool { return b >= '0' && b <= '5' }

// validToneByte reports whether b is one of P8's six legend bytes,
// "0"-"5" (spec.md §6: OFF/CTCSS ENC-DEC/CTCSS ENC/DCS/PR FREQ/REV TONE).
// Same treatment as validKindByte — doc.go's register entry KIND (P7) AND
// TONE (P8) ARE STORED VERBATIM, NOT HARDWARE-CONFIRMED.
func validToneByte(b byte) bool { return b >= '0' && b <= '5' }

// validShiftByte reports whether b is one of P10's three printed values,
// "0" Simplex, "1" Plus, "2" Minus (spec.md §3.1 row P10).
func validShiftByte(b byte) bool { return b >= '0' && b <= '2' }

// p9Fixed is the literal MW must carry on P9 — `"00" (Fixed)` (spec.md
// §3.1 row P9) — doc.go's register entry P9 MUST BE THE LITERAL "00" ON
// EVERY SET. MR always answers it too.
const p9Fixed = "00"

// --- The shared 27-byte MR/MW memory field block (spec.md §3.1) ---
//
// Positions are 0-indexed offsets INTO THE BLOCK — i.e. into a frame's
// bytes after the two-byte command name and before the terminator:
//
//	block offset  width  field
//	0             5      P1  address
//	5             9      P2  frequency (9-digit ASCII Hz)
//	14            1      P3  clarifier sign
//	15            4      P3  clarifier magnitude
//	19            1      P4  RX clarifier flag
//	20            1      P5  TX clarifier flag
//	21            1      P6  mode
//	22            1      P7  kind
//	23            1      P8  tone
//	24            2      P9  "00" (Fixed)
//	26            1      P10 shift
//
// memBlockLen (27) plus the two-byte opcode and the terminator makes the
// counted 30-byte MR-answer/MW-set frame.
const freqWireLen = 9
const memBlockLen = 27

const (
	blkAddrStart, blkAddrEnd       = 0, addrWireLen
	blkFreqStart, blkFreqEnd       = blkAddrEnd, blkAddrEnd + freqWireLen
	blkClarSign                    = blkFreqEnd
	blkClarMagStart, blkClarMagEnd = blkClarSign + 1, blkClarSign + 5
	blkRXClar                      = blkClarMagEnd
	blkTXClar                      = blkRXClar + 1
	blkMode                        = blkTXClar + 1
	blkKind                        = blkMode + 1
	blkTone                        = blkKind + 1
	blkP9Start, blkP9End           = blkTone + 1, blkTone + 3
	blkShift                       = blkP9End
)

// parseMemoryBlock validates a memBlockLen-byte field block against every
// vocabulary spec.md §3.1 prints (doc.go register entry 3) and returns
// the address it names together with the state it encodes. ok is false
// on the first violation found; the caller answers "?;" and changes
// nothing.
func parseMemoryBlock(block []byte) (addr string, s MemState, ok bool) {
	if len(block) != memBlockLen {
		return "", MemState{}, false
	}
	addr = string(block[blkAddrStart:blkAddrEnd])

	freq := string(block[blkFreqStart:blkFreqEnd])
	if !validFreqDigits(freq) {
		return "", MemState{}, false
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
	kind := block[blkKind]
	if !validKindByte(kind) {
		return "", MemState{}, false
	}
	tone := block[blkTone]
	if !validToneByte(tone) {
		return "", MemState{}, false
	}
	if string(block[blkP9Start:blkP9End]) != p9Fixed {
		return "", MemState{}, false
	}
	shift := block[blkShift]
	if !validShiftByte(shift) {
		return "", MemState{}, false
	}

	return addr, MemState{
		Freq:     freq,
		ClarSign: sign,
		ClarMag:  mag,
		RXClar:   rx == '1',
		TXClar:   tx == '1',
		Mode:     mode,
		Kind:     kind,
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

// appendMemBlock concatenates an already-validated MemState into its
// memBlockLen-byte field block. It trusts its input — state reaching here
// came from a validated Set or from an image constant.
func appendMemBlock(out []byte, addr string, s MemState) []byte {
	out = append(out, addr...)
	out = append(out, s.Freq...)
	out = append(out, s.ClarSign)
	out = append(out, s.ClarMag...)
	out = append(out, boolFlagByte(s.RXClar))
	out = append(out, boolFlagByte(s.TXClar))
	out = append(out, s.Mode)
	out = append(out, s.Kind)
	out = append(out, s.Tone)
	out = append(out, p9Fixed...)
	out = append(out, s.Shift)
	return out
}

// --- MW: MEMORY WRITE (spec.md §3.2) ---
//
// Set only ("Set O, Read X, Ans X" — spec.md §9). Set frame (30 bytes)
// "MW" + the 27-byte field block + ';'.

func (r *Radio) handleMW(body []byte) []byte {
	if len(body) != memBlockLen {
		return rejection
	}
	addr, s, ok := parseMemoryBlock(body)
	if !ok {
		return rejection
	}
	if !validAddr(addr) {
		return rejection
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	// Creates an absent slot as well as overwriting a present one — doc.go's
	// register entry A SET CREATES AN ABSENT SLOT. Never touches r.tags
	// (register entry MW AND MT MUTATE INDEPENDENT FIELDS).
	r.slots[addr] = s
	// MW answers an accepted Set with silence, unlike MT's echo — doc.go's
	// register entry MT'S SET ANSWERS WITH AN ECHO (its own converse).
	return nil // fire-and-forget success
}

// --- MR: MEMORY READ (spec.md §3.1) ---
//
// Read/Answer only ("Set X, Read O, Ans O" — spec.md §9). Read frame (8
// bytes) "MR" + 5-byte address + ';'; Answer frame (30 bytes) "MR" + the
// 27-byte field block + ';'.

func buildMRAnswer(addr string, s MemState) []byte {
	out := make([]byte, 0, 2+memBlockLen+1)
	out = append(out, 'M', 'R')
	out = appendMemBlock(out, addr, s)
	out = append(out, ';')
	return out
}

func (r *Radio) handleMR(body []byte) []byte {
	if len(body) != addrWireLen {
		return rejection
	}
	addr := string(body)
	if !validAddr(addr) {
		return rejection
	}
	r.mu.Lock()
	s, ok := r.slots[addr]
	r.mu.Unlock()
	if !ok {
		// Empty slot — doc.go register entry 1.
		return rejection
	}
	return buildMRAnswer(addr, s)
}

// --- MT: MEMORY CHANNEL WRITE/TAG (spec.md §3.3) ---
//
// Set, Read AND Answer are all real ("Set O, Read O, Ans O" — spec.md
// §9). Read frame (8 bytes) "MT" + address(5) + ';'; Set/Answer frame (20
// bytes) "MT" + address(5) + tag(12) + ';' — NO DISPLAY BYTE
// (f1-report.md item 3).

const tagWireLen = 12
const mtSetBodyLen = addrWireLen + tagWireLen

// validTag reports whether tag is an acceptable 12-byte MT tag field:
// exactly 12 bytes, printable ASCII 0x20-0x7E, excluding ';' (the frame
// terminator — accepting it would make command injection possible).
// Doc.go's register entry TAG CHARSET AND INJECTION SAFETY.
func validTag(tag []byte) bool {
	if len(tag) != tagWireLen {
		return false
	}
	for _, b := range tag {
		if b < 0x20 || b > 0x7E || b == ';' {
			return false
		}
	}
	return true
}

func buildMTAnswer(addr string, tag string) []byte {
	out := make([]byte, 0, 2+addrWireLen+tagWireLen+1)
	out = append(out, 'M', 'T')
	out = append(out, addr...)
	out = append(out, tag...)
	out = append(out, ';')
	return out
}

func (r *Radio) handleMT(body []byte) []byte {
	switch len(body) {
	case addrWireLen:
		addr := string(body)
		if !validAddr(addr) {
			return rejection
		}
		r.mu.Lock()
		tag, ok := r.tags[addr]
		r.mu.Unlock()
		if !ok {
			// No tag ever recorded for this address — doc.go register
			// entry 1.
			return rejection
		}
		return buildMTAnswer(addr, tag)

	case mtSetBodyLen:
		addr := string(body[:addrWireLen])
		tagBytes := body[addrWireLen:mtSetBodyLen]
		if !validAddr(addr) {
			return rejection
		}
		if !validTag(tagBytes) {
			return rejection
		}
		tag := string(tagBytes)
		r.mu.Lock()
		// Creates an absent slot's tag (register entry 6); never touches
		// r.slots (register entry 5).
		r.tags[addr] = tag
		r.mu.Unlock()
		// The Set answers with an echo, unlike MW's silence — doc.go
		// register entry 7.
		return buildMTAnswer(addr, tag)
	}
	return rejection
}

// --- ID (spec.md §1) ---
//
// Read only. Read "ID;" (3 bytes), Answer "ID" + the 4-digit CATID + ';'
// (7 bytes). Always "0840" — no body option (doc.go).

func (r *Radio) handleID(body []byte) []byte {
	if len(body) != 0 {
		return rejection
	}
	out := make([]byte, 0, 2+4+1)
	out = append(out, 'I', 'D')
	out = append(out, catID...)
	out = append(out, ';')
	return out
}

// --- AI: AUTO INFORMATION ---
//
// Set frame (4 bytes) "AI" + P1 + ';', fire-and-forget; Read "AI;" (3
// bytes) answered by "AI" + P1 + ';' (4 bytes). P1 is '0' OFF / '1' ON.
//
// core/transport.Engine.Init opens every CAT session with an
// unconditional "AI0;", so this handler's silent-accept path is on the
// critical path of any future fake session — an unmodelled AI answering
// "?;" like any other unknown command would make Init's write draw a
// rejection and fail the open outright.
//
// THIS FAKE NEVER PUSHES AN UNSOLICITED FRAME, WHATEVER AI IS SET TO —
// doc.go's register entry AUTOMATIC-INFORMATION SUPPRESSION (spec.md §9's
// own AI=X column for MR/MT/MW/MC). "AI1;" is accepted, stored and read
// back faithfully, and nothing follows from it.

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

// upperASCII folds the two ASCII bytes of a command name to upper case
// and leaves every other byte alone. Not strings.ToUpper: this must be a
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
// MC, VM, GT AND EX ALL FALL THROUGH TO THE DEFAULT CASE, answering "?;"
// exactly like any other unknown command — none of them has a handler in
// this package (doc.go: MC IS NOT MODELLED AT ALL, and the plan's own
// scoping for VM/GT/EX).
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
	case [2]byte{'M', 'R'}:
		return r.handleMR(rest)
	case [2]byte{'M', 'W'}:
		return r.handleMW(rest)
	case [2]byte{'M', 'T'}:
		return r.handleMT(rest)
	default:
		return rejection
	}
}
