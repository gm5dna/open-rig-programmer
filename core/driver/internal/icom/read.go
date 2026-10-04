// SPDX-License-Identifier: GPL-3.0-or-later

package icom

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// RecordIsAbsent reports whether raw is an all-0xFF record, the reading of
// an unwritten channel.
//
// IT IS THE PRE-PARSE HOOK, and it must run before the record parser,
// because an all-0xFF record dies on its first BCD nibble or its first
// unknown enum value with a failure INDISTINGUISHABLE from a corrupted
// record. Whether an all-0xFF record means empty is the driver's decision
// on its own model's evidence; this is the one place that decision is
// taken. An empty raw is not absent: a zero-length record cannot arise
// (the length fingerprint has already run) and reading "empty" out of one
// would be inventing a result.
func RecordIsAbsent(raw []byte) bool {
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

// ReadRaw performs ONE 1A 00 read and returns the decoded record together
// with its raw bytes.
//
// IT IS THE ONE READ PRIMITIVE. ReadChannel and a write's preservation
// read both go through it, so the address check and the rejection branch
// exist in exactly one place and cannot drift apart.
//
// FA IS AN ERROR, NOT A FRAME: Engine.Do consumes the FA and returns
// transport.ErrRejected with NO frame, so the empty-slot branch keys on
// errors.Is and never on "an FA arrived".
//
// ANSWER-ADDRESS EQUALITY. The landed MemoryAnswerMatcher is deliberately
// envelope-only, so the DRIVER compares the decoded ChannelAddress against
// the one it asked for BEFORE ANY USE of the answer: before empty
// recognition, before record mapping, before a write's template check. A
// mismatch is *driver.AnswerMismatchError plus a count on mismatches,
// never a silently mis-attributed record.
//
// THE RETURN CARRIES BOTH THE RECORD AND THE RAW BYTES because its two
// callers need different halves of the same single exchange: ReadChannel
// maps the decoded record, and a write compares the RAW unmapped nibbles
// against the profile's template, a judgement no decoded value can
// express. Two reads would open a window in which the slot could change
// between them.
func ReadRaw(ctx context.Context, p *Params, eng *transport.Engine, mismatches *atomic.Uint64, a civ.ChannelAddress) (civ.MemoryRecord, []byte, bool, error) {
	prof := p.Profile()
	cmd, err := prof.BuildMemoryRead(a)
	if err != nil {
		return civ.MemoryRecord{}, nil, false, fmt.Errorf("%s: building the 1A 00 read for %s: %w", p.Name, a, err)
	}
	frame, err := eng.Do(ctx, cmd, civ.CIVReadSpec(prof.MemoryAnswerMatcher(), 1))
	if errors.Is(err, transport.ErrRejected) {
		return civ.MemoryRecord{}, nil, true, nil // an empty slot, not an error
	}
	if err != nil {
		return civ.MemoryRecord{}, nil, false, err
	}
	got, raw, err := prof.MemoryAnswerRecord(frame)
	if err != nil {
		// Includes *civ.RecordLengthError, which is the probe's LENGTH
		// FINGERPRINT being continuous rather than one-shot: every record
		// read re-checks it, so a wrong-model session cannot be opened
		// once and then trusted. A wrong LENGTH is a wrong radio; an
		// ABSENT record is an empty slot; the two are different answers.
		//
		// Wrapped with driver.ErrRecordDecode: a genuine record-decode
		// failure, recoverable per-slot by core/clone.Service.readAll
		// rather than fatal to the whole radio read.
		return civ.MemoryRecord{}, nil, false, fmt.Errorf("%w: %w", driver.ErrRecordDecode, err)
	}
	if got != a { // BEFORE any use of raw
		mismatches.Add(1)
		return civ.MemoryRecord{}, nil, false, &driver.AnswerMismatchError[civ.ChannelAddress]{Model: p.Name, Requested: a, Answered: got}
	}
	if RecordIsAbsent(raw) {
		return civ.MemoryRecord{}, nil, true, nil
	}
	// AFTER the all-FF branch and BEFORE the parse: an all-FF record is an
	// empty slot, not a malformed one. The check's error is returned as
	// given, not wrapped with driver.ErrRecordDecode.
	if p.ReadCheck != nil {
		if err := p.ReadCheck(raw); err != nil {
			return civ.MemoryRecord{}, nil, false, err
		}
	}
	rec, err := prof.ParseMemoryAnswer(frame)
	if err != nil {
		return civ.MemoryRecord{}, nil, false, fmt.Errorf("%w: %w", driver.ErrRecordDecode, err)
	}
	return rec, raw, false, nil
}

// ReadChannel is ONE 1A 00 read, mapped into one codeplug.Channel.
//
// AN EMPTY SLOT COMES BACK AS AN EMPTY CHANNEL (Data nil), never an error
// that would abort a caller's ReadAll — the neutral contract at
// core/driver/driver.go. Both empty readings land there: a rejected read
// and an all-0xFF record (see RecordIsAbsent).
//
// A WRONG RECORD LENGTH IS AN ERROR, and deliberately not an empty
// channel: no partial parse, no fake Unavailable channel.
//
// NEVER A GUESSED VALUE ANYWHERE. Every field the 1A 00 record does not
// express comes back Unavailable — "there is no such field" — rather than
// Unknown, which would mean "the radio has one and this read did not learn
// it". The two unmapped nibbles come back Unavailable too: an unmapped
// region is not decoded, so there is nothing to report. That does not
// make the channel unreadable; a write is where such a channel becomes
// unwritable.
//
// THE TONE ARMS: a civ-layer tone number INSIDE the declared domain maps
// to a Known ToneField; one OUTSIDE it — 0 INCLUDED — maps to Unknown. The
// civ layer is lossless and semantics-free: it hands up the number 0
// unharmed from a tone-OFF channel whose bytes are 00 00 00. The
// CAPABILITY does not admit 0, because 0 Hz is not a tone. So the DRIVER
// is where the difference is resolved, and it resolves it towards
// Unknown: A READ NEVER CONSTRUCTS A KNOWN VALUE codeplug.Validate WOULD
// THEN REFUSE.
func ReadChannel(ctx context.Context, p *Params, eng *transport.Engine, caps spec.Capabilities, mismatches *atomic.Uint64, slot string) (codeplug.Channel, error) {
	a, _, err := SlotToAddress(p.Name, slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("%s: ReadChannel: %w", p.Name, err)
	}
	rec, _, empty, err := ReadRaw(ctx, p, eng, mismatches, a)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("%s: ReadChannel %s: %w", p.Name, slot, err)
	}
	if empty {
		return codeplug.Channel{Slot: slot}, nil
	}

	freq, ok := rec.RXFreqHz.Get()
	if !ok {
		// Unreachable through a profile whose layout maps a frequency
		// span, but refused rather than defaulted to zero, which would be
		// a fabricated 0 Hz channel.
		return codeplug.Channel{}, fmt.Errorf("%s: ReadChannel %s: the record carries no frequency", p.Name, slot)
	}
	// A read must not construct a frequency codeplug.Validate will reject.
	// The synthetic ChannelData carries only FreqHz; every other field is
	// at its zero value, which is not FieldState Known, so a domain hook's
	// tone arms never trip on this call.
	if err := p.Domain(codeplug.ChannelData{FreqHz: freq}, caps); err != nil {
		return codeplug.Channel{}, err
	}
	mode, ok := rec.Mode.Get()
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("%s: ReadChannel %s: the record carries no mode", p.Name, slot)
	}
	name, _ := rec.Name.Get()
	txFreq := codeplug.FreqField{State: codeplug.Unavailable}
	if p.TXDuplicate {
		v, ok := rec.TXFreqHz.Get()
		if !ok {
			// Unreachable through a layout that maps the TX-duplicate
			// span unconditionally, but refused rather than defaulted.
			return codeplug.Channel{}, fmt.Errorf("%s: ReadChannel %s: the record carries no TX-duplicate frequency", p.Name, slot)
		}
		// Always on the wire, so always Known, never Unavailable; this
		// read reports whatever the radio held.
		txFreq = codeplug.FreqField{State: codeplug.Known, Value: v}
	}

	data := &codeplug.ChannelData{
		FreqHz: freq,
		Mode:   mode,
		Tag:    name,

		// The three remaining mapped fields, each a tri-state carrying
		// what the record said.
		Filter:   optionalString(rec.Filter),
		ToneMode: optionalString(rec.ToneMode),
		ToneTx:   toneField(caps, rec.ToneTXDeciHz),
		ToneRx:   toneField(caps, rec.ToneRXDeciHz),

		// UNAVAILABLE, because the 1A 00 record has no such field. Not
		// Unknown: Unknown would claim the radio has one and this read
		// did not learn it.
		TxFreqHz:     txFreq,
		TagDisplay:   codeplug.BoolField{State: codeplug.Unavailable},
		CTCSSTone:    codeplug.ToneField{State: codeplug.Unavailable},
		Duplex:       codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz:     codeplug.FreqField{State: codeplug.Unavailable},
		DTCSCode:     codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},

		// UNAVAILABLE because their nibbles are UNMAPPED: the bytes are
		// on the wire and this driver deliberately does not decode them.
		ScanSkip:            codeplug.BoolField{State: codeplug.Unavailable},
		DataMode:            codeplug.BoolField{State: codeplug.Unavailable},
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
		Preamp:              codeplug.StringField{State: codeplug.Unavailable},
		Antenna:             codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},
		SatBandSwap:         codeplug.BoolField{State: codeplug.Unavailable},
		SatTrace:            codeplug.BoolField{State: codeplug.Unavailable},
		SatTraceRev:         codeplug.BoolField{State: codeplug.Unavailable},
	}

	// The Yaesu-shaped plain fields (ClarHz/RxClar/TxClar, CTCSS, Shift)
	// are left at their zero values: they carry no state, this radio's
	// record has no such field, and its capabilities declare neither
	// vocabulary — so codeplug.Validate's Yaesu checks, which key on the
	// VOCABULARY being supplied, do not run on them.

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

// toneField maps a civ-layer tone number to a codeplug.ToneField.
//
// The predicate is spec.Capabilities.AdmitsTone — the ONE predicate, and
// the same one codeplug.ToneField.Valid consults — asked of THIS SESSION'S
// capabilities, so the read arm and the validator can never disagree about
// what this radio admits. Anything outside the domain, 0 included, becomes
// UNKNOWN: "preserve whatever the radio currently has" is the only honest
// instruction for a value this project must not hand on as Known.
func toneField(caps spec.Capabilities, o civ.Optional[uint64]) codeplug.ToneField {
	v, ok := o.Get()
	if !ok {
		return codeplug.ToneField{State: codeplug.Unavailable}
	}
	t := spec.Tone(v)
	if !caps.AdmitsTone(t) {
		return codeplug.ToneField{State: codeplug.Unknown}
	}
	return codeplug.ToneField{State: codeplug.Known, Value: t}
}
