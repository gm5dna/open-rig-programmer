# M9c-2 extable parameterisation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `internal/extable` able to generate a second radio model's EX
inventory into its own package, without changing a single byte of the
FT-710's generated file.

**Architecture:** Every FT-710-specific literal in `internal/extable` moves
into a per-model `Profile` value held in a package registry. `ParseCSV`,
`ParseObservedCSV` and `RenderGo` all take that profile, so the generator
and every staleness test read one source and cannot drift. A second profile,
used only in tests, disagrees with the FT-710 in every parameterised
dimension and proves each bound is read rather than hardcoded.

**Tech Stack:** Go 1.25, standard library only. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-07-28-m9c2-extable-parameterisation-design.md`
(revision 2). Read it before starting. Review adjudication:
`.superpowers/sdd/m9c2-spec-review-adjudication.md`.

## Global Constraints

- **British English** in all prose, comments and documentation.
- **SPDX header** `// SPDX-License-Identifier: GPL-3.0-or-later` on every new
  `.go` file, followed by a blank line.
- **`gofmt -l .` must report nothing** at the end of every task. This is not
  optional: every one of M9c-1's per-task reviews missed gofmt drift because
  reviewers were told not to re-run the suite, and only the final gate
  caught it.
- **`core/cat/exinventory_gen.go` must never change.** `git diff --exit-code
  -- core/cat/exinventory_gen.go` must pass at the end of every task.
- **Never regenerate `core/cat/testdata/evidence-literals.golden`.** See the
  boxed warning below.
- **The file lists in this plan are hypotheses, not inventories.** Five of
  five M9c-1 briefs carried incomplete call-site lists. Each task's final
  step re-runs the repository-wide grep; the grep is the gate, not the list.
- **Never regenerate a golden** in `core/cat/testdata/` — it must stay at
  exactly two commits (`ff5c19b`, `1d38941`).
- `go test -race ./core/...` exceeds a ten-minute foreground limit; run it in
  the background if you run it at all.

