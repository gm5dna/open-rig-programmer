// SPDX-License-Identifier: GPL-3.0-or-later

package ts890

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
)

// answerFor renders one MA0 answer for slot number n from a Record, THROUGH
// THE CODEC'S OWN BUILDER.
//
// THE ANSWER AND THE SET ARE THE SAME BYTES on this row — the two grids are
// drawn identically (890:3166-3182 against 890:3189-3204) — and core/kw/ma
// pins Parse ∘ Build as the identity, so building the fixture rather than
// hand-spelling it means no test in this package can assert a frame the codec
// would not produce.
func answerFor(t *testing.T, n int, rec ma.Record) string {
	t.Helper()
	l := layout()
	slot, err := l.NewSlot(n)
	if err != nil {
		t.Fatalf("NewSlot(%d): %v", n, err)
	}
	rec.Slot = slot
	cmd, err := l.BuildMA0Set(rec)
	if err != nil {
		t.Fatalf("BuildMA0Set(%+v): %v", rec, err)
	}
	return string(cmd.Bytes())
}

// blankAnswer890 is the answer a blank channel gives: P2-P12 all spaces, and
// whatever residue the caller wants in the floating name window.
//
// IT IS HAND-SPELLED BECAUSE NO BUILDER CAN PRODUCE IT, and that is spec
// decision 15 rather than a gap: writing an empty channel would be an ERASE,
// so core/kw/ma has no builder for one. Bytes 1-3 are the opcode, 4-6 the
// channel number, 7-39 the thirty-three parameter bytes the book's blank note
// covers ("parameters P2 to P12 becomes blank", 890:3215-3216), and the name
// window floats from byte 40.
func blankAnswer890(n int, residue string) string {
	return fmt.Sprintf("MA0%03d%s%s;", n, strings.Repeat(" ", 33), residue)
}

// populated is the fixture record every mapping test reads back: a split
// channel with a tone pair, a name and the lockout set, so that no field
// under test is at its zero value.
func populated() ma.Record {
	return ma.Record{
		FreqHz: 14_250_000, Mode: '2', ToneType: '3',
		ToneIndex: 50, CTCSSIndex: 12,
		TXFreqHz: 14_255_000, TXMode: '2',
		Split: true, Lockout: true, Name: "GB3IV",
	}
}

