// SPDX-License-Identifier: GPL-3.0-or-later

package yaesu

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// ShiftByName is the repeater-shift vocabulary. THE SAME THREE NAMES ON
// EVERY ONE OF THESE RADIOS — the shift field is three-valued in each
// manual's memory record — so it is one table rather than a Params field.
var ShiftByName = map[string]cat.Shift{
	"SIMPLEX": cat.ShiftSimplex,
	"PLUS":    cat.ShiftPlus,
	"MINUS":   cat.ShiftMinus,
}

// CTCSSMap turns a Params.CTCSS vocabulary into the lookup the write path
// uses. The slice keeps the manual's own legend ORDER, which CTCSSLegend
// renders into the refusal text; the map is only for the lookup.
func CTCSSMap(vocab []CTCSSName) map[string]cat.CTCSSState {
	m := make(map[string]cat.CTCSSState, len(vocab))
	for _, c := range vocab {
		m[c.Name] = c.State
	}
	return m
}

// CTCSSLegend renders a vocabulary as the slash-separated legend the
// refusal text names, e.g. "OFF/ENC-DEC/ENC". Derived from the same slice
// the lookup is built from, so a radio cannot acquire a state its own
// refusal fails to mention.
func CTCSSLegend(vocab []CTCSSName) string {
	names := make([]string, len(vocab))
	for i, c := range vocab {
		names[i] = c.Name
	}
	return strings.Join(names, "/")
}

// CTCSSState looks a state name up in vocab. A LINEAR SCAN over three to
// five entries, not a map: the vocabulary is this short on every one of
// these radios, and a scan costs no allocation on a path that runs per
// write.
func CTCSSState(vocab []CTCSSName, name string) (cat.CTCSSState, bool) {
	for _, c := range vocab {
		if c.Name == name {
			return c.State, true
		}
	}
	return 0, false
}

// MTSetSpec is the transport spec for an MT Set: a write, so no answer is
// waited for beyond the transport's own acceptance rule.
func MTSetSpec() transport.CommandSpec {
	return transport.CATWriteSpec()
}

// TierRequestedField pairs a tier-added spec.Field with the predicate that
// says whether a channel carries a Known value for it.
type TierRequestedField struct {
	Field   spec.Field
	Present func(codeplug.ChannelData) bool
}

// TierRequestedFields is the seventeen tier-added fields, IN
// spec.AllFields ORDER — a write requests one only when the channel
// actually carries a Known value for it, so a codeplug that never
// mentions a field cannot be refused for it.
var TierRequestedFields = []TierRequestedField{
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

// RequestedFields is the set of spec.Fields a write of data ASKS FOR —
// what the capability gate below is run against. The six the combined MT
// record always carries are unconditional; the rest are requested only
// when the channel carries a Known value for them.
func RequestedFields(data codeplug.ChannelData) []spec.Field {
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
	for _, t := range TierRequestedFields {
		if t.Present(data) {
			fields = append(fields, t.Field)
		}
	}
	return fields
}

// WriteChannel writes one memory channel as a SINGLE combined MT Set
// frame — the one-frame Yaesu write path, shared by every one of these
// radios but the FT-710, whose write is two frames and stays its own.
//
// EVERY REFUSAL HAPPENS BEFORE ANY BYTE GOES OUT, in a fixed order: the
// slot must parse, it must belong to a bank this session supports, an
// empty channel is an erase this codec cannot express, the field states
// must be writable at all, and every field the write ASKS FOR must be
// write-Supported for this session's profile. Only then is the frame
// built, and building it can refuse again on the value level.
//
// The caller holds its own operation mutex around this call; nothing here
// takes a lock.
func WriteChannel(ctx context.Context, eng *transport.Engine, dialect cat.Dialect, caps spec.Capabilities, p *Params, ch codeplug.Channel) (driver.WriteResult, error) {
	res := driver.WriteResult{Steps: []driver.WriteStep{}}

	if _, err := dialect.ParseSlot(ch.Slot); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: fmt.Sprintf("not a valid slot: %v", err)}
	}
	bank, ok := caps.BankOf(ch.Slot)
	if !ok {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: "slot is not part of any bank this session supports"}
	}

	if ch.Empty() {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: p.EraseReason,
		}
	}

	if field, err := driver.CheckFieldStates(caps, *ch.Data); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{field}, Reason: err.Error()}
	}

	var unwritable []spec.Field
	for _, f := range RequestedFields(*ch.Data) {
		fs := caps.FieldSupport(bank, f)
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

	cmd, err := BuildWriteCommand(dialect, caps, p, ch)
	if err != nil {
		return res, err
	}

	res.Steps = []driver.WriteStep{{Command: "MT"}}
	const mtStep = 0

	if _, err := eng.Do(ctx, cmd, MTSetSpec()); err != nil {
		if errors.Is(err, cat.ErrRejected) {
			res.Steps[mtStep].Sent = true
			return res, fmt.Errorf("%s: WriteChannel %s: MT rejected by radio: %w", p.Name, ch.Slot, err)
		}
		return res, fmt.Errorf("%s: WriteChannel %s: MT: %w", p.Name, ch.Slot, err)
	}
	res.Steps[mtStep].Sent, res.Steps[mtStep].Confirmed = true, true

	return res, nil
}

