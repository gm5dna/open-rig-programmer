// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import "slices"

// Field identifies a single addressable piece of memory-channel data whose
// per-radio support is described by a FieldSupport value inside a Bank.
// Field is a neutral, radio-family concept: drivers map their own
// protocol-specific data onto these constants, rather than generic code
// ever knowing a protocol's own field names.
type Field string

// The Field constants this project currently models. Erase is deliberately
// a Field, not a separate mechanism: modelling "can this slot be erased"
// as just another field means the same Support/FieldSupport machinery
// (including the CanWrite hardware gate) governs it for free.
const (
	FieldFrequency  Field = "frequency"
	FieldMode       Field = "mode"
	FieldClarifier  Field = "clarifier"
	FieldCTCSSState Field = "ctcss_state"
	FieldCTCSSTone  Field = "ctcss_tone"
	FieldShift      Field = "shift"
	FieldTag        Field = "tag"
	FieldTagDisplay Field = "tag_display"
	FieldScanSkip   Field = "scan_skip"
	FieldErase      Field = "erase"
)

// The Fields the Icom tier (Tier 4) adds to the neutral memory model
// (design D4). Every one of them is UNSUPPORTED on the four Yaesu NEWCAT
// models registered before this tier: their banks simply do not list
// these Fields, and Capabilities.FieldSupport answers the zero
// FieldSupport — Unsupported both ways — for a Field a bank does not
// list. That is what keeps the Yaesu side of every capability-keyed
// decision (codeplug.Diff's touched set, codeplug.Validate's vocabulary
// checks, core/csvio's CHIRP mapping, the grid's column set) exactly as
// it was.
//
// The two vocabularies never coexist on one model: a radio expresses
// either the Yaesu shift/ctcss_state pair or the Icom
// duplex/tone_mode pair, never both. Nothing here removes the Yaesu
// Fields.
const (
	// FieldTxFrequency is an independent transmit ("split") frequency
	// stored on the channel itself, distinct from a repeater shift
	// applied to the receive frequency.
	//
	// TRANSMIT-ONLY: see IsTransmitField.
	FieldTxFrequency Field = "tx_frequency"
	// FieldDuplex is the Icom-family repeater duplex selector, whose
	// vocabulary a radio supplies as Capabilities.DuplexOptions (e.g.
	// "OFF", "DUP+", "DUP-").
	FieldDuplex Field = "duplex"
	// FieldOffset is the per-channel repeater offset magnitude in hertz.
	// The Yaesu family has no such per-channel field (its shift
	// magnitude is a global menu setting), which is why it is new here.
	FieldOffset Field = "offset"
	// FieldToneMode is the Icom-family tone-squelch mode, whose
	// vocabulary a radio supplies as Capabilities.ToneModes (e.g. "OFF",
	// "TONE", "TSQL", "DTCS"). It is the per-model superset the Yaesu
	// FieldCTCSSState three-state vocabulary is not.
	FieldToneMode Field = "tone_mode"
	// FieldToneTx is the transmitted CTCSS tone.
	//
	// TRANSMIT-ONLY: see IsTransmitField.
	FieldToneTx Field = "tone_tx"
	// FieldToneRx is the CTCSS tone required to open squelch on receive.
	// A radio that cannot hold the two independently reports only one of
	// this pair.
	FieldToneRx Field = "tone_rx"
	// FieldDTCSCode is the DTCS/DCS code number (CHIRP's DtcsCode
	// column's own numbering, e.g. 23 for "023").
	FieldDTCSCode Field = "dtcs_code"
	// FieldDTCSPolarity is the DTCS polarity pair, whose vocabulary a
	// radio supplies as Capabilities.DTCSPolarities (e.g. "NN", "NR",
	// "RN", "RR").
	FieldDTCSPolarity Field = "dtcs_polarity"
	// FieldFilter is the per-channel IF filter selection, whose
	// vocabulary a radio supplies as Capabilities.Filters (e.g. "FIL1",
	// "FIL2", "FIL3").
	FieldFilter Field = "filter"
	// FieldDataMode is the per-channel data-mode flag (Icom's D
	// modifier), stored alongside — not inside — the mode name.
	FieldDataMode Field = "data_mode"
)

