// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civicr8600 "github.com/gm5dna/open-rig-programmer/core/civ/icr8600"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// unconditionalFields is this model's matrix section 2 supported set for
// ChannelData's three plain scalar fields. The Yaesu clarifier/CTCSS/shift
// trio is deliberately absent: all three are matrix zeros on the IC-R8600.
var unconditionalFields = []spec.Field{
	spec.FieldFrequency,
	spec.FieldMode,
	spec.FieldTag,
}

// tierRequestedFields follows ChannelData declaration order. Unsupported
// fields remain here when a caller supplies a Known/non-zero request so the
// capability gate refuses them by name instead of silently dropping them.
var tierRequestedFields = []struct {
	field   spec.Field
	present func(codeplug.ChannelData) bool
}{
	{spec.FieldClarifier, func(d codeplug.ChannelData) bool { return d.ClarHz != 0 || d.RxClar || d.TxClar }},
	{spec.FieldCTCSSState, func(d codeplug.ChannelData) bool { return d.CTCSS != "" }},
	{spec.FieldCTCSSTone, func(d codeplug.ChannelData) bool { return d.CTCSSTone.State == codeplug.Known }},
	{spec.FieldShift, func(d codeplug.ChannelData) bool { return d.Shift != "" }},
	{spec.FieldTagDisplay, func(d codeplug.ChannelData) bool { return d.TagDisplay.State == codeplug.Known }},
	{spec.FieldScanSkip, func(d codeplug.ChannelData) bool { return d.ScanSkip.State == codeplug.Known }},
	{spec.FieldTxFrequency, func(d codeplug.ChannelData) bool { return d.TxFreqHz.State == codeplug.Known }},
	{spec.FieldDuplex, func(d codeplug.ChannelData) bool { return d.Duplex.State == codeplug.Known }},
	{spec.FieldOffset, func(d codeplug.ChannelData) bool { return d.OffsetHz.State == codeplug.Known }},
	{spec.FieldToneMode, func(d codeplug.ChannelData) bool { return d.ToneMode.State == codeplug.Known }},
	{spec.FieldToneTx, func(d codeplug.ChannelData) bool { return d.ToneTx.State == codeplug.Known }},
	{spec.FieldToneRx, func(d codeplug.ChannelData) bool { return d.ToneRx.State == codeplug.Known }},
	{spec.FieldDTCSCode, func(d codeplug.ChannelData) bool { return d.DTCSCode.State == codeplug.Known }},
	{spec.FieldDTCSPolarity, func(d codeplug.ChannelData) bool { return d.DTCSPolarity.State == codeplug.Known }},
	{spec.FieldFilter, func(d codeplug.ChannelData) bool { return d.Filter.State == codeplug.Known }},
	{spec.FieldDataMode, func(d codeplug.ChannelData) bool { return d.DataMode.State == codeplug.Known }},
	{spec.FieldTuningStepEnabled, func(d codeplug.ChannelData) bool { return d.TuningStepEnabled.State == codeplug.Known }},
	{spec.FieldTuningStep, func(d codeplug.ChannelData) bool { return d.TuningStep.State == codeplug.Known }},
	{spec.FieldProgramTuningStep, func(d codeplug.ChannelData) bool { return d.ProgramTuningStepHz.State == codeplug.Known }},
	{spec.FieldAttenuator, func(d codeplug.ChannelData) bool { return d.AttenuatorDB.State == codeplug.Known }},
	{spec.FieldPreamp, func(d codeplug.ChannelData) bool { return d.Preamp.State == codeplug.Known }},
	{spec.FieldAntenna, func(d codeplug.ChannelData) bool { return d.Antenna.State == codeplug.Known }},
	{spec.FieldIPPlus, func(d codeplug.ChannelData) bool { return d.IPPlus.State == codeplug.Known }},
}

func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := append([]spec.Field(nil), unconditionalFields...)
	for _, entry := range tierRequestedFields {
		if entry.present(data) {
			fields = append(fields, entry.field)
		}
	}
	return fields
}

