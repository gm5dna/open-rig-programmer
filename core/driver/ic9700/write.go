// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700

import (
	"context"
	"fmt"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// requestedFields lists every spec.Field a write of data actually
// requests: THREE unconditionally, plus every conditional the channel
// carries a Known value for.
//
// THE UNCONDITIONAL SET IS THIS MODEL'S, NOT THE YAESU FAMILY'S, and that
// is the one thing about this function most likely to be got wrong by
// analogy. core/driver/ftdx101's requestedFields opens with SIX —
// frequency, mode, clarifier, CTCSS state, shift, tag — because its
// combined MT Set carries all six in one frame whether or not any of them
// changed. On this radio the middle three do not exist: the record carries
// no clarifier (matrix §1 #6/#7), and this family expresses duplex and
// tone_mode where Yaesu expresses shift and ctcss_state. They are
// Unsupported on every bank here, and the very next rung of the ladder
// capability-checks every requested field — so the Yaesu six, copied
// across, would have REFUSED EVERY IC-9700 WRITE. The three that remain
// are this model's §2 grid: the fields graded Unverified on MEM that a
// memory set always carries.
//
// THE CONDITIONALS INCLUDE THE SIX THIS RADIO DOES NOT SUPPORT, and that
// is deliberate rather than an oversight. The Wave-1 C2 contract says a
// Known value is a REQUEST: silently dropping one the caller explicitly
// marked Known would be a lie, so a Known clarifier, CTCSS state, CTCSS
// tone, shift, tag display or scan skip is REQUESTED here and then
// REFUSED BY NAME at the capability gate. A channel this driver itself
// produced carries none of them (read.go reports every one Unavailable),
// so the ordinary write is untouched by their presence.
//
// The ORDER is ChannelData's declaration order, pre-tier set then tier
// set, mirroring the diff layer's own requested-set derivation so that
// this driver's defence-in-depth gate and the layer above judge the same
// set for the same channel.
func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := []spec.Field{
		spec.FieldFrequency,
		spec.FieldMode,
		spec.FieldTag,
	}
	for _, c := range conditionalFields {
		if c.present(data) {
			fields = append(fields, c.field)
		}
	}
	return fields
}

// conditionalFields pairs each conditionally-requested Field with the
// predicate reporting whether this channel's data REQUESTS it.
//
// The six pre-tier entries come first, then the ten the Icom tier added,
// in ChannelData's own declaration order.
//
// THE FIRST THREE PREDICATES ARE NOT FieldState CHECKS, because those
// three fields are not tri-state: ClarHz/RxClar/TxClar, CTCSS and Shift
// are plain values on ChannelData, held over from before the tri-state
// pattern existed. "Known" for them can only mean "carries content", so a
// zero clarifier and an empty CTCSS or shift string are read as absent —
// which is exactly what read.go produces for this radio, and what a file
// written for it holds.
var conditionalFields = []struct {
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
}

// noSteps is the empty, non-nil Steps slice a refusal reports: the write
// was refused before any frame was built, so there is no sequence to
// describe.
func noSteps() driver.WriteResult { return driver.WriteResult{Steps: []driver.WriteStep{}} }

// refuse builds this package's one refusal type.
func refuse(slot string, fields []spec.Field, format string, args ...any) error {
	return &driver.WriteRefusedError{
		Slot:   slot,
		Fields: fields,
		Reason: fmt.Sprintf(format, args...),
	}
}

