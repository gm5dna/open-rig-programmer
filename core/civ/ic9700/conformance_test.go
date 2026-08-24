// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ/civtest"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic9700"
)

// TestConformance runs core/civ's own suite against this profile.
//
// IT IS RUN, NOT SUMMARISED. civtest.Run walks every builder, every
// accepted record length, the gate's re-encode rule and the refusal
// paths, and it carries its own non-vacuity check — a run that passed
// while exercising nothing is the failure mode this suite exists to
// catch, so read the -v summary rather than the ok line.
func TestConformance(t *testing.T) { civtest.Run(t, ic9700.Profile()) }

// TestZeroValueProfile holds the ZERO profile to the same suite: an
// unconfigured profile must refuse every builder and admit no frame at
// the gate. It takes no argument because the thing under test is the zero
// value itself.
func TestZeroValueProfile(t *testing.T) { civtest.RunZeroValue(t) }
