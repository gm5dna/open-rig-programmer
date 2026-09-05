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

// --- The channel number, and the 50-byte memory record ---
//
// THE SLOT SPACE THIS FAKE SERVES IS THE FLAT 00-99, AND THAT IS THE WHOLE
// OF WHAT A FRAME CAN NAME. Byte 4 is a printed constant on this radio —
// "Always 0 for the TS-480." (480:910, 480:953) — where the 590 pair carry
// the channel's hundreds digit, so the channel number is P3's two digits
// alone, "00 ~ 99: Memory channel number" (480:912, 480:955), and MC's own
// P1 says the same of the bank: "0: Always 0 for the TS-480 (Memory bank
// number)." (480:827).
//
// THERE IS THEREFORE NO OUT-OF-DOMAIN CHANNEL NUMBER TO REFUSE. Every number
// a well-formed frame can spell is a channel this fake serves; a frame
// reaching past 99 needs a third digit and dies as a width violation
// (TestMR_TheWholeSlotSpaceIsExpressibleAndNothingElseIs). The 590 pair's
// 100-119 question does not arise on this row at all.
//
// CHANNELS 90-99 CARRY THE BOOK'S OWN P1 OVERLOAD — "Memory channel 90 ~ 99:
// P1=0 (start frequency), P1=1 (end frequency)" (480:943-944, 480:986-987) —
// and they are ORDINARY MEMORIES that also answer a second frame, not a
// separate bank: this radio's record has no bank field. Decision 15 gives the
// row one flat MEM bank of 00-99 and no scan bank, so the upper half is not a
// published slot and image.go ships no image for it; this parser still serves
// the frame, because the chart prints it.

// The served slot space's bounds, which are P3's own printed range
// (480:912): "00 ~ 99".
const (
	lowestChannel  = 0
	highestChannel = 99
)

// The 50-byte record's field offsets, 0-indexed into the whole frame, counted
// off the position rulers both charts print (480:923-943, 480:955-976). The
// comment on each names the chart's own 1-indexed position.
//
// THE GEOMETRY IS THE 590 PAIR'S AND THE MEANINGS ARE NOT: byte 19 is the
// lockout here (480:920) where they carry the data mode, bytes 39-40 are the
// tuning step (480:937) where they carry an FM bandwidth flag, and bytes 4,
// 28 and 41 are printed constants (480:910, 480:931, 480:939) where they
// carry the hundreds digit, the FILTER selection and the lockout.
const (
	recLen = 50 // the ';' at position 50 (480:943)

	recP1Off       = 2 // position 3
	recP2Off       = 3 // position 4
	recP3Off       = 4 // positions 5-6
	recP3Len       = 2
	recFreqOff     = 6 // positions 7-17
	recFreqLen     = 11
	recModeOff     = 17 // position 18
	recLockoutOff  = 18 // position 19
	recToneModeOff = 19 // position 20
	recToneNoOff   = 20 // positions 21-22
	recCTCSSNoOff  = 22 // positions 23-24
	recP10Off      = 24 // positions 25-27
	recP11Off      = 27 // position 28
	recP12Off      = 28 // position 29
	recP13Off      = 29 // positions 30-38
	recStepOff     = 38 // positions 39-40
	recStepLen     = 2
	recP15Off      = 40 // position 41
	recNameOff     = 41 // positions 42-49
	recNameLen     = 8
)

// The six runs both charts print as constants on this radio — three more than
// the 590 pair's record has — quoted from this book: P2 "Always 0 for the
// TS-480." (480:910, 480:953), P10 "Always 000 for the TS-480." (480:929,
// 480:971), P11 (480:931, 480:973), P12 (480:933, 480:975), P13 "Always
// 000000000 for the TS-480." (480:935, 480:977) and P15 (480:939, 480:982).
//
// THEY ARE REQUIRED ON THE SET SIDE, WHICH IS A CHOICE. This book's own
// general note permits a Set to fill a parameter "not applicable to this
// transceiver" with "any character except the ASCII control codes (00 to
// 1Fh) and the terminator (;)" (480:108-111), so strictness here is this
// programme's rule and the design's A24, not a deduction. A24's lift would
// turn it into "accepted and normalised".
const (
	printedP2  = "0"
	printedP10 = "000"
	printedP11 = "0"
	printedP12 = "0"
	printedP13 = "000000000"
	printedP15 = "0"
)

