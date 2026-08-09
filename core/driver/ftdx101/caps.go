// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101

import (
	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// The four bank display LABELS this driver mints. Every one of them is a
// CHOICE — a display string, not a protocol fact — and each is transcribed
// from the M9d-2 capability matrix rather than from the sibling FTdx10
// driver, whose identical strings are that radio's own choice and not
// evidence about this one.
//
//   - bankMemoryLabel / bankPMSLabel are the two STATIC banks' (matrix
//     §1.3.1 and §1.3.2, both "CHOICE").
//   - bank60mLabel / bankEMGLabel are the two DISCOVERED banks' (matrix
//     §1.3.4, "Labels … : CHOICE"). This manual's own words are "5xx (5MHz
//     BAND)" and "EMG (EMERGENCY CH)" (the slot legends at layout
//     1225-1227, 1082-1083, 1278-1279, 1312-1313 and 1436-1437); "60 m" is
//     the band's amateur name.
//
// The discovered pair are consts rather than literals because BOTH the
// live path (effectiveCapabilities, reached from Open) and the offline one
// (SynthesiseDiscoveredBanks) mint them, and two spellings that drifted
// apart would give an offline codeplug different bank headings from a live
// session's.
const (
	bankMemoryLabel = "Memories"
	bankPMSLabel    = "Scan limits (PMS)"
	bank60mLabel    = "60 m channels"
	bankEMGLabel    = "Emergency (EMG)"
)

// writeTrialsCompleteD is the FTDX101D's hardware write guard, and it is
// FALSE: no FTDX101D has ever been written to by this project. There is no
// docs/hardware-notes.md section for that model, no write-trial protocol
// run, and no captured frame from one (matrix §3.11).
//
// While it is false there is no hardware-verified capability profile for a
// D driver to select AT ALL — deliberately not even a placeholder one: a
// RealHardware D session gets capabilitiesUnverified (see
// ftdx101Driver.Capabilities), nothing is writable anywhere, and the
// capability gate refuses every write before a frame is built.
//
// It is consulted by no production code, and that is the point. Flipping it
// is a TWO-PART change — this constant AND a CapabilitiesRealHardware
// profile built field class by field class from the D's OWN trial evidence,
// AND the Capabilities switch rewritten to select it — with the evidence
// linked and TestWriteTrialsComplete_PinnedFalse's D row rewritten so the
// flip is a visible, reviewable test change. Making the constant
// load-bearing on its own would mean a one-character edit could unlock a
// write.
//
// THERE ARE TWO OF THESE, ONE PER MODEL, and the pair is not redundancy.
// The matrix's per-model evidence rule (§3.10, restated at §3.11) is that a
// capture from one model is never evidence about the other, so a D write
// trial must be able to flip the D's guard and NOTHING else: one shared
// constant could not represent a single-model flip, and a D trial that
// flipped the MP's would unlock writes to a radio nobody had asked.
const writeTrialsCompleteD = false

// writeTrialsCompleteMP is the FTDX101MP's hardware write guard, and it is
// FALSE for the same reason and with the same consequences as
// writeTrialsCompleteD: no FTDX101MP has ever been written to by this
// project, there is no docs/hardware-notes.md section for that model, no
// write-trial protocol run and no captured frame from one (matrix §3.11).
//
// Its flip is the MP's own two-part change, justified by an MP trial —
// never by the D's. See writeTrialsCompleteD's doc comment for the shape of
// that change and for why the two constants exist separately.
const writeTrialsCompleteMP = false

// Profile selects which capability profile NewD and NewMP build the driver
// with.
//
// The zero value is RealHardware ON PURPOSE, mirroring the FT-710's and the
// FTdx10's reasoning: a forgotten or zero-valued Profile must fail towards
// the real-hardware capability set — which for this driver is the
// all-Unverified one, nothing writable — and NEVER towards the simulator's,
// whose Supported writes are a claim about internal/fakedx101 and about
// nothing else. Any OTHER unrecognised Profile value fails the same way,
// through Capabilities' explicit default arm.
//
// ONE Profile type for both models, and no model dimension in it: which
// radio a driver is for is fixed by which constructor built it, not by a
// value a caller could get wrong (plan D1).
type Profile int

const (
	// RealHardware is the profile for sessions against a physical radio.
	// While writeTrialsCompleteD and writeTrialsCompleteMP are false it
	// selects the all-Unverified capability set for either model: reads
	// labelled Unverified, every candidate field's Write Unverified,
	// nothing writable.
	RealHardware Profile = iota
	// Simulated is the profile for internal/fakedx101-backed sessions ONLY
	// (the CLI's --fake mode, the GUI's demo mode): Write Supported for the
	// six fields the combined MT form can express, so the write
	// choreography can be exercised end to end with no hardware at risk.
	Simulated
)

// modeNames returns the selectable mode display names one model's
// capability data advertises, in wire-code order, DERIVED FROM THAT MODEL'S
// DIALECT rather than transcribed here (matrix §1.4).
//
// There is deliberately no local mode table. core/cat/ftdx101 transcribed
// the names once, fresh from THIS radio's manual — the mode legend printed
// beside five commands, all five identical and all five running 1 to F
// (MD's P2 at layout 1240-1243, IF's P6 at 1089-1091, MR's P6 at 1286-1288,
// MT's P6 at 1321-1323, MW's P6 at 1361-1363) — and enumerating that table
// makes a transcription drift between driver and dialect unrepresentable
// rather than merely tested for. (A sixth legend exists, OI's P6 at
// 1443-1446, and is a printing defect the dialect records and excludes;
// nothing here re-reads it.)
//
// Wire-code order comes free: cat.Mode's underlying value IS the wire byte,
// so ascending byte order is the legend's own order ('1'-'9' then 'A'-'F').
//
// cat.ModeUnset ('0', "-") is excluded explicitly, which the matrix marks a
// CHOICE and the right one: it is a parse-accept-only placeholder that
// appears in NO FTdx101 mode legend — the DIALECT register's entry "the
// cat.ModeUnset member of the mode table" — and core/cat refuses to emit it
// in any Set frame, so offering it as a selectable mode would invite a user
// to write a value the codec will reject.
//
// Nothing on a wire path consults this: the read path renders through the
// session's own dialect (s.dialect.ModeName, read.go) and the write path
// will resolve through dialect.ModeByName.
func modeNames(d cat.Dialect) []string {
	var names []string
	for b := 0; b <= 0xFF; b++ {
		m := cat.Mode(byte(b))
		if m == cat.ModeUnset || !d.ValidMode(m) {
			continue
		}
		names = append(names, d.ModeName(m))
	}
	return names
}

// memSlots returns the MEM bank's slot inventory, "001".."099", built
// through the DIALECT's own MemorySlot so the wire forms this capability
// data advertises are the ones that radio's slot space actually accepts —
// never a locally formatted string ParseSlot might later refuse. The
// range's end is where MemorySlot stops accepting an ordinal, which is
// core/cat/ftdx101's declared MemoryLo/MemoryHi (1..99, dialect.go:95) and
// not a number written out here.
//
// MANUAL-EVIDENCED (matrix §1.3.1): "001-099 (Memory Channel)" appears in
// all six of this manual's slot legends — MC (layout 1225-1227), IF
// (1082-1083), MR (1278-1279), MT (1312-1313), MW (1353) and OI
// (1436-1437) — none of them model-qualified.
func memSlots(d cat.Dialect) []string {
	var slots []string
	for n := 1; ; n++ {
		s, err := d.MemorySlot(n)
		if err != nil {
			return slots
		}
		slots = append(slots, s.Wire())
	}
}

// pmsSlots returns the PMS bank's slot inventory, "P1L".."P9U", built
// through the dialect's PMSSlot for the same reason memSlots uses
// MemorySlot: the pair count is the dialect's declared PMSPairs (9,
// dialect.go:100), read by walking until it refuses, not restated here.
//
// MANUAL-EVIDENCED (matrix §1.3.2): "P1L -P9U (PMS)" in the same six
// legends. (The chart sets the MC legend with letter tracking, which the
// layout extraction renders as spaced-out characters; core/cat/ftdx101's
// doc.go records that it was read from the rendered page instead.)
func pmsSlots(d cat.Dialect) []string {
	var slots []string
	for pair := 1; ; pair++ {
		lower, err := d.PMSSlot(pair, false)
		if err != nil {
			return slots
		}
		upper, err := d.PMSSlot(pair, true)
		if err != nil {
			// Unreachable: PMSSlot's bound is on the pair, not the half.
			// Refuse to emit a half-pair rather than guess.
			return slots
		}
		slots = append(slots, lower.Wire(), upper.Wire())
	}
}

// bankFields builds the per-field support map shared by the MEM and PMS
// banks, transcribed from the capability matrix's §2.1 table. EVERY
// spec.Field this project models is listed explicitly, including the four
// that are the zero FieldSupport: a field left out of this map reads
// identically to a field deliberately zeroed (Capabilities.FieldSupport
// returns the zero value for an absent key), and only a written-down zero
// is legible as a decision.
//
//   - rw covers the five fields the combined MT record expresses and this
//     driver maps in both directions: frequency (MT P2, positions 6-14),
//     mode (P6 at 22), CTCSS STATE (P8 at 24), shift (P10 at 27) and tag
//     (P12 at 29-40). All five MANUAL-EVIDENCED (matrix §2.1; legends at
//     layout 1311-1330, positions independently counted off 300 dpi raster
//     renders in core/cat/ftdx101/testdata/geometry-witness.csv).
//
//   - clar covers spec.FieldClarifier separately, purely so the two
//     profiles can differ on it without the rest moving. Its POSITIONS are
//     manual-evidenced (P3 sign+magnitude at 15-19, the P4/P5 flags at
//     20-21); its MINUS-DIRECTION BYTE is not, and is the DIALECT
//     register's entry "The CLARIFIER'S MINUS-DIRECTION BYTE, the ASCII
//     HYPHEN-MINUS 0x2D ('-')" — this manual prints that direction as a
//     two-hyphen glyph and the golden deriver recorded it UNREADABLE rather
//     than resolving it. A FieldSupport pair says whether a field is
//     readable and writable, not which byte encodes its sign, so no value
//     here depends on that assumption; it is named so this cell does not
//     read as certifying a byte the dialect records as unread.
//     It is deliberately NOT spec.Inert in any profile: Inert is the
//     FT-710's HARDWARE finding (13/07/2026 — that radio accepted MW frames
//     carrying non-zero clarifier values and read back zeros every time),
//     and neither FTdx101 has ever been asked. See doc.go's non-borrowing
//     note.
//
//   - spec.FieldTagDisplay is the ZERO FieldSupport — Read AND Write
//     Unsupported — on every bank and in every profile, and this is a
//     MANUAL-EVIDENCED ABSENCE rather than an assumption (matrix §3.7): the
//     combined MT record has no display flag. Its P11 is "0: (Fixed)"
//     (layout 1329), a single fixed position at byte 28, and the frame's 41
//     positions are fully accounted for by the independent geometry witness
//     (geometry-witness.csv's MT set/answer rows run P1..P12 and ';' over
//     1-41 with no gap). cat.Dialect.BuildMTSetCombined's signature takes
//     no display argument because there is nowhere to put one. Reads
//     therefore report codeplug.Unavailable (read.go) — "this radio has no
//     such field" — which is a different statement from Unknown.
//
//   - spec.FieldCTCSSTone and spec.FieldScanSkip are the zero FieldSupport
//     for a WEAKER reason, and it is this driver's ASSUMED-register entry 6
//     ("TONE AND SCAN-SKIP UNREACHABILITY", matrix §2.2). What is
//     structural and manual-evidenced is that the combined record accounts
//     for all 41 of its positions, that P9 is documented fixed "00" (layout
//     1326), and that no command this driver sends carries a tone NUMBER or
//     a scan-skip flag for a memory channel at all. What is ASSUMED is the
//     step from that to "these fields are unreachable on this radio":
//     nothing verifies that the CTCSS-state byte means anything live here,
//     and whether some OTHER command in this manual could reach a memory
//     channel's tone number is not established either way. The FT-710's
//     answer — that none can — is that radio's hardware finding and is not
//     borrowed.
//
//   - spec.FieldErase is the zero FieldSupport in both directions, on every
//     bank and profile, and this too is a MANUAL-EVIDENCED ABSENCE (matrix
//     §2.3): the command availability table (layout 236-337) lists this
//     radio's entire CAT command set and contains NO erase command for a
//     memory channel, so there is nothing for this driver to offer.
//     Unsupported is additionally the direction that needs no evidence —
//     whether some Set frame has an erasing side effect here is unknown and
//     deliberately not claimed — and it keeps a populated channel going
//     back to empty permanently blocked (codeplug.Diff gates on FieldErase,
//     not on Bank.NoBlank).
//
// Identical for the D and the MP, and matrix §2.5 is why: the
// memory-channel surface is printed ONCE for both radios with no model
// qualifier anywhere — the MT block and its legends (layout 1311-1345), MR
// (1277-1294), MW (1352-1367), the slot legends, and the Table 2
// applicability sweep's explicit attestation that "NO row's stored
// properties are model-conditional".
//
// Each call returns a fresh map, so no two banks share one.
func bankFields(rw, clar spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:  rw,
		spec.FieldMode:       rw,
		spec.FieldClarifier:  clar,
		spec.FieldCTCSSState: rw,
		spec.FieldShift:      rw,
		spec.FieldTag:        rw,
		// MANUAL-EVIDENCED ABSENCE: no display flag exists in the combined
		// MT record (P11 fixed "0", layout 1329). See the doc comment.
		spec.FieldTagDisplay: {},
		// ASSUMED — this driver's register entry 6.
		spec.FieldCTCSSTone: {},
		spec.FieldScanSkip:  {},
		// MANUAL-EVIDENCED ABSENCE: no erase command exists in this radio's
		// command set at all (availability table, layout 236-337).
		spec.FieldErase: {},
	}
}

