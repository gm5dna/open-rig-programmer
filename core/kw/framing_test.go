// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"errors"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestNewFraming_RefusesAnUnsetBook is the fail-closed rule this package
// inherits from the house standing rules: an omitted config semantic is
// REFUSED, never defaulted. The book is not decoration — it is what decides
// which of the two cause sentences an "O;" carries (E13), so a framing built
// without one would quote a document it was never told it was talking to.
func TestNewFraming_RefusesAnUnsetBook(t *testing.T) {
	f, err := NewFraming(BookUnset)
	if err == nil {
		t.Fatalf("NewFraming(BookUnset) returned a framing (%v) and no error", f)
	}
	if !errors.Is(err, ErrUnconfiguredBook) {
		t.Errorf("NewFraming error = %v, want errors.Is match against ErrUnconfiguredBook", err)
	}
	if f != nil {
		t.Errorf("NewFraming returned %v alongside its error, want nil", f)
	}
	if _, err := NewFraming(Book(99)); err == nil {
		t.Error("NewFraming accepted a Book value neither book names")
	}
}

// TestNewFraming_BothBooksBuild pins the positive control that the refusal
// above is not vacuous.
func TestNewFraming_BothBooksBuild(t *testing.T) {
	for _, b := range []Book{Book590, Book480} {
		f, err := NewFraming(b)
		if err != nil {
			t.Fatalf("NewFraming(%v): unexpected error: %v", b, err)
		}
		if f == nil {
			t.Fatalf("NewFraming(%v) returned nil with no error", b)
		}
	}
}

// TestFraming_InitSequenceIsAI0AndNothingElse pins the ONE frame a Kenwood
// session writes at open.
//
// AI0 is the only AI state either radio is ever sent. The 590 pair's legend
// has no AI1 and no AI3 at all (590:159-162) and its AI2/AI4 push a response
// per changed parameter (590:165-167); the 480's AI1/AI3 push an IF frame
// every 1.5 s while the IF parameters change (480:194-195). Nothing in this
// programme wants either, and the two assertions below are what stop a
// later reader adding one: exactly ONE command, and that command byte-equal
// to "AI0;". A further loop over "AI1;".."AI4;" used to follow them, and it
// could only ever fire in company with the equality check — it is gone,
// because a test line that cannot fail alone reads as coverage and is not.
func TestFraming_InitSequenceIsAI0AndNothingElse(t *testing.T) {
	for _, b := range []Book{Book590, Book480} {
		f, err := NewFraming(b)
		if err != nil {
			t.Fatalf("NewFraming(%v): %v", b, err)
		}
		seq := f.InitSequence()
		if len(seq) != 1 {
			t.Fatalf("%v: InitSequence() has %d commands, want exactly 1", b, len(seq))
		}
		if got := string(seq[0].Bytes()); got != "AI0;" {
			t.Errorf("%v: InitSequence()[0] = %q, want \"AI0;\" — no AI state but 0 is ever transmitted", b, got)
		}
	}
}

// TestFraming_IsRejectionIsTheNAKOnly restates frame.go's rule at the seam
// that matters: the engine turns IsRejection into ErrRejected, so an "E;"
// admitted here would be reported to the user as their radio refusing their
// command when in fact the link failed (spec §"Where E; and O; live",
// rejected route 3).
func TestFraming_IsRejectionIsTheNAKOnly(t *testing.T) {
	f, err := NewFraming(Book590)
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	if !f.IsRejection([]byte("?;")) {
		t.Error("IsRejection(\"?;\") = false, want true")
	}
	for _, frame := range []string{"E;", "O;", "ID023;", ";"} {
		if f.IsRejection([]byte(frame)) {
			t.Errorf("IsRejection(%q) = true, want false", frame)
		}
	}
}

