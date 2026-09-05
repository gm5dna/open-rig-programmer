// SPDX-License-Identifier: GPL-3.0-or-later

package extable

import (
	"fmt"
	"strings"
	"testing"
)

// fixtureRequired is a second-model profile that disagrees with the FT-710
// in EVERY parameterised dimension: package, type qualifier, import path,
// variable name, output file, source CSV names, digit bounds, text width,
// observation ceiling, expected row count and doc prose. A fixture that
// agrees with the FT-710 in any dimension proves nothing about that one —
// the M9b lesson is that only a fixture built to DISAGREE ever found a
// defect. It is deliberately NOT registered, so no staleness consumer goes
// looking for a generated file that does not exist.
var fixtureRequired = Profile{
	Model:       "FIXTURE",
	Package:     "ftdx10",
	Types:       TypesImported,
	ImportPath:  "github.com/gm5dna/open-rig-programmer/core/cat",
	ImportAlias: "cat",
	VarName:     "exItems",
	OutFile:     "fixture_gen.go",
	ManualCSV:   "fixture.csv",
	ObservedCSV: "fixture-observed.csv",

	Addresses:     AddressTriple,
	LabelPolicy:   LabelsRequired,
	TextRowPolicy: TextRowsAllowed,

	DigitsCeiling:    MaxDigitsCeiling,
	MinDigits:        2,
	MaxDigits:        6,
	TextWidth:        8,
	MaxObservedWidth: 9,
	ExpectedRows:     1,

	Observations: ObservationsRequired,
	DocLines:     []string{"exItems is the fixture inventory."},
}

// fixtureAbsent is fixtureRequired under the manual-only regime. The two
// exist as a pair because an ObservationsAbsent profile never reaches the
// observation-width ceiling at all, so it cannot exercise MaxObservedWidth.
var fixtureAbsent = func() Profile {
	p := fixtureRequired
	p.Observations = ObservationsAbsent
	p.ObservedCSV = ""
	p.DocLines = []string{"exItems is the fixture inventory (manual only)."}
	return p
}()

// fixturePairRequired is fixtureRequired under the OTHER address form,
// AddressPair — the FT-891's own shape, where the wire field carries P1 and
// P2 only and every row's P3 must be 0. It exists only for
// TestRenderGo_PairProfileKeysObservationsByFourDigitForm (extable_test.go):
// a fixture-only profile, never registered, so no staleness consumer goes
// looking for a generated file that does not exist.
var fixturePairRequired = func() Profile {
	p := fixtureRequired
	p.Addresses = AddressPair
	return p
}()

// withRows returns p with ExpectedRows set to n. Tests whose subject is not
// the row-count gate use it so each test asserts one thing.
func withRows(p Profile, n int) Profile {
	p.ExpectedRows = n
	return p
}

func TestProfileValidate_AcceptsRegisteredAndFixtures(t *testing.T) {
	for _, p := range []Profile{FT710Profile(), fixtureRequired, fixtureAbsent} {
		if err := p.Validate(); err != nil {
			t.Errorf("Validate() on %s: unexpected error: %v", p.Model, err)
		}
	}
}

