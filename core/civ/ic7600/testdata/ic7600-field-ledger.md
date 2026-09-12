# IC-7600 CI-V — memory-record data-block field ledger

## Source

Document title (PDF metadata): "IC-7600 Instruction Manual", Subject
"HF/50 MHz Transceiver". No separate CI-V Reference Guide is published
for this model; command tables and data-content descriptions are Section
12, "CONTROL COMMAND". File:
`docs/fixtures-private/manuals/ic7600_fullmanual_4.pdf` (gitignored),
196 PDF pages. Printed folio = PDF page − 9.

## Provenance, and how this differs from the IC-7610 exemplar

The IC-7610 package's own field ledger is one of four independently
QUARANTINED legs, each authored by an agent that never opened this
repository. This ledger is not that: it restates the nine field rows of
`docs/superpowers/icom-matrices/ic7600-capability-matrix.md` rev 1's
own §2/§3.11, itself self-reviewed against a direct 300 dpi render of
PDF pp.176-178, in the ledger CSV's tabular shape
(`ic7600-field-ledger.csv`) so `crosscheck_test.go` can bind it to
`ic7600-transcription-b.csv` (the Tier 7 S1 evidence leg's own,
independently-produced transcription) and to this package's profile.

## The nine rows

All nine rows are diagram D1 - the single "Memory content setting"
record diagram, `Command: 1A 00`, PDF p.178 (folio 169). Unlike the
IC-7610's own page, this radio's page carries NO enlarged sub-diagram
for byte (3) or byte (11) (the S1 evidence leg's own transcription found
one diagram only, ten field rows covering the record); §3.15(a) of the
matrix records this as a genuine divergence from the IC-7610's own page,
not an oversight.

| field_index | label | note |
|---|---|---|
| 1,2 | Memory channel number | the ADDRESS, outside the record (spec Erratum 1) |
| 3 | Select memory setting | whole-byte enum, UNMAPPED under E6 (matrix S2 row 9) |
| 4~8 | Operating frequency setting | 5-byte little-endian packed BCD |
| 9 | Operating mode setting | shares a printed legend with (10) |
| 10 | Filter setting | shares a printed legend with (9) |
| 11 | Data mode and tone type settings | tone_mode mapped (low nibble), data_mode UNMAPPED (high nibble) |
| 12~14 | Repeater tone frequency setting | 3-byte big-endian packed BCD |
| 15~17 | Tone squelch frequency setting | 3-byte big-endian packed BCD |
| 18~27 | Memory name setting | 10-byte ASCII, TagCharset contradiction recorded at matrix S1/S3.9(ii) |

See `ic7600-field-ledger.csv` for the per-row citation and the matrix
section each row's grading rests on.
