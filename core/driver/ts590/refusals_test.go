// SPDX-License-Identifier: GPL-3.0-or-later

package ts590

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// assertNoWireTraffic fails unless the only frames this session ever sent are
// the probe's three. EVERY refusal in this file is a PRE-WIRE refusal, and
// the transcript is the only thing that can prove it: an error returned
// after a frame went out is a different — and much worse — event than the
// same error returned before one was built.
func assertNoWireTraffic(t *testing.T, p *respondingPort, what string) {
	t.Helper()
	if got := p.Transcript(); !reflect.DeepEqual(got, probeFrames) {
		t.Errorf("%s: transcript = %v, want the probe's three frames and nothing more", what, got)
	}
}

// assertRegister fails unless err is THIS PACKAGE'S semantic refusal naming
// register entry want.
//
// IT IS THE HALF OF EVERY SEMANTIC PIN THAT MAKES IT NON-VACUOUS (plan P7's
// H2). Every rung of this ladder returns "refused"; a test that asserted
// only the fact of refusal would pass on the CAPABILITY gate — which answers
// first for every write on an unconsented RealHardware session, since
// writeTrialsComplete is false on both rows — and the semantic rung it
// claimed to pin need not exist at all. The capability gate returns a plain
// *driver.WriteRefusedError, so errors.As for *RefusalError fails on it, and
// the Register field then says WHICH semantic rung answered.
func assertRegister(t *testing.T, err error, want string, what string) {
	t.Helper()
	var ref *RefusalError
	if !errors.As(err, &ref) {
		t.Fatalf("%s: err = %v (%T), want *RefusalError naming %s", what, err, err, want)
	}
	if ref.Register != want {
		t.Errorf("%s: refusal names register entry %q, want %q", what, ref.Register, want)
	}
	if !strings.Contains(ref.Reason, want) {
		t.Errorf("%s: reason %q does not name %s", what, ref.Reason, want)
	}
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Errorf("%s: refusal does not match driver.ErrWriteRefused", what)
	}
	// A semantic refusal is STILL a write refusal on the neutral seam: a
	// caller that recovers the fleet type must find it.
	var fleet *driver.WriteRefusedError
	if !errors.As(err, &fleet) {
		t.Errorf("%s: refusal is not recoverable as *driver.WriteRefusedError", what)
	}
}

// TestRegisterConstants_NameTheAuthoritativeEntries is LOW-1's fix (Opus
// review, T12 fix round 1): assertRegister compares a rung's own Register
// argument against the SAME constant the rung passed in, so nothing pins the
// constants' VALUES against the authoritative register core/kw/doc.go names
// (core/kw/register_test.go:63-65 pins the register itself, by row). Editing
// a constant's value here would rename every refusal's citation with the
// whole suite green, and a user reading a log would be told the wrong
// assumption.
//
// registerA13 IS NOT CHECKED AS MEMBERSHIP: it is the composite "A13/A14",
// one rung answering for two register rows because one comparison consults
// both (see the const block's own comment), so it cannot be checked against
// core/kw's per-entry list directly.
func TestRegisterConstants_NameTheAuthoritativeEntries(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{registerA9, "A9"},
		{registerA13, "A13/A14"},
		{registerA23, "A23"},
		{registerDecision14, "decision 14"},
	} {
		if tc.got != tc.want {
			t.Errorf("register constant = %q, want %q", tc.got, tc.want)
		}
	}
}

