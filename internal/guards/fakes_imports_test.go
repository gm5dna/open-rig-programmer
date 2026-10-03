// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// fakepipeImport is the ONE project-internal import any fake may have:
// internal/fakepipe is protocol-free plumbing (see internal/fakeradio/doc.go,
// THE HARD RULE). fakepipe itself imports nothing project-internal.
const fakepipeImport = modulePrefix + "internal/fakepipe"

// fakeAllowed lists the bespoke rows: fakepipe imports nothing project-
// internal; fakeyaesuclone (a clone-image fake, never fenced before) also
// shares core/clonewire's image framing. Every other fake gets fakepipe only.
var fakeAllowed = map[string][]string{
	"fakepipe":       nil,
	"fakeyaesuclone": {fakepipeImport, modulePrefix + "core/clonewire"},
}

// fakeImportViolations walks root and every directory beneath it, parsing each
// non-test .go file's import block, and returns the files and imports that are
// project-internal and not in allowed. _test.go files and testdata/ are
// skipped: the rule is about what the PACKAGE depends on. It also returns how
// many files and imports it saw, for the vacuity checks.
func fakeImportViolations(root string, allowed ...string) (bad []string, files, imports int, err error) {
	fset := token.NewFileSet()
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if perr != nil {
			return perr
		}
		files++
		for _, imp := range f.Imports {
			p, uerr := strconv.Unquote(imp.Path.Value)
			if uerr != nil {
				return uerr
			}
			imports++
			if strings.HasPrefix(p, modulePrefix) && !sliceHas(allowed, p) {
				bad = append(bad, path+" imports "+p)
			}
		}
		return nil
	})
	return bad, files, imports, err
}

func sliceHas(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// TestFakesImportNothingProjectInternal enforces THE HARD RULE (see
// internal/fakeradio/doc.go) for every internal/fake* directory, recursively:
// a fake may import only the standard library, third-party packages and
// internal/fakepipe, so a systematic bug in the production codec cannot pass
// end-to-end tests invisibly. fakepipe itself may import nothing
// project-internal. The guards package sits outside the fakes, so the fakes
// still share no code with each other or with core/.
func TestFakesImportNothingProjectInternal(t *testing.T) {
	dirs, err := filepath.Glob(filepath.Join(repoRoot(t), "internal", "fake*"))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, dir := range dirs {
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			continue
		}
		name := filepath.Base(dir)
		t.Run(name, func(t *testing.T) {
			allowed := fakeAllowed[name]
			if _, ok := fakeAllowed[name]; !ok {
				allowed = []string{fakepipeImport}
			}
			bad, files, imports, err := fakeImportViolations(dir, allowed...)
			if err != nil {
				t.Fatal(err)
			}
			for _, b := range bad {
				t.Errorf("%s — %s MUST NOT import any project-internal package beyond internal/fakepipe (THE HARD RULE, internal/fakeradio/doc.go)", b, name)
			}
			if files == 0 || imports == 0 {
				t.Fatalf("scanned %d files and %d imports — the walk or filter is broken and this test would pass vacuously", files, imports)
			}
		})
		checked++
	}
	if checked < 40 {
		t.Fatalf("found only %d internal/fake* directories, want at least 40 — the glob is broken", checked)
	}
}

// TestFakeImportViolations_Self proves the scan: it is recursive, honours the
// allow-list, rejects the repo directory name as a module path (no match, no
// violation) and skips _test.go files and testdata/.
func TestFakeImportViolations_Self(t *testing.T) {
	root := t.TempDir()
	for rel, content := range map[string]string{
		"clean.go":           "package p\n\nimport \"io\"\n\nvar _ io.Reader\n",
		"pipe.go":            "package p\n\nimport _ \"" + fakepipeImport + "\"\n",
		"gen/main.go":        "package main\n\nimport _ \"" + modulePrefix + "internal/extable\"\n",
		"gen/main_test.go":   "package main\n\nimport _ \"" + modulePrefix + "core/cat\"\n",
		"testdata/vendor.go": "package fixture\n\nimport _ \"" + modulePrefix + "core/spec\"\n",
		"lookalike.go":       "package p\n\nimport _ \"ft710-programmer/core/cat\"\n",
		"fork.go":            "package p\n\nimport _ \"github.com/someone/open-rig-programmer-fork/core/cat\"\n",
	} {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	bad, files, _, err := fakeImportViolations(root, fakepipeImport)
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) != 1 || !strings.Contains(bad[0], filepath.Join("gen", "main.go")) || !strings.HasSuffix(bad[0], "internal/extable") {
		t.Fatalf("violations = %q, want exactly gen/main.go importing internal/extable", bad)
	}
	if files != 5 {
		t.Errorf("parsed %d files, want 5 (the _test.go and the testdata fixture are skipped)", files)
	}

	// With no allow-list the fakepipe import is a violation too: fakepipe's own rule.
	if bad, _, _, _ := fakeImportViolations(root); len(bad) != 2 {
		t.Errorf("with no allow-list got %d violations (%q), want 2", len(bad), bad)
	}
}
