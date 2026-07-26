// SPDX-License-Identifier: GPL-3.0-or-later

package cat

// mcSetLen is the fixed length of an MC set/answer frame: "MC" + 3-byte
// slot + ";". Golden vector G11: "MC099;".
const mcSetLen = 6

// mcReadFrame is the fixed MC read request. Reference: "Read: MC; ->
// Answer MC P1 P1 P1;".
const mcReadFrame = "MC;"

// mcValid reports whether s is a legal MC target UNDER THIS DIALECT'S slot
// space: memory, PMS, 60m or EMG.
// Reference slot table's "MC set" column is ✗ only for "000" (semantics
// unknown, do not emit); 001-099, P1L-P9U, 5xx and EMG are all ✓.
//
// ASSUMED: the answer to an MC read shares the identical 6-byte shape as
// the Set frame (reference: "Read: MC; -> Answer MC P1 P1 P1;"), and the
// manual documents no separate slot domain for the answer, so
// ParseMCAnswer applies this same restriction rather than accepting "000"
// there.
//
// M5a (13/07/2026, docs/hardware-notes.md §MC-answer semantics)
// PARTIALLY resolves this: with the radio sitting on M-06, "MC;" ->
// "MC006;" — a 3-digit memory slot, exactly the shape this parser
// already accepts, HW-CONFIRMED. What remains open: whether "MC;" can
// ever legitimately answer "MC000;" when the radio is in VFO mode (not
// on any stored memory) — this session never put the radio in that state
// and queried MC directly, so ParseMCAnswer's rejection of "000" is
// UNVERIFIED either way. The live AI1-flood capture's IF frames showed a
// "000" channel field while the VFO dial was being spun (a DIFFERENT
// command, IF, not modelled by this codec) — suggestive corroboration
// that "000" means "no stored-memory state" as a general CAT concept,
// but not direct evidence for MC's own answer shape. Still ASSUMED;
// verify with a direct "MC;" query in VFO mode at a future session.
//
// It classifies through d.classifySlot, not the package-level
// classifySlotWire: this helper is shared by BuildMCSet, ParseMCAnswer and
// AllowedCommand's MC grammar check, all Dialect methods.
func (d Dialect) mcValid(s Slot) bool {
	switch d.classifySlot(s.Wire()) {
	case slotKindMemory, slotKindPMS, slotKind60m, slotKindEMG:
		return true
	default:
		return false
	}
}

// BuildMCSet builds an MC (memory channel recall) Set frame for slot s.
//
// SIDE EFFECT: sending this to the radio recalls the channel and changes
// its operating state. Reference: "MC — MEMORY CHANNEL (recall) ... Set (6
// bytes): MC P1 P1 P1 ; ... Side effect: recalls the channel on the
// radio (changes operating state!)." Golden vector G11: "MC099;" -> recall
// M-99.
func (d Dialect) BuildMCSet(s Slot) (Command, error) {
	if !d.mcValid(s) {
		return Command{}, newParseError([]byte(s.Wire()), "MC: slot must not be \"000\"/invalid (reference MC set column: ✗)")
	}
	frame := make([]byte, 0, mcSetLen)
	frame = append(frame, 'M', 'C')
	frame = append(frame, s.Wire()...)
	frame = append(frame, ';')
	return newCommand(frame), nil
}

// BuildMCRead builds the MC read request. Reference: "Read: MC;".
//
// Takes a dialect receiver even though nothing about this frame varies by
// radio: uniform method form means M9c adds a dialect by writing a table
// rather than by re-plumbing signatures. Do not "tidy" this back to a
// package-level function.
func (d Dialect) BuildMCRead() Command {
	return newCommand([]byte(mcReadFrame))
}

// ParseMCAnswer parses an MC answer frame ("MC" + 3-byte slot + ";") into
// the recalled Slot, under this dialect's slot space. See mcValid's doc
// for why "000" is rejected here too.
func (d Dialect) ParseMCAnswer(frame []byte) (Slot, error) {
	if len(frame) != mcSetLen {
		return Slot{}, newParseError(frame, "MC answer must be 6 bytes")
	}
	if frame[0] != 'M' || frame[1] != 'C' {
		return Slot{}, newParseError(frame, "MC answer missing \"MC\" prefix")
	}
	if frame[5] != ';' {
		return Slot{}, newParseError(frame, "MC answer missing ';' terminator")
	}
	s, err := d.ParseSlot(string(frame[2:5]))
	if err != nil {
		return Slot{}, newParseError(frame, "MC answer: invalid slot field")
	}
	if !d.mcValid(s) {
		return Slot{}, newParseError(frame, "MC answer: slot must not be \"000\" (reference MC set column: ✗)")
	}
	return s, nil
}
