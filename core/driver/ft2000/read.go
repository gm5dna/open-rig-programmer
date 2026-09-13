// SPDX-License-Identifier: GPL-3.0-or-later

package ft2000

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// mrSpec is the transport spec for an MR read: fixed 27-byte answer (this
// family's frame width, matrix §1.1 — one byte narrower than every
// registered sibling's 28). One retry: an MR read is idempotent.
func mrSpec() transport.CommandSpec {
	return transport.CATReadSpec("MR", 27, 1)
}

// ctcssNames maps the wire CTCSS state to codeplug's display spelling.
var ctcssNames = map[cat.CTCSSState]string{
	cat.CTCSSOff:    "OFF",
	cat.CTCSSEncDec: "ENC-DEC",
	cat.CTCSSEnc:    "ENC",
}

// shiftNames maps the wire shift state to codeplug's display spelling.
var shiftNames = map[cat.Shift]string{
	cat.ShiftSimplex: "SIMPLEX",
	cat.ShiftPlus:    "PLUS",
	cat.ShiftMinus:   "MINUS",
}

// acceptedKinds is the P7 kind-byte domain an MR answer may legitimately
// carry, for EITHER bank. Matrix §1.4: the read side prints only "0: VFO
// 1: Memory" — the ordinary two-value domain, not the FT-710's
// hardware-widened lenient set. NO FT-2000 OR FT-2000D HAS EVER BEEN
// ASKED ANYTHING BY THIS PROJECT, so there is no hardware finding to widen
// this against, on MEM or on PMS alike — the strict, manual-printed
// domain is all this driver claims.
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
// There is no MT to sequence beside it — this family has no tag/name
// route over CAT at all (matrix §0), so ChannelData's Tag stays "" and
// TagDisplay stays Unavailable on every read.
//
// The empty-slot rule: a "?;" rejection of the MR read is mapped to an
// EMPTY channel, not an error — ASSUMED, the same reasoning every
// registered sibling's read path carries (no FT-2000 has ever been asked
// what an unpopulated slot's MR answers).
//
// CTCSSTone comes back KNOWN, unlike every registered sibling: matrix
// §1.3 — P9 is a live two-digit index into the standard 50-entry CTCSS
// tone chart, the first memory record in this fleet to carry one at all.
// ScanSkip stays Unknown: no scan-skip byte exists in the 27-byte record
// (matrix §3), so this driver must never guess one.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	sl, err := s.dialect.ParseSlot(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft2000: ReadChannel: %w", err)
	}

	cmd, err := s.dialect.BuildMRRead(sl)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft2000: ReadChannel: %w", err)
	}

	frame, err := s.eng.Do(ctx, cmd, mrSpec())
	if errors.Is(err, cat.ErrRejected) {
		// ASSUMED empty-slot answer — see the doc comment above.
		return codeplug.Channel{Slot: sl.Wire()}, nil
	}
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft2000: ReadChannel %s: MR: %w", sl.Wire(), err)
	}

	m, err := s.dialect.ParseMRAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ft2000: ReadChannel %s: %w", sl.Wire(), err)
	}
	if m.Slot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Model: s.dialect.CATID(), Requested: sl.Wire(), Answered: m.Slot.Wire()}
	}
	if !kindAccepted(m.Kind) {
		return codeplug.Channel{}, &KindMismatchError{Slot: sl.Wire(), Got: m.Kind, Want: acceptedKinds}
	}

	ctcss, ok := ctcssNames[m.CTCSS]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ft2000: ReadChannel %s: unmapped CTCSS state %q", sl.Wire(), m.CTCSS)
	}
	shift, ok := shiftNames[m.Shift]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ft2000: ReadChannel %s: unmapped shift %q", sl.Wire(), m.Shift)
	}

	caps := s.caps
	var tone codeplug.ToneField
	if int(m.ToneIndex) < len(caps.CTCSSTones) {
		tone = codeplug.ToneField{State: codeplug.Known, Value: caps.CTCSSTones[m.ToneIndex]}
	} else {
		// Unreachable after cat.ParseMRAnswer's own P9ToneIndex bound
		// (0-49, memdata.go), which is narrower than CTCSSTones' 50
		// entries; refuse rather than silently mislabel if it ever isn't.
		return codeplug.Channel{}, fmt.Errorf("ft2000: ReadChannel %s: tone index %d out of range", sl.Wire(), m.ToneIndex)
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
			Tag:        "", // NoTag: no tag/name route over CAT at all (matrix §0)
			TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown}, // unreadable via CAT (matrix §3)

			// The fields the Icom model extensions added to the neutral
			// memory model. UNAVAILABLE on this radio: this family's
			// memory frame carries none of them, and caps.go's bank
			// grades all of them the zero FieldSupport.
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