// TestWriteChannel_TheCapabilityGateAnswersFirstOnUnconsentedRealHardware is
// the ONE rung pinned on the RealHardware profile, and it is pinned SEPARATELY
// for that reason (plan P7's H2).
//
// The zero Profile IS RealHardware by fail-safe and writeTrialsComplete is
// false on both rows, so every field of an unconsented real-hardware session
// is Unverified, spec.FieldSupport.CanWrite is false for all of them, and the
// capability gate is the first answer for EVERY write — including the
// perfectly ordinary FM channel every semantic rung below uses as its
// positive control. That is the profile working, not a limitation of the
// method, and the one route past it is the user's own recorded consent.
//
// MEDIUM-2's ROW (Opus review, T12 fix round 1): every OTHER case here runs
// at the default FV1.00;, so none of them can distinguish "the gate fires
// first" from "A13/A14 was hoisted above it" — a session must trip BOTH to
// tell the two apart, and an S at FV2.00 is the only row that does.
// Hoisting A13/A14 above the gate answered an unconsented RealHardware S at
// FV2.00 with the FIRMWARE refusal instead of the CONSENT one, attributing to
// an unlifted assumption what is actually a missing consent — exactly the
// confusion RefusalError.Register exists to prevent — and left the whole
// package green.
func TestWriteChannel_TheCapabilityGateAnswersFirstOnUnconsentedRealHardware(t *testing.T) {
	cases := []struct {
		name string
		row  Row
		img  radioImage
	}{
		{modelNameFor(RowS), RowS, radioImage{}},
		{modelNameFor(RowSG), RowSG, radioImage{}},
		{modelNameFor(RowS) + " at FV2.00, still unconsented", RowS, radioImage{fvAnswer: "FV2.00;"}},
	}
	for _, tc := range cases {
		sess, p := openWriteSession(t, tc.row, RealHardware, tc.img)
		_, err := sess.WriteChannel(context.Background(), writableChannel(tc.row, "042"))
		var ref *driver.WriteRefusedError
		if !errors.As(err, &ref) {
			t.Fatalf("%s: err = %v (%T), want *driver.WriteRefusedError", tc.name, err, err)
		}
		var semantic *RefusalError
		if errors.As(err, &semantic) {
			t.Errorf("%s: the capability gate returned a SEMANTIC refusal (%s); the two must stay distinguishable", tc.name, semantic.Register)
		}
		if len(ref.Fields) == 0 {
			t.Errorf("%s: the capability refusal names no field", tc.name)
		}
		assertNoWireTraffic(t, p, tc.name)
	}
}

// TestWriteChannel_ConsentIsTheRouteThroughTheCapabilityGate is the
// capability rung's own positive control, on its own profile: the SAME
// channel that RealHardware refuses above reaches the wire once the user has
// consented. Without it, "refused on RealHardware" would be indistinguishable
// from "refused for some other reason on every profile".
//
// CONSENT WIDENS WHAT MAY BE ATTEMPTED, NEVER HOW CAREFULLY (matrix §2.1):
// the semantic rungs below still fire on a consented session, which is what
// the A9 row of the next test asserts on this very profile.
func TestWriteChannel_ConsentIsTheRouteThroughTheCapabilityGate(t *testing.T) {
	for _, row := range bothRows {
		what := modelNameFor(row)
		sess, p := openWriteSession(t, row, RealHardware, radioImage{}, WithConsentedUnverifiedWrites())
		res, err := sess.WriteChannel(context.Background(), writableChannel(row, "042"))
		if err != nil {
			t.Fatalf("%s: a consented write was refused: %v", what, err)
		}
		if len(res.Steps) != 1 || !res.Steps[0].Sent {
			t.Errorf("%s: Steps = %+v, want one MW reported Sent", what, res.Steps)
		}
		if got := p.Transcript(); len(got) != len(probeFrames)+1 {
			t.Errorf("%s: transcript = %v, want the probe and one MW", what, got)
		}
	}
}

// TestWriteChannel_A9RefusesAChannelWithNoKnownTXDisposition pins decision 11
// on a session that has ALREADY PASSED the capability gate, with its positive
// control on the same footing.
//
// A9 IS WHAT CLOSES THE SILENT-FLATTENING LOOP, and the loop is worth stating
// because nothing else in the stack closes it: codeplug.Diff counts
// tx_frequency only when its state is Known, Validate does not refuse
// Unavailable, core/clone hands the driver the whole candidate, and its
// verify compares TxFreqHz only when BOTH sides are Known. Composed, a split
// channel read as Unavailable, edited in its name alone and sent, would be
// flattened to simplex by an MW with P1='0' — "the channel becomes a simplex
// channel, even if it was already a split channel" (590:1521-1523) — read
// back as Unavailable and reported VERIFIED. So the refusal lives here,
// before any frame is built.
//
// THE CONTROL RUNS ON THE SAME PROFILE AND MUST SUCCEED TO THE WIRE: a
// control that were itself refused would make the pin vacuous in the exact
// way H2 describes, with both sides reading as "refused".
func TestWriteChannel_A9RefusesAChannelWithNoKnownTXDisposition(t *testing.T) {
	for _, row := range bothRows {
		what := modelNameFor(row)
		sess, p := openWriteSession(t, row, Simulated, radioImage{})
		ch := writableChannel(row, "042")
		ch.Data.TxFreqHz = codeplug.FreqField{State: codeplug.Unavailable}
		_, err := sess.WriteChannel(context.Background(), ch)
		assertRegister(t, err, registerA9, what)
		var ref *RefusalError
		if errors.As(err, &ref) && !reflect.DeepEqual(ref.Fields, []spec.Field{spec.FieldTxFrequency}) {
			t.Errorf("%s: A9 refusal names %v, want [tx_frequency]", what, ref.Fields)
		}
		assertNoWireTraffic(t, p, what)

		// The control: the same channel with a Known TX disposition.
		ctrl, cp := openWriteSession(t, row, Simulated, radioImage{})
		if _, err := ctrl.WriteChannel(context.Background(), writableChannel(row, "042")); err != nil {
			t.Fatalf("%s: A9's positive control was refused: %v", what, err)
		}
		if got := cp.Transcript(); len(got) != len(probeFrames)+1 {
			t.Errorf("%s: A9's control sent %v, want one MW", what, got)
		}
	}
}

