// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300mk2

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// unconditionalFields are the SEVEN spec.Fields the 1A 00 record ALWAYS
// carries, and which this driver therefore always encodes — changed or not,
// because the record has no way to omit a field.
//
// THE YAESU TRIO IS NOT HERE. clarifier, ctcss_state and shift are graded
// Unsupported on every bank of this model (caps.go), so requesting them
// unconditionally would make the gate refuse every write this driver could
// ever make.
var unconditionalFields = []spec.Field{
	spec.FieldFrequency,
	spec.FieldMode,
	spec.FieldTag,
	spec.FieldTxFrequency,
	spec.FieldFilter,
	spec.FieldDataMode,
	spec.FieldToneMode,
}

// conditionalFields pairs each remaining spec.Field with the predicate its
// OWN representation permits, in codeplug.ChannelData's declaration order.
//
// THE PREDICATES ARE NOT ALL ".State == Known", and they cannot be. In the
// landed codeplug.ChannelData, ClarHz/RxClar/TxClar, CTCSS, Shift, Tag, Mode
// and FreqHz are PLAIN SCALARS with no FieldState at all, so for the three
// scalar-backed rows below the only question a scalar can answer is "did the
// caller actually set this?" — and that is the question asked. A channel
// this driver produced answers false to all three, so an ordinary write is
// unaffected; a channel loaded from a file written for a DIFFERENT radio
// answers true, and is refused BY NAME rather than silently dropped.
//
// FieldErase is ABSENT BY DESIGN. It has no ChannelData member — erasure IS
// Channel.Data == nil — so it cannot be derived from a channel's content at
// all, and it belongs to WriteChannel's third rung.
//
// tone_tx and tone_rx are here rather than among the mandatory fields
// (ruling T1(4)): a non-Known tone is PRESERVED from the just-read record,
// so requesting it would ask the gate about a value the caller never set.
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
	{spec.FieldDuplex, func(d codeplug.ChannelData) bool { return d.Duplex.State == codeplug.Known }},
	{spec.FieldOffset, func(d codeplug.ChannelData) bool { return d.OffsetHz.State == codeplug.Known }},
	{spec.FieldToneTx, func(d codeplug.ChannelData) bool { return d.ToneTx.State == codeplug.Known }},
	{spec.FieldToneRx, func(d codeplug.ChannelData) bool { return d.ToneRx.State == codeplug.Known }},
	{spec.FieldDTCSCode, func(d codeplug.ChannelData) bool { return d.DTCSCode.State == codeplug.Known }},
	{spec.FieldDTCSPolarity, func(d codeplug.ChannelData) bool { return d.DTCSPolarity.State == codeplug.Known }},
}

// requestedFields lists every spec.Field a write of data actually REQUESTS:
// the seven unconditional ones, then whichever conditionals the channel
// carries, in ChannelData's own declaration order.
//
// It is this driver's defence-in-depth gate input, and it mirrors the diff
// layer's requested-set derivation so that the two judge the same set for the
// same channel.
func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := make([]spec.Field, 0, len(unconditionalFields)+len(conditionalFields))
	fields = append(fields, unconditionalFields...)
	for _, c := range conditionalFields {
		if c.present(data) {
			fields = append(fields, c.field)
		}
	}
	return fields
}

