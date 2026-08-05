// SPDX-License-Identifier: GPL-3.0-or-later

package extable

import (
	"fmt"
	"go/token"
	"path"
	"path/filepath"
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

// TypeRefPolicy declares how the generated file refers to EXItem and
// EXAddress. Its zero value is deliberately NOT a valid policy: emitting
// into core/cat and emitting elsewhere are both legitimate, so an OMITTED
// policy must refuse rather than silently choosing one — a profile for a
// foreign package that forgot its import would otherwise validate and then
// emit a file that does not compile (Codex plan review, finding 3; the
// M9c-1 omitted-semantics ruling again).
type TypeRefPolicy int

const (
	// TypesLocal: the file is emitted into core/cat itself; EXItem and
	// EXAddress are unqualified and nothing is imported. ImportPath and
	// ImportAlias must both be empty.
	TypesLocal TypeRefPolicy = iota + 1
	// TypesImported: the file is emitted into another package; ImportPath
	// is imported under the explicit alias ImportAlias, and every type
	// reference is qualified by that alias. Deriving the qualifier FROM the
	// alias makes qualifier/import drift structurally impossible — they are
	// one string, not two.
	TypesImported
)

func (t TypeRefPolicy) String() string {
	switch t {
	case TypesLocal:
		return "TypesLocal"
	case TypesImported:
		return "TypesImported"
	default:
		return fmt.Sprintf("TypeRefPolicy(%d)", int(t))
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
	// Types declares how the generated file refers to EXItem/EXAddress —
	// TypesLocal inside core/cat, TypesImported anywhere else. Zero is
	// refused.
	Types TypeRefPolicy
	// ImportPath is imported by the generated file under ImportAlias.
	// Both are set iff Types is TypesImported; the alias is also the type
	// qualifier, so the two cannot drift.
	ImportPath  string
	ImportAlias string
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
	// The generator reads the CSVs and then writes OutFile unconditionally,
	// so an OutFile naming a source would DESTROY that source on the next
	// go generate (Codex plan review, finding 2).
	//
	// The comparison is case-INSENSITIVE because the filesystems this
	// repository is developed and built on resolve case-aliased names to one
	// file: APFS is case-insensitive by default on macOS, as is NTFS on
	// Windows. Under a byte-equal compare, "TABLE2.CSV" alongside
	// "table2.csv" would validate happily and the generator would then
	// overwrite the committed source. Folding costs nothing on a
	// case-sensitive filesystem — it only refuses a spelling no profile has
	// any reason to use — and the refuse-early posture is worth more than
	// that theoretical freedom.
	if strings.EqualFold(p.OutFile, p.ManualCSV) {
		return fmt.Errorf("extable: profile %s: OutFile %q collides with its ManualCSV %q (compared case-insensitively) — generating would overwrite the source", p.Model, p.OutFile, p.ManualCSV)
	}
	if p.ObservedCSV != "" && strings.EqualFold(p.OutFile, p.ObservedCSV) {
		return fmt.Errorf("extable: profile %s: OutFile %q collides with its ObservedCSV %q (compared case-insensitively) — generating would overwrite the source", p.Model, p.OutFile, p.ObservedCSV)
	}
	switch p.Types {
	case TypesLocal:
		if p.ImportPath != "" || p.ImportAlias != "" {
			return fmt.Errorf("extable: profile %s: ImportPath/ImportAlias are set under TypesLocal", p.Model)
		}
	case TypesImported:
		if p.ImportPath == "" {
			return fmt.Errorf("extable: profile %s: TypesImported requires an ImportPath", p.Model)
		}
		if !isGoIdent(p.ImportAlias) {
			return fmt.Errorf("extable: profile %s: ImportAlias %q must be a valid non-keyword Go identifier", p.Model, p.ImportAlias)
		}
	default:
		return fmt.Errorf("extable: profile %s: TypeRefPolicy %v must be set explicitly", p.Model, p.Types)
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

// checkRelPath requires a clean, strictly local relative path.
// filepath.IsLocal does the heavy lifting — it rejects "", "..", parent
// traversal, platform-native absolute paths and Windows reserved names —
// and the two explicit checks close what it leaves: "." (the directory
// itself) and unclean spellings like "./x".
func checkRelPath(name, v string) error {
	if v == "." || !filepath.IsLocal(v) || v != path.Clean(v) {
		return fmt.Errorf("%s %q must be a clean local relative path", name, v)
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
	Types:       TypesLocal,
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

// ftdx10Profile carries the FTdx10's Table 2 transcription facts. It is the
// registry's first TypesImported entry: the inventory is emitted into
// core/cat/ftdx10, a model package OUTSIDE core/cat, so EXItem and EXAddress
// are qualified by the explicit "cat" alias rather than being local names.
//
// Evidence, all from CAT manual rev 2308-F's Table 2 "MENU Chart" (see
// core/cat/ftdx10/table2.csv's own provenance header, which records the
// chart's header-vs-chart anomaly): the chart's Digits column runs 1..4 for
// every numeric row, and its ONE text row — MY CALL. at (04,01,01) — is 12.
// MinDigits/MaxDigits/TextWidth are therefore chart-verified for THIS radio,
// not inherited from the FT-710's identical-looking values.
//
// Deliberately NOT given a named accessor of its own (compare FT710Profile):
// its only consumers reach it through Lookup/RegisteredProfiles, which is
// what lets core/cat/ftdx10's staleness test select its registration by
// Package rather than by hardcoding a lookup name.
var ftdx10Profile = Profile{
	Model:       "FTdx10",
	Package:     "ftdx10",
	Types:       TypesImported,
	ImportPath:  "github.com/gm5dna/open-rig-programmer/core/cat",
	ImportAlias: "cat",
	VarName:     "exItems",
	OutFile:     "exinventory_gen.go",
	ManualCSV:   "table2.csv",

	MinDigits: 1,
	MaxDigits: 4,
	TextWidth: 12,
	// MaxObservedWidth is an INERT API-REQUIRED SENTINEL here. Validate
	// demands a positive value from every profile, but this profile declares
	// ObservationsAbsent — no FTdx10 hardware exists to this project, so no
	// observation CSV is ever parsed and this bound is never consulted. It
	// carries NO hardware claim about the FTdx10, and must not be read as
	// one: the moment observations do exist, it is re-derived from them
	// rather than kept. It is spelt 12 only because a sentinel has to be
	// spelt something.
	MaxObservedWidth: 12,
	// ExpectedRows comes from the group-boundary ledger
	// (core/cat/ftdx10/testdata/group-ledger.csv), which was derived from
	// the rendered PDF before any transcription existed — NOT from
	// transcription A and B agreeing with each other. The distinction is the
	// point: this gate freezes an externally established count, it does not
	// create one. If A and B agree on a number that is not this one, the
	// answer is arbitration against the PDF, never an edit here.
	ExpectedRows: 197,

	Observations: ObservationsAbsent,
	DocLines: []string{
		"exItems is the FTdx10's EX address inventory, sorted by (P1,P2,P3), built",
		`from ONE source: the manual transcription in table2.csv (the FTdx10 CAT`,
		`manual rev 2308-F's Table 2 "MENU Chart"). Unlike the FT-710's inventory`,
		"there are no hardware READ observations to join — no FTdx10 has ever been",
		"asked anything — so every item carries the absence sentinels",
		"ObservedReadWidth 0 and ObservedReadShape \"\". Regenerate with",
		"`go generate ./core/cat/ftdx10`; do not edit by hand.",
	},
}

// ftdx101Profile carries the FTdx101D/MP's Table 2 transcription facts —
// ONE profile for BOTH models, because the spec-reviewed applicability
// sweep (core/cat/ftdx101/testdata/group-ledger.md) attests that no
// stored property is model-conditional; only P4 VALUE ranges differ, and
// P4 semantics are not stored. Same TypesImported shape as ftdx10.
var ftdx101Profile = Profile{
	Model:       "FTdx101D/MP",
	Package:     "ftdx101",
	Types:       TypesImported,
	ImportPath:  "github.com/gm5dna/open-rig-programmer/core/cat",
	ImportAlias: "cat",
	VarName:     "exItems",
	OutFile:     "exinventory_gen.go",
	ManualCSV:   "table2.csv",

	MinDigits: 1,
	MaxDigits: 4,
	TextWidth: 12,
	// MaxObservedWidth is an INERT API-REQUIRED SENTINEL here, exactly as
	// on the ftdx10 profile: ObservationsAbsent means no observation CSV
	// is ever parsed and this bound is never consulted. No hardware claim.
	MaxObservedWidth: 12,
	// ExpectedRows comes from the group-boundary ledger
	// (core/cat/ftdx101/testdata/group-ledger.csv), derived from the
	// rendered PDF before any transcription existed. If A and B agree on
	// a number that is not this one, the answer is arbitration against
	// the PDF, never an edit here.
	ExpectedRows: 193,

	Observations: ObservationsAbsent,
	DocLines: []string{
		"exItems is the FTdx101D/MP's EX address inventory, sorted by (P1,P2,P3),",
		"built from ONE source: the manual transcription in table2.csv (the",
		`FTDX101MP/FTDX101D CAT manual rev 2308-L's Table 2 "MENU Chart"). It`,
		"serves BOTH models: the ledger's applicability attestation records that",
		"no stored property is model-conditional. There are no hardware READ",
		"observations to join — no FTdx101 has ever been asked anything — so",
		"every item carries the absence sentinels ObservedReadWidth 0 and",
		"ObservedReadShape \"\". Regenerate with `go generate ./core/cat/ftdx101`;",
		"do not edit by hand.",
	},
}

// registry maps a lookup name to its profile. It is validated at init, so an
// inconsistent profile panics the build tooling rather than emitting a wrong
// inventory.
var registry = mustRegistry(map[string]Profile{
	"ft710":   ft710Profile,
	"ftdx10":  ftdx10Profile,
	"ftdx101": ftdx101Profile,
})

func mustRegistry(m map[string]Profile) map[string]Profile {
	if err := validateRegistry(m); err != nil {
		panic(err)
	}
	return m
}

// validateRegistry checks each profile, its lookup name, and the invariants
// that only exist because profiles share namespaces: two profiles writing
// the same file in the same package would have the second `go generate`
// silently overwrite the first's artefact, and one profile's OUTPUT naming
// another's INPUT in the same package directory would destroy a committed
// source.
//
// Duplicate lookup NAMES need no check here: the registry is built as a map
// literal, where a duplicate key is a compile error. (A map cannot even
// carry a duplicate to detect.)
func validateRegistry(m map[string]Profile) error {
	if len(m) == 0 {
		return fmt.Errorf("extable: the profile registry is empty")
	}
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)

	// Path collision keys are LOWER-CASED, for the reason Validate's
	// intra-profile checks fold: on APFS and on NTFS a case-only difference
	// is not a different file, so a byte-equal key would let two profiles
	// that write one file, or one profile that writes another's source, pass
	// the sweep and then collide on disk.
	//
	// VarName is deliberately NOT folded. It is a Go identifier, and case is
	// significant to the compiler: two package-level variables in one package
	// differing only in case — exItems and EXItems — are legal Go that
	// compiles and links, one unexported and one exported. Folding that key
	// would refuse a pair of profiles Go itself accepts, which is a
	// correctness loss, not a safety gain. The Package component of each key
	// is left verbatim for the same reason: it is an identifier, not a path.
	type outFile struct{ entry, path string }
	outFiles := map[string]outFile{}
	varNames := map[string]string{}
	inputs := map[string]string{} // package-qualified source CSVs, folded -> entry
	for _, n := range names {
		p := m[n]
		// The name is the CLI-facing -profile token; a blank or
		// whitespace-bearing one would be unselectable or ambiguous.
		if strings.TrimSpace(n) == "" || strings.ContainsAny(n, " \t\n") {
			return fmt.Errorf("extable: registry has an entry whose lookup name %q is blank or contains whitespace", n)
		}
		if err := p.Validate(); err != nil {
			return fmt.Errorf("extable: registry entry %q: %w", n, err)
		}
		outPath := p.Package + "/" + p.OutFile
		outKey := p.Package + "/" + strings.ToLower(p.OutFile)
		if prev, dup := outFiles[outKey]; dup {
			return fmt.Errorf("extable: registry entries %q and %q both write %s (paths compared case-insensitively: %s)", prev.entry, n, outPath, prev.path)
		}
		outFiles[outKey] = outFile{entry: n, path: outPath}

		varKey := p.Package + "." + p.VarName
		if prev, dup := varNames[varKey]; dup {
			return fmt.Errorf("extable: registry entries %q and %q both declare %s", prev, n, varKey)
		}
		varNames[varKey] = n

		inputs[p.Package+"/"+strings.ToLower(p.ManualCSV)] = n
		if p.ObservedCSV != "" {
			inputs[p.Package+"/"+strings.ToLower(p.ObservedCSV)] = n
		}
	}
	// Cross-profile output-vs-input collisions, both registration orders.
	for outKey, out := range outFiles {
		if owner, hit := inputs[outKey]; hit {
			return fmt.Errorf("extable: registry entry %q writes %s, which is entry %q's source CSV (paths compared case-insensitively)", out.entry, out.path, owner)
		}
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
