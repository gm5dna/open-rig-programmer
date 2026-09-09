// SPDX-License-Identifier: GPL-3.0-or-later

package ts890

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// assertNoWireTraffic fails unless the only frames this session ever sent are
// the probe's three. EVERY Part-1 rung is a PRE-WIRE refusal, and the
// transcript is the only thing that can prove it: an error returned after a
// frame went out is a different — and much worse — event than the same error
// returned before one was built.
func assertNoWireTraffic(t *testing.T, p *respondingPort, what string) {
	t.Helper()
	if got := p.Transcript(); !slices.Equal(got, probeFrames) {
		t.Errorf("%s: transcript = %v, want the probe's three frames and nothing more", what, got)
	}
}

// assertReadButNoSet fails unless the session sent the probe and EXACTLY ONE
// MA0 read — the shape a Part-2 rung must leave behind, since both read-
// dependent rungs quote an answer they could not have without the read, and
// neither may let the mutating frame out.
func assertReadButNoSet(t *testing.T, p *respondingPort, slot, what string) {
	t.Helper()
	want := append(slices.Clone(probeFrames), "MA0"+slot+";")
	if got := p.Transcript(); !slices.Equal(got, want) {
		t.Errorf("%s: transcript = %v, want %v — the one read, and no Set", what, got, want)
	}
}

// assertRegister fails unless err is THIS PACKAGE'S semantic refusal naming
// register entry want.
//
// IT IS THE HALF OF EVERY SEMANTIC PIN THAT MAKES IT NON-VACUOUS (plan P7).
// Every rung of this ladder answers "refused"; a test that asserted only the
// FACT of refusal would pass on the CAPABILITY gate — which answers first for
// every write on an unconsented RealHardware session, writeTrialsComplete
// being false — and the semantic rung it claimed to pin need not exist at
// all. The capability gate returns a plain *driver.WriteRefusedError, so
// errors.As for *RefusalError fails on it, and Register then says WHICH
// semantic rung answered.
func assertRegister(t *testing.T, err error, want, what string) *RefusalError {
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
	return ref
}

// TestRegisterConstants_NameTheAuthoritativeEntries pins the constants'
// VALUES. assertRegister compares a rung's Register against the same constant
// the rung passed in, so without this nothing holds those strings to the
// authoritative register (core/kw/ma/doc.go's A-entries and the spec's
// numbered decisions): editing one would rename every refusal's citation with
// the whole suite green, and a user reading a log would be told the wrong
// assumption.
func TestRegisterConstants_NameTheAuthoritativeEntries(t *testing.T) {
	for _, tc := range []struct{ got, want string }{
		{registerA2, "A2"},
		{registerA3, "A3"},
		{registerDecision8, "decision 8"},
		{registerDecision9, "decision 9"},
		{registerDecision15, "decision 15"},
	} {
		if tc.got != tc.want {
			t.Errorf("register constant = %q, want %q", tc.got, tc.want)
		}
	}
}

// TestWriteChannel_TheLadderRefusesBeforeItReachesARow is rungs 1 and 2 — the
// slot identifier's SYNTAX and then membership in THIS session's published
// banks. Both are settled from the capabilities alone and the radio is never
// asked.
//
// "100" IS THE CASE THAT CARRIES THE DESIGN: the codec's slot space declares
// the Programmable VFO and extension classes because the book prints them, and
// the DRIVER publishes neither, so a write to one is refused by exactly the
// mechanism that refuses "999" — no special case and no invented radio
// behaviour (decision 12).
func TestWriteChannel_TheLadderRefusesBeforeItReachesARow(t *testing.T) {
	for _, tc := range []struct{ name, slot string }{
		{"a malformed identifier", "12"},
		{"a non-numeric identifier", "0x7"},
		{"outside the printed space", "999"},
		{"a Programmable VFO channel", "100"},
		{"an extension channel", "119"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sess, port := openTestSession(t, Simulated, radioImage{})
			_, err := sess.WriteChannel(context.Background(), simplexChannel(tc.slot))
			var unknown *UnknownSlotError
			if !errors.As(err, &unknown) || !errors.Is(err, ErrUnknownSlot) {
				t.Fatalf("WriteChannel(%q) err = %v (%T), want an *UnknownSlotError", tc.slot, err, err)
			}
			assertNoWireTraffic(t, port, tc.name)
		})
	}
}

