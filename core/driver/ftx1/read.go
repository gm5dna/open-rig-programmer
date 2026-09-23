// SPDX-License-Identifier: GPL-3.0-or-later

package ftx1

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

// mrSpec is the transport spec for an MR read: fixed 30-byte answer
// (dialect.go's MemoryFrameLen — spec.md §3.1). One retry: an MR read is
// idempotent.
func mrSpec() transport.CommandSpec {
	return transport.CATReadSpec("MR", 30, 1)
}

// mtSpec builds the transport spec for an MT read against d: the prefix,
// the EXACT answer length, and one retry. THE LENGTH IS DERIVED FROM THE
// DIALECT (yaesu.MTSpec, shared with ftdx10/ft991a's own mtSpec — see
// their doc comments): d's own MTAnswerBounds reports the geometry its
// declared MT form implies — for MTFormShortNoDisplay that is an exact
// 20, 2 + SlotDigits(5) + TagMaxBytes(12) + 1 (mtnodisplay.go) — so there
// is deliberately no 20 written out anywhere in this package.
func mtSpec(d cat.Dialect) (transport.CommandSpec, error) {
	return yaesu.MTSpec(d, &params)
}

// ctcssNames maps the wire CTCSS state to codeplug's display spelling —
// the strings codeplug.Validate checks for and caps.go's own
// Capabilities.ToneModes advertises (toneModes). Deliberately NOT
// cat.CTCSSState.String(), whose spellings ("off", "ENC/DEC") are log
// labels, not model values.
//
// SIX ENTRIES: the FTX-1's own P8 domain (spec.md §6) is wider than every
// registered dialect but the FT-991A's, and its own '3' is UNSPLIT ("3:
// DCS", not "DCS ENC/DEC"/"DCS ENC"). Bytes '4' ("PR FREQ") and '5' ("REV
// TONE") carry NO NAMED cat.CTCSSState constant at all — core/cat's own
// register explains why (dialectconfig.go's ToneStatesSix doc comment: a
// second name for byte '4' would collide with the FT-991A's CTCSSDCSEnc
// map key) — so they are constructed here by casting the byte directly,
// which is all cat.CTCSSState(c) needs to round-trip a value with no name.
var ctcssNames = map[cat.CTCSSState]string{
	cat.CTCSSOff:        "OFF",
	cat.CTCSSEncDec:     "ENC-DEC",
	cat.CTCSSEnc:        "ENC",
	cat.CTCSSDCSEncDec:  "DCS",
	cat.CTCSSState('4'): "PR-FREQ",
	cat.CTCSSState('5'): "REV-TONE",
}

// shiftNames maps the wire shift state to codeplug's display spelling
// ("SIMPLEX", "PLUS", "MINUS") — the same three-value vocabulary as
// yaesu.ShiftByName's write-direction inverse.
var shiftNames = map[cat.Shift]string{
	cat.ShiftSimplex: "SIMPLEX",
	cat.ShiftPlus:    "PLUS",
	cat.ShiftMinus:   "MINUS",
}

