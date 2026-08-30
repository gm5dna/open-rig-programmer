// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7760 "github.com/gm5dna/open-rig-programmer/core/civ/ic7760"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// memorySetStep is the mnemonic this driver reports for its one write
// frame. A LABEL for the audit trail and for a human reading a journal,
// never a token a generic layer branches on (driver.WriteStep's contract).
const memorySetStep = "1A 00"

// ErrUnmappedRegion is the sentinel for ruling E6: the slot's unmapped
// record regions differ from the profile's Fixed template.
var ErrUnmappedRegion = errors.New("ic7760: the slot's unmapped record regions differ from this profile's Fixed template")

// UnmappedRegionError reports E6's refusal, naming exactly which region
// disagreed and by how much.
//
// RULING E6, VERBATIM: a driver may write a slot ONLY when its unmapped
// regions equal the profile's Fixed template; anything else is REFUSED
// with the reason named, NEVER REWRITTEN.
//
// THE COST, stated as E6 requires: A CHANNEL THAT IS IN A SELECT GROUP
// (★1/★2/★3), OR WHOSE DATA MODE IS DATA 1/2/3, CANNOT BE WRITTEN BY THIS
// PROGRAMME AT ALL. It is never silently downgraded to ★1/DATA 1 and never
// silently cleared to OFF. The encoder always writes the template, so
// "preserve what was there" is not available to it — that allow-case was
// STRUCK from E6 REV 2 as unimplementable and as licensing the very
// corruption the ruling forbids.
type UnmappedRegionError struct {
	// Offset is the 0-based record byte whose unmapped region differed.
	Offset int
	// Nibble is "low" or "high" — which half of that byte.
	Nibble string
	// Want and Got are that nibble's value in the template and in the
	// slot's actual record.
	Want byte
	Got  byte
}

func (e *UnmappedRegionError) Error() string {
	what := "an unmapped region"
	switch {
	case e.Offset == 0 && e.Nibble == "low":
		what = "the SELECT-group marker (0=OFF, 1=★1, 2=★2, 3=★3)"
	case e.Offset == 8 && e.Nibble == "high":
		what = "the data mode (0=OFF, 1=DATA 1, 2=DATA 2, 3=DATA 3)"
	}
	return fmt.Sprintf(
		"ic7760: this slot's record byte %d carries %#x in its %s nibble where the profile's Fixed template carries %#x — %s. Ruling E6: a slot may be written ONLY when its unmapped regions equal the template, and anything else is REFUSED with the reason named, never rewritten. This channel cannot be written by this programme at all; it is not downgraded and not cleared",
		e.Offset, e.Got, e.Nibble, e.Want, what)
}

// Unwrap lets errors.Is(err, ErrUnmappedRegion) match.
//
// DELIBERATELY NOT driver.ErrWriteRefused. That sentinel's contract says
// the channel was refused BEFORE ANY WIRE TRAFFIC, and this refusal is
// tier ruling T5's ONE RECORDED EXCEPTION: it necessarily follows the
// single read that obtained the slot's current state. A caller keying on
// ErrWriteRefused and inferring "nothing went out" would be wrong here by
// exactly one read exchange, so this refusal carries its own sentinel and
// says why.
func (e *UnmappedRegionError) Unwrap() error { return ErrUnmappedRegion }

// ErrOutOfDomain is the sentinel for a Known value outside what this
// radio's record can encode.
var ErrOutOfDomain = errors.New("ic7760: a Known value lies outside what this radio's record can encode")

// OutOfDomainError reports a Known numeric value the wire cannot express.
//
// DEFENCE IN DEPTH, AND NOT THE GATE — see doc.go §6. civ.FieldSpan has no
// numeric domain, so civ.Profile.AllowedCommand would ADMIT a set carrying
// 70 MHz. codeplug.Validate already bounds the primary frequency and
// enabler E3 bounds tones, so every path through the model layer is
// covered; these refusals close the driver's own door as well, and
// TestNumericRefusalIsDefenceInDepthNotTheGate asserts the gap that
// remains so that closing it later is a visible test change.
type OutOfDomainError struct {
	// Field is the neutral field whose value was refused.
	Field spec.Field
	// Value is what was asked for, in the field's own neutral unit (hertz
	// for a frequency, tenths of a hertz for a tone).
	Value uint64
	// Max is the largest value this radio's record can encode for it.
	Max uint64
}

