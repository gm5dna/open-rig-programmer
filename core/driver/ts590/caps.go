// SPDX-License-Identifier: GPL-3.0-or-later

package ts590

import (
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	// ALIASED deliberately: the layout package's own name is also "ts590",
	// and an unaliased import would put a second meaning on the spelling
	// this package already answers to. kwts590 reads as "the core/kw side of
	// the TS-590", which is exactly what it is.
	kwts590 "github.com/gm5dna/open-rig-programmer/core/kw/ts590"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// Row selects which of this package's TWO registry rows a driver is for.
//
// THE ZERO VALUE NAMES NO RADIO AND IS NOT USABLE (plan P1). One package
// serves two rows because the TS-590S and the TS-590SG share a book, a
// 50-byte grid, a mode legend and both tone charts — and differ on exactly
// four things that reach a capability value or a runtime branch (matrix §4,
// and doc.go names them). A caller that forgot to say which gets a driver
// that names no model, publishes the zero capability set (which spec.Validate
// refuses) and refuses to Open, because the alternative — defaulting to one
// of them — would drive an SG with an S's row, or the reverse.
type Row int

const (
	// RowUnset is the zero value: no row. Every surface fails closed on it.
	RowUnset Row = iota
	// RowS is the TS-590S: "021: TS-590S" (590:1114).
	RowS
	// RowSG is the TS-590SG: "023: TS-590SG" (590:1116).
	RowSG
)

// String renders r for refusals and diagnostics.
func (r Row) String() string {
	if name := modelNameFor(r); name != "" {
		return name
	}
	return "unset TS-590 row"
}

// The two registry keys, and the three-digit identities their radios answer
// "ID;" with (matrix §1.1, §1.2).
//
// THE NAMES ARE A CHOICE OVER A MANUAL-EVIDENCED FACT. That the radios are
// called TS-590S and TS-590SG is the ID legend's own ("021: TS-590S",
// 590:1114; "023: TS-590SG", 590:1116) and the cover's; the project's
// SPELLING of the registry key is the choice, and Kenwood sets both names in
// the same hyphenated form the keys use, so unlike the FTdx10's FT-DX10
// near-miss there is no spelling to reconcile.
//
// P1 IS THREE DIGITS, NOT FOUR, and that is a new width for this project.
// core/driver.Driver's CATID doc records the convention as "four hex digits
// on Yaesu; the CI-V address … on Icom" (core/driver/driver.go:22), and
// core/spec constrains CATID only to be non-empty. Kenwood is a third form,
// recorded here so a later reader does not "fix" a three-digit value into
// four.
const (
	modelNameS  = "TS-590S"
	modelNameSG = "TS-590SG"
	catIDS      = "021"
	catIDSG     = "023"
)

// modelNameFor is the display name for r — the driver registry key, and
// necessarily equal to Capabilities().Model (core/driver.Driver's contract).
// It returns "" for an unset row, which matches no registry key.
func modelNameFor(r Row) string {
	switch r {
	case RowS:
		return modelNameS
	case RowSG:
		return modelNameSG
	default:
		return ""
	}
}

// catIDFor is the identity r's radio answers "ID;" with, and "" for an unset
// row — a value no radio can answer, so the probe's comparison fails closed.
func catIDFor(r Row) string {
	switch r {
	case RowS:
		return catIDS
	case RowSG:
		return catIDSG
	default:
		return ""
	}
}

// layoutFor is the codec layout r's radio speaks, and whether r names a radio
// at all. The two layouts differ on exactly two axes — byte 28's policy (A14)
// and the slot ceiling (A12) — and core/kw/ts590 is where that difference
// lives; this function only chooses between them.
func layoutFor(r Row) (kw.Layout, bool) {
	switch r {
	case RowS:
		return kwts590.LayoutS(), true
	case RowSG:
		return kwts590.LayoutSG(), true
	default:
		return kw.Layout{}, false
	}
}

// Profile selects which capability profile New builds the driver with.
//
// The zero value is RealHardware ON PURPOSE (matrix §2.1): a forgotten or
// zero-valued Profile must fail towards the real-hardware capability set —
// which for these rows is the all-Unverified one, nothing writable — and
// NEVER towards the simulator's, whose Supported writes are a claim about
// internal/fakets590 and about nothing else. Any OTHER unrecognised Profile
// value fails the same way, through Capabilities' explicit default arm.
// Shared with every other driver package (core/driver.Profile); this
// package keeps its own Simulated selector, which
// internal/guards.TestSimulatedProfileTokensConfinement requires.

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// writeTrialsComplete is THIS package's hardware write guard for BOTH of its
// rows, and it is FALSE: no TS-590S and no TS-590SG has ever been written to
// by this project — none has ever been ASKED anything at all (matrix §3.12).
//
// While it is false there is no hardware-verified capability profile for
// either row to select AT ALL, deliberately not even a placeholder one: a
// RealHardware session gets CapabilitiesUnverified, nothing is writable
// anywhere, and the capability gate refuses every write before a frame is
// built.
//
// It is consulted by no production code, and that is the point. Flipping it
// is a TWO-PART change — this constant AND a CapabilitiesRealHardware profile
// built field class by field class from the trial evidence, AND the
// Capabilities switch rewritten to select it — with the evidence linked and
// the pin below rewritten so the flip is a visible, reviewable test change.
// Making this constant load-bearing on its own would mean a one-character
// edit could unlock a write.
//
// IT IS ONE CONSTANT OVER TWO ROWS AND THE PIN IS PER ROW. A trial run on a
// TS-590S lifts the S row and the S row only (A9's own paragraph is the
// sharpest case), so the flip is not one edit either: it is a per-row profile
// plus this constant. TestWriteTrialsComplete_PinnedFalse asserts both halves
// — the constant, AND that each row's RealHardware baseline is genuinely
// nothing-writable — so a constant-only edit cannot pass while leaving the
// consequence untested.
//
// core/driver/icr8600/caps.go:14 is the shape precedent (matrix §3.12).
const writeTrialsComplete = false

// The two bank display labels, minted as THIS package's own consts.
//
// A DISPLAY LABEL IS NOT A PROTOCOL FACT (matrix §1.4.1, §1.4.3, both marked
// CHOICE there). "Memories" coincides with the FT-891's today because the
// neutral bank ID means the same thing to a user across the app, and nothing
// forces the two to stay equal. "Scan edges (P00–P09)" spells the book's own
// channel numbering — "Channel numbers P00 ~ P09 are represented by 100 ~
// 109" (590:1345) — rather than its phrase "section defined channels", which
// names the mechanism instead of the user's slots.
const (
	memBankLabel  = "Memories"
	scanBankLabel = "Scan edges (P00–P09)"
)

// The two FILTER labels, spelled as the chart prints them: "P11 (Selected
// status of FILTER A/B …) 0: FILTER A 1: FILTER B" (590:1560-1563).
//
// THE SPELLING IS THE MANUAL'S AND NOT THE DESIGN'S — matrix erratum M-E7,
// which records that the design's neutral-model table writes "FIL A"/"FIL B".
// The distinction is not cosmetic: this value lands in Capabilities.Filters
// verbatim, reaches the GUI's column vocabulary, and is what a CHIRP or CSV
// round trip must match.
const (
	filterALabel = "FILTER A"
	filterBLabel = "FILTER B"
)

// fmNarrowWire is P14's printed narrow value, "01: FM Narrow"
// (590:1569-1571). It is here so modeNames can ask the LAYOUT for the name it
// publishes for a narrow FM record rather than concatenating a suffix of its
// own — see modeNames.
const fmNarrowWire = "01"

// slotProbeCeiling bounds the walk that derives each row's bank inventories
// from its layout. MR/MW carry the channel number in P2 and P3's two digits
// (590:1539-1540, referring to MC), so three digits is the widest number the
// grid can address at all; a layout refuses everything above its own printed
// space long before this.
const slotProbeCeiling = 999

// kenwoodCTCSSTones is the 43-entry Kenwood tone chart, index = the CAT tone
// number, transcribed IN THIS PACKAGE from TN's own printed table
// (590:2296-2306), with "An entered value of 43 or higher results in an
// error" at 590:2309.
//
// IT IS NOT THE PROJECT'S SHARED CHART, AND CONFUSING THE TWO WOULD
// MISADDRESS EVERY TONE ON BOTH ROWS (plan P6, matrix §1.9). The shared
// 50-tone table in core/spec/tones.go is the Yaesu family's, and no Kenwood
// package may name its accessor; the Kenwood chart is that chart MINUS EIGHT
// interstitial tones — 159.8, 165.5, 171.3, 177.3, 183.5, 189.9, 196.6 and
// 199.5 — which is why 43 values sit where 50 do, and why every Kenwood index
// above 25 differs from the Yaesu index for the same frequency.
// TestCTCSSTones_AreNotTheProjectsSharedChart is the inequality pin and
// TestNoProductionFileNamesTheSharedToneChart holds the production half down.
//
// THE 43RD ENTRY IS AN OVER-CLAIM ON THE RECEIVE SIDE, AND THAT IS ERRATUM
// M-E1 RATHER THAN AN OVERSIGHT. spec.Capabilities carries ONE tone domain
// and ONE predicate, AdmitsTone, which codeplug.ToneField.Valid applies to
// ToneTx and ToneRx alike, so a 43-entry list admits 1750 Hz as a tone_rx
// value the CN chart (00-41, 590:416-426) does not print. The resolution is
// to publish 43 and refuse a Known tone_rx of 1750 Hz IN THE WRITE PATH
// (decision 14) — the same shape byte 28 gets: a runtime refusal in the
// driver, not a claim in the capability table. A 1750 Hz tone_rx cannot come
// off a radio in the first place, because core/kw bounds P9 at CN's own 41.
//
// spec.Validate requires CTCSSTones to be strictly ascending precisely so the
// slice index doubles as the CAT tone number; 254.1 → 1750.0 is ascending.
var kenwoodCTCSSTones = []spec.Tone{
	670, 693, 719, 744, 770, 797, 825, 854, 885, 915, 948, 974,
	1000, 1035, 1072, 1109, 1148, 1188, 1230, 1273, 1318, 1365,
	1413, 1462, 1514, 1567, 1622, 1679, 1738, 1799, 1862, 1928,
	2035, 2065, 2107, 2181, 2257, 2291, 2336, 2418, 2503, 2541,
	17500,
}

// modeNames returns the selectable mode display names this row's capability
// data advertises, DERIVED FROM THE LAYOUT rather than transcribed here.
//
// There is deliberately no local mode table. MR/MW's P5 carries no legend of
// its own on either row — both charts say "refer to the MD command"
// (590:1544-1545) — so the memory mode vocabulary IS MD's, and core/kw/ts590
// transcribed it once from 590:1353-1363. Enumerating the layout makes a
// drifting second transcription unrepresentable.
//
// Wire-code order comes free: kw.Mode's underlying value IS the wire byte, so
// ascending byte order is the legend's own order, and nibbles 0 and 8 — both
// printed "None (setting failure)" (590:1353, 590:1362) — are absent from the
// legend and so from this list.
//
// FM-N IS SYNTHESISED AND IS WHY THIS LIST IS NINE NAMES OVER EIGHT NIBBLES
// (matrix §1.5). P14 is a two-byte flag orthogonal to the mode nibble whose
// printed meanings are only "00: FM Normal" and "01: FM Narrow"
// (590:1569-1571); the Yaesu family folds narrow FM into the mode legend and
// Kenwood does not. The narrow name is ASKED OF THE LAYOUT, by handing it a
// record whose mode is FM and whose P14 is the printed narrow value, so that
// the name this capability list publishes is BYTE-FOR-BYTE the name the read
// path will produce for such a record — rather than a suffix concatenated
// here, which would be a second rule able to drift from kw.RecordModeName's.
//
// That P14 means anything at all OUTSIDE FM is unknown and is A23, whose
// consequence is the non-FM write refusal in the write path, not an omission
// from this list.
func modeNames(l kw.Layout) []string {
	var names []string
	for b := 0; b <= 0xFF; b++ {
		m := kw.Mode(byte(b))
		name, ok := l.ModeName(m)
		if !ok {
			continue
		}
		names = append(names, name)
		if m != kw.ModeFM {
			continue
		}
		narrow, ok := l.RecordModeName(kw.Record{Mode: m, Byte3940: fmNarrowWire})
		if !ok {
			// Unreachable on a row whose P14 is the FM bandwidth flag,
			// which is both of this package's. Omit rather than invent a
			// name: a mode list is what the GUI offers a user to write.
			continue
		}
		names = append(names, narrow)
	}
	return names
}

// memSlots returns this row's MEM inventory, "000".."099", built through the
// LAYOUT's own NewSlot so the wire forms this capability data advertises are
// the ones that row's slot space actually accepts.
//
// THE CLASS FILTER IS WHAT OMITS THE TS-590SG'S EXTENSION CHANNELS, and that
// is Stuart's decisions row 6, RULED 05/09/2026. The SG layout DECLARES
// 110-119 as kw.SlotExtension, because the book prints them (590:1346-1347)
// and a front-panel recall of E00 produces an "MR0115;" answer this codec
// must parse rather than refuse. What an extension channel IS is never
// explained anywhere in the book (A11), so the ten slots are OMITTED from the
// DRIVER's published banks until A11 lifts — not folded into MEM, not a
// seventh spec.BankID, and not a bank of their own. Selecting on
// kw.SlotMemory rather than on a number range is what keeps the two halves —
// the codec's domain and the driver's inventory — a single edit apart from
// each other rather than two independent lists.
func memSlots(l kw.Layout) []string {
	var slots []string
	for n := 0; n <= slotProbeCeiling; n++ {
		s, err := l.NewSlot(n, kw.ScanHalfNone)
		if err != nil || s.Class() != kw.SlotMemory {
			continue
		}
		slots = append(slots, s.String())
	}
	return slots
}

// scanSlots returns this row's SCAN inventory, "100L","100U".."109L","109U".
//
// A SECTION CHANNEL HOLDS TWO FREQUENCIES AND A NEUTRAL CHANNEL HOLDS ONE
// (matrix §1.4.3), so ten wire channels become twenty neutral slots. The two
// are reached by the same slot number with different P1 bytes — "When reading
// the start frequency of a section defined channel, enter 0 for parameter P1.
// When reading the end frequency, enter 1" (590:1449-1451), the MW half at
// 590:1529-1531 — and kw.NewSlot admits a half ONLY for kw.SlotScan, so this
// walk needs no range of its own and cannot pick up a memory or an extension
// channel by accident.
func scanSlots(l kw.Layout) []string {
	var slots []string
	for n := 0; n <= slotProbeCeiling; n++ {
		lower, err := l.NewSlot(n, kw.ScanLower)
		if err != nil {
			continue
		}
		upper, err := l.NewSlot(n, kw.ScanUpper)
		if err != nil {
			// Unreachable: NewSlot's half rule is per class, so a number
			// that admits a lower half admits an upper one. Refuse to
			// publish half a pair rather than guess.
			continue
		}
		slots = append(slots, lower.String(), upper.String())
	}
	return slots
}

// bankFields builds one bank's per-field support map. ALL TWENTY-SEVEN
// spec.Fields are listed explicitly, including the seventeen that are the
// zero FieldSupport: a field left out of the map reads identically to a field
// deliberately zeroed (Capabilities.FieldSupport returns the zero value for
// an absent key), and only a written-down zero is legible as a decision
// (matrix §2). core/driver/icr8600/caps.go's bankFields is the SHAPE
// precedent.
//
// Two cells are per-BANK and one is per-ROW, and they are the whole of the
// variation matrix §2.1 records:
//
//   - spec.FieldTxFrequency is graded in MEM and ZERO in SCAN (§2.4, M-E2).
//     The record expresses a split channel as two frames over one channel
//     number, P1=0 for the receive side and P1=1 for the transmit side
//     (590:1519-1520) — but on a SECTION-DEFINED channel P1 selects the START
//     or the END frequency instead (590:1449-1451, 590:1529-1531), so grading
//     it there would publish a scan edge as a transmit frequency. A
//     Bank.Fields map is per bank, which is the only place in the model this
//     distinction can live, and it is why the SCAN bank exists at all.
//
//   - spec.FieldFilter is graded on the SG row and ZERO on the S row (§1.22,
//     §2.7, Q12). Byte 28 is a live FILTER A/B selector on a TS-590SG and on
//     a TS-590S at firmware >= 2.00, and "always 0" only on a TS-590S at 1.xx
//     (590:1478, 590:1564 — the two wordings being erratum E7).
//     spec.Capabilities is a STATIC PER-MODEL VALUE and the TS-590S is ONE
//     registry row, so no row can publish "Supported iff FV >= 2.00". The
//     cost is published rather than hidden: a TS-590S at >= 2.00 loses its
//     filter selection through this programme. Byte 28 is still ACCEPTED as
//     '0' OR '1' ON PARSE on BOTH rows (core/kw's Byte28FilterEither) —
//     requiring '0' would make a >= 2.00 S using FILTER B fail the whole
//     channel read.
//
//   - spec.FieldScanSkip is graded in SCAN as well as MEM. Whether locking
//     out a scan-edge channel is MEANINGFUL is not printed anywhere; that the
//     byte is present and readable is (P15, 590:1572-1574). Grading the field
//     says the record can express it, which is true; it does not say the
//     radio does anything with it (§2.2).
//
// Each call returns a fresh map, so no two banks share one.
func bankFields(rw spec.FieldSupport, row Row, bank spec.BankID) map[spec.Field]spec.FieldSupport {
	txFreq := spec.FieldSupport{}
	if bank == spec.BankMemory {
		txFreq = rw
	}
	filter := spec.FieldSupport{}
	if row == RowSG {
		filter = rw
	}
	return map[spec.Field]spec.FieldSupport{
		// The nine the 50-byte record expresses on both banks.
		spec.FieldFrequency: rw, // P4, 11 digits at bytes 7-17 (590:1541-1543)
		spec.FieldMode:      rw, // P5 at byte 18, legend via MD (590:1544-1545)
		spec.FieldTag:       rw, // P16, 8 bytes at 42-49 (590:1575-1577)
		spec.FieldScanSkip:  rw, // P15 at byte 41 (590:1572-1574)
		spec.FieldToneMode:  rw, // P7 at byte 20, four values (590:1549-1553)
		spec.FieldToneTx:    rw, // P8 at 21-22 via TN (590:1554-1555)
		spec.FieldToneRx:    rw, // P9 at 23-24 via CN (590:1556-1557)
		spec.FieldDataMode:  rw, // P6 at byte 19 via DA (590:1546-1548)

		// The two that vary; see the doc comment.
		spec.FieldTxFrequency: txFreq,
		spec.FieldFilter:      filter,

		// MANUAL-EVIDENCED absence FROM THE RECORD, over a COMPLETE 47-byte
		// account (590:1539-1577). NOT a consequence of decision 6's
		// vocabulary rule, which constrains exactly one pair and does not
		// name this field (matrix M-E5) — and THESE RADIOS DO HAVE RIT AND
		// XIT, as radio-level settings no memory channel stores. The zero
		// means "no per-channel clarifier field", never "this radio has no
		// clarifier". ClarMaxHz and ClarStepHz are 0/0 in consequence.
		spec.FieldClarifier: {},

		// The Yaesu half of the vocabulary pair (decision 6,
		// core/spec/field.go:41-44). This record expresses tone as P7's mode
		// selector with two INDEPENDENT indices, which is the
		// tone_mode/tone_tx/tone_rx shape, and repeater operation as an
		// independent transmit frequency rather than a shift selector.
		// FieldCTCSSTone is additionally ONE field where the record carries
		// TWO indices.
		spec.FieldCTCSSState: {},
		spec.FieldCTCSSTone:  {},
		spec.FieldShift:      {},

		// No tag-display flag anywhere in either record: the 47 parameter
		// bytes are fully accounted for.
		spec.FieldTagDisplay: {},

		// The standing no-erase rule (decision 8, §2.8). The only erase
		// route printed is a side effect of an undocumented-width MW —
		// "If you do not specify one digit in P16 and execute all the
		// parameters from P4 to P15 set to 0, the channels specified by P2
		// and P3 will be erased" (590:1579-1581) — whose length is a
		// reading rather than a printed number (A5, erratum E19). The short
		// form is never built, and core/kw's gate refuses any MW that is not
		// exactly 50 bytes, so the guard is positive rather than negative.
		spec.FieldErase: {},

		// No duplex selector and no offset magnitude anywhere in the record;
		// split is expressed ONLY as two frames, which is FieldTxFrequency.
		spec.FieldDuplex: {},
		spec.FieldOffset: {},

		// DCS appears NOWHERE in either book — a case-insensitive search of
		// both documents for DCS and DTCS returns zero hits (§1.20, §1.21).
		// This is the strongest form of the absence and needs no caveat.
		spec.FieldDTCSCode:     {},
		spec.FieldDTCSPolarity: {},

		// Bytes 39-40 are the FM Normal/Narrow flag on these rows
		// (590:1569-1571), not a step; no step magnitude in hertz and no
		// on/off flag for a step exists in the record at all (§1.23, §1.24).
		spec.FieldTuningStepEnabled: {},
		spec.FieldTuningStep:        {},
		spec.FieldProgramTuningStep: {},

		// MANUAL-EVIDENCED absence FROM THE RECORD, and the caveat matters:
		// these radios HAVE an attenuator and a pre-amplifier, as RADIO-LEVEL
		// functions with their own commands — RA "[TS-590S / TS-590SG
		// common]" (590:1801) and PA (590:1653-1658). Neither has a per-
		// channel position among the 47 parameter bytes.
		spec.FieldAttenuator: {},
		spec.FieldPreamp:     {},
		spec.FieldAntenna:    {},

		// An Icom concept with no position in this frame and no mention in
		// either book.
		spec.FieldIPPlus: {},

		// The three TS-2000-only Satellite Memory bank fields (v1.10.0):
		// a different Kenwood row's bank, with no position in this
		// record and no bearing on either book.
		spec.FieldSatBandSwap: {},
		spec.FieldSatTrace:    {},
		spec.FieldSatTraceRev: {},
	}
}

// baseCapabilities assembles one row's static baseline at the given evidence
// grade.
//
// ALL TWENTY-EIGHT spec.Capabilities fields are populated explicitly — the
// non-zero ones and the seventeen deliberately EMPTY ones alike — and
// TestCapabilities_EveryFieldExplicit reflects over the struct to enforce
// both halves. A zero left in one of the populated ones is not a neutral
// omission: a zero MaxFreqHz reads as "no ceiling" to core/codeplug's
// validator, a zero TagLen makes core/csvio's CHIRP import truncate every
// imported name to "", a non-positive Bauds entry reaches SerialConfig.Baud,
// and an empty vocabulary where a bank reaches its field fails spec.Validate
// outright.
//
// THE EMPTY ONES ARE THE POSITIVE STATEMENT "this radio expresses no such
// vocabulary" (matrix §1.16-§1.28), which is what every capability-keyed
// check in core/codeplug and core/csvio tests before it runs; populating any
// of them would be the mistake. They are written out below with their reasons
// rather than left off, because an omission and a decision would otherwise
// read the same.
func baseCapabilities(row Row, rw spec.FieldSupport) spec.Capabilities {
	l, ok := layoutFor(row)
	if !ok {
		// An unset row describes no radio: the zero capability set, which
		// spec.Validate refuses and which grades nothing writable.
		return spec.Capabilities{}
	}
	return spec.Capabilities{
		Model: modelNameFor(row),
		CATID: catIDFor(row),
		// MANUAL-EVIDENCED (§1.3): both are HF transceivers with documented
		// transmit surfaces — TN, the transmit-tone command, is printed at
		// 590:2296 and its whole purpose is what the radio sends.
		// spec.Validate refuses the zero value, so this is a declaration
		// each row must make.
		Transmit: spec.HasTransmitter,
		// MANUAL-EVIDENCED: MW/MR carries NO transmit-frequency field at
		// all — split is two frames over one channel number, selected by P1,
		// and "When registering a simplex channel, set parameter P1 to 0.
		// After setting P1 to 0, the channel becomes a simplex channel, even
		// if it was already a split channel" (590:1521-1523). Simplex here
		// is arithmetic, tx == rx, which is the rule write.go's own A9 rung
		// refuses on. Read only by core/csvio's CHIRP importer, on the blank
		// Duplex arm; pinned by TestCapabilities_SimplexTx.
		SimplexTx: spec.SimplexTxEqualsRx,
		Banks: []spec.Bank{
			{
				ID:    spec.BankMemory,
				Label: memBankLabel,
				Slots: memSlots(l),
				// NoBlank FALSE, STATED (§2.5). NoBlank means "every slot
				// in this bank must be populated" — the Yaesu PMS
				// invariant — and nothing in this book imposes it; the
				// book's own note says the opposite, that an empty channel
				// is a normal state with a defined answer (590:1492-1493).
				// A NoBlank MEM bank would make codeplug.Validate refuse
				// every candidate with a single blank channel.
				NoBlank: false,
				Fields:  bankFields(rw, row, spec.BankMemory),
			},
			{
				ID:    spec.BankScan,
				Label: scanBankLabel,
				Slots: scanSlots(l),
				// NoBlank FALSE here TOO, and this is the bank where a
				// reader would most expect true, by analogy with PMS.
				// spec.BankScan exists precisely so a scan edge on a
				// non-Yaesu family is not obliged to honour the PMS pair
				// invariants (core/spec/bank.go:30-34), and no Kenwood
				// sentence requires a section channel to hold both ends.
				//
				// THE BANK IS BankScan AND NOT BankPMS for that same
				// reason (§1.4.3): PMS carries the Yaesu pair invariants,
				// which this radio's section channels do not.
				NoBlank: false,
				Fields:  bankFields(rw, row, spec.BankScan),
			},
		},
		Modes: modeNames(l),
		// MANUAL-EVIDENCED per row (§1.6): P16 is "Memory name (up to 8
		// digits)" (590:1576), drawn over positions 42-49 with the
		// terminator at 50. The WIDTH is the manual's; the PAD BYTE is not,
		// and is A1 — assumed spaces on write and right-trim on read.
		TagLen: 8,
		// 0/0, and the reason is the RECORD ACCOUNT rather than decision 6
		// — matrix M-E5, and the spec makes this comment's CONTENT part of
		// the specification. P4-P16 account for every one of the 47
		// parameter bytes (590:1539-1577, and the TS-480's own complete
		// account at 480:951-984) and NONE OF THEM IS AN RIT/XIT OFFSET.
		// THESE RADIOS DO HAVE RIT AND XIT — IS/XT/XO are in both books, and
		// the 480's RC "Clears the RIT offset frequency" (480:1205) — as
		// RADIO-LEVEL settings that no memory channel stores. Writing "the
		// Yaesu vocabulary is refused" here would read as "this radio has no
		// clarifier", which is false. spec.Validate places no constraint on
		// a zero ClarMaxHz, and FieldClarifier is Unsupported on every bank
		// of both rows, so nothing consults these.
		ClarMaxHz:  0,
		ClarStepHz: 0,
		// The 43-entry Kenwood chart; see kenwoodCTCSSTones. Copied per call
		// so no two capability values share the backing array.
		CTCSSTones: append([]spec.Tone(nil), kenwoodCTCSSTones...),
		// nil (§1.10): the Kenwood tone field is an INDEX into a printed
		// chart, not a number — MR/MW P8 and P9 are two-digit indices
		// "refer to the TN/CN command" (590:1555, 590:1557). A radio
		// declares a list or a range, never both.
		CTCSSToneRange: nil,
		// FIVE rates, and 4800 is deliberately OMITTED — matrix erratum
		// M-E4. Both books print the same six: "Baud Rate Selectable from
		// 4800*/ 9600/ 19200/ 38400/ 57600/ 115200 bps" (590:52-53). 4800 is
		// CONDITIONAL on both radios in two different ways and
		// spec.Capabilities can express neither. On these rows the asterisk
		// resolves to "4800 bps cannot be used with the USB-B connector"
		// (590:63) — and USB-B is the path most owners will use — while a
		// Bauds list is one flat list per row and cannot say "except over
		// one of this radio's two paths". On the TS-480 the same rate
		// requires TWO STOP BITS (480:22-23), which live in
		// transport.SerialConfig and are chosen once per session
		// independently of the rate, so offering 4800 anywhere in this
		// family would offer a rate this programme then opens with the
		// wrong framing. Publishing five claims nothing false.
		Bauds: []int{9600, 19200, 38400, 57600, 115200},
		// ASSUMED — A15, and an OPERATIONAL ASSUMPTION rather than a
		// conservative choice (§1.12). Neither book prints a factory value:
		// the hardware table lists the selectable rates and no default
		// (590:52-53). A wrong baud is not a safe baud — it is an
		// unreachable radio, and the symptom is a timeout that looks like a
		// dead port. NO AUTO-BAUD: probing by re-opening the port at five
		// speeds is a discovery mechanism this programme does not have and
		// would not test. spec.Validate requires this value to appear in
		// Bauds, and it does.
		DefaultBaud: 9600,
		// 0/0 — CANNOT ESTABLISH, matrix erratum M-E6, the registered
		// core/driver/icr8600 precedent. NEITHER PC-COMMAND DOCUMENT PRINTS
		// A FREQUENCY RANGE FOR ANY OF THESE RADIOS: FA/FB say only
		// "Frequency (11 digits in Hz)" with the worked example
		// "enter 00014195000 for 14.195 MHz" (590:961-966), and MR/MW P4
		// says "Frequency (depending on the P1 setting, unused high-end
		// digits will become 0)" (590:1541-1543). A FIELD WIDTH IS NOT A
		// TUNING RANGE, and publishing one as the other would put a
		// fabricated capability in the capability table.
		//
		// A ZERO IS A DISABLED CHECK, WHICH IS THE POSITIVE PROPERTY HERE:
		// codeplug.Validate's floor and ceiling both test != 0 first, so a
		// channel at 1 Hz and one at 99 GHz alike pass validation on these
		// rows — no false refusal and no false promise —
		// while a frequency needing more than eleven digits is refused by
		// the CODEC, whose kw.OutOfDomainError says in as many words that it
		// names the FIELD WIDTH and not any radio's tuning range (A17).
		// TestCapabilities_ZeroFrequencyBoundsDisableTheCheck is the
		// positive proof. Once Q1's instruction manuals give the bounds a
		// source they go in and Validate's check turns itself on with no
		// code change anywhere.
		MinFreqHz: 0,
		MaxFreqHz: 0,
		// nil (§1.15): RequiredSlots names individual slots that must never
		// be empty — the FT-710's M-01. Neither Kenwood book marks any
		// channel mandatory, and this book's own note says the opposite.
		// Distinct from Bank.NoBlank, which is also false everywhere.
		RequiredSlots: nil,
		// Both EMPTY (§1.16, §1.17): the Yaesu half of the vocabulary pair,
		// which core/spec/field.go:41-44 forbids a model expressing
		// alongside the Icom half. Empty is legal here only because no bank
		// grades FieldShift or FieldCTCSSState above Unsupported, which is
		// what spec.Validate's own pair rules are conditional on.
		ShiftOptions: nil,
		// EMPTY (§1.18), and this is the one place these rows publish an
		// INCOMPLETE Icom pair: the record carries no duplex selector and no
		// offset magnitude, and split is expressed only as two frames, which
		// is FieldTxFrequency and not FieldDuplex. A row that later opened
		// FieldDuplex without supplying this would fail Validate loudly,
		// which is the correct direction.
		DuplexOptions: nil,
		// FOUR values, MANUAL-EVIDENCED (§1.19): MR/MW P7 reads "0:
		// TONE/CTCSS OFF, 1: TONE ON, 2: CTCSS ON, 3: Cross Tone ON"
		// (590:1549-1553). The TS-480's legend stops at 2 (480:964), which
		// is one of the two rows' four-vs-three divergences and that row's
		// business, not this one's.
		//
		// THE SEMANTICS COME FROM ONE SENTENCE, and it is IF P14's note:
		// "When Tone is ON, this number is the Tone frequency. When CTCSS is
		// ON, this number is the CTCSS frequency. When Cross Tone is ON, the
		// transceiver transmits on the Tone frequency and receives on the
		// CTCSS frequency" (590:1167-1171). That sentence is the whole
		// evidence for three separate things, all three used here: that P8
		// is the TRANSMIT index, that P9 is the RECEIVE index, and that TONE
		// encodes while CTCSS squelches. Hence ToneModeCTCSS (transmits a
		// tone, requires none on receive) for 1 and ToneModeCTCSSRxSquelch
		// (requires a received tone, transmits none) for 2.
		//
		// CROSS is declared and IS usable on these rows: ToneModeCross
		// requires a model that also expresses the fields the combination
		// needs, and these rows do — tone_tx from P8 and tone_rx from P9,
		// independently, which is exactly what Cross Tone uses.
		//
		// Canonical is false on every entry: no semantic is expressed twice,
		// so spec.Validate's canonical rule requires nothing.
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS},
			{Value: "CTCSS", Semantics: spec.ToneModeCTCSSRxSquelch},
			{Value: "CROSS", Semantics: spec.ToneModeCross},
		},
		// Both EMPTY (§1.20, §1.21): see FieldDTCSCode in bankFields.
		DTCSPolarities: nil,
		DTCSCodes:      nil,
		// Per row, and it is the SG that publishes them; see bankFields'
		// FieldFilter paragraph for why the S row publishes none.
		Filters: filtersFor(row),
		// EMPTY / nil (§1.23, §1.24): bytes 39-40 are the FM Normal/Narrow
		// flag on these rows (590:1569-1571), so no tuning-step position
		// exists in their 50-byte record and no step magnitude in hertz does
		// either. (On the TS-480 the same two bytes ARE a step, present and
		// refused, which is that row's A22 and not this package's.)
		TuningSteps:            nil,
		ProgramTuningStepRange: nil,
		// All EMPTY (§1.25-§1.27), with the caveat bankFields states: the
		// radios have an attenuator and a pre-amplifier as radio-level
		// functions, and no record position for either; nothing here says
		// they have none.
		AttenuatorDB:   nil,
		PreampOptions:  nil,
		AntennaOptions: nil,
		// EMPTY (§1.28), a CHOICE taking the strict direction. The empty
		// string selects the family default — printable ASCII 0x20-0x7E
		// excluding ';' (spec.Capabilities.TagByteOK) — which on these rows
		// is EXACTLY what the book says: P16's only charset statement is
		// "';' (semicolon) cannot be used for the parameter P16"
		// (590:1577), the default's own exclusion for the default's own
		// reason. THE CLAIM IS BOUNDED AT 0x7F and A2 says so in terms:
		// 0x80-0xFF is unevidenced AND unclaimed — the programme refuses
		// those bytes by its own charset rule and this design says nothing
		// about what either radio would do with one.
		TagCharset: "",
	}
}

