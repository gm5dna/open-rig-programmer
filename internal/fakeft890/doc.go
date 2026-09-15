// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeft890 is an independent, stdlib-only FT-890 CAT simulator:
// the test double a future core/driver/ft890900 and its round-trip tests
// run against, built the way every other fake in this fleet is — from the
// radio's own manual and this milestone's capability matrix
// (docs/superpowers/ft890-capability-matrix.md), never from driver source
// (THE HARD RULE, imports_test.go; see any sibling fake's own doc.go for
// the rationale: a systematic bug in the production codec must not be
// able to pass end-to-end tests invisibly by also being present here).
//
// QUARANTINE (task brief 6A): this package was written against the
// capability matrix and the manual layout text
// (docs/fixtures-private/manuals/ft890_manual_mirror_ocr_layout.txt) only.
// No core/driver/ft890900 exists on this branch to read even by accident.
//
// WIRE SHAPE. Every command is a fixed 5-byte frame: four argument bytes
// followed by the opcode byte, sent left to right (matrix §1.1) — no
// preamble, no terminator, so reassembly is nothing more than grouping the
// incoming byte stream into 5-byte chunks. This family is fire-and-forget:
// only the Status Update opcode (0x10) ever produces a reply, and an
// out-of-range parameter gets SILENCE, never an error frame — "If you send
// a parameter that is out of range ... the FT-890 should do nothing"
// (manual p.34, matrix §1.8).
//
// STATE MODEL. One active VFO (A or B, selected by opcode 0x05) carries
// frequency/mode/clarifier/tone/shift; the write opcodes this milestone's
// choreography uses (0x05, 0x0A, 0x0C, 0x09, 0x84, 0xF9, 0x90) mutate
// whichever VFO is active, and Store (VFO->M, 0x03) copies the active
// VFO's state into a channel's FRONT 9-byte sub-record. The channel's REAR
// 9-byte sub-record is never computed by this fake: it is seeded once at
// construction and echoed back verbatim on every read, Store included.
// THIS IS NOT A PRESERVATION CLAIM ABOUT ANY REAL FT-890 — the matrix's
// own §2 states plainly that the real radio's post-write content there is
// "whatever the radio itself derives", with no documented operation this
// codec could model even in principle (Codex #6). Echoing fixed bytes back
// is this fake's own simplification for read-after-read consistency only;
// a test that treated it as evidence the real radio preserves anything
// there would be fabricating agreement this matrix explicitly withdraws.
//
// CHUNKED FULL-DUMP. WithChunkedFullDump splits a U=0 (full RAM table)
// reply into several separate writes with a sleep between them, modelling
// the manual's own documented worst case ("almost 3 minutes" at maximum
// Pacing delay, p.31) closely enough to let a caller's read timeout
// actually be exercised, rather than every fake reply arriving as one
// instantaneous write.
package fakeft890
