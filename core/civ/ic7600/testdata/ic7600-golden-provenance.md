# IC-7600 golden wire frames — provenance

## Source

`docs/fixtures-private/manuals/ic7600_fullmanual_4.pdf` (gitignored),
title "IC-7600 Instruction Manual", 196 pages, SHA-256
`5ff4418a1a39d567f29097acc55afc86843e244403d032096c014955a06891b7`
(matches `docs/fixtures-private/manuals/ic7600-manual-provenance.md`).
Printed-folio offset: PDF page − 9.

## How these vectors were built

Unlike the IC-7610 exemplar's four-leg blind quarantine, these four
vectors were hand-assembled from the capability matrix
(`docs/superpowers/icom-matrices/ic7600-capability-matrix.md` rev 1),
which is itself self-reviewed against a direct 300 dpi render of PDF
pp.176-178 (§0 of that matrix) and cross-checked against the Tier 7 S1
evidence leg's own transcription. Every byte's provenance is itemised in
`ic7600-golden-assumptions.csv`.

`read-record`, `set-record-name-with-space` and `read-transceiver-id` are
built directly from the matrix's §2/§3.7/§3.10/§3.12 findings, with the
same neutral field values the IC-7610 exemplar's own golden vector uses
(14.250000 MHz, USB, FIL1, TONE, 88.5 Hz repeater tone, 100.0 Hz tone
squelch, name "HOME QTH01") - these are synthetic test values, not a
manual quotation, chosen to exercise every mapped field including a
name containing a space (D5 entry 3).

`manual-example-14` (the multi-preamble power-ON frame, command `18 01`)
is carried across from the IC-7610 exemplar's own vector of the same
name, on the general Icom CI-V family convention that a sleeping radio
needs several leading `FE` preamble bytes to wake; it was not
independently re-confirmed against this radio's own document for this
task (recorded in `ic7600-golden-assumptions.csv` as
`inherited_assumed`). Its only job is to prove `AllowedCommand` refuses a
command this tier never sends.

## Hardware status

UNVERIFIED, for all four vectors. Not one has been sent to, or captured
from, a real IC-7600 (matrix §0 "Hardware status", §3.14). Green here
means the codec agrees with the matrix's own reading of the manual, not
that any radio accepts these bytes.
