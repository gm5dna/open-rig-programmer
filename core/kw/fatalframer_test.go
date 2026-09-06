// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// This file is core/kw's half of the E;/O; design: the FOUR-STATE INJECTION
// MATRIX the spec requires (§Testing, "Acknowledgement"), and the RE-PIN of
// core/transport's two adversarial pins through THIS package's accumulator
// and THIS package's typed cause.
//
// The two are not the same test and neither replaces the other. The matrix
// asks "in each of the four states the engine can be in, does an E; or an
// O; come back as the typed error naming its own book's sentence, never as
// a rejection, never retried, never matched as an answer?" — a question
// about the CAUSE. The adversarial pair asks "did the failure land where it
// is claimed to land, or merely land eventually?" — a question about ORDER.
// Without the second, the matrix passes by injecting the token and then
// calling Do, which proves deferred detection while reading as proof of
// immediate closure; core/transport/fatalframer_test.go says so in as many
// words and this file is that contract re-pinned one layer up.

// --- fixtures -----------------------------------------------------------

// testPort is a transport.Port whose Read blocks until the test delivers
// bytes, whose writes are recorded, and whose Close is OBSERVABLE — which
// is what makes the post-purge handshake two-way, since closePort calls
// Port.Close from inside the engine's fatal gate.
type testPort struct {
	mu        sync.Mutex
	pending   []byte
	writes    [][]byte
	onWrite   func()
	wake      chan struct{}
	closed    chan struct{}
	closeOnce sync.Once
}

func newTestPort() *testPort {
	return &testPort{wake: make(chan struct{}, 8), closed: make(chan struct{})}
}

func (p *testPort) Read(b []byte) (int, error) {
	for {
		p.mu.Lock()
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
			return 0, errTestPortClosed
		}
	}
}

func (p *testPort) Write(b []byte) (int, error) {
	p.mu.Lock()
	p.writes = append(p.writes, append([]byte(nil), b...))
	hook := p.onWrite
	p.mu.Unlock()
	if hook != nil {
		hook()
	}
	return len(b), nil
}

// Close is the publication's acknowledgement: closePort calls it from
// INSIDE the engine's fatal gate, so a test that waits on p.closed has
// waited for the fatal publication to have COMPLETED, not merely started.
func (p *testPort) Close() error {
	p.closeOnce.Do(func() { close(p.closed) })
	return nil
}

func (p *testPort) written() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([][]byte(nil), p.writes...)
}