// TestWriteChannel_AnEraseNamesTheStandingRuleOverAPrintedMA5 is rung 3, and
// it is STRUCTURAL as well as semantic: codeplug.Channel.Data is a pointer and
// every rung below dereferences it, so a driver without this rung PANICS on
// the empty channel core/clone hands it for an emptied row in a loaded file.
func TestWriteChannel_AnEraseNamesTheStandingRuleOverAPrintedMA5(t *testing.T) {
	sess, port := openTestSession(t, Simulated, radioImage{})
	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "007"})
	ref := assertRegister(t, err, registerDecision15, "an empty channel")
	if !slices.Equal(ref.Fields, []spec.Field{spec.FieldErase}) {
		t.Errorf("Fields = %v, want [erase]", ref.Fields)
	}
	if !strings.Contains(ref.Reason, "MA5") {
		t.Errorf("reason %q does not name the printed MA5 this rule declines to build", ref.Reason)
	}
	assertNoWireTraffic(t, port, "an empty channel")
}

// TestWriteChannel_AnIncoherentFieldIsRefusedNotInterpreted is rung 4, the
// FLEET's FieldState walk (driver.CheckFieldStates) consumed as a black box.
//
// WHAT IT PREVENTS IS SILENT. A value carried alongside a state meaning
// "preserve whatever the radio has" is never named by requestedFields, so
// without this rung it is dropped from the frame and the write reports
// SUCCESS. The pin is therefore the value-with-no-claim case and not a
// domain error: an Absent tone carrying a value, which is what a caller who
// set Value and forgot State leaves behind.
func TestWriteChannel_AnIncoherentFieldIsRefusedNotInterpreted(t *testing.T) {
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
			ch := simplexChannel("007")
			tc.mutate(ch.Data)
			sess, port := openTestSession(t, Simulated, radioImage{})
			_, err := sess.WriteChannel(context.Background(), ch)
			var refused *driver.WriteRefusedError
			if !errors.As(err, &refused) {
				t.Fatalf("err = %v (%T), want a *driver.WriteRefusedError", err, err)
			}
			var semantic *RefusalError
			if errors.As(err, &semantic) {
				t.Errorf("err names register %q: the FieldState walk is the fleet's and carries no register entry", semantic.Register)
			}
			if !slices.Equal(refused.Fields, []spec.Field{tc.field}) {
				t.Errorf("Fields = %v, want [%s]", refused.Fields, tc.field)
			}
			assertNoWireTraffic(t, port, tc.name)
		})
	}
}

// TestWriteChannel_TheCapabilityGateAnswersFirstOnUnconsentedRealHardware is
// rung 5, pinned SEPARATELY and on its own profile because it answers FIRST
// for every write while writeTrialsComplete is false.
//
// The zero Profile IS RealHardware by fail-safe, so every field of an
// unconsented real-hardware session is Unverified, FieldSupport.CanWrite is
// false for all of them, and the perfectly ordinary channel every semantic
// rung below uses as its positive control is refused HERE. That is the
// profile working, not a limitation of the method.
func TestWriteChannel_TheCapabilityGateAnswersFirstOnUnconsentedRealHardware(t *testing.T) {
	sess, port := openTestSession(t, RealHardware, populatedImage(t, 7, occupiedSimplex()))
	_, err := sess.WriteChannel(context.Background(), simplexChannel("007"))
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v (%T), want a *driver.WriteRefusedError", err, err)
	}
	var semantic *RefusalError
	if errors.As(err, &semantic) {
		t.Fatalf("the gate answered with a SEMANTIC refusal naming %q: the capability gate comes BEFORE the semantic rungs", semantic.Register)
	}
	// Every one of the eight the record always carries is named, because the
	// record transmits all eight on every write.
	if len(refused.Fields) != 8 {
		t.Errorf("Fields = %v, want all eight the record always carries", refused.Fields)
	}
	assertNoWireTraffic(t, port, "an unconsented real-hardware write")
}

