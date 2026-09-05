// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import "fmt"

// MC IS THE CHART THE CHANNEL-NUMBER CONVENTION IS PRINTED IN, and every
// other frame in this codec borrows it by reference.
//
// MR's answer chart and MW's Set chart both say only "Channel number (refer
// to the MC command)" (590:1452-1453, 590:1539-1540), which is why A10
// exists: that MR and MW inherit MC's space-or-zero rule is assumed rather
// than printed. Here the rule is printed in full — "When entering a setting
// command, enter 0 or a space for a channel number less than 100. For a
// response command, a space is entered for a channel number less than 100."
// (590:1334-1337) — so this file states it once and slotWire (builders.go)
// renders it for all three commands. Two renderers a file apart would be one
// edit from disagreeing, and the disagreement would name a different channel.
//
// MC'S OWN PARAMETER LETTERING DIFFERS FROM MR/MW'S, and mixing the two is
// the transcription trap this comment exists to stop. In MC, P1 is the
// hundreds digit and P2 the two-digit remainder (590:1332-1343, 480:827-830);
// in MR and MW, P1 is the direction byte, P2 the hundreds digit and P3 the
// two digits. The offsets below are therefore named for their POSITIONS,
// not for their parameter letters.

// The MC frame lengths, counted off each book's printed position ruler.
const (
	// MCReadLen is "M C ;" (590:1337, 480:834).
	MCReadLen = 3
	// MCSetLen is "M C P1 P2 P2 ;" (590:1333, 480:830).
	MCSetLen = 6
	// MCAnswerLen is "M C P1 P2 P2 ;" (590:1341, 480:838) — the same six
	// bytes as the Set, which is precisely why the outbound gate judges a
	// six-byte MC frame against the SEND domain and never the answer's.
	MCAnswerLen = 6
)

// Byte offsets (0-indexed) into an MC Set or Answer frame, with each book's
// 1-indexed positions beside them.
const (
	mcPrefixOff   = 0 // positions 1-2, "MC"
	mcHundredsOff = 2 // position 3,    the 100's digit
	mcDigitsOff   = 3 // positions 4-5, the two-digit remainder
	mcTermOff     = 5 // position 6,    ';'
	mcDigits      = 2
)

// mcReadFrame is the whole of the MC Read chart on both radios.
const mcReadFrame = "MC;"

// BuildMCRead builds the memory-channel read, "MC;" (590:1337, 480:834).
//
// NOTHING IN THIS MILESTONE SENDS IT. Plan P13 reads memory with MR alone
// and states in as many words that "MC is never used to read"; this builder
// exists because the gate admits the frame — MC's Read chart is printed on
// both radios and the gate's job is to describe the grammars this codec
// knows, not to double as a scheduler — and because a gate admitting a frame
// no builder can produce is a gate nothing pins.
func (l Layout) BuildMCRead() (Command, error) {
	return l.buildFixedRead("MC read", mcReadFrame, MCReadLen)
}

// BuildMCSet builds the memory-channel RECALL for s: "M C P1 P2 P2 ;"
// (590:1333, 480:830).
//
// THE SEND DOMAIN IS NARROWED TO ORDINARY MEMORY, WHICH IS A16 (L-DEC-2)
// AND A DECISION RATHER THAN A DOCUMENT FACT. 590:1345-1347 says the section
// and extension numbers are selectable; this milestone declines to select
// them. Recalling a channel CHANGES THE RADIO'S OPERATING STATE, so the
// M9c-1 reasoning behind core/cat's MCSlotPolicy applies verbatim: the
// narrowing is recorded here so that a later widening is a reviewed decision
// rather than a drift, and the refusal below names A16 so a reader finds the
// entry rather than re-deriving the rule.
//
// The ANSWER domain is NOT narrowed — see ParseMCAnswer — and the two
// directions differing is the whole point.
func (l Layout) BuildMCSet(s Slot) (Command, error) {
	if !l.Configured() {
		return Command{}, newParseError(nil, "MC set: this layout is unconfigured and describes no radio")
	}
	// checkSlot first, so a slot minted under another layout is reported as
	// that rather than as a domain refusal: the two failures have different
	// repairs.
	if err := l.checkSlot("MC set", s); err != nil {
		return Command{}, err
	}
	if !mcSendValid(s) {
		return Command{}, newParseError(nil, "MC set: slot %v is %v on the %s, and this milestone narrows the MC SET domain to ordinary memory (A16, L-DEC-2) — recalling a channel changes the radio's operating state, and 590:1345-1347's selectable section and extension numbers are deliberately not selected; the ANSWER domain is the full printed space", s, s.Class(), l.model)
	}
	wire, err := l.slotWire(s)
	if err != nil {
		return Command{}, err
	}
	frame := make([]byte, 0, MCSetLen)
	frame = append(frame, 'M', 'C')
	frame = append(frame, wire...)
	frame = append(frame, ';')
	if len(frame) != MCSetLen {
		return Command{}, newParseError(frame, "MC set: built %d bytes, want exactly %d (590:1333, 480:830)", len(frame), MCSetLen)
	}
	return newCommand(frame), nil
}

