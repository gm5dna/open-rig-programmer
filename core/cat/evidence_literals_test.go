// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const evidenceLiteralsPath = "testdata/evidence-literals.golden"

// literalRecord is one STRING, CHAR or INT literal identified by WHERE it
// is and WHICH occurrence it is — not merely by its spelling. That is what
// lets the pin detect a literal being moved, orphaned or deleted while an
// identical spelling survives elsewhere.
type literalRecord struct {
	file    string
	ordinal int
	token   string // strconv.Quote'd, so every record is exactly one line
}

func (r literalRecord) String() string {
	return fmt.Sprintf("%s\t%d\t%s", r.file, r.ordinal, r.token)
}

// collectTestStringLiterals walks this package's evidence test files in a
// stable order, recording every STRING, CHAR and INT literal with its file
// and ordinal. (Name kept from revision 1 for git-blame continuity; it now
// collects more than strings — see the INT/CHAR handling below, fix-round
// finding I1.)
func collectTestStringLiterals(t *testing.T) []literalRecord {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, "_test.go") {
			continue
		}
		// The pin's own tooling is not evidence; its literals would churn
		// as the tooling evolves.
		//
		// parsercorpus_test.go is DELIBERATELY NOT in this list (fix-round
		// finding C2): goldenMRFramesForCorpus there holds the three MR
		// golden frames copied verbatim from mr_test.go — hardware- and
		// manual-derived evidence, exactly the kind of literal this pin
		// exists to protect. Excluding it as "tooling" let a one-character
		// edit to a golden MR frame pass silently.
		switch n {
		case "evidence_literals_test.go", "framecorpus_test.go", "allowlistcorpus_test.go",
			"dialect_test.go", "seconddialect_test.go":
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)

	if len(names) < 10 {
		t.Fatalf("only found %d evidence test files — the walker or its filter is broken, and this check would pass vacuously", len(names))
	}

	var out []literalRecord
	fset := token.NewFileSet()
	for _, name := range names {
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		n := 0
		ast.Inspect(f, func(node ast.Node) bool {
			bl, ok := node.(*ast.BasicLit)
			if !ok {
				return true
			}
			// STRING and CHAR literals share Go's quoting rules, so both
			// unquote/re-quote the same way (a backtick and a
			// double-quoted form of the same value compare equal). INT
			// literals are not quoted at all; bl.Value already IS the
			// exact source text (e.g. "29_620_000" or "0x1F"), so it is
			// quoted as-is rather than evaluated — this pin cares whether
			// the SOURCE TEXT changed, not its numeric value, and
			// evaluating first would make "007" and "7" compare equal.
			//
			// CHAR and INT were added in the fix round (finding I1):
			// without them, hardware-derived evidence expressed as a
			// number rather than a string — e.g. hw_derived_test.go's live
			// capture FreqHz/Mode/CTCSS/Shift fields, or mr_test.go's whole
			// MemoryData{...} want literals — was invisible to this pin.
			switch bl.Kind {
			case token.STRING, token.CHAR:
				val, err := strconv.Unquote(bl.Value)
				if err != nil {
					val = bl.Value
				}
				out = append(out, literalRecord{file: name, ordinal: n, token: strconv.Quote(val)})
				n++
			case token.INT:
				out = append(out, literalRecord{file: name, ordinal: n, token: strconv.Quote(bl.Value)})
				n++
			}
			return true
		})
	}
	return out
}

// TestEvidenceLiterals_OrderedRecordsSurvive asserts every pre-M9b
// literal is still in the same file at the same ordinal.
//
// If this fails because a task legitimately ADDED a literal mid-file,
// that is a conversation to have, not a golden file to regenerate.
func TestEvidenceLiterals_OrderedRecordsSurvive(t *testing.T) {
	current := collectTestStringLiterals(t)

	raw, err := os.ReadFile(filepath.FromSlash(evidenceLiteralsPath))
	if err != nil {
		t.Fatalf("reading %s: %v", evidenceLiteralsPath, err)
	}

	byKey := map[string]string{}
	for _, r := range current {
		byKey[fmt.Sprintf("%s\t%d", r.file, r.ordinal)] = r.token
	}

	var problems []string
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			t.Fatalf("malformed golden line %q", line)
		}
		key := parts[0] + "\t" + parts[1]
		got, present := byKey[key]
		switch {
		case !present:
			problems = append(problems, fmt.Sprintf("%s: literal #%s is gone (was %s)", parts[0], parts[1], parts[2]))
		case got != parts[2]:
			problems = append(problems, fmt.Sprintf("%s: literal #%s changed\n    was: %s\n    now: %s", parts[0], parts[1], parts[2], got))
		}
	}

	if len(problems) > 0 {
		shown := problems
		if len(shown) > 15 {
			shown = shown[:15]
		}
		t.Fatalf("%d expected literal(s) changed or vanished:\n  %s\n\nA call-site rewrite must not touch expected VALUES. Do NOT regenerate the golden file.",
			len(problems), strings.Join(shown, "\n  "))
	}
}
