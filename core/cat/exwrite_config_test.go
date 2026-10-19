// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"reflect"
	"strings"
	"testing"
)

var (
	cfgSigned = EXAddress{P1: 7, P2: 1, P3: 1} // Digits 3, -20..+10
	cfgEnum   = EXAddress{P1: 7, P2: 1, P3: 2} // Digits 2, {0,1,2}
	cfgRange  = EXAddress{P1: 7, P2: 1, P3: 3} // Digits 4, {1000} + 0..100
)

// exWriteConfig is a baseline config with a three-row write table, one
// signed, one enum, one hybrid.
func exWriteConfig(paper bool) DialectConfig {
	cfg := validBaselineConfig()
	cfg.EXItems = []EXItem{
		{Addr: cfgSigned, Name: "SIGNED", Digits: 3},
		{Addr: cfgEnum, Name: "ENUM", Digits: 2},
		{Addr: cfgRange, Name: "RANGE", Digits: 4},
	}
	cfg.EXWrite = []EXWriteItem{
		{Addr: cfgSigned, Domain: Domain{Lo: -20, Hi: 10, Step: 1, Signed: true}},
		{Addr: cfgEnum, Domain: Domain{Codes: []int{0, 1, 2}}},
		{Addr: cfgRange, Domain: Domain{Codes: []int{1000}, Lo: 0, Hi: 100, Step: 1}},
	}
	cfg.EXWritePaperWidths = paper
	return cfg
}

func TestValidateEXWrite_Rules(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*DialectConfig)
		want string // "" = accepted
	}{
		{"signed -20 at Digits 3", func(c *DialectConfig) {}, ""},
		{"unknown address", func(c *DialectConfig) { c.EXWrite[0].Addr = EXAddress{P1: 9} }, "not in EXItems"},
		{"duplicate address", func(c *DialectConfig) { c.EXWrite[1].Addr = cfgSigned }, "repeats address"},
		{"zero Domain", func(c *DialectConfig) { c.EXWrite[1].Domain = Domain{} }, "zero Domain"},
		{"Lo > Hi", func(c *DialectConfig) { c.EXWrite[0].Domain.Lo = 11 }, "Lo"},
		{"Step < 1", func(c *DialectConfig) { c.EXWrite[2].Domain.Step = 0 }, "Step"},
		{"duplicate code", func(c *DialectConfig) { c.EXWrite[1].Domain.Codes = []int{1, 1} }, "repeats code"},
		{"negative code", func(c *DialectConfig) { c.EXWrite[1].Domain.Codes = []int{-1} }, "negative code"},
		{"code inside range", func(c *DialectConfig) { c.EXWrite[2].Domain.Codes = []int{50} }, "inside its range"},
		{"+200 at Digits 3", func(c *DialectConfig) { c.EXWrite[0].Domain.Hi = 200 }, "does not fit"},
		{"code too wide", func(c *DialectConfig) { c.EXWrite[1].Domain.Codes = []int{100} }, "does not fit"},
		{"paper plus observed width", func(c *DialectConfig) {
			c.EXWritePaperWidths = true
			c.EXWrite[1].ObservedSetWidth = 2
		}, "contradict"},
	}
	for _, tc := range cases {
		cfg := exWriteConfig(false)
		tc.mut(&cfg)
		err := validateEXWrite(cfg)
		switch {
		case tc.want == "" && err != nil:
			t.Errorf("%s: %v", tc.name, err)
		case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
			t.Errorf("%s: error %v, want it to contain %q", tc.name, err, tc.want)
		}
	}
	// V8 refuses Digits < 1 first, so the paper Digits-0 clause is driven
	// through the rule directly.
	cfg := exWriteConfig(true)
	cfg.EXItems[1].Digits = 0
	if err := validateEXWrite(cfg); err == nil || !strings.Contains(err.Error(), "paper width") {
		t.Errorf("paper with Digits 0: %v", err)
	}
	// And through NewDialect: a bad write table is refused at construction.
	bad := exWriteConfig(false)
	bad.EXWrite[0].Addr = EXAddress{P1: 9}
	if _, err := NewDialect(bad); err == nil {
		t.Error("NewDialect accepted an EXWrite row with an unknown address")
	}
}

