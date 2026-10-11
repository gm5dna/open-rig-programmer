// SPDX-License-Identifier: GPL-3.0-or-later

package icom

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// MemorySetStep is the mnemonic reported for the one write frame: a LABEL
// for the audit trail and for a human reading a journal, never a token a
// generic layer branches on (driver.WriteStep's contract).
const MemorySetStep = "1A 00"

// Region is one unmapped part of the record that a write compares against
// Params.Template: Nibble is "whole", "high" or "low", the part of byte
// Offset it covers.
type Region struct {
	Offset int
	Nibble string
}

// WriteText is the wording of the refusals only one radio can word, each
// the model's own sentence or fragment.
type WriteText struct {
	// Erase is the whole reason for the erase refusal.
	Erase string
	// Unwritable is the reason's tail after "this session cannot write
	// these fields on bank <bank>: ".
	Unwritable string
	// TagCharsetTail names where the radio's name charset is printed; it
	// closes "...which is not in this radio's printed memory-name charset
	// (<tail>)".
	TagCharsetTail string
	// NoDefaultTone is the whole reason for a tone span with no source on a
	// CREATE.
	NoDefaultTone string
	// LayoutNote names the record layout in the UPDATE-arm tone reason,
	// "which this profile's <note> makes impossible".
	LayoutNote string
	// ModeWhere, FilterWhere and ToneModeWhere say where each vocabulary is
	// printed, inside "(<where>: <values>)".
	ModeWhere, FilterWhere, ToneModeWhere string
}

// mask reduces one record byte to the part r names.
func (r Region) mask(b byte) byte {
	switch r.Nibble {
	case "high":
		return b >> 4
	case "low":
		return b & 0x0F
	}
	return b
}

// conditionalField pairs a state-bearing spec.Field outside the base set
// with a predicate reporting whether the channel carries a Known value for
// it.
type conditionalField struct {
	Field   spec.Field
	Present func(codeplug.ChannelData) bool
}

// ConditionalRequestedFields is every state-bearing spec.Field OUTSIDE the
// base set, in ChannelData declaration order, appended after the base set.
//
// A FIELD MISSING FROM THIS TABLE IS A FIELD THE GATE WOULD NEVER SEE, and
// therefore a Known value that would be silently dropped rather than
// refused. TWO PREDICATES ARE EXACT AND ONE IS AN APPROXIMATION:
// FieldCTCSSState and FieldShift are plain strings whose empty value is not
// a vocabulary member of any radio, so "non-empty" and "the caller asked"
// are the same test. THE CLARIFIER IS NOT: ClarHz 0 with both flags false
// is indistinguishable from a channel that never carried one, so an
// explicitly-zero clarifier is not appended. That costs nothing on a record
// with no clarifier span, and is the known bound of this table.
var ConditionalRequestedFields = []conditionalField{
	{spec.FieldClarifier, func(d codeplug.ChannelData) bool { return d.ClarHz != 0 || d.RxClar || d.TxClar }},
	{spec.FieldCTCSSState, func(d codeplug.ChannelData) bool { return d.CTCSS != "" }},
	{spec.FieldCTCSSTone, func(d codeplug.ChannelData) bool { return d.CTCSSTone.State == codeplug.Known }},
	{spec.FieldShift, func(d codeplug.ChannelData) bool { return d.Shift != "" }},
	{spec.FieldTagDisplay, func(d codeplug.ChannelData) bool { return d.TagDisplay.State == codeplug.Known }},
	{spec.FieldScanSkip, func(d codeplug.ChannelData) bool { return d.ScanSkip.State == codeplug.Known }},
	{spec.FieldTxFrequency, func(d codeplug.ChannelData) bool { return d.TxFreqHz.State == codeplug.Known }},
	{spec.FieldDuplex, func(d codeplug.ChannelData) bool { return d.Duplex.State == codeplug.Known }},
	{spec.FieldOffset, func(d codeplug.ChannelData) bool { return d.OffsetHz.State == codeplug.Known }},
	{spec.FieldDTCSCode, func(d codeplug.ChannelData) bool { return d.DTCSCode.State == codeplug.Known }},
	{spec.FieldDTCSPolarity, func(d codeplug.ChannelData) bool { return d.DTCSPolarity.State == codeplug.Known }},
	{spec.FieldDataMode, func(d codeplug.ChannelData) bool { return d.DataMode.State == codeplug.Known }},
}

