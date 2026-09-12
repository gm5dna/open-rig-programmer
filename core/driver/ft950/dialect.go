// SPDX-License-Identifier: GPL-3.0-or-later

package ft950

import "github.com/gm5dna/open-rig-programmer/core/cat"

// modeNames is the FT-950's P6 mode table, TRANSCRIBED from this radio's
// own manual (matrix §1.3) rather than copied from any sibling's. The
// memory-record legend (MR/MW, layout:854-855/895-897) prints twelve
// names, '1'-'9' then 'A'-'C', with no 'D'/'E'/'F' row at all.
//
// THE MANUAL'S OWN SPELLING IS KEPT, parenthetical and all ("FSK
// (RTTY-LSB)", not "RTTY-LSB"; "PKT-L", not "DATA-L") — the FTdx9000
// precedent in this same wave, not FT-2000's (which substitutes core/cat's
// canonical words for the identical legend text). No nibble here disagrees
// about WHICH mode it is, only its display word, so no per-model
// ModeName override table is needed beyond this map.
//
// cat.ModeUnset ('0', "-") is included as an ASSUMED member, matching
// every registered sibling: it appears in no FT-950 legend, but parsers
// must accept it and core/cat refuses to ever EMIT it in a Set frame.
var modeNames = map[cat.Mode]string{
	cat.ModeUnset: "-",

	cat.Mode('1'): "LSB",
	cat.Mode('2'): "USB",
	cat.Mode('3'): "CW",
	cat.Mode('4'): "FM",
	cat.Mode('5'): "AM",
	cat.Mode('6'): "FSK (RTTY-LSB)",
	cat.Mode('7'): "CW-R",
	cat.Mode('8'): "PKT-L",
	cat.Mode('9'): "FSK-R (RTTY-USB)",
	cat.Mode('A'): "PKT-FM",
	cat.Mode('B'): "FM-N",
	cat.Mode('C'): "PKT-U",
	// 'D', 'E', 'F' are not printed in the memory-record legend at all
	// (matrix §1.3). MD's OWN P2 legend prints a thirteenth value, "D:
	// AM-N" (layout:812) — the matrix's own §1.7 erratum, recorded not
	// resolved (doc.go). It is deliberately NOT added here: this dialect
	// encodes the memory blocks' P6, not MD's P2, and widening this table
	// would let the driver claim a mode no memory-record byte can carry.
}