// baseCapabilities assembles the static baseline both profiles share for
// ONE model, with the given per-bank field maps.
//
// ALL FIFTEEN spec.Capabilities fields are populated explicitly, each from
// the M9d-2 capability matrix's own §1 entry (cited per field below), and
// TestCapabilities_EveryFieldExplicit reflects over the struct to enforce
// it for both models. A zero left in a capability field is not a neutral
// omission: a zero MaxFreqHz reads as "no ceiling" to every validator
// (core/spec/validate.go:123-125), a zero TagLen makes core/csvio's CHIRP
// import truncate every imported name to nothing and report it as an
// approximated loss rather than refusing (validate.go:178-184), and a
// NON-POSITIVE entry in Bauds would reach SerialConfig.Baud, which
// transport.OpenSerial treats as "unset" and silently replaces with its own
// default (validate.go:126-131). Where the honest value is unverified it is
// populated anyway and doc.go's ASSUMED register carries the provenance,
// PER MODEL.
//
// ONLY TWO VALUES HERE ARE MODEL-CONDITIONAL — Model and CATID — and matrix
// §4 is the sweep that establishes it: Yaesu prints one CAT manual for both
// radios and distinguishes them in exactly three places (the ID answer's P1
// legend, the P4 VALUE ranges of three MAX POWER rows in Table 2, and PC's
// P1 range), of which only the first touches a capability value. Everything
// else below is one value serving two radios because the manual states it
// once, unconditionally, for both.
//
// Banks: MEM "001"-"099" and PMS "P1L"-"P9U", both with NoBlank stated
// FALSE explicitly (see the per-bank comments). NO 5xx or EMG bank: that
// inventory is DISCOVERED per session by Open, never asserted statically —
// see doc.go, "Discovery walks the WHOLE declared range".
func baseCapabilities(m modelParams, memFields, pmsFields map[spec.Field]spec.FieldSupport) spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	return spec.Capabilities{
		// §1.1 — a CHOICE over a manual-evidenced fact. That two models
		// exist and are named FTDX101D and FTDX101MP is the manual's (the
		// ID P1 legend, layout 1070 and 1072); the project's SPELLING of
		// the registry key is the spec's choice, and the manual sets both
		// names in full capitals throughout. Model must equal the registry
		// key and must differ between the two registrations, so it is
		// model-conditional by construction — it comes from modelParams.
		Model: m.name,
		// §1.2 — MANUAL-EVIDENCED, and the one GENUINE D-vs-MP divergence
		// in the whole matrix: ID's P1 legend prints "0681: FTDX101D" and
		// "0682: FTDX101MP" (layout 1070 and 1072, printed page 14; the
		// frame block at 1069-1078). Sourced from the DIALECT rather than
		// restated, so the string the ID probe compares against is the
		// string the capability data advertises — ONE source per model.
		CATID: m.dialect.CATID(),
		Banks: []spec.Bank{
			{
				ID: spec.BankMemory,
				// §1.3.1 — CHOICE. A display string, not a protocol fact.
				Label: bankMemoryLabel,
				Slots: memSlots(m.dialect),
				// §1.3.1 — NoBlank FALSE, stated: CHOICE, conservative.
				// Nothing in this manual says a memory channel must stay
				// populated, and a NoBlank MEM bank would make
				// codeplug.Validate refuse every candidate with a single
				// blank channel. The one channel this driver claims must
				// stay populated is RequiredSlots' "001" (§1.13), which is
				// the per-SLOT mechanism, not the per-BANK one; asserting
				// NoBlank here in order to protect 001 would be the M5b
				// failure repeated.
				NoBlank: false,
				Fields:  memFields,
			},
			{
				ID: spec.BankPMS,
				// §1.3.2 — CHOICE.
				Label: bankPMSLabel,
				Slots: pmsSlots(m.dialect),
				// §1.3.2 — NoBlank FALSE, stated: CHOICE, conservative, and
				// deliberately NOT the FT-710's original guess. Nothing
				// establishes that an FTdx101 of either model ships with its
				// PMS pairs populated, and the FT-710's own NoBlank PMS bank
				// was REMOVED at M5b for exactly the failure a wrong guess
				// here causes: real radios shipped all-PMS-empty, so
				// codeplug.Validate rejected every real-derived candidate
				// before Diff ever ran, including MEM-only edits. A
				// populated PMS slot going back to empty stays blocked
				// regardless, by FieldErase never being writable (§2.3).
				NoBlank: false,
				Fields:  pmsFields,
			},
		},
		// §1.4 — MANUAL-EVIDENCED, fifteen modes in wire-code order '1'..'F',
		// enumerated from this model's dialect (see modeNames).
		Modes: modeNames(m.dialect),
		// §1.5 — MANUAL-EVIDENCED: MT's P12 legend, "TAG Characters (up to
		// 12 characters) (ASCII)" (layout 1330, printed page 16),
		// unconditional for both models, and independently confirmed as
		// positions 29-40 by the geometry witness counted off 300 dpi raster
		// renders. The dialect's MTPolicy.TagMaxBytes is 12 to match. The
		// byte the radio PADS a short tag with is a separate question and is
		// the DIALECT register's entry "MTPolicy.TagFill = ' '"; TagLen does
		// not depend on it.
		TagLen: 12,
		// §1.6 — MANUAL-EVIDENCED: "Clarifier Offset: 0000 - 9990 (Hz)" in
		// five frame blocks (IF at layout 1086, MR 1282, MT 1317, MW 1357,
		// OI 1440) and both dedicated clarifier commands agree (RD's P1 at
		// 1602, RU's P1 at 1700). Seven unconditional statements.
		ClarMaxHz: 9990,
		// §1.7 — ASSUMED: NO step is stated anywhere in this manual. The
		// 0000-9990 range supports the inherited value without proving it (a
		// 20 Hz radio could not reach its own stated 9990; a 1 Hz one would
		// be free to stop at 9999 and does not). The entry lives in the
		// DIALECT register, as "ClarifierPolicy.StepHz = 10" — cited here
		// and deliberately NOT re-registered as this driver's own, because
		// correcting it is a dialect change. core/cat enforces it as a
		// multiple-of-step rule on every MW and combined-MT Set, so it is
		// load-bearing on the write path even while nothing may be written.
		ClarStepHz: 10,
		// §1.8 — MANUAL-EVIDENCED: this manual prints its own "Table 1
		// (CTCSS Tone Chart)" (heading at layout 566, body 567-575, printed
		// page 8), reached from CN's P3 legend "000 - 049: Tone Frequency
		// Number (See Table 1)" (layout 559). It was compared element by
		// element, all fifty, against spec.standardCTCSSTones while the
		// matrix was written: every index 000-049 agrees, 67.0 Hz at 000
		// through 254.1 Hz at 049, with no insertion, omission or
		// reordering. The array index IS the CAT tone number P3.
		//
		// This describes the vocabulary the radio's tone chart uses; it does
		// NOT claim a memory record can carry a tone number, which is
		// FieldCTCSSTone's zero FieldSupport (§2.2) and a separate question.
		CTCSSTones: tones[:],
		// §1.9 — MANUAL-EVIDENCED: menu item (03,01,11) CAT RATE, P4 legend
		// "0: 4800 bps 1: 9600 bps 2: 19200 bps 3: 38400 bps" at layout 863
		// (printed page 11), committed at core/cat/ftdx101/table2.csv and
		// generated into that package's exinventory_gen.go. FOUR rates, no
		// 115200. These are the CAT menu's rates — this radio has a SECOND
		// rate menu, (03,01,09) 232C RATE at layout 861, for the rear
		// RS-232C jack; the two legends print the same four rates, and Bauds
		// describes the CAT/USB path this project opens (§1.9, §3.12).
		Bauds: []int{4800, 9600, 19200, 38400},
		// §1.10 — ASSUMED, and NOT covered by the citation one line above:
		// the factory default is not in this CAT manual at all. Table 2's
		// column headers are "P1 P2 P3 Function P4 Digits" (layout 716) —
		// there is no factory-default column, and the trailing 1 on layout
		// 863 is the DIGITS field, exactly how the committed inventory reads
		// it. This driver's ASSUMED-register entry 3 carries the
		// provenance and the per-model lift; it matters operationally
		// because internal/wiring's OpenRealSessionFor opens a real radio at
		// exactly this DefaultBaud.
		DefaultBaud: 38400,
		// §1.11 and §1.12 — ASSUMED, one register entry (4) covering the
		// pair: this manual carries NO storable-frequency range statement.
		// Every frequency legend says only "VFO-A Frequency (Hz)" or
		// "Frequency (Hz)" over a 9-digit field, which bounds the ENCODING
		// and says nothing about what either radio will store. BS (BAND
		// SELECT)'s P1 legend (layout 506-514) enumerates band BUTTONS and a
		// general-coverage position and is NOT evidence for these fields —
		// reading its 70 MHz entry as a MaxFreqHz would be inventing a
		// number. MaxFreqHz is the ledgered dangerous-zero field (a zero
		// reads as "no ceiling"), so it MUST be populated.
		MinFreqHz: 30_000,
		MaxFreqHz: 75_000_000,
		// §1.13 — ASSUMED (register entry 5): that memory channel 001 must
		// never be empty. This manual states no such rule for either model
		// anywhere. Claiming it makes codeplug validation refuse a candidate
		// whose 001 is blank, which is the conservative direction, but it IS
		// a claim. Per-SLOT, and the only slot claimed; the per-BANK
		// mechanism is NoBlank, false on both static banks above.
		RequiredSlots: []string{"001"},
		// §1.14 — MANUAL-EVIDENCED vocabulary, CHOICE of order: the P10
		// legend "0: Simplex 1: Plus Shift 2: Minus Shift" is printed in
		// five frame blocks (IF at layout 1097, MR 1294, MT 1327, MW 1367,
		// OI 1452), all identical and none model-qualified. Three values,
		// exactly the three spec.StandardShiftOptions() carries; the display
		// order is the shared standard's, which happens to match the
		// legend's own 0/1/2.
		ShiftOptions: spec.StandardShiftOptions(),
		// §1.15 — MANUAL-EVIDENCED vocabulary, CHOICE of display spellings
		// and order: the P8 legend is printed in five frame blocks, none
		// model-qualified — MR (layout 1291), MT (1325), MW (1365) and OI
		// (1449) print "0: CTCSS \"OFF\" 1: CTCSS ENC/DEC 2: CTCSS ENC", and
		// IF (1095) the same three values with its off state abbreviated.
		// The difference is typographic; same three values, same three
		// indices. The project's spellings ("ENC-DEC") and their order are
		// the shared standard's, not the manual's punctuation.
		//
		// This describes the vocabulary the WIRE protocol expresses. That
		// the state byte does anything live on either radio is unverified —
		// see register entry 6 — and FieldCTCSSState's support level, not
		// this list, is where that caution is expressed.
		CTCSSStates: spec.StandardCTCSSStates(),
	}
}