// TestWriteChannel_TheCapabilityGateAnswersFirstEvenWhenASemanticRungWouldAlsoRefuse
// pins the gate's POSITION rather than merely its error TYPE. Every other
// case in this suite runs at the default RealHardware profile OR writes a
// channel that trips no semantic rung, so none of them can distinguish "the
// gate fires first" from "the gate was hoisted below a rung that would have
// refused anyway" — a session must trip BOTH to tell the two apart (review
// s2-t14-review-opus.md MED-1; the same finding as pair 1's
// core/driver/ts590/refusals_test.go:100-110). Left unpinned, that gap would
// let an unconsented session learn which of its channels the programme
// dislikes before it learned it may not write at all — exactly what plan
// P7's pinning paragraph forbids.
//
// The channel also trips rung 8 (registerDecision8's own fixture, a Known
// 1750 Hz tone_rx): if the gate answered after rung 8 rather than before it,
// this case would see a *RefusalError naming "decision 8" instead of the
// gate's plain *driver.WriteRefusedError.
func TestWriteChannel_TheCapabilityGateAnswersFirstEvenWhenASemanticRungWouldAlsoRefuse(t *testing.T) {
	sess, port := openTestSession(t, RealHardware, populatedImage(t, 7, occupiedSimplex()))
	ch := simplexChannel("007")
	ch.Data.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: 17500}
	_, err := sess.WriteChannel(context.Background(), ch)
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v (%T), want a *driver.WriteRefusedError", err, err)
	}
	var semantic *RefusalError
	if errors.As(err, &semantic) {
		t.Fatalf("the gate answered with a SEMANTIC refusal naming %q: the capability gate comes BEFORE rung 8, which this channel also trips", semantic.Register)
	}
	assertNoWireTraffic(t, port, "an unconsented real-hardware write that also trips rung 8")
}

