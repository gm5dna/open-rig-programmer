// SPDX-License-Identifier: GPL-3.0-or-later

package ts990

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestReadChannel_SendsOneMA0ReadAndNothingElse is plan P12's choreography as
// a transcript: ONE seven-byte MA0 Read per slot, carrying its own channel
// number, and NO MN in front of it — which is A18, the load-bearing read
// assumption of the whole milestone. With MN off the outbound roster the
// driver has no way to send one even if it had to, so if A18 is false no read
// on this row works at all.
func TestReadChannel_SendsOneMA0ReadAndNothingElse(t *testing.T) {
	frame := populatedMA0("042")
	assertFrameWidth(t, frame)
	sess, p := openTestSession(t, ma0Image("042", frame))
	if _, err := sess.ReadChannel(context.Background(), "042"); err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	want := []string{"AI0;", "ID;", "FV;", "MA0042;"}
	if got := p.Transcript(); !reflect.DeepEqual(got, want) {
		t.Errorf("transcript = %v, want %v (P12, matrix §3.8)", got, want)
	}
	for _, sent := range p.Transcript() {
		if strings.HasPrefix(sent, "MN") {
			t.Errorf("an MN frame was sent: %q — MN is off the outbound roster in both directions (A18, decision 5)", sent)
		}
	}
}

// TestReadChannel_MapsThePrimarySideOntoTheNeutralModel is matrix §2.1's
// TS-990S column as one populated channel, field by field, with the three
// zeroes a reader coming from pair 1 will look for spelled out.
func TestReadChannel_MapsThePrimarySideOntoTheNeutralModel(t *testing.T) {
	sess, _ := openTestSession(t, ma0Image("042", populatedMA0("042")))
	ch, err := sess.ReadChannel(context.Background(), "042")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Slot != "042" || ch.Data == nil {
		t.Fatalf("Channel = %+v, want slot 042 with data", ch)
	}
	want := codeplug.ChannelData{
		FreqHz: 14_175_000,
		Mode:   "FM",
		// The eighteen parameters account for every byte of the grid and
		// none of them is an RIT/XIT offset (M-E4, §1.7): plain scalars
		// with no state to say so, so the honest reading is their zero.
		ClarHz: 0, RxClar: false, TxClar: false,
		// The Yaesu half of the vocabulary pair (decision 6).
		CTCSS: "", Shift: "",
		CTCSSTone:  codeplug.ToneField{State: codeplug.Unavailable},
		Tag:        "Bench",
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
		// P17 at byte 46, and its OFF value is '1' rather than '0' — E8.
		ScanSkip: codeplug.BoolField{State: codeplug.Known, Value: false},
		// Known on every channel of every read (§2.4): this is the split
		// flag's own answer, not a guess.
		TxFreqHz: codeplug.FreqField{State: codeplug.Known, Value: 0},
		Duplex:   codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz: codeplug.FreqField{State: codeplug.Unavailable},
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: "TONE"},
		ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: 885},
		ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: 1000},
		// DCS appears nowhere in this book (§1.20, §1.21).
		DTCSCode:     codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},
		// No filter byte in this grid, unlike the 590SG's byte 28 (§1.22).
		Filter: codeplug.StringField{State: codeplug.Unavailable},
		// M-E3: no DA command, no data byte — the data-ness is in the mode
		// NAMES, and this is the sharpest single divergence from pair 1,
		// whose 590 rows report a Known data_mode from byte 19.
		DataMode:            codeplug.BoolField{State: codeplug.Unavailable},
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
		Preamp:              codeplug.StringField{State: codeplug.Unavailable},
		Antenna:             codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},
	}
	if !reflect.DeepEqual(*ch.Data, want) {
		t.Errorf("ChannelData =\n %+v\nwant\n %+v", *ch.Data, want)
	}
	// The fleet pins, consumed as BLACK BOXES: a fresh read leaves no tier
	// field Absent, reports the seven receiver fields Unavailable, and the
	// channel survives the normalised lowest-schema save/load migration.
	drivertest.AssertFreshReadSaveLoad(t, ch, sess.Capabilities(), codeplug.Load)
}

