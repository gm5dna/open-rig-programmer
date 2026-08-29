// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760_test

import (
	"github.com/gm5dna/open-rig-programmer/core/civ/civtest"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7760"
	"testing"
)

func TestConformance(t *testing.T) { civtest.Run(t, ic7760.Profile()) }