var mandatoryKnownFields = []struct {
	field spec.Field
	known func(codeplug.ChannelData) bool
}{
	{spec.FieldDuplex, func(d codeplug.ChannelData) bool { return d.Duplex.State == codeplug.Known }},
	{spec.FieldOffset, func(d codeplug.ChannelData) bool { return d.OffsetHz.State == codeplug.Known }},
	{spec.FieldFilter, func(d codeplug.ChannelData) bool { return d.Filter.State == codeplug.Known }},
	{spec.FieldTuningStepEnabled, func(d codeplug.ChannelData) bool { return d.TuningStepEnabled.State == codeplug.Known }},
	{spec.FieldTuningStep, func(d codeplug.ChannelData) bool { return d.TuningStep.State == codeplug.Known }},
	{spec.FieldProgramTuningStep, func(d codeplug.ChannelData) bool { return d.ProgramTuningStepHz.State == codeplug.Known }},
	{spec.FieldAttenuator, func(d codeplug.ChannelData) bool { return d.AttenuatorDB.State == codeplug.Known }},
	{spec.FieldPreamp, func(d codeplug.ChannelData) bool { return d.Preamp.State == codeplug.Known }},
	{spec.FieldAntenna, func(d codeplug.ChannelData) bool { return d.Antenna.State == codeplug.Known }},
	{spec.FieldIPPlus, func(d codeplug.ChannelData) bool { return d.IPPlus.State == codeplug.Known }},
}

// unmappedHighNibble is one byte whose profile span maps only the low
// nibble, together with what the guide actually PRINTS in the half a
// rebuilt record would overwrite with zero.
//
// NONE OF IT IS ASSUMED, and the refusal must not say so. Byte 0's high
// nibble is a three-valued printed enum; bytes 8, 18, 19, 20 and FM 37
// each print "0 (Fixed)"; FM 41 sits inside the DTCS span, against which
// the guide prints no value at all. What an operator must do at the radio
// differs in each case, so the refusal names which it is. Corrected under
// review finding F4 and pinned by
// TestE6_TheHighNibbleRefusalNamesWhatEachNibbleActuallyIs.
type unmappedHighNibble struct {
	offset  int
	printed string
}

// commonUnmappedHighNibbles are the five common fields — SELECT, duplex,
// preamp, antenna, IP+ — whose span maps only the low nibble. E6 requires
// identity with zero before a full-record write may rebuild them.
var commonUnmappedHighNibbles = []unmappedHighNibble{
	{0, "the printed scan-skip enum 0=SKIP OFF / 1=SKIP / 2=PSKIP (matrix section 2 row 9), which this profile maps nowhere"},
	{8, "printed 0 (Fixed)"},
	{18, "printed 0 (Fixed)"},
	{19, "printed 0 (Fixed)"},
	{20, "printed 0 (Fixed)"},
}

// fmUnmappedHighNibbles are the two the FM tail adds. The mapped tone-mode
// and receive-polarity enums also occupy only their low nibble.
var fmUnmappedHighNibbles = []unmappedHighNibble{
	{37, "printed 0 (Fixed)"},
	{41, "an unlabelled half of the printed DTCS code span, whose value the guide states nowhere"},
}

// digitalTailOffset is where every mode-keyed tail begins: the 37 common
// head bytes are followed by the class's own bytes, or by nothing at all.
const digitalTailOffset = 37

// The offset field's evidenced domain, matrix 3.15.7: eight nibble leaders
// of which the 100 Hz and 1 GHz digits are printed "0 (Fixed)" and the
// 100 MHz digit runs 0~2. The four-byte BCD span is wider than this, so the
// bound is enforced rather than inferred from the field width.
const (
	offsetCeilingHz    = 299_999_000
	offsetResolutionHz = 1_000
)

