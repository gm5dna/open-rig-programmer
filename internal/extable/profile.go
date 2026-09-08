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

// MaxDigitsCeiling is CORE/CAT'S width ceiling, and so the value every
// profile that renders into core/cat carries in its own DigitsCeiling field.
// It mirrors core/cat's maxEXDigits, which refuses a wider P4 because the
// answer frame would exceed DefaultMaxFrame. Declaring it here means a bad
// profile fails at registry construction rather than two packages downstream
// inside NewDialect's V8 rule, and core/cat/exdigits_ceiling_test.go pins the
// two equal.
//
// It is NOT the bound Validate consults. That is Profile.DigitsCeiling, for
// the reason recorded there: a profile rendering into a different package has
// a different frame budget, and bounding it by this one would be a bound
// consulted from one place with its datum taken from another.
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

// AddressForm, Labels, TextRows and ParameterlessRows are the four
// CHART-SHAPE policies. Each zero value is refused by Profile.Validate for
// the reason ObservationPolicy and TypeRefPolicy are: an omitted semantic
// must refuse, never default.
//
// THEY ARE extable's OWN TYPES, not core/cat's. This package imports no
// core/cat — it is build-time tooling that RENDERS core/cat source text, and
// an import would make the transcoder depend on the package it generates
// into. THREE OF THE FOUR address forms correspond ONE-FOR-ONE with
// core/cat's — AddressTriple with EXAddressTriple, AddressPair with
// EXAddressPair, AddressSingle with EXAddressSingle — and each correspondence
// is a fact about the radios' charts rather than a type relationship: a Pair
// profile's CSV carries P3 == 0 on every row and a Single profile's carries
// P2 AND P3 == 0, which is exactly what core/cat's rule V12 requires of a
// Pair and a Single dialect's inventory. AddressGrouped has NO core/cat
// counterpart and is not owed one: no Yaesu chart in this repository prints
// its shape, and the absence of a counterpart is not a gap to be filled by
// inventing one. This comment used to say
// AddressSingle had no core/cat counterpart and rendered through a separate
// Kenwood package; cat.EXAddressSingle is the seam the FT-991A milestone
// added, and the generated ft991a inventory is what consumes it (Codex
// third seat, LOW C-L2).
type AddressForm int

const (
	// AddressTriple: the chart's MENU Number is a (P1,P2,P3) triple and the
	// radio's EX address field is six digits.
	AddressTriple AddressForm = iota + 1
	// AddressPair: the field is four digits, so every row's p3 column must
	// be 0. ParseCSV refuses any other value rather than dropping it: a
	// component that reaches no frame must not reach the inventory either.
	AddressPair
	// AddressSingle: the chart prints ONE menu number and that number is the
	// whole address, so every row's p2 AND p3 columns must be 0. ParseCSV
	// refuses any other value rather than dropping it, for the reason
	// AddressPair refuses a non-zero p3 — this form simply carries the rule
	// one component further down.
	//
	// P1's WIDTH AND DOMAIN ARE A PER-FORM FACT, owned by the arms that
	// implement the form and not by this constant: parseRecord's 0..999
	// domain check (TestParseCSV_AddressSingleP1DomainIs0To999 pins it, and
	// TestParseCSV_TheOtherFormsKeepTheTwoDigitDomain pins that the other
	// two forms keep 0..99), ParseObservedCSV's exactly-three-digits column
	// check, and RenderGo's "%03d" observation key. The last two are the two
	// sides of one join and agree only while both render the same width, so
	// they widen together; widening one side alone makes every observation
	// miss, on a complete CSV, silently.
	//
	// The domain WAS 0..99, a package-wide cap that sat above the form
	// switch, and the FT-991A milestone widened it: that radio's chart is
	// "P1 : 001 - 153" over 153 contiguous rows, so 54 of them were
	// untranscribable and the generator would have failed on row 100. The
	// Kenwood charts that first carried this form stop at 099 and are
	// unaffected — 0..99 is a subset of 0..999 — but their observation KEY
	// moved from "08" to "008" with the rest, which is the half a "wider
	// domain is a superset" argument does not cover.
	AddressSingle
	// AddressGrouped: the chart prints a (P1,P2,P3) triple whose components
	// are ONE, TWO and TWO digits — five wire characters. All three are on
	// the wire, so unlike AddressPair and AddressSingle this form refuses no
	// component; TestParseCSV_AddressGroupedCarriesAllThreeComponents is the
	// discriminating pin, because an arm that copied its neighbours' "must
	// be 0" rules would refuse every row of a real chart.
	//
	// P1's DOMAIN IS 0..1, AND IT IS THE ONE BOUND IN THIS TYPE THAT IS NOT
	// A FIELD'S CAPACITY. A one-digit field carries 0..9; the manuals
	// enumerate the values instead — "P1 (Menu type number) 0: Menu 1:
	// Advanced Menu" (docs/fixtures-private/manuals/
	// ts890s_pc_rev1_layout.txt:1897-1900, ts990s_pc_rev2_layout.txt:1720-1723
	// — the parenthetical is the 890S's wording; the 990S prints a bare
	// "P1" over the same two-value list) — so the domain is the
	// enumeration and a P1 of 2 names a group no chart prints.
	// TestParseCSV_AddressGroupedP1DomainIs0To1 pins it, including the 9 a
	// capacity argument would have admitted. P2 and P3 keep the two-digit
	// domain and its SHIPPED refusal sentence, because their fields really
	// are two digits wide.
	//
	// The observation key is "%d%02d%02d" — five characters, one token per
	// component, on both sides of the join
	// (TestRenderGo_GroupedProfileKeysObservationsByFiveDigitForm).
	//
	// REUSING AddressTriple WAS REJECTED. Its components' domains are 0..99
	// each and its wire width is six. Narrowing P1 to 0..1 for some profiles
	// and not others would make the domain a PER-PROFILE width datum — this
	// member with a worse name and a shared enum constant — and the
	// observation key's width would move for the Yaesu rows that use the
	// form, silently, which is exactly the failure AddressSingle's comment
	// above records from the FT-991A widening: widening one side alone makes
	// every observation miss, on a complete CSV, with nothing to see.
	AddressGrouped
)

func (a AddressForm) String() string {
	switch a {
	case AddressTriple:
		return "AddressTriple"
	case AddressPair:
		return "AddressPair"
	case AddressSingle:
		return "AddressSingle"
	case AddressGrouped:
		return "AddressGrouped"
	default:
		return fmt.Sprintf("AddressForm(%d)", int(a))
	}
}

// Labels declares whether the model's chart prints GROUP LABELS — the P1 and
// P2 column headings the FT-710's Table 2 carries and the FT-891's does not.
type Labels int

const (
	// LabelsRequired: both label columns carry text, and a blank one is a
	// transcription error.
	LabelsRequired Labels = iota + 1
	// LabelsAbsent: the chart prints no group labels, so both columns must
	// be BLANK and the rendered EXItem carries "" for each. Inventing a
	// label would put words in a manual's mouth; carrying a whitespace-only
	// cell through would give a consumer a space where it must see an
	// absence, which is why RenderGo normalises rather than copies.
	LabelsAbsent
)

func (l Labels) String() string {
	switch l {
	case LabelsRequired:
		return "LabelsRequired"
	case LabelsAbsent:
		return "LabelsAbsent"
	default:
		return fmt.Sprintf("Labels(%d)", int(l))
	}
}

// TextRows declares whether the model's chart has free-text rows: the
// FT-710's six (MY CALL and five PRESET NAMEs), the FTdx10's and FTdx101's
// one each, and — on the evidence read so far — the FT-891's none.
//
// It makes the transcriber's "a text row is a STOP" convention MECHANICAL.
// Under TextRowsAbsent a row with text=true is refused by ParseCSV rather
// than being carried into an inventory whose radio's chart never printed it.
type TextRows int

const (
	// TextRowsAllowed: text rows exist and each carries one of the widths
	// TextWidths names.
	TextRowsAllowed TextRows = iota + 1
	// TextRowsAbsent: the chart has none, TextWidths must be empty, and any
	// row flagged text is refused.
	TextRowsAbsent
)

