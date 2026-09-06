// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestPrefixLenMatcher_FixedLengthFamilies pins the ordinary branch: an
// answer must start with the prefix AND be exactly exactLen bytes.
func TestPrefixLenMatcher_FixedLengthFamilies(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		exactLen int
		frame    string
		want     bool
	}{
		{"ID answer, 6 bytes", "ID", 6, "ID023;", true},
		{"a Yaesu 7-byte ID answer refused", "ID", 6, "ID0800;", false},
		{"FV answer, 7 bytes", "FV", 7, "FV1.00;", true},
		{"TY answer, 6 bytes", "TY", 6, "TY001;", true},
		{"a five-byte TY refused", "TY", 6, "TY01;", false},
		{"MR answer, 50 bytes", "MR", 50, "MR" + "0" + "0" + "07" + "00014250000" + "0" + "0" + "0" + "00" + "00" + "000" + "0" + "0" + "000000000" + "00" + "0" + "NAME    " + ";", true},
		{"wrong prefix", "MR", 50, "MW0007;", false},
		{"shorter than the prefix", "MR", 50, "M", false},
		{"the bare terminator a noisy line delivers", "MR", 50, ";", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PrefixLenMatcher(tt.prefix, tt.exactLen)([]byte(tt.frame)); got != tt.want {
				t.Errorf("matcher(%q) = %v, want %v (prefix %q, exactLen %d)", tt.frame, got, tt.want, tt.prefix, tt.exactLen)
			}
		})
	}
}

// TestPrefixLenMatcher_VariableLengthBranchIsGenuine pins the exactLen <= 0
// branch, which on the Yaesu side only MT uses and which here carries the ONE
// variable-length frame this milestone parses: the EX answer, whose P5 is
// declared "variable length" with no printed ceiling (590:555-556, 480:409-411).
func TestPrefixLenMatcher_VariableLengthBranchIsGenuine(t *testing.T) {
	m := PrefixLenMatcher("EX000000", 0)
	for _, frame := range []string{"EX000000" + "1" + ";", "EX000000" + "12" + ";", "EX000000" + "1.00" + ";"} {
		if !m([]byte(frame)) {
			t.Errorf("matcher(%q) = false, want true — the EX answer's P5 width is not fixed", frame)
		}
	}
	if m([]byte("EX00000")) {
		t.Error("matcher accepted a frame shorter than the prefix")
	}
}

// TestPrefixLenMatcher_FullAddressObligation is the negative-space proof the
// matcher's doc comment names, and it is the reason the EX prefix must carry
// the whole address.
//
// Every one of the TS-590SG's 100, the TS-590S's 88 and the TS-480's 61 menu
// addresses answers with a frame starting "EX". A bare "EX" prefix therefore
// correlates ANY EX answer as this read's answer — a different address's
// reply still in flight, say — and returns the wrong address's data silently.
// The pair below is the whole finding: the bare prefix accepts a foreign
// answer; the full-address prefix refuses it and still accepts our own.
func TestPrefixLenMatcher_FullAddressObligation(t *testing.T) {
	const ours = "EX000000" // menu 000, P2 "00", P3 '0'
	const foreign = "EX056000"

	bare := PrefixLenMatcher("EX", 0)
	if !bare([]byte(foreign + "3;")) {
		t.Fatal("the bare-prefix matcher did not accept a foreign address's answer — this test's premise has gone stale")
	}

	full := PrefixLenMatcher(ours, 0)
	if full([]byte(foreign + "3;")) {
		t.Error("the full-address matcher accepted address 056's answer while reading 000 — the prefix must carry the whole three-digit menu number")
	}
	if !full([]byte(ours + "5;")) {
		t.Error("the full-address matcher refused our own address's answer")
	}
}

// TestPrefixLenMatcher_DoesNotRetainTheFrame pins the CONTRACT ON frame: the
// matcher is handed the engine's own live receive buffer, so it may read it
// and must never retain it.
func TestPrefixLenMatcher_DoesNotRetainTheFrame(t *testing.T) {
	m := PrefixLenMatcher("ID", 6)
	frame := []byte("ID023;")
	if !m(frame) {
		t.Fatal("matcher refused a well-formed ID answer")
	}
	frame[2] = '9'
	if !m(frame) {
		t.Error("matcher's verdict changed after the caller mutated its own buffer — it is holding state it should not")
	}
}

