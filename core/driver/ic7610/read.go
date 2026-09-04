// SPDX-License-Identifier: GPL-3.0-or-later

package ic7610

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7610 "github.com/gm5dna/open-rig-programmer/core/civ/ic7610"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// The two scan edges' wire channel numbers.
//
// MATRIX §3.15(d): P1 AND P2 ARE NOT A SEPARATE BANK IN THE WIRE
// PROTOCOL. They are two more values of the same two-byte selector — PDF
// p.12 (folio 11) prints "00 01 ~ 00 99: Memory channel 01 ~ 99",
// "01 00: Programmed scan edge P1", "01 01: Programmed scan edge P2", one
// contiguous space with three printed forms — and core/civ/ic7610's
// profile declares exactly that range (ChannelLo 1, ChannelHi 101).
//
// This project models them as a SCAN bank only because the neutral memory
// model needs the distinction between a memory and a scan edge; the codec
// knows nothing of it.
const (
	scanEdgeP1Channel = 100
	scanEdgeP2Channel = 101
	lastMemoryChannel = 99
)

// slotToAddress maps a canonical wire-form slot to the channel address the
// codec addresses it by, and to the bank it belongs to.
//
// "001".."099" are the memories; "P1" and "P2" are the scan edges. Nothing
// else is a slot on this radio: "000" (there is no channel zero), "100"
// (the memories stop at 99), "P0"/"P3", a bare "1", "" and the sparse
// group form "G05-012" (which belongs to the group-addressed models, not
// to this flat one) are all errors.
func slotToAddress(slot string) (civ.ChannelAddress, spec.BankID, error) {
	switch slot {
	case "P1":
		return civ.ChannelAddress{Channel: scanEdgeP1Channel}, spec.BankScan, nil
	case "P2":
		return civ.ChannelAddress{Channel: scanEdgeP2Channel}, spec.BankScan, nil
	}
	if len(slot) != 3 {
		return civ.ChannelAddress{}, "", fmt.Errorf("ic7610: %q is not a slot on this radio: a memory is three digits (\"001\"..\"099\") and a scan edge is \"P1\" or \"P2\"", slot)
	}
	n, err := strconv.Atoi(slot)
	if err != nil {
		return civ.ChannelAddress{}, "", fmt.Errorf("ic7610: %q is not a slot on this radio: %w", slot, err)
	}
	if n < 1 || n > lastMemoryChannel {
		return civ.ChannelAddress{}, "", fmt.Errorf("ic7610: %q is outside this radio's memory range \"001\"..\"099\"", slot)
	}
	return civ.ChannelAddress{Channel: n}, spec.BankMemory, nil
}

// addressToSlot is slotToAddress's inverse.
func addressToSlot(a civ.ChannelAddress) (string, error) {
	if a.Group != 0 {
		return "", fmt.Errorf("ic7610: %s carries a group index; this radio's channel selector is a flat two-byte number", a)
	}
	switch a.Channel {
	case scanEdgeP1Channel:
		return "P1", nil
	case scanEdgeP2Channel:
		return "P2", nil
	}
	if a.Channel < 1 || a.Channel > lastMemoryChannel {
		return "", fmt.Errorf("ic7610: channel %d is outside this radio's addressable space (1..99, plus 100 and 101 for the scan edges)", a.Channel)
	}
	return fmt.Sprintf("%03d", a.Channel), nil
}

// recordIsAbsent reports whether raw is an all-0xFF record — spec D5 entry
// 2(b)'s reading of an unwritten channel.
//
// IT IS THE R10 PRE-PARSE HOOK, and it must run before the record parser,
// because an all-0xFF record dies on its first BCD nibble or its first
// unknown enum value with a failure INDISTINGUISHABLE from a corrupted
// record. civ.Profile.MemoryAnswerRecord exists for exactly this — its own
// doc comment says the split is offered and the judgement is not, because
// "whether an all-0xFF record means empty is the driver's decision on its
// own model's evidence".
//
// THE EVIDENCE IS TWO SEPARATE, UNVERIFIED ENTRIES AND ONE CAPTURE CANNOT
// ESTABLISH BOTH. D5 entry 2(a) / matrix lift R2a is the FA reading;
// D5 entry 2(b) / matrix lift R2b is this one. R2a's scope EXCLUDES the
// scan edges (matrix erratum 2), so P1/P2 emptiness rides matrix lift R18
// instead — and R18's capture names P1 only, so P2 is uncovered even by
// that.
//
// An empty raw is not absent: a zero-length record cannot arise (the
// length fingerprint has already run) and reading "empty" out of one would
// be inventing a result.
func recordIsAbsent(raw []byte) bool {
	if len(raw) == 0 {
		return false
	}
	for _, b := range raw {
		if b != 0xFF {
			return false
		}
	}
	return true
}

