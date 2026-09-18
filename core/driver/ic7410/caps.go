// SPDX-License-Identifier: GPL-3.0-or-later

package ic7410

import (
	civic7410 "github.com/gm5dna/open-rig-programmer/core/civ/ic7410"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// writeTrialsComplete stays FALSE until a real IC-7410 has confirmed a
// write. No IC-7410 has ever been asked anything by this project (matrix
// §0, §3.14): every byte in this package's tables came from the IC-7410
// Instruction Manual through the reviewed capability matrix, and from
// nothing else.
const writeTrialsComplete = false

// Profile selects the static capability arm used by the driver. The zero
// value is RealHardware on purpose: a forgotten or zero-valued Profile
// must fail TOWARDS the real-hardware capability set, which while
// writeTrialsComplete is false is the all-Unverified one.
type Profile = driver.Profile

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// The numeric bounds this driver declares and, at WriteChannel's domain
// rung, enforces.
const (
	// MinRadioFreqHz/MaxRadioFreqHz are the receiver's printed coverage,
	// matrix §1 rows 15-16: "Receive 0.030-60.000 MHz". The RADIO's
	// ceiling is declared, not the wider field capacity (99.999999 MHz —
	// this model's 10 MHz digit is printed 0-9, unlike the IC-7610
	// family's 0-6): declaring the field width would authorise, after
	// consent, a frequency this radio's receiver cannot cover.
	MinRadioFreqHz = 30_000
	MaxRadioFreqHz = 60_000_000
	// MinToneDeciHz/MaxToneDeciHz/ToneStepDeciHz are the tone spans' own
	// wire capacity in tenths of a hertz, matrix §1 row 9/12: "four rotated
	// digit labels (100 Hz, 10 Hz, 1 Hz, 0.1 Hz digit)" — the SAME 4-digit
	// domain as every 7610-family sibling. The tier's recorded doctrine
	// (IC-7300 matrix erratum 12, repeated at IC-7100/IC-7851 caps.go):
	// declare the wire's own capacity, not the printed selectable-tone
	// chart, because the record indexes no table. The floor is 1, not the
	// printed 0, because 0 Hz is not a tone and spec.ToneRange requires
	// MinDeciHz > 0.
	MinToneDeciHz  = 1
	MaxToneDeciHz  = 2999
	ToneStepDeciHz = 1
)

// deliberatelyZero is the spec.Capabilities struct-field audit: every
// field this model leaves at its zero value, with the matrix reading that
// says so. TestCapabilities_EveryFieldExplicit requires the two sets —
// this map and every non-zero field in baseCapabilities — to partition the
// struct's 29 fields exactly.
var deliberatelyZero = map[string]string{
	"ClarMaxHz":              "the 40-byte record has no clarifier field (matrix §1 row 9, poor fit, MANUAL-EVIDENCED absence)",
	"ClarStepHz":             "the same (matrix §1 row 10)",
	"ShiftOptions":           "this model's per-channel flag is boolean Split, not a shift-magnitude vocabulary; superseded by tx_frequency + SimplexTx (matrix §1 row 18)",
	"DuplexOptions":          "Split is boolean ON/OFF with no printed direction, which does not fit DuplexOption's {Value, Direction} shape at all (matrix §1 row 20)",
	"DTCSCodes":              "\"DTCS\" and \"DCS\" occur zero times in the 124-page manual; the tone-type nibble stops at 2: TSQL (matrix §1 row 22)",
	"DTCSPolarities":         "the same sweep, same reasoning (matrix §1 row 23)",
	"TuningSteps":            "additions design D8 — the 1A 00 record carries no receiver tuning-step field",
	"ProgramTuningStepRange": "additions design D8 — no programmable tuning-step field",
	"AttenuatorDB":           "additions design D8 — no attenuator field",
	"PreampOptions":          "additions design D8 — no preamp field",
	"AntennaOptions":         "additions design D8 — no antenna field",
	"RequiredSlots":          "nothing in this document is declared never-empty (matrix §1 row 17)",
	"CTCSSTones":             "the tone domain is CTCSSToneRange, not a chart index; the wire carries a BCD frequency (matrix §1 row 11)",
	"NoTag":                  "the IC-7410 supports channel names via the 9-byte name field (matrix §1 rows 7-8); NoTag is false",
}

// memSlots is the MEM bank's inventory: "0001".."0099" (matrix §1b "The
// banks").
func memSlots() []string { return spec.NumberedSlots(1, 99, "%04d") }

// scanSlots is the SCAN bank's inventory: the two scan edges P1 and P2,
// which are two more values of the same two-byte selector, not a
// protocol-level bank (matrix §1b, "The banks").
func scanSlots() []string { return []string{"P1", "P2"} }

// memFields is the MEM bank's field grid. Matrix §2 MEM table: nine
// fields Sup/Sup, everything else the zero FieldSupport.
//
// tx_frequency IS MAPPED, unlike every other radio in this wave sharing
// the IC-7610 family's shape: the headline finding (spec.md's ruling 7) is that
// this model's 40-byte record carries a genuine TX-duplicate block whose
// first five bytes are an independent transmit frequency. data_mode is
// ALSO mapped, unlike the IC-7610 family's nibble-shared four-valued
// version: this model's ⑪ is a genuine whole-byte two-valued boolean
// (matrix §1b, offset 8 row).
func memFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:   rw, // ④~⑧
		spec.FieldMode:        rw, // ⑨
		spec.FieldFilter:      rw, // ⑩
		spec.FieldDataMode:    rw, // ⑪, whole byte
		spec.FieldToneMode:    rw, // ⑫ high nibble
		spec.FieldToneTx:      rw, // ⑬~⑮
		spec.FieldToneRx:      rw, // ⑯~⑱
		spec.FieldTxFrequency: rw, // TX-duplicate block's frequency span
		spec.FieldTag:         rw, // ⑲~㉗, 9 bytes

		// MANUAL-EVIDENCED ABSENCE: no such span in this record.
		spec.FieldClarifier:    {},
		spec.FieldCTCSSState:   {},
		spec.FieldCTCSSTone:    {},
		spec.FieldShift:        {},
		spec.FieldTagDisplay:   {},
		spec.FieldDuplex:       {},
		spec.FieldOffset:       {},
		spec.FieldDTCSCode:     {},
		spec.FieldDTCSPolarity: {},

		// scan_skip: matrix §2 MEM row 9 — unlike the IC-7610 family, byte
		// ③ on this model is NOT a four-valued select-scan-group marker;
		// its two nibbles are two-valued Select-memory OFF/ON and Split
		// OFF/ON, neither of which is a scan-skip vocabulary of any shape.
		// There is no scan-skip byte in this record at all.
		spec.FieldScanSkip: {},

		// erase: the tier-wide Icom rule — every Icom driver gives
		// FieldErase zero FieldSupport, matrix §2 MEM row 10.
		spec.FieldErase: {},
	}
}

