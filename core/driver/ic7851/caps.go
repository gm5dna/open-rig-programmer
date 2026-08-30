// SPDX-License-Identifier: GPL-3.0-or-later

package ic7851

import (
	"fmt"

	civic7851 "github.com/gm5dna/open-rig-programmer/core/civ/ic7851"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// writeTrialsComplete7851 and writeTrialsComplete7850 are this family's
// per-model hardware write guards, and both are FALSE.
//
// FALSE BECAUSE NO IC-7851 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT
// (matrix §0, §3.14). There is no docs/hardware-notes.md section for this
// model, no write-trial protocol run, and no captured frame from one: no
// write trial has occurred, so no write result has been observed, so the
// flag cannot be true. Every byte in this package's tables came from the
// IC-7850/IC-7851 Instruction Manual (Revision 3, 283 pages) through the
// reviewed capability matrix, and from nothing else. There is no
// standalone CI-V reference guide for this model; see doc.go §0.
//
// FLIPPING IT IS A TWO-PART CHANGE, and stating that here is the point of
// either constant. It is never enough to edit this line: the flip requires
//
//  1. this constant, AND
//  2. a capabilitiesRealHardware profile built field class by field class
//     from an IC-7851's OWN trial evidence — not from this model's manual,
//     and never from another model's capture, AND
//  3. the Capabilities switch in ic7851.go rewritten to select it,
//
// with TestWriteTrialsComplete_PinnedFalse's expectation rewritten so the
// flip lands in a diff a reviewer can see. That test also asserts that no
// field is writable on either row while the guards are false, which is
// what actually holds the fail-safe down.
//
// The two constants are deliberately separate even though the models share
// one manual and one profile: evidence for one model is never evidence for
// its sibling (matrix §4).
const writeTrialsComplete7851 = false
const writeTrialsComplete7850 = false

// Profile selects the static capability arm used by the driver.
//
// The zero value is RealHardware ON PURPOSE, mirroring every Yaesu driver
// in this tree: a forgotten or zero-valued Profile must fail TOWARDS the
// real-hardware capability set — which while both write-trial guards are false
// is the all-Unverified one, nothing writable — and NEVER towards the
// simulator's, whose Supported writes are a claim about internal/fakeic7851
// and about nothing else. Any other unrecognised Profile value fails the
// same way, through Capabilities' explicit default arm.
//
// No model dimension: this family has one member.
type Profile int

const (
	// RealHardware is the profile for sessions against a physical radio.
	// While the model's write-trial guard is false it selects the all-Unverified
	// capability set: reads labelled Unverified, every mapped field's
	// Write Unverified, nothing writable without recorded consent.
	RealHardware Profile = iota
	// Simulated is the profile for internal/fakeic7851-backed sessions
	// ONLY (the CLI's --fake mode, the GUI's demo mode): Read and Write
	// Supported for exactly the seven fields the 1A 00 record maps, so the
	// write choreography can be exercised end to end with no hardware at
	// risk.
	Simulated
)

// The numeric bounds this driver DECLARES and, at WriteChannel's rung 4,
// ENFORCES. Stated once here so the capability domain and the pre-build
// refusal cannot drift apart, and read off the reviewed matrix rather than
// off the width of a BCD field.
//
// WHY THE RADIO'S NUMBERS AND NOT THE FIELD'S. The record's frequency
// digits could express 69 999 999 Hz and its tone digits 2999 deci-Hz, and
// an earlier draft declared exactly those. But spec.Capabilities is what a
// consented write is judged against, so declaring the FIELD's width
// authorises a frequency this radio cannot receive and a tone that is not
// on its chart. Matrix §1 row 13 makes the narrower, radio-true ceiling
// the CHOICE for that reason, and rows 9 and 12 supply the other three
// numbers.
//
// THEY ARE DEFENCE IN DEPTH AND NOT THE GATE. civ.FieldSpan carries no
// numeric domain and civ's validateSpanValue checks only BCD width and
// scale, so Profile.AllowedCommand — the last defence before a radio sees
// bytes — would admit a 1A 00 set carrying 65 MHz. Closing that at the
// gate is a shared core/civ change deferred to a post-Wave-3 enabler
// follow-up; see doc.go, and see caps_test.go's
// TestPreBuildRefusalEnforcesTheCapabilityBounds, which is what proves the
// driver's own door is shut.
const (
	// MinRadioFreqHz is the receiver coverage floor, MANUAL-EVIDENCED:
	// matrix §1 row 12, PDF p.267 (folio 19-2), "• Frequency coverage
	// (unit: MHz): Receiver 0.030000–60.000000". That a MEMORY may be
	// stored anywhere in that coverage is the row's ASSUMED half, register
	// entry ic7851-memory-freq-bounds.
	MinRadioFreqHz = 30_000
	// MaxRadioFreqHz is the same row's ceiling. Matrix §1 row 13 records
	// that the FIELD reaches 69 999 999 Hz — the 10 MHz digit is labelled
	// 0–6 over a fixed "0 : 0" fifth cell — and that declaring the
	// narrower radio figure is the CHOICE. Same register entry.
	MaxRadioFreqHz = 60_000_000
	// MinToneDeciHz and MaxToneDeciHz are the CTCSS tone span's own BCD
	// CAPACITY, in tenths of a hertz: matrix §1 row 9 / PDF p.262 (folio
	// 18-13), whose rotated leaders print 100 Hz digit: 0–2 and 10/1/0.1 Hz
	// digits: 0–9, i.e. 000.0–299.9 Hz. The floor is the capability floor
	// of 1 rather than the printed 0, because 0 Hz is not a tone and
	// spec.ToneRange requires MinDeciHz > 0.
	//
	// NOT THE PRINTED CHART, and the difference is the tier's recorded
	// doctrine. PDF p.115 (folio 5-38), "• Selectable tone frequencies
	// (unit: Hz)", prints a 50-tone table running 67.0 to 254.1 — the
	// PANEL-selectable set. The record indexes no table: it stores a BCD
	// frequency, so a fifty-entry domain would fail closed on every
	// encodable value outside it. The IC-7300 settled exactly this
	// (core/driver/ic7300/caps.go:242-251), and it landed as IC-7300 matrix
	// ERRATUM 12. The chart stays in doc.go §6c, as prose about the panel.
	//
	// Pinned by TestCapabilityValuesArePinnedToTheMatrix.
	MinToneDeciHz = 1
	MaxToneDeciHz = 2999
	// ToneStepDeciHz is the wire's own resolution: the "0.1 Hz digit: 0–9"
	// leader gives every tenth of a hertz a distinct encoding. Register
	// entry ic7851-tone-step-domain still asks whether the radio ACCEPTS an
	// off-chart value; it no longer decides this declaration, because the
	// declaration is now what the RECORD can carry rather than what the
	// panel offers.
	ToneStepDeciHz = 1
)

// deliberatelyZero is adjudication R11's audit table: every
// spec.Capabilities field this model leaves at its zero value, with the
// reason. TestDeliberatelyZeroAudit reflects over the struct and requires
// each field to be either populated (from the matrix, cited in the comment
// beside it in baseCapabilities) or listed here.
//
// A zero is never a neutral omission in this struct — a zero MaxFreqHz
// reads as "no ceiling" to every validator — so the audit exists to make a
// new spec.Capabilities field arriving in a shared enabler a decision this
// driver has to take, not a default it silently inherits.
var deliberatelyZero = map[string]string{
	"ClarMaxHz":              "the 1A 00 record has no clarifier field, so there is no offset bound to state (matrix §2, clarifier row)",
	"ClarStepHz":             "the same: no clarifier field, no step",
	"CTCSSTones":             "this radio's tone spans are BCD FREQUENCIES, not indices into a chart. There is no tone number to index, and spec.Capabilities.Validate refuses a model declaring both a chart and a range. CTCSSToneRange is the declaration (tier ruling T1(2), enabler E3)",
	"RequiredSlots":          "RequiredSlots names INDIVIDUAL slots that must stay populated, and this radio has no such slot: its 99 regular channels are all clearable and its two scan edges are all non-clearable. The scan edges' rule is a WHOLE-BANK one and is carried by SCAN's NoBlank instead (matrix §1b.3, which sets the two mechanisms against each other explicitly)",
	"ShiftOptions":           "the Yaesu repeater-shift vocabulary. The 1A 00 record has no shift field at all, and FieldShift carries the zero FieldSupport on both banks, so enabler E5b's anyBankReaches guard makes the empty list lawful",
	"CTCSSStates":            "the Yaesu tone-state vocabulary. This radio expresses tone through ToneModes, the Icom one; a model declares one or the other",
	"TuningSteps":            "additions design D8 — the IC-7851 record carries no receiver tuning-step field",
	"ProgramTuningStepRange": "additions design D8 — the IC-7851 record carries no programmable tuning-step field",
	"AttenuatorDB":           "additions design D8 — the IC-7851 record carries no attenuator field",
	"PreampOptions":          "additions design D8 — the IC-7851 record carries no preamp field",
	"AntennaOptions":         "additions design D8 — the IC-7851 record carries no antenna field",
	"DuplexOptions":          "the Icom repeater vocabulary. The 1A 00 record has no duplex field, FieldDuplex carries the zero FieldSupport on both banks, and E5b's guard makes the empty list lawful",
	"DTCSPolarities":         "the strings \"DTCS\" and \"DCS\" occur ZERO times in all 283 pages — swept for the matrix (§1 row 19). An empty list is the positive statement that this radio expresses no DTCS polarity",
	"DTCSCodes":              "the same sweep: no DTCS code table is printed anywhere in this document, and field ⑪'s tone-type nibble stops at 2: TSQL",
}

// memSlots is the MEM bank's inventory: "001".."099".
//
// Matrix §1b — the two-byte channel selector runs 1..99 for the memories
// (BCD "00 01".."00 99"), and civ.ProfileConfig.ChannelLo/ChannelHi carry
// the same range. Ninety-nine, not a hundred: there is no channel 000.
func memSlots() []string {
	slots := make([]string, 0, 99)
	for i := 1; i <= 99; i++ {
		slots = append(slots, fmt.Sprintf("%03d", i))
	}
	return slots
}

// scanSlots is the SCAN bank's inventory: the two scan edges.
//
// Matrix §3.15(d) — P1 and P2 are NOT a separate bank in the wire
// protocol. They are two more values of the same two-byte selector (BCD
// "01 00" and "01 01", channel numbers 100 and 101, which is exactly what
// civ.ProfileConfig.ChannelHi = 101 encodes). This project models them as
// a SCAN bank only because the neutral memory model needs the distinction
// between a memory and a scan edge; the codec knows one contiguous space.
func scanSlots() []string { return []string{"P1", "P2"} }

// bankFields returns one bank's field map, with rw applied to every field
// the 1A 00 record MAPS and the zero FieldSupport everywhere else.
//
// IDENTICAL FOR MEM AND SCAN, and matrix §3.15(d) is why: both banks read
// and write the SAME 1A 00 record at different values of the same
// selector, so a per-cell difference between them would be a claim this
// document does not make. Whether every field is HONOURED on a scan edge
// is a separate question this document does not answer.
//
// EVERY spec.Field IS LISTED, including the ones this radio does not have.
// An absent key and an explicit zero FieldSupport mean the same thing to
// spec.Capabilities.FieldSupport, but only one of them is a decision a
// reader can check.
//
// Each call returns a fresh map, so no two banks share one.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		// The seven the record maps — core/civ/ic7851's layout() carries a
		// FieldSpan for each and nothing else.
		spec.FieldFrequency: rw, // ④~⑧, little-endian BCD, ⑧ a fixed-zero pad
		spec.FieldMode:      rw, // ⑨, the ten printed mode codes
		spec.FieldFilter:    rw, // ⑩, FIL1/FIL2/FIL3
		spec.FieldToneMode:  rw, // ⑪ low nibble, OFF/TONE/TSQL
		spec.FieldToneTx:    rw, // ⑫~⑭, big-endian BCD, ⑫ a fixed-zero pad
		spec.FieldToneRx:    rw, // ⑮~⑰, big-endian BCD, ⑮ a fixed-zero pad
		spec.FieldTag:       rw, // ⑱~㉗, ten name bytes

		// RULING E6 — THE TWO UNMAPPED NIBBLES. Byte ③'s LOW nibble is a
		// four-valued SELECT-group marker (00: OFF, 01: ★1, 02: ★2,
		// 03: ★3; matrix §3.16.2, register entry
		// ic7851-select-memory-vocabulary) and byte ⑪'s HIGH nibble is a
		// four-valued data mode (0: OFF, 1: DATA 1, 2: DATA 2, 3: DATA 3;
		// matrix §3.15.1). Their neutral homes,
		// codeplug.ChannelData.ScanSkip and .DataMode, are BOTH BoolField.
		//
		// A 4→2 collapse would rewrite a user's SELECT group or data mode
		// on every write-back while readback verification compared equal —
		// silent corruption of the kind this project refuses. So both
		// nibbles are UNMAPPED in the civ layout and both fields carry the
		// zero FieldSupport here, on both banks, under both profiles.
		//
		// THE CONSEQUENCE: a Known ScanSkip or DataMode is REFUSED by the
		// capability gate before any wire traffic — never dropped, never
		// collapsed. TestWriteChannel_LocalRefusalsPrecedeAllWireTraffic
		// exercises both against a session with no engine at all.
		//
		// On Icom, scan_skip is SELECT-GROUP MEMBERSHIP, never a skip.
		spec.FieldScanSkip: {},
		spec.FieldDataMode: {},

		// MANUAL-EVIDENCED ABSENCE: the 1A 00 record has no such span.
		// Each of these is a field the neutral model has room for and this
		// radio's memory record does not express.
		spec.FieldClarifier:   {}, // no clarifier span
		spec.FieldShift:       {}, // no repeater-shift span
		spec.FieldDuplex:      {}, // no duplex span
		spec.FieldOffset:      {}, // no repeater-offset span
		spec.FieldTxFrequency: {}, // ONE frequency span; no split-TX field
		spec.FieldTagDisplay:  {}, // no name-display flag
		// The Yaesu tone pair. This radio expresses tone as tone_mode plus
		// two BCD tone frequencies, so ctcss_state and ctcss_tone have no
		// home in this record.
		spec.FieldCTCSSState: {},
		spec.FieldCTCSSTone:  {},
		// DTCS is printed nowhere in all 283 pages.
		spec.FieldDTCSCode:     {},
		spec.FieldDTCSPolarity: {},

		// ERASE. Unlike Yaesu, the wire form EXISTS on this radio, in two
		// shapes (1A 00 <ch> FF at PDF p.263, and top-level 0B at PDF
		// p.252) — and NO IC-7851 has ever been asked to use either. The
		// zero FieldSupport is what makes core/clone/execute.go's
		// DiffErased branch unreachable for this model, and
		// spec.ConsentUnverifiedWrites structurally never consents this
		// field (its `f != FieldErase` guard), so consent cannot open it
		// either. Matrix §3.13's requirements for a future write-trial
		// milestone are reproduced in doc.go §9.
		spec.FieldErase: {},
	}
}

