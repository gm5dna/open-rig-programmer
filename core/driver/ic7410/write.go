// SPDX-License-Identifier: GPL-3.0-or-later

package ic7410

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7410 "github.com/gm5dna/open-rig-programmer/core/civ/ic7410"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// memorySetStep is the mnemonic this driver reports for its one write
// frame.
const memorySetStep = "1A 00"

// ErrUnmappedRegion is the sentinel for the E6-style refusal: the slot's
// unmapped record regions differ from the profile's Fixed template.
var ErrUnmappedRegion = errors.New("ic7410: the slot's unmapped record regions differ from this profile's Fixed template")

// UnmappedRegionError names the unmapped record region that disagreed.
//
// A driver may write a slot ONLY when its unmapped regions equal the
// profile's Fixed template; anything else is REFUSED, never rewritten.
// THE COST: a channel whose Select-memory/Split byte is non-zero, or whose
// TX-duplicate-block "still necessary" bytes carry real content from a
// real radio's own front panel, cannot be written by this programme at
// all — it is never silently cleared and never silently overwritten.
type UnmappedRegionError struct {
	// Offset is the 0-based record byte whose unmapped region differed.
	Offset int
	// Want and Got are that byte's value in the template and in the
	// slot's actual record.
	Want, Got byte
}

func (e *UnmappedRegionError) Error() string {
	what := "an unmapped region"
	switch {
	case e.Offset == civic7410.SelectSplitOffset:
		what = "the Select-memory + Split byte (matrix §1b, printed ③)"
	case e.Offset >= civic7410.TXDupUnmappedOffset && e.Offset < civic7410.TXDupUnmappedOffset+civic7410.TXDupUnmappedLength:
		what = "the TX-duplicate block's unmapped remainder (TX mode/filter/data-mode/tone-mode/tone-tx/tone-rx, matrix §1b)"
	}
	return fmt.Sprintf(
		"ic7410: this slot's record byte %d carries %#02x where the profile's Fixed template carries %#02x — %s. A slot may be written ONLY when its unmapped regions equal the template, and anything else is REFUSED, never rewritten",
		e.Offset, e.Got, e.Want, what)
}

func (e *UnmappedRegionError) Unwrap() error { return ErrUnmappedRegion }

// ErrOutOfDomain is the sentinel for a Known value outside what this
// radio's record can encode.
var ErrOutOfDomain = errors.New("ic7410: a Known value lies outside what this radio's record can encode")

// OutOfDomainError reports a Known numeric value outside the domain THIS
// SESSION'S CAPABILITIES declare — defence in depth, and not the outbound
// gate: civ.FieldSpan carries no numeric domain.
type OutOfDomainError struct {
	Field    spec.Field
	Value    uint64
	Min, Max uint64
	Where    string
}

func (e *OutOfDomainError) Error() string {
	return fmt.Sprintf(
		"ic7410: %s = %d is outside this radio's declared domain %d..%d (%s) — refused by the driver",
		e.Field, e.Value, e.Min, e.Max, e.Where)
}

func (e *OutOfDomainError) Unwrap() error { return ErrOutOfDomain }