// RequestedFields is every spec.Field a write of d would TRANSMIT or
// REQUEST, in ChannelData declaration order: the seven fields the 1A 00
// record maps, then every conditional field the caller carries Known.
//
// THE CONDITIONALS ARE WHAT MAKE THE GATE HONEST. A caller who hands
// WriteChannel a Known value this radio cannot express must meet a
// WriteRefusedError NAMING THE FIELD, not a silent drop — INCLUDING THE
// UNMAPPED FIELDS A DRIVER REFUSES ON PURPOSE. core/codeplug/diff.go is
// explicit that Known conditional fields are never filtered out even when
// unreachable, which is what makes the gate able to REFUSE rather than DROP.
func RequestedFields(d codeplug.ChannelData) []spec.Field {
	fields := []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldTag,
		spec.FieldToneMode,
		spec.FieldToneTx,
		spec.FieldToneRx,
		spec.FieldFilter,
	}
	for _, c := range ConditionalRequestedFields {
		if c.Present(d) {
			fields = append(fields, c.Field)
		}
	}
	return fields
}

// refused builds the neutral refusal, with an empty (never nil) Steps
// slice: a refusal that happens before the frames are built has no
// sequence to describe.
func refused(slot string, fields []spec.Field, reason string) (driver.WriteResult, error) {
	return driver.WriteResult{Steps: []driver.WriteStep{}},
		&driver.WriteRefusedError{Slot: slot, Fields: fields, Reason: reason}
}