func (t TextRows) String() string {
	switch t {
	case TextRowsAllowed:
		return "TextRowsAllowed"
	case TextRowsAbsent:
		return "TextRowsAbsent"
	default:
		return fmt.Sprintf("TextRows(%d)", int(t))
	}
}

// ParameterlessRows declares whether the model's chart prints a row with NO
// PARAMETER: a menu number that occupies a line of the chart but names no
// settable field, so its parameter column and its Digits cell are drawn as
// hyphens. The FT-991A's chart has exactly one — 087 RADIO ID, whose
// parameter column is ten hyphens and whose Digits cell is a single hyphen —
// and the four charts registered before it have none.
//
// IT IS INDEPENDENT OF AddressForm. A four-digit chart may print a
// parameterless row and a single-number chart may have none; the two
// policies answer different questions about a chart and neither constrains
// the other. TestParseCSV_ParameterlessIsIndependentOfAddressForm pins the
// independence on a Pair-form fixture.
//
// The excluded row is TRANSCRIBED (its printed cells reach Row verbatim) and
// COUNTED (ExpectedRows is the chart's own row count), and it is omitted from
// the generated inventory, because an EX item with no field is not an address
// anything may read or write.
//
// ParameterlessExcluded × ObservationsRequired, stated because the policy is
// radio-independent and the combination is reachable: RenderGo's
// ObservationsRequired arm compares len(observed) against len(rows), and
// under Excluded the inventory it renders is len(rows) −
// len(ParameterlessAddresses) items, so a complete observation sweep of the
// REGISTRABLE addresses is one short of that comparison. No registered
// profile meets it: every profile in the registry today declares
// ParameterlessRefused, so the only Excluded profiles that exist are this
// package's own test fixtures, and the FT-991A's stanza — the first Excluded
// one the registry will hold — is planned as ObservationsAbsent, which does
// not reach this arm either. (This sentence said "the only Excluded profile
// this repository has declares ObservationsAbsent", which described a
// registered profile that does not yet exist — Stage 0 close review, seat 1
// LOW-3.) So the arm is left exactly as it was rather than changed against a
// case nothing exercises. The first profile that does meet it owns the
// change, and this comment is the record that the arithmetic was known and
// deferred, not missed.
type ParameterlessRows int

const (
	// ParameterlessRefused: the chart prints no such row, so a hyphen in a
	// Digits cell is a transcription error and ParseCSV refuses it naming
	// the row. ParameterlessAddresses must be empty.
	ParameterlessRefused ParameterlessRows = iota + 1
	// ParameterlessExcluded: the chart prints one or more, each named in
	// ParameterlessAddresses. A hyphen is admitted on THOSE addresses alone,
	// and a numeric width on one of them is refused — the licence is per
	// address, never per policy.
	ParameterlessExcluded
)

