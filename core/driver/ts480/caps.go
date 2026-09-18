// SPDX-License-Identifier: GPL-3.0-or-later

package ts480

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	// ALIASED deliberately: the layout package's own name is also "ts480",
	// and an unaliased import would put a second meaning on the spelling
	// this package already answers to. kwts480 reads as "the core/kw side of
	// the TS-480", which is exactly what it is — the same aliasing
	// core/driver/ts590/caps.go uses for its own sibling.
	kwts480 "github.com/gm5dna/open-rig-programmer/core/kw/ts480"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// The registry key and the three-digit identity this radio answers "ID;"
// with (matrix §1.1, §1.2).
//
// ONE ROW FOR FOUR PRINTED VARIANTS (decision 4). TY's P2 discriminates the
// TS-480HX, the TS-480SAT and two Japanese types (480:1626-1629), and the
// neutral memory model expresses none of the difference — same ID, same
// 50-byte record, same 00-99 space. The variant is read at probe and reported
// (Session.Variant); it never reaches the registry key.
//
// P1 IS THREE DIGITS, NOT FOUR, and that is Kenwood's width across this
// family: "020: TS-480" (480:678). core/driver.Driver's CATID doc records the
// convention as four hex digits on Yaesu and the CI-V address on Icom, so this
// is a third form, recorded here so a later reader does not "fix" it.
const (
	modelName = "TS-480"
	catID     = "020"
)

// layout is the codec layout this radio speaks. It is a func rather than a
// package var so this package holds no copy of a value core/kw/ts480 owns.
//
// THERE IS NO ROW ARGUMENT, where core/driver/ts590 has a required one. That
// package serves TWO registry rows differing on byte 28's write policy (A14)
// and on their slot ceilings (A12); this one serves one.
func layout() kw.Layout { return kwts480.Layout() }

// Profile selects which capability profile New builds the driver with.
//
// The zero value is RealHardware ON PURPOSE (matrix §2.1): a forgotten or
// zero-valued Profile must fail towards the real-hardware capability set —
// which for this row is the all-Unverified one, nothing writable — and NEVER
// towards the simulator's, whose Supported writes are a claim about
// internal/fakets480 and about nothing else. Any OTHER unrecognised Profile
// value fails the same way, through Capabilities' explicit default arm.
// Shared with every other driver package (core/driver.Profile); this
// package keeps its own Simulated selector, which
// internal/guards.TestSimulatedProfileTokensConfinement requires.
type Profile = driver.Profile

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// writeTrialsComplete is THIS row's hardware write guard, and it is FALSE: no
// TS-480 has ever been written to by this project — none has ever been ASKED
// anything at all (matrix §3.12).
//
// ON THIS ROW IT IS THE SECOND OF TWO GUARDS AND NOT THE ONLY ONE. A22
// refuses every TS-480 channel write outright (write.go, decision 12), and
// the row is additionally absent from internal/wiring until A4 is lifted
// (P3, doc.go). Flipping this constant alone would therefore change nothing
// at all — which is the correct relationship, not a redundancy: each guard
// answers a different question, and the write path's own tests assert A22
// through its TYPED error precisely so that the capability gate cannot stand
// in for it.
//
// Flipping it is a TWO-PART change — this constant AND a
// CapabilitiesRealHardware profile built field class by field class from the
// trial evidence, AND the Capabilities switch rewritten to select it — with
// the evidence linked and TestWriteTrialsComplete_PinnedFalse rewritten so
// the flip is a visible, reviewable test change.
//
// core/driver/icr8600/caps.go's namesake is the shape precedent.
const writeTrialsComplete = false

// memBankLabel is the bank's display label, minted as THIS package's own
// const.
//
// A DISPLAY LABEL IS NOT A PROTOCOL FACT (matrix §1.4.1, marked CHOICE
// there). "Memories" coincides with the FT-891's and the TS-590's today
// because the neutral bank ID means the same thing to a user across the app,
// and nothing forces the three to stay equal.
const memBankLabel = "Memories"

// slotProbeCeiling bounds the walk that derives the bank inventory from the
// layout. MR/MW carry the channel number in P3's two digits (480:955) with P2
// a printed constant (480:953), so ninety-nine is the widest number the grid
// can address at all; the layout refuses everything above its own printed
// space long before this.
const slotProbeCeiling = 999