// WriteChannel is ONE acknowledged 1A 00 memory set, preceded by ONE read.
//
// THE LADDER, IN TIER RULING T5's ORDER:
//
//	LOCALLY DECIDABLE (all of these precede ALL wire traffic):
//	  1. erase?                Channel.Data == nil            -> ErrWriteRefused
//	  2. capability gate       RequestedFields x FieldSupport -> ErrWriteRefused
//	  3. field-state shape     non-Known mandatory field      -> ErrWriteRefused
//	  3b. vocabularies         mode/filter/tone_mode not expressible -> ErrWriteRefused
//	  4. numeric domains       Params.Domain                  -> the package's own error
//	  4b. field-state walk     driver.CheckFieldStates        -> ErrWriteRefused
//	  -----------------------------------------------------------------------
//	  5. ONE read              ReadRaw (T2 address check, T4 rejection branch)
//	  -----------------------------------------------------------------------
//	READ-DEPENDENT (the single recorded exception):
//	  6. E6 unmapped regions   Params.Unmapped vs Params.Template -> Params.NewUnmapped
//	  7. tone source           UPDATE: the just-read bytes; CREATE: refuse
//	  -----------------------------------------------------------------------
//	  8. BuildMemorySet -> the outbound gate -> the acknowledged exchange
//
// RUNGS 1-4b ARE THE REASON A REFUSED WRITE PUTS **ZERO** BYTES ON THE WIRE;
// rungs 6-7 are the reason an E6 refusal costs exactly ONE READ'S WORTH and
// no set frame. The order is observable (it decides which error a caller
// meets), so it is part of the contract.
//
// THE WRITE IS NEVER RESENT. civ.CIVWriteWithAckSpec fixes RetryReads at
// zero and Engine.Do refuses a non-zero value on ClassWriteWithAck before
// writing anything (safety obligation 2): a lost write's outcome is
// genuinely ambiguous, and resending one is how a radio ends up written
// twice. NO READ-BACK VERIFICATION either: that is the clone service's job.
//
// PRECONDITION: p is a Params Open accepted, with the write-side fields
// (Template, Unmapped, NewUnmapped, Text) set; Domain may be unset.
func WriteChannel(ctx context.Context, p *Params, eng *transport.Engine, caps spec.Capabilities, mismatches *atomic.Uint64, ch codeplug.Channel) (driver.WriteResult, error) {
	// RUNG 1 — ERASE. ChannelData has NO Erase member: erase is
	// represented solely by Channel.Data == nil, "the SOLE discriminator
	// between empty and populated" (core/codeplug/channel.go). FieldErase
	// carries the zero FieldSupport and spec.ConsentUnverifiedWrites
	// structurally never consents it, so this refusal stands under consent
	// too.
	if ch.Data == nil {
		return refused(ch.Slot, []spec.Field{spec.FieldErase}, p.Text.Erase)
	}
	d := *ch.Data

	a, bank, err := SlotToAddress(p.Name, ch.Slot)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}}, fmt.Errorf("%s: WriteChannel: %w", p.Name, err)
	}

	// RUNG 2 — THE CAPABILITY GATE. Every field this write would transmit
	// or request must be writable on THIS BANK under THIS SESSION's
	// effective capabilities. Defence in depth below the clone service,
	// which has already made the same check.
	var unwritable []spec.Field
	for _, f := range RequestedFields(d) {
		if !caps.FieldSupport(bank, f).CanWrite() {
			unwritable = append(unwritable, f)
		}
	}
	if len(unwritable) > 0 {
		return refused(ch.Slot, unwritable,
			"this session cannot write these fields on bank "+string(bank)+": "+p.Text.Unwritable)
	}

	// RUNG 3 — FIELD-STATE SHAPE. Every MAPPED field must have something
	// to encode, and this rung is what makes the driver the first detector
	// rather than the builder: civ's encodeRecord returns a codec error for
	// an absent mapped field, and by then the preservation read would
	// already have put traffic on the wire. T5 forbids that. The tone spans
	// are NOT here: their source depends on whether this is an UPDATE or a
	// CREATE, which cannot be known without the read. Rung 7 settles them.
	if missing, reason := missingMandatory(p, d); missing != "" {
		return refused(ch.Slot, []spec.Field{missing}, reason)
	}

	// RUNG 3b — VOCABULARIES. Locally decidable: the three enum
	// vocabularies are on this session's own capabilities.
	if field, reason := OutsideVocabulary(p, d, caps); field != "" {
		return refused(ch.Slot, []spec.Field{field}, reason)
	}

	// RUNG 4 — NUMERIC DOMAINS. Defence in depth and NOT the gate.
	if p.Domain != nil {
		if err := p.Domain(d, caps); err != nil {
			return driver.WriteResult{Steps: []driver.WriteStep{}}, err
		}
	}

	// RUNG 4b — THE FIELD-STATE WALK. core/driver.CheckFieldStates is THE
	// FLEET'S shared walk: every field of ChannelData that carries a
	// FieldState, judged against this session's own vocabularies. What it
	// prevents is silent rather than loud — a value carried alongside a
	// state meaning "preserve whatever the radio has" (or alongside Absent,
	// the zero state a hand-built ChannelData leaves behind) is never named
	// by RequestedFields, so without this rung it would be DROPPED from the
	// frame and the write would report success. PLACED AFTER RUNG 4: that
	// rung also judges a Known tone's domain and returns the more specific
	// package error, whereas this walk's ToneField.Valid(caps) asks the same
	// question, so placed earlier it would intercept those cases with a
	// generic refusal. Here it is reached only for what rung 4 does not
	// judge — Unknown/Unavailable/Absent carrying a stray value.
	if field, err := driver.CheckFieldStates(caps, d); err != nil {
		return refused(ch.Slot, []spec.Field{field}, err.Error())
	}

	// RUNG 5 — THE ONE READ. E6 needs the slot's RAW unmapped regions and
	// T1(4) needs its tone numbers; both come from this single exchange,
	// through the same primitive ReadChannel uses, so the T2 address check
	// and the T4 rejection branch cannot drift.
	prior, raw, empty, err := ReadRaw(ctx, p, eng, mismatches, a)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}},
			fmt.Errorf("%s: WriteChannel %s: the pre-write preservation read: %w", p.Name, ch.Slot, err)
	}

	// RUNG 6 — THE E6 UNMAPPED-REGION CHECK. An EMPTY slot has no unmapped
	// regions to compare and the write proceeds against the template.
	if !empty {
		if e := UnmappedRegionsDiffer(p, raw); e != nil {
			return driver.WriteResult{Steps: []driver.WriteStep{}}, e
		}
	}

	// RUNG 7 — THE TONE SOURCE (tier ruling T1(4) and T1(5)).
	toneTx, ok := toneSource(d.ToneTx, prior.ToneTXDeciHz, empty)
	if !ok {
		return refused(ch.Slot, []spec.Field{spec.FieldToneTx}, ToneRefusalReason(p, empty))
	}
	toneRx, ok := toneSource(d.ToneRx, prior.ToneRXDeciHz, empty)
	if !ok {
		return refused(ch.Slot, []spec.Field{spec.FieldToneRx}, ToneRefusalReason(p, empty))
	}

	// RUNG 8 — BUILD, GATE, EXCHANGE. BuildMemorySet writes the profile's
	// Fixed template into every unmapped region and every mapped span from
	// this record, so the record bytes are always sent in full.
	rec := civ.MemoryRecord{
		Address:      a,
		RXFreqHz:     civ.Available(d.FreqHz),
		Mode:         civ.Available(d.Mode),
		Filter:       civ.Available(d.Filter.Value),
		ToneMode:     civ.Available(d.ToneMode.Value),
		ToneTXDeciHz: civ.Available(toneTx),
		ToneRXDeciHz: civ.Available(toneRx),
		Name:         civ.Available(d.Tag),
	}
	prof := p.Profile()
	cmd, err := prof.BuildMemorySet(rec)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}},
			fmt.Errorf("%s: WriteChannel %s: building the 1A 00 set: %w", p.Name, ch.Slot, err)
	}

	steps := []driver.WriteStep{{Command: MemorySetStep}}
	_, err = eng.Do(ctx, cmd, civ.CIVWriteWithAckSpec(prof.AcknowledgementMatcher()))
	switch {
	case err == nil:
		// FB arrived.
		steps[0].Sent = true
		steps[0].Confirmed = true
		return driver.WriteResult{Steps: steps}, nil

	case errors.Is(err, transport.ErrRejected):
		// FA arrived, from THIS radio's address (E1's source-address
		// check). An explicit rejection is an ATTRIBUTABLE outcome, so Sent
		// is true and Confirmed is false.
		steps[0].Sent = true
		return driver.WriteResult{Steps: steps},
			fmt.Errorf("%s: WriteChannel %s: the radio rejected the %s set: %w", p.Name, ch.Slot, MemorySetStep, err)

	default:
		// AN ACK TIMEOUT REPORTS AN UNKNOWN OUTCOME, and BOTH FLAGS ARE
		// FALSE. core/driver/driver.go defines a false Sent as "the
		// outcome is NOT known-clean … a transport-level failure left its
		// outcome unknowable", which is exactly this. Reporting Sent: true
		// would put a false ATTRIBUTABLE outcome in the audit trail. What
		// proves the never-retransmit rule is the port's byte log showing
		// exactly ONE write.
		return driver.WriteResult{Steps: steps},
			fmt.Errorf("%s: WriteChannel %s: the %s set was written and never acknowledged, so its outcome is unknown: %w", p.Name, ch.Slot, MemorySetStep, err)
	}
}

