// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft710"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// scriptedVFOSession wraps a real, opened *ft710.Session (embedding the
// concrete type, not driver.Session, so its real ReadChannel/Identity/
// Capabilities/Close and its real MemorySelector methods are all
// promoted unchanged) with a SCRIPTED VFOStateRestorer and a call-order
// trace across every method Execute's VFOStateRestorer/MemorySelector
// bracket touches.
//
// This is the "scripted fake Session" Phase 2's plan calls for
// (1-plan-adjudication.md finding 4): the six lifecycle-ordering
// properties below are about the SEQUENCE Execute calls these methods
// in, which a real driver's own frame-level choreography cannot
// demonstrate any more clearly than a recorded call order can — and
// per-frame residual-state proof (which of the FT-890/900 driver's eight
// wire frames actually ran) is explicitly Phase 3's job, against a real
// driver, not this package's.
type scriptedVFOSession struct {
	*ft710.Session
	trace *[]string

	snapshotErr error
	restoreErr  error
	snapshot    VFOSnapshot

	onWriteChannel func()
}

func (s *scriptedVFOSession) SnapshotVFOState(context.Context) (VFOSnapshot, error) {
	*s.trace = append(*s.trace, "SnapshotVFO")
	if s.snapshotErr != nil {
		return VFOSnapshot{}, s.snapshotErr
	}
	return s.snapshot, nil
}

func (s *scriptedVFOSession) RestoreVFOState(context.Context, VFOSnapshot) error {
	*s.trace = append(*s.trace, "RestoreVFO")
	return s.restoreErr
}

// CurrentMemory/RecallMemory shadow the embedded *ft710.Session's real
// (promoted) implementation purely to add tracing — behaviour is
// otherwise unchanged, delegating straight through.
func (s *scriptedVFOSession) CurrentMemory(ctx context.Context) (string, error) {
	*s.trace = append(*s.trace, "CurrentMemory")
	return s.Session.CurrentMemory(ctx)
}

func (s *scriptedVFOSession) RecallMemory(ctx context.Context, slot string) error {
	*s.trace = append(*s.trace, "RecallMemory")
	return s.Session.RecallMemory(ctx, slot)
}

// WriteChannel shadows the embedded session's real implementation to
// trace it too (so "writes" has a visible position in the trace) and to
// let a test hook a side effect (e.g. cancelling the caller's ctx) onto
// exactly the moment a wire write completes.
func (s *scriptedVFOSession) WriteChannel(ctx context.Context, ch codeplug.Channel) (driver.WriteResult, error) {
	*s.trace = append(*s.trace, "WriteChannel")
	res, err := s.Session.WriteChannel(ctx, ch)
	if s.onWriteChannel != nil {
		s.onWriteChannel()
	}
	return res, err
}

// openScriptedVFOSession opens a Simulated ft710 session exactly like
// openSimSession, then wraps it as a *scriptedVFOSession — asserting out
// the concrete *ft710.Session that ft710Driver.Open always returns
// dynamically (ft710.go), behind the driver.Session interface
// openSimSession's own signature returns.
func openScriptedVFOSession(t *testing.T, opts ...fakeradio.Option) (*fakeradio.Radio, *scriptedVFOSession, *[]string) {
	t.Helper()
	radio, sess := openSimSession(t, opts...)
	real, ok := sess.(*ft710.Session)
	if !ok {
		t.Fatalf("openSimSession returned %T, want *ft710.Session", sess)
	}
	trace := new([]string)
	return radio, &scriptedVFOSession{Session: real, trace: trace}, trace
}

// TestExecute_VFOStateRestorer_NoDeltas_NeitherCalled: a plan with no
// unblocked Added/Modified entries returns before the delta-write loop
// (the existing len(deltas)==0 early return) — neither SnapshotVFOState,
// RestoreVFOState, nor MemorySelector's CurrentMemory/RecallMemory
// should ever be reached.
func TestExecute_VFOStateRestorer_NoDeltas_NeitherCalled(t *testing.T) {
	_, scripted, trace := openScriptedVFOSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	svc := NewService(scripted, newStore(t), WithNow(stepClock(fixedNow)))

	file := matchingCandidateFile(scripted.Capabilities(), minimalFactoryPopulated(), nil)
	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}
	if d := plan.Diff(); d.Added != 0 || d.Modified != 0 {
		t.Fatalf("candidate has Added=%d Modified=%d, want a no-op candidate", d.Added, d.Modified)
	}

	if _, err := svc.Execute(testCtx(t), plan, plan.ConfirmationDigest()); err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	if got := *trace; len(got) != 0 {
		t.Errorf("trace = %v, want empty — no deltas means the VFOStateRestorer/MemorySelector bracket is never reached", got)
	}
}

