// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx5000

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// mwSetSpec is the transport spec for the MW Set frame: fire-and-forget,
// like every CAT write.
func mwSetSpec() transport.CommandSpec { return transport.CATWriteSpec() }

// requestedFields is the set of spec.Fields a write of data asks for.
//
// UNLIKE yaesu.RequestedFields, spec.FieldTag is NOT unconditional: this
// radio has no tag route at all (NoTag), so an ordinary write — every
// read of this radio always leaves Tag "" — must not be refused for
// asking about a field it never touches. A NON-EMPTY tag IS requested,
// so a codeplug written for a tag-bearing radio is refused here rather
// than silently dropped. spec.FieldCTCSSTone IS unconditional: this
// radio's P9 is always on the wire (doc.go), so every write asks for it,
// and buildWriteCommand separately refuses when it is not Known (there is
// no "leave it alone" encoding for a byte that is always sent).
func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldClarifier,
		spec.FieldCTCSSState,
		spec.FieldCTCSSTone,
		spec.FieldShift,
	}
	if data.Tag != "" {
		fields = append(fields, spec.FieldTag)
	}
	if data.TagDisplay.State == codeplug.Known {
		fields = append(fields, spec.FieldTagDisplay)
	}
	if data.ScanSkip.State == codeplug.Known {
		fields = append(fields, spec.FieldScanSkip)
	}
	for _, t := range yaesu.TierRequestedFields {
		if t.Present(data) {
			fields = append(fields, t.Field)
		}
	}
	return fields
}

// toneIndexFor returns the standard chart's CAT tone number for v, and
// whether v is one of the chart's fifty entries — the write-direction
// mirror of read.go's toneForIndex.
func toneIndexFor(v spec.Tone) (uint8, bool) {
	for i, t := range spec.StandardCTCSSTones() {
		if t == v {
			return uint8(i), true
		}
	}
	return 0, false
}

// WriteChannel implements driver.Session: writes one memory channel as a
// single MW Set frame.
//
// EVERY REFUSAL HAPPENS BEFORE ANY BYTE GOES OUT: the slot must parse and
// belong to a bank this session supports, an empty channel is an erase
// this codec cannot express, the field states must be writable at all,
// and every field the write asks for must be write-Supported for this
// session's profile. Only then is the frame built, which can refuse again
// at the value level (buildWriteCommand).
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
			Reason: eraseReason,
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

	cmd, err := buildWriteCommand(s.dialect, ch)
	if err != nil {
		return res, err
	}

	res.Steps = []driver.WriteStep{{Command: "MW"}}
	const mwStep = 0

	if _, err := s.eng.Do(ctx, cmd, mwSetSpec()); err != nil {
		if errors.Is(err, cat.ErrRejected) {
			res.Steps[mwStep].Sent = true
			return res, fmt.Errorf("ftdx5000: WriteChannel %s: MW rejected by radio: %w", ch.Slot, err)
		}
		return res, fmt.Errorf("ftdx5000: WriteChannel %s: MW: %w", ch.Slot, err)
	}
	res.Steps[mwStep].Sent, res.Steps[mwStep].Confirmed = true, true

	return res, nil
}

// buildWriteCommand renders ch as this radio's MW Set frame, or refuses
// with a *driver.WriteRefusedError naming the field at fault.
//
// A PURE function — no session, no wire I/O — so every refusal can be
// exercised directly from a table test.
func buildWriteCommand(dialect cat.Dialect, ch codeplug.Channel) (cat.Command, error) {
	sl, err := dialect.ParseSlot(ch.Slot)
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{Slot: ch.Slot, Reason: err.Error()}
	}
	data := *ch.Data

	// NoTag: this radio's record has no field after P10 Shift at all
	// (matrix §2). A non-empty tag or a Known tag_display cannot be
	// expressed and must be refused rather than silently dropped.
	if data.Tag != "" {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldTag},
			Reason: "this radio's CAT command set has no channel-name/tag route at all (matrix §2/§3): a non-empty tag cannot be written",
		}
	}
	if data.TagDisplay.State == codeplug.Known {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldTagDisplay},
			Reason: "this radio's 27-byte record has no display flag (matrix §2): a Known tag_display cannot be written",
		}
	}

	mode, ok := dialect.ModeByName(data.Mode)
	if !ok {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldMode},
			Reason: fmt.Sprintf("mode %q is not a mode this radio supports", data.Mode),
		}
	}

	ctcss, ok := yaesu.CTCSSState(ctcssVocab, data.CTCSS)
	if !ok {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldCTCSSState},
			Reason: fmt.Sprintf("ctcss state %q is not one of %s", data.CTCSS, yaesu.CTCSSLegend(ctcssVocab)),
		}
	}
	shift, ok := yaesu.ShiftByName[data.Shift]
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

	// CTCSSTone is a LIVE field on this radio with no "leave it alone"
	// encoding (doc.go): every MW frame carries a P9 value, so a write
	// with no Known tone would either be refused wholesale or silently
	// write a fabricated index — this dialect refuses instead.
	if data.CTCSSTone.State != codeplug.Known {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldCTCSSTone},
			Reason: "this radio's P9 byte is a live CTCSS tone-table index with no \"leave it alone\" encoding (matrix §1.4/§2): every write must carry a Known tone",
		}
	}
	toneIdx, ok := toneIndexFor(data.CTCSSTone.Value)
	if !ok {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldCTCSSTone},
			Reason: fmt.Sprintf("tone %v is not in the standard 50-entry CTCSS chart this radio uses", data.CTCSSTone.Value),
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
		Kind:      cat.KindVFO, // matrix §2's write-fixed P7 byte
		CTCSS:     ctcss,
		ToneIndex: toneIdx,
		Shift:     shift,
	})
	if err != nil {
		return cat.Command{}, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Reason: fmt.Sprintf("cannot encode the MW Set frame: %v", err),
		}
	}
	return cmd, nil
}