// WriteChannel implements driver.Session: ONE acknowledged 1A 00 set,
// preceded by exactly one read.
//
// THE LADDER IS ORDERED, and doc.go states it in full. Rungs 1–6 are LOCALLY
// DECIDABLE and precede ALL wire traffic; rung 7 is the single read; rung 8
// is the answer's address check and the three READ-DEPENDENT refusals, which
// are the one recorded exception to driver.Session's "refusal before any wire
// traffic" sentence — a refusal that depends on the SLOT'S CURRENT STATE
// cannot precede the read that obtains that state; rung 9 is the set.
//
// THE NUMBERING IS D21'S AS RECLASSIFIED, not as first written. D21 listed the
// SCAN bank's ③-must-be-zero constraint seventh, among the locally decidable
// rungs, and it is not locally decidable: the value it judges is record byte
// ③'s SELECT nibble, which no spec.Field carries, so it exists nowhere but the
// record the radio has just handed back. It moved down beside E6's own check,
// and every rung after it moved up by one. doc.go carries the reasoning; this
// comment carries the numbers, and the two must not drift.
//
// NO READ-BACK VERIFICATION. Reading the slot back and comparing is the
// clone service's job, exactly as for every other driver here: verification
// policy lives in one place above every driver rather than half-implemented
// inside each.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	// Every refusal below returns res unchanged, i.e. an EXPLICITLY EMPTY
	// step list — never nil. The clone service journals this result, and a
	// nil slice marshals as JSON null, which an auditor would have to read as
	// "unknown" rather than the truth, "no frame was ever built".
	res := driver.WriteResult{Steps: []driver.WriteStep{}}

	// RUNG 1 — the slot parses.
	addr, bank, ok := parseSlot(ch.Slot)
	if !ok {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: "not a slot this radio has: its slots are 001..099, P1 and P2, in exactly those spellings"}
	}

	// RUNG 2 — the slot is in a bank THIS SESSION has. Distinct from rung 1
	// and not a restatement of it: "this radio has no such channel" and "this
	// channel is not writable" are different refusals and must read
	// differently. The walk is over the session's EFFECTIVE banks.
	if !s.hasBank(bank) {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: fmt.Sprintf("slot is not part of any bank this session supports (it would belong to %s)", bank)}
	}

	// RUNG 3 — an EMPTY channel is an erase request, and it is refused. This
	// rung is third STRUCTURALLY rather than by preference: an empty channel
	// has no Data at all, and every check below dereferences one.
	if ch.Empty() {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "this tier ships no erase path: the document prints two clear forms — a truncated 1A 00 set, and command 0B, whose own row says P1 and P2 cannot be cleared — and neither is implemented; spec.ConsentUnverifiedWrites refuses to consent an erase at any label",
		}
	}
	data := *ch.Data

	// RUNG 4 — field validity and vocabularies. A malformed field (an
	// unrecognised state, or a non-Known value smuggled alongside a value) is
	// refused rather than interpreted, and a Known value outside THIS RADIO's
	// vocabulary is refused rather than sent. An EMPTY vocabulary fails
	// CLOSED, which is what catches a Known duplex, DTCS code or polarity on
	// a radio that has none of the three.
	if err := s.validateFields(ch.Slot, data); err != nil {
		return res, err
	}

	// RUNG 5 — THE capability gate, defence in depth below the clone service.
	// spec.Inert is accepted as transmittable per the neutral rule; no field
	// of this driver's is Inert today (that is an FT-710 HARDWARE finding
	// about ITS clarifier and is not borrowed), so the arm is currently
	// unreachable and is kept deliberately.
	var unwritable []spec.Field
	for _, f := range requestedFields(data) {
		fs := s.caps.FieldSupport(bank, f)
		if !fs.CanWrite() && fs.Write != spec.Inert {
			unwritable = append(unwritable, f)
		}
	}
	if len(unwritable) > 0 {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: unwritable,
			Reason: "not write-Supported for this session (the CI-V record cannot express the field, or this session's capability profile does not support writing it)",
		}
	}

	// RUNG 6 — the mandatory-Known rules. Only a Known value is ever encoded,
	// and a non-Known value for a field the record CANNOT OMIT is REFUSED,
	// never synthesised and never carried over from a cache.
	if err := s.mandatoryFields(ch.Slot, data); err != nil {
		return res, err
	}

	// RUNGS 7-9 — one read, the read-dependent refusals, and the set, all
	// inside ONE critical section (D15). transport.Engine.Do locks a single
	// exchange, not a read-modify-write SEQUENCE, and driver.Session does not
	// promise single-threaded use, so this mutex is what makes "the record I
	// checked is the record I am writing" true.
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	prev, raw, exists, err := s.preservationRead(ctx, addr)
	if err != nil {
		return res, fmt.Errorf("ic7300mk2: WriteChannel %s: preservation read: %w", ch.Slot, err)
	}

	rec, err := s.buildRecord(ch.Slot, bank, addr, data, prev, raw, exists)
	if err != nil {
		return res, err
	}

	cmd, err := s.p.BuildMemorySet(rec)
	if err != nil {
		// The builder validates through the SAME validator the outbound gate
		// re-runs, so a record it refuses is a frame the gate would refuse:
		// this is a refusal, not a transport failure, and nothing has gone out.
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: err.Error()}
	}

	// THE step list, declared in full once the frame provably exists and
	// before it goes near the wire. ONE element, because this radio's write
	// choreography IS one frame.
	res.Steps = []driver.WriteStep{{Command: "1A 00"}}
	const setStep = 0

	// ClassWriteWithAck, whose RetryReads is ZERO by construction: a timeout
	// is NEVER resolved by resending a write, and a second copy would be a
	// second write to a radio the caller never asked for. The matcher is the
	// profile's own SOURCE-ADDRESS-CHECKED six-byte FB, so another station's
	// acknowledgement on a shared bus is never read as ours.
	if _, err := s.eng.Do(ctx, cmd, civ.CIVWriteWithAckSpec(s.p.AcknowledgementMatcher())); err != nil {
		if errors.Is(err, transport.ErrRejected) {
			// The frame WAS transmitted and the radio explicitly refused it:
			// Sent true, Confirmed false. An FA says the radio would not take
			// the frame, and nothing about why.
			res.Steps[setStep].Sent = true
			return res, fmt.Errorf("ic7300mk2: WriteChannel %s: the radio refused the memory set (FA): %w", ch.Slot, err)
		}
		// Transport-level failure or timeout: the frame's fate is NOT
		// attributable — the host cannot tell whether it reached the radio —
		// so Sent stays false and the error carries the distinction.
		return res, fmt.Errorf("ic7300mk2: WriteChannel %s: memory set: %w", ch.Slot, err)
	}
	// Confirmed means the radio's own FB arrived. On CI-V that is a real
	// acknowledgement rather than CAT's "no rejection was heard".
	res.Steps[setStep].Sent, res.Steps[setStep].Confirmed = true, true
	return res, nil
}

