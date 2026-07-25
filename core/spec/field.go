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
