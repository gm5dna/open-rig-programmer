// SPDX-License-Identifier: GPL-3.0-or-later

package fakets890

import "strings"

// This file is fakets890's own, independent byte-level parser and reply
// builder for the TS-890S. It is derived from that radio's own position charts
// in the TS-890S PC Control Command Reference Guide (rev 1) — cited line by
// line beside each section below as "890:NNNN" — and NOT from core/kw/ma. See
// doc.go for why that independence matters and for the full ASSUMED register;
// individual assumed points are flagged inline, next to the code that
// implements them.
//
// The manual itself is gitignored (docs/fixtures-private/manuals/), so the
// line references here are citations in the sense core/kw/ma/doc.go uses them:
// they name where a chart is, they are not links.

// --- General framing ---

// rejection is the protocol's one and only NAK, "?;". This book's error table
// gives it two causes and says they are not distinguished — "Command syntax
// was incorrect" and "Command was not executed due to the current status of
// the transceiver (even though the command syntax was correct)"
// (890:106-112) — so a malformed frame, an unknown command, an unserved slot
// and an overflowed accumulator are all indistinguishable to the host, which
// is the whole of the convention.
var rejection = []byte("?;")

// StreamError is one of the two SERIAL-LINE error tokens this book prints
// beside "?;", which are not command outcomes at all: "E;" for "A
// communication error occurred, such as an overrun or framing error during a
// serial data transmission" (890:118-120) and "O;" for "A receive buffer
// overrun error occurred" (890:121-123).
//
// The zero value is refused by WithStreamError: a scripted fault that
// defaulted to one of the two tokens would put a test on the wrong sentence.
type StreamError int

