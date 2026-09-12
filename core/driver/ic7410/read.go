// SPDX-License-Identifier: GPL-3.0-or-later

package ic7410

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7410 "github.com/gm5dna/open-rig-programmer/core/civ/ic7410"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// The two scan edges' wire channel numbers. Matrix §1b "The banks": P1 and
// P2 are NOT a separate bank in the wire protocol — they are two more
// values of the same two-byte selector (BCD "01 00"/"01 01"), and
// core/civ/ic7410's profile declares exactly that range.
const (
	scanEdgeP1Channel = 100
	scanEdgeP2Channel = 101
	lastMemoryChannel = 99
)

// slotToAddress maps a canonical wire-form slot to the channel address the
// codec addresses it by, and to the bank it belongs to. "0001".."0099" are
// the memories (caps.go's memSlots, spec.NumberedSlots(1, 99, "%04d"));
// "P1" and "P2" are the scan edges.
func slotToAddress(slot string) (civ.ChannelAddress, spec.BankID, error) {
	switch slot {
	case "P1":
		return civ.ChannelAddress{Channel: scanEdgeP1Channel}, spec.BankScan, nil
	case "P2":
		return civ.ChannelAddress{Channel: scanEdgeP2Channel}, spec.BankScan, nil
	}
	if len(slot) != 4 {
		return civ.ChannelAddress{}, "", fmt.Errorf("ic7410: %q is not a slot on this radio: a memory is four digits (\"0001\"..\"0099\") and a scan edge is \"P1\" or \"P2\"", slot)
	}
	n, err := strconv.Atoi(slot)
	if err != nil {
		return civ.ChannelAddress{}, "", fmt.Errorf("ic7410: %q is not a slot on this radio: %w", slot, err)
	}
	if n < 1 || n > lastMemoryChannel {
		return civ.ChannelAddress{}, "", fmt.Errorf("ic7410: %q is outside this radio's memory range \"0001\"..\"0099\"", slot)
	}
	return civ.ChannelAddress{Channel: n}, spec.BankMemory, nil
}

// addressToSlot is slotToAddress's inverse.
func addressToSlot(a civ.ChannelAddress) (string, error) {
	if a.Group != 0 {
		return "", fmt.Errorf("ic7410: %s carries a group index; this radio's channel selector is a flat two-byte number", a)
	}
	switch a.Channel {
	case scanEdgeP1Channel:
		return "P1", nil
	case scanEdgeP2Channel:
		return "P2", nil
	}
	if a.Channel < 1 || a.Channel > lastMemoryChannel {
		return "", fmt.Errorf("ic7410: channel %d is outside this radio's addressable space (1..99, plus 100 and 101 for the scan edges)", a.Channel)
	}
	return fmt.Sprintf("%04d", a.Channel), nil
}

// recordIsAbsent reports whether raw is an all-0xFF record. ASSUMED (matrix
// §3.8): what an unwritten channel's read actually carries is what
// register entries ic7410-empty-read-answers-fa and ic7410-ff-record-meaning
// would settle; no IC-7410 has ever been read.
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
// WriteChannel's E6 preservation read both go through it.
func (s *Session) readRaw(ctx context.Context, a civ.ChannelAddress) (civ.MemoryRecord, []byte, bool, error) {
	p := civic7410.Profile()
	cmd, err := p.BuildMemoryRead(a)
	if err != nil {
		return civ.MemoryRecord{}, nil, false, fmt.Errorf("ic7410: building the 1A 00 read for %s: %w", a, err)
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
		return civ.MemoryRecord{}, nil, false, &AnswerMismatchError{Model: "ic7410", Requested: a, Answered: got}
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
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	a, _, err := slotToAddress(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ic7410: ReadChannel: %w", err)
	}
	rec, _, empty, err := s.readRaw(ctx, a)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ic7410: ReadChannel %s: %w", slot, err)
	}
	if empty {
		return codeplug.Channel{Slot: slot}, nil
	}

	freq, ok := rec.RXFreqHz.Get()
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ic7410: ReadChannel %s: the record carries no frequency", slot)
	}
	if err := domainRefusal(codeplug.ChannelData{FreqHz: freq}, s.caps); err != nil {
		return codeplug.Channel{}, err
	}
	mode, ok := rec.Mode.Get()
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ic7410: ReadChannel %s: the record carries no mode", slot)
	}
	name, _ := rec.Name.Get()

	data := &codeplug.ChannelData{
		FreqHz: freq,
		Mode:   mode,
		Tag:    name,

		Filter:   optionalString(rec.Filter),
		ToneMode: optionalString(rec.ToneMode),
		ToneTx:   s.toneField(rec.ToneTXDeciHz),
		ToneRx:   s.toneField(rec.ToneRXDeciHz),
		DataMode: optionalBool(rec.DataMode),
		TxFreqHz: optionalFreq(rec.TXFreqHz),

		// UNAVAILABLE: the 40-byte record has no such field.
		TagDisplay:   codeplug.BoolField{State: codeplug.Unavailable},
		CTCSSTone:    codeplug.ToneField{State: codeplug.Unavailable},
		Duplex:       codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz:     codeplug.FreqField{State: codeplug.Unavailable},
		DTCSCode:     codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},

		// UNAVAILABLE: byte 0 (Select-memory + Split) has no neutral
		// scan_skip home on this model (matrix §2 MEM row 9), and the D8
		// receiver fields have no home on any transceiver in this wave.
		ScanSkip:            codeplug.BoolField{State: codeplug.Unavailable},
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
		Preamp:              codeplug.StringField{State: codeplug.Unavailable},
		Antenna:             codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},
	}

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

// optionalBool maps a civ tri-state "OFF"/"ON" string to codeplug's
// BoolField — the data_mode span's own vocabulary (matrix §1b, offset 8).
func optionalBool(o civ.Optional[string]) codeplug.BoolField {
	v, ok := o.Get()
	if !ok {
		return codeplug.BoolField{State: codeplug.Unavailable}
	}
	return codeplug.BoolField{State: codeplug.Known, Value: v == "ON"}
}

// optionalFreq maps a civ tri-state numeric field to codeplug's FreqField —
// the TX-duplicate block's frequency span.
func optionalFreq(o civ.Optional[uint64]) codeplug.FreqField {
	v, ok := o.Get()
	if !ok {
		return codeplug.FreqField{State: codeplug.Unavailable}
	}
	return codeplug.FreqField{State: codeplug.Known, Value: v}
}

// toneField maps a civ-layer tone number to a codeplug.ToneField under
// tier ruling T1(3): a number outside this session's declared domain, 0
// included, comes back Unknown rather than a Known value
// codeplug.Validate would then refuse.
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
