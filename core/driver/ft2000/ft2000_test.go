// SPDX-License-Identifier: GPL-3.0-or-later

package ft2000

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// testCtxTimeout is small: unlike every 5xx/EMG-discovering sibling, this
// radio's Open is AI0 + one ID probe and nothing else (yaesuParams.Probe
// is yaesu.NoProbe — matrix §2.4, no such bank exists), so there is no
// discovery walk to budget wall clock for.
const testCtxTimeout = 5 * time.Second

// Compile-time proof of the seams this package satisfies.
var (
	_ driver.Driver              = (*ft2000Driver)(nil)
	_ driver.Session             = (*Session)(nil)
	_ driver.DiagnosticsReporter = (*Session)(nil)
)

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testCtxTimeout)
	t.Cleanup(cancel)
	return ctx
}

var testIdentity = driver.Identity{Port: "scripted-pipe", USBSerial: "SIM0251"}

// testModel is one row of the per-model table nearly every test in this
// package walks: matrix §4's claim ("the two rows differ only in Model
// and CATID") is a claim this package makes, and a suite exercising only
// one model would leave the other's registration entirely untested.
type testModel struct {
	name         string
	params       modelParams
	newDrv       func(Profile, ...Option) driver.Driver
	catID        string
	siblingName  string
	siblingCATID string
}

var testModels = []testModel{
	{
		name: "FT-2000", params: modelFT2000, newDrv: NewFT2000, catID: "0251",
		siblingName: "FT-2000D", siblingCATID: "0252",
	},
	{
		name: "FT-2000D", params: modelFT2000D, newDrv: NewFT2000D, catID: "0252",
		siblingName: "FT-2000", siblingCATID: "0251",
	},
}

// openSession starts a respondingPort serving img, opens a session over
// it for model m with the given profile, and registers cleanup for both.
func openSession(t *testing.T, m testModel, profile Profile, img slotImage, opts ...Option) (*respondingPort, *Session) {
	t.Helper()
	if img.catID == "" {
		img.catID = m.params.dialect.CATID()
	}
	p := newRespondingPort(t, img)

	sess, err := m.newDrv(profile, opts...).Open(testCtx(t), p.Port(), testIdentity)
	if err != nil {
		t.Fatalf("Open: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	s, ok := sess.(*Session)
	if !ok {
		t.Fatalf("Open returned a %T, want *ft2000.Session", sess)
	}
	return p, s
}

func TestDriver_ModelAndCapabilities(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			d := m.newDrv(RealHardware)
			if got := d.Model(); got != m.name {
				t.Errorf("Model() = %q, want %q", got, m.name)
			}
			caps := d.Capabilities()
			if caps.Model != m.name {
				t.Errorf("Capabilities().Model = %q, want %q", caps.Model, m.name)
			}
			if caps.CATID != m.catID {
				t.Errorf("Capabilities().CATID = %q, want %q", caps.CATID, m.catID)
			}
			// RealHardware, unconditionally: writeTrialsComplete is false.
			for _, bank := range caps.Banks {
				if fs := caps.FieldSupport(bank.ID, "frequency"); fs.CanWrite() {
					t.Errorf("bank %s: frequency is write-Supported under RealHardware, want nothing writable (writeTrialsComplete is false)", bank.ID)
				}
			}
		})
	}
}

func TestDriver_UnrecognisedProfileFailsSafe(t *testing.T) {
	d := NewFT2000(Profile(99))
	caps := d.Capabilities()
	for _, bank := range caps.Banks {
		if fs := caps.FieldSupport(bank.ID, "frequency"); fs.CanWrite() {
			t.Errorf("bank %s: frequency is write-Supported under an unrecognised Profile, want nothing writable", bank.ID)
		}
	}
}

func TestOpen_Succeeds(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			p, s := openSession(t, m, Simulated, slotImage{})
			if got := s.Identity().CATID; got != m.catID {
				t.Errorf("Identity().CATID = %q, want %q", got, m.catID)
			}
			// AI0 + ID only — no discovery sweep (matrix §2.4).
			transcript := p.Transcript()
			if len(transcript) != 2 {
				t.Errorf("transcript = %v, want exactly [AI0;, ID;] — no 5xx/EMG discovery for this radio", transcript)
			}
		})
	}
}

func TestOpen_WrongRadioNamesTheSibling(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			p := newRespondingPort(t, slotImage{catID: m.siblingCATID})
			sess, err := m.newDrv(Simulated).Open(testCtx(t), p.Port(), testIdentity)
			if err == nil {
				_ = sess.Close()
				t.Fatal("Open succeeded against the sibling's CAT ID, want *driver.WrongRadioError")
			}
			var wrong *driver.WrongRadioError
			if !errors.As(err, &wrong) {
				t.Fatalf("Open error = %v, want *driver.WrongRadioError", err)
			}
			if wrong.WantModel != m.name || wrong.GotModel != m.siblingName {
				t.Errorf("WrongRadioError = {Want:%q WantModel:%q Got:%q GotModel:%q}, want WantModel %q GotModel %q",
					wrong.Want, wrong.WantModel, wrong.Got, wrong.GotModel, m.name, m.siblingName)
			}
		})
	}
}

func TestOpen_ForeignCATIDNamesNoModel(t *testing.T) {
	p := newRespondingPort(t, slotImage{catID: "9999"})
	sess, err := NewFT2000(Simulated).Open(testCtx(t), p.Port(), testIdentity)
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open succeeded against a foreign CAT ID, want *driver.WrongRadioError")
	}
	var wrong *driver.WrongRadioError
	if !errors.As(err, &wrong) {
		t.Fatalf("Open error = %v, want *driver.WrongRadioError", err)
	}
	if wrong.GotModel != "" {
		t.Errorf("GotModel = %q, want \"\" — this package names only its own two radios", wrong.GotModel)
	}
}

func TestOpen_UnconfiguredDialectRefusesToOpen(t *testing.T) {
	p := newRespondingPort(t, slotImage{catID: "0251"})
	d := &ft2000Driver{Base: driver.Base{Profile: Simulated}} // zero model, deliberately not via NewFT2000

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
	_, s := openSession(t, testModels[0], Simulated, slotImage{})
	if got := s.Diagnostics().UnexpectedFrames; got != 0 {
		t.Errorf("UnexpectedFrames = %d, want 0 on a clean session", got)
	}
}

func TestSession_Close_Idempotent(t *testing.T) {
	_, s := openSession(t, testModels[0], Simulated, slotImage{})
	if err := s.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
