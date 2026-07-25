// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"fmt"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// Severity classifies how serious a validation Issue is.
type Severity string

const (
	// SeverityError marks an issue that must block a send to a radio.
	SeverityError Severity = "error"
	// SeverityWarning marks an issue worth surfacing to the user but that
	// does not, by itself, block a send.
	SeverityWarning Severity = "warning"
)

// Issue is one thing Validate found wrong (or worth flagging) with a
// Codeplug against a Capabilities.
type Issue struct {
	// Slot is the canonical wire-form slot identifier this issue concerns,
	// or "" for a codeplug-level issue that is not about any single slot.
	Slot string
	// Field is the spec.Field this issue concerns, or "" when the issue is
	// not specific to one field.
	Field spec.Field
	// Severity says whether this issue must block a send (SeverityError)
	// or is merely worth surfacing (SeverityWarning).
	Severity Severity
	// Msg is a human-readable, self-contained description: understanding
	// it never requires cross-referencing anything else.
	Msg string
}

// validTagByte reports whether b is a legal channel-tag byte: printable
// ASCII 0x20-0x7E, EXCLUDING ';' (0x3B).
//
// This restates core/cat's validMTTagByte (core/cat/mt.go, used by
// BuildMTSet) rather than importing core/cat: there, the same charset is a
// wire rule (command-injection defence — a ';' would let a tag terminate
// the MT frame early and smuggle a second command onto the wire); here it
// is a model rule, catching the same problem before a tag ever reaches a
// builder. codeplug imports spec + stdlib only, so the check is small
// enough to duplicate rather than share.
func validTagByte(b byte) bool {
	return b >= 0x20 && b <= 0x7E && b != ';'
}

// containsString reports whether s appears in list.
func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// findToneState returns the spec.ToneState in states whose Value equals
// value, and true, or the zero spec.ToneState and false if none matches.
func findToneState(states []spec.ToneState, value string) (spec.ToneState, bool) {
	for _, s := range states {
		if s.Value == value {
			return s, true
		}
	}
	return spec.ToneState{}, false
}

// toneStateValues returns the Value of every entry in states, in order —
// for building a caps-driven vocabulary list for an error message
// without re-deriving a []string by hand at the call site.
func toneStateValues(states []spec.ToneState) []string {
	values := make([]string, len(states))
	for i, s := range states {
		values[i] = s.Value
	}
	return values
}

// quotedList formats vals as a comma-separated list of double-quoted
// values, e.g. []string{"OFF", "ENC"} -> `"OFF", "ENC"` — so a Validate
// error message names caps' own vocabulary list rather than a hardcoded
// one.
func quotedList(vals []string) string {
	quoted := make([]string, len(vals))
	for i, v := range vals {
		quoted[i] = fmt.Sprintf("%q", v)
	}
	return strings.Join(quoted, ", ")
}

// findChannel returns the Channel in channels with the given slot, and
// true, or the zero Channel and false if no channel has that slot.
func findChannel(channels []Channel, slot string) (Channel, bool) {
	for _, ch := range channels {
		if ch.Slot == slot {
			return ch, true
		}
	}
	return Channel{}, false
}

