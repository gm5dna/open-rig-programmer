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
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// assertNoWireTraffic fails unless the only frames this session ever sent are
// the probe's three. EVERY refusal in this file is a PRE-WIRE refusal, and the
// transcript is the only thing that can prove it: an error returned after a
// frame went out is a different — and much worse — event than the same error
// returned before one was built. On this row it is the strongest single
// statement the write path makes, because NO frame is ever built at all.
func assertNoWireTraffic(t *testing.T, p *respondingPort, what string) {
	t.Helper()
	if got := p.Transcript(); !reflect.DeepEqual(got, probeFrames) {
		t.Errorf("%s: transcript = %v, want the probe's three frames and nothing more", what, got)
	}
}

// writableChannel is an ordinary channel this row's capability table grades
// entirely writable: every field it names is one of the five the record
// expresses, and every field it does not is Unavailable.
//
// IT IS THE CHANNEL EVERY POSITIVE CONTROL WOULD USE, and on this row there is
// no positive control to be had: A22 refuses it too. That is the point of the
// row's whole write path, and TestWriteChannel_TheA22PinIsNotVacuous is what
// stops "every write is refused" being trivially true — the SAME Simulated
// session reads the SAME slot back and gets a populated channel.
func writableChannel(id, toneMode string) codeplug.Channel {
	return codeplug.Channel{Slot: id, Data: &codeplug.ChannelData{
		FreqHz:   145_500_000,
		Mode:     "FM",
		Tag:      "SIMPLEX",
		ScanSkip: codeplug.BoolField{State: codeplug.Known, Value: false},
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: toneMode},
		// EVERY OTHER FIELD IS UNAVAILABLE, which on this row is most of
		// them: tone_tx and tone_rx have no chart in this book, data_mode has
		// no position at all, filter is a printed constant, tuning_step is
		// refused, and tx_frequency is Unsupported on the whole row.
		ToneTx:              codeplug.ToneField{State: codeplug.Unavailable},
		ToneRx:              codeplug.ToneField{State: codeplug.Unavailable},
		DataMode:            codeplug.BoolField{State: codeplug.Unavailable},
		Filter:              codeplug.StringField{State: codeplug.Unavailable},
		TuningStep:          codeplug.StringField{State: codeplug.Unavailable},
		TuningStepEnabled:   codeplug.BoolField{State: codeplug.Unavailable},
		ProgramTuningStepHz: codeplug.FreqField{State: codeplug.Unavailable},
		TxFreqHz:            codeplug.FreqField{State: codeplug.Unavailable},
		Duplex:              codeplug.StringField{State: codeplug.Unavailable},
		OffsetHz:            codeplug.FreqField{State: codeplug.Unavailable},
		TagDisplay:          codeplug.BoolField{State: codeplug.Unavailable},
		CTCSSTone:           codeplug.ToneField{State: codeplug.Unavailable},
		DTCSCode:            codeplug.IntField{State: codeplug.Unavailable},
		DTCSPolarity:        codeplug.StringField{State: codeplug.Unavailable},
		AttenuatorDB:        codeplug.IntField{State: codeplug.Unavailable},
		Preamp:              codeplug.StringField{State: codeplug.Unavailable},
		Antenna:             codeplug.StringField{State: codeplug.Unavailable},
		IPPlus:              codeplug.BoolField{State: codeplug.Unavailable},
	}}
}

// TestRequestedFields_MembershipAndOrder pins the table against this package's
// own literal field list (P5), less spec.FieldErase — which is not a field a
// write requests but the whole shape of a DIFFERENT frame, and one this radio
// has no route to at all (§2.8: the only "clear" in the book is RC,
// 480:1205).
func TestRequestedFields_MembershipAndOrder(t *testing.T) {
	var want []spec.Field
	for _, f := range allSpecFields {
		if f == spec.FieldErase {
			continue
		}
		want = append(want, f)
	}
	if len(requestedFieldRules) != len(want) {
		t.Fatalf("requestedFieldRules has %d entries, want %d (the twenty-seven less FieldErase)", len(requestedFieldRules), len(want))
	}
	for i, r := range requestedFieldRules {
		if r.field != want[i] {
			t.Errorf("requestedFieldRules[%d] = %s, want %s — the table follows allSpecFields' own order", i, r.field, want[i])
		}
	}
}