func (e *OutOfDomainError) Error() string {
	return fmt.Sprintf(
		"ic7760: %s = %d is outside what this radio's record can encode (maximum %d) — refused before any frame was built. This is a DRIVER-level refusal and NOT the outbound gate: civ.FieldSpan carries no numeric domain, so the gate would admit this frame (see doc.go, the deferred gate-domain gap)",
		e.Field, e.Value, e.Max)
}

// Unwrap lets errors.Is(err, ErrOutOfDomain) match.
func (e *OutOfDomainError) Unwrap() error { return ErrOutOfDomain }

// requestedFields is every spec.Field a write of d would TRANSMIT or
// REQUEST, in ChannelData declaration order.
//
// THE BASE SET IS THIS MODEL'S, DERIVED FROM ITS OWN MATRIX §2 — the
// Yaesu clarifier/ctcss_state/shift trio appears nowhere, because this
// record has no such fields. It is the seven fields the 1A 00 record MAPS.
//
// THE CONDITIONALS ARE WHAT MAKE THE GATE HONEST. A caller who hands
// WriteChannel a Known value this radio cannot express must meet a
// WriteRefusedError NAMING THE FIELD, not a silent drop — so every
// state-bearing field NOT in the base set is appended when Known,
// INCLUDING THE TWO E6 UNMAPS THIS DRIVER REFUSES ON PURPOSE.
// core/codeplug/diff.go is explicit that Known conditional fields are
// never filtered out even when unreachable, which is exactly what makes
// the gate able to REFUSE rather than DROP them.
func requestedFields(d codeplug.ChannelData) []spec.Field {
	fields := []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldTag,
		spec.FieldToneMode,
		spec.FieldToneTx,
		spec.FieldToneRx,
		spec.FieldFilter,
	}
	for _, c := range conditionalRequestedFields {
		if c.present(d) {
			fields = append(fields, c.field)
		}
	}
	return fields
}

// conditionalRequestedFields pairs every state-bearing spec.Field OUTSIDE
// the base set with a predicate reporting whether this channel carries a
// Known value for it. ChannelData declaration order, appended after the
// base set.
//
// Mirrors core/codeplug's own conditional-field table in shape, and is
// mirrored rather than imported for the reason requestedFields gives: the
// BASE half is this model's own matrix reading, so the two halves belong
// in one place, here.
//
// A FIELD MISSING FROM THIS TABLE IS A FIELD THE GATE WOULD NEVER SEE, and
// therefore a Known value that would be silently dropped.
//
// TWO PREDICATES ARE EXACT AND ONE IS AN APPROXIMATION, and the difference
// is worth naming. FieldCTCSSState and FieldShift are plain strings whose
// empty value is not a vocabulary member of any radio, so "non-empty" and
// "the caller asked for something" are the same test. THE CLARIFIER IS
// NOT: ClarHz 0 with both flags false is indistinguishable from a channel
// that never carried a clarifier at all, so a caller asking for an
// explicitly-zero clarifier is not appended and the gate never sees it.
// On THIS model that costs nothing — the 1A 00 record has no clarifier
// span, and core/codeplug's own touchedFields treats clarifier as one of
// the unconditional six and FILTERS IT OUT on a bank that cannot reach it,
// so the clone service would not request it either. Recorded as the known
// bound of this table rather than papered over; it is doc.go's honesty row.
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
	{spec.FieldTxFrequency, func(d codeplug.ChannelData) bool { return d.TxFreqHz.State == codeplug.Known }},
	{spec.FieldDuplex, func(d codeplug.ChannelData) bool { return d.Duplex.State == codeplug.Known }},
	{spec.FieldOffset, func(d codeplug.ChannelData) bool { return d.OffsetHz.State == codeplug.Known }},
	{spec.FieldDTCSCode, func(d codeplug.ChannelData) bool { return d.DTCSCode.State == codeplug.Known }},
	{spec.FieldDTCSPolarity, func(d codeplug.ChannelData) bool { return d.DTCSPolarity.State == codeplug.Known }},
	{spec.FieldDataMode, func(d codeplug.ChannelData) bool { return d.DataMode.State == codeplug.Known }},
}

