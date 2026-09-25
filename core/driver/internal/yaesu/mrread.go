// SPDX-License-Identifier: GPL-3.0-or-later

package yaesu

import (
	"context"
	"errors"
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// MRParams is one radio's configuration of the shared MR-read/MW-write
// bodies (ReadChannel, MRWriteChannel, BuildMWCommand) — a second
// Params-style value in this package, distinct from Params (the 4-rig MT
// combined-record family's own configuration). The two field sets barely
// overlap: a merged struct would drag Probe/MTRetries/DescriptorVersion
// into every MW-only driver for nothing.
//
// AN UNSET FIELD MEANS "NOT THIS RADIO", same convention as Params. Every
// field is now read by ReadChannel/MRWriteChannel/BuildMWCommand below —
// ExplicitTagRefusal and RequestConditionalTagFields (ftdx5000, the last
// migration) were the final two.
type MRParams struct {
	// Name is the error prefix every message this package mints carries,
	// and the Model KindMismatchError names, e.g. "ftdx1200".
	Name string
	// Model overrides AnswerMismatchError's Model when set; the zero
	// value uses the session's own dialect.CATID() (every driver in
	// scope today).
	Model string
	// MRAnswerLen is the exact MR answer length in bytes (27 for all 7
	// in-scope drivers).
	MRAnswerLen int
	// CTCSS is the CTCSS state vocabulary this radio's read/write paths
	// accept, IN LEGEND ORDER — matches Params.CTCSS's own convention.
	CTCSS []CTCSSName
	// AcceptedKinds is the P7 kind-byte domain an MR answer may carry.
	// nil means no kind check at all (ftdx5000, ftdx9000); a non-nil
	// slice is checked by ReadChannel.
	AcceptedKinds []byte
	// ToneRead selects this radio's CTCSSTone read behaviour.
	ToneRead ToneReadMode
	// ToneWrite selects this radio's CTCSSTone write behaviour.
	ToneWrite ToneWriteMode
	// WriteKind supplies the MW Set frame's P7 kind byte. Every driver in
	// scope today passes dialect.MWWriteKind(); ftdx5000 will instead
	// pass a func returning the hardcoded cat.KindVFO (Q2).
	WriteKind func(cat.Dialect) byte
	// EraseReason is the refusal text for a write of an empty channel.
	EraseReason string
	// ExplicitTagRefusal — ftdx5000 only. When true, BuildMWCommand refuses
	// a non-empty Tag / Known TagDisplay itself, with model-specific
	// wording, ahead of its other value-level checks.
	ExplicitTagRefusal bool
	// SkipTierFields — ftdx9000 only. When true, mrRequestedFields skips
	// the TierRequestedFields loop entirely: this radio's 27-byte record
	// has no room for any of the 17 Icom-tier fields, and its fixed
	// request list never asked after them (spec-v2 finding 9) — a Known
	// one is accepted and silently dropped, not refused.
	SkipTierFields bool
	// RequestConditionalTagFields — ftdx5000 only. When true,
	// mrRequestedFields appends Tag if non-empty, TagDisplay if Known,
	// ScanSkip if Known (no other driver ever requests any of the three).
	RequestConditionalTagFields bool
	// ScanSkipUnavailable — ftdx5000, ftdx9000 only. When true,
	// ReadChannel reports ScanSkip as Unavailable rather than Unknown:
	// the record has no scan-skip byte at all, a positive statement, not
	// an open question.
	ScanSkipUnavailable bool
}

// ToneReadMode selects a radio's CTCSSTone read behaviour.
type ToneReadMode int

const (
	// ToneUnavailable: P9's read side is printed-fixed, so there is no
	// live tone state to report (ftdx1200).
	ToneUnavailable ToneReadMode = iota
	// ToneLiveKnown: P9's read side is a live two-digit index into the
	// radio's own CTCSS tone chart (every other in-scope driver).
	ToneLiveKnown
	// ToneUnknown: this radio's CAT protocol has no command that reads a
	// memory channel's live tone-table index at all — Unknown, not
	// Unavailable, because a write path downstream still means "preserve
	// whatever the radio has" by it (ftx1: MTFormShortNoDisplay's record
	// has no such field, spec.md §3.1).
	ToneUnknown
)

// ToneWriteMode selects a radio's CTCSSTone write behaviour.
type ToneWriteMode int

const (
	// ToneNeverRequested: P9 is printed-fixed on write too, so a write
	// never asks for FieldCTCSSTone at all (ftdx1200).
	ToneNeverRequested ToneWriteMode = iota
	// ToneOptionalIfKnown: a write requests FieldCTCSSTone only when the
	// channel's tone is Known; whether that request then succeeds is the
	// per-driver capability table's call, not this package's (ft2000,
	// ft450d, ft950, ftdx3000).
	ToneOptionalIfKnown
	// ToneRequiredKnown: a write requires the tone Known and refuses
	// otherwise (ftdx5000, ftdx9000).
	ToneRequiredKnown
)

// mrKindAccepted reports whether got is one of accepted's P7 kind bytes.
func mrKindAccepted(accepted []byte, got byte) bool {
	for _, k := range accepted {
		if k == got {
			return true
		}
	}
	return false
}

// ctcssNameFor looks up state's read-direction display name in vocab —
// CTCSSMap's/CTCSSState's inverse, needed because an MR answer carries
// the state and ReadChannel must render the name a ChannelData shows.
func ctcssNameFor(vocab []CTCSSName, state cat.CTCSSState) (string, bool) {
	for _, c := range vocab {
		if c.State == state {
			return c.Name, true
		}
	}
	return "", false
}

// shiftNames is the read-direction inverse of ShiftByName. THE SAME THREE
// NAMES ON EVERY ONE OF THESE RADIOS (matrix-confirmed across all 7), so
// it is one table here rather than a per-driver copy.
var shiftNames = map[cat.Shift]string{
	cat.ShiftSimplex: "SIMPLEX",
	cat.ShiftPlus:    "PLUS",
	cat.ShiftMinus:   "MINUS",
}

// ReadChannel implements the single-MR-read body shared by the in-scope
// MW/MR-family drivers (ftx1 stays separate — two-frame MR+MT). There is
// no MT to sequence beside the read, so Tag stays "" and TagDisplay stays
// Unavailable; ScanSkip stays Unknown, UNLESS ScanSkipUnavailable is set
// (ftdx5000, ftdx9000: the 27-byte record has no scan-skip byte at all —
// a positive statement, not an open question). The caller holds its own
// operation mutex around this call; nothing here takes a lock, matching
// MRWriteChannel and the MT family's WriteChannel.
func ReadChannel(ctx context.Context, eng *transport.Engine, dialect cat.Dialect, caps spec.Capabilities, p *MRParams, slot string) (codeplug.Channel, error) {
	sl, err := dialect.ParseSlot(slot)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("%s: ReadChannel: %w", p.Name, err)
	}

	cmd, err := dialect.BuildMRRead(sl)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("%s: ReadChannel: %w", p.Name, err)
	}

	frame, err := eng.Do(ctx, cmd, transport.CATReadSpec("MR", p.MRAnswerLen, 1))
	if errors.Is(err, cat.ErrRejected) {
		return codeplug.Channel{Slot: sl.Wire()}, nil
	}
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("%s: ReadChannel %s: MR: %w", p.Name, sl.Wire(), err)
	}

	m, err := dialect.ParseMRAnswer(frame)
	if err != nil {
		return codeplug.Channel{}, fmt.Errorf("%s: ReadChannel %s: %w: %w", p.Name, sl.Wire(), driver.ErrRecordDecode, err)
	}
	if m.Slot.Wire() != sl.Wire() {
		model := p.Model
		if model == "" {
			model = dialect.CATID()
		}
		return codeplug.Channel{}, &driver.AnswerMismatchError[string]{Model: model, Requested: sl.Wire(), Answered: m.Slot.Wire()}
	}
	if p.AcceptedKinds != nil && !mrKindAccepted(p.AcceptedKinds, m.Kind) {
		return codeplug.Channel{}, &KindMismatchError{Model: p.Name, Slot: sl.Wire(), Got: m.Kind, Want: p.AcceptedKinds}
	}

	ctcss, ok := ctcssNameFor(p.CTCSS, m.CTCSS)
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("%s: ReadChannel %s: unmapped CTCSS state %q", p.Name, sl.Wire(), m.CTCSS)
	}
	shift, ok := shiftNames[m.Shift]
	if !ok {
		return codeplug.Channel{}, fmt.Errorf("%s: ReadChannel %s: unmapped shift %q", p.Name, sl.Wire(), m.Shift)
	}

	scanSkip := codeplug.Unknown
	if p.ScanSkipUnavailable {
		scanSkip = codeplug.Unavailable
	}

	var tone codeplug.ToneField
	switch p.ToneRead {
	case ToneLiveKnown:
		if int(m.ToneIndex) >= len(caps.CTCSSTones) {
			// Unreachable after cat.ParseMRAnswer's own 0-49 bound
			// (memdata.go), narrower than CTCSSTones' 50 entries; refuse
			// rather than silently mislabel if it ever isn't.
			return codeplug.Channel{}, fmt.Errorf("%s: ReadChannel %s: tone index %d out of range", p.Name, sl.Wire(), m.ToneIndex)
		}
		tone = codeplug.ToneField{State: codeplug.Known, Value: caps.CTCSSTones[m.ToneIndex]}
	case ToneUnknown:
		tone = codeplug.ToneField{State: codeplug.Unknown}
	default: // ToneUnavailable
		tone = codeplug.ToneField{State: codeplug.Unavailable}
	}

	return codeplug.Channel{
		Slot: sl.Wire(),
		Data: &codeplug.ChannelData{
			FreqHz:     uint64(m.FreqHz),
			Mode:       dialect.ModeName(m.Mode),
			ClarHz:     int(m.ClarHz),
			RxClar:     m.RxClar,
			TxClar:     m.TxClar,
			CTCSS:      ctcss,
			CTCSSTone:  tone,
			Shift:      shift,
			Tag:        "",
			TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			ScanSkip:   codeplug.BoolField{State: scanSkip},

			// The Icom-tier fields: UNAVAILABLE on every one of these
			// radios.
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
