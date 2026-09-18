// SPDX-License-Identifier: GPL-3.0-or-later

package ic9100

import (
	"fmt"

	civic9100 "github.com/gm5dna/open-rig-programmer/core/civ/ic9100"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// Profile selects the evidence gate used by New. RealHardware is the zero
// value so an uninitialised profile fails safe.
type Profile = driver.Profile

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// writeTrialsComplete remains false until an IC-9100 has actually been
// bench-connected (matrix §3.14). No IC-9100 has ever been asked anything
// by this program (matrix §0 "Hardware status").
const writeTrialsComplete = false

const (
	minFreqHz = 30_000 // matrix §1 row 15.
	// maxFreqHz is the base 3-band radio's own UHF ceiling (matrix §1 row
	// 16, "420.000 ~ 480.000 MHz"). The UX-9100's 1200 MHz band is
	// deferred (core/civ/ic9100/doc.go): its own record encoding is
	// UNRESOLVED (matrix §1 row 16 / §3.15(6)), so this driver declares no
	// domain that would authorise a write into it.
	maxFreqHz   = 480_000_000
	defaultBaud = 19_200
)

var (
	modeNames = []string{"LSB", "USB", "AM", "CW", "RTTY", "FM", "CW-R", "RTTY-R", "DV"}
	baudRates = []int{300, 1200, 4800, 9600, 19200}
	// standardDTCSCodes: matrix §1 row 23, the same 104-value standard
	// chart as IC-7100/IC-7610.
	standardDTCSCodes = []int{
		23, 25, 26, 31, 32, 36, 43, 47, 51, 53, 54, 65, 71, 72, 73, 74,
		114, 115, 116, 122, 125, 131, 132, 134, 143, 145, 152, 155, 156,
		162, 165, 172, 174, 205, 212, 223, 225, 226, 243, 244, 245, 246,
		251, 252, 255, 261, 263, 265, 266, 271, 274, 306, 311, 315, 325,
		331, 332, 343, 346, 351, 356, 364, 365, 371, 411, 412, 413, 423,
		431, 432, 445, 446, 452, 454, 455, 462, 464, 465, 466, 503, 506,
		516, 523, 526, 532, 546, 565, 606, 612, 624, 627, 631, 632, 654,
		662, 664, 703, 712, 723, 731, 732, 734, 743, 754,
	}
	// bandNames: matrix §1b "Slot string" — the record's own band names
	// (PDF p.204 field q), not IC-7100's single-letter convention, because
	// this radio's bands are not single letters.
	bandNames = []string{"HF", "144", "430"}
)

func slotName(band int, channel int) string {
	return fmt.Sprintf("%s-%03d", bandNames[band], channel)
}

func duplexOptions() []spec.DuplexOption {
	return []spec.DuplexOption{
		{Value: "OFF", Direction: spec.DuplexOff},
		{Value: "DUP-", Direction: spec.DuplexDown},
		{Value: "DUP+", Direction: spec.DuplexUp},
	}
}

func toneModes() []spec.ToneMode {
	return []spec.ToneMode{
		{Value: "OFF", Semantics: spec.ToneModeOff},
		{Value: "TONE", Semantics: spec.ToneModeCTCSS},
		{Value: "TSQL", Semantics: spec.ToneModeCTCSSSquelch},
		{Value: "DTCS", Semantics: spec.ToneModeDTCS},
	}
}

func memorySlots() []string {
	slots := make([]string, 0, 3*99)
	for band := 0; band < 3; band++ {
		for channel := 1; channel <= 99; channel++ {
			slots = append(slots, slotName(band, channel))
		}
	}
	return slots
}

func fieldGrid(write spec.Support) map[spec.Field]spec.FieldSupport {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: write}
	return map[spec.Field]spec.FieldSupport{
		// Matrix §2: the twelve fields the 57-byte record expresses. No
		// FieldTxFrequency: unlike IC-7100/IC-7700 in the same wave, this
		// record carries no TX-side frequency block (matrix §1b).
		spec.FieldFrequency: rw, spec.FieldMode: rw, spec.FieldTag: rw,
		spec.FieldDuplex: rw, spec.FieldOffset: rw,
		spec.FieldToneMode: rw, spec.FieldToneTx: rw, spec.FieldToneRx: rw,
		spec.FieldDTCSCode: rw, spec.FieldDTCSPolarity: rw,
		spec.FieldFilter: rw, spec.FieldDataMode: rw,

		// Matrix §2 deliberately-zero audit.
		spec.FieldClarifier: {}, spec.FieldCTCSSState: {}, spec.FieldCTCSSTone: {},
		spec.FieldShift: {}, spec.FieldTagDisplay: {}, spec.FieldScanSkip: {},
		spec.FieldErase: {}, spec.FieldTxFrequency: {},
		spec.FieldTuningStepEnabled: {}, spec.FieldTuningStep: {},
		spec.FieldProgramTuningStep: {}, spec.FieldAttenuator: {},
		spec.FieldPreamp: {}, spec.FieldAntenna: {}, spec.FieldIPPlus: {},
	}
}

