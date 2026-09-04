// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"strings"
	"testing"
)

// This file is NEW, and every case in it is new. It is a separate file
// rather than an addition to ex_test.go or dialectvalidate_test.go on
// purpose: both of those are pinned literal-by-literal and ORDINAL-by-
// ordinal in core/cat/testdata/evidence-literals.golden, so a single new
// string constant inserted anywhere but the very end of the file shifts
// every record after it. A new file carries no pinned ordinals at all.

// TestEXAddressForm_String pins the three renderings V12's error text and
// every test failure message are read through. The zero value has NO name
// of its own — the form constants start at iota+1 — so it falls to the
// numeric default, which is what makes "must be set explicitly" legible.
func TestEXAddressForm_String(t *testing.T) {
	for _, tc := range []struct {
		form EXAddressForm
		want string
	}{
		{EXAddressTriple, "EXAddressTriple"},
		{EXAddressPair, "EXAddressPair"},
		{EXAddressForm(0), "EXAddressForm(0)"},
		{EXAddressForm(99), "EXAddressForm(99)"},
	} {
		if got := tc.form.String(); got != tc.want {
			t.Errorf("EXAddressForm(%d).String() = %q, want %q", int(tc.form), got, tc.want)
		}
	}
}

// TestWireEXAddress_RendersPerForm is the whole seam in one assertion: the
// SAME address becomes six digits under one form and four under the other,
// and the four-digit render drops P3 — which is safe only because V12 has
// already required every Pair member's P3 to be zero.
//
// The formless case renders "" deliberately. A dialect that never declared
// a form has no wire address; NewDialect refuses to build one (V12), so the
// only receiver that can reach this branch is the hand-built zero Dialect,
// which is inert by design. Returning "" rather than a plausible six digits
// is what keeps EXAddressWidth() (which measures this render) honest for it.
func TestWireEXAddress_RendersPerForm(t *testing.T) {
	for _, tc := range []struct {
		name string
		form EXAddressForm
		addr EXAddress
		want string
	}{
		{"triple 1,2,3", EXAddressTriple, EXAddress{P1: 1, P2: 2, P3: 3}, "010203"},
		{"triple 8,3,0", EXAddressTriple, EXAddress{P1: 8, P2: 3, P3: 0}, "080300"},
		{"triple zero value", EXAddressTriple, EXAddress{}, "000000"},
		{"pair 8,3,0", EXAddressPair, EXAddress{P1: 8, P2: 3, P3: 0}, "0803"},
		{"pair 1,2,3 drops P3", EXAddressPair, EXAddress{P1: 1, P2: 2, P3: 3}, "0102"},
		{"formless renders nothing", EXAddressForm(0), EXAddress{P1: 1, P2: 2, P3: 3}, ""},
	} {
		if got := wireEXAddress(tc.form, tc.addr); got != tc.want {
			t.Errorf("%s: wireEXAddress(%v, %+v) = %q, want %q", tc.name, tc.form, tc.addr, got, tc.want)
		}
	}
}

// TestDialect_EXWireAndWidth holds the two accessors to each other and to
// the renderer: EXWire is wireEXAddress with this dialect's form supplied,
// and EXAddressWidth is the LENGTH of that render rather than a second
// table of 6 and 4. A width consulted from anywhere but the datum it
// measures is the drift shape this repository has paid for four times.
func TestDialect_EXWireAndWidth(t *testing.T) {
	for _, tc := range []struct {
		name      string
		d         Dialect
		addr      EXAddress
		wantWire  string
		wantWidth int
	}{
		{"FT710 is Triple", FT710, EXAddress{P1: 1, P2: 2, P3: 3}, "010203", 6},
		{"pairDialect is Pair", pairDialect, EXAddress{P1: 8, P2: 3, P3: 0}, "0803", 4},
		{"zero Dialect has no form", Dialect{}, EXAddress{P1: 1, P2: 2, P3: 3}, "", 0},
	} {
		if got := tc.d.EXWire(tc.addr); got != tc.wantWire {
			t.Errorf("%s: EXWire(%+v) = %q, want %q", tc.name, tc.addr, got, tc.wantWire)
		}
		if got := tc.d.EXAddressWidth(); got != tc.wantWidth {
			t.Errorf("%s: EXAddressWidth() = %d, want %d", tc.name, got, tc.wantWidth)
		}
		if got, want := tc.d.EXAddressWidth(), len(tc.d.EXWire(tc.addr)); got != want {
			t.Errorf("%s: EXAddressWidth() = %d but its own EXWire render is %d bytes — the bound has stopped being measured from its datum", tc.name, got, want)
		}
	}
}

