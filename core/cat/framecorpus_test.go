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
		{"mem001", must(FT710.MemorySlot(1))},
		{"mem050", must(FT710.MemorySlot(50))},
		{"mem099", must(FT710.MemorySlot(99))},
		{"pms1L", must(FT710.PMSSlot(1, false))},
		{"pms9U", must(FT710.PMSSlot(9, true))},
		{"sixty501", must(FT710.SixtyMSlot(1))},
		{"sixty599", must(FT710.SixtyMSlot(99))},
		{"emg", FT710.EMGSlot()},
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

// corpusMemoryDataVariant is corpusMemoryData's non-zero counterpart: every
// field corpusMemoryData holds at its zero-ish value (ClarHz 0, both clar
// flags false, CTCSSOff, ShiftSimplex) is here a real, non-default value
// instead — the same combination golden vector G7 exercises on the parse
// side (mr_test.go's TestParseMRAnswer_G7SharedLayout). Without this,
// BuildMWSet's encode side never proves it gets the clarifier sign, the
// clar flags, or a non-off CTCSS/Shift onto the wire correctly: a sign
// inversion or a CTCSS/Shift encode remap changed no golden byte before
// this existed (fix-round finding I3).
func corpusMemoryDataVariant(s Slot) MemoryData {
	return MemoryData{
		Slot:   s,
		FreqHz: 14_250_000,
		ClarHz: -120,
		RxClar: true,
		TxClar: true,
		Mode:   ModeUSB,
		Kind:   KindMemory,
		CTCSS:  CTCSSEncDec,
		Shift:  ShiftMinus,
	}
}

// corpusLine is one parsed golden line.
type corpusLine struct {
	label string
	frame string

	// rejected is true when the builder itself returned an error for this
	// label: the line reads "label\tREJECTED: <error text>".
	rejected bool

	// malformed is true when the LINE is not a valid "label\tvalue" pair
	// at all — a corpus/parsing bug, never a builder outcome. Distinct
	// from rejected: assertGolden's input always ends "...\n", so naively
	// splitting a golden file's full text on "\n" yields a trailing ""
	// element. Before this field existed, splitCorpusLine folded that ""
	// (and any other malformed line) into rejected — so a truncated or
	// corrupted golden file would silently look like an all-rejections
	// pass to a consumer (Task 57) instead of failing loudly (fix-round
	// finding M5).
	malformed bool
}

// splitCorpusLine parses a line buildFrameCorpus emitted. Task 57 uses it
// to feed built frames to a zero dialect. Callers MUST treat malformed
// distinctly from rejected — see corpusLine.malformed's doc comment — and
// treat a malformed line as fatal, not as a rejection to skip past.
func splitCorpusLine(line string) corpusLine {
	parts := strings.SplitN(line, "\t", 2)
	if len(parts) != 2 {
		return corpusLine{label: line, malformed: true}
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

	emit("ID.read", string(FT710.BuildIDRead().Bytes()))
	emit("AI.set.on", string(FT710.BuildAISet(true).Bytes()))
	emit("AI.set.off", string(FT710.BuildAISet(false).Bytes()))
	emit("MC.read", string(FT710.BuildMCRead().Bytes()))

	for _, sc := range corpusSlots(t) {
		recordOrReject(t, &out, "MR.read."+sc.label, func() (Command, error) { return FT710.BuildMRRead(sc.slot) })
		recordOrReject(t, &out, "MT.read."+sc.label, func() (Command, error) { return FT710.BuildMTRead(sc.slot) })
		recordOrReject(t, &out, "MC.set."+sc.label, func() (Command, error) { return FT710.BuildMCSet(sc.slot) })
		recordOrReject(t, &out, "MT.set.tag."+sc.label, func() (Command, error) { return FT710.BuildMTSet(sc.slot, true, "TAG") })
		recordOrReject(t, &out, "MT.set.clear."+sc.label, func() (Command, error) { return FT710.BuildMTSet(sc.slot, false, "") })
		recordOrReject(t, &out, "MW.set."+sc.label, func() (Command, error) { return FT710.BuildMWSet(corpusMemoryData(sc.slot)) })
		recordOrReject(t, &out, "MW.set.variant."+sc.label, func() (Command, error) { return FT710.BuildMWSet(corpusMemoryDataVariant(sc.slot)) })
	}

	for _, a := range FT710.EXAddresses() {
		recordOrReject(t, &out, "EX.read."+FT710.EXWire(a), func() (Command, error) { return FT710.BuildEXRead(a) })
	}

	// The zero-value EXAddress is not a Table 2 member: exercises
	// BuildEXRead's KnownEXAddress guard (ex.go), previously untested here
	// because the EXAddresses() loop above only ever supplies real
	// members, so all 296 always succeeded (fix-round finding I6).
	recordOrReject(t, &out, "EX.read.invalid", func() (Command, error) { return FT710.BuildEXRead(EXAddress{}) })

	return out
}

// TestFrameCorpus_MatchesGolden is the builder half of the byte-identity
// pin. A failure means a builder's output or its rejection changed, which
// during a call-site-only refactor is a bug. Do not regenerate.
func TestFrameCorpus_MatchesGolden(t *testing.T) {
	assertGolden(t, frameCorpusPath, strings.Join(buildFrameCorpus(t), "\n")+"\n")
}

// TestSplitCorpusLine_MalformedVsRejected pins the fix for finding M5: a
// line with no tab — in particular the trailing "" a naive
// strings.Split(golden, "\n") produces, since assertGolden's golden text
// always ends "...\n" — must be reported as malformed, never folded into
// rejected. A genuine "REJECTED: ..." line must still classify as
// rejected, not malformed.
func TestSplitCorpusLine_MalformedVsRejected(t *testing.T) {
	if cl := splitCorpusLine(""); !cl.malformed || cl.rejected {
		t.Errorf("splitCorpusLine(%q) = %+v, want malformed=true, rejected=false", "", cl)
	}
	if cl := splitCorpusLine("no tab here"); !cl.malformed || cl.rejected {
		t.Errorf("splitCorpusLine(%q) = %+v, want malformed=true, rejected=false", "no tab here", cl)
	}
	if cl := splitCorpusLine("EX.read.invalid\tREJECTED: boom"); cl.malformed || !cl.rejected {
		t.Errorf("splitCorpusLine(%q) = %+v, want malformed=false, rejected=true", "EX.read.invalid\tREJECTED: boom", cl)
	}
	if cl := splitCorpusLine("ID.read\tID;"); cl.malformed || cl.rejected || cl.frame != "ID;" {
		t.Errorf("splitCorpusLine(%q) = %+v, want a plain frame", "ID.read\tID;", cl)
	}
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
