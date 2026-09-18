// SPDX-License-Identifier: GPL-3.0-or-later

package ts990

import (
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// modelName is the registry key, and it is a CHOICE OVER A DOCUMENTARY
// SILENCE rather than over a printed fact (matrix §1.1).
//
// This book's ID legend prints "022" BARE, with no model name beside it
// (990:2612) — unlike the TS-890S's, which prints "024: TS-890S". The whole
// of this document's identification of itself is its cover and its running
// header ("You can connect the TS-990S transceiver to a PC", 990:19), and
// the COMMAND legend does not say. So binding 022 to this string is this
// project's decision, and the alternative would be a registry key naming the
// number. Capabilities().Model must equal the driver registry key
// (core/driver.Driver's contract), so it is minted once, here.
const modelName = "TS-990S"

// catID is the three printed digits this radio answers "ID;" with
// (990:2612; the Read at 990:2614 and the six-byte Answer at 990:2617).
//
// P1 IS THREE DIGITS, NOT FOUR. core/driver.Driver's CATID doc records the
// convention as "four hex digits on Yaesu; the CI-V address … on Icom"
// (core/driver/driver.go:22), and core/spec constrains CATID only to be
// non-empty. Kenwood is a third form, recorded so a later reader does not
// "fix" a three-digit value into four.
const catID = "022"

// memBankLabel is this package's own display label (matrix §1.4.1, marked
// CHOICE there). A DISPLAY LABEL IS NOT A PROTOCOL FACT: it coincides with
// other rows' today because the neutral bank ID means the same thing to a
// user across the app, and nothing forces the two to stay equal.
const memBankLabel = "Memories"

// narrowSuffix is the spelling of a narrow FM mode name, and it is a CHOICE
// this matrix makes rather than a name either book prints (§1.5): neither
// book names a width-byte combination at all. Pair 1 established "-N" on the
// 590 rows and this package inherits the spelling for consistency across the
// manufacturer.
const narrowSuffix = "-N"

// writeTrialsComplete is THIS package's hardware write guard, and it is
// FALSE: no TS-990S has ever been written to by this project — none has ever
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
// built field class by field class from this radio's own trial evidence, AND
// the Capabilities switch rewritten to select it. Making the constant
// load-bearing on its own would mean a one-character edit could unlock a
// write.
//
// IT IS ONE CONSTANT PER PACKAGE AND THE PACKAGE IS ONE ROW (plan P18). A
// trial on a TS-990S lifts the TS-990S and nothing else, and the sibling
// package's constant is a different constant for that reason.
// TestWriteTrialsComplete_PinnedFalse asserts both halves.
const writeTrialsComplete = false

// Profile selects which capability profile New builds the driver with.
//
// The zero value is RealHardware ON PURPOSE (matrix §2.1): a forgotten or
// zero-valued Profile must fail towards the real-hardware capability set —
// which for this row is the all-Unverified one, nothing writable — and NEVER
// towards the simulator's, whose Supported writes are a claim about
// internal/fakets990 and about nothing else. Any OTHER unrecognised Profile
// value fails the same way, through Capabilities' explicit default arm.
type Profile = driver.Profile

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// layout is this row's MA0 codec layout. One function rather than a bare
// call site so the package has one place that says which of core/kw/ma's two
// rows it is: an ma.Layout890() here would be an MA codec reading this
// radio's frames against the other book's grid, and the two grids share no
// field at a common offset (plan P24).
func layout() ma.Layout { return ma.Layout990() }

// ctcssTones is the 51-entry TN chart, index = the CAT tone number,
// transcribed IN THIS PACKAGE from this book's own printed table
// (990:4949-4974; body 990:4960-4972), in decihertz. Index 50 is 1750 Hz
// (990:4971); index 99 is "Default" and "a setting command only"
// (990:4972, 990:4974), so it is not a tone and is not published.
//
// IT IS NOT THE PROJECT'S SHARED CHART AND IT IS NOT PAIR 1'S (plan P6,
// matrix §1.9). This chart's first fifty entries do COINCIDE with
// core/spec/tones.go's shared Yaesu table, and that coincidence is exactly
// why the transcription is per package: a chart that agrees today is not the
// same fact as a chart read from this radio's book, and pair 1's Kenwood
// chart — 43 entries, eight interstitial tones shorter — is the proof that
// the domain is per row rather than per manufacturer.
// TestCTCSSTones_AreNeitherSharedChart is the inequality pin and
// TestNoProductionFileNamesTheSharedToneChart holds the production half down.
//
// THE 51ST ENTRY IS AN OVER-CLAIM ON THE RECEIVE SIDE, and that is matrix
// M-E1 recurring rather than an oversight. spec.Capabilities carries ONE tone
// domain and ONE predicate, AdmitsTone, which codeplug.ToneField.Valid
// applies to ToneTx and ToneRx alike, so a 51-entry list admits 1750 Hz as a
// tone_rx value the CN chart (00-49, 990:1251-1264) does not print. The
// resolution is to publish 51 and refuse a Known tone_rx of 1750 Hz IN THE
// WRITE PATH — a runtime refusal, not a claim in the capability table. Such a
// value cannot come off a radio in the first place: the codec bounds P8 at
// CN's own 49.
//
// spec.Validate requires CTCSSTones to be strictly ascending precisely so the
// slice index doubles as the CAT tone number; 254.1 → 1750.0 is ascending.
var ctcssTones = []spec.Tone{
	670, 693, 719, 744, 770, 797, 825, 854, 885, 915, 948, 974, 1000,
	1035, 1072, 1109, 1148, 1188, 1230, 1273, 1318, 1365, 1413, 1462, 1514, 1567,
	1598, 1622, 1655, 1679, 1713, 1738, 1773, 1799, 1835, 1862, 1899, 1928, 1966,
	1995, 2035, 2065, 2107, 2181, 2257, 2291, 2336, 2418, 2503, 2541, 17500,
}

// modeDisplayName is the ONE place this package turns a mode byte and an FM
// width byte into a published name, consulted by the capability list and by
// the read path alike so the two cannot drift.
//
// THE LEGEND IS THE LAYOUT'S AND THE SUFFIX IS THIS PACKAGE'S. MA0 carries no
// mode legend of its own — P4 says "refer to the P2 value of the OM command"
// (990:2907-2910), and P10 prints "refer to the P1 value of the OM command"
// for frequency 2 (990:2930) — the book's own erratum E9, since the legend it
// means is OM P2's, not P1's — so the vocabulary is OM's, transcribed once in
// core/kw/ma (990:3706-3730). What the layout does
// NOT carry is the width byte: P5 is "0: FM Wide for frequency 1 / 1: FM
// Narrow for frequency 1" (990:2912-2914), a flag ORTHOGONAL to the mode byte
// whose legend scopes it to no particular mode value, and pair 1's
// kw.RecordModeName has no MA-family counterpart. So the narrow twin is
// synthesised here.
//
// THE NARROW TWIN IS OFFERED FOR EVERY FM-NAMED MODE, WHICH IS FOUR OF THEM
// ON THIS ROW — 4 FM, E FM-D1, I FM-D2, M FM-D3 (matrix §1.5) — and the test
// is the layout's own published NAME rather than a list of bytes, so a legend
// correction in core/kw/ma cannot leave a stale byte list behind here. The
// trade is stated in §1.5 and is a CHOICE with a published cost: if the radio
// ignores the width byte on a data-FM mode, this programme publishes two names
// for one radio state, which the read side reports honestly. Scoping the byte
// to plain FM instead would mean emitting it with no source on every data-FM
// write — a defaulted byte, and this milestone has exactly one of those (A14).
func modeDisplayName(l ma.Layout, mode byte, narrow bool) (string, bool) {
	name, ok := l.ModeName(mode)
	if !ok {
		return "", false
	}
	if narrow && strings.HasPrefix(name, "FM") {
		return name + narrowSuffix, true
	}
	return name, true
}

// modeNames returns the selectable mode display names this row advertises,
// DERIVED FROM THE LAYOUT rather than transcribed here — twenty-six, being
// the twenty-two live legend values plus the four narrow twins (matrix §1.5).
//
// Wire-byte order comes free, and on this row that order is not a hex
// nibble's: the legend runs '0'-'9' then 'A'-'N' (990:3706-3730), so ascending
// byte order puts LSB-D1 at 'C' after PSK-R at 'B' and AM-D3 at 'N' last.
// Values '0' and '8' are printed "Unused" (990:3707, 990:3715), are absent
// from the layout's legend, and so are absent from this list.
func modeNames(l ma.Layout) []string {
	var names []string
	for b := 0; b <= 0xFF; b++ {
		wide, ok := modeDisplayName(l, byte(b), false)
		if !ok {
			continue
		}
		names = append(names, wide)
		if narrow, _ := modeDisplayName(l, byte(b), true); narrow != wide {
			names = append(names, narrow)
		}
	}
	return names
}

// memSlots returns this row's MEM inventory, "000".."099", built through the
// LAYOUT's own Slots() and NewSlot so the wire forms this capability data
// advertises are the ones the codec's slot space actually accepts.
//
// THE CLASS FILTER IS WHAT OMITS 100-119 (plan P11, matrix §1.4.2, §1.4.3;
// decisions.md row 14). The layout DECLARES those twenty slots, because both
// books print them and a frame naming one must parse rather than be refused
// as malformed. What such a channel holds is never explained: what an MA0
// read of a section-defined channel answers is A9, what its "frequency 2"
// means is A8, and the only thing this book says about an E channel anywhere
// is the number-mapping sentence itself (990:2896). So the twenty slots are
// published NOWHERE — not appended to MEM, not a bank of their own, and not a
// spec.BankScan, which would be twenty slot identities invented out of a
// command this programme never sends (§1.4.5). Selecting on kw.SlotMemory
// rather than on a number range keeps the codec's domain and the driver's
// inventory a single edit apart from each other.
//
// Walking l.Slots() rather than probing every number 0-999 costs nothing:
// the layout already publishes its own ranges, so there is no ceiling to
// derive or defend.
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
// deliberately zeroed (Capabilities.FieldSupport returns the zero value for an
// absent key), and only a written-down zero is legible as a decision (matrix
// §2.1). core/driver/ts590/caps.go's bankFields is the SHAPE precedent, and
// the VALUES are this book's.
//
// NOTHING HERE VARIES. This row has exactly one bank (§2.2), so every graded
// field is graded once, and the eight graded fields are the eight the
// eighteen-parameter grid expresses.
//
// Each call returns a fresh map, so no two capability values share one.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		// The eight the 57-byte record expresses.
		spec.FieldFrequency: rw, // P3, 11 digits at bytes 8-18 (990:2905-2906)
		spec.FieldMode:      rw, // P4 at byte 19, legend via OM P2 (990:2907-2910)
		spec.FieldTag:       rw, // P18, 10 bytes at 47-56 (990:2955-2956)
		spec.FieldToneMode:  rw, // P6 at byte 21, four values (990:2915-2919)
		spec.FieldToneTx:    rw, // P7 at 22-23 via TN P2 (990:2920-2922)
		spec.FieldToneRx:    rw, // P8 at 24-25 via CN P2 (990:2924-2926)

		// P17 at byte 46, AND THE ENCODING IS THIS RADIO'S OWN: "1: Scan
		// Lockout OFF / 2: Scan Lockout ON" (990:2952-2954), where the
		// TS-890S prints 0/1 (890:3205-3207) and where THIS BOOK'S OWN MA3
		// P2 prints "0: Scan Lockout OFF / 1: Scan Lockout ON" (990:3019-3021).
		// That is erratum E8 and the sharpest trap in either book; the
		// published value comes from MA0 P17, because MA0 is the record
		// this programme reads and MA3 is not on the outbound roster
		// (matrix §2.9).
		spec.FieldScanSkip: rw,

		// P9 at bytes 26-36 with the split flag P15 at byte 44
		// (990:2927-2928, 990:2946-2948), and this is the pair's largest
		// gain over pair 1 (§2.4): ONE read returns both sides and the
		// flag, so the disposition is never guessed. It is graded ONCE,
		// in MEM, for 000-099 — M-E7: slots 100-119 are ABSENT from the
		// model rather than codeplug.Unavailable, because no channel is
		// ever produced for one.
		spec.FieldTxFrequency: rw,

		// MANUAL-EVIDENCED absence FROM THE RECORD over a complete
		// eighteen-parameter account (990:2891-2956) — and the caveat is
		// M-E5's: THE RADIO HAS RIT AND XIT, as radio-level settings no
		// memory channel stores (RT 990:4252, XT 990:5210). The zero means
		// "no per-channel clarifier field", never "this radio has no
		// clarifier". ClarMaxHz and ClarStepHz are 0/0 in consequence.
		spec.FieldClarifier: {},

		// The Yaesu half of the vocabulary pair (decision 6,
		// core/spec/field.go:41-44). This record expresses tone as P6's
		// four-value mode selector with two INDEPENDENT indices, which is
		// the tone_mode/tone_tx/tone_rx shape, and repeater operation as
		// an absolute second frequency rather than a shift selector.
		// FieldCTCSSTone is additionally ONE field where this record
		// carries TWO indices — four, counting frequency 2's.
		spec.FieldCTCSSState: {},
		spec.FieldCTCSSTone:  {},
		spec.FieldShift:      {},

		// No tag-display flag anywhere in the grid: the parameter account
		// is complete.
		spec.FieldTagDisplay: {},

		// THE STANDING NO-ERASE RULE, over evidence STRONGER than pair 1's
		// rather than weaker (M-E4, §2.8). Pair 1's only erase route was an
		// unprinted-width MW side effect whose English admitted two
		// readings; this radio prints a dedicated command, MA5 "Channel
		// Deletion" (990:3042-3047). There is no ambiguity to record and
		// the command is simply never built — the outbound gate admits what
		// this programme builds, so no frame beginning MA5 can leave.
		spec.FieldErase: {},

		// A case-insensitive search of this document for "duplex" returns
		// zero hits (§1.18): no duplex command, no duplex legend, no duplex
		// menu row, and no per-channel offset magnitude. Split is expressed
		// ONLY as an absolute second frequency, which is FieldTxFrequency.
		spec.FieldDuplex: {},
		spec.FieldOffset: {},

		// DCS appears NOWHERE in this book — a case-insensitive search for
		// DCS (which subsumes DTCS) returns zero hits (§1.20, §1.21). This
		// is the strongest form of the absence and needs no caveat.
		spec.FieldDTCSCode:     {},
		spec.FieldDTCSPolarity: {},

		// MANUAL-EVIDENCED absence FROM THE RECORD, with the M-E5 caveat:
		// this radio HAS per-mode filter selection, as four radio-level
		// commands — FL0 "Select the Receive Filter" (990:2411), FL1
		// "Roofing Filter" (990:2432), FL2 "IF Filter Shape" (990:2467),
		// FL3 "AF Filter Type" (990:2488). This is where the pair diverges
		// sharply from the TS-590SG, whose byte 28 IS a FILTER A/B
		// selector: a codeplug round trip through this programme does not
		// preserve a channel's filter here, and cannot, because the record
		// does not carry one (§1.22).
		spec.FieldFilter: {},

		// ERRATUM M-E3, and the sharpest single divergence from pair 1,
		// whose 590 rows grade this field from a byte of its own. THERE IS
		// NO DA COMMAND ON THIS RADIO AND NO DATA BYTE IN THIS GRID: the
		// data-ness is carried by the mode NAMES, and this row prints three
		// data families where the TS-890S prints one — D1 at C-F, D2 at
		// G-J, D3 at K-N (990:3719-3730).
		spec.FieldDataMode: {},

		// No step field in the grid, and — unlike the TS-480, whose bytes
		// 39-40 are an ST index and whose whole write path pair 1 had to
		// refuse in consequence — THIS RADIO HAS NO ST COMMAND AT ALL
		// (§1.23). Step sizes exist only as EX menu rows, which are
		// radio-level settings reached through the settings surface. The
		// consequence stated positively: the TS-480's blanket write refusal
		// has no counterpart here, because there is no step to lose.
		spec.FieldTuningStepEnabled: {},
		spec.FieldTuningStep:        {},
		spec.FieldProgramTuningStep: {},

		// MANUAL-EVIDENCED absence FROM THE RECORD, and the caveat matters
		// (M-E5): this radio HAS an attenuator (RA, 990:4094) and a
		// pre-amplifier (PA, 990:3734) as radio-level functions, and it
		// selects antennas by command — AN0 "Antenna Selection" (990:211)
		// and AN1 "Antenna Name" (990:240), the sharper case, because the
		// radio holds antenna NAMES no memory channel references. None of
		// the three has a per-channel position in the grid.
		spec.FieldAttenuator: {},
		spec.FieldPreamp:     {},
		spec.FieldAntenna:    {},

		// An Icom concept with no position in this frame and no mention in
		// this book.
		spec.FieldIPPlus: {},
	}
}

