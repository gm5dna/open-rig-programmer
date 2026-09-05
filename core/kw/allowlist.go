// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"bytes"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// AllowedCommand reports whether frame is safe to write to the radio: it is
// EXACTLY one of the EIGHT command grammars this package knows how to build
// — ID read, AI read/set, FV read, TY read, MC read/set, MR read, MW set, EX
// read — fully re-validated field by field against the same rules the
// corresponding builder enforces, not merely a two-byte command-name prefix.
//
// IT IS core/cat's AllowedCommand DISCIPLINE, COPIED RATHER THAN IMPORTED.
// That gate is hard-coded to seven Yaesu grammars over a cat.Dialect and
// cannot be parameterised into this; the fence (imports_test.go) forbids
// borrowing it anyway. What is copied is the discipline, verbatim:
//
//   - EVERY CHECK SHARES THE BUILDER'S OWN HELPERS rather than duplicating
//     them — parseRecordFrame, BuildMWSet, BuildMRRead, parseMCFields,
//     mcSendValid, exReadAddress — so "what this admits" and "what the
//     builders produce" cannot drift apart. Two copies of a field rule would
//     be one edit from disagreeing, and the disagreement that matters is the
//     one where the parser refuses a frame and the gate admits it.
//   - NO ANSWER FRAME IS ADMITTED EXCEPT WHERE A SET THIS CODEC BUILDS IS
//     BYTE-IDENTICAL TO IT, and there are exactly two such cases, both
//     unavoidable and both named where the check is made: "AI0;"
//     (validAICommand) and an MC Set naming ORDINARY MEMORY, 000-099
//     (validMCCommand), which are indistinguishable on the wire from the
//     answers a radio sends for the same state (590:1333 against 590:1341).
//     Every OTHER inbound shape that coincides with a Set stays refused —
//     EX's Answer is its Set's shape, the 50-byte MR answer is the MW Set's
//     grid with a different prefix, and an MC naming a section or extension
//     channel is refused however well formed — because admitting one would
//     let a captured reply be written back.
//   - AN EMBEDDED ';' IS REFUSED even if a prefix matches: exactly one
//     command must reach the wire per call, and a second terminator anywhere
//     splits one frame into two on the radio's own parser.
//   - A ZERO LAYOUT FAILS CLOSED. A zero Layout is constructible by any
//     caller and its AllowedCommand is a non-nil method value, so nothing
//     about the method's existence says which radio it speaks for — or that
//     it speaks for one at all. The slot-aware checks give the refusal for
//     free wherever slot data is consulted (an empty slot space matches no
//     slot), and the EX read is closed for every address but 000 by a zero
//     MaxEXAddress — but "ID;", "AI;", "AI0;", "MC;" and "EX0000000;"
//     consult no layout datum they can fail on, and the Configured guard is
//     what closes them.
//
// IT GATES FOR THE LAYOUT IT IS CALLED ON, AND FOR NO OTHER — but not every
// check below reads the receiver in the same way, and the three tiers are a
// genuine property of the gate worth stating rather than an accident: MR,
// MW, MC's Set half and the EX read are PER-RADIO (slot space, mode legend,
// the four byte-28/39-40/41 policies, P2 policy, and the printed menu
// domain); FV and TY are PER-BOOK; ID, AI and MC's read half are
// PER-FAMILY, because there is nothing in any of those three frames to
// vary. What is true on every tier, and is the safety point this gate
// exists for, is that a frame legal on one Kenwood row is refused by a
// layout describing another: a TS-480 MW whose P14 is an ST step index is
// refused by a 590 row, a read of extension channel 110 is refused by the
// TS-590S (A12), an EX read of menu 088 is refused by the TS-590S and
// admitted by the TS-590SG (590:543 against 590:544), "FV;" is refused by
// the 480 and "TY;" by the 590 pair. A gate that re-validated against a
// package-level datum would accept, on any radio, whatever one radio
// accepts — a safety failure rather than merely a correctness one.
//
// THE ONE PLACE THE ADMITTED SET IS WIDER THAN THE BUILDER SET, and it is
// deliberate: MC's own chart PRINTS both spellings of the hundreds digit,
// "enter 0 or a space for a channel number less than 100" (590:1334-1335),
// so the gate admits a space there although the builder always emits '0'.
// Refusing a spelling the manufacturer prints would be this codec inventing
// a rule. MR and MW get the OPPOSITE answer, and the difference is the line
// this gate draws: those two charts say only "refer to the MC command"
// (590:1452-1453, 590:1539-1540), so that they share the convention at all
// is A10 — ASSUMED — and a gate does not widen itself on an assumption. The
// MW check re-encodes and therefore refuses a space-spelled one.
//
// ADDING A COMMAND, OR LOOSENING ANY CHECK BELOW, IS A REVIEWED DECISION:
// this is the last defence before bytes reach a physical radio. Every
// builder's own output MUST satisfy AllowedCommand — enforced by
// TestAllowedCommand_AcceptsExactlyTheEightGrammars and
// TestAllowedCommand_AdmitsOnlyFramesTheEnvelopeAlsoAdmits — and every
// answer frame in this package's tests MUST NOT.
func (l Layout) AllowedCommand(frame []byte) bool {
	if !l.Configured() {
		return false
	}
	if len(frame) < IDReadLen { // the shortest legal frame is "ID;"
		return false
	}
	if !exactlyOneTrailingSemicolon(frame) {
		return false
	}

	switch string(frame[:2]) {
	case "ID":
		return l.validIDCommand(frame)
	case "AI":
		return l.validAICommand(frame)
	case "FV":
		return l.validFVCommand(frame)
	case "TY":
		return l.validTYCommand(frame)
	case "MC":
		return l.validMCCommand(frame)
	case "MR":
		return l.validMRCommand(frame)
	case "MW":
		return l.validMWCommand(frame)
	case "EX":
		return l.validEXRead(frame)
	default:
		return false
	}
}

