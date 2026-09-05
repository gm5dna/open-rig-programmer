// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// bankFor reports which of this session's banks claims slot.
func (s *Session) bankFor(slot string) (spec.BankID, bool) {
	for _, b := range s.caps.Banks {
		for _, sl := range b.Slots {
			if sl == slot {
				return b.ID, true
			}
		}
	}
	return "", false
}

// requestedFields lists every spec.Field a write of data actually
// requests: the six plain fields are ALWAYS requested (the MW frame
// carries frequency/mode/clarifier/ctcss-state/shift whether or not they
// changed, and the MT frame likewise carries the tag), plus TagDisplay,
// CTCSSTone and ScanSkip when — and only when — their FieldState is
// Known: per codeplug's write rule, Unknown/Unavailable mean "preserve
// whatever the radio has", i.e. nothing is requested for that field.
//
// This mirrors the DIFF LAYER'S REQUESTED-SET DERIVATION — same membership,
// the same conditionals, the same order — so the driver's defence-in-depth
// gate and the diff layer's gate judge the same set. That derivation is two
// pieces on the codeplug side and both are mirrored here: addedFields' six
// unconditional plus three conditional fields, and then the SEVENTEEN Icom-tier
// conditionals codeplug carries in tierAddedFieldFor and appends in
// touchedFields. The seventeen come LAST, in ChannelData's declaration order,
// exactly as they do there, so no BlockReason a user has ever read is
// reordered by their arrival — and so a caller who marks a tier field Known
// meets the capability gate below rather than having the value silently
// omitted from the frame.
// TagDisplay keeps the PLACE it held while it was unconditional: after
// Tag, before the tone/skip conditionals, i.e. seventh whenever it appears
// at all (TestRequestedFields_MembershipAndOrder pins this, as
// TestAddedFields_MembershipAndOrder pins the other side).
//
// TagDisplay's conditional needs a word its two neighbours do not, because
// MT's display flag (P1) is MANDATORY on the frame: a non-Known TagDisplay
// is never quietly omitted from the wire, it is REFUSED outright by
// buildWriteCommands before any other field mapping. Dropping it from this
// set therefore cannot let a non-Known value through — what it fixes is
// the one channel that would otherwise meet the wrong gate first: on a
// session whose FieldTagDisplay is not write-Supported, the loop below
// would have refused it naming a not-writable field NOBODY ASKED TO
// WRITE, instead of the refusal that names the real problem. (addedFields
// carries the conditional for the same reason — see its doc comment.)
func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldClarifier,
		spec.FieldCTCSSState,
		spec.FieldShift,
		spec.FieldTag,
	}
	if data.TagDisplay.State == codeplug.Known {
		fields = append(fields, spec.FieldTagDisplay)
	}
	if data.CTCSSTone.State == codeplug.Known {
		fields = append(fields, spec.FieldCTCSSTone)
	}
	if data.ScanSkip.State == codeplug.Known {
		fields = append(fields, spec.FieldScanSkip)
	}
	for _, t := range tierRequestedFields {
		if t.present(data) {
			fields = append(fields, t.field)
		}
	}
	return fields
}

