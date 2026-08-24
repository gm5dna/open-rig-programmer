// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"strings"
	"testing"
)

// TestSparseSlot_RoundTrip pins the canonical group-addressed slot form
// (design D4: "G05-012") and its strict parse.
func TestSparseSlot_RoundTrip(t *testing.T) {
	for _, tt := range []struct {
		group, channel int
		want           string
	}{
		{1, 1, "G01-001"},
		{5, 12, "G05-012"},
		{100, 100, "G100-100"},
		{99, 999, "G99-999"},
	} {
		got := SparseSlot(tt.group, tt.channel)
		if got != tt.want {
			t.Errorf("SparseSlot(%d, %d) = %q, want %q", tt.group, tt.channel, got, tt.want)
		}
		g, c, ok := ParseSparseSlot(got)
		if !ok || g != tt.group || c != tt.channel {
			t.Errorf("ParseSparseSlot(%q) = (%d, %d, %v), want (%d, %d, true)", got, g, c, ok, tt.group, tt.channel)
		}
	}
}

// TestParseSparseSlot_RefusesNonCanonical: an alternative spelling of the
// same address must be refused, not silently accepted as a second name
// for one slot.
func TestParseSparseSlot_RefusesNonCanonical(t *testing.T) {
	for _, s := range []string{
		"", "G", "G05", "05-012", "G5-12", "G005-0012", "G05-12", "Gxx-012",
		"G05-abc", "g05-012", "G05-012-3", "G-012", "G05-",
	} {
		if _, _, ok := ParseSparseSlot(s); ok {
			t.Errorf("ParseSparseSlot(%q) accepted a non-canonical form", s)
		}
	}
}

// TestBankWithinSpace_DenseIsMembershipOnly: a dense bank answers exactly
// the question it always has — is this slot in Slots — and a
// group-address string is not in its space just because it parses.
func TestBankWithinSpace_DenseIsMembershipOnly(t *testing.T) {
	b := Bank{ID: BankMemory, Slots: []string{"001", "002"}}
	if !b.WithinSpace("001") {
		t.Error("WithinSpace(\"001\") = false, want true for a listed slot")
	}
	if b.WithinSpace("003") {
		t.Error("WithinSpace(\"003\") = true, want false for an unlisted slot on a dense bank")
	}
	if b.WithinSpace("G01-001") {
		t.Error("WithinSpace(\"G01-001\") = true, want false: a dense bank has no addressable space beyond Slots")
	}
}

// TestBankWithinSpace_Sparse: a sparse bank admits its materialised slots
// AND every well-formed address inside Groups x PerGroup, and nothing
// outside it.
func TestBankWithinSpace_Sparse(t *testing.T) {
	b := Bank{
		ID: BankMemory, Slots: []string{"G01-001"},
		Sparse: true, Groups: 100, PerGroup: 100, Budget: 500,
	}
	for _, in := range []string{"G01-001", "G01-002", "G100-100", "G50-050"} {
		if !b.WithinSpace(in) {
			t.Errorf("WithinSpace(%q) = false, want true", in)
		}
	}
	for _, out := range []string{"G00-001", "G01-000", "G101-001", "G01-101", "001", "P1L"} {
		if b.WithinSpace(out) {
			t.Errorf("WithinSpace(%q) = true, want false", out)
		}
	}
}

// TestValidate_SparseDescriptorRules pins both halves of the symmetric
// rule: the three descriptor fields are legal only WITH Sparse, and must
// all be zero without it.
func TestValidate_SparseDescriptorRules(t *testing.T) {
	base := func(b Bank) Capabilities {
		c := minimalCaps()
		c.Banks = []Bank{b}
		return c
	}
	for _, tt := range []struct {
		name string
		bank Bank
		want string // substring; "" means no error
	}{
		{
			name: "sparse with a complete descriptor is valid",
			bank: Bank{ID: BankMemory, Slots: []string{"G01-001"}, Sparse: true, Groups: 100, PerGroup: 100, Budget: 500},
		},
		{
			name: "sparse without Groups",
			bank: Bank{ID: BankMemory, Sparse: true, PerGroup: 100, Budget: 500},
			want: "must have Groups greater than zero",
		},
		{
			name: "sparse without PerGroup",
			bank: Bank{ID: BankMemory, Sparse: true, Groups: 100, Budget: 500},
			want: "must have PerGroup greater than zero",
		},
		{
			name: "sparse without Budget",
			bank: Bank{ID: BankMemory, Sparse: true, Groups: 100, PerGroup: 100},
			want: "must have Budget greater than zero",
		},
		{
			name: "Groups on a dense bank",
			bank: Bank{ID: BankMemory, Slots: []string{"001"}, Groups: 100},
			want: "Groups 100 is set on a bank that is not Sparse",
		},
		{
			name: "Budget on a dense bank",
			bank: Bank{ID: BankMemory, Slots: []string{"001"}, Budget: 500},
			want: "Budget 500 is set on a bank that is not Sparse",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := base(tt.bank).Validate()
			if tt.want == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() = %v, want an error containing %q", err, tt.want)
			}
		})
	}
}

