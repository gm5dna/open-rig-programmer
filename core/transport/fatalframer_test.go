// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// This file pins the OPTIONAL FatalFramer hook: the additive seam by which
// a framing names a frame that means "this stream is finished" rather than
// "this command was refused". Every assertion here is about ORDER — which
// of two goroutines got there first — so nothing in it may be satisfied by
// a run that merely won a race.
//
// The hook exists for Kenwood's E; and O; tokens, which arrive as ordinary
// frames and would otherwise be counted as unexpected (or, worse, matched
// as an answer on the very chunk that carried the link failure). No
// framing registered before it implements it: catFraming does not, and a
// framing that does not implement it pays one nil-field check per read
// chunk and takes no lock at all — TestFatalFramer_AbsentIsInert pins that.

// --- fixtures -----------------------------------------------------------

// fatalTokenFrame is the test dialect's stream-fatal token: lineFraming
// frames on '\n', so "E\n" stands in for Kenwood's "E;".
const fatalTokenFrame = "E\n"

// stubFatalError stands in for the typed core/kw error IsFatal will really
// return — a value the driver recovers with errors.As, naming the token.
// Clause 2 of the contract is that THIS value, not a sentinel, is what
// closePort records, so the assertions below compare pointers.
type stubFatalError struct{ Token string }

func (e *stubFatalError) Error() string {
	return fmt.Sprintf("stub framing: fatal stream token %q", e.Token)
}

// fatalLineFraming is lineFraming plus FatalFramer, with an optional
// NoteSent hook the post-purge handshake drives. It is deliberately
// test-local: core/kw's own re-pin of this contract lands with core/kw.
type fatalLineFraming struct {
	lineFraming
	cause  *stubFatalError
	onNote func(frame []byte)
}

func newFatalLineFraming() *fatalLineFraming {
	return &fatalLineFraming{
		lineFraming: lineFraming{policy: fastPolicy},
		cause:       &stubFatalError{Token: fatalTokenFrame},
	}
}

// IsFatal is the whole of the optional interface: a non-nil return is the
// typed cause the engine closes with.
func (f *fatalLineFraming) IsFatal(frame []byte) error {
	if string(frame) == fatalTokenFrame {
		return f.cause
	}
	return nil
}

func (f *fatalLineFraming) NoteSent(frame []byte) {
	f.lineFraming.NoteSent(frame)
	if f.onNote != nil {
		f.onNote(frame)
	}
}

// --- clause 1 and clause 2: the same-chunk pin --------------------------