// layoutForMode returns the layout a write of this neutral mode would build.
// Length cannot choose — FM and DCR are both 44 bytes — so the mode span's
// enum is what discriminates, exactly as the profile's own mode key does.
func layoutForMode(p civ.Profile, mode string) (civ.RecordLayout, bool) {
	for _, layout := range p.Layouts() {
		for _, span := range layout.Fields {
			if span.Field != civ.FieldMode {
				continue
			}
			for _, name := range span.Enum {
				if name == mode {
					return layout, true
				}
			}
		}
	}
	return civ.RecordLayout{}, false
}

func (s *Session) memorySetSpec() transport.CommandSpec {
	sp := civ.CIVWriteWithAckSpec(s.profile.AcknowledgementMatcher())
	sp.Timeout, sp.Settle = s.readTimeout, s.settle
	return sp
}

func (s *Session) bankFor(slot string) (spec.BankID, bool) {
	for _, bank := range s.caps.Banks {
		if bank.WithinSpace(slot) {
			return bank.ID, true
		}
	}
	return "", false
}

// occupiedSurprise refuses a write when the preservation read finds a channel
// that the session's discovery walk never materialised. The bounded walk can
// miss such a channel whenever a later group's channel 00 is empty; allowing
// the write would overwrite a channel absent from the codeplug. Pinned by
// TestEndToEnd_WriteToAnOccupiedSlotTheBoundedWalkMissedIsRefused.
//
// UNLIKE THE IC-905'S SAME-NAMED RUNG, this one asks only whether discovery
// listed the slot — it has no knownOccupied set a confirmed write can grow.
// That is safe only because WriteChannel's create rung (above this one)
// refuses every write to an empty slot unconditionally: no session can ever
// add a slot this inventory does not already list, so the inventory never
// needs to grow. If a later change ever lets the R8600 create a channel
// (an honest SELECT default), this rung must grow an occupied-this-session
// set too, or it will wrongly refuse the second write to a slot the same
// session just created.
//
// THE DIAGNOSTIC IS TAILORED TO THE WALK THIS SESSION ACTUALLY RAN (closing
// review). One sentence cannot be true of both walks, and the wrong one
// sends the user looking for the wrong thing:
//
//   - After the BOUNDED walk the slot may simply lie outside its reach, and
//     re-discovery is worth trying, because that reach depends on which
//     channel 00s answer.
//   - After a FULL walk it cannot. Every one of the 10,000 addresses was
//     read, so a record here means the channel arrived AFTER this session
//     opened — at the front panel, or from another controller — and the
//     user needs to be told that rather than to re-run a walk that already
//     covered everything.
//
// AND THE OLD SENTENCE OVERSTATED THE REST. "This build offers no setting
// that widens it" is true of the command line and the window, which expose
// nothing, but this package does export WithFullInventoryWalk
// (icr8600.go:34) and internal/wiring's row simply does not pass it. Saying
// which of those is meant costs one clause and stops the statement being
// false to anyone who reads the package. The IC-905's same-named rung
// already refuses to name a remedy the user cannot reach (ic905/write.go,
// "NO REMEDY IS NAMED THAT THE USER CANNOT REACH"); this keeps that rule
// and removes the over-claim it left behind.
func (s *Session) occupiedSurprise(slot string, readReturnedRecord bool) error {
	if !readReturnedRecord {
		return nil
	}
	mem, ok := s.caps.Bank(spec.BankMemory)
	if ok && slices.Contains(mem.Slots, slot) {
		return nil
	}
	const surprise = "this session's inventory does not list this slot, but the receiver answered the pre-write read with a record: the discovery walk never saw it, so writing would overwrite a channel nothing has read. "
	reason := surprise + "This session ran the BOUNDED walk — group 0 in full, then channel 00 of every other group and the rest of a group whose 00 answered — so a channel stored outside it is never listed. Re-discover the receiver; if the slot is still not listed it lies outside that walk, and nothing on this build's command line or in its window widens it (the driver's own WithFullInventoryWalk is a Go-level option no registered composition passes)"
	if s.report.InventoryComplete {
		reason = surprise + "This session read the WHOLE 100x100 memory space, so the channel was stored AFTER this session opened — at the receiver's front panel, or by another controller. Re-discover the receiver before writing to it"
	}
	return &driver.WriteRefusedError{Slot: slot, Reason: reason}
}

