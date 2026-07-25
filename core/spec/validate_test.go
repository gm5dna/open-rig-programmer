// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"strings"
	"testing"
)

// validTestCapabilities builds a small, fully self-consistent
// Capabilities that Validate() accepts outright — every test below
// mutates a fresh copy to break exactly one rule at a time. Every call
// returns independently-allocated slices/maps, so callers may mutate the
// result freely.
func validTestCapabilities() Capabilities {
	return Capabilities{
		Model: "TEST-710",
		CATID: "0000",
		Banks: []Bank{
			{
				ID:    BankMemory,
				Label: "Memories",
				Slots: []string{"001", "002"},
				Fields: map[Field]FieldSupport{
					FieldFrequency: {Read: Supported, Write: Supported},
					FieldMode:      {Read: Supported, Write: Unverified},
				},
			},
			{
				ID:      BankPMS,
				Label:   "Scan limits (PMS)",
				Slots:   []string{"P1L", "P1U"},
				NoBlank: true,
				Fields: map[Field]FieldSupport{
					FieldFrequency: {Read: Supported, Write: Unverified},
				},
			},
		},
		Modes:        []string{"USB", "LSB"},
		TagLen:       12,
		ClarMaxHz:    9990,
		ClarStepHz:   10,
		CTCSSTones:   []Tone{670, 693, 719},
		Bauds:        []int{4800, 9600, 38400},
		DefaultBaud:  9600,
		MinFreqHz:    30000,
		MaxFreqHz:    56000000,
		ShiftOptions: StandardShiftOptions(),
		CTCSSStates:  StandardCTCSSStates(),
	}
}

