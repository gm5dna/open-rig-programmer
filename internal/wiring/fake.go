// SPDX-License-Identifier: GPL-3.0-or-later

package wiring

import (
	"context"
	"fmt"
	"io"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft2000"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft450d"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft710"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft891"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft950"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft991a"
	"github.com/gm5dna/open-rig-programmer/core/driver/ftdx10"
	"github.com/gm5dna/open-rig-programmer/core/driver/ftdx101"
	"github.com/gm5dna/open-rig-programmer/core/driver/ftdx1200"
	"github.com/gm5dna/open-rig-programmer/core/driver/ftdx3000"
	"github.com/gm5dna/open-rig-programmer/core/driver/ftdx5000"
	"github.com/gm5dna/open-rig-programmer/core/driver/ftdx9000"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic705"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic7100"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic7200"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic7300"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic7410"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic7600"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic7610"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic7700"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic7760"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic7800"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic7851"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic905"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic9100"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic9700"
	"github.com/gm5dna/open-rig-programmer/core/driver/icr8600"
	"github.com/gm5dna/open-rig-programmer/core/driver/ts2000"
	"github.com/gm5dna/open-rig-programmer/core/driver/ts570"
	"github.com/gm5dna/open-rig-programmer/core/driver/ts590"
	"github.com/gm5dna/open-rig-programmer/core/driver/ts870s"
	"github.com/gm5dna/open-rig-programmer/core/driver/ts890"
	"github.com/gm5dna/open-rig-programmer/core/driver/ts990"
	"github.com/gm5dna/open-rig-programmer/internal/fakedx10"
	"github.com/gm5dna/open-rig-programmer/internal/fakedx101"
	"github.com/gm5dna/open-rig-programmer/internal/fakeft2000"
	"github.com/gm5dna/open-rig-programmer/internal/fakeft450d"
	"github.com/gm5dna/open-rig-programmer/internal/fakeft891"
	"github.com/gm5dna/open-rig-programmer/internal/fakeft950"
	"github.com/gm5dna/open-rig-programmer/internal/fakeft991a"
	"github.com/gm5dna/open-rig-programmer/internal/fakeftdx1200"
	"github.com/gm5dna/open-rig-programmer/internal/fakeftdx3000"
	"github.com/gm5dna/open-rig-programmer/internal/fakeftdx5000"
	"github.com/gm5dna/open-rig-programmer/internal/fakeftdx9000"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic705"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7100"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7200"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7300"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7300mk2"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7410"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7600"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7610"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7700"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7760"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7800"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7851"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic905"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic9100"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic9700"
	"github.com/gm5dna/open-rig-programmer/internal/fakeicr8600"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
	"github.com/gm5dna/open-rig-programmer/internal/fakets2000"
	"github.com/gm5dna/open-rig-programmer/internal/fakets570"
	"github.com/gm5dna/open-rig-programmer/internal/fakets590"
	"github.com/gm5dna/open-rig-programmer/internal/fakets870s"
	"github.com/gm5dna/open-rig-programmer/internal/fakets890"
	"github.com/gm5dna/open-rig-programmer/internal/fakets990"
)

// FakeSessionOpts holds extra fakeradio.Option values applied, on top of
// the always-empty production default, to the FT-710's fake rig on every
// OpenFakeSessionFor call in this process. It exists solely as a minimal
// test-only seam (task-12 brief §3, moved verbatim from
// cmd/rigprog/wiring.go's openFakeSessionOpts by task-15's extraction):
// an in-process inventory-mismatch case needs a caller to run against a
// NON-default factory image (e.g. fakeradio.ImageUS) through the EXACT
// SAME code path a real "--fake"/demo invocation uses, rather than
// hand-building a session that bypasses this package's constructor
// entirely.
//
// FT-710-SPECIFIC BY DESIGN, recorded (M9c-5 E5). Its type names
// internal/fakeradio, which is the FT-710's simulator; it is captured by
// the FT-710 entry's own newRadio closure below, not applied by
// OpenFakeSessionFor to whatever rig a model happens to build. A second
// model's fake rig is a different type with different options, and it
// gets its own capture in its own closure — deliberately NOT a shared,
// generic option channel, which could only be typed by emptying it of
// meaning. The seam stays as narrow as the one test need that justifies
// it.
//
// The FTdx10's registration (M9c-6) is that design's first real test, and
// it held: FTdx10FakeSessionOpts below is a SEPARATE variable of a
// different element type ([]fakedx10.Option), read in its own closure. No
// generic option plumbing was needed, and neither model's options can
// reach the other's rig.
//
// THE DIFFERENT-ELEMENT-TYPE PART OF THAT IS NOT UNIVERSAL, and M9d-2 is
// where it stopped being: FTdx101DFakeSessionOpts and
// FTdx101MPFakeSessionOpts (two declarations below) are BOTH
// []fakedx101.Option, because one simulator serves both FTDX101 siblings.
// The separate-variable-per-model rule holds there; the compiler-enforced
// half does not, and tests stand in for it. See FTdx101DFakeSessionOpts'
// own doc comment.
//
// No production flag or GUI control populates this — it does not add a
// second ft710.Simulated reference to any non-test file:
// TestSimulatedProfileTokensConfinement (internal/guards) keeps passing
// unchanged. It is analogous to core/clone's Service.openJournal field,
// which a test in that package overwrites directly for the same reason.
//
// A test that sets it MUST restore the previous value (e.g. via
// t.Cleanup) — this is shared, unsynchronised package state, acceptable
// only because no test using it calls t.Parallel().
var FakeSessionOpts []fakeradio.Option

// FTdx10FakeSessionOpts is the FTdx10's own option source: extra
// fakedx10.Option values applied, on top of the always-empty production
// default, to the FTdx10's fake rig on every OpenFakeSessionFor call in
// this process. It is FakeSessionOpts' FTdx10 counterpart and NOT a
// generalisation of it (M9c-5 E5, realised at M9c-6 task 6) — a separate
// variable, of a different element type, read at CALL time inside the
// FTdx10 entry's own newRadio closure below.
//
// Its element type is the point. internal/fakedx10 simulates the FTdx10;
// its Option is a func(*fakedx10.Radio) and cannot configure an
// *fakeradio.Radio, nor the reverse. Two typed variables therefore make a
// crossed application a COMPILE error, where one shared generically-typed
// option channel would have made it a silent no-op at runtime.
//
// Its users today are a wiring test that opens an FTdx10 fake carrying a
// populated 5 MHz bank (fakedx10.With5xx) through the very code path a
// real "--fake --model FTdx10" invocation uses, and app/uispec_test.go's
// D5c acceptance tests, which reach GetUISpec through the same path with
// a discovered bank present — the discovery-through-wiring
// property no default-image session can express, since the default FTdx10
// image deliberately has no 5xx bank at all.
//
// No production flag or GUI control populates this — it adds no second
// ftdx10.Simulated reference to any non-test file, so
// TestSimulatedProfileTokensConfinement's new ftdx10 row keeps passing.
//
// A test that sets it MUST restore the previous value (e.g. via
// t.Cleanup) — this is shared, unsynchronised package state, acceptable
// only because no test using it calls t.Parallel().
var FTdx10FakeSessionOpts []fakedx10.Option

// FTdx101DFakeSessionOpts is the FTDX101D's own option source: extra
// fakedx101.Option values applied, on top of the always-empty production
// default, to the FTDX101D's fake rig on every OpenFakeSessionFor call in
// this process. It is FakeSessionOpts' and FTdx10FakeSessionOpts' FTdx101D
// counterpart and, like them, NOT a generalisation of anything — a separate
// variable read at CALL time inside the FTdx101D entry's own newRadio
// closure below.
//
// NO OPTION LEAKAGE ACROSS SIBLINGS (spec A6), and this pair of variables
// is where that rule is enforced. A test setting options in THIS variable
// must not steer an FTDX101MP session, and a test setting them in
// FTdx101MPFakeSessionOpts must not steer a D's: the two models share a
// driver package, a dialect config and a simulator type, so nothing about
// their TYPES prevents a crossing the way []fakeradio.Option and
// []fakedx10.Option prevent theirs. Both vars are []fakedx101.Option, and a
// crossed application would COMPILE. Separation here is therefore a
// structural choice enforced by which closure reads which variable, and it
// is pinned by tests
// (TestOpenFakeSessionFor_FTdx101DOptionSourceIsItsOwn and its MP sibling)
// rather than by the compiler — the one place in this file where that is
// true, and the reason those tests exist at all.
//
// No production flag or GUI control populates this — it adds no second
// ftdx101.Simulated reference to any non-test file, so
// TestSimulatedProfileTokensConfinement's two ftdx101 rows keep passing.
//
// A test that sets it MUST restore the previous value (e.g. via
// t.Cleanup) — this is shared, unsynchronised package state, acceptable
// only because no test using it calls t.Parallel().
var FTdx101DFakeSessionOpts []fakedx101.Option

// FTdx101MPFakeSessionOpts is the FTDX101MP's own option source, on exactly
// the same terms as FTdx101DFakeSessionOpts — see that variable's doc
// comment for the no-leakage rule the pair exists to carry, and for why
// this one is a SECOND variable of the SAME element type rather than a
// shared one keyed by model.
//
// Restated because it is the whole point: setting options here must not
// steer an FTDX101D session. The two radios differ on the wire in the ID
// answer alone, so an option that leaked from one sibling to the other
// would produce a session that looked entirely well-formed and was
// answering from the wrong rig's inventory.
var FTdx101MPFakeSessionOpts []fakedx101.Option

// IC705FakeSessionOpts is the IC-705's own option source, on the same
// terms as every other model's own variable above — see
// FTdx10FakeSessionOpts' doc comment for the shape this restates:
// internal/fakeic705 simulates the IC-705 specifically, its Option is a
// func(*fakeic705.Radio), read at CALL time inside the IC705Model entry's
// own newRadio closure below, and never captured at package init.
//
// No production flag or GUI control populates this — it adds no second
// ic705.Simulated reference to any non-test file, so
// TestSimulatedProfileTokensConfinement's new ic705 row keeps passing.
//
// A test that sets it MUST restore the previous value (e.g. via
// t.Cleanup) — this is shared, unsynchronised package state, acceptable
// only because no test using it calls t.Parallel().
var IC705FakeSessionOpts []fakeic705.Option

