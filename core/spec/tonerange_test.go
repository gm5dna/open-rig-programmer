// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"strings"
	"testing"
)

// TestAdmitsTone_IsTheOneSharedPredicate walks the three shapes a radio's
// tone domain can take. It is the predicate BOTH codeplug.ToneField.Valid
// and core/csvio's CHIRP import consult, which is the whole point of it
// existing: before it, each of those had its own copy of "is t in
// caps.CTCSSTones", so a radio declaring a RANGE would have been refused
// by two independently-written loops neither of which knew ranges existed.
func TestAdmitsTone_IsTheOneSharedPredicate(t *testing.T) {
	list := Capabilities{CTCSSTones: []Tone{670, 693, 719}}
	rng := Capabilities{CTCSSToneRange: &ToneRange{MinDeciHz: 670, MaxDeciHz: 2541, StepDeciHz: 1}}
	coarse := Capabilities{CTCSSToneRange: &ToneRange{MinDeciHz: 670, MaxDeciHz: 690, StepDeciHz: 10}}

	cases := []struct {
		name string
		caps Capabilities
		tone Tone
		want bool
	}{
		{"list: a listed tone", list, 693, true},
		{"list: an unlisted tone", list, 700, false},
		{"list: below the list", list, 600, false},
		{"range: the lower bound", rng, 670, true},
		{"range: the upper bound", rng, 2541, true},
		{"range: inside", rng, 1000, true},
		{"range: below", rng, 669, false},
		{"range: above", rng, 2542, false},
		{"coarse range: on a step", coarse, 680, true},
		{"coarse range: off a step", coarse, 675, false},

		// FAIL-CLOSED, and pinned: a radio declaring NEITHER a list nor a
		// range admits NOTHING. "No chart known" must never become "no
		// chart needed" — this project's refuse-never-corrupt posture,
		// and the behaviour codeplug.ToneField.Valid has had all along
		// for an empty CTCSSTones.
		{"neither declared: fail-closed", Capabilities{}, 670, false},
		{"neither declared: zero tone too", Capabilities{}, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.caps.AdmitsTone(tc.tone); got != tc.want {
				t.Errorf("AdmitsTone(%v) = %v, want %v", tc.tone, got, tc.want)
			}
		})
	}
}

// TestValidate_ToneRangeRules covers every refusal the range shape adds.
// Each is a shape a driver author could plausibly write and each would,
// unrefused, make AdmitsTone answer nonsense.
func TestValidate_ToneRangeRules(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(c *Capabilities)
		wantSub string
	}{
		{
			// BOTH is the important one. Two declarations of the same
			// domain can disagree, and a predicate consulting one of them
			// would silently pick a winner.
			name: "both a list and a range",
			mutate: func(c *Capabilities) {
				c.CTCSSToneRange = &ToneRange{MinDeciHz: 670, MaxDeciHz: 2541, StepDeciHz: 1}
			},
			wantSub: "a list OR a range, never both",
		},
		{
			name: "inverted bounds",
			mutate: func(c *Capabilities) {
				c.CTCSSTones = nil
				c.CTCSSToneRange = &ToneRange{MinDeciHz: 2541, MaxDeciHz: 670, StepDeciHz: 1}
			},
			wantSub: "MinDeciHz",
		},
		{
			name: "non-positive minimum",
			mutate: func(c *Capabilities) {
				c.CTCSSTones = nil
				c.CTCSSToneRange = &ToneRange{MinDeciHz: 0, MaxDeciHz: 2541, StepDeciHz: 1}
			},
			wantSub: "must be greater than zero",
		},
		{
			name: "non-positive maximum",
			mutate: func(c *Capabilities) {
				c.CTCSSTones = nil
				c.CTCSSToneRange = &ToneRange{MinDeciHz: 670, MaxDeciHz: -1, StepDeciHz: 1}
			},
			wantSub: "must be greater than zero",
		},
		{
			name: "non-positive step",
			mutate: func(c *Capabilities) {
				c.CTCSSTones = nil
				c.CTCSSToneRange = &ToneRange{MinDeciHz: 670, MaxDeciHz: 2541, StepDeciHz: 0}
			},
			wantSub: "StepDeciHz",
		},
		{
			// A declared maximum the step can never land on is an author
			// stating a bound their own radio does not have.
			name: "a maximum no whole number of steps reaches",
			mutate: func(c *Capabilities) {
				c.CTCSSTones = nil
				c.CTCSSToneRange = &ToneRange{MinDeciHz: 670, MaxDeciHz: 685, StepDeciHz: 10}
			},
			wantSub: "whole number of StepDeciHz",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validTestCapabilities()
			tc.mutate(&c)
			err := c.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want a problem mentioning %q", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("Validate() = %v, want a problem mentioning %q", err, tc.wantSub)
			}
		})
	}
}

// TestValidate_AWellFormedToneRangeIsAccepted is the positive half: the
// rules above must refuse the broken shapes and nothing else.
func TestValidate_AWellFormedToneRangeIsAccepted(t *testing.T) {
	c := validTestCapabilities()
	c.CTCSSTones = nil
	c.CTCSSToneRange = &ToneRange{MinDeciHz: 670, MaxDeciHz: 2541, StepDeciHz: 1}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil for a radio declaring a range and no list", err)
	}
}

// TestValidate_DTCSCodesRemainATable pins the adjudication's other half:
// DTCS is NOT given a range shape. The 512 codes 000..777 are the octal-
// looking set where every digit is 7 or less, which is not contiguous —
// 008 through 077 are not codes at all — so no min/max/step can describe
// it and models supply the table.
func TestValidate_DTCSCodesRemainATable(t *testing.T) {
	c := validTestCapabilities()
	c.DTCSCodes = []int{23, 25, 26, 31}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	// The gap the table exists to express: 24 is not a DTCS code, and a
	// contiguous range from 23 to 31 would have admitted it.
	for _, notACode := range []int{24, 27, 28, 29, 30} {
		for _, got := range c.DTCSCodes {
			if got == notACode {
				t.Fatalf("fixture error: %d is in the table", notACode)
			}
		}
	}
}