// capabilitiesUnverified is the all-Unverified FAIL-SAFE profile for one
// model, and it is what a RealHardware FTdx101 session of that model gets
// today: every field the combined MT record expresses is labelled Read
// Unverified / Write Unverified — documented in the CAT manual and
// exercised against scripted peers, but never proven against a radio — and
// every field the record does not express stays the zero FieldSupport
// (matrix §2.1's profile table).
//
// Because Unverified makes FieldSupport.CanWrite false, this profile blocks
// every write project-wide: codeplug.Diff refuses the change, the clone
// service refuses to execute a plan containing it, and Session.WriteChannel
// re-checks and refuses before building a frame. It is also what any
// UNRECOGNISED Profile value selects — the failure direction is always
// "nothing writable".
//
// The READ labels are Unverified rather than Supported for the same reason,
// and the matrix calls it the honest choice: this driver's read path has
// been exercised against a fake and a manual, and NO FTdx101 OF EITHER
// MODEL HAS EVER ANSWERED A FRAME.
//
// UNEXPORTED, unlike the FTdx10 driver's CapabilitiesUnverified: this
// package's profiles are per MODEL, and modelParams is deliberately
// unexported (plan D1 — no exported model enum, whose zero value would need
// its own fail-safe arm). Every consumer outside this package reaches these
// values through NewD/NewMP and driver.Driver.Capabilities(), which is the
// same route internal/wiring's StaticCapabilities already takes for every
// registered model.
func capabilitiesUnverified(m modelParams) spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	clar := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	return baseCapabilities(m, bankFields(rw, clar), bankFields(rw, clar))
}