// domainRefusal applies WriteChannel's numeric-domain rung to one channel.
// The TX frequency span shares the RX span's own domain: matrix §1b reads
// it as the same field shape, "programmed in the same manner as ④~⑧".
func domainRefusal(d codeplug.ChannelData, caps spec.Capabilities) error {
	if d.FreqHz < caps.MinFreqHz || d.FreqHz > caps.MaxFreqHz {
		return &OutOfDomainError{
			Field: spec.FieldFrequency, Value: d.FreqHz,
			Min: caps.MinFreqHz, Max: caps.MaxFreqHz,
			Where: "spec.Capabilities.MinFreqHz/MaxFreqHz, the receiver coverage printed in the specifications",
		}
	}
	if d.TxFreqHz.State == codeplug.Known {
		v := d.TxFreqHz.Value
		if v < caps.MinFreqHz || v > caps.MaxFreqHz {
			return &OutOfDomainError{
				Field: spec.FieldTxFrequency, Value: v,
				Min: caps.MinFreqHz, Max: caps.MaxFreqHz,
				Where: "spec.Capabilities.MinFreqHz/MaxFreqHz — the TX-duplicate block's frequency span shares the RX span's own field shape (matrix §1b)",
			}
		}
	}
	r := caps.CTCSSToneRange
	for _, tt := range []struct {
		field spec.Field
		tone  codeplug.ToneField
	}{
		{spec.FieldToneTx, d.ToneTx},
		{spec.FieldToneRx, d.ToneRx},
	} {
		if tt.tone.State != codeplug.Known {
			continue
		}
		v := uint64(tt.tone.Value)
		if r == nil {
			return &OutOfDomainError{Field: tt.field, Value: v, Where: "this session declares no CTCSSToneRange at all, so no tone is authorised"}
		}
		if v < uint64(r.MinDeciHz) || v > uint64(r.MaxDeciHz) {
			return &OutOfDomainError{
				Field: tt.field, Value: v,
				Min: uint64(r.MinDeciHz), Max: uint64(r.MaxDeciHz),
				Where: "spec.Capabilities.CTCSSToneRange, the tone span's own BCD capacity",
			}
		}
	}
	return nil
}

// unconditionalFields are the eight spec.Fields the 40-byte record ALWAYS
// carries and this driver therefore always requests against the
// capability gate. tx_frequency is deliberately NOT here — see
// conditionalRequestedFields: it is capability-gated only when the caller
// has actually asked for a value, which is what lets an ordinary SCAN
// write (which never sets one) succeed even though that bank's
// tx_frequency WRITE is Unsupported.
var unconditionalFields = []spec.Field{
	spec.FieldFrequency,
	spec.FieldMode,
	spec.FieldFilter,
	spec.FieldDataMode,
	spec.FieldToneMode,
	spec.FieldToneTx,
	spec.FieldToneRx,
	spec.FieldTag,
}

// conditionalRequestedFields pairs every remaining state-bearing
// spec.Field with a predicate reporting whether this write actually asks
// for it, so the gate can REFUSE a Known value it cannot honour rather
// than silently dropping it.
var conditionalRequestedFields = []struct {
	field   spec.Field
	present func(codeplug.ChannelData) bool
}{
	{spec.FieldClarifier, func(d codeplug.ChannelData) bool { return d.ClarHz != 0 || d.RxClar || d.TxClar }},
	{spec.FieldCTCSSState, func(d codeplug.ChannelData) bool { return d.CTCSS != "" }},
	{spec.FieldCTCSSTone, func(d codeplug.ChannelData) bool { return d.CTCSSTone.State == codeplug.Known }},
	{spec.FieldShift, func(d codeplug.ChannelData) bool { return d.Shift != "" }},
	{spec.FieldTagDisplay, func(d codeplug.ChannelData) bool { return d.TagDisplay.State == codeplug.Known }},
	{spec.FieldScanSkip, func(d codeplug.ChannelData) bool { return d.ScanSkip.State == codeplug.Known }},
	// tx_frequency: MATRIX §2 SCAN ROW 11's own reasoning, made mechanical.
	// A caller who sets an explicit TxFreqHz is asking this radio to honour
	// an independent transmit frequency, and on the SCAN bank that is
	// refused (write Unsupported there); a caller who leaves it Unknown
	// gets the mirrored-from-RX value write.go's WriteChannel builds
	// unconditionally, on every bank, because the wire's TX-duplicate block
	// is "still necessary" regardless (spec.md's ruling 7).
	{spec.FieldTxFrequency, func(d codeplug.ChannelData) bool { return d.TxFreqHz.State == codeplug.Known }},
	{spec.FieldDuplex, func(d codeplug.ChannelData) bool { return d.Duplex.State == codeplug.Known }},
	{spec.FieldOffset, func(d codeplug.ChannelData) bool { return d.OffsetHz.State == codeplug.Known }},
	{spec.FieldDTCSCode, func(d codeplug.ChannelData) bool { return d.DTCSCode.State == codeplug.Known }},
	{spec.FieldDTCSPolarity, func(d codeplug.ChannelData) bool { return d.DTCSPolarity.State == codeplug.Known }},
}

