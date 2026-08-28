// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// --- Init sends AI0 and drains ---

// TestEngine_Init_SendsAI0AndDrains exercises Init against fakeradio's
// real, independent CAT parser: AI0; must be accepted (fakeradio's own
// grammar agrees with core/cat's — nothing rejected), and DrainToQuiet must
// complete without hanging. core/cat has no "AI;" READ builder (only
// BuildAISet for the two Set forms — see ai.go), so a wire-level round trip
// verifying the exact echoed state isn't possible from this package; the
// companion test below (TestEngine_Init_WritesExactlyAI0) pins the precise
// bytes written using a stub Port instead.
func TestEngine_Init_SendsAI0AndDrains(t *testing.T) {
	_, eng := newTestEngine(t, nil)
	ctx := testCtx(t)

	if err := eng.Init(ctx); err != nil {
		t.Fatalf("Init: unexpected error: %v", err)
	}
}

// TestEngine_Init_WritesExactlyAI0 pins Init's wire-level behaviour: exactly
// one write, byte-for-byte identical to cat.FT710.BuildAISet(false).Bytes()
// ("AI0;"), before it moves on to draining. A stub Port is used here
// (rather than fakeradio) specifically because the property under test is
// "what bytes did Do's write call actually send", which fakeradio's
// black-box Port doesn't let a test observe directly.
func TestEngine_Init_WritesExactlyAI0(t *testing.T) {
	port := newStubPort("") // no replies at all: AI0's error window and the drain both just see silence
	t.Cleanup(func() { _ = port.Close() })
	eng, err := NewEngine(port, cat.FT710)
	if err != nil {
		t.Fatalf("NewEngine: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := eng.Init(ctx); err != nil {
		t.Fatalf("Init: unexpected error: %v", err)
	}

	port.mu.Lock()
	writes := port.writes
	port.mu.Unlock()

	if len(writes) != 1 {
		t.Fatalf("Init wrote %d frames, want exactly 1: %q", len(writes), writes)
	}
	want := cat.FT710.BuildAISet(false).Bytes()
	if string(writes[0]) != string(want) {
		t.Errorf("Init wrote %q, want %q", writes[0], want)
	}
}

// --- Close: idempotent, unblocks in-flight Do ---

func TestEngine_Close_Idempotent(t *testing.T) {
	_, eng := newTestEngine(t, nil)

	err1 := eng.Close()
	err2 := eng.Close()
	if err1 != nil {
		t.Errorf("first Close: unexpected error: %v", err1)
	}
	if err2 != nil {
		t.Errorf("second Close: unexpected error: %v", err2)
	}
}

func TestEngine_Close_UnblocksInFlightDo(t *testing.T) {
	// FaultDropReplies(1) makes the fake genuinely silent for exchange 1
	// onward, so a Do call with a long Timeout is guaranteed to be
	// blocked, waiting, when Close fires — not racing to finish first.
	_, eng := newTestEngine(t, []fakeradio.Option{
		fakeradio.WithFault(fakeradio.FaultDropReplies(1)),
	})
	ctx := testCtx(t)

	idCmd := cat.FT710.BuildIDRead()
	done := make(chan error, 1)
	go func() {
		_, err := eng.Do(ctx, idCmd, CommandSpec{Class: ClassRead, Match: cat.PrefixLenMatcher("ID", 7), Timeout: 30 * time.Second})
		done <- err
	}()

	// Give the goroutine a moment to actually enter its blocking wait
	// before we close.
	time.Sleep(50 * time.Millisecond)

	closeStart := time.Now()
	if err := eng.Close(); err != nil {
		t.Fatalf("Close: unexpected error: %v", err)
	}
	closeElapsed := time.Since(closeStart)
	if closeElapsed > 2*time.Second {
		t.Errorf("Close took %v to return; want it to join promptly", closeElapsed)
	}

	select {
	case err := <-done:
		if !errors.Is(err, ErrPortClosed) {
			t.Errorf("blocked Do returned %v, want errors.Is match against ErrPortClosed", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("blocked Do never returned after Close — it must be unblocked, not left hanging")
	}
}

// --- Goroutine-leak check ---

func TestEngine_Close_NoGoroutineLeak(t *testing.T) {
	baseline := runtime.NumGoroutine()

	const n = 25
	for i := 0; i < n; i++ {
		r := fakeradio.New()
		eng, err := NewEngine(r.Port(), cat.FT710)
		if err != nil {
			t.Fatalf("iteration %d: NewEngine: unexpected error: %v", i, err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, _ = eng.Do(ctx, cat.FT710.BuildIDRead(), CommandSpec{Class: ClassRead, Match: cat.PrefixLenMatcher("ID", 7)})
		cancel()

		if err := eng.Close(); err != nil {
			t.Fatalf("iteration %d: Close: unexpected error: %v", i, err)
		}
		_ = r.Close()
	}

	// Poll for goroutine count to settle back near baseline — the Go
	// runtime/GC can lag briefly even once every goroutine we care about
	// has actually exited (Close() already joined the reader goroutine
	// synchronously per-iteration above, so this poll is a defensive
	// margin against runtime bookkeeping/finalizer goroutines, not
	// evidence Close itself is asynchronous).
	const tolerance = 3
	deadline := time.Now().Add(3 * time.Second)
	var final int
	for {
		final = runtime.NumGoroutine()
		if final <= baseline+tolerance || time.Now().After(deadline) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if final > baseline+tolerance {
		t.Errorf("NumGoroutine() = %d after %d Engine lifecycles, want <= baseline(%d)+%d", final, n, baseline, tolerance)
	}
}

// --- Concurrent Do serialisation ---

func TestEngine_ConcurrentDo_Serialises(t *testing.T) {
	_, eng := newTestEngine(t, nil)

	const goroutines = 12
	const opsPerGoroutine = 3

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*opsPerGoroutine)

	for g := 0; g < goroutines; g++ {
		g := g
		wg.Add(1)
		go func() {
			defer wg.Done()
			slot, err := cat.FT710.MemorySlot(10 + g)
			if err != nil {
				errCh <- fmt.Errorf("goroutine %d: MemorySlot: %w", g, err)
				return
			}
			mode, _ := cat.FT710.ParseMode('2')
			ctcss, _ := cat.ParseCTCSSState('0')
			shift, _ := cat.ParseShift('0')

			for i := 0; i < opsPerGoroutine; i++ {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				freq := uint32(7000000 + g*1000 + i)
				writeCmd, err := cat.FT710.BuildMWSet(cat.MemoryData{Slot: slot, FreqHz: freq, Mode: mode, Kind: cat.KindMemory, CTCSS: ctcss, Shift: shift})
				if err != nil {
					errCh <- fmt.Errorf("goroutine %d op %d: BuildMWSet: %w", g, i, err)
					cancel()
					continue
				}
				if _, err := eng.Do(ctx, writeCmd, CommandSpec{Class: ClassWrite, ErrorWindow: 10 * time.Millisecond, Settle: time.Millisecond}); err != nil {
					errCh <- fmt.Errorf("goroutine %d op %d: MW Do: %w", g, i, err)
					cancel()
					continue
				}

				readCmd, err := cat.FT710.BuildMRRead(slot)
				if err != nil {
					errCh <- fmt.Errorf("goroutine %d op %d: BuildMRRead: %w", g, i, err)
					cancel()
					continue
				}
				got, err := eng.Do(ctx, readCmd, CommandSpec{Class: ClassRead, Match: cat.PrefixLenMatcher("MR", 28), Timeout: time.Second, Settle: time.Millisecond})
				cancel()
				if err != nil {
					errCh <- fmt.Errorf("goroutine %d op %d: MR Do: %w", g, i, err)
					continue
				}
				m, err := cat.FT710.ParseMRAnswer(got)
				if err != nil {
					errCh <- fmt.Errorf("goroutine %d op %d: ParseMRAnswer(%q): %w", g, i, got, err)
					continue
				}
				if m.FreqHz != freq {
					errCh <- fmt.Errorf("goroutine %d op %d: read back FreqHz=%d, want %d (a concurrent write from ANOTHER goroutine corrupted this exchange — serialisation failed)", g, i, m.FreqHz, freq)
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
}

// --- WithClock wiring (isolated unit test, no fakeradio) ---

// stubPort is a minimal, deterministic Port for tests that want to isolate
// Engine's own logic from fakeradio's real elapsed time entirely — used
// only for the WithClock wiring test below, where the point IS to avoid
// any real timing dependency.
type stubPort struct {
	mu       sync.Mutex
	toRead   []byte
	writes   [][]byte
	closed   bool
	closeSig chan struct{}
	// replyGate is nil for newStubPort (reply available immediately) and
	// non-nil for newReplyOnWriteStubPort, where Read parks on it until the
	// first Write closes it. See newReplyOnWriteStubPort for why both
	// behaviours are needed.
	replyGate chan struct{}
	gateOnce  sync.Once
}

func newStubPort(reply string) *stubPort {
	return &stubPort{toRead: []byte(reply), closeSig: make(chan struct{})}
}

// newReplyOnWriteStubPort withholds the canned reply until the first Write,
// the way a real radio answers a command rather than volunteering bytes.
//
// Use it for any test that performs a request/response EXCHANGE. The plain
// newStubPort hands its reply to the first Read regardless of any write,
// which races Engine construction: NewEngine starts the reader goroutine
// immediately, so under load the reader can consume and buffer the reply
// BEFORE Do runs — whereupon Do's entry purge (purgeBufferedLocked)
// correctly discards it as a stale unsolicited frame, and the read then
// times out with its answer already thrown away. Measured at 1 failure in
// 480 runs of the settle/clock tests at 16-way concurrency on 11 cores.
//
// Note the failure is a LOST reply, not a late one, so raising timeouts
// does not fix it: a 10s bound was tried first and failed identically, just
// 10s later. That is the tell for this class — a bigger timeout that buys
// nothing means the data is gone, not slow.
//
// newStubPort is deliberately left ungated rather than replaced:
// TestNewEngine_UnconfiguredDialectIsRefused detects a wrongly started
// reader goroutine precisely BY its consuming those bytes with no write,
// so gating it would make that guard pass whether or not the goroutine
// started.
func newReplyOnWriteStubPort(reply string) *stubPort {
	return &stubPort{
		toRead:    []byte(reply),
		closeSig:  make(chan struct{}),
		replyGate: make(chan struct{}),
	}
}

func (p *stubPort) Read(b []byte) (int, error) {
	if p.replyGate != nil {
		select {
		case <-p.replyGate:
		case <-p.closeSig:
			return 0, errClosedStub
		}
	}
	p.mu.Lock()
	if len(p.toRead) > 0 {
		n := copy(b, p.toRead)
		p.toRead = p.toRead[n:]
		p.mu.Unlock()
		return n, nil
	}
	p.mu.Unlock()
	<-p.closeSig
	return 0, errClosedStub
}

func (p *stubPort) Write(b []byte) (int, error) {
	p.mu.Lock()
	cp := append([]byte(nil), b...)
	p.writes = append(p.writes, cp)
	p.mu.Unlock()
	// Release the gated reply, if this port has one. Done after recording
	// the write so a reader that wakes immediately cannot observe a reply
	// the write list does not yet account for.
	if p.replyGate != nil {
		p.gateOnce.Do(func() { close(p.replyGate) })
	}
	return len(b), nil
}

func (p *stubPort) Close() error {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.closeSig)
	}
	p.mu.Unlock()
	return nil
}

var errClosedStub = errors.New("stubPort: closed")

// fakeClock is a manual clock for the WithClock wiring test. Only Sleep is
// actually faked (recorded, not slept) — the property under test is "Do
// consults the injected clock's Sleep for Settle, not a hardcoded real
// sleep". After deliberately DELEGATES to the real time.After: making it
// return an already-fired channel would race every select in
// waitForAnswer/waitFireAndForget against genuinely arriving frames (the
// timeout branch would be "ready" from the instant the select starts,
// before the reader goroutine has had any chance to deliver the real
// answer), which is a correctness hazard, not a useful thing to fake.
type fakeClock struct {
	mu          sync.Mutex
	sleeps      []time.Duration
	sleepCalled chan struct{}
}

func newFakeClock() *fakeClock {
	return &fakeClock{sleepCalled: make(chan struct{}, 64)}
}

func (c *fakeClock) Now() time.Time                         { return time.Now() }
func (c *fakeClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

func (c *fakeClock) Sleep(d time.Duration) {
	c.mu.Lock()
	c.sleeps = append(c.sleeps, d)
	c.mu.Unlock()
	c.sleepCalled <- struct{}{}
}

func TestEngine_WithClock_SettleUsesInjectedClock(t *testing.T) {
	// Gated: this is a real exchange, so the reply must not be able to
	// arrive before the request. See newReplyOnWriteStubPort.
	port := newReplyOnWriteStubPort("ID0800;")
	t.Cleanup(func() { _ = port.Close() })
	fc := newFakeClock()
	eng, err := NewEngine(port, cat.FT710, WithClock(fc))
	if err != nil {
		t.Fatalf("NewEngine: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	settle := 37 * time.Millisecond
	_, err = eng.Do(ctx, cat.FT710.BuildIDRead(), CommandSpec{Class: ClassRead, Match: cat.PrefixLenMatcher("ID", 7), Settle: settle})
	if err != nil {
		t.Fatalf("Do: unexpected error: %v", err)
	}

	select {
	case <-fc.sleepCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("fakeClock.Sleep was never called — Settle did not use the injected clock")
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()
	if len(fc.sleeps) != 1 || fc.sleeps[0] != settle {
		t.Errorf("fakeClock.sleeps = %v, want exactly [%v]", fc.sleeps, settle)
	}
}

// TestEngine_Settle_AppliesAfterRejectionToo pins radioResponded's rule: a
// "?;" rejection is just as much a completed exchange as a matched answer
// — the radio parsed the command and replied — so Settle applies to it too,
// not only to success. Only a genuine non-response (ErrTimeout) skips it.
func TestEngine_Settle_AppliesAfterRejectionToo(t *testing.T) {
	// Gated for the same reason as the clock test above: a "?;" rejection
	// is still an answer to a command, and must not precede it.
	port := newReplyOnWriteStubPort("?;")
	t.Cleanup(func() { _ = port.Close() })
	fc := newFakeClock()
	eng, err := NewEngine(port, cat.FT710, WithClock(fc))
	if err != nil {
		t.Fatalf("NewEngine: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	settle := 41 * time.Millisecond
	_, err = eng.Do(ctx, cat.FT710.BuildIDRead(), CommandSpec{Class: ClassRead, Match: cat.PrefixLenMatcher("ID", 7), Settle: settle})
	if !errors.Is(err, cat.ErrRejected) {
		t.Fatalf("Do = %v, want errors.Is match against cat.ErrRejected", err)
	}

	select {
	case <-fc.sleepCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("fakeClock.Sleep was never called after a rejection — Settle should still apply")
	}

	fc.mu.Lock()
	defer fc.mu.Unlock()
	if len(fc.sleeps) != 1 || fc.sleeps[0] != settle {
		t.Errorf("fakeClock.sleeps = %v, want exactly [%v]", fc.sleeps, settle)
	}
}

// --- WithMaxFrame wiring (isolated unit test) ---

func TestEngine_WithMaxFrame_TriggersContaminationSooner(t *testing.T) {
	// A reply with no terminator, longer than a small WithMaxFrame, must
	// contaminate — proving the option's value actually reaches the
	// reader goroutine's accumulator (see engine_faults_test.go's
	// TestEngine_Contamination_DrainToQuiet_Recovers for the
	// fakeradio-driven equivalent; this is the isolated, stub-port
	// version pinning WithMaxFrame's wiring specifically).
	overLong := make([]byte, 20) // no ';' at all
	for i := range overLong {
		overLong[i] = 'Z'
	}
	port := newStubPort(string(overLong))
	t.Cleanup(func() { _ = port.Close() })
	eng, err := NewEngine(port, cat.FT710, WithMaxFrame(8))
	if err != nil {
		t.Fatalf("NewEngine: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = eng.Do(ctx, cat.FT710.BuildIDRead(), CommandSpec{Class: ClassRead, Match: cat.PrefixLenMatcher("ID", 7), Timeout: time.Second})
	if !errors.Is(err, ErrContaminated) {
		t.Fatalf("Do = %v, want errors.Is match against ErrContaminated (WithMaxFrame(8) should have made a 20-byte unterminated reply overflow)", err)
	}
}
