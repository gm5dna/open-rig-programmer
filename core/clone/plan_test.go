// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// TestPrepareSend_SnapshotExistsLoadsDigestMatches: obligation 9 — the
// snapshot PrepareSend saves exists at SnapshotPath(), loads back as a
// valid codeplug, and its BaselineDigest matches the plan's own
// BaselineDigest().
func TestPrepareSend_SnapshotExistsLoadsDigestMatches(t *testing.T) {
	_, sess := openSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	store := newStore(t)
	svc := NewService(sess, store, WithNow(func() time.Time { return fixedNow }))

	baseline, err := svc.ReadAll(testCtx(t))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	file := withChannel(baseline, "010", writableChannel("010", 14_250_000, "TEST").Data)

	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: unexpected error: %v", err)
	}

	if plan.SnapshotPath() == "" {
		t.Fatal("SnapshotPath() is empty")
	}
	loaded, err := codeplug.Load(plan.SnapshotPath())
	if err != nil {
		t.Fatalf("codeplug.Load(%s): %v", plan.SnapshotPath(), err)
	}
	if loaded.Radio.BaselineDigest != plan.BaselineDigest() {
		t.Errorf("snapshot's BaselineDigest = %q, want plan's BaselineDigest() %q", loaded.Radio.BaselineDigest, plan.BaselineDigest())
	}
	if plan.BaselineDigest() != codeplug.Digest(loaded.Channels) {
		t.Errorf("plan.BaselineDigest() = %q, want Digest of the snapshot's own Channels %q", plan.BaselineDigest(), codeplug.Digest(loaded.Channels))
	}
}

// TestPrepareSend_SnapshotSavedEvenOnValidationFailure: obligation 9 says
// "before returning a plan" — the snapshot must still exist even when
// Validate goes on to refuse the candidate, since it happens first in
// PrepareSend's fixed order.
func TestPrepareSend_SnapshotSavedEvenOnValidationFailure(t *testing.T) {
	_, sess := openSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	store := newStore(t)
	svc := NewService(sess, store, WithNow(func() time.Time { return fixedNow }))

	baseline, err := svc.ReadAll(testCtx(t))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	bad := writableChannel("010", 14_250_000, "TEST")
	bad.Data.Mode = "NOT-A-MODE"
	file := withChannel(baseline, "010", bad.Data)

	_, err = svc.PrepareSend(testCtx(t), file)
	var vfe *ValidationFailedError
	if !errors.As(err, &vfe) {
		t.Fatalf("PrepareSend = %v, want a *ValidationFailedError", err)
	}

	entries, direrr := readDir(t, store.Dir)
	if direrr != nil {
		t.Fatalf("reading store dir: %v", direrr)
	}
	found := false
	for _, name := range entries {
		if hasSuffixOrp(name) {
			found = true
		}
	}
	if !found {
		t.Errorf("no *.orp.json snapshot found in %s after a validation failure, want one saved before Validate ran", store.Dir)
	}
}

// TestPrepareSend_ValidationFailureCarriesIssues: doctoring the candidate
// with an out-of-range value must refuse with a *ValidationFailedError
// naming that issue.
func TestPrepareSend_ValidationFailureCarriesIssues(t *testing.T) {
	_, sess := openSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	svc := NewService(sess, newStore(t), WithNow(func() time.Time { return fixedNow }))

	baseline, err := svc.ReadAll(testCtx(t))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	bad := writableChannel("010", 14_250_000, "TEST")
	bad.Data.FreqHz = 999_000_000 // above the FT-710's 75 MHz RX ceiling
	file := withChannel(baseline, "010", bad.Data)

	_, err = svc.PrepareSend(testCtx(t), file)
	var vfe *ValidationFailedError
	if !errors.As(err, &vfe) {
		t.Fatalf("PrepareSend = %v, want a *ValidationFailedError", err)
	}
	if !errors.Is(err, ErrValidationFailed) {
		t.Errorf("errors.Is(err, ErrValidationFailed) = false, want true")
	}
	found := false
	for _, issue := range vfe.Issues {
		if issue.Slot == "010" && issue.Severity == codeplug.SeverityError {
			found = true
		}
	}
	if !found {
		t.Errorf("ValidationFailedError.Issues = %+v, want an error-severity issue for slot \"010\"", vfe.Issues)
	}
}

