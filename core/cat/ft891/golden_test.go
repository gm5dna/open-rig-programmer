// SPDX-License-Identifier: GPL-3.0-or-later

package ft891_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/ft891"
)

// This file is Stage 1 item 3: the mechanical byte-compare of this
// dialect's codec against the nineteen hand-derived wire frames in
// testdata/*.golden.
//
// # What the vectors are, and what this file may do with them
//
// They are evidence leg G, derived by a QUARANTINED agent that never opened
// this repository: no code, no generator, no fixture and no other document,
// only 300 and 600 dpi renders of the Yaesu FT-891 CAT Operation Reference
// Book rev 1909-C's position charts and their parameter legends. Every field
// width, every position boundary and every assumption that had to be
// inherited rather than read is itemised in testdata/provenance.md and
// repeated in each vector file's own header block, which the tables below
// cite for the values they hardcode.
//
// THIS FILE MAY NOT MODIFY ANY VECTOR, and no failure here is ever fixed by
// editing one. The five artefacts were frozen at commit adf3d21, whose
// message records their SHA-256s; TestGoldenVectorsFrozen below enforces the
// same hashes in CI, so the freeze survives a rewritten history or a stray
// regeneration rather than depending on someone running a diff gate. A
// golden-vs-codec mismatch is a STOP for orchestrator arbitration AGAINST
// THE PDF — either the hand derivation or the codec misreads the manual —
// which is why requireGoldenFrame prints both sides, both lengths and the
// first differing wire position: the failure output is the arbitration's
// input.
//
// crosscheck_test.go's frozenEvidenceSHA256 covers the OTHER two legs (the
// group-boundary ledger and transcription B) and says in as many words that
// leg G's five artefacts "belong to the frame-geometry task and are pinned
// by its own test". This is that test; between the two maps every artefact
// commit adf3d21 froze is pinned by whichever file reads it.
//
// # Why the expectations are hardcoded rather than parsed out of the frame
//
// A test that parsed a frame and rebuilt it would prove the codec is
// self-consistent and nothing else: a decoder and an encoder sharing one
// wrong offset would round-trip perfectly. So every table below states the
// vector's fields as LITERALS read by hand off the golden file's own
// documented field map, and each literal carries the 1-indexed wire position
// it was read from. The parse leg then binds the codec's reading of those
// bytes to the derivation's stated intent, and the build leg binds the
// encoder to the bytes themselves.
//
// # Hardware status
//
// UNVERIFIED, for all nineteen vectors, and there is no route to verifying
// them: no FT-891 has ever been asked anything by this project (doc.go's
// provenance section). Green here means the codec agrees with the manual as
// one agent read it, not that any radio accepts these bytes. doc.go's
// ASSUMED register names the Stage R capture that would lift each inherited
// assumption, and this file cites those entries BY NAME wherever a vector's
// bytes rest on one.

// goldenDir is where evidence leg G lives, relative to this package's
// directory (go test's working directory).
const goldenDir = "testdata"

// frozenVectorSHA256 is the freeze, transcribed from the commit message of
// adf3d21 ("ft891: the three quarantined evidence legs, committed
// verbatim"), whose "SHA-256 at commit:" block records one hash per
// artefact.
//
// provenance.md is in here with the four vector files because it is not
// commentary about them: it is the assumption register the tests below cite
// for every value they hardcode, and a vector file whose assumptions had
// been quietly rewritten would be as corrupt as one whose bytes had.
var frozenVectorSHA256 = map[string]string{
	"mt-vectors.golden": "85f7c12247925bd8c8af77b76ef50e34dca19b9aa30c94cae7d130587e37e537",
	"mw-vectors.golden": "53eac204b8422861816e0854159c4cfd27aa8e37e1c4aeff4c3a374317dc82e7",
	"mr-vectors.golden": "b0267d41651bf38d3f527dfdd2d71047e61e5629a38890e0e9908cb803a59f30",
	"ex-vectors.golden": "b99804a22f645eee9e1abd753787dc122323f15066f829a2699420dceb9f4490",
	"provenance.md":     "a1575c969762272f114a4844eb7c5424cbe4b8f6d99acdc5f8676d337cd9dafc",
}

// TestGoldenVectorsFrozen recomputes each frozen artefact's SHA-256 and
// compares it with the value commit adf3d21 recorded, so that the freeze is
// self-enforcing in CI rather than a fact recoverable only by git
// archaeology.
//
// The second half of the test is the one that catches the interesting case:
// a walk of testdata/*.golden requires EVERY vector file present to be
// covered by the map above. Without it, a new unfrozen vector file could be
// added beside the four and pass a test that only ever looked up names it
// already knew.
func TestGoldenVectorsFrozen(t *testing.T) {
	for name, want := range frozenVectorSHA256 {
		path := filepath.Join(goldenDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading frozen artefact %s: %v", path, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("FREEZE BROKEN — %s has changed since commit adf3d21.\n"+
				"  recorded SHA-256 %s\n"+
				"  present  SHA-256 %s\n"+
				"This artefact is quarantined evidence: it is never regenerated and\n"+
				"never edited to satisfy a test. Restore it from the repository root\n"+
				"with `git checkout adf3d21 -- core/cat/ft891/testdata/%s` and report\n"+
				"the change.",
				path, want, got, name)
		}
	}

	present, err := filepath.Glob(filepath.Join(goldenDir, "*.golden"))
	if err != nil {
		t.Fatalf("globbing %s: %v", goldenDir, err)
	}
	if len(present) == 0 {
		t.Fatalf("no *.golden files found in %s — the vectors are missing", goldenDir)
	}
	for _, path := range present {
		if _, ok := frozenVectorSHA256[filepath.Base(path)]; !ok {
			t.Errorf("%s is a vector file with no recorded SHA-256: every golden in %s must be frozen by a commit that records its hash",
				path, goldenDir)
		}
	}
}

// goldenVector is one record of a *.golden file: the format is
// "name<TAB>frame" per line, with '#' comment lines and no other content.
type goldenVector struct {
	file  string // the file it came from, for failure messages
	line  int    // 1-indexed line number, likewise
	name  string
	frame string
}

