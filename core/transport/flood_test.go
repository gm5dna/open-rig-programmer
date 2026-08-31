// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// This file is D2's starvation-deadline proof (round 2, F3≈C4). Address
// filtering prevents FALSE MATCHES; it does nothing about STARVATION. A
// transceive flood — factory-ON on SOME of the Icom models this programme
// registers, with no off-switch shipped — is a stream that never goes
// quiet, and every loop in this package whose exit condition is "silence"
// would wait for it forever. (internal/wiring/wiring_test.go's icomModels
// is the registered set and TestYaesuAndIcomModelsPartitionSupportedModels pins
// it; the count is deliberately not repeated here, because one radio that
// never goes quiet is the whole hazard and a number only goes stale.)
//
// The tests below all use a port that NEVER goes quiet, not merely one
// that interleaves noise with answers. Each asserts the same shape of
// property: the call RETURNS, with an honest error, inside a bound that
// does not depend on the arrival rate. A test that hangs here is the bug
// it is looking for, so each is given a watchdog rather than being left to
// the package timeout.

// floodPort delivers a well-formed, never-matching frame every
// floodInterval, forever, and answers nothing — the transceive-broadcast
// shape. Writes are recorded and always succeed.
type floodPort struct {
	interval time.Duration
	frame    string

	mu      sync.Mutex
	writes  [][]byte
	closed  bool
	closeCh chan struct{}
}

// floodInterval is far below any IdleGap used here, so the port's silence
// gaps can never satisfy a drain.
const floodInterval = time.Millisecond

func newFloodPort() *floodPort {
	return &floodPort{
		interval: floodInterval,
		frame:    "ZZ0000;", // well-formed CAT framing, matches no spec below
		closeCh:  make(chan struct{}),
	}
}

func (p *floodPort) Read(b []byte) (int, error) {
	select {
	case <-time.After(p.interval):
	case <-p.closeCh:
		return 0, errClosedStub
	}
	return copy(b, p.frame), nil
}

func (p *floodPort) Write(b []byte) (int, error) {
	p.mu.Lock()
	p.writes = append(p.writes, append([]byte(nil), b...))
	p.mu.Unlock()
	return len(b), nil
}

func (p *floodPort) Close() error {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.closeCh)
	}
	p.mu.Unlock()
	return nil
}

func (p *floodPort) written() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([][]byte(nil), p.writes...)
}

// newFloodEngine builds an Engine over a never-quiet port, using the
// non-CAT lineFraming's fast drain policy so the bounds under test are
// tens of milliseconds rather than hundreds. The framing's own accumulator
// is CAT-shaped here only because the flood frames are; nothing in these
// tests depends on which protocol it is.
func newFloodEngine(t *testing.T, opts ...Option) (*floodPort, *Engine) {
	t.Helper()
	port := newFloodPort()
	t.Cleanup(func() { _ = port.Close() })

	f := &floodFraming{policy: DrainPolicy{IdleGap: 20 * time.Millisecond, Cap: 40 * time.Millisecond}}
	e, err := NewEngineWith(port, f, opts...)
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return port, e
}

// floodFraming frames on ';' (so the flood's "ZZ0000;" frames are whole
// frames rather than noise that would trip CONTAMINATED and end the flood
// by a different route — the point is a stream of PERFECTLY VALID frames
// that simply never stops), admits everything, and has no init sequence.
type floodFraming struct {
	policy DrainPolicy
}

func (f *floodFraming) NewAccumulator(max int) Accumulator { return &semicolonAccumulator{max: 256} }
func (f *floodFraming) IsRejection(frame []byte) bool      { return string(frame) == "?;" }
func (f *floodFraming) Allow([]byte) bool                  { return true }
func (f *floodFraming) InitSequence() []Command            { return nil }
func (f *floodFraming) DrainPolicy() DrainPolicy           { return f.policy }
func (f *floodFraming) NoteSent([]byte)                    {}

type semicolonAccumulator struct {
	buf []byte
	max int
}

