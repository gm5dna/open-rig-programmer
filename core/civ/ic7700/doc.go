// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7700 holds the Icom IC-7700's CI-V dialect: the memory
// record's geometry, its three value enums, its name charset and the
// civ.Profile that binds them. It is DATA ONLY — no driver, no fake, no
// registration, no session, no wire. SupportedModels derives solely from
// internal/wiring's driver table.
//
// # Provenance
//
// Everything here comes from the IC-7700 Instruction Manual, Revision 7,
// 232 pages (docs/fixtures-private/manuals/ic7700_fullmanual_7.pdf,
// gitignored, so page references are citations rather than links), through
// the reviewed capability matrix (docs/superpowers/icom-matrices/
// ic7700-capability-matrix.md rev 1) and from nothing else. There is no
// separate CI-V Reference Guide for this model; CI-V is Section 14 of the
// full manual.
//
// UNLIKE THE EARLIER-LANDED MODELS IN THIS TREE (ic7610, ic7300, ic7851),
// this package's testdata is single-authored rather than the product of
// four independent QUARANTINED readings: this wave's brief funds one
// Sonnet implementer per model, not a four-leg blind cross-check. The
// golden vectors (testdata/IC-7700-vectors.golden, golden_test.go) are
// therefore hand-derived from the matrix's own citations and deliberately
// built from trivial (mostly zero) field values, so every byte follows
// from arithmetic a reader can check without a second reading to arbitrate
// against. This is a real, stated reduction in cross-check rigour versus
// the tier's earlier convention, not a hidden one.
//
// NO IC-7700 HARDWARE HAS EVER BEEN ASKED ANYTHING by this project. Every
// statement in this package is a reading of one manual, mediated by the
// matrix.
//
// # Record geometry
//
//	RecordOnlyLength  39   the 1A 00 data block EXCLUDING the two channel
//	                       selector bytes (q,w) — what civ.Profile
//	                       carries, and what BuildMemorySet's <record>
//	                       denotes.
//	DataAreaLength    41   the 1A 00 data block INCLUDING them — the
//	                       matrix's own eight-term addition (§3.11).
//	AddressBytes       2   <ch-hi> <ch-lo>, civ.AddressFormFlat.
//
// A 1A 00 set frame is therefore 48 bytes (6 + 2 + 39 + 1), a 1A 00 read
// frame 9 (6 + 2 + 1) and a 19 00 read frame 7 (6 + 1).
//
// # Ruling E6 — three unmapped regions, not two
//
// This model carries the tier's usual two E6 unmapped regions — the
// split/select-group byte (here: idx0, BOTH nibbles, because unlike the
// IC-7610's analogous byte the split flag also lives in this byte with no
// neutral home) and the data-mode nibble (idx8 high) — PLUS a third,
// unique to this radio's duplicated TX block: idx20-28 (9 bytes), the
// TX-duplicate block's own mode/filter/tone-type/tone_tx/tone_rx bytes.
//
// Matrix §3.15's own layout table calls idx20-28 "UNMAPPED — no TX-side
// spec.Field exists in the tier vocabulary" and flags it as "this model's
// largest single open question for the plan": a driver could either
// mirror the RX copy into these bytes on every write, or leave them
// genuinely unmodelled and round-trip whatever the radio last held. The
// coordinator's ruling of 12/09/2026 settles it for THIS wave: the block's
// own FREQUENCY sub-span (idx15-19) is mapped to civ.FieldTXFrequency,
// because that field already exists in the tier's vocabulary and the
// manual is explicit that it is always required ("Even when the split
// setting is OFF, these settings are still necessary", PDF p.213); the
// remaining nine mode/filter/tone bytes are left UNMAPPED — "second
// TX-side copy, no neutral field" — governed by the same all-zero Fixed
// template and E6 refusal-on-mismatch as idx0 and idx8's high nibble,
// rather than mirrored via a reused FieldSpan (core/civ/ic7300's approach
// for its own, differently-scoped TX-dup block). The driver package's
// write-time policy for idx15-19 on a Split-OFF (simplex) channel — mirror
// the RX frequency in, rather than refusing — lives in
// core/driver/ic7700/write.go and is recorded there, not here: this
// package states only what the WIRE carries, never a write-time choice.
//
// The consequence, stated as E6 requires: a slot whose idx0, idx8-high or
// idx20-28 bytes are non-zero when read CANNOT BE WRITTEN BACK BY THIS
// PROGRAMME — the write is refused, naming the region, never silently
// collapsed or cleared.
//
// # No clear builder
//
// core/civ ships no clear builder for this model, though the manual
// documents two clear forms (1A 00 <ch> FF, PDF p.213; whole command 0B,
// PDF p.203) — see core/driver/ic7700/caps.go's FieldErase grading and
// matrix §3.13, which additionally records that the scan edges P1/P2
// cannot be cleared by either form (PDF p.136's CLEAR column reads "No"
// for them specifically).
//
// # THE REGISTER
//
// Every ASSUMED value in this package's exported constants cites a matrix
// register entry by name (ic7700-name-pad-byte, ic7700-tagcharset-space,
// ic7700-mode-code-completeness); the full register, with each entry's
// lift, lives in the matrix document itself
// (docs/superpowers/icom-matrices/ic7700-capability-matrix.md §5) and is
// not reproduced here to avoid two copies drifting apart. Driver-homed
// entries (serial framing, control lines, echo, the 1A 00 read-request
// form, empty-channel reply, the all-FF question, storable frequency
// bounds, scan-edge honouring) are named in
// core/driver/ic7700/doc.go instead.
package ic7700
