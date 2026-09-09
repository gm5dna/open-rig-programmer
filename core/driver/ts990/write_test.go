// SPDX-License-Identifier: GPL-3.0-or-later

package ts990

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// probeFrames is what every session sends before anything a test asked for:
// Init's AI0; and the two-frame identity probe (plan P9).
var probeFrames = []string{"AI0;", "ID;", "FV;"}

// writableData is populatedFields' record READ BACK AS NEUTRAL FIELDS, so a
// channel built here and a channel this driver reads from that fixture
// describe the same radio state: 14.175 MHz FM with TONE on, TN index 08 and
// CN index 12, scan lockout off, named "Bench", and — the pair's own gain — a
// Known tx_frequency of 0, which is this record's way of saying "this channel
// stores no independent transmit frequency" (A16, 990:2964-2965).
//
// EVERY OTHER FIELD IS Unavailable, exactly as read.go publishes it: a
// fixture leaving one Absent would be judged by a different rung than the one
// under test (driver.CheckFieldStates' Absent arm), and one setting a value
// the 57-byte record has no position for would be refused by the capability
// gate for a reason no test here is asking about.
//
// A fresh pointer per call, so a test that mutates one cell cannot reach
// another test's fixture.
func writableData() *codeplug.ChannelData {
	return &codeplug.ChannelData{
		FreqHz:   14_175_000,
		Mode:     "FM",
		Tag:      "Bench",
		ScanSkip: codeplug.BoolField{State: codeplug.Known, Value: false},
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: "TONE"},
		ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: ctcssTones[8]},
		ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: ctcssTones[12]},
		TxFreqHz: codeplug.FreqField{State: codeplug.Known, Value: 0},

		CTCSSTone:           codeplug.ToneField{State: codeplug.Unavailable},
		TagDisplay:          codeplug.BoolField{State: codeplug.Unavailable},
		Duplex:              codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz:            codeplug.FreqField{State: codeplug.Unavailable},
		DTCSCode:            codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity:        codeplug.StringField{State: codeplug.Unavailable},
		Filter:              codeplug.StringField{State: codeplug.Unavailable},
		DataMode:            codeplug.BoolField{State: codeplug.Unavailable},
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
		Preamp:              codeplug.StringField{State: codeplug.Unavailable},
		Antenna:             codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},
	}
}

// writableChannel is writableData at slot id.
func writableChannel(id string) codeplug.Channel {
	return codeplug.Channel{Slot: id, Data: writableData()}
}

// writeImage is the scripted radio a write test wants: the target slot
// answers the pre-write MA0 read with frame.
func writeImage(slot, frame string) radioImage {
	return radioImage{ma0Answers: map[string]string{slot: frame}}
}

// assertPositiveControl is P7's pinning half, and it runs under the SAME
// profile as the rung it accompanies: an UNMUTATED channel must succeed TO
// THE WIRE — one MA0 read followed by one MA0 Set, reported Sent.
//
// Without it a semantic pin proves nothing. Every rung below the capability
// gate is pinned on a session that has already passed that gate, and a test
// asserting only "refused" would pass on the gate itself with the rung it
// claims to pin absent from the code.
func assertPositiveControl(t *testing.T, profile Profile, opts ...Option) {
	t.Helper()
	const id = "042"
	sess, p := openSessionAt(t, profile, writeImage(id, populatedMA0(id)), opts...)
	res, err := sess.WriteChannel(context.Background(), writableChannel(id))
	if err != nil {
		t.Fatalf("positive control: WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Sent || res.Steps[0].Confirmed {
		t.Fatalf("positive control: Steps = %+v, want one MA0 step Sent and never Confirmed", res.Steps)
	}
	got := p.Transcript()
	if len(got) != len(probeFrames)+2 {
		t.Fatalf("positive control: transcript = %v, want the probe, one MA0 read and one MA0 Set", got)
	}
	if len(got[len(probeFrames)]) != ma0ReadLen || len(got[len(probeFrames)+1]) != ma0AnswerLen {
		t.Fatalf("positive control: transcript = %v, want a seven-byte read then a 57-byte Set", got)
	}
}

// assertNoSetReached fails unless the transcript carries no MA0 SET at all —
// the property every refusal shares, and the one a refusal arriving too late
// would break.
func assertNoSetReached(t *testing.T, p *respondingPort) {
	t.Helper()
	for _, f := range p.Transcript() {
		if strings.HasPrefix(f, "MA0") && len(f) == ma0AnswerLen {
			t.Fatalf("an MA0 Set reached the wire: %q", f)
		}
	}
}

// assertRegister fails unless err is this package's RefusalError naming
// register — the machine-readable half of "a register entry is part of the
// refusal, not a comment on it".
func assertRegister(t *testing.T, err error, register string) *RefusalError {
	t.Helper()
	var refusal *RefusalError
	if !errors.As(err, &refusal) {
		t.Fatalf("errors.As(*RefusalError) = false for %v", err)
	}
	if refusal.Register != register {
		t.Fatalf("Register = %q, want %q (%v)", refusal.Register, register, err)
	}
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("errors.Is(err, driver.ErrWriteRefused) = false for %v", err)
	}
	return refusal
}

