// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// TestExecute_HappyPath: mutating two existing channels and adding one
// more produces exactly 3 writes and 3 verifies, correct report counts,
// journal lines in order, and the fake's stored slot state matches what
// was sent.
func TestExecute_HappyPath(t *testing.T) {
	radio, sess := openSimSession(t, fakeradio.WithFactoryImage(happyPathImage))
	store := newStore(t)
	svc := NewService(sess, store, WithNow(stepClock(fixedNow)))

	caps := sess.Capabilities()
	file := matchingCandidateFile(caps, happyPathPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 14_100_000, "MOD-ONE").Data,
		"005": writableChannel("005", 14_200_000, "MOD-TWO").Data,
		"010": writableChannel("010", 14_300_000, "ADDED").Data,
	})

	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	report, err := svc.Execute(testCtx(t), plan, plan.ConfirmationDigest(), ExecuteOptions{FirmwareConfirmed: "1.23"})
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if report.Written != 3 {
		t.Errorf("Written = %d, want 3", report.Written)
	}
	if report.Verified != 3 {
		t.Errorf("Verified = %d, want 3", report.Verified)
	}
	if report.Aborted {
		t.Errorf("Aborted = true, want false (reason: %s)", report.AbortReason)
	}
	wantUnchanged := len(plan.diff.Entries) - 3
	if report.Unchanged != wantUnchanged {
		t.Errorf("Unchanged = %d, want %d", report.Unchanged, wantUnchanged)
	}
	if report.SkippedBlocked != 0 {
		t.Errorf("SkippedBlocked = %d, want 0 (Simulated profile writes everything this candidate touches)", report.SkippedBlocked)
	}

	for _, tt := range []struct {
		slot     string
		freqHz   uint32
		tag      string
		wantKind byte
	}{
		{"001", 14_100_000, "MOD-ONE", '1'},
		{"005", 14_200_000, "MOD-TWO", '1'},
		{"010", 14_300_000, "ADDED", '1'},
	} {
		st, ok := radio.SlotState(tt.slot)
		if !ok {
			t.Errorf("SlotState(%q): no state recorded", tt.slot)
			continue
		}
		if !st.Populated || st.Freq != freqDigits(tt.freqHz) || st.Tag != tt.tag {
			t.Errorf("SlotState(%q) = %+v, want Freq=%s Tag=%q Populated=true", tt.slot, st, freqDigits(tt.freqHz), tt.tag)
		}
	}
	// An untouched PMS slot must be exactly as the factory image left it —
	// empty (Codex M5b fix wave, Fix 3, adjudicated HIGH: real radios ship
	// all-PMS-empty; happyPathImage no longer artificially populates PMS).
	if _, ok := radio.SlotState("P1L"); ok {
		t.Error("untouched slot \"P1L\" has recorded state, want none (real-shape PMS starts empty)")
	}

	// Obligation 8: the journal is real, ordered, parses as JSON.
	events := readJournalEvents(t, report.JournalPath)
	if len(events) < 2 || events[0] != "prepare" {
		t.Fatalf("journal events = %v, want to start with \"prepare\"", events)
	}
	if events[len(events)-1] != "completion" {
		t.Errorf("journal's last event = %q, want \"completion\"", events[len(events)-1])
	}
	var writeAttempts, verifyResults int
	for _, e := range events {
		if e == "write_attempt" {
			writeAttempts++
		}
		if e == "verify_result" {
			verifyResults++
		}
	}
	if writeAttempts != 3 {
		t.Errorf("journal write_attempt count = %d, want 3", writeAttempts)
	}
	if verifyResults != 3 {
		t.Errorf("journal verify_result count = %d, want 3", verifyResults)
	}
}

// freqDigits mirrors fakeradio's 9-digit zero-padded ASCII frequency
// encoding, for asserting against SlotState.Freq directly.
func freqDigits(hz uint32) string {
	s := ""
	for i := 0; i < 9; i++ {
		s = string(rune('0'+hz%10)) + s
		hz /= 10
	}
	return s
}

// readJournalEvents reads path (a journal *.jsonl file) and returns each
// line's "event" field, in file order.
func readJournalEvents(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open journal %s: %v", path, err)
	}
	defer f.Close()

	var events []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("journal line does not parse as JSON: %v (%q)", err, string(line))
		}
		if _, ok := rec["t"]; !ok {
			t.Errorf("journal line missing \"t\": %q", string(line))
		}
		ev, _ := rec["event"].(string)
		events = append(events, ev)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanning journal: %v", err)
	}
	return events
}

// preparedSinglePlan is the common starting point every refusal test
// perturbs exactly one thing away from: a Simulated session over
// minimalFactoryImage, a plan that modifies "001", and the exact
// confirmedDigest Execute would accept.
func preparedSinglePlan(t *testing.T) (*Service, *fakeradio.Radio, driver.Session, *SendPlan, string) {
	t.Helper()
	radio, sess := openSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	svc := NewService(sess, newStore(t), WithNow(stepClock(fixedNow)))

	caps := sess.Capabilities()
	file := matchingCandidateFile(caps, minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 14_150_000, "MODIFIED").Data,
	})
	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}
	return svc, radio, sess, plan, plan.ConfirmationDigest()
}

// TestExecute_Refusals covers each pre-write refusal check, one test
// case each: wrong confirmedDigest, empty firmware, and a session-changed
// plan handed to a DIFFERENT Service.
func TestExecute_Refusals(t *testing.T) {
	t.Run("wrong confirmedDigest", func(t *testing.T) {
		svc, _, _, plan, _ := preparedSinglePlan(t)
		_, err := svc.Execute(testCtx(t), plan, "not-the-right-digest", ExecuteOptions{FirmwareConfirmed: "1.0"})
		var cme *ConfirmationMismatchError
		if !errors.As(err, &cme) {
			t.Fatalf("Execute = %v, want a *ConfirmationMismatchError", err)
		}
		if !errors.Is(err, ErrConfirmationMismatch) {
			t.Error("errors.Is(err, ErrConfirmationMismatch) = false")
		}
	})

	t.Run("firmware not confirmed", func(t *testing.T) {
		svc, _, _, plan, digest := preparedSinglePlan(t)
		_, err := svc.Execute(testCtx(t), plan, digest, ExecuteOptions{})
		if !errors.Is(err, ErrFirmwareUnconfirmed) {
			t.Fatalf("Execute = %v, want errors.Is match against ErrFirmwareUnconfirmed", err)
		}
	})

	t.Run("session changed (different Service instance)", func(t *testing.T) {
		_, _, _, plan, digest := preparedSinglePlan(t)

		// A second, independent session (and Service) — obligation 4:
		// even a session opened with the IDENTICAL caller-supplied
		// Identity must still be refused, because generation (not
		// content) is what distinguishes a genuinely different session
		// instance.
		_, sess2 := openSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
		svc2 := NewService(sess2, newStore(t))

		_, err := svc2.Execute(testCtx(t), plan, digest, ExecuteOptions{FirmwareConfirmed: "1.0"})
		var sce *SessionChangedError
		if !errors.As(err, &sce) {
			t.Fatalf("Execute = %v, want a *SessionChangedError", err)
		}
		if !errors.Is(err, ErrSessionChanged) {
			t.Error("errors.Is(err, ErrSessionChanged) = false")
		}
	})
}

