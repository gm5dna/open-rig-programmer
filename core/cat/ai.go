// SPDX-License-Identifier: GPL-3.0-or-later

package cat

// BuildAISet builds the Auto Information (AI) set frame. Golden vector G2:
// on=false -> "AI0;". Reference: "AI0; disables Auto Information
// (unsolicited pushes) for this port. AI resets to OFF at radio
// power-off."
//
// Takes a dialect receiver even though nothing about this frame varies by
// radio: uniform method form means M9c adds a dialect by writing a table
// rather than by re-plumbing signatures. Do not "tidy" this back to a
// package-level function.
func (d Dialect) BuildAISet(on bool) Command {
	if on {
		return newCommand([]byte("AI1;"))
	}
	return newCommand([]byte("AI0;"))
}
