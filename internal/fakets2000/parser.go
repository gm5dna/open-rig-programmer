// SPDX-License-Identifier: GPL-3.0-or-later

package fakets2000

// This file is fakets2000's own, independent byte-level parser and reply
// builder for the TS-2000/TS-2000X/TS-B2000. It is derived from that
// document's own position charts in the PC CONTROL COMMAND TABLES appendix —
// cited line by line beside each section below as "ts2000:NNNN" — and NOT
// from core/kw, and NOT from any sibling fake. See doc.go for why that
// independence matters and for the full ASSUMED register; individual assumed
// points are flagged inline, next to the code that implements them.
//
// The manual itself is gitignored (docs/fixtures-private/manuals/), so the
// line references here name where the chart is, not a link to it.

import "strings"

// --- General framing ---

// rejection is the protocol's one and only NAK, "?;". The book's error table
// gives it two causes and says they are not distinguished — "Command syntax
// was incorrect." and "Command was not executed due to the current status of
// the transceiver (even though the command syntax was correct)."
// (ts2000:9603-9606) — so an empty slot, a malformed frame, an unknown
// command and an overflowed accumulator are all indistinguishable to the
// host.
var rejection = []byte("?;")

// StreamError is one of the two SERIAL-LINE error tokens this book prints
// beside "?;": "E;" for "A communication error occurred such as an overrun or
// framing error during a serial data transmission." (ts2000:9614-9616) and
// "O;" for "Receive data was sent but processing was not completed."
// (ts2000:9617-9618) — word for word the TS-480's sentence (480:143-144), not
// the TS-590's "receive buffer overrun" (590:113); doc.go's sibling section
// records the agreement as independent, not borrowed.
//
// The zero value is refused by WithStreamError: a scripted fault that
// defaulted to one of the two tokens would put a test on the wrong sentence.
type StreamError int

const (
	StreamErrorUnset StreamError = iota
	// StreamErrorE is "E;" (ts2000:9614-9616).
	StreamErrorE
	// StreamErrorO is "O;" (ts2000:9617-9618).
	StreamErrorO
)

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
// bounded-input policy, not a manual figure (doc.go's register entry 14).
const maxAccumulatorBytes = 256

// reassembler turns an arbitrary stream of Write() chunks into complete
// ';'-terminated frames. Framing only. The terminator is the book's own —
// "it is necessary to use a semicolon (;)" (ts2000:9588-9590).
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

// --- The channel number, and the 50-byte memory record ---
//
// THE SLOT SPACE THIS FAKE SERVES IS 000-299, THREE BANKS OF 100. MC's own
// P1 prints "_ (space): No bank number, 0 ~ 2: Memory bank number"
// (ts2000:10591-10592) and its P2 "00 ~ 99: Channel number" (ts2000:10595),
// with the mapping stated outright: "Memory channel numbers from 00 to 99 are
// treated as Memory bank 0. Memory channel numbers from 100 to 199 are
// treated as Memory bank 1. Memory channel numbers from 200 to 299 are
// treated as Memory bank 2." (ts2000:10597-10601). MR/MW's own P2/P3 cells
// are both glossed "Bank and channel number. See MC command."
// (ts2000:10694-10695, 10764-10765), so this fake reads the two the same way.
//
// CHANNELS 290-299 CARRY THE BOOK'S OWN P1 OVERLOAD, exactly as an ordinary
// TS-480/TS-590 channel does: "Memory channel 290 ~ 299: P1=0 (start
// frequency), P1=1 (end frequency)" (ts2000:10726-10727). They are ORDINARY
// MEMORIES that also answer a second frame, not a separate bank.

const (
	lowestChannel  = 0
	highestChannel = 299
)

