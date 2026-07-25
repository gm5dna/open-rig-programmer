// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"strings"
	"testing"
)

const allowlistCorpusPath = "testdata/allowlist-corpus.golden"

// allowedLabel renders AllowedCommand's verdict as a stable word, so the
// golden file reads as a decision, not a bool.
func allowedLabel(b bool) string {
	if b {
		return "ALLOWED"
	}
	return "REJECTED"
}

// buildAllowlistCorpus feeds every frame the frame corpus builds — every
// command every builder in this package can produce, across the same slot
// and address boundaries the frame corpus exercises — plus every golden MR
// ANSWER frame, through AllowedCommand, and records ALLOWED/REJECTED per
// frame.
//
// AllowedCommand (allowlist.go) is the outbound write gate: the last
// defence the transport layer relies on before writing arbitrary bytes to
// a physical radio, and arguably the most dialect-shaped function in this
// package — M9b moves it onto the receiver. Neither the frame corpus (which
// only sees what builders EMIT) nor the parser corpus (which only sees what
// PARSES) can catch AllowedCommand's own accept/reject boundary shifting;
// this corpus exists solely for that (fix-round finding I5).
//
// A REJECTED entry here for a golden MR answer is the CORRECT verdict, not
// noise: AllowedCommand's own doc comment requires it reject every answer
// frame outbound (see TestAllowedCommand_RejectsGoldenAnswerFrames), and a
// golden answer that started reading ALLOWED would itself be the
// regression this corpus is watching for.
func buildAllowlistCorpus(t *testing.T) []string {
	t.Helper()
	var out []string

	for _, line := range buildFrameCorpus(t) {
		cl := splitCorpusLine(line)
		if cl.malformed {
			t.Fatalf("allowlist corpus: malformed frame-corpus line %q", line)
		}
		if cl.rejected {
			// The builder itself refused this input; there is no frame to
			// feed AllowedCommand.
			continue
		}
		out = append(out, cl.label+"\t"+allowedLabel(AllowedCommand([]byte(cl.frame))))
	}

	for _, gv := range goldenMRFramesForCorpus() {
		out = append(out, "golden."+gv.label+"\t"+allowedLabel(AllowedCommand([]byte(gv.frame))))
	}

	return out
}

// TestAllowlistCorpus_MatchesGolden is the write-gate half of the
// byte-identity pin, alongside the frame corpus (builders) and the parser
// corpus (parsers). A failure means AllowedCommand's accept/reject verdict
// changed for some frame the other two corpora already exercise.
func TestAllowlistCorpus_MatchesGolden(t *testing.T) {
	assertGolden(t, allowlistCorpusPath, strings.Join(buildAllowlistCorpus(t), "\n")+"\n")
}