// TestCapabilitiesValidate is table-driven over every structural rule
// Validate checks, one broken per case, plus the happy-path baseline.
func TestCapabilitiesValidate(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(c *Capabilities)
		wantErr bool
		wantSub string
	}{
		{
			name:   "valid capabilities has no error",
			mutate: func(c *Capabilities) {},
		},
		{
			name:    "empty Model",
			mutate:  func(c *Capabilities) { c.Model = "" },
			wantErr: true,
			wantSub: "Model",
		},
		{
			name:    "empty CATID",
			mutate:  func(c *Capabilities) { c.CATID = "" },
			wantErr: true,
			wantSub: "CATID",
		},
		{
			name: "duplicate BankID",
			mutate: func(c *Capabilities) {
				c.Banks = append(c.Banks, Bank{ID: BankMemory, Label: "duplicate", Slots: []string{"999"}})
			},
			wantErr: true,
			wantSub: "duplicate BankID",
		},
		{
			name: "slot claimed by two banks",
			mutate: func(c *Capabilities) {
				c.Banks[1].Slots = append(c.Banks[1].Slots, "001")
			},
			wantErr: true,
			wantSub: "claimed by both",
		},
		{
			// Inert is a DECLARED Support constant (M5b, HW-CONFIRMED: the
			// FT-710's clarifier is transmitted but ignored on write) —
			// Validate must accept it, on Read and Write alike, exactly
			// like the other three declared states.
			name: "FieldSupport Write Inert is valid",
			mutate: func(c *Capabilities) {
				c.Banks[0].Fields[FieldShift] = FieldSupport{Read: Supported, Write: Inert}
			},
		},
		{
			name: "FieldSupport Read out of range",
			mutate: func(c *Capabilities) {
				c.Banks[0].Fields[FieldShift] = FieldSupport{Read: Support(99), Write: Supported}
			},
			wantErr: true,
			wantSub: "out of range",
		},
		{
			name: "FieldSupport Write out of range",
			mutate: func(c *Capabilities) {
				c.Banks[0].Fields[FieldShift] = FieldSupport{Read: Supported, Write: Support(-1)}
			},
			wantErr: true,
			wantSub: "out of range",
		},
		{
			name: "MinFreqHz greater than MaxFreqHz, both set",
			mutate: func(c *Capabilities) {
				c.MinFreqHz = 100
				c.MaxFreqHz = 50
			},
			wantErr: true,
			wantSub: "MinFreqHz",
		},
		{
			name: "MinFreqHz greater than MaxFreqHz but MaxFreqHz unset is not checked",
			mutate: func(c *Capabilities) {
				c.MinFreqHz = 100
				c.MaxFreqHz = 0
			},
			wantErr: false,
		},
		{
			name: "MinFreqHz greater than MaxFreqHz but MinFreqHz unset is not checked",
			mutate: func(c *Capabilities) {
				c.MinFreqHz = 0
				c.MaxFreqHz = 50
			},
			wantErr: false,
		},
		{
			name:    "DefaultBaud not present in Bauds",
			mutate:  func(c *Capabilities) { c.DefaultBaud = 12345 },
			wantErr: true,
			wantSub: "DefaultBaud",
		},
		{
			name:    "CTCSSTones with a duplicate is not strictly ascending",
			mutate:  func(c *Capabilities) { c.CTCSSTones = []Tone{670, 670, 719} },
			wantErr: true,
			wantSub: "ascending",
		},
		{
			name:    "CTCSSTones descending",
			mutate:  func(c *Capabilities) { c.CTCSSTones = []Tone{719, 693, 670} },
			wantErr: true,
			wantSub: "ascending",
		},
		{
			name:    "CTCSSTones empty is fine (not checked)",
			mutate:  func(c *Capabilities) { c.CTCSSTones = nil },
			wantErr: false,
		},
		{
			name:    "CTCSSTones single entry is trivially ascending",
			mutate:  func(c *Capabilities) { c.CTCSSTones = []Tone{670} },
			wantErr: false,
		},
		{
			name: "RequiredSlot not found in any bank",
			mutate: func(c *Capabilities) {
				c.RequiredSlots = []string{"001", "999"}
			},
			wantErr: true,
			wantSub: "RequiredSlot \"999\"",
		},
		{
			name: "all RequiredSlots found in banks",
			mutate: func(c *Capabilities) {
				c.RequiredSlots = []string{"001", "P1L"}
			},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caps := validTestCapabilities()
			tc.mutate(&caps)
			err := caps.Validate()
			if !tc.wantErr {
				if err != nil {
					t.Errorf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("Validate() error = %q, want substring %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// TestCapabilitiesValidate_RequiresVocab covers Validate's ShiftOptions
// and CTCSSStates rules specifically: both must be non-empty, and each
// rejects a blank value and a duplicate value. validTestCapabilities'
// baseline already carries StandardShiftOptions()/StandardCTCSSStates(),
// so every case here mutates a fresh copy to break exactly one rule.
func TestCapabilitiesValidate_RequiresVocab(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(c *Capabilities)
		wantSub string
	}{
		{
			name:    "ShiftOptions empty",
			mutate:  func(c *Capabilities) { c.ShiftOptions = nil },
			wantSub: "ShiftOptions must not be empty",
		},
		{
			name:    "ShiftOptions blank value",
			mutate:  func(c *Capabilities) { c.ShiftOptions = []string{"SIMPLEX", "", "MINUS"} },
			wantSub: "ShiftOptions must not contain a blank value",
		},
		{
			name:    "ShiftOptions duplicate value",
			mutate:  func(c *Capabilities) { c.ShiftOptions = []string{"SIMPLEX", "PLUS", "SIMPLEX"} },
			wantSub: `ShiftOptions contains duplicate value "SIMPLEX"`,
		},
		{
			name:    "CTCSSStates empty",
			mutate:  func(c *Capabilities) { c.CTCSSStates = nil },
			wantSub: "CTCSSStates must not be empty",
		},
		{
			name: "CTCSSStates blank Value",
			mutate: func(c *Capabilities) {
				c.CTCSSStates = []ToneState{{Value: "OFF"}, {Value: "", RequiresTone: true}}
			},
			wantSub: "CTCSSStates must not contain a blank value",
		},
		{
			name: "CTCSSStates duplicate Value",
			mutate: func(c *Capabilities) {
				c.CTCSSStates = []ToneState{{Value: "OFF"}, {Value: "OFF", RequiresTone: true}}
			},
			wantSub: `CTCSSStates contains duplicate value "OFF"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caps := validTestCapabilities()
			tc.mutate(&caps)
			err := caps.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("Validate() error = %q, want substring %q", err.Error(), tc.wantSub)
			}
		})
	}
}
