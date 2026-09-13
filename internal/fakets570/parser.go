// SPDX-License-Identifier: GPL-3.0-or-later

package fakets570

// This file is fakets570's own, independent byte-level parser and reply
// builder for the TS-570 family. It is derived from
// docs/superpowers/ts570-capability-matrix.md (itself the TS-570 Instruction
// Manual's own "COMPUTER CONTROL COMMAND TABLES" appendix, and the manual's
// general command-grammar pages, cited "PDF p.N (printed M)" or "printed
// folio N") — NOT from core/kw, NOT from core/kw/ts570 and NOT from any
// sibling fake. See doc.go for the full ASSUMED register; individual assumed
// points are flagged inline, beside the code that implements them.

import "strings"

// rejection is the protocol's one and only NAK, "?;" — printed directly in
// this radio's own manual (printed folio 70): "Command syntax was
// incorrect." / "Command was not executed due to the current status of the
// transceiver (even though the command syntax was correct)."
var rejection = []byte("?;")

// StreamError is one of the two SERIAL-LINE error tokens this radio's own
// manual prints beside "?;" (PDF p.76, printed folio 70, layout lines
// 5158-5175): "E;" for "A communication error occurred such as an overrun or
// framing error during a serial data transmission" and "O;" for "Receive
// data was sent but processing was not completed." — the identical two
// sentences the TS-480/590 pair's own books print (core/kw's lift-K
// follow-up, commit e7515d0, cites this same manual span as
// "ts570:5158-5175").
//
// These are not command outcomes: they REPLACE whatever an exchange would
// otherwise have produced, including a fire-and-forget silence, exactly as
// the sibling Kenwood fakes' own StreamError does — see Radio.handleEvent.
//
// The zero value is refused by WithStreamError: a scripted fault that
// defaulted to one of the two tokens would put a test on the wrong sentence.
type StreamError int

// The two stream-error tokens, plus the refusing default.
const (
	StreamErrorUnset StreamError = iota
	// StreamErrorE is "E;" (printed folio 70).
	StreamErrorE
	// StreamErrorO is "O;" (printed folio 70).
	StreamErrorO
)

// token renders s as the bytes the radio would put on the wire.
func (s StreamError) token() string {
	switch s {
	case StreamErrorE:
		return "E;"
	case StreamErrorO:
		return "O;"
	default:
		return ""
	}
}

// String renders s for panics and test failures.
func (s StreamError) String() string {
	switch s {
	case StreamErrorE:
		return "StreamErrorE"
	case StreamErrorO:
		return "StreamErrorO"
	default:
		return "StreamErrorUnset"
	}
}

// maxAccumulatorBytes is the reassembler's byte cap — this package's own
// bounded-input policy, not a manual figure, matching the sibling Kenwood
// fakes' own choice.
const maxAccumulatorBytes = 256

// reassembler turns an arbitrary stream of Write() chunks into complete
// ';'-terminated frames. Framing only: it says nothing about what any frame
// means. The terminator is the manual's own — "To signal the end of a
// command, it is necessary to use a semicolon (;)" (printed folio 70).
type reassembler struct {
	buf       []byte
	max       int
	resyncing bool
}

func newReassembler() *reassembler { return &reassembler{max: maxAccumulatorBytes} }

// accEvent is one unit reassembler.push hands back: either a complete frame
// (terminator included) or an overflow signal (frame == nil, overflow true).
type accEvent struct {
	frame    []byte
	overflow bool
}

// push appends chunk to the internal buffer, byte by byte, and returns, in
// arrival order, every complete frame and overflow event it produced. Once
// more than max bytes have accumulated without completing a frame, push
// reports one overflow event and discards every byte up to and including the
// next ';', then resumes normal framing.
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