func capabilities(write spec.Support) spec.Capabilities {
	p := civic9100.Profile()
	return spec.Capabilities{
		Model: p.Model(), // Matrix §1 row 1.
		// Matrix §1 row 2 — HEADLINE FINDING (matrix §3.4): the printed
		// default is 7Ch, not the 88h/E0h pair spec.md §1 and this wave's
		// own dispatch title assumed.
		CATID:    "7C",
		Transmit: spec.HasTransmitter, // Matrix §1 row 3.
		// Matrix §1 row 5 / §1b: one dense MEM bank, the base 3-band
		// 297-slot rectangle (the UX-9100 4th band is deferred, caps.go's
		// maxFreqHz comment and core/civ/ic9100/doc.go).
		Banks: []spec.Bank{{
			ID: spec.BankMemory, Label: "Memories", Slots: memorySlots(),
			NoBlank: false, Fields: fieldGrid(write), Sparse: false,
			Groups: 0, GroupBase: 0, PerGroup: 0, ChannelBase: 0,
			Budget: 0, BudgetUnstated: false,
		}},
		Modes:  append([]string(nil), modeNames...), // Matrix §1 row 6.
		TagLen: 9,                                   // Matrix §1 row 7.
		// NoTag is left at its zero value, false: matrix §1 row 8 — a
		// positive TagLen and no evidence this radio lacks a name route.

		ClarMaxHz:  0,   // Matrix §1 row 9: no per-channel clarifier field.
		ClarStepHz: 0,   // Matrix §1 row 10: as above.
		CTCSSTones: nil, // Matrix §1 row 11: the wire carries a number, not a table index.
		// Matrix §1 row 12: bounds MANUAL-EVIDENCED from the printed
		// 50-tone chart (67.0-254.1 Hz); STEP is ASSUMED at the field's
		// own finest digit, 0.1 Hz. Register entry ic9100-tone-range-step.
		CTCSSToneRange: &spec.ToneRange{MinDeciHz: 670, MaxDeciHz: 2541, StepDeciHz: 1},

		Bauds: append([]int(nil), baudRates...), // Matrix §1 row 13.
		// Matrix §1 row 14: 19200 is the highest documented numeric rate;
		// ASSUMED factory Auto locking on the first 19 00, register entry
		// ic9100-default-baud-auto.
		DefaultBaud: defaultBaud,
		MinFreqHz:   minFreqHz, // Matrix §1 row 15.
		MaxFreqHz:   maxFreqHz, // Matrix §1 row 16; see the constant's own comment.

		RequiredSlots: nil,             // Matrix §1 row 17: no never-empty slot.
		ShiftOptions:  nil,             // Matrix §1 row 18: FieldDuplex replaces this vocabulary.
		DuplexOptions: duplexOptions(), // Matrix §1 row 20.
		ToneModes:     toneModes(),     // Matrix §1 row 21.
		// Matrix §1 row 22.
		DTCSPolarities: []string{"NN", "NR", "RN", "RR"},
		// Matrix §1 row 23: the conservative 104-code CHOICE, pending
		// register entry ic9100-dtcs-code-clamp.
		DTCSCodes: append([]int(nil), standardDTCSCodes...),
		Filters:   []string{"FIL1", "FIL2", "FIL3"}, // Matrix §1 row 24.

		TuningSteps:            nil,                     // Matrix §1 row 25 / §1b D8.
		ProgramTuningStepRange: nil,                     // Matrix §1 row 26 / §1b D8.
		AttenuatorDB:           nil,                     // Matrix §1 row 27 / §1b D8.
		PreampOptions:          nil,                     // Matrix §1 row 28 / §1b D8.
		AntennaOptions:         nil,                     // Matrix §1 row 29 / §1b D8.
		TagCharset:             string(p.NameCharset()), // Matrix §1 row 30.
	}
}

// CapabilitiesUnverified is the real-radio baseline. All mapped writes are
// Unverified while writeTrialsComplete is false.
func CapabilitiesUnverified() spec.Capabilities { return capabilities(spec.Unverified) }

// CapabilitiesSimulated enables the twelve profile-expressible memory
// fields so an in-package responding port can exercise the write choreography.
func CapabilitiesSimulated() spec.Capabilities { return capabilities(spec.Supported) }