// TestMA0SetSpec_IsFireAndForgetAndNeverRetries pins the three properties of
// the write's transport spec that are load-bearing rather than incidental.
//
// NO Match, and therefore no answer length: on this family's assumed
// convention an accepted Set produces no answer at all (A20/L-HW-3), so a
// spec that waited for one would spend a whole read timeout and then report
// a timeout for a write the radio had accepted. transport.Engine refuses a
// ClassWrite spec carrying a Match outright.
//
// RetryReads 0, NECESSARILY: transport safety obligation 2 forbids resending
// a write, and Do refuses a write-class spec with a non-zero RetryReads
// before writing anything.
func TestMA0SetSpec_IsFireAndForgetAndNeverRetries(t *testing.T) {
	got := ma0SetSpec()
	if got.Class != transport.ClassWrite {
		t.Errorf("ma0SetSpec().Class = %v, want transport.ClassWrite", got.Class)
	}
	if got.Match != nil {
		t.Error("ma0SetSpec() carries a Match; a fire-and-forget write must not wait for an answer")
	}
	if got.RetryReads != 0 {
		t.Errorf("ma0SetSpec().RetryReads = %d, want 0 — a write is never resent", got.RetryReads)
	}
}

// TestRequestedFields_MembershipAndOrder pins the requested-set table against
// this package's own literal list of the twenty-seven spec.Fields (plan P5:
// no Kenwood file names spec.AllFields).
//
// THE TABLE NAMES TWENTY-SIX OF THE TWENTY-SEVEN, AND THE ONE IT OMITS IS
// spec.FieldErase, BY NAME: an erase is not a field a write requests, it is
// the whole shape of a different frame — MA5, which this programme never
// builds (990:3042-3047) — and WriteChannel refuses an empty channel a rung
// above this table.
func TestRequestedFields_MembershipAndOrder(t *testing.T) {
	var want []spec.Field
	for _, f := range allSpecFields {
		if f == spec.FieldErase {
			continue
		}
		want = append(want, f)
	}
	var got []spec.Field
	for _, r := range requestedFieldRules {
		got = append(got, r.field)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("requestedFieldRules names\n %v\nwant the twenty-seven less spec.FieldErase, in that order\n %v", got, want)
	}
}

// TestRequestedFields_TheEightTheRecordAlwaysCarries pins which fields a
// write requests UNCONDITIONALLY, and it is the 57-byte frame's own shape:
// the record has a position for each of them on every write, with no "leave
// it alone" encoding anywhere in the grid (990:2891-2956).
//
// TWO OF THE EIGHT DIVERGE FROM PAIR 1, in opposite directions, which is why
// this is a literal list rather than a count: tx_frequency IS unconditional
// here (P9 with P15 goes out on every Set, as the printed zeroed form when
// the channel is simplex — A16), where the 590 rows made it conditional; and
// data_mode is NOT here (M-E3: no DA command, no data byte, the data-ness is
// inside the mode legend), where the 590 rows wrote a byte for it.
func TestRequestedFields_TheEightTheRecordAlwaysCarries(t *testing.T) {
	want := []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldTag, spec.FieldScanSkip,
		spec.FieldTxFrequency, spec.FieldToneMode, spec.FieldToneTx, spec.FieldToneRx,
	}
	got := requestedFields(codeplug.ChannelData{})
	seen := map[spec.Field]int{}
	for _, f := range got {
		seen[f]++
	}
	for _, f := range want {
		seen[f]--
	}
	for f, n := range seen {
		if n != 0 {
			t.Errorf("requestedFields(zero) and the eight the record always carries disagree about %s", f)
		}
	}
	if len(got) != len(want) {
		t.Errorf("requestedFields(zero) = %v, want %v", got, want)
	}
}

// TestRequestedFields_AFieldWithNoPositionInTheRecordIsNamedWhenACallerSetsIt
// is the conditionals' whole reason: a caller handing this driver a value the
// 57-byte record has no room for — a Known offset from a Yaesu CSV, a Known
// IP+ from an Icom native file — has that field NAMED by the capability gate
// rather than dropped silently from a frame with nowhere to put it.
func TestRequestedFields_AFieldWithNoPositionInTheRecordIsNamedWhenACallerSetsIt(t *testing.T) {
	data := *writableData()
	if got := requestedFields(data); len(got) != 8 {
		t.Fatalf("an ordinary channel requests %v, want the eight alone", got)
	}
	data.OffsetHz = codeplug.FreqField{State: codeplug.Known, Value: 600_000}
	found := false
	for _, f := range requestedFields(data) {
		if f == spec.FieldOffset {
			found = true
		}
	}
	if !found {
		t.Error("a Known offset is not requested, so the capability gate would never name it")
	}
}

