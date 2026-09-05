// SPDX-License-Identifier: GPL-3.0-or-later

package ts480

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestParseSlotID is the syntax rung, and its whole difference from the 590
// pair's namesake is the WIDTH: two digits, and no "L"/"U" suffix, because
// this row has no section-channel class for a half to name (§1.4.3).
func TestParseSlotID(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want int
		ok   bool
	}{
		{"00", 0, true},
		{"07", 7, true},
		{"42", 42, true},
		{"99", 99, true},
		// THE 590 PAIR'S OWN FORMS, refused here: three digits is that
		// family's printed width and "L"/"U" name a section channel's two
		// halves, neither of which this radio's MC prints (480:827,
		// 480:830).
		{"000", 0, false},
		{"042", 0, false},
		{"100L", 0, false},
		{"42U", 0, false},
		{"", 0, false},
		{"4", 0, false},
		{"4x", 0, false},
		{"-1", 0, false},
	} {
		got, err := parseSlotID(tc.id)
		if tc.ok {
			if err != nil {
				t.Errorf("parseSlotID(%q): %v", tc.id, err)
			} else if got != tc.want {
				t.Errorf("parseSlotID(%q) = %d, want %d", tc.id, got, tc.want)
			}
			continue
		}
		if err == nil {
			t.Errorf("parseSlotID(%q) = %d, want a refusal", tc.id, got)
		}
	}
}

// TestParseSlotID_IsTheInverseOfWhatTheBankPublishes: every published slot
// string parses, and parsing it back gives the number the inventory was built
// from. The two halves cannot drift because they are the same rendering.
func TestParseSlotID_IsTheInverseOfWhatTheBankPublishes(t *testing.T) {
	caps := CapabilitiesSimulated()
	for i, id := range caps.Banks[0].Slots {
		got, err := parseSlotID(id)
		if err != nil {
			t.Fatalf("the bank publishes %q, which parseSlotID refuses: %v", id, err)
		}
		if got != i {
			t.Errorf("parseSlotID(%q) = %d, want %d", id, got, i)
		}
	}
}

