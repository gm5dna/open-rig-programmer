// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import "testing"

// satelliteLayout is layout480()'s own config with Satellite: true added
// — a standalone literal rather than a layout480() copy-and-override
// (Layout has no setter; TestZeroLayout_FailsClosedOnEveryAxis's own
// reasoning), used only to prove the Satellite axis's gate behaviour in
// isolation from any real row's other axes.
func satelliteLayout() Layout {
	return MustNewLayout(LayoutConfig{
		Book:         Book480,
		Model:        "SATELLITE-TEST",
		RecordLen:    RecordLen,
		P10:          P10FixedZero,
		P12:          P12FixedZero,
		P13:          P13FixedZero,
		P2:           P2FixedZero,
		Byte19:       Byte19Lockout,
		Byte28:       Byte28FixedZero,
		Byte3940:     Byte3940StepIndex,
		Byte41:       Byte41FixedZero,
		ToneModes:    ToneModesThree,
		MaxEXAddress: 60,
		ModeNames:    modeNames480(),
		Slots:        []SlotRange{{Class: SlotMemory, Lo: 0, Hi: 99}},
		PrintedFixed: append(commonPrintedFixed(),
			FixedField{Pos: 4, Printed: "0"},
			FixedField{Pos: 28, Printed: "0"},
			FixedField{Pos: 41, Printed: "0"},
		),
		Satellite: true,
	})
}

// TestAllowedCommand_SatelliteAxisGatesSAAndSI is the safety pin allowlist.go's
// own doc comment demands of any addition here: a layout that does not set
// Satellite refuses SA/SI outright, and one that does admits exactly the
// shapes core/kw/ts2000's BuildSARead/BuildSASet/BuildSISet produce and
// nothing malformed.
func TestAllowedCommand_SatelliteAxisGatesSAAndSI(t *testing.T) {
	sat := satelliteLayout()
	notSat := layout480() // Satellite left at its zero value, false

	admitted := []string{
		"SA;",          // the bare read
		"SA0000000;",   // a Set, every flag off, channel 0
		"SA1910101;",   // a Set, a representative mix of flags, channel 9
		"SI0ABCDEFGH;", // an SI Set, an eight-byte name
		"SI9        ;", // an SI Set, an eight-space name
	}
	for _, frame := range admitted {
		if !sat.AllowedCommand([]byte(frame)) {
			t.Errorf("satelliteLayout: AllowedCommand(%q) = false, want true", frame)
		}
		if notSat.AllowedCommand([]byte(frame)) {
			t.Errorf("layout480 (Satellite unset): AllowedCommand(%q) = true, want false — this row's book prints no SA/SI", frame)
		}
	}

	refused := []string{
		"SA",            // no terminator
		"SA;;",          // two terminators
		"SA200000000;",  // twelve bytes: longer than the ten-byte Set shape
		"SA200000;",     // nine bytes: shorter than the Set shape
		"SA2000000;",    // P1 = '2', not '0'/'1'
		"SA0A00000;",    // P2 = 'A', not a digit
		"SA0000002;",    // P7 = '2', not '0'/'1'
		"SI;",           // SI has no Read this codec builds
		"SIA00000000;",  // P1 = 'A', not a digit 0-9
		"SI0ABCDEFG;",   // eleven bytes: shorter than the twelve-byte Set shape
		"SI0ABCDEFGHI;", // thirteen bytes: longer than the Set shape
	}
	for _, frame := range refused {
		if sat.AllowedCommand([]byte(frame)) {
			t.Errorf("satelliteLayout: AllowedCommand(%q) = true, want false", frame)
		}
	}
}

// TestAllowedCommand_SatelliteBuildersAreAdmitted is
// TestAllowedCommand_AcceptsExactlyTheEightGrammars' own discipline
// (allowlist_test.go) restated for the two frames that discipline cannot
// cover — core/kw/ts2000.BuildSARead/BuildSASet/BuildSISet are in a
// different package this one may not import (the doc comment on
// validSACommand explains why), so this re-validates the SHAPES
// core/kw/ts2000/satellite_test.go pins those builders produce, rather
// than calling the builders themselves.
func TestAllowedCommand_SatelliteBuildersAreAdmitted(t *testing.T) {
	sat := satelliteLayout()
	for _, frame := range []string{"SA;", "SA0000000;", "SA1911111;", "SI5SO-50   ;"} {
		if !sat.AllowedCommand([]byte(frame)) {
			t.Errorf("AllowedCommand(%q) = false, want true (core/kw/ts2000's own builder shape)", frame)
		}
	}
}
