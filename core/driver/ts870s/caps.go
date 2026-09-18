// SPDX-License-Identifier: GPL-3.0-or-later

package ts870s

import (
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// Profile selects the evidence gate used by New. RealHardware is the zero
// value so an uninitialised profile fails safe — the same shape every
// other driver package in this tier declares (core/driver/ic7200/caps.go).
type Profile = driver.Profile

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// writeTrialsComplete is FALSE. NO TS-870S HAS EVER BEEN ASKED ANYTHING BY
// THIS PROJECT (matrix header): every byte here came from the rigpix.com
// mirror of B62-1536-00 through the reviewed capability matrix, and from
// nothing else.
const writeTrialsComplete = false

// deliberatelyZero is the audit table TestDeliberatelyZeroAudit reflects
// over: every spec.Capabilities field this driver leaves at its zero
// value, with the reason — the ic7200 shape, cited to the matrix.
var deliberatelyZero = map[string]string{
	"SimplexTx": "matrix does not state what the P1=1 TX-frequency frame reads on a simplex (non-split) channel; SimplexTxUnstated is the legal zero declaration (spec.SimplexTx's own doc comment: Validate adds no rule for it)",
	"TagLen":    "matrix §1.6: NoTag — total absence (zero hits for a name/tag route in the 9,758-line extraction), cited to the 12/09/2026 nameless-capability rule (merge d0b2498, validate.go:253-301), not the old S2 triage verdict",

	"ClarMaxHz":  "matrix §1.7: the 22-byte record has no clarifier field at all — stated precisely as absence FROM THE RECORD, not from the radio (RIT/XIT exist as radio-level RC/RD/RU/RT/XT commands, never stored in a channel)",
	"ClarStepHz": "matrix §1.8: the same",

	"CTCSSToneRange": "matrix §1.10: P8 is a two-digit INDEX into this row's own 39-entry printed chart (CTCSSTones), not a raw frequency number — a radio declares a list or a range, never both",

	"RequiredSlots": "matrix §1.15: the manual's own vacant-channel sentence names no channel as mandatory",

	"ShiftOptions":  "matrix §1.16/§1.18: no shift/duplex selector exists in the 22-byte record — split is expressed only as the independent P1=1 TX frame (FieldTxFrequency); FieldShift and FieldDuplex both carry the zero FieldSupport, so E5b's anyBankReaches guard makes the empty list lawful",
	"DuplexOptions": "matrix §1.18: no duplex selector or offset-magnitude field exists among the 22 bytes; split is the two P1 frames, which is FieldTxFrequency, not FieldDuplex",

	"DTCSPolarities": "matrix §1.20/§1.21: zero case-insensitive hits for \"dcs\"/\"dtcs\" anywhere in the extraction",
	"DTCSCodes":      "matrix §1.20/§1.21: the same",
	"Filters":        "matrix §1.22: no FILTER A/B byte in the 22-byte record, and zero hits for \"FILTER A\"/\"FILTER B\" anywhere in the document",

	"TuningSteps":            "matrix §1.23: additions design D8 — no D8 receiver field applies to any of this wave's seven packages",
	"ProgramTuningStepRange": "matrix §1.24: as above",
	"AttenuatorDB":           "matrix §1.25: as above",
	"PreampOptions":          "matrix §1.26: as above",
	"AntennaOptions":         "matrix §1.27: as above",

	"TagCharset": "matrix §1.28: NoTag — no name route to have a charset for (the ic7200 shape)",
}

// memSlots is the one MEM bank's inventory, "00".."99" (matrix §1.4).
func memSlots() []string { return spec.NumberedSlots(0, 99, "%02d") }

// bankFields returns the MEM bank's field map, with rw applied to every
// field the 22-byte record MAPS and the zero FieldSupport everywhere else.
//
// EVERY spec.Field IS LISTED (ic7200's own rule, caps_test.go's field
// audit): an absent key and an explicit zero FieldSupport mean the same
// thing to FieldSupport, but only one is a decision a reader can check.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		// The six fields the 22-byte record maps (matrix §2).
		spec.FieldFrequency: rw, // P4, positions 6-16
		spec.FieldMode:      rw, // P5, position 17, eight-value legend (matrix §1.5)
		spec.FieldScanSkip:  rw, // P6, position 18, memory lockout
		// P1=1 frame, position 3 — published Unavailable at slot "99"
		// (matrix §1.4/§2.2, the TS-480 channel-99 precedent); that is a
		// session-level fact about one slot, not a capability grade.
		spec.FieldTxFrequency: rw,
		spec.FieldToneMode:    rw, // P7, position 19, OFF/TONE only (matrix §1.19)
		spec.FieldToneTx:      rw, // P8, positions 20-21, index into the 39-entry chart

		// MANUAL-EVIDENCED ABSENCE (matrix §2 bank table).
		spec.FieldClarifier:    {},
		spec.FieldCTCSSState:   {},
		spec.FieldCTCSSTone:    {},
		spec.FieldShift:        {},
		spec.FieldDuplex:       {},
		spec.FieldOffset:       {},
		spec.FieldToneRx:       {}, // no P9 byte at all on this row (matrix §1.19)
		spec.FieldDTCSCode:     {},
		spec.FieldDTCSPolarity: {},
		spec.FieldFilter:       {},
		spec.FieldDataMode:     {},

		// NOTAG (matrix §1.6): no name route over CAT at all.
		spec.FieldTag:        {},
		spec.FieldTagDisplay: {},

		// CHOICE (matrix §2): no erase command is documented for MR/MW
		// itself, and a short/zeroed MW's exact behaviour is not printed
		// at the pages this matrix read.
		spec.FieldErase: {},

		// Additions design D8 — no D8 receiver field applies to any of
		// the v1.7.0 seven Kenwood/Yaesu packages.
		spec.FieldTuningStepEnabled: {},
		spec.FieldTuningStep:        {},
		spec.FieldProgramTuningStep: {},
		spec.FieldAttenuator:        {},
		spec.FieldPreamp:            {},
		spec.FieldAntenna:           {},
		spec.FieldIPPlus:            {},
	}
}

