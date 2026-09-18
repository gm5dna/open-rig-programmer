// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"context"
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/ts2000"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakets2000"
)

// openTS2000SimSession opens a *ts2000.Session (Simulated profile) against
// a fresh internal/fakets2000.Radio, registering cleanup for both — the
// shared fake, not this package's own FT-710-shaped fakeradio, because the
// defect this file guards against (ReadAll aborting on the Satellite
// Memory bank) is specific to this row's SA read.
func openTS2000SimSession(t *testing.T) (*fakets2000.Radio, driver.Session) {
	t.Helper()
	r := fakets2000.New()
	t.Cleanup(func() { _ = r.Close() })

	d := ts2000.NewTS2000(ts2000.WithSimulatedProfile())
	sess, err := d.Open(context.Background(), r.Port(), driver.Identity{Port: "/dev/test"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return r, sess
}

// TestReadAll_TS2000_SkipsSatelliteBank pins the fix for the defect the
// final byte-identity run found: a whole-radio ReadAll on a TS-2000 must
// not abort at the Satellite Memory bank ("SA;" answers only for the
// currently selected channel, never an arbitrary requested slot). ReadAll
// must succeed and its result must carry no Satellite Memory slot at all
// — the bank spec.Bank.CurrentChannelOnly marks unenumerable, not a
// partial or guessed read of it.
func TestReadAll_TS2000_SkipsSatelliteBank(t *testing.T) {
	_, sess := openTS2000SimSession(t)
	svc := NewService(sess, newStore(t))

	cp, err := svc.ReadAll(context.Background())
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(cp.Channels) == 0 {
		t.Fatal("ReadAll returned no channels at all")
	}

	caps := sess.Capabilities()
	var satSlots int
	for _, bank := range caps.Banks {
		if bank.ID == spec.BankSatellite {
			satSlots = len(bank.Slots)
		}
	}
	if satSlots == 0 {
		t.Fatal("test setup: this row's capabilities carry no Satellite Memory bank")
	}

	for _, ch := range cp.Channels {
		if bankID, ok := satBankFor(caps.Banks, ch.Slot); ok && bankID == spec.BankSatellite {
			t.Errorf("ReadAll included slot %q from the Satellite Memory bank, want it skipped entirely", ch.Slot)
		}
	}
}

// satBankFor is this test file's own minimal bank lookup — the same
// membership question codeplug.bankForSlot answers, restated here rather
// than exported from that package for one test's sake.
func satBankFor(banks []spec.Bank, slot string) (spec.BankID, bool) {
	for _, b := range banks {
		if b.WithinSpace(slot) {
			return b.ID, true
		}
	}
	return "", false
}

// TestReadChannel_TS2000_Satellite_SingleSlotUnaffected pins that this
// fix touches ONLY the bulk ReadAll path: a single-slot ReadChannel for
// the currently selected satellite channel still succeeds, and one for
// any other slot still refuses with the honest answer-mismatch error —
// exactly the behaviour core/driver/ts2000/satellite_test.go's own tests
// pin at the driver layer, restated here as this fix's own regression
// guard.
func TestReadChannel_TS2000_Satellite_SingleSlotUnaffected(t *testing.T) {
	_, sess := openTS2000SimSession(t)

	if _, err := sess.ReadChannel(context.Background(), "0"); err != nil {
		t.Errorf("ReadChannel(\"0\") (the fake's default selected channel): %v", err)
	}

	if _, err := sess.ReadChannel(context.Background(), "5"); !errors.Is(err, driver.ErrAnswerMismatch) {
		t.Errorf("ReadChannel(\"5\") = %v, want errors.Is(_, driver.ErrAnswerMismatch)", err)
	}
}
