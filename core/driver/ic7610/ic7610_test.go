// SPDX-License-Identifier: GPL-3.0-or-later

package ic7610

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7610 "github.com/gm5dna/open-rig-programmer/core/civ/ic7610"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// goldenRecord is the 25-byte record the G leg derived by hand from PDF
// p.12 and committed as core/civ/ic7610/testdata/ic7610-vectors.golden's
// `set-record-name-with-space` vector. Written out here in wire bytes
// rather than built by the codec: a fixture assembled by the encoder under
// test would agree with a wrong offset as happily as a right one.
//
//	offset  0      00            UNMAPPED (E6): SELECT-group nibble, OFF
//	offset  1-5    00 00 25 14 00  14 250 000 Hz, little-endian BCD
//	offset  6      01            USB
//	offset  7      01            FIL1
//	offset  8      01            hi = data mode OFF (E6), lo = TONE
//	offset  9-11   00 08 85      tone TX 885 deci-Hz (88.5 Hz)
//	offset  12-14  00 10 00      tone RX 1000 deci-Hz (100.0 Hz)
//	offset  15-24  "HOME QTH01"
var goldenRecord = []byte{
	0x00,
	0x00, 0x00, 0x25, 0x14, 0x00,
	0x01,
	0x01,
	0x01,
	0x00, 0x08, 0x85,
	0x00, 0x10, 0x00,
	0x48, 0x4F, 0x4D, 0x45, 0x20, 0x51, 0x54, 0x48, 0x30, 0x31,
}

// The two probe frames this package's tests compare byte for byte, from
// the plan's ONE TABLE and from the golden vectors' own `read-record` and
// `read-transceiver-id` lines.
var (
	idReadFrame = []byte{0xFE, 0xFE, 0x98, 0xE0, 0x19, 0x00, 0xFD}
)

func memReadFrame(ch int) []byte {
	hi, lo := encodeChannel(ch)
	return []byte{0xFE, 0xFE, 0x98, 0xE0, 0x1A, 0x00, hi, lo, 0xFD}
}

