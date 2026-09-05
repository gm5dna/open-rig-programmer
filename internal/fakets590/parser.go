// SPDX-License-Identifier: GPL-3.0-or-later

package fakets590

// This file is fakets590's own, independent byte-level parser and reply
// builder for the TS-590S and the TS-590SG. It is derived from those radios'
// own position charts in the TS-590S/TS-590SG PC Control Command Reference
// Guide (rev 3) — cited line by line beside each section below as
// "590:NNNN" — and NOT from core/kw. See doc.go for why that independence
// matters and for the full ASSUMED register; individual assumed points are
// flagged inline, next to the code that implements them.
//
// The manual itself is gitignored (docs/fixtures-private/manuals/), so the
// line references here are citations in the sense core/kw/doc.go uses them:
// they name where the chart is, they are not links.

// --- General framing ---

// rejection is the protocol's one and only NAK, "?;". The book's error table
// gives it two causes and says they are not distinguished — "Command syntax
// was incorrect" and "Command was not executed due to the current status of
// the transceiver (even though the command syntax was correct)"
// (590:100-105) — so an empty slot, a malformed frame, an unknown command
// and an overflowed accumulator are all indistinguishable to the host, which
// is the whole of the convention.
//
// UNLIKE THE YAESU FAMILY'S, THIS CONVENTION IS PRINTED IN THIS RADIO'S OWN
// BOOK (590:93-113) rather than inherited from a sibling's reference.
var rejection = []byte("?;")

// StreamError is one of the two SERIAL-LINE error tokens this book prints
// beside "?;", which are not command outcomes at all: "E;" for "A
// communication error occurred, such as an overrun or framing error during a
// serial data transmission" (590:110-112) and "O;" for "A receive buffer
// overrun error occurred" (590:113).
//
// The zero value is refused by WithStreamError: a scripted fault that
// defaulted to one of the two tokens would put a test on the wrong sentence.
type StreamError int

// The two stream-error tokens, plus the refusing default.
const (
	StreamErrorUnset StreamError = iota
	// StreamErrorE is "E;" (590:110-112).
	StreamErrorE
	// StreamErrorO is "O;" (590:113). Erratum E13 records that the TS-480's
	// book gives this same token a DIFFERENT cause, which is why no Kenwood
	// fake may borrow the other's error vocabulary.
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

// String renders s for refusals and test failures.
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
// bounded-input policy, not a manual figure (doc.go's register entry THE
// FRAME ACCUMULATOR'S CAP AND RESYNC).
const maxAccumulatorBytes = 256

// reassembler turns an arbitrary stream of Write() chunks into complete
// ';'-terminated frames. Framing only: it says nothing about what any frame
// means. The terminator is the book's own — "To signal the end of a command,
// it is necessary to use a semicolon (;)" (590:87-91).
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

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return len(s) > 0
}

// --- The channel number, and the 50-byte memory record ---
//
// THE SLOT SPACE THIS FAKE SERVES IS 000-109 ON BOTH ROWS:
//
//   - 000-099, ordinary memory channels. MC's P2 prints "00 ~ 99: Two digit
//     channel number" and its P1 the hundreds digit (590:1332-1343), and
//     both memory charts refer their P2/P3 cells to MC (590:1452-1453,
//     590:1539-1540).
//   - 100-109, the section-defined channels: "Channel numbers P00 ~ P09 are
//     represented by 100 ~ 109." (590:1345).
//
// THE TS-590SG'S 110-119 ARE PRINTED AND ARE DELIBERATELY NOT SERVED.
// "TS-590SG extension channel numbers E00 ~ P09 are represented by 110 ~
// 119." (590:1346-1347 — the second name is a printed typo for E09). What an
// extension channel IS is never explained anywhere in the book, and Stuart
// ruled on 05/09/2026 that the ten slots are omitted from the driver's
// published banks until that is lifted. There is no slot ID left for this
// fake to serve, so 110-119 is refused here exactly as any other
// out-of-domain number is — one refusal path, no special case — and BOTH ROWS
// serve the same space. doc.go's register entry THE SG'S EXTENSION CHANNELS
// ARE NOT SERVED records that this is a deliberate narrowing of the printed
// domain and NOT a claim that either radio refuses those numbers.

