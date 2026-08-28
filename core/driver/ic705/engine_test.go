// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import (
	"bytes"
	"io"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic705 "github.com/gm5dna/open-rig-programmer/core/civ/ic705"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// THE MODEL PACKAGE IS ALIASED, and it has to be: core/civ/ic705's package
// name is also "ic705", and this package answers to that spelling already.
// The alias reads as "the core/civ side of the IC-705", which is exactly
// what it is — core/driver/ftdx101's catftdx101 import is the same move for
// the same reason. The plan's own fragments write the model profile as
// `ic705.Profile()`, which is the spelling a test OUTSIDE this package
// would use; inside it, `Profile` is the driver's own capability-profile
// TYPE (Task 8), so the qualified name is the only one available and the
// only one that cannot be misread.

// silentPort is a transport.Port that never speaks: Read blocks until
// Close, and every Write is recorded.
//
// It is not a scripted radio (Task 9 has one of those). It is what the
// engine-construction tests need and no more: a port an Engine can be
// built over, whose reader goroutine parks rather than spinning, and which
// can testify that a construction path wrote NOTHING.
type silentPort struct {
	closed chan struct{}
	once   sync.Once

	mu      sync.Mutex
	written [][]byte
}

func newSilentPort() *silentPort { return &silentPort{closed: make(chan struct{})} }

func (p *silentPort) Read(b []byte) (int, error) {
	<-p.closed
	return 0, io.EOF
}

func (p *silentPort) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.written = append(p.written, append([]byte(nil), b...))
	return len(b), nil
}

func (p *silentPort) Close() error {
	p.once.Do(func() { close(p.closed) })
	return nil
}

// Written returns a copy of every frame written to this port, in order.
func (p *silentPort) Written() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([][]byte, len(p.written))
	copy(out, p.written)
	return out
}

// memoryAnswer builds a `1A 00` answer frame in the ANSWER direction — to
// the controller, from the radio — for the given wire group and channel,
// carrying a 111-byte record of the given filler byte.
//
// BUILT BY HAND, not by a builder in the package under test. civ has no
// answer-direction builder at all (its builders emit COMMANDS), and a
// fixture derived from the code being tested would agree with a wrong
// answer geometry as readily as with a right one.
func memoryAnswer(to, from byte, group, channel int, fill byte) []byte {
	f := []byte{0xFE, 0xFE, to, from, 0x1A, 0x00}
	f = append(f, bcd2(group)...)
	f = append(f, bcd2(channel)...)
	rec := make([]byte, 111)
	for i := range rec {
		rec[i] = fill
	}
	f = append(f, rec...)
	return append(f, 0xFD)
}

func TestNewEngineHandsBackTheStatsCarrier(t *testing.T) {
	// The carrier the DIAGNOSTICS CARRIER ruling requires. The two-result
	// assertion is made HERE, once, at the only place that holds the
	// concrete framing value — transport.Engine keeps its framing
	// unexported, so a Session that never saw it could not assert
	// civ.AccumulatorStatsReporter later at all.
	//
	// A failure is a STOP: E1 lands AccumulatorStatsReporter on its
	// adapter, so a non-implementing value means the tree is not what this
	// plan was written against.
	port := newSilentPort()
	eng, stats, err := newEngine(port)
	if err != nil {
		t.Fatalf("newEngine: %v", err)
	}
	defer eng.Close()
	if stats == nil {
		t.Fatal("newEngine returned no AccumulatorStatsReporter — Diagnostics() could not be built")
	}
	if got := stats.AccumulatorStats(); got.Frames != 0 {
		t.Errorf("a fresh accumulator reports Frames = %d, want 0", got.Frames)
	}
}

func TestEngineIsBuiltFromTheCodecsOwnFraming(t *testing.T) {
	// One call site, and it is E1's constructor — not a local adapter, not
	// transport.NewEngine's CAT wrapper.
	fr, err := civ.NewFraming(civic705.Profile())
	if err != nil {
		t.Fatalf("civ.NewFraming: %v", err)
	}
	if got := fr.InitSequence(); len(got) != 0 {
		t.Errorf("InitSequence returned %d commands, want none — Init must never write to the radio", len(got))
	}
	if _, ok := fr.(civ.AccumulatorStatsReporter); !ok {
		t.Fatalf("civ.NewFraming returned %T, which does not report accumulator stats — the diagnostics carrier has no source", fr)
	}
}

func TestUnconfiguredProfileCannotBuildAnEngine(t *testing.T) {
	// E1 errors on a zero profile; this driver's framing step must
	// PROPAGATE that, never fall back to a default framing. A default
	// would install a gate speaking for no radio.
	fr, stats, err := newFramingFor(civ.Profile{})
	if err == nil {
		t.Fatal("newFramingFor accepted a zero civ.Profile — a framing built from one gates for no radio")
	}
	if fr != nil || stats != nil {
		t.Errorf("newFramingFor returned framing %v / stats %v alongside its error — the failure must be total", fr, stats)
	}
}

