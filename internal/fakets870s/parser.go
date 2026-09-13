// SPDX-License-Identifier: GPL-3.0-or-later

package fakets870s

// This file is fakets870s's own, independent byte-level parser and reply
// builder for the TS-870S. It is derived from the community mirror of
// Kenwood document B62-1536-00 (docs/fixtures-private/manuals) — cited line
// by line as "ts870s:NNNN" — and NOT from core/kw, and NOT from any sibling
// fake. See doc.go for why that independence matters and for the full
// ASSUMED register.

import "strings"

// --- General framing ---

// rejection is the protocol's one and only NAK, "?;". This book's error
// table gives it two causes: "Command syntax was incorrect." and "Command
// was not executed due to the current status of the transceiver (even
// though the command syntax was correct)." (ts870s:8434-8438), with a note
// that it may occasionally not appear at all (ts870s:8440-8442, played by
// WithTransientNAKSuppressed rather than assumed away).
var rejection = []byte("?;")

// maxAccumulatorBytes is the reassembler's byte cap — this package's own
// bounded-input policy, not a manual figure. THE FRAME ACCUMULATOR'S CAP AND
// RESYNC is doc.go's register entry for this whole reassembler.
const maxAccumulatorBytes = 256

// reassembler turns an arbitrary stream of Write() chunks into complete
// ';'-terminated frames. Framing only. The terminator is the book's own —
// "To signal the end of a command, it is necessary to use a semicolon (;)."
// (ts870s:8383-8384).
//
// Overflow behaviour: once more than maxAccumulatorBytes bytes have
// accumulated without completing a frame, push reports one overflow event —
// the caller replies "?;" for it — and discards every byte up to and
// including the next ';', then resumes normal framing. The zero value is not
// usable; construct with newReassembler.
type reassembler struct {
	buf       []byte
	max       int
	resyncing bool
}

