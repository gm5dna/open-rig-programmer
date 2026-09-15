// SPDX-License-Identifier: GPL-3.0-or-later

package ft1000mp

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/bincat"
	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

var _ clone.VFOStateRestorer = (*Session)(nil)

// The dump's three fixed non-memory record positions (matrix §1.6): the
// currently-active operating data, VFO-A and VFO-B, in that order, before
// the 113 memory records recordIndex addresses.
const (
	dumpCurrentOpRecordIndex = 0
	dumpVFOARecordIndex      = 1
	dumpVFOBRecordIndex      = 2
)

// shiftByName/shiftCodeOf map codeplug's shift spelling to OpShift's wire
// code (0/1/2 — spec.md §Write model step 5, shared across the family).
var shiftByName = map[string]byte{"SIMPLEX": 0, "MINUS": 1, "PLUS": 2}

func shiftCodeOf(name string) (byte, error) {
	c, ok := shiftByName[name]
	if !ok {
		return 0, fmt.Errorf("shift %q is not one of SIMPLEX/MINUS/PLUS", name)
	}
	return c, nil
}

// buildOffsetArgs builds OpOffset's four argument bytes for a repeater
// offset magnitude of hz — see bincat.Profile.OffsetZeroArg/
// OffsetBoundArg for the FT-1000MP-specific byte positions this must
// satisfy (X4=00H, X3 in 0-2).
//
// ASSUMED ENCODING, flagged rather than silently trusted: the manual's
// own "Repeater Offset" row (ft1000mpmarkv_manual layout:5195-5210,
// printed p.95-96) states "0 ~ 500 kHz in 1-kHz step... X3 is must be
// 00H, 01H, or 02H" — but a hundreds-of-kHz digit capped at 0-2 can only
// reach 299 kHz, not the claimed 500 kHz ceiling, an internal
// contradiction this driver cannot resolve from the manual alone (the
// same kind of self-contradiction spec.md's channel-numbering-base
// finding already documents for this radio). This driver therefore
// ENCODES ONLY 0-299 kHz (X1=00H always — the manual's own step size is
// whole kHz, so no sub-kHz component is ever needed; X2=BCD ones+tens of
// kHz; X3=hundreds of kHz, 0-2) and REFUSES anything above that rather
// than guess a fourth byte's meaning. Owner-probe: capture a real F9H
// frame for an offset above 299 kHz (e.g. the common 600 kHz 2 m split)
// before trusting this encoding beyond that range.
func buildOffsetArgs(hz uint64) ([4]byte, error) {
	const maxHz = 299_000
	if hz > maxHz {
		return [4]byte{}, fmt.Errorf("ft1000mp: repeater offset %d Hz exceeds this driver's encodable range (0-%d Hz — see buildOffsetArgs's doc comment for why 500 kHz is not safely encodable)", hz, maxHz)
	}
	if hz%1000 != 0 {
		return [4]byte{}, fmt.Errorf("ft1000mp: repeater offset %d Hz is not a whole kHz (this radio's F9H step is 1 kHz)", hz)
	}
	khz := hz / 1000
	x2, err := bincat.EncodeBCD(khz%100, 1)
	if err != nil {
		return [4]byte{}, fmt.Errorf("ft1000mp: repeater offset: %w", err)
	}
	x3 := byte(khz / 100)
	return [4]byte{0x00, x2[0], x3, 0x00}, nil
}

// buildClarArgs builds OpClarifier's four argument bytes (matrix's own
// evidence: ft1000mpmarkv_manual layout:4610-4633, "CONSTRUCTING AND
// SENDING CAT COMMANDS" Example #2 — C1=BCD tens-of-Hz (0-990 Hz), C2=BCD
// thousands-of-Hz/kHz (0-9000 Hz), C3=00H positive/FFH negative, C4
// selects which clarifier the frame turns on, or CLAR CLEAR).
//
// Only ONE of rx/tx can be expressed per frame (the manual's own C4
// legend has no "both" value) — tx WINS when both are true, matching
// C4's own bit shape (0x80 added to the rx-on value). Neither true sends
// CLAR CLEAR (C1=C2=C3=0, C4=FFH), which this driver also sends whenever
// the channel carries no clarifier — WriteChannel's step 4 must transmit
// this explicit off value, never merely omit the frame (spec.md §Write
// model, Codex #7's step-count rule).
func buildClarArgs(hz int, rxClar, txClar bool) ([4]byte, error) {
	if !rxClar && !txClar {
		return [4]byte{0x00, 0x00, 0x00, 0xFF}, nil
	}
	mag := hz
	if mag < 0 {
		mag = -mag
	}
	const maxHz = 9990
	if mag > maxHz {
		return [4]byte{}, fmt.Errorf("ft1000mp: clarifier %d Hz exceeds this radio's %d Hz write range", hz, maxHz)
	}
	if mag%10 != 0 {
		return [4]byte{}, fmt.Errorf("ft1000mp: clarifier %d Hz is not a whole 10 Hz (this radio's C1 step)", hz)
	}
	thousands := mag / 1000
	tensOfHz := (mag % 1000) / 10
	c1, err := bincat.EncodeBCD(uint64(tensOfHz), 1)
	if err != nil {
		return [4]byte{}, fmt.Errorf("ft1000mp: clarifier: %w", err)
	}
	c2, err := bincat.EncodeBCD(uint64(thousands), 1)
	if err != nil {
		return [4]byte{}, fmt.Errorf("ft1000mp: clarifier: %w", err)
	}
	c3 := byte(0x00)
	if hz < 0 {
		c3 = 0xFF
	}
	c4 := byte(0x01) // RX CLAR ON
	if txClar {
		c4 = 0x81 // TX CLAR ON — wins when both are true, per this func's doc comment
	}
	return [4]byte{c1[0], c2[0], c3, c4}, nil
}

