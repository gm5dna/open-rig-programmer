// SPDX-License-Identifier: GPL-3.0-or-later

package cat

// Frame is a single CAT wire frame: a command, its parameters, and the
// trailing ';' terminator. Reference: "General framing — ASCII command
// frames terminated by ';'."
type Frame []byte

// rejectionFrame is the radio's one and only NAK. Reference golden vector
// G12: "?;" — rejection reply.
const rejectionFrame = "?;"

// SplitFrames splits a receive buffer into complete frames on the ';'
// terminator, which is INCLUDED at the end of each returned frame. Any
// trailing bytes after the last terminator (a not-yet-complete frame, or
// nothing) are returned as rest, for the caller to prepend to the next
// read.
//
// Consecutive terminators (e.g. a stray leading ';') are tolerated and
// yield a 1-byte frame containing only the terminator, rather than being
// treated as an error — callers decide what, if anything, an empty frame
// means.
//
// Every byte of buf appears in exactly one of the returned frames or in
// rest; SplitFrames never allocates new backing arrays, only subslices of
// buf.
func SplitFrames(buf []byte) (frames [][]byte, rest []byte) {
	start := 0
	for i, b := range buf {
		if b == ';' {
			frames = append(frames, buf[start:i+1])
			start = i + 1
		}
	}
	return frames, buf[start:]
}

// IsRejection reports whether frame is exactly the radio's NAK, "?;"
// (golden vector G12). Reference: "The only NAK is ?; — an unattributed
// generic command failure."
func IsRejection(frame []byte) bool {
	return string(frame) == rejectionFrame
}