// scanFields is the SCAN bank's field grid. Matrix §2 SCAN table:
// identical to MEM except tx_frequency, whose WRITE is Uns — byte ③'s
// Split nibble must read 0 on P1/P2 (matrix §1b "The banks"), so nothing
// enables the duplicate block's "matching transmit settings" role on a
// scan edge; a write path letting a caller set tx_frequency there would
// write a value the record's own constraint note says is not meaningful
// on that slot.
func scanFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	f := memFields(rw)
	f[spec.FieldTxFrequency] = spec.FieldSupport{Read: rw.Read, Write: spec.Unsupported}
	return f
}

// baseCapabilities assembles the static baseline both profiles share.
func baseCapabilities(memF, scanF map[spec.Field]spec.FieldSupport) spec.Capabilities {
	return spec.Capabilities{
		// Matrix §1 row 1. PDF p.1 cover: "HF/50 MHz TRANSCEIVER"; PDF p.96
		// "The IC-7410's address is 80h."
		Model: "IC-7410",
		// Matrix §1 row 2 / §3.4. PDF p.96, Set-mode item 43: "CI-V Address
		// (Default: 80h)."
		CATID:    "80",
		Transmit: spec.HasTransmitter, // Matrix §1 row 3.
		// spec.md's ruling 7: map the TX frequency span to
		// spec.FieldTxFrequency with a SimplexTx statement. This model's
		// Split flag (byte ③ low nibble) has no neutral FieldDuplex home
		// (deliberatelyZero, "DuplexOptions"), so from spec.Capabilities'
		// own perspective this record carries no split flag at all —
		// EXACTLY the reading IC-7300's own SimplexTx declaration rests on
		// (core/driver/ic7300/caps.go). The manual's own text that the
		// duplicate block is "still necessary" even when Split is OFF is
		// the reading that a channel this record calls simplex transmits
		// where it receives; the write path (write.go) mirrors FreqHz into
		// TxFreqHz whenever a caller has not set one, rather than refusing
		// the write (a deliberate deviation from IC-7300's "REV 1 struck"
		// rule, ruled explicitly for this model at spec.md's ruling 7).
		SimplexTx: spec.SimplexTxEqualsRx,
		Banks: []spec.Bank{
			{
				ID: spec.BankMemory, Label: "Memories", Slots: memSlots(),
				// FALSE: this document says nothing about whether a memory
				// channel must stay populated (matrix §1b bank descriptors).
				NoBlank: false, Fields: memF,
			},
			{
				ID: spec.BankScan, Label: "Scan edges", Slots: scanSlots(),
				// FALSE, same reasoning — nothing says a scan edge must
				// stay populated (matrix §1b bank descriptors, "Bank.NoBlank
				// — SCAN").
				NoBlank: false, Fields: scanF,
			},
		},
		// Matrix §1 row 6: eight printed pairs, no WFM, no DV, no PSK — the
		// eight values of core/civ/ic7410's mode enum.
		Modes:  []string{"LSB", "USB", "AM", "CW", "RTTY", "FM", "CW-R", "RTTY-R"},
		TagLen: 9, // Matrix §1 row 7 / §3.9(i): "9 characters (fixed)".

		ClarMaxHz:  0,   // deliberatelyZero.
		ClarStepHz: 0,   // deliberatelyZero.
		CTCSSTones: nil, // deliberatelyZero.
		// Matrix §1 row 12, and MinToneDeciHz/MaxToneDeciHz's doc comment
		// for why the wire capacity is declared rather than a chart.
		CTCSSToneRange: &spec.ToneRange{MinDeciHz: MinToneDeciHz, MaxDeciHz: MaxToneDeciHz, StepDeciHz: ToneStepDeciHz},
		// Matrix §1 row 13: a genuinely stronger citation than the IC-7610
		// family's — a printed, closed set. PDF p.94, item 42.
		Bauds: []int{300, 1200, 4800, 9600, 19200},
		// Matrix §1 row 14: the factory default is documented as auto-baud
		// ("Auto"), which is stronger evidence than the IC-7610 family had,
		// but the NUMERIC rate a driver opens at is still a CHOICE among
		// the five listed rates — any should succeed against a
		// factory-default radio, which auto-detects the host's rate.
		// 19200 is the arbitrary highest, mirroring the tier's convention
		// on this exact "Auto"-only evidence shape (core/driver/ic7100,
		// core/driver/ic7851).
		DefaultBaud: 19_200,
		MinFreqHz:   MinRadioFreqHz, // Matrix §1 row 15.
		MaxFreqHz:   MaxRadioFreqHz, // Matrix §1 row 16.

		RequiredSlots: nil, // deliberatelyZero.
		ShiftOptions:  nil, // deliberatelyZero.
		DuplexOptions: nil, // deliberatelyZero.
		// Matrix §1 row 21: the three-value vocabulary, byte ⑫'s high
		// nibble. ToneModeCTCSS/ToneModeCTCSSSquelch is this project's
		// existing convention for an identical OFF/TONE/TSQL triple.
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "TONE", Semantics: spec.ToneModeCTCSS},
			{Value: "TSQL", Semantics: spec.ToneModeCTCSSSquelch},
		},
		DTCSPolarities: nil,                              // deliberatelyZero.
		DTCSCodes:      nil,                              // deliberatelyZero.
		Filters:        []string{"FIL1", "FIL2", "FIL3"}, // Matrix §1 row 24.

		TuningSteps:            nil, // deliberatelyZero (D8).
		ProgramTuningStepRange: nil, // deliberatelyZero (D8).
		AttenuatorDB:           nil, // deliberatelyZero (D8).
		PreampOptions:          nil, // deliberatelyZero (D8).
		AntennaOptions:         nil, // deliberatelyZero (D8).
		// Matrix §1 row 25: the printed table, transcribed once in
		// core/civ/ic7410 and referenced here.
		TagCharset: civic7410.NameCharset,
	}
}

// capabilitiesUnverified is the REAL-HARDWARE profile while
// writeTrialsComplete is false: every mapped field Read Unverified and
// Write Unverified.
func capabilitiesUnverified() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	return baseCapabilities(memFields(rw), scanFields(rw))
}

// capabilitiesSimulated is the in-package-scripted-port-backed profile
// (CLI --fake, GUI demo) and never a real radio: Read AND Write Supported
// for every mapped field.
func capabilitiesSimulated() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	return baseCapabilities(memFields(rw), scanFields(rw))
}
