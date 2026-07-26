// SPDX-License-Identifier: GPL-3.0-or-later

package cat

// idAnswerLen is the fixed length of an ID answer frame: "ID" + 4-char ID
// + ";". Golden vector G1: "ID; -> ID0800;".
const idAnswerLen = 7

// BuildIDRead builds the ID read request. Golden vector G1.
//
// Takes a dialect receiver even though nothing about this frame varies by
// radio: uniform method form means M9c adds a dialect by writing a table
// rather than by re-plumbing signatures. Do not "tidy" this back to a
// package-level function.
func (d Dialect) BuildIDRead() Command {
	return newCommand([]byte("ID;"))
}

// BuildIDRead builds the ID read request. Golden vector G1.
//
// Migration scaffold: delegates to FT710; removed in Task 55.
func BuildIDRead() Command {
	return FT710.BuildIDRead()
}

// ParseIDAnswer parses an ID answer frame ("ID" + 4-char ID + ";",
// golden vector G1: "ID0800;") and returns the 4-character radio ID (e.g.
// "0800", fixed for the FT-710). It enforces exact length, prefix, and
// terminator; the reference does not otherwise constrain the ID body's
// charset, so any 4 bytes there are accepted structurally.
//
// It does NOT check the returned ID against this dialect's own CATID: the
// whole point of reading ID is to discover which radio answered, so the
// comparison belongs to the caller doing the identification, not to the
// parser. Nothing about this frame therefore varies by radio.
//
// Takes a dialect receiver even though nothing about this frame varies by
// radio: uniform method form means M9c adds a dialect by writing a table
// rather than by re-plumbing signatures. Do not "tidy" this back to a
// package-level function.
func (d Dialect) ParseIDAnswer(frame []byte) (radioID string, err error) {
	if len(frame) != idAnswerLen {
		return "", newParseError(frame, "ID answer must be 7 bytes")
	}
	if frame[0] != 'I' || frame[1] != 'D' {
		return "", newParseError(frame, "ID answer missing \"ID\" prefix")
	}
	if frame[6] != ';' {
		return "", newParseError(frame, "ID answer missing ';' terminator")
	}
	return string(frame[2:6]), nil
}

// ParseIDAnswer parses an ID answer frame and returns the 4-character
// radio ID.
//
// Migration scaffold: delegates to FT710; removed in Task 55.
func ParseIDAnswer(frame []byte) (radioID string, err error) {
	return FT710.ParseIDAnswer(frame)
}
