// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// tierFieldCell is the CSV half of one core/codeplug.TierField: how that
// field's cell is rendered, and how it is read back.
type tierFieldCell struct {
	// Cell renders the field as its CSV cell, reserved state spellings
	// and all (see exportFieldState).
	Cell func(codeplug.ChannelData) string
	// Parse fills the field in d from a cell, or returns the first
	// diagnostic. Like every cell parser here it is SYNTACTIC only:
	// whether the value is in this radio's vocabulary is
	// codeplug.Validate's question.
	Parse func(d *codeplug.ChannelData, cell string) error
}

// tierFieldCells is that half for all seventeen tier fields, keyed by
// spec.Field so a codeplug.TierFields row finds its own.
//
// IT LIVES HERE rather than as two more columns on codeplug.TierField
// because both closures are made of THIS package's file format — the
// reserved spellings cellUnavailable/cellAbsent, and the column-named
// parse diagnostics — which core/codeplug neither holds nor should
// import. codeplug.TierField's doc comment says the same thing from the
// other side.
//
// The point is TierFields' own: tierCellsFor, parseTierGroup and the two
// header lists used to enumerate these seventeen fields four times
// between them, so an eighteenth field meant four more edits and one
// missed edit was a column that silently exported or imported as
// nothing. Each field now appears once, and
// TestTierFieldCells_RoundTripEveryField fails on a row whose two
// closures do not name the same field.
//
// Every tier column takes allowAbsent = true: unlike the pre-tier
// ctcss_tone, scan_skip and tag_display columns, a tier column can spell
// Absent, because a pre-tier schema has no such column at all.
var tierFieldCells = map[spec.Field]tierFieldCell{
	spec.FieldTxFrequency: {
		Cell: func(d codeplug.ChannelData) string { return exportFreqField(d.TxFreqHz) },
		Parse: func(d *codeplug.ChannelData, cell string) (err error) {
			d.TxFreqHz, err = parseFreqFieldCell(cell, "tx_frequency")
			return
		},
	},
	spec.FieldDuplex: {
		Cell: func(d codeplug.ChannelData) string { return exportStringField(d.Duplex) },
		Parse: func(d *codeplug.ChannelData, cell string) error {
			d.Duplex = parseStringFieldCell(cell)
			return nil
		},
	},
	spec.FieldOffset: {
		Cell: func(d codeplug.ChannelData) string { return exportFreqField(d.OffsetHz) },
		Parse: func(d *codeplug.ChannelData, cell string) (err error) {
			d.OffsetHz, err = parseFreqFieldCell(cell, "offset")
			return
		},
	},
	spec.FieldToneMode: {
		Cell: func(d codeplug.ChannelData) string { return exportStringField(d.ToneMode) },
		Parse: func(d *codeplug.ChannelData, cell string) error {
			d.ToneMode = parseStringFieldCell(cell)
			return nil
		},
	},
	spec.FieldToneTx: {
		Cell: func(d codeplug.ChannelData) string { return exportToneField(d.ToneTx, true) },
		Parse: func(d *codeplug.ChannelData, cell string) (err error) {
			d.ToneTx, err = parseToneFieldCell(cell, "tone_tx", true)
			return
		},
	},
	spec.FieldToneRx: {
		Cell: func(d codeplug.ChannelData) string { return exportToneField(d.ToneRx, true) },
		Parse: func(d *codeplug.ChannelData, cell string) (err error) {
			d.ToneRx, err = parseToneFieldCell(cell, "tone_rx", true)
			return
		},
	},
	spec.FieldDTCSCode: {
		Cell: func(d codeplug.ChannelData) string { return exportIntField(d.DTCSCode) },
		Parse: func(d *codeplug.ChannelData, cell string) (err error) {
			d.DTCSCode, err = parseIntFieldCell(cell, "dtcs_code")
			return
		},
	},
	spec.FieldDTCSPolarity: {
		Cell: func(d codeplug.ChannelData) string { return exportStringField(d.DTCSPolarity) },
		Parse: func(d *codeplug.ChannelData, cell string) error {
			d.DTCSPolarity = parseStringFieldCell(cell)
			return nil
		},
	},
	spec.FieldFilter: {
		Cell: func(d codeplug.ChannelData) string { return exportStringField(d.Filter) },
		Parse: func(d *codeplug.ChannelData, cell string) error {
			d.Filter = parseStringFieldCell(cell)
			return nil
		},
	},
	spec.FieldDataMode: {
		Cell: func(d codeplug.ChannelData) string { return exportBoolField(d.DataMode, true) },
		Parse: func(d *codeplug.ChannelData, cell string) (err error) {
			d.DataMode, err = parseBoolFieldCell(cell, "data_mode", true)
			return
		},
	},
	spec.FieldTuningStepEnabled: {
		Cell: func(d codeplug.ChannelData) string { return exportBoolField(d.TuningStepEnabled, true) },
		Parse: func(d *codeplug.ChannelData, cell string) (err error) {
			d.TuningStepEnabled, err = parseBoolFieldCell(cell, "tuning_step_enabled", true)
			return
		},
	},
	spec.FieldTuningStep: {
		Cell: func(d codeplug.ChannelData) string { return exportStringField(d.TuningStep) },
		Parse: func(d *codeplug.ChannelData, cell string) error {
			d.TuningStep = parseStringFieldCell(cell)
			return nil
		},
	},
	spec.FieldProgramTuningStep: {
		Cell: func(d codeplug.ChannelData) string { return exportFreqField(d.ProgramTuningStepHz) },
		Parse: func(d *codeplug.ChannelData, cell string) (err error) {
			d.ProgramTuningStepHz, err = parseFreqFieldCell(cell, "program_tuning_step")
			return
		},
	},
	spec.FieldAttenuator: {
		Cell: func(d codeplug.ChannelData) string { return exportIntField(d.AttenuatorDB) },
		Parse: func(d *codeplug.ChannelData, cell string) (err error) {
			d.AttenuatorDB, err = parseIntFieldCell(cell, "attenuator")
			return
		},
	},
	spec.FieldPreamp: {
		Cell: func(d codeplug.ChannelData) string { return exportStringField(d.Preamp) },
		Parse: func(d *codeplug.ChannelData, cell string) error {
			d.Preamp = parseStringFieldCell(cell)
			return nil
		},
	},
	spec.FieldAntenna: {
		Cell: func(d codeplug.ChannelData) string { return exportStringField(d.Antenna) },
		Parse: func(d *codeplug.ChannelData, cell string) error {
			d.Antenna = parseStringFieldCell(cell)
			return nil
		},
	},
	spec.FieldIPPlus: {
		Cell: func(d codeplug.ChannelData) string { return exportBoolField(d.IPPlus, true) },
		Parse: func(d *codeplug.ChannelData, cell string) (err error) {
			d.IPPlus, err = parseBoolFieldCell(cell, "ip_plus", true)
			return
		},
	},
}
