// SPDX-License-Identifier: GPL-3.0-or-later

package ts570

import (
	"context"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// ErrUnknownSlot is the sentinel a caller compares against (via errors.Is)
// when a slot identifier is not one this row publishes.
var ErrUnknownSlot = driver.ErrUnknownSlot

// UnknownSlotError reports a slot identifier that is not in this row's
// bank — core/driver/ts480's own shape. Fields are driver.UnknownSlotError's
// shared shape (core/driver/errors.go); this package keeps its own Error()
// because the "ts570:" prefix is a package literal, not a field.
type UnknownSlotError struct {
	driver.UnknownSlotError
}

func (e *UnknownSlotError) Error() string {
	return fmt.Sprintf("ts570: slot %q is not published by the %s: %s", e.Slot, e.Model, e.Reason)
}

func (e *UnknownSlotError) Unwrap() error { return ErrUnknownSlot }

// ErrAnswerMismatch is the sentinel a caller compares against (via
// errors.Is) when a slot-addressed answer does not name what the read
// asked for.
var ErrAnswerMismatch = driver.ErrAnswerMismatch

// AnswerMismatchError reports the requested and the answered channel.
type AnswerMismatchError = driver.AnswerMismatchError[string]

// parseSlotID reads a canonical slot identifier as a channel number: two
// digits, no half suffix — this row has no section-channel class at all
// (core/kw/ts570's one flat SlotMemory range).
func parseSlotID(id string) (int, error) {
	if len(id) != 2 {
		return 0, fmt.Errorf("slot %q: a slot identifier on this row is exactly two digits — P2Unused carries no hundreds digit, so the channel number is P3's two digits alone", id)
	}
	for _, b := range []byte(id) {
		if b < '0' || b > '9' {
			return 0, fmt.Errorf("slot %q: the channel number is two decimal digits", id)
		}
	}
	return int(id[0]-'0')*10 + int(id[1]-'0'), nil
}

func (s *Session) bankFor(id string) (spec.Bank, bool) {
	bankID, ok := s.caps.BankOf(id)
	if !ok {
		return spec.Bank{}, false
	}
	return s.caps.Bank(bankID)
}

// bankNames renders this session's bank inventory for a refusal.
func (s *Session) bankNames() string {
	var out string
	for i, b := range s.caps.Banks {
		if len(b.Slots) == 0 {
			continue
		}
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%s %s-%s", b.ID, b.Slots[0], b.Slots[len(b.Slots)-1])
	}
	return out
}

// mrSpec is the transport spec for one MR read of slot: the codec's own MR
// answer matcher, and one retry — core/driver/ts480's own reasoning in
// full (kw.Layout.MRAnswerMatcher rather than a prefix-and-length match,
// since every memory answer starts "MR" with the channel number at P2/P3).
func (s *Session) mrSpec(slot kw.Slot) transport.CommandSpec {
	return s.newReadSpec(s.layout.MRAnswerMatcher(slot), 1)
}

// ReadChannel implements driver.Session: ONE MR frame per slot.
//
// AN EMPTY CHANNEL IS NOT A FAILURE: an MR answer in this row's own
// documented vacant shape — P4 through P8 all zero, the channel number
// preserved ("For a vacant channel, the Answer command sends '0' for all
// parameters except the memory channel number.", manual layout lines
// ~5926-5928) — is reported as codeplug.Channel{Slot: id} with nil Data.
// core/kw.Layout.ParseMRAnswer sets Record.Empty for this shape itself
// (core/kw's isEmptyWindow became width-aware in the Lift K follow-up,
// commit e7515d0); this package no longer restates the shape locally.
//
// A "?;" IS A DEFINITIVE REJECTION and a timeout is the typed
// kw.TimeoutError — neither is "absent" (decision 5, applied identically
// to every Kenwood row).
func (s *Session) ReadChannel(ctx context.Context, id string) (codeplug.Channel, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	number, err := parseSlotID(id)
	if err != nil {
		return codeplug.Channel{}, &UnknownSlotError{driver.UnknownSlotError{Slot: id, Model: s.model.name, Reason: err.Error()}}
	}
	if _, ok := s.bankFor(id); !ok {
		return codeplug.Channel{}, &UnknownSlotError{driver.UnknownSlotError{
			Slot: id, Model: s.model.name,
			Reason: fmt.Sprintf("this row publishes %s", s.bankNames()),
		}}
	}
	slot, err := s.layout.NewSlot(number, kw.ScanHalfNone)
	if err != nil {
		return codeplug.Channel{}, &UnknownSlotError{driver.UnknownSlotError{Slot: id, Model: s.model.name, Reason: err.Error()}}
	}

	cmd, err := s.layout.BuildMRRead(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts570: ReadChannel %s: %w", id, err)
	}
	frame, err := s.eng.Do(ctx, cmd, s.mrSpec(slot))
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts570: ReadChannel %s: %w", id, wireFailure(s.layout, "MR", err))
	}

	rec, err := s.layout.ParseMRAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts570: ReadChannel %s: %w: %w", id, driver.ErrRecordDecode, err)
	}
	if got := rec.Slot.Number(); got != number {
		return codeplug.Channel{}, &AnswerMismatchError{Model: "ts570", Requested: id, Answered: slotID(got)}
	}
	if rec.Empty {
		return codeplug.Channel{Slot: id}, nil
	}

	data, err := s.channelData(rec)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts570: ReadChannel %s: %w", id, err)
	}
	return codeplug.Channel{Slot: id, Data: data}, nil
}

