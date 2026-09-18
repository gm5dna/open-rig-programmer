// SPDX-License-Identifier: GPL-3.0-or-later

package ftx1

import (
	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// modelName is this package's registry-key spelling. No body suffix
// ("field"/"optima"): the wire carries no body-distinguishing byte at all
// (dialect.go's own CATID comment, spec.md §1), so this driver names the
// PRODUCT LINE, not a specific body.
const modelName = "FTX-1"

// writeTrialsComplete is FALSE. NO FTX-1 HAS EVER BEEN ASKED ANYTHING BY
// THIS PROJECT — no hardware exists to ask, and no owner has run a write
// trial (spec.md's own header note, "no-additional-hardware-2026-09-13").
const writeTrialsComplete = false

// catID is the identity an FTX-1 answers "ID;" with, sourced from the
// dialect rather than restated here.
var catID = dialect.CATID()

// modeDisplayNames returns the selectable mode display names, in wire-code
// order, DERIVED FROM THE DIALECT — the sixteen FT-710-shared names plus
// C4FM-DN/C4FM-VW (dialect.go's modeNames). cat.ModeUnset is excluded
// (accept-only placeholder, never a selectable mode); 'G'/'J' are simply
// not in the dialect's ModeNames map, so they never appear here either.
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

// memSlots, pmsSlots, sixtyMSlots and emgSlots build each bank's slot
// inventory FROM THE DIALECT's own builders (MemorySlot/PMSSlot/
// SixtyMSlot/EMGSlot) rather than from a raw numeric range written out
// here a second time — the same derivation ftdx1200/ftdx3000's own
// caps.go use, so a slot-space edit in dialect.go cannot leave this list
// silently stale.
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

// sixtyMSlots builds the "5 MHz BAND" bank's slot inventory, "50001"
// through "50020" (spec.md §3.1/§10) — the SAME field (SlotSpace.
// SixtyLo/SixtyHi) every other registered dialect's 60m bank uses, just
// five digits wide.
//
// UNLIKE THE FT-710/FTDX10 FAMILY, this bank is STATIC here, not
// DISCOVERED at Open time: the manual documents it as part of every P1
// domain uniformly (MR/MW/IF/OI all print it, spec.md §3.1) alongside
// memory and PMS, with no hint of the FT-710's region-dependent
// population — so ftx1.go's Open runs no discovery sweep, and this bank
// is declared in every profile exactly like MEM and PMS are.
func sixtyMSlots() []string {
	var slots []string
	for n := 1; ; n++ {
		s, err := dialect.SixtyMSlot(n)
		if err != nil {
			return slots
		}
		slots = append(slots, s.Wire())
	}
}

// emgSlots builds the EMGCH bank's one-slot inventory. Static for
// sixtyMSlots' own reason.
func emgSlots() []string {
	s := dialect.EMGSlot()
	if s.Wire() == "" {
		return nil
	}
	return []string{s.Wire()}
}

// bankFields builds the per-field support map shared by every bank: rw
// covers the SIX fields the shared MR/MW/MT record always carries
// (frequency, mode, clarifier, CTCSS state, shift, tag) — writable
// selects whether this bank's Write mirrors rw.Write or is forced
// Unsupported.
//
//   - writable is true for MEM/PMS: cat.Dialect.writableSlot admits MW
//     writes to both (spec.md §3.2/§3.3 — MW's own P1 domain omits the 5
//     MHz/EMGCH lines, and this codec's project-wide writableSlot rule
//     already restricts every MW target to memory-or-PMS regardless).
//   - writable is false for the 5 MHz and EMGCH banks: MW cannot target
//     either (cat.Dialect.writableSlot structurally excludes them, the
//     same treatment the FT-710's DISCOVERED 60m/EMG banks get) — read
//     stays at rw.Read (MR/MT both read every bank alike, spec.md §3.3's
//     own MTReadsReadable finding), write is forced Unsupported.
//
// FieldTagDisplay is the ZERO FieldSupport, unconditionally: FTX-1's MT
// form (MTFormShortNoDisplay) carries NO display byte at all (spec.md
// §3.3/§8) — there is no such flag anywhere on this radio's CAT surface
// to grade. FieldCTCSSTone/FieldScanSkip/FieldErase stay the zero
// FieldSupport too: the CAT protocol has no way to read or write a memory
// channel's tone table index, scan-skip flag, or to erase a channel at
// all (no CAT erase command is documented anywhere in this manual). The
// seventeen Icom-tier fields stay the zero FieldSupport: none of that
// vocabulary appears in this radio's command table (matrix §6).
//
// Each call returns a fresh map, so no two banks ever share one.
func bankFields(rw spec.FieldSupport, writable bool) map[spec.Field]spec.FieldSupport {
	write := rw.Write
	if !writable {
		write = spec.Unsupported
	}
	return map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:  {Read: rw.Read, Write: write},
		spec.FieldMode:       {Read: rw.Read, Write: write},
		spec.FieldClarifier:  {Read: rw.Read, Write: write},
		spec.FieldCTCSSState: {Read: rw.Read, Write: write},
		spec.FieldShift:      {Read: rw.Read, Write: write},
		spec.FieldTag:        {Read: rw.Read, Write: write},

		spec.FieldTagDisplay: {},
		spec.FieldCTCSSTone:  {},
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

// baseCapabilities assembles the static baseline every profile shares:
// four banks (memory/PMS/5 MHz/EMGCH — all static, none discovered; see
// sixtyMSlots' doc comment), with rw applied to MEM/PMS (writable) and
// the 5 MHz/EMGCH banks (read-only).
func baseCapabilities(rw spec.FieldSupport) spec.Capabilities {
	return spec.Capabilities{
		Model: modelName,
		CATID: catID,
		// MANUAL-EVIDENCED: this is a transceiver (the whole CAT
		// reference's PTT/MOX/transmit command surface), not a receiver.
		Transmit: spec.HasTransmitter,
		Banks: []spec.Bank{
			{ID: spec.BankMemory, Label: "Memories", Slots: memSlots(), Fields: bankFields(rw, true)},
			{ID: spec.BankPMS, Label: "Scan limits (PMS)", Slots: pmsSlots(), Fields: bankFields(rw, true)},
			{ID: spec.Bank60m, Label: "5 MHz band channels", Slots: sixtyMSlots(), Fields: bankFields(rw, false)},
			{ID: spec.BankEMG, Label: "Emergency (EMGCH)", Slots: emgSlots(), Fields: bankFields(rw, false)},
		},
		Modes: modeDisplayNames(),
		// "up to 12 characters" (spec.md §8).
		TagLen: 12,
		// "0000-9990 (Hz)", 10 Hz steps (spec.md §7) — consulted from the
		// dialect rather than restated, so the two cannot drift.
		ClarMaxHz:  dialect.Clarifier().MaxAbsHz,
		ClarStepHz: dialect.Clarifier().StepHz,
		// CTCSSTones/CTCSSToneRange stay nil/empty: matrix §6 records no
		// CTCSS tone-frequency chart located in the manual excerpts this
		// spec pass read — OPEN, not derived this milestone. This is
		// separate from ToneModes below: the six-value P8 state domain is
		// evidenced regardless of whether a tone-frequency chart is.
		//
		// Bauds/DefaultBaud: the manual documents a PER-PORT rate menu
		// (CAT-1/CAT-3 factory 38400, CAT-2 factory 4800; matrix §1/§6,
		// OPEN for plan-time resolution) — wider than this single-axis
		// field. ASSUMED/CHOICE: the full menu-selectable set is offered
		// and CAT-1/3's own factory default is taken as THE default,
		// since Capabilities has no per-port axis to represent CAT-2's
		// separately.
		Bauds:       []int{4800, 9600, 19200, 38400, 115200},
		DefaultBaud: 38400,
		// MinFreqHz/MaxFreqHz stay 0 (no bound): matrix §6 records this
		// as "Not derived this pass" — the record's 9-digit frequency
		// field is a wire-encoding width, not a tuning-range statement,
		// and no general-coverage range statement was read for this
		// spec.
		//
		// RequiredSlots stays nil: matrix §6, "ASSUMED absent, no manual
		// statement of a mandatory slot".
		ShiftOptions: spec.StandardShiftOptions(),
		// The FTX-1's own six-value tone-mode vocabulary (spec.md §6):
		// the three shared CTCSS states, one unsplit DCS state, and the
		// two states with no analogue elsewhere in this project's
		// vocabulary (ToneModePRFreq/ToneModeRevTone — core/spec/vocab.go,
		// added by lane T for exactly this radio). Wire bytes '4'/'5'
		// carry no named cat.CTCSSState constant (read.go/write.go's own
		// ctcssNames/ctcssByName cast the byte directly); this list is
		// the neutral-layer vocabulary the capability surface, the UI
		// and codeplug.Validate see.
		ToneModes: toneModes(),
	}
}

// toneModes is the FTX-1's own six-value P8 vocabulary (spec.md §6),
// deliberately NOT spec.StandardToneModes() (the FT-710's shared
// three-value list): this radio's domain is wider, so it builds its own,
// the same "a radio whose CTCSS field expresses more than the three shared
// states builds its own list" precedent StandardToneModes' own doc comment
// names (and core/driver/ft991a/caps.go's toneModes follows for its own
// five-value domain).
func toneModes() []spec.ToneMode {
	return []spec.ToneMode{
		{Value: "OFF", Semantics: spec.ToneModeOff},
		{Value: "ENC-DEC", Semantics: spec.ToneModeCTCSSSquelch},
		{Value: "ENC", Semantics: spec.ToneModeCTCSS},
		// "3: DCS" is a SINGLE unsplit value (spec.md §6) — the same
		// encode-and-require-a-match semantic ToneModeDCSEncodeDecode
		// already names for the FT-991A's own split '3' ("DCS ENC/DEC").
		// NeedsTxTone/NeedsRxTone/NeedsDTCS all report false: a DCS state
		// needs a CODE, not a CTCSS tone, and this byte names the state
		// only — core/cat's own P8 memory field carries no DCS code at
		// all (core/spec/vocab.go's own doc comment for the two DCS
		// members).
		{Value: "DCS", Semantics: spec.ToneModeDCSEncodeDecode},
		// NEUTRAL states with no tone/DTCS analogue anywhere else in this
		// project (spec.md §6, core/spec/vocab.go's own doc comment for
		// these two members): "PR FREQ" (pseudo-repeater frequency?) and
		// "REV TONE" (tone-squelch reversal), neither further explained
		// by this manual's CAT chapter.
		{Value: "PR-FREQ", Semantics: spec.ToneModePRFreq},
		{Value: "REV-TONE", Semantics: spec.ToneModeRevTone},
	}
}

// CapabilitiesUnverified is the all-Unverified fail-safe: what a
// RealHardware session gets while writeTrialsComplete is false (always,
// so far), and what any UNRECOGNISED Profile value gets too. Every
// expressible field's Read AND Write is spec.Unverified where the codec
// can express it at all, on MEM/PMS; the 5 MHz/EMGCH banks additionally
// force Write to spec.Unsupported (bankFields' writable=false arm).
func CapabilitiesUnverified() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified})
}

// CapabilitiesSimulated is the fake-radio-backed profile (CLI --fake, GUI
// demo) ONLY, never a real radio: Write Supported for exactly the fields
// the codec can express AND the target bank permits (MEM/PMS only — the
// 5 MHz/EMGCH banks stay write-Unsupported even here, since no CAT write
// can ever reach them regardless of profile).
func CapabilitiesSimulated() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Supported, Write: spec.Supported})
}
