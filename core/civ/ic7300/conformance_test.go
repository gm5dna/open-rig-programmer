// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ/civtest"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7300"
)

// TestConformance holds this model's profile to every property the shared
// CI-V conformance suite can state through core/civ's exported API.
//
// civtest.RunZeroValue is NOT called here. It is universal, it is owned by
// the codec package, and re-running it per model would say nothing about
// this model — core/cat/ftdx10/dialect_test.go:42 makes the same point for
// the CAT suite.
//
// NOTHING IN core/civ MAY BE EDITED TO MAKE THIS MODEL PASS. If the suite
// fails here, the profile is wrong, not the suite.
func TestConformance(t *testing.T) {
	civtest.Run(t, ic7300.Profile())
}
