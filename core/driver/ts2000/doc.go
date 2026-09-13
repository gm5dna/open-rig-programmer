// SPDX-License-Identifier: GPL-3.0-or-later

// Package ts2000 is the driver for the TS-2000, TS-2000X and TS-B2000:
// three registry rows, one 50-byte MR/MW codec (core/kw/ts2000), zero byte
// difference between them (ts2000-capability-matrix.md §1-§2).
//
// CONSTRUCTORS: NewTS2000, NewTS2000X, NewTSB2000 — no bare New (the
// ic7851 shape: one unexported modelParams, three constructors closing
// over it). Options: WithSimulatedProfile, WithConsentedUnverifiedWrites.
//
// NO RADIO OF ANY OF THE THREE ROWS HAS EVER BEEN ASKED ANYTHING BY THIS
// PROJECT (matrix, line 8). Every capability value below carries its own
// grade (MANUAL-EVIDENCED / CHOICE / ASSUMED / CANNOT ESTABLISH) in caps.go
// and the matrix, and RealHardware sessions get the all-Unverified
// CapabilitiesUnverified profile while writeTrialsComplete is false.
//
// WRITE POSTURE: "write existing" (spec.md's own row for this package),
// the core/driver/ts890/ts990 READ-THEN-WRITE shape, not the TS-480's
// blanket A22 refusal. WriteChannel performs one pre-write MR read and
// PRESERVES four raw values this milestone models no spec.Field for — P10
// (DCS code), P11 (REVERSE) and P15 (Memory Group), always, and P14 (the
// tuning-step index) only when the write does not cross its mode-
// conditional legend boundary (SSB/CW/FSK vs AM/FM, ts2000:11508-11521) —
// write.go's own doc comment has the full byte-level reasoning. Two new
// refusals besides the family's usual ladder: registerCreate (the
// pre-write read shows the slot unassigned; this programme does not
// create channels) and registerP14Family (the write's mode and the
// pre-write read's mode fall in different P14 legend families).
//
// READ SURFACE: nine spec.Fields (frequency, mode, tag, scan_skip,
// tone_mode, duplex, offset, tone_tx, tone_rx) — wider than the TS-480's
// five, because this document prints its OWN 39-entry CTCSS/tone chart
// (caps.go's kenwoodTS2000CTCSSTones, ts2000:3837-3847) where the TS-480's
// prints none, and because P12/P13 (matrix's live Shift/Offset axes) map
// onto the Icom vocabulary half (FieldDuplex/FieldOffset, design decision
// 6) rather than sitting unexpressed. DCS (P10), REVERSE (P11) and Memory
// Group (P15) are parsed by the codec and deliberately NOT published —
// caps_test.go's unexpressedFields gives each its own reason.
//
// THE BOOK CITED FOR WIRE-LEVEL ERRORS IS core/kw.Book480, NOT Book590 —
// core/kw/ts2000/layout.go's own doc comment carries the full reasoning
// (this document's TY probe and its "O;" cause sentence both match
// Book480's, not Book590's) and the citation-line cost that choice
// accepts (Book480's own line numbers, not this document's).
//
// CATID IS THE SAME ASSUMED VALUE ON ALL THREE ROWS ("019", MANUAL-
// EVIDENCED for TS-2000 alone; matrix §4) — a consequence worth restating
// here: this driver's Open cannot distinguish a TS-2000X or a TS-B2000
// from a TS-2000 by wire identity, only by which constructor the caller
// chose.
//
// SATELLITE MEMORY (SA/SI, MU) IS OUT OF THIS WAVE, spec §6 open question 1
// (matrix §3, IC-9100 D-STAR-block precedent) — recorded in
// TestDeliberatelyZeroAudit, not built.
package ts2000
