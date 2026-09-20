// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7200 "github.com/gm5dna/open-rig-programmer/core/civ/ic7200"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// The two scan edges' wire channel numbers (matrix §1 row 4, §3.15(d)):
// P1 and P2 are two more values of the same two-byte selector, not a
// separate bank in the wire protocol.
const (
	scanEdgeP1Channel = 200
	scanEdgeP2Channel = 201
	lastMemoryChannel = 199
)

// slotToAddress maps a canonical wire-form slot to the channel address the
// codec addresses it by, and to the bank it belongs to.
func slotToAddress(slot string) (civ.ChannelAddress, spec.BankID, error) {
	switch slot {
	case "P1":
		return civ.ChannelAddress{Channel: scanEdgeP1Channel}, spec.BankScan, nil
	case "P2":
		return civ.ChannelAddress{Channel: scanEdgeP2Channel}, spec.BankScan, nil
	}
	if len(slot) != 3 {
		return civ.ChannelAddress{}, "", fmt.Errorf("ic7200: %q is not a slot on this radio: a memory is three digits (\"001\"..\"199\") and a scan edge is \"P1\" or \"P2\"", slot)
	}
	n, err := strconv.Atoi(slot)
	if err != nil {
		return civ.ChannelAddress{}, "", fmt.Errorf("ic7200: %q is not a slot on this radio: %w", slot, err)
	}
	if n < 1 || n > lastMemoryChannel {
		return civ.ChannelAddress{}, "", fmt.Errorf("ic7200: %q is outside this radio's memory range \"001\"..\"199\"", slot)
	}
	return civ.ChannelAddress{Channel: n}, spec.BankMemory, nil
}

// recordIsAbsent reports whether raw is an all-0xFF record — the same
// ASSUMED reading (matrix §3.8(a), register entry) every sibling Icom
// package in this tier gives an unwritten channel.
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
// with its raw bytes. The one read primitive: ReadChannel and
// WriteChannel's E6-style preservation read both go through it.
func (s *Session) readRaw(ctx context.Context, a civ.ChannelAddress) (civ.MemoryRecord, []byte, bool, error) {
	p := civic7200.Profile()
	cmd, err := p.BuildMemoryRead(a)
	if err != nil {
		return civ.MemoryRecord{}, nil, false, fmt.Errorf("ic7200: building the 1A 00 read for %s: %w", a, err)
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
		return civ.MemoryRecord{}, nil, false, err
	}
	if got != a { // T2, before any use of raw
		s.answerMismatches.Add(1)
		return civ.MemoryRecord{}, nil, false, &AnswerMismatchError{Model: "ic7200", Requested: a, Answered: got}
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

// ReadChannel implements driver.Session: ONE 1A 00 read, mapped into one
// codeplug.Channel.
//
// AN EMPTY SLOT COMES BACK AS AN EMPTY CHANNEL (Data nil), never an
// error. Every field the 17-byte record does not express comes back
// Unavailable, never a guessed value.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	a, _, err := slotToAddress(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ic7200: ReadChannel: %w", err)
	}
	rec, _, empty, err := s.readRaw(ctx, a)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ic7200: ReadChannel %s: %w", slot, err)
	}
	if empty {
		return codeplug.Channel{Slot: slot}, nil
	}

	freq, ok := rec.RXFreqHz.Get()
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ic7200: ReadChannel %s: the record carries no frequency", slot)
	}
	if freq > MaxRadioFreqHz {
		return codeplug.Channel{}, &OutOfDomainError{driver.OutOfDomainError{Field: spec.FieldFrequency, Value: freq, Max: MaxRadioFreqHz}}
	}
	mode, ok := rec.Mode.Get()
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ic7200: ReadChannel %s: the record carries no mode", slot)
	}

	data := &codeplug.ChannelData{
		FreqHz: freq,
		Mode:   mode,

		Filter:   optionalString(rec.Filter),
		DataMode: optionalDataMode(rec.DataMode),
		TxFreqHz: txFreqField(rec.TXFreqHz),

		// UNAVAILABLE: this record has no such field (NoTag; matrix §1
		// row 6/§1b/§3.9).
		TagDisplay:   codeplug.BoolField{State: codeplug.Unavailable},
		CTCSSTone:    codeplug.ToneField{State: codeplug.Unavailable},
		Duplex:       codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz:     codeplug.FreqField{State: codeplug.Unavailable},
		ToneMode:     codeplug.StringField{State: codeplug.Unavailable},
		ToneTx:       codeplug.ToneField{State: codeplug.Unavailable},
		ToneRx:       codeplug.ToneField{State: codeplug.Unavailable},
		DTCSCode:     codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},
		ScanSkip:     codeplug.BoolField{State: codeplug.Unavailable},

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
	// Tag is left "" (its zero value): NoTag means this radio has no
	// name route at all, and codeplug.ChannelData.Tag is a plain string
	// with no tri-state to mark Unavailable.

	return codeplug.Channel{Slot: slot, Data: data}, nil
}

// optionalString maps a civ tri-state string to codeplug's: present
// becomes Known, absent becomes Unavailable.
func optionalString(o civ.Optional[string]) codeplug.StringField {
	v, ok := o.Get()
	if !ok {
		return codeplug.StringField{State: codeplug.Unavailable}
	}
	return codeplug.StringField{State: codeplug.Known, Value: v}
}

// optionalDataMode maps the civ-layer "ON"/"OFF" data-mode string to a
// codeplug.BoolField.
func optionalDataMode(o civ.Optional[string]) codeplug.BoolField {
	v, ok := o.Get()
	if !ok {
		return codeplug.BoolField{State: codeplug.Unavailable}
	}
	return codeplug.BoolField{State: codeplug.Known, Value: v == "ON"}
}

// txFreqField maps the civ-layer TX-duplicate-block frequency to a
// codeplug.FreqField, with the same defence-in-depth domain check
// ReadChannel applies to the primary frequency.
func txFreqField(o civ.Optional[uint64]) codeplug.FreqField {
	v, ok := o.Get()
	if !ok {
		return codeplug.FreqField{State: codeplug.Unavailable}
	}
	if v > MaxRadioFreqHz {
		return codeplug.FreqField{State: codeplug.Unknown}
	}
	return codeplug.FreqField{State: codeplug.Known, Value: v}
}
