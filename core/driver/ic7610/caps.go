// SPDX-License-Identifier: GPL-3.0-or-later

package ic7610

import (
	"fmt"

	civic7610 "github.com/gm5dna/open-rig-programmer/core/civ/ic7610"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// writeTrialsComplete is this model's hardware write guard, and it is
// FALSE.
//
// FALSE BECAUSE NO IC-7610 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT
// (matrix §0, §3.14). There is no docs/hardware-notes.md section for this
// model, no write-trial protocol run, and no captured frame from one: no
// write trial has occurred, so no write result has been observed, so the
// flag cannot be true. Every byte in this package's tables came from the
// IC-7610 CI-V Reference Guide rev 4 and from nothing else.
//
// FLIPPING IT IS A TWO-PART CHANGE, and stating that here is the point of
// the constant. It is never enough to edit this line: the flip requires
//
//  1. this constant, AND
//  2. a capabilitiesRealHardware profile built field class by field class
//     from an IC-7610's OWN trial evidence — not from this model's manual,
//     and never from another model's capture, AND
//  3. the Capabilities switch in ic7610.go rewritten to select it,
//
// with TestWriteTrialsComplete_PinnedFalse's expectation rewritten so the
// flip lands in a diff a reviewer can see.
//
// THERE IS NO REGISTERED SIBLING (matrix §4), so this is ONE constant and
// ONE register row, not a pair. core/driver/ftdx101 carries two because
// the FTDX101D and the FTDX101MP are two radios sharing one manual and a
// capture from either is never evidence about the other; the IC-7610 has
// no such twin in this tier.
const writeTrialsComplete = false

// Profile selects which capability profile New builds the driver with.
//
// The zero value is RealHardware ON PURPOSE, mirroring every Yaesu driver
// in this tree: a forgotten or zero-valued Profile must fail TOWARDS the
// real-hardware capability set — which while writeTrialsComplete is false
// is the all-Unverified one, nothing writable — and NEVER towards the
// simulator's, whose Supported writes are a claim about internal/fakeic7610
// and about nothing else. Any other unrecognised Profile value fails the
// same way, through Capabilities' explicit default arm.
//
// No model dimension: this family has one member.
type Profile int

const (
	// RealHardware is the profile for sessions against a physical radio.
	// While writeTrialsComplete is false it selects the all-Unverified
	// capability set: reads labelled Unverified, every mapped field's
	// Write Unverified, nothing writable without recorded consent.
	RealHardware Profile = iota
	// Simulated is the profile for internal/fakeic7610-backed sessions
	// ONLY (the CLI's --fake mode, the GUI's demo mode): Read and Write
	// Supported for exactly the seven fields the 1A 00 record maps, so the
	// write choreography can be exercised end to end with no hardware at
	// risk.
	Simulated
)

// The two numeric bounds Task 12's pre-build refusals enforce, stated once
// here so the capability domain and the refusal cannot drift apart.
//
// THEY ARE DEFENCE IN DEPTH AND NOT THE GATE. civ.FieldSpan carries no
// numeric domain and civ's validateSpanValue checks only BCD width and
// scale, so Profile.AllowedCommand — the last defence before a radio sees
// bytes — would admit a 1A 00 set carrying 70 MHz. Closing that at the
// gate is a shared core/civ change the orchestrator DEFERRED on
// 24/08/2026 to a post-Wave-3 enabler follow-up; see doc.go, and see
// write_test.go's TestNumericRefusalIsDefenceInDepthNotTheGate, which
// asserts the gap so that closing it is a visible test change.
const (
	// MaxEncodableFreqHz is the largest frequency the record's five-byte
	// little-endian BCD frequency span can carry. Matrix §1 row 12: the
	// 10 MHz digit is labelled 0–6 and the fifth cell is a printed
	// "0 : 0" marked (Fixed), so 69 999 999 Hz is the ceiling.
	//
	// The STORABLE ceiling — what an IC-7610 will actually keep in a
	// memory — is NOT established by this document (matrix lift R17a).
	// This is the encodable figure, which is the only number the document
	// yields; see deliberatelyZero's note and §Proposed matrix errata.
	MaxEncodableFreqHz = 69_999_999
	// MaxToneDeciHz is the largest tone the record's three-byte
	// big-endian BCD tone spans can carry, in tenths of a hertz. Matrix
	// §1 row 8 / PDF p.14's six rotated nibble labels: 100 Hz: 0–2,
	// 10 Hz: 0–9, 1 Hz: 0–9, 0.1 Hz: 0–9.
	MaxToneDeciHz = 2999
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
	"ClarMaxHz":      "the 1A 00 record has no clarifier field, so there is no offset bound to state (matrix §2, clarifier row)",
	"ClarStepHz":     "the same: no clarifier field, no step",
	"CTCSSTones":     "this radio's tone spans are BCD FREQUENCIES, not indices into a chart. There is no tone number to index, and spec.Capabilities.Validate refuses a model declaring both a chart and a range. CTCSSToneRange is the declaration (tier ruling T1(2), enabler E3)",
	"RequiredSlots":  "nothing in this document says any IC-7610 memory or scan edge must stay populated. RequiredSlots is the per-SLOT mechanism and this radio uses none of it",
	"ShiftOptions":   "the Yaesu repeater-shift vocabulary. The 1A 00 record has no shift field at all, and FieldShift carries the zero FieldSupport on both banks, so enabler E5b's anyBankReaches guard makes the empty list lawful",
	"CTCSSStates":    "the Yaesu tone-state vocabulary. This radio expresses tone through ToneModes, the Icom one; a model declares one or the other",
	"DuplexOptions":  "the Icom repeater vocabulary. The 1A 00 record has no duplex field, FieldDuplex carries the zero FieldSupport on both banks, and E5b's guard makes the empty list lawful",
	"DTCSPolarities": "DTCS is printed NOWHERE in this document's 17 pages — swept for the matrix. An empty list is the positive statement that this radio expresses no DTCS polarity",
	"DTCSCodes":      "the same sweep: no DTCS code table is printed anywhere in this document",
	"MinFreqHz":      "zero IS this radio's declared floor rather than an omission: the record's frequency span is unsigned BCD and its smallest encodable value is 0 Hz. The STORABLE floor is not established by this document (matrix lift R17a), and declaring a non-zero guess would be a radio claim",
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
// is a separate question and rides matrix lift R18.
//
// EVERY spec.Field IS LISTED, including the ones this radio does not have.
// An absent key and an explicit zero FieldSupport mean the same thing to
// spec.Capabilities.FieldSupport, but only one of them is a decision a
// reader can check.
//
// Each call returns a fresh map, so no two banks share one.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		// The seven the record maps — core/civ/ic7610's layout() carries a
		// FieldSpan for each and nothing else.
		spec.FieldFrequency: rw, // (4)~(8), five bytes, little-endian BCD
		spec.FieldMode:      rw, // (9), the ten printed mode codes
		spec.FieldFilter:    rw, // (10), FIL1/FIL2/FIL3
		spec.FieldToneMode:  rw, // (11) low nibble, OFF/TONE/TSQL
		spec.FieldToneTx:    rw, // (12)~(14), three bytes, big-endian BCD
		spec.FieldToneRx:    rw, // (15)~(17), three bytes, big-endian BCD
		spec.FieldTag:       rw, // (18)~(27), ten name bytes

		// RULING E6 — THE TWO UNMAPPED NIBBLES. Byte (3)'s LOW nibble is a
		// four-valued SELECT-group marker (0=OFF, 1=★1, 2=★2, 3=★3;
		// matrix §3.16 ADDED-1) and byte (11)'s HIGH nibble is a
		// four-valued data mode (0=OFF, 1=DATA 1, 2=DATA 2, 3=DATA 3;
		// matrix §1b as corrected by erratum 5). Their neutral homes,
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
		// collapsed (adjudication R6). These grades DIFFER from matrix §2,
		// which has scan_skip Sup/Sup on MEM and data_mode Sup/Sup on
		// both; two errata are proposed under R16.
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
		// DTCS is printed nowhere in this document's 17 pages.
		spec.FieldDTCSCode:     {},
		spec.FieldDTCSPolarity: {},

		// ERASE. Unlike Yaesu, the wire form EXISTS on this radio, in two
		// shapes (1A 00 <ch> FF, and command 0B) — and NO IC-7610 has ever
		// been asked to use either. The zero FieldSupport is what makes
		// core/clone/execute.go's DiffErased branch unreachable for this
		// model, and spec.ConsentUnverifiedWrites structurally never
		// consents this field (its `f != FieldErase` guard), so consent
		// cannot open it either. Matrix §3.13's six requirements for a
		// future write-trial milestone are reproduced in doc.go.
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
		// §1 row 1 — the model name, which must equal the registry key
		// Wave 4 will register this driver under. The document sets it
		// "IC-7610" throughout.
		Model: "IC-7610",
		// §3.4 — the CI-V address 0x98, MANUAL-EVIDENCED, rendered as the
		// document prints it.
		//
		// THE STATIC VALUE IS THE ADDRESS ALONE (spec D3.2). A CI-V radio
		// has no CAT-ID equivalent readable before a port opens: the 19 00
		// token is a per-session observation, and this driver RECORDS it
		// and NEVER MATCHES it (D5 entry 7, matrix lift R7). Session
		// Identity carries "98" followed by the observed token; this
		// pre-probe field carries the half that is known statically.
		//
		// The stale "4-character CAT ID" comments in
		// core/spec/capabilities.go, core/driver/driver.go and
		// core/codeplug/radioinfo.go contradict spec D3.2. Those are files
		// this worktree does not own; doc.go records them for the Wave-4
		// doc pass.
		CATID: "98",
		Banks: []spec.Bank{
			{
				ID: spec.BankMemory,
				// A display string, not a protocol fact.
				Label: "Memories",
				Slots: memSlots(),
				// §1b — 99 memories, addressed 1..99.
				//
				// NoBlank stated FALSE explicitly, and conservatively:
				// nothing in this document says an IC-7610 memory channel
				// must stay populated.
				NoBlank: false,
				Fields:  memFields,
			},
			{
				ID:    spec.BankScan,
				Label: "Scan edges",
				Slots: scanSlots(),
				// §3.15(d) — P1 and P2 are two more values of the same
				// selector, not a protocol-level bank. NoBlank false for
				// the memory bank's reason.
				NoBlank: false,
				Fields:  scanFields,
			},
		},
		// §1 row 4 — the "①Receiving mode" column's ten printed codes, in
		// the UI's preferred order, which here is the printed one (left
		// sub-column then right): PDF p.11's "Operating mode" table,
		// corroborated at PDF p.14's "Command: 26".
		//
		// THESE ARE THE TEN VALUES OF core/civ/ic7610's modeEnum, and
		// TestModes_MatchTheCodec pins the two against each other by
		// decoding a record for every one. They are written out here
		// rather than derived from the codec because modeEnum is
		// unexported and core/civ/ic7610 is frozen evidence-bound code
		// this worktree's Stage 2 may not extend; the test is what makes
		// the duplication safe.
		//
		// Codes 0x06 and 0x09–0x11 are printed nowhere and are
		// deliberately absent from both lists: a record carrying one fails
		// to decode with a parse error naming the offset (register entry
		// ic7610-mode-code-completeness, matrix lift R19).
		//
		// THE RADIX OF THE LAST TWO CODES IS A RULING, NOT A READING —
		// RULING OQ1 of 24/08/2026 is HEXADECIMAL, so PSK is the wire byte
		// 0x12 and PSK-R is 0x13. See doc.go.
		Modes: []string{"LSB", "USB", "AM", "CW", "RTTY", "FM", "CW-R", "RTTY-R", "PSK", "PSK-R"},
		// §1 row 5 — PDF p.11's "Filter setting" column, whose fourth and
		// fifth rows are printed "-" (matrix Errata (rev 1) erratum 6).
		// The three values of core/civ/ic7610's filterEnum, pinned against
		// it by TestFilters_MatchTheCodec.
		//
		// 0x00 IS NOT A MEMBER, and that is a decision with a consequence:
		// a record whose (10) is 0x00 fails to decode rather than being
		// read as "no filter". The page prints three values and no
		// default; inventing a fourth would be a radio claim (register
		// entry ic7610-filter-value-set).
		Filters: []string{"FIL1", "FIL2", "FIL3"},
		// §1 row 8 / erratum 5 — byte (11)'s LOW nibble, with the
		// semantics the neutral model needs. TONE transmits a CTCSS tone;
		// TSQL transmits one AND squelches on the received one.
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS},
			{Value: "TSQL", Semantics: spec.ToneModeCTCSSSquelch},
		},
		// §1 row 8 / PDF p.14's six rotated nibble labels — and tier
		// ruling T1(2).
		//
		// THE PRINTED DIGIT RANGE AND THE DECLARED DOMAIN DIFFER, AND THE
		// DIFFERENCE IS THE POINT. The labels print 100 Hz: 0–2, 10 Hz:
		// 0–9, 1 Hz: 0–9, 0.1 Hz: 0–9, so the WIRE encodes 0..2999
		// deci-Hz. The CAPABILITY floor is max(printed minimum, 1),
		// because 0 Hz is not a tone: spec.ToneRange requires
		// MinDeciHz > 0 and Capabilities.Validate refuses a zero minimum.
		//
		// The wire's 0 is not lost by that — it is handled one layer up,
		// at ReadChannel, which maps an out-of-domain tone number (0
		// included) to UNKNOWN rather than to a Known value
		// codeplug.Validate would then refuse (T1(3)).
		//
		// RECORDED COST, from E3: the tone PICKER stays list-driven, so on
		// this model the grid shows and round-trips tones but the picker
		// cannot offer them. A Wave-4 item and an honesty row.
		CTCSSToneRange: &spec.ToneRange{MinDeciHz: 1, MaxDeciHz: MaxToneDeciHz, StepDeciHz: 1},
		// §1 row 9 — the six rates the document names for the CI-V link,
		// MANUAL-EVIDENCED as named; that the list is COMPLETE is ASSUMED
		// (matrix lift R12).
		Bauds: []int{4800, 9600, 19200, 38400, 57600, 115200},
		// §3.3 / OQ2 — ASSUMED, register home ic7610-default-baud, matrix
		// lift R11. THE DOCUMENT MARKS NO DEFAULT. The choice of 19200
		// within the printed six is ARBITRARY and is recorded as such: no
		// reading of this document favours one of them. What makes it safe
		// is not the choice but the grading and the failure mode — the
		// probe requires an address-matched 19 00 reply, and silence is
		// silence, so a wrong guess costs a clean timeout at Open and
		// never a wrong byte. The driver cannot sweep: internal/wiring
		// opens the port from this value, and Wave 3 may never touch it.
		DefaultBaud: 19200,
		// §1 row 12 — see MaxEncodableFreqHz. MinFreqHz is in
		// deliberatelyZero.
		MaxFreqHz: MaxEncodableFreqHz,
		// §1 row 7 — the name span (18)~(27) is ten bytes wide.
		TagLen: 10,
		// PDF p.12's two character tables, transcribed ONCE in
		// core/civ/ic7610 and referenced here. The Icom charset omits
		// several bytes the pre-Icom family default would allow, which is
		// exactly why spec.TagByteOK consults a capability-supplied set.
		TagCharset: civic7610.NameCharset,
	}
}

// capabilitiesUnverified is the REAL-HARDWARE profile while
// writeTrialsComplete is false: every mapped field Read Unverified and
// Write Unverified, so NOTHING is writable without recorded consent.
//
// The READ labels are Unverified rather than Supported, and that is the
// honest choice: this driver's read path has been exercised against a
// manual and (from Task 14) a fake, and NO IC-7610 HAS EVER ANSWERED A
// FRAME.
//
// Unexported, like core/driver/ftdx101's pair: every consumer outside this
// package reaches these values through New and driver.Driver.Capabilities,
// which is the route internal/wiring's StaticCapabilities already takes.
func capabilitiesUnverified() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	return baseCapabilities(bankFields(rw), bankFields(rw))
}

// capabilitiesSimulated is the internal/fakeic7610-backed profile (CLI
// --fake, GUI demo) and NEVER a real radio: Read AND Write Supported for
// exactly the seven fields the 1A 00 record maps, on MEM and SCAN alike.
//
// Against the fake, hardware risk is moot and the write choreography
// itself is what is being exercised end to end, so claiming Supported here
// is a claim about internal/fakeic7610 and about nothing else.
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
