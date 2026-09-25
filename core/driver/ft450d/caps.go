// SPDX-License-Identifier: GPL-3.0-or-later

package ft450d

import (
	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// catDialect is the CAT dialect this driver speaks — the one place this
// package names an instance of its own dialect.go value.
var catDialect = Dialect()

// modelName is the registry key and Capabilities().Model — matrix §2.1: a
// CHOICE spelling of the manual's own name (the OM's title and the CAT
// book's cover both name the radio "FT-450D"; "FT-450" without the D is a
// different real product, never fetched for this matrix).
const modelName = "FT-450D"

// catID is this dialect's own canonical identity, sourced from the dialect
// rather than restated.
var catID = catDialect.CATID()

// writeTrialsComplete is FALSE: NO FT-450D HAS EVER BEEN ASKED ANYTHING BY
// THIS PROJECT, AND NONE IS AVAILABLE TO IT (matrix header). There is no
// CapabilitiesRealHardware profile for this package — RealHardware selects
// the all-Unverified baseline unconditionally (ft450d.go).
const writeTrialsComplete = false

// memBankLabel and pmsBankLabel are this package's own display labels —
// CHOICE, per every sibling driver's identical reasoning.
const (
	memBankLabel = "Memories"
	pmsBankLabel = "Scan limits (PMS)"
)

// standardTones is the 50-tone chart every registered Yaesu dialect shares
// (matrix §1.3/§2.9), and the SAME chart this radio's live P9 tone index
// addresses.
var standardTones = spec.StandardCTCSSTones()

// modeNamesList returns this radio's selectable mode display names in
// wire-code order, DERIVED FROM THE DIALECT rather than transcribed here.
// cat.ModeUnset is excluded: a parse-accept-only placeholder that appears
// in no FT-450D mode legend (matrix §1.2).
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

// memSlots returns the MEM bank's slot inventory, "001".."500" (matrix
// §2.5), built through the dialect's own MemorySlot so the wire forms
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

// pmsSlots returns the PMS bank's slot inventory, "501".."504" — built
// through the dialect's PMSSlot, walking the pair number until the dialect
// refuses one. THE GENERATION IS LOAD-BEARING here exactly as it is on the
// FT-991A/FT-2000/FT-950: under PMSFormNumeric the pair number never
// reaches the wire, so a copy-pasted "P1L".."P2U" literal would produce
// four strings this dialect's own ParseSlot refuses.
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

// memFields builds the MEM bank's per-field support map. ALL TWENTY-SEVEN
// spec.Fields are listed explicitly, including the twenty-one that are the
// zero FieldSupport, following core/driver/ic7200/caps.go's own convention.
//
//   - rw covers the SIX fields the 27-byte MR/MW frame carries: frequency
//     (P2), mode (P6), clarifier (P3-P5), CTCSS state (P8), the LIVE CTCSS
//     tone index (P9, matrix §1.3) and shift (P10) — matrix §3's MEM
//     column.
//   - spec.FieldTag / spec.FieldTagDisplay are the zero FieldSupport:
//     NoTag (matrix §0/§2.7) — this radio HAS a front-panel 7-character
//     tag (OM folio 62), but no CAT route carries tag TEXT (EX036 is a
//     display-mode toggle, not a name field) — the 12/09/2026
//     nameless-capability rule (core/spec/validate.go:253-301), not the old
//     S3 triage verdict.
//   - spec.FieldScanSkip is the zero FieldSupport: the 27-byte record
//     (matrix §1.1) accounts for every byte and none of them is a
//     scan-skip flag; "SC" SCAN is a live receiver behaviour, not a stored
//     per-channel flag.
//   - spec.FieldErase is the zero FieldSupport: no erase/clear command for
//     a memory channel specifically exists in the 90-command index — MC
//     selects a channel, it does not clear one.
//   - the seventeen Icom-tier fields are the zero FieldSupport: this is a
//     Yaesu-family record, and the two vocabularies never coexist
//     (core/spec/capabilities.go:129-133).
//
// Each call returns a fresh map, so no two banks share one.
func memFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:  rw,
		spec.FieldMode:       rw,
		spec.FieldClarifier:  rw,
		spec.FieldCTCSSState: rw,
		spec.FieldCTCSSTone:  rw,
		spec.FieldShift:      rw,

		// NoTag (matrix §0/§2.7): no CAT route that carries tag TEXT.
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

// pmsFields builds the PMS bank's OWN per-field support map — the shape
// this package must NOT share with MEM (matrix §3's own heading, spec.md
// §1's "PMS-vs-MEM write split ... needs its own small map distinguishing
// the two banks' write posture"; unlike ft991a/ft2000, which share ONE map
// across both banks because nothing distinguishes them field by field).
//
// Starts from memFields(rw) — PMS reads exactly the same six fields MEM
// does, over the identical 27-byte record — then forces WRITE to
// Unsupported (never merely Unverified) on every one of those six: the
// roadmap's SAFE SHAPE ruling (radio-roadmap.md:80-87) is "write 001-500
// ONLY ... PMS 501-504 READ-ONLY until an owner probe shows MW 501
// succeeding (then a one-line capability flip)". Matrix §3's own framing:
// this is the CHOICE side of the ruling's caution, not a manual-stated
// restriction — the CAT book's own MW legend prints no ceiling narrower
// than 504. Read stays rw.Read (Unverified on RealHardware, Supported on
// Simulated) exactly like MEM's own read column — only the write column is
// forced, the same shape core/driver/ic7410/caps.go's scanFields takes for
// its own bank-specific write carve-out.
func pmsFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	f := memFields(rw)
	unsupported := spec.FieldSupport{Read: rw.Read, Write: spec.Unsupported}
	f[spec.FieldFrequency] = unsupported
	f[spec.FieldMode] = unsupported
	f[spec.FieldClarifier] = unsupported
	f[spec.FieldCTCSSState] = unsupported
	f[spec.FieldCTCSSTone] = unsupported
	f[spec.FieldShift] = unsupported
	return f
}

