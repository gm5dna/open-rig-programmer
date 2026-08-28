// SPDX-License-Identifier: GPL-3.0-or-later

package ic905_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ/civtest"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic905"
)

// civtest.Run drives this profile through the whole shared corpus:
// every builder's output admitted by its own gate with per-builder
// contribution counting, round trips through the parser, the seventeen
// refusal frames, the mutated-unmapped-nibble refusal that the Fixed
// template makes reachable, and — the check that matters most here —
// checkEveryAcceptedLength, which packs a record at BOTH 64 and 65 with
// civtest's OWN second encoder and gates it. That second encoder is
// written from the wire convention rather than borrowed from civ, so
// agreement is evidence rather than a tautology.
func TestConformance(t *testing.T) { civtest.Run(t, ic905.Profile()) }

// The zero-value profile builds nothing, parses nothing and admits
// nothing. Run refuses to run over it (the vacuity trap), so the
// inertness is checked by its own entry point.
func TestConformanceZeroValue(t *testing.T) { civtest.RunZeroValue(t) }
