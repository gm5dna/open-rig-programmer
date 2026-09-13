// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx3000

import "github.com/gm5dna/open-rig-programmer/core/cat"

// modeNames is the FTdx3000's P6 mode table, transcribed from the MW-Set
// and MR-Answer legends (matrix §1.2, ftdx3000_layout.txt:920-926,
// :950-952 — the two agree word for word): twelve names, '1'-'C', no
// D/E/F. The manual's own D (AM-N) exists ONLY on the separate live MD
// (OPERATING MODE) command's legend (layout:891), never on MW/MR — matrix
// §1.2's correction of the plan/brief's assumed 'D' ceiling — so it is
// simply absent from this map, the same "not a member" treatment every
// other absent nibble in this fleet gets.
//
// Display spellings: the manual prints "6: FSK (RTTY-LSB)" and "9: FSK-R
// (RTTY-USB)" — a CHOICE (not a manual fact) picks the parenthetical
// half, "RTTY-LSB"/"RTTY-USB", since that is the plain ham-radio term and
// it is also how the sibling ftdx1200 spells its OWN, differently-coded,
// overlapping values ("6: RTTY-LSB", "9: RTTY-USB") — the two radios name
// the same code differently only in which half of the manual's own
// parenthetical they lead with, not in which mode it is.
var modeNames = map[cat.Mode]string{
	cat.ModeUnset: "-", // ASSUMED accept-only placeholder — no MW/MR/MD legend prints it

	cat.Mode('1'): "LSB",
	cat.Mode('2'): "USB",
	cat.Mode('3'): "CW",
	cat.Mode('4'): "FM",
	cat.Mode('5'): "AM",
	cat.Mode('6'): "RTTY-LSB",
	cat.Mode('7'): "CW-R",
	cat.Mode('8'): "PKT-L",
	cat.Mode('9'): "RTTY-USB",
	cat.Mode('A'): "PKT-FM",
	cat.Mode('B'): "FM-N",
	cat.Mode('C'): "PKT-U",
	// 'D' (AM-N) is printed on the live MD command only, never on MW/MR
	// (matrix §1.2) — ASSUMED-excluded from the write-capable Mode enum,
	// deliberately absent here.
}