// TestWriteChannel_ConsentIsTheRouteThroughTheCapabilityGate is the gate's
// positive control, and it runs on the SAME RealHardware profile: consent
// re-labels the write-side Unverified fields ConsentedUnverified at session
// assembly, and CanWrite is true for that state.
//
// CONSENT WIDENS WHAT MAY BE ATTEMPTED, NEVER HOW CAREFULLY: the write still
// performs the pre-write read and still meets every rung below.
func TestWriteChannel_ConsentIsTheRouteThroughTheCapabilityGate(t *testing.T) {
	p := newRespondingPort(t, populatedImage(t, 7, occupiedSimplex()))
	d := New(RealHardware, testTiming(), WithConsentedUnverifiedWrites())
	s, err := d.Open(context.Background(), p.Port(), driver.Identity{Port: "/dev/test"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	res, err := s.WriteChannel(context.Background(), simplexChannel("007"))
	if err != nil {
		t.Fatalf("a consented write: %v", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Sent {
		t.Fatalf("Steps = %+v, want one MA0 step Sent", res.Steps)
	}
	want := append(slices.Clone(probeFrames), "MA0007;", simplexSetFrame)
	if got := p.Transcript(); !slices.Equal(got, want) {
		t.Errorf("transcript = %v,\nwant %v", got, want)
	}
}

// TestWriteChannel_AFieldTheRowDoesNotPublishIsRefusedPreWireNamingTheField
// is rung 5's other half, and it fires on a SIMULATED session where the eight
// graded fields are all Supported: a caller handing this driver a Known IP+ —
// an Icom concept with no position in this frame and no mention in this book —
// is refused BY NAME rather than having the value dropped from a frame with
// nowhere to put it.
func TestWriteChannel_AFieldTheRowDoesNotPublishIsRefusedPreWireNamingTheField(t *testing.T) {
	ch := simplexChannel("007")
	ch.Data.IPPlus = codeplug.BoolField{State: codeplug.Known, Value: true}
	sess, port := openTestSession(t, Simulated, radioImage{})
	_, err := sess.WriteChannel(context.Background(), ch)
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v (%T), want a *driver.WriteRefusedError", err, err)
	}
	if !slices.Equal(refused.Fields, []spec.Field{spec.FieldIPPlus}) {
		t.Errorf("Fields = %v, want [ip_plus]", refused.Fields)
	}
	assertNoWireTraffic(t, port, "a Known ip_plus")
}

// TestWriteChannel_Decision9RefusesAChannelWithNoKnownTXDisposition is rung 6,
// and its fixture MUST be a file-loaded or CHIRP-imported channel (M-E8).
//
// ONE MA0 ANSWER CARRIES BOTH FREQUENCIES AND THE SPLIT FLAG, so a channel
// this programme has READ has a Known tx_frequency by construction and can
// never reach this rung — which is the difference between this pair and pair
// 1, where the same clause was a blanket obstruction. The second subtest is
// that statement as a pin: a fresh-read channel is NOT refused here.
func TestWriteChannel_Decision9RefusesAChannelWithNoKnownTXDisposition(t *testing.T) {
	t.Run("a file-loaded channel with no TX disposition", func(t *testing.T) {
		ch := simplexChannel("007")
		ch.Data.TxFreqHz = codeplug.FreqField{State: codeplug.Unknown}
		sess, port := openTestSession(t, Simulated, radioImage{})
		_, err := sess.WriteChannel(context.Background(), ch)
		ref := assertRegister(t, err, registerDecision9, "an Unknown tx_frequency")
		if !slices.Equal(ref.Fields, []spec.Field{spec.FieldTxFrequency}) {
			t.Errorf("Fields = %v, want [tx_frequency]", ref.Fields)
		}
		assertNoWireTraffic(t, port, "an Unknown tx_frequency")
	})

	t.Run("a fresh-read channel is not refused for this reason", func(t *testing.T) {
		sess, _ := openTestSession(t, Simulated, populatedImage(t, 7, occupiedSimplex()))
		ch, err := sess.ReadChannel(context.Background(), "007")
		if err != nil {
			t.Fatalf("ReadChannel: %v", err)
		}
		if ch.Data.TxFreqHz.State != codeplug.Known {
			t.Fatalf("a fresh read left tx_frequency %v, want Known — M-E7", ch.Data.TxFreqHz.State)
		}
		if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
			t.Fatalf("writing back a freshly read channel: %v", err)
		}
	})
}

// TestWriteChannel_AModeThisRowDoesNotPublishIsRefused is rung 7.
//
// IT IS A PLAIN FLEET REFUSAL AND NAMES NO REGISTER ENTRY, deliberately: what
// produces it is the printed legend (890:3976-3992), not an assumption, and
// putting a register name on an ordinary unpublished mode would attribute a
// typo to an unlifted claim. The legend's two "Unused" values, '0' and '8'
// (890:3977, 890:3985), are unreachable from a NAME — the layout carries no
// name for either — so this rung is where the ladder answers for them, and
// TestModeWire_IsTheExactInverseOfModeName is the other half of that proof.
func TestWriteChannel_AModeThisRowDoesNotPublishIsRefused(t *testing.T) {
	ch := simplexChannel("007")
	ch.Data.Mode = "C4FM"
	sess, port := openTestSession(t, Simulated, radioImage{})
	_, err := sess.WriteChannel(context.Background(), ch)
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v (%T), want a *driver.WriteRefusedError", err, err)
	}
	var semantic *RefusalError
	if errors.As(err, &semantic) {
		t.Errorf("err names register %q: an unpublished mode is not an assumption's refusal", semantic.Register)
	}
	if !slices.Equal(refused.Fields, []spec.Field{spec.FieldMode}) {
		t.Errorf("Fields = %v, want [mode]", refused.Fields)
	}
	if !strings.Contains(refused.Reason, "C4FM") {
		t.Errorf("reason %q does not quote the mode it refused", refused.Reason)
	}
	assertNoWireTraffic(t, port, "an unpublished mode")
}

// TestWriteChannel_Decision8RefusesAKnown1750HzToneRx is rung 8, and the
// asymmetry it enforces is a capability-model limit rather than a radio one:
// spec.Capabilities carries ONE tone domain and one AdmitsTone predicate for
// both directions, the published TN chart runs 00-50 with 1750.0 Hz at index
// 50 (890:5149-5163), and the printed CN chart stops at 49 (890:1354-1369).
//
// THE BOUND IS CONSULTED FROM THE SAME PLACE AS ITS DATUM: the layout's own
// MaxCTCSSIndex, which is core/kw/ma's reading of that chart and also what
// the MA0 builder enforces. A 1750 Hz tone_rx cannot come OFF a radio — the
// parser refuses the index — so this rung is about a value arriving from a
// FILE.
func TestWriteChannel_Decision8RefusesAKnown1750HzToneRx(t *testing.T) {
	ch := simplexChannel("007")
	ch.Data.ToneRx = codeplug.ToneField{State: codeplug.Known, Value: 17500}
	sess, port := openTestSession(t, Simulated, radioImage{})
	_, err := sess.WriteChannel(context.Background(), ch)
	ref := assertRegister(t, err, registerDecision8, "a Known 1750 Hz tone_rx")
	if !slices.Equal(ref.Fields, []spec.Field{spec.FieldToneRx}) {
		t.Errorf("Fields = %v, want [tone_rx]", ref.Fields)
	}
	assertNoWireTraffic(t, port, "a Known 1750 Hz tone_rx")
}

// TestWriteChannel_A1750HzToneTxIsWritten is rung 8's positive control and the
// reason the capability table publishes fifty-one tones rather than fifty:
// index 50 is a tone TN itself prints, and refusing it on transmit would be
// strictly worse than refusing it on receive.
func TestWriteChannel_A1750HzToneTxIsWritten(t *testing.T) {
	ch := simplexChannel("007")
	ch.Data.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: 17500}
	sess, port := openTestSession(t, Simulated, populatedImage(t, 7, occupiedSimplex()))
	if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	got := port.Transcript()
	// P6 is at positions 21-22, index 50 rendered as the CAT tone number.
	if set := got[len(got)-1]; set[20:22] != "50" {
		t.Errorf("the Set's P6 is %q, want \"50\" (890:5162)", set[20:22])
	}
}

