// SPDX-License-Identifier: GPL-3.0-or-later

package fakets480

// This file is fakets480's own, independent byte-level parser and reply
// builder for the TS-480. It is derived from that radio's own position
// charts in the PC CONTROL COMMAND REFERENCE FOR THE TS-480HX/SAT
// TRANSCEIVER (2003) — cited line by line beside each section below as
// "480:NNNN" — and NOT from core/kw, and NOT from internal/fakets590. See
// doc.go for why that independence matters and for the full ASSUMED
// register; individual assumed points are flagged inline, next to the code
// that implements them.
//
// The manual itself is gitignored (docs/fixtures-private/manuals/), so the
// line references here are citations in the sense core/kw/doc.go uses them:
// they name where the chart is, they are not links.

// --- General framing ---

// rejection is the protocol's one and only NAK, "?;". This book's error
// table gives it two causes and says they are not distinguished — "Command
// syntax was incorrect" and "Command was not executed due to the current
// status of the transceiver (even though the command syntax was correct)"
// (480:130-135) — so an empty slot, a malformed frame, an unknown command
// and an overflowed accumulator are all indistinguishable to the host, which
// is the whole of the convention.
//
// UNLIKE THE YAESU FAMILY'S, THIS CONVENTION IS PRINTED IN THIS RADIO'S OWN
// BOOK (480:126-144) rather than inherited from a sibling's reference.
var rejection = []byte("?;")

// StreamError is one of the two SERIAL-LINE error tokens this book prints
// beside "?;", which are not command outcomes at all: "E;" for "A
// communication error occurred such as an overrun or framing error during a
// serial data transmission" (480:140-142) and "O;" for "Receive data was
// sent but processing was not completed" (480:143-144).
//
// THE "O;" CAUSE IS NOT THE 590 PAIR'S. That book calls the same token "A
// receive buffer overrun error occurred" (590:113); this one describes an
// incomplete processing of data that WAS received. The two books disagree —
// core/kw/doc.go's errata schedule records it as E13 — which is why no
// Kenwood fake may borrow another's error vocabulary and why this type is
// declared here rather than shared.
//
// The zero value is refused by WithStreamError: a scripted fault that
// defaulted to one of the two tokens would put a test on the wrong sentence.
type StreamError int