// TestReadChannel_APopulatedChannel is the read path end to end, and every
// assertion below is a field the TS-480's record spends DIFFERENTLY from the
// 590 pair's or does not spend at all.
func TestReadChannel_APopulatedChannel(t *testing.T) {
	sess, p := openSession(t, Simulated, radioImage{
		mrAnswers: map[string]string{mrAddr("42"): populatedMR("42")},
	})
	ch, err := sess.ReadChannel(context.Background(), "42")
	if err != nil {
		t.Fatalf("ReadChannel(\"42\"): %v", err)
	}
	if ch.Slot != "42" {
		t.Errorf("Slot = %q, want %q", ch.Slot, "42")
	}
	if ch.Data == nil {
		t.Fatal("Data is nil on a populated channel")
	}
	d := *ch.Data

	if d.FreqHz != 145_500_000 {
		t.Errorf("FreqHz = %d, want 145500000 (P4, 480:957)", d.FreqHz)
	}
	// EIGHT MODE NAMES AND NO FM-N: P5='4' is FM and P14 is a STEP INDEX
	// here (480:979), so nothing is synthesised from the pair.
	if d.Mode != "FM" {
		t.Errorf("Mode = %q, want %q — this row synthesises no FM-N", d.Mode, "FM")
	}
	if d.Tag != "SIMPLEX" {
		t.Errorf("Tag = %q, want %q (P16, 480:984, right-trimmed under A1)", d.Tag, "SIMPLEX")
	}
	// SCAN SKIP COMES FROM BYTE 19, NOT BYTE 41. "Lockout status. 0:
	// Lockout OFF, 1: Lockout ON" (480:962) — the 590 pair carry their
	// lockout at byte 41 and this radio prints a constant there (480:982).
	if d.ScanSkip.State != codeplug.Known || d.ScanSkip.Value {
		t.Errorf("ScanSkip = %+v, want Known/false from BYTE 19 (480:962)", d.ScanSkip)
	}
	if d.ToneMode.State != codeplug.Known || d.ToneMode.Value != "TONE" {
		t.Errorf("ToneMode = %+v, want Known \"TONE\" (P7, 480:964)", d.ToneMode)
	}
	// THE TWO TONE INDICES ARE READ AND NOT PUBLISHED. The record carries
	// them; this book carries neither chart (480:1559-1560, 480:339-340),
	// so there is no hertz value to report and the neutral model has no
	// opaque-index field. Unavailable, matching the capability grade.
	if d.ToneTx.State != codeplug.Unavailable || d.ToneRx.State != codeplug.Unavailable {
		t.Errorf("ToneTx/ToneRx = %+v/%+v, want Unavailable on both — §1.9's charts are not in this book", d.ToneTx, d.ToneRx)
	}
	// DATA MODE IS UNAVAILABLE, because byte 19 is spent on the lockout.
	if d.DataMode.State != codeplug.Unavailable {
		t.Errorf("DataMode = %+v, want Unavailable — byte 19 is the LOCKOUT on this row (480:962)", d.DataMode)
	}
	if d.Filter.State != codeplug.Unavailable {
		t.Errorf("Filter = %+v, want Unavailable — byte 28 is \"Always 0 for the TS-480.\" (480:973)", d.Filter)
	}
	if d.TuningStep.State != codeplug.Unavailable || d.TuningStepEnabled.State != codeplug.Unavailable {
		t.Errorf("TuningStep = %+v — bytes 39-40 ARE a step here and the field is REFUSED (A22), which is Unavailable in the neutral model and Unsupported in the table", d.TuningStep)
	}
	// TX FREQUENCY: Unsupported on the whole row (M-E2), so Unavailable
	// here and never a value.
	if d.TxFreqHz.State != codeplug.Unavailable {
		t.Errorf("TxFreqHz = %+v, want Unavailable — the field is Unsupported on the WHOLE row (M-E2)", d.TxFreqHz)
	}
	if d.ClarHz != 0 || d.RxClar || d.TxClar {
		t.Error("a clarifier value was published; the record has no clarifier position over this book's own complete account (480:951-984, M-E5)")
	}

	// ONE FRAME PER SLOT, P1=0, P2=0.
	if got := p.Transcript(); !reflect.DeepEqual(got, append(append([]string{}, probeFrames...), "MR0042;")) {
		t.Errorf("transcript = %v, want the probe's three frames and one MR0042;", got)
	}

	// The fleet's fresh-read rule, as a BLACK BOX (plan P5).
	drivertest.AssertFreshReadSaveLoad(t, ch, sess.Capabilities(), codeplug.Load)
}

// TestReadChannel_TheLockoutIsByte19 is the byte-19 divergence pinned BOTH
// ways, because it is "the one divergence no roadmap line records" (§5) and a
// driver that read byte 41 instead would be green on a fixture where both are
// '0'.
//
// BYTE 41 IS A PRINTED CONSTANT HERE (480:982) and is required on parse
// (A24), so the mirror half cannot be tested by setting byte 41 to '1' — that
// frame is refused. The discriminating pin is that byte 19 '1' reaches
// scan_skip, which a byte-41 reader would report as false.
func TestReadChannel_TheLockoutIsByte19(t *testing.T) {
	f := populatedFields("07")
	f.b19 = '1'
	sess, _ := openSession(t, Simulated, radioImage{
		mrAnswers: map[string]string{mrAddr("07"): f.frame()},
	})
	ch, err := sess.ReadChannel(context.Background(), "07")
	if err != nil {
		t.Fatalf("ReadChannel(\"07\"): %v", err)
	}
	if ch.Data.ScanSkip.State != codeplug.Known || !ch.Data.ScanSkip.Value {
		t.Errorf("ScanSkip = %+v with byte 19 = '1', want Known/true — a driver reading byte 41 would report false (480:962 against 480:982)", ch.Data.ScanSkip)
	}
	if ch.Data.DataMode.State != codeplug.Unavailable {
		t.Errorf("DataMode = %+v — byte 19 must not ALSO be published as a data mode", ch.Data.DataMode)
	}
}