> ### ⚠️ The evidence-literal trap — read before touching `core/cat`
>
> `core/cat/evidence_literals_test.go` walks **every** `*_test.go` file in
> `core/cat` and records each STRING, CHAR and INT literal by file and
> ordinal. `core/cat/testdata/evidence-literals.golden` pins fourteen
> literals of `exinventory_stale_test.go` at lines 541-554, including
> `"table2.csv"` (#4) and `"exinventory_gen.go"` (#11).
>
> `TestEvidenceLiterals_OrderedRecordsSurvive` is a **survival** check: it
> walks the golden and requires each `(file, ordinal)` still to hold the same
> token. Therefore:
>
> - **Safe:** adding a whole new `_test.go` file to `core/cat` (it has no
>   golden records); appending literals *after* the last pinned one in a file.
> - **Fatal:** inserting or deleting any literal *before* an existing pinned
>   one — every later ordinal shifts and the test fails.
>
> This is why Task 4 threads `extable.FT710Profile()` into
> `exinventory_stale_test.go` rather than `extable.Lookup("ft710")`: a
> function call is an identifier, but `"ft710"` would be a **new string
> literal ahead of `"table2.csv"`**, shifting all fourteen ordinals.

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/extable/profile.go` | **New.** `Profile`, `ObservationPolicy`, per-profile and cross-profile validation, the registry, `Lookup`, `FT710Profile`, `RegisteredProfiles`. |
| `internal/extable/profile_test.go` | **New.** Validation tests; the two disagreeing test fixtures used by later tasks. |
| `internal/extable/extable.go` | The three APIs take a `Profile`; all bounds read from it; `maxObservedWidth` deleted. |
| `internal/extable/extable_test.go` | Existing tests take the FT-710 profile; new disagreeing-fixture tests. |
| `internal/extable/gen/main.go` | `-profile` replaces the three path flags. |
| `internal/extable/observe/main.go` | One `ParseCSV` call site threads the profile. Its own `textWidth` constant **stays**. |
| `internal/extable/observe/main_test.go` | Two `ParseCSV` call sites. |
| `core/cat/exinventory.go` | The `go:generate` directive. |
| `core/cat/exinventory_stale_test.go` | Three call sites gain a profile argument. Nothing else. |
| `core/cat/exdigits_ceiling_test.go` | **New.** Pins `extable.MaxDigitsCeiling == maxEXDigits`. |

---

### Task 1: `Profile`, the registry, and the disagreeing fixtures

Creates the type and registry with full validation. Nothing consumes it yet,
so this task changes no behaviour and cannot break the generated file.

**Files:**
- Create: `internal/extable/profile.go`
- Create: `internal/extable/profile_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `Profile` (struct, fields below); `ObservationPolicy` with
  `ObservationsRequired`/`ObservationsAbsent`; `NamedProfile{Name string;
  Profile Profile}`; `MaxDigitsCeiling = 247`; `func (Profile) Validate()
  error`; `func Lookup(name string) (Profile, bool)`; `func FT710Profile()
  Profile`; `func RegisteredProfiles() []NamedProfile`. Test-only:
  `fixtureRequired`, `fixtureAbsent`, `withRows(Profile, int) Profile`.

- [ ] **Step 1: Write the failing validation tests**

Create `internal/extable/profile_test.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

package extable

import (
	"strings"
	"testing"
)

// fixtureRequired is a second-model profile that disagrees with the FT-710
// in EVERY parameterised dimension: package, type qualifier, import path,
// variable name, digit bounds, text width, observation ceiling, expected
// row count and doc prose. A fixture that merely differs proves little —
// the M9b lesson is that only a fixture built to DISAGREE ever found a
// defect. It is deliberately NOT registered, so no staleness consumer goes
// looking for a generated file that does not exist.
var fixtureRequired = Profile{
	Model:       "FIXTURE",
	Package:     "ftdx10",
	TypeQual:    "cat.",
	ImportPath:  "github.com/gm5dna/open-rig-programmer/core/cat",
	VarName:     "exItems",
	OutFile:     "exinventory_gen.go",
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
		{"escaping ManualCSV", func(p *Profile) { p.ManualCSV = "../table2.csv" }},
		{"unclean ManualCSV", func(p *Profile) { p.ManualCSV = "./table2.csv" }},
		{"blank ManualCSV", func(p *Profile) { p.ManualCSV = "" }},
		{"ImportPath without TypeQual", func(p *Profile) { p.TypeQual = ""; p.ImportPath = "x/y" }},
		{"TypeQual without ImportPath", func(p *Profile) { p.TypeQual = "cat."; p.ImportPath = "" }},
		{"TypeQual missing dot", func(p *Profile) { p.TypeQual = "cat"; p.ImportPath = "x/y" }},
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
	if p.Package != "cat" || p.VarName != "exItemsGen" || p.TypeQual != "" || p.ImportPath != "" {
		t.Errorf("identity drifted: %+v", p)
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/extable/ -run 'Profile|Registry|FT710' -v`
Expected: FAIL — the package does not compile, `undefined: Profile`.

- [ ] **Step 3: Write `profile.go`**

Create `internal/extable/profile.go`:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

package extable

import (
	"fmt"
	"go/token"
	"path"
	"sort"
	"strings"
)

// MaxDigitsCeiling is the largest width any profile may declare. It mirrors
// core/cat's maxEXDigits, which refuses a wider P4 because the answer frame
// would exceed DefaultMaxFrame. Refusing here means a bad profile fails at
// registry construction rather than two packages downstream inside
// NewDialect's V8 rule. core/cat/exdigits_ceiling_test.go pins the two equal.
const MaxDigitsCeiling = 247

// ObservationPolicy declares whether a model has hardware READ observations.
// Its zero value is deliberately NOT a valid regime, so a profile that omits
// it is refused rather than defaulting to one — the M9c-1 ruling on
// ShiftDirection and ToneSemantics, for the same reason.
type ObservationPolicy int

const (
	// ObservationsRequired: the observation CSV must cover the inventory
	// exactly, in both directions.
	ObservationsRequired ObservationPolicy = iota + 1
	// ObservationsAbsent: no hardware exists for this model, so the
	// observation map must be EMPTY — not partial.
	ObservationsAbsent
)

func (o ObservationPolicy) String() string {
	switch o {
	case ObservationsRequired:
		return "ObservationsRequired"
	case ObservationsAbsent:
		return "ObservationsAbsent"
	default:
		return fmt.Sprintf("ObservationPolicy(%d)", int(o))
	}
}

// Profile is every fact about one radio model that the transcoder needs and
// that differs between models. It is the single source those facts have: the
// generator and every staleness test read the same value, so they cannot
// drift apart. That matters because a bound consulted from one place with
// its datum taken from another is the defect shape that appeared four times
// across M9b.
type Profile struct {
	// Model is the human name, used in error text only.
	Model string
	// Package is the generated file's package clause.
	Package string
	// TypeQual qualifies EXItem/EXAddress in the generated file — "" when
	// emitting into core/cat itself, "cat." from anywhere else. Set it and
	// ImportPath together or not at all.
	TypeQual string
	// ImportPath is imported by the generated file. Set iff TypeQual is.
	ImportPath string
	// VarName is the generated slice variable.
	VarName string
	// OutFile, ManualCSV and ObservedCSV are relative to the profile's own
	// package directory, which is the working directory for both
	// `go:generate` and that package's staleness test.
	OutFile     string
	ManualCSV   string
	ObservedCSV string // must be empty iff Observations is ObservationsAbsent

	// MinDigits and MaxDigits bound a non-text row's Digits column.
	MinDigits int
	MaxDigits int
	// TextWidth is the exact Digits a text row must carry. It is a
	// MANUAL-SCHEMA fact and must never be used as an evidence bound: see
	// MaxObservedWidth.
	TextWidth int
	// MaxObservedWidth bounds a hardware observation's P4 width. It is
	// deliberately independent of MinDigits/MaxDigits/TextWidth, which are
	// manual-schema facts. The two categories can disagree — this repository
	// holds the proof in core/cat/table2-corrections.csv, where TONE FREQ
	// declares two digits and answered three — so deriving one from the
	// other would be the MTPolicy.PadByte conflation in a new disguise.
	MaxObservedWidth int
	// ExpectedRows is how many rows a complete inventory has. Without it,
	// deleting the same address from both CSVs renders happily: RenderGo
	// compares the two supplied sets against each other only.
	ExpectedRows int

	Observations ObservationPolicy
	// DocLines is the generated file's descriptive prose, one line per
	// entry, WITHOUT the leading "// ". The SPDX and "Code generated by"
	// lines are composed by the renderer and are not carried here.
	DocLines []string
}

// Validate reports whether p is internally consistent. Every rule exists
// because an omitted or wrong field would otherwise become a plausible wrong
// answer rather than a refusal.
func (p Profile) Validate() error {
	if strings.TrimSpace(p.Model) == "" {
		return fmt.Errorf("extable: profile has a blank Model")
	}
	for _, f := range []struct{ name, val string }{
		{"Package", p.Package},
		{"VarName", p.VarName},
	} {
		if !isGoIdent(f.val) {
			return fmt.Errorf("extable: profile %s: %s %q is not a valid non-keyword Go identifier", p.Model, f.name, f.val)
		}
	}
	for _, f := range []struct{ name, val string }{
		{"OutFile", p.OutFile},
		{"ManualCSV", p.ManualCSV},
	} {
		if err := checkRelPath(f.name, f.val); err != nil {
			return fmt.Errorf("extable: profile %s: %w", p.Model, err)
		}
	}
	switch {
	case p.TypeQual == "" && p.ImportPath != "":
		return fmt.Errorf("extable: profile %s: ImportPath is set but TypeQual is not", p.Model)
	case p.TypeQual != "" && p.ImportPath == "":
		return fmt.Errorf("extable: profile %s: TypeQual is set but ImportPath is not", p.Model)
	case p.TypeQual != "":
		if !strings.HasSuffix(p.TypeQual, ".") || !isGoIdent(strings.TrimSuffix(p.TypeQual, ".")) {
			return fmt.Errorf("extable: profile %s: TypeQual %q must be a Go identifier followed by a dot", p.Model, p.TypeQual)
		}
	}
	for _, f := range []struct {
		name string
		val  int
	}{
		{"MinDigits", p.MinDigits},
		{"MaxDigits", p.MaxDigits},
		{"TextWidth", p.TextWidth},
		{"MaxObservedWidth", p.MaxObservedWidth},
		{"ExpectedRows", p.ExpectedRows},
	} {
		if f.val <= 0 {
			return fmt.Errorf("extable: profile %s: %s must be positive, got %d", p.Model, f.name, f.val)
		}
	}
	if p.MinDigits > p.MaxDigits {
		return fmt.Errorf("extable: profile %s: MinDigits %d exceeds MaxDigits %d", p.Model, p.MinDigits, p.MaxDigits)
	}
	for _, f := range []struct {
		name string
		val  int
	}{
		{"MaxDigits", p.MaxDigits},
		{"TextWidth", p.TextWidth},
		{"MaxObservedWidth", p.MaxObservedWidth},
	} {
		if f.val > MaxDigitsCeiling {
			return fmt.Errorf("extable: profile %s: %s %d exceeds the %d-byte ceiling core/cat enforces", p.Model, f.name, f.val, MaxDigitsCeiling)
		}
	}
	switch p.Observations {
	case ObservationsRequired:
		if err := checkRelPath("ObservedCSV", p.ObservedCSV); err != nil {
			return fmt.Errorf("extable: profile %s: %w (ObservationsRequired)", p.Model, err)
		}
	case ObservationsAbsent:
		if p.ObservedCSV != "" {
			return fmt.Errorf("extable: profile %s: ObservedCSV %q is set under ObservationsAbsent", p.Model, p.ObservedCSV)
		}
	default:
		return fmt.Errorf("extable: profile %s: ObservationPolicy %v must be set explicitly", p.Model, p.Observations)
	}
	if len(p.DocLines) == 0 {
		return fmt.Errorf("extable: profile %s: DocLines is empty", p.Model)
	}
	for i, l := range p.DocLines {
		if strings.TrimSpace(l) == "" {
			return fmt.Errorf("extable: profile %s: DocLines[%d] is blank", p.Model, i)
		}
		if strings.ContainsAny(l, "\n\r") {
			return fmt.Errorf("extable: profile %s: DocLines[%d] contains a line break", p.Model, i)
		}
	}
	return nil
}

// clone returns a copy whose DocLines cannot be mutated into the registry.
func (p Profile) clone() Profile {
	c := p
	c.DocLines = append([]string(nil), p.DocLines...)
	return c
}

func isGoIdent(s string) bool {
	return s != "" && token.IsIdentifier(s) && !token.IsKeyword(s)
}

func checkRelPath(name, v string) error {
	if v == "" {
		return fmt.Errorf("%s is blank", name)
	}
	if path.IsAbs(v) || v != path.Clean(v) || strings.HasPrefix(v, "../") {
		return fmt.Errorf("%s %q must be a clean relative path", name, v)
	}
	return nil
}

// NamedProfile pairs a registry lookup name with its profile.
type NamedProfile struct {
	Name    string
	Profile Profile
}

// ft710Profile carries every literal internal/extable held for the FT-710
// before M9c-2. TestFT710Profile_MatchesTodaysConstants pins the values, and
// core/cat's staleness test pins the bytes they produce.
var ft710Profile = Profile{
	Model:       "FT-710",
	Package:     "cat",
	VarName:     "exItemsGen",
	OutFile:     "exinventory_gen.go",
	ManualCSV:   "table2.csv",
	ObservedCSV: "table2-observed.csv",

	MinDigits:        1,
	MaxDigits:        4,
	TextWidth:        12,
	MaxObservedWidth: 12,
	ExpectedRows:     296,

	Observations: ObservationsRequired,
	DocLines: []string{
		"exItemsGen is the EX address inventory, sorted by (P1,P2,P3), built from",
		"TWO sources of different provenance: the manual transcription in",
		`table2.csv (the FT-710 CAT manual's Table 2 "MENU Chart"), and the M8c`,
		"hardware READ observations in table2-observed.csv (what one radio",
		"answered — see that file's header for the scope). Regenerate with",
		"`go generate ./core/cat`; do not edit by hand.",
	},
}

// registry maps a lookup name to its profile. It is validated at init, so an
// inconsistent profile panics the build tooling rather than emitting a wrong
// inventory.
var registry = mustRegistry(map[string]Profile{
	"ft710": ft710Profile,
})

func mustRegistry(m map[string]Profile) map[string]Profile {
	if err := validateRegistry(m); err != nil {
		panic(err)
	}
	return m
}

// validateRegistry checks each profile and the invariants that only exist
// because profiles share namespaces: two profiles writing the same file in
// the same package would have the second `go generate` silently overwrite
// the first's artefact.
func validateRegistry(m map[string]Profile) error {
	if len(m) == 0 {
		return fmt.Errorf("extable: the profile registry is empty")
	}
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)

	outFiles := map[string]string{}
	varNames := map[string]string{}
	for _, n := range names {
		p := m[n]
		if err := p.Validate(); err != nil {
			return fmt.Errorf("extable: registry entry %q: %w", n, err)
		}
		outKey := p.Package + "/" + p.OutFile
		if prev, dup := outFiles[outKey]; dup {
			return fmt.Errorf("extable: registry entries %q and %q both write %s", prev, n, outKey)
		}
		outFiles[outKey] = n

		varKey := p.Package + "." + p.VarName
		if prev, dup := varNames[varKey]; dup {
			return fmt.Errorf("extable: registry entries %q and %q both declare %s", prev, n, varKey)
		}
		varNames[varKey] = n
	}
	return nil
}