// TestRequestedFields_TheFiveTheRecordAlwaysCarries is the difference from the
// TS-590 pair's eight, and every one of the three missing is a byte §5's
// divergence table accounts for.
//
// THE FIVE UNCONDITIONAL ONES ARE EXACTLY THE FIVE THIS ROW GRADES, and that
// is not a coincidence but the design: the 50-byte record carries a frequency,
// a mode, a lockout, a tone mode and a name on EVERY write with no "leave it
// alone" encoding anywhere in the grid, so a write requests all five whether
// the caller edited them or not — which is what makes the capability gate a
// real gate.
//
// THE THREE THE 590 PAIR CARRY UNCONDITIONALLY AND THIS ROW DOES NOT:
//
//   - tone_tx and tone_rx. The record really does carry P8 and P9 on every
//     write (480:966, 480:969), but this row publishes NO TONE DOMAIN — the
//     charts are not in this book (480:1559-1560, 480:339-340) — so naming
//     them unconditionally would make the CAPABILITY GATE refuse every write
//     before A22 could, and A22 would never be reached on any profile. That
//     would be the vacuity plan P7's H2 exists to prevent, one rung lower
//     down: "every write is refused" would be true for the wrong reason.
//     Naming them only when a value is present is what leaves A22 the answer
//     for an ordinary channel and still refuses a caller who hands this driver
//     a tone it cannot address.
//   - data_mode. Byte 19 is the LOCKOUT here (480:962), so there is no
//     data-mode position to carry.
func TestRequestedFields_TheFiveTheRecordAlwaysCarries(t *testing.T) {
	blank := codeplug.ChannelData{}
	want := []spec.Field{
		spec.FieldFrequency, spec.FieldMode, spec.FieldTag,
		spec.FieldScanSkip, spec.FieldToneMode,
	}
	if got := requestedFields(blank); !reflect.DeepEqual(got, want) {
		t.Errorf("requestedFields(zero) = %v, want %v", got, want)
	}
	// The five unconditional ones are the five the table grades.
	graded := map[spec.Field]bool{}
	for _, f := range auditedFields() {
		graded[f] = true
	}
	for _, f := range want {
		if !graded[f] {
			t.Errorf("%s is requested unconditionally but is not one of the five this row grades", f)
		}
	}
	if len(want) != len(graded) {
		t.Errorf("%d fields are requested unconditionally and %d are graded; on this row the two sets are the same", len(want), len(graded))
	}
}

