// SPDX-License-Identifier: GPL-3.0-or-later

package ft2000

import "github.com/gm5dna/open-rig-programmer/core/cat"

// modeNames is the FT-2000 series' P6 mode table: twelve names, '1'-'9'
// then 'A'-'C', identical on MW's Set legend (ft2000_layout.txt:924-926)
// and MR/OI's read-side legends (:905, :752) — matrix §1.2. D/E/F are
// printed nowhere; see doc.go's register for the reason this driver leaves
// them out of the field audit rather than mapping them.
//
// cat.ModeUnset ('0', "-") is included as an ASSUMED member, matching
// every registered sibling: it appears in no FT-2000 legend, but parsers
// must accept it and core/cat refuses to ever EMIT it in a Set frame.
var modeNames = map[cat.Mode]string{
	cat.ModeUnset: "-",

	cat.Mode('1'): "LSB",
	cat.Mode('2'): "USB",
	cat.Mode('3'): "CW-U",
	cat.Mode('4'): "FM",
	cat.Mode('5'): "AM",
	cat.Mode('6'): "RTTY-L",
	cat.Mode('7'): "CW-L",
	cat.Mode('8'): "DATA-L",
	cat.Mode('9'): "RTTY-U",
	cat.Mode('A'): "DATA-FM",
	cat.Mode('B'): "FM-N",
	cat.Mode('C'): "DATA-U",
	// 'D', 'E', 'F' are not printed in this radio's mode legend at all
	// (matrix §1.2): this manual's own domain stops at 'C'.
}

