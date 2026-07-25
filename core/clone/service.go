// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// journalAppender is the seam Execute and PrepareSend append journal
// lines through. *Journal (journal.go) is the production implementation
// and already satisfies it with no changes. Defined as an interface so a
// test can substitute a fake that fails on a chosen event, injecting a
// journal durability failure without needing a real unwritable
// filesystem path — see Service.openJournal and the ratified journal
// fail-safe policy in doc.go.
type journalAppender interface {
	Append(now time.Time, event string, fields map[string]any) error
	Path() string
}

// generatorID identifies this software in every codeplug.Codeplug this
// package writes (Codeplug.Generator), including PrepareSend's snapshot.
const generatorID = "open-rig-programmer/core/clone"

// Progress is the callback a Service reports its work through, one call per
// slot. phase is one of "read" (ReadAll), "verify-read" (Execute's
// pre-write drift check, obligation 11), "write" (Execute's per-channel
// WriteChannel), or "verify" (Execute's per-channel read-back check,
// obligation 7). done/total are 1-based/absolute counts within that phase;
// slot is the canonical wire-form slot just processed.
type Progress func(phase string, done, total int, slot string)

// Logger is the injectable sink a Service uses to report diagnostics.
// Mirrors transport.Logger's shape so a caller can plug the same
// implementation into both layers. NewService defaults to a Logger that
// drops everything.
type Logger interface {
	Printf(format string, args ...any)
}

// nopLogger is the default Logger: it drops everything.
type nopLogger struct{}

func (nopLogger) Printf(string, ...any) {}

// nopProgress is the default Progress: it drops everything.
func nopProgress(string, int, int, string) {}

// serviceGeneration is a package-level counter, incremented once per
// NewService call, so every Service instance gets a distinct generation —
// see Service.generation's doc comment for why this matters.
var serviceGeneration int64

// Service owns one radio session's end-to-end safety choreography: reading
// a fresh baseline, preparing an immutable send plan against it, and
// executing that plan with per-channel write/verify and an append-only
// journal. See doc.go for the ten (plus one) binding obligations this
// package enforces.
//
// A Service is bound to exactly one driver.Session for its entire
// lifetime: NewService takes sess once, and nothing in this package ever
// swaps it out. This is load-bearing for the session-identity binding
// obligation (see SendPlan and Execute's generation check) — a NEW Service
// is required to talk to a NEW session (e.g. after a reconnect), and a
// SendPlan built by one Service is refused by any other.
type Service struct {
	sess  driver.Session
	store SnapshotStore

	// openJournal opens the journal for a given snapshot path — defaults
	// (set in NewService) to a thin wrapper around s.store.OpenJournal,
	// which keeps SnapshotStore's own public API (used outside this
	// package too) as a concrete *Journal return, unchanged. A test in
	// this package may overwrite this field directly to substitute a
	// journalAppender that fails on a chosen event (see journalAppender).
	openJournal func(snapshotPath string) journalAppender

	logger   Logger
	now      func() time.Time
	progress Progress

	// generation uniquely identifies this Service instance (see
	// serviceGeneration), independent of sess.Identity()'s content: a
	// reconnect to a DIFFERENT physical radio that happens to answer with
	// the same CAT ID/port/USB serial as before would otherwise be
	// indistinguishable from the original session by identity content
	// alone (obligation 4's stated rationale). Every SendPlan this
	// Service prepares records this value, and Execute refuses any plan
	// whose recorded generation does not match it.
	generation int64

	mu                sync.Mutex
	firmwareConfirmed bool
	firmwareVersion   string

	// opBusy/opInProgress implement Fix 2's try-lock pattern (see ErrBusy):
	// guarded by mu (a short critical section to decide the try-lock
	// outcome, NOT held across a whole ReadAll/PrepareSend/Execute call).
	// opInProgress names whichever operation currently holds it, so a
	// refused caller's *BusyError can say what it collided with.
	opBusy       bool
	opInProgress string
}

// Option configures a *Service at construction time. See NewService.
type Option func(*Service)

// WithLogger sets the Logger a Service uses to report diagnostics. A nil
// Logger is ignored (the nopLogger default is kept).
func WithLogger(l Logger) Option {
	return func(s *Service) {
		if l != nil {
			s.logger = l
		}
	}
}

