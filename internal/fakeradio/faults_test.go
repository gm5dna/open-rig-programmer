// SPDX-License-Identifier: GPL-3.0-or-later

package fakeradio

import (
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// --- FaultDropReplies ---

func TestFaultDropReplies(t *testing.T) {
	_, conn := newTestRadio(t, WithFault(FaultDropReplies(2)))

	writeFrame(t, conn, "ID;")
	if got, want := mustReadFrame(t, conn), "ID0800;"; got != want {
		t.Fatalf("exchange 1 = %q, want %q (before the drop takes effect)", got, want)
	}

	writeFrame(t, conn, "ID;")
	assertNoReply(t, conn) // exchange 2: dropped

	writeFrame(t, conn, "ID;")
	assertNoReply(t, conn) // exchange 3: still dropped ("from exchange N onward")
}

// --- FaultGarbleReply ---

func TestFaultGarbleReply(t *testing.T) {
	_, conn := newTestRadio(t, WithFault(FaultGarbleReply(2)))

	writeFrame(t, conn, "ID;")
	if got, want := mustReadFrame(t, conn), "ID0800;"; got != want {
		t.Fatalf("exchange 1 = %q, want %q (unaffected)", got, want)
	}

	writeFrame(t, conn, "ID;")
	got := mustReadFrame(t, conn)
	if len(got) != len("ID0800;") {
		t.Fatalf("exchange 2 length = %d, want %d (garble must not change length)", len(got), len("ID0800;"))
	}
	if got[0] != 'I'^0xFF {
		t.Errorf("exchange 2 first byte = %q, want %q ('I' with every bit flipped)", got[0], byte('I')^0xFF)
	}
	if got[1:] != "D0800;" {
		t.Errorf("exchange 2 remainder = %q, want %q (only the first byte should be corrupted)", got[1:], "D0800;")
	}

	writeFrame(t, conn, "ID;")
	if got, want := mustReadFrame(t, conn), "ID0800;"; got != want {
		t.Errorf("exchange 3 = %q, want %q (garble was scripted for exchange 2 only)", got, want)
	}
}

// --- FaultSpuriousFrame ---

func TestFaultSpuriousFrame(t *testing.T) {
	spurious := []byte("FA00007000000;")
	_, conn := newTestRadio(t, WithFault(FaultSpuriousFrame(spurious, 2)))

	writeFrame(t, conn, "ID;")
	if got, want := mustReadFrame(t, conn), "ID0800;"; got != want {
		t.Fatalf("exchange 1 = %q, want %q", got, want)
	}

	writeFrame(t, conn, "ID;")
	// The spurious frame arrives BEFORE exchange 2's normal reply.
	if got, want := mustReadFrame(t, conn), string(spurious); got != want {
		t.Errorf("spurious frame = %q, want %q", got, want)
	}
	if got, want := mustReadFrame(t, conn), "ID0800;"; got != want {
		t.Errorf("exchange 2's own reply = %q, want %q", got, want)
	}
}

func TestFaultSpuriousFrame_CopiesInput(t *testing.T) {
	buf := []byte("ID0800;")
	f := FaultSpuriousFrame(buf, 1)
	buf[0] = 'X' // mutate the caller's slice after registering the fault
	_, conn := newTestRadio(t, WithFault(f))

	writeFrame(t, conn, "AI;") // any command that gets a normal reply
	if got, want := mustReadFrame(t, conn), "ID0800;"; got != want {
		t.Errorf("spurious frame = %q, want %q (must not alias the caller's slice)", got, want)
	}
}

// --- FaultDelayedRejection ---

func TestFaultDelayedRejection(t *testing.T) {
	// NOTE: this deliberately does ONE readOneFrame call, not "assert
	// nothing arrives within a short window, then assert something
	// arrives within a longer one": net.Pipe's Write is a rendezvous
	// with whichever Read is CURRENTLY blocked, so a first, short-timeout
	// read that times out leaves its underlying goroutine's Read still
	// blocked — and that abandoned Read, not a second call's fresh one,
	// is what the delayed write would actually rendezvous with,
	// silently swallowing the reply from the test's point of view.
	// Measuring elapsed time around a single read proves "late" without
	// that hazard.
	delay := 200 * time.Millisecond
	_, conn := newTestRadio(t, WithFault(FaultDelayedRejection(1, delay)))

	start := time.Now()
	writeFrame(t, conn, "ID;")

	frame, err, timedOut := readOneFrame(t, conn, time.Second)
	elapsed := time.Since(start)
	if timedOut {
		t.Fatal("the delayed \"?;\" never arrived within 1s")
	}
	if err != nil && len(frame) == 0 {
		t.Fatalf("unexpected error waiting for the delayed rejection: %v", err)
	}
	if got, want := string(frame), "?;"; got != want {
		t.Errorf("delayed reply = %q, want %q — overriding what would normally be a successful ID0800; answer", got, want)
	}
	if elapsed < delay {
		t.Errorf("reply arrived after %v, want at least %v (it must genuinely be LATE, not immediate)", elapsed, delay)
	}
}

// --- FaultDelayedReply ---

func TestFaultDelayedReply(t *testing.T) {
	// Same single-read hazard as TestFaultDelayedRejection above applies
	// here — see that test's comment.
	delay := 200 * time.Millisecond
	_, conn := newTestRadio(t, WithFault(FaultDelayedReply(1, delay)))

	start := time.Now()
	writeFrame(t, conn, "ID;")

	frame, err, timedOut := readOneFrame(t, conn, time.Second)
	elapsed := time.Since(start)
	if timedOut {
		t.Fatal("the delayed reply never arrived within 1s")
	}
	if err != nil && len(frame) == 0 {
		t.Fatalf("unexpected error waiting for the delayed reply: %v", err)
	}
	if got, want := string(frame), "ID0800;"; got != want {
		t.Errorf("delayed reply = %q, want %q — content must be the NORMAL reply, unlike FaultDelayedRejection which overrides it to \"?;\"", got, want)
	}
	if elapsed < delay {
		t.Errorf("reply arrived after %v, want at least %v (it must genuinely be LATE, not immediate)", elapsed, delay)
	}
}

func TestFaultDelayedReply_OnlyAffectsScriptedExchange(t *testing.T) {
	_, conn := newTestRadio(t, WithFault(FaultDelayedReply(2, 150*time.Millisecond)))

	start := time.Now()
	writeFrame(t, conn, "ID;")
	if got, want := mustReadFrame(t, conn), "ID0800;"; got != want {
		t.Fatalf("exchange 1 = %q, want %q (unaffected — delay scripted for exchange 2 only)", got, want)
	}
	if elapsed := time.Since(start); elapsed >= 150*time.Millisecond {
		t.Errorf("exchange 1 took %v, want well under 150ms (the fault must not apply yet)", elapsed)
	}

	start2 := time.Now()
	writeFrame(t, conn, "ID;")
	if got, want := mustReadFrame(t, conn), "ID0800;"; got != want {
		t.Fatalf("exchange 2 = %q, want %q", got, want)
	}
	if elapsed := time.Since(start2); elapsed < 150*time.Millisecond {
		t.Errorf("exchange 2 arrived after %v, want at least 150ms", elapsed)
	}
}

func TestFaultDelayedReply_ReturnsNaturalRejectionWhenApplicable(t *testing.T) {
	// The delay applies to whatever the reply would NORMALLY have been —
	// including a natural "?;" rejection (e.g. reading an unpopulated
	// slot) — NOT an override to always-success, unlike FaultDelayedReply
	// being confused for FaultDelayedRejection's opposite. Exercises the
	// "reply != nil but IS a rejection" path through the same delay.
	_, conn := newTestRadio(t, WithFault(FaultDelayedReply(1, 100*time.Millisecond)))

	start := time.Now()
	writeFrame(t, conn, "MR007;") // M-07: not in the factory image
	frame, _, timedOut := readOneFrame(t, conn, time.Second)
	elapsed := time.Since(start)
	if timedOut {
		t.Fatal("the delayed reply never arrived within 1s")
	}
	if got, want := string(frame), "?;"; got != want {
		t.Errorf("delayed reply = %q, want %q (the NATURAL empty-slot rejection, delayed)", got, want)
	}
	if elapsed < 100*time.Millisecond {
		t.Errorf("reply arrived after %v, want at least 100ms", elapsed)
	}
}

func TestFaultDelayedReply_ComposesWithGarble(t *testing.T) {
	_, conn := newTestRadio(t,
		WithFault(FaultDelayedReply(1, 100*time.Millisecond)),
		WithFault(FaultGarbleReply(1)),
	)

	start := time.Now()
	writeFrame(t, conn, "ID;")
	got := mustReadFrame(t, conn)
	elapsed := time.Since(start)
	if elapsed < 100*time.Millisecond {
		t.Errorf("reply arrived after %v, want at least 100ms (delay must still apply with garble composed)", elapsed)
	}
	if len(got) != len("ID0800;") {
		t.Fatalf("garbled+delayed reply length = %d, want %d", len(got), len("ID0800;"))
	}
	if got[0] != 'I'^0xFF {
		t.Errorf("first byte = %q, want the garbled 'I'", got[0])
	}
}

// --- FaultDisconnect ---

func TestFaultDisconnect(t *testing.T) {
	_, conn := newTestRadio(t, WithFault(FaultDisconnect(1)))

	writeFrame(t, conn, "ID;")
	// Exchange 1 still gets its normal reply before the pipe closes.
	if got, want := mustReadFrame(t, conn), "ID0800;"; got != want {
		t.Fatalf("exchange 1 = %q, want %q", got, want)
	}

	// The pipe is now closed; the host observes io.EOF.
	buf := make([]byte, 8)
	_, err := conn.Read(buf)
	if !errors.Is(err, io.EOF) {
		t.Errorf("Read after FaultDisconnect fired: err = %v, want io.EOF", err)
	}
}

func TestFaultDisconnect_SubsequentExchangeNeverAnswered(t *testing.T) {
	_, conn := newTestRadio(t, WithFault(FaultDisconnect(1)))

	writeFrame(t, conn, "ID;")
	mustReadFrame(t, conn) // exchange 1, drains the reply and lets the disconnect fire

	// Writing again after the disconnect must fail — the peer is gone.
	// (net.Pipe's Write is synchronous, so give the close a moment to
	// land before asserting.)
	deadline := time.Now().Add(time.Second)
	var werr error
	for time.Now().Before(deadline) {
		if _, werr = conn.Write([]byte("ID;")); werr != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if werr == nil {
		t.Error("Write after FaultDisconnect fired: want an error, got nil")
	}
}

// --- FaultChunkedReplies ---

// countingReader wraps a net.Conn and counts how many underlying Read
// calls were made, so a test can directly observe that a reply arrived
// in several small pieces rather than one Write's worth in a single
// Read. It forwards SetReadDeadline so it still satisfies readOneFrame's
// deadliner requirement (see fakeradio_test.go).
type countingReader struct {
	net.Conn
	reads int32
}

func (r *countingReader) Read(p []byte) (int, error) {
	atomic.AddInt32(&r.reads, 1)
	return r.Conn.Read(p)
}

func TestFaultChunkedReplies_ByteAtATime(t *testing.T) {
	_, conn := newTestRadio(t, WithFault(FaultChunkedReplies(1)))
	cr := &countingReader{Conn: conn.(net.Conn)}

	writeFrame(t, conn, "ID;")
	frame, err, timedOut := readOneFrame(t, cr, testTimeout)
	if timedOut {
		t.Fatal("timed out waiting for a chunked reply")
	}
	if err != nil && len(frame) == 0 {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := string(frame), "ID0800;"; got != want {
		t.Errorf("reassembled reply = %q, want %q", got, want)
	}
	// "ID0800;" is 7 bytes; byte-at-a-time chunking must take at least 7
	// separate Read calls to deliver it (vs. exactly 1 without the
	// fault — see TestNoFaultChunkedReplies_OneReadSuffices).
	if n := atomic.LoadInt32(&cr.reads); n < 7 {
		t.Errorf("underlying Read call count = %d, want >= 7 (byte-at-a-time chunking)", n)
	}
}

func TestFaultChunkedReplies_FixedSizeChunks(t *testing.T) {
	_, conn := newTestRadio(t, WithFault(FaultChunkedReplies(3)))
	cr := &countingReader{Conn: conn.(net.Conn)}

	writeFrame(t, conn, "ID;")
	frame, _, timedOut := readOneFrame(t, cr, testTimeout)
	if timedOut {
		t.Fatal("timed out waiting for a chunked reply")
	}
	if got, want := string(frame), "ID0800;"; got != want {
		t.Errorf("reassembled reply = %q, want %q", got, want)
	}
	// 7 bytes in chunks of 3 needs ceil(7/3) = 3 writes/reads.
	if n := atomic.LoadInt32(&cr.reads); n < 3 {
		t.Errorf("underlying Read call count = %d, want >= 3 (3-byte chunking)", n)
	}
}

func TestNoFaultChunkedReplies_OneReadSuffices(t *testing.T) {
	_, conn := newTestRadio(t) // no chunking fault
	cr := &countingReader{Conn: conn.(net.Conn)}

	writeFrame(t, conn, "ID;")
	frame, _, timedOut := readOneFrame(t, cr, testTimeout)
	if timedOut {
		t.Fatal("timed out waiting for a reply")
	}
	if got, want := string(frame), "ID0800;"; got != want {
		t.Errorf("reply = %q, want %q", got, want)
	}
	if n := atomic.LoadInt32(&cr.reads); n != 1 {
		t.Errorf("underlying Read call count = %d, want exactly 1 (whole reply in one Write, no chunking fault active)", n)
	}
}

// --- WithLatency ---

func TestWithLatency_DelaysEveryReply(t *testing.T) {
	latency := 150 * time.Millisecond
	_, conn := newTestRadio(t, WithLatency(latency))

	start := time.Now()
	writeFrame(t, conn, "ID;")
	if got, want := mustReadFrame(t, conn), "ID0800;"; got != want {
		t.Fatalf("reply = %q, want %q", got, want)
	}
	elapsed := time.Since(start)
	if elapsed < latency {
		t.Errorf("reply arrived after %v, want at least %v (WithLatency)", elapsed, latency)
	}
}

// --- Fault composition ---

func TestFaults_Compose_LatencyAndChunking(t *testing.T) {
	_, conn := newTestRadio(t,
		WithLatency(50*time.Millisecond),
		WithFault(FaultChunkedReplies(2)),
	)
	cr := &countingReader{Conn: conn.(net.Conn)}

	start := time.Now()
	writeFrame(t, conn, "ID;")
	frame, _, timedOut := readOneFrame(t, cr, testTimeout)
	if timedOut {
		t.Fatal("timed out waiting for a reply")
	}
	if got, want := string(frame), "ID0800;"; got != want {
		t.Errorf("reply = %q, want %q", got, want)
	}
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Errorf("reply arrived after %v, want at least 50ms (latency must still apply with chunking)", elapsed)
	}
	if n := atomic.LoadInt32(&cr.reads); n < 4 {
		t.Errorf("underlying Read call count = %d, want >= 4 (2-byte chunking of a 7-byte reply)", n)
	}
}

func TestFaults_Compose_DropAfterGarble(t *testing.T) {
	// Garble scripted for exchange 2, but drop scripted from exchange 2
	// onward too: the drop wins (there is nothing to garble once the
	// reply itself is suppressed).
	_, conn := newTestRadio(t,
		WithFault(FaultGarbleReply(2)),
		WithFault(FaultDropReplies(2)),
	)

	writeFrame(t, conn, "ID;")
	mustReadFrame(t, conn) // exchange 1, unaffected

	writeFrame(t, conn, "ID;")
	assertNoReply(t, conn) // exchange 2: dropped, not garbled-then-sent
}

// --- Close promptness (M3 Codex-review fix wave, Fix 8) ---

// closePromptly writes trigger to the radio, gives its serve goroutine a
// moment to enter the scripted delay, then asserts Close returns well
// within bound — nowhere near the scripted delay itself. Shared by the
// three delay-path tests below (delayed reply, delayed rejection,
// per-reply latency): before Fix 8, each of those paths was a bare
// time.Sleep, so a Close issued mid-delay blocked (via wg.Wait on the
// sleeping serve goroutine) until the FULL scripted delay had elapsed.
func closePromptly(t *testing.T, r *Radio, conn io.Writer, trigger string, bound time.Duration) {
	t.Helper()
	writeFrame(t, conn, trigger)
	// Let serve consume the frame and enter the delay. net.Pipe's Write
	// above is a rendezvous with serve's Read, so serve HAS the frame by
	// the time writeFrame returns; this short sleep only covers its
	// dispatch into handleEvent and the delay path.
	time.Sleep(50 * time.Millisecond)

	start := time.Now()
	if err := r.Close(); err != nil {
		t.Fatalf("Close: unexpected error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > bound {
		t.Errorf("Close took %v with a scripted delay pending, want < %v — delays must be interruptible by Close", elapsed, bound)
	}
}

func TestClose_PromptDuringDelayedReply(t *testing.T) {
	r := New(WithFault(FaultDelayedReply(1, 30*time.Second)))
	closePromptly(t, r, r.Port(), "ID;", time.Second)
}

func TestClose_PromptDuringDelayedRejection(t *testing.T) {
	r := New(WithFault(FaultDelayedRejection(1, 30*time.Second)))
	closePromptly(t, r, r.Port(), "ID;", time.Second)
}

func TestClose_PromptDuringLatency(t *testing.T) {
	r := New(WithLatency(30 * time.Second))
	closePromptly(t, r, r.Port(), "ID;", time.Second)
}