// WriteChannel performs one read-modify-write transaction and one
// address-matched acknowledged 1A 00 set. All locally decidable refusals
// precede the preservation read; E6 is necessarily read-dependent.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	res := driver.WriteResult{Steps: []driver.WriteStep{}}
	addr, err := slotAddress(ch.Slot)
	if err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: fmt.Sprintf("not a valid IC-R8600 memory slot: %v", err)}
	}
	bank, ok := s.bankFor(ch.Slot)
	if !ok {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: "slot is not part of a bank this session supports"}
	}
	if ch.Empty() {
		return res, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: []spec.Field{spec.FieldErase},
			Reason: "this tier ships no erase path: FieldErase is deliberately zero and the documented 1A 00 clear form is not admitted by the outbound gate",
		}
	}
	data := *ch.Data

	if err := s.validateWriteFields(ch.Slot, data); err != nil {
		return res, err
	}

	var notKnown []spec.Field
	for _, mandatory := range mandatoryKnownFields {
		if !mandatory.known(data) {
			notKnown = append(notKnown, mandatory.field)
		}
	}
	if len(notKnown) > 0 {
		return res, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: notKnown,
			Reason: "a 1A 00 set carries the whole record: every common mapped field must be Known, never synthesised",
		}
	}

	var unwritable []spec.Field
	for _, field := range requestedFields(data) {
		support := s.caps.FieldSupport(bank, field)
		if !support.CanWrite() && support.Write != spec.Inert {
			unwritable = append(unwritable, field)
		}
	}
	if len(unwritable) > 0 {
		return res, &driver.WriteRefusedError{
			Slot: ch.Slot, Fields: unwritable,
			Reason: "not write-Supported for this session (the record cannot express the field, or unverified writes have not been consented)",
		}
	}

	if data.Mode != "FM" {
		var fmOnly []spec.Field
		for _, entry := range []struct {
			field spec.Field
			known bool
		}{
			{spec.FieldToneMode, data.ToneMode.State == codeplug.Known},
			{spec.FieldToneRx, data.ToneRx.State == codeplug.Known},
			{spec.FieldDTCSCode, data.DTCSCode.State == codeplug.Known},
			{spec.FieldDTCSPolarity, data.DTCSPolarity.State == codeplug.Known},
		} {
			if entry.known {
				fmOnly = append(fmOnly, entry.field)
			}
		}
		if len(fmOnly) > 0 {
			return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: fmOnly, Reason: "these receive-squelch fields exist only in this profile's FM record layout"}
		}
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	stored, present, err := s.recordAt(ctx, addr)
	if err != nil {
		return res, fmt.Errorf("icr8600: WriteChannel %s: preservation read: %w", ch.Slot, err)
	}
	occupied := present && !isEmptyRecord(stored)
	if !occupied {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Reason: "the slot is empty and its SELECT group has no honest source: the tier forbids mapping SELECT as scan_skip, so a create is refused rather than inventing OFF",
		}
	}

	layout, err := s.profile.LayoutForRecord(stored)
	if err != nil {
		return res, fmt.Errorf("icr8600: WriteChannel %s: preservation record: %w", ch.Slot, err)
	}
	unmappedHighNibbles := commonUnmappedHighNibbles
	if layout.ModeClass == "FM" {
		unmappedHighNibbles = append(append([]unmappedHighNibble(nil), unmappedHighNibbles...), fmUnmappedHighNibbles...)
	}
	for _, nibble := range unmappedHighNibbles {
		if stored[nibble.offset]&0xF0 == 0 {
			continue
		}
		return res, &driver.WriteRefusedError{
			Slot: ch.Slot,
			Reason: fmt.Sprintf(
				"record byte %d has unmapped high nibble %#02x rather than the zero a rebuilt record would write; that nibble is %s, so writing would silently replace it (E6)",
				nibble.offset, stored[nibble.offset]&0xF0, nibble.printed,
			),
		}
	}
	if layout.ModeClass != "NONE" && layout.ModeClass != "FM" {
		if len(layout.Fixed) != len(stored) || !bytes.Equal(stored[37:], layout.Fixed[37:]) {
			return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: civicr8600.DigitalTailRefusalReason}
		}
	}
	if err := s.occupiedSurprise(ch.Slot, occupied); err != nil {
		return res, err
	}

	prior, err := s.parseRecord(addr, stored)
	if err != nil {
		return res, fmt.Errorf("icr8600: WriteChannel %s: preservation record: %w", ch.Slot, err)
	}
	if data.Mode == "FM" {
		if mode, _ := prior.Mode.Get(); mode != "FM" && !fmTailKnown(data) {
			return res, &driver.WriteRefusedError{
				Slot:   ch.Slot,
				Fields: missingFMTailFields(data),
				Reason: "the target FM record needs explicit receive-squelch values because the stored non-FM record has no FM tail to preserve",
			}
		}
	}
	// The same refusal for the digital classes, whose tails no neutral
	// field carries at all. E6 above compares the STORED record with the
	// STORED layout's template, so it says nothing about a mode change:
	// when the target class differs, every one of its tail bytes would
	// come from the assumed icr8600-tail-templates entry rather than from
	// anything read back. Pinned by
	// TestEndToEnd_AModeChangeIntoADigitalClassIsRefusedRatherThanInventTheTail.
	if target, ok := layoutForMode(s.profile, data.Mode); ok && len(target.Fixed) > 0 && target.ModeClass != layout.ModeClass {
		return res, &driver.WriteRefusedError{
			Slot: ch.Slot,
			Reason: fmt.Sprintf(
				"the stored %s record has no %s tail to preserve: record bytes %d-%d would be invented from the assumed icr8600-tail-templates entry (% X) — set the digital squelch at the radio instead",
				layout.ModeClass, target.ModeClass, digitalTailOffset, len(target.Fixed)-1, target.Fixed[digitalTailOffset:],
			),
		}
	}

	record := recordForWrite(addr, data, prior)
	cmd, err := s.profile.BuildMemorySet(record)
	if err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: err.Error()}
	}
	res.Steps = []driver.WriteStep{{Command: "1A 00"}}
	if _, err := s.eng.Do(ctx, cmd, s.memorySetSpec()); err != nil {
		if errors.Is(err, transport.ErrRejected) {
			res.Steps[0].Sent = true
			return res, fmt.Errorf("icr8600: WriteChannel %s: the radio rejected the memory set: %w", ch.Slot, err)
		}
		if errors.Is(err, transport.ErrTimeout) {
			return res, fmt.Errorf("icr8600: WriteChannel %s: the set was transmitted and never acknowledged, so its outcome is UNATTRIBUTABLE and it will not be resent: %w", ch.Slot, err)
		}
		return res, fmt.Errorf("icr8600: WriteChannel %s: memory set: %w", ch.Slot, err)
	}
	res.Steps[0].Sent, res.Steps[0].Confirmed = true, true
	return res, nil
}

