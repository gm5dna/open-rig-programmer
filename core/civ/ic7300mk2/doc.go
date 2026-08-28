// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7300mk2 holds the Icom IC-7300MK2's CI-V profile: the record
// geometry, the enum vocabularies, the name policy and the two addresses,
// as one civ.ProfileConfig literal. It is DATA ONLY — no driver, no fake,
// no registration, no session, no wire. The package cannot register itself
// with the application: SupportedModels derives solely from
// internal/wiring's driver table, which this package never touches.
//
// # Provenance
//
// Everything here is a reading of the Icom IC-7300MK2 CI-V REFERENCE GUIDE
// — a 27-page document, not a full manual — by way of the four quarantined
// evidence legs frozen in testdata/ (the field ledger L, the semantic
// transcription B, the geometry witness W and the golden vectors G), and
// of docs/superpowers/icom-matrices/ic7300mk2-capability-matrix.md, which
// cites the guide page by page.
//
// NO IC-7300MK2 HAS EVER BEEN ASKED ANYTHING by this project. Not one byte
// here has been sent to or captured from a radio.
//
// # THIS PACKAGE BORROWS NOTHING FROM ITS SIBLING
//
// The IC-7300's 180-page full manual is silent about the MK2 in every
// respect, and this 27-page guide is silent about the IC-7300: the string
// "7300" occurs in it five times and every one of them is "IC-7300MK2".
// Both matrices' §4 say in terms that no assumption in one covers the
// other and no lift in one lifts anything for the other.
//
// So where the two models' layouts agree — and they agree for every byte
// through offset 28 — that is a FINDING, arrived at from two documents
// independently, and never an input. Nothing in this package may be
// justified by pointing at core/civ/ic7300, and every register entry below
// names a capture ON AN IC-7300MK2.
//
// # The two numbers, and which one this profile carries
//
// | CI-V default address       | B6h |
// | Controller address         | E0h |
// | Channel-address width      | 2 bytes |
// | RECORD-ONLY length         | 45  |
// | Data-area length (record + address) | 47 |
// | Whole 1A 00 set frame on the wire   | 54 |
//
// civ.ProfileConfig.Layouts[i].Length denotes the RECORD-ONLY figure, per
// spec Erratum 1. The tier spec's D6 row for this pair gives "41 B / 47 B"
// — 47 IS THE DATA-AREA ACCOUNTING, the record plus the two channel-address
// bytes, and it is the number a reader counting printed indices gets
// (2+1+5+2+1+3+3+14+16 = 47). The difference between D6's 47 and this
// profile's 45 is exactly those two bytes: the same nine printed groups,
// counted to a different boundary. A fingerprint built on 47 is a
// different test.
//
// No total is printed anywhere in this guide (matrix §3.11): the 1A 00
// diagram carries no byte count, the command table's Data cell for 1A* 00
// reads only "See p. 17.", and no prose states a record length. 45 is a
// derivation by field arithmetic and is registered as ASSUMED below.
//
// # MaxFrame is 64, and it is a CHOICE
//
// civ.DefaultMaxFrame is 256 and civ's V9 requires only 7 + 2 + 45 = 54.
// 64 is neither. It is the smallest round bound that admits BOTH siblings'
// longest frames — 54 here and 48 on the IC-7300 — so that a foreign
// 48-byte answer arriving at this profile fails as a
// *civ.RecordLengthError{Want:[45], Got:39}, which is the length
// fingerprint spec D3.2 asks for, rather than being pre-empted by
// ErrFrameTooLong before a record ever exists.
//
// # Field positions are ACCUMULATED WIDTHS, never the printed index
//
// The numbered band on PDF p.17 draws its indices in two glyph classes:
// OUTLINED (a black numeral in a thin circle) for ①, ② through ⑱ ~ ㉝, and
// FILLED (a white numeral reversed out of a solid black disc) for ❹ and ⓱,
// on the one bracket between ⑮ ~ ⑰ and ⑱ ~ ㉝. The printed index is
// therefore not single-valued, and the sequence runs backwards once.
//
// pdftotext FLATTENS BOTH CLASSES TO IDENTICAL BARE DIGITS, which turns
// the NOTE box's sentence "The same data as ④ ~ ⑰ are stored in ❹ ~ ⓱."
// into a tautology: field identity in this record is NOT recoverable from
// the text layer, however complete the numerals look there. All three legs
// that read this page re-rendered it and recorded the same two classes
// independently.
//
// Every FieldSpan.Offset in profile.go is consequently the sum of the
// widths before it. That derivation carries its own assumption — that the
// wire order is the diagram's left-to-right order, past the duplicated
// block included — registered below as D5 entry 5.
//
// # ③: the SELECT nibble is mapped, the SPLIT nibble is not
//
// PDF p.17's ③ inset divides one byte with a dotted centre line and labels
// the halves with two up-arrows: SPLIT on the LEFT (high) nibble, SELECT on
// the RIGHT (low) one. This model's B leg records that from the arrow
// labels directly.
//
// The LOW nibble is mapped onto civ.FieldSelect with the four printed
// values (0=OFF, 1=★1, 2=★2, 3=★3, spelled OFF/SEL1/SEL2/SEL3 here). The
// HIGH nibble — the split flag — is UNMAPPED, and the layout's
// full-length, all-zero Fixed template is what declares it so. That is the
// tier ruling E6, and the consequence is worth stating plainly:
//
//   - civ's decodeRecord IGNORES every nibble no span maps and
//     encodeRecord WRITES THE TEMPLATE there, so a driver that neither
//     carried ③'s high half through nor refused would clear a user's split
//     flag on every write-back, unseen by every layer above.
//   - Fixed[0]&0xF0 == 0x00 — Split OFF — IS this model's unmapped-region
//     contract. A Split-ON channel READS normally and is REFUSED on write,
//     loudly, rather than silently corrected. That cost is accepted for
//     this tier.
//
// Also printed beneath that inset, and a constraint the SCAN bank's driver
// carries rather than this profile: "ⓘ Set 00 for P1 and P2."
//
// # The mode byte has a hole in it, and where the wording parts company
//
// PDF p.16's "• Operating mode" table prints 00–05, then 07, then 08. 06
// is printed nowhere and no note explains the hole; the matrix grades that
// a MANUAL-EVIDENCED absence (§3.16 A7). This package invents no name for
// 06.
//
// THE WORDING GAP, RECORDED RATHER THAN SMOOTHED. The matrix's A7 says a
// record read back with 06 in byte ⑨ "is an unknown mode, not a parse
// error". The civ codec has no third state to put that in: an enum value
// no span declares IS a *civ.ParseError, naming the byte and the offset,
// and the read fails rather than yielding a record with a hole in it. The
// two statements agree on the substance — nothing is invented for 06, and
// no mode is reached by interpolation — and differ on what the failure is
// called. This package refuses; a future tier that wanted a third state
// would have to add one to core/civ, with its own round-trip rules.
//
// # THE ASSUMED REGISTER
//
// CITE THESE ENTRIES BY NAME, NEVER BY POSITION (core/civ/doc.go's
// convention). Every entry is scoped to the IC-7300MK2 ALONE and names ONE
// capture ON AN IC-7300MK2.
//
// The rows whose home is "D5 entry N" are THIS MODEL'S INSTANCE of the
// tier-wide recurring entry, not a shared one.
//
//   - `1A 00 <addr>` read form — D5 entry 1. The frame this profile builds
//     is FE FE B6 E0 1A 00 <ch-hi> <ch-lo> FD. The envelope, the command
//     and the channel address in the data area are each MANUAL-EVIDENCED
//     (PDF pp.3, 6 and 17), and PDF p.4's asterisk NOTE gives the general
//     read rule; what is ASSUMED is that a bare 1A 00 <addr> with no
//     further data is what THIS radio accepts as a read and answers with
//     the record. No worked read example is printed for 1A 00.
//     LIFT: MK2-R1 — send FE FE B6 E0 1A 00 00 01 FD to an IC-7300MK2
//     whose channel 01 is written and record the answer frame byte for
//     byte.
//
//   - name space is 0x20 — D5 entry 3, the SPACE half. PDF p.18's
//     usable-characters note names "(space)" among a memory name's usable
//     characters — MANUAL-EVIDENCED — but neither p.18 table prints a row
//     for it, so its CODE is not given for this command. THE TRAP THIS
//     MODEL SETS: the byte 20 is printed for a space TWICE in this guide
//     and neither time for the memory name (PDF p.16 under command 17, and
//     PDF p.19 under 1A 02, each with its own charset table). Neither is
//     evidence about 1A 00.
//     LIFT: MK2-R9 — enter a memory name containing an internal space from
//     the front panel, read the record with 1A 00, and record bytes
//     ⑱ ~ ㉝.
//
//   - name pad is 0x20 — D5 entry 3, the PAD half, graded separately. The
//     field is a fixed sixteen bytes and a name may be shorter; what fills
//     the remainder is printed nowhere. This guide gives no padding rule,
//     no termination rule and no "trailing spaces are unnecessary" note
//     for 1A 00 — it gives such a note for the KEYER memory (PDF p.19),
//     which is a different command and is not carried across.
//     LIFT: MK2-W3 — write a name shorter than sixteen characters padded
//     with 0x20, read it back, and compare byte for byte.
//
//   - full record including ❹ ~ ⓱ required on write — D5 entry 4. The
//     p.17 NOTE box's third bullet is advisory wording on a fixed-width
//     field — "we recommend", not "you must" — and the diagram draws the
//     block as part of one continuous record with no optional marking.
//     That a set frame is REFUSED, or silently mis-stores, if the block is
//     omitted or left inconsistent is ASSUMED. This profile can only emit
//     the whole 45 bytes, and mirrors ④ ~ ⑰ into ❹ ~ ⓱ by mapping the two
//     to the same field ids.
//     LIFT: MK2-W2 — send one set frame with the duplicated block zeroed
//     while the RX block is populated, and record both the acknowledgement
//     and the read-back.
//
//   - ❹ ~ ⓱ repeats ④ ~ ⑰ in the same internal order — D5 entry 5. The
//     NOTE box establishes that the block holds a second copy; that its
//     bytes run in the SAME order as the first copy, and that the record's
//     wire order is the band's left-to-right order past it, is ASSUMED.
//     Every offset in profile.go rests on this.
//     LIFT: MK2-R1 — the same capture, read for the byte positions rather
//     than for the answer's existence.
//
//   - record total 45 (data area 47) — D5 entry 6. Derived by field
//     arithmetic from the printed index ranges; no total is printed.
//     LIFT: MK2-R1 — read one occupied channel and count the bytes of the
//     answer's data area.
//
//   - `ic7300mk2-default-baud` — civ PROFILE register. NO FACTORY DEFAULT
//     IS PRINTED. PDF p.3 says the rate is configured in Set mode and
//     points at the BASIC MANUAL, adding that the setting is "Required
//     only when the controller (PC) is connected to the [REMOTE] jack" —
//     which is evidence that it governs the REMOTE jack rather than the
//     USB link this driver uses.
//     LIFT: MK2-R6 — on a factory-reset IC-7300MK2, attempt 19 00 at each
//     candidate rate and record which answers. Scoped to the default
//     alone: a sweep observes the rate the radio is CURRENTLY using and
//     nothing about the rates it could be set to.
//
//   - `ic7300mk2-baud-list` — civ PROFILE register. THE ONLY RATES THIS
//     GUIDE PRINTS ARE THE THREE ROWS OF AN FE-COUNT TABLE, and that table
//     is a WAKE-UP-COMMAND TABLE, NOT A SUPPORTED-RATE LIST: PDF p.16's
//     "• Turning the transceiver ON / Command: 18 01" says "To send this
//     command through the [REMOTE] jack, you must enter multiple "FE"
//     characters. The required number of "FE" entries depends on the baud
//     rate.", and gives 19200 bps: 25 "FE"s, 9600 bps: 13, 4800 bps: 7.
//     Three worked examples for one command are not a rate list.
//     LIFT: MK2-R21 — read the Set-mode CI-V baud item on the front panel
//     of a factory-reset radio and record every value it offers. A RATE
//     SWEEP CANNOT LIFT THIS: it observes only the rate in use.
//
//   - `ic7300mk2-auto-baud-absent` — civ PROFILE register. This guide
//     prints no AUTO setting and does not say whether the USB link is
//     rate-agnostic; its absence is ASSUMED. (The IC-7300 ships set to
//     Auto — which is a fact about the IC-7300 and is not carried across.)
//     LIFT: MK2-R21 — the same panel read, recording whether AUTO is among
//     the offered values.
//
//   - `ic7300mk2-address-range` — civ PROFILE register. That B6 is a
//     DEFAULT and that the address is user-changeable is
//     MANUAL-EVIDENCED (PDF p.3 calls it a default twice and lists it
//     among the Set-mode items). The PERMITTED RANGE is not printed: this
//     guide gives no menu path and no range.
//     LIFT: MK2-R22 — read the Set-mode CI-V address item on the front
//     panel and record its permitted range.
//
//   - `ic7300mk2-scan-edge-record-layout` — civ PROFILE register. THIS
//     GUIDE NEVER STATES WHETHER A P1/P2 RECORD CARRIES THE SAME 45 BYTES.
//     It gives the two addresses (01 00, 01 01) and one rule about them
//     ("ⓘ Set 00 for P1 and P2." for field ③), and says nothing else — not
//     whether the sixteen-byte name field is present, not whether the
//     fourteen-byte duplicated block is, not whether a read returns 45
//     bytes at all (§3.16 A5). The consequence names what silence means
//     during a capture: a SHORT or FA answer from 1A 00 01 00 would be a
//     fact about the scan-edge bank, not a fault.
//     LIFT: MK2-R10, capture `ic7300mk2-scan-edge-p1-read` — read P1 and
//     record the answer's length and bytes.
//
//   - `ic7300mk2-tone-tx-encoding` — civ PROFILE register. THE
//     "⑫ ~ ⑭ Repeater tone frequency setting" HEADING IS PRINTED WITH
//     NOTHING BENEATH IT (§3.16 A6): no value, no code, no encoding
//     statement and no cross-reference of its own. The only
//     ⓘ See "Repeater tone/tone squelch frequency settings." (p. 23) in
//     that area is set under the NEXT heading, ⑮ ~ ⑰. This profile assumes
//     p.23's BCD form for ⑫ ~ ⑭ — three bytes, most significant pair
//     first, in tenths of a hertz — which is a carry-across a reader could
//     easily make without noticing it was never pointed at ⑫ ~ ⑭.
//     LIFT: MK2-R17, capture `ic7300mk2-tone-encoding-ch01` — set one
//     channel's repeater tone from the panel, read the channel back, and
//     record bytes ⑫ ~ ⑭ verbatim.
//
//   - `ic7300mk2-min-frequency` / `ic7300mk2-max-frequency` — civ PROFILE
//     register. The CEILING, 79 999 999 Hz, is the ENCODING ceiling and is
//     MANUAL-EVIDENCED (PDF p.16: 10 MHz digit 0 ~ 7, with the 1 GHz and
//     100 MHz digits printed fixed 0); that the radio's own TUNING ceiling
//     is at or below it is ASSUMED. THE FLOOR IS NOT PRINTED ANYWHERE: the
//     per-digit ranges admit 0 Hz, and PDF p.19's band-code table
//     describes band-stacking registers rather than the memory record and
//     is not carried across. Borrowing the IC-7300's 30 kHz would be
//     exactly the cross-model contamination both matrices' §4 forbid.
//     LIFT: MK2-R15, capture `ic7300mk2-tuning-range` — read the radio's
//     printed or panel-reported tuning range.
//
//   - `ic7300mk2-no-further-banks` — civ PROFILE register, NARROWED by
//     matrix Erratum 2 to what a capture can bound. It does NOT claim that
//     no bank exists beyond MEM and the two scan edges — that is a
//     universal negative no capture can settle. It claims that THE FOUR
//     PROBED channel-address forms outside 00 01–00 99, 01 00 and 01 01
//     are not answered as records on this model. The documentary evidence
//     is unchanged and is careful: PDF p.17's ①, ② legend and PDF p.4's 08
//     row each list exactly three address forms and no fourth.
//     LIFT: MK2-R19, capture `ic7300mk2-bank-probe` — send a 1A 00 read
//     for each of 00 00, 01 02, 02 00 and 03 00, and record for each
//     whether a record comes back or the radio answers FA.
//
// # D13 — 0x60 is a legal name byte whose GLYPH is not established
//
// PDF p.18's Symbols table, fifth data row, draws the SAME GLYPH — a right
// single quotation mark — in both its left and right Character cells, with
// ASCII code cells reading 27 and 60 respectively. Confirmed at 700 %
// enlargement in two halves, and recorded independently by the B leg as
// its STOP 2. Neither glyph is drawn as a grave accent.
//
// So 0x60 IS in this profile's charset — the table prints it as a usable
// code — and its identity is UNKNOWN. IT MUST NEVER BE SILENTLY RENDERED
// AS 0x27. Anything above this package that displays a name byte-for-byte
// is displaying an unresolved character, and a capture is what settles it.
// Recorded as printed and not resolved; it is an erratum candidate against
// the DOCUMENT rather than against the matrix.
//
// # Open questions, named as such
//
// A. RESOLVED, and no longer open — see core/civ/ic7300/doc.go's own
// paragraph. Byte ③'s two halves are settled by E6 and D14: the split half
// is unmapped under the Fixed template, a non-conforming unmapped region
// is REFUSED on write, and REV 1's per-slot cache is struck.
//
// B. NOT OPEN ON THIS MODEL. The IC-7300 carries an open question about
// its per-symbol name codes, because its manual enumerates the glyphs and
// prints only three of the codes. THIS document prints an ASCII code
// against every symbol and the B leg transcribed all thirty-two, so
// nothing here is derived. The two models' charsets coinciding is a fact
// about ASCII, not evidence crossing the sibling boundary.
//
// # Non-goals, restated from core/civ/doc.go
//
// THERE IS NO CLEAR/ERASE BUILDER, NO TRANSCEIVE-SET BUILDER AND NO 1A 05
// MENU SURFACE. This guide prints two clear forms and this package builds
// neither, its gate admits neither, and no test in it may construct one
// except to assert the gate's refusal. P1 and P2 cannot be cleared on this
// radio at all (PDF p.4's 0B row: "ⓘ P1 and P2 cannot be cleared."), which
// this tier honours by shipping no erase path whatsoever.
//
// It also prints, on PDF p.16, a worked 18 01 power-ON frame with fifteen
// leading FE bytes. THAT FRAME IS NEVER CONSTRUCTIBLE HERE: 18 01 is not
// one of the three grammars the gate admits, and the golden vectors carry
// it as a NEGATIVE vector precisely so the refusal has a witness.
//
// Init writes NOTHING to this radio: civ's Framing.InitSequence() is empty
// and transceive broadcasts are excluded by ADDRESS FILTERING in
// civ.FrameAccumulator, never by writing a setting to the radio.
package ic7300mk2