// WriteChannel implements driver.Session: ONE acknowledged memory set,
// after a ladder of refusals that is the whole design of this driver.
//
// THE ORDER IS T5'S AND IS NOT REARRANGEABLE. Rungs 1–5 are LOCALLY
// DECIDABLE and precede ALL wire traffic; rung 6 is the SINGLE read; rungs
// 7–8 are the two read-dependent refusals — the one recorded exception to
// "refusal before any wire traffic", because a driver cannot know a slot's
// unmapped bytes without reading it; and only then does rung 9 write.
//
//  1. the slot becomes an address and a bank, or the write is refused;
//  2. every requested field is re-checked against THIS session's
//     capabilities — defence in depth below the clone service;
//  3. every Known value is checked against this radio's OWN vocabularies
//     and numeric domains, through the landed typed validators;
//  4. the manual's three cross-field constraints are checked against the
//     incoming data and the band the slot names;
//  5. the single read;
//  6. the E6 template guard: the unmapped regions must equal the
//     profile's Fixed template, ④ must read OFF, and ⑬ must not read RPS;
//  7. the CREATE mandatory-field check, which is decidable only once the
//     read has said whether the slot is empty;
//  8. the merge — Known values from the caller, everything else from THIS
//     READ's record, verbatim;
//  9. BuildMemorySet, and one ClassWriteWithAck exchange.
//
// NOTHING IS SYNTHESISED, ANYWHERE. A field that is not Known on a MODIFY
// carries the just-read record's own civ-layer value; on a CREATE there is
// no such value and the write is REFUSED naming the field. No default is
// invented and no value is taken from a cache that outlived its read.
//
// WHAT THIS REFUSES, stated because it is the design rather than a
// limitation to apologise for: any channel whose DV call signs differ from
// the template's, any with digital squelch set, any in a SELECT-memory
// star group, any set to RPS. Every one is a refusal with the reason
// named, before any frame is built — which is also why a wrong Fixed
// template could only ever cost MORE REFUSALS and never corruption.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	// Rung 1. An EMPTY channel is not a write this tier can perform: it
	// would be an erase, and erase is unshipped — no builder names the
	// clear frame and the gate has no branch that could admit one.
	if ch.Data == nil {
		return noSteps(), refuse(ch.Slot, nil,
			"this channel is empty, which would be an erase; the IC-9700's clear form is documented and deliberately not shipped by this tier")
	}
	addr, bank, err := slotAddress(ch.Slot)
	if err != nil {
		return noSteps(), err
	}
	data := *ch.Data

	// Rung 2: the capability re-check, per requested field.
	requested := requestedFields(data)
	var unwritable []spec.Field
	for _, f := range requested {
		if !s.caps.FieldSupport(bank, f).CanWrite() {
			unwritable = append(unwritable, f)
		}
	}
	if len(unwritable) > 0 {
		return noSteps(), refuse(ch.Slot, unwritable,
			"this session cannot write %s on bank %s", fieldList(unwritable), bank)
	}

	// Rung 3: Known-value validity, against this radio's own vocabularies
	// and domains.
	if err := s.validateKnownValues(ch.Slot, data); err != nil {
		return noSteps(), err
	}

	// Rung 4: the manual's cross-field constraints, on the INCOMING data.
	if err := s.crossFieldRefusal(ch.Slot, addr, data.Mode, knownString(data.Duplex)); err != nil {
		return noSteps(), err
	}

	// Rung 5: THE SINGLE READ. Its own T2 address check runs inside, and
	// a mismatch aborts the write rather than letting one slot's state
	// validate another's.
	current, record, stored, err := s.readChannelRaw(ctx, ch.Slot)
	if err != nil {
		return noSteps(), err
	}

	if current.Data != nil {
		// Rung 6: the read-dependent refusals (E6).
		if err := s.templateGuard(ch.Slot, record, stored); err != nil {
			return noSteps(), err
		}
	}

	// Rung 7 and rung 8: build the record to send. On a CREATE every
	// mapped field must have arrived Known; on a MODIFY the gaps are
	// filled from THIS read's record, verbatim.
	rec, err := s.mergedRecord(ch.Slot, addr, data, current.Data != nil, stored)
	if err != nil {
		return noSteps(), err
	}

	// The cross-field constraints again, on the EFFECTIVE values — the
	// merge can pair an incoming Known mode with a preserved duplex the
	// caller never mentioned. Defence in depth: the manual's constraints
	// are about the RECORD, and this is the last point at which the
	// record's own values are known.
	if err := s.crossFieldRefusal(ch.Slot, addr, stringOf(rec.Mode), stringOf(rec.Duplex)); err != nil {
		return noSteps(), err
	}

	cmd, err := s.profile.BuildMemorySet(rec)
	if err != nil {
		return noSteps(), fmt.Errorf("ic9700: WriteChannel %s: %w", ch.Slot, err)
	}

	// Rung 9. The steps are declared once the frame is built and before
	// it goes out, so an aborted sequence reads as "this frame was part
	// of the plan and never attributably went out".
	steps := []driver.WriteStep{{Command: "1A 00"}}
	result := driver.WriteResult{Steps: steps}

	// ClassWriteWithAck: Do waits for the six-byte address-checked FB and
	// NEVER retransmits — the helper sets RetryReads to zero and Do
	// refuses any other value on this class before writing anything. A
	// timeout leaves the write's outcome genuinely unknown, and the one
	// thing that must not happen is sending it again to find out.
	if _, err := s.eng.Do(ctx, cmd, civ.CIVWriteWithAckSpec(s.profile.AcknowledgementMatcher())); err != nil {
		return result, fmt.Errorf("ic9700: WriteChannel %s: %w", ch.Slot, err)
	}
	result.Steps[0].Sent = true
	result.Steps[0].Confirmed = true
	return result, nil
}