// TestEveryDialect_FrameLengthsFollowTheAddressWidth pins the three derived
// lengths against the width they are derived from, for every configured
// dialect. It is the property that makes a four-digit family expressible at
// all: exReadLen was a package const of 9 until this seam, read through a
// Dialect receiver.
func TestEveryDialect_FrameLengthsFollowTheAddressWidth(t *testing.T) {
	for _, nd := range allTestDialects() {
		w := nd.dia.EXAddressWidth()
		if got, want := nd.dia.exReadLen(), 2+w+1; got != want {
			t.Errorf("%s: exReadLen() = %d, want %d (2 + width %d + 1)", nd.name, got, want, w)
		}
		if got, want := nd.dia.exAnswerMinLen(), 2+w+1+1; got != want {
			t.Errorf("%s: exAnswerMinLen() = %d, want %d", nd.name, got, want)
		}
		if got, want := nd.dia.exAnswerMaxLen(), 2+w+nd.dia.exP4MaxBytes()+1; got != want {
			t.Errorf("%s: exAnswerMaxLen() = %d, want %d", nd.name, got, want)
		}
	}
	if got := pairDialect.exReadLen(); got != 7 {
		t.Errorf("pairDialect.exReadLen() = %d, want 7 — the four-digit read frame is \"EX\"+4+\";\"", got)
	}
	if got := FT710.exReadLen(); got != 9 {
		t.Errorf("FT710.exReadLen() = %d, want 9 — the six-digit read frame must not have moved", got)
	}
}

// TestEXAddressString_IsNotAWireField is the negative half of deleting
// EXAddress.Wire(). String() survived the deletion, so the risk it leaves
// behind is that a debug print is mistaken for — or fed back as — a wire
// field. Two independent guards: it must contain a byte no address field
// may carry, and EVERY configured dialect's own ParseEXAddress must refuse
// it, whatever that dialect's width.
func TestEXAddressString_IsNotAWireField(t *testing.T) {
	addrs := []EXAddress{{}, {P1: 1, P2: 2, P3: 3}, {P1: 8, P2: 3, P3: 0}, {P1: 99, P2: 99, P3: 99}}
	for _, a := range addrs {
		s := a.String()
		if !strings.ContainsFunc(s, func(r rune) bool { return r < '0' || r > '9' }) {
			t.Errorf("EXAddress%+v.String() = %q, which is all digits — a debug form that could be read back as an address field is exactly what deleting Wire() was meant to end", a, s)
		}
		for _, nd := range allTestDialects() {
			if _, err := nd.dia.ParseEXAddress(s); err == nil {
				t.Errorf("%s: ParseEXAddress(%q) accepted the debug String() form", nd.name, s)
			}
		}
	}
	// The debug form must still SAY everything the wire form can lose: a
	// Pair render drops P3, so a non-zero P3 has to survive here.
	if got, want := (EXAddress{P1: 8, P2: 3, P3: 7}).String(), "P1=08 P2=03 P3=07"; got != want {
		t.Errorf("EXAddress{8,3,7}.String() = %q, want %q", got, want)
	}
}

// TestValidateDialectConfig_V12 covers both clauses of the new rule in both
// directions. The zero form must be REFUSED, not defaulted (the M9c-1
// ruling): defaulting to Triple would give a four-digit radio a frame two
// bytes too long, built, gate-approved and sent.
func TestValidateDialectConfig_V12(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*DialectConfig)
		wantErr string // "" means the config MUST be accepted
	}{
		{"baseline is valid", func(*DialectConfig) {}, ""},
		{"V12 zero form refused", func(c *DialectConfig) { c.EXAddressForm = 0 }, "EXAddressForm"},
		{"V12 unknown form refused", func(c *DialectConfig) { c.EXAddressForm = EXAddressForm(9) }, "EXAddressForm"},
		{"V12 Triple accepts a non-zero P3", func(c *DialectConfig) { c.EXItems[0].Addr.P3 = 7 }, ""},
		{"V12 Pair with an all-zero-P3 inventory accepted", func(c *DialectConfig) {
			c.EXAddressForm = EXAddressPair
			c.EXItems[0].Addr.P3, c.EXItems[1].Addr.P3 = 0, 0
			c.EXItems[1].Addr.P2 = 9 // keep the two addresses distinct once P3 is gone
		}, ""},
		{"V12 Pair with a non-zero P3 refused", func(c *DialectConfig) {
			c.EXAddressForm = EXAddressPair
			c.EXItems[0].Addr.P3, c.EXItems[1].Addr.P3 = 0, 0
			c.EXItems[1].Addr.P2, c.EXItems[1].Addr.P3 = 9, 4
		}, "P3"},
		{"V12 Pair empty inventory accepted", func(c *DialectConfig) {
			c.EXAddressForm, c.EXItems = EXAddressPair, nil
		}, ""},
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

// TestValidateDialectConfig_V12PairErrorNamesTheAddress pins that the
// refusal identifies WHICH member offends, rendered through the same
// renderer the frame would have used. An error that says only "some member
// has a non-zero P3" leaves a 300-row inventory to be searched by hand.
func TestValidateDialectConfig_V12PairErrorNamesTheAddress(t *testing.T) {
	cfg := validBaselineConfig()
	cfg.EXAddressForm = EXAddressPair
	cfg.EXItems = []EXItem{
		{Addr: EXAddress{P1: 8, P2: 1, P3: 0}, Name: "FINE", Digits: 2},
		{Addr: EXAddress{P1: 8, P2: 3, P3: 7}, Name: "OFFENDER", Digits: 2},
	}
	err := validateDialectConfig(cfg)
	if err == nil {
		t.Fatal("validateDialectConfig() accepted a Pair config whose second member has P3 == 7")
	}
	for _, want := range []string{"EXItems[1]", "0803", "EXAddressPair"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("V12 refusal %q does not contain %q", err, want)
		}
	}
}

