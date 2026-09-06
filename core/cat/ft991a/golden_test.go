// SPDX-License-Identifier: GPL-3.0-or-later

package ft991a_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/ft991a"
)

// This file is Stage 1 task 8: the mechanical byte-compare of this dialect's
// codec against the TWENTY-NINE hand-derived wire frames in
// testdata/*.golden, and the record of the COUNTED chart geometry those
// frames were measured from.
//
// # What the vectors are, and what this file may do with them
//
// They are evidence leg G, derived by a QUARANTINED agent that never opened
// this repository: no code, no generator, no fixture and no other document,
// only 300 and 600 dpi renders (and one glyph at 1800 dpi) of the Yaesu
// FT-991A CAT Operation Reference Manual, printed revision 1711-D. Every
// field width, every position boundary and every assumption that had to be
// inherited rather than read is itemised in testdata/provenance.md and
// repeated in each vector file's own header block, which the tables below
// cite for the values they hardcode.
//
// THIS FILE MAY NOT MODIFY ANY VECTOR, and no failure here is ever fixed by
// editing one. The six artefacts were frozen at commit 8f2bad4, whose
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
// page ledger and transcription B) and names leg G's six artefacts in
// futureGoldenFreezeFiles as "the six artefacts task 8's own test freezes".
// This is that test, and TestGoldenVectorsFrozen asserts the two lists are
// exact complements over testdata/, so neither file can quietly stop
// covering an artefact the other believes it covers.
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
// # The display-LESS combined pair is the only pair this dialect has
//
// This radio's MT block prints "P11 0: (Fixed)" (ft991a_layout.txt:1015), so
// its dialect declares cat.P11Fixed and core/cat's DISPLAY-bearing pair
// (BuildMTSetCombinedDisplay, ParseMTAnswerCombinedDisplay) refuses outright
// — the exact opposite of core/cat/ft891/golden_test.go, whose radio prints
// a live TAG ON/OFF flag at the same position and whose golden file
// therefore carries a TAG OFF vector. There is no such vector here because
// the chart offers no such position; mt-vectors.golden says so in as many
// words ("No flag was invented"). TestGoldenMTCombinedSetVectors asserts the
// refusal once, so this file's choice of API is a checked fact rather than a
// convention.
//
// # Hardware status
//
// UNVERIFIED, for all twenty-nine vectors, and there is no route to
// verifying them: no FT-991A has ever been asked anything by this project
// (doc.go's provenance section). Green here means the codec agrees with the
// manual as one agent read it, not that any radio accepts these bytes.
// doc.go's ASSUMED register names the capture that would lift each inherited
// assumption, and this file cites those entries BY NAME wherever a vector's
// bytes rest on one.

// goldenDir is where evidence leg G lives, relative to this package's
// directory (go test's working directory).
const goldenDir = "testdata"

// frozenVectorSHA256 is the freeze, transcribed from the commit message of
// 8f2bad4 ("core/cat/ft991a: import the quarantined evidence legs — L
// ledger, B transcription, G geometry"), whose "SHA-256 of every imported
// file:" block records one hash per artefact.
//
// provenance.md is in here with the five vector files because it is not
// commentary about them: it is the assumption register the tests below cite
// for every value they hardcode, and a vector file whose assumptions had
// been quietly rewritten would be as corrupt as one whose bytes had.
var frozenVectorSHA256 = map[string]string{
	"mt-vectors.golden": "695eb193df437c829d768b310aa7bc675e12dedd9a40f4d5352e6c768a02bb4b",
	"mw-vectors.golden": "7933bb03f251fc45c6bffae907e538c557fee50fc4289c44c768d18febc864fd",
	"mr-vectors.golden": "8c214e96c5e72a6d75cbb28dddca7ac7005d36dd290377bf702bf0e473679d08",
	"mc-vectors.golden": "ceb6dd7d5bf0936232558fbba39a1a2071067463a04115e4626df79651683b49",
	"ex-vectors.golden": "bc673380769bd6e49670904a8a8f9a710a671c2d6d6a07f6710dc5fcdbf6df90",
	"provenance.md":     "9f001c2f66d5ec8fe38cabcd1a34a1c26158741158772d6e9a72526f8f2e79e8",
}

