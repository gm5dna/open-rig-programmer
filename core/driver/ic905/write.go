// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic905 "github.com/gm5dna/open-rig-programmer/core/civ/ic905"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// unconditionalFields is what EVERY write of this model requests,
// whatever the channel holds: the three fields with no FieldState of
// their own, so a channel always speaks about them.
//
// IT IS THIS MODEL'S MATRIX §2, NOT codeplug.addedFields (ruling
// R6-COMPLETION). addedFields' unconditional six are the YAESU set —
// frequency, mode, clarifier, ctcss_state, shift, tag — and three of
// those this model grades ZERO ON BOTH BANKS (matrix §2 rows 3, 4 and 6).
// A driver that copied them verbatim would request clarifier,
// ctcss_state and shift on every write, and every write would then be
// refused BY ITS OWN CAPABILITY GATE.
var unconditionalFields = []spec.Field{
	spec.FieldFrequency, // matrix §2 row 1 — bytes ⑥~⑩
	spec.FieldMode,      // row 2 — byte ⑪
	spec.FieldTag,       // row 7 — bytes 53~68
}

// tierRequestedFields are requested only when the channel carries them
// KNOWN, in codeplug.ChannelData's own declaration order — the order
// codeplug's touchedFields uses, so this driver's defence-in-depth gate
// and the diff layer's gate name fields in the same order.
//
// A FIELD MISSING FROM THIS TABLE IS A FIELD THE GATE NEVER SEES, and
// therefore a Known value SILENTLY DROPPED — which is exactly what
// core/driver's contract forbids: "A field carrying FieldState Known that
// the protocol cannot express is likewise refused, never silently
// dropped."
//
// THAT IS WHY THREE OF THIS MATRIX'S ZEROS ARE IN THIS TABLE.
// tag_display, scan_skip and tx_frequency (matrix §2 rows 9, 10 and 11)
// are fields this record cannot express AND fields a codeplug.ChannelData
// can nonetheless speak Known about, so they are requested when Known
// PRECISELY SO rung 7's capability gate meets them: caps.go's bankFields
// grades all three the zero FieldSupport on BOTH banks, and
// spec.ConsentUnverifiedWrites re-labels only Unverified, never
// Unsupported — so a Known value for any of the three is REFUSED, with
// the field named and nothing on the wire, on every session this driver
// can open. Before they were listed here such a value was dropped
// between the gate and the encoder, and the 1A 00 set went out as though
// the caller had never asked.
//
// FOUR ZEROS REMAIN ABSENT, and each for a reason rather than by
// omission. clarifier, ctcss_state and shift carry NO FieldState at all —
// there is no Known for this table to test, and their neutral zero values
// (ClarHz 0 with both flags false, the empty string) are indistinguishable
// from a channel that never spoke about them; that is this table's known
// bound, recorded rather than papered over. erase is not something a
// populated channel can request: rung 3 refuses the empty channel that
// would mean it, above this table entirely.
var tierRequestedFields = []struct {
	field   spec.Field
	present func(codeplug.ChannelData) bool
}{
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

// requestedFields lists every spec.Field a write of data actually
// requests: the three unconditional ones, then each tier field the
// channel carries Known. Per codeplug's write rule, Unknown and
// Unavailable both mean "preserve whatever the radio has", i.e. nothing
// is requested for that field.
func requestedFields(data codeplug.ChannelData) []spec.Field {
	fields := append([]spec.Field(nil), unconditionalFields...)
	for _, t := range tierRequestedFields {
		if t.present(data) {
			fields = append(fields, t.field)
		}
	}
	return fields
}

// mandatoryKnownFields are the mapped fields whose value the encoder MUST
// have and which nothing can supply for it — ruling R6's "only Known
// values are ever encoded", applied to this layout.
//
// EVERY MAPPED FIELD IS HERE EXCEPT THE TWO TONES, and the exception is
// ruling T1(4) rather than an oversight: a non-Known tone is filled from
// the JUST-READ RECORD'S OWN BYTES, which is value-level preservation of
// what the radio holds rather than a value this program chose. There is
// no such source for anything else — civ has no preserve-by-cache and
// validateRecordFields refuses a mapped field with no value — so a
// non-Known one is REFUSED, never synthesised.
//
// frequency, mode and tag are absent because they carry no FieldState at
// all: a codeplug.ChannelData always speaks about them.
//
// The order is ChannelData's declaration order, so a refusal naming
// several of them names them the way every other layer does.
var mandatoryKnownFields = []struct {
	field spec.Field
	known func(codeplug.ChannelData) bool
}{
	{spec.FieldDuplex, func(d codeplug.ChannelData) bool { return d.Duplex.State == codeplug.Known }},
	{spec.FieldOffset, func(d codeplug.ChannelData) bool { return d.OffsetHz.State == codeplug.Known }},
	{spec.FieldToneMode, func(d codeplug.ChannelData) bool { return d.ToneMode.State == codeplug.Known }},
	{spec.FieldDTCSCode, func(d codeplug.ChannelData) bool { return d.DTCSCode.State == codeplug.Known }},
	{spec.FieldDTCSPolarity, func(d codeplug.ChannelData) bool { return d.DTCSPolarity.State == codeplug.Known }},
	{spec.FieldFilter, func(d codeplug.ChannelData) bool { return d.Filter.State == codeplug.Known }},
	{spec.FieldDataMode, func(d codeplug.ChannelData) bool { return d.DataMode.State == codeplug.Known }},
}

// unmappedRange is one run of record bytes no FieldSpan maps, with the
// printed index it carries and what a difference from the template means.
//
// THERE ARE SIX OF THEM AND THEY TOTAL TWENTY-SEVEN BYTES. Byte ⑤ is one
// of them, and its inclusion is the fix for this plan's one CRITICAL
// review finding: a MEM channel carrying SELECT ★1/★2/★3 on ⑤ would
// otherwise have passed the preservation read and been rewritten as
// SELECT OFF — a silent conversion of an unsupported state into a drop,
// which is precisely what the tier's E6 ruling forbids and what the
// scan_skip-is-SELECT constraint exists to protect.
type unmappedRange struct {
	// printed is the index PDF p.19 (folio 18) prints for this run.
	printed string
	// offset is its first byte in the SHORT (64-byte) layout; the wide
	// layout shifts everything after the frequency by one, which
	// offsetIn applies.
	offset int
	length int
	// meaning is what a difference from the template says the channel
	// holds — the words the refusal puts in front of a user.
	meaning string
}

// unmappedRanges is the six runs, in record order.
var unmappedRanges = []unmappedRange{
	{"⑤", 0, 1, "a SELECT ★ tag is set on this channel"},
	{"⑮", 10, 1, "a digital-squelch mode is set"},
	{"㉕", 20, 1, "a DV code squelch is set"},
	{"㉙~㊱", 24, 8, "a UR (destination) call sign is stored"},
	{"㊲~㊹", 32, 8, "an R1 (access repeater) call sign is stored"},
	{"㊺~52", 40, 8, "an R2 (gateway/link) call sign is stored"},
}

// offsetIn is r's first byte in a record of the given length. Byte ⑤ sits
// BEFORE the frequency and never moves; everything after it shifts by the
// frequency field's extra byte in the wide layout.
func (r unmappedRange) offsetIn(recordLength int) int {
	if r.offset == 0 {
		return 0
	}
	return r.offset + recordLength - civic905.RecordLengthShort
}

// unmodelledRegionDiffers compares all six unmapped runs of a STORED
// record against the profile's Fixed template and returns a description
// of each that differs, in record order.
//
// THE COMPARISON IS AGAINST THE TEMPLATE AND NOTHING ELSE. E6 struck its
// own earlier "or the just-read bytes it is provably preserving"
// allow-case as unimplementable — civ's encoder fills unmapped bytes from
// the template, and there is no way to ask it to put different ones back
// — so "equals the template" is the only condition under which a write
// can be shown to change nothing it does not model.
//
// An empty ranges result means the write may proceed as far as this rung
// is concerned.
func unmodelledRegionDiffers(stored []byte, layout civ.RecordLayout) []string {
	var ranges []string
	template := layout.Fixed
	for _, r := range unmappedRanges {
		off := r.offsetIn(layout.Length)
		end := off + r.length
		if end > len(stored) || end > len(template) {
			// A record shorter than its own layout cannot reach here —
			// civ.MemoryAnswerRecord has already refused any length
			// outside the accepted set — but refuse rather than index
			// past the end if it ever does.
			ranges = append(ranges, fmt.Sprintf("record bytes %d..%d (printed %s): the record is too short to hold them", off, end-1, r.printed))
			continue
		}
		if !bytes.Equal(stored[off:end], template[off:end]) {
			ranges = append(ranges, fmt.Sprintf("record bytes %d..%d (printed %s): %s", off, end-1, r.printed, r.meaning))
		}
	}
	return ranges
}

// memorySetSpec is the transport spec for the 1A 00 memory set, assembled
// from E1's helper over the CODEC's own address-checked ack matcher.
//
// The set waits for the radio's OK message, FE FE E0 AC FB FD (PDF p.3,
// folio 2, "OK message to controller (PC)", cell labelled "OK code
// (fixed)"); the NG message FE FE E0 AC FA FD is the rejection. Those two
// are the ONLY fully concrete byte sequences printed anywhere in this
// document.
//
// CIVWriteWithAckSpec sets Class = ClassWriteWithAck and RetryReads = 0
// for us: NO RETRANSMISSION ON TIMEOUT, ever, and the write quarantine
// applies — because a set whose acknowledgement never arrives has an
// UNATTRIBUTABLE outcome, and sending it again could write the channel
// twice. (CATWriteSpec is fire-and-forget and is NOT the mirror for
// memory sets: CAT has no acknowledged write form at all.)
func (s *Session) memorySetSpec() transport.CommandSpec {
	return civ.CIVWriteWithAckSpec(s.profile.AcknowledgementMatcher())
}

// bankFor reports which of this session's EFFECTIVE banks claims slot.
//
// The walk is over the effective set — MEM's discovered slots and CALL's
// twelve static ones — so a slot this radio does not have is in no bank
// and is refused outright rather than gated per-field against a bank that
// does not exist. It is also what keeps the profile's known
// over-admission unreachable by hand: civ's single global channel range
// admits CALL-group channels 12…99, and none of them is in any bank.
//
// A SPARSE BANK CLAIMS ITS WHOLE SPACE, not only what discovery
// materialised (spec.Bank.WithinSpace): an address no read has ever
// returned is a perfectly legal place for a user to ADD a channel, which
// is exactly why the occupied-surprise rung below exists.
func (s *Session) bankFor(slot string) (spec.BankID, bool) {
	for _, b := range s.caps.Banks {
		if b.WithinSpace(slot) {
			return b.ID, true
		}
	}
	return "", false
}

// occupiedSurprise refuses a write that would ADD a channel to a slot the
// radio turns out to already hold.
//
// IT IS THE FIX FOR THE ROUND-3 CRITICAL (ruling T3). The bounded default
// walk probes only channel 00 in groups 1…99 and skips the rest when 00
// is empty. InventoryComplete lives in THIS driver's own diagnostics;
// core/clone.ReadAll neither sees nor consults it — it trusts Bank.Slots
// — and sparse Diff then permits an ADD at any unmaterialised address. So
// an existing G06-038 that the walk missed because G06-001 was empty
// arrives here as an apparent ADD, and the rung below is the only thing
// standing between it and being overwritten. Comparing unmapped bytes is
// not enough: a radio-held record whose unmapped region happens to match
// the template would have sailed through.
//
// IT FIRES ONLY WHEN BOTH ARE TRUE: the slot is absent from the inventory
// this session materialised, AND the pre-write read returned a record.
// Either alone is ordinary — an inventory-absent slot that reads empty is
// a genuine add, and an inventory-PRESENT slot that reads occupied is a
// modify.
//
// The remedy is NAMED, because the user can act on it.
func (s *Session) occupiedSurprise(slot string, readReturnedRecord bool) error {
	if !readReturnedRecord || s.inventory[slot] {
		return nil
	}
	return &driver.WriteRefusedError{
		Slot:   slot,
		Reason: "this session's inventory does not list this slot, but the radio answered the pre-write read with a record: the discovery walk never saw it, so writing would overwrite a channel nothing has read. Re-discover the radio, or reopen the session with WithFullInventoryWalk()",
	}
}

// WriteChannel implements driver.Session: the refusal ladder, ONE
// preservation read, and ONE acknowledged 1A 00 set.
//
// # The ladder's order is contractual
//
// core/driver's contract is that a driver refuses BEFORE ANY WIRE
// TRAFFIC, with the one recorded exception ruling T5 fixes: a refusal
// that needs the SLOT'S CURRENT STATE cannot precede the read that
// obtains it. So every locally decidable check comes first, then ONE
// read, then the two read-dependent refusals, then the write:
//
//	R1   the slot parses (both namespaces)
//	R2   the slot is in an effective bank
//	R3   the channel is not empty — this tier has NO ERASE PATH
//	R4   every mapped field but the tones is Known (R6)
//	R5   the frequency fits the record shape this document draws (OQ-1)
//	     AND lies inside the declared storable range
//	R6   the DTCS code's digits are 0–7 (OQ-6, defence in depth)
//	R6b  the tones lie inside the declared tone domain
//	R6c  the vocabularies, the tag and the offset — everything else
//	     this radio cannot SAY
//	R7   the capability gate (defence in depth below the clone service)
//	R8   the cross-field combination rules the page prints
//	---- everything above precedes ALL wire traffic ----
//	R9   THE ONE READ
//	R10  the E6 template mismatch (OQ-4): 27 unmodelled bytes
//	R11  the T3 occupied surprise
//	R12  build the frame
//	R13  declare the steps
//	R14  the acknowledged set
//
// The order also decides the order WriteRefusedError.Fields names fields,
// which is why it is written down rather than left to whatever reads
// naturally.
//
// # Nothing may be written to a real IC-905 today
//
// and this method is not what enforces it — the CAPABILITY PROFILE is.
// writeTrialsComplete is false, so a RealHardware session gets the
// all-Unverified fail-safe and rung 7 refuses every field of every
// channel. This ladder is the choreography that becomes reachable when a
// profile allows it: today the Simulated profile, or a RealHardware
// session the user has explicitly consented for.
//
// # A set carries the WHOLE record
//
// Matrix §3.10 grades that ASSUMED — the document draws one complete
// layout and never authorises a short write except in the clear form.
// Register: ic905.write_full_record_required. Lift: ic905-W-02 (send one
// complete 1A 00 set to a scratch channel; record whether the answer is
// FB or FA). Every rung above rests on it, and rung 12 is where it bites.
//
// # No read-back
//
// WriteChannel reports only sent/acknowledged. Reading the slot back and
// comparing is the clone service's job — verification policy lives in one
// place above every driver rather than half-implemented inside each.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	// Every refusal below returns res unchanged, i.e. an EXPLICITLY EMPTY
	// step list — never nil. The clone service journals this result, and
	// a nil slice marshals as JSON null, which an auditor would have to
	// read as "unknown" rather than the truth, "no frame was ever built,
	// so nothing was attempted".
	res := driver.WriteResult{Steps: []driver.WriteStep{}}

	// RUNG 1. The slot parses, in one of the two namespaces.
	addr, err := slotAddress(ch.Slot)
	if err != nil {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: fmt.Sprintf("not a valid slot: %v", err)}
	}

	// RUNG 2. The slot belongs to a bank this session has.
	bank, ok := s.bankFor(ch.Slot)
	if !ok {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Reason: "slot is not part of any bank this session supports"}
	}

	// RUNG 3. An empty channel is an ERASE request, and this tier ships
	// NO ERASE PATH AT ALL: no builder, no gate admission, FieldErase the
	// zero FieldSupport in both profiles, and spec.ConsentUnverifiedWrites
	// structurally never consents it.
	//
	// The wire form EXISTS on this radio and is recorded in doc.go —
	// FE FE AC E0 1A 00 <group> <chan> FF FD, with the CALL group
	// excluded ("You cannot specify group \"01 00\"") — together with
	// what a future write-trial milestone would need. Nothing here
	// implements it.
	//
	// This rung must precede the FieldState checks STRUCTURALLY, not
	// merely by preference: an empty channel has no Data at all, and
	// every rung below dereferences it.
	if ch.Empty() {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldErase},
			Reason: "this tier ships no erase path for any Icom: the documented 1A 00 clear form is recorded in this package's doc.go and implemented nowhere, and FieldErase is not write-Supported in any profile",
		}
	}
	data := *ch.Data

	// RUNG 4. Only Known values are ever encoded (ruling R6).
	var notKnown []spec.Field
	for _, m := range mandatoryKnownFields {
		if !m.known(data) {
			notKnown = append(notKnown, m.field)
		}
	}
	if len(notKnown) > 0 {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: notKnown,
			Reason: "a 1A 00 set carries the whole record, and this codec has no preserve-by-cache: a field the record maps must carry a Known value or the write is refused, never synthesised",
		}
	}

	// RUNG 5. The frequency must fit the record shape this document
	// draws. THIS IS OQ-1'S CONSEQUENCE AND IT FAILS BEFORE THE WIRE,
	// not in the encoder.
	//
	// civ.ProfileConfig.BuildLength is a single static int, so a profile
	// cannot pick per record; this one declares 64, the only shape the
	// memory-content diagram draws. A 10 GHz channel needs the six-byte
	// frequency form, which is ASSUMED and which no IC-905 has ever been
	// asked to accept. Register: the D5 model-specific entry for the 905.
	// Lift: ic905-R-06.
	if civic905.NeedsWideFrequency(data.FreqHz) {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldFrequency},
			Reason: fmt.Sprintf("%d Hz needs the six-byte frequency form (a %d-byte record), which the memory-content diagram does not draw and which no IC-905 has ever been asked to accept: ASSUMED, lift ic905-R-06", data.FreqHz, civic905.RecordLengthWide),
		}
	}

	// AND THE DECLARED BOUNDS, which is the half no encoder can catch.
	// The check above is about the RECORD'S SHAPE; this one is about the
	// RADIO'S RANGE, and they are different questions on different
	// evidence. 143.999999 MHz encodes into five bytes perfectly happily,
	// so nothing below here would have refused it: civ validates the
	// ENCODING, and codeplug.Validate — which does check this — sits
	// UPSTREAM, on the very apply path this seam is the defence in depth
	// for.
	//
	// Both ends are ASSUMED as memory-record limits, read off the band
	// table (PDF p.20, folio 19, "Band stacking register"): registers
	// ic905.min_storable_hz (lift ic905-R-05) and ic905.max_storable_hz
	// (lift ic905-R-06).
	//
	// THE CEILING IS UNREACHABLE TODAY — the wide-form refusal above
	// bites at 9,999,999,999 Hz, below the declared 10.5 GHz — and it is
	// checked anyway, so that lifting ic905-R-06 cannot silently remove a
	// bound while it removes the shape restriction.
	if data.FreqHz < s.caps.MinFreqHz || (s.caps.MaxFreqHz != 0 && data.FreqHz > s.caps.MaxFreqHz) {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldFrequency},
			Reason: fmt.Sprintf("%d Hz is outside what this radio is declared to store, %d ~ %d Hz (ASSUMED as memory-record limits: ic905.min_storable_hz, lift ic905-R-05; ic905.max_storable_hz, lift ic905-R-06)", data.FreqHz, s.caps.MinFreqHz, s.caps.MaxFreqHz),
		}
	}

	// RUNG 6. The DTCS code's digits are 0–7 (OQ-6).
	//
	// The PRIMARY gate is the explicit 512-code table this driver
	// declares, which codeplug.Validate consults before this driver is
	// reached. This is the driver seam's own defence-in-depth re-check,
	// which its contract requires; neither is a widening (ruling R11).
	if !validDTCSCode(data.DTCSCode.Value) {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: []spec.Field{spec.FieldDTCSCode},
			Reason: fmt.Sprintf("DTCS code %03d has a digit above 7: the printed ranges are 0 ~ 7 for all three digits (PDF p.24, folio 23), and civ's BCD encoder would accept it", data.DTCSCode.Value),
		}
	}

	// RUNG 6b. THE TONE DOMAIN — the sibling re-check, on the sibling
	// field, from the same printed page.
	//
	// A Known tone outside the declared CTCSSToneRange puts BYTES ON THE
	// WIRE THAT THE PRINTED DIGIT RANGES FORBID. PDF p.24 (folio 23)
	// prints byte 1 as "0 : 0", BOTH halves "Fixed digit: 0*", and byte
	// 2's 100 Hz digit as "0 ~ 2" — so 99999 deciHz encodes 09 99 99 and
	// violates both. civ's BCD encoder accepts it exactly as it accepts a
	// DTCS digit above 7, which is the whole reason rung 6 exists; this
	// is that same obligation, on the field printed beside it.
	//
	// IT IS ALSO THE WRITE-SIDE MIRROR OF read.go's toneField. That
	// function is scrupulous under T1(3) — a READ never constructs a
	// Known value codeplug.Validate would refuse, and maps a zero or an
	// out-of-domain tone to Unknown — and until this rung the write path
	// had no equivalent care: it encoded whatever Known value it was
	// handed.
	//
	// AdmitsTone is asked rather than the range's fields: it is the ONE
	// predicate that knows about both declaration shapes and fails closed
	// when a radio declares neither.
	//
	// A NON-KNOWN tone is not a domain question at all — it is
	// preservation (T1(4)) on an occupied slot, and the create rule on an
	// empty one — so it is not judged here.
	var badTones []spec.Field
	if data.ToneTx.State == codeplug.Known && !s.caps.AdmitsTone(data.ToneTx.Value) {
		badTones = append(badTones, spec.FieldToneTx)
	}
	if data.ToneRx.State == codeplug.Known && !s.caps.AdmitsTone(data.ToneRx.Value) {
		badTones = append(badTones, spec.FieldToneRx)
	}
	if len(badTones) > 0 {
		return res, &driver.WriteRefusedError{
			Slot:   ch.Slot,
			Fields: badTones,
			Reason: fmt.Sprintf("the tone is outside this radio's declared domain of %d ~ %d tenths of a hertz in steps of %d (the printed digit ranges, PDF p.24, folio 23): zero is not a tone, and a value above the ceiling encodes digits the page says the field cannot hold",
				int(s.caps.CTCSSToneRange.MinDeciHz), int(s.caps.CTCSSToneRange.MaxDeciHz), int(s.caps.CTCSSToneRange.StepDeciHz)),
		}
	}

	// RUNG 6c. THE VOCABULARIES, THE TAG AND THE OFFSET — everything else
	// this radio cannot SAY. See unsayable for the argument: every one of
	// these was refused before this rung existed, but by the ENCODER at
	// rung 12, after the preservation read had already put a frame on the
	// wire, and as an error the neutral refusal sentinel could not see.
	if f, why := unsayable(data, s.caps); f != "" {
		return res, &driver.WriteRefusedError{Slot: ch.Slot, Fields: []spec.Field{f}, Reason: why}
	}

	// RUNG 7. THE CAPABILITY GATE — defence in depth below the clone
	// service. Every requested field must pass FieldSupport.CanWrite for
	// this slot's bank in THIS session's capabilities (spec.Supported, or
	// spec.ConsentedUnverified, which is the label every writable field
	// of a consented real-hardware session carries) OR be spec.Inert,
	// which is acceptable to TRANSMIT.
	//
	// No field of this driver's is Inert today — Inert is the FT-710's
	// hardware finding about ITS clarifier, and no IC-905 has been asked
	// anything — so the Inert arm is currently unreachable and is kept
	// deliberately: it is the neutral rule spec.Inert documents, and a
	// future finding that marked a field Inert must not also have to
	// change this gate.
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
			Reason: "not write-Supported for this session (the record cannot express the field, or this session's capability profile does not support writing it)",
		}
	}

	// RUNG 8. The cross-field combination rules the page prints.
	if err := crossFieldRefusal(ch.Slot, data); err != nil {
		return res, err
	}

	// ---- everything above this line precedes ALL wire traffic ----

	// RUNG 9. THE ONE READ. Its three outcomes are all defined:
	//
	//	transport.ErrRejected   the slot is empty (D5 entry 2(a))
	//	an all-0xFF record      the slot is empty (D5 entry 2(b))
	//	a decodable record      the slot is occupied
	//
	// recordAt carries the continuous length fingerprint, the T2 address
	// check and the T4 rejection branch, so this rung inherits all three
	// rather than restating any.
	stored, present, err := s.recordAt(ctx, addr)
	if err != nil {
		return res, fmt.Errorf("ic905: WriteChannel %s: preservation read: %w", ch.Slot, err)
	}
	occupied := present && !isEmptyRecord(stored)

	var prior civ.MemoryRecord
	if occupied {
		if prior, err = s.parseRecord(addr, stored); err != nil {
			return res, fmt.Errorf("ic905: WriteChannel %s: preservation read: %w", ch.Slot, err)
		}

		// RUNG 10. THE E6 TEMPLATE MISMATCH (OQ-4). Twenty-seven of the
		// sixty-four record bytes have no home in the neutral model, and
		// civ's encoder fills every unmapped byte from the profile's
		// Fixed template — so a write to a channel that really carries a
		// call sign, a digital squelch, a DV code squelch or a SELECT ★
		// tag would silently replace them.
		//
		// E6's rule: a driver may write a slot ONLY when its unmapped
		// regions equal the template; anything else is REFUSED with the
		// reason named, never rewritten. The stated cost is accepted for
		// this tier — such channels are refused, never corrupted, at one
		// extra read per write.
		layout, ok := s.profile.LayoutFor(len(stored))
		if !ok {
			// Unreachable: recordAt has already refused any length
			// outside the accepted set.
			return res, fmt.Errorf("ic905: WriteChannel %s: no layout for the %d-byte record the radio returned", ch.Slot, len(stored))
		}
		if diffs := unmodelledRegionDiffers(stored, layout); len(diffs) > 0 {
			return res, &driver.WriteRefusedError{
				Slot:   ch.Slot,
				Reason: "this channel holds data this tier cannot model, and writing it would silently replace it — " + strings.Join(diffs, "; "),
			}
		}
	}

	// RUNG 11. The T3 occupied surprise.
	if err := s.occupiedSurprise(ch.Slot, occupied); err != nil {
		return res, err
	}

	// AN EMPTY-SLOT CREATE HAS NO PRIOR RECORD, so a non-Known tone has
	// no source (ruling T1(5)). This manual documents NO DEFAULT TONE —
	// it prints the field's digit ranges (PDF p.24, folio 23) and no
	// default value anywhere, unlike the models whose manuals print
	// "Default: 88.5 Hz" — so the create REFUSES, naming the field, which
	// is T1(5)'s otherwise-branch. Register: ic905.create_default_tone.
	// Lift: ic905-R-18.
	if !occupied {
		var missing []spec.Field
		if data.ToneTx.State != codeplug.Known {
			missing = append(missing, spec.FieldToneTx)
		}
		if data.ToneRx.State != codeplug.Known {
			missing = append(missing, spec.FieldToneRx)
		}
		if len(missing) > 0 {
			return res, &driver.WriteRefusedError{
				Slot:   ch.Slot,
				Fields: missing,
				Reason: "creating a channel in an empty slot needs an explicit tone: there is no prior record to preserve one from, and this document prints no default tone value anywhere (MANUAL-EVIDENCED ABSENCE, ic905.create_default_tone, lift ic905-R-18)",
			}
		}
	}

	// RUNG 12. Build the frame — the whole record, per
	// ic905.write_full_record_required (lift ic905-W-02).
	cmd, err := s.profile.BuildMemorySet(s.recordFor(addr, data, prior, occupied))
	if err != nil {
		return res, fmt.Errorf("ic905: WriteChannel %s: %w", ch.Slot, err)
	}

	// RUNG 13. THE step list, declared in full HERE: after the frame
	// provably exists, before it goes near the wire. It has ONE element
	// because this radio's write choreography IS one frame.
	res.Steps = []driver.WriteStep{{Command: "1A 00"}}
	const setStep = 0

	// RUNG 14. The acknowledged set.
	if _, err := s.eng.Do(ctx, cmd, s.memorySetSpec()); err != nil {
		switch {
		case errors.Is(err, transport.ErrRejected):
			// The radio answered the printed NG message: the frame was
			// transmitted and explicitly refused. Attributable, and a
			// refusal.
			res.Steps[setStep].Sent = true
			return res, fmt.Errorf("ic905: WriteChannel %s: the radio rejected the set: %w", ch.Slot, err)
		case errors.Is(err, transport.ErrTimeout):
			// THE ACKNOWLEDGEMENT NEVER ARRIVED. The frame provably left
			// the port — Engine.Do writes before it waits — and BOTH
			// FLAGS ARE FALSE ANYWAY, because driver.WriteStep.Sent is
			// not "bytes went out": it "reports that the frame was
			// transmitted with an ATTRIBUTABLE outcome — success or an
			// explicit rejection", and a silent radio attributed none.
			// A false Sent is that neutral type's word for precisely
			// this — "a transport-level failure left its outcome
			// unknowable" — and the error below carries the half the
			// flags cannot, that this frame DID go out and the slot's
			// on-radio state is therefore UNVERIFIED rather than
			// untouched.
			//
			// NOTHING HERE DEPENDS ON THE FLAG TO PREVENT A SECOND
			// ATTEMPT, and it must not: RetryReads is zero on this class
			// and Do refuses a non-zero value outright, so no
			// retransmission is representable whatever this step says.
			return res, fmt.Errorf("ic905: WriteChannel %s: the set was transmitted and never acknowledged, so its outcome is UNATTRIBUTABLE and it will not be resent: %w", ch.Slot, err)
		default:
			// A transport-level failure before or around the write: the
			// frame may never have been transmitted at all. Sent stays
			// false and the error carries the distinction.
			return res, fmt.Errorf("ic905: WriteChannel %s: %w", ch.Slot, err)
		}
	}
	// Confirmed means the radio sent its OK message, which — unlike a CAT
	// Set's silence — is a positive acknowledgement.
	res.Steps[setStep].Sent, res.Steps[setStep].Confirmed = true, true
	return res, nil
}

