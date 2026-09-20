// SPDX-License-Identifier: GPL-3.0-or-later

package ts990

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestModel_IsTheRegistryKey pins core/driver.Driver's own contract: the
// display name and Capabilities().Model must be the same string, or the
// driver matches no registry row.
func TestModel_IsTheRegistryKey(t *testing.T) {
	d := New(RealHardware)
	if d.Model() != modelName {
		t.Errorf("Model() = %q, want %q", d.Model(), modelName)
	}
	if got := d.Capabilities().Model; got != d.Model() {
		t.Errorf("Capabilities().Model = %q, want %q (core/driver.Driver's contract)", got, d.Model())
	}
}

// TestCapabilities_TheZeroProfileIsRealHardwareAndAnyOtherFailsSafe is matrix
// §2.1's failure direction: a forgotten Profile must fail towards the
// all-Unverified set, never towards the simulator's Supported writes.
func TestCapabilities_TheZeroProfileIsRealHardwareAndAnyOtherFailsSafe(t *testing.T) {
	var zero driver.Profile
	if zero != RealHardware {
		t.Fatalf("the zero Profile is %v, want RealHardware (matrix §2.1)", zero)
	}
	unverified := CapabilitiesUnverified()
	for name, d := range map[string]driver.Driver{
		"zero":         New(zero),
		"realhardware": New(RealHardware),
		"unrecognised": New(driver.Profile(99)),
	} {
		if got := d.Capabilities(); !reflect.DeepEqual(got, unverified) {
			t.Errorf("%s: Capabilities() is not the all-Unverified fail-safe set", name)
		}
	}
	if got := New(Simulated).Capabilities(); !reflect.DeepEqual(got, CapabilitiesSimulated()) {
		t.Error("Simulated: Capabilities() is not the simulated set")
	}
}

// TestStopBits_IsOneAndTheInterfaceIsPresent is plan P10's first leg, and it
// is a SESSION PRECONDITION rather than a nicety: transport.DefaultStopBits
// is 2 — the Yaesu family's framing — and this book specifies ONE, so a
// driver that omitted the interface would open every session at the wrong
// framing and fail like a dead port.
//
// The assertion is on the CONCRETE driver and never a type switch that would
// pass when the interface is absent.
func TestStopBits_IsOneAndTheInterfaceIsPresent(t *testing.T) {
	d := New(RealHardware).(*ts990Driver)
	if got := d.StopBits(); got != 1 {
		t.Errorf("StopBits() = %d, want 1 (matrix §3.1, P10)", got)
	}
	if transport.DefaultStopBits == 1 {
		t.Error("transport.DefaultStopBits is 1; this pin exists because the fleet default is 2 and this family needs the interface to say otherwise")
	}
	var _ driver.SerialFramingReporter = d
}

// TestOpen_SendsThreeFramesAndTheProbeIsTwoOfThem is plan P9 as a transcript.
// AI0; is transport.Engine.Init's own frame and the matrix files it under
// session init rather than as part of the probe.
func TestOpen_SendsThreeFramesAndTheProbeIsTwoOfThem(t *testing.T) {
	sess, p := openTestSession(t, radioImage{})
	if got, want := p.Transcript(), []string{"AI0;", "ID;", "FV;"}; !reflect.DeepEqual(got, want) {
		t.Errorf("transcript = %v, want %v (P9, matrix §3.5, §3.6)", got, want)
	}
	if got := sess.Identity().CATID; got != catID {
		t.Errorf("Identity().CATID = %q, want %q", got, catID)
	}
}

