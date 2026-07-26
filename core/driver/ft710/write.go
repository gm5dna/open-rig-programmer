// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// bankFor reports which of this session's banks claims slot.
func (s *Session) bankFor(slot string) (spec.BankID, bool) {
	for _, b := range s.caps.Banks {
		for _, sl := range b.Slots {
			if sl == slot {
				return b.ID, true
			}
		}
	}
	return "", false
}

// requestedFields lists every spec.Field a write of data actually
// requests: the seven codec-expressible fields are ALWAYS requested (the
// MW frame carries frequency/mode/clarifier/ctcss-state/shift whether or
// not they changed, and the MT frame likewise carries tag+display), plus
// CTCSSTone/ScanSkip when — and only when — their FieldState is Known:
// per codeplug's write rule, Unknown/Unavailable mean "preserve whatever
// the radio has", i.e. nothing is requested for that field. This mirrors
// codeplug.Diff's addedFields so the driver's defence-in-depth gate and
// the diff layer's gate judge the same set.
func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldClarifier,
		spec.FieldCTCSSState,
		spec.FieldShift,
		spec.FieldTag,
		spec.FieldTagDisplay,
	}
	if data.CTCSSTone.State == codeplug.Known {
		fields = append(fields, spec.FieldCTCSSTone)
	}
	if data.ScanSkip.State == codeplug.Known {
		fields = append(fields, spec.FieldScanSkip)
	}
	return fields
}

// WriteChannel implements driver.Session: MW (channel data) then MT
// (tag), both fire-and-forget with the transport's bounded "?;" listen.
//
// Refusal comes FIRST — before ANY wire traffic. This is deliberate
// defence in depth below the clone service: even if every layer above
// failed, this method re-derives the channel's requested field set
// (requestedFields) and re-checks each against THIS session's
// capabilities (FieldSupport.CanWrite, with spec.Inert additionally
// acceptable-to-transmit — see the gate below). In particular:
//
//   - On the all-Unverified fail-safe profile NOTHING is writable (the
//     six rw fields are Unverified), so every channel is refused here —
//     the clarifier's Inert marking alone can never unlock a write.
//   - A Known CTCSS tone or scan skip is refused even on the Simulated
//     profile: the CAT codec cannot express either, and silently
//     dropping a value the caller explicitly marked Known would be a
//     lie.
//   - An empty channel (erase) is refused: no CAT erase command exists
//     (HW-CONFIRMED 2026-07-13 by a properly isolated re-probe — four
//     range/mode-isolated candidate MW frames, every one rejected; see
//     docs/hardware-notes.md's "No CAT erase" section), and FieldErase is
//     nowhere write-Supported.
//   - The clarifier (spec.Inert) is NOT refused here, whatever its
//     value: the radio provably ignores it, so transmitting it is
//     harmless — and this method lacks the baseline needed to tell a
//     changed value from an unchanged one. codeplug.Diff owns that
//     half of the Inert rule (see spec.Inert's doc comment).
//
// Kind-on-write: the MW frame's P7 is ALWAYS '1' (KindMemory), for both
// memory and PMS slots. HW-CONFIRMED 2026-07-13 (M5b write trials,
// docs/hardware-notes.md): the former ASSUMED pairing (KindPMS '5' for a
// PMS slot) is hardware-refuted — the radio REJECTS a PMS write carrying
// KindPMS with an immediate "?;", and accepts the identical write when
// it carries KindMemory instead. cat.Dialect.BuildMWSet enforces the
// same rule (see core/cat/mw.go) and fakeradio mirrors the radio's
// rejection (see internal/fakeradio/parser.go's handleMW).
//
// NO read-back: WriteChannel reports only sent/unrejected (see
// driver.WriteResult). Reading the slot back and comparing is the clone
// service's job — the boundary is deliberate, so verification policy
// (when, how often, what to do on mismatch) lives in one place above
// every driver rather than being half-implemented inside each.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	// Fix 2: hold opMu for the WHOLE MW+MT sequence, not just around each
	// individual Do call — see Session's doc comment. Taken even before
	// the refusal checks below: a refused write returns fast (no wire
	// traffic) either way, so there is no cost to holding it uniformly.
	s.opMu.Lock()
	defer s.opMu.Unlock()

	var res driver.WriteResult

	if _, err := s.dialect.ParseSlot(ch.Slot); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: fmt.Sprintf("not a valid slot: %v", err)}
	}
	bank, ok := s.bankFor(ch.Slot)
	if !ok {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: "slot is not part of any bank this session supports"}
	}

	if ch.Empty() {
		// An empty channel is an ERASE request. FieldErase is nowhere
		// write-Supported, and M5b settled WHY for good: NO CAT erase
		// exists — a properly isolated 13/07/2026 re-probe (four
		// range/mode-isolated candidate MW frames, every one rejected;
		// see docs/hardware-notes.md's "No CAT erase" section)
		// HW-CONFIRMED this permanently, and this codec has no erase
		// command to express one with either. Erased entries stay
		// Blocked by design (Erased->Blocked, confirmed correct).
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "erase cannot be expressed by the CAT codec, and FieldErase is not write-Supported",
		}
	}

	// FieldState sanity before anything else trusts .State: a malformed
	// field (unknown State, or a non-Known value smuggled alongside) is
	// refused, not interpreted.
	if err := ch.Data.CTCSSTone.Valid(); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldCTCSSTone}, Reason: err.Error()}
	}
	if err := ch.Data.ScanSkip.Valid(); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldScanSkip}, Reason: err.Error()}
	}

	// THE write gate (defence in depth below the clone service): every
	// requested field must be write-Supported for this slot's bank in
	// THIS session's capabilities — OR spec.Inert, which is acceptable to
	// TRANSMIT (M5b, HW-CONFIRMED 2026-07-13: the radio ignores the
	// clarifier's transmitted value entirely, so transmitting it cannot
	// alter the radio's state). The Inert enforcement split, documented at
	// both ends (see spec.Inert): blocking a CHANGED Inert value needs the
	// BASELINE to compare against, and this method holds only the channel
	// — that half of the rule lives in codeplug.Diff, which has both
	// sides; this defence-in-depth re-check enforces everything decidable
	// from the channel alone.
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

	// Build BOTH frames before any wire traffic, so a mapping/validation
	// failure in either can still refuse the whole write cleanly.
	mwCmd, mtCmd, err := buildWriteCommands(s.dialect, ch)
	if err != nil {
		return res, err
	}

	// MW first: channel data before its label. Fire-and-forget — the
	// transport listens for a bounded window for a delayed "?;".
	if _, err := s.eng.Do(ctx, mwCmd, fnfSpec()); err != nil {
		if errors.Is(err, cat.ErrRejected) {
			// The frame WAS transmitted; the radio explicitly refused it.
			res.MWSent = true
			return res, fmt.Errorf("ft710: WriteChannel %s: MW rejected by radio: %w", ch.Slot, err)
		}
		return res, fmt.Errorf("ft710: WriteChannel %s: MW: %w", ch.Slot, err)
	}
	res.MWSent, res.MWConfirmed = true, true

	if _, err := s.eng.Do(ctx, mtCmd, fnfSpec()); err != nil {
		if errors.Is(err, cat.ErrRejected) {
			res.MTSent = true
			return res, fmt.Errorf("ft710: WriteChannel %s: MT rejected by radio: %w", ch.Slot, err)
		}
		return res, fmt.Errorf("ft710: WriteChannel %s: MT: %w", ch.Slot, err)
	}
	res.MTSent, res.MTConfirmed = true, true

	return res, nil
}

