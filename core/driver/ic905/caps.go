// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

import (
	civic905 "github.com/gm5dna/open-rig-programmer/core/civ/ic905"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// The two bank display LABELS this driver mints. Both are CHOICES —
// display strings, not protocol facts — transcribed from the IC-905
// capability matrix (§1b) rather than from a sibling driver, whose
// identical strings would be that radio's choice and not evidence about
// this one.
const (
	bankMemoryLabel = "Memories"
	bankCallLabel   = "Call channels"
)

// writeTrialsComplete is the IC-905's hardware write guard, and it is
// FALSE: no IC-905 has ever been asked anything by this project. There is
// no docs/hardware-notes.md section for this model, no write-trial
// protocol run, no captured frame from one, and every byte of every
// golden vector in core/civ/ic905/testdata is documentation-derived
// (matrix §3.14).
//
// While it is false there is no hardware-verified capability profile for
// this driver to select AT ALL — deliberately not even a placeholder
// one: a RealHardware session gets capabilitiesUnverified (see
// ic905Driver.Capabilities), nothing is writable anywhere, and the
// capability gate refuses every write before a frame is built.
//
// It is consulted by no production code, and that is the point. Flipping
// it is a TWO-PART change — this constant AND a hardware-derived profile
// built field class by field class from THIS model's own trial evidence,
// AND the Capabilities switch rewritten to select it — with the evidence
// linked and TestWriteTrialsComplete_PinnedFalse rewritten so the flip is
// a visible, reviewable test change. That test pins BOTH HALVES, so
// flipping the constant alone unlocks nothing.
//
// ONE constant, one model: unlike core/driver/ftdx101, this package
// drives a single radio with no registered sibling, so there is no
// sibling FALSE to share or be confused with.
const writeTrialsComplete = false

// Profile selects which capability profile New builds the driver with.
//
// The zero value is RealHardware ON PURPOSE: a forgotten or zero-valued
// Profile must fail towards the real-hardware capability set — which for
// this driver is the all-Unverified one, nothing writable — and NEVER
// towards the simulator's, whose Supported writes are a claim about
// internal/fakeic905 and about nothing else. Any OTHER unrecognised
// Profile value fails the same way, through Capabilities' explicit
// default arm.
type Profile int

const (
	// RealHardware is the profile for sessions against a physical radio.
	// While writeTrialsComplete is false it selects the all-Unverified
	// capability set: reads labelled Unverified, every candidate field's
	// Write Unverified, nothing writable.
	RealHardware Profile = iota
	// Simulated is the profile for internal/fakeic905-backed sessions
	// ONLY (the CLI's --fake mode, the GUI's demo mode): Write Supported
	// for the twelve fields the 1A 00 record can express, so the write
	// choreography can be exercised end to end with no hardware at risk.
	Simulated
)

// dtcsCodes generates the 512 DTCS codes this radio expresses: three
// OCTAL digits, printed as decimal, from 000 to 777.
//
// PDF p.24 (folio 23), "• DTCS code and polarity setting", Command:
// 1B 02 — byte ② is "0 (fixed)" / "First digit: 0 ~ 7", byte ③ is
// "Second digit: 0 ~ 7" / "Third digit: 0 ~ 7". Eight values per digit,
// three digits, so 8³ = 512 codes and every decimal digit is 7 or less.
//
// AN EXPLICIT TABLE RATHER THAN A RANGE, which is E3's own split: the
// tone domain is a contiguous numeric range and is declared as one
// (CTCSSToneRange below), but the DTCS codes are NOT contiguous — 008,
// 018 and 080 are all absent — so a range would admit codes this radio
// cannot store. It is generated rather than transcribed because a
// hand-typed 512-row literal is 512 chances to mistype a table the
// document states as one digit rule.
//
// IT IS THE PRIMARY GATE for OQ-6's digit-range rule: codeplug.Validate
// refuses a Known dtcs_code outside this table before the driver is
// reached. core/driver/ic905's own pre-build re-check (write.go, rung 6)
// is defence in depth, which the driver seam's contract requires;
// neither is a widening (ruling R11).
func dtcsCodes() []int {
	codes := make([]int, 0, 512)
	for first := 0; first < 8; first++ {
		for second := 0; second < 8; second++ {
			for third := 0; third < 8; third++ {
				codes = append(codes, first*100+second*10+third)
			}
		}
	}
	return codes
}

// callSlots is the CALL bank's twelve static slots, "C01".."C12", built
// through core/civ/ic905's own CallSlot so the strings this capability
// data advertises are exactly the ones slotAddress will accept.
//
// A DISTINCT CANONICAL NAMESPACE FROM MEM'S (ruling R4). MEM's sparse
// space spells itself with spec.SparseSlot's "G%02d-%03d";
// spec.ParseSparseSlot refuses any string without a leading "G", so no
// CALL slot can ever be read as a MEM address and no MEM address can
// ever render as a CALL slot. The disjointness is STRUCTURAL rather than
// an arithmetic accident of where the CALL group happens to sit, and
// core/civ/ic905's profile_test.go proves it over the whole
// 10,000-address space.
//
// They map to wire group 100, channels 00 00 … 00 11 (PDF p.19, folio
// 18: "00 00, 00 01: 144 C1, C2" … "00 10, 00 11: 10G C1, C2").
func callSlots() []string {
	slots := make([]string, 0, civic905.CallChannels)
	for n := 0; n < civic905.CallChannels; n++ {
		slots = append(slots, civic905.CallSlot(n))
	}
	return slots
}

// bankFields builds the per-field support map BOTH banks share,
// transcribed from matrix §2's eighty gradings. EVERY spec.Field this
// project models is listed explicitly, the eight zeros included: a field
// left out of the map reads identically to a field deliberately zeroed
// (Capabilities.FieldSupport returns the zero value for an absent key),
// and only a written-down zero is legible as a decision.
//
// ONE map shape for MEM and CALL, and matrix §2 is why: it grades the
// two banks IDENTICALLY on all twenty fields. The CALL column's one
// documented difference is byte ⑤'s footnote "* Set 0 for Call
// channel.", which is a VALUE constraint on a call-channel write, not a
// support difference — and it is satisfied by construction, ⑤ being
// Fixed 0x00 in both layouts.
//
// The twelve rw fields, with their record bytes:
//
//   - frequency (⑥~⑩), mode (⑪), filter (⑫), data_mode (⑬),
//     duplex (⑭ high), tone_mode (⑭ low), tone_tx (⑯~⑱),
//     tone_rx (⑲~㉑), dtcs_polarity (㉒), dtcs_code (㉓,㉔),
//     offset (㉖~㉘) and tag (53~68).
//
// The eight zeros, each for its own recorded reason (matrix §2 rows 3-6,
// 8-11):
//
//   - clarifier: MANUAL-EVIDENCED ABSENCE — the record's eighteen printed
//     field indices contain no clarifier and no RIT entry. (This radio
//     HAS a RIT surface, command 21 00, but it is operating state, not a
//     memory-record field.)
//   - ctcss_state and shift: CHOICE zeros, superseded by tone_mode and
//     duplex. D4's rule is that the two vocabularies never coexist on one
//     model, and this record's are the Icom pair: eight tone modes and
//     four duplex values, neither expressible in the Yaesu three.
//   - ctcss_tone: CHOICE zero, superseded by tone_tx/tone_rx. The tone is
//     a BCD FREQUENCY here, not an index into a fifty-entry chart, which
//     is also why CTCSSTones is empty and CTCSSToneRange is declared.
//   - tag_display: CHOICE zero — one writable name field (53~68) and no
//     separate display-only tag. The three call-sign blocks are call
//     signs, not names.
//   - scan_skip: CHOICE zero, and the TIER'S HARD CONSTRAINT. The nearest
//     candidate is byte ⑤, whose right nibble enumerates 0=OFF, 1=★1,
//     2=★2, 3=★3 — SELECT-group membership, which the document never
//     calls a scan-skip flag. scan_skip is NEVER mapped as skip on an
//     Icom, and ⑤ stays unmapped in Fixed at 0x00 (see
//     core/civ/ic905/doc.go, and write.go's rung 10, which REFUSES rather
//     than clears a channel whose ⑤ is set).
//   - erase: CHOICE zero, tier-wide. This tier ships no erase path at
//     all — no clear builder, no clear frame admitted by the gate, and
//     spec.ConsentUnverifiedWrites structurally never consents erase. A
//     clear form IS printed for this radio (PDF p.19, folio 18) and is
//     recorded in doc.go; nothing implements it.
//   - tx_frequency: MANUAL-EVIDENCED ABSENCE — exactly one frequency
//     field, no duplicated TX block (unlike the three models of spec D5
//     entry 4).
//
// Each call returns a fresh map, so no two banks share one.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		// The twelve the 1A 00 record expresses (matrix §2 rows 1, 2, 7,
		// 12-20).
		spec.FieldFrequency:    rw,
		spec.FieldMode:         rw,
		spec.FieldTag:          rw,
		spec.FieldDuplex:       rw,
		spec.FieldOffset:       rw,
		spec.FieldToneMode:     rw,
		spec.FieldToneTx:       rw,
		spec.FieldToneRx:       rw,
		spec.FieldDTCSCode:     rw,
		spec.FieldDTCSPolarity: rw,
		spec.FieldFilter:       rw,
		spec.FieldDataMode:     rw,

		// The eight written-down zeros (matrix §2 rows 3-6, 8-11). See
		// the doc comment for each one's own reason.
		spec.FieldClarifier:   {},
		spec.FieldCTCSSState:  {},
		spec.FieldCTCSSTone:   {},
		spec.FieldShift:       {},
		spec.FieldTagDisplay:  {},
		spec.FieldScanSkip:    {},
		spec.FieldErase:       {},
		spec.FieldTxFrequency: {},

		// The seven receiver per-channel fields the additions design (D8)
		// minted for the IC-R8600: this transceiver's 64/65-byte record
		// carries none of them, so each is a written-down zero, pinned by
		// TestFieldGrid_GradesEverySpecFieldThereIs.
		spec.FieldTuningStepEnabled: {},
		spec.FieldTuningStep:        {},
		spec.FieldProgramTuningStep: {},
		spec.FieldAttenuator:        {},
		spec.FieldPreamp:            {},
		spec.FieldAntenna:           {},
		spec.FieldIPPlus:            {},
	}
}

