// SPDX-License-Identifier: GPL-3.0-or-later

// Package ts590 holds the TS-590S and TS-590SG halves of the Kenwood codec:
// the two layout values and the two menu inventories.
//
// TWO REGISTRY ROWS, ONE PACKAGE, AND NEVER ONE RADIO. Kenwood prints the
// S and the SG in one document, which is a property of the book and not of
// the radios: they have different firmware, different menu domains (88 rows
// against 100, A26) and a byte whose liveness differs between them (byte 28
// / P11, A14). Nothing in this package may say "a TS-590" — the ASSUMED
// register's standing rule from draft 5, and the reason both inventories
// and both layout values are per-row rather than shared.
//
// The codec itself — framing, the accumulator, the matcher, the EX types
// and the typed error family — is core/kw's. This package carries only what
// differs between the two rows. It never imports core/cat or core/civ
// (core/kw/imports_test.go's fence covers this directory too).
//
// # The layout axes, and the line of this book each rests on
//
// kw.Layout has nine axes. SEVEN ARE THE BOOK'S and are identical on both
// rows; TWO ARE THE ROW'S. layout.go carries each citation at its own field
// and layout_test.go pins both lists, so the table below is a reader's index
// rather than a second copy of the values:
//
//	byte 4     P2   The channel number's hundreds digit. MR and MW say only
//	                "Channel number (refer to the MC command)" (590:1453,
//	                590:1539-1540); MC's own chart prints the convention —
//	                "enter 0 or a space for a channel number less than 100.
//	                For a response command, a space is entered for a channel
//	                number less than 100." (590:1332-1337). That MR and MW
//	                inherit it is A10.
//	byte 19    P6   The data mode, "refer to the DA command"
//	                (590:1546-1548).
//	byte 20    P7   FOUR tone modes, the fourth being "3: Cross Tone ON"
//	                (590:1549-1553).
//	byte 28    P11  FILTER A/B (590:1560-1563) — THE ONE AXIS THE TWO ROWS
//	                DIFFER ON, because the note beside it is scoped to one
//	                of them: "* In firmware version 1.xx of TS-590S, always
//	                \"0\"." (590:1478). A14. The SG's byte is live; the S's
//	                is accepted either way on a read, and what a WRITE may
//	                carry is the driver's question.
//	bytes 39-40 P14 "00: FM Normal / 01: FM Narrow" (590:1569-1571), and
//	                nothing else. What the byte means outside FM is
//	                unprinted, which is A23.
//	byte 41    P15  The channel lockout (590:1572-1574).
//	P5's legend     MR/MW P5 carries none of its own — "refer to the MD
//	                command" (590:1544-1545) — so the memory mode
//	                vocabulary IS MD's, ten nibbles of which eight name a
//	                mode (590:1353-1363). Nibbles 0 and 8 are "None
//	                (setting failure)" (590:1353, 590:1362) and are absent
//	                from the legend; nibble 0 is still the empty channel's
//	                own answer value (590:1492-1493), which is A18a and
//	                which the record parser tests for before it reaches the
//	                legend.
//	slot space      000-099 ordinary memory (590:1341); 100-109 the
//	                section-defined channels P00 ~ P09 (590:1345); 110-119
//	                the SG's extension channels (590:1346-1347) — THE
//	                SECOND AXIS THE ROWS DIFFER ON. The book never states
//	                the S's own ceiling, which is A12, so the S stops at
//	                109.
//	printed-fixed   THIRTEEN of the 47 parameter bytes, in three runs: P10
//	                (590:1558-1559), P12 (590:1565-1566) and P13
//	                (590:1567-1568). Bytes 4, 28 and 41 all carry meanings
//	                on this book, so this row's set has no fourth run.
//
// THE CODEC'S SLOT DOMAIN AND THE DRIVER'S PUBLISHED BANKS ARE TWO
// DIFFERENT QUESTIONS. The SG layout DECLARES 110-119 because the book
// prints them (590:1346-1347, documented fact) and a front-panel recall of
// E00 must parse rather than be refused; A12 is the S's UN-stated ceiling,
// not this declaration. Stuart ruled on 05/09/2026 (decision row 6) that
// the same ten slots are OMITTED from the driver's published Banks until
// A11 lifts, because what an extension channel IS is never explained
// anywhere in the book. The codec admits what the book prints; the driver
// publishes what is confirmed.
//
// # This package's share of the errata schedule
//
// The schedule itself lives once, in core/kw/doc.go, with the ASSUMED
// register. These are the rows this book raises; each is named there in
// full and is listed here so that a reader of this package finds them:
//
//	E1   The MR Read chart's terminator cell prints ':' (590:1442). It is
//	     evidence leg G's FINDING F1 as well, and it is RECORDED, NOT
//	     RESOLVED: no vector with a ';' terminator is emitted for that
//	     frame, and core/kw's gate refuses all four of leg G's MR read
//	     vectors on the ground that they carry no terminator at all.
//	E2   "TS-590SG extension channel numbers E00 ~ P09" for E00 ~ E09
//	     (590:1346), which is the very sentence this package's SG slot
//	     range rests on.
//	E4   SC Set's parameter is P1 and the Answer's first is P2
//	     (590:1974 vs 590:1981). Neither is built here.
//	E5   VR renumbers P1 to P2 between Set and Answer and reports a
//	     different datum (590:2476, 590:2483). Not built here.
//	E7   MR's and MW's firmware notes on P11 say the same thing in
//	     different words, and MW's prints a mismatched quotation mark
//	     (590:1478 vs 590:1564). It is the datum byte 28's axis rests on,
//	     so either line may be cited and neither line's text may appear
//	     under the other's number.
//	E16  The EX Set/Answer charts print a ';' at a nominal position
//	     although P5 is declared "(variable length)" (590:547,
//	     590:556-560) — the chart is illustrative, not a width.
//	E19  The MW erase note reads "If you do not specify one digit in P16"
//	     (590:1579-1581), whose intended sense is almost certainly "no
//	     digits". This programme builds no erase frame.
//	E20  RD and RU are opposites and both are described as increasing the
//	     scan speed (590:1846, 590:1848). Neither is built here.
//
// AND ONE THAT IS NOT A DEFECT: E6, the SS/MC numbering trap. SS numbers
// the section channels 0-9 where MC numbers them 100-109 (590:2181 vs
// 590:1345), which is a cross-command convention the book is entitled to
// have — but a transcriber who carries SS's numbers into an MC frame writes
// the wrong channel.
//
// The ASSUMED register lives once, in core/kw/doc.go, and is not restated
// here.
package ts590