// openWith opens a session against p, failing the test on error.
func openWith(t *testing.T, p *scriptedPort, opts ...Option) *Session {
	t.Helper()
	d := New(Simulated, opts...)
	sess, err := d.Open(t.Context(), p.Port(), driver.Identity{Port: "/dev/scripted"})
	if err != nil {
		t.Fatalf("Open: %v\ntranscript:\n  %s", err, hexFrames(p.Transcript()))
	}
	s, ok := sess.(*Session)
	if !ok {
		t.Fatalf("Open returned %T, want *Session", sess)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// occupiedRadio is the ordinary image: an ID answer and a record in
// channel 1, so the probe fingerprints on its first read.
func occupiedRadio() radioImage {
	return radioImage{
		idToken: []byte{0x98},
		records: map[int][]byte{1: goldenRecord},
	}
}

// TestOpen_AddressMatchedIDIsRequired pins spec D3.2's opening move: what
// identifies the radio is that an ADDRESS-MATCHED 19 00 reply arrived at
// all, so a reply from another station, a broadcast, or silence must not
// open a session.
//
// The address check is the CODEC's (Profile.answerBody compares both the
// `to` and the `from` byte) reached through
// Profile.TransceiverIDAnswerMatcher, never a rule written here — R1: the
// matcher comes from the codec.
//
// Open takes ownership of the port on BOTH outcomes
// (core/driver/driver.go's Driver contract), so each failing arm also
// asserts the port was closed.
func TestOpen_AddressMatchedIDIsRequired(t *testing.T) {
	t.Run("address-matched answer opens", func(t *testing.T) {
		t.Parallel()
		p := newScriptedPort(t, occupiedRadio())
		s := openWith(t, p)
		if s.Identity().Port != "/dev/scripted" {
			t.Errorf("Identity().Port = %q, want the caller-supplied path", s.Identity().Port)
		}
	})

	for _, tt := range []struct {
		name string
		img  radioImage
		why  string
	}{
		{
			name: "answer from another station",
			img:  radioImage{idToken: []byte{0x98}, idFrom: 0x94},
			why:  "a 19 00 answer from 0x94 is another radio on the same bus, not this one",
		},
		{
			name: "broadcast answer",
			img:  radioImage{idToken: []byte{0x98}, idBroadcast: true},
			why:  "a to=00 frame is a transceive broadcast; civ's accumulator counts and drops it before any engine event",
		},
		{
			name: "silence",
			img:  radioImage{},
			why:  "silence is silence — the failure mode a wrong CI-V address or a wrong assumed baud produces",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := newScriptedPort(t, tt.img)
			d := New(RealHardware)
			sess, err := d.Open(t.Context(), p.Port(), driver.Identity{})
			if err == nil {
				_ = sess.Close()
				t.Fatalf("Open succeeded: %s", tt.why)
			}
			// Open owns the port on both outcomes, so it must have
			// closed it: a further read now fails.
			if _, rerr := p.Port().Read(make([]byte, 1)); rerr == nil {
				t.Error("the port is still open after a failed Open - Driver.Open takes ownership on BOTH outcomes")
			}
		})
	}
}

// TestOpen_IDTokenIsRecordedNeverMatched: the 19 00 reply VALUE is
// undocumented on every model in this tier (spec D5 entry 7, matrix lift
// R7), so this driver records it and compares it against nothing. Three
// different tokens all open a session, and Identity().CATID is the static
// address followed by whatever the radio said.
func TestOpen_IDTokenIsRecordedNeverMatched(t *testing.T) {
	for _, tt := range []struct {
		token []byte
		want  string
	}{
		{[]byte{0x98}, "9898"},
		{[]byte{0x00}, "9800"},
		{[]byte{0xA4, 0x12}, "98a412"},
	} {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			img := occupiedRadio()
			img.idToken = tt.token
			p := newScriptedPort(t, img)
			s := openWith(t, p)
			if got := s.Identity().CATID; got != tt.want {
				t.Errorf("Identity().CATID = %q, want %q (the static address 98 followed by the recorded token)", got, tt.want)
			}
			if got := s.OpenDiagnostics().IDToken; !reflect.DeepEqual(got, tt.token) {
				t.Errorf("OpenDiagnostics().IDToken = % x, want % x", got, tt.token)
			}
		})
	}
}

// TestOpen_FingerprintConfirmsOn25: after the ID, the driver reads 1A 00
// over channels 1..probeSlotCount until one answers with a record, and a
// 25-byte RECORD-ONLY length confirms the fingerprint (spec D3.2).
//
// Twenty-five is the record EXCLUDING the two channel-selector bytes
// (spec Erratum 1, matrix Errata (rev 1) erratum 1). The data area
// INCLUDING them is 27 and the whole set frame is 34; getting that
// accounting wrong is the one failure the matrix review called
// downstream-fatal.
func TestOpen_FingerprintConfirmsOn25(t *testing.T) {
	for _, occupied := range []int{1, 4, 10} {
		t.Run("occupied slot", func(t *testing.T) {
			img := radioImage{idToken: []byte{0x98}, records: map[int][]byte{occupied: goldenRecord}}
			p := newScriptedPort(t, img)
			s := openWith(t, p)
			length, confirmed := s.Fingerprint()
			if !confirmed || length != civic7610.RecordOnlyLength {
				t.Errorf("Fingerprint() = (%d, %v), want (%d, true)", length, confirmed, civic7610.RecordOnlyLength)
			}
			r := s.OpenDiagnostics()
			if !r.Fingerprinted || r.RecordLength != 25 {
				t.Errorf("OpenDiagnostics() = %+v, want Fingerprinted with RecordLength 25", r)
			}
			if r.SlotsTried != occupied {
				t.Errorf("SlotsTried = %d, want %d - the search stops at the first occupied slot", r.SlotsTried, occupied)
			}
		})
	}
}

