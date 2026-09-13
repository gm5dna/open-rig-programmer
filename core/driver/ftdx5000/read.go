// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx5000

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// mrSpec is the transport spec for an MR read: this radio's OWN
// MemoryFrameLen (27 bytes, matrix §1.1), one retry (a read is
// idempotent).
func mrSpec() transport.CommandSpec {
	return transport.CATReadSpec("MR", 27, 1)
}

// ReadChannel implements driver.Session: one MR read, mapped into one
// codeplug.Channel. Unlike every other registered Yaesu driver this is
// NOT an MT read — this radio has no MT command at all (doc.go).
//
// The empty-slot rule: a "?;" rejection is mapped to an EMPTY channel
// (Data nil, the slot carried through), the same ASSUMED convention every
// registered sibling's MR/MT read uses — "?;" is the protocol's single
// unattributed NAK, so reading "empty" out of it is an interpretation, and
// this radio's MR read of an empty channel has never been observed.
//
// TagDisplay comes back Unavailable, ALWAYS: matrix §2 confirms no field
// exists after P10 Shift in the 27-byte record at all, so there is no
// display flag to report. CTCSSTone comes back Known, ALWAYS: unlike every
// sibling's fixed "00" P9, this radio's is a live, decoded tone-table
// index (doc.go). ScanSkip and the seventeen Icom-tier fields come back
// Unavailable: this record has no room for any of them.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	sl, err := s.dialect.ParseSlot(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx5000: ReadChannel: %w", err)
	}

	cmd, err := s.dialect.BuildMRRead(sl)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx5000: ReadChannel: %w", err)
	}

	frame, err := s.eng.Do(ctx, cmd, mrSpec())
	if errors.Is(err, cat.ErrRejected) {
		return codeplug.Channel{Slot: sl.Wire()}, nil
	}
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx5000: ReadChannel %s: MR: %w", sl.Wire(), err)
	}

	m, err := s.dialect.ParseMRAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftdx5000: ReadChannel %s: %w", sl.Wire(), err)
	}
	if m.Slot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Model: modelName, Requested: sl.Wire(), Answered: m.Slot.Wire()}
	}

	ctcssName, ok := ctcssNames[m.CTCSS]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ftdx5000: ReadChannel %s: unmapped CTCSS state %q", sl.Wire(), m.CTCSS)
	}
	shiftName, ok := shiftNames[m.Shift]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ftdx5000: ReadChannel %s: unmapped shift %q", sl.Wire(), m.Shift)
	}

	tone, ok := toneForIndex(m.ToneIndex)
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ftdx5000: ReadChannel %s: tone index %d is outside the standard 50-entry chart", sl.Wire(), m.ToneIndex)
	}

	return codeplug.Channel{
		Slot: sl.Wire(),
		Data: &codeplug.ChannelData{
			FreqHz: uint64(m.FreqHz),
			// Rendered through THIS session's dialect, not cat.Mode.String:
			// the string is user-visible, so it comes from the mode table
			// of the radio that answered.
			Mode:      s.dialect.ModeName(m.Mode),
			ClarHz:    int(m.ClarHz),
			RxClar:    m.RxClar,
			TxClar:    m.TxClar,
			CTCSS:     ctcssName,
			CTCSSTone: codeplug.ToneField{State: codeplug.Known, Value: tone},
			Shift:     shiftName,
			// NoTag: no tag exists to read (Tag stays "").
			TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unavailable},

			// The Icom-tier fields (design D4/D8): UNAVAILABLE on this
			// radio — the 27-byte record carries none of them, and caps.go's
			// banks list none of these spec.Fields either.
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

// toneForIndex returns the standard chart's tone at index idx, and whether
// idx was in range (0-49) — the read-direction mirror of write.go's
// toneIndexFor.
func toneForIndex(idx uint8) (spec.Tone, bool) {
	tones := spec.StandardCTCSSTones()
	if int(idx) >= len(tones) {
		return 0, false
	}
	return tones[idx], true
}