// TestReadChannel_TheNarrowFMNameIsSynthesisedFromTwoBytes pins §1.5's
// mechanism on the read side: P5 is orthogonal to the mode byte, and the name
// the read produces must be BYTE-FOR-BYTE one Capabilities().Modes
// advertises — which is what modeDisplayName being the single site buys.
func TestReadChannel_TheNarrowFMNameIsSynthesisedFromTwoBytes(t *testing.T) {
	for name, tc := range map[string]struct {
		mode, narrow byte
		want         string
	}{
		"FM wide":      {'4', '0', "FM"},
		"FM narrow":    {'4', '1', "FM-N"},
		"FM-D2 wide":   {'I', '0', "FM-D2"},
		"FM-D3 narrow": {'M', '1', "FM-D3-N"},
		// The width byte is orthogonal to the mode byte and this book
		// scopes it to no particular mode value, so a narrow flag on a
		// non-FM mode changes no NAME: LSB is LSB either way.
		"LSB with the narrow flag set": {'1', '1', "LSB"},
	} {
		f := populatedFields("007")
		f.mode, f.narrow = tc.mode, tc.narrow
		sess, _ := openTestSession(t, ma0Image("007", f.frame()))
		ch, err := sess.ReadChannel(context.Background(), "007")
		if err != nil {
			t.Fatalf("%s: ReadChannel: %v", name, err)
		}
		if ch.Data.Mode != tc.want {
			t.Errorf("%s: Mode = %q, want %q (§1.5)", name, ch.Data.Mode, tc.want)
		}
		if !slices.Contains(sess.Capabilities().Modes, ch.Data.Mode) {
			t.Errorf("%s: the read produced %q, which Capabilities().Modes does not advertise", name, ch.Data.Mode)
		}
	}
}

// TestReadChannel_TheSplitFlagDecidesWhetherTheSecondFrequencyIsATransmitOne
// is §2.4 and §2.7's read rule together: ONE answer carries both sides and
// the flag, so tx_frequency is Known on every channel — and P15 is what says
// whether frequency 2 IS a transmit frequency, rather than the mere presence
// of a non-zero P9.
func TestReadChannel_TheSplitFlagDecidesWhetherTheSecondFrequencyIsATransmitOne(t *testing.T) {
	split := populatedFields("011")
	split.txFreq, split.txMode, split.split = "00014275000", '4', '1'
	sess, _ := openTestSession(t, ma0Image("011", split.frame()))
	ch, err := sess.ReadChannel(context.Background(), "011")
	if err != nil {
		t.Fatalf("split: ReadChannel: %v", err)
	}
	if want := (codeplug.FreqField{State: codeplug.Known, Value: 14_275_000}); ch.Data.TxFreqHz != want {
		t.Errorf("split: TxFreqHz = %+v, want %+v (§2.4)", ch.Data.TxFreqHz, want)
	}

	// A DUAL-RECEPTION channel carries a live second side with P15 = 0
	// (990:2946-2951). Its frequency 2 is a sub-band RECEIVE frequency, and
	// reporting it as tx_frequency would publish a receive frequency as a
	// transmit one — §2.7's "read, checked, and published nowhere".
	dual := populatedFields("012")
	dual.txFreq, dual.txMode, dual.dual = "00007100000", '3', '1'
	sess2, _ := openTestSession(t, ma0Image("012", dual.frame()))
	ch2, err := sess2.ReadChannel(context.Background(), "012")
	if err != nil {
		t.Fatalf("dual: ReadChannel: %v — a channel the user can see on the front panel must still read (§2.7)", err)
	}
	if want := (codeplug.FreqField{State: codeplug.Known, Value: 0}); ch2.Data.TxFreqHz != want {
		t.Errorf("dual: TxFreqHz = %+v, want %+v — P15 is 0, so frequency 2 is not a transmit frequency (§2.7)", ch2.Data.TxFreqHz, want)
	}
}

