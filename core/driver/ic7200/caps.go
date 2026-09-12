// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200

import (
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// Profile selects the evidence gate used by New. RealHardware is the zero
// value so an uninitialised profile fails safe. Shared with every other
// driver package (core/driver.Profile); this package keeps its own
// Simulated selector, which internal/guards.TestSimulatedProfileTokensConfinement
// requires.
type Profile = driver.Profile

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// writeTrialsComplete is FALSE. NO IC-7200 HAS EVER BEEN ASKED ANYTHING BY
// THIS PROJECT (matrix §0, §3.14): every byte here came from the IC-7200
// Advanced Instructions manual through the reviewed capability matrix, and
// from nothing else.
const writeTrialsComplete = false

const (
	// MinRadioFreqHz is the encoding floor: the ④~⑧ digit legend admits 0
	// on every digit (matrix §1 row 13). The STORABLE floor is not
	// established by this document (matrix §3.15(e): the manual prints no
	// receiver-coverage statement for the main receiver at all), so the
	// encoding floor is what is declared — the same reasoning
	// core/driver/ic7610/caps.go gives for its own MinFreqHz.
	MinRadioFreqHz = 0
	// MaxRadioFreqHz is the encoding ceiling: matrix §1 row 14, the ④~⑧
	// digit legend's largest value, "1000 MHz digit: 0 (Fixed)", "100 MHz
	// digit: 0 (Fixed)", "10 MHz digit: 0-5", i.e. 0 0 5 9.999999 MHz =
	// 59.999999 MHz. The storable ceiling is likewise not established
	// (matrix §3.15(e)); register entry ic7200-storable-frequency-range.
	MaxRadioFreqHz = 59_999_999
)

// deliberatelyZero is the audit table TestDeliberatelyZeroAudit reflects
// over: every spec.Capabilities field this driver leaves at its zero
// value, with the reason.
var deliberatelyZero = map[string]string{
	"TagLen":                 "matrix §1 row 6: NoTag — this radio has no channel-name route over CAT at all, so TagLen is 0 by declaration, not omission (core/spec/validate.go's NoTag pairing rule requires exactly this)",
	"MinFreqHz":              "matrix §1 row 13: zero IS this radio's declared floor rather than an omission — the record's frequency span is unsigned BCD and its smallest encodable value is 0 Hz; the STORABLE floor is not established by this document",
	"ClarMaxHz":              "matrix §1 row 7: the 1A 00 record has no clarifier field, so there is no offset bound to state",
	"ClarStepHz":             "matrix §1 row 8: the same — no clarifier field, no step",
	"CTCSSTones":             "matrix §1 row 9: this radio has NO tone field of any kind — no CTCSS/TSQL/DTCS index anywhere in the record",
	"CTCSSToneRange":         "matrix §1 row 10: same as CTCSSTones — there is no BCD tone-frequency field to declare a range over, unlike the IC-7610/IC-7100 family this radio otherwise resembles",
	"RequiredSlots":          "matrix §1 row 15: no memory or scan-edge channel is documented as one that must stay populated",
	"ShiftOptions":           "matrix §1 row 16: the record has no shift/duplex/offset field at all, and FieldShift/FieldDuplex both carry the zero FieldSupport on both banks, so enabler E5b's anyBankReaches guard makes the empty list lawful",
	"CTCSSStates":            "matrix §1 row 17: same reasoning as CTCSSTones — no tone vocabulary of any kind exists on this record",
	"DuplexOptions":          "matrix §1b: the record has no duplex field; Split (③) is a TX-block-enable flag with no +/- sense, not a shift vocabulary (matrix §3.15(a))",
	"ToneModes":              "matrix §1 row 9/§1b: this radio has no tone route over CI-V at all",
	"DTCSPolarities":         "matrix §1b: DTCS is printed nowhere in the command table",
	"DTCSCodes":              "matrix §1b: the same — no DTCS code table is printed anywhere in this document",
	"TuningSteps":            "additions design D8: no D8 receiver field applies to any of the v1.7.0 six transceivers (matrix header note)",
	"ProgramTuningStepRange": "additions design D8: as above",
	"AttenuatorDB":           "additions design D8: as above",
	"PreampOptions":          "additions design D8: as above",
	"AntennaOptions":         "additions design D8: as above",
	"TagCharset":             "NoTag: this radio has no channel-name route over CAT at all, so there is no charset to declare (matrix §1 row 6)",
}

// memSlots is the MEM bank's inventory: "001".."199" (matrix §1 row 4).
func memSlots() []string { return spec.NumberedSlots(1, 199, "%03d") }

// scanSlots is the SCAN bank's inventory: the two scan edges.
//
// Matrix §3.15(d) — P1 and P2 are NOT a separate bank in the wire
// protocol. They are two more values of the same two-byte selector (BCD
// "02 00" channel 200 and "02 01" channel 201), exactly as
// core/civ/ic7200's profile declares (ChannelHi 201).
func scanSlots() []string { return []string{"P1", "P2"} }

// bankFields returns one bank's field map, with rw applied to every field
// the 1A 00 record MAPS and the zero FieldSupport everywhere else.
//
// IDENTICAL FOR MEM AND SCAN (matrix §2's SCAN bank note: the `1A 00`
// diagram is one diagram governing all three address forms). Whether
// every field is HONOURED on a scan edge is a separate, ASSUMED question
// (register entry ic7200-scan-edge-record-fields).
//
// EVERY spec.Field IS LISTED, including the ones this radio does not
// have — an absent key and an explicit zero FieldSupport mean the same
// thing to spec.Capabilities.FieldSupport, but only one of them is a
// decision a reader can check (caps_test.go's field audit).
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		// The five fields the 17-byte record maps — core/civ/ic7200's
		// layout() carries a FieldSpan for each and nothing else.
		spec.FieldFrequency:   rw, // ④~⑧, little-endian BCD
		spec.FieldMode:        rw, // ⑨, the seven printed mode codes
		spec.FieldFilter:      rw, // ⑩, Wide/Mid/Narrow
		spec.FieldDataMode:    rw, // ⑪, a full byte, 00/10
		spec.FieldTxFrequency: rw, // ❹~❽, the TX-duplicate block's frequency span (matrix §3.11)

		// MANUAL-EVIDENCED ABSENCE (matrix §2): each of these is a field
		// the neutral model has room for and this radio's 17-byte record
		// does not express.
		spec.FieldClarifier:    {},
		spec.FieldCTCSSState:   {},
		spec.FieldCTCSSTone:    {},
		spec.FieldShift:        {},
		spec.FieldScanSkip:     {}, // matrix §2 row 9: idx③ is Split, a wholly different concept
		spec.FieldDuplex:       {},
		spec.FieldOffset:       {},
		spec.FieldToneMode:     {},
		spec.FieldToneTx:       {},
		spec.FieldToneRx:       {},
		spec.FieldDTCSCode:     {},
		spec.FieldDTCSPolarity: {},

		// NOTAG (matrix §1 row 6): no name route over CAT at all.
		// core/spec/validate.go's NoTag rule refuses any bank that
		// grades either of these.
		spec.FieldTag:        {},
		spec.FieldTagDisplay: {},

		// ERASE. This radio documents a `0B` clear command (matrix
		// §3.13), but no IC-7200 has ever been asked to use it — the
		// zero FieldSupport is what makes core/clone/execute.go's
		// DiffErased branch unreachable for this model, and
		// spec.ConsentUnverifiedWrites structurally never consents this
		// field.
		spec.FieldErase: {},

		// Additions design D8 — no D8 receiver field applies to any of
		// the v1.7.0 six transceivers.
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
		Model:    "IC-7200",           // matrix §1 row 1
		CATID:    "76",                // matrix §1 row 2
		Transmit: spec.HasTransmitter, // matrix §1 row 3
		// The manual's own NOTE (matrix §3.11): "even if the Split
		// function is OFF, enter the data into ❹-⓫ to match your
		// transceiver. We recommend that you set the same data as
		// ④-⑪" — a simplex channel transmits where it receives.
		SimplexTx: spec.SimplexTxEqualsRx,

		Banks: []spec.Bank{
			{
				ID: spec.BankMemory, Label: "Memories", Slots: memSlots(),
				NoBlank: false, Fields: bankFields(rw),
			},
			{
				ID: spec.BankScan, Label: "Scan edges", Slots: scanSlots(),
				// matrix §3.15(c): "when the program channel is
				// selected, both settings should be 00" bears on
				// Split, which is unmapped — no radio-facing claim
				// that a scan edge must stay populated is printed.
				NoBlank: false, Fields: bankFields(rw),
			},
		},

		// matrix §1 row 5: seven codes, UI order following the printed
		// command-table order.
		Modes:  []string{"LSB", "USB", "AM", "CW", "RTTY", "CW-R", "RTTY-R"},
		TagLen: 0,    // matrix §1 row 6
		NoTag:  true, // matrix §1 row 6

		Bauds:       []int{300, 1200, 4800, 9600, 19200}, // matrix §1 row 11
		DefaultBaud: 19200,                               // matrix §1 row 12 / register ic7200-default-baud-auto: factory Auto locks to the highest documented rate, per the ic7100-default-baud-auto precedent
		MinFreqHz:   MinRadioFreqHz,                      // matrix §1 row 13
		MaxFreqHz:   MaxRadioFreqHz,                      // matrix §1 row 14

		Filters: []string{"Wide", "Mid", "Narrow"}, // matrix §1b
	}
}

// CapabilitiesUnverified is the real-radio baseline. All mapped writes are
// Unverified while writeTrialsComplete is false.
func CapabilitiesUnverified() spec.Capabilities { return capabilities(spec.Unverified) }

// CapabilitiesSimulated enables the five profile-expressible fields so an
// in-package scripted port can exercise the write choreography.
func CapabilitiesSimulated() spec.Capabilities { return capabilities(spec.Supported) }