// readRaw performs ONE 1A 00 read and returns the decoded record together
// with its raw bytes.
//
// IT IS THE ONE READ PRIMITIVE. ReadChannel and WriteChannel's E6
// preservation read both go through it, so the T2 address check and the T4
// rejection branch exist in exactly one place and cannot drift apart.
//
// T4 — FA IS AN ERROR, NOT A FRAME. Engine.Do consumes the FA and returns
// transport.ErrRejected with NO frame, so the empty-slot branch keys on
// errors.Is(err, transport.ErrRejected) and never on "an FA arrived".
// Nothing in this driver calls civ.IsRejection; that stays the framing's
// internal concern.
//
// T2 — ANSWER-ADDRESS EQUALITY. The landed MemoryAnswerMatcher is
// deliberately envelope-only (to/from/cn/sc), so the DRIVER compares the
// decoded ChannelAddress against the one it asked for BEFORE ANY USE of
// the answer: before empty recognition, before any caching, before record
// mapping, before WriteChannel's E6 template check and before a write
// merge. A mismatch is *AnswerMismatchError plus a diagnostic count, never
// a silently mis-attributed record.
//
// THE RETURN CARRIES BOTH THE RECORD AND THE RAW BYTES because its two
// callers need different halves of the same single exchange: ReadChannel
// maps the decoded record, and WriteChannel's E6 rung compares the RAW
// unmapped nibbles against the profile's Fixed template — a judgement no
// decoded value can express, since an unmapped region is by definition not
// decoded. Splitting them into two reads would put a second exchange on
// the wire and open a window in which the slot could change between them.
func (s *Session) readRaw(ctx context.Context, a civ.ChannelAddress) (civ.MemoryRecord, []byte, bool, error) {
	p := civic7610.Profile()
	cmd, err := p.BuildMemoryRead(a)
	if err != nil {
		return civ.MemoryRecord{}, nil, false, fmt.Errorf("ic7610: building the 1A 00 read for %s: %w", a, err)
	}
	frame, err := s.eng.Do(ctx, cmd, civ.CIVReadSpec(p.MemoryAnswerMatcher(), 1))
	if errors.Is(err, transport.ErrRejected) {
		return civ.MemoryRecord{}, nil, true, nil // T4: an empty slot, not an error
	}
	if err != nil {
		return civ.MemoryRecord{}, nil, false, err
	}
	got, raw, err := p.MemoryAnswerRecord(frame)
	if err != nil {
		// Includes *civ.RecordLengthError, which is the probe's LENGTH
		// FINGERPRINT being continuous rather than one-shot: every record
		// read re-checks it, so a wrong-model session cannot be opened
		// once and then trusted. A wrong LENGTH is a wrong radio; an
		// ABSENT record is an empty slot; the two are different answers.
		return civ.MemoryRecord{}, nil, false, err
	}
	if got != a { // T2, BEFORE any use of raw
		s.answerMismatches.Add(1)
		return civ.MemoryRecord{}, nil, false, &AnswerMismatchError{Want: a, Got: got}
	}
	if recordIsAbsent(raw) {
		return civ.MemoryRecord{}, nil, true, nil
	}
	rec, err := p.ParseMemoryAnswer(frame)
	if err != nil {
		return civ.MemoryRecord{}, nil, false, err
	}
	return rec, raw, false, nil
}

// AnswerMismatches reports how many memory answers this session has seen
// whose decoded channel address was not the one requested (tier ruling
// T2). A diagnostic count beside the typed error, so a bus that
// occasionally mis-attributes is visible even when each individual read
// was refused correctly.
func (s *Session) AnswerMismatches() uint64 { return s.answerMismatches.Load() }

