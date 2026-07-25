// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"strings"
	"testing"
)

const parserCorpusPath = "testdata/parser-corpus.golden"

// parserInputs is a fixed set of frames spanning every parser, valid and
// invalid. The invalid ones matter as much as the valid: a receiver
// hardwired to the wrong data changes which inputs are REJECTED long
// before it changes a successful parse.
func parserInputs() []struct{ label, frame string } {
	return []struct{ label, frame string }{
		{"ID.valid", "ID0800;"},
		{"ID.other", "ID0761;"},
		{"ID.short", "ID080;"},
		{"AI.on", "AI1;"},
		{"AI.off", "AI0;"},
		{"AI.garbage", "AI2;"},
		{"MC.mem", "MC001;"},
		{"MC.pms", "MCP1L;"},
		{"MC.sixty", "MC501;"},
		{"MC.emg", "MCEMG;"},
		{"MC.outofspace", "MC100;"},
		{"MT.short", "MT0011TAG;"},
		{"MT.emptytag", "MT0011;"},
		{"MT.badslot", "MT1001TAG;"},
		{"MR.badlen", "MR001;"},
		{"EX.valid", "EX010101000;"},
		{"EX.nonmember", "EX050101000;"},
	}
}

// buildParserCorpus records what every parser returns for every input —
// the value on success, the error text on failure. Error strings are part
// of the contract: they are what a user sees.
func buildParserCorpus(t *testing.T) []string {
	t.Helper()
	var out []string

	for _, in := range parserInputs() {
		f := []byte(in.frame)

		id, err := ParseIDAnswer(f)
		out = append(out, record("ParseIDAnswer."+in.label, id, err))

		on, err := ParseAIAnswer(f)
		out = append(out, record("ParseAIAnswer."+in.label, fmt.Sprintf("%v", on), err))

		slot, err := ParseMCAnswer(f)
		out = append(out, record("ParseMCAnswer."+in.label, slotWire(slot), err))

		s, disp, tag, err := ParseMTAnswer(f)
		out = append(out, record("ParseMTAnswer."+in.label, fmt.Sprintf("%s|%v|%q", slotWire(s), disp, tag), err))

		md, err := ParseMRAnswer(f)
		out = append(out, record("ParseMRAnswer."+in.label, fmt.Sprintf("%s|%d|%c|%c", slotWire(md.Slot), md.FreqHz, md.Mode.Wire(), md.Kind), err))

		addr, raw, err := ParseEXAnswer(f)
		out = append(out, record("ParseEXAnswer."+in.label, fmt.Sprintf("%s|%q", addr.Wire(), raw), err))
	}

	// A real MR answer, taken from the existing golden vectors rather
	// than invented — read core/cat/mr_test.go for G4/G6/G7 and use those
	// frames verbatim here.
	for _, gv := range goldenMRFramesForCorpus() {
		md, err := ParseMRAnswer([]byte(gv.frame))
		out = append(out, record("ParseMRAnswer.golden."+gv.label,
			fmt.Sprintf("%s|%d|%c|%c|%d", slotWire(md.Slot), md.FreqHz, md.Mode.Wire(), md.Kind, md.ClarHz), err))
	}

	// Membership rules, most at risk of being silently hardwired.
	for _, w := range []string{"001", "099", "100", "P1L", "P9U", "P0L", "501", "599", "600", "EMG", "000", "00001", "abc"} {
		s, err := ParseSlot(w)
		out = append(out, record("ParseSlot."+w, slotWire(s), err))
	}
	for _, c := range []byte{'0', '1', '9', 'A', 'F', 'G', 'a', '!'} {
		m, err := ParseMode(c)
		out = append(out, record(fmt.Sprintf("ParseMode.%c", c), string(m.Wire()), err))
	}
	for _, w := range []string{"010101", "010321", "050101", "999999", "01010"} {
		a, err := ParseEXAddress(w)
		out = append(out, record("ParseEXAddress."+w, a.Wire(), err))
	}

	return out
}

// goldenMRFramesForCorpus returns real 28-byte MR answer frames copied
// verbatim from this package's existing golden vectors: core/cat/mr_test.go's
// TestParseMRAnswer_G4, TestParseMRAnswer_G6 and TestParseMRAnswer_G7SharedLayout.
// G7's test constructs its frame with the MW prefix swapped to MR, but the
// literal string in that test is already MR-prefixed, so it is copied as-is.
func goldenMRFramesForCorpus() []struct{ label, frame string } {
	return []struct{ label, frame string }{
		{"G4", "MR001007000000+000000110000;"},
		{"G6", "MRP1L001810000+000000150000;"},
		{"G7", "MR099052354000-012010411002;"},
	}
}

// record renders one parser outcome as a single golden line.
func record(label, value string, err error) string {
	if err != nil {
		return label + "\tERR: " + err.Error()
	}
	return label + "\tOK: " + value
}

// slotWire renders a Slot safely, including the zero value.
func slotWire(s Slot) string {
	if s.Wire() == "" {
		return "<zero>"
	}
	return s.Wire()
}

// TestParserCorpus_MatchesGolden is the parser half of the pin — the
// check that catches a helper still consulting FT710 through a Dialect
// receiver. The frame corpus cannot see that, because builders and
// parsers share the membership helpers (Codex F3).
func TestParserCorpus_MatchesGolden(t *testing.T) {
	assertGolden(t, parserCorpusPath, strings.Join(buildParserCorpus(t), "\n")+"\n")
}
