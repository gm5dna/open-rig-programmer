// SPDX-License-Identifier: GPL-3.0-or-later

package bincat

import (
	"fmt"
	"strings"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// FrameLen is the length of every command this family sends: four
// argument bytes followed by the opcode byte, sent left to right, opcode
// last (spec.md §Frame grammar — the shape FT-920, FT-890/900 and
// FT-1000MP all document explicitly; the FT-890 manual's own "send these
// five bytes in reverse order" instruction is confirmed to describe how
// its diagram is laid out, not a wire-level difference — no genuine
// byte-order difference exists across the four).
//
// There is no preamble and no terminator: FrameLen is the ENTIRE
// structural fact this family's frames have.
const FrameLen = 5

// BuildFrame returns a fresh 5-byte frame: args[0..3] followed by opcode.
// The returned slice never aliases args.
func BuildFrame(opcode byte, args [4]byte) []byte {
	return []byte{args[0], args[1], args[2], args[3], opcode}
}

// ParseFrame splits frame back into its opcode and four argument bytes,
// refusing anything not exactly FrameLen bytes.
func ParseFrame(frame []byte) (opcode byte, args [4]byte, err error) {
	if len(frame) != FrameLen {
		return 0, args, fmt.Errorf("%w: frame is %d bytes, want exactly %d", ErrFrame, len(frame), FrameLen)
	}
	copy(args[:], frame[:4])
	return frame[4], args, nil
}

// Command is the transport.Command every bincat driver hands to
// Engine.Do: one fixed 5-byte frame, opaque beyond that.
type Command struct {
	frame [FrameLen]byte
}

// NewCommand returns a Command wrapping opcode and args — see BuildFrame.
func NewCommand(opcode byte, args [4]byte) Command {
	return Command{frame: [FrameLen]byte{args[0], args[1], args[2], args[3], opcode}}
}

// Bytes returns a fresh copy of c's wire bytes — transport.Command's
// contract that every call returns an independently owned slice.
func (c Command) Bytes() []byte {
	b := make([]byte, FrameLen)
	copy(b, c.frame[:])
	return b
}

// String renders c as space-separated hex, the form every manual in this
// family's evidence table prints its own worked examples as.
func (c Command) String() string {
	return hexFrame(c.frame[:])
}

var _ transport.Command = Command{}

// hexFrame renders bytes as space-separated lower-case hex pairs — this
// family's frames are binary, so %q's escapes would hide the byte a reader
// is looking for (core/civ's frame.go draws the identical distinction).
func hexFrame(b []byte) string {
	parts := make([]string, len(b))
	for i, by := range b {
		parts[i] = fmt.Sprintf("%02x", by)
	}
	return strings.Join(parts, " ")
}