// TestOpen_EmptyRadioOpensUnfingerprinted: every slot in the search
// answers FA, so there is no record to measure. The session opens ON
// ADDRESS EVIDENCE ALONE and says so (spec D3.2, D5 entry 2(a), matrix
// lift R2a).
//
// Tier ruling T4: Engine.Do consumes the FA and returns
// transport.ErrRejected with NO frame, so the driver's "keep looking"
// branch keys on errors.Is(err, transport.ErrRejected) and never on "an FA
// arrived".
func TestOpen_EmptyRadioOpensUnfingerprinted(t *testing.T) {
	p := newScriptedPort(t, radioImage{idToken: []byte{0x98}})
	s := openWith(t, p)
	if length, confirmed := s.Fingerprint(); confirmed || length != 0 {
		t.Errorf("Fingerprint() = (%d, %v), want (0, false)", length, confirmed)
	}
	r := s.OpenDiagnostics()
	if r.Fingerprinted || r.RecordLength != 0 {
		t.Errorf("OpenDiagnostics() = %+v, want UNFINGERPRINTED", r)
	}
	if r.SlotsTried != probeSlotCount {
		t.Errorf("SlotsTried = %d, want %d - the whole bounded search ran", r.SlotsTried, probeSlotCount)
	}
}

// TestOpen_WrongLengthRefusesWithoutAttribution: a record at any length
// but 25 fails the open with an error satisfying
// errors.Is(err, driver.ErrWrongRadio).
//
// THE ERROR NAMES NO FOUND MODEL, and that is deliberate. Cross-model
// record-length distinctness is a TIER-LEVEL Wave-4 check; this model has
// no registered sibling and this package has no table of other radios'
// record lengths, so naming one would be a claim it cannot support. What
// the message carries instead is the length found, the length expected,
// and that the expected length is itself an ASSUMED derivation from one
// document (D5 entry 6, matrix lift R6).
func TestOpen_WrongLengthRefusesWithoutAttribution(t *testing.T) {
	for _, n := range []int{24, 26, 27, 39} {
		t.Run("record length", func(t *testing.T) {
			rec := make([]byte, n)
			copy(rec, goldenRecord)
			img := radioImage{idToken: []byte{0x98}, records: map[int][]byte{1: rec}}
			p := newScriptedPort(t, img)
			d := New(RealHardware)
			sess, err := d.Open(t.Context(), p.Port(), driver.Identity{})
			if err == nil {
				_ = sess.Close()
				t.Fatalf("a %d-byte record opened a session", n)
			}
			if !errors.Is(err, driver.ErrWrongRadio) {
				t.Errorf("err = %v, want one satisfying errors.Is(err, driver.ErrWrongRadio)", err)
			}
			var wrong *driver.WrongRadioError
			if errors.As(err, &wrong) && wrong.GotModel != "" {
				t.Errorf("the refusal names a found model (%q) - cross-model attribution is a Wave-4 tier check and this model has no registered sibling", wrong.GotModel)
			}
			// THE TYPED ERROR, not a substring sweep: a Got field wired
			// to the wrong value would slip past a message check that
			// only looked for "25".
			var mismatch *RecordLengthMismatchError
			if !errors.As(err, &mismatch) {
				t.Fatalf("err = %v, want a *RecordLengthMismatchError", err)
			}
			if mismatch.Got != n {
				t.Errorf("*RecordLengthMismatchError.Got = %d, want %d - the FOUND length is half of what the plan requires the refusal to give", mismatch.Got, n)
			}
			if mismatch.Want != civic7610.RecordOnlyLength {
				t.Errorf("*RecordLengthMismatchError.Want = %d, want %d", mismatch.Want, civic7610.RecordOnlyLength)
			}
			// And the message carries all three things the plan names:
			// the length found, the length expected, and that the
			// expectation is itself an ASSUMED derivation.
			msg := err.Error()
			for _, want := range []string{strconv.Itoa(n), "25", "ASSUMED"} {
				if !strings.Contains(msg, want) {
					t.Errorf("the refusal %q does not mention %q", msg, want)
				}
			}
		})
	}
}

