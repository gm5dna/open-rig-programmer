// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"errors"
	"strings"
	"testing"
)

// TestZeroLayout_FailsClosedOnEveryAxis is the FT-891 Stage 0 lesson applied
// from birth: a policy-reading site must not default on a zero axis. The
// zero Layout describes no radio, so it must not be Configured, must carry
// the unset value on every one of the nine axes, and must build and parse
// nothing.
func TestZeroLayout_FailsClosedOnEveryAxis(t *testing.T) {
	var l Layout

	if l.Configured() {
		t.Error("zero Layout reports Configured() = true")
	}

	axes := []struct {
		name string
		got  any
		want any
	}{
		{"Book", l.Book(), BookUnset},
		{"P2Policy", l.P2Policy(), P2Unset},
		{"Byte19", l.Byte19(), Byte19Unset},
		{"Byte28", l.Byte28(), Byte28Unset},
		{"Byte3940", l.Byte3940(), Byte3940Unset},
		{"Byte41", l.Byte41(), Byte41Unset},
		{"ToneModes", l.ToneModes(), ToneModesUnset},
	}
	for _, a := range axes {
		if a.got != a.want {
			t.Errorf("zero Layout %s = %v, want %v", a.name, a.got, a.want)
		}
	}
	if n := len(l.ModeNames()); n != 0 {
		t.Errorf("zero Layout ModeNames has %d entries, want 0", n)
	}
	if n := len(l.Slots()); n != 0 {
		t.Errorf("zero Layout Slots has %d ranges, want 0", n)
	}
	if n := len(l.PrintedFixed()); n != 0 {
		t.Errorf("zero Layout PrintedFixed has %d fields, want 0", n)
	}
}

// TestNewLayout_RefusesAnUnsetAxis walks the ten axes one at a time: each
// case starts from a complete, valid config and blanks exactly one axis, so
// a refusal can only be attributed to that axis.
//
// It is the guard against the failure the plan names — a zero axis silently
// taking a neighbour's meaning — expressed as a refusal at construction
// rather than a check at every reading site.
func TestNewLayout_RefusesAnUnsetAxis(t *testing.T) {
	tests := []struct {
		name  string
		spoil func(*LayoutConfig)
		want  string
	}{
		{"book", func(c *LayoutConfig) { c.Book = BookUnset }, "Book"},
		{"model", func(c *LayoutConfig) { c.Model = "" }, "Model"},
		{"P2", func(c *LayoutConfig) { c.P2 = P2Unset }, "P2"},
		{"byte 19", func(c *LayoutConfig) { c.Byte19 = Byte19Unset }, "byte 19"},
		{"byte 28", func(c *LayoutConfig) { c.Byte28 = Byte28Unset }, "byte 28"},
		{"bytes 39-40", func(c *LayoutConfig) { c.Byte3940 = Byte3940Unset }, "bytes 39-40"},
		{"byte 41", func(c *LayoutConfig) { c.Byte41 = Byte41Unset }, "byte 41"},
		{"tone modes", func(c *LayoutConfig) { c.ToneModes = ToneModesUnset }, "tone-mode"},
		{"EX menu domain", func(c *LayoutConfig) { c.MaxEXAddress = 0 }, "EX menu number"},
		{"mode legend", func(c *LayoutConfig) { c.ModeNames = nil }, "mode legend"},
		{"slot classes", func(c *LayoutConfig) { c.Slots = nil }, "slot"},
		{"printed-fixed set", func(c *LayoutConfig) { c.PrintedFixed = nil }, "printed-fixed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validLayoutConfig()
			tt.spoil(&cfg)
			l, err := NewLayout(cfg)
			if err == nil {
				t.Fatalf("NewLayout accepted a config with %s blanked", tt.name)
			}
			if !errors.Is(err, ErrLayoutInvalid) {
				t.Errorf("errors.Is(err, ErrLayoutInvalid) = false for %v", err)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q does not name %q", err, tt.want)
			}
			if l.Configured() {
				t.Error("NewLayout returned a Configured Layout alongside an error")
			}
		})
	}
}