// TestWriteChannel_A9DoesNotGateTheSCANBank is the pin that fails if
// decision 11's gate is written as an UNCONDITIONAL field test rather than a
// question about whether the bank PUBLISHES FieldTxFrequency (plan P12,
// matrix M-E2).
//
// In the SCAN bank P1 selects the section channel's START or END frequency
// (590:1449-1451, 590:1529-1531), not a transmit frequency, so the field is
// the zero FieldSupport there and there is no TX disposition to require;
// requiring one would refuse every scan-edge write forever under a name that
// describes something else.
//
// THE FIXTURE IS AN FM SCAN EDGE (M9), deliberately: a scan edge's mode is
// rarely FM, and the A23 rung would otherwise refuse this channel first,
// making the pin vacuous for a second and independent reason.
func TestWriteChannel_A9DoesNotGateTheSCANBank(t *testing.T) {
	for _, row := range bothRows {
		for _, id := range []string{"100L", "100U"} {
			what := modelNameFor(row) + " " + id
			sess, p := openWriteSession(t, row, Simulated, radioImage{})
			ch := writableChannel(row, id)
			if ch.Data.TxFreqHz.State != codeplug.Unavailable {
				t.Fatalf("%s: the fixture carries a TX disposition; the pin would be vacuous", what)
			}
			if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
				t.Fatalf("%s: a scan-edge write was refused: %v", what, err)
			}
			if got := p.Transcript(); len(got) != len(probeFrames)+1 {
				t.Errorf("%s: transcript = %v, want one MW", what, got)
			}
		}
	}
}

// TestWriteChannel_ASplitReadEditedInItsNameOnlyIsRefused is A9's regression
// proof in the exact shape the design describes: a channel READ off the
// radio — where a fresh read leaves tx_frequency Unavailable on every Kenwood
// row — edited in its NAME ALONE and sent back.
//
// A wrong implementation flattens it and reports success. This one refuses,
// naming A9, before a frame exists.
func TestWriteChannel_ASplitReadEditedInItsNameOnlyIsRefused(t *testing.T) {
	const id = "042"
	for _, row := range bothRows {
		what := modelNameFor(row)
		sess, p := openWriteSession(t, row, Simulated, radioImage{mrAnswers: map[string]string{mrAddr(id): populatedMR(id)}})
		ch, err := sess.ReadChannel(context.Background(), id)
		if err != nil {
			t.Fatalf("%s: ReadChannel: %v", what, err)
		}
		if ch.Data.TxFreqHz.State == codeplug.Known {
			t.Fatalf("%s: a fresh read produced a Known tx_frequency; the loop this pins cannot happen", what)
		}
		ch.Data.Tag = "RENAMED"
		_, err = sess.WriteChannel(context.Background(), ch)
		assertRegister(t, err, registerA9, what)
		if got := p.Transcript(); len(got) != len(probeFrames)+1 {
			t.Errorf("%s: transcript = %v, want the probe and the ONE MR — no MW", what, got)
		}
	}
}

