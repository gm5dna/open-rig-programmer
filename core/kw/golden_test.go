// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// This file is the Kenwood half of Stage 1's third item: the mechanical
// replay of evidence leg G's eighty-two hand-derived wire frames against the
// codec that exists in this package.
//
// # What the vectors are, and what this file may do with them
//
// They are evidence leg G, derived by QUARANTINED agents that never opened
// this repository: no code, no generator, no fixture and no other document,
// only 300 and 600 dpi renders of the two Kenwood PC control command
// references' position charts and their parameter legends. Every field width,
// every position boundary and every assumption that had to be inherited
// rather than read is itemised in testdata/provenance.md and repeated in each
// vector file's own header block, which the tables below cite for the values
// they hardcode.
//
// THIS FILE MAY NOT MODIFY ANY VECTOR, and no failure here is ever fixed by
// editing one. The fifteen artefacts were frozen at commit a1e3779, whose
// message records their SHA-256s; TestGoldenVectorsFrozen below enforces the
// same hashes in CI, so the freeze survives a rewritten history or a stray
// regeneration rather than depending on someone running a diff gate. A
// golden-vs-codec mismatch is a STOP for orchestrator arbitration AGAINST THE
// PDF — either the hand derivation or the codec misreads the manual — which
// is why every failure below prints both sides and both lengths: the failure
// output is the arbitration's input.
//
// # WHAT THIS FILE CAN REPLAY TODAY, AND WHAT IT CANNOT
//
// THERE IS NO PER-COMMAND BUILDER OR PARSER IN THIS PACKAGE YET. Task 5 built
// the ENVELOPE — frames, the ';' splitter, the accumulator, the typed errors,
// the outbound gate and the framing adapter — and tasks 6, 7 and 8 (the
// record codec, the identity/MC/EX grammars and the two layouts) had not
// landed when this file was written. So there is no BuildMR, no ParseIF, no
// EX builder to hand a menu number to, and a test that claimed to replay
// those would be claiming a codec that does not exist.
//
// What this file therefore does is exactly what CAN be done honestly:
//
//  1. HASH-FREEZES all fifteen artefacts (TestGoldenVectorsFrozen).
//  2. Pins the ROSTER — every file's vectors, in order, with the frame length
//     the deriver COUNTED off the printed ruler
//     (TestGoldenVectors_RosterAndCountedLengths). The lengths are literals
//     transcribed from each file's own "COUNTED FRAME LENGTHS" block, not
//     measured from the bytes they check.
//  3. Replays every vector through EVERY PIECE OF THE CODEC THAT EXISTS —
//     five replay legs, plus the F1 splitter leg
//     (TestGoldenVectors_TheF1ReadFramesAreNotFramesToTheSplitter), and the
//     list is exhaustive of what core/kw ships today:
//     the outbound gate for the host-built frames
//     (TestGoldenVectors_TheOutboundGateAdmitsEveryHostBuiltFrame); the frame
//     accumulator for every radio-sent one
//     (TestGoldenVectors_TheAccumulatorReassemblesEveryAnswer);
//     InitSequence against the AI0; vector each book prints
//     (TestGoldenVectors_InitSequenceIsTheAI0Vector), the only WHOLE frame
//     any shipped code builds today; EXAddress.Wire against the address field
//     of all seventeen EX vectors
//     (TestGoldenVectors_TheEXAddressFieldIsWhatWireRenders), which is the
//     one FIELD a shipped renderer produces; and PrefixLenMatcher against
//     every answer vector
//     (TestGoldenVectors_EveryAnswerIsMatchedByItsOwnReadsMatcher), the
//     answer-correlation half of the codec.
//  4. Leaves TestTODO_T8_ReplayEveryVectorThroughItsOwnCodec, which names
//     what task 8 must add.
//
// # Hardware status
//
// UNVERIFIED, for all eighty-two vectors, and there is no route to verifying
// them: no Kenwood radio has ever been asked anything by this project (A19).
// Green here means the frames satisfy the envelope both books print, as one
// agent read those books, and NOT that any radio accepts these bytes.

// goldenDir is where evidence leg G lives, relative to this package's
// directory (go test's working directory).
const goldenDir = "testdata"

// frozenVectorSHA256 is the freeze, transcribed from the commit message of
// a1e3779 ("core/kw: import the quarantined evidence legs — L×3 ledgers, B×3
// transcriptions, G geometry"), whose "SHA-256 of every imported file:" block
// records one hash per artefact.
//
// provenance.md is in here with the fourteen vector files because it is not
// commentary about them: it is the assumption register this file cites for
// every value it hardcodes, and a vector file whose assumptions had been
// quietly rewritten would be as corrupt as one whose bytes had. The other
// twelve artefacts a1e3779 froze — legs L and B, the ledgers and transcription
// B — belong to the two menu-chart cross-checks and are pinned by
// core/kw/ts590/crosscheck_test.go and core/kw/ts480/crosscheck_test.go.
var frozenVectorSHA256 = map[string]string{
	"AI-480.golden": "16ebc96588108947a596a284575d339d60b51502d0b0a6c2fc404c9c159b6979",
	"AI-590.golden": "93f8913b8efe780d7f9e12fabf74b46218669b873d6d1bfe452d649537453ab2",
	"EX-480.golden": "5e21df001d9a9f86c83c39edf4681d6166c6dc48335b2da1065a94d152c7d3e1",
	"EX-590.golden": "470eea93dc1f4a041e1c825c19062b0c46e9095e62b392862b244808baf98149",
	"FV-590.golden": "d71f7f2075bfd1e8e6436793fe61040efb024dc4a45ff73573b8138c081f0d00",
	"ID-480.golden": "78faf6d69ffa427c7047a9cfa66823e10bb9f9f554d5c5376f99abba2501ef17",
	"ID-590.golden": "643d710d3159280c50b15f3d219dffa93689561fe59b232629cacba5cb99012e",
	"MC-480.golden": "64aa541e995295ed043fd9bfae161e7d3915dc15ecb54b0b91ee5afdb294253e",
	"MC-590.golden": "6dd87ca8012dadb9c234f9c37037ec999ffd6149753816e4b9b55861bd8ef860",
	"MR-480.golden": "c8ae5cf0c41ead28b64b37bce4329c6b6f27789eadc8c951bb97779402890d41",
	"MR-590.golden": "fc00c69cde8a810bd3dd89cb7afe2e93b7008f83c445b42ec38768cd5fa7e6cf",
	"MW-480.golden": "79a4a2aa88fd6f5007732a10ebc29a7d0ba9bdfea9cc7b67326f3c74d0bee9be",
	"MW-590.golden": "743ecd830415ab8c46bcc0e360a4ebaa8e164a31bfc217f6136b777cb2108277",
	"TY-480.golden": "3f4aef149ebcc9b4c6c21b258d2e666e43380cc397c47ebcd444ba24e84f7b34",
	"provenance.md": "ecc0cee88035107952f3148f3f750a7b389a9d218e4f7e2f9f2b8aa7b47811d5",
}

