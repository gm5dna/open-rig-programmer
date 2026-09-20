// SPDX-License-Identifier: GPL-3.0-or-later

package ft450d

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/yaesu"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// mrAnswerLen is this radio's whole MR-answer/MW-set frame length: 27
// bytes (matrix §1.1, Lift Y's MemoryFrameLen).
const mrAnswerLen = 27

func mrSpec() transport.CommandSpec { return transport.CATReadSpec("MR", mrAnswerLen, 1) }

// ctcssNames/ctcssByName map the wire CTCSS state to codeplug's display
// spelling — matrix §2.14's three-value family, the same three every
// sibling driver in this wave uses.
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
// spelling — matrix §2.13's three-value family.
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

// acceptedKinds is the P7 kind-byte domain an MR answer may legitimately
// carry, for EITHER bank. The MR answer's own legend prints "0: VFO
// 1: Memory" (layout:796) — the ordinary two-value domain. NO FT-450D HAS
// EVER BEEN ASKED ANYTHING BY THIS PROJECT, so there is no hardware finding
// to widen this against, on MEM or on PMS alike.
var acceptedKinds = []byte{cat.KindVFO, cat.KindMemory}

func kindAccepted(got byte) bool {
	for _, k := range acceptedKinds {
		if k == got {
			return true
		}
	}
	return false
}

// ReadChannel implements driver.Session: a single MR (channel data) read.
// THIS RADIO HAS NO MT COMMAND (matrix §0), so there is no second (tag)
// exchange at all — ChannelData's Tag stays "" and TagDisplay stays
// Unavailable on every read, including PMS slots.
//
// The empty-slot rule: a "?;" rejection of the MR read is mapped to an
// EMPTY channel, not an error — the same ASSUMED convention every sibling
// driver's read path carries (no FT-450D has ever been asked what an
// unpopulated slot's MR answers).
//
// CTCSSTone comes back KNOWN: matrix §1.3 — P9 is a live two-digit index
// into the standard 50-entry CTCSS tone chart. ScanSkip stays Unknown: no
// scan-skip byte exists in the 27-byte record, so this driver must never
// guess one. Reading a PMS slot (501-504) uses the SAME MR frame as MEM —
// only the write side is restricted (caps.go's pmsFields); this method
// makes no bank distinction at all.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	sl, err := s.dialect.ParseSlot(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft450d: ReadChannel: %w", err)
	}

	cmd, err := s.dialect.BuildMRRead(sl)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft450d: ReadChannel: %w", err)
	}

	frame, err := s.eng.Do(ctx, cmd, mrSpec())
	if errors.Is(err, cat.ErrRejected) {
		// ASSUMED empty-slot answer — see the doc comment above.
		return codeplug.Channel{Slot: sl.Wire()}, nil
	}
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft450d: ReadChannel %s: MR: %w", sl.Wire(), err)
	}

	m, err := s.dialect.ParseMRAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft450d: ReadChannel %s: %w", sl.Wire(), err)
	}
	if m.Slot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Model: modelName, Requested: sl.Wire(), Answered: m.Slot.Wire()}
	}
	if !kindAccepted(m.Kind) {
		return codeplug.Channel{}, &KindMismatchError{Model: "ft450d", Slot: sl.Wire(), Got: m.Kind, Want: acceptedKinds}
	}

	ctcss, ok := ctcssNames[m.CTCSS]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ft450d: ReadChannel %s: unmapped CTCSS state %q", sl.Wire(), m.CTCSS)
	}
	shift, ok := shiftNames[m.Shift]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ft450d: ReadChannel %s: unmapped shift %q", sl.Wire(), m.Shift)
	}
	tone, ok := toneForIndex(m.ToneIndex)
	if !ok {
		// Unreachable after ParseMRAnswer's own P9ToneIndex validation
		// (0-49); refuse rather than silently mislabel if it ever isn't.
		return codeplug.Channel{}, fmt.Errorf("ft450d: ReadChannel %s: tone index %d is outside the 50-entry chart", sl.Wire(), m.ToneIndex)
	}

	return codeplug.Channel{
		Slot: sl.Wire(),
		Data: &codeplug.ChannelData{
			FreqHz: uint64(m.FreqHz),
			Mode:   s.dialect.ModeName(m.Mode),
			ClarHz: int(m.ClarHz),
			RxClar: m.RxClar,
			TxClar: m.TxClar,
			CTCSS:  ctcss,
			CTCSSTone: codeplug.ToneField{
				State: codeplug.Known, Value: tone,
			},
			Shift:      shift,
			Tag:        "", // NoTag: no CAT route carries tag text (matrix §0/§2.7)
			TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown}, // unreadable via CAT

			// The fields the Icom model extensions added to the neutral
			// memory model. UNAVAILABLE on this radio: this family's
			// memory frame carries none of them, and caps.go's bank grades
			// all of them the zero FieldSupport.
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
// of this radio's accepted read-side values (acceptedKinds, above). The
// shared form (yaesu.KindMismatchError) carries the model name so this
// package needs no typed error of its own.
type KindMismatchError = yaesu.KindMismatchError