// TestReadChannel_MapsThePopulatedRecordOntoTheNeutralModel walks every
// neutral field of one read, and is the test the matrix §2.1 table is
// enforced by on the READ side.
func TestReadChannel_MapsThePopulatedRecordOntoTheNeutralModel(t *testing.T) {
	sess, port := openTestSession(t, Simulated, radioImage{
		ma0Answers: map[string]string{"007": answerFor(t, 7, populated())},
	})
	ch, err := sess.ReadChannel(context.Background(), "007")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Slot != "007" || ch.Data == nil {
		t.Fatalf("ReadChannel = %+v, want a populated 007", ch)
	}
	d := ch.Data

	// ONE MA0 FRAME PER SLOT AND NOTHING ELSE (plan P12): no MN precedes it,
	// in either direction, and the frame carries its own channel number
	// (890:3184-3186). THAT IT NEEDS NO MN IS A18, whose lift is L-HW-12 and
	// whose failure is TOTAL: if A18 is false no read on this row works at
	// all.
	if got, want := port.Transcript(), []string{"AI0;", "ID;", "FV;", "MA0007;"}; !slices.Equal(got, want) {
		t.Errorf("transcript = %v, want %v — one MA0 read, no MN (P12, A18)", got, want)
	}

	if d.FreqHz != 14_250_000 {
		t.Errorf("FreqHz = %d, want 14250000 (P2, 890:3171-3172)", d.FreqHz)
	}
	if d.Mode != "USB" {
		t.Errorf("Mode = %q, want USB (P3 = '2' via OM, 890:3174-3175)", d.Mode)
	}
	if d.Tag != "GB3IV" {
		t.Errorf("Tag = %q, want GB3IV (P13, 890:3208-3209)", d.Tag)
	}
	if d.ScanSkip.State != codeplug.Known || !d.ScanSkip.Value {
		t.Errorf("ScanSkip = %+v, want Known true (P12, 890:3205-3207)", d.ScanSkip)
	}
	if d.ToneMode.State != codeplug.Known || d.ToneMode.Value != "CROSS" {
		t.Errorf("ToneMode = %+v, want Known CROSS (P5 = '3', 890:3180-3185)", d.ToneMode)
	}
	// P6 is the TN index and P7 the CN index; THAT P6 IS THE TRANSMIT ONE IS
	// K-D1 and not this book's.
	if d.ToneTx.State != codeplug.Known || d.ToneTx.Value != 17500 {
		t.Errorf("ToneTx = %+v, want Known 1750.0 Hz (P6 = 50, 890:5162)", d.ToneTx)
	}
	if d.ToneRx.State != codeplug.Known || d.ToneRx.Value != 1000 {
		t.Errorf("ToneRx = %+v, want Known 100.0 Hz (P7 = 12, 890:1354-1369)", d.ToneRx)
	}
	if d.TxFreqHz.State != codeplug.Known || d.TxFreqHz.Value != 14_255_000 {
		t.Errorf("TxFreqHz = %+v, want Known 14255000 (P8, 890:3191-3192)", d.TxFreqHz)
	}

	// The fifteen with a FieldState the record does not express, every one
	// ANSWERED Unavailable and never left Absent. The other four have no
	// state to carry: clarifier, ctcss_state and shift are scalars checked
	// below, and erase has no ChannelData member at all.
	for name, state := range map[string]codeplug.FieldState{
		"ctcss_tone":          d.CTCSSTone.State,
		"tag_display":         d.TagDisplay.State,
		"duplex":              d.Duplex.State,
		"offset":              d.OffsetHz.State,
		"dtcs_code":           d.DTCSCode.State,
		"dtcs_polarity":       d.DTCSPolarity.State,
		"filter":              d.Filter.State,
		"data_mode":           d.DataMode.State,
		"tuning_step_enabled": d.TuningStepEnabled.State,
		"tuning_step":         d.TuningStep.State,
		"program_tuning_step": d.ProgramTuningStepHz.State,
		"attenuator":          d.AttenuatorDB.State,
		"preamp":              d.Preamp.State,
		"antenna":             d.Antenna.State,
		"ip_plus":             d.IPPlus.State,
	} {
		if state != codeplug.Unavailable {
			t.Errorf("%s state = %q, want Unavailable", name, state)
		}
	}
	if d.ClarHz != 0 || d.RxClar || d.TxClar || d.CTCSS != "" || d.Shift != "" {
		t.Errorf("the Yaesu-vocabulary members are populated: %+v", d)
	}

	// The fleet's own pin, consumed as a BLACK BOX: no tier field left
	// Absent, and the populated channel survives the normalised lowest-schema
	// save/load migration.
	drivertest.AssertFreshReadSaveLoad(t, ch, sess.Capabilities(), codeplug.Load)
}

// TestReadChannel_EveryPublishedModeNameRoundTripsThroughTheReadPath is the
// anti-drift pin between caps.go and read.go: a name the read produces that
// Capabilities.Modes does not carry would be refused by codeplug.Validate on
// the very channel this driver just read. It walks all sixteen, which is what
// makes the FM narrow twins (P4, 890:3176-3178) tested rather than assumed.
func TestReadChannel_EveryPublishedModeNameRoundTripsThroughTheReadPath(t *testing.T) {
	l := layout()
	answers := map[string]string{}
	wantName := map[string]string{}
	slot := 0
	for b := 0; b <= 0xFF; b++ {
		if _, ok := l.ModeNames()[byte(b)]; !ok {
			continue
		}
		for _, narrow := range []bool{false, true} {
			rec := populated()
			rec.Mode, rec.FMNarrow = byte(b), narrow
			rec.TXMode = byte(b)
			rec.TXFMNarrow = narrow
			id := fmt.Sprintf("%03d", slot)
			answers[id] = answerFor(t, slot, rec)
			wantName[id], _ = modeName(l, byte(b), narrow)
			slot++
		}
	}
	sess, _ := openTestSession(t, Simulated, radioImage{ma0Answers: answers})
	published := sess.Capabilities().Modes
	for id, want := range wantName {
		ch, err := sess.ReadChannel(context.Background(), id)
		if err != nil {
			t.Fatalf("ReadChannel %s: %v", id, err)
		}
		if ch.Data.Mode != want {
			t.Errorf("slot %s Mode = %q, want %q", id, ch.Data.Mode, want)
		}
		if !slices.Contains(published, ch.Data.Mode) {
			t.Errorf("slot %s read back mode %q, which Capabilities.Modes does not publish", id, ch.Data.Mode)
		}
	}
}