func TestProfileValidate_Refusals(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Profile)
	}{
		{"blank Model", func(p *Profile) { p.Model = "" }},
		{"blank Package", func(p *Profile) { p.Package = "" }},
		{"Package not an identifier", func(p *Profile) { p.Package = "core/cat" }},
		{"Package is a keyword", func(p *Profile) { p.Package = "range" }},
		{"blank VarName", func(p *Profile) { p.VarName = "" }},
		{"VarName not an identifier", func(p *Profile) { p.VarName = "ex items" }},
		{"blank OutFile", func(p *Profile) { p.OutFile = "" }},
		{"absolute OutFile", func(p *Profile) { p.OutFile = "/tmp/out.go" }},
		{"OutFile equals ManualCSV", func(p *Profile) { p.OutFile = p.ManualCSV }},
		{"OutFile equals ObservedCSV", func(p *Profile) { p.OutFile = p.ObservedCSV }},
		// On APFS and on Windows these name the SAME file as the source they
		// upper-case, so byte-equality would wave them through and the next
		// `go generate` would overwrite a committed CSV.
		{"OutFile case-aliases ManualCSV", func(p *Profile) { p.OutFile = strings.ToUpper(p.ManualCSV) }},
		{"OutFile case-aliases ObservedCSV", func(p *Profile) { p.OutFile = strings.ToUpper(p.ObservedCSV) }},
		{"escaping ManualCSV", func(p *Profile) { p.ManualCSV = "../table2.csv" }},
		{"unclean ManualCSV", func(p *Profile) { p.ManualCSV = "./table2.csv" }},
		{"dot-dot ManualCSV", func(p *Profile) { p.ManualCSV = ".." }},
		{"dot ManualCSV", func(p *Profile) { p.ManualCSV = "." }},
		{"blank ManualCSV", func(p *Profile) { p.ManualCSV = "" }},
		{"omitted TypeRefPolicy", func(p *Profile) { p.Types = 0 }},
		{"unknown TypeRefPolicy", func(p *Profile) { p.Types = TypeRefPolicy(99) }},
		{"TypesImported without ImportPath", func(p *Profile) { p.ImportPath = "" }},
		{"TypesImported without ImportAlias", func(p *Profile) { p.ImportAlias = "" }},
		{"ImportAlias not an identifier", func(p *Profile) { p.ImportAlias = "not an ident" }},
		{"zero DigitsCeiling", func(p *Profile) { p.DigitsCeiling = 0 }},
		{"negative DigitsCeiling", func(p *Profile) { p.DigitsCeiling = -1 }},
		{"zero MinDigits", func(p *Profile) { p.MinDigits = 0 }},
		{"zero MaxDigits", func(p *Profile) { p.MaxDigits = 0 }},
		{"zero TextWidth", func(p *Profile) { p.TextWidth = 0 }},
		{"zero MaxObservedWidth", func(p *Profile) { p.MaxObservedWidth = 0 }},
		{"zero ExpectedRows", func(p *Profile) { p.ExpectedRows = 0 }},
		{"negative ExpectedRows", func(p *Profile) { p.ExpectedRows = -1 }},
		{"MinDigits above MaxDigits", func(p *Profile) { p.MinDigits = 7; p.MaxDigits = 6 }},
		{"MaxDigits above ceiling", func(p *Profile) { p.MaxDigits = MaxDigitsCeiling + 1 }},
		{"TextWidth above ceiling", func(p *Profile) { p.TextWidth = MaxDigitsCeiling + 1 }},
		{"MaxObservedWidth above ceiling", func(p *Profile) { p.MaxObservedWidth = MaxDigitsCeiling + 1 }},
		{"omitted ObservationPolicy", func(p *Profile) { p.Observations = 0 }},
		{"unknown ObservationPolicy", func(p *Profile) { p.Observations = ObservationPolicy(99) }},
		{"ObservedCSV blank under Required", func(p *Profile) { p.ObservedCSV = "" }},
		{"empty DocLines", func(p *Profile) { p.DocLines = nil }},
		{"blank DocLine", func(p *Profile) { p.DocLines = []string{"   "} }},
		{"DocLine with newline", func(p *Profile) { p.DocLines = []string{"a\nb"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := fixtureRequired
			tc.mut(&p)
			if err := p.Validate(); err == nil {
				t.Error("Validate() accepted an invalid profile; want an error")
			}
		})
	}
}

// TestProfileValidate_CeilingComesFromTheProfile proves the width ceiling is
// READ from the profile rather than from this package's MaxDigitsCeiling
// constant, in both directions.
//
// That constant mirrors core/cat's maxEXDigits, and core/cat's own
// exdigits_ceiling_test.go pins the two equal. Bounding a profile that renders
// into a DIFFERENT package by it would be a bound consulted from one place
// with its datum taken from another — the defect shape Profile's own doc
// comment says this type exists to prevent.
//
// Downwards is the case that matters: a family whose frame budget is narrower
// than core/cat's would have every width core/cat admits waved through here,
// and the refusal would arrive two packages downstream if at all. Upwards is
// asserted because a ceiling that silently clamped to the constant would pass
// every downward case and still not be the profile's own.
func TestProfileValidate_CeilingComesFromTheProfile(t *testing.T) {
	// 100 is far BELOW MaxDigitsCeiling's 247, so a check against the
	// constant would accept every refusal case here.
	for _, tc := range []struct {
		name       string
		mut        func(*Profile)
		wantRefuse bool
	}{
		{"a width inside the profile's own ceiling", func(p *Profile) { p.MaxDigits = 32 }, false},
		{"MaxDigits above the profile's own ceiling", func(p *Profile) { p.MaxDigits = 100 }, true},
		{"TextWidth above the profile's own ceiling", func(p *Profile) { p.TextWidth = 100 }, true},
		{"MaxObservedWidth above the profile's own ceiling", func(p *Profile) { p.MaxObservedWidth = 100 }, true},
	} {
		p := fixtureRequired
		p.DigitsCeiling = 32
		tc.mut(&p)
		err := p.Validate()
		switch {
		case !tc.wantRefuse && err != nil:
			t.Errorf("%s: Validate() = %v, want accepted", tc.name, err)
		case tc.wantRefuse && err == nil:
			t.Errorf("%s: Validate() accepted a width above the profile's ceiling of 32; want a refusal", tc.name)
		case tc.wantRefuse && !strings.Contains(err.Error(), "32"):
			t.Errorf("%s: Validate() = %v, want the refusal to name the profile's own ceiling of 32", tc.name, err)
		}
	}

	// Upwards. No registered profile does this — all four Yaesu
	// registrations carry MaxDigitsCeiling, because all four render into
	// core/cat — and it is asserted only to prove the constant is not
	// consulted behind the field's back.
	wide := fixtureRequired
	wide.DigitsCeiling = MaxDigitsCeiling + 100
	wide.MaxDigits = MaxDigitsCeiling + 50
	if err := wide.Validate(); err != nil {
		t.Errorf("Validate() refused a width inside the profile's own wider ceiling: %v", err)
	}
}

