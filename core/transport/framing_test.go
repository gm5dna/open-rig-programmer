// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// This file proves the D2 seam is real rather than nominal: an Engine
// built over a framing that is NOT CAT — different terminator, different
// rejection frame, an EMPTY init sequence, a real NoteSent — runs the same
// state machine and reaches the same outcomes. It is core/civ's rehearsal,
// written here because core/civ does not exist in this worktree and a seam
// with exactly one implementation is not a seam.

// --- a deliberately non-CAT framing -------------------------------------

// lineFraming frames on '\n' instead of ';', rejects with "NAK", and
// acknowledges writes with "ACK" — a protocol shaped like CI-V's (a NAK
// frame AND an ACK frame) and unlike CAT's (a NAK and silence). Nothing
// about it is a radio; it exists to hold the seam open.
type lineFraming struct {
	init   []Command
	policy DrainPolicy
	allow  func([]byte) bool

	mu   sync.Mutex
	sent [][]byte
}

func (f *lineFraming) NewAccumulator(max int) Accumulator {
	if max <= 0 {
		max = 64
	}
	return &lineAccumulator{max: max}
}

func (f *lineFraming) IsRejection(frame []byte) bool {
	return string(frame) == "NAK\n"
}

func (f *lineFraming) Allow(frame []byte) bool {
	if f.allow == nil {
		return true
	}
	return f.allow(frame)
}

func (f *lineFraming) InitSequence() []Command { return f.init }

func (f *lineFraming) DrainPolicy() DrainPolicy { return f.policy }

// NoteSent honours the contract: it COPIES what it needs and neither
// retains nor mutates the caller's slice. sentFrames' comparison against
// the port's own record is what proves the copy was taken before the
// write; TestNoteSent_DoesNotRetainOrMutateTheEngineSlice takes the
// opposite tack and breaks the contract deliberately, to show what the
// engine guarantees even then.
func (f *lineFraming) NoteSent(frame []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, append([]byte(nil), frame...))
}

func (f *lineFraming) sentFrames() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]byte(nil), f.sent...)
}

// lineAccumulator is lineFraming's Accumulator: '\n'-terminated frames,
// with the same maximum-length contract cat.FrameAccumulator has (an
// oversize run is *FrameTooLongError, and the transport-level sentinel is
// what the engine matches on — the re-export in action).
type lineAccumulator struct {
	buf []byte
	max int
}

func (a *lineAccumulator) Push(chunk []byte) ([][]byte, error) {
	a.buf = append(a.buf, chunk...)
	var frames [][]byte
	for {
		i := bytes.IndexByte(a.buf, '\n')
		if i < 0 {
			break
		}
		raw := a.buf[:i+1]
		if len(raw) > a.max {
			discarded := len(a.buf)
			a.buf = nil
			return frames, &FrameTooLongError{DiscardedLen: discarded}
		}
		frames = append(frames, append([]byte(nil), raw...))
		a.buf = a.buf[i+1:]
	}
	if len(a.buf) > a.max {
		discarded := len(a.buf)
		a.buf = nil
		return frames, &FrameTooLongError{DiscardedLen: discarded}
	}
	return frames, nil
}

// lineCommand is a Command whose Bytes returns a fresh copy per call, as
// the interface requires.
type lineCommand string

func (c lineCommand) Bytes() []byte  { return []byte(c) }
func (c lineCommand) String() string { return fmt.Sprintf("%q", string(c)) }

// lineMatch builds a Match for an answer with the given prefix.
func lineMatch(prefix string) func([]byte) bool {
	return func(frame []byte) bool { return strings.HasPrefix(string(frame), prefix) }
}

// scriptedPort answers each write with the next canned reply, the way a
// radio answers a command. A reply of "" means silence.
type scriptedPort struct {
	mu      sync.Mutex
	replies []string
	writes  [][]byte
	pending []byte
	wake    chan struct{}
	closeCh chan struct{}
	closed  bool
}

func newScriptedPort(replies ...string) *scriptedPort {
	return &scriptedPort{
		replies: replies,
		wake:    make(chan struct{}, 64),
		closeCh: make(chan struct{}),
	}
}

func (p *scriptedPort) Read(b []byte) (int, error) {
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
		case <-p.closeCh:
			return 0, errClosedStub
		}
	}
}