// allDigits reports whether s is a non-empty run of ASCII digits.
func allDigits(s string) bool {
	return s != "" && strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

// The served slot space's bounds — P3's own printed range, "00~99" (matrix
// §1.2, format code 7).
const (
	lowestChannel  = 0
	highestChannel = 99
)

// Frame body lengths, EXCLUDING the two-letter command name and the
// terminator — matrix §1.2's byte offset table.
const (
	mrReadBodyLen = 1 + 1 + 2                          // P1, P2, P3
	mwBodyLen     = 1 + 1 + 2 + 11 + 1 + 1 + 1 + 2 + 5 // P1..P9
)

// validFillerByte reports whether b is admissible in P2 or P9, the two
// unused positions (matrix §1.2, register entry 3): the manual's general
// Set-direction rule for an inapplicable parameter, "any character except
// the ASCII control codes (00 to 1Fh) and the terminator (;)" (printed folio
// 70). The upper bound at 0x7E is this package's own choice — the manual
// states no ceiling for a filler byte — matching every other printable-ASCII
// field in this family.
func validFillerByte(b byte) bool { return b >= 0x20 && b <= 0x7e && b != ';' }

// validModeByte reports whether b is one of the mode legend's ten printed
// nibbles, '0'-'9' (matrix §1.3) — including both "No selection" values.
func validModeByte(b byte) bool { return isDigit(b) }

// validLockoutByte: "0: Not locked out / 1: Locked out" (matrix §1.2, format
// code 10).
func validLockoutByte(b byte) bool { return b == '0' || b == '1' }

// validToneModeByte: "0: OFF / 1: ON" (matrix §1.2, format code 1) — TWO
// values, narrower than the 590 pair's four or the 480's three.
func validToneModeByte(b byte) bool { return b == '0' || b == '1' }

// parseChannel decodes P3's two digits into a channel number. There is no
// bank byte on this row — P2 carries no channel role at all (matrix §1.2) —
// so, unlike the sibling Kenwood fakes, this parser never rejects a channel
// on P2's account.
func parseChannel(twoDigits string) (int, bool) {
	if len(twoDigits) != 2 || !allDigits(twoDigits) {
		return 0, false
	}
	n := int(twoDigits[0]-'0')*10 + int(twoDigits[1]-'0')
	if n < lowestChannel || n > highestChannel {
		return 0, false
	}
	return n, true
}

// --- MR: the memory read (matrix §1.1, §1.2) ---

// buildMRAnswer builds the 28-byte answer for one channel half.
func buildMRAnswer(channel int, half Half, s MemState) []byte {
	out := make([]byte, 0, 28)
	out = append(out, 'M', 'R')
	out = append(out, byte(half))
	out = append(out, s.P2)
	out = append(out, byte('0'+(channel/10)%10), byte('0'+channel%10))
	out = append(out, s.Freq...)
	out = append(out, s.Mode, s.Lockout, s.ToneMode)
	out = append(out, s.ToneNo...)
	out = append(out, s.P9...)
	out = append(out, ';')
	return out
}

func (r *Radio) handleMR(body []byte) []byte {
	if len(body) != mrReadBodyLen {
		return rejection
	}
	half := Half(body[0])
	if !half.valid() || !validFillerByte(body[1]) {
		return rejection
	}
	channel, ok := parseChannel(string(body[2:4]))
	if !ok {
		return rejection
	}

	r.mu.Lock()
	s, ok := r.records[recordKey{channel: channel, half: half}]
	r.mu.Unlock()
	if !ok {
		// doc.go register entry 2: an unwritten channel, either half,
		// answers the zero record.
		s = zeroRecord()
	}
	return buildMRAnswer(channel, half, s)
}

// --- MW: the memory write (matrix §1.1, §1.2) ---
//
// THE MANUAL'S OWN ERASE ROUTE: "The memory channel becomes a vacant channel
// if all frequency digits are '0'" — an MW whose P4 is eleven '0' digits
// vacates the addressed half rather than storing a zero record, so a
// subsequent MR of it answers the zero record via the empty-channel path
// (register entry 2), not via a stored all-zero one. This is
// MANUAL-EVIDENCED, not an assumption of this fake's own.
const allZeroFreq = "00000000000"

func (r *Radio) handleMW(body []byte) []byte {
	if len(body) != mwBodyLen {
		return rejection
	}

	half := Half(body[0])
	if !half.valid() {
		return rejection
	}
	if !validFillerByte(body[1]) {
		return rejection
	}
	channel, ok := parseChannel(string(body[2:4]))
	if !ok {
		return rejection
	}

	freq := string(body[4:15])
	mode := body[15]
	lockout := body[16]
	toneMode := body[17]
	toneNo := string(body[18:20])
	p9 := string(body[20:25])

	switch {
	case !allDigits(freq):
		return rejection
	case !validModeByte(mode):
		return rejection
	case !validLockoutByte(lockout):
		return rejection
	case !validToneModeByte(toneMode):
		return rejection
	case !allDigits(toneNo):
		// doc.go register entry 4: the shape is enforced, the printed
		// "01~39" range is not.
		return rejection
	}
	for _, b := range []byte(p9) {
		if !validFillerByte(b) {
			return rejection
		}
	}

	key := recordKey{channel: channel, half: half}
	r.mu.Lock()
	if freq == allZeroFreq {
		delete(r.records, key)
	} else {
		r.records[key] = MemState{
			Freq: freq, P2: body[1], Mode: mode, Lockout: lockout,
			ToneMode: toneMode, ToneNo: toneNo, P9: p9,
		}
	}
	r.mu.Unlock()
	return nil // fire-and-forget success
}

// --- ID: the identity, and the row it names (matrix §2) ---
//
// X O O X: no Set — doc.go register entry 5 — Read "ID;", a six-byte Answer
// "I D P1 P1 P1 ;" carrying this row's CATID (matrix §2; PDF p.83, printed
// 77).

func (r *Radio) handleID(body []byte) []byte {
	if len(body) != 0 {
		return rejection
	}
	return []byte("ID" + r.catID + ";")
}

// --- Top-level dispatch ---

func (r *Radio) handleFrame(frame []byte) []byte {
	if len(frame) == 0 || frame[len(frame)-1] != ';' {
		return rejection // defensive: the reassembler never hands us this
	}
	body := frame[:len(frame)-1]
	if len(body) < 2 {
		return rejection
	}
	cmd := [2]byte{body[0], body[1]}
	rest := body[2:]

	switch cmd {
	case [2]byte{'M', 'R'}:
		return r.handleMR(rest)
	case [2]byte{'M', 'W'}:
		return r.handleMW(rest)
	case [2]byte{'I', 'D'}:
		return r.handleID(rest)
	default:
		return rejection
	}
}