// baseCapabilities assembles the static baseline both profiles share, at
// the given support grade.
//
// It takes the GRADE rather than a built field map, and calls bankFields
// once per bank, so MEM and CALL can never end up sharing one map value —
// spec.Capabilities.Bank hands out defensive copies, but the baseline
// itself is walked directly by cloneCapabilities and by the tests.
//
// ALL TWENTY-SEVEN spec.Capabilities fields (twenty-two after the Icom
// tier, plus the additions design's five D8 receiver vocabularies, which
// this transceiver leaves deliberately empty) are populated explicitly, each
// from the IC-905 capability matrix's own §1 or §1b entry (cited per
// field below), and TestCapabilities_EveryFieldExplicit reflects over the
// struct to enforce it. A zero left in a capability field is not a
// neutral omission: a zero MaxFreqHz reads as "no ceiling" to every
// validator, a zero TagLen makes core/csvio's CHIRP import truncate every
// imported name to nothing, and a non-positive entry in Bauds reaches
// transport.OpenSerial's silent default substitution. Where the honest
// value is unverified it is populated anyway and the ASSUMED register
// carries the provenance — in core/civ/ic905/doc.go for the profile's
// nineteen entries, and in this package's doc.go for the driver's five.
func baseCapabilities(rw spec.FieldSupport) spec.Capabilities {
	return spec.Capabilities{
		// §1 row 1 — a CHOICE of display label over a manual-evidenced
		// fact (PDF p.1 cover panel, model mark IC-905). Sourced from the
		// DIALECT rather than restated, so the registry key and the
		// profile's own name are one string.
		Model: civic905.Model,
		// §1 row 2 — SPLIT. "AC" is MANUAL-EVIDENCED (PDF p.3, folio 2,
		// cell ②); the 19 00 ID token is ASSUMED (D5 entry 7, lift
		// ic905-R-02) and is undocumented on all six of this tier's
		// documents.
		//
		// THE STATIC VALUE IS THE ADDRESS ALONE, and it must be: there is
		// no observed token before a session exists. Open joins the two
		// as "AC:"+token on the SESSION's capabilities copy AND on
		// Identity, because core/clone's ReadAll records the SESSION
		// capabilities' CATID into the codeplug — see ic905.go.
		CATID: "AC",
		Banks: []spec.Bank{
			{
				ID: spec.BankMemory,
				// §1b — CHOICE. A display string, not a protocol fact.
				Label: bankMemoryLabel,
				// EMPTY in the static baseline, deliberately: MEM is a
				// SPARSE bank whose materialised set is DISCOVERED at
				// Open (read.go's discoverInventory) and published as
				// Bank.Slots, because core/clone.Service.ReadAll walks
				// exactly that. Asserting ten thousand slots statically
				// would claim every address is occupied.
				Slots: nil,
				// §1b — NoBlank FALSE, MANUAL-EVIDENCED: the clear form
				// explicitly admits memory groups 00 00 ~ 00 99 (PDF
				// p.19, folio 18), so memory channels are documented as
				// clearable. (This tier ships no erase path regardless.)
				NoBlank: false,
				Fields:  bankFields(rw),
				// The sparse space descriptor (§1b). Groups and PerGroup
				// are MANUAL-EVIDENCED — "00 00 ~ 00 99: Memory channel
				// group" and "00 00 ~ 00 99: 00 ~ 99", one hundred each.
				// Budget is ASSUMED: this document prints NO capacity
				// anywhere, and 500 is D4's roadmap-era figure. Register:
				// ic905.group_budget. Lift: ic905-R-09.
				//
				// The three exist so "is this slot within the space?" is
				// decidable from the Bank alone, which is what lets
				// codeplug.Diff admit an add at an address no read has
				// materialised — and which is exactly why write.go's
				// rung 11 (ruling T3) has to refuse an add the bounded
				// walk never saw.
				Sparse:   true,
				Groups:   100,
				PerGroup: 100,
				Budget:   500,
			},
			{
				ID: spec.BankCall,
				// §1b — CHOICE.
				Label: bankCallLabel,
				Slots: callSlots(),
				// §1b — NoBlank TRUE, MANUAL-EVIDENCED: the clear form's
				// own block prints, indented beneath the group line, "You
				// cannot specify group \"01 00\" (Call channel group)"
				// (PDF p.19, folio 18). The call channels cannot be
				// cleared by the documented clear form.
				NoBlank: true,
				Fields:  bankFields(rw),
			},
		},
		// §1 row 4 — vocabulary MANUAL-EVIDENCED, UI order a CHOICE (the
		// printed table's own). PDF p.17 (folio 16), "• Operating mode",
		// column "①Operating mode". Code 06 is absent from the printed
		// table with no note explaining the gap (matrix §6 EC-4) and is
		// absent here for the same reason.
		Modes: []string{"LSB", "USB", "AM", "CW", "RTTY", "FM", "CW-R", "RTTY-R", "DV", "DD", "ATV"},
		// §1 row 5 — MANUAL-EVIDENCED: "53~68: Memory name setting (16
		// characters, fixed)" (PDF p.19, folio 18, right legend column),
		// corroborated by the geometry witness's sixteen printed byte
		// positions.
		TagLen: 16,
		// §1 row 6 — CHOICE zero, over a MANUAL-EVIDENCED absence: the
		// record's eighteen printed field indices carry no clarifier and
		// no RIT entry. Zero is the honest value, and it is written down
		// rather than left out.
		ClarMaxHz: 0,
		// §1 row 7 — the same absence, graded separately because they are
		// separate struct fields and zeroing both without saying why
		// would be indistinguishable from forgetting them.
		ClarStepHz: 0,
		// §1 row 8 — EMPTY BY DECLARATION, not by omission. CTCSSTones is
		// indexed "by CAT tone number", which this radio's wire form does
		// not use: the tone is three packed-BCD bytes carrying tenths of
		// a hertz (PDF p.24, folio 23, "Command: 1B 00, 1B 01"), a NUMBER
		// rather than an index into a fifty-entry chart. The numeric
		// CTCSSToneRange below is what carries this radio's tone domain,
		// and spec.Validate refuses a radio that declares both.
		CTCSSTones: nil,
		// THE E3 TONE DOMAIN, AS RULING T1 REQUIRES IT DECLARED.
		//
		// The printed digit ranges, PDF p.24 (folio 23), "• Repeater
		// tone/tone squelch frequency settings", Command: 1B 00, 1B 01:
		// byte ① is "0 : 0", both halves "Fixed digit: 0*"; byte ② is
		// "100 Hz digit: 0 ~ 2" / "10 Hz digit: 0 ~ 9"; byte ③ is "1 Hz
		// digit: 0 ~ 9" / "0.1 Hz digit: 0 ~ 9". The ENCODABLE set is
		// therefore 0 … 2999 tenths of a hertz.
		//
		// THE DECLARED CAPABILITY DOMAIN IS THE INTERSECTION OF THAT WITH
		// "IS A TONE AT ALL" (T1(2)): min = max(printed minimum, 1) = 1,
		// because 0 Hz is not a tone, and spec.Validate refuses
		// MinDeciHz <= 0 outright. So:
		//
		//	encodable at the civ layer   0 .. 2999 deciHz  (lossless BCD)
		//	admissible as a CAPABILITY   1 .. 2999 deciHz  (declared here)
		//
		// The two differ by exactly the value zero, and the difference is
		// where T1(3) bites: a civ-layer tone of 0 maps to Unknown on
		// READ (read.go's toneField), never to a Known value
		// codeplug.Validate would refuse.
		//
		// THE BOUND IS THE PRINTED DIGIT RANGE, NOT WHAT BCD COULD HOLD
		// (ruling R11 forbids encoding-domain widening): three BCD bytes
		// could carry 999999 tenths; the page's 100 Hz digit stops at 2.
		//
		// Without this declaration codeplug.ToneField.Valid fails CLOSED
		// on an empty CTCSSTones — "rejecting every Known tone" — and
		// every channel this driver reads with a tone would fail
		// validation. Both golden vectors carry 88.5 Hz.
		CTCSSToneRange: &spec.ToneRange{MinDeciHz: 1, MaxDeciHz: 2999, StepDeciHz: 1},
		// §1 rows 9-10 — a CHOICE of which rates the UI offers, over an
		// ASSUMED default. THIS DOCUMENT PRINTS NO RATE, ANYWHERE.
		//
		// The full argument, both register entries and both lifts are in
		// this package's doc.go, "The serial rates: a CHOICE over an
		// ASSUMED default", where the plan puts them; they are cited here
		// rather than restated so the two cannot drift.
		Bauds:       []int{4800, 9600, 19200, 38400, 115200},
		DefaultBaud: 19200,
		// §1 rows 11-12 — the VALUES are MANUAL-EVIDENCED (PDF p.20,
		// folio 19, "• Band stacking register", table "①: Frequency band
		// codes", rows 01 | 144 | 144.000000 ~ 148.000000 and 06 | 10G |
		// 10000.000000 ~ 10500.000000); reading that table as the MEMORY
		// RECORD's storable range is the ASSUMPTION. Registers:
		// ic905.min_storable_hz (lift ic905-R-05) and
		// ic905.max_storable_hz (lift ic905-R-06).
		//
		// MaxFreqHz is the field that forced D4's uint64 widening:
		// 10,500,000,000 exceeds uint32's ceiling by a factor of about
		// 2.4, so the pre-tier struct could not hold this radio's ceiling
		// at all.
		MinFreqHz: 144_000_000,
		MaxFreqHz: 10_500_000_000,
		// §1 row 13 — EMPTY. This radio's non-clearable set is a whole
		// BANK, which Bank.NoBlank expresses on CALL above; RequiredSlots
		// is the per-SLOT mechanism and no individual memory channel is
		// named non-erasable anywhere in the document.
		RequiredSlots: nil,
		// §1 row 14 — EMPTY, superseded by DuplexOptions (D4: the two
		// vocabularies never coexist on one model). Mapping four Icom
		// duplex values onto three Yaesu shift values would lose RPS.
		ShiftOptions: nil,
		// §1 row 15 — EMPTY, superseded by ToneModes for the same reason:
		// StandardCTCSSStates' three semantics cannot express this
		// record's four split TX/RX combinations or its three DTCS states
		// without lying about what the radio stores.
		CTCSSStates: nil,
		// FOUR duplex values, E5-canonical-marked. PDF p.19 (folio 18),
		// the ⑭ breakout's LEFT nibble. RPS (Repeater Simplex, DD mode)
		// is simplex and therefore shares DuplexOff's semantics with OFF,
		// which is exactly the multiplicity E5 admits: OFF is marked
		// canonical, so csvio's reverse mapping still has one answer.
		DuplexOptions: []spec.DuplexOption{
			{Value: "OFF", Direction: spec.DuplexOff, Canonical: true},
			{Value: "DUP-", Direction: spec.DuplexDown, Canonical: true},
			{Value: "DUP+", Direction: spec.DuplexUp, Canonical: true},
			{Value: "RPS", Direction: spec.DuplexOff},
		},
		// EIGHT tone modes, same breakout's RIGHT nibble, same rule: four
		// of them are cross combinations, and TONE(T)/DTCS(R) is marked
		// canonical among them.
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff, Canonical: true},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS, Canonical: true},
			{Value: "TSQL", Semantics: spec.ToneModeCTCSSSquelch, Canonical: true},
			{Value: "DTCS", Semantics: spec.ToneModeDTCS, Canonical: true},
			{Value: "DTCS(T)", Semantics: spec.ToneModeCross},
			{Value: "TONE(T)/DTCS(R)", Semantics: spec.ToneModeCross, Canonical: true},
			{Value: "DTCS(T)/TSQL(R)", Semantics: spec.ToneModeCross},
			{Value: "TONE(T)/TSQL(R)", Semantics: spec.ToneModeCross},
		},
		// Byte ㉒, one nibble per direction: high is TRANSMIT, low is
		// RECEIVE (PDF p.24, folio 23, cell ①). The NIBBLE ASSIGNMENT is
		// ASSUMED — the leaders were followed by eye on a 600 dpi render
		// and found to NEST rather than cross (matrix Erratum 3).
		// Register: ic905.dtcs_polarity_nibbles. Lift: ic905-R-08. The
		// four spellings themselves are transmit-then-receive, N for
		// Normal and R for Reverse.
		DTCSPolarities: []string{"NN", "NR", "RN", "RR"},
		// The 512 codes 000…777, every digit ≤ 7 — see dtcsCodes.
		DTCSCodes: dtcsCodes(),
		// §1b — MANUAL-EVIDENCED: PDF p.17 (folio 16), "• Operating
		// mode" table, column "②Filter setting": 01:FIL1 02:FIL2 03:FIL3.
		Filters: []string{"FIL1", "FIL2", "FIL3"},
		// THE PROFILE'S OWN CHARSET, derived rather than restated: the
		// ninety-five bytes PDF p.20 (folio 19)'s "Codes for character
		// entries" prints, plus the ASSUMED 0x20 (register
		// ic905.name_pad_byte, lift ic905-R-17). Taking it from the
		// dialect makes a drift between what the UI offers and what the
		// codec accepts unrepresentable rather than merely tested for —
		// and civ's own V4 already requires the pad to be a charset
		// member.
		TagCharset: string(civic905.Profile().NameCharset()),
	}
}

