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

	// HeaderBytes and TrailerBytes are bytes at the start/end of this
	// block's validated chunk that are framing, not image content, and so
	// are stripped before the remainder is appended to Image.Raw. Both
	// default to 0 (the whole chunk is content — Phase 1's own fixture
	// profiles never set these, and their behaviour is unchanged).
	//
	// Phase 2 needs this: every real family in this package (informed-by
	// CHIRP's yaesu_clone.py/ft817.py) frames each wire block as
	// [1-byte block number][payload][1-byte checksum], but Checksum (above)
	// must see that whole framed chunk to validate it, while Image.Raw must
	// hold only the concatenated PAYLOAD bytes (what CHIRP's own mmap
	// contains, and what a Profile's ImageLen/RecordOffset/RecordWidth are
	// stated against). This is the "genuinely divergent wire schedule"
	// case doc.go's candidate-profile note anticipates: extended inside
	// Receive, with no change to Arm/Receive/ParseImage's signatures.
	HeaderBytes  int
	TrailerBytes int
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
	// RecordOffset is where the ChannelCount*RecordWidth channel-record
	// bank begins within the complete image (Image.Raw) — zero for Phase
	// 1's own fixtures (records start at byte 0), and a real, non-zero,
	// CHIRP-informed offset for every family Phase 2 adds (the channel
	// bank sits after several header/settings blocks, never at offset 0).
	RecordOffset int
	// FreqOffset locates the 4-byte big-endian frequency field within one
	// RecordWidth-byte record (informed-by CHIRP's mem_struct field
	// order). Zero means "this Profile carries no per-channel decode" —
	// no real Profile in this family ever has FreqOffset 0, since the
	// mode byte always occupies offset 0 instead (see DecodeRecord).
	FreqOffset int

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

// The FT-817/FT-817ND/FT-818, FT-857/FT-857D and FT-897/FT-897D families
// (Phase 2a/2b/2c) all inherit ONE wire convention from CHIRP's
// yaesu_clone.YaesuCloneModeRadio base class: every block is framed
// [1-byte block number][payload][1-byte checksum] and ACKed once its
// checksum passes — informed-by chirp/drivers/ft817.py (_read, _clone_in)
// and chirp/drivers/yaesu_clone.py (YaesuChecksum), pinned at commit
// e7347e6a66ef8f9edb50e3e534510c2d3ae6b329 (see doc.go). ASSUMED workable
// at PC UART timing; no HW capture exists for any of these eight radios
// (spec.md §Frame grammar).
const (
	// yaesuStartDeadline is informed-by ft817.py _read: the FIRST block
	// (blocknum 0) is retried up to 60 times at a 0.5s sleep — CHIRP's own
	// "be very patient" comment. ASSUMED, no HW capture.
	yaesuStartDeadline = 30 * time.Second
	// yaesuInterBlockDeadline is informed-by the same function: every
	// later block retries up to 5 times at 0.5s (2.5s) — doubled here for
	// PC UART margin over CHIRP's own radio-tuned retry loop. ASSUMED, no
	// HW capture.
	yaesuInterBlockDeadline = 5 * time.Second
	// yaesuTotalDeadline is a generous, ASSUMED bound for a ~6-7.5KB
	// image at 9600 baud plus per-block ACK round trips (up to 50 blocks
	// for this family) — not itself a CHIRP-cited figure, no HW capture.
	yaesuTotalDeadline = 120 * time.Second
)

// yaesuChecksum validates one wire block shared by every Profile this
// package models (informed-by CHIRP's YaesuChecksum: an 8-bit sum, mod
// 256, of the payload bytes only, stored as the block's trailing byte). It
// does not check the leading block-number byte's value — Checksum is
// called with only the raw chunk, never a block index — a wrong block
// number is still caught indirectly: it shifts every following block's
// framing, so the transfer ends either short (ErrImageIncomplete) or with
// trailing bytes (also ErrImageIncomplete), never a silently-accepted
// wrong image.
func yaesuChecksum(chunk []byte) bool {
	if len(chunk) < 2 {
		return false
	}
	payload := chunk[1 : len(chunk)-1]
	var sum byte
	for _, b := range payload {
		sum += b
	}
	return sum == chunk[len(chunk)-1]
}

// yaesuBlocks expands one payload length, repeated n times, into n
// wire-framed Blocks (see yaesuChecksum/HeaderBytes/TrailerBytes above).
// CHIRP's own _clone_in ACKs every block, including every repeat of a
// repeated block, never only the last — Ack is true on every element.
func yaesuBlocks(payloadLen, n int) []Block {
	blocks := make([]Block, n)
	for i := range blocks {
		blocks[i] = Block{
			Len:          payloadLen + 2,
			HeaderBytes:  1,
			TrailerBytes: 1,
			Ack:          true,
			Checksum:     yaesuChecksum,
		}
	}
	return blocks
}

// yaesuSchedule builds a full BlockSchedule from CHIRP's own block-length
// list shape: a handful of fixed leading blocks, one payload length
// repeated (CHIRP's "block N is to be repeated forty times" comment — the
// channel-record bank), then a handful of fixed trailing blocks.
func yaesuSchedule(leading []int, repeatPayload, repeatCount int, trailing []int) []Block {
	var sched []Block
	for _, l := range leading {
		sched = append(sched, yaesuBlocks(l, 1)...)
	}
	sched = append(sched, yaesuBlocks(repeatPayload, repeatCount)...)
	for _, l := range trailing {
		sched = append(sched, yaesuBlocks(l, 1)...)
	}
	return sched
}