// baseCapabilities assembles the static baseline both profiles share, with
// the given per-bank field maps.
//
// Every populated field cites the matrix section it came from. Every field
// left at its zero value is listed in deliberatelyZero with its reason,
// and TestDeliberatelyZeroAudit enforces that the two sets partition the
// struct (adjudication R11).
func baseCapabilities(memFields, scanFields map[spec.Field]spec.FieldSupport) spec.Capabilities {
	return spec.Capabilities{
		// §1 row 1 — the model name, overwritten per row by Capabilities;
		// it must equal the registry key Wave 4 will register each row
		// under. PDF p.1 (front cover): "THE TRANSCEIVERS / IC-7850 /
		// IC-7851 / Instruction Manual".
		Model: "IC-7851",
		// §1 row 2 / §3.4 — the CI-V address 0x8E, MANUAL-EVIDENCED: PDF
		// p.229 (folio 15-18), item "CI-V Address (Default: 8Eh)", with
		// the note '"8Eh" is the default address of IC-7850/IC-7851.' —
		// the one place the shared claim is PRINTED rather than inherited.
		//
		// THE STATIC VALUE IS THE ADDRESS ALONE (spec D3.2). A CI-V radio
		// has no CAT-ID equivalent readable before a port opens: the 19 00
		// token is a per-session observation, and this driver RECORDS it
		// and NEVER MATCHES it (register entry ic7851-id-reply-value).
		// Session Identity carries "8e" followed by the observed token;
		// this pre-probe field carries the half that is known statically.
		//
		// THE LOWER-CASE RENDERING IS THIS MODEL'S FIXED CONVENTION, not a
		// per-session choice: the tier's casing is not uniform, and
		// internal/wiring compares a session CATID against this value as
		// an exact-case prefix.
		CATID:    "8e",
		Transmit: spec.HasTransmitter,
		Banks: []spec.Bank{
			{
				ID: spec.BankMemory,
				// A display string, not a protocol fact.
				Label: "Memories",
				Slots: memSlots(),
				// §1 row 3 / §1b.3 — 99 memories, addressed 0001..0099.
				//
				// NoBlank FALSE, and MANUAL-EVIDENCED rather than merely
				// conservative: matrix §1b.3, PDF p.181 (folio 11-2),
				// "■ Memory channels", capability table, the "Regular
				// memory channels / 1–99" row's CLEAR column reads "Yes".
				NoBlank: false,
				Fields:  memFields,
			},
			{
				ID:    spec.BankScan,
				Label: "Scan edges",
				Slots: scanSlots(),
				// §3.15(d) — P1 and P2 are two more values of the same
				// selector, not a protocol-level bank.
				//
				// NoBlank TRUE, and it is the one place the two banks
				// differ: matrix §1b.3 reads the same capability table's
				// "Scan Edge memory channels / P1, P2" row as TRANSFER TO
				// VFO "Yes", OVER-WRITING "Yes", CLEAR **"No"**. A scan
				// edge may be overwritten and may not be cleared, which is
				// a WHOLE-BANK rule and therefore Bank.NoBlank rather than
				// a RequiredSlots list (that list names INDIVIDUAL slots
				// and this radio has no such slot). MANUAL-EVIDENCED.
				NoBlank: true,
				Fields:  scanFields,
			},
		},
		// §1 row 4 — the "①Receiving mode" column's ten printed codes, in
		// the UI's preferred order, which here is the printed one (left
		// sub-column then right): PDF p.260 (folio 18-11), "• Operating
		// mode / Command: 01, 04, 06", the "① Operating mode" table,
		// reprinted on the record page itself at PDF p.263 (folio 18-14)
		// under "Command : 26".
		//
		// THESE ARE THE TEN VALUES OF core/civ/ic7851's mode enum, and
		// TestModes_MatchTheCodec pins the two against each other. They
		// are written out here rather than read from the codec at run time
		// because this list is the UI's vocabulary and its ORDER is a
		// display choice, while the codec's map is the wire reading; the
		// test is what makes the duplication safe.
		//
		// Codes 0x06 and 0x09–0x11 are printed nowhere and are
		// deliberately absent from both lists: a record carrying one fails
		// to decode with a parse error naming the offset. That the ten
		// printed pairs are the COMPLETE set is not stated anywhere, and
		// the honest consequence is the parse failure rather than a
		// guessed eleventh value.
		//
		// THE RADIX OF THE LAST TWO CODES IS A RULING, NOT A READING —
		// RULING OQ1 of 24/08/2026 is HEXADECIMAL, so PSK is the wire byte
		// 0x12 and PSK-R is 0x13. See doc.go.
		Modes: []string{"LSB", "USB", "AM", "CW", "RTTY", "FM", "CW-R", "RTTY-R", "PSK", "PSK-R"},
		// §1 row 21 — PDF p.260 (folio 18-11), the "② Filter setting"
		// column: 01: FIL1, 02: FIL2, 03: FIL3. Reprinted on the record
		// page at PDF p.263 (folio 18-14) under Command 26's "③ Filter"
		// column. The three values of core/civ/ic7851's filter enum,
		// pinned against it by TestFilters_MatchTheCodec.
		//
		// 0x00 IS NOT A MEMBER, and that is a decision with a consequence:
		// a record whose ⑩ is 0x00 fails to decode rather than being read
		// as "no filter". The page prints three values and no default, and
		// the wire domain starts at 01; inventing a fourth would be a
		// radio claim.
		Filters: []string{"FIL1", "FIL2", "FIL3"},
		// §1 row 18 — byte ⑪'s LOW (right) nibble, read off the 400 dpi
		// ×4.0 crop of PDF p.263 (folio 18-14)'s sub-diagram: "0: OFF,
		// 1: TONE, 2: TSQL". The nibble's domain is exactly three values;
		// there is no fourth code printed. The semantics mapping is the
		// neutral model's, and a CHOICE: TONE transmits a CTCSS tone;
		// TSQL transmits one AND squelches on the received one.
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS},
			{Value: "TSQL", Semantics: spec.ToneModeCTCSSSquelch},
		},
		// §1 row 9 — the TONE SPAN'S OWN BCD CAPACITY, and tier ruling
		// T1(2). See MinToneDeciHz/MaxToneDeciHz for the reading, and for
		// why the wire's digit domain is declared rather than the printed
		// 50-tone chart: the record stores a BCD frequency indexing no
		// table, which is IC-7300 matrix erratum 12's recorded doctrine
		// (core/driver/ic7300/caps.go:242-251). Its clone-family sibling
		// the IC-7760 already declares the same {1, 2999, 1}, so a
		// 254.2–299.9 Hz tone now round-trips on both rather than on one.
		//
		// THE WIRE'S 0 IS NOT LOST BY THAT. A tone-OFF channel's bytes are
		// 00 00 00 and the codec hands the number 0 up unharmed; it is
		// handled one layer up, at ReadChannel, which maps an
		// out-of-domain tone number — 0 included — to UNKNOWN rather than
		// to a Known value codeplug.Validate would then refuse (T1(3)).
		// The write path preserves such a number VERBATIM, so a channel
		// this driver reads as Unknown is still writable.
		//
		// RECORDED COST: the tone PICKER stays list-driven, so on this
		// model the grid shows and round-trips tones but the picker cannot
		// offer them. A Wave-4 item and an honesty row.
		CTCSSToneRange: &spec.ToneRange{MinDeciHz: MinToneDeciHz, MaxDeciHz: MaxToneDeciHz, StepDeciHz: ToneStepDeciHz},
		// §1 row 10 — the six rates PDF p.229 (folio 15-18) names for the
		// [USB B] CI-V link, MANUAL-EVIDENCED. spec.Capabilities has ONE
		// list and this radio has two: [REMOTE] prints only 4800, 9600 and
		// 19200. Declaring the USB superset is the CHOICE; register entry
		// ic7851-baud-list-per-port, and doc.go §5 states the cost.
		Bauds: []int{4800, 9600, 19200, 38400, 57600, 115200},
		// §3.3 / §1 row 11 — ASSUMED, register entry
		// ic7851-auto-baud-open. THE DOCUMENT MARKS NO NUMERIC DEFAULT:
		// both CI-V paths print "(Default: Auto)". The choice of 19200
		// within the printed six is ARBITRARY and is recorded as such: no
		// reading of this document favours one of them. What makes it safe
		// is not the choice but the grading and the failure mode — the
		// probe requires an address-matched 19 00 reply, and silence is
		// silence, so a wrong guess costs a clean timeout at Open and
		// never a wrong byte. The driver cannot sweep: internal/wiring
		// opens the port from this value, and Wave 3 may never touch it.
		DefaultBaud: 19200,
		// §1 rows 12 and 13 — the receiver's printed coverage, both ends.
		// See MinRadioFreqHz and MaxRadioFreqHz for the reading and for
		// why the RADIO's ceiling is declared rather than the FIELD's.
		//
		// NEITHER IS IN deliberatelyZero AND NEITHER MAY BE: that table is
		// the zero-value audit's arm for a field left at its zero, and both
		// of these are POPULATED. TestDeliberatelyZeroAudit enforces the
		// partition in both directions, so a populated field appearing in
		// the table fails its "the table has gone stale" arm.
		MinFreqHz: MinRadioFreqHz,
		MaxFreqHz: MaxRadioFreqHz,
		// §1 row 5 — the name span ⑱~㉗ is ten bytes wide: PDF p.263
		// (folio 18-14), "Memory name settings / Up to 10 characters.",
		// corroborated at PDF p.185 (folio 11-6).
		TagLen: 10,
		// §1 row 22 — the 95 printable ASCII bytes, transcribed ONCE in
		// core/civ/ic7851 and referenced here. PDF p.261 (folio 18-12)
		// prints a code for every letter and symbol and NONE for the
		// digits or the space, while its per-command table's 1A 00 row
		// reads "Memory name / All characters are usable." and PDF p.185
		// (folio 11-6) supplies the repertoire in words. The missing code
		// VALUES are ASSUMED: register entry
		// ic7851-name-digit-space-codes.
		//
		// IT MUST BE SUPPLIED EXPLICITLY. spec.Capabilities.TagByteOK's
		// empty-string default excludes ';' as the NEWCAT frame
		// terminator, but ';' is 3B in this radio's own printed symbol
		// table and CI-V has no such terminator, so the default rule would
		// wrongly reject a legal IC-7851 name character.
		TagCharset: civic7851.NameCharset,
	}
}

