// SPDX-License-Identifier: GPL-3.0-or-later

package kw

import (
	"crypto/sha256"
	"encoding/hex"
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
// # WHAT THIS FILE REPLAYS
//
// IT WAS WRITTEN IN TWO PASSES, AND THE SEAM IS STILL VISIBLE. When task 10
// wrote it, core/kw held the ENVELOPE and nothing else — frames, the ';'
// splitter, the accumulator, the typed errors, the outbound gate and the
// framing adapter — so the first five legs are all envelope legs, and a
// SKIPPED TestTODO_T8_ReplayEveryVectorThroughItsOwnCodec stood at the end
// naming per file what could not yet be written. Tasks 6, 7 and 8 built the
// eight grammars and the two layouts; the sixth leg is that TODO, written,
// and the TODO itself is gone rather than left standing beside its own
// completion.
//
// The envelope legs are NOT superseded by the per-command one and are not
// folded into it. They assert a different thing — that these bytes satisfy
// the rules both books print about what a frame LOOKS like, independently of
// whether any grammar recognises them — and that statement survives a
// grammar being widened, which is when it matters most.
//
//  1. HASH-FREEZES all fifteen artefacts (TestGoldenVectorsFrozen).
//  2. Pins the ROSTER — every file's vectors, in order, with the frame length
//     the deriver COUNTED off the printed ruler
//     (TestGoldenVectors_RosterAndCountedLengths). The lengths are literals
//     transcribed from each file's own "COUNTED FRAME LENGTHS" block, not
//     measured from the bytes they check.
//  3. Replays every vector through THE ENVELOPE — five legs, plus the F1
//     splitter leg
//     (TestGoldenVectors_TheF1ReadFramesAreNotFramesToTheSplitter):
//     the outbound gate for the host-built frames
//     (TestGoldenVectors_TheOutboundGateAdmitsEveryHostBuiltFrame); the frame
//     accumulator for every radio-sent one
//     (TestGoldenVectors_TheAccumulatorReassemblesEveryAnswer);
//     InitSequence against the AI0; vector each book prints
//     (TestGoldenVectors_InitSequenceIsTheAI0Vector); EXAddress.Wire against
//     the address field of all seventeen EX vectors
//     (TestGoldenVectors_TheEXAddressFieldIsWhatWireRenders); and
//     PrefixLenMatcher against every answer vector
//     (TestGoldenVectors_EveryAnswerIsMatchedByItsOwnReadsMatcher), the
//     answer-correlation half of the codec.
//  4. Replays every vector THROUGH ITS OWN GRAMMAR
//     (TestGoldenVectors_EveryVectorReplaysThroughItsOwnCodec): each of the
//     eighty-two is BUILT byte for byte, PARSED field by field, REFUSED with
//     its reason, or recorded as F1, and the walk requires the roster and the
//     disposition table to be the same set, so "complete" is mechanical.
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
// IT IS THE ENVELOPE, NOT A GRAMMAR, and that is the whole claim. The gate
// under test here is NewFraming's Allow, which framing.go's own doc says is
// the envelope alone: it knows the book, not the layout, so it cannot ask
// whether a frame is a valid MR, MW or EX — that per-command question is
// layoutFraming's (NewFramingFor), reached only through a configured
// Layout, and this test deliberately does not go through it. So a pass here
// means the frame satisfies the rules both books print about what a frame
// LOOKS like: a terminator, exactly one, as the last byte; at least two
// upper-case name bytes before it; every body byte 0x20..0x7E; never a
// radio-to-host token; never longer than DefaultMaxFrame. It does NOT mean
// the codec agrees the frame is a valid MR, MW or EX — this leg cannot say
// that: the per-command REPLAY — build from fields, compare bytes — is
// TestGoldenVectors_EveryVectorReplaysThroughItsOwnCodec, a separate leg,
// and this one is the gate check alone.
//
// The gate is reached through NewFraming, not NewFramingFor: the pin is on
// the envelope NewFraming ships, not on the layout-narrowed gate a driver
// actually uses.
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
// grammar; this programme never builds that frame, and the per-command gate
// T7 built (layoutFraming.Allow, reached through NewFramingFor) is what
// refuses it. TestNewFramingFor_PutsTheGrammarsInFrontOfTheEnvelope pins
// exactly this pair on this same 42-byte shape: the book-only framing
// admits it and the layout-bearing one refuses it.
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
// lost one would otherwise reach the per-command leg unremarked.
//
// WHAT THIS DOES NOT SAY: nothing here builds a whole EX frame. That is
// TestGoldenVectors_EveryVectorReplaysThroughItsOwnCodec's, and its P5 widths
// are LITERALS read off each vector's own field map rather than an inventory
// lookup — core/kw/ts590 and core/kw/ts480 import this package, so a test
// here that read their inventories would be an import cycle.
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

// ---------------------------------------------------------------------------
// THE PER-COMMAND REPLAY — what TestTODO_T8_ReplayEveryVectorThroughItsOwnCodec
// named, now written.
// ---------------------------------------------------------------------------
//
// That test stood here, SKIPPED, listing per file the replays task 8 had to
// add once tasks 6 and 7 built the eight grammars. They exist, so the TODO is
// GONE rather than left standing beside its own completion: a skipped test
// that has been done is worse than no test at all, because `go test -v` keeps
// reporting a gap that closed.
//
// # What a replay is, and why every vector has exactly one
//
// Each of the eighty-two vectors goes through the ONE thing this codec can
// honestly do with it. The walk below requires every vector in the roster to
// have exactly one entry in perCommandReplays and every entry to name a
// vector in the roster, so "complete" is MECHANICAL rather than a claim: a
// vector added, renamed or left out fails here.
//
//	BUILT    a builder is handed the vector's own documented fields and its
//	         output must equal the vector BYTE FOR BYTE.
//	PARSED   a parser decodes it, and every field is compared against a
//	         literal transcribed from that vector's own field map.
//	REFUSED  this codec builds no such frame and the layout's own outbound
//	         gate must refuse it. The reason is stated per vector and is
//	         never "not implemented": it is a decision or an ASSUMED-register
//	         entry, and a refusal whose reason had quietly gone would fail
//	         here rather than pass.
//	F1       the four TS-590 MR Read frames whose last byte is a COLON, which
//	         is RECORDED AND NOT RESOLVED. See f1MRReadVectors.
//
// A vector may be both BUILT and PARSED — an MC Set naming ordinary memory is
// byte-identical to the answer a radio sitting on that channel sends — and
// the entry then does both and counts as one.
//
// # The layouts these replays run against
//
// core/kw's OWN fixtures (testlayouts_test.go), never the shipping values in
// core/kw/ts590 and core/kw/ts480: those packages import this one, so a test
// here that reached for them would be an import cycle. The 590 files run
// against layout590SG, which is the book's own row for every command these
// vectors exercise; where the SIBLING differs — the MC vectors naming 110 and
// 119, which A12 puts outside the TS-590S's space — the entry consults
// layout590S as well and says so.
//
// # Three pieces the TODO named do not exist, and each absence is a design
//
//  1. THERE IS NO AI ANSWER PARSER, and there is no state for one to report.
//     This codec builds exactly one AI frame, "AI0;" (ai.go), because the
//     session disables Auto Information at open and never asks what the state
//     was. So the AI answer vectors are dispositioned by the GATE: "AI0;" is
//     one of the gate's two disclosed answer-admissions, being byte-identical
//     to the Set this codec really does build, and every other AI value in
//     either book's legend is refused. Finding F9 — that the two books' P1
//     legends differ, 0/2/4 against 0/1/2/3 — is why the refused list is
//     spelt per file rather than shared.
//  2. THERE IS NO EX SET BUILDER. This milestone reads the menu surface and
//     writes none of it (allowlist.go's validEXRead), so the EX Set vectors
//     are REFUSED — and their bytes are also the ANSWER shape, which is
//     exactly why admitting them would let a captured menu reply be written
//     back into a radio. Each Set entry therefore refuses the frame outbound
//     AND decodes the same bytes inbound.
//  3. THERE IS NO WAY TO ASK FOR A SPLIT WRITE, OR FOR P1='1' ON ORDINARY
//     MEMORY. P1 is derived from the SLOT'S CLASS and never chosen freely
//     (M9, Slot.P1), and A9 records that an MR with P1=1 on a simplex channel
//     is not safe to send blind. Five vectors turn on this — mr590's split
//     read, mr480's TX read and its 90-99 end-frequency read, and both split
//     MW Sets — and every one of them is a REFUSED with that reason, not a
//     replay this file could not be bothered to write.
//
// # The EX widths, and the import this file cannot have
//
// ParseEXAnswer bounds P5 by the inventory row's own Digits (A19), and the
// inventories live in the two model packages. The widths below are LITERALS
// read off each vector's own field map — the same numbers the roster's length
// constants are built from — so this leg needs no inventory and makes no
// claim about membership, which stays the per-row inventory's business.
//
// # Hardware status
//
// UNVERIFIED, for all eighty-two. Green here means this codec and one
// quarantined reading of the two books agree, and NOT that any radio accepts
// or sends these bytes (A19, A27).

// replay is one vector's disposition: what this codec does with it, and the
// kind that disposition counts as for the non-vacuity guard below.
type replay struct {
	kind  string
	check func(t *testing.T, l Layout, frame string)
}

// The four kinds, named once so a typo in a table entry cannot invent a
// fifth and pass the coverage guard.
const (
	kindBuilt   = "BUILT byte for byte"
	kindParsed  = "PARSED field by field"
	kindRefused = "REFUSED, with its reason"
	kindF1      = "F1: recorded, not resolved"
)

// replayLayout is the fixture a file's vectors replay against. See the
// section comment for why it is core/kw's own and not the model packages'.
func replayLayout(t *testing.T, book Book) Layout {
	t.Helper()
	switch book {
	case Book590:
		return layout590SG()
	case Book480:
		return layout480()
	}
	t.Fatalf("no replay layout for book %v", book)
	return Layout{}
}

// mustSlot (builders_test.go) resolves a number against a layout or fails
// the test; it is the package's own helper and is not repeated here.

// builtBy asserts that build's output is the vector, byte for byte.
func builtBy(what string, build func(t *testing.T, l Layout) (Command, error)) replay {
	return replay{kind: kindBuilt, check: func(t *testing.T, l Layout, frame string) {
		t.Helper()
		cmd, err := build(t, l)
		if err != nil {
			t.Fatalf("GOLDEN-VS-CODEC MISMATCH — THIS IS A STOP.\n  %s refused to build a frame the manual's own chart prints: %v", what, err)
		}
		if got := string(cmd.Bytes()); got != frame {
			t.Errorf("GOLDEN-VS-CODEC MISMATCH — THIS IS A STOP, NOT A TEST TO ADJUST.\n"+
				"  builder %s\n"+
				"  built   %q (%d bytes)\n"+
				"  vector  %q (%d bytes)\n"+
				"Either the hand derivation or this builder misreads the book. The\n"+
				"vectors are frozen (SHA-256s at commit a1e3779) and are never edited\n"+
				"to settle it; the arbitration is against the PDF.",
				what, got, len(got), frame, len(frame))
		}
		if !l.AllowedCommand(cmd.Bytes()) {
			t.Errorf("%s: the %s's own gate refused the frame its own builder produced (%q)", what, l.Model(), cmd.Bytes())
		}
	}}
}

// refusedBecause asserts that the layout's outbound gate refuses the vector,
// and reports why when it does not.
func refusedBecause(why string) replay {
	return replay{kind: kindRefused, check: func(t *testing.T, l Layout, frame string) {
		t.Helper()
		if l.AllowedCommand([]byte(frame)) {
			t.Errorf("the %s's gate ADMITTED %q.\nThis codec builds no such frame: %s", l.Model(), frame, why)
		}
	}}
}

// refusedAndParsed is the disposition of a frame this codec never SENDS and
// must still READ — an EX Set's bytes are its Answer's, and an MC naming a
// section or extension channel is a recall a radio really answers with.
func refusedAndParsed(why string, parse func(t *testing.T, l Layout, frame string)) replay {
	return replay{kind: kindRefused, check: func(t *testing.T, l Layout, frame string) {
		t.Helper()
		if l.AllowedCommand([]byte(frame)) {
			t.Errorf("the %s's gate ADMITTED %q.\nThis codec builds no such frame: %s", l.Model(), frame, why)
		}
		parse(t, l, frame)
	}}
}

// parsedBy is the disposition of an answer: decode it, compare its fields,
// and require the gate to refuse it outbound.
func parsedBy(parse func(t *testing.T, l Layout, frame string)) replay {
	return replay{kind: kindParsed, check: func(t *testing.T, l Layout, frame string) {
		t.Helper()
		parse(t, l, frame)
		if l.AllowedCommand([]byte(frame)) {
			t.Errorf("the %s's gate ADMITTED the ANSWER %q — a captured reply could then be written back to a radio", l.Model(), frame)
		}
	}}
}

// aiDisclosedCoincidence is "AI0;", which is the Set this codec builds AND
// the answer a radio in that state sends. The gate necessarily admits it, and
// AllowedCommand's own doc comment discloses it as one of exactly two such
// cases; spec erratum S-E1 is the record that §Testing's "refuses every
// answer frame" was never achievable.
func aiDisclosedCoincidence() replay {
	return replay{kind: kindParsed, check: func(t *testing.T, l Layout, frame string) {
		t.Helper()
		cmd, err := l.BuildAISetOff()
		if err != nil {
			t.Fatalf("BuildAISetOff: %v", err)
		}
		if got := string(cmd.Bytes()); got != frame {
			t.Errorf("the AI answer vector %q is not byte-identical to the one AI Set this codec builds (%q); the gate's disclosed admission rests on their being one wire shape", frame, got)
		}
		if !l.AllowedCommand([]byte(frame)) {
			t.Errorf("the %s's gate refused %q, which is the frame its own BuildAISetOff produces", l.Model(), frame)
		}
	}}
}

// decodedRecord is one MR answer's decoded content, every value transcribed
// from that vector's own field map rather than measured from the bytes it
// checks.
type decodedRecord struct {
	number   int
	class    SlotClass
	half     ScanHalf
	answerP1 byte
	freqHz   uint64
	mode     Mode
	byte19   byte
	toneMode ToneMode
	tone     int
	ctcss    int
	byte28   byte
	byte3940 string
	byte41   byte
	name     string
}

// mrAnswer is the disposition of a 50-byte MR answer: decode it and compare
// all fourteen fields.
func mrAnswer(want decodedRecord) replay {
	return parsedBy(func(t *testing.T, l Layout, frame string) {
		t.Helper()
		rec, err := l.ParseMRAnswer([]byte(frame))
		if err != nil {
			t.Fatalf("GOLDEN-VS-CODEC MISMATCH — THIS IS A STOP.\n  the %s refused a 50-byte MR answer its own book's chart describes: %v", l.Model(), err)
		}
		if rec.Empty {
			t.Errorf("the record came back as the empty channel of 590:1492-1493, and this vector's P4-P15 are not all zero")
		}
		for _, f := range []struct {
			field    string
			got, wnt any
		}{
			{"the slot number", rec.Slot.Number(), want.number},
			{"the slot class", rec.Slot.Class(), want.class},
			{"the section half", rec.Slot.Half(), want.half},
			{"P1 as read", rec.AnswerP1, want.answerP1},
			{"P4, the frequency", rec.FreqHz, want.freqHz},
			{"P5, the mode nibble", rec.Mode, want.mode},
			{"byte 19 (P6)", rec.Byte19, want.byte19},
			{"P7, the tone mode", rec.ToneMode, want.toneMode},
			{"P8, the tone index", rec.ToneIndex, want.tone},
			{"P9, the CTCSS index", rec.CTCSSIndex, want.ctcss},
			{"byte 28 (P11)", rec.Byte28, want.byte28},
			{"bytes 39-40 (P14)", rec.Byte3940, want.byte3940},
			{"byte 41 (P15)", rec.Byte41, want.byte41},
			{"P16, the memory name", rec.Name, want.name},
		} {
			if f.got != f.wnt {
				t.Errorf("%s decoded as %v, and this vector's own field map prints %v", f.field, f.got, f.wnt)
			}
		}
	})
}

// idAnswer, fvAnswer and tyAnswer are the identity answers' dispositions.
func idAnswer(want string) replay {
	return parsedBy(func(t *testing.T, l Layout, frame string) {
		t.Helper()
		got, err := l.ParseIDAnswer([]byte(frame))
		if err != nil {
			t.Fatalf("ParseIDAnswer(%q): %v", frame, err)
		}
		if got != want {
			t.Errorf("ParseIDAnswer(%q) = %q, and the vector's own legend prints %q", frame, got, want)
		}
	})
}

func fvAnswer(want string) replay {
	return parsedBy(func(t *testing.T, l Layout, frame string) {
		t.Helper()
		got, err := l.ParseFVAnswer([]byte(frame))
		if err != nil {
			t.Fatalf("ParseFVAnswer(%q): %v", frame, err)
		}
		if got != want {
			t.Errorf("ParseFVAnswer(%q) = %q, want %q", frame, got, want)
		}
	})
}

func tyAnswer(variant byte, variantName string) replay {
	return parsedBy(func(t *testing.T, l Layout, frame string) {
		t.Helper()
		a, err := l.ParseTYAnswer([]byte(frame))
		if err != nil {
			t.Fatalf("ParseTYAnswer(%q): %v", frame, err)
		}
		// [T1]: P1's two bytes are ASSUMED "00" in every vector of this
		// file, because the chart prints the word "Reserved" and no value at
		// all (480:1623). The parser carries them OPAQUELY and makes no
		// claim, which is decision 4; asserting them here asserts the
		// DERIVATION's assumption, not the radio's behaviour.
		if a.Reserved != "00" {
			t.Errorf("P1 came back %q, and this file's [T1] assumes the two ASCII characters \"00\"", a.Reserved)
		}
		if a.Variant != variant {
			t.Errorf("P2 came back %q, want %q", a.Variant, variant)
		}
		if got := a.VariantName(); got != variantName {
			t.Errorf("VariantName() = %q, and the book prints %q (480:1626-1629)", got, variantName)
		}
	})
}

// mcChannel is the disposition of an MC frame this codec both builds and
// parses: an ordinary-memory recall, whose Set and Answer are one wire shape.
func mcChannel(number int) replay {
	return replay{kind: kindBuilt, check: func(t *testing.T, l Layout, frame string) {
		t.Helper()
		cmd, err := l.BuildMCSet(mustSlot(t, l, number, ScanHalfNone))
		if err != nil {
			t.Fatalf("BuildMCSet(%03d): %v", number, err)
		}
		if got := string(cmd.Bytes()); got != frame {
			t.Errorf("GOLDEN-VS-CODEC MISMATCH — THIS IS A STOP.\n  BuildMCSet(%03d) built %q, the vector is %q", number, got, frame)
		}
		mcParses(t, l, frame, number, SlotMemory)
		if !l.AllowedCommand([]byte(frame)) {
			t.Errorf("the %s's gate refused %q, the recall its own builder produced", l.Model(), frame)
		}
	}}
}

// mcParses decodes an MC frame and compares its channel and class.
func mcParses(t *testing.T, l Layout, frame string, number int, class SlotClass) {
	t.Helper()
	ch, err := l.ParseMCAnswer([]byte(frame))
	if err != nil {
		t.Fatalf("GOLDEN-VS-CODEC MISMATCH — THIS IS A STOP.\n  the %s refused an MC frame its own book prints: %v", l.Model(), err)
	}
	if ch.Number != number {
		t.Errorf("ParseMCAnswer(%q) named channel %d, want %d", frame, ch.Number, number)
	}
	if ch.Class != class {
		t.Errorf("ParseMCAnswer(%q) resolved class %v, want %v", frame, ch.Class, class)
	}
}

// exRead is the disposition of a ten-byte EX read.
func exRead(menu uint8) replay {
	return builtBy("BuildEXRead", func(t *testing.T, l Layout) (Command, error) {
		return l.BuildEXRead(EXAddress{P1: menu})
	})
}

// exSetOrAnswer is the disposition of an EX Set or Answer: refused outbound —
// they are ONE wire shape, so admitting either would let a captured menu
// reply be written back — and decoded inbound against the width this vector's
// own field map prints.
func exSetOrAnswer(menu uint8, digits int, wantP5 string) replay {
	parse := func(t *testing.T, l Layout, frame string) {
		t.Helper()
		item := EXItem{Addr: EXAddress{P1: menu}, Name: "the vector's own row", Digits: digits}
		got, err := l.ParseEXAnswer([]byte(frame), item)
		if err != nil {
			t.Fatalf("GOLDEN-VS-CODEC MISMATCH — THIS IS A STOP.\n  ParseEXAnswer(%q, Digits=%d): %v", frame, digits, err)
		}
		if got != wantP5 {
			t.Errorf("P5 came back %q, and this vector's field map prints %q — P5 is returned VERBATIM, trailing spaces included, because neither book states a padding rule for it", got, wantP5)
		}
	}
	return refusedAndParsed("this milestone reads the menu surface and writes none of it, and EX's Set and Answer are one wire shape (allowlist.go's validEXRead)", parse)
}

// f1MRRead is the disposition of the four TS-590 MR Read vectors, and it
// RECORDS the divergence rather than resolving it.
//
// THE VECTOR IS REFUSED, AND THAT IS THE PIN. Its last byte is a ':' — the
// terminator cell the 590SG's MR Read chart really prints (590:1442, erratum
// E1, leg G's FINDING F1) — so it carries no terminator and the gate refuses
// it. What this entry adds to the envelope leg above is WHERE the divergence
// is: when the same slot is buildable, the builder's frame agrees with the
// vector on every byte but the last, and the last is exactly ':' against ';'.
// A future arbitration that settles F1 has to come back here.
//
// buildable is false for the split read, which is refused TWICE OVER: P1 is
// derived from the slot's class and never chosen (M9), and A9 records that an
// MR with P1=1 on ordinary memory is not safe to send blind.
func f1MRRead(number int, half ScanHalf, buildable bool, why string) replay {
	return replay{kind: kindF1, check: func(t *testing.T, l Layout, frame string) {
		t.Helper()
		if l.AllowedCommand([]byte(frame)) {
			t.Fatalf("the %s's gate ADMITTED %q: its last byte is a COLON, and FINDING F1 is recorded and unresolved. Admitting it would emit a frame no document describes.", l.Model(), frame)
		}
		if frame[len(frame)-1] != ':' {
			t.Fatalf("%q no longer ends in the colon this entry exists to record (590:1442, E1)", frame)
		}
		if !buildable {
			// THE ASSERTION IS ON P1, not on the whole frame. This vector's
			// slot is perfectly resolvable and the builder happily produces
			// a read for it; what it CANNOT produce is this vector's P1,
			// because P1 comes from the slot's class. So the check is that
			// the two disagree at position 3 and that the byte the codec
			// derives is '0'.
			cmd, err := l.BuildMRRead(mustSlot(t, l, number, half))
			if err != nil {
				t.Fatalf("BuildMRRead(%d): %v", number, err)
			}
			got := string(cmd.Bytes())
			if got[recP1Off] == frame[recP1Off] {
				t.Errorf("the %s built %q, whose P1 matches this vector's %q, and %s", l.Model(), got, frame, why)
			}
			if got[recP1Off] != '0' {
				t.Errorf("the %s derived P1 %q for an ordinary memory slot, and Slot.P1 derives '0' for every class but a section channel's UPPER half", l.Model(), got[recP1Off])
			}
			return
		}
		cmd, err := l.BuildMRRead(mustSlot(t, l, number, half))
		if err != nil {
			t.Fatalf("BuildMRRead(%d): %v", number, err)
		}
		got := string(cmd.Bytes())
		if len(got) != len(frame) {
			t.Fatalf("the builder produced %d bytes and the vector is %d", len(got), len(frame))
		}
		if got[:len(got)-1] != frame[:len(frame)-1] {
			t.Errorf("GOLDEN-VS-CODEC MISMATCH — THIS IS A STOP.\n"+
				"  built  %q\n  vector %q\n"+
				"F1 accounts for the LAST byte alone; a difference anywhere else is a\n"+
				"real disagreement between the derivation and this builder.", got, frame)
		}
		if got[len(got)-1] != ';' {
			t.Errorf("the builder's last byte is %q; every frame this codec emits ends in the terminator both books print (590:87-91, 480:113-118)", got[len(got)-1])
		}
	}}
}

// mwRecord is the record a 50-byte MW Set vector carries, every field
// transcribed from that vector's own field map.
func mwSet(number int, rec decodedRecord) replay {
	return builtBy("BuildMWSet", func(t *testing.T, l Layout) (Command, error) {
		return l.BuildMWSet(Record{
			Slot:       mustSlot(t, l, number, rec.half),
			FreqHz:     rec.freqHz,
			Mode:       rec.mode,
			Byte19:     rec.byte19,
			ToneMode:   rec.toneMode,
			ToneIndex:  rec.tone,
			CTCSSIndex: rec.ctcss,
			Byte28:     rec.byte28,
			Byte3940:   rec.byte3940,
			Byte41:     rec.byte41,
			Name:       rec.name,
		})
	})
}

// perCommandReplays is the roster's other half: one disposition per vector,
// keyed by the vector's own name.
var perCommandReplays = map[string]replay{
	// ----- AI, and the one Set this codec has -----------------------------
	"ai590_set_off":                  builtBy("BuildAISetOff", func(t *testing.T, l Layout) (Command, error) { return l.BuildAISetOff() }),
	"ai590_read":                     builtBy("BuildAIRead", func(t *testing.T, l Layout) (Command, error) { return l.BuildAIRead() }),
	"ai590_set_on_without_backup":    refusedBecause("\"2: AI ON (without backup)\" pushes a response frame per changed parameter into a session that correlates answers by prefix; no AI state but OFF is ever built"),
	"ai590_set_on_with_backup":       refusedBecause("\"4: AI ON (with backup)\" likewise, and it is supported only from firmware 2.00 of the TS-590S"),
	"ai590_answer_off":               aiDisclosedCoincidence(),
	"ai590_answer_on_without_backup": refusedBecause("an AI ANSWER reporting a state this codec never sets; there is no AI answer parser because the session disables Auto Information at open and never asks what the state was"),
	"ai590_answer_on_with_backup":    refusedBecause("the same, for the with-backup state"),

	"ai480_set_off": builtBy("BuildAISetOff", func(t *testing.T, l Layout) (Command, error) { return l.BuildAISetOff() }),
	"ai480_read":    builtBy("BuildAIRead", func(t *testing.T, l Layout) (Command, error) { return l.BuildAIRead() }),
	// F9: this book's legend is 0/1/2/3 where the 590 book's is 0/2/4, so
	// the refused list is per file and one builder could not serve both.
	"ai480_set_old_format_only":         refusedBecause("\"1: the old AI format only\" — this book's own note says the transceiver then sends an IF frame every 1.5 seconds (480:194-195)"),
	"ai480_set_extended_format_only":    refusedBecause("\"2: the extended AI format only\", an ON state (finding F9: this book's legend is 0/1/2/3 where the 590 book's is 0/2/4)"),
	"ai480_set_both_formats":            refusedBecause("\"3: both formats\", the loudest ON state this book prints"),
	"ai480_answer_off":                  aiDisclosedCoincidence(),
	"ai480_answer_old_format_only":      refusedBecause("an AI ANSWER reporting a state this codec never sets"),
	"ai480_answer_extended_format_only": refusedBecause("the same, for the extended format"),
	"ai480_answer_both_formats":         refusedBecause("the same, for both formats"),

	// ----- ID, FV and TY: the three identity grammars ---------------------
	"id590_read":           builtBy("BuildIDRead", func(t *testing.T, l Layout) (Command, error) { return l.BuildIDRead() }),
	"id590_answer_ts590s":  idAnswer("021"),
	"id590_answer_ts590sg": idAnswer("023"),
	"id480_read":           builtBy("BuildIDRead", func(t *testing.T, l Layout) (Command, error) { return l.BuildIDRead() }),
	"id480_answer_ts480":   idAnswer("020"),

	"fv590_read": builtBy("BuildFVRead", func(t *testing.T, l Layout) (Command, error) {
		// The OTHER book has no FV block at all, so the TS-480 layout must
		// refuse the very frame this one builds. It is asserted here rather
		// than in an entry of its own because no TS-480 vector exists for a
		// command that radio does not have.
		if _, err := layout480().BuildFVRead(); err == nil {
			t.Error("the TS-480 layout built an FV read, and FV appears nowhere in the 2003 document (E15: that radio has no CAT-readable firmware version)")
		}
		return l.BuildFVRead()
	}),
	// The book's own worked example, quoted character for character
	// (590:1035).
	"fv590_answer_v1_00": fvAnswerAndTheUnparseableCase("1.00"),
	// [V1]: same shape, other versions — the bytes are ASSUMED.
	"fv590_answer_v2_00": fvAnswer("2.00"),
	"fv590_answer_v1_08": fvAnswer("1.08"),

	"ty480_read": builtBy("BuildTYRead", func(t *testing.T, l Layout) (Command, error) {
		// And the mirror of FV's: the 590 book prints no TY anywhere.
		if _, err := layout590SG().BuildTYRead(); err == nil {
			t.Error("the TS-590SG layout built a TY read, and TY appears nowhere in the TS-590S/SG document")
		}
		return l.BuildTYRead()
	}),
	"ty480_answer_ts480hx_200w":     tyAnswer('0', "TS-480HX (200 W)"),
	"ty480_answer_ts480sat_100w_at": tyAnswer('1', "TS-480SAT (100 W + AT)"),
	"ty480_answer_japanese_50w":     tyAnswer('2', "Japanese 50 W type"),
	"ty480_answer_japanese_20w":     tyAnswer('3', "Japanese 20 W type"),

	// ----- MC, whose Set domain and Answer domain are different domains ---
	"mc590_read": builtBy("BuildMCRead", func(t *testing.T, l Layout) (Command, error) { return l.BuildMCRead() }),
	// The '0' form is the one this codec emits (A10, slotWire).
	"mc590_set_ch03_zero_hundreds": mcChannel(3),
	// The SPACE form is MC's own printed Set convention — "enter 0 or a
	// space for a channel number less than 100" (590:1334-1335) — so the
	// gate ADMITS it and the parser decodes it, while the builder emits the
	// digit. That asymmetry is A10 and it is deliberate: a digit passes the
	// envelope unremarkably and reads the same in a log.
	"mc590_set_ch03_space_hundreds": replay{kind: kindParsed, check: func(t *testing.T, l Layout, frame string) {
		t.Helper()
		mcParses(t, l, frame, 3, SlotMemory)
		if !l.AllowedCommand([]byte(frame)) {
			t.Errorf("the %s's gate refused %q; MC's own chart prints the space as legal on a Set below channel 100 (590:1334-1335)", l.Model(), frame)
		}
		cmd, err := l.BuildMCSet(mustSlot(t, l, 3, ScanHalfNone))
		if err != nil {
			t.Fatalf("BuildMCSet(003): %v", err)
		}
		if string(cmd.Bytes()) == frame {
			t.Errorf("the builder emitted the SPACE form %q; A10 records that this codec always emits '0' and always accepts either on parse", frame)
		}
	}},
	"mc590_set_ch100_P00": refusedAndParsed(
		"A16 (L-DEC-2) narrows the MC SET domain to ordinary memory — recalling a channel changes the radio's operating state — while 590:1345-1347's section numbers stay selectable in the ANSWER domain",
		func(t *testing.T, l Layout, frame string) { mcParses(t, l, frame, 100, SlotScan) }),
	"mc590_set_ch110_E00": refusedAndParsed(
		"A16 again, and this is the extension range: what an extension channel IS is never explained anywhere in the book (A11)",
		func(t *testing.T, l Layout, frame string) {
			mcParses(t, l, frame, 110, SlotExtension)
			// The SIBLING: A12 puts 110 outside the TS-590S's space, so the
			// same frame is refused there. It is the only place in this file
			// where the two 590 rows part company.
			if _, err := layout590S().ParseMCAnswer([]byte(frame)); err == nil {
				t.Errorf("the TS-590S parsed %q; the book gives 110-119 to the SG (590:1346-1347) and never states the S's own ceiling (A12)", frame)
			}
		}),
	"mc590_answer_ch03":      parsedByMC(3, SlotMemory),
	"mc590_answer_ch100_P00": parsedByMC(100, SlotScan),
	"mc590_answer_ch119": replay{kind: kindParsed, check: func(t *testing.T, l Layout, frame string) {
		t.Helper()
		mcParses(t, l, frame, 119, SlotExtension)
		if l.AllowedCommand([]byte(frame)) {
			t.Errorf("the %s's gate ADMITTED %q; A16 keeps the SEND domain at ordinary memory", l.Model(), frame)
		}
		if _, err := layout590S().ParseMCAnswer([]byte(frame)); err == nil {
			t.Errorf("the TS-590S parsed %q, the top of the SG's extension range (A12)", frame)
		}
	}},

	"mc480_read":     builtBy("BuildMCRead", func(t *testing.T, l Layout) (Command, error) { return l.BuildMCRead() }),
	"mc480_set_ch00": mcChannel(0),
	"mc480_set_ch03": mcChannel(3),
	// Decision 15: 90-99 are ORDINARY memories on this row that also answer
	// a second frame, not a scan class, so this recall is an ordinary one.
	"mc480_set_ch90_program_scan_lower": mcChannel(90),
	"mc480_set_ch99_top_of_range":       mcChannel(99),
	"mc480_answer_ch03":                 parsedByMC(3, SlotMemory),
	"mc480_answer_ch99":                 parsedByMC(99, SlotMemory),

	// ----- MR: the read request and the 50-byte answer --------------------
	"mr590_read_ch03_simplex_rx":   f1MRRead(3, ScanHalfNone, true, ""),
	"mr590_read_ch04_simplex_rx":   f1MRRead(4, ScanHalfNone, true, ""),
	"mr590_read_ch100_P00_simplex": f1MRRead(100, ScanLower, true, ""),
	"mr590_read_ch12_split_tx": f1MRRead(12, ScanHalfNone, false,
		"P1 is derived from the SLOT'S CLASS and never chosen freely (M9, Slot.P1); an ordinary memory slot derives '0', and A9 records that an MR with P1=1 on a simplex channel is not safe to send blind"),

	"mr480_read_ch03_rx": builtBy("BuildMRRead", func(t *testing.T, l Layout) (Command, error) {
		return l.BuildMRRead(mustSlot(t, l, 3, ScanHalfNone))
	}),
	"mr480_read_ch04_rx": builtBy("BuildMRRead", func(t *testing.T, l Layout) (Command, error) {
		return l.BuildMRRead(mustSlot(t, l, 4, ScanHalfNone))
	}),
	"mr480_read_ch90_start_freq": builtBy("BuildMRRead", func(t *testing.T, l Layout) (Command, error) {
		return l.BuildMRRead(mustSlot(t, l, 90, ScanHalfNone))
	}),
	"mr480_read_ch12_tx":       refusedBecause("P1='1' is \"1: TX frequency\" (480:951) on an ordinary memory slot, and P1 is derived from the slot's class (M9); A9 records that this frame is not safe to send blind"),
	"mr480_read_ch90_end_freq": refusedBecause("the book really does print \"Memory channel 90 ~ 99: P1=0 (start frequency), P1=1 (end frequency)\" (480:943-944), but decision 15 rules those ten ordinary memories rather than a scan class on this row, so the P1=1 half of 90-99 is unreachable through this programme"),

	"mr590_answer_ch03_simplex_name8": mrAnswer(decodedRecord{
		number: 3, class: SlotMemory, half: ScanHalfNone, answerP1: '0',
		freqHz: 14_250_000, mode: ModeUSB, byte19: '0', toneMode: ToneModeOff,
		byte28: '0', byte3940: "00", byte41: '0', name: "DXCLUSTR",
	}),
	"mr590_answer_ch04_simplex_name3": mrAnswer(decodedRecord{
		number: 4, class: SlotMemory, half: ScanHalfNone, answerP1: '0',
		freqHz: 7_150_000, mode: ModeLSB, byte19: '0', toneMode: ToneModeOff,
		byte28: '0', byte3940: "00", byte41: '0',
		// A1: P16's five trailing spaces are right-trimmed on read.
		name: "NET",
	}),
	"mr590_answer_ch12_split_tx_tone_on": mrAnswer(decodedRecord{
		number: 12, class: SlotMemory, half: ScanHalfNone, answerP1: '1',
		freqHz: 51_500_000, mode: ModeFM, byte19: '0', toneMode: ToneModeTone,
		tone: 8, byte28: '1', byte3940: "01", byte41: '0', name: "REPEATER",
	}),

	"mr480_answer_ch03_rx_name8": mrAnswer(decodedRecord{
		number: 3, class: SlotMemory, half: ScanHalfNone, answerP1: '0',
		freqHz: 14_250_000, mode: ModeUSB, byte19: '0', toneMode: ToneModeOff,
		byte28: '0', byte3940: "00", byte41: '0', name: "DXCLUSTR",
	}),
	"mr480_answer_ch04_rx_name3": mrAnswer(decodedRecord{
		number: 4, class: SlotMemory, half: ScanHalfNone, answerP1: '0',
		freqHz: 7_150_000, mode: ModeLSB, byte19: '0', toneMode: ToneModeOff,
		byte28: '0', byte3940: "00", byte41: '0', name: "NET",
	}),
	"mr480_answer_ch12_tx_tone_on": mrAnswer(decodedRecord{
		number: 12, class: SlotMemory, half: ScanHalfNone, answerP1: '1',
		freqHz: 51_500_000, mode: ModeFM, byte19: '0', toneMode: ToneModeTone,
		tone: 8, byte28: '0', byte3940: "02", byte41: '0', name: "REPEATER",
	}),

	// ----- MW: the 50-byte Set, and the erase shape this codec never builds
	"mw590_set_ch03_simplex_name8": mwSet(3, decodedRecord{
		half: ScanHalfNone, freqHz: 14_250_000, mode: ModeUSB, byte19: '0',
		toneMode: ToneModeOff, byte28: '0', byte3940: "00", byte41: '0',
		name: "DXCLUSTR",
	}),
	"mw480_set_ch03_rx_name8": mwSet(3, decodedRecord{
		half: ScanHalfNone, freqHz: 14_250_000, mode: ModeUSB, byte19: '0',
		toneMode: ToneModeOff, byte28: '0', byte3940: "00", byte41: '0',
		name: "DXCLUSTR",
	}),
	"mw590_set_ch12_split_tone_on": refusedBecause("its P1 is '1', the SPLIT registration of 590:1519-1520, and this codec derives P1 from the slot's CLASS (M9, Slot.P1) — an ordinary memory slot derives '0'. There is no way to ask for a split write, and the gate refuses this frame because BuildMWSet refuses the record the parser decoded from it"),
	"mw480_set_ch12_tx_tone_on":    refusedBecause("its P1 is '1', \"1: TX frequency\" (480:951), and the same M9 derivation applies; on this row A22 refuses every channel write in the driver as well"),
	"mw590_set_ch03_erase_no_P16":  refusedBecause("it is the 42-byte ERASE shape of 590:1579-1581 (A5, erratum E19). Decision 8 builds no erase frame, and BuildMWSet emits exactly RecordLen bytes with checkRecordLen as its own gate — the vector is evidence of what the book prints, never a shape to emit"),

	// ----- EX: ten fixed bytes, and a P5 whose width is the row's ---------
	"ex590_read_menu000":            exRead(0),
	"ex590_read_menu001":            exRead(1),
	"ex590_read_menu002":            exRead(2),
	"ex590_read_menu087_last_590S":  exRead(87),
	"ex590_read_menu099_last_590SG": exRead(99),
	// The SG's menu 001, "Power on message", eight characters (590:750);
	// P5 comes back VERBATIM, the two trailing spaces included.
	"ex590_set_menu001_poweron_msg_8":    exSetOrAnswer(1, 8, "MYCALL  "),
	"ex590_answer_menu001_poweron_msg_8": exSetOrAnswer(1, 8, "MYCALL  "),
	// [E1]: menu 002's one-character width is ASSUMED from a table of
	// VALUES rather than counted off a field map; the chart itself draws
	// eight (erratum E16 — that drawn ';' is illustrative, not a width).
	"ex590_set_menu002_brightness_3":    exSetOrAnswer(2, 1, "3"),
	"ex590_answer_menu002_brightness_3": exSetOrAnswer(2, 1, "3"),

	"ex480_read_menu000":      exRead(0),
	"ex480_read_menu032":      exRead(32),
	"ex480_read_menu060_last": exRead(60),
	// The two frames quoted from the book's own worked examples
	// (480:415-416), one-character P5.
	"ex480_set_menu000_illumination_off": exSetOrAnswer(0, 1, "0"),
	"ex480_set_menu000_brightness_3":     exSetOrAnswer(0, 1, "3"),
	"ex480_answer_menu000_brightness_3":  exSetOrAnswer(0, 1, "3"),
	// Menu 032 is one of the rows the EX block's own prose names as
	// two-digit (480:411). Menu 034 is another and the prose omits it,
	// which is erratum E22 — recorded in core/kw/ts480/doc.go, and no
	// vector of this file exercises it.
	"ex480_set_menu032_two_digit":    exSetOrAnswer(32, 2, "05"),
	"ex480_answer_menu032_two_digit": exSetOrAnswer(32, 2, "05"),
}

// parsedByMC is the disposition of an MC ANSWER: decoded, and refused
// outbound unless it coincides with a Set this codec builds.
func parsedByMC(number int, class SlotClass) replay {
	return replay{kind: kindParsed, check: func(t *testing.T, l Layout, frame string) {
		t.Helper()
		mcParses(t, l, frame, number, class)
		// AN MC ANSWER NAMING ORDINARY MEMORY IS ADMITTED, and that is one
		// of the gate's exactly two disclosed answer-admissions: it is
		// byte-identical to the recall BuildMCSet emits for the same
		// channel (590:1333 against 590:1341). Every other class is
		// refused, which is what A16's narrowing actually buys.
		want := class == SlotMemory
		if got := l.AllowedCommand([]byte(frame)); got != want {
			t.Errorf("the %s's gate returned %v for the MC answer %q (class %v), want %v — an ordinary-memory answer coincides with the Set this codec builds and every other class does not", l.Model(), got, frame, class, want)
		}
	}}
}

// fvAnswerAndTheUnparseableCase is fvAnswer plus the case the TODO named:
// a four-character P1 that is not a version number at all.
//
// A13 PINS THE WIDTH AND NOT THE GRAMMAR. The answer chart gives four
// positions and the only format statement anywhere is one worked example
// (590:1035), so a parser demanding digit-dot-digit-digit would assert a
// grammar the document does not print and would fail a session on a radio the
// document permits. The consumer that must survive it is the TS-590S write
// gate, which reads FV to decide whether byte 28 is live (A14).
//
// THE MUTATED FRAME IS DERIVED FROM THIS VECTOR AND IS NOT A NEW ONE. Leg G's
// files are frozen and nothing here may add to them; the four bytes are
// replaced in a copy, in this test only.
func fvAnswerAndTheUnparseableCase(want string) replay {
	inner := fvAnswer(want)
	return replay{kind: inner.kind, check: func(t *testing.T, l Layout, frame string) {
		t.Helper()
		inner.check(t, l, frame)

		odd := []byte(frame)
		copy(odd[2:6], "X-.Z")
		got, err := l.ParseFVAnswer(odd)
		if err != nil {
			t.Errorf("ParseFVAnswer refused %q: A13 pins the WIDTH and not the grammar, and a parser demanding digit-dot-digit-digit would fail a session on a radio the document permits (%v)", odd, err)
		}
		if got != "X-.Z" {
			t.Errorf("ParseFVAnswer(%q) = %q, want the four bytes verbatim", odd, got)
		}
	}}
}

// TestGoldenVectors_EveryVectorReplaysThroughItsOwnCodec is the walk: every
// vector in the roster, through the disposition perCommandReplays gives it.
//
// THE COVERAGE CHECK IS THE POINT OF THE MAP. A leg that walked whatever it
// found would silently shrink as vectors were added; this one requires the
// roster and the map to be the SAME SET, so a vector with no disposition and
// a disposition with no vector each fail here by name.
func TestGoldenVectors_EveryVectorReplaysThroughItsOwnCodec(t *testing.T) {
	seen := map[string]bool{}
	kinds := map[string]int{}
	total := 0

	for _, spec := range goldenSpecs {
		l := replayLayout(t, spec.book)
		for _, v := range loadGoldenVectors(t, spec.file) {
			r, ok := perCommandReplays[v.name]
			if !ok {
				t.Errorf("%s (%s:%d) has no per-command disposition. Every vector must be BUILT, PARSED, REFUSED with its reason, or recorded as F1; there is no fifth answer and \"not covered\" is not one of them.", v.name, v.file, v.line)
				continue
			}
			if seen[v.name] {
				t.Errorf("%s appears twice in the roster; the dispositions are keyed by name and the second would be checked against the first's frame", v.name)
			}
			seen[v.name] = true
			kinds[r.kind]++
			total++
			t.Run(v.name, func(t *testing.T) {
				r.check(t, l, v.frame)
			})
		}
	}

	for name := range perCommandReplays {
		if !seen[name] {
			t.Errorf("perCommandReplays carries a disposition for %q, which is in no vector file — a renamed or deleted vector leaves its disposition asserting nothing", name)
		}
	}
	if total != totalVectors {
		t.Errorf("%d vectors were replayed, and leg G is %d", total, totalVectors)
	}
	// Non-vacuity: all four dispositions must actually occur. A table that
	// had collapsed into "everything is refused" would otherwise pass.
	for _, kind := range []string{kindBuilt, kindParsed, kindRefused, kindF1} {
		if kinds[kind] == 0 {
			t.Errorf("no vector was dispositioned %q — a table with only one kind in it proves far less than it appears to", kind)
		}
	}
	if kinds[kindF1] != len(f1MRReadVectors) {
		t.Errorf("%d vectors are dispositioned F1, and f1MRReadVectors names %d", kinds[kindF1], len(f1MRReadVectors))
	}
	t.Logf("replayed %d vectors: %d built, %d parsed, %d refused with a reason, %d recorded as F1",
		total, kinds[kindBuilt], kinds[kindParsed], kinds[kindRefused], kinds[kindF1])
}
