// SPDX-License-Identifier: GPL-3.0-or-later

package driver

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// FieldStateCheck pairs one spec.Field with the verdict the fleet's
// FieldState stance reached for it. A nil Err is an admission, not a
// silence: the walk looked at the field and found nothing to refuse.
type FieldStateCheck struct {
	// Field is the spec.Field the ChannelData member belongs to — the
	// field a refusal names.
	Field spec.Field
	// Err is nil when the field is coherent, and otherwise the typed
	// validator's own sentence, which a driver puts straight into its
	// WriteRefusedError.Reason.
	Err error
}

// FieldStateChecks walks EVERY spec.Field whose codeplug.ChannelData
// counterpart carries a codeplug FieldState (codeplug.FreqField,
// BoolField, ToneField, StringField or IntField) — TWENTY of
// spec.AllFields()'s twenty-seven — and returns, in ChannelData's own
// declaration order, each field paired with the fleet stance's verdict on
// it. The other seven (FieldFrequency, FieldMode, FieldClarifier,
// FieldCTCSSState, FieldShift, FieldTag, FieldErase) are ChannelData's
// plain uint64/string/int members (FreqHz, Mode, ClarHz, CTCSS, Shift,
// Tag) or, for FieldErase, no struct member at all — none carries a
// FieldState and none has a Valid to call.
//
// THE FLEET STANCE, in five rules and no more (the 05/09/2026 write-gate
// sweep's decision (i), ERRATUM 05/09/2026 11:30 from the Opus review's
// MEDIUM-1; the IC-9700's documented stance,
// core/driver/ic9700/write.go's validateKnownValues, SHARPENED by the
// FT-891 closing review's C-M1 finding and then by MEDIUM-1):
//
//   - codeplug.Known — the field's own Valid() decides. For a
//     ToneField/StringField/IntField that always means BOTH the
//     coherence question (a value that fits its state) and the DOMAIN
//     question (a value this radio can express, judged against caps' own
//     vocabulary or table); a Known BoolField or FreqField's Valid()
//     answers only the coherence question — those two types carry no
//     vocabulary of their own, so their domain question belongs to the
//     capability gate one rung down and, for TxFreqHz, to the codec;
//   - codeplug.Unknown or codeplug.Unavailable carrying a NON-ZERO
//     Value — REFUSED as incoherent. Both states mean "preserve whatever
//     the radio has"; a value alongside one is a claim about what that
//     preserved state should be, which is not what either state means,
//     and nothing further down a write path would ever look at it. This
//     is C-M1: a malformed value must be refused, never silently dropped
//     from the frame;
//   - codeplug.Unknown or codeplug.Unavailable with a ZERO Value, and
//     codeplug.Absent WITH A ZERO VALUE — ADMITTED. Absent is the ZERO
//     FieldState, what a hand-built ChannelData leaves behind, and a
//     caller who set nothing has requested nothing. This is the rule the
//     IC-9700's stance insists on and it is why the walk cannot simply
//     call Valid on every field: codeplug's typed validators reject
//     Absent OUTRIGHT (see codeplug.Absent's own doc comment), so an
//     unconditional walk would refuse every ordinary MODIFY;
//   - codeplug.Absent carrying a NON-ZERO Value — REFUSED as incoherent,
//     the SAME malformation C-M1 names for Unknown/Unavailable. MEDIUM-1
//     (Opus review): Absent is the zero FieldState, so
//     ToneField{Value: 1000} — a caller who set a value and forgot to set
//     State — IS Absent, and the pre-erratum walk let it straight
//     through: both this radio's write frames went out with the value
//     silently dropped, where the pre-sweep code had refused the same
//     channel outright. A value with no state recorded is not a claim
//     this project can act on, exactly as one alongside Unknown or
//     Unavailable is not;
//   - any other State string — REFUSED, by the same Valid() call, since
//     an unrecognised state is not a claim this project can act on.
//
// The reachable/unreachable question is NOT re-decided here: whether an
// Absent field on a REACHABLE slot is acceptable is codeplug.Validate's,
// and it refuses one (the parked sweep's C6). A driver asking this walk
// asks only whether the data it holds is internally coherent and inside
// this radio's domains.
//
// caps IS THIS RADIO'S OWN, not a standard chart: the vocabulary and table
// members it supplies (CTCSSTones, CTCSSToneRange, DuplexOptions,
// ToneModes, DTCSCodes, DTCSPolarities, Filters, TuningSteps,
// AttenuatorDB, PreampOptions, AntennaOptions) are what each
// ToneField/StringField/IntField is judged against — a ToneField via
// caps.AdmitsTone, which reads CTCSSTones or CTCSSToneRange (never both,
// spec.Capabilities.Validate's own rule). A radio declaring an EMPTY
// vocabulary for a field fails closed on every Known value for it —
// StringField.Valid's and IntField.Valid's
// own documented rule, and the right answer for a radio whose record has
// no room for the field at all. The COHERENCE half of the stance is
// settled without consulting caps, so an empty vocabulary changes no
// incoherent channel's outcome.
//
// ORDER is ChannelData's own declaration order (codeplug/channel.go),
// which is also, independently, spec.AllFields()'s order with the seven
// plain fields filtered out;
// TestFieldStateWalk_CoversEveryFieldStateField pins that identity — not
// merely a count — so a spec.Field this walk forgets to mirror, or that
// AllFields gains and this walk does not, fails that test rather than
// silently letting a future field's malformed value through.
//
// ONE walk, shared, rather than a copy per driver: it lives HERE, in
// core/driver, because core/driver already imports core/codeplug (the
// Session interface takes a codeplug.Channel), so there is no cycle to
// avoid, and because a stance the fleet agreed once should not be four
// tables able to drift apart. core/driver/internal/drivertest was
// considered and rejected: that package is test-only, and this is a
// production write-path rung.
func FieldStateChecks(caps spec.Capabilities, d codeplug.ChannelData) []FieldStateCheck {
	duplex := make([]string, len(caps.DuplexOptions))
	for i, o := range caps.DuplexOptions {
		duplex[i] = o.Value
	}
	toneModes := make([]string, len(caps.ToneModes))
	for i, m := range caps.ToneModes {
		toneModes[i] = m.Value
	}
	return []FieldStateCheck{
		{spec.FieldCTCSSTone, judge(d.CTCSSTone.State, d.CTCSSTone.Value, func() error { return d.CTCSSTone.Valid(caps) })},
		{spec.FieldTagDisplay, judge(d.TagDisplay.State, d.TagDisplay.Value, d.TagDisplay.Valid)},
		{spec.FieldScanSkip, judge(d.ScanSkip.State, d.ScanSkip.Value, d.ScanSkip.Valid)},
		{spec.FieldTxFrequency, judge(d.TxFreqHz.State, d.TxFreqHz.Value, d.TxFreqHz.Valid)},
		{spec.FieldDuplex, judge(d.Duplex.State, d.Duplex.Value, func() error { return d.Duplex.Valid(duplex) })},
		{spec.FieldOffset, judge(d.OffsetHz.State, d.OffsetHz.Value, d.OffsetHz.Valid)},
		{spec.FieldToneMode, judge(d.ToneMode.State, d.ToneMode.Value, func() error { return d.ToneMode.Valid(toneModes) })},
		{spec.FieldToneTx, judge(d.ToneTx.State, d.ToneTx.Value, func() error { return d.ToneTx.Valid(caps) })},
		{spec.FieldToneRx, judge(d.ToneRx.State, d.ToneRx.Value, func() error { return d.ToneRx.Valid(caps) })},
		{spec.FieldDTCSCode, judge(d.DTCSCode.State, d.DTCSCode.Value, func() error { return d.DTCSCode.Valid(caps.DTCSCodes) })},
		{spec.FieldDTCSPolarity, judge(d.DTCSPolarity.State, d.DTCSPolarity.Value, func() error { return d.DTCSPolarity.Valid(caps.DTCSPolarities) })},
		{spec.FieldFilter, judge(d.Filter.State, d.Filter.Value, func() error { return d.Filter.Valid(caps.Filters) })},
		{spec.FieldDataMode, judge(d.DataMode.State, d.DataMode.Value, d.DataMode.Valid)},
		{spec.FieldTuningStepEnabled, judge(d.TuningStepEnabled.State, d.TuningStepEnabled.Value, d.TuningStepEnabled.Valid)},
		{spec.FieldTuningStep, judge(d.TuningStep.State, d.TuningStep.Value, func() error { return d.TuningStep.Valid(caps.TuningSteps) })},
		{spec.FieldProgramTuningStep, judge(d.ProgramTuningStepHz.State, d.ProgramTuningStepHz.Value, d.ProgramTuningStepHz.Valid)},
		{spec.FieldAttenuator, judge(d.AttenuatorDB.State, d.AttenuatorDB.Value, func() error { return d.AttenuatorDB.Valid(caps.AttenuatorDB) })},
		{spec.FieldPreamp, judge(d.Preamp.State, d.Preamp.Value, func() error { return d.Preamp.Valid(caps.PreampOptions) })},
		{spec.FieldAntenna, judge(d.Antenna.State, d.Antenna.Value, func() error { return d.Antenna.Valid(caps.AntennaOptions) })},
		{spec.FieldIPPlus, judge(d.IPPlus.State, d.IPPlus.Value, d.IPPlus.Valid)},
	}
}