// IC905FakeSessionOpts is the IC-905's own option source, on the same
// terms as every other model's own variable above — see
// FTdx10FakeSessionOpts' doc comment for the shape this restates:
// internal/fakeic905 simulates the IC-905 specifically, its Option is a
// func(*fakeic905.Radio) (options.go's WithRecord, WithEmpty and the
// rest), read at CALL time inside the IC905Model entry's own newRadio
// closure below, and never captured at package init.
//
// LEFT AT ITS NIL ZERO VALUE, THE SESSION STARTS EMPTY. internal/fakeic905's
// own New builds ten occupied channels in group 0 by default (image.go's
// defaultImage), but every one of them holds an ALL-ZERO invented record
// that core/civ/ic905/profile.go's filter refuses to decode (byte 0x00 at
// offset 7 is not a value that filter defines) — so the IC905Model row
// below empties all ten with WithEmpty BEFORE appending this variable's
// options, and an unset IC905FakeSessionOpts therefore leaves NO occupied
// channel, not the ten-channel default this comment used to describe. A
// test that wants a populated channel reaches it through this variable
// exactly as every other model's own seam works — WithRecord after the
// row's own WithEmpty calls still wins, per options.go's "later one wins"
// rule.
//
// No production flag or GUI control populates this — it adds no second
// ic905.Simulated reference to any non-test file, so
// TestSimulatedProfileTokensConfinement's new ic905 row keeps passing.
//
// A test that sets it MUST restore the previous value (e.g. via
// t.Cleanup) — this is shared, unsynchronised package state, acceptable
// only because no test using it calls t.Parallel().
var IC905FakeSessionOpts []fakeic905.Option

// IC7851FakeSessionOpts and IC7850FakeSessionOpts are the IC-7851's and
// the IC-7850's own option sources, on the same terms as every other
// model's own variable above — see FTdx10FakeSessionOpts' doc comment for
// the shape this restates.
//
// BOTH ARE []fakeic7851.Option, because ONE simulator serves both rows,
// and that is the FTdx101D/FTdx101MP hazard again rather than a new one:
// unlike every other pairing in this file a crossing here would COMPILE.
// What keeps the two seams apart is that each is read at CALL time inside
// its OWN row's newRadio closure below, never captured at package init
// and never shared — exactly as FTdx101DFakeSessionOpts and
// FTdx101MPFakeSessionOpts are, and pinned the same way
// (TestOpenFakeSessionFor_IC7851OptionSourceIsItsOwn and its IC-7850
// sibling).
//
// LEFT AT ITS NIL ZERO VALUE THE DEMO RADIO IS EMPTY, and that is
// internal/fakeic7851's own default rather than anything this file
// arranges: its defaultConfig seeds no channel at all, so a
// `--fake --model IC-7851` session opens UNFINGERPRINTED (every probed
// slot answers FA) and reads each of its 101 slots as an empty channel —
// which is a decodable answer, not a failure, so
// TestOpenFakeSessionFor_EveryRegisteredModel_ReadsEveryDefaultSlot
// passes without this row seeding anything. Contrast the IC905Model row
// below, whose fake's own default image had to be emptied here.
//
// No production flag or GUI control populates either — they add no second
// ic7851.WithSimulatedProfile reference to any non-test file, so
// TestSimulatedProfileTokensConfinement's new ic7851 row keeps passing.
//
// A test that sets one MUST restore the previous value (e.g. via
// t.Cleanup) — this is shared, unsynchronised package state, acceptable
// only because no test using it calls t.Parallel().
var IC7851FakeSessionOpts []fakeic7851.Option

// IC7850FakeSessionOpts is the IC-7850's own option source, on exactly
// the same terms as IC7851FakeSessionOpts — see that variable's doc
// comment, and note in particular that the two are the same TYPE and so
// mutually assignable: only the two closures below keep them apart.
var IC7850FakeSessionOpts []fakeic7851.Option

// FT891FakeSessionOpts is the FT-891's own option source: extra
// fakeft891.Option values applied, on top of the always-empty production
// default, to the FT-891's fake rig on every OpenFakeSessionFor call in this
// process. It is FakeSessionOpts' FT-891 counterpart and, like every
// variable above, NOT a generalisation of anything — a separate variable, of
// a different element type, read at CALL time inside the FT891 entry's own
// newRadio closure below.
//
// NO SHARED-TYPE HAZARD, as for the single-model Icom variables above and
// unlike the FTdx101 and IC-7851 pairs': internal/fakeft891 simulates the
// FT-891 specifically and its Option is a func(*fakeft891.Radio), so a
// closure reading another model's variable is a COMPILE ERROR rather than a
// silent crossing. This row therefore needs no non-interference test of the
// FTdx101 pair's kind — the type system already carries that proof — and the
// test it does have (TestOpenFakeSessionFor_FT891OptionSourceIsItsOwn) pins
// the OTHER half: that this variable actually reaches this model's rig.
//
// LEFT AT ITS NIL ZERO VALUE THE DEMO FT-891 SHIPS TWO OCCUPIED MEMORY
// CHANNELS AND NINE POPULATED PMS PAIRS, and NO 5 MHz or emergency channel
// at all — internal/fakeft891's own DefaultImage, constrained that way on
// purpose (plan decision P13). The ">= 1 populated MEM channel" half is what
// keeps this package's read-every-default-slot fleet pin non-vacuous for
// this row; the "no 5xx/EMG" half is what keeps a plain `--fake --model
// FT-891` session's discovery walk finding nothing, so the discovered banks
// are a thing a test asks for rather than a thing the demo radio happens to
// have.
//
// ITS USERS ARE THE TESTS THAT NEED WHAT THE DEFAULT IMAGE DELIBERATELY
// WITHHOLDS: fakeft891.With5MHz() / fakeft891.WithEMG() for the discovered
// banks (this package's own SynthesiseDiscoveredBanks agreement test and
// app/uispec_test.go's FT-891 discovered-bank test), and
// fakeft891.WithEXUnavailable / WithEXSetting for the settings legs — each
// through the very code path a real `--fake --model FT-891` invocation uses.
//
// No production flag or GUI control populates this — it adds no second
// ft891.Simulated reference to any non-test file, so
// TestSimulatedProfileTokensConfinement's new ft891 row keeps passing.
//
// A test that sets it MUST restore the previous value (e.g. via t.Cleanup) —
// this is shared, unsynchronised package state, acceptable only because no
// test using it calls t.Parallel().
var FT891FakeSessionOpts []fakeft891.Option

// FT991AFakeSessionOpts is the FT-991A's own option source: extra
// fakeft991a.Option values applied, on top of the always-empty production
// default, to the FT-991A's fake rig on every OpenFakeSessionFor call in this
// process. It is FakeSessionOpts' FT-991A counterpart and, like every
// variable above, NOT a generalisation of anything — a separate variable, of
// a different element type, read at CALL time inside the FT991A entry's own
// newRadio closure below.
//
// NO SHARED-TYPE HAZARD, as for the FT-891's and the single-model Icom
// variables above and unlike the FTdx101 and IC-7851 pairs':
// internal/fakeft991a simulates the FT-991A specifically and its Option is a
// func(*fakeft991a.Radio), so a closure reading another model's variable is a
// COMPILE ERROR rather than a silent crossing. This row therefore needs no
// non-interference test of the FTdx101 pair's kind — the type system already
// carries that proof — and the test it does have
// (TestOpenFakeSessionFor_FT991AOptionSourceIsItsOwn) pins the OTHER half:
// that this variable actually reaches this model's rig.
//
// LEFT AT ITS NIL ZERO VALUE THE DEMO FT-991A SHIPS TWO OCCUPIED MEMORY
// CHANNELS AND TWO POPULATED PMS PAIRS — the first (100/101) and the last
// (116/117) — and nothing else (internal/fakeft991a's own DefaultImage,
// constrained that way on purpose, plan decision P14). BOTH halves of that
// are load-bearing here: the memory channel keeps this package's
// read-every-default-slot fleet pin non-vacuous for this row, and the PMS
// pairs keep it non-vacuous on the bank where this radio is unlike every
// registered sibling — decimal slot numbers where theirs are "P1L".."P9U".
// There is NO discovered bank to keep empty, unlike the FT-891's row: this
// radio discovers nothing at Open (matrix §3.4).
//
// ITS USERS ARE THE TESTS THAT NEED WHAT THE DEFAULT IMAGE DELIBERATELY
// WITHHOLDS: fakeft991a.WithDCSChannels() for the five-state P8 legs,
// fakeft991a.WithSlot for a particular record, and
// fakeft991a.WithEXUnavailable / WithEXSetting for the settings legs — each
// through the very code path a real `--fake --model FT-991A` invocation uses.
//
// No production flag or GUI control populates this — it adds no second
// ft991a.Simulated reference to any non-test file, so
// TestSimulatedProfileTokensConfinement's new ft991a row keeps passing.
//
// A test that sets it MUST restore the previous value (e.g. via t.Cleanup) —
// this is shared, unsynchronised package state, acceptable only because no
// test using it calls t.Parallel().
var FT991AFakeSessionOpts []fakeft991a.Option

// TS590SFakeSessionOpts and TS590SGFakeSessionOpts are the TS-590 pair's own
// option sources: extra fakets590.Option values applied, on top of the
// always-empty production default, to each sibling's fake rig on every
// OpenFakeSessionFor call in this process. They are FakeSessionOpts'
// TS-590 counterparts and, like every variable above, NOT a generalisation
// of anything — separate variables, read at CALL time inside each row's own
// newRadio closure below.
//
// A SHARED-TYPE HAZARD, like the FTdx101 and IC-7851 pairs' and unlike the
// single-model variables above: internal/fakets590 simulates BOTH siblings
// and its Option is a func(*fakets590.Radio), so a closure reading the other
// row's variable would COMPILE and quietly seed one demo radio from the
// other's seam. Reading each variable inside its own row's closure is the
// only thing keeping the two apart, exactly as it is for the FTdx101 pair,
// and TestOpenFakeSessionFor_TS590SOptionSourceIsItsOwn and
// TestOpenFakeSessionFor_TS590SGOptionSourceIsItsOwn pin that each variable
// reaches its own rig and NOT the sibling's.
//
// LEFT AT THEIR NIL ZERO VALUES BOTH DEMO RADIOS SHIP THE SAME FIVE
// POPULATED RECORDS OVER FOUR CHANNELS — memory channels 000, 001 and 002,
// one record each, and BOTH HALVES of section-defined channel 100 — which is
// internal/fakets590's own DefaultImage, every byte of it a printed
// constant, a printed example or the eight-space name (plan decision P19,
// that package's PROVENANCE.md). Records and channels are counted separately
// because 100's two halves are two records at one channel number. The
// ">= 1 populated MEM channel" half is what keeps this package's
// read-every-default-slot fleet pin non-vacuous for both rows.
//
// NO IMAGE EXISTS FOR THE SG's 110-119 AT ALL, and that is decision row 6
// rather than an omission — at THIS layer. The CODEC ADMITS those ten slots
// deliberately (core/kw/ts590/layout.go: the book prints 110-119 for the SG
// row, so an "MR0115;" answer must parse rather than be refused); what an
// extension channel IS is never explained (A11), so the DRIVER publishes no
// bank containing them and the fake therefore has no slot ID for an image to
// represent. Codec domain and driver publication are two different
// questions, and it is the second that closes here.
//
// No production flag or GUI control populates either — they add no second
// ts590.Simulated reference to any non-test file, so
// TestSimulatedProfileTokensConfinement's new ts590 row keeps passing.
//
// A test that sets one MUST restore the previous value (e.g. via
// t.Cleanup) — this is shared, unsynchronised package state, acceptable
// only because no test using it calls t.Parallel().
var (
	TS590SFakeSessionOpts  []fakets590.Option
	TS590SGFakeSessionOpts []fakets590.Option
)

