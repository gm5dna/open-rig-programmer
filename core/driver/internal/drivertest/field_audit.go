// SPDX-License-Identifier: GPL-3.0-or-later

package drivertest

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// AssertFieldAuditCoversEverySpecField pins the two-way set equality every
// driver's field audit must hold against spec.AllFields(): every audited
// field is a real spec.Field and appears at most once, every deliberately
// unexpressed field carries a non-empty reason and is not also audited,
// and the union of the two names every spec.Field exactly — no gap, no
// extra. listName is the caller's identifier for audited (e.g. "allFields",
// "fieldGrid"), used only to make failures point at the right slice.
func AssertFieldAuditCoversEverySpecField(t testing.TB, listName string, audited []spec.Field, unexpressed map[spec.Field]string) {
	t.Helper()
	seen := make(map[spec.Field]bool, len(audited)+len(unexpressed))
	for _, field := range audited {
		if seen[field] {
			t.Errorf("%s lists %s more than once", listName, field)
		}
		seen[field] = true
	}
	for field, reason := range unexpressed {
		if reason == "" {
			t.Errorf("deliberatelyUnexpressedFields[%s] has no reason", field)
		}
		if seen[field] {
			t.Errorf("field %s is both audited and deliberately unexpressed", field)
		}
		seen[field] = true
	}
	for _, field := range spec.AllFields() {
		if !seen[field] {
			t.Errorf("spec.Field %s is neither audited nor deliberately unexpressed", field)
		}
		delete(seen, field)
	}
	for field := range seen {
		t.Errorf("field audit names %s, which spec.AllFields does not", field)
	}
}