// TestReadChannel_TxFrequencyIsKnownOnASimplexChannelToo pins matrix §2.4 and
// M-E7 — this pair's largest gain over pair 1 — and the ONE decision the
// mapping makes: the value is the RECORD'S OWN.
//
// The book prints the unsplit case, "When reading a single memory channel,
// all parameters for Split Transmission become 0" (890:3217-3218), which is
// A16, DOCUMENTED rather than assumed. So a Known ZERO is the record's own
// statement that this channel has no separate transmit frequency. Publishing
// the receive frequency instead would assert a byte the radio did not send,
// and would make a genuinely split channel whose two sides happen to be equal
// indistinguishable from a simplex one.
//
// NEVER Unavailable, on either kind of channel: that word is pair 1's, whose
// rows could not read the TX side at all.
func TestReadChannel_TxFrequencyIsKnownOnASimplexChannelToo(t *testing.T) {
	simplex := ma.Record{FreqHz: 14_250_000, Mode: '2', ToneType: '0', Name: "SIMPLEX"}
	sess, _ := openTestSession(t, Simulated, radioImage{ma0Answers: map[string]string{
		"001": answerFor(t, 1, simplex),
		"002": answerFor(t, 2, populated()),
	}})
	for id, want := range map[string]uint64{"001": 0, "002": 14_255_000} {
		ch, err := sess.ReadChannel(context.Background(), id)
		if err != nil {
			t.Fatalf("ReadChannel %s: %v", id, err)
		}
		if ch.Data.TxFreqHz.State != codeplug.Known {
			t.Errorf("slot %s TxFreqHz state = %q, want Known on EVERY channel (§2.4, M-E7)", id, ch.Data.TxFreqHz.State)
		}
		if ch.Data.TxFreqHz.Value != want {
			t.Errorf("slot %s TxFreqHz = %d, want %d", id, ch.Data.TxFreqHz.Value, want)
		}
	}
}

// TestReadChannel_AnUnassignedChannelIsNotAnError is plan P14's pin, and the
// NEGATIVE form is the half that matters: clone.ReadAll returns on the FIRST
// channel error and abandons the whole radio's read
// (core/clone/read.go:65-67), so one blank slot answering with an error would
// make a fresh TS-890S unreadable end to end.
func TestReadChannel_AnUnassignedChannelIsNotAnError(t *testing.T) {
	sess, _ := openTestSession(t, Simulated, radioImage{ma0Answers: map[string]string{
		"005": blankAnswer890(5, ""),
	}})
	ch, err := sess.ReadChannel(context.Background(), "005")
	if err != nil {
		t.Fatalf("a blank channel must be reported unassigned, never as an error: %v", err)
	}
	if ch.Slot != "005" || ch.Data != nil {
		t.Errorf("ReadChannel = %+v, want slot 005 with a nil Data — the seam's spelling of unassigned", ch)
	}
}

// TestReadChannel_ANameResidueIsCarriedAndNotRaisedOn is A21's pin, and it
// covers the one thing this book leaves open that pair 1's does not: the
// blank-channel note stops at P12 (erratum E4), so a fresh radio may answer a
// blank channel with bytes still in its floating name window. THE PREDICATE
// IGNORES P13 — the channel is unassigned whatever the window holds — and the
// residue is REPORTED rather than discarded silently or raised as an error,
// the third option being the one that would cost the whole radio's read.
func TestReadChannel_ANameResidueIsCarriedAndNotRaisedOn(t *testing.T) {
	var log recordingLogger
	sess, _ := openTestSession(t, Simulated, radioImage{ma0Answers: map[string]string{
		"005": blankAnswer890(5, "OLDNAME"),
		"006": blankAnswer890(6, ""),
	}}, WithTransportLogger(&log))

	ch, err := sess.ReadChannel(context.Background(), "005")
	if err != nil {
		t.Fatalf("a residue must not raise: %v", err)
	}
	if ch.Data != nil {
		t.Errorf("ReadChannel = %+v, want unassigned whatever P13 holds (A21, E4)", ch)
	}
	if got := log.lines(); len(got) != 1 || !strings.Contains(got[0], "OLDNAME") {
		t.Errorf("the residue was not carried: logger saw %v", got)
	}

	if _, err := sess.ReadChannel(context.Background(), "006"); err != nil {
		t.Fatalf("ReadChannel 006: %v", err)
	}
	if got := log.lines(); len(got) != 1 {
		t.Errorf("a blank channel with NO residue wrote a note: %v", got)
	}
}