// writeStep sends one write-class frame and reports the WriteStep it
// produced — Sent/Confirmed both true on success (no rejection is
// possible in this family, §Context), Sent false and an error otherwise.
func (s *Session) writeStep(ctx context.Context, mnemonic string, cmd bincat.Command) (driver.WriteStep, error) {
	_, err := s.eng.Do(ctx, cmd, bincat.WriteSpec(0))
	if err != nil {
		return driver.WriteStep{Command: mnemonic, Sent: false, Confirmed: false}, err
	}
	return driver.WriteStep{Command: mnemonic, Sent: true, Confirmed: true}, nil
}

// WriteChannel implements driver.Session: the family's VFO→memory
// choreography (spec.md §Write model), narrowed to this radio's own
// steps — there is no Tone step at all (matrix §2: nothing in the
// 16-byte record can carry a committed tone, so step 6 is simply
// skipped, not sent with an "off" value).
//
// Every plain (non-tri-state) field on ch.Data — FreqHz, Mode, ClarHz/Rx/
// TxClar, Shift — is ALWAYS sent, including its off/simplex/clear value
// when the channel does not use it: this radio's Store/Enter commits
// WHATEVER VFO-A currently holds, so a stale VFO-A value must never be
// left in place by omission (spec.md's step-count Amendment). Offset
// (F9H) is the one CONDITIONAL step: sent only when Shift is MINUS or
// PLUS, per the family's own rule that it is meaningless otherwise; a
// non-SIMPLEX Shift with ch.Data.OffsetHz not Known is refused BEFORE any
// wire traffic (ErrWriteRefused) — this radio cannot "keep the previous
// offset", so an unstated magnitude is not writable at all.
//
// Store/Enter's own K byte position is ASSUMED (caps.go/matrix §1.8);
// per the 15/09/2026 override this ships anyway, consent-gated. A
// Store/Enter failure is reported as this method's own error, wrapping
// ErrStoreUnresolved: the memory's true state is UNKNOWN afterwards,
// never "resolved" by anything WriteChannel itself does (obligation 7's
// read-back is the CALLER's job, not this method's — driver.Session.
// WriteChannel's own contract).
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	if ch.Data == nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: "ft1000mp: WriteChannel: an empty channel has nothing to write (this radio has no erase route — matrix has no Mask/Un-Mask in scope)"}
	}

	// Capability gate — defence in depth below the clone service
	// (driver.Session.WriteChannel's own contract): every field this
	// choreography sends must be CanWrite on THIS session, checked
	// before any wire traffic. Offset is checked only when the channel
	// actually needs it (ch.Data.Shift below decides that) — a SIMPLEX
	// channel never touches FieldOffset at all, consented or not.
	bank, ok := s.caps.BankOf(ch.Slot)
	if !ok {
		return driver.WriteResult{Steps: []driver.WriteStep{}}, fmt.Errorf("ft1000mp: WriteChannel: %w", &driver.WriteRefusedError{Slot: ch.Slot, Reason: "slot is not in any of this session's banks"})
	}
	requestedFields := []spec.Field{spec.FieldFrequency, spec.FieldMode, spec.FieldClarifier, spec.FieldShift}
	if ch.Data.Shift != "SIMPLEX" {
		requestedFields = append(requestedFields, spec.FieldOffset)
	}
	var unwritable []spec.Field
	for _, f := range requestedFields {
		if !s.caps.FieldSupport(bank, f).CanWrite() {
			unwritable = append(unwritable, f)
		}
	}
	if len(unwritable) > 0 {
		return driver.WriteResult{Steps: []driver.WriteStep{}}, &driver.WriteRefusedError{Slot: ch.Slot, Fields: unwritable, Reason: "not writable on this session (consent required for this radio's Unverified fields)"}
	}

	target, err := storeArg(ch.Slot)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}}, fmt.Errorf("ft1000mp: WriteChannel: %w", err)
	}
	modeByte, ok := modeByName[ch.Data.Mode]
	if !ok {
		return driver.WriteResult{Steps: []driver.WriteStep{}}, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldMode}, Reason: fmt.Sprintf("mode %q is not one of this radio's modes", ch.Data.Mode)}
	}
	shiftCode, err := shiftCodeOf(ch.Data.Shift)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}}, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldShift}, Reason: err.Error()}
	}
	sendOffset := shiftCode != shiftByName["SIMPLEX"]
	var offsetArgs [4]byte
	if sendOffset {
		if ch.Data.OffsetHz.State != codeplug.Known {
			return driver.WriteResult{Steps: []driver.WriteStep{}}, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldOffset}, Reason: "a MINUS/PLUS shift needs a Known OffsetHz — this radio cannot preserve a previous offset"}
		}
		offsetArgs, err = buildOffsetArgs(ch.Data.OffsetHz.Value)
		if err != nil {
			return driver.WriteResult{Steps: []driver.WriteStep{}}, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldOffset}, Reason: err.Error()}
		}
	}
	clarArgs, err := buildClarArgs(ch.Data.ClarHz, ch.Data.RxClar, ch.Data.TxClar)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}}, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldClarifier}, Reason: err.Error()}
	}
	if ch.Data.FreqHz%10 != 0 {
		return driver.WriteResult{Steps: []driver.WriteStep{}}, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldFrequency}, Reason: "frequency must be a whole 10 Hz (this radio's SetFreq step)"}
	}
	freqArgs, err := bincat.EncodeBCD(ch.Data.FreqHz/10, 4)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}}, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{spec.FieldFrequency}, Reason: err.Error()}
	}

	// Every check above is locally decidable and precedes all wire
	// traffic (driver.Session.WriteChannel's ordering rule). The dump
	// cache is invalidated now, before the first frame: whatever this
	// call does next makes it stale regardless of outcome.
	s.dump = nil

	var steps []driver.WriteStep
	do := func(mnemonic string, opcode byte, args [4]byte) error {
		step, err := s.writeStep(ctx, mnemonic, bincat.NewCommand(opcode, args))
		steps = append(steps, step)
		return err
	}

	if err := do("A/B", bincat.OpABSelect, [4]byte{0, 0, 0, 0}); err != nil {
		return driver.WriteResult{Steps: steps}, fmt.Errorf("ft1000mp: WriteChannel %s: A/B select: %w", ch.Slot, err)
	}
	var freqArr [4]byte
	copy(freqArr[:], freqArgs)
	if err := do("SetFreq", bincat.OpSetFreq, freqArr); err != nil {
		return driver.WriteResult{Steps: steps}, fmt.Errorf("ft1000mp: WriteChannel %s: SetFreq: %w", ch.Slot, err)
	}
	if err := do("SetMode", bincat.OpSetMode, [4]byte{modeByte, 0, 0, 0}); err != nil {
		return driver.WriteResult{Steps: steps}, fmt.Errorf("ft1000mp: WriteChannel %s: SetMode: %w", ch.Slot, err)
	}
	if err := do("Clarifier", bincat.OpClarifier, clarArgs); err != nil {
		return driver.WriteResult{Steps: steps}, fmt.Errorf("ft1000mp: WriteChannel %s: Clarifier: %w", ch.Slot, err)
	}
	if err := do("Shift", bincat.OpShift, [4]byte{shiftCode, 0, 0, 0}); err != nil {
		return driver.WriteResult{Steps: steps}, fmt.Errorf("ft1000mp: WriteChannel %s: Shift: %w", ch.Slot, err)
	}
	if sendOffset {
		if err := do("Offset", bincat.OpOffset, offsetArgs); err != nil {
			return driver.WriteResult{Steps: steps}, fmt.Errorf("ft1000mp: WriteChannel %s: Offset: %w", ch.Slot, err)
		}
	}
	// Store/Enter: K=00H Enter in the ASSUMED 1st argument byte
	// (caps.go/matrix §1.8) — byte-identical regardless of which
	// candidate byte the firmware actually reads as K, since every
	// candidate is sent 0x00 (matrix §1.8's own reasoning).
	if err := do("Store", bincat.OpStore, [4]byte{0x00, 0x00, 0x00, target}); err != nil {
		return driver.WriteResult{Steps: steps}, fmt.Errorf("ft1000mp: WriteChannel %s: %w: %v", ch.Slot, ErrStoreUnresolved, err)
	}
	return driver.WriteResult{Steps: steps}, nil
}

