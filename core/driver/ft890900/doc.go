// SPDX-License-Identifier: GPL-3.0-or-later

// Package ft890900 implements driver.Driver for the Yaesu FT-890 and
// FT-900 — one shared package, per plan.md's Phase 3 Agent A brief and
// spec.md's v2 write model, because the two radios share an identical
// 5-byte binary-CAT opcode set and 19-byte memory record (spec.md §Frame
// grammar table): only the slot count (32 vs 100) and full-dump length
// (649 vs 1941 bytes) genuinely differ.
//
// NewFT890 and NewFT900 are separate constructors, not one constructor
// taking a model enum: nothing else in this family varies per model, but
// the two radios cannot be told apart at all without physically probing
// the port (§Identity below), so a caller must already know, from
// whatever chose the port, which one it is asking to open — this package
// merely proves that choice was right.
//
// # Identity
//
// Neither radio carries a CAT-ID byte on the wire (point-to-point RS-232,
// no device address); FT-890 and FT-900 share an identical opcode set,
// making them the hardest pair in this project to discriminate (spec.md
// §Identity probe). Open sends two fixed 19-byte Status-Update probes
// (opcode 10H, U=4): CH=01H, which both radios must answer, then CH=21H
// (channel 33) — out of FT-890's documented 1-32 range, in FT-900's 1-100
// range. The family's own documented no-ack/no-NAK behaviour ("an
// out-of-range parameter makes the radio do nothing", spec.md §Context)
// is the discriminating signal at that boundary: FT890's Open treats an
// answer to the boundary probe as ErrWrongRadio and silence as
// confirmation; FT900's Open treats it the other way round. Neither probe,
// nor any step of the write choreography, reaches the wire before both
// probes resolve.
//
// The boundary probe (channel 33) is one channel past FT-890's
// documented top channel (32). bincat.Profile.AllowedCommand gates every
// outbound Status-Update read to its own SlotCount, so ft890EngineProfile
// (profile.go) sets SlotCount to 33, not 32, purely so this one
// diagnostic frame can be transmitted and its silence (or answer)
// observed — the true 32-channel range is what caps.go and slotToChannel
// (read.go) advertise and enforce for every ordinary Read/WriteChannel
// call; channel 33 is never reachable through the public Session API
// except by Open's own probe.
//
// # Write model
//
// There is no per-channel "write record N" verb: every write is VFO→M
// (spec.md §Write model). WriteChannel runs the family's 8-step
// choreography (A/B-select, SetFreq, SetMode, Clarifier, Shift, Offset,
// Tone, Store) against VFO-A, via core/clone.VFOStateRestorer — this
// package's Session implements SnapshotVFOState/RestoreVFOState
// (vfostate.go) so the clone service can save and restore whatever the
// operator had on VFO-A around a batch of channel writes.
//
// # Known, permanent limitations
//
//   - The 19-byte record's second 9-byte sub-record (offsets 10-18) is
//     not modelled at all — core/bincat.Record doesn't decode it, and
//     Store/Enter overwrites the WHOLE record from live VFO state, so
//     there is no coherent "preserve the current unknown bytes"
//     operation available here even in principle (spec.md Amendment
//     14/09/2026, Codex #6).
//   - The clarifier-set opcode (09H)'s argument byte layout could not be
//     recovered from either manual (core/bincat's own OpClarifier doc
//     comment); this driver can transmit only the documented
//     clarifier-OFF pattern and refuses a write that asks for a non-zero
//     clarifier rather than guess an encoding (write.go).
//   - Neither manual documents a CTCSS on/off toggle distinct from the
//     tone-code byte itself (capability matrices, §"CTCSSStates: OPEN");
//     FieldCTCSSState is graded Unsupported, and this driver cannot
//     control whether a written tone is actually applied.
//   - writeTrialsComplete is false for both radios: every write-side
//     field is Unverified, gated behind ConsentedUnverified (caps.go).
package ft890900
