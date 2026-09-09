// SPDX-License-Identifier: GPL-3.0-or-later

package ts890

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// probeFrames is what every session in this package has already sent by the
// time a test's own first frame goes out (plan P9): the engine's own AI0;
// followed by the two-frame identity probe.
var probeFrames = []string{"AI0;", "ID;", "FV;"}

// simplexChannel is the ordinary candidate this file writes: a populated
// memory channel with no split side, every field the record expresses Known,
// and nothing at its zero value that a mapping bug could hide behind.
//
// tx_frequency IS KNOWN ZERO AND THAT IS THE SIMPLEX STATE ITSELF, not a
// missing value: the book prints "When reading a single memory channel, all
// parameters for Split Transmission become 0" (890:3217-3218, A16), so a
// Known zero is the radio's own word for "this channel has no separate
// transmit frequency" and is exactly what read.go publishes for one.
func simplexChannel(slot string) codeplug.Channel {
	return codeplug.Channel{Slot: slot, Data: &codeplug.ChannelData{
		FreqHz:    14_250_000,
		Mode:      "USB",
		Tag:       "GB3IV",
		ScanSkip:  codeplug.BoolField{State: codeplug.Known, Value: false},
		ToneMode:  codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		ToneTx:    codeplug.ToneField{State: codeplug.Known, Value: 670},
		ToneRx:    codeplug.ToneField{State: codeplug.Known, Value: 670},
		TxFreqHz:  codeplug.FreqField{State: codeplug.Known, Value: 0},
		CTCSSTone: codeplug.ToneField{State: codeplug.Unavailable},
	}}
}

// simplexSetFrame is simplexChannel("007")'s MA0 Set, HAND-DERIVED from the
// book's own position ruler rather than built through the codec: opcode and
// channel at 1-6, P2's eleven frequency digits at 7-17, P3 the mode byte '2'
// (USB) at 18, P4 '0' normal at 19, P5 '0' OFF at 20, P6 "00" and P7 "00" at
// 21-24, P8's eleven zeros at 25-35, P9 '0' and P10 '0' at 36-37, P11 '0'
// simplex at 38, P12 '0' lockout off at 39, then the name and the FLOATING
// terminator at 40 + len(name) (890:3166-3182).
//
// HAND-DERIVED IS THE WHOLE POINT: every other fixture in this package is
// rendered through core/kw/ma's own builder, which cannot catch a driver that
// fills the right positions with the wrong values.
const simplexSetFrame = "MA0007000142500002000000000000000000000GB3IV;"

// blankTargetImage answers every MA0 read with the blank-channel frame — the
// fresh radio, and rung 10's fixture.
func blankTargetImage(slot int) radioImage {
	return radioImage{ma0Answers: map[string]string{
		slotID(slot): blankAnswer890(slot, ""),
	}}
}

// occupiedSimplex is the RADIO-SIDE image every simplexChannel positive
// control writes into: read_test.go's populated record with its split side
// returned to the printed zeroed form (890:3217-3218).
//
// IT HAS TO MATCH THE CANDIDATE'S SECONDARY SIDE, and that is rung 11 working
// rather than a fixture convenience: a simplex candidate written over a
// SPLIT slot is refused, because the Set would flatten a transmit mode the
// neutral model never carried.
func occupiedSimplex() ma.Record {
	rec := populated()
	rec.TXFreqHz, rec.TXMode, rec.TXFMNarrow, rec.Split = 0, 0, false, false
	return rec
}

// populatedImage answers slot's MA0 read with rec, which is what the write
// ladder's ONE read meets.
func populatedImage(t *testing.T, slot int, rec ma.Record) radioImage {
	t.Helper()
	return radioImage{ma0Answers: map[string]string{
		slotID(slot): answerFor(t, slot, rec),
	}}
}

// slotID renders a channel number as this row's three-digit identifier.
func slotID(n int) string {
	return string([]byte{byte('0' + n/100), byte('0' + n/10%10), byte('0' + n%10)})
}

