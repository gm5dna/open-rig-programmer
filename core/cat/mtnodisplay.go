// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"strings"
)

// mtNoDisplayLen is THIS dialect's exact MTFormShortNoDisplay frame length:
// "MT"(2) + slot(slotDigits()) + tag(TagMaxBytes, fixed width) + ";"(1) —
// 20 for the FTX-1's 5-digit slot and 12-byte tag (FTX-1 spec.md §3.3/§8).
// A receiver method for mtCombinedLen's own reason: it reaches the OUTBOUND
// WRITE GATE (allowlist.go's validMTCommand) as well as the builder and
// parser below.
func (d Dialect) mtNoDisplayLen() int {
	return 2 + d.slotDigits() + d.mt.TagMaxBytes + 1
}

// mtNoDisplayTagOff is the offset of the fixed-width tag field: immediately
// after the slot field, since this form carries no display byte.
func (d Dialect) mtNoDisplayTagOff() int {
	return 2 + d.slotDigits()
}

// BuildMTSetNoDisplay builds an MTFormShortNoDisplay MT (memory channel
// tag) Set frame: slot then a FIXED-WIDTH tag field, no display byte at
// all. Reference: FTX-1 spec.md §3.3, "MT" + P0(5) + P1(12, tag) + ";".
//
// THE TAG FIELD IS ALWAYS EMITTED AT FULL WIDTH, padded with this
// dialect's TagFill — mtcombined.go's BuildMTSetCombined's own mechanism,
// reused here rather than reinvented, because this form's tag field is
// fixed-width for the same reason the combined form's is: the manual gives
// one exact byte count for the whole frame, not a floor/ceiling window (see
// MTFormShortNoDisplay's own doc comment, dialectconfig.go). An EMPTY tag
// is therefore the all-fill field, exactly as under the combined form.
//
// It refuses any dialect that is not MTFormShortNoDisplay, symmetric with
// BuildMTSet's and BuildMTSetCombined's own form refusals.
func (d Dialect) BuildMTSetNoDisplay(s Slot, tag string) (Command, error) {
	if d.mt.Form != MTFormShortNoDisplay {
		return Command{}, newParseError(nil, fmt.Sprintf("MT: no-display-form Set called on a %v dialect — use the matching form's API", d.mt.Form))
	}
	if !d.mtSlotValid(s) {
		return Command{}, newParseError([]byte(s.Wire()), d.mtSlotDomainRefusal())
	}
	if !d.validMTTag(tag) {
		return Command{}, newParseError([]byte(tag), fmt.Sprintf("MT: tag must be 0-%d bytes of printable ASCII 0x20-0x7E, excluding ';', with no control bytes", d.mt.TagMaxBytes))
	}
	if tag != "" && tag[len(tag)-1] == d.mt.TagFill {
		return Command{}, newParseError([]byte(tag), fmt.Sprintf("MT: tag must not end in the fill byte %q — the field is padded to %d bytes with it and trimmed of it on the way back, so a trailing fill byte would not round-trip; trim it (an all-fill tag is refused by this rule too, \"\" being its canonical spelling)", d.mt.TagFill, d.mt.TagMaxBytes))
	}

	frame := make([]byte, d.mtNoDisplayLen())
	frame[0], frame[1] = 'M', 'T'
	copy(frame[2:], s.Wire())
	field := frame[d.mtNoDisplayTagOff() : d.mtNoDisplayTagOff()+d.mt.TagMaxBytes]
	n := copy(field, tag)
	for i := n; i < len(field); i++ {
		field[i] = d.mt.TagFill
	}
	frame[len(frame)-1] = ';'
	return newCommand(frame), nil
}

// ParseMTAnswerNoDisplay strictly parses an MTFormShortNoDisplay MT
// Set/Answer frame into the slot it names and its tag. It refuses any
// dialect that is not MTFormShortNoDisplay, symmetric with ParseMTAnswer
// and ParseMTAnswerCombined.
func (d Dialect) ParseMTAnswerNoDisplay(frame []byte) (Slot, string, error) {
	if d.mt.Form != MTFormShortNoDisplay {
		return Slot{}, "", newParseError(frame, fmt.Sprintf("MT: no-display-form answer parsed on a %v dialect — use the matching form's API", d.mt.Form))
	}
	if want := d.mtNoDisplayLen(); len(frame) != want {
		return Slot{}, "", newParseError(frame, fmt.Sprintf("MT answer must be %d bytes", want))
	}
	if frame[0] != 'M' || frame[1] != 'T' {
		return Slot{}, "", newParseError(frame, "MT answer missing \"MT\" prefix")
	}
	if frame[len(frame)-1] != ';' {
		return Slot{}, "", newParseError(frame, "MT answer missing ';' terminator")
	}
	slot, err := d.ParseSlot(string(frame[2 : 2+d.slotDigits()]))
	if err != nil {
		return Slot{}, "", newParseError(frame, "MT answer: invalid slot field")
	}
	if d.classifySlot(slot.Wire()) == slotKindNone {
		return Slot{}, "", newParseError(frame, "MT answer: slot must not be the none form (reference MT column: ✗)")
	}
	tag := strings.TrimRight(string(frame[d.mtNoDisplayTagOff():d.mtNoDisplayTagOff()+d.mt.TagMaxBytes]), string(d.mt.TagFill))
	return slot, tag, nil
}
