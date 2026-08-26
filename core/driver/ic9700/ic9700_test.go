// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic9700"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// openWith opens a RealHardware session against a scripted radio and fails
// the test if it will not open. It is the ordinary path; the tests that
// are ABOUT Open's refusals call Open directly.
func openWith(t *testing.T, port *recordingPort) *ic9700.Session {
	t.Helper()
	sess, err := ic9700.New(ic9700.RealHardware).Open(context.Background(), port.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess.(*ic9700.Session)
}

// assertUnfingerprintedRecorded requires that the session RECORDED the
// unfingerprinted state rather than leaving it to be inferred from the
// absence of something.
func assertUnfingerprintedRecorded(t *testing.T, sess driver.Session) {
	t.Helper()
	s, ok := sess.(*ic9700.Session)
	if !ok {
		t.Fatalf("session is %T, not *ic9700.Session", sess)
	}
	if s.CIVDiagnostics().Fingerprinted {
		t.Error("Fingerprinted is true on a radio that answered FA to every read")
	}
}

func TestOpenSendsNothingButTheProbe(t *testing.T) {
	// NO RADIO MUTATION AT INIT, EVER. Every frame the driver writes
	// during Open must be one of the two READ grammars: the `19 00`
	// identity read and the `1A 00` memory read. Init itself sends
	// nothing at all — InitSequence is empty for CI-V — so a set frame
	// appearing here would be a mutation of somebody's radio performed
	// before the driver had established what radio it was.
	port := newRecordingPort(t, factoryAnswers())
	sess := openWith(t, port)
	_ = sess

	frames := port.frames()
	if len(frames) == 0 {
		t.Fatal("Open wrote nothing at all; the probe asks two questions")
	}
	for _, f := range frames {
		cn, sc, _ := civ.FrameCommand(f)
		switch {
		case cn == 0x19 && sc == 0x00:
		case cn == 0x1A && sc == 0x00 && len(f) == 10: // a read: address, no record
		default:
			t.Errorf("Open wrote % X — the tier writes nothing to a radio outside the consented memory-set path", f)
		}
	}
}

func TestOpenRequiresAnAddressMatchedReply(t *testing.T) {
	port := newRecordingPort(t, answersFrom(0xA4)) // a different radio's address
	sess, err := ic9700.New(ic9700.RealHardware).Open(context.Background(), port.Port(), driver.Identity{})
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open succeeded against a radio answering from another address")
	}
}

func TestProbeFingerprintsTheRecordNotTheDataArea(t *testing.T) {
	// The wire shows 114 bytes between `1A 00` and `FD`; the profile
	// declares 111. Mixing them is this model's characteristic bug, and
	// the error must report the RECORD-ONLY number in both halves.
	port := newRecordingPort(t, answersWithDataArea(114))
	openWith(t, port)

	port2 := newRecordingPort(t, answersWithDataArea(111)) // 108-byte record
	_, err := ic9700.New(ic9700.RealHardware).Open(context.Background(), port2.Port(), driver.Identity{})
	var rl *civ.RecordLengthError
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v, want *civ.RecordLengthError", err)
	}
	if rl.Got != 108 || !reflect.DeepEqual(rl.Want, []int{111}) {
		t.Errorf("RecordLengthError{Want:%v, Got:%d}, want {[111], 108}", rl.Want, rl.Got)
	}
}

func TestUnexpectedLengthIsRefusedWithoutAttribution(t *testing.T) {
	// Cross-model record-length distinctness is a TIER-level Wave-4
	// check. This driver must not name another model.
	port := newRecordingPort(t, answersWithDataArea(50))
	sess, err := ic9700.New(ic9700.RealHardware).Open(context.Background(), port.Port(), driver.Identity{})
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open accepted a record at a length this profile does not declare")
	}
	var wrong *driver.WrongRadioError
	if errors.As(err, &wrong) {
		t.Fatalf("driver attributed the radio to %q; Wave 3 may not claim cross-model distinctness", wrong.GotModel)
	}
}

func TestEmptyRadioOpensUnfingerprinted(t *testing.T) {
	port := newRecordingPort(t, allRejections()) // every 1A 00 read answers FA
	sess, err := ic9700.New(ic9700.RealHardware).Open(context.Background(), port.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("an empty radio must open on address evidence alone: %v", err)
	}
	defer sess.Close()
	if !strings.Contains(sess.Identity().CATID, "A2") {
		t.Errorf("CATID = %q, want the address half present", sess.Identity().CATID)
	}
	// the unfingerprinted diagnostic must be recorded, not inferred
	assertUnfingerprintedRecorded(t, sess)
}

