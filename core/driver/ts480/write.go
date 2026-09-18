// SPDX-License-Identifier: GPL-3.0-or-later

package ts480

import (
	"context"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// The ASSUMED-register entry and the open question this file's SEMANTIC
// refusal names. They are cited BY NAME rather than by position, and the
// authoritative register is core/kw/doc.go's — no entry is re-registered here.
//
// A REGISTER ENTRY IS PART OF THE REFUSAL, NOT A COMMENT ON IT. Every rung of
// this ladder answers "refused", and on an unconsented RealHardware session
// the CAPABILITY gate answers first for every write (writeTrialsComplete is
// false), so a caller — or a test — that could see only the fact of refusal
// could not tell an unlifted assumption from a missing consent.
// RefusalError.Register and RefusalError.Causes are what distinguish them.
const (
	// registerA22 is the reason there is no TS-480 channel write at all:
	// bytes 39-40 are a STEP INDEX on this row, "Step size. Refer to the ST
	// command." (480:979), and ST's legend is MODE-CONDITIONAL over two
	// different ranges — 00 ~ 04 for SSB/CW/FSK and 00 ~ 09 for AM/FM, with
	// index 00 meaning 0.5 kHz in the first and 5 kHz in the second
	// (480:1494-1500). Capabilities.TuningSteps is a flat label list with no
	// mode axis, so no honest vocabulary exists and FieldTuningStep is
	// published Unsupported — which means the source channel never retains
	// the raw P14 index, so this programme cannot tell a non-default step
	// from a default one and has nothing to write into those two bytes but a
	// guess. LIFT: L-HW-16, ONE TRIAL PER ST MODE CLASS, and a partial result
	// lifts nothing.
	registerA22 = "A22"
	// registerQ2 is the tone half, and it is a SUBSIDIARY cause rather than a
	// rung — see WriteChannel. This book prints neither tone chart ("Refer to
	// page 32 of the TS-480 instruction manual", 480:1559-1560; page 33 for
	// CN, 480:339-340), so CTCSSTones is nil, AdmitsTone fails closed and a
	// channel carrying a tone cannot be restored to this radio by this
	// programme. LIFT: the TS-480 instruction manual, which is a fetch and
	// not a wire observation.
	registerQ2 = "Q2"
)

// RefusalError is a write refusal that names the ASSUMED-register entries it
// comes from.
//
// IT IS A *driver.WriteRefusedError AND MORE, never instead: Unwrap returns
// the embedded fleet error, so errors.Is(err, driver.ErrWriteRefused) holds
// through it, errors.As recovers the fleet type for a caller that wants the
// slot and the fields, and errors.As recovers THIS type for a caller — or a
// test — that needs to know WHICH rung answered.
//
// WHY THE DISTINCTION MATTERS ENOUGH TO MINT A TYPE (plan P7's H2, and on this
// row it matters MORE rather than less): the capability gate refuses every
// write on an unconsented RealHardware session while writeTrialsComplete is
// false, and returns the PLAIN fleet error. A test that asserted only
// "refused" would therefore pass on the capability gate — and since A22
// refuses everything too, "every TS-480 write is refused" would be trivially
// satisfied and A22 need not exist at all, which is the one thing this row's
// entire write path is for.
//
// ONE TYPE, ONE PRIMARY CAUSE, AND A CAUSE LIST — see WriteChannel for why Q2
// is a member of that list rather than a rung of its own.
type RefusalError struct {
	driver.WriteRefusedError
	// Register is the PRIMARY cause, and on this row it is ALWAYS A22.
	Register string
	// Causes is the full cause list in order: A22 alone, or A22 and Q2. It
	// is what a test distinguishes the two shapes by, and what a log reader
	// sees named in Reason.
	Causes []string
}

// Unwrap exposes the fleet refusal, which in turn unwraps to
// driver.ErrWriteRefused.
func (e *RefusalError) Unwrap() error { return &e.WriteRefusedError }

// requestedFieldRules pairs each spec.Field with the predicate that reports
// whether a write of this channel actually REQUESTS it.
//
// "REQUESTED" MEANS SOMETHING NARROWER HERE THAN IN core/driver/ts590: there
// "requested" tracks what the frame carries, because that pair's frame always
// carries it; here — since no frame is ever built — it tracks only what the
// CALLER asked for, which is why tone_tx/tone_rx/data_mode are conditional
// rather than unconditional (see below) and why L-HW-16 inherits an
// obligation from it.
//
// TWENTY-SIX ENTRIES — every spec.Field but spec.FieldErase, in the same order
// this package's own literal list carries them (caps_test.go's allSpecFields;
// plan P5 forbids a Kenwood file naming spec.AllFields). FieldErase is not a
// field a write requests: it is the whole shape of a DIFFERENT frame, and this
// radio has no erase route of any kind — the only "clear" in the book is RC,
// "Clears the RIT offset frequency" (480:1205). WriteChannel refuses an empty
// channel a rung above this table.
//
// FIVE ARE UNCONDITIONAL, AND THAT IS THE FRAME'S OWN SHAPE — where the 590
// pair have eight. The 50-byte record carries a frequency, a mode, a lockout,
// a tone mode and a name on EVERY write, changed or not, with no "leave it
// alone" encoding anywhere in the grid (480:951-984). A write therefore
// requests all five whether the caller edited them or not, which is what makes
// the capability gate below a real gate.
//
// THE THREE THE 590 PAIR CARRY UNCONDITIONALLY AND THIS ROW DOES NOT, because
// naming them here would put the capability gate in front of A22 for every
// ordinary channel and leave A22 unreachable on any profile:
//
//   - spec.FieldToneTx and spec.FieldToneRx. The record really does carry P8
//     and P9 on every write (480:966, 480:969), but this row publishes NO TONE
//     DOMAIN at all — the charts are not in this book — so every field-level
//     write of them is Unsupported. Naming them unconditionally would make
//     "every TS-480 write is refused" true for the WRONG REASON, and the
//     A22 pins would be satisfied by the gate. Naming them only when a value
//     is present leaves A22 the answer for an ordinary channel and still
//     refuses a caller who hands this driver a tone it cannot address.
//   - spec.FieldDataMode. Byte 19 is the channel LOCKOUT on this radio
//     (480:962), so there is no data-mode position to carry at all.
//
// THE REMAINING CONDITIONALS EXIST FOR ONE CASE: a caller who hands this
// driver a value the record has no room for — a Known IP+ from an Icom native
// file, a Yaesu shift from a CSV import, a filter cloned off a TS-590SG.
// Naming the field only when a value is actually present is what lets an
// ordinary write reach A22 while REFUSING that one at the gate, rather than
// dropping the value silently from a frame with nowhere to put it.
//
// THREE OF THE CONDITIONALS TEST A VALUE AND NOT A FieldState, because their
// codeplug.ChannelData members are plain scalars with no state to test:
// ClarHz/RxClar/TxClar (three Go members under one spec.Field), CTCSS and
// Shift. A caller who set one has requested it just as surely as a Known
// FieldState does, and the fleet's FieldState walk cannot see them at all.
var requestedFieldRules = []struct {
	field   spec.Field
	present func(codeplug.ChannelData) bool
}{
	{spec.FieldFrequency, always},
	{spec.FieldMode, always},
	{spec.FieldClarifier, func(d codeplug.ChannelData) bool { return d.ClarHz != 0 || d.RxClar || d.TxClar }},
	{spec.FieldCTCSSState, func(d codeplug.ChannelData) bool { return d.CTCSS != "" }},
	{spec.FieldCTCSSTone, func(d codeplug.ChannelData) bool { return d.CTCSSTone.State == codeplug.Known }},
	{spec.FieldShift, func(d codeplug.ChannelData) bool { return d.Shift != "" }},
	{spec.FieldTag, always},
	{spec.FieldTagDisplay, func(d codeplug.ChannelData) bool { return d.TagDisplay.State == codeplug.Known }},
	{spec.FieldScanSkip, always},
	// spec.FieldErase is DELIBERATELY ABSENT; see the doc comment.
	{spec.FieldTxFrequency, func(d codeplug.ChannelData) bool { return d.TxFreqHz.State == codeplug.Known }},
	{spec.FieldDuplex, func(d codeplug.ChannelData) bool { return d.Duplex.State == codeplug.Known }},
	{spec.FieldOffset, func(d codeplug.ChannelData) bool { return d.OffsetHz.State == codeplug.Known }},
	{spec.FieldToneMode, always},
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
	// The three TS-2000-only Satellite Memory bank fields (v1.10.0).
	// This row has no such record position; the predicates are
	// carried only because allSpecFields is this table's own
	// completeness contract.
	{spec.FieldSatBandSwap, func(d codeplug.ChannelData) bool { return d.SatBandSwap.State == codeplug.Known }},
	{spec.FieldSatTrace, func(d codeplug.ChannelData) bool { return d.SatTrace.State == codeplug.Known }},
	{spec.FieldSatTraceRev, func(d codeplug.ChannelData) bool { return d.SatTraceRev.State == codeplug.Known }},
}

// always is the predicate of a field the record carries on every write.
func always(codeplug.ChannelData) bool { return true }

// requestedFields lists, in requestedFieldRules' order, the spec.Fields this
// channel's write requests.
func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := make([]spec.Field, 0, len(requestedFieldRules))
	for _, r := range requestedFieldRules {
		if r.present(data) {
			fields = append(fields, r.field)
		}
	}
	return fields
}

