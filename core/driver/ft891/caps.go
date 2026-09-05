// SPDX-License-Identifier: GPL-3.0-or-later

package ft891

import (
	"github.com/gm5dna/open-rig-programmer/core/cat"
	// ALIASED deliberately: the dialect package's own name is also
	// "ft891", and an unaliased import would put a second meaning on the
	// spelling this package already answers to. catft891 reads as "the
	// core/cat side of the FT-891", which is exactly what it is, and it
	// appears at ONE call site (catDialect, below).
	catft891 "github.com/gm5dna/open-rig-programmer/core/cat/ft891"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// catDialect is the CAT dialect this driver speaks: the ONE place
// core/driver/ft891 names an instance from core/cat. Everything else here
// derives from it — catID, the capability data's modes and clarifier
// policy, the slot inventories, the MT answer geometry, and (through the
// driver value ft891.go builds) every Session's own codec calls — so the
// package has a single construction site rather than a scatter of
// references for a fourth radio to miss.
//
// It sits beside the capability data it feeds rather than in ft891.go,
// because the package-level construction that runs before any driver value
// exists (catID, the bank slot lists, the mode list) is all here.
// ft891Driver copies it in New, and no METHOD reaches for it: every codec
// call goes through the dialect field the driver or session carries, so a
// hand-built driver with a zero dialect fails closed rather than silently
// borrowing this one (see TestOpen_UnconfiguredDialectRefusesToOpen).
//
// Deliberately a package-level binding rather than a per-driver argument,
// mirroring the FT-710's and FTdx10's reasoning: the FT-891's dialect is
// what makes this package the FT-891's driver.
var catDialect = catft891.Dialect()

// bank60mLabel and bankEMGLabel are the display labels this driver mints
// for the two DISCOVERED banks — both by live discovery
// (effectiveCapabilities) and by offline classification
// (SynthesiseDiscoveredBanks), so the two paths cannot drift apart.
//
// They are THIS package's own consts, not another driver's: the strings
// coincide with its siblings' today because both radios' banks mean the
// same thing to a user and the neutral bank IDs render the same way across
// the app, but a display label is not a protocol fact and nothing forces
// the two to stay equal (matrix §1.4.3, a CHOICE). This radio's own slot
// legend calls these "501 - 510 (5 MHz, U.S. and U.K. version only)" and
// "EMG (Emergency)" (layout 962, 964); "60 m" is the band's amateur name.
const (
	bank60mLabel = "60 m channels"
	bankEMGLabel = "Emergency (EMG)"
)

// memBankLabel and pmsBankLabel are this package's own display labels for
// the two STATIC banks — matrix §1.4.1 and §1.4.2, both marked CHOICE
// there for the same reason as the discovered banks' above.
const (
	memBankLabel = "Memories"
	pmsBankLabel = "Scan limits (PMS)"
)

// writeTrialsComplete is THIS driver's hardware write guard, and it is
// FALSE: no FT-891 has ever been written to by this project (matrix
// §3.11). There is no docs/hardware-notes.md section for this model, no
// write-trial protocol run, and no captured frame from a real one — no
// FT-891 has ever been ASKED anything at all.
//
// While it is false there is no hardware-verified capability profile for
// this driver to select AT ALL — deliberately not even a placeholder one:
// a RealHardware session gets CapabilitiesUnverified (see
// ft891Driver.Capabilities), nothing is writable anywhere, and the
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
// The pin: TestWriteTrialsComplete_PinnedFalse asserts both halves — the
// constant is false, AND the RealHardware baseline is genuinely
// nothing-writable, so a constant-only edit cannot pass while leaving the
// consequence untested.
const writeTrialsComplete = false

// modelName is the FT-891's display name — the driver registry key, and
// necessarily equal to Capabilities().Model (core/driver.Driver's
// contract).
//
// Matrix §1.1: that the radio is named FT-891 is a manual fact (ID's P1
// legend "0650: FT-891" at layout 763, and the cover); the project's
// SPELLING of the registry key is a CHOICE, fixed by the spec's decision 2
// — name "FT-891", package slug ft891, wiring.FT891Model — and not by the
// manual. The registration task's radiotext entry takes this same
// spelling, and its near-miss list keeps "FT891", "ft-891", "FT 891"
// deliberately unknown.
const modelName = "FT-891"

// catID is the identity an FT-891 answers "ID;" with, sourced from the
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
// internal/fakeft891 and about nothing else. Any OTHER unrecognised
// Profile value fails the same way, through Capabilities' explicit default
// arm.
type Profile int

const (
	// RealHardware is the profile for sessions against a physical radio.
	// While writeTrialsComplete is false it selects
	// CapabilitiesUnverified: reads labelled Unverified, every candidate
	// field's Write Unverified, nothing writable.
	RealHardware Profile = iota
	// Simulated is the profile for internal/fakeft891-backed sessions
	// ONLY (the CLI's --fake mode, the GUI's demo mode): Write Supported
	// for the seven fields the combined MT form can express, so the write
	// choreography can be exercised end to end with no hardware at risk.
	Simulated
)

// modeNames returns the selectable mode display names this radio's
// capability data advertises, in wire-code order, DERIVED FROM THE DIALECT
// rather than transcribed here (matrix §1.5).
//
// There is deliberately no local mode table. The FT-710's driver keeps one
// and pins it against its dialect with a test, because it predates the
// dialect seam; a fourth driver copying that shape would copy its one real
// hazard too — a transcription that drifts from the radio's own legend and
// offers a user a mode the radio has never heard of. Enumerating the
// dialect instead makes the drift unrepresentable: the names ARE the
// dialect's, which transcribed them once, from this radio's own three
// identical mode legends (MR's P6 at layout 972-974, MT's at 1007-1010,
// MW's at 1043-1046).
//
// Wire-code order comes free: cat.Mode's underlying value IS the wire
// byte, so ascending byte order is the legend's own order — and this
// radio's legend has a printed HOLE at 'A' ("A: -") and no 'E' or 'F' at
// all, so the walk yields TWELVE names where the FTdx10's yields fifteen.
// The hole needs no special handling here: the dialect simply has no
// member at 'A', and ValidMode says so.
//
// cat.ModeUnset ('0', "-") is excluded explicitly. It is a
// parse-accept-only placeholder that appears in NO FT-891 mode legend (the
// DIALECT register's entry "THE cat.ModeUnset MEMBER OF THE MODE TABLE",
// cited here and not re-registered) and is present in the dialect's table
// only so that parsers may accept it; offering it as a selectable mode
// would invite a user to write a value core/cat refuses to emit.
//
// Nothing on a wire path consults this: the read path renders through the
// session's own dialect (s.dialect.ModeName, read.go) and the write path
// will resolve through dialect.ModeByName.
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
// core/cat/ft891's declared MemoryLo/MemoryHi (1..99, layout 960/998/1035)
// and not a number written out here.
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

// pmsSlots returns the PMS bank's slot inventory, "P1L".."P9U", built
// through the dialect's PMSSlot for the same reason memSlots uses
// MemorySlot: the pair count is the dialect's declared PMSPairs (9, layout
// 961/999/1036), read by walking until it refuses, not restated here.
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

// bankFields builds the per-field support map shared by the MEM and PMS
// banks (matrix §2.7: the memory-channel surface is printed once and
// carries no per-bank qualifier, so one product serves both).
//
// ALL TWENTY-SEVEN spec.Fields are listed explicitly, including the twenty
// that are the zero FieldSupport: a field left out of this map reads
// identically to a field deliberately zeroed (Capabilities.FieldSupport
// returns the zero value for an absent key), and only a written-down zero
// is legible as a decision (matrix §2). The FTdx10's ten-field map is the
// SHAPE precedent and predates the Icom tier; this one lists them all.
//
//   - rw covers the SIX fields the combined MT record expresses and this
//     driver maps in both directions: frequency (P2), mode (P6), CTCSS
//     STATE (P8), shift (P10), the tag (P12) and the display flag (P11).
//
//   - clar covers spec.FieldClarifier separately, purely so the two
//     profiles can differ on it without the rest moving. It is
//     deliberately NOT spec.Inert in any profile: Inert is the FT-710's
//     HARDWARE finding (that radio accepts the clarifier on a write and
//     reads back zeros), and no FT-891 has ever been asked — matrix §2.1,
//     and doc.go's non-borrowing note.
//
//   - spec.FieldTagDisplay is the profile's rw pair, and THIS RADIO IS THE
//     FIRST OF THIS FAMILY FOR WHICH THAT IS TRUE. MT's P11 legend reads
//     `P11 0: TAG "OFF" 1: TAG "ON"` (layout 1016) where every registered
//     combined-form sibling prints "0: (Fixed)", so byte 28 is a LIVE flag
//     the radio reports and the caller must supply — MANUAL-EVIDENCED
//     PRESENCE, matrix §3.7, carried by the dialect as
//     MTPolicy.P11 = cat.P11TagDisplay. The FTdx10's zero here is that
//     radio's manual fact and is not the shape to copy.
//
//   - spec.FieldClarifier is graded normally even though this driver
//     REFUSES a TxClar-true record pre-wire (matrix §2.2): ClarHz, RxClar
//     and TxClar are three Go fields under ONE spec.Field, so grading the
//     field unwritable would block the offset and the RX flag as well, on
//     a radio whose frame carries both. The refusal is an explicit
//     pre-wire check in the write path (write.go's buildWriteCommand),
//     never a capability grade.
//
//   - spec.FieldCTCSSTone and spec.FieldScanSkip are the zero
//     FieldSupport, and the reason is the WEAKER one: the ASSUMED register
//     entry TONE AND SCAN-SKIP UNREACHABILITY (matrix §2.3). The 41-byte
//     record accounts for every one of its positions and none of them is a
//     tone number or a skip flag, and P9 is documented "00: (Fixed)"
//     (layout 1013) — but nothing verifies that no OTHER command could
//     reach a channel's stored tone on this radio, and the FT-710's answer
//     that none can is that radio's hardware finding, not this one's.
//
//   - spec.FieldErase is the zero FieldSupport in both directions, on
//     every bank and profile, and here the reason is STRONG: the Control
//     Command List (layout 111-147) is this radio's entire CAT command set
//     and contains no erase command at all — a MANUAL-EVIDENCED absence
//     (matrix §2.6). Whether some Set frame has an erasing side effect is
//     unknown and deliberately not claimed; Unsupported is the direction
//     that needs no evidence, and it keeps a populated channel going back
//     to empty permanently blocked (codeplug.Diff gates on FieldErase, not
//     on Bank.NoBlank).
//
//   - the seventeen Icom-tier fields are the zero FieldSupport as
//     MANUAL-EVIDENCED ABSENCES FROM THE RECORD (matrix §2.1's rows,
//     §1.18-1.28's reasons). Note two precisions the matrix draws and this
//     comment keeps: this radio HAS DCS (Table 2, layout 432-447) and has
//     no per-channel DCS FIELD, and its IPO is a preamp bypass, not Icom's
//     IP+.
//
// Each call returns a fresh map, so no two banks share one.
func bankFields(rw, clar spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		// The six the record expresses, plus the clarifier beside them.
		spec.FieldFrequency:  rw,
		spec.FieldMode:       rw,
		spec.FieldClarifier:  clar,
		spec.FieldCTCSSState: rw,
		spec.FieldShift:      rw,
		spec.FieldTag:        rw,
		// A LIVE flag on this radio — the inversion against every
		// registered combined-form sibling. See the doc comment.
		spec.FieldTagDisplay: rw,

		// The ASSUMED register's TONE AND SCAN-SKIP UNREACHABILITY entry.
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
// empty ShiftOptions or CTCSSStates fails spec.Validate outright. Where
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
// Banks: MEM "001"-"099" and PMS "P1L"-"P9U", both with NoBlank stated
// FALSE explicitly (see the per-bank comments). NO 5xx or EMG bank: that
// inventory is DISCOVERED per session by Open, never asserted statically —
// the region condition "U.S. and U.K. version only" (layout 962) is a fact
// about which unit is in front of you, which discovery asks (matrix
// §1.4.3, §3.4).
func baseCapabilities(memFields, pmsFields map[spec.Field]spec.FieldSupport) spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	return spec.Capabilities{
		Model: modelName,
		CATID: catID,
		// MANUAL-EVIDENCED (matrix §1.3): a documented transmit surface —
		// TX, MX/MOX, the VOX group and four PTT-select menu rows. The
		// zero value is refused by spec.Validate, so this is a
		// declaration the driver must make.
		Transmit: spec.HasTransmitter,
		Banks: []spec.Bank{
			{
				ID:    spec.BankMemory,
				Label: memBankLabel,
				Slots: memSlots(),
				// NoBlank FALSE, stated (matrix §2.4): an empty memory
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
				// NoBlank FALSE, stated (matrix §2.4): nothing
				// establishes that an FT-891 ships with its PMS pairs
				// populated, and the FT-710's own NoBlank PMS bank was
				// REMOVED at M5b for exactly the failure a wrong guess
				// causes — real radios shipped all-PMS-empty, so
				// codeplug.Validate rejected every real-derived candidate
				// before Diff ever ran, MEM-only edits included. A
				// populated slot going back to empty stays blocked
				// regardless, by FieldErase never being writable.
				NoBlank: false,
				Fields:  pmsFields,
			},
		},
		Modes: modeNames(),
		// TagLen: MANUAL-EVIDENCED (matrix §1.6). P12's legend is "TAG
		// Characters (up to 12 characters) (ASCII)" (layout 1017) and the
		// Set chart draws the field over positions 29-40 — twelve —
		// independently counted twice off 300 dpi renders. The byte the
		// radio PADS a short tag with is the DIALECT's ASSUMED TagFill
		// (its own register entry, MTPolicy.TagFill, cited not restated);
		// the WIDTH is the manual's.
		TagLen: 12,
		// Clarifier policy, from the dialect and ASSUMED IN BOTH HALVES —
		// the DIALECT register's single entry "ClarifierPolicy.StepHz = 10
		// AND ClarifierPolicy.MaxAbsHz = 9990", cited here and
		// deliberately not re-registered as this driver's own. THE MANUAL
		// PRINTS 9999, not 9990, on every block carrying the field (MR
		// 967, MT 1003, MW 1040, IF 781, OI 1126) and states NO step
		// anywhere; 9999 is not a multiple of the inherited 10, so 9990 is
		// the largest multiple of the assumed step inside the printed
		// range — a deduction from an assumption, not a transcription
		// (matrix §1.7, §1.8).
		ClarMaxHz:  9990,
		ClarStepHz: 10,
		// The 50-tone chart, MANUAL-EVIDENCED (matrix §1.9): Table 1,
		// layout 420-429, printed folio 6, entries 000-049, 67.0-254.1 Hz,
		// spot-checked element for element against spec.standardCTCSSTones
		// while the matrix was written. CN's P3 legend points at it ("000
		// - 049: Tone Frequency Number (See Table 1, page 6)", layout
		// 370), which is also why CTCSSToneRange is nil: this radio names
		// a tone by INDEX.
		CTCSSTones: tones[:],
		// Bauds: MANUAL-EVIDENCED (matrix §1.11) — menu row 0506 CAT RATE,
		// legend "0: 4800 bps 1: 9600 bps 2: 19200 bps 3: 38400 bps",
		// layout 553, and the same row in the committed inventory. Four
		// rates, no 115200.
		//
		// DefaultBaud: ASSUMED (matrix §1.12, erratum M-E4), the register
		// entry "DefaultBaud 38400". This manual has NO factory-default
		// column at all — the chart's headers are "P1 | Function | P2 |
		// Digits" (layout 524) and the trailing 1 on line 553 is the
		// DIGITS field, exactly as the generated inventory reads it. The
		// FTdx10 milestone's spec once misread that digit as a default
		// index and concluded 9600; the misreading is recorded so it
		// cannot recur silently here. The legend's first option being 4800
		// is not evidence either — that is the option list's ordering. It
		// matters because internal/wiring's OpenRealSessionFor opens a
		// real radio at exactly this rate and NO baud override exists in
		// the CLI or the GUI.
		Bauds:       []int{4800, 9600, 19200, 38400},
		DefaultBaud: 38400,
		// The NUMBERS are MANUAL-EVIDENCED and this radio is better
		// evidenced here than its siblings: FA's P1 legend "000030000 -
		// 056000000 (Hz)" (layout 702) and FB's identical one (718) are
		// the only frequency range this manual prints anywhere. What is
		// ASSUMED is the step from "the VFO tuning domain" to "the
		// memory-storable domain" — the memory blocks' own legends say
		// only "Frequency (Hz)" over a nine-digit field, which bounds the
		// ENCODING. The register entry "MinFreqHz 30_000 / MaxFreqHz
		// 56_000_000 — the FA/FB RANGE READ AS THE MEMORY-STORABLE RANGE"
		// (matrix §1.13, §1.14). MaxFreqHz must not be left zero — a zero
		// ceiling reads as "unbounded".
		MinFreqHz: 30_000,
		MaxFreqHz: 56_000_000,
		// ASSUMED (matrix §1.15), the register entry RequiredSlots
		// {"001"}: THIS MANUAL STATES NO SUCH RULE ANYWHERE. The FT-710's
		// M-01 is individually required because that radio keeps it
		// populated — an FT-710 hardware fact, not borrowed. Claiming it
		// makes codeplug validation refuse a candidate whose 001 is blank,
		// which is the conservative direction, but it IS a claim.
		RequiredSlots: []string{"001"},
		// The shift and CTCSS-state vocabularies the wire protocol
		// expresses, both MANUAL-EVIDENCED (matrix §1.16, §1.17): P10's
		// "0: Simplex 1: Plus Shift 2: Minus Shift" and P8's "0: CTCSS
		// \"OFF\" 1: CTCSS ENC/DEC 2: CTCSS ENC", each printed identically
		// on all five blocks that carry them. The DISPLAY spellings are
		// the family-wide neutral vocabulary and a CHOICE; the domains and
		// their ordering are the manual's.
		//
		// CT's FOURTH value ("3: DCS \"ON\"", layout 414) is LIVE STATE,
		// not a memory field, and is deliberately not read across: the
		// memory record's P8 has three values and no DCS on all five
		// blocks that print it.
		ShiftOptions: spec.StandardShiftOptions(),
		CTCSSStates:  spec.StandardCTCSSStates(),
	}
}

// CapabilitiesUnverified is the all-Unverified FAIL-SAFE profile, and it is
// what a RealHardware FT-891 session gets today: every field the combined
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
// (sessionCapabilities, ft891.go), and CanWrite is true for that state — so
// a CONSENTED RealHardware session can write, while this static profile is
// untouched and every unconsented session still cannot. The profile keeps
// saying the true thing either way: it describes the EVIDENCE (none), and
// consent is a decision about risk, not evidence. Two guards keep the route
// narrow: the transform never touches FieldErase, and it is skipped
// entirely for an unrecognised Profile.
//
// The READ labels are Unverified rather than Supported for the same reason
// (matrix §2.1, "the honest one"): this driver's read path will have been
// exercised against a fake and a manual, and no FT-891 has ever answered a
// frame.
func CapabilitiesUnverified() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	clar := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	return baseCapabilities(bankFields(rw, clar), bankFields(rw, clar))
}