// TestWriteChannel_TheCapabilityGateIsTheFirstAnswerOnAnUnconsentedSession is
// P7 rung 5, pinned ONCE and on its own profile: with writeTrialsComplete
// false a RealHardware session's every field is Unverified, CanWrite is
// false, and the gate refuses before any frame — the pre-write read included.
//
// IT IS THE PLAIN FLEET ERROR AND NOT THIS PACKAGE'S RefusalError, which is
// what makes the semantic pins below honest: a rung's pin asserts the typed
// register-bearing error, so it cannot pass on this gate by accident.
func TestWriteChannel_TheCapabilityGateIsTheFirstAnswerOnAnUnconsentedSession(t *testing.T) {
	const id = "042"
	sess, p := openSessionAt(t, RealHardware, writeImage(id, populatedMA0(id)))
	res, err := sess.WriteChannel(context.Background(), writableChannel(id))
	if err == nil {
		t.Fatal("an unconsented RealHardware session accepted a write")
	}
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("errors.As(*driver.WriteRefusedError) = false for %v", err)
	}
	var semantic *RefusalError
	if errors.As(err, &semantic) {
		t.Errorf("the capability gate answered with a semantic register (%q); it must be the plain fleet refusal", semantic.Register)
	}
	if len(refused.Fields) != 8 {
		t.Errorf("Fields = %v, want the eight the record always carries", refused.Fields)
	}
	if len(res.Steps) != 0 {
		t.Errorf("Steps = %v, want none — no frame is built", res.Steps)
	}
	if got := p.Transcript(); !reflect.DeepEqual(got, probeFrames) {
		t.Errorf("transcript = %v, want the probe alone — not even the pre-write read", got)
	}

	// THE HARDER CASE, AND THE ONE THAT PINS THE GATE'S POSITION RATHER THAN
	// MERELY ITS ERROR TYPE. An ordinary channel trips no semantic rung, so a
	// suite asserting only that case cannot distinguish "the gate fires
	// first" from "the gate was hoisted below rungs 6-9" — both answer the
	// same way on a channel rung 8 would wave through. Here the channel ALSO
	// carries rung 6's own fixture (an Unknown tx_frequency) and rung 8's (a
	// Known tone_rx of 1750 Hz); if the gate is first it never reaches either,
	// and if it were moved below rung 6 or rung 8 this would answer
	// registerDecision9 or registerDecision8 instead.
	//
	// RUNG 6 IS THE ONE THAT MATTERS MOST and was the one the earlier fixture
	// did not reach (review s2-close-review-opus-2.md MED-2): it is the rung
	// an ordinary CHIRP import trips, so it is the rung an unconsented session
	// is likeliest to meet first if the order ever slips.
	ch := writableChannel(id)
	ch.Data.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: 17500}
	ch.Data.TxFreqHz = codeplug.FreqField{State: codeplug.Unknown}
	sess2, p2 := openSessionAt(t, RealHardware, writeImage(id, populatedMA0(id)))
	res2, err2 := sess2.WriteChannel(context.Background(), ch)
	if err2 == nil {
		t.Fatal("an unconsented RealHardware session accepted a write for a channel that also trips rungs 6 and 8")
	}
	var refused2 *driver.WriteRefusedError
	if !errors.As(err2, &refused2) {
		t.Fatalf("errors.As(*driver.WriteRefusedError) = false for %v", err2)
	}
	var semantic2 *RefusalError
	if errors.As(err2, &semantic2) {
		t.Errorf("the capability gate answered with a semantic register (%q) for a channel that also trips rungs 6 and 8; it must be the plain fleet refusal", semantic2.Register)
	}
	if len(res2.Steps) != 0 {
		t.Errorf("Steps = %v, want none — no frame is built", res2.Steps)
	}
	if got := p2.Transcript(); !reflect.DeepEqual(got, probeFrames) {
		t.Errorf("transcript = %v, want the probe alone — not even the pre-write read", got)
	}
}

// TestWriteChannel_AnIncoherentFieldIsRefusedNotInterpreted is rung 4, the
// FLEET's FieldState walk (driver.CheckFieldStates) consumed as a black box —
// the sibling row's own table (ts890/refusals_test.go), ported here because
// this row had no case only this rung can answer (review
// s2-close-review-opus-2.md MED-1).
//
// WHAT IT PREVENTS IS SILENT. A value carried alongside a state meaning
// "preserve whatever the radio has" is never named by requestedFields, so
// without this rung it is dropped from the frame and the write reports
// SUCCESS. The ladder's own rung-4 row cannot show that: it carries an
// Unknown scan_skip, and setRecord refuses a non-Known scan_skip on its own
// account one rung further down, so the ladder stayed green with rung 4
// disabled. The FIRST TWO cases here are the value-with-no-claim kind — an
// Unavailable or stateless field carrying a number — which nothing below this
// rung reads, and disabling rung 4 fails both. The third, a Known tone outside
// the printed chart, is also refused by setRecord's own index lookup; it is
// kept for the sibling's shape and because the two refusals name the same
// field with different messages.
func TestWriteChannel_AnIncoherentFieldIsRefusedNotInterpreted(t *testing.T) {
	const id = "042"
	for _, tc := range []struct {
		name   string
		field  spec.Field
		mutate func(*codeplug.ChannelData)
	}{
		{"a tone with a value and no state", spec.FieldCTCSSTone, func(d *codeplug.ChannelData) {
			d.CTCSSTone = codeplug.ToneField{Value: 1000}
		}},
		{"an Unavailable offset carrying a value", spec.FieldOffset, func(d *codeplug.ChannelData) {
			d.OffsetHz = codeplug.FreqField{State: codeplug.Unavailable, Value: 600_000}
		}},
		{"a Known tone outside this row's chart", spec.FieldToneTx, func(d *codeplug.ChannelData) {
			d.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: 1}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ch := writableChannel(id)
			tc.mutate(ch.Data)
			sess, p := openSessionAt(t, Simulated, writeImage(id, populatedMA0(id)))
			_, err := sess.WriteChannel(context.Background(), ch)
			var refused *driver.WriteRefusedError
			if !errors.As(err, &refused) {
				t.Fatalf("err = %v (%T), want a *driver.WriteRefusedError", err, err)
			}
			var semantic *RefusalError
			if errors.As(err, &semantic) {
				t.Errorf("err names register %q: the FieldState walk is the fleet's and carries no register entry", semantic.Register)
			}
			if len(refused.Fields) != 1 || refused.Fields[0] != tc.field {
				t.Errorf("Fields = %v, want [%s]", refused.Fields, tc.field)
			}
			assertNoSetReached(t, p)
		})
	}
}

