// SPDX-License-Identifier: GPL-3.0-or-later

package drivertest

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// This file is the POSITIVE half of the provenance pins. The blacklist tests
// beside the drivers (core/driver/ic7760/provenance_test.go) name literals
// that a rejected branch actually copied from another radio; they are cheap
// and they document real contaminations, but a paraphrase walks past them.
// The positive list asks the opposite question: of every document citation
// this package makes, can the model's OWN authority supply it?

// Citation is one document citation found in a package's Go sources, with
// the place it was written so a failure can be walked straight to.
type Citation struct {
	// Token is the canonical form: "PDF p.263", "folio 18-14",
	// "matrix §3.15.1", "matrix erratum 12", "ic7851-serial-framing".
	Token string
	File  string
	Line  int
}

// normalised is source text with comment markers and line wrapping removed,
// carrying the source line of every byte it holds.
type normalised struct {
	text string
	line []int
}

// lineAt reports the source line of the byte at offset off.
func (n normalised) lineAt(off int) int {
	if off < 0 || off >= len(n.line) {
		return 0
	}
	return n.line[off]
}

// normalise strips "//", folds every en/em dash to a plain hyphen, collapses
// each run of whitespace to one space, and records the source line of each
// byte it keeps.
//
// THE NORMALISATION IS LOAD-BEARING, not tidiness, and it does three jobs:
//
//   - A citation that wraps across two comment lines is "PDF\n// p.263" on
//     disk, and a naive scan never sees it. The existing blacklist helper
//     (core/driver/ic7760/provenance_test.go) collapses whitespace for the
//     same reason: one IC-7610 lift identifier survived that sweep's first
//     pass on exactly this trick.
//   - A register entry wraps at its own hyphen — core/driver/ic7851/doc.go:494
//     writes "ic7851-serial-\n// framing" — so a whitespace run that CONTAINED
//     A NEWLINE and follows a hyphen is dropped rather than folded to a space.
//     Without that the extractor invents the token "ic7851-serial", which no
//     authority can supply, and the list would have to carry a lie to go green.
//   - Icom's documents and this repository's prose both print page RANGES with
//     an en dash whilst the Go sources type a hyphen. Folding both to "-" lets
//     the same extractor run over a package and over its authority and have
//     their tokens compare.
func normalise(src string) normalised {
	buf := make([]byte, 0, len(src))
	lines := make([]int, 0, len(src))
	line := 1
	pendingSpace, pendingNewline := false, false
	emit := func(c byte) {
		buf = append(buf, c)
		lines = append(lines, line)
	}
	for i := 0; i < len(src); {
		if strings.HasPrefix(src[i:], "–") || strings.HasPrefix(src[i:], "—") {
			if pendingSpace && len(buf) > 0 {
				emit(' ')
			}
			pendingSpace, pendingNewline = false, false
			emit('-')
			i += len("–")
			continue
		}
		if src[i] == '/' && i+1 < len(src) && src[i+1] == '/' {
			pendingSpace = true
			i += 2
			continue
		}
		c := src[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			pendingSpace = true
			if c == '\n' {
				pendingNewline = true
				line++
			}
			i++
			continue
		}
		if pendingSpace {
			joinsHyphen := pendingNewline && len(buf) > 0 && buf[len(buf)-1] == '-'
			if len(buf) > 0 && !joinsHyphen {
				emit(' ')
			}
			pendingSpace, pendingNewline = false, false
		}
		emit(c)
		i++
	}
	return normalised{text: string(buf), line: lines}
}