// CapabilitiesSimulated is the internal/fakeft891-backed profile (CLI
// --fake, GUI demo) and NEVER a real radio: Read AND Write Supported for
// exactly the SEVEN fields the combined MT form can express — frequency,
// mode, clarifier, CTCSS state, shift, tag and the LIVE display flag — on
// MEM and PMS alike.
//
// SEVEN, where the FTdx10's and FTdx101's are six: this radio's P11 is a
// TAG flag rather than a form constant (matrix §3.7).
//
// Against the fake, hardware risk is moot and the write choreography itself
// is what is being exercised end to end, so claiming Supported here is a
// claim about internal/fakeft891 and about nothing else (matrix §2.1).
//
// THE CLARIFIER IS SUPPORTED, NOT Inert. Inert is the FT-710's hardware
// finding about the FT-710; no FT-891 has ever been asked, so there is no
// finding to borrow — see doc.go's non-borrowing note for what a future
// Stage W finding would change, and where.
//
// ctcss_tone, scan_skip, erase and the seventeen Icom-tier fields stay the
// zero FieldSupport, simulator or not: the FORM cannot express them (or,
// for tone and skip, is not known to), and no amount of cooperative fake on
// the other end of the wire changes what the frame has room for.
func CapabilitiesSimulated() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	clar := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	return baseCapabilities(bankFields(rw, clar), bankFields(rw, clar))
}

