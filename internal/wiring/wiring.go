// SPDX-License-Identifier: GPL-3.0-or-later

// Package wiring holds the session-construction plumbing shared by every
// composition root in this repository (cmd/rigprog, app/), so the GUI and
// the CLI share one registry/driver/session wiring rather than two
// independently-drifting copies.
//
// It keeps a deliberate structural-exclusivity shape: two fully
// self-contained, model-keyed session paths — the REAL one (this file:
// OpenRealSessionWith, the single implementation, plus OpenRealSessionFor,
// its zero-option delegate — two exported names over one body) and the
// SIMULATED one (fake.go's OpenFakeSessionFor) — with no shared helper
// accepting a profile alongside a port. That absence is the point: it keeps
// the invalid RealHardware/fake-rig or Simulated/real-port pairings
// structurally unrepresentable in the code shape, not merely unreached. What
// a caller may vary on the real path is bounded by SessionOptions, which
// carries the user's consent and may never carry a profile or a port object
// — the constraint is about what can be PAIRED, and it is untouched by the
// second name.
//
// EACH registered driver's simulated-profile selector — e.g. ft710.Simulated,
// ftdx10.Simulated, ftdx101.Simulated (one token for both FTDX101 siblings,
// since one driver package drives both) — is referenced in exactly ONE
// non-test .go file repo-wide, fake.go, pinned per driver by
// internal/guards' TestSimulatedProfileTokensConfinement.
package wiring

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft710"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft891"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft991a"
	"github.com/gm5dna/open-rig-programmer/core/driver/ftdx10"
	"github.com/gm5dna/open-rig-programmer/core/driver/ftdx101"
	"github.com/gm5dna/open-rig-programmer/core/driver/ftdx5000"
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
	"github.com/gm5dna/open-rig-programmer/core/driver/ts590"
	"github.com/gm5dna/open-rig-programmer/core/driver/ts890"
	"github.com/gm5dna/open-rig-programmer/core/driver/ts990"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// DefaultModel names the driver.Registry key the model-keyed lookups below
// use when a caller has not (or cannot yet) name a model explicitly: the
// FALLBACK model, not the only registrable one. cmd/rigprog resolves it
// when --model is absent, and app/ when the frontend passes "" (it has no
// model picker yet).
//
// It stays exactly "FT-710" however many other models are registered:
// which radio a caller gets by DEFAULT is a compatibility promise about
// every file, snapshot and journal written before any second model existed
// (see ResolveSnapshotDir's own model rule), not a statement about how many
// models this package supports. Changing it would silently re-point every
// default-model caller at a different radio.
const DefaultModel = "FT-710"

// FTdx10Model names the FTdx10's realDrivers/fakeDrivers key, which must
// equal ftdx10.New(...).Model() — that agreement is not assumed here but
// pinned by TestDriverTableKeysMatchDriverModel, which walks both tables.
// A named constant rather than a bare literal at each of its uses (the two
// table keys) because the two MUST be the same string: a typo in one alone
// would build a model openable for real but not simulated, which is the
// very drift TestRealAndFakeDriverTablesAgree exists to catch — and this
// way it cannot happen at all. DefaultModel is exported and this is too,
// so a caller naming the model (a CLI --model default, a GUI picker) has
// the same kind of handle for both.
const FTdx10Model = "FTdx10"

// FTdx101DModel names the FTDX101D's realDrivers/fakeDrivers key, which must
// equal ftdx101.NewD(...).Model() — pinned, like FTdx10Model's, by
// TestDriverTableKeysMatchDriverModel walking both tables. A named constant
// rather than a bare literal at each of its uses for exactly the reason
// FTdx10Model is one: the two table keys MUST be the same string, and a typo
// in one alone would build a model openable for real but not simulated.
//
// THE SPELLING IS THE PROJECT'S, of a manual fact (matrix §1.1). The manual
// prints "FTDX101D" in full capitals throughout; this project writes
// "FTdx101D", matching how "FTdx10" and "FT-710" are already spelt here. The
// two are NOT interchangeable: this constant is the driver-registry key and
// the radiotext key, and internal/radiotext deliberately leaves "FTDX101D"
// unknown so a caller that reached for the manual's spelling fails loudly
// rather than serving blank advisories.
//
// NOT the same string as internal/extable's "FTdx101D/MP", which is the
// JOINT inventory form: one EX profile serves both radios because the manual
// prints Table 2 once for the pair. That is a statement about a shared
// chart; this is a registry key for one radio.
const FTdx101DModel = "FTdx101D"

// FTdx101MPModel names the FTDX101MP's realDrivers/fakeDrivers key, which
// must equal ftdx101.NewMP(...).Model(). See FTdx101DModel for the spelling
// rule and the extable-form distinction, which apply here unchanged.
//
// TWO constants for two radios, and no shared "FTdx101" handle between them,
// deliberately: core/driver/ftdx101 offers NewD and NewMP over one
// implementation and no bare New, and core/cat/ftdx101 offers DialectD and
// DialectMP over one config and no bare Dialect(), for the same reason —
// there are two models, so neither is the other's fallback and neither is
// reachable by a caller that failed to choose.
const FTdx101MPModel = "FTdx101MP"

// IC7610Model names the IC-7610's realDrivers/fakeDrivers key, which must
// equal ic7610.New(...).Model() — pinned, like the three Yaesu constants
// above, by TestDriverTableKeysMatchDriverModel walking both tables. A
// named constant rather than a bare literal at each of its uses for the
// same reason those three are: the two table keys MUST be the same
// string, and a typo in one alone would build a model openable for real
// but not simulated.
//
// THIS IS THE FIRST NON-YAESU REGISTRATION. The spelling
// is the manufacturer's own, and it carries the hyphen ic7610.go's own
// Model() method and Capabilities().Model both declare ("IC-7610", not
// "IC7610" or "ic7610") — the two agreements TestDriverTableKeysMatchDriverModel
// and internal/guards' simulated-token guard both depend on, exactly as
// they do for every Yaesu row.
const IC7610Model = "IC-7610"

// IC7300Model names the IC-7300's realDrivers/fakeDrivers key, which must
// equal ic7300.New(...).Model() — pinned, like IC7610Model, by
// TestDriverTableKeysMatchDriverModel walking both tables. A named
// constant rather than a bare literal at each of its uses for the same
// reason every other model constant is: the two table keys MUST be the
// same string, and a typo in one alone would build a model openable for
// real but not simulated.
//
// THIS IS THE SECOND ICOM REGISTRATION, and the FIRST PAIR — the IC-7300
// and IC-7300MK2 register together, over ONE driver package
// (core/driver/ic7300, New and NewMK2) and SEPARATE fakes
// (internal/fakeic7300 / internal/fakeic7300mk2). The two documents are
// mutually silent about each other, so the separation the pair needs is
// carried by that package's modelParams rows — two rows, each populated
// from its own manual and neither derived from the other — and by the two
// independently written fakes each model's end-to-end tests run against.
// It was two packages until the v1.4.1 sweep folded them
// (core/driver/ic7300/doc.go's package comment, and doc_mk2.go for the
// MK2's own record).
const IC7300Model = "IC-7300"

// IC7300MK2Model names the IC-7300MK2's realDrivers/fakeDrivers key, which
// must equal ic7300.NewMK2(...).Model(). See IC7300Model for the pairing
// rationale: a SEPARATE constant, a SEPARATE modelParams row, a SEPARATE
// fake, because the two Icom documents this pair is built from never
// reference each other and no lift in one is a lift for the sibling.
const IC7300MK2Model = "IC-7300MK2"

// IC705Model names the IC-705's realDrivers/fakeDrivers key, which must
// equal ic705.New(...).Model() — pinned, like every other Icom constant
// above, by TestDriverTableKeysMatchDriverModel walking both tables.
//
// THIS IS THE THIRD ICOM REGISTRATION, and the FIRST
// SINGLE-MODEL one since the IC-7610: a lone driver package
// (core/driver/ic705) and a lone fake (internal/fakeic705), on the same
// one-row footing as IC7610Model above — no sibling, no pairing rationale
// to restate.
const IC705Model = "IC-705"

// IC9700Model names the IC-9700's realDrivers/fakeDrivers key, which must
// equal ic9700.New(...).Model() — pinned, like every other Icom constant
// above, by TestDriverTableKeysMatchDriverModel walking both tables.
//
// THIS IS THE FOURTH ICOM REGISTRATION, and the SECOND
// SINGLE-MODEL one since the IC-705: a lone driver package
// (core/driver/ic9700) and a lone fake (internal/fakeic9700), on the same
// one-row footing as IC705Model above — no sibling, no pairing rationale
// to restate.
//
// UNLIKE EVERY OTHER REGISTERED ICOM MODEL, this radio's static Banks is
// THREE, not two: MEM, SCAN and CALL (core/driver/ic9700/caps.go's banks),
// all DENSE (321 addressable slots total, completely enumerable — no
// group-addressed sparse space of the kind the IC-705 declares). Nothing
// about registration itself changes for a third bank; it is app/uispec.go's
// own bank-shape tests that have to say so, not this table.
const IC9700Model = "IC-9700"

// IC905Model names the IC-905's realDrivers/fakeDrivers key, which must
// equal ic905.New(...).Model() — pinned, like every other Icom constant
// above, by TestDriverTableKeysMatchDriverModel walking both tables.
//
// THIS IS THE FIFTH ICOM REGISTRATION AND THE LAST OF
// THE TIER: a lone driver package (core/driver/ic905) and a lone fake
// (internal/fakeic905), on the same one-row footing as IC705Model and
// IC9700Model above — no sibling, no pairing rationale to restate.
//
// BACK TO TWO BANKS, but not back to the IC-705's shape either: MEM
// (core/driver/ic905/caps.go's baseCapabilities) is SPARSE over a
// 100 x 100 group-addressed space — the same spec.Bank.Sparse/Groups/
// PerGroup/Budget descriptor the IC-705's own MEM bank carries — and its
// materialised set is DISCOVERED at Open, never enumerated statically.
// CALL is a DENSE bank of twelve named slots, "C01".."C12", in a namespace
// spec.ParseSparseSlot structurally refuses to parse — disjoint from MEM's
// "G%02d-%03d" addresses by construction, not by an arithmetic accident of
// where the two group ranges happen to sit (ruling R4).
const IC905Model = "IC-905"