// mrReadLen is the read request's width, "M R P1 P2 P3 P3 ;" — seven
// positions (480:918).
const mrReadLen = 7

// mcSetLen is the MC Set and Answer width, "M C P1 P2 P2 ;" — six positions
// (480:830, 480:838).
const mcSetLen = 6

// --- Field validators (wire level) ---
//
// EVERY ONE OF THEM IS ENFORCED ON THE SET DIRECTION, and every one is
// ASSUMED to be what the radio itself enforces — doc.go's register entry
// SET-DIRECTION FIELD STRICTNESS. Nothing here normalises: a byte outside its
// printed legend is refused, never quietly corrected, because a driver that
// sent one would otherwise pass its own tests and fail on hardware.

// validP1 reports whether b is one of P1's two printed values, "0: RX
// frequency, 1: TX frequency" (480:908, 480:951).
func validP1(b byte) bool { return b == '0' || b == '1' }

// validModeByte reports whether b is a nibble the MD legend prints
// (480:843-854). ALL TEN ARE ADMITTED, including the two this book calls
// "Not used for the TS-480" — see handleMW.
func validModeByte(b byte) bool { return isDigit(b) }

// validLockoutByte: "Lockout status. 0: Lockout OFF, 1: Lockout ON."
// (480:920, 480:962) — byte 19, where the 590 pair carry the DATA-mode flag.
func validLockoutByte(b byte) bool { return b == '0' || b == '1' }

// validToneModeByte: "0: OFF, 1: TONE, 2: CTCSS" (480:922, 480:964). THREE
// values. The 590 pair print a fourth, "3: Cross Tone ON" (590:1467), and
// admitting it here would be one book's legend applied to the other radio —
// TestMW_RefusesTheToneModeTheSiblingPrints.
func validToneModeByte(b byte) bool { return b >= '0' && b <= '2' }

// validNameField reports whether the eight bytes of P16 are ones a name may
// carry. This book prints nothing about P16's charset beyond "A maximum of 8
// characters." (480:941), and forbids the control codes 00-1Fh and the
// terminator in any parameter generally (480:108-111, 480:127-129) — which
// does not exclude 0x7F or above. The bound applied here is A2's, whose claim
// stops at 0x7E: 0x7F and above is unevidenced AND unclaimed, and refusing it
// is this programme's own charset rule rather than a statement about the
// radio.
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
// admissible.
//
// THE BANK BYTE IS REQUIRED TO BE '0'. It is a printed constant on this radio
// (480:910, 480:953, 480:827), so the 590 pair's space-or-zero spelling rule
// (590:1334-1335) has no counterpart here and a space is refused —
// TestMR_RefusesABankByteOtherThanZero.
func parseChannelDigits(bank byte, twoDigits string) (int, bool) {
	if bank != printedP2[0] || len(twoDigits) != recP3Len || !allDigits(twoDigits) {
		return 0, false
	}
	n := int(twoDigits[0]-'0')*10 + int(twoDigits[1]-'0')
	// THE BOUND CANNOT FIRE ON THIS RADIO, and it is here anyway because a
	// bound is consulted from the same place as its datum — a standing rule of
	// this repository. Two digits cannot spell a number outside 00-99, so the
	// served space and the expressible space coincide; on the 590 pair the
	// same check is live, because their byte 4 carries a hundreds digit. If
	// recP3Len ever grew, this line is what would keep the range honest.
	if n < lowestChannel || n > highestChannel {
		return 0, false
	}
	return n, true
}