// TestGoldenVectorsFrozen recomputes each frozen artefact's SHA-256 and
// compares it with the value commit a1e3779 recorded, so that the freeze is
// self-enforcing in CI rather than a fact recoverable only by git
// archaeology.
//
// The second half is the one that catches the interesting case: a walk of
// testdata requires EVERY file present to be covered by the map above.
// Without it, a new unfrozen vector file could be added beside the fourteen
// and pass a test that only ever looked up names it already knew.
func TestGoldenVectorsFrozen(t *testing.T) {
	for name, want := range frozenVectorSHA256 {
		path := filepath.Join(goldenDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading frozen artefact %s: %v", path, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			t.Errorf("FREEZE BROKEN — %s has changed since commit a1e3779.\n"+
				"  recorded SHA-256 %s\n"+
				"  present  SHA-256 %s\n"+
				"This artefact is quarantined evidence: it is never regenerated and\n"+
				"never edited to satisfy a test. Restore it from the repository root\n"+
				"with `git checkout a1e3779 -- core/kw/testdata/%s` and report the\n"+
				"change.",
				path, want, got, name)
		}
	}

	present, err := filepath.Glob(filepath.Join(goldenDir, "*"))
	if err != nil {
		t.Fatalf("globbing %s: %v", goldenDir, err)
	}
	if len(present) == 0 {
		t.Fatalf("no artefacts found in %s — the vectors are missing", goldenDir)
	}
	for _, path := range present {
		if _, ok := frozenVectorSHA256[filepath.Base(path)]; !ok {
			t.Errorf("%s has no recorded SHA-256: every artefact in %s must be frozen by a commit that records its hash",
				path, goldenDir)
		}
	}
}

// THE COUNTED FRAME LENGTHS, transcribed from each vector file's own "COUNTED
// FRAME LENGTHS" block — two independent counts of the printed numbered
// ruler, and a third look at 600 dpi wherever a terminator cell was in doubt.
//
// They are LITERALS, read off the prose, and the vectors are the independent
// side: a length computed from the bytes it checks would prove nothing —
// except the two EX-590 one-character-P5 vectors below
// (ex590_set_menu002_brightness_3, ex590_answer_menu002_brightness_3), whose
// width EX-590.golden's own [E1] marks ASSUMED rather than counted.
//
// The two books agree on every one of these, which the AI-480 and MC-480
// headers say in as many words ("identical to the TS-590 book's AI lengths").
// Where a command's charts differ between the books it is spelt out below.
const (
	aiSetLen    = 4
	aiReadLen   = 3
	aiAnswerLen = 4

	idReadLen   = 3
	idAnswerLen = 6

	fvReadLen   = 3
	fvAnswerLen = 7

	tyReadLen   = 3
	tyAnswerLen = 6

	mcSetLen    = 6
	mcReadLen   = 3
	mcAnswerLen = 6

	mrReadLen   = 7
	mrAnswerLen = 50

	mwSetLen = 50
	// mw590EraseLen is the 42-position shape the 590 book's own note
	// describes — "If you do not specify one digit in P16 and execute all the
	// parameters from P4 to P15 set to 0, the channels specified by P2 and P3
	// will be erased" — read by the deriver's [B5] as P16 omitted entirely.
	// It is EVIDENCE OF WHAT THE BOOK PRINTS AND NOTHING MORE: this programme
	// builds no erase frame (the standing no-erase rule), and pinning the
	// vector's length here is not a licence to build it. The 480 book prints
	// no such note and its file emits no erase vector.
	mw590EraseLen = 42

	exReadLen = 10
	// exFixedBytes is the part of an EX Set/Answer frame the charts DO fix:
	// "EX", P1's three digits, P2's two, P3, P4 and the terminator — ten
	// bytes around a P5 both books call "(variable length)". The 590 book
	// draws P5 as eight characters (terminator in cell 18) and the 480 book
	// as two (cell 12), so the illustrated lengths differ between the books
	// while this constant does not; each vector below states its own P5
	// width, taken from that vector's field map.
	exFixedBytes = 10
)

// vectorSpec is one expected vector: its name, in file order, and the frame
// length the chart's ruler was counted to.
type vectorSpec struct {
	name   string
	length int
}

// goldenSpec is one vector file: which book's charts it was derived from and
// what it must contain.
type goldenSpec struct {
	file    string
	book    Book
	vectors []vectorSpec
}