// IC7851Model and IC7850Model name the IC-7851's and the IC-7850's
// realDrivers/fakeDrivers keys, which must equal
// ic7851.New7851(...).Model() and ic7851.New7850(...).Model() — pinned,
// like every other Icom constant above, by
// TestDriverTableKeysMatchDriverModel walking both tables.
//
// THIS IS THE ADDITIONS TIER'S FIRST REGISTRATION, and
// the first Icom PAIR SHARING ONE DRIVER PACKAGE: core/driver/ic7851
// offers New7851 and New7850 over ONE implementation, ONE civ.Profile
// (core/civ/ic7851) and ONE fake (internal/fakeic7851), with no bare New.
// That is the FTdx101D/FTdx101MP shape (FTdx101DModel above), not the
// IC-7300/IC-7300MK2 one — that pair has separate driver packages and
// separate fakes; these two rows differ in Model(), in Identity and in
// nothing else the wire can see.
//
// THE USER PICKS THE ROW, AND THE PROBE CANNOT NARROW IT (spec D1.2).
// The two models share one manual, one CI-V address (8Eh, printed as "the
// default address of IC-7850/IC-7851") and one frame shape, and the 19 00
// reply value is undocumented for both — so they are INDISTINGUISHABLE by
// this programme's admitted evidence and probe, at factory defaults and
// not merely under a moved address. core/driver/ic7851/doc.go §1 states
// it; core/civ/tier_test.go's `indistinguishable` table records it at
// tier level with its citation; and neither row may ever be inferred
// from the other, so a session opened as an IC-7851 reports an IC-7851
// because that is what the user chose, never because anything confirmed
// it.
//
// TWO BANKS, BOTH DENSE, the IC-7610's shape: MEM ("001".."099") and SCAN
// (the two programmed scan edges "P1" and "P2"), which the wire addresses
// as two further values of the SAME flat two-byte selector
// (core/driver/ic7851/caps.go's memSlots and scanSlots). There is no
// sparse space to discover here, so a plain `--model IC-7851` session
// enumerates its whole inventory statically and Open walks a bounded run
// of channels for the record-length fingerprint alone.
const (
	IC7851Model = "IC-7851"
	IC7850Model = "IC-7850"
)

// IC7760Model names the IC-7760's realDrivers/fakeDrivers key, which must
// equal ic7760.New(...).Model() — pinned, like every other Icom constant
// above, by TestDriverTableKeysMatchDriverModel walking both tables.
//
// THE ADDITIONS TIER'S SECOND REGISTRATION, and a
// SINGLE-ROW one: core/driver/ic7760 has ONE member (its caps.go says so
// in as many words — "this family has one member"), so there is one
// constant, one driver package, one civ.Profile and one fake, on the same
// footing as IC7610Model above and NOT on the IC-7851 pair's. It takes
// its profile as an ARGUMENT (ic7760.New(profile, opts...)), which is the
// IC-7610's constructor shape rather than the IC-7851's option shape, and
// that is what decides both realDrivers' IC7760Model row and the
// TOKEN internal/guards confines for this package (ic7760.Simulated, a
// Profile constant, not an option).
//
// TWO BANKS, BOTH DENSE, the IC-7610's shape: MEM ("001".."099") and SCAN
// (the two programmed scan edges "P1" and "P2"), which the wire addresses
// as two further values of the SAME flat two-byte selector — the profile
// declares the memories as its base range 1..99 and P1/P2 as ONE extra
// flat range 100..101 (core/civ/ic7760/profile.go), so the base inventory
// cannot silently absorb them. There is no sparse space to discover, so a
// plain `--model IC-7760` session enumerates its whole inventory
// statically and Open walks a bounded run of channels — ten early
// memories, then P1 and P2 — for the record-length fingerprint alone.
//
// ITS 99 + P1/P2 INVENTORY IS MANUAL-EVIDENCED (additions spec Erratum
// 5), not assumed: PDF p.20 (folio 19) and PDF p.4 (folio 3) of the
// IC-7760 CI-V Reference Guide revision 2 both print the addresses, so
// the count carries no assumption-register entry of its own.
//
// INDISTINGUISHABLE FROM THE IC-7610 AND THE IC-7851/IC-7850 BY THE
// LENGTH FINGERPRINT ALONE (spec D5's 25 B / Flat row): this radio
// completes that declared set of three, and core/civ/tier_test.go's
// `indistinguishable` table now carries both of its pairings with their
// citations. It is a fingerprint limitation and not a field one — the
// three factory addresses B2h, 98h and 8Eh differ, so a wrong radio at
// its own address does not answer at all.
const IC7760Model = "IC-7760"

// IC7100Model names the IC-7100's realDrivers/fakeDrivers key, which must
// equal ic7100.New(...).Model() — pinned, like every other Icom constant
// above, by TestDriverTableKeysMatchDriverModel walking both tables.
//
// THE ADDITIONS TIER'S THIRD REGISTRATION, and a
// single-row one again: core/driver/ic7100 has one member, one civ.Profile
// and one fake, on the IC-7610's and IC-7760's footing rather than the
// IC-7851 pair's. It takes its profile as an ARGUMENT
// (ic7100.New(profile, opts...)), which decides both realDrivers' IC7100Model row's
// body below and the TOKEN internal/guards confines for this package
// (ic7100.Simulated, a Profile constant, not an option) — checked against
// core/driver/ic7100/ic7100.go, not assumed from the nearest precedent.
//
// A BANK-ADDRESSED CHANNEL SPACE, the first this registry holds: the wire
// address is a packed-BCD bank byte followed by a two-byte channel
// (civ.AddressFormBankChannel, three address bytes), and the profile's base
// rectangle is banks 01–05 × channels 0001–0099 — 495 DENSE slots, rendered
// "A-001".."E-099" (core/civ/ic7100/profile.go; additions spec D2.2). ONE
// bank, spec.BankMemory. There is no sparse space to discover, so a plain
// `--model IC-7100` session enumerates its whole inventory statically and
// Open walks a bounded run of channels for the record-length fingerprint
// alone.
//
// THE SCAN-EDGE AND CALL CHANNELS ARE REFUSED, AND THAT IS THE POINT. The
// field legend names ten further channel codes — programmed scan edges
// 0100–0105 and call channels 0106–0109 — but section 20 never says what
// the BANK byte carries for any of them, and the clearing block omits that
// field entirely. Inventing a bank byte would put an assumed address on the
// wire, so the profile declares NO ExtraRanges and core/driver/ic7100's
// parseSlot refuses every channel outside 1..99 (read.go, pinned by
// TestReadChannelRefusesSpecialsBeforeTraffic). Register entry
// ic7100-special-bank-byte carries the lift, and README.md and
// docs/icom-models.md say so to the user rather than quietly listing 495
// slots as if that were the whole radio.
//
// NO SerialFramingReporter, WHICH MAKES IT THE FIRST REGISTERED ICOM ROW TO
// OPEN AT 8-N-2. Every Icom driver before it reports one stop bit; this one
// deliberately implements nothing, because the manual's only 8-N-1 sentence
// (PDF p.174) is the DV low-speed DATA application and not the CI-V link
// (core/civ/ic7100/doc.go, register entry ic7100-serial-framing). So
// stopBitsFor finds no report and the port opens at transport.DefaultStopBits,
// the Yaesu rows' path — pinned by
// TestOpenRealSessionFor_IC7100OpensAtEightNTwo, which exists because the
// absence is a claim, not an oversight.
//
// INDISTINGUISHABLE FROM THE IC-9700 BY THE LENGTH FINGERPRINT ALONE (spec
// D5's 111 B row): both accept a 111-byte record over a THREE-byte address
// whose leading byte is a small 01-upwards index, so neither property the
// probe holds can separate them, and core/civ/tier_test.go's
// `indistinguishable` table carries that pairing with its citation. The
// IC-705 shares the same 111 bytes and IS separable, by address width alone
// (four bytes against three). As everywhere else in this tier it is a
// fingerprint limitation and not a field one — the factory addresses 88h and
// A2h differ, so a wrong radio at its own address does not answer at all.
const IC7100Model = "IC-7100"