// exactlyOneTrailingSemicolon reports whether frame contains exactly one ';'
// byte and that byte is the very last one.
//
// BOTH HALVES ARE LOAD-BEARING. A count-only check would admit ";ID" and a
// suffix-only check would admit "ID;ID;", which is two commands the radio
// would execute in turn — the injection this gate exists to refuse.
func exactlyOneTrailingSemicolon(frame []byte) bool {
	count, pos := 0, -1
	for i, b := range frame {
		if b == ';' {
			count++
			pos = i
		}
	}
	return count == 1 && pos == len(frame)-1
}

// validIDCommand admits the ID READ and nothing else. ID has no Set form on
// either radio and BuildIDRead produces exactly this one frame, so a literal
// match is a complete re-validation rather than a shortcut.
//
// THE SIX-BYTE ANSWER IS WHAT THIS REFUSES, and it is worth naming: "ID023;"
// is a perfectly well-formed frame that a radio sends, and a Yaesu one
// ("ID0800;", seven bytes, four digits) is a well-formed frame of another
// family. Neither is ever written by this host.
func (l Layout) validIDCommand(frame []byte) bool { return string(frame) == idReadFrame }

// validAICommand admits the AI READ and the ONE Set this codec builds,
// "AI0;" — and no other AI state, which is the plan's negative pin.
//
// The two books' legends differ (0/2/4 against 0/1/2/3, ai.go), so there is
// no non-zero value that means the same on both; every non-zero value in
// either set turns Auto Information ON, and an AI-ON radio pushes frames
// nobody asked for into a session correlating answers by prefix and length.
// The Set and the Answer share these four bytes exactly, so admitting "AI0;"
// admits its own answer with it — an acknowledged and unavoidable exception,
// written down here rather than left to be rediscovered.
func (l Layout) validAICommand(frame []byte) bool {
	switch string(frame) {
	case aiReadFrame, initFrame:
		return true
	default:
		return false
	}
}

// validFVCommand admits "FV;" on a row whose book prints it, and nothing on
// the TS-480, whose 2003 document has no FV command anywhere.
func (l Layout) validFVCommand(frame []byte) bool {
	return l.book == Book590 && string(frame) == fvReadFrame
}

// validTYCommand admits "TY;" on the TS-480 alone, TY appearing nowhere in
// the TS-590S/SG document.
func (l Layout) validTYCommand(frame []byte) bool {
	return l.book == Book480 && string(frame) == tyReadFrame
}

