// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7200 is the IC-7200's CI-V dialect: a civ.Profile built from
// docs/superpowers/icom-matrices/ic7200-capability-matrix.md (rev 1,
// 12/09/2026), the IC-7200 Advanced Instructions manual
// (docs/fixtures-private/manuals/ic7200_advancedmanual_0b.pdf) and the
// tier spec's TX-duplicate ruling (spec.md's "Matrix reconciliations"
// note for this model).
//
// NO IC-7200 HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT (matrix §0,
// §3.14). Every byte, width, address and vocabulary here is ASSUMED from
// printed documentation; core/driver/ic7200's writeTrialsComplete is
// pinned FALSE.
//
// THE RECORD IS 17 BYTES, NOT 9. Matrix §3.11 is the central finding: a
// 300 dpi render of PDF p.120 (folio 11-6) resolves a second glyph class
// (filled/reversed circled numerals ❹~⓫) as a genuinely distinct
// TX-duplicate block, not a second citation of the primary ④~⑪ span —
// which is exactly how spec.md's IC-7200 clause and the S1 evidence file
// both misread it. This supersedes spec.md's own IC-7200 clause wherever
// the two disagree; spec.md's "Matrix reconciliations" note records the
// correction and the ruling it implies.
//
// Register entries this package assumes, D5-shared unless named
// otherwise (matrix §5's own table has the full citation for each):
//
//	ic7200-default-baud-auto           factory Auto locks to 19200 on first 19 00
//	ic7200-civ-rate-list               the five named rates are the complete list
//	ic7200-1a00-set-ack                a 1A 00 set is acknowledged with FB
//	ic7200-full-record-mandatory       the full 17-byte record is mandatory on write
//	ic7200-scan-edge-record-fields     whether P1/P2 honour every field
//	ic7200-storable-frequency-range    the radio's actual tuning floor/ceiling
//	ic7200-tx-duplicate-block-internal-order  whether ❹❺❻❼❽❾❿⓫ mirrors ④⑤⑥⑦⑧⑨⑩⑪ byte-for-byte
//
// NoTag (matrix §1 row 6): TagLen 0, NoTag true, reusing
// core/spec/validate.go's existing NoTag pairing rule verbatim — no bank
// may grade FieldTag or FieldTagDisplay (spec.md §2). This is the first
// non-test NoTag registration since the capability merged (commit
// 8f0352e).
package ic7200
