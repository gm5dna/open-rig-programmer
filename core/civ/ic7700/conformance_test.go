// SPDX-License-Identifier: GPL-3.0-or-later

package ic7700_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ/civtest"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7700"
)

// TestConformance runs the shared civ.Profile conformance suite: frame
// self-consistency, memory read/set round trips, the length
// discriminator, the outbound gate's refusal of a mutated unmapped byte
// (ruling E6 — this profile's idx0, idx8 high nibble and idx20-28 all fall
// out of this generically) and the profile's own non-vacuity.
func TestConformance(t *testing.T) {
	civtest.Run(t, ic7700.Profile())
}
