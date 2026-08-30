package fakeic7851

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestNoProjectImports(t *testing.T) {
	files, imports := 0, 0
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		files++
		for _, spec := range f.Imports {
			imports++
			p, _ := strconv.Unquote(spec.Path.Value)
			if strings.HasPrefix(p, "github.com/gm5dna/open-rig-programmer/") {
				t.Errorf("%s imports project package %q", path, p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 || imports == 0 {
		t.Fatalf("guard scanned files=%d imports=%d", files, imports)
	}
}
