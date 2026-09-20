// SPDX-License-Identifier: GPL-3.0-or-later

package ts570

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// Profile selects the evidence gate used by New{D,S,DG}. RealHardware is
// the zero value, shared with every other driver package.

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// writeTrialsComplete is FALSE for every row. No TS-570 of any suffix has
// ever been asked anything by this project (matrix §0).
const writeTrialsComplete = false

// memBankLabel is this row's one bank's display label — a CHOICE, not a
// protocol fact, matching every sibling package's own "Memories".
const memBankLabel = "Memories"

// slotProbeCeiling bounds the walk deriving the bank inventory from the
// layout. P2Unused carries no hundreds digit (core/kw/ts570), so 99 is the
// widest channel number the grid can address at all.
const slotProbeCeiling = 99

// slotID renders a channel number as this row's canonical slot identifier:
// two digits, core/driver/ts480's own convention and for the identical
// reason — P2Unused means this row's channel number is P3's two digits
// alone, with no hundreds digit to print a third with.
func slotID(number int) string { return fmt.Sprintf("%02d", number) }

// memSlots returns this row's MEM inventory, "00".."99", built through the
// layout's own NewSlot so the numbers this capability data advertises are
// the ones the row's slot space actually accepts.
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

// modeNames returns the selectable mode display names this row advertises,
// derived from the layout rather than transcribed here — core/kw/ts570's
// own eight-entry legend (matrix §1.3), zero new vocabulary.
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

// ts570CTCSSTones is this row's own 39-entry subtone chart, transcribed
// directly from the manual's own table (layout lines 2179-2190, printed
// page 25) rather than borrowed from core/driver/ts590's 43-entry chart —
// see doc.go for the three conventional tones (69.3, 206.5, 229.1 Hz) this
// document's table omits that the 590/480 chart carries.
//
// INDEX i OF THIS SLICE IS WIRE TONE NUMBER i+1: the printed domain is
// "01~39" (matrix §1.2, Format 14), not "00~39" — there is no tone number
// 00 on this row at all, unlike every registered Kenwood row, whose TN/CN
// charts both start at 00. tone() below is where that one-based offset is
// applied, once.
//
// The 39th entry, 1750.0 Hz, is the European FM-repeater burst tone the
// same manual page calls out separately (matrix §2, register
// ts570-tone39-burst-not-ctcss) — carried here as an ordinary 39th chart
// entry, the technically-accurate-to-the-wire reading the matrix records
// without resolving.
var ts570CTCSSTones = []spec.Tone{
	670, 719, 744, 770, 797, 825, 854, 885, 915, 948,
	974, 1000, 1035, 1072, 1109, 1148, 1188, 1230, 1273, 1318,
	1365, 1413, 1462, 1514, 1567, 1622, 1679, 1738, 1799, 1862,
	1928, 2035, 2107, 2181, 2257, 2336, 2418, 2503,
	17500,
}

// tone maps P8's wire index (1-39) onto this row's own chart. index 0, or
// anything past 39, is refused rather than clamped — this row's book
// prints no meaning for either.
func tone(index int) (spec.Tone, error) {
	if index < 1 || index > len(ts570CTCSSTones) {
		return 0, fmt.Errorf("tone index %d is outside this row's printed 01~39 domain", index)
	}
	return ts570CTCSSTones[index-1], nil
}

// toneWireIndex is tone's inverse: the wire index for value, if this row's
// chart carries it.
func toneWireIndex(value spec.Tone) (int, bool) {
	for i, t := range ts570CTCSSTones {
		if t == value {
			return i + 1, true
		}
	}
	return 0, false
}

// toneModeNames maps the record's P7 wire byte to the neutral tone_mode
// vocabulary this row publishes. TWO VALUES ONLY (ToneModesTwo,
// core/kw/ts570): this row's byte 20 is "0: OFF / 1: ON" (matrix §1.2),
// narrower than every other registered Kenwood row's three or four.
var toneModeNames = map[kw.ToneMode]string{
	kw.ToneModeOff:  "OFF",
	kw.ToneModeTone: "ON",
}

// toneModeWire is toneModeNames read backwards, derived rather than
// transcribed a second time (core/driver/ts590's own toneModeWire shape).
var toneModeWire = invertToneModes()

