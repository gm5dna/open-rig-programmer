// SPDX-License-Identifier: GPL-3.0-or-later

package ic7700

import (
	"context"
	"fmt"

	civic7700 "github.com/gm5dna/open-rig-programmer/core/civ/ic7700"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/icom"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// ErrUnmappedRegion is the sentinel for ruling E6: the slot's unmapped
// record regions differ from the profile's Fixed template.
var ErrUnmappedRegion = driver.ErrUnmappedRegion

// UnmappedRegionError reports E6's refusal, naming exactly which region
// disagreed and by how much.
//
// RULING E6, VERBATIM: a driver may write a slot ONLY when its unmapped
// regions equal the profile's Fixed template; anything else is REFUSED
// with the reason named, NEVER REWRITTEN.
//
// THE COST, stated as E6 requires: A CHANNEL THAT IS IN A SELECT GROUP
// (★1/★2/★3), OR WHOSE DATA MODE IS DATA 1/2/3, CANNOT BE WRITTEN BY THIS
// PROGRAMME AT ALL. It is never silently downgraded to ★1/DATA 1 and never
// silently cleared to OFF. The encoder always writes the template, so
// "preserve what was there" is not available to it — that allow-case was
// STRUCK from E6 REV 2 as unimplementable and as licensing the very
// corruption the ruling forbids.
// Fields are driver.UnmappedRegionError's shared shape
// (core/driver/errors.go); Nibble is "low" or "high" — which half of the
// byte.
type UnmappedRegionError struct {
	driver.UnmappedRegionError
}

func (e *UnmappedRegionError) Error() string {
	what := "an unmapped region"
	switch {
	case e.Offset == civic7700.SplitSelectByteOffset:
		what = "the split flag / SELECT-group marker byte (hi nibble: 0=Split OFF/1=Split ON; lo nibble: 0=OFF, 1=★1, 2=★2, 3=★3)"
	case e.Offset == civic7700.DataModeNibbleOffset && e.Nibble == "high":
		what = "the data mode (0=OFF, 1=DATA 1, 2=DATA 2, 3=DATA 3)"
	case e.Offset >= civic7700.TXDupUnmappedOffset && e.Offset < civic7700.TXDupUnmappedOffset+civic7700.TXDupUnmappedLength:
		what = "the TX-duplicate block's own mode/filter/tone byte, which has no neutral field (\"second TX-side copy, no neutral field\")"
	}
	return fmt.Sprintf(
		"ic7700: this slot's record byte %d carries %#x in its %s nibble where the profile's Fixed template carries %#x — %s. Ruling E6: a slot may be written ONLY when its unmapped regions equal the template, and anything else is REFUSED with the reason named, never rewritten. This channel cannot be written by this programme at all; it is not downgraded and not cleared",
		e.Offset, e.Got, e.Nibble, e.Want, what)
}

// Unwrap lets errors.Is(err, ErrUnmappedRegion) match.
//
// DELIBERATELY NOT driver.ErrWriteRefused. That sentinel's contract says
// the channel was refused BEFORE ANY WIRE TRAFFIC, and this refusal is
// tier ruling T5's ONE RECORDED EXCEPTION: it necessarily follows the
// single read that obtained the slot's current state. A caller keying on
// ErrWriteRefused and inferring "nothing went out" would be wrong here by
// exactly one read exchange, so this refusal carries its own sentinel and
// says why.
func (e *UnmappedRegionError) Unwrap() error { return ErrUnmappedRegion }

// ErrOutOfDomain is the sentinel for a Known value outside what this
// radio's record can encode.
var ErrOutOfDomain = driver.ErrOutOfDomain

// OutOfDomainError reports a Known numeric value outside the domain THIS
// SESSION'S CAPABILITIES declare.
//
// THE BOUNDS ARE THE CAPABILITIES', NOT THE FIELD WIDTH'S, and that is the
// whole point of the type. On the FREQUENCY they COINCIDE on this model,
// deliberately: MinFreqHz/MaxFreqHz are set to the RX/TX spans' own
// encoding bounds exactly (matrix §1 rows 15-16's CHOICE, because this
// radio's own STORABLE floor/ceiling is unprinted and stays ASSUMED), so
// this refusal is unreachable through FreqHz alone — it exists for the day
// a tighter, hardware-verified bound replaces the encoding one. On the
// TONE they genuinely DIFFER: the span's four BCD digits can encode
// 000.0-299.9 Hz, while caps.go declares the narrower 000.0-254.9 Hz the
// matrix's own reading states (matrix §1 rows 11-12) — so a tone between
// those two ceilings is exactly what this refusal catches.
//
// DEFENCE IN DEPTH, AND NOT THE GATE — see doc.go. civ.FieldSpan has no
// numeric domain, so civ.Profile.AllowedCommand would ADMIT a set carrying
// a 260.0 Hz tone. codeplug.Validate already bounds the primary frequency
// against the same capability numbers and enabler E3 bounds tones, so
// every path through the model layer is covered; this refusal closes the
// driver's own door as well.
// Fields are driver.OutOfDomainError's shared shape
// (core/driver/errors.go).
type OutOfDomainError struct {
	driver.OutOfDomainError
}

// Error's wording holds at both call sites: WriteChannel raises this
// before a frame is built, and ReadChannel raises it after decoding one
// (via domainRefusal), where there is no outbound frame at all — see
// TestReadChannel_RefusesFrequencyOutsideRadioDomain. So the gate
// reference below states what civ.FieldSpan can never check, not what a
// specific frame would do.
func (e *OutOfDomainError) Error() string {
	return fmt.Sprintf(
		"ic7700: %s = %d is outside this radio's declared domain %d..%d (%s) — refused by the driver. This is a DRIVER-level refusal, not the outbound gate: civ.FieldSpan carries no numeric domain, so the gate offers no such check (see doc.go, the deferred gate-domain gap)",
		e.Field, e.Value, e.Min, e.Max, e.Where)
}

// Unwrap lets errors.Is(err, ErrOutOfDomain) match.
func (e *OutOfDomainError) Unwrap() error { return ErrOutOfDomain }

// domainRefusal applies WriteChannel's rung 4 to one channel, against one
// capability set.
//
// SPLIT OUT OF THE LADDER so that a test can ask "is this value inside the
// declared domain?" without needing a session and an engine, and so that
// the answer a test gets is the same code path a write takes.
//
// The frequency arm reads caps.MinFreqHz/MaxFreqHz and the tone arms
// caps.CTCSSToneRange, so the numbers a UI offers, the numbers
// codeplug.Validate judges against and the numbers this rung enforces are
// ONE declaration. A missing tone range refuses every Known tone rather
// than admitting one: a radio that declares no domain has authorised
// nothing.
func domainRefusal(d codeplug.ChannelData, caps spec.Capabilities) error {
	if d.FreqHz < caps.MinFreqHz || d.FreqHz > caps.MaxFreqHz {
		return &OutOfDomainError{driver.OutOfDomainError{
			Field: spec.FieldFrequency, Value: d.FreqHz,
			Min: caps.MinFreqHz, Max: caps.MaxFreqHz,
			Where: "spec.Capabilities.MinFreqHz/MaxFreqHz, the RX/TX frequency spans' own encoding bounds",
		}}
	}
	// tx_frequency shares the same bounds as frequency: idx15-19 is the
	// SAME bcd_packed codec as idx1-5 (matrix §3.15). Only checked when
	// the caller supplied a value — an absent one is mirrored from FreqHz
	// at rung 8, which rung 4 has already bounded above.
	if d.TxFreqHz.State == codeplug.Known {
		v := d.TxFreqHz.Value
		if v < caps.MinFreqHz || v > caps.MaxFreqHz {
			return &OutOfDomainError{driver.OutOfDomainError{
				Field: spec.FieldTxFrequency, Value: v,
				Min: caps.MinFreqHz, Max: caps.MaxFreqHz,
				Where: "spec.Capabilities.MinFreqHz/MaxFreqHz, the RX/TX frequency spans' own encoding bounds",
			}}
		}
	}
	r := caps.CTCSSToneRange
	for _, tt := range []struct {
		field spec.Field
		tone  codeplug.ToneField
	}{
		{spec.FieldToneTx, d.ToneTx},
		{spec.FieldToneRx, d.ToneRx},
	} {
		if tt.tone.State != codeplug.Known {
			continue
		}
		v := uint64(tt.tone.Value)
		if r == nil {
			return &OutOfDomainError{driver.OutOfDomainError{
				Field: tt.field, Value: v,
				Where: "this session declares no CTCSSToneRange at all, so no tone is authorised",
			}}
		}
		if v < uint64(r.MinDeciHz) || v > uint64(r.MaxDeciHz) {
			return &OutOfDomainError{driver.OutOfDomainError{
				Field: tt.field, Value: v,
				Min: uint64(r.MinDeciHz), Max: uint64(r.MaxDeciHz),
				Where: "spec.Capabilities.CTCSSToneRange, the tone span's BCD capacity (000.0-299.9 Hz)",
			}}
		}
	}
	return nil
}

// WriteChannel implements driver.Session: the shared write ladder in
// core/driver/internal/icom, run against this radio's own Params.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	return icom.WriteChannel(ctx, &params, s.eng, s.caps, &s.answerMismatches, ch)
}