// TestNewLayout_RefusesTheTwoBooksThisRecordDoesNotDescribe is the door the
// Book widening opens in the OPPOSITE direction to decision 1's.
//
// core/kw now READS four books, because all four print the same envelope and
// the same error-message table. It DESCRIBES two: this file's 50-byte MR/MW
// record is the 590 pair's and the TS-480's. Without the refusal below,
// NewLayout's only book test is Book.valid(), so a caller could mint
// kw.NewLayout(LayoutConfig{Book: kw.Book890, …}) and hold a kw.Layout
// claiming to speak the TS-890S document — one whose eight-opcode
// AllowedCommand admits MR, MW, MC and TY for a radio that has none of them,
// whose hard-wiring checks would be satisfied by 590-shaped data, and whose
// MRAnswerMatcher would correlate 50-byte frames. It is reached by a
// plausible mistake rather than by a type error.
//
// kwtest's own switch catches it only for a layout somebody remembers to
// pass to kwtest, and no ma.Layout ever will.
func TestNewLayout_RefusesTheTwoBooksThisRecordDoesNotDescribe(t *testing.T) {
	for _, book := range []Book{Book890, Book990} {
		t.Run(book.String(), func(t *testing.T) {
			cfg := validLayoutConfig()
			cfg.Book = book
			l, err := NewLayout(cfg)
			if err == nil {
				t.Fatalf("NewLayout accepted %v — a 50-byte MR/MW layout must not claim a document that prints no such record", book)
			}
			if !errors.Is(err, ErrLayoutInvalid) {
				t.Errorf("errors.Is(err, ErrLayoutInvalid) = false for %v", err)
			}
			// BY NAME, and pointing at the package that does describe
			// these two radios: a refusal reading "Book is unset" would
			// send a reader looking for a missing field.
			if !strings.Contains(err.Error(), "core/kw/ma") {
				t.Errorf("the refusal reads %v — it must name core/kw/ma, which is where these two radios' memory channel lives", err)
			}
			if l.Configured() {
				t.Error("NewLayout returned a Configured Layout alongside its error")
			}
		})
	}
}

// TestNewLayout_RefusesAPrintedFixedSetThatContradictsAnAxis pins the
// cross-check: byte 4, byte 28 and byte 41 each have BOTH an axis and a
// possible entry in the printed-fixed set, and the two must agree. A bound
// consulted from somewhere other than its own datum is this repository's
// standing hazard, and here the two would be one edit apart.
func TestNewLayout_RefusesAPrintedFixedSetThatContradictsAnAxis(t *testing.T) {
	tests := []struct {
		name  string
		spoil func(*LayoutConfig)
	}{
		{
			"byte 4 declared fixed by the axis but absent from the set",
			func(c *LayoutConfig) { c.P2 = P2FixedZero },
		},
		{
			"byte 4 in the set but the axis says hundreds digit",
			func(c *LayoutConfig) {
				c.PrintedFixed = append(c.PrintedFixed, FixedField{Pos: 4, Printed: "0"})
			},
		},
		{
			"byte 28 declared fixed by the axis but absent from the set",
			func(c *LayoutConfig) { c.Byte28 = Byte28FixedZero },
		},
		{
			"byte 28 in the set but the axis says the filter is live",
			func(c *LayoutConfig) {
				c.PrintedFixed = append(c.PrintedFixed, FixedField{Pos: 28, Printed: "0"})
			},
		},
		{
			"byte 41 declared fixed by the axis but absent from the set",
			func(c *LayoutConfig) { c.Byte41 = Byte41FixedZero },
		},
		{
			"byte 41 in the set but the axis says lockout",
			func(c *LayoutConfig) {
				c.PrintedFixed = append(c.PrintedFixed, FixedField{Pos: 41, Printed: "0"})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validLayoutConfig()
			tt.spoil(&cfg)
			if _, err := NewLayout(cfg); err == nil {
				t.Fatal("NewLayout accepted a printed-fixed set contradicting an axis")
			} else if !errors.Is(err, ErrLayoutInvalid) {
				t.Errorf("errors.Is(err, ErrLayoutInvalid) = false for %v", err)
			}
		})
	}
}