// TestRequestedFields_EveryConditionalIsReachable: a conditional whose
// predicate no channel can satisfy is a rung that never fires, which is
// indistinguishable from a rung that does not exist.
func TestRequestedFields_EveryConditionalIsReachable(t *testing.T) {
	for _, r := range requestedFieldRules {
		if r.present(codeplug.ChannelData{}) {
			continue // unconditional; covered above
		}
		d := codeplug.ChannelData{}
		switch r.field {
		case spec.FieldClarifier:
			d.ClarHz = 100
		case spec.FieldCTCSSState:
			d.CTCSS = "ON"
		case spec.FieldCTCSSTone:
			d.CTCSSTone = codeplug.ToneField{State: codeplug.Known, Value: 885}
		case spec.FieldShift:
			d.Shift = "+"
		case spec.FieldTagDisplay:
			d.TagDisplay = codeplug.BoolField{State: codeplug.Known}
		case spec.FieldTxFrequency:
			d.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: 1}
		case spec.FieldDuplex:
			d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "SIMPLEX"}
		case spec.FieldOffset:
			d.OffsetHz = codeplug.FreqField{State: codeplug.Known, Value: 1}
		case spec.FieldToneTx:
			d.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: 885}
		case spec.FieldToneRx:
			d.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: 885}
		case spec.FieldDTCSCode:
			d.DTCSCode = codeplug.IntField{State: codeplug.Known, Value: 23}
		case spec.FieldDTCSPolarity:
			d.DTCSPolarity = codeplug.StringField{State: codeplug.Known, Value: "NN"}
		case spec.FieldFilter:
			d.Filter = codeplug.StringField{State: codeplug.Known, Value: "FILTER A"}
		case spec.FieldDataMode:
			d.DataMode = codeplug.BoolField{State: codeplug.Known}
		case spec.FieldTuningStepEnabled:
			d.TuningStepEnabled = codeplug.BoolField{State: codeplug.Known}
		case spec.FieldTuningStep:
			d.TuningStep = codeplug.StringField{State: codeplug.Known, Value: "5 kHz"}
		case spec.FieldProgramTuningStep:
			d.ProgramTuningStepHz = codeplug.FreqField{State: codeplug.Known, Value: 1}
		case spec.FieldAttenuator:
			d.AttenuatorDB = codeplug.IntField{State: codeplug.Known, Value: 12}
		case spec.FieldPreamp:
			d.Preamp = codeplug.StringField{State: codeplug.Known, Value: "ON"}
		case spec.FieldAntenna:
			d.Antenna = codeplug.StringField{State: codeplug.Known, Value: "1"}
		case spec.FieldIPPlus:
			d.IPPlus = codeplug.BoolField{State: codeplug.Known}
		default:
			t.Fatalf("%s is conditional and this test has no case for it", r.field)
		}
		if !r.present(d) {
			t.Errorf("%s's predicate is unreachable: no value this test could set makes it fire", r.field)
		}
	}
}

// TestWriteChannel_TheCapabilityGateAnswersFirstOnUnconsentedRealHardware is
// the ONE rung pinned on the RealHardware profile, and it is pinned SEPARATELY
// for that reason (plan P7's H2).
//
// The zero Profile IS RealHardware by fail-safe and writeTrialsComplete is
// false, so every field of an unconsented real-hardware session is Unverified,
// spec.FieldSupport.CanWrite is false for all of them, and the capability gate
// is the first answer for EVERY write. It returns the PLAIN fleet error, so a
// test that asserted only "refused" would pass here and A22 need not exist at
// all — which is the one thing this row's entire write path is for.
func TestWriteChannel_TheCapabilityGateAnswersFirstOnUnconsentedRealHardware(t *testing.T) {
	sess, p := openSession(t, RealHardware, radioImage{})
	_, err := sess.WriteChannel(context.Background(), writableChannel("42", "OFF"))
	var fleet *driver.WriteRefusedError
	if !errors.As(err, &fleet) {
		t.Fatalf("err = %v (%T), want *driver.WriteRefusedError", err, err)
	}
	var semantic *RefusalError
	if errors.As(err, &semantic) {
		t.Errorf("the capability gate returned a SEMANTIC refusal (%s); the two must stay distinguishable, or A22's own pins are satisfied by the gate", semantic.Register)
	}
	if len(fleet.Fields) == 0 {
		t.Error("the capability refusal names no field")
	}
	assertNoWireTraffic(t, p, "unconsented RealHardware")
}