// TestPrepareSend_DiffCorrectness: a candidate that modifies one channel,
// adds another, and leaves the rest untouched produces a DiffResult with
// exactly those counts, against the FRESH baseline (obligation 1).
func TestPrepareSend_DiffCorrectness(t *testing.T) {
	_, sess := openSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	svc := NewService(sess, newStore(t), WithNow(func() time.Time { return fixedNow }))

	baseline, err := svc.ReadAll(testCtx(t))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	file := withChannel(baseline, "001", writableChannel("001", 14_100_000, "MODIFIED").Data) // "001" is populated -> Modified
	file = withChannel(file, "002", writableChannel("002", 14_200_000, "ADDED").Data)         // "002" is empty -> Added

	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}
	diff := plan.Diff()
	if diff.Modified != 1 {
		t.Errorf("diff.Modified = %d, want 1", diff.Modified)
	}
	if diff.Added != 1 {
		t.Errorf("diff.Added = %d, want 1", diff.Added)
	}
	wantUnchanged := len(baseline.Channels) - 2
	if diff.Unchanged != wantUnchanged {
		t.Errorf("diff.Unchanged = %d, want %d", diff.Unchanged, wantUnchanged)
	}
	if diff.Erased != 0 {
		t.Errorf("diff.Erased = %d, want 0", diff.Erased)
	}
}

// TestSendPlan_AccessorsReturnDefensiveCopies: mutating what Diff()/
// Issues() return must never be observable through a second call — the
// plan's own private state must stay untouched.
func TestSendPlan_AccessorsReturnDefensiveCopies(t *testing.T) {
	_, sess := openSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	svc := NewService(sess, newStore(t), WithNow(func() time.Time { return fixedNow }))

	baseline, err := svc.ReadAll(testCtx(t))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	file := withChannel(baseline, "001", writableChannel("001", 14_100_000, "MODIFIED").Data)

	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	d1 := plan.Diff()
	if len(d1.Entries) == 0 {
		t.Fatal("Diff().Entries is empty")
	}
	// Mutate the returned copy's first entry as aggressively as possible.
	d1.Entries[0].Slot = "TAMPERED"
	if d1.Entries[0].After != nil {
		d1.Entries[0].After.FreqHz = 999
	}
	d1.Added = 999999

	d2 := plan.Diff()
	if d2.Entries[0].Slot == "TAMPERED" {
		t.Error("mutating a Diff() copy's Entries leaked into a later Diff() call")
	}
	if d2.Added == 999999 {
		t.Error("mutating a Diff() copy's Added count leaked into a later Diff() call")
	}

	issues1 := plan.Issues()
	issues1 = append(issues1, codeplug.Issue{Msg: "injected"})
	issues2 := plan.Issues()
	for _, i := range issues2 {
		if i.Msg == "injected" {
			t.Error("mutating an Issues() copy leaked into a later Issues() call")
		}
	}
}

// TestPrepareSend_ContextCancelled: a context cancelled before PrepareSend
// starts is refused immediately.
func TestPrepareSend_ContextCancelled(t *testing.T) {
	_, sess := openSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	svc := NewService(sess, newStore(t))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := svc.PrepareSend(ctx, &codeplug.Codeplug{}); err == nil {
		t.Error("PrepareSend with an already-cancelled context = nil error, want an error")
	}
}