// validDTCSCode reports whether every decimal digit of code is 7 or less
// — the printed range for all three DTCS digits (PDF p.24, folio 23,
// "• DTCS code and polarity setting").
func validDTCSCode(code int) bool {
	if code < 0 || code > 777 {
		return false
	}
	for n := code; n > 0; n /= 10 {
		if n%10 > 7 {
			return false
		}
	}
	return true
}

// The mode and duplex spellings the cross-field rules key on. They are
// the printed values core/civ/ic905's own enum tables carry, named once
// here so the rules and the vocabularies cannot drift apart.
const (
	modeDD     = "DD"
	modeATV    = "ATV"
	duplexRPS  = "RPS"
	duplexUp   = "DUP+"
	duplexDown = "DUP-"
	// band1200FloorHz is the 1200 MHz band's floor, from the band table
	// (PDF p.20, folio 19, "• Band stacking register", row
	// 03 | 1200 | 1240.000000 ~ 1300.000000).
	band1200FloorHz = 1_240_000_000
)

// crossFieldRefusal applies the combination rules THE PAGE PRINTS, which
// the generic codec cannot: it validates each enum independently, so
// without this a consented write could send a combination the manual
// forbids.
//
// Two printed notes, three refusals:
//
//   - PDF p.19 (folio 18), the ⑭ breakout's note: "ⓘ RPS can be set when
//     DD mode is selected, and Duplex (+, -) can be set when other than
//     DD mode is selected." — which forbids RPS without DD, and DUP±
//     with DD.
//   - PDF p.17 (folio 16)'s mode table footnote: "* The operating mode
//     can be set when the 1200 MHz or higher band is selected", against
//     22:DD* and 23:ATV*.
//
// THAT THE MEMORY RECORD ENFORCES WHAT THE COMMAND TABLE'S FOOTNOTES
// STATE IS ASSUMED. The footnotes are printed against the standalone
// commands, not against the 1A 00 record. Register:
// ic905.mode_band_constraint. Lift: ic905-R-19 (store a DD channel below
// 1200 MHz from the front panel, read it back, record whether the radio
// accepted it). The entry lives in this package's register rather than
// the profile's, because a civ.Profile cannot express a cross-field rule
// and this is the code that enforces it.
//
// Refusing is the conservative direction either way: a combination the
// radio would have rejected costs the user an error message, and one it
// would have silently reinterpreted costs them a channel that is not what
// they asked for.
func crossFieldRefusal(slot string, d codeplug.ChannelData) error {
	duplex, mode := d.Duplex.Value, d.Mode

	if duplex == duplexRPS && mode != modeDD {
		return &driver.WriteRefusedError{
			Slot:   slot,
			Fields: []spec.Field{spec.FieldDuplex, spec.FieldMode},
			Reason: fmt.Sprintf("duplex %q needs mode %q, and this channel is mode %q (PDF p.19, folio 18, the ⑭ breakout's note; ASSUMED for the record, ic905.mode_band_constraint, lift ic905-R-19)", duplex, modeDD, mode),
		}
	}
	if (duplex == duplexUp || duplex == duplexDown) && mode == modeDD {
		return &driver.WriteRefusedError{
			Slot:   slot,
			Fields: []spec.Field{spec.FieldDuplex, spec.FieldMode},
			Reason: fmt.Sprintf("duplex %q cannot be set in mode %q, where the page offers %q instead (PDF p.19, folio 18, the ⑭ breakout's note; ASSUMED for the record, ic905.mode_band_constraint, lift ic905-R-19)", duplex, modeDD, duplexRPS),
		}
	}
	if (mode == modeDD || mode == modeATV) && d.FreqHz < band1200FloorHz {
		return &driver.WriteRefusedError{
			Slot:   slot,
			Fields: []spec.Field{spec.FieldMode, spec.FieldFrequency},
			Reason: fmt.Sprintf("mode %q can be set only on the 1200 MHz band or higher, and %d Hz is below that band's floor of %d Hz (PDF p.17, folio 16, mode table footnote; PDF p.20, folio 19, band table row 03; ASSUMED for the record, ic905.mode_band_constraint, lift ic905-R-19)", mode, d.FreqHz, band1200FloorHz),
		}
	}
	return nil
}