// TestReadChannel_TheSecondarySideIsReadAndPublishedNowhere is §2.7's read
// half stated as the negative it is: a record whose frequency-2 tuple carries
// a DIFFERENT mode, width and tone pair from the primary side reads without
// error, and none of those five values appears anywhere in the channel this
// driver produces.
//
// A READ DOES NOT FAIL ON THAT, because refusing to read a channel the user
// can see on the front panel would be worse than reporting the part the model
// holds. The refusal is the WRITE path's (T14, rung 11).
func TestReadChannel_TheSecondarySideIsReadAndPublishedNowhere(t *testing.T) {
	f := populatedFields("013")
	f.txFreq, f.txMode, f.txNarrow = "00021300000", '3', '1'
	f.txToneType, f.txTone, f.txCTCSS = "2"[0], "20", "30"
	f.split, f.dual = '1', '1'
	sess, _ := openTestSession(t, ma0Image("013", f.frame()))
	ch, err := sess.ReadChannel(context.Background(), "013")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	// The primary side is what the model publishes.
	if ch.Data.Mode != "FM" || ch.Data.ToneMode.Value != "TONE" {
		t.Errorf("the primary side is %q/%q, want FM/TONE — the frequency-2 tuple must not displace it", ch.Data.Mode, ch.Data.ToneMode.Value)
	}
	// CW is P10's mode and 136.5/173.8 Hz are P13's and P14's tones; none
	// may appear anywhere in the neutral channel.
	if ch.Data.Mode == "CW" || ch.Data.ToneTx.Value == 1365 || ch.Data.ToneRx.Value == 1738 {
		t.Errorf("a frequency-2 value reached the neutral model: %+v (§2.7)", *ch.Data)
	}
	// P15 = 1 decides ALONE: with P16 = 1 too, frequency 2 still publishes as
	// the transmit frequency — dual reception does not override the split
	// flag (see channelData's doc comment).
	if want := (codeplug.FreqField{State: codeplug.Known, Value: 21_300_000}); ch.Data.TxFreqHz != want {
		t.Errorf("split+dual: TxFreqHz = %+v, want %+v — P15 decides alone", ch.Data.TxFreqHz, want)
	}
}

// TestReadChannel_ClassByteReadSide pins doc.go's forward note (LOW-5): P2,
// the channel type, is parsed and kept on the record but published nowhere —
// a Dual ('1') or Section-defined ('2') channel reads as the SAME plain
// record as a Single ('0') one, and only the class itself refuses the read
// (T14's decision 9). '3' is outside the three values the book prints
// (990:2897-2903) and the codec refuses it.
func TestReadChannel_ClassByteReadSide(t *testing.T) {
	var baseline *codeplug.ChannelData
	for _, class := range []byte{'0', '1', '2'} {
		f := populatedFields("014")
		f.class = class
		sess, _ := openTestSession(t, ma0Image("014", f.frame()))
		ch, err := sess.ReadChannel(context.Background(), "014")
		if err != nil {
			t.Fatalf("class %q: ReadChannel: %v", class, err)
		}
		if baseline == nil {
			baseline = ch.Data
			continue
		}
		if !reflect.DeepEqual(*ch.Data, *baseline) {
			t.Errorf("class %q: ChannelData =\n %+v\nwant the same record as class '0':\n %+v", class, *ch.Data, *baseline)
		}
	}

	f := populatedFields("014")
	f.class = '3'
	sess, _ := openTestSession(t, ma0Image("014", f.frame()))
	_, err := sess.ReadChannel(context.Background(), "014")
	if err == nil {
		t.Fatal("class '3' was accepted; the book prints only three values (990:2897-2903)")
	}
	if !errors.Is(err, kw.ErrParse) {
		t.Errorf("class '3': err = %v, want one wrapping kw.ErrParse", err)
	}
	if !strings.Contains(err.Error(), "2897-2903") {
		t.Errorf("class '3': %q does not cite 990:2897-2903", err)
	}
}