// WriteChannel implements driver.Session, and on this row it NEVER WRITES.
// Every channel write is refused before a frame is built (A22, decision 12),
// and no MW of any width is constructed anywhere in this package.
//
// THE LADDER, IN P7's ORDER, EVERY RUNG PRE-WIRE:
//
//	parseSlotID          the identifier's SYNTAX, and no radio (read.go)
//	bankFor              membership in THIS session's published bank
//	the erase            an empty channel, refused — and on THIS row for a
//	                     reason of its own: the radio has no erase route at
//	                     all (480:1205), where the 590 pair's is an ambiguous
//	                     short MW (A5)
//	CheckFieldStates     the fleet's FieldState walk, P7's Valid() rung
//	THE CAPABILITY GATE  defence in depth below the clone service
//	A22                  ANY TS-480 channel write, with Q2 as a subsidiary
//	                     cause inside it
//
// AND THERE THE LADDER STOPS, DELIBERATELY. P7's remaining rungs are the 590
// pair's — A13/A14 is a TS-590S firmware gate on a radio with no CAT-readable
// firmware at all (E15), A23 is a P14 mode gate on a byte that is a step here,
// A9 gates a field this row does not publish (M-E2), and decision 14's 1750 Hz
// gates a tone domain this row does not have. NONE of them could execute below
// A22 in any case, because A22 rejects every write by definition.
//
// THE FRAME-BUILDING HALF IS NOT WRITTEN, AND THAT IS THE SAME DECISION SAID
// ONCE MORE. There is no buildMWSet here, no MW transport spec and no send:
// code that no profile can reach is code no test can falsify, and a dead
// builder would invite a later reader to enable it without the trial that
// makes it honest. THE COMMIT THAT LIFTS A22 (L-HW-16) IS WHERE THE BUILDER,
// THE STANDALONE Q2 RUNG WITH ITS OWN ORDERING AND ITS OWN POSITIVE CONTROL,
// AND THE MW CHOREOGRAPHY ALL LAND TOGETHER — and L-HW-16 is one trial per ST
// mode class, SSB/CW/FSK over 00 ~ 04 and then AM/FM over 00 ~ 09
// (480:1494-1500), because a two-legend field cannot be settled from one class
// and a partial result lifts nothing.
//
// THAT COMMIT ALSO INHERITS AN OBLIGATION FROM requestedFieldRules: the
// record carries P8 and P9 on every write (480:966, 480:969), but tone_tx and
// tone_rx are named here only when the caller asked for them, so an ordinary
// tone-OFF channel never names them at the gate. The builder needs either an
// explicit default for those two bytes on a tone-OFF write, or the table
// returned to unconditional naming of tone_tx/tone_rx with the standalone Q2
// rung ordered ahead of the gate rather than folded into it as a subsidiary
// cause.
//
// ONE TYPED REFUSAL, TWO CAUSES, AND Q2 IS NOT A RUNG WHILE A22 STANDS
// (Codex HIGH 2; spec §Error handling; plan P7). Revision 2 ordered A22 ahead
// of a separate Q2 rung and asked for two separately recoverable typed errors.
// A22 rejects EVERY TS-480 channel write by definition, so a Q2 rung placed
// after it can never execute on any profile and the two instructions could not
// both be obeyed. The corrected relationship, which this method implements:
//
//   - There is ONE typed TS-480 write refusal. errors.As recovers it once.
//   - Its PRIMARY cause is always A22.
//   - When the source channel's tone_mode is not OFF it ADDITIONALLY carries
//     Q2 in its subsidiary-cause list, and its message names both.
//   - A test distinguishes the two shapes through the error's FIELDS, not
//     through a second rung: a tone_mode == OFF channel yields a cause list of
//     A22 alone, and a tone_mode != OFF channel one of A22 and Q2. That is a
//     real, falsifiable difference — an implementation that forgot Q2 fails
//     the second assertion — and it does not require Q2 ever to be reached
//     first.
//
// Nothing about Q2's substance changes: tone_tx and tone_rx stay Unsupported,
// CTCSSTones stays nil, the interim refusal is still specified and still
// tested, and when A22 is lifted the Q2 cause is already written and already
// pinned. What changes is how it is reached.
//
// WHICH PROFILE EACH RUNG IS PINNED UNDER (plan P7's H2, and here it matters
// MORE rather than less). The capability-gate rung is pinned ONCE, on an
// unconsented RealHardware session, where it answers first for every write.
// A22 — in BOTH its cause shapes — is pinned on the SIMULATED profile, a
// session that has already passed that gate, asserting the TYPED error by
// errors.As and its register entry. Otherwise "every write is refused" is
// trivially satisfied by the capability gate and A22 need not exist at all.
// THE CONTROL THAT THE A22 PIN IS NOT VACUOUS is the same Simulated session's
// READ of the same slot, which succeeds and returns a populated channel; there
// is no positive WRITE control on this row and there cannot be one.
//
// THE OPERATION MUTEX IS HELD FOR THE WHOLE CALL (P13/P14, matrix M-E2), and
// is taken even before the refusal checks: the rule is ONE DRIVER OPERATION at
// a time, not "one operation at a time, except the ones that return early".
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	// Every refusal below returns res unchanged: an EXPLICITLY EMPTY step
	// list, never nil. The clone service journals this result, and a nil
	// slice marshals as JSON null, which an auditor would have to read as
	// "unknown" rather than the truth, "no frame was ever built". On this row
	// it is returned unchanged on EVERY path, because there is no path that
	// builds one.
	res := driver.WriteResult{Steps: []driver.WriteStep{}}

	if _, err := parseSlotID(ch.Slot); err != nil {
		return res, &UnknownSlotError{Slot: ch.Slot, Model: modelName, Reason: err.Error()}
	}
	bank, ok := s.bankFor(ch.Slot)
	if !ok {
		// The same branch, and the same message, that refuses a READ of an
		// unpublished slot (read.go).
		return res, &UnknownSlotError{
			Slot:   ch.Slot,
			Model:  modelName,
			Reason: fmt.Sprintf("this row publishes %s", s.bankNames()),
		}
	}

	if ch.Empty() {
		// AN EMPTY CHANNEL IS AN ERASE REQUEST, AND IT IS REFUSED — and the
		// reason is THIS RADIO'S OWN rather than the 590 pair's. Those rows
		// refuse because the only clear their book prints is a side effect of
		// a short MW whose length is a reading rather than a printed number
		// (590:1579-1581, A5, erratum E19). THIS BOOK PRINTS NO ERASE ROUTE
		// OF ANY KIND: the only "clear" anywhere in it is RC, "Clears the RIT
		// offset frequency" (480:1205), which is not about memories at all.
		// Quoting the 590's sentence here would describe a paragraph this
		// document does not contain.
		//
		// The rung must also stay AHEAD of the field checks below
		// STRUCTURALLY and not merely by preference: an empty channel has no
		// Data at all, and those checks dereference it.
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "this radio has no erase route at all: the only \"clear\" printed anywhere in its PC-command reference is RC, \"Clears the RIT offset frequency\" (480:1205), which is not about memory channels — so there is no frame to refuse to build, FieldErase is not write-Supported on this row's bank, and the standing no-erase rule needs no exception here",
		}
	}
	data := *ch.Data

	// P7's Valid() rung, and it is THE FLEET'S walk rather than a table of
	// this package's own (core/driver.CheckFieldStates): every field of
	// ChannelData that carries a FieldState, judged against this session's own
	// vocabularies. What it prevents is silent rather than loud — a value
	// carried alongside a state meaning "preserve whatever the radio has" is
	// never named by requestedFields, so without this rung it would be DROPPED
	// and the write would report success.
	if field, err := driver.CheckFieldStates(s.caps, data); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{field}, Reason: err.Error()}
	}

	// THE CAPABILITY GATE. Every requested field must pass
	// spec.FieldSupport.CanWrite for THIS slot's bank in THIS session's
	// capabilities — spec.Supported, or spec.ConsentedUnverified, which is the
	// label every writable field of a consented real-hardware session carries.
	// spec.Inert is accepted as acceptable-to-TRANSMIT, which is the fleet's
	// stated stance; no Kenwood field is Inert today, since Inert is a
	// HARDWARE finding and no Kenwood radio has been asked anything.
	//
	// A cloned TS-590SG channel carrying a Known filter or tone_tx/tone_rx is
	// refused ONE RUNG ABOVE, not here: CheckFieldStates rejects a Known
	// Filter because StringField.Valid fails closed against a nil
	// caps.Filters, and a Known tone_tx/tone_rx the same way because
	// ToneField.Valid's AdmitsTone fails closed against a nil CTCSSTones.
	// The general rule is that an empty-vocabulary conditional can never fire
	// through WriteChannel: CheckFieldStates always answers first. Of the
	// three fields a clone can carry, only a Known data_mode survives to
	// reach THIS gate, because BoolField.Valid accepts any Known value
	// regardless of vocabulary — it is refused HERE, with the field named,
	// rather than dropped from a frame with nowhere to put it.
	var unwritable []spec.Field
	for _, f := range requestedFields(data) {
		fs := s.caps.FieldSupport(bank.ID, f)
		if !fs.CanWrite() && fs.Write != spec.Inert {
			unwritable = append(unwritable, f)
		}
	}
	if len(unwritable) > 0 {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: unwritable,
			Reason: "not write-Supported for this session (the 50-byte record cannot express the field on this row, or this session's capability profile does not support writing it)",
		}
	}

	// A22 — ANY TS-480 CHANNEL WRITE, and it is the last rung because it is
	// the only one below the gate that can ever execute here.
	return res, s.refuseUnderA22(ch.Slot, data)
}

