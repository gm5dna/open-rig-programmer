// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestKenwoodRegisterCitationsMatchTheRegister is the Stage 2 close
// adjudication's register-to-lift close check (plan-rev2b:1751, never
// built; reviews/s2-close-adjudication.md, Codex C-HIGH-1/C-HIGH-2). It
// parses core/kw/ma/doc.go's A1-A22 register — the ONE authoritative
// {A<n> -> lift, scope} table for this pair — and walks every comment (test
// and non-test) under the two drivers and their two independent fakes,
// asserting that no comment:
//
//   - (a) cites an A-number past the register's last row (a stale pair-1
//     number surviving a copy-paste; the Codex review's "A25" find);
//   - (b) pairs an A-number with an L-HW-n/L-DOC-n/L-DEC-n lift the
//     register does not carry for that row (the "A2 ... L-HW-9" find,
//     A2's own lift being L-HW-2 and L-HW-9 being A12's);
//   - (c) cites a row the register scopes to ONE radio (the head sentence
//     reads "TS-890S ONLY:"/"TS-990S ONLY:"/"On the 890S,"/"On the 990S,"/
//     "The 890S's"/"The 990S's") from the OTHER radio's driver or fake (the
//     A17 890S-only frame-width borrow into ts990's write ladder);
//   - (d) calls an accepted Set's silence out by any A-number other than
//     A20 (A20/L-HW-3 is the ONLY entry this pair's register lifts to "an
//     MA0 Set produces no answer while AI is off"; the review's ts990 sites
//     misnamed it A6, which is a DIFFERENT entry — A6 is what a blank BYTE
//     inside an answer means, lifted by the same session as A4 but nothing
//     to do with Set silence).
//
// (e) from the ORDER item's own list — a fuzzy "silence adjacent to an
// A-number" predicate with no anchor — was dropped rather than built: it
// cannot be made deterministic without either the (d) "Set" anchor below or
// a curated site list, and a curated list is not a guard. (d) is the
// anchored, kept version of it.
//
// WHY A TEXT GUARD AND NOT A go/vet-STYLE AST WALK: the citations this
// checks are entirely in prose (doc comments, error-message string
// literals are explicitly OUT of scope — the register's own citation
// discipline binds comments, per core/kw/ma/doc.go's own header), so the
// unit of analysis is the comment, not the syntax tree. parseRepo (this
// package's other guards) is AST-based and, deliberately, skips _test.go
// files; this guard needs both, so it does its own line-based walk.
func TestKenwoodRegisterCitationsMatchTheRegister(t *testing.T) {
	root := repoRoot(t)

	register, lastRow := parseKWRegister(t, root)

	seen := 0
	for _, dir := range kwRegisterCheckedDirs {
		chunks := kwWalkGoComments(t, root, dir)
		seen += len(chunks)
		for _, c := range chunks {
			checkKWRegisterCitations(t, c, dir, register, lastRow)
		}
	}
	if seen == 0 {
		t.Fatalf("kwWalkGoComments found zero comment paragraphs under %v — the walk or its filters are broken and every check above passed vacuously", kwRegisterCheckedDirs)
	}
}

// kwRegisterCheckedDirs are the five packages a register citation can
// legitimately name: the codec that HOLDS the register, the two drivers and
// their two independent fakes. Repo-relative, slash-separated.
//
// core/kw/ma IS WALKED, doc.go INCLUDED, and that is deliberate rather than
// incidental (the milestone-close review's S1-LOW-1/S2-LOW-1): the package
// where the register lives is the package whose own A-citations are densest —
// shared.go, envelope.go, the two codecs and layout.go — and a wrong A-number
// there would be the most authoritative wrong number in the tree. Skipping
// doc.go to dodge its ONE cross-register reference would have taken the
// register's own rows out of the walk with it; kwOtherRegisterRE below is the
// narrower escape.
//
// Check (c) is inert on this directory by construction: its otherDirs are the
// two drivers and the two fakes, and the codec is neither row's "other row".
// That is right — the codec speaks for both radios, so a 890S-only row cited
// from a file serving both is not the borrow (c) exists to catch.
var kwRegisterCheckedDirs = []string{
	"core/kw/ma",
	"core/driver/ts890",
	"core/driver/ts990",
	"internal/fakets890",
	"internal/fakets990",
}

