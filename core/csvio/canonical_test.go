// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// TestReverseMappingConsultsTheCanonicalEntry is E5's consumer half. The
// CHIRP importer maps a foreign dialect's "+"/"-" onto this radio's own
// vocabulary by asking "which wire code means DOWN?", and until E5 it took
// the FIRST slice entry with that direction — so on a model expressing one
// direction with two codes, which code an imported file produced depended
// on the order the driver author happened to write the table in.
//
// The declaration order in each case below is deliberately the OPPOSITE of
// the canonical marking, so a first-match implementation returns the wrong
// answer rather than the right one by luck.
func TestReverseMappingConsultsTheCanonicalEntry(t *testing.T) {
	t.Run("duplex: the canonical entry wins over the first", func(t *testing.T) {
		caps := spec.Capabilities{DuplexOptions: []spec.DuplexOption{
			{Value: "DUP-AUTO", Direction: spec.DuplexDown},
			{Value: "DUP-", Direction: spec.DuplexDown, Canonical: true},
		}}
		got, ok := duplexFor(caps, spec.DuplexDown)
		if !ok {
			t.Fatal("duplexFor found no option for DuplexDown")
		}
		if got != "DUP-" {
			t.Errorf("duplexFor = %q, want %q — the canonical entry, not the first declared", got, "DUP-")
		}
	})

	t.Run("duplex: a lone entry needs no marking", func(t *testing.T) {
		caps := spec.Capabilities{DuplexOptions: []spec.DuplexOption{
			{Value: "DUP+", Direction: spec.DuplexUp},
		}}
		got, ok := duplexFor(caps, spec.DuplexUp)
		if !ok || got != "DUP+" {
			t.Errorf("duplexFor = %q, %v; want %q, true", got, ok, "DUP+")
		}
	})

	t.Run("duplex: a direction this radio does not express", func(t *testing.T) {
		caps := spec.Capabilities{DuplexOptions: []spec.DuplexOption{
			{Value: "OFF", Direction: spec.DuplexOff},
		}}
		if got, ok := duplexFor(caps, spec.DuplexUp); ok {
			t.Errorf("duplexFor = %q, true; want the not-found answer", got)
		}
	})

	t.Run("duplex: two unmarked entries are refused, not guessed", func(t *testing.T) {
		// spec.Validate refuses this shape outright, so it is reachable
		// only from a hand-built Capabilities that never passed it. The
		// answer must still be "I do not know" rather than a coin toss
		// dressed as a result.
		caps := spec.Capabilities{DuplexOptions: []spec.DuplexOption{
			{Value: "DUP-A", Direction: spec.DuplexDown},
			{Value: "DUP-B", Direction: spec.DuplexDown},
		}}
		if got, ok := duplexFor(caps, spec.DuplexDown); ok {
			t.Errorf("duplexFor = %q, true; want the not-found answer for an ambiguous vocabulary", got)
		}
	})

	t.Run("tone mode: the canonical entry wins over the first", func(t *testing.T) {
		caps := spec.Capabilities{ToneModes: []spec.ToneMode{
			{Value: "TONE-ALT", Semantics: spec.ToneModeCTCSS},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS, Canonical: true},
		}}
		got, ok := toneModeFor(caps, spec.ToneModeCTCSS)
		if !ok {
			t.Fatal("toneModeFor found no mode for ToneModeCTCSS")
		}
		if got != "TONE" {
			t.Errorf("toneModeFor = %q, want %q — the canonical entry, not the first declared", got, "TONE")
		}
	})

	t.Run("tone mode: a lone entry needs no marking", func(t *testing.T) {
		caps := spec.Capabilities{ToneModes: []spec.ToneMode{
			{Value: "TSQL", Semantics: spec.ToneModeCTCSSSquelch},
		}}
		got, ok := toneModeFor(caps, spec.ToneModeCTCSSSquelch)
		if !ok || got != "TSQL" {
			t.Errorf("toneModeFor = %q, %v; want %q, true", got, ok, "TSQL")
		}
	})

	t.Run("tone mode: two unmarked entries are refused, not guessed", func(t *testing.T) {
		caps := spec.Capabilities{ToneModes: []spec.ToneMode{
			{Value: "TONE-A", Semantics: spec.ToneModeCTCSS},
			{Value: "TONE-B", Semantics: spec.ToneModeCTCSS},
		}}
		if got, ok := toneModeFor(caps, spec.ToneModeCTCSS); ok {
			t.Errorf("toneModeFor = %q, true; want the not-found answer for an ambiguous vocabulary", got)
		}
	})
}
