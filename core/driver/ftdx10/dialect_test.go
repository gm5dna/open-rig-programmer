// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

import (
	"errors"
	"testing"

	catftdx10 "github.com/gm5dna/open-rig-programmer/core/cat/ftdx10"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestCATID_ComesFromTheDialect pins the linkage: the driver's identity
// value is DERIVED from the dialect rather than restated alongside it, so
// the value the ID probe compares against and the value the capability data
// advertises cannot drift. The second assertion keeps the documented
// literal in the test as well as in the code — a dialect edit that silently
// changed the FTdx10's CAT ID would fail here rather than quietly
// redefining what "an FTdx10" is.
func TestCATID_ComesFromTheDialect(t *testing.T) {
	if catID != catftdx10.Dialect().CATID() {
		t.Errorf("catID = %q, want the dialect's %q", catID, catftdx10.Dialect().CATID())
	}
	if catID != "0761" {
		t.Errorf("catID = %q, want the documented %q (manual lines 976-984: the ID answer's four-character P1)", catID, "0761")
	}
	if got := CapabilitiesUnverified().CATID; got != catID {
		t.Errorf("Capabilities().CATID = %q, want the same %q the probe compares against", got, catID)
	}
}

// TestDriver_CarriesADialect pins that New populates the field Open hands
// the engine. A zero cat.Dialect would still compile; catching the omission
// at the COMPOSITION is what makes the cause legible, rather than meeting
// it as a refusal at the first Open (which the test below covers as the
// backstop).
func TestDriver_CarriesADialect(t *testing.T) {
	d, ok := New(Simulated).(*ftdx10Driver)
	if !ok {
		t.Fatal("New did not return a *ftdx10Driver")
	}
	if !d.dialect.Configured() {
		t.Fatal("ftdx10Driver.dialect is a zero cat.Dialect — New must initialise it (an unconfigured gate accepts nothing, so every frame would be refused)")
	}
	if d.dialect.CATID() != "0761" {
		t.Errorf("driver dialect CATID = %q, want the FTdx10's \"0761\" — this driver must not be built over another radio's dialect", d.dialect.CATID())
	}
}

// TestSession_CarriesTheDriversDialect pins the other half of the plumb: a
// Session codes and decodes through the SAME dialect its transport engine
// was gated with. A session encoding with one radio's dialect while its
// transport gated with another's is the exact failure the M9c dialect seam
// exists to prevent, and it would be invisible in a repository with one
// radio in it.
func TestSession_CarriesTheDriversDialect(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	if !sess.dialect.Configured() {
		t.Fatal("Session.dialect is a zero cat.Dialect — Open must copy the driver's")
	}
	if sess.dialect.CATID() != "0761" {
		t.Errorf("Session dialect CATID = %q, want \"0761\" — a session must code through the same dialect its engine gates with", sess.dialect.CATID())
	}
}

// TestOpen_UnconfiguredDialectRefusesToOpen is the driver-side half of the
// fail-closed story, using a HAND-BUILT driver rather than New: a driver
// that was never given a radio to speak for must not reach the wire at
// all.
//
// transport.NewEngine takes the cat.Dialect whole and checks Configured(),
// so the refusal happens at construction — before any engine, reader
// goroutine or wire exchange exists — and says something true about the
// cause. The wire-level backstops remain: core/cat refuses every frame for
// an unconfigured dialect, and Engine.Do consults its gate before every
// write. This pins the OUTERMOST of the three, which is the one a
// misassembled driver actually meets.
//
// The transcript assertion is the "no engine created" half: not one byte
// may be written to the port on this path.
func TestOpen_UnconfiguredDialectRefusesToOpen(t *testing.T) {
	p := newRespondingPort(t, slotImage{})

	// Deliberately NOT via New: a hand-built driver with the zero dialect.
	d := &ftdx10Driver{profile: Simulated}

	sess, err := d.Open(testCtx(t), p.Port(), testIdentity)
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open with a zero dialect succeeded — a driver with no dialect must not reach the wire at all")
	}
	if !errors.Is(err, transport.ErrUnconfiguredDialect) {
		t.Errorf("Open with a zero dialect = %v, want errors.Is match against transport.ErrUnconfiguredDialect", err)
	}
	if got := p.Transcript(); len(got) != 0 {
		t.Errorf("the port received %v, want nothing — the refusal must precede the engine, not merely its first frame", got)
	}
}
