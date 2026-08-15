// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// DiffKind classifies how a single slot differs between a Diff baseline
// and file.
type DiffKind string

const (
	// DiffAdded means the slot was empty in the baseline and populated in
	// the file.
	DiffAdded DiffKind = "added"
	// DiffModified means the slot is populated in both, with different
	// ChannelData (including field state — see Diff's Equality doc).
	DiffModified DiffKind = "modified"
	// DiffErased means the slot was populated in the baseline and empty
	// in the file.
	DiffErased DiffKind = "erased"
	// DiffUnchanged means the slot is identical in both (both empty, or
	// both populated with equal ChannelData).
	DiffUnchanged DiffKind = "unchanged"
)

// DiffEntry describes how one slot differs between a Diff baseline and
// file.
type DiffEntry struct {
	// Slot is the canonical wire-form slot identifier.
	Slot string
	// Bank is the bank this slot belongs to, per caps. It is the zero
	// BankID ("") if caps does not claim this slot in any bank — Diff
	// treats that as unwritable (see Blocked).
	Bank spec.BankID
	// Kind classifies this slot's change.
	Kind DiffKind
	// Before is a defensive copy of the baseline's ChannelData, or nil
	// when the slot was previously empty.
	Before *ChannelData
	// After is a defensive copy of the file's ChannelData, or nil when
	// the slot is now empty.
	After *ChannelData
	// Blocked is true when this change cannot be sent to a radio: the bank
	// does not support writing FieldFrequency at all (e.g. 60M/EMG); it
	// is an erase and FieldErase does not pass the write gate for this
	// bank; (Modified/Added only) its TagDisplay is not Known while this
	// bank does transmit the display flag; or (Modified/Added only) the
	// change touches at least one field that is not WRITABLE for this
	// bank — writable meaning spec.FieldSupport.CanWrite, which since the
	// consent milestone is true for spec.Supported AND for
	// spec.ConsentedUnverified, so a field the user has consented to
	// writing does not block here. See Diff's Blocked doc for the exact
	// rule and precedence order.
	Blocked bool
	// BlockReason explains Blocked, e.g. "erase not supported on this
	// radio", "bank 60M is read-only", "tag display unknown — set On or
	// Off before sending", "ctcss_tone not writable on this radio", or
	// "clarifier changes are ignored by the radio and cannot be sent".
	// Empty when Blocked is false.
	BlockReason string
}

// DiffResult is the full comparison of a baseline Codeplug against a
// candidate file Codeplug, ready for a user to review and confirm before
// anything is sent to a radio.
type DiffResult struct {
	// Entries holds one DiffEntry per slot present in either input, in
	// baseline slot order (see Diff's Determinism doc).
	Entries []DiffEntry
	// Added is the count of Entries with Kind == DiffAdded.
	Added int
	// Modified is the count of Entries with Kind == DiffModified.
	Modified int
	// Erased is the count of Entries with Kind == DiffErased.
	Erased int
	// Unchanged is the count of Entries with Kind == DiffUnchanged.
	Unchanged int
	// Blocked is the count of Entries with Blocked == true. A Blocked
	// entry is counted here IN ADDITION TO its Kind count above, not
	// instead of it.
	Blocked int
	// BaselineDigest is Digest(baseline.Channels) — what a send
	// confirmation must be bound to.
	BaselineDigest string
	// CandidateDigest is Digest(file.Channels) — the exact candidate
	// image this DiffResult was computed against.
	//
	// A DiffResult is only trustworthy for describing the images whose
	// digests match BaselineDigest and CandidateDigest AT THE MOMENT IT
	// WAS COMPUTED. Anything reviewed by a user and acted on later — a
	// different process, a delay, a reconnect — can have drifted from
	// what was diffed, and Digest cannot by itself distinguish a
	// content-identical re-read from "nothing changed" (see Digest's doc
	// comment). A sender MUST therefore recompute both digests, and
	// compare them against the baseline/candidate it is actually about to
	// transmit, IMMEDIATELY BEFORE transmission, and MUST re-run Validate
	// at that same boundary rather than trusting a DiffResult computed
	// any earlier. Constructing the actual immutable send-plan object
	// that carries this guarantee through to the wire — including binding
	// it to session/device identity (CAT ID, USB serial, a read
	// generation) — is the clone service's job, not this package's; this
	// field only supplies the content half of that check.
	CandidateDigest string
}

