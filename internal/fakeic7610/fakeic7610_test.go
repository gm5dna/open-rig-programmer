// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7610

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"
)

// The frame builders, the record builder and the port helpers this file uses
// live in parser_test.go, and every expectation there is hand-built from the
// guide's printed framing skeleton rather than from this package's own
// builders. The same discipline applies here.

// --- a draining reader -------------------------------------------------------

// stream is a background reader that takes everything the radio sends and
// delivers it as whole frames.
//
// The flood tests need it. Port() is one end of an unbuffered net.Pipe, so a
// test that wants to write a command WHILE a flood is running cannot also be
// the thing reading the flood: it would be parked in a Write with nothing
// draining the other direction. A separate reader is not a test convenience, it
// is what a real consumer of a flooding radio has to do too.
type stream struct {
	frames chan []byte
	done   chan struct{}
}

// takeFrame lifts one frame off buf, INDEPENDENTLY of this package's
// reassembler: it finds a preamble pair, then the next terminator. Deliberately
// its own implementation, for the same reason every expectation here is
// hand-built.
func takeFrame(buf []byte) (f, rest []byte, ok bool) {
	start := -1
	for i := 0; i+1 < len(buf); i++ {
		if buf[i] == 0xFE && buf[i+1] == 0xFE {
			start = i
			break
		}
	}
	if start < 0 {
		return nil, buf, false
	}
	for i := start + 4; i < len(buf); i++ {
		if buf[i] == 0xFD {
			return append([]byte(nil), buf[start:i+1]...), buf[i+1:], true
		}
	}
	return nil, buf[start:], false
}

// drain does NOT wait for its goroutine at cleanup. It cannot: a test's radio
// is closed by a cleanup registered BEFORE this one, and cleanups run
// last-registered-first, so waiting here would park before the thing that ends
// the read has happened. The goroutine ends on its own when the radio closes
// the pipe; the one test that needs to see that happen waits on s.done itself.
func drain(t *testing.T, conn net.Conn) *stream {
	t.Helper()
	s := &stream{frames: make(chan []byte, 8192), done: make(chan struct{})}
	go func() {
		defer close(s.done)
		buf := make([]byte, 4096)
		var acc []byte
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				acc = append(acc, buf[:n]...)
				for {
					f, rest, ok := takeFrame(acc)
					if !ok {
						break
					}
					acc = rest
					select {
					case s.frames <- f:
					default:
					}
				}
			}
			if err != nil {
				return
			}
		}
	}()
	return s
}

// next waits up to d for the next frame, or returns nil.
func (s *stream) next(d time.Duration) []byte {
	select {
	case f := <-s.frames:
		return f
	case <-time.After(d):
		return nil
	}
}

// flush discards everything already delivered.
func (s *stream) flush() {
	for {
		select {
		case <-s.frames:
		default:
			return
		}
	}
}

// requireQuiet requires that no frame at all arrives within d.
func (s *stream) requireQuiet(t *testing.T, d time.Duration, why string) {
	t.Helper()
	if f := s.next(d); f != nil {
		t.Fatalf("%s: a frame % X arrived, want silence", why, f)
	}
}

// frameTo returns a frame's destination address.
func frameTo(f []byte) byte { return f[2] }

// --- echo --------------------------------------------------------------------

// TestWithUSBEchoEchoesTheReceivedBytesExactlyBeforeTheAnswer.
//
// The guide records a "CI-V USB Echo Back" setting and a [REMOTE]-linked bus
// case; both look identical on the wire, which is why one option covers both.
// EXACTLY means exactly: the bytes that come back first are the bytes that went
// in, preamble padding and all, not a frame this package rebuilt.
func TestWithUSBEchoEchoesTheReceivedBytesExactlyBeforeTheAnswer(t *testing.T) {
	r := newTestRadio(t, WithUSBEcho())
	s := drain(t, r.Port())

	sent := request(0x19, 0x00)
	send(t, r, sent)

	echoed := s.next(replyWait)
	if echoed == nil {
		t.Fatal("no echo")
	}
	if !bytes.Equal(echoed, sent) {
		t.Errorf("echo = % X, want % X — verbatim", echoed, sent)
	}

	answered := s.next(replyWait)
	if answered == nil {
		t.Fatal("no answer after the echo")
	}
	if want := answer(0x19, 0x00, 0xA5); !bytes.Equal(answered, want) {
		t.Errorf("answer = % X, want % X", answered, want)
	}
}