func newReassembler() *reassembler {
	return &reassembler{max: maxAccumulatorBytes}
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

// allDigits reports whether s is a non-empty run of ASCII digits.
func allDigits(s string) bool {
	return s != "" && strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

// --- The channel number, and the 22-byte memory record ---
//
// THE SLOT SPACE IS THE FLAT 00-99. P3 is "00 ~ 99" (Format 7,
// ts870s:8267-8268) and there is no byte before it to hold a bank number or a
// hundreds digit — unlike the 480/590 family, whose byte 4 is either a
// printed-constant bank byte or a hundreds digit. A channel number here needs
// no bound check beyond its own two digits: two ASCII digits cannot spell a
// number outside 00-99.

// The record's field offsets, 0-indexed into the whole frame, counted off
// the Read/Set/Answer diagrams (ts870s:9097, 9112-9113, 9139-9142).
//
// P2 AND P9 OCCUPY NO POSITION AT ALL — see doc.go and state.go — so P3
// starts immediately after P1, and the frame ends immediately after P8.
const (
	recLen = 22 // the ';' at position 22

	recP1Off       = 2 // position 3
	recP3Off       = 3 // positions 4-5
	recP3Len       = 2
	recFreqOff     = 5 // positions 6-16
	recFreqLen     = 11
	recModeOff     = 16 // position 17
	recLockoutOff  = 17 // position 18
	recToneModeOff = 18 // position 19
	recToneNoOff   = 19 // positions 20-21
	recToneNoLen   = 2
)

// zeroFreq is the all-zero-digits frequency that marks a Set as an erase to
// vacant — doc.go's register entry A WRITE WITH ALL FREQUENCY DIGITS ZERO
// MARKS THE CHANNEL VACANT — and is also what a zero/vacant channel's Answer
// reports for P4 (ts870s:9101-9104).
const zeroFreq = "00000000000"

// mrReadLen is the Read frame's width, "M R P1 P3 P3 ;" (ts870s:9097) — six
// positions.
const mrReadLen = 6

// --- Field validators (wire level) ---
//
// EVERY ONE OF THEM IS ENFORCED ON THE SET DIRECTION for a write that is NOT
// the all-zero-frequency erase — doc.go's register entry SET-DIRECTION FIELD
// STRICTNESS ON EVERY OTHER WRITE — and is ASSUMED to be what the radio
// itself enforces.

// validModeByte reports whether b is a nibble Format 2's legend prints
// (ts870s:8256-8262). ALL TEN ARE ADMITTED, including the two holes "No
// mode" (0) and "No Mode" (8) — the SET-DIRECTION FIELD STRICTNESS ON EVERY
// OTHER WRITE register entry, and internal/fakets480's identical posture for
// its own mode nibble.
func validModeByte(b byte) bool { return isDigit(b) }

// validLockoutByte: "0: Not locked out, 1: Locked out" (Format 10).
func validLockoutByte(b byte) bool { return b == '0' || b == '1' }

// validToneModeByte: "0: OFF, 1: ON" (Format 1, ts870s:8256) — TWO values,
// where the 480/590 pair print three or four.
func validToneModeByte(b byte) bool { return b == '0' || b == '1' }

// parseChannelDigits decodes the two-digit channel field. Any two ASCII
// digits spell a channel this fake serves — there is no bank byte and no
// hundreds digit to bound separately (see above).
func parseChannelDigits(twoDigits string) (int, bool) {
	if len(twoDigits) != recP3Len || !allDigits(twoDigits) {
		return 0, false
	}
	return int(twoDigits[0]-'0')*10 + int(twoDigits[1]-'0'), true
}

// --- MR: the memory read (ts870s:9067-9106) ---
//
// X O O X: no Set direction is printed for MR — the diagram jumps straight
// from the Function line to a Read row (ts870s:9097) — a Read
// (ts870s:9097), a 22-byte Answer (ts870s:9112-9113), no unsolicited push.

// buildMRAnswer builds the 22-byte answer for one channel half.
func buildMRAnswer(channel int, half Half, s MemState) []byte {
	out := make([]byte, 0, recLen)
	out = append(out, 'M', 'R')
	out = append(out, byte(half))
	out = append(out, byte('0'+(channel/10)%10), byte('0'+channel%10))
	out = append(out, s.Freq...)
	out = append(out, s.Mode, s.Lockout, s.ToneMode)
	out = append(out, s.ToneNo...)
	out = append(out, ';')
	return out
}

// emptyRecord is the zero shape a vacant or unwritten channel's Answer
// carries — DOCUMENTARY on the read side: "For a vacant channel, the Answer
// command sends '0' for all parameters except the memory channel number."
// (ts870s:9101-9104). doc.go's register entry AN UNWRITTEN OR VACANT
// CHANNEL'S EITHER HALF ANSWERS THE ZERO RECORD covers extending it to the
// TX/End half too.
func emptyRecord() MemState {
	return MemState{
		Freq:     zeroFreq,
		Mode:     '0',
		Lockout:  '0',
		ToneMode: '0',
		ToneNo:   "00",
	}
}

func (r *Radio) handleMR(body []byte) []byte {
	if len(body) != mrReadLen-3 {
		return rejection
	}
	half := Half(body[0])
	if !half.valid() {
		return rejection
	}
	channel, ok := parseChannelDigits(string(body[1:3]))
	if !ok {
		return rejection
	}
	if r.memoryReadUnsupported {
		return rejection
	}

	r.mu.Lock()
	s, ok := r.records[recordKey{channel: channel, half: half}]
	r.mu.Unlock()
	if !ok {
		s = emptyRecord()
	}
	return buildMRAnswer(channel, half, s)
}

// --- MW: the memory write (ts870s:9139-9163) ---
//
// O X X X: a 22-byte Set and nothing else — no Read or Answer row is printed
// for MW — so an accepted write is silent (doc.go's register entry 1).

func (r *Radio) handleMW(body []byte) []byte {
	if len(body) != recLen-3 {
		return rejection
	}
	// Re-index against the whole frame's rulers so every offset constant
	// reads as the chart's own position.
	frame := append(append([]byte{'M', 'W'}, body...), ';')

	half := Half(frame[recP1Off])
	if !half.valid() {
		return rejection
	}
	channel, ok := parseChannelDigits(string(frame[recP3Off : recP3Off+recP3Len]))
	if !ok {
		return rejection
	}
	freq := string(frame[recFreqOff : recFreqOff+recFreqLen])
	if !allDigits(freq) {
		return rejection
	}

	key := recordKey{channel: channel, half: half}

	// THE ALL-ZERO-FREQUENCY ERASE (doc.go's register entry 2): "The memory
	// channel becomes a vacant channel if all frequency digits are '0'. ...
	// Other parameters are ignored." (ts870s:9155-9163). No other field is
	// validated or stored on this branch.
	if freq == zeroFreq {
		r.mu.Lock()
		delete(r.records, key)
		r.mu.Unlock()
		return nil
	}

	s := MemState{
		Freq:     freq,
		Mode:     frame[recModeOff],
		Lockout:  frame[recLockoutOff],
		ToneMode: frame[recToneModeOff],
		ToneNo:   string(frame[recToneNoOff : recToneNoOff+recToneNoLen]),
	}

	switch {
	case !validModeByte(s.Mode):
		return rejection
	case !validLockoutByte(s.Lockout):
		return rejection
	case !validToneModeByte(s.ToneMode):
		return rejection
	case !allDigits(s.ToneNo):
		// THE TONE NUMBER IS STORED, NOT RANGE-CHECKED against Format 14's
		// "01~39" — doc.go's register entry of that name.
		return rejection
	}

	r.mu.Lock()
	r.records[key] = s
	r.mu.Unlock()
	return nil // fire-and-forget success
}

// --- ID: the identity a probe turns into a wrong-radio refusal
// (ts870s:8986-9009) ---
//
// X O O X: no Set (the diagram's own "Set" heading has no ID content under
// it — the ruler beneath it belongs to the neighbouring KY block), Read
// "ID;" (ts870s:8998), a six-byte Answer "I D P1 P1 P1 ;" (ts870s:9009).

// idAnswer is the whole of this radio's identity answer: "015" is Format
// 16's own printed value, "The TS-870S number is 015." (ts870s:8300-8302).
const idAnswer = "ID015;"

func (r *Radio) handleID(body []byte) []byte {
	if len(body) != 0 {
		return rejection
	}
	return []byte(idAnswer)
}

// --- AI: Auto Information (ts870s:8612-8639) ---
//
// O O O O. Set "A I P1 ;" (ts870s:8625), Read "A I ;" (ts870s:8631), Answer
// "A I P1 ;" (ts870s:8639) — all four bytes. An AI Set is fire-and-forget,
// matching every other accepted Set in this package.
//
// AI NEVER PUSHES ANYTHING UNSOLICITED, whatever it is set to — doc.go's
// register entry of that name. Format 32's own legend ties AI's non-zero
// values to automatic Answer pushes this fake does not model.

// The AI legend (Format 32, "AI NUMBER"): "0: AI OFF", "1: IF command
// outputs its Answer command periodically.", "2: For parameter changes, the
// corresponding Answer command is output." — THREE consecutive values.
const (
	aiOff          = '0'
	aiHighestValue = '2'
)

func validAIByte(b byte) bool { return b >= aiOff && b <= aiHighestValue }

func (r *Radio) handleAI(body []byte) []byte {
	switch len(body) {
	case 0:
		r.mu.Lock()
		ai := r.ai
		r.mu.Unlock()
		return []byte{'A', 'I', ai, ';'}
	case 1:
		if !validAIByte(body[0]) {
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
// leaves every other byte alone — see internal/fakets480's identical helper
// for why this is not bytes.ToUpper/strings.ToUpper.
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
// success, or a non-nil frame otherwise. Unknown and garbled commands fall
// through to rejection.
//
// COMMAND NAMES ARE MATCHED IN EITHER CASE: "A command is composed of 2
// alphabetical characters" (ts870s:8195) and "A command may consist of
// either lower or upper case alphabetical characters." (ts870s:8214).
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
	default:
		return rejection
	}
}
