// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestDriver_CarriesADialect pins that New populates the field Open hands
// the engine. A zero cat.Dialect would still compile; catching the omission
// at the COMPOSITION is what makes the cause legible, rather than meeting it
// as a refusal at the first Open (which the test below covers as the
// backstop).
func TestDriver_CarriesADialect(t *testing.T) {
	d, ok := New(Simulated).(*ft991aDriver)
	if !ok {
		t.Fatal("New did not return a *ft991aDriver")
	}
	if !d.dialect.Configured() {
		t.Fatal("ft991aDriver.dialect is a zero cat.Dialect — New must initialise it (an unconfigured gate accepts nothing, so every frame would be refused)")
	}
	if d.dialect.CATID() != "0670" {
		t.Errorf("driver dialect CATID = %q, want the FT-991A's \"0670\" — this driver must not be built over another radio's dialect", d.dialect.CATID())
	}
}

// TestSession_CarriesAnFT991ADialect pins that Open's Session carries A
// CONFIGURED FT-991A dialect: Configured() true, CATID "0670", and the three
// read-shaping policies this radio's read path depends on — TWO OF WHICH ARE
// EXACTLY INVERTED against the FT-891 exemplar (matrix erratum M-E3).
//
// MTP11 is cat.P11Fixed here, so byte 28 is SCHEMA and read.go reports
// spec.FieldTagDisplay Unavailable; on the FT-891 it is P11TagDisplay and
// that driver reports the flag Known. MemoryP5 is cat.P5TxClar here, so byte
// 21 is a LIVE TX-clarifier state the parser carries through; on the FT-891
// it is P5Fixed and TxClar can never come back true. A driver copied from
// that package without changing these two would demand a byte this frame
// does not have and refuse one it does.
//
// NARROWED FROM A STRONGER CLAIM, on the FT-891's own recorded reasoning
// (its LOW-2, task-1 review): this test cannot pin "the SAME dialect value
// Open's engine was gated with", because this package holds exactly ONE
// FT-991A dialect value (catDialect), so every assertion below is equally
// true of a Session built by substituting catDialect for d.dialect in Open's
// Session literal. Making the stronger property testable needs a SECOND,
// deliberately distinct but still-Configured cat.Dialect value, and no seam
// in this package builds one — the zero value that
// TestOpen_UnconfiguredDialectRefusesToOpen uses cannot stand in for a
// distinguishable configured one.
func TestSession_CarriesAnFT991ADialect(t *testing.T) {
	_, sess := openSession(t, Simulated, slotImage{})

	if !sess.dialect.Configured() {
		t.Fatal("Session.dialect is a zero cat.Dialect — Open must copy the driver's")
	}
	if sess.dialect.CATID() != "0670" {
		t.Errorf("Session dialect CATID = %q, want \"0670\" — a session must code through the same dialect its engine gates with", sess.dialect.CATID())
	}
	if got := sess.dialect.MTP11(); got != cat.P11Fixed {
		t.Errorf("Session dialect MTP11 = %v, want cat.P11Fixed — MT's P11 legend reads \"P11 0: (Fixed)\" on this radio (layout 1015) where the FT-891 prints a live TAG flag, so the display-BEARING codec pair refuses here (matrix §2.3, erratum M-E3)", got)
	}
	if got := sess.dialect.MemoryP5(); got != cat.P5TxClar {
		t.Errorf("Session dialect MemoryP5 = %v, want cat.P5TxClar — `P5 0: TX CLAR \"OFF\" 1: TX CLAR \"ON\"` is printed on all five blocks carrying the grid (971, 1004, 1042, 787, 1122) where the FT-891 prints \"(Fixed)\" (matrix §2.2, erratum M-E3)", got)
	}
	if got := sess.dialect.ToneStates(); got != cat.ToneStatesCTCSSAndDCS {
		t.Errorf("Session dialect ToneStates = %v, want cat.ToneStatesCTCSSAndDCS — this radio's P8 legend prints five values where every registered sibling prints three (matrix §1.17)", got)
	}
}

// TestOpen_UnconfiguredDialectRefusesToOpen is the driver-side half of the
// fail-closed story, using a HAND-BUILT driver rather than New: a driver that
// was never given a radio to speak for must not reach the wire at all.
//
// transport.NewEngine takes the cat.Dialect whole and checks Configured(), so
// the refusal happens at construction — before any engine, reader goroutine
// or wire exchange exists — and says something true about the cause. The
// wire-level backstops remain: core/cat refuses every frame for an
// unconfigured dialect, and Engine.Do consults its gate before every write.
// This pins the OUTERMOST of the three, which is the one a misassembled
// driver actually meets.
//
// The transcript assertion is the "no engine created" half: not one byte may
// be written to the port on this path.
func TestOpen_UnconfiguredDialectRefusesToOpen(t *testing.T) {
	p := newRespondingPort(t, slotImage{})

	// Deliberately NOT via New: a hand-built driver with the zero dialect.
	d := &ft991aDriver{profile: Simulated}

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
