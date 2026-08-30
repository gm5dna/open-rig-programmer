// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ/civtest"
)

// TestConformance runs the landed independent codec corpus against this
// profile. The core/civ allTestProfiles fixture is intentionally not edited:
// its disagreeing receiver remains the codec package's own non-vacuity guard,
// while this model is exercised through the public conformance seam.
func TestConformance(t *testing.T) { civtest.Run(t, Profile()) }
