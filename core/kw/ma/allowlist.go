// SPDX-License-Identifier: GPL-3.0-or-later

package ma

import (
	"bytes"

	"github.com/gm5dna/open-rig-programmer/core/kw"
)

// AllowedCommand reports whether frame is safe to write to the radio: it is
// EXACTLY one of the SEVEN command grammars across FIVE opcodes this package
// knows how to build — ID read, AI read, AI Set to 0 ONLY, FV read, EX read,
// MA0 Read, MA0 Set — fully re-validated field by field against the same
// rules the corresponding builder enforces, not merely a two-byte command-name
// prefix. That roster is spec decision 5, and decision 15 is what keeps
// everything else off it.
//
// IT IS core/cat's AllowedCommand DISCIPLINE, COPIED THROUGH core/kw'S OWN
// COPY OF IT (core/kw/allowlist.go), never imported: the fence
// (imports_test.go) forbids reaching into another family, and core/kw's is
// hard-coded to eight grammars over a kw.Layout, which no ma.Layout is
// (decision 1). What is copied is the discipline, verbatim:
//
//   - EVERY CHECK SHARES THE BUILDER'S OWN HELPERS rather than duplicating
//     them — parseSlotField, BuildMA0Read, ParseMA0Answer, BuildMA0Set,
//     BuildEXRead — so "what this admits" and "what the builders produce"
//     cannot drift apart. Two copies of a field rule would be one edit from
//     disagreeing, and the disagreement that matters is the one where the
//     parser refuses a frame and the gate admits it. There is NO second
//     validator in this file.
//   - NO ANSWER FRAME IS ADMITTED EXCEPT WHERE A SET THIS CODEC BUILDS IS
//     BYTE-IDENTICAL TO IT, and here there are exactly TWO such cases, both
//     unavoidable and both named where the check is made: "AI0;", whose Set
//     and Answer share those four bytes (890:175-181, 990:173-178), and the
//     MA0 SET, because BOTH BOOKS DRAW ONE GRID FOR SET AND ANSWER ALIKE
//     (890:3166-3182 Set against 890:3187-3204 Answer; 990:2893-2915 against
//     990:2919-2938) — see validMA0Set, where that admission is what makes a
//     captured MA0 reply writable back into the slot it came from. Every
//     OTHER inbound shape stays refused: the ID answer carries three digits
//     where the read carries none, the EX answer splices P4 and P5 in before
//     its terminator, and a BLANK MA0 answer is refused outright because
//     BuildMA0Set refuses the record the parser marks Empty (the standing no
//     erase rule, which declines even the printed MA5 of 890:3305-3311,
//     990:3042-3047).
//   - AN EMBEDDED ';' IS REFUSED even where a prefix matches: exactly one
//     command must reach the wire per call, and a second terminator anywhere
//     splits one frame into two on the radio's own parser.
//   - A ZERO LAYOUT FAILS CLOSED. A zero Layout is constructible by any
//     caller and its AllowedCommand is a perfectly non-nil method value, so
//     nothing about the method's existence says which radio it speaks for —
//     or that it speaks for one at all. The slot-aware and inventory-aware
//     arms give that refusal for free (an empty slot space resolves no
//     channel; an empty inventory contains no address), but "ID;", "AI;",
//     "AI0;" and "FV;" consult no layout datum they can fail on, and the
//     Configured guard is what closes them.
//
// THE EX ARM ASKS INVENTORY MEMBERSHIP, NEVER A SCALAR BOUND, and that is
// the one place this gate is stricter than core/kw's could be. core/kw bounds
// an EX read with a maximum address because pair 1's books print a CONTIGUOUS
// menu domain; neither of these books prints one — both charts are sparse,
// with gaps inside every category — so a ceiling would admit an address no
// chart prints, breaching the standing rule that every frame this programme
// sends is one the book describes. core/kw/ex.go records why core/kw itself
// could not hold membership: its rows' inventories live in the packages that
// IMPORT it, so a check there would be an import cycle. THAT OBSTACLE DOES
// NOT EXIST HERE — both layouts and both generated inventories are in this
// package — so the gate asks Layout.EXItem, through BuildEXRead, exactly as
// the builder and the parser do. No diagnostic maximum is carried.
//
// IT GATES FOR THE LAYOUT IT IS CALLED ON, AND FOR NO OTHER, and the tiers
// are worth stating because they are not uniform: the MA0 Set and the EX read
// are PER-RADIO (the grid, the mode legend, the lockout encoding, the tone
// tuples, the channel-type byte, and this chart's own menu addresses); ID's
// digits are per-radio too but no READ carries them; and ID, AI, FV and the
// MA0 Read are PER-FAMILY, because there is nothing in those four frames to
// vary — both books print them identically (890:2735/181/2655/3184-3186,
// 990:2614/179/2532/2916-2918). A blanket cross-refusal is therefore
// impossible, which is why the conformance walk has two tiers
// (TestAllowedCommand_TheTwoTierConformanceWalk).
//
// ADDING A COMMAND, OR LOOSENING ANY CHECK BELOW, IS A REVIEWED DECISION:
// this is the last defence before bytes reach a physical radio.
// TestAllowedCommand_AdmitsExactlyTheSevenGrammars requires every builder's
// own output to pass, TestAllowedCommand_RefusesTheWholeNegativeRoster
// requires every MA1-MA7, MI, MN, MV, QA, QD, QI, MR, MW, MC and TY frame the
// books print to fail, and internal/guards' TestMAFramingGatesTheRoster drives
// both corpora through the framing NewFramingFor really returns. THIS METHOD
// AND THAT GUARD DO NOT SUBSTITUTE FOR ONE ANOTHER: a perfect roster wired to
// the wrong constructor would pass everything in this package.
func (l Layout) AllowedCommand(frame []byte) bool {
	if !l.Configured() {
		return false
	}
	if len(frame) < kw.IDReadLen { // the shortest legal frame is "ID;"
		return false
	}
	if !exactlyOneTrailingSemicolon(frame) {
		return false
	}

	switch string(frame[:2]) {
	case "ID":
		return string(frame) == idReadFrame
	case "AI":
		// The read, and the ONE Set this codec builds. No other AI state:
		// the two books' legends differ (890:175-181, 990:173-178) and
		// every non-zero value in either turns Auto Information ON, which
		// pushes unasked-for frames into a session that correlates answers
		// by prefix and length.
		return string(frame) == aiReadFrame || string(frame) == aiSetFrame
	case "FV":
		// BOTH books print FV, which is the difference from core/kw's arm:
		// there the TS-480 has no FV command at all, so that gate tests the
		// book. Here neither row refuses the other's (890:2650-2659,
		// 990:2527-2536).
		return string(frame) == fvReadFrame
	case "EX":
		return l.validEXRead(frame)
	case "MA":
		return l.validMA0Command(frame)
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
//
// IT IS THIS PACKAGE'S OWN because core/kw's is unexported. One line on
// stdlib's bytes.IndexByte; the alternative, exporting kw's, would put a gate
// internal on the family's public surface for no caller's benefit.
func exactlyOneTrailingSemicolon(frame []byte) bool {
	i := bytes.IndexByte(frame, ';')
	return i >= 0 && i == len(frame)-1
}

// validEXRead admits the eight-byte EX READ whose address is in THIS ROW's
// inventory, and refuses the Set and the Answer entirely.
//
// THE RE-ENCODE THROUGH BuildEXRead IS THE CHECK, and it buys three things a
// field comparison would not. The membership question is asked from the same
// place as its datum, so the builder and the gate cannot disagree about what
// this radio has. The address is re-RENDERED and compared, so no spelling
// WireEXAddress would not have produced is admitted. And the ANSWER is refused
// by construction: it carries P4 and P5 before its terminator and is never
// eight bytes (890:1909-1913, 990:1738-1747).
//
// EX IS NARROWER THAN THE OTHER SIX AND THAT IS NOT A PHASE RESTRICTION: this
// milestone reads the menu surface and writes none of it, so there is no EX
// Set builder for this arm to be narrower than.
func (l Layout) validEXRead(frame []byte) bool {
	if len(frame) != EXReadLen {
		return false
	}
	p1, err1 := decodeDigits("EX P1", frame[exAddrOff:exAddrOff+1])
	p2, err2 := decodeDigits("EX P2", frame[exAddrOff+1:exAddrOff+3])
	p3, err3 := decodeDigits("EX P3", frame[exAddrOff+3:exAddrOff+exAddrLen])
	if err1 != nil || err2 != nil || err3 != nil {
		return false
	}
	cmd, err := l.BuildEXRead(kw.EXAddress{P1: uint8(p1), P2: uint8(p2), P3: uint8(p3)})
	return err == nil && bytes.Equal(cmd.Bytes(), frame)
}

// validMA0Command splits the family's one admitted member from the eight
// refused ones, and the third byte is the whole of that split.
//
// MA1-MA7, MI, MV AND MN ARE ALL REFUSED HERE OR ON THE DEFAULT ARM ABOVE,
// which is spec decision 15 landing in one comparison: MA5's Set is
// "M A 5 P1 P1 P1 ;" (890:3305-3311, 990:3042-3047), seven bytes, the MA0
// Read's shape with a different third byte, and it is the printed ERASE this
// programme's standing no-erase rule declines. A gate keyed on "MA" and a
// length would admit it.
func (l Layout) validMA0Command(frame []byte) bool {
	if frame[2] != '0' {
		return false
	}
	if len(frame) == ma0ReadLen {
		return l.validMA0Read(frame)
	}
	return l.validMA0Set(frame)
}

// validMA0Read admits a seven-byte MA0 READ that THIS LAYOUT'S OWN builder
// would have produced, byte for byte (890:3184-3186, 990:2916-2918).
//
// The re-encode enforces this row's SLOT SPACE — a channel number outside
// 000-119 resolves to no class and is refused before any frame is built — and
// it is what keeps the read on the same per-radio footing as the Set, on the
// day the two rows' slot domains stop being identical.
func (l Layout) validMA0Read(frame []byte) bool {
	s, err := l.parseSlotField(frame)
	if err != nil {
		return false
	}
	cmd, err := l.BuildMA0Read(s)
	return err == nil && bytes.Equal(cmd.Bytes(), frame)
}

// validMA0Set admits an MA0 SET that THIS LAYOUT'S OWN encoder would have
// produced, byte for byte, on this row's own grid.
//
// THE FRAME IS DECODED, VALIDATED AND RE-ENCODED, which is core/civ's rule and
// is what makes "admits only builder-producible frames" literally true rather
// than approximately: ParseMA0Answer applies every field rule the answer
// parser applies — the slot space, the eleven-digit frequencies, the mode
// legend, the tone charts, the tone type, this row's own lockout encoding
// (E8), A2's name charset — and BuildMA0Set then applies the WRITE-direction
// rules on top: the refusal of a record carrying a field this grid has no
// position for, the P4/P10 agreement rule on the 890S (890:3219-3221), the
// zeroed-secondary-side pairing (A16), and the refusal of an EMPTY record.
//
// IT ADMITS A CAPTURED MA0 ANSWER, AND THAT IS DISCLOSED RATHER THAN AVOIDED.
// Both books draw ONE grid for Set and Answer (890:3166-3182 against
// 890:3187-3204; 990:2893-2915 against 990:2919-2938), so a well-formed answer
// for channel 007 is byte-identical to the Set that writes what it reports —
// there is no byte to tell them apart, and refusing the shape would refuse the
// only Set this milestone builds. What is NOT admitted is the answer that
// matters: a BLANK channel's, whose window of spaces or zeros parses as an
// unassigned record and which BuildMA0Set refuses outright, so the erase this
// programme never sends cannot arrive wearing a Set's clothes.
//
// EVERY WIDTH BUT THIS ROW'S IS ALREADY GONE by the time a field is looked at,
// through each codec's own length check — 40 to 50 bytes on the TS-890S, whose
// terminator floats under a ruler head printed "x" (890:3181-3182), and
// exactly 57 on the TS-990S (990:2915) — which is what makes the two rows
// cross-refuse each other's Set frames on the length alone.
func (l Layout) validMA0Set(frame []byte) bool {
	rec, err := l.ParseMA0Answer(frame)
	if err != nil {
		return false
	}
	cmd, err := l.BuildMA0Set(rec)
	return err == nil && bytes.Equal(cmd.Bytes(), frame)
}
