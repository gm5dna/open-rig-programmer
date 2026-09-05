// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"strings"
	"testing"
)

// This file is NEW, and every case in it is new, for the reason
// exaddressform_test.go's own header gives: ex_test.go, exinventory_test.go
// and the rest of the evidence set are pinned literal-by-literal AND
// ordinal-by-ordinal in core/cat/testdata/evidence-literals.golden, so a
// literal inserted anywhere but the very end of one of them re-indexes every
// record below it. A new file carries no pinned ordinals at all, which is
// what lets the FT-991A's Single-form cases be added without the frozen
// golden moving a byte.

// singleEXItems is the three-digit form's inventory. Its four addresses are
// chosen to straddle every boundary this seam moves:
//
//   - 001 is the chart's first row and the narrowest render;
//   - 099 is the OLD package-wide component ceiling, still legal;
//   - 100 is the first address that was inexpressible before this seam —
//     refused by V8's 0..99 bound and, under uint8, not even representable
//     in a generated literal;
//   - 153 is the FT-991A chart's last row, the address the whole widening
//     exists to make registrable.
//
// Every member has P2 == 0 and P3 == 0, which is not decoration: it is what
// V12 requires of a Single-form dialect, because the three-digit field
// renders P1 alone and a non-zero P2 or P3 would be dropped from every frame
// silently — the AddressPair discipline one component further down.
//
// The widest Digits is 8, so this fixture's exAnswerMaxLen is 2+3+8+1 = 14
// and TestEXAddressSingle_DerivedFrameLengths can pin that number against
// the plan's own arithmetic rather than against whatever the fixture happens
// to hold.
var singleEXItems = []EXItem{
	{Addr: EXAddress{P1: 1}, Name: "SINGLE ITEM 001", Digits: 1},
	{Addr: EXAddress{P1: 99}, Name: "SINGLE ITEM 099", Digits: 2},
	{Addr: EXAddress{P1: 100}, Name: "SINGLE ITEM 100", Digits: 4},
	{Addr: EXAddress{P1: 153}, Name: "SINGLE ITEM 153", Digits: 8},
}

// singleDialect is the fixture that makes the EX address width a THREE-valued
// variable rather than a two-valued one, and it is in allTestDialects() for
// the reason pairDialect is: every gate, conformance and round-trip property
// this package states over "any dialect" must be stated over a three-digit
// dialect too, or a length that happens to be right for 6 and 4 goes on
// passing every positive test in the package.
//
// Its EX read frame is SIX bytes ("EX" + 3 + ";"), against pairDialect's
// seven and the FT-710's nine.
//
// Everything else is deliberately unremarkable — short MT form, a small
// memory range, no 60 m bank, no emergency channel — so that a failure here
// points at the address width and not at some second difference. It is
// FIXTURE-ONLY: no radio is registered by it, and the FT-991A's own dialect
// is Stage 1's work.
var singleDialect = mustFixtureDialect(DialectConfig{
	CATID: "7777",
	ModeNames: map[Mode]string{
		ModeLSB: "LSB-SINGLE",
		ModeUSB: "USB-SINGLE",
	},
	Slots: SlotSpace{
		MemoryLo: 1, MemoryHi: 20,
		SixtyLo: 0, SixtyHi: 0,
		PMSPairs:      2,
		EmergencyWire: "",
		NoneWire:      "000",
		MCSelects:     MCSelectsAll,
		PMSForm:       PMSFormToken, // the SLOT lane's axis: this fixture varies the EX form only
	},
	EXItems:       singleEXItems,
	EXAddressForm: EXAddressSingle,
	MT:            MTPolicy{Form: MTFormShort, ReadSlots: MTReadsReadable, TagMaxBytes: 12, ClearTagByte: ' ', PadByte: ' '},
	Clarifier:     ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
	ToneStates:    ToneStatesCTCSS, // the SLOT lane's axis: three states, as every token dialect
	MemoryP5:      P5TxClar,
	MWWriteKind:   KindMemory,
})

