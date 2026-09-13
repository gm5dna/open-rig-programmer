// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx5000

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

const testCtxTimeout = 10 * time.Second

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testCtxTimeout)
	t.Cleanup(cancel)
	return ctx
}

var testIdentity = driver.Identity{Port: "scripted-pipe", USBSerial: "SIM0362"}

// openSession starts a respondingPort serving img, opens a session over it
// with profile, and registers cleanup for both.
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
		t.Fatalf("Open returned a %T, want *ftdx5000.Session", sess)
	}
	return p, s
}

// TestDriver_ModelAndIdentity: the driver names itself consistently, and
// driver.Driver's contract that Model() equals Capabilities().Model holds
// on every profile.
func TestDriver_ModelAndIdentity(t *testing.T) {
	for _, tt := range []struct {
		name string
		p    Profile
	}{
		{"RealHardware", RealHardware},
		{"Simulated", Simulated},
		{"unrecognised", Profile(99)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := New(tt.p)
			if d.Model() != modelName {
				t.Errorf("Model() = %q, want %q", d.Model(), modelName)
			}
			if got := d.Capabilities().Model; got != d.Model() {
				t.Errorf("Capabilities().Model = %q, Model() = %q", got, d.Model())
			}
		})
	}
}

// TestOpen_ProbesIdentityAndNoDiscovery: this radio has no 5xx/EMG bank at
// all, so a successful Open sends exactly AI0; then ID; and nothing else —
// unlike every other registered Yaesu driver, which walks a discovery
// sweep after the ID probe.
func TestOpen_ProbesIdentityAndNoDiscovery(t *testing.T) {
	p, sess := openSession(t, Simulated, slotImage{})

	id := sess.Identity()
	if id.CATID != "0362" {
		t.Errorf("Identity().CATID = %q, want \"0362\"", id.CATID)
	}
	if id.Port != testIdentity.Port || id.USBSerial != testIdentity.USBSerial {
		t.Errorf("Identity() = %+v, want the caller's Port/USBSerial preserved", id)
	}

	if got, want := p.Transcript(), []string{"AI0;", "ID;"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Open's transcript = %v, want %v — this radio has no 5xx/EMG bank, so nothing follows the ID probe", got, want)
	}

	if len(sess.Capabilities().Banks) != 2 {
		t.Errorf("session has %d banks, want the 2 static ones (MEM, PMS) — no discovered bank exists on this radio", len(sess.Capabilities().Banks))
	}
}

// TestOpen_WrongRadio: the port answers ID; with "ID0800;" — an FT-710.
// Open must fail with a typed *driver.WrongRadioError carrying both IDs
// and must close the port.
func TestOpen_WrongRadio(t *testing.T) {
	p := newRespondingPort(t, slotImage{catID: "0800"})

	_, err := New(RealHardware).Open(testCtx(t), p.Port(), testIdentity)
	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Fatalf("Open = %v, want errors.Is match against driver.ErrWrongRadio", err)
	}
	var wre *driver.WrongRadioError
	if !errors.As(err, &wre) {
		t.Fatalf("Open error %v is not a *driver.WrongRadioError", err)
	}
	if wre.Want != "0362" || wre.Got != "0800" {
		t.Errorf("WrongRadioError = {Want:%q Got:%q}, want {Want:\"0362\" Got:\"0800\"}", wre.Want, wre.Got)
	}
	if _, werr := p.Port().Write([]byte("x")); werr == nil {
		t.Error("the port is still writable after a failed Open — Open must close the port it took ownership of")
	}
}

// TestSession_CapabilitiesIsADefensiveCopy: mutating what
// Session.Capabilities returned must never alter what the session itself
// enforces.
func TestSession_CapabilitiesIsADefensiveCopy(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	first := sess.Capabilities()
	mem, ok := first.Bank(spec.BankMemory)
	if !ok {
		t.Fatal("missing MEM bank")
	}
	mem.Fields[spec.FieldErase] = spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	first.Banks[0].Slots[0] = "TAMPERED"

	second := sess.Capabilities()
	if fs := second.FieldSupport(spec.BankMemory, spec.FieldErase); fs.CanWrite() {
		t.Error("FieldErase became writable in a fresh Capabilities() after a caller tampered with an earlier copy")
	}
	if second.Banks[0].Slots[0] != "001" {
		t.Errorf("MEM.Slots[0] = %q after tampering with an earlier copy, want \"001\"", second.Banks[0].Slots[0])
	}
}