// capabilitiesSimulated is the internal/fakedx101-backed profile (CLI
// --fake, GUI demo) and NEVER a real radio: Read AND Write Supported for
// exactly the SIX fields the combined MT form can express — frequency,
// mode, clarifier, CTCSS state, shift and tag — on MEM and PMS alike
// (matrix §2.1's profile table).
//
// Against the fake, hardware risk is moot and the write choreography itself
// is what is being exercised end to end, so claiming Supported here is a
// claim about internal/fakedx101 and about nothing else.
//
// THE CLARIFIER IS SUPPORTED, NOT Inert. Inert is the FT-710's hardware
// finding about the FT-710, and neither FTdx101 has ever been asked, so
// there is no finding to record and borrowing one would be answering a
// question about one radio with another radio's evidence. See doc.go's
// non-borrowing note for what a future Stage W finding would change, and
// where.
//
// tag_display, ctcss_tone, scan_skip and erase stay the zero FieldSupport,
// simulator or not: the FORM cannot express them (or, for tone and skip, is
// not known to), and no amount of cooperative fake on the other end of the
// wire changes what the frame has room for.
func capabilitiesSimulated(m modelParams) spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	clar := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	return baseCapabilities(m, bankFields(rw, clar), bankFields(rw, clar))
}

// readOnlyFields derives a DISCOVERED bank's field map from base's MEM
// bank: Read supports carried through unchanged (whatever the profile says
// about reading), every Write forced to spec.Unsupported.
//
// No profile — not even Simulated — may claim a discovered 5xx or EMG slot
// writable, and the two halves of that have DIFFERENT evidence statuses.
// The matrix (§1.3.5) requires both to be stated rather than compressed
// into one clause the way the FTdx10 driver's equivalent comment does:
//
//   - MW cannot address 5xx or EMG: MANUAL-EVIDENCED FOR THIS RADIO. This
//     manual's MW slot legend gives only "001-099 (Memory Channel), P1L
//     -P9U (PMS)" (layout 1353) where MT's gives the full vocabulary
//     including "5xx (5MHz BAND), EMG (EMERGENCY CH)" (1312-1313), and
//     MW's legend is unambiguously the Set direction's P1.
//     cat.Dialect.writableSlot excludes those slots from MW accordingly.
//     (Was "cat.Slot.Writable (core/cat/slot.go:159-162)" — the symbol was
//     removed at M9d and the line numbers were already stale; the
//     replacement is named without them on purpose.)
//
//   - MT cannot address 5xx or EMG: PROJECT POLICY, not a manual fact for
//     this radio. core/cat's combined-MT write policy refuses them
//     (Dialect.mtSlotValid, core/cat/mt.go:115, reached by
//     validateCombinedMTFields at core/cat/mtcombined.go:105), and mt.go's
//     own doc comment (core/cat/mt.go:103-106) says so in terms: "our
//     policy: reject sets to 5xx/EMG until hardware-verified". The gloss
//     that follows there — a project decision, not a manual requirement —
//     is mt.go's own, and sits OUTSIDE that quotation in mt.go, where the
//     quoted words are the reference's and the parenthesis is the
//     project's. That policy was adopted
//     against the FT-710's manual; the FTdx101's own MT legend is headed
//     "P0/1", merging the Read direction's slot parameter with the Set
//     direction's under one vocabulary, so this manual does not separately
//     state that an MT Set may address 5xx or EMG — but nothing in it
//     requires the refusal either.
//
// So the read-only discovered banks are correct and conservative, and half
// the reason is a policy this project could revisit with hardware while the
// other half is the radio's documented restriction. Each call returns a
// fresh map.
func readOnlyFields(base spec.Capabilities) map[spec.Field]spec.FieldSupport {
	mem, _ := base.Bank(spec.BankMemory) // already a defensive copy
	fields := make(map[spec.Field]spec.FieldSupport, len(mem.Fields))
	for f, fs := range mem.Fields {
		fs.Write = spec.Unsupported
		fields[f] = fs
	}
	return fields
}

