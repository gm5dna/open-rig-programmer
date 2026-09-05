// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/ftdx101"
)

// This file is M9d-1 task 7b: the mechanical byte-compare of this dialect's
// codec against the ten hand-derived wire frames in testdata/*.golden, and
// the D-vs-MP frame-identity proof.
//
// # What the vectors are, and what this file may do with them
//
// The vectors were derived at task 7a by a QUARANTINED agent that never
// opened this repository: no code, no generator, no fixture and no other
// document, only 400 dpi renders of the FTDX101MP/FTDX101D CAT Operation
// Reference Manual rev 2308-L's position charts and their parameter legends.
// Every field width, every position boundary and every assumption that had to
// be inherited rather than read is itemised in testdata/provenance.md, which
// each test below cites by section for the values it hardcodes.
//
// THIS FILE MAY NOT MODIFY ANY VECTOR, and no failure here is ever fixed by
// editing one. The five artefacts were frozen at commit eebf845, whose message
// records their SHA-256s; the per-task diff gate over core/cat/ftdx101/testdata/
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
// # Every clarifier field in this set is '+', deliberately
//
// The deriver could not read the minus-direction glyph unambiguously — the
// legend prints it as two hyphens, "--: Minus Shift", in all three of MR, MT
// and MW, while P3 is five positions wide and the offset alone is documented
// as four digits (provenance.md N2). Rather than guess a width, G used the
// plus direction in every vector, so NO DELIVERED BYTE DEPENDS ON THE MINUS
// READING. That is why the tables below carry '+' throughout and why no test
// here "adds coverage" for the minus case: doing so would require inventing a
// byte the manual did not yield.
//
// WHAT IS AND IS NOT COVERED ELSEWHERE. The codec's minus PATH is exercised
// against this dialect — geometry_test.go assembles frames with ClarHz -1230
// and drives the negative branch of encodeMemoryFields through them. The '-'
// byte in those frames is NOT thereby attested: it is the ASSUMED
// FT-710/FTdx10 convention, entry 7 of doc.go's ASSUMED register, and the
// geometry witness supplies the position's WIDTH rather than its value. So
// the state of the minus direction is: path exercised, SIGN BYTE UNATTESTED,
// awaiting its Stage R capture (a captured MR or MT answer for a channel with
// a negative clarifier offset, per model). The earlier wording of this comment
// called the gap "a gap in the QUARANTINED evidence, not in the dialect's test
// coverage"; that overstated, and the M9d-1 milestone review corrected it.
//
// # Hardware status
//
// UNVERIFIED, for all ten vectors. Not one has been sent to, or captured
// from, a real FTDX101D or FTDX101MP — they are paper derivations, and green
// here means the codec agrees with the manual as one agent read it, not that
// any radio accepts these bytes. provenance.md §5 names the Stage R capture
// that would lift each inherited assumption, and it names it PER MODEL: a
// capture from a D lifts nothing for an MP.

// goldenDir is where task 7a's frozen artefacts live, relative to this
// package's directory (go test's working directory).
const goldenDir = "testdata"

// frozenGoldenSHA256 is the freeze, transcribed from the commit message of
// eebf845 ("M9d-1 task 7a: assumption-itemised manual-derived golden
// vectors"), whose recorded hash block carries one hash per artefact.
//
// provenance.md is in here with the four vector files because it is not
// commentary about them: it is the assumption register the tests below cite
// for every value they hardcode, and a vector file whose assumptions had been
// quietly rewritten would be as corrupt as one whose bytes had.
var frozenGoldenSHA256 = map[string]string{
	"mt-vectors.golden": "62ab9e27978d858af8bda5b377cb06bb5b05d1b04bbd97ff034c0b0a01278629",
	"mw-vectors.golden": "4024b51c9b97da37ae865fb1b7989b26e5180c5bd1c4027bbbfc82e232ddb5a5",
	"mr-vectors.golden": "ea0582ec435a9b850d181dfc63cf88487de6d935cba18b8048e8e08a89329c0c",
	"ex-vectors.golden": "2b867bb7fe67c26f43131ee72721ec4cb18d6bfab1ce3f617e553980a47b8bc0",
	"provenance.md":     "3994de9ab3980f68e9780673467859cd379ea03698e625caf4fa68c92f5ba413",
}