// TestEXAddressSingle_WireRenderIsThreeDigitsOfP1 pins the render itself.
// P2 and P3 are DROPPED — V12 has already required them zero, and the two
// facts are the two halves of one rule, exactly as under EXAddressPair.
func TestEXAddressSingle_WireRenderIsThreeDigitsOfP1(t *testing.T) {
	for _, tc := range []struct {
		addr EXAddress
		want string
	}{
		{EXAddress{P1: 1}, "001"},
		{EXAddress{P1: 99}, "099"},
		{EXAddress{P1: 100}, "100"},
		{EXAddress{P1: 153}, "153"},
		{EXAddress{}, "000"},
		{EXAddress{P1: 8, P2: 3, P3: 7}, "008"},
	} {
		if got := wireEXAddress(EXAddressSingle, tc.addr); got != tc.want {
			t.Errorf("wireEXAddress(EXAddressSingle, %+v) = %q, want %q", tc.addr, got, tc.want)
		}
	}
}

// TestEXAddressSingle_DerivedFrameLengths is the property the seam exists
// for: nothing in ex.go knows the number 3. EXAddressWidth MEASURES
// wireEXAddress's render, and the three frame lengths are arithmetic over
// that measurement, so a three-digit family is expressible without a second
// table of widths anywhere.
func TestEXAddressSingle_DerivedFrameLengths(t *testing.T) {
	if got := singleDialect.EXAddressWidth(); got != 3 {
		t.Fatalf("singleDialect.EXAddressWidth() = %d, want 3", got)
	}
	if got, want := singleDialect.exReadLen(), 6; got != want {
		t.Errorf("exReadLen() = %d, want %d — \"EX\"(2) + address(3) + \";\"(1)", got, want)
	}
	if got, want := singleDialect.exAnswerMinLen(), 7; got != want {
		t.Errorf("exAnswerMinLen() = %d, want %d", got, want)
	}
	// 2 + 3 + 8 + 1: the fixture's widest item is 8 digits wide, and the
	// bound is derived from THIS dialect's own inventory.
	if got, want := singleDialect.exAnswerMaxLen(), 14; got != want {
		t.Errorf("exAnswerMaxLen() = %d, want %d (2 + 3 + %d + 1)", got, want, singleDialect.exP4MaxBytes())
	}
	if singleDialect.exAnswerMaxLen() >= DefaultMaxFrame {
		t.Errorf("exAnswerMaxLen() = %d, which is not under DefaultMaxFrame (%d)", singleDialect.exAnswerMaxLen(), DefaultMaxFrame)
	}
}

// TestParseEXAddress_SingleRoundTripsItsOwnWire goes through BOTH sides of
// the seam on every member, including 100 and 153 — the addresses that were
// unrepresentable before it. A parser tested against hand-written strings
// alone can disagree with the renderer and still pass.
func TestParseEXAddress_SingleRoundTripsItsOwnWire(t *testing.T) {
	for _, it := range singleEXItems {
		wire := singleDialect.EXWire(it.Addr)
		if len(wire) != 3 {
			t.Errorf("EXWire(%+v) = %q, want three digits", it.Addr, wire)
		}
		got, err := singleDialect.ParseEXAddress(wire)
		if err != nil {
			t.Errorf("ParseEXAddress(%q) = %v, want the member back", wire, err)
			continue
		}
		if got != it.Addr {
			t.Errorf("ParseEXAddress(%q) = %+v, want %+v", wire, got, it.Addr)
		}
	}
	// The literal wire spellings too, so the round trip above cannot pass by
	// both sides agreeing on the WRONG width.
	for _, tc := range []struct {
		wire string
		want EXAddress
	}{
		{"001", EXAddress{P1: 1}},
		{"099", EXAddress{P1: 99}},
		{"100", EXAddress{P1: 100}},
		{"153", EXAddress{P1: 153}},
	} {
		got, err := singleDialect.ParseEXAddress(tc.wire)
		if err != nil {
			t.Errorf("ParseEXAddress(%q) = %v, want %+v", tc.wire, err, tc.want)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseEXAddress(%q) = %+v, want %+v", tc.wire, got, tc.want)
		}
	}
}