// loadGoldenVectors reads one vector file into its records, in file order.
//
// The frame is taken VERBATIM after the single tab — never trimmed.
// Trailing SPACE is significant frame content in two of the MT vectors (the
// tag field is padded to its full 12 positions, mt-vectors.golden's
// INHERITED-ASSUMED item 1), so a convenience TrimSpace here would silently
// rewrite the evidence this file exists to compare against. The parser
// instead refuses anything that is not exactly one tab, and refuses a CR, so
// a file that acquired either would fail loudly rather than be read
// approximately.
func loadGoldenVectors(t *testing.T, file string) []goldenVector {
	t.Helper()

	path := filepath.Join(goldenDir, file)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading vectors %s: %v", path, err)
	}

	var out []goldenVector
	for i, line := range strings.Split(string(data), "\n") {
		lineNo := i + 1
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "\r") {
			t.Fatalf("%s:%d: record contains a CR — the vector files are LF-only", path, lineNo)
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			t.Fatalf("%s:%d: record must be exactly \"name<TAB>frame\", got %d tab-separated fields", path, lineNo, len(parts))
		}
		if parts[0] == "" || parts[1] == "" {
			t.Fatalf("%s:%d: record has an empty name or frame", path, lineNo)
		}
		out = append(out, goldenVector{file: path, line: lineNo, name: parts[0], frame: parts[1]})
	}
	if len(out) == 0 {
		t.Fatalf("%s: no vector records found", path)
	}
	return out
}

// requireVectorNames pins both the number of records in a file and their
// names, in order, so that the tables below are known to be addressing the
// vectors they claim to. A vector added, removed or renamed under a table
// that still matched positionally would otherwise test the wrong frame.
func requireVectorNames(t *testing.T, got []goldenVector, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: want %d vectors, got %d", got[0].file, len(want), len(got))
	}
	for i := range want {
		if got[i].name != want[i] {
			t.Fatalf("%s:%d: want vector %q at position %d, got %q", got[i].file, got[i].line, want[i], i, got[i].name)
		}
	}
}

// requireGoldenLength pins the frame length the quarantined deriver counted
// twice off the chart (provenance.md's "Method" section records both passes
// for every chart), so that a length disagreement is reported as itself
// rather than as a byte difference at the first position that happens to
// shift.
func requireGoldenLength(t *testing.T, v goldenVector, want int) {
	t.Helper()
	if got := len(v.frame); got != want {
		t.Fatalf("%s (%s:%d) is %d bytes, want %d — the counted chart length. THIS IS A STOP.",
			v.name, v.file, v.line, got, want)
	}
}

// requireGoldenFrame is the byte comparison, and the STOP report.
//
// builtBy names the API under test so that the failure says which of the
// codec's builders disagreed, and firstDifference converts the byte offset
// into the manual's own 1-indexed wire position — the coordinate the
// position charts and provenance.md are both written in, so that a mismatch
// can be taken straight to the chart.
func requireGoldenFrame(t *testing.T, v goldenVector, built cat.Command, builtBy string) {
	t.Helper()
	got, want := string(built.Bytes()), v.frame
	if got == want {
		return
	}
	t.Fatalf("GOLDEN-VS-CODEC MISMATCH — THIS IS A STOP, NOT A TEST TO ADJUST.\n"+
		"  vector    %s (%s:%d)\n"+
		"  built by  %s\n"+
		"  golden    %q (%d bytes)\n"+
		"  codec     %q (%d bytes)\n"+
		"  %s\n"+
		"The vectors are frozen (SHA-256s recorded at commit adf3d21) and this\n"+
		"test may not be made to pass by editing one. Either the quarantined hand\n"+
		"derivation or this codec misreads manual rev 1909-C; the orchestrator\n"+
		"arbitrates against the PDF.",
		v.name, v.file, v.line, builtBy, want, len(want), got, len(got), firstDifference(got, want))
}

// firstDifference describes where two frames first diverge, in 1-indexed
// wire positions.
func firstDifference(got, want string) string {
	n := len(got)
	if len(want) < n {
		n = len(want)
	}
	for i := 0; i < n; i++ {
		if got[i] != want[i] {
			return fmt.Sprintf("first difference at wire position %d: golden %q (%#02x), codec %q (%#02x)",
				i+1, want[i], want[i], got[i], got[i])
		}
	}
	return fmt.Sprintf("frames agree over the first %d positions and differ only in length", n)
}

// dialectTagFill asks the dialect what it pads a short combined-MT tag
// field with, by building a one-character tag and reading the last position
// of the field — the byte before the terminator.
//
// ASKED, NOT SPELT. MTPolicy.TagFill is unexported and this is an external
// test package, but that is the lesser reason: the fill byte is an ASSUMED
// entry on this dialect's register (doc.go, entry "MTPolicy.TagFill" —
// inherited from the FT-710, the P12 legend naming a width and an alphabet
// and no fill), and a test that wrote ' ' here would be asserting the
// assumption against itself. Asking the dialect makes the padding
// assertions below say what they mean: the golden's pad bytes are whatever
// THIS DIALECT declares, and if Stage R replaces the declaration the
// assertion moves with it — at which point the goldens themselves become
// the STOP, which is the correct outcome.
func dialectTagFill(t *testing.T, d cat.Dialect) byte {
	t.Helper()
	slot, err := d.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot(1): %v", err)
	}
	probe, err := d.BuildMTSetCombinedDisplay(cat.MemoryData{
		Slot: slot, FreqHz: 7_100_000, Mode: cat.Mode('1'),
		Kind: cat.CombinedMTSetKind, CTCSS: cat.CTCSSOff, Shift: cat.ShiftSimplex,
	}, "A", false)
	if err != nil {
		t.Fatalf("building the tag-fill probe: %v", err)
	}
	b := probe.Bytes()
	// Position 41 is ';' and positions 30-40 are fill, the one-byte tag
	// having taken position 29 alone.
	return b[len(b)-2]
}