// validMCCommand admits the fixed "MC;" read, or a six-byte Set whose
// channel this layout resolves to ORDINARY MEMORY.
//
// A16 (L-DEC-2) NARROWS THE SEND-SIDE DOMAIN TO ORDINARY MEMORY, and that is
// the right domain to gate on — but it does not make this check able to
// tell a Set from an Answer. "MC003;" is byte-identical to both a Set
// naming channel 3 and the Answer a radio sitting on ordinary channel 3
// sends (590:1333 against 590:1341, six bytes either way), and this is the
// second of the gate's two disclosed answer-admissions (AllowedCommand's
// doc comment above names both, beside "AI0;"): AN MC ANSWER NAMING
// ORDINARY MEMORY IS ADMITTED, because it is what BuildMCSet emits.
// A16 makes the admitted MC set a STRICT SUBSET of the wider answer domain
// rather than a disjoint one — every MC answer naming a section-defined or
// extension channel is refused here, because mcSendValid narrows to
// ordinary memory, and that refusal is what A16 actually buys: judging this
// frame by ParseMCAnswer's wider domain instead of mcSendValid's narrower
// one would let a side-effecting recall of a section-defined or extension
// channel be admitted by its own gate.
//
// It decodes through parseMCFields, the shape decoder both directions share,
// and then applies mcSendValid, the same predicate BuildMCSet applies.
func (l Layout) validMCCommand(frame []byte) bool {
	if string(frame) == mcReadFrame {
		return true
	}
	if len(frame) != MCSetLen {
		return false
	}
	ch, err := l.parseMCFields("MC set", frame)
	if err != nil {
		return false
	}
	return mcSendValid(ch.Class)
}

// validMRCommand admits a seven-byte MR READ that THIS LAYOUT'S OWN BUILDER
// would have produced, byte for byte.
//
// THE RE-ENCODE IS THE CHECK, and it buys two properties a field-by-field
// comparison would not. It refuses the 50-byte MR ANSWER outright, since no
// builder emits one. And it enforces M9 — P1 is derived from the SLOT'S
// CLASS and never chosen freely (Slot.P1) — so "MR1007;", a read of
// ordinary memory channel 007 with P1='1', is refused. THE HONEST REASON IS
// NARROWER THAN "SIMPLEX": P1=1 is the DOCUMENTED read of the transmit
// frequency of a SPLIT channel (590:1444-1447, "0: RX frequency, 1: TX
// frequency" 480:951) and "MR1007;" is exactly that printed read whenever
// channel 007 happens to be split. What this gate cannot know — because a
// Layout's SlotMemory class carries no split/simplex property — is which
// kind of channel 007 is, so it refuses rather than guess. A9 records the
// same refusal, worded "an MR with P1=1 on a SIMPLEX channel", which is
// unprinted on both radios and is not safe to send blind; the frame that
// wording names is a strict SUBSET of what this check actually refuses. The
// section channel's own "MR1100;" is admitted, because there P1='1' is the
// DOCUMENTED read of the end frequency (590:1449-1451) and section-defined
// channels are never ordinary memory, so no such ambiguity arises.
func (l Layout) validMRCommand(frame []byte) bool {
	if len(frame) != MRReadLen {
		return false
	}
	p1 := frame[recP1Off]
	if p1 != '0' && p1 != '1' {
		return false
	}
	slot, err := l.parseSlot("MR read", frame, p1)
	if err != nil {
		return false
	}
	cmd, err := l.BuildMRRead(slot)
	if err != nil {
		return false
	}
	return bytes.Equal(cmd.Bytes(), frame)
}

// validMWCommand admits a 50-byte MW SET that THIS LAYOUT'S OWN BUILDER
// would have produced, byte for byte.
//
// THE FRAME IS DECODED, VALIDATED AND RE-ENCODED, which is core/civ's rule
// and is what makes "admits only builder-producible frames" literally true
// rather than approximately. parseRecordFrame applies every field rule the
// answer parser applies — the printed-fixed bytes, the mode legend, the tone
// index charts, the byte-19/28/39-40/41 policies, A2's name charset — and
// BuildMWSet then applies the WRITE-direction rules on top: A18b's mode
// nibbles, the P1-from-slot-class refusal, and the refusal of an EMPTY
// record, because the only documented way to clear a channel is the short MW
// of 590:1579-1581 and this milestone never builds it.
//
// EVERY WIDTH BUT FIFTY IS ALREADY GONE by then, through the one predicate
// both directions consult (checkRecordLen), which is what keeps the erase
// form out: its length is a reading rather than a printed number (A5,
// erratum E19) and no frame of any other width reaches a field check here.
func (l Layout) validMWCommand(frame []byte) bool {
	rec, err := l.parseRecordFrame("MW", "MW set", frame)
	if err != nil {
		return false
	}
	cmd, err := l.BuildMWSet(rec)
	if err != nil {
		return false
	}
	return bytes.Equal(cmd.Bytes(), frame)
}

