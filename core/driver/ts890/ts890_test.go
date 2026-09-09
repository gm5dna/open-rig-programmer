// SPDX-License-Identifier: GPL-3.0-or-later

package ts890

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// TestModel_IsTheRegistryKey pins core/driver.Driver's own contract:
// Model() and Capabilities().Model are one string, or internal/wiring's
// registry and the app's model description disagree about which radio this
// driver is.
func TestModel_IsTheRegistryKey(t *testing.T) {
	d := New(RealHardware)
	if d.Model() != "TS-890S" || d.Capabilities().Model != d.Model() {
		t.Errorf("Model = %q, Capabilities().Model = %q, want both %q", d.Model(), d.Capabilities().Model, "TS-890S")
	}
}

// TestStopBits_IsOne is plan P10's FIRST leg, and it is a SESSION
// PRECONDITION rather than a nicety: transport.DefaultStopBits is 2 and this
// book prints ONE (890:15-18), so a driver that omitted the interface would
// open every session at the wrong framing and fail like a dead port.
//
// IT COMPARES THE VALUE AND NEVER TYPE-SWITCHES. A test of the form "if r, ok
// := d.(driver.SerialFramingReporter); ok { … }" passes when the interface is
// ABSENT, which is precisely the failure this pin exists to catch.
//
// The SECOND leg is unreachable from here: internal/wiring's stopBitsFor is
// unexported, so what a registered row's session actually opens with is
// pinned at registration (T17).
func TestStopBits_IsOne(t *testing.T) {
	r, ok := New(RealHardware).(driver.SerialFramingReporter)
	if !ok {
		t.Fatal("the driver does not implement driver.SerialFramingReporter; transport.DefaultStopBits is 2 and this radio documents 1 (890:17)")
	}
	if got := r.StopBits(); got != 1 {
		t.Errorf("StopBits() = %d, want 1 (890:15-18, matrix §3.1)", got)
	}
}

// TestNew_TheZeroProfileIsRealHardwareAndAnUnrecognisedOneFailsTheSameWay
// pins matrix §2.1's fail-safe direction: nothing writable, whether the
// caller said RealHardware, said nothing, or said something this package does
// not know.
func TestNew_TheZeroProfileIsRealHardwareAndAnUnrecognisedOneFailsTheSameWay(t *testing.T) {
	var zero Profile
	if zero != RealHardware {
		t.Fatalf("the zero Profile is %v, want RealHardware — a forgotten profile must never select the simulator's Supported writes", zero)
	}
	want := CapabilitiesUnverified()
	for name, profile := range map[string]Profile{
		"zero":         zero,
		"realhardware": RealHardware,
		"unrecognised": Profile(99),
	} {
		caps := New(profile).Capabilities()
		for _, f := range spec.AllFields() {
			if caps.Banks[0].Fields[f].CanWrite() {
				t.Errorf("%s: %s is writable", name, f)
			}
		}
		if len(caps.Modes) != len(want.Modes) {
			t.Errorf("%s: Modes has %d entries, want %d", name, len(caps.Modes), len(want.Modes))
		}
	}
}

// TestOpen_ThreeFramesGoOutAndTheProbeIsTwoOfThem pins plan P9 as a
// TRANSCRIPT: the engine's own AI0; preamble (matrix §3.6, session init and
// not part of the probe), then "ID;", then "FV;". Nothing else — no
// discovery frame of any kind (§3.4), and no MN in either direction.
func TestOpen_ThreeFramesGoOutAndTheProbeIsTwoOfThem(t *testing.T) {
	_, port := openTestSession(t, Simulated, radioImage{})
	want := []string{"AI0;", "ID;", "FV;"}
	if got := port.Transcript(); !slices.Equal(got, want) {
		t.Errorf("Open transcript = %v, want %v (P9, §3.5, §3.6)", got, want)
	}
}

// TestOpen_WrongRadioIsRefusedByNameAndSendsNothingMore pins P9's other half.
// A wrong radio receives EXACTLY the preamble and "ID;" — asserted as the
// whole transcript rather than as "no MA0 frame" — and the refusal names the
// token. All five printed Kenwood identities are now known to the programme,
// so four of them are refused by NAME rather than by an unknown-model
// sentence; anything else carries the ID alone.
func TestOpen_WrongRadioIsRefusedByNameAndSendsNothingMore(t *testing.T) {
	for _, tc := range []struct {
		id       string
		wantName string
	}{
		{"020", "TS-480"},
		{"021", "TS-590S"},
		{"022", "TS-990S"},
		{"023", "TS-590SG"},
		{"099", ""},
	} {
		t.Run(tc.id, func(t *testing.T) {
			port := newRespondingPort(t, radioImage{catID: tc.id})
			d := New(Simulated, testTiming())
			_, err := d.Open(context.Background(), port.Port(), driver.Identity{})
			var wrong *driver.WrongRadioError
			if !errors.As(err, &wrong) {
				t.Fatalf("Open with ID %s: err = %v, want a *driver.WrongRadioError", tc.id, err)
			}
			if wrong.Want != "024" || wrong.Got != tc.id || wrong.WantModel != "TS-890S" {
				t.Errorf("WrongRadioError = %+v, want Want 024 / Got %s / WantModel TS-890S", wrong, tc.id)
			}
			if wrong.GotModel != tc.wantName {
				t.Errorf("GotModel = %q, want %q", wrong.GotModel, tc.wantName)
			}
			if got := port.Transcript(); !slices.Equal(got, []string{"AI0;", "ID;"}) {
				t.Errorf("a wrong radio received %v, want exactly the preamble and ID; (P9)", got)
			}
		})
	}
}

