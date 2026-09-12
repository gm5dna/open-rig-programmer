// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ/civtest"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7200"
)

func TestConformance(t *testing.T)          { civtest.Run(t, ic7200.Profile()) }
func TestConformanceZeroValue(t *testing.T) { civtest.RunZeroValue(t) }