// lateAnswer007 is a well-formed 50-byte MR ANSWER for memory 007 spelled
// the way the 590 pair spell an answer below channel 100: P2 a SPACE, which
// is MC's printed convention (590:1332-1337) inherited by MR under A10.
func lateAnswer007(t *testing.T) []byte {
	t.Helper()
	f := answer590()
	f.p2 = " "
	return f.frame(t)
}

// mrReadSpec is a ClassRead spec for one MR read, with no retry: the
// late-answer sequence needs the first read to time out ONCE and leave the
// port suspect, which is the state the engine's entry quarantine is for.
func mrReadSpec(match func(frame []byte) bool, timeout time.Duration) transport.CommandSpec {
	return transport.CommandSpec{
		Class:   transport.ClassRead,
		Match:   match,
		Timeout: timeout,
		Settle:  time.Millisecond,
	}
}

// TestMRAnswerMatcher_ALateAnswerIsNeverTheNextReadsAnswer is the
// transport-level proof of the finding, run through a scripted Port and the
// real engine rather than against the predicate alone.
//
// THE SEQUENCE IS THE ONE THE REVIEW NAMES, and every step of it is
// ordinary: a read of memory 007 times out; the engine quarantines the port
// and the driver moves on; a read of memory 008 goes out; and the radio's
// very late answer for 007 arrives while 008's read is waiting. Every
// Kenwood memory answer is 50 bytes and starts "MR", so a matcher keyed on
// the command name and the length alone accepts it — and ParseMRAnswer then
// returns a record whose Slot says 007 while the caller asked for 008. Only
// a driver that thought to compare the slot itself would notice, which is
// the undocumented obligation this matcher removes.
//
// THE ANSWER CARRIES THE SPACE SPELLING, deliberately: A10's response form
// is what most 590 answers below channel 100 actually look like, so a
// matcher that compared the P2 byte literally against the read's own '0'
// would reject every real answer. The comparison is of the SLOT the two
// spellings name, decoded through the same parseSlot the record parser uses.
//
// The delivery is made from inside the port's Write on the SECOND write, so
// the late frame is released strictly after the entry quarantine has
// returned and 008's read is on the wire — no sleep, and no chance of it
// being swallowed by the drain instead.
func TestMRAnswerMatcher_ALateAnswerIsNeverTheNextReadsAnswer(t *testing.T) {
	l := layout590SG()
	slot7, err := l.NewSlot(7, ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot(7): %v", err)
	}
	slot8, err := l.NewSlot(8, ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot(8): %v", err)
	}
	read7, err := l.BuildMRRead(slot7)
	if err != nil {
		t.Fatalf("BuildMRRead(007): %v", err)
	}
	read8, err := l.BuildMRRead(slot8)
	if err != nil {
		t.Fatalf("BuildMRRead(008): %v", err)
	}
	late := lateAnswer007(t)

	port := newTestPort()
	t.Cleanup(func() { _ = port.Close() })
	var writes atomic.Int32
	port.onWrite = func() {
		if writes.Add(1) == 2 {
			port.deliver(string(late))
		}
	}

	e, err := transport.NewEngineWith(port, kwFraming(t, Book590))
	if err != nil {
		t.Fatalf("NewEngineWith: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step one: 007's read times out with nothing on the line.
	if _, err := e.Do(ctx, read7, mrReadSpec(l.MRAnswerMatcher(slot7), 200*time.Millisecond)); !errors.Is(err, transport.ErrTimeout) {
		t.Fatalf("the read of 007 returned %v, want transport.ErrTimeout — this test's premise is that it times out and leaves the port suspect", err)
	}

	// Step two: 008's read goes out, and 007's answer arrives behind it.
	got, err := e.Do(ctx, read8, mrReadSpec(l.MRAnswerMatcher(slot8), 400*time.Millisecond))
	if err == nil {
		rec, perr := l.ParseMRAnswer(got)
		t.Fatalf("the read of memory 008 returned %q as its answer (ParseMRAnswer: slot %v, err %v) — that frame is memory 007's late reply, and correlating it here hands the caller one channel's record under another's number", got, rec.Slot, perr)
	}
	if got != nil {
		t.Errorf("the read of 008 returned frame %q alongside its error, want nil", got)
	}
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("the read of 008 returned %v, want transport.ErrTimeout — the late answer is not this read's answer, so nothing matched and the deadline is what ends the wait", err)
	}
	if n := e.UnexpectedFrames(); n == 0 {
		t.Error("the engine counted no unexpected frames — the late answer must be seen and rejected, not simply never delivered, or this test would pass on an empty line")
	}
}

// TestMRAnswerMatcher_CorrelatesTheSlotAndNotTheBytes is the predicate
// itself, below the engine.
//
// The three properties that make it more than a length test: the 590's
// SPACE spelling of the hundreds digit is the same slot as the '0' this
// codec sends (A10, 590:1332-1337); the hundreds digit is READ, so 107's
// answer is not 007's; and P1 is NOT compared, which is what leaves a
// driver's own start/end-half check (the section-defined channel of
// 590:1529-1531) something to catch rather than turning it into a timeout.
func TestMRAnswerMatcher_CorrelatesTheSlotAndNotTheBytes(t *testing.T) {
	sg := layout590SG()
	slot7, err := sg.NewSlot(7, ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot(7): %v", err)
	}
	scan107, err := sg.NewSlot(107, ScanLower)
	if err != nil {
		t.Fatalf("NewSlot(107): %v", err)
	}

	// frame renders an answer with the given P1, P2 and P3 over the
	// populated 590 fixture.
	frame := func(p1, p2, p3 string) []byte {
		f := answer590()
		f.p1, f.p2, f.p3 = p1, p2, p3
		return f.frame(t)
	}

	for _, tt := range []struct {
		name  string
		slot  Slot
		frame []byte
		want  bool
	}{
		{"007, the spelling this codec sends", slot7, frame("0", "0", "07"), true},
		{"007, the 590's answer spelling with a space (A10)", slot7, frame("0", " ", "07"), true},
		{"008's answer is not 007's", slot7, frame("0", "0", "08"), false},
		{"107's answer is not 007's — the hundreds digit is read", slot7, frame("0", "1", "07"), false},
		{"007's answer is not 107's", scan107, frame("0", "0", "07"), false},
		{"107, the start half", scan107, frame("0", "1", "07"), true},
		{"107, the END half still correlates — P1 is the driver's check, not the matcher's", scan107, frame("1", "1", "07"), true},
		{"a non-digit in P3 correlates with nothing", slot7, frame("0", "0", "0X"), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := sg.MRAnswerMatcher(tt.slot)(tt.frame); got != tt.want {
				t.Errorf("MRAnswerMatcher(%v)(%q) = %v, want %v", tt.slot, tt.frame, got, tt.want)
			}
		})
	}

	// The shape checks, which are PrefixLenMatcher's two and are kept
	// because this matcher does not call it: an MW Set is the same fifty
	// bytes under another name and is never an answer (erratum E17), and a
	// short frame is not one either.
	m := sg.MRAnswerMatcher(slot7)
	mw := append([]byte{}, frame("0", "0", "07")...)
	mw[0], mw[1] = 'M', 'W'
	if m(mw) {
		t.Error("the matcher correlated an MW Set as an MR answer; MR has no Set and MW has no Answer on either radio (480:911, erratum E17)")
	}
	if m([]byte("MR0007;")) {
		t.Error("the matcher correlated the host's own seven-byte MR READ as the radio's answer")
	}
	if m(append(append([]byte{}, frame("0", "0", "07")...), ';')) {
		t.Error("the matcher correlated a 51-byte frame; both books print one 50-byte grid")
	}

	// The zero layout has no slot space and no byte-4 policy, so it
	// correlates nothing at all — the same fail-closed property every other
	// method on it has.
	var zero Layout
	if zero.MRAnswerMatcher(slot7)(frame("0", "0", "07")) {
		t.Error("a zero Layout's matcher correlated a frame; it describes no radio and no slot space")
	}
}