// TestSession_CloseIdempotent: Close may be called twice.
func TestSession_CloseIdempotent(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})
	if err := sess.Close(); err != nil {
		t.Errorf("first Close() = %v, want nil", err)
	}
	if err := sess.Close(); err != nil {
		t.Errorf("second Close() = %v, want nil (idempotent)", err)
	}
}

// TestSession_DiagnosticsCountsUnexpectedFrames: a fresh session has seen
// none.
func TestSession_DiagnosticsCountsUnexpectedFrames(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})
	if got := sess.Diagnostics(); got.UnexpectedFrames != 0 {
		t.Errorf("Diagnostics() = %+v on a fresh session, want UnexpectedFrames 0", got)
	}
}

// TestWithTransportLogger_NilIsIgnored: a nil logger leaves the engine's
// own default in place.
func TestWithTransportLogger_NilIsIgnored(t *testing.T) {
	d, ok := New(Simulated, WithTransportLogger(nil)).(*ftdx5000Driver)
	if !ok {
		t.Fatal("New did not return a *ftdx5000Driver")
	}
	if d.transportLogger != nil {
		t.Error("WithTransportLogger(nil) installed a logger, want the engine's default kept")
	}
}

// capsContains reports whether ANY bank field of caps carries s, on
// either side.
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

// TestConsentOption_SessionCapsTransformed: a RealHardware session built
// WITH WithConsentedUnverifiedWrites carries exactly
// spec.ConsentUnverifiedWrites' product over the same session's
// unconsented capabilities, and FieldErase stays exempt.
func TestConsentOption_SessionCapsTransformed(t *testing.T) {
	_, plain := openSession(t, RealHardware, slotImage{})
	_, consented := openSession(t, RealHardware, slotImage{}, WithConsentedUnverifiedWrites())

	if capsContains(plain.Capabilities(), spec.ConsentedUnverified) {
		t.Fatal("an unconsented RealHardware session already carries ConsentedUnverified")
	}

	want := spec.ConsentUnverifiedWrites(plain.Capabilities())
	got := consented.Capabilities()
	if !reflect.DeepEqual(got, want) {
		t.Error("consented session capabilities differ from spec.ConsentUnverifiedWrites' product")
	}
	if fs := got.FieldSupport(spec.BankMemory, spec.FieldFrequency); fs.Write != spec.ConsentedUnverified || !fs.CanWrite() {
		t.Errorf("MEM frequency Write = %v (CanWrite %v), want ConsentedUnverified and writable", fs.Write, fs.CanWrite())
	}
	if fs := got.FieldSupport(spec.BankMemory, spec.FieldErase); fs.CanWrite() {
		t.Error("MEM erase became writable under consent — FieldErase is exempt from the transform structurally")
	}
}

// TestConsentOption_UnrecognisedProfileStaysFailSafe: an unrecognised
// Profile plus the consent option must produce NO writable field anywhere.
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

// TestProfileRecognised_MatchesTheDeclaredConstants: profileRecognised is
// true for exactly the two declared Profile constants and false for
// everything else.
func TestProfileRecognised_MatchesTheDeclaredConstants(t *testing.T) {
	for _, p := range []Profile{RealHardware, Simulated} {
		d, ok := New(p).(*ftdx5000Driver)
		if !ok {
			t.Fatal("New did not return a *ftdx5000Driver")
		}
		if !d.profileRecognised() {
			t.Errorf("profileRecognised() = false for declared Profile %v", p)
		}
	}
	for _, p := range []Profile{-1, 2, 3, 42, 99, Profile(math.MinInt), Profile(math.MaxInt)} {
		d, ok := New(p).(*ftdx5000Driver)
		if !ok {
			t.Fatal("New did not return a *ftdx5000Driver")
		}
		if d.profileRecognised() {
			t.Errorf("profileRecognised() = true for Profile(%d), which this package does not declare", int(p))
		}
	}
}

// TestOpen_UnconfiguredDialectRefusesToOpen: a hand-built driver with no
// dialect must not reach the wire at all.
func TestOpen_UnconfiguredDialectRefusesToOpen(t *testing.T) {
	p := newRespondingPort(t, slotImage{})

	d := &ftdx5000Driver{Base: driver.Base{Profile: Simulated}}

	sess, err := d.Open(testCtx(t), p.Port(), testIdentity)
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open with a zero dialect succeeded")
	}
	if got := p.Transcript(); len(got) != 0 {
		t.Errorf("the port received %v, want nothing — the refusal must precede the engine", got)
	}
}
