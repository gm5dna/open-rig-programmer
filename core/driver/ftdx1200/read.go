// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx1200

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
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
// carry, for EITHER bank — matrix §1.4: read side prints only "0: VFO
// 1: Memory".
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
// to sequence beside it (matrix §0), so Tag stays "" and TagDisplay stays
// Unavailable. CTCSSTone comes back Unavailable, ALWAYS: unlike the
// sibling ftdx3000, P9 is printed-fixed on this radio's READ side too
// (matrix §1.3) — there is no live tone state to report. ScanSkip stays
// Unknown: no scan-skip byte exists in the 27-byte record.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	sl, err := s.dialect.ParseSlot(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx1200: ReadChannel: %w", err)
	}

	cmd, err := s.dialect.BuildMRRead(sl)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx1200: ReadChannel: %w", err)
	}

	frame, err := s.eng.Do(ctx, cmd, mrSpec())
	if errors.Is(err, cat.ErrRejected) {
		return codeplug.Channel{Slot: sl.Wire()}, nil
	}
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx1200: ReadChannel %s: MR: %w", sl.Wire(), err)
	}

	m, err := s.dialect.ParseMRAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx1200: ReadChannel %s: %w", sl.Wire(), err)
	}
	if m.Slot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Model: s.dialect.CATID(), Requested: sl.Wire(), Answered: m.Slot.Wire()}
	}
	if !kindAccepted(m.Kind) {
		return codeplug.Channel{}, &KindMismatchError{Slot: sl.Wire(), Got: m.Kind, Want: acceptedKinds}
	}

	ctcss, ok := ctcssNames[m.CTCSS]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ftdx1200: ReadChannel %s: unmapped CTCSS state %q", sl.Wire(), m.CTCSS)
	}
	shift, ok := shiftNames[m.Shift]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ftdx1200: ReadChannel %s: unmapped shift %q", sl.Wire(), m.Shift)
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
			CTCSSTone:  codeplug.ToneField{State: codeplug.Unavailable},
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
		},
	}, nil
}

// KindMismatchError reports that an MR answer's P7 kind byte was not one
// of this radio's accepted read-side values.
type KindMismatchError struct {
	Slot string
	Got  byte
	Want []byte
}

// Error implements the error interface.
func (e *KindMismatchError) Error() string {
	want := make([]string, len(e.Want))
	for i, k := range e.Want {
		want[i] = fmt.Sprintf("%q", rune(k))
	}
	return fmt.Sprintf("ftdx1200: MR answer for slot %q carries kind %q, want one of {%s}", e.Slot, rune(e.Got), strings.Join(want, ","))
}
