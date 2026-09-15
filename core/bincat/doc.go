// SPDX-License-Identifier: GPL-3.0-or-later

// Package bincat is the codec for a fourth radio-control generation this
// project registers: the 1990s Yaesu binary-CAT family (FT-920, FT-890,
// FT-900, FT-1000MP/Mark-V), fixed 5-byte command frames — four
// argument/dummy bytes then the opcode, always sent left to right, opcode
// last — 4800 bit/s, point-to-point RS-232, with no bus/device-address
// byte and, critically, no acknowledgement and no NAK for a write (the
// FT-1000MP manual: an out-of-range or illegal parameter makes the radio
// "do nothing", not answer).
//
// It is a SIBLING of core/cat (printable ASCII, ';'-terminated) and
// core/civ (binary, but addressed and terminator-framed — FE FE … FD).
// Neither is imported for its FRAMING: this family has no preamble, no
// terminator and no address, so nothing in core/cat's or core/civ's frame
// grammar applies. What IS reused, deliberately and narrowly, is
// civ.DecodeBCD2 for single-byte packed-BCD fields (bcd.go) — a genuinely
// protocol-neutral two-digit decode, not a framing borrowing — and
// core/transport, whose Framing seam (D2) this package implements exactly
// as core/civ's framing.go implements it for CI-V.
//
// # The transport seam
//
//	Framing method     this package's piece
//	-----------------  -------------------------------------------
//	NewAccumulator      an accumulator built in NewFraming (closes the
//	                    reader-goroutine init race — see framing.go)
//	IsRejection         always false — this family answers a rejected
//	                    write with SILENCE, not a NAK frame
//	Allow               Profile.AllowedCommand
//	InitSequence        EMPTY — nothing this tier sends mutates a setting
//	                    to open a session
//	DrainPolicy         DrainIdleGap / DrainCap
//	NoteSent            records the EXPECTED REPLY LENGTH of the frame
//	                    about to be sent (Profile.ReplyLength), not an
//	                    echo to suppress — see framing.go's doc comment
//	                    for why this family needs that and CI-V does not
//
// # No FT-920, no Recall, no channel-numbering-offset field
//
// This package ships no FT-920 Profile value: its memory-record
// frequency-encoding is undocumented at the source, and guessing one is a
// data-corruption risk (milestone spec, Decision 1). FT-920's own p.88
// worked SetFreq example survives only as bcd_test.go's round-trip
// fixture, byte-identical to FT-1000MP's own (spec.md §Frame grammar).
// There is no Recall opcode constant: every driver this milestone builds
// reads via Status Update directly and declines MemorySelector, so Recall
// is unused surface a package should not ship. There is no FT-1000MP
// channel-numbering-offset field on Profile while that offset stays OPEN
// (the manual self-contradicts) — an unset field nothing consumes is not a
// safety measure, and the OPEN status belongs in the capability matrix,
// not in a struct shipped this milestone.
//
// # Values live in the driver packages, not here
//
// Profile is ONE Go type; the concrete FT-890 and FT-900 values (they
// differ genuinely — slot count and full-dump length, 32/649 vs 100/1941 —
// so one value cannot describe both) are built and owned by
// core/driver/ft890900, mirroring how core/civ's Profile type is generic
// and each model's own value lives in its core/civ/<model> package.
package bincat