// readOnlyFields derives a DISCOVERED bank's field map from base's MEM
// bank: Read supports carried through, every Write forced to
// spec.Unsupported, AND spec.FieldTag and spec.FieldTagDisplay forced to
// the ZERO FieldSupport.
//
// THE TAG PAIR IS THIS DRIVER'S OWN RULE AND NOT THE FTdx10'S OR
// FTdx101'S, and the plan (P4) says so in terms. Those drivers carry every
// Read support through unchanged because they read every bank with the
// SAME combined MT frame, which carries the tag. THE FT-891 READS ITS
// DISCOVERED BANKS BY MR ALONE (matrix §2.5, §3.4, §3.5), and MR's Answer
// is 28 positions carrying neither a tag nor a display flag (layout
// 968-975; the geometry witness counted it). So a Read support carried
// through for those two fields would advertise a readable tag on a bank
// whose only frame cannot carry one, and every read of them there reports
// codeplug.Unavailable instead.
//
// THE HONEST READING OF THE ZERO is "this driver's read of this bank
// cannot reach the field", NOT "this radio has no such field". Whether an
// FT-891's 5 MHz channels carry tags at all is not established by this
// manual either way, and nothing here claims they do not.
//
// No profile — not even Simulated — may claim a discovered 5xx/EMG slot
// writable: cat.Dialect's MW policy excludes those slots, its combined-MT
// write policy refuses them too, and on THIS dialect an MC Set of one is
// refused as well (Slots.MCSelects = MCSelectsMemoryPMS, transcribed from
// MC's own legend at layout 907-909), so a Supported label would advertise
// a write the codec will not build. The manual is SILENT on whether an
// FT-891 would accept such a write by some other route; nothing here claims
// it would not.
//
// Each call returns a fresh map.
func readOnlyFields(base spec.Capabilities) map[spec.Field]spec.FieldSupport {
	// THE ok RESULT IS DISCARDED, AND HERE IS WHAT MAKES THAT SAFE: base is
	// always a profile baseline from baseCapabilities, which builds the MEM
	// bank unconditionally as Banks[0], and every caller passes exactly
	// that — effectiveCapabilities' production callers are Open (with
	// d.Capabilities()) and SynthesiseDiscoveredBanks (which re-passes
	// d.Capabilities() on purpose, so live and offline synthesis cannot
	// drift), and the tests take their base from the same place.
	// TestBaseline_Shape asserts Bank(spec.BankMemory) succeeds on both
	// profiles.
	//
	// IF IT EVER DID NOT: mem would be the zero Bank, its Fields nil, the
	// loop below would run zero times, and the discovered bank would ship
	// an EMPTY field map — every field Unsupported for read AND write,
	// since that is spec.FieldSupport's zero. Fail-closed for the write
	// gate, but silently wrong everywhere a field's Read support is
	// consulted, which is why the guarantee is written down rather than
	// trusted.
	mem, _ := base.Bank(spec.BankMemory) // already a defensive copy
	fields := make(map[spec.Field]spec.FieldSupport, len(mem.Fields))
	for f, fs := range mem.Fields {
		fs.Write = spec.Unsupported
		if f == spec.FieldTag || f == spec.FieldTagDisplay {
			// MR carries neither — see the doc comment. The ZERO pair,
			// not merely an unwritable one.
			fs = spec.FieldSupport{}
		}
		fields[f] = fs
	}
	return fields
}

