// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7200 "github.com/gm5dna/open-rig-programmer/core/civ/ic7200"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// memorySetStep is the mnemonic this driver reports for its one write
// frame.
const memorySetStep = "1A 00"

// ErrUnmappedRegion is the sentinel for this model's E6-style refusal:
// the slot's unmapped record regions (Split, and the TX-duplicate
// block's mode/filter/data-mode mirror) differ from the profile's Fixed
// template.
var ErrUnmappedRegion = errors.New("ic7200: the slot's unmapped record regions differ from this profile's Fixed template")

// UnmappedRegionError reports which unmapped byte disagreed, and by how
// much. A channel whose Split byte is ON, or whose TX-mirror bytes carry
// something this driver does not understand, CANNOT be written by this
// programme at all — it is never silently cleared and never silently
// rewritten (matrix §3.16 ADDED-1; the same ruling every sibling Icom
// package in this tier gives its own unmapped regions).
type UnmappedRegionError struct {
	// Offset is the 0-based record byte whose unmapped region differed.
	Offset int
	// Want and Got are that byte's value in the template and in the
	// slot's actual record.
	Want, Got byte
}

func (e *UnmappedRegionError) Error() string {
	what := "an unmapped region"
	switch e.Offset {
	case civic7200.SplitOffset:
		what = "the Split flag (00: OFF, 10: ON)"
	case civic7200.TXModeOffset:
		what = "the TX mode mirror"
	case civic7200.TXFilterOffset:
		what = "the TX filter mirror"
	case civic7200.TXDataModeOffset:
		what = "the TX data-mode mirror"
	}
	return fmt.Sprintf(
		"ic7200: this slot's record byte %d carries %#02x where the profile's Fixed template carries %#02x — %s. This channel cannot be written by this programme at all; it is not downgraded and not cleared",
		e.Offset, e.Got, e.Want, what)
}

// Unwrap lets errors.Is(err, ErrUnmappedRegion) match.
func (e *UnmappedRegionError) Unwrap() error { return ErrUnmappedRegion }

// ErrOutOfDomain is the sentinel for a Known value outside what this
// radio's record can encode.
var ErrOutOfDomain = errors.New("ic7200: a Known value lies outside what this radio's record can encode")

// OutOfDomainError reports a Known numeric value — asked for on a write,
// or decoded on a read — that this radio's record cannot express.
//
// DEFENCE IN DEPTH, AND NOT THE GATE: civ.FieldSpan carries no numeric
// domain, so civ.Profile.AllowedCommand would admit a set carrying
// 65 MHz. codeplug.Validate already bounds the primary frequency at the
// capability ceiling; this refusal closes the driver's own door too.
type OutOfDomainError struct {
	Field spec.Field
	Value uint64
	Max   uint64
}

func (e *OutOfDomainError) Error() string {
	return fmt.Sprintf(
		"ic7200: %s = %d is outside what this radio's record can encode (maximum %d) — refused by the driver",
		e.Field, e.Value, e.Max)
}

// Unwrap lets errors.Is(err, ErrOutOfDomain) match.
func (e *OutOfDomainError) Unwrap() error { return ErrOutOfDomain }

// refused builds the neutral refusal, with an empty (never nil) Steps
// slice: a refusal that happens before the frames are built has no
// sequence to describe.
func refused(slot string, fields []spec.Field, reason string) (driver.WriteResult, error) {
	return driver.WriteResult{Steps: []driver.WriteStep{}},
		&driver.WriteRefusedError{Slot: slot, Fields: fields, Reason: reason}
}

// requestedFields is every spec.Field a write of d would transmit —
// matrix §2's five mapped fields, frequency and mode unconditional plus
// tx_frequency only when the caller supplied one (its absence is not a
// request; rung 7 supplies the SimplexTxEqualsRx default instead).
func requestedFields(d codeplug.ChannelData) []spec.Field {
	fields := []spec.Field{spec.FieldFrequency, spec.FieldMode, spec.FieldFilter, spec.FieldDataMode}
	if d.TxFreqHz.State == codeplug.Known {
		fields = append(fields, spec.FieldTxFrequency)
	}
	return fields
}

