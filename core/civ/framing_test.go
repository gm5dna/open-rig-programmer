// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestNewFraming_RefusesAnUnconfiguredProfile is the constructor's
// fail-closed rule: a zero Profile describes no radio, so the adapter
// built from it would install a gate that speaks for nobody. The engine's
// own nil-Framing check cannot see that — a Framing built from a zero
// Profile is a perfectly non-nil interface value — so the refusal has to
// happen here, before an Engine can be bound to it.
func TestNewFraming_RefusesAnUnconfiguredProfile(t *testing.T) {
	f, err := NewFraming(Profile{})
	if err == nil {
		t.Fatalf("NewFraming(zero Profile) returned %v and no error; an unconfigured profile must be refused at construction", f)
	}
	if !errors.Is(err, ErrInvalidProfile) {
		t.Errorf("NewFraming(zero Profile) error = %v, want one matching ErrInvalidProfile", err)
	}
	if f != nil {
		t.Errorf("NewFraming(zero Profile) returned a non-nil Framing %v alongside its error", f)
	}
}

// TestNewFraming_IsAConfiguredProfilesFraming walks the disagreeing
// fixtures: every configured profile must yield a usable adapter, and the
// adapter's InitSequence must be EMPTY for all of them (spec D2,
// adjudication 3 — nothing this tier sends to an Icom radio mutates it).
func TestNewFraming_IsAConfiguredProfilesFraming(t *testing.T) {
	for _, np := range allTestProfiles() {
		t.Run(np.name, func(t *testing.T) {
			f, err := NewFraming(np.p)
			if err != nil {
				t.Fatalf("NewFraming(%s): %v", np.name, err)
			}
			if got := f.InitSequence(); len(got) != 0 {
				t.Errorf("InitSequence() = %v, want empty: a CI-V session opens without writing anything to the radio", got)
			}
			var _ transport.Framing = f
		})
	}
}

// --- spec D2's acceptance: the engine's own state machine over CI-V -----
//
// "The full CAT engine test suite unchanged plus the same suite run over a
// CI-V framing" is D2's acceptance line. core/transport already runs its
// suite over a deliberately non-CAT framing (framing_test.go's
// lineFraming), which proves the seam is a seam; what THAT cannot prove is
// that the REAL CI-V adapter — this package's accumulator, gate, echo
// removal and address checks — satisfies the same contract. These tests
// are that half: an Engine built over civ.NewFraming, driven through the
// same exchanges, reaching the same outcomes.

// civPort is a scripted transport.Port: it hands the reader goroutine
// whatever bytes a test queues, records every write, and can be told to
// answer a write with a canned reply.
type civPort struct {
	mu      sync.Mutex
	replies [][]byte
	writes  [][]byte
	pending []byte
	echo    bool
	wake    chan struct{}
	closeCh chan struct{}
	closed  bool
}

func newCIVPort(replies ...[]byte) *civPort {
	return &civPort{
		replies: replies,
		wake:    make(chan struct{}, 64),
		closeCh: make(chan struct{}),
	}
}

func (p *civPort) Read(b []byte) (int, error) {
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
			return 0, errCIVPortClosed
		}
	}
}

func (p *civPort) Write(b []byte) (int, error) {
	p.mu.Lock()
	p.writes = append(p.writes, append([]byte(nil), b...))
	if p.echo {
		// A REMOTE-bus radio (and many USB adapters) send the written
		// frame straight back before any answer.
		p.pending = append(p.pending, b...)
	}
	if len(p.replies) > 0 {
		p.pending = append(p.pending, p.replies[0]...)
		p.replies = p.replies[1:]
	}
	p.mu.Unlock()
	p.poke()
	return len(b), nil
}

// deliver queues bytes the radio sends unprompted — a transceive
// broadcast, a late answer, a burst of noise.
func (p *civPort) deliver(b []byte) {
	p.mu.Lock()
	p.pending = append(p.pending, b...)
	p.mu.Unlock()
	p.poke()
}

func (p *civPort) poke() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