// TestFatalFramer_SameChunk_FatalSuppressesTheAnswerItArrivedWith is the
// clause-1 pin, and it is the one the whole hook exists for.
//
// A chunk of "«matching answer»\nE\n" reaches acc.Push as ONE Push call.
// Without the hook the engine delivers the frames first and the error
// afterwards (engine.go's readLoop), so waitForAnswer matches the answer,
// reports SUCCESS, and the link failure lands on the NEXT command — a
// per-channel write verification reporting a match on the very chunk that
// carried the failure. That is spec §"Where E; and O; live" window (i).
//
// FOUR assertions, because two of them are satisfied by the defect they
// were written to catch: "zero later writes" and "errors.As recovers the
// cause" both hold when the failure is merely DEFERRED to the next call.
// Assertion 1 is what tells suppression from deferral.
func TestFatalFramer_SameChunk_FatalSuppressesTheAnswerItArrivedWith(t *testing.T) {
	f := newFatalLineFraming()
	port := newScriptedPort("RD ok\n" + fatalTokenFrame)
	t.Cleanup(func() { _ = port.Close() })

	e, err := NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	spec := CommandSpec{Class: ClassRead, Match: lineMatch("RD "), Timeout: 5 * time.Second, Settle: time.Millisecond}

	// Assertion 1 — the CURRENT Do fails with the typed fatal cause, and
	// the matching answer never yields a success. This is the clause-1
	// assertion: a frame-by-frame consultation inside readLoop's existing
	// delivery loop reproduces window (i) exactly and fails here.
	got, err := e.Do(ctx, lineCommand("RD?\n"), spec)
	if err == nil {
		t.Fatalf("Do returned the matching answer %q with no error — the fatal token in the SAME chunk must suppress it, not merely defer the failure to the next call", got)
	}
	var fatal *stubFatalError
	if !errors.As(err, &fatal) {
		t.Fatalf("Do error = %v, want errors.As to recover *stubFatalError — clause 2: closePort records the TYPED cause IsFatal returned, not a generic reader error", err)
	}
	if fatal != f.cause {
		t.Errorf("Do recovered %p (%v), want the very cause value IsFatal returned (%p)", fatal, fatal, f.cause)
	}
	if got != nil {
		t.Errorf("Do returned frame %q alongside its error, want nil", got)
	}

	// Assertion 3 — fatal-first: the fatal publication won the first
	// close, so ErrPortClosed is reachable and the cause it wraps is the
	// typed one (assertion 1 already checked the cause itself).
	if !errors.Is(err, ErrPortClosed) {
		t.Errorf("Do error = %v, want errors.Is match against ErrPortClosed — the publication closes the port from the reader goroutine", err)
	}

	// Assertion 2 — the FOLLOWING Do performs ZERO writes and returns the
	// SAME recorded cause, not a fresh or generic one.
	writesBefore := len(port.written())
	_, err = e.Do(ctx, lineCommand("RD?\n"), spec)
	if err == nil {
		t.Fatal("the following Do succeeded, want a refusal on the closed port")
	}
	var again *stubFatalError
	if !errors.As(err, &again) {
		t.Fatalf("following Do error = %v, want errors.As to recover *stubFatalError", err)
	}
	if again != f.cause {
		t.Errorf("following Do recovered %p (%v), want the cause closePort recorded (%p)", again, again, f.cause)
	}
	if w := len(port.written()); w != writesBefore {
		t.Errorf("the following Do wrote %d further frame(s), want 0 — nothing leaves the host after a fatal frame the engine has received", w-writesBefore)
	}
}