// judge applies the fleet stance's Absent rule and then defers to the
// field's own typed validator for everything else.
//
// It is ONE function rather than an `if` at each of the twenty call sites
// because the Absent rule is the whole difference between this stance and
// an unconditional walk, and a rule stated twenty times is a rule that can
// be forgotten once. valid is a thunk so that the typed validators'
// differing signatures (some take caps, some a vocabulary, some nothing)
// stay at the call site where the right argument is obvious.
//
// value IS THE FIELD'S OWN Value member, generic over the five FieldState
// value types (spec.Tone, bool, uint64, string, int — comparable is the
// only bound any of them needs). MEDIUM-1 (Opus review, 05/09/2026): the
// old judge looked only at state, so codeplug.Absent — the ZERO
// FieldState — was admitted regardless of Value, and ToneField{Value: 1000}
// (a caller who set a value and forgot to set State) passed straight
// through with the value silently dropped from the frame. Absent's
// zero-Value check needs to be right for each type's own zero (0, "",
// false, spec.Tone(0)), and none of core/codeplug's five FieldState types
// offers an IsZero helper — a generic zero comparison against the type the
// caller already has in hand (its own exported Value field) is the honest
// place for that check, not a new method added to core/codeplug for this
// one caller.
func judge[T comparable](state codeplug.FieldState, value T, valid func() error) error {
	if state == codeplug.Absent {
		var zero T
		if value != zero {
			return fmt.Errorf("driver: Absent field must have zero Value, got %v — a value with no State recorded is the same malformation Unknown/Unavailable refuse when they carry one", value)
		}
		return nil
	}
	return valid()
}

// CheckFieldStates is FieldStateChecks reduced to the answer a write
// ladder wants: the FIRST field whose state or value is incoherent, and
// the reason, or the zero Field and a nil error when every field is
// coherent.
//
// FIRST, in ChannelData's declaration order, so a channel broken in
// several ways at once is refused for one field deterministically —
// TestCheckFieldStates_ReportsTheFirstIncoherentField is the pin. Which
// one it is matters only to the message; that a refusal happens at all
// does not depend on the order.
func CheckFieldStates(caps spec.Capabilities, d codeplug.ChannelData) (spec.Field, error) {
	for _, c := range FieldStateChecks(caps, d) {
		if c.Err != nil {
			return c.Field, c.Err
		}
	}
	return "", nil
}