// BuildWriteCommand renders ch as this radio's combined MT Set frame, or
// refuses with a *driver.WriteRefusedError naming the field at fault.
//
// A PURE function — no session, no wire I/O — so every refusal below can
// be exercised directly from a table test.
//
// THE ORDER OF THE CHECKS IS THE ORDER THE REFUSALS FIRE IN, and each
// radio's optional ones sit where that radio's own code had them: the
// FT-891's two record-shape refusals before the value checks (a live tag
// flag and a TX clarifier its record cannot express at all, so neither is
// a matter of the value being out of range), the FT-991A's ModeUnset
// refusal directly after the mode lookup that could produce it, and its
// frequency-range refusal after the clarifier check and before the
// codec's own frequency encoding.
func BuildWriteCommand(dialect cat.Dialect, caps spec.Capabilities, p *Params, ch codeplug.Channel) (cat.Command, error) {
	sl, err := dialect.ParseSlot(ch.Slot)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: err.Error()}
	}
	data := *ch.Data

	if p.RefuseTagDisplayUnknown != "" && data.TagDisplay.State != codeplug.Known {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldTagDisplay},
			Reason: fmt.Sprintf(p.RefuseTagDisplayUnknown, data.TagDisplay.State, codeplug.Known),
		}
	}
	if p.RefuseTxClar != "" && data.TxClar {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldClarifier},
			Reason: p.RefuseTxClar,
		}
	}

	mode, ok := dialect.ModeByName(data.Mode)
	if !ok {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldMode},
			Reason: fmt.Sprintf("mode %q is not a mode this radio supports", data.Mode),
		}
	}
	if p.RefuseModeUnset != "" && mode == cat.ModeUnset {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldMode},
			Reason: fmt.Sprintf(p.RefuseModeUnset, data.Mode),
		}
	}

	ctcss, ok := CTCSSState(p.CTCSS, data.CTCSS)
	if !ok {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldCTCSSState},
			Reason: fmt.Sprintf("ctcss state %q is not one of %s", data.CTCSS, CTCSSLegend(p.CTCSS)),
		}
	}
	shift, ok := ShiftByName[data.Shift]
	if !ok {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldShift},
			Reason: fmt.Sprintf("shift %q is not one of SIMPLEX/PLUS/MINUS", data.Shift),
		}
	}

	clar := dialect.Clarifier()
	if data.ClarHz < -clar.MaxAbsHz || data.ClarHz > clar.MaxAbsHz {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldClarifier},
			Reason: fmt.Sprintf("clarifier %d Hz exceeds +/-%d Hz", data.ClarHz, clar.MaxAbsHz),
		}
	}

	if p.CheckFreqRange && (data.FreqHz < caps.MinFreqHz || data.FreqHz > caps.MaxFreqHz) {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldFrequency},
			Reason: fmt.Sprintf("frequency %d Hz is outside this radio's storable range %d-%d Hz", data.FreqHz, caps.MinFreqHz, caps.MaxFreqHz),
		}
	}

	freqHz, err := cat.MemoryFreqHz(data.FreqHz)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldFrequency}, Reason: err.Error()}
	}

	cmd, err := p.BuildMT(dialect, cat.MemoryData{
		Slot:   sl,
		FreqHz: freqHz,
		ClarHz: int16(data.ClarHz),
		RxClar: data.RxClar,
		TxClar: data.TxClar,
		Mode:   mode,
		Kind:   cat.CombinedMTSetKind,
		CTCSS:  ctcss,
		Shift:  shift,
	}, data.Tag, data.TagDisplay.Value)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Reason: fmt.Sprintf("cannot encode the combined MT Set frame: %v", err),
		}
	}
	return cmd, nil
}
