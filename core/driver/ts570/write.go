// SPDX-License-Identifier: GPL-3.0-or-later

package ts570

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// mwSetSpec is the transport spec for the ONE MW Set this driver sends:
// fire-and-forget, core/driver/ts590's own mwSetSpec.
func mwSetSpec() transport.CommandSpec {
	return transport.CommandSpec{Class: transport.ClassWrite}
}

// modeWire is the layout's ModeNames read backwards: the neutral mode
// name this row publishes back to core/kw.Mode. Derived at call time from
// the layout rather than transcribed a second time, so this package's own
// wire byte and its published Modes vocabulary cannot drift apart.
func modeWire(l kw.Layout, name string) (kw.Mode, bool) {
	for m, n := range l.ModeNames() {
		if n == name {
			return m, true
		}
	}
	return 0, false
}

// requestedFieldRules pairs each spec.Field with the predicate that
// reports whether a write of this channel actually requests it.
//
// SIX ARE UNCONDITIONAL: frequency, mode, scan_skip, tone_mode, tone_tx
// and tone_rx are the whole of what this row's 28-byte record carries
// (matrix §1.2), transmitted on every write with no "leave it alone"
// encoding anywhere in the grid — the same reasoning core/driver/ts590's
// own table states for its own eight. spec.FieldErase is deliberately
// absent, on that package's own precedent: it is not a field a write
// requests, and WriteChannel refuses an empty channel a rung above this
// table.
//
// EVERYTHING ELSE IS CONDITIONAL, and every one of them is Unsupported on
// this row (caps.go): the predicates exist so that a caller handing this
// driver a value this record has no room for is REFUSED rather than
// silently dropped.
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
	{spec.FieldTag, func(d codeplug.ChannelData) bool { return d.Tag != "" }},
	{spec.FieldTagDisplay, func(d codeplug.ChannelData) bool { return d.TagDisplay.State == codeplug.Known }},
	{spec.FieldScanSkip, always},
	{spec.FieldTxFrequency, func(d codeplug.ChannelData) bool { return d.TxFreqHz.State == codeplug.Known }},
	{spec.FieldDuplex, func(d codeplug.ChannelData) bool { return d.Duplex.State == codeplug.Known }},
	{spec.FieldOffset, func(d codeplug.ChannelData) bool { return d.OffsetHz.State == codeplug.Known }},
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

func always(codeplug.ChannelData) bool { return true }

func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := make([]spec.Field, 0, len(requestedFieldRules))
	for _, r := range requestedFieldRules {
		if r.present(data) {
			fields = append(fields, r.field)
		}
	}
	return fields
}

// mandatoryFieldRefusal reports that a field this row's record carries on
// every write was not Known — core/driver/ts590's own shape.
func mandatoryFieldRefusal(slotID string, field spec.Field, state codeplug.FieldState, position string) *driver.WriteRefusedError {
	return &driver.WriteRefusedError{
		Slot:   slotID,
		Fields: []spec.Field{field},
		Reason: fmt.Sprintf("%s FieldState is %q, not %q, and %s is transmitted on every write with no \"leave it alone\" encoding — so a non-Known value here could only be manufactured, which is what a state meaning \"preserve whatever the radio has\" forbids", field, state, codeplug.Known, position),
	}
}

func boolWire(on bool) byte {
	if on {
		return '1'
	}
	return '0'
}

