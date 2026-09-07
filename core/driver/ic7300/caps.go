// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300

import (
	"github.com/gm5dna/open-rig-programmer/core/civ"
	ic7300civ "github.com/gm5dna/open-rig-programmer/core/civ/ic7300"
	// ALIASED for the same reason ic7300civ is: the two CI-V profile
	// packages are this package's ONLY named instances from core/civ, one
	// per model, and each appears at exactly ONE call site — the model row
	// below that carries it.
	ic7300mk2civ "github.com/gm5dna/open-rig-programmer/core/civ/ic7300mk2"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// The bank labels this driver publishes. Two banks, and only two: MEM and
// SCAN. There is no CALL bank, no PMS pair and no group addressing on this
// model — all MANUAL-EVIDENCED absences (matrix §1b), and all recorded here
// as banks that do not exist rather than as banks left out.
const (
	bankMemoryLabel = "Memories"
	bankScanLabel   = "Scan edges (P1/P2)"
)

// writeTrialsComplete7300 is the IC-7300's hardware write guard, and it is
// FALSE.
//
// NO IC-7300 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT. Matrix §3.14
// states it for this model alone — "The registered sibling's FALSE is not
// stated here" is the MK2's mirror of the same sentence — so no write trial
// has completed, and the RealHardware profile therefore grades every field
// Unverified, which CanWrite() refuses. Consent
// (spec.ConsentUnverifiedWrites, applied only when the user passed
// WithConsentedUnverifiedWrites) is the ONE key that opens the gate on real
// hardware today, and it opens it for the caller's own explicitly accepted
// risk, never for this table's authority.
//
// Flipping it is a HARDWARE milestone with evidence, ON AN IC-7300. The pin
// in caps_test.go names what a flip must be accompanied by.
const writeTrialsComplete7300 = false

// writeTrialsCompleteMK2 is the IC-7300MK2's, and it is FALSE for the MK2's
// OWN reasons.
//
// TWO CONSTANTS, NOT ONE, and that is the point of them. Matrix §3.14
// states this model's FALSE in its own terms — "The registered sibling's
// FALSE is not stated here" — so the IC-7300's pin lifts nothing for this
// radio and this one lifts nothing for the IC-7300. The evidence is per
// model, and a single shared constant could not express a one-model flip.
// core/driver/ftdx101 keeps writeTrialsCompleteD and writeTrialsCompleteMP
// apart for exactly this reason.
//
// Flipping it is a HARDWARE milestone with evidence, ON AN IC-7300MK2.
const writeTrialsCompleteMK2 = false

// Profile selects which capability description a driver value publishes:
// the fail-safe one a real radio gets, and the one the fake gets. Shared
// with every other driver package (core/driver.Profile); this package
// keeps its own Simulated selector, which
// internal/guards.TestSimulatedProfileTokensConfinement requires.
type Profile = driver.Profile

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// modelParams is everything that differs between the two radios this
// package drives, on core/driver/ftdx101's shape.
//
// IT IS A TABLE OF SEPARATELY-SOURCED VALUES, NOT A SHARED READING, and
// that distinction is the whole reason this struct may exist at all. Both
// matrices' §4 close with the same rule in terms: no assumption in one
// document may be read as covering the other model, and no lift in one
// lifts anything for the sibling. What that rule forbids is a value
// DERIVED from the other radio's manual; what it does not forbid is two
// values, each cited to its own document, standing in adjacent rows. Every
// field below is populated twice, once per model, each from that model's
// own evidence — and the four long refusal strings are carried VERBATIM
// from the two packages this table replaced, lift tokens and page
// references included, precisely so that no sentence acquires the other
// radio's authority on the way past.
//
// It is UNEXPORTED, and so is every function taking one, because the
// exported surface is two thin constructors: New (the IC-7300) and NewMK2.
// An exported model enum's zero value would need its own fail-safe arm, and
// a registration-table closure holding a model value could hold a forged
// one.
type modelParams struct {
	// name is the model's display name and driver-registry key. It is what
	// Model() and Capabilities().Model return.
	name string
	// profile is that model's CI-V profile: the ONE place this package
	// names an instance from core/civ for that radio. Everything
	// wire-shaped derives from it — the framing, the record geometry, the
	// name charset, the address.
	//
	// A VALUE, NOT THE Profile CONSTRUCTOR FUNCTION, and that is
	// load-bearing rather than a style choice. internal/wiring's
	// TestRealDriverFor_DefaultPathByteIdentical compares two driver values
	// with reflect.DeepEqual to prove the consent-false arm carries no
	// smuggled option, and DeepEqual reports two non-nil funcs UNEQUAL even
	// when they are the same function — a func field here would fail that
	// guard for every model in the table. civ.Profile carries no func
	// fields, so the value compares.
	profile civ.Profile
	// catID is the CI-V ADDRESS HEX, not a CAT ID, LOWERCASE to match the
	// runtime form Go's "%02x" verb produces in Identity.CATID (core/clone
	// persists this exact value into a codeplug file's RadioInfo.CATID and
	// downstream comparisons are case-sensitive).
	catID string
	// scanNoBlank is the SCAN bank's spec.Bank.NoBlank.
	scanNoBlank bool
	// tagLen is the name field's width in bytes.
	tagLen int
	// bauds is the CAT serial rate list this model's document names.
	bauds []int
	// minFreqHz and maxFreqHz are spec.Capabilities' two frequency bounds.
	minFreqHz uint64
	maxFreqHz uint64
	// foreignRecordLengths attributes a record length this model does not
	// declare to the sibling that does.
	//
	// ONE ENTRY EACH, and it is a HINT rather than a distinctness claim
	// (plan decision D10). BOTH lengths are ASSUMED derivations from
	// printed field widths — neither document prints a record total —
	// which is why the error text carries the word *provisional* and names
	// both numbers. Cross-model record-length distinctness is a TIER-level
	// check belonging to registration, and it is what may add or correct
	// entries here. DO NOT ADD A SECOND ENTRY to either row from this
	// package.
	foreignRecordLengths map[int]string
	// txFreqSpan and tagSpan are the two record spans the mandatory-field
	// refusals print. They differ in the two documents' own typography
	// (the IC-7300's en dash, the MK2's tilde) and, for the name field, in
	// its end circled numeral — the width difference tagLen carries.
	txFreqSpan string
	tagSpan    string
	// eraseReason, maxFreqReason, selectNibbleReason and scanEdgeReason are
	// the four refusal texts whose wording is this model's own reading of
	// its own document, lift tokens and page references included. They are
	// PINNED BYTE-FOR-BYTE by write_test.go and are carried verbatim.
	// maxFreqReason and scanEdgeReason are fmt format strings; the other
	// two are literal.
	eraseReason        string
	maxFreqReason      string
	selectNibbleReason string
	scanEdgeReason     string
}

// model7300 is the IC-7300, whose every value below comes from the IC-7300
// FULL MANUAL through core/civ/ic7300 and from that model's own capability
// matrix. Nothing here is read from the MK2's document.
var model7300 = modelParams{
	name:    "IC-7300",
	profile: ic7300civ.Profile(),
	// Matrix §3.4, PDF p.126: CI-V Address (Default: 94h).
	catID: "94",
	// FALSE, and it is a decision rather than an oversight. The MK2's
	// document prints "P1 and P2 cannot be cleared" and its SCAN bank is
	// NoBlank as a result; THIS document says nothing of the kind (matrix
	// §1b, register entry `ic7300-scan-edge-noblank`, lift
	// `ic7300-scan-edge-read`), and borrowing the sibling's sentence is
	// exactly the cross-model contamination both matrices' §4 forbid.
	scanNoBlank: false,
	// ⑱–㉗, ten bytes (matrix §3.9, PDF p.169 "Up to 10 characters.").
	tagLen: 10,
	// The [USB] rate list (matrix §1 row 9; this tier connects over USB,
	// PDF p.160 "◇ CI-V connection").
	bauds: []int{4800, 9600, 19200, 38400, 57600, 115200},
	// The COVERAGE floor (PDF p.150). The encoding admits 0 Hz, so
	// 30 000 is the tighter documented bound at this end.
	minFreqHz: 30_000,
	// The STORABLE ceiling (PDF p.167): the record's 10 MHz digit is
	// capped at 6 and the 100 MHz and 1 GHz digits are printed fixed 0.
	// The 74 800 000 figure on PDF p.150 is TUNING COVERAGE, and
	// publishing it would let a codeplug carry a value this encoder must
	// afterwards refuse. In each direction the tighter of the two printed
	// bounds governs (plan decision D6). Matrix Erratum 4(a) marked §1 row
	// 12 PENDING this plan's explicit decision; the decision landed as
	// matrix ERRATUM 10, which discharges that PENDING marker — so 4(a) is
	// closed rather than outstanding.
	maxFreqHz:            69_999_999,
	foreignRecordLengths: map[int]string{45: "IC-7300MK2 (provisional)"},
	txFreqSpan:           "❹–⑧",
	tagSpan:              "⑱–㉗",
	eraseReason:          "this tier ships no erase path: the document prints two clear forms and neither is implemented, and spec.ConsentUnverifiedWrites refuses to consent an erase at any label",
	maxFreqReason:        "%d Hz is above what a memory channel can store on this model (%d Hz): the record's 10 MHz digit is capped at 6, and the 74.8 MHz figure is tuning COVERAGE rather than storable frequency",
	selectNibbleReason:   "the slot is empty and this is a CREATE: record byte ③'s SELECT nibble has no honest source — no spec.Field carries the SELECT group (the tier forbids mapping it as scan_skip), and writing OFF would put the channel into a scan group the caller never chose. Behind it the two tone spans have no documented default either (`ic7300-documented-default-tone-absent`). Write into a slot the radio already holds, or lift `ic7300-select-nibble-on-create`",
	scanEdgeReason:       "record byte ③ is %#02x on a scan edge, and this document prints \"Set both 0 for P1 and P2.\" — the SELECT group is %q, and writing it back would send a value the manual says these two slots must not carry (the value is the radio's own, so it is refused rather than rewritten)",
}

// modelMK2 is the IC-7300MK2, whose every value below comes from that
// radio's OWN 27-page CI-V Reference Guide through core/civ/ic7300mk2 and
// from its own capability matrix. Nothing here is read from the IC-7300's
// document — where the two rows agree, they agree because two documents
// say the same thing, never because one row was copied.
var modelMK2 = modelParams{
	name:    "IC-7300MK2",
	profile: ic7300mk2civ.Profile(),
	// Matrix §3.4: CI-V Address (Default: B6h). The IC-7300 answers at
	// 94h, which is why the two cannot confuse each other in the field.
	catID: "b6",
	// TRUE, and MANUAL-EVIDENCED on this model: P1 and P2 cannot be
	// cleared (PDF p.4, the 0B row "ⓘ P1 and P2 cannot be cleared."; PDF
	// p.17, "* Except for \"01 00\" and \"01 01\" (P1/P2)."). NoBlank is
	// the WHOLE-BANK form and this bank is exactly those two slots, so the
	// fact is stated once and cannot drift out of step with a list of slot
	// strings — which is why RequiredSlots stays empty (D8).
	scanNoBlank: true,
	// ⑱ ~ ㉝, SIXTEEN bytes (matrix §3.9). The IC-7300's field is ten; the
	// two record lengths differ by exactly this six.
	tagLen: 16,
	// THIS DOCUMENT PRINTS NO RATE LIST (matrix §1 #9). The only rates it
	// names anywhere are the three rows of the `18 01` FE-count table — A
	// WAKE-UP-COMMAND TABLE, NOT A SUPPORTED-RATE LIST, and this comment
	// says so in those words because the distinction is the whole content
	// of the derivation. Publishing the three it names is the conservative
	// reading; the IC-7300's six-rate [USB] list is that radio's and is not
	// borrowed. Register entry `ic7300mk2-baud-list`, lift MK2-R21, beside
	// `ic7300mk2-auto-baud-absent` (this document prints no Auto setting at
	// all, where the IC-7300 ships both baud items on it).
	bauds: []int{4800, 9600, 19200},
	// DELIBERATELY ZERO, AND IT IS NOT A FLOOR. This document prints no
	// tuning floor anywhere (matrix §1 #11), and taking the IC-7300's
	// 30 000 Hz would be exactly the cross-model contamination both
	// matrices' §4 forbid. A zero here DISABLES the lower-bound check
	// (core/spec/capabilities.go); it does not assert a known 0 Hz floor,
	// and it is in caps_test.go's deliberatelyZero audit map for that
	// reason. A populated channel at 0 Hz is separately rejected by
	// core/codeplug's own validator, so nothing is admitted that should
	// not be. Register entry `ic7300mk2-min-frequency`, lift MK2-R15
	// (capture `ic7300mk2-tuning-range`).
	minFreqHz: 0,
	// The ENCODING ceiling, and MANUAL-EVIDENCED (matrix §1 #12, PDF
	// p.16): the 10 MHz digit runs `0 ~ 7` and the 1 GHz and 100 MHz
	// digits are printed fixed `0`. Register entry
	// `ic7300mk2-max-frequency`, lift MK2-R15.
	maxFreqHz:            79_999_999,
	foreignRecordLengths: map[int]string{39: "IC-7300 (provisional)"},
	txFreqSpan:           "❹ ~ ⑧",
	tagSpan:              "⑱ ~ ㉝",
	eraseReason:          "this tier ships no erase path: the document prints two clear forms — a truncated 1A 00 set, and command 0B, whose own row says P1 and P2 cannot be cleared — and neither is implemented; spec.ConsentUnverifiedWrites refuses to consent an erase at any label",
	maxFreqReason:        "%d Hz is above what a memory channel can store on this model (%d Hz): the record's 10 MHz digit runs 0 ~ 7 and its 1 GHz and 100 MHz digits are printed fixed 0 (PDF p.16)",
	selectNibbleReason:   "the slot is empty and this is a CREATE: record byte ③'s SELECT nibble has no honest source — no spec.Field carries the SELECT group (§3.16 A10 reads it as group membership, the opposite sense to a skip flag), and writing OFF would put the channel into a scan group the caller never chose. Behind it the two tone spans have no documented default either (`ic7300mk2-documented-default-tone-absent`). Write into a slot the radio already holds, or lift `ic7300mk2-select-nibble-on-create`",
	scanEdgeReason:       "record byte ③ is %#02x on a scan edge, and this document prints \"Set 00 for P1 and P2.\" (PDF p.17) — the SELECT group is %q, and writing it back would send a value the document says these two slots must not carry (the value is the radio's own, so it is refused rather than rewritten)",
}

// memSlots is the MEM bank's canonical slot inventory: "001".."099",
// M-CH01..M-CH99 as the front panel names them (D11).
//
// DENSE, not sparse. This model addresses a memory channel with two packed
// BCD bytes and nothing else, so every slot in the range is addressable and
// the bank lists all of them; spec.Bank.Sparse and its three companions
// stay zero, which spec.Capabilities.Validate enforces as a set.
func memSlots() []string { return spec.NumberedSlots(1, 99, "%03d") }

// scanSlots is the SCAN bank's inventory: "P1" and "P2", which is what the
// manual prints and what codeplug.DisplaySlot's identity fallback passes
// through unchanged (D11). On the wire they are the channel addresses
// 01 00 and 01 01 — channels 100 and 101 in this profile's channel space —
// and read.go's parseSlot is the one place that mapping is written down.
func scanSlots() []string {
	return []string{"P1", "P2"}
}

// bankFields is one bank's field-support map, parameterised by the grade
// the record's OWN fields carry — Unverified on RealHardware, Supported on
// Simulated.
//
// EVERY ONE OF THE TWENTY spec.Fields IS NAMED, including the eleven this
// radio does not have. A Field left out of the map answers the zero
// FieldSupport anyway (spec.Capabilities.FieldSupport), so naming it adds
// no behaviour — it adds the RECORD that somebody looked at the field and
// decided, which is what caps_test.go's allFields pin exists to keep true.
//
// The nine graded rw are exactly the fields the 1A 00 record carries:
//
//   - frequency, tx_frequency — ④–⑧ and ❹–⑧ (the MK2 document's own
//     typography is ④ ~ ⑧ and ❹ ~ ⑧), five packed-BCD bytes each. The
//     transmit frequency is a DISTINCT field, so a split channel round
//     trips.
//   - mode, filter, data_mode, tone_mode — ⑨, ⑩ and ⑪'s two nibbles.
//   - tone_tx, tone_rx — ⑫–⑭ and ⑮–⑰, BCD tenths of a hertz.
//   - tag — ⑱–㉗, ten bytes on the IC-7300; ⑱ ~ ㉝, SIXTEEN on the MK2.
//     The two record lengths differ by exactly that six, and each width is
//     its own model's manual fact (modelParams.tagLen).
//
// The eleven graded ZERO, each for a stated reason:
//
//   - clarifier, ctcss_state, ctcss_tone, shift — the Yaesu vocabulary,
//     which this record does not carry at all (matrix §1 rows 6, 7, 14, 15;
//     spec D4 puts ctcss_state's job on tone_mode for Icom models).
//
//   - duplex, offset — MANUAL-EVIDENCED absences (matrix §1b). Spec D6 puts
//     per-channel duplex and offset OUT OF SCOPE for this pair, and this is
//     the honest empty shape enabler E5b exists to admit: a bank that
//     reaches neither shift nor duplex is not required to name a
//     vocabulary for either.
//
//   - dtcs_code, dtcs_polarity — the record carries no DTCS field
//     (matrix §1b). Vacuous rather than skipped.
//
//   - scan_skip — the tier's hard constraint: ③'s SELECT nibble is group
//     MEMBERSHIP, the inverse of a skip flag, and mapping it as skip is
//     forbidden (plan decision D4). The value is not discarded — it lives
//     inside the civ record on civ.FieldSelect and round-trips there.
//
//   - tag_display — this record carries NO display flag, so a read reports
//     Unavailable and the grading says the same thing rather than
//     declaring a readable field whose every read is Unavailable. The
//     in-tree precedent is core/driver/ftdx10/caps.go:194 and
//     core/driver/ftdx101/caps.go:225 on their own flagless records
//     (plan decision D5, R13).
//
//     IT IS A NARROWING, AND IT IS RECORDED AS ONE. This model's matrix §2
//     row 8 grades the field read-✓ on both banks, as a CHOICE rather than
//     manual evidence; the ZERO FieldSupport here is narrower than that row,
//     so it IS a deviation and is recorded as IC-7300 matrix ERRATUM 13 on
//     the flagless-record precedent above. Plan D5's "no deviation from
//     either matrix remains on this field" was true of the MK2 only, whose
//     §2 row 8 already grades it — — — — and whose rev-1 proposed erratum is
//     recorded WITHDRAWN. The two models agree; only one of them had to move
//     to get there.
//
//   - erase — this tier ships no erase path (spec D4). The document prints
//     TWO clear forms and neither is implemented; doc.go says what a future
//     write-trial milestone would need.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		// In the 1A 00 record.
		spec.FieldFrequency:   rw,
		spec.FieldMode:        rw,
		spec.FieldTag:         rw,
		spec.FieldTxFrequency: rw,
		spec.FieldFilter:      rw,
		spec.FieldDataMode:    rw,
		spec.FieldToneMode:    rw,
		spec.FieldToneTx:      rw,
		spec.FieldToneRx:      rw,

		// Not in the 1A 00 record. The zero FieldSupport, named.
		spec.FieldClarifier:    {},
		spec.FieldCTCSSState:   {},
		spec.FieldCTCSSTone:    {},
		spec.FieldShift:        {},
		spec.FieldTagDisplay:   {},
		spec.FieldScanSkip:     {},
		spec.FieldDuplex:       {},
		spec.FieldOffset:       {},
		spec.FieldDTCSCode:     {},
		spec.FieldDTCSPolarity: {},
		spec.FieldErase:        {},
	}
}

