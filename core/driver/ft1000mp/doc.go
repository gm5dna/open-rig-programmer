// SPDX-License-Identifier: GPL-3.0-or-later

// Package ft1000mp is the driver for the Yaesu FT-1000MP and Mark-V
// FT-1000MP — one row, both bodies (spec.md's Decisions 14/09/2026 #2;
// docs/superpowers/ft1000mp-capability-matrix.md header: "one row covers
// both bodies", no CAT-chapter passage distinguishes them). It speaks
// core/bincat's fixed 5-byte binary frames, not core/cat's ASCII NEWCAT
// grammar or core/civ's CI-V framing.
//
// # Override in force (Stuart, 15/09/2026)
//
// spec.md's final Amendment ships this driver's Store/Enter (VFO→M)
// write path CONSENT-GATED — Unverified, relabelled ConsentedUnverified
// under WithConsentedUnverifiedWrites, the SAME mechanism every other
// write-side field in this project's Unverified tier uses
// (core/driver/ftdx10/caps.go's CapabilitiesUnverified precedent) — NOT
// Unsupported as plan.md's Phase 0 v2 text originally had it. The
// Store/Enter opcode's own `K` byte position is ASSUMED (matrix §1.8);
// this milestone ships only K=00H (Enter), never Mask/Un-Mask. The
// channel-numbering base is ASSUMED 1-based (matrix §1.4); the runtime
// check is a read-back after every Store, never a paper resolution — a
// mismatch is reported as a failed write (WriteChannel's own comment),
// nothing is retried.
//
// # Read design — full-dump-and-index, not per-channel
//
// ReadChannel never sends a per-channel Status Update: the channel
// argument's numbering base is exactly what §1.4 leaves ASSUMED, and a
// per-channel read would stake correctness on it. Instead ReadChannel
// fetches the whole U=00H dump (1,863 bytes) once per session, caches it,
// and indexes into it by the dump's OWN fixed record position — which
// does not depend on which numbering convention is correct at all
// (matrix §1.6; plan.md's Phase 3 Agent B v2 read design, Codex #10).
//
// # What this driver does not do
//
// No tag/name route exists over CAT at all (NoTag, matrix §0/§3). No
// CTCSS tone byte exists in the 16-byte record (matrix §2): Setting a
// live VFO-level tone (opcode 90H) is real, but nothing survives a
// Store/Enter to persist it, so FieldCTCSSTone/FieldCTCSSState are
// Unsupported and this driver's write choreography never sends 90H.
// Mask/Un-Mask (K=01H/02H) are out of scope. Registration
// (internal/wiring's fakeDrivers map, README, radio-roadmap.md) is
// Phase 5, not this package.
package ft1000mp
