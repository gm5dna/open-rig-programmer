// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx1200

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
)

const testCtxTimeout = 5 * time.Second

var (
	_ driver.Driver              = (*ftdx1200Driver)(nil)
	_ driver.Session             = (*Session)(nil)
	_ driver.DiagnosticsReporter = (*Session)(nil)
)

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testCtxTimeout)
	t.Cleanup(cancel)
	return ctx
}

var testIdentity = driver.Identity{Port: "scripted-pipe", USBSerial: "SIM0582"}

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
		t.Fatalf("Open returned a %T, want *ftdx1200.Session", sess)
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
	for _, bank := range caps.Banks {
		if fs := caps.FieldSupport(bank.ID, "frequency"); fs.CanWrite() {
			t.Errorf("bank %s: frequency is write-Supported under RealHardware, want nothing writable", bank.ID)
		}
	}
}

func TestDriver_UnrecognisedProfileFailsSafe(t *testing.T) {
	d := New(Profile(99))
	caps := d.Capabilities()
	for _, bank := range caps.Banks {
		if fs := caps.FieldSupport(bank.ID, "frequency"); fs.CanWrite() {
			t.Errorf("bank %s: frequency is write-Supported under an unrecognised Profile, want nothing writable", bank.ID)
		}
	}
}

// TestOpen_AcceptsEitherCATID is the option-split's own proof: matrix
// §1.6/§2.2 — one radio, two IDs. Both must identify; a third must not.
func TestOpen_AcceptsEitherCATID(t *testing.T) {
	for _, id := range []string{"0582", "0583"} {
		t.Run(id, func(t *testing.T) {
			p, s := openSession(t, Simulated, slotImage{catID: id})
			if got := s.Identity().CATID; got != id {
				t.Errorf("Identity().CATID = %q, want %q", got, id)
			}
			if transcript := p.Transcript(); len(transcript) != 2 {
				t.Errorf("transcript = %v, want exactly [AI0;, ID;] — no discovery sweep", transcript)
			}
		})
	}
}

func TestOpen_AThirdIDRefuses(t *testing.T) {
	p := newRespondingPort(t, slotImage{catID: "0581"})
	sess, err := New(Simulated).Open(testCtx(t), p.Port(), testIdentity)
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open succeeded against a third, unaccepted CAT ID, want *driver.WrongRadioError")
	}
	var wrong *driver.WrongRadioError
	if !errors.As(err, &wrong) {
		t.Fatalf("Open error = %v, want *driver.WrongRadioError", err)
	}
	if wrong.Got != "0581" {
		t.Errorf("Got = %q, want \"0581\"", wrong.Got)
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
}

func TestOpen_UnconfiguredDialectRefusesToOpen(t *testing.T) {
	p := newRespondingPort(t, slotImage{})
	d := &ftdx1200Driver{Base: driver.Base{Profile: Simulated}} // zero dialect, deliberately not via New

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
