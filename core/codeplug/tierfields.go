// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import "github.com/gm5dna/open-rig-programmer/core/spec"

// TierField describes one of the seventeen tri-state fields the two Icom
// model extensions added to ChannelData (the ten of design D4, then the
// seven receiver fields of additions design D8).
//
// IT EXISTS BECAUSE SIXTEEN FUNCTIONS ENUMERATED THOSE SEVENTEEN FIELDS BY
// HAND — six in this package, eight in core/csvio, one in core/csvio's
// CHIRP importer and one in core/clone — so an eighteenth field meant
// sixteen edits, and one missed edit was a field that silently did not
// diff, did not validate, or did not export. Every function since folded
// onto this table is one fewer; the ones still enumerating by hand are the
// ones asking a question no column here carries.
//
// THE COLUMNS ARE THE ONES AT LEAST TWO OF THOSE FUNCTIONS NEED, and no
// more. What is deliberately NOT here is a per-field Valid closure — only
// validateTierFields asks, and every field asks a different question of
// spec.Capabilities. A per-field cell RENDERER AND PARSER DO exist — the
// eighteenth-field hazard is exactly as real for a CSV column as for a
// diff — but as core/csvio's own tierFieldCells, keyed by spec.Field so a
// row here finds its pair, and not here: both are made of that package's
// file format, its reserved state spellings ("n/a", "absent") and its
// column-named parse diagnostics, and hoisting them here would move the
// CSV format into the model package, which is a worse trade than one map
// lookup. A field added to TierFields with no matching tierFieldCells
// entry is a nil-func panic on export or import, not a silent omission —
// TestTierFieldCells_RoundTripEveryField's first check catches it, with a
// message naming the missing field.
type TierField struct {
	// Name is the ChannelData struct field's own Go name, e.g.
	// "TxFreqHz". It names the thing the accessors below reach, which
	// neither Field nor Column does, and it is what diagnostics and this
	// table's test identify a row by.
	Name string
	// Field is the neutral spec.Field this channel field carries.
	Field spec.Field
	// Column is the CSV column name core/csvio reads and writes it under,
	// e.g. "tx_frequency".
	Column string
	// Receiver marks the seven D8 receiver fields, which appear as their
	// own appended group in both the JSON schema (5, over 4) and the CSV
	// header (version 3, over 2). The ten D4 fields leave it false.
	Receiver bool
	// State returns a POINTER to this field's FieldState within d, so a
	// caller can read the state of a field it has only a row for.
	//
	// READ THROUGH IT; DO NOT WRITE THROUGH IT. Setting a state here
	// leaves the field's Value in place, and a non-Known state must carry
	// a zero Value (see FieldState) — SetState is the assignment that
	// gets that right.
	State func(d *ChannelData) *FieldState
	// SetState replaces this field entirely with the zero value in the
	// given state, so no value can survive a transition out of Known.
	SetState func(d *ChannelData, s FieldState)
	// Equal reports whether this field is identical in two channels,
	// comparing the WHOLE tri-state struct — state and value together, so
	// a state transition counts as a change exactly as a value change
	// does, which is what changedFields has always done.
	Equal func(before, after ChannelData) bool
}