// TestProfileValidate_ObservedCSVSetUnderAbsent is separate because it
// mutates the ABSENT fixture, not the required one.
func TestProfileValidate_ObservedCSVSetUnderAbsent(t *testing.T) {
	p := fixtureAbsent
	p.ObservedCSV = "fixture-observed.csv"
	if err := p.Validate(); err == nil {
		t.Error("Validate() accepted ObservedCSV under ObservationsAbsent; want an error")
	}
}

// TestProfileValidate_TypesLocalRefusesImportFields is separate because it
// needs a TypesLocal base — the FT-710 — where the refusal table mutates the
// TypesImported fixture.
func TestProfileValidate_TypesLocalRefusesImportFields(t *testing.T) {
	t.Run("ImportPath set under TypesLocal", func(t *testing.T) {
		p := FT710Profile()
		p.ImportPath = "x/y"
		if err := p.Validate(); err == nil {
			t.Error("Validate() accepted ImportPath under TypesLocal; want an error")
		}
	})
	t.Run("ImportAlias set under TypesLocal", func(t *testing.T) {
		p := FT710Profile()
		p.ImportAlias = "cat"
		if err := p.Validate(); err == nil {
			t.Error("Validate() accepted ImportAlias under TypesLocal; want an error")
		}
	})
}

func TestValidateRegistry_RejectsDuplicatesAndEmptiness(t *testing.T) {
	a := fixtureRequired
	b := fixtureRequired

	if err := validateRegistry(map[string]Profile{}); err == nil {
		t.Error("validateRegistry accepted an empty registry; want an error")
	}

	t.Run("duplicate package and out file", func(t *testing.T) {
		b.VarName = "other"
		if err := validateRegistry(map[string]Profile{"a": a, "b": b}); err == nil {
			t.Error("accepted two profiles writing the same package/OutFile; want an error")
		}
	})
	t.Run("duplicate package and var name", func(t *testing.T) {
		b.VarName = a.VarName
		b.OutFile = "other_gen.go"
		if err := validateRegistry(map[string]Profile{"a": a, "b": b}); err == nil {
			t.Error("accepted two profiles declaring the same package variable; want an error")
		}
	})
	t.Run("same names in different packages are fine", func(t *testing.T) {
		b.Package = "other"
		b.VarName = a.VarName
		b.OutFile = a.OutFile
		if err := validateRegistry(map[string]Profile{"a": a, "b": b}); err != nil {
			t.Errorf("rejected identical names in different packages: %v", err)
		}
	})
	t.Run("blank lookup name", func(t *testing.T) {
		if err := validateRegistry(map[string]Profile{"": a}); err == nil {
			t.Error("accepted a blank lookup name; want an error — the name is the CLI-facing -profile token")
		}
	})
	t.Run("lookup name with whitespace", func(t *testing.T) {
		if err := validateRegistry(map[string]Profile{"ft 710": a}); err == nil {
			t.Error("accepted a lookup name containing whitespace; want an error")
		}
	})
	t.Run("one profile's output is another's input", func(t *testing.T) {
		// Same package directory: b's generated file would overwrite a's
		// committed source CSV on the next go generate.
		b = fixtureRequired
		b.VarName = "other"
		b.OutFile = a.ManualCSV
		b.ManualCSV = "third.csv"
		if err := validateRegistry(map[string]Profile{"a": a, "b": b}); err == nil {
			t.Error("accepted a profile whose OutFile is another profile's source CSV in the same package; want an error")
		}
	})
	t.Run("out files differing only in case", func(t *testing.T) {
		// APFS and Windows resolve these to one file: the second profile's
		// generate would silently replace the first's artefact.
		b = fixtureRequired
		b.VarName = "other"
		b.OutFile = strings.ToUpper(a.OutFile)
		if err := validateRegistry(map[string]Profile{"a": a, "b": b}); err == nil {
			t.Error("accepted two profiles whose OutFiles differ only in case; want an error")
		}
	})
	t.Run("output case-aliases another profile's input", func(t *testing.T) {
		// The destructive form of the same aliasing: b's generated file lands
		// on a's committed source CSV.
		b = fixtureRequired
		b.VarName = "other"
		b.OutFile = strings.ToUpper(a.ManualCSV)
		b.ManualCSV = "third.csv"
		if err := validateRegistry(map[string]Profile{"a": a, "b": b}); err == nil {
			t.Error("accepted a profile whose OutFile case-aliases another profile's source CSV; want an error")
		}
	})
}

func TestRegistry_LookupAndEnumeration(t *testing.T) {
	if _, ok := Lookup("no-such-model"); ok {
		t.Error("Lookup succeeded for an unregistered name")
	}
	p, ok := Lookup("ft710")
	if !ok {
		t.Fatal("Lookup(\"ft710\") failed; the FT-710 must be registered")
	}
	if p.Model != "FT-710" {
		t.Errorf("Lookup(\"ft710\").Model = %q, want \"FT-710\"", p.Model)
	}

	got := RegisteredProfiles()
	if len(got) == 0 {
		t.Fatal("RegisteredProfiles() returned nothing")
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Name >= got[i].Name {
			t.Errorf("RegisteredProfiles() not sorted: %q before %q", got[i-1].Name, got[i].Name)
		}
	}

	// Mutating a returned profile must not reach the registry.
	got[0].Profile.DocLines[0] = "mutated"
	if again := RegisteredProfiles(); again[0].Profile.DocLines[0] == "mutated" {
		t.Error("RegisteredProfiles() shares its DocLines backing array with the registry")
	}
}

