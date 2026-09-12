# IC-7600 CI-V — memory-record data-block geometry witness

## Source

`docs/fixtures-private/manuals/ic7600_fullmanual_4.pdf` (gitignored),
196 PDF pages. Printed folio = PDF page − 9. Record diagram: PDF p.178
(folio 169).

## Provenance, and how this differs from the IC-7610 exemplar

The IC-7610 package's own geometry witness (W leg) was measured directly
off 400-500 dpi raster renders by a quarantined agent with no repository
access. This file does not repeat that independent measurement pass: it
restates `docs/superpowers/icom-matrices/ic7600-capability-matrix.md`
rev 1's own §3.11 arithmetic — itself checked against a direct 300 dpi
render of PDF p.178 — as a cumulative byte-position table
(`ic7600-geometry-witness.csv`), so `geometry_test.go` can assemble a
record from these positions and compare it byte-for-byte against
`BuildMemorySet`'s own output.

## The tiling

Nine D1 rows tile data-area bytes 1-27 with no gap and no overlap,
exactly matching the matrix's own seven-term record sum (25 bytes) plus
the 2-byte address:

```
  1  (1,2)    address, excluded from RecordOnlyLength
+ 1  (3)      select memory setting - UNMAPPED whole byte (E6)
+ 5  (4~8)    operating frequency setting
+ 1  (9)      operating mode setting
+ 1  (10)     filter setting
+ 1  (11)     data mode / tone type - UNMAPPED high nibble, mapped low nibble (E6)
+ 3  (12~14)  repeater tone frequency setting
+ 3  (15~17)  tone squelch frequency setting
+ 10 (18~27)  memory name setting
-----
= 27 data-area bytes; 25 record bytes excluding the address.
```

No enlarged D2/D3 sub-diagram exists on this radio's own page (matrix
§3.15(a)): byte (3) is drawn as one undivided enum cell, so this witness
carries no separate nibble-level sub-diagram row for it, unlike the
IC-7610's own geometry witness.
