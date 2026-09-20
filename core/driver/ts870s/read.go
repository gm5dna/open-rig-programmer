// SPDX-License-Identifier: GPL-3.0-or-later

package ts870s

import (
	"context"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// AnswerMismatchError is driver.AnswerMismatchError specialised to this
// row's own slot identifiers (plain two-digit strings — matrix §1.4).
type AnswerMismatchError = driver.AnswerMismatchError[string]

// parseSlotID resolves id (e.g. "07") into its channel number.
//
// IT IS SYNTAX ONLY, the ts480 shape (read.go): bank membership — whether
// the number is one this row actually publishes — is a separate rung
// below, so a user never meets two different refusals for one mistake.
// Two digits, not three: this row's own P3 is a flat "00"-"99" (matrix
// §1.4), with no bank-prefix digit and no section-channel "L"/"U" half —
// there is no scan class on this document at all (record870.go's own P1
// refusal).
func parseSlotID(id string) (int, error) {
	if len(id) != 2 {
		return 0, fmt.Errorf("slot %q: a TS-870S slot identifier is exactly two digits (matrix §1.4)", id)
	}
	for _, b := range []byte(id) {
		if b < '0' || b > '9' {
			return 0, fmt.Errorf("slot %q: the channel number is two decimal digits", id)
		}
	}
	return int(id[0]-'0')*10 + int(id[1]-'0'), nil
}

// UnknownSlotError reports that a slot id names no channel this row
// publishes. Fields are driver.UnknownSlotError's shared shape
// (core/driver/errors.go); Model is left unset — this row's message
// carries no model name at all, unlike the other six ts* packages.
type UnknownSlotError struct {
	driver.UnknownSlotError
}

func (e *UnknownSlotError) Error() string {
	return fmt.Sprintf("ts870s: slot %q: %s", e.Slot, e.Reason)
}

// ErrUnknownSlot is the sentinel a caller compares against.
var ErrUnknownSlot = driver.ErrUnknownSlot

func (e *UnknownSlotError) Unwrap() error { return ErrUnknownSlot }

// bankFor returns the MEM bank of this session's own published
// capabilities that holds id, and whether any does — read from the
// session's EFFECTIVE set, the ts480/ts590 shape, so a slot is admitted
// only if the very capabilities this session handed its caller say it
// exists.
func (s *Session) bankFor(id string) (spec.Bank, bool) {
	bankID, ok := s.caps.BankOf(id)
	if !ok {
		return spec.Bank{}, false
	}
	return s.caps.Bank(bankID)
}

// ReadChannel implements driver.Session: ONE MR frame per slot, P1
// always '0' (this grid has no scan/extension class, record870.go's own
// P1 refusal).
//
// FAILURE MODES, the ts480/ts590 reading applied to this row's own
// codec:
//
//   - An UNKNOWN SLOT is refused before any frame is built.
//   - A "?;" is a DEFINITIVE REJECTION (kw.RejectionError), never
//     "absent"; a timeout is the typed kw.TimeoutError. Both fail the
//     session read WHOLE (core/clone's ReadAll propagates the first
//     error, so a partial read the user could not tell from a complete
//     one never happens).
//   - AN EMPTY CHANNEL IS NOT A FAILURE: "the Answer command sends '0'
//     for all parameters except the memory channel number"
//     (ts870s:9101-9104, matrix §1.15/§2.1) decodes as Record870.Empty
//     (record870.go, the Lift K follow-up), reported as
//     codeplug.Channel{Slot: id} with a nil Data.
func (s *Session) ReadChannel(ctx context.Context, id string) (codeplug.Channel, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	channel, err := parseSlotID(id)
	if err != nil {
		return codeplug.Channel{}, &UnknownSlotError{driver.UnknownSlotError{Slot: id, Reason: err.Error()}}
	}
	if _, ok := s.bankFor(id); !ok {
		return codeplug.Channel{}, &UnknownSlotError{driver.UnknownSlotError{Slot: id, Reason: "this row publishes MEM 00-99 only"}}
	}

	cmd, err := s.layout.BuildMRRead(channel)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts870s: ReadChannel %s: %w", id, err)
	}
	frame, err := s.eng.Do(ctx, cmd, s.mrSpec(channel))
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts870s: ReadChannel %s: %w", id, wireFailure(s.layout.Book(), "MR", err))
	}

	rec, err := s.layout.ParseMRAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts870s: ReadChannel %s: %w", id, err)
	}
	if rec.Channel != channel {
		return codeplug.Channel{}, &AnswerMismatchError{Model: "ts870s", Requested: id, Answered: fmt.Sprintf("%02d", rec.Channel)}
	}
	if rec.Empty {
		return codeplug.Channel{Slot: id}, nil
	}

	data, err := s.channelData(rec)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ts870s: ReadChannel %s: %w", id, err)
	}
	return codeplug.Channel{Slot: id, Data: data}, nil
}

