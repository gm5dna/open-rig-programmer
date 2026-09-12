// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7410 contains the independently evidenced IC-7410 CI-V dialect:
// one validated flat-address profile. The driver next door owns probing,
// capabilities and write policy.
//
// # The document
//
// There is NO standalone CI-V reference guide for this model. The
// authority is the IC-7410 Instruction Manual, revision 01
// (`docs/fixtures-private/manuals/ic7410_fullmanual_01.pdf`, gitignored,
// Icom copyright; SHA-256 verified against
// `docs/fixtures-private/manuals/ic7410-manual-provenance.md`). Its CI-V
// material is Section 14 "CONTROL COMMAND", PDF pp.108-115, and the memory record
// is PDF p.115, "Memory content setting / Command: 1A 00". The radio's
// printed default CI-V address is 80h and the controller's is E0h, so
// every frame this package builds is FE FE 80 E0 ... FD.
//
// The full reading is
// `docs/superpowers/icom-matrices/ic7410-capability-matrix.md` (gitignored,
// Phase-1 deliverable of the v1.7.0 Icom wave,
// `.superpowers/sdd/2026-09-12-icom-wave-v170/spec.md`).
//
// # The headline finding: 40 bytes, not 25
//
// The wave spec's own §1 IC-7410 diff said the record is "25 B ...
// unchanged by coincidence of the repack" and named no new field family.
// That reading summed only the record diagram's OUTLINE-circle printed
// indices (③ through ㉗ = 25) and missed a fifteen-byte block drawn with
// FILLED reference circles ❹-⑱ between the tone-squelch group and the
// name field: "④-⑱: are programmed in the same manner as ④-⑱ ... when the
// split setting is ON, these settings are the matching transmit settings
// ... [still necessary] even when the split setting is OFF." That is a
// genuine TX-duplicate block — the tier spec's own matrix reconciliation
// (spec.md's ruling 7) corrects the record to 40 bytes and rules that the block's
// first five bytes (the TX frequency span) map to `civ.FieldTXFrequency`
// while its remaining ten bytes carry no neutral field and are left
// UNMAPPED. See consts.go and profile.go for the corrected offset table.
//
// # Two things this model does differently from the IC-7610 family
//
// Both are read directly off the record diagram, not inherited from a
// sibling profile:
//
//   - `data_mode` (record offset 8, printed ⑪) is a WHOLE BYTE on this
//     model — a genuine two-valued boolean with no nibble-sharing — where
//     the IC-7610 family packs a four-valued data mode into a shared
//     nibble that this project leaves UNMAPPED. IC-7410's data_mode is
//     therefore MAPPED (spec.FieldDataMode is writable), a genuine
//     divergence from every 7610-family sibling in this wave.
//   - `tone_mode` (record offset 9, printed ⑫) sits on the HIGH nibble,
//     the opposite of the IC-7610 family's LOW-nibble convention.
//
// # Hardware status
//
// No IC-7410 has ever been asked anything by this project. Every byte,
// width, address and vocabulary here is derived from the printed manual;
// every value handed to a future driver is Unverified.
package ic7410
