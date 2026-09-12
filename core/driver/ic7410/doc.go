// SPDX-License-Identifier: GPL-3.0-or-later

// Package ic7410 is the IC-7410 driver: probing, capabilities and write
// policy over core/civ/ic7410's independently evidenced CI-V dialect.
//
// # Evidence
//
// docs/superpowers/icom-matrices/ic7410-capability-matrix.md (gitignored),
// written from the IC-7410 Instruction Manual alone (no standalone CI-V
// Reference Guide exists for this model). No IC-7410 has ever been asked
// anything by this project: every capability value here cites a matrix
// row, and writeTrialsComplete (caps.go) stays false until one has.
//
// # The headline deviation from the IC-7610 family this wave otherwise
// shares
//
// spec.md's ruling 7 (the orchestrator's matrix reconciliation) rules that this
// model's 40-byte record — not the tier spec's originally assumed 25 —
// carries a genuine 15-byte TX-duplicate block. Its first five bytes are
// an independent transmit frequency and are mapped to spec.FieldTxFrequency
// with a SimplexTx declaration (caps.go); its remaining ten bytes carry no
// neutral field and are left deliberately zero, UNMAPPED, exactly like
// this model's byte 0 (Select-memory + Split, matrix §1b). Two more
// per-field divergences worth stating up front: data_mode occupies a
// WHOLE byte on this model, unlike the IC-7610 family's nibble-shared
// four-valued version, so it is a genuinely MAPPED, writable field here;
// and tone_mode sits on the HIGH nibble of its byte rather than the LOW
// one.
//
// # Write policy: mirroring, not refusal
//
// core/driver/ic7300's own driver REFUSES a write whose TxFreqHz is not
// Known ("REV 1's substitution is STRUCK and must not come back" —
// core/driver/ic7300/write.go), because on that model the manual states
// nothing about what a simplex channel's transmit frequency should be.
// This model's manual DOES say something: the duplicate block is "still
// necessary" even when Split is OFF. spec.md's ruling 7 reads that as licence to
// MIRROR the receive frequency into the transmit span whenever a caller
// has not set one — write.go implements exactly that, and it is a
// deliberate, named deviation from the IC-7300 convention rather than an
// oversight.
package ic7410