// TestValidate_VocabularyPairs: the non-empty rule applies to each PAIR
// (Yaesu shift/CTCSS-state or Icom duplex/tone-mode), never to one half
// unconditionally — and the message for "neither" is the same one the
// unconditional rule produced, so a caller matching on it is unaffected.
func TestValidate_VocabularyPairs(t *testing.T) {
	t.Run("neither half of the shift pair", func(t *testing.T) {
		c := minimalCaps()
		c.ShiftOptions = nil
		err := c.Validate()
		if err == nil || !strings.Contains(err.Error(), "ShiftOptions must not be empty") {
			t.Fatalf("Validate() = %v, want an error containing %q", err, "ShiftOptions must not be empty")
		}
	})
	t.Run("Icom half alone satisfies the shift pair", func(t *testing.T) {
		c := minimalCaps()
		c.ShiftOptions = nil
		c.DuplexOptions = []DuplexOption{
			{Value: "OFF", Direction: DuplexOff},
			{Value: "DUP+", Direction: DuplexUp},
			{Value: "DUP-", Direction: DuplexDown},
		}
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})
	t.Run("neither half of the tone pair", func(t *testing.T) {
		c := minimalCaps()
		c.CTCSSStates = nil
		err := c.Validate()
		if err == nil || !strings.Contains(err.Error(), "CTCSSStates must not be empty") {
			t.Fatalf("Validate() = %v, want an error containing %q", err, "CTCSSStates must not be empty")
		}
	})
	t.Run("Icom half alone satisfies the tone pair", func(t *testing.T) {
		c := minimalCaps()
		c.CTCSSStates = nil
		c.ToneModes = []ToneMode{
			{Value: "OFF", Semantics: ToneModeOff},
			{Value: "TONE", Semantics: ToneModeCTCSS},
			{Value: "TSQL", Semantics: ToneModeCTCSSSquelch},
			{Value: "DTCS", Semantics: ToneModeDTCS},
		}
		if err := c.Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})
}