// TestParseEXAddress_SingleRefusesTheOtherFormsGeometry is the disagreeing
// half: the three-digit parser must refuse the four- and six-digit fields,
// and a non-member three-digit field is refused by MEMBERSHIP rather than by
// width. Without both halves a length check hardwired to 6 or 4 passes every
// positive case above.
func TestParseEXAddress_SingleRefusesTheOtherFormsGeometry(t *testing.T) {
	for _, tc := range []struct {
		name string
		wire string
		want string
	}{
		{"two digits is short", "08", "exactly three digits"},
		{"four digits is the Pair geometry", "0803", "exactly three digits"},
		{"six digits is the Triple geometry", "010203", "exactly three digits"},
		{"non-digit", "0X1", "three ASCII digits"},
		{"a non-member of the right width", "002", "not a known Table 2 member"},
	} {
		_, err := singleDialect.ParseEXAddress(tc.wire)
		if err == nil {
			t.Errorf("%s: ParseEXAddress(%q) succeeded, want a refusal naming %q", tc.name, tc.wire, tc.want)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: ParseEXAddress(%q) = %v, want it to say %q", tc.name, tc.wire, err, tc.want)
		}
	}
}

// TestValidateEXItems_ComponentBoundIsFormDependent is the domain widening's
// own pin, and it is a TABLE over both forms because the property is a
// DISAGREEMENT: the same component value is legal under one form and refused
// under another, which no single-form case can state.
//
// P1 = 300 appears twice on purpose. The plan's §OQ1 names it as "the red
// proof that is impossible under uint8", and it is: under uint8 the field
// could not hold 300 at all and a generated literal of 300 failed at compile
// time in a file no human wrote. Under uint16 it is representable, and what
// it then proves is the DISAGREEMENT — accepted under Single (300 <= 999),
// refused under Triple by the unchanged 0..99 sentence. The plan's own
// wording also calls 300 "refused by V8 naming 999", which cannot be true of
// a value below 999; the row that actually exercises the 999 bound is
// P1 = 1000, and it is here too, so the pin holds under either reading.
func TestValidateEXItems_ComponentBoundIsFormDependent(t *testing.T) {
	for _, tc := range []struct {
		name    string
		form    EXAddressForm
		addr    EXAddress
		wantErr string // "" means V8 MUST accept
	}{
		{"Single accepts 099, the old ceiling", EXAddressSingle, EXAddress{P1: 99}, ""},
		{"Single accepts 100, the first address the old bound refused", EXAddressSingle, EXAddress{P1: 100}, ""},
		{"Single accepts 153, the chart's last row", EXAddressSingle, EXAddress{P1: 153}, ""},
		{"Single accepts 300, which uint8 could not even hold", EXAddressSingle, EXAddress{P1: 300}, ""},
		{"Single accepts 999, the top of the three-digit field", EXAddressSingle, EXAddress{P1: 999}, ""},
		{"Single refuses 1000, one over the field", EXAddressSingle, EXAddress{P1: 1000}, "want <= 999"},
		{"Single keeps P2 at 99", EXAddressSingle, EXAddress{P1: 1, P2: 100}, "want <= 99"},
		{"Single keeps P3 at 99", EXAddressSingle, EXAddress{P1: 1, P3: 100}, "want <= 99"},
		{"Triple still refuses 100", EXAddressTriple, EXAddress{P1: 100, P2: 1, P3: 1}, "want <= 99"},
		{"Triple still refuses 300", EXAddressTriple, EXAddress{P1: 300, P2: 1, P3: 1}, "want <= 99"},
		{"Pair still refuses 100", EXAddressPair, EXAddress{P1: 100, P2: 1}, "want <= 99"},
	} {
		cfg := validBaselineConfig()
		cfg.EXAddressForm = tc.form
		cfg.EXItems = []EXItem{{Addr: tc.addr, Name: "ITEM", Digits: 2}}
		err := validateEXItems(cfg)
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("%s: validateEXItems() = %v, want accepted", tc.name, err)
		case tc.wantErr != "" && err == nil:
			t.Errorf("%s: validateEXItems() accepted %+v, want a refusal naming %q", tc.name, tc.addr, tc.wantErr)
		case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
			t.Errorf("%s: validateEXItems() = %v, want it to name %q", tc.name, err, tc.wantErr)
		}
	}
}

