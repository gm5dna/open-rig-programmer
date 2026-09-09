// SPDX-License-Identifier: GPL-3.0-or-later

package ts890

import (
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// The registry key and the three-digit identity this radio answers "ID;"
// with (matrix §1.1, §1.2).
//
// THE NAME IS A CHOICE OVER A MANUAL-EVIDENCED FACT: the ID answer's P1
// legend prints the model name beside the number, "024: TS-890S"
// (890:2733), in the same hyphenated form the registry key uses, so the
// choice is only that this string and not "TS890S" is the key.
//
// P1 IS THREE DIGITS and a later reader must not "fix" it into four:
// driver.Identity's CATID doc comment gives the convention as four hex
// digits on Yaesu and a CI-V address on Icom, and core/spec constrains
// CATID only to be non-empty. "ID;" is three bytes and its answer six
// (890:2735, 890:2738).
const (
	modelName = "TS-890S"
	catID     = "024"
)

// Profile selects which capability profile New builds the driver with.
//
// The zero value is RealHardware ON PURPOSE (matrix §2.1): a forgotten or
// zero-valued Profile must fail towards the real-hardware capability set —
// which for this row is the all-Unverified one, nothing writable — and NEVER
// towards the simulator's, whose Supported writes are a claim about
// internal/fakets890 and about nothing else. Any OTHER unrecognised Profile
// value fails the same way, through Capabilities' explicit default arm.
// Shared with every other driver package (core/driver.Profile); this package
// keeps its own Simulated selector, which
// internal/guards.TestSimulatedProfileTokensConfinement requires.
type Profile = driver.Profile

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// writeTrialsComplete is THIS package's hardware write guard, and it is
// FALSE: no TS-890S has ever been written to by this project — none has ever
// been ASKED anything at all (matrix §3.12).
//
// While it is false there is no hardware-verified capability profile for this
// row to select AT ALL, deliberately not even a placeholder one: a
// RealHardware session gets CapabilitiesUnverified, nothing is writable
// anywhere, and the capability gate refuses every write before a frame is
// built.
//
// It is consulted by no production code, and that is the point. Flipping it
// is a TWO-PART change — this constant AND a CapabilitiesRealHardware profile
// built field class by field class from this radio's trial evidence, AND the
// Capabilities switch rewritten to select it — so a one-character edit cannot
// unlock a write. TestWriteTrialsComplete_PinnedFalse asserts both halves,
// the constant and the nothing-writable consequence.
//
// ONE CONSTANT PER PACKAGE, WHICH IS ONE PER ROW HERE (plan P18). A trial on
// a TS-890S lifts the TS-890S and nothing else, and core/driver/ts990's own
// constant is a different constant in a different package, which is what
// makes that structurally obvious.
const writeTrialsComplete = false

// memBankLabel is this row's MEM display label, minted as THIS package's own
// const.
//
// A DISPLAY LABEL IS NOT A PROTOCOL FACT (matrix §1.4.1, marked CHOICE
// there). It coincides with several siblings' today because the neutral bank
// ID means the same thing to a user across the app, and nothing forces them
// to stay equal.
const memBankLabel = "Memories"

// narrowSuffix is the name this project appends for a narrow FM record.
//
// IT IS THIS PROJECT'S SPELLING AND NOT THE BOOK'S (matrix §1.5): neither
// book prints a name for a narrow FM record at all, because neither prints
// names for width-byte combinations in the first place. Pair 1 established
// the suffix on the 590 rows and this package inherits the SPELLING —
// nothing else — for consistency across the manufacturer.
const narrowSuffix = "-N"

// ctcssTones890 is the 51-entry tone chart, index = the CAT tone number,
// transcribed IN THIS PACKAGE from TN's own printed table (890:5149-5163,
// the 1750.0 entry at 890:5162). Index 99 is "To default" and is "a setting
// command only" (890:5163, 890:5166): it is not a tone and is not here.
//
// IT IS NOT THE PROJECT'S SHARED CHART AND IT MUST NOT NAME ITS ACCESSOR
// (plan P6, matrix §1.9). The shared 50-tone table in core/spec/tones.go is
// the Yaesu family's; this chart happens to be that table PLUS 1750 Hz, in
// the same order, and a coincidence of contents is not a licence to alias —
// a Kenwood row's tone chart must be a reading of a Kenwood chart.
// TestCTCSSTones_AreNotTheProjectsSharedChart is the inequality pin and
// TestNoProductionFileNamesTheSharedToneChart holds the production half down.
//
// IT IS ALSO NOT PAIR 1'S 43-ENTRY KENWOOD CHART, and the difference is
// eight interstitial tones this book prints and the 590's does not — 159.8,
// 165.5, 171.3, 177.3, 183.5, 189.9, 196.6 and 199.5, indices 26, 28, 30, 32,
// 34, 36, 38 and 39 here. So THE TONE DOMAIN IS PER ROW AND NOT PER
// MANUFACTURER, and every index above 25 differs from pair 1's for the same
// frequency.
//
// THE 51ST ENTRY IS AN OVER-CLAIM ON THE RECEIVE SIDE, and that is pair 1's
// M-E1 recurring rather than an oversight. spec.Capabilities carries ONE tone
// domain and ONE predicate, AdmitsTone, which codeplug.ToneField.Valid
// applies to ToneTx and ToneRx alike, so a 51-entry list admits 1750 Hz as a
// tone_rx value the CN chart (00-49 plus 99, 890:1354-1369) does not print.
// The resolution is to publish 51 and refuse a Known tone_rx of 1750 Hz IN
// THE WRITE PATH (matrix §2.6 rung 8, the ladder's) — a runtime refusal, not
// a claim in the capability table. Publishing 50 instead would refuse on
// transmit an index TN itself prints, which is strictly worse.
//
// spec.Validate requires CTCSSTones to be strictly ascending precisely so the
// slice index doubles as the CAT tone number; 254.1 → 1750.0 is ascending.
var ctcssTones890 = []spec.Tone{
	670, 693, 719, 744, 770, 797, 825, 854, 885, 915, 948, 974,
	1000, 1035, 1072, 1109, 1148, 1188, 1230, 1273, 1318, 1365,
	1413, 1462, 1514, 1567, 1598, 1622, 1655, 1679, 1713, 1738,
	1773, 1799, 1835, 1862, 1899, 1928, 1966, 1995, 2035, 2065,
	2107, 2181, 2257, 2291, 2336, 2418, 2503, 2541,
	17500,
}

// layout is the codec layout this row speaks. core/kw/ma holds both rows'
// layouts in one package; this function only names ours.
func layout() ma.Layout { return ma.Layout890() }

// modeName is THE ONE mode-naming rule in this package, and both the
// capability list and the read path go through it so the name a channel is
// published with is BYTE-FOR-BYTE the one Capabilities.Modes advertises.
//
// TWO BYTES, NOT ONE (matrix §1.5). MA0 P3 is the mode, whose legend is OM
// P2's (890:3174-3175, 890:3976-3992), and MA0 P4 is a SEPARATE "FM Normal/
// Narrow information" flag, "0: Normal / 1: Narrow" (890:3176-3178). The
// legend prints no narrow names, so FM-N and FM-D-N are SYNTHESISED here,
// which is why this row publishes sixteen names over fourteen live legend
// values.
//
// THE WIDTH BYTE IS APPLIED TO EVERY FM-BEARING NAME, INCLUDING THE DATA
// ONES, AND THAT IS A CHOICE WITH A PUBLISHED COST (matrix §1.5): the legend
// does not scope P4 to a particular mode value, so if the radio ignores the
// byte on FM-D this programme publishes two names for one radio state — a
// cosmetic redundancy the read path reports honestly. Scoping it to plain FM
// instead would mean emitting a width byte with no source on every data-FM
// write, and a defaulted byte is the more expensive error.
//
// THE FM-BEARING SET IS ASKED OF THE LEGEND ITSELF and not listed here: a
// name this book spells beginning "FM" is an FM mode, which on this row is
// '4' FM and 'E' FM-D and nothing else (890:3976-3992). A second local list
// of the two would be a copy of the legend able to drift from it.
//
// ON A NON-FM MODE THE WIDTH BYTE DOES NOT REACH THE NAME, which is
// kw.RecordModeName's own rule one book over: what P4 means outside FM is
// nowhere printed, and inventing "LSB-N" would publish a name
// Capabilities.Modes does not carry and codeplug.Validate would then refuse.
func modeName(l ma.Layout, mode byte, fmNarrow bool) (string, bool) {
	base, ok := l.ModeName(mode)
	if !ok {
		return "", false
	}
	if fmNarrow && strings.HasPrefix(base, "FM") {
		return base + narrowSuffix, true
	}
	return base, true
}

// modeNames returns the selectable mode display names this row advertises,
// DERIVED FROM THE LAYOUT rather than transcribed here.
//
// There is deliberately no local mode table. MA0's P3 carries no legend of
// its own — it says "Refer to the P2 value of the OM command"
// (890:3174-3175) — so the memory mode vocabulary IS OM P2's, and core/kw/ma
// transcribed it once from 890:3976-3992. Enumerating the layout makes a
// drifting second transcription unrepresentable.
//
// WIRE ORDER COMES FREE because the legend's key IS the wire byte, so
// ascending byte order is the book's own order — '1'-'9' then 'A'-'F' on this
// row. Values '0' and '8' are printed "Unused" (890:3977, 890:3985); the
// layout does not carry them, so they cannot appear here.
//
// The narrow twin follows its base immediately, which is the matrix's own
// ordering of the sixteen (§1.5).
func modeNames(l ma.Layout) []string {
	legend := l.ModeNames()
	var names []string
	for b := 0; b <= 0xFF; b++ {
		if _, ok := legend[byte(b)]; !ok {
			continue
		}
		name, _ := modeName(l, byte(b), false)
		names = append(names, name)
		if narrow, _ := modeName(l, byte(b), true); narrow != name {
			names = append(names, narrow)
		}
	}
	return names
}

// memSlots returns this row's MEM inventory, "000".."099", built through the
// LAYOUT's own NewSlot so the wire forms this capability data advertises are
// the ones the codec actually accepts.
//
// THE CLASS FILTER IS WHAT OMITS 100-119, and that is plan P11 and matrix
// M-E1. The layout DECLARES those two classes because the book prints them —
// "Channels P0 ~ P9 are represented as 100 ~ 109 and channels E0 ~ E9 are
// represented as 110 ~ 119" (890:3168-3169) — and a codec that refused such a
// number could not parse a front-panel-written answer at all. What a MA0 read
// of one ANSWERS is nowhere printed (A9 for the Programmable VFO class, A10
// for the E channels), and what the record's SECOND frequency means on a
// Programmable VFO slot is A7 — MA6 sets an END frequency as a command of its
// own (890:3315-3323), and whether MA0's split side carries it is unprinted.
// So the twenty slots are published NOWHERE: not appended to MEM, not a bank
// of their own, not slot IDs anywhere on this row.
// TestBanks_NoSlotInTheUpperClassesIsPublished is the pin.
//
// Selecting on kw.SlotMemory rather than on a number range is what keeps the
// codec's domain and the driver's inventory a single edit apart from each
// other rather than two independent lists.
//
// Walking l.Slots() rather than probing every number 0-999 costs nothing: the
// layout already publishes its own ranges, so there is no ceiling to derive or
// defend. The sibling row landed this at T13 and this one did not (review
// s2-close-review-opus-2.md LOW-2).
func memSlots(l ma.Layout) []string {
	var slots []string
	for _, r := range l.Slots() {
		if r.Class != kw.SlotMemory {
			continue
		}
		for n := r.Lo; n <= r.Hi; n++ {
			s, err := l.NewSlot(n)
			if err != nil {
				continue
			}
			slots = append(slots, s.String())
		}
	}
	return slots
}

// bankFields builds the one bank's per-field support map. ALL TWENTY-SEVEN
// spec.Fields are listed explicitly, including the nineteen that are the zero
// FieldSupport: a field left out of the map reads identically to a field
// deliberately zeroed (Capabilities.FieldSupport returns the zero value for
// an absent key), and only a written-down zero is legible as a decision
// (matrix §2, §2.1). Coverage is asserted against spec.AllFields() (plan P5).
//
// EIGHT FIELDS ARE GRADED AND NINETEEN ARE ZERO, and on this row there is no
// per-bank or per-context variation at all: one bank, one grade each (§2.2).
//
// A fresh map per call, so no two banks share one.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		// The eight the 40-to-50-byte record expresses.
		spec.FieldFrequency: rw, // P2, 11 digits at bytes 7-17 (890:3171-3172)
		spec.FieldMode:      rw, // P3 at byte 18, legend via OM (890:3174-3175)
		spec.FieldTag:       rw, // P13 from byte 40, up to 10 chars (890:3208-3209)
		spec.FieldScanSkip:  rw, // P12 at byte 39, 0/1 (890:3205-3207)
		spec.FieldToneMode:  rw, // P5 at byte 20, four values (890:3180-3185)
		spec.FieldToneTx:    rw, // P6 at 21-22 via TN (890:3186-3187)
		spec.FieldToneRx:    rw, // P7 at 23-24 via CN (890:3188-3190)

		// P8 at bytes 25-35 with the split flag P11 at byte 38
		// (890:3191-3192, 890:3201-3203). GRADED ONCE, IN MEM, FOR 000-099
		// — matrix M-E7. The design's own wording graded it "Unavailable
		// for 100-119", and that word belongs to a layer that never sees
		// those slots: codeplug.FieldState is a per-CHANNEL word, and no
		// channel is ever produced for a class published in no bank.
		//
		// IT IS THIS PAIR'S LARGEST GAIN OVER PAIR 1 (§2.4). One MA0 answer
		// returns both frequencies AND the flag, where the 590 rows needed
		// a second frame nobody could send blind and had to publish every
		// channel's TX side Unavailable.
		spec.FieldTxFrequency: rw,

		// MANUAL-EVIDENCED absence FROM THE RECORD, over the COMPLETE
		// thirteen-parameter account (890:3164-3209). NOT a consequence of
		// the vocabulary rule, which constrains exactly one pair and does
		// not name this field (matrix M-E4, M-E5) — and THIS RADIO HAS RIT
		// AND XIT, as radio-level settings no memory channel stores: RT
		// "RIT Function State, RIT Shift" (890:4558) and XT (890:5411). The
		// zero means "no per-channel clarifier field", never "this radio has
		// no clarifier". ClarMaxHz and ClarStepHz are 0/0 in consequence.
		spec.FieldClarifier: {},

		// The Yaesu half of the vocabulary pair, which
		// core/spec/field.go:41-44 forbids a model expressing alongside the
		// Icom half. This record expresses tone as P5's four-value mode
		// selector with two INDEPENDENT indices, which is the
		// tone_mode/tone_tx/tone_rx shape, and repeater operation as an
		// absolute second frequency rather than a shift selector.
		// FieldCTCSSTone is additionally ONE field where the record carries
		// TWO indices (P6 and P7).
		spec.FieldCTCSSState: {},
		spec.FieldCTCSSTone:  {},
		spec.FieldShift:      {},

		// No tag-display flag anywhere in the grid; the thirteen parameters
		// account for every byte of it.
		spec.FieldTagDisplay: {},

		// A CHOICE OVER A PRINTED ERASE COMMAND, not over ambiguous
		// evidence — matrix M-E4, §2.8. This book carries a DEDICATED MA5
		// "Memory Channel (Channel Deletion)" (890:3305, its channel-class
		// note at 890:3311), which is STRONGER evidence than pair 1's
		// ambiguous short-MW side effect. The standing no-erase rule
		// declines to build it: this programme does not delete a user's
		// channels. core/kw/ma's outbound gate refuses an MA5 frame
		// besides, so the guard is positive rather than negative.
		spec.FieldErase: {},

		// MANUAL-EVIDENCED absence, and here it is TOTAL rather than
		// record-local (§1.18): a case-insensitive search of this book for
		// "duplex" returns ZERO hits — no duplex command, no legend, no menu
		// row. There is no per-channel offset magnitude either; the only
		// offsets printed anywhere are a transverter offset (XO,
		// 890:5389-5395) and a marker/carrier offset, both radio-level.
		spec.FieldDuplex: {},
		spec.FieldOffset: {},

		// DCS APPEARS NOWHERE IN THIS BOOK — a case-insensitive search for
		// "DCS" (which subsumes DTCS) returns zero hits, in any command, any
		// legend, any menu row (§1.20, §1.21). Unlike the FT-891, whose
		// matrix had to say carefully that the RADIO has DCS while the
		// record has no field for it, this is the strongest form of the
		// absence and needs no caveat.
		spec.FieldDTCSCode:     {},
		spec.FieldDTCSPolarity: {},

		// MANUAL-EVIDENCED absence FROM THE RECORD, with the M-E5 caveat,
		// and this is where the pair diverges sharply from the TS-590SG,
		// whose byte 28 IS a live FILTER A/B selector. THIS RADIO HAS
		// PER-MODE FILTER SELECTION as four radio-level commands: FL0
		// "Select the Receive Filter" (890:2410), FL1 "Roofing Filter"
		// (890:2431), FL2 "IF Filter Shape" (890:2458) and FL3 "AF Filter
		// Type" (890:2480). A codeplug round trip through this programme
		// does not preserve a channel's filter — and cannot, because the
		// record does not carry one.
		spec.FieldFilter: {},

		// MANUAL-EVIDENCED absence, AND THE WHOLE OF ERRATUM M-E3 — the
		// sharpest single divergence from pair 1, whose 590 rows grade this
		// field from byte 19 "refer to the DA command". THIS RADIO HAS NO DA
		// COMMAND AND THIS GRID HAS NO DATA BYTE: the data-ness is inside
		// the mode LEGEND, as the names LSB-D, USB-D, FM-D and AM-D at
		// legend values C-F (890:3989-3992). So a data channel is not a
		// channel with a flag set; it is a channel in a different mode, and
		// Capabilities.Modes carries all four names.
		spec.FieldDataMode: {},

		// MANUAL-EVIDENCED absence, and stronger than the record account
		// alone: THIS RADIO HAS NO ST COMMAND AT ALL — a search of this book
		// for a command block whose mnemonic is ST returns zero hits (§1.23,
		// §1.24). Step sizes exist only as EX menu rows, which are
		// radio-level settings reached through the settings surface. The
		// consequence is a large one stated positively: THE TS-480'S BLANKET
		// WRITE REFUSAL HAS NO COUNTERPART HERE. Pair 1 had to refuse every
		// TS-480 channel write because the source channel could not retain a
		// raw step index; this record has no step to lose.
		spec.FieldTuningStepEnabled: {},
		spec.FieldTuningStep:        {},
		spec.FieldProgramTuningStep: {},

		// MANUAL-EVIDENCED absence FROM THE RECORD, with the M-E5 caveat
		// again: this radio HAS an attenuator, a pre-amplifier and an
		// antenna selector as RADIO-LEVEL commands — RA "Attenuator"
		// (890:4391), PA "Pre-amplifier" (890:4000), AN "Antenna Selection"
		// (890:214) — and none of them has a position among the thirteen
		// parameters. Empty means "no per-channel field", NOT "this radio
		// has no such function".
		spec.FieldAttenuator: {},
		spec.FieldPreamp:     {},
		spec.FieldAntenna:    {},

		// An Icom concept with no position in this frame and no mention
		// anywhere in this book.
		spec.FieldIPPlus: {},
	}
}