// TestWriteChannel_ConsentOpensTheGateAndEveryRungBelowItStillFires is the
// other half of the gate: consent widens WHAT may be attempted, never HOW
// carefully.
func TestWriteChannel_ConsentOpensTheGateAndEveryRungBelowItStillFires(t *testing.T) {
	assertPositiveControl(t, RealHardware, WithConsentedUnverifiedWrites())

	const id = "042"
	sess, p := openSessionAt(t, RealHardware, writeImage(id, populatedMA0(id)), WithConsentedUnverifiedWrites())
	ch := writableChannel(id)
	ch.Data.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: 17500}
	_, err := sess.WriteChannel(context.Background(), ch)
	assertRegister(t, err, registerDecision8)
	assertNoSetReached(t, p)
}

// TestWriteChannel_TheLadder walks P7's rungs, each pinned INDIVIDUALLY —
// its own error type, its own register where an assumption or a decision
// produced it, its own quoted citations — and each with its own positive
// control under the same profile.
//
// Rungs 1-9 are locally decidable and NO frame at all may reach the wire;
// rungs 10-11 are read-dependent, so the pre-write MA0 read is expected and
// only the SET must be absent. Rung 5, the capability gate, is pinned
// separately above on the profile P7 names for it.
//
// THREE ERROR SHAPES, AND THE DIFFERENCE IS NOT COSMETIC: an unpublished slot
// is *UnknownSlotError, the same refusal a READ of one earns; a refusal that
// comes from an ASSUMED-register entry or a design decision is *RefusalError
// naming it; and an ordinary refusal over a PRINTED fact — a mode this row
// does not publish, a name wider than the printed window — is the plain fleet
// error, because putting a register entry's name on it would put an
// assumption's name on an ordinary typo.
func TestWriteChannel_TheLadder(t *testing.T) {
	const id = "042"
	populated := populatedMA0(id)

	// A DUAL-RECEPTION answer, and its P2 is '1' for the same reason every
	// live frequency 2 in this file carries one: classFor derives the type
	// from that side alone (990:2901-2903), and it does not consult P16.
	dual := populatedFields(id)
	dual.class = '1'
	dual.dual = '1'
	dual.txFreq, dual.txMode = "00014200000", '4'

	// The three SELF-CONTRADICTORY answers: a flag and the window it
	// describes disagreeing, in both directions for P15 and in the one
	// direction P16 can take.
	splitFlagOnly := populatedFields(id)
	splitFlagOnly.split = '1'

	splitContentOnly := populatedFields(id)
	splitContentOnly.txFreq, splitContentOnly.txMode = "00014200000", '4'

	dualFlagOnly := populatedFields(id)
	dualFlagOnly.dual = '1'

	// The EIGHTH (P15, P16) cell — both flags set, frequency 2 entirely
	// zero — refused by the SAME P16 arm as dualFlagOnly alone: the check is
	// `DualRecv && TXFreqHz == 0` and does not consult Split.
	dualFlagOnlySplit := populatedFields(id)
	dualFlagOnlySplit.dual, dualFlagOnlySplit.split = '1', '1'

	dualAndSplit := populatedFields(id)
	dualAndSplit.class = '1'
	dualAndSplit.dual, dualAndSplit.split = '1', '1'
	dualAndSplit.txFreq, dualAndSplit.txMode = "00014200000", '4'

	// THE SPLIT BASE, MIRRORING ITS PRIMARY SIDE ON THE SECONDARY ONE — the
	// same shape as
	// TestWriteChannel_ASplitChannelRoundTripsWhenTheSetReproducesItsSecondarySide's
	// own f, and the answer every one-byte variant below departs from in
	// EXACTLY ONE byte, so each isolates the ONE secondaryDiffs clause it
	// pins: P10-P14 have no other guard on this row, and a fixture differing
	// in several at once cannot tell one clause's absence from another's.
	//
	// ITS P2 IS '1', WHICH IS THE ONLY CLASS A SPLIT ANSWER CAN CARRY. The
	// chart says the channel type is "decided while setting the P9 and P10
	// values" (990:2901-2903), so a live frequency 2 is not a Single Memory
	// channel and internal/fakets990's classFor derives exactly that. A
	// split fixture with P2 = '0' would be a frame no radio this project
	// models can send, and rung 11 pinned against one would be pinned
	// against nothing (review s2-close-review-opus-1.md MED-1).
	//
	// THE CANDIDATE MUST BE A SPLIT WRITE for the P10-P14 rows (splitCandidate
	// mutates TxFreqHz to Known): it makes setRecord copy the PRIMARY's own
	// mode, width and tones onto the Set's P10-P14 — the mirror this answer
	// matches everywhere but the one byte under test — and it is also what
	// carries the write PAST rung 11's A14 clause, which refuses a simplex
	// candidate over a Dual target before any comparison runs.
	mirroredSecondary := func() ma0Fields {
		f := populatedFields(id)
		f.class = '1'
		f.split = '1'
		f.txFreq, f.txMode, f.txNarrow = "00014200000", f.mode, f.narrow
		f.txToneType, f.txTone, f.txCTCSS = f.toneType, f.tone, f.ctcss
		return f
	}
	splitCandidate := func(d *codeplug.ChannelData) {
		d.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: 14_200_000}
	}

	// The A14 row is that SAME Dual target written by the DEFAULT SIMPLEX
	// candidate: zeroing the frequency-2 side is what would re-type the
	// channel, so this write is refused where the split rows below fall
	// through to the P10-P14 comparison.
	classDual := mirroredSecondary()

	// P2 = '2' is the class no clause can judge and no fake can compose:
	// what a Section-defined channel's frequency-2 side holds is unprinted
	// (A8), so the refusal is unconditional and this answer's own secondary
	// side — the printed zeroed one — is beside the point.
	classSection := populatedFields(id)
	classSection.class = '2'

	secondaryMode := mirroredSecondary()
	secondaryMode.txMode = '3'

	secondaryNarrow := mirroredSecondary()
	secondaryNarrow.txNarrow = '1'

	secondaryToneType := mirroredSecondary()
	secondaryToneType.txToneType = '2'

	secondaryTone := mirroredSecondary()
	secondaryTone.txTone = "09"

	secondaryCTCSS := mirroredSecondary()
	secondaryCTCSS.txCTCSS = "13"

	const (
		kindSlot  = "slot"  // *UnknownSlotError
		kindPlain = "plain" // the plain *driver.WriteRefusedError
	)

	for _, tc := range []struct {
		name string
		rung string
		// answer, when set, is what the target slot answers the pre-write
		// read with; the default is an ordinary populated simplex channel
		// that no Part 1 rung may ever consult.
		answer string
		// mutate edits the writable channel; nil leaves it as it is, and
		// empty says the candidate carries no Data at all.
		mutate func(*codeplug.ChannelData)
		empty  bool
		slot   string
		// kind is kindSlot, kindPlain, or the register name a *RefusalError
		// must carry.
		kind     string
		wants    []string
		field    spec.Field
		readSent bool
	}{{
		name:  "rung 1 — the slot identifier's syntax",
		rung:  "1",
		slot:  "42",
		kind:  kindSlot,
		wants: []string{"three digits", "990:2893-2895"},
	}, {
		name:  "rung 2 — a slot this row publishes nowhere",
		rung:  "2",
		slot:  "105",
		kind:  kindSlot,
		wants: []string{"MEM 000-099"},
	}, {
		name:  "rung 3 — an empty candidate is an erase request",
		rung:  "3",
		slot:  id,
		empty: true,
		kind:  registerDecision15,
		wants: []string{"MA5", "990:3042"},
		field: spec.FieldErase,
	}, {
		name:   "rung 4 — the fleet's FieldState walk",
		rung:   "4",
		slot:   id,
		mutate: func(d *codeplug.ChannelData) { d.ScanSkip = codeplug.BoolField{State: codeplug.Unknown, Value: true} },
		kind:   kindPlain,
		wants:  []string{"scan_skip"},
		field:  spec.FieldScanSkip,
	}, {
		name:   "rung 6 — no Known TX disposition (M-E8, file-side only)",
		rung:   "6",
		slot:   id,
		mutate: func(d *codeplug.ChannelData) { d.TxFreqHz = codeplug.FreqField{State: codeplug.Unavailable} },
		kind:   registerDecision9,
		wants:  []string{"tx_frequency", "990:2927-2948"},
		field:  spec.FieldTxFrequency,
	}, {
		name:   "rung 7 — a mode this row does not publish",
		rung:   "7",
		slot:   id,
		mutate: func(d *codeplug.ChannelData) { d.Mode = "FT8" },
		kind:   kindPlain,
		wants:  []string{"990:3706-3730", "990:3707"},
		field:  spec.FieldMode,
	}, {
		name:   "rung 8 — a Known tone_rx of 1750 Hz",
		rung:   "8",
		slot:   id,
		mutate: func(d *codeplug.ChannelData) { d.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: 17500} },
		kind:   registerDecision8,
		wants:  []string{"990:1251-1264", "49"},
		field:  spec.FieldToneRx,
	}, {
		name:   "rung 9 — a name wider than the printed window",
		rung:   "9",
		slot:   id,
		mutate: func(d *codeplug.ChannelData) { d.Tag = "ELEVENCHARS" },
		kind:   kindPlain,
		wants:  []string{"990:2955-2956", "10"},
		field:  spec.FieldTag,
	}, {
		name:   "rung 9 — a name carrying the terminator",
		rung:   "9",
		slot:   id,
		mutate: func(d *codeplug.ChannelData) { d.Tag = "A;B" },
		kind:   registerA2,
		wants:  []string{"990:96-99"},
		field:  spec.FieldTag,
	}, {
		name:     "rung 10 — the target channel is unassigned",
		rung:     "10",
		answer:   blankMA0(id),
		slot:     id,
		kind:     registerA3,
		wants:    []string{"990:2962-2963", "does not create channels"},
		readSent: true,
	}, {
		name:     "rung 11 — a SIMPLEX candidate would re-type a Dual target (P2 = '1')",
		rung:     "11",
		answer:   classDual.frame(),
		slot:     id,
		kind:     registerA14,
		wants:    []string{"P2", "'1'", "990:2897-2903", "SIMPLEX"},
		readSent: true,
	}, {
		name:     "rung 11 — the target is Section defined (P2 = '2')",
		rung:     "11",
		answer:   classSection.frame(),
		slot:     id,
		kind:     registerA8,
		wants:    []string{"P2", "'2'", "990:2897-2903", "990:3051-3059"},
		readSent: true,
	}, {
		name:     "the answer contradicts itself — P15 says split and frequency 2 is zero",
		rung:     "10a",
		answer:   splitFlagOnly.frame(),
		slot:     id,
		kind:     kindPlain,
		wants:    []string{"P15", "990:2946-2951", "990:2964-2965"},
		readSent: true,
	}, {
		name:     "the answer contradicts itself — P15 says simplex and frequency 2 has content",
		rung:     "10a",
		answer:   splitContentOnly.frame(),
		slot:     id,
		kind:     kindPlain,
		wants:    []string{"P15", "990:2946-2951", "990:2964-2965"},
		readSent: true,
	}, {
		name:     "the answer contradicts itself — P16 says dual reception and frequency 2 is zero",
		rung:     "10a",
		answer:   dualFlagOnly.frame(),
		slot:     id,
		kind:     kindPlain,
		wants:    []string{"P16", "990:2949-2951", "990:2964-2965"},
		readSent: true,
	}, {
		name:     "the answer contradicts itself — P15 AND P16 both say split/dual and frequency 2 is zero",
		rung:     "10a",
		answer:   dualFlagOnlySplit.frame(),
		slot:     id,
		kind:     kindPlain,
		wants:    []string{"P16", "990:2949-2951", "990:2964-2965"},
		readSent: true,
	}, {
		name:     "rung 11 — dual reception is refused unconditionally, whatever P15 says",
		rung:     "11",
		answer:   dual.frame(),
		mutate:   splitCandidate,
		slot:     id,
		kind:     registerDecision9,
		wants:    []string{"P16", "990:2949-2951"},
		readSent: true,
	}, {
		name:     "rung 11 — dual reception is refused even alongside a split",
		rung:     "11",
		answer:   dualAndSplit.frame(),
		mutate:   splitCandidate,
		slot:     id,
		kind:     registerDecision9,
		wants:    []string{"P16", "990:2949-2951"},
		readSent: true,
	}, {
		name:     "rung 11 — P10, the mode for frequency 2, differs alone",
		rung:     "11",
		answer:   secondaryMode.frame(),
		mutate:   splitCandidate,
		slot:     id,
		kind:     registerDecision9,
		wants:    []string{"P10", "'3'", "'4'", "990:2929-2945"},
		readSent: true,
	}, {
		name:     "rung 11 — P11, FM wide/narrow for frequency 2, differs alone",
		rung:     "11",
		answer:   secondaryNarrow.frame(),
		mutate:   splitCandidate,
		slot:     id,
		kind:     registerDecision9,
		wants:    []string{"P11", "'1'", "'0'", "990:2929-2945"},
		readSent: true,
	}, {
		name:     "rung 11 — P12, the tone function for frequency 2, differs alone",
		rung:     "11",
		answer:   secondaryToneType.frame(),
		mutate:   splitCandidate,
		slot:     id,
		kind:     registerDecision9,
		wants:    []string{"P12", "'2'", "'1'", "990:2929-2945"},
		readSent: true,
	}, {
		name:     "rung 11 — P13, the tone frequency for frequency 2, differs alone",
		rung:     "11",
		answer:   secondaryTone.frame(),
		mutate:   splitCandidate,
		slot:     id,
		kind:     registerDecision9,
		wants:    []string{"P13", "09", "08", "990:2929-2945"},
		readSent: true,
	}, {
		name:     "rung 11 — P14, the CTCSS frequency for frequency 2, differs alone",
		rung:     "11",
		answer:   secondaryCTCSS.frame(),
		mutate:   splitCandidate,
		slot:     id,
		kind:     registerDecision9,
		wants:    []string{"P14", "13", "12", "990:2929-2945"},
		readSent: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			answer := tc.answer
			if answer == "" {
				answer = populated
			}
			assertFrameWidth(t, answer)
			sess, p := openSessionAt(t, Simulated, writeImage(id, answer))

			ch := codeplug.Channel{Slot: tc.slot}
			if !tc.empty {
				ch.Data = writableData()
				if tc.mutate != nil {
					tc.mutate(ch.Data)
				}
			}

			res, err := sess.WriteChannel(context.Background(), ch)
			if err == nil {
				t.Fatalf("rung %s accepted the write", tc.rung)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want none — the Set was never built", res.Steps)
			}
			var refused *driver.WriteRefusedError
			switch tc.kind {
			case kindSlot:
				var unknown *UnknownSlotError
				if !errors.As(err, &unknown) {
					t.Fatalf("errors.As(*UnknownSlotError) = false for %v", err)
				}
			case kindPlain:
				if !errors.As(err, &refused) {
					t.Fatalf("errors.As(*driver.WriteRefusedError) = false for %v", err)
				}
				var semantic *RefusalError
				if errors.As(err, &semantic) {
					t.Errorf("a printed-fact refusal named register %q; it must be the plain fleet error", semantic.Register)
				}
			default:
				refused = &assertRegister(t, err, tc.kind).WriteRefusedError
			}
			if refused != nil && tc.field != "" {
				if len(refused.Fields) != 1 || refused.Fields[0] != tc.field {
					t.Errorf("Fields = %v, want [%s]", refused.Fields, tc.field)
				}
			}
			for _, want := range tc.wants {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("message does not quote %q:\n %v", want, err)
				}
			}
			assertNoSetReached(t, p)
			want := len(probeFrames)
			if tc.readSent {
				want++
			}
			if got := p.Transcript(); len(got) != want {
				t.Errorf("transcript = %v, want %d frames", got, want)
			}

			assertPositiveControl(t, Simulated)
		})
	}
}

