// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const frameCorpusPath = "testdata/frame-corpus.golden"

// corpusSlot pairs a stable label with a Slot, so the golden file reads
// as documentation rather than as opaque wire bytes.
type corpusSlot struct {
	label string
	slot  Slot
}

// corpusSlots spans every slot kind the codec knows, at its boundaries.
//
// NOTE on SixtyMSlot: it takes a 60m channel ORDINAL (1-99) and formats
// it as wire "501".."599" — see core/cat/slot.go. Passing 501 is an
// error. Revision 1 of this plan got that wrong (Codex F8).
func corpusSlots(t *testing.T) []corpusSlot {
	t.Helper()
	must := func(s Slot, err error) Slot {
		t.Helper()
		if err != nil {
			t.Fatalf("corpus slot construction failed: %v", err)
		}
		return s
	}
	return []corpusSlot{
		{"mem001", must(MemorySlot(1))},
		{"mem050", must(MemorySlot(50))},
		{"mem099", must(MemorySlot(99))},
		{"pms1L", must(PMSSlot(1, false))},
		{"pms9U", must(PMSSlot(9, true))},
		{"sixty501", must(SixtyMSlot(1))},
		{"sixty599", must(SixtyMSlot(99))},
		{"emg", EMGSlot()},
	}
}

// corpusMemoryData returns a fixed MemoryData for slot s. Every field is
// a constant so the frame depends only on the slot. CTCSS and Shift are
// set explicitly because their zero values are not legal wire bytes.
func corpusMemoryData(s Slot) MemoryData {
	return MemoryData{
		Slot:   s,
		FreqHz: 14_250_000,
		ClarHz: 0,
		RxClar: false,
		TxClar: false,
		Mode:   ModeUSB,
		// KindMemory ('1') for memory and PMS slots alike — the pairing
		// rule the M5b write trials confirmed on hardware.
		Kind:  KindMemory,
		CTCSS: CTCSSOff,
		Shift: ShiftSimplex,
	}
}

// corpusLine is one parsed golden line.
type corpusLine struct {
	label    string
	frame    string
	rejected bool
}

// splitCorpusLine parses a line buildFrameCorpus emitted. Task 57 uses it
// to feed built frames to a zero dialect.
func splitCorpusLine(line string) corpusLine {
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) != 2 {
		return corpusLine{label: line, rejected: true}
	}
	if strings.HasPrefix(parts[1], "REJECTED: ") {
		return corpusLine{label: parts[0], rejected: true}
	}
	return corpusLine{label: parts[0], frame: parts[1]}
}

// recordOrReject records either the built frame or the exact rejection.
// A builder that STOPS rejecting something is as much a regression as one
// whose bytes change, and revision 1's "skip on error" hid exactly that.
func recordOrReject(t *testing.T, out *[]string, label string, build func() (Command, error)) {
	t.Helper()
	c, err := build()
	if err != nil {
		*out = append(*out, label+"\tREJECTED: "+err.Error())
		return
	}
	*out = append(*out, label+"\t"+string(c.Bytes()))
}

// buildFrameCorpus drives every frame-producing builder over a fixed
// input set. Keep the inputs stable: appending a case is fine (and needs
// a regenerate), changing one destroys the comparison this exists for.
func buildFrameCorpus(t *testing.T) []string {
	t.Helper()

	var out []string
	emit := func(label, frame string) { out = append(out, label+"\t"+frame) }

	emit("ID.read", string(BuildIDRead().Bytes()))
	emit("AI.set.on", string(BuildAISet(true).Bytes()))
	emit("AI.set.off", string(BuildAISet(false).Bytes()))
	emit("MC.read", string(BuildMCRead().Bytes()))

	for _, sc := range corpusSlots(t) {
		recordOrReject(t, &out, "MR.read."+sc.label, func() (Command, error) { return BuildMRRead(sc.slot) })
		recordOrReject(t, &out, "MT.read."+sc.label, func() (Command, error) { return BuildMTRead(sc.slot) })
		recordOrReject(t, &out, "MC.set."+sc.label, func() (Command, error) { return BuildMCSet(sc.slot) })
		recordOrReject(t, &out, "MT.set.tag."+sc.label, func() (Command, error) { return BuildMTSet(sc.slot, true, "TAG") })
		recordOrReject(t, &out, "MT.set.clear."+sc.label, func() (Command, error) { return BuildMTSet(sc.slot, false, "") })
		recordOrReject(t, &out, "MW.set."+sc.label, func() (Command, error) { return BuildMWSet(corpusMemoryData(sc.slot)) })
	}

	for _, a := range EXAddresses() {
		recordOrReject(t, &out, "EX.read."+a.Wire(), func() (Command, error) { return BuildEXRead(a) })
	}

	return out
}

// TestFrameCorpus_MatchesGolden is the builder half of the byte-identity
// pin. A failure means a builder's output or its rejection changed, which
// during a call-site-only refactor is a bug. Do not regenerate.
func TestFrameCorpus_MatchesGolden(t *testing.T) {
	assertGolden(t, frameCorpusPath, strings.Join(buildFrameCorpus(t), "\n")+"\n")
}

// assertGolden compares got against the committed file at path and
// reports the first diverging line. Shared with the parser corpus.
func assertGolden(t *testing.T, path, got string) {
	t.Helper()
	want, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if got == string(want) {
		return
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	for i := 0; i < len(gotLines) && i < len(wantLines); i++ {
		if gotLines[i] != wantLines[i] {
			t.Fatalf("%s diverged at line %d:\n  golden: %q\n  now:    %q\n\nThis is a behaviour change, not a diff to accept.", path, i+1, wantLines[i], gotLines[i])
		}
	}
	t.Fatalf("%s length differs: golden %d lines, now %d lines", path, len(wantLines), len(gotLines))
}