// baseCapabilities builds the whole capability description, with EVERY ONE
// of spec.Capabilities' twenty-eight fields set explicitly: the fields
// that are deliberately zero — named by caps_test.go's deliberatelyZero
// audit, including D8's five receiver vocabularies — are set to their
// zero value in the literal below, beside the reading that says why, and
// caps_test.go's reflection pin refuses a field that is neither non-zero
// nor named in its deliberatelyZero map.
//
// Additions design D4.2 moved the pinned count from twenty-seven to
// twenty-eight by requiring this driver's transmitter anatomy explicitly.
func baseCapabilities(m modelParams, memFields, scanFields map[spec.Field]spec.FieldSupport) spec.Capabilities {
	return spec.Capabilities{
		// Matrix §1 row 1 (§1 #1 in the MK2's matrix).
		Model: m.name,
		// The CI-V ADDRESS HEX, not a CAT ID: CI-V has no ID string, and
		// spec D3.2 fixes the address as this field's content. The 19 00
		// token this driver observes at Open is APPENDED to the session's
		// Identity.CATID ("94:<token>" on the IC-7300, "b6:<token>" on the
		// MK2 — ic7300.go's fmt.Sprintf("%02x:%s", p.RadioAddress(), token))
		// and compared against nothing — the reply value is undocumented on
		// every model in this tier. Each address is its own model's manual
		// fact; see modelParams.catID and the two rows above it. LOWERCASE,
		// matching the runtime form "%02x" always produces, because
		// core/clone persists this exact value into a codeplug file's
		// RadioInfo.CATID and downstream comparisons are case-sensitive.
		CATID:    m.catID,
		Transmit: spec.HasTransmitter,
		Banks: []spec.Bank{
			{
				ID:    spec.BankMemory,
				Label: bankMemoryLabel,
				Slots: memSlots(),
				// FALSE: this document says nothing about whether a memory
				// channel must stay populated (matrix §1b).
				NoBlank: false,
				Fields:  memFields,
			},
			{
				ID:    spec.BankScan,
				Label: bankScanLabel,
				Slots: scanSlots(),
				// PER MODEL, and the two rows above carry each document's
				// own sentence: the MK2's prints "P1 and P2 cannot be
				// cleared" and its SCAN bank is NoBlank as a result; the
				// IC-7300's says nothing of the kind and its bank is not.
				// Neither sentence lifts anything for the other model.
				NoBlank: m.scanNoBlank,
				Fields:  scanFields,
			},
		},
		// The eight mode names ⑨ / ❾ can hold, in wire-value order
		// (matrix §1 row 2; PDF p.167 "① Operating mode"). 0x06 IS ABSENT
		// FROM THE PRINTED COLUMN and is invented nowhere: a record
		// carrying it fails the read with a *civ.ParseError naming the byte
		// and the offset (plan decision D12).
		Modes: []string{"LSB", "USB", "AM", "CW", "RTTY", "FM", "CW-R", "RTTY-R"},
		// The name field's width, per model (matrix §3.9 in both): ⑱–㉗,
		// ten bytes on the IC-7300 (PDF p.169 "Up to 10 characters.");
		// ⑱ ~ ㉝, sixteen on the MK2.
		TagLen: m.tagLen,
		// DELIBERATELY ZERO: there is no clarifier/RIT field in the 1A 00
		// record at all (matrix §1 rows 6 and 7 grade both a poor fit).
		// Graded, not silently omitted.
		ClarMaxHz:  0,
		ClarStepHz: 0,
		// DELIBERATELY EMPTY, and the deviation is recorded. Matrix §1 row 8
		// lists the fifty printed selectable tone frequencies (PDF p.64) —
		// but those are the PANEL-selectable set, and the record stores a
		// BCD FREQUENCY indexing no table, as that row says in terms. A
		// fifty-entry list here would fail closed on every encodable value
		// outside it. The fifty stay in doc.go. The deviation is RECORDED,
		// and it landed as IC-7300 matrix ERRATUM 12 (REV 3's change log
		// called it "proposed erratum 3", which is the number it was asked
		// for under, not the number it was given).
		CTCSSTones: nil,
		// THE TONE DOMAIN, declared as the numeric range E3 added for
		// exactly this shape (D16, ruling T1). The printed per-digit legend
		// (PDF p.171) gives `100 Hz digit: 0 ~ 2` and `10/1/0.1 Hz digits:
		// 0 ~ 9`, i.e. 0..2999 deciHz on a 0.1 Hz grid; intersected with the
		// capability floor of 1 — because 0 Hz is not a tone — the declared
		// domain is {1, 2999, 1}. The civ layer stays lossless over the
		// whole encodable range, zero included, and read.go maps an
		// out-of-domain tone (zero included) to Unknown rather than handing
		// up a Known value codeplug.ToneField.Valid would refuse.
		CTCSSToneRange: &spec.ToneRange{MinDeciHz: 1, MaxDeciHz: 2999, StepDeciHz: 1},
		// The serial rate list, per model. The IC-7300's is its printed
		// [USB] list (matrix §1 row 9; this tier connects over USB, PDF
		// p.160 "◇ CI-V connection"); the MK2's document prints no rate
		// list at all and its three come from its own `18 01` table, as
		// modelParams.bauds records. Neither list is borrowed.
		Bauds: m.bauds,
		// A CHOICE ON BOTH MODELS, AND THE SAME NUMBER FOR DIFFERENT
		// REASONS — which is why it is a shared literal rather than a
		// modelParams row: neither derivation is read from the other
		// document. On the IC-7300 there is NO numeric factory default
		// (both baud items ship set to `Auto`, matrix §3.3,
		// MANUAL-EVIDENCED), so 19200 is the highest rate present in BOTH
		// the [USB] and [REMOTE] lists (4800/9600/19200) — the one rate
		// that still works when `CI-V USB Port` is set to `Link to
		// [REMOTE]` (matrix §3.16 A4); register entry
		// `ic7300-default-open-baud`, lift `ic7300-open-rate`. On the MK2
		// the document prints no factory default either (matrix §1 #10,
		// §3.3), so opening at the highest rate it names anywhere is the
		// derivation; register entry `ic7300mk2-default-baud`, lift
		// MK2-R6.
		DefaultBaud: 19200,
		// The two frequency bounds, per model, each from its own document
		// (modelParams.minFreqHz / maxFreqHz carry the readings). The
		// IC-7300 publishes a 30 kHz COVERAGE floor and a 69 999 999 Hz
		// STORABLE ceiling; the MK2's document prints no floor at all, so
		// its minimum is a deliberate zero rather than a borrowed 30 000,
		// and its ceiling is that record's own encoding limit.
		MinFreqHz: m.minFreqHz,
		MaxFreqHz: m.maxFreqHz,
		// DELIBERATELY EMPTY ON BOTH, for each model's own reason (plan
		// decision D8). The IC-7300 declares nothing never-empty at all
		// (matrix §1 row 13). The MK2 does say P1 and P2 cannot be
		// cleared, and the SCAN bank's NoBlank above states that once; a
		// RequiredSlots list would say the same thing a second time and
		// give it a second place to drift.
		RequiredSlots: nil,
		// DELIBERATELY EMPTY: no shift or duplex field exists on this model
		// (matrix §1 row 14). Enabler E5b is what admits the shape: no bank
		// reaches FieldShift or FieldDuplex, so no vocabulary is demanded.
		// Inventing a dummy one to satisfy a validator would be dishonest.
		ShiftOptions: nil,
		// DELIBERATELY EMPTY: displaced by ToneModes on Icom models
		// (spec D4; matrix §1 row 15).
		CTCSSStates: nil,
		// DELIBERATELY EMPTY: MANUAL-EVIDENCED absence (matrix §1b, duplex).
		DuplexOptions: nil,
		// ⑪'s LOW nibble: "0: OFF, 1: TONE, 2: TSQL" (the IC-7300's PDF
		// p.169 ⑪ detail box; the MK2's own B leg reads the same nibble
		// assignment directly from its arrow labels, DATA left, TONE
		// right). Three values, three distinct semantics, so no entry
		// needs spec.ToneMode.Canonical.
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS},
			{Value: "TSQL", Semantics: spec.ToneModeCTCSSSquelch},
		},
		// DELIBERATELY EMPTY, both: the record carries no DTCS field at all
		// (matrix §1b). A polarity table for a field this radio does not
		// have would be an invention, so the absence is recorded rather
		// than filled.
		DTCSPolarities: nil,
		DTCSCodes:      nil,
		// ⑩ / ❿, the whole byte (the IC-7300's PDF p.167, "② Filter"; the
		// MK2's own ⑩ / ❿ column).
		Filters: []string{"FIL1", "FIL2", "FIL3"},
		// TAKEN FROM THIS MODEL'S PROFILE, never restated. The bytes are
		// the codec's own charset, so the driver's advertised set and the
		// set civ's validName enforces cannot drift apart — and a name this
		// driver advertises as legal is one BuildMemorySet will accept.
		//
		// ON THE MK2, 0x60 IS IN IT AND ITS GLYPH IS NOT ESTABLISHED: that
		// document's PDF p.18 Symbols table draws the same glyph against
		// both 27 and 60 (§3.16 A2). NameCharset is a byte SET, not a glyph
		// map, so both are legal name bytes and 0x60 must never be silently
		// rendered or rewritten as 0x27 (plan decision D13).
		TagCharset: string(m.profile.NameCharset()),
	}
}