// TestFT710Profile_MatchesTodaysConstants pins the profile against the
// literals it replaces, so a typo cannot silently change what is generated.
func TestFT710Profile_MatchesTodaysConstants(t *testing.T) {
	p := FT710Profile()
	if p.Model != "FT-710" {
		t.Errorf("Model = %q, want \"FT-710\"", p.Model)
	}
	if p.Package != "cat" || p.VarName != "exItemsGen" || p.Types != TypesLocal || p.ImportPath != "" || p.ImportAlias != "" {
		t.Errorf("identity drifted: %+v", p)
	}
	// The three path strings need pinning here because nothing else pins
	// OutFile at all: it is emitted nowhere in the generated bytes and read
	// by no committed test, so a typo in it would quietly write a stray file
	// on every `go generate` with the whole suite still green. ManualCSV and
	// ObservedCSV at least appear in the generated-by header, but they are
	// pinned alongside so that all three paths have one statement of record.
	if p.OutFile != "exinventory_gen.go" {
		t.Errorf("OutFile = %q, want \"exinventory_gen.go\"", p.OutFile)
	}
	if p.ManualCSV != "table2.csv" {
		t.Errorf("ManualCSV = %q, want \"table2.csv\"", p.ManualCSV)
	}
	if p.ObservedCSV != "table2-observed.csv" {
		t.Errorf("ObservedCSV = %q, want \"table2-observed.csv\"", p.ObservedCSV)
	}
	if p.MinDigits != 1 || p.MaxDigits != 4 || p.TextWidth != 12 || p.MaxObservedWidth != 12 {
		t.Errorf("bounds drifted: %+v", p)
	}
	// Every registered profile renders into core/cat, so every one of them
	// carries core/cat's own ceiling. A Kenwood profile will not: it renders
	// into core/kw and supplies that package's constant instead, which is
	// what makes the field per-family rather than global.
	if p.DigitsCeiling != MaxDigitsCeiling {
		t.Errorf("DigitsCeiling = %d, want %d (core/cat's own ceiling)", p.DigitsCeiling, MaxDigitsCeiling)
	}
	if p.ExpectedRows != 296 {
		t.Errorf("ExpectedRows = %d, want 296", p.ExpectedRows)
	}
	if p.Observations != ObservationsRequired {
		t.Errorf("Observations = %v, want ObservationsRequired", p.Observations)
	}
	if !strings.HasPrefix(p.DocLines[0], "exItemsGen is the EX address inventory") {
		t.Errorf("DocLines[0] = %q", p.DocLines[0])
	}
}

// TestFTdx10Profile_Registered pins the FTdx10's registration: that it is
// reachable under the name the go:generate directive passes, and that the
// two fields the generated file's SHAPE depends on — the package clause and
// the TypesImported/alias pair — are what core/cat/ftdx10 compiles against.
// A wrong Package or a TypesLocal here would emit a file that does not
// compile, which is a build failure a long way from its cause.
func TestFTdx10Profile_Registered(t *testing.T) {
	p, ok := Lookup("ftdx10")
	if !ok {
		t.Fatal("Lookup(\"ftdx10\") failed; the FTdx10 must be registered")
	}
	if p.Model != "FTdx10" {
		t.Errorf("Model = %q, want \"FTdx10\"", p.Model)
	}
	if p.Package != "ftdx10" {
		t.Errorf("Package = %q, want \"ftdx10\"", p.Package)
	}
	if p.Types != TypesImported {
		t.Errorf("Types = %v, want TypesImported", p.Types)
	}
	if p.ImportPath != "github.com/gm5dna/open-rig-programmer/core/cat" {
		t.Errorf("ImportPath = %q", p.ImportPath)
	}
	if p.ImportAlias != "cat" {
		t.Errorf("ImportAlias = %q, want \"cat\"", p.ImportAlias)
	}
	if p.VarName != "exItems" {
		t.Errorf("VarName = %q, want \"exItems\"", p.VarName)
	}
	// Pinned for the reason the FT-710's three paths are: OutFile appears in
	// none of the generated bytes, so a typo would quietly write a stray
	// file on every `go generate` with the suite still green.
	if p.OutFile != "exinventory_gen.go" {
		t.Errorf("OutFile = %q, want \"exinventory_gen.go\"", p.OutFile)
	}
	if p.ManualCSV != "table2.csv" {
		t.Errorf("ManualCSV = %q, want \"table2.csv\"", p.ManualCSV)
	}
	if p.ObservedCSV != "" {
		t.Errorf("ObservedCSV = %q, want empty under ObservationsAbsent", p.ObservedCSV)
	}
	if p.MinDigits != 1 || p.MaxDigits != 4 || p.TextWidth != 12 || p.MaxObservedWidth != 12 {
		t.Errorf("bounds drifted: %+v", p)
	}
	if p.DigitsCeiling != MaxDigitsCeiling {
		t.Errorf("DigitsCeiling = %d, want %d (core/cat's own ceiling)", p.DigitsCeiling, MaxDigitsCeiling)
	}
	if p.ExpectedRows != 197 {
		t.Errorf("ExpectedRows = %d, want 197 (the group-boundary ledger's count)", p.ExpectedRows)
	}
	if p.Observations != ObservationsAbsent {
		t.Errorf("Observations = %v, want ObservationsAbsent", p.Observations)
	}
	if !strings.HasPrefix(p.DocLines[0], "exItems is the FTdx10's EX address inventory") {
		t.Errorf("DocLines[0] = %q", p.DocLines[0])
	}
}