// hasBank reports whether this session's EFFECTIVE capabilities carry bank.
func (s *Session) hasBank(id spec.BankID) bool {
	for _, b := range s.caps.Banks {
		if b.ID == id {
			return true
		}
	}
	return false
}

// validateFields is rung 4: internal consistency, and Known values judged
// against THIS RADIO's own vocabularies.
func (s *Session) validateFields(slot string, d codeplug.ChannelData) error {
	refuse := func(f spec.Field, err error) error {
		return &driver.WriteRefusedError{Slot: slot, Fields: []spec.Field{f}, Reason: err.Error()}
	}
	if err := d.CTCSSTone.Valid(s.caps); err != nil {
		return refuse(spec.FieldCTCSSTone, err)
	}
	if err := d.TagDisplay.Valid(); err != nil {
		return refuse(spec.FieldTagDisplay, err)
	}
	if err := d.ScanSkip.Valid(); err != nil {
		return refuse(spec.FieldScanSkip, err)
	}
	if err := d.DataMode.Valid(); err != nil {
		return refuse(spec.FieldDataMode, err)
	}
	if err := d.TxFreqHz.Valid(); err != nil {
		return refuse(spec.FieldTxFrequency, err)
	}
	if err := d.OffsetHz.Valid(); err != nil {
		return refuse(spec.FieldOffset, err)
	}
	if err := d.ToneTx.Valid(s.caps); err != nil {
		return refuse(spec.FieldToneTx, err)
	}
	if err := d.ToneRx.Valid(s.caps); err != nil {
		return refuse(spec.FieldToneRx, err)
	}
	if err := d.Filter.Valid(s.caps.Filters); err != nil {
		return refuse(spec.FieldFilter, err)
	}
	if err := d.ToneMode.Valid(toneModeValues(s.caps)); err != nil {
		return refuse(spec.FieldToneMode, err)
	}
	// The three EMPTY vocabularies. Empty is not "anything goes": it fails
	// CLOSED, which is exactly how a Known duplex, DTCS code or polarity —
	// values that can only have come from a file written for another radio —
	// meets a refusal naming the field rather than being dropped.
	if err := d.Duplex.Valid(duplexOptionValues(s.caps)); err != nil {
		return refuse(spec.FieldDuplex, err)
	}
	if err := d.DTCSCode.Valid(s.caps.DTCSCodes); err != nil {
		return refuse(spec.FieldDTCSCode, err)
	}
	if err := d.DTCSPolarity.Valid(s.caps.DTCSPolarities); err != nil {
		return refuse(spec.FieldDTCSPolarity, err)
	}
	return nil
}