// TestOpen_WrongModelAtAnotherAddressJustTimesOut states the limitation
// plainly (spec D3.3). This driver builds every frame for 0x98 and the
// codec refuses any answer not from 0x98, so a radio at a different CI-V
// address — including a different Icom model at ITS factory address — is
// indistinguishable from an empty port: silence.
//
// OQ2's SECOND FAILURE MODE IS THE SAME ONE. A radio at a CI-V baud other
// than the assumed 19200 fails here identically, which is exactly why a
// wrong default-baud guess costs a clean timeout at Open and never a wrong
// byte. There is no --civ-address option and no baud sweep:
// internal/wiring opens the port from Capabilities().DefaultBaud, and
// Wave 3 may never touch internal/wiring.
func TestOpen_WrongModelAtAnotherAddressJustTimesOut(t *testing.T) {
	t.Parallel()
	// A radio that answers 19 00 perfectly well — from 0x94, an IC-9700's
	// factory address.
	p := newScriptedPort(t, radioImage{idToken: []byte{0x94}, idFrom: 0x94})
	d := New(RealHardware)
	sess, err := d.Open(t.Context(), p.Port(), driver.Identity{})
	if err == nil {
		_ = sess.Close()
		t.Fatal("a radio at another CI-V address opened a session")
	}
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("err = %v, want a timeout - a radio off 0x98 is indistinguishable from silence", err)
	}
	if errors.Is(err, driver.ErrWrongRadio) {
		t.Error("the refusal claims a wrong radio; nothing was heard from, so nothing can be attributed")
	}
}

// TestOpen_BroadcastFloodDoesNotReachTheEngine is R9-SPLIT half (a), and
// THIS IS THE TEST REV 2 GOT BACKWARDS.
//
// The scripted port emits to=00 frames continuously. civ's accumulator
// counts them and NEVER RETURNS them (core/civ/accumulator.go, "counted
// and NEVER RETURNED"), so they never become engine events, the drain's
// idle timer is never re-armed, and Engine.Init SUCCEEDS. No drain cap is
// reached and no engine event occurs.
//
// What rises is the ADAPTER's own counter. A test expecting a drain-cap
// event from a to=00 flood is unsatisfiable, and mislabelling the
// broadcast counter as a drain-cap diagnostic to make one pass is
// forbidden.
func TestOpen_BroadcastFloodDoesNotReachTheEngine(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio())
	p.startFlood(0x00)
	defer p.stopFlood()

	s := openWith(t, p)
	if s.OpenDiagnostics().InitDrainCapExceeded {
		t.Error("InitDrainCapExceeded is true under a BROADCAST flood - those frames never reach the engine, so no drain can hit its cap on them")
	}
	if _, confirmed := s.Fingerprint(); !confirmed {
		t.Error("the probe did not fingerprint through a broadcast flood - the flood must be invisible to it")
	}
	if got := s.WireStats().Unexpected; got == 0 {
		t.Fatal("WireStats().Unexpected is zero under a broadcast flood - the adapter's counter is the ONLY place this traffic is visible")
	}
	before := s.WireStats().Unexpected
	time.Sleep(50 * time.Millisecond)
	if after := s.WireStats().Unexpected; after <= before {
		t.Errorf("WireStats().Unexpected went %d -> %d, want it still rising while the flood runs", before, after)
	}
}

// TestOpen_AddressedFloodCapIsNonfatalWithDiagnostic is R9-SPLIT half (b).
//
// The port emits frames addressed to THIS CONTROLLER (to = E0). Those DO
// pass the address filter, DO become engine events, and DO re-arm the
// drain's idle timer, so Engine.Init's drain reaches its absolute cap and
// returns ErrDrainCapExceeded. That INITIAL failure is
// NONFATAL-WITH-DIAGNOSTIC: E1's drain is bounded precisely so it "cannot
// fail the open", and the session records what happened.
//
// ONLY A CONTROLLER-ADDRESSED FLOOD CAN PRODUCE THIS. An implementer who
// builds a broadcast flood and meets a false here should read half (a).
func TestOpen_AddressedFloodCapIsNonfatalWithDiagnostic(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio())
	p.startFlood(civ.ControllerAddressDefault)
	// The flood must outlive Init's 600 ms absolute cap and stop before
	// the probe needs an answer of its own.
	p.stopFloodAfter(750 * time.Millisecond)

	s := openWith(t, p)
	if !s.OpenDiagnostics().InitDrainCapExceeded {
		t.Error("InitDrainCapExceeded is false under a CONTROLLER-ADDRESSED flood - Init's drain cannot have found its idle gap")
	}
	if _, confirmed := s.Fingerprint(); !confirmed {
		t.Error("the session did not fingerprint - a capped Init drain must not prevent the probe from running")
	}
}