// channelData maps one parsed 28-byte record onto the neutral channel
// model. There is no Byte3940/FM-Narrow synthesis on this row (no tail:
// RecordLen 28), so RecordModeName reduces to the plain legend name.
func (s *Session) channelData(rec kw.Record) (*codeplug.ChannelData, error) {
	mode, ok := s.layout.ModeName(rec.Mode)
	if !ok {
		// Unreachable after core/kw's own ParseMode, which refuses a
		// nibble outside this row's legend.
		return nil, fmt.Errorf("unmapped mode nibble %v", rec.Mode)
	}
	toneModeName, ok := toneModeNames[rec.ToneMode]
	if !ok {
		// Unreachable after Layout.ValidToneMode, whose ToneModesTwo axis
		// admits only OFF and TONE (rendered here "ON").
		return nil, fmt.Errorf("unmapped tone mode %v", rec.ToneMode)
	}

	// P8 is this row's ONE tone index, serving both directions (matrix
	// §2). A radio answering an index outside the printed 01-39 domain
	// is refused rather than reported Unavailable, on core/driver/ts590's
	// own tone() precedent: core/kw has already bounded the wire byte to
	// its OWN package-wide 00-42 sanity check (core/kw/tone.go), which is
	// wider than this row's own printed chart, so an index this chart has
	// no entry for is a genuine disagreement between the radio and its
	// own book rather than something this codec should have caught first.
	toneTx := codeplug.ToneField{State: codeplug.Unavailable}
	toneRx := codeplug.ToneField{State: codeplug.Unavailable}
	if rec.ToneIndex != 0 {
		t, err := tone(rec.ToneIndex)
		if err != nil {
			return nil, fmt.Errorf("P8: %w", err)
		}
		if !s.caps.AdmitsTone(t) {
			return nil, fmt.Errorf("P8 index %d is %v, which is not a tone this row's capability table admits", rec.ToneIndex, t)
		}
		toneTx = codeplug.ToneField{State: codeplug.Known, Value: t}
		toneRx = codeplug.ToneField{State: codeplug.Known, Value: t}
	}

	return &codeplug.ChannelData{
		FreqHz: rec.FreqHz,
		Mode:   mode,

		// No per-channel clarifier position in this row's 28-byte record
		// (matrix §2); this radio's RIT/XIT are radio-level settings no
		// memory channel stores.
		ClarHz: 0,
		RxClar: false,
		TxClar: false,
		// The Yaesu half of the vocabulary pair, displaced by tone_mode
		// below (decision 6).
		CTCSS:     "",
		Shift:     "",
		CTCSSTone: codeplug.ToneField{State: codeplug.Unavailable},

		// NoTag: no name byte exists on this row at all.
		Tag:        "",
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
		// P6 at position 19: the channel LOCKOUT (matrix §1.2).
		ScanSkip: codeplug.BoolField{State: codeplug.Known, Value: rec.Byte19 == '1'},

		TxFreqHz: codeplug.FreqField{State: codeplug.Unavailable},
		Duplex:   codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz: codeplug.FreqField{State: codeplug.Unavailable},
		// P7 at position 20: OFF/ON only.
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: toneModeName},
		// P8 at positions 21-22, shared by both directions.
		ToneTx: toneTx,
		ToneRx: toneRx,

		DTCSCode:     codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},
		Filter:       codeplug.StringField{State: codeplug.Unavailable},
		// Byte 19 is the lockout, published above; no data-mode position
		// exists anywhere in this row's record.
		DataMode: codeplug.BoolField{State: codeplug.Unavailable},

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
	}, nil
}
