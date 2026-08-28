// SPDX-License-Identifier: GPL-3.0-or-later

package spec

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