// TestWriteChannel_A22RefusesEveryWriteOnASessionPastTheGate is the row's
// central claim, pinned on a session that has ALREADY PASSED the capability
// gate — the SIMULATED profile — so that the refusal is A22's and not the
// gate's (plan P7's H2, and the task's own "here it matters MORE, not less").
//
// ONE TYPED REFUSAL, TWO CAUSES, AND THE Q2 RELATIONSHIP IS TESTED THROUGH THE
// ERROR'S FIELDS (Codex HIGH 2, spec §Error handling's corrected paragraph).
// A22 rejects every TS-480 channel write BY DEFINITION, so a standalone Q2
// rung placed after it could never execute on any profile and the two
// instructions could not both be obeyed. Instead: the refusal's primary cause
// is ALWAYS A22, and it ADDITIONALLY carries Q2 when the source channel's
// tone_mode is not OFF. The difference is real and falsifiable — an
// implementation that forgot Q2 fails the second row of this table — and it
// does not require Q2 ever to be reached first.
func TestWriteChannel_A22RefusesEveryWriteOnASessionPastTheGate(t *testing.T) {
	for _, tc := range []struct {
		toneMode string
		causes   []string
	}{
		{"OFF", []string{registerA22}},
		{"TONE", []string{registerA22, registerQ2}},
		{"CTCSS", []string{registerA22, registerQ2}},
	} {
		sess, p := openSession(t, Simulated, radioImage{})
		res, err := sess.WriteChannel(context.Background(), writableChannel("42", tc.toneMode))

		var ref *RefusalError
		if !errors.As(err, &ref) {
			t.Fatalf("tone_mode %q: err = %v (%T), want *RefusalError", tc.toneMode, err, err)
		}
		if ref.Register != registerA22 {
			t.Errorf("tone_mode %q: primary cause = %q, want %q — A22 is the primary cause of EVERY TS-480 write refusal", tc.toneMode, ref.Register, registerA22)
		}
		if !reflect.DeepEqual(ref.Causes, tc.causes) {
			t.Errorf("tone_mode %q: causes = %v, want %v", tc.toneMode, ref.Causes, tc.causes)
		}
		for _, cause := range tc.causes {
			if !strings.Contains(ref.Reason, cause) {
				t.Errorf("tone_mode %q: reason %q does not name %s", tc.toneMode, ref.Reason, cause)
			}
		}
		if len(tc.causes) == 1 && strings.Contains(ref.Reason, registerQ2) {
			t.Errorf("tone_mode OFF: reason %q names Q2, which is a cause only when a tone is requested", ref.Reason)
		}

		// The neutral seam still sees a write refusal, three ways.
		if !errors.Is(err, driver.ErrWriteRefused) {
			t.Errorf("tone_mode %q: the refusal does not match driver.ErrWriteRefused", tc.toneMode)
		}
		var fleet *driver.WriteRefusedError
		if !errors.As(err, &fleet) {
			t.Errorf("tone_mode %q: the refusal is not recoverable as *driver.WriteRefusedError", tc.toneMode)
		} else if fleet.Slot != "42" {
			t.Errorf("tone_mode %q: the refusal names slot %q", tc.toneMode, fleet.Slot)
		}

		// NO FRAME IS BUILT, and the step list is an EXPLICITLY EMPTY slice
		// rather than nil: the clone service journals this result, and a nil
		// slice marshals as JSON null, which an auditor would have to read
		// as "unknown" rather than the truth, "no frame was ever built".
		if res.Steps == nil {
			t.Errorf("tone_mode %q: Steps is nil, want an explicitly empty slice", tc.toneMode)
		}
		if len(res.Steps) != 0 {
			t.Errorf("tone_mode %q: Steps = %v, want empty — no frame is ever built on this row", tc.toneMode, res.Steps)
		}
		assertNoWireTraffic(t, p, "Simulated, tone_mode "+tc.toneMode)
	}
}

// TestWriteChannel_TheA22PinIsNotVacuous is the control the task names in
// terms: the SAME Simulated session's READ of the SAME slot succeeds and
// returns a populated channel.
//
// Without it, "every write is refused on the Simulated profile" would be
// indistinguishable from "this session cannot do anything at all" — a broken
// probe, a dead pipe, a capability set that grades nothing. There is no
// positive WRITE control available on this row and there cannot be one: A22
// refuses every write by definition. The read is the control that exists.
func TestWriteChannel_TheA22PinIsNotVacuous(t *testing.T) {
	sess, _ := openSession(t, Simulated, radioImage{
		mrAnswers: map[string]string{mrAddr("42"): populatedMR("42")},
	})
	if _, err := sess.WriteChannel(context.Background(), writableChannel("42", "OFF")); err == nil {
		t.Fatal("a write succeeded on a TS-480; A22 refuses every channel write (decision 12)")
	}
	ch, err := sess.ReadChannel(context.Background(), "42")
	if err != nil {
		t.Fatalf("the control read of the same slot failed: %v — the A22 pin above would then be vacuous", err)
	}
	if ch.Data == nil {
		t.Fatal("the control read returned an empty channel; it must return a populated one")
	}
	if ch.Data.FreqHz != 145_500_000 {
		t.Errorf("the control read returned FreqHz %d, want a populated record", ch.Data.FreqHz)
	}
}