func (p *scriptedPort) Write(b []byte) (int, error) {
	p.mu.Lock()
	p.writes = append(p.writes, append([]byte(nil), b...))
	if len(p.replies) > 0 {
		p.pending = append(p.pending, p.replies[0]...)
		p.replies = p.replies[1:]
	}
	p.mu.Unlock()
	select {
	case p.wake <- struct{}{}:
	default:
	}
	return len(b), nil
}

func (p *scriptedPort) Close() error {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.closeCh)
	}
	p.mu.Unlock()
	return nil
}

func (p *scriptedPort) written() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([][]byte(nil), p.writes...)
}

// fastPolicy is a DrainPolicy scaled for tests: the same two-part shape
// catFraming supplies, an order of magnitude quicker, so a test that must
// WAIT OUT a drain (every flood test below does) costs tens of
// milliseconds rather than hundreds.
var fastPolicy = DrainPolicy{IdleGap: 20 * time.Millisecond, Cap: 40 * time.Millisecond}

// --- the seam itself ----------------------------------------------------

// TestNewEngineWith_NonCATFramingRoundTrips is the seam's existence proof:
// an Engine over a framing with a different terminator, a different
// rejection frame and no init sequence at all completes a read, honours a
// rejection, and reports its NoteSent — none of which core/cat is involved
// in.
func TestNewEngineWith_NonCATFramingRoundTrips(t *testing.T) {
	f := &lineFraming{policy: fastPolicy}
	port := newScriptedPort("RD ok\n", "NAK\n")
	t.Cleanup(func() { _ = port.Close() })

	e, err := NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := e.Do(ctx, lineCommand("RD?\n"), CommandSpec{Class: ClassRead, Match: lineMatch("RD "), Settle: time.Millisecond})
	if err != nil {
		t.Fatalf("Do (read): unexpected error: %v", err)
	}
	if string(got) != "RD ok\n" {
		t.Errorf("Do (read) = %q, want %q", got, "RD ok\n")
	}

	_, err = e.Do(ctx, lineCommand("RD?\n"), CommandSpec{Class: ClassRead, Match: lineMatch("RD "), Settle: time.Millisecond})
	if !errors.Is(err, ErrRejected) {
		t.Errorf("Do (rejected) = %v, want errors.Is match against ErrRejected — the framing's OWN rejection frame, not CAT's \"?;\"", err)
	}

	// The re-export is the point of this assertion: the framing never
	// mentioned core/cat, yet the sentinel a caller compares against is
	// the same value cat.ErrRejected is. That is the recorded wart, made
	// observable.
	sent := f.sentFrames()
	if len(sent) != 2 {
		t.Fatalf("framing was told of %d sent frames, want 2", len(sent))
	}
	for i, s := range sent {
		if string(s) != "RD?\n" {
			t.Errorf("NoteSent[%d] = %q, want %q", i, s, "RD?\n")
		}
	}
}

// TestNewEngineWith_NilFramingIsRefused pins the fail-closed constructor:
// a nil framing is refused before the reader goroutine starts, with a nil
// *Engine, so no half-built Engine exists for a caller to ignore the error
// and use anyway.
func TestNewEngineWith_NilFramingIsRefused(t *testing.T) {
	port := newScriptedPort()
	t.Cleanup(func() { _ = port.Close() })

	e, err := NewEngineWith(port, nil)
	if !errors.Is(err, ErrNoFraming) {
		t.Fatalf("NewEngineWith(port, nil) error = %v, want errors.Is match against ErrNoFraming", err)
	}
	if e != nil {
		t.Fatal("NewEngineWith returned a non-nil *Engine alongside its refusal")
	}
	if got := port.written(); len(got) != 0 {
		t.Errorf("port saw %d writes on the refusal path, want 0 — a refused construction must not touch the port", len(got))
	}
}