func (p ParameterlessRows) String() string {
	switch p {
	case ParameterlessRefused:
		return "ParameterlessRefused"
	case ParameterlessExcluded:
		return "ParameterlessExcluded"
	default:
		return fmt.Sprintf("ParameterlessRows(%d)", int(p))
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

	// Addresses, LabelPolicy, TextRowPolicy and ParameterlessPolicy are the
	// chart-shape policies. Each has no default; see AddressForm, Labels,
	// TextRows and ParameterlessRows.
	Addresses           AddressForm
	LabelPolicy         Labels
	TextRowPolicy       TextRows
	ParameterlessPolicy ParameterlessRows

	// ParameterlessAddresses names every (P1,P2,P3) the chart prints with no
	// parameter. It is a SET OF ADDRESSES and there is deliberately no
	// count beside it: a count is a bound consulted from somewhere other
	// than its datum, and a count-only gate would accept an inventory that
	// omitted the WRONG row and still satisfied the arithmetic. RenderGo
	// therefore excludes BY ADDRESS and then checks the count that follows.
	//
	// Non-empty iff ParameterlessPolicy is ParameterlessExcluded, and its
	// members must be distinct: a duplicate would make
	// ExpectedRows − len(ParameterlessAddresses) understate the inventory.
	// TestProfileValidate_Parameterless pins both rules.
	ParameterlessAddresses [][3]int

	// DigitsCeiling is the largest width THIS profile's family admits, and
	// it is what bounds MaxDigits, every TextWidths entry and
	// MaxObservedWidth. It is a
	// PER-FAMILY datum because the bound is a property of the family's own
	// frame budget: the five Yaesu profiles render into core/cat and carry
	// MaxDigitsCeiling, which core/cat/exdigits_ceiling_test.go pins to that
	// package's maxEXDigits; a profile rendering into another package
	// supplies that package's own constant and pins the pair there.
	//
	// Reading MaxDigitsCeiling directly here instead would be a bound
	// consulted from one place with its datum taken from another — the
	// defect shape this type's own doc comment above says it exists to
	// prevent, and the one that appeared four times across M9b. Zero is
	// refused, as every other omitted semantic on this type is.
	// TestProfileValidate_CeilingComesFromTheProfile pins both directions.
	DigitsCeiling int
	// MinDigits and MaxDigits bound a non-text row's Digits column.
	MinDigits int
	MaxDigits int
	// TextWidths names every Digits a text row may carry. It is a
	// MANUAL-SCHEMA fact and must never be used as an evidence bound: see
	// MaxObservedWidth.
	//
	// It is a SET rather than a single width because a chart may print text
	// rows of more than one length — a screen-saver message of up to 10
	// characters beside a power-on message of up to 15 — and no single value
	// admits both. Membership is what ParseCSV tests
	// (TestParseCSV_TextRowMatchesAnyDeclaredWidth), so a chart with one
	// width declares a one-entry set and loses nothing.
	//
	// It carries at least one STRICTLY POSITIVE entry under TextRowsAllowed
	// and is EMPTY under TextRowsAbsent — the one cross-field rule the three
	// new policies carry. A model with no text rows and a text width of 12
	// is stating two incompatible things about one chart, and so is one
	// whose absence is spelt []int{0}: that declares a text row whose width
	// is nothing, which is why absence is the NIL SLICE and Validate refuses
	// the zero entry in both directions.
	TextWidths []int
	// MaxObservedWidth bounds a hardware observation's P4 width. It is
	// deliberately independent of MinDigits/MaxDigits/TextWidths, which are
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
	switch p.Addresses {
	case AddressTriple, AddressPair, AddressSingle, AddressGrouped:
	default:
		return fmt.Errorf("extable: profile %s: AddressForm %v must be set explicitly", p.Model, p.Addresses)
	}
	switch p.LabelPolicy {
	case LabelsRequired, LabelsAbsent:
	default:
		return fmt.Errorf("extable: profile %s: Labels %v must be set explicitly", p.Model, p.LabelPolicy)
	}
	// TextWidths is validated INSIDE this switch rather than in the
	// positive-integer sweep below, because its permitted shape is the
	// policy's to say: at least one strictly positive entry under Allowed,
	// empty under Absent. The check is POSITIVE-ONLY — every entry must be a
	// width a chart could print — so neither an empty set under Allowed nor
	// a zero entry under either policy can pass as "no text row".
	switch p.TextRowPolicy {
	case TextRowsAllowed:
		if len(p.TextWidths) == 0 {
			return fmt.Errorf("extable: profile %s: TextWidths is empty under TextRowsAllowed — a chart that prints a text row must say how wide", p.Model)
		}
		for _, w := range p.TextWidths {
			if w <= 0 {
				return fmt.Errorf("extable: profile %s: TextWidths entry %d is not positive — every text width is a width a chart prints", p.Model, w)
			}
		}
	case TextRowsAbsent:
		if len(p.TextWidths) != 0 {
			return fmt.Errorf("extable: profile %s: TextWidths is %v under TextRowsAbsent, want empty — a model whose chart prints no text row has no text width, and []int{0} would declare one of width nothing", p.Model, p.TextWidths)
		}
	default:
		return fmt.Errorf("extable: profile %s: TextRows %v must be set explicitly", p.Model, p.TextRowPolicy)
	}
	// The exclusion SET is validated inside this switch, as TextWidths is
	// inside the one above and for the same reason: whether it may be
	// populated is the policy's to say, in both directions. A Refused
	// profile carrying an address would declare an exclusion nothing acts
	// on; an Excluded profile carrying none would declare a regime with no
	// subject.
	switch p.ParameterlessPolicy {
	case ParameterlessExcluded:
		if len(p.ParameterlessAddresses) == 0 {
			return fmt.Errorf("extable: profile %s: ParameterlessAddresses is empty under %v — the policy names the rows by address, so a chart that prints one must say which", p.Model, p.ParameterlessPolicy)
		}
		seen := make(map[[3]int]bool, len(p.ParameterlessAddresses))
		for _, a := range p.ParameterlessAddresses {
			if seen[a] {
				return fmt.Errorf("extable: profile %s: ParameterlessAddresses lists %d/%d/%d twice — a duplicate would make ExpectedRows minus the excluded count understate the inventory", p.Model, a[0], a[1], a[2])
			}
			seen[a] = true
		}
		// ExpectedRows COUNTS the excluded rows — they are printed, so they
		// are transcribed — and the inventory is what remains, so a chart
		// consisting only of parameterless rows would generate nothing.
		if p.ExpectedRows <= len(p.ParameterlessAddresses) {
			return fmt.Errorf("extable: profile %s: ExpectedRows %d does not exceed the %d excluded address(es) — the count includes them, so the inventory would be empty", p.Model, p.ExpectedRows, len(p.ParameterlessAddresses))
		}
	case ParameterlessRefused:
		if len(p.ParameterlessAddresses) != 0 {
			return fmt.Errorf("extable: profile %s: ParameterlessAddresses names %d address(es) under %v — this chart prints no parameterless row, so there is nothing to exclude", p.Model, len(p.ParameterlessAddresses), p.ParameterlessPolicy)
		}
	default:
		return fmt.Errorf("extable: profile %s: ParameterlessRows %v must be set explicitly", p.Model, p.ParameterlessPolicy)
	}
	for _, f := range []struct {
		name string
		val  int
	}{
		// DigitsCeiling is swept here, ahead of the ceiling comparison
		// below, because a zero one would otherwise be READ as a bound and
		// refuse every width — the omitted-semantic trap the two parsers'
		// own self-validation calls exist to avoid.
		{"DigitsCeiling", p.DigitsCeiling},
		{"MinDigits", p.MinDigits},
		{"MaxDigits", p.MaxDigits},
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
		{"MaxObservedWidth", p.MaxObservedWidth},
	} {
		// The bound is the PROFILE'S, not this package's constant: see
		// DigitsCeiling's own doc comment for why, and
		// TestProfileValidate_CeilingComesFromTheProfile for the pin.
		if f.val > p.DigitsCeiling {
			return fmt.Errorf("extable: profile %s: %s %d exceeds the %d-digit ceiling this profile declares", p.Model, f.name, f.val, p.DigitsCeiling)
		}
	}
	// The ceiling sweep walks TextWidths ENTRY BY ENTRY rather than taking
	// the set's first or largest member: the ceiling bounds each width a
	// frame must carry, so a set whose second entry overflows is as
	// unrenderable as one whose first does.
	for _, w := range p.TextWidths {
		if w > p.DigitsCeiling {
			return fmt.Errorf("extable: profile %s: TextWidths entry %d exceeds the %d-digit ceiling this profile declares", p.Model, w, p.DigitsCeiling)
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

// clone returns a copy whose DocLines and ParameterlessAddresses cannot be
// mutated into the registry. Both are slices, so a bare struct copy would
// hand every caller the registry's own backing array — and for
// ParameterlessAddresses that would let one caller silently change which
// address a later generation omits.
// TestProfileValidate_ParameterlessAddressesAreCopied pins the second half.
func (p Profile) clone() Profile {
	c := p
	c.DocLines = append([]string(nil), p.DocLines...)
	c.ParameterlessAddresses = append([][3]int(nil), p.ParameterlessAddresses...)
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

	// Today's behaviour, said out loud rather than inherited: the FT-710's
	// Table 2 prints a (P1,P2,P3) MENU Number, group labels in both label
	// columns, and six 12-byte free-text rows. Every row names a field, so
	// the chart prints no parameterless row.
	Addresses:           AddressTriple,
	LabelPolicy:         LabelsRequired,
	TextRowPolicy:       TextRowsAllowed,
	ParameterlessPolicy: ParameterlessRefused,

	// DigitsCeiling is core/cat's own MaxDigitsCeiling because this profile
	// renders into core/cat: the ceiling and the frames it bounds belong to
	// the same package. The three profiles below carry it for the same
	// reason, and a profile rendering elsewhere would not.
	DigitsCeiling:    MaxDigitsCeiling,
	MinDigits:        1,
	MaxDigits:        4,
	TextWidths:       []int{12},
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
// MinDigits/MaxDigits/TextWidths are therefore chart-verified for THIS radio,
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

	// Today's behaviour, said out loud rather than inherited: the FTdx10's
	// chart prints a (P1,P2,P3) MENU Number, both group labels, and one
	// 12-byte text row (MY CALL. at 04/01/01). Every row names a field, so
	// the chart prints no parameterless row.
	Addresses:           AddressTriple,
	LabelPolicy:         LabelsRequired,
	TextRowPolicy:       TextRowsAllowed,
	ParameterlessPolicy: ParameterlessRefused,

	// core/cat's ceiling, because this profile renders into core/cat.
	DigitsCeiling: MaxDigitsCeiling,
	MinDigits:     1,
	MaxDigits:     4,
	TextWidths:    []int{12},
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

	// Today's behaviour, said out loud rather than inherited: the FTdx101's
	// chart prints a (P1,P2,P3) MENU Number, both group labels, and one
	// 12-byte text row. Every row names a field, so the chart prints no
	// parameterless row.
	Addresses:           AddressTriple,
	LabelPolicy:         LabelsRequired,
	TextRowPolicy:       TextRowsAllowed,
	ParameterlessPolicy: ParameterlessRefused,

	// core/cat's ceiling, because this profile renders into core/cat.
	DigitsCeiling: MaxDigitsCeiling,
	MinDigits:     1,
	MaxDigits:     4,
	TextWidths:    []int{12},
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

// ft891Profile carries the FT-891's menu-chart transcription facts. It is
// the registry's first entry to take the MINORITY value of all three
// chart-shape policies at once, which is what those policies were added for:
// the FT-891's EX address field is four digits, its chart prints no group
// labels, and it prints no free-text row.
//
// Evidence, all from CAT manual rev 1909-C's menu chart (see
// core/cat/ft891/table2.csv's own provenance header, which records the
// chart's printed quirks): the Digits column runs 1..5, the 5 coming from
// exactly two rows — 0803 OTHER DISP and 0804 OTHER SHIFT, at
// ft891_layout.txt:595-596, whose signed "-3000 Hz - 0 - +3000 Hz"
// parameter counts its sign as a digit. MaxDigits is therefore 5 where the
// other three profiles carry 4, and it is a reading of THIS chart rather
// than a widening of theirs.
//
// TextWidths is EMPTY, which under TextRowsAbsent is the only shape
// Validate admits: a chart with no text row has no text width, and spelling
// 12 here out of resemblance to the other three would state two incompatible
// things about one chart.
//
// Deliberately NOT given a named accessor, for the reason the ftdx10 and
// ftdx101 profiles are not: its only consumers reach it through
// Lookup/RegisteredProfiles, which is what lets core/cat/ft891's staleness
// test select its registration by Package rather than by a lookup name.
var ft891Profile = Profile{
	Model:       "FT-891",
	Package:     "ft891",
	Types:       TypesImported,
	ImportPath:  "github.com/gm5dna/open-rig-programmer/core/cat",
	ImportAlias: "cat",
	VarName:     "exItems",
	OutFile:     "exinventory_gen.go",
	ManualCSV:   "table2.csv",

	// The FT-891's chart, said out loud: a four-digit MENU Number whose
	// two halves are the whole address (every row's p3 is 0), no group
	// labels in either column, and no text row. Every row names a field, so
	// the chart prints no parameterless row.
	Addresses:           AddressPair,
	LabelPolicy:         LabelsAbsent,
	TextRowPolicy:       TextRowsAbsent,
	ParameterlessPolicy: ParameterlessRefused,

	// core/cat's ceiling, because this profile renders into core/cat.
	DigitsCeiling: MaxDigitsCeiling,
	MinDigits:     1,
	MaxDigits:     5,
	TextWidths:    nil,
	// MaxObservedWidth is an INERT API-REQUIRED SENTINEL here, exactly as
	// on the ftdx10 and ftdx101 profiles: ObservationsAbsent means no
	// observation CSV is ever parsed and this bound is never consulted. It
	// carries NO hardware claim about the FT-891 — no FT-891 has ever been
	// asked anything — and must not be read as one; the moment observations
	// do exist it is re-derived from them rather than kept. It is spelt 12
	// only because a sentinel has to be spelt something, and 12 is NOT this
	// radio's text width: this chart has no text row at all.
	MaxObservedWidth: 12,
	// ExpectedRows comes from the group-boundary ledger
	// (core/cat/ft891/testdata/), derived from the rendered PDF before any
	// transcription existed — NOT from transcriptions A and B agreeing with
	// each other. If they agree on a number that is not this one, the answer
	// is arbitration against the PDF, never an edit here.
	ExpectedRows: 159,

	Observations: ObservationsAbsent,
	DocLines: []string{
		"exItems is the FT-891's EX address inventory, sorted by (P1,P2), built",
		"from ONE source: the manual transcription in table2.csv (the FT-891 CAT",
		"manual rev 1909-C's menu chart). The FT-891's EX address is a PAIR: the",
		"chart's four-digit MENU Number is P1 and P2, every item's P3 is 0, and",
		"the chart prints no group labels, so every P1Label and P2Label is \"\".",
		"There are no hardware READ observations to join — no FT-891 has ever",
		"been asked anything — so every item carries the absence sentinels",
		"ObservedReadWidth 0 and ObservedReadShape \"\". Regenerate with",
		"`go generate ./core/cat/ft891`; do not edit by hand.",
	},
}

// ft991aProfile carries the FT-991A's menu-chart transcription facts. It is
// the registry's first AddressSingle entry and its first ParameterlessExcluded
// one, which is what those two policies were added for: this chart prints ONE
// three-digit MENU Number that is the whole EX address, and one of its 153
// rows names no settable field at all.
//
// Evidence, all from CAT manual rev 1711-D's menu chart (see
// core/cat/ft991a/table2.csv's own provenance header, which records the
// chart's printed quirks):
//
//   - The chart's grammar block prints "P1 : 001 - 153 (MENU Number)"
//     (ft991a_layout.txt:520) and the chart's own rows run 001 to 153 with no
//     gap and no repeat, so the address is P1 ALONE and every row's p2 and p3
//     are 0 — extable's AddressSingle, core/cat's EXAddressSingle.
//   - MaxDigits is 8, from ONE row: 151 PRESET FREQUENCY (ft991a_layout.txt:692),
//     "00030000 ~ 47000000". It is a reading of THIS chart, not a widening of
//     the other four profiles' 4 and 5.
//   - 087 RADIO ID (ft991a_layout.txt:623) prints ten hyphens for its
//     parameter and a single hyphen for its Digits. ParameterlessAddresses
//     names it {87,0,0} so that ParseCSV admits the hyphen THERE and refuses
//     it on the other 152 rows, and so that RenderGo omits that one address
//     from the inventory: a menu number naming no field is not an address an
//     EX frame could read or write.
//
// TextWidths is EMPTY, which under TextRowsAbsent is the only shape
// Validate admits: this chart prints no free-text row, and spelling 12 here
// out of resemblance to the FT-710 family would state two incompatible things
// about one chart.
//
// Deliberately NOT given a named accessor, for the reason the ftdx10, ftdx101
// and ft891 profiles are not: its consumers reach it through
// Lookup/RegisteredProfiles.
var ft991aProfile = Profile{
	Model:       "FT-991A",
	Package:     "ft991a",
	Types:       TypesImported,
	ImportPath:  "github.com/gm5dna/open-rig-programmer/core/cat",
	ImportAlias: "cat",
	VarName:     "exItems",
	OutFile:     "exinventory_gen.go",
	ManualCSV:   "table2.csv",

	// The FT-991A's chart, said out loud: a three-digit MENU Number that is
	// the whole address (every row's p2 AND p3 are 0), no group labels in
	// either column, no text row, and ONE row that names no field.
	Addresses:              AddressSingle,
	LabelPolicy:            LabelsAbsent,
	TextRowPolicy:          TextRowsAbsent,
	ParameterlessPolicy:    ParameterlessExcluded,
	ParameterlessAddresses: [][3]int{{87, 0, 0}},

	// core/cat's ceiling, because this profile renders into core/cat.
	DigitsCeiling: MaxDigitsCeiling,
	MinDigits:     1,
	MaxDigits:     8,
	TextWidths:    nil,
	// MaxObservedWidth is an INERT API-REQUIRED SENTINEL here, exactly as on
	// the ftdx10, ftdx101 and ft891 profiles: ObservationsAbsent means no
	// observation CSV is ever parsed and this bound is never consulted. It
	// carries NO hardware claim about the FT-991A — no FT-991A has ever been
	// asked anything — and must not be read as one; the moment observations
	// do exist it is re-derived from them rather than kept. It is spelt 12
	// only because a sentinel has to be spelt something, and 12 is NOT this
	// radio's text width: this chart has no text row at all, nor its widest
	// Digits, which is 8.
	MaxObservedWidth: 12,
	// ExpectedRows COUNTS 087, the parameterless row: the chart prints it, so
	// it is transcribed and counted, and the inventory is the 152 that remain
	// after ParameterlessAddresses excludes it BY ADDRESS. RenderGo checks
	// that arithmetic itself.
	ExpectedRows: 153,

	Observations: ObservationsAbsent,
	DocLines: []string{
		"exItems is the FT-991A's EX address inventory, sorted by (P1,P2,P3),",
		"built from ONE source: the manual transcription in table2.csv (the",
		"FT-991A CAT Operation Reference Manual rev 1711-D's menu chart). The",
		"FT-991A's EX address is a SINGLE component: the chart's three-digit",
		"MENU Number is P1, every item's P2 and P3 are 0, and the chart prints",
		"no group labels, so every P1Label and P2Label is \"\". There are no",
		"hardware READ observations to join — no FT-991A has ever been asked",
		"anything — so every item carries the absence sentinels",
		"ObservedReadWidth 0 and ObservedReadShape \"\". Regenerate with",
		"`go generate ./core/cat/ft991a`; do not edit by hand.",
	},
}

// ts590sProfile carries the TS-590S's menu-chart transcription facts. It is
// one of the three Kenwood registrations, all of which render OUTSIDE
// core/cat: the inventory is emitted into core/kw/ts590, so EXItem and
// EXAddress are qualified by the explicit "kw" alias, and — the part that
// matters — the ceiling it declares is core/kw's, not this package's
// constant.
//
// "FIRST" IS NOT SAID OF IT, and that is deliberate. The registry is a map
// (see registry below), so the only order that exists is the sort
// RegisteredProfiles applies, under which the first Kenwood entry is ts480.
// The three stanzas landed together in one milestone; none preceded the
// others.
//
// THE CEILING IS TRANSCRIBED, WHICH IS WHY IT IS TESTED TWICE. 246 is
// core/kw.MaxEXDigits (core/kw/exdigits.go): that package's DefaultMaxFrame
// of 256 less the ten fixed bytes of a Kenwood EX answer, "EX"(2) + P1(3) +
// P2(2) + P3(1) + P4(1) + ";"(1). It cannot be written here as the symbol —
// this package is build-time tooling that RENDERS core/kw source text, and
// importing core/kw would cycle the dependency that one-way rule exists to
// keep — so core/kw/exdigits_ceiling_test.go pins every profile with this
// ImportPath to it, and TestRegisteredProfiles_DeclareTodaysBehaviourExplicitly
// names it here. It is NOT MaxDigitsCeiling: that constant is 247, core/cat's
// number, derived from a Yaesu EX answer's NINE bytes of overhead, and the
// two differ by exactly one byte, which is what would make the copy-paste
// invisible.
//
// THE S AND THE SG ARE TWO CHARTS, NOT ONE. Kenwood prints "EX Command
// Parameter List (for TS-590S)" and a separate list for the SG over
// COLLIDING addresses with different meanings, so this profile and the SG's
// share package ts590 and NOTHING else: different OutFile, VarName and
// ManualCSV, which are validateRegistry's three collision keys.
//
// Evidence, all from the TS-590S/TS-590SG PC Control Command Reference Guide
// of January/30/2019 (see core/kw/ts590/menu590s.csv's own provenance header,
// which records how the widths were read from a chart that prints no Digits
// column at all):
//
//   - The chart prints ONE three-digit Menu number and that number is the
//     whole address — AddressSingle, every row's p2 and p3 columns 0 — and no
//     group labels of any kind, so LabelsAbsent.
//   - MinDigits 1 / MaxDigits 3. The width is the character count of the
//     column HEADER over a row's rightmost populated cell, since the headers
//     ARE the P5 codes: 75 rows stop at or before header 9 (width 1), four
//     reach the "10 ~" column (width 2), and the eight PF rows print their
//     own "000 ~ 255 (3-digit)" (width 3).
//   - TextRowsAllowed with TextWidths {8}: menu 087 Power on message, "Power on
//     Message (up to 8 ASCII characters)", the chart's only free-text field.
//     The PF rows' fixed-width character field is NOT one, on the FT-891's
//     version-row precedent.
//
// ExpectedRows is 88 and comes from the transcription leg's own boundary
// ledger, derived from the rendered PDF before any transcription existed —
// NOT from transcriptions A and B agreeing with each other, and not from the
// printed domain "000 ~ 087" implying 88 addresses. If they agree on a number
// that is not this one, the answer is arbitration against the PDF, never an
// edit here.
//
// Deliberately NOT given a named accessor, for the reason the ftdx10, ftdx101
// and ft891 profiles are not.
var ts590sProfile = Profile{
	Model:       "TS-590S",
	Package:     "ts590",
	Types:       TypesImported,
	ImportPath:  "github.com/gm5dna/open-rig-programmer/core/kw",
	ImportAlias: "kw",
	VarName:     "exItems590S",
	OutFile:     "exinventory590s_gen.go",
	ManualCSV:   "menu590s.csv",

	Addresses:     AddressSingle,
	LabelPolicy:   LabelsAbsent,
	TextRowPolicy: TextRowsAllowed,
	// Every row of this chart's Digits column carries a value — the chart
	// prints no parameterless row at all — so a hyphen there is a
	// transcription error and ParseCSV refuses it, exactly as it does for
	// the four Yaesu profiles above.
	ParameterlessPolicy: ParameterlessRefused,

	// core/kw's ceiling, transcribed from kw.MaxEXDigits; see above.
	DigitsCeiling: 246,
	MinDigits:     1,
	MaxDigits:     3,
	TextWidths:    []int{8},
	// MaxObservedWidth is an INERT API-REQUIRED SENTINEL here, exactly as on
	// the three ObservationsAbsent profiles above: no TS-590S has ever been
	// asked anything by this project, so no observation CSV is ever parsed
	// and this bound is never consulted. It carries NO hardware claim and
	// must not be read as one; the moment observations do exist it is
	// re-derived from them rather than kept. It is spelt 12 for the reason
	// ts480Profile gives: that is what the other absent profiles spell, and
	// a sentinel that coincided with this chart's own widest width would
	// look derived from the chart, which is the one property a sentinel must
	// not have.
	MaxObservedWidth: 12,
	ExpectedRows:     88,

	Observations: ObservationsAbsent,
	DocLines: []string{
		"exItems590S is the TS-590S's EX menu inventory, sorted by menu number,",
		"built from ONE source: the manual transcription in menu590s.csv (the",
		`TS-590S/TS-590SG PC Control Command Reference Guide's "EX Command`,
		`Parameter List (for TS-590S)"). It is NOT the TS-590SG's table: the book`,
		"prints two separate lists over colliding addresses with different",
		"meanings, and exItems590SG is the other one. The TS-590S's EX address is",
		"a SINGLE component: the chart's three-digit Menu number is P1, every",
		`item's P2 and P3 are 0, and the chart prints no group labels, so every`,
		`P1Label and P2Label is "". There are no hardware READ observations to`,
		"join — no TS-590S has ever been asked anything — so every item carries",
		`the absence sentinels ObservedReadWidth 0 and ObservedReadShape "".`,
		"Regenerate with `go generate ./core/kw/ts590`; do not edit by hand.",
	},
}

// ts590sgProfile carries the TS-590SG's menu-chart transcription facts. It is
// one of the three Kenwood registrations, which are the reason DigitsCeiling
// is a per-profile field at all: they render outside core/cat, so their
// ceiling is core/kw's MaxEXDigits and not this package's MaxDigitsCeiling.
// None of the three is "first" — the registry is a map, and under
// RegisteredProfiles' sort ts480 sorts ahead of both 590 rows.
//
// Evidence, all from the Kenwood TS-590S/TS-590SG PC Control Command
// Reference Guide Rev.3 (see core/kw/ts590/menu590sg.csv's own provenance
// header): the chart headed "EX Command Parameter List (for TS-590SG)" prints
// a THREE-DIGIT Menu (P1) number that is the whole address, no group-label
// columns, and one free-text row.
//
// THE BOOK PRINTS TWO CHARTS AND THIS IS ONE OF THEM. The TS-590S's list and
// the TS-590SG's collide on every address with different meanings — 000 is
// Firmware Version here — so the two radios get two profiles, two CSVs and
// two generated files in one package directory, which validateRegistry
// permits because OutFile, VarName and ManualCSV all differ.
//
// DigitsCeiling is core/kw's MaxEXDigits, transcribed as the literal 246
// because internal/extable is build-time tooling and importing the package it
// renders into would cycle the dependency. That is not an unchecked
// transcription: core/kw/exdigits_ceiling_test.go selects every profile whose
// ImportPath is core/kw and requires this field to equal MaxEXDigits, so a
// copy-paste of core/cat's 247 fails there. The two numbers differ by exactly
// one byte, because a Kenwood EX answer carries one more fixed byte than a
// Yaesu one, which is what would have made the mistake invisible.
//
// MaxDigits is 4, and the 4 comes from ONE row: 000 Firmware Version, "Version
// information (4 ASCII characters) read only", carried as digits=4 text=false
// on the FT-891's 18/01/00 MAIN VERSION precedent. Every other numbered row is
// 1, 2 or 3 — 3 being the thirteen PF-assignment rows whose merged cell prints
// "000 ~ 255 (3-digit)". TextWidths is {8}, the width of the chart's single
// free-text row, 001 Power on message; a text row of a width this set does not
// name is what ParseCSV refuses, and is why 000 is not one.
//
// Deliberately NOT given a named accessor, for the reason the ftdx10, ftdx101
// and ft891 profiles are not: its only consumers reach it through
// Lookup/RegisteredProfiles.
var ts590sgProfile = Profile{
	Model:       "TS-590SG",
	Package:     "ts590",
	Types:       TypesImported,
	ImportPath:  "github.com/gm5dna/open-rig-programmer/core/kw",
	ImportAlias: "kw",
	VarName:     "exItems590SG",
	OutFile:     "exinventory590sg_gen.go",
	ManualCSV:   "menu590sg.csv",

	// The TS-590SG's chart, said out loud: ONE three-digit Menu number that
	// is the whole address (every row's p2 and p3 are 0), no group labels,
	// and one free-text row.
	Addresses:     AddressSingle,
	LabelPolicy:   LabelsAbsent,
	TextRowPolicy: TextRowsAllowed,
	// Every row of this chart's Digits column carries a value — the chart
	// prints no parameterless row at all — so a hyphen there is a
	// transcription error and ParseCSV refuses it, exactly as it does for
	// the four Yaesu profiles above.
	ParameterlessPolicy: ParameterlessRefused,

	// core/kw's ceiling, because this profile renders into core/kw. The
	// value is kw.MaxEXDigits (DefaultMaxFrame 256 - 10 fixed EX-answer
	// bytes); core/kw/exdigits_ceiling_test.go pins this field to it.
	DigitsCeiling: 246,
	MinDigits:     1,
	MaxDigits:     4,
	TextWidths:    []int{8},
	// MaxObservedWidth is an INERT API-REQUIRED SENTINEL here, exactly as on
	// the three Yaesu profiles that carry one: ObservationsAbsent means no
	// observation CSV is ever parsed and this bound is never consulted. It
	// carries NO hardware claim about the TS-590SG — no TS-590SG has ever
	// been asked anything — and must not be read as one; the moment
	// observations do exist it is re-derived from them rather than kept. It
	// is spelt 12, the same sentinel those three spell, and it is NOT a
	// claim that the widest answer this radio gives is twelve bytes.
	MaxObservedWidth: 12,
	// ExpectedRows comes from the group-boundary ledger derived from the
	// rendered PDF before any transcription existed — NOT from the
	// transcriptions agreeing with each other. If they agree on a number that
	// is not this one, the answer is arbitration against the PDF, never an
	// edit here.
	ExpectedRows: 100,

	Observations: ObservationsAbsent,
	DocLines: []string{
		"exItems590SG is the TS-590SG's EX address inventory, sorted by (P1,P2),",
		"built from ONE source: the manual transcription in menu590sg.csv (the",
		"TS-590S/TS-590SG PC Control Command Reference Guide Rev.3, the chart",
		"headed \"EX Command Parameter List (for TS-590SG)\"). This radio's EX",
		"address is a SINGLE component: the chart's three-digit Menu number is",
		"P1, every item's P2 and P3 are 0, and the chart prints no group labels,",
		"so every P1Label and P2Label is \"\". It is NOT the TS-590S's table — the",
		"book prints two, over colliding addresses with different meanings.",
		"There are no hardware READ observations to join — no TS-590SG has ever",
		"been asked anything — so every item carries the absence sentinels",
		"ObservedReadWidth 0 and ObservedReadShape \"\". Regenerate with",
		"`go generate ./core/kw/ts590`; do not edit by hand.",
	},
}

// ts480Profile carries the TS-480's menu-chart transcription facts. It is the
// first registry entry that does NOT render into core/cat — first under the
// only ordering there is, RegisteredProfiles' sort, where "ts480" precedes
// "ts590s" and "ts590sg". Its inventory is emitted into core/kw/ts480, so its
// types are qualified by the "kw" alias and its width ceiling is that
// family's own, not this package's.
//
// Evidence, all from the TS-480 PC control command reference of 27/11/2003
// (see core/kw/ts480/menu480.csv's own provenance header, which records the
// chart's printed quirks): the chart prints a single three-digit Menu No. over
// the declared domain 000 ~ 060, no group columns of any kind, and a P5 GRID
// headed 0..9 and "Over" rather than a parameter-description column. Reading
// that grid, the Digits column runs 1..2 — 2 on exactly eight rows, 032, 034,
// 035 and the five PF-key rows 048-052, each of which either prints
// "(2-digit)" or carries a range its codes 0-9 cannot reach.
//
// Addresses is AddressSingle, the form this family is why extable has: the
// printed menu number IS the whole address, so ParseCSV requires p2 AND p3 to
// be 0 on every row.
//
// TextRowPolicy is TextRowsAbsent and TextWidths empty, which under that
// policy is the only shape Validate admits. The five PF-key rows are the
// near miss and are deliberately not text: their shared cell prints
// "00 ~ 99 (2-digit)", which is a numeric code range, and Text marks the
// chart's free-text row — the one a transcriber must stop at — not any
// row whose values happen to be looked up elsewhere.
//
// Deliberately NOT given a named accessor, for the reason the ftdx10, ftdx101
// and ft891 profiles are not: its only consumers reach it through
// Lookup/RegisteredProfiles, and core/kw/ts480's staleness test selects it by
// the (Package, VarName) pair rather than by a hardcoded lookup name.
var ts480Profile = Profile{
	Model:       "TS-480",
	Package:     "ts480",
	Types:       TypesImported,
	ImportPath:  "github.com/gm5dna/open-rig-programmer/core/kw",
	ImportAlias: "kw",
	VarName:     "exItems480",
	OutFile:     "exinventory_gen.go",
	ManualCSV:   "menu480.csv",

	// The TS-480's chart, said out loud: one three-digit Menu No. that is
	// the whole address (every row's p2 and p3 are 0), no group labels in
	// either column, and no free-text row.
	Addresses:     AddressSingle,
	LabelPolicy:   LabelsAbsent,
	TextRowPolicy: TextRowsAbsent,
	// Every row of this chart's Digits column carries a value — the chart
	// prints no parameterless row at all — so a hyphen there is a
	// transcription error and ParseCSV refuses it, exactly as it does for
	// the four Yaesu profiles above.
	ParameterlessPolicy: ParameterlessRefused,

	// core/kw's ceiling, NOT this package's MaxDigitsCeiling — this profile
	// renders into core/kw, so the bound and the frames it bounds belong to
	// the same package, which is the whole reason DigitsCeiling is a
	// per-profile field. The value is core/kw.MaxEXDigits, and it is
	// transcribed as a literal because internal/extable is build-time
	// tooling that renders that package's source text and may not import
	// the package it generates into. core/kw/exdigits_ceiling_test.go pins
	// every profile whose ImportPath is core/kw to the constant, so the two
	// cannot drift; the number differs from MaxDigitsCeiling (247) by
	// exactly one byte, which is what would make a copy-paste invisible.
	DigitsCeiling: 246,
	MinDigits:     1,
	MaxDigits:     2,
	TextWidths:    nil,
	// MaxObservedWidth is an INERT API-REQUIRED SENTINEL here, exactly as on
	// the ftdx10, ftdx101 and ft891 profiles: ObservationsAbsent means no
	// observation CSV is ever parsed and this bound is never consulted. It
	// carries NO hardware claim about the TS-480 — no Kenwood radio has ever
	// been asked anything by this project, which is A19's whole point — and
	// must not be read as one; the moment observations do exist it is
	// re-derived from them rather than kept. It is spelt 12 because that is
	// what the other three absent profiles spell, and 12 is six times this
	// chart's widest printed field, so it can only be read as a sentinel.
	MaxObservedWidth: 12,
	// ExpectedRows is A26's number for this radio, and it comes from the
	// boundary ledger derived from the rendered PDF before any transcription
	// existed — NOT from transcriptions A and B agreeing with each other. If
	// they agree on a number that is not this one, the answer is arbitration
	// against the PDF, never an edit here. The printed domain 000 ~ 060 is
	// arithmetically 61 addresses and the ledger says 61; A26 records that
	// where the two ever disagree, the ledger wins.
	ExpectedRows: 61,

	Observations: ObservationsAbsent,
	DocLines: []string{
		"exItems480 is the TS-480's EX menu inventory, sorted by (P1,P2,P3),",
		"built from ONE source: the manual transcription in menu480.csv (the",
		"TS-480 PC control command reference of 27/11/2003, whose EX parameter",
		"chart the book leaves untitled). The TS-480's EX address is a SINGLE",
		"component: the chart's three-digit Menu No. is P1, every item's P2 and",
		"P3 are 0, and the chart prints no group labels, so every P1Label and",
		"P2Label is \"\". There are no hardware READ observations to join — no",
		"TS-480 has ever been asked anything — so every item carries the absence",
		"sentinels ObservedReadWidth 0 and ObservedReadShape \"\". Regenerate with",
		"`go generate ./core/kw/ts480`; do not edit by hand.",
	},
}

// ts890sProfile carries the TS-890S's menu-chart transcription facts. Like
// the three older Kenwood registrations it renders OUTSIDE core/cat — the
// inventory is emitted into core/kw/ma, so EXItem and EXAddress are
// qualified by the explicit "kw" alias and the ceiling it declares is
// core/kw's, not this package's constant.
//
// IT IS THE FIRST AddressGrouped REGISTRATION. This book prints THREE
// address cells whose widths are one, two and two — "P1 (Menu type number)
// 0: Menu 1: Advanced Menu", "P2 (Category number) 00 ~ 99", "P3 (Item
// number) 00 ~ 99" (ts890s_pc_rev1_layout.txt:1897-1910) — five wire
// characters in all, which is neither the six of AddressTriple nor the four
// of AddressPair. All three components are on the wire, so unlike the Pair
// and Single forms this one refuses no component; see AddressForm's own doc
// comment, which already cites this book for the P1 enumeration that makes
// that component's domain 0..1 rather than a one-digit field's 0..9.
//
// THE CHART IS TWO LISTS UNDER ONE HEADING, NOT TWO CHARTS. "EX Command
// Parameter Lists" (1934) prints a "Menu" table (1935-2204) and an
// "Advanced Menu" table (2212-2281) whose rows differ only in P1, so both
// belong to one inventory and one CSV — which is why ExpectedRows counts
// across both and why the Advanced rows are not a second registration.
//
// EVIDENCE FOR EACH POLICY, all from that layout text and confirmed on the
// renders named above:
//
//   - LabelsAbsent. The table's heading rows are P1 | P2 | P3 | Function |
//     P5, and P5's own sub-heading row is the code list "000 001 002 003
//     004 005 006 ~" (1935-1938, 2006-2009, 2098-2101, 2181-2184,
//     2212-2215). There is no group-label column of any kind. This was read
//     off the layout text AND confirmed against renders of PDF pages 26, 27,
//     28, 29 and 30 (printed 25-29), which show the ruled table headings
//     directly.
//
//   - TextRowsAllowed with TextWidths {10, 15}. The chart prints exactly two
//     free-text fields, and their widths differ, which is the case
//     TextWidths is a SET for: 0/00/05 Screen Saver Message, "Up to 10
//     alphanumeric characters" (1946), and 0/00/06 Power-on Message, "Up to
//     15 alphanumeric characters" (1947). The EX command's own P5 legend
//     states the same two lengths from the other side — "A power-on message
//     can vary in length from 0 to 15 characters. Screen saver text can vary
//     in length from 0 to 10 characters." (1920-1921).
//
//   - MinDigits 3 / MaxDigits 4. The P5 column HEADINGS are the codes
//     themselves and every one of them is three characters wide, so a row
//     that enumerates values is 3. Nineteen rows are four wide. Two print a
//     numeric legend: 0/05/12 Contest Number, "0001 ~ 9999 (Must be a
//     4-digit number)" (2108), and 1/00/05 Reference Oscillator
//     Calibration, "Parameter value of 0000 ~ 1000, corresponding to setting
//     values of -500 ~ +500" (2235-2236). Neither is a text row: a numeric
//     legend states a width, not a string. The other seventeen are the
//     PF-key rows 0/00/15 to 0/00/31, whose own cell states no width and
//     whose width the EX command page prints instead — "PF key settings use
//     4 digits (refer to the PF Key assignment ID lists)" (1918-1919), the
//     lists themselves running to four-character IDs (2292-2342). The
//     transcription leg read those rows as 3 and recorded the disagreement;
//     ORCHESTRATOR RULING R-B settled it at 4. MaxDigits did not move.
//
//   - ParameterlessExcluded over FOUR addresses — 1/00/23 Touchscreen
//     Calibration (2273), 1/00/24 Software License Agreement (2274),
//     1/00/25 Important Notices concerning Free Open Source (2276) and
//     1/00/26 About Various Software License Agreements (2279). Each prints
//     "Does not correspond to a command" where a parameter table would be.
//     They are transcribed and COUNTED — ExpectedRows includes them — and
//     omitted from the inventory by address, because a menu line that names
//     no field is not an address an EX frame may read or write.
//
// THE CEILING IS TRANSCRIBED, AS THE OTHER KENWOOD STANZAS' ARE. 246 is
// core/kw.MaxEXDigits (core/kw/exdigits.go), and it cannot be written here
// as the symbol — this package is build-time tooling that RENDERS core/kw
// source text, and importing core/kw would cycle the dependency that
// one-way rule exists to keep. core/kw/exdigits_ceiling_test.go pins every
// profile with this ImportPath to it.
//
// THIS BOOK'S OWN ARITHMETIC WOULD GIVE 247, AND IT IS NOT USED. An EX
// answer here is "EX"(2) + P1(1) + P2(2) + P3(2) + P4(1) + ";"(1) = NINE
// fixed bytes before P5 (the frame table at 1898-1913), one fewer than the
// ten core/kw derives from pair 1's three-digit menu number, so 256 − 9 =
// 247 — which is numerically core/cat's MaxDigitsCeiling, the Yaesu number
// this family's ceiling test exists to keep distinct from its own. The
// family ceiling is what a Kenwood profile declares, so this stanza
// declares 246 and stays inside the arithmetic rather than one byte outside
// it. Neither figure is a claim about the widest value the radio prints:
// that argument rests on the ASSUMED-register entry the design labels A19,
// and an assumption is not lifted by a chart.
//
// ExpectedRows is 162: the printed ADDRESSED rows of both lists, 135 with
// P1=0 and 27 with P1=1. The chart's last printed line, "Firmware Version"
// (2281), is not among them — its P2 cell prints an em dash rather than an
// address, under the table's own note "P2 is any value." (2283) — and the
// four parameterless rows above ARE among them. If a transcription disagrees
// with this number, the answer is arbitration against the PDF, never an edit
// here.
//
// Deliberately NOT given a named accessor, for the reason the ftdx10, ftdx101,
// ft891 and ts590 profiles are not.
var ts890sProfile = Profile{
	Model:       "TS-890S",
	Package:     "ma",
	Types:       TypesImported,
	ImportPath:  "github.com/gm5dna/open-rig-programmer/core/kw",
	ImportAlias: "kw",
	VarName:     "exItems890S",
	OutFile:     "exinventory890s_gen.go",
	ManualCSV:   "menu890s.csv",

	Addresses:     AddressGrouped,
	LabelPolicy:   LabelsAbsent,
	TextRowPolicy: TextRowsAllowed,
	// Named BY ADDRESS, never by a count: a count-only gate would accept an
	// inventory that omitted the wrong four rows and still satisfied the
	// arithmetic. staleness890s_test.go fires exactly that falsification.
	ParameterlessPolicy: ParameterlessExcluded,
	ParameterlessAddresses: [][3]int{
		{1, 0, 23}, // Touchscreen Calibration
		{1, 0, 24}, // Software License Agreement
		{1, 0, 25}, // Important Notices concerning Free Open Source
		{1, 0, 26}, // About Various Software License Agreements
	},

	// core/kw's ceiling, transcribed from kw.MaxEXDigits; see above for why
	// this book's own 247 is not what a Kenwood profile declares.
	DigitsCeiling: 246,
	MinDigits:     3,
	MaxDigits:     4,
	TextWidths:    []int{10, 15},
	// MaxObservedWidth is an INERT API-REQUIRED SENTINEL here, exactly as on
	// the ObservationsAbsent profiles above: no TS-890S has ever been asked
	// anything by this project, so no observation CSV is ever parsed and this
	// bound is never consulted. It carries NO hardware claim and must not be
	// read as one; the moment observations do exist it is re-derived from
	// them rather than kept. It is spelt 12 for the reason ts480Profile
	// gives: that is what the other absent profiles spell, and a sentinel
	// that coincided with this chart's own widest width would look derived
	// from the chart, which is the one property a sentinel must not have.
	MaxObservedWidth: 12,
	ExpectedRows:     162,

	Observations: ObservationsAbsent,
	DocLines: []string{
		"exItems890S is the TS-890S's EX menu inventory, sorted by menu address,",
		"built from ONE source: the manual transcription in menu890s.csv (the",
		`TS-890S PC Control Command Reference Guide's "EX Command Parameter`,
		`Lists"). The address is a GROUPED triple — a one-digit menu type, a`,
		"two-digit category and a two-digit item, five wire characters — and the",
		"book prints its two menu types as two tables under one heading, so both",
		"are here: P1 0 is the Menu list and P1 1 the Advanced Menu list. The",
		"chart prints no group labels, so every P1Label and P2Label is \"\". There",
		"are no hardware READ observations to join — no TS-890S has ever been",
		"asked anything — so every item carries the absence sentinels",
		`ObservedReadWidth 0 and ObservedReadShape "".`,
		"Regenerate with `go generate ./core/kw/ma`; do not edit by hand.",
	},
}

// ts990sProfile carries the TS-990S's menu-chart transcription facts. Its
// inventory is emitted into core/kw/ma, outside core/cat, so EXItem and
// EXAddress are qualified by the explicit "kw" alias — and the ceiling it
// declares is core/kw's, not this package's constant.
//
// THE CEILING IS 246, WHICH IS core/kw.MaxEXDigits (core/kw/exdigits.go),
// AND THE ARITHMETIC FOR THIS RADIO WOULD HAVE GIVEN 247. Its EX answer is
// "EX"(2) + P1(1) + P2(2) + P3(2) + P4(1) + ";"(1) = NINE fixed bytes
// (docs/fixtures-private/manuals/ts990s_pc_rev2_layout.txt:1719-1747), so
// 256 − 9 = 247 is what a per-radio derivation would produce. 247 is
// numerically extable.MaxDigitsCeiling — core/cat's number, from a Yaesu
// answer's own nine bytes — and core/kw/exdigits_ceiling_test.go is what
// keeps the two distinct by requiring every profile with this ImportPath to
// carry core/kw's 246. This stanza carries 246 for that reason and not
// because the chart's widest printed value asks for it: the "widest printed
// value" argument rests on the ASSUMED bound the design labels A19.
//
// Evidence, all from the book whose cover prints "TS-990S / PC CONTROL
// COMMAND / Reference Guide" over "January/30/2019" (see
// core/kw/ma/menu990s.csv's own provenance header, which carries every
// transcription judgement and its layout line):
//
//   - The chart prints a (P1,P2,P3) triple of ONE, TWO and TWO digits —
//     AddressGrouped, all three components on the wire (layout :1767-1768
//     for the column headers, :1723 for the P1 enumeration "0: Menu /
//     1: Advanced Menu").
//   - LabelsAbsent. The chart has no group-label columns at all: its
//     columns are P1, P2, P3, Function and P5 (:1767-1768). This was read
//     off the layout text AND CONFIRMED against 300-dpi renders of printed
//     pages 22, 23, 24, 25, 26 and 27 (PDF pages 23-28). The two table
//     titles "Menu" (:1765) and "Advanced Menu" (:2194) are the P1
//     enumeration, not per-row labels.
//   - MinDigits 3 / MaxDigits 8. The P5 column headers ARE the codes and
//     they are three characters wide (:1768), which is the width of every
//     enumerated row and the narrowest the chart prints; 8 is the widest,
//     from the 28 Fixed Mode band-limit rows' "8-digit frequency (in Hz)
//     with unused digits entered as 0" (:2070-2159). Nineteen rows lie
//     between: 0/05/11 Contest Number at 4 (:1972), and the eighteen
//     PF-key rows 0/00/15 to 0/00/32, whose own cell states no width and
//     whose width the EX command page prints instead — "PF key settings
//     use 4 digits (refer to the PF Key assignment ID lists)" (:1747-1748),
//     the "PF Key Assignment Lists" (:2287) running to four-character IDs
//     (:2341). The transcription leg read those rows as 3 and recorded the
//     disagreement; ORCHESTRATOR RULING R-B settled it at 4, on both this
//     radio and the 890S. MaxDigits did not move.
//   - TextRowsAllowed with TextWidths {10, 15}: menu 0/00/06 Screen Saver
//     Message, "Up to 10 alphanumeric characters" (:1778), and menu 0/00/07
//     Power-on Message, "Up to 15 alphanumeric characters" (:1779). The EX
//     command page states both counts again (:1751-1752). They are the
//     chart's only two free-text fields; the numeric-legend rows above
//     state a WIDTH, not a character count, and are NOT text.
//   - ParameterlessRefused. Every ADDRESSED row of this chart names a
//     settable field, so a hyphen in a Digits cell is a transcription
//     error. The four rows drawn with a dash where the address should be
//     (:2272-2279) are not rows of the inventory at all: an unaddressed row
//     names no EX address, so there is nothing for the exclusion policy to
//     exclude.
//
// ExpectedRows is 194 — 167 rows with P1=0 and 27 with P1=1 — and it comes
// from the transcription leg's own count of the chart's ADDRESSED rows, not
// from two transcriptions agreeing with each other. If a later leg counts
// something else, the answer is arbitration against the PDF, never an edit
// here.
//
// Deliberately NOT given a named accessor, for the reason the other
// TypesImported profiles are not.
var ts990sProfile = Profile{
	Model:       "TS-990S",
	Package:     "ma",
	Types:       TypesImported,
	ImportPath:  "github.com/gm5dna/open-rig-programmer/core/kw",
	ImportAlias: "kw",
	VarName:     "exItems990S",
	OutFile:     "exinventory990s_gen.go",
	ManualCSV:   "menu990s.csv",

	Addresses:           AddressGrouped,
	LabelPolicy:         LabelsAbsent,
	TextRowPolicy:       TextRowsAllowed,
	ParameterlessPolicy: ParameterlessRefused,

	// core/kw's ceiling, transcribed from kw.MaxEXDigits; see above for why
	// this radio's own 247 is the one number it must not be.
	DigitsCeiling: 246,
	MinDigits:     3,
	MaxDigits:     8,
	TextWidths:    []int{10, 15},
	// MaxObservedWidth is an INERT API-REQUIRED SENTINEL here. Validate
	// demands a positive value from every profile, but this profile declares
	// ObservationsAbsent — no TS-990S has ever been asked anything by this
	// project — so no observation CSV is ever parsed and this bound is never
	// consulted. It carries NO hardware claim and must not be read as one;
	// the moment observations do exist it is re-derived from them rather
	// than kept. It is spelt 12 because that is what the other
	// ObservationsAbsent profiles spell, and a sentinel that coincided with
	// one of this chart's own widths would look derived from the chart,
	// which is the one property a sentinel must not have.
	MaxObservedWidth: 12,
	ExpectedRows:     194,

	Observations: ObservationsAbsent,
	DocLines: []string{
		"exItems990S is the TS-990S's EX menu inventory, sorted by (P1,P2,P3),",
		"built from ONE source: the manual transcription in menu990s.csv (the",
		`TS-990S PC Control Command Reference Guide's "EX Command Parameter`,
		`Lists" — both printed lists, "Menu" and "Advanced Menu"). The address`,
		"is a THREE-component group of one, two and two digits, all of them on",
		"the wire, and the chart prints no group labels, so every P1Label and",
		`P2Label is "". There are no hardware READ observations to join — no`,
		"TS-990S has ever been asked anything — so every item carries the",
		`absence sentinels ObservedReadWidth 0 and ObservedReadShape "".`,
		"Regenerate with `go generate ./core/kw/ma`; do not edit by hand.",
	},
}

// registry maps a lookup name to its profile. It is validated at init, so an
// inconsistent profile panics the build tooling rather than emitting a wrong
// inventory.
var registry = mustRegistry(map[string]Profile{
	"ft710":   ft710Profile,
	"ft891":   ft891Profile,
	"ft991a":  ft991aProfile,
	"ftdx10":  ftdx10Profile,
	"ftdx101": ftdx101Profile,
	"ts480":   ts480Profile,
	"ts590s":  ts590sProfile,
	"ts590sg": ts590sgProfile,
	"ts890s":  ts890sProfile,
	"ts990s":  ts990sProfile,
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
