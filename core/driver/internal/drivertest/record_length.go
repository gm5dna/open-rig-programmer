// SPDX-License-Identifier: GPL-3.0-or-later

package drivertest

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// AssertRecordLengthMismatch pins the common probe-refusal contract: callers
// can classify the radio and recover the codec's measured lengths, while the
// driver's user-facing text remains exact. Three of the four driver probe
// tests (ic7610, ic7760, ic7851) supply outerGot and outerWant from their
// own exported mismatch error's Got/Want fields; ic9700's mismatch type has
// no such fields of its own, so its test passes its wrapped
// *civ.RecordLengthError's Got/Want instead — a tautological comparison
// there, since that is also what errors.As(err, &lengthErr) below recovers.
func AssertRecordLengthMismatch(t testing.TB, err error, outerGot, outerWant int, wantText string) {
	t.Helper()
	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Errorf("errors.Is(err, driver.ErrWrongRadio) = false for %v", err)
	}
	var lengthErr *civ.RecordLengthError
	if !errors.As(err, &lengthErr) {
		t.Fatalf("errors.As(err, *civ.RecordLengthError) = false for %v", err)
	}
	if lengthErr.Got != outerGot || len(lengthErr.Want) != 1 || lengthErr.Want[0] != outerWant {
		t.Errorf("civ.RecordLengthError = Got %d/Want %v, want outer error's %d/%d", lengthErr.Got, lengthErr.Want, outerGot, outerWant)
	}
	if got := err.Error(); got != wantText {
		t.Errorf("Error() = %q, want pinned text %q", got, wantText)
	}
}
