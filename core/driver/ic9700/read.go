// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// ErrAnswerMismatch is the sentinel a caller compares against (errors.Is)
// when a memory answer names a DIFFERENT slot than the one requested.
//
// THE CHECK IS THE DRIVER'S BECAUSE THE MATCHER CANNOT MAKE IT. The landed
// Profile.MemoryAnswerMatcher is deliberately ENVELOPE-ONLY: it checks the
// frame's envelope and the minimum address width and matches no channel
// address at all (core/civ/framing.go's own doc comment says why — an
// address-matching matcher would turn any difference in address ENCODING
// into a silent timeout on a tier whose encoding is assumed throughout).
// So a reply for another slot satisfies the matcher and reaches this
// package, and only a decoded-address comparison here can catch it.
//
// core/driver/ftdx101's same-shaped pair is the precedent; this is this
// package's own, in its own namespace, because a caller distinguishing
// which radio's read went wrong needs distinct types.
var ErrAnswerMismatch = errors.New("ic9700: answer names a different slot than was requested")

// AnswerMismatchError reports the requested and the answered address.
type AnswerMismatchError struct {
	Requested civ.ChannelAddress
	Answered  civ.ChannelAddress
}

func (e *AnswerMismatchError) Error() string {
	return fmt.Sprintf("ic9700: requested %v but the answer names %v — refusing to map a reply onto the wrong slot",
		e.Requested, e.Answered)
}

// Unwrap lets errors.Is(err, ErrAnswerMismatch) match.
func (e *AnswerMismatchError) Unwrap() error { return ErrAnswerMismatch }

// ReadChannel implements driver.Session: one `1A 00` read of one memory
// slot, mapped into the neutral codeplug model.
//
// THE ORDER OF WHAT FOLLOWS IS THE WHOLE OF R10 AND T2, and none of it may
// be rearranged:
//
//  1. the slot string becomes a wire address, or the read is refused
//     before any traffic;
//  2. the read goes out through civ.CIVReadSpec and the codec's own
//     matcher — no CommandSpec literal, no hand-rolled matcher;
//  3. T4: a REJECTION is an EMPTY CHANNEL, not an error;
//  4. the answer is split into address and RAW record bytes, with a
//     length outside this profile's accepted set propagating as an error;
//  5. T2: the decoded address is compared with the requested one BEFORE
//     ANY USE WHATSOEVER of the record;
//  6. an all-0xFF record is the second empty form, recognised BEFORE the
//     parser;
//  7. and only then is the record parsed and mapped.
//
// AN EMPTY SLOT IS NOT AN ERROR, and getting that backwards is what would
// break every clone of a partly filled radio. driver.Session's contract
// states it and core/clone/read.go depends on it: that file aborts the
// whole ReadAll on the first error, so reporting an unwritten channel as a
// failure would turn the ordinary case — a radio with some channels used —
// into a walk that never finishes.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	ch, _, _, err := s.readChannelRaw(ctx, slot)
	return ch, err
}