// TestFTdx101Profile_MatchesTodaysConstants pins the FTdx101D/MP's numeric
// constants as LITERALS, the way the FTdx10's are pinned above. The four
// values look identical to the FTdx10's and to the FT-710's, and that
// resemblance is exactly why they are written out here rather than compared
// against another profile: each was established from the FTdx101D/MP's OWN
// Table 2 (core/cat/ftdx101/table2.csv's provenance header), and a pin that
// read another model's field would turn a coincidence into a dependency.
//
// ExpectedRows is the one field that is NOT a chart reading by this package:
// it is the group-boundary ledger's count, derived from the rendered PDF
// before any transcription existed. Pinning it here means an edit to the
// profile's copy fails a test rather than quietly re-baselining the gate that
// RenderGo enforces.
func TestFTdx101Profile_MatchesTodaysConstants(t *testing.T) {
	p, ok := Lookup("ftdx101")
	if !ok {
		t.Fatal("Lookup(\"ftdx101\") failed; the FTdx101D/MP must be registered")
	}
	if p.Model != "FTdx101D/MP" {
		t.Errorf("Model = %q, want \"FTdx101D/MP\"", p.Model)
	}
	if p.Package != "ftdx101" {
		t.Errorf("Package = %q, want \"ftdx101\"", p.Package)
	}
	if p.Types != TypesImported {
		t.Errorf("Types = %v, want TypesImported", p.Types)
	}
	if p.ImportPath != "github.com/gm5dna/open-rig-programmer/core/cat" {
		t.Errorf("ImportPath = %q", p.ImportPath)
	}
	if p.ImportAlias != "cat" {
		t.Errorf("ImportAlias = %q, want \"cat\"", p.ImportAlias)
	}
	if p.VarName != "exItems" {
		t.Errorf("VarName = %q, want \"exItems\"", p.VarName)
	}
	if p.OutFile != "exinventory_gen.go" {
		t.Errorf("OutFile = %q, want \"exinventory_gen.go\"", p.OutFile)
	}
	if p.ManualCSV != "table2.csv" {
		t.Errorf("ManualCSV = %q, want \"table2.csv\"", p.ManualCSV)
	}
	if p.ObservedCSV != "" {
		t.Errorf("ObservedCSV = %q, want empty under ObservationsAbsent", p.ObservedCSV)
	}
	if p.MinDigits != 1 {
		t.Errorf("MinDigits = %d, want 1", p.MinDigits)
	}
	if p.MaxDigits != 4 {
		t.Errorf("MaxDigits = %d, want 4", p.MaxDigits)
	}
	if p.TextWidth != 12 {
		t.Errorf("TextWidth = %d, want 12", p.TextWidth)
	}
	if p.MaxObservedWidth != 12 {
		t.Errorf("MaxObservedWidth = %d, want 12 (the inert sentinel)", p.MaxObservedWidth)
	}
	if p.DigitsCeiling != MaxDigitsCeiling {
		t.Errorf("DigitsCeiling = %d, want %d (core/cat's own ceiling)", p.DigitsCeiling, MaxDigitsCeiling)
	}
	if p.ExpectedRows != 193 {
		t.Errorf("ExpectedRows = %d, want 193 (the group-boundary ledger's count)", p.ExpectedRows)
	}
	if p.Observations != ObservationsAbsent {
		t.Errorf("Observations = %v, want ObservationsAbsent", p.Observations)
	}
	if !strings.HasPrefix(p.DocLines[0], "exItems is the FTdx101D/MP's EX address inventory") {
		t.Errorf("DocLines[0] = %q", p.DocLines[0])
	}
}