// baseCapabilities assembles this row's static baseline at the given evidence
// grade.
//
// ALL TWENTY-EIGHT spec.Capabilities fields are populated explicitly — the
// non-zero ones and the eighteen deliberately EMPTY ones alike — and
// TestCapabilities_EveryFieldExplicit reflects over the struct to enforce both
// halves. A zero left in one of the populated ones is not a neutral omission:
// a zero MaxFreqHz reads as "no ceiling" to core/codeplug's validator, a zero
// TagLen makes core/csvio's CHIRP import truncate every imported name to "",
// a non-positive Bauds entry reaches SerialConfig.Baud, and an empty
// vocabulary where a bank reaches its field fails spec.Validate outright.
//
// THE EMPTY ONES ARE THE POSITIVE STATEMENT "this radio expresses no such
// vocabulary" (matrix §1.16-§1.28), which is what every capability-keyed check
// in core/codeplug and core/csvio tests before it runs; populating any of them
// would be the mistake. They are written out below with their reasons rather
// than left off, because an omission and a decision would otherwise read the
// same.
func baseCapabilities(rw spec.FieldSupport) spec.Capabilities {
	l := layout()
	return spec.Capabilities{
		Model: modelName,
		CATID: catID,
		// MANUAL-EVIDENCED (§1.3). This book prints no TX block — that
		// citation is the TS-890S's — so the transmit surface is evidenced
		// by TN, the transmit-tone command whose whole purpose is what the
		// radio sends (990:4949-4954), and by each MA0 record's own
		// frequency-2 side (990:2927-2948). spec.Validate refuses the zero
		// value, so this is a declaration this row must make.
		Transmit: spec.HasTransmitter,
		// MANUAL-EVIDENCED from THIS book: MA0's frequency-2 side and P15
		// "0: Simplex / 1: Split" (990:2946-2948), with the printed answer
		// "…all parameters for frequency 2 become 0" (990:2964-2965), which
		// is why write.go derives split as TxFreqHz.Value != 0. Read only by
		// core/csvio's CHIRP importer, on the blank Duplex arm; pinned by
		// TestCapabilities_SimplexTx.
		SimplexTx: spec.SimplexTxZero,
		Banks: []spec.Bank{{
			ID:    spec.BankMemory,
			Label: memBankLabel,
			Slots: memSlots(l),
			// NoBlank FALSE, STATED (§2.5). NoBlank means "every slot in
			// this bank must be populated" — the Yaesu PMS invariant — and
			// nothing in this book imposes it; the book says the opposite,
			// that a blank channel is a normal state with a defined answer
			// (990:2962-2963), and it prints MA5, whose existence
			// presupposes that a channel may be empty (990:3042-3047). A
			// NoBlank MEM bank would make codeplug.Validate refuse every
			// candidate with a single blank channel, which on a
			// hundred-slot space is every real codeplug.
			NoBlank: false,
			Fields:  bankFields(rw),
			// Sparse false, and the five sparse-space fields with it: this
			// is a small dense space fully printed in the book
			// (990:2893-2896), and spec.Validate requires those fields to
			// be zero when Sparse is false.
			Sparse: false,
		}},
		Modes: modeNames(l),
		// MANUAL-EVIDENCED WIDTH, ASSUMED PAD BYTE (§1.6, A1). P18 is
		// "Channel Name (Up to 10 digits.)" (990:2955-2956) drawn over a
		// FIXED ten-byte window at bytes 47-56 with the terminator nailed
		// to 57 — "up to" and a fixed window cannot both be literal, and
		// the book states no pad byte (erratum E13). The codec assumes
		// ASCII space on write and right-trim on read; the width itself is
		// printed. Ten, not pair 1's eight, and it is per row.
		TagLen: 10,
		// 0/0, and the reason is the RECORD ACCOUNT rather than decision
		// 6's vocabulary rule — M-E4, which records that `clarifier` has no
		// row in the design's own table. The eighteen parameters account
		// for every byte of the grid (990:2891-2956) and none of them is an
		// RIT/XIT offset. THIS RADIO DOES HAVE RIT AND XIT (990:4252,
		// 990:5210), as radio-level settings no memory channel stores.
		// spec.Validate places no constraint on a zero ClarMaxHz, and
		// FieldClarifier is Unsupported on the one bank, so nothing
		// consults these.
		ClarMaxHz:  0,
		ClarStepHz: 0,
		// The 51-entry TN chart; see ctcssTones. Copied per call so no two
		// capability values share the backing array.
		CTCSSTones: append([]spec.Tone(nil), ctcssTones...),
		// nil (§1.10): the tone field is an INDEX into a printed chart, not
		// a number — P7 and P8 refer to TN P2 and CN P2 (990:2920-2926),
		// and P13/P14 the same for frequency 2 (990:2940-2945). A radio
		// declares a list or a range, never both, and Validate refuses both
		// together.
		CTCSSToneRange: nil,
		// FIVE rates, and 4800 is deliberately OMITTED (§1.11). The book
		// prints six: "Selectable from 4800*/ 9600/ 19200/ 38400/ 57600/
		// 115200 bps" (990:13-14). IT IS CONDITIONAL IN TWO INDEPENDENT
		// WAYS ON THIS RADIO, and spec.Capabilities can express neither.
		// The asterisk resolves on the same page to "4800 bps cannot be
		// used with the USB-B connector" (990:23) — and USB-B is the
		// connector this book's own "Using a USB Cable" section steers a PC
		// user to (990:33-40) — while a Bauds list is ONE FLAT LIST PER ROW
		// with no per-path axis and cannot say "except over one of this
		// radio's paths". And the framing table's stop-bit row makes the
		// rate the one place two stop bits are available, "1 (2 is
		// available only when using 4800 bps)" (990:18), where stop bits
		// live in transport.SerialConfig and are chosen once per session
		// independently of the rate — so offering 4800 would offer a rate
		// this programme then opens with framing the book scopes to it.
		// Publishing five claims nothing false; the failure mode publishing
		// six invites is a timeout that looks like a dead port.
		Bauds: []int{9600, 19200, 38400, 57600, 115200},
		// ASSUMED — A11, and an OPERATIONAL ASSUMPTION rather than a
		// conservative choice (§1.12). This book prints no factory value:
		// the hardware table lists the selectable rates and marks no
		// default (990:13-14). A wrong baud is not a safe baud — it is an
		// unreachable radio, and the symptom is a timeout that looks like a
		// dead port. NO AUTO-BAUD: probing by re-opening the port at five
		// speeds is a discovery mechanism this programme does not have and
		// would not test. The lift cannot be a wire observation, because
		// reading it back over EX presupposes a working session at the very
		// baud in question. spec.Validate requires this value to appear in
		// Bauds, and it does.
		DefaultBaud: 9600,
		// 0/0 — CANNOT ESTABLISH, A15 (§1.13, §1.14). THIS PC-COMMAND
		// DOCUMENT PRINTS NO FREQUENCY RANGE: MA0 P3 says only "Frequency 1
		// (11 digits in Hz.)" (990:2905-2906). A FIELD WIDTH IS NOT A
		// TUNING RANGE, and publishing one as the other would put a
		// fabricated capability in the capability table.
		//
		// A ZERO IS A DISABLED CHECK, WHICH IS THE POSITIVE PROPERTY HERE:
		// codeplug.Validate's floor and ceiling both test != 0 first, so a
		// channel at 1 Hz and one at 99 GHz alike pass validation on this
		// row — no false refusal and no false promise — while a frequency
		// needing more than eleven digits is refused by the CODEC, whose
		// refusal names the FIELD WIDTH and not any radio's tuning range.
		// The consequence is a live gap in the shipping programme, and it
		// closes with no code change the day the instruction manual's
		// general-specifications table gives the bounds a source.
		MinFreqHz: 0,
		MaxFreqHz: 0,
		// nil (§1.15): RequiredSlots names individual slots that must never
		// be empty — the FT-710's M-01. This book marks no channel
		// mandatory and says the opposite. Distinct from Bank.NoBlank,
		// which is also false.
		RequiredSlots: nil,
		// Both EMPTY (§1.16, §1.17): the Yaesu half of the vocabulary pair,
		// which core/spec/field.go:41-44 forbids a model expressing
		// alongside the Icom half. Empty is legal here only because no bank
		// grades FieldShift or FieldCTCSSState above Unsupported, which is
		// what spec.Validate's own pair rules are conditional on.
		ShiftOptions: nil,
		// EMPTY (§1.18), and this is the one place this row publishes an
		// INCOMPLETE Icom pair: the record carries no duplex selector and
		// no offset magnitude, and split is expressed only as an absolute
		// second frequency, which is FieldTxFrequency and not FieldDuplex.
		// A row that later opened FieldDuplex without supplying this would
		// fail Validate loudly, which is the correct direction. It is also
		// what makes a CHIRP row with a BLANK Duplex import as simplex
		// under lane L's arm, while "off" still blocks.
		DuplexOptions: nil,
		// FOUR values, MANUAL-EVIDENCED (§1.19): MA0 P6 reads "0: FM Tone
		// function OFF / 1: Tone / 2: CTCSS / 3: Cross Tone" for frequency
		// 1 (990:2915-2919) and P12 the same four for frequency 2
		// (990:2935-2939), corroborated by TO P2 (990:4978-4989).
		//
		// THE SEMANTICS ARE ASSUMED, AND THE REGISTER ENTRY IS K-D1 IN THIS
		// PACKAGE'S OWN doc.go (matrix M-E2). Pair 1's mapping rested on
		// ONE sentence in the 590 book — IF P14's "the transceiver
		// transmits on the Tone frequency and receives on the CTCSS
		// frequency" (590:1167-1171) — and THIS BOOK CONTAINS NEITHER THAT
		// SENTENCE NOR AN IF COMMAND BLOCK AT ALL. So assigning
		// ToneModeCTCSS to value 1 and ToneModeCTCSSRxSquelch to value 2 —
		// and, with them, the tone index to FieldToneTx and the CTCSS index
		// to FieldToneRx — is an assumption made from a different radio's
		// book. Publishing four with the semantics recorded ASSUMED is the
		// honest position: the VALUES round-trip on the evidence of the
		// charts, and only their meaning is borrowed. Publishing OFF alone
		// until K-D1 lifts would refuse to READ every channel carrying a
		// tone.
		//
		// CROSS is declared and IS structurally usable: ToneModeCross
		// requires a model that also expresses the fields the combination
		// needs, and this row does — an independent tone index and an
		// independent CTCSS index side by side in one record.
		//
		// Canonical is false on every entry: no semantic is expressed
		// twice, so spec.Validate's canonical rule requires nothing.
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS},
			{Value: "CTCSS", Semantics: spec.ToneModeCTCSSRxSquelch},
			{Value: "CROSS", Semantics: spec.ToneModeCross},
		},
		// Both EMPTY (§1.20, §1.21): see FieldDTCSCode in bankFields.
		DTCSPolarities: nil,
		DTCSCodes:      nil,
		// EMPTY (§1.22), and NOT per row as it is on the 590 pair: see
		// bankFields' FieldFilter paragraph. FL0-FL3 are radio-level.
		Filters: nil,
		// EMPTY / nil (§1.23, §1.24): no step position in the grid and no
		// ST command anywhere in this book, so there is no step magnitude
		// and no on/off flag to publish.
		TuningSteps:            nil,
		ProgramTuningStepRange: nil,
		// All EMPTY (§1.25-§1.27), with the caveat bankFields states: the
		// radio has an attenuator, a pre-amplifier and named antennas as
		// radio-level functions, and no record position for any of them;
		// nothing here says it has none.
		AttenuatorDB:   nil,
		PreampOptions:  nil,
		AntennaOptions: nil,
		// EMPTY (§1.28), a CHOICE taking the strict direction. The empty
		// string selects the family default — printable ASCII 0x20-0x7E
		// excluding ';' (spec.Capabilities.TagByteOK). P18 carries no
		// charset statement of its own (990:2955-2956); what the book DOES
		// print is a global coding rule whose range above 0x7E depends on a
		// menu setting this programme does not read — "the letters assigned
		// to 80h ~ FFh are replaced … by Menu 9-01 (Keyboard Language)"
		// (990:32-39) — so narrowing to 0x7E is the direction that cannot
		// put a byte of ambiguous meaning on the wire. The ';' exclusion is
		// FORCED rather than assumed: the terminator's position "differs
		// depending on the command used" (990:96-99), so a name containing
		// one would split the frame at the radio's own parser. A2 is the
		// register home, and its upper bound at 0x7E stays open.
		TagCharset: "",
	}
}