// mandatoryFields is rung 6: D18's table, for the fields the record cannot
// omit. A non-Known value here is REFUSED — never synthesised, never
// substituted from a neighbouring field, and never carried over from a
// cache.
//
// REV 1's "TxFreqHz when Known, else FreqHz" substitution is STRUCK and must
// not come back: it manufactured a value out of Absent/Unknown/Unavailable,
// against codeplug's own "only a Known value is ever sent to a radio", and on
// an existing split channel it would overwrite a transmit frequency the
// caller never asked to change.
func (s *Session) mandatoryFields(slot string, d codeplug.ChannelData) error {
	refuse := func(f spec.Field, format string, args ...any) error {
		return &driver.WriteRefusedError{Slot: slot, Fields: []spec.Field{f}, Reason: fmt.Sprintf(format, args...)}
	}

	if err := s.frequencyInRange(d.FreqHz); err != nil {
		return refuse(spec.FieldFrequency, "%v", err)
	}
	if d.TxFreqHz.State != codeplug.Known {
		return refuse(spec.FieldTxFrequency, "the record's transmit-frequency field (❹ ~ ⑧) cannot be omitted, and %q is not a value: nothing is synthesised for it, and substituting the receive frequency would overwrite a split channel's own transmit frequency", d.TxFreqHz.State)
	}
	if err := s.frequencyInRange(d.TxFreqHz.Value); err != nil {
		return refuse(spec.FieldTxFrequency, "%v", err)
	}
	if d.Mode == "" {
		return refuse(spec.FieldMode, "the record's mode byte (⑨) cannot be omitted and no mode was given")
	}
	if !contains(s.caps.Modes, d.Mode) {
		return refuse(spec.FieldMode, "%q is not one of this radio's modes %v — mode code 06 is printed nowhere in this document and no name is invented for it", d.Mode, s.caps.Modes)
	}
	if d.Filter.State != codeplug.Known {
		return refuse(spec.FieldFilter, "the record's filter byte (⑩) cannot be omitted, and %q is not a value", d.Filter.State)
	}
	if d.DataMode.State != codeplug.Known {
		return refuse(spec.FieldDataMode, "the record's data-mode nibble (⑪ high) cannot be omitted, and %q is not a value", d.DataMode.State)
	}
	if d.ToneMode.State != codeplug.Known {
		return refuse(spec.FieldToneMode, "the record's tone-mode nibble (⑪ low) cannot be omitted, and %q is not a value", d.ToneMode.State)
	}
	// An EMPTY tag is a legitimate blank name — the field is always ten bytes
	// and is padded with 0x20 — so only length and charset are refused here.
	if len(d.Tag) > s.caps.TagLen {
		return refuse(spec.FieldTag, "the name field (⑱ ~ ㉝) is %d bytes and %q is %d: truncating it would write a name the caller did not choose", s.caps.TagLen, d.Tag, len(d.Tag))
	}
	for i := 0; i < len(d.Tag); i++ {
		if !s.caps.TagByteOK(d.Tag[i]) {
			return refuse(spec.FieldTag, "%q has byte %#02x at offset %d, which is not in %s", d.Tag, d.Tag[i], i, s.caps.TagCharsetDescription())
		}
	}
	return nil
}

