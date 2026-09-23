// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx3000

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// mrSpec is the transport spec for an MR read: fixed 27-byte answer
// (matrix §1.1). One retry: an MR read is idempotent.
func mrSpec() transport.CommandSpec {
	return transport.CATReadSpec("MR", 27, 1)
}

var ctcssNames = map[cat.CTCSSState]string{
	cat.CTCSSOff:    "OFF",
	cat.CTCSSEncDec: "ENC-DEC",
	cat.CTCSSEnc:    "ENC",
}

var shiftNames = map[cat.Shift]string{
	cat.ShiftSimplex: "SIMPLEX",
	cat.ShiftPlus:    "PLUS",
	cat.ShiftMinus:   "MINUS",
}

// acceptedKinds is the P7 kind-byte domain an MR answer may legitimately
// carry, for EITHER bank. Matrix §1.4: the read side prints only "0: VFO
// 1: Memory" — the ordinary two-value domain. NO FTdx3000 HAS EVER BEEN
// ASKED ANYTHING BY THIS PROJECT, so there is no hardware finding to widen
// this against.
var acceptedKinds = []byte{cat.KindVFO, cat.KindMemory}

func kindAccepted(got byte) bool {
	for _, k := range acceptedKinds {
		if k == got {
			return true
		}
	}
	return false
}

// ReadChannel implements driver.Session: a single MR read. There is no MT
// to sequence beside it — this family has no tag/name route over CAT at
// all (matrix §0), so ChannelData's Tag stays "" and TagDisplay stays
// Unavailable on every read.
//
// The empty-slot rule: a "?;" rejection is mapped to an EMPTY channel, not
// an error — ASSUMED, the same reasoning every registered sibling's read
// path carries.
//
// CTCSSTone comes back KNOWN: matrix §1.3 — P9's read side is a live
// two-digit index into the standard 50-entry CTCSS tone chart, even
// though the write side is fixed (dialect.go's P9ToneIndexReadOnly).
// ScanSkip stays Unknown: no scan-skip byte exists in the 27-byte record.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	sl, err := s.dialect.ParseSlot(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx3000: ReadChannel: %w", err)
	}

	cmd, err := s.dialect.BuildMRRead(sl)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx3000: ReadChannel: %w", err)
	}

	frame, err := s.eng.Do(ctx, cmd, mrSpec())
	if errors.Is(err, cat.ErrRejected) {
		return codeplug.Channel{Slot: sl.Wire()}, nil
	}
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx3000: ReadChannel %s: MR: %w", sl.Wire(), err)
	}

	m, err := s.dialect.ParseMRAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx3000: ReadChannel %s: %w: %w", sl.Wire(), driver.ErrRecordDecode, err)
	}
	if m.Slot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Model: s.dialect.CATID(), Requested: sl.Wire(), Answered: m.Slot.Wire()}
	}
	if !kindAccepted(m.Kind) {
		return codeplug.Channel{}, &KindMismatchError{Model: "ftdx3000", Slot: sl.Wire(), Got: m.Kind, Want: acceptedKinds}
	}

	ctcss, ok := ctcssNames[m.CTCSS]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ftdx3000: ReadChannel %s: unmapped CTCSS state %q", sl.Wire(), m.CTCSS)
	}
	shift, ok := shiftNames[m.Shift]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ftdx3000: ReadChannel %s: unmapped shift %q", sl.Wire(), m.Shift)
	}

	caps := s.caps
	var tone codeplug.ToneField
	if int(m.ToneIndex) < len(caps.CTCSSTones) {
		tone = codeplug.ToneField{State: codeplug.Known, Value: caps.CTCSSTones[m.ToneIndex]}
	} else {
		// Unreachable after cat.ParseMRAnswer's own 0-49 bound
		// (memdata.go), narrower than CTCSSTones' 50 entries; refuse
		// rather than silently mislabel if it ever isn't.
		return codeplug.Channel{}, fmt.Errorf("ftdx3000: ReadChannel %s: tone index %d out of range", sl.Wire(), m.ToneIndex)
	}

	return codeplug.Channel{
		Slot: sl.Wire(),
		Data: &codeplug.ChannelData{
			FreqHz:     uint64(m.FreqHz),
			Mode:       s.dialect.ModeName(m.Mode),
			ClarHz:     int(m.ClarHz),
			RxClar:     m.RxClar,
			TxClar:     m.TxClar,
			CTCSS:      ctcss,
			CTCSSTone:  tone,
			Shift:      shift,
			Tag:        "",
			TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},

			// The Icom-tier fields: UNAVAILABLE on this radio.
			TxFreqHz:            codeplug.FreqField{State: codeplug.Unavailable},
			Duplex:              codeplug.StringField{State: codeplug.Unavailable},
			OffsetHz:            codeplug.FreqField{State: codeplug.Unavailable},
			ToneMode:            codeplug.StringField{State: codeplug.Unavailable},
			ToneTx:              codeplug.ToneField{State: codeplug.Unavailable},
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
		},
	}, nil
}

// KindMismatchError reports that an MR answer's P7 kind byte was not one
// of this radio's accepted read-side values. The shared form
// (yaesu.KindMismatchError) carries the model name so this package needs
// no typed error of its own.
type KindMismatchError = yaesu.KindMismatchError
