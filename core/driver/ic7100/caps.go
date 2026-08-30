// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"fmt"

	civic7100 "github.com/gm5dna/open-rig-programmer/core/civ/ic7100"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

func slotName(bank byte, channel int) string {
	return fmt.Sprintf("%c-%03d", bank, channel)
}

// Profile selects the evidence gate used by New. RealHardware is the zero
// value so an uninitialised profile fails safe.
type Profile int

const (
	RealHardware Profile = iota
	Simulated
)

// writeTrialsComplete remains false until the named Stage-W hardware lifts
// in register.go have been completed. TestWriteTrialsCompletePinnedFalse pins
// the safety claim this constant represents.
const writeTrialsComplete = false

const (
	minFreqHz   = 30_000
	maxFreqHz   = 470_000_000
	defaultBaud = 19_200
)

var (
	modeNames         = []string{"LSB", "USB", "AM", "CW", "RTTY", "FM", "WFM", "CW-R", "RTTY-R", "DV"}
	baudRates         = []int{300, 1200, 4800, 9600, 19200}
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
)

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
	slots := make([]string, 0, 5*99)
	for bank := byte('A'); bank <= 'E'; bank++ {
		for channel := 1; channel <= 99; channel++ {
			slots = append(slots, slotName(bank, channel))
		}
	}
	return slots
}

func fieldGrid(write spec.Support) map[spec.Field]spec.FieldSupport {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: write}
	return map[spec.Field]spec.FieldSupport{
		// Matrix §2: the thirteen fields the 111-byte record expresses.
		spec.FieldFrequency: rw, spec.FieldMode: rw, spec.FieldTag: rw,
		spec.FieldTxFrequency: rw, spec.FieldDuplex: rw, spec.FieldOffset: rw,
		spec.FieldToneMode: rw, spec.FieldToneTx: rw, spec.FieldToneRx: rw,
		spec.FieldDTCSCode: rw, spec.FieldDTCSPolarity: rw,
		spec.FieldFilter: rw, spec.FieldDataMode: rw,

		// Matrix §2 deliberately-zero audit. Field ④ is select-memory
		// membership, not scan_skip; erase is unshipped even with consent.
		spec.FieldClarifier: {}, spec.FieldCTCSSState: {}, spec.FieldCTCSSTone: {},
		spec.FieldShift: {}, spec.FieldTagDisplay: {}, spec.FieldScanSkip: {},
		spec.FieldErase: {}, spec.FieldTuningStepEnabled: {},
		spec.FieldTuningStep: {}, spec.FieldProgramTuningStep: {},
		spec.FieldAttenuator: {}, spec.FieldPreamp: {}, spec.FieldAntenna: {},
		spec.FieldIPPlus: {},
	}
}

