// SPDX-License-Identifier: GPL-3.0-or-later

package ft1000mp

import (
	"errors"

	"github.com/gm5dna/open-rig-programmer/core/bincat"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// modelName is this driver's display name — one row for both bodies, per
// the matrix's header note. internal/wiring's registration (Phase 5) is
// what will eventually pair this with the "FT-1000MP" slug.
const modelName = "FT-1000MP"

// catID is this radio's fixed, DISPLAY-ONLY synthetic CATID (matrix §4
// "CATID"): the family has no wire CAT-ID byte at all, so nothing here is
// wire-derived — identity is proved on the wire entirely by the FAH probe
// (ft1000mp.go's probeIdentity), never by this string. It exists only to satisfy
// spec.Capabilities.Validate's non-empty-CATID invariant and the
// maker-wide registration tests.
const catID = "1000"

// errBadSlot is this package's own slot-parsing sentinel.
var errBadSlot = errors.New("ft1000mp: not a recognised slot")

// The 12-value mode legend the 0CH SetMode opcode argument uses (matrix
// §1.5, ft1000mpmarkv_manual layout:5062-5070, printed p.94's Opcode
// Command Chart (2)) — transcribed in full, in wire order, rather than
// re-deriving it at each call site.
const (
	modeLSB    byte = 0x00
	modeUSB    byte = 0x01
	modeCW     byte = 0x02
	modeCWR    byte = 0x03
	modeAM     byte = 0x04
	modeAMSync byte = 0x05
	modeFM     byte = 0x06
	modeFMW    byte = 0x07
	modeRTTYL  byte = 0x08
	modeRTTYU  byte = 0x09
	modePKTL   byte = 0x0A
	modePKTF   byte = 0x0B
)

// modeNames maps every 0CH wire code to its display name, in wire order —
// this driver's Capabilities().Modes and every SetMode frame this driver
// builds go through this one table.
var modeNames = map[byte]string{
	modeLSB:    "LSB",
	modeUSB:    "USB",
	modeCW:     "CW",
	modeCWR:    "CW-R",
	modeAM:     "AM",
	modeAMSync: "AM-SYNC",
	modeFM:     "FM",
	modeFMW:    "FM-W",
	modeRTTYL:  "RTTY-L",
	modeRTTYU:  "RTTY-U",
	modePKTL:   "PKT-L",
	modePKTF:   "PKT-F",
}

// modeNamesOrdered is modeNames rendered as the ordered list
// Capabilities.Modes wants, in ascending wire-code order.
func modeNamesOrdered() []string {
	order := []byte{modeLSB, modeUSB, modeCW, modeCWR, modeAM, modeAMSync, modeFM, modeFMW, modeRTTYL, modeRTTYU, modePKTL, modePKTF}
	names := make([]string, len(order))
	for i, b := range order {
		names[i] = modeNames[b]
	}
	return names
}

// modeByName is modeNames inverted, for building a SetMode frame from a
// codeplug.ChannelData.Mode string.
var modeByName = func() map[string]byte {
	m := make(map[string]byte, len(modeNames))
	for b, name := range modeNames {
		m[name] = b
	}
	return m
}()

// recordModeBase maps the 16-byte record's own 3-bit Operating Mode code
// (byte 7, bits 5-7 — ft1000mpmarkv_manual layout:4929-4946, printed
// p.90-91's "Operating Mode Byte (7)": "Bit0* User Mode flag, Bits1-4
// dummy, Bits5-7 Mode Data (3-bit code): LSB=000 USB=001 CW=010 AM=011
// FM=100 RTTY=101 PKT=110") onto the 0CH opcode's own BASE member of
// each mode family (modeNames, above).
//
// THE RECORD CANNOT CARRY THE VARIANT BIT the 0CH opcode's 12-value
// legend distinguishes (CW/CW-R, AM/AM-SYNC, FM/FM-W, RTTY-L/RTTY-U,
// PKT-L/PKT-F): its 3-bit field has only 7 values for the opcode's 12.
// This is not a guess at the mapping — it falls out of the opcode
// table's OWN structure: LSB(00H) and USB(01H) are unpaired singles,
// then every subsequent pair of adjacent codes (02H/03H, 04H/05H,
// 06H/07H, 08H/09H, 0AH/0BH) shares one base family, so integer-dividing
// a code >= 2 by two after subtracting 2, plus 2, recovers exactly the
// 3-bit family index the record's compact field independently states.
// Verified by construction (recordModeBase's own value for every 0CH
// code equals this formula) rather than assumed — see
// caps_test.go's TestRecordModeBase_MatchesOpcodeFamilyFormula.
//
// A WRITE-THEN-VERIFY of CW-R, AM-SYNC, FM-W, RTTY-U or PKT-F will
// therefore read back as its family's BASE name (CW, AM, FM, RTTY-L,
// PKT-L) — a genuine, documented limitation of this radio's compact
// record, not a driver defect, and exactly the kind of finding the
// project's write-then-verify discipline exists to surface honestly
// rather than paper over.
var recordModeBase = map[byte]byte{
	0: modeLSB,
	1: modeUSB,
	2: modeCW,
	3: modeAM,
	4: modeFM,
	5: modeRTTYL,
	6: modePKTL,
}

// bincatProfile is this radio's core/bincat.Profile value: the
// generic codec's family-wide opcodes plus the geometry this radio
// alone owns (slot space, full-dump length, mode legend, the
// FT-1000MP-only Store/Offset argument layouts — see
// bincat.Profile.StoreChanArg/OffsetZeroArg/OffsetBoundArg's own doc
// comments for why those differ from FT-890/900's).
//
// HasTone is false and there is deliberately no ToneOffset/FlagsOffset
// set: this driver decodes its OWN 16-byte record (record.go), never
// bincat.ParseRecord — FT-1000MP's record geometry (4-byte nibble-decimal
// frequency, no tone byte) does not fit bincat.Record's FT-890/900-shaped
// decode at all (24-bit binary frequency, tone byte). RecordLen is set
// only so Profile.Configured() holds; nothing calls
// Profile.ReplyLength(UMemoryRecord)/AllowedCommand(UMemoryRecord) here —
// this driver never sends a per-channel Status Update (doc.go's read
// design note).
func bincatProfile() bincat.Profile {
	return bincat.Profile{
		Model:          modelName,
		CATID:          catID,
		SlotBase:       1,
		SlotCount:      dumpRecordCount - 3, // 113 — Store's ValidChannel bound
		RecordLen:      16,
		FullDumpLen:    dumpHeaderLen + dumpRecordCount*16,
		Modes:          modeNames,
		HasTone:        false,
		StoreChanArg:   3, // X is the 4th argument byte — matrix §1.8
		OffsetZeroArg:  3, // X4 must be 00H — matrix §4 ShiftOptions citation
		OffsetBoundArg: 2, // X3 must be 00H/01H/02H
	}
}

// Profile selects which capability profile New builds the driver with —
// driver.Base's shared pair, re-declared as this package's own names per
// internal/guards' TestSimulatedProfileTokensConfinement (see
// core/driver/base.go's Profile doc comment).
type Profile = driver.Profile

const (
	RealHardware = driver.RealHardware
	Simulated    = driver.Simulated
)

// banks returns the three MEM/P/QMB banks with fields graded per rw
// (write-side) — RealHardware's all-Unverified baseline or Simulated's
// all-Supported one. Every bank shares one field map: nothing in the
// matrix distinguishes MEM from P/QMB capability-wise (matrix §4
// "Banks" — the only difference is CHIRP reachability, a csvio-layer
// fact, not a FieldSupport one).
func banks(rw spec.FieldSupport) []spec.Bank {
	fields := map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency: rw,
		spec.FieldMode:      rw,
		spec.FieldClarifier: rw,
		spec.FieldShift:     rw,
		// FieldOffset is WRITE-ONLY, on both profiles: the repeater
		// offset MAGNITUDE (F9H) has no byte anywhere in the 16-byte
		// record (matrix §1.2 — only the shift DIRECTION survives, in
		// the VFO/MEM Operating Flags byte record.go decodes). This is
		// a protocol fact, not a profile choice, so even
		// CapabilitiesSimulated's rw.Read is overridden to Unsupported
		// here rather than reused.
		spec.FieldOffset: spec.FieldSupport{Read: spec.Unsupported, Write: rw.Write},
		// FieldCTCSSTone/FieldCTCSSState, FieldTag/FieldTagDisplay and
		// FieldScanSkip are all absent — Unsupported both ways (matrix
		// §0 NoTag, §2 tone, §1.2's undocumented scan-skip bit
		// positions) — see Bank.Fields' own "absent means Unsupported"
		// contract.
	}
	return []spec.Bank{
		{ID: spec.BankMemory, Label: "Memories", Slots: memSlots(), Fields: fields},
		{ID: "P", Label: "P (Program) memories", Slots: pSlots(), Fields: fields},
		{ID: "QMB", Label: "QMB (Quick Memory Bank)", Slots: qmbSlots(), Fields: fields},
	}
}

