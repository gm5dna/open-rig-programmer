// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a

import (
	"github.com/gm5dna/open-rig-programmer/core/cat"
	// ALIASED deliberately: the dialect package's own name is also
	// "ft991a", and an unaliased import would put a second meaning on the
	// spelling this package already answers to. catft991a reads as "the
	// core/cat side of the FT-991A", which is exactly what it is, and it
	// appears at ONE call site (catDialect, below).
	catft991a "github.com/gm5dna/open-rig-programmer/core/cat/ft991a"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// catDialect is the CAT dialect this driver speaks: the ONE place
// core/driver/ft991a names an instance from core/cat. Everything else here
// derives from it — catID, the capability data's modes and clarifier
// policy, the slot inventories, the MT answer geometry, and (through the
// driver value ft991a.go builds) every Session's own codec calls — so the
// package has a single construction site rather than a scatter of
// references for a later reader to miss.
//
// It sits beside the capability data it feeds rather than in ft991a.go,
// because the package-level construction that runs before any driver value
// exists (catID, the bank slot lists, the mode list) is all here.
// ft991aDriver copies it in New, and no METHOD reaches for it: every codec
// call goes through the dialect field the driver or session carries, so a
// hand-built driver with a zero dialect fails closed rather than silently
// borrowing this one (see TestOpen_UnconfiguredDialectRefusesToOpen).
var catDialect = catft991a.Dialect()

// memBankLabel and pmsBankLabel are this package's own display labels for
// the two STATIC banks — matrix §1.4.1 and §1.4.2, both marked CHOICE
// there, because a display label is not a protocol fact and nothing forces
// two radios' to stay equal.
//
// "Scan limits (PMS)" is the fleet's spelling and NOT this radio's own
// word: its MC legend prints these slots as "P-1L" … "P-9U" (layout 916)
// while every surface of this programme shows them as 100-117 (plan P20,
// matrix §3.13). That divergence is TOLD to the user in radiotext's grid
// legend and the release notes rather than hidden, which is task 15b's;
// the label here is deliberately not made to carry it.
const (
	memBankLabel = "Memories"
	pmsBankLabel = "Scan limits (PMS)"
)

// writeTrialsComplete is THIS driver's hardware write guard, and it is
// FALSE: no FT-991A has ever been written to by this project (matrix
// §3.11). There is no docs/hardware-notes.md section for this model, no
// write-trial protocol run, and no captured frame from a real one — no
// FT-991A has ever been ASKED anything at all.
//
// While it is false there is no hardware-verified capability profile for
// this driver to select AT ALL — deliberately not even a placeholder one:
// a RealHardware session gets CapabilitiesUnverified (see
// ft991aDriver.Capabilities), nothing is writable anywhere, and the
// capability gate refuses every write before a frame is built.
//
// It is consulted by no production code, and that is the point. Flipping
// it is a TWO-PART change — this constant AND a CapabilitiesRealHardware
// profile built field class by field class from the trial evidence, AND
// the Capabilities switch rewritten to select it — with the evidence
// linked and the pin test below rewritten so the flip is a visible,
// reviewable test change. Making this constant load-bearing on its own
// would mean a one-character edit could unlock a write.
//
// THE FLIP MUST ALSO RE-PUT MATRIX §7 CELL 7 — the milestone's ruled
// [STUART] decision on writable DCS states — AND THE REGISTER ENTRY
// A DCS-STATE CHANNEL'S CODE SURVIVES A REWRITE TOGETHER (matrix erratum
// M-E12, folding §3.11): cell 7's writable-DCS-state ruling is mitigated
// today only by nothing being written while this constant is false, and
// that entry — a DCS-state channel's code surviving a rewrite — is the
// hazard the mitigation defers, so the DCS-write decision is RE-TAKEN with
// trial evidence in hand at the flip rather than inherited through it
// unexamined. This driver publishes all five P8 values as writable
// (ctcssStates), so the flip is the moment a real FT-991A becomes reachable
// with a '3' or a '4' on the wire and no way to write or read the code it
// implies. The consent option's own comment (ft991a.go) names the hazard;
// this is the checklist a future implementer will actually read.
//
// The pin: TestWriteTrialsComplete_PinnedFalse asserts both halves — the
// constant is false, AND the RealHardware baseline is genuinely
// nothing-writable, so a constant-only edit cannot pass while leaving the
// consequence untested.
const writeTrialsComplete = false

// modelName is the FT-991A's display name — the driver registry key, and
// necessarily equal to Capabilities().Model (core/driver.Driver's
// contract).
//
// Matrix §1.1: that the radio is named FT-991A is a manual fact (ID's P1
// legend "0670: FT-991A" at layout 772); the project's SPELLING of the
// registry key is a CHOICE, fixed by the spec's decision 2 — name
// "FT-991A", package slug ft991a, wiring.FT991AModel — and not by the
// manual.
//
// "FT-991" IS A DIFFERENT REAL RADIO, not a typo of this one, and the
// registration task's near-miss list says so in those terms (plan P15).
// Nothing in this package may treat the shorter spelling as an alias.
//
// The running foot of this radio's own manual is NOT evidence either way:
// the extraction's footers read "FT-991 CAT Operation Reference Book"
// because the printed foot is "FT-991Ⓐ" and pdftotext drops the boxed
// reversed capital A without trace (matrix §1.1; the dialect's provenance
// note, "The dropped Ⓐ").
const modelName = "FT-991A"

// catID is the identity an FT-991A answers "ID;" with, sourced from the
// dialect rather than restated here: one place this string exists, and the
// value the ID probe compares against is the same value the capability
// data advertises. TestCATID_ComesFromTheDialect pins both the linkage and
// the documented literal (matrix §1.2).
var catID = catDialect.CATID()

// Profile selects which capability profile New builds the driver with.
//
// The zero value is RealHardware ON PURPOSE (matrix §2.1): a forgotten or
// zero-valued Profile must fail towards the real-hardware capability set —
// which for this driver is the all-Unverified one, nothing writable — and
// NEVER towards the simulator's, whose Supported writes are a claim about
// internal/fakeft991a and about nothing else. Any OTHER unrecognised
// Profile value fails the same way, through Capabilities' explicit default
// arm.
type Profile = driver.Profile

// RealHardware and Simulated are this package's own names for the shared
// profile constants — AN ALIAS AND UNTYPED RE-DECLARATIONS, never a fresh
// named type. The alias keeps this package's Profile and driver.Profile
// the SAME type, so driver.Base can be embedded, while the selector
// internal/wiring names stays this package's own: TestSimulatedProfile
// TokensConfinement walks for it by package-local name, and that is what
// confines the fake-only profile to one non-test file in the repository.
const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// modeNames returns the selectable mode display names this radio's
// capability data advertises, in wire-code order, DERIVED FROM THE DIALECT
// rather than transcribed here (matrix §1.5).
//
// There is deliberately no local mode table, and on THIS radio that is
// more than the tidiness rule it is elsewhere. Its 'E' is C4FM where the
// FTdx10's is PSK — one nibble, two REAL and different modes — so
// core/cat's package-level Mode.String() fallback is ACTIVELY WRONG here,
// not merely unauthoritative (core/cat/ft991a/doc.go, "The mode fallback is
// WRONG for this radio"). A second transcription in this package would be
// a second place for that divergence to be got wrong.
//
// Wire-code order comes free: cat.Mode's underlying value IS the wire
// byte, so ascending byte order is the legend's own order. This radio's
// five identical legends run '1'…'9' then 'A'…'E' with every nibble named
// — NO hole and NO 'F', where the FTdx10 fills 'F' and the FT-891 prints a
// hole at 'A' — so the walk yields FOURTEEN names.
//
// cat.ModeUnset ('0', "-") is excluded explicitly. It is a
// parse-accept-only placeholder that appears in NO FT-991A mode legend
// (the DIALECT register's entry "THE cat.ModeUnset MEMBER OF THE MODE
// TABLE", cited here and not re-registered) and is present in the
// dialect's table only so that parsers may accept it; offering it as a
// selectable mode would invite a user to write a value core/cat refuses to
// emit.
//
// Nothing on a wire path consults this: the read path renders through the
// session's own dialect (s.dialect.ModeName, read.go) and the write path
// resolves through dialect.ModeByName.
func modeNames() []string {
	var names []string
	for b := 0; b <= 0xFF; b++ {
		m := cat.Mode(byte(b))
		if m == cat.ModeUnset || !catDialect.ValidMode(m) {
			continue
		}
		names = append(names, catDialect.ModeName(m))
	}
	return names
}

// memSlots returns the MEM bank's slot inventory, "001".."099", built
// through the DIALECT's own MemorySlot so the wire forms this capability
// data advertises are the ones that radio's slot space actually accepts —
// never a locally formatted string that ParseSlot might later refuse. The
// range's end is where MemorySlot stops accepting an ordinal, which is
// core/cat/ft991a's declared MemoryLo/MemoryHi (1..99, from the MC block's
// own legend at layout 915) and not a number written out here.
func memSlots() []string {
	var slots []string
	for n := 1; ; n++ {
		s, err := catDialect.MemorySlot(n)
		if err != nil {
			return slots
		}
		slots = append(slots, s.Wire())
	}
}

// pmsSlots returns the PMS bank's slot inventory, "100".."117", built
// through the dialect's PMSSlot — walking the pair number until the
// dialect refuses one, so the pair count is its declared PMSPairs (9, the
// MC legend at layout 916) and not a bound restated here.
//
// THE GENERATION IS LOAD-BEARING ON THIS RADIO IN A WAY IT IS NOT ON ITS
// SIBLINGS, and both the spec (Stage 2 item 7) and the plan (task 10) say
// so by name. core/driver/ftdx10/caps.go:159-174 is the shape, and every
// registered sibling's PMS slot strings are "P1L"…"P9U": under this
// radio's cat.PMSFormNumeric the pair number never reaches the wire at
// all, so the first copy-paste that keeps those literals produces eighteen
// slot strings THIS dialect's ParseSlot refuses — a silent non-slot rather
// than a compile error. TestPMSSlotsAreGeneratedThroughTheDialect asserts
// it in both directions (every advertised slot parses; "P1L" does not),
// and task 15b's PMS-by-string sweep is the fleet-wide half.
func pmsSlots() []string {
	var slots []string
	for pair := 1; ; pair++ {
		lower, err := catDialect.PMSSlot(pair, false)
		if err != nil {
			return slots
		}
		upper, err := catDialect.PMSSlot(pair, true)
		if err != nil {
			// Unreachable: PMSSlot's bound is on the pair, not the half.
			// Refuse to emit a half-pair rather than guess.
			return slots
		}
		slots = append(slots, lower.Wire(), upper.Wire())
	}
}

// toneModes returns THIS RADIO'S OWN FIVE-member P8 vocabulary, and it is
// deliberately NOT spec.StandardToneModes() (matrix §1.17, §3.7).
//
// The memory record's P8 legend prints `0: CTCSS "OFF" 1: CTCSS ENC/DEC
// 2: CTCSS ENC 3: DCS ENC/DEC 4: DCS ENC`, identically on all five blocks
// that carry it — IF 795-796, MR 977-978, MT 1010-1011, MW 1048-1049, OI
// 1128-1129 — where EVERY registered sibling prints 0/1/2 only. That is
// what the dialect carries as cat.ToneStatesCTCSSAndDCS, and this is the
// first Yaesu memory record in this fleet with a DCS state at all.
//
// THE STANDARD THREE ARE A PREFIX OF THESE FIVE, which is exactly why the
// shared helper must not be reached for: a driver that called it would
// pass every length-agnostic check and silently publish a three-state
// vocabulary for a five-state radio. TestToneModes_AreThisRadiosOwnFive
// asserts the negative half as well as the positive one.
//
// THE TWO DCS SPELLINGS ARE NOT FREE. "DCS-ENC-DEC" and "DCS-ENC" are
// already fixed by Stage 0's own tests across four packages —
// core/spec/dcssemantics_test.go, core/codeplug/dcsstate_roundtrip_test.go,
// core/csvio/dcsstate_roundtrip_test.go and app/dcsstate_uispec_test.go —
// so these strings are the ones those tests assert, not a choice made here.
//
// Whether the radio ACCEPTS a DCS state written without a CN code first is
// a different question and is the DIALECT register's "THE DCS STATES' SET
// ACCEPTANCE" entry, cited here and not re-registered. What happens to the
// code the radio already holds for such a channel is this driver's own
// register entry, A DCS-STATE CHANNEL'S CODE SURVIVES A REWRITE.
//
// Each call returns a fresh slice, so no two profiles share one.
func toneModes() []spec.ToneMode {
	return []spec.ToneMode{
		{Value: "OFF", Semantics: spec.ToneModeOff},
		{Value: "ENC-DEC", Semantics: spec.ToneModeCTCSSSquelch},
		{Value: "ENC", Semantics: spec.ToneModeCTCSS},
		// NeedsTxTone/NeedsRxTone report FALSE for both, deliberately
		// (core/spec's own doc comment, matrix §1.17): a DCS state needs
		// a CODE, not a tone, and on this radio the code is not a field
		// of the memory record at all. Reporting true would make
		// core/codeplug's validator demand a Known CTCSSTone for a
		// channel whose tone this programme can never read.
		{Value: "DCS-ENC-DEC", Semantics: spec.ToneModeDCSEncodeDecode},
		{Value: "DCS-ENC", Semantics: spec.ToneModeDCSEncode},
	}
}

// bankFields builds the per-field support map shared by the MEM and PMS
// banks (matrix §2.7: the memory-channel surface is printed once and
// carries no per-bank qualifier, and on this radio that is even more
// literally true than on its siblings, because the slot space is ONE
// contiguous three-digit number line and only the MC legend decomposes it
// at all).
//
// ALL TWENTY-SEVEN spec.Fields are listed explicitly, including the
// twenty-one that are the zero FieldSupport: a field left out of this map
// reads identically to a field deliberately zeroed (Capabilities.FieldSupport
// returns the zero value for an absent key), and only a written-down zero
// is legible as a decision (matrix §2).
//
//   - rw covers the FIVE fields the combined MT record expresses and this
//     driver maps in both directions besides the clarifier: frequency (P2),
//     mode (P6), CTCSS STATE (P8), shift (P10) and the tag (P12).
//
//   - clar covers spec.FieldClarifier separately, purely so the two
//     profiles can differ on it without the rest moving. It is
//     deliberately NOT spec.Inert in any profile: Inert is the FT-710's
//     HARDWARE finding (that radio accepts the clarifier on a write and
//     reads back zeros), and no FT-991A has ever been asked — matrix §2.1.
//     ALL THREE HALVES OF THE FIELD ARE LIVE HERE. ClarHz, RxClar and
//     TxClar are three Go fields under one spec.Field, and unlike the
//     FT-891 this radio prints `P5 0: TX CLAR "OFF" 1: TX CLAR "ON"` on
//     every block carrying the grid (MR 971, MT 1004, MW 1042, IF 787, OI
//     1122), so the TX half is a live state rather than a printed
//     constant. That is matrix erratum M-E3's first half, and it is why
//     write.go has no TxClar pre-wire refusal, where the FT-891's does:
//     under cat.P5TxClar the codec ACCEPTS a TxClar-true record, so
//     copying the FT-891's rung would refuse, on every write, a field this
//     radio's five legends print as live.
//
//   - spec.FieldTagDisplay is the ZERO FieldSupport, and this is the other
//     half of M-E3. MT's P11 legend reads "P11 0: (Fixed)" (layout 1015)
//     where the FT-891 prints `0: TAG "OFF" 1: TAG "ON"`, so byte 28 is
//     SCHEMA on this radio and the dialect carries cat.P11Fixed. A
//     MANUAL-EVIDENCED ABSENCE (matrix §2.3), and the honest reading of the
//     zero is "this radio's memory frame has no display flag" — stronger
//     than the FT-891 matrix's §2.5 zero, which meant only "this driver's
//     read of this bank cannot reach the field".
//
//   - spec.FieldCTCSSTone, spec.FieldToneTx, spec.FieldToneRx,
//     spec.FieldDTCSCode and spec.FieldScanSkip are the zero FieldSupport
//     on the WEAKER ground: three register entries, one per claim (matrix
//     §2.4, split by erratum M-E10 because each has its own lifting
//     capture). TONE-NUMBER UNREACHABILITY covers FieldCTCSSTone,
//     FieldToneTx and FieldToneRx; DCS-CODE UNREACHABILITY covers
//     FieldDTCSCode; SCAN-SKIP UNREACHABILITY covers FieldScanSkip. The
//     41-position record accounts for every one of its positions and none
//     of them is a tone number, a DCS code or a skip flag, and P9 is
//     documented "00: (Fixed)" (layout 1012) — but nothing verifies that no
//     OTHER command could reach a channel's stored tone on this radio, and
//     the FT-710's answer that none can is that radio's hardware finding.
//     spec.FieldDTCSPolarity IS NOT ON THIS LIST and must not be added to
//     it: it is a different claim in kind — a MANUAL-EVIDENCED ABSENCE from
//     the record, graded at matrix §1.20 and covered by the last bullet
//     below — and none of these three entries' captures tests polarity.
//     THIS RADIO'S GAP IS ITS OWN AND IT IS USER-VISIBLE: its P8 can SAY
//     DCS and its record cannot carry the code, so this programme can read
//     and write "this channel uses DCS encode+decode" while being unable to
//     read, write or even display WHICH code.
//
//   - spec.FieldErase is the zero FieldSupport in both directions, on
//     every bank and profile, and here the reason is STRONG: the Control
//     Command List (layout 123-196) is this radio's entire CAT command set
//     and contains no erase command at all — a MANUAL-EVIDENCED absence
//     (matrix §2.6). The nearest things are QI QMB STORE and QR QMB RECALL,
//     which address the quick-memory bank and store rather than erase.
//     Whether some Set frame has an erasing side effect is unknown and
//     deliberately not claimed; Unsupported is the direction that needs no
//     evidence, and it keeps a populated channel going back to empty
//     permanently blocked (codeplug.Diff gates on FieldErase, not on
//     Bank.NoBlank).
//
//   - the seventeen Icom-tier fields are the zero FieldSupport as
//     MANUAL-EVIDENCED ABSENCES FROM THE RECORD (matrix §2.1's rows,
//     §1.18-1.28's reasons). Note two precisions the matrix draws and this
//     comment keeps: this radio HAS DCS polarity (menu 086, layout 622) as
//     a RADIO-LEVEL setting and no per-channel polarity field, and its IPO
//     is a preamp bypass, not Icom's IP+.
//
// Each call returns a fresh map, so no two banks share one.
func bankFields(rw, clar spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		// The five the record expresses, plus the clarifier beside them.
		spec.FieldFrequency:  rw,
		spec.FieldMode:       rw,
		spec.FieldClarifier:  clar,
		spec.FieldCTCSSState: rw,
		spec.FieldShift:      rw,
		spec.FieldTag:        rw,

		// A MANUAL-EVIDENCED ABSENCE on this radio — the inversion of the
		// FT-891's most distinctive cell. See the doc comment.
		spec.FieldTagDisplay: {},

		// The register's TONE-NUMBER UNREACHABILITY and SCAN-SKIP
		// UNREACHABILITY entries respectively — separate entries with
		// separate captures (matrix erratum M-E10).
		spec.FieldCTCSSTone: {},
		spec.FieldScanSkip:  {},
		// No erase command exists in this radio's command set at all.
		spec.FieldErase: {},

		// The Icom-family vocabularies, absent from this record.
		spec.FieldTxFrequency:       {},
		spec.FieldDuplex:            {},
		spec.FieldOffset:            {},
		spec.FieldToneMode:          {},
		spec.FieldToneTx:            {},
		spec.FieldToneRx:            {},
		spec.FieldDTCSCode:          {},
		spec.FieldDTCSPolarity:      {},
		spec.FieldFilter:            {},
		spec.FieldDataMode:          {},
		spec.FieldTuningStepEnabled: {},
		spec.FieldTuningStep:        {},
		spec.FieldProgramTuningStep: {},
		spec.FieldAttenuator:        {},
		spec.FieldPreamp:            {},
		spec.FieldAntenna:           {},
		spec.FieldIPPlus:            {},
	}
}