// TestOpen_LaterQuarantineFailureFailsClosed is the other half of the
// nonfatal/fail-closed rule, and the two are pinned by two tests so
// neither can be relaxed into the other.
//
// The line is quiet while Init drains, so Init succeeds. The
// controller-addressed flood then begins, and the first memory probe's
// read times out and RETRIES — and a retry's drain-to-quiet is a LATER
// quarantine drain. It fails at the same absolute cap, and that failure is
// an error the caller sees: Open fails closed.
func TestOpen_LaterQuarantineFailureFailsClosed(t *testing.T) {
	img := occupiedRadio()
	img.memSilent = true
	p := newScriptedPort(t, img)
	// Start the flood only once the ID probe has been answered, so Init's
	// own drain completes cleanly and the failure is unambiguously a
	// LATER one.
	go func() {
		for {
			if len(p.Transcript()) >= 2 {
				p.startFlood(civ.ControllerAddressDefault)
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()
	defer p.stopFlood()

	d := New(RealHardware)
	sess, err := d.Open(t.Context(), p.Port(), driver.Identity{})
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open succeeded through a later drain-cap failure - only the INITIAL Init drain is nonfatal")
	}
	if !errors.Is(err, transport.ErrDrainCapExceeded) {
		t.Errorf("err = %v, want one satisfying errors.Is(err, transport.ErrDrainCapExceeded)", err)
	}
}

// TestOpen_InitWritesNothing: NO RADIO MUTATION AT INIT, EVER.
//
// E1's InitSequence() is EMPTY (core/civ/framing.go), which is a safety
// property rather than an omission: transceive broadcasts are excluded
// structurally, by address filtering, instead of by writing a
// transceive-off setting, so opening a session touches nothing outside the
// consent regime.
//
// An EXACT byte-sequence comparison, not a substring search: the only
// bytes an Open puts on the wire are the 19 00 read and the probe's 1A 00
// reads. No 1A 05, no transceive set, no clear, nothing else.
func TestOpen_InitWritesNothing(t *testing.T) {
	t.Run("occupied radio", func(t *testing.T) {
		p := newScriptedPort(t, occupiedRadio())
		openWith(t, p)
		want := [][]byte{idReadFrame, memReadFrame(1)}
		if got := p.Transcript(); !reflect.DeepEqual(got, want) {
			t.Errorf("Open wrote:\n  %s\nwant:\n  %s", hexFrames(got), hexFrames(want))
		}
	})
	t.Run("empty radio", func(t *testing.T) {
		p := newScriptedPort(t, radioImage{idToken: []byte{0x98}})
		openWith(t, p)
		want := [][]byte{idReadFrame}
		for ch := 1; ch <= probeSlotCount; ch++ {
			want = append(want, memReadFrame(ch))
		}
		if got := p.Transcript(); !reflect.DeepEqual(got, want) {
			t.Errorf("Open wrote:\n  %s\nwant:\n  %s", hexFrames(got), hexFrames(want))
		}
	})
}

// TestOpen_ControlLinesAreNeverToggled: the driver makes NO SetRTS or
// SetDTR call.
//
// transport.OpenSerial drives both lines low at open
// (core/transport/port.go, safety obligation 4) before this driver ever
// sees the port, and matrix §3.2 / ADDED-2 is why the driver must not
// touch them afterwards: this radio's 1A 05 00 95/00 96/00 97 rows assign
// USB SEND and CW/RTTY keying to USB1(A)/USB1(B) DTR or RTS, and the CI-V
// endpoint is one of those same USB serial ports. Erratum 8 records that
// the document never says WHICH of the two carries CI-V, which is why the
// conservative choice is the only defensible one: a driver that asserts
// RTS or DTR when it opens the CI-V port can KEY THE TRANSMITTER of a
// radio whose owner has set USB SEND to that line.
//
// transport.Port carries neither method, so the only way to reach them is
// a type assertion. This test is what makes that assertion visible if
// anyone ever writes one.
func TestOpen_ControlLinesAreNeverToggled(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio())
	s := openWith(t, p)
	_ = s.Close()
	if rts, dtr := p.controlLineCalls(); rts != 0 || dtr != 0 {
		t.Errorf("the driver made %d SetRTS and %d SetDTR calls, want none (matrix §3.2 / ADDED-2, lift R16)", rts, dtr)
	}
}

// TestDiagnostics_SumBothSidesOfTheFilter pins the DIAGNOSTICS CARRIER
// ruling.
//
// The neutral driver.SessionDiagnostics carries ONE aggregate counter, and
// a driver filling it from Engine.UnexpectedFrames alone would report a
// HEALTHY ZERO on a line saturated with transceive — the accumulator
// swallowed those frames before the engine could count one. So this
// session's Diagnostics SUMS both sides of the filter, and the test
// asserts the engine's own counter would have missed the burst entirely.
func TestDiagnostics_SumBothSidesOfTheFilter(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio())
	s := openWith(t, p)

	engineBefore := s.eng.UnexpectedFrames()
	p.startFlood(0x00)
	defer p.stopFlood()
	// Long enough for a good many frames at one every 2 ms.
	time.Sleep(120 * time.Millisecond)

	wire := s.WireStats().Unexpected
	if wire == 0 {
		t.Fatal("WireStats().Unexpected is zero after a broadcast burst")
	}
	if got := s.eng.UnexpectedFrames(); got != engineBefore {
		t.Errorf("Engine.UnexpectedFrames() moved %d -> %d during a broadcast burst; the whole premise of the carrier ruling is that it does NOT see these frames", engineBefore, got)
	}
	diag := s.Diagnostics()
	if diag.UnexpectedFrames < uint64(wire) {
		t.Errorf("Diagnostics().UnexpectedFrames = %d, which does not include the adapter's %d - neither number alone is the truth about this wire", diag.UnexpectedFrames, wire)
	}
}