// TestValidateEXItems_SingleAndTripleSentencesAreDistinct pins BOTH texts in
// full. The Triple/Pair sentence is SHIPPED and must not have moved by a
// byte; the Single one is new and must name its own form, its own bound and
// its own render width, or a reader meeting it has to know which form was in
// force to know which number is being quoted.
func TestValidateEXItems_SingleAndTripleSentencesAreDistinct(t *testing.T) {
	triple := validBaselineConfig()
	triple.EXItems = []EXItem{{Addr: EXAddress{P1: 100, P2: 1, P3: 1}, Name: "ITEM", Digits: 2}}
	err := validateEXItems(triple)
	if err == nil {
		t.Fatal("validateEXItems() accepted a Triple component of 100")
	}
	wantTriple := fmt.Sprintf("cat: EXItems[0].Addr.P1 is 100, want <= %d — wireEXAddress renders %%02d, a MINIMUM width, so a larger component overruns the fixed-width address field this dialect's own ParseEXAddress reads back", maxEXComponent)
	if err.Error() != wantTriple {
		t.Errorf("the shipped component refusal moved\n    was: %s\n    now: %s", wantTriple, err.Error())
	}

	single := validBaselineConfig()
	single.EXAddressForm = EXAddressSingle
	single.EXItems = []EXItem{{Addr: EXAddress{P1: 1000}, Name: "ITEM", Digits: 2}}
	err = validateEXItems(single)
	if err == nil {
		t.Fatal("validateEXItems() accepted a Single P1 of 1000")
	}
	wantSingle := fmt.Sprintf("cat: EXItems[0].Addr.P1 is 1000, want <= %d under EXAddressSingle — wireEXAddress renders %%03d under this form, a MINIMUM width, so a larger P1 overruns the three-digit address field this dialect's own ParseEXAddress reads back", maxEXComponentSingleP1)
	if err.Error() != wantSingle {
		t.Errorf("the Single component refusal is not the designed sentence\n    want: %s\n    got:  %s", wantSingle, err.Error())
	}
}

// TestValidateDialectConfig_V12Single covers the new clause in both
// directions. Under Single the wire field renders P1 alone, so a non-zero P2
// or P3 names something no frame this dialect builds can carry — refused, not
// dropped, for the reason the Pair clause refuses a non-zero P3.
func TestValidateDialectConfig_V12Single(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*DialectConfig)
		wantErr string // "" means the config MUST be accepted
	}{
		{"Single with an all-zero P2/P3 inventory accepted", func(c *DialectConfig) {
			c.EXAddressForm = EXAddressSingle
			c.EXItems = []EXItem{
				{Addr: EXAddress{P1: 1}, Name: "ONE", Digits: 2},
				{Addr: EXAddress{P1: 153}, Name: "TWO", Digits: 2},
			}
		}, ""},
		{"Single empty inventory accepted", func(c *DialectConfig) {
			c.EXAddressForm, c.EXItems = EXAddressSingle, nil
		}, ""},
		{"Single with a non-zero P2 refused", func(c *DialectConfig) {
			c.EXAddressForm = EXAddressSingle
			c.EXItems = []EXItem{{Addr: EXAddress{P1: 8, P2: 3}, Name: "ONE", Digits: 2}}
		}, "P2"},
		{"Single with a non-zero P3 refused", func(c *DialectConfig) {
			c.EXAddressForm = EXAddressSingle
			c.EXItems = []EXItem{{Addr: EXAddress{P1: 8, P3: 7}, Name: "ONE", Digits: 2}}
		}, "P3"},
	} {
		cfg := validBaselineConfig()
		tc.mutate(&cfg)
		err := validateDialectConfig(cfg)
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("%s: validateDialectConfig() = %v, want accepted", tc.name, err)
		case tc.wantErr != "" && err == nil:
			t.Errorf("%s: validateDialectConfig() accepted the config, want a refusal naming %q", tc.name, tc.wantErr)
		case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
			t.Errorf("%s: validateDialectConfig() = %v, want it to name %q", tc.name, err, tc.wantErr)
		}
	}
}

