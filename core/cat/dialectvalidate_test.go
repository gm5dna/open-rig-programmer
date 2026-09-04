// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"strings"
	"testing"
)

// TestValidWireByte_EveryByteValue exhausts all 256 values rather than
// sampling. A sampled test — six rejects and a handful of accepts — passes
// for an implementation that admits some untested control byte, which is
// precisely the defect class this domain exists to close: a caller-built
// dialect must not be able to put a NUL, a DEL or a high byte inside a
// frame the gate then approves (Codex spec review, finding 2).
//
// The expectation is written as an independent expression rather than by
// calling the function under test, so this cannot degenerate into
// comparing the implementation with itself.
func TestValidWireByte_EveryByteValue(t *testing.T) {
	for i := 0; i < 256; i++ {
		b := byte(i)
		want := b >= 0x20 && b <= 0x7E && b != ';'
		if got := validWireByte(b); got != want {
			t.Errorf("validWireByte(%#02x) = %v, want %v", b, got, want)
		}
	}
}

// TestValidWireByte_BoundariesAndTerminator names the interesting values
// explicitly, so a failure reports WHICH boundary moved rather than only
// that some byte disagreed.
func TestValidWireByte_BoundariesAndTerminator(t *testing.T) {
	tests := []struct {
		b    byte
		want bool
		why  string
	}{
		{0x1F, false, "one below the printable range"},
		{0x20, true, "space: the low boundary, and FT-710's own clear-tag byte"},
		{0x3A, true, "':' — one below ';', so the exclusion must not be a range"},
		{';', false, "the frame terminator: a datum carrying one would split a command in two"},
		{0x3C, true, "'<' — one above ';', same reason"},
		{0x7E, true, "'~': the high boundary"},
		{0x7F, false, "DEL, one above the printable range"},
		{0x80, false, "the first byte of a multi-byte UTF-8 rune"},
		{0xFF, false, "the top of the byte range"},
		{0x00, false, "NUL — the byte finding 2 got past every rule"},
	}
	for _, tc := range tests {
		if got := validWireByte(tc.b); got != tc.want {
			t.Errorf("validWireByte(%#02x) = %v, want %v (%s)", tc.b, got, tc.want, tc.why)
		}
	}
}

func TestValidWireString(t *testing.T) {
	tests := []struct {
		s    string
		want bool
		why  string
	}{
		{"", true, "empty: callers decide separately whether empty is allowed"},
		{"EMG", true, "an ordinary special-slot wire form"},
		{"0800", true, "a CAT ID"},
		{"\x00AB", false, "finding 2's EmergencyWire, which passed a length-only rule"},
		{"AB;", false, "an embedded terminator anywhere in the string"},
		{";", false, "a lone terminator"},
		{"AB\x7F", false, "a trailing DEL"},
		{"é", false, "every byte of a multi-byte rune is >= 0x80"},
	}
	for _, tc := range tests {
		if got := validWireString(tc.s); got != tc.want {
			t.Errorf("validWireString(%q) = %v, want %v (%s)", tc.s, got, tc.want, tc.why)
		}
	}
}

// validBaselineConfig is a KNOWN-GOOD config that is deliberately NOT the
// FT-710's: its own values, so a rule accidentally written against FT-710
// data fails here rather than passing by coincidence.
//
// Every table entry below perturbs exactly ONE thing from this baseline, so
// a failure identifies the rule rather than the fixture.
func validBaselineConfig() DialectConfig {
	return DialectConfig{
		CATID: "1234",
		ModeNames: map[Mode]string{
			Mode('2'): "ALPHA",
			Mode('3'): "BETA",
		},
		Slots: SlotSpace{
			MemoryLo: 10, MemoryHi: 40,
			SixtyLo: 700, SixtyHi: 720,
			PMSPairs:      3,
			EmergencyWire: "HLP",
			NoneWire:      "888",
			MCSelects:     MCSelectsAll,
		},
		EXItems: []EXItem{
			{Addr: EXAddress{P1: 7, P2: 1, P3: 1}, Name: "ITEM ONE", Digits: 2},
			{Addr: EXAddress{P1: 7, P2: 1, P3: 2}, Name: "ITEM TWO", Digits: 5},
		},
		MT:          MTPolicy{Form: MTFormShort, ReadSlots: MTReadsReadable, TagMaxBytes: 8, ClearTagByte: '_'},
		Clarifier:   ClarifierPolicy{StepHz: 5, MaxAbsHz: 500},
		MWWriteKind: KindMemory,
	}
}

