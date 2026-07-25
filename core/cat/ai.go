// SPDX-License-Identifier: GPL-3.0-or-later

package cat

// aiFrameLen is the fixed length of an AI Set/Answer frame: "AI" + state
// digit + ";". Golden vector G2: "AI0;" (auto-information off).
const aiFrameLen = 4

// BuildAISet builds the Auto Information (AI) set frame. Golden vector G2:
// on=false -> "AI0;". Reference: "AI0; disables Auto Information
// (unsolicited pushes) for this port. AI resets to OFF at radio
// power-off."
func BuildAISet(on bool) Command {
	if on {
		return newCommand([]byte("AI1;"))
	}
	return newCommand([]byte("AI0;"))
}

// ParseAIAnswer parses an AI Set or Answer frame ("AI" + '0'/'1' + ";")
// and reports whether Auto Information is on. It enforces exact length,
// prefix, terminator, and that the state byte is exactly '0' or '1'.
func ParseAIAnswer(frame []byte) (on bool, err error) {
	if len(frame) != aiFrameLen {
		return false, newParseError(frame, "AI frame must be 4 bytes")
	}
	if frame[0] != 'A' || frame[1] != 'I' {
		return false, newParseError(frame, "AI frame missing \"AI\" prefix")
	}
	if frame[3] != ';' {
		return false, newParseError(frame, "AI frame missing ';' terminator")
	}
	switch frame[2] {
	case '0':
		return false, nil
	case '1':
		return true, nil
	default:
		return false, newParseError(frame, "AI state byte must be '0' or '1'")
	}
}
