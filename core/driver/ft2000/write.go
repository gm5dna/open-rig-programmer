// SPDX-License-Identifier: GPL-3.0-or-later

package ft2000

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// fnfSpec is the transport spec for the MW Set: fire-and-forget, only the
// bounded listen for a delayed "?;" rejection.
func fnfSpec() transport.CommandSpec {
	return transport.CATWriteSpec()
}

// ctcssByName is ctcssNames' write-direction inverse.
var ctcssByName = map[string]cat.CTCSSState{
	"OFF":     cat.CTCSSOff,
	"ENC-DEC": cat.CTCSSEncDec,
	"ENC":     cat.CTCSSEnc,
}

// requestedFields lists every spec.Field a write of data actually
// requests: the five fields the bare MW frame always carries
// (frequency/mode/clarifier/ctcss-state/shift), unconditionally, plus
// CTCSSTone when — and only when — its FieldState is Known (this radio's
// P9 is a live tone-table index, matrix §1.3 — unlike every registered
// sibling, where the byte is printed-fixed and there is no such field to
// request at all), plus the Icom tier's seventeen fields — SHARED with
// every other Yaesu package, yaesu.TierRequestedFields — each only when
// its own FieldState is Known.
//
// THERE IS NO FieldTag/FieldTagDisplay HERE, UNLIKE EVERY REGISTERED
// SIBLING'S requestedFields (and yaesu.RequestedFields, which this
// function deliberately does NOT reuse for exactly this reason): this
// family has no tag/name route over CAT at all (matrix §0), and both
// fields carry the zero FieldSupport on every bank (caps.go) — requesting
// either unconditionally would refuse every ordinary write on this radio,
// where every sibling's tag is instead always transmitted alongside the
// channel.
func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldClarifier,
		spec.FieldCTCSSState,
		spec.FieldShift,
	}
	if data.CTCSSTone.State == codeplug.Known {
		fields = append(fields, spec.FieldCTCSSTone)
	}
	for _, t := range yaesu.TierRequestedFields {
		if t.Present(data) {
			fields = append(fields, t.Field)
		}
	}
	return fields
}

// WriteChannel implements driver.Session: a single MW (channel data) Set,
// fire-and-forget with the transport's bounded "?;" listen. There is no
// MT to sequence beside it: this family has no tag/name route over CAT at
// all (matrix §0), and MW's own P7 is write-fixed (matrix §1.4) rather
// than a session-supplied kind.
//
// Refusal comes FIRST — before ANY wire traffic — mirroring the fleet's
// shared WriteChannel shape (core/driver/internal/yaesu.WriteChannel,
// core/driver/ft710's own): parse the slot, find its bank, refuse an
// empty channel as an unexpressable erase (matrix §3: no erase/clear
// command exists for a memory channel in this radio's 90-command index),
// check every FieldState is internally coherent (driver.CheckFieldStates,
// the fleet-shared walk), then the capability gate over requestedFields.
// Only then is the MW frame built, which can refuse again at the value
// level (buildMWCommand).
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	res := driver.WriteResult{Steps: []driver.WriteStep{}}

	if _, err := s.dialect.ParseSlot(ch.Slot); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: fmt.Sprintf("not a valid slot: %v", err)}
	}
	bank, ok := s.caps.BankOf(ch.Slot)
	if !ok {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: "slot is not part of any bank this session supports"}
	}

	if ch.Empty() {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "erase cannot be expressed by the CAT codec (no erase/clear command exists for a memory channel), and FieldErase is not write-Supported",
		}
	}

	if field, err := driver.CheckFieldStates(s.caps, *ch.Data); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{field}, Reason: err.Error()}
	}

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

	cmd, err := s.buildMWCommand(ch)
	if err != nil {
		return res, err
	}

	res.Steps = []driver.WriteStep{{Command: "MW"}}
	const mwStep = 0

	if _, err := s.eng.Do(ctx, cmd, fnfSpec()); err != nil {
		if errors.Is(err, cat.ErrRejected) {
			res.Steps[mwStep].Sent = true
			return res, fmt.Errorf("ft2000: WriteChannel %s: MW rejected by radio: %w", ch.Slot, err)
		}
		return res, fmt.Errorf("ft2000: WriteChannel %s: MW: %w", ch.Slot, err)
	}
	res.Steps[mwStep].Sent, res.Steps[mwStep].Confirmed = true, true

	return res, nil
}

