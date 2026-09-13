// SPDX-License-Identifier: GPL-3.0-or-later

package ts870s

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

// registerTxFrequency is the reason FieldTxFrequency is refused on every
// write regardless of consent — see the package doc comment. It is named
// so a refusal's Reason and this file's own justification cannot drift
// apart.
const registerTxFrequency = "matrix does not establish what a genuine P1=1 MW's own P5-P8 fields (mode, lockout, tone) must carry on this document; this driver never builds that frame (ts590's own TxFreqHz-Unavailable precedent, its A9)"

// mandatoryFieldRefusal is the refusal a live, always-transmitted byte
// earns when the channel has no Known value for it — the ts590 shape
// (write.go): this document's own choreography note is explicit that
// "All parameters must be entered" on a TS-870S MW ("Other parameters are
// ignored" is NOT stated here, unlike MR's vacant-channel sentence —
// matrix §3, "Read/write choreography"), so every one of P4-P8 is
// transmitted on every write with no "leave it alone" encoding.
func mandatoryFieldRefusal(slotID string, field spec.Field, state codeplug.FieldState, position string) *driver.WriteRefusedError {
	return &driver.WriteRefusedError{
		Slot:   slotID,
		Fields: []spec.Field{field},
		Reason: fmt.Sprintf("%s FieldState is %q, not %q, and %s is transmitted on every write with no \"leave it alone\" encoding (matrix §3: \"a TS-870S write is documented as needing every one of P1/P3-P8 supplied\") — so a non-Known value here could only be manufactured, which is what a state meaning \"preserve whatever the radio has\" forbids", field, state, codeplug.Known, position),
	}
}

// toneIndex is the position of tone in this row's own 39-entry chart
// (matrix §1.9) — ONE-BASED, unlike ts590's family chart: index N is
// caps.CTCSSTones[N-1] (see read.go's tone, the inverse of this).
func toneIndex(caps spec.Capabilities, tone spec.Tone) (int, bool) {
	for i, t := range caps.CTCSSTones {
		if t == tone {
			return i + 1, true
		}
	}
	return 0, false
}

// boolWire renders a neutral flag as the '0'/'1' this document's P6
// lockout byte prints.
func boolWire(on bool) byte {
	if on {
		return '1'
	}
	return '0'
}

// WriteChannel implements driver.Session: ONE MW frame per slot, refusing
// before any wire traffic whenever the channel requests something this
// session cannot honestly write.
//
// THE LADDER, in order: unknown slot; empty channel (erase — this codec
// builds no erase frame, matching the family's own refusal reasoning: no
// clear route is printed for MR/MW at all here, matrix §2 "FieldErase");
// field-state coherence (driver.CheckFieldStates); the capability gate
// (FieldSupport.CanWrite); a Known tx_frequency, refused unconditionally
// (registerTxFrequency); the five mandatory-Known fields this record
// transmits on every write; the tone-mode/tone-tx vocabulary; then the
// frame.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	res := driver.WriteResult{Steps: []driver.WriteStep{}}

	channel, err := parseSlotID(ch.Slot)
	if err != nil {
		return res, &UnknownSlotError{Slot: ch.Slot, Reason: err.Error()}
	}
	bank, ok := s.bankFor(ch.Slot)
	if !ok {
		return res, &UnknownSlotError{Slot: ch.Slot, Reason: "this row publishes MEM 00-99 only"}
	}

	if ch.Empty() {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "this codec builds no erase/empty MW frame: no clear route is printed for MR/MW on this document, and record870.go's own BuildMWSet refuses an Empty record",
		}
	}
	data := *ch.Data

	if field, err := driver.CheckFieldStates(s.caps, data); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{field}, Reason: err.Error()}
	}

	var unwritable []spec.Field
	for _, f := range []spec.Field{spec.FieldFrequency, spec.FieldMode, spec.FieldScanSkip, spec.FieldToneMode, spec.FieldToneTx} {
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

	// registerTxFrequency: refused unconditionally, ahead of the
	// mandatory-field checks below, since a Known tx_frequency is refused
	// whatever else the channel carries.
	if data.TxFreqHz.State == codeplug.Known {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldTxFrequency},
			Reason: registerTxFrequency,
		}
	}

	cmd, err := s.buildMWSet(ch.Slot, channel, data)
	if err != nil {
		return res, err
	}

	res.Steps = []driver.WriteStep{{Command: "MW"}}
	const mwStep = 0
	if derr := s.send(ctx, cmd); derr != nil {
		res.Steps[mwStep].Sent = errors.Is(derr, transport.ErrRejected)
		return res, fmt.Errorf("ts870s: WriteChannel %s: %w", ch.Slot, derr)
	}
	res.Steps[mwStep].Sent = true
	return res, nil
}

