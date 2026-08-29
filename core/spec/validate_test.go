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
		Model:    "TEST-710",
		CATID:    "0000",
		Transmit: HasTransmitter,
		Banks: []Bank{
			{
				ID:    BankMemory,
				Label: "Memories",
				Slots: []string{"001", "002"},
				Fields: map[Field]FieldSupport{
					FieldFrequency: {Read: Supported, Write: Supported},
					FieldMode:      {Read: Supported, Write: Unverified},
					// The two VOCABULARY fields, declared because this
					// fixture supplies both vocabularies. Since E5b the
					// "must not be empty" rules fire only for a model
					// whose banks reach the field, so a fixture that
					// omitted them would make those refusals untestable —
					// and every registered Yaesu model declares both.
					FieldShift:      {Read: Supported, Write: Unverified},
					FieldCTCSSState: {Read: Supported, Write: Unverified},
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
			// ConsentedUnverified is a DECLARED Support constant, minted by
			// the consent transform on the WRITE side only: Validate must
			// accept it there exactly like the other declared states.
			name: "FieldSupport Write ConsentedUnverified is valid",
			mutate: func(c *Capabilities) {
				c.Banks[0].Fields[FieldShift] = FieldSupport{Read: Supported, Write: ConsentedUnverified}
			},
		},
		{
			// ...and must REFUSE it on the read side: consent is a
			// write-side state, reads already flow and need no consent, so
			// a read label carrying it is a construction mistake.
			name: "FieldSupport Read ConsentedUnverified is rejected",
			mutate: func(c *Capabilities) {
				c.Banks[0].Fields[FieldShift] = FieldSupport{Read: ConsentedUnverified, Write: Supported}
			},
			wantErr: true,
			wantSub: "Read support must never be ConsentedUnverified",
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
			name: "ShiftOptions blank value",
			mutate: func(c *Capabilities) {
				c.ShiftOptions = []ShiftOption{
					{Value: "SIMPLEX", Direction: ShiftNone},
					{Value: "", Direction: ShiftUp},
					{Value: "MINUS", Direction: ShiftDown},
				}
			},
			wantSub: "ShiftOptions must not contain a blank value",
		},
		{
			name: "ShiftOptions duplicate value",
			mutate: func(c *Capabilities) {
				c.ShiftOptions = []ShiftOption{
					{Value: "SIMPLEX", Direction: ShiftNone},
					{Value: "PLUS", Direction: ShiftUp},
					{Value: "SIMPLEX", Direction: ShiftDown},
				}
			},
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
				c.CTCSSStates = []ToneState{{Value: "OFF", Semantics: ToneOff}, {Value: "", Semantics: ToneEncode}}
			},
			wantSub: "CTCSSStates must not contain a blank value",
		},
		{
			name: "CTCSSStates duplicate Value",
			mutate: func(c *Capabilities) {
				c.CTCSSStates = []ToneState{{Value: "OFF", Semantics: ToneOff}, {Value: "OFF", Semantics: ToneEncode}}
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

func TestValidate_ShiftOptionsDuplicateDirection(t *testing.T) {
	c := validTestCapabilities()
	c.ShiftOptions = []ShiftOption{
		{Value: "SIMPLEX", Direction: ShiftNone},
		{Value: "PLUS", Direction: ShiftUp},
		{Value: "UP-ALSO", Direction: ShiftUp},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error: two ShiftOptions share ShiftUp")
	}
	if !strings.Contains(err.Error(), "same direction") {
		t.Errorf("Validate() error = %q, want it to mention \"same direction\"", err)
	}
}

func TestValidate_CTCSSStatesDuplicateEncodeDecodePair(t *testing.T) {
	c := validTestCapabilities()
	c.CTCSSStates = []ToneState{
		{Value: "OFF", Semantics: ToneOff},
		{Value: "ENC", Semantics: ToneEncode},
		{Value: "ENC-AGAIN", Semantics: ToneEncode},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error: two ToneStates share the same semantics")
	}
	if !strings.Contains(err.Error(), "same semantics") {
		t.Errorf("Validate() error = %q, want it to mention \"same semantics\"", err)
	}
}

// TestValidate_TagLenNotPositive pins Validate's TagLen invariant. Task 2
// of this milestone (core/csvio/chirp.go) replaced a hardcoded 12-byte
// tag-truncation limit with caps.TagLen; without this check, a
// capabilities value that simply omits TagLen (leaving it at its zero
// value) passed Validate() outright, and CHIRP import then truncated
// EVERY channel name to "" — reported only as a non-blocking
// "approximated" loss entry, not the refusal this project's safety
// posture ("refuse, never corrupt") requires. Both the zero value and a
// negative TagLen must be rejected.
func TestValidate_TagLenNotPositive(t *testing.T) {
	for _, tagLen := range []int{0, -1} {
		c := validTestCapabilities()
		c.TagLen = tagLen
		err := c.Validate()
		if err == nil {
			t.Fatalf("Validate() = nil for TagLen %d, want an error", tagLen)
		}
		if !strings.Contains(err.Error(), "TagLen") {
			t.Errorf("Validate() error = %q, want it to mention \"TagLen\"", err)
		}
	}
}

// TestValidate_ShiftOptionZeroValueDirectionRejected is FIX A1's failing-
// first test: before this fix, ShiftNone was ShiftDirection's zero value,
// so a ShiftOption whose Direction was simply omitted from a struct
// literal (as "RADIO-UP" is here) silently read as simplex — a semantic
// value the author never wrote — and passed Validate outright because
// "SIMPLEX" was still a member of the declared vocabulary. Now the zero
// value is ShiftUnspecified, which Validate must reject.
func TestValidate_ShiftOptionZeroValueDirectionRejected(t *testing.T) {
	c := validTestCapabilities()
	c.ShiftOptions = []ShiftOption{
		{Value: "RADIO-UP"}, // Direction accidentally omitted
		{Value: "RADIO-DOWN", Direction: ShiftDown},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error: a ShiftOption with an omitted (zero-value) Direction must be rejected")
	}
	if !strings.Contains(err.Error(), "invalid Direction") {
		t.Errorf("Validate() error = %q, want it to mention \"invalid Direction\"", err)
	}
}

// TestValidate_ShiftOptionDirectionOutOfRange covers a Direction value
// outside the three declared ShiftDirection constants entirely (not just
// the zero value) — e.g. a corrupted or hand-built Capabilities.
func TestValidate_ShiftOptionDirectionOutOfRange(t *testing.T) {
	c := validTestCapabilities()
	c.ShiftOptions = []ShiftOption{
		{Value: "WEIRD", Direction: ShiftDirection(99)},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error: Direction 99 is out of range")
	}
	if !strings.Contains(err.Error(), "invalid Direction") {
		t.Errorf("Validate() error = %q, want it to mention \"invalid Direction\"", err)
	}
}

// TestValidate_CTCSSStateZeroValueSemanticsRejected is FIX A1's failing-
// first test for the CTCSS side: before this fix, (Encodes: false,
// Decodes: false) — CTCSS off — was the zero value of the old bool-triple
// shape, so a ToneState whose semantics were simply omitted (as
// "RADIO-ENC" is here) silently read as "off" and passed Validate because
// "OFF" was still a member of the declared vocabulary. Now the zero value
// is ToneSemanticsUnspecified, which Validate must reject.
func TestValidate_CTCSSStateZeroValueSemanticsRejected(t *testing.T) {
	c := validTestCapabilities()
	c.CTCSSStates = []ToneState{
		{Value: "RADIO-ENC"}, // Semantics accidentally omitted
		{Value: "RADIO-BOTH", Semantics: ToneEncodeDecode},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error: a ToneState with omitted (zero-value) Semantics must be rejected")
	}
	if !strings.Contains(err.Error(), "invalid Semantics") {
		t.Errorf("Validate() error = %q, want it to mention \"invalid Semantics\"", err)
	}
}

// TestValidate_CTCSSStateSemanticsOutOfRange covers a Semantics value
// outside the three declared ToneSemantics constants entirely.
func TestValidate_CTCSSStateSemanticsOutOfRange(t *testing.T) {
	c := validTestCapabilities()
	c.CTCSSStates = []ToneState{
		{Value: "WEIRD", Semantics: ToneSemantics(99)},
	}
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error: Semantics 99 is out of range")
	}
	if !strings.Contains(err.Error(), "invalid Semantics") {
		t.Errorf("Validate() error = %q, want it to mention \"invalid Semantics\"", err)
	}
}

// TestValidate_BlankSlotRejected is FIX A2's failing-first test: before
// this fix, a blank Bank.Slots entry passed the bank loop's duplicate-
// ownership check untouched (an empty string is a value like any other
// to that check), and core/csvio's CHIRP importer would go on to build a
// Channel{Slot: ""} for it with no blocking loss entry to catch the
// mistake.
func TestValidate_BlankSlotRejected(t *testing.T) {
	c := validTestCapabilities()
	c.Banks[0].Slots = append(c.Banks[0].Slots, "")
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error: a blank slot must be rejected")
	}
	if !strings.Contains(err.Error(), "blank slot") {
		t.Errorf("Validate() error = %q, want it to mention \"blank slot\"", err)
	}
}

// TestValidate_NonPositiveBaudRejected is FIX A3's failing-first test:
// before this fix, {Bauds: []int{0}, DefaultBaud: 0} passed Validate
// outright (0 is present in Bauds, so the old DefaultBaud-membership
// check alone did not catch it), and core/transport/port.go's
// resolveConfig then silently substituted 38400 for any non-positive
// SerialConfig.Baud — a guessed value standing in for a capability that
// was never actually validated.
func TestValidate_NonPositiveBaudRejected(t *testing.T) {
	c := validTestCapabilities()
	c.Bauds = []int{0}
	c.DefaultBaud = 0
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error: Bauds/DefaultBaud of 0 must be rejected")
	}
	if !strings.Contains(err.Error(), "Bauds contains non-positive entry") {
		t.Errorf("Validate() error = %q, want it to mention \"Bauds contains non-positive entry\"", err)
	}
	if !strings.Contains(err.Error(), "DefaultBaud 0 must be greater than zero") {
		t.Errorf("Validate() error = %q, want it to mention \"DefaultBaud 0 must be greater than zero\"", err)
	}
}

// TestValidate_NegativeBaudRejected covers a negative (not just zero)
// Bauds entry and DefaultBaud.
func TestValidate_NegativeBaudRejected(t *testing.T) {
	c := validTestCapabilities()
	c.Bauds = []int{-9600}
	c.DefaultBaud = -9600
	err := c.Validate()
	if err == nil {
		t.Fatal("Validate() = nil, want an error: a negative Bauds entry/DefaultBaud must be rejected")
	}
	if !strings.Contains(err.Error(), "Bauds contains non-positive entry -9600") {
		t.Errorf("Validate() error = %q, want it to mention the negative entry", err)
	}
}