// buildMWCommand maps a populated channel onto its MW Set frame, refusing
// (typed, via *driver.WriteRefusedError) any value the codec cannot
// express. Called only after WriteChannel's capability gate has passed.
func (s *Session) buildMWCommand(ch codeplug.Channel) (cat.Command, error) {
	sl, err := s.dialect.ParseSlot(ch.Slot)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: err.Error()}
	}
	data := *ch.Data

	mode, ok := s.dialect.ModeByName(data.Mode)
	if !ok {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldMode},
			Reason: fmt.Sprintf("mode %q is not a mode this radio supports", data.Mode),
		}
	}
	ctcss, ok := ctcssByName[data.CTCSS]
	if !ok {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldCTCSSState},
			Reason: fmt.Sprintf("ctcss state %q is not one of OFF/ENC-DEC/ENC", data.CTCSS),
		}
	}
	shift, ok := yaesu.ShiftByName[data.Shift]
	if !ok {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldShift},
			Reason: fmt.Sprintf("shift %q is not one of SIMPLEX/PLUS/MINUS", data.Shift),
		}
	}

	clar := s.dialect.Clarifier()
	if data.ClarHz < -clar.MaxAbsHz || data.ClarHz > clar.MaxAbsHz {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldClarifier},
			Reason: fmt.Sprintf("clarifier %d Hz exceeds +/-%d Hz", data.ClarHz, clar.MaxAbsHz),
		}
	}

	// The P9 tone index (matrix §1.3): a Known CTCSSTone must be one of
	// THIS SESSION's own CTCSSTones (already established by the capability
	// gate's driver.CheckFieldStates rung, via codeplug.ToneField.Valid ->
	// spec.Capabilities.AdmitsTone), so the lookup below cannot fail on a
	// well-formed write — but it is not re-derived here on faith: a caller
	// bypassing that gate gets a refusal naming the field, not a panic or a
	// wrong index. Non-Known leaves ToneIndex at its zero value, which
	// dialect.BuildMWSet then round-trips as tone number "00" — a real,
	// representable channel state (matrix §1.3's chart index 0), not a
	// placeholder; requestedFields above never asks for the field on this
	// path, so the gate above never demanded a value for it either.
	var toneIndex uint8
	if data.CTCSSTone.State == codeplug.Known {
		idx := slices.Index(s.caps.CTCSSTones, data.CTCSSTone.Value)
		if idx < 0 {
			return cat.Command{}, &driver.WriteRefusedError{
				Slot: ch.Slot, Fields: []spec.Field{spec.FieldCTCSSTone},
				Reason: fmt.Sprintf("tone %v is not one of this radio's 50-entry CTCSS tone chart", data.CTCSSTone.Value),
			}
		}
		toneIndex = uint8(idx)
	}

	// THIS RADIO'S OWN RANGE, checked BEFORE cat.MemoryFreqHz's generic
	// ceiling: that helper bounds a value against the REGISTERED (9-digit)
	// family's memFreqMax (memdata.go), which is one digit too WIDE for
	// this family's 8-digit P2 field (matrix §1.1, MemoryFreqDigits 8) —
	// a value between caps.MaxFreqHz and the 9-digit ceiling would pass it
	// and then overflow encodeMemoryFields' fixed-width rendering,
	// corrupting every byte after P2. caps.MaxFreqHz is this family's own
	// MANUAL-EVIDENCED ceiling (FA/FB Set legend, "00030000 - 60000000
	// (Hz)", ft2000_layout.txt:654/:666 — a Phase-3 finding beyond the
	// matrix, which had left this range open) and its digit count fits
	// the 8-digit field exactly ("60000000" is 8 characters), so this
	// check is what keeps a caller-supplied value from ever reaching that
	// gap.
	if data.FreqHz < s.caps.MinFreqHz || data.FreqHz > s.caps.MaxFreqHz {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldFrequency},
			Reason: fmt.Sprintf("frequency %d Hz is outside this radio's storable range %d-%d Hz", data.FreqHz, s.caps.MinFreqHz, s.caps.MaxFreqHz),
		}
	}

	freqHz, err := cat.MemoryFreqHz(data.FreqHz)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldFrequency}, Reason: err.Error()}
	}

	cmd, err := s.dialect.BuildMWSet(cat.MemoryData{
		Slot:      sl,
		FreqHz:    freqHz,
		ClarHz:    int16(data.ClarHz),
		RxClar:    data.RxClar,
		TxClar:    data.TxClar,
		Mode:      mode,
		Kind:      s.dialect.MWWriteKind(),
		CTCSS:     ctcss,
		ToneIndex: toneIndex,
		Shift:     shift,
	})
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: fmt.Sprintf("cannot encode MW frame: %v", err)}
	}
	return cmd, nil
}