// TestOpen_WrongRadioSendsNothingMoreAndNamesTheOtherFourByName is P9's
// refusal half. With this milestone all five printed Kenwood identities are
// known to the programme, so the message names 020, 021, 023 and 024 BY NAME
// rather than falling through to an unknown-model sentence.
func TestOpen_WrongRadioSendsNothingMoreAndNamesTheOtherFourByName(t *testing.T) {
	for id, wantModel := range map[string]string{
		"020": "TS-480",
		"021": "TS-590S",
		"023": "TS-590SG",
		"024": "TS-890S",
	} {
		p := newRespondingPort(t, radioImage{catID: id})
		_, err := New(Simulated, testTiming()).Open(context.Background(), p.Port(), driver.Identity{})
		if !errors.Is(err, driver.ErrWrongRadio) {
			t.Fatalf("ID %s: Open error = %v, want driver.ErrWrongRadio", id, err)
		}
		var wrong *driver.WrongRadioError
		if !errors.As(err, &wrong) {
			t.Fatalf("ID %s: errors.As(*driver.WrongRadioError) = false for %v", id, err)
		}
		if wrong.Got != id || wrong.Want != catID {
			t.Errorf("ID %s: Got/Want = %q/%q, want %q/%q", id, wrong.Got, wrong.Want, id, catID)
		}
		if wrong.GotModel != wantModel || wrong.WantModel != modelName {
			t.Errorf("ID %s: GotModel/WantModel = %q/%q, want %q/%q", id, wrong.GotModel, wrong.WantModel, wantModel, modelName)
		}
		if !strings.Contains(err.Error(), wantModel) {
			t.Errorf("ID %s: %q does not name %s", id, err, wantModel)
		}
		// NOTHING MORE IS SENT: a wrong radio receives exactly the
		// preamble and "ID;".
		if got, want := p.Transcript(), []string{"AI0;", "ID;"}; !reflect.DeepEqual(got, want) {
			t.Errorf("ID %s: transcript = %v, want %v — no FV; to a radio that is not this one", id, got, want)
		}
	}
}