// --- MR: the memory read (480:906-944) ---
//
// X O O X: no Set — the chart prints a Set LABEL over an EMPTY chart
// (480:911), erratum E17 — a Read (480:918), a 50-byte Answer (480:923-943),
// no AI push.

// buildMRAnswer builds the 50-byte answer for one channel half.
func buildMRAnswer(channel int, half Half, s MemState) []byte {
	out := make([]byte, 0, recLen)
	out = append(out, 'M', 'R')
	out = append(out, byte(half))
	out = append(out, printedP2...)
	out = append(out, byte('0'+(channel/10)%10), byte('0'+channel%10))
	out = append(out, s.Freq...)
	out = append(out, s.Mode, s.Lockout, s.ToneMode)
	out = append(out, s.ToneNo...)
	out = append(out, s.CTCSSNo...)
	out = append(out, printedP10...)
	out = append(out, printedP11...)
	out = append(out, printedP12...)
	out = append(out, printedP13...)
	out = append(out, s.Step...)
	out = append(out, printedP15...)
	out = append(out, s.Name...)
	out = append(out, ';')
	return out
}

func (r *Radio) handleMR(body []byte) []byte {
	if len(body) != mrReadLen-3 {
		// Includes an MR frame in the 50-byte SET shape: this book gives MR
		// no Set direction at all (480:911 prints the label over an empty
		// chart), so such a frame is simply unknown.
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
		// (480:130-135). Not a claim that any TS-480 refuses MR.
		return rejection
	}

	r.mu.Lock()
	s, ok := r.records[recordKey{channel: channel, half: half}]
	r.mu.Unlock()
	if !ok {
		// THIS FAKE ASSERTS A4 HERE, AND A4 IS UNLIFTED. This document says
		// nothing about an empty channel anywhere — neither that an MR of one
		// answers rather than refusing, nor what the answer would hold — so
		// both halves of this branch are assumed, and the shape is the 590
		// pair's printed sentence (590:1492-1493) read across to this radio.
		// doc.go's register entry AN UNWRITTEN CHANNEL ANSWERS THE ZERO
		// RECORD; A4's lift, L-HW-3, is this row's RELEASE GATE.
		//
		// The same answer is given for the TRANSMIT half of a channel that
		// has no stored record, where the book prints nothing either (the
		// design's A9) — doc.go's register entry THE TRANSMIT HALF OF A
		// CHANNEL WITH NO STORED RECORD.
		s = emptyRecord()
	}
	return buildMRAnswer(channel, half, s)
}