// TestNewLayout_RefusesAPrintedFixedFieldOutsideTheHardWiredPositions pins
// that only the positions BOTH books ever print as hard-wired may be
// declared fixed. Nothing in either chart says P4, P5, P14 or P16 is a
// constant, so a layout claiming one would let this codec refuse a
// legitimate answer.
func TestNewLayout_RefusesAPrintedFixedFieldOutsideTheHardWiredPositions(t *testing.T) {
	forbidden := []FixedField{
		{Pos: 1, Printed: "M"},           // the command prefix
		{Pos: 3, Printed: "0"},           // P1, derived from the slot class
		{Pos: 5, Printed: "00"},          // P3, the channel number
		{Pos: 7, Printed: "00000000000"}, // P4, the frequency
		{Pos: 18, Printed: "0"},          // P5, the mode
		{Pos: 19, Printed: "0"},          // P6
		{Pos: 20, Printed: "0"},          // P7, the tone mode
		{Pos: 21, Printed: "00"},         // P8
		{Pos: 23, Printed: "00"},         // P9
		{Pos: 39, Printed: "00"},         // P14
		{Pos: 42, Printed: "        "},   // P16, the name
		{Pos: 50, Printed: ";"},          // the terminator
	}
	for _, f := range forbidden {
		cfg := validLayoutConfig()
		cfg.PrintedFixed = append(cfg.PrintedFixed, f)
		if _, err := NewLayout(cfg); err == nil {
			t.Errorf("NewLayout accepted a printed-fixed field at position %d", f.Pos)
		}
	}
}

// TestNewLayout_RequiresTheHardWiringBothBooksPrint pins the other
// direction: P10, P12 and P13 are "Always 000", "Always 0" and "Always
// 000000000" in BOTH books, so no Kenwood layout may omit one.
func TestNewLayout_RequiresTheHardWiringBothBooksPrint(t *testing.T) {
	for i := range commonPrintedFixed() {
		cfg := validLayoutConfig()
		full := commonPrintedFixed()
		cfg.PrintedFixed = append(full[:i:i], full[i+1:]...)
		if _, err := NewLayout(cfg); err == nil {
			t.Errorf("NewLayout accepted a layout omitting the hard-wiring at position %d", full[i].Pos)
		}
	}
}

// TestNewLayout_RefusesAModeLegendNamingANibbleThatIsNotAMode pins A18b's
// documentary basis at the LEGEND: nibbles 0 and 8 are "None (setting
// failure)" / "Not used" in both books (590:1353, 590:1362; 480:843,
// 480:853) and name no mode a channel can be in, so no layout may publish a
// display name for either.
func TestNewLayout_RefusesAModeLegendNamingANibbleThatIsNotAMode(t *testing.T) {
	for _, m := range []Mode{ModeNone, ModeTune} {
		cfg := validLayoutConfig()
		names := modeNames590()
		names[m] = "invented"
		cfg.ModeNames = names
		if _, err := NewLayout(cfg); err == nil {
			t.Errorf("NewLayout accepted a mode legend naming nibble %q", byte(m))
		}
	}
}

// TestNewLayout_RefusesOverlappingSlotRanges pins that a slot number
// resolves to exactly one class. Two ranges claiming one number would make
// the P1-from-slot-class rule ambiguous, which is the whole of M9.
func TestNewLayout_RefusesOverlappingSlotRanges(t *testing.T) {
	cfg := validLayoutConfig()
	cfg.Slots = []SlotRange{
		{Class: SlotMemory, Lo: 0, Hi: 99},
		{Class: SlotScan, Lo: 99, Hi: 109},
	}
	if _, err := NewLayout(cfg); err == nil {
		t.Fatal("NewLayout accepted overlapping slot ranges")
	}
}