// TestExecute_VFOStateRestorer_SnapshotFailure_AbortsBeforeFirstMutation:
// unlike MemorySelector, a SnapshotVFOState error is fatal — Execute
// must abort before calling CurrentMemory or WriteChannel at all (this
// family's write choreography overwrites VFO-A's live content; there is
// no safe "proceed without a snapshot" path).
func TestExecute_VFOStateRestorer_SnapshotFailure_AbortsBeforeFirstMutation(t *testing.T) {
	radio, scripted, trace := openScriptedVFOSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	scripted.snapshotErr = errors.New("radio busy")
	svc := NewService(scripted, newStore(t), WithNow(stepClock(fixedNow)))

	file := matchingCandidateFile(scripted.Capabilities(), minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 14_150_000, "MODIFIED").Data,
	})
	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	report, err := svc.Execute(testCtx(t), plan, plan.ConfirmationDigest())
	if err == nil {
		t.Fatal("Execute = nil error, want an abort (VFO snapshot failed)")
	}
	if !errors.Is(err, ErrAborted) {
		t.Errorf("errors.Is(err, ErrAborted) = false; err = %v", err)
	}
	if report == nil || !report.Aborted {
		t.Fatalf("report = %+v, want a non-nil Aborted report", report)
	}
	if report.Written != 0 {
		t.Errorf("Written = %d, want 0", report.Written)
	}
	if want := []string{"SnapshotVFO"}; !reflect.DeepEqual(*trace, want) {
		t.Errorf("trace = %v, want %v — no CurrentMemory, no WriteChannel after a failed VFO snapshot", *trace, want)
	}
	if st, ok := radio.SlotState("001"); !ok || st.Tag == "MODIFIED" {
		t.Errorf("SlotState(\"001\") = %+v, ok=%v — WriteChannel must never have been called", st, ok)
	}
}

// TestExecute_VFOStateRestorer_BothInterfaces_ExactOrder: with both
// VFOStateRestorer and MemorySelector present, one successful delta
// produces the exact sequence SnapshotVFO -> CurrentMemory -> WriteChannel
// -> RecallMemory -> RestoreVFO (§Write model: "VFOStateRestorer brackets
// MemorySelector"). The session is seeded onto a populated slot first
// (mirroring TestExecute_MemorySelection_RestoredAfterSuccess) so
// CurrentMemory returns a usable, non-"000" snapshot and RecallMemory is
// actually reached.
func TestExecute_VFOStateRestorer_BothInterfaces_ExactOrder(t *testing.T) {
	_, scripted, trace := openScriptedVFOSession(t, fakeradio.WithFactoryImage(minimalFactoryImage), fakeradio.WithSlot("P1L", pmsSeedSlot))
	if err := scripted.RecallMemory(testCtx(t), "P1L"); err != nil {
		t.Fatalf("RecallMemory(seed \"P1L\"): unexpected error: %v", err)
	}
	*trace = nil // discard the seed recall's own trace entry — only Execute's own calls matter here

	svc := NewService(scripted, newStore(t), WithNow(stepClock(fixedNow)))
	file := matchingCandidateFile(scripted.Capabilities(), minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 14_150_000, "MODIFIED").Data,
	})
	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	if _, err := svc.Execute(testCtx(t), plan, plan.ConfirmationDigest()); err != nil {
		t.Fatalf("Execute: unexpected error: %v", err)
	}
	want := []string{"SnapshotVFO", "CurrentMemory", "WriteChannel", "RecallMemory", "RestoreVFO"}
	if !reflect.DeepEqual(*trace, want) {
		t.Errorf("trace = %v, want %v", *trace, want)
	}
}

// TestExecute_VFOStateRestorer_RestoreCalledOnEveryExit: every exit after
// a successful SnapshotVFOState must invoke RestoreVFOState — both a
// clean completion and an aborted run (here, a journal-append failure on
// write_attempt, the same abort machinery TestExecute_JournalFailure_*
// uses, chosen because it aborts deterministically without needing a
// wire fault).
func TestExecute_VFOStateRestorer_RestoreCalledOnEveryExit(t *testing.T) {
	t.Run("clean completion", func(t *testing.T) {
		_, scripted, trace := openScriptedVFOSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
		svc := NewService(scripted, newStore(t), WithNow(stepClock(fixedNow)))
		file := matchingCandidateFile(scripted.Capabilities(), minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
			"001": writableChannel("001", 14_150_000, "MODIFIED").Data,
		})
		plan, err := svc.PrepareSend(testCtx(t), file)
		if err != nil {
			t.Fatalf("PrepareSend: %v", err)
		}
		if _, err := svc.Execute(testCtx(t), plan, plan.ConfirmationDigest()); err != nil {
			t.Fatalf("Execute: unexpected error: %v", err)
		}
		if got := *trace; len(got) == 0 || got[len(got)-1] != "RestoreVFO" {
			t.Errorf("trace = %v, want it to end with RestoreVFO", got)
		}
	})

	t.Run("aborted run", func(t *testing.T) {
		_, scripted, trace := openScriptedVFOSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
		store := newStore(t)
		svc := NewService(scripted, store, WithNow(stepClock(fixedNow)))
		file := matchingCandidateFile(scripted.Capabilities(), minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
			"001": writableChannel("001", 14_150_000, "MODIFIED").Data,
		})
		plan, err := svc.PrepareSend(testCtx(t), file)
		if err != nil {
			t.Fatalf("PrepareSend: %v", err)
		}
		svc.openJournal = func(path string) journalAppender {
			return &failingJournal{inner: store.OpenJournal(path), failOn: "write_attempt"}
		}

		_, err = svc.Execute(testCtx(t), plan, plan.ConfirmationDigest())
		if err == nil {
			t.Fatal("Execute = nil error, want an abort (write_attempt journal append failed)")
		}
		if !errors.Is(err, ErrAborted) {
			t.Errorf("errors.Is(err, ErrAborted) = false; err = %v", err)
		}
		if got := *trace; len(got) == 0 || got[len(got)-1] != "RestoreVFO" {
			t.Errorf("trace = %v, want it to end with RestoreVFO even though Execute aborted", got)
		}
	})
}