// TestSessionAssertsTheStatsReporter: Open's two-result assertion is real.
//
// A compile-time `var _ civ.AccumulatorStatsReporter = ...` on the landed
// framing type is not available — that type is unexported — so this
// asserts through civ.NewFraming's result directly, which is the same
// value Open assigns.
func TestSessionAssertsTheStatsReporter(t *testing.T) {
	framing, err := civ.NewFraming(civic7610.Profile())
	if err != nil {
		t.Fatalf("civ.NewFraming: %v", err)
	}
	if _, ok := framing.(civ.AccumulatorStatsReporter); !ok {
		t.Fatal("the CI-V framing no longer satisfies civ.AccumulatorStatsReporter - this driver's diagnostics require it, and Open refuses to build a session without it")
	}
	p := newScriptedPort(t, occupiedRadio())
	s := openWith(t, p)
	if s.stats == nil {
		t.Fatal("the session did not retain the adapter's stats reporter")
	}
}

// TestOpen_OneFramingPerEngine: enablers fix wave X1 makes a second
// NewAccumulator call on one adapter fail closed (loudly), so the framing
// must be built INSIDE Open and never cached on the driver or shared
// across sessions. Two sequential Opens on one driver value prove it.
func TestOpen_OneFramingPerEngine(t *testing.T) {
	d := New(Simulated)
	for i := 0; i < 2; i++ {
		p := newScriptedPort(t, occupiedRadio())
		sess, err := d.Open(t.Context(), p.Port(), driver.Identity{})
		if err != nil {
			t.Fatalf("Open %d: %v", i, err)
		}
		_ = sess.Close()
	}
}