// fieldList renders a field list for a refusal message.
func fieldList(fields []spec.Field) string {
	names := make([]string, len(fields))
	for i, f := range fields {
		names[i] = string(f)
	}
	return strings.Join(names, ", ")
}

// knownString is a StringField's value when it is Known, and "" otherwise.
func knownString(f codeplug.StringField) string {
	if f.State != codeplug.Known {
		return ""
	}
	return f.Value
}

// validateKnownValues is the ladder's rung 3: every Known value checked
// against THIS radio's own vocabulary or domain, through the landed typed
// validators rather than a local re-implementation of them.
//
// IT IS LOAD-BEARING RATHER THAN BELT-AND-BRACES, and RPS is the proof.
// The dialect's ⑬ enum carries all four printed high-nibble values so a
// record round-trips exactly, so BuildMemorySet would encode a Known
// Duplex of "RPS" perfectly happily and the gate would admit the frame.
// The only thing that knows RPS is not a value this CODEPLUG can express
// is caps.DuplexOptions, and this is where it is asked. The same reasoning
// covers the numeric domains: neither the codec nor the builder would
// object to a 300 Hz tone or a DTCS code with an 8 in it.
//
// ONLY A Known VALUE IS JUDGED, and that is not laxity. The three
// FieldStates say what a value MEANS: Known is a request, and Unknown and
// Unavailable both mean "preserve whatever the radio has", which is
// nothing to validate. The zero state — codeplug.Absent, which a
// hand-built ChannelData leaves behind — is the same case in practice: a
// caller who set nothing has requested nothing. Validating a non-Known
// field would refuse every ordinary MODIFY, since core/codeplug's own
// typed validators reject Absent outright.
//
// Mode is checked against caps.Modes directly rather than through
// StringField.Valid, because ChannelData.Mode is a plain string rather
// than a tri-state field — it predates the pattern. The question asked is
// the same one.
func (s *Session) validateKnownValues(slot string, data codeplug.ChannelData) error {
	if data.Mode != "" && !contains(s.caps.Modes, data.Mode) {
		return refuse(slot, []spec.Field{spec.FieldMode},
			"mode %q is not one of this radio's modes %v", data.Mode, s.caps.Modes)
	}
	if data.Mode == "" {
		return refuse(slot, []spec.Field{spec.FieldMode},
			"a memory set always carries the mode, and this channel names none")
	}

	if data.FreqHz < s.caps.MinFreqHz || data.FreqHz > s.caps.MaxFreqHz {
		return refuse(slot, []spec.Field{spec.FieldFrequency},
			"frequency %d Hz is outside this radio's %d..%d Hz band plan", data.FreqHz, s.caps.MinFreqHz, s.caps.MaxFreqHz)
	}

	if len(data.Tag) > s.caps.TagLen {
		return refuse(slot, []spec.Field{spec.FieldTag},
			"tag %q is %d characters; this radio's name field holds %d", data.Tag, len(data.Tag), s.caps.TagLen)
	}
	for i := 0; i < len(data.Tag); i++ {
		if !s.caps.TagByteOK(data.Tag[i]) {
			return refuse(slot, []spec.Field{spec.FieldTag},
				"tag %q has byte %#02x at offset %d, which is not in %s", data.Tag, data.Tag[i], i, s.caps.TagCharsetDescription())
		}
	}

	for _, v := range []struct {
		field spec.Field
		f     codeplug.StringField
		vocab []string
	}{
		{spec.FieldDuplex, data.Duplex, duplexValues(s.caps)},
		{spec.FieldToneMode, data.ToneMode, toneModeValues(s.caps)},
		{spec.FieldDTCSPolarity, data.DTCSPolarity, s.caps.DTCSPolarities},
		{spec.FieldFilter, data.Filter, s.caps.Filters},
	} {
		if v.f.State != codeplug.Known {
			continue
		}
		if err := v.f.Valid(v.vocab); err != nil {
			return refuse(slot, []spec.Field{v.field}, "%v", err)
		}
	}

	for _, v := range []struct {
		field spec.Field
		f     codeplug.ToneField
	}{{spec.FieldToneTx, data.ToneTx}, {spec.FieldToneRx, data.ToneRx}} {
		if v.f.State != codeplug.Known {
			continue
		}
		if err := v.f.Valid(s.caps); err != nil {
			return refuse(slot, []spec.Field{v.field}, "%v", err)
		}
	}

	if data.DTCSCode.State == codeplug.Known {
		if err := data.DTCSCode.Valid(s.caps.DTCSCodes); err != nil {
			return refuse(slot, []spec.Field{spec.FieldDTCSCode}, "%v", err)
		}
	}
	return nil
}

