// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"strings"
	"testing"
)

// --- S0.1: SlotSpace.PMSForm, the PMS wire-form axis ---
//
// Every registered Yaesu dialect prints its PMS pairs as the token
// "P<n><L|U>". The FT-991A's MC legend prints them as ordinary decimal
// channel numbers continuing the memory range ("100: P-1L 101: P-1U ~ 116:
// P-9L 117: P-9U"), so the pair number never reaches the wire at all. The
// axis exists because the difference is a WIRE difference: without it a
// numeric-PMS dialect would still build "MW P1L…;" — a slot form its own
// manual never prints — and its own gate would admit the frame.
//
// These tests are the disagreeing side. numericPMSDialect (seconddialect_
// test.go) is the fixture; the registered four declare the token form by
// name.

// pmsFormBaseConfig is a valid token-form config every negative case below
// mutates one field of, so a refusal seen here can only be the mutation's.
func pmsFormBaseConfig() DialectConfig {
	return DialectConfig{
		CATID:     "0800",
		ModeNames: map[Mode]string{ModeLSB: "LSB", ModeUSB: "USB"},
		Slots: SlotSpace{
			MemoryLo: 1, MemoryHi: 99,
			SixtyLo: 0, SixtyHi: 0,
			PMSPairs:      9,
			PMSForm:       PMSFormToken,
			EmergencyWire: "",
			NoneWire:      "000",
			MCSelects:     MCSelectsAll,
		},
		EXAddressForm: EXAddressTriple,
		MT:            MTPolicy{Form: MTFormShort, ReadSlots: MTReadsReadable, TagMaxBytes: 12, ClearTagByte: ' ', PadByte: ' '},
		Clarifier:     ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		MemoryP5:      P5TxClar,
		MWWriteKind:   KindMemory,
	}
}

// TestV15_PMSFormRefusals walks V15's three clauses plus V6's extension.
// Each case names the FIELD and the OFFENDING VALUE the refusal must
// mention, which is the contract validateDialectConfig states.
func TestV15_PMSFormRefusals(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*DialectConfig)
		wantSub string // "" means the config must be ACCEPTED
	}{
		{"zero form with pairs is refused", func(c *DialectConfig) {
			c.Slots.PMSForm = PMSSlotForm(0)
		}, "PMSForm"},
		{"zero form with no pairs is accepted", func(c *DialectConfig) {
			c.Slots.PMSPairs, c.Slots.PMSForm = 0, PMSSlotForm(0)
		}, ""},
		{"token with a non-zero numeric base", func(c *DialectConfig) {
			c.Slots.PMSNumericLo = 100
		}, "PMSNumericLo"},
		{"numeric with a zero base", func(c *DialectConfig) {
			c.Slots.PMSForm, c.Slots.PMSNumericLo = PMSFormNumeric, 0
		}, "PMSNumericLo"},
		{"numeric reaching past 999", func(c *DialectConfig) {
			c.Slots.PMSForm, c.Slots.PMSNumericLo = PMSFormNumeric, 990
		}, "999"},
		{"numeric base with no form declared", func(c *DialectConfig) {
			c.Slots.PMSPairs, c.Slots.PMSForm, c.Slots.PMSNumericLo = 0, PMSSlotForm(0), 100
		}, "PMSNumericLo"},
		{"numeric range overlapping memory", func(c *DialectConfig) {
			c.Slots.PMSForm, c.Slots.PMSNumericLo = PMSFormNumeric, 90
		}, "overlaps PMS numeric range"},
		{"numeric range overlapping 60m", func(c *DialectConfig) {
			c.Slots.SixtyLo, c.Slots.SixtyHi = 501, 599
			c.Slots.PMSForm, c.Slots.PMSNumericLo = PMSFormNumeric, 510
		}, "overlaps PMS numeric range"},
		{"a none wire inside the numeric PMS range", func(c *DialectConfig) {
			c.Slots.PMSForm, c.Slots.PMSNumericLo = PMSFormNumeric, 100
			c.Slots.NoneWire = "105"
		}, "PMS form this dialect can build"},
		{"a token none wire under the numeric form is NOT a PMS collision", func(c *DialectConfig) {
			c.Slots.PMSForm, c.Slots.PMSNumericLo = PMSFormNumeric, 100
			c.Slots.NoneWire = "P1L"
		}, ""},
		{"the FT-991A's own shape is accepted", func(c *DialectConfig) {
			c.Slots.PMSForm, c.Slots.PMSNumericLo = PMSFormNumeric, 100
		}, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := pmsFormBaseConfig()
			tc.mutate(&cfg)
			_, err := NewDialect(cfg)
			switch {
			case tc.wantSub == "" && err != nil:
				t.Fatalf("NewDialect refused a config it must accept: %v", err)
			case tc.wantSub != "" && err == nil:
				t.Fatalf("NewDialect accepted a config it must refuse (want a message naming %q)", tc.wantSub)
			case tc.wantSub != "" && !strings.Contains(err.Error(), tc.wantSub):
				t.Fatalf("refusal %q does not name %q — a validator that reports the wrong field passes a test that only checks for non-nil", err, tc.wantSub)
			}
		})
	}
}