// TestReadChannel_AnUnassignedSlotIsNotAnError is plan P14, and the NEGATIVE
// is the half that matters: clone.ReadAll returns on the FIRST channel error
// and abandons the whole read, so one unassigned slot on a fresh radio must
// not make that radio unreadable end to end.
//
// A blank channel answers with P2-P18 blank (990:2962-2963), which on this row
// INCLUDES the name window — so unlike the sibling row there is no residue arm
// here, and nothing is carried in the read's detail.
func TestReadChannel_AnUnassignedSlotIsNotAnError(t *testing.T) {
	frame := blankMA0("099")
	assertFrameWidth(t, frame)
	sess, _ := openTestSession(t, ma0Image("099", frame))
	ch, err := sess.ReadChannel(context.Background(), "099")
	if err != nil {
		t.Fatalf("a blank channel returned an error: %v — clone.ReadAll would abandon the whole radio", err)
	}
	if ch.Slot != "099" || ch.Data != nil {
		t.Errorf("Channel = %+v, want slot 099 with nil Data — the seam's own spelling of \"this slot is empty\"", ch)
	}
}

// TestReadChannel_TheScanLockoutByteIsThisRadiosOwnEncoding is erratum E8, the
// sharpest trap in either book: MA0 P17 is "1: Scan Lockout OFF / 2: Scan
// Lockout ON" (990:2952-2954) where this same book's MA3 P2 is 0/1
// (990:3019-3021). A driver that "helpfully" accepted '0' would be silently
// importing MA3's convention into MA0's field.
func TestReadChannel_TheScanLockoutByteIsThisRadiosOwnEncoding(t *testing.T) {
	for lockout, want := range map[byte]bool{'1': false, '2': true} {
		f := populatedFields("021")
		f.lockout = lockout
		sess, _ := openTestSession(t, ma0Image("021", f.frame()))
		ch, err := sess.ReadChannel(context.Background(), "021")
		if err != nil {
			t.Fatalf("P17 = %q: ReadChannel: %v", lockout, err)
		}
		if got := (codeplug.BoolField{State: codeplug.Known, Value: want}); ch.Data.ScanSkip != got {
			t.Errorf("P17 = %q: ScanSkip = %+v, want %+v (990:2952-2954)", lockout, ch.Data.ScanSkip, got)
		}
	}
	// '0' is MA3's OFF and is NOT a value MA0 P17 prints: it is REFUSED,
	// not coerced, and the refusal names E8.
	f := populatedFields("021")
	f.lockout = '0'
	sess, _ := openTestSession(t, ma0Image("021", f.frame()))
	_, err := sess.ReadChannel(context.Background(), "021")
	if err == nil {
		t.Fatal("P17 = '0' was accepted; that is MA3's convention, not MA0's (E8)")
	}
	if !errors.Is(err, kw.ErrParse) {
		t.Errorf("P17 = '0': err = %v, want one wrapping kw.ErrParse", err)
	}
	if !strings.Contains(err.Error(), "E8") {
		t.Errorf("P17 = '0': %q does not name erratum E8", err)
	}
}