// capabilitiesUnverified is the RealHardware profile: every field the
// record carries graded Unverified, which is documented-but-unproven and
// therefore UNWRITABLE (spec.FieldSupport.CanWrite). It is what
// writeTrialsComplete == false means in capability terms, and it is the
// profile a real IC-7300 gets.
func capabilitiesUnverified(m modelParams) spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	return baseCapabilities(m, bankFields(rw), bankFields(rw))
}

// capabilitiesSimulated is the profile a fake radio gets: the same fields
// graded Supported, so the write choreography is exercisable end to end
// without a consent flag and without any claim about hardware.
func capabilitiesSimulated(m modelParams) spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	return baseCapabilities(m, bankFields(rw), bankFields(rw))
}

// ic7300Driver is this package's driver.Driver implementation, for BOTH
// radios: the model it is driving is the modelParams row it carries.
//
// ic7300.go gives it Open, the probe and the session; this file gives it
// the two things a driver must be able to answer before any port exists —
// which model it is, and what that model can do.
type ic7300Driver struct {
	driver.Base
	// m is this driver value's model row, fixed at construction by New or
	// NewMK2 and never mutated. Every model-conditional value the driver
	// and its sessions publish or print is read from it.
	m modelParams
}

// newDriver is the one constructor both exported ones call.
func newDriver(m modelParams, p Profile, opts ...Option) driver.Driver {
	d := &ic7300Driver{Base: driver.Base{Profile: p}, m: m}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// New returns a driver value for the IC-7300 under the given profile.
//
// The zero Profile is RealHardware, the fail-safe one, so a caller that
// passes nothing at all gets the description that writes nothing.
//
// IT RETURNS THE NEUTRAL driver.Driver, and everything above this package
// holds the seam rather than this type. The two optional capabilities this
// driver additionally implements — driver.SerialFramingReporter on the
// DRIVER, driver.DiagnosticsReporter on the SESSION — are reached by the
// house's two-result type assertion, never by a concrete type a caller
// would have to import this package to name.
func New(p Profile, opts ...Option) driver.Driver {
	return newDriver(model7300, p, opts...)
}

// NewMK2 returns a driver value for the IC-7300MK2. Same reasoning as New
// in every respect, including the fail-safe profile arm; the MK2's own
// write guard is writeTrialsCompleteMK2, and it is false for the MK2's own
// reasons.
func NewMK2(p Profile, opts ...Option) driver.Driver {
	return newDriver(modelMK2, p, opts...)
}

// Option configures a driver value at construction.
type Option func(*ic7300Driver)

// WithConsentedUnverifiedWrites records that the user has explicitly
// accepted writing fields that no radio of this model has ever confirmed.
// It is the second key to the hardware-write gate (spec.Support's own
// words), and it is applied only to a profile this driver recognises.
//
// PER DRIVER, THEREFORE PER MODEL: an option passed to New reaches the
// IC-7300's sessions and no MK2's, which is the right granularity for a
// consent the user gives about a radio in front of them.
func WithConsentedUnverifiedWrites() Option {
	return func(d *ic7300Driver) { d.Consented = true }
}

// Model is the display name and registry key. It equals
// Capabilities().Model, which driver.Registry.Register enforces.
func (d *ic7300Driver) Model() string { return d.m.name }

// Capabilities returns the STATIC baseline for this driver's profile.
//
// AN UNRECOGNISED PROFILE FALLS BACK TO THE FAIL-SAFE ONE, not to the
// simulated one: a Profile value outside the two declared constants is a
// construction mistake, and the safe answer to a construction mistake is
// the description that writes nothing.
func (d *ic7300Driver) Capabilities() spec.Capabilities {
	switch d.Profile {
	case Simulated:
		return capabilitiesSimulated(d.m)
	case RealHardware:
		return capabilitiesUnverified(d.m)
	default:
		return capabilitiesUnverified(d.m)
	}
}
