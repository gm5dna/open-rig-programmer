// SPDX-License-Identifier: GPL-3.0-or-later

package ftx1

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

const testCtxTimeout = 5 * time.Second

var (
	_ driver.Driver              = (*ftx1Driver)(nil)
	_ driver.Session             = (*Session)(nil)
	_ driver.DiagnosticsReporter = (*Session)(nil)
)

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testCtxTimeout)
	t.Cleanup(cancel)
	return ctx
}

var testIdentity = driver.Identity{Port: "scripted-pipe", USBSerial: "SIM0840"}

func openSession(t *testing.T, profile Profile, img slotImage, opts ...Option) (*respondingPort, *Session) {
	t.Helper()
	p := newRespondingPort(t, img)

	sess, err := New(profile, opts...).Open(testCtx(t), p.Port(), testIdentity)
	if err != nil {
		t.Fatalf("Open: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	s, ok := sess.(*Session)
	if !ok {
		t.Fatalf("Open returned a %T, want *ftx1.Session", sess)
	}
	return p, s
}

func TestDriver_ModelAndCapabilities(t *testing.T) {
	d := New(RealHardware)
	if got := d.Model(); got != modelName {
		t.Errorf("Model() = %q, want %q", got, modelName)
	}
	caps := d.Capabilities()
	if caps.Model != modelName {
		t.Errorf("Capabilities().Model = %q, want %q", caps.Model, modelName)
	}
	if err := caps.Validate(); err != nil {
		t.Errorf("Capabilities().Validate(): %v", err)
	}
	for _, bank := range caps.Banks {
		if fs := caps.FieldSupport(bank.ID, spec.FieldFrequency); fs.CanWrite() {
			t.Errorf("bank %s: frequency is write-Supported under RealHardware, want nothing writable", bank.ID)
		}
	}
}

func TestDriver_SimulatedCapabilities(t *testing.T) {
	caps := New(Simulated).Capabilities()
	if err := caps.Validate(); err != nil {
		t.Errorf("Capabilities().Validate(): %v", err)
	}
	if fs := caps.FieldSupport(spec.BankMemory, spec.FieldFrequency); !fs.CanWrite() {
		t.Error("MEM frequency not write-Supported under Simulated")
	}
	// The 5 MHz and EMGCH banks stay write-Unsupported even under
	// Simulated: MW cannot target either regardless of profile.
	for _, id := range []spec.BankID{spec.Bank60m, spec.BankEMG} {
		if fs := caps.FieldSupport(id, spec.FieldFrequency); fs.CanWrite() {
			t.Errorf("bank %s: frequency is write-Supported under Simulated, want it forced Unsupported", id)
		}
	}
}

func TestDriver_UnrecognisedProfileFailsSafe(t *testing.T) {
	d := New(Profile(99))
	caps := d.Capabilities()
	for _, bank := range caps.Banks {
		if fs := caps.FieldSupport(bank.ID, spec.FieldFrequency); fs.CanWrite() {
			t.Errorf("bank %s: frequency is write-Supported under an unrecognised Profile, want nothing writable", bank.ID)
		}
	}
}

func TestOpen_Accepts(t *testing.T) {
	p, s := openSession(t, Simulated, slotImage{})
	if got := s.Identity().CATID; got != "0840" {
		t.Errorf("Identity().CATID = %q, want \"0840\"", got)
	}
	if transcript := p.Transcript(); len(transcript) != 2 {
		t.Errorf("transcript = %v, want exactly [AI0;, ID;] — no discovery sweep (caps.go's banks are all static)", transcript)
	}
}

func TestOpen_ForeignCATIDRefuses(t *testing.T) {
	p := newRespondingPort(t, slotImage{catID: "9999"})
	sess, err := New(Simulated).Open(testCtx(t), p.Port(), testIdentity)
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open succeeded against a foreign CAT ID, want *driver.WrongRadioError")
	}
	var wrong *driver.WrongRadioError
	if !errors.As(err, &wrong) {
		t.Fatalf("Open error = %v, want *driver.WrongRadioError", err)
	}
	if wrong.Got != "9999" {
		t.Errorf("Got = %q, want \"9999\"", wrong.Got)
	}
}

func TestOpen_UnconfiguredDialectRefusesToOpen(t *testing.T) {
	p := newRespondingPort(t, slotImage{})
	d := &ftx1Driver{Base: driver.Base{Profile: Simulated}} // zero dialect, deliberately not via New

	sess, err := d.Open(testCtx(t), p.Port(), testIdentity)
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open with a zero dialect succeeded, want a refusal before any engine exists")
	}
	if got := p.Transcript(); len(got) != 0 {
		t.Errorf("the port received %v, want nothing — the refusal must precede the engine", got)
	}
}

