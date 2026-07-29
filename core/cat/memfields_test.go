// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"errors"
	"testing"
)

// TestParseMemoryFrame_DoublyInvalidFrameErrorOrder pins the ORDER in which
// parseMemoryFrame (mr.go) refuses a frame that is wrong in more than one
// way: length, then prefix, then terminator, then the field block.
//
// Nothing pinned that order before M9c-3 task 3. The existing reject tables
// (mr_test.go, mw_test.go, the parser corpus) assert the error TYPE, or an
// error text for a frame with exactly one defect, so a refactor that moved
// the prefix check behind the terminator check would leave every one of
// them green whilst silently changing which message a doubly-invalid frame
// gets. Task 3 splits this function into a framing wrapper plus a shared
// field-block decoder (parseMemoryFields), and this test is what makes that
// split provably order-preserving: it is written and run GREEN against the
// pre-refactor code, then kept green through it.
//
// The comparison is against the *ParseError's Reason rather than Error(),
// which also carries the input, so a fixture change cannot mask a message
// change.
//
// Both prefixes are exercised because wantPrefix is threaded through every
// one of these messages, and it is the only reason the extracted decoder
// takes the argument at all.
func TestParseMemoryFrame_DoublyInvalidFrameErrorOrder(t *testing.T) {
	tests := []struct {
		name string
		// prefix is parseMemoryFrame's wantPrefix argument: "MR" as
		// ParseMRAnswer calls it, "MW" as the outbound gate does.
		prefix string
		frame  string
		// wantFrameLen is the length the fixture CLAIMS to have, asserted
		// before the parse: a mistyped fixture that is accidentally the
		// wrong length would otherwise pass the length-first cases for the
		// wrong reason.
		wantFrameLen int
		wantReason   string
	}{
		// The headline case: BOTH the prefix and the terminator are wrong,
		// at the documented length. Today's order yields the PREFIX error,
		// and it must keep doing so.
		{
			name:         "MR: bad prefix and bad terminator yields the prefix error",
			prefix:       "MR",
			frame:        "XX001007000000+000000110000X",
			wantFrameLen: memoryFrameLen,
			wantReason:   `MR frame missing "MR" prefix`,
		},
		{
			name:         "MW: bad prefix and bad terminator yields the prefix error",
			prefix:       "MW",
			frame:        "XX001007000000+000000110000X",
			wantFrameLen: memoryFrameLen,
			wantReason:   `MW frame missing "MW" prefix`,
		},
		// A frame carrying the OTHER command's prefix is a bad prefix, not
		// a near miss: the gate decodes MW frames with wantPrefix "MW", so
		// an MR-prefixed frame must be refused there by this same check.
		{
			name:         "MW: an MR-prefixed frame with a bad terminator still yields the prefix error",
			prefix:       "MW",
			frame:        "MR001007000000+000000110000X",
			wantFrameLen: memoryFrameLen,
			wantReason:   `MW frame missing "MW" prefix`,
		},
		// The prefix check also precedes the field block: a frame with a
		// bad prefix AND an undecodable slot field reports the prefix.
		{
			name:         "MR: bad prefix beats an invalid field block",
			prefix:       "MR",
			frame:        "XXXXX007000000+000000110000;",
			wantFrameLen: memoryFrameLen,
			wantReason:   `MR frame missing "MR" prefix`,
		},

		// Good prefix, bad terminator: the terminator error, and the field
		// block is not consulted (the fields here are all valid, so this
		// case pins only the terminator; the next one pins the precedence).
		{
			name:         "MR: good prefix and bad terminator yields the terminator error",
			prefix:       "MR",
			frame:        "MR001007000000+000000110000X",
			wantFrameLen: memoryFrameLen,
			wantReason:   "MR frame missing ';' terminator",
		},
		{
			name:         "MW: good prefix and bad terminator yields the terminator error",
			prefix:       "MW",
			frame:        "MW001007000000+000000110000X",
			wantFrameLen: memoryFrameLen,
			wantReason:   "MW frame missing ';' terminator",
		},
		{
			name:         "MR: bad terminator beats an invalid field block",
			prefix:       "MR",
			frame:        "MRXXX007000000+000000110000X",
			wantFrameLen: memoryFrameLen,
			wantReason:   "MR frame missing ';' terminator",
		},

		// The length check is first, whatever else is wrong: these frames
		// have a bad prefix, a bad terminator and a garbage field block,
		// and still report their length.
		{
			name:         "MR: too short reports the length however else it is wrong",
			prefix:       "MR",
			frame:        "XX001007000000+00000011000X",
			wantFrameLen: memoryFrameLen - 1,
			wantReason:   "MR frame must be 28 bytes",
		},
		{
			name:         "MR: too long reports the length however else it is wrong",
			prefix:       "MR",
			frame:        "XX001007000000+0000001100000X",
			wantFrameLen: memoryFrameLen + 1,
			wantReason:   "MR frame must be 28 bytes",
		},
		{
			name:         "MW: empty frame reports the length",
			prefix:       "MW",
			frame:        "",
			wantFrameLen: 0,
			wantReason:   "MW frame must be 28 bytes",
		},

		// The tail of the order: a well-framed frame reaches the field
		// block, whose first check is the slot. This is the case the three
		// above must NOT collapse into after the extraction.
		{
			name:         "MR: a well-framed frame reaches the field block",
			prefix:       "MR",
			frame:        "MRXXX007000000+000000110000;",
			wantFrameLen: memoryFrameLen,
			wantReason:   "MR frame: invalid slot field (P1)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.frame) != tc.wantFrameLen {
				t.Fatalf("fixture %q is %d bytes, but the case claims %d — fix the fixture, not the expectation", tc.frame, len(tc.frame), tc.wantFrameLen)
			}

			_, err := FT710.parseMemoryFrame([]byte(tc.frame), tc.prefix)
			if err == nil {
				t.Fatalf("FT710.parseMemoryFrame(%q, %q) = nil error, want %q", tc.frame, tc.prefix, tc.wantReason)
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("FT710.parseMemoryFrame(%q, %q) error is %T, want *ParseError", tc.frame, tc.prefix, err)
			}
			if pe.Reason != tc.wantReason {
				t.Errorf("FT710.parseMemoryFrame(%q, %q) Reason = %q, want exactly %q — this pins WHICH check fires first, not merely that one did", tc.frame, tc.prefix, pe.Reason, tc.wantReason)
			}
		})
	}
}