// Lookup returns a copy of the named profile.
func Lookup(name string) (Profile, bool) {
	p, ok := registry[name]
	if !ok {
		return Profile{}, false
	}
	return p.clone(), true
}

// FT710Profile returns the FT-710's profile.
//
// It exists as a named accessor, rather than callers writing
// Lookup("ft710"), for one specific reason: core/cat's staleness test has
// all fourteen of its string literals pinned by ordinal in
// core/cat/testdata/evidence-literals.golden, and introducing the literal
// "ft710" ahead of "table2.csv" would shift every one of them.
func FT710Profile() Profile { return ft710Profile.clone() }

// RegisteredProfiles returns every registration, sorted by name, as copies.
func RegisteredProfiles() []NamedProfile {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]NamedProfile, 0, len(names))
	for _, n := range names {
		out = append(out, NamedProfile{Name: n, Profile: registry[n].clone()})
	}
	return out
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/extable/ -run 'Profile|Registry|FT710' -v`
Expected: PASS, every subtest named.

- [ ] **Step 5: Verify nothing else moved**

```bash
gofmt -l .
go build ./... && go vet ./...
git diff --exit-code -- core/cat/exinventory_gen.go
```
Expected: no output from any of them.

- [ ] **Step 6: Commit**

```bash
git add internal/extable/profile.go internal/extable/profile_test.go
git commit -m "M9c-2 task 1: per-model Profile, registry and disagreeing fixtures"
```

---

### Task 2: `ParseCSV` reads its bounds from the profile

**Files:**
- Modify: `internal/extable/extable.go:68-146` (`ParseCSV`, `parseRecord`)
- Modify: `internal/extable/extable_test.go:119,135`
- Modify: `internal/extable/gen/main.go:40`
- Modify: `internal/extable/observe/main.go:143`
- Modify: `internal/extable/observe/main_test.go:40,155`
- Modify: `core/cat/exinventory_stale_test.go:26`

**Interfaces:**
- Consumes: `Profile`, `FT710Profile()`, `fixtureRequired`, `withRows` (Task 1).
- Produces: `func ParseCSV(p Profile, data []byte) ([]Row, error)`.

- [ ] **Step 1: Write the failing disagreement tests**

Append to `internal/extable/extable_test.go`:

```go
// TestParseCSV_BoundsComeFromProfile proves each digit bound is READ from
// the profile rather than hardcoded, by asserting rows that are legal under
// one profile and illegal under the other — in both directions. A test that
// only checked one direction would pass with the bounds still constant.
func TestParseCSV_BoundsComeFromProfile(t *testing.T) {
	const (
		text12  = "04,01,01,DISPLAY SETTING,DISPLAY,MY CALL,Up to 12 characters,12,true,879\n"
		text8   = "04,01,01,DISPLAY SETTING,DISPLAY,MY CALL,Up to 8 characters,8,true,879\n"
		digits1 = "01,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,1,false,646\n"
		digits6 = "01,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,6,false,646\n"
	)
	ft := withRows(FT710Profile(), 1)
	fx := withRows(fixtureRequired, 1)

	cases := []struct {
		name    string
		profile Profile
		csv     string
		wantErr bool
	}{
		{"text width 12 under FT-710", ft, text12, false},
		{"text width 12 under fixture", fx, text12, true},
		{"text width 8 under fixture", fx, text8, false},
		{"text width 8 under FT-710", ft, text8, true},
		{"digits 1 under FT-710", ft, digits1, false},
		{"digits 1 under fixture", fx, digits1, true},
		{"digits 6 under fixture", fx, digits6, false},
		{"digits 6 under FT-710", ft, digits6, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCSV(tc.profile, []byte(tc.csv))
			if tc.wantErr && err == nil {
				t.Error("ParseCSV accepted a row its profile forbids; want an error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ParseCSV rejected a row its profile permits: %v", err)
			}
		})
	}
}