// TestPrepareSend_RealHardwareProfile_RealShapePMS_MemOnlyEdit is Codex
// M5b fix wave, Fix 3 (adjudicated HIGH, the adjudicated real-shape
// test): the RealHardware profile's PMS bank must not require every one
// of its 18 slots to stay populated. M5a's characterised radio began
// with all 18 PMS slots empty, and M5b created only P1L, leaving the
// rest untouched (docs/hardware-notes.md) — "real radios ship
// all-PMS-empty". Before this fix, PMS carried NoBlank:true in every
// profile, so codeplug.Validate rejected EVERY real-derived candidate
// containing an empty PMS slot before Diff ever ran, even a MEM-only
// edit that never touches PMS at all — the newly-armed real-radio write
// path was nonfunctional against the very radio that supplied its
// evidence. This test uses the OBSERVED shape directly against the
// RealHardware profile: PMS mostly empty, P1L alone populated, and a
// tag-only edit on "001" — PrepareSend must reach a writable plan (a
// real, non-Blocked diff entry), not a *ValidationFailedError.
func TestPrepareSend_RealHardwareProfile_RealShapePMS_MemOnlyEdit(t *testing.T) {
	_, sess := openRealHardwareSession(t,
		fakeradio.WithFactoryImage(minimalFactoryImage),
		fakeradio.WithSlot("P1L", fakeradio.MemState{
			Freq: "007100000", ClarSign: '+', ClarMag: "0000",
			Mode: '1', Kind: '1', CTCSS: '0', Shift: '0',
			Populated: true,
		}),
	)
	svc := NewService(sess, newStore(t), WithNow(stepClock(fixedNow)))

	baseline, err := svc.ReadAll(testCtx(t))
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}

	tagOnly := writableChannel("001", 7_000_000, "RENAMED").Data
	tagOnly.Mode = "LSB" // match the baseline exactly except the tag — a MEM-only edit
	file := withChannel(baseline, "001", tagOnly)

	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: unexpected error: %v (want a writable plan — mostly-empty PMS must not block validation)", err)
	}
	diff := plan.Diff()
	if diff.Modified != 1 {
		t.Errorf("diff.Modified = %d, want 1", diff.Modified)
	}
	found := false
	for _, e := range diff.Entries {
		if e.Slot != "001" {
			continue
		}
		found = true
		if e.Blocked {
			t.Errorf("entry %q Blocked = true (%s), want a writable entry", e.Slot, e.BlockReason)
		}
	}
	if !found {
		t.Fatal("no diff entry for \"001\"")
	}
}