// TestV15_RunsBeforeTheRulesThatReadItsData pins the rule ORDER the spec
// fixes at position 4.
//
// V6 and V7 both consult PMSForm and PMSNumericLo. A config with PMSPairs 9
// and PMSForm omitted would, if V15 ran last, be diagnosed by V6 as an
// overlap with the numeric range 0..17 — a range nobody configured — rather
// than by V15's honest "declare a form". The V8/V12 precedent
// (dialectvalidate.go's renderEXAddressForV8) is the same hazard seen from
// the other side.
func TestV15_RunsBeforeTheRulesThatReadItsData(t *testing.T) {
	cfg := pmsFormBaseConfig()
	cfg.Slots.PMSForm = PMSSlotForm(0)
	_, err := NewDialect(cfg)
	if err == nil {
		t.Fatal("NewDialect accepted a config with no PMSForm and 9 pairs")
	}
	if !strings.Contains(err.Error(), "PMSForm") {
		t.Fatalf("the FIRST refusal is %q, want V15's — a rule reading PMSForm reported before the rule requiring it names a range nobody configured", err)
	}
}

// TestV3_PairBoundIsFormAware pins that the single-ASCII-digit ceiling is
// the TOKEN form's, and that its sentence is unchanged there.
//
// Under PMSFormNumeric the pair number is never on the wire, so that reason
// does not hold and the operative ceiling is V15's (PMSNumericLo + 2*pairs
// - 1 <= 999). Dialect.pmsCap follows the same split, so a numeric dialect
// declaring more than nine pairs is BUILT with them rather than silently
// clamped to nine.
func TestV3_PairBoundIsFormAware(t *testing.T) {
	tokenTen := pmsFormBaseConfig()
	tokenTen.Slots.PMSPairs = 10
	_, err := NewDialect(tokenTen)
	if err == nil {
		t.Fatal("NewDialect accepted PMSPairs 10 under PMSFormToken")
	}
	if want := "the wire form's pair number is a single ASCII digit"; !strings.Contains(err.Error(), want) {
		t.Errorf("V3's token sentence is %q, want it to contain %q verbatim", err, want)
	}

	numericTwelve := pmsFormBaseConfig()
	numericTwelve.Slots.PMSPairs = 12
	numericTwelve.Slots.PMSForm, numericTwelve.Slots.PMSNumericLo = PMSFormNumeric, 100
	d, err := NewDialect(numericTwelve)
	if err != nil {
		t.Fatalf("NewDialect refused 12 numeric pairs (100..123, inside 999): %v", err)
	}
	s, err := d.PMSSlot(12, true)
	if err != nil {
		t.Fatalf("PMSSlot(12, true) on a 12-pair numeric dialect: %v", err)
	}
	if s.Wire() != "123" {
		t.Errorf("PMSSlot(12, true) = %q, want \"123\" — pair 12's upper slot is PMSNumericLo + 2*11 + 1", s.Wire())
	}
	if got := d.classifySlot("123"); got != slotKindPMS {
		t.Errorf("classifySlot(%q) = %v, want slotKindPMS — pmsCap clamped a numeric dialect to nine pairs and the classifier disagreed with its own builder", "123", got)
	}
}

