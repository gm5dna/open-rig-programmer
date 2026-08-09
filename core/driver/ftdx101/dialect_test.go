// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	catftdx101 "github.com/gm5dna/open-rig-programmer/core/cat/ftdx101"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestCATID_ComesFromTheDialect pins the linkage, per model: the driver's
// identity value is DERIVED from that model's dialect rather than restated
// alongside it, so the value the ID probe compares against and the value
// the capability data advertises cannot drift. The second assertion keeps
// the documented literal in the test as well as in the code — a dialect
// edit that silently changed either CAT ID would fail here rather than
// quietly redefining what "an FTDX101D" is.
func TestCATID_ComesFromTheDialect(t *testing.T) {
	for _, tt := range []struct {
		model testModel
		// dialect names the core/cat/ftdx101 accessor this model's
		// modelParams must have been built from — the two the dialect
		// package exports, and it deliberately exports no bare Dialect()
		// ("there are two models, so there are two of them").
		dialect func() cat.Dialect
	}{
		{testModels[0], catftdx101.DialectD},
		{testModels[1], catftdx101.DialectMP},
	} {
		t.Run(tt.model.name, func(t *testing.T) {
			fromPackage := tt.dialect().CATID()
			if got := tt.model.params.dialect.CATID(); got != fromPackage {
				t.Errorf("modelParams dialect CATID = %q, want core/cat/ftdx101's own %q", got, fromPackage)
			}
			if fromPackage != tt.model.catID {
				t.Errorf("dialect CATID = %q, want the documented %q (ID's P1 legend, layout 1070 and 1072: \"0681: FTDX101D\", \"0682: FTDX101MP\")", fromPackage, tt.model.catID)
			}
			if got := capabilitiesUnverified(tt.model.params).CATID; got != tt.model.catID {
				t.Errorf("Capabilities().CATID = %q, want the same %q the probe compares against", got, tt.model.catID)
			}
		})
	}
}

// TestSiblingNames_MapsExactlyThisPackagesTwoRadios pins the refusal's
// ID→name table, which is what makes a wrong-sibling refusal able to name
// both models (spec A5, plan D2).
//
// TWO entries and no more, deliberately. This package owns two
// registrations and knows their names; every other radio's name belongs to
// whatever package registers it, and a third entry here would be this
// package asserting something about a radio it does not drive. The
// consequence — a lookup miss yields "", which leaves
// WrongRadioError.GotModel empty and renders the ID-only text — is pinned
// end to end by TestOpen_WrongRadio's foreign-radio case.
//
// cmd/rigprog gains no ID→name table of its own (plan D2): its formatter
// renders a name only when the error carries one, and the driver is the
// only layer that knows both.
func TestSiblingNames_MapsExactlyThisPackagesTwoRadios(t *testing.T) {
	if len(siblingNames) != 2 {
		t.Fatalf("siblingNames has %d entries, want exactly 2 — this package registers two radios and must not claim to name a third", len(siblingNames))
	}
	for _, m := range testModels {
		if got := siblingNames[m.catID]; got != m.name {
			t.Errorf("siblingNames[%q] = %q, want %q", m.catID, got, m.name)
		}
	}
	// An FT-710's ID: a real radio, driven by another package, and
	// deliberately unknown here.
	if got, ok := siblingNames["0800"]; ok || got != "" {
		t.Errorf("siblingNames[\"0800\"] = %q (present: %v), want absent — naming another package's radio is that package's business", got, ok)
	}
}

// TestDriver_CarriesADialect pins that each constructor populates the field
// Open hands the engine, with the RIGHT model's dialect. A zero cat.Dialect
// would still compile; catching the omission at the COMPOSITION is what
// makes the cause legible, rather than meeting it as a refusal at the first
// Open (which the test below covers as the backstop).
func TestDriver_CarriesADialect(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			d, ok := m.newDrv(Simulated).(*ftdx101Driver)
			if !ok {
				t.Fatal("the constructor did not return a *ftdx101Driver")
			}
			if !d.model.dialect.Configured() {
				t.Fatal("the driver's dialect is a zero cat.Dialect — the constructor must initialise it (an unconfigured gate accepts nothing, so every frame would be refused)")
			}
			if got := d.model.dialect.CATID(); got != m.catID {
				t.Errorf("driver dialect CATID = %q, want %q — this constructor must not be built over the OTHER model's dialect, which is the one mistake two near-identical registrations make", got, m.catID)
			}
			if d.model.name != m.name {
				t.Errorf("driver model name = %q, want %q", d.model.name, m.name)
			}
		})
	}
}