// slotID renders a channel number as this row's canonical slot identifier.
//
// TWO DIGITS, AND DELIBERATELY NOT kw.Slot.String() — which is the ONE place
// this driver may not reuse the codec's own rendering. kw.Slot.String formats
// "%03d" because that is the 590 pair's printed width, where MC prints a
// hundreds digit and a two-digit remainder (590:1333). This radio's MC prints
// neither: P1 is "0: Always 0 for the TS-480 (Memory bank number)."
// (480:827) and P2 is "00 ~ 99: Channel number" (480:830), with MR's own P3
// likewise "00 ~ 99" (480:912). So the canonical slot string here is the
// book's own two digits — matrix §1.4.1, where the wire form is recorded as a
// CHOICE over the radios' printed widths — and a row publishing "042" would
// misdescribe its own wire.
//
// The cost of the divergence is one function and one comparison: read.go
// compares the ANSWERED SLOT NUMBER rather than the answered slot STRING, and
// TestBanks_IsOneFlatMEMBankOfTwoDigitSlots pins the width.
func slotID(number int) string { return fmt.Sprintf("%02d", number) }

// modeNames returns the selectable mode display names this row advertises,
// DERIVED FROM THE LAYOUT rather than transcribed here.
//
// There is deliberately no local mode table. MR/MW's P5 carries no legend of
// its own — both charts say "Mode. Refer to the MD command." (480:917,
// 480:959) — so the memory mode vocabulary IS MD's, and core/kw/ts480
// transcribed it once from 480:843-854. Enumerating the layout makes a
// drifting second transcription unrepresentable.
//
// Wire-code order comes free: kw.Mode's underlying value IS the wire byte, so
// ascending byte order is the legend's own order, and nibbles 0 and 8 — "0:
// No mode (Not used for the TS-480)" (480:843) and "8: Tune (Not used for the
// TS-480)" (480:853) — are absent from the legend and so from this list.
//
// EIGHT NAMES OVER EIGHT NIBBLES, WHERE THE 590 PAIR PUBLISH NINE, and the
// missing one is FM-N. On those rows P14 is a two-byte FM Normal/Narrow flag
// and the driver synthesises a ninth name from P5=4 × P14; here P14 is "Step
// size. Refer to the ST command." (480:979), so there is no width flag to
// synthesise from and nothing to fold into a mode name.
func modeNames(l kw.Layout) []string {
	var names []string
	for b := 0; b <= 0xFF; b++ {
		name, ok := l.ModeName(kw.Mode(byte(b)))
		if !ok {
			continue
		}
		names = append(names, name)
	}
	return names
}

// memSlots returns this row's MEM inventory, "00".."99", built through the
// LAYOUT's own NewSlot so the numbers this capability data advertises are the
// ones the row's slot space actually accepts — then rendered in this row's own
// two-digit form (see slotID).
//
// THE CLASS FILTER IS WHAT KEEPS 90-99 IN THE BANK. layout480 declares one
// flat SlotMemory range (480:955), so the filter admits every channel; it is
// written as a class test rather than a number range for the same reason the
// 590 driver writes one — the codec's domain and the driver's inventory stay
// a single edit apart rather than being two independent lists — and here it
// additionally records that this row has NO SlotScan at all.
func memSlots(l kw.Layout) []string {
	var slots []string
	for n := 0; n <= slotProbeCeiling; n++ {
		s, err := l.NewSlot(n, kw.ScanHalfNone)
		if err != nil || s.Class() != kw.SlotMemory {
			continue
		}
		slots = append(slots, slotID(s.Number()))
	}
	return slots
}

