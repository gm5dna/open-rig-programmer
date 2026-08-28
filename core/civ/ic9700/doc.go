// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic9700 is the Icom IC-9700's CI-V DIALECT: the memory record's
// geometry, the (band, channel) address form, the name policy and the
// printed enums, expressed as core/civ data.
//
// WHAT IT IS NOT, emphatically. It is DATA ONLY. There is no driver here,
// no simulator, no registration in any wiring table and no wire of any
// kind: nothing in this package opens a port, builds a session, sends a
// frame or decides whether a write may proceed. Those live in
// core/driver/ic9700 and internal/fakeic9700, and the separation is what
// lets this package be read as a transcription of a document rather than
// as a program that talks to a radio.
//
// The exported surface is small on purpose: Profile, and the three length
// constants (RecordLength, AddressBytes, DataAreaLength) that every later
// file in this tier cites by name instead of repeating a digit.
//
// # NO IC-9700 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT
//
// NO IC-9700 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT. Not one frame
// has been sent to one, not one byte has been read back from one, and no
// value in this package has been confirmed against hardware. Every claim
// below is a transcription of a printed page or a deduction from one, and
// the register at the foot of this file names which is which. Consent is
// the only key that opens the driver's write gate, and consent is not
// evidence.
//
// # Provenance
//
// The single source is the **IC-9700 CI-V REFERENCE GUIDE**, revision
// A7508-3EX-4, "© 2019–2023 Icom Inc. Mar. 2023", SHA-256
//
//	875cde8faba30d333f11b7a9f46ab44b710de85f06e922badd47b7be3eb8bf3f
//
// The PDF and its provenance note,
// docs/fixtures-private/manuals/ic9700-manual-provenance.md, are BOTH
// gitignored. Every reference in this package is therefore a CITATION,
// not a link: a reader without the guide cannot follow one, and no test
// in this package depends on the file being present. What IS committed is
// the four evidence legs under testdata/, which is where a reader goes
// instead.
//
// THE TWO PAGE NUMBERINGS DIFFER BY ONE. The guide's printed folio N is
// PDF page N + 1 — the cover carries no folio — so a citation to "PDF
// p.15 (folio 14)" names one page under two systems and not two pages.
// This package's comments cite PDF pages, which is what a reader with the
// file in a viewer will see.
//
// No other document is admitted. In particular the IC-9700 Basic Manual
// is UNOPENED: the tier has admitted the IC-705's Basic Manual for
// defaults on that model alone, and no equivalent admission has been made
// here, so this model's factory default baud remains an assumption on
// this guide's silence rather than a fact borrowed from elsewhere. No
// Yaesu value is cited as evidence about an Icom anywhere in this
// package — notably not core/spec's fifty-tone CTCSS chart, which is
// verified against a Yaesu manual.
//
// # Defects in the source, recorded as seen and never repaired
//
// Two printed defects sit inside the pages this package transcribes.
// Both are recorded exactly as drawn, and a later transcription that
// silently repairs either one FAILS a test here (crosscheck_test.go's
// TestSourceIndexDefectIsPinnedNotRepaired).
//
//   - THE ④ DETAIL BOX IS CAPTIONED WITH A CIRCLED THREE (matrix §3.16
//     A4). On PDF p.15 the one-byte mini-diagram that expands "④ Select
//     memory setting" carries a circled 3 above it, while the heading
//     immediately over it, and the clear instructions in the same right-
//     hand column, both use ④ for the same field. Legs L (its STOP 1,
//     confirmed at 400 and 600 dpi) and W (its STOP 8) recorded it
//     independently. The ledger keeps the row indexed 3 and unlabelled,
//     because that is what the page prints.
//
//   - THE SATELLITE RECORD'S NAME INDICES CONTRADICT THEMSELVES (matrix
//     §3.15(b)2, §3.16 A8). On PDF p.20 the `1A 07` artwork's final
//     bracket prints ㊼ ~ (55) while the legend on the same page prints
//     circled 47 to circled 67 for a 16-character name. Neither reading
//     agrees with the other, and 47~55 is nine indices for a sixteen-byte
//     field. It touches nothing here — satellite memories are out of
//     scope for this tier and no 1A 07 builder exists — and is recorded
//     so a later milestone starts from evidence rather than from silence.
//
// # The clear form EXISTS and is deliberately unshipped
//
// Unlike the Yaesu case, where no erase wire form was ever found, THIS
// MODEL'S CLEAR FORM IS PRINTED. PDF p.15, right legend column, under
// "To clear the memory channel contents on 1A 00:", gives a `1A 00` set
// carrying the band and channel address, `FF` in field ④, and no data
// after it — with the channel range printed as `0001~0099` only, i.e.
// narrower than the read form's, excluding every scan-edge and call
// channel (matrix §3.13, §3.16 A7).
//
// NOTHING IN THIS TIER MAY SEND IT. There is no clear builder in this
// package; civ.AllowedCommand does not admit the frame (the gate admits
// three grammars — 19 00, a 1A 00 read and a 1A 00 set — and no more);
// core/driver/ic9700 gives spec.FieldErase a zero spec.FieldSupport on
// every bank; spec.ConsentUnverifiedWrites structurally never consents an
// erase; and core/clone/execute.go's DiffErased branch stays unreachable
// for this model. The form is recorded here as EVIDENCE, not as a
// capability.
//
// What a future write-trial milestone would need before any of that could
// change: a consented hardware session on a real IC-9700 that writes a
// scratch channel, sends the clear frame to it and records the FB or FA
// answer, re-reads the channel and records what an emptied slot answers,
// and repeats the clear against a scan-edge channel (0100) and a call
// channel (0106) to establish whether the printed 0001~0099 restriction
// is real. Only then could a builder, a gate admission and a non-zero
// FieldErase support be justified. Register entry
// `ic9700-clear-form-accepted`, lift W5, lives in the DRIVER's register.
//
// # This dialect's own ASSUMED register
//
// The tier's FAMILY assumptions (spec D5) and the IC-9700 DRIVER's own
// eighteen entries live in core/driver/ic9700/doc.go. What follows is the
// register for the assumptions THIS package's data embodies — the five
// entries the IC-9700 plan ADDS to the matrix's twenty-nine, in the
// three-part form: the assumption in CAPS with the file that implements
// it, WHAT DEPENDS ON IT, and LIFTED BY naming ONE capture.
//
// Each is IC-9700-only. An entry that names a model means it: its claim
// is no wider than the capture named.
//
//   - THE MODE CODES ARE HEXADECIMAL (`ic9700-mode-codes-are-hexadecimal`;
//     profile.go's modeNames). PDF p.14's operating-mode table prints the
//     glyphs 00, 01, 02, 03, 04, 05, 07, 08, 17, 22 and NOT the base.
//     For eight of the ten it does not matter, because the glyphs are the
//     same byte read either way. For DV (17) and DD (22) it decides the
//     byte: read as decimal they would be 0x11 and 0x16 instead.
//     Hexadecimal is the reading every other code field in the same guide
//     uses, and it is the one taken here.
//     WHAT DEPENDS ON IT: every DV or DD channel this tier reads or
//     writes on an IC-9700, and the driver's A2 cross-field rule that
//     refuses DD outside the 1.2 GHz band.
//     LIFTED BY: **R22** — one `1A 00` read of a channel set to DV from
//     the front panel, recording byte ⑩ verbatim.
//
//   - A SLOT WHOSE UNMAPPED REGIONS DIFFER FROM THE TEMPLATE IS REFUSED,
//     NEVER REWRITTEN (`ic9700-unmapped-regions-refused`; profile.go's
//     fixedTemplateBytes, enforced by core/driver/ic9700's write path).
//     Fifty-two of the 111 record bytes have no civ.FieldID home — ⑭
//     digital squelch, ㉔ DV code squelch, the three eight-byte call
//     signs, and each of their copies inside the duplicated block — and
//     civ.encodeRecord writes them from the layout's Fixed template. That
//     a channel carrying different values there may be REFUSED rather
//     than silently rewritten is this tier's rule (E6), not a fact about
//     the radio; what is assumed is that refusing is the right price.
//     WHAT DEPENDS ON IT: whether this tier can write an IC-9700 channel
//     without destroying data it cannot represent.
//     LIFTED BY: **W8** — one write trial that sets a scratch channel's
//     UR call sign from the front panel, writes the channel through this
//     driver, and re-reads the unmapped regions.
//
//   - THE TEMPLATE'S VALUES ARE THE GOLDEN'S STATE, AND THE FACTORY
//     STATE IS INFERRED (`ic9700-unmapped-template-is-the-golden-state`;
//     profile.go's fixedTemplateBytes, proved equal to the vector by
//     profile_test.go). The template is taken from the frozen
//     set-record-name-with-space vector's own record bytes: UR `CQCQCQ`
//     plus two pad spaces, blank R1 and R2, ⑭ and ㉔ zero. `CQCQCQ` is
//     the D-STAR broadcast destination and is the plausible state of an
//     untouched memory — but THAT IS AN INFERENCE. No capture shows what
//     a factory IC-9700 memory holds in those 52 bytes.
//     THE ASYMMETRY IS WHY IT IS SHIPPABLE: the guard compares and
//     refuses on mismatch in every case, so if the inference is right the
//     guard admits the common channel and refuses the unusual one, and if
//     it is wrong the guard simply refuses more channels. The failure
//     mode of a wrong template is MORE REFUSALS, never corruption, and
//     the guard runs before any frame is built, so no wrong-template
//     outcome reaches a radio.
//     WHAT DEPENDS ON IT: which real channels the write guard admits.
//     LIFTED BY: **R23** — one `1A 00` read of an untouched
//     factory-default memory channel, recording the unmapped regions
//     verbatim. The first hardware session settles it.
//
//   - THE DUPLICATED BLOCK AGREES WITH THE PRIMARY ON READ
//     (`ic9700-duplicate-block-agrees-on-read`; profile.go's
//     recordFields, which maps ❿…❺❶ as REPEATS of ⑩…51's ids at +47).
//     Because they are repeats, civ.decodeRecord requires the two copies
//     to AGREE and a disagreeing record fails to parse rather than
//     letting one copy win silently. The manual's grey NOTE asserts the
//     identity — "The same data as ⑤ ~ 51 are stored in ❺ ~ 51" — and
//     also says the filled block is what the radio transmits with when
//     Split is ON, which is precisely the case where a real radio's two
//     blocks could differ. No capture confirms either.
//     WHAT DEPENDS ON IT: whether ReadAll succeeds on a radio whose Split
//     settings differ between the two blocks.
//     LIFTED BY: **W2** — a set whose duplicated block deliberately
//     disagrees with its primary, read back.
//
//   - THIS MANUAL DOCUMENTS NO DEFAULT TONE
//     (`ic9700-no-documented-default-tone`; consumed by
//     core/driver/ic9700's empty-slot CREATE path, which this package
//     supplies no value for). Icom commonly prints "Default: 88.5 Hz";
//     this guide does not. PDF p.21's tone diagram prints digit RANGES
//     and nothing else, and leg G's 88.5 Hz is recorded in its own
//     provenance as a CHOICE ("Chose 88.5 Hz"), not as a printed default.
//     This is a MANUAL-EVIDENCED ABSENCE, which is a finding rather than
//     a gap: it is why a create whose tone spans are not Known is refused
//     naming the field instead of being filled with a plausible number.
//     WHAT DEPENDS ON IT: whether a CREATE with unspecified tones can
//     proceed at all.
//     LIFTED BY: **R24** — read a factory-default channel whose tone has
//     never been set and record its tone spans verbatim, which also gives
//     a later milestone the value it would write.
package ic9700