// dialectConfig is the ONE config shared by FT-2000 and FT-2000D: matrix
// §1 and §4 find the two rows byte-identical on every position except
// CATID, so the whole of the two dialects' difference is the four-digit
// string this function takes.
//
// Built INLINE over cat.NewDialect/MustNewDialect rather than a
// core/cat/ft2000 subpackage (v1.7.0 Kenwood/Yaesu wave brief, Yaesu
// four): this family shares one dialect variant differing only in Model
// name and CATID, so a literal here is the whole of it.
func dialectConfig(catID string) cat.DialectConfig {
	return cat.DialectConfig{
		CATID:     catID,
		ModeNames: modeNames,
		Slots: cat.SlotSpace{
			// "001-099: Regular Memory Channel" (MC legend,
			// ft2000_layout.txt:846) — matrix §2.4.
			MemoryLo: 1, MemoryHi: 99,
			// No 60m bank: "5xx"/"5 MHz"/"EMG" appear in no slot legend of
			// this manual (matrix §2.4's mechanical check; the one "5 MHz"
			// hit is BS band-select's band list, layout:274, not a memory
			// bank).
			SixtyLo: 0, SixtyHi: 0,
			// Nine PMS pairs, numbered decimally continuing the memory
			// range: "100:P1L 101:P1U ~ 116:P9L 117:P9U" (MC legend,
			// layout:847-850) — matrix §2.4. The wire numbers ARE the pair
			// number (no "P1L"-style token, unlike the FT-991A's own
			// labelling, which still wires the SAME numeric form).
			PMSPairs:     9,
			PMSForm:      cat.PMSFormNumeric,
			PMSNumericLo: 100,
			// No emergency channel: "EMG" appears in no slot legend
			// (matrix §2.4).
			EmergencyWire: "",
			// No "000" none/VFO placeholder either: the MC legend's whole
			// domain is "001-117" (layout:845), with no zero form printed
			// anywhere in this manual's memory-channel legends — unlike
			// every registered sibling, whose MR answer prints "000" as a
			// VFO placeholder. cat.SlotSpace spells that absence "".
			NoneWire: "",
			// MC's own legend prints the WHOLE "001-117" span as one P1
			// range (layout:845) before decomposing it into memory and
			// PMS — the same shape the FT-991A's own MC legend takes for
			// the identical reason (core/driver/ft991a/caps.go's own
			// citation of it). This radio has no 5xx/EMG bank at all, so
			// MCSelectsAll and MCSelectsMemoryPMS give IDENTICAL verdicts
			// on every wire form there is — the wide value is declared
			// because it is what the legend's own top line says, not
			// because anything depends on the distinction.
			MCSelects: cat.MCSelectsAll,
		},
		// EX/menu inventory is OUT OF SCOPE for this wave (brief, "EX/menu
		// inventory is OUT for all seven packages"): EXItems stays empty.
		EXItems: nil,
		// The EX grammar prints "P1 : 001-149 (MENU Number)" against a
		// SINGLE three-digit P1 component (ft2000_layout.txt:434-441) —
		// the FT-991A's EXAddressSingle shape, not the six- or four-digit
		// forms. Required even with EXItems empty: it also sizes the EX
		// read frame the outbound gate measures (cat.DialectConfig.
		// EXAddressForm's own doc comment).
		EXAddressForm: cat.EXAddressSingle,
		// THIS FAMILY HAS NO MT (TAG) COMMAND AT ALL — a whole-document
		// grep of ft2000_layout.txt finds no "MT" command block, and
		// matrix §0/§1.1 confirm no combined MT-style record and no
		// tag/name route exists anywhere in the 90-command set. But
		// cat.DialectConfig.MT has no default (validateMTPolicy/V9 refuses
		// the zero value unconditionally, whether or not a radio's own MT
		// command exists), so this is INFRASTRUCTURE THIS DRIVER NEVER
		// EXERCISES: core/driver/ft2000's read/write paths build MR/MW
		// frames directly (read.go, write.go) and never call
		// BuildMTRead/BuildMTSet/ParseMTAnswer — the same "declared but
		// unreachable" shape lift K's newStreamError gap takes for
		// ts570/ts870s. TagMaxBytes is pinned at its allowed minimum (1)
		// and ClearTagByte at an arbitrary valid wire byte purely to
		// satisfy V9's construction-time validation; neither value
		// describes anything this radio's manual prints.
		MT: cat.MTPolicy{
			Form: cat.MTFormShort,
			// MTReadsReadable, the WIDE value: this radio has neither a
			// 5xx nor an EMG bank for MTReadsMemoryPMS's narrowing to
			// differ over, and there is no MT legend to cite either way
			// (this family has no MT command at all — see the comment
			// above). The wide default matches every registered
			// sibling's own reading and keeps this genuinely inert axis
			// from asserting a narrowing this radio's manual never states.
			ReadSlots:    cat.MTReadsReadable,
			TagMaxBytes:  1,
			ClearTagByte: ' ',
			PadByte:      0,
			TagFill:      0,
		},
		Clarifier: cat.ClarifierPolicy{
			// "Clarifier Offset: 0000 - 9999 (Hz)" on every block carrying
			// P3 (layout:927, :1000) — matrix §2.7. StepHz is ASSUMED at
			// 10 Hz, the Yaesu-family default every registered sibling
			// carries (no step is printed anywhere in this manual).
			StepHz: 10,
			// CORRECTION TO THE MATRIX: §2.7 states 9999 needs "no step
			// assumption ... for the CEILING", but cat.NewDialect's V10
			// rule requires MaxAbsHz to be an exact multiple of StepHz —
			// 9999 is not a multiple of 10. 9990, the largest multiple of
			// the assumed 10 Hz step inside the printed 0000-9999 range,
			// is the FT-991A's own precedent for exactly this shape
			// (a deduction from the assumed step, not a transcription).
			MaxAbsHz: 9990,
		},
		// P5 (TX CLAR) is a live flag on this radio, "0: TX CLAR OFF
		// 1: TX CLAR ON" printed on MW/MR/OI alike (layout:928, :899,
		// :1004) — the four registered dialects' own reading, not the
		// FT-891/FT-991A's fixed-byte shape.
		MemoryP5: cat.P5TxClar,
		// P8 is the ordinary three-state legacy domain, "0: CTCSS OFF
		// 1: CTCSS ENC/DEC 2: CTCSS ENC" (layout:935, :1000) — matrix
		// §2.9: no DCS member, the opposite of the FT-991A's novelty.
		ToneStates: cat.ToneStatesCTCSS,
		// MW's P7 write-fixed byte is '0' (layout:934, "P7 0: (Fixed)"),
		// and matrix §1.4 flags the trap: '0' is cat.KindVFO, NOT
		// cat.KindMemory (memdata.go's own Kind byte legend, "0 VFO,
		// 1 Memory").
		MWWriteKind: cat.KindVFO,
		// 27-byte frame, 8-digit P2 (matrix §1.1: one digit narrower than
		// every registered sibling, a pure position shift for P3 onward).
		MemoryFrameLen:   27,
		MemoryFreqDigits: 8,
		// P9 is a LIVE two-digit CTCSS tone-table index (matrix §1.3),
		// not the registered siblings' printed-fixed "00" — the ft2000
		// family is MemoryP9Policy's own worked example (dialectconfig.go).
		MemoryP9: cat.P9ToneIndex,
	}
}

// dialectFT2000 and dialectFT2000D are this package's two Dialects, built
// once at init: MustNewDialect because this is a compile-time-constant
// table (a mistake here is a build-time defect, not a runtime error to
// thread through registration). CATID is the sole difference — matrix §4,
// §1.6: "0251" for FT-2000, "0252" for FT-2000D, from the ID command's
// P1 legend (ft2000_layout.txt:725-732), which directly contradicts the
// S3 evidence file's "no ID-select command exists" claim (matrix §1.6
// finding 1).
var (
	dialectFT2000  = cat.MustNewDialect(dialectConfig("0251"))
	dialectFT2000D = cat.MustNewDialect(dialectConfig("0252"))
)
