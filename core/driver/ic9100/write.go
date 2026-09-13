// SPDX-License-Identifier: GPL-3.0-or-later

package ic9100

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic9100 "github.com/gm5dna/open-rig-programmer/core/civ/ic9100"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// maxOffsetHz: matrix §1b "offset" — three BCD bytes, byte 3's high nibble
// (the 10 MHz digit) printed fixed at 0, giving 0-9.9999 MHz at 100 Hz
// resolution. civ.FieldSpan carries no numeric domain (this project's
// stated principle), so this ceiling is enforced here, defence in depth,
// exactly as core/driver/ic7100 does for its own identically-shaped span.
const maxOffsetHz = 9_999_900

func noSteps() driver.WriteResult { return driver.WriteResult{Steps: []driver.WriteStep{}} }

func refuse(slot string, fields []spec.Field, format string, args ...any) error {
	return &driver.WriteRefusedError{Slot: slot, Fields: fields, Reason: fmt.Sprintf(format, args...)}
}

var conditionalWriteFields = []struct {
	field spec.Field
	known func(codeplug.ChannelData) bool
}{
	{spec.FieldCTCSSTone, func(d codeplug.ChannelData) bool { return d.CTCSSTone.State == codeplug.Known }},
	{spec.FieldTagDisplay, func(d codeplug.ChannelData) bool { return d.TagDisplay.State == codeplug.Known }},
	{spec.FieldScanSkip, func(d codeplug.ChannelData) bool { return d.ScanSkip.State == codeplug.Known }},
	// FieldTxFrequency is checked here, deliberately, although this
	// record has no such span (matrix §1b): a caller supplying a Known
	// TxFreqHz must meet a proper capability refusal rather than have it
	// silently dropped — FieldSupport for it is the zero value, so the
	// capability-gate rung below refuses it by name.
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

func requestedFields(d codeplug.ChannelData) []spec.Field {
	fields := []spec.Field{spec.FieldFrequency, spec.FieldMode, spec.FieldTag}
	if d.ClarHz != 0 || d.RxClar || d.TxClar {
		fields = append(fields, spec.FieldClarifier)
	}
	if d.CTCSS != "" {
		fields = append(fields, spec.FieldCTCSSState)
	}
	if d.Shift != "" {
		fields = append(fields, spec.FieldShift)
	}
	for _, item := range conditionalWriteFields {
		if item.known(d) {
			fields = append(fields, item.field)
		}
	}
	return fields
}

// WriteChannel performs one read-dependent unmapped-region preservation
// check followed by exactly one acknowledged full-record set. The
// read+set pair is serialised as one operation; the engine alone
// serialises only individual exchanges.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if ch.Data == nil {
		return noSteps(), refuse(ch.Slot, []spec.Field{spec.FieldErase}, "this channel is empty, which would be an erase; erase is deliberately unshipped even with consent")
	}
	addr, bank, err := parseSlot(ch.Slot)
	if err != nil {
		return noSteps(), refuse(ch.Slot, nil, "%v", err)
	}
	data := *ch.Data

	// core/driver.CheckFieldStates is THE FLEET'S shared walk rather than
	// a table of this package's own: every field of ChannelData that
	// carries a FieldState, judged against this session's own
	// vocabularies. It exists because s.validateWriteValues below only
	// judges a field when its State == codeplug.Known — so a value
	// carried alongside Unknown/Unavailable/Absent (a caller who set a
	// value and forgot to set the state) would otherwise be DROPPED from
	// the frame and the write would report success, never refused (the
	// FT-891 closing review's C-M1, sharpened by MEDIUM-1 to
	// codeplug.Absent).
	if field, err := driver.CheckFieldStates(s.caps, data); err != nil {
		return noSteps(), refuse(ch.Slot, []spec.Field{field}, "%v", err)
	}

	requested := requestedFields(data)
	var blocked []spec.Field
	for _, field := range requested {
		if !s.caps.FieldSupport(bank, field).CanWrite() {
			blocked = append(blocked, field)
		}
	}
	if len(blocked) > 0 {
		return noSteps(), refuse(ch.Slot, blocked, "this session cannot write %s on bank %s", fieldList(blocked), bank)
	}
	if err := s.validateWriteValues(ch.Slot, data); err != nil {
		return noSteps(), err
	}

	answer, raw, present, err := s.readRawForWrite(ctx, ch.Slot, addr)
	if err != nil {
		return noSteps(), err
	}
	var stored civ.MemoryRecord
	if present {
		if err := s.templateGuard(ch.Slot, raw); err != nil {
			return noSteps(), err
		}
		stored, err = s.profile.ParseMemoryAnswer(answer)
		if err != nil {
			return noSteps(), refuse(ch.Slot, nil, "the stored record cannot be preserved safely: %v", err)
		}
	}

	record, err := mergeRecord(ch.Slot, addr, data, present, stored)
	if err != nil {
		return noSteps(), err
	}
	cmd, err := s.profile.BuildMemorySet(record)
	if err != nil {
		return noSteps(), fmt.Errorf("ic9100: WriteChannel %s: %w", ch.Slot, err)
	}

	result := driver.WriteResult{Steps: []driver.WriteStep{{Command: "1A 00"}}}
	// CIVWriteWithAckSpec declares ClassWriteWithAck and RetryReads zero.
	// A timeout is therefore quarantined and NEVER retransmitted.
	if _, err := s.eng.Do(ctx, cmd, civ.CIVWriteWithAckSpec(s.profile.AcknowledgementMatcher())); err != nil {
		if errors.Is(err, transport.ErrRejected) {
			result.Steps[0].Sent = true
			return result, fmt.Errorf("ic9100: WriteChannel %s: radio refused the memory set with FA: %w", ch.Slot, err)
		}
		return result, fmt.Errorf("ic9100: WriteChannel %s: write fate unattributable; set was not retransmitted: %w", ch.Slot, err)
	}
	result.Steps[0].Sent = true
	result.Steps[0].Confirmed = true
	return result, nil
}