// TS890SFakeSessionOpts and TS990SFakeSessionOpts are the TS-890S's and
// TS-990S's own option sources: extra fakets890.Option / fakets990.Option
// values applied, on top of the always-empty production default, to that
// radio's fake rig on every OpenFakeSessionFor call in this process. They are
// FakeSessionOpts' Tier-6-second-pair counterparts and, like every variable
// above, NOT a generalisation of anything — separate variables, read at CALL
// time inside each row's own newRadio closure below.
//
// THE SHARED-TYPE HAZARD THE 590 PAIR CARRIES IS ABSENT HERE, and saying so
// is worth more than repeating the warning: internal/fakets890 and
// internal/fakets990 are two packages with two Option types, so a closure
// reading the other row's variable does not compile. What CAN still go wrong
// is a copy-paste that builds the 890S TWICE — both rows calling
// fakets890.New with the 890's variable — which compiles perfectly and gives
// a "TS-990S" session answering "024". TestOpenFakeSessionFor_TS890SOptionSourceIsItsOwn
// and its 990S mirror read each variable through OpenFakeSessionFor and
// assert the SIBLING's session does not see it, which catches that; the
// identity leg of TestOpenFakeSessionFor_EveryRegisteredModel catches it
// again from the CAT ID.
//
// LEFT AT THEIR NIL ZERO VALUES the two demo radios ship DIFFERENT images,
// and neither is the other's. internal/fakets890's DefaultImage populates
// memory channels 000-003 and puts a BLANK record carrying a residual name in
// 004 (that package's A21 fixture); internal/fakets990's populates 000-003 and
// has no blank-with-residue slot at all. Every byte of both is a printed
// constant, a printed example or a printed character (plan decision P19, each
// package's PROVENANCE.md), and the composition of each RECORD is that
// package's own A22 entry. The populated channels are what keep this
// package's read-every-default-slot fleet pin non-vacuous for both rows.
//
// NO IMAGE EXISTS FOR SLOTS 100-119 ON EITHER ROW, and that is plan decision
// P11 rather than an omission: both radios have those channels, neither book
// prints what selects a section channel's start or end frequency, so the
// DRIVER publishes no bank containing them and the fake has no slot ID for an
// image to represent.
//
// No production flag or GUI control populates either — they add no second
// ts890.Simulated or ts990.Simulated reference to any non-test file, so
// TestSimulatedProfileTokensConfinement's two new rows keep passing.
//
// A test that sets one MUST restore the previous value (e.g. via t.Cleanup) —
// this is shared, unsynchronised package state, acceptable only because no
// test using it calls t.Parallel().
var (
	TS890SFakeSessionOpts []fakets890.Option
	TS990SFakeSessionOpts []fakets990.Option
)

// TS2000FakeSessionOpts is the TS-2000 family's own option source: extra
// fakets2000.Option values applied, on top of the always-empty production
// default, to the shared fake rig on every OpenFakeSessionFor call. ONE
// variable for THREE registry rows — internal/fakets2000's New takes no row
// argument at all (WithModelName is the option form, per fake.md), so there
// is no per-row variable to keep separate the way the 590 pair's are.
//
// A test that sets this MUST restore the previous value (e.g. via
// t.Cleanup) — shared, unsynchronised package state, acceptable only
// because no test using it calls t.Parallel().
var TS2000FakeSessionOpts []fakets2000.Option

// TS570DFakeSessionOpts is the TS-570 family's own option source: extra
// fakets570.Option values applied, on top of the always-empty production
// default, to the shared fake rig on every OpenFakeSessionFor call. ONE
// variable for THREE registry rows — internal/fakets570's New takes no row
// argument (WithModelName is the option form, per fake.md), so there is no
// per-row variable to keep separate the way the 590 pair's are.
//
// A test that sets this MUST restore the previous value (e.g. via
// t.Cleanup) — shared, unsynchronised package state, acceptable only
// because no test using it calls t.Parallel().
var TS570DFakeSessionOpts []fakets570.Option

// TS870SFakeSessionOpts is the TS-870S's own option source: extra
// fakets870s.Option values applied, on top of the always-empty production
// default, to its fake rig on every OpenFakeSessionFor call — one row, one
// package, one simulator, no sibling to share either with.
//
// A test that sets this MUST restore the previous value (e.g. via
// t.Cleanup) — shared, unsynchronised package state, acceptable only
// because no test using it calls t.Parallel().
var TS870SFakeSessionOpts []fakets870s.Option

// FT2000FakeSessionOpts is the FT-2000 family's own option source: extra
// fakeft2000.Option values applied, on top of the always-empty production
// default, to the shared fake rig on every OpenFakeSessionFor call. ONE
// variable for TWO registry rows — internal/fakeft2000's New takes no row
// argument (WithModelName is the option form, per fake.md, a deliberate
// deviation this package's own doc.go records), so there is no per-row
// variable to keep separate.
//
// A test that sets this MUST restore the previous value (e.g. via
// t.Cleanup) — shared, unsynchronised package state, acceptable only
// because no test using it calls t.Parallel().
var FT2000FakeSessionOpts []fakeft2000.Option

// FTdx5000FakeSessionOpts is the FTdx5000's own option source, on the same
// terms as every single-row model's above — one row, one package, one
// simulator.
var FTdx5000FakeSessionOpts []fakeftdx5000.Option

// FTdx9000FakeSessionOpts is the FTdx9000's own option source, on the same
// terms as every single-row model's above — one row, one package, one
// simulator.
var FTdx9000FakeSessionOpts []fakeftdx9000.Option

// FT950FakeSessionOpts is the FT-950's own option source, on the same
// terms as every single-row model's above — one row, one package, one
// simulator.
var FT950FakeSessionOpts []fakeft950.Option

// FTdx3000FakeSessionOpts is the FTDX3000's own option source, on the
// same terms as every single-row model's above — one row, one package,
// one simulator.
var FTdx3000FakeSessionOpts []fakeftdx3000.Option

// FTdx1200FakeSessionOpts is the FTDX1200's own option source, on the
// same terms as every single-row model's above — one row, one package,
// one simulator.
var FTdx1200FakeSessionOpts []fakeftdx1200.Option

// FT450DFakeSessionOpts is the FT-450D's own option source, on the same
// terms as every single-row model's above — one row, one package, one
// simulator.
var FT450DFakeSessionOpts []fakeft450d.Option

// IC7800FakeSessionOpts is the IC-7800's own option source, on the same
// terms as every other model's own variable above: internal/fakeic7800
// simulates the IC-7800 specifically, its Option is a
// func(*fakeic7800.Radio), read at CALL time inside the IC7800Model entry's
// own newRadio closure below, and never captured at package init.
//
// LEFT AT ITS NIL ZERO VALUE THE DEMO RADIO IS EMPTY: internal/fakeic7800's
// own default seeds no channel (its slots map starts empty), the same
// footing as the IC-7610's and every other single-row Icom fake here.
//
// No production flag or GUI control populates this — it adds no second
// ic7800.Simulated reference to any non-test file, so
// TestSimulatedProfileTokensConfinement's new ic7800 row keeps passing.
//
// A test that sets it MUST restore the previous value (e.g. via
// t.Cleanup) — this is shared, unsynchronised package state, acceptable
// only because no test using it calls t.Parallel().
var IC7800FakeSessionOpts []fakeic7800.Option

// IC7600FakeSessionOpts is the IC-7600's own option source, on the same
// terms as IC7800FakeSessionOpts above. Left at its nil zero value the
// demo radio is empty, the same footing as every other single-row Icom
// fake here.
var IC7600FakeSessionOpts []fakeic7600.Option

// IC7410FakeSessionOpts is the IC-7410's own option source, on the same
// terms as IC7800FakeSessionOpts above. Left at its nil zero value the
// demo radio is empty.
var IC7410FakeSessionOpts []fakeic7410.Option

// IC7700FakeSessionOpts is the IC-7700's own option source, on the same
// terms as IC7800FakeSessionOpts above. Left at its nil zero value the
// demo radio is empty.
var IC7700FakeSessionOpts []fakeic7700.Option

// IC9100FakeSessionOpts is the IC-9100's own option source, on the same
// terms as IC7800FakeSessionOpts above. Left at its nil zero value the
// demo radio is empty.
var IC9100FakeSessionOpts []fakeic9100.Option

// IC7200FakeSessionOpts is the IC-7200's own option source, on the same
// terms as IC7800FakeSessionOpts above. Left at its nil zero value the
// demo radio is empty.
var IC7200FakeSessionOpts []fakeic7200.Option