// TestWriteChannel_ANameOutsideP13sDomainIsRefused is rung 9, and its three
// causes are kept apart because their EVIDENCE differs.
//
// The width is DOCUMENTED — "Up to 10 characters" (890:3208-3209). The ';'
// exclusion is FORCED rather than assumed: it terminates a frame, and a name
// carrying one would split the frame at the radio's own parser. Only the
// CHARSET is an assumption, and A2 is its register home — so only that case
// carries a register entry.
func TestWriteChannel_ANameOutsideP13sDomainIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name, tag, register string
	}{
		{"an eleventh character", "GB3IVGB3IV1", ""},
		{"an embedded terminator", "GB3IV;", ""},
		{"a byte outside the charset", "GB3IV\x1f", registerA2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ch := simplexChannel("007")
			ch.Data.Tag = tc.tag
			sess, port := openTestSession(t, Simulated, radioImage{})
			_, err := sess.WriteChannel(context.Background(), ch)
			if tc.register != "" {
				ref := assertRegister(t, err, tc.register, tc.name)
				if !slices.Equal(ref.Fields, []spec.Field{spec.FieldTag}) {
					t.Errorf("Fields = %v, want [tag]", ref.Fields)
				}
			} else {
				var refused *driver.WriteRefusedError
				if !errors.As(err, &refused) {
					t.Fatalf("err = %v (%T), want a *driver.WriteRefusedError", err, err)
				}
				var semantic *RefusalError
				if errors.As(err, &semantic) {
					t.Errorf("err names register %q: this cause is printed, not assumed", semantic.Register)
				}
				if !slices.Equal(refused.Fields, []spec.Field{spec.FieldTag}) {
					t.Errorf("Fields = %v, want [tag]", refused.Fields)
				}
			}
			assertNoWireTraffic(t, port, tc.name)
		})
	}
}