func (s *Session) readRawForWrite(ctx context.Context, slot string, want civ.ChannelAddress) ([]byte, []byte, bool, error) {
	cmd, err := s.profile.BuildMemoryRead(want)
	if err != nil {
		return nil, nil, false, err
	}
	answer, err := s.eng.Do(ctx, cmd, civ.CIVReadSpec(s.profile.MemoryAnswerMatcher(), retryReads))
	if errors.Is(err, transport.ErrRejected) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, fmt.Errorf("ic9100: WriteChannel %s preservation read: %w", slot, err)
	}
	got, raw, err := s.profile.MemoryAnswerRecord(answer)
	if err != nil {
		return nil, nil, false, err
	}
	if got != want {
		s.noteMismatch()
		return nil, nil, false, &AnswerMismatchError{Model: "ic9100", Requested: want, Answered: got}
	}
	if allFF(raw) {
		return answer, raw, false, nil
	}
	return answer, raw, true, nil
}

// unmappedRegions is every record byte no spec.Field span covers (doc.go):
// the two D-STAR squelch bytes and the three 8-byte call signs.
var unmappedRegions = []struct {
	name     string
	from, to int
}{
	{"digital squelch", civic9100.DigitalSquelchOffset, civic9100.DigitalSquelchOffset + 1},
	{"digital code squelch", civic9100.DigitalCodeSquelchOffset, civic9100.DigitalCodeSquelchOffset + 1},
	{"D-STAR destination call sign", civic9100.DestCallOffset, civic9100.DestCallOffset + 8},
	{"D-STAR R1 call sign", civic9100.R1CallOffset, civic9100.R1CallOffset + 8},
	{"D-STAR R2 call sign", civic9100.R2CallOffset, civic9100.R2CallOffset + 8},
}

func (s *Session) templateGuard(slot string, raw []byte) error {
	layout, ok := s.profile.LayoutFor(civic9100.RecordLength)
	if !ok || len(raw) != len(layout.Fixed) {
		return fmt.Errorf("ic9100: WriteChannel %s: no matching fixed template", slot)
	}
	// Record byte 0's low nibble is the mapped select state, but its high
	// nibble is the unmapped split flag (matrix §3.11 term 3, "split and
	// select memory settings"). Preserve-or-refuse, mirroring IC-7100's
	// identical byte.
	if raw[0]&0xF0 != layout.Fixed[0]&0xF0 {
		return refuse(slot, nil, "split flag differs from the assumed template at record byte 0 (%#02x != %#02x); writing would silently clear Split ON", raw[0]&0xF0, layout.Fixed[0]&0xF0)
	}
	for _, region := range unmappedRegions {
		for offset := region.from; offset < region.to; offset++ {
			if raw[offset] != layout.Fixed[offset] {
				return refuse(slot, nil, "%s bytes differ from the assumed template at record byte %d (%#02x != %#02x)", region.name, offset, raw[offset], layout.Fixed[offset])
			}
		}
	}
	return nil
}

