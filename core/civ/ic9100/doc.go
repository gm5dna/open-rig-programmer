// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic9100 describes the IC-9100 CI-V memory-record dialect.
//
// The profile is manual-derived and has not been exercised against a radio
// (docs/superpowers/icom-matrices/ic9100-capability-matrix.md, §0 "Hardware
// status": "No IC-9100 has ever been asked anything by this program").
//
// # The headline finding: CI-V address 7Ch, not 88h
//
// matrix §3.4: the manual's own SET MODE item 61 ("CI-V Address (Default:
// 7Ch)") states the address in words — "The IC-9100's address is 7Ch." —
// and the 88h/E0h figure v1.7.0's spec.md §1 carried is a documented Icom
// manual erratum: the CI-V data-format frame diagram on the same manual's
// PDF p.192 is captioned "Controller to IC-7100"/"IC-7100 to controller",
// boilerplate copied from Icom's own IC-7100 documentation and never
// updated for this model. RadioAddress is 0x7C here, matching the matrix's
// ruling and spec.md §7's correction, not the dispatch title's 88h.
//
// # The band-byte address form
//
// matrix §3.11: the printed `1A 00` record draws its band byte and two-byte
// channel number inside the same continuous field enumeration as every
// other field, with no visual break marking them as an address distinct
// from the record — the identical evidentiary gap
// ic7100-capability-matrix.md §1b records for its own `①②③`. Resolved by
// the same CHOICE, citing that precedent: civ.AddressFormBankChannel, one
// packed-BCD band byte (00 HF/50MHz, 01 144MHz, 02 430MHz, 03 1200MHz) then
// the two-byte channel number. RecordOnlyLength is therefore 60 - 3 = 57.
//
// Groups is 3 (GroupBase 0, band values 00-02: HF/50, 144, 430 MHz), the
// base 3-band radio's 297-slot inventory (matrix §1 row 5). The optional
// UX-9100's 4th band (03, 1200 MHz, 396 slots with it fitted) is DEFERRED
// rather than modelled: matrix §1 row 16 / §3.15(6) finds that this
// record's 5-byte frequency field caps at 499.999999 MHz — the 100 MHz
// digit is documented "0-4" — narrower than the 1320 MHz UX-9100 ceiling,
// so how a 1200 MHz value is encoded in a field that cannot hold it
// directly is NOT resolved by the matrix. Modelling a band this profile
// cannot yet encode a frequency for would be a wire address with no
// honest write path behind it; the simpler and safer scope is the base
// radio, with the 4th band left for the register entry
// ic9100-1200mhz-frequency-encoding to resolve before it is added.
//
// # D-STAR bytes: no neutral field, so none is mapped
//
// matrix §1b: this record carries a digital squelch byte (offset 10, matrix
// §3.11 term 8), a second "digital code squelch" byte (offset 20, term 12)
// and three 8-byte D-STAR call signs (destination/R1/R2, offsets 24-47).
// None has any spec.Field anywhere in this codebase. This is the identical
// situation ic7100-capability-matrix.md records for the IC-7100 (same
// D-STAR family), and spec.md's per-radio note for IC-9100 cites that
// precedent by name: these bytes are simply never read into the codeplug.
// fixedTemplateBytes gives them the same conservative default the IC-7100
// package uses ("CQCQCQ" plus trailing spaces for each call sign, 0x00 for
// the two squelch bytes) — a CHOICE, not a manual-stated default, since the
// manual states no factory value for any of the four.
//
// # No TX-duplicate block
//
// matrix §1b / §3.15(5): unlike IC-7100 and IC-7700 in the same v1.7.0
// wave, this record's field enumeration sums to exactly 60 bytes with no
// room for a duplicated TX-side block, and no "the same data as … are
// stored in …" note appears against it as it does on IC-7100's own page.
// spec.FieldTxFrequency is therefore unsupported on this model; the `r`
// byte's Split flag is a display/behaviour hint with no accompanying
// stored TX frequency this document describes.
//
// # Stage-1 assumptions, each pinned to one named hardware lift
//
//   - ic9100-civ-address-confirm: read the address of a factory-fresh
//     IC-9100 by other means (its own SET MODE display) and confirm 7Ch.
//   - ic9100-read-request-form: send FE FE 7C E0 1A 00 00 00 01 FD and
//     record the reply form.
//   - ic9100-name-pad-byte: set a 3-character name and read all 9 bytes.
//   - ic9100-tag-charset-on-wire: write ';', '\\' and '~' in a name and
//     read it back.
//   - ic9100-short-set-acceptance: write a deliberately truncated data area
//     and record the radio's response; this package always sends the full
//     57-byte record.
//   - ic9100-empty-channel-fa / ic9100-all-ff-record: clear a scratch
//     channel and read it, recording which empty form the radio uses.
//   - ic9100-serial-framing / ic9100-broadcast-address-form /
//     ic9100-echo-default: transport/driver-boundary assumptions this
//     data-only profile does not expose.
//
// See the matrix's own §3.16 register table for the complete list this
// package and its driver sibling share.
package ic9100
