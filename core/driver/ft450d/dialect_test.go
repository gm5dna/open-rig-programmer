// SPDX-License-Identifier: GPL-3.0-or-later

package ft450d

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/dialecttest"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestConformance runs core/cat/dialecttest's whole conformance suite
// against this radio's dialect — the same proof every registered dialect
// carries, invoked HERE rather than from a core/cat/ft450d subpackage: this
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
// documented literal (matrix §1.6).
func TestCATID_ComesFromTheDialect(t *testing.T) {
	if catID != catDialect.CATID() {
		t.Errorf("catID = %q, want the dialect's %q", catID, catDialect.CATID())
	}
	if catID != "0244" {
		t.Errorf("catID = %q, want the documented %q (matrix §1.6: ID command's P1 legend, layout:605)", catID, "0244")
	}
	if got := CapabilitiesUnverified().CATID; got != catID {
		t.Errorf("Capabilities().CATID = %q, want the same %q the probe compares against", got, catID)
	}
}

// TestDriver_CarriesADialect pins that New populates the field Open hands
// the engine.
func TestDriver_CarriesADialect(t *testing.T) {
	d, ok := New(Simulated).(*ft450dDriver)
	if !ok {
		t.Fatal("New did not return a *ft450dDriver")
	}
	if !d.dialect.Configured() {
		t.Fatal("ft450dDriver.dialect is a zero cat.Dialect — New must initialise it")
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

	d := &ft450dDriver{Base: driver.Base{Profile: Simulated}}

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

// TestMemorySlot_Range pins matrix §2.5: 001-500, one more than the write
// ceiling would ever reach (PMS starts at 501).
func TestMemorySlot_Range(t *testing.T) {
	d := Dialect()
	if _, err := d.MemorySlot(0); err == nil {
		t.Error("MemorySlot(0) succeeded, want refused (MemoryLo is 1)")
	}
	sl, err := d.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot(1) = %v, want no error", err)
	}
	if sl.Wire() != "001" {
		t.Errorf("MemorySlot(1).Wire() = %q, want %q", sl.Wire(), "001")
	}
	if _, err := d.MemorySlot(501); err == nil {
		t.Error("MemorySlot(501) succeeded, want refused (MemoryHi is 500; 501 is the PMS bank)")
	}
}

// TestModeHoleAtA pins matrix §1.2: 'A' is a clean hole, not a valid mode.
func TestModeHoleAtA(t *testing.T) {
	d := Dialect()
	if d.ValidMode(cat.Mode('A')) {
		t.Error("ValidMode('A') = true, want false — no P6-carrying block in this manual prints an 'A' row")
	}
	for _, m := range []byte{'1', '9', 'B', 'C'} {
		if !d.ValidMode(cat.Mode(m)) {
			t.Errorf("ValidMode(%q) = false, want true", string(m))
		}
	}
	for _, m := range []byte{'D', 'E', 'F'} {
		if d.ValidMode(cat.Mode(m)) {
			t.Errorf("ValidMode(%q) = true, want false — absent from every P6-carrying block", string(m))
		}
	}
}