// sortedSlots returns the Slot of every channel in channels, sorted. Used
// only to compare two channel lists' slot inventories irrespective of
// input order.
func sortedSlots(channels []Channel) []string {
	slots := make([]string, len(channels))
	for i, ch := range channels {
		slots[i] = ch.Slot
	}
	sort.Strings(slots)
	return slots
}

// equalStrings reports whether a and b hold the same strings in the same
// order.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// copyChannelData returns a defensive copy of d, or nil if d is nil.
// ChannelData holds only value types (no slices/maps/pointers), so a
// single dereference-and-copy is already a full deep copy.
func copyChannelData(d *ChannelData) *ChannelData {
	if d == nil {
		return nil
	}
	cp := *d
	return &cp
}

// changedFields returns every spec.Field whose value differs between
// before and after, in a fixed order (frequency, mode, clarifier,
// ctcss_state, ctcss_tone, shift, tag, tag_display, scan_skip). ClarHz,
// RxClar and TxClar are three Go fields but one spec.Field
// (FieldClarifier): a change to any of the three counts as one changed
// field. CTCSSTone and ScanSkip are compared as whole structs, so a
// FieldState transition (e.g. tone Known -> Unknown) counts as a change
// to that field, exactly the same as a Value change.
func changedFields(before, after ChannelData) []spec.Field {
	var out []spec.Field
	if before.FreqHz != after.FreqHz {
		out = append(out, spec.FieldFrequency)
	}
	if before.Mode != after.Mode {
		out = append(out, spec.FieldMode)
	}
	if before.ClarHz != after.ClarHz || before.RxClar != after.RxClar || before.TxClar != after.TxClar {
		out = append(out, spec.FieldClarifier)
	}
	if before.CTCSS != after.CTCSS {
		out = append(out, spec.FieldCTCSSState)
	}
	if before.CTCSSTone != after.CTCSSTone {
		out = append(out, spec.FieldCTCSSTone)
	}
	if before.Shift != after.Shift {
		out = append(out, spec.FieldShift)
	}
	if before.Tag != after.Tag {
		out = append(out, spec.FieldTag)
	}
	if before.TagDisplay != after.TagDisplay {
		out = append(out, spec.FieldTagDisplay)
	}
	if before.ScanSkip != after.ScanSkip {
		out = append(out, spec.FieldScanSkip)
	}
	return out
}

// addedFields returns every spec.Field an Added channel's data
// introduces — i.e. every field whose value will actually be sent to the
// radio when this channel is written. Every plain (non-FieldState) field
// is always introduced, even at its zero value (e.g. ClarHz == 0 is a
// real requested "no clarifier", not an absent value). CTCSSTone,
// TagDisplay and ScanSkip are FieldState-carrying: per FieldState's write
// rule, only a Known value is ever sent — Unknown or Unavailable means
// "preserve whatever the radio has", i.e. nothing is requested for that
// field at all, so it is deliberately NOT counted as introduced here.
//
// ORDER IS PART OF THE CONTRACT, not incidental: this slice is what
// Diff's generic per-field gate walks, and therefore the order in which
// fieldGateBlockReason/modifiedBlockReason name fields in a BlockReason a
// user reads. TagDisplay keeps its PLACE — after tag, before the
// tone/skip conditionals, i.e. seventh whenever it appears at all —
// exactly where it sat when it was unconditional
// (TestAddedFields_MembershipAndOrder pins this).
//
// TagDisplay's conditional is subtler than the other two, because a
// non-Known TagDisplay is normally refused OUTRIGHT before this set is
// ever consulted (see Diff's doc comment, gate 3): the conditional here
// is what makes the one case that survives that gate come out right — a
// target whose FieldTagDisplay.Write is Unsupported never transmits the
// display flag at all, so an unknown value for it is not a problem to
// report, and leaving it in this set would have blocked the channel with
// a not-writable reason for a field nobody asked to write.
func addedFields(data ChannelData) []spec.Field {
	out := []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldClarifier,
		spec.FieldCTCSSState,
		spec.FieldShift,
		spec.FieldTag,
	}
	if data.TagDisplay.State == Known {
		out = append(out, spec.FieldTagDisplay)
	}
	if data.CTCSSTone.State == Known {
		out = append(out, spec.FieldCTCSSTone)
	}
	if data.ScanSkip.State == Known {
		out = append(out, spec.FieldScanSkip)
	}
	return out
}