func capabilities(write spec.Support) spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: write}
	return spec.Capabilities{
		Model:    "TS-870S",           // matrix §1.1
		CATID:    "015",               // matrix §1.2
		Transmit: spec.HasTransmitter, // matrix §1.3

		Banks: []spec.Bank{
			{
				ID: spec.BankMemory, Label: "Memories", Slots: memSlots(),
				NoBlank: false, Fields: bankFields(rw),
			},
		},

		// matrix §1.5: eight modes, Format 2's own order minus the two
		// "no mode" holes.
		Modes: []string{"LSB", "USB", "CW", "FM", "AM", "FSK", "CW-R", "FSK-R"},

		TagLen: 0,    // matrix §1.6
		NoTag:  true, // matrix §1.6

		// matrix §1.9: this row's OWN 39-entry SUBTONE TABLE, not
		// spec.StandardCTCSSTones() — a distinct, narrower list.
		CTCSSTones: []spec.Tone{
			670, 719, 744, 770, 797, 825, 854, 885, 915, 948,
			974, 1000, 1035, 1072, 1109, 1148, 1188, 1230, 1273, 1318,
			1365, 1413, 1462, 1514, 1567, 1622, 1679, 1738, 1799, 1862,
			1928, 2035, 2107, 2181, 2257, 2336, 2418, 2503, 17500,
		},

		// matrix §1.11: all seven printed rates, including 4800 — this
		// row's own permissive 1-or-2-stop-bit note, unlike the 590/480
		// exemplar's mandatory-2-stop-bit reason for omitting it.
		Bauds: []int{1200, 2400, 4800, 9600, 19200, 38400, 57600},
		// matrix §1.12: the manual states this outright, and it is
		// ASSUMED anyway per the wave's blanket rule (doc.go A2).
		DefaultBaud: 9600,

		MinFreqHz: 100_000,    // matrix §1.13
		MaxFreqHz: 30_000_000, // matrix §1.14

		// The Icom-style pair (design D4): this row expresses ToneModes,
		// not ShiftOptions (matrix §1.16/§1.19).
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS}, // transmits only, no receive tone (matrix §1.19)
		},
	}
}

// CapabilitiesUnverified is the real-radio baseline. All mapped writes are
// Unverified while writeTrialsComplete is false.
func CapabilitiesUnverified() spec.Capabilities { return capabilities(spec.Unverified) }

// CapabilitiesSimulated enables the mapped fields so an in-package
// scripted/fake port can exercise the write choreography end to end.
func CapabilitiesSimulated() spec.Capabilities { return capabilities(spec.Supported) }