// TestMA0SetSpec_IsFireAndForgetAndNeverRetries pins all three properties of
// the ONE mutating frame's spec: write class, no answer matcher, and no
// retry — a write is never resent (transport safety obligation 2), and
// Engine.Do refuses a write-class spec with a non-zero RetryReads before
// writing anything.
func TestMA0SetSpec_IsFireAndForgetAndNeverRetries(t *testing.T) {
	got := ma0SetSpec()
	if got.Class != transport.ClassWrite {
		t.Errorf("Class = %v, want transport.ClassWrite", got.Class)
	}
	if got.Match != nil {
		t.Error("Match is non-nil: the Set is fire-and-forget and correlates no answer")
	}
	if got.RetryReads != 0 {
		t.Errorf("RetryReads = %d, want 0 — a write is never resent", got.RetryReads)
	}
}

// TestModeWire_IsTheExactInverseOfModeName walks EVERY published mode name
// back through modeWire and forward again through modeName.
//
// A HAND-WRITTEN INVERSE OF A HAND-WRITTEN MAP IS ONE EDIT FROM DISAGREEING,
// so modeWire derives its answer from the layout's own legend. This test is
// what says so in both directions, and it is also where the two-byte shape
// bites: "FM" and "FM-N" are ONE legend value with two P4 bytes, and every
// non-FM name must come back with P4 NORMAL — publishing "USB" with the
// narrow flag set would put a byte on the wire whose meaning this book never
// prints outside FM.
func TestModeWire_IsTheExactInverseOfModeName(t *testing.T) {
	l := layout()
	names := modeNames(l)
	if len(names) == 0 {
		t.Fatal("modeNames returned nothing")
	}
	for _, name := range names {
		b, narrow, ok := modeWire(l, name)
		if !ok {
			t.Errorf("modeWire(%q) = not ok, but the row publishes it", name)
			continue
		}
		back, ok := modeName(l, b, narrow)
		if !ok || back != name {
			t.Errorf("modeName(modeWire(%q)) = %q (ok=%v), want %q", name, back, ok, name)
		}
		if narrow && !strings.HasPrefix(name, "FM") {
			t.Errorf("modeWire(%q) set the FM narrow flag on a non-FM mode", name)
		}
	}
	// '0' and '8' are printed "Unused" (890:3977, 890:3985), the layout
	// carries no name for either, and so no candidate channel can reach
	// them: rung 7 refuses the NAME, and the byte is unreachable by
	// construction.
	if _, _, ok := modeWire(l, "Unused"); ok {
		t.Error(`modeWire("Unused") = ok: the legend's two "Unused" values must be unreachable from a name`)
	}
}

// TestToneModeWire_IsTheExactInverseOfToneModeNames is the same rule for P5's
// four values, inverted at package init from read.go's forward map so the
// read path and the write path cannot name the byte differently.
func TestToneModeWire_IsTheExactInverseOfToneModeNames(t *testing.T) {
	if len(toneModeWire) != len(toneModeNames) {
		t.Fatalf("toneModeWire has %d entries, toneModeNames has %d", len(toneModeWire), len(toneModeNames))
	}
	for wire, name := range toneModeNames {
		if got, ok := toneModeWire[name]; !ok || got != wire {
			t.Errorf("toneModeWire[%q] = %q (ok=%v), want %q", name, got, ok, wire)
		}
	}
}

// TestRequestedFields_MembershipAndOrder pins the table against
// spec.AllFields() — twenty-six entries, every spec.Field but FieldErase, in
// declaration order (plan P5 permits naming the helper on these rows).
//
// FieldErase is not a field a write REQUESTS: it is the whole shape of a
// different frame, the printed MA5 this milestone never builds, and rung 3
// refuses an empty channel a rung above this table.
func TestRequestedFields_MembershipAndOrder(t *testing.T) {
	var want []spec.Field
	for _, f := range spec.AllFields() {
		if f == spec.FieldErase {
			continue
		}
		want = append(want, f)
	}
	var got []spec.Field
	for _, r := range requestedFieldRules {
		got = append(got, r.field)
	}
	if !slices.Equal(got, want) {
		t.Errorf("requestedFieldRules = %v,\nwant %v", got, want)
	}
}