// TestReadChannel_EveryPublishedModeIsNamed walks the eight nibbles this row's
// legend carries and asserts each reaches the neutral model under the name the
// capability table advertises — so the two cannot drift.
//
// Nibbles 0 and 8 are absent from that legend: "0: No mode (Not used for the
// TS-480)" (480:843) and "8: Tune (Not used for the TS-480)" (480:853). A
// frame carrying either, with a non-zero P4-P15 window, is refused.
func TestReadChannel_EveryPublishedModeIsNamed(t *testing.T) {
	names := CapabilitiesSimulated().Modes
	wire := map[string]byte{"LSB": '1', "USB": '2', "CW": '3', "FM": '4', "AM": '5', "FSK": '6', "CW-R": '7', "FSK-R": '9'}
	if len(names) != len(wire) {
		t.Fatalf("Modes has %d names, this table knows %d", len(names), len(wire))
	}
	for _, name := range names {
		nibble, ok := wire[name]
		if !ok {
			t.Fatalf("Modes publishes %q, which the MD legend at 480:843-854 does not name", name)
		}
		f := populatedFields("11")
		f.mode = nibble
		sess, _ := openSession(t, Simulated, radioImage{
			mrAnswers: map[string]string{mrAddr("11"): f.frame()},
		})
		ch, err := sess.ReadChannel(context.Background(), "11")
		if err != nil {
			t.Fatalf("ReadChannel with P5 %q: %v", nibble, err)
		}
		if ch.Data.Mode != name {
			t.Errorf("P5 %q read as %q, want %q", nibble, ch.Data.Mode, name)
		}
	}

	// The two nibbles the legend does not name, in a NON-empty record.
	for _, nibble := range []byte{'0', '8'} {
		f := populatedFields("11")
		f.mode = nibble
		sess, _ := openSession(t, Simulated, radioImage{
			mrAnswers: map[string]string{mrAddr("11"): f.frame()},
		})
		if _, err := sess.ReadChannel(context.Background(), "11"); !errors.Is(err, kw.ErrParse) {
			t.Errorf("P5 %q: err = %v, want a parse refusal — the MD legend names no mode there (480:843, 480:853)", nibble, err)
		}
	}
}

// TestReadChannel_TheSixteenPrintedFixedBytesAreRequiredOnParse is A24, and
// the CHOICE is named at the site rather than assumed.
//
// The book's own general note permits a SET to fill an inapplicable parameter
// with "any character except the ASCII control codes (00 to 1Fh) and the
// terminator (;)" (480:108-111), so a radio answering a hard-wired byte with
// something else would not necessarily be faulty. This programme requires the
// printed constant on PARSE anyway — the strict direction — and A24 is the
// register entry that records the choice, with its lift a dozen real reads of
// a real TS-480 (L-HW-18). A single counter-example turns the rule from
// "required" into "accepted and normalised".
//
// SIXTEEN BYTES IN SIX RUNS, against the 590 pair's thirteen in three: this
// row adds its own P2 (480:953), P11 (480:973) and P15 (480:982) to the three
// both books print.
func TestReadChannel_TheSixteenPrintedFixedBytesAreRequiredOnParse(t *testing.T) {
	fixed := layout().PrintedFixed()
	total := 0
	for _, ff := range fixed {
		total += len(ff.Printed)
	}
	if len(fixed) != 6 || total != 16 {
		t.Fatalf("the layout declares %d runs over %d bytes, want 6 runs over 16 (§5)", len(fixed), total)
	}

	base := populatedMR("42")
	for _, ff := range fixed {
		spoiled := []byte(base)
		// '9' is a printable non-control byte the general note would
		// permit on a Set, which is exactly what makes the refusal a
		// CHOICE rather than an envelope rule.
		spoiled[ff.Pos-1] = '9'
		sess, _ := openSession(t, Simulated, radioImage{
			mrAnswers: map[string]string{mrAddr("42"): string(spoiled)},
		})
		_, err := sess.ReadChannel(context.Background(), "42")
		if !errors.Is(err, kw.ErrParse) {
			t.Errorf("position %d spoiled: err = %v, want a parse refusal (A24)", ff.Pos, err)
		}
		if err != nil && !strings.Contains(err.Error(), "A24") {
			t.Errorf("position %d spoiled: err = %v, want the refusal to name A24 — on this row the strictness is a choice, not a deduction", ff.Pos, err)
		}
	}
}