// TestWithUSBEchoEchoesPreamblePaddingVerbatim: the echo is the received bytes,
// not a reconstruction of them, so padding survives it.
func TestWithUSBEchoEchoesPreamblePaddingVerbatim(t *testing.T) {
	r := newTestRadio(t, WithUSBEcho())
	s := drain(t, r.Port())

	padded := []byte{0xFE, 0xFE, 0xFE, 0xFE, 0xFE, 0x98, 0xE0, 0x19, 0x00, 0xFD}
	send(t, r, padded)

	echoed := s.next(replyWait)
	if echoed == nil {
		t.Fatal("no echo")
	}
	if !bytes.Equal(echoed, padded) {
		t.Errorf("echo = % X, want % X — the five preamble bytes must come back too", echoed, padded)
	}
}

// TestWithUSBEchoEchoesAFrameAddressedElsewhereAndDoesNotAnswerIt pins the one
// place "ignored entirely" and "echo every frame verbatim" could be read as
// contradicting each other.
//
// The echo is a property of the LINE — a USB codec or a bus reflecting what was
// put on it — so it happens before the address filter. The ANSWER is a property
// of the radio, so it does not happen at all. The frame comes back once, and
// nothing follows it. That ordering is a modelling decision, not a documented
// one; PROVENANCE.md records it as such.
func TestWithUSBEchoEchoesAFrameAddressedElsewhereAndDoesNotAnswerIt(t *testing.T) {
	r := newTestRadio(t, WithUSBEcho())
	s := drain(t, r.Port())

	elsewhere := requestTo(0x94, 0x19, 0x00)
	send(t, r, elsewhere)

	echoed := s.next(replyWait)
	if echoed == nil {
		t.Fatal("no echo of the frame addressed elsewhere")
	}
	if !bytes.Equal(echoed, elsewhere) {
		t.Errorf("echo = % X, want % X", echoed, elsewhere)
	}
	s.requireQuiet(t, silenceWait, "a frame addressed to 0x94 must draw no answer")

	if log := r.CommandLog(); len(log) != 0 {
		t.Errorf("CommandLog = %v, want empty — echoing a frame is not seeing it", log)
	}
}

// TestWithoutUSBEchoNothingIsEchoed: the default is a quiet line.
func TestWithoutUSBEchoNothingIsEchoed(t *testing.T) {
	r := newTestRadio(t)
	s := drain(t, r.Port())

	send(t, r, request(0x19, 0x00))
	first := s.next(replyWait)
	if first == nil {
		t.Fatal("no answer")
	}
	if want := answer(0x19, 0x00, 0xA5); !bytes.Equal(first, want) {
		t.Errorf("first frame back = % X, want the answer % X — nothing should precede it", first, want)
	}
	s.requireQuiet(t, silenceWait, "only one frame should come back")
}

// --- the two floods ----------------------------------------------------------

const floodEvery = 5 * time.Millisecond

// requireFloodFrames collects n frames and requires every one of them to be a
// flood frame addressed to `to`.
func requireFloodFrames(t *testing.T, s *stream, to byte, n int) {
	t.Helper()
	want := []byte{0xFE, 0xFE, to, 0x98, 0x19, 0x00, 0xA5, 0xFD}
	for i := 0; i < n; i++ {
		f := s.next(replyWait)
		if f == nil {
			t.Fatalf("flood frame %d of %d did not arrive within %v", i+1, n, replyWait)
		}
		if !bytes.Equal(f, want) {
			t.Fatalf("flood frame %d = % X, want % X", i+1, f, want)
		}
	}
}

// TestWithTransceiveFloodEmitsBroadcastFrames.
//
// `to` = 0x00 is ASSUMED. The document prints NO broadcast frame — the only
// answer-direction skeleton it prints has `to` = 0xE0 — so this test asserts an
// assumption, and asserting it is not evidence for it. What it genuinely pins
// is that the option produces a steady stream of well-formed frames a consumer
// can be tested against.
func TestWithTransceiveFloodEmitsBroadcastFrames(t *testing.T) {
	r := newTestRadio(t, WithTransceiveFlood(floodEvery))
	s := drain(t, r.Port())
	requireFloodFrames(t, s, 0x00, 5)
}

