// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600

import (
	civicr8600 "github.com/gm5dna/open-rig-programmer/core/civ/icr8600"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// writeTrialsComplete is false because no IC-R8600 has ever been written by
// this project (matrix section 3.14). TestConsentAndWriteTrialsRemainFailSafe
// pins the value and the independently load-bearing Unverified profile; a
// future Stage W trial must change both deliberately.
const writeTrialsComplete = false

// Profile selects the evidence grading New uses. The zero value is the
// fail-safe physical-radio profile.
type Profile int

const (
	RealHardware Profile = iota
	Simulated
)

func dtcsCodes() []int {
	codes := make([]int, 0, 512)
	for a := 0; a < 8; a++ {
		for b := 0; b < 8; b++ {
			for c := 0; c < 8; c++ {
				codes = append(codes, a*100+b*10+c)
			}
		}
	}
	return codes
}

// bankFields transcribes every cell of matrix section 2. The select-memory
// marker is civ.FieldSelect, not neutral scan_skip; the latter is therefore a
// written-down zero so the ten-valued SELECT group is never collapsed to a
// BoolField. All mapped fields use the caller's evidence grade.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency: rw, spec.FieldMode: rw, spec.FieldTag: rw,
		spec.FieldDuplex: rw, spec.FieldOffset: rw, spec.FieldToneMode: rw,
		spec.FieldToneRx: rw, spec.FieldDTCSCode: rw,
		spec.FieldDTCSPolarity: rw, spec.FieldFilter: rw,
		spec.FieldTuningStepEnabled: rw, spec.FieldTuningStep: rw,
		spec.FieldProgramTuningStep: rw, spec.FieldAttenuator: rw,
		spec.FieldPreamp: rw, spec.FieldAntenna: rw, spec.FieldIPPlus: rw,

		// Matrix section 2's deliberate zeros, plus the global SELECT
		// ruling described above. FieldErase stays zero even after consent.
		spec.FieldClarifier: {}, spec.FieldCTCSSState: {},
		spec.FieldCTCSSTone: {}, spec.FieldShift: {},
		spec.FieldTagDisplay: {}, spec.FieldScanSkip: {},
		spec.FieldErase: {}, spec.FieldTxFrequency: {},
		spec.FieldToneTx: {}, spec.FieldDataMode: {},
	}
}