// crossFieldRefusal enforces the three constraints the manual states and
// the record cannot express (matrix §3.16 A2; adjudication R11).
//
// All three are MANUAL-EVIDENCED, and they are enforced NOW rather than
// left to the radio:
//
//  1. mode DD "can be selected when setting the 1200 MHz band to other
//     than the satellite mode" — PDF p.14 (folio 13), the footnote under
//     the Operating mode table;
//  2. "RPS can be set when DD mode is selected" — PDF p.15 (folio 14),
//     the ⓘ note under the ⑬ detail box;
//  3. "…and Duplex (−, +) can be set when other than DD mode is
//     selected" — the same note.
//
// THE BAND COMES FROM THE SLOT, which is what makes the check locally
// decidable: this radio's address carries the band, so the constraint is
// answerable before any wire traffic. R21 lifts the radio's REACTION to
// an invalid combination, not the constraints themselves — those are
// printed, and are enforced now.
//
// A duplex of "RPS" reaching here at all means it came from a STORED
// record rather than from a Known incoming value: rung 3 refuses an
// incoming one unconditionally, because caps.DuplexOptions cannot name it.
func (s *Session) crossFieldRefusal(slot string, addr civ.ChannelAddress, mode, duplex string) error {
	const ddBand = 3 // the 1.2 GHz band, wire code 03

	if mode == "DD" && addr.Group != ddBand {
		return refuse(slot, []spec.Field{spec.FieldMode},
			"mode DD can be selected only in the 1200 MHz band, and %s is in the %s MHz band",
			slot, bandNames[addr.Group-1])
	}
	if duplex == "RPS" && mode != "DD" {
		return refuse(slot, []spec.Field{spec.FieldDuplex, spec.FieldMode},
			"RPS can be set only when DD mode is selected, and this channel's mode is %q", mode)
	}
	if (duplex == "DUP+" || duplex == "DUP-") && mode == "DD" {
		return refuse(slot, []spec.Field{spec.FieldDuplex, spec.FieldMode},
			"duplex %s can be set only when the mode is other than DD", duplex)
	}
	return nil
}

// unmappedRegion names one run of record bytes no field span maps, so a
// refusal can say WHICH bytes disagreed rather than only that some did.
type unmappedRegion struct {
	name     string
	from, to int // [from, to)
}

// unmappedRegions is the 52 bytes of this record that have no
// civ.FieldID home, named as the manual names them.
//
// The offsets are the Task-2 layout table's, and the duplicated block's
// copies sit at +47 exactly as every other repeated field does. They are
// listed rather than derived so that a refusal message can name a region;
// templateGuard checks the DERIVED set as well, so a region added to the
// layout and forgotten here still cannot slip past unguarded.
var unmappedRegions = []unmappedRegion{
	{"digital squelch", 10, 11},
	{"DV code squelch", 20, 21},
	{"UR (destination) call sign", 24, 32},
	{"R1 (access repeater) call sign", 32, 40},
	{"R2 (gateway repeater) call sign", 40, 48},
	{"duplicated digital squelch", 57, 58},
	{"duplicated DV code squelch", 67, 68},
	{"duplicated UR call sign", 71, 79},
	{"duplicated R1 call sign", 79, 87},
	{"duplicated R2 call sign", 87, 95},
}