// TestWriteChannel_ASplitTheOneFrameCannotExpressIsRefused is A9's other
// edge, and it is NOT A9: a channel whose TX disposition is Known AND
// DIFFERENT from its receive frequency is a genuine split, and this
// choreography sends ONE MW whose P1 comes from the slot's class (P14, M9).
// P1='0' would flatten it — 590:1521-1523 says so in as many words — so the
// difference is refused rather than dropped.
//
// It is a plain *driver.WriteRefusedError and carries no register entry: no
// assumption is at stake, only the arithmetic of a one-frame write against a
// two-frame representation of split.
func TestWriteChannel_ASplitTheOneFrameCannotExpressIsRefused(t *testing.T) {
	for _, row := range bothRows {
		what := modelNameFor(row)
		sess, p := openWriteSession(t, row, Simulated, radioImage{})
		ch := writableChannel(row, "042")
		ch.Data.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: ch.Data.FreqHz + 600_000}
		_, err := sess.WriteChannel(context.Background(), ch)
		var ref *driver.WriteRefusedError
		if !errors.As(err, &ref) {
			t.Fatalf("%s: err = %v (%T), want *driver.WriteRefusedError", what, err, err)
		}
		var semantic *RefusalError
		if errors.As(err, &semantic) {
			t.Errorf("%s: the split refusal claims register entry %s; it names no assumption", what, semantic.Register)
		}
		if !reflect.DeepEqual(ref.Fields, []spec.Field{spec.FieldTxFrequency}) {
			t.Errorf("%s: refusal names %v, want [tx_frequency]", what, ref.Fields)
		}
		assertNoWireTraffic(t, p, what)
	}
}

// TestWriteChannel_A23RefusesANonFMWrite pins the mode gate and its control.
//
// P14 IS A TWO-BYTE FLAG WHOSE ONLY PRINTED MEANINGS ARE "00: FM Normal" and
// "01: FM Narrow" (590:1569-1571). Nothing says what it means in SSB, CW, AM
// or FSK, so a non-FM write would have to put a byte on the wire whose
// meaning this programme does not hold — which decision 12 forbids. The
// refusal lifts per (row, mode) CELL, not per mode and not per pair
// (decision 13), which is why this table walks every non-FM mode on both
// rows rather than one mode on one row.
func TestWriteChannel_A23RefusesANonFMWrite(t *testing.T) {
	for _, row := range bothRows {
		// ONE session for all seven refusals, and the transcript assertion
		// is stronger for it: seven refused writes in a row leave the port
		// carrying nothing but the probe.
		sess, p := openWriteSession(t, row, Simulated, radioImage{})
		for _, mode := range []string{"LSB", "USB", "CW", "CW-R", "AM", "FSK", "FSK-R"} {
			what := modelNameFor(row) + " " + mode
			ch := writableChannel(row, "042")
			ch.Data.Mode = mode
			_, err := sess.WriteChannel(context.Background(), ch)
			assertRegister(t, err, registerA23, what)
			var ref *RefusalError
			if errors.As(err, &ref) && !reflect.DeepEqual(ref.Fields, []spec.Field{spec.FieldMode}) {
				t.Errorf("%s: A23 refusal names %v, want [mode]", what, ref.Fields)
			}
			assertNoWireTraffic(t, p, what)
		}
		// The control, BOTH FM names, on the same profile and to the wire.
		for _, mode := range []string{"FM", "FM-N"} {
			what := modelNameFor(row) + " " + mode
			sess, p := openWriteSession(t, row, Simulated, radioImage{})
			ch := writableChannel(row, "042")
			ch.Data.Mode = mode
			if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
				t.Fatalf("%s: A23's positive control was refused: %v", what, err)
			}
			got := p.Transcript()
			if len(got) != len(probeFrames)+1 {
				t.Fatalf("%s: transcript = %v, want one MW", what, got)
			}
			// P5 is FM's nibble on both names; P14 is what separates them.
			frame := got[len(got)-1]
			wantP14 := "00"
			if mode == "FM-N" {
				wantP14 = "01"
			}
			if frame[17] != '4' {
				t.Errorf("%s: P5 = %q, want '4' (590:1358)", what, frame[17])
			}
			if frame[38:40] != wantP14 {
				t.Errorf("%s: P14 = %q, want %q (590:1569-1571)", what, frame[38:40], wantP14)
			}
		}
	}
}