// The two stream-error tokens, plus the refusing default.
const (
	StreamErrorUnset StreamError = iota
	// StreamErrorE is "E;" (890:118-120).
	StreamErrorE
	// StreamErrorO is "O;" (890:121-123). This book's cause sentence for it
	// is the TS-590's word for word and NOT the TS-480's, which erratum E13
	// records as the one document out of four that says something else.
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
// bounded-input policy, not a manual figure (doc.go's register entry THE FRAME
// ACCUMULATOR'S CAP AND RESYNC).
const maxAccumulatorBytes = 256

// reassembler turns an arbitrary stream of Write() chunks into complete
// ';'-terminated frames. Framing only: it says nothing about what any frame
// means. The terminator is this book's own — "To signal the end of a command,
// it is necessary to use a semicolon (;)." (890:92-96).
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

// allDigits reports whether s is a non-empty run of ASCII digits. The
// predicate is spelt out rather than taken from unicode: a wire field is ASCII
// or it is not this radio's, and unicode.IsDigit would admit other scripts'
// decimal digits.
func allDigits(s string) bool {
	return s != "" && strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

// --- The channel number, and the MA0 grid ---
//
// THE SLOT SPACE THIS FAKE SERVES IS 000-099, AND THAT IS NARROWER THAN THE
// PRINTED DOMAIN. The chart prints "000 ~ 119" with two further classes
// mapped onto it — "Channels P0 ~ P9 are represented as 100 ~ 109 and channels
// E0 ~ E9 are represented as 110 ~ 119." (890:3167-3169) — and that number
// mapping is the ONLY thing this book says about either class. What a
// Programmable VFO slot answers to an MA0 read is unprinted (the design's A9);
// what an E channel even IS is never explained anywhere in the document
// (erratum E18, which counts eight sites, every one the same mapping
// sentence). Both classes are published in NO bank of this registry row, so
// there is no slot ID for an image to represent and nothing for this fake to
// serve.
//
// 100-119 is therefore refused here exactly as any other out-of-domain number
// is — one refusal path, no special case. doc.go's register entry SLOTS
// 100-119 ARE NOT SERVED records that this is a deliberate narrowing of the
// printed domain and NOT a claim that a TS-890S refuses those numbers.

// The served slot space's bounds.
const (
	lowestChannel  = 0
	highestChannel = 99
)

// The MA0 grid's field offsets, 0-indexed into the whole frame, counted off
// the position rulers both directions print (890:3166-3182 Set, 890:3189-3204
// Answer). The comment on each names the chart's own 1-indexed position.
//
// EVERY ONE OF THE THIRTEEN IS A LIVE FIELD. This grid has no hard-wired byte
// at all, which is where it parts company with the 590 pair's three printed
// constant runs.
const (
	// recFixedLen is positions 1 to 39: the name "MA0", the three-digit
	// channel number and P2 to P12. The name and the terminator follow it.
	recFixedLen = 39

	recSlotOff          = 3 // positions 4-6
	recSlotLen          = 3
	recFreqOff          = 6 // positions 7-17
	recFreqLen          = 11
	recModeOff          = 17 // position 18
	recFMNarrowOff      = 18 // position 19
	recToneTypeOff      = 19 // position 20
	recToneNoOff        = 20 // positions 21-22
	recCTCSSNoOff       = 22 // positions 23-24
	recSplitFreqOff     = 24 // positions 25-35
	recSplitModeOff     = 35 // position 36
	recSplitFMNarrowOff = 36 // position 37
	recSplitOff         = 37 // position 38
	recLockoutOff       = 38 // position 39
	recNameOff          = 39 // position 40 onwards
	recIndexLen         = 2  // P6 and P7 are two digits each

	// maxNameLen is P13's counted bound, "Up to 10 characters"
	// (890:3208-3209). With the floating terminator (890:3181-3182) that
	// makes an MA0 frame 40 to 50 bytes — the 40 being the design's A17,
	// derived rather than printed, and the shape a blank channel takes.
	maxNameLen = 10
)

// ma0ReadLen is the read request's width, "M A 0 P1 P1 P1 ;" — seven positions
// (890:3184-3186).
const ma0ReadLen = 7

// --- Field validators (wire level) ---
//
// EVERY ONE OF THEM IS ENFORCED ON THE SET DIRECTION, and every one is ASSUMED
// to be what the radio itself enforces — doc.go's register entry SET-DIRECTION
// FIELD STRICTNESS. Nothing here normalises: a byte outside its printed legend
// is refused, never quietly corrected, because a driver that sent one would
// otherwise pass its own tests and fail on hardware.

// validModeByte reports whether b is a nibble the OM P2 legend prints
// (890:3976-3992). ALL SIXTEEN ARE ADMITTED, including the two the legend
// calls "Unused" — see handleMA0Set. The letters are upper case only: the
// case-folding sentence at 890:76-80 is about the command NAME and says
// nothing about parameters.
func validModeByte(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'A' && b <= 'F')
}

// validZeroOne is P4 (890:3177-3178), P10 (890:3199-3200), P11
// (890:3202-3203) and P12 (890:3206-3207): four fields, one legend shape, two
// printed values each.
func validZeroOne(b byte) bool { return b == '0' || b == '1' }

// validToneTypeByte: P5's four printed values, "0: OFF / 1: Tone / 2: CTCSS /
// 3: Cross Tone" (890:3181-3185).
func validToneTypeByte(b byte) bool { return b >= '0' && b <= '3' }

// validNameField reports whether the name a Set carries is one P13 may hold.
//
// TWO RULES, AND THEY COME FROM DIFFERENT PLACES. The LENGTH is the chart's
// own count, "Up to 10 characters" (890:3208-3209). The CHARSET is A2(i): the
// printable-ASCII characters this design writes, 0x20-0x7E. ';' cannot arrive
// here anyway, since the reassembler ends a frame at the first one. 0x7F and
// above is unevidenced AND unclaimed — this book's own coding rule remaps
// 80h-FFh by Menu 9-01 (890:31-43) — so refusing it is this programme's
// charset rule rather than a statement about the radio.
func validNameField(name string) bool {
	if len(name) > maxNameLen {
		return false
	}
	for i := 0; i < len(name); i++ {
		if name[i] < 0x20 || name[i] > 0x7e {
			return false
		}
	}
	return true
}

// parseSlot decodes the three-digit channel field and reports whether the
// spelling and the number are both admissible in the served slot space.
//
// THE SPELLING IS THREE ASCII DIGITS AND NOTHING ELSE. Unlike the TS-590's MC
// chart, which prints "enter 0 or a space for a channel number less than 100",
// this book prints one domain — "000 ~ 119" (890:3167) — over three P1 cells
// and describes no space form anywhere. Admitting one would be an invented
// leniency.
func parseSlot(cell string) (int, bool) {
	if len(cell) != recSlotLen || !allDigits(cell) {
		return 0, false
	}
	n := int(cell[0]-'0')*100 + int(cell[1]-'0')*10 + int(cell[2]-'0')
	if n < lowestChannel || n > highestChannel {
		return 0, false
	}
	return n, true
}

// slotWire renders a channel number as the three bytes an ANSWER carries.
//
// THREE ZERO-PADDED ASCII DIGITS, NEVER SPACE-PADDED — doc.go's register entry
// THE ANSWER'S CHANNEL NUMBER IS THREE ZERO-PADDED DIGITS, which is the
// design's A5. No book states what this radio answers with; the entry is
// narrow and its failure direction is safe, because a space-padded answer
// misses the codec's prefix matcher and times out rather than being
// mis-attributed to another channel.
func slotWire(channel int) string {
	return string([]byte{
		byte('0' + channel/100),
		byte('0' + (channel/10)%10),
		byte('0' + channel%10),
	})
}

// --- MA0: the memory channel configuration (890:3164-3221) ---
//
// O O O X: a Set (890:3166-3182), a Read (890:3184-3186) and an Answer
// (890:3189-3204). The Set and the Answer print the SAME thirteen positions,
// so they are the same bytes in the two directions and are disambiguated by
// length: a seven-byte frame is the Read, a 40-to-50-byte one the Set.
//
// AN ACCEPTED SET IS SILENT. MA0's own notes (890:3211-3221) say nothing about
// an acknowledgement, where MA2, MA3 and MA6 each print "When the AI function
// is ON, a response is provided by the MA0 command" (890:3266-3267,
// 890:3283-3284, 890:3327-3328). That is the design's A20 and doc.go's
// register entry AN ACCEPTED SET PRODUCES NO REPLY.

// buildMA0Answer builds the answer frame for one channel: 39 fixed positions,
// the name, and the floating terminator.
func buildMA0Answer(channel int, s MemState) []byte {
	out := make([]byte, 0, recFixedLen+maxNameLen+1)
	out = append(out, "MA0"...)
	out = append(out, slotWire(channel)...)
	out = append(out, s.Freq...)
	out = append(out, s.Mode, s.FMNarrow, s.ToneType)
	out = append(out, s.ToneNo...)
	out = append(out, s.CTCSSNo...)
	out = append(out, s.SplitFreq...)
	out = append(out, s.SplitMode, s.SplitFMNarrow, s.Split, s.Lockout)
	out = append(out, s.Name...)
	out = append(out, ';')
	return out
}

// handleMA0 dispatches on the body's length: the Read and the Set share a
// name and are told apart by the ruler each is drawn to.
func (r *Radio) handleMA0(body []byte) []byte {
	switch {
	case len(body) == ma0ReadLen-4:
		return r.handleMA0Read(string(body))
	case len(body) >= recFixedLen-3 && len(body) <= recFixedLen-3+maxNameLen:
		return r.handleMA0Set(body)
	}
	return rejection
}

func (r *Radio) handleMA0Read(slotCell string) []byte {
	channel, ok := parseSlot(slotCell)
	if !ok {
		return rejection
	}
	if r.memoryReadUnsupported {
		// WithMemoryReadUnsupported: this book's second "?;" cause, played
		// — "Command was not executed due to the current status of the
		// transceiver (even though the command syntax was correct)"
		// (890:108-112). Not a claim that any TS-890S refuses MA0.
		return rejection
	}

	r.mu.Lock()
	s, ok := r.records[channel]
	r.mu.Unlock()
	if !ok {
		// THE BLANK CHANNEL IS A NORMAL, ANSWERABLE STATE ON THIS RADIO:
		// "When reading a blank channel, parameters P2 to P12 becomes
		// blank." (890:3215-3216). It is answered, never refused, which is
		// what core/driver/ts890 must read as "empty channel" and not as
		// "parse error" — the distinction core/clone's whole-read
		// abandonment depends on.
		//
		// WHAT "BLANK" MEANS IS A6 (ASCII space) and WHETHER THE NAME
		// WINDOW IS THERE AT ALL IS A4 — the note stops at P12 where the
		// 990S's covers P2 to P18 (erratum E4). doc.go's register entry A
		// BLANK CHANNEL ANSWERS THE 40-BYTE FRAME carries both.
		s = BlankRecord()
	}
	return buildMA0Answer(channel, s)
}

func (r *Radio) handleMA0Set(body []byte) []byte {
	// Re-index against the whole frame's rulers so that every offset
	// constant below reads as the chart's own position.
	frame := append(append([]byte("MA0"), body...), ';')

	channel, ok := parseSlot(string(frame[recSlotOff : recSlotOff+recSlotLen]))
	if !ok {
		return rejection
	}

	s := MemState{
		Freq:          string(frame[recFreqOff : recFreqOff+recFreqLen]),
		Mode:          frame[recModeOff],
		FMNarrow:      frame[recFMNarrowOff],
		ToneType:      frame[recToneTypeOff],
		ToneNo:        string(frame[recToneNoOff : recToneNoOff+recIndexLen]),
		CTCSSNo:       string(frame[recCTCSSNoOff : recCTCSSNoOff+recIndexLen]),
		SplitFreq:     string(frame[recSplitFreqOff : recSplitFreqOff+recFreqLen]),
		SplitMode:     frame[recSplitModeOff],
		SplitFMNarrow: frame[recSplitFMNarrowOff],
		Split:         frame[recSplitOff],
		Lockout:       frame[recLockoutOff],
		Name:          string(frame[recNameOff : len(frame)-1]),
	}

	switch {
	case !allDigits(s.Freq) || !allDigits(s.SplitFreq):
		// "Blank digits must be entered as '0'." (890:3172, 890:3192) — the
		// SET direction is digits, and the blank spelling A6 reads on the
		// ANSWER direction has no place here.
		return rejection
	case !validModeByte(s.Mode) || !validModeByte(s.SplitMode):
		// ALL SIXTEEN NIBBLES ARE ADMITTED, the two the OM legend calls
		// "Unused" included (890:3977, 890:3985). Nothing says what a radio
		// does with a Set carrying one, so this fake stores the nibble
		// rather than inventing a refusal, which is what lets a test drive
		// the codec's own build refusal against a real fake. doc.go's
		// register entry A SET CARRYING AN "UNUSED" MODE NIBBLE IS STORED.
		return rejection
	case !validZeroOne(s.FMNarrow) || !validZeroOne(s.SplitFMNarrow):
		// THE P4/P10 AGREEMENT IS NOT CHECKED HERE, only each byte's own
		// legend. The book instructs the HOST to keep the two equal on a
		// split channel (890:3219-3221) and says nothing about what the
		// radio does with a disagreeing Set; core/driver/ts890 carries the
		// refusal rung, and a fake that pre-empted it would put that rung
		// out of reach of a real fake. doc.go's register entry THE P4/P10
		// AGREEMENT IS NOT ENFORCED.
		return rejection
	case !validToneTypeByte(s.ToneType):
		return rejection
	case !allDigits(s.ToneNo) || !allDigits(s.CTCSSNo):
		// THE INDICES ARE STORED, NOT RANGE-CHECKED. The TN and CN charts
		// each print "Entering a value that does not exist is invalid" for
		// their OWN command (890:5165, 890:1371); whether the same rule
		// holds inside an MA0 frame is unprinted, so only the field's shape
		// is enforced — doc.go's register entry TONE INDICES ARE STORED,
		// NOT RANGE-CHECKED.
		return rejection
	case !validZeroOne(s.Split) || !validZeroOne(s.Lockout):
		return rejection
	case !validNameField(s.Name):
		return rejection
	}

	r.mu.Lock()
	r.records[channel] = s
	r.mu.Unlock()
	// A Set to a channel this fake holds no record for is STORED, not
	// refused: whether MA0 alone can create a channel is the design's A3 and
	// core/driver/ts890 refuses such a write on that ground. Manufacturing
	// the refusal here would assert A3 as a fact about the radio and put the
	// driver's own refusal out of reach of a real fake. doc.go's register
	// entry A SET TO A BLANK CHANNEL IS STORED.
	return nil // fire-and-forget success
}

// --- ID: the identity a probe turns into a wrong-radio refusal
// (890:2730-2738) ---
//
// X O O X: no Set, Read "ID;", a six-byte Answer "I D P1 P1 P1 ;"
// (890:2738). The VALUE is the one this book prints and no other — "024:
// TS-890S" (890:2733) — which is what makes a probe against this fake succeed
// for this registry row and produce a wrong-radio refusal for any other.

// idAnswer is the whole of this radio's identity reply (890:2733, 890:2738).
const idAnswer = "ID024;"

func (r *Radio) handleID(body []byte) []byte {
	if len(body) != 0 {
		// The ID block prints a Read and an Answer and no Set at all, so
		// anything between "ID" and ';' is simply unknown.
		// TestID_HasNoSetDirection.
		return rejection
	}
	return []byte(idAnswer)
}

// --- FV: the firmware version string (890:2650-2659) ---
//
// X O O X: Read "FV;", a seven-byte Answer "F V P1 P1 P1 P1 ;" (890:2659).
// The book gives the field no grammar, only a width and one worked example:
// "For example: 'FV1.00;' (firmware version 1.00)" (890:2657).
//
// THIS FAKE APPLIES NO GRAMMAR OF ITS OWN to the four bytes, and that is a
// decision rather than an omission: reading them as "M.NN" is the design's
// A13, an assumption the DRIVER carries on its own refusal path, and a fake
// that validated the field would be asserting A13 as a fact about the radio.
// What it does enforce is the WIDTH, which the chart's ruler counts.

// defaultFirmware is this book's one worked example (890:2657), used because
// it is the only FV answer the document prints — doc.go's register entry THE
// DEFAULT FIRMWARE STRING. It is not a claim about any radio's firmware.
const defaultFirmware = "1.00"

// firmwareFieldLen is the width of FV's P1 field, counted off the answer row's
// position ruler (890:2659).
const firmwareFieldLen = 4

func (r *Radio) handleFV(body []byte) []byte {
	if len(body) != 0 {
		return rejection
	}
	out := make([]byte, 0, 2+firmwareFieldLen+1)
	out = append(out, 'F', 'V')
	out = append(out, defaultFirmware...)
	out = append(out, ';')
	return out
}

// --- AI: Auto Information (890:172-190) ---
//
// O O O O. Set and Answer are four bytes (890:177, 890:184), Read is "AI;"
// (890:181). An AI Set is fire-and-forget.
//
// THE VALUE SET IS THIS BOOK'S AND NOT A SIBLING'S. Here the legend prints
// FIVE values — "0: AI OFF", "1: Not used", "2: AI ON (Not back up the ON
// state)", "3: Not used", "4: AI ON (Back up the ON state)" (890:175-181) —
// where the TS-590's prints three with no 1 and no 3 at all, and the TS-480's
// prints four with different meanings. There is no non-zero value that means
// the same thing across the family, which is one of the reasons no Kenwood
// fake may borrow another's tables.
//
// This fake never PUSHES anything unsolicited whatever AI is set to. No
// TS-890S has been observed by this project, and modelling silence is the
// honest default — doc.go's register entry AUTOMATIC-INFORMATION SUPPRESSION.
// core/transport.Engine.Init opens every session with an AI-off Set, so this
// handler's silent-accept path is on the critical path of every fake session.

// The three P1 values this book prints WITH A MEANING (890:175, 890:179,
// 890:181).
const (
	aiOff             = '0'
	aiOnWithoutBackup = '2'
	aiOnWithBackup    = '4'
)

// validAIByte admits the three values above and REFUSES the two the legend
// prints as "Not used" (890:177, 890:180) — doc.go's register entry THE AI
// VALUES PRINTED "NOT USED" ARE REFUSED. What a radio does with one of them is
// unprinted, and refusing is the strict direction this package takes on every
// Set field.
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

// upperASCII folds the ASCII letters of a command name to upper case and
// leaves every other byte alone.
//
// NOT bytes.ToUpper or strings.ToUpper: those are Unicode-aware, so a body
// carrying arbitrary line noise comes back re-encoded and of a DIFFERENT
// LENGTH — longer for an invalid byte (U+FFFD), shorter for a valid sequence
// that case-folds to fewer bytes ("ı" is two bytes and uppercases to
// one). A fake whose whole job is byte-exact wire behaviour folds ASCII and
// touches nothing else.
func upperASCII(s string) string {
	out := []byte(s)
	for i, b := range out {
		if b >= 'a' && b <= 'z' {
			out[i] = b - 'a' + 'A'
		}
	}
	return string(out)
}

// handleFrame parses one complete, ';'-terminated frame (as produced by
// reassembler.push) and returns the reply to send: nil for a fire-and-forget
// success, or a non-nil frame — a real answer, or rejection — otherwise.
// Unknown and garbled commands fall through to rejection.
//
// THE NAME IS NOT A FIXED TWO BYTES ON THIS RADIO: "A command consists of 2 to
// 5 alphanumeric characters." (890:76-80), and this milestone's roster carries
// one three-character name ("MA0") beside four two-character ones. So the
// dispatcher tries the LONGER name first and falls back to the shorter, which
// is also what makes "MA1;" — a real command of this family that is not on the
// roster — fall through to rejection rather than being read as an "MA" with a
// stray parameter. There is no MA command.
//
// COMMAND NAMES ARE MATCHED IN EITHER CASE, and that is a MANUAL FACT of this
// radio: the same sentence at 890:76-80 says "You may use either lower or
// upper case characters". TestCommandNamesAreAcceptedInEitherCase pins it,
// including the mixed-case form: "either lower or upper" says nothing about
// mixing, so admitting it is a CONSEQUENCE of folding each byte independently,
// not a separate invented leniency.
//
// FIELD VALUES REMAIN CASE-SENSITIVE. The sentence is about the command NAME
// and says nothing about parameters, so extending it would be an invented
// leniency — TestFieldValuesRemainCaseSensitive, on the OM legend's A-F.
func (r *Radio) handleFrame(frame []byte) []byte {
	if len(frame) == 0 || frame[len(frame)-1] != ';' {
		return rejection // defensive: the reassembler never hands us this
	}
	body := string(frame[:len(frame)-1])
	if len(body) < 2 {
		return rejection
	}

	if len(body) >= 3 && upperASCII(body[:3]) == "MA0" {
		return r.handleMA0([]byte(body[3:]))
	}
	rest := []byte(body[2:])
	switch upperASCII(body[:2]) {
	case "ID":
		return r.handleID(rest)
	case "FV":
		return r.handleFV(rest)
	case "AI":
		return r.handleAI(rest)
	case "EX":
		// READ ONLY (ex.go). An EX SET falls through handleEX's own body
		// check to "?;" — a MODELLING GAP, stated in doc.go, not a claim
		// that a real TS-890S refuses EX Set.
		return r.handleEX(rest)
	}
	return rejection
}
