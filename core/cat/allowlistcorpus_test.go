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
		out = append(out, cl.label+"\t"+allowedLabel(FT710.AllowedCommand([]byte(cl.frame))))
	}

	for _, gv := range goldenMRFramesForCorpus() {
		out = append(out, "golden."+gv.label+"\t"+allowedLabel(FT710.AllowedCommand([]byte(gv.frame))))
	}

	out = append(out, stage0PolicyCorpus(t)...)

	return out
}

// stage0PolicyCorpus is the FT-891 Stage 0 axes' half of this corpus, and
// its rows are APPENDED after every FT-710 row above rather than interleaved
// with them. That ordering is the point: the golden's existing lines must
// stay byte-identical at their existing positions, so a designed delta shows
// up as new lines at the end of the file and nothing else moves.
//
// EVERY OTHER ROW IN THIS FILE IS THE FT-710's, and the FT-710 declares the
// WIDE reading of all four axes — so none of them can say anything about the
// narrow one. These rows offer the SAME frame to two dialects that disagree
// about it, which is the only shape in which a corpus can witness a policy:
// a row that read ALLOWED on both would mean the axis was not reaching the
// gate at all.
//
// The frames are built rather than typed where a builder will produce them,
// so a row can never drift from the shape this package actually emits; the
// forged ones are spliced from a frame the same dialect built, one byte
// changed, so that byte is the only thing the verdict can be about.
func stage0PolicyCorpus(t *testing.T) []string {
	t.Helper()

	var out []string
	record := func(label string, d Dialect, frame []byte) {
		out = append(out, label+"\t"+allowedLabel(d.AllowedCommand(frame)))
	}

	// S0.2 — MC of a 60m slot and of EMG, under both policies.
	for _, wire := range []string{"501", "EMG"} {
		record("stage0.mc."+wire+".MCSelectsAll", FT710, []byte("MC"+wire+";"))
		record("stage0.mc."+wire+".MCSelectsMemoryPMS", mcMemoryPMSDialect, []byte("MC"+wire+";"))
	}

	// S0.3 — MT read of a 60m slot and of EMG, under both policies, with the
	// MR read of the same slot alongside: the narrow dialect must still read
	// those banks with MR, and a row that lost that would be recording a
	// regression as if it were the design.
	for _, wire := range []string{"501", "EMG"} {
		record("stage0.mtread."+wire+".MTReadsReadable", FT710, []byte("MT"+wire+";"))
		record("stage0.mtread."+wire+".MTReadsMemoryPMS", mtReadMemoryPMSDialect, []byte("MT"+wire+";"))
		record("stage0.mrread."+wire+".MTReadsMemoryPMS", mtReadMemoryPMSDialect, []byte("MR"+wire+";"))
	}

	// S0.4 — an MW Set whose P5 byte is '1', under both policies.
	txClarOn := func(d Dialect) []byte {
		slot, err := d.MemorySlot(7)
		if err != nil {
			t.Fatalf("stage0 corpus: MemorySlot(7): %v", err)
		}
		m := MemoryData{
			Slot: slot, FreqHz: 14250000, TxClar: true,
			Mode: ModeUSB, Kind: d.MWWriteKind(),
			CTCSS: CTCSSOff, Shift: ShiftSimplex,
		}
		cmd, err := d.BuildMWSet(m)
		if err != nil {
			// P5Fixed refuses to BUILD one, which is the design; the frame
			// the gate must judge is then spliced from the same dialect's
			// own TxClar-false Set.
			m.TxClar = false
			cmd, err = d.BuildMWSet(m)
			if err != nil {
				t.Fatalf("stage0 corpus: BuildMWSet: %v", err)
			}
			frame := append([]byte(nil), cmd.Bytes()...)
			frame[memTxClarOffset] = '1'
			return frame
		}
		return cmd.Bytes()
	}
	record("stage0.mw.p5one.P5TxClar", FT710, txClarOn(FT710))
	record("stage0.mw.p5one.P5Fixed", p5FixedDialect, txClarOn(p5FixedDialect))

	// S0.6 — a combined MT Set whose P11 byte is '1', under both policies.
	p11One := func(d Dialect) []byte {
		m := p11TestRecord(t, d)
		var (
			cmd Command
			err error
		)
		if d.MTP11() == P11TagDisplay {
			cmd, err = d.BuildMTSetCombinedDisplay(m, "A", true)
		} else {
			// P11Fixed has no builder that will emit a '1' there, which is
			// the design; splice it into the frame it does build.
			cmd, err = d.BuildMTSetCombined(m, "A")
		}
		if err != nil {
			t.Fatalf("stage0 corpus: combined MT Set: %v", err)
		}
		frame := append([]byte(nil), cmd.Bytes()...)
		frame[mtCombinedP11Offset] = '1'
		return frame
	}
	record("stage0.mtcombined.p11one.P11TagDisplay", combinedTagDisplayDialect, p11One(combinedTagDisplayDialect))
	record("stage0.mtcombined.p11one.P11Fixed", combinedDialect, p11One(combinedDialect))

	return out
}

// TestAllowlistCorpus_MatchesGolden is the write-gate half of the
// byte-identity pin, alongside the frame corpus (builders) and the parser
// corpus (parsers). A failure means AllowedCommand's accept/reject verdict
// changed for some frame the other two corpora already exercise.
func TestAllowlistCorpus_MatchesGolden(t *testing.T) {
	assertGolden(t, allowlistCorpusPath, strings.Join(buildAllowlistCorpus(t), "\n")+"\n")
}