// TestWriteChannel_RefusesWhenThePreWriteReadsP11DisagreesWithP8 pins the
// check between the read and rung 10, from the T12 review (HIGH-1): no layer
// of this row holds the invariant P11 = 1 iff P8 != 0, so a pre-write answer
// whose P11 disagrees with its own P8 is a frame the neutral model cannot
// represent — read.go drops P11 entirely and candidate rebuilds it from
// tx_frequency, so writing such a channel back would flip the radio's own
// split flag with nothing in the file asking for it.
//
// BOTH FRAMES ARE HAND-SPELT FROM THE RULER (890:3164-3221), not built
// through the codec: core/kw/ma's own ParseMA0Answer decodes P11
// unconditionally at byte 38 while P8-P10 are domain-checked only when they
// carry content (A16, codec890.go:163-181), so a frame no builder would
// produce is exactly what a real radio could still answer, and only a
// hand-spelt fixture can pose it.
func TestWriteChannel_RefusesWhenThePreWriteReadsP11DisagreesWithP8(t *testing.T) {
	for _, tc := range []struct {
		name   string
		answer string
		quote  string
	}{
		{
			name:   "P11 says simplex but P8 carries a split frequency",
			answer: "MA0007000142500002035012000142550002001GB3IV;",
			quote:  "split=false while its P8 split transmission frequency is 14255000",
		},
		{
			name:   "P11 says split but P8-P10 are the printed zeroed side",
			answer: "MA0007000142500002035012000000000000011GB3IV;",
			quote:  "split=true while its P8 split transmission frequency is 0",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sess, port := openTestSession(t, Simulated, radioImage{ma0Answers: map[string]string{"007": tc.answer}})
			_, err := sess.WriteChannel(context.Background(), simplexChannel("007"))
			var refused *driver.WriteRefusedError
			if !errors.As(err, &refused) {
				t.Fatalf("err = %v (%T), want a *driver.WriteRefusedError", err, err)
			}
			var semantic *RefusalError
			if errors.As(err, &semantic) {
				t.Errorf("err names register %q: this refusal's authority is a printed line, not an assumption", semantic.Register)
			}
			for _, want := range []string{tc.quote, "890:3191-3203", "890:3217-3218"} {
				if !strings.Contains(refused.Reason, want) {
					t.Errorf("reason %q does not quote %s", refused.Reason, want)
				}
			}
			assertReadButNoSet(t, port, "007", tc.name)
		})
	}
}

// TestWriteChannel_A3RefusesABlankTarget is rung 10, the FIRST read-dependent
// rung: the pre-write read's answer satisfies the empty predicate, so the
// target channel is unassigned NOW, and whether an MA0 Set can create one is
// A3 — wholly unprinted for this command while five of its siblings print an
// unassigned-channel prohibition.
//
// THIS PROGRAMME DOES NOT CREATE CHANNELS. The lift is hardware item 1
// (L-HW-3), the gate for one-frame writes on this row, and until it runs a
// fresh radio is not programmable by this driver — which is exactly what
// "registered" must not be read to mean.
func TestWriteChannel_A3RefusesABlankTarget(t *testing.T) {
	sess, port := openTestSession(t, Simulated, blankTargetImage(7))
	res, err := sess.WriteChannel(context.Background(), simplexChannel("007"))
	ref := assertRegister(t, err, registerA3, "a blank target")
	if !strings.Contains(ref.Reason, "hardware item 1") {
		t.Errorf("reason %q does not name the observation that would lift it", ref.Reason)
	}
	if len(res.Steps) != 0 {
		t.Errorf("Steps = %+v, want none: the Set was never built", res.Steps)
	}
	assertReadButNoSet(t, port, "007", "a blank target")
}

// TestWriteChannel_A3RefusesABlankTargetAndLogsTheResidue is LOW-2 from the
// T12 review: the pre-write read can meet the same A21 residue ReadChannel
// reports (erratum E4 — this book's blank note stops at P12), and the A3
// refusal must not let it go unlogged. A caller told the channel is blank
// should also learn its name window is not empty — the one fact that would
// say the slot is not as fresh as the refusal implies.
func TestWriteChannel_A3RefusesABlankTargetAndLogsTheResidue(t *testing.T) {
	var log recordingLogger
	sess, port := openTestSession(t, Simulated, radioImage{ma0Answers: map[string]string{
		"007": blankAnswer890(7, "OLDNAME"),
	}}, WithTransportLogger(&log))

	_, err := sess.WriteChannel(context.Background(), simplexChannel("007"))
	assertRegister(t, err, registerA3, "a blank target with residue")
	assertReadButNoSet(t, port, "007", "a blank target with residue")

	if got := log.lines(); len(got) != 1 || !strings.Contains(got[0], "OLDNAME") {
		t.Errorf("the residue was not logged: logger saw %v", got)
	}
}

