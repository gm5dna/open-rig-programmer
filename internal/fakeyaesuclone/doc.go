// SPDX-License-Identifier: GPL-3.0-or-later

// Package fakeyaesuclone simulates the RADIO side of a clone-mode transfer
// for all three families core/clonewire models: FT-817/FT-817ND/FT-818,
// FT-857/FT-857D and FT-897/FT-897D (Phase 3, milestone
// 2026-09-25-clone-mode-read). One package, not one per family, because all
// three share CHIRP's own yaesu_clone.YaesuCloneModeRadio block/ACK
// convention — the constructor selects the Profile and this package emits
// whatever schedule that Profile names.
//
// # No capture exists
//
// Per spec.md §Fakes and byte-identity, no owner capture exists for any of
// these eight radios. This fake is its own oracle for drift-detection only —
// proof that this project's own code round-trips a well-formed image of the
// ASSUMED shape, never evidence that shape matches a real radio.
//
// # Quarantine
//
// This package uses core/clonewire's PUBLIC types only (Profile, Block,
// Image, Arm, Receive, the error sentinels) to drive the wire and to check
// its own output — never clonewire's unexported helpers (yaesuChecksum,
// yaesuBlocks, ...). Where a fact from clonewire's own Profile values would
// otherwise have to be re-derived (the block checksum algorithm), this
// package computes it independently, from the same CHIRP source clonewire
// cites, rather than calling clonewire's private implementation. That is the
// internal/fakedx10 rationale, restated: two independent implementations of
// one protocol, checked against each other, are what makes a systematic bug
// in one visible instead of invisible. CHIRP pin:
// chirp/drivers/{ft817,ft818,ft857}.py + yaesu_clone.py, commit
// e7347e6a66ef8f9edb50e3e534510c2d3ae6b329 (same commit core/clonewire/doc.go
// cites).
//
// # RADIO role, not PC role
//
// spec.md's own Q1 text and CHIRP's driver: the PC (here, core/clonewire's
// Receive) sends the 0x06 ACK back to the radio after every received block —
// the radio never sends it. This package plays the RADIO role: for each
// Block in the selected Profile's BlockSchedule, in order, it writes that
// block's wire bytes, then — when the Profile expects an ACK — waits for
// exactly one 0x06 from its peer before writing the next block. Any other
// byte, or no byte within the wait window, fails the transfer (Radio.Err()),
// mirroring a real radio giving up on a confused host.
//
// # Fabricated image content
//
// BuildImage synthesises an ImageLen-byte image from a Profile's own
// geometry fields (RecordOffset/RecordWidth/ChannelCount/FreqOffset) with no
// capture to copy: a handful of populated channels (real-looking frequency,
// mode and name, informed-by CHIRP's own field layout), CHIRP's own
// "unused" convention for the rest of the channel bank (an all-0xFF erased
// record — DecodeRecord already treats 0xFFFFFFFF frequency as Empty), and a
// fixed NON-zero fill byte (unmappedFill, 0x5A) for every byte outside the
// channel bank — a zero-filled unmapped region would misread as "confirmed
// empty", which it is not (spec.md §Fakes and byte-identity).
package fakeyaesuclone