// The citation grammar. Each shape is here because the token ITSELF names the
// document that has to supply it, which is what makes a positive list over it
// mean something.
//
// TWO EARLIER EXCLUSIONS WERE TOO WIDE, and fix round 1 narrowed them:
//
//   - A page used to need the "PDF" prefix. Nine sites cite one bare —
//     core/driver/ic7851/read.go:22 ("p.263 (folio 18-14)"),
//     core/driver/ic7760/read.go:22 ("p.20 (folio 19)") — and that is the
//     EXACT contamination shape this pin exists for: the IC-7610 leak was
//     PDF p.11/p.12/p.14, so a paraphrase writing "p.11" walked past the
//     blacklist AND this list. Only the "p."/"pp." abbreviation is admitted
//     bare; the spelled-out "page 20" still needs its "PDF", because a bare
//     "page N" is ordinary prose rather than a citation.
//   - A section used to need the word "matrix". That rationale — "doc.go §6c"
//     is an internal cross-reference, not a citation — is sound only for the
//     FILE-QUALIFIED form. caps.go cites the matrix by bare section
//     throughout, and a section number attached to a MANUAL-EVIDENCED grading
//     IS the provenance for that capability row. The exclusion is now
//     "<file>.go §N" alone, and a bare "§N" canonicalises to the same
//     "matrix §N" token as the qualified form, so the two collapse rather
//     than double-count.
//
// One exclusion stands:
//
//   - A bare "Erratum 5". In these packages that number belongs to the tier
//     additions spec, not to the model's matrix ("per additions-spec Erratum
//     5", "4b Erratum 1", "Erratum 6"), and the additions spec is shared by
//     every model — it is not a provenance question for one radio. Only the
//     qualified "matrix erratum N" form is a claim about a radio's matrix, so
//     only that form is extracted.
// sectionNumber matches a matrix section: "1", "1b", "3.15.1", "3.12a".
const sectionNumber = `\d+[a-z]?(?:\.\d+[a-z]?)*`

var citationShapes = []struct {
	re    *regexp.Regexp
	canon func([]string) string
}{
	{
		// "PDF p.263", "PDF page 20", "PDF pages 250-265", "PDF pp. 228-229",
		// and the bare "p.20" / "pp. 361-375" the packages also write.
		re: regexp.MustCompile(`(?:PDF (?:pp?\.|pages? )|\bpp?\.) ?(\d+)(?:-(\d+))?`),
		canon: func(m []string) string {
			if m[2] == "" {
				return "PDF p." + m[1]
			}
			return "PDF pp." + m[1] + "-" + m[2]
		},
	},
	{
		// "folio 19", "folio 18-14", "printed folios 18-1 to 18-16".
		re:    regexp.MustCompile(`\bfolios? (\d+(?:-\d+)?)`),
		canon: func(m []string) string { return "folio " + m[1] },
	},
	{
		// "matrix §3.15.1", "MATRIX §3.15", "Matrix section 1", "matrix
		// section 1b". An "IC-7300 matrix §..." qualifier is kept in the
		// token: it is the prose's own attribution and the list must show it.
		re: regexp.MustCompile(`(?:(IC-[0-9A-Za-z]+) )?(?i:matrix) (?:§ ?|section )(` + sectionNumber + `)`),
		canon: func(m []string) string {
			if m[1] != "" {
				return m[1] + " matrix §" + m[2]
			}
			return "matrix §" + m[2]
		},
	},
	{
		// A BARE section mark, "§3.4" — the dominant form in caps.go, and the
		// one the first round missed. It canonicalises to the same token as
		// the qualified shape above, so the two collapse.
		//
		// A "matrix §" hit is left to that shape, so an "IC-nnnn matrix §..."
		// qualifier the prose wrote is not silently dropped here; and a
		// FILE-QUALIFIED "doc.go §6c" is dropped, because it points at this
		// package's own prose rather than at any document.
		re: regexp.MustCompile(`([A-Za-z0-9_./-]+ )?§ ?(` + sectionNumber + `)`),
		canon: func(m []string) string {
			lead := strings.TrimSpace(m[1])
			if strings.HasSuffix(lead, ".go") || strings.EqualFold(lead, "matrix") {
				return ""
			}
			return "matrix §" + m[2]
		},
	},
	{
		// "IC-7300 matrix erratum 12", "matrix errata 1-10".
		re: regexp.MustCompile(`(?:(IC-[0-9A-Za-z]+) )?(?i:matrix) (?i:erratum|errata) (\d+(?:-\d+)?)`),
		canon: func(m []string) string {
			if m[1] != "" {
				return m[1] + " matrix erratum " + m[2]
			}
			return "matrix erratum " + m[2]
		},
	},
	{
		// Assumption-register entry ids: "ic7851-serial-framing",
		// "icr8600-mode-wire-codes". Finding F3 of the IC-7760 sweep was a
		// set of register names INVENTED for the package instead of taken
		// from the matrix's register column; the blacklist names the five
		// that were caught, and this shape generalises it to all of them.
		re:    regexp.MustCompile(`\bicr?\d+[a-z0-9]*-[a-z0-9]+(?:-[a-z0-9]+)*\b`),
		canon: func(m []string) string { return m[0] },
	},
}