// ICR8600Model names the IC-R8600's realDrivers/fakeDrivers key, which
// must equal icr8600.New(...).Model() — pinned, like every other Icom
// constant above, by TestDriverTableKeysMatchDriverModel walking both
// tables.
//
// THE ADDITIONS TIER'S FOURTH AND LAST REGISTRATION,
// and a single-row one: core/driver/icr8600 has one member, one
// civ.Profile and one fake. It takes its profile as an ARGUMENT
// (icr8600.New(profile, opts...)), which decides both
// realDrivers' ICR8600Model row and the TOKEN internal/guards
// confines for this package (icr8600.Simulated, a Profile constant, not
// an option) — checked against core/driver/icr8600/icr8600.go:48, not
// assumed from the nearest precedent.
//
// THE FIRST RECEIVER THIS REGISTRY HOLDS, and the first row whose
// capabilities declare spec.ReceiveOnly (core/driver/icr8600/caps.go;
// additions spec D4.2). Everything downstream of that is already
// capability-keyed and needed no code here: spec.Validate refuses a
// ReceiveOnly model that grades tx_frequency or tone_tx above
// Unsupported, app.GetUISpec carries the radio-level Transmit string, and
// internal/radiotext's icr8600Text supplies the "receiver — no transmit
// fields" grid legend D4.2 asks for by name. What this row adds is the
// first model that exercises any of it.
//
// A ZERO-BASED, SPARSE, WIDE-GROUP ADDRESS SPACE, the first of each in
// this registry: two packed-BCD group bytes then two channel bytes
// (civ.AddressFormWideGroupChannel, FOUR address bytes), over groups
// 0000–0099 × channels 00–99 with BOTH bases 0 (additions spec Erratum 2;
// core/civ/icr8600/profile.go), rendered "G00-000".."G99-099". The bank is
// SPARSE and its CAPACITY IS UNDOCUMENTED — spec.Bank.BudgetUnstated is
// the positive declaration of that silence (additions spec D3.4, register
// entry icr8600-budget), so codeplug.Diff skips its over-budget refusal
// and the honesty documents say the radio's behaviour when full is
// unknown rather than inventing a number.
//
// A MODE-KEYED RECORD, the first civ.DiscriminatorModeByte profile:
// record-only lengths {37, 39, 41, 43, 44, 45}, one per declared tail,
// with FM and DCR BOTH at 44 and told apart by the mode byte rather than
// by length (core/civ/icr8600/profile.go's recordLayouts).
//
// SEPARABLE FROM EVERY OTHER REGISTERED PROFILE, so it adds NO entry to
// core/civ/tier_test.go's `indistinguishable` table — the first additions
// row of which that is true. Its set is disjoint from every other Icom
// family's EXCEPT the IC-7300's {39} and the IC-7300MK2's {45}, both of
// which it contains; those two pairings are separated by ADDRESS WIDTH
// (four bytes against two), measured rather than declared by
// TestTierRecordShapes_DistinctOrDeclared.
//
// NO --civ-address OPTION, as everywhere else in this tier: this driver
// talks only to 96h (core/civ/icr8600/profile.go's RadioAddress), and a
// receiver moved off it simply times out (register entry
// icr8600-address-move).
const ICR8600Model = "IC-R8600"

// FT891Model names the FT-891's realDrivers/fakeDrivers key, which must
// equal ft891.New(...).Model() — pinned, like every constant above, by
// TestDriverTableKeysMatchDriverModel walking both tables. A named
// constant rather than a bare literal at each of its uses for the same
// reason every other model constant is: the two table keys MUST be the
// same string, and a typo in one alone would build a model openable for
// real but not simulated.
//
// TIER 1's REGISTRATION, the first Yaesu model added after the Icom
// tiers above. It is a
// single-row registration on the FTdx10's footing rather than the
// FTdx101 pair's: core/driver/ft891 has one member, core/cat/ft891 offers
// one bare Dialect(), and internal/fakeft891 one bare New. The spelling
// is the manual's own, hyphen included (capability matrix §1.1).
//
// TWO SLUGS EXIST FOR THIS RADIO AND THEY ARE DIFFERENT STRINGS The Go PACKAGE slug is "ft891" —
// core/driver/ft891, core/cat/ft891, internal/fakeft891, and
// internal/extable's own profile key. ModelSlug(FT891Model), which is
// what names this radio's snapshot and journal directory, is "ft-891":
// the hyphen in the model name is collapsed to a separator, not deleted.
// TestModelSlug pins the second and TestModelSlugsUnique pins that it
// collides with no other registered model's.
//
// TWO STATIC BANKS AND UP TO TWO MORE DISCOVERED AT Open. MEM ("001".."099")
// and PMS ("P1L".."P9U") are declared statically and DENSE; the 5 MHz bank
// ("501".."510") and the emergency channel ("EMG") are DISCOVERED, by an
// eleven-frame MR walk at Open, and appear only when the radio answers
// (core/driver/ft891/ft891.go's discoverInventory, matrix §3.4). Those two
// are READ-ONLY, and more sharply so than the FTdx10's and FTdx101's
// equivalents: this radio reads them by MR alone, and MR's 28-position
// answer carries neither a tag nor a tag-display flag, so both of those
// fields take the ZERO FieldSupport there rather than merely an unwritable
// one (matrix §2.5). Nothing about registration itself
// changes for that; it is app/uispec_test.go's own bank-shape tests that
// have to say so, not this table.
//
// NO SerialFramingReporter, like the four Yaesu rows above it and unlike
// every Icom row but the IC-7100's: this radio's CAT manual carries no
// serial-framing statement at all, so 8-N-2 for the FT-891 is an ASSUMED
// entry in core/driver/ft891/doc.go's own register — FRAMING: 8 DATA BITS,
// NO PARITY, TWO STOP BITS — with a named hardware lift, and the port opens
// at transport.DefaultStopBits by the absence of a report rather than by a
// driver's claim.
const FT891Model = "FT-891"

// FT991AModel names the FT-991A's realDrivers/fakeDrivers key, which must
// equal ft991a.New(...).Model() — pinned, like every constant above, by
// TestDriverTableKeysMatchDriverModel walking both tables. A named
// constant rather than a bare literal at each of its uses for the same
// reason every other model constant is: the two table keys MUST be the
// same string, and a typo in one alone would build a model openable for
// real but not simulated.
//
// TIER 1's SECOND REGISTRATION, following the FT-891's row above. It is a
// single-row registration on the FT-891's own footing rather than the
// FTdx101 pair's: core/driver/ft991a has one member, core/cat/ft991a
// offers one bare Dialect(), and internal/fakeft991a one bare New. The
// spelling is the manual's own, hyphen and trailing capital A included
// (capability matrix §1.1). "FT-991" IS A DIFFERENT REAL RADIO — an
// earlier Yaesu product this project does not support — so it is never a
// key here and never a fallback for one.
//
// TWO SLUGS EXIST FOR THIS RADIO AND THEY ARE DIFFERENT STRINGS The Go PACKAGE slug is "ft991a" —
// core/driver/ft991a, core/cat/ft991a, internal/fakeft991a, and
// internal/extable's own profile key. ModelSlug(FT991AModel), which is
// what names this radio's snapshot and journal directory, is "ft-991a":
// the hyphen in the model name is collapsed to a separator, not deleted,
// and the trailing A is lowercased with the rest. TestModelSlug pins the
// second (its FT-991A row, added by the fix round that found the claim
// standing without one) and TestModelSlugsUnique pins that it collides
// with no other registered model's — which matters here because "ft-891"
// and "ft-991a" differ only by an inserted "9" and a trailing "a", the
// kind of near-miss a reader skims past.
//
// TWO STATIC BANKS AND NO DISCOVERED BANK AT ALL, which is where this row
// differs from the FT-891's above. MEM ("001".."099") and PMS
// ("100".."117") are declared statically and DENSE, and Open probes
// nothing: this radio's manual describes no 5 MHz bank and no emergency
// channel, so there is nothing to discover and the absence is
// TRANSCRIBED rather than deferred (matrix §3.4). A session's bank list
// is therefore its static capability set exactly.
//
// THE PMS SLOTS ARE THE WIRE NUMBERS "100".."117", not the "P1L".."P9U"
// every registered sibling uses (matrix §1.4.2 and §3.13). This dialect's PMSForm is numeric — the pair number never
// reaches the wire — so those sibling literals are strings this radio's
// own ParseSlot REFUSES. The radio's own MC legend prints the same slots
// as "P-1L".."P-9U", and that divergence is TOLD to the user in
// internal/radiotext's GridLegendNote rather than papered over here.
//
// NO SerialFramingReporter, like the five Yaesu rows above it and unlike
// every Icom row but the IC-7100's: this radio's CAT manual carries no
// serial-framing statement at all, so 8-N-2 for the FT-991A is an ASSUMED
// entry in core/cat/ft991a's own register — FRAMING: 8 DATA BITS, NO
// PARITY, TWO STOP BITS — with a named hardware lift, and the port opens
// at transport.DefaultStopBits by the absence of a report rather than by a
// driver's claim.
const FT991AModel = "FT-991A"