// TestWithAddressedFloodEmitsControllerAddressedFrames.
//
// `to` = 0xE0: as though the radio were answering continuously. A SYNTHETIC
// line condition — the document describes no radio doing this — which exists so
// that a consumer that must survive a jabbering peer can be shown surviving
// one.
func TestWithAddressedFloodEmitsControllerAddressedFrames(t *testing.T) {
	r := newTestRadio(t, WithAddressedFlood(floodEvery))
	s := drain(t, r.Port())
	requireFloodFrames(t, s, 0xE0, 5)
}

// TestTheTwoFloodsAreIndependent is why they are two options and not one option
// with a flag: a consumer switches on WHICH is running, so each must be able to
// run without the other, and both must be able to run at once.
func TestTheTwoFloodsAreIndependent(t *testing.T) {
	t.Run("broadcast alone emits nothing addressed to the controller", func(t *testing.T) {
		r := newTestRadio(t, WithTransceiveFlood(floodEvery))
		s := drain(t, r.Port())
		for i := 0; i < 20; i++ {
			f := s.next(replyWait)
			if f == nil {
				t.Fatalf("frame %d did not arrive", i+1)
			}
			if to := frameTo(f); to != 0x00 {
				t.Fatalf("frame %d addressed to %02X, want 00 — the addressed flood is not running", i+1, to)
			}
		}
	})

	t.Run("addressed alone emits nothing addressed to nobody", func(t *testing.T) {
		r := newTestRadio(t, WithAddressedFlood(floodEvery))
		s := drain(t, r.Port())
		for i := 0; i < 20; i++ {
			f := s.next(replyWait)
			if f == nil {
				t.Fatalf("frame %d did not arrive", i+1)
			}
			if to := frameTo(f); to != 0xE0 {
				t.Fatalf("frame %d addressed to %02X, want E0 — the broadcast flood is not running", i+1, to)
			}
		}
	})

	t.Run("both at once produce both", func(t *testing.T) {
		r := newTestRadio(t, WithTransceiveFlood(floodEvery), WithAddressedFlood(floodEvery))
		s := drain(t, r.Port())

		seen := map[byte]int{}
		deadline := time.Now().Add(replyWait)
		for time.Now().Before(deadline) && (seen[0x00] < 3 || seen[0xE0] < 3) {
			f := s.next(replyWait)
			if f == nil {
				break
			}
			seen[frameTo(f)]++
		}
		if seen[0x00] < 3 || seen[0xE0] < 3 {
			t.Errorf("saw %d broadcast and %d controller-addressed frames, want at least 3 of each", seen[0x00], seen[0xE0])
		}
	})

	t.Run("neither by default", func(t *testing.T) {
		r := newTestRadio(t)
		s := drain(t, r.Port())
		s.requireQuiet(t, 20*floodEvery, "a radio with no flood option is silent until spoken to")
	})

	t.Run("a non-positive interval starts nothing", func(t *testing.T) {
		r := newTestRadio(t, WithTransceiveFlood(0), WithAddressedFlood(-time.Second))
		s := drain(t, r.Port())
		s.requireQuiet(t, 20*floodEvery, "a zero or negative interval is \"no flood\"")
	})
}

// TestFloodsCanBeStartedAfterConstructionAndStopped is the sequence a consumer
// actually needs: a QUIET Open, then a NOISY session.
//
// It matters because an Open that succeeds on a quiet line and a session that
// then has to cope with a flood is the realistic failure — not a radio that was
// already screaming when the port was opened.
func TestFloodsCanBeStartedAfterConstructionAndStopped(t *testing.T) {
	cases := []struct {
		name  string
		start func(*Radio)
		to    byte
	}{
		{"broadcast", func(r *Radio) { r.StartBroadcastFlood(floodEvery) }, 0x00},
		{"controller-addressed", func(r *Radio) { r.StartAddressedFlood(floodEvery) }, 0xE0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRadio(t)
			s := drain(t, r.Port())

			// Quiet to begin with.
			s.requireQuiet(t, 20*floodEvery, "the radio was constructed without a flood option")

			tc.start(r)
			requireFloodFrames(t, s, tc.to, 5)

			r.StopFloods()
			// Frames already queued when the flood stopped still get written —
			// the same as a radio whose last transceive frame was already in
			// the UART. Drain them, then require real silence.
			time.Sleep(20 * floodEvery)
			s.flush()
			s.requireQuiet(t, 40*floodEvery, "after StopFloods the line must go quiet")
		})
	}

	t.Run("StopFloods stops both, and is idempotent", func(t *testing.T) {
		r := newTestRadio(t, WithTransceiveFlood(floodEvery), WithAddressedFlood(floodEvery))
		s := drain(t, r.Port())
		if f := s.next(replyWait); f == nil {
			t.Fatal("no flood frames at all")
		}

		r.StopFloods()
		r.StopFloods()
		r.StopFloods()

		time.Sleep(20 * floodEvery)
		s.flush()
		s.requireQuiet(t, 40*floodEvery, "both floods must be stopped")
	})

	t.Run("a flood still answers commands", func(t *testing.T) {
		r := newTestRadio(t)
		s := drain(t, r.Port())
		r.StartBroadcastFlood(floodEvery)

		send(t, r, request(0x19, 0x00))
		want := answer(0x19, 0x00, 0xA5)
		deadline := time.Now().Add(replyWait)
		for time.Now().Before(deadline) {
			f := s.next(replyWait)
			if f == nil {
				break
			}
			if bytes.Equal(f, want) {
				return
			}
			if to := frameTo(f); to != 0x00 {
				t.Fatalf("unexpected frame % X", f)
			}
		}
		t.Fatalf("the ID answer % X never arrived through the flood", want)
	})
}