// refuseUnderA22 builds the ONE typed refusal every TS-480 channel write
// earns: primary cause A22 always, plus Q2 when the source channel asks for a
// tone this row cannot address.
//
// THE Q2 TEST IS ON THE SOURCE CHANNEL'S tone_mode AND NOT ON ITS TONE VALUES,
// which is the spec's own wording — "a write whose tone_mode is not OFF on a
// TS-480" — and it is the right test: the tone VALUES arrive Unavailable off
// this driver's own read path (there is no chart to give them hertz), so a
// value-based test would never fire on a channel this programme itself
// produced, while the MODE round-trips and is exactly what a restore would be
// trying to put back.
//
// THE VALUE COMPARED AGAINST IS THE CAPABILITY TABLE'S OWN "OFF", read from
// the same place the read path maps P7 through, so the two cannot drift.
//
// Fields NAMES ONLY THE PRIMARY CAUSE'S FIELD, spec.FieldTuningStep, EVEN ON
// THE TWO-CAUSE SHAPE: when Q2 also fires, tone_tx/tone_rx are not added to
// it. Nothing is hidden from a caller that looks — Causes carries Q2 and
// Reason names the tone fields at length — but a caller reading Fields alone
// sees only tuning_step.
func (s *Session) refuseUnderA22(slotID string, data codeplug.ChannelData) *RefusalError {
	causes := []string{registerA22}
	reason := "A22: no TS-480 P14 value is known to be a \"no change\" value, so this programme cannot write bytes 39-40 at all and EVERY TS-480 channel write is refused (decision 12). Those two bytes are the tuning step on this radio — \"Step size. Refer to the ST command.\" (480:979) — and ST's legend is mode-conditional over two different ranges, 00 ~ 04 for SSB/CW/FSK and 00 ~ 09 for AM/FM, with index 00 meaning 0.5 kHz in the first and 5 kHz in the second (480:1494-1500). No flat capability vocabulary is truthful over two legends, so tuning_step is published Unsupported, the source channel never retains the raw index, and there is nothing to put in those bytes but a guess. The lift is L-HW-16, one trial per ST mode class"

	if data.ToneMode.State == codeplug.Known && data.ToneMode.Value != toneModeNames[kw.ToneModeOff] {
		causes = append(causes, registerQ2)
		reason += ". AND Q2, SUBSIDIARILY: this channel's tone_mode is " +
			fmt.Sprintf("%q", data.ToneMode.Value) +
			" rather than \"OFF\", and this row publishes no tone domain at all — the TN and CN charts are not in this document (\"Refer to page 32 of the TS-480 instruction manual\", 480:1559-1560; page 33 for CN, 480:339-340) — so tone_tx and tone_rx are Unsupported and a tone-bearing channel could not be restored to this radio even if A22 were lifted. Q2 is recorded as a SUBSIDIARY cause and not as a rung of its own, because A22 refuses every write by definition and a rung below it could never execute; the standalone Q2 rung, with its own ordering and its own positive control, is written in the commit that lifts A22 (L-HW-16) and not before"
	}

	return &RefusalError{
		WriteRefusedError: driver.WriteRefusedError{
			Slot:   slotID,
			Fields: []spec.Field{spec.FieldTuningStep},
			Reason: reason,
		},
		Register: registerA22,
		Causes:   causes,
	}
}
