// SPDX-License-Identifier: GPL-3.0-or-later

package ic7600

import (
	civic7600 "github.com/gm5dna/open-rig-programmer/core/civ/ic7600"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// writeTrialsComplete is this model's hardware write guard, and it is
// FALSE.
//
// FALSE BECAUSE NO IC-7600 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT
// (matrix §0, §3.14). Every byte in this package's tables came from the
// IC-7600's own Instruction Manual, via
// docs/superpowers/icom-matrices/ic7600-capability-matrix.md rev 1, and
// from nothing else.
//
// FLIPPING IT IS A TWO-PART CHANGE, and stating that here is the point of
// the constant: (1) this constant, AND (2) a capabilitiesRealHardware
// profile built field class by field class from an IC-7600's OWN trial
// evidence, AND (3) the Capabilities switch in ic7600.go rewritten to
// select it.
//
// THERE IS NO REGISTERED SIBLING (matrix §4), so this is ONE constant and
// ONE register row.
const writeTrialsComplete = false

// Profile selects which capability profile New builds the driver with.
//
// The zero value is RealHardware ON PURPOSE: a forgotten or zero-valued
// Profile must fail TOWARDS the real-hardware capability set - which
// while writeTrialsComplete is false is the all-Unverified one, nothing
// writable - and NEVER towards the simulator's.
type Profile = driver.Profile

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// The two numeric bounds Task 12's pre-build refusals enforce, stated once
// here so the capability domain and the refusal cannot drift apart. THEY
// ARE DEFENCE IN DEPTH AND NOT THE GATE (see core/driver/ic7610's own
// doc.go for the shared reasoning).
const (
	// MaxEncodableFreqHz is the largest frequency the record's five-byte
	// little-endian BCD frequency span can carry. Matrix S1 row 16: the
	// 10 MHz digit is labelled 0-6 and the fifth cell is fixed "0 : 0",
	// so 69 999 999 Hz is the ceiling - IDENTICAL to the IC-7610's own
	// bound, same digit range.
	//
	// The STORABLE ceiling is NOT established by this document (matrix
	// lift R17, this radio's own entry ic7600-ctcss-tone-list is the
	// tone-list analogue; the frequency-ceiling equivalent mirrors the
	// IC-7610's own R17a/R17b). This is the ENCODABLE figure.
	MaxEncodableFreqHz = 69_999_999
	// MaxToneDeciHz is the largest tone the record's three-byte
	// big-endian BCD tone spans can carry, in tenths of a hertz. Matrix
	// S1 row 12: 100 Hz digit 0-2, 10 Hz/1 Hz/0.1 Hz digits 0-9 -
	// IDENTICAL to the IC-7610's own bound.
	MaxToneDeciHz = 2999
)

// deliberatelyZero is adjudication R11's audit table: every
// spec.Capabilities field this model leaves at its zero value, with the
// reason. TestDeliberatelyZeroAudit reflects over the struct and requires
// each field to be either populated (cited in baseCapabilities) or listed
// here.
var deliberatelyZero = map[string]string{
	"ClarMaxHz":              "the 1A 00 record has no clarifier field (matrix S1 row 9)",
	"ClarStepHz":             "the same: no clarifier field, no step (matrix S1 row 10)",
	"CTCSSTones":             "this radio's tone spans are BCD FREQUENCIES, not indices into a chart (matrix S1 row 11/12). CTCSSToneRange is the declaration (tier ruling T1(2), enabler E3)",
	"RequiredSlots":          "nothing in this document says any IC-7600 memory or scan edge must stay populated (matrix S1 row 17)",
	"ShiftOptions":           "the Yaesu repeater-shift vocabulary. The 1A 00 record has no shift field at all (matrix S1 row 18), and FieldShift carries the zero FieldSupport on both banks",
	"CTCSSStates":            "the Yaesu tone-state vocabulary (matrix S1 row 19). This radio expresses tone through ToneModes, the Icom one",
	"TuningSteps":            "additions design D8 - not applicable to this transceiver (matrix S1 row 25)",
	"ProgramTuningStepRange": "additions design D8 (matrix S1 row 26)",
	"AttenuatorDB":           "additions design D8 (matrix S1 row 27)",
	"PreampOptions":          "additions design D8 (matrix S1 row 28)",
	"AntennaOptions":         "additions design D8 (matrix S1 row 29)",
	"SimplexTx":              "this row does not grade FieldTxFrequency (matrix S1 row 4), so the blank arm leaves nothing to state",
	"DuplexOptions":          "the Icom repeater vocabulary (matrix S1 row 20). The 1A 00 record has no duplex field, FieldDuplex carries the zero FieldSupport on both banks",
	"DTCSPolarities":         "DTCS is printed NOWHERE in this document (matrix S1 row 21/22, full-document sweep) - this radio predates digital code squelch",
	"DTCSCodes":              "the same sweep: no DTCS code table is printed anywhere (matrix S1 row 23)",
	"MinFreqHz":              "zero IS this radio's declared floor: the record's frequency span is unsigned BCD, smallest encodable value 0 Hz (matrix S1 row 15). The STORABLE floor is not established by this document",
	"NoTag":                  "the IC-7600 supports channel names via the (18)~(27) ten-byte tag field (matrix S1 row 7/8); NoTag is false",
}

// memSlots is the MEM bank's inventory: "001".."099".
//
// Matrix S1 row 5 - the two-byte channel selector runs 1..99 for the
// memories (BCD "00 01".."00 99"), and civ.ProfileConfig.ChannelLo/
// ChannelHi carry the same range.
func memSlots() []string { return spec.NumberedSlots(1, 99, "%03d") }

// scanSlots is the SCAN bank's inventory: the two scan edges.
//
// Matrix S1b - P1 and P2 are NOT a separate bank in the wire protocol.
// They are two more values of the same two-byte selector (BCD "01 00"
// and "01 01", channel numbers 100 and 101, matching
// civ.ProfileConfig.ChannelHi = 101).
func scanSlots() []string { return []string{"P1", "P2"} }

// bankFields returns one bank's field map, with rw applied to every field
// the 1A 00 record MAPS and the zero FieldSupport everywhere else.
//
// IDENTICAL FOR MEM AND SCAN (matrix S2): both banks read and write the
// SAME 1A 00 record at different values of the same selector. Whether
// every field is HONOURED on a scan edge rides matrix lift R18
// (ic7600-scan-edge-record-fields).
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		// The seven the record maps - core/civ/ic7600's layout() carries a
		// FieldSpan for each and nothing else.
		spec.FieldFrequency: rw, // (4)~(8), five bytes, little-endian BCD
		spec.FieldMode:      rw, // (9), the ten printed mode codes
		spec.FieldFilter:    rw, // (10), FIL1/FIL2/FIL3
		spec.FieldToneMode:  rw, // (11) low nibble, OFF/TONE/TSQL
		spec.FieldToneTx:    rw, // (12)~(14), three bytes, big-endian BCD
		spec.FieldToneRx:    rw, // (15)~(17), three bytes, big-endian BCD
		spec.FieldTag:       rw, // (18)~(27), ten name bytes

		// RULING E6 - THE TWO UNMAPPED REGIONS. Byte (3) is a whole-byte,
		// four-valued SELECT-group marker (0=OFF, 1=star1, 2=star2,
		// 3=star3; matrix S3.16 ADDED-1) - drawn as one undivided enum
		// cell on THIS radio's own page, not the IC-7610's two-nibble
		// split (matrix S3.15(a); see core/civ/ic7600's SelectByteOffset
		// comment) - and byte (11)'s HIGH nibble is a four-valued data
		// mode (0=OFF, 1=DATA 1, 2=DATA 2, 3=DATA 3; matrix S2 row 20, no
		// divergence from the IC-7610 there). Their neutral homes,
		// codeplug.ChannelData.ScanSkip and .DataMode, are BOTH BoolField.
		//
		// A 4->2 collapse would rewrite a user's SELECT group or data
		// mode on every write-back while readback verification compared
		// equal - silent corruption this project refuses. So both regions
		// are UNMAPPED in the civ layout and both fields carry the zero
		// FieldSupport here, on both banks, under both profiles.
		//
		// THE CONSEQUENCE: a Known ScanSkip or DataMode is REFUSED by the
		// capability gate before any wire traffic. These grades DIFFER
		// from a naive reading of matrix S2 (which grades the underlying
		// WIRE vocabulary MANUAL-EVIDENCED before applying E6) - the
		// divergence is flagged there, not silently applied.
		spec.FieldScanSkip: {},
		spec.FieldDataMode: {},

		// MANUAL-EVIDENCED ABSENCE: the 1A 00 record has no such span.
		spec.FieldClarifier:    {}, // no clarifier span
		spec.FieldShift:        {}, // no repeater-shift span
		spec.FieldDuplex:       {}, // no duplex span
		spec.FieldOffset:       {}, // no repeater-offset span
		spec.FieldTxFrequency:  {}, // ONE frequency span; no split-TX field
		spec.FieldTagDisplay:   {}, // no name-display flag
		spec.FieldCTCSSState:   {},
		spec.FieldCTCSSTone:    {},
		spec.FieldDTCSCode:     {},
		spec.FieldDTCSPolarity: {},

		// ERASE. The wire form EXISTS on this radio, in two shapes
		// (1A 00 <ch> FF, and command 0B) - and NO IC-7600 has ever been
		// asked to use either. The zero FieldSupport is what makes
		// core/clone/execute.go's DiffErased branch unreachable for this
		// model.
		spec.FieldErase: {},
	}
}