// TestReadChannel_ASpaceInByteFourIsCORRELATEDANDTHENREFUSED is the
// consequence the T13 fix wave's space-P2 concern lands on, pinned at driver
// level because this is the row where P2 is P2FixedZero.
//
// THE TWO HALVES DISAGREE ON PURPOSE. kw.Layout.MRAnswerMatcher correlates an
// answer by decoding its slot through parseSlot, and parseSlot's P2FixedZero
// arm does not look at byte 4 at all — it takes the hundreds digit as 0 by
// construction — so an answer spelling P2 as the 590 pair's SPACE is
// correlated as this read's own. ParseMRAnswer then refuses it, because
// checkPrintedFixed runs FIRST and requires the '0' this book prints
// (480:953).
//
// THE OUTCOME A USER SEES IS THEREFORE A NAMED PARSE REFUSAL, NEVER A TIMEOUT,
// and that is the right side to err on: the frame really was the answer to
// this read, so reporting it as "no answer" would hide a radio disagreeing
// with its own book behind a symptom that looks like a dead cable.
func TestReadChannel_ASpaceInByteFourIsCORRELATEDANDTHENREFUSED(t *testing.T) {
	f := populatedFields("42")
	f.byte4 = ' '
	frame := f.frame()

	// Half one: the matcher DOES correlate it.
	slot, err := layout().NewSlot(42, kw.ScanHalfNone)
	if err != nil {
		t.Fatalf("NewSlot: %v", err)
	}
	if !layout().MRAnswerMatcher(slot)([]byte(frame)) {
		t.Fatal("MRAnswerMatcher refused a space-P2 answer; this test's whole point is that it does NOT, so the parser is what refuses")
	}

	// Half two: the driver refuses it, naming the line that prints the
	// constant, and does NOT time out.
	sess, _ := openSession(t, Simulated, radioImage{
		mrAnswers: map[string]string{mrAddr("42"): frame},
	})
	_, err = sess.ReadChannel(context.Background(), "42")
	if !errors.Is(err, kw.ErrParse) {
		t.Fatalf("err = %v, want a parse refusal", err)
	}
	if errors.Is(err, transport.ErrTimeout) {
		t.Error("a correlated answer was reported as a timeout; the frame reached the parser and the parser is what refused it")
	}
	// THE MESSAGE NAMES THE POSITION, THE PRINTED VALUE AND A24 — and NOT
	// the line 480:953, which is where core/kw/ts480/layout.go's own
	// PrintedFixed entry carries it. core/kw's checkPrintedFixed builds one
	// message for six runs on this row and thirteen bytes on the 590 pair's,
	// and kw.FixedField has no citation member for it to quote; adding one
	// would be a core/kw change, outside this task's files. What the refusal
	// DOES carry is enough to find the line: the position, the byte the book
	// prints there, and A24 by name.
	for _, want := range []string{"positions 4-4", `"0"`, "A24"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want the message to name %s", err, want)
		}
	}
}

// TestReadChannel_MRIsNeverSentWithP1One is the plan's own bullet, and it is
// a REAL LOSS published rather than a tidy invariant: "Memory channel 90 ~ 99:
// P1=0 (start frequency), P1=1 (end frequency)" (480:943-944, 480:986-987) is
// genuinely printed, and this programme never sends the second frame — so the
// end frequency of channels 90-99 is unreachable through it.
//
// The channels themselves are NOT lost: 90-99 are ordinary memories and are
// read like any other (§1.4.3, decision 15).
func TestReadChannel_MRIsNeverSentWithP1One(t *testing.T) {
	answers := map[string]string{}
	for n := 90; n <= 99; n++ {
		id := string([]byte{byte('0' + n/10), byte('0' + n%10)})
		answers[mrAddr(id)] = populatedMR(id)
	}
	sess, p := openSession(t, Simulated, radioImage{mrAnswers: answers})
	for n := 90; n <= 99; n++ {
		id := string([]byte{byte('0' + n/10), byte('0' + n%10)})
		if _, err := sess.ReadChannel(context.Background(), id); err != nil {
			t.Fatalf("ReadChannel(%q): %v — 90-99 are ordinary memories on this row", id, err)
		}
	}
	for _, frame := range p.Transcript() {
		if strings.HasPrefix(frame, "MR") && frame[2] != '0' {
			t.Errorf("the driver sent %q; P1 is derived from the slot's class and this row has only SlotMemory, so P1=1 is never addressed", frame)
		}
	}
}