// templateGuard is the E6 rule, and it is the reason this tier can write
// an IC-9700 channel at all without destroying data it cannot represent.
//
// Fifty-two of the 111 record bytes have no civ.FieldID: ⑭, ㉔, the three
// eight-byte call signs, and each of their copies. civ.encodeRecord writes
// every one of them from the layout's Fixed template, so a memory set
// assembled from neutral fields alone would OVERWRITE whatever the channel
// held there. E6 rules the shape: a driver may write a slot ONLY when its
// unmapped regions equal the template; anything else is REFUSED with the
// reason named, never rewritten.
//
// REV 1'S "take them from the freshly-read record" IS STRUCK. R6 forbids
// preservation-by-cache, and the encoder always writes the template — so
// an allow-case for "the just-read bytes it is provably preserving" was
// unimplementable, and read as permission it would have licensed the exact
// corruption the ruling forbids.
//
// TWO MAPPED FIELDS ARE GOVERNED BY THE SAME RULE, because neither has a
// neutral value the caller could have supplied: ④'s four-valued star group
// must read OFF (the neutral field is a boolean — OQ-4), and ⑬'s duplex
// must not read RPS (caps.DuplexOptions has three directions and RPS is
// not one — OQ-6). Anything else is refused, never flattened.
func (s *Session) templateGuard(slot string, record []byte, stored civ.MemoryRecord) error {
	layout, ok := s.profile.LayoutFor(civic9700RecordLength)
	if !ok {
		return fmt.Errorf("ic9700: WriteChannel %s: this profile declares no %d-byte layout", slot, civic9700RecordLength)
	}
	template := layout.Fixed
	if len(template) != len(record) {
		return fmt.Errorf("ic9700: WriteChannel %s: the stored record is %d bytes and the template %d", slot, len(record), len(template))
	}

	mapped := mappedOffsets(layout)
	for _, region := range unmappedRegions {
		for off := region.from; off < region.to; off++ {
			if mapped[off] || record[off] == template[off] {
				continue
			}
			return refuse(slot, nil,
				"the stored channel's %s differs from the one state this tier can write (record byte %d is %#02x, the template's is %#02x); rewriting it would destroy data this codeplug cannot represent",
				region.name, off, record[off], template[off])
		}
	}
	// The derived sweep: any unmapped byte the named regions missed.
	for off := range record {
		if mapped[off] || record[off] == template[off] {
			continue
		}
		return refuse(slot, nil,
			"the stored channel's record byte %d is %#02x where this tier can only write the template's %#02x", off, record[off], template[off])
	}

	if sel := stringOf(stored.Select); sel != "OFF" {
		return refuse(slot, nil,
			"the stored channel is in SELECT-memory group %s, which the neutral scan-skip field cannot express; writing it would silently move the channel out of that group", sel)
	}
	if dup := stringOf(stored.Duplex); dup == "RPS" {
		return refuse(slot, []spec.Field{spec.FieldDuplex},
			"the stored channel is set to RPS, which this codeplug's duplex vocabulary cannot name; writing it would silently change the channel to %q", "OFF")
	}
	return nil
}

// civic9700RecordLength is the record-only length this driver's profile
// declares. It is spelt here rather than imported as a bare 111 so the
// number appears once in this file and reads as what it is.
const civic9700RecordLength = 111

// mappedOffsets marks every record byte at least one field span covers.
//
// DERIVED FROM THE LAYOUT, not restated. It is what makes the unmapped set
// exactly the layout's own complement: a field added to core/civ/ic9700
// narrows this guard automatically, and a field REMOVED widens it, which
// is the safe direction in both cases.
func mappedOffsets(layout civ.RecordLayout) []bool {
	mapped := make([]bool, layout.Length)
	for _, sp := range layout.Fields {
		for i := 0; i < sp.Length && sp.Offset+i < layout.Length; i++ {
			mapped[sp.Offset+i] = true
		}
	}
	return mapped
}