// baseCapabilities assembles the static baseline both profiles share, with
// the given per-bank field maps.
//
// ALL TWENTY-EIGHT spec.Capabilities fields are populated explicitly —
// sixteen non-zero and twelve deliberately EMPTY (matrix §1) — and
// TestCapabilities_EveryFieldExplicit reflects over the struct to enforce
// both halves. A zero left in one of the sixteen is not a neutral
// omission: a zero MaxFreqHz reads as "no ceiling" to every validator, a
// zero TagLen makes core/csvio's CHIRP import truncate every imported name
// to "", a non-positive Bauds entry reaches SerialConfig.Baud, and an
// empty ShiftOptions or ToneModes fails spec.Validate outright. Where
// the honest value is unverified it is populated anyway and doc.go's
// register carries the provenance (the DefaultBaud 38400,
// MinFreqHz/MaxFreqHz and RequiredSlots entries).
//
// The TWELVE EMPTY ones are not listed here at all, and that is the
// decision rather than an omission: empty is the positive statement "this
// radio expresses no such vocabulary" (matrix §1.10, §1.18-1.28), which is
// what every capability-keyed check in core/codeplug and core/csvio tests
// before it runs, and populating any of them would be the mistake. Each
// one's own reason is recorded at tierFieldsMustBeEmpty in caps_test.go.
//
// Banks: MEM "001"-"099" and PMS "100"-"117", both DENSE and both with
// NoBlank stated FALSE explicitly (see the per-bank comments). NOTHING IS
// DISCOVERED and no 5 MHz or emergency bank is declared statically either
// — "5xx", "5 MHz", "5MHz" and "EMG" appear in no slot legend of this
// manual, checked mechanically over the whole extraction (matrix §1.4.3),
// so this radio's Open probes nothing at all (§3.4). That is this driver's
// largest structural divergence from the FT-891, which spends up to eleven
// exchanges per Open discovering banks this radio does not have.
func baseCapabilities(memFields, pmsFields map[spec.Field]spec.FieldSupport) spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	return spec.Capabilities{
		Model: modelName,
		CATID: catID,
		// MANUAL-EVIDENCED (matrix §1.3): a documented transmit surface —
		// TX (layout 158), MX/MOX (184), VX/VOX (164), PR SPEECH
		// PROCESSOR (131), XT TX CLAR (165), and six menu rows routing PTT
		// or keying to a control line. The zero value is refused by
		// spec.Validate, so this is a declaration the driver must make.
		Transmit: spec.HasTransmitter,
		Banks: []spec.Bank{
			{
				ID:    spec.BankMemory,
				Label: memBankLabel,
				Slots: memSlots(),
				// NoBlank FALSE, stated (matrix §2.5): an empty memory
				// channel is an ordinary state on this radio as on any
				// other, and a NoBlank MEM bank would make
				// codeplug.Validate refuse every candidate with a single
				// blank channel. The one channel this driver claims must
				// stay populated is RequiredSlots' "001" (its own ASSUMED
				// register entry), which is the per-slot mechanism, not
				// the per-bank one.
				NoBlank: false,
				Fields:  memFields,
			},
			{
				ID:    spec.BankPMS,
				Label: pmsBankLabel,
				Slots: pmsSlots(),
				// NoBlank FALSE, stated (matrix §2.5): nothing establishes
				// that an FT-991A ships with its PMS pairs populated, and
				// the FT-710's own NoBlank PMS bank was REMOVED at M5b for
				// exactly the failure a wrong guess causes — real radios
				// shipped all-PMS-empty, so codeplug.Validate rejected
				// every real-derived candidate before Diff ever ran,
				// MEM-only edits included. A populated slot going back to
				// empty stays blocked regardless, by FieldErase never
				// being writable.
				NoBlank: false,
				Fields:  pmsFields,
			},
		},
		Modes: modeNames(),
		// TagLen: MANUAL-EVIDENCED (matrix §1.6). P12's legend is "TAG
		// Characters (up to 12 characters) (ASCII)" (layout 1017) and the
		// Set chart draws the field over positions 29-40 — twelve —
		// counted twice by evidence leg G at 600 dpi. The byte the radio
		// PADS a short tag with is the DIALECT's ASSUMED TagFill (its own
		// register entry, MTPolicy.TagFill = ' ', cited not restated); the
		// WIDTH is the manual's.
		TagLen: 12,
		// Clarifier policy, CONSULTED FROM THE DIALECT and never
		// re-transcribed: the DIALECT register carries ONE entry for the
		// pair, "ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz
		// = 9990", because one capture settles both and neither is
		// readable without the other. This package carries NO literal for
		// either — a bound and its datum living in two places is exactly
		// the drift the FT-891's own closing review (C-H2) found and
		// fixed. THE MANUAL PRINTS 9999, not 9990, on every block carrying
		// the field (IF 785, MR 968, MT 1001, MW 1039, OI 1119) and states
		// NO step anywhere; 9999 is not a multiple of the inherited 10, so
		// 9990 is the largest multiple of the assumed step inside the
		// printed range — a deduction from an assumption, not a
		// transcription (matrix §1.7, §1.8).
		// TestCapabilities_ClarifierDerivesFromDialect pins the identity,
		// not merely the values.
		ClarMaxHz:  catDialect.Clarifier().MaxAbsHz,
		ClarStepHz: catDialect.Clarifier().StepHz,
		// The 50-tone chart, MANUAL-EVIDENCED (matrix §1.9): Table 1,
		// header at layout 419, rows 420-428, entries 000-049,
		// 67.0-254.1 Hz, spot-checked element for element against
		// spec.standardCTCSSTones while the matrix was written. CN's P3
		// legend points at it ("000 - 049: Tone Frequency Number (See
		// Table 1)", layout 369), which is also why CTCSSToneRange is nil:
		// this radio names a tone by INDEX.
		//
		// IT IS A PUBLISHED DOMAIN WITH NO WRITABLE FIELD BEHIND IT
		// (§2.4): the memory record carries no tone number at all, so
		// nothing this driver reads or writes selects from this chart.
		CTCSSTones: tones[:],
		// Bauds: MANUAL-EVIDENCED, FROM MENU 031 ALONE (matrix §1.11, plan
		// P11) — row "031 CAT RATE", legend "0: 4800 bps 1: 9600 bps
		// 2: 19200 bps 3: 38400 bps", layout 561, and the same row in the
		// committed inventory. Four rates, no 115200.
		//
		// MENU 029 IS A DIFFERENT PORT AND IS NOT A SOURCE FOR THIS FIELD.
		// Layout 559 is "029 232C RATE", the RS-232C jack's rate, and that
		// jack is itself gated by "028 GPS/232C SELECT" (558), while the
		// USB jack this driver's sessions are opened on is a separate
		// enumerated device (46-47). Both menus print the same four rates,
		// so nothing changes numerically — but a user sent to 029 sets the
		// wrong port's rate.
		//
		// DefaultBaud: ASSUMED (matrix §1.12), the register entry
		// "DefaultBaud 38400". This manual has NO factory-default column
		// at all — the chart's headers are "P1 | Function | P2 | Digits"
		// (layout 530) and the trailing 1 on line 561 is the DIGITS field,
		// exactly as the generated inventory reads it. The FTdx10
		// milestone's spec once misread that digit as a default index and
		// concluded 9600; the misreading is recorded so it cannot recur
		// silently here. The legend's first option being 4800 is not
		// evidence either — that is the option list's ordering. It matters
		// because internal/wiring's OpenRealSessionWith opens a real radio
		// at exactly this rate and NO baud override exists in the CLI or
		// the GUI.
		//
		// Owners report 38400 as the factory setting (Stuart, 06/09/2026,
		// third-party corroboration) — the value does not change on that
		// evidence, and the label stays ASSUMED until a document or a
		// probe.
		Bauds:       []int{4800, 9600, 19200, 38400},
		DefaultBaud: 38400,
		// The NUMBERS are MANUAL-EVIDENCED: FA's P1 legend
		// "000030000 - 470000000 (Hz)" (layout 699) and FB's identical one
		// (715) are the only frequency range this manual prints in a frame
		// legend, and a grep for either endpoint returns exactly those two
		// lines. What is ASSUMED is the step from "the VFO tuning domain"
		// to "the memory-storable domain" — the memory blocks' own P2
		// legends say only "VFO-A Frequency (Hz)" or "Frequency (Hz)" over
		// a nine-digit field, which bounds the ENCODING and says nothing
		// about what a memory channel will store. The register entry
		// "MinFreqHz 30 000 / MaxFreqHz 470 000 000 — THE FA/FB RANGE READ
		// AS THE MEMORY-STORABLE RANGE" (matrix §1.13, §1.14).
		//
		// Menu row "151 PRESET FREQUENCY" prints the same endpoints in
		// units of 10 Hz (layout 692) and is deliberately NOT used as
		// evidence for these fields: it is the WIRES-X preset, a menu
		// parameter about what the radio tunes.
		//
		// THIS IS THE FIRST REGISTERED YAESU WITH VHF/UHF, more than eight
		// times the FT-891's 56 MHz ceiling, so a fixture reused across
		// the two models is in range here and out of range there — and
		// only one of those directions fails loudly. MaxFreqHz must not be
		// left zero: a zero ceiling reads as "unbounded".
		MinFreqHz: 30_000,
		MaxFreqHz: 470_000_000,
		// ASSUMED (matrix §1.15), the register entry RequiredSlots
		// {"001"}: THIS MANUAL STATES NO SUCH RULE ANYWHERE. The FT-710's
		// M-01 is individually required because that radio keeps it
		// populated — an FT-710 hardware fact, not borrowed. Claiming it
		// makes codeplug validation refuse a candidate whose 001 is blank,
		// which is the conservative direction, but it IS a claim, and it
		// is kept here as the fleet's convention rather than as this
		// manual's statement.
		RequiredSlots: []string{"001"},
		// MANUAL-EVIDENCED (matrix §1.16): P10's "0: Simplex 1: Plus Shift
		// 2: Minus Shift", printed identically on MR 981, MT 1014, MW
		// 1051, IF 799 and OI 1132. The DISPLAY spellings are the
		// family-wide neutral vocabulary and a CHOICE; the three-value
		// domain and its ordering are the manual's.
		//
		// The OS command's own legend prints the same three values on a
		// different command (block 1136-1146, with the footnote "This
		// command can be activated only with an FM mode" at 1143). It is
		// live state, not a memory field, and its footnote is not a
		// constraint on the memory record's P10.
		ShiftOptions: spec.StandardShiftOptions(),
		// THIS RADIO'S OWN FIVE, not the shared three — see toneModes.
		ToneModes: toneModes(),
	}
}