// TestOpen_AnUnprintedIdentityIsRefusedWithoutInventingAName is the other
// half of siblingModelName's contract: a token no document this milestone
// reads prints leaves GotModel empty, and WrongRadioError renders its
// ID-only sentence rather than putting a name in a manufacturer's mouth.
func TestOpen_AnUnprintedIdentityIsRefusedWithoutInventingAName(t *testing.T) {
	p := newRespondingPort(t, radioImage{catID: "019"})
	_, err := New(Simulated, testTiming()).Open(context.Background(), p.Port(), driver.Identity{})
	var wrong *driver.WrongRadioError
	if !errors.As(err, &wrong) {
		t.Fatalf("errors.As(*driver.WrongRadioError) = false for %v", err)
	}
	if wrong.GotModel != "" {
		t.Errorf("GotModel = %q for an identity no book prints, want \"\"", wrong.GotModel)
	}
	if want := "driver: connected radio identified as CAT ID \"019\", want \"022\" — wrong radio model on this port"; err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

// TestOpen_AProbeFrameThatDoesNotAnswerRefusesTheSession pins the two wire
// events on BOTH probe frames, in this family's own typed vocabulary: a "?;"
// is a *kw.RejectionError, which names the two indistinguishable causes the
// book prints for it, and silence is a *kw.TimeoutError, which says in as
// many words that it is not an inference of absence.
//
// THE FV LEG IS A DECISION AND NOT AN OVERSIGHT. The EXISTENCE of FV is
// printed for this row, with a Read chart, an Answer chart and a worked
// example (990:2527-2536); a radio that answered "ID;" with this row's
// identity and then rejects or ignores "FV;" is outside a document that is
// complete here, and the standing rule is to REFUSE where the document is
// complete and we are outside it.
func TestOpen_AProbeFrameThatDoesNotAnswerRefusesTheSession(t *testing.T) {
	for name, tc := range map[string]struct {
		img     radioImage
		command string
		wantIs  error
	}{
		"ID rejected": {radioImage{idReject: true}, "ID", transport.ErrRejected},
		"ID silent":   {radioImage{idSilent: true}, "ID", transport.ErrTimeout},
		"FV rejected": {radioImage{fvReject: true}, "FV", transport.ErrRejected},
		"FV silent":   {radioImage{fvSilent: true}, "FV", transport.ErrTimeout},
	} {
		p := newRespondingPort(t, tc.img)
		_, err := New(Simulated, testTiming()).Open(context.Background(), p.Port(), driver.Identity{})
		if err == nil {
			t.Fatalf("%s: Open succeeded", name)
		}
		if !errors.Is(err, tc.wantIs) {
			t.Errorf("%s: err = %v, want one wrapping %v", name, err, tc.wantIs)
		}
		if !strings.Contains(err.Error(), tc.command) {
			t.Errorf("%s: %q does not name the frame that failed", name, err)
		}
		switch tc.wantIs {
		case transport.ErrRejected:
			var rej *kw.RejectionError
			if !errors.As(err, &rej) {
				t.Errorf("%s: errors.As(*kw.RejectionError) = false for %v", name, err)
			}
		case transport.ErrTimeout:
			var to *kw.TimeoutError
			if !errors.As(err, &to) {
				t.Errorf("%s: errors.As(*kw.TimeoutError) = false for %v", name, err)
			}
		}
	}
}

// TestOpen_AMalformedFVAnswerIsAMalformedFrame pins the ENVELOPE half of the
// FV probe: the 7-byte structural parse — prefix, width, printable ASCII — is
// the envelope's and its failure is a malformed frame like any other.
//
// WHAT THIS ROW DOES NOT DO IS BRANCH ON THE VERSION (matrix §3.5): pair 1's
// TS-590S gates byte 28's write policy on the firmware, and this radio has no
// field whose meaning depends on it. So the driver reads FV to prove the
// session is talking to a radio that answers its own book, and keeps nothing.
func TestOpen_AMalformedFVAnswerIsAMalformedFrame(t *testing.T) {
	for name, answer := range map[string]string{
		"too short":    "FV1.0;",
		"too long":     "FV1.000;",
		"wrong prefix": "XV1.00;",
	} {
		p := newRespondingPort(t, radioImage{fvAnswer: answer})
		_, err := New(Simulated, testTiming()).Open(context.Background(), p.Port(), driver.Identity{})
		if err == nil {
			t.Fatalf("%s: Open succeeded on FV answer %q", name, answer)
		}
	}
}

// TestOpen_TheFVAnswerIsCarriedVerbatim pins that the four characters reach
// the session exactly as the radio spelled them, whatever they spell — the
// sibling row's own pin, on this row for the same reason.
//
// NOTHING HERE BRANCHES ON THE VERSION (matrix §3.5), so the accessor's whole
// purpose is that an owner can see what their radio said: without it the datum
// is unreachable outside this package and rigprog probe prints the firmware
// line for the TS-890S and silently omits it for the TS-990S (review
// s2-close-review-opus-1.md LOW-1). A version this programme could not read
// as a version must therefore still reach the user unedited.
func TestOpen_TheFVAnswerIsCarriedVerbatim(t *testing.T) {
	for _, answer := range []string{"FV1.00;", "FV2.31;", "FVABCD;"} {
		sess, _ := openTestSession(t, radioImage{fvAnswer: answer})
		if got, want := sess.FirmwareAnswer(), answer[2:6]; got != want {
			t.Errorf("FirmwareAnswer() = %q, want %q verbatim", got, want)
		}
	}
}

// TestSession_SatisfiesFirmwareAnswerReporter is the seam the accessor exists
// for: cmd/rigprog/probe.go type-asserts this optional capability and prints
// the answer quoted, so a session that did not satisfy it would drop the line
// with no compile error anywhere.
func TestSession_SatisfiesFirmwareAnswerReporter(t *testing.T) {
	var _ driver.FirmwareAnswerReporter = (*Session)(nil)
}

// TestOpen_ClosesThePortOnEveryFailurePath is Open's ownership obligation: it
// takes the port on BOTH outcomes, and a refused Open must not leak it.
func TestOpen_ClosesThePortOnEveryFailurePath(t *testing.T) {
	p := newRespondingPort(t, radioImage{catID: "024"})
	if _, err := New(Simulated, testTiming()).Open(context.Background(), p.Port(), driver.Identity{}); err == nil {
		t.Fatal("Open succeeded against a TS-890S")
	}
	if _, err := p.Port().Read(make([]byte, 1)); err == nil {
		t.Error("the port is still readable after a refused Open; Open owns it on both outcomes")
	}
}

// TestSessionCapabilities_AreACopyPerCall keeps the write gate honest: a
// caller mutating what it was handed must never alter what the session
// enforces.
func TestSessionCapabilities_AreACopyPerCall(t *testing.T) {
	sess, _ := openTestSession(t, radioImage{})
	got := sess.Capabilities()
	got.Banks[0].Fields[spec.FieldErase] = spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	if sess.Capabilities().FieldSupport(spec.BankMemory, spec.FieldErase).CanWrite() {
		t.Error("mutating the returned capability set changed what the session holds")
	}
}

// TestSessionCapabilities_ConsentIsASessionStatementAndNeverTheRadios pins
// the consent transform's two halves at once: a consented RealHardware
// session's write labels open, and the driver's STATIC set is untouched.
func TestSessionCapabilities_ConsentIsASessionStatementAndNeverTheRadios(t *testing.T) {
	d := New(RealHardware, testTiming(), WithConsentedUnverifiedWrites())
	if !reflect.DeepEqual(d.Capabilities(), CapabilitiesUnverified()) {
		t.Error("consent changed the driver's STATIC capability set; it is a statement about a SESSION")
	}
	sess, _ := openSessionAt(t, RealHardware, radioImage{}, WithConsentedUnverifiedWrites())
	if !sess.Capabilities().FieldSupport(spec.BankMemory, spec.FieldFrequency).CanWrite() {
		t.Error("a consented session cannot write frequency; consent re-labels write-side Unverified as ConsentedUnverified")
	}
	plain, _ := openSessionAt(t, RealHardware, radioImage{})
	if plain.Capabilities().FieldSupport(spec.BankMemory, spec.FieldFrequency).CanWrite() {
		t.Error("an UNCONSENTED RealHardware session can write; the capability gate must answer first")
	}
}

// TestWithTransportLogger_ReachesTheEngine pins the option's whole point: the
// engine's diagnostics — unexpected frames, quarantine drains, contamination
// (transport safety obligation 3, "surfaced, never silently discarded") —
// otherwise fall into the engine's own drop-everything default with no way
// for a caller of this driver to receive them.
func TestWithTransportLogger_ReachesTheEngine(t *testing.T) {
	var log testLogger
	// An UNSOLICITED frame is the cheapest event the engine reports: it
	// arrives with no command outstanding, so the engine records it and
	// tells the logger.
	sess, p := openTestSession(t, radioImage{}, WithTransportLogger(&log))
	if _, err := p.remote.Write([]byte("ID022;")); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Provoke one exchange so the reader loop has certainly processed the
	// unsolicited frame before the assertion.
	_, _ = sess.ReadChannel(context.Background(), "000")
	if log.count() == 0 {
		t.Error("the transport logger received nothing; WithTransportLogger must reach the engine")
	}
}

// TestWithTransportLogger_NilIsIgnored: a nil logger leaves the engine's own
// drop-everything default in place rather than installing a nil that would
// panic on the first diagnostic.
func TestWithTransportLogger_NilIsIgnored(t *testing.T) {
	d, ok := New(Simulated, WithTransportLogger(nil)).(*ts990Driver)
	if !ok {
		t.Fatal("New did not return a *ts990Driver")
	}
	if d.transportLogger != nil {
		t.Error("WithTransportLogger(nil) installed a logger, want the engine's default kept")
	}
}

// testLogger is a counting transport.Logger.
type testLogger struct {
	mu sync.Mutex
	n  int
}

func (l *testLogger) Printf(string, ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.n++
}

func (l *testLogger) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.n
}

// TestDiagnostics_ReportsTheEnginesCounters pins the optional
// driver.DiagnosticsReporter surface, which is the only route by which the
// engine's own accessors are reachable from outside this package.
func TestDiagnostics_ReportsTheEnginesCounters(t *testing.T) {
	sess, p := openTestSession(t, radioImage{})
	var _ driver.DiagnosticsReporter = sess
	if got := sess.Diagnostics().UnexpectedFrames; got != 0 {
		t.Errorf("UnexpectedFrames = %d on a fresh session, want 0", got)
	}
	if _, err := p.remote.Write([]byte("ID022;")); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _ = sess.ReadChannel(context.Background(), "000")
	if got := sess.Diagnostics().UnexpectedFrames; got == 0 {
		t.Error("UnexpectedFrames is 0 after an unsolicited frame")
	}
}