// recordingLogger captures the driver's own notes. transport.Logger is the
// only sink a driver has for a per-channel detail: codeplug.Channel carries
// none, driver.SessionDiagnostics is a counter, and an error would cost the
// whole read.
type recordingLogger struct {
	mu   sync.Mutex
	seen []string
}

func (l *recordingLogger) Printf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seen = append(l.seen, fmt.Sprintf(format, args...))
}

func (l *recordingLogger) lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.seen...)
}

// TestReadChannel_AnUnknownSlotIsRefusedBeforeAnyFrame covers all three ways
// a slot fails, INCLUDING the twenty this row deliberately does not publish.
// The check is bank MEMBERSHIP, settled from this session's own capabilities,
// so the radio is never asked — asserted as the transcript.
func TestReadChannel_AnUnknownSlotIsRefusedBeforeAnyFrame(t *testing.T) {
	sess, port := openTestSession(t, Simulated, radioImage{})
	before := len(port.Transcript())
	for _, id := range []string{
		// Malformed.
		"", "7", "0007", "00A", "100L",
		// Outside the printed space entirely.
		"999",
		// PRINTED and deliberately unpublished: the Programmable VFO class
		// (890:3315-3319, A9/A7) and the E channels (890:3169, A10).
		"100", "109", "110", "119",
	} {
		_, err := sess.ReadChannel(context.Background(), id)
		var unknown *UnknownSlotError
		if !errors.As(err, &unknown) {
			t.Errorf("ReadChannel %q: err = %v, want an *UnknownSlotError", id, err)
		}
		if !errors.Is(err, ErrUnknownSlot) {
			t.Errorf("ReadChannel %q: errors.Is(err, ErrUnknownSlot) = false", id)
		}
	}
	if got := port.Transcript(); len(got) != before {
		t.Errorf("an unknown slot put %v on the wire", got[before:])
	}
}

// TestReadChannel_ARejectionIsDefinitiveAndNeverAbsence: the book prints "?;"
// as EITHER "Command syntax was incorrect" OR "Command was not executed due
// to the current status of the transceiver" (890:106-112) — indistinguishable
// — so it is never read as "this slot is empty". It fails the read WHOLE.
func TestReadChannel_ARejectionIsDefinitiveAndNeverAbsence(t *testing.T) {
	// An unscripted channel is answered "?;" by the peer's own default.
	sess, _ := openTestSession(t, Simulated, radioImage{})
	ch, err := sess.ReadChannel(context.Background(), "042")
	var rej *kw.RejectionError
	if !errors.As(err, &rej) {
		t.Fatalf("err = %v, want a *kw.RejectionError naming both printed causes", err)
	}
	if ch.Slot != "" || ch.Data != nil {
		t.Errorf("a rejection produced a channel %+v; it must never be read as absence", ch)
	}
}

// TestReadChannel_ATimeoutIsNotAnInferenceOfAbsence: silence carries no
// information on this family, so a read that draws nothing fails the session
// read whole with the typed error that says so.
func TestReadChannel_ATimeoutIsNotAnInferenceOfAbsence(t *testing.T) {
	sess, _ := openTestSession(t, Simulated, radioImage{ma0Silent: map[string]bool{"042": true}})
	ch, err := sess.ReadChannel(context.Background(), "042")
	var to *kw.TimeoutError
	if !errors.As(err, &to) {
		t.Fatalf("err = %v, want a *kw.TimeoutError", err)
	}
	if ch.Data != nil {
		t.Errorf("a timeout produced a populated channel %+v", ch)
	}
}