// tierRequestedFields pairs each spec.Field the Icom tier added with a
// predicate reporting whether this channel's data actually REQUESTS it —
// i.e. carries a Known value for it. It is the mirror of codeplug's
// tierAddedFieldFor (diff.go), down to the order: ChannelData's own
// declaration order, appended AFTER the pre-tier set.
//
// Every one of these predicates answers false for a channel this driver
// produced: an FT-710 read leaves all seventeen UNAVAILABLE, and a load of a
// schema-1/2/3 file migrates to the same. So the ordinary write is
// unchanged by their presence, and what they add is the one case the gate
// promised to cover and did not — a caller who hands WriteChannel a
// ChannelData with a Known tier value, which this codec cannot express and
// must therefore REFUSE rather than drop.
//
// SEVENTEEN, and this table used to carry only the first TEN: the
// pre-D8-wave count, unchanged here when codeplug's tierAddedFieldFor gained
// the seven receiver-tier fields (TuningStepEnabled through IPPlus). A
// channel carrying a Known TuningStep, Preamp or Antenna was therefore
// written with the value silently DROPPED — nothing named the field, so the
// capability gate never saw it and this radio's record has no position for
// it. That is a breach of "an omitted config semantic is REFUSED, never
// defaulted"; the write-gate sweep's item (g) closes it, as the FT-891's own
// HIGH-1 fix did. TestWriteChannel_KnownD8TierFieldsRefusedBeforeWire is the
// behavioural pin.
//
// Mirrored, NOT imported: tierAddedFieldFor is unexported, and the mirror is
// held by both sides pinning the same shape
// (TestRequestedFields_MembershipAndOrder here, codeplug's own tests there).
// That test's "the seventeen are exactly spec.AllFields()'s tier-added tail"
// subtest pins the membership against an INDEPENDENT derivation — the same
// exported spec.Field constants tierAddedFieldFor is itself built from —
// rather than against a count, so a future spec.Field this table fails to
// mirror fails there rather than passing silently.
var tierRequestedFields = []struct {
	field   spec.Field
	present func(codeplug.ChannelData) bool
}{
	{spec.FieldTxFrequency, func(d codeplug.ChannelData) bool { return d.TxFreqHz.State == codeplug.Known }},
	{spec.FieldDuplex, func(d codeplug.ChannelData) bool { return d.Duplex.State == codeplug.Known }},
	{spec.FieldOffset, func(d codeplug.ChannelData) bool { return d.OffsetHz.State == codeplug.Known }},
	{spec.FieldToneMode, func(d codeplug.ChannelData) bool { return d.ToneMode.State == codeplug.Known }},
	{spec.FieldToneTx, func(d codeplug.ChannelData) bool { return d.ToneTx.State == codeplug.Known }},
	{spec.FieldToneRx, func(d codeplug.ChannelData) bool { return d.ToneRx.State == codeplug.Known }},
	{spec.FieldDTCSCode, func(d codeplug.ChannelData) bool { return d.DTCSCode.State == codeplug.Known }},
	{spec.FieldDTCSPolarity, func(d codeplug.ChannelData) bool { return d.DTCSPolarity.State == codeplug.Known }},
	{spec.FieldFilter, func(d codeplug.ChannelData) bool { return d.Filter.State == codeplug.Known }},
	{spec.FieldDataMode, func(d codeplug.ChannelData) bool { return d.DataMode.State == codeplug.Known }},
	{spec.FieldTuningStepEnabled, func(d codeplug.ChannelData) bool { return d.TuningStepEnabled.State == codeplug.Known }},
	{spec.FieldTuningStep, func(d codeplug.ChannelData) bool { return d.TuningStep.State == codeplug.Known }},
	{spec.FieldProgramTuningStep, func(d codeplug.ChannelData) bool { return d.ProgramTuningStepHz.State == codeplug.Known }},
	{spec.FieldAttenuator, func(d codeplug.ChannelData) bool { return d.AttenuatorDB.State == codeplug.Known }},
	{spec.FieldPreamp, func(d codeplug.ChannelData) bool { return d.Preamp.State == codeplug.Known }},
	{spec.FieldAntenna, func(d codeplug.ChannelData) bool { return d.Antenna.State == codeplug.Known }},
	{spec.FieldIPPlus, func(d codeplug.ChannelData) bool { return d.IPPlus.State == codeplug.Known }},
}