// baseCapabilities assembles the static baseline both profiles share, with
// the given per-bank field maps (memFields/pmsFields above — genuinely
// different, matrix §3).
func baseCapabilities(memF, pmsF map[spec.Field]spec.FieldSupport) spec.Capabilities {
	return spec.Capabilities{
		Model: modelName,
		CATID: catID,
		// matrix §2.3: a documented transmit surface — TX, FT (FUNCTION
		// TX), PC (POWER CONTROL), RT (CLAR), all O/O/O/O
		// Set/Read/Answer/AI (layout:164, :152, :132, :156).
		Transmit: spec.HasTransmitter,
		Banks: []spec.Bank{
			{
				ID: spec.BankMemory, Label: memBankLabel,
				Slots: memSlots(), NoBlank: false, Fields: memF,
			},
			{
				// NoBlank: true (matrix §2.5) — OM folio 58 lists PMS pairs
				// among the channel types the radio always carries: they
				// bound a scan range, so an unset pair is meaningless, the
				// same reasoning ft991a's own PMS bank takes.
				ID: spec.BankPMS, Label: pmsBankLabel,
				Slots: pmsSlots(), NoBlank: true, Fields: pmsF,
			},
		},
		Modes: modeNamesList(),
		// matrix §0/§2.7: NoTag, TagLen 0 by declaration — the 12/09/2026
		// nameless-capability rule (merge d0b2498), not the old S3 triage
		// verdict.
		TagLen:     0,
		NoTag:      true,
		ClarMaxHz:  catDialect.Clarifier().MaxAbsHz,
		ClarStepHz: catDialect.Clarifier().StepHz,
		// matrix §1.3/§2.9: the standard 50-tone chart. CTCSSToneRange
		// stays nil — this radio names a tone by chart INDEX, not a raw
		// number.
		CTCSSTones:     standardTones[:],
		CTCSSToneRange: nil,
		// matrix §2.10: menu "010 CATRATE", four rates (layout:401).
		// DefaultBaud is ASSUMED — no factory-default marker exists in this
		// menu chart, the same grounds every registered Yaesu sibling
		// carries.
		Bauds:       []int{4800, 9600, 19200, 38400},
		DefaultBaud: 38400,
		// Phase-2 finding beyond the matrix, which left this pair OPEN
		// (§2.11) pending a direct read of the FA/FB blocks: FA's Set
		// legend prints "P1 30000 - 60000000 (Hz)" (ft450d_layout.txt:555)
		// — the VFO tuning domain read as the memory-storable domain, the
		// same two-part grading (numbers MANUAL-EVIDENCED, the
		// VFO-to-memory step ASSUMED) every sibling in this wave carries.
		MinFreqHz: 30_000,
		MaxFreqHz: 60_000_000,
		// ASSUMED (matrix §2.12): no manual statement that any specific
		// channel must stay populated — the PMS bank's own NoBlank: true
		// already expresses "these four channels are never blank" at the
		// bank level, so naming individual MEM slots here would be an
		// invented constraint.
		RequiredSlots: nil,
		// matrix §2.13: P10 legend "0: Simplex 1: Plus Shift 2: Minus
		// Shift" (layout:801, :626) — the standard three-value vocabulary.
		ShiftOptions: spec.StandardShiftOptions(),
		// matrix §2.14: the ordinary three-state legacy domain, "0: CTCSS
		// OFF 1: CTCSS ENC/DEC 2: CTCSS ENC" — NOT ft991a's five-state
		// DCS-bearing domain.
		ToneModes: spec.StandardToneModes(),
		// The twelve fields matrix §2.15 finds MANUAL-EVIDENCED ABSENT stay
		// nil/empty: DuplexOptions, ToneModes, DTCSPolarities, DTCSCodes,
		// Filters, TuningSteps, ProgramTuningStepRange, AttenuatorDB,
		// PreampOptions, AntennaOptions, TagCharset (empty means "no
		// charset to declare" for a NoTag radio). SimplexTx stays the zero
		// value (SimplexTxUnstated, matrix §2.4): this record has one
		// FreqHz field, no separate TX half, for either FieldTxFrequency or
		// FieldDuplex to answer the question about.
	}
}