// TestARunningFloodDoesNotDisturbAConcurrentReadOrWrite.
//
// This is the property the whole two-flood design exists to make testable: the
// line is noisy, and the conversation still works. Both floods run; a caller
// writes sets and reads and gets every answer, in order, unmangled.
func TestARunningFloodDoesNotDisturbAConcurrentReadOrWrite(t *testing.T) {
	r := newTestRadio(t, WithTransceiveFlood(floodEvery), WithAddressedFlood(floodEvery))
	s := drain(t, r.Port())

	// nextAnswer returns the next frame that is a reply to a command, skipping
	// flood frames. A flood frame is 19 00 <token>; a reply here is a 1A 00
	// answer or one of the two fixed codes.
	nextAnswer := func() []byte {
		deadline := time.Now().Add(replyWait)
		for time.Now().Before(deadline) {
			f := s.next(replyWait)
			if f == nil {
				return nil
			}
			if len(f) >= 6 && f[4] == 0x19 {
				continue // a flood frame
			}
			return f
		}
		return nil
	}

	for ch := 1; ch <= 8; ch++ {
		rec := testRecord(byte(ch*3), RecordLen)
		lo := byte((ch/10)<<4 | ch%10)

		send(t, r, request(append([]byte{0x1A, 0x00, 0x00, lo}, rec...)...))
		got := nextAnswer()
		if want := ok(); !bytes.Equal(got, want) {
			t.Fatalf("channel %d: set answered % X, want % X", ch, got, want)
		}

		send(t, r, request(0x1A, 0x00, 0x00, lo))
		got = nextAnswer()
		if want := answer(append([]byte{0x1A, 0x00, 0x00, lo}, rec...)...); !bytes.Equal(got, want) {
			t.Fatalf("channel %d: read answered % X, want % X", ch, got, want)
		}
	}

	// And the Go-side accessors are safe to use from another goroutine while
	// all that is going on. -race is what makes this assertion mean anything.
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, _ = r.SlotState(1)
				_ = r.CommandLog()
				_ = r.BytesWritten()
			}
		}()
	}
	time.Sleep(20 * floodEvery)
	close(stop)
	wg.Wait()
}

// --- record length -----------------------------------------------------------

// TestWithRecordLengthAnswersReadsAtThatLength.
//
// RecordLen is DERIVED from an evidence artefact's field widths, not read off a
// page and not confirmed against hardware. This option is the acknowledgement
// that a derivation can be wrong — so it must genuinely move the accepted
// length, both for a set and for a read.
func TestWithRecordLengthAnswersReadsAtThatLength(t *testing.T) {
	for _, n := range []int{1, 8, RecordLen, 40} {
		r := New(WithRecordLength(n))

		rec := testRecord(0x81, n)
		if got, want := exchange(t, r, request(append([]byte{0x1A, 0x00, 0x00, 0x11}, rec...)...)), ok(); !bytes.Equal(got, want) {
			t.Fatalf("n=%d: set answered % X, want % X", n, got, want)
		}

		got := exchange(t, r, request(0x1A, 0x00, 0x00, 0x11))
		if want := answer(append([]byte{0x1A, 0x00, 0x00, 0x11}, rec...)...); !bytes.Equal(got, want) {
			t.Errorf("n=%d: read answered % X, want % X", n, got, want)
		}
		if carried := len(got) - 9; carried != n {
			t.Errorf("n=%d: the answer carries %d record bytes, want %d", n, carried, n)
		}

		// The default length is no longer accepted, which is what proves the
		// option changed the rule rather than widening it.
		if n != RecordLen {
			wrong := testRecord(0x82, RecordLen)
			if got, want := exchange(t, r, request(append([]byte{0x1A, 0x00, 0x00, 0x12}, wrong...)...)), ng(); !bytes.Equal(got, want) {
				t.Errorf("n=%d: a %d-byte record answered % X, want % X", n, RecordLen, got, want)
			}
		}
		_ = r.Close()
	}
}