// deliver queues bytes for the next Read and wakes it. Everything handed to
// ONE deliver call reaches ONE Accumulator.Push, which is what the
// same-chunk pin depends on.
func (p *testPort) deliver(s string) {
	p.mu.Lock()
	p.pending = append(p.pending, s...)
	p.mu.Unlock()
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

var errTestPortClosed = errors.New("kw test: port closed")

// hookedFraming is this package's own framing with ONE addition: a hook on
// NoteSent, which the engine calls on the Do goroutine between its entry
// purge and its write (engine.go's Do). It is how the post-purge fixture
// reaches that interval, and it is the ONLY difference from what a driver
// gets from NewFraming — the accumulator, IsRejection, IsFatal, the typed
// cause, Allow and the drain policy are all the shipping ones.
//
// It embeds the concrete framing, not the transport.Framing interface: an
// embedded interface contributes only that interface's methods, so IsFatal
// — which is not one of Framing's six — would be lost and the wrapper would
// silently stop being a FatalFramer.
type hookedFraming struct {
	framing
	onNote func()
}

func (f hookedFraming) NoteSent(frame []byte) {
	f.framing.NoteSent(frame)
	if f.onNote != nil {
		f.onNote()
	}
}

// kwFraming builds the shipping framing for book, failing the test if the
// book is refused.
func kwFraming(t *testing.T, book Book) transport.Framing {
	t.Helper()
	f, err := NewFraming(book)
	if err != nil {
		t.Fatalf("NewFraming(%v): %v", book, err)
	}
	return f
}

// idAnswerSpec is a ClassRead spec for the 6-byte ID answer — the shortest
// real exchange this milestone has, used wherever the matrix needs a read
// in flight rather than for anything about identity.
func idAnswerSpec() transport.CommandSpec {
	return transport.CommandSpec{
		Class:   transport.ClassRead,
		Match:   PrefixLenMatcher("ID", 6),
		Timeout: 5 * time.Second,
		Settle:  time.Millisecond,
	}
}

// wantCause is the sentence each (token, book) pair must carry — the same
// table errors_test.go pins the constructor against, restated here so the
// matrix asserts the END-TO-END value a driver recovers rather than
// trusting the constructor it just tested.
func wantCause(token string, book Book) string {
	switch {
	case token == "E;":
		return "A communication error occurred, such as an overrun or framing error during a serial data transmission"
	case book == Book590:
		return "A receive buffer overrun error occurred"
	default:
		return "Receive data was sent but processing was not completed"
	}
}

// assertTypedFatal is every assertion the matrix makes about one outcome,
// in one place: the typed cause, its book's own sentence, the port closed
// with it, and the two negatives — never a rejection, never contamination.
func assertTypedFatal(t *testing.T, err error, token string, book Book) {
	t.Helper()
	if err == nil {
		t.Fatal("the exchange succeeded — a stream-health token must fail it")
	}
	var se *StreamError
	if !errors.As(err, &se) {
		t.Fatalf("err = %v, want errors.As to recover *StreamError", err)
	}
	if se.Token != token {
		t.Errorf("StreamError.Token = %q, want %q", se.Token, token)
	}
	if se.Book != book {
		t.Errorf("StreamError.Book = %v, want %v", se.Book, book)
	}
	if want := wantCause(token, book); se.Cause != want {
		t.Errorf("StreamError.Cause = %q, want this book's own sentence %q", se.Cause, want)
	}
	// Option A's addition to each of the four: the port is CLOSED, and it
	// is closed carrying the typed cause.
	if !errors.Is(err, transport.ErrPortClosed) {
		t.Errorf("err = %v, want errors.Is match against transport.ErrPortClosed — the publication closes the port from the reader goroutine", err)
	}
	if errors.Is(err, transport.ErrRejected) {
		t.Errorf("err = %v matches transport.ErrRejected — a link failure must never be collapsed into the ?; path", err)
	}
	var tooLong *transport.FrameTooLongError
	if errors.As(err, &tooLong) {
		t.Errorf("err = %v matches *transport.FrameTooLongError — the link's end must not be reported as recoverable contamination", err)
	}
}

// --- the four-state injection matrix (M3) -------------------------------

// TestFatalTokens_FourStatesTwoTokensTwoBooks is the matrix the spec
// requires: E; and O; injected in EACH of four states — the read wait, the
// write error-window, the drain, and with no command outstanding — through
// the core/kw accumulator, each asserting the typed error with its OWN
// book's cause sentence, never ErrRejected, never retried, never matched as
// an answer. Sixteen cells: four states x two tokens x two books.
//
// WHAT IT DOES NOT PROVE, said here rather than left for a reader to
// assume: it is a test about the CAUSE, not about the instant. Three of the
// four states inject the token and then observe the outcome, which a merely
// DEFERRED failure would also satisfy. The two adversarial pins below are
// what tell suppression from deferral, and they are why this test is not
// the whole of the design's evidence.
func TestFatalTokens_FourStatesTwoTokensTwoBooks(t *testing.T) {
	states := []struct {
		name string
		run  func(t *testing.T, f transport.Framing, token string) (*testPort, error)
	}{
		{"read wait", runReadWait},
		{"write error-window", runWriteErrorWindow},
		{"drain", runDrain},
		{"no command outstanding", runIdle},
	}
	for _, st := range states {
		for _, token := range []string{"E;", "O;"} {
			for _, book := range []Book{Book590, Book480} {
				t.Run(st.name+"/"+token+"/"+book.String(), func(t *testing.T) {
					f := kwFraming(t, book)
					port, err := st.run(t, f, token)
					assertTypedFatal(t, err, token, book)
					// Never retried: whatever this state wrote, it wrote
					// once. A retransmission after a link failure is the
					// one thing safety obligation 2 forbids outright.
					if w := len(port.written()); w > 1 {
						t.Errorf("the port saw %d writes, want at most 1 — a stream-health token is never retried", w)
					}
				})
			}
		}
	}
}

// runReadWait injects the token while a ClassRead Do is waiting for its
// answer. Delivery happens from INSIDE the port's Write, so the token is
// released strictly after Do has transmitted and is on its way into the
// read wait — no sleep, and no chance of the token preceding the command.
func runReadWait(t *testing.T, f transport.Framing, token string) (*testPort, error) {
	t.Helper()
	port := newTestPort()
	t.Cleanup(func() { _ = port.Close() })
	var once sync.Once
	port.onWrite = func() { once.Do(func() { port.deliver(token) }) }

	e, err := transport.NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	got, doErr := e.Do(ctx, newCommand([]byte("ID;")), idAnswerSpec())
	if got != nil {
		t.Errorf("Do returned frame %q alongside its outcome — a stream-health token is never matched as an answer", got)
	}
	return port, doErr
}

// runWriteErrorWindow injects the token while a fire-and-forget ClassWrite
// Do is listening for a delayed rejection. Same delivery route, same
// ordering guarantee.
func runWriteErrorWindow(t *testing.T, f transport.Framing, token string) (*testPort, error) {
	t.Helper()
	port := newTestPort()
	t.Cleanup(func() { _ = port.Close() })
	var once sync.Once
	port.onWrite = func() { once.Do(func() { port.deliver(token) }) }

	e, err := transport.NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_, doErr := e.Do(ctx, newCommand([]byte("MC007;")), transport.CommandSpec{
		Class:       transport.ClassWrite,
		ErrorWindow: 5 * time.Second,
		Settle:      time.Millisecond,
	})
	return port, doErr
}

// runDrain injects the token around a DrainToQuiet.
//
// THE ORDERING HERE IS NOT PROVED, DELIBERATELY, and the assertion is
// written so that it does not need to be: DrainToQuiet offers no hook by
// which a test can observe that its wait has begun. The token is released
// from a goroutine started immediately before the call, and BOTH
// interleavings satisfy the same assertion — if the drain is already
// waiting it consumes the reader event and closes with the typed cause; if
// the publication got there first, drainToQuietLocked's entry sees the
// closed port and returns the SAME recorded cause through closedErr. What
// this leg proves is the CAUSE a drain reports, which is what the matrix is
// for; ordering is the adversarial pins' subject, not this one's.
func runDrain(t *testing.T, f transport.Framing, token string) (*testPort, error) {
	t.Helper()
	port := newTestPort()
	t.Cleanup(func() { _ = port.Close() })

	e, err := transport.NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	go port.deliver(token)
	return port, e.DrainToQuiet(ctx)
}

// runIdle injects the token with NOTHING outstanding, and this leg IS
// deterministic: it waits for the port's own Close — which closePort calls
// from inside the engine's fatal gate — before issuing the next command. So
// the assertion "the following Do wrote nothing" is ordered by
// construction, not by timing.
//
// It is also the leg that shows what the FatalFramer hook bought. Without
// it, sendEvent would put the token on a 16-deep buffered channel and
// close nothing (engine.go's sendEvent); the OS port would stay open until
// some caller consumed the event, and there is no idle consumer.
func runIdle(t *testing.T, f transport.Framing, token string) (*testPort, error) {
	t.Helper()
	port := newTestPort()
	t.Cleanup(func() { _ = port.Close() })

	e, err := transport.NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	port.deliver(token)
	select {
	case <-port.closed:
	case <-time.After(10 * time.Second):
		t.Fatal("the port was never closed after a token arrived with nothing outstanding — the fatal publication did not run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_, doErr := e.Do(ctx, newCommand([]byte("ID;")), idAnswerSpec())
	if w := len(port.written()); w != 0 {
		t.Errorf("the following Do wrote %d frame(s), want 0 — nothing leaves the host after a fatal frame the engine has received", w)
	}
	return port, doErr
}

// --- the two adversarial pins, re-pinned through core/kw ----------------

// TestFatalTokens_SameChunk_SuppressesTheAnswerItArrivedWith is
// core/transport's clause-1 pin re-run against the real Kenwood
// accumulator, the real "E;" token and the real typed cause.
//
// A chunk of "ID023;E;" reaches Accumulator.Push as ONE call. Without the
// hook the engine delivers the frames first and the error afterwards
// (readLoop), so waitForAnswer matches the ID answer, reports SUCCESS, and
// the link failure lands on the NEXT command — a per-channel write
// verification reporting a match on the very chunk that carried the
// failure. That is spec §"Where E; and O; live" window (i).
//
// FOUR assertions, because two of them are satisfied by the defect they
// were written to catch: "zero later writes" and "errors.As recovers the
// cause" both hold when the failure is merely DEFERRED to the next call.
// Assertion 1 is what tells suppression from deferral.
func TestFatalTokens_SameChunk_SuppressesTheAnswerItArrivedWith(t *testing.T) {
	for _, book := range []Book{Book590, Book480} {
		t.Run(book.String(), func(t *testing.T) {
			port := newTestPort()
			t.Cleanup(func() { _ = port.Close() })
			var once sync.Once
			port.onWrite = func() { once.Do(func() { port.deliver("ID023;E;") }) }

			e, err := transport.NewEngineWith(port, kwFraming(t, book))
			if err != nil {
				t.Fatalf("NewEngineWith: %v", err)
			}
			t.Cleanup(func() { _ = e.Close() })

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			// Assertion 1 — the CURRENT Do fails, and the matching answer
			// never yields a success.
			got, doErr := e.Do(ctx, newCommand([]byte("ID;")), idAnswerSpec())
			if doErr == nil {
				t.Fatalf("Do returned the matching answer %q with no error — the E; in the SAME chunk must suppress it, not merely defer the failure to the next call", got)
			}
			if got != nil {
				t.Errorf("Do returned frame %q alongside its error, want nil", got)
			}
			// Assertions 2 and 3 — the typed cause, and the port closed
			// carrying it.
			assertTypedFatal(t, doErr, "E;", book)

			// Assertion 4 — the FOLLOWING Do performs ZERO further writes
			// and returns the SAME recorded cause.
			writesBefore := len(port.written())
			_, again := e.Do(ctx, newCommand([]byte("ID;")), idAnswerSpec())
			if again == nil {
				t.Fatal("the following Do succeeded, want a refusal on the closed port")
			}
			assertTypedFatal(t, again, "E;", book)
			if w := len(port.written()); w != writesBefore {
				t.Errorf("the following Do wrote %d further frame(s), want 0", w-writesBefore)
			}
		})
	}
}

// TestFatalTokens_PostPurgeRace_NoFrameLeavesAfterAReceivedFatalFrame is
// core/transport's clause-4 pin re-run through this package: the token
// arrives strictly AFTER Do's entry purge has returned and BEFORE its
// write, the interval in which nothing in Do re-reads e.closed and nothing
// orders the two goroutines at all (spec window (iii), which needs no flood
// at all).
//
// THE HANDSHAKE IS TWO-WAY, and that is the whole point. A one-way release
// — "NoteSent lets the bytes go and returns" — would let Do reach the gate
// and write before readLoop had even parsed the chunk, and the test would
// go green on a race it happened to win. So NoteSent releases the bytes and
// then BLOCKS until the publication has COMPLETED, acknowledged by the
// port's own Close, which closePort calls from inside the fatal gate. The
// write attempt is therefore ordered strictly after the publication by
// construction, not by a sleep.
//
// Under Option B — no hook, the route this milestone did not take — the
// same fixture would assert the OPPOSITE: the frame goes out and the
// failure lands on the following command. That inversion is what makes this
// pin the red/green proof of Q13 rather than a restatement of it.
func TestFatalTokens_PostPurgeRace_NoFrameLeavesAfterAReceivedFatalFrame(t *testing.T) {
	for _, book := range []Book{Book590, Book480} {
		t.Run(book.String(), func(t *testing.T) {
			port := newTestPort()
			t.Cleanup(func() { _ = port.Close() })

			var (
				noteOnce      sync.Once
				handshakeDone atomic.Bool
			)
			f := hookedFraming{framing: framing{book: book}}
			f.onNote = func() {
				noteOnce.Do(func() {
					port.deliver("O;")
					select {
					case <-port.closed:
						handshakeDone.Store(true)
					case <-time.After(10 * time.Second):
						// Recorded rather than fatal: the assertions
						// below then fail loudly, and the test does not
						// wedge.
						handshakeDone.Store(false)
					}
				})
			}

			e, err := transport.NewEngineWith(port, f)
			if err != nil {
				t.Fatalf("NewEngineWith: %v", err)
			}
			t.Cleanup(func() { _ = e.Close() })

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, doErr := e.Do(ctx, newCommand([]byte("ID;")), idAnswerSpec())

			if !handshakeDone.Load() {
				t.Fatal("the two-way handshake never completed: the port's Close was not observed within 10s of NoteSent releasing the fatal bytes — the publication did not run, so the assertions below would prove nothing")
			}
			if w := port.written(); len(w) != 0 {
				t.Errorf("the port saw %d write(s) after a RECEIVED fatal frame, want 0: %q", len(w), w)
			}
			assertTypedFatal(t, doErr, "O;", book)
		})
	}
}

// TestHookedFraming_IsStillAFatalFramer guards the fixture above against
// the one way it could go quietly vacuous: if hookedFraming stopped
// implementing transport.FatalFramer, the engine would resolve a nil hook,
// no publication would run, and the post-purge pin would fail on its
// handshake rather than on its subject — but a future edit that also
// relaxed the handshake would leave a green test proving nothing.
func TestHookedFraming_IsStillAFatalFramer(t *testing.T) {
	var f transport.Framing = hookedFraming{framing: framing{book: Book590}}
	ff, ok := f.(transport.FatalFramer)
	if !ok {
		t.Fatal("hookedFraming no longer implements transport.FatalFramer — the post-purge fixture would test nothing")
	}
	if ff.IsFatal([]byte("E;")) == nil {
		t.Error("hookedFraming.IsFatal(\"E;\") = nil — the wrapper is not forwarding the shipping IsFatal")
	}
	if ff.IsFatal([]byte("ID023;")) != nil {
		t.Error("hookedFraming.IsFatal(\"ID023;\") returned a cause for an ordinary frame")
	}
}

// TestFraming_IsFatalNamesTheTwoTokensAndNothingElse pins the hook itself,
// below the engine: only the two tokens are fatal, and "?;" — the NAK — is
// emphatically not, because a refusal is not a link failure.
func TestFraming_IsFatalNamesTheTwoTokensAndNothingElse(t *testing.T) {
	f, err := NewFraming(Book480)
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	ff, ok := f.(transport.FatalFramer)
	if !ok {
		t.Fatal("the shipping framing does not implement transport.FatalFramer — Q13 Option A is what this milestone took")
	}
	for _, tt := range []struct {
		frame string
		fatal bool
	}{
		{"E;", true},
		{"O;", true},
		{"?;", false},
		{"ID020;", false},
		{"EX0000000;", false},
		{";", false},
		{"", false},
	} {
		got := ff.IsFatal([]byte(tt.frame))
		if (got != nil) != tt.fatal {
			t.Errorf("IsFatal(%q) = %v, want fatal=%v", tt.frame, got, tt.fatal)
		}
	}
}