// TestExecute_VFOStateRestorer_CancellationDoesNotSuppressRestore: the
// caller's ctx is honoured only BETWEEN slots (execute.go's own doc
// comment); if it is cancelled once the first of two deltas' writes has
// completed, the second delta's iteration aborts on ctx.Err() at the top
// of the loop — but the deferred RestoreVFOState still runs, on its own
// internal bounded context, exactly as restoreMemorySelection's
// established pattern does for obligation 12.
func TestExecute_VFOStateRestorer_CancellationDoesNotSuppressRestore(t *testing.T) {
	_, scripted, trace := openScriptedVFOSession(t, fakeradio.WithFactoryImage(happyPathImage))
	svc := NewService(scripted, newStore(t), WithNow(stepClock(fixedNow)))
	file := matchingCandidateFile(scripted.Capabilities(), happyPathPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 14_100_000, "MOD-ONE").Data,
		"005": writableChannel("005", 14_200_000, "MOD-TWO").Data,
	})
	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}

	ctx, cancel := context.WithCancel(testCtx(t))
	scripted.onWriteChannel = func() {
		scripted.onWriteChannel = nil // cancel once, after the FIRST delta's write completes
		cancel()
	}

	report, err := svc.Execute(ctx, plan, plan.ConfirmationDigest())
	if err == nil {
		t.Fatal("Execute = nil error, want an abort (ctx cancelled before the second delta)")
	}
	if !errors.Is(err, ErrAborted) {
		t.Errorf("errors.Is(err, ErrAborted) = false; err = %v", err)
	}
	if report == nil || report.Written != 1 {
		t.Fatalf("report = %+v, want Written = 1 (only the first delta completed before cancellation)", report)
	}
	if got := *trace; len(got) == 0 || got[len(got)-1] != "RestoreVFO" {
		t.Errorf("trace = %v, want it to end with RestoreVFO despite the cancelled caller ctx", got)
	}
}

// TestExecute_VFOStateRestorer_RestoreFailureDoesNotReplaceOriginalError:
// a RestoreVFOState failure is a journal warning, never a cause that
// replaces the abort Execute already returned — mirroring
// restoreMemorySelection's identical, already-established policy for
// obligation 12. Forces an abort via a journal-append failure (same
// deterministic mechanism as the "aborted run" case above) with
// scripted.restoreErr ALSO set, and asserts the returned error is still
// about the journal failure, not the restore failure.
func TestExecute_VFOStateRestorer_RestoreFailureDoesNotReplaceOriginalError(t *testing.T) {
	_, scripted, _ := openScriptedVFOSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	scripted.restoreErr = errors.New("restore blew up")
	store := newStore(t)
	svc := NewService(scripted, store, WithNow(stepClock(fixedNow)))
	file := matchingCandidateFile(scripted.Capabilities(), minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 14_150_000, "MODIFIED").Data,
	})
	plan, err := svc.PrepareSend(testCtx(t), file)
	if err != nil {
		t.Fatalf("PrepareSend: %v", err)
	}
	svc.openJournal = func(path string) journalAppender {
		return &failingJournal{inner: store.OpenJournal(path), failOn: "write_attempt"}
	}

	_, err = svc.Execute(testCtx(t), plan, plan.ConfirmationDigest())
	if err == nil {
		t.Fatal("Execute = nil error, want an abort (write_attempt journal append failed)")
	}
	if !errors.Is(err, ErrJournalFailed) {
		t.Errorf("errors.Is(err, ErrJournalFailed) = false; err = %v — the restore failure must never replace the original abort cause", err)
	}
	if strErr := err.Error(); strErr == "" || (errors.Is(err, ErrAborted) && !errors.Is(err, ErrJournalFailed)) {
		t.Errorf("err = %v, want it to still name the journal failure", err)
	}
}