// TestGoldenVectorsFrozen recomputes each frozen artefact's SHA-256 and
// compares it with the value commit 8f2bad4 recorded, so that the freeze is
// self-enforcing in CI rather than a fact recoverable only by git
// archaeology.
//
// THE CONVERSE LEG GLOBS THE WHOLE OF testdata/, not just *.golden, and it
// is the one that catches the interesting case: a walk of testdata/*
// requires EVERY file present to be covered by this map OR by
// crosscheck_test.go's frozenEvidenceSHA256. Without it a new unfrozen
// artefact could be added beside the ten and pass a test that only ever
// looked up names it already knew — which is the widening the task-6 file's
// own fix round made there, made here for the same reason.
//
// The third leg binds the two files together: crosscheck_test.go declares
// futureGoldenFreezeFiles, the six artefacts it deliberately does NOT hash
// because they are this task's, and that list is asserted to be exactly this
// map's key set. Without it, a file dropped from this map and left in that
// one would be covered by neither test while both still passed their own
// converse leg.
func TestGoldenVectorsFrozen(t *testing.T) {
	for name, want := range frozenVectorSHA256 {
		path := filepath.Join(goldenDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading frozen artefact %s: %v", path, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("FREEZE BROKEN — %s has changed since commit 8f2bad4.\n"+
				"  recorded SHA-256 %s\n"+
				"  present  SHA-256 %s\n"+
				"This artefact is quarantined evidence: it is never regenerated and\n"+
				"never edited to satisfy a test. Restore it from the repository root\n"+
				"with `git checkout 8f2bad4 -- core/cat/ft991a/testdata/%s` and report\n"+
				"the change.",
				path, want, got, name)
		}
	}

	present, err := filepath.Glob(filepath.Join(goldenDir, "*"))
	if err != nil {
		t.Fatalf("globbing %s: %v", goldenDir, err)
	}
	if len(present) == 0 {
		t.Fatalf("no files found in %s — the quarantined evidence is missing", goldenDir)
	}
	for _, path := range present {
		name := filepath.Base(path)
		if _, ok := frozenVectorSHA256[name]; ok {
			continue
		}
		if _, ok := frozenEvidenceSHA256[name]; ok {
			continue
		}
		t.Errorf("%s is present under %s with no recorded SHA-256: every file there must be frozen either by this file's frozenVectorSHA256 (evidence leg G) or by crosscheck_test.go's frozenEvidenceSHA256 (legs L and B)",
			path, goldenDir)
	}

	for name := range futureGoldenFreezeFiles {
		if _, ok := frozenVectorSHA256[name]; !ok {
			t.Errorf("crosscheck_test.go's futureGoldenFreezeFiles names %q as an artefact this file freezes, and this file does not: it is now frozen by neither test", name)
		}
	}
	for name := range frozenVectorSHA256 {
		if !futureGoldenFreezeFiles[name] {
			t.Errorf("this file freezes %q but crosscheck_test.go's futureGoldenFreezeFiles does not name it, so that file's own converse leg would report it as unfrozen", name)
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
//
// The five counts it enforces are the plan's own — MT 9, MW 6, MR 6 (four
// reads and two answers), MC 4, EX 4, twenty-nine in all.
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
// for every chart, and every one agreed on the first pass), so that a length
// disagreement is reported as itself rather than as a byte difference at the
// first position that happens to shift.
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
		"The vectors are frozen (SHA-256s recorded at commit 8f2bad4) and this\n"+
		"test may not be made to pass by editing one. Either the quarantined hand\n"+
		"derivation or this codec misreads manual revision 1711-D; the orchestrator\n"+
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

// dialectTagFill asks the dialect what it pads a short combined-MT tag field
// with, by building a one-character tag and reading the last position of the
// field — the byte before the terminator.
//
// ASKED, NOT SPELT. MTPolicy.TagFill is unexported and this is an external
// test package, but that is the lesser reason: the fill byte is an ASSUMED
// entry on this dialect's register (doc.go, entry "MTPolicy.TagFill = ' '"
// — the P12 legend naming a width and an alphabet and no fill), and a test
// that wrote ' ' here would be asserting the assumption against itself.
// Asking the dialect makes the padding assertions below say what they mean:
// the golden's pad bytes are whatever THIS DIALECT declares, and if the
// lifting capture replaces the declaration the assertion moves with it — at
// which point the goldens themselves become the STOP, which is the correct
// outcome.
//
// The display-LESS builder, because this dialect is cat.P11Fixed.
func dialectTagFill(t *testing.T, d cat.Dialect) byte {
	t.Helper()
	slot, err := d.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot(1): %v", err)
	}
	probe, err := d.BuildMTSetCombined(cat.MemoryData{
		Slot: slot, FreqHz: 7_100_000, Mode: cat.Mode('1'),
		Kind: cat.CombinedMTSetKind, CTCSS: cat.CTCSSOff, Shift: cat.ShiftSimplex,
	}, "A")
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
// provenance.md "MT — mt-vectors.golden"), which counts the 41 positions as:
//
//	1-2 "MT" | 3-5 P1 | 6-14 P2 | 15 "+/-" | 16-19 P3 | 20 P4 | 21 P5
//	| 22 P6 | 23 P7 | 24 P8 | 25-26 P9 | 27 P10 | 28 P11 | 29-40 P12 | 41 ";"
//
// TWO POSITIONS ARE THIS RADIO'S OWN READING and are why this dialect exists
// as a separate one at all:
//
//   - P11 (position 28) is printed "0: (Fixed)" (ft991a_layout.txt:1015), so
//     it is SCHEMA, not the TAG ON/OFF flag the FT-891 prints there. All
//     nine vectors carry '0', and p11Valid admits nothing else under
//     cat.P11Fixed. Pinned against the FT-891 by dialect_test.go's
//     TestDifferencePinMTP11.
//   - P5 (position 21) is printed `0: TX CLAR "OFF" 1: TX CLAR "ON"`
//     (ft991a_layout.txt:1004), so it is a LIVE FLAG where the FT-891 prints
//     "(Fixed)". Pinned by TestDifferencePinMemoryP5.
//
// EVERY VECTOR CARRIES '0' AT POSITION 21, so the goldens exercise only the
// TX-clarifier-off half of a byte this radio makes live. That is an evidence
// gap, not a codec fact, and it is recorded rather than papered over:
// TestGoldenMWSetVectors builds the TxClar-true counterpart of an MW vector
// and asserts it differs from the golden at position 21 and nowhere else, so
// the half the derivation did not write is at least held to the position the
// chart gives it.
//
// P8 (position 24) is where this file's vector count exceeds its siblings':
// this manual's legend prints FIVE states, `0: CTCSS "OFF" 1: CTCSS ENC/DEC
// 2: CTCSS ENC 3: DCS ENC/DEC 4: DCS ENC` (1010-1011), and the derivation
// wrote one vector per state above the off case. The DCS pair is the reason
// cat.ToneStatesCTCSSAndDCS exists; whether the radio ACCEPTS a DCS state
// written without a CN code first is doc.go's register entry "THE DCS
// STATES' SET ACCEPTANCE", and nothing here claims it does.
//
// P7 (position 23) is '0' in all nine, the Set direction's own fixed value:
// this radio's MT legend prints "P7 Set: 0: (Fixed) / Read: 0: VFO
// 1: Memory" (1009), so on a Set frame '0' means "(Fixed)" and not "VFO".
// cat.CombinedMTSetKind IS that byte.
var mtVectors = []struct {
	name      string
	slotWire  string // P1, positions 3-5
	slotIsPMS bool   // ...and which side of the MC legend's 001-099 / 100-117 split it falls
	freqHz    uint32 // P2, positions 6-14
	clarHz    int16  // P3, positions 15-19 (sign then 4-digit magnitude)
	rxClar    bool   // P4, position 20
	mode      cat.Mode
	ctcss     cat.CTCSSState // P8, position 24
	shift     cat.Shift      // P10, position 27
	tag       string         // P12, positions 29-40, trailing fill trimmed
}{
	{
		// The full-width case: 12 tag bytes, no padding at all.
		name: "mt_ch001_7m100_lsb_tag_full_12char", slotWire: "001", freqHz: 7_100_000,
		clarHz: 0, rxClar: false,
		mode: cat.Mode('1'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
		tag: "HIGHLANDNET1",
	},
	{
		// Four tag bytes and eight pad bytes. The LOGICAL tag is what the
		// codec's API deals in on both sides — the builder pads to width,
		// decodeCombinedTag trims back — so the eight trailing spaces appear
		// here only as eight bytes the build leg must reproduce. Their
		// identity is INHERITED-ASSUMED (doc.go's register, entry
		// "MTPolicy.TagFill = ' '"; mt-vectors.golden INHERITED-ASSUMED item
		// 1), as is the fact that the padding is TRAILING rather than
		// leading; the loop below asserts them against the dialect's own
		// declared fill rather than against a literal.
		name: "mt_ch002_14m250_usb_tag_short_padded", slotWire: "002", freqHz: 14_250_000,
		clarHz: 0, rxClar: false,
		mode: cat.Mode('2'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
		tag: "CLUB",
	},
	{
		// The cleared case is the all-fill field, not a distinct clear
		// encoding: the combined form documents none, which is why this
		// dialect's MTPolicy sets ClearTagByte and PadByte to zero and
		// carries TagFill alone (dialect.go; core/cat's decodeCombinedTag).
		// Note that there is no second fact to state here, unlike the
		// FT-891's equivalent vector: this chart has no TAG display flag to
		// be off as well.
		name: "mt_ch003_18m100_usb_tag_cleared_all_pad", slotWire: "003", freqHz: 18_100_000,
		clarHz: 0, rxClar: false,
		mode: cat.Mode('2'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
		tag: "",
	},
	{
		// THE PMS SLOT, SPELT AS A NUMBER. Its vector name says "p_1l", the
		// MC legend's own display spelling of channel 100 ("100: P-1L",
		// ft991a_layout.txt:916) — but the WIRE carries "100", because on
		// this radio the PMS pairs are decimal channel numbers continuing
		// the memory range and no token form reaches the wire at all. That
		// is cat.PMSFormNumeric, and dialect_test.go's
		// TestDifferencePinPMSFormAndSlotSpace holds it against the FTdx10,
		// which builds "P1L" for the same pair. This vector is the
		// hand-derived evidence under it.
		name: "mt_ch100_p_1l_pms_slot_7m000_lsb", slotWire: "100", slotIsPMS: true, freqHz: 7_000_000,
		clarHz: 0, rxClar: false,
		mode: cat.Mode('1'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
		tag: "BANDEDGELOW1",
	},
	{
		// The one vector that exercises the clarifier, a tone state and the
		// repeater shift together: 145.500000 MHz FM, RX clarifier ON at
		// -500 Hz, CTCSS ENC/DEC, minus shift.
		//
		// -500 Hz is a multiple of this dialect's ASSUMED 10 Hz clarifier
		// step and inside its ASSUMED 9990 Hz ceiling, so the builder's
		// clarifier policy admits it (doc.go's register, entry
		// "ClarifierPolicy.StepHz = 10 AND ClarifierPolicy.MaxAbsHz =
		// 9990"). The MINUS BYTE at position 15 is a second inheritance and
		// a sharper one than the FT-891's: this manual prints the glyph as
		// TWO hyphens, "--: Minus Shift", verified at 1800 dpi with a gap
		// between them, against a P3 field the chart gives exactly five
		// positions of which four are the offset digits (mt-vectors.golden
		// INHERITED-ASSUMED item 2; provenance.md disagreement 1). The
		// single ASCII 0x2D the vector carries is core/cat's memory codec's
		// convention (register entry "THE CLARIFIER'S MINUS-DIRECTION BYTE,
		// the ASCII HYPHEN-MINUS 0x2D ('-')"), and the codec and the
		// derivation agree here by sharing it, not by evidence.
		//
		// 145.5 MHz rather than the FT-891 vector's 51 MHz because this
		// radio has the band: FA/FB print "000030000 - 470000000 (Hz)".
		name: "mt_ch010_145m500_fm_clar_minus_0500_ctcss_encdec_minus_shift", slotWire: "010", freqHz: 145_500_000,
		clarHz: -500, rxClar: true,
		mode: cat.Mode('4'), ctcss: cat.CTCSSEncDec, shift: cat.ShiftMinus,
		tag: "GLENREPEATER",
	},
	// The four P8 states above "off", one vector each, identical in every
	// other position: the ONLY difference between consecutive frames here is
	// byte 24, which is the point of having four.
	{
		name: "mt_ch011_145m100_fm_p8_1_ctcss_encdec", slotWire: "011", freqHz: 145_100_000,
		clarHz: 0, rxClar: false,
		mode: cat.Mode('4'), ctcss: cat.CTCSSEncDec, shift: cat.ShiftSimplex,
		tag: "TONEENCDEC01",
	},
	{
		name: "mt_ch012_145m100_fm_p8_2_ctcss_enc", slotWire: "012", freqHz: 145_100_000,
		clarHz: 0, rxClar: false,
		mode: cat.Mode('4'), ctcss: cat.CTCSSEnc, shift: cat.ShiftSimplex,
		tag: "TONEENCONLY2",
	},
	{
		name: "mt_ch013_145m100_fm_p8_3_dcs_encdec", slotWire: "013", freqHz: 145_100_000,
		clarHz: 0, rxClar: false,
		mode: cat.Mode('4'), ctcss: cat.CTCSSDCSEncDec, shift: cat.ShiftSimplex,
		tag: "DCSENCDEC003",
	},
	{
		name: "mt_ch014_145m100_fm_p8_4_dcs_enc", slotWire: "014", freqHz: 145_100_000,
		clarHz: 0, rxClar: false,
		mode: cat.Mode('4'), ctcss: cat.CTCSSDCSEnc, shift: cat.ShiftSimplex,
		tag: "DCSENCONLY04",
	},
}

// TestGoldenMTCombinedSetVectors decomposes each 41-byte combined MT Set
// frame through ParseMTAnswerCombined, checks the decoded record and tag
// against the literals above, rebuilds the frame with BuildMTSetCombined and
// byte-compares it with the golden — then asserts the frame is admissible
// outbound.
//
// THE DISPLAY-LESS PAIR IS THE ONLY PAIR THIS DIALECT HAS, and the check
// above the loop proves the parse half of it rather than assuming it: under
// cat.P11Fixed, ParseMTAnswerCombinedDisplay refuses outright. The build
// half — that BuildMTSetCombinedDisplay refuses too — is pinned separately
// by dialect_test.go's TestDifferencePinMTP11 (its BuildMTSetCombinedDisplay
// assertion), not here.
// dialect_test.go's TestDifferencePinMTP11 holds both halves of that against
// the FT-891, where the refusals run the other way.
//
// The vector files are Set-direction frames, and this radio's Set and Answer
// charts print an identical 41-position layout (mt-vectors.golden, "Chart
// used"), which is what makes parsing a Set frame with the Answer parser the
// right decomposition rather than a convenience.
func TestGoldenMTCombinedSetVectors(t *testing.T) {
	d := ft991a.Dialect()
	fill := dialectTagFill(t, d)
	vs := loadGoldenVectors(t, "mt-vectors.golden")
	requireVectorNames(t, vs,
		"mt_ch001_7m100_lsb_tag_full_12char",
		"mt_ch002_14m250_usb_tag_short_padded",
		"mt_ch003_18m100_usb_tag_cleared_all_pad",
		"mt_ch100_p_1l_pms_slot_7m000_lsb",
		"mt_ch010_145m500_fm_clar_minus_0500_ctcss_encdec_minus_shift",
		"mt_ch011_145m100_fm_p8_1_ctcss_encdec",
		"mt_ch012_145m100_fm_p8_2_ctcss_enc",
		"mt_ch013_145m100_fm_p8_3_dcs_encdec",
		"mt_ch014_145m100_fm_p8_4_dcs_enc",
	)

	// The display-bearing pair, refused once on a real golden frame rather
	// than on a synthetic one, so the refusal is bound to the bytes this file
	// actually works with.
	if _, _, _, err := d.ParseMTAnswerCombinedDisplay([]byte(vs[0].frame)); err == nil {
		t.Errorf("ParseMTAnswerCombinedDisplay ACCEPTED a golden frame under %v — byte 28 is printed \"(Fixed)\" on this radio and carries no TAG flag to report", d.MTP11())
	}

	for i, want := range mtVectors {
		v := vs[i]
		t.Run(v.name, func(t *testing.T) {
			requireGoldenLength(t, v, 41)

			// Positions read off the FILE, before any codec runs: these are
			// claims about the golden's bytes rather than about the parser's
			// output.
			if got := v.frame[27]; got != '0' {
				t.Fatalf("P11 (position 28) of the golden is %q, want '0' — this manual prints the byte \"(Fixed)\" (ft991a_layout.txt:1015). THIS IS A STOP.", got)
			}
			if got := v.frame[22]; got != cat.CombinedMTSetKind {
				t.Fatalf("P7 (position 23) of the golden is %q, want %q — the Set direction's own \"(Fixed)\" value. THIS IS A STOP.", got, cat.CombinedMTSetKind)
			}
			if got := v.frame[20]; got != '0' {
				t.Fatalf("P5 (position 21) of the golden is %q, want '0' — every G vector carries the TX-clarifier-off byte; see this table's own note on the evidence gap. THIS IS A STOP.", got)
			}
			if got := v.frame[24:26]; got != "00" {
				t.Fatalf("P9 (positions 25-26) of the golden is %q, want \"00\" — the legend prints \"P9 00: (Fixed)\". THIS IS A STOP.", got)
			}
			// P3's sign byte (position 15): read off the file, uniformly
			// with the other INHERITED-ASSUMED positions above, rather than
			// bound only through ClarHz and the rebuild.
			if got := v.frame[14]; got != '+' && got != '-' {
				t.Fatalf("P3 (position 15) of the golden is %q, want '+' or '-' — this dialect's memory codec's clarifier sign byte. THIS IS A STOP.", got)
			}

			m, tag, err := d.ParseMTAnswerCombined([]byte(v.frame))
			if err != nil {
				t.Fatalf("ParseMTAnswerCombined(%q) refused a golden frame: %v\n"+
					"THIS IS A STOP: the derivation or the codec misreads revision 1711-D.", v.frame, err)
			}

			if got := m.Slot.Wire(); got != want.slotWire {
				t.Errorf("P1 (positions 3-5): got %q, want %q", got, want.slotWire)
			}
			// WHICH SIDE OF THE MC LEGEND'S SPLIT the slot falls on, asked
			// of the slot this dialect decoded: 001-099 regular, 100-117
			// PMS (ft991a_layout.txt:915-916). Without it the PMS vector
			// would prove only that three digits survive a round trip.
			if got := m.Slot.IsPMS(); got != want.slotIsPMS {
				t.Errorf("P1 (positions 3-5): slot %q decoded IsPMS()=%v, want %v — the MC legend splits 001-099 from 100-117", m.Slot.Wire(), got, want.slotIsPMS)
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
			// P5 is LIVE on this radio, so this asserts the decoded STATE
			// and not merely that a schema byte was ignored — the opposite
			// of the FT-891's equivalent assertion.
			if m.TxClar {
				t.Errorf("P5 (position 21): TxClar decoded true under %v from a golden whose byte 21 is '0'", d.MemoryP5())
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

			// The PAD BYTES, for the two vectors whose logical tag is
			// shorter than the field. INHERITED-ASSUMED: neither the byte
			// nor the side it sits on is printed anywhere in this manual
			// (doc.go's register, entry "MTPolicy.TagFill = ' '").
			for pos := 29 + len(want.tag); pos <= 40; pos++ {
				if got := v.frame[pos-1]; got != fill {
					t.Errorf("P12 padding at position %d: golden has %q, this dialect's declared TagFill is %q (ASSUMED; see doc.go's register, entry \"MTPolicy.TagFill = ' '\")",
						pos, got, fill)
				}
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
			// validateCombinedMTFields and byte 28 through the same p11Valid
			// the parser used, so a gate admitting less than the builder
			// emits would strand this dialect's own output — and the two DCS
			// vectors are the half a ToneStatesCTCSS reading of this radio
			// would refuse.
			if !d.AllowedCommand([]byte(v.frame)) {
				t.Errorf("AllowedCommand refused a Set-direction golden frame %q — THIS IS A STOP.", v.frame)
			}
		})
	}
}

// mtReadRequest is the MT Read frame mt-vectors.golden records IN ITS
// COMMENTS rather than as a vector: "For memory channel 001 the Read request
// is: MT001;", counted six bytes off the block's own Read chart
// ("M T P0 P0 P0 ;", ft991a_layout.txt:1018).
//
// It is a comment and not a record because the file's vectors are all
// Set-direction. Unlike the FT-891, this manual does NOT contradict itself
// about whether MT can be read: the command list prints "MT | MEMORY CHANNEL
// WRITE/TAG | Set: O | Read: O | Ans.: O | AI: X" and the detail block
// prints all three charts filled, which mt-vectors.golden's
// "COMMAND-LIST vs DETAIL-BLOCK" section records as an agreement. That is
// why this dialect declares cat.MTReadsReadable rather than carrying an
// FT-891-shaped caveat.
const mtReadRequest = "MT001;"

// TestGoldenMTReadRequest byte-compares BuildMTRead's output for memory
// channel 001 with the frame the MT block's Read chart prints, and asserts
// the outbound gate admits it.
//
// The read domain is the block's own span: this radio's MT legend prints
// "P0/1 001-117 (Memory Channel)" (999) and nothing else, so MTReadsReadable
// and MTReadsMemoryPMS give identical verdicts here — there is no 5 MHz or
// EMG bank for them to differ over. dialect_test.go's
// TestDegeneracyPinWideAndNarrowAgree is where that coincidence is made a
// checked fact; this test asserts only the frame.
func TestGoldenMTReadRequest(t *testing.T) {
	d := ft991a.Dialect()

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

	// mtReadRequest is re-typed from the golden's COMMENT block, which
	// loadGoldenVectors skips — the freeze above does not reach it. Assert
	// it verbatim against the frozen file's bytes, so an edit to this const
	// is caught by the same mechanism that protects every other byte here.
	raw, err := os.ReadFile(filepath.Join(goldenDir, "mt-vectors.golden"))
	if err != nil {
		t.Fatalf("reading mt-vectors.golden: %v", err)
	}
	if !strings.Contains(string(raw), mtReadRequest) {
		t.Errorf("mtReadRequest %q does not appear verbatim in mt-vectors.golden — the const has drifted from the frozen comment it was transcribed from", mtReadRequest)
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
// MW'S P7 IS A PRINTED WIDTH DEFECT, and it is the reason this radio's MW
// vectors carry the byte they do: the legend prints "P7 00: (Fixed)" — two
// characters — against a chart that gives P7 position 23 alone
// (ft991a_layout.txt:1047; mw-vectors.golden INHERITED-ASSUMED item 1;
// provenance.md disagreement 2). A two-character P7 cannot fit the counted
// 28-byte frame, so the derivation wrote one '0' and recorded the defect.
// This dialect's MWWriteKind is cat.CombinedMTSetKind, which IS that byte,
// and TestIdentityPinMWWriteKind says exactly that much and no more; the
// loop below asks the dialect for the value rather than spelling it.
//
// The six cases mirror MT vectors 1 and 5-9 (the tag-carrying positions do
// not exist here: the MW chart stops at 28 and its legend has no P11 or
// P12). mw-vectors.golden's own trailing comment says "three cases" mirror
// MT 1, 5 and 6 — stale against the file's actual six vectors; recorded
// here, not fixed there, because the file is frozen.
//
// THERE IS NO PMS MW VECTOR, unlike the FT-891's file — this radio's MW
// legend prints only "P1 001-117 (Memory Channel)" and never decomposes the
// span (mw-vectors.golden's "SLOT-RANGE LEGEND" note), so the derivation
// declined to spell a PMS slot under a command whose own legend does not.
// That evidence gap is acceptable; what is NOT is treating the MT ch100
// vector as covering it — MT and MW do NOT share a slot predicate (MT's
// combined Set gates on d.mtSlotValid, mtcombined.go:110-111; MW gates on
// d.writableSlot, mw.go:81-82; mtcombined.go:90-94 says the split is
// deliberate and the two rules are free to diverge), so the MT ch100 vector
// exercises mtSlotValid(PMS) and cannot stand in for writableSlot(PMS).
// TestGoldenMWSetVectors therefore also builds a PMS-slot MW frame — a
// constructed record, not a golden vector, exactly as its TxClar
// counterpart is — so the write gate's PMS arm is exercised by this package
// at least once.
var mwVectors = []struct {
	name    string
	channel int // P1, positions 3-5, through MemorySlot
	freqHz  uint32
	clarHz  int16
	rxClar  bool
	mode    cat.Mode
	ctcss   cat.CTCSSState
	shift   cat.Shift
}{
	{
		name: "mw_ch001_7m100_lsb_plain", channel: 1,
		freqHz: 7_100_000, clarHz: 0, rxClar: false,
		mode: cat.Mode('1'), ctcss: cat.CTCSSOff, shift: cat.ShiftSimplex,
	},
	{
		name: "mw_ch010_145m500_fm_clar_minus_0500_ctcss_encdec_minus_shift", channel: 10,
		freqHz: 145_500_000, clarHz: -500, rxClar: true,
		mode: cat.Mode('4'), ctcss: cat.CTCSSEncDec, shift: cat.ShiftMinus,
	},
	{
		name: "mw_ch011_145m100_fm_p8_1_ctcss_encdec", channel: 11,
		freqHz: 145_100_000, clarHz: 0, rxClar: false,
		mode: cat.Mode('4'), ctcss: cat.CTCSSEncDec, shift: cat.ShiftSimplex,
	},
	{
		name: "mw_ch012_145m100_fm_p8_2_ctcss_enc", channel: 12,
		freqHz: 145_100_000, clarHz: 0, rxClar: false,
		mode: cat.Mode('4'), ctcss: cat.CTCSSEnc, shift: cat.ShiftSimplex,
	},
	{
		name: "mw_ch013_145m100_fm_p8_3_dcs_encdec", channel: 13,
		freqHz: 145_100_000, clarHz: 0, rxClar: false,
		mode: cat.Mode('4'), ctcss: cat.CTCSSDCSEncDec, shift: cat.ShiftSimplex,
	},
	{
		name: "mw_ch014_145m100_fm_p8_4_dcs_enc", channel: 14,
		freqHz: 145_100_000, clarHz: 0, rxClar: false,
		mode: cat.Mode('4'), ctcss: cat.CTCSSDCSEnc, shift: cat.ShiftSimplex,
	},
}

// TestGoldenMWSetVectors builds each MW Set frame from the hand-decomposed
// record and byte-compares it with the golden, then asserts admissibility —
// and, once after the loop, builds the TWO records the goldens never wrote.
//
// THE TX-CLARIFIER HALF IS THE EVIDENCE GAP MADE MECHANICAL, and it is the
// mirror image of core/cat/ft891/golden_test.go's refusal half. Byte 21 is
// printed `P5 0: TX CLAR "OFF" 1: TX CLAR "ON"` on this radio's MW block
// (ft991a_layout.txt:1042), so a record carrying TxClar true describes
// something the manual DOES document — and every G vector nonetheless
// carries '0' there. So the assertion is that the TxClar-true frame builds,
// and that it differs from the golden at position 21 and at no other
// position: the state the derivation did not write is still held to the
// position the chart gives it. dialect_test.go's TestDifferencePinMemoryP5
// carries the FT-891 counter-example, where the same record is refused. It
// is checked once, against vector 0, rather than inside every subtest: what
// it proves ("and at no other position") does not depend on which vector it
// is run against, so six repetitions would prove nothing six extra times.
//
// THE PMS HALF OF THE WRITE DOMAIN IS THE OTHER GAP MADE MECHANICAL. MW's
// own legend never decomposes its P1 span, so the derivation wrote no PMS
// vector here (mw-vectors.golden's "SLOT-RANGE LEGEND" note) — but MW's
// write-direction slot predicate, d.writableSlot (mw.go:81-82), is a
// DIFFERENT rule from MT's, d.mtSlotValid (mtcombined.go:110-111; the split
// is deliberate, mtcombined.go:90-94), so the MT ch100 vector's PMS pass
// does not exercise this one. This builds a PMS-slot MW frame directly, a
// constructed record rather than a golden vector, exactly as the TxClar
// counterpart above is.
func TestGoldenMWSetVectors(t *testing.T) {
	d := ft991a.Dialect()
	vs := loadGoldenVectors(t, "mw-vectors.golden")
	requireVectorNames(t, vs,
		"mw_ch001_7m100_lsb_plain",
		"mw_ch010_145m500_fm_clar_minus_0500_ctcss_encdec_minus_shift",
		"mw_ch011_145m100_fm_p8_1_ctcss_encdec",
		"mw_ch012_145m100_fm_p8_2_ctcss_enc",
		"mw_ch013_145m100_fm_p8_3_dcs_encdec",
		"mw_ch014_145m100_fm_p8_4_dcs_enc",
	)

	// The record vector 0 decomposes to, kept for the two constructed
	// counterparts run once after the loop below.
	var firstRecord cat.MemoryData

	for i, want := range mwVectors {
		v := vs[i]
		t.Run(v.name, func(t *testing.T) {
			requireGoldenLength(t, v, 28)

			// The 28-byte reader is MR's, and it is prefix-checked: this is
			// the assertion behind "MW has no parser to decompose through",
			// made mechanical so that a future loosening of the prefix check
			// is noticed here rather than assumed away.
			if _, err := d.ParseMRAnswer([]byte(v.frame)); err == nil {
				t.Fatalf("ParseMRAnswer accepted an MW frame %q — the prefix check has been lost", v.frame)
			}

			if got := v.frame[20]; got != '0' {
				t.Fatalf("P5 (position 21) of the golden is %q, want '0' — every G vector carries the TX-clarifier-off byte. THIS IS A STOP.", got)
			}
			// P7, position 23: the frame's own byte against the policy this
			// dialect declares, rather than against a value this test chose.
			// TestIdentityPinMWWriteKind is where the policy's own value and
			// its caveat are pinned.
			if got := v.frame[22]; got != d.MWWriteKind() {
				t.Fatalf("P7 (position 23) of the golden is %q, but this dialect's MWWriteKind is %q — the legend's misprinted \"00: (Fixed)\" against a one-position field. THIS IS A STOP.", got, d.MWWriteKind())
			}
			if got := v.frame[24:26]; got != "00" {
				t.Fatalf("P9 (positions 25-26) of the golden is %q, want \"00\". THIS IS A STOP.", got)
			}
			// P3's sign byte (position 15): read off the file, uniformly
			// with the other positions above.
			if got := v.frame[14]; got != '+' && got != '-' {
				t.Fatalf("P3 (position 15) of the golden is %q, want '+' or '-' — this dialect's memory codec's clarifier sign byte. THIS IS A STOP.", got)
			}

			slot, err := d.MemorySlot(want.channel)
			if err != nil {
				t.Fatalf("MemorySlot(%d): %v", want.channel, err)
			}
			if got, wantWire := slot.Wire(), fmt.Sprintf("%03d", want.channel); got != wantWire {
				t.Fatalf("MemorySlot(%d).Wire() = %q, want %q", want.channel, got, wantWire)
			}
			m := cat.MemoryData{
				Slot:   slot,
				FreqHz: want.freqHz,
				ClarHz: want.clarHz,
				RxClar: want.rxClar,
				TxClar: false, // the byte the goldens carry; see the doc comment.
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

			if i == 0 {
				// Held here rather than in a closure, so the constructed
				// counterparts below can reuse it verbatim.
				firstRecord = m
			}
		})
	}

	// The TxClar-true counterpart, checked once (see the doc comment for
	// why one vector is enough): it differs from golden vector 0 at
	// position 21 and at no other position.
	t.Run("constructed_txclar_true_counterpart", func(t *testing.T) {
		v := vs[0]
		txClar := firstRecord
		txClar.TxClar = true
		withTx, err := d.BuildMWSet(txClar)
		if err != nil {
			t.Fatalf("BuildMWSet REFUSED a TxClar-true record: %v. Under %v byte 21 is a live TX-clarifier flag on this radio (ft991a_layout.txt:1042), so the record is one the manual describes.", err, d.MemoryP5())
		}
		got := string(withTx.Bytes())
		if len(got) != len(v.frame) {
			t.Fatalf("the TxClar-true frame is %d bytes against the golden's %d", len(got), len(v.frame))
		}
		for pos := 1; pos <= len(got); pos++ {
			same := got[pos-1] == v.frame[pos-1]
			if pos == 21 && same {
				t.Errorf("the TxClar-true frame matches the golden at position 21 (%q) — the TX-clarifier flag is not reaching byte 21", got[20])
			}
			if pos != 21 && !same {
				t.Errorf("the TxClar-true frame differs from the golden at position %d (%q against %q) — only byte 21 carries this flag", pos, got[pos-1], v.frame[pos-1])
			}
		}
	})

	// The PMS-slot counterpart: a constructed record, not a golden vector,
	// naming PMS pair 1 lower ("100"). See the doc comment — this is the
	// hole M1 of the s1-t8 review names: MT ch100's admission does not
	// exercise MW's own writableSlot PMS arm, so this does.
	t.Run("constructed_pms_slot_counterpart", func(t *testing.T) {
		slot, err := d.PMSSlot(1, false)
		if err != nil {
			t.Fatalf("PMSSlot(1, false): %v", err)
		}
		if got := slot.Wire(); got != "100" {
			t.Fatalf("PMSSlot(1, false).Wire() = %q, want %q", got, "100")
		}
		if !slot.IsPMS() {
			t.Fatalf("PMSSlot(1, false).IsPMS() = false")
		}
		m := firstRecord
		m.Slot = slot
		built, err := d.BuildMWSet(m)
		if err != nil {
			t.Fatalf("BuildMWSet refused a PMS-slot record naming %q: %v — this dialect's MW write gate (d.writableSlot) must admit PMS as well as memory slots", slot.Wire(), err)
		}
		if got, want := len(built.Bytes()), 28; got != want {
			t.Fatalf("BuildMWSet on a PMS slot built %d bytes, want %d", got, want)
		}
		if got := string(built.Bytes())[2:5]; got != "100" {
			t.Errorf("the built frame's P1 (positions 3-5) is %q, want %q", got, "100")
		}
		if !d.AllowedCommand(built.Bytes()) {
			t.Errorf("AllowedCommand refused a PMS-slot MW frame %q — THIS IS A STOP: MW's write gate must admit its own builder's output.", built.Bytes())
		}
	})
}

// mrReadVectors states which slot each MR Read request names, and by which
// constructor.
//
// ALL FOUR ARE ON ONE NUMBER LINE, and that is this radio's whole story
// about slots: its MR legend prints exactly one range, "P0/1 001-117 (Memory
// Channel)" (ft991a_layout.txt:966), with no 5 MHz bank, no EMG channel and
// no PMS token — where the FT-891's MR legend prints "501 - 510 (5 MHz…)"
// and "EMG (Emergency)". So the four vectors walk the ends of the single
// span instead: the first and last regular channel, and the first and last
// PMS channel. The PMS spellings in the vector NAMES (100 = P-1L,
// 117 = P-9U) are the MC legend's, borrowed and labelled as borrowed by
// mr-vectors.golden's own "SLOT RANGES THE MR LEGEND LISTS" note.
//
// THE CONSTRUCTORS ARE THE ASSERTION. 100 and 117 are reached through
// PMSSlot, not MemorySlot, so a dialect that had merely widened its memory
// range to 117 would build the same bytes and fail the IsPMS check below.
var mrReadVectors = []struct {
	name         string
	slot         func(cat.Dialect) (cat.Slot, error)
	wantSlotWire string
	wantPMS      bool
}{
	{"mr_read_ch001_ordinary", func(d cat.Dialect) (cat.Slot, error) { return d.MemorySlot(1) }, "001", false},
	{"mr_read_ch099_last_regular", func(d cat.Dialect) (cat.Slot, error) { return d.MemorySlot(99) }, "099", false},
	{"mr_read_ch100_pms_p_1l", func(d cat.Dialect) (cat.Slot, error) { return d.PMSSlot(1, false) }, "100", true},
	{"mr_read_ch117_pms_p_9u", func(d cat.Dialect) (cat.Slot, error) { return d.PMSSlot(9, true) }, "117", true},
}

// mrAnswerVectors states, as literals, what each 28-byte MR Answer carries.
//
// EVERY DATA BYTE OF AN ANSWER IS A PREDICTION: the manual prints no worked
// MR example anywhere, so these two frames are hand-derived shapes filled
// with legend-legal values, not observed replies (mr-vectors.golden
// INHERITED-ASSUMED item 3). P7 is the sharpest of them — the MR legend
// prints "0: VFO 1: Memory" (976) but never says which a memory read answers
// with, and '1' is assumed (item 1). That byte is also what distinguishes an
// answer from the MT/MW Set frames above, where the same position is a fixed
// '0'.
var mrAnswerVectors = []struct {
	name     string
	slotWire string
	wantPMS  bool
	freqHz   uint32
	mode     cat.Mode
}{
	{"mr_answer_ch001_7m100_lsb_memory", "001", false, 7_100_000, cat.Mode('1')},
	{"mr_answer_ch100_pms_p_1l_7m000_lsb_memory", "100", true, 7_000_000, cat.Mode('1')},
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
// "Set: X" and the block's Set chart is an empty grid), so no builder in
// this package produces a 28-byte MR frame and there is nothing to re-encode
// with: PARSE-ONLY IS THE ANSWER DIRECTION'S TEST. The gate assertion at the
// end is the same point from the other side — an answer frame is never a
// legal outbound command.
func TestGoldenMRVectors(t *testing.T) {
	d := ft991a.Dialect()
	vs := loadGoldenVectors(t, "mr-vectors.golden")
	requireVectorNames(t, vs,
		"mr_read_ch001_ordinary",
		"mr_read_ch099_last_regular",
		"mr_read_ch100_pms_p_1l",
		"mr_read_ch117_pms_p_9u",
		"mr_answer_ch001_7m100_lsb_memory",
		"mr_answer_ch100_pms_p_1l_7m000_lsb_memory",
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
			if got := slot.IsPMS(); got != want.wantPMS {
				t.Errorf("slot %q: IsPMS() = %v, want %v — 001-099 are regular channels and 100-117 the nine PMS pairs (ft991a_layout.txt:915-916)", slot.Wire(), got, want.wantPMS)
			}
			built, err := d.BuildMRRead(slot)
			if err != nil {
				t.Fatalf("BuildMRRead(%q): %v\nTHIS IS A STOP: the vector names a slot inside this radio's only printed range.", slot.Wire(), err)
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
				t.Fatalf("P5 (position 21) of the golden is %q, want '0' — every G vector carries the TX-clarifier-off byte. THIS IS A STOP.", got)
			}

			m, err := d.ParseMRAnswer([]byte(v.frame))
			if err != nil {
				t.Fatalf("ParseMRAnswer(%q) refused a golden frame: %v\n"+
					"THIS IS A STOP: the derivation or the codec misreads revision 1711-D.", v.frame, err)
			}

			if got := m.Slot.Wire(); got != want.slotWire {
				t.Errorf("P1 (positions 3-5): got %q, want %q", got, want.slotWire)
			}
			if got := m.Slot.IsPMS(); got != want.wantPMS {
				t.Errorf("P1 (positions 3-5): slot %q decoded IsPMS()=%v, want %v", m.Slot.Wire(), got, want.wantPMS)
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
				t.Errorf("P5 (position 21): got TxClar true, want false — the golden's byte 21 is '0'")
			}
			if m.Mode != want.mode {
				t.Errorf("P6 (position 22): got %q (%s), want %q (%s)",
					m.Mode.Wire(), d.ModeName(m.Mode), want.mode.Wire(), d.ModeName(want.mode))
			}
			if got, wantName := d.ModeName(m.Mode), "LSB"; got != wantName {
				t.Errorf("P6 (position 22) mode name: got %q, want %q", got, wantName)
			}
			// The Read direction's vocabulary, and the whole point of having
			// an answer vector at all: '1' Memory, ASSUMED, not the Set
			// direction's fixed '0'.
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

// mcVectors states which slot each MC Set vector names, and by which
// constructor.
//
// THESE FOUR FRAMES ARE THE MILESTONE'S ONLY HAND-DERIVED EVIDENCE that MC's
// legend gives the whole 001-117 space, and they have a SECOND reader:
// dialect_test.go's TestDegeneracyPinWideAndNarrowAgree loads the same file
// through mcGoldenFrames for the degeneracy pin, and Stage 2's fake will
// serve the same space from it. That is why the file is named in the plan's
// task-8 acceptance rather than left as a golden nobody consults.
//
// The four walk both ends of both halves of the legend's split: "001 - 099:
// Regular Memory Channel" and "100: P-1L 101: P-1U ~ 116: P-9L 117: P-9U"
// (ft991a_layout.txt:915-916). 012 rather than 001 is the derivation's own
// free choice inside the regular range.
var mcVectors = []struct {
	name         string
	slot         func(cat.Dialect) (cat.Slot, error)
	wantSlotWire string
	wantPMS      bool
}{
	{"mc_ch012_regular_memory_channel", func(d cat.Dialect) (cat.Slot, error) { return d.MemorySlot(12) }, "012", false},
	{"mc_ch099_last_regular_memory_channel", func(d cat.Dialect) (cat.Slot, error) { return d.MemorySlot(99) }, "099", false},
	{"mc_ch100_first_pms_slot_p_1l", func(d cat.Dialect) (cat.Slot, error) { return d.PMSSlot(1, false) }, "100", true},
	{"mc_ch117_last_pms_slot_p_9u", func(d cat.Dialect) (cat.Slot, error) { return d.PMSSlot(9, true) }, "117", true},
}

// TestGoldenMCSetVectors builds each 6-byte MC Set frame, byte-compares it
// with the golden, asserts the outbound gate admits it, and parses it back
// through ParseMCAnswer.
//
// PARSING A SET FRAME WITH THE ANSWER PARSER IS THE RIGHT DECOMPOSITION
// HERE, exactly as it is for MT: the MC block's Set and Answer charts print
// the identical 6-position layout, which mc-vectors.golden counted
// independently and recorded ("The ANSWER chart in the same block was
// counted independently and is the identical 6-position layout").
func TestGoldenMCSetVectors(t *testing.T) {
	d := ft991a.Dialect()
	vs := loadGoldenVectors(t, "mc-vectors.golden")
	requireVectorNames(t, vs,
		"mc_ch012_regular_memory_channel",
		"mc_ch099_last_regular_memory_channel",
		"mc_ch100_first_pms_slot_p_1l",
		"mc_ch117_last_pms_slot_p_9u",
	)

	for i, want := range mcVectors {
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
			built, err := d.BuildMCSet(slot)
			if err != nil {
				t.Fatalf("BuildMCSet(%q): %v\nTHIS IS A STOP: the vector names a slot this radio's MC legend prints.", slot.Wire(), err)
			}
			requireGoldenFrame(t, v, built, "Dialect().BuildMCSet")

			if !d.AllowedCommand([]byte(v.frame)) {
				t.Errorf("AllowedCommand refused a Set-direction golden frame %q — THIS IS A STOP.", v.frame)
			}

			back, err := d.ParseMCAnswer([]byte(v.frame))
			if err != nil {
				t.Fatalf("ParseMCAnswer(%q) refused a golden frame: %v\nTHIS IS A STOP.", v.frame, err)
			}
			if got := back.Wire(); got != want.wantSlotWire {
				t.Errorf("ParseMCAnswer recovered slot %q, want %q", got, want.wantSlotWire)
			}
			if got := back.IsPMS(); got != want.wantPMS {
				t.Errorf("ParseMCAnswer recovered slot %q with IsPMS()=%v, want %v", back.Wire(), got, want.wantPMS)
			}
		})
	}
}

// The ID frames mc-vectors.golden records IN ITS COMMENTS rather than as
// vectors, counted off the ID block's own charts (printed folio 10 = PDF
// page 11): the Answer chart is "I D P1 P1 P1 P1 ;" — seven positions — with
// the legend "P1 0670: FT-991A", and the Read chart is "I D ;".
//
// THE ANSWER IS LOAD-BEARING FOR THE DIALECT'S CATID: "0670" is the only
// place this radio's identity reaches the wire, and TestGoldenIDFrames binds
// the golden's recorded frame to the value dialect.go declares. The ID Set
// chart is printed as an empty grid, which is why nothing here builds one.
const (
	idAnswerFrame = "ID0670;"
	idReadRequest = "ID;"
)

// TestGoldenIDFrames byte-compares BuildIDRead with the recorded Read frame,
// parses the recorded Answer, and binds the recovered ID to this dialect's
// declared CATID.
//
// ParseIDAnswer deliberately does not compare against the dialect's own
// CATID — the point of reading ID is to discover which radio answered — so
// the comparison is made here, where the golden supplies the WANT side.
func TestGoldenIDFrames(t *testing.T) {
	d := ft991a.Dialect()

	// idReadRequest and idAnswerFrame are re-typed from mc-vectors.golden's
	// COMMENT block, which loadGoldenVectors skips — the freeze above does
	// not reach them, and idAnswerFrame is load-bearing for this dialect's
	// CATID (the doc comment above). Assert both verbatim against the
	// frozen file's bytes, so an edit to either const is caught by the same
	// mechanism that protects every other byte here.
	raw, err := os.ReadFile(filepath.Join(goldenDir, "mc-vectors.golden"))
	if err != nil {
		t.Fatalf("reading mc-vectors.golden: %v", err)
	}
	if !strings.Contains(string(raw), idAnswerFrame) {
		t.Errorf("idAnswerFrame %q does not appear verbatim in mc-vectors.golden — the const has drifted from the frozen comment it was transcribed from", idAnswerFrame)
	}
	if !strings.Contains(string(raw), idReadRequest) {
		t.Errorf("idReadRequest %q does not appear verbatim in mc-vectors.golden — the const has drifted from the frozen comment it was transcribed from", idReadRequest)
	}

	if got := string(d.BuildIDRead().Bytes()); got != idReadRequest {
		t.Errorf("BuildIDRead built %q, want %q — the ID block's Read chart", got, idReadRequest)
	}
	if !d.AllowedCommand([]byte(idReadRequest)) {
		t.Errorf("AllowedCommand refused the ID read request %q — THIS IS A STOP.", idReadRequest)
	}

	if got, want := len(idAnswerFrame), 7; got != want {
		t.Fatalf("the recorded ID answer is %d bytes, want %d — the counted chart length", got, want)
	}
	id, err := d.ParseIDAnswer([]byte(idAnswerFrame))
	if err != nil {
		t.Fatalf("ParseIDAnswer(%q): %v\nTHIS IS A STOP.", idAnswerFrame, err)
	}
	if id != d.CATID() {
		t.Errorf("the ID answer mc-vectors.golden records carries %q, but this dialect's CATID is %q.\n"+
			"The legend prints \"P1   0670: FT-991A\" (ft991a_layout.txt:772); one of the two is wrong. THIS IS A STOP.",
			id, d.CATID())
	}
	if d.AllowedCommand([]byte(idAnswerFrame)) {
		t.Errorf("AllowedCommand ADMITTED the ID answer %q — ID has no Set direction, and an answer is never an outbound frame.", idAnswerFrame)
	}
}

// exVectors states each EX read vector's menu number and the menu-chart row
// the quarantined deriver recorded beside it in ex-vectors.golden.
//
// THE ADDRESS IS A SINGLE COMPONENT on this radio — cat.EXAddressSingle,
// "P1 : 001 - 153 (MENU Number)" (ft991a_layout.txt:520) — so P2 and P3 are
// zero for every member and are not components the chart prints. They are
// not written here either; NewEXAddress is given the menu number and two
// zeros, and rule V12 requires nothing else.
//
// THE NAME AND Digits BIND TWO INDEPENDENT READINGS of one printed chart:
// this quarantined derivation, and transcription A (table2.csv), from which
// the dialect's inventory is generated. Neither agent saw the other's work.
// Disagreement would be a STOP arbitrated against the PDF, not a test to
// relax — the same shape crosscheck_test.go applies to the whole chart,
// narrowed here to the rows the frame-geometry leg happened to quote.
//
// ROW 028 IS BOUND BY MEMBERSHIP ONLY, and deliberately. ex-vectors.golden
// quotes verbatim chart rows for 001, 153 and 151 and quotes NO row for 028
// — its vector name is the only annotation it carries — so a name and a
// Digits value asserted here would be this file's invention rather than the
// derivation's reading. What binds row 028's name and Digits is the A-vs-B
// cross-check, which covers all 153 rows. hasRow below records which of the
// four the golden actually quotes.
var exVectors = []struct {
	name     string
	menu     int
	hasRow   bool
	itemName string
	digits   int
}{
	// The FIRST row of the chart, quoted verbatim in the golden as
	// "001 | AGC FAST DELAY | 20 ~ 4000 msec … | 4".
	{"ex_read_menu_001_agc_fast_delay_first_row", 1, true, "AGC FAST DELAY", 4},
	// The LAST row, quoted as "153 | WIRES DG-ID | 00: AUTO … | 2".
	{"ex_read_menu_153_wires_dg_id_last_row", 153, true, "WIRES DG-ID", 2},
	// The largest Digits value anywhere in the chart, unique in the table,
	// quoted as "151 | PRESET FREQUENCY | 00030000 ~ 47000000 | 8". It is
	// what makes the derived 14-byte Answer length below.
	{"ex_read_menu_151_preset_frequency_largest_digits_8", 151, true, "PRESET FREQUENCY", 8},
	// A free choice, and the row Stage 2 cites as the RS-232C gate. The
	// golden quotes no chart row for it; see the doc comment.
	{"ex_read_menu_028_gps_232c_select", 28, false, "", 0},
}

// TestGoldenEXReadVectors builds the 6-byte EX read request for each menu
// number and byte-compares it with the golden, asserts admissibility, and
// binds the golden's own menu-chart quotations to this dialect's generated
// inventory.
//
// SIX BYTES, THE NARROWEST IN THE FAMILY. This radio's EX Read chart is
// "E X P1 P1 P1 ;" against the FT-891's four-digit address and the FTdx10's
// six, which is what cat.EXAddressSingle exists to carry (doc.go's
// reused-command verification; dialect_test.go's
// TestDifferencePinEXAddressForm). A Read request carries no parameter at
// all — the chart terminates at position 6 — which is also why only the READ
// frame is admissible outbound: EX Set and Answer share the read's prefix
// and address field with a longer body, and the gate refuses them by shipped
// policy (the M8d menu-write no-go), not by accident of shape.
func TestGoldenEXReadVectors(t *testing.T) {
	d := ft991a.Dialect()
	vs := loadGoldenVectors(t, "ex-vectors.golden")
	requireVectorNames(t, vs,
		"ex_read_menu_001_agc_fast_delay_first_row",
		"ex_read_menu_153_wires_dg_id_last_row",
		"ex_read_menu_151_preset_frequency_largest_digits_8",
		"ex_read_menu_028_gps_232c_select",
	)

	items := d.EXItems()

	for i, want := range exVectors {
		v := vs[i]
		t.Run(v.name, func(t *testing.T) {
			requireGoldenLength(t, v, 6)

			addr, err := d.NewEXAddress(want.menu, 0, 0)
			if err != nil {
				t.Fatalf("NewEXAddress(%d,0,0): %v\nTHIS IS A STOP: the vector names a menu-chart row "+
					"this dialect's inventory does not hold.", want.menu, err)
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
			if !want.hasRow {
				return
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
// records AS A COMMENT — "pos 1-2 EX, pos 3-5 P1, pos 6.. P2", worked
// through for menu item 151 as 5 + 8 + 1 = 14 bytes.
//
// THE FOURTEEN IS DERIVED, NOT COUNTED, and the golden says so in terms:
// "The frame length 14 is DERIVED (chart run length + Digits column), not
// counted from a closed chart, because the EX Answer chart has no printed
// final position number." That derivation is exactly what core/cat's
// exAnswerMaxLen computes — 2 + address width + widest Digits + 1 — and what
// the Stage 0 task-4 test exaddresssingle_test.go already pins; this test
// binds the golden's own arithmetic to it from the evidence side.
//
// THE BODY IS SYNTHETIC AND IS NOT EVIDENCE. The vector file deliberately
// writes no P2 value — "Marked shape, value NOT invented" — because no
// FT-991A has ever answered anything and inventing a reply would put a
// fabricated observation into a quarantined artefact. The eight bytes below
// are this TEST's construction, chosen to sit inside the row's printed range
// ("00030000 ~ 47000000"), and what is asserted is the SHAPE the codec
// accepts and the address it recovers, never that a radio would send these
// bytes. core/cat's ParseEXAnswer enforces no width policy — it bounds the
// frame by this dialect's widest Digits and returns the body VERBATIM — so
// nothing here is a claim about what the radio would send either.
func TestGoldenEXAnswerShape(t *testing.T) {
	d := ft991a.Dialect()

	addr, err := d.NewEXAddress(151, 0, 0)
	if err != nil {
		t.Fatalf("NewEXAddress(151,0,0): %v", err)
	}
	synthetic := "00030000" // eight bytes, this test's own; see the doc comment.
	if len(synthetic) != 8 {
		t.Fatalf("the synthetic P2 body is %d bytes, want 8 — row 151's Digits column", len(synthetic))
	}
	frame := "EX" + d.EXWire(addr) + synthetic + ";"
	if got, want := len(frame), 14; got != want {
		t.Fatalf("the answer frame is %d bytes, want %d — the golden works it through as 5 + 8 + 1", got, want)
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

	// One byte past the derived ceiling: the bound is this dialect's own
	// widest Digits, so a fifteen-byte answer is outside a window the chart
	// never closed but the menu table does.
	if _, _, err := d.ParseEXAnswer([]byte("EX" + d.EXWire(addr) + synthetic + "0;")); err == nil {
		t.Errorf("ParseEXAnswer accepted a 15-byte answer — the widest Digits value in this chart is 8, so the ceiling is 2+3+8+1")
	}

	// The other direction of the M8d menu-write no-go: an EX frame carrying
	// a parameter is not a read, and the outbound gate admits only the
	// 6-byte read on this dialect.
	if d.AllowedCommand([]byte(frame)) {
		t.Errorf("AllowedCommand ADMITTED the EX answer %q — only the read request is an outbound EX frame.", frame)
	}
}

// TestGoldenCountedGeometry is the plan's task-8 acceptance item that nothing
// recorded until now: the COUNTED chart geometry the twenty-nine vectors were
// measured from, held against what the dialect's own APIs produce.
//
// It exists because task 7's identity pins DEPEND on these numbers — MT/MW/MR
// 41/28/28 in TestIdentityPinFrameGeometry — and until this test the numbers
// lived only in the goldens' comment blocks and the task-6 commit message.
// Each row below cites where its number was counted:
//
//	MT   Set 41 / Read 6 / Answer 41    mt-vectors.golden; provenance.md "MT"
//	MW   Set 28                         mw-vectors.golden ("the Read and Answer
//	                                    charts are printed as empty grids, so
//	                                    no Read or Answer layout exists")
//	MR   Read 6 / Answer 28             mr-vectors.golden; the Set chart is an
//	                                    empty grid, so there is no Set length
//	MC   Set 6 / Read 3 / Answer 6      mc-vectors.golden
//	EX   Read 6                         ex-vectors.golden; the Set and Answer
//	                                    charts print an open-ended run and have
//	                                    NO countable length
//	ID   Answer 7 (Read 3)              mc-vectors.golden's ID section
//
// The EX Answer's 14 is deliberately absent from that list and lives in
// TestGoldenEXAnswerShape instead, because it is DERIVED rather than counted.
func TestGoldenCountedGeometry(t *testing.T) {
	d := ft991a.Dialect()

	memory, err := d.MemorySlot(1)
	if err != nil {
		t.Fatalf("MemorySlot(1): %v", err)
	}
	pms, err := d.PMSSlot(1, false)
	if err != nil {
		t.Fatalf("PMSSlot(1, false): %v", err)
	}

	// MT Answer, both bounds: equal bounds are the combined form's signature,
	// and 41 is the counted Set and Answer length alike.
	min, max, err := d.MTAnswerBounds()
	if err != nil {
		t.Fatalf("MTAnswerBounds(): %v", err)
	}
	if min != 41 || max != 41 {
		t.Errorf("MTAnswerBounds() = (%d, %d), want (41, 41) — the MT Answer chart's counted length", min, max)
	}

	record := cat.MemoryData{
		Slot: memory, FreqHz: 7_100_000, Mode: cat.Mode('1'),
		Kind: cat.CombinedMTSetKind, CTCSS: cat.CTCSSOff, Shift: cat.ShiftSimplex,
	}
	mtSet, err := d.BuildMTSetCombined(record, "TAG")
	if err != nil {
		t.Fatalf("BuildMTSetCombined: %v", err)
	}
	mtRead, err := d.BuildMTRead(memory)
	if err != nil {
		t.Fatalf("BuildMTRead: %v", err)
	}
	mwRecord := record
	mwRecord.Kind = d.MWWriteKind()
	mwSet, err := d.BuildMWSet(mwRecord)
	if err != nil {
		t.Fatalf("BuildMWSet: %v", err)
	}
	mrRead, err := d.BuildMRRead(memory)
	if err != nil {
		t.Fatalf("BuildMRRead: %v", err)
	}
	mcSet, err := d.BuildMCSet(pms)
	if err != nil {
		t.Fatalf("BuildMCSet: %v", err)
	}
	exAddr, err := d.NewEXAddress(1, 0, 0)
	if err != nil {
		t.Fatalf("NewEXAddress(1,0,0): %v", err)
	}
	exRead, err := d.BuildEXRead(exAddr)
	if err != nil {
		t.Fatalf("BuildEXRead: %v", err)
	}

	for _, c := range []struct {
		what  string
		built []byte
		want  int
	}{
		{"MT Set (chart: 41 positions)", mtSet.Bytes(), 41},
		{"MT Read (chart: \"M T P0 P0 P0 ;\")", mtRead.Bytes(), 6},
		{"MW Set (chart: 28 positions)", mwSet.Bytes(), 28},
		{"MR Read (chart: \"M R P0 P0 P0 ;\")", mrRead.Bytes(), 6},
		{"MC Set (chart: \"M C P1 P1 P1 ;\")", mcSet.Bytes(), 6},
		{"MC Read (chart: \"M C ;\")", d.BuildMCRead().Bytes(), 3},
		{"EX Read (chart: \"E X P1 P1 P1 ;\")", exRead.Bytes(), 6},
		{"ID Read (chart: \"I D ;\")", d.BuildIDRead().Bytes(), 3},
	} {
		if got := len(c.built); got != c.want {
			t.Errorf("%s: this dialect built %d bytes (%q), want %d — the counted chart length. THIS IS A STOP.", c.what, got, c.built, c.want)
		}
	}

	// The two ANSWER lengths this package has no builder for, asserted
	// through the parsers instead: one byte either side of each is refused,
	// which is what makes the length a bound rather than a coincidence.
	mrAnswer := append([]byte(nil), mwSet.Bytes()...)
	mrAnswer[0], mrAnswer[1] = 'M', 'R'
	if _, err := d.ParseMRAnswer(mrAnswer); err != nil {
		t.Errorf("ParseMRAnswer refused a 28-byte frame (%q): %v — the MR Answer chart is the MW Set chart under another prefix", mrAnswer, err)
	}
	if _, err := d.ParseMRAnswer(mrAnswer[:27]); err == nil {
		t.Errorf("ParseMRAnswer accepted a 27-byte frame — the MR Answer chart runs to 28")
	}
	if _, err := d.ParseMRAnswer(append(mrAnswer[:27:27], '0', ';')); err == nil {
		t.Errorf("ParseMRAnswer accepted a 29-byte frame — the MR Answer chart runs to 28")
	}
	if _, err := d.ParseMCAnswer(mcSet.Bytes()); err != nil {
		t.Errorf("ParseMCAnswer refused a 6-byte frame (%q): %v — the MC Answer chart is the MC Set chart", mcSet.Bytes(), err)
	}
	if _, err := d.ParseMCAnswer(mcSet.Bytes()[:5]); err == nil {
		t.Errorf("ParseMCAnswer accepted a 5-byte frame — the MC Answer chart runs to 6")
	}
	if _, err := d.ParseMCAnswer([]byte("MC0012;")); err == nil {
		t.Errorf("ParseMCAnswer accepted a 7-byte frame — the MC Answer chart runs to 6")
	}
	if _, err := d.ParseIDAnswer([]byte(idAnswerFrame)[:6]); err == nil {
		t.Errorf("ParseIDAnswer accepted a 6-byte frame — the ID Answer chart runs to 7")
	}
	if _, err := d.ParseIDAnswer([]byte("ID06700;")); err == nil {
		t.Errorf("ParseIDAnswer accepted an 8-byte frame — the ID Answer chart runs to 7")
	}
}

// spliceSlot returns frame with its P1 field (positions 3-5) replaced by
// wire: the MT, MW, MR and MC frames all put their three-position address
// immediately after the two command letters, so one splice serves the four
// families it is used on below (EX's address is elsewhere in its frame and
// is not spliced here). The only call passes a 3-character wire.
func spliceSlot(frame, wire string) string {
	return frame[:2] + wire + frame[5:]
}

// TestGoldenGateAdmissibility is the plan's task-8 gate leg, stated on the
// golden frames themselves rather than on synthetic ones: a DCS-state vector
// ADMITTED, a '5' P8 REFUSED, a "P1L" slot REFUSED.
//
// The admissions are already asserted per-vector in the three Set-direction
// tests above; what this test adds is the two REFUSALS, and it derives them
// by forging exactly one field of a frame the gate has just admitted. That
// is the shape that says something: a refusal of a frame the gate would have
// refused anyway proves nothing, so each case here is a one-field mutation
// of a known-admissible golden, and the unmutated frame's admission is
// re-asserted beside it.
//
//   - THE DCS STATES are this radio's own P8 domain, `3: DCS ENC/DEC` and
//     `4: DCS ENC` (ft991a_layout.txt:1010-1011), and cat.ToneStatesCTCSSAndDCS
//     is what carries them. Whether the radio ACCEPTS a DCS state written
//     without a CN code first is doc.go's register entry "THE DCS STATES'
//     SET ACCEPTANCE"; admissibility here is a statement about this
//     programme's gate, not about the radio.
//   - '5' IS ONE PAST THE PRINTED DOMAIN. The legend stops at '4', so a
//     frame carrying '5' at position 24 describes a state no chart prints.
//   - "P1L" IS A FORM THIS MANUAL NEVER PRINTS. The PMS pairs are decimal
//     channel numbers here (cat.PMSFormNumeric), and the token is the
//     FTdx10's and the FT-891's spelling. dialect_test.go's
//     TestDifferencePinPMSFormAndSlotSpace holds the same refusal against
//     those radios' ability to build it; this test holds it on the golden
//     frames, where the mutation is visibly one field of an otherwise
//     admissible command.
func TestGoldenGateAdmissibility(t *testing.T) {
	d := ft991a.Dialect()

	mt := loadGoldenVectors(t, "mt-vectors.golden")
	mw := loadGoldenVectors(t, "mw-vectors.golden")
	mr := loadGoldenVectors(t, "mr-vectors.golden")
	mc := loadGoldenVectors(t, "mc-vectors.golden")

	// The DCS pair, one from each Set-direction command, named by the
	// vectors' own names rather than by index into a table.
	dcs := []goldenVector{mt[7], mt[8], mw[4], mw[5]}
	for _, v := range dcs {
		if !strings.Contains(v.name, "dcs") {
			t.Fatalf("%s:%d: expected a DCS vector at this position, got %q — the tables and the files have drifted", v.file, v.line, v.name)
		}
		if got := v.frame[23]; got != '3' && got != '4' {
			t.Fatalf("%s: P8 (position 24) is %q, want '3' or '4'. THIS IS A STOP.", v.name, got)
		}
		if !d.AllowedCommand([]byte(v.frame)) {
			t.Errorf("AllowedCommand refused the DCS-state golden frame %s (%q) — this manual's P8 legend prints both DCS states. THIS IS A STOP.", v.name, v.frame)
		}

		// ONE FIELD FORGED: P8 becomes '5', one past the printed domain.
		forged := v.frame[:23] + "5" + v.frame[24:]
		if len(forged) != len(v.frame) {
			t.Fatalf("the forged frame changed length, %d against %d", len(forged), len(v.frame))
		}
		if d.AllowedCommand([]byte(forged)) {
			t.Errorf("AllowedCommand ADMITTED %q, whose P8 (position 24) is '5'. This manual's legend prints '0'-'4' and stops.", forged)
		}
	}
	if _, err := d.ParseCTCSSState('5'); err == nil {
		t.Error("ParseCTCSSState('5') was ACCEPTED — without this the gate refusals above could be a length accident")
	}

	// "P1L" spliced into one frame of every command that carries a slot,
	// including the two read requests, so the refusal is not a property of
	// one grammar branch.
	for _, v := range []goldenVector{mt[0], mw[0], mr[0], mc[0]} {
		if !d.AllowedCommand([]byte(v.frame)) {
			t.Fatalf("AllowedCommand refused the unmutated golden %s (%q) — the mutation below would prove nothing. THIS IS A STOP.", v.name, v.frame)
		}
		forged := spliceSlot(v.frame, "P1L")
		if d.AllowedCommand([]byte(forged)) {
			t.Errorf("AllowedCommand ADMITTED %q, whose slot field is the token \"P1L\". No FT-991A legend prints a PMS token; the pairs are channels 100-117.", forged)
		}
	}
	if _, err := d.ParseSlot("P1L"); err == nil {
		t.Error("ParseSlot(\"P1L\") was ACCEPTED — without this the gate refusals above could be a length accident")
	}
}