func requestedFields(d codeplug.ChannelData) []spec.Field {
	fields := append([]spec.Field(nil), unconditionalFields...)
	for _, c := range conditionalRequestedFields {
		if c.present(d) {
			fields = append(fields, c.field)
		}
	}
	return fields
}

// refused builds the neutral refusal, with an empty (never nil) Steps.
func refused(slot string, fields []spec.Field, reason string) (driver.WriteResult, error) {
	return driver.WriteResult{Steps: []driver.WriteStep{}},
		&driver.WriteRefusedError{Slot: slot, Fields: fields, Reason: reason}
}

// WriteChannel implements driver.Session: ONE acknowledged 1A 00 memory
// set, preceded by ONE read.
//
// THE LADDER, IN TIER RULING T5's ORDER:
//
//	LOCALLY DECIDABLE (precede ALL wire traffic):
//	  1. erase?                Channel.Data == nil            -> ErrWriteRefused
//	  2. capability gate       requestedFields x FieldSupport -> ErrWriteRefused
//	  3. field-state shape     non-Known mandatory field      -> ErrWriteRefused
//	  3b. vocabularies         mode/filter/tone_mode           -> ErrWriteRefused
//	  4. numeric domains       freq, tx_frequency, tone        -> *OutOfDomainError
//	  ---------------------------------------------------------------------
//	  5. ONE read              readRaw (T2 address check, T4 rejection branch)
//	  ---------------------------------------------------------------------
//	READ-DEPENDENT:
//	  6. unmapped regions      byte 0, bytes 21-30              -> *UnmappedRegionError
//	  7. tone source           UPDATE: the just-read bytes; CREATE: refuse
//	  ---------------------------------------------------------------------
//	  8. BuildMemorySet -> the outbound gate -> the acknowledged exchange
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	// RUNG 1 — ERASE.
	if ch.Data == nil {
		return refused(ch.Slot, []spec.Field{spec.FieldErase},
			"this radio's memory-clear form is printed in its document but no IC-7410 has ever been asked to use it, so FieldErase carries the zero FieldSupport and consent structurally never reaches it (matrix §3.13)")
	}
	d := *ch.Data

	a, bank, err := slotToAddress(ch.Slot)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}}, fmt.Errorf("ic7410: WriteChannel: %w", err)
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
			"this session cannot write these fields on bank "+string(bank)+": either no IC-7410 has ever been written to by this project and no consent was recorded, or the field is one this record leaves UNMAPPED or Unsupported on this bank (matrix §2)")
	}

	// RUNG 3 — FIELD-STATE SHAPE.
	if missing, reason := missingMandatory(d); missing != "" {
		return refused(ch.Slot, []spec.Field{missing}, reason)
	}

	// RUNG 3b — VOCABULARIES.
	if field, reason := outsideVocabulary(d, s.caps); field != "" {
		return refused(ch.Slot, []spec.Field{field}, reason)
	}

	// RUNG 4 — NUMERIC DOMAINS.
	if err := domainRefusal(d, s.caps); err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}}, err
	}

	// RUNG 5 — THE ONE READ.
	prior, raw, empty, err := s.readRaw(ctx, a)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}},
			fmt.Errorf("ic7410: WriteChannel %s: the pre-write preservation read: %w", ch.Slot, err)
	}

	// RUNG 6 — UNMAPPED-REGION CHECK. An EMPTY slot has no unmapped
	// regions to compare and the write proceeds against the template.
	if !empty {
		if e := unmappedRegionsDiffer(raw); e != nil {
			return driver.WriteResult{Steps: []driver.WriteStep{}}, e
		}
	}

	// RUNG 7 — THE TONE SOURCE.
	toneTx, ok := toneSource(d.ToneTx, prior.ToneTXDeciHz, empty)
	if !ok {
		return refused(ch.Slot, []spec.Field{spec.FieldToneTx}, toneRefusalReason(empty))
	}
	toneRx, ok := toneSource(d.ToneRx, prior.ToneRXDeciHz, empty)
	if !ok {
		return refused(ch.Slot, []spec.Field{spec.FieldToneRx}, toneRefusalReason(empty))
	}

	// THE TX-FREQUENCY SOURCE. spec.md's ruling 7: a Known caller value is
	// used as given (rung 4 has already bounded it); otherwise the RX span
	// is MIRRORED into the TX span, unconditionally — the manual states the
	// TX-duplicate block is "still necessary" even when Split is OFF, and
	// this record's Split flag is itself UNMAPPED (always written zero by
	// rung 6's own template), so no state this driver tracks is ever
	// genuinely "Split ON". This is a DELIBERATE DEVIATION from
	// core/driver/ic7300's "REV 1 struck, never synthesise TxFreqHz" rule,
	// ruled explicitly for this model because — unlike the IC-7300 — this
	// manual says something about the OFF case at all. NO READ IS NEEDED
	// for this substitution, unlike the tone spans: d.FreqHz is always at
	// hand on the channel already being written.
	txFreqHz := d.FreqHz
	if d.TxFreqHz.State == codeplug.Known {
		txFreqHz = d.TxFreqHz.Value
	}

	// RUNG 8 — BUILD, GATE, EXCHANGE.
	rec := civ.MemoryRecord{
		Address:      a,
		RXFreqHz:     civ.Available(d.FreqHz),
		TXFreqHz:     civ.Available(txFreqHz),
		Mode:         civ.Available(d.Mode),
		Filter:       civ.Available(d.Filter.Value),
		DataMode:     civ.Available(dataModeName(d.DataMode.Value)),
		ToneMode:     civ.Available(d.ToneMode.Value),
		ToneTXDeciHz: civ.Available(toneTx),
		ToneRXDeciHz: civ.Available(toneRx),
		Name:         civ.Available(d.Tag),
	}
	cmd, err := civic7410.Profile().BuildMemorySet(rec)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}},
			fmt.Errorf("ic7410: WriteChannel %s: building the 1A 00 set: %w", ch.Slot, err)
	}

	steps := []driver.WriteStep{{Command: memorySetStep}}
	p := civic7410.Profile()
	_, err = s.eng.Do(ctx, cmd, civ.CIVWriteWithAckSpec(p.AcknowledgementMatcher()))
	switch {
	case err == nil:
		steps[0].Sent = true
		steps[0].Confirmed = true
		return driver.WriteResult{Steps: steps}, nil

	case errors.Is(err, transport.ErrRejected):
		steps[0].Sent = true
		return driver.WriteResult{Steps: steps},
			fmt.Errorf("ic7410: WriteChannel %s: the radio rejected the %s set: %w", ch.Slot, memorySetStep, err)

	default:
		return driver.WriteResult{Steps: steps},
			fmt.Errorf("ic7410: WriteChannel %s: the %s set was written and never acknowledged, so its outcome is unknown: %w", ch.Slot, memorySetStep, err)
	}
}

