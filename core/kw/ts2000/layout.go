// SPDX-License-Identifier: GPL-3.0-or-later

package ts2000

import "github.com/gm5dna/open-rig-programmer/core/kw"

// THE THREE ROWS, AND WHY ONE LAYOUT SERVES ALL THREE.
//
// TS-2000, TS-2000X and TS-B2000 share one PC-command document
// ("TS-2000_2000X_B2000", ts2000-manual-provenance.md) and, per each row's
// own "MODELS COVERED BY THIS MANUAL" citation (PDF p.3), print ZERO byte
// difference in the 50-byte MR/MW grid (ts2000-capability-matrix.md §1-§2):
// same width, same sixteen parameter fields, same terminator. The only
// per-row facts are the Model NAME (used in this codec's own refusal text)
// and the CATID (a driver-package concern, caps.go) — so one LayoutConfig
// literal, repeated three times with only Model varying, is the whole of
// this file.
//
// FIVE LIVE AXES, THREE NEW TYPES, TWO REUSED WITH A NEW VALUE
// (matrix §2). The registered 590-pair/TS-480 record hard-wires bytes
// 25-27 (P10), 29 (P12) and 30-38 (P13) as constants; the TS-2000 carries
// all three LIVE — a 3-digit DCS code, a shift-status enum and a 9-digit
// offset frequency — so P10Policy/P12Policy/P13Policy (new types, lift K)
// are set to their live members here. Byte 28 (P11, REVERSE) and byte 41
// (P15, Memory Group) reuse the EXISTING Byte28Policy/Byte41Meaning axes
// with the new members lift K added for this row (Byte28Reverse,
// Byte41MemoryGroup) — see core/kw/layout.go for both axes' full doc
// comments; this file only picks the member each one already documents.
//
// P2, BYTE 19 AND BYTES 39-40 ARE PLAIN REUSE, NO NEW VALUE. Byte 4 (P2) is
// the channel's hundreds digit exactly as the 590 pair's own MC (matrix
// §2, ts2000:10590-10600 area — "P1 _ (space): No bank number, 0~2: Memory
// bank number", the same "no bank"/digit split 590:1332-1337 prints).
// Byte 19 (P6) is the channel lockout, the TS-480's own meaning
// (ts2000:10704, "Lockout status. 0: Lockout OFF, 1: Lockout ON"). Bytes
// 39-40 (P14) are a tuning-step index, again the TS-480's meaning
// ("Step size. See ST command.", ts2000:10720-10722) — ST's own legend is
// mode-conditional over two ranges here too (ts2000:11508-11521,
// "SSB/CW/FSK mode: 00~03" against "AM/FM mode: 00~09"), which is why
// EX/menu and tuning-step vocabulary are both excluded this wave (matrix §2
// row P14; spec §6 Q4) and why this row's driver never builds an MW Set —
// see core/driver/ts2000/write.go.
//
// THE BOOK CITED FOR ERRORS IS kw.Book480, NOT kw.Book590 — A DELIBERATE
// CHOICE THIS PACKAGE MAKES ON ITS OWN EVIDENCE, recorded here because
// lift K minted no Book2000 and this package may not touch
// core/kw/framing.go or core/kw/errors.go to add one (brief, "touch only
// your own new package directories"). Two independent facts point the same
// way:
//
//   - THIS RADIO PROBES WITH TY, NOT FV. Its own manual prints "Sets or
//     reads the microprocessor fimware type" under TY (ts2000:11678,
//     P1 Reserved, P2 a three-value hardware-type legend — "0: Overseas
//     type / 1: Japanese 100 W type / 2: Japanese 20 W type",
//     ts2000:11683-11685) — the TS-480's own shape. The 590 pair's FV
//     ("firmware VERSION", four characters) appears nowhere in this
//     document. kw.Layout.BuildTYRead/ParseTYAnswer requireBook(Book480)
//     and BuildFVRead/ParseFVAnswer requireBook(Book590): naming Book590
//     here would make this row's own identity probe unbuildable.
//   - THIS RADIO'S "O;" CAUSE SENTENCE MATCHES BOOK480'S WORDING, NOT
//     BOOK590'S. Its own error-message table (ts2000:9600-9618, PDF
//     p.113-114) prints "E;" with the sentence every book shares and "O;"
//     as "Receive data was sent but processing was not completed." — the
//     TS-480's own cause (480:143-144, erratum E13), not the 590 pair's "A
//     receive buffer overrun error occurred." Citing Book590 here would
//     put the WRONG CAUSE, not merely the wrong line number, in a
//     StreamError a real session could raise.
//
// THE COST IS HONEST AND STATED RATHER THAN HIDDEN: kw.RejectionError,
// kw.TimeoutError and kw.newStreamError's bookCitations/newStreamError
// tables cite Book480's OWN LINE NUMBERS (480:130-135, 480:136-138,
// 480:143-144), not this document's (ts2000:9603-9618) — a citation-LINE
// mismatch inherent in reusing an existing Book value, on a document
// neither of the two-books-per-axis touched. It is the closer of the two
// legal choices on both grounds above, and it is not core/kw/errors.go's
// "no transcription exists" gap (that binds Book570/Book870S only, and does
// not apply here: a transcription DOES exist, ts2000:9603-9618 above) — it
// is a citation-attribution cost this package accepts rather than a
// missing-evidence one.
var (
	// TS2000 is the base row: CATID "019" (driver caps.go), MANUAL-EVIDENCED
	// (ts2000-capability-matrix.md §1, decisive cite PDF pp.136-137).
	TS2000 = newLayout("TS-2000")
	// TS2000X shares this document, MODELS COVERED naming it at PDF p.3
	// (ts2000x.md §1) — its own citation, not inherited (matrix §1).
	TS2000X = newLayout("TS-2000X")
	// TSB2000 likewise, tsb2000.md §1 (matrix §1).
	TSB2000 = newLayout("TS-B2000")
)

