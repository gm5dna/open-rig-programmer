// SPDX-License-Identifier: GPL-3.0-or-later

package ft2000

import (
	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// writeTrialsComplete is FALSE. NO FT-2000 OR FT-2000D HAS EVER BEEN ASKED
// ANYTHING BY THIS PROJECT (matrix, header note): every byte here came
// from the FT-2000 SERIES CAT manual through the reviewed capability
// matrix, and from nothing else. There is therefore no
// CapabilitiesRealHardware profile for this package — RealHardware
// selects the all-Unverified baseline unconditionally (ft2000.go).
const writeTrialsComplete = false

// modeNames returns the selectable mode display names this model's
// capability data advertises, in wire-code order, DERIVED FROM THE
// DIALECT rather than transcribed here — matrix §1.2/§2.5. cat.ModeUnset
// is excluded: it is a parse-accept-only placeholder present in no
// FT-2000 mode legend, and offering it as a selectable mode would invite
// a write core/cat refuses to build.
func modeDisplayNames(d cat.Dialect) []string {
	var names []string
	for b := 0; b <= 0xFF; b++ {
		m := cat.Mode(byte(b))
		if m == cat.ModeUnset || !d.ValidMode(m) {
			continue
		}
		names = append(names, d.ModeName(m))
	}
	return names
}

// memSlots returns the MEM bank's slot inventory, "001".."099", built
// through the dialect's own MemorySlot (matrix §2.4).
func memSlots(d cat.Dialect) []string {
	var slots []string
	for n := 1; ; n++ {
		s, err := d.MemorySlot(n)
		if err != nil {
			return slots
		}
		slots = append(slots, s.Wire())
	}
}

// pmsSlots returns the PMS bank's slot inventory, "100".."117", built
// through the dialect's PMSSlot — walking the pair number until the
// dialect refuses one. THE GENERATION IS LOAD-BEARING here exactly as it
// is on the FT-991A (core/driver/ft991a/caps.go's own doc comment): under
// PMSFormNumeric the pair number never reaches the wire, so a copy-pasted
// "P1L".."P9U" literal would produce eighteen strings this dialect's own
// ParseSlot refuses.
func pmsSlots(d cat.Dialect) []string {
	var slots []string
	for pair := 1; ; pair++ {
		lower, err := d.PMSSlot(pair, false)
		if err != nil {
			return slots
		}
		upper, err := d.PMSSlot(pair, true)
		if err != nil {
			return slots
		}
		slots = append(slots, lower.Wire(), upper.Wire())
	}
}

// bankFields builds the per-field support map shared by the MEM and PMS
// banks (matrix §3: one field grid governs both — nothing in this manual
// distinguishes them field by field).
//
// ALL TWENTY-SEVEN spec.Fields are listed explicitly, including the
// twenty-one that are the zero FieldSupport, following
// core/driver/ic7200/caps.go's own convention: an absent key and an
// explicit zero FieldSupport mean the same thing to
// spec.Capabilities.FieldSupport, but only the written-down zero is
// legible as a decision (which is also what lets this package's
// TestFieldAuditCoversEverySpecField treat every one of the 27 as
// "audited" rather than needing a separate deliberately-unexpressed
// reason per field).
//
//   - rw covers the FIVE fields the bare MW/MR frame always carries:
//     frequency (P2), mode (P6), clarifier (P3/P4/P5), CTCSS state (P8)
//     and shift (P10) — matrix §3.
//   - FieldCTCSSTone is ALSO rw, a Phase-3 decision the matrix left OPEN
//     (§1.3/§2.9/§3): P9 is a live two-digit index into the standard
//     50-entry CTCSS tone chart, round-tripped by cat.MemoryData.ToneIndex
//     under MemoryP9Policy=P9ToneIndex (lift Y) — the first memory record
//     in this fleet to carry a live tone index at all, unlike every
//     registered sibling's printed-fixed "00". The codec fully expresses
//     it in both directions, so this driver grades it alongside the other
//     five mapped fields rather than leaving it Unsupported.
//   - FieldTag and FieldTagDisplay are the zero FieldSupport: this family
//     has no tag/name route over CAT at all (matrix §0/§2.6, the NoTag
//     rule, 12/09/2026 nameless-capability merge d0b2498). NoTag: true
//     below is what core/spec/validate.go's NoTag pairing rule requires
//     alongside this.
//   - FieldScanSkip is the zero FieldSupport: no scan-skip byte exists in
//     the 27-byte record (matrix §3) — "SC" SCAN is a live receiver
//     behaviour, not a stored per-channel flag.
//   - FieldErase is the zero FieldSupport: no erase/clear command was
//     found in the 90-command index for a memory channel specifically
//     (matrix §3) — MC selects a channel, it does not clear one.
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

		// NoTag (matrix §0/§2.6): no name route over CAT at all.
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

// baseCapabilities assembles the static baseline both profiles share for
// model m, with the given per-bank field map (shared by MEM and PMS —
// matrix §3).
func baseCapabilities(m modelParams, rw spec.FieldSupport) spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	return spec.Capabilities{
		Model: m.name,
		CATID: m.dialect.CATID(),
		// MANUAL-EVIDENCED (matrix §2.3): a documented transmit surface —
		// TX, MX (MOX SET), VX (VOX), PR (SPEECH PROCESSOR), XT (TX CLAR),
		// all present with Set/Read/Answer/AI availability (layout:164 and
		// neighbouring rows).
		Transmit: spec.HasTransmitter,
		Banks: []spec.Bank{
			{
				// A fresh map per bank (bankFields' own doc comment): MEM
				// and PMS must not share one map instance.
				ID: spec.BankMemory, Label: "Memories",
				Slots: memSlots(m.dialect), NoBlank: false, Fields: bankFields(rw),
			},
			{
				ID: spec.BankPMS, Label: "Scan limits (PMS)",
				Slots: pmsSlots(m.dialect), NoBlank: false, Fields: bankFields(rw),
			},
		},
		Modes: modeDisplayNames(m.dialect),
		// NoTag (matrix §0/§2.6): MANUAL-EVIDENCED ABSENCE — a
		// whole-document grep for tag/name/label finds nothing but an
		// unrelated connector-pin legend, and the 90-command index carries
		// no naming command. Cited to the 12/09/2026 nameless-capability
		// rule (merge d0b2498, core/spec/validate.go:253-301), not the
		// old S3 triage verdict, per the wave's NoTag citation rule.
		TagLen: 0,
		NoTag:  true,
		// ASSUMED (matrix §2.7): the Yaesu-family default every registered
		// sibling carries; this manual states no step. MaxAbsHz is 9990,
		// not the printed range's own 9999 — see dialect.go's Clarifier
		// comment for why the codec's own multiple-of-step rule forces
		// this down from the matrix's literal reading.
		ClarMaxHz:  m.dialect.Clarifier().MaxAbsHz,
		ClarStepHz: m.dialect.Clarifier().StepHz,
		// The standard 50-tone chart (matrix §1.3/§2.8): P9 and CN's P2
		// are both explicit table INDEXES, never a frequency number, so
		// CTCSSToneRange stays nil.
		CTCSSTones: tones[:],
		// MANUAL-EVIDENCED (matrix §2.10): Table 1 row "028 CAT BAUD RATE"
		// legend, layout:480. DefaultBaud is ASSUMED (matrix §2.10): no
		// factory-default column exists in this manual; 38400 is what
		// every registered Yaesu sibling carries on the same ground.
		Bauds:       []int{4800, 9600, 19200, 38400},
		DefaultBaud: 38400,
		// MANUAL-EVIDENCED (Phase-3 finding beyond the matrix, which left
		// this range explicitly open pending a re-read of the FA/FB detail
		// blocks): "P1 00030000 - 60000000 (Hz)", printed identically on
		// FA's and FB's own Set legends (ft2000_layout.txt:654, :666) —
		// this driver reads them so the matrix's §2.11 OPEN item does not
		// have to stay open.
		MinFreqHz: 30_000,
		MaxFreqHz: 60_000_000,
		// ASSUMED (matrix §2.12): nothing in the MC/MR/MW legends states
		// that any channel must stay populated; left empty rather than
		// guessed populated.
		RequiredSlots: nil,
		// MANUAL-EVIDENCED (matrix §2.13): P10 legend "0: Simplex 1: Plus
		// Shift 2: Minus Shift" (layout:936, :1005) — the standard
		// three-value vocabulary.
		ShiftOptions: spec.StandardShiftOptions(),
		// MANUAL-EVIDENCED (matrix §2.9): the ordinary three-state legacy
		// domain, "0: CTCSS OFF 1: CTCSS ENC/DEC 2: CTCSS ENC" — NOT the
		// FT-991A's five-state DCS-bearing domain.
		ToneModes: spec.StandardToneModes(),
		// The twelve fields the matrix (§2.14) finds MANUAL-EVIDENCED
		// ABSENT stay nil/empty: DuplexOptions, ToneModes, DTCSPolarities,
		// DTCSCodes, Filters, TuningSteps, ProgramTuningStepRange,
		// AttenuatorDB, PreampOptions, AntennaOptions, TagCharset (empty
		// means "no charset to declare" for a NoTag radio). SimplexTx
		// stays the zero value (SimplexTxUnstated): this record has no
		// TxFrequency field for either FieldTxFrequency or FieldDuplex to
		// answer the question about.
	}
}

// CapabilitiesUnverified is the real-radio baseline for model m: every
// mapped field Unverified in both directions, since writeTrialsComplete
// is false.
func capabilitiesUnverified(m modelParams) spec.Capabilities {
	return baseCapabilities(m, spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified})
}

// CapabilitiesSimulated is the fake-radio-backed profile (CLI --fake, GUI
// demo) and NEVER a real radio: Read AND Write Supported for the six
// fields this frame expresses (frequency, mode, clarifier, CTCSS state,
// CTCSS tone and shift), on MEM and PMS alike.
func capabilitiesSimulated(m modelParams) spec.Capabilities {
	return baseCapabilities(m, spec.FieldSupport{Read: spec.Supported, Write: spec.Supported})
}