// TestWriteChannel_AModeThisRowDoesNotPublishIsNotA23 keeps the two mode
// refusals apart. A mode name that is not in this row's legend at all is an
// ordinary unsupported-value refusal; A23 is the narrower and more
// interesting statement that a mode this radio DOES have cannot be written
// because one byte beside it has no printed meaning there.
func TestWriteChannel_AModeThisRowDoesNotPublishIsNotA23(t *testing.T) {
	sess, p := openWriteSession(t, RowSG, Simulated, radioImage{})
	ch := writableChannel(RowSG, "042")
	ch.Data.Mode = "C4FM"
	_, err := sess.WriteChannel(context.Background(), ch)
	var ref *driver.WriteRefusedError
	if !errors.As(err, &ref) {
		t.Fatalf("err = %v (%T), want *driver.WriteRefusedError", err, err)
	}
	var semantic *RefusalError
	if errors.As(err, &semantic) {
		t.Errorf("an unpublished mode was refused as %s; A23 is about a mode this radio HAS", semantic.Register)
	}
	if !reflect.DeepEqual(ref.Fields, []spec.Field{spec.FieldMode}) {
		t.Errorf("refusal names %v, want [mode]", ref.Fields)
	}
	assertNoWireTraffic(t, p, "an unpublished mode")
}

// TestWriteChannel_A13A14RefusesChannelWritesOnAnSWhoseFVIsHighOrUnreadable
// pins the firmware gate — the S row's write path alone (matrix §4,
// divergence 2).
//
// TWO CAUSES, ONE RUNG. A TS-590S at FV >= 2.00 may have a LIVE byte 28
// ("always 0" is guaranteed for 1.xx only, 590:1478/590:1564), and an FV
// this programme cannot read as A13's assumed M.NN form has to take the
// conservative branch — which is what makes A13 load-bearing on a REFUSAL
// path rather than a success path. THE SESSION IS NOT REFUSED either way:
// the radio stays fully readable and the raw FV bytes reach the probe note.
//
// AND NOTHING ON THE SG ROW BRANCHES ON FV AT ALL, because the SG's byte 28
// is live by construction — the last row below is the pin that fails if a
// future edit generalises the gate to "a 590".
func TestWriteChannel_A13A14RefusesChannelWritesOnAnSWhoseFVIsHighOrUnreadable(t *testing.T) {
	for _, tc := range []struct {
		name     string
		row      Row
		fv       string
		refused  bool
		register string
	}{
		{"S at 2.00", RowS, "FV2.00;", true, registerA13},
		{"S at 3.14", RowS, "FV3.14;", true, registerA13},
		{"S with an unreadable FV", RowS, "FVWXYZ;", true, registerA13},
		{"S at 1.00 — the control", RowS, "FV1.00;", false, ""},
		{"S at 1.99 — the control's upper edge", RowS, "FV1.99;", false, ""},
		{"SG at 2.00 refuses nothing", RowSG, "FV2.00;", false, ""},
		{"SG with an unreadable FV refuses nothing", RowSG, "FVWXYZ;", false, ""},
	} {
		sess, p := openWriteSession(t, tc.row, Simulated, radioImage{fvAnswer: tc.fv})
		_, err := sess.WriteChannel(context.Background(), writableChannel(tc.row, "042"))
		if !tc.refused {
			if err != nil {
				t.Errorf("%s: write refused: %v", tc.name, err)
			}
			if got := p.Transcript(); len(got) != len(probeFrames)+1 {
				t.Errorf("%s: transcript = %v, want one MW", tc.name, got)
			}
			continue
		}
		assertRegister(t, err, tc.register, tc.name)
		assertNoWireTraffic(t, p, tc.name)
	}
}