// The served slot space's bounds.
//
// highestChannel is 109 on BOTH rows, but for two different reasons. On the
// SG it is entry 10's deliberate narrowing below the printed 110-119. On the
// S the book never states a ceiling at all — doc.go's register entry THE S
// ROW'S CEILING IS UNSTATED, the design's A12 (unlifted; core/kw/ts590's
// layout.go names A12 for the same row).
const (
	lowestChannel  = 0
	highestChannel = 109
)

// The 50-byte record's field offsets, 0-indexed into the whole frame, counted
// off the position rulers both charts print (590:1440-1461, 590:1518-1536).
// The comment on each names the chart's own 1-indexed position.
const (
	recLen = 50 // the ';' at position 50 (590:1459-1461)

	recP1Off       = 2 // position 3
	recP2Off       = 3 // position 4
	recP3Off       = 4 // positions 5-6
	recP3Len       = 2
	recFreqOff     = 6 // positions 7-17
	recFreqLen     = 11
	recModeOff     = 17 // position 18
	recDataModeOff = 18 // position 19
	recToneModeOff = 19 // position 20
	recToneNoOff   = 20 // positions 21-22
	recCTCSSNoOff  = 22 // positions 23-24
	recP10Off      = 24 // positions 25-27
	recFilterOff   = 27 // position 28
	recP12Off      = 28 // position 29
	recP13Off      = 29 // positions 30-38
	recFMNarrowOff = 38 // positions 39-40
	recLockoutOff  = 40 // position 41
	recNameOff     = 41 // positions 42-49
	recNameLen     = 8
	recTermOff     = 49 // position 50
)

// The three runs both charts print as constants, quoted from this book:
// "000: Always 000" (590:1473, 590:1558-1559), "0: Always 0" (590:1480,
// 590:1565-1566) and "000000000: Always 000000000" (590:1482, 590:1567-1568).
const (
	printedP10 = "000"
	printedP12 = "0"
	printedP13 = "000000000"
)

// mrReadLen is the read request's width, "M R P1 P2 P3 P3 ;" — seven
// positions (590:1440-1442). The chart's own terminator cell prints ':'
// rather than ';' at position 7; that is a print defect (core/kw/doc.go's
// errata schedule records it as E1) and the terminator is the ';' the front
// matter requires of every command (590:87-91).
const mrReadLen = 7

// mcSetLen is the MC Set and Answer width, "M C P1 P2 P2 ;" — six positions
// (590:1333, 590:1341).
const mcSetLen = 6

// --- Field validators (wire level) ---
//
// EVERY ONE OF THEM IS ENFORCED ON THE SET DIRECTION, and every one is
// ASSUMED to be what the radio itself enforces — doc.go's register entry
// SET-DIRECTION FIELD STRICTNESS. Nothing here normalises: a byte outside its
// printed legend is refused, never quietly corrected, because a driver that
// sent one would otherwise pass its own tests and fail on hardware.

// validP1 reports whether b is one of P1's two printed values, "0: Simplex"
// and "1: Split" (590:1441-1443, 590:1519-1520).
func validP1(b byte) bool { return b == '0' || b == '1' }

// validHundredsByte reports whether b is a legal spelling of the channel
// number's 100's digit on a REQUEST: "When entering a setting command, enter
// 0 or a space for a channel number less than 100." (590:1334-1335), which
// both memory charts refer to. A10's read half: this fake accepts either
// spelling and answers the space form.
func validHundredsByte(b byte) bool { return isDigit(b) || b == ' ' }

// validModeByte reports whether b is a nibble the MD legend prints
// (590:1353-1363). ALL TEN ARE ADMITTED, including the two the legend calls
// "None (setting failure)" — see handleMW.
func validModeByte(b byte) bool { return isDigit(b) }