// send writes cmd and types the two wire events a Kenwood exchange can
// meet, through wireFailure (ts870s.go), so the read path and the write
// path apply one rule once.
func (s *Session) send(ctx context.Context, cmd kw.Command) error {
	_, err := s.eng.Do(ctx, cmd, transport.CommandSpec{Class: transport.ClassWrite})
	if err != nil {
		return wireFailure(s.layout.Book(), "MW", err)
	}
	return nil
}

// buildMWSet maps a populated channel onto its ONE 22-byte MW Set,
// refusing any value the record cannot carry.
func (s *Session) buildMWSet(slotID string, channel int, data codeplug.ChannelData) (kw.Command, error) {
	for _, m := range []struct {
		field    spec.Field
		state    codeplug.FieldState
		position string
	}{
		{spec.FieldScanSkip, data.ScanSkip.State, "P6, the channel lockout"},
		{spec.FieldToneMode, data.ToneMode.State, "P7, the tone mode"},
	} {
		if m.state != codeplug.Known {
			return kw.Command{}, mandatoryFieldRefusal(slotID, m.field, m.state, m.position)
		}
	}

	var toneMode kw.ToneMode
	var toneIdx int
	switch data.ToneMode.Value {
	case "OFF":
		toneMode = kw.ToneModeOff
	case "TONE":
		toneMode = kw.ToneModeTone
		if data.ToneTx.State != codeplug.Known {
			return kw.Command{}, mandatoryFieldRefusal(slotID, spec.FieldToneTx, data.ToneTx.State, "P8, the tone number (required whenever P7 is TONE)")
		}
		idx, ok := toneIndex(s.caps, data.ToneTx.Value)
		if !ok {
			return kw.Command{}, &driver.WriteRefusedError{
				Slot:   slotID,
				Fields: []spec.Field{spec.FieldToneTx},
				Reason: fmt.Sprintf("tone_tx %v is not in the 39-entry chart this row publishes (matrix §1.9)", data.ToneTx.Value),
			}
		}
		toneIdx = idx
	default:
		// Unreachable after CheckFieldStates, which judges a Known tone
		// mode against this session's ToneModes vocabulary — the same
		// two values this switch names.
		return kw.Command{}, &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldToneMode},
			Reason: fmt.Sprintf("tone mode %q is not one this row publishes", data.ToneMode.Value),
		}
	}

	// data.Mode: this row's own MD-legend name, resolved through the
	// layout's forward map (the same map read.go's channelData reads
	// from the other direction).
	mode, ok := s.modeWire(data.Mode)
	if !ok {
		return kw.Command{}, &driver.WriteRefusedError{
			Slot: slotID, Fields: []spec.Field{spec.FieldMode},
			Reason: fmt.Sprintf("mode %q is not one this row's MD legend names (matrix §1.5)", data.Mode),
		}
	}

	cmd, err := s.layout.BuildMWSet(kw.Record870{
		Channel:   channel,
		FreqHz:    data.FreqHz,
		Mode:      mode,
		Lockout:   boolWire(data.ScanSkip.Value),
		ToneMode:  toneMode,
		ToneIndex: toneIdx,
	})
	if err != nil {
		return kw.Command{}, fmt.Errorf("ts870s: WriteChannel %s: %w: %w", slotID, driver.ErrWriteRefused, err)
	}
	return cmd, nil
}

// modeWire resolves a neutral mode name against this layout's own legend,
// the inverse of read.go's channelData lookup.
func (s *Session) modeWire(name string) (kw.Mode, bool) {
	for m, n := range s.layout.ModeNames() {
		if n == name {
			return m, true
		}
	}
	return 0, false
}