// TestNewLayout_RefusesAFixedZeroByte4AboveTheTwoDigitCeiling is the
// cross-check between the P2 axis and the SLOT SPACE, and it is the one axis
// where nothing else would catch a mis-mint.
//
// A row whose byte 4 is "Always 0 for the TS-480." (480:953) has no hundreds
// digit to carry: its channel number is the two digits of P3, "00 ~ 99"
// (480:955). A layout that declared P2FixedZero and then claimed slots above
// 99 would ask slotWire to render a three-digit number into a field that
// holds two, and the frame that came out would name a DIFFERENT channel and
// be reported as Sent — the silent data loss decision 11 exists to prevent,
// on the one byte the printed-fixed cross-check cannot see.
//
// The bound is consulted from the same place as its datum: 480:955 prints
// the ceiling that 480:953 implies.
func TestNewLayout_RefusesAFixedZeroByte4AboveTheTwoDigitCeiling(t *testing.T) {
	fixedZero := func() LayoutConfig {
		cfg := validLayoutConfig()
		cfg.P2 = P2FixedZero
		cfg.PrintedFixed = append(cfg.PrintedFixed, FixedField{Pos: 4, Printed: "0"})
		return cfg
	}

	// The TS-480's own shape: a flat 00-99, which is what 480:955 prints.
	ok := fixedZero()
	ok.Slots = []SlotRange{{Class: SlotMemory, Lo: 0, Hi: 99}}
	if _, err := NewLayout(ok); err != nil {
		t.Fatalf("NewLayout refused a fixed-zero byte 4 over the flat 00-99 of 480:955: %v", err)
	}

	for _, tt := range []struct {
		name  string
		slots []SlotRange
	}{
		{"the 590 pair's section channels", []SlotRange{{Class: SlotMemory, Lo: 0, Hi: 99}, {Class: SlotScan, Lo: 100, Hi: 109}}},
		{"one slot past the ceiling", []SlotRange{{Class: SlotMemory, Lo: 0, Hi: 100}}},
		{"a range wholly above it", []SlotRange{{Class: SlotMemory, Lo: 0, Hi: 99}, {Class: SlotExtension, Lo: 110, Hi: 119}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := fixedZero()
			cfg.Slots = tt.slots
			l, err := NewLayout(cfg)
			if err == nil {
				t.Fatalf("NewLayout accepted a fixed-zero byte 4 alongside slots above 99: %v", l.slotSpaceText())
			}
			if !errors.Is(err, ErrLayoutInvalid) {
				t.Errorf("errors.Is(err, ErrLayoutInvalid) = false for %v", err)
			}
			if !strings.Contains(err.Error(), "480:953") || !strings.Contains(err.Error(), "480:955") {
				t.Errorf("refusal = %q, want it to cite both 480:953 (\"Always 0\") and 480:955 (\"00 ~ 99\")", err)
			}
		})
	}
}

// TestLayout_AccessorsReturnIndependentCopies: a caller's mutation of what
// it is handed must never become every session's layout.
func TestLayout_AccessorsReturnIndependentCopies(t *testing.T) {
	l := layout590SG()

	names := l.ModeNames()
	names[ModeLSB] = "tampered"
	if got := l.ModeNames()[ModeLSB]; got != "LSB" {
		t.Errorf("ModeNames()[ModeLSB] = %q after a caller mutated an earlier copy, want %q", got, "LSB")
	}

	slots := l.Slots()
	slots[0].Hi = 7
	if got := l.Slots()[0].Hi; got != 99 {
		t.Errorf("Slots()[0].Hi = %d after a caller mutated an earlier copy, want 99", got)
	}

	fixed := l.PrintedFixed()
	fixed[0].Printed = "999"
	if got := l.PrintedFixed()[0].Printed; got != "000" {
		t.Errorf("PrintedFixed()[0].Printed = %q after a caller mutated an earlier copy, want %q", got, "000")
	}
}

// TestMustNewLayout_PanicsOnAnInvalidConfig: the model packages mint their
// layouts at package initialisation, where an error return has nowhere to
// go. A layout that cannot be built must stop the programme rather than
// yield a zero value that fails closed silently at every later site.
func TestMustNewLayout_PanicsOnAnInvalidConfig(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustNewLayout did not panic on an invalid config")
		}
	}()
	MustNewLayout(LayoutConfig{})
}

// validLayoutConfig is the TS-590SG's config, which every refusal test above
// spoils in exactly one place. It must itself be accepted, or the tests
// built on it would pass for the wrong reason.
func validLayoutConfig() LayoutConfig {
	return LayoutConfig{
		Book:         Book590,
		Model:        "TS-590SG",
		P2:           P2HundredsDigit,
		Byte19:       Byte19DataMode,
		Byte28:       Byte28FilterLive,
		Byte3940:     Byte3940FMNarrowFlag,
		Byte41:       Byte41Lockout,
		ToneModes:    ToneModesFour,
		MaxEXAddress: 99,
		ModeNames:    modeNames590(),
		Slots:        []SlotRange{{Class: SlotMemory, Lo: 0, Hi: 99}, {Class: SlotScan, Lo: 100, Hi: 109}},
		PrintedFixed: commonPrintedFixed(),
	}
}

// TestValidLayoutConfig_IsAccepted keeps the spoiling tests above honest: if
// the baseline were itself refused, every one of them would pass without
// testing its own axis.
func TestValidLayoutConfig_IsAccepted(t *testing.T) {
	if _, err := NewLayout(validLayoutConfig()); err != nil {
		t.Fatalf("NewLayout(validLayoutConfig()) = %v, want nil", err)
	}
}