// WriteChannel implements driver.Session: MW (channel data) then MT
// (tag), both fire-and-forget with the transport's bounded "?;" listen.
//
// Refusal comes FIRST — before ANY wire traffic. This is deliberate
// defence in depth below the clone service: even if every layer above
// failed, this method re-derives the channel's requested field set
// (requestedFields) and re-checks each against THIS session's
// capabilities (FieldSupport.CanWrite, with spec.Inert additionally
// acceptable-to-transmit — see the gate below). In particular:
//
//   - On the all-Unverified fail-safe profile NOTHING is writable (the
//     six rw fields are Unverified), so every channel is refused here —
//     the clarifier's Inert marking alone can never unlock a write.
//   - A Known CTCSS tone or scan skip is refused even on the Simulated
//     profile: the CAT codec cannot express either, and silently
//     dropping a value the caller explicitly marked Known would be a
//     lie. The same holds for any of the SEVENTEEN Icom-tier fields
//     (requestedFields' tierRequestedFields): none appears in this
//     radio's capability map at all, so FieldSupport answers the zero
//     FieldSupport and the gate below refuses. That is not an accident of
//     the map — it is what makes the sentence above true for the tier as
//     well as for the tone and skip.
//   - A TagDisplay that is not Known is refused, in buildWriteCommands and
//     before ANY other field mapping: MT's display flag is mandatory, so
//     there is no way to send the channel without inventing a value for it
//     (see buildWriteCommands). codeplug.Diff blocks such a channel at plan
//     time; this refusal is the defence-in-depth behind that.
//   - An empty channel (erase) is refused: no CAT erase command exists
//     (HW-CONFIRMED 2026-07-13 by a properly isolated re-probe — four
//     range/mode-isolated candidate MW frames, every one rejected; see
//     docs/hardware-notes.md's "No CAT erase" section), and FieldErase is
//     nowhere write-Supported.
//   - The clarifier (spec.Inert) is NOT refused here, whatever its
//     value: the radio provably ignores it, so transmitting it is
//     harmless — and this method lacks the baseline needed to tell a
//     changed value from an unchanged one. codeplug.Diff owns that
//     half of the Inert rule (see spec.Inert's doc comment).
//
// Kind-on-write: the MW frame's P7 is the SESSION DIALECT's declared write
// kind (cat.Dialect.MWWriteKind, consulted since M9c-3 task 9), which for
// the FT-710 is ALWAYS '1' (KindMemory), for both memory and PMS slots.
// HW-CONFIRMED 2026-07-13 (M5b write trials,
// docs/hardware-notes.md): the former ASSUMED pairing (KindPMS '5' for a
// PMS slot) is hardware-refuted — the radio REJECTS a PMS write carrying
// KindPMS with an immediate "?;", and accepts the identical write when
// it carries KindMemory instead. cat.Dialect.BuildMWSet enforces the
// same rule (see core/cat/mw.go) and fakeradio mirrors the radio's
// rejection (see internal/fakeradio/parser.go's handleMW).
//
// NO read-back: WriteChannel reports only sent/unrejected (see
// driver.WriteResult). Reading the slot back and comparing is the clone
// service's job — the boundary is deliberate, so verification policy
// (when, how often, what to do on mismatch) lives in one place above
// every driver rather than being half-implemented inside each.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	// Fix 2: hold opMu for the WHOLE MW+MT sequence, not just around each
	// individual Do call — see Session's doc comment. Taken even before
	// the refusal checks below: a refused write returns fast (no wire
	// traffic) either way, so there is no cost to holding it uniformly.
	s.opMu.Lock()
	defer s.opMu.Unlock()

	// Every refusal below this point returns res unchanged, i.e. an
	// EXPLICITLY EMPTY step list — never nil. The distinction is not
	// cosmetic: the clone service journals this result, and a nil slice
	// marshals as JSON null, which an auditor would have to read as
	// "unknown" rather than the truth, "no frame was ever built, so
	// nothing was attempted".
	res := driver.WriteResult{Steps: []driver.WriteStep{}}

	if _, err := s.dialect.ParseSlot(ch.Slot); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: fmt.Sprintf("not a valid slot: %v", err)}
	}
	bank, ok := s.bankFor(ch.Slot)
	if !ok {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: "slot is not part of any bank this session supports"}
	}

	if ch.Empty() {
		// An empty channel is an ERASE request. FieldErase is nowhere
		// write-Supported, and M5b settled WHY for good: NO CAT erase
		// exists — a properly isolated 13/07/2026 re-probe (four
		// range/mode-isolated candidate MW frames, every one rejected;
		// see docs/hardware-notes.md's "No CAT erase" section)
		// HW-CONFIRMED this permanently, and this codec has no erase
		// command to express one with either. Erased entries stay
		// Blocked by design (Erased->Blocked, confirmed correct).
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "erase cannot be expressed by the CAT codec, and FieldErase is not write-Supported",
		}
	}

	// FieldState sanity before anything else trusts .State: a malformed
	// field (unknown State, or a non-Known value smuggled alongside) is
	// refused, not interpreted.
	//
	// EVERY FieldState field of ch.Data, not the three (CTCSSTone,
	// ScanSkip, TagDisplay) this rung used to check — the write-gate
	// sweep's item (i), and the same gap the FT-891's C-M1 found: a tier
	// field carrying State Unavailable and a non-zero Value passed both
	// codeplug.Validate (which skips every non-Recorded field outright)
	// and the old three-field list, and requestedFields is Known-only, so
	// nothing ever named it and the malformed value was silently DROPPED
	// from the frame rather than refused.
	// TestWriteChannel_IncoherentFieldStateRefusedBeforeWire is the pin,
	// and its comment quotes the pre-fix transcript (both frames sent,
	// TxFreqHz simply absent from them).
	//
	// THE WALK IS THE FLEET'S, driver.CheckFieldStates, not a fourth
	// private copy: its doc comment carries the five-rule stance, of which
	// the load-bearing one HERE is that codeplug.Absent WITH A ZERO VALUE
	// is ADMITTED. Every ChannelData this driver's own read path produces
	// leaves the tier fields Unavailable, but a hand-built one — the
	// GUI's, a test's, a caller's composite literal — leaves them Absent,
	// and refusing those would refuse every ordinary MODIFY.
	// TestWriteChannel_AbsentFieldStatesStillWrite is that half's pin;
	// TestWriteChannel_AbsentFieldStateWithValueRefusedBeforeWire pins the
	// erratum's other half — an Absent field carrying a non-zero Value is
	// still refused (MEDIUM-1, Opus review 05/09/2026).
	//
	// TagDisplay is the ONE field this rung does not settle on its own:
	// MT's P1 flag is mandatory, so a non-Known value — Absent included —
	// is refused by buildWriteCommands further down
	// (TestWriteChannel_NonKnownTagDisplayRefusedBeforeWire). That is a
	// question about a well-formed field and belongs there, not here.
	if field, err := driver.CheckFieldStates(s.caps, *ch.Data); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{field}, Reason: err.Error()}
	}

	// THE write gate (defence in depth below the clone service): every
	// requested field must pass spec.FieldSupport.CanWrite for this slot's
	// bank in THIS session's capabilities — spec.Supported, or
	// spec.ConsentedUnverified, the label a session assembled under the
	// user's recorded consent carries (sessionCapabilities, ft710.go; on
	// THIS radio that transform is a proven no-op, so the consented arm is
	// currently unreachable here and is kept because the gate states the
	// neutral rule, not this radio's luck) — OR spec.Inert, which is
	// acceptable to TRANSMIT (M5b, HW-CONFIRMED 2026-07-13: the radio ignores the
	// clarifier's transmitted value entirely, so transmitting it cannot
	// alter the radio's state). The Inert enforcement split, documented at
	// both ends (see spec.Inert): blocking a CHANGED Inert value needs the
	// BASELINE to compare against, and this method holds only the channel
	// — that half of the rule lives in codeplug.Diff, which has both
	// sides; this defence-in-depth re-check enforces everything decidable
	// from the channel alone.
	var unwritable []spec.Field
	for _, f := range requestedFields(*ch.Data) {
		fs := s.caps.FieldSupport(bank, f)
		if !fs.CanWrite() && fs.Write != spec.Inert {
			unwritable = append(unwritable, f)
		}
	}
	if len(unwritable) > 0 {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: unwritable,
			Reason: "not write-Supported for this session (the CAT codec cannot express the field, or this session's capability profile does not support writing it)",
		}
	}

	// Build BOTH frames before any wire traffic, so a mapping/validation
	// failure in either can still refuse the whole write cleanly.
	mwCmd, mtCmd, err := buildWriteCommands(s.dialect, ch)
	if err != nil {
		return res, err
	}

	// THE step list, declared in full HERE: after both frames provably
	// exist, before either goes near the wire. Declaring the whole
	// choreography up front is what makes a partial outcome legible — an
	// MT step that is present but never Sent says "the tag write was part
	// of this write and never went out", where a step list appended to as
	// frames succeed could only say nothing at all, which is
	// indistinguishable from a driver that never intended an MT.
	res.Steps = []driver.WriteStep{{Command: "MW"}, {Command: "MT"}}
	const (
		mwStep = 0
		mtStep = 1
	)

	// MW first: channel data before its label. Fire-and-forget — the
	// transport listens for a bounded window for a delayed "?;".
	if _, err := s.eng.Do(ctx, mwCmd, fnfSpec()); err != nil {
		if errors.Is(err, cat.ErrRejected) {
			// The frame WAS transmitted; the radio explicitly refused it.
			res.Steps[mwStep].Sent = true
			return res, fmt.Errorf("ft710: WriteChannel %s: MW rejected by radio: %w", ch.Slot, err)
		}
		// Transport-level failure: the frame's fate is not attributable,
		// so Sent stays false.
		return res, fmt.Errorf("ft710: WriteChannel %s: MW: %w", ch.Slot, err)
	}
	res.Steps[mwStep].Sent, res.Steps[mwStep].Confirmed = true, true

	if _, err := s.eng.Do(ctx, mtCmd, fnfSpec()); err != nil {
		if errors.Is(err, cat.ErrRejected) {
			res.Steps[mtStep].Sent = true
			return res, fmt.Errorf("ft710: WriteChannel %s: MT rejected by radio: %w", ch.Slot, err)
		}
		return res, fmt.Errorf("ft710: WriteChannel %s: MT: %w", ch.Slot, err)
	}
	res.Steps[mtStep].Sent, res.Steps[mtStep].Confirmed = true, true

	return res, nil
}