func TestReadSpecMatchesOnlyThisRadiosAnswer(t *testing.T) {
	p := civic705.Profile()
	s := civ.CIVReadSpec(p.MemoryAnswerMatcher(), 1)
	if s.Class != transport.ClassRead {
		t.Errorf("Class = %v, want ClassRead", s.Class)
	}
	if s.RetryReads != 1 {
		t.Errorf("RetryReads = %d, want 1 — an identity-shaped read is idempotent and one swallowed reply must not fail an open", s.RetryReads)
	}
	if !s.Match(memoryAnswer(0xE0, 0xA4, 0, 12, 0x00)) {
		t.Error("the read spec did not match this radio's own memory answer")
	}
	if s.Match(memoryAnswer(0xE0, 0x94, 0, 12, 0x00)) {
		t.Error("the read spec matched a memory answer from ANOTHER radio's address")
	}
	if s.Match(memoryAnswer(0xA4, 0xE0, 0, 12, 0x00)) {
		t.Error("the read spec matched our own echo (to == A4) — an echo is not an answer")
	}

	// AND THE FACT THAT MAKES T2 NECESSARY. The landed MemoryAnswerMatcher
	// is deliberately ENVELOPE-ONLY (core/civ/framing.go's own doc
	// comment), so an answer for the WRONG CHANNEL matches. Pinned here so
	// that nobody mistakes the matcher for an address check: the equality
	// check is the DRIVER's job (T2, Task 10), and this is the reason.
	if !s.Match(memoryAnswer(0xE0, 0xA4, 0, 99, 0x00)) {
		t.Error("the memory answer matcher rejected a wrong-channel answer — if the codec checks the address, Task 10's own check is redundant and this plan's T2 rests on a false premise")
	}
}

func TestWriteWithAckSpecNeverRetransmits(t *testing.T) {
	s := civ.CIVWriteWithAckSpec(civic705.Profile().AcknowledgementMatcher())
	if s.Class != transport.ClassWriteWithAck {
		t.Errorf("Class = %v, want ClassWriteWithAck — a CI-V memory set waits for FB", s.Class)
	}
	if s.RetryReads != 0 {
		t.Errorf("RetryReads = %d, want 0 — a retransmitted memory set could write the channel twice", s.RetryReads)
	}
	if !s.Match([]byte{0xFE, 0xFE, 0xE0, 0xA4, 0xFB, 0xFD}) {
		t.Error("the ack matcher did not match the FB the manual prints as the OK message (PDF p.3, folio 2)")
	}
	if s.Match([]byte{0xFE, 0xFE, 0xE0, 0xA4, 0xFA, 0xFD}) {
		t.Error("the ack matcher matched an FA — rejection is IsRejection's job, not Match's")
	}
	if s.Match([]byte{0xFE, 0xFE, 0xE0, 0x94, 0xFB, 0xFD}) {
		t.Error("the ack matcher matched an FB from ANOTHER radio's address")
	}
}

func TestNoteSentAndPushAreSafeUnderRace(t *testing.T) {
	// E1 owns the lock; this is the CONSUMER's proof that the landed
	// adapter actually holds it, because it is this driver that would
	// corrupt memory if it did not. Run under -race.
	fr, err := civ.NewFraming(civic705.Profile())
	if err != nil {
		t.Fatalf("civ.NewFraming: %v", err)
	}
	acc := fr.NewAccumulator(0)
	sent := memoryAnswer(0xA4, 0xE0, 0, 1, 0x00)
	inbound := memoryAnswer(0xE0, 0xA4, 0, 1, 0x00)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			fr.NoteSent(sent)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			if _, err := acc.Push(inbound); err != nil {
				t.Errorf("Push: %v", err)
				return
			}
		}
	}()
	wg.Wait()

	stats := fr.(civ.AccumulatorStatsReporter).AccumulatorStats()
	if stats.Frames == 0 {
		t.Error("the accumulator returned no frames at all — the race test pushed 1000 complete answers")
	}
}

func TestNewEngineWritesNothingAtConstruction(t *testing.T) {
	// Construction is not a conversation. Building the engine must not put
	// a byte on the wire: InitSequence is empty, and even Init is a
	// separate call the driver makes later.
	port := newSilentPort()
	eng, _, err := newEngine(port)
	if err != nil {
		t.Fatalf("newEngine: %v", err)
	}
	defer eng.Close()
	if got := port.Written(); len(got) != 0 {
		t.Errorf("construction wrote %d frames (%X) — no IC-705 may be spoken to before its driver decides to", len(got), bytes.Join(got, nil))
	}
}
