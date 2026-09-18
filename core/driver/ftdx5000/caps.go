// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx5000

import (
	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// writeTrialsComplete is THIS driver's hardware write guard, and it is
// FALSE: no FTdx5000 has ever been written to by this project. See
// core/driver/ftdx10/caps.go's own writeTrialsComplete doc comment for
// what flipping this must look like (a two-part change: this constant AND
// a real CapabilitiesRealHardware profile built from trial evidence).
//
// The pin: TestWriteTrialsComplete_PinnedFalse asserts both halves — the
// constant is false, AND a RealHardware driver's baseline is genuinely
// nothing-writable.
const writeTrialsComplete = false

// modelName is the FTdx5000's display name and future driver-registry
// key (matrix's own spelling convention, roadmap's "ftdx5000" package /
// "FTdx5000" registry key).
const modelName = "FTdx5000"

// Profile selects which capability profile New builds the driver with.
// The zero value is RealHardware on purpose: a forgotten or zero-valued
// Profile must fail towards the real-hardware (all-Unverified, nothing
// writable) capability set, never towards the simulator's.
type Profile = driver.Profile

// RealHardware and Simulated are this package's own names for the shared
// profile constants.
const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// memSlots returns the MEM bank's slot inventory, "001".."099", built
// through the dialect's own MemorySlot so the wire forms advertised here
// are the ones ParseSlot actually accepts.
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

// pmsSlots returns the PMS bank's slot inventory, "100".."117" (the
// numeric form — matrix §1.5), built through the dialect's own PMSSlot.
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

// modeDisplayNames returns the selectable mode display names this radio's
// capability data advertises, in wire-code order, derived from the
// dialect rather than transcribed a second time here. cat.ModeUnset is
// excluded: it is a parse-accept-only placeholder no FTdx5000 legend
// prints, and core/cat refuses to emit it in any Set frame.
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

// bankFields builds the per-field support map shared by the MEM and PMS
// banks. Every spec.Field this project models is listed explicitly,
// including the zero ones, so a field left out of this map cannot be
// mistaken for one nobody considered.
//
//   - rw covers the six fields the 27-byte MR/MW record expresses and
//     this driver maps in both directions: frequency, mode, clarifier,
//     CTCSS state, CTCSS tone (see dialect.go/doc.go for why this radio's
//     live P9 is mapped, unlike every registered sibling's fixed one) and
//     shift.
//   - spec.FieldTag and spec.FieldTagDisplay are the zero FieldSupport:
//     NoTag (matrix §0/§3) — this radio has no channel-name/tag route
//     over CAT at all, and no display flag either (no field exists after
//     P10 Shift in the 27-byte record).
//   - spec.FieldScanSkip is the zero FieldSupport: no scan-skip byte
//     exists anywhere in this record.
//   - spec.FieldErase is the zero FieldSupport in both directions: this
//     radio's CAT command set has no erase command (eraseReason,
//     dialect.go).
//
// Each call returns a fresh map, so no two banks share one.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:  rw,
		spec.FieldMode:       rw,
		spec.FieldClarifier:  rw,
		spec.FieldCTCSSState: rw,
		spec.FieldCTCSSTone:  rw,
		spec.FieldShift:      rw,
		// matrix §2's last row: no field exists after P10 Shift, and no
		// TAG/NAME/LABEL command anywhere in the 20-page command set.
		spec.FieldTag:        {},
		spec.FieldTagDisplay: {},
		spec.FieldScanSkip:   {},
		spec.FieldErase:      {},
	}
}