// TestWriteChannel_SendsOneReadAndOneSetAndReportsSentNeverConfirmed is the
// choreography pin (P13): TWO frames on the wire per channel write, one of
// them mutating, and no read-back verification — that pair is core/clone's,
// and WriteChannel never calls ReadChannel.
//
// THE SET IS BYTE-IDENTICAL TO THE ANSWER THE RADIO GAVE, which is a real
// property of this row rather than a coincidence of the fixture: this book
// draws ONE grid for Set and Answer (990:2893-2915 against 990:2919-2938), so
// writing back an unedited class-'0' channel reproduces its own answer byte
// for byte.
func TestWriteChannel_SendsOneReadAndOneSetAndReportsSentNeverConfirmed(t *testing.T) {
	const id = "042"
	sess, p := openSessionAt(t, Simulated, writeImage(id, populatedMA0(id)))
	res, err := sess.WriteChannel(context.Background(), writableChannel(id))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	want := append(append([]string(nil), probeFrames...), "MA0"+id+";", populatedMA0(id))
	if got := p.Transcript(); !reflect.DeepEqual(got, want) {
		t.Fatalf("transcript =\n %v\nwant\n %v", got, want)
	}
	if len(res.Steps) != 1 || res.Steps[0].Command != "MA0" {
		t.Fatalf("Steps = %+v, want one MA0 step", res.Steps)
	}
	if !res.Steps[0].Sent {
		t.Error("Steps[0].Sent = false, want true")
	}
	if res.Steps[0].Confirmed {
		t.Error("Steps[0].Confirmed = true; silence is inconclusive on this family (A6) and this driver never claims otherwise")
	}
}