// recordFor maps a neutral channel into the civ.MemoryRecord
// BuildMemorySet encodes.
//
// EVERY MAPPED FIELD BUT THE TONES COMES FROM THE CHANNEL, and rung 4 has
// already guaranteed each of them is Known. The tones are ruling T1(4):
// a Known value is that value, and a NON-Known one is the JUST-READ
// RECORD'S OWN TONE NUMBER, VERBATIM.
//
// THAT IS PRESERVATION, NOT SYNTHESIS, and the distinction is the whole
// of T1(4): nothing is chosen, invented or defaulted — the radio's own
// byte is put back, and it is available precisely because the E6/T5 read
// at rung 9 is already mandatory. On an ADD there is no such source,
// which is why the create refuses instead.
//
// Byte ⑤ and the other unmapped runs are deliberately absent: they are
// the profile's Fixed template's, and rung 10 has already refused any
// stored record whose own bytes differ from it.
func (s *Session) recordFor(addr civ.ChannelAddress, d codeplug.ChannelData, prior civ.MemoryRecord, occupied bool) civ.MemoryRecord {
	toneTX := uint64(d.ToneTx.Value)
	if d.ToneTx.State != codeplug.Known && occupied {
		toneTX, _ = prior.ToneTXDeciHz.Get()
	}
	toneRX := uint64(d.ToneRx.Value)
	if d.ToneRx.State != codeplug.Known && occupied {
		toneRX, _ = prior.ToneRXDeciHz.Get()
	}

	dataMode := dataModeOff
	if d.DataMode.Value {
		dataMode = dataModeOn
	}

	return civ.MemoryRecord{
		Address:      addr,
		RXFreqHz:     civ.Available(d.FreqHz),
		Mode:         civ.Available(d.Mode),
		Filter:       civ.Available(d.Filter.Value),
		DataMode:     civ.Available(dataMode),
		Duplex:       civ.Available(d.Duplex.Value),
		ToneMode:     civ.Available(d.ToneMode.Value),
		ToneTXDeciHz: civ.Available(toneTX),
		ToneRXDeciHz: civ.Available(toneRX),
		DTCSPolarity: civ.Available(d.DTCSPolarity.Value),
		DTCSCode:     civ.Available(uint64(d.DTCSCode.Value)),
		OffsetHz:     civ.Available(d.OffsetHz.Value),
		Name:         civ.Available(d.Tag),
	}
}