// TestCapabilitiesSwitch_FailsSafeOnAnUnrecognisedProfile: an
// unrecognised Profile value falls to the all-Unverified fail-safe arm
// through its OWN explicit branch, so a reader can see the fail-safe is a
// decision rather than a coincidence of writeTrialsComplete's state.
func TestCapabilitiesSwitch_FailsSafeOnAnUnrecognisedProfile(t *testing.T) {
	for _, p := range []Profile{RealHardware, Profile(42), Profile(-1)} {
		caps := New(p).Capabilities()
		if !reflect.DeepEqual(caps, capabilitiesUnverified()) {
			t.Errorf("Profile(%d) selects a capability set other than the all-Unverified one", p)
		}
	}
	if !reflect.DeepEqual(New(Simulated).Capabilities(), capabilitiesSimulated()) {
		t.Error("Simulated does not select the simulated capability set")
	}
	if got := New(RealHardware).Model(); got != "IC-7610" {
		t.Errorf("Model() = %q, want IC-7610", got)
	}
}

// TestSessionCapabilities_ConsentAppliesOnlyToRecognisedProfiles: the
// consent transform reaches the SESSION's effective set and never the
// driver's static one (which is what internal/wiring reads to decide
// whether to ASK for consent), and an unrecognised profile stays
// untransformed so the fail-safe direction survives consent.
func TestSessionCapabilities_ConsentAppliesOnlyToRecognisedProfiles(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio())
	d := New(RealHardware, WithConsentedUnverifiedWrites())
	if got := d.Capabilities().FieldSupport(bothBanks[0], mappedFields[0]).Write; got.String() != "Unverified" {
		t.Errorf("the driver's STATIC capabilities are consented (%v); consent belongs to the session's effective set", got)
	}
	sess, err := d.Open(t.Context(), p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = sess.Close() }()
	if got := sess.Capabilities().FieldSupport(bothBanks[0], mappedFields[0]).Write; got.String() != "ConsentedUnverified" {
		t.Errorf("the session's frequency write support = %v, want ConsentedUnverified", got)
	}

	q := newScriptedPort(t, occupiedRadio())
	unrecognised := New(Profile(42), WithConsentedUnverifiedWrites())
	usess, err := unrecognised.Open(t.Context(), q.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open (unrecognised profile): %v", err)
	}
	defer func() { _ = usess.Close() }()
	if usess.Capabilities().FieldSupport(bothBanks[0], mappedFields[0]).CanWrite() {
		t.Error("an unrecognised Profile produced a writable session under consent - the fail-safe direction must survive consent")
	}
}

// TestOpen_ProbeAnswerForAnotherChannelIsRefused pins tier ruling T2 in the
// OPEN path.
//
// T2 says the driver compares the decoded address of EVERY memory answer
// before ANY use of it. During the probe, "use" means TAKING THE ANSWER'S
// LENGTH AS THIS RADIO'S FINGERPRINT — so a bus neighbour (or a confused
// radio) answering for a different channel with a 25-byte record would
// fingerprint the session on evidence that was never about the slot asked
// for, and every later read would be trusted on that basis.
//
// probeSlot carries its own copy of the check because no Session exists
// yet; without this test, deleting that copy leaves the whole suite green.
// The diagnostic COUNT is deliberately not kept here — there is no Session
// to keep it on — and the typed error is what carries the event instead.
func TestOpen_ProbeAnswerForAnotherChannelIsRefused(t *testing.T) {
	for _, tt := range []struct {
		name    string
		answers func(asked int) int
		wantGot int
	}{
		{"the first probed slot answers for another channel", func(int) int { return 7 }, 7},
		{"the answer is off by one", func(asked int) int { return asked + 1 }, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := newScriptedPort(t, radioImage{
				idToken:       []byte{0x98},
				records:       map[int][]byte{1: goldenRecord},
				answerAddress: tt.answers,
			})
			d := New(RealHardware)
			sess, err := d.Open(t.Context(), p.Port(), driver.Identity{})
			if err == nil {
				_ = sess.Close()
				t.Fatal("Open fingerprinted the session on a record that was never about the channel it asked for")
			}
			var mismatch *AnswerMismatchError
			if !errors.As(err, &mismatch) {
				t.Fatalf("err = %v, want an *AnswerMismatchError (tier ruling T2 applies to the probe's answers too)", err)
			}
			if mismatch.Want.Channel != 1 || mismatch.Got.Channel != tt.wantGot {
				t.Errorf("*AnswerMismatchError = {Want: %s, Got: %s}, want {ch1, ch%d}", mismatch.Want, mismatch.Got, tt.wantGot)
			}
			// Driver.Open takes ownership of the port on BOTH outcomes.
			if _, rerr := p.Port().Read(make([]byte, 1)); rerr == nil {
				t.Error("the port is still open after a failed Open")
			}
		})
	}
}