// baseCapabilities assembles this row's static baseline at the given evidence
// grade.
//
// ALL TWENTY-EIGHT spec.Capabilities fields are populated explicitly — the
// non-zero ones and the deliberately EMPTY ones alike — and
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
func baseCapabilities(rw spec.FieldSupport) spec.Capabilities {
	l := layout()
	return spec.Capabilities{
		Model: modelName,
		CATID: catID,
		// MANUAL-EVIDENCED (§1.3): an HF transceiver with a documented
		// transmit surface in its own book — TX "Transmission Mode"
		// (890:5201-5205), the TN transmit-tone command whose whole purpose
		// is what the radio sends (890:5143-5147), and this record's own
		// split-transmission side (890:3191-3203). spec.Validate refuses the
		// zero value, so this is a declaration the row must make.
		Transmit: spec.HasTransmitter,
		Banks: []spec.Bank{
			{
				ID:    spec.BankMemory,
				Label: memBankLabel,
				Slots: memSlots(l),
				// NoBlank FALSE, STATED rather than left to the zero value
				// (plan P11, matrix §2.5). NoBlank means "every slot in
				// this bank must be populated" — the Yaesu PMS invariant —
				// and nothing in this book imposes it; the book says the
				// opposite, documenting the empty channel as a normal state
				// with a defined answer ("When reading a blank channel,
				// parameters P2 to P12 becomes blank", 890:3215-3216), and
				// printing an MA5 deletion command whose existence
				// presupposes that a channel may be empty. A NoBlank MEM
				// bank would make codeplug.Validate refuse every candidate
				// with a single blank channel, which on a hundred-slot
				// space is every real codeplug.
				NoBlank: false,
				Fields:  bankFields(rw),
				// Sparse FALSE, stated, with all five sparse-space fields
				// left zero as spec.Validate requires when it is: this is a
				// small dense space fully printed in the book
				// (890:3167), not an Icom-style
				// group-addressed one where Slots lists only what a read
				// materialised.
				Sparse: false,
			},
			// THERE IS NO SCAN BANK ON THIS ROW, where pair 1 has one, and
			// the reason is evidential rather than vocabulary (matrix
			// §1.4.5). spec.BankScan would be available. But the 590 book
			// prints which P1 byte selects a section channel's start
			// frequency and which its end, so a two-slot-per-index bank was
			// a READING; this book prints no such selector — MA0's Read form
			// is MA0 + channel + ';' and carries nothing else (890:3186) —
			// and the end frequency lives in MA6, which this milestone never
			// sends. A "100L"/"100U" bank here would be twenty slot
			// identities invented out of a command the programme does not
			// build.
		},
		// SIXTEEN published names from fourteen live legend values plus the
		// two synthesised narrow twins; see modeName and modeNames (§1.5).
		Modes: modeNames(l),
		// MANUAL-EVIDENCED WIDTH (§1.6): P13 is "Channel name / Up to 10
		// characters" (890:3208-3209). TEN, not pair 1's eight, and it is
		// per row rather than per manufacturer.
		//
		// WHAT HAPPENS TO A SHORT NAME IS NOT AN ASSUMPTION ON THIS ROW.
		// The terminator FLOATS at 40 + len(name) — the wrap row prints "x"
		// above the second cell with P13 and ';' beneath it (890:3181-3182)
		// — so nothing is padded, "AB ;" and "AB;" are DISTINCT frames, and
		// core/kw/ma carries P13 VERBATIM in both directions. A trailing
		// space is real content here, not padding: this row's A1 half is
		// explicitly NOT an assumption (core/kw/ma/doc.go's A1, the C-MED-1
		// reversal). The TS-990S's fixed ten-byte window is the row that
		// assumes a pad.
		TagLen: 10,
		// 0/0, and the reason is the RECORD ACCOUNT rather than the
		// vocabulary rule — matrix M-E4, M-E5, and the design does not state
		// it at all. The thirteen parameters account for every byte of the
		// grid (890:3164-3209) and NONE OF THEM IS AN RIT/XIT OFFSET. See
		// FieldClarifier in bankFields for the radio-level counterparts.
		// spec.Validate places no constraint on a zero ClarMaxHz, and
		// FieldClarifier is Unsupported on this row's one bank, so nothing
		// consults these.
		ClarMaxHz:  0,
		ClarStepHz: 0,
		// The 51-entry chart; see ctcssTones890. Copied per call so no two
		// capability values share the backing array.
		CTCSSTones: append([]spec.Tone(nil), ctcssTones890...),
		// nil (§1.10): the tone field is an INDEX into a printed chart, not
		// a number — P6 "Refer to the P1 value of the TN command"
		// (890:3186-3187) and P7 the same for CN (890:3188-3190). A radio
		// declares a list or a range, never both, and Validate refuses the
		// two together.
		CTCSSToneRange: nil,
		// FIVE rates, and 4800 is deliberately OMITTED (§1.11). The book
		// prints six — "Baud Rate Selectable from 4800*/ 9600/ 19200/ 38400/
		// 57600/ 115200 bps" (890:13-14) — with the asterisk resolving on
		// the same page to "4800 bps cannot be used with the USB-B
		// connector" (890:20). A Bauds list is ONE FLAT LIST PER ROW with no
		// per-path axis, and USB-B is the connector this book's own "Using a
		// USB Cable" section steers a PC user to (890:31-42). Publishing
		// 4800 would promise a rate this programme cannot deliver on the
		// ordinary connection, and the failure mode would be a timeout that
		// looks like a dead port. The 4800 parenthesis in the framing table
		// — two stop bits at that rate alone (890:17) — is the second reason
		// and would cost this session its one-stop-bit framing besides.
		Bauds: []int{9600, 19200, 38400, 57600, 115200},
		// ASSUMED — A11, and an OPERATIONAL ASSUMPTION rather than a
		// conservative choice (§1.12, §3.3). The book prints only the
		// selectable list and marks no default (890:13-14). A WRONG BAUD IS
		// NOT A SAFE BAUD — it is an unreachable radio, and the symptom is a
		// timeout that looks like a dead port. NO AUTO-BAUD: probing by
		// re-opening the port at five speeds is a discovery mechanism this
		// programme does not have and would not test. A11's lift cannot be a
		// wire observation — reading it back over EX presupposes a session at
		// the very baud in question — so it is the instruction manual's baud
		// menu page, an owner reading the front panel, or a decision.
		// spec.Validate requires this value to appear in Bauds, and it does.
		DefaultBaud: 9600,
		// 0/0 — CANNOT ESTABLISH, A15, the registered core/driver/icr8600
		// and core/driver/ts590 precedent (§1.13, §1.14). THIS PC-COMMAND
		// DOCUMENT PRINTS NO FREQUENCY RANGE FOR THIS RADIO: MA0 P2 says
		// only "Frequency information (11 digits in Hz.)" with "Blank digits
		// must be entered as '0'" (890:3171-3172), and the front-matter FA
		// worked example gives 00007000000 for 7 MHz (890:116-118). A FIELD
		// WIDTH IS NOT A TUNING RANGE, and publishing one as the other would
		// put a fabricated capability in the capability table.
		//
		// A ZERO IS A DISABLED CHECK, WHICH IS THE POSITIVE PROPERTY HERE:
		// codeplug.Validate's floor and ceiling each test != 0 first
		// (core/codeplug/validate.go:289, :295), so no false refusal and no
		// false promise. The 11-digit encodability guard is not lost — it
		// lives in the codec, where a frequency beyond the field is an
		// out-of-domain error at BUILD time, which is a statement about the
		// wire and is true. A15's lift is L-DOC-2, this radio's INSTRUCTION
		// MANUAL's General Specifications table; no wire observation lifts
		// it.
		MinFreqHz: 0,
		MaxFreqHz: 0,
		// nil (§1.15): RequiredSlots names individual slots that must never
		// be empty — the FT-710's M-01. This book marks no channel
		// mandatory and says the opposite (890:3215-3216). Distinct from
		// Bank.NoBlank, which is also false.
		RequiredSlots: nil,
		// Both EMPTY (§1.16, §1.17): the Yaesu half of the vocabulary pair,
		// which core/spec/field.go:41-44 forbids a model expressing
		// alongside the Icom half. Empty is legal here only because no bank
		// grades FieldShift or FieldCTCSSState above Unsupported, which is
		// what spec.Validate's own pair rules are conditional on
		// (core/spec/validate.go:380, :415).
		ShiftOptions: nil,
		CTCSSStates:  nil,
		// EMPTY (§1.18), AND THIS ROW PUBLISHES ONLY ONE HALF OF THE ICOM
		// HALF, deliberately: ToneModes below is non-empty and this is not,
		// because the MA0 grid carries no duplex selector and no offset
		// magnitude — split is an absolute second frequency plus a flag, not
		// a shift. That is legal for the reason above, and a row that later
		// opened FieldDuplex without supplying this would fail Validate
		// loudly, which is the correct direction.
		//
		// It is also what makes a blank CHIRP Duplex import as SIMPLEX on
		// this row under v1.5.0 lane L: a radio declaring no shift
		// vocabulary has the blank as its only state. "off" still blocks.
		DuplexOptions: nil,
		// FOUR values, MANUAL-EVIDENCED (§1.19): MA0 P5 is "FM tone type —
		// 0: OFF / 1: Tone / 2: CTCSS / 3: Cross Tone" (890:3180-3185),
		// corroborated by the standalone TO command's identical legend
		// (890:5170-5178).
		//
		// THE SEMANTICS ARE ASSUMED AND THE REGISTER ENTRY IS K-D1 (matrix
		// M-E2, doc.go). Pair 1's mapping rested on ONE sentence in the 590
		// book — IF P14's — and THIS BOOK CONTAINS NEITHER THAT SENTENCE NOR
		// AN IF BLOCK AT ALL. So the assignment of ToneModeCTCSS (transmits
		// a tone, requires none on receive) to value 1 and
		// ToneModeCTCSSRxSquelch (requires a received tone, transmits none)
		// to value 2 is borrowed from another radio's book, and with it the
		// assignment of P6 to tone_tx and P7 to tone_rx. Publishing OFF
		// alone until K-D1 lifts would refuse to READ every channel carrying
		// a tone, which is worse.
		//
		// CROSS IS DECLARED AND IS STRUCTURALLY USABLE, and that is stated
		// rather than assumed: ToneModeCross requires a model that also
		// expresses the fields the combination needs
		// (core/spec/vocab.go:261-266), and this record has an independent
		// tone index and an independent CTCSS index side by side. Whether
		// the DIRECTION is the 590's is K-D1's question, not this one's.
		//
		// Canonical is false on every entry: no semantic is expressed twice.
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS},
			{Value: "CTCSS", Semantics: spec.ToneModeCTCSSRxSquelch},
			{Value: "CROSS", Semantics: spec.ToneModeCross},
		},
		// Both EMPTY (§1.20, §1.21): see FieldDTCSCode in bankFields — zero
		// hits for DCS anywhere in this book.
		DTCSPolarities: nil,
		DTCSCodes:      nil,
		// EMPTY (§1.22), with bankFields' M-E5 caveat: this radio has FL0-FL3
		// at radio level and no per-channel filter field.
		Filters: nil,
		// EMPTY / nil (§1.23, §1.24): no step field in the grid, and no ST
		// command in the book at all.
		TuningSteps:            nil,
		ProgramTuningStepRange: nil,
		// All EMPTY (§1.25-§1.27), with bankFields' M-E5 caveat: RA, PA and
		// AN are radio-level commands with no per-channel position. Nothing
		// here says the radio has none.
		AttenuatorDB:   nil,
		PreampOptions:  nil,
		AntennaOptions: nil,
		// EMPTY (§1.28), a CHOICE taking the STRICT direction. The empty
		// string selects the family default — printable ASCII 0x20-0x7E
		// excluding ';' (spec.Capabilities.TagByteOK).
		//
		// THE BOOK PRINTS NO CHARSET FOR THE MEMORY NAME: P13 is "Channel
		// name / Up to 10 characters" (890:3208-3209) and carries no
		// character rule. What it DOES print is a global coding rule —
		// ASCII, with "the letters assigned to 80h ~ FFh are replaced as
		// follows by Menu 9-01 (Keyboard Language)" (890:31-43) — so the
		// range above 0x7E is not merely unevidenced: its MEANING depends on
		// a menu setting this programme does not read. Narrowing to 0x7E is
		// the direction that cannot put a byte of ambiguous meaning on the
		// wire. THE ';' EXCLUSION IS FORCED AND IS NOT AN ASSUMPTION: it
		// terminates a frame, and this book says the terminator's position
		// "differs depending on the command used" (890:92-96), so a name
		// containing one would split the frame at the radio's own parser.
		// A2 is the register home and its claim is bounded at 0x7E.
		TagCharset: "",
	}
}