// tagDisplayUnknownReason is the BlockReason Diff's TagDisplay gate
// produces (see Diff's doc comment, gate 3). It is deliberately phrased
// as an INSTRUCTION rather than a description: unlike every other block
// reason in this file — a bank the radio will not write, a field awaiting
// hardware verification, a value the radio provably ignores — this one
// names something the user can fix immediately, and the whole reason the
// gate is ordered ahead of the generic aggregation is so that instruction
// is never buried in a "; "-joined list of problems they cannot fix at
// all. "On"/"Off" are the front panel's own words for the setting.
const tagDisplayUnknownReason = "tag display unknown — set On or Off before sending"

// fieldGateBlockReason builds a DiffEntry.BlockReason naming every field
// in fields (assumed non-empty) that is not WRITABLE for this entry's
// bank — spec.FieldSupport.CanWrite false, so neither spec.Supported nor
// spec.ConsentedUnverified. A consented field never reaches this list.
func fieldGateBlockReason(fields []spec.Field) string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = string(f)
	}
	return fmt.Sprintf("%s not writable on this radio", strings.Join(names, ", "))
}

// modifiedBlockReason is fieldGateBlockReason for a DiffModified entry,
// additionally noting — per named field — when that field is NOT among
// changed (M3 Codex-review fix wave, Fix 4): since fields is now every
// TRANSMITTED field (see Diff's per-field gate doc, point 4), not merely
// every CHANGED one, a field that blocks the whole entry despite this
// particular edit leaving its value untouched is the surprising case a
// reviewer needs called out explicitly — indistinguishable from a field
// that actually changed, if the annotation were omitted.
func modifiedBlockReason(fields, changed []spec.Field) string {
	changedSet := make(map[spec.Field]bool, len(changed))
	for _, f := range changed {
		changedSet[f] = true
	}
	names := make([]string, len(fields))
	for i, f := range fields {
		if changedSet[f] {
			names[i] = string(f)
		} else {
			names[i] = fmt.Sprintf("%s (rewritten by MW even though unchanged)", f)
		}
	}
	return fmt.Sprintf("%s not writable on this radio", strings.Join(names, ", "))
}

// inertBlockReason builds a DiffEntry.BlockReason naming every Inert
// field in fields (assumed non-empty) whose value this entry would
// CHANGE. Distinct wording from fieldGateBlockReason on purpose: an
// Inert field is not "awaiting verification" — the radio was verified to
// IGNORE the transmitted value (HW-CONFIRMED 2026-07-13,
// docs/hardware-notes.md), so the requested change can never take
// effect, whatever future trials run.
func inertBlockReason(fields []spec.Field) string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = string(f)
	}
	return fmt.Sprintf("%s changes are ignored by the radio and cannot be sent", strings.Join(names, ", "))
}