// TS590SModel and TS590SGModel name the TS-590S's and TS-590SG's
// realDrivers/fakeDrivers keys, each of which must equal
// ts590.New(row, ...).Model() for ITS OWN row — pinned, like every constant
// above, by TestDriverTableKeysMatchDriverModel walking both tables on BOTH
// consent arms.
//
// TIER 6's REGISTRATION, and the registry's FIRST KENWOOD ROWS. They are a
// SIBLING PAIR on the FTdx101D/FTdx101MP footing, not two independent
// models: core/driver/ts590 drives both radios from one type and
// core/kw/ts590 holds two layout values, so the pair gets TWO rows here for
// the reason that pair does — this table is keyed by MODEL and a user
// selects a radio, not a family. The spelling is the ID legend's own,
// hyphen included ("021: TS-590S" at 590:1114, "023: TS-590SG" at
// 590:1116; capability matrix §1.1), and unlike the FTdx10's FT-DX10
// near-miss there is no rival spelling to reconcile.
//
// THE ROW IS A REQUIRED ARGUMENT, WHICH IS STRICTER THAN THE FTdx101 PAIR'S
// TWO CONSTRUCTORS. core/driver/ts590 offers ONE New taking the row FIRST
// (ts590.go, `func New(row Row, profile Profile, opts ...Option)`) and the
// zero Row names neither radio: a driver built without one publishes the
// zero capability set and refuses to Open. So a registration that forgot a
// row fails closed rather than silently choosing a sibling — but one that
// passed the WRONG row builds a perfectly valid driver for the other radio,
// which is what TestRealDriverFor_DefaultPathByteIdentical's two rows and
// TestOpenFakeSessionFor_EveryRegisteredModel's identity check exist to
// catch.
//
// TWO SLUGS EXIST HERE TOO, and they differ from the Go package slug in the
// same way the FT-891's do. The PACKAGE slug is "ts590"
// — core/driver/ts590, core/kw/ts590, internal/fakets590, and
// internal/extable's two profile keys "ts590s"/"ts590sg". ModelSlug, which
// names each radio's snapshot and journal directory, gives "ts-590s" and
// "ts-590sg": the hyphen in the model name is collapsed to a separator, not
// deleted. TestModelSlug pins both, and TestModelSlugsUnique pins that
// neither collides — which matters more here than anywhere else in this
// file, "TS-590S" being a strict PREFIX of "TS-590SG".
//
// TWO STATIC BANKS EACH AND NOTHING DISCOVERED. MEM ("000".."099") and SCAN
// ("100L","100U".."109L","109U") are declared statically and DENSE on both
// rows, and NO discovery frame of any kind is ever built (decision 5,
// matrix §3.4): the slot space is fully printed in the book, so a discovery
// walk would be asking a question the manual answers, and the books say the
// NAK is unreliable, so silence would carry no information anyway. The SG's
// 110-119 are NOT published (Stuart decision row 6, plan P11): the printed
// phrase that would make them ordinary records is unconfirmed, and an
// unreached part of a radio is not a bank.
//
// BOTH ROWS IMPLEMENT driver.SerialFramingReporter AND RETURN 1, unlike
// every Yaesu row above and like most Icom rows: these manuals print the
// framing outright (matrix §3.1), so this is documentary rather than
// assumed. stopBitsFor is what carries it to the port, and
// TestStopBitsFor_EveryKenwoodDriverReportsOne is the pin — including for
// the TS-480, which is BUILT AND NOT REGISTERED and has no constant here
// at all. That absence is deliberate: a TS480Model constant added here
// without the rest of its registration would be the beginning of a
// half-registered radio.
const (
	TS590SModel  = "TS-590S"
	TS590SGModel = "TS-590SG"
)

// TS890SModel and TS990SModel name the TS-890S's and TS-990S's
// realDrivers/fakeDrivers keys, each of which must equal ts890.New(...).Model()
// and ts990.New(...).Model() respectively — pinned, like every constant above,
// by TestDriverTableKeysMatchDriverModel walking both tables on BOTH consent
// arms.
//
// TIER 6's SECOND KENWOOD PAIR, AND NOT A SIBLING PAIR. The TS-590 pair above
// is two rows over ONE driver package because one 50-byte MW record serves
// both radios; these two radios' memory records are DIFFERENT SHAPES — a
// 40-to-50-byte MA0 answer with thirteen parameters on the 890S and a 57-byte
// one with eighteen on the 990S — so plan decision P1 gives each its own
// driver package (core/driver/ts890, core/driver/ts990) over one shared codec
// package (core/kw/ma, which holds both layouts). A crossed constructor here
// is therefore a compile error rather than a working driver for the wrong
// radio, which is the one hazard this pair does NOT carry.
//
// THE SPELLINGS COME FROM TWO DIFFERENT PLACES AND THE DIFFERENCE MATTERS.
// "TS-890S" is the ID legend's own, printed with the token beside it ("024:
// TS-890S", 890:2733; capability matrix §1.1). "TS-990S" is NOT printed
// anywhere this project has read: 990:2612 prints the token 022 BARE, with no
// model name against it, so this string is a key this PROJECT mints over a
// documentary silence — a recorded choice (matrix §1.1, plan decision P2), not
// a transcription. core/driver/ts990's own modelName mints the same string,
// and core/driver/ts890's siblingModelName binds 022 to it in a refusal
// message; core/driver/ts590's does the same as of this milestone, which is
// what makes the choice one statement rather than three.
//
// TWO SLUGS EXIST HERE TOO, on the FT-891's and the 590 pair's terms. The
// PACKAGE slugs are "ts890" and "ts990" (core/driver/ts890, internal/fakets890,
// internal/extable's "ts890s"/"ts990s" profile keys — and NOT a package of
// their own under core/kw, where both layouts live in core/kw/ma). ModelSlug,
// which names each radio's snapshot and journal directory, gives "ts-890s" and
// "ts-990s"; TestModelSlug pins both values and TestModelSlugsUnique pins that
// neither collides.
//
// ONE STATIC BANK EACH AND NOTHING DISCOVERED, where the 590 pair has two.
// MEM ("000".."099") is declared statically and DENSE on both rows and no
// discovery frame of any kind is ever built (plan P11, matrix §1.4, §3.4).
// There is no SCAN bank on either row and that is evidential rather than a
// vocabulary limit: neither book prints which parameter selects a section
// channel's start frequency and which its end, so a two-slot-per-index bank
// would be a reading. Slots 100-119 exist in both radios and are published in
// no bank for the same reason.
//
// BOTH ROWS IMPLEMENT driver.SerialFramingReporter AND RETURN 1, like the 590
// pair and unlike every Yaesu row: these manuals print the framing outright
// (matrix §3.1), so it is documentary rather than assumed.
// TestStopBitsFor_EveryKenwoodDriverReportsOne carries all four Kenwood rows.
//
// NEITHER ROW CAN PROGRAMME A BLANK RADIO, and that is the published cost of
// registering them (internal/radiotext's two entries state it to users): one
// MA0 Set carries every field of a channel, so this programme reads the
// channel it is about to write and refuses when the read comes back blank —
// there is no printed frame that creates a channel from nothing.
const (
	TS890SModel = "TS-890S"
	TS990SModel = "TS-990S"
)

// FTdx5000Model names the FTdx5000's realDrivers/fakeDrivers key, which
// must equal ftdx5000.New(...).Model() — pinned, like every other constant
// above, by TestDriverTableKeysMatchDriverModel.
//
// v1.7.0 KENWOOD/YAESU WAVE, TENTH ROW: bare New (single row, own
// document, own package — the ft2000 dialect family's own words, not a
// shared package). 27-byte MR/MW frame, 8-digit FreqHz (Lift Y). NOTAG:
// TagLen 0, no tag/name command in the 20-page manual. CATID "0362"
// (matrix). CTCSSTone is MAPPED (rw) — the live P9 tone-table index —
// unlike every registered 9-digit-family dialect's fixed "00".
//
// NO driver.SerialFramingReporter, like every other Yaesu row.
const FTdx5000Model = "FTdx5000"

// IC7800Model names the IC-7800's realDrivers/fakeDrivers key, which must
// equal ic7800.New(...).Model() — pinned, like every other Icom constant
// above, by TestDriverTableKeysMatchDriverModel walking both tables.
//
// THE v1.7.0 ICOM WAVE's FIRST REGISTRATION, and a single-row one: one
// driver package, one civ.Profile, one fake, on IC7610Model's footing. Its
// own capability review found it a HIGH-proximity clone of the already
// registered IC-7610 (same 25 B record-only / 2 B address, same TagLen 10),
// with one independently-derived wire difference from that sibling: the
// tone_mode/data_mode nibble pair in byte 8 is SWAPPED relative to the
// IC-7610's own (core/driver/ic7800's own FieldSpan). Its own CI-V address
// is 6Ah, distinct from every sibling's.
const IC7800Model = "IC-7800"

// IC7600Model names the IC-7600's realDrivers/fakeDrivers key, which must
// equal ic7600.New(...).Model() — pinned, like every other Icom constant
// above, by TestDriverTableKeysMatchDriverModel walking both tables.
//
// THE v1.7.0 ICOM WAVE's SECOND REGISTRATION, and a single-row one on
// IC7800Model's footing: another literal-copy clone of the IC-7610's 25 B
// / 2 B flat record, with a whole-byte (not nibble-split) SelectByteOffset
// deviation of its own (matrix S3.15(a)) and no change of shape. Its own
// CI-V address is 7Ah.
const IC7600Model = "IC-7600"

// IC7410Model names the IC-7410's realDrivers/fakeDrivers key, which must
// equal ic7410.New(...).Model() — pinned, like every other Icom constant
// above, by TestDriverTableKeysMatchDriverModel walking both tables.
//
// THE v1.7.0 ICOM WAVE's THIRD REGISTRATION, and a single-row one. UNLIKE
// its two siblings above, core/driver/ic7410's New takes NO profile
// argument — the IC-7610's own bare-New shape, with WithSimulatedProfile()
// as the simulated-arm option — so this row's constructor calls read
// differently from IC7800Model's and IC7600Model's. Its own record is 40
// bytes, not 25: this is not a literal IC-7610 clone, and its own capability
// review found a genuinely different TX-duplicate block. Its own CI-V
// address is 80h.
const IC7410Model = "IC-7410"

// IC7700Model names the IC-7700's realDrivers/fakeDrivers key, which must
// equal ic7700.New(...).Model() — pinned, like every other Icom constant
// above, by TestDriverTableKeysMatchDriverModel walking both tables.
//
// THE v1.7.0 ICOM WAVE's FOURTH REGISTRATION, and a single-row one on
// IC7800Model's footing: profile is a positional argument. Its own record
// is 39 bytes — the same record-only length as the already-registered
// IC-7300 — over the same 2-byte flat address, which is why it joins that
// pairing in `indistinguishable` below rather than standing apart the way
// IC7410Model does. Its RX fields match the IC-7610 family exactly; its
// own capability review found a TX-duplicate block reusing existing field
// types rather than introducing new ones. Its own CI-V address is 74h.
const IC7700Model = "IC-7700"