// TestParseCSV_AddressComponentRange closes a gate that used to exist only
// by accident. ParseCSV never range-checked P1/P2/P3; the observation CSV's
// exactly-two-digits rule rejected the matching row instead. Under
// ObservationsAbsent there is no observation CSV, so a component of 100
// would render into an EXAddress whose Wire() is seven digits.
func TestParseCSV_AddressComponentRange(t *testing.T) {
	cases := []struct {
		name    string
		csv     string
		wantErr bool
	}{
		{"99 accepted", "99,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,false,646\n", false},
		{"P1 100 rejected", "100,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,false,646\n", true},
		{"P2 100 rejected", "01,100,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,false,646\n", true},
		{"P3 100 rejected", "01,01,100,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,false,646\n", true},
		{"negative P1 rejected", "-1,01,01,RADIO SETTING,MODE SSB,AF TREBLE GAIN,x,3,false,646\n", true},
	}
	p := withRows(FT710Profile(), 1)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCSV(p, []byte(tc.csv))
			if tc.wantErr && err == nil {
				t.Error("ParseCSV accepted an out-of-range address component; want an error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ParseCSV rejected a valid address: %v", err)
			}
		})
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/extable/ -run TestParseCSV_ -v`
Expected: FAIL to compile — `too many arguments in call to ParseCSV`.

- [ ] **Step 3: Change `ParseCSV` and `parseRecord`**

In `internal/extable/extable.go`, change the two signatures and the bound
reads. `ParseCSV`'s first line becomes:

```go
func ParseCSV(p Profile, data []byte) ([]Row, error) {
```

and its call to `parseRecord` becomes `parseRecord(p, rec)`. Then:

```go
func parseRecord(p Profile, rec []string) (Row, error) {
```

Immediately after the three `strconv.Atoi` calls that set `row.P1`,
`row.P2` and `row.P3`, insert:

```go
	for i, v := range []int{row.P1, row.P2, row.P3} {
		if v < 0 || v > 99 {
			return Row{}, fmt.Errorf("address component P%d must be 0..99, got %d", i+1, v)
		}
	}
```

and replace the Digits/Text consistency block at the end with:

```go
	// Digits/Text consistency: a text item carries exactly this radio's text
	// width; every other item is a numeric field within its digit bounds.
	if row.Text {
		if row.Digits != p.TextWidth {
			return Row{}, fmt.Errorf("text row (%s) must have digits %d, got %d", row.Name, p.TextWidth, row.Digits)
		}
	} else if row.Digits < p.MinDigits || row.Digits > p.MaxDigits {
		return Row{}, fmt.Errorf("non-text row (%s) digits must be %d..%d, got %d", row.Name, p.MinDigits, p.MaxDigits, row.Digits)
	}
	return row, nil
}
```

Update `ParseCSV`'s doc comment: replace "a non-text row whose Digits is not
1..4, or a text row whose Digits is not 12" with "a non-text row whose Digits
falls outside the profile's MinDigits..MaxDigits, a text row whose Digits is
not the profile's TextWidth, an address component outside 0..99".

- [ ] **Step 4: Update the six other call sites**

`internal/extable/extable_test.go:119` → `ParseCSV(withRows(FT710Profile(), 1), []byte(tc.csv))`.

> The existing `TestParseCSV_Strictness` has a "duplicate triple" case whose
> CSV has two rows; use `withRows(FT710Profile(), 2)` for that one, or set
> `ExpectedRows` per case. `ExpectedRows` is not enforced by `ParseCSV` until
> Task 5, so either is fine now — but write it correctly now so Task 5 needs
> no revisit.

`internal/extable/extable_test.go:135` → `ParseCSV(withRows(FT710Profile(), 1), []byte(csv))`.

`internal/extable/gen/main.go:40` → `extable.ParseCSV(p, data)` (the `p`
variable arrives in Task 6; for now use `extable.FT710Profile()`).

`internal/extable/observe/main.go:143` → `extable.ParseCSV(extable.FT710Profile(), manualCSV)`.

`internal/extable/observe/main_test.go:40` and `:155` → same substitution.

`core/cat/exinventory_stale_test.go:26` → `extable.ParseCSV(extable.FT710Profile(), csv)`.

**Do not** introduce any string literal into `exinventory_stale_test.go` —
see the boxed warning. `extable.FT710Profile()` is an identifier and a call,
so the fourteen pinned ordinals are unaffected.

- [ ] **Step 5: Run the tests**

```bash
go test ./internal/extable/... ./core/cat/
```
Expected: PASS, including `TestEXInventoryGenerated_NotStale` and
`TestEvidenceLiterals_OrderedRecordsSurvive`.

- [ ] **Step 6: Re-run the call-site gate**

```bash
grep -rn "ParseCSV(" --include=*.go . | grep -v "func ParseCSV"
gofmt -l .
go build ./... && go vet ./...
git diff --exit-code -- core/cat/exinventory_gen.go core/cat/testdata/evidence-literals.golden
```
Expected: every `ParseCSV(` hit passes a profile as its first argument; no
other output.

- [ ] **Step 7: Commit**

```bash
git add -A internal/extable core/cat/exinventory_stale_test.go
git commit -m "M9c-2 task 2: ParseCSV takes a Profile; address components gain a 0..99 gate"
```

---

### Task 3: `ParseObservedCSV` reads its ceiling from the profile

**Files:**
- Modify: `internal/extable/extable.go:148-213` (delete `maxObservedWidth`, change `ParseObservedCSV`)
- Modify: `internal/extable/extable_test.go:216,254`
- Modify: `internal/extable/gen/main.go:50`
- Modify: `core/cat/exinventory_stale_test.go:34`

**Interfaces:**
- Consumes: `Profile` (Task 1).
- Produces: `func ParseObservedCSV(p Profile, data []byte) (map[string]Observed, error)`.

- [ ] **Step 1: Write the failing test**

Append to `internal/extable/extable_test.go`:

```go
// TestParseObservedCSV_CeilingComesFromProfile is the test that kills
// revision 1's derived ceiling. The fixture's MaxDigits is 6 and its
// TextWidth is 8, so a ceiling still computed as max(MaxDigits, TextWidth)
// would be 8 and would wrongly REJECT a width of 9. The rejection case
// alone passes under either implementation and proves nothing — the pair is
// the point.
func TestParseObservedCSV_CeilingComesFromProfile(t *testing.T) {
	cases := []struct {
		name    string
		profile Profile
		csv     string
		wantErr bool
	}{
		{"width 9 under the fixture's ceiling of 9", fixtureRequired, "01,01,01,9,numeric\n", false},
		{"width 10 above the fixture's ceiling of 9", fixtureRequired, "01,01,01,10,numeric\n", true},
		{"width 10 under the FT-710's ceiling of 12", FT710Profile(), "01,01,01,10,numeric\n", false},
		{"width 13 above the FT-710's ceiling of 12", FT710Profile(), "01,01,01,13,numeric\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseObservedCSV(tc.profile, []byte(tc.csv))
			if tc.wantErr && err == nil {
				t.Error("ParseObservedCSV accepted a width above its profile's ceiling; want an error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ParseObservedCSV rejected a width its profile permits: %v", err)
			}
		})
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/extable/ -run TestParseObservedCSV_Ceiling -v`
Expected: FAIL to compile — `too many arguments`.

- [ ] **Step 3: Change `ParseObservedCSV` and delete the constant**

Delete these four lines from `internal/extable/extable.go`:

```go
// maxObservedWidth is the largest P4 width any EX item can have (the
// 12-byte Text items). A wider observation means a malformed artefact.
const maxObservedWidth = 12
```

Change the signature to:

```go
func ParseObservedCSV(p Profile, data []byte) (map[string]Observed, error) {
```

and the width check to:

```go
		width, err := strconv.Atoi(rec[3])
		if err != nil || width < 1 || width > p.MaxObservedWidth {
			return nil, fmt.Errorf("extable: observation row %d (%s): observed_read_width must be an integer in 1..%d", i+1, addr, p.MaxObservedWidth)
		}
```

In the doc comment, replace "each width an integer in 1..maxObservedWidth"
with "each width an integer in 1..the profile's MaxObservedWidth", and add:
"That bound is hardware-evidence policy and is deliberately independent of
the manual-schema widths in MinDigits/MaxDigits/TextWidth — the two
categories can disagree, as table2-corrections.csv records."

- [ ] **Step 4: Update the three other call sites**

- `internal/extable/extable_test.go:216` → `ParseObservedCSV(FT710Profile(), []byte("# provenance comment\n"+observedBody))`
- `internal/extable/extable_test.go:254` → `ParseObservedCSV(FT710Profile(), []byte(tc.csv))`
- `internal/extable/gen/main.go:50` → `extable.ParseObservedCSV(extable.FT710Profile(), observedData)`
- `core/cat/exinventory_stale_test.go:34` → `extable.ParseObservedCSV(extable.FT710Profile(), observedCSV)`

The existing case named `"width above the 12-byte text maximum"` in
`TestParseObservedCSV_Strictness` still passes under the FT-710 profile
(13 > 12). Leave its name and data alone.

- [ ] **Step 5: Run the tests**

```bash
go test ./internal/extable/... ./core/cat/
```
Expected: PASS.

- [ ] **Step 6: Re-run the call-site gate**

```bash
grep -rn "ParseObservedCSV(" --include=*.go . | grep -v "func ParseObservedCSV"
grep -rn "maxObservedWidth" --include=*.go .
gofmt -l .
go build ./... && go vet ./...
git diff --exit-code -- core/cat/exinventory_gen.go core/cat/testdata/evidence-literals.golden
```
Expected: every call passes a profile; `maxObservedWidth` has no hits at all;
no other output.

- [ ] **Step 7: Commit**

```bash
git add -A internal/extable core/cat/exinventory_stale_test.go
git commit -m "M9c-2 task 3: ParseObservedCSV takes a Profile; the observation ceiling becomes independent evidence policy"
```

---

### Task 4: `RenderGo` emits the profile's identity

The byte-identity task. The FT-710's generated file must not move.

**Files:**
- Modify: `internal/extable/extable.go:223-283` (`RenderGo`)
- Modify: `internal/extable/extable_test.go:168,172,270,279,293`
- Modify: `internal/extable/gen/main.go:55`
- Modify: `core/cat/exinventory_stale_test.go:38`

**Interfaces:**
- Consumes: `Profile` (Task 1).
- Produces: `func RenderGo(p Profile, rows []Row, observed map[string]Observed) ([]byte, error)`.

- [ ] **Step 1: Write the failing emission test**

Append to `internal/extable/extable_test.go`:

```go
// TestRenderGo_IdentityComesFromProfile proves the generated file's package
// clause, import, variable name and type qualification are all read from the
// profile. The FT-710 must emit none of the qualified forms; the fixture
// must emit all of them.
func TestRenderGo_IdentityComesFromProfile(t *testing.T) {
	rows := []Row{
		{P1: 1, P2: 1, P3: 1, P1Label: "RADIO SETTING", P2Label: "MODE SSB", Name: "AF TREBLE GAIN", P4: "-20 - +10", Digits: 3, Text: false, ManualLine: 646},
	}
	observed := map[string]Observed{"010101": {ReadWidth: 3, ReadShape: "signed"}}

	t.Run("fixture emits qualified identity", func(t *testing.T) {
		out, err := RenderGo(withRows(fixtureRequired, 1), rows, observed)
		if err != nil {
			t.Fatalf("RenderGo: %v", err)
		}
		for _, want := range []string{
			"package ftdx10",
			`import "github.com/gm5dna/open-rig-programmer/core/cat"`,
			"var exItems = []cat.EXItem{",
			"{Addr: cat.EXAddress{",
			"// exItems is the fixture inventory.",
			"// Code generated by internal/extable/gen from fixture.csv and",
			"// fixture-observed.csv. DO NOT EDIT.",
		} {
			if !strings.Contains(string(out), want) {
				t.Errorf("fixture output missing %q", want)
			}
		}
	})

	t.Run("FT-710 emits unqualified identity", func(t *testing.T) {
		out, err := RenderGo(withRows(FT710Profile(), 1), rows, observed)
		if err != nil {
			t.Fatalf("RenderGo: %v", err)
		}
		s := string(out)
		for _, want := range []string{"package cat", "var exItemsGen = []EXItem{", "{Addr: EXAddress{"} {
			if !strings.Contains(s, want) {
				t.Errorf("FT-710 output missing %q", want)
			}
		}
		for _, unwanted := range []string{"cat.EXItem", "cat.EXAddress", "import "} {
			if strings.Contains(s, unwanted) {
				t.Errorf("FT-710 output must not contain %q", unwanted)
			}
		}
	})

	t.Run("manual-only profile names one source", func(t *testing.T) {
		out, err := RenderGo(withRows(fixtureAbsent, 1), rows, nil)
		if err != nil {
			t.Fatalf("RenderGo: %v", err)
		}
		s := string(out)
		if !strings.Contains(s, "// Code generated by internal/extable/gen from fixture.csv. DO NOT EDIT.") {
			t.Error("manual-only output should name exactly one source CSV")
		}
		if strings.Contains(s, "fixture-observed.csv") {
			t.Error("manual-only output must not name an observation CSV it has none of")
		}
		if !strings.Contains(s, `ObservedReadWidth: 0`) || !strings.Contains(s, `ObservedReadShape: ""`) {
			t.Error("manual-only rows must render the documented absence sentinels")
		}
	})
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/extable/ -run TestRenderGo_Identity -v`
Expected: FAIL to compile.

- [ ] **Step 3: Rewrite `RenderGo`'s header and entry emission**

Change the signature to:

```go
func RenderGo(p Profile, rows []Row, observed map[string]Observed) ([]byte, error) {
```

Replace the fixed header block (the eleven `buf.WriteString` calls from
`"// SPDX-License-Identifier..."` through `"var exItemsGen = []EXItem{\n"`)
with:

```go
	var buf bytes.Buffer
	buf.WriteString("// SPDX-License-Identifier: GPL-3.0-or-later\n\n")
	// The generated-by marker names the profile's own sources. It is split
	// across two physical lines for a two-source profile because that is how
	// the FT-710's committed file has always been written; no physical line
	// matches Go's ^// Code generated .* DO NOT EDIT\.$ convention, and it
	// never has. Byte identity of the committed artefact is the acceptance
	// bar for M9c-2, so the non-conformance is preserved deliberately rather
	// than fixed here.
	if p.ObservedCSV != "" {
		fmt.Fprintf(&buf, "// Code generated by internal/extable/gen from %s and\n// %s. DO NOT EDIT.\n\n", p.ManualCSV, p.ObservedCSV)
	} else {
		fmt.Fprintf(&buf, "// Code generated by internal/extable/gen from %s. DO NOT EDIT.\n\n", p.ManualCSV)
	}
	fmt.Fprintf(&buf, "package %s\n\n", p.Package)
	if p.ImportPath != "" {
		fmt.Fprintf(&buf, "import %s\n\n", strconv.Quote(p.ImportPath))
	}
	for _, l := range p.DocLines {
		fmt.Fprintf(&buf, "// %s\n", l)
	}
	fmt.Fprintf(&buf, "var %s = []%sEXItem{\n", p.VarName, p.TypeQual)
```

and the per-row `fmt.Fprintf` with:

```go
		fmt.Fprintf(&buf,
			"\t{Addr: %sEXAddress{P1: %d, P2: %d, P3: %d}, P1Label: %s, P2Label: %s, Name: %s, Digits: %d, Text: %t, ObservedReadWidth: %d, ObservedReadShape: %s}, // manual line %d\n",
			p.TypeQual, r.P1, r.P2, r.P3,
			strconv.Quote(r.P1Label), strconv.Quote(r.P2Label), strconv.Quote(r.Name),
			r.Digits, r.Text, obs.ReadWidth, strconv.Quote(obs.ReadShape), r.ManualLine)
```

Leave the observation lookup exactly as it is for now — Task 5 changes it.

- [ ] **Step 4: Update the six other call sites**

- `extable_test.go:168,172` → `RenderGo(withRows(FT710Profile(), 3), rows, observed)`
- `extable_test.go:270,279` → `RenderGo(withRows(FT710Profile(), 1), rows, ...)`
- `extable_test.go:293` → `RenderGo(withRows(FT710Profile(), 1), rows, observed)`
- `gen/main.go:55` → `extable.RenderGo(extable.FT710Profile(), rows, observed)`
- `core/cat/exinventory_stale_test.go:38` → `extable.RenderGo(extable.FT710Profile(), rows, observed)`

- [ ] **Step 5: Run the tests — byte identity is the point**

```bash
go test ./internal/extable/... ./core/cat/
go generate ./core/cat && git diff --exit-code -- core/cat/exinventory_gen.go
```
Expected: PASS, and the regenerated file is byte-identical. If `git diff`
shows anything at all, the profile's `DocLines` or the header composition is
wrong — fix the profile, never the generated file.

- [ ] **Step 6: Re-run the call-site gate**

```bash
grep -rn "RenderGo(" --include=*.go . | grep -v "func RenderGo"
gofmt -l .
go build ./... && go vet ./...
git diff --exit-code -- core/cat/testdata/evidence-literals.golden
```

- [ ] **Step 7: Commit**

```bash
git add -A internal/extable core/cat/exinventory_stale_test.go
git commit -m "M9c-2 task 4: RenderGo emits the profile's package, import, variable and type qualification"
```

---

### Task 5: Observation regimes and the completeness gate

**Files:**
- Modify: `internal/extable/extable.go` (`RenderGo`'s preamble and row loop)
- Modify: `internal/extable/extable_test.go`

**Interfaces:**
- Consumes: `ObservationPolicy`, `ExpectedRows` (Task 1); `RenderGo` (Task 4).
- Produces: no signature change.

- [ ] **Step 1: Write the failing tests**

Append to `internal/extable/extable_test.go`:

```go
// TestRenderGo_ObservationRegimes pins both regimes, including the states
// revision 1 of the spec wrongly claimed were impossible.
func TestRenderGo_ObservationRegimes(t *testing.T) {
	row := Row{P1: 1, P2: 1, P3: 1, P1Label: "RADIO SETTING", P2Label: "MODE SSB", Name: "AF TREBLE GAIN", P4: "-20 - +10", Digits: 3, Text: false, ManualLine: 646}
	obs := map[string]Observed{"010101": {ReadWidth: 3, ReadShape: "signed"}}

	t.Run("absent regime refuses a non-empty map", func(t *testing.T) {
		if _, err := RenderGo(withRows(fixtureAbsent, 1), []Row{row}, obs); err == nil {
			t.Error("RenderGo accepted observations under ObservationsAbsent; want an error")
		}
	})
	t.Run("absent regime accepts an empty map", func(t *testing.T) {
		if _, err := RenderGo(withRows(fixtureAbsent, 1), []Row{row}, nil); err != nil {
			t.Errorf("RenderGo refused a valid manual-only render: %v", err)
		}
	})
	t.Run("jointly truncated sources are refused", func(t *testing.T) {
		// Both sides consistently one row short. Nothing in the set-equality
		// check can see this: the two supplied sets agree with each other.
		if _, err := RenderGo(withRows(FT710Profile(), 2), []Row{row}, obs); err == nil {
			t.Error("RenderGo accepted an inventory one row short of ExpectedRows; want an error")
		}
	})
	t.Run("both sources empty are refused", func(t *testing.T) {
		if _, err := RenderGo(FT710Profile(), nil, map[string]Observed{}); err == nil {
			t.Error("RenderGo accepted two empty sources; want an error")
		}
		if _, err := RenderGo(fixtureAbsent, nil, nil); err == nil {
			t.Error("RenderGo accepted an empty manual-only inventory; want an error")
		}
	})
	t.Run("exact row count is accepted", func(t *testing.T) {
		if _, err := RenderGo(withRows(FT710Profile(), 1), []Row{row}, obs); err != nil {
			t.Errorf("RenderGo refused a complete inventory: %v", err)
		}
	})
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/extable/ -run TestRenderGo_ObservationRegimes -v`
Expected: FAIL — the absent-regime and truncation subtests report that
`RenderGo` accepted what it should refuse.

- [ ] **Step 3: Add the gates to `RenderGo`**

Replace the existing coverage check at the top of `RenderGo`:

```go
	if len(observed) != len(rows) {
		return nil, fmt.Errorf("extable: %d observations for %d inventory rows — the observation CSV must cover the inventory exactly", len(observed), len(rows))
	}
```

with:

```go
	// Completeness first. Neither regime below can detect a JOINTLY
	// truncated pair of sources: RenderGo compares the two supplied sets
	// against each other, so deleting the same address from both — or
	// emptying both — would otherwise render happily.
	if len(rows) != p.ExpectedRows {
		return nil, fmt.Errorf("extable: profile %s: parsed %d inventory rows, want exactly %d — a source is incomplete", p.Model, len(rows), p.ExpectedRows)
	}
	switch p.Observations {
	case ObservationsRequired:
		if len(observed) != len(rows) {
			return nil, fmt.Errorf("extable: profile %s: %d observations for %d inventory rows — the observation CSV must cover the inventory exactly", p.Model, len(observed), len(rows))
		}
	case ObservationsAbsent:
		if len(observed) != 0 {
			return nil, fmt.Errorf("extable: profile %s declares no hardware observations, but %d were supplied", p.Model, len(observed))
		}
	default:
		return nil, fmt.Errorf("extable: profile %s: ObservationPolicy %v must be set explicitly", p.Model, p.Observations)
	}
```

and in the row loop replace:

```go
		obs, ok := observed[addr]
		if !ok {
			return nil, fmt.Errorf("extable: no hardware observation for address %s", addr)
		}
```

with:

```go
		var obs Observed
		if p.Observations == ObservationsRequired {
			var ok bool
			if obs, ok = observed[addr]; !ok {
				return nil, fmt.Errorf("extable: no hardware observation for address %s", addr)
			}
		}
```

Under `ObservationsAbsent` the zero `Observed` renders `ObservedReadWidth:
0` and `ObservedReadShape: ""`, which `core/cat/exinventory.go:58-76`
already documents as "no observation". Extend `RenderGo`'s doc comment to
state both regimes and the `ExpectedRows` gate.

- [ ] **Step 4: Run the tests**

```bash
go test ./internal/extable/... ./core/cat/
go generate ./core/cat && git diff --exit-code -- core/cat/exinventory_gen.go
```
Expected: PASS and no diff.

- [ ] **Step 5: Verify**

```bash
gofmt -l .
go build ./... && go vet ./...
git diff --exit-code -- core/cat/testdata/evidence-literals.golden
```

- [ ] **Step 6: Commit**

```bash
git add -A internal/extable
git commit -m "M9c-2 task 5: declared observation regimes and an ExpectedRows completeness gate"
```

---

### Task 6: `gen -profile`, the ceiling pin, and the full gate

**Files:**
- Modify: `internal/extable/gen/main.go`
- Modify: `core/cat/exinventory.go:5`
- Create: `core/cat/exdigits_ceiling_test.go`

**Interfaces:**
- Consumes: `Lookup`, `MaxDigitsCeiling`, all three parameterised APIs.
- Produces: nothing further.

- [ ] **Step 1: Write the failing ceiling pin**

Create `core/cat/exdigits_ceiling_test.go`. It is a **new file**, so it has
no records in the evidence golden and is safe to add:

```go
// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// TestExtableCeilingMatchesDialectBound pins internal/extable's profile
// ceiling to core/cat's own maxEXDigits. The two are declared separately —
// a build-time tool must not import the runtime package it generates into —
// so without this pin they could drift, and a profile would be accepted at
// registry construction only to be refused later by NewDialect's V8 rule.
func TestExtableCeilingMatchesDialectBound(t *testing.T) {
	if extable.MaxDigitsCeiling != maxEXDigits {
		t.Errorf("extable.MaxDigitsCeiling = %d, core/cat maxEXDigits = %d — they must agree",
			extable.MaxDigitsCeiling, maxEXDigits)
	}
}
```

- [ ] **Step 2: Run to verify it compiles and passes**

Run: `go test ./core/cat/ -run TestExtableCeilingMatchesDialectBound -v`
Expected: PASS (both are 247). If it fails, one of the two is wrong — fix
`MaxDigitsCeiling`, never `maxEXDigits`.

> **No import cycle exists** — verified 28/07/2026: `go list -deps
> ./internal/extable` returns only the package itself among this module's
> packages, and `core/cat` already imports `internal/extable` from
> `exinventory_stale_test.go`. Note this new file is `package cat`
> (internal), not `package cat_test`, because `maxEXDigits` is unexported.
> The evidence walker walks every `*_test.go` in the directory regardless of
> its package clause, but a new file has no golden records, so it is safe.

- [ ] **Step 3: Rewrite `gen/main.go` to take `-profile`**

Replace the flag block and body of `main` with:

```go
	profileName := flag.String("profile", "", "name of the registered extable profile to generate (e.g. ft710)")
	flag.Parse()

	if *profileName == "" {
		log.Fatal("-profile is required; see internal/extable/profile.go for the registered names")
	}
	p, ok := extable.Lookup(*profileName)
	if !ok {
		var names []string
		for _, np := range extable.RegisteredProfiles() {
			names = append(names, np.Name)
		}
		log.Fatalf("unknown profile %q; registered: %v", *profileName, names)
	}

	data, err := os.ReadFile(p.ManualCSV)
	if err != nil {
		log.Fatalf("reading %s: %v", p.ManualCSV, err)
	}
	rows, err := extable.ParseCSV(p, data)
	if err != nil {
		log.Fatalf("parsing %s: %v", p.ManualCSV, err)
	}

	// A manual-only profile has no observation source; RenderGo requires the
	// map to be empty for it, and a nil map is empty.
	var observed map[string]extable.Observed
	if p.ObservedCSV != "" {
		observedData, err := os.ReadFile(p.ObservedCSV)
		if err != nil {
			log.Fatalf("reading %s: %v", p.ObservedCSV, err)
		}
		if observed, err = extable.ParseObservedCSV(p, observedData); err != nil {
			log.Fatalf("parsing %s: %v", p.ObservedCSV, err)
		}
	}

	out, err := extable.RenderGo(p, rows, observed)
	if err != nil {
		log.Fatalf("rendering Go: %v", err)
	}
	if err := os.WriteFile(p.OutFile, out, 0o644); err != nil {
		log.Fatalf("writing %s: %v", p.OutFile, err)
	}
```

Update the command's doc comment: the paths now come from the named profile
rather than from flags, and the working directory is the profile's own
package directory.

- [ ] **Step 4: Update the `go:generate` directive**

In `core/cat/exinventory.go:5`, replace the directive with:

```go
//go:generate go run github.com/gm5dna/open-rig-programmer/internal/extable/gen -profile ft710
```

- [ ] **Step 5: Regenerate and prove byte identity**

```bash
go generate ./core/cat
git diff --exit-code -- core/cat/exinventory_gen.go
```
Expected: no diff. This is the milestone's acceptance bar.

- [ ] **Step 6: Run the full local gate**

```bash
gofmt -l .
go build ./...
go vet ./...
go test ./...
git diff --exit-code -- core/cat/exinventory_gen.go core/cat/testdata/evidence-literals.golden
grep -rn "ParseCSV(\|ParseObservedCSV(\|RenderGo(" --include=*.go . | grep -v "^./internal/extable/extable.go"
```
Expected: `gofmt` silent; build, vet and the full suite green; no diff; every
call site passing a profile.

- [ ] **Step 7: Commit**

```bash
git add -A internal/extable core/cat
git commit -m "M9c-2 task 6: gen takes -profile; pin the extable ceiling to core/cat's maxEXDigits"
```

---

## Self-review against the spec

Checked after writing; issues fixed inline.

**Spec coverage.** Every "In" scope item maps to a task: `Profile` and
registry → Task 1; all three APIs taking it → Tasks 2-4; package/type/import
emission → Task 4; independent observation ceiling → Tasks 1 and 3; declared
regime → Task 5; `ExpectedRows` → Task 5; address-component range → Task 2;
enumeration API → Task 1; disagreeing fixture → Task 1, exercised in 2-5;
`maxEXDigits` relation → Tasks 1 and 6.

Two spec items correctly produce **no** task, by design: `observe`'s
`textWidth` stays (Task 2 threads its `ParseCSV` call but leaves the
constant), and the FT-710 staleness test is not made table-driven.

**Type consistency.** `ParseCSV(p, data)`, `ParseObservedCSV(p, data)` and
`RenderGo(p, rows, observed)` all take the profile first and are used that
way in every later task. `withRows` is defined in Task 1 and used from Task 2
on. `fixtureRequired`/`fixtureAbsent` are defined once.

**Known ordering constraint.** Task 2 leaves `gen/main.go` calling
`extable.FT710Profile()` directly; Task 6 replaces that with the
`-profile`-resolved value. This is deliberate — it keeps every intermediate
commit compiling and green.