func TestTheProbeSearchIsBoundedAndStopsAtTheFirstRecord(t *testing.T) {
	// N = 8 is a CHOICE (spec D3.2 leaves the bound to the driver), and
	// both halves of it are asserted: an empty radio is asked exactly
	// eight times and no more, and a radio with a record in the first
	// channel is asked exactly once.
	empty := newRecordingPort(t, allRejections())
	sess, err := ic9700.New(ic9700.RealHardware).Open(context.Background(), empty.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sess.Close()
	if got := empty.countReads(); got != 8 {
		t.Errorf("an empty radio drew %d memory reads, want the bounded 8", got)
	}

	occupied := newRecordingPort(t, factoryAnswers())
	s2 := openWith(t, occupied)
	if got := occupied.countReads(); got != 1 {
		t.Errorf("a radio answering the first read drew %d reads, want 1", got)
	}
	if !s2.CIVDiagnostics().Fingerprinted {
		t.Error("a 111-byte record did not fingerprint the session")
	}
}

func TestTheProbeRefusesToFingerprintFromAnotherSlotsRecord(t *testing.T) {
	// T2 ON THE SEARCH READS (plan Task 10 step 3b). The bounded
	// occupied-slot search asks about band 1 channels 1..8; a radio that
	// answers every one of them with a well-formed record naming a
	// DIFFERENT channel has told the driver nothing about the channel it
	// asked about, so no fingerprint may be taken from it.
	//
	// The misdirection is in force from the first byte, which is what
	// makes this reachable: every other deliberate fault in this package
	// is armed after Open so it cannot corrupt the diagnostics a test is
	// about, and under that arrangement the probe's own comparison was
	// deletable with the suite green.
	img := baseImage()
	elsewhere := civ.ChannelAddress{Group: 1, Channel: 42}
	img.misdirectAlways = &elsewhere
	port := newRecordingPort(t, img)

	sess, err := ic9700.New(ic9700.RealHardware).Open(context.Background(), port.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v — a misdirecting radio is not a reason to refuse the open", err)
	}
	defer sess.Close()

	d := sess.(*ic9700.Session).CIVDiagnostics()
	if d.Fingerprinted {
		t.Error("the probe fingerprinted this profile from a record belonging to another slot")
	}
	if d.AnswerMismatches != 8 {
		t.Errorf("AnswerMismatches = %d, want 8 — one per search read, counted rather than swallowed", d.AnswerMismatches)
	}
	if got := port.countReads(); got != 8 {
		t.Errorf("the search made %d reads, want the bounded 8 — a mismatch skips the slot, it does not stop the search", got)
	}
}

func TestTheIdentityTokenIsRecordedAndNeverMatched(t *testing.T) {
	// D5 entry 7, lift R6: the `19 00` reply VALUE is undocumented on all
	// six models in this tier. What the probe requires is that an
	// ADDRESS-MATCHED reply arrived at all, so a radio answering with an
	// unexpected token still opens — and the token is recorded so a
	// future capture has something to compare against.
	img := factoryAnswers()
	img.idData = []byte{0x5A}
	port := newRecordingPort(t, img)
	sess := openWith(t, port)

	if got := sess.CIVDiagnostics().IDToken; len(got) != 1 || got[0] != 0x5A {
		t.Errorf("IDToken = % X, want 5A recorded verbatim", got)
	}
	if !strings.Contains(sess.Identity().CATID, "A2") {
		t.Errorf("CATID = %q, want the ADDRESS half present; the token is not the identity", sess.Identity().CATID)
	}
}

func TestBroadcastFloodNeverReachesTheDrainCapAndInitSucceeds(t *testing.T) {
	// R9-SPLIT (a). The accumulator's address filter drops every to=00
	// frame BEFORE the engine sees it, so the idle timer is never
	// re-armed and Init returns nil — the landed behaviour. The traffic
	// is visible ONLY in the adapter's own counters, which is exactly why
	// the driver holds the framing value (DIAGNOSTICS CARRIER).
	port := newBroadcastFloodPort(t, factoryAnswers()) // to=00, never quiet
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sess, err := ic9700.New(ic9700.RealHardware).Open(ctx, port.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open under a broadcast flood: %v — broadcasts cannot reach the drain cap", err)
	}
	defer sess.Close()

	d := sess.(*ic9700.Session).CIVDiagnostics()
	if d.InitDrainCapExceeded {
		t.Error("a broadcast flood recorded a drain-cap event; the filter should have dropped every frame first")
	}
	if d.Accumulator.Unexpected == 0 {
		t.Error("a saturated broadcast line counted zero Unexpected frames")
	}
	// And the engine's own counter is NOT the source: it is systematically
	// zero here, which is the trap this assertion documents.
	if got := sess.(driver.DiagnosticsReporter).Diagnostics().UnexpectedFrames; got != 0 {
		t.Logf("engine UnexpectedFrames = %d (informational; the adapter's count is the real one)", got)
	}
}

func TestControllerAddressedFloodIsNonfatalWithADiagnostic(t *testing.T) {
	// R9-SPLIT (b). Frames addressed to the CONTROLLER do reach the
	// engine, so they re-arm the idle timer and Init returns
	// ErrDrainCapExceeded. The driver treats the INITIAL one as
	// nonfatal-with-diagnostic and continues to the 19 00 probe. Because
	// these frames are returned to the engine, they do NOT increment
	// AccumulatorStats.Unexpected — which is why this assertion and the
	// previous test's cannot live in one test.
	port := newAddressedFloodPort(t, factoryAnswers()) // to=E0, never quiet
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sess, err := ic9700.New(ic9700.RealHardware).Open(ctx, port.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v — an INITIAL ErrDrainCapExceeded is nonfatal (the bounded drain cannot fail the open)", err)
	}
	defer sess.Close()

	if !sess.(*ic9700.Session).CIVDiagnostics().InitDrainCapExceeded {
		t.Error("the drain-cap event was swallowed; nonfatal does not mean unrecorded")
	}
}

func TestDriverModelAndCapabilitiesAgree(t *testing.T) {
	d := ic9700.New(ic9700.RealHardware)
	if got, want := d.Model(), "IC-9700"; got != want {
		t.Errorf("Model() = %q, want %q", got, want)
	}
	if got := d.Capabilities().Model; got != d.Model() {
		t.Errorf("Capabilities().Model = %q, want %q — the Registry keys on their equality", got, d.Model())
	}
	if err := d.Capabilities().Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestConsentOption_StaticCapabilitiesNeverConsented(t *testing.T) {
	// Consent is a statement about a SESSION, never about the radio: the
	// STATIC capabilities internal/wiring publishes are untouched by the
	// option, whichever way it is set.
	d := ic9700.New(ic9700.RealHardware, ic9700.WithConsentedUnverifiedWrites())
	assertNoConsentedLabel(t, "static capabilities", d.Capabilities())
}

func TestProfilesNeverEmitConsented(t *testing.T) {
	// No profile CONSTRUCTOR ever contains the state: the baselines
	// describe evidence, and consent is not evidence. It is minted at
	// session-capability assembly by the shared transform and nowhere
	// else.
	assertNoConsentedLabel(t, "unverified", ic9700.CapabilitiesUnverified())
	assertNoConsentedLabel(t, "simulated", ic9700.CapabilitiesSimulated())
}

func TestConsentOption_UnrecognisedProfileStaysFailSafe(t *testing.T) {
	// The fail-safe arm. An unrecognised Profile value selects the
	// all-Unverified set AND is not consented, so a forged or corrupted
	// profile goes on writing nothing however the option is set.
	port := newRecordingPort(t, factoryAnswers())
	sess, err := ic9700.New(ic9700.Profile(99), ic9700.WithConsentedUnverifiedWrites()).
		Open(context.Background(), port.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sess.Close()
	assertNoConsentedLabel(t, "unrecognised-profile session", sess.Capabilities())
}

func TestConsentOption_RecognisedProfileOpensTheGate(t *testing.T) {
	// The other side of the same coin, so the fail-safe test above cannot
	// pass merely because the transform is never applied at all.
	port := newRecordingPort(t, factoryAnswers())
	sess, err := ic9700.New(ic9700.RealHardware, ic9700.WithConsentedUnverifiedWrites()).
		Open(context.Background(), port.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer sess.Close()
	if fs := sess.Capabilities().FieldSupport(spec.BankMemory, spec.FieldFrequency); !fs.CanWrite() {
		t.Errorf("MEM/frequency = %+v on a consented session; consent is what opens the gate", fs)
	}
	// Consent never reaches erase — the transform exempts it structurally.
	if fs := sess.Capabilities().FieldSupport(spec.BankMemory, spec.FieldErase); fs.CanWrite() {
		t.Error("consent opened the erase gate")
	}
}

// assertNoConsentedLabel requires that no bank in caps carries the
// consented write state.
func assertNoConsentedLabel(t *testing.T, what string, caps spec.Capabilities) {
	t.Helper()
	for _, bank := range caps.Banks {
		for f, fs := range bank.Fields {
			if fs.Write == spec.ConsentedUnverified {
				t.Errorf("%s: %s/%s carries ConsentedUnverified", what, bank.ID, f)
			}
		}
	}
}