// TestReadChannel_AnAnswerNamingAnotherSlotIsRefused documents the SECOND
// line of defence, not the first: Layout.MA0AnswerMatcher's prefix is
// ma0Prefix plus the three channel digits (core/kw/ma/shared.go), and
// rec.Slot is decoded from those same bytes, so on this row the matcher's
// prefix already carries the whole correlation key and a stale answer for
// another slot is refused before AnswerMismatchError's comparison in
// read.go ever runs — unlike the 590, whose P1 sits too deep in the frame
// to spell as a prefix. The guard stays (T12's read-then-Set wants the same
// comparison), but on this row it can only be exercised by a fault the
// matcher cannot see, which this fixture does not construct. What IS
// observable, and what this test asserts, is the timeout: the peer's
// slot-008 frame never matches a read of 007, so nothing answers.
func TestReadChannel_AnAnswerNamingAnotherSlotIsRefused(t *testing.T) {
	// The peer is keyed on the READ's channel digits, so serving 008's frame
	// for a read of 007 is exactly the fault under test.
	sess, _ := openTestSession(t, Simulated, radioImage{ma0Answers: map[string]string{
		"007": answerFor(t, 8, populated()),
	}})
	_, err := sess.ReadChannel(context.Background(), "007")
	var to *kw.TimeoutError
	if !errors.As(err, &to) {
		t.Fatalf("err = %v, want a *kw.TimeoutError: the matcher's prefix refuses slot 008's answer before AnswerMismatchError's comparison can run", err)
	}
	if errors.Is(err, ErrAnswerMismatch) {
		t.Error("errors.Is(err, ErrAnswerMismatch) = true, want false: the mismatch guard is unreachable on this row")
	}
}

// TestReadChannel_AWrongWidthAnswerNeverReachesTheParser pins BOTH halves of
// the length contract on this row, which is the first in the repository whose
// frame length is a RANGE rather than a constant.
//
//   - THE MATCHER refuses a frame outside 40-50 bytes, so it is not this
//     read's answer at all and the read times out. A bare unbounded prefix
//     matcher would correlate a two-hundred-byte run of noise opening with
//     the right six bytes.
//   - THE PARSER refuses one too, if a caller hands it one directly, and the
//     refusal is classifiable and quotes the MEASURED width. That half is
//     asserted through drivertest.AssertKenwoodMAFrameLengthMismatch — the
//     range-shaped fleet helper, consumed here and in core/driver/ts990 and
//     nowhere else.
func TestReadChannel_AWrongWidthAnswerNeverReachesTheParser(t *testing.T) {
	short := "MA0007000142500002000000000000000000;" // 37 bytes: below the 40 A17 gives
	sess, _ := openTestSession(t, Simulated, radioImage{ma0Answers: map[string]string{"007": short}})
	_, err := sess.ReadChannel(context.Background(), "007")
	var to *kw.TimeoutError
	if !errors.As(err, &to) {
		t.Errorf("err = %v, want a *kw.TimeoutError — a frame outside 40-50 bytes is not this read's answer", err)
	}

	_, perr := layout().ParseMA0Answer([]byte(short))
	drivertest.AssertKenwoodMAFrameLengthMismatch(t, perr, "MA0 answer", len(short))
}

// TestReadChannel_IsAtomicUnderOpMu pins plan P13's lock. It is the pin whose
// DELETION pair 1 recorded as leaving every other test green, so it parks one
// read inside the lock deterministically through the test-only hook rather
// than relying on goroutine scheduling: Go's sync.Mutex favours an
// immediately-re-locking goroutine so heavily that the interleaving opMu
// exists to forbid is near-impossible to reproduce by hammering alone.
func TestReadChannel_IsAtomicUnderOpMu(t *testing.T) {
	sess, _ := openTestSession(t, Simulated, radioImage{ma0Answers: map[string]string{
		"007": answerFor(t, 7, populated()),
		"008": answerFor(t, 8, populated()),
	}})

	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	readChannelGapHook = func() {
		once.Do(func() {
			close(entered)
			<-release
		})
	}
	t.Cleanup(func() { readChannelGapHook = nil })

	done := make(chan error, 1)
	go func() {
		_, err := sess.ReadChannel(context.Background(), "007")
		done <- err
	}()
	<-entered

	second := make(chan error, 1)
	go func() {
		_, err := sess.ReadChannel(context.Background(), "008")
		second <- err
	}()
	select {
	case err := <-second:
		t.Fatalf("a second ReadChannel completed (%v) while the first held opMu", err)
	default:
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("the parked read: %v", err)
	}
	if err := <-second; err != nil {
		t.Fatalf("the second read: %v", err)
	}
}