// readChannelRaw is ReadChannel plus the RAW record bytes the answer
// carried AND the decoded civ.MemoryRecord behind them, both of which the
// write path needs: the bytes for the E6 template guard, the record for
// the value-level preservation of a field the caller left non-Known.
//
// BOTH ARE RETURNED RATHER THAN FETCHED AFTERWARDS, so the write path is
// provably looking at THIS read's answer. Preservation-by-cache is what R6
// forbids, and a value that outlived its read is exactly what it forbids
// reaching for.
//
// An empty slot returns the zero record: there is nothing to preserve, and
// the write path's CREATE branch is what handles that case.
func (s *Session) readChannelRaw(ctx context.Context, slot string) (codeplug.Channel, []byte, civ.MemoryRecord, error) {
	addr, _, err := slotAddress(slot)
	if err != nil {
		return codeplug.Channel{}, nil, civ.MemoryRecord{}, err
	}
	cmd, err := s.profile.BuildMemoryRead(addr)
	if err != nil {
		return codeplug.Channel{}, nil, civ.MemoryRecord{}, fmt.Errorf("ic9700: ReadChannel %s: %w", slot, err)
	}

	answer, err := s.eng.Do(ctx, cmd, civ.CIVReadSpec(s.profile.MemoryAnswerMatcher(), retryReads))
	if errors.Is(err, transport.ErrRejected) {
		// D5 entry 2(a), lift R11 — and T4: Do CONSUMED the FA and
		// returned no frame, so there is no rejection frame to inspect
		// and no branch here that could inspect one.
		s.rememberRaw(slot, nil)
		return codeplug.Channel{Slot: slot}, nil, civ.MemoryRecord{}, nil
	}
	if err != nil {
		return codeplug.Channel{}, nil, civ.MemoryRecord{}, fmt.Errorf("ic9700: ReadChannel %s: %w", slot, err)
	}

	// The pre-parse hook: address and RAW record, with no field decoded.
	// A *civ.RecordLengthError from here PROPAGATES — a malformed record
	// is not an empty slot, and spec D4's adjudication 13 says a read of
	// one should abort the walk.
	got, record, err := s.profile.MemoryAnswerRecord(answer)
	if err != nil {
		return codeplug.Channel{}, nil, civ.MemoryRecord{}, err
	}

	// T2, BEFORE ANY USE. Nothing below this line runs for a mismatched
	// answer, and nothing above it used the record for anything.
	if got != addr {
		s.noteAnswerMismatch()
		return codeplug.Channel{}, nil, civ.MemoryRecord{}, &AnswerMismatchError{Requested: addr, Answered: got}
	}

	if allFF(record) {
		// D5 entry 2(b), lift R12. It is recognised HERE, before the
		// parser, because civ.decodeRecord would reject 0xFF against
		// this profile's enums and report a parse error — a failure
		// indistinguishable from a corrupted record, which is not what
		// an empty slot is.
		s.rememberRaw(slot, nil)
		return codeplug.Channel{Slot: slot}, nil, civ.MemoryRecord{}, nil
	}

	rec, err := s.profile.ParseMemoryAnswer(answer)
	if err != nil {
		return codeplug.Channel{}, nil, civ.MemoryRecord{}, err
	}
	s.rememberRaw(slot, record)

	data := s.channelData(rec)
	return codeplug.Channel{Slot: slot, Data: &data}, record, rec, nil
}

// allFF reports whether every byte of record is 0xFF — this radio's
// second way of saying a channel is empty.
//
// An EMPTY record is not all-FF: len 0 would answer true vacuously, and
// the profile's accepted length set has no zero in it, but the guard is
// cheap and the alternative reads as "an answer carrying nothing means the
// channel is empty", which is a different and unevidenced claim.
func allFF(record []byte) bool {
	if len(record) == 0 {
		return false
	}
	for _, b := range record {
		if b != 0xFF {
			return false
		}
	}
	return true
}

// channelData maps one decoded civ.MemoryRecord onto the neutral model.
//
// WHAT IS ABSENT IS AS DELIBERATE AS WHAT IS PRESENT. The Yaesu fields —
// the clarifier trio, CTCSS state, CTCSS tone, shift, tag display — are
// reported as this radio not having them, because it does not: the record
// carries no clarifier and no display flag (matrix §1 #6/#7), and this
// family expresses duplex/tone_mode/tone_tx/tone_rx instead of the Yaesu
// shift/CTCSS pair. Reporting a guessed value for any of them would be a
// claim about a radio, made up.
//
// ScanSkip is Unavailable rather than derived from ④, and that is OQ-4's
// whole point: ④ is a four-valued SELECT-memory group tag (0=OFF, 1=★1,
// 2=★2, 3=★3) and the neutral field is a boolean. The dialect DOES decode
// it — the value is read and is visible — and this driver declines to
// present it as something it is not.
func (s *Session) channelData(rec civ.MemoryRecord) codeplug.ChannelData {
	data := codeplug.ChannelData{
		FreqHz: numberOf(rec.RXFreqHz),
		Mode:   stringOf(rec.Mode),
		Tag:    stringOf(rec.Name),

		// The fields this radio does not have. Unavailable is the
		// POSITIVE statement "this radio/protocol has no such field",
		// which is what a file must record and what the write path's
		// requested-field derivation keys on.
		CTCSSTone:  codeplug.ToneField{State: codeplug.Unavailable},
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
		ScanSkip:   codeplug.BoolField{State: codeplug.Unavailable},

		TxFreqHz: codeplug.FreqField{State: codeplug.Known, Value: numberOf(rec.TXFreqHz)},
		OffsetHz: codeplug.FreqField{State: codeplug.Known, Value: numberOf(rec.OffsetHz)},

		Duplex:       s.vocabField(rec.Duplex, duplexValues(s.caps)),
		ToneMode:     s.vocabField(rec.ToneMode, toneModeValues(s.caps)),
		DTCSPolarity: s.vocabField(rec.DTCSPolarity, s.caps.DTCSPolarities),
		Filter:       s.vocabField(rec.Filter, s.caps.Filters),

		ToneTx: s.toneField(rec.ToneTXDeciHz),
		ToneRx: s.toneField(rec.ToneRXDeciHz),

		DTCSCode: s.dtcsField(rec.DTCSCode),
		DataMode: codeplug.BoolField{State: codeplug.Known, Value: stringOf(rec.DataMode) == "ON"},
	}
	return data
}

