// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7800 holds the Icom IC-7800's CI-V dialect: the memory record's
// geometry, its three value enums, its name charset and the civ.Profile
// that binds them. It is DATA ONLY - no driver, no fake, no registration,
// no session, no wire. The package cannot register itself with the
// application: SupportedModels derives solely from internal/wiring's
// driver table.
//
// # Provenance
//
// Everything here comes from the IC-7800 Instruction Manual, Revision 16
// (docs/fixtures-private/manuals/ic7800_fullmanual_16.pdf, gitignored, so
// the page references below are citations rather than links). Its CI-V
// material is Section 14, "CONTROL COMMAND", PDF pp.200-217 (printed
// folios 14-2 to 14-19); the memory record itself is PDF p.211 (folio
// 14-12/14-13). The radio's printed default CI-V address is 6Ah and the
// controller's is E0h, so every frame this package builds is
// FE FE 6A E0 ... FD.
//
// The graded evidence authority is
// docs/superpowers/icom-matrices/ic7800-capability-matrix.md, written from
// one reading of the manual alone — UNLIKE the IC-7610 lineage's four
// independently-blind legs (a field ledger, a geometry witness, a
// transcription and a golden-vectors leg), because no separate CI-V
// Reference Guide exists for this model and its byte-strip indices (q, w,
// e, r...i, o, !0, !1, !2...) survive text extraction losslessly, so no
// raster re-measurement leg was needed either (matrix §0). This package's
// testdata/ follows suit: one real evidence artefact,
// ic7800-transcription.csv (carried over unchanged from the earlier S1
// sweep, testdata/SHA256SUMS-frozen), plus hand-derived golden vectors and
// their provenance note — not a fabricated set of independent legs.
//
// NO IC-7800 HARDWARE HAS EVER BEEN ASKED ANYTHING by this project. Every
// statement in this package is a reading of a manual, and the register at
// the foot of this file is the list of the places where it is not even
// that.
//
// # A literal IC-7610 copy, by spec.md §1's own finding
//
// spec.md §1's IC-7800 entry states "every field SAME position/width" as
// the already-registered IC-7610, and independent re-derivation for the
// matrix confirmed it: no offset difference anywhere in the 25-byte
// record. The only wire-level deltas turned out to already be true of
// THIS codebase's IC-7610 package too — its own toneModeEnum already
// carries three values with no DTCS and its own modeEnum already lacks
// WFM (0x06) — so this package's enums are, in the event, identical to
// core/civ/ic7610's, and every offset, encoding and byte order below is a
// literal copy of it with only the model name and address changed.
//
// # The three lengths
//
//	RecordOnlyLength  25   the 1A 00 data block EXCLUDING the two channel
//	                       selector bytes q,w - what civ.Profile carries,
//	                       and what BuildMemorySet's <record> denotes
//	DataAreaLength    27   the 1A 00 data block INCLUDING them - the
//	                       evidence file's own eight-term addition,
//	                       1+5+2+1+3+3+10 = 25, plus the 2-byte selector
//	AddressBytes       2   <ch-hi> <ch-lo>, civ.AddressFormFlat
//
// A 1A 00 set frame is therefore 34 bytes (6 + 2 + 25 + 1), a 1A 00 read
// frame 9 (6 + 2 + 1) and a 19 00 read frame 7 (6 + 1).
//
// # Ruling E6: the two nibbles this record deliberately does not map
//
// Byte 0 (printed 'e') carries a four-valued SELECT-group marker, printed
// 00: OFF / 01: ★1 / 02: ★2 / 03: ★3 (PDF p.211). Byte 8 (printed '!1')'s
// HIGH nibble carries a four-valued data mode, 0: OFF / 1: DATA 1 /
// 2: DATA 2 / 3: DATA 3 (PDF p.211). Neither has a faithful neutral home —
// codeplug.ChannelData.ScanSkip and .DataMode are both BoolField — and a
// 4→2 collapse would rewrite a user's SELECT group or data mode on every
// write-back while readback verification compared equal. Tier ruling E6
// (established for the IC-7610 lineage, applied identically here) settles
// this: a driver may write a slot ONLY when its unmapped regions equal the
// profile's all-zero Fixed template; anything else is REFUSED, never
// rewritten. SelectNibbleOffset and DataModeNibbleOffset name the two
// regions so the driver's refusal check does not re-derive them.
//
// # No clear builder, though this radio documents a clear form
//
// PDF p.211's channel-number field note says a clear is the channel
// number followed by FF in place of the data bytes; PDF p.201 (folio
// 14-3)'s command table also carries a standalone command 0B, "Memory
// clear", with no sub-command and no data. core/civ ships NO clear
// builder for either shape, and Profile.AllowedCommand has no branch that
// admits either: only 19 00, a valid 1A 00 read and a re-validated 1A 00
// set. Every Icom driver in this tier gives spec.FieldErase the zero
// FieldSupport and spec.ConsentUnverifiedWrites structurally never
// consents it, so core/clone/execute.go's DiffErased branch stays
// unreachable for this model too.
//
// # THE REGISTER
//
// Every entry names the assumption, what depends on it, and what a future
// write-trial milestone would need to lift it — none of which has
// happened: no IC-7800 has ever been connected to this project.
//
//   - ic7800-record-length — that the eight printed field widths derive to
//     25 record-only and 27 data-area bytes. The document prints no
//     explicit byte count for 1A 00; the total is a derivation, corroborated
//     independently by the matrix and by evidence/ic7800.md's own addition.
//     LIFT: read M-CH01 with 1A 00 and count the bytes between the
//     selector and FD.
//   - ic7800-name-pad-byte — that a name shorter than ten characters is
//     padded with 0x20. The legend says "Up to 10 characters" and states
//     nothing about what pads the rest. LIFT: set a three-character name
//     from the front panel, read it back with 1A 00, and record the seven
//     unused bytes.
//   - ic7800-mode-code-completeness — that the ten printed mode codes
//     (00,01,02,03,04,05,07,08,12,13) are the complete set, i.e. that 06
//     and 09-11 name nothing on this model. LIFT: attempt to select each
//     of those codes from the front panel and record whether the radio
//     offers them.
//   - ic7800-select-marker-semantics / ic7800-data-mode-nibble — the two
//     E6-unmapped nibbles' values, as read back from a channel set to a
//     non-OFF SELECT group or data mode from the front panel. LIFT: set
//     memory channel 05 to ★2 (or DATA 2), read it with 1A 00, and record
//     the byte.
//   - ic7800-default-baud — the CI-V default baud is documented as "Auto"
//     (PDF p.177, folio 12-21), a negotiation mode rather than a fixed
//     rate; which numeric rate a driver should open at is UNRESOLVED here,
//     same as the IC-7610's own D5 register carries for its own arbitrary
//     19200 choice. Register home for the resolution:
//     core/driver/ic7800/doc.go.
//   - ic7800-scan-edge-record-fields — whether the radio honours every
//     record field (not just frequency) on a P1/P2 scan edge. The
//     `1A 00` diagram is one diagram governing all three address forms
//     (matrix §2), so the record SHAPE is manual-evidenced; whether each
//     field is HONOURED there is not.
package ic7800