func TestSession_Diagnostics(t *testing.T) {
	_, s := openSession(t, Simulated, slotImage{})
	if got := s.Diagnostics().UnexpectedFrames; got != 0 {
		t.Errorf("UnexpectedFrames = %d, want 0 on a clean session", got)
	}
}

func TestSession_Close_Idempotent(t *testing.T) {
	_, s := openSession(t, Simulated, slotImage{})
	if err := s.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestWithTransportLogger_NilIsIgnored(t *testing.T) {
	d, ok := New(Simulated, WithTransportLogger(nil)).(*ftx1Driver)
	if !ok {
		t.Fatal("New did not return a *ftx1Driver")
	}
	if d.transportLogger != nil {
		t.Error("WithTransportLogger(nil) installed a logger, want the engine's default kept")
	}
}

// capsContains reports whether ANY bank field of caps carries s, on
// EITHER side.
func capsContains(caps spec.Capabilities, s spec.Support) bool {
	for _, b := range caps.Banks {
		for _, fs := range b.Fields {
			if fs.Read == s || fs.Write == s {
				return true
			}
		}
	}
	return false
}

// TestConsentOption_SessionCapsTransformed is the option's whole point: a
// RealHardware session built WITH WithConsentedUnverifiedWrites carries
// exactly spec.ConsentUnverifiedWrites' product over the set the same
// session would otherwise have had.
func TestConsentOption_SessionCapsTransformed(t *testing.T) {
	_, plain := openSession(t, RealHardware, slotImage{})
	_, consented := openSession(t, RealHardware, slotImage{}, WithConsentedUnverifiedWrites())

	if capsContains(plain.Capabilities(), spec.ConsentedUnverified) {
		t.Fatal("an UNCONSENTED RealHardware session already carries ConsentedUnverified — the option is not what put it there")
	}

	want := spec.ConsentUnverifiedWrites(plain.Capabilities())
	got := consented.Capabilities()
	if !reflect.DeepEqual(got, want) {
		t.Error("consented session capabilities differ from spec.ConsentUnverifiedWrites' product")
	}

	if fs := got.FieldSupport(spec.BankMemory, spec.FieldFrequency); fs.Write != spec.ConsentedUnverified || !fs.CanWrite() {
		t.Errorf("MEM frequency Write = %v (CanWrite %v), want ConsentedUnverified and writable", fs.Write, fs.CanWrite())
	}
	if fs := got.FieldSupport(spec.Bank60m, spec.FieldFrequency); fs.CanWrite() {
		t.Error("5 MHz bank frequency became writable under consent — that bank is write-Unsupported structurally, exempt from the transform's effect since Unsupported never becomes ConsentedUnverified")
	}
	if fs := got.FieldSupport(spec.BankMemory, spec.FieldErase); fs.CanWrite() {
		t.Error("MEM erase became writable under consent — FieldErase is exempt from the transform structurally")
	}
	for _, b := range got.Banks {
		for f, fs := range b.Fields {
			if fs.Read == spec.ConsentedUnverified {
				t.Errorf("bank %s field %s: Read = ConsentedUnverified — consent is a write-side state", b.ID, f)
			}
		}
	}
}

func TestConsentOption_StaticCapabilitiesNeverConsented(t *testing.T) {
	for _, tt := range []struct {
		name string
		p    Profile
	}{
		{"Simulated", Simulated},
		{"RealHardware", RealHardware},
		{"unrecognised", Profile(99)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := New(tt.p, WithConsentedUnverifiedWrites()).Capabilities()
			if capsContains(got, spec.ConsentedUnverified) {
				t.Error("the consent option reached the STATIC capability set")
			}
			if !reflect.DeepEqual(got, New(tt.p).Capabilities()) {
				t.Error("a consented driver's static Capabilities() differ from an unconsented one's")
			}
		})
	}
}

func TestConsentOption_UnrecognisedProfileStaysFailSafe(t *testing.T) {
	_, sess := openSession(t, Profile(99), slotImage{}, WithConsentedUnverifiedWrites())
	caps := sess.Capabilities()

	if capsContains(caps, spec.ConsentedUnverified) {
		t.Error("an unrecognised Profile + the consent option produced ConsentedUnverified")
	}
	for _, b := range caps.Banks {
		for f := range b.Fields {
			if caps.FieldSupport(b.ID, f).CanWrite() {
				t.Errorf("bank %s field %s: CanWrite() = true on an unrecognised Profile with consent", b.ID, f)
			}
		}
	}
}