// cloneCapabilities returns a deep copy of caps: Banks (each with fresh
// Slots and Fields) and every other populated slice independently
// allocated, so mutating the copy can never reach the original.
//
// Load-bearing for the write gate, exactly as in the sibling drivers:
// Session.Capabilities hands copies out, and a caller mutating one must
// never alter what WriteChannel enforces.
//
// The twelve deliberately-empty Icom-tier slices are not copied because
// they are nil on this radio in every profile (matrix §1.10, §1.18-1.28)
// and appending nil to nil yields nil — there is nothing to alias. A
// future edit that populated one would have to add its copy here, which is
// what TestCapabilities_EveryFieldExplicit's inverted rule is there to
// prevent happening silently.
func cloneCapabilities(caps spec.Capabilities) spec.Capabilities {
	out := caps
	out.Banks = make([]spec.Bank, 0, len(caps.Banks))
	for _, b := range caps.Banks {
		// Capabilities.Bank returns a defensive copy (fresh Slots and
		// Fields) — reuse that guarantee rather than restating per-field
		// copying here.
		//
		// THE ok RESULT IS DISCARDED, AND HERE IS WHAT MAKES THAT SAFE:
		// b came out of caps.Banks and Bank scans that same slice for
		// b.ID, so the lookup cannot miss. The only way it could return
		// the WRONG bank is a DUPLICATE BankID, and
		// spec.Capabilities.Validate refuses one outright, with
		// TestProfiles_Validate running it over both profiles. That
		// validation covers the BASELINES only, and the load-bearing
		// caller is Session.Capabilities, which passes
		// effectiveCapabilities' output — discovered banks and all. What
		// closes it there is CONSTRUCTION, not validation:
		// effectiveCapabilities appends at most one spec.Bank60m and at
		// most one spec.BankEMG to a baseline holding MEM and PMS, so
		// four distinct IDs at most.
		cp, _ := caps.Bank(b.ID)
		out.Banks = append(out.Banks, cp)
	}
	out.Modes = append([]string(nil), caps.Modes...)
	out.CTCSSTones = append([]spec.Tone(nil), caps.CTCSSTones...)
	out.Bauds = append([]int(nil), caps.Bauds...)
	out.RequiredSlots = append([]string(nil), caps.RequiredSlots...)
	out.ShiftOptions = append([]spec.ShiftOption(nil), caps.ShiftOptions...)
	out.CTCSSStates = append([]spec.ToneState(nil), caps.CTCSSStates...)
	return out
}

