package fakeic7760

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

const modulePrefix = "github.com/gm5dna/open-rig-programmer/"

func isForbiddenImport(path string) bool { return strings.HasPrefix(path, modulePrefix) }

type importViolation struct{ file, path string }

func scanImports(root string) ([]importViolation, int, error) {
	var violations []importViolation
	files := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		files++
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return err
			}
			if isForbiddenImport(p) {
				violations = append(violations, importViolation{path, p})
			}
		}
		return nil
	})
	return violations, files, err
}

func TestNoCoreImports(t *testing.T) {
	violations, files, err := scanImports(".")
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 {
		t.Fatal("recursive import scan examined no production Go files")
	}
	for _, v := range violations {
		t.Errorf("%s imports forbidden project package %q", v.file, v.path)
	}
}

func TestScanImportsIsRecursive(t *testing.T) {
	root := t.TempDir()
	if err := osWrite(filepath.Join(root, "nested", "bad.go"), "package nested\nimport _ \"github.com/gm5dna/open-rig-programmer/core/civ\"\n"); err != nil {
		t.Fatal(err)
	}
	violations, _, err := scanImports(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 {
		t.Fatalf("found %d violations, want 1", len(violations))
	}
}

func osWrite(path, contents string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(contents), 0o644)
}