func mergeRecord(slot string, addr civ.ChannelAddress, data codeplug.ChannelData, modify bool, prior civ.MemoryRecord) (civ.MemoryRecord, error) {
	record := civ.MemoryRecord{Address: addr, Select: civ.Available("OFF"), RXFreqHz: civ.Available(data.FreqHz), Mode: civ.Available(data.Mode), Name: civ.Available(data.Tag)}
	var missing []spec.Field
	tone := func(field spec.Field, incoming codeplug.ToneField, old civ.Optional[uint64]) civ.Optional[uint64] {
		if incoming.State == codeplug.Known {
			return civ.Available(uint64(incoming.Value))
		}
		if modify {
			return old
		}
		missing = append(missing, field)
		return civ.Optional[uint64]{}
	}
	number := func(field spec.Field, incoming codeplug.FreqField, old civ.Optional[uint64]) civ.Optional[uint64] {
		if incoming.State == codeplug.Known {
			return civ.Available(incoming.Value)
		}
		if modify {
			return old
		}
		missing = append(missing, field)
		return civ.Optional[uint64]{}
	}
	text := func(field spec.Field, incoming codeplug.StringField, old civ.Optional[string]) civ.Optional[string] {
		if incoming.State == codeplug.Known {
			return civ.Available(incoming.Value)
		}
		if modify {
			return old
		}
		missing = append(missing, field)
		return civ.Optional[string]{}
	}
	record.OffsetHz = number(spec.FieldOffset, data.OffsetHz, prior.OffsetHz)
	record.Duplex = text(spec.FieldDuplex, data.Duplex, prior.Duplex)
	record.ToneMode = text(spec.FieldToneMode, data.ToneMode, prior.ToneMode)
	record.ToneTXDeciHz = tone(spec.FieldToneTx, data.ToneTx, prior.ToneTXDeciHz)
	record.ToneRXDeciHz = tone(spec.FieldToneRx, data.ToneRx, prior.ToneRXDeciHz)
	record.DTCSPolarity = text(spec.FieldDTCSPolarity, data.DTCSPolarity, prior.DTCSPolarity)
	record.Filter = text(spec.FieldFilter, data.Filter, prior.Filter)
	if data.DTCSCode.State == codeplug.Known {
		record.DTCSCode = civ.Available(uint64(data.DTCSCode.Value))
	} else if modify {
		record.DTCSCode = prior.DTCSCode
	} else {
		missing = append(missing, spec.FieldDTCSCode)
	}
	if data.DataMode.State == codeplug.Known {
		value := "OFF"
		if data.DataMode.Value {
			value = "ON"
		}
		record.DataMode = civ.Available(value)
	} else if modify {
		record.DataMode = prior.DataMode
	} else {
		missing = append(missing, spec.FieldDataMode)
	}
	if len(missing) > 0 {
		return civ.MemoryRecord{}, refuse(slot, missing, "empty slot has no values to preserve; create needs %s", fieldList(missing))
	}
	return record, nil
}

func (s *Session) validateWriteValues(slot string, data codeplug.ChannelData) error {
	if !slices.Contains(s.caps.Modes, data.Mode) {
		return refuse(slot, []spec.Field{spec.FieldMode}, "mode %q is not in %v", data.Mode, s.caps.Modes)
	}
	if data.FreqHz < s.caps.MinFreqHz || data.FreqHz > s.caps.MaxFreqHz {
		return refuse(slot, []spec.Field{spec.FieldFrequency}, "frequency %d is outside %d..%d Hz", data.FreqHz, s.caps.MinFreqHz, s.caps.MaxFreqHz)
	}
	if data.OffsetHz.State == codeplug.Known && data.OffsetHz.Value > maxOffsetHz {
		return refuse(slot, []spec.Field{spec.FieldOffset}, "offset %d Hz exceeds the documented maximum %d Hz", data.OffsetHz.Value, uint64(maxOffsetHz))
	}
	if len(data.Tag) > s.caps.TagLen {
		return refuse(slot, []spec.Field{spec.FieldTag}, "tag is %d bytes; maximum is %d", len(data.Tag), s.caps.TagLen)
	}
	for i := range len(data.Tag) {
		if !s.caps.TagByteOK(data.Tag[i]) {
			return refuse(slot, []spec.Field{spec.FieldTag}, "tag byte %#02x at %d is outside the matrix §1 row 30 charset", data.Tag[i], i)
		}
	}
	for _, item := range []struct {
		field spec.Field
		value codeplug.StringField
		vocab []string
	}{
		{spec.FieldDuplex, data.Duplex, duplexValues(s.caps)}, {spec.FieldToneMode, data.ToneMode, toneModeValues(s.caps)}, {spec.FieldDTCSPolarity, data.DTCSPolarity, s.caps.DTCSPolarities}, {spec.FieldFilter, data.Filter, s.caps.Filters},
	} {
		if item.value.State == codeplug.Known {
			if err := item.value.Valid(item.vocab); err != nil {
				return refuse(slot, []spec.Field{item.field}, "%v", err)
			}
		}
	}
	for _, item := range []struct {
		field spec.Field
		value codeplug.ToneField
	}{{spec.FieldToneTx, data.ToneTx}, {spec.FieldToneRx, data.ToneRx}} {
		if item.value.State == codeplug.Known {
			if err := item.value.Valid(s.caps); err != nil {
				return refuse(slot, []spec.Field{item.field}, "%v", err)
			}
		}
	}
	if data.DTCSCode.State == codeplug.Known {
		if err := data.DTCSCode.Valid(s.caps.DTCSCodes); err != nil {
			return refuse(slot, []spec.Field{spec.FieldDTCSCode}, "%v", err)
		}
	}
	return nil
}

func fieldList(fields []spec.Field) string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = string(f)
	}
	return strings.Join(names, ", ")
}