// TestNoteSent_HappensBeforeTheWrite pins Framing.NoteSent's contract's
// load-bearing half. An echo-removing accumulator (CI-V's) must have the
// frame recorded before its own echo can arrive, and the echo can arrive
// as soon as the write returns — so "before the write" is an ordering
// requirement, not a stylistic one.
//
// The framing observes the ordering directly: NoteSent asks the port what
// it has seen, and finding this frame ALREADY there would mean the write
// went first.
func TestNoteSent_HappensBeforeTheWrite(t *testing.T) {
	port := newScriptedPort("RD ok\n")
	t.Cleanup(func() { _ = port.Close() })

	var (
		mu             sync.Mutex
		writesAtNote   int
		noteCalls      int
		noteFrameAtNow []byte
	)
	f := &orderingFraming{
		lineFraming: lineFraming{policy: fastPolicy},
		onNote: func(frame []byte) {
			mu.Lock()
			defer mu.Unlock()
			noteCalls++
			writesAtNote = len(port.written())
			noteFrameAtNow = append([]byte(nil), frame...)
		},
	}

	e, err := NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := e.Do(ctx, lineCommand("RD?\n"), CommandSpec{Class: ClassRead, Match: lineMatch("RD "), Settle: time.Millisecond}); err != nil {
		t.Fatalf("Do: unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if noteCalls != 1 {
		t.Fatalf("NoteSent called %d times, want exactly 1 — one per transmission attempt, exactly as Bytes() is (safety obligation 1)", noteCalls)
	}
	if writesAtNote != 0 {
		t.Errorf("the port had already seen %d writes when NoteSent ran, want 0 — NoteSent must precede the write", writesAtNote)
	}
	if string(noteFrameAtNow) != "RD?\n" {
		t.Errorf("NoteSent saw %q, want %q — it must be handed the very frame that goes out", noteFrameAtNow, "RD?\n")
	}
	if w := port.written(); len(w) != 1 || string(w[0]) != "RD?\n" {
		t.Errorf("port writes = %q, want exactly [%q]", w, "RD?\n")
	}
}

// orderingFraming is lineFraming with a NoteSent hook.
type orderingFraming struct {
	lineFraming
	onNote func([]byte)
}

func (f *orderingFraming) NoteSent(frame []byte) {
	if f.onNote != nil {
		f.onNote(frame)
	}
	f.lineFraming.NoteSent(frame)
}

// hostileFraming BREAKS Framing.NoteSent's contract on purpose: it retains
// the engine's own slice instead of copying it, and mutates it in place.
// It exists to show what Engine guarantees against an adapter that does
// the forbidden thing — nothing here is a model for a real framing.
type hostileFraming struct {
	lineFraming

	mu     sync.Mutex
	kept   [][]byte // the engine's slices, RETAINED, never copied
	allowd [][]byte // what the gate was actually handed, copied at the gate
}

func (f *hostileFraming) NoteSent(frame []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.kept = append(f.kept, frame) // forbidden: retention
	if len(frame) > 0 {
		frame[0] = 'X' // forbidden: mutation
	}
}

func (f *hostileFraming) Allow(frame []byte) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.allowd = append(f.allowd, append([]byte(nil), frame...))
	return true
}

func (f *hostileFraming) snapshot() (kept, allowed [][]byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	kept = append([][]byte(nil), f.kept...)
	allowed = append([][]byte(nil), f.allowd...)
	return kept, allowed
}