// TestFatalFramer_OtherCloseFirst_TypedCauseDoesNotSurvive is assertion 4,
// and it is the doc comment's second liveness fact made falsifiable: the
// typed cause is universal ONLY when the fatal publication wins the first
// close. closePort's closeOnce (engine.go's closePort) keeps the FIRST
// cause any caller supplies, and four call sites reach it without ever
// queueing for fatalGate — Engine.Close, a terminal read error, a consumed
// reader error and the gated write's own error branch.
//
// Three of those four are pinned here as the winner; the fourth (a
// consumed reader error inside drainToQuietLocked) reaches the identical
// closeOnce by the identical route.
func TestFatalFramer_OtherCloseFirst_TypedCauseDoesNotSurvive(t *testing.T) {
	errRead := errors.New("stub: terminal read error")
	errWrite := errors.New("stub: write failed")

	tests := []struct {
		name string
		// win drives whichever close should win, and returns the cause
		// a later caller must see (nil = the bare ErrPortClosed
		// sentinel, which is what an explicit Engine.Close records).
		win func(t *testing.T, e *Engine, port *faultPort) error
	}{
		{
			name: "Engine.Close wins",
			win: func(t *testing.T, e *Engine, port *faultPort) error {
				if err := e.Close(); err != nil {
					t.Fatalf("Close: unexpected error: %v", err)
				}
				return nil
			},
		},
		{
			name: "a terminal read error wins",
			win: func(t *testing.T, e *Engine, port *faultPort) error {
				port.failRead(errRead)
				<-e.readerDone
				return errRead
			},
		},
		{
			name: "the gated write's own error branch wins",
			win: func(t *testing.T, e *Engine, port *faultPort) error {
				port.failWrite(errWrite)
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, err := e.Do(ctx, lineCommand("RD?\n"), CommandSpec{Class: ClassRead, Match: lineMatch("RD "), Timeout: 2 * time.Second, Settle: time.Millisecond})
				if !errors.Is(err, errWrite) {
					t.Fatalf("Do error = %v, want errors.Is match against the write error", err)
				}
				// The write-error branch returns from INSIDE the
				// gate, and it is the path a single trailing Unlock
				// in gatedWrite would skip — leaving fatalGate held
				// forever and hanging the publication below. TryLock
				// turns that into an assertion rather than a wedge.
				if !e.fatalGate.TryLock() {
					t.Fatal("fatalGate is still held after gatedWrite's write-error branch returned — every path taken after the Lock must release it at the helper's return")
				}
				e.fatalGate.Unlock()
				return errWrite
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFatalLineFraming()
			port := newFaultPort()
			t.Cleanup(func() { _ = port.Close() })

			e, err := NewEngineWith(port, f)
			if err != nil {
				t.Fatalf("NewEngineWith: unexpected error: %v", err)
			}
			t.Cleanup(func() { _ = e.Close() })

			wantCause := tc.win(t, e, port)

			// The fatal publication runs SECOND, through the very
			// method readLoop uses. closeOnce has already been spent,
			// so the typed cause is dropped on the floor — correctly,
			// since the port is already closed — and this test says so
			// out loud rather than leaving it a caveat in a comment.
			e.publishFatal(f.cause)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err = e.Do(ctx, lineCommand("RD?\n"), CommandSpec{Class: ClassRead, Match: lineMatch("RD "), Timeout: time.Second, Settle: time.Millisecond})
			if err == nil {
				t.Fatal("Do succeeded on a closed engine")
			}
			var fatal *stubFatalError
			if errors.As(err, &fatal) {
				t.Errorf("Do error = %v recovered the typed fatal cause, want it NOT to survive — %s", err, tc.name)
			}
			if !errors.Is(err, ErrPortClosed) {
				t.Errorf("Do error = %v, want errors.Is match against ErrPortClosed", err)
			}
			if wantCause == nil {
				if !errors.Is(err, ErrPortClosed) || err != ErrPortClosed {
					t.Errorf("Do error = %v, want the bare ErrPortClosed sentinel — an explicit Engine.Close records no cause", err)
				}
			} else if !errors.Is(err, wantCause) {
				t.Errorf("Do error = %v, want errors.Is match against the cause that DID win (%v)", err, wantCause)
			}
		})
	}
}

// --- clause 3: the lock-order theorem's second half ---------------------

// TestFatalFramer_GateIsNeverHeldAcrossAChannelReceive is the pin the plan
// calls REQUIRED, and its red proof is the struck implementation itself.
//
// The gate must be taken and released inside gatedWrite. A
// `defer e.fatalGate.Unlock()` written in Do's BODY releases at Do's
// return instead — and Do does not return until after its read wait,
// nextEvent's blocking receive on e.events, whose ONLY producer is
// readLoop, which must take fatalGate to publish. That is a hang in the
// package every registered radio goes through.
//
// Here the fatal token arrives WHILE Do is in that read wait. Under the
// admitted form Do closes in milliseconds; under the struck form neither
// goroutine can proceed, so this test fails as a bounded TIMEOUT rather
// than wedging the suite. spec.Timeout is deliberately far longer than the
// bound, so a hang cannot be rescued by the read timeout.
func TestFatalFramer_GateIsNeverHeldAcrossAChannelReceive(t *testing.T) {
	f := newFatalLineFraming()
	port := newScriptedPort(fatalTokenFrame)
	t.Cleanup(func() { _ = port.Close() })

	e, err := NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	type result struct {
		err error
	}
	done := make(chan result, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		_, err := e.Do(ctx, lineCommand("RD?\n"), CommandSpec{
			Class:   ClassRead,
			Match:   lineMatch("RD "),
			Timeout: 30 * time.Second,
			Settle:  time.Millisecond,
		})
		done <- result{err: err}
	}()

	select {
	case res := <-done:
		var fatal *stubFatalError
		if !errors.As(res.err, &fatal) {
			t.Fatalf("Do error = %v, want errors.As to recover *stubFatalError", res.err)
		}
	case <-time.After(5 * time.Second):
		// Unwedge before failing: Engine.Close reaches closePort
		// without queueing for fatalGate, so it releases the blocked
		// Do even under the struck form.
		_ = e.Close()
		t.Fatal("Do did not return within 5s while a fatal frame arrived during its read wait — fatalGate is being held across a channel receive (the struck defer-in-Do form)")
	}
}