// kwRegisterEntry is one A-row's {lift, scope}, as the register states it.
type kwRegisterEntry struct {
	// lifts is the set of lift IDs (L-HW-n, L-DOC-n, L-DEC-n) the register
	// names for this row. More than one only for A11 ("L-DOC-1 ... or
	// L-DEC-1").
	lifts map[string]bool
	// scope is "890", "990", or "" (both rows) per the head sentence's own
	// scoping words.
	scope string
}

// kwKnownLiftRE matches a lift ID token wherever it appears.
var kwKnownLiftRE = regexp.MustCompile(`L-(?:HW|DOC|DEC)-\d+`)

// kwRegisterHeadRE matches an A-row's head line, "A<n> " immediately after
// the comment's own leading "//" and whitespace have been stripped.
var kwRegisterHeadRE = regexp.MustCompile(`^A(\d+)\s`)

// kwScopeRE finds the register's own scoping words. Both the explicit
// "TS-<row> ONLY:" tag (A1, A21) and this design's other phrasing for the
// same thing ("On the <row>," — A4, A7, A8; "The <row>'s" — A14, A17) are
// matched: A17's own head sentence never uses the word ONLY, but its
// Lift line names the same TS-890S-ONLY blank-channel read A4's does, and
// re-deriving scope from the row's own subject sentence (rather than
// threading lift-sharing between rows, which A6 shares with A4/A17/A21
// without being scoped itself) is what makes this deterministic.
var kwScopeRE = regexp.MustCompile(`TS-(890|990)S ONLY:|On the (890|990)S,|The (890|990)S's`)

// parseKWRegister parses core/kw/ma/doc.go's A1-A22 register into
// {number -> entry}, and returns the last row's number (22, but re-derived
// rather than hard-coded: a widened register should widen this guard's
// ceiling for free).
func parseKWRegister(t *testing.T, root string) (map[int]kwRegisterEntry, int) {
	t.Helper()
	const docPath = "core/kw/ma/doc.go"
	paras := kwCommentParagraphs(t, filepath.Join(root, docPath))

	register := map[int]kwRegisterEntry{}
	for _, p := range paras {
		if len(p.lines) == 0 {
			continue
		}
		m := kwRegisterHeadRE.FindStringSubmatch(p.lines[0])
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("%s:%d: A-row number %q does not parse: %v", docPath, p.startLine, m[1], err)
		}
		body := strings.Join(p.lines, "\n")

		scope := ""
		if sm := kwScopeRE.FindStringSubmatch(body); sm != nil {
			for _, g := range sm[1:] {
				if g != "" {
					scope = g
					break
				}
			}
		}

		lifts := map[string]bool{}
		for _, l := range kwKnownLiftRE.FindAllString(body, -1) {
			lifts[l] = true
		}

		register[n] = kwRegisterEntry{lifts: lifts, scope: scope}
	}

	if len(register) == 0 {
		t.Fatalf("%s: parsed zero A-rows — kwRegisterHeadRE or the paragraph walk is broken and this guard would pass vacuously", docPath)
	}
	lastRow := 0
	for n := range register {
		if n > lastRow {
			lastRow = n
		}
	}
	return register, lastRow
}

// kwParagraph is one contiguous, non-empty run of "//" comment lines (a
// blank "//" line, a code line, or EOF ends it), with "//" and surrounding
// whitespace already stripped from each line.
type kwParagraph struct {
	startLine int
	lines     []string
}

