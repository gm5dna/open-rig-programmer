// SPDX-License-Identifier: GPL-3.0-or-later

package ft950

import (
	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// catDialect is the CAT dialect this driver speaks — the one place this
// package names an instance of its own dialect.go value. Everything else
// here derives from it.
var catDialect = Dialect()

// modelName is the registry key and Capabilities().Model — the manual's
// own cover-page name, "FT-950 CAT OPERATION REFERENCE BOOK".
const modelName = "FT-950"

// catID is this dialect's own canonical identity, sourced from the dialect
// rather than restated.
var catID = catDialect.CATID()

// Profile selects which capability profile New builds the driver with. The
// zero value is RealHardware on purpose: a forgotten or zero-valued
// Profile must fail towards the real-hardware capability set, never the
// simulator's.
type Profile = driver.Profile

// RealHardware and Simulated are this package's own names for the shared
// profile constants — an alias and untyped re-declarations, never a fresh
// named type.
const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// writeTrialsComplete is FALSE: no FT-950 has ever been asked anything over
// CAT by this project, and none is available to it (matrix §0). There is
// no CapabilitiesRealHardware profile for this package — RealHardware
// selects the all-Unverified baseline unconditionally (ft950.go).
const writeTrialsComplete = false

// memBankLabel and pmsBankLabel are this package's own display labels —
// CHOICE, per every sibling driver's identical reasoning.
const (
	memBankLabel = "Memories"
	pmsBankLabel = "Scan limits (PMS)"
)

// standardTones is the 50-tone chart every registered Yaesu dialect shares
// (matrix §4), and the SAME chart this radio's live P9 tone index
// addresses (matrix §1.4).
var standardTones = spec.StandardCTCSSTones()

// toneForIndex reports the chart tone for a wire tone-index 0-49, and
// false for anything outside that domain.
func toneForIndex(idx uint8) (spec.Tone, bool) {
	if int(idx) >= len(standardTones) {
		return 0, false
	}
	return standardTones[idx], true
}

// indexForTone is toneForIndex's write-direction inverse: a linear scan
// over fifty entries, cheap enough to run once per write.
func indexForTone(t spec.Tone) (uint8, bool) {
	for i, v := range standardTones {
		if v == t {
			return uint8(i), true
		}
	}
	return 0, false
}

// modeNamesList returns this radio's selectable mode display names in
// wire-code order, DERIVED FROM THE DIALECT rather than transcribed here.
// cat.ModeUnset is excluded: a parse-accept-only placeholder that appears
// in no FT-950 mode legend (matrix §1.3).
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

// memSlots returns the MEM bank's slot inventory, "000".."099" — starting
// at ZERO, not one, this radio's own delta (matrix §1.5, doc.go entry 4).
// Built through the dialect's own MemorySlot so the wire forms advertised
// are exactly the ones this dialect's own ParseSlot accepts.
func memSlots() []string {
	var slots []string
	for n := 0; ; n++ {
		s, err := catDialect.MemorySlot(n)
		if err != nil {
			return slots
		}
		slots = append(slots, s.Wire())
	}
}

// pmsSlots returns the PMS bank's slot inventory, "100".."117" — built
// through the dialect's PMSSlot, walking the pair number until the dialect
// refuses one. THE GENERATION IS LOAD-BEARING here exactly as it is on the
// FT-991A/FT-2000/FTdx9000: under PMSFormNumeric the pair number never
// reaches the wire, so a copy-pasted "P1L".."P9U" literal would produce
// eighteen strings this dialect's own ParseSlot refuses.
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
// per-bank qualifier, so one product serves both.
//
// ALL TWENTY-SEVEN spec.Fields are listed explicitly — see
// TestFieldAuditCoversEverySpecField, which enforces this.
//
//   - rw covers the SIX fields this record maps: frequency (P2), mode
//     (P6), clarifier (P3-P5, folding sign/magnitude/RxClar/TxClar), CTCSS
//     state (P8), the LIVE CTCSS tone index (P9 — matrix §1.4, Lift Y's
//     ToneIndex axis; graded here rather than left zero because it
//     genuinely round-trips through the same standard chart every
//     registered dialect shares, and the matrix explicitly leaves the
//     grading choice to the driver — doc.go entry 1), and shift (P10).
//   - spec.FieldTag / spec.FieldTagDisplay are the zero FieldSupport:
//     NoTag (matrix §0/§3) — this radio has no channel-name route over CAT
//     at all, per the 12/09/2026 nameless-capability rule
//     (core/spec/validate.go:253-301), not the old S3 triage verdict.
//   - spec.FieldScanSkip is the zero FieldSupport: the 27-byte record
//     (matrix §2) accounts for every byte and none of them is a scan-skip
//     flag; "SC" SCAN is a live receiver behaviour, not a stored
//     per-channel flag.
//   - spec.FieldErase is the zero FieldSupport: no erase/clear command for
//     a memory channel specifically exists in the 90-command index (matrix
//     §2's own review) — MC selects a channel, it does not clear one.
//   - the seventeen Icom-tier fields are the zero FieldSupport: this is a
//     Yaesu-family record, and the two vocabularies never coexist
//     (core/spec/capabilities.go:129-133).
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

		// NoTag (matrix §0/§3): no name route over CAT at all.
		spec.FieldTag:        {},
		spec.FieldTagDisplay: {},

		spec.FieldScanSkip: {},
		spec.FieldErase:    {},

		// The Icom-family vocabularies, absent from this Yaesu record.
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

// baseCapabilities assembles the static baseline both profiles share, with
// the given per-bank field map (shared by MEM and PMS — matrix §2).
//
// RequiredSlots is left EMPTY (doc.go entry 6): matrix §4 explicitly
// declines to resolve whether this radio needs one at all, only what its
// slot string would be IF one were carried ("000", not the fleet's usual
// "001") — following FT-991A/FT-2000/FTdx9000's identical choice in this
// wave, a wrong guess refuses real candidates rather than merely
// describing one.
func baseCapabilities(memFields, pmsFields map[spec.Field]spec.FieldSupport) spec.Capabilities {
	return spec.Capabilities{
		Model: modelName,
		CATID: catID,
		// matrix §4: a documented transmit surface — TX, MX (MOX SET), VX
		// (VOX), XT (TX CLAR), all present in the Control Command List
		// (layout:161-181).
		Transmit: spec.HasTransmitter,
		Banks: []spec.Bank{
			{ID: spec.BankMemory, Label: memBankLabel, Slots: memSlots(), NoBlank: false, Fields: memFields},
			{ID: spec.BankPMS, Label: pmsBankLabel, Slots: pmsSlots(), NoBlank: false, Fields: pmsFields},
		},
		Modes: modeNamesList(),
		// matrix §0/§3: NoTag, TagLen 0 by declaration — the 12/09/2026
		// nameless-capability rule (merge d0b2498), not the old S3 triage
		// verdict.
		TagLen:     0,
		NoTag:      true,
		ClarMaxHz:  catDialect.Clarifier().MaxAbsHz,
		ClarStepHz: catDialect.Clarifier().StepHz,
		// matrix §1.4/§4: the standard 50-tone chart. CTCSSToneRange stays
		// nil — this radio names a tone by chart INDEX, not a raw number.
		CTCSSTones:     standardTones[:],
		CTCSSToneRange: nil,
		// matrix §1.6: menu row "026 CAT BAUD RATE" (layout:476), four
		// rates matching the family's already-registered Bauds list.
		// DefaultBaud is ASSUMED, same grounds as every registered
		// sibling: the table prints no factory-default column.
		Bauds:       []int{4800, 9600, 19200, 38400},
		DefaultBaud: 38400,
		// doc.go entry 5: a Phase-3 finding beyond the matrix, which is
		// silent on this pair entirely. FA's/FB's own Set legends both
		// print "0030000 - 56000000 (Hz)" (layout:606, :618) — the VFO
		// tuning domain read as the memory-storable domain, the same
		// two-part grading every sibling in this wave carries.
		MinFreqHz: 30_000,
		MaxFreqHz: 56_000_000,
		// RequiredSlots: left nil — see this function's own doc comment.
		// matrix §4: P10's "0: Simplex 1: Plus Shift 2: Minus Shift"
		// (layout:857, :898) — the standard three-value vocabulary.
		ShiftOptions: spec.StandardShiftOptions(),
		// matrix §4: the ordinary three-state legacy domain, "0: CTCSS OFF
		// 1: CTCSS ENC/DEC 2: CTCSS ENC" — no DCS member.
		ToneModes: spec.StandardToneModes(),
		// The twelve fields the matrix (§4) finds MANUAL-EVIDENCED ABSENT
		// stay nil/empty: DuplexOptions, ToneModes, DTCSPolarities,
		// DTCSCodes, Filters, TuningSteps, ProgramTuningStepRange,
		// AttenuatorDB, PreampOptions, AntennaOptions, TagCharset (empty
		// means "no charset to declare" for a NoTag radio). SimplexTx stays
		// the zero value (SimplexTxUnstated): this record has no
		// TxFrequency field for either FieldTxFrequency or FieldDuplex to
		// answer the question about (matrix §4).
	}
}

// CapabilitiesUnverified is the real-radio baseline: every mapped field
// Unverified in both directions, since writeTrialsComplete is false.
func CapabilitiesUnverified() spec.Capabilities {
	return baseCapabilities(bankFields(spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}), bankFields(spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}))
}

// CapabilitiesSimulated is the fake-radio-backed profile (CLI --fake, GUI
// demo) and NEVER a real radio: Read AND Write Supported for the six
// fields this frame expresses, on MEM and PMS alike.
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