// WriteChannel implements driver.Session: ONE acknowledged 1A 00 memory
// set, preceded by ONE read.
//
// THE LADDER:
//
//	LOCALLY DECIDABLE (all precede ALL wire traffic):
//	  1. erase?              Channel.Data == nil          -> ErrWriteRefused
//	  2. capability gate      requestedFields x FieldSupport -> ErrWriteRefused
//	  3. field-state shape    mode/filter/data_mode Known   -> ErrWriteRefused
//	  3b. vocabularies        mode/filter not expressible   -> ErrWriteRefused
//	  4. numeric domains      freq/tx_freq out of range     -> *OutOfDomainError
//	  -----------------------------------------------------------------------
//	  5. ONE read             readRaw (T2 address check, T4 rejection branch)
//	  -----------------------------------------------------------------------
//	READ-DEPENDENT (the one recorded exception):
//	  6. unmapped regions     Split, TX mode/filter/data-mode mirror -> *UnmappedRegionError
//	  -----------------------------------------------------------------------
//	  7. TX frequency source  Known -> caller's value; else -> mirror RX (SimplexTxEqualsRx)
//	  8. BuildMemorySet -> the outbound gate -> the acknowledged exchange
//
// RUNG 7 NEEDS NO "PRIOR RECORD" ARM, unlike a sibling model's tone
// preservation: the manual's own NOTE (matrix §3.11) states the
// TX-duplicate block should carry the SAME data as the RX span
// regardless of Split, so an absent TxFreqHz always has a deterministic
// answer and this driver never refuses a CREATE for lack of one.
//
// This driver NEVER writes Split ON: the byte is unmapped (matrix
// §3.15(a)) and BuildMemorySet always emits the Fixed template's 0x00
// there, matching the manual's own SCAN-bank guidance ("both settings
// should be 00").
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// RUNG 1 — ERASE.
	if ch.Data == nil {
		return refused(ch.Slot, []spec.Field{spec.FieldErase},
			"this radio documents a 0B clear command (matrix §3.13), but no IC-7200 has ever been asked to use it, so FieldErase carries the zero FieldSupport and consent structurally never reaches it")
	}
	d := *ch.Data

	a, bank, err := slotToAddress(ch.Slot)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}}, fmt.Errorf("ic7200: WriteChannel: %w", err)
	}

	// RUNG 1b — THE FIELD-STATE WALK. core/driver.CheckFieldStates is
	// THE FLEET'S shared walk rather than a table of this package's own:
	// every field of ChannelData that carries a FieldState, judged
	// against this session's own vocabularies. What it prevents is
	// silent rather than loud — a value carried alongside a state
	// meaning "preserve whatever the radio has" (or alongside Absent,
	// the zero state a hand-built ChannelData leaves behind) is never
	// named by requestedFields, so without this rung it would be
	// DROPPED from the frame and the write would report success (the
	// FT-891 closing review's C-M1, sharpened by MEDIUM-1 to
	// codeplug.Absent). This package's own RUNG 3/3b below cover only
	// mode, filter and data_mode; this rung is what covers the other
	// seventeen FieldState-carrying fields.
	if field, err := driver.CheckFieldStates(s.caps, d); err != nil {
		return refused(ch.Slot, []spec.Field{field}, err.Error())
	}

	// RUNG 2 — THE CAPABILITY GATE.
	var unwritable []spec.Field
	for _, f := range requestedFields(d) {
		if !s.caps.FieldSupport(bank, f).CanWrite() {
			unwritable = append(unwritable, f)
		}
	}
	if len(unwritable) > 0 {
		return refused(ch.Slot, unwritable,
			"this session cannot write these fields on bank "+string(bank)+": no IC-7200 has ever been written to by this project and no consent was recorded")
	}

	// RUNG 3 — FIELD-STATE SHAPE. Mode, filter and data mode are all
	// mandatory enums with no "leave it alone" encoding.
	if d.Mode == "" {
		return refused(ch.Slot, []spec.Field{spec.FieldMode}, "the record's ⑨ is a mode enum with no \"leave it alone\" encoding")
	}
	if d.Filter.State != codeplug.Known {
		return refused(ch.Slot, []spec.Field{spec.FieldFilter}, "the record's ⑩ is a filter enum with no \"leave it alone\" encoding")
	}
	if d.DataMode.State != codeplug.Known {
		return refused(ch.Slot, []spec.Field{spec.FieldDataMode}, "the record's ⑪ is a data-mode enum with no \"leave it alone\" encoding")
	}

	// RUNG 3b — VOCABULARIES.
	if !slices.Contains(s.caps.Modes, d.Mode) {
		return refused(ch.Slot, []spec.Field{spec.FieldMode}, fmt.Sprintf("%q is not a value this radio can express (%v)", d.Mode, s.caps.Modes))
	}
	if !slices.Contains(s.caps.Filters, d.Filter.Value) {
		return refused(ch.Slot, []spec.Field{spec.FieldFilter}, fmt.Sprintf("%q is not a value this radio can express (%v)", d.Filter.Value, s.caps.Filters))
	}

	// RUNG 4 — NUMERIC DOMAINS.
	if d.FreqHz < s.caps.MinFreqHz || d.FreqHz > s.caps.MaxFreqHz {
		return driver.WriteResult{Steps: []driver.WriteStep{}},
			&OutOfDomainError{Field: spec.FieldFrequency, Value: d.FreqHz, Max: s.caps.MaxFreqHz}
	}
	if d.TxFreqHz.State == codeplug.Known && (d.TxFreqHz.Value < s.caps.MinFreqHz || d.TxFreqHz.Value > s.caps.MaxFreqHz) {
		return driver.WriteResult{Steps: []driver.WriteStep{}},
			&OutOfDomainError{Field: spec.FieldTxFrequency, Value: d.TxFreqHz.Value, Max: s.caps.MaxFreqHz}
	}

	// RUNG 5 — THE ONE READ.
	_, raw, empty, err := s.readRaw(ctx, a)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}},
			fmt.Errorf("ic7200: WriteChannel %s: the pre-write preservation read: %w", ch.Slot, err)
	}

	// RUNG 6 — THE UNMAPPED-REGION CHECK. An EMPTY slot has no unmapped
	// regions to compare and the write proceeds against the template.
	if !empty {
		if e := unmappedRegionsDiffer(raw); e != nil {
			return driver.WriteResult{Steps: []driver.WriteStep{}}, e
		}
	}

	// RUNG 7 — THE TX-FREQUENCY SOURCE (SimplexTxEqualsRx).
	txFreq := d.FreqHz
	if d.TxFreqHz.State == codeplug.Known {
		txFreq = d.TxFreqHz.Value
	}
	dataModeValue := "OFF"
	if d.DataMode.Value {
		dataModeValue = "ON"
	}

	// RUNG 8 — BUILD, GATE, EXCHANGE. BuildMemorySet writes the
	// profile's Fixed template into every unmapped region (Split and the
	// TX-mirror bytes), so the 17 record bytes are always sent in full
	// (register entry ic7200-full-record-mandatory).
	rec := civ.MemoryRecord{
		Address:  a,
		RXFreqHz: civ.Available(d.FreqHz),
		Mode:     civ.Available(d.Mode),
		Filter:   civ.Available(d.Filter.Value),
		DataMode: civ.Available(dataModeValue),
		TXFreqHz: civ.Available(txFreq),
	}
	p := civic7200.Profile()
	cmd, err := p.BuildMemorySet(rec)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}},
			fmt.Errorf("ic7200: WriteChannel %s: building the 1A 00 set: %w", ch.Slot, err)
	}

	steps := []driver.WriteStep{{Command: memorySetStep}}
	_, err = s.eng.Do(ctx, cmd, civ.CIVWriteWithAckSpec(p.AcknowledgementMatcher()))
	switch {
	case err == nil:
		// FB arrived (register entry ic7200-1a00-set-ack: the codes are
		// MANUAL-EVIDENCED, but that a 1A 00 SET is answered by one of
		// them is ASSUMED).
		steps[0].Sent = true
		steps[0].Confirmed = true
		return driver.WriteResult{Steps: steps}, nil

	case errors.Is(err, transport.ErrRejected):
		steps[0].Sent = true
		return driver.WriteResult{Steps: steps},
			fmt.Errorf("ic7200: WriteChannel %s: the radio rejected the %s set: %w", ch.Slot, memorySetStep, err)

	default:
		return driver.WriteResult{Steps: steps},
			fmt.Errorf("ic7200: WriteChannel %s: the %s set was written and never acknowledged, so its outcome is unknown: %w", ch.Slot, memorySetStep, err)
	}
}

// unmappedRegionsDiffer applies this model's E6-style rule to the four
// unmapped record bytes, in a fixed order so the refusal a caller meets
// is deterministic.
func unmappedRegionsDiffer(raw []byte) error {
	tmpl := civic7200.FixedTemplate()
	if len(raw) < len(tmpl) {
		// UNREACHABLE: the length fingerprint has already refused any
		// answer but this profile's own length before readRaw could
		// return it.
		return fmt.Errorf("ic7200: internal: the unmapped-region comparison was handed a %d-byte record where the profile declares %d", len(raw), len(tmpl))
	}
	for _, offset := range []int{civic7200.SplitOffset, civic7200.TXModeOffset, civic7200.TXFilterOffset, civic7200.TXDataModeOffset} {
		if raw[offset] != tmpl[offset] {
			return &UnmappedRegionError{Offset: offset, Want: tmpl[offset], Got: raw[offset]}
		}
	}
	return nil
}