// missingMandatory reports the first mapped field with nothing to encode,
// and why that is a refusal rather than something to fill in.
//
// The five checked here are the ones whose source is the CHANNEL alone.
// Frequency is not among them: ChannelData.FreqHz is a plain uint64 that
// always carries a value, so its analogue is rung 4's numeric domain. The
// two tone spans are not among them either: their source depends on the
// preservation read (rung 7).
func missingMandatory(p *Params, d codeplug.ChannelData) (spec.Field, string) {
	if d.Mode == "" {
		return spec.FieldMode, "the record's ⑨ is a mode enum with no \"leave it alone\" encoding, and this project refuses rather than synthesises a value"
	}
	if d.Filter.State != codeplug.Known {
		return spec.FieldFilter, "the record's ⑩ is a filter enum with no \"leave it alone\" encoding; the page prints three values and no default, and inventing a fourth would be a radio claim"
	}
	if d.ToneMode.State != codeplug.Known {
		return spec.FieldToneMode, "the record's ⑪ low nibble is a tone-mode enum with no \"leave it alone\" encoding"
	}
	prof := p.Profile()
	charset := prof.NameCharset()
	for i := 0; i < len(d.Tag); i++ {
		if bytes.IndexByte(charset, d.Tag[i]) < 0 {
			return spec.FieldTag, fmt.Sprintf("the tag carries byte %#02x at index %d, which is not in this radio's printed memory-name charset (%s)", d.Tag[i], i, p.Text.TagCharsetTail)
		}
	}
	if len(d.Tag) > prof.NameLength() {
		return spec.FieldTag, fmt.Sprintf("the tag is %d characters and this radio's name span ⑱~㉗ holds %d", len(d.Tag), prof.NameLength())
	}
	return "", ""
}

// UnmappedRegionsDiffer applies ruling E6 to the record's unmapped regions,
// in Params.Unmapped's order so the refusal a caller meets is
// deterministic. A region that differs from Params.Template is returned
// through Params.NewUnmapped.
//
// The template is asked for by name rather than compared against a literal
// zero, because E6's ruling is stated in terms of "the profile's Fixed
// template" and a driver that hard-coded the zeroes would keep passing if
// the template ever changed.
func UnmappedRegionsDiffer(p *Params, raw []byte) error {
	if len(raw) < len(p.Template) {
		// UNREACHABLE, and it says so rather than inventing a nibble.
		// civ.Profile.MemoryAnswerRecord has already refused any length but
		// this profile's own — that check is the probe's CONTINUOUS length
		// fingerprint and ReadRaw cannot return past it — so a short record
		// here means the fingerprint invariant has been broken upstream. A
		// refusal that named a nibble would print "byte 0 carries 0x0 where
		// the template carries 0x0", which is a lie about a record that was
		// never measured.
		return fmt.Errorf("%s: internal: the E6 comparison was handed a %d-byte record where the profile declares %d — the length fingerprint should have refused this answer before it reached here", p.Name, len(raw), len(p.Template))
	}
	for _, r := range p.Unmapped {
		want, got := r.mask(p.Template[r.Offset]), r.mask(raw[r.Offset])
		if want != got {
			return p.NewUnmapped(driver.UnmappedRegionError{Offset: r.Offset, Nibble: r.Nibble, Want: want, Got: got})
		}
	}
	return nil
}

