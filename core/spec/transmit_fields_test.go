// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// mapKeys returns the keys of an *ast.Package map, for an error message
// only — never for ordering-sensitive logic.
func mapKeys(pkgs map[string]*ast.Package) []string {
	var keys []string
	for k := range pkgs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestTransmitFields_MatchTheDeclaredMarker proves that TransmitFields
// holds EXACTLY the Fields whose own declaration says they belong to the
// transmitter, by reading this package's source rather than by restating
// the list.
//
// A test that simply compared TransmitFields against a second literal
// would pin nothing: both copies would be written by the same hand in
// the same change, and a third transmit-only Field added later would be
// missing from both. So the SOURCE is the evidence. The marker sits in
// the Field constant's doc comment, where the person adding a Field is
// already writing prose about what it means, and this test fails the
// moment a marked constant is absent from TransmitFields — which is the
// whole point of deriving the ReceiveOnly check from one declaration
// (core/spec/validate.go's bank loop) instead of a two-item literal.
//
// The precedent for reading source as evidence in a test is
// core/cat/evidence_literals_test.go.
//
// IT PARSES THE WHOLE PACKAGE DIRECTORY, not just field.go (registration
// review, deferred minor): today every Field constant happens to live in
// field.go, but a transmit-only Field declared in some future
// core/spec/*.go file would carry the marker and be silently missed if
// this test read one named file, and the len(marked)==0 guard below would
// not fire because field.go's own two markers still parse. Parsing the
// directory removes that trap without needing anyone to remember it.
func TestTransmitFields_MatchTheDeclaredMarker(t *testing.T) {
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

	var marked []string
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, s := range gen.Specs {
				vs, ok := s.(*ast.ValueSpec)
				if !ok || vs.Doc == nil {
					continue
				}
				if !strings.Contains(vs.Doc.Text(), transmitOnlyMarker) {
					continue
				}
				if len(vs.Values) != 1 {
					t.Fatalf("%v: a %s constant must have exactly one value", fset.Position(vs.Pos()), transmitOnlyMarker)
				}
				lit, ok := vs.Values[0].(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Fatalf("%v: a %s constant must be declared as a string literal", fset.Position(vs.Pos()), transmitOnlyMarker)
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("%v: unquoting %s: %v", fset.Position(vs.Pos()), lit.Value, err)
				}
				marked = append(marked, value)
			}
		}
	}
	if len(marked) == 0 {
		t.Fatalf("this package declares no %q constant at all — the marker convention has been lost", transmitOnlyMarker)
	}

	var declared []string
	for _, f := range TransmitFields {
		declared = append(declared, string(f))
	}
	sort.Strings(marked)
	sort.Strings(declared)
	if strings.Join(marked, ",") != strings.Join(declared, ",") {
		t.Errorf("TransmitFields = %v, but field.go marks %v as %s — every marked Field must appear in TransmitFields, and nothing else may",
			declared, marked, transmitOnlyMarker)
	}
}

// TestValidate_ReceiveOnlyRefusesEveryTransmitField walks TransmitFields
// itself, so a Field added to that set is refused on a ReceiveOnly radio
// from the moment it is added, with no test edit needed.
func TestValidate_ReceiveOnlyRefusesEveryTransmitField(t *testing.T) {
	for _, field := range TransmitFields {
		t.Run(string(field), func(t *testing.T) {
			c := validTestCapabilities()
			c.Transmit = ReceiveOnly
			c.Banks[0].Fields[field] = FieldSupport{Read: Supported}
			err := c.Validate()
			if err == nil || !strings.Contains(err.Error(), string(field)) || !strings.Contains(err.Error(), "ReceiveOnly") {
				t.Fatalf("Validate() = %v, want a ReceiveOnly refusal naming %s", err, field)
			}
		})
	}
}
