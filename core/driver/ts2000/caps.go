// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000

import (
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	kwts2000 "github.com/gm5dna/open-rig-programmer/core/kw/ts2000"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// modelParams is the whole of what distinguishes the three registry rows:
// the display name, the CAT ID this document's ID legend prints (or
// ASSUMES), and the codec Layout to speak with — which the matrix pins as
// byte-identical across all three (§1-§2), so the same LAYOUT VALUE is
// shared and only the name/CATID pair varies.
type modelParams struct {
	name   string
	catID  string
	layout kw.Layout
}

// The three rows. CATID is MANUAL-EVIDENCED for TS-2000 alone — "019:
// TS-2000" is the only value this document's ID legend prints
// (ts2000:10429, PDF p.124) — and ASSUMED for the other two: no second or
// third ID value is printed anywhere in the command table for the
// TS-2000X or the TS-B2000, and the matrix's own register entry (lift: an
// "ID;" read on an actual TS-2000X and TS-B2000, once per row) is what
// would upgrade either. A CONSEQUENCE WORTH STATING: because all three
// rows are given the SAME assumed CATID, this driver's probe cannot tell
// a TS-2000X from a TS-2000 or a TS-B2000 by identity alone — opening any
// one of the three constructors against a real radio bearing "019"
// succeeds identically, and only the caller's own choice of constructor
// says which row it believes it is talking to.
var (
	paramsTS2000  = modelParams{name: "TS-2000", catID: "019", layout: kwts2000.TS2000}
	paramsTS2000X = modelParams{name: "TS-2000X", catID: "019", layout: kwts2000.TS2000X}
	paramsTSB2000 = modelParams{name: "TS-B2000", catID: "019", layout: kwts2000.TSB2000}
)

// siblingModelName names the model a Kenwood ID token belongs to, for a
// wrong-radio refusal — core/driver/ts480's namesake, restated here because
// this row's own three CAT IDs (all "019") tell a wrong-radio refusal
// nothing a caller does not already know.
func siblingModelName(id string) string {
	switch id {
	case "019":
		return "TS-2000 / TS-2000X / TS-B2000"
	case "020":
		return "TS-480"
	case "021":
		return "TS-590S"
	case "022":
		return "TS-990S"
	case "023":
		return "TS-590SG"
	case "024":
		return "TS-890S"
	default:
		return ""
	}
}

// Profile selects which capability profile New builds the driver with —
// shared with every other driver package (core/driver.Profile); this
// package keeps its own Simulated selector, which
// internal/guards.TestSimulatedProfileTokensConfinement requires.

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// writeTrialsComplete is FALSE: no TS-2000, TS-2000X or TS-B2000 has ever
// been asked anything by this project (matrix, line 8: "NO
// TS-2000/2000X/B2000 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT").
//
// ON A REAL RADIO IT IS THE ONLY GUARD, UNLIKE THE TS-480'S TWO: this row's
// write.go DOES build and send MW frames once a session is consented (or
// Simulated) — the "write existing" ladder, preserving what it does not
// model (write.go's own doc comment) — so with writeTrialsComplete false
// it is this constant alone that keeps an unconsented RealHardware
// session's writes all-Unverified and therefore unwritable. This row is
// additionally unregistered until Phase 4.
const writeTrialsComplete = false

// memBankLabel, scanBankLabel and satBankLabel are the three banks'
// display labels, minted as this package's own consts — a CHOICE, not a
// protocol fact (matrix §1.4.1's precedent).
const (
	memBankLabel  = "Memories"
	scanBankLabel = "Program Scan"
	satBankLabel  = "Satellite Memory"
)

// satSlots is the ten-channel Satellite Memory bank's slot inventory —
// SA's own P2 domain, "0 ~ 9: Satellite Memory Channel number"
// (ts2000:11312-11313). Fixed and small enough to spell out rather than
// loop-build like memSlots/scanSlots.
func satSlots() []string {
	return []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}
}

// modeNames returns the selectable mode display names this row advertises,
// derived from the layout rather than transcribed here — see
// core/kw/ts2000's own modeNames for the citation.
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

// memSlots and scanSlots return this row's two bank inventories, built
// through the LAYOUT's own NewSlot so the numbers this capability data
// advertises are the ones the row's slot space actually accepts.
func memSlots(l kw.Layout) []string {
	var slots []string
	for n := 0; n <= 289; n++ {
		s, err := l.NewSlot(n, kw.ScanHalfNone)
		if err != nil || s.Class() != kw.SlotMemory {
			continue
		}
		slots = append(slots, s.String())
	}
	return slots
}

