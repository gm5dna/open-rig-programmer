// SPDX-License-Identifier: GPL-3.0-or-later

package icomtest

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
)

// TestIcomtestCitesNothing holds the shared suites to generic prose: each
// package supplies its own citations.
func TestIcomtestCitesNothing(t *testing.T) {
	for _, c := range drivertest.ScanCitations(t, []string{"."}, nil) {
		t.Errorf("%s:%d cites %q", c.File, c.Line, c.Token)
	}
}
