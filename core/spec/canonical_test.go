// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"strings"
	"testing"
)

// icomVocabCaps is a Capabilities carrying the ICOM half of both
// vocabulary pairs and neither Yaesu half — the shape E5's rules are
// about, and one no registered model has yet.
func icomVocabCaps() Capabilities {
	c := validTestCapabilities()
	c.ShiftOptions = nil
	c.CTCSSStates = nil
	c.Banks[0].Fields[FieldDuplex] = FieldSupport{Read: Supported, Write: Unverified}
	c.Banks[0].Fields[FieldToneMode] = FieldSupport{Read: Supported, Write: Unverified}
	delete(c.Banks[0].Fields, FieldShift)
	delete(c.Banks[0].Fields, FieldCTCSSState)
	c.DuplexOptions = []DuplexOption{
		{Value: "OFF", Direction: DuplexOff, Canonical: true},
		{Value: "DUP-", Direction: DuplexDown, Canonical: true},
		{Value: "DUP+", Direction: DuplexUp, Canonical: true},
	}
	c.ToneModes = []ToneMode{
		{Value: "OFF", Semantics: ToneModeOff, Canonical: true},
		{Value: "TONE", Semantics: ToneModeCTCSS, Canonical: true},
		{Value: "TSQL", Semantics: ToneModeCTCSSSquelch, Canonical: true},
	}
	return c
}