// TestRequestedFields_TheEightTheRecordAlwaysCarries pins the eight the
// 40-to-50-byte record transmits on every write, changed or not: there is no
// "leave it alone" encoding anywhere in this grid, so a write requests all
// eight whether the caller edited them or not — which is what makes the
// capability gate a real gate.
func TestRequestedFields_TheEightTheRecordAlwaysCarries(t *testing.T) {
	got := requestedFields(codeplug.ChannelData{})
	want := []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldTag, spec.FieldScanSkip,
		spec.FieldTxFrequency, spec.FieldToneMode, spec.FieldToneTx, spec.FieldToneRx,
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("an empty ChannelData requests %v, want the eight the record always carries: %v", got, want)
	}
}

// TestRequestedFields_EveryConditionalIsReachable proves that each of the
// eighteen conditional rules can actually fire. A conditional nothing can
// trip is a rule that silently drops the value it was written to refuse.
func TestRequestedFields_EveryConditionalIsReachable(t *testing.T) {
	base := len(requestedFields(codeplug.ChannelData{}))
	for _, tc := range []struct {
		field spec.Field
		data  codeplug.ChannelData
	}{
		{spec.FieldClarifier, codeplug.ChannelData{ClarHz: 100}},
		{spec.FieldClarifier, codeplug.ChannelData{RxClar: true}},
		{spec.FieldClarifier, codeplug.ChannelData{TxClar: true}},
		{spec.FieldCTCSSState, codeplug.ChannelData{CTCSS: "on"}},
		{spec.FieldCTCSSTone, codeplug.ChannelData{CTCSSTone: codeplug.ToneField{State: codeplug.Known, Value: 670}}},
		{spec.FieldShift, codeplug.ChannelData{Shift: "up"}},
		{spec.FieldTagDisplay, codeplug.ChannelData{TagDisplay: codeplug.BoolField{State: codeplug.Known}}},
		{spec.FieldDuplex, codeplug.ChannelData{Duplex: codeplug.StringField{State: codeplug.Known, Value: "off"}}},
		{spec.FieldOffset, codeplug.ChannelData{OffsetHz: codeplug.FreqField{State: codeplug.Known, Value: 600_000}}},
		{spec.FieldDTCSCode, codeplug.ChannelData{DTCSCode: codeplug.IntField{State: codeplug.Known, Value: 23}}},
		{spec.FieldDTCSPolarity, codeplug.ChannelData{DTCSPolarity: codeplug.StringField{State: codeplug.Known, Value: "NN"}}},
		{spec.FieldFilter, codeplug.ChannelData{Filter: codeplug.StringField{State: codeplug.Known, Value: "A"}}},
		{spec.FieldDataMode, codeplug.ChannelData{DataMode: codeplug.BoolField{State: codeplug.Known, Value: true}}},
		{spec.FieldTuningStepEnabled, codeplug.ChannelData{TuningStepEnabled: codeplug.BoolField{State: codeplug.Known, Value: true}}},
		{spec.FieldTuningStep, codeplug.ChannelData{TuningStep: codeplug.StringField{State: codeplug.Known, Value: "10k"}}},
		{spec.FieldProgramTuningStep, codeplug.ChannelData{ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Known, Value: 1000}}},
		{spec.FieldAttenuator, codeplug.ChannelData{AttenuatorDB: codeplug.IntField{State: codeplug.Known, Value: 12}}},
		{spec.FieldPreamp, codeplug.ChannelData{Preamp: codeplug.StringField{State: codeplug.Known, Value: "1"}}},
		{spec.FieldAntenna, codeplug.ChannelData{Antenna: codeplug.StringField{State: codeplug.Known, Value: "1"}}},
		{spec.FieldIPPlus, codeplug.ChannelData{IPPlus: codeplug.BoolField{State: codeplug.Known, Value: true}}},
	} {
		got := requestedFields(tc.data)
		if len(got) != base+1 || !slices.Contains(got, tc.field) {
			t.Errorf("a channel carrying %s requests %v, want the eight plus %s", tc.field, got, tc.field)
		}
	}
}