func (a *semicolonAccumulator) Push(chunk []byte) ([][]byte, error) {
	a.buf = append(a.buf, chunk...)
	var frames [][]byte
	for {
		i := strings.IndexByte(string(a.buf), ';')
		if i < 0 {
			break
		}
		frames = append(frames, append([]byte(nil), a.buf[:i+1]...))
		a.buf = a.buf[i+1:]
	}
	if len(a.buf) > a.max {
		discarded := len(a.buf)
		a.buf = nil
		return frames, &FrameTooLongError{DiscardedLen: discarded}
	}
	return frames, nil
}

// burstFloodPort is floodPort's HARDER sibling: it returns many frames per
// Read with only a token pause, so the engine's event channel is
// essentially never empty. That difference is the whole point.
//
// floodPort's one-frame-per-millisecond stream starves the DRAINS (whose
// idle timer is re-armed by every arrival) but not the answer waits: at
// that rate nextEvent's blocking select is still reached often enough for
// its timeout channel to fire. burstFloodPort starves the ANSWER WAITS
// too, because nextEvent's buffered-events priority check — deliberate,
// and load-bearing for a different hazard (see its doc comment) — wins
// EVERY iteration when there is always an event waiting, so the timeout
// channel and ctx.Done() are never selected on at all. Comparing the clock
// before touching the channel is the only thing that bounds this shape.
type burstFloodPort struct {
	floodPort
	perRead int
}

func newBurstFloodPort() *burstFloodPort {
	p := &burstFloodPort{perRead: 32}
	p.interval = 50 * time.Microsecond
	p.frame = "ZZ0000;"
	p.closeCh = make(chan struct{})
	return p
}

func (p *burstFloodPort) Read(b []byte) (int, error) {
	select {
	case <-time.After(p.interval):
	case <-p.closeCh:
		return 0, errClosedStub
	}
	return copy(b, strings.Repeat(p.frame, p.perRead)), nil
}

// TestBurstFlood_AnswerWaitEndsOnScheduleWhenEventsAlwaysWait is the
// answer wait's own starvation proof, in the arrival pattern where
// nextEvent's buffered-events priority check would otherwise never yield.
//
// TWO INGREDIENTS ARE BOTH REQUIRED, and the second is the one that is
// easy to leave out. A fast producer alone does not starve the wait: the
// consumer is faster still, so the channel keeps going momentarily empty
// and the blocking select — with its timeout case — is reached anyway.
// What actually holds the channel non-empty is a SLOW CONSUMER, and the
// engine has a real one: safety obligation 3 logs every unexpected frame
// through the injected Logger, and a Logger that writes to a file or a
// console is orders of magnitude slower than a channel receive. slowLogger
// stands in for that. With both, e.events is never empty when nextEvent
// looks, the priority check wins every iteration, and the deadline
// comparison at the top of nextEvent is the only thing left that can end
// the wait.
func TestBurstFlood_AnswerWaitEndsOnScheduleWhenEventsAlwaysWait(t *testing.T) {
	port := newBurstFloodPort()
	t.Cleanup(func() { _ = port.Close() })

	f := &floodFraming{policy: DrainPolicy{IdleGap: 20 * time.Millisecond, Cap: 40 * time.Millisecond}}
	e, err := NewEngineWith(port, f, WithLogger(slowLogger{delay: 2 * time.Millisecond}))
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var readErr error
	withinBound(t, "Do (read under burst flood)", 5*time.Second, func() {
		_, readErr = e.Do(ctx, lineCommand("RD?;"), CommandSpec{
			Class:   ClassRead,
			Match:   lineMatch("RD "),
			Timeout: 50 * time.Millisecond,
			Settle:  time.Millisecond,
		})
	})
	if !errors.Is(readErr, ErrTimeout) {
		t.Fatalf("Do (read under burst flood) = %v, want errors.Is match against ErrTimeout", readErr)
	}

	var writeErr error
	withinBound(t, "Do (write under burst flood)", 5*time.Second, func() {
		_, writeErr = e.Do(ctx, lineCommand("WR x;"), CommandSpec{
			Class:       ClassWrite,
			ErrorWindow: 30 * time.Millisecond,
			Settle:      time.Millisecond,
		})
	})
	// The second exchange enters suspect (the first read's terminal
	// timeout), so its entry quarantine cannot succeed under the flood
	// and it refuses to transmit — which is the correct behaviour and
	// still, crucially, BOUNDED. Either outcome is acceptable here; a
	// hang is not.
	if writeErr != nil && !errors.Is(writeErr, ErrQuarantineFailed) {
		t.Fatalf("Do (write under burst flood) = %v, want nil or ErrQuarantineFailed", writeErr)
	}
}