// TestPMSSlot_RendersItsDeclaredForm is the axis's simplest statement.
func TestPMSSlot_RendersItsDeclaredForm(t *testing.T) {
	token := mustFixtureDialect(pmsFormBaseConfig())
	numericCfg := pmsFormBaseConfig()
	numericCfg.Slots.PMSForm, numericCfg.Slots.PMSNumericLo = PMSFormNumeric, 100
	numeric := mustFixtureDialect(numericCfg)

	for _, tc := range []struct {
		d          Dialect
		name       string
		pair       int
		upper      bool
		wantWire   string
		wantParsed bool
	}{
		{token, "token pair 1 lower", 1, false, "P1L", true},
		{token, "token pair 9 upper", 9, true, "P9U", true},
		{numeric, "numeric pair 1 lower", 1, false, "100", true},
		{numeric, "numeric pair 1 upper", 1, true, "101", true},
		{numeric, "numeric pair 9 lower", 9, false, "116", true},
		{numeric, "numeric pair 9 upper", 9, true, "117", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := tc.d.PMSSlot(tc.pair, tc.upper)
			if err != nil {
				t.Fatalf("PMSSlot(%d, %t): %v", tc.pair, tc.upper, err)
			}
			if s.Wire() != tc.wantWire {
				t.Fatalf("PMSSlot(%d, %t) = %q, want %q", tc.pair, tc.upper, s.Wire(), tc.wantWire)
			}
			if !s.IsPMS() {
				t.Errorf("PMSSlot(%d, %t).IsPMS() = false", tc.pair, tc.upper)
			}
			// The four static kinds are not a second opinion: what the
			// constructor built must be what the same dialect's own
			// classifier says (slot.go's invariant).
			if got := tc.d.classifySlot(s.Wire()); got != slotKindPMS {
				t.Errorf("classifySlot(%q) = %v, want slotKindPMS — the constructor and the classifier disagree", s.Wire(), got)
			}
		})
	}
}

// TestClassifySlot_TokenBranchIsGuardedByTheForm is the arm that matters
// most, and the one a numeric arm alone does not supply.
//
// Before S0.1 the token branch's only guard was pmsCap() > 0, so on a
// dialect declaring nine pairs it fired for "P1L" whatever the form said —
// and Dialect.writableSlot returns true for slotKindPMS, so the FT-991A
// would have BUILT "MW P1L…;" / "MT P1L…;" and AllowedCommand would have
// admitted them.
func TestClassifySlot_TokenBranchIsGuardedByTheForm(t *testing.T) {
	numericCfg := pmsFormBaseConfig()
	numericCfg.Slots.PMSForm, numericCfg.Slots.PMSNumericLo = PMSFormNumeric, 100
	numeric := mustFixtureDialect(numericCfg)

	for _, wire := range []string{"P1L", "P1U", "P5L", "P9U"} {
		if got := numeric.classifySlot(wire); got != slotKindInvalid {
			t.Errorf("classifySlot(%q) = %v on a numeric-PMS dialect, want slotKindInvalid — its manual prints no token PMS form", wire, got)
		}
		if _, err := numeric.ParseSlot(wire); err == nil {
			t.Errorf("ParseSlot(%q) accepted a token PMS form on a numeric-PMS dialect", wire)
		}
	}

	// And the write gate, which is the reason the arm exists: a token slot
	// forged under another dialect must be refused by this one's builders.
	token := mustFixtureDialect(pmsFormBaseConfig())
	forged, err := token.PMSSlot(1, false)
	if err != nil {
		t.Fatalf("building the forged slot: %v", err)
	}
	if numeric.writableSlot(forged) {
		t.Fatalf("writableSlot(%q) = true on a numeric-PMS dialect", forged.Wire())
	}
	m := MemoryData{Slot: forged, FreqHz: 14250000, Mode: ModeUSB, Kind: KindMemory, CTCSS: CTCSSOff, Shift: ShiftSimplex}
	if _, err := numeric.BuildMWSet(m); err == nil {
		t.Errorf("BuildMWSet built an MW frame naming %q on a numeric-PMS dialect", forged.Wire())
	}
}

