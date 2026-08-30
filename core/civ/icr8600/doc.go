// SPDX-License-Identifier: GPL-3.0-or-later

// Package icr8600 holds the Icom IC-R8600's CI-V profile.
//
// # Frozen evidence
//
// The files in testdata are quarantined readings of the source material: the
// field ledger (L), geometry witness (W), semantic transcription (B) and golden
// vectors (G), together with their provenance and assumption records. They are
// frozen evidence, not generated fixtures. Tests and implementation must read
// them as supplied; they must not regenerate, normalise or rewrite them.
//
// testdata/SHA256SUMS records the evidence hashes at the quarantine hand-off.
//
// # Stage 1 assumption register
//
// These entries keep every value which is not settled by the printed guide
// attached to one lift. Green tests mean that the codec agrees with the
// frozen manual-derived evidence and these choices; they do not mean that a
// real receiver has confirmed them. TestCrosscheckMatrixBAndLedgerJoinByDiagramAndField
// SHA-pins the authoritative icr8600-capability-matrix.md,
// icr8600-capability-matrix-report.md and icr8600-capability-matrix-review.md
// documents before joining their extracted A rows to B and L.
//
//   - icr8600-mode-wire-codes: the printed two-character mode codes are
//     emitted as BCD-looking bytes. TestModeLayoutsSelectSevenTailsAndSixRecordLengths
//     pins them. Stage R reads FM, DCR and D-STAR channels and records ⑪.
//   - icr8600-filter-byte-digital: FIL1 (01) is used for digital records
//     because the guide leaves ⑫ drawn but prints a dash instead of a value.
//     TestGoldenVectorsReplay pins G's choice. Stage R reads ⑫ from D-STAR
//     and P25 channels.
//   - icr8600-read-request-form: a read carries the four address bytes and no
//     record. TestGoldenVectorsReplay pins it. Stage R captures one occupied
//     memory read request and answer.
//   - icr8600-empty-reply-fa and icr8600-empty-reply-ff: neither possible
//     empty response is interpreted by this profile. TestGoldenAllFFRecordsRemainRawBeforeLayoutSelection
//     pins the raw-before-layout rule; Stage R reads known-empty channels and
//     records whether FA or an FF record arrives.
//   - icr8600-record-lengths and icr8600-answer-length: the derived record
//     set is 37/39/41/43/44/45 and answers are required to carry the full
//     selected layout. TestModeLayoutsSelectSevenTailsAndSixRecordLengths
//     and TestGeometryWitnessPinsEveryTailOffsetAndLength pin the choice.
//     Stage R reads one channel of every class, including FM and DCR.
//   - icr8600-name-charset-codes and icr8600-name-pad: printed glyphs are
//     mapped to ASCII and padded with space. TestNamePolicyPins and
//     TestNameEncodingPadsAndNeverTruncates pin both choices. Stage R writes
//     representative glyphs and reads a short front-panel name.
//   - icr8600-budget: the populated-channel limit is unstated, hence
//     BudgetUnstated. TestAddressSparseMemoryBankUsesZeroBasedCanonicalSlots
//     pins that conservative description. Stage R fills ordinary memories
//     until a further write is refused.
//   - icr8600-scan-edge-encoding: group 0102 stays outside the address union.
//     TestAddressSpaceIsOnlyTheNormalMemoryRectangle pins the refusal. Stage R
//     captures one programmable scan edge.
//   - icr8600-tuning-step-enabled, icr8600-tuning-step,
//     icr8600-program-tuning-step, icr8600-attenuator, icr8600-preamp,
//     icr8600-antenna and icr8600-ip-plus: the D8 neutral fields use the
//     record-page wire tables and positions. TestRecordCommonHeadEncodesAndDecodesEveryMappedField
//     pins all seven together; Stage R changes each setting independently
//     and reads ⑱–㉕.
//   - icr8600-progstep-floor and icr8600-ts-off-code: the guide does not
//     state the smallest programmable step or what ⑲ must hold when ⑱ is
//     OFF, so Stage 1 imposes neither extra constraint. The common-head test
//     pins only the documented wire encoding. Stage R reads the smallest
//     front-panel step and a TS-OFF channel.
//   - icr8600-tsql-chart-bounds and icr8600-dtcs-chart: the guide prints the
//     field domains, not the receiver's selectable charts. TestGoldenVectorsReplay
//     pins only G's 88.5 Hz and 023 examples. Stage R records the lowest and
//     highest TSQL choices and walks the DTCS list.
//   - icr8600-tuning-range: Stage 1 adds no range validator because the
//     receiver's actual tuning bounds are not printed. Stage R probes the
//     documented band edges before Stage 2 advertises a capability range.
//   - icr8600-short-set: although the guide permits omitted tails on writes,
//     their defaults are unstated, so the codec always emits a full selected
//     layout. TestCrosscheckArbitrations/spec_full_layout_wins_over_documented_short_set
//     pins why. Stage W writes a head-only FM record and reads it back.
//   - icr8600-tail-templates (called icr8600-digital-tail-template by the
//     implementation plan): G's five digital tail values are assumed because
//     the guide prints domains but no defaults. TestTailTemplatesAndFMToneFieldsEncodeEveryDeclaredClass
//     pins them. Stage R reads a factory-fresh channel of every digital class.
package icr8600
