// SPDX-License-Identifier: GPL-3.0-or-later

package icom

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
)

// TestEngineCitesNothing holds the engine to generic prose: every page,
// folio and register citation belongs to the package whose radio it is
// about.
func TestEngineCitesNothing(t *testing.T) {
	for _, c := range drivertest.ScanCitations(t, []string{"."}, nil) {
		t.Errorf("%s:%d cites %q", c.File, c.Line, c.Token)
	}
}