// cloneCapabilities returns a deep copy of caps: Banks (each with fresh
// Slots and Fields) and every other slice independently allocated, so
// mutating the copy can never reach the original.
//
// Load-bearing for the write gate, exactly as in the FT-710's and FTdx10's
// drivers: Session.Capabilities hands copies out, and a caller mutating one
// must never alter what WriteChannel enforces.
func cloneCapabilities(caps spec.Capabilities) spec.Capabilities {
	out := caps
	out.Banks = make([]spec.Bank, 0, len(caps.Banks))
	for _, b := range caps.Banks {
		// Capabilities.Bank returns a defensive copy (fresh Slots and
		// Fields) — reuse that guarantee rather than restating per-field
		// copying here.
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
// 60M when any 5xx slot answered, EMG when EMG did, in that fixed order.
//
// The EMG bank's wire form comes from the DIALECT passed in (EMGSlot), not
// from a literal here, for the reason memSlots and pmsSlots give: the value
// this capability data advertises must be one that dialect's own slot space
// accepts.
//
// Both discovered banks are NoBlank TRUE (matrix §1.3.4): those channels
// exist in a session because they answered a read, and this driver offers
// no way to blank them — no erase command exists in this radio's command
// set (§2.3), and core/cat refuses 5xx/EMG slots in both write builders
// (see readOnlyFields for the two different reasons). That is a statement
// about the PROTOCOL SURFACE this project offers, not about either radio's
// factory contents.
func effectiveCapabilities(d cat.Dialect, base spec.Capabilities, slots60m []string, emg bool) spec.Capabilities {
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
			Slots:   []string{d.EMGSlot().Wire()},
			NoBlank: true,
			Fields:  readOnlyFields(base),
		})
	}
	return caps
}