// TestExecute_FirmwareConfirmedOnlyOnce: once a Service's first Execute
// call supplies FirmwareConfirmed, a LATER Execute call on the same
// Service does not need to repeat it.
func TestExecute_FirmwareConfirmedOnlyOnce(t *testing.T) {
	svc, _, sess, plan, digest := preparedSinglePlan(t)

	report, err := svc.Execute(testCtx(t), plan, digest, ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err != nil {
		t.Fatalf("first Execute: unexpected error: %v", err)
	}
	if report.Written != 1 {
		t.Fatalf("first Execute Written = %d, want 1", report.Written)
	}

	// A second PrepareSend/Execute pair, no FirmwareConfirmed this time.
	// The radio's "001" now holds what the first Execute wrote, not the
	// factory default — populated2 reflects that, so this second
	// candidate's only REAL delta is "005".
	caps := sess.Capabilities()
	populated2 := minimalFactoryPopulated()
	populated2["001"] = writableChannel("001", 14_150_000, "MODIFIED").Data
	file2 := matchingCandidateFile(caps, populated2, map[string]*codeplug.ChannelData{
		"005": writableChannel("005", 14_400_000, "AGAIN").Data,
	})
	plan2, err := svc.PrepareSend(testCtx(t), file2)
	if err != nil {
		t.Fatalf("second PrepareSend: %v", err)
	}
	report2, err := svc.Execute(testCtx(t), plan2, plan2.ConfirmationDigest(), ExecuteOptions{})
	if err != nil {
		t.Fatalf("second Execute (no FirmwareConfirmed): unexpected error: %v", err)
	}
	if report2.Written != 1 {
		t.Errorf("second Execute Written = %d, want 1", report2.Written)
	}
}

// TestExecute_FirmwareConfirmed_JournalAndReport (Fix 10b, obligation
// 10's audit clause): the confirmed firmware version must actually flow
// somewhere — journaled ONCE, at the first Execute, before the delta
// loop, and returned in Report.FirmwareConfirmed on that AND every later
// Execute call for the same Service — not sit in a write-only field
// nothing ever reads back.
func TestExecute_FirmwareConfirmed_JournalAndReport(t *testing.T) {
	const version = "1.23.4"
	svc, _, sess, plan, digest := preparedSinglePlan(t)

	report, err := svc.Execute(testCtx(t), plan, digest, ExecuteOptions{FirmwareConfirmed: version})
	if err != nil {
		t.Fatalf("first Execute: unexpected error: %v", err)
	}
	if report.FirmwareConfirmed != version {
		t.Errorf("report.FirmwareConfirmed = %q, want %q", report.FirmwareConfirmed, version)
	}

	records := readJournalRecords(t, report.JournalPath)
	var firmwareEvents, writeAttemptIdx, firmwareIdx int
	writeAttemptIdx, firmwareIdx = -1, -1
	for i, rec := range records {
		ev, _ := rec["event"].(string)
		if ev == "firmware_confirmed" {
			firmwareEvents++
			if firmwareIdx == -1 {
				firmwareIdx = i
			}
			if v, _ := rec["version"].(string); v != version {
				t.Errorf("firmware_confirmed event's \"version\" = %v, want %q", rec["version"], version)
			}
		}
		if ev == "write_attempt" && writeAttemptIdx == -1 {
			writeAttemptIdx = i
		}
	}
	if firmwareEvents != 1 {
		t.Errorf("journal has %d \"firmware_confirmed\" events, want exactly 1", firmwareEvents)
	}
	if writeAttemptIdx != -1 && firmwareIdx != -1 && firmwareIdx > writeAttemptIdx {
		t.Errorf("\"firmware_confirmed\" (line %d) appeared after the first \"write_attempt\" (line %d) — must be emitted BEFORE the delta-write loop", firmwareIdx, writeAttemptIdx)
	}

	// A second PrepareSend/Execute pair on the SAME Service, no
	// FirmwareConfirmed this time: must not re-prompt (already proven by
	// TestExecute_FirmwareConfirmedOnlyOnce), must not re-emit the
	// journal event into the SECOND plan's (distinct) journal file, and
	// the report must still carry the persisted version.
	caps := sess.Capabilities()
	populated2 := minimalFactoryPopulated()
	populated2["001"] = writableChannel("001", 14_150_000, "MODIFIED").Data
	file2 := matchingCandidateFile(caps, populated2, map[string]*codeplug.ChannelData{
		"005": writableChannel("005", 14_400_000, "AGAIN").Data,
	})
	plan2, err := svc.PrepareSend(testCtx(t), file2)
	if err != nil {
		t.Fatalf("second PrepareSend: %v", err)
	}
	report2, err := svc.Execute(testCtx(t), plan2, plan2.ConfirmationDigest(), ExecuteOptions{})
	if err != nil {
		t.Fatalf("second Execute (no FirmwareConfirmed): unexpected error: %v", err)
	}
	if report2.FirmwareConfirmed != version {
		t.Errorf("second report.FirmwareConfirmed = %q, want the persisted %q", report2.FirmwareConfirmed, version)
	}
	for _, e := range readJournalEventDetails(t, report2.JournalPath) {
		if e.event == "firmware_confirmed" {
			t.Error("second Execute's journal contains a \"firmware_confirmed\" event, want none (already confirmed for this Service)")
		}
	}
}

// TestExecute_CandidateImmutable: obligation 2's central proof. After
// PrepareSend, the caller mutates the very *codeplug.Codeplug it passed
// in; Execute must still write the PLAN's captured values, not the
// caller's edited ones.
func TestExecute_CandidateImmutable(t *testing.T) {
	radio, sess := openSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	svc := NewService(sess, newStore(t), WithNow(stepClock(fixedNow)))

	caps := sess.Capabilities()
	originalData := writableChannel("001", 14_150_000, "ORIGINAL").Data
	file := matchingCandidateFile(caps, minimalFactoryPopulated(), map[string]*codeplug.ChannelData{"001": originalData})

	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	// Mutate the caller's own file in place, AFTER PrepareSend returned.
	for i := range file.Channels {
		if file.Channels[i].Slot == "001" {
			file.Channels[i].Data.FreqHz = 99_999_000
			file.Channels[i].Data.Tag = "TAMPERED"
		}
	}

	report, err := svc.Execute(testCtx(t), plan, plan.ConfirmationDigest(), ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if report.Written != 1 || report.Aborted {
		t.Fatalf("report = %+v, want Written=1, Aborted=false", report)
	}

	st, ok := radio.SlotState("001")
	if !ok {
		t.Fatal("SlotState(\"001\"): no state recorded")
	}
	if st.Tag != "ORIGINAL" || st.Freq != freqDigits(14_150_000) {
		t.Errorf("radio received %+v, want the PLAN's original values (Tag=\"ORIGINAL\", Freq=%s) — post-PrepareSend mutation of the caller's file leaked into Execute", st, freqDigits(14_150_000))
	}
}

// TestExecute_RealHardwareProfile_BlockedEntriesNeverWritten is the
// post-M5b-flip rewrite of TestExecute_BlockedProfile_NoOp. The old
// test pinned "on a RealHardware session EVERY entry is Blocked and
// Execute is a total no-op" — retired by the flip (the RealHardware
// profile is now write-capable for the verified fields; see
// core/driver/ft710's TestWriteTrialsComplete_FlippedTrue_M5b). Its
// honest replacement, NOT a deletion: on the REAL profile, an entry
// that IS still Blocked — here a clarifier change, which the radio
// provably ignores (spec.Inert) — is skipped with the correct count and
// ZERO wire writes after PrepareSend. Real writes are gated by the
// clone choreography (digest + confirmation + firmware bindings, all
// pinned by TestExecute_Refusals) and per-entry Blocked flags, not by a
// blanket capability veto.
func TestExecute_RealHardwareProfile_BlockedEntriesNeverWritten(t *testing.T) {
	_, port, sess := openCountingRealHardwareSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	svc := NewService(sess, newStore(t), WithNow(stepClock(fixedNow)))

	caps := sess.Capabilities()
	clarEdit := writableChannel("001", 7_000_000, "").Data
	clarEdit.Mode = "LSB"
	clarEdit.ClarHz = 100 // the ONLY change vs the baseline: Blocked (Inert)
	file := matchingCandidateFile(caps, minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": clarEdit,
	})
	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	writesBeforeExecute := port.writes.Load()

	report, err := svc.Execute(testCtx(t), plan, plan.ConfirmationDigest(), ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if report.Written != 0 || report.Verified != 0 {
		t.Errorf("Written/Verified = %d/%d, want 0/0", report.Written, report.Verified)
	}
	if report.SkippedBlocked != 1 {
		t.Errorf("SkippedBlocked = %d, want 1", report.SkippedBlocked)
	}
	if report.Aborted {
		t.Errorf("Aborted = true, want false")
	}
	if got := port.writes.Load(); got != writesBeforeExecute {
		t.Errorf("Execute produced %d wire Write() calls, want 0 (a Blocked entry must never be written, on any profile)", got-writesBeforeExecute)
	}
}

// TestExecute_VerifyReadPhase_BaselineDrift (obligation 11): after
// PrepareSend, the radio's content for the to-be-written slot is changed
// by a DIFFERENT means (a direct WriteChannel through the same session,
// bypassing the clone Service — modelling a second tool or a front-panel
// edit). Execute's verify-read phase must catch this BEFORE any write is
// attempted.
func TestExecute_VerifyReadPhase_BaselineDrift(t *testing.T) {
	radio, sess := openSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	svc := NewService(sess, newStore(t), WithNow(stepClock(fixedNow)))

	caps := sess.Capabilities()
	file := matchingCandidateFile(caps, minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 14_150_000, "MODIFIED").Data,
	})
	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	// Drift the radio directly, bypassing the Service entirely.
	if _, err := sess.WriteChannel(testCtx(t), writableChannel("001", 21_000_000, "DRIFTED")); err != nil {
		t.Fatalf("direct WriteChannel (simulating out-of-band drift): %v", err)
	}

	report, err := svc.Execute(testCtx(t), plan, plan.ConfirmationDigest(), ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err == nil {
		t.Fatal("Execute = nil error, want ErrStaleBaseline (radio drifted since PrepareSend)")
	}
	if !errors.Is(err, ErrStaleBaseline) {
		t.Errorf("errors.Is(err, ErrStaleBaseline) = false; err = %v", err)
	}
	if !errors.Is(err, ErrAborted) {
		t.Errorf("errors.Is(err, ErrAborted) = false; err = %v", err)
	}
	var sbe *StaleBaselineError
	if !errors.As(err, &sbe) {
		t.Fatalf("error %v is not a *StaleBaselineError", err)
	}
	if sbe.Slot != "001" {
		t.Errorf("StaleBaselineError.Slot = %q, want \"001\"", sbe.Slot)
	}
	if report == nil {
		t.Fatal("report is nil, want a non-nil partial report even on this abort")
	}
	if report.Written != 0 {
		t.Errorf("Written = %d, want 0 (caught before any write)", report.Written)
	}
	if !report.Aborted {
		t.Error("Aborted = false, want true")
	}

	// The drifted value must survive: Execute must not have written
	// anything after catching the drift.
	st, _ := radio.SlotState("001")
	if st.Tag != "DRIFTED" {
		t.Errorf("SlotState(\"001\").Tag = %q, want \"DRIFTED\" (Execute must not have written after aborting)", st.Tag)
	}
}

// TestExecute_VerifyReadPerSlot_FirstWrittenBeforeSecondDrifts (M3
// Codex-review fix wave, Fix 3): obligation 11's verify-read check must
// run PER-SLOT, immediately before that slot's own write — not as a
// single up-front phase covering every to-be-written slot before ANY of
// them are written (closing the drift-to-write window down to as small
// as physically possible). Proof: drift ONLY the SECOND delta ("005")
// before Execute runs; the FIRST delta ("001") is left untouched. A
// per-slot check still catches "005"'s drift and aborts — but "001" must
// have been fully written and verified FIRST, since its own
// verify-read/write/verify sequence runs to completion before the loop
// ever reaches "005". Before this fix, a single up-front verify-read
// phase re-read EVERY delta before writing ANY of them, so "005"'s drift
// would abort before "001" was ever written at all (Written would be 0,
// not 1) — this is the difference this test pins down.
func TestExecute_VerifyReadPerSlot_FirstWrittenBeforeSecondDrifts(t *testing.T) {
	radio, sess := openSimSession(t, fakeradio.WithFactoryImage(happyPathImage))
	svc := NewService(sess, newStore(t), WithNow(stepClock(fixedNow)))

	caps := sess.Capabilities()
	file := matchingCandidateFile(caps, happyPathPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 14_100_000, "MOD-ONE").Data,
		"005": writableChannel("005", 14_200_000, "MOD-TWO").Data,
	})
	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	// Drift ONLY the second delta ("005"), bypassing the Service — "001"
	// is left exactly as PrepareSend read it.
	if _, err := sess.WriteChannel(testCtx(t), writableChannel("005", 21_000_000, "DRIFTED")); err != nil {
		t.Fatalf("direct WriteChannel (simulating out-of-band drift on \"005\"): %v", err)
	}

	report, err := svc.Execute(testCtx(t), plan, plan.ConfirmationDigest(), ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err == nil {
		t.Fatal("Execute = nil error, want ErrStaleBaseline (slot \"005\" drifted since PrepareSend)")
	}
	if !errors.Is(err, ErrStaleBaseline) {
		t.Errorf("errors.Is(err, ErrStaleBaseline) = false; err = %v", err)
	}
	var sbe *StaleBaselineError
	if !errors.As(err, &sbe) {
		t.Fatalf("error %v is not a *StaleBaselineError", err)
	}
	if sbe.Slot != "005" {
		t.Errorf("StaleBaselineError.Slot = %q, want \"005\"", sbe.Slot)
	}
	if report == nil {
		t.Fatal("report is nil, want a non-nil partial report")
	}

	// The proof: "001" (the FIRST delta, untouched by the drift) must have
	// been fully written and verified BEFORE the loop reached "005" and
	// aborted.
	if report.Written != 1 {
		t.Errorf("Written = %d, want 1 (\"001\" must complete its own write+verify pair before \"005\"'s drift is even checked)", report.Written)
	}
	if report.Verified != 1 {
		t.Errorf("Verified = %d, want 1", report.Verified)
	}
	if st, ok := radio.SlotState("001"); !ok || st.Tag != "MOD-ONE" {
		t.Errorf("SlotState(\"001\") = %+v, ok=%v, want the WRITTEN value — its pair must have completed before \"005\" was even checked", st, ok)
	}
	// "005" must still show the drifted value: Execute must not have
	// written over it after catching the drift.
	if st, _ := radio.SlotState("005"); st.Tag != "DRIFTED" {
		t.Errorf("SlotState(\"005\").Tag = %q, want \"DRIFTED\" (Execute must not have written after aborting)", st.Tag)
	}
}