// yaesuSum sums a slice of ints — a small helper for stating a Profile's
// RecordOffset as "sum of the leading blocks" next to the schedule that
// produces it, rather than a bare repeated literal.
func yaesuSum(ns []int) int {
	total := 0
	for _, n := range ns {
		total += n
	}
	return total
}

// FT-817/FT-817ND/FT-818 (Phase 2a). Every image length, block schedule and
// record geometry below is informed-by CHIRP's chirp/drivers/ft817.py and
// ft818.py, pinned at commit e7347e6a66ef8f9edb50e3e534510c2d3ae6b329 (see
// doc.go) — ASSUMED, no HW capture; no manual prints this family's byte
// layout (spec.md §Frame grammar). Baud/DataBits/Parity/StopBits are
// informed-by the same source (BAUD_RATE = 9600, CHIRP's 8N1 convention
// for this whole family).
//
// spec.md §Identity probe: CHIRP shows FIVE distinct image lengths for
// this family, never one shared shape — none of the five collide, so no
// byte-level distinguishing check beyond ImageLen is needed here (matchAndParse's
// existing length-only match already decides identity correctly for every
// pair in FT817Family).
var (
	ft817Leading = []int{2, 40, 208, 182, 208, 182, 198, 53}
	ft818Leading = []int{2, 40, 208, 208, 208, 208, 198, 53}

	// FT817 is the base FT-817 (6509 bytes, non-US).
	FT817 = Profile{
		Model:              "FT-817",
		ProfileID:          "ft817",
		Baud:               9600,
		DataBits:           8,
		Parity:             "N",
		StopBits:           1,
		ImageLen:           6509,
		RecordWidth:        26,
		ChannelCount:       200,
		RecordOffset:       yaesuSum(ft817Leading),
		FreqOffset:         10,
		BlockSchedule:      yaesuSchedule(ft817Leading, 130, 40, []int{118, 118}),
		AckExpected:        true,
		StartDeadline:      yaesuStartDeadline,
		InterBlockDeadline: yaesuInterBlockDeadline,
		TotalDeadline:      yaesuTotalDeadline,
	}

	// FT817ND is the FT-817ND (6521 bytes, non-US).
	FT817ND = Profile{
		Model:              "FT-817ND",
		ProfileID:          "ft817nd",
		Baud:               9600,
		DataBits:           8,
		Parity:             "N",
		StopBits:           1,
		ImageLen:           6521,
		RecordWidth:        26,
		ChannelCount:       200,
		RecordOffset:       yaesuSum(ft817Leading),
		FreqOffset:         10,
		BlockSchedule:      yaesuSchedule(ft817Leading, 130, 40, []int{118, 130}),
		AckExpected:        true,
		StartDeadline:      yaesuStartDeadline,
		InterBlockDeadline: yaesuInterBlockDeadline,
		TotalDeadline:      yaesuTotalDeadline,
	}

	// FT817NDUS is the FT-817ND (US) — CHIRP's own comment: "radios
	// configured for 5MHz operations send one packet more than others".
	FT817NDUS = Profile{
		Model:              "FT-817ND (US)",
		ProfileID:          "ft817nd-us",
		Baud:               9600,
		DataBits:           8,
		Parity:             "N",
		StopBits:           1,
		ImageLen:           6651,
		RecordWidth:        26,
		ChannelCount:       200,
		RecordOffset:       yaesuSum(ft817Leading),
		FreqOffset:         10,
		BlockSchedule:      yaesuSchedule(ft817Leading, 130, 40, []int{118, 130, 130}),
		AckExpected:        true,
		StartDeadline:      yaesuStartDeadline,
		InterBlockDeadline: yaesuInterBlockDeadline,
		TotalDeadline:      yaesuTotalDeadline,
	}

	// FT818 is the base FT-818 (6573 bytes, non-US).
	FT818 = Profile{
		Model:              "FT-818",
		ProfileID:          "ft818",
		Baud:               9600,
		DataBits:           8,
		Parity:             "N",
		StopBits:           1,
		ImageLen:           6573,
		RecordWidth:        26,
		ChannelCount:       200,
		RecordOffset:       yaesuSum(ft818Leading),
		FreqOffset:         10,
		BlockSchedule:      yaesuSchedule(ft818Leading, 130, 40, []int{118, 130}),
		AckExpected:        true,
		StartDeadline:      yaesuStartDeadline,
		InterBlockDeadline: yaesuInterBlockDeadline,
		TotalDeadline:      yaesuTotalDeadline,
	}

	// FT818NDUS is the FT-818ND (US) (6703 bytes).
	FT818NDUS = Profile{
		Model:              "FT-818ND (US)",
		ProfileID:          "ft818nd-us",
		Baud:               9600,
		DataBits:           8,
		Parity:             "N",
		StopBits:           1,
		ImageLen:           6703,
		RecordWidth:        26,
		ChannelCount:       200,
		RecordOffset:       yaesuSum(ft818Leading),
		FreqOffset:         10,
		BlockSchedule:      yaesuSchedule(ft818Leading, 130, 40, []int{118, 130, 130}),
		AckExpected:        true,
		StartDeadline:      yaesuStartDeadline,
		InterBlockDeadline: yaesuInterBlockDeadline,
		TotalDeadline:      yaesuTotalDeadline,
	}
)

// FT817Family lists every distinct FT-817/FT-817ND/FT-818 image shape this
// package models — spec.md §Identity probe: CHIRP's own driver files show
// these as five DISTINCT lengths, never collapsed into one Profile.
var FT817Family = []Profile{FT817, FT817ND, FT817NDUS, FT818, FT818NDUS}