// TestNoteSent_DoesNotRetainOrMutateTheEngineSlice pins the half of
// Framing.NoteSent's contract that TestNoteSent_HappensBeforeTheWrite does
// not: the slice must be neither retained nor mutated. It asserts from the
// hostile side, since a framing that OBEYS the contract would prove
// nothing — this one retains and mutates, and the engine's guarantees have
// to survive it.
//
// Two properties, both consequences of NoteSent running BEFORE the gate
// rather than between the gate and the write (see Do's transmission
// comment, and safety obligation 1 in doc.go):
//
//   - Mutation cannot divert the gated bytes. Whatever the framing did to
//     the slice, the gate judged THAT, and THAT is byte-for-byte what the
//     port received. Were NoteSent to run after the gate, this framing
//     would put bytes on the wire that the gate never approved — which is
//     precisely the hole the ordering closes.
//   - A retained slice is never rewritten underneath the framing. Every
//     transmission re-derives its frame from Command.Bytes, whose
//     contract is a fresh copy per call, so the slice this framing
//     wrongly kept from the FIRST write still reads as that first write
//     after a second, different command has gone out.
func TestNoteSent_DoesNotRetainOrMutateTheEngineSlice(t *testing.T) {
	port := newScriptedPort("RD ok\n", "WR ok\n")
	t.Cleanup(func() { _ = port.Close() })

	f := &hostileFraming{lineFraming: lineFraming{policy: fastPolicy}}
	e, err := NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := e.Do(ctx, lineCommand("RD?\n"), CommandSpec{Class: ClassRead, Match: lineMatch("RD "), Settle: time.Millisecond}); err != nil {
		t.Fatalf("Do (first): unexpected error: %v", err)
	}
	if _, err := e.Do(ctx, lineCommand("WR?\n"), CommandSpec{Class: ClassRead, Match: lineMatch("WR "), Settle: time.Millisecond}); err != nil {
		t.Fatalf("Do (second): unexpected error: %v", err)
	}

	kept, allowed := f.snapshot()
	written := port.written()

	// The framing mutated the first byte of each frame before the gate
	// ran, so 'X' is what the gate saw and what went out. The literal
	// values are stated rather than derived, so a change of ordering
	// shows up here as a concrete mismatch.
	wantOnWire := []string{"XD?\n", "XR?\n"}
	if len(allowed) != len(wantOnWire) {
		t.Fatalf("gate saw %d frames (%q), want %d", len(allowed), allowed, len(wantOnWire))
	}
	if len(written) != len(wantOnWire) {
		t.Fatalf("port saw %d writes (%q), want %d", len(written), written, len(wantOnWire))
	}
	for i, want := range wantOnWire {
		if string(allowed[i]) != want {
			t.Errorf("gate saw %q at %d, want %q — NoteSent runs before the gate, so the gate must judge the MUTATED bytes", allowed[i], i, want)
		}
		if !bytes.Equal(allowed[i], written[i]) {
			t.Errorf("write %d: gate approved %q but the port received %q — nothing may come between the check and the write (safety obligation 1)", i, allowed[i], written[i])
		}
	}

	// Retention: the slice kept from the first transmission must still
	// hold the first transmission's bytes.
	if len(kept) != 2 {
		t.Fatalf("NoteSent called %d times, want 2 — one per transmission attempt", len(kept))
	}
	if string(kept[0]) != "XD?\n" {
		t.Errorf("the slice retained from write 0 now reads %q, want %q — a later attempt must re-derive its own frame, never reuse this one", kept[0], "XD?\n")
	}
	if string(kept[1]) != "XR?\n" {
		t.Errorf("the slice retained from write 1 reads %q, want %q", kept[1], "XR?\n")
	}
	if &kept[0][0] == &kept[1][0] {
		t.Error("both transmissions were handed the SAME backing array — Command.Bytes must return a fresh copy per call")
	}
}

// TestEngineInit_EmptyInitSequenceWritesNothing is the CI-V property D2
// calls a safety guarantee rather than an omission (adjudication 3): Init
// performs NO radio mutation when the framing's init sequence is empty. It
// drains and nothing else — no transceive write, no pre-identity
// mutation, nothing outside the consent regime.
func TestEngineInit_EmptyInitSequenceWritesNothing(t *testing.T) {
	f := &lineFraming{policy: fastPolicy} // no init commands
	port := newScriptedPort()
	t.Cleanup(func() { _ = port.Close() })

	e, err := NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := e.Init(ctx); err != nil {
		t.Fatalf("Init: unexpected error: %v", err)
	}

	if got := port.written(); len(got) != 0 {
		t.Errorf("Init wrote %q, want NOTHING — an empty InitSequence must not mutate the radio", got)
	}
	if got := f.sentFrames(); len(got) != 0 {
		t.Errorf("Init reported %d sent frames, want 0", len(got))
	}
}

// TestEngineInit_SendsEveryInitCommandInOrder pins the general case the
// CAT wrapper is one instance of: each init command is transmitted, in
// order, as a fire-and-forget write.
func TestEngineInit_SendsEveryInitCommandInOrder(t *testing.T) {
	f := &lineFraming{
		init:   []Command{lineCommand("A\n"), lineCommand("B\n")},
		policy: fastPolicy,
	}
	port := newScriptedPort()
	t.Cleanup(func() { _ = port.Close() })

	e, err := NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := e.Init(ctx); err != nil {
		t.Fatalf("Init: unexpected error: %v", err)
	}

	got := port.written()
	if len(got) != 2 || string(got[0]) != "A\n" || string(got[1]) != "B\n" {
		t.Errorf("Init wrote %q, want [\"A\\n\" \"B\\n\"] in that order", got)
	}
}

// --- ClassWriteWithAck --------------------------------------------------