// ReadChannel implements driver.Session: MR (channel data) + MT (tag)
// reads, mapped into one codeplug.Channel — the FT-710's own two-frame
// shape, the only precedent in this fleet for a slot-address-only MT form
// beside a separate memory-data frame (every other registered Yaesu
// dialect either has no MT command at all or a COMBINED one carrying both
// in a single frame).
//
// The empty-slot rule: a "?;" rejection of the MR read is mapped to an
// EMPTY channel, not an error. ASSUMED, exactly as it is on every sibling
// dialect: no FTX-1 has ever answered an MR read of an unpopulated slot,
// and "?;" is the protocol's single unattributed NAK.
//
// NO KIND-BYTE NARROWING. Unlike the FT-710 (whose read-side leniency is
// HW-CONFIRMED) and the FTdx1200 (whose narrower read-side legend is
// separately printed in its own manual), the FTX-1's P7 legend states one
// domain for the whole 27-byte block (spec.md §3.1) with no distinct
// read-side chart, and no FTX-1 has ever been read to confirm or narrow
// it further (matrix §2, probe 4 — OPEN). cat.Dialect.ParseMRAnswer
// already validates the byte against the package-wide documented domain
// ('0'-'5'/'-'); this driver adds no further restriction it has no
// evidence for.
//
// TagDisplay comes back UNAVAILABLE, ALWAYS: MTFormShortNoDisplay carries
// no display byte at all (spec.md §3.3/§8) — there is no such flag to
// report, and Unavailable is what codeplug.BoolField means by "there is
// no such field" (as opposed to Unknown, "the radio has one and this read
// did not learn it").
//
// CTCSSTone and ScanSkip come back UNKNOWN, ALWAYS: the CAT protocol has
// no command that reads a memory channel's live tone-table index or
// scan-skip flag on this radio either (spec.md §3.1's own field table has
// no position for either) — Unknown means "preserve whatever the radio
// has" to every write path downstream.
func (s *Session) ReadChannel(ctx context.Context, slot string) (codeplug.Channel, error) {
	s.opMu.Lock()
	defer s.opMu.Unlock()

	sl, err := s.dialect.ParseSlot(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftx1: ReadChannel: %w", err)
	}

	mrCmd, err := s.dialect.BuildMRRead(sl)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftx1: ReadChannel: %w", err)
	}

	frame, err := s.eng.Do(ctx, mrCmd, mrSpec())
	if errors.Is(err, cat.ErrRejected) {
		return codeplug.Channel{Slot: sl.Wire()}, nil
	}
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftx1: ReadChannel %s: MR: %w", sl.Wire(), err)
	}

	m, err := s.dialect.ParseMRAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftx1: ReadChannel %s: %w: %w", sl.Wire(), driver.ErrRecordDecode, err)
	}
	if m.Slot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Model: params.Name, Requested: sl.Wire(), Answered: m.Slot.Wire()}
	}

	mtCmd, err := s.dialect.BuildMTRead(sl)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftx1: ReadChannel %s: %w", sl.Wire(), err)
	}
	mtCmdSpec, err := mtSpec(s.dialect)
	if err != nil {
		return codeplug.Channel{}, err
	}
	tframe, err := s.eng.Do(ctx, mtCmd, mtCmdSpec)
	if err != nil {
		// A successful MR followed by a rejected MT is a genuine error
		// here, not an empty-tag signal — the same posture the FT-710's
		// own ReadChannel takes (fakeradio's register item 4, applied by
		// analogy: tag state is modelled as independent of channel-data
		// state).
		return codeplug.Channel{}, fmt.Errorf("ftx1: ReadChannel %s: MT: %w", sl.Wire(), err)
	}
	tslot, tag, err := s.dialect.ParseMTAnswerNoDisplay(tframe)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("ftx1: ReadChannel %s: %w: %w", sl.Wire(), driver.ErrRecordDecode, err)
	}
	if tslot.Wire() != sl.Wire() {
		return codeplug.Channel{}, &AnswerMismatchError{Model: params.Name, Requested: sl.Wire(), Answered: tslot.Wire()}
	}

	ctcss, ok := ctcssNames[m.CTCSS]
	if !ok {
		// Unreachable after ParseMRAnswer's own validation; refuse rather
		// than silently mislabel if it ever isn't.
		return codeplug.Channel{}, fmt.Errorf("ftx1: ReadChannel %s: unmapped CTCSS state %q", sl.Wire(), m.CTCSS)
	}
	shift, ok := shiftNames[m.Shift]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("ftx1: ReadChannel %s: unmapped shift %q", sl.Wire(), m.Shift)
	}

	return codeplug.Channel{
		Slot: sl.Wire(),
		Data: &codeplug.ChannelData{
			FreqHz: uint64(m.FreqHz),
			// Rendered through THIS session's dialect, not
			// cat.Mode.String: user-visible, so it must be the mode
			// table of the radio that answered.
			Mode:       s.dialect.ModeName(m.Mode),
			ClarHz:     int(m.ClarHz),
			RxClar:     m.RxClar,
			TxClar:     m.TxClar,
			CTCSS:      ctcss,
			CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
			Shift:      shift,
			Tag:        tag,
			TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},

			// The Icom-tier fields (design D4/D8): UNAVAILABLE on this
			// radio — this family's memory frame carries none of them.
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