// The 50-byte record's field offsets, 0-indexed into the whole frame, counted
// off MR's own position ruler (ts2000:10692-10727; MW's is the same shape,
// ts2000:10762-10774 continuing past the excerpt read for this package).
const (
	recLen = 50 // the ';' at position 50

	recP1Off          = 2 // position 3
	recP2Off          = 3 // position 4 (hundreds/bank digit)
	recP3Off          = 4 // positions 5-6
	recP3Len          = 2
	recFreqOff        = 6 // positions 7-17
	recFreqLen        = 11
	recModeOff        = 17 // position 18
	recLockoutOff     = 18 // position 19
	recToneModeOff    = 19 // position 20
	recToneNoOff      = 20 // positions 21-22
	recCTCSSNoOff     = 22 // positions 23-24
	recDCSOff         = 24 // positions 25-27
	recDCSLen         = 3
	recReverseOff     = 27 // position 28
	recShiftOff       = 28 // position 29
	recOffsetOff      = 29 // positions 30-38
	recOffsetLen      = 9
	recStepOff        = 38 // positions 39-40
	recStepLen        = 2
	recMemoryGroupOff = 40 // position 41
	recNameOff        = 41 // positions 42-49
	recNameLen        = 8
)

// mrReadLen is the read request's width, "M R P1 P2 P3 P3 ;" — seven
// positions (ts2000:10696-10701).
const mrReadLen = 7

// mcSetLen is the MC Set and Answer width, "M C P1 P2 P2 ;" — six positions
// (ts2000:10589-10601).
const mcSetLen = 6

// --- Field validators (wire level) ---
//
// EVERY ONE OF THEM IS ENFORCED ON THE SET DIRECTION, and every one is
// ASSUMED to be what the radio itself enforces — doc.go's register entry 2.

// validBankByte reports whether b is a legal spelling of the bank digit:
// space ("no bank number") or '0'-'2' (ts2000:10591-10592) — THREE bank
// digits on this row, where the TS-590 pair has two and the TS-480 has a
// fixed zero. Space is accepted on the SET direction and folded to bank 0 —
// doc.go's register entry 4, which is NOT the 590 pair's directional rule.
func validBankByte(b byte) bool { return b == ' ' || (b >= '0' && b <= '2') }

// validModeByte admits every ASCII digit. The MD legend runs 1-9 with 8 a
// printed hole and 0 undocumented (matrix §5); nothing says what a Set
// carrying 0 or 8 does, so this fake stores it rather than inventing a
// refusal — doc.go's register entry 9.
func validModeByte(b byte) bool { return isDigit(b) }

// validLockoutByte: "0: Lockout OFF, 1: Lockout ON." (ts2000:10700-10701).
func validLockoutByte(b byte) bool { return b == '0' || b == '1' }

// validToneModeByte: "0: OFF, 1: TONE, 2: CTCSS, 3: DCS."
// (ts2000:10702-10703, 10773-10774). FOUR values, all printed — unlike
// REVERSE below, this legend is complete and is enforced strictly.
func validToneModeByte(b byte) bool { return b >= '0' && b <= '3' }

// validReverseByte admits every ASCII digit. P11 prints no legend at all —
// "REVERSE status." (ts2000:10712) and nothing else — so this fake enforces
// only the one-byte width, never a value list this book does not print.
// doc.go's register entry 5.
func validReverseByte(b byte) bool { return isDigit(b) }

// validShiftByte: "0: Simplex / 1: + / 2: - / 3: = (All E-types)", the
// matrix's own citation of ts2000:10935-10938 (the OS command's legend, which
// P12 refers to). Four values, all printed.
func validShiftByte(b byte) bool { return b >= '0' && b <= '3' }

// validMemoryGroupByte: "Memory Group number (0 ~ 9)." (ts2000:10722-10723).
// The legend IS the whole ten-value range.
func validMemoryGroupByte(b byte) bool { return isDigit(b) }