// TierFields is the seventeen tier fields in ChannelData's own declaration
// order — the ten D4 fields, then the seven D8 receiver ones.
//
// THE ORDER IS LOAD-BEARING. It is the order a VerifyMismatchError's field
// list, a BlockReason's field list and a CSV header all render in, and the
// D8 group comes last so schema 4 and CSV version 2 stay a frozen PREFIX
// of schema 5 and version 3.
var TierFields = [...]TierField{
	{
		Name: "TxFreqHz", Field: spec.FieldTxFrequency, Column: "tx_frequency",
		State:    func(d *ChannelData) *FieldState { return &d.TxFreqHz.State },
		SetState: func(d *ChannelData, s FieldState) { d.TxFreqHz = FreqField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.TxFreqHz == b.TxFreqHz },
	},
	{
		Name: "Duplex", Field: spec.FieldDuplex, Column: "duplex",
		State:    func(d *ChannelData) *FieldState { return &d.Duplex.State },
		SetState: func(d *ChannelData, s FieldState) { d.Duplex = StringField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.Duplex == b.Duplex },
	},
	{
		Name: "OffsetHz", Field: spec.FieldOffset, Column: "offset",
		State:    func(d *ChannelData) *FieldState { return &d.OffsetHz.State },
		SetState: func(d *ChannelData, s FieldState) { d.OffsetHz = FreqField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.OffsetHz == b.OffsetHz },
	},
	{
		Name: "ToneMode", Field: spec.FieldToneMode, Column: "tone_mode",
		State:    func(d *ChannelData) *FieldState { return &d.ToneMode.State },
		SetState: func(d *ChannelData, s FieldState) { d.ToneMode = StringField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.ToneMode == b.ToneMode },
	},
	{
		Name: "ToneTx", Field: spec.FieldToneTx, Column: "tone_tx",
		State:    func(d *ChannelData) *FieldState { return &d.ToneTx.State },
		SetState: func(d *ChannelData, s FieldState) { d.ToneTx = ToneField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.ToneTx == b.ToneTx },
	},
	{
		Name: "ToneRx", Field: spec.FieldToneRx, Column: "tone_rx",
		State:    func(d *ChannelData) *FieldState { return &d.ToneRx.State },
		SetState: func(d *ChannelData, s FieldState) { d.ToneRx = ToneField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.ToneRx == b.ToneRx },
	},
	{
		Name: "DTCSCode", Field: spec.FieldDTCSCode, Column: "dtcs_code",
		State:    func(d *ChannelData) *FieldState { return &d.DTCSCode.State },
		SetState: func(d *ChannelData, s FieldState) { d.DTCSCode = IntField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.DTCSCode == b.DTCSCode },
	},
	{
		Name: "DTCSPolarity", Field: spec.FieldDTCSPolarity, Column: "dtcs_polarity",
		State:    func(d *ChannelData) *FieldState { return &d.DTCSPolarity.State },
		SetState: func(d *ChannelData, s FieldState) { d.DTCSPolarity = StringField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.DTCSPolarity == b.DTCSPolarity },
	},
	{
		Name: "Filter", Field: spec.FieldFilter, Column: "filter",
		State:    func(d *ChannelData) *FieldState { return &d.Filter.State },
		SetState: func(d *ChannelData, s FieldState) { d.Filter = StringField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.Filter == b.Filter },
	},
	{
		Name: "DataMode", Field: spec.FieldDataMode, Column: "data_mode",
		State:    func(d *ChannelData) *FieldState { return &d.DataMode.State },
		SetState: func(d *ChannelData, s FieldState) { d.DataMode = BoolField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.DataMode == b.DataMode },
	},
	{
		Name: "TuningStepEnabled", Field: spec.FieldTuningStepEnabled, Column: "tuning_step_enabled", Receiver: true,
		State:    func(d *ChannelData) *FieldState { return &d.TuningStepEnabled.State },
		SetState: func(d *ChannelData, s FieldState) { d.TuningStepEnabled = BoolField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.TuningStepEnabled == b.TuningStepEnabled },
	},
	{
		Name: "TuningStep", Field: spec.FieldTuningStep, Column: "tuning_step", Receiver: true,
		State:    func(d *ChannelData) *FieldState { return &d.TuningStep.State },
		SetState: func(d *ChannelData, s FieldState) { d.TuningStep = StringField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.TuningStep == b.TuningStep },
	},
	{
		Name: "ProgramTuningStepHz", Field: spec.FieldProgramTuningStep, Column: "program_tuning_step", Receiver: true,
		State:    func(d *ChannelData) *FieldState { return &d.ProgramTuningStepHz.State },
		SetState: func(d *ChannelData, s FieldState) { d.ProgramTuningStepHz = FreqField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.ProgramTuningStepHz == b.ProgramTuningStepHz },
	},
	{
		Name: "AttenuatorDB", Field: spec.FieldAttenuator, Column: "attenuator", Receiver: true,
		State:    func(d *ChannelData) *FieldState { return &d.AttenuatorDB.State },
		SetState: func(d *ChannelData, s FieldState) { d.AttenuatorDB = IntField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.AttenuatorDB == b.AttenuatorDB },
	},
	{
		Name: "Preamp", Field: spec.FieldPreamp, Column: "preamp", Receiver: true,
		State:    func(d *ChannelData) *FieldState { return &d.Preamp.State },
		SetState: func(d *ChannelData, s FieldState) { d.Preamp = StringField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.Preamp == b.Preamp },
	},
	{
		Name: "Antenna", Field: spec.FieldAntenna, Column: "antenna", Receiver: true,
		State:    func(d *ChannelData) *FieldState { return &d.Antenna.State },
		SetState: func(d *ChannelData, s FieldState) { d.Antenna = StringField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.Antenna == b.Antenna },
	},
	{
		Name: "IPPlus", Field: spec.FieldIPPlus, Column: "ip_plus", Receiver: true,
		State:    func(d *ChannelData) *FieldState { return &d.IPPlus.State },
		SetState: func(d *ChannelData, s FieldState) { d.IPPlus = BoolField{State: s} },
		Equal:    func(a, b ChannelData) bool { return a.IPPlus == b.IPPlus },
	},
}