// TestRegistry_HoldsEveryModel pins the registry's membership itself, not
// just each entry in isolation. TestRegistry_LookupAndEnumeration checks
// that RegisteredProfiles is sorted and non-empty, which one entry already
// satisfied; this asserts the EXACT set, so silently dropping a registration
// — or adding a fourth without updating this pin — is a failure rather than a
// smaller happy enumeration. The sort order is asserted by value here, not
// merely as "ascending": "ft710" < "ft891" < "ftdx10" < "ftdx101" is the
// ordering the CLI's -profile listing and every registry-selected staleness
// test see. ASCII puts "ft891" second, between the FT-710 and the FTdx10,
// which is not the order the models were added in — pinning it by value is
// how that stops being a surprise. The three Kenwood names sort after all
// four Yaesu ones only because "t" follows "f"; that is an accident of the
// lookup names, not a family grouping the registry knows about, so it too is
// pinned by value here rather than assumed.
//
// (Named for two models until M9d-1; the FTdx101D/MP made "both" wrong.)
func TestRegistry_HoldsEveryModel(t *testing.T) {
	got := RegisteredProfiles()
	var names []string
	for _, np := range got {
		names = append(names, np.Name)
	}
	want := []string{"ft710", "ft891", "ftdx10", "ftdx101", "ts480", "ts590s", "ts590sg"}
	if len(names) != len(want) {
		t.Fatalf("RegisteredProfiles() names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("RegisteredProfiles() names = %v, want %v", names, want)
		}
	}
	wantModels := []string{"FT-710", "FT-891", "FTdx10", "FTdx101D/MP", "TS-480", "TS-590S", "TS-590SG"}
	for i := range wantModels {
		if got[i].Profile.Model != wantModels[i] {
			t.Errorf("models[%d] = %q, want %q", i, got[i].Profile.Model, wantModels[i])
		}
	}
	// No two profiles may share the datum that would let one `go generate`
	// overwrite another's artefact, or read another's source CSV. That datum
	// is validateRegistry's four collision keys — OutFile, VarName,
	// ManualCSV and ObservedCSV — NOT the package clause.
	//
	// Until the FT-891 every registration lived in a package of its own, so
	// "no two share a Package" and the real rule happened to coincide; they do
	// not coincide in general, and a family whose two sibling inventories
	// belong in ONE directory is refused by the narrower reading while
	// validateRegistry accepts it. The assertion is widened rather than
	// deleted, and sharedGenerateDatum is what states it —
	// TestSharedPackageNeedsAllKeysToDiffer proves it both fires and
	// does not fire, so the widening cannot quietly become no rule at all.
	for _, v := range sharedGenerateDatum(got) {
		t.Error(v)
	}
}

// sharedGenerateDatum reports each pair in ps that shares a package AND any
// one of validateRegistry's four collision keys. A shared package alone is
// permitted: it is a directory, not a datum, and two profiles in one
// directory collide with nothing provided their output file, their generated
// variable and their source CSVs all differ. Package is a proxy for the
// output DIRECTORY here, not the collision itself: Go requires one package
// clause per directory, so two profiles that write into one directory always
// share Package, and the only failure mode this proxy can have is a false
// positive for two profiles in different directories that happen to declare
// the same package name — the safe direction.
//
// The keys fold exactly as validateRegistry's own do, and for its reasons:
// OutFile, ManualCSV and ObservedCSV are PATHS, and APFS and NTFS resolve a
// case-only difference to one file, so a byte-equal comparison would miss a
// real collision. VarName is a Go IDENTIFIER, where case is significant to
// the compiler — exItems and EXItems are two legal package-level variables —
// so folding it would refuse a pair Go itself accepts.
//
// Two of the four are also refused by validateRegistry at init. ManualCSV and
// ObservedCSV are not: that function's inputs map exists to catch one
// profile's OUTPUT landing on another's source, and it simply overwrites a
// shared input rather than refusing the second write. So this helper is the
// whole of that rule for both source keys, and what it protects is a sibling
// inventory being generated from the wrong radio's chart — from the wrong
// manual CSV, or (ObservationsRequired siblings) the wrong observation CSV.
func sharedGenerateDatum(ps []NamedProfile) []string {
	var out []string
	for i := range ps {
		for j := i + 1; j < len(ps); j++ {
			a, b := ps[i], ps[j]
			if a.Profile.Package != b.Profile.Package {
				continue
			}
			for _, k := range []struct{ key, av, bv string }{
				{"OutFile", strings.ToLower(a.Profile.OutFile), strings.ToLower(b.Profile.OutFile)},
				{"VarName", a.Profile.VarName, b.Profile.VarName},
				{"ManualCSV", strings.ToLower(a.Profile.ManualCSV), strings.ToLower(b.Profile.ManualCSV)},
			} {
				if k.av == k.bv {
					out = append(out, fmt.Sprintf("profiles %q and %q both emit into package %q and share %s %q",
						a.Name, b.Name, a.Profile.Package, k.key, k.av))
				}
			}
			// ObservedCSV is checked separately, with an empty-string guard
			// the other three keys do not need: "" is a real ManualCSV-shape
			// collision but not a real ObservedCSV one, because "" is what
			// every ObservationsAbsent profile carries (Validate refuses any
			// other value under that policy) and validateRegistry itself
			// never adds a blank ObservedCSV to its inputs map
			// (profile.go:756-757) — two absent siblings share nothing.
			if av, bv := strings.ToLower(a.Profile.ObservedCSV), strings.ToLower(b.Profile.ObservedCSV); av != "" && av == bv {
				out = append(out, fmt.Sprintf("profiles %q and %q both emit into package %q and share %s %q",
					a.Name, b.Name, a.Profile.Package, "ObservedCSV", av))
			}
		}
	}
	return out
}