// TestWriteChannel_ConsentReachesA22AndNoFurther is the other half of H2's
// profile discipline: consent is the route THROUGH the capability gate on a
// RealHardware session, and what waits on the other side of it is A22 rather
// than success.
//
// It is also what makes WithConsentedUnverifiedWrites worth having on a row
// where no write can ever succeed: without it there would be no way to reach
// A22 on a real-hardware session at all, and "every write is refused" would be
// satisfied vacuously by the gate.
func TestWriteChannel_ConsentReachesA22AndNoFurther(t *testing.T) {
	sess, p := openSession(t, RealHardware, radioImage{}, WithConsentedUnverifiedWrites())
	_, err := sess.WriteChannel(context.Background(), writableChannel("42", "OFF"))
	var ref *RefusalError
	if !errors.As(err, &ref) {
		t.Fatalf("err = %v (%T), want *RefusalError — consent must pass the gate and meet A22", err, err)
	}
	if ref.Register != registerA22 {
		t.Errorf("primary cause = %q, want %q", ref.Register, registerA22)
	}
	assertNoWireTraffic(t, p, "consented RealHardware")
}

// TestWriteChannel_TheLadderRefusesBeforeItReachesA22 walks the rungs ABOVE
// A22 in P7's order, each on a session that has passed the capability gate, so
// that each is answered by its own rung and not by the row's blanket refusal.
//
// Each asserts the refusal is NOT an *RefusalError: A22's type is what
// distinguishes "this radio cannot be written at all" from "this particular
// channel is malformed", and a caller that could not tell them apart would
// report an unlifted assumption for a caller's own mistake.
func TestWriteChannel_TheLadderRefusesBeforeItReachesA22(t *testing.T) {
	empty := codeplug.Channel{Slot: "42"}
	incoherent := writableChannel("42", "OFF")
	// An Unknown state carrying a value is the malformation the fleet's
	// FieldState walk exists to catch: a value alongside a state meaning
	// "preserve whatever the radio has" would otherwise be dropped silently.
	incoherent.Data.ScanSkip = codeplug.BoolField{State: codeplug.Unknown, Value: true}
	// A field this row does not publish, carrying a value: the capability
	// gate's own case, and on this row it is a real one — a channel read off
	// a TS-590SG and cloned here brings a filter with it.
	foreign := writableChannel("42", "OFF")
	foreign.Data.Filter = codeplug.StringField{State: codeplug.Known, Value: "FILTER A"}

	for _, tc := range []struct {
		name  string
		ch    codeplug.Channel
		field spec.Field
	}{
		{"an unknown slot", writableChannel("042", "OFF"), ""},
		{"an erase", empty, spec.FieldErase},
		{"an incoherent field state", incoherent, spec.FieldScanSkip},
		{"a field this row does not publish", foreign, spec.FieldFilter},
	} {
		sess, p := openSession(t, Simulated, radioImage{})
		_, err := sess.WriteChannel(context.Background(), tc.ch)
		if err == nil {
			t.Fatalf("%s: the write succeeded", tc.name)
		}
		var ref *RefusalError
		if errors.As(err, &ref) {
			t.Errorf("%s: answered by A22 (%v), want its own rung above it", tc.name, ref.Causes)
		}
		if tc.field != "" {
			var fleet *driver.WriteRefusedError
			if !errors.As(err, &fleet) {
				t.Fatalf("%s: err = %v (%T), want *driver.WriteRefusedError", tc.name, err, err)
			}
			if !reflect.DeepEqual(fleet.Fields, []spec.Field{tc.field}) {
				t.Errorf("%s: the refusal names %v, want [%s]", tc.name, fleet.Fields, tc.field)
			}
		} else if !errors.Is(err, ErrUnknownSlot) {
			t.Errorf("%s: err = %v, want ErrUnknownSlot", tc.name, err)
		}
		assertNoWireTraffic(t, p, tc.name)
	}
}