// dataModeOff is byte ⑬'s printed OFF spelling — the other half of the
// pair read.go's dataModeOn names.
const dataModeOff = "OFF"

// offsetBounds derives the duplex-offset field's own limits from the
// PROFILE'S LAYOUT rather than restating them: the largest value its BCD
// digits can carry at its declared scale, and the scale itself, which is
// also its step.
//
// DERIVED, NOT WRITTEN DOWN, and that is the point. The span at ㉖~㉘ is
// three bytes — six BCD digits — at Scale 100, so the field reaches
// 999999 × 100 = 99,999,900 Hz in 100 Hz steps. A literal here would be a
// second copy of the layout's own arithmetic, free to drift from it; this
// way a layout change moves the check with it.
//
// ok is false only for a profile with no offset span at all, which is
// unreachable for this model and is refused rather than guessed at.
func offsetBounds(p civ.Profile) (maxHz, stepHz uint64, ok bool) {
	layout, found := p.LayoutFor(civic905.RecordLengthShort)
	if !found {
		return 0, 0, false
	}
	for _, span := range layout.Fields {
		if span.Field != civ.FieldOffset {
			continue
		}
		max := uint64(1)
		for i := 0; i < 2*span.Length; i++ {
			max *= 10
		}
		return (max - 1) * span.Scale, span.Scale, true
	}
	return 0, 0, false
}

