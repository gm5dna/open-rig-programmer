// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx3000

import (
	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// writeTrialsComplete is FALSE. NO FTdx3000 HAS EVER BEEN ASKED ANYTHING
// BY THIS PROJECT (matrix header note): every byte here came from the
// FTdx3000 CAT manual (revision 2006-D) through the reviewed capability
// matrix, and from nothing else.
const writeTrialsComplete = false

// modeDisplayNames returns the selectable mode display names, in wire-code
// order, DERIVED FROM THE DIALECT — matrix §1.2/§2.5. cat.ModeUnset is
// excluded: an accept-only placeholder present in no MW/MR/MD legend.
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

// memSlots returns the MEM bank's slot inventory, "001".."099".
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

// pmsSlots returns the PMS bank's slot inventory, "100".."117", walking
// the pair number until the dialect refuses one — LOAD-BEARING under
// PMSFormNumeric, where the pair number never reaches the wire (matches
// core/driver/ft2000/caps.go's own reasoning).
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
//     frequency (P2), mode (P6), clarifier (P3/P4/P5), CTCSS state (P8)
//     and shift (P10).
//   - FieldCTCSSTone is `{Read: rw.Read, Write: spec.Unsupported}` —
//     matrix §1.3/§3's headline wrinkle: P9 is a live tone-table index on
//     READ (round-tripped by cat.MemoryData.ToneIndex under
//     cat.P9ToneIndexReadOnly, this wave's own core/cat lift) but
//     printed-fixed on WRITE, unconditionally, regardless of profile —
//     the ic7410/caps.go `scanFields` shape for a field whose write side
//     is a structural ceiling, not a profile-dependent one.
//   - FieldTag/FieldTagDisplay are the zero FieldSupport: NoTag (matrix
//     §0, the 12/09/2026 nameless-capability rule, merge d0b2498) — no
//     channel-name route over CAT at all.
//   - FieldScanSkip/FieldErase are the zero FieldSupport: no scan-skip
//     byte in the 27-byte record, no erase/clear command for a memory
//     channel specifically (matrix §3).
//   - the seventeen Icom-tier fields are the zero FieldSupport: this is a
//     Yaesu-family record, and the two vocabularies never coexist
//     (core/spec/capabilities.go:129-133).
//
// Each call returns a fresh map, so MEM and PMS never share one.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:  rw,
		spec.FieldMode:       rw,
		spec.FieldClarifier:  rw,
		spec.FieldCTCSSState: rw,
		spec.FieldCTCSSTone:  {Read: rw.Read, Write: spec.Unsupported},
		spec.FieldShift:      rw,

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
// the given per-bank field map (shared by MEM and PMS — matrix §3).
func baseCapabilities(rw spec.FieldSupport) spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	return spec.Capabilities{
		Model: modelName,
		CATID: dialect.CATID(),
		// MANUAL-EVIDENCED (matrix §2.3): a documented transmit surface
		// (MX MOX SET, layout:965-969, and the rest of the transmit
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
		// NoTag (matrix §0/§2.6): MANUAL-EVIDENCED ABSENCE — a
		// whole-document grep for tag/name/label finds one irrelevant hit
		// (a connector-pin legend) and nothing else. Cited to the
		// 12/09/2026 nameless-capability rule (merge d0b2498,
		// core/spec/validate.go:253-301).
		TagLen: 0,
		NoTag:  true,
		// ASSUMED (matrix §2.7): the Yaesu-family default; this manual
		// states no step. MaxAbsHz is 9990, not the matrix's literal 9999
		// — see dialect.go's Clarifier comment.
		ClarMaxHz:  dialect.Clarifier().MaxAbsHz,
		ClarStepHz: dialect.Clarifier().StepHz,
		// The standard 50-tone chart (matrix §2.8): P9 is a table INDEX,
		// never a frequency, so CTCSSToneRange stays nil.
		CTCSSTones: tones[:],
		// MANUAL-EVIDENCED (matrix §2.10): menu 038 GENERAL CAT RATE,
		// layout:492. DefaultBaud ASSUMED (matrix §2.10): no
		// factory-default column in this manual; 38400 is what every
		// registered Yaesu sibling carries on the same ground.
		Bauds:       []int{4800, 9600, 19200, 38400},
		DefaultBaud: 38400,
		// MANUAL-EVIDENCED (matrix §2.11): FA/FB Set legends both print
		// "P1 0030000 - 60000000 (Hz)" (layout:704, :715).
		MinFreqHz: 30_000,
		MaxFreqHz: 60_000_000,
		// ASSUMED (matrix §2.12): no manual statement that any channel
		// must stay populated.
		RequiredSlots: nil,
		// MANUAL-EVIDENCED (matrix §2.13): P10/P11 legend "0: Simplex
		// 1: Plus Shift 2: Minus Shift".
		ShiftOptions: spec.StandardShiftOptions(),
		// MANUAL-EVIDENCED (matrix §2.9): the ordinary three-state legacy
		// domain, no DCS member.
		CTCSSStates: spec.StandardCTCSSStates(),
		// The twelve fields the matrix (§2.14) finds MANUAL-EVIDENCED
		// ABSENT stay nil/empty: DuplexOptions, ToneModes, DTCSPolarities,
		// DTCSCodes, Filters, TuningSteps, ProgramTuningStepRange,
		// AttenuatorDB, PreampOptions, AntennaOptions, TagCharset (NoTag —
		// no charset to declare). SimplexTx stays the zero value: no
		// TxFrequency field for either FieldTxFrequency or FieldDuplex to
		// answer the question about.
	}
}

// capabilitiesUnverified is the real-radio baseline: every mapped field
// Unverified in both directions where the codec expresses it at all,
// since writeTrialsComplete is false. FieldCTCSSTone's write side stays
// Unsupported regardless — a structural ceiling, not a profile.
func capabilitiesUnverified() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified})
}

// capabilitiesSimulated is the fake-radio-backed profile (CLI --fake, GUI
// demo) and NEVER a real radio: Read AND Write Supported for the fields
// this frame expresses — except FieldCTCSSTone's write side, still
// Unsupported (bankFields' own doc comment: not a profile-dependent
// ceiling).
func capabilitiesSimulated() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Supported, Write: spec.Supported})
}