// slowLogger is a Logger slow enough to make the engine's frame consumer
// slower than a saturating producer — see
// TestBurstFlood_AnswerWaitEndsOnScheduleWhenEventsAlwaysWait for why that
// is the load-bearing half of that test's setup. A real Logger writing to
// a file or a console is slow in exactly this way.
type slowLogger struct{ delay time.Duration }

func (l slowLogger) Printf(string, ...any) { time.Sleep(l.delay) }

// withinBound runs fn and fails the test if it has not returned inside
// bound. It exists because the failure mode under test is a HANG: without
// a watchdog the symptom would be the whole package timing out ten minutes
// later, attributed to nothing in particular.
func withinBound(t *testing.T, what string, bound time.Duration, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(bound):
		t.Fatalf("%s did not return within %v against a stream that never goes quiet — that is the starvation D2's absolute deadlines exist to bound", what, bound)
	}
}

// TestFlood_DrainToQuietFailsInsteadOfWaitingForever is the base case:
// under a continuous flood there is no quiet to reach, and the drain says
// so — bounded by DrainPolicy.Cap — instead of postponing itself forever
// by re-arming its idle timer on every arrival.
func TestFlood_DrainToQuietFailsInsteadOfWaitingForever(t *testing.T) {
	_, e := newFloodEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	withinBound(t, "DrainToQuiet", 5*time.Second, func() { err = e.DrainToQuiet(ctx) })
	if !errors.Is(err, ErrDrainCapExceeded) {
		t.Fatalf("DrainToQuiet under flood = %v, want errors.Is match against ErrDrainCapExceeded", err)
	}
}

// TestFlood_ReadTimesOutOnScheduleAndDoesNotRetryForever pins the answer
// wait's absolute deadline. Every arriving frame is non-matching, so
// before D2 the wait's timeout channel was never selected on at all — the
// buffered-events priority check in nextEvent won every iteration.
func TestFlood_ReadTimesOutOnScheduleAndDoesNotRetryForever(t *testing.T) {
	port, e := newFloodEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	withinBound(t, "Do (read under flood)", 5*time.Second, func() {
		_, err = e.Do(ctx, lineCommand("RD?;"), CommandSpec{
			Class:   ClassRead,
			Match:   lineMatch("RD "),
			Timeout: 50 * time.Millisecond,
			Settle:  time.Millisecond,
		})
	})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("Do (read under flood) = %v, want errors.Is match against ErrTimeout", err)
	}
	if w := port.written(); len(w) != 1 {
		t.Errorf("port saw %d writes, want 1 — RetryReads was 0, and the flood must not have provoked a retransmission", len(w))
	}
	if e.UnexpectedFrames() == 0 {
		t.Error("UnexpectedFrames = 0 — the flood was not actually delivered, so this test proved nothing (safety obligation 3 also requires those frames be counted)")
	}
}

// TestFlood_ReadWithRetriesStillEndsBounded pins the retry path: each
// retry's own quarantine drain fails under the flood, and Do surfaces that
// rather than looping. The write count is the safety half — a bounded
// number of transmissions, not one per flood frame.
func TestFlood_ReadWithRetriesStillEndsBounded(t *testing.T) {
	port, e := newFloodEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	withinBound(t, "Do (read with retries under flood)", 5*time.Second, func() {
		_, err = e.Do(ctx, lineCommand("RD?;"), CommandSpec{
			Class:      ClassRead,
			Match:      lineMatch("RD "),
			Timeout:    50 * time.Millisecond,
			RetryReads: 2,
			Settle:     time.Millisecond,
		})
	})
	if err == nil {
		t.Fatal("Do (read with retries under flood) = nil error, want a refusal — nothing ever answered")
	}
	if !errors.Is(err, ErrTimeout) && !errors.Is(err, ErrDrainCapExceeded) {
		t.Fatalf("Do (read with retries under flood) = %v, want ErrTimeout or ErrDrainCapExceeded (the retry's own quarantine cannot succeed under a flood)", err)
	}
	if w := port.written(); len(w) > 3 {
		t.Errorf("port saw %d writes, want at most 3 (the initial attempt plus RetryReads=2)", len(w))
	}
}