// TestReadChannel_AnUnknownSlotIsRefusedBeforeAnyFrameIsBuilt is P11's
// negative, and it is where slots 100-119 land: the check is bank MEMBERSHIP,
// settled entirely from this session's own published capabilities, so the
// radio is never asked.
func TestReadChannel_AnUnknownSlotIsRefusedBeforeAnyFrameIsBuilt(t *testing.T) {
	for _, id := range []string{
		"100", // Section defined / Programmable VFO — A9, §1.4.2
		"109",
		"110", // E0-E9, never explained anywhere — A10, §1.4.3
		"119",
		"999", // outside the printed space altogether
		"42",  // malformed: this row's identifier is three digits
		"04A",
		"042L", // pair 1's section-channel suffix, which this row has no P1 half for
	} {
		sess, p := openTestSession(t, radioImage{})
		before := len(p.Transcript())
		_, err := sess.ReadChannel(context.Background(), id)
		if !errors.Is(err, ErrUnknownSlot) {
			t.Errorf("slot %q: err = %v, want one matching ErrUnknownSlot", id, err)
		}
		var unknown *UnknownSlotError
		if !errors.As(err, &unknown) {
			t.Errorf("slot %q: errors.As(*UnknownSlotError) = false for %v", id, err)
		}
		if got := len(p.Transcript()); got != before {
			t.Errorf("slot %q: %d frames were sent; the refusal is settled from the capability set alone", id, got-before)
		}
	}
}

// TestReadChannel_AnAnswerNamingAnotherSlotIsNeverDelivered is where pair 1's
// answer-mismatch refusal went. On this row the correlation key IS the whole
// identifier — "MA0" plus the three channel digits, at an exact 57 bytes — so
// an answer naming another channel is not this read's answer at all: it is
// never delivered, the read times out, and no comparison inside the driver is
// reachable to make. Storing one channel's content under another's identifier
// is refused by the matcher rather than by a rung.
func TestReadChannel_AnAnswerNamingAnotherSlotIsNeverDelivered(t *testing.T) {
	// The responder is keyed on the requested slot, so this serves slot
	// 042's read an answer whose own P1 says 043.
	sess, _ := openTestSession(t, ma0Image("042", populatedMA0("043")))
	_, err := sess.ReadChannel(context.Background(), "042")
	if !errors.Is(err, transport.ErrTimeout) {
		t.Fatalf("err = %v, want a timeout — an answer naming 043 is not the answer to a read of 042", err)
	}
	if errors.Is(err, kw.ErrParse) {
		t.Errorf("the foreign answer reached the parser: %v", err)
	}
}

// TestReadChannel_AMisSizedAnswerNeverReachesTheParser pins the correlation
// key's own decision, which on this row is an EXACT length (P12, §3.8): the
// key spells as a prefix because the channel number is bytes 4-6, immediately
// after the opcode, and the width is nailed at 57 (990:2938). A frame of any
// other width is not this read's answer at all, so it is not delivered and the
// read times out — the right order, because the frame the radio sent is not
// the frame this command asked for.
//
// THE SECOND HALF IS THE CODEC'S OWN WIDTH PREDICATE, asserted here on the
// same bytes so that the two rules cannot drift: the matcher decides what is
// delivered, and core/kw/ma decides what a memory frame IS.
func TestReadChannel_AMisSizedAnswerNeverReachesTheParser(t *testing.T) {
	// FIFTY-SIX BYTES: the whole record less its last name byte, then the
	// terminator, so the frame is one short of the printed grid and is
	// otherwise well formed.
	full := populatedMA0("042")
	short := full[:ma0AnswerLen-2] + ";"
	sess, p := openTestSession(t, ma0Image("042", short))
	_, err := sess.ReadChannel(context.Background(), "042")
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("a 56-byte MA0 answer: err = %v, want a timeout — an exact-width matcher must not correlate it", err)
	}
	if errors.Is(err, kw.ErrParse) {
		t.Errorf("a 56-byte MA0 answer reached the parser: %v", err)
	}
	if got := p.Transcript(); len(got) < 4 || got[3] != "MA0042;" {
		t.Errorf("transcript = %v, want the MA0 read to have gone out", got)
	}

	// The codec's own width predicate, on the same bytes, through the FLEET
	// helper — the MA-family sibling of the one core/driver/ts590 and
	// core/driver/ts480 call. It asserts the SHARED half only: the failure
	// is classifiable (kw.ErrParse), recoverable as a *kw.ParseError, and
	// its Reason opens by naming the frame and the MEASURED width.
	_, perr := sess.layout.ParseMA0Answer([]byte(short))
	drivertest.AssertKenwoodMAFrameLengthMismatch(t, perr, "MA0 answer", ma0AnswerLen-1)

	// THE BOUND CLAUSE IS THIS ROW'S OWN AND IS PINNED HERE RATHER THAN IN
	// THE HELPER. The two codecs' clauses differ in KIND — the TS-890S's
	// terminator floats, so its bound is a RANGE, and this row's is an
	// EQUALITY at 57 (990:2919-2938) — and pinning either in a shared helper
	// would make one row's assertion the other's, which is the sibling
	// inheritance this milestone's per-row rule exists to prevent.
	if !strings.Contains(perr.Error(), "exactly 57") {
		t.Errorf("the refusal %q does not spell this row's own bound, an equality at 57", perr)
	}
}