// TestOpen_AProbeFrameThatDoesNotAnswerRefusesTheSession pins the two wire
// events on BOTH probe frames, each with the typed error the book's own
// sentences justify: a "?;" is a definitive rejection with two
// indistinguishable causes (890:106-112), and silence is a timeout that is
// explicitly NOT an inference of absence.
//
// FV IS PROBED AT ALL BECAUSE THE BOOK IS COMPLETE HERE — a Read chart, an
// Answer chart and a worked example (890:2653-2659) — and the standing rule
// is to refuse where the document is complete and we are outside it. Nothing
// on this row branches on the VERSION (matrix §3.5), so unlike pair 1 there
// is no grammar to fail softly on.
func TestOpen_AProbeFrameThatDoesNotAnswerRefusesTheSession(t *testing.T) {
	for name, tc := range map[string]struct {
		img      radioImage
		rejected bool
	}{
		"ID rejects": {radioImage{idReject: true}, true},
		"ID silent":  {radioImage{idSilent: true}, false},
		"FV rejects": {radioImage{fvReject: true}, true},
		"FV silent":  {radioImage{fvSilent: true}, false},
	} {
		t.Run(name, func(t *testing.T) {
			port := newRespondingPort(t, tc.img)
			_, err := New(Simulated, testTiming()).Open(context.Background(), port.Port(), driver.Identity{})
			if err == nil {
				t.Fatal("Open succeeded against a probe frame that did not answer")
			}
			var rej *kw.RejectionError
			var to *kw.TimeoutError
			if tc.rejected && !errors.As(err, &rej) {
				t.Errorf("err = %v, want a *kw.RejectionError naming the book's two indistinguishable causes", err)
			}
			if !tc.rejected && !errors.As(err, &to) {
				t.Errorf("err = %v, want a *kw.TimeoutError, which says it is not an inference of absence", err)
			}
		})
	}
}

// TestOpen_TheFVAnswerIsCarriedVerbatim pins that the four characters reach
// the session exactly as the radio spelled them, whatever they spell.
// NOTHING ON THIS ROW BRANCHES ON THE VERSION (§3.5), so the accessor's whole
// purpose is that an owner can see what their radio said; a version this
// programme could not read must therefore not refuse the session.
func TestOpen_TheFVAnswerIsCarriedVerbatim(t *testing.T) {
	for _, answer := range []string{"FV1.00;", "FV2.31;", "FVABCD;"} {
		sess, _ := openTestSession(t, Simulated, radioImage{fvAnswer: answer})
		if got, want := sess.FirmwareAnswer(), answer[2:6]; got != want {
			t.Errorf("FirmwareAnswer() = %q, want %q verbatim", got, want)
		}
	}
}

// TestSessionCapabilities_ConsentTransformAndDefensiveCopies pins both halves
// of the consent seam. CONSENT WIDENS WHAT MAY BE ATTEMPTED, NEVER HOW
// CAREFULLY: it re-labels the write side of a SESSION and never touches the
// static profile, and spec.FieldErase is exempt inside the transform itself,
// so no consent can mint an erase on a radio whose MA5 this programme
// declines to build.
//
// The defensive copy is asserted THROUGH AN OPENED SESSION rather than
// against a freshly constructed set, which would be trivially true whatever
// Clone did, since baseCapabilities allocates on every call.
func TestSessionCapabilities_ConsentTransformAndDefensiveCopies(t *testing.T) {
	unconsented, _ := openTestSession(t, RealHardware, radioImage{})
	if unconsented.Capabilities().Banks[0].Fields[spec.FieldFrequency].CanWrite() {
		t.Error("an UNCONSENTED RealHardware session grades frequency writable while writeTrialsComplete is false")
	}

	consented, _ := openTestSession(t, RealHardware, radioImage{}, WithConsentedUnverifiedWrites())
	fields := consented.Capabilities().Banks[0].Fields
	if !fields[spec.FieldFrequency].CanWrite() {
		t.Error("a CONSENTED RealHardware session does not grade frequency writable; consent re-labels the write side")
	}
	if fields[spec.FieldErase].CanWrite() {
		t.Error("consent minted an erase; spec.FieldErase is exempt inside the transform, and this row refuses MA5 besides (M-E4)")
	}
	if New(RealHardware, WithConsentedUnverifiedWrites()).Capabilities().Banks[0].Fields[spec.FieldFrequency].CanWrite() {
		t.Error("the STATIC capability set was transformed by consent; consent is a statement about a session")
	}

	handed := consented.Capabilities()
	handed.Banks[0].Slots[0] = "MUTATED"
	handed.Banks[0].Fields[spec.FieldFrequency] = spec.FieldSupport{}
	handed.Modes[0] = "MUTATED"
	if again := consented.Capabilities(); again.Banks[0].Slots[0] != "000" || again.Modes[0] != "LSB" ||
		!again.Banks[0].Fields[spec.FieldFrequency].CanWrite() {
		t.Error("a caller mutating what Capabilities handed it altered what the session enforces")
	}
}

// TestDiagnostics_ReportsTheEnginesCounter pins the optional
// driver.DiagnosticsReporter surface: it is the driver-layer window onto the
// engine's own accessors, which are otherwise unreachable.
func TestDiagnostics_ReportsTheEnginesCounter(t *testing.T) {
	sess, _ := openTestSession(t, Simulated, radioImage{})
	r, ok := any(sess).(driver.DiagnosticsReporter)
	if !ok {
		t.Fatal("the session does not implement driver.DiagnosticsReporter")
	}
	if got := r.Diagnostics().UnexpectedFrames; got != 0 {
		t.Errorf("UnexpectedFrames = %d on a session that met only its own answers, want 0", got)
	}
}