// Diff compares a baseline Codeplug (read from a radio) against a
// candidate file Codeplug and reports, per slot, what would change if
// file were sent — and which of those changes this project cannot safely
// perform yet.
//
// Slot inventory: baseline and file must describe the exact same set of
// slots (their Slot values, sorted, must match exactly) — this is how
// Diff enforces that file descends from a read of the same radio layout.
// A mismatch is reported as an error (not a panic); every other finding is
// an Entry, never an error.
//
// Equality: two populated slots are equal (DiffUnchanged) only when their
// ChannelData values are == , which compares every field including
// ToneField/BoolField.State — a tone going Known -> Unknown is therefore
// a DiffModified, not merely a value change. Two empty slots (both
// Data == nil) are always DiffUnchanged.
//
// Kind: empty -> populated is DiffAdded; populated -> empty is
// DiffErased; both populated and unequal is DiffModified; anything else
// (both empty, or both populated and equal) is DiffUnchanged.
//
// Blocked: DiffUnchanged entries are never Blocked (there is nothing to
// send). For any other Kind, Diff checks the following gates IN ORDER,
// stopping at the first that fires (an earlier gate's reason always takes
// precedence over a later one):
//
//  1. Bank-level: does the slot's bank support writing FieldFrequency at
//     all — CanWrite, which is Supported OR spec.ConsentedUnverified (the
//     user's recorded consent to an unproven write), OR spec.Inert (Fix 4,
//     Codex M5b fix wave, adjudicated MEDIUM: transmissible, even if the
//     radio may ignore it — mirrors point 4's per-field Inert exception,
//     and Session.WriteChannel's own gate) — (an Unknown-bank slot counts
//     as not supporting it)? If not, the WHOLE entry is Blocked with a
//     "bank ... is read-only" (or unknown-bank) reason, regardless of Kind.
//
//  2. Erase-specific: for a DiffErased entry only, does the bank's
//     FieldErase pass CanWrite? If not, Blocked with an "erase not
//     supported..." reason.
//
//     CONSENT CANNOT OPEN THIS GATE, and the exclusion is structural
//     rather than a promise made here: spec.ConsentUnverifiedWrites
//     exempts FieldErase from the transform outright (see its doc
//     comment), so no capability set reaching this gate can carry a
//     consented erase, and populated-to-empty stays blocked on every
//     radio whose erase is not genuinely Supported — consented or not.
//
//  3. TagDisplay knowledge (M9c-5, E1b): for a DiffModified OR a DiffAdded
//     entry, is After's TagDisplay.State anything other than Known while
//     this bank's FieldTagDisplay.Write is anything other than
//     spec.Unsupported? If so, the WHOLE entry is Blocked with
//     tagDisplayUnknownReason, and gate 4 does not run for it.
//
//     Why a gate of its own rather than another contributor to gate 4:
//     the display flag in a write frame is MANDATORY on this radio family
//     — there is no "leave it alone" encoding — so a non-Known TagDisplay
//     cannot be transmitted at all without manufacturing a value the user
//     never chose. That is a different KIND of problem from gate 4's
//     ("this radio cannot write that field"): it is the one block a user
//     can clear themselves, by deciding. Ordering it ahead of gate 4, and
//     stopping there, is what keeps that instruction from being merged
//     into a "; "-joined list of unfixable findings.
//
//     The Write != Unsupported condition (read DIRECTLY from caps here —
//     gate 4 cannot report it, because a non-Known TagDisplay is by then
//     no longer in addedFields' set at all) is the honest converse: a
//     target that never transmits the display flag needs no value for it,
//     so such a channel is not blocked by this gate — or by gate 4, per
//     addedFields' doc comment. The condition is deliberately NOT
//     FieldSupport.CanWrite(), which is false for Unverified and Inert
//     too: a merely-unverified field is still a field the frame carries a
//     value for, so the unknown-value problem is real there and this
//     reason (which names the remedy) must win over gate 4's (which does
//     not).
//
//  4. Per-field (v1, all-or-nothing per channel): for a DiffModified OR a
//     DiffAdded entry, Diff computes "touched" as addedFields(after) —
//     the SAME field set for both Kinds (M3 Codex-review fix wave, Fix
//     4): every field the write would actually TRANSMIT (the six
//     always-sent fields, frequency/mode/clarifier/ctcss_state/shift/
//     tag, plus TagDisplay/CTCSSTone/ScanSkip only when their FieldState
//     is Known — per FieldState's write rule, an Unknown/Unavailable
//     field is never sent, so it introduces no write request to gate).
//     This is DELIBERATELY NOT "only the fields that changed"
//     (changedFields) for a Modified entry, which is what earlier
//     versions of this function used, and is superseded here: MW+MT
//     rewrite EVERY expressible field on the slot each time they run,
//     whether or not that field's value actually changed in this
//     particular edit, so an unwritable field that happens to be
//     unchanged would still be clobbered by the write and must still
//     block. If ANY touched field is not WRITABLE for this bank —
//     spec.FieldSupport.CanWrite false, so neither spec.Supported nor
//     spec.ConsentedUnverified; a field the user has consented to writing
//     passes this gate exactly as a hardware-proven one does — the WHOLE
//     entry is Blocked, naming every such field — for a
//     Modified entry, additionally noting "(rewritten by MW even though
//     unchanged)" against any named field that changedFields does NOT
//     also report changed (see modifiedBlockReason), since that is the
//     surprising case a reviewer needs flagged explicitly.
//
//     EXCEPTION — the Inert rule (M5b, HW-CONFIRMED 2026-07-13; see
//     spec.Inert): a touched field whose Write support is spec.Inert is
//     transmitted but IGNORED by the radio, so it blocks the entry ONLY
//     when its value differs Before->After (or, for an Added entry,
//     differs from the zero value a freshly-created channel reads back)
//     — with inertBlockReason's distinct wording, never
//     fieldGateBlockReason's generic not-writable one. An
//     UNCHANGED Inert field does not block: the write re-transmits the
//     baseline value, the radio ignores it, and the read-back verify
//     matches — without this exception, an always-transmitted Inert
//     field (the FT-710's clarifier) would block EVERY Added/Modified
//     entry project-wide, which is exactly the contradiction the M5b
//     adjudication resolved by introducing Inert. An entry with both
//     unwritable and changed-Inert fields names both, joined with "; " —
//     the two findings THIS gate can make. Gate 3's reason never appears
//     in that join: it is a stop, not a contributor.
//
// Before/After are always defensive copies (see copyChannelData): mutating
// a DiffResult's entries never mutates baseline or file.
//
// Determinism: Entries appear in baseline.Channels slice order (every slot
// in file is guaranteed present in baseline by the inventory check above),
// so the same two inputs always produce the same Entries in the same
// order.
func Diff(baseline, file *Codeplug, caps spec.Capabilities) (DiffResult, error) {
	baseSlots := sortedSlots(baseline.Channels)
	fileSlots := sortedSlots(file.Channels)
	if !equalStrings(baseSlots, fileSlots) {
		return DiffResult{}, fmt.Errorf("codeplug: Diff: baseline and file slot inventories differ; the file must descend from a read of this radio's current layout — re-read the radio and try again")
	}

	fileBySlot := make(map[string]Channel, len(file.Channels))
	for _, ch := range file.Channels {
		fileBySlot[ch.Slot] = ch
	}

	entries := make([]DiffEntry, 0, len(baseline.Channels))
	var added, modified, erased, unchanged, blocked int

	for _, baseCh := range baseline.Channels {
		fileCh := fileBySlot[baseCh.Slot]

		before := baseCh.Data
		after := fileCh.Data

		var kind DiffKind
		switch {
		case before == nil && after == nil:
			kind = DiffUnchanged
		case before == nil && after != nil:
			kind = DiffAdded
		case before != nil && after == nil:
			kind = DiffErased
		case *before == *after:
			kind = DiffUnchanged
		default:
			kind = DiffModified
		}

		bankID, _ := bankForSlot(caps, baseCh.Slot)

		var isBlocked bool
		var reason string
		if kind != DiffUnchanged {
			// Bank-level gate (Fix 4, Codex M5b fix wave, adjudicated
			// MEDIUM): FieldFrequency.CanWrite() is a proxy for "is this
			// bank writable at all", but CanWrite() is false for
			// spec.Inert exactly as it is for Unsupported/Unverified — an
			// Inert frequency (a hypothetical future case; never
			// HW-observed) is still TRANSMISSIBLE, just possibly ignored,
			// so it must count as transmissible here too, mirroring
			// Session.WriteChannel's own gate (core/driver/ft710/write.go)
			// and the generic per-field Inert exception below (point 4).
			// Leaving this bank-gate check as CanWrite()-only would
			// wrongly Block EVERY entry — even a tag-only edit with an
			// unchanged frequency — as "bank ... is read-only", before the
			// generic changed-Inert gate ever got a chance to decide.
			if fs := caps.FieldSupport(bankID, spec.FieldFrequency); !fs.CanWrite() && fs.Write != spec.Inert {
				isBlocked = true
				if bankID == "" {
					reason = fmt.Sprintf("slot %q is not part of any bank this radio supports and cannot be written", baseCh.Slot)
				} else {
					reason = fmt.Sprintf("bank %s is read-only", bankID)
				}
			} else if kind == DiffErased {
				if es := caps.FieldSupport(bankID, spec.FieldErase); !es.CanWrite() {
					isBlocked = true
					// Wording updated at M5b: for the FT-710 this is no longer
					// "awaiting verification" — NO CAT erase exists at all,
					// HW-CONFIRMED by a properly isolated 13/07/2026
					// re-probe (four range/mode-isolated candidate MW
					// frames, every one rejected — see
					// docs/hardware-notes.md's "No CAT erase" section and
					// core/driver/ft710/caps.go). The reason text stays
					// radio-generic, since this package is.
					reason = "erase not supported on this radio"
				}
			} else if (kind == DiffModified || kind == DiffAdded) &&
				after.TagDisplay.State != Known &&
				caps.FieldSupport(bankID, spec.FieldTagDisplay).Write != spec.Unsupported {
				// Gate 3 (M9c-5, E1b — see this function's doc comment):
				// the write frame's display flag is mandatory, so a
				// channel whose TagDisplay is not Known cannot be sent to
				// a target that transmits that flag without inventing a
				// value. Refuse it HERE, per channel, at plan time, and
				// stop: gate 4 must not run, because its findings would
				// bury the one instruction that actually clears this
				// block.
				//
				// caps is read DIRECTLY rather than through the touched
				// set gate 4 walks: a non-Known TagDisplay is no longer
				// in addedFields' result at all (that is what keeps the
				// Write-Unsupported case from blocking), so the support
				// entry has to be fetched explicitly to decide whether
				// the flag would be transmitted in the first place.
				isBlocked = true
				reason = tagDisplayUnknownReason
			} else if kind == DiffModified || kind == DiffAdded {
				// M3 Codex-review fix wave, Fix 4: DiffModified is gated
				// against the SAME field set as DiffAdded — addedFields(after),
				// every field the write would actually TRANSMIT — not just
				// changedFields. See this function's doc comment, point 4,
				// for why: MW+MT rewrite every expressible field each time,
				// whether or not its value changed, so an unwritable field
				// left unchanged by THIS particular edit would still be
				// clobbered by the write.
				//
				// The Inert rule (M5b, HW-CONFIRMED 2026-07-13 — see
				// spec.Inert and this function's doc comment, point 4): a
				// transmitted field whose Write support is Inert blocks the
				// entry ONLY when its value differs Before->After — the
				// radio ignores the transmitted value, so a CHANGED value
				// is an intent that can never be honoured, while an
				// UNCHANGED one merely re-transmits the baseline value the
				// read-back verify will match anyway. For a DiffAdded entry
				// there is no baseline: the Inert comparison runs against
				// the zero ChannelData, since a freshly-created channel
				// reads back zero clarifier (HW-CONFIRMED: the live
				// empty-slot MW create read back "+0000"/flags 0), so any
				// non-zero Inert value on an Added entry is unhonourable.
				touched := addedFields(*after)
				inertBase := ChannelData{}
				if kind == DiffModified {
					inertBase = *before
				}
				inertChangedSet := make(map[spec.Field]bool)
				for _, f := range changedFields(inertBase, *after) {
					inertChangedSet[f] = true
				}
				var unwritable, inertChanged []spec.Field
				for _, f := range touched {
					fs := caps.FieldSupport(bankID, f)
					switch {
					case fs.CanWrite():
						// Writable: no gate. Either hardware-proven
						// (spec.Supported) or opened by the user's recorded
						// consent (spec.ConsentedUnverified) — CanWrite does
						// not distinguish them, and neither does this gate.
					case fs.Write == spec.Inert:
						if inertChangedSet[f] {
							inertChanged = append(inertChanged, f)
						}
					default:
						unwritable = append(unwritable, f)
					}
				}
				if len(unwritable) > 0 || len(inertChanged) > 0 {
					isBlocked = true
					var parts []string
					if len(unwritable) > 0 {
						if kind == DiffModified {
							parts = append(parts, modifiedBlockReason(unwritable, changedFields(*before, *after)))
						} else {
							parts = append(parts, fieldGateBlockReason(unwritable))
						}
					}
					if len(inertChanged) > 0 {
						parts = append(parts, inertBlockReason(inertChanged))
					}
					reason = strings.Join(parts, "; ")
				}
			}
		}

		entries = append(entries, DiffEntry{
			Slot:        baseCh.Slot,
			Bank:        bankID,
			Kind:        kind,
			Before:      copyChannelData(before),
			After:       copyChannelData(after),
			Blocked:     isBlocked,
			BlockReason: reason,
		})

		switch kind {
		case DiffAdded:
			added++
		case DiffModified:
			modified++
		case DiffErased:
			erased++
		case DiffUnchanged:
			unchanged++
		}
		if isBlocked {
			blocked++
		}
	}

	return DiffResult{
		Entries:         entries,
		Added:           added,
		Modified:        modified,
		Erased:          erased,
		Unchanged:       unchanged,
		Blocked:         blocked,
		BaselineDigest:  Digest(baseline.Channels),
		CandidateDigest: Digest(file.Channels),
	}, nil
}
