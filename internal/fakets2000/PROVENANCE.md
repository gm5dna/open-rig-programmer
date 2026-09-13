<!-- SPDX-License-Identifier: GPL-3.0-or-later -->

# `internal/fakets2000` — where every byte it answers comes from

**Hardware status: UNVERIFIED.** No TS-2000, TS-2000X or TS-B2000 has ever
been asked anything by this project (`docs/superpowers/ts2000-capability-matrix.md`'s
own opening line). Nothing in this directory is an observation; this fake
agreeing with `core/driver/ts2000` proves the two agree with each other, and
nothing about any radio.

## The two sources

Every byte offset, field width and legend value in this package is
transcribed from the TS-2000/TS-2000X/TS-B2000 instruction manual's own **PC
CONTROL COMMAND TABLES** appendix
(`docs/fixtures-private/manuals/ts2000_manual_50_layout.txt`, gitignored, so
citations here name a line rather than link to it) and from
`docs/superpowers/ts2000-capability-matrix.md`'s own transcription and
grading of that appendix — and from nothing else. This package imports
nothing project-internal, in this directory or any beneath it,
`imports_test.go` enforces that mechanically and recursively, and the reason
is doc.go's THE HARD RULE: a fake built from the production codec cannot
disagree with it, so a systematic bug there would sit on both sides of every
test and never surface.

**It also imports nothing from `internal/fakets480` or `internal/fakets590`.**
The three books print an overlapping 50-byte grid and give five of its bytes
(P10, P11, P12, P13, P15) an entirely different job on this row — live data
where the other two print a constant — so a borrowed table would be a silent
wrong answer on exactly the bytes the matrix flags as this row's whole point.

## The evidence posture of a memory image

Every byte of every record `image.go` ships is exactly one of:

1. a **printed example** — the frequency `00007000000`, "FA00007000000;" for
   7 MHz (`ts2000:9567`, repeated at `9595`/`9600`/`9607`);
2. a **printed legend value** — the mode nibble `1` (LSB, MD's first value,
   `ts2000:10611`), or one of `emptyRecord`'s own "nothing here" spellings
   (tone/CTCSS/DCS/offset/step at zero, REVERSE and Shift at their lowest
   printed value, Memory Group `0`);
3. the **name field**, filled with **eight spaces** — no name string is
   printed anywhere in this document, so blank is the least this fake
   invents, exactly as every sibling Kenwood fake's own name default.

No MR, MW or MC frame is printed as a literal anywhere in this document, so
the record's CROSS-FIELD COMBINATION has never been printed or observed.
These are not observed contents and not factory defaults.

## The unwritten-channel answer is weaker evidence than either sibling's

The TS-590 pair's book states outright what an empty channel's `MR` answer
holds: "If the selected channel is empty, P4 ~ P15 will be 0 and P16 will be
blank." (`590:1492-1493`). The TS-480's book says nothing of the kind, but at
least shares close enough a family that reading the 590 pair's sentence
across was arguable (its own doc.go, entry 3). **This document says nothing
about an empty channel at all**, and the matrix says so explicitly (§5, §6
item 5) — there is no sibling sentence to read across here either, because
this row's OWN book is silent, not merely different. `emptyRecord()` in
`image.go` is offered anyway, purely so the protocol has something to answer
with; it is invented, not assumed, and every byte of it is overridable per
channel via `WithChannel`/`WithFactoryImage`/`WithEmptyChannel` — the "option
where the manual leaves it open" the brief for this package asks for.

## Why one package answers as three rows with one wire identity

`ID` answers `"019"` on every construction of this Radio, whatever
`WithModelName` was given (`ts2000:10429-10441`, "019: TS-2000" the only
value printed). The matrix's own evidence files (`ts2000x.md`, `tsb2000.md`)
found no second or third `ID` value, and no "2000X" or "B2000" qualifier
anywhere beside `MR`/`MW`/`MC`/`ID` in the command tables — the CATID for the
X and B2000 rows is ASSUMED, not printed, on that basis. `WithModelName`
therefore changes only what `Model()` reports for a test's own bookkeeping;
it is not a claim that a real TS-2000X or TS-B2000 would answer any
differently on the wire, because nothing in this document says it would.

## Rules not modelled, and why

- **Satellite Memory (`SA`/`SI`)** — a separate 10-channel record with no
  frequency field, out of this wave (matrix §3, spec §6 open question 1).
  Not simulated, not even as an unsupported stub.
- **Tuning steps and the EX/menu surface** — design addition D8 does not
  apply to this wave (matrix §4); no menu chart is cited for this row, so
  this package carries no `ex.go`.
- **FV/firmware, TY** — not cited anywhere in the matrix or the transcribed
  appendix pages read for this package; not modelled, to avoid inventing a
  command this document does not print for this row.
- **Fault injection beyond the book** — dropped replies, garbled bytes,
  chunked writes: these exercise `core/transport.Engine`, one
  model-independent implementation already covered elsewhere.