// fakeRadio is everything OpenFakeSessionFor needs from a model's fake
// rig: a port to hand the driver, and a way to shut the rig down
// afterwards. Interface-typed rather than *fakeradio.Radio (M9c-5 E5)
// because internal/fakeradio simulates the FT-710 specifically — a second
// model's simulator is a different type, and a concretely-typed table
// could not hold it at all. The FTdx10's *fakedx10.Radio (M9c-6) is that
// second type, and it needed no change here: it satisfies this interface
// as written, which is what the interface was extracted for. The
// *fakedx101.Radio (M9d-2) is the third and needed no change either — ONE
// simulator type serving TWO registered models, since the FTDX101 siblings
// differ on the wire in the ID answer alone.
//
// Port returns io.ReadWriteCloser, NOT transport.Port, and that is
// forced: Go matches interface methods by EXACT signature, so a
// Port() transport.Port method would be unsatisfiable by
// *fakeradio.Radio, whose Port() returns io.ReadWriteCloser. Nothing is
// lost at the call site — transport.Port IS io.ReadWriteCloser
// (core/transport/port.go), so the returned value is assignable to the
// parameter driver.Driver.Open declares.
type fakeRadio interface {
	Port() io.ReadWriteCloser
	Close() error
}

// The compile-time proofs that each registered model's simulator satisfies
// the interface its table entry is typed by — so a change to either side is
// a build failure here rather than a surprise at the entry.
var (
	_ fakeRadio = (*fakeradio.Radio)(nil)
	_ fakeRadio = (*fakedx10.Radio)(nil)
	// ONE assertion for the FTdx101 pair: fakedx101.NewD and
	// fakedx101.NewMP both return *fakedx101.Radio, so there is one type
	// to prove and two table rows that depend on the proof.
	_ fakeRadio = (*fakedx101.Radio)(nil)
	// The IC-7610's, the first Icom simulator this table holds — via
	// ic7610FakeAdapter, not *fakeic7610.Radio directly. See that
	// adapter's own doc comment for why.
	_ fakeRadio = ic7610FakeAdapter{}
	// The IC-7300's and IC-7300MK2's, the second Icom pair (Wave 4 task
	// R3) — DIRECTLY, unlike the IC-7610's, and NO ADAPTER IS NEEDED FOR
	// EITHER: internal/fakeic7300's and internal/fakeic7300mk2's own
	// Port() methods are both already declared to return
	// io.ReadWriteCloser (checked against each package's source before
	// this registration, per the task brief), so *fakeic7300.Radio and
	// *fakeic7300mk2.Radio satisfy fakeRadio as written, exactly as
	// *fakeradio.Radio, *fakedx10.Radio and *fakedx101.Radio do above.
	_ fakeRadio = (*fakeic7300.Radio)(nil)
	_ fakeRadio = (*fakeic7300mk2.Radio)(nil)
	// The IC-705's, the third Icom simulator this table holds (Wave 4
	// task R4) — DIRECTLY, on the same footing as the IC-7300 pair's:
	// internal/fakeic705's own Port() method is already declared to
	// return io.ReadWriteCloser (checked against source before this
	// registration, per the task brief), so *fakeic705.Radio satisfies
	// fakeRadio as written, with no adapter needed.
	_ fakeRadio = (*fakeic705.Radio)(nil)
	// The IC-9700's, the fourth Icom simulator this table holds (Wave 4
	// task R5) — DIRECTLY, on the same footing as the IC-705's above:
	// internal/fakeic9700's own Port() method is already declared to
	// return io.ReadWriteCloser (checked against source before this
	// registration), so *fakeic9700.Radio satisfies fakeRadio as written,
	// with no adapter needed.
	_ fakeRadio = (*fakeic9700.Radio)(nil)
	// The IC-905's, the fifth and LAST Icom simulator this table holds
	// (Wave 4 task R6) — DIRECTLY, on the same footing as the IC-705's
	// and IC-9700's above: internal/fakeic905's own Port() method is
	// already declared to return io.ReadWriteCloser (checked against
	// source before this registration), so *fakeic905.Radio satisfies
	// fakeRadio as written, with no adapter needed.
	_ fakeRadio = (*fakeic905.Radio)(nil)
	// The IC-7851/IC-7850's, the additions tier's first (Tier 4b) — via
	// ic7851FakeAdapter, like the IC-7610's and unlike the four Icom
	// simulators between them, because internal/fakeic7851's Port()
	// returns net.Conn. ONE assertion for the PAIR: fakeic7851.New is a
	// single constructor serving both rows, so there is one type to
	// prove and two table rows that depend on the proof — the
	// *fakedx101.Radio assertion's shape, over the IC-7610's adapter.
	_ fakeRadio = ic7851FakeAdapter{}
	// The IC-7760's, the additions tier's second (Tier 4b) — via
	// ic7760FakeAdapter, like the IC-7610's and the IC-7851 pair's and
	// unlike the four Icom simulators between them, because
	// internal/fakeic7760's Port() returns net.Conn (checked against that
	// package's source before this registration, per the task brief).
	_ fakeRadio = ic7760FakeAdapter{}
	// The IC-7100's, the additions tier's third (Tier 4b) — DIRECTLY, and
	// so NO FOURTH ADAPTER IS NEEDED: internal/fakeic7100's own Port()
	// method is already declared to return io.ReadWriteCloser
	// (internal/fakeic7100/fake.go, checked against source before this
	// registration, per the task brief), so *fakeic7100.Radio satisfies
	// fakeRadio as written. It is the IC-705's, IC-9700's and IC-905's
	// case rather than the two adapters' immediately above — the split in
	// this table runs by which package the simulator was written against,
	// not by which tier registered it.
	_ fakeRadio = (*fakeic7100.Radio)(nil)
	// The IC-R8600's, the additions tier's fourth and last (Tier 4b) —
	// DIRECTLY again, and so STILL no fourth adapter:
	// internal/fakeicr8600's own Port() method is already declared to
	// return io.ReadWriteCloser (internal/fakeicr8600/fakeicr8600.go:100,
	// checked against source before this registration, per the task
	// brief), so *fakeicr8600.Radio satisfies fakeRadio as written. The
	// IC-705's, IC-9700's, IC-905's and IC-7100's case, not the two
	// adapters' above.
	_ fakeRadio = (*fakeicr8600.Radio)(nil)
	// The FT-891's (Tier 1) — DIRECTLY, and so still no fourth adapter:
	// internal/fakeft891's own Port() method is already declared to return
	// io.ReadWriteCloser (internal/fakeft891/fakeft891.go:89, checked
	// against source before this registration, per the task brief), so
	// *fakeft891.Radio satisfies fakeRadio as written. It is the five Yaesu
	// simulators' case as much as the IC-7100's and IC-R8600's: the split
	// in this table runs by which package the simulator was written
	// against, not by maker and not by which tier registered it.
	_ fakeRadio = (*fakeft891.Radio)(nil)
	// The FT-991A's (Tier 1's second) — DIRECTLY, and so still no fourth
	// adapter: internal/fakeft991a's own Port() method is already declared
	// to return io.ReadWriteCloser (internal/fakeft991a/fakeft991a.go:93,
	// checked against source before this registration, per the task
	// brief), so *fakeft991a.Radio satisfies fakeRadio as written. It is
	// the FT-891's case exactly, and the five Yaesu simulators' between
	// them: the split in this table runs by which package the simulator
	// was written against, not by maker and not by which tier registered
	// it.
	_ fakeRadio = (*fakeft991a.Radio)(nil)
	// The TS-590 pair's (Tier 6) — DIRECTLY, and so still no fourth adapter:
	// internal/fakets590's own Port() method is already declared to return
	// io.ReadWriteCloser (internal/fakets590/fakets590.go, checked against
	// source before this registration, per the task brief), so
	// *fakets590.Radio satisfies fakeRadio as written. ONE assertion for the
	// PAIR, the *fakedx101.Radio's shape: fakets590.New is a single
	// constructor serving both rows through a REQUIRED row argument, so there
	// is one type to prove and two table rows that depend on the proof.
	_ fakeRadio = (*fakets590.Radio)(nil)
	// Tier 6's SECOND pair — DIRECTLY again, and TWO assertions rather
	// than the 590 pair's one: internal/fakets890 and internal/fakets990
	// are two packages with two Radio types (plan decision P1), each
	// declaring Port() io.ReadWriteCloser already (checked against source
	// before this registration, per the task brief), so both satisfy
	// fakeRadio as written and neither needs an adapter.
	_ fakeRadio = (*fakets890.Radio)(nil)
	_ fakeRadio = (*fakets990.Radio)(nil)
	// v1.7.0 Kenwood/Yaesu wave, first row: fakets2000's Port() already
	// returns io.ReadWriteCloser, so no adapter is needed.
	_ fakeRadio = (*fakets2000.Radio)(nil)
	// v1.7.0 Kenwood/Yaesu wave, fourth row: internal/fakets570's Port()
	// returns net.Conn, on the ic7610 footing, not the 590/890/990/2000
	// rows' direct one.
	_ fakeRadio = ts570FakeAdapter{}
	// v1.7.0 Kenwood/Yaesu wave, seventh row: fakets870s's Port() already
	// returns io.ReadWriteCloser, so no adapter is needed.
	_ fakeRadio = (*fakets870s.Radio)(nil)
	// v1.7.0 Kenwood/Yaesu wave, eighth row: fakeft2000's Port() already
	// returns io.ReadWriteCloser, so no adapter is needed.
	_ fakeRadio = (*fakeft2000.Radio)(nil)
	// v1.7.0 Kenwood/Yaesu wave, tenth row: fakeftdx5000's Port() already
	// returns io.ReadWriteCloser, so no adapter is needed.
	_ fakeRadio = (*fakeftdx5000.Radio)(nil)
	// v1.7.0 Kenwood/Yaesu wave, eleventh row: fakeftdx9000's Port() already
	// returns io.ReadWriteCloser, so no adapter is needed.
	_ fakeRadio = (*fakeftdx9000.Radio)(nil)
	// v1.7.0 Kenwood/Yaesu wave, twelfth and last row: fakeft950's Port()
	// already returns io.ReadWriteCloser, so no adapter is needed.
	_ fakeRadio = (*fakeft950.Radio)(nil)
	// v1.8.0 Yaesu trio, first row: fakeftdx3000's Port() already returns
	// io.ReadWriteCloser, so no adapter is needed.
	_ fakeRadio = (*fakeftdx3000.Radio)(nil)
	// v1.8.0 Yaesu trio, second row: fakeftdx1200's Port() already
	// returns io.ReadWriteCloser, so no adapter is needed.
	_ fakeRadio = (*fakeftdx1200.Radio)(nil)
	// v1.8.0 Yaesu trio, third and last row: fakeft450d's Port() already
	// returns io.ReadWriteCloser, so no adapter is needed.
	_ fakeRadio = (*fakeft450d.Radio)(nil)
	// The IC-7800's (v1.7.0 Icom wave) — via ic7800FakeAdapter, like the
	// IC-7610's and unlike the four directly-satisfying Icom simulators:
	// internal/fakeic7800's Port() returns net.Conn.
	_ fakeRadio = ic7800FakeAdapter{}
	// The IC-7600's, on the same footing — internal/fakeic7600's Port()
	// also returns net.Conn.
	_ fakeRadio = ic7600FakeAdapter{}
	// The IC-7410's, on the same footing — internal/fakeic7410's Port()
	// also returns net.Conn.
	_ fakeRadio = ic7410FakeAdapter{}
	// The IC-7700's, on the same footing — internal/fakeic7700's Port()
	// also returns net.Conn.
	_ fakeRadio = ic7700FakeAdapter{}
	// The IC-9100's, on the same footing — internal/fakeic9100's Port()
	// also returns net.Conn.
	_ fakeRadio = ic9100FakeAdapter{}
	// The IC-7200's, on the same footing — internal/fakeic7200's Port()
	// also returns net.Conn.
	_ fakeRadio = ic7200FakeAdapter{}
)

