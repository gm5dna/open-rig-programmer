// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7700 is the Icom IC-7700 driver: the capability profiles, the
// session probe, the acknowledged write, and the serial-framing report. It
// sits above the neutral driver.Driver/Session seam and below
// core/civ/ic7700's profile, which it never bypasses: every frame this
// package sends goes through civ.Profile.AllowedCommand.
//
// # Provenance
//
// Everything model-specific here comes from the reviewed capability matrix
// (docs/superpowers/icom-matrices/ic7700-capability-matrix.md rev 1), built
// from the IC-7700 Instruction Manual, Revision 7, 232 pages. There is no
// separate CI-V Reference Guide for this model; CI-V is Section 14 of the
// full manual. NO IC-7700 HAS EVER BEEN CONNECTED TO THIS PROJECT.
//
// # Record geometry and ruling E6
//
// See core/civ/ic7700/doc.go for the full record layout and the three
// unmapped regions ruling E6 governs (idx0 whole byte, idx8 high nibble,
// idx20-28). This package's write.go enforces E6 at rung 6 of
// WriteChannel's ladder: a slot whose unmapped bytes disagree with the
// profile's Fixed template is REFUSED, never rewritten.
//
// # The TX-duplicate block (idx15-28) — this model's largest single
// # decision this wave made for it
//
// The manual states the block is "still necessary" even when Split is OFF
// (PDF p.213, matrix §1 row 4). This package's write policy, settled by
// coordinator ruling 12/09/2026:
//
//   - idx15-19 (the frequency sub-span) is mapped to spec.FieldTxFrequency
//     and is ALWAYS written: the caller's value when Known (a genuine
//     split channel), otherwise the receive frequency MIRRORED IN. This is
//     what caps.go's SimplexTx: spec.SimplexTxEqualsRx states.
//   - idx20-28 (nine bytes — mode, filter, the data/tone-type nibble,
//     tone_tx, tone_rx, each a "second TX-side copy" of a field the tier
//     vocabulary already carries once) have NO neutral field and are left
//     entirely UNMAPPED, governed by the same E6 Fixed-template refusal as
//     idx0 and idx8's high nibble — not mirrored via a reused civ.FieldSpan
//     the way core/driver/ic7300's own (differently-scoped) TX-dup block
//     is. A channel whose TX-dup mode/filter/tone bytes are non-zero on
//     read CANNOT BE WRITTEN BACK by this driver.
//
// This is a genuine, stated DEVIATION from the ic7300 sibling's approach,
// made because the coordinator's ruling explicitly separated "the one field
// the tier vocabulary already has a slot for" (frequency, mirror it) from
// "everything else" (no slot exists, leave it E6-unmapped) rather than
// inventing mirroring behaviour for five bytes no ruling asked for.
//
// # No clear builder
//
// core/civ ships no clear builder for this model. FieldErase carries the
// zero FieldSupport on both banks (matrix §3.13); the scan edges P1/P2 are
// additionally documented as non-clearable even by the mechanism that
// clears a regular channel (PDF p.136, folio 8-1, the CLEAR column reading
// "No" for scan-edge channels specifically).
//
// # No USB-CDC CI-V port
//
// Unlike every other model in this tier, the IC-7700 has no USB serial
// CI-V endpoint at all (matrix §3.1/§3.15(3)): CI-V reaches it only
// through the [REMOTE] jack (behind an external CT-17 level converter) or
// the [RS-232C] D-sub port directly, set to its factory "CI-V" output
// mode. StopBits (ic7700.go) and TestOpen_ControlLinesAreNeverToggled
// (framing_test.go) record what follows from that.
//
// # DefaultBaud — a CHOICE, not a reading
//
// The factory default is "Auto" (PDF p.179, folio 12-17), which
// DefaultBaud int cannot represent (matrix §3.16 ADDED-1). caps.go declares
// 19200 — this radio's own fastest LISTED rate, since unlike every 7610-
// family sibling in this tree it has no rate above 19200 to prefer.
// SAFE BECAUSE OF THE FAILURE MODE, not the choice: Open's probe requires
// an address-matched 19 00 reply, so a wrong guess costs a clean timeout,
// never a wrong byte.
//
// # Driver-homed register entries
//
// Every entry below is named in the matrix's own §5 register (which also
// carries the civ-homed ones, reproduced in core/civ/ic7700/doc.go). Each
// is ASSUMED and awaits a real IC-7700's own capture:
//
//   - ic7700-framing-8n1 — that the [RS-232C] CI-V link is 8-N-1.
//   - ic7700-control-lines-inert — that neither RTS nor DTR keys the radio
//     or blocks CI-V traffic on this port.
//   - ic7700-no-echo-setting — that no USB-echo-style setting exists to be
//     configured (there being no USB-CDC port for one to apply to).
//   - ic7700-1a00-read-request — that a bare 1A 00 <ch> read (no data
//     bytes) is the accepted request form.
//   - ic7700-empty-channel-reply / ic7700-all-ff-empty — the two SEPARATE
//     readings of what an unwritten channel answers (FA, per the general
//     OK/NG scheme) and of whether an all-FF record also means empty.
//     recordIsAbsent (read.go) implements the all-FF half; T4's
//     transport.ErrRejected handling implements the FA half.
//   - ic7700-name-pad-byte — the pad byte for a name shorter than ten
//     characters (0x20, ASSUMED).
//   - ic7700-scan-edge-record-fields — whether every mapped field is
//     HONOURED on a scan edge, not merely shaped the same (matrix §2 SCAN
//     header). This package makes no claim either way: ReadChannel and
//     WriteChannel treat P1/P2 exactly like a memory channel.
//   - ic7700-storable-frequency-floor / -ceiling — this radio's own tuning
//     bounds, narrower (perhaps) than the record's encoding bounds
//     MinFreqHz/MaxFreqHz declare (caps.go).
//   - ic7700-mode-code-completeness — that codes 06 and 09-11 name nothing.
//   - ic7700-simplex-tx-value (matrix §1 row 4) — what the TX-duplicate
//     block holds on a real Split-OFF channel. This package's own write
//     policy (mirror the RX frequency in) is a CHOICE this entry's lift
//     would confirm or correct, not evidence for it.
package ic7700