// documentName matches the matrix trio's own filenames, which the entry shape
// would otherwise report as register entries. They are the authority, not a
// citation into it, and they are already SHA-pinned by each package's freeze
// and crosscheck tests.
var documentName = regexp.MustCompile(`^icr?\d+[a-z0-9]*-capability-matrix(?:-report|-review)?$`)

// extractTokens returns the canonical citation tokens in already-normalised
// text, without positions. It is what runs over an AUTHORITY document.
func extractTokens(text string) map[string]bool {
	found := map[string]bool{}
	for _, shape := range citationShapes {
		for _, m := range shape.re.FindAllStringSubmatch(text, -1) {
			token := shape.canon(m)
			if token == "" || documentName.MatchString(token) {
				continue
			}
			found[token] = true
		}
	}
	return found
}

// ScanCitations returns every citation in the .go files of the given
// directories, first occurrence per token, sorted by token.
//
// Test files are scanned too, and deliberately: finding F2's wrong-address
// lifts survived in test-file comments after the production sites were
// corrected. A sweep restricted to doc.go and caps.go would miss them again.
func ScanCitations(t testing.TB, dirs []string, exclude []string) []Citation {
	t.Helper()
	skip := map[string]bool{}
	for _, name := range exclude {
		skip[name] = true
	}
	seen := map[string]Citation{}
	var files int
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read dir %s: %v", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || skip[e.Name()] {
				continue
			}
			path := filepath.Join(dir, e.Name())
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			files++
			n := normalise(string(b))
			for _, shape := range citationShapes {
				for _, loc := range shape.re.FindAllStringSubmatchIndex(n.text, -1) {
					m := make([]string, len(loc)/2)
					for g := range m {
						if loc[2*g] >= 0 {
							m[g] = n.text[loc[2*g]:loc[2*g+1]]
						}
					}
					token := shape.canon(m)
					if token == "" || documentName.MatchString(token) {
						continue
					}
					if _, dup := seen[token]; dup {
						continue
					}
					seen[token] = Citation{Token: token, File: path, Line: n.lineAt(loc[0])}
				}
			}
		}
	}
	if files == 0 {
		t.Fatalf("no .go sources found under %v", dirs)
	}
	out := make([]Citation, 0, len(seen))
	for _, c := range seen {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Token < out[j].Token })
	return out
}

// CitationPin is one package's positive-list provenance check.
type CitationPin struct {
	// Model is the lowercase register prefix, "ic7851" or "icr8600". A
	// register entry that does not start with it, and any citation carrying
	// an explicit "IC-nnnn" qualifier, is FOREIGN-ATTRIBUTED: the prose names
	// whose document supplies it, so this model's authority is not asked to.
	// It must still be in the list, because the list is what a reviewer reads.
	Model string
	// Dirs are the package directories to scan (driver and civ).
	Dirs []string
	// Exclude names files to skip by base name.
	Exclude []string
	// ListPath is the checked-in allowed-citation list.
	ListPath string
	// Matrix is the capability matrix FILE ITSELF, and MatrixSupport is its
	// report and its review. Together they are the only authority for page,
	// folio, section and erratum citations — but SECTION citations are
	// answered from Matrix ALONE.
	//
	// That second split is structural for the same reason the first one is.
	// The report and the review NUMBER THEIR OWN SECTIONS, and a driver cites
	// the matrix, not its review. Reading sections out of the concatenated
	// trio admitted "matrix §5.1" and "matrix §7" for the IC-7100 against a
	// matrix that heads neither: §5.1 is the report's own numbering
	// (ic7100-capability-matrix-report.md:307) and §7 is the review's
	// (ic7100-capability-matrix-review.md:170).
	//
	// Plans names the model's implementation plan, and it may answer
	// REGISTER-ID citations ALONE. That split is the orchestrator's ruling and
	// it has to be structural, not a comment: the plans carry page and folio
	// citations of their own (docs/superpowers/plans/2026-08-28-icom-ic7100.md
	// cites PDF p.174 and PDF p.317; …-icr8600.md:85 cites folios 3/9/18), so
	// a single concatenated authority would let a plan answer a page claim
	// about the radio. A register ID is a different kind of thing: it is a
	// name this project mints for an open question, and the plan is where
	// several were minted — core/civ/icr8600/doc.go:77 says so outright
	// ("called icr8600-digital-tail-template by the implementation plan").
	// An id in neither document is an invented one, which is finding F3's
	// whole point.
	//
	// Both live under docs/superpowers/, which is gitignored: present in a
	// working checkout, absent on CI and in a fresh clone (the v1.2.1 CI run,
	// 30/08/2026).
	Matrix        string
	MatrixSupport []string
	Plans         []string
}