// CapabilitiesUnverified is this row's all-Unverified FAIL-SAFE profile, and
// it is what a RealHardware session gets today: every field the 57-byte record
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
// THE ONE ROUTE PAST THAT, and it is the user's own: a session opened with
// WithConsentedUnverifiedWrites re-labels these write-side Unverified fields
// spec.ConsentedUnverified at session-capability assembly, and CanWrite is
// true for that state — so a CONSENTED RealHardware session can attempt a
// write, while this static profile is untouched and every unconsented session
// still cannot. CONSENT WIDENS WHAT MAY BE ATTEMPTED, NEVER HOW CAREFULLY:
// every pre-wire refusal of the write ladder still fires ahead of it.
//
// The READ labels are Unverified rather than Supported for the same reason
// (§2.1, "the honest one"): this driver's read path is exercised against a
// fake and a book, and no TS-990S has ever answered a frame.
//
// NO FIELD IS spec.Inert ON THIS ROW. Inert is the FT-710's HARDWARE finding
// about the FT-710; this radio has been asked nothing, so there is no finding
// to record, and borrowing one would answer a question about one radio with
// another radio's evidence. Worth stating for this row in particular, because
// §2.7's secondary-side problem LOOKS like the problem Inert solves and is
// not: Inert is for a field the protocol transmits on every write whose value
// the radio ignores, and this radio's frequency-2 tuple is a field it very
// much does not ignore.
func CapabilitiesUnverified() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified})
}

// CapabilitiesSimulated is this row's internal/fakets990-backed profile (CLI
// --fake, GUI demo) and NEVER a real radio: Read AND Write Supported for
// exactly the eight fields the 57-byte record expresses.
//
// Against the fake, hardware risk is moot and the write choreography itself is
// what is being exercised end to end, so claiming Supported here is a claim
// about internal/fakets990 and about nothing else (§2.1, §3.14).
//
// The nineteen fields the record does not express stay the zero FieldSupport,
// simulator or not: the FORM cannot express them, and no amount of cooperative
// fake on the other end of the wire changes what the frame has room for.
func CapabilitiesSimulated() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Supported, Write: spec.Supported})
}
