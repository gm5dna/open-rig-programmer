// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civicr8600 "github.com/gm5dna/open-rig-programmer/core/civ/icr8600"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

var ErrAnswerMismatch = errors.New("icr8600: the memory answer names a different channel than was requested")

type AnswerMismatchError struct {
	Requested civ.ChannelAddress
	Answered  civ.ChannelAddress
}

func (e *AnswerMismatchError) Error() string {
	return fmt.Sprintf("icr8600: requested %s but the answer names %s", e.Requested, e.Answered)
}

func (e *AnswerMismatchError) Unwrap() error { return ErrAnswerMismatch }

func slotAddress(slot string) (civ.ChannelAddress, error) {
	g, c, ok := spec.ParseSparseSlot(slot)
	if !ok || g < 0 || g >= civicr8600.MemoryGroups || c < 0 || c >= civicr8600.MemoryChannelsPerGroup {
		return civ.ChannelAddress{}, fmt.Errorf("icr8600: slot %q is outside the zero-based 100x100 MEM space", slot)
	}
	return civ.ChannelAddress{Group: g, Channel: c}, nil
}

func isEmptyRecord(record []byte) bool {
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

// discover performs one bounded sparse walk. Group zero is read in full;
// each later group's channel zero is sampled and the remainder is read only
// when that sample is occupied. Thus the walk is between 199 and 10,000
// reads, never outside the declared 100x100 space. The first occupied record
// supplies the diagnostic fingerprint; every record is still validated.
func (s *Session) discover(ctx context.Context) ([]string, error) {
	var slots []string
	probe := func(addr civ.ChannelAddress) (bool, error) {
		s.report.SlotsTried++
		record, present, err := s.recordAt(ctx, addr)
		if err != nil {
			var lengthErr *civ.RecordLengthError
			if errors.As(err, &lengthErr) {
				want := make([]string, len(lengthErr.Want))
				for i, n := range lengthErr.Want {
					want[i] = strconv.Itoa(n)
				}
				return false, &driver.WrongRadioError{Want: "record " + strings.Join(want, "/"), Got: "record " + strconv.Itoa(lengthErr.Got)}
			}
			return false, err
		}
		if !present || isEmptyRecord(record) {
			return false, nil
		}
		if _, err := s.profile.LayoutForRecord(record); err != nil {
			return false, err
		}
		slot := spec.SparseSlot(addr.Group, addr.Channel)
		slots = append(slots, slot)
		if !s.report.Fingerprinted {
			s.report.Fingerprinted = true
			s.report.RecordLength = len(record)
			s.report.FirstOccupied = slot
		}
		return true, nil
	}

	for channel := 0; channel < civicr8600.MemoryChannelsPerGroup; channel++ {
		if _, err := probe(civ.ChannelAddress{Group: 0, Channel: channel}); err != nil {
			return nil, err
		}
	}
	for group := 1; group < civicr8600.MemoryGroups; group++ {
		present, err := probe(civ.ChannelAddress{Group: group, Channel: 0})
		if err != nil {
			return nil, err
		}
		if !present {
			continue
		}
		for channel := 1; channel < civicr8600.MemoryChannelsPerGroup; channel++ {
			if _, err := probe(civ.ChannelAddress{Group: group, Channel: channel}); err != nil {
				return nil, err
			}
		}
	}
	return slots, nil
}

func (s *Session) recordAt(ctx context.Context, addr civ.ChannelAddress) ([]byte, bool, error) {
	cmd, err := s.profile.BuildMemoryRead(addr)
	if err != nil {
		return nil, false, err
	}
	frame, err := s.eng.Do(ctx, cmd, s.memoryReadSpec())
	if errors.Is(err, transport.ErrRejected) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	got, record, err := s.profile.MemoryAnswerRecord(frame)
	if err != nil {
		return nil, false, err
	}
	if got != addr {
		s.answerMismatches.Add(1)
		return nil, false, &AnswerMismatchError{Requested: addr, Answered: got}
	}
	return record, true, nil
}

func (s *Session) parseRecord(addr civ.ChannelAddress, record []byte) (civ.MemoryRecord, error) {
	read, err := s.profile.BuildMemoryRead(addr)
	if err != nil {
		return civ.MemoryRecord{}, err
	}
	request := read.Bytes()
	frame := []byte{0xFE, 0xFE, s.profile.ControllerAddress(), s.profile.RadioAddress(), 0x1A, 0x00}
	frame = append(frame, request[6:len(request)-1]...)
	frame = append(frame, record...)
	frame = append(frame, 0xFD)
	return s.profile.ParseMemoryAnswer(frame)
}

func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	addr, err := slotAddress(slot)
	if err != nil {
		return codeplug.Channel{}, err
	}
	record, present, err := s.recordAt(ctx, addr)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("icr8600: ReadChannel %s: %w", slot, err)
	}
	if !present || isEmptyRecord(record) {
		return codeplug.Channel{Slot: slot}, nil
	}
	rec, err := s.parseRecord(addr, record)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("icr8600: ReadChannel %s: %w", slot, err)
	}
	return neutralChannel(rec, slot, s.caps), nil
}