// TestWriteChannel_Decision14RefusesAKnown1750HzToneRx pins the one index
// the published tone domain carries that the printed CN chart does not.
//
// spec.Capabilities has ONE tone domain and ONE predicate, AdmitsTone, which
// codeplug.ToneField.Valid applies to tone_tx and tone_rx alike, so the
// 43-entry TN list (590:2296-2306) admits 1750 Hz as a tone_rx value that
// CN's own 00-41 range (590:411, its chart at 590:416-426) cannot express. The resolution is
// erratum M-E1's: publish 43 and refuse the value HERE, in the write path,
// rather than claiming something false in the table.
//
// THE ASYMMETRY WITH THE READ PATH IS DELIBERATE: a 1750 Hz tone_rx cannot
// come off a radio in the first place, because core/kw bounds P9 at CN's own
// 41 — and if one somehow did, refusing it would fail a whole-radio read
// over a channel this programme merely cannot write back.
func TestWriteChannel_Decision14RefusesAKnown1750HzToneRx(t *testing.T) {
	const tone1750 = spec.Tone(17500)
	if kenwoodCTCSSTones[len(kenwoodCTCSSTones)-1] != tone1750 {
		t.Fatalf("the published domain's last entry is %v, want 1750 Hz — this pin's subject has moved", kenwoodCTCSSTones[len(kenwoodCTCSSTones)-1])
	}
	for _, row := range bothRows {
		what := modelNameFor(row)
		sess, p := openWriteSession(t, row, Simulated, radioImage{})
		ch := writableChannel(row, "042")
		ch.Data.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: tone1750}
		_, err := sess.WriteChannel(context.Background(), ch)
		assertRegister(t, err, registerDecision14, what)
		var ref *RefusalError
		if errors.As(err, &ref) && !reflect.DeepEqual(ref.Fields, []spec.Field{spec.FieldToneRx}) {
			t.Errorf("%s: decision 14 refusal names %v, want [tone_rx]", what, ref.Fields)
		}
		assertNoWireTraffic(t, p, what)

		// The control, on the same profile and to the wire: 100.0 Hz, which
		// is index 12 on both charts.
		ctrl, cp := openWriteSession(t, row, Simulated, radioImage{})
		ok := writableChannel(row, "042")
		ok.Data.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: 1000}
		if _, err := ctrl.WriteChannel(context.Background(), ok); err != nil {
			t.Fatalf("%s: decision 14's positive control was refused: %v", what, err)
		}
		got := cp.Transcript()
		if len(got) != len(probeFrames)+1 {
			t.Fatalf("%s: control transcript = %v, want one MW", what, got)
		}
		if frame := got[len(got)-1]; frame[22:24] != "12" {
			t.Errorf("%s: P9 = %q, want \"12\" for 100.0 Hz", what, frame[22:24])
		}
	}
}

// TestWriteChannel_A1750HzToneTxIsWritten is decision 14's OTHER half, and
// it is the pin that fails if the refusal is written as "1750 Hz anywhere".
// TN's chart runs 00-42 and 1750 Hz is its last entry (590:2296-2309), so a
// TRANSMIT tone of 1750 Hz is an ordinary, printed, writable value.
func TestWriteChannel_A1750HzToneTxIsWritten(t *testing.T) {
	sess, p := openWriteSession(t, RowSG, Simulated, radioImage{})
	ch := writableChannel(RowSG, "042")
	ch.Data.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: 17500}
	if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("a 1750 Hz tone_tx was refused: %v", err)
	}
	got := p.Transcript()
	if frame := got[len(got)-1]; frame[20:22] != "42" {
		t.Errorf("P8 = %q, want \"42\" — TN's last printed index (590:2291)", frame[20:22])
	}
}

// TestWriteChannel_TheLadderRefusesBeforeItReachesARow pins the three rungs
// that answer from this session's own published capabilities and never ask
// the radio anything: the slot's SYNTAX, its bank MEMBERSHIP, and the erase
// an empty channel would be.
//
// THE TS-590SG'S EXTENSION CHANNELS ARE HERE, AND BY EXACTLY THE MEMBERSHIP
// MECHANISM — Stuart decisions row 6, RULED 05/09/2026. The codec's SG
// layout still DECLARES 110-119 (the book prints them, 590:1346-1347), so an
// "MR0115;" from a front-panel recall parses; the DRIVER publishes 000-099
// only, so a write naming one is refused the same shape as any other
// unpublished slot, with no invented radio behaviour. T11 pinned the read
// side and could not pin this one while WriteChannel was a placeholder.
func TestWriteChannel_TheLadderRefusesBeforeItReachesARow(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  Row
		ch   func(Row) codeplug.Channel
		is   error
	}{
		{"a malformed slot", RowSG, func(r Row) codeplug.Channel { return writableChannel(r, "42") }, ErrUnknownSlot},
		{"a slot outside the printed space", RowSG, func(r Row) codeplug.Channel { return writableChannel(r, "999") }, ErrUnknownSlot},
		{"a scan half on a memory slot", RowSG, func(r Row) codeplug.Channel { return writableChannel(r, "042L") }, ErrUnknownSlot},
		{"SG extension channel 110", RowSG, func(r Row) codeplug.Channel { return writableChannel(r, "110") }, ErrUnknownSlot},
		{"SG extension channel 115", RowSG, func(r Row) codeplug.Channel { return writableChannel(r, "115") }, ErrUnknownSlot},
		{"SG extension channel 119", RowSG, func(r Row) codeplug.Channel { return writableChannel(r, "119") }, ErrUnknownSlot},
		{"110 on the S row too", RowS, func(r Row) codeplug.Channel { return writableChannel(r, "110") }, ErrUnknownSlot},
		{"an empty channel is an erase", RowSG, func(r Row) codeplug.Channel {
			return codeplug.Channel{Slot: "042"}
		}, driver.ErrWriteRefused},
	} {
		sess, p := openWriteSession(t, tc.row, Simulated, radioImage{})
		_, err := sess.WriteChannel(context.Background(), tc.ch(tc.row))
		if !errors.Is(err, tc.is) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.is)
		}
		assertNoWireTraffic(t, p, tc.name)
	}
}

