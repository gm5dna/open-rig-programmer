// SPDX-License-Identifier: GPL-3.0-or-later

package ft890900

import (
	"context"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/bincat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// The wire encoding's own hard ceiling for the frequency field (24-bit
// binary, 10s of Hz, range 10000-3000000 — capability matrices §1.4):
// defence in depth, independent of Capabilities.MinFreqHz/MaxFreqHz,
// which the matrices leave OPEN rather than conflate with this
// wire-encoding bound (caps.go's baseCapabilities doc comment).
const (
	wireFreqMinHz = 100_000
	wireFreqMaxHz = 30_000_000
)

// defaultToneIndex is sent whenever a channel's CTCSSTone is not Known:
// this family's tone byte has no off/null encoding at all (every code
// 0-0x20 is a real CTCSS tone, doc.go's "Known, permanent limitations"),
// so the choreography's mandatory Tone step (always sent) must carry
// SOME code. 0x00 (67.0 Hz, the chart's first entry) is an ASSUMED,
// arbitrary placeholder — this driver never claims to control whether a
// tone is actually applied (capability matrices, "CTCSSStates: OPEN").
const defaultToneIndex byte = 0x00

// clarifierOffArgs is the fixed pattern this driver sends for OpClarifier
// whenever a channel does not request a real clarifier value.
// ASSUMED: opcode 09H's argument byte layout could not be recovered from
// either manual (core/bincat's own OpClarifier doc comment), so all-zero
// is a guess at the family's usual "off" convention, never a verified
// one — caps.go grades FieldClarifier's Write Unsupported for exactly
// this reason, and this pattern is the ONLY clarifier frame this driver
// ever transmits.
var clarifierOffArgs = [4]byte{0, 0, 0, 0}

// modeByteFor and shiftByteFor are sharedModes'/the shift vocabulary's
// write-direction inverses.
func modeByteFor(name string) (byte, bool) {
	for b, n := range sharedModes {
		if n == name {
			return b, true
		}
	}
	return 0, false
}

// Shift byte values (opcode 84H, capability matrices §"Shift
// representation"): 0=SIMPLEX, 1=MINUS, 2=PLUS.
func shiftByteFor(name string) (byte, bool) {
	switch name {
	case "SIMPLEX":
		return 0, true
	case "MINUS":
		return 1, true
	case "PLUS":
		return 2, true
	default:
		return 0, false
	}
}

// requestedFields is every spec.Field a write of d would transmit.
// Frequency/Mode/Shift/CTCSSTone are unconditional: the choreography
// always sends all four (Tone with defaultToneIndex when not Known — see
// its doc comment), so treating them as "always requested" is what makes
// the whole write refuse when this session lacks consent, exactly as it
// would for the FT-710 or any other Unverified-baseline driver. Offset is
// requested only when Shift is not SIMPLEX (the choreography's own "only
// when Minus/Plus" rule); Clarifier only when the channel asks for a
// real, non-off value this driver cannot encode at all (caps.go's
// clarSupport).
func requestedFields(d codeplug.ChannelData) []spec.Field {
	fields := []spec.Field{spec.FieldFrequency, spec.FieldMode, spec.FieldShift, spec.FieldCTCSSTone}
	if d.Shift != "SIMPLEX" {
		fields = append(fields, spec.FieldOffset)
	}
	if d.ClarHz != 0 || d.RxClar || d.TxClar {
		fields = append(fields, spec.FieldClarifier)
	}
	return fields
}

// namedFrame pairs one choreography step's mnemonic with the bincat
// opcode/args it sends.
type namedFrame struct {
	label  string
	opcode byte
	args   [4]byte
}

// buildVFOFrames builds steps 1-7 of the write choreography (spec.md
// §Write model, Codex #7's 8-step table minus Store) for content:
// A/B-select, SetFreq, SetMode, Clarifier, Shift, Offset (only when
// non-simplex AND requireOffset tolerates its absence — see the
// requireOffset parameter), Tone.
//
// requireOffset is false only for RestoreVFOState's best-effort restore
// (vfostate.go): VFO-A's repeater-offset MAGNITUDE has no wire read route
// at all (caps.go's offsetSupport doc comment), so a restore can recover
// the shift DIRECTION but never the magnitude a prior channel write may
// have overwritten in between — that frame is simply omitted, a
// documented permanent limitation, never a guess. WriteChannel always
// passes true: a caller-authored channel that asks for a non-simplex
// shift must supply its own offset magnitude, since nothing here may
// invent one.
func buildVFOFrames(d codeplug.ChannelData, requireOffset bool) ([]namedFrame, error) {
	freqTens := d.FreqHz / 10
	bcdFreq, err := bincat.EncodeBCD(freqTens, 4)
	if err != nil {
		return nil, fmt.Errorf("ft890900: encoding frequency %d Hz: %w", d.FreqHz, err)
	}
	modeByte, ok := modeByteFor(d.Mode)
	if !ok {
		return nil, fmt.Errorf("ft890900: %q is not a mode this radio can express", d.Mode)
	}
	shiftByte, ok := shiftByteFor(d.Shift)
	if !ok {
		return nil, fmt.Errorf("ft890900: %q is not a shift value this radio can express", d.Shift)
	}
	toneByte := defaultToneIndex
	if d.CTCSSTone.State == codeplug.Known {
		if b, ok := toneIndexFor(d.CTCSSTone.Value); ok {
			toneByte = b
		}
	}

	frames := []namedFrame{
		{"A/B", bincat.OpABSelect, [4]byte{0, 0, 0, 0}},
		{"SetFreq", bincat.OpSetFreq, [4]byte{bcdFreq[0], bcdFreq[1], bcdFreq[2], bcdFreq[3]}},
		{"SetMode", bincat.OpSetMode, [4]byte{modeByte, 0, 0, 0}},
		{"Clar", bincat.OpClarifier, clarifierOffArgs},
		{"Shift", bincat.OpShift, [4]byte{shiftByte, 0, 0, 0}},
	}

	if d.Shift != "SIMPLEX" {
		switch {
		case d.OffsetHz.State == codeplug.Known:
			bcdOff, err := bincat.EncodeBCD(d.OffsetHz.Value, 3)
			if err != nil {
				return nil, fmt.Errorf("ft890900: encoding repeater offset %d Hz: %w", d.OffsetHz.Value, err)
			}
			frames = append(frames, namedFrame{"Offset", bincat.OpOffset, [4]byte{0, bcdOff[0], bcdOff[1], bcdOff[2]}})
		case requireOffset:
			return nil, fmt.Errorf("ft890900: shift %q requires a known offset magnitude, and this record has no leave-it-alone encoding for it", d.Shift)
			// else: best-effort restore, offset magnitude unrecoverable — omit the frame (see doc comment).
		}
	}

	frames = append(frames, namedFrame{"Tone", bincat.OpTone, [4]byte{toneByte, 0, 0, 0}})
	return frames, nil
}

// sendFrames transmits frames in order via ClassWrite exchanges, building
// the WHOLE intended WriteResult.Steps slice up front (driver.go's
// WriteResult doc comment) before the first frame goes out. It stops at
// the first transport-level error — this family gives no NAK
// (core/bincat's IsRejection is always false), so an error here means a
// transport fault, never a documented rejection.
func (s *Session) sendFrames(ctx context.Context, frames []namedFrame) (driver.WriteResult, error) {
	steps := make([]driver.WriteStep, len(frames))
	for i, f := range frames {
		steps[i] = driver.WriteStep{Command: f.label}
	}
	for i, f := range frames {
		cmd := bincat.NewCommand(f.opcode, f.args)
		if _, err := s.eng.Do(ctx, cmd, bincat.WriteSpec(0)); err != nil {
			return driver.WriteResult{Steps: steps}, fmt.Errorf("ft890900: writing %s: %w", f.label, err)
		}
		steps[i].Sent = true
		steps[i].Confirmed = true
	}
	return driver.WriteResult{Steps: steps}, nil
}

// refused builds the neutral refusal, with an empty (never nil) Steps
// slice: a refusal that happens before the frames are built has no
// sequence to describe.
func refused(slot string, fields []spec.Field, reason string) (driver.WriteResult, error) {
	return driver.WriteResult{Steps: []driver.WriteStep{}},
		&driver.WriteRefusedError{Slot: slot, Fields: fields, Reason: reason}
}

// OutOfDomainError reports a value this radio's wire encoding cannot
// express at all — defence in depth below the capability gate, the same
// role core/driver/ic7200's own OutOfDomainError plays.
type OutOfDomainError struct {
	Field spec.Field
	Value uint64
	Min   uint64
	Max   uint64
}

func (e *OutOfDomainError) Error() string {
	return fmt.Sprintf("ft890900: %s = %d is outside what this radio's wire encoding can express (%d..%d) — refused by the driver", e.Field, e.Value, e.Min, e.Max)
}

// WriteChannel implements driver.Session: the family's 8-step VFO→M
// choreography (spec.md §Write model; doc.go's package comment), with NO
// read-back verification — that is the clone service's job.
//
// THE LADDER (locally decidable rungs all precede any wire traffic):
//
//  1. erase?             ch.Data == nil                -> ErrWriteRefused (FieldErase Unsupported)
//  2. field-state shape   driver.CheckFieldStates       -> ErrWriteRefused
//  3. capability gate     requestedFields x FieldSupport -> ErrWriteRefused
//  4. vocabularies        mode/shift not expressible     -> ErrWriteRefused
//  5. shift/offset shape  non-simplex needs a known offset -> ErrWriteRefused
//  6. numeric domains     freq/offset outside wire encoding -> *OutOfDomainError
//  7. BUILD, GATE, SEND — the 7 VFO-content frames, then Store.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	// RUNG 1 — ERASE.
	if ch.Data == nil {
		return refused(ch.Slot, []spec.Field{spec.FieldErase},
			"no per-channel erase/clear opcode is documented for this radio, so FieldErase carries the zero FieldSupport and consent structurally never reaches it")
	}
	d := *ch.Data

	chNum, err := s.slotToChannel(ch.Slot)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}}, fmt.Errorf("ft890900: WriteChannel: %w", err)
	}

	// RUNG 2 — FIELD-STATE SHAPE (the fleet's shared walk).
	if field, err := driver.CheckFieldStates(s.caps, d); err != nil {
		return refused(ch.Slot, []spec.Field{field}, err.Error())
	}

	// RUNG 3 — THE CAPABILITY GATE.
	var unwritable []spec.Field
	for _, f := range requestedFields(d) {
		if !s.caps.FieldSupport(spec.BankMemory, f).CanWrite() {
			unwritable = append(unwritable, f)
		}
	}
	if len(unwritable) > 0 {
		return refused(ch.Slot, unwritable,
			"this session cannot write these fields on bank MEM: no FT-890/FT-900 has ever been written to by this project and no consent was recorded (or, for clarifier, this driver cannot encode a requested value at all — see caps.go)")
	}

	// RUNG 4 — VOCABULARIES.
	if _, ok := modeByteFor(d.Mode); !ok {
		return refused(ch.Slot, []spec.Field{spec.FieldMode}, fmt.Sprintf("%q is not a value this radio can express (%v)", d.Mode, s.caps.Modes))
	}
	if _, ok := shiftByteFor(d.Shift); !ok {
		return refused(ch.Slot, []spec.Field{spec.FieldShift}, fmt.Sprintf("%q is not a shift value this radio can express", d.Shift))
	}

	// RUNG 5 — SHIFT/OFFSET SHAPE.
	if d.Shift != "SIMPLEX" && d.OffsetHz.State != codeplug.Known {
		return refused(ch.Slot, []spec.Field{spec.FieldOffset},
			"a repeater-offset magnitude is required whenever shift is not simplex, and this record has no leave-it-alone encoding for it")
	}

	// RUNG 6 — NUMERIC DOMAINS.
	if d.FreqHz%10 != 0 || d.FreqHz < wireFreqMinHz || d.FreqHz > wireFreqMaxHz {
		return driver.WriteResult{Steps: []driver.WriteStep{}},
			&OutOfDomainError{Field: spec.FieldFrequency, Value: d.FreqHz, Min: wireFreqMinHz, Max: wireFreqMaxHz}
	}
	if d.Shift != "SIMPLEX" && d.OffsetHz.Value > s.info.OffsetMaxHz {
		return driver.WriteResult{Steps: []driver.WriteStep{}},
			&OutOfDomainError{Field: spec.FieldOffset, Value: d.OffsetHz.Value, Max: s.info.OffsetMaxHz}
	}

	// RUNG 7 — BUILD, GATE, SEND.
	frames, err := buildVFOFrames(d, true)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}}, fmt.Errorf("ft890900: WriteChannel %s: %w", ch.Slot, err)
	}
	frames = append(frames, namedFrame{"Store", bincat.OpStore, [4]byte{byte(chNum), 0, 0, 0}})

	return s.sendFrames(ctx, frames)
}