// bankFields builds the bank's per-field support map. ALL TWENTY-SEVEN
// spec.Fields are listed explicitly, including the twenty-two that are the
// zero FieldSupport: a field left out of the map reads identically to a field
// deliberately zeroed (Capabilities.FieldSupport returns the zero value for an
// absent key), and only a written-down zero is legible as a decision (matrix
// §2). core/driver/icr8600/caps.go's bankFields is the SHAPE precedent.
//
// FIVE FIELDS ARE GRADED HERE AND NINE ARE ON THE TS-590SG, and the four this
// row loses are the whole of §5's divergence table reaching a capability
// value. There is no per-bank variation and no per-row variation to carry:
// this row has ONE bank and IS one row.
//
// The six zeroes that are NOT family facts — the ones a reader would expect
// to be graded, having read the 590 pair's table — each carry their own
// reason, and caps_test.go's unexpressedFields holds the long form:
//
//   - spec.FieldTxFrequency — M-E2. A split channel is two frames over one
//     number (480:951), but channels 90-99 are ORDINARY MEMORIES that also
//     answer a second frame (480:943-944, 480:986-987), and a Bank.Fields map
//     is per bank rather than per slot, so no bank split can grade the field
//     for 00-89 and not for 90-99. Unsupported on the whole row, and the P1=1
//     half of 90-99 is unreachable through this programme — a real loss,
//     published rather than papered over.
//   - spec.FieldToneTx and spec.FieldToneRx — Q2. P8 and P9 are printed
//     indices (480:966, 480:969) whose CHARTS are not in this book: "Refer to
//     page 32 of the TS-480 instruction manual" (480:1559-1560) and page 33
//     for CN (480:339-340). The ranges are printed and the mapping is not, and
//     borrowing the TS-590's table across a model boundary is the cross-model
//     inference this milestone refuses. CTCSSTones is nil in consequence and
//     AdmitsTone fails closed.
//   - spec.FieldDataMode — the absent byte-19 data flag. Byte 19 is the
//     channel LOCKOUT here (480:962), where the 590 pair carry the data mode
//     (590:1546-1548); there is no data-mode position anywhere in this
//     record. §5 calls this the divergence no roadmap line records.
//   - spec.FieldTuningStep — A22's refusal, and the only zero on this row
//     that means "present and refused" rather than "absent". Bytes 39-40 ARE
//     a step (480:979) and ST's legend is mode-conditional over two different
//     ranges (480:1494-1500), so no flat Capabilities.TuningSteps vocabulary
//     is truthful.
//   - spec.FieldFilter — the hard-wired byte 28, "Always 0 for the TS-480."
//     (480:973), where the TS-590SG's is a live FILTER A/B selector.
//
// Each call returns a fresh map, so no two banks share one.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		// The five this row's 50-byte record expresses.
		spec.FieldFrequency: rw, // P4, 11 digits at bytes 7-17 (480:957)
		spec.FieldMode:      rw, // P5 at byte 18, legend via MD (480:959)
		spec.FieldTag:       rw, // P16, 8 bytes at 42-49 (480:984)
		// P6 at BYTE 19 — the lockout (480:962). The 590 pair read the
		// same neutral field from byte 41 (590:1572-1574), where this
		// radio prints a constant (480:982): the two swap the bytes.
		spec.FieldScanSkip: rw,
		spec.FieldToneMode: rw, // P7 at byte 20, THREE values (480:964)

		// The six the doc comment above accounts for one by one.
		spec.FieldTxFrequency: {},
		spec.FieldToneTx:      {},
		spec.FieldToneRx:      {},
		spec.FieldDataMode:    {},
		spec.FieldTuningStep:  {},
		spec.FieldFilter:      {},

		// MANUAL-EVIDENCED absence FROM THE RECORD, over this book's own
		// COMPLETE 47-byte account (480:951-984). NOT a consequence of
		// decision 6's vocabulary rule, which constrains exactly one pair
		// and does not name this field (matrix M-E5) — and THIS RADIO DOES
		// HAVE RIT AND XIT: RC "Clears the RIT offset frequency"
		// (480:1205), and the IS/XT/XO family beside it, as radio-level
		// settings no memory channel stores. The zero means "no
		// per-channel clarifier field", never "this radio has no
		// clarifier". ClarMaxHz and ClarStepHz are 0/0 in consequence.
		spec.FieldClarifier: {},

		// The Yaesu half of the vocabulary pair (decision 6,
		// core/spec/field.go). This record expresses tone as P7's mode
		// selector with two independent indices, and repeater operation as
		// an independent transmit frequency rather than a shift selector.
		// FieldCTCSSTone is additionally ONE field where the record carries
		// TWO indices.
		spec.FieldCTCSSState: {},
		spec.FieldCTCSSTone:  {},
		spec.FieldShift:      {},

		// No tag-display flag anywhere in the record: the 47 parameter
		// bytes are fully accounted for (480:951-984).
		spec.FieldTagDisplay: {},

		// THIS RADIO HAS NO ERASE ROUTE AT ALL, which is a stronger
		// statement than the 590 pair's (§2.8): the only "clear" in the
		// whole book is RC, "Clears the RIT offset frequency" (480:1205).
		// The short-MW ambiguity of 590:1579-1581 (A5, erratum E19) does
		// not arise here, and this row's write path builds no frame of any
		// width in any case (A22).
		spec.FieldErase: {},

		// No duplex selector and no offset magnitude anywhere in the
		// record; split is expressed ONLY as two frames, which on this row
		// is a field nothing publishes.
		spec.FieldDuplex: {},
		spec.FieldOffset: {},

		// DCS appears NOWHERE in either book — a case-insensitive search
		// of both documents for DCS and DTCS returns zero hits (§1.20,
		// §1.21). This is the strongest form of the absence and needs no
		// caveat.
		spec.FieldDTCSCode:     {},
		spec.FieldDTCSPolarity: {},

		// No on/off flag for a step and no step magnitude in hertz exists
		// in the record; the 480's step is an INDEX into ST, not a
		// magnitude (§1.23, §1.24).
		spec.FieldTuningStepEnabled: {},
		spec.FieldProgramTuningStep: {},

		// MANUAL-EVIDENCED absence FROM THE RECORD, and the caveat matters:
		// this radio HAS an attenuator and a pre-amplifier as RADIO-LEVEL
		// functions with their own commands (480:1185, 480:1055-1058).
		// Neither has a per-channel position among the 47 parameter bytes.
		// TY P2's AT-equipped variant (480:1627) is likewise a radio-level
		// fact for the probe note, not a per-channel antenna field.
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

