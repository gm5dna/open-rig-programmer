// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300mk2_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ/civtest"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7300mk2"
)

// TestConformance holds this model's profile to every property the shared
// CI-V conformance suite can state through core/civ's exported API.
//
// civtest.RunZeroValue is NOT re-run per model: it is universal and owned
// by the codec package.
//
// NOTHING IN core/civ MAY BE EDITED TO MAKE THIS MODEL PASS. If the suite
// fails here, the profile is wrong, not the suite.
func TestConformance(t *testing.T) {
	civtest.Run(t, ic7300mk2.Profile())
}