// frequencyInRange refuses a frequency this MEMORY CHANNEL cannot store.
//
// The ceiling is the RECORD's, and it is MANUAL-EVIDENCED on this model: the
// frequency field's 10 MHz digit runs 0 ~ 7 and its two highest digits are
// printed fixed zero (PDF p.16), so a value above it is one the encoder must
// afterwards refuse. Refusing here names the field; refusing at the encoder
// would not.
//
// THE FLOOR IS NOT CHECKED, and that is deliberate rather than an omission:
// this document prints no tuning floor anywhere, MinFreqHz is therefore
// zero, and a zero DISABLES the lower-bound check rather than asserting a
// 0 Hz floor. Borrowing the IC-7300's 30 000 Hz would be exactly the
// cross-model contamination both matrices' §4 forbid. A populated channel at
// 0 Hz is separately rejected by core/codeplug's own validator.
func (s *Session) frequencyInRange(hz uint64) error {
	if s.caps.MinFreqHz != 0 && hz < s.caps.MinFreqHz {
		return fmt.Errorf("%d Hz is below this radio's documented floor of %d Hz", hz, s.caps.MinFreqHz)
	}
	if s.caps.MaxFreqHz != 0 && hz > s.caps.MaxFreqHz {
		return fmt.Errorf("%d Hz is above what a memory channel can store on this model (%d Hz): the record's 10 MHz digit runs 0 ~ 7 and its 1 GHz and 100 MHz digits are printed fixed 0 (PDF p.16)", hz, s.caps.MaxFreqHz)
	}
	return nil
}

// preservationRead is rung 7 and the first half of rung 8: ONE read of the
// slot about to be written, its answer's channel address checked before any
// use of it (D20/T2).
//
// It returns the decoded record, its RAW bytes and whether the slot is
// OCCUPIED. Both forms of empty — the FA and the all-FF record — report
// exists false, because a write to either is a CREATE and a create has
// nothing to preserve.
func (s *Session) preservationRead(ctx context.Context, want civ.ChannelAddress) (civ.MemoryRecord, []byte, bool, error) {
	cmd, err := s.p.BuildMemoryRead(want)
	if err != nil {
		return civ.MemoryRecord{}, nil, false, err
	}
	frame, err := s.eng.Do(ctx, cmd, civ.CIVReadSpec(s.p.MemoryAnswerMatcher(), 1))
	if errors.Is(err, transport.ErrRejected) {
		// T4: the FA is CONSUMED by Engine.Do, which returns ErrRejected and
		// no frame. This branch never keys on "an FA frame arrived".
		return civ.MemoryRecord{}, nil, false, nil
	}
	if err != nil {
		return civ.MemoryRecord{}, nil, false, err
	}
	got, raw, err := s.p.MemoryAnswerRecord(frame)
	if err != nil {
		return civ.MemoryRecord{}, nil, false, err
	}
	if got != want {
		s.noteAnswerMismatch()
		return civ.MemoryRecord{}, nil, false, &AnswerMismatchError{Requested: want.String(), Answered: got.String()}
	}
	if allFF(raw) {
		return civ.MemoryRecord{}, raw, false, nil
	}
	rec, err := s.p.ParseMemoryAnswer(frame)
	if err != nil {
		return civ.MemoryRecord{}, nil, false, err
	}
	return rec, raw, true, nil
}

