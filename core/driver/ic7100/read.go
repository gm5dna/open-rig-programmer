// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

var ErrAnswerMismatch = errors.New("ic7100: answer names a different slot than requested")

type AnswerMismatchError struct {
	Requested civ.ChannelAddress
	Answered  civ.ChannelAddress
}

func (e *AnswerMismatchError) Error() string {
	return fmt.Sprintf("ic7100: requested %v but answer names %v", e.Requested, e.Answered)
}
func (e *AnswerMismatchError) Unwrap() error { return ErrAnswerMismatch }

func parseSlot(slot string) (civ.ChannelAddress, spec.BankID, error) {
	if len(slot) != 5 || slot[0] < 'A' || slot[0] > 'E' || slot[1] != '-' {
		return civ.ChannelAddress{}, "", fmt.Errorf("ic7100: slot %q is outside dense MEM space A-001..E-099", slot)
	}
	channel := 0
	for i := 2; i < 5; i++ {
		if slot[i] < '0' || slot[i] > '9' {
			return civ.ChannelAddress{}, "", fmt.Errorf("ic7100: slot %q is outside dense MEM space A-001..E-099", slot)
		}
		channel = channel*10 + int(slot[i]-'0')
	}
	if channel < 1 || channel > 99 {
		// ic7100-special-bank-byte: TestReadChannelRefusesSpecialsBeforeTraffic
		// pins refusal of 0100–0109 until the named bank-byte lift exists.
		return civ.ChannelAddress{}, "", fmt.Errorf("ic7100: slot %q is outside dense MEM space A-001..E-099", slot)
	}
	return civ.ChannelAddress{Group: int(slot[0]-'A') + 1, Channel: channel}, spec.BankMemory, nil
}

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

func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	channel, _, _, err := s.readChannelRaw(ctx, slot)
	return channel, err
}

func (s *Session) readChannelRaw(ctx context.Context, slot string) (codeplug.Channel, []byte, civ.MemoryRecord, error) {
	addr, _, err := parseSlot(slot)
	if err != nil {
		return codeplug.Channel{}, nil, civ.MemoryRecord{}, err
	}
	cmd, err := s.profile.BuildMemoryRead(addr)
	if err != nil {
		return codeplug.Channel{}, nil, civ.MemoryRecord{}, fmt.Errorf("ic7100: ReadChannel %s: %w", slot, err)
	}
	answer, err := s.eng.Do(ctx, cmd, civ.CIVReadSpec(s.profile.MemoryAnswerMatcher(), retryReads))
	if errors.Is(err, transport.ErrRejected) {
		// ASSUMED: ic7100-empty-channel-fa; pinned by
		// TestReadChannelTreatsFAAndAllFFAsSeparateEmptyForms.
		return codeplug.Channel{Slot: slot}, nil, civ.MemoryRecord{}, nil
	}
	if err != nil {
		return codeplug.Channel{}, nil, civ.MemoryRecord{}, fmt.Errorf("ic7100: ReadChannel %s: %w", slot, err)
	}
	got, record, err := s.profile.MemoryAnswerRecord(answer)
	if err != nil {
		return codeplug.Channel{}, nil, civ.MemoryRecord{}, err
	}
	if got != addr {
		s.noteMismatch()
		return codeplug.Channel{}, nil, civ.MemoryRecord{}, &AnswerMismatchError{Requested: addr, Answered: got}
	}
	// ASSUMED: ic7100-all-ff-record. This raw check must precede the typed
	// parser, whose BCD/enums correctly reject FF as data.
	if allFF(record) {
		return codeplug.Channel{Slot: slot}, nil, civ.MemoryRecord{}, nil
	}
	rec, err := s.profile.ParseMemoryAnswer(answer)
	if err != nil {
		return codeplug.Channel{}, nil, civ.MemoryRecord{}, err
	}
	data := s.channelData(rec)
	return codeplug.Channel{Slot: slot, Data: &data}, record, rec, nil
}