// validEXRead admits the ten-byte EX READ and refuses the Set and the Answer
// entirely.
//
// EX IS DELIBERATELY NARROWER THAN THE OTHER SEVEN, and it is not a phase
// restriction here: this milestone reads the menu surface and writes none of
// it, so there is no EX Set builder for the gate to be narrower than. The
// Set and the Answer are the ten fixed bytes with P5 inserted before the
// terminator (590:543-560, 480:403-416), so admitting that shape would admit
// a captured answer being written back into a radio's menu.
//
// exReadAddress is the shared decoder (ex.go), and it compares a RE-RENDERED
// address with the field rather than trusting the digits, so no frame this
// gate admits is one BuildEXRead could not have emitted.
//
// THE ADDRESS IS THEN BOUNDED BY THIS ROW'S OWN PRINTED MENU DOMAIN, read
// from the same axis BuildEXRead reads (Layout.MaxEXAddress, transcribed
// from 590:543, 590:544 and 480:401). exReadAddress is the FORM decoder and
// is deliberately layout-free — 000 to 255 is what an EXAddress can hold —
// so this is where the row's book comes in, and it is what keeps EX on the
// same per-row footing as MR, MW and MC's Set half rather than leaving it
// the one grammar a sibling row's frame could pass.
func (l Layout) validEXRead(frame []byte) bool {
	addr, err := exReadAddress(frame)
	return err == nil && addr.P1 <= l.maxEXAddress
}

// NewFramingFor returns the transport.Framing for a CONFIGURED LAYOUT: the
// same adapter NewFraming builds for that layout's book, with the outbound
// gate narrowed from the envelope to the eight grammars.
//
// THIS IS THE CONSTRUCTOR EVERY DRIVER USES, and the difference from
// NewFraming is the whole of it. NewFraming(book) knows which document a
// session speaks and therefore which cause sentence an "O;" carries (E13),
// but it does not know which RADIO — the layout axes are per row — so its
// Allow can only be the envelope both books print: a terminator, exactly
// one, as the last byte; two upper-case name bytes; printable interior; no
// radio-to-host token; no frame past DefaultMaxFrame. That is a true
// statement about what a Kenwood frame looks like and it is not a grammar.
// A session opened through it would admit, for instance, the 42-byte erase
// shape of 590:1579-1581 — a frame the book really prints and this programme
// really never builds.
//
// THE GRAMMARS SIT IN FRONT OF THE ENVELOPE RATHER THAN REPLACING IT. The
// conjunction costs nothing (the grammars are strictly narrower —
// TestAllowedCommand_AdmitsOnlyFramesTheEnvelopeAlsoAdmits is the pin) and
// it means the envelope's rules stay in force on the day a grammar is
// widened, which is when they matter most.
//
// IT IS A SECOND TYPE RATHER THAN AN OPTIONAL FIELD ON THE FIRST, and that
// is deliberate. A framing with a Layout field left unset would be a
// perfectly usable value whose gate had silently fallen back to the envelope
// — a weaker gate reached by forgetting something, which is the failure
// shape this package refuses everywhere else. Here the only way to hold a
// layoutFraming is to have passed a configured layout to this constructor.
func NewFramingFor(l Layout) (transport.Framing, error) {
	if !l.Configured() {
		return nil, fmt.Errorf("%w: the layout is unconfigured and describes no radio, so its outbound gate would speak for none", ErrLayoutInvalid)
	}
	base, err := NewFraming(l.Book())
	if err != nil {
		return nil, err
	}
	f, ok := base.(framing)
	if !ok {
		return nil, fmt.Errorf("%w: NewFraming returned a %T", ErrLayoutInvalid, base)
	}
	return layoutFraming{framing: f, layout: l}, nil
}

// layoutFraming is the book framing with the per-radio outbound gate.
//
// It EMBEDS framing rather than reimplementing it, so IsRejection, IsFatal,
// InitSequence, DrainPolicy, NoteSent and NewAccumulator are the shipping
// ones by construction and cannot drift; only Allow is overridden. The
// compiler assertions below are what say so.
type layoutFraming struct {
	framing
	layout Layout
}

var (
	_ transport.Framing     = layoutFraming{}
	_ transport.FatalFramer = layoutFraming{}
)

// Allow is the eight grammars AND the documented envelope.
//
// A zero layoutFraming admits nothing by either half: its layout is
// unconfigured and its book is unset, so neither term can be true.
func (f layoutFraming) Allow(frame []byte) bool {
	return f.layout.AllowedCommand(frame) && f.framing.Allow(frame)
}