// Validate checks cp against caps and returns every Issue found. It never
// mutates cp or caps.
//
// Determinism: Issues are returned in a stable order for the same inputs.
// The radio-identity check comes first (see below), then per-channel
// issues, in cp.Channels slice order: for each channel, slot-inventory
// membership, then (if the slot is a duplicate of an earlier channel) the
// duplicate-slot issue, then — for a populated channel — Frequency, Mode,
// Clarifier, CTCSS state, CTCSSTone.Valid(), ScanSkip.Valid(), the
// CTCSS-tone-pairing warning, Shift, and Tag, in that fixed order.
// Codeplug-level issues come last, in a fixed order: the completeness
// check (see below; caps.Banks order, each bank's Slots in bank order),
// then caps.RequiredSlots (in caps order) for the populated-ness check,
// then caps.Banks (in caps order) for each NoBlank bank's Slots (in bank
// order) for its populated-ness check.
//
// Radio identity: if caps.Model or caps.CATID is non-empty and cp.Radio's
// corresponding field disagrees with it, that is an Error — this
// codeplug was read from a different radio than the one Validate is
// being asked to gate a send to, and every other check below is
// meaningless until that is resolved (a matching slot in the wrong
// radio's memory bank is not the same slot). Either side being empty
// (e.g. caps under test, or a hand-built Codeplug with RadioInfo left
// zero) is not itself an error: this rule only fires on an actual
// disagreement between two known values.
//
// Completeness: the full expected slot set is the union of every
// caps.Bank's Slots (RequiredSlots and each NoBlank bank's Slots are
// always subsets of it). Every expected slot must appear among
// cp.Channels at least once — a slot present zero times is an Error —
// which subsumes what used to be a NoBlank-only (and RequiredSlots-only)
// "missing entirely" check, and closes a duplicate-multiset gap: a
// codeplug whose channel COUNT happens to match the expected count,
// because one slot is duplicated while a different one is missing
// entirely, still fails here on the missing slot specifically (the
// per-channel duplicate-slot check above independently flags the
// duplicate side of the same scenario). Presence is necessary but not
// sufficient for a RequiredSlots slot or a NoBlank bank's slot: those
// keep their own separate populated-ness checks (a present-but-empty
// Channel for either is still an Error), unaffected by this rule.
func Validate(cp *Codeplug, caps spec.Capabilities) []Issue {
	var issues []Issue

	modelMismatch := caps.Model != "" && cp.Radio.Model != caps.Model
	catIDMismatch := caps.CATID != "" && cp.Radio.CATID != caps.CATID
	if modelMismatch || catIDMismatch {
		issues = append(issues, Issue{
			Severity: SeverityError,
			Msg: fmt.Sprintf("codeplug was read from %s/%s but the target radio is %s/%s; re-read the radio",
				cp.Radio.Model, cp.Radio.CATID, caps.Model, caps.CATID),
		})
	}

	seen := make(map[string]bool, len(cp.Channels))
	for _, ch := range cp.Channels {
		if _, ok := bankForSlot(caps, ch.Slot); !ok {
			issues = append(issues, Issue{
				Slot:     ch.Slot,
				Severity: SeverityError,
				Msg:      fmt.Sprintf("slot %q is not part of any bank this radio supports", ch.Slot),
			})
		}
		if seen[ch.Slot] {
			issues = append(issues, Issue{
				Slot:     ch.Slot,
				Severity: SeverityError,
				Msg:      fmt.Sprintf("slot %q appears more than once in this codeplug's channel list", ch.Slot),
			})
		}
		seen[ch.Slot] = true

		if ch.Empty() {
			continue
		}
		issues = append(issues, validateChannelData(ch.Slot, *ch.Data, caps)...)
	}

	// Completeness: every slot listed in ANY caps.Bank must appear (at
	// least — the appears-more-than-once check above independently
	// catches "too many") once among cp.Channels. This is the full
	// expected slot set, not just caps.RequiredSlots or NoBlank banks'
	// Slots (both of which are always subsets of it), and it is what
	// closes the duplicate-multiset gap: a codeplug whose total channel
	// count happens to match, because one slot is duplicated while a
	// completely different one is missing, still fails here on the
	// missing slot specifically — it is never enough for the totals to
	// line up. seenSlot dedupes so a slot claimed by more than one bank
	// (a caps inconsistency this function does not otherwise police —
	// see spec.Capabilities.Validate) is only reported once, at its first
	// occurrence in caps.Banks order.
	seenSlot := make(map[string]bool)
	for _, b := range caps.Banks {
		for _, slot := range b.Slots {
			if seenSlot[slot] {
				continue
			}
			seenSlot[slot] = true
			if !seen[slot] {
				issues = append(issues, Issue{
					Slot:     slot,
					Severity: SeverityError,
					Msg:      fmt.Sprintf("slot %q is expected (bank %s) but missing from this codeplug entirely", slot, b.ID),
				})
			}
		}
	}

	// Populated-ness: presence alone is not enough for a RequiredSlots
	// slot or a NoBlank bank's slot — both must also stay non-empty. A
	// slot missing entirely is already reported by the completeness check
	// above, so these loops only fire for a slot that IS present.
	for _, slot := range caps.RequiredSlots {
		ch, ok := findChannel(cp.Channels, slot)
		if !ok {
			continue
		}
		if ch.Empty() {
			issues = append(issues, Issue{
				Slot:     slot,
				Severity: SeverityError,
				Msg:      fmt.Sprintf("required slot %q must not be empty", slot),
			})
		}
	}

	for _, b := range caps.Banks {
		if !b.NoBlank {
			continue
		}
		for _, slot := range b.Slots {
			ch, ok := findChannel(cp.Channels, slot)
			if !ok {
				continue
			}
			if ch.Empty() {
				issues = append(issues, Issue{
					Slot:     slot,
					Severity: SeverityError,
					Msg:      fmt.Sprintf("slot %q is part of NoBlank bank %q and must stay populated, but is empty", slot, b.ID),
				})
			}
		}
	}

	return issues
}