func TestNewDialect_EXWriteWidths(t *testing.T) {
	plain, paper := MustNewDialect(exWriteConfig(false)), MustNewDialect(exWriteConfig(true))
	for _, a := range []EXAddress{cfgSigned, cfgEnum, cfgRange} {
		if _, w, ok := plain.EXWriteDescriptor(a); !ok || w != 0 {
			t.Errorf("unpaper %v: width %d admitted %v, want 0 true", a, w, ok)
		}
	}
	if plain.exWritePaper || !paper.exWritePaper {
		t.Error("exWritePaper not recorded")
	}
	if _, err := paper.BuildEXSet(cfgSigned, "-20"); err != nil {
		t.Errorf("paper dialect refused -20 at Digits 3: %v", err)
	}
	if _, err := paper.BuildEXSet(cfgSigned, "+11"); err == nil {
		t.Error("paper dialect accepted +11, outside -20..+10")
	}
}

// TestNewDialect_EXWriteNoAliasing: the paper and unpaper dialects built
// from one config share no map and no Codes backing array with each other
// or with the config, and exercising the paper one never opens the other.
func TestNewDialect_EXWriteNoAliasing(t *testing.T) {
	cfg := exWriteConfig(false)
	plain := MustNewDialect(cfg)
	pcfg := cfg
	pcfg.EXWritePaperWidths = true
	paper := MustNewDialect(pcfg)

	if reflect.ValueOf(plain.exWrite).Pointer() == reflect.ValueOf(paper.exWrite).Pointer() {
		t.Fatal("paper and unpaper dialects share one exWrite map")
	}
	for _, a := range []EXAddress{cfgEnum, cfgRange} {
		pc, qc := plain.exWrite[a].Domain.Codes, paper.exWrite[a].Domain.Codes
		if &pc[0] == &qc[0] {
			t.Errorf("%v: Codes backing arrays shared between dialects", a)
		}
		if &pc[0] == &cfg.EXWrite[map[EXAddress]int{cfgEnum: 1, cfgRange: 2}[a]].Domain.Codes[0] {
			t.Errorf("%v: Codes backing array shared with the config", a)
		}
	}

	// Mutate the config after construction: behaviour unchanged.
	cfg.EXWrite[1].Domain.Codes[0] = 2
	cfg.EXWrite[0] = EXWriteItem{}
	for _, d := range []Dialect{plain, paper} {
		if _, _, ok := d.EXWriteDescriptor(cfgSigned); !ok {
			t.Error("config mutation removed a row from a built dialect")
		}
		if dom, _, _ := d.EXWriteDescriptor(cfgEnum); dom.Codes[0] != 0 {
			t.Error("config mutation changed a built dialect's Codes")
		}
	}

	// Drive the paper dialect through its accepting path for every address,
	// then assert the unpaper one still refuses all of them everywhere.
	for a, v := range map[EXAddress]string{cfgSigned: "+02", cfgEnum: "01", cfgRange: "1000"} {
		cmd, err := paper.BuildEXSet(a, v)
		if err != nil || !paper.AllowedCommand(cmd.Bytes()) || !paper.CanSetEX(a) {
			t.Fatalf("paper dialect refused %v=%s: %v", a, v, err)
		}
		frame := "EX" + plain.EXWire(a) + v + ";"
		if plain.AllowedCommand([]byte(frame)) || plain.CanSetEX(a) {
			t.Errorf("unpaper dialect admits %v after the paper twin was used", a)
		}
		if _, err := plain.BuildEXSet(a, v); err == nil {
			t.Errorf("unpaper BuildEXSet accepted %v=%s", a, v)
		}
	}
}

// TestNewDialect_EXWriteGateClosedWithoutPaper: a config-built dialect with
// no paper widths and no observations refuses every EX Set, whatever the
// write table holds.
func TestNewDialect_EXWriteGateClosedWithoutPaper(t *testing.T) {
	d := MustNewDialect(exWriteConfig(false))
	for _, it := range d.EXItems() {
		if d.CanSetEX(it.Addr) {
			t.Errorf("CanSetEX(%v) = true on an unpaper dialect", it.Addr)
		}
		if _, err := d.BuildEXSet(it.Addr, strings.Repeat("0", it.Digits)); err == nil {
			t.Errorf("BuildEXSet(%v) accepted on an unpaper dialect", it.Addr)
		}
		if d.AllowedCommand([]byte("EX" + d.EXWire(it.Addr) + strings.Repeat("0", it.Digits) + ";")) {
			t.Errorf("AllowedCommand admits an EX Set for %v on an unpaper dialect", it.Addr)
		}
	}
}