// TestValidateEXItems_TripleErrorTextIsByteIdentical is a BYTE-IDENTITY
// pin, not a behaviour test. V8's three messages rendered the address
// through EXAddress's own %v — which was the six-digit wire form — and now
// render it through wireEXAddress with the config's form supplied. The
// Triple text must not have moved by a single byte: the milestone's
// standing claim is that every existing error text survives Stage 0
// unchanged, and these three are the ones the change passes through.
//
// The expected strings are written out in full here rather than composed
// from the code under test, which is the whole point of a pin.
func TestValidateEXItems_TripleErrorTextIsByteIdentical(t *testing.T) {
	dup := EXAddress{P1: 7, P2: 1, P3: 1}
	for _, tc := range []struct {
		name  string
		items []EXItem
		want  string
	}{
		{
			"duplicate address",
			[]EXItem{
				{Addr: dup, Name: "ITEM ONE", Digits: 2},
				{Addr: dup, Name: "ITEM TWO", Digits: 2},
			},
			"cat: EXItems[1] repeats address 070101, already at index 0",
		},
		{
			"digits below one",
			[]EXItem{{Addr: dup, Name: "ITEM ONE", Digits: 0}},
			"cat: EXItems[0] (070101) has Digits 0, want >= 1",
		},
		{
			"digits above the frame bound",
			[]EXItem{{Addr: dup, Name: "ITEM ONE", Digits: maxEXDigits + 1}},
			fmt.Sprintf("cat: EXItems[0] (070101) has Digits %d, want <= %d — a wider P4 describes an answer frame longer than DefaultMaxFrame (%d), which this dialect's own transport could never assemble", maxEXDigits+1, maxEXDigits, DefaultMaxFrame),
		},
	} {
		cfg := validBaselineConfig()
		cfg.EXItems = tc.items
		err := validateEXItems(cfg)
		if err == nil {
			t.Errorf("%s: validateEXItems() accepted the config", tc.name)
			continue
		}
		if err.Error() != tc.want {
			t.Errorf("%s: validateEXItems() error text moved\n    was: %s\n    now: %s", tc.name, tc.want, err.Error())
		}
	}
}

// TestParseEXAddress_RefusalTextNamesTheFormsWidthInWords pins both
// spellings. "six" is the shipped text — parser-corpus.golden line 145
// carries it verbatim — and "four" is its Pair counterpart. Two literals
// selected by form rather than one composed from a number, because the
// shipped one has to survive byte for byte.
func TestParseEXAddress_RefusalTextNamesTheFormsWidthInWords(t *testing.T) {
	for _, tc := range []struct {
		name  string
		d     Dialect
		wire  string
		wants []string
	}{
		{"Triple, wrong length", FT710, "01010", []string{"exactly six digits"}},
		{"Triple, non-digit", FT710, "01010X", []string{"six ASCII digits"}},
		{"Pair, wrong length", pairDialect, "080", []string{"exactly four digits"}},
		{"Pair, non-digit", pairDialect, "08X3", []string{"four ASCII digits"}},
		{"formless dialect refuses every field", Dialect{}, "010101", []string{"address"}},
	} {
		_, err := tc.d.ParseEXAddress(tc.wire)
		if err == nil {
			t.Errorf("%s: ParseEXAddress(%q) succeeded, want a refusal", tc.name, tc.wire)
			continue
		}
		for _, want := range tc.wants {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: ParseEXAddress(%q) = %v, want it to say %q", tc.name, tc.wire, err, want)
			}
		}
	}
	// The exact shipped Triple sentence, byte for byte.
	if _, err := FT710.ParseEXAddress("01010"); err == nil || err.Error() != `cat: parse error: EX address must be exactly six digits (input="01010")` {
		t.Errorf("the shipped Triple length refusal moved: %v", err)
	}
}