// kwCommentParagraphs reads path and groups its "//" line comments into
// paragraphs. It is line-based rather than go/ast-based because it must
// see _test.go files (parseRepo, this package's other helper, deliberately
// excludes them) and because comment TEXT LAYOUT — where one paragraph
// ends and the next begins — is exactly what distinguishes a citation from
// an unrelated word two sentences later (see kwBulletChunks).
func kwCommentParagraphs(t *testing.T, path string) []kwParagraph {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var out []kwParagraph
	var cur kwParagraph
	for i, raw := range strings.Split(string(data), "\n") {
		lineNo := i + 1
		trimmed := strings.TrimSpace(raw)
		content, isComment := strings.CutPrefix(trimmed, "//")
		content = strings.TrimSpace(content)
		if isComment && content != "" {
			if len(cur.lines) == 0 {
				cur.startLine = lineNo
			}
			cur.lines = append(cur.lines, content)
			continue
		}
		if len(cur.lines) > 0 {
			out = append(out, cur)
			cur = kwParagraph{}
		}
	}
	if len(cur.lines) > 0 {
		out = append(out, cur)
	}
	return out
}

// kwBulletChunks splits one paragraph into its markdown "-" bullet items
// (plus a leading chunk for any prose before the first bullet), so that a
// silence/A-number/lift co-occurrence check run over a four-bullet "what
// this package deliberately does NOT do" list does not cross-contaminate
// between bullets — e.g. bullet 1's unrelated NAK-probe "silence" and
// bullet 2's "An MN Set ... which is A18" would otherwise look, wrongly,
// like ONE comment calling A18 the reason an MA0 Set is silent. A
// paragraph with no bullets is one chunk (itself).
func kwBulletChunks(p kwParagraph) []string {
	var chunks []string
	var cur []string
	for _, l := range p.lines {
		if strings.HasPrefix(l, "- ") && len(cur) > 0 {
			chunks = append(chunks, strings.Join(cur, "\n"))
			cur = nil
		}
		cur = append(cur, l)
	}
	if len(cur) > 0 {
		chunks = append(chunks, strings.Join(cur, "\n"))
	}
	return chunks
}

// kwCommentChunk is one bullet chunk (or whole paragraph, if it has no
// bullets) with its file and starting line, for failure messages.
type kwCommentChunk struct {
	relPath   string
	startLine int
	text      string
}

// kwWalkGoComments returns every comment chunk in every .go file (test AND
// non-test) directly under root/dir and its subdirectories.
func kwWalkGoComments(t *testing.T, root, dir string) []kwCommentChunk {
	t.Helper()
	var out []kwCommentChunk
	base := filepath.Join(root, dir)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)
		for _, p := range kwCommentParagraphs(t, path) {
			for _, chunk := range kwBulletChunks(p) {
				out = append(out, kwCommentChunk{relPath: rel, startLine: p.startLine, text: chunk})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", base, err)
	}
	return out
}

// --- citation extraction ---

// kwCollisionRE strips the one known non-register use of "A4" in these
// packages: the A4-FORMAT capability matrix (paper size), not the
// register's A4 (a TS-890S name-window assumption). Both spellings this
// codebase uses ("A4-format capability matrix" and "the A4 matrix's §1")
// are matched and removed before citation extraction runs.
var kwCollisionRE = regexp.MustCompile(`A4-format|A4 matrix`)

// kwRangeRE strips a register RANGE expression ("A1..A22" or "A1-A22"):
// prose naming the whole register by its span, not citing either
// endpoint's row.
var kwRangeRE = regexp.MustCompile(`A\d{1,2}(?:\.\.|-)A\d{1,2}`)

// kwOtherRegisterRE strips a labelled cross-reference to PAIR 1's register,
// which is a DIFFERENT numbering space that happens to share the "A<n>"
// spelling (core/kw/doc.go's rows against core/kw/ma/doc.go's — the two
// registers restart, as core/kw/ma/doc.go's header now says in as many
// words). core/kw/ma/doc.go's A15 says "As pair 1's A17 ... Lift: L-DOC-2",
// naming pair 1's A17 beside pair 2's own lift; read as a citation of THIS
// register's A17 it is a mismatch, and it is not one.
//
// The escape is the label, not the number: only an A-number preceded by
// "pair 1's" is stripped, so a bare A17 in the same file is still checked.
var kwOtherRegisterRE = regexp.MustCompile(`(?i)pair 1's A\d{1,2}`)