// TestSendPlan_ConfirmationDigest_PlanSpecific (Fix 1, adjudicated HIGH):
// two plans PrepareSend builds from the SAME baseline read but different
// candidate files used to share BaselineDigest() — the value Execute's
// confirmedDigest was checked against — so a caller who confirmed plan A's
// diff could hand that same digest to Execute(planB, ...) and have it
// silently accepted. ConfirmationDigest() must differ between the two
// plans, and Execute must refuse a plan handed the OTHER plan's
// ConfirmationDigest.
func TestSendPlan_ConfirmationDigest_PlanSpecific(t *testing.T) {
	_, sess := openSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	svc := NewService(sess, newStore(t), WithNow(stepClock(fixedNow)))
	caps := sess.Capabilities()

	fileA := matchingCandidateFile(caps, minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 14_100_000, "PLAN-A").Data,
	})
	planA, err := svc.PrepareSend(testCtx(t), fileA)
	if err != nil {
		t.Fatalf("PrepareSend(A): %v", err)
	}

	// A second PrepareSend against the SAME, still-untouched radio state
	// (the baseline read is identical) but a DIFFERENT candidate.
	fileB := matchingCandidateFile(caps, minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 21_200_000, "PLAN-B").Data,
	})
	planB, err := svc.PrepareSend(testCtx(t), fileB)
	if err != nil {
		t.Fatalf("PrepareSend(B): %v", err)
	}

	if planA.BaselineDigest() != planB.BaselineDigest() {
		t.Fatalf("BaselineDigest differs between plans prepared from the same untouched radio state (A=%q, B=%q) — this test's premise requires them to match", planA.BaselineDigest(), planB.BaselineDigest())
	}
	if planA.ConfirmationDigest() == planB.ConfirmationDigest() {
		t.Fatal("ConfirmationDigest is identical for two plans built from different candidates sharing a baseline — a caller confirming plan A's diff could Execute plan B")
	}

	// Confirming plan A's diff, but Executing plan B with THAT digest, must
	// be refused — not silently accepted just because the baseline half
	// matches.
	_, err = svc.Execute(testCtx(t), planB, planA.ConfirmationDigest(), ExecuteOptions{FirmwareConfirmed: "1.0"})
	var cme *ConfirmationMismatchError
	if !errors.As(err, &cme) {
		t.Fatalf("Execute(planB, planA's ConfirmationDigest) = %v, want a *ConfirmationMismatchError", err)
	}
	if !errors.Is(err, ErrConfirmationMismatch) {
		t.Error("errors.Is(err, ErrConfirmationMismatch) = false")
	}

	// The happy path: each plan's OWN ConfirmationDigest is accepted.
	reportA, err := svc.Execute(testCtx(t), planA, planA.ConfirmationDigest(), ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err != nil {
		t.Fatalf("Execute(planA, planA's own ConfirmationDigest): unexpected error: %v", err)
	}
	if reportA.Written != 1 {
		t.Errorf("reportA.Written = %d, want 1", reportA.Written)
	}
}

// TestPrepareSend_JournalRecordsConsentedUnverified: the "prepare" journal
// line states, for every plan, whether this session's write gate was
// opened by a user's RECORDED CONSENT (a ConsentedUnverified write label)
// rather than by hardware evidence alone. It is the audit trail's only
// record of that fact — the journal carries no capability snapshot for a
// reader to work it out from later (see capsConsented's doc comment) — so
// the field must be present and correct on BOTH arms, not merely present
// when true: an absent field and a false one are indistinguishable to a
// reader who cannot tell whether the run predates the field.
func TestPrepareSend_JournalRecordsConsentedUnverified(t *testing.T) {
	for _, tt := range []struct {
		name      string
		consented bool
	}{
		{name: "plain Simulated session", consented: false},
		{name: "consented labels", consented: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, consentedSess, plainSess := openConsentedSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
			sess := plainSess
			if tt.consented {
				sess = consentedSess
			}
			svc := NewService(sess, newStore(t), WithNow(stepClock(fixedNow)))

			caps := sess.Capabilities()
			file := matchingCandidateFile(caps, minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
				"001": writableChannel("001", 14_150_000, "MODIFIED").Data,
			})
			plan, err := svc.PrepareSend(testCtx(t), file)
			if err != nil {
				t.Fatalf("PrepareSend: unexpected error: %v", err)
			}

			rec := prepareRecord(t, journalPathFor(plan.SnapshotPath()))
			got, ok := rec["consented_unverified"]
			if !ok {
				t.Fatalf("prepare journal line has no \"consented_unverified\" field: %v", rec)
			}
			if got != tt.consented {
				t.Errorf("prepare line's consented_unverified = %v, want %v", got, tt.consented)
			}
		})
	}
}

// prepareRecord returns the single "prepare" record from the journal at
// path, failing the test if there is not exactly one.
func prepareRecord(t *testing.T, path string) map[string]any {
	t.Helper()
	var found map[string]any
	for _, rec := range readJournalRecords(t, path) {
		if rec["event"] != "prepare" {
			continue
		}
		if found != nil {
			t.Fatalf("journal %s has more than one \"prepare\" record", path)
		}
		found = rec
	}
	if found == nil {
		t.Fatalf("journal %s has no \"prepare\" record", path)
	}
	return found
}
