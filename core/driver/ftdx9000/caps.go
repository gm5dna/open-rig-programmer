// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx9000

import (
	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// catDialect is the CAT dialect this driver speaks — the one place this
// package names an instance of its own dialect.go value. Everything else
// here derives from it.
var catDialect = Dialect()

// modelName is the registry key and Capabilities().Model. The manual never
// spells "FT-9000" or "FTdx9000" as such: it prints "FTDX9000D/Contest/MP"
// (ID legend) and "FT DX 9000" (spaced, running headers) — matrix §1.1.
// "FTdx9000" is this project's own registry spelling (spec.md §0/§1,
// settled answer 5); "FT-9000" is a radiotext-only alias for Phase 4,
// never cited here as a manual string.
const modelName = "FTdx9000"

// catID is this dialect's own canonical identity, sourced from the dialect
// rather than restated — see dialect.go's CATID comment for why it is
// "0101" and not one of its two siblings.
var catID = catDialect.CATID()

// acceptedCATIDs are every ID; answer this ONE registered row accepts —
// matrix §1.2: the manual's own ID legend names three sub-variants
// (FTDX9000D "0101", FTDX9000Contest "0102", FTDX9000MP "0103") that this
// project registers as a single row. Which of the three (or whether all)
// an ID probe should accept is left open by the matrix; accepting all
// three here is this package's own decision (ftdx9000.go's handshake).
var acceptedCATIDs = []string{"0101", "0102", "0103"}

// Profile selects which capability profile New builds the driver with. The
// zero value is RealHardware on purpose (matrix §2.1's sibling reasoning):
// a forgotten or zero-valued Profile must fail towards the real-hardware
// capability set, never the simulator's.
type Profile = driver.Profile

// RealHardware and Simulated are this package's own names for the shared
// profile constants — an alias and untyped re-declarations, never a fresh
// named type (see driver.Profile's own doc comment for why).
const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// writeTrialsComplete is FALSE: no FTdx9000 has ever been asked anything
// over CAT by this project. See the sibling drivers' identical constant
// for the two-part flip a future hardware trial would need.
const writeTrialsComplete = false

// memBankLabel and pmsBankLabel are this package's own display labels —
// CHOICE, per every sibling driver's identical reasoning.
const (
	memBankLabel = "Memories"
	pmsBankLabel = "Scan limits (PMS)"
)

// standardTones is the 50-tone chart every registered Yaesu dialect shares
// (matrix §1.8), and the SAME chart this radio's live P9 tone index
// addresses (matrix §1.10).
var standardTones = spec.StandardCTCSSTones()

// toneForIndex reports the chart tone for a wire tone-index 0-49, and false
// for anything outside that domain — see mode.go's Lift-Y comment: the
// array index IS the CAT tone number.
func toneForIndex(idx uint8) (spec.Tone, bool) {
	if int(idx) >= len(standardTones) {
		return 0, false
	}
	return standardTones[idx], true
}

// indexForTone is toneForIndex's write-direction inverse: a linear scan
// over fifty entries, cheap enough to run once per write and simpler than
// maintaining a second, invertible map by hand.
func indexForTone(t spec.Tone) (uint8, bool) {
	for i, v := range standardTones {
		if v == t {
			return uint8(i), true
		}
	}
	return 0, false
}

// modeNamesList returns this radio's selectable mode display names in
// wire-code order, DERIVED FROM THE DIALECT rather than transcribed here —
// see the sibling drivers' identical reasoning (ft891/ft991a caps.go).
// cat.ModeUnset is excluded: a parse-accept-only placeholder that appears
// in no FTdx9000 mode legend (matrix §1.5).
func modeNamesList() []string {
	var names []string
	for b := 0; b <= 0xFF; b++ {
		m := cat.Mode(byte(b))
		if m == cat.ModeUnset || !catDialect.ValidMode(m) {
			continue
		}
		names = append(names, catDialect.ModeName(m))
	}
	return names
}

// memSlots and pmsSlots return this radio's two static bank inventories,
// built through the DIALECT's own MemorySlot/PMSSlot so the wire forms
// advertised are exactly the ones this dialect's own ParseSlot accepts.
func memSlots() []string {
	var slots []string
	for n := 1; ; n++ {
		s, err := catDialect.MemorySlot(n)
		if err != nil {
			return slots
		}
		slots = append(slots, s.Wire())
	}
}

func pmsSlots() []string {
	var slots []string
	for pair := 1; ; pair++ {
		lower, err := catDialect.PMSSlot(pair, false)
		if err != nil {
			return slots
		}
		upper, err := catDialect.PMSSlot(pair, true)
		if err != nil {
			return slots
		}
		slots = append(slots, lower.Wire(), upper.Wire())
	}
}

// bankFields builds the per-field support map shared by the MEM and PMS
// banks: matrix §2's 27-byte record is printed once and carries no
// per-bank qualifier, so one product serves both (ft891's own precedent).
//
// ALL TWENTY-SEVEN spec.Fields are listed explicitly — see
// TestFieldAuditCoversEverySpecField, which enforces this.
//
//   - rw covers the SIX fields this record maps: frequency (P2), mode
//     (P6), clarifier (P3-P5, folding sign/magnitude/RxClar/TxClar), CTCSS
//     state (P8), the LIVE CTCSS tone index (P9 — matrix §1.10, Lift Y's
//     ToneIndex axis; graded here rather than left zero because it
//     genuinely round-trips through the same standard chart every
//     registered dialect shares, and this matrix explicitly leaves the
//     grading choice to the driver, see spec.md §2), and shift (P10).
//   - spec.FieldTag / spec.FieldTagDisplay are the zero FieldSupport:
//     NoTag (matrix §0/§1.6) — this radio has no channel-name route over
//     CAT at all, per the 12/09/2026 nameless-capability rule
//     (core/spec/validate.go:253-301), not the old S3 triage verdict.
//   - spec.FieldScanSkip is the zero FieldSupport: the 27-byte record
//     (matrix §2) accounts for every byte and none of them is a scan-skip
//     flag.
//   - spec.FieldErase is the zero FieldSupport in both directions: spec.md
//     §3 states no create/erase for any of the twelve rows in this wave.
//   - the seventeen Icom-tier fields are the zero FieldSupport as
//     MANUAL-EVIDENCED ABSENCES (matrix §1.17's table).
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:  rw,
		spec.FieldMode:       rw,
		spec.FieldClarifier:  rw,
		spec.FieldCTCSSState: rw,
		spec.FieldCTCSSTone:  rw,
		spec.FieldShift:      rw,

		spec.FieldTag:        {},
		spec.FieldTagDisplay: {},
		spec.FieldScanSkip:   {},
		spec.FieldErase:      {},

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

// baseCapabilities assembles the static baseline both profiles share.
//
// RequiredSlots is left EMPTY — matrix §1.14 is an open note, not a pin,
// and following the FT-991A's own counter-argument (§1.15/§2.5 there): a
// wrong guess here refuses real candidates rather than merely describing
// one, so this is a driver-time CHOICE against pre-baking a value.
func baseCapabilities(memFields, pmsFields map[spec.Field]spec.FieldSupport) spec.Capabilities {
	return spec.Capabilities{
		Model: modelName,
		CATID: catID,
		// matrix §1.3: a documented transmit surface (MX/MOX/TX etc).
		Transmit: spec.HasTransmitter,
		Banks: []spec.Bank{
			{ID: spec.BankMemory, Label: memBankLabel, Slots: memSlots(), NoBlank: false, Fields: memFields},
			{ID: spec.BankPMS, Label: pmsBankLabel, Slots: pmsSlots(), NoBlank: false, Fields: pmsFields},
		},
		Modes: modeNamesList(),
		// matrix §1.6/§0: NoTag, TagLen 0 by declaration.
		TagLen:     0,
		NoTag:      true,
		ClarMaxHz:  catDialect.Clarifier().MaxAbsHz,
		ClarStepHz: catDialect.Clarifier().StepHz,
		// matrix §1.8: the standard 50-tone chart.
		CTCSSTones: standardTones[:],
		// matrix §1.9: nil — this radio names a tone by chart INDEX, not a
		// raw number.
		CTCSSToneRange: nil,
		// matrix §1.11/§1.12: four rates, DefaultBaud ASSUMED (menu 033's
		// Digits column, not a stated default — the same trap FT-991A's
		// own matrix records for its menu 031).
		Bauds:       []int{4800, 9600, 19200, 38400},
		DefaultBaud: 38400,
		// matrix §1.12/§1.13: FA/FB's own printed range, ASSUMED for the
		// VFO-to-memory-domain step (same two-part grading as FT-991A).
		MinFreqHz: 30_000,
		MaxFreqHz: 60_000_000,
		// matrix §1.15: three-value family vocabulary.
		ShiftOptions: spec.StandardShiftOptions(),
		// matrix §1.16: the family THREE (OFF/ENC-DEC/ENC), not FT-991A's
		// five — no DCS member.
		CTCSSStates: spec.StandardCTCSSStates(),
	}
}

// CapabilitiesUnverified is the all-Unverified fail-safe profile — what a
// RealHardware session gets while writeTrialsComplete is false, and what
// any unrecognised Profile value selects too.
func CapabilitiesUnverified() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	return baseCapabilities(bankFields(rw), bankFields(rw))
}

// CapabilitiesSimulated is the internal/fakeftdx9000-backed profile (a
// later phase's fake), never a real radio: Read and Write Supported for
// every field this record maps.
func CapabilitiesSimulated() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	return baseCapabilities(bankFields(rw), bankFields(rw))
}

// allFields is every spec.Field this package's bank maps carry an explicit
// entry for — see TestFieldAuditCoversEverySpecField (caps_test.go), which
// checks this against spec.AllFields() two ways.
var allFields = func() []spec.Field {
	fields := make([]spec.Field, 0, len(bankFields(spec.FieldSupport{})))
	for f := range bankFields(spec.FieldSupport{}) {
		fields = append(fields, f)
	}
	return fields
}()