// TestClassifySlot_TokenDialectsRefuseNumericPMS is the other direction: a
// token dialect must not classify the FT-991A's numeric PMS numbers as PMS
// merely because they are three digits.
func TestClassifySlot_TokenDialectsRefuseNumericPMS(t *testing.T) {
	token := mustFixtureDialect(pmsFormBaseConfig()) // memory 001-099, no 60m bank
	for _, wire := range []string{"100", "101", "116", "117"} {
		if got := token.classifySlot(wire); got != slotKindInvalid {
			t.Errorf("classifySlot(%q) = %v on a token-PMS dialect, want slotKindInvalid", wire, got)
		}
	}
}

// TestClassifierSweep_EveryWireFormHasExactlyOneKind is S0.1's sweep,
// restated as a property one build can prove.
//
// Every wire form in "000"-"999", plus every "P<1-9><L|U>", classifies to
// exactly ONE kind under each dialect this package can see — which is what
// "exactly one" means for a switch, so the real content is the SECOND half:
// the numeric fixture refuses every token form, and each token dialect
// refuses every numeric form outside its own memory and 60m ranges.
func TestClassifierSweep_EveryWireFormHasExactlyOneKind(t *testing.T) {
	for _, nd := range allTestDialects() {
		t.Run(nd.name, func(t *testing.T) {
			d := nd.dia
			for n := 0; n <= 999; n++ {
				wire := fmt.Sprintf("%03d", n)
				kind := d.classifySlot(wire)
				if kind == slotKindInvalid {
					continue
				}
				// A classified numeric form must round-trip through this
				// dialect's own ParseSlot with the SAME kind.
				s, err := d.ParseSlot(wire)
				if err != nil {
					t.Fatalf("classifySlot(%q) = %v but ParseSlot refused it: %v", wire, kind, err)
				}
				if got := kindOfSlot(s); got != kind {
					t.Fatalf("ParseSlot(%q) stored kind %v, classifySlot says %v", wire, got, kind)
				}
				if kind == slotKindPMS && d.PMSForm() != PMSFormNumeric {
					t.Fatalf("classifySlot(%q) = slotKindPMS under %v — only the numeric form puts PMS on a decimal wire number", wire, d.PMSForm())
				}
			}
			for pair := 1; pair <= 9; pair++ {
				for _, suffix := range []byte{'L', 'U'} {
					wire := string([]byte{'P', byte('0' + pair), suffix})
					kind := d.classifySlot(wire)
					switch {
					case d.PMSForm() == PMSFormNumeric:
						if kind != slotKindInvalid {
							t.Errorf("classifySlot(%q) = %v on a numeric-PMS dialect, want slotKindInvalid", wire, kind)
						}
					case kind != slotKindInvalid && kind != slotKindPMS:
						t.Errorf("classifySlot(%q) = %v, want slotKindPMS or slotKindInvalid", wire, kind)
					}
				}
			}
		})
	}
}

// kindOfSlot reads the kind a Slot stored, for the round-trip assertion
// above. In-package only: Slot's exported predicates each answer one
// question, and this test needs the whole verdict in one value.
func kindOfSlot(s Slot) slotKind {
	switch {
	case s.IsMemory():
		return slotKindMemory
	case s.IsPMS():
		return slotKindPMS
	case s.Is60m():
		return slotKind60m
	case s.IsEMG():
		return slotKindEMG
	case s.IsNone():
		return slotKindNone
	default:
		return slotKindInvalid
	}
}

// TestPMSForm_RegisteredDialectDeclaresTheTokenForm holds the FT-710's own
// declaration. Its siblings assert the same thing in their own packages,
// against their own manuals, because a citation belongs beside the dialect
// it describes.
func TestPMSForm_RegisteredDialectDeclaresTheTokenForm(t *testing.T) {
	if got := FT710.PMSForm(); got != PMSFormToken {
		t.Errorf("FT710 declares PMSForm %v, want PMSFormToken — its reference prints \"P1L-P9U | PMS pairs (9 lower/upper pairs)\"", got)
	}
	if got := FT710.PMSNumericLo(); got != 0 {
		t.Errorf("FT710 declares PMSNumericLo %d, want 0 under the token form", got)
	}
}
