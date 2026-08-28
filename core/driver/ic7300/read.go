// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// parseSlot maps one canonical slot string onto this model's channel
// address and bank, and reports whether the slot exists at all.
//
// IT IS D11'S TABLE, and it is the ONLY place that table is written down in
// this package — the write path reuses it rather than restating it, so the
// two cannot drift:
//
//	"001".."099"  ->  wire 00 01 .. 00 99  ->  channel 1..99    MEM
//	"P1"          ->  wire 01 00           ->  channel 100      SCAN
//	"P2"          ->  wire 01 01           ->  channel 101      SCAN
//
// P1 and P2 are what the manual prints, and codeplug.DisplaySlot's identity
// fallback passes them through unchanged, so no per-model display table is
// needed for them.
//
// STRICT ABOUT SPELLING. "1" and "0001" are refused, not normalised: a slot
// string has ONE canonical form here, and quietly accepting a second
// spelling would make two names for one channel, which is how a codeplug
// ends up with the same slot twice.
func parseSlot(slot string) (civ.ChannelAddress, spec.BankID, bool) {
	switch slot {
	case "P1":
		return civ.ChannelAddress{Channel: 100}, spec.BankScan, true
	case "P2":
		return civ.ChannelAddress{Channel: 101}, spec.BankScan, true
	}
	if len(slot) != 3 {
		return civ.ChannelAddress{}, "", false
	}
	n := 0
	for i := 0; i < 3; i++ {
		c := slot[i]
		if c < '0' || c > '9' {
			return civ.ChannelAddress{}, "", false
		}
		n = n*10 + int(c-'0')
	}
	if n < 1 || n > 99 {
		return civ.ChannelAddress{}, "", false
	}
	return civ.ChannelAddress{Channel: n}, spec.BankMemory, true
}

// allFF reports whether every byte of rec is 0xFF.
//
// EXACT EQUALITY, never a threshold. This is the second of D5 entry 2's two
// unverified empty-channel answers (`ic7300-ff-record`), and a record that
// is all-FF except one byte is a CORRUPT record, not an empty channel: a
// heuristic here would blank a damaged slot in a codeplug and call it
// normal.
func allFF(rec []byte) bool {
	if len(rec) == 0 {
		return false
	}
	for _, b := range rec {
		if b != 0xFF {
			return false
		}
	}
	return true
}

// ReadChannel implements driver.Session: ONE 1A 00 read of one slot.
//
// THE ORDER OF THE BRANCHES IS THE DESIGN (ruling T2, plan decision D20):
//
//  1. transport.ErrRejected — the FA — is an EMPTY channel. Engine.Do
//     CONSUMES the FA and returns this error with no frame, so this branch
//     keys on the ERROR and never on "an FA frame arrived" (ruling T4).
//  2. civ.Profile.MemoryAnswerRecord splits the answer and applies the
//     CONTINUOUS length fingerprint. A wrong length fails the read.
//  3. THE ADDRESS CHECK, before any other use of the answer. The codec's
//     memory-answer matcher is envelope-only by design, so this is the
//     driver's to make — and it must precede the all-FF branch, or an
//     all-FF answer for the WRONG slot would be accepted as "this slot is
//     empty" and would blank a populated channel silently.
//  4. only THEN the all-FF branch.
//  5. only then the record parser.
//
// NOTHING IS CACHED. The write path performs its own read under the
// session's writeMu (D15), so the record it inspects is the record it
// builds against; a per-slot cache would go stale between the two and could
// carry one slot's SELECT nibble onto another.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	want, _, ok := parseSlot(slot)
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ic7300: ReadChannel: %q is not a slot this radio has — its slots are 001..099, P1 and P2", slot)
	}
	cmd, err := s.p.BuildMemoryRead(want)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ic7300: ReadChannel %s: %w", slot, err)
	}

	// retryReads is ONE, stated rather than left to the signature: a read is
	// idempotent and a CI-V read of a memory channel changes nothing.
	frame, err := s.eng.Do(ctx, cmd, civ.CIVReadSpec(s.p.MemoryAnswerMatcher(), 1))
	if errors.Is(err, transport.ErrRejected) {
		// D5 entry 2(a), ASSUMED, lift `ic7300-empty-read`: an unwritten
		// channel answers FA. An EMPTY channel — Data nil — and never an
		// error, because an error here would abort the whole ReadAll over a
		// perfectly ordinary blank memory.
		return codeplug.Channel{Slot: slot}, nil
	}
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ic7300: ReadChannel %s: %w", slot, err)
	}

	got, raw, err := s.p.MemoryAnswerRecord(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ic7300: ReadChannel %s: %w", slot, err)
	}
	if got != want {
		s.noteAnswerMismatch()
		return codeplug.Channel{}, &AnswerMismatchError{Requested: want.String(), Answered: got.String()}
	}
	if allFF(raw) {
		return codeplug.Channel{Slot: slot}, nil
	}

	rec, err := s.p.ParseMemoryAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ic7300: ReadChannel %s: %w", slot, err)
	}
	data, err := s.channelData(rec)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ic7300: ReadChannel %s: %w", slot, err)
	}
	return codeplug.Channel{Slot: slot, Data: data}, nil
}