// TestValidateDialectConfig_EveryClause covers every CLAUSE of every rule,
// in BOTH directions, and asserts on error CONTENT.
//
// Clause-level rather than rule-level: V2 alone has four clauses and V4
// has four, and one entry per numbered rule would leave most of them
// unexercised. Content-level rather than just non-nil: a validator that
// returns a generic error from the wrong branch passes a wantErr bool
// check while reporting the wrong field to whoever has to fix it.
func TestValidateDialectConfig_EveryClause(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*DialectConfig)
		wantErr string // "" means the config MUST be accepted
	}{
		{"baseline is valid", func(*DialectConfig) {}, ""},

		// V1 — CAT ID
		{"V1 too short", func(c *DialectConfig) { c.CATID = "080" }, "CATID"},
		{"V1 too long", func(c *DialectConfig) { c.CATID = "08000" }, "CATID"},
		{"V1 empty", func(c *DialectConfig) { c.CATID = "" }, "CATID"},
		{"V1 four bytes but one is a terminator", func(c *DialectConfig) { c.CATID = "08;0" }, "outside printable"},
		{"V1 four printable bytes accepted", func(c *DialectConfig) { c.CATID = "0761" }, ""},

		// V2 — modes: four clauses
		{"V2 empty map", func(c *DialectConfig) { c.ModeNames = map[Mode]string{} }, "ModeNames is empty"},
		{"V2 nil map", func(c *DialectConfig) { c.ModeNames = nil }, "ModeNames is empty"},
		{"V2 empty name", func(c *DialectConfig) { c.ModeNames[Mode('2')] = "" }, "is empty, want a display name"},
		{"V2 duplicate name", func(c *DialectConfig) { c.ModeNames[Mode('4')] = "ALPHA" }, "duplicate name"},
		{"V2 key is NUL", func(c *DialectConfig) { c.ModeNames[Mode(0x00)] = "NUL" }, "outside printable"},
		{"V2 key is a terminator", func(c *DialectConfig) { c.ModeNames[Mode(';')] = "SEMI" }, "outside printable"},
		{"V2 key is high byte", func(c *DialectConfig) { c.ModeNames[Mode(0x80)] = "HIGH" }, "outside printable"},
		{"V2 unusual but printable key accepted", func(c *DialectConfig) { c.ModeNames[Mode('z')] = "ZULU" }, ""},
		{"V2 single mode accepted", func(c *DialectConfig) { c.ModeNames = map[Mode]string{Mode('2'): "ONLY"} }, ""},

		// V3 — PMS pairs
		{"V3 ten rejected not clamped", func(c *DialectConfig) { c.Slots.PMSPairs = 10 }, "PMSPairs"},
		{"V3 negative", func(c *DialectConfig) { c.Slots.PMSPairs = -1 }, "PMSPairs"},
		{"V3 nine accepted", func(c *DialectConfig) { c.Slots.PMSPairs = 9 }, ""},
		{"V3 zero accepted (family has none)", func(c *DialectConfig) { c.Slots.PMSPairs, c.Slots.EmergencyWire = 0, "HLP" }, ""},

		// V4 — special wires: length, domain, absence, both fields
		{"V4 emergency wrong length", func(c *DialectConfig) { c.Slots.EmergencyWire = "HELP" }, "EmergencyWire"},
		{"V4 none wrong length", func(c *DialectConfig) { c.Slots.NoneWire = "88" }, "NoneWire"},
		{"V4 emergency has NUL", func(c *DialectConfig) { c.Slots.EmergencyWire = "\x00AB" }, "outside printable"},
		{"V4 none has terminator", func(c *DialectConfig) { c.Slots.NoneWire = "8;8" }, "outside printable"},
		{"V4 emergency absent accepted", func(c *DialectConfig) { c.Slots.EmergencyWire = "" }, ""},
		{"V4 none absent accepted", func(c *DialectConfig) { c.Slots.NoneWire = "" }, ""},

		// V5 — memory range
		{"V5 MemoryLo 0 is PERMITTED", func(c *DialectConfig) { c.Slots.MemoryLo = 0 }, ""},
		{"V5 dead absence 99..0", func(c *DialectConfig) { c.Slots.MemoryLo, c.Slots.MemoryHi = 99, 0 }, "express an absent range"},
		{"V5 inverted", func(c *DialectConfig) { c.Slots.MemoryLo, c.Slots.MemoryHi = 40, 10 }, "want 0 <= Lo <= Hi"},
		{"V5 above 3 digits", func(c *DialectConfig) { c.Slots.MemoryLo, c.Slots.MemoryHi = 10, 1000 }, "3 digits"},
		{"V5 absent (0,0) accepted", func(c *DialectConfig) { c.Slots.MemoryLo, c.Slots.MemoryHi = 0, 0 }, ""},

		// V6 — 60m range and overlap
		{"V6 dead absence", func(c *DialectConfig) { c.Slots.SixtyLo, c.Slots.SixtyHi = 700, 0 }, "express an absent range"},
		{"V6 overlaps memory", func(c *DialectConfig) { c.Slots.SixtyLo, c.Slots.SixtyHi = 30, 60 }, "overlaps"},
		{"V6 abuts memory without overlapping", func(c *DialectConfig) { c.Slots.SixtyLo, c.Slots.SixtyHi = 41, 60 }, ""},
		{"V6 absent accepted", func(c *DialectConfig) { c.Slots.SixtyLo, c.Slots.SixtyHi = 0, 0 }, ""},

		// V7 — shadowing
		{"V7 none shadows memory", func(c *DialectConfig) { c.Slots.MemoryLo, c.Slots.NoneWire = 0, "000" }, "memory range"},
		{"V7 none shadows 60m", func(c *DialectConfig) { c.Slots.NoneWire = "710" }, "60m range"},
		{"V7 emergency shadows memory", func(c *DialectConfig) { c.Slots.EmergencyWire = "020" }, "memory range"},
		{"V7 emergency collides with PMS", func(c *DialectConfig) { c.Slots.EmergencyWire = "P1L" }, "PMS form"},
		{"V7 PMS collision only within declared pairs", func(c *DialectConfig) { c.Slots.EmergencyWire = "P9U" }, ""},
		{"V7 none equals emergency", func(c *DialectConfig) { c.Slots.NoneWire = "HLP" }, "both"},
		{"V7 non-digit non-PMS accepted", func(c *DialectConfig) { c.Slots.EmergencyWire = "ZZZ" }, ""},

		// V8 — EX inventory
		{"V8 duplicate address", func(c *DialectConfig) { c.EXItems[1].Addr = c.EXItems[0].Addr }, "repeats address"},
		{"V8 component 100", func(c *DialectConfig) { c.EXItems[0].Addr.P1 = 100 }, "P1"},
		{"V8 component 255", func(c *DialectConfig) { c.EXItems[0].Addr.P3 = 255 }, "P3"},
		{"V8 component 99 accepted", func(c *DialectConfig) { c.EXItems[0].Addr.P2 = 99 }, ""},
		{"V8 zero digits", func(c *DialectConfig) { c.EXItems[0].Digits = 0 }, "want >= 1"},
		{"V8 digits over the frame bound", func(c *DialectConfig) { c.EXItems[0].Digits = maxEXDigits + 1 }, "DefaultMaxFrame"},
		{"V8 digits at the frame bound accepted", func(c *DialectConfig) { c.EXItems[0].Digits = maxEXDigits }, ""},
		{"V8 empty inventory accepted", func(c *DialectConfig) { c.EXItems = nil }, ""},

		// V9 — MT policy
		{"V9 zero tag width", func(c *DialectConfig) { c.MT.TagMaxBytes = 0 }, "TagMaxBytes"},
		{"V9 tag width over ceiling", func(c *DialectConfig) { c.MT.TagMaxBytes = maxMTTagBytes + 1 }, "TagMaxBytes"},
		{"V9 tag width at ceiling accepted", func(c *DialectConfig) { c.MT.TagMaxBytes = maxMTTagBytes }, ""},
		{"V9 clear byte is NUL", func(c *DialectConfig) { c.MT.ClearTagByte = 0x00 }, "ClearTagByte"},
		{"V9 clear byte is a terminator", func(c *DialectConfig) { c.MT.ClearTagByte = ';' }, "ClearTagByte"},
		{"V9 space clear byte accepted", func(c *DialectConfig) { c.MT.ClearTagByte = ' ' }, ""},
		// PadByte: 0 is the "no padding declared" sentinel, unambiguous
		// because 0 is outside the permitted tag-byte domain. Anything
		// else must be a byte that can legitimately appear in a tag, since
		// decoding trims it from answers. Without these the validation
		// clause could be deleted and the suite stayed green (re-review 4).
		{"V9 PadByte 0 accepted (no padding declared)", func(c *DialectConfig) { c.MT.PadByte = 0 }, ""},
		{"V9 PadByte space accepted", func(c *DialectConfig) { c.MT.PadByte = ' ' }, ""},
		{"V9 PadByte printable accepted", func(c *DialectConfig) { c.MT.PadByte = '.' }, ""},
		{"V9 PadByte NUL-adjacent control byte", func(c *DialectConfig) { c.MT.PadByte = 0x01 }, "PadByte"},
		{"V9 PadByte terminator", func(c *DialectConfig) { c.MT.PadByte = ';' }, "PadByte"},
		{"V9 PadByte high byte", func(c *DialectConfig) { c.MT.PadByte = 0x80 }, "PadByte"},
		{"V9 PadByte DEL", func(c *DialectConfig) { c.MT.PadByte = 0x7F }, "PadByte"},

		// V10 — clarifier
		{"V10 zero step", func(c *DialectConfig) { c.Clarifier.StepHz = 0 }, "StepHz"},
		{"V10 negative step", func(c *DialectConfig) { c.Clarifier.StepHz = -1 }, "StepHz"},
		{"V10 negative max", func(c *DialectConfig) { c.Clarifier.MaxAbsHz = -10 }, "MaxAbsHz"},
		{"V10 max beyond the 4-digit field", func(c *DialectConfig) { c.Clarifier.MaxAbsHz = 10000 }, "4 digits"},
		{"V10 max not a multiple of step", func(c *DialectConfig) { c.Clarifier.MaxAbsHz = 501 }, "not a multiple"},
		{"V10 FT-710's own pair accepted", func(c *DialectConfig) { c.Clarifier = ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990} }, ""},
		{"V10 1 Hz step to the field edge accepted", func(c *DialectConfig) { c.Clarifier = ClarifierPolicy{StepHz: 1, MaxAbsHz: 9999} }, ""},

		// V11 — MW write kind
		{"V11 undocumented kind", func(c *DialectConfig) { c.MWWriteKind = 'X' }, "MWWriteKind"},
		{"V11 zero byte", func(c *DialectConfig) { c.MWWriteKind = 0x00 }, "MWWriteKind"},
		{"V11 KindPMS accepted", func(c *DialectConfig) { c.MWWriteKind = KindPMS }, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validBaselineConfig()
			tc.mutate(&cfg)
			err := validateDialectConfig(cfg)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("validateDialectConfig() = %v, want accepted", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("validateDialectConfig() = nil, want an error mentioning %q", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("validateDialectConfig() = %q, want it to mention %q — a rule reporting the wrong field sends whoever has to fix it to the wrong place", err, tc.wantErr)
			}
		})
	}
}

// TestValidateDialectConfig_BaselineIsNotTheFT710 guards the table above
// against the failure where every rule is quietly written for FT-710 data
// and passes by coincidence.
func TestValidateDialectConfig_BaselineIsNotTheFT710(t *testing.T) {
	c := validBaselineConfig()
	if c.CATID == FT710.CATID() {
		t.Error("baseline CATID equals the FT-710's — the fixture must disagree with it")
	}
	if c.Slots.MemoryHi == 99 && c.Slots.MemoryLo == 1 {
		t.Error("baseline memory range equals the FT-710's")
	}
	if c.MT.TagMaxBytes == 12 || c.MT.ClearTagByte == ' ' {
		t.Error("baseline MT policy matches the FT-710's")
	}
	if c.Clarifier.StepHz == 10 || c.Clarifier.MaxAbsHz == 9990 {
		t.Error("baseline clarifier policy matches the FT-710's")
	}
}