// SnapshotVFOState implements clone.VFOStateRestorer: it reads VFO-A's
// current content (dumpVFOARecordIndex) for RestoreVFOState to replay,
// and infers which VFO the operator had active by comparing the
// dump's own "current operating data" record (dumpCurrentOpRecordIndex)
// against VFO-A's and VFO-B's records byte-for-byte.
//
// THE COMPARISON IS THIS DRIVER'S OWN INFERENCE, not a documented
// opcode: neither manual names a single byte that states "A or B is
// active" outside the Status Flags' Lock/Tracking bits (which describe
// something else). Current Operating Data is, by definition, whichever
// VFO is presently selected, so it is byte-identical to that VFO's own
// record at the moment of the read — an equality this driver can check
// without guessing. Ambiguous (matches both, or neither — a genuinely
// unexpected radio state) defaults to "A", the choreography's own
// default selection, rather than refusing the whole snapshot.
func (s *Session) SnapshotVFOState(ctx context.Context) (clone.VFOSnapshot, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	s.dump = nil // force a fresh read: a stale cache here would snapshot old VFO-A content
	current, err := s.recordAt(ctx, dumpCurrentOpRecordIndex)
	if err != nil {
		return clone.VFOSnapshot{}, fmt.Errorf("ft1000mp: SnapshotVFOState: %w", err)
	}
	vfoA, err := s.recordAt(ctx, dumpVFOARecordIndex)
	if err != nil {
		return clone.VFOSnapshot{}, fmt.Errorf("ft1000mp: SnapshotVFOState: %w", err)
	}
	vfoB, err := s.recordAt(ctx, dumpVFOBRecordIndex)
	if err != nil {
		return clone.VFOSnapshot{}, fmt.Errorf("ft1000mp: SnapshotVFOState: %w", err)
	}

	active := "A"
	if current != vfoA && current == vfoB {
		active = "B"
	}

	content, err := recordToChannelData(vfoA)
	if err != nil {
		return clone.VFOSnapshot{}, fmt.Errorf("ft1000mp: SnapshotVFOState: VFO-A: %w", err)
	}
	return clone.VFOSnapshot{Content: *content, ActiveVFO: active}, nil
}