// TestWithRecordLengthRejectsAZeroLengthRecord: a zero-length record is not a
// shorter record, it is the absence of one, and it would make a set
// indistinguishable from a read.
func TestWithRecordLengthRejectsAZeroLengthRecord(t *testing.T) {
	for _, n := range []int{0, -1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("WithRecordLength(%d) did not panic", n)
				}
			}()
			New(WithRecordLength(n))
		}()
	}
}

// TestSetSlotPanicsOnWhatTheRadioCannotHold. A fake that quietly accepted
// either of these would be lying to the test that trusted it.
func TestSetSlotPanicsOnWhatTheRadioCannotHold(t *testing.T) {
	t.Run("an unaddressable channel", func(t *testing.T) {
		r := newTestRadio(t)
		defer func() {
			if recover() == nil {
				t.Error("SetSlot(100, ...) did not panic")
			}
		}()
		r.SetSlot(100, MemState{Raw: testRecord(0, RecordLen)})
	})

	t.Run("a record of the wrong length", func(t *testing.T) {
		r := newTestRadio(t)
		defer func() {
			if recover() == nil {
				t.Error("SetSlot with a short record did not panic")
			}
		}()
		r.SetSlot(1, MemState{Raw: testRecord(0, RecordLen-1)})
	})
}

// TestSlotStateAndSetSlotCopyTheirRecords: a caller's slice is not the radio's
// state, in either direction.
func TestSlotStateAndSetSlotCopyTheirRecords(t *testing.T) {
	r := newTestRadio(t)

	rec := testRecord(0x91, RecordLen)
	original := append([]byte(nil), rec...)
	r.SetSlot(5, MemState{Raw: rec})

	rec[0] ^= 0xFF // mutate the caller's slice
	m, set := r.SlotState(5)
	if !set || !bytes.Equal(m.Raw, original) {
		t.Errorf("after mutating the caller's slice, stored record = % X, want % X", m.Raw, original)
	}

	m.Raw[0] ^= 0xFF // mutate what SlotState returned
	m2, _ := r.SlotState(5)
	if !bytes.Equal(m2.Raw, original) {
		t.Errorf("after mutating the returned record, stored record = % X, want % X", m2.Raw, original)
	}
}

// --- latency -----------------------------------------------------------------

// TestWithLatencyDelaysTheAnswer.
//
// No IC-7610 timing has ever been observed by this project, so this models
// nothing real; it is the knob a consumer's own timeout handling is proven
// against.
func TestWithLatencyDelaysTheAnswer(t *testing.T) {
	const latency = 150 * time.Millisecond
	r := newTestRadio(t, WithLatency(latency))

	start := time.Now()
	got := exchange(t, r, request(0x19, 0x00))
	elapsed := time.Since(start)

	if want := answer(0x19, 0x00, 0xA5); !bytes.Equal(got, want) {
		t.Fatalf("answer = % X, want % X", got, want)
	}
	// A little slack for a coarse timer; the point is that it waited, not that
	// it waited to the nanosecond.
	if min := latency - 20*time.Millisecond; elapsed < min {
		t.Errorf("the answer came back after %v, want at least %v", elapsed, min)
	}
}

