// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "fmt"

// mcSetLen is the fixed length of an MC set/answer frame: "MC" + 3-byte
// slot + ";". Golden vector G11: "MC099;".
const mcSetLen = 6

// mcReadFrame is the fixed MC read request. Reference: "Read: MC; ->
// Answer MC P1 P1 P1;".
const mcReadFrame = "MC;"

// mcParseValid reports whether s is a legal MC target UNDER THIS DIALECT'S
// slot space, in the READ direction: memory, PMS, 60m or EMG.
//
// It was mcValid, one predicate shared by all three callers, until the
// FT-891's Stage 0: that radio's MC block prints memory and PMS ONLY where
// every registered sibling's prints 5xx and EMG too, so the SEND domain
// became dialect data (SlotSpace.MCSelects) while this one did not. See
// mcSendValid below for the direction that varies, and for why the split
// runs this way round and not the other.
//
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
// It classifies through d.classifySlot rather than reading the kind s
// carries: this helper is shared by BuildMCSet, ParseMCAnswer and
// AllowedCommand's MC grammar check, all Dialect methods, and a
// caller-supplied Slot's stored kind is the verdict of whichever dialect
// built it (see Slot in slot.go), not of this one. Before M9d the same
// line guarded against a package-level classifySlotWire helper that
// answered for the FT-710 on every dialect; that helper is gone.
func (d Dialect) mcParseValid(s Slot) bool {
	switch d.classifySlot(s.Wire()) {
	case slotKindMemory, slotKindPMS, slotKind60m, slotKindEMG:
		return true
	default:
		return false
	}
}

// mcSendValid reports whether s is a legal target for an MC Set THIS
// DIALECT MAY EMIT: memory and PMS always, 60m and EMG only under
// MCSelectsAll.
//
// SHARED BY BuildMCSet AND AllowedCommand's validMCCommand, and by nothing
// else. Those two are the SEND direction, and the outbound gate must judge
// by the same rule the builder does or the two drift apart — the standing
// rule of this package's gate.
//
// THE SPLIT IS SAFE ONLY IN THIS DIRECTION, and the asymmetry is the point.
// AllowedCommand is an OUTBOUND gate: it answers "may this program write
// these bytes to the radio", never "may the radio have said this". An MC Set
// and an MC Answer share one wire shape — "MC" + 3 + ";" — so a 6-byte MC
// frame reaching the gate can only ever be a Set, because an answer is not
// something this program sends. Reading that shape by the SEND domain is
// therefore correct at the gate and would be wrong in ParseMCAnswer, which
// keeps mcParseValid: a radio parked on a 60m channel it reached from the
// front panel answers "MC5xx;" however narrow its Set domain is, and
// refusing that would turn a legitimate answer into an error.
//
// It classifies through d.classifySlot for the same reason mcParseValid
// does: a caller-supplied Slot's stored kind is the verdict of whichever
// dialect built it (see Slot in slot.go), not of this one.
//
// Pinned by mcpolicy_test.go's
// TestMCSelects_MemoryPMSRefusesSixtyAndEMGAtBuilderAndGate (both halves on
// the same wire forms) and TestEveryDialect_MCGateAgreesWithItsBuilder.
func (d Dialect) mcSendValid(s Slot) bool {
	// A SWITCH ON THE POLICY FIRST, not a classify-then-if-else with an
	// implicit "everything else is narrow" arm: the S0-close review's
	// HIGH-1 finding was exactly that shape — a zero MCSlotPolicy fell
	// through to the memory/PMS reading instead of refusing. NewDialect's
	// V13 (dialectvalidate.go) already refuses a zero MCSlotPolicy at
	// construction, but the default branch below is what keeps this site
	// from silently taking ANY reading, memory/PMS included, in the one
	// place V13 cannot reach — see unsetpolicy_test.go.
	switch d.slots.mcSelects {
	case MCSelectsAll:
		switch d.classifySlot(s.Wire()) {
		case slotKindMemory, slotKindPMS, slotKind60m, slotKindEMG:
			return true
		default:
			return false
		}
	case MCSelectsMemoryPMS:
		switch d.classifySlot(s.Wire()) {
		case slotKindMemory, slotKindPMS:
			return true
		default:
			return false
		}
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
	// TWO REFUSALS, in this order, because they say different things. The
	// first is the MC command's own slot space — "000" and anything this
	// dialect does not classify at all — and its wording is unchanged, which
	// is what keeps every FT-710 refusal in the frame corpus byte-identical.
	// The second is the dialect's declared SEND domain, and it fires only
	// where MCSelects narrows the space the first one admits.
	if !d.mcParseValid(s) {
		return Command{}, newParseError([]byte(s.Wire()), "MC: slot must not be \"000\"/invalid (reference MC set column: ✗)")
	}
	if !d.mcSendValid(s) {
		// TWO WORDINGS, because an unset policy and a DECLARED narrow one
		// are different facts. The narrow wording asserts "memory and PMS
		// only" of this dialect's MC legend; saying that of a policy which
		// declares nothing would put a reading in a caller's hands that no
		// manual supports. The unset arm therefore takes the P5/P11 sites'
		// own wording (memdata.go, mtcombined.go) — the policy is unset and
		// this refuses to guess. Unreachable through NewDialect, whose V13
		// (dialectvalidate.go) refuses a zero MCSlotPolicy at construction;
		// reachable through a hand-built zero Dialect, which is what
		// unsetpolicy_test.go builds.
		if d.slots.mcSelects != MCSelectsAll && d.slots.mcSelects != MCSelectsMemoryPMS {
			return Command{}, newParseError([]byte(s.Wire()), "MC: slot policy unset — refusing to guess whether this dialect's MC legend prints the 5xx and EMG banks")
		}
		return Command{}, newParseError([]byte(s.Wire()), fmt.Sprintf("MC: slot %q is outside this dialect's MC send domain (%v: memory and PMS only) — its MC legend does not print the 5xx or EMG banks, and an MC Set recalls the channel on the radio", s.Wire(), d.slots.mcSelects))
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
// the recalled Slot, under this dialect's slot space. See mcParseValid's
// doc for why "000" is rejected here too.
//
// IT IS DELIBERATELY NOT GOVERNED BY SlotSpace.MCSelects. That policy is
// the SEND domain; this is the read direction, and it keeps the full
// readable space on every dialect — see mcSendValid for the reasoning.
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
	if !d.mcParseValid(s) {
		return Slot{}, newParseError(frame, "MC answer: slot must not be \"000\" (reference MC set column: ✗)")
	}
	return s, nil
}