// TestWriteChannel_OneMA0ReadThenOneMA0SetReportedSentNeverConfirmed is the
// whole choreography in one assertion: TWO frames on the wire per channel
// write, one of them mutating, the Set hand-derived byte for byte, and the
// result Sent and never Confirmed.
//
// SENT, NEVER CONFIRMED, is this family's most load-bearing posture: an
// ACCEPTED Set draws nothing at all, so silence is inconclusive, and a driver
// reporting Confirmed on silence would be asserting that reading as a fact.
func TestWriteChannel_OneMA0ReadThenOneMA0SetReportedSentNeverConfirmed(t *testing.T) {
	sess, port := openTestSession(t, Simulated, populatedImage(t, 7, occupiedSimplex()))

	res, err := sess.WriteChannel(context.Background(), simplexChannel("007"))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || res.Steps[0].Command != "MA0" {
		t.Fatalf("Steps = %+v, want exactly one MA0 step", res.Steps)
	}
	if !res.Steps[0].Sent {
		t.Error("Steps[0].Sent = false, want true")
	}
	if res.Steps[0].Confirmed {
		t.Error("Steps[0].Confirmed = true: silence is inconclusive on this family and nothing may report otherwise")
	}
	want := append(slices.Clone(probeFrames), "MA0007;", simplexSetFrame)
	if got := port.Transcript(); !slices.Equal(got, want) {
		t.Errorf("transcript = %v,\nwant %v — one MA0 read then one MA0 Set, and nothing else", got, want)
	}
}

// TestWriteChannel_NeverReadsBackAndNeverCallsReadChannel is the negative
// half of the choreography, asserted by frame COUNT.
//
// The read-back verification is core/clone's, exactly as on every other
// family (core/clone/execute.go's write-then-verify pair), so performing it
// here would double every write. The pre-write frame is this driver's OWN MA0
// read and not a call to ReadChannel: ReadChannel takes opMu, which this
// method already holds, so a call would deadlock — and the transcript is what
// proves the read happened once rather than twice.
func TestWriteChannel_NeverReadsBackAndNeverCallsReadChannel(t *testing.T) {
	sess, port := openTestSession(t, Simulated, populatedImage(t, 7, occupiedSimplex()))
	if _, err := sess.WriteChannel(context.Background(), simplexChannel("007")); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	got := port.Transcript()
	if n := len(got) - len(probeFrames); n != 2 {
		t.Fatalf("a write put %d frames on the wire (%v), want exactly two", n, got)
	}
	reads := 0
	for _, f := range got[len(probeFrames):] {
		if f == "MA0007;" {
			reads++
		}
	}
	if reads != 1 {
		t.Errorf("transcript = %v, want exactly ONE MA0 read: the read-back is core/clone's", got)
	}
}

// TestWriteChannel_ARoundTripFromTheReadPathReachesTheWire reads a populated
// SPLIT channel and writes the very same channel back, unedited.
//
// THE SET AND THE ANSWER ARE THE SAME BYTES on this row — both books draw one
// grid for Set and Answer alike (890:3166-3182 against 890:3187-3204) — so a
// faithful round trip is a byte-for-byte identity, and this is the strongest
// single statement that the read path's mapping and the write path's
// unmapping are inverses. It is also the one case where rung 11 sees a real
// secondary side and passes it.
func TestWriteChannel_ARoundTripFromTheReadPathReachesTheWire(t *testing.T) {
	answer := answerFor(t, 7, populated())
	sess, port := openTestSession(t, Simulated, radioImage{
		ma0Answers: map[string]string{"007": answer},
	})
	ch, err := sess.ReadChannel(context.Background(), "007")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	got := port.Transcript()
	if len(got) == 0 || got[len(got)-1] != answer {
		t.Errorf("the Set was %q, want the answer's own bytes %q", got[len(got)-1], answer)
	}
}