// filtersFor is the per-row filter label vocabulary: the two printed labels
// on the SG, and NONE on the S. See bankFields' FieldFilter paragraph — the
// S's empty list is a REFUSAL (Q12, §2.7), not an absence, and it is the
// consequence of spec.Capabilities being a static per-model value while byte
// 28's liveness on that row is firmware-conditional.
func filtersFor(row Row) []string {
	if row == RowSG {
		return []string{filterALabel, filterBLabel}
	}
	return nil
}

// CapabilitiesUnverified is row's all-Unverified FAIL-SAFE profile, and it is
// what a RealHardware session gets today: every field the 50-byte record
// expresses is labelled Read Unverified / Write Unverified — documented in
// the PC-command reference and exercised against scripted peers, but never
// proven against a radio — and every field the record does not express stays
// the zero FieldSupport.
//
// Because Unverified makes FieldSupport.CanWrite false, this profile AS
// LABELLED blocks every write project-wide: codeplug.Diff refuses the change,
// the clone service refuses to execute a plan containing it, and
// Session.WriteChannel re-checks and refuses before building a frame. It is
// also what any UNRECOGNISED Profile value selects — the failure direction is
// always "nothing writable" (matrix §2.1).
//
// THE ONE ROUTE PAST THAT, and it is the user's own: a session opened with
// WithConsentedUnverifiedWrites re-labels these write-side Unverified fields
// spec.ConsentedUnverified at session-capability assembly, and CanWrite is
// true for that state — so a CONSENTED RealHardware session can attempt a
// write, while this static profile is untouched and every unconsented session
// still cannot. The profile keeps saying the true thing either way: it
// describes the EVIDENCE (none), and consent is a decision about risk. CONSENT
// WIDENS WHAT MAY BE ATTEMPTED, NEVER HOW CAREFULLY — every pre-wire refusal
// of the write ladder still fires ahead of it.
//
// The READ labels are Unverified rather than Supported for the same reason
// (§2.1, "the honest one"): this driver's read path is exercised against a
// fake and a manual, and no TS-590 has ever answered a frame.
//
// NO FIELD IS spec.Inert ON EITHER ROW. Inert is the FT-710's HARDWARE
// finding about the FT-710; no Kenwood radio has been asked anything, so
// there is no finding to record and borrowing one would answer a question
// about one radio with another radio's evidence.
func CapabilitiesUnverified(row Row) spec.Capabilities {
	return baseCapabilities(row, spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified})
}

// CapabilitiesSimulated is row's internal/fakets590-backed profile (CLI
// --fake, GUI demo) and NEVER a real radio: Read AND Write Supported for
// exactly the fields the 50-byte record expresses, on MEM and SCAN alike.
//
// Against the fake, hardware risk is moot and the write choreography itself
// is what is being exercised end to end, so claiming Supported here is a
// claim about internal/fakets590 and about nothing else (§2.1).
//
// The seventeen fields the record does not express stay the zero
// FieldSupport, simulator or not: the FORM cannot express them, and no amount
// of cooperative fake on the other end of the wire changes what the frame has
// room for. The S row's filter stays zero here too, because that refusal is
// about a firmware condition a static table cannot carry, not about evidence.
func CapabilitiesSimulated(row Row) spec.Capabilities {
	return baseCapabilities(row, spec.FieldSupport{Read: spec.Supported, Write: spec.Supported})
}
