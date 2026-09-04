// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"go/ast"
	"go/constant"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
)

// declaredFieldsInSourceOrder reads every non-test file because a Field
// declared outside field.go must be covered too. parser.ParseDir returns the
// files in a map, so sorting their names here is what makes the evidence order
// match AllFields' documented file-name-then-source-position order.
func declaredFieldsInSourceOrder(t *testing.T) []Field {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing package directory: %v", err)
	}
	pkg, ok := pkgs["spec"]
	if !ok {
		t.Fatalf("package directory has no %q package — found %v", "spec", mapKeys(pkgs))
	}

	fileNames := make([]string, 0, len(pkg.Files))
	for name := range pkg.Files {
		fileNames = append(fileNames, name)
	}
	sort.Strings(fileNames)
	files := make([]*ast.File, 0, len(fileNames))
	for _, name := range fileNames {
		files = append(files, pkg.Files[name])
	}

	info := &types.Info{Defs: map[*ast.Ident]types.Object{}}
	checked, err := (&types.Config{Importer: importer.Default()}).Check(
		"github.com/gm5dna/open-rig-programmer/core/spec", fset, files, info)
	if err != nil {
		t.Fatalf("type-checking package source: %v", err)
	}
	fieldType := checked.Scope().Lookup("Field").Type()

	var fields []Field
	for _, file := range files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range valueSpec.Names {
					declared, ok := info.Defs[name].(*types.Const)
					if !ok || !types.Identical(declared.Type(), fieldType) {
						continue
					}
					if declared.Val().Kind() != constant.String {
						t.Fatalf("%v: Field constant %s is not a string", fset.Position(name.Pos()), name.Name)
					}
					fields = append(fields, Field(constant.StringVal(declared.Val())))
				}
			}
		}
	}
	if len(fields) == 0 {
		t.Fatal("parsed no Field constants, so the AllFields audit would pass vacuously")
	}
	return fields
}

func TestAllFieldsMatchesDeclarationsInDeterministicOrder(t *testing.T) {
	declared := declaredFieldsInSourceOrder(t)
	if len(declared) != 27 {
		t.Fatalf("core/spec declares %d Field constants, want 27 — when minting a Field, update this count and AllFields in the same change", len(declared))
	}
	if got := AllFields(); !slices.Equal(got, declared) {
		t.Errorf("AllFields() = %v, want Field declarations in file-name then source-position order: %v", got, declared)
	}
}

func TestAllFieldsReturnsAFreshSlice(t *testing.T) {
	first := AllFields()
	if len(first) == 0 {
		t.Fatal("AllFields() is empty")
	}
	first[0] = "caller mutation"
	if got := AllFields()[0]; got == first[0] {
		t.Errorf("a caller mutation changed the next AllFields() result to %q", got)
	}
}
