// SPDX-License-Identifier: GPL-3.0-or-later

package ic7800

import (
	"context"
	"fmt"

	civic7800 "github.com/gm5dna/open-rig-programmer/core/civ/ic7800"
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
	case e.Offset == civic7800.SelectNibbleOffset && e.Nibble == "low":
		what = "the SELECT-group marker (0=OFF, 1=★1, 2=★2, 3=★3)"
	case e.Offset == civic7800.DataModeNibbleOffset && e.Nibble == "low":
		what = "the data mode (0=OFF, 1=DATA 1, 2=DATA 2, 3=DATA 3)"
	}
	return fmt.Sprintf(
		"ic7800: this slot's record byte %d carries %#x in its %s nibble where the profile's Fixed template carries %#x — %s. Ruling E6: a slot may be written ONLY when its unmapped regions equal the template, and anything else is REFUSED with the reason named, never rewritten. This channel cannot be written by this programme at all; it is not downgraded and not cleared",
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

// OutOfDomainError reports a Known numeric value — asked for on a write,
// or decoded on a read — that this radio's record cannot express.
//
// DEFENCE IN DEPTH, AND NOT THE GATE — see doc.go §6. civ.FieldSpan has no
// numeric domain, so civ.Profile.AllowedCommand would ADMIT a set carrying
// 70 MHz. codeplug.Validate already bounds the primary frequency and
// enabler E3 bounds tones, so every path through the model layer is
// covered; these refusals close the driver's own door as well, and
// TestNumericRefusalIsDefenceInDepthNotTheGate asserts the gap that
// remains so that closing it later is a visible test change.
// Fields are driver.OutOfDomainError's shared shape
// (core/driver/errors.go); Min and Where are left unset — this package
// only ever had a ceiling.
type OutOfDomainError struct {
	driver.OutOfDomainError
}

// Error's wording holds at both call sites: WriteChannel raises this
// before a frame is built, and ReadChannel raises it after decoding one,
// where there is no outbound frame at all — see
// TestReadChannel_RefusesFrequencyAboveCeiling. So the gate reference
// below states what civ.FieldSpan can never check, not what a specific
// frame would do.
func (e *OutOfDomainError) Error() string {
	return fmt.Sprintf(
		"ic7800: %s = %d is outside what this radio's record can encode (maximum %d) — refused by the driver. This is a DRIVER-level refusal, not the outbound gate: civ.FieldSpan carries no numeric domain, so the gate offers no such check (see doc.go, the deferred gate-domain gap)",
		e.Field, e.Value, e.Max)
}

// Unwrap lets errors.Is(err, ErrOutOfDomain) match.
func (e *OutOfDomainError) Unwrap() error { return ErrOutOfDomain }

// domainRefusal applies WriteChannel's rung 4 to one channel. Params.Domain
// names it too, so a read refuses a frequency the write would refuse. The
// capability set is unused: this radio's bounds are the package constants.
func domainRefusal(d codeplug.ChannelData, _ spec.Capabilities) error {
	return icom.CeilingRefusal(d, MaxEncodableFreqHz, MaxToneDeciHz, func(e driver.OutOfDomainError) error { return &OutOfDomainError{e} })
}

// WriteChannel implements driver.Session: the shared write ladder in
// core/driver/internal/icom, run against this radio's own Params.
func (s *Session) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	return icom.WriteChannel(ctx, &params, s.eng, s.caps, &s.answerMismatches, ch)
}