// isGoldenClass reports whether a file inside testdata/ is one of the
// quarantined golden artefacts, and therefore must be covered by the freeze
// above.
//
// TWO CLASSES, not one. Any "*.golden" file is a vector file by name, and
// "provenance.md" is the assumption register those vectors are read through.
// Everything else in this package's testdata/ belongs to another task and is
// frozen by that task's own mechanism — the group ledger and transcription B
// are pinned by crosscheck_test.go's header and content checks, the geometry
// witness by geometry_test.go — so claiming them here would give this file two
// owners for one artefact rather than more safety.
func isGoldenClass(name string) bool {
	return strings.HasSuffix(name, ".golden") || name == "provenance.md"
}

// TestGoldenVectorsFrozen recomputes each frozen artefact's SHA-256 and
// compares it with the value commit eebf845 recorded, so that the freeze is
// self-enforcing in CI rather than a fact recoverable only by git archaeology.
//
// The second half of the test is the one that catches the interesting case: a
// WALK of testdata/ requires EVERY golden-class file present, at any depth, to
// be covered by the map above. Without it, a new unfrozen vector file could be
// added beside the four and pass a test that only ever looked up names it
// already knew. A walk rather than a glob because a glob sees one directory
// level: provenance.md §8 mentions a "_render/" subdirectory of intermediate
// crops (not carried into the tree), and a vector smuggled into any such
// subdirectory must fail this test rather than sit unnoticed beneath it.
func TestGoldenVectorsFrozen(t *testing.T) {
	for name, want := range frozenGoldenSHA256 {
		path := filepath.Join(goldenDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading frozen artefact %s: %v", path, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("FREEZE BROKEN — %s has changed since commit eebf845.\n"+
				"  recorded SHA-256 %s\n"+
				"  present  SHA-256 %s\n"+
				"This artefact is quarantined evidence: it is never regenerated and\n"+
				"never edited to satisfy a test. Restore it from the repository root\n"+
				"with `git checkout eebf845 -- core/cat/ftdx101/testdata/%s` and report the change.",
				path, want, got, name)
		}
	}

	var seen int
	err := filepath.WalkDir(goldenDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !isGoldenClass(d.Name()) {
			return nil
		}
		seen++
		if _, ok := frozenGoldenSHA256[d.Name()]; !ok {
			t.Errorf("%s is a golden-class artefact with no recorded SHA-256: every vector file and every "+
				"provenance register under %s must be frozen by a commit that records its hash",
				path, goldenDir)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", goldenDir, err)
	}
	if seen != len(frozenGoldenSHA256) {
		t.Errorf("walked %s and found %d golden-class artefacts, but the freeze covers %d — a frozen artefact "+
			"has been moved or a new one added", goldenDir, seen, len(frozenGoldenSHA256))
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
// SPACE is significant frame content in two of the MT vectors (the tag field
// is padded to its full 12 positions, provenance.md A1/A6), so a convenience
// TrimSpace here would silently rewrite the evidence this file exists to
// compare against. The parser instead refuses anything that is not exactly one
// tab, and refuses a CR, so a file that acquired either would fail loudly
// rather than be read approximately.
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
		"The vectors are frozen (SHA-256s recorded at commit eebf845) and this\n"+
		"test may not be made to pass by editing one. Either the quarantined hand\n"+
		"derivation or this codec misreads manual rev 2308-L; the orchestrator\n"+
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
// provenance.md §4 "mt-vectors.golden"), which counts the 41 positions as:
//
//	1-2 "MT" | 3-5 P1 | 6-14 P2 | 15-19 P3 | 20 P4 | 21 P5 | 22 P6 | 23 P7
//	| 24 P8 | 25-26 P9 | 27 P10 | 28 P11 | 29-40 P12 | 41 ";"
//
// The three vectors vary the tag field — full width, short-and-padded, and
// cleared — and also vary the channel, frequency and mode with them
// (provenance.md A4), so the table states all of those rather than only the
// tag.
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
		name: "tag_full_width_12", channel: 7, freqHz: 14_250_000,
		clarHz: 0, rxClar: false, txClar: false,
		mode: cat.Mode('2'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
		// The full-width case: 12 tag bytes, no padding at all. The payload
		// is a SYNTHETIC string invented for the fixture, not a callsign
		// (mt-vectors.golden "TAG PAYLOADS").
		tag: "SYNTHTAG0001",
	},
	{
		name: "tag_short_space_padded", channel: 12, freqHz: 7_100_000,
		clarHz: 0, rxClar: false, txClar: false,
		mode: cat.Mode('1'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
		// Seven tag bytes and five pad bytes. The LOGICAL tag is what the
		// codec's API deals in on both sides — BuildMTSetCombined pads to
		// width, decodeCombinedTag trims back — so the five trailing spaces
		// appear here only as the five bytes the build leg must reproduce.
		// The pad byte's identity and the left justification that puts it at
		// the END of the field are both INHERITED, not manual
		// (provenance.md A1).
		tag: "TESTTAG",
	},
	{
		name: "tag_cleared_all_pad", channel: 23, freqHz: 21_300_000,
		clarHz: 0, rxClar: false, txClar: false,
		mode: cat.Mode('3'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
		// The cleared case is the all-fill field, not a distinct clear
		// encoding: the combined form documents none (provenance.md A6),
		// which is why this dialect's MTPolicy sets ClearTagByte and PadByte
		// to zero and carries TagFill alone (dialect.go; mtcombined.go's
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
// The vectors are Set frames and this manual's legend line reads
// "P7  Set: 0: (Fixed) / Read: 0: VFO  1: Memory", so '0' here means
// "(Fixed)" and not "VFO" — the coincidence CombinedMTSetKind's doc comment
// exists to keep on record without making it load-bearing.
func TestGoldenMTCombinedSetVectors(t *testing.T) {
	d := ftdx101.DialectD()
	vs := loadGoldenVectors(t, "mt-vectors.golden")
	requireVectorNames(t, vs,
		"tag_full_width_12",
		"tag_short_space_padded",
		"tag_cleared_all_pad",
	)

	for i, want := range mtVectors {
		v := vs[i]
		t.Run(v.name, func(t *testing.T) {
			m, tag, err := d.ParseMTAnswerCombined([]byte(v.frame))
			if err != nil {
				t.Fatalf("ParseMTAnswerCombined(%q) refused a golden frame: %v\n"+
					"THIS IS A STOP: the derivation or the codec misreads rev 2308-L.", v.frame, err)
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
			requireGoldenFrame(t, v, built, "DialectD().BuildMTSetCombined")

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
// map mw-vectors.golden documents and provenance.md §4 repeats, and the round
// trip runs the other way: literal -> BuildMWSet -> byte-compare. The manual
// itself gives no other option: MW's Read and Answer grids are printed with
// EMPTY token rows on this radio (provenance.md N3), so there is no MW answer
// to parse even in principle.
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
// P7 is '0' in both vectors — MW's legend on this radio gives it as
// "0: (Fixed)" with no Read vocabulary at all — and this dialect declares
// MWWriteKind accordingly, so the builder's kind check passes on the frame's
// own byte rather than on a value the test chose.
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
		// MW003007074000+000000C00000; — 7.074 MHz DATA-U, simplex,
		// clarifier off. The '+' on a zero offset is INHERITED
		// (provenance.md A3): the legend defines two direction characters
		// and is silent on which accompanies 0000. encodeMemoryFields emits
		// '+' for any non-negative value, so the codec and the derivation
		// agree here by sharing the same convention, not by evidence.
		name: "write_ch003_datau_7m074_simplex", channel: 3, freqHz: 7_074_000,
		clarHz: 0, rxClar: false, txClar: false,
		mode: cat.Mode('C'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
	},
	{
		// MW045051000000+010011401002; — 51.000 MHz FM, minus repeater
		// shift, BOTH clarifiers on at +100 Hz, CTCSS ENC/DEC. +100 Hz is a
		// multiple of this dialect's ASSUMED 10 Hz clarifier step and inside
		// its 9990 Hz range (dialect.go), so the builder's clarifier policy
		// admits it — provenance.md A9 records that the real step is
		// unverified, the 9990 upper bound being the only printed hint.
		// A10 and A11 record that neither the missing shift magnitude nor
		// the missing CTCSS tone is documented anywhere in the MW frame.
		name: "write_ch045_fm_51m000_minus_shift", channel: 45, freqHz: 51_000_000,
		clarHz: 100, rxClar: true, txClar: true,
		mode: cat.Mode('4'), ctcss: cat.CTCSSEncDec, shift: cat.ShiftMinus,
	},
}

// mwMemoryData assembles one MW vector's hand-decomposed record under d.
//
// A helper rather than an inline literal because the D-vs-MP identity test
// builds the same records through the other instance: the slot must be
// constructed by the dialect under test (d.MemorySlot), so that the identity
// claim covers slot construction and not merely encoding.
func mwMemoryData(t *testing.T, d cat.Dialect, i int) cat.MemoryData {
	t.Helper()
	want := mwVectors[i]
	slot, err := d.MemorySlot(want.channel)
	if err != nil {
		t.Fatalf("MemorySlot(%d): %v", want.channel, err)
	}
	return cat.MemoryData{
		Slot:   slot,
		FreqHz: want.freqHz,
		ClarHz: want.clarHz,
		RxClar: want.rxClar,
		TxClar: want.txClar,
		Mode:   want.mode,
		// P7, position 23: the frame's own byte. MW's legend fixes it and
		// this dialect's MWWriteKind is that same value.
		Kind:  cat.CombinedMTSetKind,
		CTCSS: want.ctcss,
		Shift: want.shift,
	}
}

// TestGoldenMWSetVectors builds each MW Set frame from the hand-decomposed
// record and byte-compares it with the golden, then asserts admissibility.
func TestGoldenMWSetVectors(t *testing.T) {
	d := ftdx101.DialectD()
	vs := loadGoldenVectors(t, "mw-vectors.golden")
	requireVectorNames(t, vs,
		"write_ch003_datau_7m074_simplex",
		"write_ch045_fm_51m000_minus_shift",
	)

	for i := range mwVectors {
		v := vs[i]
		t.Run(v.name, func(t *testing.T) {
			// The 28-byte reader is MR's, and it is prefix-checked: this is
			// the assertion behind "MW has no parser to decompose through",
			// made mechanical so that a future loosening of the prefix check
			// is noticed here rather than assumed away.
			if _, err := d.ParseMRAnswer([]byte(v.frame)); err == nil {
				t.Fatalf("ParseMRAnswer accepted an MW frame %q — the prefix check has been lost", v.frame)
			}

			built, err := d.BuildMWSet(mwMemoryData(t, d, i))
			if err != nil {
				t.Fatalf("BuildMWSet refused the record decomposed from %q: %v\nTHIS IS A STOP.", v.frame, err)
			}
			requireGoldenFrame(t, v, built, "DialectD().BuildMWSet")

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
// empty; the command summary on printed page 5 marks MR Set X, Read O,
// Ans. O), so no builder in this package produces a 28-byte MR frame and
// there is nothing to re-encode with — encodeMemoryFields is core/cat-internal
// and is reached only from BuildMWSet and BuildMTSetCombined, neither of which
// writes an "MR" prefix. Re-encoding is therefore not merely unnecessary here,
// it would have to fabricate an API the protocol does not have: PARSE-ONLY IS
// THE ANSWER DIRECTION'S TEST. The gate assertion at the end is the same point
// from the other side — an answer frame is never a legal outbound command.
func TestGoldenMRVectors(t *testing.T) {
	d := ftdx101.DialectD()
	vs := loadGoldenVectors(t, "mr-vectors.golden")
	requireVectorNames(t, vs, "read_request_ch007", "answer_ch007_usb_14m250")
	read, answer := vs[0], vs[1]

	t.Run(read.name, func(t *testing.T) {
		// MR007; — 6 bytes, "MR" + P0 + ';'.
		slot, err := d.MemorySlot(7)
		if err != nil {
			t.Fatalf("MemorySlot(7): %v", err)
		}
		built, err := d.BuildMRRead(slot)
		if err != nil {
			t.Fatalf("BuildMRRead refused slot %q: %v\nTHIS IS A STOP.", slot.Wire(), err)
		}
		requireGoldenFrame(t, read, built, "DialectD().BuildMRRead")

		if !d.AllowedCommand([]byte(read.frame)) {
			t.Errorf("AllowedCommand refused the MR read request %q — THIS IS A STOP.", read.frame)
		}
	})

	t.Run(answer.name, func(t *testing.T) {
		// MR007014250000+000000210000; — 28 bytes, the same position map as
		// MW with "MR" as the opcode. Two of these fields are INHERITED
		// rather than read: that positions 3-5 carry the memory channel at
		// all, this manual's MR legend printing a P0 line and no P1 line
		// (provenance.md A7, anomaly N1), and that a real answer would echo
		// the channel that was asked for (A8). P7 is '1' Memory here, the
		// Read-direction vocabulary, which is what distinguishes this answer
		// from an MT/MW Set frame where the same position is a fixed '0'.
		m, err := d.ParseMRAnswer([]byte(answer.frame))
		if err != nil {
			t.Fatalf("ParseMRAnswer(%q) refused a golden frame: %v\n"+
				"THIS IS A STOP: the derivation or the codec misreads rev 2308-L.", answer.frame, err)
		}

		if got, want := m.Slot.Wire(), "007"; got != want {
			t.Errorf("P1 (positions 3-5): got %q, want %q", got, want)
		}
		if got, want := m.FreqHz, uint32(14_250_000); got != want {
			t.Errorf("P2 (positions 6-14): got %d Hz, want %d Hz (14.250 MHz)", got, want)
		}
		if m.ClarHz != 0 {
			t.Errorf("P3 (positions 15-19): got %d Hz, want 0 Hz (no clarifier offset)", m.ClarHz)
		}
		if m.RxClar || m.TxClar {
			t.Errorf("P4/P5 (positions 20-21): got RxClar=%v TxClar=%v, want both false (clarifier off)", m.RxClar, m.TxClar)
		}
		if got, want := m.Mode, cat.Mode('2'); got != want {
			t.Errorf("P6 (position 22): got %q (%s), want %q (USB)", got.Wire(), d.ModeName(got), want.Wire())
		}
		if got, want := d.ModeName(m.Mode), "USB"; got != want {
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
// into the wrong positions would fail rather than agree with itself.
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
	{"read_radio_ssb_tx_bpf_sel", 1, 1, 10, "RADIO SETTING", "MODE SSB", "TX BPF SEL", 1},
	{"read_cw_keyer_cw_weight", 2, 2, 5, "CW SETTING", "KEYER", "CW WEIGHT", 2},
	{"read_display_scope_ctr", 4, 2, 2, "DISPLAY SETTING", "SCOPE", "SCOPE CTR", 1},
}

// TestGoldenEXReadVectors builds the 9-byte EX read request for each address
// and byte-compares it with the golden, asserts admissibility, and binds the
// golden's own Table 2 annotation to this dialect's generated inventory.
//
// A Read request carries no P4: the chart's Read row terminates at position 9
// with ';', leaving the parameter columns empty (provenance.md A13). That is
// also why only the READ frame is admissible outbound — EX Set and Answer
// share the read's prefix and address field with a longer body, and the gate
// refuses them by shipped policy (the M8d menu-write no-go), not by accident
// of shape.
func TestGoldenEXReadVectors(t *testing.T) {
	d := ftdx101.DialectD()
	vs := loadGoldenVectors(t, "ex-vectors.golden")
	requireVectorNames(t, vs,
		"read_radio_ssb_tx_bpf_sel",
		"read_cw_keyer_cw_weight",
		"read_display_scope_ctr",
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
			requireGoldenFrame(t, v, built, "DialectD().BuildEXRead")

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

// TestGoldenFramesIdenticalAcrossModels is the D-vs-MP frame-identity proof,
// and it is the reason this package ships TWO dialect instances rather than
// one.
//
// dialect_test.go's sibling pins compare the two instances ACCESSOR BY
// ACCESSOR — same modes, same slot space, same EX inventory, same MT policy.
// That is a claim about the tables. This test is the claim one level down: the
// BYTES. Two dialects can hold identical tables and still emit different
// frames the day a builder starts consulting something else about its
// receiver, and it is the frames, not the tables, that reach a radio.
//
// THE NON-VACUITY GUARD IS FIRST. Every assertion below is "D and MP agree",
// which is trivially true of one dialect compared with itself, so the test
// begins by requiring the two to be distinguishable at all. CATID is the one
// documented difference (0681 D / 0682 MP, layout 1070-1072).
//
// AND THAT DIFFERENCE IS NOT A FRAME. ParseIDAnswer deliberately does not
// compare the answer against the receiver's own CATID — identifying the radio
// is the caller's job (id.go) — so even the ID exchange is byte-identical
// between the two instances, and the model distinction is made by a consumer
// comparing CATID() with what came back. The ID read is included in the
// representative set below to keep that on record.
func TestGoldenFramesIdenticalAcrossModels(t *testing.T) {
	dd, mp := ftdx101.DialectD(), ftdx101.DialectMP()
	if dd.CATID() == mp.CATID() {
		t.Fatalf("both instances answer CATID() = %q — with nothing to tell them apart, every identity "+
			"assertion in this test is a dialect compared with itself", dd.CATID())
	}

	// Leg 1: every SET-DIRECTION golden, built through both instances.
	//
	// These are the frames this programme would write to a radio, so they are
	// the ones where a per-model divergence would matter most. Each is
	// required to equal the golden on BOTH instances, not merely to equal the
	// other instance's output: two dialects agreeing on the same wrong bytes
	// is not the property under test.
	t.Run("set_direction_goldens", func(t *testing.T) {
		mts := loadGoldenVectors(t, "mt-vectors.golden")
		requireVectorNames(t, mts, "tag_full_width_12", "tag_short_space_padded", "tag_cleared_all_pad")
		for i, want := range mtVectors {
			v := mts[i]
			t.Run("MT/"+v.name, func(t *testing.T) {
				// The record comes from EACH instance's own parser, so the
				// decode leg is compared as well as the encode leg.
				mD, tagD, errD := dd.ParseMTAnswerCombined([]byte(v.frame))
				mMP, tagMP, errMP := mp.ParseMTAnswerCombined([]byte(v.frame))
				if (errD == nil) != (errMP == nil) {
					t.Fatalf("ParseMTAnswerCombined(%q): D err=%v, MP err=%v — one model parses a frame the "+
						"other refuses. THIS IS A STOP.", v.frame, errD, errMP)
				}
				if errD != nil {
					t.Fatalf("ParseMTAnswerCombined(%q) refused on both models: %v\nTHIS IS A STOP.", v.frame, errD)
				}
				if !reflect.DeepEqual(mD, mMP) || tagD != tagMP {
					t.Fatalf("ParseMTAnswerCombined(%q) decoded differently:\n  D  %+v tag %q\n  MP %+v tag %q\n"+
						"THIS IS A STOP.", v.frame, mD, tagD, mMP, tagMP)
				}
				if tagD != want.tag {
					t.Fatalf("decoded tag %q, want %q", tagD, want.tag)
				}

				builtD, err := dd.BuildMTSetCombined(mD, tagD)
				if err != nil {
					t.Fatalf("D BuildMTSetCombined: %v\nTHIS IS A STOP.", err)
				}
				builtMP, err := mp.BuildMTSetCombined(mMP, tagMP)
				if err != nil {
					t.Fatalf("MP BuildMTSetCombined: %v\nTHIS IS A STOP.", err)
				}
				requireGoldenFrame(t, v, builtD, "DialectD().BuildMTSetCombined")
				requireGoldenFrame(t, v, builtMP, "DialectMP().BuildMTSetCombined")

				// The gate, on both: a frame one model would send and the
				// other would refuse is the same defect as a frame they
				// build differently.
				if got, want := dd.AllowedCommand([]byte(v.frame)), mp.AllowedCommand([]byte(v.frame)); got != want {
					t.Errorf("AllowedCommand(%q): D %v, MP %v — THIS IS A STOP.", v.frame, got, want)
				}
			})
		}

		mws := loadGoldenVectors(t, "mw-vectors.golden")
		requireVectorNames(t, mws, "write_ch003_datau_7m074_simplex", "write_ch045_fm_51m000_minus_shift")
		for i := range mwVectors {
			v := mws[i]
			t.Run("MW/"+v.name, func(t *testing.T) {
				builtD, err := dd.BuildMWSet(mwMemoryData(t, dd, i))
				if err != nil {
					t.Fatalf("D BuildMWSet: %v\nTHIS IS A STOP.", err)
				}
				builtMP, err := mp.BuildMWSet(mwMemoryData(t, mp, i))
				if err != nil {
					t.Fatalf("MP BuildMWSet: %v\nTHIS IS A STOP.", err)
				}
				requireGoldenFrame(t, v, builtD, "DialectD().BuildMWSet")
				requireGoldenFrame(t, v, builtMP, "DialectMP().BuildMWSet")

				if got, want := dd.AllowedCommand([]byte(v.frame)), mp.AllowedCommand([]byte(v.frame)); got != want {
					t.Errorf("AllowedCommand(%q): D %v, MP %v — THIS IS A STOP.", v.frame, got, want)
				}
			})
		}
	})

	// Leg 2: the representative BUILD set — the four the plan names (MT Set,
	// MW Set, MT read, EX read), plus MR read and the ID read.
	//
	// MT read and EX read have no Set-direction golden to anchor them, so
	// they are compared instance-against-instance rather than against a
	// frozen byte string; the goldens above are what anchor the MT and MW
	// Set builders to the manual. Each closure takes the dialect as its
	// receiver and does its OWN slot and address construction, so what is
	// compared is the whole chain — d.MemorySlot, d.NewEXAddress, then the
	// builder — and not merely the last encode step.
	t.Run("representative_build_set", func(t *testing.T) {
		frames := []struct {
			what  string
			build func(d cat.Dialect) (cat.Command, error)
		}{
			{"MT Set (combined, 41 bytes)", func(d cat.Dialect) (cat.Command, error) {
				slot, err := d.MemorySlot(mtVectors[0].channel)
				if err != nil {
					return cat.Command{}, err
				}
				return d.BuildMTSetCombined(cat.MemoryData{
					Slot: slot, FreqHz: mtVectors[0].freqHz, ClarHz: mtVectors[0].clarHz,
					RxClar: mtVectors[0].rxClar, TxClar: mtVectors[0].txClar,
					Mode: mtVectors[0].mode, Kind: cat.CombinedMTSetKind,
					CTCSS: mtVectors[0].ctcss, Shift: mtVectors[0].shift,
				}, mtVectors[0].tag)
			}},
			{"MW Set (28 bytes)", func(d cat.Dialect) (cat.Command, error) {
				return d.BuildMWSet(mwMemoryData(t, d, 1))
			}},
			{"MT read (6 bytes)", func(d cat.Dialect) (cat.Command, error) {
				slot, err := d.MemorySlot(mtVectors[1].channel)
				if err != nil {
					return cat.Command{}, err
				}
				return d.BuildMTRead(slot)
			}},
			{"EX read (9 bytes)", func(d cat.Dialect) (cat.Command, error) {
				addr, err := d.NewEXAddress(exVectors[1].p1, exVectors[1].p2, exVectors[1].p3)
				if err != nil {
					return cat.Command{}, err
				}
				return d.BuildEXRead(addr)
			}},
			{"MR read (6 bytes)", func(d cat.Dialect) (cat.Command, error) {
				slot, err := d.MemorySlot(7)
				if err != nil {
					return cat.Command{}, err
				}
				return d.BuildMRRead(slot)
			}},
			// The ID read, included because it is the command a reader would
			// most expect to differ. It does not: the request is the fixed
			// "ID;" on both, and the DIFFERENCE between the models lives in
			// CATID(), which a caller compares against ParseIDAnswer's
			// result. Not one byte of the exchange is per-model.
			{"ID read (3 bytes)", func(d cat.Dialect) (cat.Command, error) {
				return d.BuildIDRead(), nil
			}},
		}

		for _, f := range frames {
			t.Run(f.what, func(t *testing.T) {
				builtD, errD := f.build(dd)
				builtMP, errMP := f.build(mp)
				if (errD == nil) != (errMP == nil) {
					t.Fatalf("%s: D err=%v, MP err=%v — one model builds a frame the other refuses. THIS IS A STOP.",
						f.what, errD, errMP)
				}
				if errD != nil {
					t.Fatalf("%s refused on both models: %v", f.what, errD)
				}
				gotD, gotMP := string(builtD.Bytes()), string(builtMP.Bytes())
				if gotD != gotMP {
					t.Fatalf("MODEL-DIVERGENT FRAME — THIS IS A STOP.\n"+
						"  frame  %s\n"+
						"  D      %q (%d bytes)\n"+
						"  MP     %q (%d bytes)\n"+
						"  %s\n"+
						"The FTDX101D and the FTDX101MP share one CAT manual and one printed\n"+
						"MENU Chart. That manual distinguishes them in THREE places only: the\n"+
						"ID answer's value, the P4 VALUE ranges of three MAX POWER rows, and\n"+
						"the PC command's P1 range. The latter two are not modelled here — no\n"+
						"EXItem stores P4 semantics, and M9d-1 models no PC command — so no\n"+
						"property THIS FRAME CARRIES is model-conditional except the CAT ID.\n"+
						"A per-model frame contradicts that evidence.",
						f.what, gotD, len(gotD), gotMP, len(gotMP), firstDifference(gotMP, gotD))
				}
				// Built by one model, admitted by the other: the gate is
				// consulted on every outbound frame, so an asymmetry here
				// would strand a frame the sibling instance produced.
				if got, want := dd.AllowedCommand(builtD.Bytes()), mp.AllowedCommand(builtD.Bytes()); got != want {
					t.Errorf("%s: AllowedCommand disagrees — D %v, MP %v. THIS IS A STOP.", f.what, got, want)
				}
			})
		}
	})

	// Leg 3: the PARSE direction, over the answer-direction goldens.
	//
	// Frames a radio sends, decoded by both instances. An identity proved
	// only over the frames this package BUILDS would say nothing about the
	// half of the codec that reads: a per-model narrowing in a parser would
	// make one model reject a reply the other accepted, which is a divergence
	// in exactly the same sense.
	t.Run("answer_direction_parse", func(t *testing.T) {
		mrs := loadGoldenVectors(t, "mr-vectors.golden")
		requireVectorNames(t, mrs, "read_request_ch007", "answer_ch007_usb_14m250")
		answer := mrs[1]

		mD, errD := dd.ParseMRAnswer([]byte(answer.frame))
		mMP, errMP := mp.ParseMRAnswer([]byte(answer.frame))
		if (errD == nil) != (errMP == nil) {
			t.Fatalf("ParseMRAnswer(%q): D err=%v, MP err=%v — THIS IS A STOP.", answer.frame, errD, errMP)
		}
		if errD != nil {
			t.Fatalf("ParseMRAnswer(%q) refused on both models: %v\nTHIS IS A STOP.", answer.frame, errD)
		}
		if !reflect.DeepEqual(mD, mMP) {
			t.Fatalf("ParseMRAnswer(%q) decoded differently:\n  D  %+v\n  MP %+v\nTHIS IS A STOP.", answer.frame, mD, mMP)
		}

		// The ID answer, both directions of the one documented difference.
		// Neither parser judges the ID against its own CATID, so both return
		// the same string for the same frame — including for the OTHER
		// model's ID. That is the point: the identification is the caller's.
		for _, id := range []string{"0681", "0682"} {
			frame := []byte("ID" + id + ";")
			gotD, errD := dd.ParseIDAnswer(frame)
			gotMP, errMP := mp.ParseIDAnswer(frame)
			if errD != nil || errMP != nil {
				t.Fatalf("ParseIDAnswer(%q): D err=%v, MP err=%v — both must parse structurally", frame, errD, errMP)
			}
			if gotD != id || gotMP != id {
				t.Fatalf("ParseIDAnswer(%q): D %q, MP %q, want %q from both — the parser reports what answered, "+
					"it does not filter by the receiver's own CATID", frame, gotD, gotMP, id)
			}
		}
	})
}