func capabilities(write spec.Support) spec.Capabilities {
	p := civic7100.Profile()
	return spec.Capabilities{
		Model: p.Model(), // Matrix §1 row 1.
		// Matrix §1 row 2; the ASSUMED observed 19 00 token is joined per
		// session and pinned to register entry ic7100-id-reply-value.
		CATID:    "88",
		Transmit: spec.HasTransmitter, // Matrix §1 row 23.
		// Matrix §1 row 3 and §1b: one dense 495-slot MEM bank; all
		// sparse descriptors, including BudgetUnstated, are explicitly zero.
		Banks: []spec.Bank{{
			ID: spec.BankMemory, Label: "Memories", Slots: memorySlots(),
			NoBlank: false, Fields: fieldGrid(write), Sparse: false,
			Groups: 0, GroupBase: 0, PerGroup: 0, ChannelBase: 0,
			Budget: 0, BudgetUnstated: false,
		}},
		Modes:  append([]string(nil), modeNames...), // Matrix §1 row 4.
		TagLen: 16,                                  // Matrix §1 row 5.

		ClarMaxHz:  0,   // Matrix §1 row 6: no per-channel clarifier field.
		ClarStepHz: 0,   // Matrix §1 row 7: no per-channel clarifier field.
		CTCSSTones: nil, // Matrix §1 row 8: the wire carries a number, not a table index.
		// Matrix §1 row 9 / §3.16.2: THE WIRE FIELD'S OWN CAPACITY, at its
		// own 0.1 Hz resolution. The three-byte tone span is a BCD
		// FREQUENCY indexing no table, and its printed per-digit legend
		// admits 000.0–299.9 Hz, so the declared domain is that capacity
		// intersected with the capability floor of 1 — because 0 Hz is not
		// a tone and spec.ToneRange requires MinDeciHz > 0.
		//
		// THE TIER'S RECORDED DOCTRINE, followed here rather than argued
		// again. The IC-7300 met the identical artefact — a printed 50-tone
		// chart over a BCD frequency field — and declared the capacity
		// anyway (core/driver/ic7300/caps.go:242-251: "the record stores a
		// BCD FREQUENCY indexing no table… A fifty-entry list here would
		// fail closed on every encodable value outside it"), and that
		// landed as IC-7300 matrix ERRATUM 12. Declaring the chart's
		// 67.0–254.1 Hz bounds instead made a 254.2–299.9 Hz tone read
		// Unknown and unwritable on this model whilst the same wire value
		// round-trips on every sibling; the tier review refused the split.
		//
		// THE 50-TONE CHART IS NOT LOST — it is prose, in doc.go. It
		// remains what the PANEL offers, which is a different claim from
		// what the record can carry. Register entry ic7100-tone-range-step
		// still asks whether the radio ACCEPTS an off-chart tenth of a
		// hertz; it no longer decides this declaration.
		//
		// Pinned by TestCapabilityValuesFromMatrix and
		// TestCTCSSToneDomainAdmitsEveryChartTone.
		CTCSSToneRange: &spec.ToneRange{MinDeciHz: 1, MaxDeciHz: 2999, StepDeciHz: 1},

		Bauds: append([]int(nil), baudRates...), // Matrix §1 row 10.
		// Matrix §1 row 11: 19200 is the highest documented numeric rate;
		// ASSUMED factory Auto locking on the first 19 00 is pinned to
		// register entry ic7100-default-baud-auto and remains version-dependent.
		DefaultBaud: defaultBaud,
		// Matrix §1 row 12; ASSUMED storable bound pinned to register entry
		// ic7100-storable-frequency-range.
		MinFreqHz: minFreqHz,
		// Matrix §1 row 13; ASSUMED storable bound pinned to register entries
		// ic7100-storable-frequency-range and ic7100-out-of-coverage-write.
		MaxFreqHz: maxFreqHz,

		RequiredSlots: nil,             // Matrix §1 row 14: no never-empty slot.
		ShiftOptions:  nil,             // Matrix §1 row 15: FieldDuplex replaces this vocabulary.
		CTCSSStates:   nil,             // Matrix §1 row 16: FieldToneMode replaces this vocabulary.
		DuplexOptions: duplexOptions(), // Matrix §1 row 17.
		ToneModes:     toneModes(),     // Matrix §1 row 18.
		// Matrix §1 row 19.
		DTCSPolarities: []string{"NN", "NR", "RN", "RR"},
		DTCSCodes:      append([]int(nil), standardDTCSCodes...), // Matrix §1 row 20: the conservative 104-code CHOICE.
		Filters:        []string{"FIL1", "FIL2", "FIL3"},         // Matrix §1 row 21.

		TuningSteps:            nil,                     // Matrix §1b D8: no tuning-step field.
		ProgramTuningStepRange: nil,                     // Matrix §1b D8: no programmable-step field.
		AttenuatorDB:           nil,                     // Matrix §1b D8: attenuator is VFO-level only.
		PreampOptions:          nil,                     // Matrix §1b D8: preamp is VFO-level only.
		AntennaOptions:         nil,                     // Matrix §1b D8: no per-channel antenna byte.
		TagCharset:             string(p.NameCharset()), // Matrix §1 row 22: printable ASCII 0x20–0x7e.
	}
}

// CapabilitiesUnverified is the real-radio baseline. All mapped writes are
// Unverified while writeTrialsComplete is false.
func CapabilitiesUnverified() spec.Capabilities { return capabilities(spec.Unverified) }

// CapabilitiesSimulated enables only the thirteen profile-expressible memory
// fields so an in-package responding port can exercise the write choreography.
func CapabilitiesSimulated() spec.Capabilities { return capabilities(spec.Supported) }

func cloneCapabilities(c spec.Capabilities) spec.Capabilities {
	out := c
	out.Banks = make([]spec.Bank, 0, len(c.Banks))
	for _, b := range c.Banks {
		cp, _ := c.Bank(b.ID)
		out.Banks = append(out.Banks, cp)
	}
	out.Modes = append([]string(nil), c.Modes...)
	out.CTCSSTones = append([]spec.Tone(nil), c.CTCSSTones...)
	if c.CTCSSToneRange != nil {
		r := *c.CTCSSToneRange
		out.CTCSSToneRange = &r
	}
	out.Bauds = append([]int(nil), c.Bauds...)
	out.RequiredSlots = append([]string(nil), c.RequiredSlots...)
	out.ShiftOptions = append([]spec.ShiftOption(nil), c.ShiftOptions...)
	out.CTCSSStates = append([]spec.ToneState(nil), c.CTCSSStates...)
	out.DuplexOptions = append([]spec.DuplexOption(nil), c.DuplexOptions...)
	out.ToneModes = append([]spec.ToneMode(nil), c.ToneModes...)
	out.DTCSPolarities = append([]string(nil), c.DTCSPolarities...)
	out.DTCSCodes = append([]int(nil), c.DTCSCodes...)
	out.Filters = append([]string(nil), c.Filters...)
	out.TuningSteps = append([]string(nil), c.TuningSteps...)
	out.AttenuatorDB = append([]int(nil), c.AttenuatorDB...)
	out.PreampOptions = append([]string(nil), c.PreampOptions...)
	out.AntennaOptions = append([]string(nil), c.AntennaOptions...)
	if c.ProgramTuningStepRange != nil {
		r := *c.ProgramTuningStepRange
		out.ProgramTuningStepRange = &r
	}
	return out
}