// scanSlots renders the ten Program Scan channels as their two halves,
// "290L".."299L" then "290U".."299U" — the 590-pair SCAN-bank shape (matrix
// §4, "same reasoning as exemplar §1.4.3").
func scanSlots(l kw.Layout) []string {
	var slots []string
	for _, half := range []kw.ScanHalf{kw.ScanLower, kw.ScanUpper} {
		for n := 290; n <= 299; n++ {
			s, err := l.NewSlot(n, half)
			if err != nil || s.Class() != kw.SlotScan {
				continue
			}
			slots = append(slots, s.String())
		}
	}
	return slots
}

// kenwoodTS2000CTCSSTones is THIS DOCUMENT'S OWN 39-entry tone chart, index
// = the CAT tone number, transcribed from TN's own printed table
// (ts2000:3837-3847, PDF p.35 body page) and TN's legend ("01~39... Refer to
// page 35", ts2000:11615-11617).
//
// IT IS A THIRD KENWOOD CHART, NOT THE 590 PAIR'S 43-ENTRY ONE (matrix §4):
// do not borrow, and TestNoProductionFileNamesTheSharedToneChart (like the
// 590/480 pair's own) holds that down.
//
// THE 39TH ENTRY IS AN OVER-CLAIM ON THE RECEIVE SIDE, THE 590 PAIR'S OWN
// M-E1 ERRATUM RESTATED FOR THIS CHART: TN's (P8, tone_tx) legend prints
// "01~39" while CN's (P9, tone_rx) prints "01~38" (matrix §2's P9 row cites
// "See CN command"; the matrix's CTCSSToneRange entry cites CN's own
// "01~38... refer to page 35", ts2000:9930 region) — so 1750 Hz (index 38,
// zero-based) is a value TN can carry and CN cannot. write.go's own
// candidate() enforces this on the write path (a Known tone_rx of 1750 Hz
// is refused), the same runtime rung the 590 pair's own M-E1 needs — this
// row's writes are no longer refused unconditionally, so the note alone
// would no longer be enough.
//
// TRANSCRIBED DIRECTLY FROM THE CHART, NOT COPIED FROM THE MATRIX'S OWN
// INLINE LISTING: the matrix's §4 CTCSSTones cell (as written) omits
// 192.8 Hz (No. 31) and prints two values, 206.5 and 229.1, that the chart
// itself does not contain — a transcription slip in that summary line. Its
// own prose ("38 standard tones + 1750 Hz") and its own citation
// (ts2000:3837-3847) are what this list is checked against, per the
// matrix's own instruction to "transcribe independently in the driver's
// doc.go" rather than copy the inline figure verbatim.
var kenwoodTS2000CTCSSTones = []spec.Tone{
	670, 719, 744, 770, 797, 825, 854, 885, 915, 948, // 01-10
	974, 1000, 1035, 1072, 1109, 1148, 1188, 1230, 1273, 1318, // 11-20
	1365, 1413, 1462, 1514, 1567, 1622, 1679, 1738, 1799, 1862, // 21-30
	1928, 2035, 2107, 2181, 2257, 2336, 2418, 2503, // 31-38
	17500, // 39
}

// duplexOptions is the FieldDuplex vocabulary: three of OS's four printed
// values (matrix §4), Value spelt as the record's own single wire digit
// (P12, checkP12) so read.go/write.go round-trip it with no translation
// table of its own.
func duplexOptions() []spec.DuplexOption {
	return []spec.DuplexOption{
		{Value: "0", Direction: spec.DuplexOff},
		{Value: "1", Direction: spec.DuplexUp},
		{Value: "2", Direction: spec.DuplexDown},
	}
}

// toneModes is the FieldToneMode vocabulary: three of P7's four printed
// values (matrix §4) — the fourth, DCS, is recorded as an open item rather
// than graded (matrix §6 item 1; see read.go).
func toneModes() []spec.ToneMode {
	return []spec.ToneMode{
		{Value: "OFF", Semantics: spec.ToneModeOff},
		{Value: "TONE", Semantics: spec.ToneModeCTCSS},
		{Value: "CTCSS", Semantics: spec.ToneModeCTCSSRxSquelch},
	}
}