// kwBareANumberRE matches a bare "A<n>" register citation.
var kwBareANumberRE = regexp.MustCompile(`\bA(\d{1,2})\b`)

// kwRegisterConstRE matches this codebase's "registerA<n>" constant-naming
// convention (core/driver/ts890/write.go, core/driver/ts990/write.go):
// each constant's doc comment reads "registerA2 is the tag charset ...
// LIFT: L-HW-2, per registry row.", citing its row through the identifier
// name rather than a bare "A2" token. \b does not fire between "register"
// and "A" (both word characters), so this needs its own pattern.
var kwRegisterConstRE = regexp.MustCompile(`\bregisterA(\d{1,2})\b`)

// kwCiteExemptRE marks a chunk as ABOUT the register or a sibling file's
// citation of it, in prose, rather than USING a citation to justify this
// row's own behaviour — the two false-positive shapes an inspection of the
// current tree turned up that no simpler rule avoided:
//
//   - a markdown bullet ("- **A2**"), which only ever appears in a test
//     walking PROVENANCE.md's own bullet list (e.g.
//     internal/fakets990/register_test.go's collision-avoidance
//     discussion of "- **A2**" vs "- **A21**");
//   - "PROVENANCE.md" itself, for the same reason.
var kwCiteExemptRE = regexp.MustCompile(`PROVENANCE\.md|- \*\*`)

// kwHeadingRE recognises a bare markdown section heading ("# A1 on this
// row, ..."), a navigation aid rather than a citation: doc.go's forward
// reference to A1's own full paragraph two lines below it is exactly what
// that paragraph settles.
var kwHeadingRE = regexp.MustCompile(`^#`)

// kwCiteNumbers returns the register numbers text cites, by either
// spelling, after stripping the known non-register collision and range
// expressions.
func kwCiteNumbers(text string) map[int]bool {
	stripped := kwCollisionRE.ReplaceAllString(text, "")
	stripped = kwRangeRE.ReplaceAllString(stripped, "")
	stripped = kwOtherRegisterRE.ReplaceAllString(stripped, "")

	nums := map[int]bool{}
	for _, m := range kwBareANumberRE.FindAllStringSubmatch(stripped, -1) {
		n, _ := strconv.Atoi(m[1])
		nums[n] = true
	}
	// The registerA<n> identifier form is read from the UNSTRIPPED text:
	// it cannot collide with the A4-format/range patterns above (neither
	// contains "registerA").
	for _, m := range kwRegisterConstRE.FindAllStringSubmatch(text, -1) {
		n, _ := strconv.Atoi(m[1])
		nums[n] = true
	}
	return nums
}

// --- the four checks ---

// kwScopeMentionRE reports whether text mentions the OTHER row by number
// (a bare "890" or "990" substring — deliberately not anchored to "...S",
// since a citation this design makes as a cross-reference also spells the
// sibling FILE ("internal/fakets890") or a manual PAGE ("890:3215-3216"),
// neither of which carries a trailing S) or uses one of this codebase's
// narrow, explicit non-applicability phrases. Both signals were derived
// from every current cross-reference in the tree (core/driver/ts890's
// three "this row's A1 half is NOT an assumption" sites; every
// internal/fakets990 comment that discusses the sibling row's A4/A17/A21
// by number) rather than assumed: a bare "NOT" or "deliberately" was
// tried first and rejected — this package's prose uses both words
// constantly for reasons that have nothing to do with register scope
// ("Building is not sending", "TWO SENTINELS, DELIBERATELY"), and either
// one wrongly exempted a real ts990 A17 borrow site.
func kwScopeMentionRE(scope string) *regexp.Regexp {
	if scope == "890" {
		return regexp.MustCompile(`890`)
	}
	return regexp.MustCompile(`990`)
}

var kwNonApplicabilityRE = regexp.MustCompile(`(?i)NOT an assumption|NOT ON THIS|NOT among|no counterpart|excluded for their own reasons|mirror image|deliberately NOT`)