func (p *civPort) Close() error {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		close(p.closeCh)
	}
	p.mu.Unlock()
	return nil
}

func (p *civPort) written() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([][]byte(nil), p.writes...)
}

var errCIVPortClosed = errors.New("civ test port closed")

// rawFrame assembles FE FE <to> <from> body… FD without going through a
// builder, so a test can put on the wire what no builder here would emit —
// an answer, another station's traffic, a mutated echo.
func rawFrame(to, from byte, body ...byte) []byte {
	out := []byte{PreambleByte, PreambleByte, to, from}
	out = append(out, body...)
	return append(out, EndByte)
}

// idAnswer is the `19 00` reply this profile's probe would see.
func idAnswer(p Profile) []byte {
	return rawFrame(p.ControllerAddress(), p.RadioAddress(), CmdTransceiverID, SubTransceiverID, 0x94)
}

// newCIVEngine builds an Engine over p's framing and the given port,
// returning the adapter too so a test can read its counters.
func newCIVEngine(t *testing.T, p Profile, port transport.Port) (*transport.Engine, transport.Framing) {
	t.Helper()
	f, err := NewFraming(p)
	if err != nil {
		t.Fatalf("NewFraming(%s): %v", p.Model(), err)
	}
	e, err := transport.NewEngineWith(port, f)
	if err != nil {
		t.Fatalf("NewEngineWith: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e, f
}

// fastRead is CIVReadSpec with a test-scale answer deadline, so a test
// that means to WAIT OUT a timeout costs a tenth of a second rather than a
// whole one. Nothing else about the spec changes — the class is still
// ClassRead and the matcher is still the profile's own.
func fastRead(match func([]byte) bool, retries int) transport.CommandSpec {
	spec := CIVReadSpec(match, retries)
	spec.Timeout = 100 * time.Millisecond
	return spec
}

// TestFraming_EngineRoundTripsAReadOverEveryProfile is D2's acceptance in
// its simplest form, run over the DISAGREEING fixtures: the engine writes
// the profile's own `19 00` frame, the radio answers, and the answer comes
// back — with no CAT anywhere in the path and no init sequence written.
func TestFraming_EngineRoundTripsAReadOverEveryProfile(t *testing.T) {
	for _, np := range allTestProfiles() {
		t.Run(np.name, func(t *testing.T) {
			p := np.p
			port := newCIVPort(idAnswer(p))
			t.Cleanup(func() { _ = port.Close() })
			e, _ := newCIVEngine(t, p, port)

			cmd, err := p.BuildTransceiverIDRead()
			if err != nil {
				t.Fatalf("BuildTransceiverIDRead: %v", err)
			}
			got, err := e.Do(context.Background(), cmd, fastRead(p.TransceiverIDAnswerMatcher(), 0))
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			if want := idAnswer(p); string(got) != string(want) {
				t.Errorf("answer = % x, want % x", got, want)
			}
			if n := len(port.written()); n != 1 {
				t.Errorf("port saw %d writes, want exactly 1 — a CI-V session opens without an init sequence", n)
			}
			token, err := p.ParseTransceiverID(got)
			if err != nil {
				t.Fatalf("ParseTransceiverID(%x): %v", got, err)
			}
			if token != "94" {
				t.Errorf("ParseTransceiverID = %q, want %q", token, "94")
			}
		})
	}
}

// TestFraming_InitWritesNothing pins the empty InitSequence end to end:
// Engine.Init over a CI-V framing transmits not one byte. It is the
// consent property spec D2 adjudication 3 states — a session opens without
// touching anyone's radio settings — and it is worth proving through the
// ENGINE rather than by reading InitSequence's return value, since it is
// the engine that would send them.
func TestFraming_InitWritesNothing(t *testing.T) {
	p := flatProfile
	port := newCIVPort()
	t.Cleanup(func() { _ = port.Close() })
	e, _ := newCIVEngine(t, p, port)

	// The line is silent, so the bounded drain simply observes its idle
	// gap and returns.
	if err := e.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if n := len(port.written()); n != 0 {
		t.Fatalf("Init wrote %d frames (%x), want none: nothing this tier sends to an Icom radio mutates it", n, port.written())
	}
}

// TestFraming_WriteWithAckIsAcknowledged is the memory set's happy path
// through the engine: ClassWriteWithAck waits for the radio's six-byte FB
// and returns it.
func TestFraming_WriteWithAckIsAcknowledged(t *testing.T) {
	p := flatProfile
	ack := rawFrame(p.ControllerAddress(), p.RadioAddress(), AckByte)
	port := newCIVPort(ack)
	t.Cleanup(func() { _ = port.Close() })
	e, _ := newCIVEngine(t, p, port)

	cmd, err := p.BuildMemorySet(sampleRecord(t, p, p.BuildRecordLength()))
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	spec := CIVWriteWithAckSpec(p.AcknowledgementMatcher())
	spec.Timeout = 100 * time.Millisecond
	got, err := e.Do(context.Background(), cmd, spec)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if string(got) != string(ack) {
		t.Errorf("answer = % x, want the FB ack % x", got, ack)
	}
}

// TestFraming_NeitherWriteClassEverRetransmits is safety obligation 2 over
// CI-V, for BOTH write classes: a write whose answer never comes is
// reported, never resent. One write reaches the port and exactly one.
func TestFraming_NeitherWriteClassEverRetransmits(t *testing.T) {
	p := flatProfile
	rec := sampleRecord(t, p, p.BuildRecordLength())

	t.Run("ClassWriteWithAck", func(t *testing.T) {
		port := newCIVPort() // the radio never acknowledges
		t.Cleanup(func() { _ = port.Close() })
		e, _ := newCIVEngine(t, p, port)

		cmd, err := p.BuildMemorySet(rec)
		if err != nil {
			t.Fatalf("BuildMemorySet: %v", err)
		}
		spec := CIVWriteWithAckSpec(p.AcknowledgementMatcher())
		spec.Timeout = 100 * time.Millisecond
		if _, err := e.Do(context.Background(), cmd, spec); !errors.Is(err, transport.ErrTimeout) {
			t.Fatalf("Do error = %v, want ErrTimeout", err)
		}
		if n := len(port.written()); n != 1 {
			t.Errorf("port saw %d writes, want exactly 1: an acknowledged write is still a WRITE and is never retransmitted", n)
		}
	})

	t.Run("ClassWrite", func(t *testing.T) {
		port := newCIVPort()
		t.Cleanup(func() { _ = port.Close() })
		e, _ := newCIVEngine(t, p, port)

		cmd, err := p.BuildMemorySet(rec)
		if err != nil {
			t.Fatalf("BuildMemorySet: %v", err)
		}
		spec := transport.CommandSpec{Class: transport.ClassWrite, ErrorWindow: 50 * time.Millisecond}
		if _, err := e.Do(context.Background(), cmd, spec); err != nil {
			t.Fatalf("Do: %v", err)
		}
		if n := len(port.written()); n != 1 {
			t.Errorf("port saw %d writes, want exactly 1", n)
		}
	})
}

// TestFraming_RetryReadsOnAWriteIsRefused pins the other half of that
// obligation: CIVWriteWithAckSpec sets RetryReads to zero, and a caller
// who overrides it is refused BEFORE anything reaches the port.
func TestFraming_RetryReadsOnAWriteIsRefused(t *testing.T) {
	p := flatProfile
	spec := CIVWriteWithAckSpec(p.AcknowledgementMatcher())
	if spec.Class != transport.ClassWriteWithAck {
		t.Errorf("CIVWriteWithAckSpec Class = %v, want ClassWriteWithAck", spec.Class)
	}
	if spec.RetryReads != 0 {
		t.Errorf("CIVWriteWithAckSpec RetryReads = %d, want 0", spec.RetryReads)
	}

	port := newCIVPort()
	t.Cleanup(func() { _ = port.Close() })
	e, _ := newCIVEngine(t, p, port)

	cmd, err := p.BuildMemorySet(sampleRecord(t, p, p.BuildRecordLength()))
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	spec.RetryReads = 2
	if _, err := e.Do(context.Background(), cmd, spec); !errors.Is(err, transport.ErrInvalidSpec) {
		t.Fatalf("Do error = %v, want ErrInvalidSpec", err)
	}
	if n := len(port.written()); n != 0 {
		t.Errorf("port saw %d writes, want none: an invalid spec is refused before anything is transmitted", n)
	}
}

// TestFraming_ReadSpecIsAReadThatMayRetry is CIVReadSpec's own shape: a
// read is idempotent, so retrying one is safe and the helper carries the
// caller's retry count through unchanged.
func TestFraming_ReadSpecIsAReadThatMayRetry(t *testing.T) {
	spec := CIVReadSpec(flatProfile.MemoryAnswerMatcher(), 2)
	if spec.Class != transport.ClassRead {
		t.Errorf("CIVReadSpec Class = %v, want ClassRead", spec.Class)
	}
	if spec.RetryReads != 2 {
		t.Errorf("CIVReadSpec RetryReads = %d, want 2", spec.RetryReads)
	}
	if spec.Match == nil {
		t.Error("CIVReadSpec Match is nil — a read has no answer-matching rule of its own since D2")
	}
}

// TestFraming_RejectionIsSourceAddressChecked is the ic705 review's catch,
// end to end through the engine. An FA addressed to this controller but
// from ANOTHER radio on the bus is not a refusal of OUR transaction: the
// engine must not turn it into ErrRejected and tell the user their radio
// refused their command.
func TestFraming_RejectionIsSourceAddressChecked(t *testing.T) {
	p := flatProfile
	const otherRadio = 0x66
	if otherRadio == p.RadioAddress() {
		t.Fatal("fixture error: the other station shares this profile's address")
	}

	t.Run("from another radio it is not our rejection", func(t *testing.T) {
		port := newCIVPort(rawFrame(p.ControllerAddress(), otherRadio, NakByte))
		t.Cleanup(func() { _ = port.Close() })
		e, _ := newCIVEngine(t, p, port)

		cmd, err := p.BuildTransceiverIDRead()
		if err != nil {
			t.Fatalf("BuildTransceiverIDRead: %v", err)
		}
		_, err = e.Do(context.Background(), cmd, fastRead(p.TransceiverIDAnswerMatcher(), 0))
		if errors.Is(err, transport.ErrRejected) {
			t.Fatalf("Do error = %v: an FA from %#02x is another station's refusal, not ours", err, otherRadio)
		}
		if !errors.Is(err, transport.ErrTimeout) {
			t.Fatalf("Do error = %v, want ErrTimeout — the frame is unexpected traffic, counted and never matched", err)
		}
	})

	t.Run("from our radio it is", func(t *testing.T) {
		port := newCIVPort(rawFrame(p.ControllerAddress(), p.RadioAddress(), NakByte))
		t.Cleanup(func() { _ = port.Close() })
		e, _ := newCIVEngine(t, p, port)

		cmd, err := p.BuildTransceiverIDRead()
		if err != nil {
			t.Fatalf("BuildTransceiverIDRead: %v", err)
		}
		if _, err := e.Do(context.Background(), cmd, fastRead(p.TransceiverIDAnswerMatcher(), 0)); !errors.Is(err, transport.ErrRejected) {
			t.Fatalf("Do error = %v, want ErrRejected", err)
		}
	})
}

// TestFraming_AckMatcherIsSourceAddressChecked is the same catch on the
// other frame. An FB from another radio acknowledges another controller's
// write; treating it as ours would report a memory set as landed when this
// radio never saw it.
func TestFraming_AckMatcherIsSourceAddressChecked(t *testing.T) {
	p := flatProfile
	const otherRadio = 0x66
	port := newCIVPort(rawFrame(p.ControllerAddress(), otherRadio, AckByte))
	t.Cleanup(func() { _ = port.Close() })
	e, _ := newCIVEngine(t, p, port)

	cmd, err := p.BuildMemorySet(sampleRecord(t, p, p.BuildRecordLength()))
	if err != nil {
		t.Fatalf("BuildMemorySet: %v", err)
	}
	spec := CIVWriteWithAckSpec(p.AcknowledgementMatcher())
	spec.Timeout = 100 * time.Millisecond
	if _, err := e.Do(context.Background(), cmd, spec); !errors.Is(err, transport.ErrTimeout) {
		t.Fatalf("Do error = %v, want ErrTimeout: an FB from %#02x is not our acknowledgement", err, otherRadio)
	}
}

// TestFraming_MatchersAreSourceAddressCheckedOnEveryProfile walks the
// disagreeing fixtures over all three matchers, as unit predicates: a
// matcher that reached for ControllerAddressDefault or for a package-level
// address instead of its receiver's would pass on two fixtures and fail
// here.
func TestFraming_MatchersAreSourceAddressCheckedOnEveryProfile(t *testing.T) {
	for _, np := range allTestProfiles() {
		t.Run(np.name, func(t *testing.T) {
			p := np.p
			const other = 0x66
			ctrl, radio := p.ControllerAddress(), p.RadioAddress()

			id := p.TransceiverIDAnswerMatcher()
			if !id(rawFrame(ctrl, radio, CmdTransceiverID, SubTransceiverID, 0x01)) {
				t.Error("TransceiverIDAnswerMatcher refused this profile's own answer")
			}
			if id(rawFrame(ctrl, other, CmdTransceiverID, SubTransceiverID, 0x01)) {
				t.Error("TransceiverIDAnswerMatcher matched an answer from another radio")
			}
			if id(rawFrame(radio, ctrl, CmdTransceiverID, SubTransceiverID)) {
				t.Error("TransceiverIDAnswerMatcher matched the outbound COMMAND — the address direction is reversed")
			}

			ack := p.AcknowledgementMatcher()
			if !ack(rawFrame(ctrl, radio, AckByte)) {
				t.Error("AcknowledgementMatcher refused this profile's own FB")
			}
			if ack(rawFrame(ctrl, other, AckByte)) {
				t.Error("AcknowledgementMatcher matched an FB from another radio")
			}
			if ack(rawFrame(ctrl, radio, AckByte, 0x00)) {
				t.Error("AcknowledgementMatcher matched an FB-shaped frame carrying data — the length must be exact")
			}
			if ack(rawFrame(ctrl, radio, NakByte)) {
				t.Error("AcknowledgementMatcher matched an FA")
			}

			if p.IsRejection(rawFrame(ctrl, other, NakByte)) {
				t.Error("Profile.IsRejection accepted an FA from another radio")
			}
			if !p.IsRejection(rawFrame(ctrl, radio, NakByte)) {
				t.Error("Profile.IsRejection refused this profile's own FA")
			}
		})
	}
}

// TestFraming_MemoryAnswerMatcherIsAddressChecked covers the third
// matcher: the memory answer, which must be from this radio, to this
// controller, and long enough to carry this profile's own address field.
func TestFraming_MemoryAnswerMatcherIsAddressChecked(t *testing.T) {
	for _, np := range allTestProfiles() {
		t.Run(np.name, func(t *testing.T) {
			p := np.p
			ctrl, radio := p.ControllerAddress(), p.RadioAddress()
			n := p.AddressForm().addressBytes()

			body := []byte{CmdMemory, SubMemoryContents}
			body = append(body, make([]byte, n)...)
			m := p.MemoryAnswerMatcher()
			if !m(rawFrame(ctrl, radio, body...)) {
				t.Error("MemoryAnswerMatcher refused a well-addressed memory answer")
			}
			if m(rawFrame(ctrl, 0x66, body...)) {
				t.Error("MemoryAnswerMatcher matched an answer from another radio")
			}
			short := append([]byte{CmdMemory, SubMemoryContents}, make([]byte, n-1)...)
			if m(rawFrame(ctrl, radio, short...)) {
				t.Errorf("MemoryAnswerMatcher matched a frame too short for this profile's %d-byte address field", n)
			}
		})
	}
}

// TestFraming_EchoIsSuppressedOnlyWhenByteIdentical is the seam's echo
// contract, both halves. A REMOTE-bus radio echoes what we write; the
// accumulator drops the echo BY RECORDED BYTES. A frame that merely looks
// like our write — one byte different — is NOT our echo and must not be
// swallowed: on a shared bus it is another controller's traffic, and
// suppressing it by position or by count would discard whatever arrived
// first, answer included.
func TestFraming_EchoIsSuppressedOnlyWhenByteIdentical(t *testing.T) {
	p := flatProfile

	t.Run("byte-identical echo is dropped", func(t *testing.T) {
		port := newCIVPort(idAnswer(p))
		port.echo = true
		t.Cleanup(func() { _ = port.Close() })
		e, f := newCIVEngine(t, p, port)

		cmd, err := p.BuildTransceiverIDRead()
		if err != nil {
			t.Fatalf("BuildTransceiverIDRead: %v", err)
		}
		got, err := e.Do(context.Background(), cmd, fastRead(p.TransceiverIDAnswerMatcher(), 0))
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		if string(got) != string(idAnswer(p)) {
			t.Errorf("answer = % x, want the radio's own % x — the echo reached the caller", got, idAnswer(p))
		}
		stats := f.(AccumulatorStatsReporter).AccumulatorStats()
		if stats.Echoes != 1 {
			t.Errorf("AccumulatorStats().Echoes = %d, want 1", stats.Echoes)
		}
	})

	t.Run("a mutated echo is not suppressed", func(t *testing.T) {
		port := newCIVPort()
		t.Cleanup(func() { _ = port.Close() })
		e, f := newCIVEngine(t, p, port)

		cmd, err := p.BuildTransceiverIDRead()
		if err != nil {
			t.Fatalf("BuildTransceiverIDRead: %v", err)
		}
		// The bus returns a frame ONE BYTE different from what we wrote:
		// same command, another station's address in the `from` slot.
		mutated := rawFrame(p.ControllerAddress(), 0x66, CmdTransceiverID, SubTransceiverID)
		go func() {
			<-time.After(10 * time.Millisecond)
			port.deliver(mutated)
			port.deliver(idAnswer(p))
		}()
		got, err := e.Do(context.Background(), cmd, fastRead(p.TransceiverIDAnswerMatcher(), 0))
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		if string(got) != string(idAnswer(p)) {
			t.Errorf("answer = % x, want % x", got, idAnswer(p))
		}
		stats := f.(AccumulatorStatsReporter).AccumulatorStats()
		if stats.Echoes != 0 {
			t.Errorf("AccumulatorStats().Echoes = %d, want 0 — a frame one byte different from the write is NOT this program's echo", stats.Echoes)
		}
	})
}

// TestFraming_OversizeFrameContaminatesRatherThanClosingThePort is the
// ERROR TRANSLATION proof, end to end and in exactly the terms that make
// it matter. civ mints its OWN *FrameTooLongError; transport's
// handleReaderErr recognises only its own type. Handed over untranslated,
// a noisy line would be read as a dead port and TORN DOWN; translated, it
// enters the recoverable CONTAMINATED state a DrainToQuiet can clear.
func TestFraming_OversizeFrameContaminatesRatherThanClosingThePort(t *testing.T) {
	p := flatProfile
	port := newCIVPort()
	t.Cleanup(func() { _ = port.Close() })
	e, _ := newCIVEngine(t, p, port)

	// A preamble pair followed by more body bytes than this profile's own
	// frame bound allows, and no terminator.
	noise := []byte{PreambleByte, PreambleByte}
	for i := 0; i < p.MaxFrame()+16; i++ {
		noise = append(noise, 0x01)
	}
	port.deliver(noise)

	cmd, err := p.BuildTransceiverIDRead()
	if err != nil {
		t.Fatalf("BuildTransceiverIDRead: %v", err)
	}
	var lastErr error
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, lastErr = e.Do(context.Background(), cmd, fastRead(p.TransceiverIDAnswerMatcher(), 0))
		if errors.Is(lastErr, transport.ErrContaminated) || errors.Is(lastErr, transport.ErrPortClosed) {
			break
		}
	}
	if errors.Is(lastErr, transport.ErrPortClosed) {
		t.Fatalf("Do error = %v: an oversize CI-V frame closed the port as generic I/O — the error was not translated onto transport's contamination sentinel", lastErr)
	}
	if !errors.Is(lastErr, transport.ErrContaminated) {
		t.Fatalf("Do error = %v, want ErrContaminated", lastErr)
	}
	var tooLong *transport.FrameTooLongError
	if !errors.As(lastErr, &tooLong) {
		t.Fatalf("Do error = %v: no *transport.FrameTooLongError to recover DiscardedLen from", lastErr)
	}
	if tooLong.DiscardedLen <= 0 {
		t.Errorf("DiscardedLen = %d, want the count of bytes thrown away", tooLong.DiscardedLen)
	}
}

// TestFraming_ContinuousFloodDoesNotWedgeTheEngine is D2's starvation
// deadline over the real adapter, with a fake that NEVER goes quiet — and
// it pins TWO different outcomes, because on CI-V the two halves of a
// flood are absorbed at different layers.
//
// A TRANSCEIVE BROADCAST FLOOD NEVER REACHES THE ENGINE AT ALL. The
// accumulator's `to` filter drops every frame not addressed to this
// controller before Push returns it, so the reader goroutine delivers no
// event, so nothing postpones the drain's idle timer. That is spec D2's
// "broadcasts are excluded by address matching" turning out to be a
// STARVATION property as well as a correctness one, and it is the reason
// this tier can tolerate factory-ON transceive without writing a setting
// to anybody's radio: the flood is structurally invisible to the state
// machine. Init succeeds under it.
//
// A FLOOD ADDRESSED TO US IS THE ONE THAT COSTS. Another controller
// configured to this program's own address, a radio spraying answers, a
// wedged adapter repeating a frame — those pass the filter, reach the
// engine, and re-arm the idle gap indefinitely. THAT is what DrainCap is
// for, and the test requires the drain to give up at the absolute cap
// rather than postponing itself, and the engine to still be answering
// afterwards.
func TestFraming_ContinuousFloodDoesNotWedgeTheEngine(t *testing.T) {
	p := flatProfile

	// flood starts a goroutine spraying frame at the port until cleanup.
	flood := func(t *testing.T, port *civPort, frame []byte) {
		t.Helper()
		stop := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				select {
				case <-stop:
					return
				default:
				}
				port.deliver(frame)
				time.Sleep(2 * time.Millisecond)
			}
		}()
		t.Cleanup(func() { close(stop); <-done })
	}

	t.Run("transceive broadcasts never reach the engine", func(t *testing.T) {
		port := newCIVPort()
		t.Cleanup(func() { _ = port.Close() })
		// Addressed to 0x00, the broadcast address, from this radio —
		// factory-ON on at least four models in this tier.
		flood(t, port, rawFrame(0x00, p.RadioAddress(), 0x00, 0x00, 0x01, 0x02, 0x03))
		e, f := newCIVEngine(t, p, port)

		if err := e.Init(context.Background()); err != nil {
			t.Fatalf("Init under a continuous transceive flood: %v — broadcasts are excluded by address matching and must not postpone a drain", err)
		}
		if n := len(port.written()); n != 0 {
			t.Errorf("Init wrote %d frames, want none", n)
		}
		stats := f.(AccumulatorStatsReporter).AccumulatorStats()
		if stats.Unexpected == 0 {
			t.Error("AccumulatorStats().Unexpected = 0 under a continuous broadcast flood — the broadcasts were not counted, so this test proved nothing")
		}
		if stats.Frames != 0 {
			t.Errorf("AccumulatorStats().Frames = %d, want 0 — a broadcast was returned to the engine", stats.Frames)
		}
	})

	t.Run("a flood addressed to us fails the drain at its absolute cap", func(t *testing.T) {
		port := newCIVPort()
		t.Cleanup(func() { _ = port.Close() })
		flood(t, port, rawFrame(p.ControllerAddress(), p.RadioAddress(), 0x00, 0x00, 0x01, 0x02, 0x03))
		e, _ := newCIVEngine(t, p, port)

		start := time.Now()
		err := e.Init(context.Background())
		elapsed := time.Since(start)
		if !errors.Is(err, transport.ErrDrainCapExceeded) {
			t.Fatalf("Init error = %v, want ErrDrainCapExceeded under a stream that never goes quiet", err)
		}
		if elapsed > 4*DrainCap {
			t.Errorf("Init took %v against an absolute cap of %v — the drain postponed itself", elapsed, DrainCap)
		}

		// THE RULE THIS TIER READS OFF THAT ERROR: at Init it is nonfatal
		// with a diagnostic, so the session goes on to talk to the radio.
		// The engine is not wedged — it answers a read taken from the
		// same flooded stream.
		port.deliver(idAnswer(p))
		cmd, cerr := p.BuildTransceiverIDRead()
		if cerr != nil {
			t.Fatalf("BuildTransceiverIDRead: %v", cerr)
		}
		spec := fastRead(p.TransceiverIDAnswerMatcher(), 5)
		spec.Timeout = 300 * time.Millisecond
		if _, derr := e.Do(context.Background(), cmd, spec); derr != nil {
			t.Fatalf("Do after a capped Init drain: %v — the engine is wedged", derr)
		}
	})
}

