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
//   - A TagDisplay that is not Known is refused, in buildWriteCommands and
//     before ANY other field mapping: MT's display flag is mandatory, so
//     there is no way to send the channel without inventing a value for it
//     (see buildWriteCommands). codeplug.Diff blocks such a channel at plan
//     time; this refusal is the defence-in-depth behind that.
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
// Kind-on-write: the MW frame's P7 is the SESSION DIALECT's declared write
// kind (cat.Dialect.MWWriteKind, consulted since M9c-3 task 9), which for
// the FT-710 is ALWAYS '1' (KindMemory), for both memory and PMS slots.
// HW-CONFIRMED 2026-07-13 (M5b write trials,
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
	if err := ch.Data.CTCSSTone.Valid(s.caps); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldCTCSSTone}, Reason: err.Error()}
	}
	if err := ch.Data.ScanSkip.Valid(); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldScanSkip}, Reason: err.Error()}
	}
	if err := ch.Data.TagDisplay.Valid(); err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldTagDisplay}, Reason: err.Error()}
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

	// THE pre-wire refusal for TagDisplay, FIRST and before any other field
	// mapping. The MT frame's display flag (P1) is MANDATORY — the frame has
	// no "leave it alone" encoding — so sending a channel whose TagDisplay is
	// not Known would MANUFACTURE a value for a field whose FieldState says
	// "preserve whatever the radio has", which is exactly what codeplug's
	// write rule forbids.
	//
	// Position is load-bearing, not stylistic: a channel that is wrong in
	// several ways at once must still name THIS field, because this is the
	// one whose failure mode is a silent wrong byte on the wire rather than
	// a refusal. From this commit there is no path from here to BuildMTSet
	// that carries a non-Known display flag.
	//
	// codeplug.Diff blocks such a channel at PLAN time, which is the
	// user-facing route and the one that produces a helpful message; this is
	// the belt to that pair of braces, in the same spirit as WriteChannel's
	// capability gate.
	if data.TagDisplay.State != codeplug.Known {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldTagDisplay},
			Reason: fmt.Sprintf("tag display FieldState is %q, not %q; only a Known value is ever sent to a radio", data.TagDisplay.State, codeplug.Known),
		}
	}

	// Resolved through THIS dialect (task 67, M9c-0), not a driver-private
	// table: before this, a dialect that renamed a mode had no effect on
	// what got written — see modeTable's doc comment (caps.go) and
	// cat.Dialect.ModeByName's own for the finding this closes.
	mode, ok := dialect.ModeByName(data.Mode)
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
	//
	// The bound is THIS DIALECT'S (M9c-3 task 9), in the comparison and in
	// the message alike: both hardcoded +-9990 until now, so a receiver with
	// a narrower range had its over-range values waved past this check, and
	// a wider one had its legitimate values refused here with a bound that
	// was never its own. cat.FT710's own MaxAbsHz is 9990, so this renders
	// byte-identically for the FT-710.
	clar := dialect.Clarifier()
	if data.ClarHz < -clar.MaxAbsHz || data.ClarHz > clar.MaxAbsHz {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldClarifier},
			Reason: fmt.Sprintf("clarifier %d Hz exceeds +/-%d Hz", data.ClarHz, clar.MaxAbsHz),
		}
	}

	// Kind-on-write: THIS DIALECT'S declared write kind, for both memory and
	// PMS slots. The FT-710's is KindMemory ('1'), HW-CONFIRMED 2026-07-13
	// (see WriteChannel's doc comment) — but that evidence is about one
	// radio, so since M9c-3 task 9 the byte comes from the receiver rather
	// than a cat.KindMemory literal here, which wrote the FT-710's finding
	// onto whatever dialect this path was handed and had
	// cat.Dialect.BuildMWSet refuse another receiver's legitimate write.
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
		Kind:   dialect.MWWriteKind(),
		CTCSS:  ctcss,
		Shift:  shift,
	})
	if err != nil {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: fmt.Sprintf("cannot encode MW frame: %v", err)}
	}

	// data.TagDisplay.Value is safe to read here and ONLY here: the refusal
	// at the top of this function has already established State == Known.
	mtCmd, err = dialect.BuildMTSet(sl, data.TagDisplay.Value, data.Tag)
	if err != nil {
		return cat.Command{}, cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldTag},
			Reason: fmt.Sprintf("cannot encode MT frame: %v", err),
		}
	}
	return mwCmd, mtCmd, nil
}
