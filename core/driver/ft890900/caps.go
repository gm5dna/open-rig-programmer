// SPDX-License-Identifier: GPL-3.0-or-later

package ft890900

import (
	"github.com/gm5dna/open-rig-programmer/core/bincat"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// writeTrialsComplete is FALSE for both radios: NO FT-890 OR FT-900 HAS
// EVER BEEN ASKED ANYTHING BY THIS PROJECT, AND NEITHER IS AVAILABLE TO
// IT (capability matrices' own header). There is no
// CapabilitiesRealHardware profile for either — RealHardware selects the
// all-Unverified baseline unconditionally (ft890900.go).
const writeTrialsComplete = false

const memBankLabel = "Memories"

// standardTonesArray backs standardTones below — StandardCTCSSTones
// returns an array value, which must be addressable before it can be
// sliced.
var standardTonesArray = spec.StandardCTCSSTones()

// standardTones is the 33-entry prefix of the standard 50-tone chart both
// radios' tone byte addresses (capability matrices §1.6, `CC` 0-0x20 =
// 0-32 decimal, 33 values) — ASSUMED beyond the endpoints each matrix
// independently spot-checked; see each matrix's own owner-probe note.
var standardTones = standardTonesArray[:33]

// modelInfo carries what genuinely differs between FT-890 and FT-900 for
// capability-building purposes: everything else (frame shape, record
// layout, opcodes, tone table, clarifier bound, baud) is identical
// (capability matrices, throughout).
type modelInfo struct {
	Name          string
	CATID         string
	TrueSlotCount int
	EngineProfile bincat.Profile
	// OffsetMaxHz is the repeater-offset magnitude ceiling opcode F9H
	// documents for this model (200,000 Hz FT-890, 500,000 Hz FT-900 —
	// matrices §2's "Shift representation" note). Defence-in-depth only
	// (write.go): bincat's own AllowedCommand gate for OpOffset is
	// narrower than FT-900's documented ceiling (core/bincat/profile.go's
	// AllowedCommand caps the leading BCD byte at 2), so a FT-900 write
	// above roughly 299,999 Hz is refused at the wire gate regardless of
	// what this driver permits — a Phase 1 limitation this package does
	// not attempt to work around.
	OffsetMaxHz uint64
	// AnswersBoundaryProbe is whether THIS model answers Open's second
	// identity probe, CH=33 (spec.md §Identity probe): false for FT-890
	// (channel 33 is out of its documented 1-32 range), true for FT-900
	// (in range, 1-100).
	AnswersBoundaryProbe bool
}

var (
	ft890Info = modelInfo{Name: "FT-890", CATID: ft890CATID, TrueSlotCount: ft890TrueSlotCount, EngineProfile: ft890EngineProfile, OffsetMaxHz: 200_000, AnswersBoundaryProbe: false}
	ft900Info = modelInfo{Name: "FT-900", CATID: ft900CATID, TrueSlotCount: ft900TrueSlotCount, EngineProfile: ft900EngineProfile, OffsetMaxHz: 500_000, AnswersBoundaryProbe: true}
)

// otherInfo returns the sibling model's info — used to name the actual
// radio a failed identity probe's evidence points to (ft890900.go's
// probeIdentity).
func otherInfo(m modelInfo) modelInfo {
	if m.Name == ft890Info.Name {
		return ft900Info
	}
	return ft890Info
}

// memSlots returns "001"..the model's true top channel, zero-padded to 3
// digits — the canonical wire-form slot inventory (capability matrices
// §1.9/"Banks").
func (m modelInfo) memSlots() []string {
	return spec.NumberedSlots(slotBase, m.TrueSlotCount, "%03d")
}

// memFields builds the MEM bank's per-field support map. Every
// spec.Field is listed explicitly, following core/driver/ic7200/caps.go's
// own audit-map convention.
//
//   - rw covers the four plain fields the record and choreography both
//     carry: frequency, mode, shift and the repeater-offset magnitude
//     (matrices §2, §"Shift representation").
//   - CTCSSTone is rw too (record offset 7, opcode 90H — matrices §1.6),
//     graded independently of the (unmapped) CTCSS on/off state below.
//   - Clarifier is READ rw, WRITE UNSUPPORTED: opcode 09H's argument byte
//     layout could not be recovered from either manual
//     (core/bincat/profile.go's OpClarifier doc comment) — this driver
//     can transmit only the documented clarifier-OFF pattern, never a
//     caller-requested value (write.go), so FieldClarifier is not
//     genuinely writable.
//   - CTCSSState is the zero FieldSupport: neither manual documents a
//     CTCSS on/off toggle distinct from the tone-code byte itself
//     (matrices, "CTCSSStates: OPEN").
//   - Tag/TagDisplay are the zero FieldSupport: NoTag (matrices §0) — no
//     TAG/NAME opcode exists in either radio's command table.
//   - ScanSkip is the zero FieldSupport: the capability matrices document
//     a scan-skip bit (Operating Flags bit 2), but core/bincat.Record —
//     the codec this driver consumes as given — does not decode it, so
//     this driver has no access to it at all.
//   - Erase is the zero FieldSupport: no per-channel clear/erase opcode
//     is documented for either radio.
//   - The seventeen Icom-tier fields are the zero FieldSupport: this is a
//     Yaesu-family record.
//
// Each call returns a fresh map.
func memFields(rw spec.FieldSupport) map[spec.Field]spec.FieldSupport {
	clarSupport := spec.FieldSupport{Read: rw.Read, Write: spec.Unsupported}
	// offsetSupport: WRITE-ONLY, the same shape core/driver/ft710/caps.go
	// gives FieldErase. Opcode F9H sets the repeater-offset magnitude, but
	// the 9-byte VFO/Memory Data Record's own fields stop at offset 8
	// (Operating Flags) — no byte anywhere carries the magnitude back, so
	// ReadChannel can never recover it (read.go returns Unavailable).
	offsetSupport := spec.FieldSupport{Read: spec.Unsupported, Write: rw.Write}
	return map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency: rw,
		spec.FieldMode:      rw,
		spec.FieldShift:     rw,
		spec.FieldOffset:    offsetSupport,
		spec.FieldCTCSSTone: rw,
		spec.FieldClarifier: clarSupport,

		spec.FieldCTCSSState: {},
		spec.FieldTag:        {},
		spec.FieldTagDisplay: {},
		spec.FieldScanSkip:   {},
		spec.FieldErase:      {},

		spec.FieldTxFrequency:       {},
		spec.FieldDuplex:            {},
		spec.FieldToneMode:          {},
		spec.FieldToneTx:            {},
		spec.FieldToneRx:            {},
		spec.FieldDTCSCode:          {},
		spec.FieldDTCSPolarity:      {},
		spec.FieldFilter:            {},
		spec.FieldDataMode:          {},
		spec.FieldTuningStepEnabled: {},
		spec.FieldTuningStep:        {},
		spec.FieldProgramTuningStep: {},
		spec.FieldAttenuator:        {},
		spec.FieldPreamp:            {},
		spec.FieldAntenna:           {},
		spec.FieldIPPlus:            {},
	}
}

