// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeft900 is an independent, stdlib-only FT-900 CAT simulator:
// the test double a future core/driver/ft890900 and its round-trip tests
// run against, built the way every other fake in this fleet is — from the
// radio's own manual and this milestone's capability matrix
// (docs/superpowers/ft900-capability-matrix.md), never from driver source
// (THE HARD RULE, imports_test.go). It is fakeft890's sibling, written
// independently rather than shared with it (see that package's doc.go for
// why a bug shared between a fake and its production codec is the failure
// mode this rule exists to prevent) — the two packages happen to end up
// structurally similar because the FT-890 and FT-900 documents describe
// byte-for-byte the same wire protocol (both matrices' own §1-§2), not
// because one was derived from the other's code.
//
// QUARANTINE (task brief 6A): this package was written against the
// capability matrix and the manual layout text
// (docs/fixtures-private/manuals/ft900_manual_mirror_ocr_layout.txt) only.
// No core/driver/ft890900 exists on this branch to read even by accident.
//
// THE ONE GENUINE WIRE DIFFERENCE FROM FT-890: 100 channels (1..100, not
// 1..32) and a 1941-byte full-dump (not 649) — state.go's slotCount and
// fullDumpLen constants. Everything else — frame shape, opcodes, record
// layout, the no-ack/silence model, the unmodelled rear sub-record — is
// identical, and is documented on fakeft890's own doc.go rather than
// repeated here.
//
// STATE MODEL, CHUNKED FULL-DUMP: see fakeft890's doc.go — identical
// design, independently written.
package fakeft900