// --- MW: the memory write (480:949-987) ---
//
// O X X X: a 50-byte Set and nothing else — the chart prints Read and Answer
// LABELS over EMPTY charts (480:980, 480:985), erratum E17 — so an accepted
// write is silent.
//
// THIS BOOK DESCRIBES NO ERASE FORM AT ALL. The sentence the 590 pair's
// document prints (590:1579-1581) has no counterpart here, so the 42-byte
// shape is not even a documented frame on this radio; it dies as a width
// violation like any other malformed width, and this programme builds no
// erase frame on any radio in any case. TestMW_RefusesEveryOtherWidth.

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
		Lockout:  frame[recLockoutOff],
		ToneMode: frame[recToneModeOff],
		ToneNo:   string(frame[recToneNoOff : recToneNoOff+2]),
		CTCSSNo:  string(frame[recCTCSSNoOff : recCTCSSNoOff+2]),
		Step:     string(frame[recStepOff : recStepOff+recStepLen]),
		Name:     string(frame[recNameOff : recNameOff+recNameLen]),
	}

	switch {
	case !allDigits(s.Freq):
		return rejection
	case !validModeByte(s.Mode):
		// ALL TEN NIBBLES ARE ADMITTED, the two this book calls "No mode
		// (Not used for the TS-480)" and "Tune (Not used for the TS-480)"
		// included (480:843, 480:853). Nothing says what a Set carrying one
		// does — that is the design's A18b, whose lift is a hardware trial —
		// so this fake stores the nibble rather than inventing a refusal,
		// which is what lets a test drive core/kw's own build refusal against
		// a real fake. doc.go's register entry A SET CARRYING AN UNUSED MODE
		// NIBBLE IS STORED.
		return rejection
	case !validLockoutByte(s.Lockout):
		return rejection
	case !validToneModeByte(s.ToneMode):
		return rejection
	case !allDigits(s.ToneNo) || !allDigits(s.CTCSSNo):
		// THE INDICES ARE STORED, NOT RANGE-CHECKED. TN prints "00 ~ 42"
		// (480:1557) and CN "00 ~ 41" (480:337), and this book says nothing
		// about what either rule does inside a memory frame — the design's
		// A21, unlifted. Refusing here would assert A21 as a fact about the
		// radio, so only the field's shape is enforced. doc.go's register
		// entry TONE INDICES ARE STORED, NOT RANGE-CHECKED.
		return rejection
	case !allDigits(s.Step):
		// THE STEP INDEX IS STORED, NOT RANGE-CHECKED, and this field is the
		// reason every TS-480 channel write in this programme is refused. P14
		// says only "Step size. Refer to the ST command." (480:937), and ST's
		// legend is MODE-CONDITIONAL over two different ranges (480:1494-1500)
		// with no printed "no change" value — the design's A22. A fake that
		// range-checked it would have to pick one of the two legends and would
		// be asserting A22 lifted. doc.go's register entry THE STEP INDEX IS
		// STORED, NOT RANGE-CHECKED.
		return rejection
	case string(frame[recP10Off:recP10Off+len(printedP10)]) != printedP10:
		return rejection
	case string(frame[recP11Off:recP11Off+len(printedP11)]) != printedP11:
		return rejection
	case string(frame[recP12Off:recP12Off+len(printedP12)]) != printedP12:
		return rejection
	case string(frame[recP13Off:recP13Off+len(printedP13)]) != printedP13:
		return rejection
	case string(frame[recP15Off:recP15Off+len(printedP15)]) != printedP15:
		return rejection
	case !validNameField(frame[recNameOff : recNameOff+recNameLen]):
		return rejection
	}

	r.mu.Lock()
	r.records[recordKey{channel: channel, half: half}] = s
	r.mu.Unlock()
	// A Set does not move the selection: nothing in the MW block mentions
	// it (480:949-987) — doc.go's register entry A SET DOES NOT MOVE THE
	// SELECTED CHANNEL.
	return nil // fire-and-forget success
}

// --- MC: the selected memory channel (480:825-838) ---
//
// O O O X. Set (480:830) and Answer (480:838) are six bytes, Read is "MC;"
// (480:834). Disambiguated by length.
//
// THIS FAKE MODELS NO FRONT PANEL, so the selection moves only by an MC Set
// and this fake never produces an MC answer naming a channel no host asked
// for. Nothing in the MC block conditions the selection on the channel
// holding anything, so an MC Set of an unwritten channel is accepted; what
// that channel HOLDS is A4's question, not this handler's.
//
// THERE IS NO ANSWER CONVENTION TO APPLY. P1 is the printed constant "0:
// Always 0 for the TS-480 (Memory bank number)." (480:827) in every
// direction, where the 590 pair print a space below channel 100 on an answer
// (590:1336-1337).

func buildMCAnswer(channel int) []byte {
	out := make([]byte, 0, mcSetLen)
	out = append(out, 'M', 'C')
	out = append(out, printedP2...)
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
	case [2]byte{'M', 'R'}:
		return r.handleMR(rest)
	case [2]byte{'M', 'W'}:
		return r.handleMW(rest)
	case [2]byte{'M', 'C'}:
		return r.handleMC(rest)
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