// buildWriteCommands maps a populated channel onto its MW and MT Set
// frames, refusing (typed, via *driver.WriteRefusedError) any value the
// codec cannot express. Called only after WriteChannel's capability gate
// has passed.
func buildWriteCommands(dialect cat.Dialect, ch codeplug.Channel) (mwCmd, mtCmd cat.Command, err error) {
	sl, err := dialect.ParseSlot(ch.Slot)
	if err != nil {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: err.Error()}
	}
	data := *ch.Data

	mode, ok := modeByName[data.Mode]
	if !ok {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldMode},
			Reason: fmt.Sprintf("mode %q is not a mode this radio supports", data.Mode),
		}
	}
	ctcss, ok := ctcssByName[data.CTCSS]
	if !ok {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldCTCSSState},
			Reason: fmt.Sprintf("ctcss state %q is not one of OFF/ENC-DEC/ENC", data.CTCSS),
		}
	}
	shift, ok := shiftByName[data.Shift]
	if !ok {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldShift},
			Reason: fmt.Sprintf("shift %q is not one of SIMPLEX/PLUS/MINUS", data.Shift),
		}
	}
	// Bounds-check BEFORE the int -> int16 conversion below can wrap; the
	// builder re-validates magnitude and step on top.
	if data.ClarHz < -9990 || data.ClarHz > 9990 {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldClarifier},
			Reason: fmt.Sprintf("clarifier %d Hz exceeds +/-9990 Hz", data.ClarHz),
		}
	}

	// Kind-on-write (HW-CONFIRMED 2026-07-13 — see WriteChannel's doc
	// comment): always KindMemory ('1'), for both memory and PMS slots.
	// Discovered banks (5xx/EMG) can never reach here — their fields are
	// read-only, so the capability gate refused them already;
	// cat.Dialect.BuildMWSet would reject their slots too (not Writable()).
	mwCmd, err = dialect.BuildMWSet(cat.MemoryData{
		Slot:   sl,
		FreqHz: data.FreqHz,
		ClarHz: int16(data.ClarHz),
		RxClar: data.RxClar,
		TxClar: data.TxClar,
		Mode:   mode,
		Kind:   cat.KindMemory,
		CTCSS:  ctcss,
		Shift:  shift,
	})
	if err != nil {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: fmt.Sprintf("cannot encode MW frame: %v", err)}
	}

	mtCmd, err = dialect.BuildMTSet(sl, data.TagDisplay, data.Tag)
	if err != nil {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldTag},
			Reason: fmt.Sprintf("cannot encode MT frame: %v", err),
		}
	}
	return mwCmd, mtCmd, nil
}