// IcomCitationPin builds the standard pin for an Icom model whose driver is
// core/driver/<model> and whose dialect is core/civ/<model>. Every path is
// relative to the DRIVER package directory, because that is where the test
// runs; keeping the convention here is what stops four packages drifting into
// four slightly different ideas of where their authority lives.
func IcomCitationPin(model string) CitationPin {
	root := filepath.Join("..", "..", "..")
	matrices := filepath.Join(root, "docs", "superpowers", "icom-matrices")
	// The plan is dated in its filename, so it is matched rather than spelled.
	plans, _ := filepath.Glob(filepath.Join(root, "docs", "superpowers", "plans", "*-icom-"+model+".md"))
	return CitationPin{
		Model: model,
		Dirs:  []string{".", filepath.Join("..", "..", "civ", model)},
		// provenance_test.go is excluded because it IS the blacklist: the
		// foreign literals it forbids necessarily appear in it, and a scan
		// that read them would demand the list allow exactly what that file
		// exists to reject.
		Exclude:  []string{"provenance_test.go"},
		ListPath: filepath.Join("testdata", "citations.txt"),
		Matrix: filepath.Join(matrices, model+"-capability-matrix.md"),
		MatrixSupport: []string{
			filepath.Join(matrices, model+"-capability-matrix-report.md"),
			filepath.Join(matrices, model+"-capability-matrix-review.md"),
		},
		Plans: plans,
	}
}

// Assert fails if the package cites anything the list does not allow, if the
// list allows anything the package no longer cites, or — where the authority
// documents are present — if the authority cannot supply a listed token.
func (p CitationPin) Assert(t testing.TB) {
	t.Helper()
	allowed, err := readCitationList(p.ListPath)
	if err != nil {
		t.Fatalf("read %s: %v", p.ListPath, err)
	}
	cited := ScanCitations(t, p.Dirs, p.Exclude)

	citedSet := map[string]bool{}
	for _, c := range cited {
		citedSet[c.Token] = true
		if !allowed[c.Token] {
			t.Errorf("%s:%d cites %q, which %s is not allowed to claim: it is not in %s.\n"+
				"Add it ONLY after checking the model's own authority prints it; if the authority "+
				"cannot supply it, the prose is wrong, not the list.",
				c.File, c.Line, c.Token, p.Model, p.ListPath)
		}
	}
	for token := range allowed {
		if !citedSet[token] {
			t.Errorf("%s allows %q, which the package no longer cites; a list that outlives its "+
				"citations stops being evidence", p.ListPath, token)
		}
	}

	matrix, err := readAuthority([]string{p.Matrix})
	if err != nil {
		t.Fatalf("read matrix authority: %v", err)
	}
	support, err := readAuthority(p.MatrixSupport)
	if err != nil {
		t.Fatalf("read matrix support authority: %v", err)
	}
	plans, err := readAuthority(p.Plans)
	if err != nil {
		t.Fatalf("read plan authority: %v", err)
	}
	// Presence is ALL-OR-NOTHING across both. A checkout holding some of the
	// documents but not others would run supply checks against a partial
	// authority and fail honest citations, which is a worse failure than not
	// checking: the checked-in list is enforced above either way.
	if !matrix.present || !support.present || !plans.present {
		t.Logf("matrix authority absent (docs/superpowers is gitignored; not in this checkout) — "+
			"authority supply checks skipped, the %s list is still enforced", p.ListPath)
		return
	}
	// THREE READINGS, and which one a token gets is the ruling. The trio
	// answers pages, folios and errata; the matrix ALONE answers sections;
	// the trio plus the plans answers register ids and nothing else.
	trio := authorityText{
		raw:  matrix.raw + "\n" + support.raw,
		norm: matrix.norm + "\n" + support.norm,
	}
	matrixTokens := extractTokens(trio.norm)
	registerTokens := extractTokens(trio.norm + "\n" + plans.norm)
	for token := range allowed {
		if isForeign(token, p.Model) || authoritySupplies(trio, matrix.raw, matrixTokens, registerTokens, token) {
			continue
		}
		t.Errorf("%s allows %q, but the %s authority does not supply it (a page, folio, section or "+
			"erratum must come from the capability matrix; only a register id may come from the "+
			"implementation plan)", p.ListPath, token, p.Model)
	}
}