// baseCapabilities assembles this row's static baseline at the given evidence
// grade.
//
// ALL TWENTY-EIGHT spec.Capabilities fields are populated explicitly — the
// non-zero ones and the nineteen deliberately EMPTY ones alike — and
// TestCapabilities_EveryFieldExplicit reflects over the struct to enforce both
// halves. A zero left in one of the populated ones is not a neutral omission:
// a zero MaxFreqHz reads as "no ceiling" to core/codeplug's validator, a zero
// TagLen makes core/csvio's CHIRP import truncate every imported name to "",
// and a non-positive Bauds entry reaches SerialConfig.Baud.
func baseCapabilities(rw spec.FieldSupport) spec.Capabilities {
	l := layout()
	return spec.Capabilities{
		Model: modelName,
		CATID: catID,
		// MANUAL-EVIDENCED (§1.3): an HF transceiver with a documented
		// transmit surface — TX appears as a literal frame in this book's
		// own front matter (480:51-52). spec.Validate refuses the zero
		// value, so this is a declaration the row must make.
		Transmit: spec.HasTransmitter,
		// ONE BANK, AND NO SCAN BANK — §1.4.3, decision 15. The book prints
		// the P1 overload for channels 90-99 (480:943-944, 480:986-987),
		// but on this radio those ten are ORDINARY memories that also
		// answer a second frame rather than a separate class: there is no
		// bank field in the record at all (480:827), a slot string is
		// unique across a codeplug, and a Bank.Fields map is per bank
		// rather than per slot. A scan bank here would either duplicate ten
		// slot identities or take ten ordinary memories away from the
		// owner.
		Banks: []spec.Bank{{
			ID:    spec.BankMemory,
			Label: memBankLabel,
			Slots: memSlots(l),
			// NoBlank FALSE, STATED (§2.5). NoBlank means "every slot in
			// this bank must be populated" — the Yaesu PMS invariant —
			// and nothing in this book imposes it. This book says
			// nothing about empty channels at all, which is A4; an
			// UNDOCUMENTED empty-channel behaviour is not evidence that
			// a channel must be populated.
			NoBlank: false,
			Fields:  bankFields(rw),
		}},
		Modes: modeNames(l),
		// MANUAL-EVIDENCED (§1.6): P16 is "Memory name. A maximum of 8
		// characters." (480:984), drawn over positions 42-49 with the
		// terminator at 50. The WIDTH is the manual's; the PAD BYTE is not,
		// and is A1 — assumed spaces on write and right-trim on read, with
		// this book's own KY precedent for a different command.
		TagLen: 8,
		// 0/0, and the reason is the RECORD ACCOUNT rather than decision 6
		// — matrix M-E5. P4-P16 account for every one of the 47 parameter
		// bytes (480:951-984) and NONE OF THEM IS AN RIT/XIT OFFSET. THIS
		// RADIO DOES HAVE RIT AND XIT — RC "Clears the RIT offset
		// frequency" (480:1205) — as radio-level settings that no memory
		// channel stores. Writing "the Yaesu vocabulary is refused" here
		// would read as "this radio has no clarifier", which is false.
		ClarMaxHz:  0,
		ClarStepHz: 0,
		// nil — CANNOT ESTABLISH (§1.9), and it is the sharpest single
		// difference from the two 590 rows. This book prints NEITHER tone
		// chart: TN says "Refer to page 32 of the TS-480 instruction
		// manual for the Tone numbers and frequencies." (480:1559-1560)
		// and CN "Refer to page 33 …" (480:339-340). The RANGES are
		// printed — TN P1 00 ~ 42 (480:1557), CN P1 00 ~ 41 (480:337) —
		// and they match the 590's, but the MAPPING is not established,
		// and borrowing the TS-590SG's table across a model boundary is
		// exactly the non-borrowing rule this project enforces elsewhere.
		//
		// spec.Capabilities.AdmitsTone FAILS CLOSED when neither a list nor
		// a range is declared, which is the correct direction: NO TONE IS
		// SENDABLE TO A TS-480 BY THIS PROGRAMME. The consequence —
		// tone_tx/tone_rx Unsupported and a write refused whenever the
		// source channel's tone_mode is not OFF — is bankFields' and
		// write.go's.
		CTCSSTones: nil,
		// nil (§1.10): a radio declares a list or a range, never both, and
		// this row declares neither. The Kenwood tone field is an INDEX
		// into a printed chart in any case (480:966, 480:969), not a
		// number.
		CTCSSToneRange: nil,
		// FIVE rates, and 4800 is deliberately OMITTED — matrix erratum
		// M-E4. This book prints the six in the EX menu 056 legend, "COM
		// port communication speed 4800 9600 19200 38400 57600 115200
		// (bps)" (480:533). On THIS radio 4800 requires TWO STOP BITS —
		// "1 start bit, 8 data bits, and 1 stop bit (4800 bps must be
		// configured as 2 stop bits)" (480:22-23) — and stop bits live in
		// transport.SerialConfig, chosen once per session independently of
		// the rate, so offering 4800 would offer a rate this programme then
		// opens with the wrong framing. That sentence is the BINDING one
		// for the family: the 590 pair's equivalent is permissive
		// ("2 is available only when using 4800 bps", 590:58). Publishing
		// five claims nothing false.
		Bauds: []int{9600, 19200, 38400, 57600, 115200},
		// ASSUMED — A15, and an OPERATIONAL ASSUMPTION rather than a
		// conservative choice (§1.12). This book prints no factory value:
		// menu 056's legend lists the rates and marks no default
		// (480:533). A wrong baud is not a safe baud — it is an
		// unreachable radio, and the symptom is a timeout that looks like a
		// dead port. NO AUTO-BAUD: probing by re-opening the port at five
		// speeds is a discovery mechanism this programme does not have and
		// would not test. spec.Validate requires this value to appear in
		// Bauds, and it does.
		DefaultBaud: 9600,
		// 0/0 — CANNOT ESTABLISH, matrix erratum M-E6, the registered
		// core/driver/icr8600 precedent. THIS DOCUMENT PRINTS NO FREQUENCY
		// RANGE: FA/FB say only "Frequency in Hz (11-digit)" and MR/MW P4
		// the same (480:957). A FIELD WIDTH IS NOT A TUNING RANGE, and
		// publishing one as the other would put a fabricated capability in
		// the capability table.
		//
		// A ZERO IS A DISABLED CHECK, WHICH IS THE POSITIVE PROPERTY HERE:
		// codeplug.Validate's floor and ceiling both test != 0 first, so a
		// channel at 1 Hz and one at 99 GHz alike pass validation on this
		// row — no false refusal and no false promise — while a frequency
		// needing more than eleven digits is refused by the CODEC, whose
		// kw.OutOfDomainError says in as many words that it names the FIELD
		// WIDTH and not any radio's tuning range (A17).
		MinFreqHz: 0,
		MaxFreqHz: 0,
		// nil (§1.15): RequiredSlots names individual slots that must never
		// be empty — the FT-710's M-01. This book marks no channel
		// mandatory and says nothing about empty channels at all. Distinct
		// from Bank.NoBlank, which is also false.
		RequiredSlots: nil,
		// Both EMPTY (§1.16, §1.17): the Yaesu half of the vocabulary pair,
		// which core/spec/field.go forbids a model expressing alongside the
		// Icom half. Empty is legal here only because no bank grades
		// FieldShift or FieldCTCSSState above Unsupported, which is what
		// spec.Validate's own pair rules are conditional on.
		ShiftOptions: nil,
		// EMPTY (§1.18), the deliberately INCOMPLETE Icom pair: the record
		// carries no duplex selector and no offset magnitude, and split is
		// expressed only as two frames — which on this row is a field
		// nothing publishes at all (M-E2). A row that later opened
		// FieldDuplex without supplying this would fail Validate loudly,
		// which is the correct direction.
		DuplexOptions: nil,
		// THREE values, MANUAL-EVIDENCED (§1.19): MR/MW P7 reads "0: OFF,
		// 1: TONE, 2: CTCSS" (480:964, and identically on MR at 480:922).
		// The 590 pair's fourth, "3: Cross Tone ON" (590:1549-1553), has no
		// counterpart in this legend — one of the two radios' divergences
		// (§5).
		//
		// THE SEMANTICS ARE K-D1 AND ARE ASSUMED, WHICH THE VALUES ARE NOT.
		// This book prints the three as bare labels and never says which
		// direction each acts in; the only sentence that does is
		// 590:1167-1171, an IF note in the OTHER book, which decision 13
		// does not let reach this row. So ToneModeCTCSS for 1 and
		// ToneModeCTCSSRxSquelch for 2 are K-D1's claim, registered in
		// doc.go with its lift (L-DOC-5 or L-HW-20), not a reading of
		// 480:964.
		//
		// THE TONE ENTRY HAS NeedsTxTone() TRUE AND NO TONE DOMAIN BEHIND
		// IT, and that is stated rather than hidden (§1.19). Nothing in
		// core/spec couples the two, so this validates; the honest reading
		// is that a TS-480 channel's tone MODE round-trips and its tone
		// VALUE does not — a documented lossy round trip, and the reason
		// Q2's interim refusal rides inside A22's on the write path.
		// Publishing OFF alone instead would refuse to READ the channels
		// that carry a tone, which is worse.
		//
		// Canonical is false on every entry: no semantic is expressed
		// twice, so spec.Validate's canonical rule requires nothing.
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS},
			{Value: "CTCSS", Semantics: spec.ToneModeCTCSSRxSquelch},
		},
		// Both EMPTY (§1.20, §1.21): see FieldDTCSCode in bankFields.
		DTCSPolarities: nil,
		DTCSCodes:      nil,
		// EMPTY (§1.22): byte 28 is "Always 0 for the TS-480." (480:973) —
		// one of this row's sixteen printed-fixed parameter bytes. There is
		// no per-channel filter selection to publish. This is a plain
		// absence, unlike the TS-590S's empty list, which is a
		// firmware-conditional REFUSAL (§2.7).
		Filters: nil,
		// EMPTY / nil (§1.23, §1.24), and this empty is a REFUSAL rather
		// than an absence — the one place on this row where the two differ.
		// Bytes 39-40 ARE a step: "Step size. Refer to the ST command."
		// (480:979). ST's legend is MODE-CONDITIONAL over two different
		// ranges — 00 ~ 04 for SSB/CW/FSK and 00 ~ 09 for AM/FM, with index
		// 00 meaning 0.5 kHz in the first and 5 kHz in the second
		// (480:1494-1500) — and Capabilities.TuningSteps is a flat label
		// list with no mode axis, so NO HONEST VOCABULARY EXISTS. That is
		// A22, and its consequence is the milestone's largest single
		// refusal: with the field Unsupported the source channel never
		// retains the raw P14 index, so this programme cannot tell a
		// non-default step from a default one, and every TS-480 channel
		// write is refused (decision 12, write.go).
		TuningSteps:            nil,
		ProgramTuningStepRange: nil,
		// All EMPTY (§1.25-§1.27), with the caveat bankFields states: the
		// radio has an attenuator and a pre-amplifier as radio-level
		// functions (480:1185, 480:1055-1058) and no record position for
		// either; nothing here says it has none.
		AttenuatorDB:   nil,
		PreampOptions:  nil,
		AntennaOptions: nil,
		// EMPTY (§1.28), a CHOICE taking the strict direction, and on THIS
		// row the default is STRICTER THAN THE BOOK rather than equal to
		// it. The empty string selects the family default — printable ASCII
		// 0x20-0x7E excluding ';' — while P16 here carries no charset note
		// at all (480:984) and the general rule is only "Do not use the
		// control characters 00 to 1Fh since they are either ignored or
		// cause a '?' answer" (480:127-129), which excludes neither 0x7F
		// nor 0x80-0xFF. Narrowing rather than widening is the direction
		// that cannot put an unexpected byte on the wire. THE CLAIM IS
		// BOUNDED AT 0x7F and A2 says so in terms: 0x80-0xFF is unevidenced
		// AND unclaimed.
		TagCharset: "",
	}
}

