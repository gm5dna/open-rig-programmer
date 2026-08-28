// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic705 holds the Icom IC-705's CI-V dialect: one
// civ.ProfileConfig literal describing the `1A 00` memory record, and the
// four quarantined evidence legs under testdata/ that every test here
// binds it to. It is DATA ONLY — no driver, no fake, no registration, no
// session, no wire. The package cannot register itself with the
// application: SupportedModels derives solely from internal/wiring's
// driver table, and registration is a later wave's commit on another
// branch.
//
// Profile() is the only exported symbol. internal/guards rule 4 refuses a
// core/civ subpackage that references civ.BuildMemorySet, and that fence
// is exact rather than a prefix: every future model package is data-only,
// and a write-builder call site appearing in one is precisely the
// regression it exists to catch.
//
// # Provenance
//
// Everything here comes from the Icom IC-705 CI-V REFERENCE GUIDE,
// revision A7560-8EX-6 (© 2020–2023 Icom Inc., Jan. 2023), 31 PDF pages,
// SHA-256 36876db53a4dec7a9d74133ac4546bd161bcb6d56ee7c79668ff00cf1f92ea9c,
// at docs/fixtures-private/manuals/ic705_civ_rev6.pdf — gitignored, so
// every citation below is a citation and not a link.
//
// THE FOLIO OFFSET IS printed page = PDF page − 1 (corrected 23/08/2026;
// docs/fixtures-private/manuals/ic705-manual-provenance.md §Erratum, which
// records that earlier notes said −2 and were wrong). EVERY PAGE CITATION
// IN THIS PACKAGE GIVES THE PDF PAGE FIRST, with the folio in brackets.
// The whole memory record comes from PDF p.19 (folio 18), `• Memory
// content`, `Command: 1A 00`.
//
// NO IC-705 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT. Every statement
// in this package is a reading of a document. There is no observation
// file, no correction file and no captured frame, which is why the ASSUMED
// register below exists at all and why each of its entries names the one
// hardware observation that would lift it.
//
// The evidence is four INDEPENDENT legs, each an agent reading the same
// rendered pages without sight of the others' work, and all four are
// frozen by hash in freeze_test.go:
//
//   - testdata/field-ledger.csv/.md — leg L, every printed index with its
//     verbatim legend label and the visual anchor it was read from.
//   - testdata/geometry-witness.csv/.md — leg W, the MEASURED byte
//     positions, counted as cells on a raster rather than read off the
//     printed numerals.
//   - testdata/transcription-b.csv/.md — leg B, labels, widths and value
//     lists.
//   - testdata/vectors.golden, golden-assumptions.csv, provenance.md — leg
//     G, three hand-derived frames with a per-byte assumption register.
//
// # The two numbers, and why both are stated
//
// Spec Erratum 1: a civ.Profile carries RECORD-ONLY lengths, and every
// per-radio plan pins BOTH numbers with the address width named. For this
// radio:
//
//   - the `1A 00` DATA AREA is 115 bytes — what the witness measures,
//     positions 1..115;
//   - the ADDRESS is 4 of them — two packed-BCD group bytes (`①②`) then
//     two packed-BCD channel bytes (`③④`), which is why this profile
//     declares civ.AddressFormWideGroupChannel;
//   - the RECORD is therefore 111 bytes, and 111 is the ONLY number this
//     profile declares. AcceptsRecordLength(115) is false, and a test pins
//     that: 115 in a profile field would be a data area masquerading as a
//     record, and the probe's length fingerprint would then match the
//     wrong model.
//
// The set frame on the wire is 122 bytes = FE FE + A4 + E0 + 1A 00 + 115 +
// FD, which is what `vectors.golden`'s set-record-name-with-space
// measures, and the read request is 11.
//
// OFFSETS IN THIS PACKAGE ARE 0-BASED FROM THE START OF THE RECORD. The
// matrix and the witness number DATA-AREA POSITIONS from 1, so
// offset = data-area position − 5.
//
// # Printed indices are not positions
//
// The diagram's last bracket prints `53~68` over cells the witness
// measures at 100..115 (leg W's STOP 3). The printed indices past the
// duplicated block are LOGICAL — they name the field the block copies, not
// the byte it occupies — so a layout that trusted the label would put the
// memory name forty-seven bytes early. That is why every offset in
// profile.go comes from the witness's measured first_byte and never from a
// printed numeral, and why geometry_test.go pins the discrepancy directly.
//
// # The name charset
//
// Matrix §3.9 as corrected by erratum 2: PDF p.20 (folio 19), `Codes for
// character entries`, prints A~Z against 41~5A, a~z against 61~7A, 0~9
// against 30~39, and a symbols table of THIRTY-TWO entries. Those
// thirty-two are exactly ASCII's punctuation set, so the enumerated
// characters plus the assumed space add up to precisely 0x20..0x7E —
// which also reconciles the same page's flat `All characters are usable.`
//
// It is an OBSERVATION ABOUT WHAT THE TABLES ADD UP TO, not a licence to
// declare a range. The space is not printed in any table (spec D5 entry 3
// records that every model's charset table omits it while the radios
// plainly accept one), so it is ASSUMED, and so is the pad byte.
//
// # The unmapped record areas — 53 whole bytes
//
// The whole 115-byte data area is sent on every set (PDF p.19's NOTE
// panel), so a write must carry back unchanged every area no neutral field
// claims. civ's encoder fills those bytes from the layout's Fixed
// template, and has no opaque carrier. The tier's ruling (enablers E6) is
// therefore: A DRIVER MAY WRITE A SLOT ONLY WHEN ITS UNMAPPED REGIONS
// EQUAL THE TEMPLATE; ANYTHING ELSE IS REFUSED WITH THE REASON NAMED,
// NEVER REWRITTEN.
//
// Under this layout the unmapped set is these record offsets:
//
//	0 (BOTH nibbles)   data-area 5      Split OFF/ON and the ★n Select marking
//	10 and 57          15 and 62        digital squelch setting and its TX-block copy
//	20 and 67          25 and 72        DV digital code squelch and its copy
//	24–47 and 71–94    29–52 and 76–99  the three 8-character DV call signs and their copies
//
// = 27 bytes in the RX area and 26 in the TX block, 53 in all. Matrix
// erratum 6 counts "68 bytes and one nibble" over the same document; the
// difference is arithmetic, not disagreement. civ permits a field to
// appear twice in one layout and requires the copies to agree, so 16 of
// the duplicated block's bytes are claimed by SECOND spans and survive by
// construction (68 + the Split nibble + the Select nibble − 16 = 53).
//
// THE COST, STATED PLAINLY AND ACCEPTED FOR THIS TIER: this dialect can
// write an FM or SSB channel, and CANNOT write a channel carrying D-STAR
// routing, a digital-squelch setting, Split ON, or a ★1/★2/★3 Select
// marking. Each is refused with the reason named. It refuses; it never
// corrupts. An opaque carrier was considered and REJECTED for v1 —
// it conflicts with the gate's byte-identity re-encode, a conflict
// verified by execution in review — and is revisited with hardware.
//
// # The ★n Select marking is not scan_skip
//
// The low nibble of record offset 0 is four-valued and marks a channel
// INTO one of three select groups (witness D2; the verbs are `0E B0` and
// `0E B2`, matrix erratum 3). The neutral scan-skip field is a two-valued
// flag, so mapping the nibble onto it would report a ★2-marked channel as
// "skip" and, worse, write OFF back on any unrelated edit — the outgoing
// record is built from neutral channel data, which carries no Select
// member at all. Both directions are refused by the tier's rule that only
// Known values are ever encoded and a non-Known mandatory field is
// REFUSED, never synthesised. So the nibble is UNMAPPED, and it joins the
// Split nibble beside it to make offset 0 a whole unmapped byte fixed at
// 0x00. This DIVERGES from matrix §2, which grades the cell Supported; a
// matrix erratum is proposed.
//
// # No factory default is printed anywhere in this document
//
// A negative finding, recorded so that a later reader does not go looking.
// This guide marks no factory defaults at all, by asterisk or otherwise
// (matrix erratum 4). In particular it prints NO default tone value — not
// in the `1A 00` memory-content legend (PDF p.19, folio 18) and not in
// `• Repeater tone/tone squelch frequency settings` (PDF p.23, folio 22),
// which prints digit ranges and no shipped value. The IC-705 Basic Manual
// is admitted for three values only (default baud, transceive default,
// USB echo-back default) and a tone default is not among them. So a
// create into an empty slot whose tone fields are not Known is REFUSED
// naming those fields, and NO register entry for a default tone exists
// here: refusing is the ABSENCE of an assumption, and filing one would
// record a claim this package does not make.
//
// # The ASSUMED register
//
// Every entry this package owns, with the section it was read from and the
// ONE named hardware lift that would settle it. Nothing here is cited from
// memory: each names its artefact and its anchor.
//
//   - ic705-record-length (spec D5 entry 6) — the 111-byte RECORD-ONLY
//     length. The document prints no record length anywhere; 111 is
//     DERIVED as the witness's measured 115-position data area less the
//     four address bytes it also measures.
//     LIFT L-RECLEN: one `1A 00` read of any occupied channel, the answer
//     captured whole, and the byte count after the address field taken
//     directly. A different count refutes this profile's only declared
//     length and every test that turns on it.
//
//   - spec D5 entry 5 — that WIRE ORDER IS DIAGRAM ORDER past the
//     duplicated block, and that the printed indices there are LOGICAL
//     (matrix erratum 10). The diagram draws the block as a single wide
//     dashed band labelled with two FILLED circled numerals `❻ ~ ❺❷` and
//     prints no cell boundaries inside it; the 47 positions come from the
//     bracket's own end numerals.
//     LIFT L-RECLEN, the same read: the captured record's byte 53 onwards
//     either repeats bytes 6..52 or it does not.
//
//   - spec D5 entry 1 — the `1A 00` READ-REQUEST FORM: command, then the
//     address, and no data field. No document in this tier prints the read
//     request; PDF p.6 (folio 5) gives only the table row `1A* / 00 / See
//     pp. 18 and 19. / Send/read memory contents`, and the asterisk key on
//     PDF p.17 (folio 16) reads `*(Asterisk) Send/read data`. The form is
//     assumed family-wide.
//     LIFT L-READFORM: send the 11-byte request to a real radio and
//     observe whether an address-matched `1A 00` answer comes back.
//
//   - spec D5 entry 3 — NAME SPACE HANDLING. The charset tables on PDF
//     p.20 (folio 19) print no space row, and this profile declares 0x20
//     legal on the strength of the same page's `All characters are
//     usable.`
//     LIFT L-NAME-SPACE: write a name containing an interior space and
//     read it back.
//
//   - spec D5 entry 3, second limb — THE NAME PAD BYTE, 0x20. The document
//     says nothing about how a name shorter than sixteen characters is
//     filled.
//     LIFT L-NAME-PAD: read a channel whose name is shorter than sixteen
//     characters and record record offsets 95..110 as raw bytes. (The
//     consequence civ already documents applies here: a name ENDING in the
//     pad byte cannot round-trip, because padding erases the
//     data-versus-fill distinction on the wire.)
//
//   - ic705-tone-field-roles — WHICH THREE-BYTE FIELD IS THE TX TONE.
//     `⑯~⑱` (offset 11) and `⑲~㉑` (offset 14) carry the IDENTICAL printed
//     label `Repeater tone frequency setting` (matrix §3.16 A1), and all
//     three legs that read the legend recorded the disagreement without
//     resolving it. This package reads the first as tone_tx and the second
//     as tone_rx, on the argument that the shared cross-reference is headed
//     "Repeater tone/tone squelch" in that order for commands `1B 00, 1B
//     01` in that order. AN ARGUED ASSUMPTION, NOT EVIDENCE — and the
//     golden vector cannot discriminate, since it encodes 88.5 Hz into
//     both fields.
//     LIFT L-TONE-ROLE: set a channel's repeater tone and its tone squelch
//     to DIFFERENT frequencies from the front panel, read the record, and
//     see which number lands at offset 11.
//
//   - ic705-dtcs-nibble-roles — that data-area position 22 (offset 17)
//     carries DTCS POLARITY as `{00:NN, 01:NR, 10:RN, 11:RR}` and
//     positions 23–24 the three-digit code. The page prints all three
//     bytes as one entry, `DTCS code setting`, and only cross-references
//     `DTCS code and polarity setting` on PDF p.23 (folio 22); the split
//     comes from matrix §1b.
//     LIFT L-DTCS-POLARITY: set a channel to DTCS with a known code and
//     each of the four polarities in turn, and record offsets 17..19.
//
//   - ic705-dv-mode-code — that the operating-mode code printed as `17`
//     for DV is HEX 0x17. Every other code on PDF p.18 (folio 17) reads
//     identically in either base; this one does not, and there is no
//     `0x`-style marker anywhere on the page.
//     LIFT L-DV-MODE: set memory channel 12 to DV from the front panel,
//     read it, and record record byte 11's value. It observes only which
//     byte value DV is on this radio.
//
//   - ic705-unmapped-record-areas — that the 53 bytes tabulated above hold
//     what this layout's Fixed template writes on a channel that has never
//     carried D-STAR routing, a digital squelch setting, Split ON or a ★n
//     marking. The template is what makes such a channel writable at all,
//     and a radio that stores something else there makes every write to it
//     a refusal rather than a corruption — so the failure mode is safe,
//     but the assumption is real.
//     LIFT L-UNMAPPED: read one occupied channel that has never had DV
//     routing set and record record offsets 0, 10, 20, 24–47, 57, 67 and
//     71–94 as raw bytes.
//
//   - ic705-blank-callsign-byte — that an UNSET DV call sign reads back as
//     0x20 eight times, which is why the template fixes those runs at 0x20
//     rather than 0x00. PDF p.24 (folio 23)'s `Character's code of the
//     call sign` table gives the character codes and says nothing about
//     what an unset field holds.
//     LIFT L-CALLSIGN-BLANK: read a channel whose UR call sign has never
//     been set and record record offsets 24..31.
//
// # What this package deliberately does NOT decide
//
// The tone DOMAIN. These spans decode and encode plain BCD deciHertz over
// the whole encodable range — 0..2999, the printed digit leaders on PDF
// p.23 (folio 22) being `100Hz digit: 0 ~ 2` then `0 ~ 9` three times —
// with ZERO INCLUDED. Zero is what the radio holds for "no tone set", and
// it is NOT a tone: the declared capability floor is 0.1 Hz, so the
// encodable set and the declared set differ by exactly the value zero.
// Deciding that here would cost the write gate its byte-identity
// re-encode, so civ stays lossless and semantics-free and the driver owns
// the Known/Unknown decision. Recorded so the one-value gap reads as a
// stated design fact rather than an off-by-one somebody later "fixes".
//
// The CALL group's channel ceiling. civ carries ONE channel range for
// every group, so this profile's declared 0..99 admits CALL channels 4..99
// although the radio has four call channels. That residual gate width is
// deliberate, bounded and covered twice over in the driver — see its own
// doc.go, which records it.
package ic705
