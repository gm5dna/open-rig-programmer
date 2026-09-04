// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/ftdx10"
)

// This file is M9c-4 task 7b: the mechanical byte-compare of this dialect's
// codec against the ten hand-derived wire frames in testdata/*.golden.
//
// # What the vectors are, and what this file may do with them
//
// The vectors were derived at task 7a by a QUARANTINED agent that never
// opened this repository: no code, no generator, no fixture and no other
// document, only 300 dpi renders of the Yaesu FTDX10 CAT Operation Reference
// Manual rev 2308-F's position charts and their parameter legends. Every
// field width, every position boundary and every assumption that had to be
// inherited rather than read is itemised in testdata/provenance.md, which
// each test below cites by section for the values it hardcodes.
//
// THIS FILE MAY NOT MODIFY ANY VECTOR, and no failure here is ever fixed by
// editing one. The four files were frozen at commit 14e9254, whose message
// records their SHA-256s; the per-task diff gate over core/cat/ftdx10/testdata/
// enforces the freeze in git, and TestGoldenVectorsFrozen below enforces the
// same hashes in CI, so the freeze survives a rewritten history or a stray
// regeneration rather than depending on someone running the gate. A
// golden-vs-codec mismatch is a STOP for orchestrator arbitration AGAINST THE
// PDF — either the hand derivation or the codec misreads the manual — which is
// why requireGoldenFrame prints both sides, both lengths and the first
// differing wire position: the failure output is the arbitration's input.
//
// # Why the expectations are hardcoded rather than parsed out of the frame
//
// A test that parsed a frame and rebuilt it would prove the codec is
// self-consistent and nothing else: a decoder and an encoder that shared one
// wrong offset would round-trip perfectly. So every test below states the
// vector's fields as LITERALS read by hand off the golden file's own
// documented field map, and each literal carries the 1-indexed wire position
// it was read from. The parse leg then binds the codec's reading of those
// bytes to the derivation's stated intent, and the build leg binds the
// encoder to the bytes themselves.
//
// # Hardware status
//
// UNVERIFIED, for all ten vectors. Not one has been sent to, or captured
// from, a real FTDX10 — they are paper derivations, and green here means the
// codec agrees with the manual as one agent read it, not that any radio
// accepts these bytes. provenance.md names the Stage R capture that would
// lift each inherited assumption.

// goldenDir is where task 7a's frozen artefacts live, relative to this
// package's directory (go test's working directory).
const goldenDir = "testdata"

// frozenGoldenSHA256 is the freeze, transcribed from the commit message of
// 14e9254 ("M9c-4 task 7a: assumption-itemised manual-derived golden
// vectors"), whose "Frozen SHA-256s:" block records one hash per artefact.
//
// provenance.md is in here with the four vector files because it is not
// commentary about them: it is the assumption register the tests below cite
// for every value they hardcode, and a vector file whose assumptions had been
// quietly rewritten would be as corrupt as one whose bytes had.
var frozenGoldenSHA256 = map[string]string{
	"mt-vectors.golden": "f50ccc0b7f4c6549fb40d0d3130d3a21a244389832a282838f8b6adbfcbe4506",
	"mw-vectors.golden": "8724532b1b4c488a67c9b0aa11b9192ee53fe2f873c5390b2fc0701661335340",
	"mr-vectors.golden": "ef18509ab5c3ced1fcd2c9d94f3cd369d06c2f338f29b16e88127a5ce95fe5ab",
	"ex-vectors.golden": "a18f286882d900b5df4e3e3b124925128a0a7ec5f4c0687d5fccc4a1a4e2bf4c",
	"provenance.md":     "3b9754f8b0ee1e3cd6624406d2676abe982d0e58ccc018bdebe52bfaea951ef1",
}