// TestExecute_VerifyReadPhase_ReadError (Fix 10b, Fix 4 — uniform abort
// reporting): the per-slot verify-read check (obligation 11 — see Fix 3's
// doc comment in doc.go for why this is no longer a separate up-front
// phase) can fail outright, as distinct from succeeding but disagreeing
// (that is TestExecute_VerifyReadPhase_BaselineDrift,
// actionVerifyReadDrift). FaultDropReplies on the FIRST delta's own
// verify-read exchange (125, per preparedThreeDeltaPlan's doc comment —
// unchanged by Fix 3's merge, since delta1's verify-read is still the
// very first thing Execute's delta-write loop does, right after
// obligation 12's own MC-snapshot query at exchange 124) makes that
// ReadChannel call itself fail. Execute must abort BEFORE any write, and
// record a SlotResult naming this distinct case (actionVerifyReadError).
func TestExecute_VerifyReadPhase_ReadError(t *testing.T) {
	const firstVerifyReadExchange = 125
	svc, radio, _, plan, digest := preparedThreeDeltaPlan(t,
		fakeradio.WithFault(fakeradio.FaultDropReplies(firstVerifyReadExchange)),
	)

	report, err := svc.Execute(testCtx(t), plan, digest, ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err == nil {
		t.Fatal("Execute = nil error, want an abort (verify-read phase's re-read never answers)")
	}
	if !errors.Is(err, ErrAborted) {
		t.Errorf("errors.Is(err, ErrAborted) = false; err = %v", err)
	}
	if report == nil {
		t.Fatal("report is nil, want a non-nil partial report")
	}
	if report.Written != 0 {
		t.Errorf("Written = %d, want 0 (caught before any write)", report.Written)
	}
	if !report.Aborted {
		t.Error("Aborted = false, want true")
	}
	if len(report.Slots) != 1 {
		t.Fatalf("report.Slots = %+v, want exactly one entry (the in-flight slot)", report.Slots)
	}
	got := report.Slots[0]
	if got.Slot != "001" || got.Action != actionVerifyReadError || got.Detail == "" {
		t.Errorf("report.Slots[0] = %+v, want Slot=\"001\" Action=%q with a non-empty Detail", got, actionVerifyReadError)
	}

	// "001"/"005" pre-exist in happyPathImage (the factory image) — they
	// must still show their UNTOUCHED factory values, not the candidate's
	// edits; "010" never existed at all, so it must show no recorded
	// state whatsoever.
	for _, tt := range []struct {
		slot         string
		wantExisting bool
	}{
		{"001", true},
		{"005", true},
		{"010", false},
	} {
		st, ok := radio.SlotState(tt.slot)
		if tt.wantExisting {
			if !ok {
				t.Errorf("SlotState(%q): no state recorded, want the untouched factory value", tt.slot)
				continue
			}
			if st.Tag == "MOD-ONE" || st.Tag == "MOD-TWO" {
				t.Errorf("SlotState(%q) = %+v, want the untouched factory value — nothing must have been written", tt.slot, st)
			}
			continue
		}
		if ok {
			t.Errorf("SlotState(%q) has recorded state, want none — never attempted", tt.slot)
		}
	}
}

// TestExecute_DualDigestRecheck_SelfConsistency: obligation 3's dual-digest
// recheck is a defensive self-consistency assertion — SendPlan's opacity
// means no PUBLIC API can ever trigger it (that is the whole point of
// obligation 2), but this test package shares SendPlan's package, so it
// can corrupt a plan's private copies directly and prove Execute still
// catches it before writing anything.
func TestExecute_DualDigestRecheck_SelfConsistency(t *testing.T) {
	t.Run("baseline", func(t *testing.T) {
		svc, _, _, plan, digest := preparedSinglePlan(t)
		plan.baseline[0].Data.FreqHz++ // corrupt the plan's own private copy
		_, err := svc.Execute(testCtx(t), plan, digest, ExecuteOptions{FirmwareConfirmed: "1.0"})
		if !errors.Is(err, ErrStaleBaseline) {
			t.Fatalf("Execute = %v, want errors.Is match against ErrStaleBaseline", err)
		}
	})
	t.Run("candidate", func(t *testing.T) {
		svc, _, _, plan, digest := preparedSinglePlan(t)
		plan.candidate[0].Data.FreqHz++
		_, err := svc.Execute(testCtx(t), plan, digest, ExecuteOptions{FirmwareConfirmed: "1.0"})
		if !errors.Is(err, ErrCandidateChanged) {
			t.Fatalf("Execute = %v, want errors.Is match against ErrCandidateChanged", err)
		}
	})
}

// TestExecute_ContextCancelled: a context cancelled before Execute starts
// is refused immediately.
func TestExecute_ContextCancelled(t *testing.T) {
	svc, _, _, plan, digest := preparedSinglePlan(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := svc.Execute(ctx, plan, digest, ExecuteOptions{FirmwareConfirmed: "1.0"}); err == nil {
		t.Error("Execute with an already-cancelled context = nil error, want an error")
	}
}

// TestExecute_CallerCancellation_DoesNotAbandonWriteVerifyPair (M3
// Codex-review fix wave, Fix 7, adjudicated MEDIUM): once write_attempt
// is durably journaled for a slot, that slot's write+verify pair must run
// to completion even if the CALLER's ctx is cancelled mid-pair — a write
// without its paired verify is an unknown state, and abandoning one
// because the caller happened to give up at exactly the wrong instant
// would leave the radio's memory and this package's bookkeeping unable to
// agree on what happened.
//
// Deterministic construction: cancel the caller's ctx from WITHIN the
// Progress callback on phase "write" for slot "001" — i.e. immediately
// after slot 1's WriteChannel has already succeeded, but before its
// paired read-back verify has run. The cancellation must still be
// honoured, but only BETWEEN pairs: slot 1 completes (written AND
// verified), and Execute aborts before slot 2 ("005") is ever touched.
func TestExecute_CallerCancellation_DoesNotAbandonWriteVerifyPair(t *testing.T) {
	radio, sess := openSimSession(t, fakeradio.WithFactoryImage(happyPathImage))
	store := newStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc := NewService(sess, store, WithNow(stepClock(fixedNow)), WithProgress(func(phase string, done, total int, slot string) {
		if phase == "write" && slot == "001" {
			cancel()
		}
	}))

	caps := sess.Capabilities()
	file := matchingCandidateFile(caps, happyPathPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 14_100_000, "MOD-ONE").Data,
		"005": writableChannel("005", 14_200_000, "MOD-TWO").Data,
	})
	plan, err := svc.PrepareSend(ctx, file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	report, err := svc.Execute(ctx, plan, plan.ConfirmationDigest(), ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err == nil {
		t.Fatal("Execute = nil error, want an abort (caller ctx was cancelled mid-run)")
	}
	if !errors.Is(err, ErrAborted) {
		t.Errorf("errors.Is(err, ErrAborted) = false; err = %v", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("errors.Is(err, context.Canceled) = false; err = %v — the abort's Cause must be the caller's ctx cancellation", err)
	}
	if report == nil {
		t.Fatal("report is nil, want a non-nil partial report")
	}
	if report.Written != 1 || report.Verified != 1 {
		t.Errorf("Written/Verified = %d/%d, want 1/1 (slot \"001\"'s write+verify pair must complete despite the caller ctx cancelling mid-pair)", report.Written, report.Verified)
	}
	if !report.Aborted {
		t.Error("Aborted = false, want true")
	}

	// "001" must show the WRITTEN value: the pair completed cleanly
	// despite the caller's ctx dying mid-pair.
	if st, ok := radio.SlotState("001"); !ok || st.Tag != "MOD-ONE" {
		t.Errorf("SlotState(\"001\") = %+v, ok=%v, want the written value (its write+verify pair must have completed)", st, ok)
	}
	// "005" must never have been attempted: the cancellation is honoured
	// BETWEEN pairs, before slot 2's own verify-read/write ever starts.
	if st, ok := radio.SlotState("005"); !ok || st.Tag == "MOD-TWO" {
		t.Errorf("SlotState(\"005\") = %+v, ok=%v, want the untouched factory value — never attempted", st, ok)
	}

	// Journal: write_attempt/write_result/verify_result all present for
	// "001"; no write_attempt at all for "005".
	events := readJournalEventDetails(t, report.JournalPath)
	sawSlot1Verify := false
	sawSlot2Attempt := false
	for _, e := range events {
		if e.event == "verify_result" && e.slot == "001" {
			sawSlot1Verify = true
		}
		if e.event == "write_attempt" && e.slot == "005" {
			sawSlot2Attempt = true
		}
	}
	if !sawSlot1Verify {
		t.Error("journal has no verify_result for slot \"001\" — its write+verify pair must have been journaled to completion")
	}
	if sawSlot2Attempt {
		t.Error("journal records a write_attempt for slot \"005\", want none — the caller cancellation must be honoured BETWEEN pairs, before slot 2 ever starts")
	}
}

// TestExecute_ConcurrentCalls_OneBusy (M3 Codex-review fix wave, Fix 2,
// adjudicated MEDIUM): two Execute calls against the SAME Service,
// started concurrently, must never interleave their wire traffic —
// Service enforces this with a try-lock (Service.acquireOp): whichever
// caller arrives while the OTHER is still running gets a typed *BusyError
// immediately, never blocking, never interleaved.
func TestExecute_ConcurrentCalls_OneBusy(t *testing.T) {
	svc, _, _, plan, digest := preparedThreeDeltaPlan(t)

	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		_, err := svc.Execute(testCtx(t), plan, digest, ExecuteOptions{FirmwareConfirmed: "1.0"})
		done <- err
	}()

	<-started
	// Give the first call a moment to actually enter Execute and acquire
	// the operation lock before the second call is attempted — Execute's
	// delta-write loop over 3 channels takes well over 100ms (transport
	// settle/error-window pacing), a wide margin against this short sleep.
	time.Sleep(5 * time.Millisecond)

	_, err2 := svc.Execute(testCtx(t), plan, digest, ExecuteOptions{FirmwareConfirmed: "1.0"})
	err1 := <-done

	busy1 := errors.Is(err1, ErrBusy)
	busy2 := errors.Is(err2, ErrBusy)
	if busy1 == busy2 {
		t.Fatalf("want exactly one of the two concurrent Execute calls to get ErrBusy, got err1=%v (busy=%v) err2=%v (busy=%v)", err1, busy1, err2, busy2)
	}

	var be *BusyError
	busyErr := err1
	if busy2 {
		busyErr = err2
	}
	if !errors.As(busyErr, &be) {
		t.Fatalf("busy error %v is not a *BusyError", busyErr)
	}
	if be.InProgress != "Execute" {
		t.Errorf("BusyError.InProgress = %q, want \"Execute\"", be.InProgress)
	}
}

// preparedThreeDeltaPlan wires a Simulated session over happyPathImage
// (M-01, M-05 populated; every PMS slot EMPTY — Codex M5b fix wave, Fix
// 3, adjudicated HIGH: real radios ship all-PMS-empty, so happyPathImage
// (via minimalFactoryImage) no longer artificially populates PMS just to
// satisfy the (now-removed) NoBlank rule; no 60m/EMG — Region() "no-60m",
// HW-CONFIRMED 2026-07-13 label, see docs/hardware-notes.md) with a
// candidate that modifies "001" and "005" and adds
// "010": exactly three unblocked deltas, in that fixed order (diff.Entries
// follows baseline slot order, i.e. numeric MEM order).
//
// Exchange arithmetic (fault tests below pin exact numbers against this):
// Open = AI0;(1) + ID;(2) + MR501;(3, rejected: no 60m) + MREMG;(4,
// rejected: no EMG) = 4. ReadAll walks MEM("001".."099") then PMS: "001"
// and "005" populated (2 exchanges each: MR+MT), the other 97 MEM slots
// empty (1 exchange: MR rejected only), all 18 PMS slots EMPTY (1
// exchange each: MR rejected only) = (2+2+97) + 18 = 119, exchanges
// 5..123.
//
// Obligation 12 (docs/hardware-notes.md, M5b — MW moves the radio's
// selection): Execute now spends exchange 124 on its own MC-snapshot
// query, BEFORE the delta-write loop below — see snapshotMemorySelection
// (memory_selector.go).
//
// Fix 3 (M3 Codex-review fix wave) merged the verify-read check into each
// slot's own write+verify sequence — there is no longer a separate
// up-front verify-read phase, so the numbering below runs PER SLOT,
// back to back: delta1 "001" (already populated) — verify-read MR=125,
// MT=126; write MW=127, MT=128; verify-readback MR=129, MT=130. delta2
// "005" (already populated) — verify-read MR=131, MT=132; write
// MW=133, MT=134 (the "2nd write"); verify-readback MR=135, MT=136.
// delta3 "010" (still empty going in) — verify-read MR=137 (rejected:
// empty, so no MT); write MW=138, MT=139; verify-readback MR=140,
// MT=141 (now populated).
func preparedThreeDeltaPlan(t *testing.T, opts ...fakeradio.Option) (*Service, *fakeradio.Radio, driver.Session, *SendPlan, string) {
	t.Helper()
	allOpts := append([]fakeradio.Option{fakeradio.WithFactoryImage(happyPathImage)}, opts...)
	radio, sess := openSimSession(t, allOpts...)
	svc := NewService(sess, newStore(t), WithNow(stepClock(fixedNow)))

	caps := sess.Capabilities()
	file := matchingCandidateFile(caps, happyPathPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 14_100_000, "MOD-ONE").Data,
		"005": writableChannel("005", 14_200_000, "MOD-TWO").Data,
		"010": writableChannel("010", 14_300_000, "ADDED").Data,
	})
	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}
	return svc, radio, sess, plan, plan.ConfirmationDigest()
}