// CapabilitiesUnverified is the all-Unverified FAIL-SAFE profile, and it is
// what a RealHardware FT-991A session gets today: every field the combined
// MT record expresses is labelled Read Unverified / Write Unverified —
// documented in the CAT manual and exercised against scripted peers, but
// never proven against a radio — and every field the record does not
// express stays the zero FieldSupport.
//
// Because Unverified makes FieldSupport.CanWrite false, this profile AS
// LABELLED blocks every write project-wide: codeplug.Diff refuses the
// change, the clone service refuses to execute a plan containing it, and
// Session.WriteChannel re-checks and refuses before building a frame. It is
// also what any UNRECOGNISED Profile value selects — the failure direction
// is always "nothing writable" (matrix §2.1).
//
// THE ONE ROUTE PAST THAT, and it is the user's own: a session opened with
// WithConsentedUnverifiedWrites re-labels these write-side Unverified
// fields spec.ConsentedUnverified at session-capability assembly
// (sessionCapabilities, ft991a.go), and CanWrite is true for that state — so
// a CONSENTED RealHardware session can write, while this static profile is
// untouched and every unconsented session still cannot. The profile keeps
// saying the true thing either way: it describes the EVIDENCE (none), and
// consent is a decision about risk, not evidence. Two guards keep the route
// narrow: the transform never touches FieldErase, and it is skipped
// entirely for an unrecognised Profile.
//
// The READ labels are Unverified rather than Supported for the same reason
// (matrix §2.1, "the honest one"): this driver's read path will have been
// exercised against a scripted peer and a manual, and no FT-991A has ever
// answered a frame.
func CapabilitiesUnverified() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	clar := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	return baseCapabilities(bankFields(rw, clar), bankFields(rw, clar))
}