// TestFraming_DrainPolicyIsSizedForThe480Push pins P20: the cap is a NAMED
// decision, not transport's default, and it is sized against the 480's
// documented worst case read UNCONDITIONALLY — one IF frame per 1.5 s
// (480:194-195) — because that is the conservative reading.
func TestFraming_DrainPolicyIsSizedForThe480Push(t *testing.T) {
	f, err := NewFraming(Book480)
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	p := f.DrainPolicy()
	if p.IdleGap != transport.QuietPeriod {
		t.Errorf("IdleGap = %v, want transport.QuietPeriod (%v)", p.IdleGap, transport.QuietPeriod)
	}
	if p.Cap != DrainCap {
		t.Errorf("Cap = %v, want DrainCap (%v)", p.Cap, DrainCap)
	}
	// The arithmetic, pinned rather than left in the doc comment: one full
	// push interval plus two idle gaps. A drain that starts just after a
	// push must be able to wait out the next one and still find its gap.
	const pushInterval = 1500 * time.Millisecond
	if want := pushInterval + 2*transport.QuietPeriod; p.Cap < want {
		t.Errorf("Cap = %v, want at least %v — one 480 IF push interval plus two idle gaps", p.Cap, want)
	}
	// And it is still a CEILING: a line that never pauses must fail the
	// drain rather than hold the engine mutex indefinitely.
	if p.Cap > 5*time.Second {
		t.Errorf("Cap = %v — too generous to be a ceiling on one drain", p.Cap)
	}
	// transport's own default would be 2*IdleGap; a value equal to it would
	// mean P20 had been implemented as "leave it to the default".
	if p.Cap == 2*p.IdleGap {
		t.Error("Cap equals transport's own default (2*IdleGap) — P20 requires a sized decision, not a default")
	}
}

// TestFraming_NoteSentIsANoOp pins A25's consequence: Kenwood radios are
// assumed not to echo the host's own frames, so nothing is recorded and
// nothing is suppressed. The assertion is behavioural, not "the method body
// is empty": a frame byte-equal to one just noted must still come back out
// of the accumulator.
func TestFraming_NoteSentIsANoOp(t *testing.T) {
	f, err := NewFraming(Book590)
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	acc := f.NewAccumulator(0)
	f.NoteSent([]byte("AI0;"))
	frames, err := acc.Push([]byte("AI0;"))
	if err != nil {
		t.Fatalf("Push: unexpected error: %v", err)
	}
	if len(frames) != 1 || string(frames[0]) != "AI0;" {
		t.Errorf("got %q, want the noted frame delivered unchanged — this framing suppresses no echo", frames)
	}
}

// TestFraming_NewAccumulatorHonoursMax pins the engine's WithMaxFrame route.
func TestFraming_NewAccumulatorHonoursMax(t *testing.T) {
	f, err := NewFraming(Book480)
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	acc := f.NewAccumulator(8)
	if _, err := acc.Push([]byte("123456789")); err == nil {
		t.Error("Push accepted 9 bytes through an accumulator built with max 8")
	}
	if _, err := f.NewAccumulator(0).Push([]byte("123456789")); err != nil {
		t.Errorf("max <= 0 did not select DefaultMaxFrame: %v", err)
	}
}

// TestEnvelopeAllows_TheDocumentedEnvelope pins what T5's gate knows: the
// ENVELOPE both books print, and nothing about which commands exist. T7
// completes the gate with the eight grammars; until then this is the whole
// of it, and it is written so that widening it later is an addition rather
// than a rewrite.
//
// The three OUTBOUND-TOKEN rows record an outcome, not a mechanism: "?;",
// "E;" and "O;" are two bytes each and are refused by the two-byte-opcode
// floor, so they would still be refused with the explicit token branch
// deleted. envelopeAllows' fourth bullet says so rather than letting these
// rows imply otherwise.
func TestEnvelopeAllows_TheDocumentedEnvelope(t *testing.T) {
	tests := []struct {
		name  string
		frame string
		want  bool
	}{
		{"an ordinary read", "ID;", true},
		{"a three-character opcode (VS0 exists, E3)", "VS0;", true},
		{"the mandatory literal space IS Set carries (590:1184)", "IS 0000;", true},
		{"an MC Set below 100 with the space convention (590:1334-1337)", "MC 07;", true},
		{"empty", "", false},
		{"no terminator", "ID", false},
		{"an embedded terminator", "ID;;", false},
		{"a terminator that is not last", "I;D;", false},
		{"one byte of opcode", "I;", false},
		{"a control byte in the body (480:127-129)", "MW\x01 0007;", false},
		{"a DEL byte in the body", "MW\x7f0007;", false},
		{"a high byte in the body (A2 claims nothing above 0x7E)", "MW\xc30007;", false},
		{"a lower-case opcode is not built by this programme", "id;", false},
		{"the NAK, outbound", "?;", false},
		{"the communication-error token, outbound", "E;", false},
		{"the overrun token, outbound", "O;", false},
		{"the bare terminator a noisy line delivers", ";", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := envelopeAllows([]byte(tt.frame)); got != tt.want {
				t.Errorf("envelopeAllows(%q) = %v, want %v", tt.frame, got, tt.want)
			}
		})
	}
}