// TestReadChannel_ARejectionIsDefinitiveAndNeverAbsence and its sibling below
// are decision 5's two wire events. The book prints "?;" as EITHER a syntax
// error OR a command not executed in the current status — indistinguishable —
// so it is the typed kw.RejectionError and it fails the session read WHOLE. It
// is NEVER reported as an empty channel: an empty channel has its own printed
// answer (990:2962-2963) and confusing the two would silently blank a slot.
func TestReadChannel_ARejectionIsDefinitiveAndNeverAbsence(t *testing.T) {
	// An unscripted slot is answered "?;" by the responder.
	sess, _ := openTestSession(t, radioImage{})
	ch, err := sess.ReadChannel(context.Background(), "042")
	if err == nil {
		t.Fatalf("a \"?;\" was reported as %+v rather than refused", ch)
	}
	if !errors.Is(err, transport.ErrRejected) {
		t.Errorf("err = %v, want one wrapping transport.ErrRejected", err)
	}
	var rej *kw.RejectionError
	if !errors.As(err, &rej) {
		t.Errorf("errors.As(*kw.RejectionError) = false for %v", err)
	}
}

// TestReadChannel_ATimeoutIsNotAnInferenceOfAbsence: the book says the NAK is
// unreliable, so silence carries no information at all — neither "absent" nor
// "rejected" — and the read fails whole with the typed error that says so.
func TestReadChannel_ATimeoutIsNotAnInferenceOfAbsence(t *testing.T) {
	sess, _ := openTestSession(t, radioImage{ma0Silent: map[string]bool{"042": true}})
	ch, err := sess.ReadChannel(context.Background(), "042")
	if err == nil {
		t.Fatalf("silence was reported as %+v rather than refused", ch)
	}
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("err = %v, want one wrapping transport.ErrTimeout", err)
	}
	var to *kw.TimeoutError
	if !errors.As(err, &to) {
		t.Errorf("errors.As(*kw.TimeoutError) = false for %v", err)
	}
}

// TestReadChannel_IsAtomicUnderOpMu is P12/P13's concurrency pin, and it is a
// NEGATIVE one made deterministic by a test-only hook rather than by
// hammering: Go's sync.Mutex favours an immediately-re-locking goroutine so
// heavily that the interleaving opMu forbids is near-impossible to reproduce
// otherwise. The hook runs with opMu ALREADY HELD, so entering it is proof
// that a second operation cannot start until the first returns.
func TestReadChannel_IsAtomicUnderOpMu(t *testing.T) {
	sess, p := openTestSession(t, radioImage{ma0Answers: map[string]string{
		"001": populatedMA0("001"),
		"002": populatedMA0("002"),
	}})

	// THE HOOK SIGNALS ENTRY ON EVERY CALL AND PARKS ON THE FIRST, which is
	// what makes the pin bite: a hook that only parked would let a second
	// operation run to completion unobserved, and the test would pass with
	// the lock removed. Non-blocking send, so a later call can never
	// deadlock on a receiver the test has stopped providing.
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
		if _, err := sess.ReadChannel(context.Background(), "001"); err != nil {
			t.Errorf("parked ReadChannel: %v", err)
		}
	}()
	<-entered // the first operation is inside the lock and parked

	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := sess.ReadChannel(context.Background(), "002"); err != nil {
			t.Errorf("second ReadChannel: %v", err)
		}
	}()

	select {
	case <-entered:
		t.Fatal("a second operation entered ReadChannel while the first held opMu")
	case <-time.After(250 * time.Millisecond):
	}
	// Nothing has reached the wire either: the parked operation is ahead of
	// its own frame, and the second is ahead of the lock.
	if got := p.Transcript(); len(got) != 3 {
		t.Errorf("transcript while one operation is parked inside opMu = %v, want the probe's three frames alone", got)
	}

	close(release)
	wg.Wait()

	want := []string{"AI0;", "ID;", "FV;", "MA0001;", "MA0002;"}
	if got := p.Transcript(); !reflect.DeepEqual(got, want) {
		t.Errorf("transcript = %v, want %v — the two operations must not interleave", got, want)
	}
}

