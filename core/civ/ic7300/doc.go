// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7300 holds the Icom IC-7300's CI-V profile: the record
// geometry, the enum vocabularies, the name policy and the two addresses,
// as one civ.ProfileConfig literal. It is DATA ONLY — no driver, no fake,
// no registration, no session, no wire. The package cannot register itself
// with the application: SupportedModels derives solely from
// internal/wiring's driver table, which this package never touches.
//
// # Provenance
//
// Everything here is a reading of the Icom IC-7300 FULL MANUAL (the
// 180-page edition whose section 19 carries the CI-V reference), by way of
// the four quarantined evidence legs frozen in testdata/ — the field
// ledger (L), the semantic transcription (B), the geometry witness (W) and
// the golden vectors (G) — and of
// docs/superpowers/icom-matrices/ic7300-capability-matrix.md, which cites
// the manual page by page.
//
// NO IC-7300 HAS EVER BEEN ASKED ANYTHING by this project. Not one byte in
// this package has been sent to or captured from a radio. Green tests here
// mean that two independently written readings of one manual agree with
// each other and with this code, which is why the ASSUMED register below
// exists at all.
//
// # The two numbers, and which one this profile carries
//
// | CI-V default address       | 94h |
// | Controller address         | E0h |
// | Channel-address width      | 2 bytes |
// | RECORD-ONLY length         | 39  |
// | Data-area length (record + address) | 41 |
// | Whole 1A 00 set frame on the wire   | 48 |
//
// civ.ProfileConfig.Layouts[i].Length and BuildMemorySet's <record>
// argument denote the RECORD-ONLY figure, per spec Erratum 1. The
// data-area figure — 41 — is what a reader counting printed indices from
// ① gets, and the tier spec's D6 row quotes it; the two agree exactly once
// the boundary is named. A FINGERPRINT BUILT ON 41 IS A DIFFERENT TEST,
// and this profile's is built on 39.
//
// Neither number is printed. Icom prints no record total anywhere in
// section 19 (matrix §3.11), so 39 is a derivation by field arithmetic:
// 1 (③) + 5 (④–⑧) + 2 (⑨, ⑩) + 1 (⑪) + 3 (⑫–⑭) + 3 (⑮–⑰) + 14 (❹–⓱) +
// 10 (⑱–㉗). It is registered as ASSUMED below.
//
// # MaxFrame is 64, and it is a CHOICE
//
// civ.DefaultMaxFrame is 256 and civ's V9 requires only
// 7 + 2 + longest layout, i.e. 48 here. 64 is neither of those. It is the
// smallest round bound that admits BOTH siblings' longest frames — 48 on
// this model and 54 on the IC-7300MK2 — so that a foreign 54-byte answer
// arriving at this profile fails as a *civ.RecordLengthError{Want:[39],
// Got:45}, which is the length fingerprint spec D3.2 asks for, rather than
// being pre-empted by ErrFrameTooLong before a record ever exists. A tight
// bound would refuse the wrong-sibling answer for the wrong reason and
// tell the user nothing about which radio is on the other end.
//
// # Field positions are ACCUMULATED WIDTHS, never the printed index
//
// The strip on PDF p.169 (folio 19-11) prints indices 4–17 TWICE: once as
// black numerals in outlined circles over discrete cells (④–⑰), and once
// as white numerals reversed out of solid black discs over one undivided
// dashed-edged region (❹–⓱). The printed index is therefore not
// single-valued — index 4 names both record byte 4 and record byte 18 —
// and the sequence runs backwards once, ⑮–⑰ being followed by ❹–⓱ and
// then by ⑱–㉗. All four IC-7300 legs stopped on this and none resolved
// it; nor does the matrix (§3.15).
//
// So every FieldSpan.Offset in profile.go is the sum of the widths before
// it. A table keyed on the printed index would be ambiguous, and a reader
// editing this layout must accumulate widths rather than read a numeral.
// That derivation carries its own assumption — that the record's WIRE
// order is the diagram's left-to-right order, including past the
// duplicated block — registered below as D5 entry 5.
//
// # ③: the SELECT nibble is mapped, the SPLIT nibble is not
//
// Byte ③ carries SPLIT in its HIGH nibble and SELECT in its LOW one; the
// assignment is a leader-order reading traced by eye at 400 and 500 dpi (W
// hazard (c)), and the leaders' printed top-to-bottom order is the REVERSE
// of the nibbles' left-to-right order.
//
// The LOW nibble is mapped onto civ.FieldSelect with the four printed
// values (0=OFF, 1=★1, 2=★2, 3=★3, spelled OFF/SEL1/SEL2/SEL3 here). The
// HIGH nibble — the split flag — is UNMAPPED, and the layout's
// full-length, all-zero Fixed template is what declares it so. That is a
// tier ruling (enablers E6), not a local choice, and it has a consequence
// worth stating plainly:
//
//   - civ's decodeRecord IGNORES every nibble no span maps, and
//     encodeRecord WRITES THE TEMPLATE there. A driver that neither
//     carried ③'s high half through nor refused would therefore clear a
//     user's split flag on every write-back, and no layer above could see
//     it happen.
//   - Fixed[0]&0xF0 == 0x00 — Split OFF — IS this model's unmapped-region
//     contract, and it is what each driver's E6 check compares a
//     just-read record against. A Split-ON channel READS normally and is
//     REFUSED on write, loudly, rather than silently corrected. That cost
//     is accepted for this tier.
//
// Nothing here reclassifies the split flag as a SELECT value: an
// eight-value whole-byte enum was REV 1's design and is overruled.
//
// # The mode byte has a hole in it, and this package does not fill it
//
// PDF p.167 (folio 19-9), "① Operating mode", prints 00: LSB, 01: USB,
// 02: AM, 03: CW, 04: RTTY, 05: FM, 07: CW-R, 08: RTTY-R. **06 IS NOT
// PRINTED**, and the two spare cells beside it are struck through with a
// diagonal rule; the matrix grades that a MANUAL-EVIDENCED ABSENCE
// (§3.16 A7). This package invents no name for 06. A record carrying it
// fails the read with a *civ.ParseError naming the byte and the offset —
// an honest refusal rather than a guess that would put the radio in a mode
// the user never chose. The same holds for any other value no enum above
// declares.
//
// # THE ASSUMED REGISTER
//
// CITE THESE ENTRIES BY NAME, NEVER BY POSITION (core/civ/doc.go's
// convention). A positional citation is correct only until somebody adds
// or reorders an entry, and it then points silently at the wrong
// assumption rather than failing.
//
// Every entry below is scoped to the IC-7300 ALONE and names ONE capture
// ON AN IC-7300. Nothing here covers the IC-7300MK2, and no lift here
// lifts anything for that sibling: the IC-7300's full manual is silent
// about the MK2 in every respect (matrix §4), and the MK2's own document
// is silent about this radio.
//
// The rows whose home is "D5 entry N" are THIS MODEL'S INSTANCE of the
// tier-wide recurring entry, not a shared one.
//
//   - `1A 00` read-request form — D5 entry 1. The request this profile
//     builds is FE FE 94 E0 1A 00 <ch-hi> <ch-lo> FD, nine bytes, with no
//     data area beyond the channel address. The document prints no read
//     request for 1A 00 or for any 1A sub-command; what it prints is the
//     command table's `Send/read memory contents` row, the asterisk
//     footnote defining `1A*` as bidirectional, and a clear recipe that
//     specifies a truncated form by listing exactly which record fields
//     are present. A read lists none, and must still name a channel.
//     LIFT: Stage R capture `ic7300-read-request` — send exactly
//     FE FE 94 E0 1A 00 00 01 FD to an IC-7300 whose channel 01 is
//     written, and record whether a 1A 00 answer returns rather than an
//     FA.
//
//   - name space code 0x20 — D5 entry 3, the SPACE half. Neither printed
//     character table on PDF p.168 has a row for a space, while the same
//     page's command table says of a memory name `All characters are
//     usable.` The only space code this document prints anywhere is under
//     the different heading `• Codes for CW message contents / Command :
//     17` (PDF p.171), which is scoped to command 17 and is not treated as
//     documenting this field. 0x20 is also the ASCII code point, and the
//     ranges this field IS given are ASCII code points.
//     LIFT: Stage R capture `ic7300-name-space` — read back a channel
//     whose name already contains a space at a known position and record
//     the byte at that position.
//
//   - name pad 0x20, and the field is always ten bytes — D5 entry 3, the
//     PAD half, graded separately because neither capture can lift the
//     other. `Up to 10 characters.` is a maximum on the CONTENT; the
//     document names no padding character, no terminator and no shorter
//     width, so a nine-character name has an unstated tenth byte. This
//     profile pads with 0x20 and trims 0x20, and civ's V4 is why 0x20 must
//     also be in the charset.
//     LIFT: Stage R capture `ic7300-name-pad` — set a channel's name to
//     nine characters from the panel, read the channel back over CI-V, and
//     record the tenth byte.
//
//   - duplicated TX block mandatory on write — D5 entry 4. The NOTE box on
//     PDF p.169 says the same data as ④–⑰ are stored in ❹–⓱, that the
//     block is used for transmit when Split is ON, and — advisory language
//     on a fixed-width field — `We recommend that you set the same data as
//     ④–⑰.` That the FULL record is mandatory on write, i.e. that a set
//     with a short data area is refused rather than partially applied, is
//     ASSUMED. This profile can only emit the whole 39 bytes.
//     LIFT: Stage W capture `ic7300-short-record-write` — send a 1A 00 set
//     whose data area stops after ⑮–⑰ (25 record bytes, the TX block and
//     name omitted) and record whether FB, FA or silence comes back.
//
//   - wire order = diagram order past the duplicated block — D5 entry 5.
//     Icom's printed field indices are logical rather than positional, and
//     on this model the strip is the only thing that fixes the order. That
//     the record's WIRE order is the diagram's left-to-right order —
//     INCLUDING past ❹–⓱, where the printed indices go backwards — is
//     ASSUMED, and every offset in profile.go rests on it.
//     LIFT: Stage R capture `ic7300-wire-order` — set a channel from the
//     panel to values that make every field distinguishable (a frequency,
//     a mode, a repeater tone and a tone squelch that differ from one
//     another, and a ten-character name), read the channel back, and
//     record which byte offsets carry which values.
//
//   - record total length, 39 derived — D5 entry 6. Derived by field
//     arithmetic from the printed index ranges, never printed. The two
//     related figures are 41 (data area) and 48 (whole frame); see the
//     table above.
//     LIFT: Stage R capture `ic7300-record-length` — read a known-occupied
//     channel on an IC-7300 and count the bytes between the two
//     channel-address bytes and the terminating FD.
//
//   - `ic7300-default-open-baud` — civ PROFILE register. THIS RADIO HAS NO
//     NUMERIC FACTORY-DEFAULT CI-V RATE: both baud items ship set to
//     `Auto` (PDF pp.126–127), which is a negotiation and not a rate, and
//     the document does not say how Auto decides, how many frames it needs
//     or what it does with a rate outside the printed options. Whatever
//     numeric rate a driver opens at is therefore ASSUMED. The profile
//     carries no rate of its own — the capability lives on the driver —
//     and the entry is recorded here because the choice is the model's,
//     not the driver's.
//     LIFT: Stage R capture `ic7300-open-rate` — open at the chosen rate
//     against an IC-7300 left at its factory `Auto` and record whether the
//     19 00 read is answered.
//
//   - `ic7300-name-charset-symbol-codes` — civ PROFILE register, and the
//     one entry this package's own transcription created. Matrix §3.9
//     quotes the three printed RANGES (A–Z 41–5A, a–z 61–7A, 0–9 30–39)
//     and the Symbols table's endpoints (! 21, ~ 7E, @ 40) and enumerates
//     the 32 symbol GLYPHS, but does not quote the other thirty codes; the
//     B leg records only the cross-reference. Taking those glyphs at their
//     ASCII code points is a DERIVATION, not a transcription. See open
//     question B below.
//     LIFT: Stage R capture `ic7300-name-symbol-readback` — set a channel
//     name containing each symbol from the panel and read back the bytes.
//
// # Open questions, named as such
//
// A. RESOLVED, and no longer open. REV 1 of the plan carried byte ③ as an
// open question with a whole-byte-enum-plus-cache interim. The enablers'
// E6 names this very byte and rules otherwise: the split half is unmapped
// under the Fixed template, a non-conforming unmapped region is REFUSED on
// write, and the cache is struck. The alternative REV 1 offered — a new
// civ.FieldSplit plus a spec.Field — is rejected for v1 by E6's own
// reasoning, an opaque carrier conflicting with the gate's byte-identity
// re-encode invariant. Revisit post-v1, with hardware.
//
// B. OPEN. The IC-7300's per-symbol name character codes are a
// derivation, registered as `ic7300-name-charset-symbol-codes` above. The
// MK2 has no such gap — its B leg transcribes every code — and the two
// models' charsets agreeing is a coincidence of ASCII, not evidence
// borrowed across the sibling boundary.
//
// # Non-goals, restated from core/civ/doc.go
//
// THERE IS NO CLEAR/ERASE BUILDER, NO TRANSCEIVE-SET BUILDER AND NO 1A 05
// MENU SURFACE, here or anywhere in this tier. This document prints TWO
// clear forms — a 1A 00 set with ③ = "FF", and command 0B — and this
// package builds neither, its gate admits neither, and no test in it may
// construct one except to assert the gate's refusal. A frame no builder
// can name is a frame civ.AllowedCommand refuses by construction rather
// than by a rule somebody could later relax.
//
// Init writes NOTHING to this radio: civ's Framing.InitSequence() is empty
// and transceive broadcasts are excluded by ADDRESS FILTERING in
// civ.FrameAccumulator, never by writing a setting to the radio.
package ic7300
