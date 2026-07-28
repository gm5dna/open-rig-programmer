// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// TestExtableCeilingMatchesDialectBound pins internal/extable's profile
// ceiling to core/cat's own maxEXDigits. The two are declared separately —
// a build-time tool must not import the runtime package it generates into —
// so without this pin they could drift, and a profile would be accepted at
// registry construction only to be refused later by NewDialect's V8 rule.
func TestExtableCeilingMatchesDialectBound(t *testing.T) {
	if extable.MaxDigitsCeiling != maxEXDigits {
		t.Errorf("extable.MaxDigitsCeiling = %d, core/cat maxEXDigits = %d — they must agree",
			extable.MaxDigitsCeiling, maxEXDigits)
	}
}