// buildWriteCommands maps a populated channel onto its MW and MT Set
// frames, refusing (typed, via *driver.WriteRefusedError) any value the
// codec cannot express. Called only after WriteChannel's capability gate
// has passed.
func buildWriteCommands(dialect cat.Dialect, ch codeplug.Channel) (mwCmd, mtCmd cat.Command, err error) {
	sl, err := dialect.ParseSlot(ch.Slot)
	if err != nil {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: err.Error()}
	}
	data := *ch.Data

	// THE pre-wire refusal for TagDisplay, FIRST and before any other field
	// mapping. The MT frame's display flag (P1) is MANDATORY — the frame has
	// no "leave it alone" encoding — so sending a channel whose TagDisplay is
	// not Known would MANUFACTURE a value for a field whose FieldState says
	// "preserve whatever the radio has", which is exactly what codeplug's
	// write rule forbids.
	//
	// Position is load-bearing, not stylistic: a channel that is wrong in
	// several ways at once must still name THIS field, because this is the
	// one whose failure mode is a silent wrong byte on the wire rather than
	// a refusal. From this commit there is no path from here to BuildMTSet
	// that carries a non-Known display flag.
	//
	// codeplug.Diff blocks such a channel at PLAN time, which is the
	// user-facing route and the one that produces a helpful message; this is
	// the belt to that pair of braces, in the same spirit as WriteChannel's
	// capability gate.
	if data.TagDisplay.State != codeplug.Known {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldTagDisplay},
			Reason: fmt.Sprintf("tag display FieldState is %q, not %q; only a Known value is ever sent to a radio", data.TagDisplay.State, codeplug.Known),
		}
	}

	// Resolved through THIS dialect (task 67, M9c-0), not a driver-private
	// table: before this, a dialect that renamed a mode had no effect on
	// what got written — see modeTable's doc comment (caps.go) and
	// cat.Dialect.ModeByName's own for the finding this closes.
	mode, ok := dialect.ModeByName(data.Mode)
	if !ok {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldMode},
			Reason: fmt.Sprintf("mode %q is not a mode this radio supports", data.Mode),
		}
	}
	ctcss, ok := ctcssByName[data.CTCSS]
	if !ok {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldCTCSSState},
			Reason: fmt.Sprintf("ctcss state %q is not one of OFF/ENC-DEC/ENC", data.CTCSS),
		}
	}
	shift, ok := shiftByName[data.Shift]
	if !ok {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldShift},
			Reason: fmt.Sprintf("shift %q is not one of SIMPLEX/PLUS/MINUS", data.Shift),
		}
	}
	// Bounds-check BEFORE the int -> int16 conversion below can wrap; the
	// builder re-validates magnitude and step on top.
	//
	// The bound is THIS DIALECT'S (M9c-3 task 9), in the comparison and in
	// the message alike: both hardcoded +-9990 until now, so a receiver with
	// a narrower range had its over-range values waved past this check, and
	// a wider one had its legitimate values refused here with a bound that
	// was never its own. cat.FT710's own MaxAbsHz is 9990, so this renders
	// byte-identically for the FT-710.
	clar := dialect.Clarifier()
	if data.ClarHz < -clar.MaxAbsHz || data.ClarHz > clar.MaxAbsHz {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldClarifier},
			Reason: fmt.Sprintf("clarifier %d Hz exceeds +/-%d Hz", data.ClarHz, clar.MaxAbsHz),
		}
	}

	// Kind-on-write: THIS DIALECT'S declared write kind, for both memory and
	// PMS slots. The FT-710's is KindMemory ('1'), HW-CONFIRMED 2026-07-13
	// (see WriteChannel's doc comment) — but that evidence is about one
	// radio, so since M9c-3 task 9 the byte comes from the receiver rather
	// than a cat.KindMemory literal here, which wrote the FT-710's finding
	// onto whatever dialect this path was handed and had
	// cat.Dialect.BuildMWSet refuse another receiver's legitimate write.
	// Discovered banks (5xx/EMG) can never reach here — their fields are
	// read-only, so the capability gate refused them already;
	// cat.Dialect.BuildMWSet would reject their slots too (its own
	// writableSlot rule).
	// The ONE checked conversion between the neutral model's uint64
	// frequency and this protocol's uint32 (design D4, item 7):
	// core/cat stays uint32 because a NEWCAT memory frame carries nine
	// digits and can express nothing wider, so a bare cast here would
	// truncate an out-of-range value into a plausible small one and send
	// it. The arm is unreachable for this radio — Validate has already
	// refused anything above its 75 MHz ceiling — and it is a refusal,
	// not a cast, so it stays unreachable by construction rather than by
	// habit.
	freqHz, err := cat.MemoryFreqHz(data.FreqHz)
	if err != nil {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldFrequency}, Reason: err.Error()}
	}

	mwCmd, err = dialect.BuildMWSet(cat.MemoryData{
		Slot:   sl,
		FreqHz: freqHz,
		ClarHz: int16(data.ClarHz),
		RxClar: data.RxClar,
		TxClar: data.TxClar,
		Mode:   mode,
		Kind:   dialect.MWWriteKind(),
		CTCSS:  ctcss,
		Shift:  shift,
	})
	if err != nil {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: fmt.Sprintf("cannot encode MW frame: %v", err)}
	}

	// data.TagDisplay.Value is safe to read here and ONLY here: the refusal
	// at the top of this function has already established State == Known.
	mtCmd, err = dialect.BuildMTSet(sl, data.TagDisplay.Value, data.Tag)
	if err != nil {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldTag},
			Reason: fmt.Sprintf("cannot encode MT frame: %v", err),
		}
	}
	return mwCmd, mtCmd, nil
}
