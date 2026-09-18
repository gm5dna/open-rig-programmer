// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"go/ast"
	"slices"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// TestSimulatedProfileTokensConfinement is the DATA-DRIVEN, N-driver
// generalisation of the single-driver guard
// TestSimulatedTokenSingleNonTestFileRepoWide that used to live in
// importgraph_test.go — retired at Task 58 once its pin lifted (see the
// PIN-LIFT LEDGER NOTE below). For every concrete driver listed in
// simulatedProfiles it pins the same structural-exclusivity invariant
// (task-11 brief §3) that the retired single-driver guard pinned for
// ft710 alone: the driver's simulated-profile
// selector — e.g. ft710.Simulated — may appear in exactly ONE non-test
// .go file across the whole repository; that one file must live in
// internal/wiring (the shared fake-wiring home since task-15's
// extraction); and it must ALSO call the driver's fake constructor
// (fakeradio.New). Together these keep the RealHardware/fakeradio and
// Simulated/real-port pairings structurally unrepresentable rather than
// merely conventionally so — the failure this guard exists to catch is a
// second, stray reference to a simulated-profile constant leaking the
// fake-only path into production wiring.
//
// PIN-LIFT LEDGER NOTE. This guard ran in parallel with the pinned
// single-driver TestSimulatedTokenSingleNonTestFileRepoWide, which the
// M8 roadmap kept byte-identical in importgraph_test.go. The pin lifted
// at M9b, as planned — M8e was the other candidate, and the M8d
// menu-write no-go (25/07/2026) removed it. It folded into this table
// (its ft710 row was already the sole entry here) and was deleted from
// importgraph_test.go at Task 58 (26/07/2026); this data-driven guard is
// now the sole check of the ft710 fact.
//
// ALIAS-PROOF, deliberately (Codex plan-review F10). Unlike the retired
// single-driver guard's bare `x.Name == "ft710"` identifier match, this guard resolves
// the driver package through each file's AST import map: it looks for the
// token selector on whatever LOCAL name core/driver/<pkg> is imported as.
// A file that smuggled in a second reference via an aliased import —
//
//	import drv "…/core/driver/ft710"
//	… drv.Simulated …
//
// would evade the bare-identifier guard but is caught here. The fake
// constructor is resolved the same way, through its FULL import path
// (internal/fakeradio, from the profile row's fakeCtorPath), so an aliased
// fakeradio import is honoured too — and, unlike a path-BASE match, an
// unrelated package that merely ends in "/fakeradio" cannot satisfy the
// pairing check in place of the real internal/fakeradio.New (M9a
// Codex-review finding 2, LOW).
//
// APPROXIMATE (see the package doc comment): a plain AST walk over
// selector and call expressions, not a type-checked analysis. The token
// is matched by (resolved-package-local-name, selector-name); a
// dot-import of the driver package would evade it, and nothing in this
// repo dot-imports anything.
func TestSimulatedProfileTokensConfinement(t *testing.T) {
	// simulatedProfiles is the driver table this guard walks. Each row
	// names a concrete driver package (core/driver/<pkg>), its simulated-
	// profile constant (<pkg>.<token>), and the fake constructor the sole
	// file referencing that token must also call.
	//
	// TWO rows since M9c-6 task 6, and that is the design working: the
	// FTdx10's registration added a ROW here — not a second test, not a
	// second walk — so the FTdx10's simulated profile is confined by
	// exactly the check that already confined the FT-710's, including the
	// pairing clause (the one file naming ftdx10.Simulated must also call
	// fakedx10.New, so a Simulated driver can never be wired to another
	// model's rig or to a real port). Each further driver is one more row.
	//
	// FOUR ROWS SINCE M9d-2 TASK 7, and the last two are NOT one per driver
	// package: core/driver/ftdx101 drives BOTH FTDX101 siblings from one
	// type, so there is ONE package and ONE Simulated token, but TWO fake
	// constructors — fakedx101.NewD and fakedx101.NewMP — and internal/wiring
	// must call both in its one fake-wiring file. A row is therefore
	// (package, token, fake CONSTRUCTOR), and the FTdx101 contributes two
	// rows differing only in fakeCtor. The confinement clauses (exactly one
	// non-test file, and it lives in internal/wiring) are checked twice over
	// the same token and agree trivially; the PAIRING clause is what earns
	// the second row, because a registration that wired both models to
	// fakedx101.NewD would satisfy the D row and fail the MP one.
	// THE models COLUMN IS NEW AT TIER 6 (M8, tightened at Codex re-review
	// MED-1a), and it exists because a row cannot stand for "one model"
	// without saying WHICH. This table's cardinality is one row per fake
	// CONSTRUCTOR, not per registered model: the FTdx101 pair earns two rows
	// from its two constructors, while the IC-7851/IC-7850 pair shares one
	// constructor and is ONE row for TWO registered models. A bare
	// len(SupportedModels()) comparison would therefore already have been
	// wrong before any Kenwood row existed. Each row now NAMES every
	// SupportedModels() entry its confinement clause protects, and the guard
	// below asserts that the UNION of those names equals SupportedModels()
	// as a set — see assertEveryRegisteredModelIsConfined.
	simulatedProfiles := []struct {
		pkg          string   // base name of core/driver/<pkg>
		token        string   // the simulated-profile constant, e.g. "Simulated"
		fakeCtor     string   // "<pkgbase>.<Func>" the sole file must also call — for messages and the func name
		fakeCtorPath string   // the constructor package's FULL import path, below modulePrefix (e.g. "internal/fakeradio")
		models       []string // every wiring.SupportedModels() entry this row's confinement protects
	}{
		{"ft710", "Simulated", "fakeradio.New", "internal/fakeradio", []string{"FT-710"}},
		{"ftdx10", "Simulated", "fakedx10.New", "internal/fakedx10", []string{"FTdx10"}},
		{"ftdx101", "Simulated", "fakedx101.NewD", "internal/fakedx101", []string{"FTdx101D"}},
		{"ftdx101", "Simulated", "fakedx101.NewMP", "internal/fakedx101", []string{"FTdx101MP"}},
		// The IC-7610 (Wave 4 task R1), this project's first non-Yaesu
		// row: one package, one Simulated token and one fake constructor —
		// the ftdx10 shape, not the ftdx101 shared-driver/two-siblings
		// one, since core/driver/ic7610 has no registered sibling (matrix
		// §4). The AST walk this guard runs is name-based, not
		// import-path-based, so it does not care that internal/fakeic7610's
		// New is wrapped in an ic7610FakeAdapter{...} composite literal at
		// its one call site in internal/wiring/fake.go — the call
		// expression fakeic7610.New(...) is still there, nested inside it,
		// and fileHasCall's ast.Inspect walk finds it regardless of what
		// encloses it.
		{"ic7610", "Simulated", "fakeic7610.New", "internal/fakeic7610", []string{"IC-7610"}},
		// The IC-7300 and IC-7300MK2 (Wave 4 task R3), this project's
		// second Icom family and first Icom PAIR: TWO ROWS OVER ONE
		// PACKAGE, which is the FTdx101 shape rather than the ic7610 one.
		// The v1.4.1 sweep folded core/driver/ic7300mk2 into
		// core/driver/ic7300 (audit finding 8), so there is now ONE pkg
		// and ONE Simulated token — but still TWO fakes, internal/fakeic7300
		// and internal/fakeic7300mk2, independently written and NOT folded,
		// and a row is (package, token, fake CONSTRUCTOR). The second row
		// is earned by fakeic7300mk2.New exactly as the FTdx101's is by
		// fakedx101.NewMP: the confinement clauses agree trivially over the
		// shared token, and the PAIRING clause is what the row is for —
		// a registration that wired the MK2 to fakeic7300.New would satisfy
		// the first row and fail this one. Both fakes' New calls appear
		// directly in internal/wiring/fake.go's fakeDrivers table (no
		// adapter wraps either — both Port() methods already return
		// io.ReadWriteCloser), so the AST walk finds each
		// fakeic7300.New(...) / fakeic7300mk2.New(...) call expression
		// exactly where the ic7610 row's comment says it would even if one
		// had been wrapped.
		{"ic7300", "Simulated", "fakeic7300.New", "internal/fakeic7300", []string{"IC-7300"}},
		{"ic7300", "Simulated", "fakeic7300mk2.New", "internal/fakeic7300mk2", []string{"IC-7300MK2"}},
		// The IC-705 (Wave 4 task R4), this project's third Icom
		// registration and second lone-model one: one package, one
		// Simulated token and one fake constructor, on the same ic7610
		// shape as above (no adapter wraps fakeic705.New — its Port()
		// already returns io.ReadWriteCloser — so the row's shape is the
		// simpler of the two this table already carries).
		{"ic705", "Simulated", "fakeic705.New", "internal/fakeic705", []string{"IC-705"}},
		// The IC-9700 (Wave 4 task R5), this project's fourth Icom
		// registration and second lone-model one since the IC-705: one
		// package, one Simulated token and one fake constructor, on the
		// same ic705 shape as above (no adapter wraps fakeic9700.New —
		// its Port() already returns io.ReadWriteCloser — so the row's
		// shape is the simpler of the two this table carries, three
		// static banks notwithstanding: this table asks nothing about
		// bank shape).
		{"ic9700", "Simulated", "fakeic9700.New", "internal/fakeic9700", []string{"IC-9700"}},
		// The IC-905 (Wave 4 task R6, the tier's LAST registration), this
		// project's fifth Icom registration and third lone-model one
		// since the IC-705: one package, one Simulated token and one
		// fake constructor, on the same ic705/ic9700 shape as above (no
		// adapter wraps fakeic905.New — its Port() already returns
		// io.ReadWriteCloser).
		{"ic905", "Simulated", "fakeic905.New", "internal/fakeic905", []string{"IC-905"}},
		// The IC-7851 and IC-7850 (Tier 4b, the additions tier's first
		// registration): ONE row for TWO registered models, and the
		// reason is the column definition rather than a relaxation. A row
		// is (package, token, fake CONSTRUCTOR); core/driver/ic7851 is
		// one package with one simulated-profile selector, and
		// internal/fakeic7851 offers ONE constructor which both
		// fakeDrivers rows call. The FTdx101's two rows are earned by its
		// two constructors (fakedx101.NewD and fakedx101.NewMP); there is
		// no second constructor here to earn a second row, and a
		// duplicate row would assert the identical fact twice.
		//
		// THE TOKEN IS AN OPTION, NOT A Profile CONSTANT, and it is the
		// first row here of which that is true: core/driver/ic7851 takes
		// its profile through WithSimulatedProfile() (its Profile type's
		// Simulated value is never named from outside the package), so
		// the selector a stray non-test reference would have to smuggle
		// in is that option. Naming ic7851.Simulated here instead would
		// check a token no non-test file mentions, and this guard's own
		// non-vacuity clause would fail the row rather than confine
		// anything.
		//
		// The fakeic7851.New call sits inside an ic7851FakeAdapter{...}
		// composite literal at both call sites, exactly as the IC-7610's
		// does; fileHasCall's ast.Inspect walk finds the call regardless
		// of what encloses it (see the ic7610 row's own note).
		// TWO MODEL NAMES ON ONE ROW, the only such row today: one
		// constructor confines both registry rows, and the union check
		// below would report "IC-7850" as an unconfined registered model
		// if this column named only the 7851.
		{"ic7851", "WithSimulatedProfile", "fakeic7851.New", "internal/fakeic7851", []string{"IC-7851", "IC-7850"}},
		// The IC-7760 (Tier 4b's second registration): ONE row for ONE
		// registered model, and its token is a Profile CONSTANT again —
		// core/driver/ic7760 takes its profile as New's first ARGUMENT,
		// so ic7760.Simulated is the selector a stray non-test reference
		// would have to smuggle in, exactly as for the six rows above the
		// IC-7851's. The pair's WithSimulatedProfile row remains the one
		// option-shaped exception in this table, and it is an exception
		// about THAT package's constructor shape, not a new convention.
		//
		// The fakeic7760.New call sits inside an ic7760FakeAdapter{...}
		// composite literal at its one call site, exactly as the
		// IC-7610's and the IC-7851 pair's do; fileHasCall's ast.Inspect
		// walk finds the call regardless of what encloses it (see the
		// ic7610 row's own note).
		{"ic7760", "Simulated", "fakeic7760.New", "internal/fakeic7760", []string{"IC-7760"}},
		// The IC-7100 (Tier 4b's third registration): ONE row for ONE
		// registered model, and its token is a Profile CONSTANT again —
		// core/driver/ic7100 takes its profile as New's first ARGUMENT
		// (ic7100.go: `func New(profile Profile, opts ...Option)`), so
		// ic7100.Simulated is the selector a stray non-test reference
		// would have to smuggle in. The IC-7851 pair's
		// WithSimulatedProfile row remains the one option-shaped
		// exception in this table.
		//
		// The fakeic7100.New call sits BARE at its one call site — not
		// inside a composite literal, because internal/fakeic7100's
		// Port() already returns io.ReadWriteCloser and this row needs no
		// adapter. fileHasCall's ast.Inspect walk finds the call either
		// way (see the ic7610 row's own note); the difference is recorded
		// here only so a reader is not surprised to find no
		// ic7100FakeAdapter beside the ic7610's and the ic7760's.
		{"ic7100", "Simulated", "fakeic7100.New", "internal/fakeic7100", []string{"IC-7100"}},
		// The IC-R8600 (Tier 4b's fourth and last registration): ONE row
		// for ONE registered model, and its token is a Profile CONSTANT
		// again — core/driver/icr8600 takes its profile as New's first
		// ARGUMENT (icr8600.go:41, `func New(profile Profile, opts
		// ...Option)`), so icr8600.Simulated is the selector a stray
		// non-test reference would have to smuggle in. The IC-7851 pair's
		// WithSimulatedProfile row remains the one option-shaped exception
		// in this table.
		//
		// The fakeicr8600.New call sits BARE at its one call site, as the
		// IC-7100's does and for the same reason: internal/fakeicr8600's
		// Port() already returns io.ReadWriteCloser, so this row needs no
		// adapter and there is no icr8600FakeAdapter to find beside the
		// ic7610's and the ic7760's. fileHasCall's ast.Inspect walk finds
		// the call either way (see the ic7610 row's own note).
		{"icr8600", "Simulated", "fakeicr8600.New", "internal/fakeicr8600", []string{"IC-R8600"}},
		// The FT-891 (Tier 1, the first YAESU registration since M9d-2):
		// ONE row for ONE registered model, and its token is a Profile
		// CONSTANT — core/driver/ft891 takes its profile as New's first
		// ARGUMENT (ft891.go:71, `func New(profile Profile, opts
		// ...Option) driver.Driver`), so ft891.Simulated is the selector a
		// stray non-test reference would have to smuggle in. The IC-7851
		// pair's WithSimulatedProfile row remains the one option-shaped
		// exception in this table.
		//
		// The fakeft891.New call sits BARE at its one call site, as the
		// IC-7100's and IC-R8600's do and for the same reason:
		// internal/fakeft891's Port() already returns io.ReadWriteCloser
		// (fakeft891.go:89), so this row needs no adapter and there is no
		// ft891FakeAdapter to find beside the ic7610's and the ic7760's.
		// fileHasCall's ast.Inspect walk finds the call either way (see
		// the ic7610 row's own note).
		//
		// THE PAIRING CLAUSE IS WHAT THIS ROW REALLY BUYS, exactly as the
		// ftdx10 row's comment says it did for the second driver: the one
		// file naming ft891.Simulated must also call fakeft891.New, so a
		// Simulated FT-891 driver can never be wired to another model's
		// rig or to a real port.
		{"ft891", "Simulated", "fakeft891.New", "internal/fakeft891", []string{"FT-891"}},
		// The FT-991A (Tier 1's second registration): ONE row for ONE
		// registered model, and its token is a Profile CONSTANT on the
		// FT-891's terms exactly — core/driver/ft991a takes its profile as
		// New's first ARGUMENT (`func New(profile Profile, opts ...Option)
		// driver.Driver`), so ft991a.Simulated is the selector a stray
		// non-test reference would have to smuggle in. The IC-7851 pair's
		// WithSimulatedProfile row remains the one option-shaped exception
		// in this table.
		//
		// The fakeft991a.New call sits BARE at its one call site, as the
		// FT-891's, the IC-7100's and the IC-R8600's do and for the same
		// reason: internal/fakeft991a's Port() already returns
		// io.ReadWriteCloser, so this row needs no adapter and there is no
		// ft991aFakeAdapter to find beside the ic7610's and the ic7760's.
		// fileHasCall's ast.Inspect walk finds the call either way (see
		// the ic7610 row's own note).
		//
		// THE PAIRING CLAUSE IS WHAT THIS ROW REALLY BUYS: the one file
		// naming ft991a.Simulated must also call fakeft991a.New, so a
		// Simulated FT-991A driver can never be wired to another model's
		// rig or to a real port.
		{"ft991a", "Simulated", "fakeft991a.New", "internal/fakeft991a", []string{"FT-991A"}},
		// The TS-590S and TS-590SG (Tier 6, the first KENWOOD registration):
		// ONE row for TWO registered models, the IC-7851 pair's shape rather
		// than the FTdx101's, and for the column's own reason. A row is
		// (package, token, fake CONSTRUCTOR); core/driver/ts590 is one
		// package with one simulated-profile selector, and internal/fakets590
		// offers ONE constructor — fakets590.New — which both fakeDrivers
		// rows call, differing in the ROW ARGUMENT they pass it. The
		// FTdx101's two rows are earned by its two constructors; there is no
		// second constructor here to earn a second row, and a duplicate row
		// would assert the identical fact twice.
		//
		// WHICH SIBLING EACH CALL ASKED FOR IS NOT THIS GUARD'S QUESTION —
		// this is an AST walk over call expressions and it cannot read an
		// argument's meaning. internal/wiring's own
		// TestOpenFakeSessionFor_EveryRegisteredModel is what catches a
		// crossed row, by comparing each driver's static identity against
		// what the rig answered.
		//
		// Its token is a Profile CONSTANT: core/driver/ts590's New takes the
		// profile as its SECOND argument (the row is the first), so
		// ts590.Simulated is the selector a stray non-test reference would
		// have to smuggle in. The IC-7851 pair's WithSimulatedProfile row
		// remains the one option-shaped exception in this table.
		//
		// The fakets590.New call sits BARE at both call sites, as the
		// IC-7100's, IC-R8600's and FT-891's do: internal/fakets590's Port()
		// already returns io.ReadWriteCloser, so no adapter wraps it.
		{"ts590", "Simulated", "fakets590.New", "internal/fakets590", []string{"TS-590S", "TS-590SG"}},
		// The TS-890S and TS-990S (Tier 6's SECOND Kenwood pair): TWO ROWS
		// FOR TWO REGISTERED MODELS, which is the ic7610/ft891 shape rather
		// than either of the pair shapes above it, and the column definition
		// is again what decides. A row is (package, token, fake
		// CONSTRUCTOR); these two radios' memory records are different
		// shapes, so plan decision P1 gives each its own driver package and
		// its own simulator — two packages, two Simulated tokens, two
		// constructors, one registered model each. Nothing is shared for a
		// pairing clause to have to disambiguate.
		//
		// Each token is a Profile CONSTANT: both New functions take the
		// profile as their FIRST and only required argument, so
		// ts890.Simulated and ts990.Simulated are the selectors a stray
		// non-test reference would have to smuggle in. The IC-7851 pair's
		// WithSimulatedProfile row remains the one option-shaped exception in
		// this table.
		//
		// Both fake constructors sit BARE at their one call site each:
		// internal/fakets890's and internal/fakets990's Port() methods
		// already return io.ReadWriteCloser, so neither needs an adapter.
		{"ts890", "Simulated", "fakets890.New", "internal/fakets890", []string{"TS-890S"}},
		{"ts990", "Simulated", "fakets990.New", "internal/fakets990", []string{"TS-990S"}},
		// The IC-7800 (v1.7.0 Icom wave's first registration): one row, one
		// driver package, one fake, on the IC-7610's footing.
		{"ic7800", "Simulated", "fakeic7800.New", "internal/fakeic7800", []string{"IC-7800"}},
		// The IC-7600 (v1.7.0 Icom wave's second registration), on the same
		// footing.
		{"ic7600", "Simulated", "fakeic7600.New", "internal/fakeic7600", []string{"IC-7600"}},
		// The IC-7410 (v1.7.0 Icom wave's third registration): the token is
		// an OPTION, WithSimulatedProfile, the IC-7610's own bare-New shape
		// rather than a Profile constant.
		{"ic7410", "WithSimulatedProfile", "fakeic7410.New", "internal/fakeic7410", []string{"IC-7410"}},
		// The IC-7700 (v1.7.0 Icom wave's fourth registration), on
		// IC7800Model's footing.
		{"ic7700", "Simulated", "fakeic7700.New", "internal/fakeic7700", []string{"IC-7700"}},
		// The IC-9100 (v1.7.0 Icom wave's fifth registration), on
		// IC7800Model's footing.
		{"ic9100", "Simulated", "fakeic9100.New", "internal/fakeic9100", []string{"IC-9100"}},
		// The IC-7200 (v1.7.0 Icom wave's sixth and last registration), on
		// IC7800Model's footing.
		{"ic7200", "Simulated", "fakeic7200.New", "internal/fakeic7200", []string{"IC-7200"}},
		// The FTdx5000 (v1.7.0 Kenwood/Yaesu wave, tenth row): bare New
		// takes the profile as its first argument, one package, one
		// simulator, no sibling.
		{"ftdx5000", "Simulated", "fakeftdx5000.New", "internal/fakeftdx5000", []string{"FTdx5000"}},
		// The TS-2000 family (v1.7.0 Kenwood/Yaesu wave, first row of
		// three): core/driver/ts2000 has NO Profile positional argument at
		// all (NewTS2000/NewTS2000X/NewTSB2000 take opts ...Option only),
		// so there is no bare "ts2000.Simulated" selector for this guard to
		// find anywhere — the token column names WithSimulatedProfile
		// instead, the option function fake.go calls to select the fake
		// capability arm. The mechanism is generic over any selector name,
		// not just a constant (fileHasSelector matches any <recv>.<sel>),
		// so this is the same confinement check on this package's own
		// shape. ONE row for THREE registered models.
		{"ts2000", "WithSimulatedProfile", "fakets2000.New", "internal/fakets2000", []string{"TS-2000", "TS-2000X", "TS-B2000"}},
		// The TS-570 family (v1.7.0 Kenwood/Yaesu wave, fourth row of
		// three): core/driver/ts570's NewD/NewS/NewDG each take the
		// profile as their first argument, the ordinary shape, so the
		// token is a bare "ts570.Simulated" constant. ONE row for THREE
		// registered models (one constructor family, one fake).
		{"ts570", "Simulated", "fakets570.New", "internal/fakets570", []string{"TS-570D", "TS-570S", "TS-570DG"}},
		// The TS-870S (v1.7.0 Kenwood/Yaesu wave, seventh row): bare New
		// takes the profile as its first argument, one package, one
		// simulator, no sibling.
		{"ts870s", "Simulated", "fakets870s.New", "internal/fakets870s", []string{"TS-870S"}},
		// The FT-2000 family (v1.7.0 Kenwood/Yaesu wave, eighth row of
		// two): core/driver/ft2000's NewFT2000/NewFT2000D each take the
		// profile as their first argument, the ordinary shape. ONE row for
		// TWO registered models (one constructor family, one fake).
		{"ft2000", "Simulated", "fakeft2000.New", "internal/fakeft2000", []string{"FT-2000", "FT-2000D"}},
		// The FTdx9000 (v1.7.0 Kenwood/Yaesu wave, eleventh row): bare New
		// takes the profile as its first argument, one package, one
		// simulator, no sibling.
		{"ftdx9000", "Simulated", "fakeftdx9000.New", "internal/fakeftdx9000", []string{"FTdx9000"}},
		// The FT-950 (v1.7.0 Kenwood/Yaesu wave, twelfth and last row):
		// bare New takes the profile as its first argument, one package,
		// one simulator, no sibling.
		{"ft950", "Simulated", "fakeft950.New", "internal/fakeft950", []string{"FT-950"}},
		// The FTdx3000 (v1.8.0 Yaesu trio, first row): bare New takes the
		// profile as its first argument, one package, one simulator, no
		// sibling.
		{"ftdx3000", "Simulated", "fakeftdx3000.New", "internal/fakeftdx3000", []string{"FTdx3000"}},
		// The FTdx1200 (v1.8.0 Yaesu trio, second row): bare New takes the
		// profile as its first argument, one package, one simulator, no
		// sibling.
		{"ftdx1200", "Simulated", "fakeftdx1200.New", "internal/fakeftdx1200", []string{"FTdx1200"}},
		// The FT-450D (v1.8.0 Yaesu trio, third and last row): bare New
		// takes the profile as its first argument, one package, one
		// simulator, no sibling.
		{"ft450d", "Simulated", "fakeft450d.New", "internal/fakeft450d", []string{"FT-450D"}},
		// The FT-890 (v1.9.0 binary-CAT four, first row): the
		// FTdx101D/FTdx101MP shape — core/driver/ft890900 has ONE
		// Simulated token but its own fake constructor per row,
		// fakeft890.New here and (separately registered) fakeft900.New
		// for its sibling row.
		{"ft890900", "Simulated", "fakeft890.New", "internal/fakeft890", []string{"FT-890"}},
		// The FT-900 (v1.9.0 binary-CAT four, second row): FT-890's own
		// sibling row over the same package, its own fake constructor.
		{"ft890900", "Simulated", "fakeft900.New", "internal/fakeft900", []string{"FT-900"}},
		// The FT-1000MP/Mark-V (v1.9.0 binary-CAT four, fourth and last
		// row): bare New takes the profile as its first argument, one
		// package, one simulator, no sibling.
		{"ft1000mp", "Simulated", "fakeft1000mp.New", "internal/fakeft1000mp", []string{"FT-1000MP"}},
		// The FTX-1 (v1.10.0): bare New takes the profile as its first
		// argument, one package, one simulator, no sibling.
		{"ftx1", "Simulated", "fakeftx1.New", "internal/fakeftx1", []string{"FTX-1"}},
		// NO ts480 ROW, DELIBERATELY (plan decision P3). core/driver/ts480 is
		// BUILT and NOT REGISTERED: it is absent from internal/wiring's
		// realDrivers and fakeDrivers, so there is no fake-wiring call site
		// for this guard's pairing clause to find and its non-vacuity clause
		// would fail the row rather than confine anything. The absence is
		// validated SEPARATELY and by name — see the
		// assertNoTS480RowUntilTheRowRegisters helper below, called from this
		// test — so that a premature ts480 row is caught by ITS OWN assertion rather
		// than as a side effect of the union equality, which needs no
		// exception for it: an unregistered model is simply absent from
		// SupportedModels(). Adding this row is edit 8 of the ten-edit
		// registration list (task 19).
	}

	// Non-vacuity: an empty table would make the loop below a no-op and
	// pass silently.
	if len(simulatedProfiles) == 0 {
		t.Fatal("simulatedProfiles is empty — this guard would pass vacuously; it must list every concrete driver")
	}

	// THE COMPLETENESS GUARD (M8). Until Tier 6 this file mentioned
	// wiring.SupportedModels() nowhere at all — grep: zero — and the
	// len(...) == 0 check above was its whole non-vacuity story, so a
	// REGISTERED MODEL WITH NO ROW HERE WAS SILENTLY UNCONFINED: its
	// simulated-profile token could appear in any number of non-test files
	// and nothing would say so. That is the FT-891's HIGH-1 lesson (a
	// hand-written table letting a new model escape) in a table that was
	// worse off than the ones that lesson was drawn from, because those at
	// least compared their length against the registry.
	var models, pkgs []string
	for _, prof := range simulatedProfiles {
		models = append(models, prof.models...)
		pkgs = append(pkgs, prof.pkg)
	}
	assertEveryRegisteredModelIsConfined(t, models)
	assertNoTS480RowUntilTheRowRegisters(t, pkgs, models)

	files := parseRepo(t)

	for _, prof := range simulatedProfiles {
		driverPath := modulePrefix + "core/driver/" + prof.pkg
		_, ctorFunc, ok := strings.Cut(prof.fakeCtor, ".")
		if !ok {
			t.Fatalf("profile %q: fakeCtor %q is not in <pkg>.<Func> form", prof.pkg, prof.fakeCtor)
		}
		ctorPath := modulePrefix + prof.fakeCtorPath

		var filesWithToken []string
		sawCtorInWiring := false

		for _, pf := range files {
			// Alias-proof: only files that actually import the driver can
			// name <pkg>.token, and we match the token on the LOCAL name
			// that import was bound to (alias or path base).
			driverLocal, importsDriver := importsPath(pf.file, driverPath)
			if !importsDriver {
				continue
			}
			if !fileHasSelector(pf.file, driverLocal, prof.token) {
				continue
			}
			filesWithToken = append(filesWithToken, pf.relPath)

			if pf.relDir != "internal/wiring" {
				t.Errorf("profile %q: %s references %s.%s but lives outside internal/wiring — the simulated-profile token must be confined to the fake-wiring file (task-11 brief §3)", prof.pkg, pf.relPath, prof.pkg, prof.token)
				continue
			}
			// The sole in-wiring file must also construct the fake rig,
			// resolving the constructor's package alias-proof by its FULL
			// import path (not merely its path base — see this test's doc
			// comment, finding 2): only the real internal/fakeradio import,
			// under whatever local name it is bound to, can satisfy this.
			if ctorLocal, importsCtor := importsPath(pf.file, ctorPath); importsCtor && fileHasCall(pf.file, ctorLocal, ctorFunc) {
				sawCtorInWiring = true
			}
		}

		// Non-vacuity per row: the token must have been observed at all. A
		// typo'd pkg (wrong driverPath) or a broken walk would match zero
		// files; without this the "exactly one" check could not tell
		// "moved to zero" apart from "walk found nothing".
		if len(filesWithToken) == 0 {
			t.Errorf("profile %q: never saw %s.%s in any non-test file — the driver import path (%s) or the walk is wrong, and this row was checked vacuously", prof.pkg, prof.pkg, prof.token, driverPath)
			continue
		}
		if len(filesWithToken) != 1 {
			t.Errorf("profile %q: %s.%s appears in %d non-test files repo-wide (%v), want exactly 1 — the Simulated/fake pairing must stay confined to the one fake-wiring file (task-11 brief §3)", prof.pkg, prof.pkg, prof.token, len(filesWithToken), filesWithToken)
			continue
		}
		if !sawCtorInWiring {
			t.Errorf("profile %q: the sole file referencing %s.%s (%s) does not also call %s — the fake-wiring constructor must build the simulated driver and its fake rig together (task-11 brief §3)", prof.pkg, prof.pkg, prof.token, filesWithToken[0], prof.fakeCtor)
		}
	}
}

