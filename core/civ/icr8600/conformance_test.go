// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ/civtest"
	"github.com/gm5dna/open-rig-programmer/core/civ/icr8600"
)

// TestConformance holds this model's profile to every property the shared
// CI-V conformance suite states through core/civ's exported API. In
// particular, the suite still includes core/civ's deliberately disagreeing
// mode-keyed test profile; this package adds no always-agreeing bypass.
func TestConformance(t *testing.T) {
	civtest.Run(t, icr8600.Profile())
}