// goldenSpecs is the whole roster, file by file. A vector added, removed or
// renamed fails here rather than quietly slipping past a leg that walks
// whatever it finds.
var goldenSpecs = []goldenSpec{
	{
		file: "AI-590.golden",
		book: Book590,
		vectors: []vectorSpec{
			{"ai590_set_off", aiSetLen},
			{"ai590_set_on_without_backup", aiSetLen},
			{"ai590_set_on_with_backup", aiSetLen},
			{"ai590_read", aiReadLen},
			{"ai590_answer_off", aiAnswerLen},
			{"ai590_answer_on_without_backup", aiAnswerLen},
			{"ai590_answer_on_with_backup", aiAnswerLen},
		},
	},
	{
		file: "AI-480.golden",
		book: Book480,
		vectors: []vectorSpec{
			{"ai480_set_off", aiSetLen},
			{"ai480_set_old_format_only", aiSetLen},
			{"ai480_set_extended_format_only", aiSetLen},
			{"ai480_set_both_formats", aiSetLen},
			{"ai480_read", aiReadLen},
			{"ai480_answer_off", aiAnswerLen},
			{"ai480_answer_old_format_only", aiAnswerLen},
			{"ai480_answer_extended_format_only", aiAnswerLen},
			{"ai480_answer_both_formats", aiAnswerLen},
		},
	},
	{
		file: "ID-590.golden",
		book: Book590,
		vectors: []vectorSpec{
			{"id590_read", idReadLen},
			{"id590_answer_ts590s", idAnswerLen},
			{"id590_answer_ts590sg", idAnswerLen},
		},
	},
	{
		file: "ID-480.golden",
		book: Book480,
		vectors: []vectorSpec{
			{"id480_read", idReadLen},
			{"id480_answer_ts480", idAnswerLen},
		},
	},
	{
		file: "FV-590.golden",
		book: Book590,
		vectors: []vectorSpec{
			{"fv590_read", fvReadLen},
			{"fv590_answer_v1_00", fvAnswerLen},
			{"fv590_answer_v2_00", fvAnswerLen},
			{"fv590_answer_v1_08", fvAnswerLen},
		},
	},
	{
		file: "TY-480.golden",
		book: Book480,
		vectors: []vectorSpec{
			{"ty480_read", tyReadLen},
			{"ty480_answer_ts480hx_200w", tyAnswerLen},
			{"ty480_answer_ts480sat_100w_at", tyAnswerLen},
			{"ty480_answer_japanese_50w", tyAnswerLen},
			{"ty480_answer_japanese_20w", tyAnswerLen},
		},
	},
	{
		file: "MC-590.golden",
		book: Book590,
		vectors: []vectorSpec{
			{"mc590_read", mcReadLen},
			{"mc590_set_ch03_zero_hundreds", mcSetLen},
			{"mc590_set_ch03_space_hundreds", mcSetLen},
			{"mc590_set_ch100_P00", mcSetLen},
			{"mc590_set_ch110_E00", mcSetLen},
			{"mc590_answer_ch03", mcAnswerLen},
			{"mc590_answer_ch100_P00", mcAnswerLen},
			{"mc590_answer_ch119", mcAnswerLen},
		},
	},
	{
		file: "MC-480.golden",
		book: Book480,
		vectors: []vectorSpec{
			{"mc480_read", mcReadLen},
			{"mc480_set_ch00", mcSetLen},
			{"mc480_set_ch03", mcSetLen},
			{"mc480_set_ch90_program_scan_lower", mcSetLen},
			{"mc480_set_ch99_top_of_range", mcSetLen},
			{"mc480_answer_ch03", mcAnswerLen},
			{"mc480_answer_ch99", mcAnswerLen},
		},
	},
	{
		file: "MR-590.golden",
		book: Book590,
		vectors: []vectorSpec{
			{"mr590_read_ch03_simplex_rx", mrReadLen},
			{"mr590_read_ch04_simplex_rx", mrReadLen},
			{"mr590_read_ch12_split_tx", mrReadLen},
			{"mr590_read_ch100_P00_simplex", mrReadLen},
			{"mr590_answer_ch03_simplex_name8", mrAnswerLen},
			{"mr590_answer_ch04_simplex_name3", mrAnswerLen},
			{"mr590_answer_ch12_split_tx_tone_on", mrAnswerLen},
		},
	},
	{
		file: "MR-480.golden",
		book: Book480,
		vectors: []vectorSpec{
			{"mr480_read_ch03_rx", mrReadLen},
			{"mr480_read_ch04_rx", mrReadLen},
			{"mr480_read_ch12_tx", mrReadLen},
			{"mr480_read_ch90_start_freq", mrReadLen},
			{"mr480_read_ch90_end_freq", mrReadLen},
			{"mr480_answer_ch03_rx_name8", mrAnswerLen},
			{"mr480_answer_ch04_rx_name3", mrAnswerLen},
			{"mr480_answer_ch12_tx_tone_on", mrAnswerLen},
		},
	},
	{
		file: "MW-590.golden",
		book: Book590,
		vectors: []vectorSpec{
			{"mw590_set_ch03_simplex_name8", mwSetLen},
			{"mw590_set_ch12_split_tone_on", mwSetLen},
			{"mw590_set_ch03_erase_no_P16", mw590EraseLen},
		},
	},
	{
		file: "MW-480.golden",
		book: Book480,
		vectors: []vectorSpec{
			{"mw480_set_ch03_rx_name8", mwSetLen},
			{"mw480_set_ch12_tx_tone_on", mwSetLen},
		},
	},
	{
		// The EX Set and Answer lengths are exFixedBytes + this vector's own
		// P5 width, each width read off the vector's field map: menu 001 is
		// the SG's "Power on message", eight characters; menu 002 is its
		// Display brightness — ASSUMED at one character. The chart itself
		// draws P5 as eight; EX-590.golden's [E1] says a width other than
		// that drawn 8 is ASSUMED from the menu-002 value legend, a table of
		// VALUES, not of widths, so the two menu-002 vectors below are an
		// inherited assumption, not a count off the field map.
		file: "EX-590.golden",
		book: Book590,
		vectors: []vectorSpec{
			{"ex590_read_menu000", exReadLen},
			{"ex590_read_menu001", exReadLen},
			{"ex590_read_menu002", exReadLen},
			{"ex590_read_menu087_last_590S", exReadLen},
			{"ex590_read_menu099_last_590SG", exReadLen},
			{"ex590_set_menu001_poweron_msg_8", exFixedBytes + 8},
			{"ex590_answer_menu001_poweron_msg_8", exFixedBytes + 8},
			{"ex590_set_menu002_brightness_3", exFixedBytes + 1},    // ASSUMED: [E1]
			{"ex590_answer_menu002_brightness_3", exFixedBytes + 1}, // ASSUMED: [E1]
		},
	},
	{
		// Menu 000 is the 480's Display brightness, one digit; menu 032 is
		// "Interval time for repeating the playback", one of the chart's
		// two-digit rows.
		//
		// The 480 book's EX chart and its own worked examples disagree about
		// P5's illustrated width — the chart draws two characters with ";" in
		// cell 12, the two printed examples are eleven characters with a
		// one-character P5 — which the file records as FINDING F2b. The roster
		// follows each vector's OWN documented shape rather than one length for
		// the command: the three vectors quoted from the worked examples
		// (menu 000, set and answer) are exFixedBytes+1, and the two drawn as
		// the chart draws it (menu 032, one of the legend's two-digit menus) are
		// exFixedBytes+2.
		file: "EX-480.golden",
		book: Book480,
		vectors: []vectorSpec{
			{"ex480_read_menu000", exReadLen},
			{"ex480_read_menu032", exReadLen},
			{"ex480_read_menu060_last", exReadLen},
			{"ex480_set_menu000_illumination_off", exFixedBytes + 1},
			{"ex480_set_menu000_brightness_3", exFixedBytes + 1},
			{"ex480_set_menu032_two_digit", exFixedBytes + 2},
			{"ex480_answer_menu032_two_digit", exFixedBytes + 2},
			{"ex480_answer_menu000_brightness_3", exFixedBytes + 1},
		},
	},
}