// TestSharedPackageNeedsAllKeysToDiffer is the red proof each way for
// the widening above: the rule must ADMIT a shared package when all three
// keys differ, and must still REFUSE each key on its own. A widening proved
// only in the permissive direction is indistinguishable from deleting the
// assertion.
//
// Every profile here is test-local. The three Kenwood registrations land in
// their own tasks with their own CSVs; nothing in this test registers
// anything.
func TestSharedPackageNeedsAllKeysToDiffer(t *testing.T) {
	single := func(pkg, out, varName, csv string) Profile {
		p := fixtureAbsent
		p.Package = pkg
		p.OutFile = out
		p.VarName = varName
		p.ManualCSV = csv
		return p
	}
	// withObserved switches p to ObservationsRequired and gives it the named
	// observation CSV — fixtureAbsent's ObservedCSV is "" under
	// ObservationsAbsent, so the fourth key needs a fixture that carries one.
	withObserved := func(p Profile, csv string) Profile {
		p.Observations = ObservationsRequired
		p.ObservedCSV = csv
		return p
	}
	for _, tc := range []struct {
		name                string
		a, b                Profile
		wantKey             string // "" means the pair must be permitted
		wantRegistryRefusal bool
	}{
		{
			// The shape this widening exists for: one driver's two sibling
			// inventories in one package directory.
			"one package, all three keys differ",
			single("ts590", "exinventory590s_gen.go", "exItems590S", "menu590s.csv"),
			single("ts590", "exinventory590sg_gen.go", "exItems590SG", "menu590sg.csv"),
			"", false,
		},
		{
			"one package, one OutFile",
			single("ts590", "exinventory_gen.go", "exItems590S", "menu590s.csv"),
			single("ts590", "exinventory_gen.go", "exItems590SG", "menu590sg.csv"),
			"OutFile", true,
		},
		{
			// APFS and NTFS resolve these to one file, so the second
			// generate would silently replace the first's artefact.
			"one package, OutFiles differing only in case",
			single("ts590", "exinventory590s_gen.go", "exItems590S", "menu590s.csv"),
			single("ts590", "EXINVENTORY590S_GEN.GO", "exItems590SG", "menu590sg.csv"),
			"OutFile", true,
		},
		{
			"one package, one VarName",
			single("ts590", "exinventory590s_gen.go", "exItems", "menu590s.csv"),
			single("ts590", "exinventory590sg_gen.go", "exItems", "menu590sg.csv"),
			"VarName", true,
		},
		{
			// Case IS significant to the compiler, so these are two legal
			// variables and neither the helper nor validateRegistry refuses
			// them.
			"one package, VarNames differing only in case",
			single("ts590", "exinventory590s_gen.go", "exItems", "menu590s.csv"),
			single("ts590", "exinventory590sg_gen.go", "EXItems", "menu590sg.csv"),
			"", false,
		},
		{
			// validateRegistry does NOT refuse this pair, which is why the
			// assertion states the key: the cost of dropping it would be the
			// SG's inventory generated from the S's chart, with the whole
			// suite green.
			"one package, one ManualCSV",
			single("ts590", "exinventory590s_gen.go", "exItems590S", "menu590s.csv"),
			single("ts590", "exinventory590sg_gen.go", "exItems590SG", "menu590s.csv"),
			"ManualCSV", false,
		},
		{
			// validateRegistry's fourth input key (profile.go:755-757), and
			// the one MEDIUM-1 found missing here: two ObservationsRequired
			// siblings sharing an observation chart would have the SG's
			// inventory rendered from the S's hardware readings, the whole
			// suite green, and neither this helper nor validateRegistry said
			// a word. Before this case was checked, sharedGenerateDatum's
			// key table had only three rows (OutFile, VarName, ManualCSV) and
			// reported nothing for this pair — red until the fourth
			// {"ObservedCSV", ...} row below was added.
			"one package, one ObservedCSV",
			withObserved(single("ts590", "exinventory590s_gen.go", "exItems590S", "menu590s.csv"), "menu590-observed.csv"),
			withObserved(single("ts590", "exinventory590sg_gen.go", "exItems590SG", "menu590sg.csv"), "menu590-observed.csv"),
			"ObservedCSV", false,
		},
		{
			// The four Yaesu registrations' own shape: identical file,
			// variable and source names in different packages.
			"different packages, every key identical",
			single("ts590", "exinventory_gen.go", "exItems", "table2.csv"),
			single("ts480", "exinventory_gen.go", "exItems", "table2.csv"),
			"", false,
		},
	} {
		got := sharedGenerateDatum([]NamedProfile{{Name: "a", Profile: tc.a}, {Name: "b", Profile: tc.b}})
		switch {
		case tc.wantKey == "" && len(got) != 0:
			t.Errorf("%s: reported %v, want the pair permitted", tc.name, got)
		case tc.wantKey != "" && len(got) != 1:
			t.Errorf("%s: reported %v, want exactly one violation naming %s", tc.name, got, tc.wantKey)
		case tc.wantKey != "" && !strings.Contains(got[0], tc.wantKey):
			t.Errorf("%s: reported %q, want it to name %s", tc.name, got[0], tc.wantKey)
		}
		// And the same pair against validateRegistry, so the test-side rule
		// and the function-side one are compared rather than assumed equal.
		err := validateRegistry(map[string]Profile{"a": tc.a, "b": tc.b})
		if tc.wantRegistryRefusal && err == nil {
			t.Errorf("%s: validateRegistry accepted the pair; want a refusal", tc.name)
		}
		if !tc.wantRegistryRefusal && err != nil {
			t.Errorf("%s: validateRegistry refused the pair: %v", tc.name, err)
		}
	}
}