// dialect is the FT-950, built once at init and validated by
// cat.MustNewDialect. EVERY FIELD IS SET EXPLICITLY, including the ones
// this configuration requires to be zero (SixtyLo/Hi, EmergencyWire,
// NoneWire, the short-form MT fields this radio never actually sends —
// see below).
//
// Built INLINE over cat.NewDialect/MustNewDialect rather than a
// core/cat/ft950 subpackage (v1.7.0 Kenwood/Yaesu wave brief, Yaesu four):
// this is a single-row package, so a literal here is the whole of it.
//
// MustNewDialect rather than NewDialect: a mistake in this literal is a
// build-time defect that must stop the programme loudly on first use.
var dialect = cat.MustNewDialect(cat.DialectConfig{
	// matrix §4: the ID command's own P1 legend, "P1 0310 (Fixed value)"
	// (layout:676-682, Page 8).
	CATID:     "0310",
	ModeNames: modeNames,
	Slots: cat.SlotSpace{
		// MemoryLo IS 0, NOT 1 — THIS RADIO'S OWN DELTA FROM ITS SIBLINGS
		// (matrix §1.5, doc.go entry 4). The MC legend's own first line
		// reads "000 - 099: Regular Memory Channel" (layout:797) — one
		// more regular channel than FT-2000/FTdx5000/FTdx9000 document.
		// MemoryLo == 0 is PERMITTED (dialectvalidate.go's own comment: "a
		// radio numbering its channels from 000 is legitimate").
		MemoryLo: 0, MemoryHi: 99,
		// NO 60m BANK: "5xx"/"5 MHz"/"5MHz" appear in no slot legend of
		// this manual (matrix §4's own mechanical check).
		SixtyLo: 0, SixtyHi: 0,
		// Nine PMS pairs, numbered decimally continuing the memory range:
		// the MC legend's own "100: P1L 101: P1U ~ 116: P9L 117: P9U"
		// (layout:795-802, matrix §1.5) — the same numeric form every
		// sibling in this wave declares.
		PMSPairs:     9,
		PMSForm:      cat.PMSFormNumeric,
		PMSNumericLo: 100,
		// NO EMERGENCY CHANNEL as a memory slot: "EMG" appears in no slot
		// legend of this manual — the menu row "118 EMERGENCY CHANNEL"
		// (layout:597) is a live receiver setting (0: DISABLE 1: ENABLE),
		// not a memory bank.
		EmergencyWire: "",
		// NO "000" none/VFO placeholder EITHER, and it COULD NOT be one on
		// this radio even if the manual printed such a form: "000" is
		// ALREADY a real memory channel here (MemoryLo 0), so V7's
		// shadowing rule would refuse a NoneWire colliding with it. The
		// MC legend's whole domain is "000 - 117" (layout:795) with no
		// separate zero form printed anywhere.
		NoneWire: "",
		// The MC legend prints the WHOLE "000 - 117" span as one P1 range
		// (layout:795) before decomposing it into regular/PMS — the same
		// shape FT-991A's and FT-2000's own MC legends take for the
		// identical reason. This radio has no 5xx/EMG bank at all, so
		// MCSelectsAll and MCSelectsMemoryPMS give IDENTICAL verdicts on
		// every wire form there is; the wide value is declared because it
		// is what the legend's own top line says.
		MCSelects: cat.MCSelectsAll,
	},
	// EX/menu inventory is OUT OF SCOPE for this wave (brief, "EX/menu
	// inventory is OUT for all seven packages"): EXItems stays empty.
	EXItems: nil,
	// "EX P1 P1 P1 ;" (layout:439-445): a SINGLE three-digit P1 component
	// ("P1 : 001-118 (MENU Number)") — the FT-991A/FT-2000 shape, not the
	// six- or four-digit forms. Required even with EXItems empty: it also
	// sizes the EX read frame the outbound gate measures
	// (cat.DialectConfig.EXAddressForm's own doc comment).
	EXAddressForm: cat.EXAddressSingle,
	// THIS RADIO'S CONTROL COMMAND LIST HAS NO MT ROW AT ALL (layout:118-181;
	// a whole-document grep for an "MT" command header finds none — only
	// MC, MD, MR, MW). But cat.DialectConfig.MT has no default
	// (validateMTPolicy/V9 refuses the zero value unconditionally, whether
	// or not a radio's own MT command exists), so this is INFRASTRUCTURE
	// THIS DRIVER NEVER EXERCISES: read.go/write.go build MR/MW frames
	// directly and never call BuildMTRead/BuildMTSet/ParseMTAnswer — the
	// same "declared but unreachable" shape lift K's newStreamError gap
	// takes for ts570/ts870s, and the identical gap FT-2000/FTdx5000/
	// FTdx9000 each hit independently in this same wave. TagMaxBytes is
	// pinned at its allowed minimum (1) and ClearTagByte at an arbitrary
	// valid wire byte purely to satisfy V9's construction-time validation;
	// neither value describes anything this radio's manual prints.
	MT: cat.MTPolicy{
		Form:         cat.MTFormShort,
		ReadSlots:    cat.MTReadsReadable,
		TagMaxBytes:  1,
		ClearTagByte: ' ',
		PadByte:      0,
		TagFill:      0,
	},
	// "Clarifier Offset: 0000 - 9999 (Hz)" (layout:877, :898) — matrix §2.
	// StepHz is ASSUMED at 10 Hz, the Yaesu-family default every registered
	// sibling carries (no step is printed anywhere in this manual).
	// MaxAbsHz is 9990, not the printed range's own 9999: cat.NewDialect's
	// V10 rule requires MaxAbsHz to be an exact multiple of StepHz, and
	// 9999 is not a multiple of 10 — 9990 is the largest multiple of the
	// assumed step inside the printed range, the FT-991A's own precedent
	// for exactly this shape.
	Clarifier: cat.ClarifierPolicy{
		StepHz:   10,
		MaxAbsHz: 9990,
	},
	// P5 (byte 20) is a LIVE flag: "P5 0: TX CLAR \"OFF\" 1: TX CLAR
	// \"ON\"" (layout:877, :898) — matrix §2.
	MemoryP5: cat.P5TxClar,
	// P8 is the ordinary three-state legacy domain: "0: CTCSS \"OFF\"
	// 1: CTCSS ENC/DEC 2: CTCSS ENC" (layout:857, :898) — matrix §2, no DCS
	// member.
	ToneStates: cat.ToneStatesCTCSS,
	// MW's P7 write-fixed byte is '0' (layout:895, "P7 0: (Fixed)"), and
	// matrix §2 flags the trap: '0' is cat.KindVFO, NOT cat.KindMemory
	// (memdata.go's own Kind byte legend, "0 VFO, 1 Memory") — the naming
	// trap the brief flags for this whole Yaesu family.
	MWWriteKind: cat.KindVFO,
	// 27-byte frame, 8-digit P2 (matrix §1.1/§1.2: one byte/digit narrower
	// than every registered sibling's 28/9 shape — a pure position shift
	// for P3 onward).
	MemoryFrameLen:   27,
	MemoryFreqDigits: 8,
	// P9 (positions 24-25) is a LIVE two-digit CTCSS tone-table index
	// (matrix §1.4), not the registered siblings' printed-fixed "00" — the
	// Lift-Y P9ToneIndex axis, into the same standard 50-tone chart every
	// registered dialect shares.
	MemoryP9: cat.P9ToneIndex,
})

// Dialect returns the FT-950's cat.Dialect.
//
// A function over an exported var so the package-held value cannot be
// reassigned by a consumer — see the sibling dialects' identical reasoning.
func Dialect() cat.Dialect { return dialect }