// TestFraming_AllowRefusesAnOverlongFrame pins the one bound the envelope
// gate carries that is not a byte rule: nothing longer than the accumulator
// could ever reassemble may go out.
func TestFraming_AllowRefusesAnOverlongFrame(t *testing.T) {
	f, err := NewFraming(Book590)
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	long := make([]byte, DefaultMaxFrame+1)
	for i := range long {
		long[i] = 'A'
	}
	long[len(long)-1] = ';'
	if f.Allow(long) {
		t.Error("Allow admitted a frame longer than DefaultMaxFrame")
	}
	if !f.Allow([]byte("ID;")) {
		t.Error("Allow refused a well-formed read frame")
	}
}

// TestFraming_AllowsBoundIsTheDefaultAccumulatorsBound pins the half of the
// gate's length rule that is actually true, because the doc comment used to
// claim the whole of it.
//
// The gate's ceiling and the ceiling of the accumulator THIS SAME ADAPTER
// hands out with max <= 0 are ONE datum, DefaultMaxFrame, consulted from one
// place — the house rule that a bound is consulted from the same place as
// its datum. The frame exactly at the bound must pass both; the frame one
// byte over must fail both.
//
// WHAT THIS DOES NOT PIN, AND CANNOT: an Engine built WithMaxFrame(N) for
// N < DefaultMaxFrame narrows its accumulator only. The seam gives Allow no
// sight of that option — the Framing receiver is a value and NewAccumulator
// is the only method told the number — so on such a session this gate is the
// wider of the two. framing.go's fifth envelope bullet says so rather than
// claiming otherwise.
func TestFraming_AllowsBoundIsTheDefaultAccumulatorsBound(t *testing.T) {
	f, err := NewFraming(Book590)
	if err != nil {
		t.Fatalf("NewFraming: %v", err)
	}
	frameOfLen := func(n int) []byte {
		b := make([]byte, n)
		for i := range b {
			b[i] = 'A'
		}
		b[n-1] = ';'
		return b
	}

	atBound := frameOfLen(DefaultMaxFrame)
	if !f.Allow(atBound) {
		t.Errorf("Allow refused a %d-byte frame, which is exactly the bound", DefaultMaxFrame)
	}
	frames, err := f.NewAccumulator(0).Push(atBound)
	if err != nil || len(frames) != 1 {
		t.Errorf("the default accumulator returned (%d frames, %v) for the longest frame Allow admits — the gate and the accumulator are meant to read one bound", len(frames), err)
	}

	over := frameOfLen(DefaultMaxFrame + 1)
	if f.Allow(over) {
		t.Errorf("Allow admitted a %d-byte frame, one over the bound", DefaultMaxFrame+1)
	}
	if _, err := f.NewAccumulator(0).Push(over); err == nil {
		t.Error("the default accumulator reassembled a frame one byte over the bound that Allow refuses")
	}
}

// TestFraming_ZeroValueFailsClosed is the standing rule stated for this
// adapter: a hand-built zero value describes no radio, and its gate must
// admit nothing rather than admit everything.
func TestFraming_ZeroValueFailsClosed(t *testing.T) {
	var zero framing
	if zero.Allow([]byte("ID;")) {
		t.Error("a zero framing admitted a frame — it speaks for no radio and must fail closed")
	}
	if len(zero.InitSequence()) != 0 {
		t.Error("a zero framing offered an init sequence")
	}
	// AND ITS THIRD DOOR, IsFatal, WHICH IS THE ONE THAT RUNS ON THE
	// ENGINE'S READER GOROUTINE. That goroutine has no recover, so a
	// verdict this adapter cannot honestly give must be withheld rather
	// than raised: an adapter that names no document can quote no cause
	// sentence, and inventing one would put words in a manufacturer's
	// mouth (the M9c-1 ruling — an omitted config semantic is REFUSED,
	// never defaulted). Withholding is the fail-closed direction here
	// because Allow above already admits nothing, so no frame can leave.
	for _, frame := range []string{"E;", "O;", "ID023;", ""} {
		if err := zero.IsFatal([]byte(frame)); err != nil {
			t.Errorf("a zero framing returned %v for %q — it speaks for no document and must quote none", err, frame)
		}
	}
}