// WithNow overrides the clock a Service uses for every timestamp it
// records (RadioInfo.ReadAt, snapshot filenames, journal lines) — this is
// how tests get determinism without waiting on the real wall clock. A nil
// now is ignored (the time.Now default is kept).
func WithNow(now func() time.Time) Option {
	return func(s *Service) {
		if now != nil {
			s.now = now
		}
	}
}

// WithProgress sets the Progress callback a Service reports its work
// through. A nil progress is ignored (the no-op default is kept).
func WithProgress(p Progress) Option {
	return func(s *Service) {
		if p != nil {
			s.progress = p
		}
	}
}

// journalAppend appends one journal line, logging (never returning) a
// failure via s.logger. Reserved for the journal lines the ratified
// fail-safe policy (doc.go) does NOT gate anything further on:
// "firmware_confirmed" (best-effort — the gate state itself is already
// held in memory, see Execute), "abort" (already terminating the run;
// nothing left to protect), and "completion" (the run already fully
// succeeded — a bookkeeping fsync hiccup on the very last line must not
// retroactively turn a clean run into a reported failure). Every OTHER
// journal line in the delta-write loop (write_attempt, write_result,
// verify_result) and PrepareSend's "prepare" line use a CHECKED append
// instead — see appendDeltaJournal and PrepareSend.
func (s *Service) journalAppend(j journalAppender, event string, fields map[string]any) {
	if err := j.Append(s.now(), event, fields); err != nil {
		s.logger.Printf("clone: journal %s: failed to append %q event: %v", j.Path(), event, err)
	}
}

// appendDeltaJournal appends one journal line during Execute's
// delta-write loop and, on a durability failure, returns the
// *JournalFailedError the ratified fail-safe policy (doc.go) requires:
// nil on success, a non-nil error the caller must abort on otherwise.
// Logs the underlying cause via s.logger too, exactly as journalAppend
// does, so the diagnostic is never lost even though this path also
// surfaces the failure to the caller.
func (s *Service) appendDeltaJournal(j journalAppender, event, slot string, fields map[string]any) *JournalFailedError {
	if err := j.Append(s.now(), event, fields); err != nil {
		s.logger.Printf("clone: journal %s: failed to append %q event: %v", j.Path(), event, err)
		return &JournalFailedError{Event: event, Slot: slot, Cause: err}
	}
	return nil
}

// acquireOp implements Fix 2's try-lock pattern (see ErrBusy): it refuses,
// with a typed *BusyError, if another ReadAll/PrepareSend/Execute call is
// already in progress on this Service — rather than letting two
// operations interleave their wire traffic against the same session, or
// queueing a caller behind one that might be arbitrarily slow (a
// multi-hundred-slot ReadAll, or a user reviewing a diff between
// PrepareSend and Execute). name is the operation requesting the lock,
// recorded so a REFUSED caller's error names what it collided with. Every
// successful acquireOp call MUST defer releaseOp immediately, including
// on every error/panic return path, so a Service is never left
// permanently busy.
//
// PrepareSend needs its OWN fresh ReadAll internally; it does so via the
// unexported readAll (read.go), which does NOT itself call acquireOp —
// only PrepareSend's own, single acquireOp call (covering its whole body,
// including that nested read) is taken, so PrepareSend never
// self-refuses against its own nested read.
func (s *Service) acquireOp(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.opBusy {
		return &BusyError{InProgress: s.opInProgress}
	}
	s.opBusy = true
	s.opInProgress = name
	return nil
}

// releaseOp releases the operation lock acquireOp took. See acquireOp's
// doc comment: every successful acquireOp call must defer this
// immediately.
func (s *Service) releaseOp() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.opBusy = false
	s.opInProgress = ""
}

// NewService constructs a Service bound to sess, persisting snapshots and
// journals under store. See Service's doc comment for why sess is bound
// for the Service's whole lifetime, and Option for the available
// WithLogger/WithNow/WithProgress overrides.
func NewService(sess driver.Session, store SnapshotStore, opts ...Option) *Service {
	s := &Service{
		sess:       sess,
		store:      store,
		logger:     nopLogger{},
		now:        time.Now,
		progress:   nopProgress,
		generation: atomic.AddInt64(&serviceGeneration, 1),
	}
	s.openJournal = func(snapshotPath string) journalAppender { return s.store.OpenJournal(snapshotPath) }
	for _, opt := range opts {
		opt(s)
	}
	return s
}