// buildRecord is the second half of rung 8 and the mapping of D18's table.
//
// THE THREE READ-DEPENDENT REFUSALS LIVE HERE, and they are the one recorded
// exception to "refusal before any wire traffic":
//
//   - THE CREATE. An empty slot has no SELECT value to preserve and no
//     spec.Field carries one (D4), so a create REFUSES rather than writing
//     OFF and moving the channel into a scan group the user never chose.
//     Behind it, the tone spans have no documented default either — recorded
//     so that a later resolution of the SELECT question cannot silently
//     enable an unsourced tone.
//   - E6'S TEMPLATE CHECK. The record's unmapped region — ③'s HIGH nibble,
//     the split flag — must equal the profile's Fixed template. A Split-ON
//     channel is REFUSED, not cleared: civ's encoder writes the template over
//     every unmapped nibble, so a driver that neither carried the flag
//     through nor refused would silently clear it and no layer above could
//     see it happen.
//   - THE SCAN-BANK CONSTRAINT. P1 and P2 must carry a zero ③ (PDF p.17,
//     "ⓘ Set 00 for P1 and P2."). It is READ-DEPENDENT for
//     exactly E6's reason — the value it judges is the one the radio holds —
//     so it sits here rather than among the locally decidable rungs.
func (s *Session) buildRecord(slot string, bank spec.BankID, addr civ.ChannelAddress, d codeplug.ChannelData, prev civ.MemoryRecord, raw []byte, exists bool) (civ.MemoryRecord, error) {
	if !exists {
		return civ.MemoryRecord{}, &driver.WriteRefusedError{
			Slot:   slot,
			Reason: "the slot is empty and this is a CREATE: record byte ③'s SELECT nibble has no honest source — no spec.Field carries the SELECT group (§3.16 A10 reads it as group membership, the opposite sense to a skip flag), and writing OFF would put the channel into a scan group the caller never chose. Behind it the two tone spans have no documented default either (`ic7300mk2-documented-default-tone-absent`). Write into a slot the radio already holds, or lift `ic7300mk2-select-nibble-on-create`",
		}
	}

	layout, ok := s.p.LayoutFor(len(raw))
	if !ok {
		// Unreachable: MemoryAnswerRecord already refused a length this
		// profile does not declare. Asserted rather than assumed, because the
		// cost of being wrong is an E6 check against a template of the wrong
		// width.
		return civ.MemoryRecord{}, fmt.Errorf("ic7300mk2: WriteChannel %s: no layout for the %d-byte record just read", slot, len(raw))
	}
	if len(layout.Fixed) != len(raw) {
		return civ.MemoryRecord{}, fmt.Errorf("ic7300mk2: WriteChannel %s: this layout's Fixed template is %d bytes against a %d-byte record — E6's unmapped-region check needs a full-length template", slot, len(layout.Fixed), len(raw))
	}
	if raw[0]&0xF0 != layout.Fixed[0]&0xF0 {
		return civ.MemoryRecord{}, &driver.WriteRefusedError{
			Slot:   slot,
			Reason: fmt.Sprintf("record byte ③ is %#02x: its HIGH nibble is the SPLIT flag, which this profile leaves UNMAPPED under an all-zero Fixed template, and %#02x is not the %#02x the template declares. The channel's Split function is ON, and writing it back would silently clear the user's split flag — so the write is refused instead (enablers ruling E6)", raw[0], raw[0]&0xF0, layout.Fixed[0]&0xF0),
		}
	}

	selectName, ok := prev.Select.Get()
	if !ok {
		return civ.MemoryRecord{}, fmt.Errorf("ic7300mk2: WriteChannel %s: the record just read carries no SELECT value, which this profile's layout maps", slot)
	}
	// THE WHOLE BYTE, and that is only correct because E6's check ran two
	// statements above. ③'s HIGH nibble is the split flag, and the compare
	// above has already refused every record whose high nibble is not the
	// template's zero — so by here `raw[0] != 0x00` can only be the LOW
	// nibble, the SELECT group. Were the E6 rung ever moved or removed, this
	// line would silently start refusing Split-ON scan edges with the wrong
	// reason named, so the two must stay in this order.
	if bank == spec.BankScan && raw[0] != 0x00 {
		return civ.MemoryRecord{}, &driver.WriteRefusedError{
			Slot:   slot,
			Reason: fmt.Sprintf("record byte ③ is %#02x on a scan edge, and this document prints \"Set 00 for P1 and P2.\" (PDF p.17) — the SELECT group is %q, and writing it back would send a value the document says these two slots must not carry (the value is the radio's own, so it is refused rather than rewritten)", raw[0], selectName),
		}
	}

	rec := civ.MemoryRecord{Address: addr}
	// ③ LOW — carried through from the record the radio holds. No spec.Field
	// carries it, so preserving it is the only way a write does not move the
	// channel out of its scan group.
	rec.Select = civ.Available(selectName)
	// ④ ~ ⑧ and ❹ ~ ⑧.
	rec.RXFreqHz = civ.Available(d.FreqHz)
	rec.TXFreqHz = civ.Available(d.TxFreqHz.Value)
	// ⑨/❾, ⑩/❿ and ⑪/⓫'s two nibbles. The layout maps each of these twice,
	// so the encoder writes both copies from the one value: the driver never
	// mirrors by hand, and a record whose halves disagree cannot be built.
	rec.Mode = civ.Available(d.Mode)
	rec.Filter = civ.Available(d.Filter.Value)
	rec.DataMode = civ.Available(dataModeName(d.DataMode.Value))
	rec.ToneMode = civ.Available(d.ToneMode.Value)
	// ⑫ ~ ⑭ and ⑮ ~ ⑰, PRESERVED when the caller's value is not Known
	// (T1(4)):
	// the value written is the one the radio itself holds, read moments ago
	// under this same mutex. That is preservation of an OBSERVED byte, not a
	// manufactured value, and it is why the two tone fields are conditional
	// rather than mandatory.
	rec.ToneTXDeciHz = preservedTone(d.ToneTx, prev.ToneTXDeciHz)
	rec.ToneRXDeciHz = preservedTone(d.ToneRx, prev.ToneRXDeciHz)
	// ⑱ ~ ㉝, padded to SIXTEEN bytes by the codec.
	rec.Name = civ.Available(d.Tag)
	return rec, nil
}

