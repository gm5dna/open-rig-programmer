// SPDX-License-Identifier: GPL-3.0-or-later

// Package ts2000 is core/kw's reading of the TS-2000, TS-2000X and
// TS-B2000's shared MR/MW memory-channel codec — three registry rows, one
// document, zero byte difference (ts2000-capability-matrix.md, all
// sections).
//
// SOURCE. "TS-2000_2000X_B2000" (PDF metadata title), one document covering
// all three models; provenance OFFICIAL
// (docs/fixtures-private/manuals/ts2000-manual-provenance.md,
// kasc.kenwood.com, SHA-256 recorded there, edition B62-1221-50). Citations
// below are "ts2000:NNNN", line NNNN of
// docs/fixtures-private/manuals/ts2000_manual_50_layout.txt
// (pdftotext -layout), read directly against the extraction rather than
// copied from any evidence file.
//
// WHAT THIS PACKAGE ADDS TO THE LIFT-K CODEC (layout.go's own doc comment
// carries the per-axis citations): a Layout for each of the three rows,
// sharing one 50-byte grid with the registered 590 pair and the TS-480
// (core/kw.RecordLen, unchanged), and five live field axes lift K minted
// for exactly this row — P10 (DCS code), P11 (REVERSE, Byte28Policy's new
// member), P12 (shift status), P13 (offset frequency) and P15 (Memory
// Group, Byte41Meaning's new member).
//
// SATELLITE MEMORY (SA/SI, MU) IS OUT OF THIS PACKAGE AND THIS WAVE.
// ts2000-capability-matrix.md §3: SA/SI is a separate 10-channel record
// with no frequency field of its own ("Use the FA (downlink) or FB
// (uplink) command", ts2000:11334), and MU is a scan-GROUP selector
// distinct from the P15 Memory Group NUMBER this package's Byte41 carries.
// Neither is a kw.Layout concern; both are recorded here so a later reader
// does not go looking for either in this file.
//
// EX/MENU INVENTORY IS OUT OF SCOPE THIS WAVE (spec §6 Q4), for all seven
// v1.7.0 Kenwood/Yaesu packages, not only this one. layout.go still pins
// MaxEXAddress — the codec's own BuildEXRead bound, cited to the highest
// menu number this document's own Appendix table prints (62, ts2000:10234-
// 10246) — but no core/kw/ts2000/exinventory*.go, menu CSV or generated
// EXItems table is built. A later phase that wants the settings surface
// (the core/driver/ts590 and core/driver/ts480 shape) transcribes the
// Appendix table (ts2000:10007-10246) the same way those two packages did
// from their own documents.
package ts2000
