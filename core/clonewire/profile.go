// SPDX-License-Identifier: GPL-3.0-or-later

package clonewire

import "time"

// Block describes one fixed-size chunk of a clone-mode transfer, in wire
// order. A family's whole image is the concatenation of every Block in its
// Profile's BlockSchedule, in order — never a single undifferentiated byte
// count, since the ACK point and checksum rule (where either exists) are
// per-block, informed-by each family's own CHIRP block-read loop (Phase 2).
type Block struct {
	// Len is this block's exact byte length.
	Len int
	// Ack reports whether Receive sends the ACK byte (0x06) once this
	// block is fully read and its Checksum (if any) has passed. Ignored
	// unless the owning Profile's AckExpected is also true — AckExpected
	// is the master switch (spec.md Decisions item 5: every outbound
	// byte but this one stays refused), Ack names WHERE, per block.
	Ack bool
	// Checksum validates this block's raw bytes once fully read, if set.
	// nil means no check. The wire checksum algorithm is real-radio
	// specific and is Phase 2's job (informed-by CHIRP); this field
	// stays pluggable so this package's own tests can exercise
	// checksum-failure refusal without inventing one.
	Checksum func(block []byte) bool
}

// Profile describes one candidate clone-mode image shape. Multiple
// Profile values can describe the same physical radio family (spec.md
// §Identity probe: CHIRP's own FT-817/FT-817ND/FT-818 drivers already
// record several distinct image lengths for what looks like one shared
// shape) — Receive is always given the whole candidate set, never a single
// Profile (spec.md §Identity probe; Codex blocker 3).
//
// Every field left at its zero value here is Phase 2's job to fill in,
// per family, each provenance-labelled against CHIRP or a manual page.
// This phase's own tests use a synthetic fixture Profile only.
type Profile struct {
	// Model is the radio's display name, e.g. "FT-817ND".
	Model string
	// ProfileID is this Profile's stable per-shape identifier — the same
	// string codeplug.RawImageBlob.ProfileID records, since Model alone
	// cannot distinguish this family's several image lengths.
	ProfileID string

	// Baud, DataBits, Parity and StopBits are this Profile's serial
	// parameters. Parity is one of "N", "E" or "O".
	Baud     int
	DataBits int
	Parity   string
	StopBits int

	// ImageLen is this Profile's exact total image length in bytes — the
	// value Receive matches a fully-received image's length against
	// (spec.md §Identity probe). It is a compatibility check, not an
	// identity proof.
	ImageLen int
	// RecordWidth is the byte width of one channel record; ChannelCount
	// is how many consecutive records the image carries. Both are zero
	// until a real family sets them (Phase 2); ParseImage treats zero
	// ChannelCount as "no records to slice", still returning the whole
	// raw image.
	RecordWidth  int
	ChannelCount int

	// BlockSchedule is the ordered sequence of Blocks this Profile's
	// image is transferred in. Receive drives the live wire read from
	// candidates[0]'s BlockSchedule (see doc.go) — every candidate
	// offered to one Arm/Receive call is assumed to share it.
	BlockSchedule []Block
	// AckExpected is the master ACK switch: when false, Receive sends no
	// byte at all, however a Block's own Ack field is set (spec.md
	// Decisions item 5 — refusal of every other outbound byte).
	AckExpected bool

	// StartDeadline bounds how long Receive waits, from the moment Arm
	// returned, for the very first byte. InterBlockDeadline bounds the
	// gap between bytes once the transfer has started, reset at every
	// block boundary. TotalDeadline bounds the whole transfer, from Arm.
	// Any one of the three expiring refuses the whole image
	// (spec.md §Read model, point 4-5).
	StartDeadline      time.Duration
	InterBlockDeadline time.Duration
	TotalDeadline      time.Duration
}
