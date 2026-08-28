// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import (
	"fmt"
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
	// baseline slot order followed by any sparse-bank slot only the file
	// materialised (see Diff's Determinism doc — the second group is
	// always empty for a radio with no sparse bank).
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
	// The Icom tier's ten (design D4), appended after the pre-tier nine
	// so no existing BlockReason's field list is reordered. Each is
	// compared as a WHOLE STRUCT, exactly as CTCSSTone and ScanSkip are,
	// so a state transition counts as a change like a value change —
	// including a transition out of Absent, which is a channel gaining a
	// field it did not previously speak about.
	if before.TxFreqHz != after.TxFreqHz {
		out = append(out, spec.FieldTxFrequency)
	}
	if before.Duplex != after.Duplex {
		out = append(out, spec.FieldDuplex)
	}
	if before.OffsetHz != after.OffsetHz {
		out = append(out, spec.FieldOffset)
	}
	if before.ToneMode != after.ToneMode {
		out = append(out, spec.FieldToneMode)
	}
	if before.ToneTx != after.ToneTx {
		out = append(out, spec.FieldToneTx)
	}
	if before.ToneRx != after.ToneRx {
		out = append(out, spec.FieldToneRx)
	}
	if before.DTCSCode != after.DTCSCode {
		out = append(out, spec.FieldDTCSCode)
	}
	if before.DTCSPolarity != after.DTCSPolarity {
		out = append(out, spec.FieldDTCSPolarity)
	}
	if before.Filter != after.Filter {
		out = append(out, spec.FieldFilter)
	}
	if before.DataMode != after.DataMode {
		out = append(out, spec.FieldDataMode)
	}
	if before.TuningStepEnabled != after.TuningStepEnabled {
		out = append(out, spec.FieldTuningStepEnabled)
	}
	if before.TuningStep != after.TuningStep {
		out = append(out, spec.FieldTuningStep)
	}
	if before.ProgramTuningStepHz != after.ProgramTuningStepHz {
		out = append(out, spec.FieldProgramTuningStep)
	}
	if before.AttenuatorDB != after.AttenuatorDB {
		out = append(out, spec.FieldAttenuator)
	}
	if before.Preamp != after.Preamp {
		out = append(out, spec.FieldPreamp)
	}
	if before.Antenna != after.Antenna {
		out = append(out, spec.FieldAntenna)
	}
	if before.IPPlus != after.IPPlus {
		out = append(out, spec.FieldIPPlus)
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

// tierAddedFieldFor pairs each spec.Field the two Icom model extensions
// added with a
// predicate reporting whether this channel's data actually REQUESTS it —
// i.e. carries a Known value for it. The order is ChannelData's own
// declaration order, and it is appended after the pre-tier set in
// touchedFields, so no BlockReason a user has ever read is reordered.
//
// Every one of these predicates answers false for a channel produced by
// or for a Yaesu NEWCAT radio: those channels leave all seventeen fields
// UNAVAILABLE — a read says so directly, a load of a schema-1/2/3 file
// migrates to it (design D4, decision 1), and Absent is neither Known
// either — which is why the pre-tier world's Diff output is unchanged.
var tierAddedFieldFor = []struct {
	field   spec.Field
	present func(ChannelData) bool
}{
	{spec.FieldTxFrequency, func(d ChannelData) bool { return d.TxFreqHz.State == Known }},
	{spec.FieldDuplex, func(d ChannelData) bool { return d.Duplex.State == Known }},
	{spec.FieldOffset, func(d ChannelData) bool { return d.OffsetHz.State == Known }},
	{spec.FieldToneMode, func(d ChannelData) bool { return d.ToneMode.State == Known }},
	{spec.FieldToneTx, func(d ChannelData) bool { return d.ToneTx.State == Known }},
	{spec.FieldToneRx, func(d ChannelData) bool { return d.ToneRx.State == Known }},
	{spec.FieldDTCSCode, func(d ChannelData) bool { return d.DTCSCode.State == Known }},
	{spec.FieldDTCSPolarity, func(d ChannelData) bool { return d.DTCSPolarity.State == Known }},
	{spec.FieldFilter, func(d ChannelData) bool { return d.Filter.State == Known }},
	{spec.FieldDataMode, func(d ChannelData) bool { return d.DataMode.State == Known }},
	{spec.FieldTuningStepEnabled, func(d ChannelData) bool { return d.TuningStepEnabled.State == Known }},
	{spec.FieldTuningStep, func(d ChannelData) bool { return d.TuningStep.State == Known }},
	{spec.FieldProgramTuningStep, func(d ChannelData) bool { return d.ProgramTuningStepHz.State == Known }},
	{spec.FieldAttenuator, func(d ChannelData) bool { return d.AttenuatorDB.State == Known }},
	{spec.FieldPreamp, func(d ChannelData) bool { return d.Preamp.State == Known }},
	{spec.FieldAntenna, func(d ChannelData) bool { return d.Antenna.State == Known }},
	{spec.FieldIPPlus, func(d ChannelData) bool { return d.IPPlus.State == Known }},
}

// unconditionallyAdded is the set of fields addedFields emits for EVERY
// channel, whatever it contains — the always-transmitted six (frequency,
// mode, clarifier, ctcss_state, shift, tag). It is DERIVED from
// addedFields rather than restated, by asking it about the zero
// ChannelData: on that value every FieldState-carrying field is Absent,
// so exactly the unconditional fields come back. A conditional added to
// addedFields therefore cannot silently join this set, and an
// unconditional one cannot silently leave it
// (TestUnconditionallyAdded_IsAddedFieldsOfTheZeroChannel pins the
// membership all the same).
var unconditionallyAdded = func() map[spec.Field]bool {
	m := make(map[spec.Field]bool)
	for _, f := range addedFields(ChannelData{}) {
		m[f] = true
	}
	return m
}()

// touchedFields returns every spec.Field a write of data to bank would
// TRANSMIT or REQUEST — the set Diff's per-field gate walks. It is
// addedFields' capability-keyed successor (design D4, adjudication 10),
// and the capability key applies to exactly one of the two kinds of
// field in that set:
//
//   - the UNCONDITIONAL six (unconditionallyAdded — frequency, mode,
//     clarifier, ctcss_state, shift, tag) are filtered to the fields
//     this bank can reach. "A field the capabilities mark Unreachable in
//     that bank is not touched by an add": these fields are in the set
//     because the FRAME always carries them, not because anybody asked
//     for them, so on a bank whose frame has no room for one there is no
//     request to gate — and counting one would block every channel on
//     that bank over a field nobody named. This is the Icom case the
//     filter was written for: a bank that expresses only some of the six.
//   - the CONDITIONAL fields — ctcss_tone, tag_display and scan_skip
//     from addedFields, and the extensions' seventeen from
//     tierAddedFieldFor — are
//     in the set ONLY because this channel carries a Known value for
//     them, and a Known value IS the user's explicit request (per
//     FieldState's write rule, nothing else is ever sent). Such a field
//     is NEVER filtered out: if the bank cannot reach it, the request is
//     one the radio cannot honour, and this project's posture on that is
//     to REFUSE the channel at plan time — the per-field gate's existing
//     "not writable on this radio" BlockReason, reached because an
//     unreachable field's zero FieldSupport is not CanWrite — never to
//     drop the value and write the rest. Dropping it is what Wave-1c
//     review 1 (finding 1, HIGH) caught this filter doing to the FT-710's
//     ctcss_tone/scan_skip and the FTdx10/FTdx101's tag_display, on the
//     ordinary CSV-import-then-write route; findings 1 and 5 are one
//     rule, and this is it.
//
// For a channel produced by or for a Yaesu NEWCAT radio nothing here
// changes anything: the ten tier fields are Unavailable, and the three
// pre-tier conditionals are whatever the radio itself reported.
//
// addedFields itself is left exactly as it was, still pinned by
// TestAddedFields_MembershipAndOrder: it states which fields a write
// TRANSMITS given the data alone, which is a fact about the write frame,
// and layering the capability question on top keeps the two questions
// separable.
func touchedFields(caps spec.Capabilities, bank spec.BankID, data ChannelData) []spec.Field {
	base := addedFields(data)
	out := make([]spec.Field, 0, len(base)+len(tierAddedFieldFor))
	for _, f := range base {
		if unconditionallyAdded[f] && caps.FieldSupport(bank, f).Unreachable() {
			continue
		}
		out = append(out, f)
	}
	for _, tf := range tierAddedFieldFor {
		if !tf.present(data) {
			continue
		}
		out = append(out, tf.field)
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

// inventoryMismatch is the error Diff returns when baseline and file do
// not describe the same slots. One message, unchanged since before the
// Icom tier: what "the same slots" MEANS is what the tier widened (see
// checkInventory), not what the user is told when it fails.
func inventoryMismatch() error {
	return fmt.Errorf("codeplug: Diff: baseline and file slot inventories differ; the file must descend from a read of this radio's current layout — re-read the radio and try again")
}

// checkInventory enforces Diff's slot-inventory rule and returns, in
// FILE order, every slot the file materialised inside a SPARSE bank that
// the baseline did not hold — the adds a sparse bank legitimately
// permits (design D4, adjudication 7).
//
// The rule, stated so the two worlds are visibly one rule:
//
//   - every slot the baseline holds must appear in the file. A
//     materialised slot cannot simply vanish from a candidate: on a
//     dense bank that would be an inventory mismatch, and on a sparse
//     bank it would be an erase expressed by omission rather than by an
//     empty channel, which this project will not infer;
//   - a slot the file holds and the baseline does not is legal ONLY when
//     some sparse bank's addressable space contains it
//     (spec.Bank.WithinSpace). A sparse bank's Slots lists what a read
//     found, not what the radio can hold, so an add at an unlisted
//     address is exactly the case the sparse model exists for;
//   - NO SLOT MAY APPEAR TWICE, in either list. This clause is explicit
//     only because the rule is now stated over sets: the sorted-list
//     comparison this function replaced refused a repeat for free (the
//     repeating list was simply longer), and dropping it silently let a
//     file naming one slot twice through, with the LAST occurrence
//     winning and the diff computed against that one (Wave-1c review 1,
//     finding 3). `rigprog diff` calls Diff with no Validate pass in
//     front of it, so nothing else was left to catch a hand-edited file
//     that repeats a slot.
//
// With no sparse bank in caps — every radio registered before this tier
// — the second clause can never fire, so the rule reduces to the exact
// set equality this function replaced, reporting the identical error for
// every input that ever reached it.
func checkInventory(baseline, file *Codeplug, caps spec.Capabilities) ([]string, error) {
	inFile := make(map[string]bool, len(file.Channels))
	for _, ch := range file.Channels {
		if inFile[ch.Slot] {
			return nil, inventoryMismatch()
		}
		inFile[ch.Slot] = true
	}
	inBaseline := make(map[string]bool, len(baseline.Channels))
	for _, ch := range baseline.Channels {
		if !inFile[ch.Slot] || inBaseline[ch.Slot] {
			return nil, inventoryMismatch()
		}
		inBaseline[ch.Slot] = true
	}

	var adds []string
	for _, ch := range file.Channels {
		if inBaseline[ch.Slot] {
			continue
		}
		if !withinAnySparseBank(caps, ch.Slot) {
			return nil, inventoryMismatch()
		}
		adds = append(adds, ch.Slot)
	}
	return adds, nil
}

// withinAnySparseBank reports whether slot lies in the addressable space
// of some SPARSE bank of caps. A dense bank never answers yes here, even
// for a slot it lists: a slot the baseline did not hold but a dense bank
// does list is still an inventory mismatch — the baseline is a read of
// the whole dense bank, so its absence there means the read and the file
// disagree about the radio.
func withinAnySparseBank(caps spec.Capabilities, slot string) bool {
	for _, b := range caps.Banks {
		if b.Sparse && b.WithinSpace(slot) {
			return true
		}
	}
	return false
}

// checkSparseBudget refuses a candidate that would leave a sparse bank
// holding more POPULATED channels than the radio can (spec.Bank.Budget).
//
// It is enforced HERE, at plan time, and never on the wire, because what
// an over-budget radio actually does is undocumented on every model this
// tier registers (design D4: an ASSUMED register entry per model). A
// refusal the user can act on is the only honest answer; discovering the
// limit by sending is not.
//
// Empty channels do not count: the budget is on stored channels, and an
// empty slot stores nothing. A bank with no Budget cannot reach here —
// spec.Capabilities.Validate requires a positive Budget on every Sparse
// bank and forbids one on every dense bank.
func checkSparseBudget(file *Codeplug, caps spec.Capabilities) error {
	for _, b := range caps.Banks {
		if !b.Sparse {
			continue
		}
		populated := 0
		for _, ch := range file.Channels {
			if ch.Data == nil {
				continue
			}
			if b.WithinSpace(ch.Slot) {
				populated++
			}
		}
		if populated > b.Budget {
			return fmt.Errorf("codeplug: Diff: bank %s would hold %d populated channels, exceeding this radio's limit of %d — remove %d before sending", b.ID, populated, b.Budget, populated-b.Budget)
		}
	}
	return nil
}

// Diff compares a baseline Codeplug (read from a radio) against a
// candidate file Codeplug and reports, per slot, what would change if
// file were sent — and which of those changes this project cannot safely
// perform yet.
//
// Slot inventory: baseline and file must describe the same set of slots —
// this is how Diff enforces that file descends from a read of the same
// radio layout. A mismatch is reported as an error (not a panic).
//
// On a radio with only DENSE banks — every radio registered before the
// Icom tier — "the same set" means exactly that: set equality, reported
// with the same message it always was. On a SPARSE bank (design D4,
// adjudication 7) the file may additionally materialise a slot the
// baseline never held, anywhere inside that bank's addressable space,
// which is how a channel is ADDED to a bank whose Slots lists only what
// a read found. A baseline slot may never simply vanish from the file in
// either world: an erase is an empty channel, never an omission. See
// checkInventory.
//
// Sparse BUDGET: a candidate that would leave a sparse bank holding more
// populated channels than the radio can is refused here, at plan time,
// with an error — never sent (see checkSparseBudget).
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
//     DiffAdded entry, Diff computes "touched" as
//     touchedFields(caps, bank, after) — addedFields(after) with its
//     ALWAYS-TRANSMITTED six keyed by what this bank can actually reach,
//     plus the Icom tier's own fields where the channel carries a Known
//     value for one. A Known-conditional field is never keyed away: its
//     presence is a request, and an unreachable one is refused right
//     here rather than dropped (see touchedFields; for every radio
//     registered before that tier the two answers are identical) —
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
// Determinism: Entries appear in baseline.Channels slice order, followed
// by any sparse-bank slot the file materialised and the baseline did not,
// in file.Channels slice order. On a radio with no sparse bank the second
// group is always empty and this is exactly the pre-tier rule (every slot
// in file was then guaranteed present in baseline). Either way the same
// two inputs always produce the same Entries in the same order.
func Diff(baseline, file *Codeplug, caps spec.Capabilities) (DiffResult, error) {
	sparseAdds, err := checkInventory(baseline, file, caps)
	if err != nil {
		return DiffResult{}, err
	}
	if err := checkSparseBudget(file, caps); err != nil {
		return DiffResult{}, err
	}

	fileBySlot := make(map[string]Channel, len(file.Channels))
	for _, ch := range file.Channels {
		fileBySlot[ch.Slot] = ch
	}

	entries := make([]DiffEntry, 0, len(baseline.Channels)+len(sparseAdds))
	var added, modified, erased, unchanged, blocked int

	// Baseline order first, then any sparse-bank slot the file
	// materialised that the baseline never held, in FILE order — see
	// this function's Determinism paragraph.
	order := make([]Channel, 0, len(baseline.Channels)+len(sparseAdds))
	order = append(order, baseline.Channels...)
	for _, slot := range sparseAdds {
		order = append(order, Channel{Slot: slot})
	}

	for _, baseCh := range order {
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
				touched := touchedFields(caps, bankID, *after)
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