// mtVectors states, as literals, what each MT vector's bytes encode.
//
// Read by hand off mt-vectors.golden's own field map (its header block, and
// provenance.md "MT — MEMORY WRITE & TAG"), which counts the 41 positions
// as:
//
//	1-2 "MT" | 3-5 P1 | 6-14 P2 | 15 "+/-" | 16-19 P3 | 20 P4 | 21 P5
//	| 22 P6 | 23 P7 | 24 P8 | 25-26 P9 | 27 P10 | 28 P11 | 29-40 P12 | 41 ";"
//
// TWO POSITIONS ARE THIS RADIO'S OWN READING and are why this dialect exists
// as a separate one at all:
//
//   - P5 (position 21) is printed "0: (Fixed)" on every FT-891 block that
//     carries the 28-position grid, so it is schema and not the TX
//     clarifier flag its siblings print there. All six vectors carry '0',
//     and the codec REQUIRES it (parseMemoryFields under P5Fixed). Pinned
//     against the FTdx10 by dialect_test.go's TestDifferencePinMemoryP5.
//   - P11 (position 28) is printed `0: TAG "OFF" 1: TAG "ON"`, so it is a
//     LIVE FLAG rather than the sibling family's fixed '0' — which is why
//     the table carries a display column at all, and why this file uses the
//     display-bearing builder/parser pair throughout. Pinned by
//     TestDifferencePinMTP11.
//
// P7 (position 23) is '0' in all six, and both directions of the codec
// accept that for their own reason. The parser's vocabulary for this form is
// the documented read pair {'0' VFO, '1' Memory} — which on THIS radio is an
// inference across commands, since its MT block prints only "P7 0: (Fixed)"
// and its MR block alone prints the pair (doc.go's register, entry "THE
// COMBINED ANSWER'S P7 READ DOMAIN"). The builder's rule is the Set
// direction's own fixed value, cat.CombinedMTSetKind, which IS '0'. These
// are Set frames, so '0' here means "(Fixed)" and not "VFO".
var mtVectors = []struct {
	name     string
	slotWire string // P1, positions 3-5
	freqHz   uint32 // P2, positions 6-14
	clarHz   int16  // P3, positions 15-19 (sign then 4-digit magnitude)
	rxClar   bool   // P4, position 20
	mode     cat.Mode
	ctcss    cat.CTCSSState // P8, position 24
	shift    cat.Shift      // P10, position 27
	display  bool           // P11, position 28 — the live TAG ON/OFF flag
	p11Byte  byte           // the same position, as the byte the file carries
	tag      string         // P12, positions 29-40, trailing fill trimmed
}{
	{
		// The TAG ON half of the pair. Everything but P11 and the vector
		// name is identical to the next entry, which is the point of having
		// two: the ONLY difference on the wire is byte 28.
		name: "mt_ch001_7m100_lsb_tag_full_tagon", slotWire: "001", freqHz: 7_100_000,
		clarHz: 0, rxClar: false,
		mode: cat.Mode('1'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
		display: true, p11Byte: '1',
		// The full-width case: 12 tag bytes, no padding at all.
		tag: "FORTYMETERS1",
	},
	{
		// The TAG OFF half. The tag CHARACTERS are still there — P11 is a
		// display flag, not an erase — which is exactly the distinction a
		// P11Fixed dialect cannot express.
		name: "mt_ch001_7m100_lsb_tag_full_tagoff", slotWire: "001", freqHz: 7_100_000,
		clarHz: 0, rxClar: false,
		mode: cat.Mode('1'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
		display: false, p11Byte: '0',
		tag: "FORTYMETERS1",
	},
	{
		// Six tag bytes and six pad bytes. The LOGICAL tag is what the
		// codec's API deals in on both sides — the builder pads to width,
		// decodeCombinedTag trims back — so the six trailing spaces appear
		// here only as six bytes the build leg must reproduce. Their
		// identity is INHERITED (doc.go's register, entry "MTPolicy.TagFill";
		// mt-vectors.golden INHERITED-ASSUMED item 1), as is the fact that
		// the padding is TRAILING rather than leading; the loop below
		// asserts them against the dialect's own declared fill rather than
		// against a literal.
		name: "mt_ch002_14m250_usb_tag_short_padded", slotWire: "002", freqHz: 14_250_000,
		clarHz: 0, rxClar: false,
		mode: cat.Mode('2'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
		display: true, p11Byte: '1',
		tag: "TWENTY",
	},
	{
		// The cleared case is the all-fill field, not a distinct clear
		// encoding: the combined form documents none, which is why this
		// dialect's MTPolicy sets ClearTagByte and PadByte to zero and
		// carries TagFill alone (dialect.go; core/cat's decodeCombinedTag).
		// Note that P11 is OFF here as well, so the frame is both "no tag
		// characters" and "tag display off" — two facts, at two positions.
		name: "mt_ch003_21m200_usb_tag_cleared", slotWire: "003", freqHz: 21_200_000,
		clarHz: 0, rxClar: false,
		mode: cat.Mode('2'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
		display: false, p11Byte: '0',
		tag: "",
	},
	{
		// The PMS slot, spelt as the legend spells it: "P1L - P9U (PMS)".
		// MT's own P1 legend prints memory and PMS and nothing else, which
		// is what MTPolicy.ReadSlots carries for the read direction and
		// mtSlotValid enforces for this one.
		name: "mt_pms_p1l_3m500_lsb_tag_full_tagon", slotWire: "P1L", freqHz: 3_500_000,
		clarHz: 0, rxClar: false,
		mode: cat.Mode('1'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
		display: true, p11Byte: '1',
		tag: "PMSLOWEREDGE",
	},
	{
		// The one vector that exercises the clarifier, the CTCSS state and
		// the repeater shift together: 51.000000 MHz FM, RX clarifier ON at
		// -250 Hz, CTCSS ENC/DEC, minus shift.
		//
		// -250 Hz is a multiple of this dialect's ASSUMED 10 Hz clarifier
		// step and inside its ASSUMED 9990 Hz range, so the builder's
		// clarifier policy admits it (doc.go's register, entry
		// "ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz =
		// 9990"). The MINUS BYTE at position 15 is a second inheritance: a
		// single ASCII 0x2D, from core/cat's memory codec rather than from
		// this manual, which prints a glyph and not a byte value (register
		// entry "THE CLARIFIER'S MINUS-DIRECTION BYTE"). The codec and the
		// derivation agree here by sharing a convention, not by evidence.
		//
		// 51 MHz rather than the 145 MHz a repeater vector would suggest,
		// because the only frequency range this manual prints anywhere is
		// FA/FB's "000030000 - 056000000 (Hz)" and 145 MHz falls outside it
		// (mt-vectors.golden INHERITED-ASSUMED item 3).
		name: "mt_ch010_51m000_fm_clar_minus_ctcss_encdec_minus_shift", slotWire: "010", freqHz: 51_000_000,
		clarHz: -250, rxClar: true,
		mode: cat.Mode('4'), ctcss: cat.CTCSSEncDec, shift: cat.ShiftMinus,
		display: true, p11Byte: '1',
		tag: "FMREPEATER01",
	},
}

// TestGoldenMTCombinedSetVectors decomposes each 41-byte combined MT Set
// frame through ParseMTAnswerCombinedDisplay, checks the decoded record, tag
// and TAG flag against the literals above, rebuilds the frame with
// BuildMTSetCombinedDisplay and byte-compares it with the golden — then
// asserts the frame is admissible outbound.
//
// THE DISPLAY-BEARING PAIR IS THE ONLY PAIR THIS DIALECT HAS: under
// P11TagDisplay, core/cat's display-less BuildMTSetCombined and
// ParseMTAnswerCombined refuse outright rather than defaulting byte 28, and
// dialect_test.go's TestDifferencePinMTP11 holds both halves of that against
// the FTdx10. The vector files are Set-direction frames, and this radio's
// Set and Answer charts print an identical 41-position layout
// (mt-vectors.golden, "WHICH CHART THE LAYOUT WAS COUNTED FROM"), which is
// what makes parsing a Set frame with the Answer parser the right
// decomposition rather than a convenience.
func TestGoldenMTCombinedSetVectors(t *testing.T) {
	d := ft891.Dialect()
	fill := dialectTagFill(t, d)
	vs := loadGoldenVectors(t, "mt-vectors.golden")
	requireVectorNames(t, vs,
		"mt_ch001_7m100_lsb_tag_full_tagon",
		"mt_ch001_7m100_lsb_tag_full_tagoff",
		"mt_ch002_14m250_usb_tag_short_padded",
		"mt_ch003_21m200_usb_tag_cleared",
		"mt_pms_p1l_3m500_lsb_tag_full_tagon",
		"mt_ch010_51m000_fm_clar_minus_ctcss_encdec_minus_shift",
	)

	for i, want := range mtVectors {
		v := vs[i]
		t.Run(v.name, func(t *testing.T) {
			requireGoldenLength(t, v, 41)

			// Position 28 read off the FILE, before any codec runs: the
			// TAG ON vector's byte is '1' and the TAG OFF one's is '0', and
			// that is a claim about the golden's bytes rather than about
			// the parser's output.
			if got := v.frame[27]; got != want.p11Byte {
				t.Fatalf("P11 (position 28) of the golden is %q, want %q — the vector's own name says %v. THIS IS A STOP.",
					got, want.p11Byte, want.display)
			}
			// Position 21, likewise off the file: "(Fixed)" on this radio,
			// so every vector carries '0' and there is no TX clarifier
			// state anywhere in these frames.
			if got := v.frame[20]; got != '0' {
				t.Fatalf("P5 (position 21) of the golden is %q, want '0' — this manual prints the byte \"(Fixed)\" on every memory block. THIS IS A STOP.", got)
			}

			m, tag, display, err := d.ParseMTAnswerCombinedDisplay([]byte(v.frame))
			if err != nil {
				t.Fatalf("ParseMTAnswerCombinedDisplay(%q) refused a golden frame: %v\n"+
					"THIS IS A STOP: the derivation or the codec misreads rev 1909-C.", v.frame, err)
			}

			if got := m.Slot.Wire(); got != want.slotWire {
				t.Errorf("P1 (positions 3-5): got %q, want %q", got, want.slotWire)
			}
			if m.FreqHz != want.freqHz {
				t.Errorf("P2 (positions 6-14): got %d Hz, want %d Hz", m.FreqHz, want.freqHz)
			}
			if m.ClarHz != want.clarHz {
				t.Errorf("P3 (positions 15-19): got %d Hz, want %d Hz", m.ClarHz, want.clarHz)
			}
			if m.RxClar != want.rxClar {
				t.Errorf("P4 (position 20): got %v, want %v", m.RxClar, want.rxClar)
			}
			// P5 decodes to false on this radio whatever byte 21 held —
			// the policy is what makes it schema — so this asserts the
			// codec reports no TX clarifier state, not that the byte was
			// '0' (the file assertion above did that).
			if m.TxClar {
				t.Errorf("P5 (position 21): TxClar decoded true under %v — a fixed byte carries no state", d.MemoryP5())
			}
			if m.Mode != want.mode {
				t.Errorf("P6 (position 22): got %q (%s), want %q (%s)",
					m.Mode.Wire(), d.ModeName(m.Mode), want.mode.Wire(), d.ModeName(want.mode))
			}
			// The Set direction's fixed P7, named rather than spelt '0', so
			// the assertion says WHICH fact the byte is.
			if m.Kind != cat.CombinedMTSetKind {
				t.Errorf("P7 (position 23): got %q, want %q (the combined Set's fixed \"(Fixed)\" value)",
					m.Kind, cat.CombinedMTSetKind)
			}
			if m.CTCSS != want.ctcss {
				t.Errorf("P8 (position 24): got %q (%s), want %q (%s)",
					m.CTCSS.Wire(), m.CTCSS, want.ctcss.Wire(), want.ctcss)
			}
			if m.Shift != want.shift {
				t.Errorf("P10 (position 27): got %q (%s), want %q (%s)",
					m.Shift.Wire(), m.Shift, want.shift.Wire(), want.shift)
			}
			if display != want.display {
				t.Errorf("P11 (position 28): TAG flag decoded %v, want %v", display, want.display)
			}
			if tag != want.tag {
				t.Errorf("P12 (positions 29-40, trailing fill trimmed): got %q, want %q", tag, want.tag)
			}

			// The PAD BYTES, for the two vectors whose logical tag is
			// shorter than the field. INHERITED-ASSUMED: neither the byte
			// nor the side it sits on is printed anywhere in this manual
			// (doc.go's register, entry "MTPolicy.TagFill").
			for pos := 29 + len(want.tag); pos <= 40; pos++ {
				if got := v.frame[pos-1]; got != fill {
					t.Errorf("P12 padding at position %d: golden has %q, this dialect's declared TagFill is %q (ASSUMED — inherited from the FT-710; see doc.go's register, entry \"MTPolicy.TagFill\")",
						pos, got, fill)
				}
			}

			if t.Failed() {
				t.Fatalf("decoded record disagrees with the vector's own documented field map — THIS IS A STOP.")
			}

			built, err := d.BuildMTSetCombinedDisplay(m, tag, display)
			if err != nil {
				t.Fatalf("BuildMTSetCombinedDisplay refused the record its own parser decoded from %q: %v\n"+
					"THIS IS A STOP.", v.frame, err)
			}
			requireGoldenFrame(t, v, built, "Dialect().BuildMTSetCombinedDisplay")

			// Set-direction admissibility: these are frames this programme
			// would WRITE to a radio, so the outbound gate must admit them.
			// It re-validates the whole record through the builder's own
			// validateCombinedMTFields and byte 28 through the same
			// p11Valid the parser used, so a gate admitting less than the
			// builder emits would strand this dialect's own output — and
			// the TAG ON vector is the half that a P11Fixed reading of this
			// radio would refuse.
			if !d.AllowedCommand([]byte(v.frame)) {
				t.Errorf("AllowedCommand refused a Set-direction golden frame %q — THIS IS A STOP.", v.frame)
			}
		})
	}
}

// mtReadRequest is the MT Read frame mt-vectors.golden records IN ITS
// COMMENTS rather than as a vector: "A read of memory channel 001 is
// therefore: MT001;", counted six bytes off the block's own Read chart
// ("M T P0 P0 P0 ;").
//
// It is a comment and not a record because the file's vectors are all
// Set-direction, and because this manual contradicts itself about whether MT
// can be READ at all — the command list gives MT Set only, its own detail
// block prints a filled Read chart and a filled 41-position Answer chart
// (mt-vectors.golden's "DIRECTION SUPPORT" block; doc.go, "The MT
// contradiction"). That contradiction is the DRIVER's question, not this
// package's: what is asserted here is only that the frame this dialect
// builds for a memory-channel MT read is the frame the chart prints.
const mtReadRequest = "MT001;"

// TestGoldenMTReadRequest byte-compares BuildMTRead's output for memory
// channel 001 with the frame the MT block's Read chart prints, and asserts
// the outbound gate admits it.
//
// The 5 MHz and EMG banks are deliberately absent: MT's own slot legend
// prints neither, which is what MTPolicy.ReadSlots carries and what
// dialect_test.go's TestDifferencePinMTReadSlots pins against the FTdx10.
// They are reached by MR instead, and TestGoldenMRVectors below is where
// that is exercised against vectors.
func TestGoldenMTReadRequest(t *testing.T) {
	d := ft891.Dialect()

	slot, err := d.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot(1): %v", err)
	}
	built, err := d.BuildMTRead(slot)
	if err != nil {
		t.Fatalf("BuildMTRead(%q): %v\nTHIS IS A STOP.", slot.Wire(), err)
	}
	if got := string(built.Bytes()); got != mtReadRequest {
		t.Fatalf("GOLDEN-VS-CODEC MISMATCH — THIS IS A STOP, NOT A TEST TO ADJUST.\n"+
			"  frame     the MT block's Read chart, recorded in mt-vectors.golden's comments\n"+
			"  built by  Dialect().BuildMTRead\n"+
			"  golden    %q (%d bytes)\n"+
			"  codec     %q (%d bytes)\n"+
			"  %s",
			mtReadRequest, len(mtReadRequest), got, len(got), firstDifference(got, mtReadRequest))
	}
	if !d.AllowedCommand([]byte(mtReadRequest)) {
		t.Errorf("AllowedCommand refused the MT read request %q — THIS IS A STOP.", mtReadRequest)
	}
}

// mwVectors states, as literals, what each MW vector's bytes encode.
//
// MW HAS NO PARSER TO DECOMPOSE THROUGH. ParseMRAnswer is the only exported
// reader of the 28-byte memory frame and it checks the prefix, so it refuses
// an "MW" frame by design (asserted in the test below rather than merely
// asserted here). The decomposition is therefore done by hand, off the field
// map mw-vectors.golden documents and provenance.md "MW — MEMORY CHANNEL
// WRITE" repeats, and the round trip runs the other way: literal ->
// BuildMWSet -> byte-compare.
//
// The offsets those 1-indexed positions correspond to are core/cat's own
// memdata.go constants, which are unexported and so cannot be referenced
// from this external test package. They are reproduced here as the mapping
// the literals were read with, 0-indexed offset then manual position:
//
//	memSlotOffset      2  P1  positions 3-5    3 bytes
//	memFreqOffset      5  P2  positions 6-14   9 bytes
//	memClarSignOffset 14  P3  position 15      1 byte
//	memClarMagOffset  15  P3  positions 16-19  4 bytes
//	memRxClarOffset   19  P4  position 20      1 byte
//	memTxClarOffset   20  P5  position 21      1 byte
//	memModeOffset     21  P6  position 22      1 byte
//	memKindOffset     22  P7  position 23      1 byte
//	memCTCSSOffset    23  P8  position 24      1 byte
//	memP9Offset       24  P9  positions 25-26  2 bytes, fixed "00"
//	memShiftOffset    26  P10 position 27      1 byte
//	memTermOffset     27      position 28      ';'
//
// The three cases mirror MT cases 1, 5 and 6 — same channel, frequency,
// mode, clarifier, CTCSS and shift, with no tag, because this command has
// none (mw-vectors.golden's closing note; the MW chart stops at 28 and its
// legend has no P11 or P12).
var mwVectors = []struct {
	name         string
	slot         func(cat.Dialect) (cat.Slot, error)
	wantSlotWire string
	freqHz       uint32
	clarHz       int16
	rxClar       bool
	mode         cat.Mode
	ctcss        cat.CTCSSState
	shift        cat.Shift
}{
	{
		name: "mw_ch001_7m100_lsb",
		slot: func(d cat.Dialect) (cat.Slot, error) { return d.MemorySlot(1) }, wantSlotWire: "001",
		freqHz: 7_100_000, clarHz: 0, rxClar: false,
		mode: cat.Mode('1'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
	},
	{
		name: "mw_pms_p1l_3m500_lsb",
		slot: func(d cat.Dialect) (cat.Slot, error) { return d.PMSSlot(1, false) }, wantSlotWire: "P1L",
		freqHz: 3_500_000, clarHz: 0, rxClar: false,
		mode: cat.Mode('1'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
	},
	{
		name: "mw_ch010_51m000_fm_clar_minus_ctcss_encdec_minus_shift",
		slot: func(d cat.Dialect) (cat.Slot, error) { return d.MemorySlot(10) }, wantSlotWire: "010",
		freqHz: 51_000_000, clarHz: -250, rxClar: true,
		mode: cat.Mode('4'), ctcss: cat.CTCSSEncDec, shift: cat.ShiftMinus,
	},
}

// TestGoldenMWSetVectors builds each MW Set frame from the hand-decomposed
// record and byte-compares it with the golden, then asserts admissibility —
// and, in the same subtest, that the ONE record this radio's manual will not
// express is refused.
//
// THE REFUSAL IS HALF THE TEST. Byte 21 is "P5 0: (Fixed)" on the FT-891's
// MW block, so there is no TX clarifier flag to set: a record carrying
// TxClar true describes something the manual does not, and validateMWFields
// refuses it rather than silently encoding '0' — the validate-don't-rewrite
// posture, and the M9c-1 ruling that an omitted config semantic is REFUSED
// and never defaulted. Without this half, "the vectors carry '0' at position
// 21" would prove only that nobody asked for anything else.
// dialect_test.go's TestDifferencePinMemoryP5 carries the FTdx10
// counter-example, where the same record builds.
func TestGoldenMWSetVectors(t *testing.T) {
	d := ft891.Dialect()
	vs := loadGoldenVectors(t, "mw-vectors.golden")
	requireVectorNames(t, vs,
		"mw_ch001_7m100_lsb",
		"mw_pms_p1l_3m500_lsb",
		"mw_ch010_51m000_fm_clar_minus_ctcss_encdec_minus_shift",
	)

	for i, want := range mwVectors {
		v := vs[i]
		t.Run(v.name, func(t *testing.T) {
			requireGoldenLength(t, v, 28)

			// The 28-byte reader is MR's, and it is prefix-checked: this is
			// the assertion behind "MW has no parser to decompose through",
			// made mechanical so that a future loosening of the prefix
			// check is noticed here rather than assumed away.
			if _, err := d.ParseMRAnswer([]byte(v.frame)); err == nil {
				t.Fatalf("ParseMRAnswer accepted an MW frame %q — the prefix check has been lost", v.frame)
			}

			if got := v.frame[20]; got != '0' {
				t.Fatalf("P5 (position 21) of the golden is %q, want '0' — the MW block prints \"P5 0: (Fixed)\". THIS IS A STOP.", got)
			}
			// P7, position 23: the frame's own byte against the policy this
			// dialect declares, rather than against a value this test
			// chose. TestIdentityPinMWWriteKind is where the policy's own
			// value and its caveat are pinned.
			if got := v.frame[22]; got != d.MWWriteKind() {
				t.Fatalf("P7 (position 23) of the golden is %q, but this dialect's MWWriteKind is %q. THIS IS A STOP.", got, d.MWWriteKind())
			}

			slot, err := want.slot(d)
			if err != nil {
				t.Fatalf("building slot %q: %v", want.wantSlotWire, err)
			}
			if got := slot.Wire(); got != want.wantSlotWire {
				t.Fatalf("slot constructor produced %q, want %q", got, want.wantSlotWire)
			}
			m := cat.MemoryData{
				Slot:   slot,
				FreqHz: want.freqHz,
				ClarHz: want.clarHz,
				RxClar: want.rxClar,
				TxClar: false, // P5 is schema on this radio; see above.
				Mode:   want.mode,
				Kind:   d.MWWriteKind(),
				CTCSS:  want.ctcss,
				Shift:  want.shift,
			}

			built, err := d.BuildMWSet(m)
			if err != nil {
				t.Fatalf("BuildMWSet refused the record decomposed from %q: %v\nTHIS IS A STOP.", v.frame, err)
			}
			requireGoldenFrame(t, v, built, "Dialect().BuildMWSet")

			if !d.AllowedCommand([]byte(v.frame)) {
				t.Errorf("AllowedCommand refused a Set-direction golden frame %q — THIS IS A STOP.", v.frame)
			}

			txClar := m
			txClar.TxClar = true
			if got, err := d.BuildMWSet(txClar); err == nil {
				t.Errorf("BuildMWSet ACCEPTED a TxClar-true record and emitted %q. Under %v byte 21 is printed \"(Fixed)\" and carries no TX clarifier flag, so this record must be refused rather than silently encoded as '0'.",
					got.Bytes(), d.MemoryP5())
			}
		})
	}
}

// mrReadVectors states which slot each MR Read request names, and by which
// constructor.
//
// ALL FOUR CLASSES THE P0/1 LEGEND PRINTS ARE HERE, and the 5 MHz and EMG
// ones are the point: this radio's MR legend prints "501 - 510 (5 MHz, U.S.
// and U.K. version only)" and "EMG (Emergency)" where its MT and MW legends
// print neither (provenance.md, disagreement 5). So MR reaches them and MT
// does not — dialect_test.go's TestDifferencePinMTReadSlots holds the other
// side of that, refusing "MT501;" and "MTEMG;" at both codec and gate.
//
// The 5 MHz bounds are TRANSCRIBED here rather than assumed: this manual
// prints the actual numbers, which is why doc.go's ASSUMED register
// deliberately carries no entry for them.
var mrReadVectors = []struct {
	name         string
	slot         func(cat.Dialect) (cat.Slot, error)
	wantSlotWire string
}{
	{"mr_read_ch001_regular", func(d cat.Dialect) (cat.Slot, error) { return d.MemorySlot(1) }, "001"},
	{"mr_read_pms_p1l", func(d cat.Dialect) (cat.Slot, error) { return d.PMSSlot(1, false) }, "P1L"},
	{"mr_read_5mhz_501", func(d cat.Dialect) (cat.Slot, error) { return d.SixtyMSlot(1) }, "501"},
	{"mr_read_emg", func(d cat.Dialect) (cat.Slot, error) { return d.EMGSlot(), nil }, "EMG"},
}

// mrAnswerVectors states, as literals, what each 28-byte MR Answer carries.
//
// EVERY DATA BYTE OF AN ANSWER IS A PREDICTION: the manual prints no worked
// MR example anywhere, so these two frames are hand-derived shapes filled
// with legend-legal values, not observed replies (mr-vectors.golden
// INHERITED-ASSUMED item 1). P7 is the sharpest of them — the MR legend
// prints "0: VFO  1: Memory" but never says which a memory read answers
// with, and '1' is assumed (item 2). That byte is also what distinguishes
// an answer from the MT/MW Set frames above, where the same position is a
// fixed '0'.
var mrAnswerVectors = []struct {
	name     string
	slotWire string
	freqHz   uint32
	mode     cat.Mode
}{
	{"mr_answer_ch001_7m100_lsb_memory", "001", 7_100_000, cat.Mode('1')},
	{"mr_answer_pms_p1l_3m500_lsb_memory", "P1L", 3_500_000, cat.Mode('1')},
}

// TestGoldenMRVectors covers MR's two directions, and they are tested
// differently BECAUSE THEY ARE DIFFERENT DIRECTIONS.
//
// The READ requests are something this programme emits, so they get the
// build leg: BuildMRRead for the vector's slot, byte-compared, then admitted
// by the outbound gate.
//
// The ANSWERS are something the radio emits, and there is deliberately no
// build leg for them. MR has no Set form at all (the command list prints
// Set "X" and the block's Set value row is blank), so no builder in this
// package produces a 28-byte MR frame and there is nothing to re-encode
// with: PARSE-ONLY IS THE ANSWER DIRECTION'S TEST. The gate assertion at the
// end is the same point from the other side — an answer frame is never a
// legal outbound command.
func TestGoldenMRVectors(t *testing.T) {
	d := ft891.Dialect()
	vs := loadGoldenVectors(t, "mr-vectors.golden")
	requireVectorNames(t, vs,
		"mr_read_ch001_regular",
		"mr_read_pms_p1l",
		"mr_read_5mhz_501",
		"mr_read_emg",
		"mr_answer_ch001_7m100_lsb_memory",
		"mr_answer_pms_p1l_3m500_lsb_memory",
	)

	for i, want := range mrReadVectors {
		v := vs[i]
		t.Run(v.name, func(t *testing.T) {
			requireGoldenLength(t, v, 6)

			slot, err := want.slot(d)
			if err != nil {
				t.Fatalf("building slot %q: %v", want.wantSlotWire, err)
			}
			if got := slot.Wire(); got != want.wantSlotWire {
				t.Fatalf("slot constructor produced %q, want %q", got, want.wantSlotWire)
			}
			built, err := d.BuildMRRead(slot)
			if err != nil {
				t.Fatalf("BuildMRRead(%q): %v\nTHIS IS A STOP: the vector names a slot class this radio's MR legend prints.", slot.Wire(), err)
			}
			requireGoldenFrame(t, v, built, "Dialect().BuildMRRead")

			if !d.AllowedCommand([]byte(v.frame)) {
				t.Errorf("AllowedCommand refused the MR read request %q — THIS IS A STOP.", v.frame)
			}
		})
	}

	for i, want := range mrAnswerVectors {
		v := vs[len(mrReadVectors)+i]
		t.Run(v.name, func(t *testing.T) {
			requireGoldenLength(t, v, 28)

			if got := v.frame[20]; got != '0' {
				t.Fatalf("P5 (position 21) of the golden is %q, want '0' — the MR block prints \"P5 0: (Fixed)\" like its siblings. THIS IS A STOP.", got)
			}

			m, err := d.ParseMRAnswer([]byte(v.frame))
			if err != nil {
				t.Fatalf("ParseMRAnswer(%q) refused a golden frame: %v\n"+
					"THIS IS A STOP: the derivation or the codec misreads rev 1909-C.", v.frame, err)
			}

			if got := m.Slot.Wire(); got != want.slotWire {
				t.Errorf("P1 (positions 3-5): got %q, want %q", got, want.slotWire)
			}
			if m.FreqHz != want.freqHz {
				t.Errorf("P2 (positions 6-14): got %d Hz, want %d Hz", m.FreqHz, want.freqHz)
			}
			if m.ClarHz != 0 {
				t.Errorf("P3 (positions 15-19): got %d Hz, want 0 Hz (no clarifier offset)", m.ClarHz)
			}
			if m.RxClar {
				t.Errorf("P4 (position 20): got RxClar true, want false (clarifier off)")
			}
			if m.TxClar {
				t.Errorf("P5 (position 21): TxClar decoded true under %v — a fixed byte carries no state", d.MemoryP5())
			}
			if m.Mode != want.mode {
				t.Errorf("P6 (position 22): got %q (%s), want %q (%s)",
					m.Mode.Wire(), d.ModeName(m.Mode), want.mode.Wire(), d.ModeName(want.mode))
			}
			if got, wantName := d.ModeName(m.Mode), "LSB"; got != wantName {
				t.Errorf("P6 (position 22) mode name: got %q, want %q", got, wantName)
			}
			// The Read direction's vocabulary, and the whole point of
			// having an answer vector at all: '1' Memory, ASSUMED, not the
			// Set direction's fixed '0'.
			if m.Kind != cat.KindMemory {
				t.Errorf("P7 (position 23): got %q, want %q (Memory — ASSUMED; the legend prints the pair and never says which an answer carries)", m.Kind, cat.KindMemory)
			}
			if m.CTCSS != cat.CTCSSOff {
				t.Errorf("P8 (position 24): got %q (%s), want %q (off)", m.CTCSS.Wire(), m.CTCSS, cat.CTCSSOff.Wire())
			}
			if m.Shift != cat.ShiftSimplex {
				t.Errorf("P10 (position 27): got %q (%s), want %q (simplex)", m.Shift.Wire(), m.Shift, cat.ShiftSimplex.Wire())
			}

			if d.AllowedCommand([]byte(v.frame)) {
				t.Errorf("AllowedCommand ADMITTED the 28-byte MR answer %q. An answer frame is never a legal "+
					"outbound command, and MR has no Set direction to admit it as.", v.frame)
			}
		})
	}
}

// exVectors states each EX read vector's four-digit menu number, split into
// the address components this dialect's EXAddressPair form uses, and the
// menu-chart row the quarantined deriver recorded beside it in
// ex-vectors.golden.
//
// THE ADDRESS IS HARDCODED as the (P1, P2) pair rather than sliced out of
// the frame, so that a codec packing the two halves into the wrong positions
// would fail rather than agree with itself. P3 is 0 for every member of a
// Pair inventory — rule V12 requires it — so it is not a component the chart
// prints, and it is not written here either.
//
// The name and Digits are the second half of the test, and they bind two
// INDEPENDENT readings of one printed chart: this quarantined derivation,
// and transcription A (table2.csv), from which the dialect's inventory is
// generated. Neither agent saw the other's work. Disagreement would be a
// STOP arbitrated against the PDF, not a test to relax — the same shape
// crosscheck_test.go applies to the whole chart, narrowed here to the four
// rows the frame-geometry leg happened to choose.
var exVectors = []struct {
	name     string
	p1, p2   int
	itemName string
	digits   int
}{
	// The FIRST row of the FIRST group on the chart.
	{"ex_read_0101_agc_fast_delay_first_group", 1, 1, "AGC FAST DELAY", 4},
	// A free choice, and the one row whose parameter is a CAT rate.
	{"ex_read_0506_cat_rate", 5, 6, "CAT RATE", 1},
	// The largest Digits value anywhere in the chart, which is why this row
	// carries the Answer-shape record the next test uses.
	{"ex_read_0803_other_disp_largest_digits_5", 8, 3, "OTHER DISP", 5},
	// The LAST row of the LAST group.
	{"ex_read_1803_lcd_version_last_group", 18, 3, "LCD VERSION", 4},
}

// TestGoldenEXReadVectors builds the 7-byte EX read request for each address
// and byte-compares it with the golden, asserts admissibility, and binds the
// golden's own menu-chart annotation to this dialect's generated inventory.
//
// SEVEN BYTES, NOT NINE, is the one shared-frame length that moves on this
// radio: its EX Read chart is "E X P1 P1 P1 P1 ;" against the sibling
// family's six-digit address, which is what cat.EXAddressForm exists to
// carry (doc.go's reused-command verification; dialect_test.go's
// TestDifferencePinEXAddressForm). A Read request carries no parameter at
// all — the chart terminates at position 7 — which is also why only the READ
// frame is admissible outbound: EX Set and Answer share the read's prefix
// and address field with a longer body, and the gate refuses them by shipped
// policy (the M8d menu-write no-go), not by accident of shape.
func TestGoldenEXReadVectors(t *testing.T) {
	d := ft891.Dialect()
	vs := loadGoldenVectors(t, "ex-vectors.golden")
	requireVectorNames(t, vs,
		"ex_read_0101_agc_fast_delay_first_group",
		"ex_read_0506_cat_rate",
		"ex_read_0803_other_disp_largest_digits_5",
		"ex_read_1803_lcd_version_last_group",
	)

	items := d.EXItems()

	for i, want := range exVectors {
		v := vs[i]
		t.Run(v.name, func(t *testing.T) {
			requireGoldenLength(t, v, 7)

			addr, err := d.NewEXAddress(want.p1, want.p2, 0)
			if err != nil {
				t.Fatalf("NewEXAddress(%d,%d,0): %v\nTHIS IS A STOP: the vector names a menu-chart address "+
					"this dialect's inventory does not hold.", want.p1, want.p2, err)
			}
			built, err := d.BuildEXRead(addr)
			if err != nil {
				t.Fatalf("BuildEXRead(%s): %v\nTHIS IS A STOP.", d.EXWire(addr), err)
			}
			requireGoldenFrame(t, v, built, "Dialect().BuildEXRead")

			if !d.AllowedCommand([]byte(v.frame)) {
				t.Errorf("AllowedCommand refused the EX read request %q — THIS IS A STOP.", v.frame)
			}

			var found *cat.EXItem
			for j := range items {
				if items[j].Addr == addr {
					found = &items[j]
					break
				}
			}
			if found == nil {
				t.Fatalf("address %s is a KnownEXAddress but has no EXItem — the inventory is inconsistent", d.EXWire(addr))
			}
			if found.Name != want.itemName || found.Digits != want.digits {
				t.Errorf("menu-chart row for %s disagrees between the quarantined vector derivation and transcription A.\n"+
					"  vector file: %q, %d digits\n"+
					"  inventory:   %q, %d digits\n"+
					"THIS IS A STOP: two blind readings of one printed table disagree; arbitrate against the PDF.",
					d.EXWire(addr), want.itemName, want.digits, found.Name, found.Digits)
			}
		})
	}
}

// TestGoldenEXAnswerShape exercises the Answer shape ex-vectors.golden
// records AS A COMMENT — "pos 1-2 EX, pos 3-6 P1, pos 7..n-1 P2, pos n ';'",
// worked through for item 0803 as 6 + 5 + 1 = 12 bytes.
//
// THE BODY IS SYNTHETIC AND IS NOT EVIDENCE. The vector file deliberately
// writes no P2 value — "No P2 value is written here" — because no FT-891 has
// ever answered anything and inventing a reply would put a fabricated
// observation into a quarantined artefact. The five bytes below are this
// TEST's construction, chosen to sit inside the row's printed legend
// ("-3000 Hz - 0 - +3000 Hz (P2= -3000 - -0000 or +0000 - +3000)"), and what
// is asserted is the SHAPE the codec accepts and the address it recovers,
// never that a radio would send these bytes.
//
// THE WIDTH ITSELF RESTS ON AN ASSUMPTION the vector file names: that the
// chart's open-ended "n" equals 6 + Digits + 1, i.e. that the Digits column
// gives P2's width on the wire. The chart contradicts that reading at row
// 0905, whose Digits of 1 fights its own four-digit range (doc.go's chart
// printing defects, that row first). core/cat's ParseEXAnswer enforces no
// width policy — it bounds the frame by this dialect's widest Digits and
// returns the body VERBATIM — so nothing here is a claim about what the
// radio would send either.
func TestGoldenEXAnswerShape(t *testing.T) {
	d := ft891.Dialect()

	addr, err := d.NewEXAddress(8, 3, 0)
	if err != nil {
		t.Fatalf("NewEXAddress(8,3,0): %v", err)
	}
	synthetic := "+0000" // five bytes, this test's own; see the doc comment.
	if len(synthetic) != 5 {
		t.Fatalf("the synthetic P2 body is %d bytes, want 5 — item 0803's Digits column", len(synthetic))
	}
	frame := "EX" + d.EXWire(addr) + synthetic + ";"
	if got, want := len(frame), 12; got != want {
		t.Fatalf("the answer frame is %d bytes, want %d — the vector file works it through as 6 + 5 + 1", got, want)
	}

	gotAddr, body, err := d.ParseEXAnswer([]byte(frame))
	if err != nil {
		t.Fatalf("ParseEXAnswer(%q): %v\nTHIS IS A STOP: the recorded Answer shape is one this dialect's parser refuses.", frame, err)
	}
	if gotAddr != addr {
		t.Errorf("ParseEXAnswer recovered address %s, want %s", d.EXWire(gotAddr), d.EXWire(addr))
	}
	if body != synthetic {
		t.Errorf("ParseEXAnswer returned body %q, want %q verbatim", body, synthetic)
	}

	// The other direction of the M8d menu-write no-go: an EX frame carrying
	// a parameter is not a read, and the outbound gate admits only the
	// 7-byte read on this dialect.
	if d.AllowedCommand([]byte(frame)) {
		t.Errorf("AllowedCommand ADMITTED the EX answer %q — only the read request is an outbound EX frame.", frame)
	}
}