// --- clause 4: the post-purge race, with a two-way handshake ------------

// faultPort is a Port whose Read blocks until released and whose Read and
// Write can each be made to fail on demand. It is shared by the
// other-close-first table and the post-purge pin, which need the same two
// things: a reader that delivers nothing until told, and a Close that is
// observable as the publication's acknowledgement.
type faultPort struct {
	mu        sync.Mutex
	writes    [][]byte
	pending   []byte
	readErr   error
	writeErr  error
	wake      chan struct{}
	closed    chan struct{}
	closeOnce sync.Once
}

func newFaultPort() *faultPort {
	return &faultPort{
		wake:   make(chan struct{}, 8),
		closed: make(chan struct{}),
	}
}

func (p *faultPort) Read(b []byte) (int, error) {
	for {
		p.mu.Lock()
		if p.readErr != nil {
			err := p.readErr
			p.mu.Unlock()
			return 0, err
		}
		if len(p.pending) > 0 {
			n := copy(b, p.pending)
			p.pending = p.pending[n:]
			p.mu.Unlock()
			return n, nil
		}
		p.mu.Unlock()
		select {
		case <-p.wake:
		case <-p.closed:
			return 0, errClosedStub
		}
	}
}

func (p *faultPort) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.writeErr != nil {
		return 0, p.writeErr
	}
	p.writes = append(p.writes, append([]byte(nil), b...))
	return len(b), nil
}

// Close is the publication's acknowledgement: closePort calls it from
// INSIDE fatalGate, so a test that waits on p.closed has waited for the
// fatal publication to have completed, not merely started.
func (p *faultPort) Close() error {
	p.closeOnce.Do(func() { close(p.closed) })
	return nil
}

func (p *faultPort) written() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([][]byte(nil), p.writes...)
}