// kwSilenceRE matches this pair's vocabulary for an accepted Set's silence
// (and a read's, which is why (d) also requires kwSetRE — see below).
var kwSilenceRE = regexp.MustCompile(`(?i)silen|no answer|never answers|draws nothing|draws NO REPLY`)

// kwSetRE anchors (d) to the MA0 Set specifically. Without it, "silence"
// co-occurring with an unrelated A-number in the SAME bullet or paragraph
// (A21's read-residue "discarded silently", A1's name padding "silently
// absorbed", the NO-DISCOVERY bullet's unrelated NAK-probe silence) reads
// as a false misattribution. Every current site this rule must catch
// names "Set" in the same breath as the silence ("an ACCEPTED Set draws
// nothing", "acknowledgement of a Set is silence", "AN ACCEPTED SET DRAWS
// NOTHING, which is A6 applied").
var kwSetRE = regexp.MustCompile(`\bSet\b`)

func checkKWRegisterCitations(t *testing.T, c kwCommentChunk, dir string, register map[int]kwRegisterEntry, lastRow int) {
	if kwCiteExemptRE.MatchString(c.text) {
		return
	}
	if kwHeadingRE.MatchString(c.text) {
		return
	}

	nums := kwCiteNumbers(c.text)
	liftsHere := map[string]bool{}
	for _, l := range kwKnownLiftRE.FindAllString(c.text, -1) {
		liftsHere[l] = true
	}

	// (d): a Set's silence attributed to any A-number but A20.
	if kwSilenceRE.MatchString(c.text) && kwSetRE.MatchString(c.text) {
		var bad []int
		for n := range nums {
			if n != 20 {
				bad = append(bad, n)
			}
		}
		if len(bad) > 0 {
			sort.Ints(bad)
			t.Errorf("%s:%d: names %v for an accepted Set's silence — the register's ONLY entry for that is A20/L-HW-3 (\"an MA0 Set produces no answer while AI is off\"); comment: %q", c.relPath, c.startLine, bad, c.text)
		}
	}

	for n := range nums {
		// (a): past the register's last row.
		if n > lastRow {
			t.Errorf("%s:%d: cites A%d, past the register's last row (A%d) — core/kw/ma/doc.go has no such entry; comment: %q", c.relPath, c.startLine, n, lastRow, c.text)
			continue
		}
		entry, ok := register[n]
		if !ok {
			continue
		}

		// (b): a lift named alongside A<n> that the register does not
		// carry for that row.
		if len(liftsHere) > 0 {
			matched := false
			for l := range liftsHere {
				if entry.lifts[l] {
					matched = true
					break
				}
			}
			if !matched {
				var gotLifts, wantLifts []string
				for l := range liftsHere {
					gotLifts = append(gotLifts, l)
				}
				for l := range entry.lifts {
					wantLifts = append(wantLifts, l)
				}
				sort.Strings(gotLifts)
				sort.Strings(wantLifts)
				t.Errorf("%s:%d: cites A%d alongside lift(s) %v, but the register's lift for A%d is %v; comment: %q", c.relPath, c.startLine, n, gotLifts, n, wantLifts, c.text)
			}
		}

		// (c): a row the register scopes to one radio, cited from the
		// other radio's driver or fake.
		if entry.scope == "" {
			continue
		}
		var otherDirs []string
		switch entry.scope {
		case "890":
			otherDirs = []string{"core/driver/ts990", "internal/fakets990"}
		case "990":
			otherDirs = []string{"core/driver/ts890", "internal/fakets890"}
		}
		for _, od := range otherDirs {
			if dir != od {
				continue
			}
			if kwScopeMentionRE(entry.scope).MatchString(c.text) || kwNonApplicabilityRE.MatchString(c.text) {
				continue
			}
			t.Errorf("%s:%d: cites A%d, which the register scopes TS-%sS ONLY, from the OTHER row's %s — comment: %q", c.relPath, c.startLine, n, entry.scope, dir, c.text)
		}
	}
}