// validNameField reports whether the eight bytes of P16 are ones a name may
// carry. The book says only "A maximum of 8 characters." (ts2000:10723-
// 10724); the printable-ASCII bound (0x20-0x7E) is this programme's own
// charset rule, the same assumption every sibling Kenwood fake applies to its
// own name field, not a statement about this radio.
func validNameField(field []byte) bool {
	for _, b := range field {
		if b < 0x20 || b > 0x7e {
			return false
		}
	}
	return true
}

// parseChannelDigits decodes the bank byte and the two-digit field into a
// channel number, and reports whether the spelling and the number are both
// admissible. A space bank byte is bank 0 ("no bank number",
// ts2000:10591-10592).
func parseChannelDigits(bank byte, twoDigits string) (int, bool) {
	if !validBankByte(bank) || len(twoDigits) != recP3Len || !allDigits(twoDigits) {
		return 0, false
	}
	b := 0
	if bank != ' ' {
		b = int(bank - '0')
	}
	n := b*100 + int(twoDigits[0]-'0')*10 + int(twoDigits[1]-'0')
	if n < lowestChannel || n > highestChannel {
		return 0, false
	}
	return n, true
}

// bankAnswerByte renders a channel number's bank digit for an ANSWER: always
// a concrete digit, never a space — doc.go's register entry 4. This book
// states no Set-versus-Answer asymmetry for this byte, unlike the 590 pair's
// explicit "always answer a space below channel 100" rule, so nothing here
// invents one.
func bankAnswerByte(channel int) byte { return byte('0' + channel/100) }

// --- MR: the memory read (ts2000:10689-10727) ---
//
// X O O X: no Set — the chart prints a Read and an Answer row and nothing
// else — a Read (ts2000:10696-10701), a 50-byte Answer
// (ts2000:10692-10727), no AI push.

// buildMRAnswer builds the 50-byte answer for one channel half.
func buildMRAnswer(channel int, half Half, s MemState) []byte {
	out := make([]byte, 0, recLen)
	out = append(out, 'M', 'R')
	out = append(out, byte(half))
	out = append(out, bankAnswerByte(channel))
	out = append(out, byte('0'+(channel/10)%10), byte('0'+channel%10))
	out = append(out, s.Freq...)
	out = append(out, s.Mode, s.Lockout, s.ToneMode)
	out = append(out, s.ToneNo...)
	out = append(out, s.CTCSSNo...)
	out = append(out, s.DCSCode...)
	out = append(out, s.Reverse)
	out = append(out, s.Shift)
	out = append(out, s.Offset...)
	out = append(out, s.Step...)
	out = append(out, s.MemoryGroup)
	out = append(out, s.Name...)
	out = append(out, ';')
	return out
}

func (r *Radio) handleMR(body []byte) []byte {
	if len(body) != mrReadLen-3 {
		return rejection
	}
	half := Half(body[0])
	if !half.valid() {
		return rejection
	}
	channel, ok := parseChannelDigits(body[1], string(body[2:4]))
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
		// THIS FAKE INVENTS THIS ANSWER — doc.go's register entry 3. The
		// matrix finds no sentence in this document describing what an
		// empty channel's P4-P16 hold (matrix §5, §6 item 5), a weaker
		// position than either sibling's. The zero/blank shape is offered
		// only so the protocol has something to say; WithChannel/
		// WithFactoryImage/WithEmptyChannel let a test override it entirely.
		s = emptyRecord()
	}
	return buildMRAnswer(channel, half, s)
}

// --- MW: the memory write (ts2000:10761-10774+) ---
//
// O X X X: a 50-byte Set and nothing else — so an accepted write is silent
// (doc.go's register entry 1).
//
// EVERY ONE OF THE SIXTEEN FIELDS IS VALIDATED AS DATA, NEVER AS A REQUIRED
// CONSTANT — matrix §2's finding that this row's grid has no printed-fixed
// byte at all, unlike the TS-480/TS-590 pair.