// channelData maps one decoded record onto the neutral channel model.
//
// EVERY FIELD IS WRITTEN DOWN, including the ten this record does not carry:
// leaving them to the zero value would give them codeplug.Absent, which is
// "nobody has set one at all" and which every Valid() refuses. Unavailable
// is the POSITIVE statement this radio has no such field, and it is what
// keeps a channel read here comparable with one loaded from a file.
func (s *Session) channelData(rec civ.MemoryRecord) (*codeplug.ChannelData, error) {
	freq, err := numeric(rec.RXFreqHz, civ.FieldRXFrequency)
	if err != nil {
		return nil, err
	}
	txFreq, err := numeric(rec.TXFreqHz, civ.FieldTXFrequency)
	if err != nil {
		return nil, err
	}
	mode, err := text(rec.Mode, civ.FieldMode)
	if err != nil {
		return nil, err
	}
	filter, err := text(rec.Filter, civ.FieldFilter)
	if err != nil {
		return nil, err
	}
	dataMode, err := text(rec.DataMode, civ.FieldDataMode)
	if err != nil {
		return nil, err
	}
	toneMode, err := text(rec.ToneMode, civ.FieldToneMode)
	if err != nil {
		return nil, err
	}
	name, err := text(rec.Name, civ.FieldName)
	if err != nil {
		return nil, err
	}
	toneTx, err := s.toneField(rec.ToneTXDeciHz, civ.FieldToneTX)
	if err != nil {
		return nil, err
	}
	toneRx, err := s.toneField(rec.ToneRXDeciHz, civ.FieldToneRX)
	if err != nil {
		return nil, err
	}

	return &codeplug.ChannelData{
		// ④–⑧.
		FreqHz: freq,
		// ⑨.
		Mode: mode,

		// The 1A 00 record has NO clarifier field (matrix §1 rows 6, 7), and
		// these three members are plain scalars with no state to say so —
		// so the honest reading is their zero value, which is also what a
		// write path reads as "the caller set nothing here".
		ClarHz: 0,
		RxClar: false,
		TxClar: false,
		// ctcss_state is displaced by tone_mode on Icom models (spec D4);
		// shift has no field on this record at all (matrix §1 row 14).
		CTCSS: "",
		Shift: "",
		// The Yaesu tone INDEX field. This record stores a tone FREQUENCY
		// and indexes no chart, so the value lives on ToneTx/ToneRx below.
		CTCSSTone: codeplug.ToneField{State: codeplug.Unavailable},

		// ⑱–㉗, trimmed of its 0x20 pad by the codec.
		Tag: name,
		// NO DISPLAY FLAG EXISTS IN THIS RECORD, and the capabilities grade
		// the field the ZERO FieldSupport to say the same thing (D5, R13).
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
		// ③'s SELECT nibble is group MEMBERSHIP, the inverse of a skip flag,
		// and the tier forbids mapping it here (D4). The value is not lost —
		// it round-trips inside the civ record on civ.FieldSelect, and the
		// write path carries it through unchanged.
		ScanSkip: codeplug.BoolField{State: codeplug.Unavailable},

		// ❹–⑧, a DISTINCT field, so a split channel round trips.
		TxFreqHz: codeplug.FreqField{State: codeplug.Known, Value: txFreq},
		// MANUAL-EVIDENCED absences (matrix §1b): spec D6 puts per-channel
		// duplex and offset out of scope for this pair.
		Duplex:   codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz: codeplug.FreqField{State: codeplug.Unavailable},
		// ⑪'s LOW nibble.
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: toneMode},
		// ⑫–⑭ and ⑮–⑰.
		ToneTx: toneTx,
		ToneRx: toneRx,
		// The record carries no DTCS field at all (matrix §1b).
		DTCSCode:     codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},
		// ⑩.
		Filter: codeplug.StringField{State: codeplug.Known, Value: filter},
		// ⑪'s HIGH nibble.
		DataMode: codeplug.BoolField{State: codeplug.Known, Value: dataMode == "ON"},
	}, nil
}

// toneField maps one civ-layer tone number onto the neutral tone field.
//
// THE DOMAIN IS A CAPABILITY, AND THE CIV LAYER IS WIDER THAN IT (ruling
// T1). The record can hold 00 00 00 — a zero — and the declared domain
// starts at 1 deciHz, because 0 Hz is not a tone. So a value INSIDE the
// declared domain becomes Known, and one OUTSIDE it, ZERO INCLUDED, becomes
// Unknown: a read must never construct a Known value
// codeplug.ToneField.Valid would then refuse, which is what would happen if
// a zero were handed up as a tone.
//
// It asks s.caps.AdmitsTone rather than reading the range itself: that is
// the ONE predicate that knows about both a list and a range, and asking it
// is what keeps this driver's reads and every validator above it judging the
// same domain. Registered as `ic7300-zero-tone-means-unset`, lift
// `ic7300-zero-tone-read`.
func (s *Session) toneField(v civ.Optional[uint64], id civ.FieldID) (codeplug.ToneField, error) {
	raw, err := numeric(v, id)
	if err != nil {
		return codeplug.ToneField{}, err
	}
	t := spec.Tone(raw)
	if !s.caps.AdmitsTone(t) {
		return codeplug.ToneField{State: codeplug.Unknown}, nil
	}
	return codeplug.ToneField{State: codeplug.Known, Value: t}, nil
}

// numeric and text unwrap a field the layout MAPS, refusing rather than
// substituting a zero.
//
// UNREACHABLE FOR A RECORD civ DECODED — decodeRecord fills every mapped
// span, and validateRecordFields refuses a record missing one — and asserted
// rather than assumed, because the cost of being wrong is a 0 Hz channel or
// an empty mode name handed to the caller as fact.
func numeric(v civ.Optional[uint64], id civ.FieldID) (uint64, error) {
	got, ok := v.Get()
	if !ok {
		return 0, fmt.Errorf("the decoded record carries no %s, which this profile's layout maps — refusing to substitute a zero", id)
	}
	return got, nil
}

func text(v civ.Optional[string], id civ.FieldID) (string, error) {
	got, ok := v.Get()
	if !ok {
		return "", fmt.Errorf("the decoded record carries no %s, which this profile's layout maps — refusing to substitute an empty value", id)
	}
	return got, nil
}