// validDataModeByte: "0: DATA mode OFF / 1: DATA mode ON" (590:447-449).
func validDataModeByte(b byte) bool { return b == '0' || b == '1' }

// validToneModeByte: P7's four printed values (590:1464-1467). The TS-480
// prints three (480:964), which is one of the reasons the two Kenwood fakes
// share nothing.
func validToneModeByte(b byte) bool { return b >= '0' && b <= '3' }

// validFilterByte: "0: FILTER A / 1: FILTER B" (590:1476-1477), on BOTH rows
// — doc.go's register entry BYTE 28 IS ACCEPTED EITHER WAY ON BOTH ROWS.
func validFilterByte(b byte) bool { return b == '0' || b == '1' }

// validFMNarrow: "00: FM Normal / 01: FM Narrow" (590:1484-1485). Two bytes,
// two printed values, nothing else.
func validFMNarrow(s string) bool { return s == "00" || s == "01" }

// validLockoutByte: "0: Channel Lockout OFF / 1: Channel Lockout ON"
// (590:1487-1488).
func validLockoutByte(b byte) bool { return b == '0' || b == '1' }

// validNameField reports whether the eight bytes of P16 are ones a name may
// carry. The book forbids exactly one byte — "';' (semicolon) cannot be used
// for the parameter P16." (590:1577) — which cannot arrive here anyway, since
// the reassembler ends a frame at the first ';'. The rest is A2's bound:
// printable ASCII 0x20-0x7E, which is where the evidence stops. 0x7F and
// above is unevidenced AND unclaimed, and refusing it is this programme's own
// charset rule rather than a statement about the radio.
func validNameField(field []byte) bool {
	for _, b := range field {
		if b < 0x20 || b > 0x7e {
			return false
		}
	}
	return true
}

// parseChannelDigits decodes the hundreds byte and the two-digit field into a
// channel number, and reports whether the spelling and the number are both
// admissible in the served slot space.
func parseChannelDigits(hundreds byte, twoDigits string) (int, bool) {
	if !validHundredsByte(hundreds) || !allDigits(twoDigits) || len(twoDigits) != recP3Len {
		return 0, false
	}
	h := 0
	if hundreds != ' ' {
		h = int(hundreds - '0')
	}
	n := h*100 + int(twoDigits[0]-'0')*10 + int(twoDigits[1]-'0')
	if n < lowestChannel || n > highestChannel {
		return 0, false
	}
	return n, true
}

// hundredsAnswerByte renders a channel number's 100's digit for an ANSWER.
//
// "For a response command, a space is entered for a channel number less than
// 100." (590:1336-1337) — printed for MC, and both memory charts refer their
// P2 cell to MC (590:1452-1453, 590:1539-1540). Applying MC's response
// convention to MR's answer is this fake's reading of that cross-reference,
// registered in doc.go as AN ANSWER'S HUNDREDS DIGIT IS A SPACE BELOW
// CHANNEL 100. It is deliberately the OPPOSITE spelling to the one core/kw
// emits, which always writes '0' and accepts either (A10): a fake that echoed
// the request's byte back would never exercise the codec's tolerance.
func hundredsAnswerByte(channel int) byte {
	if channel < 100 {
		return ' '
	}
	return byte('0' + channel/100)
}

// --- MR: the memory read (590:1438-1493) ---
//
// X O O X: no Set, a Read (590:1440-1442), a 50-byte Answer
// (590:1444-1461), no AI push.

// buildMRAnswer builds the 50-byte answer for one channel half.
func buildMRAnswer(channel int, half Half, s MemState) []byte {
	out := make([]byte, 0, recLen)
	out = append(out, 'M', 'R')
	out = append(out, byte(half))
	out = append(out, hundredsAnswerByte(channel))
	out = append(out, byte('0'+(channel/10)%10), byte('0'+channel%10))
	out = append(out, s.Freq...)
	out = append(out, s.Mode, s.DataMode, s.ToneMode)
	out = append(out, s.ToneNo...)
	out = append(out, s.CTCSSNo...)
	out = append(out, printedP10...)
	out = append(out, s.Filter)
	out = append(out, printedP12...)
	out = append(out, printedP13...)
	out = append(out, s.FMNarrow...)
	out = append(out, s.Lockout)
	out = append(out, s.Name...)
	out = append(out, ';')
	return out
}