// TestWriteChannel_AnEraseNamesFieldEraseAndTheStandingRule pins the erase
// refusal's own shape. The ONLY documented clear on these rows is a side
// effect of a short MW whose length is a reading rather than a printed number
// (590:1579-1581, A5, erratum E19), this milestone never builds it
// (decision 8), and spec.FieldErase is nowhere write-Supported.
func TestWriteChannel_AnEraseNamesFieldEraseAndTheStandingRule(t *testing.T) {
	sess, _ := openWriteSession(t, RowSG, Simulated, radioImage{})
	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "042"})
	var ref *driver.WriteRefusedError
	if !errors.As(err, &ref) {
		t.Fatalf("err = %v (%T), want *driver.WriteRefusedError", err, err)
	}
	if !reflect.DeepEqual(ref.Fields, []spec.Field{spec.FieldErase}) {
		t.Errorf("refusal names %v, want [erase]", ref.Fields)
	}
}

// TestWriteChannel_AnIncoherentFieldIsRefusedNotInterpreted pins the fleet's
// FieldState walk as this ladder's third rung (driver.CheckFieldStates).
//
// A value carried alongside a state that means "preserve whatever the radio
// has" is a malformation, and the failure it prevents is SILENT: without this
// rung, requestedFields (Known-only) never names the field, so the value is
// dropped from the frame and the write reports success. That is the FT-891
// closing review's C-M1, and MEDIUM-1's sharpening of it — a value with no
// State recorded at all (codeplug.Absent, the zero) is the same malformation.
func TestWriteChannel_AnIncoherentFieldIsRefusedNotInterpreted(t *testing.T) {
	sess, p := openWriteSession(t, RowSG, Simulated, radioImage{})
	for _, tc := range []struct {
		name  string
		edit  func(*codeplug.ChannelData)
		field spec.Field
	}{
		{"an Unavailable tx_frequency carrying a value", func(d *codeplug.ChannelData) {
			d.TxFreqHz = codeplug.FreqField{State: codeplug.Unavailable, Value: 145_500_000}
		}, spec.FieldTxFrequency},
		{"an Absent ctcss_tone carrying a value", func(d *codeplug.ChannelData) {
			d.CTCSSTone = codeplug.ToneField{Value: 885}
		}, spec.FieldCTCSSTone},
		{"a Known tone_tx outside this row's chart", func(d *codeplug.ChannelData) {
			d.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: 1598}
		}, spec.FieldToneTx},
		{"a Known tone_mode outside this row's legend", func(d *codeplug.ChannelData) {
			d.ToneMode = codeplug.StringField{State: codeplug.Known, Value: "DCS"}
		}, spec.FieldToneMode},
	} {
		ch := writableChannel(RowSG, "042")
		tc.edit(ch.Data)
		_, err := sess.WriteChannel(context.Background(), ch)
		var ref *driver.WriteRefusedError
		if !errors.As(err, &ref) {
			t.Fatalf("%s: err = %v (%T), want *driver.WriteRefusedError", tc.name, err, err)
		}
		if !reflect.DeepEqual(ref.Fields, []spec.Field{tc.field}) {
			t.Errorf("%s: refusal names %v, want [%s]", tc.name, ref.Fields, tc.field)
		}
		assertNoWireTraffic(t, p, tc.name)
	}
}