// refused builds the neutral refusal, with an empty (never nil) Steps
// slice: a refusal that happens before the frames are built has no
// sequence to describe.
func refused(slot string, fields []spec.Field, reason string) (driver.WriteResult, error) {
	return driver.WriteResult{Steps: []driver.WriteStep{}},
		&driver.WriteRefusedError{Slot: slot, Fields: fields, Reason: reason}
}

// WriteChannel implements driver.Session: ONE acknowledged 1A 00 memory
// set, preceded by ONE read.
//
// THE LADDER, IN TIER RULING T5's ORDER:
//
//	LOCALLY DECIDABLE (all of these precede ALL wire traffic):
//	  1. erase?                Channel.Data == nil            -> ErrWriteRefused
//	  2. capability gate       requestedFields x FieldSupport -> ErrWriteRefused
//	  3. field-state shape     non-Known mandatory field      -> ErrWriteRefused
//	  3b. vocabularies         mode/filter/tone_mode not expressible -> ErrWriteRefused
//	  4. numeric domains       freq > 69_999_999, tone > 2999 -> *OutOfDomainError
//	  -----------------------------------------------------------------------
//	  5. ONE read              readRaw (T2 address check, T4 rejection branch)
//	  -----------------------------------------------------------------------
//	READ-DEPENDENT (the single recorded exception):
//	  6. E6 unmapped regions   record byte 0, byte 8's high nibble -> *UnmappedRegionError
//	  7. tone source           UPDATE: the just-read bytes; CREATE: refuse
//	  -----------------------------------------------------------------------
//	  8. BuildMemorySet -> the outbound gate -> the acknowledged exchange
//
// RUNGS 1-4 (3b INCLUDED) ARE THE REASON A REFUSED WRITE PUTS **ZERO** BYTES ON THE
// WIRE; rungs 6-7 are the reason an E6 refusal costs exactly ONE READ'S
// WORTH and no set frame. Both facts are asserted on the scripted port's
// byte log, which is what actually proves them.
//
// THE WRITE IS NEVER RESENT. civ.CIVWriteWithAckSpec fixes RetryReads at
// zero and Engine.Do refuses a non-zero value on ClassWriteWithAck before
// writing anything (safety obligation 2): a lost write's outcome is
// genuinely ambiguous, and resending one is how a radio ends up written
// twice.
//
// NO READ-BACK VERIFICATION. That is the clone service's job, and this
// driver deliberately does not do it.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	// RUNG 1 — ERASE. ChannelData has NO Erase member: erase is
	// represented solely by Channel.Data == nil, "the SOLE discriminator
	// between empty and populated" (core/codeplug/channel.go). FieldErase
	// carries the zero FieldSupport in both profiles and
	// spec.ConsentUnverifiedWrites structurally never consents it, so
	// this refusal stands under consent too.
	//
	// UNLIKE YAESU, THE WIRE FORM EXISTS ON THIS RADIO, IN TWO SHAPES —
	// see doc.go §7 — and no builder for either exists in this tier.
	if ch.Data == nil {
		return refused(ch.Slot, []spec.Field{spec.FieldErase},
			"this radio's memory-clear forms are printed in its document but no IC-7760 has ever been asked to use one, so FieldErase carries the zero FieldSupport and consent structurally never reaches it (spec D4 \"Erase\"; matrix §3.13)")
	}
	d := *ch.Data

	a, bank, err := slotToAddress(ch.Slot)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}}, fmt.Errorf("ic7760: WriteChannel: %w", err)
	}

	// RUNG 2 — THE CAPABILITY GATE. Every field this write would transmit
	// or request must be writable on THIS BANK under THIS SESSION's
	// effective capabilities. Defence in depth below the clone service,
	// which has already made the same check.
	var unwritable []spec.Field
	for _, f := range requestedFields(d) {
		if !s.caps.FieldSupport(bank, f).CanWrite() {
			unwritable = append(unwritable, f)
		}
	}
	if len(unwritable) > 0 {
		return refused(ch.Slot, unwritable,
			"this session cannot write these fields on bank "+string(bank)+": either no IC-7760 has ever been written to by this project and no consent was recorded, or the field is one ruling E6 leaves UNMAPPED (scan_skip, data_mode) and therefore one this driver refuses rather than collapses 4->2")
	}

	// RUNG 3 — FIELD-STATE SHAPE. Every MAPPED field must have something
	// to encode, and this rung is what makes the driver the first
	// detector rather than the builder: civ's encodeRecord returns a
	// codec error for an absent mapped field, and by the time it ran the
	// preservation read would already have put traffic on the wire. T5
	// forbids that.
	//
	// The tone spans are NOT here: their source depends on whether this
	// is an UPDATE or a CREATE, which cannot be known without the read.
	// Rung 7 settles them.
	if missing, reason := missingMandatory(d); missing != "" {
		return refused(ch.Slot, []spec.Field{missing}, reason)
	}

	// RUNG 3b — VOCABULARIES. Ruling T5 names them among the refusals
	// that precede ALL wire traffic, and they are locally decidable: the
	// three enum vocabularies are on this session's own capabilities.
	if field, reason := outsideVocabulary(d, s.caps); field != "" {
		return refused(ch.Slot, []spec.Field{field}, reason)
	}

	// RUNG 4 — NUMERIC DOMAINS. Defence in depth and NOT the gate; see
	// OutOfDomainError and doc.go §6.
	if d.FreqHz > MaxEncodableFreqHz {
		return driver.WriteResult{Steps: []driver.WriteStep{}},
			&OutOfDomainError{Field: spec.FieldFrequency, Value: d.FreqHz, Max: MaxEncodableFreqHz}
	}
	for _, tt := range []struct {
		field spec.Field
		tone  codeplug.ToneField
	}{
		{spec.FieldToneTx, d.ToneTx},
		{spec.FieldToneRx, d.ToneRx},
	} {
		if tt.tone.State == codeplug.Known && uint64(tt.tone.Value) > MaxToneDeciHz {
			return driver.WriteResult{Steps: []driver.WriteStep{}},
				&OutOfDomainError{Field: tt.field, Value: uint64(tt.tone.Value), Max: MaxToneDeciHz}
		}
	}

	// RUNG 5 — THE ONE READ. E6 needs the slot's RAW unmapped regions and
	// T1(4) needs its tone numbers; both come from this single exchange,
	// through the same primitive ReadChannel uses, so the T2 address
	// check and the T4 rejection branch cannot drift.
	prior, raw, empty, err := s.readRaw(ctx, a)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}},
			fmt.Errorf("ic7760: WriteChannel %s: the pre-write preservation read: %w", ch.Slot, err)
	}

	// RUNG 6 — THE E6 UNMAPPED-REGION CHECK. An EMPTY slot has no
	// unmapped regions to compare and the write proceeds against the
	// template.
	if !empty {
		if e := unmappedRegionsDiffer(raw); e != nil {
			return driver.WriteResult{Steps: []driver.WriteStep{}}, e
		}
	}

	// RUNG 7 — THE TONE SOURCE (tier ruling T1(4) and T1(5)).
	toneTx, ok := toneSource(d.ToneTx, prior.ToneTXDeciHz, empty)
	if !ok {
		return refused(ch.Slot, []spec.Field{spec.FieldToneTx}, toneRefusalReason(empty))
	}
	toneRx, ok := toneSource(d.ToneRx, prior.ToneRXDeciHz, empty)
	if !ok {
		return refused(ch.Slot, []spec.Field{spec.FieldToneRx}, toneRefusalReason(empty))
	}

	// RUNG 8 — BUILD, GATE, EXCHANGE. BuildMemorySet writes the profile's
	// Fixed template into every unmapped region and every mapped span
	// from this record, so the 25 record bytes are always sent in full
	// (register entry ic7760-write-full-record: the
	// document prints one full-record form and one three-index clear form
	// and never says whether a short record is accepted).
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
	cmd, err := civic7760.Profile().BuildMemorySet(rec)
	if err != nil {
		return driver.WriteResult{Steps: []driver.WriteStep{}},
			fmt.Errorf("ic7760: WriteChannel %s: building the 1A 00 set: %w", ch.Slot, err)
	}

	steps := []driver.WriteStep{{Command: memorySetStep}}
	p := civic7760.Profile()
	_, err = s.eng.Do(ctx, cmd, civ.CIVWriteWithAckSpec(p.AcknowledgementMatcher()))
	switch {
	case err == nil:
		// FB arrived. Register entry ic7760-write-full-record: the FA/FB
		// CODES are MANUAL-EVIDENCED (PDF p.3, folio 2, "About the data
		// format"), but that a 1A 00 SET is answered by one of them is
		// ASSUMED, and that entry's Stage W lift is what settles it — the
		// nearest printed
		// statement is about command 29, which this driver does not read
		// across.
		steps[0].Sent = true
		steps[0].Confirmed = true
		return driver.WriteResult{Steps: steps}, nil

	case errors.Is(err, transport.ErrRejected):
		// FA arrived, from THIS radio's address (E1's source-address
		// check). An explicit rejection is an ATTRIBUTABLE outcome, so
		// Sent is true and Confirmed is false.
		steps[0].Sent = true
		return driver.WriteResult{Steps: steps},
			fmt.Errorf("ic7760: WriteChannel %s: the radio rejected the %s set: %w", ch.Slot, memorySetStep, err)

	default:
		// AN ACK TIMEOUT REPORTS AN UNKNOWN OUTCOME, and BOTH FLAGS ARE
		// FALSE. core/driver/driver.go defines a false Sent as "the
		// outcome is NOT known-clean … a transport-level failure left
		// its outcome unknowable", which is exactly this. Reporting
		// Sent: true would put a false ATTRIBUTABLE outcome in the audit
		// trail. What proves the never-retransmit rule is not this flag
		// but the port's byte log showing exactly ONE write.
		return driver.WriteResult{Steps: steps},
			fmt.Errorf("ic7760: WriteChannel %s: the %s set was written and never acknowledged, so its outcome is unknown: %w", ch.Slot, memorySetStep, err)
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
func missingMandatory(d codeplug.ChannelData) (spec.Field, string) {
	if d.Mode == "" {
		return spec.FieldMode, "the record's ⑨ is a mode enum with no \"leave it alone\" encoding, and this project refuses rather than synthesises a value"
	}
	if d.Filter.State != codeplug.Known {
		return spec.FieldFilter, "the record's ⑩ is a filter enum with no \"leave it alone\" encoding; the page prints three values and no default, and inventing a fourth would be a radio claim"
	}
	if d.ToneMode.State != codeplug.Known {
		return spec.FieldToneMode, "the record's ⑪ low nibble is a tone-mode enum with no \"leave it alone\" encoding"
	}
	if len(d.Tag) > 0 {
		caps := civic7760.NameCharset
		for i := 0; i < len(d.Tag); i++ {
			found := false
			for j := 0; j < len(caps); j++ {
				if caps[j] == d.Tag[i] {
					found = true
					break
				}
			}
			if !found {
				return spec.FieldTag, fmt.Sprintf("the tag carries byte %#02x at index %d, which is not in this radio's printed memory-name charset (PDF p.20, folio 19, the two \"Codes for character entries\" tables)", d.Tag[i], i)
			}
		}
	}
	if len(d.Tag) > civic7760.Profile().NameLength() {
		return spec.FieldTag, fmt.Sprintf("the tag is %d characters and this radio's name span ⑱~㉗ holds %d", len(d.Tag), civic7760.Profile().NameLength())
	}
	return "", ""
}

// unmappedRegionsDiffer applies ruling E6 to this model's unmapped record
// regions, in a fixed order so the refusal a caller meets is deterministic.
//
// THE UNMAPPED SET IS THE PLAN'S ONE TABLE: record byte 0 ENTIRELY —
// printed ③, whose HIGH nibble is the page's Fixed 0 and whose LOW nibble
// is the four-valued SELECT-group marker — and record byte 8's HIGH
// nibble, printed ⑪'s left nibble, the four-valued data mode. Byte 8's LOW
// nibble is tone_mode and IS mapped.
//
// The profile's two unmapped regions are its zero-valued Fixed template at
// record offsets 0 and 8 (Stage 1 layout, E6).
func unmappedRegionsDiffer(raw []byte) error {
	// The IC-7760 profile's two unmapped regions are the zero-valued
	// template at record offsets 0 and 8 (Stage 1 layout, E6).
	tmpl := make([]byte, civic7760.RecordOnlyLength)
	if len(raw) < len(tmpl) {
		// UNREACHABLE, and it says so rather than inventing a nibble.
		// civ.Profile.MemoryAnswerRecord has already refused any length
		// but this profile's own — that check is the probe's CONTINUOUS
		// length fingerprint and readRaw cannot return past it — so a
		// short record here means the fingerprint invariant has been
		// broken upstream. A refusal that named a nibble would print
		// "byte 0 carries 0x0 where the template carries 0x0", which is
		// a lie about a record that was never measured.
		return fmt.Errorf("ic7760: internal: the E6 comparison was handed a %d-byte record where the profile declares %d — the length fingerprint should have refused this answer before it reached here", len(raw), len(tmpl))
	}
	for _, chk := range []struct {
		offset int
		nibble string
		mask   func(byte) byte
	}{
		{0, "high", func(b byte) byte { return b >> 4 }},
		{0, "low", func(b byte) byte { return b & 0x0F }},
		{8, "high", func(b byte) byte { return b >> 4 }},
	} {
		want, got := chk.mask(tmpl[chk.offset]), chk.mask(raw[chk.offset])
		if want != got {
			return &UnmappedRegionError{Offset: chk.offset, Nibble: chk.nibble, Want: want, Got: got}
		}
	}
	return nil
}

// toneRefusalReason picks the reason that is TRUE of the write in hand.
//
// The two arms are different facts and must not share a sentence. On a
// CREATE there is genuinely no prior record to preserve from; on an UPDATE
// there IS one and it carried no tone span, which on this single-layout
// profile cannot happen — so saying "no prior record" there would be a
// lie told to a user in the one situation where the code is already
// somewhere it should not be.
func toneRefusalReason(create bool) string {
	if create {
		return noDefaultToneReason
	}
	return "this slot's record carried no tone span to preserve, which this profile's single 25-byte layout makes impossible: it maps ⑫~⑭ and ⑮~⑰ unconditionally, so a successfully parsed record always carries both. Reaching this refusal means the record layout and this driver's mapping have gone out of step — report it rather than working around it"
}

// noDefaultToneReason is T1(5)'s refuse arm, in words, and it is this
// model's arm because of an ABSENCE that was actually looked for.
const noDefaultToneReason = "this slot has no prior record to preserve a tone from, and THIS RADIO'S DOCUMENT PRINTS NO DEFAULT TONE VALUE — all 28 pages of the revision 2 guide were swept for the matrix and no \"Default:\" line appears against the tone frequency; the only tone material is the 1B 00 / 1B 01 digit strip on PDF p.24 (folio 23). Tier ruling T1(5)'s REFUSE arm rather than its default arm; register entry ic7760-tone-domain, whose lift reads the lowest and highest tone the front panel offers on an IC-7760"

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
//     span has no source, and this model's document prints no default —
//     see noDefaultToneReason.
func toneSource(want codeplug.ToneField, prior civ.Optional[uint64], create bool) (uint64, bool) {
	if want.State == codeplug.Known {
		return uint64(want.Value), true
	}
	if create {
		return 0, false
	}
	// Unreachable-with-false through this profile: the layout maps both
	// tone spans, so a parsed record always carries both. Refused rather
	// than defaulted to zero, which would be synthesis.
	v, ok := prior.Get()
	return v, ok
}

// outsideVocabulary reports the first mapped enum field carrying a Known
// value this radio cannot express, and why.
//
// RULING T5 NAMES VOCABULARIES among the refusals that precede ALL wire
// traffic — "capabilities, field validity, vocabularies, cross-field
// constraints, mandatory-Known rules" — and this check is locally
// decidable: the three vocabularies are on the session's own capabilities,
// with nothing to look up on the radio.
//
// LEFT TO THE BUILDER, THE FAILURE IS WRONG IN THREE WAYS AT ONCE.
// civ's encodeRecord does refuse an out-of-vocabulary enum value, but by
// then the T5 preservation read has already put a frame on the wire; the
// error is a wrapped CODEC error rather than a *driver.WriteRefusedError;
// and a caller keying on driver.ErrWriteRefused — the neutral contract's
// refusal sentinel, and what every other rung returns — sees nothing. It
// is the same argument that put the tag's charset check in
// missingMandatory, applied to the three fields whose builder-side failure
// is identical in kind.
//
// THE VOCABULARIES ARE THE CAPABILITIES', NOT THE CODEC'S, and that is
// deliberate: caps.go writes the ten mode names and three filter names out
// because core/civ/ic7760's enum tables are unexported, and
// TestModes_MatchTheCodec and TestFilters_MatchTheCodec pin the two equal.
// Asking the capabilities here means the value the UI offered, the value
// codeplug.Validate judged and the value this rung admits are one list.
//
// FREQUENCY IS NOT HERE because it is not a vocabulary: its domain is a
// RANGE, checked at rung 4 against MaxEncodableFreqHz (MinFreqHz is zero
// and FreqHz is unsigned, so there is no floor to check). The other
// FreqField-shaped members — TxFreqHz and OffsetHz — have no span in this
// record at all, so a Known value for either is refused one rung earlier,
// by the capability gate, and never reaches a domain question.
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
		{spec.FieldMode, d.Mode, caps.Modes,
			"the record's ⑨ is a mode enum, and PDF p.18 (folio 17)'s \"Operating mode\" table, column ①, prints these ten codes and no more"},
		{spec.FieldFilter, d.Filter.Value, caps.Filters,
			"the record's ⑩ is a filter enum, and PDF p.18 (folio 17)'s \"Operating mode\" table, column ② \"Filter setting\", prints these three values and no default"},
		{spec.FieldToneMode, d.ToneMode.Value, toneModes,
			"the record's ⑪ low nibble is a tone-mode enum, and PDF p.20 (folio 19)'s ⑪ sub-diagram prints these three values"},
	} {
		if containsVocab(v.vocab, v.value) {
			continue
		}
		return v.field, fmt.Sprintf(
			"%q is not a value this radio can express (%s: %s). A Known value the wire cannot say faithfully is REFUSED, never dropped and never mapped to a neighbour",
			v.value, v.where, quoted(v.vocab))
	}
	return "", ""
}

// containsVocab is exact string membership. NOT case-folded and not
// trimmed: a vocabulary value is a wire code's agreed spelling, and
// quietly accepting "usb" for "USB" would be this driver deciding what a
// user meant.
func containsVocab(vocab []string, value string) bool {
	for _, v := range vocab {
		if v == value {
			return true
		}
	}
	return false
}

// quoted renders a vocabulary for a refusal message.
func quoted(vocab []string) string {
	out := make([]string, len(vocab))
	for i, v := range vocab {
		out[i] = strconv.Quote(v)
	}
	return strings.Join(out, ", ")
}