// TestEXFramesOfTheWrongWidthAreRefused is the disagreeing-fixture pair the
// spec names: each form fed the OTHER form's geometry, at both the codec
// and the gate. Without it a length check hardwired to 9 (or to 7) passes
// every positive test in this package.
func TestEXFramesOfTheWrongWidthAreRefused(t *testing.T) {
	// A real member of each inventory, so nothing here is refused for
	// membership when the point is width.
	tripleAddr := FT710.EXAddresses()[0]
	pairAddr := pairDialect.EXAddresses()[0]

	// Positive controls first: each dialect accepts its OWN geometry.
	own := map[string][]byte{
		"FT710":       []byte("EX" + FT710.EXWire(tripleAddr) + ";"),
		"pairDialect": []byte("EX" + pairDialect.EXWire(pairAddr) + ";"),
	}
	if len(own["FT710"]) != 9 || len(own["pairDialect"]) != 7 {
		t.Fatalf("fixture broken: read frames are %d and %d bytes, want 9 and 7", len(own["FT710"]), len(own["pairDialect"]))
	}
	if !FT710.AllowedCommand(own["FT710"]) {
		t.Errorf("FT710's gate refused its own 9-byte EX read %q", own["FT710"])
	}
	if !pairDialect.AllowedCommand(own["pairDialect"]) {
		t.Errorf("pairDialect's gate refused its own 7-byte EX read %q", own["pairDialect"])
	}

	// The FT-710 fed the Pair form's seven-byte read and four-digit answer.
	if FT710.AllowedCommand(own["pairDialect"]) {
		t.Errorf("FT710's gate ACCEPTED %q, a seven-byte EX read — its address field is six digits", own["pairDialect"])
	}
	if _, _, err := FT710.ParseEXAnswer([]byte("EX01010;")); err == nil {
		t.Error("FT710.ParseEXAnswer accepted a four-digit-address answer")
	}
	// The Pair dialect fed the Triple form's nine-byte read and six-digit
	// answer.
	if pairDialect.AllowedCommand(own["FT710"]) {
		t.Errorf("pairDialect's gate ACCEPTED %q, a nine-byte EX read — its address field is four digits", own["FT710"])
	}
	// A six-digit-address answer put to the Pair dialect is refused by
	// MEMBERSHIP, not by width, and that asymmetry is worth stating: P4 is
	// variable-width, so "EX" + six digits + a body is indistinguishable on
	// LENGTH from "EX" + four digits + a two-byte-longer body. What the
	// four-digit dialect does see is that the leading four digits of the
	// FT-710's own address are not one of its addresses.
	sixDigit := []byte("EX" + FT710.EXWire(tripleAddr) + "0;")
	if _, _, err := pairDialect.ParseEXAnswer(sixDigit); err == nil {
		t.Errorf("pairDialect.ParseEXAnswer(%q) accepted an answer carrying the FT-710's six-digit address", sixDigit)
	}
	// And each still round-trips its own answer, so the refusals above are
	// not a parser that refuses everything.
	if _, raw, err := pairDialect.ParseEXAnswer([]byte("EX0801" + "12" + ";")); err != nil || raw != "12" {
		t.Errorf("pairDialect.ParseEXAnswer(%q) = (%q, %v), want (\"12\", nil)", "EX080112;", raw, err)
	}
	if _, raw, err := FT710.ParseEXAnswer([]byte("EX" + FT710.EXWire(tripleAddr) + "0;")); err != nil || raw != "0" {
		t.Errorf("FT710.ParseEXAnswer of its own minimal answer = (%q, %v), want (\"0\", nil)", raw, err)
	}
}

// TestBuildEXRead_UsesThisDialectsWidth pins the builder end: the frame a
// Pair dialect builds is seven bytes and carries four address digits.
func TestBuildEXRead_UsesThisDialectsWidth(t *testing.T) {
	a := pairDialect.EXAddresses()[1] // (08,03,00)
	cmd, err := pairDialect.BuildEXRead(a)
	if err != nil {
		t.Fatalf("pairDialect.BuildEXRead(%v) = %v", a, err)
	}
	if got, want := string(cmd.Bytes()), "EX0803;"; got != want {
		t.Errorf("pairDialect.BuildEXRead(%v) = %q, want %q", a, got, want)
	}
	if _, err := pairDialect.BuildEXRead(EXAddress{P1: 8, P2: 3, P3: 7}); err == nil {
		t.Error("pairDialect.BuildEXRead accepted (08,03,07), which is not a member of its inventory")
	}
}