// baseCapabilities assembles the static baseline both profiles share.
//
// ALL THIRTY spec.Capabilities fields are populated explicitly (the
// D-caps-explicit decision; TestCapabilities_EveryFieldExplicit reflects
// over the struct to enforce it). Banks: MEM "001"-"099" and PMS
// "100"-"117" (matrix §1.5/§4), both NoBlank false. NO 5xx/EMG bank and NO
// per-session discovery: this radio has neither class at all (dialect.go,
// doc.go), unlike every other registered Yaesu driver.
func baseCapabilities(fields map[spec.Field]spec.FieldSupport) spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	return spec.Capabilities{
		Model:    modelName,
		CATID:    catID,
		Transmit: spec.HasTransmitter,
		Banks: []spec.Bank{
			{
				ID:      spec.BankMemory,
				Label:   "Memories",
				Slots:   memSlots(),
				NoBlank: false,
				Fields:  fields,
			},
			{
				ID:      spec.BankPMS,
				Label:   "Scan limits (PMS)",
				Slots:   pmsSlots(),
				NoBlank: false,
				Fields:  fields,
			},
		},
		Modes: modeDisplayNames(),
		// matrix §2's last row: NoTag, no field after P10 Shift at all.
		// Cited to the 12/09/2026 nameless-capability rule (merge
		// d0b2498, core/spec/validate.go:253-301), never the old S3
		// triage verdict — matrix §0.
		TagLen: 0,
		NoTag:  true,
		// doc.go's ASSUMED register entry 2, the family's shared entry.
		// CONSULTED from the dialect, not a second literal (write.go
		// reads the same dialect.Clarifier() for the write-side bound).
		ClarMaxHz:  dialect.Clarifier().MaxAbsHz,
		ClarStepHz: dialect.Clarifier().StepHz,
		// The standard 50-tone chart, manual-evidenced (matrix §4:
		// "CTCSS TONE CHART", spot-checked against index 26/44).
		CTCSSTones: tones[:],
		// This radio names a tone by its P9 INDEX into CTCSSTones, so a
		// numeric range would describe a domain it does not have
		// (matrix §4).
		CTCSSToneRange: nil,
		// menu 032 CAT RATE (matrix §1.6): the four rates are
		// manual-evidenced; DefaultBaud is doc.go's ASSUMED register
		// entry 1, cited not re-registered.
		Bauds:       []int{4800, 9600, 19200, 38400},
		DefaultBaud: 38400,
		// doc.go's ASSUMED register entry 3: this manual states no
		// tuning range at all.
		MinFreqHz: 30_000,
		MaxFreqHz: 75_000_000,
		// doc.go's ASSUMED register entry 4: the matrix's own §4 leaves
		// this deliberately unresolved.
		RequiredSlots: nil,
		// P10 "0: Simplex 1: Plus Shift 2: Minus Shift" (matrix §4).
		ShiftOptions: spec.StandardShiftOptions(),
		// P8's three-state domain (matrix §4).
		ToneModes: spec.StandardToneModes(),
		// This radio expresses no Icom-family vocabulary at all (matrix
		// §4's "twelve empty Icom-family fields" — now eleven, since
		// ToneModes is shared and set above).
		DuplexOptions:          nil,
		DTCSPolarities:         nil,
		DTCSCodes:              nil,
		Filters:                nil,
		TuningSteps:            nil,
		ProgramTuningStepRange: nil,
		AttenuatorDB:           nil,
		PreampOptions:          nil,
		AntennaOptions:         nil,
		// No FieldTxFrequency/FieldDuplex is graded on this record — P2 is
		// the only frequency field, no transmit/receive split byte exists
		// — so the question SimplexTx answers does not arise (matrix §4).
		SimplexTx: spec.SimplexTxUnstated,
		// NoTag (above): the pre-Icom default alphabet is the correct
		// reading, matching every other NoTag model's own convention.
		TagCharset: "",
	}
}

// CapabilitiesUnverified is the all-Unverified FAIL-SAFE profile: every
// field the 27-byte MR/MW record expresses is Read Unverified / Write
// Unverified, and every field the record does not express stays the zero
// FieldSupport. See core/driver/ftdx10/caps.go's own CapabilitiesUnverified
// doc comment for the consent transform this profile is subject to
// (unchanged here).
func CapabilitiesUnverified() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	return baseCapabilities(bankFields(rw))
}

// CapabilitiesSimulated is the internal/fakeftdx5000-backed profile (Phase
// 3b, not this phase) and never a real radio: Read AND Write Supported for
// the six fields the 27-byte record can express.
func CapabilitiesSimulated() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	return baseCapabilities(bankFields(rw))
}