// TestWriteChannel_AFreshReadChannelIsNeverRefusedForItsTXDisposition is
// M-E8's second pin, and the difference between this pair and pair 1's
// blanket obstruction: one MA0 answer carries both frequencies and the split
// flag, so a channel this programme READ has a Known disposition by
// construction and rung 6 cannot fire on it.
func TestWriteChannel_AFreshReadChannelIsNeverRefusedForItsTXDisposition(t *testing.T) {
	const id = "042"
	sess, _ := openSessionAt(t, Simulated, writeImage(id, populatedMA0(id)))
	ch, err := sess.ReadChannel(context.Background(), id)
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data.TxFreqHz.State != codeplug.Known {
		t.Fatalf("a fresh read produced tx_frequency %q, want Known", ch.Data.TxFreqHz.State)
	}
	if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("writing back a channel this driver read: %v", err)
	}
}

// TestWriteChannel_ASplitChannelRoundTripsWhenTheSetReproducesItsSecondarySide
// is §2.4's gain exercised end to end: a split memory is READ with its
// transmit frequency and written back unrefused, because what the Set emits
// for P10-P14 is what the radio already holds.
//
// THE ANSWER CARRIES P2 = '1', WHICH IS THE ONLY CLASS A SPLIT CHANNEL CAN
// ANSWER WITH: the chart says the type is "decided while setting the P9 and
// P10 values" (990:2901-2903), so a live frequency 2 is not a Single Memory
// channel and internal/fakets990's classFor derives exactly that. A fixture
// pairing a live frequency 2 with P2 = '0' would be a state no radio this
// project models can reach, and pinning the round trip on it would have made
// this test's own claim — "§2.4's gain exercised end to end" — untrue
// (review s2-close-review-opus-1.md MED-1).
//
// THE SET IS THE ANSWER WITH P2 NORMALISED TO '0' AND NOTHING ELSE CHANGED.
// core/kw/ma emits the dummy class on every build because the book says the
// parameter "is ignored" and names no value (A14), and the radio re-derives
// the type from P9/P10 either way — so the one byte that differs is the one
// byte the book says it may.
func TestWriteChannel_ASplitChannelRoundTripsWhenTheSetReproducesItsSecondarySide(t *testing.T) {
	const id = "017"
	f := populatedFields(id)
	f.class = '1'
	f.split = '1'
	f.txFreq, f.txMode, f.txNarrow = "00014200000", f.mode, f.narrow
	f.txToneType, f.txTone, f.txCTCSS = f.toneType, f.tone, f.ctcss
	assertFrameWidth(t, f.frame())

	want := f
	want.class = '0'

	sess, p := openSessionAt(t, Simulated, writeImage(id, f.frame()))
	ch, err := sess.ReadChannel(context.Background(), id)
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if got := ch.Data.TxFreqHz; got.State != codeplug.Known || got.Value != 14_200_000 {
		t.Fatalf("tx_frequency = %+v, want Known 14200000 (P15 = '1', 990:2946-2948)", got)
	}
	if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("writing back a split channel: %v", err)
	}
	got := p.Transcript()
	if len(got) != len(probeFrames)+3 {
		t.Fatalf("transcript = %v, want the read's MA0, the write's MA0 read and its Set", got)
	}
	if set := got[len(got)-1]; set != want.frame() {
		t.Errorf("Set =\n %q\nwant the channel's own answer with P2 normalised to '0'\n %q", set, want.frame())
	}

	// AND THE EDIT ITSELF, which is the capability secondaryDiffs' doc claims
	// for P9 and P15: frequency 2 HAS a home in the neutral model, so moving
	// a split channel's transmit frequency is a change this row can carry —
	// the five bytes with no home (P10-P14) are untouched and the comparison
	// passes.
	ch.Data.TxFreqHz.Value = 14_250_000
	if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("changing a split channel's transmit frequency: %v", err)
	}
	edited := want
	edited.txFreq = "00014250000"
	if set := p.Transcript()[len(p.Transcript())-1]; set != edited.frame() {
		t.Errorf("Set =\n %q\nwant the same record with P9 moved\n %q", set, edited.frame())
	}
}