// TestSession_CarriesTheDriversDialect pins the other half of the plumb: a
// Session codes and decodes through the SAME dialect its transport engine
// was gated with, and it is the dialect of the model that Opened.
//
// A session encoding with one radio's dialect while its transport gated
// with another's is the exact failure the M9c dialect seam exists to
// prevent — and in THIS package it is not a cross-package hazard but an
// in-package one, since two dialects live a few lines apart.
func TestSession_CarriesTheDriversDialect(t *testing.T) {
	for _, m := range testModels {
		t.Run(m.name, func(t *testing.T) {
			_, sess := openSession(t, m, Simulated, slotImage{})

			if !sess.dialect.Configured() {
				t.Fatal("Session.dialect is a zero cat.Dialect — Open must copy the driver's")
			}
			if got := sess.dialect.CATID(); got != m.catID {
				t.Errorf("Session dialect CATID = %q, want %q — a session must code through the same dialect its engine gates with, and through the model it was opened for", got, m.catID)
			}
		})
	}
}

// TestOpen_UnconfiguredDialectRefusesToOpen is the driver-side half of the
// fail-closed story, using a HAND-BUILT driver rather than a constructor: a
// driver that was never given a radio to speak for must not reach the wire
// at all.
//
// It matters more here than in a one-model package. modelParams carries the
// dialect, so a hand-built ftdx101Driver — the shape a future refactor, or
// a test helper, might produce — has a ZERO model and therefore a zero
// dialect AND an empty model name. transport.NewEngine takes the
// cat.Dialect whole and checks Configured(), so the refusal happens at
// construction, before any engine, reader goroutine or wire exchange
// exists, and says something true about the cause. The wire-level backstops
// remain: core/cat refuses every frame for an unconfigured dialect, and
// Engine.Do consults its gate before every write. This pins the OUTERMOST
// of the three, which is the one a misassembled driver actually meets.
//
// The transcript assertion is the "no engine created" half: not one byte
// may be written to the port on this path.
func TestOpen_UnconfiguredDialectRefusesToOpen(t *testing.T) {
	p := newRespondingPort(t, slotImage{catID: testModels[0].catID})

	// Deliberately NOT via NewD/NewMP: a hand-built driver with the zero
	// modelParams, and so the zero dialect.
	d := &ftdx101Driver{profile: Simulated}

	sess, err := d.Open(testCtx(t), p.Port(), testIdentity)
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open with a zero dialect succeeded — a driver with no model must not reach the wire at all")
	}
	if !errors.Is(err, transport.ErrUnconfiguredDialect) {
		t.Errorf("Open with a zero dialect = %v, want errors.Is match against transport.ErrUnconfiguredDialect", err)
	}
	if got := p.Transcript(); len(got) != 0 {
		t.Errorf("the port received %v, want nothing — the refusal must precede the engine, not merely its first frame", got)
	}
}

// TestDialects_AreTheTwoDistinctInstances pins that this package holds two
// DIFFERENT dialects and did not, say, build both models over DialectD().
//
// The failure it guards is silent and total: an MP driver carrying the D's
// dialect would advertise CATID "0681", refuse every real FTDX101MP as the
// wrong radio, and pass every test that only ever asked one model about
// itself. Everything else about the two instances is identical by
// construction (core/cat/ftdx101 builds both from one config), which is
// precisely why the ONE difference has to be asserted.
func TestDialects_AreTheTwoDistinctInstances(t *testing.T) {
	if modelD.dialect.CATID() == modelMP.dialect.CATID() {
		t.Fatalf("modelD and modelMP carry dialects with the same CAT ID (%q) — both models were built over ONE dialect", modelD.dialect.CATID())
	}
	if modelD.dialect.CATID() != "0681" || modelMP.dialect.CATID() != "0682" {
		t.Errorf("dialect CAT IDs = %q/%q, want \"0681\"/\"0682\" (layout 1070 and 1072)", modelD.dialect.CATID(), modelMP.dialect.CATID())
	}
	if modelD.name == modelMP.name {
		t.Errorf("both modelParams carry the name %q", modelD.name)
	}
}