// totalVectors is leg G's whole size, spelt once so that a file quietly
// dropped from goldenSpecs is caught by something other than the same table.
const totalVectors = 82

// f1MRReadVectors are the four TS-590 MR Read frames whose last byte is a
// COLON, not the terminator.
//
// THIS IS FINDING F1 AND IT IS RECORDED, NOT RESOLVED. MR-590.golden's chart
// transcription reads "7  :   <-- the chart prints a COLON here, not a
// semicolon", verified at 600 dpi against the semicolon the same block's
// Answer chart prints at position 50, and the file states in as many words
// that "no vector with a SEMICOLON terminator is emitted for the MR Read
// frame … substituting ';' would be resolving it". The 480 book's MR Read
// chart prints a semicolon, so its five read vectors are not here.
//
// The consequence for the two legs below is exact and is asserted rather than
// tolerated: these four frames are REFUSED by the outbound gate (no
// terminator) and are not frames at all to the accumulator (nothing to split
// on). Both are pinned as the F1 exception, so that the day arbitration
// settles F1 the pins have to be revisited deliberately.
var f1MRReadVectors = map[string]bool{
	"mr590_read_ch03_simplex_rx":   true,
	"mr590_read_ch04_simplex_rx":   true,
	"mr590_read_ch12_split_tx":     true,
	"mr590_read_ch100_P00_simplex": true,
}

// goldenVector is one record of a *.golden file: the format is
// "name<TAB>frame" per line, with '#' comment lines and no other content.
type goldenVector struct {
	file  string // the file it came from, for failure messages
	line  int    // 1-indexed line number, likewise
	name  string
	frame string
}

// role is what the derivation says a vector IS: a frame the host builds and
// sends, or one the radio sends back. The two go through different halves of
// the codec, and putting an answer through the outbound gate would be
// asserting that this programme may emit it.
type role int

const (
	roleHostBuilt role = iota + 1
	roleAnswer
)