// ic7610FakeAdapter narrows *fakeic7610.Radio's Port() — which returns
// net.Conn, since internal/fakeic7610 is written against the net package
// directly rather than against this package's fakeRadio seam — to the
// io.ReadWriteCloser fakeRadio itself requires.
//
// A TYPE-IDENTITY GAP, NOT A BEHAVIOUR ONE, and that distinction is why
// this adapter lives here rather than as an edit to internal/fakeic7610
// (out of scope for this registration — "never edit ... fake behaviour").
// Go's interface satisfaction requires each method's result type to match
// EXACTLY: net.Conn already satisfies io.ReadWriteCloser structurally (it
// has Read, Write and Close, and more), but a method declared to return
// net.Conn does not, by itself, implement a method an interface declares
// to return io.ReadWriteCloser. This adapter closes that gap at the
// wiring boundary alone, by re-exposing the SAME net.Conn value through a
// method whose declared result is the interface fakeRadio needs — no
// byte on the wire, no frame, no state transition changes. Close is
// promoted unchanged from the embedded *fakeic7610.Radio.
type ic7610FakeAdapter struct{ *fakeic7610.Radio }

// Port implements fakeRadio, narrowing the embedded Radio's net.Conn to
// io.ReadWriteCloser. See ic7610FakeAdapter's own doc comment.
func (a ic7610FakeAdapter) Port() io.ReadWriteCloser { return a.Radio.Port() }

// ts570FakeAdapter narrows *fakets570.Radio's Port() — net.Conn — to
// io.ReadWriteCloser. See ic7610FakeAdapter's own doc comment for the
// type-identity reasoning; this is the same gap on the TS-570 family's own
// simulator (v1.7.0 Kenwood/Yaesu wave, fourth row).
type ts570FakeAdapter struct{ *fakets570.Radio }

// Port implements fakeRadio. See ts570FakeAdapter's own doc comment.
func (a ts570FakeAdapter) Port() io.ReadWriteCloser { return a.Radio.Port() }

// ic7851FakeAdapter narrows *fakeic7851.Radio's Port() — which returns
// net.Conn, since internal/fakeic7851 is written against the net package
// directly rather than against this package's fakeRadio seam — to the
// io.ReadWriteCloser fakeRadio itself requires.
//
// THE SAME TYPE-IDENTITY GAP ic7610FakeAdapter closes, for the same
// reason and with the same limits: see that adapter's own doc comment.
// It is a second adapter rather than a shared one because each embeds a
// DIFFERENT concrete simulator type and Go has no way to write one for
// both; nothing about either radio's behaviour is involved, and no byte,
// frame or state transition changes. Close is promoted unchanged from the
// embedded *fakeic7851.Radio.
//
// ONE ADAPTER SERVES BOTH ROWS, because one simulator does: the IC-7851
// and IC-7850 fakeDrivers entries both wrap a fakeic7851.New(...) call,
// differing only in the WithModelName each passes.
type ic7851FakeAdapter struct{ *fakeic7851.Radio }

// Port implements fakeRadio, narrowing the embedded Radio's net.Conn to
// io.ReadWriteCloser. See ic7851FakeAdapter's own doc comment.
func (a ic7851FakeAdapter) Port() io.ReadWriteCloser { return a.Radio.Port() }

// ic7760FakeAdapter narrows *fakeic7760.Radio's Port() — which returns
// net.Conn, since internal/fakeic7760 is written against the net package
// directly rather than against this package's fakeRadio seam — to the
// io.ReadWriteCloser fakeRadio itself requires.
//
// THE SAME TYPE-IDENTITY GAP ic7610FakeAdapter closes, for the same
// reason and with the same limits: see that adapter's own doc comment.
// It is a THIRD adapter rather than a shared one because each embeds a
// DIFFERENT concrete simulator type and Go has no way to write one for
// all three; nothing about any radio's behaviour is involved, and no
// byte, frame or state transition changes. Close is promoted unchanged
// from the embedded *fakeic7760.Radio.
type ic7760FakeAdapter struct{ *fakeic7760.Radio }

// Port implements fakeRadio, narrowing the embedded Radio's net.Conn to
// io.ReadWriteCloser. See ic7760FakeAdapter's own doc comment.
func (a ic7760FakeAdapter) Port() io.ReadWriteCloser { return a.Radio.Port() }

// ic7800FakeAdapter narrows *fakeic7800.Radio's Port() — which returns
// net.Conn — to the io.ReadWriteCloser fakeRadio requires. See
// ic7610FakeAdapter's own doc comment for the type-identity gap this
// closes; nothing about the radio's behaviour changes.
type ic7800FakeAdapter struct{ *fakeic7800.Radio }

// Port implements fakeRadio. See ic7800FakeAdapter's own doc comment.
func (a ic7800FakeAdapter) Port() io.ReadWriteCloser { return a.Radio.Port() }

// ic7600FakeAdapter narrows *fakeic7600.Radio's Port() — net.Conn — to
// io.ReadWriteCloser. See ic7800FakeAdapter's own doc comment.
type ic7600FakeAdapter struct{ *fakeic7600.Radio }

// Port implements fakeRadio. See ic7600FakeAdapter's own doc comment.
func (a ic7600FakeAdapter) Port() io.ReadWriteCloser { return a.Radio.Port() }

// ic7410FakeAdapter narrows *fakeic7410.Radio's Port() — net.Conn — to
// io.ReadWriteCloser. See ic7800FakeAdapter's own doc comment.
type ic7410FakeAdapter struct{ *fakeic7410.Radio }

// Port implements fakeRadio. See ic7410FakeAdapter's own doc comment.
func (a ic7410FakeAdapter) Port() io.ReadWriteCloser { return a.Radio.Port() }

// ic7700FakeAdapter narrows *fakeic7700.Radio's Port() — net.Conn — to
// io.ReadWriteCloser. See ic7800FakeAdapter's own doc comment.
type ic7700FakeAdapter struct{ *fakeic7700.Radio }

// Port implements fakeRadio. See ic7700FakeAdapter's own doc comment.
func (a ic7700FakeAdapter) Port() io.ReadWriteCloser { return a.Radio.Port() }

// ic9100FakeAdapter narrows *fakeic9100.Radio's Port() — net.Conn — to
// io.ReadWriteCloser. See ic7800FakeAdapter's own doc comment.
type ic9100FakeAdapter struct{ *fakeic9100.Radio }

// Port implements fakeRadio. See ic9100FakeAdapter's own doc comment.
func (a ic9100FakeAdapter) Port() io.ReadWriteCloser { return a.Radio.Port() }

// ic7200FakeAdapter narrows *fakeic7200.Radio's Port() — net.Conn — to
// io.ReadWriteCloser. See ic7800FakeAdapter's own doc comment.
type ic7200FakeAdapter struct{ *fakeic7200.Radio }

// Port implements fakeRadio. See ic7200FakeAdapter's own doc comment.
func (a ic7200FakeAdapter) Port() io.ReadWriteCloser { return a.Radio.Port() }

// fakeDriverEntry pairs one model's simulated-profile driver constructor
// with the fake-rig constructor OpenFakeSessionFor uses to build a live
// session against it — the fake-table analogue of wiring.go's
// realDrivers, but kept wholly in this file (task 39 brief) because the
// entries themselves are where ft710.Simulated and fakeradio.New are
// referenced, and both tokens must stay confined to this one file
// (TestSimulatedProfileTokensConfinement, internal/guards).
type fakeDriverEntry struct {
	// newDriver builds this model's simulated-profile driver.Driver.
	newDriver func() driver.Driver
	// newRadio builds a fresh fake rig for this model, already carrying
	// whatever options that model's simulator takes — the closure
	// captures its OWN option source (M9c-5 E5), so no option value has
	// to be routed through OpenFakeSessionFor and typed generically to
	// pass a model it does not belong to. Takes no parameters for that
	// reason.
	newRadio func() fakeRadio
}

