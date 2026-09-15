// SPDX-License-Identifier: GPL-3.0-or-later

package ft890900

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gm5dna/open-rig-programmer/core/bincat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// slotToChannel maps a canonical 3-digit wire-form slot ("001".."NNN") to
// this radio's own 1-based channel number, refusing anything outside its
// TRUE slot count (not the wider engine-gate SlotCount — see profile.go's
// doc comment).
func (s *Session) slotToChannel(slot string) (int, error) {
	if len(slot) != 3 {
		return 0, fmt.Errorf("ft890900: %q is not a slot on this radio: a memory is three digits (\"001\"..%03d)", slot, s.info.TrueSlotCount)
	}
	n, err := strconv.Atoi(slot)
	if err != nil {
		return 0, fmt.Errorf("ft890900: %q is not a slot on this radio: %w", slot, err)
	}
	if n < slotBase || n > s.info.TrueSlotCount {
		return 0, fmt.Errorf("ft890900: %q is outside this radio's memory range \"001\"..%03d", slot, s.info.TrueSlotCount)
	}
	return n, nil
}

// readRecord performs one Status-Update memory-record read (opcode 10H,
// U=4) and decodes it.
func (s *Session) readRecord(ctx context.Context, ch int) (bincat.Record, error) {
	cmd := bincat.NewCommand(bincat.OpStatusUpdate, [4]byte{bincat.UMemoryRecord, 0, 0, byte(ch)})
	frame, err := s.eng.Do(ctx, cmd, bincat.ReadSpec(0, 1))
	if err != nil {
		return bincat.Record{}, fmt.Errorf("ft890900: %s: reading CH=%02Xh: %w", s.info.Name, ch, err)
	}
	rec, err := bincat.ParseRecord(frame, s.info.EngineProfile)
	if err != nil {
		return bincat.Record{}, fmt.Errorf("ft890900: %s: reading CH=%02Xh: %w", s.info.Name, ch, err)
	}
	return rec, nil
}

// ReadChannel implements driver.Session. An empty (Blanked) slot comes
// back as an empty codeplug.Channel; every other slot decodes into a
// populated one.
//
// FieldOffset (the repeater-offset magnitude) comes back Unavailable
// unconditionally: the record has no byte for it (caps.go's
// offsetSupport doc comment). FieldCTCSSState (the CTCSS on/off toggle)
// likewise, since no opcode/byte for it was found in either manual
// (capability matrices).
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	ch, err := s.slotToChannel(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft890900: ReadChannel: %w", err)
	}
	rec, err := s.readRecord(ctx, ch)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft890900: ReadChannel %s: %w", slot, err)
	}
	if rec.Blanked {
		return codeplug.Channel{Slot: slot}, nil
	}
	if rec.Mode == "" {
		return codeplug.Channel{}, fmt.Errorf("ft890900: ReadChannel %s: unrecognised mode byte %#02x", slot, rec.ModeByte)
	}

	data := &codeplug.ChannelData{
		FreqHz: uint64(rec.FreqTensOfHz) * 10,
		Mode:   rec.Mode,
		Shift:  rec.Shift,

		// ASSUMED: neither manual states which of Rx/Tx this radio's
		// single clarifier byte applies to; this driver reports it as an
		// RX clarifier only (the ordinary Yaesu convention absent a
		// documented split). Read-only (caps.go's clarSupport): never
		// written back with a guessed encoding.
		ClarHz: int(rec.ClarifierHz),
		RxClar: rec.ClarifierHz != 0,
		TxClar: false,

		CTCSSTone: toneField(rec.Tone),

		// Unavailable: no wire route reads these back at all (caps.go's
		// offsetSupport doc comment; "CTCSSStates: OPEN" in the matrices).
		OffsetHz: codeplug.FreqField{State: codeplug.Unavailable},
		CTCSS:    "",

		TagDisplay:   codeplug.BoolField{State: codeplug.Unavailable},
		ScanSkip:     codeplug.BoolField{State: codeplug.Unavailable},
		Duplex:       codeplug.StringField{State: codeplug.Unavailable},
		ToneMode:     codeplug.StringField{State: codeplug.Unavailable},
		ToneTx:       codeplug.ToneField{State: codeplug.Unavailable},
		ToneRx:       codeplug.ToneField{State: codeplug.Unavailable},
		DTCSCode:     codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},
		Filter:       codeplug.StringField{State: codeplug.Unavailable},
		DataMode:     codeplug.BoolField{State: codeplug.Unavailable},
		TxFreqHz:     codeplug.FreqField{State: codeplug.Unavailable},

		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
		Preamp:              codeplug.StringField{State: codeplug.Unavailable},
		Antenna:             codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},
	}
	// Tag is left "" (NoTag: no name route over CAT at all).

	return codeplug.Channel{Slot: slot, Data: data}, nil
}

// toneField maps a raw tone-index byte to a codeplug.ToneField: Known when
// the index is inside the standard chart's 33-entry prefix this family
// addresses (caps.go's standardTones), Unknown otherwise (never a guessed
// value — spec.FieldState's own rule).
func toneField(idx byte) codeplug.ToneField {
	if int(idx) >= len(standardTones) {
		return codeplug.ToneField{State: codeplug.Unknown}
	}
	return codeplug.ToneField{State: codeplug.Known, Value: standardTones[idx]}
}

// toneIndexFor is toneField's write-direction inverse.
func toneIndexFor(t spec.Tone) (byte, bool) {
	for i, v := range standardTones {
		if v == t {
			return byte(i), true
		}
	}
	return 0, false
}