// CapabilitiesUnverified is this row's all-Unverified FAIL-SAFE profile, and
// it is what a RealHardware session gets today: every field the MA0 record
// expresses is labelled Read Unverified / Write Unverified — documented in
// the PC-command reference and exercised against scripted peers, but never
// proven against a radio — and every field the record does not express stays
// the zero FieldSupport.
//
// Because Unverified makes FieldSupport.CanWrite false
// (core/spec/support.go:141), this profile AS LABELLED blocks every write
// project-wide: codeplug.Diff refuses the change, the clone service refuses
// to execute a plan containing it, and Session.WriteChannel re-checks and
// refuses before building a frame. It is also what any UNRECOGNISED Profile
// value selects — the failure direction is always "nothing writable" (matrix
// §2.1).
//
// THE ONE ROUTE PAST THAT, and it is the user's own: a session opened with
// WithConsentedUnverifiedWrites re-labels these write-side Unverified fields
// spec.ConsentedUnverified at session-capability assembly, and CanWrite is
// true for that state. The profile keeps saying the true thing either way: it
// describes the EVIDENCE (none), and consent is a decision about risk.
// CONSENT WIDENS WHAT MAY BE ATTEMPTED, NEVER HOW CAREFULLY — every pre-wire
// refusal of the write ladder still fires ahead of it, and consent never
// touches spec.FieldErase.
//
// The READ labels are Unverified rather than Supported for the same reason
// (§2.1, "the honest one"): this driver's read path is exercised against a
// scripted peer and a book, and no TS-890S has ever answered a frame.
//
// NO FIELD IS spec.Inert ON THIS ROW. Inert is the FT-710's HARDWARE finding
// of 13/07/2026; this radio has been asked nothing, so there is no finding to
// record, and borrowing one would answer a question about this radio with
// another radio's evidence.
func CapabilitiesUnverified() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified})
}

// CapabilitiesSimulated is this row's internal/fakets890-backed profile (CLI
// --fake, GUI demo) and NEVER a real radio: Read AND Write Supported for
// exactly the eight fields the MA0 record expresses.
//
// Against the fake, hardware risk is moot and the write choreography itself
// is what is being exercised end to end, so claiming Supported here is a
// claim about internal/fakets890 and about nothing else (§2.1, §3.14, A22).
//
// The nineteen fields the record does not express stay the zero FieldSupport,
// simulator or not: the FORM cannot express them, and no amount of
// cooperative fake on the other end of the wire changes what the frame has
// room for.
func CapabilitiesSimulated() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Supported, Write: spec.Supported})
}