// authoritySupplies answers one token. Most are settled by running the same
// extractor over the authority and comparing; three shapes are written
// differently there and get their own reading.
//
// matrixTokens is the capability-matrix trio. registerTokens is that trio plus
// the implementation plan, and ONLY a register id is asked of it. matrixProper
// is the matrix FILE, and only a section is asked of it — see CitationPin.Matrix
// for why both splits have to be here rather than in a comment.
func authoritySupplies(trio authorityText, matrixProper string, matrixTokens, registerTokens map[string]bool, token string) bool {
	switch {
	case registerEntry.MatchString(token):
		return registerTokens[token]
	case strings.HasPrefix(token, "folio "):
		return suppliedByFolio(trio.norm, token)
	case strings.HasPrefix(token, "PDF pp."):
		return matrixTokens[token] || suppliedByPageBounds(matrixTokens, token)
	case strings.HasPrefix(token, "matrix §"):
		return suppliedBySection(matrixProper, token)
	case strings.HasPrefix(token, "matrix erratum "):
		return suppliedByErratum(trio.raw, token)
	}
	return matrixTokens[token]
}

var folioNumber = regexp.MustCompile(`^\d+(?:-\d+)?$`)

// suppliedByFolio answers a "folio F" token. A matrix names a specific folio
// as "folio 18-14", but states the document's EXTENT in its §0 source table as
// "printed folios 1-26" or "printed 18-1 ... 18-16" — the same claim, without
// the word repeated. All three lead-ins count, and the trailing guard stops
// folio 18-1 being answered by folio 18-16.
func suppliedByFolio(authority, token string) bool {
	f := strings.TrimPrefix(token, "folio ")
	if !folioNumber.MatchString(f) {
		return false
	}
	re := regexp.MustCompile(`(?:folios?|printed) (?:folios? )?` + regexp.QuoteMeta(f) + `(?:[^0-9-]|$)`)
	return re.MatchString(authority)
}

// suppliedByPageBounds answers a page RANGE the authority does not print as a
// range. core/driver/ic7100/e2e_test.go:30 cites "PDF pp. 361-375" for the
// span two independent transcriptions read; the IC-7100 matrix cites p.361 and
// p.375 (and most pages between) individually and never as a span. A range is
// a claim about its bounds, so the authority supplies it when it supplies both.
func suppliedByPageBounds(supplied map[string]bool, token string) bool {
	span := strings.TrimPrefix(token, "PDF pp.")
	lo, hi, ok := strings.Cut(span, "-")
	if !ok {
		return false
	}
	return supplied["PDF p."+lo] && supplied["PDF p."+hi]
}

// registerEntry recognises the register-entry shape at the start of a token.
var registerEntry = regexp.MustCompile(`^icr?\d`)

// isForeign reports whether the prose itself attributes the token to another
// model's document, in which case this model's authority is the wrong place
// to look for it. The IC-7300's tone-domain doctrine is cited by name in three
// of these four packages ("IC-7300 matrix erratum 12"); that citation is
// honest and must stay listed, but the IC-7100 matrix will never print it.
func isForeign(token, model string) bool {
	if strings.HasPrefix(token, "IC-") {
		return true
	}
	if registerEntry.MatchString(token) {
		return !strings.HasPrefix(token, model+"-")
	}
	return false
}

// letteredSubPart splits "3.12a" into "3.12" and true. A bare numeric section
// returns false: only a LETTERED sub-part may be answered by a cross-reference.
var letteredSubPart = regexp.MustCompile(`^(\d+(?:\.\d+)*)[a-z]$`)

