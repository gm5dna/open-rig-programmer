// SPDX-License-Identifier: GPL-3.0-or-later

package kw

const (
	// rejectionFrame is the radio's one and only NAK. It means EITHER
	// "Command syntax was incorrect" OR "Command was not executed due to
	// the current status of the transceiver (even though the command
	// syntax was correct)" (590:100-105, 480:130-135, 890:106-112,
	// 990:108-113), and nothing in any of the four books distinguishes the
	// two. Treated as definitive; never retried.
	rejectionFrame = "?;"
	// communicationErrorFrame is "E;": "A communication error occurred,
	// such as an overrun or framing error during a serial data
	// transmission" (590:110-112, 480:140-142).
	communicationErrorFrame = "E;"
	// receiveOverrunFrame is "O;". The TS-480 gives it a DIFFERENT cause
	// from the other three books (erratum E13) — see StreamError, which
	// carries whichever sentence belongs to the book the session is
	// talking to.
	receiveOverrunFrame = "O;"
)

// SplitFrames splits a receive buffer into complete frames on the ';'
// terminator, which is INCLUDED at the end of each returned frame. Any
// trailing bytes after the last terminator (a not-yet-complete frame, or
// nothing) are returned as rest, for the caller to prepend to the next
// read.
//
// COPIED FROM core/cat/frame.go's rule, not imported — the same decision
// core/civ took, and for the same three reasons: no cross-family import
// (core/kw/imports_test.go is the fence), its own maximum-frame constant,
// and its own evidence trail. About forty duplicated lines.
//
// Consecutive terminators (e.g. a stray leading ';', or the ";;" a noisy
// line can deliver) are tolerated and yield a 1-byte frame containing only
// the terminator, rather than being treated as an error — callers decide
// what, if anything, an empty frame means, and the matcher decides that it
// answers nothing.
//
// THAT TOLERANCE IS JUSTIFIED ON ROBUSTNESS ALONE, and specifically NOT on
// the TS-480's "; ; ; ; PS1;" wake-up string: that path is a non-goal, its
// printed semicolons are separated by spaces (480:1149), and by erratum
// E18's own reasoning the spacing may be PageMaker letter-spacing rather
// than real bytes. An accumulator that raised an error on ";;" would fail a
// session on line noise; one that emitted a zero-length frame would put one
// through the matcher.
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

// IsRejection reports whether frame is exactly the radio's NAK, "?;".
//
// EXACTLY THAT, AND NOT "E;" OR "O;". The engine turns a true answer here
// into ErrRejected — a definitive refusal of the command — so admitting a
// stream-health token would report a LINK failure as the radio REFUSING the
// user's command, and would make the three tokens indistinguishable. That
// is route 3 of spec §"Where E; and O; live", rejected there and refused
// here. The tokens reach the engine by the FatalFramer hook instead; see
// framing.go's IsFatal.
func IsRejection(frame []byte) bool {
	return string(frame) == rejectionFrame
}

// streamErrorToken returns the stream-health token frame is — "E;" or "O;"
// — or "" if it is neither.
//
// All four books print the two beside "?;" in one error-message table
// (590:97-113, 480:126-144, 890:106-123, 990:108-121) and nothing else in
// any of them is one. The comparison is exact and case-sensitive: no
// sentence in any of the four books — the TS-590S/SG, the TS-480, the
// TS-890S or the TS-990S manual — admits a lower-case token, and a codec
// that guessed at one would be inventing a frame.
func streamErrorToken(frame []byte) string {
	switch string(frame) {
	case communicationErrorFrame:
		return communicationErrorFrame
	case receiveOverrunFrame:
		return receiveOverrunFrame
	default:
		return ""
	}
}
