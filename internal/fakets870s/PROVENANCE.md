<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# `internal/fakets870s` — where every byte it answers comes from

**Hardware status: UNVERIFIED.** No TS-870S has ever been asked anything by
this project. Nothing in this directory is an observation, and this fake
agreeing with `core/driver/ts870s` proves the two agree — nothing about the
radio.

## The one source

Every byte offset, field width, legend value and validation rule in this
package is transcribed from the **community mirror (rigpix.com) of the
official Kenwood document B62-1536-00** — the TS-870S's full instruction
manual, not a PC-command-only reference — and from nothing else. It is cited
throughout as `ts870s:NNNN`, naming a line of
`ts870s_manual_mirror_layout.txt` (`pdftotext -layout`, 9,758 lines); the
manual itself is gitignored (`docs/fixtures-private/manuals/`), so the
citations name where a chart is rather than linking to it — the convention
`core/kw/doc.go` uses. SHA-256 of the source PDF:
`143e8273bd352fd0e4fb3bc25c1f095d6d5ba3ec2a0289d1dd6fc11c881f165a`
(`ts870s-manual-provenance.md`, this document's own provenance note).

This package imports nothing project-internal, in this directory or any
beneath it, and `imports_test.go` enforces that mechanically and recursively.
The reason is in `doc.go` under THE HARD RULE: a fake built from the
production codec cannot disagree with it, so a systematic bug in that codec
would be applied identically on both sides of every "send a command, check
the reply" test and would never surface.

**This package was also authored independently of `core/kw/ts870s` and
`core/driver/ts870s` themselves**, per this milestone's fake-agent
quarantine (`.superpowers/sdd/2026-09-12-v170-kenwood-yaesu/briefs/fake.md`):
its author read `reviews/driver-ts870s.md` no further than that report's own
`## Verdict` heading, the matrix
(`docs/superpowers/ts870s-capability-matrix.md`, main checkout) and the
manual directly. Any disagreement between this package and the driver's own
codec is the point of the exercise, not a defect in either.

## A DISTINCT GRID, sharing no table with the 480/590 family

The TS-870S's own MR/MW frame is **22 bytes**, six live parameters (P1,
P3-P8) of the shared Kenwood family's sixteen parameter slots. Unlike the
480/590 pair, the seven parameters this row does not carry (P2, P9-P15) are
not printed-constant filler bytes — the Parameter Table prints "–" against
each, not a digit count, and no diagram cell sits between P1 and P3 in any
of the Read, Set or Answer rows. `doc.go`'s own section works through the
consequence: every field from P3 onward sits one byte earlier than the
family's registered offsets, and this package's `parser.go` constants are
this row's own, independently counted off the Read/Set/Answer diagrams
(`ts870s:9097`, `9112-9113`, `9139-9163`) — not derived from, or checked
against, `core/kw`'s registered 50-byte grid.

## The evidence posture of a memory image

Every byte of every record `image.go` ships is one of two things:

1. the **printed example frequency** `00014230000` — "Ex.: 00014230000 is
   14.230 MHz." (Format 4, `ts870s:8267-8270`);
2. a **printed legend value** — a mode nibble from Format 2's legend
   (`ts870s:8256-8262`), the lockout's `0`/`1` (Format 10), the tone mode's
   `0`/`1` (Format 1, `ts870s:8256`), or a tone index from the printed
   `01~39` chart (Format 14, `ts870s:8296-8299`).

No MR or MW frame is printed as a literal anywhere in this book, so the
**cross-field combination** of any shipped record is a synthetic composition
of those values, never an observed reading or a factory default —
`doc.go`'s register entry THE DEFAULT IMAGE'S RECORD COMPOSITION.

## What this package deliberately leaves small, and why

- **No name field, anywhere.** The matrix's `NoTag`/`TagLen: 0` finding
  (§1.6, citing the 12/09/2026 nameless-capability rule) is a total absence
  from the 22-byte record, not a narrower charset: this package builds no
  name byte, no echo and no charset check.
- **No EX (menu) inventory.** Out of scope for the whole v1.7.0 wave
  (spec.md §3); this package parses and answers no `EX` frame at all.
- **No MC (memory-channel selection).** The command exists in the book
  (`ts870s:9028`) but no graded `spec.Capabilities` field or bank `Field`
  depends on it — MR and MW both address a channel directly by P3 — so this
  package models no selector.
- **Stream-error tokens ARE scriptable, as of 13/09/2026.** This book cites
  "E;"/"O;" itself (`ts870s:8434-8450`, worded identically to
  `internal/fakets480`'s own citation of the same two tokens). The first cut
  of this package left them unscriptable because `reviews/driver-ts870s.md`'s
  `## Verdict` — the only part of that report read at the time — showed
  `core/driver/ts870s.Open` never building a live session. Its `## Follow-up`
  (read once the coordinator pointed at it) records that lift K `e7515d0`
  and the driver's own follow-up `e97d307` wired a real live session, so
  `WithStreamError` (options.go) now scripts either token, cited to the same
  lines. `doc.go`'s register entry STREAM ERRORS ARE SCRIPTABLE carries the
  revision.