// effectiveCapabilities builds a Session's capability set: a deep copy of
// the profile baseline plus one READ-ONLY bank per discovered inventory —
// 60M when any 5xx slot answered, EMG when EMG did, in that fixed order
// (matrix §1.4.3).
//
// Both discovered banks are NoBlank TRUE (matrix §2.4): those channels
// exist because they answered a read, and this driver's CAT surface has no
// way to blank them (no erase command anywhere in the command list, and
// the write policies exclude 5xx/EMG slots outright). That is a statement
// about the write surface this project offers, not about the radio's
// factory contents.
func effectiveCapabilities(base spec.Capabilities, slots60m []string, emg bool) spec.Capabilities {
	caps := cloneCapabilities(base)

	if len(slots60m) > 0 {
		caps.Banks = append(caps.Banks, spec.Bank{
			ID:      spec.Bank60m,
			Label:   bank60mLabel,
			Slots:   append([]string(nil), slots60m...),
			NoBlank: true,
			Fields:  readOnlyFields(base),
		})
	}
	if emg {
		caps.Banks = append(caps.Banks, spec.Bank{
			ID:      spec.BankEMG,
			Label:   bankEMGLabel,
			Slots:   []string{catDialect.EMGSlot().Wire()},
			NoBlank: true,
			Fields:  readOnlyFields(base),
		})
	}
	return caps
}