// TestReadChannel_AnEmptyChannelIsNotAFailure — and the claim is carefully
// bounded, because on THIS row it rests on A4, which is unlifted and is this
// row's registration gate (L-HW-3, §3.13).
//
// WHAT THIS PINS: a frame whose P4-P15 are all zero and whose P16 is blank is
// read as an EMPTY CHANNEL — codeplug.Channel{Slot: id} with a nil Data —
// rather than as a parse failure. That is a property of the codec and of this
// driver, and it is true whatever a radio does.
//
// WHAT IT DOES NOT PIN: that a TS-480 ever sends such a frame. The 590 pair's
// rule is documentary fact (590:1492-1493); this book prints nothing about an
// empty channel anywhere, so if A4 is false — if an MR of an unwritten channel
// answers "?;" — decision 5 makes that a definitive rejection and a fresh
// TS-480 cannot be read at all. That is why the row is built and NOT
// registered.
func TestReadChannel_AnEmptyChannelIsNotAFailure(t *testing.T) {
	sess, _ := openSession(t, Simulated, radioImage{
		mrAnswers: map[string]string{mrAddr("00"): emptyMR("00")},
	})
	ch, err := sess.ReadChannel(context.Background(), "00")
	if err != nil {
		t.Fatalf("ReadChannel(\"00\") on an empty channel: %v", err)
	}
	if ch.Slot != "00" {
		t.Errorf("Slot = %q, want %q", ch.Slot, "00")
	}
	if ch.Data != nil {
		t.Errorf("Data = %+v on an empty channel, want nil — the seam's own spelling of \"this slot is empty\"", ch.Data)
	}
}

// TestReadChannel_ARejectionIsDefinitiveAndNeverAbsence: a "?;" fails the
// session read WHOLE, with the typed error naming the two indistinguishable
// causes this book prints (480:130-135) and the transient sentence
// (480:136-138). It is never read as "the channel is empty".
func TestReadChannel_ARejectionIsDefinitiveAndNeverAbsence(t *testing.T) {
	// An address with no entry in the image is answered "?;".
	sess, p := openSession(t, Simulated, radioImage{})
	ch, err := sess.ReadChannel(context.Background(), "42")
	if err == nil {
		t.Fatalf("a \"?;\" was read as %+v, want a refusal", ch)
	}
	var rej *kw.RejectionError
	if !errors.As(err, &rej) {
		t.Fatalf("err = %v (%T), want *kw.RejectionError", err, err)
	}
	if rej.Book != kw.Book480 {
		t.Errorf("the rejection quotes %v, want the 480's own book — E13: the two books give DIFFERENT causes for the same token", rej.Book)
	}
	if rej.Command != "MR" {
		t.Errorf("the rejection names command %q, want \"MR\"", rej.Command)
	}
	if !strings.Contains(err.Error(), "480:136-138") {
		t.Errorf("err = %v, want the transient-suppression sentence cited from THIS book", err)
	}
	// ONE FRAME: a rejection is definitive and is never retried.
	if got := p.Transcript(); len(got) != 4 {
		t.Errorf("transcript = %v, want the probe's three frames and exactly one MR", got)
	}
}

// TestReadChannel_ATimeoutIsNotAnInferenceOfAbsence: silence fails the read
// whole and is typed, because the book itself says the NAK is unreliable, so
// silence carries no information at all.
func TestReadChannel_ATimeoutIsNotAnInferenceOfAbsence(t *testing.T) {
	sess, p := openSession(t, Simulated, radioImage{
		mrSilent: map[string]bool{mrAddr("42"): true},
	})
	ch, err := sess.ReadChannel(context.Background(), "42")
	if err == nil {
		t.Fatalf("silence was read as %+v, want a refusal", ch)
	}
	var to *kw.TimeoutError
	if !errors.As(err, &to) {
		t.Fatalf("err = %v (%T), want *kw.TimeoutError", err, err)
	}
	if to.Book != kw.Book480 || to.Command != "MR" {
		t.Errorf("timeout = %v/%q, want the 480's book and \"MR\"", to.Book, to.Command)
	}
	// ONE RETRY, and no more: a read is idempotent and a single swallowed
	// reply should not fail a whole-radio read of a hundred slots.
	if got := p.Transcript(); len(got) != 5 {
		t.Errorf("transcript = %v, want the probe's three frames and TWO MR attempts", got)
	}
}

