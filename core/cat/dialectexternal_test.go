// SPDX-License-Identifier: GPL-3.0-or-later

// Package cat_test holds the tests that must NOT be able to reach this
// package's internals.
//
// Everything else in core/cat is an in-package test, which is right for
// checking unexported behaviour but useless for the one property this
// milestone exists to deliver: that a DIFFERENT package can describe a
// radio. An in-package test can pass while quietly depending on an
// unexported field, and would still pass if NewDialect required one —
// leaving M9c to discover at its first task that core/cat/ftdx10 cannot be
// written after all (Codex plan review, finding 8).
package cat_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
)

// fictionalRadioConfig is a complete dialect built using ONLY the exported
// API, as core/cat/ftdx10 will have to.
//
// It is deliberately not the FT-710 and not any real radio: the point is
// the API surface, not the data. If this stops compiling, an input a real
// model package needs has become unreachable from outside core/cat.
func fictionalRadioConfig() cat.DialectConfig {
	return cat.DialectConfig{
		CATID: "0761",
		ModeNames: map[cat.Mode]string{
			cat.ModeLSB: "LSB",
			cat.ModeUSB: "USB",
			cat.ModeFM:  "FM",
		},
		Slots: cat.SlotSpace{
			MemoryLo: 1, MemoryHi: 99,
			SixtyLo: 0, SixtyHi: 0,
			PMSPairs:      9,
			EmergencyWire: "",
			NoneWire:      "000",
		},
		EXItems: []cat.EXItem{
			{Addr: cat.EXAddress{P1: 1, P2: 1, P3: 1}, P1Label: "RADIO", P2Label: "GROUP", Name: "ITEM", Digits: 3},
			{Addr: cat.EXAddress{P1: 1, P2: 1, P3: 2}, P1Label: "RADIO", P2Label: "GROUP", Name: "TEXT ITEM", Digits: 16, Text: true},
		},
		MT:          cat.MTPolicy{Form: cat.MTFormShort, TagMaxBytes: 12, ClearTagByte: ' ', PadByte: ' '},
		Clarifier:   cat.ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		MWWriteKind: cat.KindMemory,
	}
}

// TestExternalPackageCanConstructADialect is the milestone's headline
// property, checked from where it actually matters.
func TestExternalPackageCanConstructADialect(t *testing.T) {
	d, err := cat.NewDialect(fictionalRadioConfig())
	if err != nil {
		t.Fatalf("cat.NewDialect from an external package: %v", err)
	}
	if !d.Configured() {
		t.Fatal("externally constructed dialect reports Configured() == false")
	}
	if d.CATID() != "0761" {
		t.Errorf("CATID() = %q, want %q", d.CATID(), "0761")
	}
}

// TestExternallyBuiltDialectIsUsable goes past construction: the dialect
// must actually WORK — build a frame, gate it, and parse the answer —
// because a constructor that returns something inert would satisfy the test
// above and still leave M9c stuck.
func TestExternallyBuiltDialectIsUsable(t *testing.T) {
	d, err := cat.NewDialect(fictionalRadioConfig())
	if err != nil {
		t.Fatalf("NewDialect: %v", err)
	}

	slot, err := d.MemorySlot(42)
	if err != nil {
		t.Fatalf("MemorySlot(42): %v", err)
	}

	cmd, err := d.BuildMRRead(slot)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}
	frame := cmd.Bytes()
	if len(frame) == 0 {
		t.Fatal("BuildMRRead produced an empty frame")
	}

	// Its OWN gate must admit what its OWN builder produced. A dialect
	// whose builder and gate disagree is worse than one that cannot be
	// built at all.
	if !d.AllowedCommand(frame) {
		t.Errorf("this dialect's gate refused a frame its own builder produced: %q", frame)
	}

	// And the EX inventory it was given is the one it reports.
	if got := len(d.EXItems()); got != 2 {
		t.Errorf("EXItems() length = %d, want 2", got)
	}
	if !d.KnownEXAddress(cat.EXAddress{P1: 1, P2: 1, P3: 2}) {
		t.Error("KnownEXAddress reports false for an address this config supplied")
	}
}

// TestExternalMustNewDialectPanicsOnBadData pins that the Must form is
// usable — and dangerous — from outside too, since that is where model
// packages will call it.
func TestExternalMustNewDialectPanicsOnBadData(t *testing.T) {
	cfg := fictionalRadioConfig()
	cfg.CATID = "toolong"

	defer func() {
		if recover() == nil {
			t.Error("cat.MustNewDialect did not panic on an invalid config")
		}
	}()
	cat.MustNewDialect(cfg)
}