// mergedRecord builds the civ.MemoryRecord a memory set will carry.
//
// ON A MODIFY, a field the caller left non-Known takes the JUST-READ
// record's own civ-layer value, verbatim. That is value-level preservation
// of what the radio itself holds — available only because the E6/T5
// preservation read is already mandatory — and it is NOT synthesis: no
// value is invented, and nothing comes from a cache that outlived the
// read.
//
// ON A CREATE there is no prior record, so nothing can be preserved and
// R6's other form applies: every field the layout maps must arrive Known
// or the write is REFUSED naming the fields that did not. There is no
// partial create.
//
// THE TONE SPANS ARE WHY THE CREATE RULE BITES ON THIS MODEL. T1(5) would
// let a CREATE write a manual-DOCUMENTED default tone when ToneMode is
// Known OFF; this manual documents none — PDF p.21's tone diagram prints
// digit RANGES and nothing else, and leg G's 88.5 Hz is recorded in its
// own provenance as a CHOICE — so the refusal stands. Register entry
// `ic9700-no-documented-default-tone`, lift R24.
//
// ④ IS ALWAYS WRITTEN AS OFF. It has no neutral home at all, so there is
// nothing to take a value from; the template state is OFF, the guard above
// has already refused any stored channel reading otherwise, and the
// encoder writes OFF.
func (s *Session) mergedRecord(slot string, addr civ.ChannelAddress, data codeplug.ChannelData, modify bool, stored civ.MemoryRecord) (civ.MemoryRecord, error) {
	rec := civ.MemoryRecord{
		Address:  addr,
		Select:   civ.Available("OFF"),
		RXFreqHz: civ.Available(data.FreqHz),
		Mode:     civ.Available(data.Mode),
		Name:     civ.Available(data.Tag),
	}

	var missing []spec.Field
	num := func(field spec.Field, f codeplug.FreqField, prior civ.Optional[uint64]) civ.Optional[uint64] {
		if f.State == codeplug.Known {
			return civ.Available(f.Value)
		}
		if modify {
			return prior
		}
		missing = append(missing, field)
		return civ.Optional[uint64]{}
	}
	tone := func(field spec.Field, f codeplug.ToneField, prior civ.Optional[uint64]) civ.Optional[uint64] {
		if f.State == codeplug.Known {
			return civ.Available(uint64(f.Value))
		}
		if modify {
			return prior
		}
		missing = append(missing, field)
		return civ.Optional[uint64]{}
	}
	str := func(field spec.Field, f codeplug.StringField, prior civ.Optional[string]) civ.Optional[string] {
		if f.State == codeplug.Known {
			return civ.Available(f.Value)
		}
		if modify {
			return prior
		}
		missing = append(missing, field)
		return civ.Optional[string]{}
	}

	rec.TXFreqHz = num(spec.FieldTxFrequency, data.TxFreqHz, stored.TXFreqHz)
	rec.OffsetHz = num(spec.FieldOffset, data.OffsetHz, stored.OffsetHz)
	rec.Duplex = str(spec.FieldDuplex, data.Duplex, stored.Duplex)
	rec.ToneMode = str(spec.FieldToneMode, data.ToneMode, stored.ToneMode)
	rec.ToneTXDeciHz = tone(spec.FieldToneTx, data.ToneTx, stored.ToneTXDeciHz)
	rec.ToneRXDeciHz = tone(spec.FieldToneRx, data.ToneRx, stored.ToneRXDeciHz)
	rec.DTCSPolarity = str(spec.FieldDTCSPolarity, data.DTCSPolarity, stored.DTCSPolarity)
	rec.Filter = str(spec.FieldFilter, data.Filter, stored.Filter)

	if data.DTCSCode.State == codeplug.Known {
		rec.DTCSCode = civ.Available(uint64(data.DTCSCode.Value))
	} else if modify {
		rec.DTCSCode = stored.DTCSCode
	} else {
		missing = append(missing, spec.FieldDTCSCode)
	}

	switch {
	case data.DataMode.State == codeplug.Known:
		rec.DataMode = civ.Available(dataModeWire(data.DataMode.Value))
	case modify:
		rec.DataMode = stored.DataMode
	default:
		missing = append(missing, spec.FieldDataMode)
	}

	if len(missing) > 0 {
		return civ.MemoryRecord{}, refuse(slot, missing,
			"this slot is empty, so there is nothing to preserve: creating a channel needs a value for %s, and this tier invents none", fieldList(missing))
	}
	return rec, nil
}

// dataModeWire is the neutral boolean as ⑫'s printed spelling.
func dataModeWire(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}

// contains reports whether list holds v.
func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
