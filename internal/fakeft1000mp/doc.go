// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeft1000mp is an independent, stdlib-only FT-1000MP/Mark-V
// simulator — the test double core/driver/ft1000mp (a later phase) runs
// against, in the shape internal/fakeft950 and this milestone's own
// internal/fakeft890/internal/fakeft900 play for their own radios.
//
// QUARANTINED (this milestone's Phase 4, brief 6B): built from
// docs/superpowers/ft1000mp-capability-matrix.md and the two manuals it
// cites (docs/fixtures-private/manuals/ft1000mp*, gitignored, Yaesu
// copyright — "layout:N" citations name where a chart is, not a link)
// ONLY. core/driver/ft1000mp was never read, and does not exist on this
// branch — see imports_test.go's fence.
//
// # Scope this milestone actually ships
//
// Only the opcodes the brief names are wired: FAH (Read Flags, identity),
// 10H/U=00H (the one full-dump Status Update this radio's read model uses
// at all — matrix §1.6, "internal full-dump-and-index, not per-channel"),
// 03H (VFO/MEM Store/Enter, K=00H only — matrix §1.8's override) and the
// handful of VFO-mutating writes Store needs something to have mutated
// first: 0AH (Set VFO-A frequency), 0CH (Set Mode), 84H (Shift). SPLIT,
// LOCK, Recall Memory, A/B select, UP/DOWN, Clarifier, Tone, Repeater
// Offset, Pacing, PTT and Mask/Un-Mask are all UNWIRED — no capability
// this milestone claims needs them, and an unhandled frame gets silence
// (this family's own no-NAK convention, matrix §Context/framing.go), not
// a wrong answer. Add a case in parser.go's handleFrame when a later
// phase's round-trip test needs one.
//
// # Record shape (16 bytes, matrix §1.2)
//
// Byte 0 Band/flags, 1-4 frequency, 5-6 clarifier, 7 mode, 8 IF filter
// offset, 9 VFO/MEM flags, 10-15 unused. This fake only ever writes bytes
// 1-4, 7 and 9 (the fields §1.4-§1.5's mutating opcodes above can reach);
// everything else stays zero — not a claim those bytes are always zero on
// a real radio, just that nothing here ever sets them.
//
// Frequency is genuinely two different wire encodings, matrix §1.3: the
// write side (0AH) is packed BCD, least-significant decimal pair in the
// first argument byte (the convention core/bincat.EncodeBCD documents for
// this whole family — reimplemented independently here, per the hard
// rule below, never imported); the read side (the 16-byte record) is
// nibble-decimal, and NOT the same bytes — the manual's own
// worked examples for the identical 1,425,000-tens-of-Hz value give "00 50
// 42 01" written and "00 05 24 10" read back. freqToRecordBytes
// reproduces that second form exactly (it is the reverse of the first
// form's decimal digit string, packed two digits per byte) —
// freqDigits_test.go pins the worked example itself, not just the shape.
//
// # Channel numbering (matrix §1.4, ASSUMED 1-based override)
//
// Store's channel argument X (01H-71H) is taken as X = the memory's
// 1-based number directly (channel 1 -> X=01H), per Stuart's 15/09/2026
// override, not derivable from the manual's own self-contradicting text.
// A real driver's read-back-after-write is what would ever catch this
// being wrong — nothing here retries or hides a mismatch, because nothing
// here can produce one against itself.
//
// # The hard rule: NOTHING project-internal but fakepipe
//
// fakeft1000mp MUST NOT import any other package of this project — not
// core/bincat (this milestone's own codec: sharing it with the driver
// would let one bug in the frame/BCD layer pass an end-to-end cross-check
// invisibly, the whole reason this family of fakes exists independently),
// not core/driver/ft1000mp (absent on this branch anyway), not core/civ,
// not core/spec, not any sibling fake. internal/fakepipe (protocol-free
// net.Pipe plumbing) is the one permitted share (imports_test.go).
package fakeft1000mp

// writeTrialsComplete records this package's one honest status line: NO
// FT-1000MP OR MARK-V HAS EVER BEEN ASKED ANYTHING BY THIS PROJECT
// (no-additional-hardware-2026-09-13; matrix's own opening line).
const writeTrialsComplete = false

// THE ASSUMED REGISTER — every place this fake had to decide something
// neither manual nor the matrix settles outright.
//
//  1. STATUS FLAGS (the dump's leading 6 bytes, and FAH's own flag bytes)
//     ARE ALWAYS ZERO. Nothing this milestone's read model claims depends
//     on any individual flag bit (matrix §1.6 reads by record position,
//     never a flag), so none is modelled.
//     (state.go: buildFullDump, parser.go: handleFAH)
//
//  2. THE "CURRENT MEMORY CHANNEL" DUMP BYTE (byte 7) IS ALWAYS 00H. Only
//     Recall Memory (02H) or a Store/Enter's own current-channel side
//     effect would ever move it, and neither Recall nor "does Enter also
//     select" is wired — matrix §1.6 indexes the dump by fixed record
//     position, never this byte.
//     (state.go: buildFullDump)
//
//  3. VFO-B AND THE "CURRENT OPERATING DATA" RECORD ARE UNMODELLED. No
//     A/B-select opcode is wired (above), so there is only ever one live
//     VFO register; "current operating data" mirrors it, and VFO-B's own
//     16 bytes stay zero.
//     (state.go: buildFullDump)
//
//  4. THE FULL-DUMP CHUNKING OPTION'S SIZES ARE THIS PACKAGE'S OWN, NOT A
//     MANUAL FACT. The manual states pacing exists (0EH, unwired) and
//     that a full dump takes "just under 5 seconds" at zero pacing
//     delay — WithFullDumpChunking exists only to let a test make one
//     read cross an arbitrary timeout, not to reproduce a real radio's
//     timing.
//     (options.go: WithFullDumpChunking)
