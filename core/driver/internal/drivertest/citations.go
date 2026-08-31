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
// mean something. Two shapes are deliberately absent:
//
//   - A bare "Erratum 5". In these packages that number belongs to the tier
//     additions spec, not to the model's matrix ("per additions-spec Erratum
//     5", "4b Erratum 1", "Erratum 6"), and the additions spec is shared by
//     every model — it is not a provenance question for one radio. Only the
//     qualified "matrix erratum N" form is a claim about a radio's matrix, so
//     only that form is extracted.
//   - A bare "§N". "doc.go §6c" is an internal cross-reference to this
//     package's own prose, not a citation, so the section shape requires the
//     word "matrix" in front of it.
var citationShapes = []struct {
	re    *regexp.Regexp
	canon func([]string) string
}{
	{
		// "PDF p.263", "PDF page 20", "PDF pages 250-265", "PDF pp. 228-229".
		re: regexp.MustCompile(`PDF (?:pp?\.|pages? ) ?(\d+)(?:-(\d+))?`),
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
		re: regexp.MustCompile(`(?:(IC-[0-9A-Za-z]+) )?(?i:matrix) (?:§ ?|section )(\d+[a-z]?(?:\.\d+)*)`),
		canon: func(m []string) string {
			if m[1] != "" {
				return m[1] + " matrix §" + m[2]
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
			if documentName.MatchString(token) {
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
					if documentName.MatchString(token) {
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
	// Authority names the model's matrix trio. These live under
	// docs/superpowers/, which is gitignored: present in a working checkout,
	// absent on CI and in a fresh clone (the v1.2.1 CI run, 30/08/2026).
	Authority []string
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
	authority := []string{
		filepath.Join(matrices, model+"-capability-matrix.md"),
		filepath.Join(matrices, model+"-capability-matrix-report.md"),
		filepath.Join(matrices, model+"-capability-matrix-review.md"),
	}
	// The implementation plan is authority for the ASSUMPTION-REGISTER NAMES
	// only, and it earns that place: the matrix grades a D8 field in §1b.2
	// without always minting an id for it, and core/civ/icr8600/doc.go:77
	// already attributes one id to the plan in so many words ("called
	// icr8600-digital-tail-template by the implementation plan"). An id in
	// neither document is an invented one, which is finding F3's whole point.
	authority = append(authority, plans...)
	return CitationPin{
		Model: model,
		Dirs:  []string{".", filepath.Join("..", "..", "civ", model)},
		// provenance_test.go is excluded because it IS the blacklist: the
		// foreign literals it forbids necessarily appear in it, and a scan
		// that read them would demand the list allow exactly what that file
		// exists to reject.
		Exclude:   []string{"provenance_test.go"},
		ListPath:  filepath.Join("testdata", "citations.txt"),
		Authority: authority,
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

	authority, err := readAuthority(p.Authority)
	if err != nil {
		t.Fatalf("read authority: %v", err)
	}
	if !authority.present {
		t.Logf("matrix authority absent (docs/superpowers is gitignored; not in this checkout) — "+
			"authority supply checks skipped, the %s list is still enforced", p.ListPath)
		return
	}
	supplied := extractTokens(authority.norm)
	for token := range allowed {
		if isForeign(token, p.Model) || authoritySupplies(authority, supplied, token) {
			continue
		}
		t.Errorf("%s allows %q, but the %s authority does not supply it", p.ListPath, token, p.Model)
	}
}

// authoritySupplies answers one token against the authority. Most tokens are
// settled by running the same extractor over the authority and comparing, but
// three shapes are written differently there and get their own reading.
func authoritySupplies(a authorityText, supplied map[string]bool, token string) bool {
	switch {
	case strings.HasPrefix(token, "folio "):
		return suppliedByFolio(a.norm, token)
	case strings.HasPrefix(token, "PDF pp."):
		return supplied[token] || suppliedByPageBounds(supplied, token)
	case strings.HasPrefix(token, "matrix §"):
		return suppliedBySection(a.raw, token)
	case strings.HasPrefix(token, "matrix erratum "):
		return suppliedByErratum(a.raw, token)
	}
	return supplied[token]
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

// suppliedBySection answers a "matrix §S" token against the authority's
// headings, which number themselves "## §3" and "### 3.15.1" rather than
// repeating the word "matrix".
func suppliedBySection(authority, token string) bool {
	s, ok := strings.CutPrefix(token, "matrix §")
	if !ok {
		return false
	}
	re := regexp.MustCompile(`(?m)^#{1,6} +§?` + regexp.QuoteMeta(s) + `(?:[^0-9.]|$)`)
	return re.MatchString(authority)
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
	out := authorityText{}
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return authorityText{}, err
		}
		out.present = true
		joined.Write(b)
		joined.WriteByte('\n')
	}
	if !out.present {
		return out, nil
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