// baseCapabilities assembles the static baseline both profiles share, with
// the given per-bank field maps.
func baseCapabilities(memFields, scanFields map[spec.Field]spec.FieldSupport) spec.Capabilities {
	return spec.Capabilities{
		// Matrix S1 row 1 - the model name.
		Model: "IC-7600",
		// Matrix S1 row 2 / S3.4 - the CI-V address 0x7A, MANUAL-EVIDENCED
		// directly on this radio's own front-panel Set Mode page (PDF
		// p.151 folio 142: "The IC-7600's address is 7Ah"), a genuine
		// improvement in evidence quality over the IC-7610's own document.
		//
		// THE STATIC VALUE IS THE ADDRESS ALONE (spec D3.2): the 19 00
		// token is a per-session observation (D5 entry 7, matrix lift R7),
		// recorded and never matched.
		CATID:    "7A",
		Transmit: spec.HasTransmitter,
		Banks: []spec.Bank{
			{
				ID:      spec.BankMemory,
				Label:   "Memories",
				Slots:   memSlots(),
				NoBlank: false, // matrix S1b - conservative, unevidenced either way
				Fields:  memFields,
			},
			{
				ID:      spec.BankScan,
				Label:   "Scan edges",
				Slots:   scanSlots(),
				NoBlank: false,
				Fields:  scanFields,
			},
		},
		// Matrix S1 row 6 - the ten printed mode codes, IDENTICAL to the
		// IC-7610's own set (that package already excludes WFM and DV;
		// this radio's "-WFM,-DV" delta from spec.md S1 lands on a set
		// that was already narrow). RULING OQ1 (core/civ/ic7610/doc.go)
		// applies directly to the printed 12/13 codes.
		Modes: []string{"LSB", "USB", "AM", "CW", "RTTY", "FM", "CW-R", "RTTY-R", "PSK", "PSK-R"},
		// Matrix S1 row 24 - IDENTICAL to the IC-7610's own filterEnum.
		Filters: []string{"FIL1", "FIL2", "FIL3"},
		// Matrix S1 row 21 - byte (11)'s LOW nibble. No DTCS value
		// anywhere in this document (this radio predates digital code
		// squelch) - IDENTICAL to the IC-7610's own toneModeEnum.
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS},
			{Value: "TSQL", Semantics: spec.ToneModeCTCSSSquelch},
		},
		// Matrix S1 row 12 - the 1B 00/1B 01 packed-BCD strip's own
		// digit range gives 1..2999 deciHz (0 excluded: spec.ToneRange
		// requires MinDeciHz > 0). The radio's actual usable CTCSS list,
		// narrower than this encoding bound, is NOT established by this
		// document (matrix lift R17, register entry
		// ic7600-ctcss-tone-list).
		CTCSSToneRange: &spec.ToneRange{MinDeciHz: 1, MaxDeciHz: MaxToneDeciHz, StepDeciHz: 1},
		// Matrix S1 row 13 - the five rates the front-panel item names for
		// the CI-V link. NARROWER THAN THE IC-7610's SIX: this document
		// offers no independent corroborating table, so completeness is
		// CE rather than the IC-7610's own ASSUMED (register entry
		// ic7600-civ-rate-list).
		Bauds: []int{300, 1200, 4800, 9600, 19200},
		// Matrix S1 row 14 / S3.3 - BETTER EVIDENCED THAN THE IC-7610 IN
		// ONE RESPECT: the front-panel item states the factory default
		// directly as the literal value "Auto," not a numbered rate. What
		// "Auto" resolves to on the wire for a given controller rate is
		// NOT stated (register entry ic7600-auto-baud-mechanics). This
		// driver still needs ONE concrete rate to open the port at
		// (internal/wiring cannot sweep), so 19200 is picked here as an
		// ARBITRARY CHOICE within the five named rates - the same posture
		// and the same safety net as the IC-7610's own DefaultBaud: the
		// probe requires an address-matched 19 00 reply, so a wrong guess
		// costs a clean timeout at Open and never a wrong byte.
		DefaultBaud: 19200,
		// Matrix S1 row 16 - see MaxEncodableFreqHz. MinFreqHz is in
		// deliberatelyZero.
		MaxFreqHz: MaxEncodableFreqHz,
		// Matrix S1 row 7 - the name span (18)~(27) is ten bytes wide.
		TagLen: 10,
		// core/civ/ic7600's own NameCharset - a CHOICE against a printed
		// contradiction on this radio's own page (matrix S1 TagCharset,
		// S3.9(ii); see core/civ/ic7600/profile.go's NameCharset comment
		// for the reasoning in full).
		TagCharset: civic7600.NameCharset,
	}
}

// capabilitiesUnverified is the REAL-HARDWARE profile while
// writeTrialsComplete is false: every mapped field Read Unverified and
// Write Unverified, so NOTHING is writable without recorded consent.
func capabilitiesUnverified() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	return baseCapabilities(bankFields(rw), bankFields(rw))
}

// capabilitiesSimulated is the in-package scripted-port-backed profile
// (CLI --fake, GUI demo) and NEVER a real radio: Read AND Write Supported
// for exactly the seven fields the 1A 00 record maps, on MEM and SCAN
// alike.
//
// EVERY WRITTEN-DOWN ZERO STAYS ZERO, simulator or not - INCLUDING
// scan_skip and data_mode. E6's unmapped regions are unmapped against the
// simulator too.
func capabilitiesSimulated() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	return baseCapabilities(bankFields(rw), bankFields(rw))
}