// TestExecute_Abort_WriteRejected: the SECOND delta's MW frame (exchange
// 133 — see preparedThreeDeltaPlan's doc comment) draws a delayed "?;"
// rejection. Execute must report exactly one Written+Verified (the
// first delta), abort, and never attempt the third delta's write at all.
func TestExecute_Abort_WriteRejected(t *testing.T) {
	const secondWriteMWExchange = 133
	svc, radio, _, plan, digest := preparedThreeDeltaPlan(t,
		fakeradio.WithFault(fakeradio.FaultDelayedRejection(secondWriteMWExchange, 50*time.Millisecond)),
	)

	report, err := svc.Execute(testCtx(t), plan, digest, ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err == nil {
		t.Fatal("Execute = nil error, want an abort (the 2nd write was rejected)")
	}
	if !errors.Is(err, ErrAborted) {
		t.Errorf("errors.Is(err, ErrAborted) = false; err = %v", err)
	}
	if report == nil {
		t.Fatal("report is nil, want a non-nil partial report")
	}
	if report.Written != 1 {
		t.Errorf("Written = %d, want 1 (only the first delta, \"001\")", report.Written)
	}
	if report.Verified != 1 {
		t.Errorf("Verified = %d, want 1", report.Verified)
	}
	if !report.Aborted {
		t.Error("Aborted = false, want true")
	}

	// "001" (delta1) must show the NEW value: it was written and verified
	// before the abort.
	if st, ok := radio.SlotState("001"); !ok || st.Tag != "MOD-ONE" {
		t.Errorf("SlotState(\"001\") = %+v, ok=%v, want the written value (delta1 completed before the abort)", st, ok)
	}

	// Fix 4 (uniform abort reporting): the write-rejection path must
	// append a SlotResult for the in-flight slot ("005"), like the other
	// abort paths already do.
	if n := len(report.Slots); n == 0 {
		t.Fatal("report.Slots is empty, want at least one entry for the in-flight slot")
	} else if got := report.Slots[n-1]; got.Slot != "005" || got.Action != actionWrite || got.VerifyOK || got.Detail == "" {
		t.Errorf("report.Slots[%d] = %+v, want Slot=\"005\" Action=%q VerifyOK=false with a non-empty Detail", n-1, got, actionWrite)
	}
	// "010" (delta3) must never have been attempted at all — the clearest
	// proof "no 3rd write" happened: unlike delta2 ("005"), whose MW the
	// fake still APPLIES internally even though it tells the host "?;"
	// (FaultDelayedRejection overrides only the reply, not the fake's own
	// state mutation — see internal/fakeradio's handleMW), "010" was never
	// even attempted, so it must have NO recorded state whatsoever.
	if _, ok := radio.SlotState("010"); ok {
		t.Error("SlotState(\"010\") has recorded state, want none — the 3rd write must never have been attempted")
	}

	// Journal: a write_result naming the rejection for "005", and an
	// abort event, with no write_attempt for "010" at all.
	events := readJournalEventDetails(t, report.JournalPath)
	sawAbort := false
	sawThirdAttempt := false
	for _, e := range events {
		if e.event == "abort" {
			sawAbort = true
		}
		if e.event == "write_attempt" && e.slot == "010" {
			sawThirdAttempt = true
		}
	}
	if !sawAbort {
		t.Error("journal has no \"abort\" event")
	}
	if sawThirdAttempt {
		t.Error("journal records a write_attempt for \"010\" (the 3rd write), want none")
	}
}

// TestExecute_Abort_VerifyAmbiguous: the first delta's verify-READBACK MR
// frame (exchange 129 — see preparedThreeDeltaPlan's doc comment) never
// gets a reply at all (fakeradio.FaultDropReplies — chosen over
// FaultGarbleReply because mrSpec's RetryReads:1 means a single garbled
// reply is simply ignored as "unexpected" and silently recovered by the
// automatic retry, which gets a FRESH, non-garbled reply; dropping every
// reply from this exchange onward defeats the retry too, producing a
// genuine, persistent ambiguity — the same "we sent something and cannot
// tell what happened" state a real stuck radio would leave). Execute must
// treat this as an aborting failure: the write itself (MW+MT) went out
// and drew no rejection, but the verify-read that should confirm it can
// never complete.
func TestExecute_Abort_VerifyAmbiguous(t *testing.T) {
	const firstVerifyMRExchange = 129
	svc, radio, _, plan, digest := preparedThreeDeltaPlan(t,
		fakeradio.WithFault(fakeradio.FaultDropReplies(firstVerifyMRExchange)),
	)

	report, err := svc.Execute(testCtx(t), plan, digest, ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err == nil {
		t.Fatal("Execute = nil error, want an abort (the 1st write's verify read-back never answers)")
	}
	if !errors.Is(err, ErrAborted) {
		t.Errorf("errors.Is(err, ErrAborted) = false; err = %v", err)
	}
	if report == nil {
		t.Fatal("report is nil, want a non-nil partial report")
	}
	// The write itself (MW+MT) completed before its verify read-back
	// stalled, so Written is 1 — but Verified must be 0: an ambiguous
	// read-back is never counted as a successful verification.
	if report.Written != 1 {
		t.Errorf("Written = %d, want 1", report.Written)
	}
	if report.Verified != 0 {
		t.Errorf("Verified = %d, want 0 (verify never completed)", report.Verified)
	}
	if !report.Aborted {
		t.Error("Aborted = false, want true")
	}

	// Fix 4 (uniform abort reporting): the verify-readback-error path
	// must append a SlotResult for the in-flight slot ("001"), like the
	// other abort paths already do.
	if n := len(report.Slots); n == 0 {
		t.Fatal("report.Slots is empty, want at least one entry for the in-flight slot")
	} else if got := report.Slots[n-1]; got.Slot != "001" || got.Action != actionWrite || got.VerifyOK || got.Detail == "" {
		t.Errorf("report.Slots[%d] = %+v, want Slot=\"001\" Action=%q VerifyOK=false with a non-empty Detail", n-1, got, actionWrite)
	}

	// Neither the second nor third delta must have been attempted.
	for _, slot := range []string{"005", "010"} {
		if slot == "005" {
			// "005" pre-existed in the factory image; it must still show
			// the ORIGINAL value, not the candidate's "MOD-TWO".
			st, ok := radio.SlotState(slot)
			if !ok || st.Tag == "MOD-TWO" {
				t.Errorf("SlotState(%q) = %+v, ok=%v, want the untouched factory value", slot, st, ok)
			}
			continue
		}
		if _, ok := radio.SlotState(slot); ok {
			t.Errorf("SlotState(%q) has recorded state, want none — never attempted", slot)
		}
	}
}

// TestExecute_WriteChannelFailure_MTRejectedAfterMWSucceeds_AttemptsReadback
// is Codex M5b fix wave, Fix 1 (adjudicated HIGH): WriteChannel can
// successfully confirm MW, then fail/reject MT — the radio's memory has
// already changed even though the OVERALL write is reported as a
// failure. The first delta's ("001") own MT write frame (exchange 128 —
// see preparedThreeDeltaPlan's doc comment: write MW=127, MT=128) draws
// a delayed "?;" rejection; fakeradio still APPLIES the MT body
// internally before substituting the rejection reply (handleFrame runs
// before the fault switch — see fakeradio.go's handleEvent), so the
// slot's true post-failure state is the FULLY written one — exactly the
// kind of silently-changed-but-reported-as-failed state Fix 1 exists to
// surface. writePair must now ALWAYS attempt a same-slot readback under
// pairCtx on this failure path, journal the observed outcome
// ("post_failure_readback"), and still abort with the ORIGINAL
// WriteChannel error (MT rejected) as the cause — never the readback's
// own outcome.
func TestExecute_WriteChannelFailure_MTRejectedAfterMWSucceeds_AttemptsReadback(t *testing.T) {
	const firstWriteMTExchange = 128
	svc, radio, _, plan, digest := preparedThreeDeltaPlan(t,
		fakeradio.WithFault(fakeradio.FaultDelayedRejection(firstWriteMTExchange, 10*time.Millisecond)),
	)

	report, err := svc.Execute(testCtx(t), plan, digest, ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err == nil {
		t.Fatal("Execute = nil error, want an abort (the 1st write's MT was rejected after its MW succeeded)")
	}
	if !errors.Is(err, ErrAborted) {
		t.Errorf("errors.Is(err, ErrAborted) = false; err = %v", err)
	}
	if report == nil {
		t.Fatal("report is nil, want a non-nil partial report")
	}
	if !report.Aborted {
		t.Error("Aborted = false, want true")
	}
	if !strings.Contains(report.AbortReason, "MT rejected") {
		t.Errorf("AbortReason = %q, want it to preserve the ORIGINAL MT-rejection cause", report.AbortReason)
	}

	// The radio's ACTUAL state: MW changed it for real (frequency), and —
	// because fakeradio applies a command's body before ever consulting
	// the fault table — MT changed it for real too, despite the "?;"
	// reply. Proves the mutation genuinely happened irrespective of what
	// WriteChannel was told.
	if st, ok := radio.SlotState("001"); !ok || st.Freq != freqDigits(14_100_000) {
		t.Errorf("SlotState(\"001\") = %+v, ok=%v, want the new frequency (MW succeeded for real)", st, ok)
	}

	// The mandatory readback: a "post_failure_readback" journal event for
	// "001", reporting the state actually observed on the wire — not an
	// absence, and not silence.
	records := readJournalRecords(t, report.JournalPath)
	var found map[string]any
	for _, rec := range records {
		if rec["event"] == "post_failure_readback" && rec["slot"] == "001" {
			found = rec
			break
		}
	}
	if found == nil {
		t.Fatalf("journal has no \"post_failure_readback\" event for slot \"001\" — readback must ALWAYS be attempted once WriteChannel has been invoked; records = %v", records)
	}
	if ok, _ := found["ok"].(bool); !ok {
		t.Errorf("post_failure_readback[\"ok\"] = %v, want true (the readback itself succeeded)", found["ok"])
	}
	if freq, _ := found["freq_hz"].(float64); uint32(freq) != 14_100_000 {
		t.Errorf("post_failure_readback[\"freq_hz\"] = %v, want 14100000 (the genuinely-written frequency)", found["freq_hz"])
	}

	// The readback ran under pairCtx AFTER the failure — two more
	// exchanges (MR+MT, 129/130) beyond the faulted MT at 128. Confirmed
	// indirectly above via the journal event and SlotState; no further
	// deltas ("005", "010") were attempted.
	for _, slot := range []string{"005", "010"} {
		if slot == "005" {
			st, ok := radio.SlotState(slot)
			if !ok || st.Tag == "MOD-TWO" {
				t.Errorf("SlotState(%q) = %+v, ok=%v, want the untouched factory value", slot, st, ok)
			}
			continue
		}
		if _, ok := radio.SlotState(slot); ok {
			t.Errorf("SlotState(%q) has recorded state, want none — never attempted", slot)
		}
	}
}

// TestExecute_InertClarifier_UnchangedFlowsCleanly is the amended M5b
// brief's named end-to-end proof: a TAG-ONLY edit on a channel whose
// clarifier is zero (matching its baseline) must write AND verify clean
// through the whole clone choreography, even though the clarifier's
// Write support is spec.Inert (HW-CONFIRMED ignored by the radio) and
// the MW frame transmits it regardless. This is the property whose
// absence made the ORIGINAL (pre-amendment) "clarifier Write:
// Unsupported" remedy block every write in the repository: the
// unchanged Inert clarifier transmits the baseline value (zero),
// fakeradio — like the radio — ignores it and stores zeros, the
// read-back matches, and obligation 7's verify needs no change at all.
func TestExecute_InertClarifier_UnchangedFlowsCleanly(t *testing.T) {
	radio, sess := openSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	svc := NewService(sess, newStore(t), WithNow(stepClock(fixedNow)))

	caps := sess.Capabilities()
	tagOnly := writableChannel("001", 7_000_000, "RENAMED").Data
	tagOnly.Mode = "LSB" // match the baseline exactly except the tag
	file := matchingCandidateFile(caps, minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": tagOnly,
	})
	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	report, err := svc.Execute(testCtx(t), plan, plan.ConfirmationDigest(), ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if report.Written != 1 || report.Verified != 1 {
		t.Errorf("Written/Verified = %d/%d, want 1/1 (a tag-only edit with an unchanged zero clarifier must write and VERIFY clean)", report.Written, report.Verified)
	}
	if report.SkippedBlocked != 0 {
		t.Errorf("SkippedBlocked = %d, want 0 (unchanged Inert clarifier must not block)", report.SkippedBlocked)
	}
	if report.Aborted {
		t.Errorf("Aborted = true (reason %q), want false", report.AbortReason)
	}
	if st, ok := radio.SlotState("001"); !ok || st.Tag != "RENAMED" {
		t.Errorf("SlotState(\"001\") = %+v, ok=%v, want the renamed tag written through", st, ok)
	}
}

// TestExecute_LiveBugRepro_UnpaddedTagWriteReadBackPadded reproduces, at
// wire-frame fidelity, the exact live production defect this task fixes
// (docs/fixtures-private, 13/07/2026, first real-radio production
// write): a hand-edited candidate carries an UNPADDED tag ("PROD TEST"),
// Validate passes, Diff shows a legitimate tag change, the MW+MT write
// succeeds on the wire — and the FT-710 is HW-CONFIRMED to answer the
// verify read-back's MT query with that SAME tag space-padded to the
// full 12-byte field ("PROD TEST   "). Pre-fix, comparing the unpadded
// candidate against the padded read-back aborts Execute with exactly
// `clone: slot "001": read-back disagrees on tag` (ErrVerifyMismatch);
// post-fix (ParseMTAnswer trims trailing spaces on parse), the
// comparison is trimmed==trimmed and the write verifies clean.
//
// fakeradio's own default wire simulation deliberately does not model
// this radio-side padding quirk (internal/fakeradio/doc.go register item
// 8) — reproducing it end to end without touching that shared behaviour
// (and the many other tests that depend on it staying exactly as it is)
// uses tagPadWireInterceptor to rewrite this ONE wire reply, below
// cat.Dialect.ParseMTAnswer, to what the live radio is now confirmed
// to send.
func TestExecute_LiveBugRepro_UnpaddedTagWriteReadBackPadded(t *testing.T) {
	wantFrame := []byte("MT0011PROD TEST;")
	padFrame := []byte("MT0011PROD TEST   ;")
	radio, sess := openSimSessionWithWirePad(t, wantFrame, padFrame, fakeradio.WithFactoryImage(minimalFactoryImage))
	svc := NewService(sess, newStore(t), WithNow(stepClock(fixedNow)))

	caps := sess.Capabilities()
	tagOnly := writableChannel("001", 7_000_000, "PROD TEST").Data
	tagOnly.Mode = "LSB" // match the baseline exactly except the tag — isolate the tag as the only intended change
	file := matchingCandidateFile(caps, minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": tagOnly,
	})
	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	report, err := svc.Execute(testCtx(t), plan, plan.ConfirmationDigest(), ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v (live production bug: an unpadded tag edit must not abort verify — the radio's own trailing-space padding is a wire-encoding detail, not a model-level tag difference)", err)
	}
	if report.Aborted {
		t.Errorf("Aborted = true (reason %q), want false", report.AbortReason)
	}
	if report.Written != 1 || report.Verified != 1 {
		t.Errorf("Written/Verified = %d/%d, want 1/1", report.Written, report.Verified)
	}
	if st, ok := radio.SlotState("001"); !ok || st.Tag != "PROD TEST" {
		t.Errorf("SlotState(\"001\") = %+v, ok=%v, want the unpadded tag written through unchanged on the wire", st, ok)
	}
}

// TestExecute_TagClear_EndToEnd pins the tag-CLEAR path end to end
// (third production bug of the tag fix wave, HW-probed 13/07/2026,
// docs/fixtures-private/ stages tagclear/tagclear2): a baseline channel
// with a non-empty tag, a candidate that clears it (Tag == ""), a real
// write, and a verify read-back that must agree the tag is now empty.
//
// The FT-710 REJECTS a zero-byte-tag MT Set ("MT0010;" — exactly what
// BuildMTSet used to emit for Tag == "") with "?;" (~4 ms live) and the
// old tag SURVIVES; its one proven clear mechanism is the all-spaces
// 12-byte tag. fakeradio now mirrors that rejection, so against the OLD
// 0-byte encoding this test aborts at the write ("MT rejected by
// radio") — with the spaces-form encoding (cat.Dialect.BuildMTSet,
// this fix) the write is accepted, the read-back echoes all spaces,
// ParseMTAnswer trims it to "", and the verify compares "" == ""
// clean.
func TestExecute_TagClear_EndToEnd(t *testing.T) {
	// minimalFactoryImage's M-01, plus the non-empty tag to be cleared.
	taggedImage := minimalFactoryImage()
	m01 := taggedImage["001"]
	m01.Tag = "OLD TAG"
	taggedImage["001"] = m01

	radio, sess := openSimSession(t, fakeradio.WithFactoryImage(func() map[string]fakeradio.MemState { return taggedImage }))
	svc := NewService(sess, newStore(t), WithNow(stepClock(fixedNow)))

	caps := sess.Capabilities()
	cleared := writableChannel("001", 7_000_000, "").Data // Tag "", TagDisplay false
	cleared.Mode = "LSB"                                  // match the baseline exactly except the tag
	file := matchingCandidateFile(caps, minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": cleared,
	})
	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	report, err := svc.Execute(testCtx(t), plan, plan.ConfirmationDigest(), ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v (tag-clear must use the radio's accepted all-spaces form, not the rejected 0-byte MT Set)", err)
	}
	if report.Aborted {
		t.Errorf("Aborted = true (reason %q), want false", report.AbortReason)
	}
	if report.Written != 1 || report.Verified != 1 {
		t.Errorf("Written/Verified = %d/%d, want 1/1", report.Written, report.Verified)
	}
	// Wire-level proof the CLEAR form landed: the fake stores exactly
	// the tag bytes it accepted — the all-spaces 12-byte clear, not ""
	// (which it could now hold only if the rejected 0-byte form had
	// somehow been sent AND accepted).
	if st, ok := radio.SlotState("001"); !ok || st.Tag != "            " {
		t.Errorf("SlotState(\"001\").Tag = %q, ok=%v, want 12 spaces (the radio's accepted tag-clear form, stored verbatim by the fake)", st.Tag, ok)
	}
}

// TestExecute_InertClarifier_ChangedIsSkippedBlocked: the other half of
// the Inert rule end-to-end — a candidate that CHANGES the clarifier
// (an intent the radio provably ignores) yields a Blocked diff entry
// with the precise Inert reason, which Execute skips without a single
// write. Identical behaviour on --fake and against the real radio,
// since both profiles mark the clarifier Inert.
func TestExecute_InertClarifier_ChangedIsSkippedBlocked(t *testing.T) {
	radio, sess := openSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	svc := NewService(sess, newStore(t), WithNow(stepClock(fixedNow)))

	caps := sess.Capabilities()
	clarEdit := writableChannel("001", 7_000_000, "").Data
	clarEdit.Mode = "LSB"
	clarEdit.ClarHz = 100 // the ONLY change vs the baseline
	file := matchingCandidateFile(caps, minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": clarEdit,
	})
	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	report, err := svc.Execute(testCtx(t), plan, plan.ConfirmationDigest(), ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if report.Written != 0 || report.Verified != 0 {
		t.Errorf("Written/Verified = %d/%d, want 0/0 (a changed clarifier cannot be honoured and must never be written)", report.Written, report.Verified)
	}
	if report.SkippedBlocked != 1 {
		t.Errorf("SkippedBlocked = %d, want 1", report.SkippedBlocked)
	}
	found := false
	for _, sr := range report.Slots {
		if sr.Slot == "001" && sr.Action == actionSkippedBlocked {
			found = true
			if !strings.Contains(sr.Detail, "clarifier changes are ignored by the radio") {
				t.Errorf("SlotResult.Detail = %q, want the Inert clarifier reason", sr.Detail)
			}
		}
	}
	if !found {
		t.Error("no skipped-blocked SlotResult for \"001\"")
	}
	// The baseline value must be untouched on the radio.
	if st, ok := radio.SlotState("001"); !ok || st.ClarMag != "0000" || st.ClarSign != '+' {
		t.Errorf("SlotState(\"001\") clarifier = %+v, ok=%v, want the untouched zero baseline", st, ok)
	}
}

// pmsSeedSlot is a populated PMS MemState used ONLY to overlay "P1L" back
// onto the (Codex M5b fix wave, Fix 3) now-empty-by-default PMS bank, for
// the memory-selection tests below: fakeradio rejects an MC-set (recall)
// targeting an EMPTY slot (the documented, still-not-probed "MC-set of an
// empty slot" behaviour — docs/hardware-notes.md's "Explicitly not
// probed" list), so seeding the selection onto a slot distinct from the
// three delta targets ("001"/"005"/"010") needs it populated.
var pmsSeedSlot = fakeradio.MemState{
	Freq: "014000000", ClarSign: '+', ClarMag: "0000",
	Mode: '2', Kind: '1', CTCSS: '0', Shift: '0',
	Populated: true,
}

// TestExecute_MemorySelection_RestoredAfterSuccess (obligation 12,
// doc.go): HW-CONFIRMED 2026-07-13 (M5b write trials,
// docs/hardware-notes.md) — MW moves the radio's selection to the
// written slot. A 3-delta Execute writes "001", "005", "010" in turn, so
// WITHOUT obligation 12's restore the radio would be left parked on
// "010" (the last write) when the run finishes. The session is first
// seeded onto "P1L" (via RecallMemory, NOT one of the three delta slots)
// so the restored value is unambiguous proof of the restore, not a
// coincidence of the last write happening to match — P1L is overlaid
// populated (pmsSeedSlot) purely so this seed recall itself succeeds.
func TestExecute_MemorySelection_RestoredAfterSuccess(t *testing.T) {
	svc, radio, sess, plan, digest := preparedThreeDeltaPlan(t, fakeradio.WithSlot("P1L", pmsSeedSlot))

	selector, ok := sess.(MemorySelector)
	if !ok {
		t.Fatal("session does not implement MemorySelector")
	}
	if err := selector.RecallMemory(testCtx(t), "P1L"); err != nil {
		t.Fatalf("RecallMemory(seed \"P1L\"): unexpected error: %v", err)
	}
	if got := radio.CurrentChannel(); got != "P1L" {
		t.Fatalf("seed RecallMemory did not move the selection: got %q", got)
	}

	report, err := svc.Execute(testCtx(t), plan, digest, ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if report.Written != 3 {
		t.Fatalf("Written = %d, want 3", report.Written)
	}

	if got := radio.CurrentChannel(); got != "P1L" {
		t.Errorf("CurrentChannel() after Execute = %q, want \"P1L\" (obligation 12: restored to the pre-write snapshot, not left on \"010\", the last written slot)", got)
	}

	events := readJournalEventDetails(t, report.JournalPath)
	var sawSnapshot, sawRestore bool
	for _, e := range events {
		if e.event == "mc_snapshot" {
			sawSnapshot = true
		}
		if e.event == "mc_restore" {
			sawRestore = true
		}
	}
	if !sawSnapshot {
		t.Error("journal has no \"mc_snapshot\" event")
	}
	if !sawRestore {
		t.Error("journal has no \"mc_restore\" event")
	}
}

// TestExecute_MemorySelection_RestoredAfterAbort (obligation 12): the
// restore must happen on the way out of an ABORTED Execute too, not just
// a clean one — the defer registered right after the snapshot covers
// every exit from the delta-write loop. The second delta's MW draws a
// delayed rejection (mirroring TestExecute_Abort_WriteRejected's own
// fault, shifted +2 exchanges versus that test's 133: +1 because P1L is
// overlaid POPULATED here (pmsSeedSlot — an MC-set to an empty slot is
// rejected, see pmsSeedSlot's doc comment), turning its ReadAll exchange
// from 1 (empty) into 2 (MR+MT); +1 more for this test's own seed
// RecallMemory, landing before the delta loop begins).
func TestExecute_MemorySelection_RestoredAfterAbort(t *testing.T) {
	const secondWriteMWExchange = 135
	svc, radio, sess, plan, digest := preparedThreeDeltaPlan(t,
		fakeradio.WithSlot("P1L", pmsSeedSlot),
		fakeradio.WithFault(fakeradio.FaultDelayedRejection(secondWriteMWExchange, 50*time.Millisecond)),
	)

	selector, ok := sess.(MemorySelector)
	if !ok {
		t.Fatal("session does not implement MemorySelector")
	}
	if err := selector.RecallMemory(testCtx(t), "P1L"); err != nil {
		t.Fatalf("RecallMemory(seed \"P1L\"): unexpected error: %v", err)
	}

	report, err := svc.Execute(testCtx(t), plan, digest, ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err == nil {
		t.Fatal("Execute = nil error, want an abort (the 2nd write was rejected)")
	}
	if !errors.Is(err, ErrAborted) {
		t.Fatalf("errors.Is(err, ErrAborted) = false; err = %v", err)
	}
	if report.Written != 1 {
		t.Fatalf("Written = %d, want 1 (only the first delta, \"001\") — the fault's exchange number may need adjusting if this fails", report.Written)
	}

	if got := radio.CurrentChannel(); got != "P1L" {
		t.Errorf("CurrentChannel() after an ABORTED Execute = %q, want \"P1L\" (obligation 12: restored even on abort)", got)
	}

	events := readJournalEventDetails(t, report.JournalPath)
	var sawRestore bool
	for _, e := range events {
		if e.event == "mc_restore" {
			sawRestore = true
		}
	}
	if !sawRestore {
		t.Error("journal has no \"mc_restore\" event after an abort")
	}
}

// TestExecute_JournalFailure_WriteAttempt_RefusesBeforeWire (Fix 10b, the
// ratified journal fail-safe policy — doc.go): when the write_attempt
// line itself cannot be persisted, Execute must refuse BEFORE ever
// calling WriteChannel for that slot — at that instant nothing has
// touched the radio, so the refusal is free. Proven by SlotState: the
// radio's stored value for "001" is left exactly as PrepareSend found it
// (a countingPort's raw byte count is NOT the right proof here — the
// verify-read phase legitimately sends its own MR frame, a wire Write(),
// before the delta-write loop is even reached; what must be zero is
// WriteChannel calls specifically, and SlotState's content is the
// clearest witness of that).
func TestExecute_JournalFailure_WriteAttempt_RefusesBeforeWire(t *testing.T) {
	radio, sess := openSimSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	store := newStore(t)
	svc := NewService(sess, store, WithNow(stepClock(fixedNow)))

	caps := sess.Capabilities()
	file := matchingCandidateFile(caps, minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 14_150_000, "MODIFIED").Data,
	})
	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	// Inject a journal that fails specifically on the write_attempt line.
	svc.openJournal = func(path string) journalAppender {
		return &failingJournal{inner: store.OpenJournal(path), failOn: "write_attempt"}
	}

	report, err := svc.Execute(testCtx(t), plan, plan.ConfirmationDigest(), ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err == nil {
		t.Fatal("Execute = nil error, want an abort (write_attempt journal append failed)")
	}
	if !errors.Is(err, ErrJournalFailed) {
		t.Errorf("errors.Is(err, ErrJournalFailed) = false; err = %v", err)
	}
	if !errors.Is(err, ErrAborted) {
		t.Errorf("errors.Is(err, ErrAborted) = false; err = %v", err)
	}
	var jfe *JournalFailedError
	if !errors.As(err, &jfe) {
		t.Fatalf("error %v is not a *JournalFailedError", err)
	}
	if jfe.Event != "write_attempt" {
		t.Errorf("JournalFailedError.Event = %q, want \"write_attempt\"", jfe.Event)
	}
	if jfe.Slot != "001" {
		t.Errorf("JournalFailedError.Slot = %q, want \"001\"", jfe.Slot)
	}
	if st, ok := radio.SlotState("001"); !ok || st.Tag == "MODIFIED" {
		t.Errorf("SlotState(\"001\") = %+v, ok=%v, want the untouched factory value — WriteChannel must never have been called", st, ok)
	}
	if report == nil {
		t.Fatal("report is nil, want a non-nil partial report")
	}
	if report.Written != 0 {
		t.Errorf("Written = %d, want 0", report.Written)
	}
	if !report.Aborted {
		t.Error("Aborted = false, want true")
	}
}

// TestExecute_JournalFailure_WriteResult_AbortsFurtherWrites (Fix 10b,
// the ratified journal fail-safe policy — doc.go): a journal-append
// failure recorded AFTER a wire write (write_result) cannot un-write it —
// the slot's write still counts — but it aborts all FURTHER writes via
// the same abort machinery a write rejection or verify mismatch uses.
func TestExecute_JournalFailure_WriteResult_AbortsFurtherWrites(t *testing.T) {
	svc, radio, _, plan, digest := preparedThreeDeltaPlan(t)

	// Reconstruct a SnapshotStore over the same directory PrepareSend
	// already used (derivable from the plan's own SnapshotPath — no need
	// for preparedThreeDeltaPlan to hand back its internal store), and
	// wrap its journal to fail on write_result only.
	realStore := SnapshotStore{Dir: filepath.Dir(plan.SnapshotPath())}
	svc.openJournal = func(path string) journalAppender {
		return &failingJournal{inner: realStore.OpenJournal(path), failOn: "write_result"}
	}

	report, err := svc.Execute(testCtx(t), plan, digest, ExecuteOptions{FirmwareConfirmed: "1.0"})
	if err == nil {
		t.Fatal("Execute = nil error, want an abort (write_result journal append failed)")
	}
	if !errors.Is(err, ErrJournalFailed) {
		t.Errorf("errors.Is(err, ErrJournalFailed) = false; err = %v", err)
	}
	if !errors.Is(err, ErrAborted) {
		t.Errorf("errors.Is(err, ErrAborted) = false; err = %v", err)
	}
	if report == nil {
		t.Fatal("report is nil, want a non-nil partial report")
	}
	// The first delta's ("001") wire write DID happen — a journal-append
	// failure recorded after it cannot un-write it, so it still counts.
	if report.Written != 1 {
		t.Errorf("Written = %d, want 1 (the first delta's write happened before the journal failed to record it)", report.Written)
	}
	if !report.Aborted {
		t.Error("Aborted = false, want true")
	}
	if report.AbortReason == "" {
		t.Error("AbortReason is empty, want the journal failure named as the reason")
	}
	// Fix 1 (adjudicated HIGH): the ORIGINAL cause must survive — still
	// the write_result journal failure itself, not anything the mandatory
	// post-failure readback below observed.
	var jfe *JournalFailedError
	if !errors.As(err, &jfe) {
		t.Fatalf("error %v is not a *JournalFailedError", err)
	}
	if jfe.Event != "write_result" {
		t.Errorf("JournalFailedError.Event = %q, want \"write_result\" (the ORIGINAL cause, unchanged by the readback)", jfe.Event)
	}

	// Fix 1: even though the JOURNAL failed to record write_result, the
	// wire write itself succeeded — writePair must still attempt a
	// same-slot readback under pairCtx and journal what it found (this
	// journal event is NOT "write_result", so failingJournal lets it
	// through).
	records := readJournalRecords(t, report.JournalPath)
	var found map[string]any
	for _, rec := range records {
		if rec["event"] == "post_failure_readback" && rec["slot"] == "001" {
			found = rec
			break
		}
	}
	if found == nil {
		t.Fatalf("journal has no \"post_failure_readback\" event for slot \"001\" — readback must ALWAYS be attempted once WriteChannel has been invoked; records = %v", records)
	}
	if ok, _ := found["ok"].(bool); !ok {
		t.Errorf("post_failure_readback[\"ok\"] = %v, want true (the readback itself succeeded)", found["ok"])
	}
	if tag, _ := found["tag"].(string); tag != "MOD-ONE" {
		t.Errorf("post_failure_readback[\"tag\"] = %q, want \"MOD-ONE\" (the genuinely-written value, confirming a real readback ran)", found["tag"])
	}

	if st, ok := radio.SlotState("001"); !ok || st.Tag != "MOD-ONE" {
		t.Errorf("SlotState(\"001\") = %+v, ok=%v, want the written value (its write happened before the journal failure aborted the run)", st, ok)
	}
	// Neither the second nor third delta must ever have been attempted:
	// the abort happens immediately after the first delta's write_result
	// fails to append, before delta2 ("005") is even reached. "005"
	// pre-exists in happyPathImage (the factory image), so its mere
	// presence in SlotState proves nothing — it must still show its
	// UNTOUCHED factory value, not the candidate's "MOD-TWO" edit. "010"
	// never existed at all, so for it, no recorded state is the proof.
	if st, ok := radio.SlotState("005"); !ok || st.Tag == "MOD-TWO" {
		t.Errorf("SlotState(\"005\") = %+v, ok=%v, want the untouched factory value — never attempted", st, ok)
	}
	if _, ok := radio.SlotState("010"); ok {
		t.Error("SlotState(\"010\") has recorded state, want none — never attempted")
	}
}

// journalEventDetail is one journal line's event name and (when present)
// slot, for the abort tests' more detailed journal assertions.
type journalEventDetail struct {
	event string
	slot  string
}

// readJournalEventDetails is readJournalEvents, but also capturing each
// line's "slot" field.
func readJournalEventDetails(t *testing.T, path string) []journalEventDetail {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open journal %s: %v", path, err)
	}
	defer f.Close()

	var out []journalEventDetail
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			t.Fatalf("journal line does not parse as JSON: %v (%q)", err, string(line))
		}
		ev, _ := rec["event"].(string)
		slot, _ := rec["slot"].(string)
		out = append(out, journalEventDetail{event: ev, slot: slot})
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanning journal: %v", err)
	}
	return out
}

// baseVerifyChannelData is a fully-populated, self-consistent
// codeplug.ChannelData used as the "want" (and, per test case, mutated
// copy as "got") baseline for TestWritableFieldsMismatch_Table and
// TestVerifyWriteResult — every WRITABLE field set to a plain value, and
// CTCSSTone/ScanSkip left Unknown (as any real write/verify pair's
// values are, per writableFieldsMismatch's own doc comment).
func baseVerifyChannelData() codeplug.ChannelData {
	return codeplug.ChannelData{
		FreqHz: 14_100_000, Mode: "USB", CTCSS: "OFF", Shift: "SIMPLEX",
		Tag: "BASE", TagDisplay: true,
		CTCSSTone: codeplug.ToneField{State: codeplug.Unknown},
		ScanSkip:  codeplug.BoolField{State: codeplug.Unknown},
	}
}

// TestWritableFieldsMismatch_Table (Fix 10b, minor — verify-mismatch
// coverage): writableFieldsMismatch is the sole decision point behind
// obligation 7's write-then-verify comparison. This table exhaustively
// covers each writable field diverging independently, several diverging
// together, and confirms CTCSSTone/ScanSkip are excluded even when they
// genuinely differ (both always read back Unknown by construction — see
// writableFieldsMismatch's own doc comment for why comparing them would
// manufacture a false mismatch on every write).
func TestWritableFieldsMismatch_Table(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*codeplug.ChannelData)
		want   []spec.Field
	}{
		{"identical", func(*codeplug.ChannelData) {}, nil},
		{"frequency differs", func(c *codeplug.ChannelData) { c.FreqHz++ }, []spec.Field{spec.FieldFrequency}},
		{"mode differs", func(c *codeplug.ChannelData) { c.Mode = "LSB" }, []spec.Field{spec.FieldMode}},
		{"clarifier ClarHz differs", func(c *codeplug.ChannelData) { c.ClarHz = 10 }, []spec.Field{spec.FieldClarifier}},
		{"clarifier RxClar differs", func(c *codeplug.ChannelData) { c.RxClar = true }, []spec.Field{spec.FieldClarifier}},
		{"clarifier TxClar differs", func(c *codeplug.ChannelData) { c.TxClar = true }, []spec.Field{spec.FieldClarifier}},
		{"ctcss differs", func(c *codeplug.ChannelData) { c.CTCSS = "ENC" }, []spec.Field{spec.FieldCTCSSState}},
		{"shift differs", func(c *codeplug.ChannelData) { c.Shift = "PLUS" }, []spec.Field{spec.FieldShift}},
		{"tag differs", func(c *codeplug.ChannelData) { c.Tag = "OTHER" }, []spec.Field{spec.FieldTag}},
		{"tagDisplay differs", func(c *codeplug.ChannelData) { c.TagDisplay = false }, []spec.Field{spec.FieldTagDisplay}},
		{"CTCSSTone differs — excluded (read back Unknown by construction)", func(c *codeplug.ChannelData) {
			c.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 100}
		}, nil},
		{"ScanSkip differs — excluded (read back Unknown by construction)", func(c *codeplug.ChannelData) {
			c.ScanSkip = codeplug.BoolField{State: codeplug.Known, Value: true}
		}, nil},
		{"frequency and tag differ together", func(c *codeplug.ChannelData) {
			c.FreqHz++
			c.Tag = "OTHER"
		}, []spec.Field{spec.FieldFrequency, spec.FieldTag}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := baseVerifyChannelData()
			got := baseVerifyChannelData()
			tt.mutate(&got)
			bad := writableFieldsMismatch(want, got)
			if !reflect.DeepEqual(bad, tt.want) {
				t.Errorf("writableFieldsMismatch(want, got) = %v, want %v", bad, tt.want)
			}
		})
	}
}