// TestGoldenVectorsFrozen recomputes each frozen artefact's SHA-256 and
// compares it with the value commit 14e9254 recorded, so that the freeze is
// self-enforcing in CI rather than a fact recoverable only by git archaeology.
//
// The second half of the test is the one that catches the interesting case: a
// walk of testdata/*.golden requires EVERY vector file present to be covered
// by the map above. Without it, a new unfrozen vector file could be added
// beside the four and pass a test that only ever looked up names it already
// knew.
func TestGoldenVectorsFrozen(t *testing.T) {
	for name, want := range frozenGoldenSHA256 {
		path := filepath.Join(goldenDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading frozen artefact %s: %v", path, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("FREEZE BROKEN — %s has changed since commit 14e9254.\n"+
				"  recorded SHA-256 %s\n"+
				"  present  SHA-256 %s\n"+
				"This artefact is quarantined evidence: it is never regenerated and\n"+
				"never edited to satisfy a test. Restore it from the repository root\n"+
				"with `git checkout 14e9254 -- core/cat/ftdx10/testdata/%s` and report the change.",
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
		if _, ok := frozenGoldenSHA256[filepath.Base(path)]; !ok {
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
// The frame is taken VERBATIM after the single tab — never trimmed. Trailing
// SPACE is significant frame content in three of the MT vectors (the tag
// field is padded to its full 12 positions, provenance.md A1/A2), so a
// convenience TrimSpace here would silently rewrite the evidence this file
// exists to compare against. The parser instead refuses anything that is not
// exactly one tab, and refuses a CR, so a file that acquired either would
// fail loudly rather than be read approximately.
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

// requireGoldenFrame is the byte comparison, and the STOP report.
//
// builtBy names the API under test so that the failure says which of the
// codec's builders disagreed, and firstDifference converts the byte offset
// into the manual's own 1-indexed wire position — the coordinate the position
// charts and provenance.md are both written in, so that a mismatch can be
// taken straight to the chart.
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
		"The vectors are frozen (SHA-256s recorded at commit 14e9254) and this\n"+
		"test may not be made to pass by editing one. Either the quarantined hand\n"+
		"derivation or this codec misreads manual rev 2308-F; the orchestrator\n"+
		"arbitrates against the PDF.",
		v.name, v.file, v.line, builtBy, want, len(want), got, len(got), firstDifference(got, want))
}

// firstDifference describes where two frames first diverge, in 1-indexed wire
// positions.
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

// mtVectors states, as literals, what each MT vector's bytes encode.
//
// Read by hand off mt-vectors.golden's own field map (its header block, and
// provenance.md "MT — mt-vectors.golden"), which counts the 41 positions as:
//
//	1-2 "MT" | 3-5 P1 | 6-14 P2 | 15-19 P3 | 20 P4 | 21 P5 | 22 P6 | 23 P7
//	| 24 P8 | 25-26 P9 | 27 P10 | 28 P11 | 29-40 P12 | 41 ";"
//
// The three vectors are deliberately identical in every field but the channel
// and the tag (provenance.md A5), so what varies across the table is exactly
// what the tag field is being exercised for: full width, short-and-padded, and
// cleared.
var mtVectors = []struct {
	name    string
	channel int    // P1, positions 3-5
	freqHz  uint32 // P2, positions 6-14
	clarHz  int16  // P3, positions 15-19 (sign then 4-digit magnitude)
	rxClar  bool   // P4, position 20
	txClar  bool   // P5, position 21
	mode    cat.Mode
	ctcss   cat.CTCSSState // P8, position 24
	shift   cat.Shift      // P10, position 27
	tag     string         // P12, positions 29-40, trailing fill trimmed
}{
	{
		name: "mt_ch001_7m100_lsb_tag_full12", channel: 1, freqHz: 7_100_000,
		clarHz: 0, rxClar: false, txClar: false,
		mode: cat.Mode('1'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
		// The full-width case: 12 tag bytes, no padding at all.
		tag: "SCOTLAND-40M",
	},
	{
		name: "mt_ch017_7m100_lsb_tag_short_pad_space", channel: 17, freqHz: 7_100_000,
		clarHz: 0, rxClar: false, txClar: false,
		mode: cat.Mode('1'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
		// Six tag bytes and six pad bytes. The LOGICAL tag is what the
		// codec's API deals in on both sides — BuildMTSetCombined pads to
		// width, decodeCombinedTag trims back — so the six trailing spaces
		// appear here only as the six bytes the build leg must reproduce.
		// The pad byte's identity is INHERITED, not manual (provenance.md
		// A1); its position at the END of the field likewise (A2).
		tag: "GB3TST",
	},
	{
		name: "mt_ch099_7m100_lsb_tag_cleared_all_pad", channel: 99, freqHz: 7_100_000,
		clarHz: 0, rxClar: false, txClar: false,
		mode: cat.Mode('1'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
		// The cleared case is the all-fill field, not a distinct clear
		// encoding: the combined form documents none, which is why this
		// dialect's MTPolicy sets ClearTagByte and PadByte to zero and
		// carries TagFill alone (dialect.go; mtcombined.go's
		// decodeCombinedTag).
		tag: "",
	},
}

// TestGoldenMTCombinedSetVectors decomposes each 41-byte combined MT Set
// frame through ParseMTAnswerCombined, checks the decoded record and tag
// against the literals above, rebuilds the frame with BuildMTSetCombined and
// byte-compares it with the golden — then asserts the frame is admissible
// outbound.
//
// P7 IS '0' IN ALL THREE, and both directions of the codec accept that for
// their own reason. The parser's vocabulary for this form is the documented
// read pair {'0' VFO, '1' Memory}, which admits '0'; the builder's rule is
// the Set direction's own fixed value, cat.CombinedMTSetKind, which IS '0'.
// The vectors are Set frames and the manual's legend line reads
// "P7  Set: 0: (Fixed) / Read: 0: VFO  1: Memory", so '0' here means
// "(Fixed)" and not "VFO" — the coincidence CombinedMTSetKind's doc comment
// exists to keep on record without making it load-bearing.
func TestGoldenMTCombinedSetVectors(t *testing.T) {
	d := ftdx10.Dialect()
	vs := loadGoldenVectors(t, "mt-vectors.golden")
	requireVectorNames(t, vs,
		"mt_ch001_7m100_lsb_tag_full12",
		"mt_ch017_7m100_lsb_tag_short_pad_space",
		"mt_ch099_7m100_lsb_tag_cleared_all_pad",
	)

	for i, want := range mtVectors {
		v := vs[i]
		t.Run(v.name, func(t *testing.T) {
			m, tag, err := d.ParseMTAnswerCombined([]byte(v.frame))
			if err != nil {
				t.Fatalf("ParseMTAnswerCombined(%q) refused a golden frame: %v\n"+
					"THIS IS A STOP: the derivation or the codec misreads rev 2308-F.", v.frame, err)
			}

			wantSlot := fmt.Sprintf("%03d", want.channel)
			if got := m.Slot.Wire(); got != wantSlot {
				t.Errorf("P1 (positions 3-5): got %q, want %q", got, wantSlot)
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
			if m.TxClar != want.txClar {
				t.Errorf("P5 (position 21): got %v, want %v", m.TxClar, want.txClar)
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
			if tag != want.tag {
				t.Errorf("P12 (positions 29-40, trailing fill trimmed): got %q, want %q", tag, want.tag)
			}
			if t.Failed() {
				t.Fatalf("decoded record disagrees with the vector's own documented field map — THIS IS A STOP.")
			}

			built, err := d.BuildMTSetCombined(m, tag)
			if err != nil {
				t.Fatalf("BuildMTSetCombined refused the record its own parser decoded from %q: %v\n"+
					"THIS IS A STOP.", v.frame, err)
			}
			requireGoldenFrame(t, v, built, "Dialect().BuildMTSetCombined")

			// Set-direction admissibility: these are frames this programme
			// would WRITE to a radio, so the outbound gate must admit them.
			// It re-validates the whole record through the builder's own
			// validateCombinedMTFields, so a gate that admitted less than the
			// builder emits would strand this dialect's own output.
			if !d.AllowedCommand([]byte(v.frame)) {
				t.Errorf("AllowedCommand refused a Set-direction golden frame %q — THIS IS A STOP.", v.frame)
			}
		})
	}
}

// mwVectors states, as literals, what each MW vector's bytes encode.
//
// MW HAS NO PARSER TO DECOMPOSE THROUGH. ParseMRAnswer is the only exported
// reader of the 28-byte memory frame and it checks the prefix, so it refuses
// an "MW" frame by design (asserted in the test below rather than merely
// asserted here). The decomposition is therefore done by hand, off the field
// map mw-vectors.golden documents and provenance.md "MW — mw-vectors.golden"
// repeats, and the round trip runs the other way: literal -> BuildMWSet ->
// byte-compare.
//
// The offsets those 1-indexed positions correspond to are memdata.go's own
// constants, which are unexported and so cannot be referenced from this
// external test package. They are reproduced here as the mapping the literals
// were read with, 0-indexed offset then manual position:
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
// P7 is '0' in both vectors — MW's legend gives it as fixed with no Read
// vocabulary at all — and this dialect declares MWWriteKind accordingly, so
// the builder's kind check passes on the frame's own byte rather than on a
// value the test chose.
var mwVectors = []struct {
	name    string
	channel int
	freqHz  uint32
	clarHz  int16
	rxClar  bool
	txClar  bool
	mode    cat.Mode
	ctcss   cat.CTCSSState
	shift   cat.Shift
}{
	{
		// MW003014250000+000000200000; — 14.250 MHz USB, simplex, clarifier
		// off. The '+' on a zero offset is INHERITED (provenance.md B2): the
		// legend defines two direction characters and is silent on which
		// accompanies 0000. encodeMemoryFields emits '+' for any
		// non-negative value, so the codec and the derivation agree here by
		// sharing the same convention, not by evidence.
		name: "mw_ch003_14m250_usb_simplex_noclar", channel: 3, freqHz: 14_250_000,
		clarHz: 0, rxClar: false, txClar: false,
		mode: cat.Mode('2'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
	},
	{
		// MW021029600000-050010401002; — 29.600 MHz FM, minus shift, RX
		// clarifier on at -500 Hz, CTCSS ENC/DEC. The minus byte is a single
		// ASCII 0x2D, INHERITED (provenance.md B1): rev 2308-F's legend
		// renders the glyph as a double dash, but the chart allots the field
		// one position. -500 Hz is a multiple of this dialect's ASSUMED 10 Hz
		// clarifier step and inside its 9990 Hz range (dialect.go), so the
		// builder's clarifier policy admits it — B3 records that the real
		// step is unverified.
		name: "mw_ch021_29m600_fm_minusshift_rxclar_ctcss", channel: 21, freqHz: 29_600_000,
		clarHz: -500, rxClar: true, txClar: false,
		mode: cat.Mode('4'), ctcss: cat.CTCSSEncDec, shift: cat.ShiftMinus,
	},
}

// TestGoldenMWSetVectors builds each MW Set frame from the hand-decomposed
// record and byte-compares it with the golden, then asserts admissibility.
func TestGoldenMWSetVectors(t *testing.T) {
	d := ftdx10.Dialect()
	vs := loadGoldenVectors(t, "mw-vectors.golden")
	requireVectorNames(t, vs,
		"mw_ch003_14m250_usb_simplex_noclar",
		"mw_ch021_29m600_fm_minusshift_rxclar_ctcss",
	)

	for i, want := range mwVectors {
		v := vs[i]
		t.Run(v.name, func(t *testing.T) {
			// The 28-byte reader is MR's, and it is prefix-checked: this is
			// the assertion behind "MW has no parser to decompose through",
			// made mechanical so that a future loosening of the prefix check
			// is noticed here rather than assumed away.
			if _, err := d.ParseMRAnswer([]byte(v.frame)); err == nil {
				t.Fatalf("ParseMRAnswer accepted an MW frame %q — the prefix check has been lost", v.frame)
			}

			slot, err := d.MemorySlot(want.channel)
			if err != nil {
				t.Fatalf("MemorySlot(%d): %v", want.channel, err)
			}
			m := cat.MemoryData{
				Slot:   slot,
				FreqHz: want.freqHz,
				ClarHz: want.clarHz,
				RxClar: want.rxClar,
				TxClar: want.txClar,
				Mode:   want.mode,
				// P7, position 23: the frame's own byte. MW's legend fixes
				// it and this dialect's MWWriteKind is that same value.
				Kind:  cat.CombinedMTSetKind,
				CTCSS: want.ctcss,
				Shift: want.shift,
			}

			built, err := d.BuildMWSet(m)
			if err != nil {
				t.Fatalf("BuildMWSet refused the record decomposed from %q: %v\nTHIS IS A STOP.", v.frame, err)
			}
			requireGoldenFrame(t, v, built, "Dialect().BuildMWSet")

			if !d.AllowedCommand([]byte(v.frame)) {
				t.Errorf("AllowedCommand refused a Set-direction golden frame %q — THIS IS A STOP.", v.frame)
			}
		})
	}
}

// TestGoldenMRVectors covers MR's two directions, and they are tested
// differently BECAUSE THEY ARE DIFFERENT DIRECTIONS.
//
// The READ request is something this programme emits, so it gets the build
// leg: BuildMRRead for the vector's slot, byte-compared, then admitted by the
// outbound gate.
//
// The ANSWER is something the radio emits, and there is deliberately no
// build leg for it. MR has no Set form at all (the chart's Set row is printed
// empty; Control Command List, printed page 5: MR Set=X, Read=O, Ans.=O), so
// no builder in this package produces a 28-byte MR frame and there is nothing
// to re-encode with — encodeMemoryFields is core/cat-internal and is reached
// only from BuildMWSet and BuildMTSetCombined, neither of which writes an
// "MR" prefix. Re-encoding is therefore not merely unnecessary here, it would
// have to fabricate an API the protocol does not have: PARSE-ONLY IS THE
// ANSWER DIRECTION'S TEST. The gate assertion at the end is the same point
// from the other side — an answer frame is never a legal outbound command.
func TestGoldenMRVectors(t *testing.T) {
	d := ftdx10.Dialect()
	vs := loadGoldenVectors(t, "mr-vectors.golden")
	requireVectorNames(t, vs, "mr_read_ch001", "mr_answer_ch001_7m100_lsb_memory")
	read, answer := vs[0], vs[1]

	t.Run(read.name, func(t *testing.T) {
		// MR001; — 6 bytes, "MR" + P0 + ';'.
		slot, err := d.MemorySlot(1)
		if err != nil {
			t.Fatalf("MemorySlot(1): %v", err)
		}
		built, err := d.BuildMRRead(slot)
		if err != nil {
			t.Fatalf("BuildMRRead refused slot %q: %v\nTHIS IS A STOP.", slot.Wire(), err)
		}
		requireGoldenFrame(t, read, built, "Dialect().BuildMRRead")

		if !d.AllowedCommand([]byte(read.frame)) {
			t.Errorf("AllowedCommand refused the MR read request %q — THIS IS A STOP.", read.frame)
		}
	})

	t.Run(answer.name, func(t *testing.T) {
		// MR001007100000+000000110000; — 28 bytes, the same position map as
		// MW with "MR" as the opcode. Two of these fields are INHERITED
		// rather than read: that positions 3-5 carry the memory channel at
		// all, the MR legend defining P0 and then jumping to P2
		// (provenance.md C1), and that P7 is '1' Memory rather than '0' VFO
		// (C2). The P7 value is what distinguishes this answer from an
		// MT/MW Set frame, where the same position is a fixed '0'.
		m, err := d.ParseMRAnswer([]byte(answer.frame))
		if err != nil {
			t.Fatalf("ParseMRAnswer(%q) refused a golden frame: %v\n"+
				"THIS IS A STOP: the derivation or the codec misreads rev 2308-F.", answer.frame, err)
		}

		if got, want := m.Slot.Wire(), "001"; got != want {
			t.Errorf("P1 (positions 3-5): got %q, want %q", got, want)
		}
		if got, want := m.FreqHz, uint32(7_100_000); got != want {
			t.Errorf("P2 (positions 6-14): got %d Hz, want %d Hz (7.100 MHz)", got, want)
		}
		if m.ClarHz != 0 {
			t.Errorf("P3 (positions 15-19): got %d Hz, want 0 Hz (no clarifier offset)", m.ClarHz)
		}
		if m.RxClar || m.TxClar {
			t.Errorf("P4/P5 (positions 20-21): got RxClar=%v TxClar=%v, want both false (clarifier off)", m.RxClar, m.TxClar)
		}
		if got, want := m.Mode, cat.Mode('1'); got != want {
			t.Errorf("P6 (position 22): got %q (%s), want %q (LSB)", got.Wire(), d.ModeName(got), want.Wire())
		}
		if got, want := d.ModeName(m.Mode), "LSB"; got != want {
			t.Errorf("P6 (position 22) mode name: got %q, want %q", got, want)
		}
		// The Read direction's vocabulary, and the whole point of having an
		// answer vector at all: '1' Memory, not the Set direction's fixed '0'.
		if m.Kind != cat.KindMemory {
			t.Errorf("P7 (position 23): got %q, want %q (Memory — the Read-direction vocabulary)", m.Kind, cat.KindMemory)
		}
		if m.CTCSS != cat.CTCSSOff {
			t.Errorf("P8 (position 24): got %q (%s), want %q (off)", m.CTCSS.Wire(), m.CTCSS, cat.CTCSSOff.Wire())
		}
		if m.Shift != cat.ShiftSimplex {
			t.Errorf("P10 (position 27): got %q (%s), want %q (simplex)", m.Shift.Wire(), m.Shift, cat.ShiftSimplex.Wire())
		}

		if d.AllowedCommand([]byte(answer.frame)) {
			t.Errorf("AllowedCommand ADMITTED the 28-byte MR answer %q. An answer frame is never a legal "+
				"outbound command, and MR has no Set direction to admit it as.", answer.frame)
		}
	})
}

// exVectors states each EX read vector's address and the Table 2 row the
// quarantined deriver recorded beside it in ex-vectors.golden.
//
// The three-level address is hardcoded from the golden's own address table
// rather than sliced out of the frame, so that a codec that packed P1/P2/P3
// into the wrong positions would fail rather than agree with itself: the
// deriver's own report records catching exactly that error in an earlier
// draft by positional decomposition.
//
// The name and labels are the second half of the test, and they bind two
// INDEPENDENT readings of Table 2: this quarantined derivation, and
// transcription A (table2.csv), from which the dialect's inventory is
// generated. Neither agent saw the other's work. Disagreement would be a
// task-5-shaped STOP arbitrated against the PDF, not a test to relax.
var exVectors = []struct {
	name             string
	p1, p2, p3       int
	p1Label, p2Label string
	itemName         string
	digits           int
}{
	{"ex_read_01_01_11_ssb_out_level", 1, 1, 11, "RADIO SETTING", "MODE SSB", "SSB OUT LEVEL", 3},
	{"ex_read_02_02_03_cw_weight", 2, 2, 3, "CW SETTING", "KEYER", "CW WEIGHT", 2},
	{"ex_read_03_01_08_cat_rate", 3, 1, 8, "OPERATION SETTING", "GENERAL", "CAT RATE", 1},
}

// TestGoldenEXReadVectors builds the 9-byte EX read request for each address
// and byte-compares it with the golden, asserts admissibility, and binds the
// golden's own Table 2 annotation to this dialect's generated inventory.
//
// A Read request carries no P4: the chart's Read row terminates at position 9
// with ';', leaving the parameter columns empty (provenance.md "EX"). That is
// also why only the READ frame is admissible outbound — EX Set and Answer
// share the read's prefix and address field with a longer body, and the gate
// refuses them by shipped policy (the M8d menu-write no-go), not by accident
// of shape.
func TestGoldenEXReadVectors(t *testing.T) {
	d := ftdx10.Dialect()
	vs := loadGoldenVectors(t, "ex-vectors.golden")
	requireVectorNames(t, vs,
		"ex_read_01_01_11_ssb_out_level",
		"ex_read_02_02_03_cw_weight",
		"ex_read_03_01_08_cat_rate",
	)

	items := d.EXItems()

	for i, want := range exVectors {
		v := vs[i]
		t.Run(v.name, func(t *testing.T) {
			addr, err := d.NewEXAddress(want.p1, want.p2, want.p3)
			if err != nil {
				t.Fatalf("NewEXAddress(%d,%d,%d): %v\nTHIS IS A STOP: the vector names a Table 2 address "+
					"this dialect's inventory does not hold.", want.p1, want.p2, want.p3, err)
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
			if found.P1Label != want.p1Label || found.P2Label != want.p2Label || found.Name != want.itemName || found.Digits != want.digits {
				t.Errorf("Table 2 row for %s disagrees between the quarantined vector derivation and transcription A.\n"+
					"  vector file: %q / %q / %q, %d digits\n"+
					"  inventory:   %q / %q / %q, %d digits\n"+
					"THIS IS A STOP: two blind readings of one printed table disagree; arbitrate against the PDF.",
					d.EXWire(addr),
					want.p1Label, want.p2Label, want.itemName, want.digits,
					found.P1Label, found.P2Label, found.Name, found.Digits)
			}
		})
	}
}