// roleOf reads the role out of the vector's own name, which every leg G file
// spells as "<cmd><book>_{set,read,answer}_…".
//
// It is derived rather than tabulated ON PURPOSE: the naming convention is
// then itself pinned, and a vector renamed into ambiguity is a Fatal instead
// of silently landing in whichever bucket a hand-written table happened to
// put it in.
func roleOf(t *testing.T, v goldenVector) role {
	t.Helper()
	set := strings.Contains(v.name, "_set")
	read := strings.Contains(v.name, "_read")
	answer := strings.Contains(v.name, "_answer")
	n := 0
	for _, b := range []bool{set, read, answer} {
		if b {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%s:%d: vector %q names %d of {_set,_read,_answer}; leg G's naming convention is exactly one, and this file's roles are read from it", v.file, v.line, v.name, n)
	}
	if answer {
		return roleAnswer
	}
	return roleHostBuilt
}

// loadGoldenVectors reads one vector file into its records, in file order.
//
// The frame is taken VERBATIM after the single tab — never trimmed. Trailing
// SPACE is significant frame content: the MR and MW answer vectors pad a
// short memory name to its eight positions with ASCII space (the files'
// [A4]/[B3] inherited assumptions) and the EX power-on-message vectors do the
// same to eight characters, so a convenience TrimSpace here would silently
// rewrite the evidence this file exists to compare against. The parser
// instead refuses anything that is not exactly one tab, and refuses a CR, so
// a file that acquired either fails loudly rather than being read
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

// TestGoldenVectors_RosterAndCountedLengths pins the roster and the counted
// chart lengths: which vectors each file holds, in what order, and how many
// bytes the printed ruler was counted to for each.
//
// The lengths come from the files' own prose and the frames from their
// records, so this leg binds a human COUNT to the BYTES the same human then
// wrote out — the one comparison in this file that does not need a codec at
// all, and the one that would catch a frame silently gaining or losing a
// position.
func TestGoldenVectors_RosterAndCountedLengths(t *testing.T) {
	seenFiles := map[string]bool{}
	total := 0
	for _, spec := range goldenSpecs {
		if seenFiles[spec.file] {
			t.Fatalf("%s appears twice in goldenSpecs", spec.file)
		}
		seenFiles[spec.file] = true

		t.Run(spec.file, func(t *testing.T) {
			got := loadGoldenVectors(t, spec.file)
			if len(got) != len(spec.vectors) {
				t.Fatalf("%s holds %d vectors, the roster says %d", spec.file, len(got), len(spec.vectors))
			}
			for i, want := range spec.vectors {
				v := got[i]
				if v.name != want.name {
					t.Errorf("%s:%d: vector %d is %q, the roster says %q", v.file, v.line, i, v.name, want.name)
					continue
				}
				if n := len(v.frame); n != want.length {
					t.Errorf("%s (%s:%d) is %d bytes, the COUNTED chart length is %d. THIS IS A STOP: the derivation's own ruler count and its own vector disagree.\n  frame %q",
						v.name, v.file, v.line, n, want.length, v.frame)
				}
			}
		})
		total += len(spec.vectors)
	}

	if total != totalVectors {
		t.Errorf("the roster holds %d vectors across %d files, leg G is %d", total, len(goldenSpecs), totalVectors)
	}
	// Every frozen *.golden must appear in the roster: without this a whole
	// file could be dropped from goldenSpecs and every leg above would pass
	// on the files that remained.
	for name := range frozenVectorSHA256 {
		if !strings.HasSuffix(name, ".golden") {
			continue
		}
		if !seenFiles[name] {
			t.Errorf("%s is a frozen vector file that the roster does not cover; every leg G file must be replayed", name)
		}
	}
}

// TestGoldenVectors_TheOutboundGateAdmitsEveryHostBuiltFrame is the replay
// through the codec that exists: every Set and Read vector is offered to the
// SHIPPED outbound gate for the book it was derived from, and must be
// admitted.
//
// IT IS THE ENVELOPE, NOT A GRAMMAR, and that is the whole claim. framing.go
// says so itself — Allow today "is the envelope alone … and it does not yet
// know which commands exist" — so a pass here means the frame satisfies the
// rules both books print about what a frame LOOKS like: a terminator, exactly
// one, as the last byte; at least two upper-case name bytes before it; every
// body byte 0x20..0x7E; never a radio-to-host token; never longer than
// DefaultMaxFrame. It does NOT mean the codec agrees the frame is a valid MR,
// MW or EX — nothing in this package can say that yet, which is what
// TestTODO_T8_ReplayEveryVectorThroughItsOwnCodec records.
//
// The gate is reached through NewFraming, the seam every caller uses, rather
// than through envelopeAllows directly: the pin is on the shipped behaviour.
//
// Two properties make this leg non-vacuous rather than a tautology over
// "printable bytes":
//
//   - The F1 vectors are asserted REFUSED, so the gate is demonstrably
//     discriminating on these very frames rather than waving everything
//     through.
//   - The mandatory literal SPACE the 590 book prints inside MC's P1 below
//     channel 100 (590:1334-1337) is carried by mc590_set_ch03_space_hundreds
//     and must be ADMITTED — the case a gate written as "alphanumerics only"
//     would refuse.
//
// mw590_set_ch03_erase_no_P16, the 590 book's erase shape, is also asserted
// ADMITTED here — a true statement about the envelope, which is not a
// grammar; this programme never builds that frame, and T7's per-command gate
// is what will refuse it.
func TestGoldenVectors_TheOutboundGateAdmitsEveryHostBuiltFrame(t *testing.T) {
	hostBuilt, refused := 0, 0
	for _, spec := range goldenSpecs {
		f := kwFraming(t, spec.book)
		for _, v := range loadGoldenVectors(t, spec.file) {
			if roleOf(t, v) != roleHostBuilt {
				continue
			}
			hostBuilt++
			got := f.Allow([]byte(v.frame))
			if f1MRReadVectors[v.name] {
				refused++
				if got {
					t.Errorf("%s (%s:%d) was ADMITTED by the %v gate: %q ends in a colon, not the terminator, and FINDING F1 is recorded and unresolved. Admitting it would emit a frame no document describes.",
						v.name, v.file, v.line, spec.book, v.frame)
				}
				continue
			}
			if !got {
				t.Errorf("GOLDEN-VS-GATE MISMATCH — THIS IS A STOP, NOT A TEST TO ADJUST.\n"+
					"  vector  %s (%s:%d)\n"+
					"  book    %v\n"+
					"  frame   %q (%d bytes)\n"+
					"The outbound gate refused a frame the manual's own position chart\n"+
					"prints. Either the derivation or the envelope misreads the book;\n"+
					"the vectors are frozen (SHA-256s recorded at commit a1e3779) and\n"+
					"neither may be edited to settle it.",
					v.name, v.file, v.line, spec.book, v.frame, len(v.frame))
			}
		}
	}
	// Vacuity guards: a role predicate that classified everything as an
	// answer, or an F1 set that had drifted off its vectors, would leave
	// this test asserting nothing.
	if hostBuilt == 0 {
		t.Fatal("no host-built vector was offered to the gate — roleOf classified every vector as an answer and this test proved nothing")
	}
	if refused != len(f1MRReadVectors) {
		t.Errorf("the F1 exception fired on %d vectors, want %d — the recorded exception names vectors that are no longer host-built reads", refused, len(f1MRReadVectors))
	}
}

// TestGoldenVectors_TheAccumulatorReassemblesEveryAnswer is the inbound half
// of the same replay: every radio-sent vector is pushed through this
// package's FrameAccumulator ONE BYTE AT A TIME and must come back out as
// exactly one frame, byte for byte.
//
// A byte at a time is the point. A serial port splits a 50-byte MR answer
// across reads routinely, and an accumulator that only ever saw whole frames
// would be tested by nothing here. The whole-file leg then pushes every
// answer in one chunk and requires them all back in order, which is the
// coalescing case.
//
// Each answer is also asserted NOT to be a rejection or a stream-health
// token: "?;", "E;" and "O;" travel radio-to-host too, and an answer vector
// that collided with one would be evidence the engine could never
// distinguish.
func TestGoldenVectors_TheAccumulatorReassemblesEveryAnswer(t *testing.T) {
	answers := 0
	for _, spec := range goldenSpecs {
		t.Run(spec.file, func(t *testing.T) {
			var whole []byte
			var wantOrder []string
			for _, v := range loadGoldenVectors(t, spec.file) {
				if roleOf(t, v) != roleAnswer {
					continue
				}
				answers++
				whole = append(whole, v.frame...)
				wantOrder = append(wantOrder, v.frame)

				if IsRejection([]byte(v.frame)) || streamErrorToken([]byte(v.frame)) != "" {
					t.Errorf("%s (%s:%d) IS a radio-to-host token (%q); an answer vector that collides with \"?;\", \"E;\" or \"O;\" is one the engine could never tell apart from a refusal or a line fault",
						v.name, v.file, v.line, v.frame)
				}

				acc := NewFrameAccumulator(0)
				var out [][]byte
				for i := 0; i < len(v.frame); i++ {
					frames, err := acc.Push([]byte(v.frame[i : i+1]))
					if err != nil {
						t.Fatalf("%s (%s:%d): Push of byte %d: %v", v.name, v.file, v.line, i, err)
					}
					out = append(out, frames...)
				}
				if len(out) != 1 {
					t.Errorf("%s (%s:%d): a byte-at-a-time push yielded %d frames, want exactly 1", v.name, v.file, v.line, len(out))
					continue
				}
				if got := string(out[0]); got != v.frame {
					t.Errorf("GOLDEN-VS-ACCUMULATOR MISMATCH — THIS IS A STOP.\n"+
						"  vector %s (%s:%d)\n"+
						"  golden %q (%d bytes)\n"+
						"  codec  %q (%d bytes)",
						v.name, v.file, v.line, v.frame, len(v.frame), got, len(got))
				}
			}
			if len(wantOrder) == 0 {
				return // MW-590 and MW-480 print no Answer chart at all
			}
			acc := NewFrameAccumulator(0)
			frames, err := acc.Push(whole)
			if err != nil {
				t.Fatalf("%s: pushing every answer as one chunk: %v", spec.file, err)
			}
			if len(frames) != len(wantOrder) {
				t.Fatalf("%s: pushing %d concatenated answers yielded %d frames", spec.file, len(wantOrder), len(frames))
			}
			for i, want := range wantOrder {
				if got := string(frames[i]); got != want {
					t.Errorf("%s: coalesced frame %d is %q, want %q", spec.file, i, got, want)
				}
			}
		})
	}
	if answers == 0 {
		t.Fatal("no answer vector reached the accumulator — roleOf classified every vector as host-built and this test proved nothing")
	}
}

// TestGoldenVectors_TheF1ReadFramesAreNotFramesToTheSplitter is the other
// half of the F1 pin: the four colon-terminated MR Read vectors are not
// merely refused by the gate, they are not frames at all to this package's
// splitter — every byte comes back as rest, waiting for a terminator that
// never arrives.
//
// Recording it makes the finding's consequence concrete rather than
// theoretical: a session that emitted one of these would then read forever.
func TestGoldenVectors_TheF1ReadFramesAreNotFramesToTheSplitter(t *testing.T) {
	seen := 0
	for _, v := range loadGoldenVectors(t, "MR-590.golden") {
		if !f1MRReadVectors[v.name] {
			continue
		}
		seen++
		frames, rest := SplitFrames([]byte(v.frame))
		if len(frames) != 0 {
			t.Errorf("%s (%s:%d): SplitFrames found %d frame(s) in %q; FINDING F1 says position 7 is a colon, so there is no terminator to split on", v.name, v.file, v.line, len(frames), v.frame)
		}
		if string(rest) != v.frame {
			t.Errorf("%s (%s:%d): SplitFrames returned rest %q, want the whole vector %q", v.name, v.file, v.line, rest, v.frame)
		}
	}
	if seen != len(f1MRReadVectors) {
		t.Errorf("MR-590.golden holds %d of the %d recorded F1 vectors", seen, len(f1MRReadVectors))
	}
}

// TestGoldenVectors_InitSequenceIsTheAI0Vector is the only WHOLE-FRAME
// builder-versus-golden byte comparison this package can make today.
// (TestGoldenVectors_TheEXAddressFieldIsWhatWireRenders below makes the same
// comparison for one FIELD across seventeen more vectors; there is no third.)
//
// InitSequence is the only frame-producing code in core/kw: it builds "AI0;",
// the single AI state this programme ever emits, and each book's own AI file
// prints that frame as its ai*_set_off vector. Binding the two closes the
// loop the rest of this file has to defer — the bytes the codec produces
// against the bytes a quarantined agent read off the chart — for the one
// command where both sides exist.
func TestGoldenVectors_InitSequenceIsTheAI0Vector(t *testing.T) {
	for _, tc := range []struct {
		book   Book
		file   string
		vector string
	}{
		{Book590, "AI-590.golden", "ai590_set_off"},
		{Book480, "AI-480.golden", "ai480_set_off"},
	} {
		t.Run(tc.book.String(), func(t *testing.T) {
			var want goldenVector
			for _, v := range loadGoldenVectors(t, tc.file) {
				if v.name == tc.vector {
					want = v
					break
				}
			}
			if want.name == "" {
				t.Fatalf("%s holds no vector named %q", tc.file, tc.vector)
			}

			seq := kwFraming(t, tc.book).InitSequence()
			if len(seq) != 1 {
				t.Fatalf("InitSequence for %v returned %d commands, want exactly 1", tc.book, len(seq))
			}
			if got := string(seq[0].Bytes()); got != want.frame {
				t.Errorf("GOLDEN-VS-CODEC MISMATCH — THIS IS A STOP, NOT A TEST TO ADJUST.\n"+
					"  vector    %s (%s:%d)\n"+
					"  built by  NewFraming(%v).InitSequence()\n"+
					"  golden    %q (%d bytes)\n"+
					"  codec     %q (%d bytes)",
					want.name, want.file, want.line, tc.book, want.frame, len(want.frame), got, len(got))
			}
		})
	}
}

// exVectorMenu reads the three-digit menu number out of an EX vector's name,
// which every leg G EX record spells "ex<book>_{read,set,answer}_menu<NNN>…".
//
// Read from the NAME rather than from the frame on purpose: the frame's
// address field is the thing under test, so taking the expected value from it
// would make TestGoldenVectors_TheEXAddressFieldIsWhatWireRenders a
// tautology. The naming convention is thereby pinned too — an EX vector
// renamed out of it is a Fatal, not a silently skipped row.
var exVectorMenuRE = regexp.MustCompile(`_menu(\d{3})`)

func exVectorMenu(t *testing.T, v goldenVector) uint8 {
	t.Helper()
	m := exVectorMenuRE.FindStringSubmatch(v.name)
	if m == nil {
		t.Fatalf("%s:%d: EX vector %q does not name its menu number as \"_menu<NNN>\"; leg G's EX naming convention is what this leg's expected address is read from", v.file, v.line, v.name)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 0 || n > 255 {
		t.Fatalf("%s:%d: EX vector %q names menu %q, which is not a menu number this address type can hold", v.file, v.line, v.name, m[1])
	}
	return uint8(n)
}

// TestGoldenVectors_TheEXAddressFieldIsWhatWireRenders replays the ONE field
// a shipped renderer produces against every EX vector's own bytes.
//
// EXAddress.Wire is the only wire-rendering function in core/kw that is not
// InitSequence: it emits P1 as three zero-padded digits, and both books draw
// that field in positions 3-5 of every EX frame — Read, Set and Answer alike
// (EX-590.golden's and EX-480.golden's field maps, "pos 3-5 P1 (3 characters,
// menu number)"). So for all seventeen EX vectors the golden bytes and a
// shipped builder can be compared directly, which is what "replay through the
// codec that exists" means for this command today.
//
// The four positions the charts print as FIXED are asserted with it, from the
// same field maps and the same "Always" legends: "EX" in 1-2, P2 "00" in 6-7,
// P3 '0' in 8 and P4 '0' in 9, on BOTH books ("00: Always 00 for the TS-480"
// / the 590 book's common legend). They are not Wire's output and are not
// claimed to be; they are the frame's other fixed cells, and a vector that
// lost one would otherwise reach T8 unremarked.
//
// WHAT THIS DOES NOT SAY: nothing here builds an EX frame. P5's width comes
// from the per-radio inventory, which this package cannot see (the import
// cycle TestTODO_T8_ReplayEveryVectorThroughItsOwnCodec records), so the
// whole-frame EX comparison stays task 8's.
func TestGoldenVectors_TheEXAddressFieldIsWhatWireRenders(t *testing.T) {
	checked := 0
	for _, spec := range goldenSpecs {
		if !strings.HasPrefix(spec.file, "EX-") {
			continue
		}
		t.Run(spec.file, func(t *testing.T) {
			for _, v := range loadGoldenVectors(t, spec.file) {
				checked++
				if len(v.frame) < exFixedBytes {
					t.Fatalf("%s (%s:%d) is %d bytes; every EX frame both books draw is at least the %d fixed ones", v.name, v.file, v.line, len(v.frame), exFixedBytes)
				}
				want := EXAddress{P1: exVectorMenu(t, v)}.Wire()
				if got := v.frame[2:5]; got != want {
					t.Errorf("GOLDEN-VS-CODEC MISMATCH — THIS IS A STOP, NOT A TEST TO ADJUST.\n"+
						"  vector    %s (%s:%d)\n"+
						"  built by  kw.EXAddress{P1: %d}.Wire()\n"+
						"  golden    %q (positions 3-5 of %q)\n"+
						"  codec     %q",
						v.name, v.file, v.line, exVectorMenu(t, v), got, v.frame, want)
				}
				for _, fixed := range []struct {
					what  string
					lo    int
					hi    int
					want  string
					cites string
				}{
					{"the command name", 0, 2, "EX", "positions 1-2"},
					{"P2", 5, 7, "00", `"Always 00"`},
					{"P3", 7, 8, "0", `"Always 0"`},
					{"P4", 8, 9, "0", `"Always 0"`},
				} {
					if got := v.frame[fixed.lo:fixed.hi]; got != fixed.want {
						t.Errorf("%s (%s:%d): %s is %q, the chart prints %q there (%s)", v.name, v.file, v.line, fixed.what, got, fixed.want, fixed.cites)
					}
				}
			}
		})
	}
	// A vacuity guard of the same shape as the others: an EX file renamed out
	// of the "EX-" prefix would leave this leg asserting nothing at all.
	if want := len(exVectorNames()); checked != want {
		t.Errorf("this leg replayed %d EX vectors, the roster holds %d", checked, want)
	}
}

// exVectorNames is the roster's own EX vectors, so the guard above counts
// against the roster rather than against the files it just walked.
func exVectorNames() []string {
	var out []string
	for _, spec := range goldenSpecs {
		if !strings.HasPrefix(spec.file, "EX-") {
			continue
		}
		for _, v := range spec.vectors {
			out = append(out, v.name)
		}
	}
	return out
}

// answerMatcherSpecs is, per command, the answer shape a read's CommandSpec
// declares — the prefix's own command bytes and the EXACT answer length, or 0
// where the answer is variable.
//
// The lengths are the SAME literals the roster above transcribed from the
// files' "COUNTED FRAME LENGTHS" blocks, referenced rather than re-typed, so
// there is one count of each printed ruler in this file and not two. EX is 0
// because its P5 is "(variable length)" with no printed ceiling on either
// book (590:555-556, 480:409-411) — matcher.go's own comment says this is the
// single user of that branch, and this leg is where that claim meets the
// evidence.
var answerMatcherSpecs = map[string]int{
	"AI": aiAnswerLen,
	"ID": idAnswerLen,
	"FV": fvAnswerLen,
	"TY": tyAnswerLen,
	"MC": mcAnswerLen,
	"MR": mrAnswerLen,
	"EX": 0,
}

// TestGoldenVectors_EveryAnswerIsMatchedByItsOwnReadsMatcher replays every
// radio-sent vector through PrefixLenMatcher — the answer-correlation half of
// the codec, and the last shipped piece of it these vectors can reach.
//
// Three properties, and the second and third are what stop it being a
// restatement of the length roster:
//
//   - Its own read's matcher ACCEPTS it.
//   - Every OTHER command's matcher REFUSES it. A matcher that accepted
//     anything would correlate an ID answer as an MR record and hand the
//     engine 6 bytes where 50 were expected.
//   - THE COMMAND'S OWN READ FRAME is refused by that command's answer
//     matcher. This is the leg that binds exactLen rather than the prefix:
//     "ID;" and "ID023;" share every byte the prefix test looks at and
//     differ only in length, and a matcher that dropped the length test
//     would correlate the host's own echoed read as the radio's answer. It
//     was added after removing the exactLen branch from PrefixLenMatcher
//     left every other assertion in this test green.
//   - THE EX FULL-ADDRESS OBLIGATION, fired on real evidence. matcher.go's
//     comment states it as a caller obligation the function cannot enforce:
//     an EX matcher built with the bare "EX" prefix correlates a DIFFERENT
//     address's answer, and only "EX" + the three-digit address refuses it.
//     The two EX answer vectors on the 590 row carry different addresses
//     (menu 001 and menu 002), so this file can demonstrate the confusion on
//     the book's own frames rather than on invented ones.
func TestGoldenVectors_EveryAnswerIsMatchedByItsOwnReadsMatcher(t *testing.T) {
	answers := 0
	for _, spec := range goldenSpecs {
		t.Run(spec.file, func(t *testing.T) {
			for _, v := range loadGoldenVectors(t, spec.file) {
				if roleOf(t, v) != roleAnswer {
					continue
				}
				answers++

				cmd := v.frame[:2]
				exactLen, known := answerMatcherSpecs[cmd]
				if !known {
					t.Fatalf("%s (%s:%d): the frame opens %q, which is not a command this milestone reads; answerMatcherSpecs must name every answer leg G holds", v.name, v.file, v.line, cmd)
				}
				// The full-address prefix for EX, the bare command name for
				// every other answer, exactly as matcher.go requires.
				prefix := cmd
				if cmd == "EX" {
					prefix = v.frame[:5]
				}
				if !PrefixLenMatcher(prefix, exactLen)(([]byte)(v.frame)) {
					t.Errorf("GOLDEN-VS-CODEC MISMATCH — THIS IS A STOP, NOT A TEST TO ADJUST.\n"+
						"  vector   %s (%s:%d)\n"+
						"  matcher  PrefixLenMatcher(%q, %d)\n"+
						"  golden   %q (%d bytes)\n"+
						"The answer the manual's own chart prints is not correlated as the\n"+
						"answer to its own read.",
						v.name, v.file, v.line, prefix, exactLen, v.frame, len(v.frame))
				}
				for otherCmd, otherLen := range answerMatcherSpecs {
					if otherCmd == cmd {
						continue
					}
					if PrefixLenMatcher(otherCmd, otherLen)(([]byte)(v.frame)) {
						t.Errorf("%s (%s:%d): %q is correlated by the %s read's matcher as well as its own; an answer two reads both claim is one the engine could deliver to the wrong caller",
							v.name, v.file, v.line, v.frame, otherCmd)
					}
				}
			}
		})
	}
	if answers == 0 {
		t.Fatal("no answer vector reached a matcher — roleOf classified every vector as host-built and this test proved nothing")
	}

	// The length half, on the read frames the same files print. EX is excluded
	// because its answer length is variable BY DECLARATION and a full-address
	// EX matcher genuinely does accept the 10-byte read frame that opens with
	// the same five bytes — a real property of this command, recorded here
	// rather than asserted away.
	//
	// This subtest covers reads only. It does not follow that a Set could
	// never be mistaken for an Answer: ai480_set_off/ai480_answer_off,
	// mc480_set_ch03/mc480_answer_ch03 and
	// ex480_set_menu032_two_digit/ex480_answer_menu032_two_digit are each the
	// same bytes on both sides, so an echoed Set on those three commands
	// WOULD be correlated as the answer. Nothing in this package's matchers
	// guards against that; today it is A25's assumption that Kenwood radios
	// do not echo that keeps it from mattering.
	t.Run("a_commands_own_read_frame_is_not_its_answer", func(t *testing.T) {
		reads := 0
		for _, spec := range goldenSpecs {
			for _, v := range loadGoldenVectors(t, spec.file) {
				if roleOf(t, v) != roleHostBuilt || !strings.Contains(v.name, "_read") {
					continue
				}
				cmd := v.frame[:2]
				exactLen, known := answerMatcherSpecs[cmd]
				if !known || exactLen == 0 {
					continue
				}
				reads++
				if PrefixLenMatcher(cmd, exactLen)(([]byte)(v.frame)) {
					t.Errorf("%s (%s:%d): the %s read frame %q (%d bytes) is correlated by the %s ANSWER matcher, whose counted answer length is %d — the host's own echoed read would be delivered as the radio's reply",
						v.name, v.file, v.line, cmd, v.frame, len(v.frame), cmd, exactLen)
				}
			}
		}
		if reads == 0 {
			t.Fatal("no fixed-length command's read frame reached its answer matcher — this leg proved nothing")
		}
	})

	// The EX obligation, on the two 590 answers the book prints at different
	// addresses.
	t.Run("the_EX_matcher_needs_the_WHOLE_address", func(t *testing.T) {
		var exAnswers []goldenVector
		for _, v := range loadGoldenVectors(t, "EX-590.golden") {
			if roleOf(t, v) == roleAnswer {
				exAnswers = append(exAnswers, v)
			}
		}
		if len(exAnswers) < 2 {
			t.Fatalf("EX-590.golden holds %d answer vectors; this leg needs two at DIFFERENT addresses", len(exAnswers))
		}
		ours, foreign := exAnswers[0], exAnswers[1]
		if ours.frame[:5] == foreign.frame[:5] {
			t.Fatalf("%s and %s carry the same address %q; the confusion this leg demonstrates needs two", ours.name, foreign.name, ours.frame[:5])
		}

		full := PrefixLenMatcher(ours.frame[:5], 0)
		if !full(([]byte)(ours.frame)) {
			t.Errorf("the full-address matcher for %q refuses its own answer %s", ours.frame[:5], ours.name)
		}
		if full(([]byte)(foreign.frame)) {
			t.Errorf("the full-address matcher for %q correlated %s (%q), a DIFFERENT menu's answer", ours.frame[:5], foreign.name, foreign.frame)
		}
		// The negative space: the bare prefix is what a caller must not use,
		// and this is the evidence that it would silently return the wrong
		// address's data rather than merely being untidy.
		if !PrefixLenMatcher("EX", 0)(([]byte)(foreign.frame)) {
			t.Errorf("a bare \"EX\" matcher refused %s; matcher.go's full-address obligation rests on the bare prefix accepting a foreign address, and this evidence no longer shows it", foreign.name)
		}
	})
}

// TestTODO_T8_ReplayEveryVectorThroughItsOwnCodec is the deliberate gap, left
// SKIPPED so that it is visible in `go test -v` rather than recorded only in
// prose.
//
// WHAT IS MISSING AND WHY. When this file was written, core/kw held the
// envelope and nothing else: no builder and no parser for any of the eight
// grammars this milestone specifies (ID read, AI read/set, FV read, TY read,
// MC read/set, MR read, MW set, EX read). Tasks 6, 7 and 8 build them. Until
// they exist, every vector's per-command replay — build the frame from its
// documented fields and compare the bytes; parse the answer and compare the
// fields — is unwritable, and the fourteen files are held by their SHA-256s
// (TestGoldenVectorsFrozen) and by the four legs above instead.
//
// WHAT TASK 8 MUST ADD, per file:
//
//	AI-590, AI-480    the AI Set builder and the AI Answer parser (note the
//	                  books' value sets DIFFER — 0/2/4 against 0/1/2/3,
//	                  finding F9 — so one builder may not serve both)
//	ID-590, ID-480     the ID read frame and the three-digit identity answer
//	FV-590             the FV read frame and the version answer, including
//	                   the unparseable case the driver must survive
//	TY-480             the TY read frame and its four answers
//	MC-590, MC-480     the MC read and Set frames, including the 590's P1
//	                   space/zero pair below channel 100
//	MR-590, MR-480     the MR read frame and the 50-byte answer record, field
//	                   by field — and the F1 colon must be settled before the
//	                   590's read builder can be written at all
//	MW-590, MW-480     the 50-byte MW Set record (this programme builds NO
//	                   erase frame; mw590_set_ch03_erase_no_P16 is evidence of
//	                   what the book prints, never a shape to emit)
//	EX-590, EX-480     the EX read frame and the variable-width Set/Answer,
//	                   whose P5 width comes from the per-radio inventory —
//	                   which is why that join cannot live in this file:
//	                   core/kw/ts590 and core/kw/ts480 import core/kw, so a
//	                   test here that read their inventories would be an
//	                   import cycle.
func TestTODO_T8_ReplayEveryVectorThroughItsOwnCodec(t *testing.T) {
	t.Skip(fmt.Sprintf("TODO(T8): %d vectors across %d files are frozen and envelope-replayed, but no per-command builder or parser exists in core/kw yet; see this test's comment for what each file needs", totalVectors, len(goldenSpecs)))
}