// dataModeName renders codeplug's bool as this model's wire vocabulary.
func dataModeName(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}

// missingMandatory reports the first mapped field with nothing to encode.
func missingMandatory(d codeplug.ChannelData) (spec.Field, string) {
	if d.Mode == "" {
		return spec.FieldMode, "the record's mode byte (⑨) has no \"leave it alone\" encoding, and this project refuses rather than synthesises a value"
	}
	if d.Filter.State != codeplug.Known {
		return spec.FieldFilter, "the record's filter byte (⑩) has no \"leave it alone\" encoding; the page prints three values and no default"
	}
	if d.DataMode.State != codeplug.Known {
		return spec.FieldDataMode, "the record's data-mode byte (⑪, a whole byte on this model) has no \"leave it alone\" encoding"
	}
	if d.ToneMode.State != codeplug.Known {
		return spec.FieldToneMode, "the record's tone-mode nibble (⑫ high) has no \"leave it alone\" encoding"
	}
	if len(d.Tag) > 0 {
		for i := 0; i < len(d.Tag); i++ {
			if !strings.Contains(civic7410.NameCharset, string(d.Tag[i])) {
				return spec.FieldTag, fmt.Sprintf("the tag carries byte %#02x at index %d, which is not in this radio's printed memory-name charset (matrix §1 row 25, §3.9)", d.Tag[i], i)
			}
		}
	}
	if len(d.Tag) > civic7410.Profile().NameLength() {
		return spec.FieldTag, fmt.Sprintf("the tag is %d characters and this radio's name span ⑲~㉗ holds %d", len(d.Tag), civic7410.Profile().NameLength())
	}
	return "", ""
}

