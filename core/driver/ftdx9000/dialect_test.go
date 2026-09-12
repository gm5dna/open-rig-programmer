// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx9000

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
// carries, invoked HERE rather than from a core/cat/ftdx9000 subpackage:
// this package never owns one (brief), because its dialect config is
// built inline in dialect.go.
func TestConformance(t *testing.T) {
	dialecttest.Run(t, Dialect())
}

// TestConformance_ZeroValue pins that an unconfigured cat.Dialect fails
// closed, universally — see dialecttest.RunZeroValue's own doc comment.
func TestConformance_ZeroValue(t *testing.T) {
	dialecttest.RunZeroValue(t)
}

// TestGate_RealP8OffsetRefusesDCSState is this package's OWN regression
// test for a conformance-suite gap TestConformance above surfaces:
// dialecttest's checkToneStateDomain forges its P8 probe at
// ctcssOffsetInMemoryFrame, a HARDCODED 23 — P8's byte offset in the
// REGISTERED 28-byte/9-digit frame every dialect before this wave shares.
// This dialect's frame is 27 bytes/8-digit (Lift Y, matrix §2): every field
// from P3 onward sits ONE BYTE TO THE LEFT of the registered layout, so
// P8's real offset here is 22, not 23. Byte 23 of THIS frame is P9's first
// tone-index digit (Lift Y's own new field), so dialecttest's splice edits
// an accidentally-still-valid tone index instead of P8 — the gate
// "admitting" it is correct behaviour on a well-formed frame, not the
// write-gate defect the upstream check is designed to catch.
//
// This test proves the actual gate is sound at the REAL offset, so
// TestConformance's failure is filed as a dialecttest bug (that suite's
// ctcssOffsetInMemoryFrame needs to derive from the dialect's own
// MemoryFrameLen/MemoryFreqDigits rather than a shared literal), not a
// defect in this package — see the driver report.
func TestGate_RealP8OffsetRefusesDCSState(t *testing.T) {
	d := Dialect()
	sl, err := d.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot(1): %v", err)
	}
	base, err := d.BuildMWSet(cat.MemoryData{
		Slot:   sl,
		FreqHz: 14250000,
		Mode:   cat.Mode('2'),
		Kind:   d.MWWriteKind(),
		CTCSS:  cat.CTCSSOff,
		Shift:  cat.ShiftSimplex,
	})
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}

	const realP8Offset = 22 // matrix §2: byte 22 (0-indexed), position 23
	frame := base.Bytes()
	if frame[realP8Offset] != cat.CTCSSOff.Wire() {
		t.Fatalf("frame[%d] = %q, want the CTCSSOff wire byte %q — this test's own offset assumption is wrong", realP8Offset, frame[realP8Offset], cat.CTCSSOff.Wire())
	}

	for _, dcs := range []cat.CTCSSState{cat.CTCSSDCSEncDec, cat.CTCSSDCSEnc} {
		forged := append([]byte(nil), frame...)
		forged[realP8Offset] = dcs.Wire()
		if d.AllowedCommand(forged) {
			t.Errorf("gate ADMITTED a real-P8-offset forged frame carrying DCS state %q under ToneStatesCTCSS: %q", dcs.Wire(), forged)
		}
	}
}

// TestCATID_ComesFromTheDialect pins the linkage: the driver's identity
// value is DERIVED from the dialect rather than restated, and is one of
// the THREE documented answers this row accepts (matrix §1.2, doc.go).
func TestCATID_ComesFromTheDialect(t *testing.T) {
	if catID != catDialect.CATID() {
		t.Errorf("catID = %q, want the dialect's %q", catID, catDialect.CATID())
	}
	found := false
	for _, id := range acceptedCATIDs {
		if catID == id {
			found = true
		}
	}
	if !found {
		t.Errorf("catID = %q, want one of %v (matrix §1.2: ID legend 0101/0102/0103)", catID, acceptedCATIDs)
	}
	if got := CapabilitiesUnverified().CATID; got != catID {
		t.Errorf("Capabilities().CATID = %q, want the same %q the probe compares against", got, catID)
	}
}

// TestDriver_CarriesADialect pins that New populates the field Open hands
// the engine.
func TestDriver_CarriesADialect(t *testing.T) {
	d, ok := New(Simulated).(*ftdx9000Driver)
	if !ok {
		t.Fatal("New did not return a *ftdx9000Driver")
	}
	if !d.dialect.Configured() {
		t.Fatal("ftdx9000Driver.dialect is a zero cat.Dialect — New must initialise it")
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

	d := &ftdx9000Driver{Base: driver.Base{Profile: Simulated}}

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