// TestWriteChannel_ARejectionIsAttributableAndSilenceIsNot: a "?;" inside the
// bounded window says the frame provably went out and the radio provably
// refused it, so Sent is true and the typed kw.RejectionError names this
// book's two indistinguishable causes.
func TestWriteChannel_ARejectionIsAttributableAndSilenceIsNot(t *testing.T) {
	const id = "042"
	img := writeImage(id, populatedMA0(id))
	img.ma0SetReject = true
	sess, _ := openSessionAt(t, Simulated, img)
	res, err := sess.WriteChannel(context.Background(), writableChannel(id))
	if err == nil {
		t.Fatal("a rejected Set reported success")
	}
	var rej *kw.RejectionError
	if !errors.As(err, &rej) {
		t.Fatalf("errors.As(*kw.RejectionError) = false for %v", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Sent {
		t.Errorf("Steps = %+v, want one step with Sent true — the frame provably went out", res.Steps)
	}
	if res.Steps[0].Confirmed {
		t.Error("Steps[0].Confirmed = true on a rejection")
	}
}

// TestWriteChannel_ASilentPreWriteReadFailsTheWriteBeforeAnythingMutates: the
// read is what the two read-dependent rungs stand on, so a radio that does
// not answer it fails the write WHOLE — with the typed kw.TimeoutError, which
// says in as many words that silence is not an inference of absence — and
// nothing mutating goes out.
func TestWriteChannel_ASilentPreWriteReadFailsTheWriteBeforeAnythingMutates(t *testing.T) {
	const id = "042"
	sess, p := openSessionAt(t, Simulated, radioImage{ma0Silent: map[string]bool{id: true}})
	res, err := sess.WriteChannel(context.Background(), writableChannel(id))
	if err == nil {
		t.Fatal("a write whose pre-write read drew nothing reported success")
	}
	var to *kw.TimeoutError
	if !errors.As(err, &to) {
		t.Fatalf("errors.As(*kw.TimeoutError) = false for %v", err)
	}
	if len(res.Steps) != 0 {
		t.Errorf("Steps = %+v, want none — the Set was never built", res.Steps)
	}
	assertNoSetReached(t, p)
}

// TestWriteChannel_IsAtomicUnderOpMu is P13's concurrency pin, and its shape
// is the point: the hook parks the write BETWEEN its pre-write read and its
// Set, which is the exact gap a dropped lock would open. A concurrent
// operation landing there would decide against one radio state and write
// against another.
//
// Made deterministic by a test-only hook rather than by hammering, for
// readChannelGapHook's own reason.
func TestWriteChannel_IsAtomicUnderOpMu(t *testing.T) {
	const id = "042"
	sess, p := openSessionAt(t, Simulated, radioImage{ma0Answers: map[string]string{
		id:    populatedMA0(id),
		"007": populatedMA0("007"),
	}})

	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	writeGapHook = func() {
		entered <- struct{}{}
		<-release
	}
	t.Cleanup(func() { writeGapHook = nil })

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := sess.WriteChannel(context.Background(), writableChannel(id)); err != nil {
			t.Errorf("parked WriteChannel: %v", err)
		}
	}()
	<-entered // the write is inside the lock, its read done and its Set not yet sent

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
		t.Fatal("a second operation completed while a write held opMu between its read and its Set")
	case <-time.After(250 * time.Millisecond):
	}
	if got := p.Transcript(); len(got) != len(probeFrames)+1 {
		t.Errorf("transcript while the write is parked = %v, want the probe and the write's own MA0 read alone", got)
	}

	close(release)
	wg.Wait()

	want := append(append([]string(nil), probeFrames...), "MA0"+id+";", populatedMA0(id), "MA0007;")
	if got := p.Transcript(); !reflect.DeepEqual(got, want) {
		t.Errorf("transcript =\n %v\nwant the write's two frames before the read's one\n %v", got, want)
	}
}

// TestWriteChannel_ThereIsNoEightNinetyOnlyRung records a NEGATIVE as a
// decision. This book prints no P4/P10 agreement sentence — the sibling's has
// no counterpart here — so this ladder carries no rung for one, and what
// governs the secondary side on this row is rung 11's comparison against the
// radio's own answer. The pin is that an ordinary FM-Narrow write, whose
// secondary side is the printed zeroed form, is accepted rather than measured
// against its own primary.
func TestWriteChannel_ThereIsNoEightNinetyOnlyRung(t *testing.T) {
	const id = "042"
	sess, p := openSessionAt(t, Simulated, writeImage(id, populatedMA0(id)))
	ch := writableChannel(id)
	ch.Data.Mode = "FM-N"
	if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("an FM-Narrow write was refused: %v", err)
	}
	set := p.Transcript()[len(probeFrames)+1]
	if set[19] != '1' {
		t.Errorf("the Set's P5 (byte 20) is %q, want '1' — FM-N is the width flag, not a mode byte (990:2912-2914)", set[19])
	}
}
