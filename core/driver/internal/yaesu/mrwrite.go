// SPDX-License-Identifier: GPL-3.0-or-later

package yaesu

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// mrRequestedFields lists the spec.Fields an MW write of data asks for:
// the five the bare frame always carries, plus FieldCTCSSTone — under
// ToneOptionalIfKnown only when the channel's tone is Known, under
// ToneRequiredKnown unconditionally (P9 is a live byte on every frame
// this radio has no "leave it alone" encoding for — ftdx9000's own doc
// comment before this migration), never under ToneNeverRequested — plus
// the Icom tier's seventeen (shared, TierRequestedFields), each only
// when Known, UNLESS SkipTierFields is set: ftdx9000's fixed 6-field
// request list never asks after any of them at all (spec-v2 finding 9),
// so a Known tier field is accepted and silently dropped rather than
// refused — see core/driver/ftdx9000/write_test.go's
// TestWriteChannel_IcomTierFieldKnown, the pin this branch must not
// break.
//
// THERE IS NO FieldTag/FieldTagDisplay HERE: no driver migrated onto this
// body so far has a tag/name route over CAT at all (ftdx5000's
// RequestConditionalTagFields wires that in when it migrates).
func mrRequestedFields(p *MRParams, data codeplug.ChannelData) []spec.Field {
	fields := []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldClarifier,
		spec.FieldCTCSSState,
		spec.FieldShift,
	}
	switch {
	case p.ToneWrite == ToneRequiredKnown:
		fields = append(fields, spec.FieldCTCSSTone)
	case p.ToneWrite == ToneOptionalIfKnown && data.CTCSSTone.State == codeplug.Known:
		fields = append(fields, spec.FieldCTCSSTone)
	}
	if p.SkipTierFields {
		return fields
	}
	for _, t := range TierRequestedFields {
		if t.Present(data) {
			fields = append(fields, t.Field)
		}
	}
	return fields
}

// MRWriteChannel writes one memory channel as a single MW Set frame,
// fire-and-forget with the transport's bounded "?;" listen — the
// single-frame MW/MR write path shared by the in-scope drivers. Named
// MRWriteChannel, not WriteChannel: that name already belongs to this
// package's 4-rig MT family's combined-record write.
//
// EVERY REFUSAL HAPPENS BEFORE ANY BYTE GOES OUT, in the fixed order the
// per-driver bodies it replaces already used: slot parse, bank lookup, an
// empty channel as an unexpressable erase, field-state coherence, then
// the capability walk over mrRequestedFields. Only then is the frame
// built, and building it can refuse again at the value level.
//
// The caller holds its own operation mutex around this call; nothing
// here takes a lock (matching ReadChannel and the MT family's
// WriteChannel).
func MRWriteChannel(ctx context.Context, eng *transport.Engine, dialect cat.Dialect, caps spec.Capabilities, p *MRParams, ch codeplug.Channel) (driver.WriteResult, error) {
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
	for _, f := range mrRequestedFields(p, *ch.Data) {
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

	cmd, err := BuildMWCommand(dialect, caps, p, ch)
	if err != nil {
		return res, err
	}

	res.Steps = []driver.WriteStep{{Command: "MW"}}
	const mwStep = 0

	if _, err := eng.Do(ctx, cmd, transport.CATWriteSpec()); err != nil {
		if errors.Is(err, cat.ErrRejected) {
			res.Steps[mwStep].Sent = true
			return res, fmt.Errorf("%s: WriteChannel %s: MW rejected by radio: %w", p.Name, ch.Slot, err)
		}
		return res, fmt.Errorf("%s: WriteChannel %s: MW: %w", p.Name, ch.Slot, err)
	}
	res.Steps[mwStep].Sent, res.Steps[mwStep].Confirmed = true, true

	return res, nil
}

// BuildMWCommand renders ch as this radio's MW Set frame, or refuses
// (typed, via *driver.WriteRefusedError) any value the codec cannot
// express. A pure function — no session, no wire I/O.
func BuildMWCommand(dialect cat.Dialect, caps spec.Capabilities, p *MRParams, ch codeplug.Channel) (cat.Command, error) {
	sl, err := dialect.ParseSlot(ch.Slot)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: err.Error()}
	}
	data := *ch.Data

	mode, ok := dialect.ModeByName(data.Mode)
	if !ok {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldMode},
			Reason: fmt.Sprintf("mode %q is not a mode this radio supports", data.Mode),
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

	// The P9 tone index. Under ToneRequiredKnown (ftdx9000, ftdx5000) P9
	// is a live byte on every frame with no "leave it alone" encoding, so
	// a non-Known tone is refused outright, before the lookup, rather
	// than merely left off the request as ToneOptionalIfKnown's channels
	// are. Under ToneOptionalIfKnown a Known tone reaching here has
	// already passed the write-Supported capability gate (mrRequestedFields,
	// above). Either way it must also already be one of this session's
	// own CTCSSTones (driver.CheckFieldStates, upstream in
	// MRWriteChannel), so the lookup below cannot fail on a well-formed
	// write — but it is not re-derived here on faith.
	if p.ToneWrite == ToneRequiredKnown && data.CTCSSTone.State != codeplug.Known {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldCTCSSTone},
			Reason: fmt.Sprintf("ctcss tone FieldState is %q, not %q; P9 is a live tone-table index on every MW frame with no \"leave it alone\" encoding, so only a Known value is ever sent", data.CTCSSTone.State, codeplug.Known),
		}
	}
	var toneIndex uint8
	toneRequested := p.ToneWrite == ToneRequiredKnown || (p.ToneWrite == ToneOptionalIfKnown && data.CTCSSTone.State == codeplug.Known)
	if toneRequested {
		idx := slices.Index(caps.CTCSSTones, data.CTCSSTone.Value)
		if idx < 0 {
			return cat.Command{}, &driver.WriteRefusedError{
				Slot: ch.Slot, Fields: []spec.Field{spec.FieldCTCSSTone},
				Reason: fmt.Sprintf("tone %v is not one of this radio's 50-entry CTCSS tone chart", data.CTCSSTone.Value),
			}
		}
		toneIndex = uint8(idx)
	}

	if data.FreqHz < caps.MinFreqHz || data.FreqHz > caps.MaxFreqHz {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldFrequency},
			Reason: fmt.Sprintf("frequency %d Hz is outside this radio's storable range %d-%d Hz", data.FreqHz, caps.MinFreqHz, caps.MaxFreqHz),
		}
	}

	freqHz, err := cat.MemoryFreqHz(data.FreqHz)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldFrequency}, Reason: err.Error()}
	}

	cmd, err := dialect.BuildMWSet(cat.MemoryData{
		Slot:      sl,
		FreqHz:    freqHz,
		ClarHz:    int16(data.ClarHz),
		RxClar:    data.RxClar,
		TxClar:    data.TxClar,
		Mode:      mode,
		Kind:      p.WriteKind(dialect),
		CTCSS:     ctcss,
		ToneIndex: toneIndex,
		Shift:     shift,
	})
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: fmt.Sprintf("cannot encode MW frame: %v", err)}
	}
	return cmd, nil
}