// TestWriteChannel_ATrailingSpaceNameReachesTheWireVerbatim is the C-MED-1
// reversal's driver-side consequence, and it exists to record a NON-CASE:
// this ladder has no special handling for a trailing space and needs none.
//
// The 890S's terminator floats after the name (890:3181-3182), so "AB ;" and
// "AB;" are DISTINCT frames, the space is real content rather than padding,
// and core/kw/ma carries P13 verbatim in both directions. A CHIRP-sanitised
// name that keeps a trailing space therefore round-trips as itself, with no
// refusal and no per-channel loss to surface. The TS-990S's fixed window is
// the row that assumes a pad (A1).
func TestWriteChannel_ATrailingSpaceNameReachesTheWireVerbatim(t *testing.T) {
	ch := simplexChannel("007")
	ch.Data.Tag = "GB3IV "
	sess, port := openTestSession(t, Simulated, populatedImage(t, 7, occupiedSimplex()))
	if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	got := port.Transcript()
	set := got[len(got)-1]
	if want := strings.Replace(simplexSetFrame, "GB3IV;", "GB3IV ;", 1); set != want {
		t.Errorf("Set = %q, want %q — the trailing space is content, not padding", set, want)
	}
}

// TestWriteChannel_ARejectionIsTypedAndAttributable pins the one write
// failure a caller can attribute: a "?;" inside the bounded error window
// means the frame provably went out and the radio provably refused it, so
// Sent is TRUE there where every other transport failure leaves it false.
func TestWriteChannel_ARejectionIsTypedAndAttributable(t *testing.T) {
	img := populatedImage(t, 7, occupiedSimplex())
	img.ma0SetRejects = map[string]bool{"007": true}
	sess, _ := openTestSession(t, Simulated, img)

	res, err := sess.WriteChannel(context.Background(), simplexChannel("007"))
	var rej *kw.RejectionError
	if !errors.As(err, &rej) {
		t.Fatalf("WriteChannel err = %v (%T), want a *kw.RejectionError", err, err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Sent {
		t.Errorf("Steps = %+v, want one step with Sent true: the frame provably went out", res.Steps)
	}
	if res.Steps[0].Confirmed {
		t.Error("Steps[0].Confirmed = true on a rejection")
	}
}

// TestWriteChannel_IsAtomicUnderOpMu is the pin plan P13 calls for and the
// one whose deletion would leave every other test in this package green: the
// pre-write READ and the Set are ONE operation, and nothing may land between
// them.
//
// The write is two exchanges, and transport.Engine serialises each exchange
// and not the pair. A concurrent operation landing between them would decide
// against one radio state and write against another (the ic705 precedent,
// core/driver/ic705/write.go's own statement of it). The hook parks the write
// INSIDE the lock, deterministically, rather than relying on scheduling.
func TestWriteChannel_IsAtomicUnderOpMu(t *testing.T) {
	sess, port := openTestSession(t, Simulated, populatedImage(t, 7, occupiedSimplex()))

	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	writeChannelGapHook = func() {
		entered <- struct{}{}
		<-release
	}
	t.Cleanup(func() { writeChannelGapHook = nil })

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := sess.WriteChannel(context.Background(), simplexChannel("007")); err != nil {
			t.Errorf("parked WriteChannel: %v", err)
		}
	}()
	<-entered

	readDone := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := sess.ReadChannel(context.Background(), "007"); err != nil {
			t.Errorf("ReadChannel: %v", err)
		}
		close(readDone)
	}()

	select {
	case <-readDone:
		t.Fatal("ReadChannel returned while a WriteChannel held opMu between its read and its Set")
	case <-time.After(250 * time.Millisecond):
	}
	if got := port.Transcript(); len(got) != len(probeFrames)+1 {
		t.Errorf("transcript while the write is parked = %v, want the probe and the write's own MA0 read alone", got)
	}

	close(release)
	wg.Wait()

	want := append(slices.Clone(probeFrames), "MA0007;", simplexSetFrame, "MA0007;")
	if got := port.Transcript(); !slices.Equal(got, want) {
		t.Errorf("transcript = %v,\nwant %v — the concurrent read is held until the Set has gone", got, want)
	}
}