// TestWriteChannel_AFieldTheRowDoesNotPublishIsRefusedPreWireNamingTheField is the
// gate's per-field half, on a session that is otherwise fully writable.
//
// THE S ROW'S FILTER IS THE MILESTONE'S OWN EXAMPLE (Q12, §2.7): byte 28 is
// live on an SG and on an S at firmware >= 2.00, and spec.Capabilities is a
// STATIC per-model value that cannot say "Supported iff FV >= 2.00", so the
// S row publishes FieldFilter unwritable and an empty Filters vocabulary.
//
// WHICH RUNG ANSWERS FIRST, STATED HONESTLY: with an EMPTY vocabulary,
// codeplug.StringField.Valid refuses any Known value, so the fleet's
// FieldState walk — plan P7's Valid() rung, one ABOVE the capability gate —
// is what answers on this row. The plan's bullet asks for "refused by the
// capability gate"; both rungs are capability-derived (the empty vocabulary
// IS the row's capability table) and both name spec.FieldFilter, so what is
// pinned here is the field and the pre-wire refusal rather than which of the
// two adjacent capability rungs fired. The SG row below is the control that
// the same value is not refused where the row publishes it.
func TestWriteChannel_AFieldTheRowDoesNotPublishIsRefusedPreWireNamingTheField(t *testing.T) {
	sess, p := openWriteSession(t, RowS, Simulated, radioImage{})
	ch := writableChannel(RowS, "042")
	ch.Data.Filter = codeplug.StringField{State: codeplug.Known, Value: filterALabel}
	_, err := sess.WriteChannel(context.Background(), ch)
	var ref *driver.WriteRefusedError
	if !errors.As(err, &ref) {
		t.Fatalf("err = %v (%T), want *driver.WriteRefusedError", err, err)
	}
	var semantic *RefusalError
	if errors.As(err, &semantic) {
		t.Errorf("a filter write on the S row was refused as %s; it is a capability refusal", semantic.Register)
	}
	if !reflect.DeepEqual(ref.Fields, []spec.Field{spec.FieldFilter}) {
		t.Errorf("refusal names %v, want [filter]", ref.Fields)
	}
	assertNoWireTraffic(t, p, "a filter write on the S row")

	// The control: the same value on the row that publishes the labels.
	ctrl, cp := openWriteSession(t, RowSG, Simulated, radioImage{})
	if _, err := ctrl.WriteChannel(context.Background(), writableChannel(RowSG, "042")); err != nil {
		t.Fatalf("the SG control was refused: %v", err)
	}
	if got := cp.Transcript(); len(got) != len(probeFrames)+1 {
		t.Errorf("the SG control sent %v, want one MW", got)
	}
}

// TestWriteChannel_ATierFieldTheRecordCannotExpressIsRefused pins the gate on
// a field neither row's 50-byte record has room for. A caller handing this
// driver a Known IP+ or preamp position — from a native file written for an
// Icom, or a GUI paste — must be REFUSED rather than have the value silently
// dropped from a frame that has nowhere to put it.
func TestWriteChannel_ATierFieldTheRecordCannotExpressIsRefused(t *testing.T) {
	sess, p := openWriteSession(t, RowSG, Simulated, radioImage{})
	for _, tc := range []struct {
		name  string
		edit  func(*codeplug.ChannelData)
		field spec.Field
	}{
		{"a Known duplex", func(d *codeplug.ChannelData) {
			d.Duplex = codeplug.StringField{State: codeplug.Known, Value: "SIMPLEX"}
		}, spec.FieldDuplex},
		{"a Known tag display", func(d *codeplug.ChannelData) {
			d.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: true}
		}, spec.FieldTagDisplay},
		{"a clarifier offset", func(d *codeplug.ChannelData) { d.ClarHz = 500 }, spec.FieldClarifier},
		{"a Yaesu ctcss state", func(d *codeplug.ChannelData) { d.CTCSS = "ENC" }, spec.FieldCTCSSState},
		{"a Yaesu shift", func(d *codeplug.ChannelData) { d.Shift = "PLUS" }, spec.FieldShift},
	} {
		ch := writableChannel(RowSG, "042")
		tc.edit(ch.Data)
		_, err := sess.WriteChannel(context.Background(), ch)
		var ref *driver.WriteRefusedError
		if !errors.As(err, &ref) {
			t.Fatalf("%s: err = %v (%T), want *driver.WriteRefusedError", tc.name, err, err)
		}
		found := false
		for _, f := range ref.Fields {
			if f == tc.field {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: refusal names %v, want it to include %s", tc.name, ref.Fields, tc.field)
		}
		assertNoWireTraffic(t, p, tc.name)
	}
}