func (s *Session) validateWriteFields(slot string, d codeplug.ChannelData) error {
	refuse := func(field spec.Field, err error) error {
		return &driver.WriteRefusedError{Slot: slot, Fields: []spec.Field{field}, Reason: err.Error()}
	}
	if d.FreqHz > 9_999_999_999 {
		return refuse(spec.FieldFrequency, fmt.Errorf("%d Hz exceeds the five-byte packed-BCD frequency field", d.FreqHz))
	}
	if !contains(s.caps.Modes, d.Mode) {
		return refuse(spec.FieldMode, fmt.Errorf("%q is not one of this radio's modes", d.Mode))
	}
	if len(d.Tag) > s.caps.TagLen {
		return refuse(spec.FieldTag, fmt.Errorf("tag is %d bytes; this radio stores at most %d", len(d.Tag), s.caps.TagLen))
	}
	for i := range len(d.Tag) {
		if !s.caps.TagByteOK(d.Tag[i]) {
			return refuse(spec.FieldTag, fmt.Errorf("tag byte %#02x at offset %d is outside this radio's charset", d.Tag[i], i))
		}
	}

	duplex := make([]string, len(s.caps.DuplexOptions))
	for i, option := range s.caps.DuplexOptions {
		duplex[i] = option.Value
	}
	toneModes := make([]string, len(s.caps.ToneModes))
	for i, mode := range s.caps.ToneModes {
		toneModes[i] = mode.Value
	}
	checks := []struct {
		field spec.Field
		err   error
	}{
		{spec.FieldCTCSSTone, d.CTCSSTone.Valid(s.caps)},
		{spec.FieldTagDisplay, d.TagDisplay.Valid()},
		{spec.FieldScanSkip, d.ScanSkip.Valid()},
		{spec.FieldTxFrequency, d.TxFreqHz.Valid()},
		{spec.FieldDuplex, d.Duplex.Valid(duplex)},
		{spec.FieldOffset, d.OffsetHz.Valid()},
		{spec.FieldToneMode, d.ToneMode.Valid(toneModes)},
		{spec.FieldToneTx, d.ToneTx.Valid(s.caps)},
		{spec.FieldToneRx, d.ToneRx.Valid(s.caps)},
		{spec.FieldDTCSCode, d.DTCSCode.Valid(s.caps.DTCSCodes)},
		{spec.FieldDTCSPolarity, d.DTCSPolarity.Valid(s.caps.DTCSPolarities)},
		{spec.FieldFilter, d.Filter.Valid(s.caps.Filters)},
		{spec.FieldDataMode, d.DataMode.Valid()},
		{spec.FieldTuningStepEnabled, d.TuningStepEnabled.Valid()},
		{spec.FieldTuningStep, d.TuningStep.Valid(s.caps.TuningSteps)},
		{spec.FieldProgramTuningStep, d.ProgramTuningStepHz.Valid()},
		{spec.FieldAttenuator, d.AttenuatorDB.Valid(s.caps.AttenuatorDB)},
		{spec.FieldPreamp, d.Preamp.Valid(s.caps.PreampOptions)},
		{spec.FieldAntenna, d.Antenna.Valid(s.caps.AntennaOptions)},
		{spec.FieldIPPlus, d.IPPlus.Valid()},
	}
	for _, check := range checks {
		if check.err != nil {
			return refuse(check.field, check.err)
		}
	}
	if d.OffsetHz.Value > 9_999_999_900 || d.OffsetHz.Value%100 != 0 {
		return refuse(spec.FieldOffset, fmt.Errorf("%d Hz is outside the four-byte packed-BCD offset field at 100 Hz resolution", d.OffsetHz.Value))
	}
	// The field is wider than its evidenced domain. Matrix 3.15.7 draws
	// the four bytes as eight nibble leaders — [1 kHz | 100 Hz (0 Fixed)],
	// [100 kHz | 10 kHz], [10 MHz | 1 MHz], [1 GHz (0 Fixed) | 100 MHz
	// (0~2)] — so the resolution is 1 kHz and the ceiling 299,999,000 Hz.
	// The two "0 (Fixed)" nibbles have no other gate: unlike bytes 8, 18,
	// 19, 20 and FM 41, they are packed into one BCD number rather than
	// left unmapped, so only this bound stops a caller writing a digit
	// into them. Pinned by
	// TestWrite_OffsetOutsideThePrintedDomainIsRefusedBeforeTheWire.
	if d.OffsetHz.Value > offsetCeilingHz || d.OffsetHz.Value%offsetResolutionHz != 0 {
		return refuse(spec.FieldOffset, fmt.Errorf("%d Hz is outside the offset domain printed at matrix 3.15.7: 0 to %d Hz in %d Hz steps, with the 100 Hz and 1 GHz digits fixed at zero", d.OffsetHz.Value, uint64(offsetCeilingHz), uint64(offsetResolutionHz)))
	}
	if d.ProgramTuningStepHz.State == codeplug.Known {
		r := s.caps.ProgramTuningStepRange
		if r == nil || d.ProgramTuningStepHz.Value < r.MinHz || d.ProgramTuningStepHz.Value > r.MaxHz || d.ProgramTuningStepHz.Value%r.ResolutionHz != 0 {
			return refuse(spec.FieldProgramTuningStep, fmt.Errorf("%d Hz is outside this radio's programmable-step range", d.ProgramTuningStepHz.Value))
		}
	}
	return nil
}