// TestFraming_AccumulatorBoundIsTheProfilesOwn proves the constructor's
// bound choice where it can be seen: a frame LONGER than this profile's
// MaxFrame but shorter than civ.DefaultMaxFrame must be refused as
// contamination. bandProfile's bound is 18; DefaultMaxFrame is 256.
func TestFraming_AccumulatorBoundIsTheProfilesOwn(t *testing.T) {
	p := bandProfile
	if p.MaxFrame() >= DefaultMaxFrame {
		t.Fatalf("fixture error: %s's MaxFrame %d is not below DefaultMaxFrame %d", p.Model(), p.MaxFrame(), DefaultMaxFrame)
	}
	f, err := NewFraming(p)
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	acc := f.NewAccumulator(0) // 0: let the framing choose its own default

	over := []byte{PreambleByte, PreambleByte}
	for i := 0; i < DefaultMaxFrame-8; i++ {
		over = append(over, 0x01)
	}
	over = append(over, EndByte)
	_, err = acc.Push(over)
	if err == nil {
		t.Fatalf("Push of a %d-byte frame succeeded: the accumulator's bound is not %s's own %d", len(over), p.Model(), p.MaxFrame())
	}
	if !errors.Is(err, transport.ErrFrameTooLong) {
		t.Errorf("Push error = %v, want one matching transport.ErrFrameTooLong", err)
	}
}

// TestFraming_NoteSentAndPushShareTheAdaptersLock is the race the
// adapter's mutex exists for, written so `go test -race` can see it:
// NoteSent runs on the caller's goroutine while Push runs on the reader's,
// and both reach the same non-concurrency-safe FrameAccumulator.
func TestFraming_NoteSentAndPushShareTheAdaptersLock(t *testing.T) {
	f, err := NewFraming(flatProfile)
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	acc := f.NewAccumulator(0)
	frame := rawFrame(flatProfile.RadioAddress(), flatProfile.ControllerAddress(), CmdTransceiverID, SubTransceiverID)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			f.NoteSent(frame)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_, _ = acc.Push(frame)
		}
	}()
	wg.Wait()

	// And the reporter is safe from a third goroutine's point of view too.
	_ = f.(AccumulatorStatsReporter).AccumulatorStats()
}