// dialect is the FTdx3000, built once at init and validated by
// cat.MustNewDialect. Built INLINE (doc.go explains why no
// core/cat/ftdx3000 subpackage exists), following the v1.7.0 Yaesu-four
// brief's own instruction for this family shape.
var dialect = cat.MustNewDialect(cat.DialectConfig{
	// ID's own P1 legend, "P1 0462: FTdx3000" (matrix §2.2, layout:759).
	CATID:     "0462",
	ModeNames: modeNames,
	Slots: cat.SlotSpace{
		// "000 - 099: Regular Memory Channel" on MC's legend (layout:876),
		// but MR/MW's OWN P1 legends print "(001 117)" — excluding 000
		// (layout:918, :945). This dialect follows MR/MW, the verbs the
		// banks are built from (matrix §2.4); channel 000's status is an
		// open erratum, the same MC-vs-MR/MW pattern the ft2000 family
		// already carries.
		MemoryLo: 1, MemoryHi: 99,
		// NO 60m BANK, NO EMERGENCY CHANNEL: no "5xx"/"5 MHz"/"EMG" slot
		// legend anywhere in the extraction (matrix §2.4's mechanical
		// check).
		SixtyLo: 0, SixtyHi: 0,
		// Nine numeric pairs, "100: P-1L" ... "117: P-9U" (matrix §2.4,
		// MC legend, layout:875-880) — decimal channel numbers continuing
		// the memory range, cat.PMSFormNumeric.
		PMSPairs:      9,
		PMSForm:       cat.PMSFormNumeric,
		PMSNumericLo:  100,
		EmergencyWire: "",
		// No "000"/VFO placeholder is printed in MR/MW's own channel-number
		// legend (matrix §2.4) — left absent rather than assumed.
		NoneWire: "",
		// MC's own P1 legend spans the WHOLE "000-117" range as one line
		// before decomposing it (layout:875-880) — the ft2000-family
		// reading (core/driver/ft2000/dialect.go's own MCSelects comment):
		// this radio has no 60m/EMG bank for MCSelectsMemoryPMS to narrow
		// away, so the two readings are behaviourally identical and All is
		// the one the legend's own top line actually states.
		MCSelects: cat.MCSelectsAll,
	},
	// EX/menu inventory is OUT OF SCOPE for this wave (brief). P1 is a
	// bare three-digit "001-196 (MENU Number)" (layout:440) — the
	// ft2000/ft991a EXAddressSingle shape, not a triple. Required even
	// with EXItems empty: it also sizes the EX read frame the outbound
	// gate measures.
	EXItems:       nil,
	EXAddressForm: cat.EXAddressSingle,
	// THIS FAMILY HAS NO MT (TAG) COMMAND AT ALL (matrix §0/§1.1: no
	// combined MT-style record and no tag/name route anywhere in the
	// command set). cat.DialectConfig.MT has no default (V9 refuses the
	// zero value unconditionally), so this is INFRASTRUCTURE THIS DRIVER
	// NEVER EXERCISES, exactly ft2000-family's own shape: read.go/write.go
	// build MR/MW frames directly and never call
	// BuildMTRead/BuildMTSet/ParseMTAnswer. The values below are the
	// minimum that satisfies construction.
	MT: cat.MTPolicy{
		Form:         cat.MTFormShort,
		ReadSlots:    cat.MTReadsReadable,
		TagMaxBytes:  1,
		ClearTagByte: ' ',
	},
	// "Clarifier Offset: 0000 - 9999 (Hz)" on every block carrying P3
	// (layout:918, :945) — matrix §2.7. StepHz ASSUMED at 10 Hz, the
	// Yaesu-family default; MaxAbsHz is 9990 (not the printed range's own
	// 9999), the largest multiple of 10 inside it — cat.NewDialect's V10
	// rule requires an exact multiple, the same correction every
	// registered ft2000-family sibling makes.
	Clarifier: cat.ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
	// P5 (TX CLAR) is a live flag: "0: TX CLAR OFF 1: TX CLAR ON", printed
	// on MW/MR alike (layout:927-928, :950) — not a fixed byte.
	MemoryP5: cat.P5TxClar,
	// P8 is the ordinary three-state legacy domain: "0: CTCSS OFF 1: CTCSS
	// ENC/DEC 2: CTCSS ENC" (layout:929, :956) — matrix §2.9, no DCS
	// member.
	ToneStates: cat.ToneStatesCTCSS,
	// MW's P7 write-fixed byte is '0' (layout:955, "P7 0: (Fixed)"), and
	// matrix §1.4 flags the trap: '0' is cat.KindVFO, NOT cat.KindMemory.
	MWWriteKind: cat.KindVFO,
	// 27-byte frame, 8-digit P2 (matrix §1.1: one digit narrower than the
	// registered 28/9 family, a pure position shift for P3 onward).
	MemoryFrameLen:   27,
	MemoryFreqDigits: 8,
	// P9 is ASYMMETRIC (matrix §1.3): MR's P9 pair is a LIVE two-digit
	// CTCSS tone-table index (layout:930-931), but MW's is printed-fixed
	// "0: (Fixed)" (layout:957) — neither of the existing binary
	// MemoryP9Policy values fits a radio whose two directions disagree.
	// cat.P9ToneIndexReadOnly (this wave's own core/cat lift, landed in
	// the commit immediately before this one) is exactly this shape: live
	// on read, fixed-and-refuse-nonzero on write.
	MemoryP9: cat.P9ToneIndexReadOnly,
})

// Dialect returns the FTdx3000's cat.Dialect.
//
// A function over an exported var so the package-held value cannot be
// reassigned by a consumer — every registered single-row Yaesu sibling's
// own reasoning (e.g. core/driver/ftdx9000/dialect.go).
func Dialect() cat.Dialect { return dialect }