// TestFlood_WriteCompletesAndQuarantineDoesNotWedge pins the write path.
// The error window elapses on schedule (nothing in the flood is a
// rejection), so the write SUCCEEDS; the unconditional post-write
// quarantine then cannot reach quiet, which is a logged best-effort
// failure that must not change the write's own already-determined outcome
// and must not hang.
func TestFlood_WriteCompletesAndQuarantineDoesNotWedge(t *testing.T) {
	logger := &capturingLogger{}
	port, e := newFloodEngine(t, WithLogger(logger))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	withinBound(t, "Do (write under flood)", 5*time.Second, func() {
		_, err = e.Do(ctx, lineCommand("WR x;"), CommandSpec{
			Class:       ClassWrite,
			ErrorWindow: 30 * time.Millisecond,
			Settle:      time.Millisecond,
		})
	})
	if err != nil {
		t.Fatalf("Do (write under flood) = %v, want nil — nothing in the flood is a rejection, and a failed best-effort quarantine must not change the write's outcome", err)
	}
	if w := port.written(); len(w) != 1 {
		t.Errorf("port saw %d writes, want exactly 1 — a write is never resent", len(w))
	}
	if !logger.contains("post-write quarantine drain") {
		t.Error("the failed post-write quarantine was not logged — safety obligation 3 requires it be surfaced, and its absence would mean the drain silently succeeded against a stream that never goes quiet")
	}
}

// TestFlood_NextDoRefusesToTransmitAfterAFailedQuarantine is the
// consequence that matters most: a failed quarantine leaves the engine
// SUSPECT, and the next Do refuses to transmit into a stream whose state
// it cannot establish — bounded, with ErrQuarantineFailed, rather than
// hanging in the entry drain.
func TestFlood_NextDoRefusesToTransmitAfterAFailedQuarantine(t *testing.T) {
	port, e := newFloodEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First exchange: a write, whose post-write quarantine cannot succeed
	// under the flood, so the engine is left suspect.
	withinBound(t, "Do (first write)", 5*time.Second, func() {
		_, _ = e.Do(ctx, lineCommand("WR x;"), CommandSpec{
			Class:       ClassWrite,
			ErrorWindow: 30 * time.Millisecond,
			Settle:      time.Millisecond,
		})
	})

	var err error
	withinBound(t, "Do (second, entering suspect)", 5*time.Second, func() {
		_, err = e.Do(ctx, lineCommand("RD?;"), CommandSpec{
			Class:   ClassRead,
			Match:   lineMatch("RD "),
			Timeout: 50 * time.Millisecond,
			Settle:  time.Millisecond,
		})
	})
	if !errors.Is(err, ErrQuarantineFailed) {
		t.Fatalf("second Do = %v, want errors.Is match against ErrQuarantineFailed — the entry drain cannot establish quiet under a flood, and transmitting into that is the hazard the mechanism exists to avoid", err)
	}
	if !errors.Is(err, ErrDrainCapExceeded) {
		t.Errorf("second Do = %v, want the underlying ErrDrainCapExceeded reachable through QuarantineFailedError.Unwrap", err)
	}
	if w := port.written(); len(w) != 1 {
		t.Errorf("port saw %d writes, want exactly 1 — the refused second exchange must not have transmitted", len(w))
	}
}