func (r *Radio) handleMR(body []byte) []byte {
	if len(body) != mrReadLen-3 {
		// Includes an MR frame in the 50-byte SET shape: this book gives MR
		// no Set direction at all (590:1438-1461, which prints a Read row
		// and an Answer row and nothing else), so such a frame is simply
		// unknown.
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
		// WithMemoryReadUnsupported: the book's second "?;" cause, played —
		// "Command was not executed due to the current status of the
		// transceiver (even though the command syntax was correct)"
		// (590:100-105). Not a claim that any TS-590 refuses MR.
		return rejection
	}

	r.mu.Lock()
	s, ok := r.records[recordKey{channel: channel, half: half}]
	r.mu.Unlock()
	if !ok {
		// THE EMPTY CHANNEL IS DOCUMENTARY FACT ON THIS PAIR, not an
		// assumption: "If the selected channel is empty, P4 ~ P15 will be 0
		// and P16 will be blank." (590:1492-1493). It is answered, never
		// refused, which is what core/driver/ts590 must read as "empty
		// channel" and not as "parse error".
		//
		// The same answer is given for the SECOND HALF of a channel that has
		// no stored transmit record, where the book prints nothing at all
		// (the design's A9) — doc.go's register entry THE SECOND HALF OF A
		// CHANNEL WITH NO STORED TRANSMIT RECORD.
		s = emptyRecord()
	}
	return buildMRAnswer(channel, half, s)
}

// --- MW: the memory write (590:1516-1581) ---
//
// O X X X: a 50-byte Set and nothing else — the chart prints no Read row and
// no Answer row (590:1518-1536) — so an accepted write is silent.
//
// THE ERASE FORM IS NOT IMPLEMENTED AND CANNOT BE REACHED. The book
// describes one: "If you do not specify one digit in P16 and execute all the
// parameters from P4 to P15 set to 0, the channels specified by P2 and P3
// will be erased." (590:1579-1581) — a SHORTER frame. This programme builds
// no erase frame on any radio, and this handler accepts exactly the width the
// chart counts, so the erase form dies as a width violation before it can
// reach any state. TestMW_RefusesTheEraseForm.