// baseCapabilities assembles m's capability set with the given per-field
// support.
func (m modelInfo) baseCapabilities(rw spec.FieldSupport) spec.Capabilities {
	return spec.Capabilities{
		Model:    m.Name,
		CATID:    m.CATID,
		Transmit: spec.HasTransmitter, // documented PTT opcode 0Fh, matrices §4.
		Banks: []spec.Bank{
			{ID: spec.BankMemory, Label: memBankLabel, Slots: m.memSlots(), NoBlank: false, Fields: memFields(rw)},
		},
		Modes: []string{"LSB", "USB", "CW", "AM", "FM"},
		// NoTag (matrices §0): no CAT route carries tag TEXT.
		TagLen: 0,
		NoTag:  true,
		// ClarMaxHz/ClarStepHz: matrices §4, identical on both radios —
		// the record's own 2's-complement clarifier field, -999 to +999
		// Hz, 1 Hz resolution.
		ClarMaxHz:  999,
		ClarStepHz: 1,
		CTCSSTones: standardTones[:],
		Bauds:      []int{4800},
		// DefaultBaud: the only rate either manual documents (matrices
		// §1.2).
		DefaultBaud: 4800,
		// MinFreqHz/MaxFreqHz: left OPEN (matrices §4) — the record's
		// wire-encoding range (100,000-30,000,000 Hz) is not necessarily
		// the radio's documented tuning range; see write.go's own
		// wireFreqMin/wireFreqMax defence-in-depth check, which enforces
		// the wire encoding's own ceiling without claiming it as this
		// radio's tuning range.
		ShiftOptions: spec.StandardShiftOptions(),
	}
}

// CapabilitiesUnverified returns m's real-hardware baseline: every mapped
// MEM field Unverified (writeTrialsComplete is false).
func (m modelInfo) CapabilitiesUnverified() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	return m.baseCapabilities(rw)
}

// CapabilitiesSimulated returns m's fake-radio-backed profile (CLI
// --fake, GUI demo), never a real radio: every mapped MEM field Supported
// both ways.
func (m modelInfo) CapabilitiesSimulated() spec.Capabilities {
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	return m.baseCapabilities(rw)
}