// IC9100Model names the IC-9100's realDrivers/fakeDrivers key, which must
// equal ic9100.New(...).Model() — pinned, like every other Icom constant
// above, by TestDriverTableKeysMatchDriverModel walking both tables.
//
// THE v1.7.0 ICOM WAVE's FIFTH REGISTRATION, and a single-row one on
// IC7800Model's footing. Its own record is 57 bytes over a 3-byte
// AddressFormBankChannel address (band + 2-byte channel) — a length no
// other family in this tier declares, so no new indistinguishable-pair
// declaration is needed. ONE STATIC BANK, MEM: this radio's own
// capability review deferred the optional 4th (1200 MHz) band, whose
// frequency-field encoding the matrix leaves unresolved.
//
// NO driver.SerialFramingReporter, like the IC-7100: this radio's own
// document states no CI-V framing fact, so it opens at
// transport.DefaultStopBits rather than carrying an assumed value. Its
// CI-V address is 7Ch — the matrix's own headline finding, overriding the
// 88h/E0h the dispatch title and spec.md §1 carried — set directly
// (`id.CATID = "7C"`, not the address-plus-token reconstruction the
// IC-7610 family uses), so unlike the IC-7800's and IC-7600's this row
// needed no case correction.
const IC9100Model = "IC-9100"

// IC7200Model names the IC-7200's realDrivers/fakeDrivers key, which must
// equal ic7200.New(...).Model() — pinned, like every other Icom constant
// above, by TestDriverTableKeysMatchDriverModel walking both tables.
//
// THE v1.7.0 ICOM WAVE's SIXTH AND LAST REGISTRATION, on IC7800Model's
// footing. Its own record is 17 bytes over a FLAT address (the matrix's
// own correction of spec.md's superseded 9 B figure) — a length no other
// family in this tier declares. NOTAG (matrix §1 row 6): this radio has
// no channel-name route over CI-V at all, TagLen 0 by declaration, so no
// Tag column is shown for it — the wave's only NoTag row. It implements
// driver.SerialFramingReporter like the tier's other four single-band
// transceivers (8-N-1), unlike the IC-9100's bare-port default. Its CI-V
// address is 76h, reconstructed (address+token), not a static literal,
// so it needed no case correction either.
const IC7200Model = "IC-7200"

// realDrivers is the model-keyed table of real-hardware driver
// constructors: model name -> a constructor building THAT model's
// real-profile driver.Driver. It is the single source of truth
// SupportedModels, OpenRealSessionWith, StaticCapabilities,
// StaticSettingsDescriptor, and SynthesiseDiscoveredBanks all key off —
// adding a radio model to this package means adding one entry here (plus
// fake.go's own table for the simulated/demo path), never touching the
// functions themselves.
//
// The FTdx10 was the first model added that way: this entry, one in
// fake.go, one radiotext entry, and not a line of the functions below.
// Every all-registered-models test in this package walks it by existing.
//
// The FTdx101D and FTdx101MP added the same three things EACH. They are
// SIBLINGS — one driver
// package, one dialect config, one simulator, differing in a name and a CAT
// ID — and they still get two rows here rather than one, because this table
// is keyed by MODEL and a user selects a radio, not a family. Sharing a row
// would mean choosing which sibling a "FTdx101" selection meant, which is
// the choice core/driver/ftdx101 refuses to offer (no bare New) and
// core/cat/ftdx101 refuses to offer (no bare Dialect()).
//
// EACH ROW TAKES THE USER'S CONSENT (the unverified-write-consent
// milestone, task 8) and all four spend it identically: consent false calls
// that model's pinned zero-argument constructor unchanged — so the default
// path is not merely "still working" but byte-identical to the one it
// replaced, pinned by TestRealDriverFor_DefaultPathByteIdentical — and
// consent true builds the SAME profile with that driver package's own
// WithConsentedUnverifiedWrites().
//
// The FT-710's row carries the option too, though its real-hardware
// capability set has no Unverified write left for the transform to touch:
// the option is a proven no-op there (core/driver/ft710's own tests own that
// proof, so this table need not restate it). A row that omitted it would be
// a second shape to reason about for no gain — and one that would quietly
// stop being a no-op the day that radio gained an unverified field.
//
// CONSENT REACHES A SESSION, NEVER A STATIC SURFACE. Every driver's
// WithConsentedUnverifiedWrites leaves its static Capabilities untouched and
// shows up only in the set Open assembles, which is why the three static
// callers below pass false and mean it, and why the option's proof is a
// session-level test (TestOpenRealSessionWith_ConsentedSessionCaps) rather
// than a capability comparison here.
var realDrivers = map[string]func(consent bool) driver.Driver{
	DefaultModel: func(consent bool) driver.Driver {
		if consent {
			return ft710.New(ft710.RealHardware, ft710.WithConsentedUnverifiedWrites())
		}
		return NewRealDriver()
	},
	FTdx10Model: func(consent bool) driver.Driver {
		if consent {
			return ftdx10.New(ftdx10.RealHardware, ftdx10.WithConsentedUnverifiedWrites())
		}
		return ftdx10.New(ftdx10.RealHardware)
	},
	FTdx101DModel: func(consent bool) driver.Driver {
		if consent {
			return ftdx101.NewD(ftdx101.RealHardware, ftdx101.WithConsentedUnverifiedWrites())
		}
		return ftdx101.NewD(ftdx101.RealHardware)
	},
	FTdx101MPModel: func(consent bool) driver.Driver {
		if consent {
			return ftdx101.NewMP(ftdx101.RealHardware, ftdx101.WithConsentedUnverifiedWrites())
		}
		return ftdx101.NewMP(ftdx101.RealHardware)
	},
	IC7610Model: func(consent bool) driver.Driver {
		if consent {
			return ic7610.New(ic7610.RealHardware, ic7610.WithConsentedUnverifiedWrites())
		}
		return ic7610.New(ic7610.RealHardware)
	},
	IC7300Model: func(consent bool) driver.Driver {
		if consent {
			return ic7300.New(ic7300.RealHardware, ic7300.WithConsentedUnverifiedWrites())
		}
		return ic7300.New(ic7300.RealHardware)
	},
	IC7300MK2Model: func(consent bool) driver.Driver {
		if consent {
			return ic7300.NewMK2(ic7300.RealHardware, ic7300.WithConsentedUnverifiedWrites())
		}
		return ic7300.NewMK2(ic7300.RealHardware)
	},
	IC705Model: func(consent bool) driver.Driver {
		if consent {
			return ic705.New(ic705.RealHardware, ic705.WithConsentedUnverifiedWrites())
		}
		return ic705.New(ic705.RealHardware)
	},
	IC9700Model: func(consent bool) driver.Driver {
		if consent {
			return ic9700.New(ic9700.RealHardware, ic9700.WithConsentedUnverifiedWrites())
		}
		return ic9700.New(ic9700.RealHardware)
	},
	IC905Model: func(consent bool) driver.Driver {
		if consent {
			return ic905.New(ic905.RealHardware, ic905.WithConsentedUnverifiedWrites())
		}
		return ic905.New(ic905.RealHardware)
	},
	// TWO ROWS OVER ONE CONSTRUCTOR PAIR, and each row calls its OWN
	// constructor: core/driver/ic7851 offers no bare New, so a
	// registration cannot accidentally wire both models to one row's
	// driver the way a package with a default constructor could.
	// core/driver/ic7851's profile is the RealHardware zero value inside
	// New7851/New7850, so consent is the only option either row passes.
	IC7851Model: func(consent bool) driver.Driver {
		if consent {
			return ic7851.New7851(ic7851.WithConsentedUnverifiedWrites())
		}
		return ic7851.New7851()
	},
	IC7850Model: func(consent bool) driver.Driver {
		if consent {
			return ic7851.New7850(ic7851.WithConsentedUnverifiedWrites())
		}
		return ic7851.New7850()
	},
	// ONE ROW, and it names its profile explicitly: core/driver/ic7760
	// takes the profile as New's first ARGUMENT, so this row reads like
	// the IC-7610's and IC-905's rather than the IC-7851 pair's. The
	// consent arm passes ic7760.RealHardware for the same reason every
	// other profile-argument row does — the option changes the SESSION's
	// effective capabilities and never the profile it was built from.
	IC7760Model: func(consent bool) driver.Driver {
		if consent {
			return ic7760.New(ic7760.RealHardware, ic7760.WithConsentedUnverifiedWrites())
		}
		return ic7760.New(ic7760.RealHardware)
	},
	// ONE ROW, naming its profile explicitly for the IC-7760 row's reason:
	// core/driver/ic7100's New takes the profile as its first ARGUMENT
	// (ic7100.go), so the consent arm passes ic7100.RealHardware rather
	// than leaving the profile to an option's absence. The option changes
	// the SESSION's effective capabilities and never the profile it was
	// built from — TestRealDriverFor_DefaultPathByteIdentical is what
	// would catch a consent arm that had quietly passed ic7100.Simulated.
	IC7100Model: func(consent bool) driver.Driver {
		if consent {
			return ic7100.New(ic7100.RealHardware, ic7100.WithConsentedUnverifiedWrites())
		}
		return ic7100.New(ic7100.RealHardware)
	},
	// ONE ROW, naming its profile explicitly for the IC-7760's and the
	// IC-7100's reason: core/driver/icr8600's New takes the profile as
	// its first ARGUMENT (icr8600.go:41), so the consent arm passes
	// icr8600.RealHardware rather than leaving the profile to an option's
	// absence. The option changes the SESSION's effective capabilities
	// and never the profile it was built from —
	// TestRealDriverFor_DefaultPathByteIdentical is what would catch a
	// consent arm that had quietly passed icr8600.Simulated.
	ICR8600Model: func(consent bool) driver.Driver {
		if consent {
			return icr8600.New(icr8600.RealHardware, icr8600.WithConsentedUnverifiedWrites())
		}
		return icr8600.New(icr8600.RealHardware)
	},
	// ONE ROW, naming its profile explicitly, and for once that is not a
	// choice this table makes among alternatives: core/driver/ft891's New
	// takes the profile as its first ARGUMENT (ft891.go:71,
	// `func New(profile Profile, opts ...Option) driver.Driver`), so the
	// consent arm passes ft891.RealHardware rather than leaving the
	// profile to an option's absence. That driver's zero Profile value IS
	// RealHardware and any unrecognised value fails the same way (its
	// caps.go says so in terms), so a consent arm that had quietly passed
	// ft891.Simulated would not be caught by a fail-safe — it would be
	// caught by TestRealDriverFor_DefaultPathByteIdentical's FT891Model row
	// (both arms pinned there, false and consent), and the session-level
	// consent transform this row feeds is separately pinned by
	// TestOpenRealSessionWith_ConsentedSessionCaps's FT-891 subtest, which
	// is why the profile is named here rather than defaulted.
	FT891Model: func(consent bool) driver.Driver {
		if consent {
			return ft891.New(ft891.RealHardware, ft891.WithConsentedUnverifiedWrites())
		}
		return ft891.New(ft891.RealHardware)
	},
	// ONE ROW, naming its profile explicitly, on exactly the FT-891 row's
	// terms above and for the same reason: core/driver/ft991a's New takes
	// the profile as its first ARGUMENT (ft991a.go's `func New(profile
	// Profile, opts ...Option) driver.Driver`), so the consent arm passes
	// ft991a.RealHardware rather than leaving the profile to an option's
	// absence. That driver's zero Profile value IS RealHardware, so a
	// consent arm that had quietly passed ft991a.Simulated would not be
	// caught by a fail-safe — it would be caught by
	// TestRealDriverFor_DefaultPathByteIdentical's FT991AModel row (both
	// arms pinned there, false and consent), and the session-level consent
	// transform this row feeds is separately pinned by
	// TestOpenRealSessionWith_ConsentedSessionCaps's FT-991A subtest.
	FT991AModel: func(consent bool) driver.Driver {
		if consent {
			return ft991a.New(ft991a.RealHardware, ft991a.WithConsentedUnverifiedWrites())
		}
		return ft991a.New(ft991a.RealHardware)
	},
	// TWO ROWS OVER ONE CONSTRUCTOR, and each names its OWN row
	// explicitly: core/driver/ts590's New takes the ROW as its first
	// argument and the profile as its second, so both arms of both rows
	// have to say which radio they are for. There is no bare New and no
	// default row — the zero Row publishes the zero capability set and
	// refuses to Open — so a forgotten row fails closed; a CROSSED one
	// does not, and TestRealDriverFor_DefaultPathByteIdentical compares
	// each arm against the constructor call the row is supposed to make
	// for exactly that reason.
	//
	// The consent arms name ts590.RealHardware for the reason every
	// profile-argument row above does: the option changes the SESSION's
	// effective capabilities and never the profile it was built from, and
	// this driver's zero Profile IS RealHardware, so a consent arm that
	// had quietly passed ts590.Simulated would be caught by that test
	// rather than by a fail-safe.
	TS590SModel: func(consent bool) driver.Driver {
		if consent {
			return ts590.New(ts590.RowS, ts590.RealHardware, ts590.WithConsentedUnverifiedWrites())
		}
		return ts590.New(ts590.RowS, ts590.RealHardware)
	},
	TS590SGModel: func(consent bool) driver.Driver {
		if consent {
			return ts590.New(ts590.RowSG, ts590.RealHardware, ts590.WithConsentedUnverifiedWrites())
		}
		return ts590.New(ts590.RowSG, ts590.RealHardware)
	},
	// The TS-890S and TS-990S (Tier 6's second pair): ONE package each, so
	// the profile is the only argument and these rows read like the FT-991A's
	// rather than like the 590 pair's above. The consent arms name
	// ts890.RealHardware and ts990.RealHardware explicitly for the reason
	// every profile-argument row here does: the option changes the SESSION's
	// effective capabilities and never the profile it was built from, and
	// each driver's zero Profile IS RealHardware, so a consent arm that had
	// quietly passed the Simulated value would build a driver that hands a
	// real radio the simulator's write-Supported set with nothing failing
	// safe. TestRealDriverFor_DefaultPathByteIdentical compares each arm
	// against the constructor call it is supposed to make.
	TS890SModel: func(consent bool) driver.Driver {
		if consent {
			return ts890.New(ts890.RealHardware, ts890.WithConsentedUnverifiedWrites())
		}
		return ts890.New(ts890.RealHardware)
	},
	TS990SModel: func(consent bool) driver.Driver {
		if consent {
			return ts990.New(ts990.RealHardware, ts990.WithConsentedUnverifiedWrites())
		}
		return ts990.New(ts990.RealHardware)
	},
	// v1.7.0 Kenwood/Yaesu wave, tenth row: bare New takes the profile as
	// its first argument.
	FTdx5000Model: func(consent bool) driver.Driver {
		if consent {
			return ftdx5000.New(ftdx5000.RealHardware, ftdx5000.WithConsentedUnverifiedWrites())
		}
		return ftdx5000.New(ftdx5000.RealHardware)
	},
	// The v1.7.0 Icom wave's first row: profile is a positional argument
	// (ic7800.New(profile, opts...)), on the IC-7760/IC-7100/ICR8600 rows'
	// footing rather than the bare-New ones', so the consent arm names
	// ic7800.RealHardware explicitly.
	IC7800Model: func(consent bool) driver.Driver {
		if consent {
			return ic7800.New(ic7800.RealHardware, ic7800.WithConsentedUnverifiedWrites())
		}
		return ic7800.New(ic7800.RealHardware)
	},
	// The v1.7.0 Icom wave's second row, on IC7800Model's footing.
	IC7600Model: func(consent bool) driver.Driver {
		if consent {
			return ic7600.New(ic7600.RealHardware, ic7600.WithConsentedUnverifiedWrites())
		}
		return ic7600.New(ic7600.RealHardware)
	},
	// The v1.7.0 Icom wave's third row: bare New, on the IC-7610's own
	// footing rather than IC7800Model's/IC7600Model's — no profile argument
	// to pass, since RealHardware is the zero value.
	IC7410Model: func(consent bool) driver.Driver {
		if consent {
			return ic7410.New(ic7410.WithConsentedUnverifiedWrites())
		}
		return ic7410.New()
	},
	// The v1.7.0 Icom wave's fourth row, on IC7800Model's footing.
	IC7700Model: func(consent bool) driver.Driver {
		if consent {
			return ic7700.New(ic7700.RealHardware, ic7700.WithConsentedUnverifiedWrites())
		}
		return ic7700.New(ic7700.RealHardware)
	},
	// The v1.7.0 Icom wave's fifth row, on IC7800Model's footing.
	IC9100Model: func(consent bool) driver.Driver {
		if consent {
			return ic9100.New(ic9100.RealHardware, ic9100.WithConsentedUnverifiedWrites())
		}
		return ic9100.New(ic9100.RealHardware)
	},
	// The v1.7.0 Icom wave's sixth and last row, on IC7800Model's footing.
	IC7200Model: func(consent bool) driver.Driver {
		if consent {
			return ic7200.New(ic7200.RealHardware, ic7200.WithConsentedUnverifiedWrites())
		}
		return ic7200.New(ic7200.RealHardware)
	},
}