// TestWriteChannel_Decision9RefusesASecondarySideTheSetWouldNotReproduce is
// rung 11, the second read-dependent rung, and it is where this pair's one
// real LOSS is enforced.
//
// codeplug.ChannelData has ONE mode and ONE FM width; the record gives the
// transmit side its own P9 mode and its own P10 width (890:3193-3200), and
// those have no home in the neutral model. So a Set built from a candidate
// channel emits the PRIMARY mode into P9 and the primary width into P10 —
// P10 by the book's own rule, "set the same setting on the transmission side
// and the reception side for FM normal / narrow information (P4, P10)"
// (890:3219-3221), which the codec enforces as an invariant. A channel whose
// radio-side secondary values differ from that is READABLE BUT NOT REWRITABLE,
// and it is refused NAMING THE P-NUMBERS AND BOTH VALUES rather than being
// flattened.
func TestWriteChannel_Decision9RefusesASecondarySideTheSetWouldNotReproduce(t *testing.T) {
	t.Run("an independent transmit mode", func(t *testing.T) {
		rec := populated()
		rec.TXMode = '5' // AM, where the receive side is USB
		sess, port := openTestSession(t, Simulated, populatedImage(t, 7, rec))
		ch := simplexChannel("007")
		ch.Data.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: 14_255_000}
		_, err := sess.WriteChannel(context.Background(), ch)
		ref := assertRegister(t, err, registerDecision9, "an independent transmit mode")
		for _, want := range []string{"P9", "'5'", "'2'"} {
			if !strings.Contains(ref.Reason, want) {
				t.Errorf("reason %q does not quote %s", ref.Reason, want)
			}
		}
		if len(ref.Fields) != 0 {
			t.Errorf("Fields = %v, want none: the record's secondary side has no spec.Field of its own", ref.Fields)
		}
		assertReadButNoSet(t, port, "007", "an independent transmit mode")
	})

	// THE OTHER DIRECTION, and the one write.go's own P8/P11 paragraph now
	// turns on: clearing tx_frequency to make a currently-split channel
	// simplex is NOT an edit this model can carry. The candidate emits
	// TXMode 0 — the printed zeroed split side (890:3217-3218) — against the
	// radio's live P9, so this rung refuses it structurally, every time. The
	// comment claimed the opposite until review s2-close-review-opus-1.md
	// MED-3, which is the standing rule broken: no comment may claim more
	// than its test pins.
	t.Run("a split slot the candidate would make simplex", func(t *testing.T) {
		sess, port := openTestSession(t, Simulated, populatedImage(t, 7, populated()))
		// simplexChannel carries a Known ZERO tx_frequency, which is this
		// model's whole spelling of "make it simplex".
		ch := simplexChannel("007")
		if got := ch.Data.TxFreqHz; got.State != codeplug.Known || got.Value != 0 {
			t.Fatalf("the fixture's tx_frequency is %+v, want Known 0", got)
		}
		_, err := sess.WriteChannel(context.Background(), ch)
		ref := assertRegister(t, err, registerDecision9, "a split slot made simplex")
		for _, want := range []string{"P9", "'2'", "the printed zeroed simplex side (890:3217-3218)"} {
			if !strings.Contains(ref.Reason, want) {
				t.Errorf("reason %q does not quote %s", ref.Reason, want)
			}
		}
		assertReadButNoSet(t, port, "007", "a split slot made simplex")
	})

	t.Run("a simplex slot the candidate would make split", func(t *testing.T) {
		rec := populated()
		rec.TXFreqHz, rec.TXMode, rec.Split = 0, 0, false
		sess, port := openTestSession(t, Simulated, populatedImage(t, 7, rec))
		ch := simplexChannel("007")
		ch.Data.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: 14_255_000}
		_, err := sess.WriteChannel(context.Background(), ch)
		assertRegister(t, err, registerDecision9, "a simplex slot made split")
		assertReadButNoSet(t, port, "007", "a simplex slot made split")
	})

	t.Run("an independent transmit width", func(t *testing.T) {
		rec := populated()
		rec.Mode, rec.TXMode = '4', '4' // FM on both sides
		rec.FMNarrow, rec.TXFMNarrow = true, true
		sess, port := openTestSession(t, Simulated, populatedImage(t, 7, rec))
		ch := simplexChannel("007")
		ch.Data.Mode = "FM" // the wide twin: P4 '0', so the Set would emit P10 '0'
		ch.Data.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: 14_255_000}
		_, err := sess.WriteChannel(context.Background(), ch)
		ref := assertRegister(t, err, registerDecision9, "an independent transmit width")
		if !strings.Contains(ref.Reason, "P10") {
			t.Errorf("reason %q does not name P10", ref.Reason)
		}
		assertReadButNoSet(t, port, "007", "an independent transmit width")
	})
}