func fmTailKnown(d codeplug.ChannelData) bool {
	return d.ToneMode.State == codeplug.Known && d.ToneRx.State == codeplug.Known &&
		d.DTCSCode.State == codeplug.Known && d.DTCSPolarity.State == codeplug.Known
}

func missingFMTailFields(d codeplug.ChannelData) []spec.Field {
	var fields []spec.Field
	if d.ToneMode.State != codeplug.Known {
		fields = append(fields, spec.FieldToneMode)
	}
	if d.ToneRx.State != codeplug.Known {
		fields = append(fields, spec.FieldToneRx)
	}
	if d.DTCSCode.State != codeplug.Known {
		fields = append(fields, spec.FieldDTCSCode)
	}
	if d.DTCSPolarity.State != codeplug.Known {
		fields = append(fields, spec.FieldDTCSPolarity)
	}
	return fields
}

func recordForWrite(addr civ.ChannelAddress, d codeplug.ChannelData, prior civ.MemoryRecord) civ.MemoryRecord {
	selectValue, _ := prior.Select.Get()
	record := civ.MemoryRecord{
		Address: addr, Select: civ.Available(selectValue),
		RXFreqHz: civ.Available(d.FreqHz), Mode: civ.Available(d.Mode),
		Filter: civ.Available(d.Filter.Value), Duplex: civ.Available(d.Duplex.Value),
		OffsetHz:            civ.Available(d.OffsetHz.Value),
		TuningStepEnabled:   civ.Available(onOff(d.TuningStepEnabled.Value)),
		TuningStep:          civ.Available(d.TuningStep.Value),
		ProgramTuningStepHz: civ.Available(d.ProgramTuningStepHz.Value),
		AttenuatorDB:        civ.Available(uint64(d.AttenuatorDB.Value)),
		Preamp:              civ.Available(d.Preamp.Value), Antenna: civ.Available(d.Antenna.Value),
		IPPlus: civ.Available(onOff(d.IPPlus.Value)), Name: civ.Available(d.Tag),
	}
	if d.Mode != "FM" {
		return record
	}
	if d.ToneMode.State == codeplug.Known {
		record.ToneMode = civ.Available(d.ToneMode.Value)
	} else {
		record.ToneMode = prior.ToneMode
	}
	if d.ToneRx.State == codeplug.Known {
		record.ToneRXDeciHz = civ.Available(uint64(d.ToneRx.Value))
	} else {
		record.ToneRXDeciHz = prior.ToneRXDeciHz
	}
	if d.DTCSCode.State == codeplug.Known {
		record.DTCSCode = civ.Available(uint64(d.DTCSCode.Value))
	} else {
		record.DTCSCode = prior.DTCSCode
	}
	if d.DTCSPolarity.State == codeplug.Known {
		record.DTCSPolarity = civ.Available(d.DTCSPolarity.Value)
	} else {
		record.DTCSPolarity = prior.DTCSPolarity
	}
	return record
}

func onOff(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