// newLayout builds one row's Layout: every field identical across the three
// rows but Model (matrix §1-§2, zero byte difference).
func newLayout(model string) kw.Layout {
	return kw.MustNewLayout(kw.LayoutConfig{
		Book:  kw.Book480,
		Model: model,

		// Unchanged width: the same 50-byte grid the 590 pair and the
		// TS-480 print (matrix §2, width delta 0).
		RecordLen: kw.RecordLen,

		P2:        kw.P2HundredsDigit, // MC's own bank digit (ts2000:10589-10600)
		Byte19:    kw.Byte19Lockout,   // "0: Lockout OFF, 1: Lockout ON" (ts2000:10704)
		Byte28:    kw.Byte28Reverse,   // P11 REVERSE status (ts2000:10713-10714)
		Byte3940:  kw.Byte3940StepIndex,
		Byte41:    kw.Byte41MemoryGroup, // P15 Memory Group 0-9 (ts2000:10723-10724)
		ToneModes: kw.ToneModesFour,     // width/count match; 4th value is DCS here, not Cross Tone (matrix §6 item 1)

		P10: kw.P10DCSCode,    // bytes 25-27, "DCS code. See QC command." (ts2000:10711-10712)
		P12: kw.P12ShiftLive,  // byte 29, "SHIFT status. See OS command." (ts2000:10715-10716)
		P13: kw.P13OffsetLive, // bytes 30-38, "Offset frequency. See OS command." (ts2000:10717-10718)

		// The EX chart's own printed domain: the highest menu number this
		// document's Appendix table prints is 62 (rows 62A-62E, "Sky
		// Command II+", ts2000:10234-10246, PDF p.122) — NOT the P1 field
		// WIDTH ("000~999: Menu No. (1st)", ts2000:9999-10001), which is a
		// digit count and not a domain, the same distinction
		// core/kw/layout.go's own MaxEXAddress doc draws. EX/menu
		// INVENTORY is out of scope this wave (brief; spec §6 Q4) — no
		// core/kw/ts2000/exinventory*.go is built — but NewLayout refuses
		// a zero MaxEXAddress unconditionally (it is the codec's own
		// BuildEXRead bound, not part of the inventory), so this axis is
		// still pinned to a real, cited number rather than left unset.
		MaxEXAddress: 62,

		ModeNames: modeNames(),

		// MEM 000-289 (ordinary memory), SCAN 290-299 (ten Program Scan
		// channels, each a start/end pair addressed by the same slot
		// number with different P1 values, exactly the 590 pair's
		// SlotScan shape). Read directly off the manual rather than taken
		// from the matrix's own summary line, which describes MEM as
		// "000-299 (300 slots, continuous)" alongside SCAN "290-299" —
		// the two would overlap if both were taken literally, and
		// kw.NewLayout refuses overlapping slot ranges outright.
		// ts2000:5614 ("00 ~ 289 | 290 ~ 299"), ts2000:10726-10727 and
		// ts2000:10797-10798 (both MR and MW: "Memory channel 290 ~ 299:
		// P1=0 (start frequency), P1=1 (end frequency)") settle the real
		// split: 290 ordinary memories, then ten scan-edge pairs.
		Slots: []kw.SlotRange{
			{Class: kw.SlotMemory, Lo: 0, Hi: 289},
			{Class: kw.SlotScan, Lo: 290, Hi: 299},
		},

		// The three positions the registered rows print as constants and
		// this row carries live (P10/P12/P13) are NOT in this set — they
		// are axis-live here, and kw.NewLayout's crossChecks refuse a
		// PrintedFixed entry at a position an axis has already made live.
		// Nothing on this row is hard-wired: all sixteen parameter fields
		// carry a live meaning, which is the TS-2000 lift's whole point
		// (matrix §2: "THREE need a NEW field-encoding type... TWO reuse
		// an existing byte-enum TYPE with new enum values").
		PrintedFixed: nil,
	})
}

// modeNames is the MD legend MR/MW's P5 is read against (ts2000:10611-10619,
// PDF p.126), in this programme's own spellings.
//
// EIGHT NAMES OVER NINE PRINTED NIBBLES. This document's own MD legend
// names 1-9 rather than 0-9 (unlike the 590 pair's/TS-480's 0-9 with 0 and
// 8 both "no mode"): nibble 8 is printed "Reserved" (ts2000:10614-10615)
// alone and nibble 0 is not documented as an MD value at all
// (ts2000:10611-10619 runs 1-9; matrix §5). Both are holes with no mode
// name, and kw.NewLayout refuses a legend naming either.
//
// "CR-R" AT NIBBLE 7 IS A MANUAL TYPO FOR "CW-R" (matrix §4, parallel to
// the TS-480's own "CWR" typo, exemplar erratum E12) — the published
// spelling here is this project's own consistent CW-R, matching the two
// registered rows' own nibble 7.
func modeNames() map[kw.Mode]string {
	return map[kw.Mode]string{
		kw.ModeLSB:  "LSB",
		kw.ModeUSB:  "USB",
		kw.ModeCW:   "CW",
		kw.ModeFM:   "FM",
		kw.ModeAM:   "AM",
		kw.ModeFSK:  "FSK",
		kw.ModeCWR:  "CW-R",
		kw.ModeFSKR: "FSK-R",
	}
}