// mrSpec is the transport spec for one MR read of channel: this row's own
// channel-correlated matcher (record870.go's MRAnswerMatcher, the Lift K
// follow-up), and one retry — an MR read is idempotent and a single
// swallowed reply should not fail a whole-radio read.
func (s *Session) mrSpec(channel int) transport.CommandSpec {
	return transport.CommandSpec{
		Class:      transport.ClassRead,
		Match:      s.layout.MRAnswerMatcher(channel),
		RetryReads: 1,
	}
}

// channelData maps one parsed 22-byte record onto the neutral channel
// model.
//
// TxFreqHz IS ALWAYS UNAVAILABLE — see the package doc comment: this
// session never sends the P1=1 read, so it has nothing to report, and
// Unavailable is codeplug's own spelling of "not read", never "absent"
// (ts590's own TxFreqHz precedent, its own A9).
func (s *Session) channelData(rec kw.Record870) (*codeplug.ChannelData, error) {
	name, ok := s.layout.ModeNames()[rec.Mode]
	if !ok {
		// Unreachable: ParseMRAnswer already refused any mode nibble not
		// in this layout's own legend.
		return nil, fmt.Errorf("ts870s: channel's mode %v is not one this layout names", rec.Mode)
	}

	toneModeValue := "OFF"
	toneTx := codeplug.ToneField{State: codeplug.Unavailable}
	if rec.ToneMode == kw.ToneModeTone {
		toneModeValue = "TONE"
		tone, err := s.tone(rec.ToneIndex)
		if err != nil {
			return nil, err
		}
		toneTx = tone
	}

	return &codeplug.ChannelData{
		FreqHz: rec.FreqHz,
		Mode:   name,
		ScanSkip: codeplug.BoolField{
			State: codeplug.Known,
			Value: rec.Lockout == '1',
		},
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: toneModeValue},
		ToneTx:   toneTx,
		// TxFreqHz: see the package doc comment — this session never
		// sends the P1=1 read.
		TxFreqHz: codeplug.FreqField{State: codeplug.Unavailable},

		// UNAVAILABLE, EVERY ONE EXPLICIT (the ts480/ts590/ic7200 shape):
		// wiring's own read invariant
		// (TestOpenFakeSessionFor_EveryRegisteredModel_ReadsEveryDefaultSlot)
		// requires every codeplug FieldState to be Known, Unknown or
		// Unavailable before Save can choose a schema — the Go zero value
		// of each of these types is Absent, which satisfies none of the
		// three, so every field this 22-byte record has no byte for must
		// be stated here rather than left at its zero value. TagDisplay
		// is Unavailable because NoTag (matrix §1.6): there is no name
		// route to display in place of the frequency at all.
		TagDisplay:          codeplug.BoolField{State: codeplug.Unavailable},
		CTCSSTone:           codeplug.ToneField{State: codeplug.Unavailable},
		Duplex:              codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz:            codeplug.FreqField{State: codeplug.Unavailable},
		ToneRx:              codeplug.ToneField{State: codeplug.Unavailable},
		DTCSCode:            codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity:        codeplug.StringField{State: codeplug.Unavailable},
		Filter:              codeplug.StringField{State: codeplug.Unavailable},
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
	}, nil
}

// tone maps a 1-39 wire index onto this row's own 39-entry chart
// (matrix §1.9). UNLIKE ts590's family chart, THE WIRE VALUE IS ONE-BASED
// and the slice is zero-based: index N is caps.CTCSSTones[N-1], not
// caps.CTCSSTones[N] — record870.go's own rec870MinToneIndex/
// rec870MaxToneIndex (1-39) are this row's, not the family's 0-42.
func (s *Session) tone(index int) (codeplug.ToneField, error) {
	tones := s.caps.CTCSSTones
	if index < 1 || index > len(tones) {
		return codeplug.ToneField{}, fmt.Errorf("ts870s: tone index %d is outside this row's own %02d-entry chart (matrix §1.9)", index, len(tones))
	}
	value := tones[index-1]
	if !s.caps.AdmitsTone(value) {
		return codeplug.ToneField{}, fmt.Errorf("ts870s: tone index %d is %v, which this row's own capability table does not admit", index, value)
	}
	return codeplug.ToneField{State: codeplug.Known, Value: value}, nil
}