func (r *Radio) handleMW(body []byte) []byte {
	if len(body) != recLen-3 {
		return rejection
	}
	// Re-index against the whole frame's rulers so that every offset
	// constant below reads as the chart's own position.
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
		Freq:     string(frame[recFreqOff : recFreqOff+recFreqLen]),
		Mode:     frame[recModeOff],
		DataMode: frame[recDataModeOff],
		ToneMode: frame[recToneModeOff],
		ToneNo:   string(frame[recToneNoOff : recToneNoOff+2]),
		CTCSSNo:  string(frame[recCTCSSNoOff : recCTCSSNoOff+2]),
		Filter:   frame[recFilterOff],
		FMNarrow: string(frame[recFMNarrowOff : recFMNarrowOff+2]),
		Lockout:  frame[recLockoutOff],
		Name:     string(frame[recNameOff : recNameOff+recNameLen]),
	}

	switch {
	case !allDigits(s.Freq):
		return rejection
	case !validModeByte(s.Mode):
		// ALL TEN NIBBLES ARE ADMITTED, the two the legend calls "None
		// (setting failure)" included (590:1353, 590:1362). Nothing says
		// what a radio does with a Set carrying one — that is the design's
		// A18b, whose lift is a hardware trial — so this fake stores the
		// nibble rather than inventing a refusal, which is what lets a test
		// drive core/kw's own build refusal against a real fake. doc.go's
		// register entry A SET CARRYING A "NONE" MODE NIBBLE IS STORED.
		return rejection
	case !validDataModeByte(s.DataMode):
		return rejection
	case !validToneModeByte(s.ToneMode):
		return rejection
	case !allDigits(s.ToneNo) || !allDigits(s.CTCSSNo):
		// THE INDICES ARE STORED, NOT RANGE-CHECKED. TN's own chart prints
		// "An entered value of 43 or higher results in an error"
		// (590:2309) for the TN COMMAND; whether the same rule holds inside
		// a memory frame is the design's A21, and unlifted. Refusing here
		// would assert A21 as a fact about the radio, so only the field's
		// shape is enforced — doc.go's register entry TONE INDICES ARE
		// STORED, NOT RANGE-CHECKED.
		return rejection
	case string(frame[recP10Off:recP10Off+len(printedP10)]) != printedP10:
		return rejection
	case !validFilterByte(s.Filter):
		return rejection
	case string(frame[recP12Off:recP12Off+len(printedP12)]) != printedP12:
		return rejection
	case string(frame[recP13Off:recP13Off+len(printedP13)]) != printedP13:
		return rejection
	case !validFMNarrow(s.FMNarrow):
		return rejection
	case !validLockoutByte(s.Lockout):
		return rejection
	case !validNameField(frame[recNameOff : recNameOff+recNameLen]):
		return rejection
	}

	r.mu.Lock()
	r.records[recordKey{channel: channel, half: half}] = s
	r.mu.Unlock()
	// A Set does not move the selection: nothing in the MW block mentions
	// it (590:1516-1581) — doc.go's register entry A SET DOES NOT MOVE THE
	// SELECTED CHANNEL.
	return nil // fire-and-forget success
}

// --- MC: the selected memory channel (590:1329-1347) ---
//
// O O O X. Set (590:1333) and Answer (590:1341) are six bytes, Read is "MC;"
// (590:1337). Disambiguated by length.
//
// THIS FAKE MODELS NO FRONT PANEL, so the selection moves only by an MC Set
// and this fake never produces an MC answer naming a channel no host asked
// for. Nothing in the MC block conditions the selection on the channel
// holding anything, and an empty channel is a documented, answerable state
// here (590:1492-1493), so an MC Set of an unwritten channel is accepted.

