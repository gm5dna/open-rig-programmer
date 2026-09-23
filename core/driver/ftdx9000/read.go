// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx9000

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// ctcssNames/ctcssByName map the wire CTCSS state to codeplug's display
// spelling — matrix §1.16's three-value family, the same three every
// sibling driver uses.
var ctcssNames = map[cat.CTCSSState]string{
	cat.CTCSSOff:    "OFF",
	cat.CTCSSEncDec: "ENC-DEC",
	cat.CTCSSEnc:    "ENC",
}

var ctcssByName = map[string]cat.CTCSSState{
	"OFF":     cat.CTCSSOff,
	"ENC-DEC": cat.CTCSSEncDec,
	"ENC":     cat.CTCSSEnc,
}

// shiftNames/shiftByName map the wire shift state to codeplug's display
// spelling — matrix §1.15's three-value family.
var shiftNames = map[cat.Shift]string{
	cat.ShiftSimplex: "SIMPLEX",
	cat.ShiftPlus:    "PLUS",
	cat.ShiftMinus:   "MINUS",
}

var shiftByName = map[string]cat.Shift{
	"SIMPLEX": cat.ShiftSimplex,
	"PLUS":    cat.ShiftPlus,
	"MINUS":   cat.ShiftMinus,
}

// mrAnswerLen is this radio's whole MR-answer/MW-set frame length: 27
// bytes (matrix §2, Lift Y's MemoryFrameLen). Written down here rather
// than derived, for the same reason ft891/read.go gives: core/cat exposes
// no accessor for the shared block's width.
const mrAnswerLen = 27

func mrSpec() transport.CommandSpec { return transport.CATReadSpec("MR", mrAnswerLen, 1) }

// ReadChannel implements driver.Session: a single MR read, mapped into one
// codeplug.Channel. THIS RADIO HAS NO MT COMMAND (matrix §2; see
// dialect.go and write.go) so, unlike every sibling driver, there is no
// second (tag) exchange at all.
//
// The empty-slot rule: a "?;" rejection of the MR read is mapped to an
// EMPTY channel, not an error — the same ASSUMED convention every sibling
// driver carries (no FTdx9000 has ever been asked; this is the protocol's
// one unattributed NAK).
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	sl, err := s.dialect.ParseSlot(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx9000: ReadChannel: %w", err)
	}

	cmd, err := s.dialect.BuildMRRead(sl)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx9000: ReadChannel: %w", err)
	}

	frame, err := s.eng.Do(ctx, cmd, mrSpec())
	if errors.Is(err, cat.ErrRejected) {
		return codeplug.Channel{Slot: sl.Wire()}, nil
	}
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx9000: ReadChannel %s: MR: %w", sl.Wire(), err)
	}

	m, err := s.dialect.ParseMRAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx9000: ReadChannel %s: %w: %w", sl.Wire(), driver.ErrRecordDecode, err)
	}
	if m.Slot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Model: modelName, Requested: sl.Wire(), Answered: m.Slot.Wire()}
	}

	ctcss, ok := ctcssNames[m.CTCSS]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ftdx9000: ReadChannel %s: unmapped CTCSS state %q", sl.Wire(), m.CTCSS)
	}
	shift, ok := shiftNames[m.Shift]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ftdx9000: ReadChannel %s: unmapped shift %q", sl.Wire(), m.Shift)
	}
	tone, ok := toneForIndex(m.ToneIndex)
	if !ok {
		// Unreachable after ParseMRAnswer's own P9ToneIndex validation
		// (0-49); refuse rather than silently mislabel if it ever isn't.
		return codeplug.Channel{}, fmt.Errorf("ftdx9000: ReadChannel %s: tone index %d is outside the 50-entry chart", sl.Wire(), m.ToneIndex)
	}

	return codeplug.Channel{
		Slot: sl.Wire(),
		Data: &codeplug.ChannelData{
			FreqHz: uint64(m.FreqHz),
			// Rendered through THIS session's dialect (matrix §1.5's
			// mode-spelling CHOICE), never cat.Mode.String.
			Mode:      s.dialect.ModeName(m.Mode),
			ClarHz:    int(m.ClarHz),
			RxClar:    m.RxClar,
			TxClar:    m.TxClar,
			CTCSS:     ctcss,
			CTCSSTone: codeplug.ToneField{State: codeplug.Known, Value: tone},
			Shift:     shift,
			// NoTag: no tag exists to read (Tag stays "").
			TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unavailable},

			// The Icom-tier fields (design D4/D8): UNAVAILABLE — the
			// 27-byte record carries none of them, and caps.go's banks
			// list none of these spec.Fields either. Stated explicitly, not
			// left at the Go zero (Absent): wiring's read invariant
			// requires a fresh read to state Known/Unknown/Unavailable for
			// every field, per ftdx5000's identical read (same family).
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
