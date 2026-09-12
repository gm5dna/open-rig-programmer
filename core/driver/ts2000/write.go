// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000

import (
	"context"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// THE THREE RAW BYTES THIS ROW HAS NO HONEST VALUE FOR, on the
// core/driver/ts480 A22 precedent exactly: a field the wire record carries
// on EVERY write, whose vocabulary this wave does not establish, so there
// is nothing to put in the byte but a guess.
//
//   - Byte 28 (P11), REVERSE status. Live on this row (kw.Byte28Reverse)
//     and no spec.Field names repeater-reverse at all (matrix §2:
//     "UNMAPPED-with-reason") — there is no source-channel value to carry
//     even in principle.
//   - Bytes 39-40 (P14), the tuning-step index. This row's own ST command
//     is mode-conditional over two different ranges exactly like the
//     TS-480's — "SSB/CW/FSK mode: 00~03" against "AM/FM mode: 00~09"
//     (ts2000:11508-11521) — and EX/menu and tuning-step vocabulary are
//     both excluded this wave (spec §6 Q4; matrix §2's P14 row). No flat
//     Capabilities.TuningSteps vocabulary would be truthful, so the
//     source channel never retains the raw index and there is nothing
//     honest to write.
//   - Byte 41 (P15), Memory Group. Live on this row (kw.Byte41MemoryGroup)
//     and no spec.Field names channel-group membership either (matrix §2:
//     "UNMAPPED-with-reason", the TS-480 exemplar's precedent for this
//     exact gap).
//
// registerP14 IS THIS PACKAGE'S OWN NAME FOR THE CAUSE, NOT A REUSE OF THE
// TS-480'S "A22": the two rows share the SAME step-index ambiguity (both
// reuse kw.Byte3940StepIndex) but are otherwise different documents, and
// citing another row's register entry by name here would misattribute it
// the same way core/kw/ts2000/layout.go's own doc comment flags for the
// Book choice. This package's brief mints no ASSUMED-register entry of its
// own (that is Phase 4's job); the citation below is this file's own
// evidence, not a borrowed one.
const registerP14 = "ts2000-capability-matrix.md §2 (P11, P14, P15)"

// RefusalError is a write refusal naming the cause above. See
// core/driver/ts480's RefusalError for why a typed distinction from the
// bare capability-gate refusal matters: writeTrialsComplete is false, so an
// unconsented RealHardware session is refused by the gate FIRST, and only a
// consented (or Simulated) session ever reaches this rung — a test
// asserting "refused" alone could not tell the two apart, and this row's
// entire write path exists to prove the SECOND one.
type RefusalError struct {
	driver.WriteRefusedError
	Register string
}

// Unwrap exposes the fleet refusal, which in turn unwraps to
// driver.ErrWriteRefused.
func (e *RefusalError) Unwrap() error { return &e.WriteRefusedError }

// requestedFieldRules pairs each spec.Field with the predicate that reports
// whether a write of this channel actually requests it — the
// core/driver/ts480 shape, widened for this row's own nine unconditional
// fields (matrix §2's five live axes add FieldDuplex/FieldOffset to the
// family's frequency/mode/tag/scan_skip/tone_mode, and this document's own
// tone chart adds tone_tx/tone_rx).
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
	// spec.FieldErase is deliberately absent; WriteChannel refuses an empty
	// channel a rung above this table.
	{spec.FieldTxFrequency, func(d codeplug.ChannelData) bool { return d.TxFreqHz.State == codeplug.Known }},
	{spec.FieldDuplex, always},
	{spec.FieldOffset, always},
	{spec.FieldToneMode, always},
	{spec.FieldToneTx, always},
	{spec.FieldToneRx, always},
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

// always is the predicate of a field the record carries on every write.
func always(codeplug.ChannelData) bool { return true }

// requestedFields lists, in requestedFieldRules' order, the spec.Fields
// this channel's write requests.
func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := make([]spec.Field, 0, len(requestedFieldRules))
	for _, r := range requestedFieldRules {
		if r.present(data) {
			fields = append(fields, r.field)
		}
	}
	return fields
}

// WriteChannel implements driver.Session, and on this row it NEVER WRITES:
// every channel write is refused before a frame is built, for the three raw
// bytes named at the top of this file. No buildMWSet call, no MW transport
// spec and no send exist anywhere in this package — code no profile can
// reach is code no test can falsify, and the commit that lifts this refusal
// is where they land together with the vocabulary that makes them honest.
//
// THE LADDER, EVERY RUNG PRE-WIRE:
//
//	parseSlotID          the identifier's syntax, and no radio (read.go)
//	bankFor               membership in THIS session's published bank
//	the erase             an empty channel, refused (this document prints
//	                      no MR/MW-reachable erase primitive either)
//	CheckFieldStates      the fleet's FieldState walk
//	THE CAPABILITY GATE   defence in depth below the clone service
//	registerP14           any channel write, unconditionally
//
// THE OPERATION MUTEX IS HELD FOR THE WHOLE CALL, taken before the refusal
// checks (core/driver/ts480's own P13/P14 reasoning: one driver operation
// at a time, not one operation at a time except the ones that return
// early).
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	res := driver.WriteResult{Steps: []driver.WriteStep{}}

	number, half, err := parseSlotID(ch.Slot)
	if err != nil {
		return res, &UnknownSlotError{Slot: ch.Slot, Model: s.p.name, Reason: err.Error()}
	}
	bank, ok := s.bankFor(ch.Slot)
	if !ok {
		return res, &UnknownSlotError{
			Slot: ch.Slot, Model: s.p.name,
			Reason: fmt.Sprintf("this row publishes %s", s.bankNames()),
		}
	}
	if _, err := s.layout.NewSlot(number, half); err != nil {
		return res, &UnknownSlotError{Slot: ch.Slot, Model: s.p.name, Reason: err.Error()}
	}

	if ch.Empty() {
		// AN EMPTY CHANNEL IS AN ERASE REQUEST, AND IT IS REFUSED: this
		// document prints no MR/MW-reachable erase primitive (matrix §4,
		// FieldErase), which is the fleet-wide write posture (spec §3) —
		// there is no frame to refuse to build.
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "this radio's PC-command reference documents no MR/MW-reachable erase route (matrix §4) — FieldErase is not write-Supported on this row's banks, and the standing no-erase rule needs no exception here",
		}
	}
	data := *ch.Data

	if field, err := driver.CheckFieldStates(s.caps, data); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{field}, Reason: err.Error()}
	}

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
			Reason: "not write-Supported for this session",
		}
	}

	return res, &RefusalError{
		WriteRefusedError: driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldTuningStep},
			Reason: "this row's 50-byte record carries three raw bytes on every write with no honest value this milestone can supply: byte 28 (P11, REVERSE — no spec.Field names it), bytes 39-40 (P14, a tuning-step index this wave's spec §6 Q4 excludes from vocabulary) and byte 41 (P15, Memory Group — likewise no spec.Field). Writing any of them would put a guess on the wire where the source channel retains no value at all, so every channel write on this row is refused (" + registerP14 + ")",
		},
		Register: registerP14,
	}
}
