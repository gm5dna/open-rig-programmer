# IC-7600 memory-record transcription

## Source

`docs/fixtures-private/manuals/ic7600_fullmanual_4.pdf` (gitignored),
196 PDF pages, "IC-7600 Instruction Manual". Record diagram: PDF p.178
(folio 169), "Memory content setting" / `Command: 1A 00`.

## Provenance

`ic7600-transcription-b.csv` is the Tier 7 S1 sweep's own evidence leg,
reproduced verbatim from
`.superpowers/sdd/2026-09-12-tier7-s1-icom/evidence/ic7600-transcription.csv`.
It was produced independently of, and before,
`docs/superpowers/icom-matrices/ic7600-capability-matrix.md` rev 1 (which
cites and supersedes it - matrix §0 "S1 evidence — availability"). The
matrix's own §3.11 arithmetic cross-checks against this transcription
and finds "no disagreement... a confirmation, not a new finding."

Nine rows, one diagram (D1) - the S1 leg's own scope note reads "one
diagram, D1, ten rows, covering the 1A 00 record only" (matrix §0); the
CSV as landed carries nine data rows. No sub-diagram (D2/D3) is
transcribed, because the S1 leg found none printed on this page - the
byte (3) select-memory cell is read as one undivided `enum_byte`, with
the note "No nibble split is printed for this byte (unlike the IC-7610
record...)"; byte (11) is read as one `enum_nibble` cell whose two value
lists sit in the SAME row, not a separate enlarged diagram.

See `ic7600-transcription-b.csv` for the full per-row transcription.