func invertToneModes() map[string]kw.ToneMode {
	m := make(map[string]kw.ToneMode, len(toneModeNames))
	for wire, name := range toneModeNames {
		m[name] = wire
	}
	return m
}

// deliberatelyZero is the audit table TestDeliberatelyZeroAudit reflects
// over: every spec.Capabilities field this driver leaves at its zero
// value, with the reason. Every row shares one table — the three rows
// differ only in Model and CATID, never in which fields are graded.
var deliberatelyZero = map[string]string{
	"TagLen":                 "matrix §4: NoTag — this row has no channel-name route over CAT at all (no name byte exists past this row's 28-byte record, positions 28-49 of the family grid simply do not exist here), so TagLen is 0 by declaration, not omission (core/spec/validate.go's NoTag pairing rule, merge d0b2498, 12/09/2026)",
	"TagCharset":             "NoTag: this row has no channel-name route over CAT at all, so there is no charset to declare (matrix §4)",
	"ClarMaxHz":              "matrix §2: the 28-byte record has no per-channel clarifier field; this radio has RIT/XIT as radio-level settings (RC, RD, RU, IS, XT) no memory channel stores",
	"ClarStepHz":             "matrix §2: the same — no clarifier field, no step",
	"CTCSSToneRange":         "matrix §2: the tone field is an INDEX into this row's own chart (Format 14), not a BCD frequency number, so the Icom-style range type does not apply — a CHOICE over the field's shape, not an omission",
	"RequiredSlots":          "matrix §2: nothing in the command table or the memory-operations chapter states a channel that must never be empty",
	"ShiftOptions":           "matrix §2: superseded by the Icom-vocabulary pair this row publishes instead (ToneModes) — no shift/duplex/offset byte exists in the 28-byte record",
	"DuplexOptions":          "matrix §2: no shift/duplex/offset byte anywhere in the 28-byte record",
	"DTCSPolarities":         "matrix §2: no DCS command, no DCS parameter format, no DCS mention anywhere in the extraction — this radio predates DCS",
	"DTCSCodes":              "matrix §2: the same — no DCS code table is printed anywhere in this document",
	"Filters":                "matrix §2: FW is a LIVE radio-level command (Format 38), not a memory-record field; the 28-byte MR/MW frame carries no filter byte at all",
	"MinFreqHz":              "matrix §2: zero is P4's own encoding floor (11 BCD digits, \"represented in Hz\") rather than an omission; the storable floor is not established by this document",
	"SimplexTx":              "this row does not grade FieldTxFrequency (no split byte exists in the 28-byte record at all, matrix §2), so the blank arm leaves nothing to state — core/driver/ts480's own reasoning",
	"TuningSteps":            "additions design D8: no D8 receiver field applies to any of the v1.7.0 wave's seven transceivers",
	"ProgramTuningStepRange": "additions design D8: as above",
	"AttenuatorDB":           "additions design D8: as above",
	"PreampOptions":          "additions design D8: as above",
	"AntennaOptions":         "additions design D8: as above",
}