// WriteChannel implements driver.Session: ONE 28-byte MW Set, reported
// Sent and never Confirmed — this family's fire-and-forget write, on
// core/driver/ts590's own reasoning (A6: that an accepted Set draws
// nothing at all is an assumption, not an observation, since no Kenwood
// radio has ever been written to by this project).
//
// THE LADDER: parse the slot's syntax, check bank membership, refuse an
// empty channel (erase — see caps.go and doc.go for why this row's
// stronger documented vacate route still cannot be built), the fleet's
// FieldState walk, the capability gate, then the ONE rung this row adds
// beyond every sibling package's own ladder — tone_tx and tone_rx must
// name the SAME tone, because P8 is one byte serving both directions
// (matrix §2) — and finally the build itself, which is core/kw's own
// refusals.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	res := driver.WriteResult{Steps: []driver.WriteStep{}}

	number, err := parseSlotID(ch.Slot)
	if err != nil {
		return res, &UnknownSlotError{driver.UnknownSlotError{Slot: ch.Slot, Model: s.model.name, Reason: err.Error()}}
	}
	bank, ok := s.bankFor(ch.Slot)
	if !ok {
		return res, &UnknownSlotError{driver.UnknownSlotError{
			Slot: ch.Slot, Model: s.model.name,
			Reason: fmt.Sprintf("this row publishes %s", s.bankNames()),
		}}
	}
	slot, err := s.layout.NewSlot(number, kw.ScanHalfNone)
	if err != nil {
		return res, &UnknownSlotError{driver.UnknownSlotError{Slot: ch.Slot, Model: s.model.name, Reason: err.Error()}}
	}

	if ch.Empty() {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "this milestone builds no erase: MW's own note documents a genuine vacate-on-all-zero-frequency route (matrix §3), but core/kw.BuildMWSet refuses any record whose FreqHz is zero unconditionally (decision 8), and FieldErase is not write-Supported on any bank of any row",
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
			Reason: "not write-Supported for this session (this row's 28-byte record cannot express the field, or this session's capability profile does not support writing it)",
		}
	}

	for _, m := range []struct {
		field    spec.Field
		state    codeplug.FieldState
		position string
	}{
		{spec.FieldScanSkip, data.ScanSkip.State, "P6 at position 19, the channel lockout (matrix §1.2)"},
		{spec.FieldToneMode, data.ToneMode.State, "P7 at position 20, the tone mode (matrix §1.2)"},
		{spec.FieldToneTx, data.ToneTx.State, "P8 at positions 21-22, this row's one tone index (matrix §2)"},
		{spec.FieldToneRx, data.ToneRx.State, "P8 at positions 21-22, this row's one tone index (matrix §2)"},
	} {
		if m.state != codeplug.Known {
			return res, mandatoryFieldRefusal(ch.Slot, m.field, m.state, m.position)
		}
	}

	toneMode, ok := toneModeWire[data.ToneMode.Value]
	if !ok {
		// Unreachable after CheckFieldStates, which judges a Known tone
		// mode against this session's ToneModes vocabulary.
		return res, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldToneMode},
			Reason: fmt.Sprintf("tone mode %q is not one this row publishes", data.ToneMode.Value),
		}
	}
	toneTxIdx, ok := toneWireIndex(data.ToneTx.Value)
	if !ok {
		return res, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldToneTx},
			Reason: fmt.Sprintf("tone_tx %v is not in the 39-entry chart this row publishes", data.ToneTx.Value),
		}
	}
	toneRxIdx, ok := toneWireIndex(data.ToneRx.Value)
	if !ok {
		return res, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldToneRx},
			Reason: fmt.Sprintf("tone_rx %v is not in the 39-entry chart this row publishes", data.ToneRx.Value),
		}
	}
	if toneTxIdx != toneRxIdx {
		// THE ONE RUNG THIS ROW ADDS BEYOND EVERY SIBLING'S LADDER (matrix
		// §2): P8 is a SINGLE byte superimposing the tone on transmit and
		// gating receive squelch with the same index, so a channel asking
		// for two different tones is a request this 28-byte record cannot
		// carry at all, not a value either field individually refuses.
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldToneTx, spec.FieldToneRx},
			Reason: fmt.Sprintf("tone_tx %v and tone_rx %v disagree, but this row has ONE tone byte (P8) serving both directions (matrix §2) — a single index cannot carry two different tones", data.ToneTx.Value, data.ToneRx.Value),
		}
	}

	mode, ok := modeWire(s.layout, data.Mode)
	if !ok {
		// Unreachable after CheckFieldStates, which judges a Known mode
		// against this session's Modes vocabulary — the same eight names
		// this layout's own legend carries.
		return res, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldMode},
			Reason: fmt.Sprintf("mode %q is not one this row publishes", data.Mode),
		}
	}

	cmd, err := s.layout.BuildMWSet(kw.Record{
		Slot:      slot,
		FreqHz:    data.FreqHz,
		Mode:      mode,
		Byte19:    boolWire(data.ScanSkip.Value),
		ToneMode:  toneMode,
		ToneIndex: toneTxIdx,
		// AnswerP1 is a PARSER output, left zero: the builder derives P1
		// from the slot's class (M9), and this row declares one flat
		// SlotMemory range so P1 is always '0'.
	})
	if err != nil {
		return res, err
	}

	// THE step list, declared in full HERE: after the frame provably
	// exists, before it goes near the wire. ONE element — this row's write
	// choreography IS one frame.
	res.Steps = []driver.WriteStep{{Command: "MW"}}
	const mwStep = 0

	// core/kw's Lift K follow-up (commit e7515d0) fills this row's P9
	// "NOT USED" span (positions 23-27) with the family's own '0' filler
	// on the hasTail=false path, so BuildMWSet's output now passes its
	// own AllowedCommand — see doc.go's former "the write path cannot
	// pass its own gate today" section, now closed, for the history.
	if _, err := s.eng.Do(ctx, cmd, mwSetSpec()); err != nil {
		// A "?;" IS ATTRIBUTABLE (the frame provably went out and the
		// radio provably refused it); any other transport failure is not,
		// so Sent is false there. Confirmed stays false on EVERY path,
		// including success: that a silent accepted Set means acceptance
		// is an assumption (A6), not an observation, since no Kenwood
		// radio has ever been written to by this project.
		res.Steps[mwStep].Sent = errors.Is(err, transport.ErrRejected)
		return res, fmt.Errorf("ts570: WriteChannel %s: %w", ch.Slot, wireFailure(s.layout, "MW", err))
	}
	res.Steps[mwStep].Sent = true
	return res, nil
}
