// SPDX-License-Identifier: GPL-3.0-or-later

package civ

import "strings"

// The four structural bytes of a CI-V frame.
//
// A frame is PreambleByte PreambleByte <to> <from> <cn> [<sc>] [<data>…]
// EndByte. These are protocol facts common to every Icom radio in this
// tier, not per-model data, so they are package constants — the same
// division core/cat draws between its frame grammar and its dialect data.
const (
	// PreambleByte (0xFE) opens a frame. Two are required; a radio may
	// send MORE as padding, which the accumulator tolerates and
	// normalises away.
	PreambleByte = 0xFE
	// EndByte (0xFD) terminates a frame.
	EndByte = 0xFD
	// NakByte (0xFA) is the whole body of a REJECTION frame: the radio
	// refused the command. Per the spec's error handling this is the
	// CI-V analogue of CAT's "?;" — an unattributed refusal with no cause
	// to infer.
	NakByte = 0xFA
	// AckByte (0xFB) is the whole body of an ACKNOWLEDGEMENT frame: the
	// radio accepted a write. It is the answer a ClassWriteWithAck
	// command waits for.
	AckByte = 0xFB
)

// ControllerAddressDefault (0xE0) is the address a PC controller uses on
// the CI-V bus by convention, and the default a ProfileConfig omitting
// ControllerAddress receives.
//
// It is a package constant because it is a property of THIS PROGRAM's role
// on the bus rather than of any radio — every Icom document that names a
// controller address names this one. A profile may still override it (the
// field exists, and one of this package's own test profiles sets it
// elsewhere precisely so nothing here can silently hardwire 0xE0).
const ControllerAddressDefault = 0xE0

// The two command numbers this tier builds, with their sub-commands.
//
// DELIBERATELY SHORT. There is no clear/erase command number here, no
// transceive (0x1A 0x05, 0x1A 0x01, 0x1C 0x00 …) and no menu surface: a
// constant this package does not declare is a frame no builder can name
// and no gate can admit. See doc.go's non-goals.
const (
	// CmdTransceiverID (0x19) with SubTransceiverID (0x00) reads the
	// radio's own CI-V address back. Its ANSWER VALUE is undocumented on
	// every model in this tier (spec D5 entry 7, the `19 00` reply
	// value): it is recorded as a diagnostic, never matched.
	CmdTransceiverID = 0x19
	SubTransceiverID = 0x00
	// CmdMemory (0x1A) with SubMemoryContents (0x00) is the memory-record
	// read request and the memory-record set. ASSUMED family-wide — see
	// doc.go's register.
	CmdMemory         = 0x1A
	SubMemoryContents = 0x00
)

// minFrameLen is the shortest well-formed frame: preamble, preamble, to,
// from, one command byte, terminator. The FA and FB frames are exactly
// this length.
const minFrameLen = 6

// WellFormed reports whether frame is structurally a single CI-V frame:
// exactly two leading PreambleByte, at least one command byte, a trailing
// EndByte, and NO interior PreambleByte or EndByte.
//
// THE INTERIOR RULE IS THE INJECTION DEFENCE, and it is the direct
// analogue of core/cat's "exactly one trailing ';'". An interior EndByte
// splits one buffer into two frames on the wire, so a gate that approved
// the whole would have authorised a second, unexamined command. An
// interior PreambleByte does the same in the other direction: a receiver
// resynchronising on it reads the tail as a fresh frame. Every builder in
// this package emits bytes that satisfy this, and profile validation
// refuses any enum wire value that could not (profilevalidate.go).
//
// It says nothing about ADDRESSES or about which command the frame
// carries. Those are a Profile's judgement, in AllowedCommand.
func WellFormed(frame []byte) bool {
	if len(frame) < minFrameLen {
		return false
	}
	if frame[0] != PreambleByte || frame[1] != PreambleByte {
		return false
	}
	if frame[len(frame)-1] != EndByte {
		return false
	}
	for _, b := range frame[2 : len(frame)-1] {
		if b == PreambleByte || b == EndByte {
			return false
		}
	}
	return true
}

// FrameTo returns frame's destination address. The bool is false for
// anything not WellFormed, so a caller cannot read an address out of noise.
func FrameTo(frame []byte) (byte, bool) {
	if !WellFormed(frame) {
		return 0, false
	}
	return frame[2], true
}

// FrameFrom returns frame's source address, on the same terms as FrameTo.
func FrameFrom(frame []byte) (byte, bool) {
	if !WellFormed(frame) {
		return 0, false
	}
	return frame[3], true
}

// FrameCommand returns frame's command number and sub-command number.
//
// The bool is false for a frame with no room for BOTH — the FA and FB
// frames carry a command byte and no sub-command, and reading their
// terminator as a sub-command is exactly the misreading this refusal
// prevents.
func FrameCommand(frame []byte) (cn, sc byte, ok bool) {
	if !WellFormed(frame) || len(frame) < minFrameLen+1 {
		return 0, 0, false
	}
	return frame[4], frame[5], true
}

// IsRejection reports whether frame is the radio's refusal: a well-formed
// frame whose entire body is NakByte.
//
// The length is EXACT. An FA-shaped frame carrying data is not a documented
// form, and treating one as a rejection would let a corrupted answer be
// reported to the user as "the radio refused the command" — an attribution
// this package cannot support. It is neither a rejection nor an
// acknowledgement, and falls through to the unexpected-traffic count.
func IsRejection(frame []byte) bool {
	return len(frame) == minFrameLen && WellFormed(frame) && frame[4] == NakByte
}

// IsAcknowledgement reports whether frame is the radio's acceptance of a
// write: a well-formed frame whose entire body is AckByte. Exact length,
// for IsRejection's reason.
func IsAcknowledgement(frame []byte) bool {
	return len(frame) == minFrameLen && WellFormed(frame) && frame[4] == AckByte
}

// hexFrame renders bytes as space-separated lower-case hex pairs.
//
// CI-V frames are BINARY, which is why this package does not reuse
// core/cat's %q convention: 0xFE has no printable rendering, and a %q of a
// whole frame is a wall of escapes that hides the byte a reader is looking
// for. Hex pairs are also what every Icom document prints, so a diagnostic
// line can be compared with the manual directly.
func hexFrame(b []byte) string {
	const digits = "0123456789abcdef"
	var sb strings.Builder
	sb.Grow(3 * len(b))
	for i, by := range b {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteByte(digits[by>>4])
		sb.WriteByte(digits[by&0x0F])
	}
	return sb.String()
}

// copyBytes returns an independent copy of b, so the result never aliases
// b's backing array.
func copyBytes(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}