// RestoreVFOState implements clone.VFOStateRestorer: it re-sends
// snapshot.Content to VFO-A (the same frames WriteChannel's steps 2-6
// build, minus Store) and re-selects snapshot.ActiveVFO — VFO-B, if
// that is what the operator had, since WriteChannel's own step 1 always
// forces A. A restore failure is returned to the caller, which (per
// clone.VFOStateRestorer's contract) treats it as a journal warning,
// never a cause to replace WriteChannel's own result.
func (s *Session) RestoreVFOState(ctx context.Context, snapshot clone.VFOSnapshot) error {
	s.opMu.Lock()
	defer s.opMu.Unlock()
	s.dump = nil

	modeByte, ok := modeByName[snapshot.Content.Mode]
	if !ok {
		return fmt.Errorf("ft1000mp: RestoreVFOState: mode %q is not one of this radio's modes", snapshot.Content.Mode)
	}
	shiftCode, err := shiftCodeOf(snapshot.Content.Shift)
	if err != nil {
		return fmt.Errorf("ft1000mp: RestoreVFOState: %w", err)
	}
	clarArgs, err := buildClarArgs(snapshot.Content.ClarHz, snapshot.Content.RxClar, snapshot.Content.TxClar)
	if err != nil {
		return fmt.Errorf("ft1000mp: RestoreVFOState: %w", err)
	}
	freqArgs, err := bincat.EncodeBCD(snapshot.Content.FreqHz/10, 4)
	if err != nil {
		return fmt.Errorf("ft1000mp: RestoreVFOState: %w", err)
	}
	var freqArr [4]byte
	copy(freqArr[:], freqArgs)

	send := func(opcode byte, args [4]byte) error {
		_, err := s.eng.Do(ctx, bincat.NewCommand(opcode, args), bincat.WriteSpec(0))
		return err
	}

	var errs []error
	errs = append(errs, send(bincat.OpSetFreq, freqArr))
	errs = append(errs, send(bincat.OpSetMode, [4]byte{modeByte, 0, 0, 0}))
	errs = append(errs, send(bincat.OpClarifier, clarArgs))
	errs = append(errs, send(bincat.OpShift, [4]byte{shiftCode, 0, 0, 0}))
	if shiftCode != shiftByName["SIMPLEX"] && snapshot.Content.OffsetHz.State == codeplug.Known {
		if offsetArgs, err := buildOffsetArgs(snapshot.Content.OffsetHz.Value); err == nil {
			errs = append(errs, send(bincat.OpOffset, offsetArgs))
		} else {
			errs = append(errs, err)
		}
	}
	activeV := byte(0)
	if snapshot.ActiveVFO == "B" {
		activeV = 1
	}
	errs = append(errs, send(bincat.OpABSelect, [4]byte{activeV, 0, 0, 0}))

	return errors.Join(errs...)
}
