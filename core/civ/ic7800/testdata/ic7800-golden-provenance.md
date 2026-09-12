# IC-7800 golden vector provenance

Single-source evidence (matrix §0): the IC-7800 Instruction Manual Section
14 (PDF pp.200-217), read once by the matrix author, not the IC-7610
lineage's four independently-blind legs. This note itemises the one vector
whose fields are not self-evident from the frame bytes alone.

## `set-record-name-with-space` (34 bytes)

Channel 1 (`00 01`), chosen as the lower bound of the printed range
`0001-0099` (PDF p.211).

| Field | Value | Record bytes | Source |
|---|---|---|---|
| Select (E6-unmapped) | OFF (0) | 0 | PDF p.211, byte `e`: `00: OFF` |
| RX frequency | 14 250 000 Hz | 1-5 | Chosen inside the printed 30 kHz-60 MHz coverage (PDF p.214), little-endian packed BCD per PDF p.208's digit strip |
| Mode | USB (0x01) | 6 | PDF p.208, `q Operating mode`: `01: USB` |
| Filter | FIL1 (0x01) | 7 | PDF p.208, `w Filter setting`: `01: FIL1` |
| Tone mode (hi nibble) | TONE (0x1) | 8 hi | PDF p.211, `!1`: `1: TONE` |
| Data mode (lo nibble, E6-unmapped) | OFF (0x0) | 8 lo | PDF p.211, `!1`: `0: OFF` |
| Tone TX | 88.5 Hz (885 deci-Hz) | 9-11 | PDF p.209, big-endian packed BCD, `00 08 85` |
| Tone RX | 100.0 Hz (1000 deci-Hz) | 12-14 | PDF p.209, big-endian packed BCD, `00 10 00` |
| Name | `HOME QTH01` | 15-24 | PDF p.209 character tables (matrix §1 row 25), ASCII, space mid-name to exercise the ASSUMED pad/space code |

`read-record` (channel 1) and `read-transceiver-id` are the two
zero-data request forms; `manual-example-14` is a padded power-ON (`18 01`)
frame documenting the family-wide preamble-padding convention this tier
never sends — included only to prove the gate refuses it, not as a claim
about a specific printed worked example in this model's own manual.

Hardware status: UNVERIFIED. No IC-7800 has ever answered a frame; every
byte above is a reading of the manual and the matrix, nothing else.