// SupportedModels returns every model name this package can open a real
// session against, sorted, so a caller (a CLI listing supported radios, a
// GUI picker) gets deterministic output.
func SupportedModels() []string {
	return slices.Sorted(maps.Keys(realDrivers))
}

// UnknownModelError is returned by every model-keyed lookup in this
// package (OpenRealSessionFor, OpenFakeSessionFor, StaticCapabilities,
// StaticSettingsDescriptor) when model names no registered driver.
// SynthesiseDiscoveredBanks, whose signature carries no error return,
// reports the equivalent condition as its bool false instead.
type UnknownModelError struct {
	// Model is the unrecognised model name the caller asked for.
	Model string
	// Supported is the full list of model names this package DOES
	// support at the time of the failed lookup (SupportedModels()'s own
	// sorted output), so the error message can name what a caller
	// should have asked for instead.
	Supported []string
}

// Error implements the error interface.
func (e *UnknownModelError) Error() string {
	return fmt.Sprintf("wiring: unknown model %q (supported: %s)", e.Model, strings.Join(e.Supported, ", "))
}

// realDriverFor looks model up in realDrivers and constructs its driver at
// the caller's consent, or fails with *UnknownModelError. It is the shared
// entry point every model-keyed real-driver lookup in this file
// (OpenRealSessionWith, StaticCapabilities, StaticSettingsDescriptor,
// SynthesiseDiscoveredBanks) goes through, so "which models this package
// supports" has exactly one answer.
//
// consent is the USER's recorded acceptance of writing this radio's
// unverified fields, threaded through to the driver package's own
// WithConsentedUnverifiedWrites (see realDrivers). It is a plain bool and
// this package reads no store to obtain it: whoever calls decides, and
// nothing here can turn a caller's "no" into a "yes".
func realDriverFor(model string, consent bool) (driver.Driver, error) {
	ctor, ok := realDrivers[model]
	if !ok {
		return nil, &UnknownModelError{Model: model, Supported: SupportedModels()}
	}
	return ctor(consent), nil
}

// RegisterDriverError is registerDriver's typed failure when
// driver.Registry.Register itself refuses d (e.g. a duplicate Model()).
// Error() carries this package's own generic "wiring: ..." wording (this
// package is shared by cmd/rigprog and app/, neither of which this
// package should assume any wording preference for); a caller wanting a
// DIFFERENT wording — e.g. cmd/rigprog's own pre-extraction "cmd/rigprog:
// register driver: ..." text — should errors.As against this and Cause
// rather than relying on Error()'s text (the same pattern internal/csvmerge's
// InventoryMismatchError/UnknownSlotsError use for the identical reason
// — see cmd/rigprog/import.go's mergeCSV/mergeCHIRP aliases).
type RegisterDriverError struct{ Cause error }

