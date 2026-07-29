// SPDX-License-Identifier: GPL-3.0-or-later

package extable

import (
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

// TestRegistry_HoldsBothModels pins the registry's membership itself, not
// just each entry in isolation. TestRegistry_LookupAndEnumeration checks
// that RegisteredProfiles is sorted and non-empty, which one entry already
// satisfied; this asserts the EXACT set, so silently dropping a registration
// — or adding a third without updating this pin — is a failure rather than a
// smaller happy enumeration. The sort order is asserted by value here, not
// merely as "ascending": "ft710" < "ftdx10" is the ordering the CLI's
// -profile listing and every registry-selected staleness test see.
func TestRegistry_HoldsBothModels(t *testing.T) {
	got := RegisteredProfiles()
	var names []string
	for _, np := range got {
		names = append(names, np.Name)
	}
	want := []string{"ft710", "ftdx10"}
	if len(names) != len(want) {
		t.Fatalf("RegisteredProfiles() names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("RegisteredProfiles() names = %v, want %v", names, want)
		}
	}
	if got[0].Profile.Model != "FT-710" || got[1].Profile.Model != "FTdx10" {
		t.Errorf("models = %q, %q; want \"FT-710\", \"FTdx10\"", got[0].Profile.Model, got[1].Profile.Model)
	}
	// The two profiles must not share the datum that would let one
	// `go generate` overwrite the other's artefact. validateRegistry already
	// refuses a collision at init; this states the expected separation.
	if got[0].Profile.Package == got[1].Profile.Package {
		t.Errorf("both profiles emit into package %q", got[0].Profile.Package)
	}
}