func (s *Session) channelData(rec civ.MemoryRecord) codeplug.ChannelData {
	return codeplug.ChannelData{
		FreqHz: numberOf(rec.RXFreqHz), Mode: stringOf(rec.Mode), Tag: stringOf(rec.Name),
		CTCSSTone:  codeplug.ToneField{State: codeplug.Unavailable},
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
		ScanSkip:   codeplug.BoolField{State: codeplug.Unavailable},
		TxFreqHz:   freqFieldOf(rec.TXFreqHz),
		Duplex:     vocabField(rec.Duplex, duplexValues(s.caps)),
		OffsetHz:   freqFieldOf(rec.OffsetHz),
		ToneMode:   vocabField(rec.ToneMode, toneModeValues(s.caps)),
		ToneTx:     s.toneField(rec.ToneTXDeciHz), ToneRx: s.toneField(rec.ToneRXDeciHz),
		DTCSCode:            s.dtcsField(rec.DTCSCode),
		DTCSPolarity:        vocabField(rec.DTCSPolarity, s.caps.DTCSPolarities),
		Filter:              vocabField(rec.Filter, s.caps.Filters),
		DataMode:            codeplug.BoolField{State: codeplug.Known, Value: stringOf(rec.DataMode) == "ON"},
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
		Preamp:              codeplug.StringField{State: codeplug.Unavailable},
		Antenna:             codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},
	}
}

func vocabField(v civ.Optional[string], allowed []string) codeplug.StringField {
	value, ok := v.Get()
	if !ok {
		return codeplug.StringField{State: codeplug.Unavailable}
	}
	for _, candidate := range allowed {
		if value == candidate {
			return codeplug.StringField{State: codeplug.Known, Value: value}
		}
	}
	return codeplug.StringField{State: codeplug.Unknown}
}

func (s *Session) toneField(v civ.Optional[uint64]) codeplug.ToneField {
	value, ok := v.Get()
	if !ok {
		return codeplug.ToneField{State: codeplug.Unavailable}
	}
	tone := spec.Tone(value)
	if !s.caps.AdmitsTone(tone) {
		return codeplug.ToneField{State: codeplug.Unknown}
	}
	return codeplug.ToneField{State: codeplug.Known, Value: tone}
}

func (s *Session) dtcsField(v civ.Optional[uint64]) codeplug.IntField {
	value, ok := v.Get()
	if !ok {
		return codeplug.IntField{State: codeplug.Unavailable}
	}
	for _, code := range s.caps.DTCSCodes {
		if code == int(value) {
			return codeplug.IntField{State: codeplug.Known, Value: code}
		}
	}
	return codeplug.IntField{State: codeplug.Unknown}
}

func duplexValues(c spec.Capabilities) []string {
	out := make([]string, 0, len(c.DuplexOptions))
	for _, v := range c.DuplexOptions {
		out = append(out, v.Value)
	}
	return out
}
func toneModeValues(c spec.Capabilities) []string {
	out := make([]string, 0, len(c.ToneModes))
	for _, v := range c.ToneModes {
		out = append(out, v.Value)
	}
	return out
}
func numberOf(v civ.Optional[uint64]) uint64 { value, _ := v.Get(); return value }

// freqFieldOf reports the correct non-Known FreqField state for an
// absent civ.Optional: Unavailable, matching how the other Icom drivers
// build a FreqField for "the record does not carry it" (e.g. TxFreqHz in
// core/driver/ic7760/read.go when the field is absent). numberOf alone
// discards the presence flag, which is the right choice only for FreqHz
// (:105) — a bare uint64 by a known upstream defect a separate design
// task owns — and the wrong one for TxFreqHz/OffsetHz, which do carry a
// FieldState.
func freqFieldOf(v civ.Optional[uint64]) codeplug.FreqField {
	value, ok := v.Get()
	if !ok {
		return codeplug.FreqField{State: codeplug.Unavailable}
	}
	return codeplug.FreqField{State: codeplug.Known, Value: value}
}
func stringOf(v civ.Optional[string]) string { value, _ := v.Get(); return value }