// capabilitiesUnverified is the all-Unverified FAIL-SAFE profile, and it
// is what a RealHardware IC-905 session gets today: every field the
// 1A 00 record expresses is labelled Read Unverified / Write Unverified —
// documented in the CI-V reference guide and exercised against scripted
// peers, but never proven against a radio — and every field the record
// does not express stays the zero FieldSupport (matrix §2's profile
// table).
//
// Because Unverified makes FieldSupport.CanWrite false, this profile AS
// LABELLED blocks every write project-wide: codeplug.Diff refuses the
// change, the clone service refuses to execute a plan containing it, and
// Session.WriteChannel re-checks and refuses before building a frame. It
// is also what any UNRECOGNISED Profile value selects — the failure
// direction is always "nothing writable".
//
// THE ONE ROUTE PAST THAT, and it is the user's own: a session opened
// with WithConsentedUnverifiedWrites re-labels these write-side
// Unverified fields spec.ConsentedUnverified at session-capability
// assembly, and CanWrite is true for that state. The static profile is
// untouched and every unconsented session still cannot write. Two guards
// keep the route narrow: spec.ConsentUnverifiedWrites never touches
// FieldErase, and the transform is skipped entirely for an unrecognised
// Profile.
//
// The READ labels are Unverified rather than Supported for the same
// reason, and it is the honest choice: this driver's read path has been
// exercised against a fake and a manual, and NO IC-905 HAS EVER ANSWERED
// A FRAME.
func capabilitiesUnverified() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified})
}