// TestValidate_DuplicatedSemanticsNeedExactlyOneCanonical is E5's decided
// rule. A radio MAY express one semantic value with more than one wire
// code — a model with both "DUP-" and a band-specific "DUP-(auto)" is real
// — but the reverse question csvio asks ("which wire code means DOWN?")
// must have exactly one answer, and before this it was answered by
// whichever entry came first in the slice.
func TestValidate_DuplicatedSemanticsNeedExactlyOneCanonical(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(c *Capabilities)
		wantSub string // empty means "must validate"
	}{
		{
			name: "two codes for one direction, exactly one canonical",
			mutate: func(c *Capabilities) {
				c.DuplexOptions = append(c.DuplexOptions, DuplexOption{Value: "DUP-A", Direction: DuplexDown})
			},
		},
		{
			name: "two codes for one direction, neither canonical",
			mutate: func(c *Capabilities) {
				c.DuplexOptions = []DuplexOption{
					{Value: "OFF", Direction: DuplexOff, Canonical: true},
					{Value: "DUP-", Direction: DuplexDown},
					{Value: "DUP-A", Direction: DuplexDown},
				}
			},
			wantSub: "no canonical",
		},
		{
			name: "two codes for one direction, both canonical",
			mutate: func(c *Capabilities) {
				c.DuplexOptions = []DuplexOption{
					{Value: "OFF", Direction: DuplexOff, Canonical: true},
					{Value: "DUP-", Direction: DuplexDown, Canonical: true},
					{Value: "DUP-A", Direction: DuplexDown, Canonical: true},
				}
			},
			wantSub: "more than one canonical",
		},
		{
			name: "a lone entry needs no canonical marking",
			mutate: func(c *Capabilities) {
				c.DuplexOptions = []DuplexOption{
					{Value: "OFF", Direction: DuplexOff},
					{Value: "DUP-", Direction: DuplexDown},
				}
			},
		},
		{
			name: "two tone modes for one semantic, exactly one canonical",
			mutate: func(c *Capabilities) {
				c.ToneModes = append(c.ToneModes, ToneMode{Value: "TONE-B", Semantics: ToneModeCTCSS})
			},
		},
		{
			name: "two tone modes for one semantic, neither canonical",
			mutate: func(c *Capabilities) {
				c.ToneModes = []ToneMode{
					{Value: "OFF", Semantics: ToneModeOff, Canonical: true},
					{Value: "TONE", Semantics: ToneModeCTCSS},
					{Value: "TONE-B", Semantics: ToneModeCTCSS},
				}
			},
			wantSub: "no canonical",
		},
		{
			name: "two tone modes for one semantic, both canonical",
			mutate: func(c *Capabilities) {
				c.ToneModes = []ToneMode{
					{Value: "OFF", Semantics: ToneModeOff, Canonical: true},
					{Value: "TONE", Semantics: ToneModeCTCSS, Canonical: true},
					{Value: "TONE-B", Semantics: ToneModeCTCSS, Canonical: true},
				}
			},
			wantSub: "more than one canonical",
		},
		{
			// Unique WIRE CODES are still the generic vocabulary rule's
			// job, and multiplicity of SEMANTICS does not relax it.
			name: "duplicated wire codes are still refused",
			mutate: func(c *Capabilities) {
				c.DuplexOptions = append(c.DuplexOptions, DuplexOption{Value: "DUP-", Direction: DuplexDown})
			},
			wantSub: `DuplexOptions contains duplicate value "DUP-"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := icomVocabCaps()
			tc.mutate(&c)
			err := c.Validate()
			if tc.wantSub == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want a problem mentioning %q", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("Validate() = %v, want a problem mentioning %q", err, tc.wantSub)
			}
		})
	}
}

// TestValidate_ABankWithNoShiftVocabularyIsAdmitted is E5b. A model whose
// memory bank legitimately carries NO repeater shift or duplex field at
// all — the IC-7300's and IC-7610's HF banks, per those plans' reviews —
// was refused outright by "ShiftOptions must not be empty", a rule written
// when every registered radio had one.
//
// THE YAESU PROTECTION IS NOT WEAKENED, and the second half of each case
// below is what says so: a model that DOES declare the field and supplies
// no vocabulary for it is still refused. Fail-closed is preserved through
// the field's own support grades — a bank that reaches the field must name
// the values it can hold.
func TestValidate_ABankWithNoShiftVocabularyIsAdmitted(t *testing.T) {
	t.Run("no bank reaches the shift or duplex field: admitted", func(t *testing.T) {
		c := validTestCapabilities()
		c.ShiftOptions = nil
		c.DuplexOptions = nil
		delete(c.Banks[0].Fields, FieldShift)
		delete(c.Banks[0].Fields, FieldDuplex)
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil for a model whose banks carry no shift or duplex field", err)
		}
	})

	t.Run("a bank reaches FieldShift with no vocabulary: refused", func(t *testing.T) {
		c := validTestCapabilities()
		c.ShiftOptions = nil
		c.DuplexOptions = nil
		c.Banks[0].Fields[FieldShift] = FieldSupport{Read: Supported, Write: Unverified}
		err := c.Validate()
		if err == nil || !strings.Contains(err.Error(), "ShiftOptions must not be empty") {
			t.Fatalf("Validate() = %v, want the empty-vocabulary refusal for a bank that reaches FieldShift", err)
		}
	})

	t.Run("a bank reaches FieldDuplex with no vocabulary: refused", func(t *testing.T) {
		c := validTestCapabilities()
		c.ShiftOptions = nil
		c.DuplexOptions = nil
		delete(c.Banks[0].Fields, FieldShift)
		c.Banks[0].Fields[FieldDuplex] = FieldSupport{Read: Supported, Write: Unverified}
		err := c.Validate()
		if err == nil || !strings.Contains(err.Error(), "ShiftOptions must not be empty") {
			t.Fatalf("Validate() = %v, want the empty-vocabulary refusal for a bank that reaches FieldDuplex", err)
		}
	})

	t.Run("an Unsupported-both-ways field does not demand a vocabulary", func(t *testing.T) {
		c := validTestCapabilities()
		c.ShiftOptions = nil
		c.DuplexOptions = nil
		delete(c.Banks[0].Fields, FieldShift)
		// Present in the map, but Unreachable: the shape a driver uses to
		// say "this radio has no such thing".
		c.Banks[0].Fields[FieldDuplex] = FieldSupport{Read: Unsupported, Write: Unsupported}
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil — an Unreachable field is not a field the bank carries", err)
		}
	})
}

// TestValidate_ABankWithNoToneVocabularyIsAdmitted is E5b's other half,
// on the CTCSSStates/ToneModes pair, with the same protection intact.
func TestValidate_ABankWithNoToneVocabularyIsAdmitted(t *testing.T) {
	t.Run("no bank reaches the tone field: admitted", func(t *testing.T) {
		c := validTestCapabilities()
		c.CTCSSStates = nil
		c.ToneModes = nil
		delete(c.Banks[0].Fields, FieldCTCSSState)
		delete(c.Banks[0].Fields, FieldToneMode)
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})

	t.Run("a bank reaches FieldCTCSSState with no vocabulary: refused", func(t *testing.T) {
		c := validTestCapabilities()
		c.CTCSSStates = nil
		c.ToneModes = nil
		c.Banks[0].Fields[FieldCTCSSState] = FieldSupport{Read: Supported, Write: Unverified}
		err := c.Validate()
		if err == nil || !strings.Contains(err.Error(), "CTCSSStates must not be empty") {
			t.Fatalf("Validate() = %v, want the empty-vocabulary refusal", err)
		}
	})

	t.Run("a bank reaches FieldToneMode with no vocabulary: refused", func(t *testing.T) {
		c := validTestCapabilities()
		c.CTCSSStates = nil
		c.ToneModes = nil
		delete(c.Banks[0].Fields, FieldCTCSSState)
		c.Banks[0].Fields[FieldToneMode] = FieldSupport{Read: Supported, Write: Unverified}
		err := c.Validate()
		if err == nil || !strings.Contains(err.Error(), "CTCSSStates must not be empty") {
			t.Fatalf("Validate() = %v, want the empty-vocabulary refusal", err)
		}
	})
}