// TestFT891Profile_MatchesTodaysConstants pins the FT-891's registration as
// LITERALS, the way the FTdx10's and FTdx101D/MP's are. It is the first
// registration to declare the three chart-shape policies in their MINORITY
// values, so each is asserted by name here rather than left to the
// registry-wide sweep in policies_test.go, which now has two populations to
// keep apart.
//
// The three numeric bounds are the FT-891's OWN chart readings, recorded in
// core/cat/ft891/table2.csv's provenance header: Digits runs 1..5, the 5
// coming from exactly two rows — 0803 OTHER DISP (ft891_layout.txt:595) and
// 0804 OTHER SHIFT (:596), whose signed "-3000 ... +3000" parameter counts
// its sign — and there is no text row anywhere in the chart, so TextWidth is
// 0 rather than the 12 the other three profiles carry.
//
// ExpectedRows is NOT a reading by this package: it is the committed
// group-boundary ledger's sum, in the sense the ftdx10 and ftdx101 profiles
// record. Pinning it here means an edit to the profile's copy fails a test
// rather than quietly re-baselining RenderGo's completeness gate.
func TestFT891Profile_MatchesTodaysConstants(t *testing.T) {
	p, ok := Lookup("ft891")
	if !ok {
		t.Fatal("Lookup(\"ft891\") failed; the FT-891 must be registered")
	}
	if p.Model != "FT-891" {
		t.Errorf("Model = %q, want \"FT-891\"", p.Model)
	}
	if p.Package != "ft891" {
		t.Errorf("Package = %q, want \"ft891\"", p.Package)
	}
	if p.Types != TypesImported {
		t.Errorf("Types = %v, want TypesImported", p.Types)
	}
	if p.ImportPath != "github.com/gm5dna/open-rig-programmer/core/cat" {
		t.Errorf("ImportPath = %q", p.ImportPath)
	}
	if p.ImportAlias != "cat" {
		t.Errorf("ImportAlias = %q, want \"cat\"", p.ImportAlias)
	}
	if p.VarName != "exItems" {
		t.Errorf("VarName = %q, want \"exItems\"", p.VarName)
	}
	if p.OutFile != "exinventory_gen.go" {
		t.Errorf("OutFile = %q, want \"exinventory_gen.go\"", p.OutFile)
	}
	if p.ManualCSV != "table2.csv" {
		t.Errorf("ManualCSV = %q, want \"table2.csv\"", p.ManualCSV)
	}
	if p.ObservedCSV != "" {
		t.Errorf("ObservedCSV = %q, want empty under ObservationsAbsent", p.ObservedCSV)
	}
	if p.Addresses != AddressPair {
		t.Errorf("Addresses = %v, want AddressPair — the FT-891's EX field is four digits", p.Addresses)
	}
	if p.LabelPolicy != LabelsAbsent {
		t.Errorf("LabelPolicy = %v, want LabelsAbsent — the chart prints no group labels", p.LabelPolicy)
	}
	if p.TextRowPolicy != TextRowsAbsent {
		t.Errorf("TextRowPolicy = %v, want TextRowsAbsent — the chart prints no free-text row", p.TextRowPolicy)
	}
	if p.MinDigits != 1 {
		t.Errorf("MinDigits = %d, want 1", p.MinDigits)
	}
	if p.MaxDigits != 5 {
		t.Errorf("MaxDigits = %d, want 5 (0803 and 0804, ft891_layout.txt:595-596)", p.MaxDigits)
	}
	if p.TextWidth != 0 {
		t.Errorf("TextWidth = %d, want 0 under TextRowsAbsent", p.TextWidth)
	}
	if p.MaxObservedWidth != 12 {
		t.Errorf("MaxObservedWidth = %d, want 12 (the inert sentinel)", p.MaxObservedWidth)
	}
	if p.DigitsCeiling != MaxDigitsCeiling {
		t.Errorf("DigitsCeiling = %d, want %d (core/cat's own ceiling)", p.DigitsCeiling, MaxDigitsCeiling)
	}
	if p.ExpectedRows != 159 {
		t.Errorf("ExpectedRows = %d, want 159 (the group-boundary ledger's count)", p.ExpectedRows)
	}
	if p.Observations != ObservationsAbsent {
		t.Errorf("Observations = %v, want ObservationsAbsent", p.Observations)
	}
	if !strings.HasPrefix(p.DocLines[0], "exItems is the FT-891's EX address inventory") {
		t.Errorf("DocLines[0] = %q", p.DocLines[0])
	}
}