// TestClassWriteWithAck_WaitsForTheAck pins the class CAT has no form of
// and CI-V does: the write waits for the codec's acknowledgement answer
// and returns it.
func TestClassWriteWithAck_WaitsForTheAck(t *testing.T) {
	f := &lineFraming{policy: fastPolicy}
	port := newScriptedPort("ACK\n")
	t.Cleanup(func() { _ = port.Close() })

	e, err := NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := e.Do(ctx, lineCommand("WR x\n"), CommandSpec{
		Class:   ClassWriteWithAck,
		Match:   lineMatch("ACK"),
		Timeout: 500 * time.Millisecond,
		Settle:  time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Do (write-with-ack): unexpected error: %v", err)
	}
	if string(got) != "ACK\n" {
		t.Errorf("Do (write-with-ack) = %q, want %q", got, "ACK\n")
	}
	if w := port.written(); len(w) != 1 {
		t.Errorf("port saw %d writes, want exactly 1", len(w))
	}
}

// TestClassWriteWithAck_TimeoutNeverRetransmits is the safety proof: a
// write whose acknowledgement never arrives has an UNKNOWN outcome, and
// the one thing that must not happen is sending it again to find out
// (safety obligation 2 — an acknowledged write is still a write). The
// port's write count is the whole assertion.
func TestClassWriteWithAck_TimeoutNeverRetransmits(t *testing.T) {
	f := &lineFraming{policy: fastPolicy}
	port := newScriptedPort() // silence: no ack ever comes
	t.Cleanup(func() { _ = port.Close() })

	e, err := NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = e.Do(ctx, lineCommand("WR x\n"), CommandSpec{
		Class:   ClassWriteWithAck,
		Match:   lineMatch("ACK"),
		Timeout: 50 * time.Millisecond,
		Settle:  time.Millisecond,
	})
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("Do (write-with-ack, no ack) = %v, want errors.Is match against ErrTimeout", err)
	}
	if w := port.written(); len(w) != 1 {
		t.Fatalf("port saw %d writes (%q), want EXACTLY 1 — a write is never resent, whatever its class", len(w), w)
	}
}

// TestClassWriteWithAck_RetryReadsIsRefused pins the structural half:
// asking for a retransmission is refused BEFORE anything reaches the wire,
// not merely declined afterwards.
func TestClassWriteWithAck_RetryReadsIsRefused(t *testing.T) {
	f := &lineFraming{policy: fastPolicy}
	port := newScriptedPort("ACK\n")
	t.Cleanup(func() { _ = port.Close() })

	e, err := NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = e.Do(ctx, lineCommand("WR x\n"), CommandSpec{
		Class:      ClassWriteWithAck,
		Match:      lineMatch("ACK"),
		RetryReads: 1,
	})
	if !errors.Is(err, ErrInvalidSpec) {
		t.Fatalf("Do (write-with-ack, RetryReads 1) = %v, want errors.Is match against ErrInvalidSpec", err)
	}
	if w := port.written(); len(w) != 0 {
		t.Errorf("port saw %d writes, want 0 — an invalid spec is refused before anything is transmitted", len(w))
	}
}

// --- DrainPolicy --------------------------------------------------------

func TestDrainPolicy_WithDefaults(t *testing.T) {
	got := DrainPolicy{}.withDefaults()
	if got.IdleGap != QuietPeriod {
		t.Errorf("IdleGap = %v, want %v", got.IdleGap, QuietPeriod)
	}
	if got.Cap != 2*QuietPeriod {
		t.Errorf("Cap = %v, want %v (twice the idle gap: room for one postponement)", got.Cap, 2*QuietPeriod)
	}

	explicit := DrainPolicy{IdleGap: 5 * time.Millisecond}.withDefaults()
	if explicit.IdleGap != 5*time.Millisecond || explicit.Cap != 10*time.Millisecond {
		t.Errorf("withDefaults(%v) = %+v, want the cap derived from the SUPPLIED gap", 5*time.Millisecond, explicit)
	}

	both := DrainPolicy{IdleGap: time.Second, Cap: time.Minute}.withDefaults()
	if both.IdleGap != time.Second || both.Cap != time.Minute {
		t.Errorf("withDefaults overrode explicit values: %+v", both)
	}
}

// TestCATFraming_DrainPolicyIsUnchangedTiming pins the byte-identity
// claim's timing half for CAT: the values D2 moved into a policy are the
// ones the engine used before it, hardcoded.
func TestCATFraming_DrainPolicyIsUnchangedTiming(t *testing.T) {
	got := catFraming{}.DrainPolicy()
	if got.IdleGap != QuietPeriod {
		t.Errorf("IdleGap = %v, want QuietPeriod (%v) — drainToQuietLocked's own timer value before D2", got.IdleGap, QuietPeriod)
	}
	if got.Cap != 2*QuietPeriod {
		t.Errorf("Cap = %v, want 2*QuietPeriod (%v) — quarantineContext's own bound before D2", got.Cap, 2*QuietPeriod)
	}
}