// fileHasSelector reports whether f contains a selector expression
// <recv>.<sel> whose receiver is the bare identifier recv (the local name
// an import was bound to). Comments are AST-invisible, so a token named
// only in prose does not count.
func fileHasSelector(f *ast.File, recv, sel string) bool {
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		s, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if id, ok := s.X.(*ast.Ident); ok && id.Name == recv && s.Sel.Name == sel {
			found = true
			return false
		}
		return true
	})
	return found
}

// fileHasCall reports whether f contains a call expression <recv>.<fn>(...)
// whose receiver is the bare identifier recv.
func fileHasCall(f *ast.File, recv, fn string) bool {
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if s, ok := call.Fun.(*ast.SelectorExpr); ok {
			if id, ok := s.X.(*ast.Ident); ok && id.Name == recv && s.Sel.Name == fn {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

// assertEveryRegisteredModelIsConfined is TestSimulatedProfileTokensConfinement's
// completeness half (M8): the UNION of every simulatedProfiles row's models
// column, as a set, must equal wiring.SupportedModels() as a set.
//
// NO EXCEPTION PARAMETER, deliberately (Codex re-review MED-1a). An
// unregistered model — the TS-480 today — is simply absent from
// SupportedModels() and needs no exemption; folding its absence in here as a
// named exclusion would make a premature ts480 row look like the exception
// working rather than the omission it is. That absence has its own assertion
// with its own two messages, assertNoTS480RowUntilTheRowRegisters below.
//
// DUPLICATES ARE AN ERROR TOO, not merely tolerated: two rows claiming the
// same model would make the union equality hold while one of the two rows
// asserted nothing anybody had noticed.
//
// RED-PROVED TWO WAYS at the commit that added it (recorded, not re-run by
// CI): adding a phantom name — "TS-999" — to some row's models fails here
// with "named in simulatedProfiles but not registered"; removing "IC-7100"
// from its row's models fails with "registered but no simulatedProfiles row
// protects it".
func assertEveryRegisteredModelIsConfined(t *testing.T, claimed []string) {
	t.Helper()

	registered := wiring.SupportedModels()
	if len(registered) == 0 {
		t.Fatal("wiring.SupportedModels() is empty — this completeness check would pass vacuously")
	}

	seen := make(map[string]bool, len(claimed))
	for _, m := range claimed {
		if seen[m] {
			t.Errorf("simulatedProfiles names model %q on more than one row — each registered model is confined by exactly one row's clause, and a duplicate hides a row that protects nothing", m)
			continue
		}
		seen[m] = true
	}

	for _, m := range registered {
		if !seen[m] {
			t.Errorf("model %q is registered in internal/wiring but no simulatedProfiles row names it — its driver's simulated-profile token is confined by nothing, so a stray non-test reference to it would be invisible to this guard", m)
		}
	}
	for m := range seen {
		if !slices.Contains(registered, m) {
			t.Errorf("simulatedProfiles names model %q, which internal/wiring does not register — a row here must protect a model a user can actually select, or the confinement it claims is about nothing", m)
		}
	}
}

// assertNoTS480RowUntilTheRowRegisters is the TS-480's absence stated as its
// OWN named assertion rather than as an exception folded into the union
// equality above (plan decision P3; Codex re-review MED-1a). Kept separate so
// that a premature ts480 row is caught by THIS check, with this reason,
// rather than as a side effect of a check about registration.
//
// core/driver/ts480 is BUILT and NOT REGISTERED at this milestone's close:
// there is no realDrivers row, no fakeDrivers row, and therefore no
// fake-wiring call site for the confinement guard's pairing clause to find. A
// ts480 row added to simulatedProfiles before the registration commit would
// fail that guard's non-vacuity clause with a message about a broken AST walk
// — a true failure with a misleading reason. This assertion says the real one.
//
// THE DAY THE ROW REGISTERS, THIS ASSERTION IS WHAT CHANGES, and that is the
// design: registering the TS-480 means adding its simulatedProfiles row (edit
// 8 of task 19's ten-edit list) and deleting this call in the same commit,
// which is a visible, reviewable pair of edits rather than a silent one. The
// SupportedModels() guard below is what stops the two drifting apart in the
// meantime — once the model is registered, leaving this assertion in place is
// itself a failure.
//
// RED PROOF (recorded, not re-run by CI): adding
// {"ts480", "Simulated", "fakets480.New", "internal/fakets480", []string{"TS-480"}}
// to simulatedProfiles fails HERE by name. The union equality would also fail
// it, as a model internal/wiring does not register, but with a message about
// registration rather than about this plan's deliberate omission.
func assertNoTS480RowUntilTheRowRegisters(t *testing.T, pkgs, models []string) {
	t.Helper()

	if slices.Contains(wiring.SupportedModels(), "TS-480") {
		t.Fatal(`wiring.SupportedModels() names "TS-480" — the row has been registered, so this assertion's premise is gone: give the TS-480 its simulatedProfiles row (task 19's edit 8) and delete this call in the same commit`)
	}
	for _, pkg := range pkgs {
		if pkg == "ts480" {
			t.Error(`simulatedProfiles has a "ts480" row while internal/wiring does not register the model (plan decision P3): core/driver/ts480 is built and unregistered, so ts480.Simulated has no fake-wiring call site to be paired against and this guard's non-vacuity clause would fail the row for a reason that has nothing to do with confinement`)
		}
	}
	for _, m := range models {
		if m == "TS-480" {
			t.Error(`simulatedProfiles names the model "TS-480" while internal/wiring does not register it — the models column lists what a row's confinement PROTECTS, and there is nothing to protect until the row exists`)
		}
	}
}