// fakeDrivers is the model-keyed table OpenFakeSessionFor looks up:
// model name -> (simulated-profile driver constructor, fake-rig
// constructor). This is the ONLY place in this repository — non-test
// file, repo-wide — that references ft710.Simulated, ftdx10.Simulated or
// ftdx101.Simulated
// (task-11 brief §3, pinned per driver by internal/guards'
// TestSimulatedProfileTokensConfinement since task-15's extraction —
// folded from the single-driver guard task-15 originally extended into
// this data-driven guard at Task 58) and the sole place a fakeradio.Radio
// or a fakedx10.Radio is constructed for a live session: the wire pattern
// proven at core/clone/helpers_test.go:194 —
// fakeradio.New() -> ft710.New(ft710.Simulated).Open(ctx, r.Port(), ...).
//
// Each newRadio is deliberately a closure CALLING the simulator's own
// constructor, not that function assigned directly — internal/guards'
// TestSimulatedProfileTokensConfinement's AST walk looks for an actual
// fakeradio.New(...) / fakedx10.New(...) / fakedx101.NewD(...) /
// fakedx101.NewMP(...) CALL expression in this file, not
// merely a reference to the function value, so all four calls must stay
// textually present here. That guard is why the FTdx101 pair contributes
// TWO rows to the guard's own table over ONE driver package and ONE token:
// the check is per fake CONSTRUCTOR, and a registration that wired both
// models to fakedx101.NewD would leave NewMP uncalled and fail there.
//
// Each entry's closure is also where THAT model's
// option source is read — FakeSessionOpts for the FT-710,
// FTdx10FakeSessionOpts for the FTdx10, FTdx101DFakeSessionOpts and
// FTdx101MPFakeSessionOpts for the two FTdx101 siblings, at CALL time,
// never captured at
// package init, so a test that sets one before OpenFakeSessionFor still
// takes effect — and reading them HERE rather than in OpenFakeSessionFor is
// what keeps each seam its own model's (M9c-5 E5; see those variables' own
// doc comments). For the FTdx101 pair that reading is the ONLY thing
// keeping the two seams apart: both are []fakedx101.Option, so unlike every
// other pairing in this table a crossing here would compile.
//
// The FTdx10's driver half is ftdx10.Simulated, and that profile is
// write-SUPPORTED (unlike its RealHardware half, where
// writeTrialsComplete=false leaves nothing writable). That is deliberate
// and it is a claim about the FAKE, not about any radio: this pairing is
// the only place ftdx10.Simulated is legal, and internal/fakedx10 stores
// and returns what the combined MT Set carries — see
// core/driver/ftdx10/doc.go's "The Simulated profile's clarifier is
// Supported, not Inert".
//
// The FTdx101 pair's driver half is ftdx101.Simulated on exactly the same
// terms, and the warning applies twice over: writeTrialsCompleteD and
// writeTrialsCompleteMP are BOTH false, so neither radio has a
// HARDWARE-EVIDENCED write path (the user's own consent can open one
// against a real radio — see OpenRealSessionWith — but that is a decision
// about risk and not evidence, and it never reaches this fake table), and
// the Supported writes these two rows reach are a claim about
// internal/fakedx101 alone. This pairing is the only place
// ftdx101.Simulated is legal.
var fakeDrivers = map[string]fakeDriverEntry{
	DefaultModel: {
		newDriver: func() driver.Driver { return ft710.New(ft710.Simulated) },
		newRadio:  func() fakeRadio { return fakeradio.New(FakeSessionOpts...) },
	},
	FTdx10Model: {
		newDriver: func() driver.Driver { return ftdx10.New(ftdx10.Simulated) },
		newRadio:  func() fakeRadio { return fakedx10.New(FTdx10FakeSessionOpts...) },
	},
	FTdx101DModel: {
		newDriver: func() driver.Driver { return ftdx101.NewD(ftdx101.Simulated) },
		newRadio:  func() fakeRadio { return fakedx101.NewD(FTdx101DFakeSessionOpts...) },
	},
	FTdx101MPModel: {
		newDriver: func() driver.Driver { return ftdx101.NewMP(ftdx101.Simulated) },
		newRadio:  func() fakeRadio { return fakedx101.NewMP(FTdx101MPFakeSessionOpts...) },
	},
	IC7610Model: {
		newDriver: func() driver.Driver { return ic7610.New(ic7610.Simulated) },
		newRadio:  func() fakeRadio { return ic7610FakeAdapter{fakeic7610.New()} },
	},
	IC7300Model: {
		newDriver: func() driver.Driver { return ic7300.New(ic7300.Simulated) },
		newRadio:  func() fakeRadio { return fakeic7300.New() },
	},
	IC7300MK2Model: {
		newDriver: func() driver.Driver { return ic7300.NewMK2(ic7300.Simulated) },
		newRadio:  func() fakeRadio { return fakeic7300mk2.New() },
	},
	IC705Model: {
		newDriver: func() driver.Driver { return ic705.New(ic705.Simulated) },
		newRadio:  func() fakeRadio { return fakeic705.New(IC705FakeSessionOpts...) },
	},
	IC9700Model: {
		newDriver: func() driver.Driver { return ic9700.New(ic9700.Simulated) },
		newRadio:  func() fakeRadio { return fakeic9700.New() },
	},
	IC905Model: {
		newDriver: func() driver.Driver { return ic905.New(ic905.Simulated) },
		// The demo IC-905 starts EMPTY, and that takes explicit work here
		// because fakeic905.New's own default (image.go's defaultImage)
		// is ten occupied channels in group 0, every one holding an
		// ALL-ZERO invented record — the least-invention placeholder
		// image.go's own doc comment explains, chosen with no reference
		// to core/civ/ic905/profile.go's filter. That filter refuses
		// 0x00 (filterNames = {01,02,03}), so those ten records are
		// UNDECODABLE by the IC-905's own driver: "read --fake --model
		// IC-905" failed with "civ: parse error: filter: byte 0x00 at
		// offset 7 is not a value this profile defines" until this row
		// stopped seeding them. fakeic905 is frozen (no behaviour edit),
		// so the fix is at the call site: WithEmpty(0, ch) for every
		// channel the default image occupies (group 0, channels 0-9 —
		// image.go's defaultImageGroup/defaultImageChannels, unexported
		// so restated here as literals) deletes each one before
		// IC905FakeSessionOpts is appended, leaving no occupied channel
		// by default. This mirrors the IC-9700's fake, which seeds
		// nothing at all — an empty
		// demo radio is the honest choice for a model whose only
		// alternative default is content nobody could source from either
		// artefact. IC905FakeSessionOpts is appended AFTER the ten
		// WithEmpty calls, so a test that wants a populated channel still
		// reaches it through the normal seam (options.go: "a later one
		// wins").
		newRadio: func() fakeRadio {
			opts := make([]fakeic905.Option, 0, 10+len(IC905FakeSessionOpts))
			for ch := 0; ch < 10; ch++ {
				opts = append(opts, fakeic905.WithEmpty(0, ch))
			}
			opts = append(opts, IC905FakeSessionOpts...)
			return fakeic905.New(opts...)
		},
	},
	// The IC-7851 and IC-7850 (Tier 4b's first registration): TWO rows
	// over ONE driver package, ONE simulator and ONE profile — the
	// FTdx101D/FTdx101MP shape, and the same warning applies twice over.
	// writeTrialsComplete7851 and writeTrialsComplete7850 are BOTH false,
	// so neither radio has a hardware-evidenced write path, and the
	// Supported writes ic7851.WithSimulatedProfile() reaches here are a
	// claim about internal/fakeic7851 alone. This pairing is the only
	// place that option is legal.
	//
	// EACH ROW NAMES ITS OWN MODEL TO THE FAKE, and that is the one wire
	// difference between them: fakeic7851's WithModelName sets the bytes
	// the simulator answers to `19 00`, which the driver RECORDS into
	// Session.Identity().CATID after the static address and NEVER MATCHES
	// (core/driver/ic7851/doc.go §4 — no page of the manual says what an
	// IC-7851 answers, so there is nothing to match against). A real
	// radio of either model would answer whatever it answers; naming the
	// row's own model here keeps the two demo sessions distinguishable in
	// diagnostics without pretending the probe could tell them apart.
	IC7851Model: {
		newDriver: func() driver.Driver { return ic7851.New7851(ic7851.WithSimulatedProfile()) },
		newRadio: func() fakeRadio {
			opts := append([]fakeic7851.Option{fakeic7851.WithModelName(IC7851Model)}, IC7851FakeSessionOpts...)
			return ic7851FakeAdapter{fakeic7851.New(opts...)}
		},
	},
	IC7850Model: {
		newDriver: func() driver.Driver { return ic7851.New7850(ic7851.WithSimulatedProfile()) },
		newRadio: func() fakeRadio {
			opts := append([]fakeic7851.Option{fakeic7851.WithModelName(IC7850Model)}, IC7850FakeSessionOpts...)
			return ic7851FakeAdapter{fakeic7851.New(opts...)}
		},
	},
	// The IC-7760 (Tier 4b's second registration): ONE row, ONE driver
	// package, ONE simulator and ONE profile — the IC-7610's shape, and
	// the standing warning applies once. writeTrialsComplete is false
	// (core/driver/ic7760/caps.go), so this radio has no
	// hardware-evidenced write path and the Supported writes
	// ic7760.Simulated reaches here are a claim about internal/fakeic7760
	// alone. This pairing is the only place that Profile value is legal
	// outside its own package, which is what internal/guards' ic7760 row
	// confines.
	//
	// NO WithModelName TO PASS, unlike the two rows above: this fake
	// answers `19 00` with its own invented, deliberately implausible
	// 0xA5 token (internal/fakeic7760's WithIDReply doc comment), the
	// guide printing no reply value for the command anywhere. The driver
	// RECORDS that token into Session.Identity().CATID after the static
	// address B2 and NEVER MATCHES it.
	IC7760Model: {
		newDriver: func() driver.Driver { return ic7760.New(ic7760.Simulated) },
		newRadio:  func() fakeRadio { return ic7760FakeAdapter{fakeic7760.New()} },
	},
	// The IC-7100 (Tier 4b's third registration): ONE row, ONE driver
	// package, ONE simulator and ONE profile — the IC-7610's shape again,
	// and the standing warning applies once. writeTrialsComplete is false
	// (core/driver/ic7100/caps.go), so this radio has no
	// hardware-evidenced write path and the Supported writes
	// ic7100.Simulated reaches here are a claim about internal/fakeic7100
	// alone. This pairing is the only place that Profile value is legal
	// outside its own package, which is what internal/guards' ic7100 row
	// confines.
	//
	// NO ADAPTER, unlike the two rows above: this fake's Port() already
	// returns io.ReadWriteCloser, so the *fakeic7100.Radio goes into the
	// table as it stands (see the fakeRadio proof above).
	//
	// NO WithModelName TO PASS EITHER, and there is no such option to
	// pass: this fake answers `19 00` with its own invented, deliberately
	// implausible DE AD token (internal/fakeic7100's
	// defaultIdentityToken), the manual's command table printing an EMPTY
	// Data cell for 19 00 and no reply value anywhere. The driver RECORDS
	// that token into Session.Identity().CATID after the static address
	// 88 and NEVER MATCHES it (register entry ic7100-id-reply-value).
	IC7100Model: {
		newDriver: func() driver.Driver { return ic7100.New(ic7100.Simulated) },
		newRadio:  func() fakeRadio { return fakeic7100.New() },
	},
	// The IC-R8600 (Tier 4b's fourth and last registration): ONE row, ONE
	// driver package, ONE simulator and ONE profile — the IC-7610's shape
	// again, and the standing warning applies once. writeTrialsComplete is
	// false (core/driver/icr8600/caps.go), so this receiver has no
	// hardware-evidenced write path and the Supported writes
	// icr8600.Simulated reaches here are a claim about internal/fakeicr8600
	// alone. This pairing is the only place that Profile value is legal
	// outside its own package, which is what internal/guards' icr8600 row
	// confines.
	//
	// NO ADAPTER: this fake's Port() already returns io.ReadWriteCloser, so
	// the *fakeicr8600.Radio goes into the table as it stands (see the
	// fakeRadio proof above).
	//
	// NO WithModelName TO PASS EITHER, and there is no such option to pass:
	// this fake answers `19 00` with its own invented, deliberately
	// implausible DE AD token (internal/fakeicr8600's
	// defaultIdentityToken), the guide's command table printing an EMPTY
	// Data cell for 19 00 and no reply value anywhere. The driver RECORDS
	// that token into Session.Identity().CATID after the static address 96
	// and NEVER MATCHES it (register entry icr8600-id-token). WithIDToken
	// exists on that package for a consumer that wants to prove exactly
	// that; this row deliberately passes none, so the default stands.
	//
	// NOTHING IS EMPTIED HERE, unlike the IC905Model row above: this fake's
	// eight default channels all decode under core/civ/icr8600's own
	// layouts, so the sparse walk materialises them.
	ICR8600Model: {
		newDriver: func() driver.Driver { return icr8600.New(icr8600.Simulated) },
		newRadio:  func() fakeRadio { return fakeicr8600.New() },
	},
	// The FT-891 (Tier 1): ONE row, ONE driver package, ONE simulator and
	// ONE profile — the FTdx10's shape, and the standing warning applies
	// once. writeTrialsComplete is false (core/driver/ft891/caps.go), so
	// this radio has no hardware-evidenced write path and the Supported
	// writes ft891.Simulated reaches here are a claim about
	// internal/fakeft891 alone: that simulator stores and returns what the
	// combined MT Set carries, byte 28's live TAG flag included. This
	// pairing is the only place that Profile value is legal outside its own
	// package, which is what internal/guards' ft891 row confines.
	//
	// NO ADAPTER: this fake's Port() already returns io.ReadWriteCloser, so
	// the *fakeft891.Radio goes into the table as it stands (see the
	// fakeRadio proof above).
	//
	// NOTHING IS EMPTIED HERE AND NOTHING IS SEEDED, unlike the IC905Model
	// row above: internal/fakeft891's DefaultImage is already constrained to
	// what a demo radio should be (plan decision P13) — two occupied memory
	// channels and nine PMS pairs, every one of them decodable by this
	// radio's own dialect, and no 5xx or EMG slot, so the eleven-frame
	// discovery walk this driver's Open runs finds none and the demo radio
	// shows the two static banks alone.
	FT891Model: {
		newDriver: func() driver.Driver { return ft891.New(ft891.Simulated) },
		newRadio:  func() fakeRadio { return fakeft891.New(FT891FakeSessionOpts...) },
	},
	// The FT-991A (Tier 1's second): ONE row, ONE driver package, ONE
	// simulator and ONE profile — the FT-891's shape, and the standing
	// warning applies once. writeTrialsComplete is false
	// (core/driver/ft991a/caps.go), so this radio has no
	// hardware-evidenced write path and the Supported writes
	// ft991a.Simulated reaches here are a claim about internal/fakeft991a
	// alone: that simulator stores and returns what the ONE combined MT
	// Set carries, the five-state P8 byte included. This pairing is the
	// only place that Profile value is legal outside its own package,
	// which is what internal/guards' ft991a row confines.
	//
	// NO ADAPTER: this fake's Port() already returns io.ReadWriteCloser, so
	// the *fakeft991a.Radio goes into the table as it stands (see the
	// fakeRadio proof above).
	//
	// NOTHING IS EMPTIED HERE AND NOTHING IS SEEDED, as for the FT891Model
	// row above: internal/fakeft991a's DefaultImage is already constrained
	// to what a demo radio should be (plan decision P14) — two occupied
	// memory channels and two PMS pairs, every one of them decodable by
	// this radio's own dialect. There is no discovery walk to starve here:
	// this driver's Open probes nothing at all.
	FT991AModel: {
		newDriver: func() driver.Driver { return ft991a.New(ft991a.Simulated) },
		newRadio:  func() fakeRadio { return fakeft991a.New(FT991AFakeSessionOpts...) },
	},
	// The TS-590S and TS-590SG (Tier 6, the registry's first Kenwood rows):
	// TWO rows over ONE driver package, ONE simulator and ONE profile — the
	// FTdx101D/FTdx101MP shape — and the standing warning applies twice
	// over. Both writeTrialsComplete guards are false
	// (core/driver/ts590/caps.go keeps one per row), so neither radio has a
	// hardware-evidenced write path, and the Supported writes
	// ts590.Simulated reaches here are a claim about internal/fakets590
	// alone: that simulator stores and returns what the 50-byte MW carried.
	// This pairing is the only place that Profile value is legal outside its
	// own package, which is what internal/guards' ts590 row confines.
	//
	// EACH ROW NAMES ITS OWN ROW TWICE — once to the DRIVER and once to the
	// FAKE — and that is what this pair costs over every single-model row
	// above. ts590.RowS/RowSG selects the driver's identity, its byte-28
	// policy and its slot ceiling; fakets590.RowS/RowSG selects what the rig
	// answers "ID;" with. Four call sites, and a copy-paste that left any
	// one of them on the sibling's row builds a session that reads and
	// writes flawlessly and is simply the wrong radio — caught by
	// TestOpenFakeSessionFor_EveryRegisteredModel's identity leg, where the
	// driver's static CATID ("021" or "023") must equal what the rig
	// answered.
	//
	// NO ADAPTER: this fake's Port() already returns io.ReadWriteCloser, so
	// the *fakets590.Radio goes into the table as it stands (see the
	// fakeRadio proof above).
	//
	// NOTHING IS EMPTIED HERE AND NOTHING IS SEEDED, unlike the IC905Model
	// row above: internal/fakets590's DefaultImage is already what a demo
	// radio should be (plan decision P19) — four populated records, every
	// byte of them printed evidence, all decodable by core/kw's own parser,
	// and no image at all for the SG's unpublished 110-119.
	TS590SModel: {
		newDriver: func() driver.Driver { return ts590.New(ts590.RowS, ts590.Simulated) },
		newRadio:  func() fakeRadio { return fakets590.New(fakets590.RowS, TS590SFakeSessionOpts...) },
	},
	TS590SGModel: {
		newDriver: func() driver.Driver { return ts590.New(ts590.RowSG, ts590.Simulated) },
		newRadio:  func() fakeRadio { return fakets590.New(fakets590.RowSG, TS590SGFakeSessionOpts...) },
	},
	// The TS-890S and TS-990S (Tier 6's second pair): TWO rows over TWO
	// driver packages, TWO simulators and TWO profiles — the ic7610/ft991a
	// shape repeated, not the 590 pair's shared-package one — and the
	// standing warning applies once per row. Both writeTrialsComplete
	// constants are false (one per package, plan decision P18), so neither
	// radio has a hardware-evidenced write path, and the Supported writes
	// ts890.Simulated and ts990.Simulated reach here are a claim about
	// internal/fakets890 and internal/fakets990 alone: each simulator
	// validates and stores what the ONE MA0 Set carried. These pairings are
	// the only places those two Profile values are legal outside their own
	// packages, which is what internal/guards' two new rows confine.
	//
	// EACH ROW NAMES ONE PACKAGE FOUR WAYS — driver, profile, fake, option
	// variable — and a copy-paste that left any of the four on the sibling
	// is caught differently depending on which: a crossed DRIVER or FAKE
	// does not compile (two packages, two types), while a crossed OPTION
	// VARIABLE inside a row whose fake is right does, and is what
	// TestOpenFakeSessionFor_TS890SOptionSourceIsItsOwn and its mirror
	// exist for. The identity leg of
	// TestOpenFakeSessionFor_EveryRegisteredModel is the backstop: the
	// driver's static CATID ("024" or "022") must equal what the rig
	// answered.
	//
	// NO ADAPTER on either row: both fakes' Port() already returns
	// io.ReadWriteCloser (see the fakeRadio proofs above).
	//
	// NOTHING IS EMPTIED HERE AND NOTHING IS SEEDED, as on the 590 rows:
	// each DefaultImage is already what a demo radio should be — four
	// populated channels apiece, every byte printed evidence, all decodable
	// by core/kw/ma's own codecs, plus the 890S's blank-with-residual-name
	// channel 004, which exists to prove the empty predicate IGNORES a
	// residue rather than erroring on it.
	TS890SModel: {
		newDriver: func() driver.Driver { return ts890.New(ts890.Simulated) },
		newRadio:  func() fakeRadio { return fakets890.New(TS890SFakeSessionOpts...) },
	},
	TS990SModel: {
		newDriver: func() driver.Driver { return ts990.New(ts990.Simulated) },
		newRadio:  func() fakeRadio { return fakets990.New(TS990SFakeSessionOpts...) },
	},
	// v1.7.0 Kenwood/Yaesu wave, tenth row: bare New, no adapter needed.
	FTdx5000Model: {
		newDriver: func() driver.Driver { return ftdx5000.New(ftdx5000.Simulated) },
		newRadio:  func() fakeRadio { return fakeftdx5000.New(FTdx5000FakeSessionOpts...) },
	},
	// v1.7.0 Kenwood/Yaesu wave, eleventh row: bare New, no adapter needed.
	FTdx9000Model: {
		newDriver: func() driver.Driver { return ftdx9000.New(ftdx9000.Simulated) },
		newRadio:  func() fakeRadio { return fakeftdx9000.New(FTdx9000FakeSessionOpts...) },
	},
	// v1.7.0 Kenwood/Yaesu wave, twelfth and last row: bare New, no
	// adapter needed.
	FT950Model: {
		newDriver: func() driver.Driver { return ft950.New(ft950.Simulated) },
		newRadio:  func() fakeRadio { return fakeft950.New(FT950FakeSessionOpts...) },
	},
	// v1.8.0 Yaesu trio, first row: bare New, no adapter needed.
	FTdx3000Model: {
		newDriver: func() driver.Driver { return ftdx3000.New(ftdx3000.Simulated) },
		newRadio:  func() fakeRadio { return fakeftdx3000.New(FTdx3000FakeSessionOpts...) },
	},
	// v1.8.0 Yaesu trio, second row: bare New, no adapter needed.
	FTdx1200Model: {
		newDriver: func() driver.Driver { return ftdx1200.New(ftdx1200.Simulated) },
		newRadio:  func() fakeRadio { return fakeftdx1200.New(FTdx1200FakeSessionOpts...) },
	},
	// v1.8.0 Yaesu trio, third and last row: bare New, no adapter needed.
	FT450DModel: {
		newDriver: func() driver.Driver { return ft450d.New(ft450d.Simulated) },
		newRadio:  func() fakeRadio { return fakeft450d.New(FT450DFakeSessionOpts...) },
	},
	// v1.7.0 Kenwood/Yaesu wave, first row: fakets2000's Port() is already
	// io.ReadWriteCloser, so no adapter is needed.
	TS2000Model: {
		newDriver: func() driver.Driver { return ts2000.NewTS2000(ts2000.WithSimulatedProfile()) },
		newRadio:  func() fakeRadio { return fakets2000.New(TS2000FakeSessionOpts...) },
	},
	// v1.7.0 Kenwood/Yaesu wave, second row: same shared TS2000FakeSessionOpts
	// as the TS-2000's own entry above — safe here, unlike the ts590 pair's
	// two SEPARATE variables, because internal/fakets2000's WithModelName
	// carries no wire meaning at all (its own doc comment), so there is no
	// "seeded the wrong radio differently" hazard for a shared variable to
	// create.
	TS2000XModel: {
		newDriver: func() driver.Driver { return ts2000.NewTS2000X(ts2000.WithSimulatedProfile()) },
		newRadio: func() fakeRadio {
			return fakets2000.New(append([]fakets2000.Option{fakets2000.WithModelName("TS-2000X")}, TS2000FakeSessionOpts...)...)
		},
	},
	// v1.7.0 Kenwood/Yaesu wave, third and last ts2000 row: same shared
	// TS2000FakeSessionOpts, same reasoning as TS-2000X's entry above.
	TSB2000Model: {
		newDriver: func() driver.Driver { return ts2000.NewTSB2000(ts2000.WithSimulatedProfile()) },
		newRadio: func() fakeRadio {
			return fakets2000.New(append([]fakets2000.Option{fakets2000.WithModelName("TS-B2000")}, TS2000FakeSessionOpts...)...)
		},
	},
	// v1.7.0 Kenwood/Yaesu wave, fourth row: fakets570's Port() returns
	// net.Conn, so it goes through ts570FakeAdapter (like ic7610's).
	TS570DModel: {
		newDriver: func() driver.Driver { return ts570.NewD(ts570.Simulated) },
		newRadio:  func() fakeRadio { return ts570FakeAdapter{fakets570.New(TS570DFakeSessionOpts...)} },
	},
	// v1.7.0 Kenwood/Yaesu wave, fifth row: same shared TS570DFakeSessionOpts
	// as the TS-570D's own entry above — internal/fakets570's WithModelName
	// changes only its own CATID answer (own doc register), on the same
	// no-hazard footing as the ts2000 family's shared variable.
	TS570SModel: {
		newDriver: func() driver.Driver { return ts570.NewS(ts570.Simulated) },
		newRadio: func() fakeRadio {
			return ts570FakeAdapter{fakets570.New(append([]fakets570.Option{fakets570.WithModelName("TS-570S")}, TS570DFakeSessionOpts...)...)}
		},
	},
	// v1.7.0 Kenwood/Yaesu wave, sixth and last ts570 row: same shared
	// TS570DFakeSessionOpts, same reasoning as TS-570S's entry above.
	TS570DGModel: {
		newDriver: func() driver.Driver { return ts570.NewDG(ts570.Simulated) },
		newRadio: func() fakeRadio {
			return ts570FakeAdapter{fakets570.New(append([]fakets570.Option{fakets570.WithModelName("TS-570DG")}, TS570DFakeSessionOpts...)...)}
		},
	},
	// v1.7.0 Kenwood/Yaesu wave, seventh row: bare New, no adapter needed.
	TS870SModel: {
		newDriver: func() driver.Driver { return ts870s.New(ts870s.Simulated) },
		newRadio:  func() fakeRadio { return fakets870s.New(TS870SFakeSessionOpts...) },
	},
	// v1.7.0 Kenwood/Yaesu wave, eighth row: internal/fakeft2000's bare
	// New() defaults to FT-2000 (doc.go), so no WithModelName is needed
	// here — unlike the FT-2000D's own entry below.
	FT2000Model: {
		newDriver: func() driver.Driver { return ft2000.NewFT2000(ft2000.Simulated) },
		newRadio:  func() fakeRadio { return fakeft2000.New(FT2000FakeSessionOpts...) },
	},
	// v1.7.0 Kenwood/Yaesu wave, ninth row: same shared FT2000FakeSessionOpts
	// as the FT-2000's own entry above, WithModelName selecting the D row.
	FT2000DModel: {
		newDriver: func() driver.Driver { return ft2000.NewFT2000D(ft2000.Simulated) },
		newRadio: func() fakeRadio {
			return fakeft2000.New(append([]fakeft2000.Option{fakeft2000.WithModelName("FT-2000D")}, FT2000FakeSessionOpts...)...)
		},
	},
	// The IC-7800 (v1.7.0 Icom wave's first registration): ONE row, ONE
	// driver package, ONE simulator, on the IC-7610's footing.
	// writeTrialsComplete is false (core/driver/ic7800/caps.go), so this
	// radio has no hardware-evidenced write path and the Supported writes
	// ic7800.Simulated reaches here are a claim about internal/fakeic7800
	// alone.
	IC7800Model: {
		newDriver: func() driver.Driver { return ic7800.New(ic7800.Simulated) },
		newRadio:  func() fakeRadio { return ic7800FakeAdapter{fakeic7800.New(IC7800FakeSessionOpts...)} },
	},
	// The IC-7600 (v1.7.0 Icom wave's second registration), on IC7800Model's
	// footing.
	IC7600Model: {
		newDriver: func() driver.Driver { return ic7600.New(ic7600.Simulated) },
		newRadio:  func() fakeRadio { return ic7600FakeAdapter{fakeic7600.New(IC7600FakeSessionOpts...)} },
	},
	// The IC-7410 (v1.7.0 Icom wave's third registration): bare New with
	// WithSimulatedProfile(), the IC-7610's own option shape rather than
	// IC7800Model's/IC7600Model's profile-argument one.
	IC7410Model: {
		newDriver: func() driver.Driver { return ic7410.New(ic7410.WithSimulatedProfile()) },
		newRadio:  func() fakeRadio { return ic7410FakeAdapter{fakeic7410.New(IC7410FakeSessionOpts...)} },
	},
	// The IC-7700 (v1.7.0 Icom wave's fourth registration), on IC7800Model's
	// footing.
	IC7700Model: {
		newDriver: func() driver.Driver { return ic7700.New(ic7700.Simulated) },
		newRadio:  func() fakeRadio { return ic7700FakeAdapter{fakeic7700.New(IC7700FakeSessionOpts...)} },
	},
	// The IC-9100 (v1.7.0 Icom wave's fifth registration), on IC7800Model's
	// footing.
	IC9100Model: {
		newDriver: func() driver.Driver { return ic9100.New(ic9100.Simulated) },
		newRadio:  func() fakeRadio { return ic9100FakeAdapter{fakeic9100.New(IC9100FakeSessionOpts...)} },
	},
	// The IC-7200 (v1.7.0 Icom wave's sixth and last registration), on
	// IC7800Model's footing.
	IC7200Model: {
		newDriver: func() driver.Driver { return ic7200.New(ic7200.Simulated) },
		newRadio:  func() fakeRadio { return ic7200FakeAdapter{fakeic7200.New(IC7200FakeSessionOpts...)} },
	},
}