// suppliedBySection answers a "matrix §S" token against THE MATRIX FILE, never
// its report or its review, and in one of two ways.
//
// THE ORDINARY WAY IS A HEADING, numbered "## §3" or "### 3.15.1" rather than
// repeating the word "matrix". Every listed section but one is answered here.
//
// THE SECOND WAY IS DELIBERATELY NARROW, and it exists for exactly one real
// citation. core/driver/ic7851/doc.go:64 cites §3.12a; the IC-7851 matrix heads
// only "### 3.12 Probe identity" (:1071) and then labels §3.12a six times in
// its own body, its own register table among them (:636, 1632, 1683, 1760,
// 1775, 1792). A document that labels a sub-part six times has supplied that
// label. So a cross-reference is accepted ONLY for a lettered sub-part WHOSE
// PARENT IS HEADED — never for a bare numbered section.
//
// THAT NARROWNESS IS THE POINT. A first attempt accepted any cross-reference
// anywhere in the trio, which let a matrix supply a section by DENYING it
// ("there is NO §3.20 in this matrix") or by citing another document's
// section, and let the report's and review's own numbering answer for the
// matrix — it admitted "matrix §5.1" and "matrix §7" for the IC-7100, which
// heads neither. A driver could then have minted "§7 — the boundary
// statement", listed it, and gone green against a matrix with no §7: a false
// provenance admitted by the pin that exists to refuse exactly that.
//
// Both arms share one trailing guard so the two agree: §3.15 is answered by
// neither a "### 3.15.1" heading nor a "§3.15.1" mention, and §3.12 is
// answered by neither a "### 3.12a" heading nor a "§3.12a" mention.
// TestSuppliedBySection pins every one of those cases.
func suppliedBySection(matrixProper, token string) bool {
	s, ok := strings.CutPrefix(token, "matrix §")
	if !ok {
		return false
	}
	if sectionHasHeading(matrixProper, s) {
		return true
	}
	parent := letteredSubPart.FindStringSubmatch(s)
	if parent == nil {
		return false
	}
	crossRef := regexp.MustCompile(`§ ?` + regexp.QuoteMeta(s) + sectionGuard)
	return sectionHasHeading(matrixProper, parent[1]) && crossRef.MatchString(matrixProper)
}

// sectionGuard stops a section number being answered by a longer one that
// merely starts with it — neither a deeper number nor a lettered sub-part.
const sectionGuard = `(?:[^0-9.a-z]|$)`

// sectionHasHeading reports whether the matrix heads a section numbered s.
func sectionHasHeading(matrixProper, s string) bool {
	return regexp.MustCompile(`(?m)^#{1,6} +§?` + regexp.QuoteMeta(s) + sectionGuard).MatchString(matrixProper)
}

// suppliedByErratum answers a "matrix erratum N" token: the matrix prints its
// errata as "Erratum N", without repeating "matrix".
func suppliedByErratum(authority, token string) bool {
	n, ok := strings.CutPrefix(token, "matrix erratum ")
	if !ok {
		return false
	}
	re := regexp.MustCompile(`(?i:errat(?:um|a)) ` + regexp.QuoteMeta(n) + `(?:[^0-9]|$)`)
	return re.MatchString(authority)
}

// readCitationList reads one token per line; "#" starts a comment, and a
// token may carry a trailing "  # why" note for the reader.
func readCitationList(path string) (map[string]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		if token := strings.TrimSpace(line); token != "" {
			out[token] = true
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s lists no citations", path)
	}
	return out, nil
}

// authorityText is a model's matrix trio, both as written (headings intact,
// for the section lookup) and normalised (for the token comparison).
type authorityText struct {
	raw     string
	norm    string
	present bool
}

// readAuthority concatenates the authority documents that exist. It follows
// core/civ/ic7100/crosscheck_test.go's idiom exactly: a missing file is the
// normal gitignored case and is reported as absent, any other error is fatal.
func readAuthority(paths []string) (authorityText, error) {
	var joined strings.Builder
	out := authorityText{present: true}
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			out.present = false
			continue
		}
		if err != nil {
			return authorityText{}, err
		}
		joined.Write(b)
		joined.WriteByte('\n')
	}
	if !out.present {
		return authorityText{}, nil
	}
	// The authorities are Markdown and emphasise their own key values, so the
	// IC-7760 matrix's source table reads "printed folios **1–26**". The
	// emphasis is typography, not part of the citation, and leaving it in
	// hides a folio the document plainly states.
	out.raw = markdownEmphasis.ReplaceAllString(joined.String(), "")
	out.norm = normalise(out.raw).text
	return out, nil
}

var markdownEmphasis = regexp.MustCompile("[*`_]")