// TestWithLatencyDoesNotDelayTheEchoOrAFlood: a reflection is not a reply, and
// neither is a transceive frame.
func TestWithLatencyDoesNotDelayTheEchoOrAFlood(t *testing.T) {
	const latency = 400 * time.Millisecond
	r := newTestRadio(t, WithLatency(latency), WithUSBEcho())
	s := drain(t, r.Port())

	start := time.Now()
	sent := request(0x19, 0x00)
	send(t, r, sent)

	echoed := s.next(replyWait)
	echoAt := time.Since(start)
	if echoed == nil || !bytes.Equal(echoed, sent) {
		t.Fatalf("echo = % X, want % X", echoed, sent)
	}
	if echoAt > latency/2 {
		t.Errorf("the echo took %v, which is most of the %v answer latency — the echo must not be delayed", echoAt, latency)
	}

	if a := s.next(replyWait); a == nil {
		t.Fatal("no answer after the echo")
	}
	if total := time.Since(start); total < latency-20*time.Millisecond {
		t.Errorf("the answer came back after %v, want at least %v", total, latency)
	}
}

// TestCloseDoesNotWaitOutAPendingLatency. Close's promptness is the whole
// reason the latency wait is interruptible rather than a bare Sleep.
func TestCloseDoesNotWaitOutAPendingLatency(t *testing.T) {
	r := New(WithLatency(30 * time.Second))
	send(t, r, request(0x19, 0x00))

	// Give the radio time to read the frame and park in its latency wait.
	time.Sleep(50 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		_ = r.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return while a 30s answer latency was pending")
	}
}

// --- lifecycle ---------------------------------------------------------------

// TestCloseIsIdempotentAndStopsTheFloods.
func TestCloseIsIdempotentAndStopsTheFloods(t *testing.T) {
	r := New(WithTransceiveFlood(floodEvery), WithAddressedFlood(floodEvery))
	s := drain(t, r.Port())
	if f := s.next(replyWait); f == nil {
		t.Fatal("no flood frames before Close")
	}

	done := make(chan struct{})
	go func() {
		_ = r.Close()
		_ = r.Close()
		_ = r.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return")
	}

	// The host end sees EOF, not ErrClosedPipe: the radio closed its own end,
	// which is the signal "the radio went away".
	<-s.done
}

// TestStartingAFloodAfterCloseIsANoOp: a closed radio does not start work.
func TestStartingAFloodAfterCloseIsANoOp(t *testing.T) {
	r := New()
	if err := r.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	r.StartBroadcastFlood(floodEvery)
	r.StartAddressedFlood(floodEvery)
	r.StopFloods()

	done := make(chan struct{})
	go func() {
		_ = r.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close after a post-close flood start did not return — a goroutine was left running")
	}
}

// --- the two logs ------------------------------------------------------------

// TestCommandLogRecordsEveryCommandSeenAndNothingElse.
func TestCommandLogRecordsEveryCommandSeenAndNothingElse(t *testing.T) {
	r := newTestRadio(t)

	rec := testRecord(0xA1, RecordLen)
	exchange(t, r, request(0x19, 0x00))
	exchange(t, r, request(append([]byte{0x1A, 0x00, 0x00, 0x01}, rec...)...))
	exchange(t, r, request(0x1A, 0x00, 0x00, 0x01))
	exchange(t, r, request(0x1A, 0x05, 0x01, 0x33)) // refused, but seen
	exchange(t, r, request(0x0B))                   // refused, but seen
	requireSilence(t, r, requestTo(0x94, 0x19, 0x00))

	want := [][2]byte{
		{0x19, 0x00},
		{0x1A, 0x00},
		{0x1A, 0x00},
		{0x1A, 0x05},
		{0x0B, 0x00}, // no sub-command; logged with a zero one
	}
	got := r.CommandLog()
	if len(got) != len(want) {
		t.Fatalf("CommandLog = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("CommandLog[%d] = %02X %02X, want %02X %02X", i, got[i][0], got[i][1], want[i][0], want[i][1])
		}
	}
}

// TestBytesWrittenRecordsEveryByteReceived — before framing, before the address
// filter, before any parsing. It is the record of the wire.
func TestBytesWrittenRecordsEveryByteReceived(t *testing.T) {
	r := newTestRadio(t)

	noise := []byte{0x00, 0x99, 0x12}
	good := request(0x19, 0x00)
	elsewhere := requestTo(0x94, 0x0B)

	send(t, r, noise)
	send(t, r, good)
	if got := receive(t, r, replyWait); got == nil {
		t.Fatal("no answer to the good frame")
	}
	send(t, r, elsewhere)
	// Let the radio read the last write before asking what it received.
	time.Sleep(50 * time.Millisecond)

	want := append(append(append([]byte{}, noise...), good...), elsewhere...)
	if got := r.BytesWritten(); !bytes.Equal(got, want) {
		t.Errorf("BytesWritten = % X, want % X", got, want)
	}
}