// OpenFakeSessionFor opens a session against a fresh in-process fake rig
// for model, via model's own entry in fakeDrivers. This function knows
// nothing about any model's simulator options: each entry's newRadio
// closure carries its own (M9c-5 E5 — one *FakeSessionOpts test-only seam
// of that kind per registered model).
//
// An unrecognised model fails with *UnknownModelError BEFORE any fake rig
// is constructed. The returned close function releases the session first,
// then the fake rig (both simulators' Close is prompt — see their
// interruptible scripted delays), returning the session's error if both
// fail.
//
// An FTdx10 or FTdx101 open is SLOW by design and that is not a defect to
// fix here: those drivers' Open probes the whole declared 5xx range plus
// EMG (~100 exchanges) because neither radio has a verified discovery
// termination rule, so each call costs seconds rather than milliseconds
// (M9c-6 and M9d-2 plans: acknowledged and budgeted, and M9d-2 doubled the
// cost by registering two more such models). Anybody shortening it must
// read that driver's doc.go first — the shortening IS the assumption.
func OpenFakeSessionFor(ctx context.Context, model string) (driver.Session, func() error, error) {
	entry, ok := fakeDrivers[model]
	if !ok {
		return nil, nil, &UnknownModelError{Model: model, Supported: SupportedModels()}
	}

	r := entry.newRadio()

	// registerDriver validates entry.newDriver() (its Model()/
	// Capabilities().Model agreement, Capabilities().Validate, the
	// ConsentedUnverified-baseline guard) and wraps any failure as
	// *RegisterDriverError.
	d := entry.newDriver()
	if err := registerDriver(d); err != nil {
		_ = r.Close()
		return nil, nil, err
	}

	sess, err := d.Open(ctx, r.Port(), driver.Identity{Port: "fake", USBSerial: "SIM0001"})
	if err != nil {
		// Open owns the port (r.Port()) on both outcomes: it is already
		// closed, but the fakeradio.Radio behind it is not — close it.
		_ = r.Close()
		return nil, nil, &OpenFakeSessionError{Cause: err}
	}

	closeAll := func() error {
		sessErr := sess.Close()
		radioErr := r.Close()
		if sessErr != nil {
			return sessErr
		}
		return radioErr
	}
	return sess, closeAll, nil
}

// OpenFakeSessionError is OpenFakeSessionFor's typed failure when the
// driver's own Open fails against the in-process fakeradio port. See
// wiring.go's RegisterDriverError doc comment for why Error() carries
// this package's own generic wording, and why a caller wanting different
// wording (Fix 7, adjudicated LOW, Codex M6 #7) should use Cause via
// errors.As instead.
type OpenFakeSessionError struct{ Cause error }

func (e *OpenFakeSessionError) Error() string {
	return fmt.Sprintf("wiring: open fake session: %v", e.Cause)
}

func (e *OpenFakeSessionError) Unwrap() error { return e.Cause }
