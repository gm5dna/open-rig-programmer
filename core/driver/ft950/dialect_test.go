// SPDX-License-Identifier: GPL-3.0-or-later

package ft950

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat/dialecttest"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestConformance runs core/cat/dialecttest's whole conformance suite
// against this radio's dialect — the same proof every registered dialect
// carries, invoked HERE rather than from a core/cat/ft950 subpackage: this
// package never owns one (brief), because its dialect config is built
// inline in dialect.go.
func TestConformance(t *testing.T) {
	dialecttest.Run(t, Dialect())
}

// TestConformance_ZeroValue pins that an unconfigured cat.Dialect fails
// closed, universally.
func TestConformance_ZeroValue(t *testing.T) {
	dialecttest.RunZeroValue(t)
}

// TestCATID_ComesFromTheDialect pins the linkage: the driver's identity
// value is DERIVED from the dialect rather than restated, and matches the
// documented literal (matrix §4).
func TestCATID_ComesFromTheDialect(t *testing.T) {
	if catID != catDialect.CATID() {
		t.Errorf("catID = %q, want the dialect's %q", catID, catDialect.CATID())
	}
	if catID != "0310" {
		t.Errorf("catID = %q, want the documented %q (matrix §4: ID command's P1 legend, layout:676-682)", catID, "0310")
	}
	if got := CapabilitiesUnverified().CATID; got != catID {
		t.Errorf("Capabilities().CATID = %q, want the same %q the probe compares against", got, catID)
	}
}

// TestDriver_CarriesADialect pins that New populates the field Open hands
// the engine.
func TestDriver_CarriesADialect(t *testing.T) {
	d, ok := New(Simulated).(*ft950Driver)
	if !ok {
		t.Fatal("New did not return a *ft950Driver")
	}
	if !d.dialect.Configured() {
		t.Fatal("ft950Driver.dialect is a zero cat.Dialect — New must initialise it")
	}
	if d.dialect.CATID() != catID {
		t.Errorf("driver dialect CATID = %q, want %q", d.dialect.CATID(), catID)
	}
}

// TestSession_CarriesTheDriversDialect pins that Open's Session carries a
// CONFIGURED dialect matching the driver's own.
func TestSession_CarriesTheDriversDialect(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	if !sess.dialect.Configured() {
		t.Fatal("Session.dialect is a zero cat.Dialect — Open must copy the driver's")
	}
	if sess.dialect.CATID() != catID {
		t.Errorf("Session dialect CATID = %q, want %q", sess.dialect.CATID(), catID)
	}
}

// TestOpen_UnconfiguredDialectRefusesToOpen is the driver-side half of the
// fail-closed story, using a HAND-BUILT driver rather than New.
func TestOpen_UnconfiguredDialectRefusesToOpen(t *testing.T) {
	p := newRespondingPort(t, slotImage{})

	d := &ft950Driver{Base: driver.Base{Profile: Simulated}}

	sess, err := d.Open(testCtx(t), p.Port(), driver.Identity{})
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open with a zero dialect succeeded — a driver with no dialect must not reach the wire at all")
	}
	if !errors.Is(err, transport.ErrUnconfiguredDialect) {
		t.Errorf("Open with a zero dialect = %v, want errors.Is match against transport.ErrUnconfiguredDialect", err)
	}
	if got := p.Transcript(); len(got) != 0 {
		t.Errorf("the port received %v, want nothing — the refusal must precede the engine", got)
	}
}

// TestMemorySlot_StartsAtZero pins this radio's own delta from every
// sibling in this wave (matrix §1.5, doc.go entry 4): channel 0 is a real,
// regular memory channel, not refused and not a none-placeholder.
func TestMemorySlot_StartsAtZero(t *testing.T) {
	d := Dialect()
	sl, err := d.MemorySlot(0)
	if err != nil {
		t.Fatalf("MemorySlot(0) = %v, want no error (MemoryLo is 0 on this radio)", err)
	}
	if sl.Wire() != "000" {
		t.Errorf("MemorySlot(0).Wire() = %q, want %q", sl.Wire(), "000")
	}
	if _, err := d.MemorySlot(100); err == nil {
		t.Error("MemorySlot(100) succeeded, want refused (MemoryHi is 99)")
	}
}