// unsayable reports the first field carrying a Known value THIS RADIO
// CANNOT SAY, and why — the five vocabularies, the tag and the offset.
//
// RULING T5 NAMES VOCABULARIES AND FIELD VALIDITY among the refusals that
// precede ALL wire traffic, and every question here is locally decidable:
// the vocabularies, the tag's length and charset, and the offset field's
// own arithmetic are all on the session's own capabilities and the
// profile's own layout, with nothing to look up on the radio.
//
// LEFT TO THE BUILDER, THE FAILURE IS WRONG IN TWO WAYS AT ONCE.
// civ.BuildMemorySet does refuse every one of these values — but by then
// rung 9's preservation read has already put a frame on the wire, and the
// error comes back as a wrapped CODEC error rather than something
// errors.Is(err, driver.ErrWriteRefused) can see, which is the neutral
// contract's own refusal sentinel and what every other rung returns.
//
// THE VOCABULARIES ARE THE CAPABILITIES', NOT THE CODEC'S, deliberately:
// asking caps here means the value the UI offered, the value
// codeplug.Validate judged and the value this rung admits are ONE list.
// Membership is EXACT — not case-folded, not trimmed — because a
// vocabulary value is a wire code's agreed spelling, and quietly
// accepting "usb" for "USB" would be this driver deciding what a user
// meant.
//
// FREQUENCY AND THE TONES ARE NOT HERE: their domains are RANGES rather
// than vocabularies, and they are rungs 5 and 6b. The DTCS code is
// rung 6, for the same reason.
//
// A NON-KNOWN optional field asks nothing of the radio and is not judged:
// Unknown and Unavailable both mean "preserve whatever the radio has".
func unsayable(d codeplug.ChannelData, caps spec.Capabilities) (spec.Field, string) {
	duplexes := make([]string, len(caps.DuplexOptions))
	for i, o := range caps.DuplexOptions {
		duplexes[i] = o.Value
	}
	toneModes := make([]string, len(caps.ToneModes))
	for i, m := range caps.ToneModes {
		toneModes[i] = m.Value
	}

	for _, v := range []struct {
		field spec.Field
		known bool
		value string
		vocab []string
		where string
	}{
		{spec.FieldMode, true, d.Mode, caps.Modes,
			`the record's ⑪ is a mode enum, and PDF p.17 (folio 16)'s "①Operating mode" column prints these codes and no more`},
		{spec.FieldFilter, d.Filter.State == codeplug.Known, d.Filter.Value, caps.Filters,
			`the record's ⑫ is a filter enum, and the same table's "②Filter setting" column prints these three`},
		{spec.FieldDuplex, d.Duplex.State == codeplug.Known, d.Duplex.Value, duplexes,
			"the record's ⑭ HIGH nibble is a duplex enum, and PDF p.19 (folio 18)'s ⑭ breakout prints these four"},
		{spec.FieldToneMode, d.ToneMode.State == codeplug.Known, d.ToneMode.Value, toneModes,
			"the record's ⑭ LOW nibble is a tone-mode enum, and the same breakout prints these eight"},
		{spec.FieldDTCSPolarity, d.DTCSPolarity.State == codeplug.Known, d.DTCSPolarity.Value, caps.DTCSPolarities,
			"the record's ㉒ is a polarity enum, and PDF p.24 (folio 23) prints one nibble per direction"},
	} {
		if !v.known || containsVocab(v.vocab, v.value) {
			continue
		}
		return v.field, fmt.Sprintf(
			"%q is not a value this radio can express (%s: %s). A Known value the wire cannot say faithfully is REFUSED, never dropped and never mapped to a neighbour",
			v.value, v.where, quotedVocab(v.vocab))
	}

	// The tag: sixteen characters fixed (PDF p.19, folio 18, "53~68:
	// Memory name setting (16 characters, fixed)"), over the charset PDF
	// p.20 (folio 19) prints plus the ASSUMED space. Truncating an
	// over-long name would write a name the caller did not choose, and
	// substituting an off-charset byte would write a character they never
	// typed.
	if len(d.Tag) > caps.TagLen {
		return spec.FieldTag, fmt.Sprintf(
			"the name field is %d characters fixed and %q is %d bytes: truncating it would write a name the caller did not choose",
			caps.TagLen, d.Tag, len(d.Tag))
	}
	for i := 0; i < len(d.Tag); i++ {
		if !caps.TagByteOK(d.Tag[i]) {
			return spec.FieldTag, fmt.Sprintf(
				"%q has byte %#02x at offset %d, which is not one this radio's name field can hold (%s)",
				d.Tag, d.Tag[i], i, caps.TagCharsetDescription())
		}
	}

	// The offset: three BCD bytes at 100 Hz resolution (PDF p.18, folio
	// 17, "Duplex Offset frequency setting"). Both halves matter — a
	// value past the field's digits cannot be encoded at all, and one off
	// its 100 Hz scale would be silently rounded by any encoder that
	// tried.
	if d.OffsetHz.State == codeplug.Known {
		maxHz, stepHz, ok := offsetBounds(civic905.Profile())
		switch {
		case !ok:
			return spec.FieldOffset, "this profile declares no duplex-offset span, so no offset can be encoded"
		case d.OffsetHz.Value > maxHz:
			return spec.FieldOffset, fmt.Sprintf(
				"%d Hz is past the ㉖~㉘ field's own ceiling of %d Hz (three packed-BCD bytes at %d Hz resolution)",
				d.OffsetHz.Value, maxHz, stepHz)
		case d.OffsetHz.Value%stepHz != 0:
			return spec.FieldOffset, fmt.Sprintf(
				"%d Hz is not a multiple of the ㉖~㉘ field's %d Hz resolution, and rounding it would write an offset the caller did not choose",
				d.OffsetHz.Value, stepHz)
		}
	}
	return "", ""
}

// containsVocab is EXACT string membership — see unsayable.
func containsVocab(vocab []string, value string) bool {
	for _, v := range vocab {
		if v == value {
			return true
		}
	}
	return false
}

// quotedVocab renders a vocabulary for a refusal message.
func quotedVocab(vocab []string) string {
	out := make([]string, len(vocab))
	for i, v := range vocab {
		out[i] = strconv.Quote(v)
	}
	return strings.Join(out, ", ")
}