// preservedTone is T1(4) in one function: a Known tone is the caller's, and
// anything else is the radio's own value carried through unchanged.
func preservedTone(f codeplug.ToneField, prev civ.Optional[uint64]) civ.Optional[uint64] {
	if f.State == codeplug.Known {
		return civ.Available(uint64(f.Value))
	}
	return prev
}

// dataModeName maps the neutral flag onto ⑪'s printed HIGH-nibble legend.
// The nibble assignment is read from this model's own B leg, which records
// it directly from the strip's arrow labels: DATA left, TONE right.
//
// A name this profile's enum does not carry would be refused by
// BuildMemorySet's own validator — the same one the outbound gate re-runs —
// so a drift here fails loudly at build time rather than putting an
// undefined nibble on the wire.
func dataModeName(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}

// toneModeValues and duplexOptionValues render a capability's vocabulary as
// the plain []string codeplug's StringField.Valid takes. They are two lines
// each and they exist so no vocabulary is restated in this package: the
// values judged are the ones the capabilities advertise.
func toneModeValues(caps spec.Capabilities) []string {
	out := make([]string, 0, len(caps.ToneModes))
	for _, m := range caps.ToneModes {
		out = append(out, m.Value)
	}
	return out
}

func duplexOptionValues(caps spec.Capabilities) []string {
	out := make([]string, 0, len(caps.DuplexOptions))
	for _, o := range caps.DuplexOptions {
		out = append(out, o.Value)
	}
	return out
}

// contains reports whether v is in list.
func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