func (r *Radio) handleMW(body []byte) []byte {
	if len(body) != recLen-3 {
		return rejection
	}
	frame := append(append([]byte{'M', 'W'}, body...), ';')

	half := Half(frame[recP1Off])
	if !half.valid() {
		return rejection
	}
	channel, ok := parseChannelDigits(frame[recP2Off], string(frame[recP3Off:recP3Off+recP3Len]))
	if !ok {
		return rejection
	}

	s := MemState{
		Freq:        string(frame[recFreqOff : recFreqOff+recFreqLen]),
		Mode:        frame[recModeOff],
		Lockout:     frame[recLockoutOff],
		ToneMode:    frame[recToneModeOff],
		ToneNo:      string(frame[recToneNoOff : recToneNoOff+2]),
		CTCSSNo:     string(frame[recCTCSSNoOff : recCTCSSNoOff+2]),
		DCSCode:     string(frame[recDCSOff : recDCSOff+recDCSLen]),
		Reverse:     frame[recReverseOff],
		Shift:       frame[recShiftOff],
		Offset:      string(frame[recOffsetOff : recOffsetOff+recOffsetLen]),
		Step:        string(frame[recStepOff : recStepOff+recStepLen]),
		MemoryGroup: frame[recMemoryGroupOff],
		Name:        string(frame[recNameOff : recNameOff+recNameLen]),
	}

	switch {
	case !allDigits(s.Freq):
		return rejection
	case !validModeByte(s.Mode):
		return rejection
	case !validLockoutByte(s.Lockout):
		return rejection
	case !validToneModeByte(s.ToneMode):
		return rejection
	case !allDigits(s.ToneNo) || !allDigits(s.CTCSSNo):
		// STORED, NOT RANGE-CHECKED — doc.go's register entry 7.
		return rejection
	case !allDigits(s.DCSCode):
		// STORED, NOT RANGE-CHECKED — doc.go's register entry 6.
		return rejection
	case !validReverseByte(s.Reverse):
		// doc.go's register entry 5.
		return rejection
	case !validShiftByte(s.Shift):
		return rejection
	case !allDigits(s.Offset):
		// STORED, NOT RANGE-CHECKED — doc.go's register entry 6.
		return rejection
	case !allDigits(s.Step):
		// STORED, NOT RANGE-CHECKED — doc.go's register entry 8.
		return rejection
	case !validMemoryGroupByte(s.MemoryGroup):
		return rejection
	case !validNameField(frame[recNameOff : recNameOff+recNameLen]):
		return rejection
	}

	r.mu.Lock()
	r.records[recordKey{channel: channel, half: half}] = s
	r.mu.Unlock()
	// A Set does not move the selection — doc.go's register entry 10.
	return nil // fire-and-forget success
}

// --- MC: the selected memory channel (ts2000:10589-10601) ---
//
// O O O X. Set and Answer are six bytes, Read is "MC;". Disambiguated by
// length.
//
// THIS FAKE MODELS NO FRONT PANEL, so the selection moves only by an MC Set.

func buildMCAnswer(channel int) []byte {
	out := make([]byte, 0, mcSetLen)
	out = append(out, 'M', 'C')
	out = append(out, bankAnswerByte(channel))
	out = append(out, byte('0'+(channel/10)%10), byte('0'+channel%10))
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

	case mcSetLen - 3:
		channel, ok := parseChannelDigits(body[0], string(body[1:3]))
		if !ok {
			return rejection
		}
		r.mu.Lock()
		r.currentChannel = channel
		r.mu.Unlock()
		return nil // fire-and-forget success
	}
	return rejection
}

// --- ID: the transceiver ID number (ts2000:10429-10441) ---
//
// X O O X: no Set, Read "ID;", a six-byte Answer "I D P1 P1 P1 ;". The value
// is "019", the only one this document prints ("019: TS-2000",
// ts2000:10431) — SHARED BY ALL THREE ROWS regardless of Model(), doc.go's
// register entry 16.

const idAnswer = "ID019;"