// vocabField maps a decoded enum onto a neutral StringField, reporting
// Unknown for a value this radio's own vocabulary cannot name.
//
// THE ONE VALUE THAT REACHES THAT ARM TODAY IS `RPS` (OQ-6). The dialect
// carries all four of ⑬'s printed high-nibble values so the record
// round-trips exactly, while core/spec declares three DuplexDirections and
// RPS has no direction to give. Unknown is the honest report — "this radio
// has a value here this codeplug's vocabulary cannot name". Flattening it
// onto OFF would be a lie about the radio and would make the write-side
// refusal impossible, because nothing downstream would know; Unavailable
// would be a lie about the field, which plainly exists.
func (s *Session) vocabField(opt civ.Optional[string], vocab []string) codeplug.StringField {
	v, ok := opt.Get()
	if !ok {
		return codeplug.StringField{State: codeplug.Unavailable}
	}
	for _, allowed := range vocab {
		if allowed == v {
			return codeplug.StringField{State: codeplug.Known, Value: v}
		}
	}
	return codeplug.StringField{State: codeplug.Unknown}
}

// toneField maps a decoded tone number onto a neutral ToneField, applying
// tier ruling T1(3).
//
// A READ NEVER CONSTRUCTS A Known VALUE THE VALIDATOR WOULD REFUSE. The
// civ layer is lossless and semantics-free: it decodes ⑮~⑰ and ⑱~⑳ as
// plain BCD numbers over the whole encodable range, ZERO INCLUDED. The
// capability domain starts at 1 deciHz, because zero is not a tone and the
// landed spec.ToneRange refuses a minimum at or below it. So the one
// encodable value the capability excludes is zero, and it — like anything
// else outside the declared domain — comes back Unknown rather than as a
// Known value codeplug.ToneField.Valid would then reject.
func (s *Session) toneField(opt civ.Optional[uint64]) codeplug.ToneField {
	v, ok := opt.Get()
	if !ok {
		return codeplug.ToneField{State: codeplug.Unavailable}
	}
	tone := spec.Tone(v)
	if !s.caps.AdmitsTone(tone) {
		return codeplug.ToneField{State: codeplug.Unknown}
	}
	return codeplug.ToneField{State: codeplug.Known, Value: tone}
}

// dtcsField maps a decoded DTCS code onto a neutral IntField, on
// toneField's rule exactly.
//
// The rule is the same because the hazard is: ㉒㉓ is packed BCD and can
// carry any three digits the wire holds, while this radio's printed domain
// is the 512 codes whose every digit is 0..7. A code with an 8 or a 9 in
// it is a value codeplug.IntField.Valid refuses, so a read must not
// present one as Known.
func (s *Session) dtcsField(opt civ.Optional[uint64]) codeplug.IntField {
	v, ok := opt.Get()
	if !ok {
		return codeplug.IntField{State: codeplug.Unavailable}
	}
	code := int(v)
	for _, allowed := range s.caps.DTCSCodes {
		if allowed == code {
			return codeplug.IntField{State: codeplug.Known, Value: code}
		}
	}
	return codeplug.IntField{State: codeplug.Unknown}
}

// duplexValues and toneModeValues flatten the two Icom vocabularies to the
// plain string lists codeplug.StringField.Valid takes. They exist here
// rather than in caps.go because this is the only direction that needs
// them: caps declares the semantics, and the neutral field only ever asks
// which spellings are legal.
func duplexValues(caps spec.Capabilities) []string {
	out := make([]string, 0, len(caps.DuplexOptions))
	for _, o := range caps.DuplexOptions {
		out = append(out, o.Value)
	}
	return out
}

func toneModeValues(caps spec.Capabilities) []string {
	out := make([]string, 0, len(caps.ToneModes))
	for _, m := range caps.ToneModes {
		out = append(out, m.Value)
	}
	return out
}

// numberOf and stringOf unwrap a civ.Optional, treating an absent value as
// the zero. Every field this profile's layout maps is present by
// construction — civ's own validator refuses a record missing one — so the
// zero arm is unreachable for a record that parsed, and it is written out
// rather than asserted because the alternative is a panic in a read path.
func numberOf(opt civ.Optional[uint64]) uint64 {
	v, _ := opt.Get()
	return v
}

func stringOf(opt civ.Optional[string]) string {
	v, _ := opt.Get()
	return v
}