// validateChannelData checks one populated channel's data and returns its
// Issues in the fixed field order documented on Validate.
func validateChannelData(slot string, d ChannelData, caps spec.Capabilities) []Issue {
	var issues []Issue

	if d.FreqHz == 0 {
		issues = append(issues, Issue{
			Slot: slot, Field: spec.FieldFrequency, Severity: SeverityError,
			Msg: fmt.Sprintf("slot %q: frequency must be greater than 0 Hz", slot),
		})
	} else {
		if caps.MinFreqHz != 0 && d.FreqHz < caps.MinFreqHz {
			issues = append(issues, Issue{
				Slot: slot, Field: spec.FieldFrequency, Severity: SeverityError,
				Msg: fmt.Sprintf("slot %q: frequency %d Hz is below this radio's minimum of %d Hz", slot, d.FreqHz, caps.MinFreqHz),
			})
		}
		if caps.MaxFreqHz != 0 && d.FreqHz > caps.MaxFreqHz {
			issues = append(issues, Issue{
				Slot: slot, Field: spec.FieldFrequency, Severity: SeverityError,
				Msg: fmt.Sprintf("slot %q: frequency %d Hz is above this radio's maximum of %d Hz", slot, d.FreqHz, caps.MaxFreqHz),
			})
		}
	}

	if !containsString(caps.Modes, d.Mode) {
		issues = append(issues, Issue{
			Slot: slot, Field: spec.FieldMode, Severity: SeverityError,
			Msg: fmt.Sprintf("slot %q: mode %q is not one of this radio's supported modes", slot, d.Mode),
		})
	}

	clarAbs := d.ClarHz
	if clarAbs < 0 {
		clarAbs = -clarAbs
	}
	if clarAbs > caps.ClarMaxHz {
		issues = append(issues, Issue{
			Slot: slot, Field: spec.FieldClarifier, Severity: SeverityError,
			Msg: fmt.Sprintf("slot %q: clarifier %d Hz exceeds this radio's maximum of %d Hz", slot, d.ClarHz, caps.ClarMaxHz),
		})
	}
	if caps.ClarStepHz != 0 && d.ClarHz%caps.ClarStepHz != 0 {
		issues = append(issues, Issue{
			Slot: slot, Field: spec.FieldClarifier, Severity: SeverityError,
			Msg: fmt.Sprintf("slot %q: clarifier %d Hz is not a multiple of this radio's step size of %d Hz", slot, d.ClarHz, caps.ClarStepHz),
		})
	}

	ctcssState, ctcssKnown := findToneState(caps.CTCSSStates, d.CTCSS)
	if !ctcssKnown {
		issues = append(issues, Issue{
			Slot: slot, Field: spec.FieldCTCSSState, Severity: SeverityError,
			Msg: fmt.Sprintf("slot %q: ctcss %q must be one of %s", slot, d.CTCSS, quotedList(toneStateValues(caps.CTCSSStates))),
		})
	}

	if err := d.CTCSSTone.Valid(); err != nil {
		issues = append(issues, Issue{
			Slot: slot, Field: spec.FieldCTCSSTone, Severity: SeverityError,
			Msg: fmt.Sprintf("slot %q: %v", slot, err),
		})
	}
	if err := d.ScanSkip.Valid(); err != nil {
		issues = append(issues, Issue{
			Slot: slot, Field: spec.FieldScanSkip, Severity: SeverityError,
			Msg: fmt.Sprintf("slot %q: %v", slot, err),
		})
	}

	// A CTCSS state that requires a tone (RequiresTone true for a state
	// matched against caps; an unmatched state — already flagged as an
	// Error above — is conservatively treated the same way, since it
	// cannot make a positive claim about not needing one) but has no
	// known CTCSSTone gets a Warning: CAT cannot set a per-channel tone,
	// so the radio's own existing tone will apply as-is.
	if (!ctcssKnown || ctcssState.RequiresTone) && d.CTCSSTone.State != Known {
		issues = append(issues, Issue{
			Slot: slot, Field: spec.FieldCTCSSTone, Severity: SeverityWarning,
			Msg: fmt.Sprintf("slot %q: tone cannot be set via CAT; the radio's current per-channel tone will apply", slot),
		})
	}

	if !containsString(caps.ShiftOptions, d.Shift) {
		issues = append(issues, Issue{
			Slot: slot, Field: spec.FieldShift, Severity: SeverityError,
			Msg: fmt.Sprintf("slot %q: shift %q must be one of %s", slot, d.Shift, quotedList(caps.ShiftOptions)),
		})
	}

	if len(d.Tag) > caps.TagLen {
		issues = append(issues, Issue{
			Slot: slot, Field: spec.FieldTag, Severity: SeverityError,
			Msg: fmt.Sprintf("slot %q: tag is %d bytes, exceeds this radio's maximum of %d", slot, len(d.Tag), caps.TagLen),
		})
	}
	for i := 0; i < len(d.Tag); i++ {
		if !validTagByte(d.Tag[i]) {
			issues = append(issues, Issue{
				Slot: slot, Field: spec.FieldTag, Severity: SeverityError,
				Msg: fmt.Sprintf("slot %q: tag contains an invalid byte (must be printable ASCII 0x20-0x7E, excluding ';')", slot),
			})
			break
		}
	}

	return issues
}

// HasErrors reports whether issues contains at least one SeverityError
// Issue. Warnings alone do not block a send.
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Severity == SeverityError {
			return true
		}
	}
	return false
}
