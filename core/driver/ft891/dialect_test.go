// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	catft891 "github.com/gm5dna/open-rig-programmer/core/cat/ft891"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestCATID_ComesFromTheDialect pins the linkage (matrix §1.2): the
// driver's identity value is DERIVED from the dialect rather than restated
// alongside it, so the value the ID probe compares against and the value
// the capability data advertises cannot drift. The second assertion keeps
// the documented literal in the test as well as in the code — a dialect
// edit that silently changed the FT-891's CAT ID would fail here rather
// than quietly redefining what "an FT-891" is.
func TestCATID_ComesFromTheDialect(t *testing.T) {
	if catID != catft891.Dialect().CATID() {
		t.Errorf("catID = %q, want the dialect's %q", catID, catft891.Dialect().CATID())
	}
	if catID != "0650" {
		t.Errorf("catID = %q, want the documented %q (matrix §1.2: ID's P1 legend, \"0650: FT-891\", layout 763)", catID, "0650")
	}
	if got := CapabilitiesUnverified().CATID; got != catID {
		t.Errorf("Capabilities().CATID = %q, want the same %q the probe compares against", got, catID)
	}
}

// TestDriver_CarriesADialect pins that New populates the field Open hands
// the engine. A zero cat.Dialect would still compile; catching the omission
// at the COMPOSITION is what makes the cause legible, rather than meeting it
// as a refusal at the first Open (which the test below covers as the
// backstop).
func TestDriver_CarriesADialect(t *testing.T) {
	d, ok := New(Simulated).(*ft891Driver)
	if !ok {
		t.Fatal("New did not return a *ft891Driver")
	}
	if !d.dialect.Configured() {
		t.Fatal("ft891Driver.dialect is a zero cat.Dialect — New must initialise it (an unconfigured gate accepts nothing, so every frame would be refused)")
	}
	if d.dialect.CATID() != "0650" {
		t.Errorf("driver dialect CATID = %q, want the FT-891's \"0650\" — this driver must not be built over another radio's dialect", d.dialect.CATID())
	}
}

// TestSession_CarriesTheDriversDialect pins that Open's Session carries A
// CONFIGURED FT-891 dialect: Configured() true, CATID "0650", and the
// three read-shaping policies (MTReadSlots, MTP11, MemoryP5) this radio's
// read path depends on.
//
// NARROWED FROM A STRONGER CLAIM (LOW-2, task-1 review). This test cannot
// pin "the SAME dialect value Open's engine was gated with", which is what
// an earlier version of this comment claimed: this package holds exactly
// ONE FT-891 dialect value (catDialect), so every assertion below is
// equally true of a Session built by substituting catDialect for d.dialect
// in Open's Session literal (ft891.go) — that mutation was run and this
// test stayed green. Making the stronger property testable needs a SECOND,
// deliberately distinct but still-Configured cat.Dialect value, and no
// seam in this package builds one today (TestOpen_UnconfiguredDialectRefusesToOpen
// is the one hand-built dialect variant that exists, and it is the ZERO
// value, which cannot stand in for a distinguishable configured one). If a
// second FT-891 dialect variant is ever needed for another purpose, this
// test should be rebuilt on it to pin the plumb itself rather than a
// property every dialect value in this package already has.
func TestSession_CarriesTheDriversDialect(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	if !sess.dialect.Configured() {
		t.Fatal("Session.dialect is a zero cat.Dialect — Open must copy the driver's")
	}
	if sess.dialect.CATID() != "0650" {
		t.Errorf("Session dialect CATID = %q, want \"0650\" — a session must code through the same dialect its engine gates with", sess.dialect.CATID())
	}
	if got := sess.dialect.MTReadSlots(); got != cat.MTReadsMemoryPMS {
		t.Errorf("Session dialect MTReadSlots = %v, want cat.MTReadsMemoryPMS — the narrowing that makes this radio's discovery MR-only (its MT block's slot legend, layout 998-999)", got)
	}
	if got := sess.dialect.MTP11(); got != cat.P11TagDisplay {
		t.Errorf("Session dialect MTP11 = %v, want cat.P11TagDisplay — byte 28 is a LIVE TAG flag on this radio (layout 1016), and the read path reports it Known on the strength of that policy", got)
	}
	if got := sess.dialect.MemoryP5(); got != cat.P5Fixed {
		t.Errorf("Session dialect MemoryP5 = %v, want cat.P5Fixed — byte 21 is printed \"(Fixed)\" on all five blocks (layout 971/1006/1042/783/1129), which is what makes TxClar unreadable-true on this radio", got)
	}
}

// TestOpen_UnconfiguredDialectRefusesToOpen is the driver-side half of the
// fail-closed story, using a HAND-BUILT driver rather than New: a driver
// that was never given a radio to speak for must not reach the wire at all.
//
// transport.NewEngine takes the cat.Dialect whole and checks Configured(),
// so the refusal happens at construction — before any engine, reader
// goroutine or wire exchange exists — and says something true about the
// cause. The wire-level backstops remain: core/cat refuses every frame for
// an unconfigured dialect, and Engine.Do consults its gate before every
// write. This pins the OUTERMOST of the three, which is the one a
// misassembled driver actually meets.
//
// The transcript assertion is the "no engine created" half: not one byte may
// be written to the port on this path.
func TestOpen_UnconfiguredDialectRefusesToOpen(t *testing.T) {
	p := newRespondingPort(t, slotImage{})

	// Deliberately NOT via New: a hand-built driver with the zero dialect.
	d := &ft891Driver{profile: Simulated}

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