// TestReadChannel_AnUnknownSlotIsRefusedBeforeAnyFrame — including the 590
// pair's own three-digit spelling, which is the near miss a reader moving
// between the two packages will actually make.
func TestReadChannel_AnUnknownSlotIsRefusedBeforeAnyFrame(t *testing.T) {
	for _, id := range []string{"042", "100L", "AB", "999", "9"} {
		sess, p := openSession(t, Simulated, radioImage{})
		_, err := sess.ReadChannel(context.Background(), id)
		if !errors.Is(err, ErrUnknownSlot) {
			t.Errorf("ReadChannel(%q): err = %v, want ErrUnknownSlot", id, err)
		}
		var unknown *UnknownSlotError
		if !errors.As(err, &unknown) {
			t.Errorf("ReadChannel(%q): err is not an *UnknownSlotError", id)
		} else if unknown.Model != modelName {
			t.Errorf("ReadChannel(%q): the refusal names %q", id, unknown.Model)
		}
		if got := p.Transcript(); !reflect.DeepEqual(got, probeFrames) {
			t.Errorf("ReadChannel(%q) sent %v; an unknown slot is refused before any frame", id, got)
		}
	}
}

// TestReadChannel_AnAnswerNamingAnotherSlotIsRefused: the driver refuses to
// map a reply onto the wrong slot rather than storing one channel's content
// under another's identifier.
func TestReadChannel_AnAnswerNamingAnotherSlotIsRefused(t *testing.T) {
	sess, _ := openSession(t, Simulated, radioImage{
		mrAnswers: map[string]string{mrAddr("42"): populatedMR("43")},
	})
	_, err := sess.ReadChannel(context.Background(), "42")
	// The matcher correlates on the slot NUMBER, so 43's answer is not
	// 42's answer at all and is never delivered: the read times out.
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("err = %v, want a timeout — MRAnswerMatcher must skip another channel's answer rather than delivering it", err)
	}
	var mismatch *AnswerMismatchError
	if errors.As(err, &mismatch) {
		t.Errorf("the foreign frame reached the parser and was refused as %v; correlation must happen first", mismatch)
	}
}

// TestReadChannel_AnAnswerWhoseP1DisagreesIsRefused is the guard the T11
// review's M1 added on the 590 pair, one row over — and here it is the ONLY
// form the check can take, because this row's slot identifier carries no half
// at all.
//
// P1='1' on this radio is the TX frequency of a split channel (480:951) and,
// on channels 90-99, the section END frequency (480:986-987). Accepting one
// for the P1='0' read this driver sends would store the wrong frequency as the
// channel's own, with tx_frequency left Unavailable — silent loss that
// codeplug.Validate cannot see.
func TestReadChannel_AnAnswerWhoseP1DisagreesIsRefused(t *testing.T) {
	for _, id := range []string{"42", "95"} {
		f := populatedFields(id)
		f.p1 = '1'
		sess, _ := openSession(t, Simulated, radioImage{
			mrAnswers: map[string]string{mrAddr(id): f.frame()},
		})
		_, err := sess.ReadChannel(context.Background(), id)
		var p1err *AnswerP1MismatchError
		if !errors.As(err, &p1err) {
			t.Fatalf("slot %q: err = %v (%T), want *AnswerP1MismatchError", id, err, err)
		}
		if p1err.Requested != '0' || p1err.Answered != '1' {
			t.Errorf("slot %q: Requested/Answered = %q/%q, want '0'/'1'", id, p1err.Requested, p1err.Answered)
		}
		if !errors.Is(err, ErrAnswerMismatch) {
			t.Errorf("slot %q: the P1 refusal does not match ErrAnswerMismatch", id)
		}
	}
}