func (e *RegisterDriverError) Error() string {
	return fmt.Sprintf("wiring: register driver: %v", e.Cause)
}

func (e *RegisterDriverError) Unwrap() error { return e.Cause }

// registerDriver validates d by registering it into a fresh, throwaway
// driver.Registry — d's Model()/Capabilities().Model agreement,
// Capabilities().Validate, and the ConsentedUnverified-baseline guard all
// run inside Register — wrapping any rejection as *RegisterDriverError.
// The registry itself is discarded: every caller (this file's
// OpenRealSessionWith/StaticCapabilities and fake.go's OpenFakeSessionFor)
// already holds d and uses it directly once registerDriver returns nil,
// so there is nothing left to look up.
func registerDriver(d driver.Driver) error {
	if err := driver.NewRegistry().Register(d); err != nil {
		return &RegisterDriverError{Cause: err}
	}
	return nil
}

// NewRealDriver builds the ft710 driver for a real-hardware session:
// profile ft710.RealHardware, the zero value. It is split out from
// OpenRealSessionFor so the capability set it implies — post-M5b-flip,
// write-capable for EXACTLY the six hardware-verified fields and
// nothing else (ft710.CapabilitiesRealHardware; before the flip,
// nothing writable at all) — can be pinned by a unit test that never
// opens a serial port (see TestNewRealDriver_HWVerifiedWriteSet).
func NewRealDriver() driver.Driver {
	return ft710.New(ft710.RealHardware)
}

// openSerial is OpenRealSessionWith's test seam (and so OpenRealSessionFor's
// too, that being its zero-option delegate): production code always leaves
// this at transport.OpenSerial, and OpenRealSessionWith calls it
// instead of transport.OpenSerial directly. It exists for exactly one
// property — that the baud handed to the serial layer is the DRIVER's
// own Capabilities().DefaultBaud rather than transport's package
// default — which is otherwise inexpressible from this package:
// transport.SerialConfig is carried into an OS-level open, and nothing
// transport exports lets a test read back the config a completed open
// used (transport's own openPort seam is package-private, and a test
// here cannot reach it). A recording seam at THIS call site is therefore
// the only place the disagreement between a driver's DefaultBaud and
// transport.DefaultBaud can be observed at all. Deliberately a
// package-level variable rather than a parameter on either function (or a
// SessionOptions field):
// adding one would put a port-construction hook in the public signature
// of a constructor whose whole shape (see this file's package comment)
// exists to keep invalid profile/port pairings unrepresentable.
var openSerial = transport.OpenSerial

// SessionOptions carries what a caller may vary about a real-hardware
// session beyond the model and the port. It is a struct rather than a
// parameter so that a later option is an added field, not a fifth argument
// at every call site — but it is deliberately NOT a general options bag: it
// may never grow a driver profile or a port object, which would recreate
// exactly the "profile + port" seam this file's structural-exclusivity shape
// exists to rule out.
type SessionOptions struct {
	// ConsentUnverifiedWrites is the USER's recorded acceptance of writing
	// this radio's unverified fields, passed to model's driver as that
	// package's WithConsentedUnverifiedWrites (see realDrivers). FALSE is
	// the zero value and the default, so OpenRealSessionFor's zero-option
	// delegation is the pre-consent behaviour exactly.
	//
	// A BOOL, and this package reads no consent store to fill it: whose
	// consent it is, where it was recorded and whether it is still current
	// are questions for the composition root that owns the user (see
	// internal/userconfig). Wiring's job is to spend the answer, not to
	// find it — which is also why no userconfig import appears in this
	// package.
	ConsentUnverifiedWrites bool
}

// OpenRealSessionWith opens a session against a real radio of model,
// attached at portPath, under opts: a real serial port via
// transport.OpenSerial, paired with model's own real-hardware driver
// constructor from realDrivers, built at opts.ConsentUnverifiedWrites.
//
// It is the ONE real implementation of this package's real-session path, and
// OpenRealSessionFor below is its zero-option delegate — two exported names
// over one body, not two constructors. The structural-exclusivity shape both
// carry is unchanged and is what SessionOptions is bounded by: neither
// function, and no helper either reaches, lets a caller supply a driver
// profile or a port object, so the invalid RealHardware/fakeradio (or
// Simulated/real-port) pairing stays unrepresentable in the code shape
// rather than merely unreached. fake.go's OpenFakeSessionFor is the
// simulated half of that shape and is wholly separate from this one.
//
// CONSENT REACHES THE SESSION ALONE. The option transforms the capability
// set the driver's Open assembles (write-side spec.Unverified becomes
// spec.ConsentedUnverified) and leaves that driver's static Capabilities
// untouched — which is why this function is where the option is proved
// (TestOpenRealSessionWith_ConsentedSessionCaps, over every consent-eligible
// model) and why the static lookups below pass false.
//
// The port is opened at the DRIVER's own factory-default CAT baud
// (Capabilities().DefaultBaud), not at transport's package default. THE SIX
// YAESU VALUES agree with transport's 38400 today — the FT-710's, read off
// hardware; the FTdx10's, the FTdx101D's and the FTdx101MP's, each ASSUMED
// 38400 in its own driver's register entry with its own named per-model
// lift; and Tier 1's FT-891 and FT-991A, ASSUMED on the same footing — so
// for those six this is a no-change derivation. THE ELEVEN ICOM VALUES DO
// NOT: every registered Icom model's DefaultBaud is 19200, and each of
// those is a port opened at a rate transport's default would have got
// wrong. That is the whole point of reading the rate from the radio's own
// table rather than from the FT-710's.
// TestOpenRealSessionFor_BaudFollowsADisagreeingDriver proves the
// derivation with a fixture that actually disagrees, since no registered
// model does.
//
// An unrecognised model fails with *UnknownModelError BEFORE any port is
// touched. transport.OpenSerial (like driver.Driver.Open) owns the port
// it opens on both outcomes: on any error below, the port is already
// closed by whichever call failed, and this function never closes it
// itself.
func OpenRealSessionWith(ctx context.Context, model, portPath string, opts SessionOptions) (driver.Session, func() error, error) {
	d, err := realDriverFor(model, opts.ConsentUnverifiedWrites)
	if err != nil {
		return nil, nil, err
	}

	// registerDriver validates d (its Model()/Capabilities().Model
	// agreement, Capabilities().Validate, the ConsentedUnverified-baseline
	// guard) and wraps any failure as *RegisterDriverError.
	if err := registerDriver(d); err != nil {
		return nil, nil, err
	}

	stopBits, err := stopBitsFor(d)
	if err != nil {
		return nil, nil, err
	}

	port, err := openSerial(portPath, transport.SerialConfig{
		// The baud is the radio's, read from the driver in hand.
		Baud: d.Capabilities().DefaultBaud,
		// The stop bits are the DRIVER's where the driver has something
		// honest to say, and transport's fixed default (8-N-2) where it
		// has not — see stopBitsFor.
		StopBits: stopBits,
	})
	if err != nil {
		return nil, nil, &OpenSerialError{Port: portPath, Cause: err}
	}

	sess, err := d.Open(ctx, port, driver.Identity{Port: portPath})
	if err != nil {
		// Open owns port on both outcomes: it is already closed.
		return nil, nil, &OpenSessionError{Port: portPath, Cause: err}
	}
	return sess, sess.Close, nil
}

// OpenRealSessionFor opens a session against a real radio of model, attached
// at portPath, with NO options: it is OpenRealSessionWith's zero-option
// delegate and has no body of its own, so a caller that has no consent
// decision to express — and every caller written before consent existed —
// gets exactly the pre-consent behaviour, by construction rather than by
// agreement between two implementations
// (TestOpenRealSessionFor_DelegatesZeroOptions pins the sessions equal).
//
// Its signature is deliberately unchanged. The consent-bearing path is a new
// NAME, not a new argument on this one: threading a bool through every
// existing call site would have made every caller state a consent position,
// including the many that have none.
func OpenRealSessionFor(ctx context.Context, model, portPath string) (driver.Session, func() error, error) {
	return OpenRealSessionWith(ctx, model, portPath, SessionOptions{})
}

// StaticCapabilities returns model's static baseline capability
// description — the same value NewRealDriver().Capabilities() reports for
// DefaultModel — via a registry lookup (mirroring OpenRealSessionFor's own
// construction, so Registry.Register's Capabilities().Validate check runs
// here too) plus Driver.Capabilities(). Fails with *UnknownModelError for
// an unrecognised model.
func StaticCapabilities(model string) (spec.Capabilities, error) {
	// consent false: a static surface describes the radio, never a user's consent.
	d, err := realDriverFor(model, false)
	if err != nil {
		return spec.Capabilities{}, err
	}
	if err := registerDriver(d); err != nil {
		return spec.Capabilities{}, err
	}
	return d.Capabilities(), nil
}

// NeedsUnverifiedConsent reports whether model is CONSENT-ELIGIBLE: whether
// its real-hardware baseline still carries a write-side spec.Unverified
// anywhere, and so whether a user's recorded consent could open a write gate
// on it at all. Fails with *UnknownModelError for an unrecognised model, and
// returns false alongside it — a caller that ignored the error must not be
// told an unnameable radio can be consented to.
//
// ONE implementation, exported, because two composition roots ask the same
// question: the CLI's "settings unverified-writes" (which models it lists as
// on/off, and which it refuses a grant for) and the GUI's own consent
// surface. A private copy in either would let the two disagree about which
// radios consent even applies to — an FT-710 owner asked to authorise an
// unverified write that cannot exist, or an FTdx10 owner refused a grant that
// would have unlocked one.
//
// STATIC capabilities, never a session's, and that is what makes the question
// answerable before any port is opened: the consent transform leaves a
// driver's static set untouched (see OpenRealSessionWith), so this predicate
// describes the RADIO — "has this project written to one of these and proved
// it?" — and never a particular user's decision. spec.ConsentedUnverified is
// deliberately not counted: it is what a consented SESSION carries, and it
// cannot appear in a static set at all. Nor is a write-side Unverified on
// spec.FieldErase — see consentCouldUnlockAWrite.
func NeedsUnverifiedConsent(model string) (bool, error) {
	caps, err := StaticCapabilities(model)
	if err != nil {
		return false, err
	}
	return consentCouldUnlockAWrite(caps), nil
}