// CapabilitiesUnverified is this row's all-Unverified FAIL-SAFE profile, and
// it is what a RealHardware session gets today: every field the 50-byte record
// expresses is labelled Read Unverified / Write Unverified — documented in the
// PC-command reference and exercised against scripted peers, but never proven
// against a radio — and every field the record does not express stays the zero
// FieldSupport.
//
// Because Unverified makes FieldSupport.CanWrite false, this profile AS
// LABELLED blocks every write project-wide: codeplug.Diff refuses the change,
// the clone service refuses to execute a plan containing it, and
// Session.WriteChannel re-checks and refuses before building a frame. It is
// also what any UNRECOGNISED Profile value selects — the failure direction is
// always "nothing writable" (matrix §2.1).
//
// THE ROUTE PAST IT IS THE USER'S OWN CONSENT, AND ON THIS ROW IT REACHES
// NOTHING. A session opened with WithConsentedUnverifiedWrites re-labels these
// write-side fields spec.ConsentedUnverified, so a consented session passes
// the capability gate — and then meets A22, which refuses every channel write
// this row can be asked for (write.go). Consent widens WHAT may be attempted,
// never HOW carefully.
//
// The READ labels are Unverified rather than Supported for the same reason
// (§2.1, "the honest one"): this driver's read path is exercised against a
// fake and a manual, and no TS-480 has ever answered a frame.
//
// NO FIELD IS spec.Inert. Inert is the FT-710's HARDWARE finding about the
// FT-710; no Kenwood radio has been asked anything, so there is no finding to
// record and borrowing one would answer a question about one radio with
// another radio's evidence.
func CapabilitiesUnverified() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified})
}

// CapabilitiesSimulated is this row's internal/fakets480-backed profile (CLI
// --fake, GUI demo) and NEVER a real radio: Read AND Write Supported for
// exactly the five fields the 50-byte record expresses on this row.
//
// Against the fake, hardware risk is moot and the choreography itself is what
// is being exercised end to end, so claiming Supported here is a claim about
// internal/fakets480 and about nothing else (§2.1). It still does not make a
// channel writable: A22 refuses every TS-480 channel write on every profile,
// which is why the write path's own tests pin A22 on THIS profile — a session
// that has already passed the capability gate is the only place the semantic
// refusal can be seen at all.
func CapabilitiesSimulated() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Supported, Write: spec.Supported})
}