// ToneRefusalReason picks the reason that is TRUE of the write in hand.
//
// The two arms are different facts and must not share a sentence. On a
// CREATE there is genuinely no prior record to preserve from; on an UPDATE
// there IS one and it carried no tone span, which on a layout that maps
// both unconditionally cannot happen — so saying "no prior record" there
// would be a lie told to a user in the one situation where the code is
// already somewhere it should not be.
func ToneRefusalReason(p *Params, create bool) string {
	if create {
		return p.Text.NoDefaultTone
	}
	return "this slot's record carried no tone span to preserve, which this profile's " + p.Text.LayoutNote + " makes impossible: it maps ⑫~⑭ and ⑮~⑰ unconditionally, so a successfully parsed record always carries both. Reaching this refusal means the record layout and this driver's mapping have gone out of step — report it rather than working around it"
}

// toneSource settles where a tone span's bytes come from — tier ruling
// T1(4) and T1(5) — reporting false when the write must be refused.
//
//   - KNOWN: the caller's value, which rung 4 has already bounded.
//   - NOT KNOWN, on an UPDATE: the JUST-READ record's civ-layer tone
//     number, VERBATIM. This is PRESERVATION, NOT SYNTHESIS — the value
//     came from the radio, in the read the E6/T5 rung already required,
//     and no value is invented. It is also why ReadChannel mapping an
//     out-of-domain tone to Unknown does not make that channel unwritable:
//     the number goes back exactly as it came, 0 included.
//   - NOT KNOWN, on a CREATE: REFUSED. There is no prior record, so the
//     span has no source, and no default is documented.
func toneSource(want codeplug.ToneField, prior civ.Optional[uint64], create bool) (uint64, bool) {
	if want.State == codeplug.Known {
		return uint64(want.Value), true
	}
	if create {
		return 0, false
	}
	// Unreachable-with-false through a layout that maps both tone spans, so
	// a parsed record always carries both. Refused rather than defaulted to
	// zero, which would be synthesis.
	v, ok := prior.Get()
	return v, ok
}

// OutsideVocabulary reports the first mapped enum field carrying a Known
// value this radio cannot express, and why.
//
// RULING T5 NAMES VOCABULARIES among the refusals that precede ALL wire
// traffic, and this check is locally decidable: the three vocabularies are
// on the session's own capabilities, with nothing to look up on the radio.
//
// LEFT TO THE BUILDER, THE FAILURE IS WRONG IN THREE WAYS AT ONCE. civ's
// encodeRecord does refuse an out-of-vocabulary enum value, but by then
// the preservation read has already put a frame on the wire; the error is a
// wrapped CODEC error rather than a *driver.WriteRefusedError; and a caller
// keying on driver.ErrWriteRefused sees nothing.
//
// THE VOCABULARIES ARE THE CAPABILITIES', NOT THE CODEC'S, and that is
// deliberate: the value the UI offered, the value codeplug.Validate judged
// and the value this rung admits are one list.
//
// FREQUENCY IS NOT HERE because it is not a vocabulary: its domain is a
// RANGE, checked at rung 4.
func OutsideVocabulary(p *Params, d codeplug.ChannelData, caps spec.Capabilities) (spec.Field, string) {
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
		{spec.FieldMode, d.Mode, caps.Modes, p.Text.ModeWhere},
		{spec.FieldFilter, d.Filter.Value, caps.Filters, p.Text.FilterWhere},
		{spec.FieldToneMode, d.ToneMode.Value, toneModes, p.Text.ToneModeWhere},
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

// quoted renders a vocabulary for a refusal message.
func quoted(vocab []string) string {
	out := make([]string, len(vocab))
	for i, v := range vocab {
		out[i] = strconv.Quote(v)
	}
	return strings.Join(out, ", ")
}