// consentCouldUnlockAWrite reports whether caps carries a write-side
// spec.Unverified that a grant could actually turn into a permitted write.
//
// spec.FieldErase is SKIPPED, and that is the whole reason this is a named
// predicate rather than an inline loop. spec.ConsentUnverifiedWrites
// structurally exempts FieldErase — it converts every other Unverified
// write label to ConsentedUnverified and leaves erase exactly as it found
// it — so an Unverified erase is not something consent can unlock. Counting
// it would make a radio whose ONLY write-side Unverified sat on erase
// "consent-eligible": its owner would be shown the arming dialogue, asked
// to authorise an unverified write, and (in the GUI) put through a
// disconnect/reconnect to grant something that provably changes nothing.
//
// No registered model has that shape today, which is exactly why the rule
// is written down here and pinned by a fixture
// (TestConsentCouldUnlockAWrite_EraseOnlyIsNotEligible) rather than left to
// be noticed when one arrives.
func consentCouldUnlockAWrite(caps spec.Capabilities) bool {
	for _, b := range caps.Banks {
		for f, fs := range b.Fields {
			if f != spec.FieldErase && fs.Write == spec.Unverified {
				return true
			}
		}
	}
	return false
}

// StaticSettingsDescriptor returns model's driver-level settings tree —
// the driver.StaticSettingsProvider capability (core/driver/optional.go)
// — when model's driver implements it. The bool result reports whether
// the capability is present: false (with a zero SettingsDescriptor and a
// nil error) when model is known but its driver has no settings surface
// at all, distinct from a non-nil error, which means model itself is
// unrecognised.
func StaticSettingsDescriptor(model string) (driver.SettingsDescriptor, bool, error) {
	// consent false: a static surface describes the radio, never a user's consent.
	d, err := realDriverFor(model, false)
	if err != nil {
		return driver.SettingsDescriptor{}, false, err
	}
	prov, ok := d.(driver.StaticSettingsProvider)
	if !ok {
		return driver.SettingsDescriptor{}, false, nil
	}
	return prov.StaticSettingsDescriptor(), true, nil
}

// SynthesiseDiscoveredBanks classifies an offline slot list into the
// read-only banks a live session would have discovered for model, via the
// driver.DiscoveredBankSynthesizer capability (core/driver/optional.go),
// when model's driver implements it. The bool result is false whenever
// the classification did not happen — for BOTH an unrecognised model and
// a known model whose driver lacks the capability — mirroring this
// function's error-free signature (D6/F4): a caller that needs to tell
// the two apart should check model against SupportedModels() itself.
func SynthesiseDiscoveredBanks(model string, slots []string) ([]spec.Bank, bool) {
	// consent false: a static surface describes the radio, never a user's consent.
	d, err := realDriverFor(model, false)
	if err != nil {
		return nil, false
	}
	synth, ok := d.(driver.DiscoveredBankSynthesizer)
	if !ok {
		return nil, false
	}
	return synth.SynthesiseDiscoveredBanks(slots), true
}

// stopBitsFor is spec D3.1's serial-framing rule in one place: consult
// the driver's OPTIONAL driver.SerialFramingReporter, and refuse anything
// it reports other than 1 or 2.
//
// STILL NOT A spec.Capabilities FIELD. The rule — a framing
// field only with hardware evidence — is untouched, and the six
// registered Yaesu models still reach the serial layer at
// transport.DefaultStopBits, because none of them implements the
// interface. What D3.1 adds is somewhere for a driver that DOES have a
// framing fact to put it, and the Icom models are that case — all of them
// but the IC-7100, whose manual states no serial format for the CI-V link
// at all and which therefore implements NOTHING here and reaches the
// serial layer at transport.DefaultStopBits like the Yaesu six
// (core/driver/ic7100/doc.go's framing paragraph).
//
// ZERO IS REFUSED WITH THE REST, and that is the rule's whole substance.
// The obvious implementation — "if the report is <= 0, use the default" —
// is exactly what must not happen: a driver whose StopBits() returns the
// zero value has not asked for 8-N-2, it has failed to answer, and
// substituting a guess there would put transport's default on the wire
// under a driver's authority and with no diagnostic. A driver with
// nothing to say implements NOTHING; a driver that implements this
// interface must mean what it returns.
//
// The refusal happens BEFORE the port is opened, so a misconfigured
// driver never gets as far as touching hardware.
func stopBitsFor(d driver.Driver) (int, error) {
	r, ok := d.(driver.SerialFramingReporter)
	if !ok {
		// The E2-owed FTdx10 verification is CLOSED, and closed as
		// SILENCE: that radio's CAT manual makes no framing statement
		// anywhere, so 8-N-2 for the FTdx10 is an
		// ASSUMED entry in core/driver/ftdx10's own register with a named
		// hardware lift — not a verified fact. Same for the other three.
		return transport.DefaultStopBits, nil
	}
	got := r.StopBits()
	if got != 1 && got != 2 {
		return 0, &UnsupportedStopBitsError{Model: d.Model(), StopBits: got}
	}
	return got, nil
}

// UnsupportedStopBitsError is OpenRealSessionWith's typed refusal when a
// driver's driver.SerialFramingReporter reports a stop-bit count that is
// not 1 or 2. Returned BEFORE any port is opened.
//
// It names the MODEL as well as the number, because the fault is in a
// driver's registration table and the message has to say which driver's.
type UnsupportedStopBitsError struct {
	// Model is the driver that reported StopBits.
	Model string
	// StopBits is what it reported.
	StopBits int
}

func (e *UnsupportedStopBitsError) Error() string {
	return fmt.Sprintf("wiring: driver %s reports %d stop bits; only 1 or 2 are supported (a driver with no framing evidence must not implement driver.SerialFramingReporter at all — zero is not a request for the default)", e.Model, e.StopBits)
}

// OpenSerialError is OpenRealSessionFor's typed failure when
// transport.OpenSerial itself cannot open portPath. See
// RegisterDriverError's doc comment for why Error() carries this
// package's own generic wording, and why a caller wanting different
// wording should use Port/Cause via errors.As instead.
type OpenSerialError struct {
	Port  string
	Cause error
}

func (e *OpenSerialError) Error() string {
	return fmt.Sprintf("wiring: open serial port %q: %v", e.Port, e.Cause)
}

func (e *OpenSerialError) Unwrap() error { return e.Cause }

// OpenSessionError is OpenRealSessionFor's typed failure when the driver's
// own Open fails against an already-open serial port. See
// RegisterDriverError's doc comment.
type OpenSessionError struct {
	Port  string
	Cause error
}

func (e *OpenSessionError) Error() string {
	return fmt.Sprintf("wiring: open session on %q: %v", e.Port, e.Cause)
}

func (e *OpenSessionError) Unwrap() error { return e.Cause }

// modelSlugNonAlnum matches a run of characters ModelSlug does not keep.
var modelSlugNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// ModelSlug turns a model name into a filesystem-safe directory
// component: lowercased, with each run of non-alphanumeric characters
// collapsed to a single "-" (e.g. "FTDX101D/MP" -> "ftdx101d-mp"). Used
// to give each model its own snapshot/journal directory.
func ModelSlug(model string) string {
	return strings.Trim(modelSlugNonAlnum.ReplaceAllString(strings.ToLower(model), "-"), "-")
}

// ResolveSnapshotDir returns the snapshot/journal directory a
// radio-touching composition root should use: override verbatim if
// non-empty, otherwise "<os.UserConfigDir()>/rigprog/snapshots" — the
// same default cmd/rigprog's own resolveSnapshotDir (cmd/rigprog/
// fileio.go) uses, so a GUI-written snapshot/journal and a CLI-written
// one land in the same place absent an override. It does not touch the
// filesystem — callers create the directory (mode 0700) on demand.
//
// model then decides whether that base directory is used directly or
// namespaced (D9): DefaultModel stays at the base directory
// unchanged — byte-identical to the original behaviour — so every
// snapshot written before per-model subdirectories existed is still
// found. Any other model gets its own <base>/<model-slug>/
// subdirectory, applied to an explicit override too, since two models
// sharing one explicitly-named directory is exactly the collision this
// rule exists to prevent. A model whose ModelSlug is "" (no
// alphanumeric characters at all) is refused with an error rather than
// silently falling back to the base directory — filepath.Join drops
// empty elements, so an unguarded empty slug would resolve to exactly
// DefaultModel's own path, the very collision this rule exists to
// prevent.
//
// Deliberately duplicated here rather than exported from cmd/rigprog:
// cmd/rigprog is a cmd-local package app/ must not import; this 3-line
// rule is cheap enough to restate directly rather than force an import
// cmd/rigprog was never meant to expose.
func ResolveSnapshotDir(override, model string) (string, error) {
	base := override
	if base == "" {
		cfgDir, err := os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("wiring: determining default snapshot directory: %w", err)
		}
		base = filepath.Join(cfgDir, "rigprog", "snapshots")
	}
	if model == DefaultModel {
		return base, nil
	}
	slug := ModelSlug(model)
	if slug == "" {
		return "", fmt.Errorf("wiring: resolving snapshot directory: model %q has no filesystem-safe characters to slug — refusing to fall back to the base directory and collide with %s's", model, DefaultModel)
	}
	return filepath.Join(base, slug), nil
}
