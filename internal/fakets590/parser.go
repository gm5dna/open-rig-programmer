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
// silence is the honest default — doc.go's register entry AUTOMATIC-
// INFORMATION SUPPRESSION. core/transport.Engine.Init opens every session
// with an AI-off Set, so this handler's silent-accept path is on the critical
// path of every fake session.

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
	default:
		// EX (MENU) falls here DELIBERATELY: this fake serves no menu
		// inventory yet, and the next task of the plan brings one in from
		// its own copy of transcription B. Until then an EX frame in either
		// direction draws "?;" — a MODELLING GAP, stated in doc.go and
		// pinned by TestEX_IsNotModelledYet so that adding EX has to change
		// a test rather than fill a silence.
		return rejection
	}
}