// The receiver fields added by the Tier 4b Icom extension (additions
// design D8). They are neutral memory-channel concepts rather than Icom
// wire names: a scanner or receiver with the same setting grades the same
// Field, and a radio without it simply leaves the Field unsupported.
const (
	// FieldTuningStepEnabled is the per-channel on/off state of the tuning
	// step function, kept separate from the selected step so both values
	// round-trip independently.
	FieldTuningStepEnabled Field = "tuning_step_enabled"
	// FieldTuningStep is the selected tuning-step label, drawn from
	// Capabilities.TuningSteps.
	FieldTuningStep Field = "tuning_step"
	// FieldProgramTuningStep is the programmable tuning step in hertz.
	FieldProgramTuningStep Field = "program_tuning_step"
	// FieldAttenuator is the selected input attenuation in decibels.
	FieldAttenuator Field = "attenuator"
	// FieldPreamp is the selected preamplifier option, drawn from
	// Capabilities.PreampOptions.
	FieldPreamp Field = "preamp"
	// FieldAntenna is the selected antenna input, drawn from
	// Capabilities.AntennaOptions.
	FieldAntenna Field = "antenna"
	// FieldIPPlus is the per-channel IP+ signal-processing flag.
	FieldIPPlus Field = "ip_plus"
)

// transmitOnlyMarker is the exact phrase a Field constant's doc comment
// carries when that Field describes the TRANSMITTER rather than the
// receiver. It is a constant, not a literal in the test, so the
// convention has one spelling and the test that enforces it cannot drift
// from the comments it reads.
const transmitOnlyMarker = "TRANSMIT-ONLY:"

// transmitFields is every Field that describes the transmitter. A radio
// with no transmitter has no anatomy for them, so
// Capabilities.Validate refuses a spec.ReceiveOnly model whose bank
// grades any of them above Unsupported.
//
// ONE declaration, beside the Field constants themselves. The check that
// consumes it (Capabilities.Validate's bank loop) used to carry the pair
// as a two-item literal, so a transmit-only Field added later would have
// been graded freely on a receiver with Validate saying nothing.
//
// PRIVATE, AND THE SET IS VALIDATION POLICY. It was briefly exported so
// core/codeplug's receive-only message wording could ask it instead of
// restating the pair, and an exported slice is an exported mutable
// global: a single `spec.TransmitFields[0] = "nothing"` anywhere in the
// process would have disabled the receive-only protection in BOTH
// core/spec's bank loop and core/codeplug's unreachable-field wording,
// silently and for every radio. A caller has no reason to hold the set;
// it only ever needs the question, so IsTransmitField is what it gets.
// TestSpecExportsNoPackageLevelVar keeps it that way.
//
// TestTransmitFields_MatchTheDeclaredMarker parses this package's
// non-test files and fails unless this slice holds exactly the constants
// whose doc comment carries transmitOnlyMarker — so a new transmit-only
// Field is caught by the marker its author has already written, rather
// than by remembering this list exists.
var transmitFields = []Field{FieldTxFrequency, FieldToneTx}

// IsTransmitField reports whether f describes the TRANSMITTER rather than
// the receiver, and is the whole of this package's exported surface on
// that question. A receive-only radio has no anatomy for such a field:
// core/spec's own Capabilities.Validate refuses a bank that grades one
// above Unsupported, and core/codeplug's validator words the resulting
// issue as "this radio has no transmitter" rather than naming the field.
// Both answers come from transmitFields, which the Field constants'
// TRANSMIT-ONLY markers pin.
func IsTransmitField(f Field) bool {
	return slices.Contains(transmitFields, f)
}