// TestValidate_TierVocabularyConsistency: when a tier vocabulary IS
// supplied it gets the same blank/duplicate/semantics rules every other
// vocabulary gets.
func TestValidate_TierVocabularyConsistency(t *testing.T) {
	for _, tt := range []struct {
		name  string
		mutin func(*Capabilities)
		want  string
	}{
		{
			name:  "blank duplex value",
			mutin: func(c *Capabilities) { c.DuplexOptions = []DuplexOption{{Value: "", Direction: DuplexOff}} },
			want:  "DuplexOptions must not contain a blank value",
		},
		{
			name: "duplicate duplex value",
			mutin: func(c *Capabilities) {
				c.DuplexOptions = []DuplexOption{{Value: "OFF", Direction: DuplexOff}, {Value: "OFF", Direction: DuplexUp}}
			},
			want: "DuplexOptions contains duplicate value \"OFF\"",
		},
		{
			name:  "unspecified duplex direction",
			mutin: func(c *Capabilities) { c.DuplexOptions = []DuplexOption{{Value: "OFF"}} },
			want:  "DuplexOptions \"OFF\" has invalid Direction 0",
		},
		{
			name: "two duplex options with one direction",
			mutin: func(c *Capabilities) {
				c.DuplexOptions = []DuplexOption{{Value: "OFF", Direction: DuplexOff}, {Value: "SIMPLEX", Direction: DuplexOff}}
			},
			// SINCE E5 this is the canonical rule's failure, not a
			// flat refusal of multiplicity: two codes for one direction
			// are allowed, but one of them must say it is the answer.
			want: "no canonical one is marked",
		},
		{
			name:  "unspecified tone-mode semantics",
			mutin: func(c *Capabilities) { c.ToneModes = []ToneMode{{Value: "OFF"}} },
			want:  "ToneModes \"OFF\" has invalid Semantics 0",
		},
		{
			name: "two tone modes with one semantics",
			mutin: func(c *Capabilities) {
				c.ToneModes = []ToneMode{{Value: "OFF", Semantics: ToneModeOff}, {Value: "NONE", Semantics: ToneModeOff}}
			},
			want: "no canonical one is marked",
		},
		{
			name:  "blank DTCS polarity",
			mutin: func(c *Capabilities) { c.DTCSPolarities = []string{"NN", ""} },
			want:  "DTCSPolarities must not contain a blank value",
		},
		{
			name:  "duplicate filter",
			mutin: func(c *Capabilities) { c.Filters = []string{"FIL1", "FIL1"} },
			want:  "Filters contains duplicate value \"FIL1\"",
		},
		{
			name:  "DTCS codes out of order",
			mutin: func(c *Capabilities) { c.DTCSCodes = []int{23, 23} },
			want:  "DTCSCodes is not strictly ascending",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c := minimalCaps()
			tt.mutin(&c)
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() = %v, want an error containing %q", err, tt.want)
			}
		})
	}
}

// TestTagByteOK_DefaultAndSupplied: an unsupplied charset is the family
// default this project has always applied (printable ASCII excluding
// ';'), and a supplied one is exactly the set given — notably able to
// EXCLUDE the space, which the Icom charset tables do.
func TestTagByteOK_DefaultAndSupplied(t *testing.T) {
	def := Capabilities{}
	for _, b := range []byte{0x20, 'A', 'z', '~', 0x7E} {
		if !def.TagByteOK(b) {
			t.Errorf("default TagByteOK(%q) = false, want true", b)
		}
	}
	for _, b := range []byte{0x00, 0x1F, ';', 0x7F, 0x80, 0xFF} {
		if def.TagByteOK(b) {
			t.Errorf("default TagByteOK(%q) = true, want false", b)
		}
	}
	if got := def.TagCharsetDescription(); got != "printable ASCII 0x20-0x7E, excluding ';'" {
		t.Errorf("default TagCharsetDescription() = %q", got)
	}

	supplied := Capabilities{TagCharset: "ABC-"}
	for _, b := range []byte{'A', 'B', 'C', '-'} {
		if !supplied.TagByteOK(b) {
			t.Errorf("supplied TagByteOK(%q) = false, want true", b)
		}
	}
	for _, b := range []byte{' ', 'D', ';'} {
		if supplied.TagByteOK(b) {
			t.Errorf("supplied TagByteOK(%q) = true, want false", b)
		}
	}
	if got := supplied.TagCharsetDescription(); !strings.Contains(got, `"ABC-"`) {
		t.Errorf("supplied TagCharsetDescription() = %q, want it to name the charset", got)
	}
}

// minimalCaps is the smallest Capabilities that passes Validate, for the
// tests above to mutate one rule at a time.
func minimalCaps() Capabilities {
	return Capabilities{
		Model:        "TEST",
		CATID:        "0000",
		TagLen:       8,
		Bauds:        []int{38400},
		DefaultBaud:  38400,
		ShiftOptions: StandardShiftOptions(),
		CTCSSStates:  StandardCTCSSStates(),
		Banks: []Bank{
			// THE BANK REACHES BOTH VOCABULARY FIELDS, which is what
			// keeps the empty-vocabulary refusals below meaningful. Since
			// E5b the pair rule fires only when some bank can reach the
			// corresponding field: a model whose banks carry no shift or
			// tone field at all is entitled to declare no vocabulary for
			// one. Every Yaesu model declares both, so this fixture is the
			// shape those refusals are actually about.
			{ID: BankMemory, Slots: []string{"001"}, Fields: map[Field]FieldSupport{
				FieldShift:      {Read: Supported, Write: Unverified},
				FieldCTCSSState: {Read: Supported, Write: Unverified},
			}},
		},
	}
}
