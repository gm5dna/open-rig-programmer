// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7760 describes the IC-7760 CI-V memory dialect.
//
// The profile uses address B2, controller E0 and a two-byte flat packed-BCD
// channel selector. The base range is MEM 0001–0099 and one extra flat
// range carries programmed scan edges 0100–0101 (P1/P2); this inventory is
// MANUAL-EVIDENCED at PDF p.20 (folio 19) and PDF p.4 (folio 3), per the
// additions-spec Erratum 5. There is no CALL bank. The guide's conflicting
// “Memory group number” label is arbitrated as the channel number because
// the values, command 08 and the clear note all say channel.
//
// The record is 25 bytes after its two-byte address, hence a 27-byte data
// area. SELECT is four-valued membership (OFF, ★1, ★2, ★3), not binary
// scan skip; its high nibble is fixed zero and P1/P2 require the whole byte
// to be zero. Data mode is the high nibble of record byte 8 and tone mode
// the low nibble. Registration, transport and erase remain outside this
// package.
//
// The profile consumes five assumptions, each with exactly one IC-7760
// hardware lift:
//
//   - Register entry `ic7760-name-space-code` (ASSUMED). Lift: Stage R
//     enters A B at the front panel and captures all ten name bytes.
//   - Register entry `ic7760-name-pad-byte` (ASSUMED). Lift: Stage R enters
//     ABC and captures all ten bytes to identify the seven pad bytes.
//   - Register entry `ic7760-record-length` (ASSUMED). Lift: Stage R reads
//     one occupied B2 channel and counts the bytes between its two-byte
//     address and FD.
//   - Register entry `ic7760-tone-domain` (ASSUMED). Lift: Stage R stores
//     the lowest and highest tones offered by the front panel and captures
//     both three-byte tone fields.
//   - Register entry `ic7760-scan-edge-record-shape` (ASSUMED). Lift:
//     Stage R reads B2 channel 0100 and records its length and every byte.
package ic7760