// TestFlood_InitDoesNotWedge covers the session-open path end to end: Init
// transmits its (here empty) sequence and then drains, and under a flood
// that drain fails. It must FAIL, bounded, rather than hang — a radio the
// host cannot open is recoverable; a host that never returns from Open is
// not.
func TestFlood_InitDoesNotWedge(t *testing.T) {
	_, e := newFloodEngine(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	withinBound(t, "Init", 5*time.Second, func() { err = e.Init(ctx) })
	if !errors.Is(err, ErrDrainCapExceeded) {
		t.Fatalf("Init under flood = %v, want errors.Is match against ErrDrainCapExceeded", err)
	}
}

// TestFlood_InitWithACommandDoesNotWedge is the CAT-shaped case: an init
// sequence that DOES transmit, under the same flood.
func TestFlood_InitWithACommandDoesNotWedge(t *testing.T) {
	port := newFloodPort()
	t.Cleanup(func() { _ = port.Close() })

	f := &floodFramingWithInit{floodFraming{policy: DrainPolicy{IdleGap: 20 * time.Millisecond, Cap: 40 * time.Millisecond}}}
	e, err := NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var initErr error
	withinBound(t, "Init (with a command)", 5*time.Second, func() { initErr = e.Init(ctx) })
	if initErr == nil {
		t.Fatal("Init under flood = nil error, want a refusal: the drain cannot reach quiet")
	}
	if w := port.written(); len(w) != 1 {
		t.Errorf("port saw %d writes, want exactly 1 — Init's one init command, transmitted once", len(w))
	}
}

type floodFramingWithInit struct{ floodFraming }

func (f *floodFramingWithInit) InitSequence() []Command { return []Command{lineCommand("AI0;")} }

// --- the entry purge's bound --------------------------------------------

// TestPurgeBuffered_IsBoundedByItsDeadline pins the purge's absolute
// bound. The purge's exit condition is "e.events is momentarily empty",
// which a flood never satisfies: the reader goroutine refills the channel
// as fast as the purge empties it, and a purge that spins forever wedges
// Do exactly as surely as a drain that waits forever would.
//
// The DEADLINE half is what this asserts, because it is the half that can
// be made deterministic: a clock that jumps past the deadline forces the
// bound on the second iteration whatever the channel holds. The frame
// count (maxPurgeFrames) is defence in depth behind it, for a clock that
// does not advance at all.
func TestPurgeBuffered_IsBoundedByItsDeadline(t *testing.T) {
	port := newFloodPort()
	t.Cleanup(func() { _ = port.Close() })

	logger := &capturingLogger{}
	f := &floodFraming{policy: DrainPolicy{IdleGap: 20 * time.Millisecond, Cap: 40 * time.Millisecond}}
	e, err := NewEngineWith(port, f, WithClock(&jumpingClock{step: time.Hour}), WithLogger(logger))
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	// Let the flood fill the events channel, so the purge has something
	// to find and cannot exit through its "nothing buffered" branch on
	// the first iteration.
	time.Sleep(50 * time.Millisecond)

	withinBound(t, "purgeBufferedLocked", 5*time.Second, func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		if err := e.purgeBufferedLocked(); err != nil {
			t.Errorf("purgeBufferedLocked = %v, want nil — hitting the bound is a normal operating state, not a failure", err)
		}
	})

	if !logger.contains("entry purge hit its bound") {
		t.Fatalf("the purge did not report hitting its bound; log was:\n%s", strings.Join(logger.lines(), "\n"))
	}
}

// jumpingClock advances by step on every Now() call, so any deadline is
// passed on the second consultation. After/Sleep delegate to real time:
// the property under test is the DEADLINE COMPARISON, not the timers.
type jumpingClock struct {
	mu   sync.Mutex
	now  time.Time
	step time.Duration
}

func (c *jumpingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.now.IsZero() {
		c.now = time.Now()
	}
	n := c.now
	c.now = c.now.Add(c.step)
	return n
}

func (c *jumpingClock) After(d time.Duration) <-chan time.Time { return time.After(d) }
func (c *jumpingClock) Sleep(d time.Duration)                  { time.Sleep(d) }

// capturingLogger keeps every line for assertion.
type capturingLogger struct {
	mu  sync.Mutex
	buf []string
}