// TestSessionCapabilities_IsADefensiveDeepCopy pins the claim
// cloneCapabilities' doc comment makes: THE COPY IS LOAD-BEARING.
//
// WriteChannel's capability gate re-checks against s.caps, the session's
// own value. A caller who could reach into what Capabilities() handed out
// and flip a FieldSupport — or move the tone range, which is a POINTER and
// would have been aliased by a plain struct copy — would be editing the
// write gate from outside it. So the test does not merely compare two
// returned values: it MUTATES one and then asks the gate.
func TestSessionCapabilities_IsADefensiveDeepCopy(t *testing.T) {
	p := newScriptedPort(t, occupiedRadio())
	s := openWith(t, p) // Simulated: the seven mapped fields are writable

	handed := s.Capabilities()

	// (a) Flip a bank's FieldSupport to Unsupported through the copy.
	mem, ok := handed.Bank(spec.BankMemory)
	if !ok {
		t.Fatal("no MEM bank")
	}
	mem.Fields[spec.FieldFrequency] = spec.FieldSupport{}
	for i := range handed.Banks {
		if handed.Banks[i].ID == spec.BankMemory {
			handed.Banks[i].Fields[spec.FieldFrequency] = spec.FieldSupport{}
			handed.Banks[i].Slots[0] = "XXX"
		}
	}
	// (b) Move the tone domain through the copy's POINTER.
	if handed.CTCSSToneRange == nil {
		t.Fatal("CTCSSToneRange is nil")
	}
	handed.CTCSSToneRange.MinDeciHz = 9000
	handed.CTCSSToneRange.MaxDeciHz = 9000
	// (c) Rewrite a top-level vocabulary through the copy.
	handed.Modes[0] = "MANGLED"
	handed.Filters[0] = "MANGLED"

	// A second call must be untouched by any of that.
	fresh := s.Capabilities()
	if got := fresh.FieldSupport(spec.BankMemory, spec.FieldFrequency); !got.CanWrite() {
		t.Error("mutating the handed-out Capabilities disarmed the session's frequency write support")
	}
	if b, _ := fresh.Bank(spec.BankMemory); b.Slots[0] != "001" {
		t.Errorf("the handed-out bank's Slots alias the session's: first slot is now %q", b.Slots[0])
	}
	if r := fresh.CTCSSToneRange; r == nil || r.MinDeciHz != 1 || r.MaxDeciHz != 2999 {
		t.Errorf("CTCSSToneRange = %+v, want {1, 2999, 1} - the pointer was aliased rather than copied", r)
	}
	if fresh.Modes[0] != "LSB" || fresh.Filters[0] != "FIL1" {
		t.Errorf("a top-level vocabulary was aliased: Modes[0] = %q, Filters[0] = %q", fresh.Modes[0], fresh.Filters[0])
	}

	// AND THE GATE ITSELF, which is the point: the write still goes
	// through, so nothing a caller did to the copy reached s.caps.
	if !s.caps.FieldSupport(spec.BankMemory, spec.FieldFrequency).CanWrite() {
		t.Fatal("the session's OWN capabilities were mutated through the handed-out copy")
	}
	if r := s.caps.CTCSSToneRange; r.MinDeciHz != 1 || r.MaxDeciHz != 2999 {
		t.Errorf("the session's OWN tone domain was moved through the handed-out copy: %+v", r)
	}
}