func (r *Radio) handleID(body []byte) []byte {
	if len(body) != 0 {
		return rejection
	}
	return []byte(idAnswer)
}

// --- TY: the microprocessor firmware type, which is NOT a Set on this row
// (ts2000:11678-11693) ---
//
// X O O X: the heading reads "Sets or reads the microprocessor fimware type."
// (ts2000:11678, the manual's own "fimware" typo — the same slip the TS-480's
// book makes, its own erratum E10) but the Set row's chart prints no content
// at all under it (ts2000:11682, ruler only), exactly the TS-480's own
// empty-Set-chart shape (480:1621, 1625); the chart wins, so this fake
// refuses a TY Set — TestTY_HasNoSetDirection.
//
// core/driver/ts2000's own Open sends "TY;" unconditionally after "ID;"
// (its own doc comment: "no radio has ever answered this probe") — this is
// the wire-level fact `reviews/registration.md` found missing here, blocking
// every OpenFakeSessionFor for this package.
//
// P1 IS TWO RESERVED BYTES (ts2000:11680, "Reserved", no legend at all) and
// P2 IS THE VARIANT: "0: Overseas type / 1: Japanese 100 W type / 2: Japanese
// 20 W type" (ts2000:11683-11685) — the driver's own report cites this same
// range independently (its "book choice" note) for an unrelated reason
// (picking Book480 for `Layout`), which is expected: both readings are of
// one chart, not one borrowed from the other.

// tyReservedLen is P1's width, counted off the answer row's position ruler
// (ts2000:11691): two bytes.
const tyReservedLen = 2

// The shipped TY answer — doc.go's register entry 17. NEITHER BYTE IS A
// CLAIM ABOUT ANY RADIO: no TS-2000/2000X/B2000 has answered this project.
// P1 has no legend to borrow a "hard-wired" convention from (this row has NO
// printed-fixed byte anywhere, matrix §2), so its two bytes are simply "00",
// an invented placeholder; P2 takes the FIRST of the three printed variants,
// "0: Overseas type" (ts2000:11683). Both are shared by all three rows —
// doc.go's register entry 16, and the coordinator's own instruction for this
// package: if the manual gives no separate TY answer for the X/B2000 rows,
// answer the TS-2000 one and record it ASSUMED.
const (
	defaultTYReserved = "00"
	defaultTYVariant  = '0'
)

func (r *Radio) handleTY(body []byte) []byte {
	if len(body) != 0 {
		return rejection
	}
	out := make([]byte, 0, 2+tyReservedLen+1+1)
	out = append(out, 'T', 'Y')
	out = append(out, defaultTYReserved...)
	out = append(out, defaultTYVariant, ';')
	return out
}

// --- AI: Auto Information (ts2000:9662-9675) ---
//
// O O O O. Set and Answer are four bytes, Read is "AI;". An AI Set is
// fire-and-forget.
//
// "0: AI OFF / 1: Only old AI format is ON / 2: Only extended AI format is
// ON / 3: Both formats are ON" (ts2000:9664-9668) — the TS-480's own four
// consecutive values (480:185-190), not the TS-590's 0/2/4.
//
// THIS FAKE NEVER PUSHES ANYTHING UNSOLICITED — doc.go's register entry 13.

const (
	aiOff         = '0'
	aiBothFormats = '3'
)

func validAIByte(b byte) bool { return b >= aiOff && b <= aiBothFormats }

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
// leaves every other byte alone — not bytes.ToUpper/strings.ToUpper, which
// are Unicode-aware and can change a byte string's length.
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
// COMMAND NAMES ARE MATCHED IN EITHER CASE: "A command consists of 2
// alphabetical characters. You may use either lower or upper case
// characters." (ts2000:9618-9620). Field values remain case-sensitive — the
// sentence is about the command name.
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
	case [2]byte{'T', 'Y'}:
		return r.handleTY(rest)
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