// TestWriteChannel_TheEraseRungNamesThisRadiosOwnAbsence: the 590 pair refuse
// an erase because the only route printed is a short MW of ambiguous width
// (A5, erratum E19). THIS RADIO HAS NO ERASE ROUTE AT ALL — the only "clear"
// in the whole book is RC, "Clears the RIT offset frequency" (480:1205) — so
// the refusal here must not borrow the 590's reasoning, which would describe a
// sentence this book does not print.
func TestWriteChannel_TheEraseRungNamesThisRadiosOwnAbsence(t *testing.T) {
	sess, _ := openSession(t, Simulated, radioImage{})
	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "42"})
	if err == nil {
		t.Fatal("an empty channel was accepted")
	}
	if strings.Contains(err.Error(), "590:") {
		t.Errorf("the erase refusal quotes the 590 book: %v — this radio has no erase route at all (480:1205)", err)
	}
	if !strings.Contains(err.Error(), "480:1205") {
		t.Errorf("the erase refusal does not cite this book's own absence: %v", err)
	}
}

// TestWriteChannel_IsAtomicUnderOpMu is P13/P14's concurrency pin for the
// write path, and it uses the READ path's hook rather than one of its own: the
// rule is "ONE DRIVER OPERATION AT A TIME", so a write that could start while
// a read is parked inside opMu would break it whichever operation holds the
// lock.
//
// RED PROOF, observed: with the two opMu lines removed from WriteChannel the
// write completes while the read is parked and this test fails at "the write
// completed while a read held opMu".
func TestWriteChannel_IsAtomicUnderOpMu(t *testing.T) {
	sess, _ := openSession(t, Simulated, radioImage{
		mrAnswers: map[string]string{mrAddr("01"): populatedMR("01")},
	})

	entered := make(chan struct{}, 1)
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
	<-entered // the read is inside the lock and parked

	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = sess.WriteChannel(context.Background(), writableChannel("42", "OFF"))
		close(done)
	}()

	select {
	case <-done:
		t.Error("the write completed while a read held opMu; the lock guards a whole driver OPERATION, not an exchange")
	case <-time.After(150 * time.Millisecond):
	}
	close(release)
	wg.Wait()
}

// TestWriteChannel_TheStandaloneQ2RungIsDeliberatelyAbSENT is a documentation
// pin with teeth, and it is the corrected reading of rev 2's instruction.
//
// Rev 2 asked for DISTINCT typed A22 and Q2 errors recovered separately by
// errors.As, ordered A22 first. A22 rejects every TS-480 write BY DEFINITION,
// so a Q2 rung after it can never execute; the two instructions could not both
// be obeyed. The ruling: Q2 rides INSIDE A22's cause list, and the standalone
// rung — with its own ordering and its own positive control — is written in
// the commit that lifts A22 (L-HW-16) and not before.
//
// This test asserts the absence so that a future reader does not add the rung
// by reflex and find it unreachable: there is no exported Q2 refusal type, and
// no refusal this package can produce carries Q2 as its PRIMARY cause.
func TestWriteChannel_TheStandaloneQ2RungIsDeliberatelyAbSENT(t *testing.T) {
	for _, toneMode := range []string{"OFF", "TONE", "CTCSS"} {
		sess, _ := openSession(t, Simulated, radioImage{})
		_, err := sess.WriteChannel(context.Background(), writableChannel("42", toneMode))
		var ref *RefusalError
		if !errors.As(err, &ref) {
			t.Fatalf("tone_mode %q: want *RefusalError", toneMode)
		}
		if ref.Register == registerQ2 {
			t.Errorf("tone_mode %q: Q2 answered as the PRIMARY cause; while A22 stands it can only be a subsidiary one, and the standalone rung belongs in the commit that lifts A22 (L-HW-16)", toneMode)
		}
		if len(ref.Causes) == 0 || ref.Causes[0] != registerA22 {
			t.Errorf("tone_mode %q: causes = %v, want A22 first", toneMode, ref.Causes)
		}
	}
}