// The two stream-error tokens, plus the refusing default.
const (
	StreamErrorUnset StreamError = iota
	// StreamErrorE is "E;" (480:140-142).
	StreamErrorE
	// StreamErrorO is "O;" (480:143-144).
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
// it is necessary to use a semicolon (;)" (480:113-118).
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

// --- ID: the identity a probe turns into a wrong-radio refusal
// (480:676-687) ---
//
// X O O X: no Set, Read "ID;" (480:683), a six-byte Answer
// "I D P1 P1 P1 ;" (480:687). The VALUE is the one this book prints and no
// other — "020: TS-480" (480:678) — which is what makes a probe against this
// fake succeed for the TS-480 registry row and produce a wrong-radio refusal
// against any other.

// idAnswer is the whole of this radio's identity answer (480:678, 480:687).
const idAnswer = "ID020;"

func (r *Radio) handleID(body []byte) []byte {
	if len(body) != 0 {
		// The ID block prints a Set LABEL over an EMPTY chart (480:679) —
		// erratum E17, where a transcriber who reads the label as evidence
		// invents a command — so anything between "ID" and ';' is simply
		// unknown. TestID_HasNoSetDirection.
		return rejection
	}
	return []byte(idAnswer)
}

// --- TY: the microprocessor type, which is NOT a firmware version
// (480:1621-1634) ---
//
// X O O X: Read "TY;" (480:1630), a six-byte Answer "T Y P1 P1 P2 ;"
// (480:1634).
//
// THIS RADIO HAS NO FV AND NO CAT-READABLE FIRMWARE VERSION AT ALL. "FV"
// appears nowhere in this document, and the document carries no revision
// number and no firmware statement either — core/kw/doc.go's errata schedule
// records that as E15. TY is the nearest thing, and what it reports is a
// HARDWARE VARIANT: "0: TS-480HX (200 W) / 1: TS-480SAT (100 W + AT) /
// 2: Japanese 50 W type / 3: Japanese 20 W type" (480:1626-1629).
//
// THE HEADING SAYS "Sets or reads" AND THE SET CHART IS EMPTY (480:1621,
// 480:1625) — erratum E10. The chart wins: this fake refuses a TY Set.
//
// P1 IS OPAQUE AND STAYS OPAQUE. The parameter list prints one word for it,
// "Reserved" (480:1623), and says nothing else anywhere, so this fake
// answers whatever bytes it was given and applies no grammar. A fake that
// validated P1 would be asserting a grammar the document does not print, on
// the one field of this family that is admitted above 0x7E.

// tyReservedLen is the width of TY's P1 field, counted off the answer row's
// position ruler (480:1634): two bytes.
const tyReservedLen = 2

// The shipped TY answer — doc.go's register entry THE DEFAULT TY ANSWER.
const (
	// defaultTYReserved is P1. The document gives the field no legend at
	// all, so the two bytes take the character every OTHER hard-wired field
	// in this book prints, "Always 0" (480:953, 480:973, 480:975, 480:982).
	defaultTYReserved = "00"
	// defaultTYVariant is P2's FIRST printed value, "0: TS-480HX (200 W)"
	// (480:1626). It is not a claim that any particular TS-480 is an HX.
	defaultTYVariant = '0'
)

func (r *Radio) handleTY(body []byte) []byte {
	if len(body) != 0 {
		return rejection
	}
	out := make([]byte, 0, 2+tyReservedLen+1+1)
	out = append(out, 'T', 'Y')
	out = append(out, r.tyReserved...)
	out = append(out, r.tyVariant, ';')
	return out
}

// --- AI: Auto Information (480:183-198) ---
//
// O O O O. Set and Answer are four bytes (480:189, 480:197), Read is "AI;"
// (480:193). An AI Set is fire-and-forget.
//
// THE VALUE SET IS THIS BOOK'S AND NOT THE 590 PAIR'S. Here P1 is "0: AI OFF
// / 1: Only old AI format is ON / 2: Only extended AI format is ON / 3: Both
// formats are ON" (480:185-190) — four consecutive values. The TS-590S/SG
// book prints 0, 2 and 4 with different meanings and no 1 or 3 at all
// (590:159-162). There is no non-zero value that means the same thing on
// both radios, which is one of the reasons no Kenwood fake may borrow
// another's tables.
//
// THIS FAKE NEVER PUSHES ANYTHING UNSOLICITED, whatever AI is set to, and on
// this radio that is a LARGER gap than on the 590 pair, because this book
// describes the push concretely: "When the extended AI format is selected,
// the transceiver automatically sends the parameters. When the old AI is ON
// and the IF parameters change, the transceiver sends the IF command every
// 1.5 seconds." (480:192-195). doc.go's register entry AUTOMATIC-INFORMATION
// SUPPRESSION says what is and is not claimed by modelling silence.
// core/transport.Engine.Init opens every session with an AI-off Set, so this
// handler's silent-accept path is on the critical path of every fake
// session.

// The ends of the P1 legend this book prints: "0: AI OFF", then "1: Only
// old AI format is ON" and "2: Only extended AI format is ON", then "3: Both
// formats are ON" (480:185-190).
const (
	aiOff         = '0'
	aiBothFormats = '3'
)

// validAIByte admits the four printed values. A RANGE IS THE WHOLE LEGEND
// HERE, not a shortcut past a gap: this book's four values are CONSECUTIVE
// (480:185-190), where the 590 pair's are 0, 2 and 4 with no 1 and no 3
// (590:159-162).
func validAIByte(b byte) bool {
	return b >= aiOff && b <= aiBothFormats
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
// COMMAND NAMES ARE MATCHED IN EITHER CASE, and that is a MANUAL FACT of this
// radio rather than a leniency inherited from a Yaesu fake: "A command
// consists of 2 alphabetical characters. You may use either lower or upper
// case characters." (480:76-78). TestCommandNamesAreAcceptedInEitherCase pins
// it, including the mixed-case form: "either lower or upper" says nothing
// about mixing, so admitting it is a CONSEQUENCE of folding each byte
// independently, not a separate invented leniency.
//
// TWO CHARACTERS, NOT "TWO OR THREE". This book's sentence gives the command
// name one width, where the 590 pair's says "2 or 3 characters" (590:62-63)
// and that book has a three-character command to go with it. Nothing here
// needs a third byte, and a name is two bytes.
//
// FIELD VALUES REMAIN CASE-SENSITIVE. The sentence is about the command NAME
// and says nothing about parameters, so extending it would be an invented
// leniency.
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
	case [2]byte{'T', 'Y'}:
		return r.handleTY(rest)
	case [2]byte{'A', 'I'}:
		return r.handleAI(rest)
	default:
		// TWO DIFFERENT KINDS OF THING LAND HERE.
		//
		// "FV;" is a command this book does not contain at all — a grep of
		// this document for the name returns nothing — so refusing it is
		// what a real TS-480 must do and is a fact about the radio.
		//
		// EX (MENU) falls here DELIBERATELY and is a MODELLING GAP: this
		// book prints a full EX chart (480:399-416) and the radio plainly
		// has the command, but this fake serves no menu inventory yet, and
		// the next task of the plan brings one in from its own copy of
		// transcription B. Until then an EX frame in either direction draws
		// "?;", doc.go says so, and TestEX_IsNotModelledYet pins the gap's
		// shape so that adding EX has to change a test rather than fill a
		// silence.
		return rejection
	}
}
