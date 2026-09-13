// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx9000

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// requestedFields lists every spec.Field a write of any populated channel
// requests. ALL SEVEN are unconditional, unlike every sibling driver's
// six-plus-conditionals: this radio's 27-byte MW frame carries a value at
// every position on every write, including P9 (the live CTCSS tone index,
// matrix §1.10) — there is no byte this codec can politely leave alone.
// See buildWriteCommand's CTCSSTone refusal for the consequence.
func requestedFields() []spec.Field {
	return []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldClarifier,
		spec.FieldCTCSSState,
		spec.FieldCTCSSTone,
		spec.FieldShift,
	}
}

func fnfSpec() transport.CommandSpec { return transport.CATWriteSpec() }

// WriteChannel implements driver.Session: a single, fire-and-forget MW
// frame. THIS RADIO HAS NO MT COMMAND (matrix §2; dialect.go), so — unlike
// every sibling driver, whose write is one combined MT frame or an
// MW-then-MT pair — this write is a single plain MW, built directly over
// cat.Dialect.BuildMWSet rather than core/driver/internal/yaesu.WriteChannel
// (which builds only the combined MT form).
//
// Refusal comes first, before any wire traffic — the same defence-in-depth
// ladder every sibling driver runs (slot validity, bank membership, erase,
// driver.CheckFieldStates, then the per-field write-support gate).
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
		// spec.md §3: no create/erase for any of the twelve rows in this
		// wave, and this radio's command set documents no erase route
		// either (matrix's own record and command-table review found
		// none). FieldErase is nowhere write-Supported.
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "erase cannot be expressed by this CAT codec, and FieldErase is not write-Supported",
		}
	}

	if field, err := driver.CheckFieldStates(s.caps, *ch.Data); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{field}, Reason: err.Error()}
	}

	var unwritable []spec.Field
	for _, f := range requestedFields() {
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

	cmd, err := buildWriteCommand(s.dialect, ch)
	if err != nil {
		return res, err
	}

	res.Steps = []driver.WriteStep{{Command: "MW"}}
	if _, err := s.eng.Do(ctx, cmd, fnfSpec()); err != nil {
		if errors.Is(err, cat.ErrRejected) {
			res.Steps[0].Sent = true
			return res, fmt.Errorf("ftdx9000: WriteChannel %s: MW rejected by radio: %w", ch.Slot, err)
		}
		return res, fmt.Errorf("ftdx9000: WriteChannel %s: MW: %w", ch.Slot, err)
	}
	res.Steps[0].Sent, res.Steps[0].Confirmed = true, true
	return res, nil
}

// buildWriteCommand maps a populated channel onto its MW Set frame,
// refusing (typed, via *driver.WriteRefusedError) any value the codec
// cannot express. Called only after WriteChannel's capability gate has
// passed.
func buildWriteCommand(dialect cat.Dialect, ch codeplug.Channel) (cat.Command, error) {
	sl, err := dialect.ParseSlot(ch.Slot)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: err.Error()}
	}
	data := *ch.Data

	// THE pre-wire refusal for CTCSSTone, mirroring TagDisplay's refusal on
	// every sibling that carries a mandatory flag: P9 is a live byte on
	// every MW frame with no "leave it alone" encoding, so a channel whose
	// CTCSSTone is not Known cannot be written without inventing a value.
	if data.CTCSSTone.State != codeplug.Known {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldCTCSSTone},
			Reason: fmt.Sprintf("ctcss tone FieldState is %q, not %q; P9 (positions 24-25) is a live tone-table index on every MW frame with no \"leave it alone\" encoding, so only a Known value is ever sent", data.CTCSSTone.State, codeplug.Known),
		}
	}
	toneIdx, ok := indexForTone(data.CTCSSTone.Value)
	if !ok {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldCTCSSTone},
			Reason: fmt.Sprintf("ctcss tone %v is not one of the standard 50-entry chart this radio's P9 indexes", data.CTCSSTone.Value),
		}
	}

	mode, ok := dialect.ModeByName(data.Mode)
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
	shift, ok := shiftByName[data.Shift]
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

	freqHz, err := cat.MemoryFreqHz(data.FreqHz)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldFrequency}, Reason: err.Error()}
	}

	return dialect.BuildMWSet(cat.MemoryData{
		Slot:      sl,
		FreqHz:    freqHz,
		ClarHz:    int16(data.ClarHz),
		RxClar:    data.RxClar,
		TxClar:    data.TxClar,
		Mode:      mode,
		Kind:      dialect.MWWriteKind(),
		CTCSS:     ctcss,
		Shift:     shift,
		ToneIndex: toneIdx,
	})
}