// bankFields returns the MEM bank's field map: rw for every field this
// row's 28-byte record maps, the zero FieldSupport everywhere else. EVERY
// spec.Field is listed explicitly (matrix §3), including the zeroes.
func bankFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	return map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency: rw, // P4, 11 digits at positions 7-17 (matrix §1.2)
		spec.FieldMode:      rw, // P5 at position 18, MD's ten nibbles reused whole (matrix §1.3)
		// P6 at position 19: the CHANNEL LOCKOUT (matrix §1.2, Format 10),
		// the TS-480's reading, not the 590 pair's data mode.
		spec.FieldScanSkip: rw,
		// MW's own note documents a genuine vacate-on-all-zero-frequency
		// erase route (matrix §3, doc.go) — stronger evidence than either
		// registered row's. It stays Unsupported anyway: core/kw.BuildMWSet
		// flatly refuses any record whose FreqHz is zero (decision 8,
		// shared across the whole Kenwood codec), so this row's driver has
		// no frame this shared builder will construct to exercise it. See
		// doc.go's "Erase is documented, and still not built".
		spec.FieldErase: {},
		// P7 at position 20: ToneModesTwo, OFF/ON only (matrix §1.2).
		spec.FieldToneMode: rw,
		// P8 at positions 21-22: ONE index serving both directions
		// (matrix §2, "both point at the SAME P8 byte"). Both graded rw;
		// write.go refuses a request naming two different tones.
		spec.FieldToneTx: rw,
		spec.FieldToneRx: rw,

		// MANUAL-EVIDENCED ABSENCE (matrix §2, §3): no byte past position
		// 27 exists in this row's 28-byte record at all.
		spec.FieldClarifier:    {},
		spec.FieldCTCSSState:   {},
		spec.FieldCTCSSTone:    {},
		spec.FieldShift:        {},
		spec.FieldTxFrequency:  {},
		spec.FieldDuplex:       {},
		spec.FieldOffset:       {},
		spec.FieldDTCSCode:     {},
		spec.FieldDTCSPolarity: {},
		spec.FieldFilter:       {},
		// Byte 19 is the LOCKOUT here (published above as scan_skip);
		// there is no data-mode position anywhere in this record.
		spec.FieldDataMode: {},

		// NOTAG (matrix §4): no name route over CAT at all.
		spec.FieldTag:        {},
		spec.FieldTagDisplay: {},

		// Additions design D8 — no D8 receiver field applies to this
		// wave's seven transceivers.
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
// evidence grade. Every spec.Capabilities field is populated explicitly.
func baseCapabilities(m modelParams, rw spec.FieldSupport) spec.Capabilities {
	l := m.layout
	return spec.Capabilities{
		Model:    m.name,
		CATID:    m.catID,
		Transmit: spec.HasTransmitter, // matrix §2: an HF/6m transceiver with a documented transmit surface
		Banks: []spec.Bank{{
			ID: spec.BankMemory, Label: memBankLabel, Slots: memSlots(l),
			// NoBlank false: this book states nothing about empty
			// channels being disallowed (matrix §2, RequiredSlots).
			NoBlank: false,
			Fields:  bankFields(rw),
		}},
		Modes: modeNames(l),
		// NoTag (matrix §4): see deliberatelyZero.
		TagLen: 0,
		NoTag:  true,
		// Seven rates, all three rows (matrix §2, Menu 35's own COM table).
		Bauds: []int{1200, 2400, 4800, 9600, 19200, 38400, 57600},
		// ASSUMED per project convention, though the VALUE is manual-stated:
		// "The defaults are 9600 bps and 1 stop bit." (matrix §2, PDF p.57
		// printed 51) — a stronger citation than either registered
		// sibling's undocumented default.
		DefaultBaud: 9600,
		// Encoding bound only (matrix §2): P4 is 11 BCD digits.
		MinFreqHz: 0,
		MaxFreqHz: 99_999_999_999,
		// This row's own 39-entry chart (doc.go); NOT ts590's 43-entry one.
		CTCSSTones: append([]spec.Tone(nil), ts570CTCSSTones...),
		// One entry beyond OFF (ToneModesTwo, matrix §1.2: byte 20 is
		// "0: OFF / 1: ON" only). The SEMANTICS are ToneModeCTCSSSquelch,
		// not plain ToneModeCTCSS: this row's single P8 index both
		// superimposes the tone on transmit AND gates receive squelch on
		// the SAME tone (matrix §2, FieldToneTx/FieldToneRx note — "a
		// subtone that you select is superimposed on your transmit
		// signal" and "the squelch in your transceiver opens only when
		// the selected subtone is received", one selection, both
		// purposes) — CHIRP's "TSQL", not its "Tone". The wire-form
		// label "ON" is this driver's own vocabulary CHOICE (matrix §2:
		// "the UI calls the ON state 'TONE' or 'CTCSS ENCDEC' is a
		// CHOICE"); the wire fact — one bit — is fixed.
		ToneModes: []spec.ToneMode{
			{Value: "OFF", Semantics: spec.ToneModeOff},
			{Value: "ON", Semantics: spec.ToneModeCTCSSSquelch},
		},
	}
}

func capabilitiesUnverified(m modelParams) spec.Capabilities {
	return baseCapabilities(m, spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified})
}

func capabilitiesSimulated(m modelParams) spec.Capabilities {
	return baseCapabilities(m, spec.FieldSupport{Read: spec.Supported, Write: spec.Supported})
}