// CapabilitiesSimulated is the internal/fakeft991a-backed profile (CLI
// --fake, GUI demo) and NEVER a real radio: Read AND Write Supported for
// exactly the SIX fields the combined MT form can express — frequency,
// mode, clarifier, CTCSS state, shift and tag — on MEM and PMS alike.
//
// SIX, where the FT-891's is seven: that radio's P11 is a live TAG flag and
// this radio's is printed "(Fixed)" (matrix §2.3), so there is no display
// field here for any profile to grade.
//
// Against the fake, hardware risk is moot and the write choreography itself
// is what is being exercised end to end, so claiming Supported here is a
// claim about internal/fakeft991a and about nothing else (matrix §2.1).
//
// THE CLARIFIER IS SUPPORTED, NOT Inert, AND ALL THREE OF ITS HALVES ARE
// LIVE. Inert is the FT-710's hardware finding about the FT-710; no FT-991A
// has ever been asked, so there is no finding to borrow. The TX half is a
// printed state on this radio rather than the FT-891's fixed byte (erratum
// M-E3), so nothing here or in the write path refuses a TxClar-true record.
//
// The tag display, the tone number, the DCS code, the scan-skip flag, erase
// and the seventeen Icom-tier fields stay the zero FieldSupport, simulator
// or not: the FORM cannot express them (or, for the tone number, the code
// and the skip flag, is not known to), and no amount of cooperative fake on
// the other end of the wire changes what the frame has room for.
func CapabilitiesSimulated() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	clar := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	return baseCapabilities(bankFields(rw, clar), bankFields(rw, clar))
}
