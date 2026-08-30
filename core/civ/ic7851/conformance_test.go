// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851_test

import (
	"github.com/gm5dna/open-rig-programmer/core/civ/civtest"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
	"testing"
)

func TestConformance(t *testing.T)          { civtest.Run(t, ic7851.Profile()) }
func TestConformanceZeroValue(t *testing.T) { civtest.RunZeroValue(t) }