// deliver queues bytes for the next Read and wakes it.
func (p *faultPort) deliver(s string) {
	p.mu.Lock()
	p.pending = append(p.pending, s...)
	p.mu.Unlock()
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

func (p *faultPort) failRead(err error) {
	p.mu.Lock()
	p.readErr = err
	p.mu.Unlock()
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

func (p *faultPort) failWrite(err error) {
	p.mu.Lock()
	p.writeErr = err
	p.mu.Unlock()
}

// TestFatalFramer_PostPurgeRace_NoFrameLeavesAfterAReceivedFatalFrame is
// the adversarial pin for spec §"Where E; and O; live" window (iii): the
// token arrives strictly AFTER Do's entry purge has returned and BEFORE
// its write, an interval in which nothing in Do re-reads e.closed and
// nothing orders the two goroutines at all.
//
// The handshake is TWO-WAY, and that is the whole point (Q13 clause 4). A
// one-way release — "NoteSent lets the bytes go and returns" — would let
// Do reach the gate and write before readLoop had even parsed the chunk,
// and the test would go green on a race it happened to win. So NoteSent
// releases the bytes and then BLOCKS until the publication has COMPLETED,
// acknowledged by the port's own Close, which closePort calls from inside
// fatalGate. The write attempt is therefore ordered strictly after the
// publication by construction, not by a sleep.
//
// Under Option B (no hook) this same fixture asserts the opposite — the
// frame GOES OUT and the failure lands on the following command — and the
// pair is Q13's own red/green proof.
func TestFatalFramer_PostPurgeRace_NoFrameLeavesAfterAReceivedFatalFrame(t *testing.T) {
	port := newFaultPort()
	t.Cleanup(func() { _ = port.Close() })

	f := newFatalLineFraming()
	var (
		noteOnce      sync.Once
		handshakeDone atomic.Bool
	)
	f.onNote = func([]byte) {
		noteOnce.Do(func() {
			port.deliver(fatalTokenFrame)
			select {
			case <-port.closed:
				handshakeDone.Store(true)
			case <-time.After(10 * time.Second):
				// Recorded rather than fatal: the assertions below
				// then fail loudly, and the test does not wedge.
				handshakeDone.Store(false)
			}
		})
	}

	e, err := NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = e.Do(ctx, lineCommand("RD?\n"), CommandSpec{Class: ClassRead, Match: lineMatch("RD "), Timeout: 5 * time.Second, Settle: time.Millisecond})

	if !handshakeDone.Load() {
		t.Fatal("the two-way handshake never completed: the port's Close was not observed within 10s of NoteSent releasing the fatal bytes — the publication did not run, so the assertions below would prove nothing")
	}
	if w := port.written(); len(w) != 0 {
		t.Errorf("the port saw %d write(s) after a RECEIVED fatal frame, want 0: %q", len(w), w)
	}
	var fatal *stubFatalError
	if !errors.As(err, &fatal) {
		t.Fatalf("Do error = %v, want errors.As to recover *stubFatalError", err)
	}
	if fatal != f.cause {
		t.Errorf("Do recovered %p (%v), want the very cause value IsFatal returned (%p)", fatal, fatal, f.cause)
	}
}

// --- additivity ---------------------------------------------------------

// TestFatalFramer_AbsentIsInert is the byte-identity claim's own pin: a
// framing that does not implement FatalFramer resolves to a nil field at
// construction, so neither hot site asserts a type, takes a lock or enters
// a branch. Every model registered before this hook uses such a framing.
func TestFatalFramer_AbsentIsInert(t *testing.T) {
	plain := &lineFraming{policy: fastPolicy}
	port := newScriptedPort("RD ok\n")
	t.Cleanup(func() { _ = port.Close() })

	e, err := NewEngineWith(port, plain)
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	if e.fatal != nil {
		t.Errorf("Engine.fatal = %v for a framing that does not implement FatalFramer, want nil", e.fatal)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	got, err := e.Do(ctx, lineCommand("RD?\n"), CommandSpec{Class: ClassRead, Match: lineMatch("RD "), Timeout: 5 * time.Second, Settle: time.Millisecond})
	if err != nil {
		t.Fatalf("Do: unexpected error: %v", err)
	}
	if string(got) != "RD ok\n" {
		t.Errorf("Do = %q, want %q", got, "RD ok\n")
	}
}

// TestFatalFramer_ResolvedOnceAtConstruction pins the other half: a
// framing that DOES implement it is resolved exactly once, where the
// framing is stored, so no hot path ever performs a type assertion.
func TestFatalFramer_ResolvedOnceAtConstruction(t *testing.T) {
	f := newFatalLineFraming()
	port := newFaultPort()
	t.Cleanup(func() { _ = port.Close() })

	e, err := NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	if e.fatal == nil {
		t.Fatal("Engine.fatal is nil for a framing that implements FatalFramer, want it resolved at construction")
	}
	if e.fatal.IsFatal([]byte(fatalTokenFrame)) != f.cause {
		t.Error("Engine.fatal does not resolve to the framing that was passed")
	}
	if e.fatal.IsFatal([]byte("RD ok\n")) != nil {
		t.Error("IsFatal claimed an ordinary frame")
	}
}
