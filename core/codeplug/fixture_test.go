// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import "github.com/gm5dna/open-rig-programmer/core/spec"

// testCapabilities builds a small, realistic Capabilities shared by
// validate_test.go and diff_test.go: a MEM bank (001-003, "001" required),
// a NoBlank PMS pair (P1L/P1U), and a 60M bank (slot "501") that is
// read-only in the sense Diff cares about (FieldFrequency.Write is not
// Supported). Only MEM's Frequency/Mode/Clarifier/CTCSSState/Shift/Tag/
// TagDisplay/ScanSkip fields are Write:Supported — matching the project-wide stance
// that nothing is writable until proven on hardware (M5b); MEM/Erase stays
// Unverified so an erase-blocked test case has something to bite on even
// within an otherwise-writable bank.
func testCapabilities() spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	return spec.Capabilities{
		Model:         "TEST-710",
		CATID:         "0000",
		Modes:         []string{"LSB", "USB", "CW-U", "FM", "AM"},
		TagLen:        12,
		ClarMaxHz:     9990,
		ClarStepHz:    10,
		CTCSSTones:    tones[:],
		MinFreqHz:     30000,
		MaxFreqHz:     56000000,
		RequiredSlots: []string{"001"},
		ShiftOptions:  spec.StandardShiftOptions(),
		CTCSSStates:   spec.StandardCTCSSStates(),
		Banks: []spec.Bank{
			{
				ID:    spec.BankMemory,
				Label: "Memories",
				Slots: []string{"001", "002", "003"},
				Fields: map[spec.Field]spec.FieldSupport{
					spec.FieldFrequency:  {Read: spec.Supported, Write: spec.Supported},
					spec.FieldMode:       {Read: spec.Supported, Write: spec.Supported},
					spec.FieldClarifier:  {Read: spec.Supported, Write: spec.Supported},
					spec.FieldCTCSSState: {Read: spec.Supported, Write: spec.Supported},
					spec.FieldCTCSSTone:  {Read: spec.Supported, Write: spec.Unverified},
					spec.FieldShift:      {Read: spec.Supported, Write: spec.Supported},
					spec.FieldTag:        {Read: spec.Supported, Write: spec.Supported},
					spec.FieldTagDisplay: {Read: spec.Supported, Write: spec.Supported},
					spec.FieldScanSkip:   {Read: spec.Supported, Write: spec.Supported},
					spec.FieldErase:      {Read: spec.Unsupported, Write: spec.Unverified},
				},
			},
			{
				ID:      spec.BankPMS,
				Label:   "Scan limits (PMS)",
				Slots:   []string{"P1L", "P1U"},
				NoBlank: true,
				Fields: map[spec.Field]spec.FieldSupport{
					spec.FieldFrequency: {Read: spec.Supported, Write: spec.Unverified},
				},
			},
			{
				ID:    spec.Bank60m,
				Label: "60 m",
				Slots: []string{"501"},
				Fields: map[spec.Field]spec.FieldSupport{
					spec.FieldFrequency: {Read: spec.Supported, Write: spec.Unverified},
				},
			},
		},
	}
}

// testBaselineCodeplug builds a fresh, valid *Codeplug matching
// testCapabilities(): "001" (required) and "002" are populated MEM
// channels, "003" is an empty MEM channel, both PMS slots (NoBlank) are
// populated, and the 60M slot "501" is populated. Every populated channel
// uses CTCSS "OFF" (so the CTCSS-tone-pairing warning never fires here)
// and otherwise-valid field values, so Validate(testBaselineCodeplug(),
// testCapabilities()) returns zero issues — callers mutate a fresh copy to
// exercise one rule at a time.
//
// Every call returns an independently-allocated value (fresh slices and
// ChannelData pointers), so callers may mutate the result freely without
// affecting any other call's result.
func testBaselineCodeplug() *Codeplug {
	return &Codeplug{
		Schema:    CurrentSchema,
		Generator: "test",
		Radio:     RadioInfo{Model: "TEST-710", CATID: "0000"},
		Channels: []Channel{
			{
				Slot: "001",
				Data: &ChannelData{
					FreqHz:    14250000,
					Mode:      "USB",
					CTCSS:     "OFF",
					CTCSSTone: ToneField{State: Unknown},
					Shift:     "SIMPLEX",
					Tag:       "CALLING",
					ScanSkip:  BoolField{State: Known, Value: false},
				},
			},
			{
				Slot: "002",
				Data: &ChannelData{
					FreqHz:    14300000,
					Mode:      "LSB",
					CTCSS:     "OFF",
					CTCSSTone: ToneField{State: Unknown},
					Shift:     "SIMPLEX",
					Tag:       "NET",
					ScanSkip:  BoolField{State: Known, Value: false},
				},
			},
			{
				Slot: "003",
				// Empty: Data == nil.
			},
			{
				Slot: "P1L",
				Data: &ChannelData{
					FreqHz:    14000000,
					Mode:      "USB",
					CTCSS:     "OFF",
					CTCSSTone: ToneField{State: Unknown},
					Shift:     "SIMPLEX",
					ScanSkip:  BoolField{State: Known, Value: false},
				},
			},
			{
				Slot: "P1U",
				Data: &ChannelData{
					FreqHz:    14350000,
					Mode:      "USB",
					CTCSS:     "OFF",
					CTCSSTone: ToneField{State: Unknown},
					Shift:     "SIMPLEX",
					ScanSkip:  BoolField{State: Known, Value: false},
				},
			},
			{
				Slot: "501",
				Data: &ChannelData{
					FreqHz:    5330500,
					Mode:      "USB",
					CTCSS:     "OFF",
					CTCSSTone: ToneField{State: Unknown},
					Shift:     "SIMPLEX",
					Tag:       "5MHZ-1",
					ScanSkip:  BoolField{State: Known, Value: false},
				},
			},
		},
	}
}