// TestValidateDialectConfig_V12SingleErrorNamesTheAddress is the Single
// counterpart of the Pair test beside it: the refusal must identify WHICH
// member offends, rendered through the same renderer the frame would have
// used, so a 153-row inventory does not have to be searched by hand.
func TestValidateDialectConfig_V12SingleErrorNamesTheAddress(t *testing.T) {
	cfg := validBaselineConfig()
	cfg.EXAddressForm = EXAddressSingle
	cfg.EXItems = []EXItem{
		{Addr: EXAddress{P1: 1}, Name: "FINE", Digits: 2},
		{Addr: EXAddress{P1: 87, P2: 3, P3: 7}, Name: "OFFENDER", Digits: 2},
	}
	err := validateDialectConfig(cfg)
	if err == nil {
		t.Fatal("validateDialectConfig() accepted a Single config whose second member has P2 == 3 and P3 == 7")
	}
	for _, want := range []string{"EXItems[1]", "087", "EXAddressSingle"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("V12 Single refusal %q does not contain %q", err, want)
		}
	}
}

// TestBuildEXRead_SingleIsSixBytesAndGateAdmissible is the builder end, and
// it goes through the dialect's OWN gate: a length hardwired to 9 or 7
// anywhere in the allowlist refuses this dialect's own builder's output, and
// nothing else in this package would report it.
func TestBuildEXRead_SingleIsSixBytesAndGateAdmissible(t *testing.T) {
	a := singleEXItems[3].Addr // 153, the chart's last row
	cmd, err := singleDialect.BuildEXRead(a)
	if err != nil {
		t.Fatalf("singleDialect.BuildEXRead(%v) = %v", a, err)
	}
	if got, want := string(cmd.Bytes()), "EX153;"; got != want {
		t.Errorf("singleDialect.BuildEXRead(%v) = %q, want %q", a, got, want)
	}
	if !singleDialect.AllowedCommand(cmd.Bytes()) {
		t.Errorf("singleDialect's gate refused its own six-byte EX read %q", cmd.Bytes())
	}
	// The other forms' geometry, refused at the same gate.
	for _, wrong := range []string{"EX0803;", "EX010203;"} {
		if singleDialect.AllowedCommand([]byte(wrong)) {
			t.Errorf("singleDialect's gate ACCEPTED %q — its address field is three digits", wrong)
		}
	}
	// And the non-member refusal names all three components through the
	// debug form, as the Pair one does: a three-digit wire render drops P2
	// and P3 entirely.
	nonMember := EXAddress{P1: 8, P2: 3, P3: 7}
	if _, err := singleDialect.BuildEXRead(nonMember); err == nil {
		t.Fatal("singleDialect.BuildEXRead accepted (08,03,07), which is not a member of its inventory")
	}
}

// TestParseEXAnswer_SingleReadsBackItsOwnAnswer closes the loop at the codec:
// a three-digit-address answer parses and yields its P4 body verbatim, and
// the other forms' answers do not.
func TestParseEXAnswer_SingleReadsBackItsOwnAnswer(t *testing.T) {
	addr, raw, err := singleDialect.ParseEXAnswer([]byte("EX15312345678;"))
	if err != nil {
		t.Fatalf("singleDialect.ParseEXAnswer of its own widest answer = %v", err)
	}
	if addr != (EXAddress{P1: 153}) {
		t.Errorf("ParseEXAnswer address = %+v, want P1 153", addr)
	}
	if raw != "12345678" {
		t.Errorf("ParseEXAnswer body = %q, want %q", raw, "12345678")
	}
	if _, _, err := singleDialect.ParseEXAnswer([]byte("EX0101010;")); err == nil {
		t.Error("singleDialect.ParseEXAnswer accepted an answer carrying a six-digit address")
	}
}