// unmappedRegionsDiffer applies the E6-style refusal to this model's two
// unmapped record regions: the whole Select-memory + Split byte at offset
// 0, and the TX-duplicate block's ten unmapped bytes at offset 21-30.
func unmappedRegionsDiffer(raw []byte) error {
	tmpl := civic7410.FixedTemplate()
	if len(raw) < len(tmpl) {
		return fmt.Errorf("ic7410: internal: the unmapped-region comparison was handed a %d-byte record where the profile declares %d — the length fingerprint should have refused this answer before it reached here", len(raw), len(tmpl))
	}
	offsets := []int{civic7410.SelectSplitOffset}
	for i := 0; i < civic7410.TXDupUnmappedLength; i++ {
		offsets = append(offsets, civic7410.TXDupUnmappedOffset+i)
	}
	for _, off := range offsets {
		if want, got := tmpl[off], raw[off]; want != got {
			return &UnmappedRegionError{Offset: off, Want: want, Got: got}
		}
	}
	return nil
}

// toneRefusalReason picks the reason that is TRUE of the write in hand.
func toneRefusalReason(create bool) string {
	if create {
		return noDefaultToneReason
	}
	return "this slot's record carried no tone span to preserve, which this profile's single 40-byte layout makes impossible: it maps ⑬~⑮ and ⑯~⑱ unconditionally. Reaching this refusal means the record layout and this driver's mapping have gone out of step — report it rather than working around it"
}

const noDefaultToneReason = "this slot has no prior record to preserve a tone from, and this radio's document prints no default tone value. Tier ruling T1(5)'s REFUSE arm rather than its default arm"

// toneSource settles where a tone span's bytes come from (tier ruling
// T1(4) and T1(5)).
func toneSource(want codeplug.ToneField, prior civ.Optional[uint64], create bool) (uint64, bool) {
	if want.State == codeplug.Known {
		return uint64(want.Value), true
	}
	if create {
		return 0, false
	}
	v, ok := prior.Get()
	return v, ok
}

// outsideVocabulary reports the first mapped enum field carrying a Known
// value this radio cannot express.
func outsideVocabulary(d codeplug.ChannelData, caps spec.Capabilities) (spec.Field, string) {
	toneModes := make([]string, len(caps.ToneModes))
	for i, m := range caps.ToneModes {
		toneModes[i] = m.Value
	}
	for _, v := range []struct {
		field spec.Field
		value string
		vocab []string
		where string
	}{
		{spec.FieldMode, d.Mode, caps.Modes, "the record's ⑨ is a mode enum, and matrix §1 row 6 prints eight codes and no more"},
		{spec.FieldFilter, d.Filter.Value, caps.Filters, "the record's ⑩ is a filter enum, and matrix §1 row 24 prints three values and no default"},
		{spec.FieldToneMode, d.ToneMode.Value, toneModes, "the record's ⑫ high nibble is a tone-mode enum, and matrix §1 row 21 prints three values"},
	} {
		if slices.Contains(v.vocab, v.value) {
			continue
		}
		return v.field, fmt.Sprintf(
			"%q is not a value this radio can express (%s: %s). A Known value the wire cannot say faithfully is REFUSED, never dropped and never mapped to a neighbour",
			v.value, v.where, quoted(v.vocab))
	}
	return "", ""
}

func quoted(vocab []string) string {
	out := make([]string, len(vocab))
	for i, v := range vocab {
		out[i] = strconv.Quote(v)
	}
	return strings.Join(out, ", ")
}