func (l *capturingLogger) Printf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf = append(l.buf, fmt.Sprintf(format, args...))
}

func (l *capturingLogger) lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.buf...)
}

func (l *capturingLogger) contains(sub string) bool {
	for _, line := range l.lines() {
		if strings.Contains(line, sub) {
			return true
		}
	}
	return false
}

// --- the cap as a real ceiling ------------------------------------------

// lateFramePort is quiet, emits ONE frame at `at` after construction, and
// is quiet again forever. It is the opposite provocation to floodPort: a
// line calm enough that the drain reaches nextEvent's BLOCKING select,
// where the loop-entry clock comparison never runs.
type lateFramePort struct {
	frame   []byte
	fire    <-chan time.Time
	done    bool
	closeCh chan struct{}
	mu      sync.Mutex
	closed  bool
}

func newLateFramePort(frame string, at time.Duration) *lateFramePort {
	return &lateFramePort{
		frame:   []byte(frame),
		fire:    time.After(at),
		closeCh: make(chan struct{}),
	}
}

func (p *lateFramePort) Read(b []byte) (int, error) {
	if !p.done {
		select {
		case <-p.fire:
			p.done = true
			return copy(b, p.frame), nil
		case <-p.closeCh:
			return 0, errClosedStub
		}
	}
	<-p.closeCh
	return 0, errClosedStub
}

func (p *lateFramePort) Write(b []byte) (int, error) { return len(b), nil }

func (p *lateFramePort) Close() error {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.closeCh)
	}
	p.mu.Unlock()
	return nil
}

// TestDrainCap_IsACeilingNotAFloor pins DrainPolicy.Cap's documented
// meaning against the case a flood does not reach.
//
// A flood keeps e.events permanently ready, so the drain never blocks and
// nextEvent's loop-entry clock comparison holds the cap. A nearly-quiet
// line is the other shape: the drain DOES block, and there the only
// channels that can wake it are the ones the select names. The idle timer
// is not a bound — every arrival re-arms it, by design — so with no
// cap-shaped case a single stale frame arriving inside (Cap-IdleGap, Cap)
// postpones "quiet" past the cap, and the drain SUCCEEDS at Cap+IdleGap.
//
// That is both a broken promise ("ABSOLUTE ceiling ... fails rather than
// continuing") and later than the pre-D2 internal quarantines, which
// hard-failed at 2*QuietPeriod on their own context. Here: idle gap
// 250ms, cap 400ms, one frame at 180ms — early enough that the first idle
// gap has not run out (so the frame is genuinely observed and the timer
// genuinely re-armed) and late enough that the re-armed gap would declare
// quiet at 430ms. The drain must fail at 400ms instead.
func TestDrainCap_IsACeilingNotAFloor(t *testing.T) {
	const (
		idleGap = 250 * time.Millisecond
		capAt   = 400 * time.Millisecond
		frameAt = 180 * time.Millisecond
	)

	port := newLateFramePort("ZZ0000;", frameAt)
	t.Cleanup(func() { _ = port.Close() })

	f := &floodFraming{policy: DrainPolicy{IdleGap: idleGap, Cap: capAt}}
	e, err := NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	var drainErr error
	withinBound(t, "DrainToQuiet (one late frame)", 5*time.Second, func() { drainErr = e.DrainToQuiet(ctx) })
	elapsed := time.Since(start)

	if !errors.Is(drainErr, ErrDrainCapExceeded) {
		t.Fatalf("DrainToQuiet = %v after %v, want ErrDrainCapExceeded at the cap — a frame at %v re-armed the %v idle gap, so anything else means the drain ran past Cap (%v) to Cap+IdleGap (%v)",
			drainErr, elapsed, frameAt, idleGap, capAt, capAt+idleGap)
	}
	// Sanity: it really waited out the cap rather than failing early for
	// some unrelated reason, and it did not exceed the loose bound.
	if elapsed < capAt*3/4 || elapsed > capAt+idleGap {
		t.Errorf("DrainToQuiet took %v, want roughly %v — the cap, not an early exit and not Cap+IdleGap", elapsed, capAt)
	}
}