// baseCapabilities builds the Capabilities banks/Slots/Modes/vocabulary
// share for both profiles, differing only in rw's grading.
func baseCapabilities(rw spec.FieldSupport) spec.Capabilities {
	return spec.Capabilities{
		Model:    modelName,
		CATID:    catID,
		Transmit: spec.HasTransmitter,
		Banks:    banks(rw),
		Modes:    modeNamesOrdered(),
		NoTag:    true,
		TagLen:   0,
		// ClarMaxHz/ClarStepHz: 16-bit signed, 0.625 Hz resolution per
		// bit (matrix §4) — ceiling and step are both derived from the
		// field's own bit width, not independently ASSUMED literals.
		ClarMaxHz: 32767 * 625 / 1000, // 16-bit signed ceiling x 0.625 Hz/bit (matrix §4)
		// ClarStepHz: the record's own bit resolution is 0.625 Hz
		// (matrix §4), finer than any whole-Hz step; 1 Hz is the
		// coarsest step that never rounds a settable offset away.
		ClarStepHz:  1,
		Bauds:       []int{4800},
		DefaultBaud: 4800,
		// MinFreqHz/MaxFreqHz: the 28-band chart's own extremes (matrix
		// §1's Band Selection table, 0.1-30 MHz).
		MinFreqHz: 100_000,
		MaxFreqHz: 30_000_000,
		// ShiftOptions must be non-empty: this bank grades FieldShift
		// above Unsupported (spec.Capabilities.Validate's anyBankReaches
		// gate) — the family-standard three-value vocabulary
		// (spec.md §Write model step 5; matrix §4 "ShiftOptions").
		ShiftOptions: spec.StandardShiftOptions(),
		// CTCSSStates/CTCSSTones/CTCSSToneRange all stay empty: this
		// bank grades neither FieldCTCSSState nor FieldCTCSSTone above
		// Unsupported, so Validate's pairing gate does not require them
		// (matrix §2 "Unsupported, not merely absent").
	}
}

// CapabilitiesUnverified is the all-Unverified fail-safe profile a
// RealHardware session gets — see doc.go's "Override in force" note:
// every field above is graded Unverified/Unverified, opened to
// ConsentedUnverified write-side only via WithConsentedUnverifiedWrites
// (driver.Base.SessionCaps), exactly the FTdx10/101 precedent.
func CapabilitiesUnverified() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified})
}

// CapabilitiesSimulated is the fake-radio-backed profile (CLI --fake,
// GUI demo) — Read AND Write Supported for every field this radio's
// record can express. internal/fakeft1000mp (Phase 4) is what a
// Simulated session actually talks to; this package declares only the
// capability shape a fake must honour.
func CapabilitiesSimulated() spec.Capabilities {
	return baseCapabilities(spec.FieldSupport{Read: spec.Supported, Write: spec.Supported})
}
