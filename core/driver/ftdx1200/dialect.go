// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx1200

import "github.com/gm5dna/open-rig-programmer/core/cat"

// modeNames is the FTDX1200's P6 mode table, transcribed from the MW-Set
// and MR-Answer legends (matrix §1.2, ftdx1200_layout.txt:900-902,
// :873-875 — the two agree word for word, as does the live MD command's
// own legend, matrix §1.2): twelve print positions, ELEVEN real names —
// 'A' is printed "----" (a reserved/unused slot, genuinely absent on this
// radio, on every command that carries the field, not just MW/MR) and is
// therefore simply not a member of this map, the same "not a member"
// treatment every other absent nibble in this fleet gets. No 'D' exists
// anywhere on this radio, live or stored — unlike the sibling ftdx3000,
// whose live MD command DOES print one.
var modeNames = map[cat.Mode]string{
	cat.ModeUnset: "-", // ASSUMED accept-only placeholder — no MW/MR/MD legend prints it

	cat.Mode('1'): "LSB",
	cat.Mode('2'): "USB",
	cat.Mode('3'): "CW",
	cat.Mode('4'): "FM",
	cat.Mode('5'): "AM",
	cat.Mode('6'): "RTTY-LSB",
	cat.Mode('7'): "CW-R",
	cat.Mode('8'): "DATA-LSB",
	cat.Mode('9'): "RTTY-USB",
	// 'A' printed "----" (a genuine hole, matrix §1.2) — deliberately
	// absent: ParseMode/ValidMode refuse it, exactly like an absent
	// nibble anywhere else in this fleet.
	cat.Mode('B'): "FM-N",
	cat.Mode('C'): "DATA-USB",
}

// dialect is the FTDX1200, built once at init and validated by
// cat.MustNewDialect. Built INLINE (doc.go explains why no
// core/cat/ftdx1200 subpackage exists).
//
// CATID is "0582" here — the FFT-1-fitted variant, listed first in the
// manual's own ID legend (matrix §1.6/§2.2) — a CHOICE of canonical
// identity for this single dialect value; ftdx1200.go's identify() logic
// accepts EITHER "0582" or "0583" at the ID probe, since the manual
// states plainly this is ONE radio with an option-split CAT ID, not two
// models (matrix §1.6: "the FFT-1 option is never mentioned again outside
// this one ID block").
var dialect = cat.MustNewDialect(cat.DialectConfig{
	CATID:     "0582",
	ModeNames: modeNames,
	Slots: cat.SlotSpace{
		// "000 - 099: Regular Memory Channel" on MC's legend
		// (layout:831-838), but MR/MW's OWN P1 legends print "(001 ～
		// 117)" — excluding 000 (layout:871, :895; the full-width tilde
		// is a font/rendering artefact of this PDF, not a different
		// character from ftdx3000's ASCII hyphen). This dialect follows
		// MR/MW, the same fleet-wide MC-vs-MR/MW pattern ftdx3000 and
		// ft2000 already carry; channel 000's status is an open erratum.
		MemoryLo: 1, MemoryHi: 99,
		// NO 60m BANK, NO EMERGENCY CHANNEL: mechanical whole-extraction
		// check, neither found.
		SixtyLo: 0, SixtyHi: 0,
		// Nine numeric pairs, "100: P-1L" ... "117: P-9U" (matrix §2.4,
		// MC legend, layout:831-838) — identical shape to ftdx3000's own.
		PMSPairs:      9,
		PMSForm:       cat.PMSFormNumeric,
		PMSNumericLo:  100,
		EmergencyWire: "",
		// No "000"/VFO placeholder printed in MR/MW's own channel-number
		// legend — left absent rather than assumed.
		NoneWire: "",
		// MC's own P1 legend spans the WHOLE "000-117" range as one line
		// before decomposing it (layout:831-838) — the same ft2000-family
		// reasoning as ftdx3000's own MCSelects comment: no 60m/EMG bank
		// for MCSelectsMemoryPMS to narrow away, so the two readings are
		// behaviourally identical and All is the one the legend's own top
		// line actually states.
		MCSelects: cat.MCSelectsAll,
	},
	// EX/menu inventory is OUT OF SCOPE for this wave. P1 is a bare
	// three-digit "001-196 (MENU Number)" (layout:442) — the same
	// EXAddressSingle shape as ftdx3000's own EX grammar. Required even
	// with EXItems empty: it also sizes the EX read frame the outbound
	// gate measures.
	EXItems:       nil,
	EXAddressForm: cat.EXAddressSingle,
	// THIS FAMILY HAS NO MT (TAG) COMMAND AT ALL (matrix §0/§1.1: zero
	// grep hits of any kind — stronger than ftdx3000's own). Structurally
	// required placeholder, never exercised — see ftdx3000's identical
	// reasoning.
	MT: cat.MTPolicy{
		Form:         cat.MTFormShort,
		ReadSlots:    cat.MTReadsReadable,
		TagMaxBytes:  1,
		ClearTagByte: ' ',
	},
	// "Clarifier Offset: 0000 - 9999 (Hz)" (layout:871, :896) — matrix
	// §2.7. StepHz ASSUMED at 10 Hz; MaxAbsHz 9990, the same V10 multiple-
	// of-step correction every ft2000-family sibling makes.
	Clarifier: cat.ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
	// P5 (TX CLAR) is a live flag, printed on MR/MW alike (layout:872,
	// :899) — not a fixed byte.
	MemoryP5: cat.P5TxClar,
	// P8 is the ordinary three-state legacy domain (layout:872, :904) —
	// matrix §2.9, no DCS member.
	ToneStates: cat.ToneStatesCTCSS,
	// MW's P7 write-fixed byte is '0' (layout:903, "P7 0: (Fixed)"), and
	// the same ft2000-family trap applies: '0' is cat.KindVFO, not
	// cat.KindMemory.
	MWWriteKind: cat.KindVFO,
	// 27-byte frame, 8-digit P2 — identical shape to ftdx3000's own
	// (matrix §1.1).
	MemoryFrameLen:   27,
	MemoryFreqDigits: 8,
	// P9 is FIXED on BOTH read and write (matrix §1.3): "P10 00: (Fixed)"
	// on MR's own read-side legend (layout:882) and "P9 00: (Fixed)" on
	// MW's (layout:905) — unlike ftdx3000's asymmetric read/write split,
	// this fits the existing binary cat.P9Fixed00 cleanly; no core/cat
	// lift needed for this package.
	MemoryP9: cat.P9Fixed00,
})

// Dialect returns the FTDX1200's cat.Dialect.
func Dialect() cat.Dialect { return dialect }