// mcSendValid is the SEND-side slot predicate, shared by BuildMCSet and by
// the outbound gate (allowlist.go) so that "what the builder produces" and
// "what the gate admits" cannot drift apart.
//
// It is A16, and it is deliberately a function of the slot's CLASS rather
// than of its number: a row whose section channels started somewhere else
// would be narrowed correctly without this predicate being edited.
func mcSendValid(s Slot) bool { return s.Class() == SlotMemory }

// MCChannel is the channel one MC ANSWER names: its number, and the class
// this layout resolves that number to.
//
// IT IS NOT A Slot, AND THE DIFFERENCE IS THE WHOLE REASON THIS TYPE EXISTS.
// A Slot naming a section-defined channel is incomplete until it says which
// of the channel's two frequencies is meant (Slot.Half, record.go), because
// MR and MW carry a P1 byte that chooses. AN MC FRAME CARRIES NO SUCH BYTE:
// its three parameter positions are the hundreds digit and the two-digit
// remainder, and nothing else. Returning a Slot here would mean inventing a
// half — and a caller would then read "100L" off a frame that said only
// "100".
type MCChannel struct {
	// Number is the channel number the frame named.
	Number int
	// Class is what that number means in the answering layout's slot space.
	Class SlotClass
}

// String renders the channel as its three printed digits — and NEVER with a
// section channel's 'L'/'U' suffix, for the reason MCChannel exists.
func (c MCChannel) String() string { return fmt.Sprintf("%03d", c.Number) }

// ParseMCAnswer decodes "M C P1 P2 P2 ;" (590:1341, 480:838) against THIS
// LAYOUT'S slot space.
//
// THE ANSWER DOMAIN IS THE FULL PRINTED SPACE, and that asymmetry with
// BuildMCSet is deliberate: a radio sitting on a section channel reached
// from the front panel will answer with it, so a parser narrowed the way the
// builder is would fail a session on a legitimate frame. A16 narrows what
// this programme SENDS and says nothing about what a radio may say.
//
// A SPACE IS ADMITTED IN THE HUNDREDS POSITION on a row whose byte is the
// hundreds digit, and it is the form the book prints for an answer below 100
// (590:1336-1337). A '0' is admitted there too: the SET direction of the
// same field explicitly prints both (590:1334-1335), so accepting the digit
// is reading the chart rather than guessing, and it is the same rule
// parseSlot applies to MR's answer. On a row whose byte is a printed
// constant — "0: Always 0 for the TS-480" (480:827) — only '0' is admitted,
// because that book prints no space convention at all.
//
// The two low bytes are ALWAYS digits: "When the channel number is less than
// 10, both for setting and response commands, the first digit is 0"
// (590:1342-1343, 480:830).
func (l Layout) ParseMCAnswer(frame []byte) (MCChannel, error) {
	if err := l.checkAnswerShape("MC answer", frame, "MC", MCAnswerLen); err != nil {
		return MCChannel{}, err
	}

	hundreds := 0
	switch b := frame[mcHundredsOff]; l.p2 {
	case P2HundredsDigit:
		switch {
		case b == ' ':
			hundreds = 0
		case b >= '0' && b <= '9':
			hundreds = int(b - '0')
		default:
			return MCChannel{}, newParseError(frame, "MC answer: position 3 is %q; on the %s it is the channel's 100's digit, which the chart prints as a digit or a space below 100 (590:1332-1337)", b, l.model)
		}
	case P2FixedZero:
		if b != '0' {
			return MCChannel{}, newParseError(frame, "MC answer: position 3 is %q, and the %s prints \"0: Always 0 for the TS-480 (Memory bank number)\" there (480:827) — that book prints no space convention", b, l.model)
		}
	default:
		return MCChannel{}, newParseError(frame, "MC answer: the 100's-digit policy is unset on this layout — refusing to guess whether position 3 is the channel's hundreds digit or a printed constant")
	}

	digits := frame[mcDigitsOff : mcDigitsOff+mcDigits]
	for i, b := range digits {
		if b < '0' || b > '9' {
			return MCChannel{}, newParseError(frame, "MC answer: position %d is %q; the chart prints both digits of the channel number, zero-padded below 10 (590:1341-1343, 480:830)", mcDigitsOff+i+1, b)
		}
	}
	number := hundreds*100 + int(digits[0]-'0')*10 + int(digits[1]-'0')

	class := l.classOf(number)
	if class == SlotClassInvalid {
		return MCChannel{}, newParseError(frame, "MC answer: channel %d is outside the %s's slot space %s", number, l.model, l.slotSpaceText())
	}
	return MCChannel{Number: number, Class: class}, nil
}