// capabilitiesSimulated is the internal/fakeic905-backed profile (CLI
// --fake, GUI demo) and NEVER a real radio: Read AND Write Supported for
// exactly the TWELVE fields the 1A 00 record can express, on MEM and CALL
// alike (matrix §2's profile table).
//
// Against the fake, hardware risk is moot and the write choreography
// itself is what is being exercised end to end, so claiming Supported
// here is a claim about internal/fakeic905 and about nothing else.
//
// The eight zeros stay zero, simulator or not: the RECORD cannot express
// them (or, for scan_skip, must never be read as expressing them), and no
// amount of cooperative fake on the other end of the wire changes what
// the frame has room for. spec.FieldErase in particular stays zero in
// BOTH profiles — this tier ships no erase path at all.
func capabilitiesSimulated() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Supported, Write: spec.Supported})
}

// cloneCapabilities returns a deep copy of caps: Banks (each with fresh
// Slots and Fields) and every other slice independently allocated, so
// mutating the copy can never reach the original.
//
// Load-bearing for the write gate, exactly as in the Yaesu drivers:
// Session.Capabilities hands copies out, and a caller mutating one must
// never alter what WriteChannel enforces.
//
// CTCSSToneRange is a POINTER, so a shallow copy would share the pointee
// and a caller could move this radio's declared tone domain out from
// under the session's own validation. It is copied by value into a fresh
// allocation here.
func cloneCapabilities(caps spec.Capabilities) spec.Capabilities {
	out := caps
	out.Banks = make([]spec.Bank, 0, len(caps.Banks))
	for _, b := range caps.Banks {
		// Capabilities.Bank returns a defensive copy (fresh Slots and
		// Fields) — reuse that guarantee rather than restating per-field
		// copying here. The ok result is discarded safely because b came
		// out of caps.Banks and Bank scans that same slice for b.ID, and
		// spec.Validate refuses a duplicate BankID (TestProfiles_Validate
		// runs it over both profiles).
		cp, _ := caps.Bank(b.ID)
		out.Banks = append(out.Banks, cp)
	}
	out.Modes = append([]string(nil), caps.Modes...)
	out.CTCSSTones = append([]spec.Tone(nil), caps.CTCSSTones...)
	out.Bauds = append([]int(nil), caps.Bauds...)
	out.RequiredSlots = append([]string(nil), caps.RequiredSlots...)
	out.ShiftOptions = append([]spec.ShiftOption(nil), caps.ShiftOptions...)
	out.CTCSSStates = append([]spec.ToneState(nil), caps.CTCSSStates...)
	out.DuplexOptions = append([]spec.DuplexOption(nil), caps.DuplexOptions...)
	out.ToneModes = append([]spec.ToneMode(nil), caps.ToneModes...)
	out.DTCSPolarities = append([]string(nil), caps.DTCSPolarities...)
	out.DTCSCodes = append([]int(nil), caps.DTCSCodes...)
	out.Filters = append([]string(nil), caps.Filters...)
	if caps.CTCSSToneRange != nil {
		r := *caps.CTCSSToneRange
		out.CTCSSToneRange = &r
	}
	return out
}

// effectiveCapabilities builds a Session's capability set: a deep copy of
// the profile baseline with the MEM bank's SPARSE inventory materialised
// from discovered.
//
// IT IS THE OTHER HALF OF discoverInventory, and without it discovery
// would be a diagnostic nobody reads: core/clone.Service.ReadAll walks
// Session.Capabilities().Banks[].Slots and calls ReadChannel per slot, so
// a sparse bank whose Slots stayed empty would return no memories at all
// (ruling R12).
//
// The CALL bank is untouched — its twelve slots are static and always
// present, and the read of one is what tells a user whether it holds
// anything.
//
// discovered is copied, never aliased: the session must not hold a slice
// the walk's caller can still write to.
func effectiveCapabilities(base spec.Capabilities, discovered []string) spec.Capabilities {
	caps := cloneCapabilities(base)
	for i := range caps.Banks {
		if caps.Banks[i].ID != spec.BankMemory {
			continue
		}
		caps.Banks[i].Slots = append([]string(nil), discovered...)
	}
	return caps
}