// TestReadChannel_AShortMRAnswerNeverReachesTheParser is L5's substance on
// this row, and it is TWO facts.
//
// FIRST, THE CORRELATION FACT: kw.Layout.MRAnswerMatcher requires exactly
// kw.RecordLen bytes, so a 49-byte "MR…" is not this read's answer at all —
// it is never delivered, the read times out, and the parser is never given the
// chance to interpret a frame the command did not ask for.
//
// SECOND, THE WIDTH PREDICATE ITSELF, asserted through the fleet-internal
// Kenwood helper this task lands (drivertest.AssertKenwoodRecordLengthMismatch)
// — the same helper core/driver/ts590 now calls, which is the whole point of
// hoisting it out of that package.
func TestReadChannel_AShortMRAnswerNeverReachesTheParser(t *testing.T) {
	short := populatedMR("42")[:kw.RecordLen-2] + ";"
	sess, p := openSession(t, Simulated, radioImage{
		mrAnswers: map[string]string{mrAddr("42"): short},
	})
	_, err := sess.ReadChannel(context.Background(), "42")
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("a 49-byte MR answer: err = %v, want a timeout — an exact-width matcher must not correlate it", err)
	}
	if errors.Is(err, kw.ErrParse) {
		t.Errorf("a 49-byte MR answer reached the parser: %v", err)
	}
	if got := p.Transcript(); len(got) < 4 || got[3] != "MR0042;" {
		t.Errorf("transcript = %v, want the MR read to have gone out", got)
	}

	// The codec's own width predicate, on the same bytes.
	_, perr := layout().ParseMRAnswer([]byte(short))
	drivertest.AssertKenwoodRecordLengthMismatch(t, perr, "MR", kw.RecordLen-1, kw.RecordLen)
}

// TestReadChannel_IsAtomicUnderOpMu is P13/P14's concurrency pin, made
// deterministic by a test-only hook rather than by hammering: Go's sync.Mutex
// favours an immediately-re-locking goroutine so heavily that the interleaving
// opMu forbids is near-impossible to reproduce otherwise.
//
// The hook runs with opMu ALREADY HELD, so entering it is proof that a
// goroutine got past the lock.
func TestReadChannel_IsAtomicUnderOpMu(t *testing.T) {
	sess, p := openSession(t, Simulated, radioImage{mrAnswers: map[string]string{
		mrAddr("01"): populatedMR("01"),
		mrAddr("02"): populatedMR("02"),
	}})

	entered := make(chan struct{}, 4)
	release := make(chan struct{})
	var parkOnce sync.Once
	readChannelGapHook = func() {
		select {
		case entered <- struct{}{}:
		default:
		}
		parkOnce.Do(func() { <-release })
	}
	t.Cleanup(func() { readChannelGapHook = nil })

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := sess.ReadChannel(context.Background(), "01"); err != nil {
			t.Errorf("parked ReadChannel: %v", err)
		}
	}()
	<-entered // the first operation is inside the lock and parked

	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := sess.ReadChannel(context.Background(), "02"); err != nil {
			t.Errorf("second ReadChannel: %v", err)
		}
	}()

	select {
	case <-entered:
		t.Error("a second operation entered ReadChannel while the first held opMu")
	case <-time.After(150 * time.Millisecond):
	}
	if got := p.Transcript(); len(got) != len(probeFrames) {
		t.Errorf("transcript = %v while one operation is parked inside the lock, want the probe's frames alone", got)
	}
	close(release)
	wg.Wait()
}

// TestReadAll_FailsWholeOnTheFirstRefusal proves the walk stops ON THE WIRE
// and not merely in the return value: a partial read the user could not tell
// from a complete one is the failure this ordering exists to prevent.
func TestReadAll_FailsWholeOnTheFirstRefusal(t *testing.T) {
	answers := map[string]string{}
	for n := 0; n <= 99; n++ {
		id := string([]byte{byte('0' + n/10), byte('0' + n%10)})
		if n == 2 {
			continue // answered "?;" by the image's default
		}
		answers[mrAddr(id)] = populatedMR(id)
	}
	sess, p := openSession(t, Simulated, radioImage{mrAnswers: answers})
	for _, id := range CapabilitiesSimulated().Banks[0].Slots {
		_, err := sess.ReadChannel(context.Background(), id)
		if id == "02" {
			if err == nil {
				t.Fatal("the rejected slot returned no error")
			}
			break
		}
		if err != nil {
			t.Fatalf("ReadChannel(%q): %v", id, err)
		}
	}
	for _, frame := range p.Transcript() {
		if frame == "MR0003;" {
			t.Error("the walk continued past the refusal; a session read fails WHOLE")
		}
	}
}