func baseCapabilities(rw spec.FieldSupport) spec.Capabilities {
	return spec.Capabilities{
		// Matrix section 1 row 1: display choice over the manual's product
		// name. Row 2: 96 is evidenced; the appended 19 00 token is
		// ASSUMED under icr8600-id-token (Stage R sends 19 00 and records
		// the complete address-matched reply).
		Model: civicr8600.Model,
		CATID: "96",
		// Matrix section 1 row 23: the guide calls the product a receiver.
		Transmit: spec.ReceiveOnly,
		// Matrix sections 1b.3 and 3.15.4: one zero-based 100x100 sparse
		// MEM space. Capacity is CE: icr8600-budget's Stage R lift fills
		// ordinary memories until the receiver refuses another.
		Banks: []spec.Bank{{
			ID: spec.BankMemory, Label: "Memories", Fields: bankFields(rw),
			Sparse: true, Groups: 100, GroupBase: 0,
			PerGroup: 100, ChannelBase: 0, BudgetUnstated: true,
		}},
		// Matrix section 1 row 4, in its chosen UI order.
		Modes: []string{"LSB", "USB", "AM", "CW", "FSK", "FM", "WFM", "CW-R", "FSK-R", "S-AM (D)", "S-AM (L)", "S-AM (U)", "P25", "D-STAR", "dPMR", "NXDN-VN", "NXDN-N", "DCR"},
		// Matrix section 1 row 5.
		TagLen: civicr8600.NameLength,
		// Matrix section 1 rows 6-7: no per-channel clarifier vocabulary.
		ClarMaxHz: 0, ClarStepHz: 0,
		// Matrix section 1 rows 8-9: the wire carries a numeric tone, not
		// an indexed chart. The field domain is 0.1..299.9 Hz; the actual
		// chart bounds remain ASSUMED under icr8600-tsql-chart-bounds,
		// lifted at Stage R by reading the lowest and highest choices.
		CTCSSTones:     nil,
		CTCSSToneRange: &spec.ToneRange{MinDeciHz: 1, MaxDeciHz: 2999, StepDeciHz: 1},
		// Matrix section 1 rows 10-11. The six-rate menu set is ASSUMED
		// under icr8600-baud-set (Stage R photographs every menu choice).
		// 19200 is the explicit conservative ASSUMED opening value under
		// icr8600-default-baud; Stage R factory-resets the receiver, reads
		// the menu without changing it and confirms a session at that rate.
		Bauds: []int{4800, 9600, 19200, 38400, 57600, 115200}, DefaultBaud: 19200,
		// Matrix section 1 rows 12-13: the actual tuning limits cannot be
		// established. The driver advertises no guessed bound.
		MinFreqHz: 0, MaxFreqHz: 0,
		// Matrix section 1 row 14: no individually mandatory slot.
		RequiredSlots: nil,
		// Matrix section 1 rows 15-16: the Yaesu vocabularies do not apply.
		ShiftOptions: nil, CTCSSStates: nil,
		// Matrix section 1 row 17.
		DuplexOptions: []spec.DuplexOption{
			{Value: "OFF", Direction: spec.DuplexOff},
			{Value: "DUP-", Direction: spec.DuplexDown},
			{Value: "DUP+", Direction: spec.DuplexUp},
		},
		// Matrix section 1 row 18 and additions Erratum 1: TSQL needs a
		// received tone and no transmitted tone.
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TSQL", Semantics: spec.ToneModeCTCSSRxSquelch},
			{Value: "DTCS", Semantics: spec.ToneModeDTCS},
		},
		// Matrix section 1 rows 19-20. The one printed polarity is receive
		// polarity. The exact selectable code chart remains ASSUMED under
		// icr8600-dtcs-chart; Stage R walks the front-panel list.
		DTCSPolarities: []string{"Normal", "Reverse"}, DTCSCodes: dtcsCodes(),
		// Matrix section 1 row 21.
		Filters: []string{"FIL1", "FIL2", "FIL3"},
		// Matrix section 1b.2. The 100 Hz floor is ASSUMED under
		// icr8600-progstep-floor; Stage R reads the smallest selectable
		// programmable step. All other values are the printed domains.
		TuningSteps:            []string{"100 Hz", "1 kHz", "2.5 kHz", "3.125 kHz", "5 kHz", "6.25 kHz", "8.33 kHz", "9 kHz", "10 kHz", "12.5 kHz", "20 kHz", "25 kHz", "100 kHz", "programmable tuning step"},
		ProgramTuningStepRange: &spec.StepRange{MinHz: 100, MaxHz: 999900, ResolutionHz: 100},
		AttenuatorDB:           []int{0, 10, 20, 30},
		PreampOptions:          []string{"OFF", "ON"},
		AntennaOptions:         []string{"ANT1", "ANT2", "ANT3"},
		// Matrix section 1 row 22. Glyphs are evidenced; ASCII code values
		// are ASSUMED under profile register icr8600-name-charset-codes.
		TagCharset: civicr8600.NameCharset,
	}
}

// CapabilitiesUnverified is the physical-radio profile. Reads and candidate
// writes are documented but have never been exercised on hardware.
func CapabilitiesUnverified() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified})
}

// CapabilitiesSimulated is the Stage 3 fake/test profile. Its support claim
// is about the independent fake only and is never evidence about hardware.
func CapabilitiesSimulated() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Supported, Write: spec.Supported})
}

func cloneCapabilities(caps spec.Capabilities) spec.Capabilities {
	out := caps
	out.Banks = make([]spec.Bank, 0, len(caps.Banks))
	for _, b := range caps.Banks {
		cp, _ := caps.Bank(b.ID)
		out.Banks = append(out.Banks, cp)
	}
	out.Modes = append([]string(nil), caps.Modes...)
	out.CTCSSTones = append([]spec.Tone(nil), caps.CTCSSTones...)
	if caps.CTCSSToneRange != nil {
		r := *caps.CTCSSToneRange
		out.CTCSSToneRange = &r
	}
	out.Bauds = append([]int(nil), caps.Bauds...)
	out.RequiredSlots = append([]string(nil), caps.RequiredSlots...)
	out.ShiftOptions = append([]spec.ShiftOption(nil), caps.ShiftOptions...)
	out.CTCSSStates = append([]spec.ToneState(nil), caps.CTCSSStates...)
	out.DuplexOptions = append([]spec.DuplexOption(nil), caps.DuplexOptions...)
	out.ToneModes = append([]spec.ToneMode(nil), caps.ToneModes...)
	out.DTCSPolarities = append([]string(nil), caps.DTCSPolarities...)
	out.DTCSCodes = append([]int(nil), caps.DTCSCodes...)
	out.Filters = append([]string(nil), caps.Filters...)
	out.TuningSteps = append([]string(nil), caps.TuningSteps...)
	if caps.ProgramTuningStepRange != nil {
		r := *caps.ProgramTuningStepRange
		out.ProgramTuningStepRange = &r
	}
	out.AttenuatorDB = append([]int(nil), caps.AttenuatorDB...)
	out.PreampOptions = append([]string(nil), caps.PreampOptions...)
	out.AntennaOptions = append([]string(nil), caps.AntennaOptions...)
	return out
}