func buildMCAnswer(channel int) []byte {
	out := make([]byte, 0, mcSetLen)
	out = append(out, 'M', 'C')
	out = append(out, hundredsAnswerByte(channel))
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

// --- ID: the identity a probe turns into a wrong-radio refusal
// (590:1111-1119) ---
//
// X O O X: no Set, Read "ID;", a six-byte Answer "I D P1 P1 P1 ;"
// (590:1119). The VALUES are the two this book prints and no others —
// "021: TS-590S", "023: TS-590SG" (590:1114-1116) — and each row answers its
// own, which is what makes a probe against this fake succeed for one registry
// row and produce a wrong-radio refusal for the other.

func (r *Radio) handleID(body []byte) []byte {
	if len(body) != 0 {
		// The ID block prints a Read and an Answer and no Set at all, so
		// anything between "ID" and ';' is simply unknown.
		// TestID_HasNoSetDirection.
		return rejection
	}
	return []byte(r.row.idAnswer())
}

// --- FV: the firmware version string (590:1030-1037) ---
//
// X O O X: Read "FV;", a seven-byte Answer "F V P1 P1 P1 P1 ;" (590:1037).
// The book gives the field no grammar, only a width and one worked example:
// "For example, for firmware version 1.00, it reads 'FV1.00;'." (590:1035).
// The width has a second printed corroborator in the menu chart — menu 000,
// "Version information (4 ASCII characters) read only" (590:749).
//
// THIS FAKE APPLIES NO GRAMMAR OF ITS OWN to the four bytes, and that is a
// decision rather than an omission: reading them as "M.NN" is A13, an
// assumption the DRIVER carries on its own refusal path, and a fake that
// validated the field would be asserting A13 as a fact about the radio. What
// it does enforce is the WIDTH, which the chart counts.

// defaultFirmware is the book's one worked example (590:1035), used because
// it is the only FV answer this document prints. It is not a claim about any
// radio's firmware — doc.go's register entry THE DEFAULT FIRMWARE STRING.
const defaultFirmware = "1.00"

// firmwareFieldLen is the width of FV's P1 field, counted off the answer
// row's position ruler (590:1037).
const firmwareFieldLen = 4

func (r *Radio) handleFV(body []byte) []byte {
	if len(body) != 0 {
		return rejection
	}
	out := make([]byte, 0, 2+firmwareFieldLen+1)
	out = append(out, 'F', 'V')
	out = append(out, r.firmware...)
	out = append(out, ';')
	return out
}

// --- AI: Auto Information (590:156-172) ---
//
// O O O O. Set and Answer are four bytes (590:160, 590:168), Read is "AI;"
// (590:164). An AI Set is fire-and-forget.
//
// THE VALUE SET IS THIS BOOK'S AND NOT THE 480'S. Here P1 is "0: AI OFF /
// 2: AI ON (without backup) / 4: AI ON (with backup)" (590:159-162) with no
// 1 and no 3 at all; the TS-480 prints 0, 1, 2 and 3 with different meanings
// (480:185-190). There is no non-zero value that means the same thing on both
// radios, which is one of the reasons no Kenwood fake may borrow another's
// tables.
//
// This fake never PUSHES anything unsolicited whatever AI is set to. No
// TS-590 of either row has been observed by this project, and modelling
// silence is the honest default — doc.go's register entry
// AUTOMATIC-INFORMATION SUPPRESSION. core/transport.Engine.Init opens every
// session with an AI-off Set, so this handler's silent-accept path is on the
// critical path of every fake session.

// The three P1 values this book prints (590:159-162).
const (
	aiOff             = '0'
	aiOnWithoutBackup = '2'
	aiOnWithBackup    = '4'
)

func validAIByte(b byte) bool {
	return b == aiOff || b == aiOnWithoutBackup || b == aiOnWithBackup
}

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

// toUpperASCII folds one ASCII lower-case byte to upper case and leaves every
// other byte alone. Used on COMMAND NAMES ONLY — see handleFrame.
func toUpperASCII(b byte) byte {
	if b >= 'a' && b <= 'z' {
		return b - 'a' + 'A'
	}
	return b
}

// handleFrame parses one complete, ';'-terminated frame (as produced by
// reassembler.push) and returns the reply to send: nil for a fire-and-forget
// success, or a non-nil frame — a real answer, or rejection — otherwise.
// Unknown and garbled commands fall through to rejection.
//
// COMMAND NAMES ARE MATCHED IN EITHER CASE, and that is a MANUAL FACT of
// these radios rather than a leniency inherited from a Yaesu fake: "A command
// consists of 2 or 3 characters. You may use either lower or upper case
// characters." (590:62-63). TestCommandNamesAreAcceptedInEitherCase pins it,
// including the mixed-case form: "either lower or upper" says nothing about
// mixing, so admitting it is a CONSEQUENCE of folding each byte
// independently, not a separate invented leniency.
//
// FIELD VALUES REMAIN CASE-SENSITIVE. The sentence is about the two-character
// command NAME and says nothing about parameters, so extending it would be an
// invented leniency.
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
	case [2]byte{'F', 'V'}:
		return r.handleFV(rest)
	case [2]byte{'A', 'I'}:
		return r.handleAI(rest)
	case [2]byte{'M', 'R'}:
		return r.handleMR(rest)
	case [2]byte{'M', 'W'}:
		return r.handleMW(rest)
	case [2]byte{'M', 'C'}:
		return r.handleMC(rest)
	case [2]byte{'E', 'X'}:
		// READ ONLY (ex.go). An EX SET falls through handleEX's own body
		// check to "?;" — a MODELLING GAP, stated in doc.go, not a claim
		// that a real TS-590 refuses EX Set.
		return r.handleEX(rest)
	default:
		return rejection
	}
}