// capabilitiesUnverified is the REAL-HARDWARE profile while
// both write-trial guards are false: every mapped field Read Unverified and
// Write Unverified, so NOTHING is writable without recorded consent.
//
// The READ labels are Unverified rather than Supported, and that is the
// honest choice: this driver's read path has been exercised against a
// manual and (from Task 14) a fake, and NO IC-7851 HAS EVER ANSWERED A
// FRAME.
//
// Unexported, like core/driver/ftdx101's pair: every consumer outside this
// package reaches these values through New and driver.Driver.Capabilities,
// which is the route internal/wiring's StaticCapabilities already takes.
func capabilitiesUnverified() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	return baseCapabilities(bankFields(rw), bankFields(rw))
}

// capabilitiesSimulated is the internal/fakeic7851-backed profile (CLI
// --fake, GUI demo) and NEVER a real radio: Read AND Write Supported for
// exactly the seven fields the 1A 00 record maps, on MEM and SCAN alike.
//
// Against the fake, hardware risk is moot and the write choreography
// itself is what is being exercised end to end, so claiming Supported here
// is a claim about internal/fakeic7851 and about nothing else.
//
// EVERY WRITTEN-DOWN ZERO STAYS ZERO, simulator or not — INCLUDING
// scan_skip and data_mode. E6's unmapped regions are unmapped against the
// fake too: no amount of cooperative fake on the other end of the wire
// changes what the record has room for, or what the neutral model can
// faithfully carry.
func capabilitiesSimulated() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	return baseCapabilities(bankFields(rw), bankFields(rw))
}