func neutralChannel(rec civ.MemoryRecord, slot string, caps spec.Capabilities) codeplug.Channel {
	freq, _ := rec.RXFreqHz.Get()
	mode, _ := rec.Mode.Get()
	name, _ := rec.Name.Get()
	duplex, _ := rec.Duplex.Get()
	offset, _ := rec.OffsetHz.Get()
	filter, _ := rec.Filter.Get()
	tsEnabled, _ := rec.TuningStepEnabled.Get()
	ts, _ := rec.TuningStep.Get()
	programStep, _ := rec.ProgramTuningStepHz.Get()
	attenuator, _ := rec.AttenuatorDB.Get()
	preamp, _ := rec.Preamp.Get()
	antenna, _ := rec.Antenna.Get()
	ipPlus, _ := rec.IPPlus.Get()

	unavailableBool := codeplug.BoolField{State: codeplug.Unavailable}
	unavailableString := codeplug.StringField{State: codeplug.Unavailable}
	unavailableFreq := codeplug.FreqField{State: codeplug.Unavailable}
	unavailableTone := codeplug.ToneField{State: codeplug.Unavailable}
	unavailableInt := codeplug.IntField{State: codeplug.Unavailable}

	d := &codeplug.ChannelData{
		FreqHz: freq, Mode: mode, Tag: name,
		CTCSSTone: unavailableTone, TagDisplay: unavailableBool,
		ScanSkip: unavailableBool, TxFreqHz: unavailableFreq,
		Duplex:   codeplug.StringField{State: codeplug.Known, Value: duplex},
		OffsetHz: codeplug.FreqField{State: codeplug.Known, Value: offset},
		ToneMode: unavailableString, ToneTx: unavailableTone, ToneRx: unavailableTone,
		DTCSCode: unavailableInt, DTCSPolarity: unavailableString,
		Filter:              codeplug.StringField{State: codeplug.Known, Value: filter},
		DataMode:            unavailableBool,
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Known, Value: tsEnabled == "ON"},
		TuningStep:          codeplug.StringField{State: codeplug.Known, Value: ts},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Known, Value: programStep},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Known, Value: int(attenuator)},
		Preamp:              codeplug.StringField{State: codeplug.Known, Value: preamp},
		Antenna:             codeplug.StringField{State: codeplug.Known, Value: antenna},
		IPPlus:              codeplug.BoolField{State: codeplug.Known, Value: ipPlus == "ON"},
	}
	if v, ok := rec.ToneMode.Get(); ok {
		d.ToneMode = codeplug.StringField{State: codeplug.Known, Value: v}
	}
	if v, ok := rec.ToneRXDeciHz.Get(); ok {
		if caps.AdmitsTone(spec.Tone(v)) {
			d.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(v)}
		} else {
			d.ToneRx = codeplug.ToneField{State: codeplug.Unknown}
		}
	}
	if v, ok := rec.DTCSCode.Get(); ok {
		d.DTCSCode = codeplug.IntField{State: codeplug.Known, Value: int(v)}
	}
	if v, ok := rec.DTCSPolarity.Get(); ok {
		d.DTCSPolarity = codeplug.StringField{State: codeplug.Known, Value: v}
	}
	return codeplug.Channel{Slot: slot, Data: d}
}