// TestWriteChannel_AWireFailureOnThePreWriteReadNeverSends is the other half
// of the two-exchange shape: if the READ does not answer, no Set is built and
// nothing mutating goes out. A timeout is typed and says in as many words
// that it is not an inference of absence.
func TestWriteChannel_AWireFailureOnThePreWriteReadNeverSends(t *testing.T) {
	sess, port := openTestSession(t, Simulated, radioImage{ma0Silent: map[string]bool{"007": true}})

	res, err := sess.WriteChannel(context.Background(), simplexChannel("007"))
	var to *kw.TimeoutError
	if !errors.As(err, &to) {
		t.Fatalf("WriteChannel err = %v (%T), want a *kw.TimeoutError", err, err)
	}
	if len(res.Steps) != 0 {
		t.Errorf("Steps = %+v, want none: no frame was ever built", res.Steps)
	}
	for _, f := range port.Transcript() {
		if strings.HasPrefix(f, "MA0") && len(f) > 7 {
			t.Errorf("a Set reached the wire after the pre-write read failed: %q", f)
		}
	}
}

// TestWriteChannel_TheStatedSplitLimit records, in a test rather than in
// prose alone, what the neutral model can and cannot say about P11.
//
// THE MODEL CARRIES NO SPLIT FLAG. codeplug.ChannelData has one frequency and
// one tx_frequency and nothing else, so this ladder derives P11 from the ONE
// thing it has: a Known tx_frequency of zero is the simplex state the book
// itself prints (890:3217-3218, A16) and anything else is a split. The stated
// cost, in both directions:
//
//   - a channel whose tx_frequency is Known and EQUAL to its receive
//     frequency is written as a SPLIT channel, because nothing in the model
//     distinguishes it from one; against a radio slot that is currently
//     simplex that write is refused by rung 11 rather than silently made, so
//     the cost is a refusal and never a wrong byte;
//   - a genuinely split channel whose two sides are equal round-trips as a
//     split, which is the direction this derivation gets right and a
//     "tx != rx" derivation would not.
//
// No flag is invented for either. The day codeplug carries a split flag, this
// is the derivation that changes.
func TestWriteChannel_TheStatedSplitLimit(t *testing.T) {
	t.Run("a Known zero writes the printed zeroed split side", func(t *testing.T) {
		sess, port := openTestSession(t, Simulated, populatedImage(t, 7, occupiedSimplex()))
		if _, err := sess.WriteChannel(context.Background(), simplexChannel("007")); err != nil {
			t.Fatalf("WriteChannel: %v", err)
		}
		got := port.Transcript()
		set := got[len(got)-1]
		// P8 at 25-35, P9 at 36, P10 at 37, P11 at 38 — all zero.
		if second := set[24:38]; second != strings.Repeat("0", 14) {
			t.Errorf("the Set's split side is %q, want fourteen zeros (P8-P11, 890:3191-3203)", second)
		}
	})

	t.Run("an equal-sided split round-trips as a split", func(t *testing.T) {
		rec := populated()
		rec.TXFreqHz = rec.FreqHz
		answer := answerFor(t, 7, rec)
		sess, port := openTestSession(t, Simulated, radioImage{
			ma0Answers: map[string]string{"007": answer},
		})
		ch, err := sess.ReadChannel(context.Background(), "007")
		if err != nil {
			t.Fatalf("ReadChannel: %v", err)
		}
		if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
			t.Fatalf("WriteChannel: %v", err)
		}
		got := port.Transcript()
		if set := got[len(got)-1]; set != answer {
			t.Errorf("Set = %q, want the answer's own bytes %q — P11 must stay '1'", set, answer)
		}
	})
}