// ReadChannel implements driver.Session: ONE 1A 00 read, mapped into one
// codeplug.Channel.
//
// AN EMPTY SLOT COMES BACK AS AN EMPTY CHANNEL (Data nil), never an error
// that would abort a caller's ReadAll — the neutral contract at
// core/driver/driver.go. Both of this model's two unverified empty
// readings land there: a rejected read (T4) and an all-0xFF record (see
// recordIsAbsent).
//
// A WRONG RECORD LENGTH IS AN ERROR, and deliberately not an empty
// channel: no partial parse, no fake Unavailable channel (spec D4,
// adjudication 13).
//
// NEVER A GUESSED VALUE ANYWHERE. Every field the 1A 00 record does not
// express comes back Unavailable — "there is no such field" — rather than
// Unknown, which would mean "the radio has one and this read did not learn
// it". The two E6-unmapped nibbles come back Unavailable too: an unmapped
// region is not decoded, so there is nothing to report.
//
// THE TONE ARMS ARE TIER RULING T1(3). A civ-layer tone number INSIDE the
// declared domain maps to a Known ToneField; one OUTSIDE it — 0 INCLUDED —
// maps to Unknown. The civ layer is lossless and semantics-free (T1(1)):
// it hands up the number 0 unharmed from a tone-OFF channel whose bytes
// are 00 00 00. The CAPABILITY does not admit 0, because 0 Hz is not a
// tone (T1(2)). So the DRIVER is where the difference is resolved, and it
// resolves it towards Unknown: A READ NEVER CONSTRUCTS A KNOWN VALUE
// codeplug.Validate WOULD THEN REFUSE.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	a, _, err := slotToAddress(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ic7610: ReadChannel: %w", err)
	}
	rec, _, empty, err := s.readRaw(ctx, a)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ic7610: ReadChannel %s: %w", slot, err)
	}
	if empty {
		return codeplug.Channel{Slot: slot}, nil
	}

	freq, ok := rec.RXFreqHz.Get()
	if !ok {
		// Unreachable through this profile — the layout maps a frequency
		// span, so a successfully parsed record always carries one — but
		// refused rather than defaulted to zero, which would be a
		// fabricated 0 Hz channel.
		return codeplug.Channel{}, fmt.Errorf("ic7610: ReadChannel %s: the record carries no frequency", slot)
	}
	// TestReadChannel_RefusesFrequencyAboveCeiling pins this refusal: a
	// read must not construct a frequency codeplug.Validate will reject.
	// TestReadChannel_AcceptsFrequencyAtCeiling pins the boundary itself:
	// the strict > below refuses only what exceeds the ceiling, not the
	// ceiling value. There is no floor check, by design: MinFreqHz is
	// deliberately zero (caps.go:367), so no decoded frequency can fall
	// below it.
	if freq > MaxEncodableFreqHz {
		return codeplug.Channel{}, &OutOfDomainError{
			Field: spec.FieldFrequency, Value: freq, Max: MaxEncodableFreqHz,
		}
	}
	mode, ok := rec.Mode.Get()
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ic7610: ReadChannel %s: the record carries no mode", slot)
	}
	name, _ := rec.Name.Get()

	data := &codeplug.ChannelData{
		FreqHz: freq,
		Mode:   mode,
		Tag:    name,

		// The three remaining mapped fields, each a tri-state carrying
		// what the record said.
		Filter:   optionalString(rec.Filter),
		ToneMode: optionalString(rec.ToneMode),
		ToneTx:   s.toneField(rec.ToneTXDeciHz),
		ToneRx:   s.toneField(rec.ToneRXDeciHz),

		// UNAVAILABLE, because the 1A 00 record has no such field. Not
		// Unknown: Unknown would claim the radio has one and this read
		// did not learn it.
		TagDisplay:   codeplug.BoolField{State: codeplug.Unavailable},
		CTCSSTone:    codeplug.ToneField{State: codeplug.Unavailable},
		TxFreqHz:     codeplug.FreqField{State: codeplug.Unavailable},
		Duplex:       codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz:     codeplug.FreqField{State: codeplug.Unavailable},
		DTCSCode:     codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},

		// UNAVAILABLE because ruling E6 leaves their nibbles UNMAPPED.
		// The bytes are on the wire and this driver deliberately does not
		// decode them: byte ③'s low nibble is a four-valued SELECT-group
		// marker and byte ⑪'s high nibble a four-valued data mode, and
		// both neutral homes are BoolField. E6 unmaps them; it does NOT
		// make the channel unreadable — WriteChannel is where such a
		// channel becomes unwritable.
		ScanSkip:            codeplug.BoolField{State: codeplug.Unavailable},
		DataMode:            codeplug.BoolField{State: codeplug.Unavailable},
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
		Preamp:              codeplug.StringField{State: codeplug.Unavailable},
		Antenna:             codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},
	}

	// The Yaesu-shaped plain fields (ClarHz/RxClar/TxClar, CTCSS, Shift)
	// are left at their zero values and are NOT written down as anything:
	// they carry no state, this radio's record has no such field, and this
	// driver's capabilities declare neither vocabulary — so
	// codeplug.Validate's Yaesu checks, which key on the VOCABULARY being
	// supplied, do not run on them.

	return codeplug.Channel{Slot: slot, Data: data}, nil
}

// optionalString maps a civ tri-state string to codeplug's: present
// becomes Known, absent becomes Unavailable. Never Unknown — an absent
// Optional on this codec means the layout has no such span.
func optionalString(o civ.Optional[string]) codeplug.StringField {
	v, ok := o.Get()
	if !ok {
		return codeplug.StringField{State: codeplug.Unavailable}
	}
	return codeplug.StringField{State: codeplug.Known, Value: v}
}

// toneField maps a civ-layer tone number to a codeplug.ToneField under
// tier ruling T1(3).
//
// The predicate is spec.Capabilities.AdmitsTone — E3's ONE predicate, and
// the same one codeplug.ToneField.Valid consults — asked of THIS SESSION'S
// capabilities, so the read arm and the validator can never disagree about
// what this radio admits. Anything outside the domain, 0 included, becomes
// UNKNOWN: "preserve whatever the radio currently has" is the only honest
// instruction for a value this project must not hand on as Known.
func (s *Session) toneField(o civ.Optional[uint64]) codeplug.ToneField {
	v, ok := o.Get()
	if !ok {
		return codeplug.ToneField{State: codeplug.Unavailable}
	}
	t := spec.Tone(v)
	if !s.caps.AdmitsTone(t) {
		return codeplug.ToneField{State: codeplug.Unknown}
	}
	return codeplug.ToneField{State: codeplug.Known, Value: t}
}