// CapabilitiesUnverified is the real-radio baseline: every mapped MEM field
// Unverified in both directions (writeTrialsComplete is false); the PMS
// bank's six shared fields read Unverified but stay write-Unsupported
// regardless (pmsFields — the SAFE SHAPE ruling's caution, independent of
// hardware verification).
func CapabilitiesUnverified() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	return baseCapabilities(memFields(rw), pmsFields(rw))
}

// CapabilitiesSimulated is the fake-radio-backed profile (CLI --fake, GUI
// demo) and NEVER a real radio: MEM's six fields are Read AND Write
// Supported; PMS's six shared fields are Read Supported but stay
// write-Unsupported, the SAME split as CapabilitiesUnverified — the ruling
// governs the codec's own posture, not a hardware-verification state.
func CapabilitiesSimulated() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	return baseCapabilities(memFields(rw), pmsFields(rw))
}

// allFields is every spec.Field this package's bank maps carry an explicit
// entry for — see TestFieldAuditCoversEverySpecField (caps_test.go), which
// checks this against spec.AllFields() two ways. memFields and pmsFields
// share the identical KEY set (only the Write column differs for six of
// them), so either map's keys serve.
var allFields = func() []spec.Field {
	fields := make([]spec.Field, 0, len(memFields(spec.FieldSupport{})))
	for f := range memFields(spec.FieldSupport{}) {
		fields = append(fields, f)
	}
	return fields
}()