// bankFields builds ONE bank's per-field support map, over the SAME
// twenty-seven-entry shape core/driver/ts480/caps.go states in full (this
// row's own reasons are unexpressedFields, caps_test.go).
//
// NINE FIELDS ARE GRADED HERE, AGAINST THE TS-480'S FIVE: this row's own
// P10/P12/P13 lift adds FieldDuplex and FieldOffset (live, unlike the
// registered rows' printed constants), and THIS DOCUMENT'S OWN 39-entry
// tone chart (above) is what additionally grades FieldToneTx/FieldToneRx —
// the TS-480's sharpest single loss (no chart in its book at all) does not
// apply here.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency: rw, // P4 (ts2000:10693-10695 area)
		spec.FieldMode:      rw, // P5, legend via MD
		spec.FieldTag:       rw, // P16, 8 bytes at 42-49
		spec.FieldScanSkip:  rw, // P6, byte 19: lockout (ts2000:10704)
		spec.FieldToneMode:  rw, // P7, byte 20 (ts2000:10706-10708)
		spec.FieldDuplex:    rw, // P12, byte 29 (ts2000:10715-10716)
		spec.FieldOffset:    rw, // P13, bytes 30-38 (ts2000:10717-10718)
		spec.FieldToneTx:    rw, // P8, this document's own 39-entry chart
		spec.FieldToneRx:    rw, // P9, the same chart (CN)

		spec.FieldClarifier:    {},
		spec.FieldCTCSSState:   {},
		spec.FieldCTCSSTone:    {},
		spec.FieldShift:        {},
		spec.FieldTagDisplay:   {},
		spec.FieldErase:        {},
		spec.FieldTxFrequency:  {},
		spec.FieldDTCSCode:     {},
		spec.FieldDTCSPolarity: {},
		spec.FieldFilter:       {},
		spec.FieldDataMode:     {},

		spec.FieldTuningStepEnabled: {},
		spec.FieldTuningStep:        {},
		spec.FieldProgramTuningStep: {},
		spec.FieldAttenuator:        {},
		spec.FieldPreamp:            {},
		spec.FieldAntenna:           {},
		spec.FieldIPPlus:            {},

		// The three Satellite Memory bank fields (v1.10.0): a different
		// bank's record, with no position in this one.
		spec.FieldSatBandSwap: {},
		spec.FieldSatTrace:    {},
		spec.FieldSatTraceRev: {},
	}
}

// satelliteBankFields builds the Satellite Memory bank's field support
// map (v1.10.0, core/kw/ts2000's SA/SI). FOUR FIELDS ARE GRADED: the name
// (SI) and the three per-channel flags core/kw/ts2000/satellite.go's own
// doc comment derives from the manual (SA's P3/P5/P6). FieldFrequency is
// explicitly Unsupported — MANUAL-EVIDENCED ABSENCE: "Use the FA
// (downlink) or FB (uplink) command to change the frequencies."
// (ts2000:11330-11331) — this bank carries no frequency of its own at
// all. Every other field is Unsupported for the same reason the memory
// bank's own eighteen zeros are: no position in THIS record.
func satelliteBankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		spec.FieldTag:         rw, // SI, the 8-byte name (ts2000:11404-11413)
		spec.FieldSatBandSwap: rw, // SA P3 (ts2000:11314-11317)
		spec.FieldSatTrace:    rw, // SA P5 (ts2000:11319-11320)
		spec.FieldSatTraceRev: rw, // SA P6 (ts2000:11320-11321)

		// No frequency field in this record at all — see this function's
		// own doc comment.
		spec.FieldFrequency: {},

		spec.FieldMode:         {},
		spec.FieldClarifier:    {},
		spec.FieldCTCSSState:   {},
		spec.FieldCTCSSTone:    {},
		spec.FieldShift:        {},
		spec.FieldTagDisplay:   {},
		spec.FieldScanSkip:     {},
		spec.FieldErase:        {},
		spec.FieldTxFrequency:  {},
		spec.FieldDuplex:       {},
		spec.FieldOffset:       {},
		spec.FieldToneMode:     {},
		spec.FieldToneTx:       {},
		spec.FieldToneRx:       {},
		spec.FieldDTCSCode:     {},
		spec.FieldDTCSPolarity: {},
		spec.FieldFilter:       {},
		spec.FieldDataMode:     {},

		spec.FieldTuningStepEnabled: {},
		spec.FieldTuningStep:        {},
		spec.FieldProgramTuningStep: {},
		spec.FieldAttenuator:        {},
		spec.FieldPreamp:            {},
		spec.FieldAntenna:           {},
		spec.FieldIPPlus:            {},
	}
}