// TestReadChannel_EveryPublishedSlotReadsAndParses walks the whole published
// inventory, which is what proves memSlots and the read path agree: every
// identifier the capability set advertises is one BuildMA0Read accepts and one
// the answer matcher correlates.
func TestReadChannel_EveryPublishedSlotReadsAndParses(t *testing.T) {
	answers := map[string]string{}
	for n := 0; n <= 99; n++ {
		answers[slotID(n)] = populatedMA0(slotID(n))
	}
	sess, _ := openTestSession(t, radioImage{ma0Answers: answers})
	for _, id := range sess.Capabilities().Banks[0].Slots {
		ch, err := sess.ReadChannel(context.Background(), id)
		if err != nil {
			t.Fatalf("slot %s: ReadChannel: %v", id, err)
		}
		if ch.Slot != id || ch.Data == nil {
			t.Fatalf("slot %s: Channel = %+v", id, ch)
		}
	}
}

// TestTone_IsAskedOfTheCapabilitySetAndNotOfTheLocalTable keeps a read from
// constructing a Known value codeplug.ToneField.Valid would then refuse: the
// domain is a CAPABILITY, and caps.AdmitsTone is the one predicate every
// validator above this driver applies.
func TestTone_IsAskedOfTheCapabilitySetAndNotOfTheLocalTable(t *testing.T) {
	sess, _ := openTestSession(t, radioImage{})
	if _, err := sess.tone(len(ctcssTones), "P7, the TN index"); err == nil {
		t.Error("an index past the end of the chart produced a tone; it must be refused rather than clamped")
	}
	got, err := sess.tone(50, "P7, the TN index")
	if err != nil {
		t.Fatalf("index 50: %v", err)
	}
	if want := (codeplug.ToneField{State: codeplug.Known, Value: 17500}); got != want {
		t.Errorf("index 50 = %+v, want %+v (990:4971)", got, want)
	}
	if err := got.Valid(sess.Capabilities()); err != nil {
		t.Errorf("the tone this read would publish is not Valid: %v", err)
	}
	if !sess.Capabilities().AdmitsTone(17500) {
		t.Error("the capability set does not admit 1750 Hz, which TN index 50 prints")
	}
}

// TestBankOf_AgreesWithTheReadPathsRefusal keeps the two halves of slot
// membership from drifting: what Capabilities publishes is exactly what
// ReadChannel accepts.
func TestBankOf_AgreesWithTheReadPathsRefusal(t *testing.T) {
	caps := CapabilitiesSimulated()
	for n := 0; n <= 999; n++ {
		id := slotID(n)
		_, published := caps.BankOf(id)
		if published != (n <= 99) {
			t.Errorf("slot %q published = %v, want %v (§1.4)", id, published, n <= 99)
		}
	}
	if bank, ok := caps.BankOf("042"); !ok || bank != spec.BankMemory {
		t.Errorf("slot 042 resolves to %q/%v, want MEM/true", bank, ok)
	}
}