// TestVerifyWriteResult (Fix 10b, minor — verify-mismatch coverage): the
// unit-level route this fix took instead of a fakeradio SetSlot mid-
// session hook (see the fix wave's rationale — no fault primitive
// produces a genuinely-different-but-successfully-read-back verify
// without a stateful hook; the decision logic itself is what needs
// covering). Drives verifyWriteResult directly with constructed inputs
// covering populated-vs-different-values AND populated-vs-empty,
// asserting both the *VerifyMismatchError contents and the SlotResult
// (Execute's "abort accounting") it produces.
func TestVerifyWriteResult(t *testing.T) {
	t.Run("clean match", func(t *testing.T) {
		data := baseVerifyChannelData()
		sent := codeplug.Channel{Slot: "001", Data: &data}
		readBack := data
		verify := codeplug.Channel{Slot: "001", Data: &readBack}

		mismatchErr, slotResult := verifyWriteResult("001", sent, verify)
		if mismatchErr != nil {
			t.Errorf("mismatchErr = %v, want nil", mismatchErr)
		}
		want := SlotResult{Slot: "001", Action: actionWrite, VerifyOK: true}
		if slotResult != want {
			t.Errorf("slotResult = %+v, want %+v", slotResult, want)
		}
	})

	t.Run("populated-vs-different-values", func(t *testing.T) {
		sentData := baseVerifyChannelData()
		sent := codeplug.Channel{Slot: "001", Data: &sentData}
		readBackData := baseVerifyChannelData()
		readBackData.FreqHz = 21_000_000
		readBackData.Tag = "DRIFTED"
		verify := codeplug.Channel{Slot: "001", Data: &readBackData}

		mismatchErr, slotResult := verifyWriteResult("001", sent, verify)
		if mismatchErr == nil {
			t.Fatal("mismatchErr = nil, want a *VerifyMismatchError")
		}
		if !errors.Is(mismatchErr, ErrVerifyMismatch) {
			t.Error("errors.Is(mismatchErr, ErrVerifyMismatch) = false")
		}
		wantFields := []spec.Field{spec.FieldFrequency, spec.FieldTag}
		if !reflect.DeepEqual(mismatchErr.Fields, wantFields) {
			t.Errorf("mismatchErr.Fields = %v, want %v", mismatchErr.Fields, wantFields)
		}
		wantMsg := `clone: slot "001": read-back disagrees on frequency, tag`
		if mismatchErr.Error() != wantMsg {
			t.Errorf("mismatchErr.Error() = %q, want %q", mismatchErr.Error(), wantMsg)
		}
		wantResult := SlotResult{Slot: "001", Action: actionWrite, VerifyOK: false, Detail: wantMsg}
		if slotResult != wantResult {
			t.Errorf("slotResult = %+v, want %+v", slotResult, wantResult)
		}
	})

	t.Run("populated-vs-empty", func(t *testing.T) {
		sentData := baseVerifyChannelData()
		sent := codeplug.Channel{Slot: "001", Data: &sentData}
		verify := codeplug.Channel{Slot: "001", Data: nil} // read back as empty/erased

		mismatchErr, slotResult := verifyWriteResult("001", sent, verify)
		if mismatchErr == nil {
			t.Fatal("mismatchErr = nil, want a *VerifyMismatchError")
		}
		if len(mismatchErr.Fields) != 0 {
			t.Errorf("mismatchErr.Fields = %v, want empty (the read-back-empty case names no specific fields)", mismatchErr.Fields)
		}
		wantMsg := `clone: slot "001": read-back shows an empty slot after a populated write`
		if mismatchErr.Error() != wantMsg {
			t.Errorf("mismatchErr.Error() = %q, want %q", mismatchErr.Error(), wantMsg)
		}
		wantResult := SlotResult{Slot: "001", Action: actionWrite, VerifyOK: false, Detail: wantMsg}
		if slotResult != wantResult {
			t.Errorf("slotResult = %+v, want %+v", slotResult, wantResult)
		}
	})
}