// baseCapabilities assembles one row's static baseline at the given
// evidence grade. All twenty-eight spec.Capabilities fields are populated
// explicitly (TestCapabilities_EveryFieldExplicit).
func baseCapabilities(p modelParams, rw spec.FieldSupport) spec.Capabilities {
	l := p.layout
	return spec.Capabilities{
		Model: p.name,
		CATID: p.catID,
		// MANUAL-EVIDENCED (matrix §4): an HF/VHF/UHF transceiver with a
		// documented transmit surface throughout (e.g. TO, "TONE ON/OFF",
		// is meaningless on a receiver).
		Transmit: spec.HasTransmitter,
		Banks: []spec.Bank{
			{
				ID: spec.BankMemory, Label: memBankLabel,
				Slots: memSlots(l), NoBlank: false, Fields: bankFields(rw),
			},
			{
				ID: spec.BankScan, Label: scanBankLabel,
				Slots: scanSlots(l), NoBlank: false, Fields: bankFields(rw),
			},
			{
				// CurrentChannelOnly: SA's read has no per-channel
				// address (satellite.go's own doc comment) — a
				// whole-radio ReadAll cannot enumerate this bank and
				// must skip it (core/clone/read.go).
				ID: spec.BankSatellite, Label: satBankLabel,
				Slots: satSlots(), NoBlank: false, Fields: satelliteBankFields(rw),
				CurrentChannelOnly: true,
			},
		},
		Modes: modeNames(l),
		// MANUAL-EVIDENCED (matrix §4): P16, "A maximum of 8 characters."
		// (ts2000:10726). NoTag stays false (the zero value): this package
		// is outside the wave's ten NoTag rows — the tag route is live.
		TagLen: 8,
		// 0/0 — MANUAL-EVIDENCED ABSENCE FROM THE RECORD (matrix §4): P1-P16
		// account for all 47 parameter bytes and none is an RIT/XIT offset.
		ClarMaxHz:  0,
		ClarStepHz: 0,
		// This document's own 39-entry chart; copied per call so no two
		// capability values share the backing array.
		CTCSSTones:     append([]spec.Tone(nil), kenwoodTS2000CTCSSTones...),
		CTCSSToneRange: nil,
		// Five rates, MANUAL-EVIDENCED (matrix §4, Menu 56): unlike the 590
		// pair, nothing here conditions 4800 bps on the connector.
		Bauds: []int{4800, 9600, 19200, 38400, 57600},
		// ASSUMED per spec §3's blanket rule, though Menu 56's own default
		// column names it (matrix §4).
		DefaultBaud: 9600,
		// 0/0 — CANNOT ESTABLISH (matrix §4): the PC-command appendix
		// prints no tuning range; the instruction manual's own General
		// Specifications page is unfetched.
		MinFreqHz: 0,
		MaxFreqHz: 0,
		// nil — MANUAL-EVIDENCED ABSENCE (matrix §4): no channel is
		// documented mandatory.
		RequiredSlots: nil,
		// Both nil (matrix §4, design decision 6): the Icom half is
		// declared instead (DuplexOptions/ToneModes below).
		ShiftOptions: nil,
		// Three of OS's four values (matrix §4); the fourth ("All
		// E-types") is recorded, not published (matrix §6 item 2).
		DuplexOptions: duplexOptions(),
		// Three of P7's four values (matrix §4); the fourth (DCS) is
		// recorded, not published (matrix §6 item 1).
		ToneModes: toneModes(),
		// Both empty (matrix §4): the DCS chart and QC's legend name a
		// code index only; no NN/NR/RN/RR polarity concept is printed.
		DTCSPolarities: nil,
		// nil — the 104-entry DCS chart is cited but not transcribed this
		// milestone (matrix §6 item 4); FieldDTCSCode is graded the zero
		// FieldSupport in consequence (caps_test.go's unexpressedFields).
		DTCSCodes: nil,
		// nil (matrix §4): byte 28 is REVERSE here, not Filter A/B; no
		// other per-channel filter position exists.
		Filters: nil,
		// All empty/nil (matrix §4): additions design D8 — none applies to
		// any of this wave's seven packages (spec §3 note).
		TuningSteps:            nil,
		ProgramTuningStepRange: nil,
		AttenuatorDB:           nil,
		PreampOptions:          nil,
		AntennaOptions:         nil,
		// "" — the family default (printable ASCII 0x20-0x7E excluding
		// ';'): no charset note beyond P16's own 8-character width is
		// printed (ASSUMED, ts590/ts480 precedent).
		TagCharset: "",
	}
}

// CapabilitiesUnverified is this row's all-Unverified FAIL-SAFE profile —
// the ts480/ts590 shape exactly: every field the 50-byte record expresses
// is Read Unverified / Write Unverified, and it is what a RealHardware
// session gets while writeTrialsComplete is false. Every unrecognised
// Profile value selects it too.
func CapabilitiesUnverified(p modelParams) spec.Capabilities {
	return baseCapabilities(p, spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified})
}

// CapabilitiesSimulated is this row's internal/fake-backed profile (CLI
// --fake, GUI demo): Read AND Write Supported for the nine fields the
// record expresses on this row. It never claims anything about a real
// TS-2000/2000X/B2000 — write.go's own "write existing" ladder is what
// actually builds and sends an MW Set once a session passes this gate
// (consented, or Simulated), preserving the four raw values no
// spec.Field models (write.go's own doc comment).
func CapabilitiesSimulated(p modelParams) spec.Capabilities {
	return baseCapabilities(p, spec.FieldSupport{Read: spec.Supported, Write: spec.Supported})
}
