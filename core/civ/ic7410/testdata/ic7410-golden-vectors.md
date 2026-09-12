# IC-7410 golden vectors

Authored by this package's own implementer directly from
`docs/superpowers/icom-matrices/ic7410-capability-matrix.md` §1b's offset
table and the IC-7410 Instruction Manual PDF p.115. This is a SINGLE-SOURCE
authoring, not an independently blind-transcribed evidence leg: `golden_test.go`
cross-checks it by assembling the same 40-byte record a second, independent
way (by offset, from this table) and comparing against `civ.Profile`'s own
encoder — agreement between the two is the check this file supports, not
proof on its own.

No IC-7410 has ever been asked anything by this project. Every value below
is a construction inside this radio's printed domain, not a captured frame.

## Vector 1 — MEM channel 1 (`0001`), a genuine split channel

| Field | Value |
|---|---|
| rx_frequency | 14,250,000 Hz (14.250000 MHz, within 30 kHz-60 MHz coverage) |
| mode | USB (`0x01`) |
| filter | FIL1 (`0x01`) |
| data_mode | ON (`0x01`) |
| tone_mode | TONE (`0x1`, high nibble of record byte 9) |
| tone_tx | 885 (88.5 Hz, tenths of a Hz) |
| tone_rx | 1000 (100.0 Hz) |
| tx_frequency | 14,750,000 Hz — deliberately DIFFERENT from rx_frequency, proving the TX-duplicate block's frequency span is independently encoded, not a copy the codec makes for free |
| name | `"TEST 1"` (6 of 9 bytes, padded with `0x20`) |

Record bytes 0 (select/split) and 21-30 (the duplicate block's remaining
ten bytes) are UNMAPPED and must read `0x00` (matrix §1b; spec.md's ruling 7).

## Vector 2 — SCAN edge P1 (channel `0100`), an empty-shaped edge

| Field | Value |
|---|---|
| rx_frequency | 7,000,000 Hz |
| mode | LSB (`0x00`) |
| filter | FIL2 (`0x02`) |
| data_mode | OFF (`0x00`) |
| tone_mode | OFF (`0x0`) |
| tone_tx | 1000 (100.0 Hz) |
| tone_rx | 1000 (100.0 Hz) |
| tx_frequency | 7,000,000 Hz — equal to rx_frequency: matrix §1b/§2 SCAN row 11 records P1/P2 write as `Uns`, so this vector exercises the READ-only shape; a driver never builds this span on this bank |
| name | `""` (nine pad bytes) |

Both vectors' record widths are 40 bytes: 1 (unmapped) + 5 (rx_frequency)
+ 1 (mode) + 1 (filter) + 1 (data_mode) + 1 (tone_mode) + 3 (tone_tx) +
3 (tone_rx) + 5 (tx_frequency) + 10 (unmapped) + 9 (name) = 40.
