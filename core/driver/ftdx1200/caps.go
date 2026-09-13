// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx1200

import (
	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// writeTrialsComplete is FALSE. NO FTdx1200 HAS EVER BEEN ASKED ANYTHING
// BY THIS PROJECT (matrix header note).
const writeTrialsComplete = false

// modeDisplayNames returns the selectable mode display names, in wire-code
// order, DERIVED FROM THE DIALECT — matrix §1.2/§2.5. cat.ModeUnset is
// excluded (accept-only placeholder); the hole at 'A' is simply not in
// the dialect's ModeNames map, so it never appears here either.
func modeDisplayNames() []string {
	var names []string
	for b := 0; b <= 0xFF; b++ {
		m := cat.Mode(byte(b))
		if m == cat.ModeUnset || !dialect.ValidMode(m) {
			continue
		}
		names = append(names, dialect.ModeName(m))
	}
	return names
}

func memSlots() []string {
	var slots []string
	for n := 1; ; n++ {
		s, err := dialect.MemorySlot(n)
		if err != nil {
			return slots
		}
		slots = append(slots, s.Wire())
	}
}

func pmsSlots() []string {
	var slots []string
	for pair := 1; ; pair++ {
		lower, err := dialect.PMSSlot(pair, false)
		if err != nil {
			return slots
		}
		upper, err := dialect.PMSSlot(pair, true)
		if err != nil {
			return slots
		}
		slots = append(slots, lower.Wire(), upper.Wire())
	}
}

// bankFields builds the per-field support map shared by the MEM and PMS
// banks (matrix §3: one field grid governs both).
//
//   - rw covers the FIVE fields the bare MW/MR frame always carries:
//     frequency, mode, clarifier, CTCSS state and shift.
//   - FieldCTCSSTone is the ZERO FieldSupport, BOTH directions,
//     UNCONDITIONALLY (both profiles) — matrix §1.3: P9 is printed-fixed
//     "00" on read AND write alike, unlike the sibling ftdx3000's
//     asymmetric split; there is no live tone state anywhere on this
//     radio's CAT surface to grade Supported/Unverified against.
//   - FieldTag/FieldTagDisplay are the zero FieldSupport: NoTag (matrix
//     §0 — zero grep hits of any kind, stronger than ftdx3000's own).
//   - FieldScanSkip/FieldErase are the zero FieldSupport: no scan-skip
//     byte, no erase/clear command for a memory channel specifically.
//   - the seventeen Icom-tier fields are the zero FieldSupport.
//
// Each call returns a fresh map, so MEM and PMS never share one.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:  rw,
		spec.FieldMode:       rw,
		spec.FieldClarifier:  rw,
		spec.FieldCTCSSState: rw,
		spec.FieldCTCSSTone:  {},
		spec.FieldShift:      rw,

		spec.FieldTag:        {},
		spec.FieldTagDisplay: {},

		spec.FieldScanSkip: {},
		spec.FieldErase:    {},

		spec.FieldTxFrequency:       {},
		spec.FieldDuplex:            {},
		spec.FieldOffset:            {},
		spec.FieldToneMode:          {},
		spec.FieldToneTx:            {},
		spec.FieldToneRx:            {},
		spec.FieldDTCSCode:          {},
		spec.FieldDTCSPolarity:      {},
		spec.FieldFilter:            {},
		spec.FieldDataMode:          {},
		spec.FieldTuningStepEnabled: {},
		spec.FieldTuningStep:        {},
		spec.FieldProgramTuningStep: {},
		spec.FieldAttenuator:        {},
		spec.FieldPreamp:            {},
		spec.FieldAntenna:           {},
		spec.FieldIPPlus:            {},
	}
}

// standardTones is the standard 50-tone chart, addressable so caps.go can
// slice it.
var standardTones = spec.StandardCTCSSTones()

// baseCapabilities assembles the static baseline both profiles share.
func baseCapabilities(rw spec.FieldSupport) spec.Capabilities {
	return spec.Capabilities{
		Model: modelName,
		CATID: dialect.CATID(),
		// MANUAL-EVIDENCED (matrix §2.3): documented transmit surface
		// (MX MOX SET, layout:914-918, and the rest of the transmit
		// command set).
		Transmit: spec.HasTransmitter,
		Banks: []spec.Bank{
			{
				ID: spec.BankMemory, Label: "Memories",
				Slots: memSlots(), NoBlank: false, Fields: bankFields(rw),
			},
			{
				ID: spec.BankPMS, Label: "Scan limits (PMS)",
				Slots: pmsSlots(), NoBlank: false, Fields: bankFields(rw),
			},
		},
		Modes: modeDisplayNames(),
		// NoTag (matrix §0/§2.6): zero grep hits of any kind.
		TagLen: 0,
		NoTag:  true,
		// ASSUMED (matrix §2.7): the Yaesu-family default; this manual
		// states no step. MaxAbsHz 9990, the same V10 correction every
		// ft2000-family sibling makes.
		ClarMaxHz:  dialect.Clarifier().MaxAbsHz,
		ClarStepHz: dialect.Clarifier().StepHz,
		// The standard 50-tone chart is still declared (matrix §2.8's own
		// reading): P9 is fixed on this radio, but CTCSSTones describes
		// the chart, not whether this radio can select from it — the same
		// distinction the Icom-tier "chart present but field zero" shape
		// elsewhere in this fleet keeps. CTCSSToneRange stays nil.
		CTCSSTones: standardTones[:],
		// MANUAL-EVIDENCED (matrix §2.10): menu 039 GENERAL CAT RATE,
		// layout:489 (numbering NOT shared with ftdx3000's own menu 038).
		// DefaultBaud ASSUMED (matrix §2.10): no factory-default column.
		Bauds:       []int{4800, 9600, 19200, 38400},
		DefaultBaud: 38400,
		// OPEN in the matrix (§2.11: FA/FB blocks exist but were not
		// re-read in full there). This driver reads them directly, the
		// same "close the matrix's own open item" move ftdx3000's own
		// caps.go makes for its own frequency range — FA/FB Set legends
		// both print "P1 0030000 - 60000000 (Hz)" (layout:664, :676),
		// the identical range to the sibling ftdx3000 (plausible given
		// the shared product family, but independently read here rather
		// than copied).
		MinFreqHz: 30_000,
		MaxFreqHz: 60_000_000,
		// ASSUMED (matrix §2.12): no manual statement either way.
		RequiredSlots: nil,
		// MANUAL-EVIDENCED (matrix §2.13): "0: Simplex 1: Plus Shift
		// 2: Minus Shift" (layout:882, :905).
		ShiftOptions: spec.StandardShiftOptions(),
		// MANUAL-EVIDENCED (matrix §2.9): the ordinary three-state legacy
		// domain, no DCS member.
		CTCSSStates: spec.StandardCTCSSStates(),
		// The twelve fields the matrix (§2.14) finds MANUAL-EVIDENCED
		// ABSENT stay nil/empty, same list as ftdx3000's own.
	}
}

// capabilitiesUnverified is the real-radio baseline: every mapped field
// Unverified in both directions where the codec expresses it at all.
func capabilitiesUnverified() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified})
}

// capabilitiesSimulated is the fake-radio-backed profile (CLI --fake, GUI
// demo) and NEVER a real radio.
func capabilitiesSimulated() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Supported, Write: spec.Supported})
}
