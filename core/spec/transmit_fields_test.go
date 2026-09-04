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

// TestTransmitFields_MatchTheDeclaredMarker proves that transmitFields
// holds EXACTLY the Fields whose own declaration says they belong to the
// transmitter, by reading this package's source rather than by restating
// the list.
//
// A test that simply compared transmitFields against a second literal
// would pin nothing: both copies would be written by the same hand in
// the same change, and a third transmit-only Field added later would be
// missing from both. So the SOURCE is the evidence. The marker sits in
// the Field constant's doc comment, where the person adding a Field is
// already writing prose about what it means, and this test fails the
// moment a marked constant is absent from transmitFields — which is the
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
	for _, f := range transmitFields {
		declared = append(declared, string(f))
	}
	sort.Strings(marked)
	sort.Strings(declared)
	if strings.Join(marked, ",") != strings.Join(declared, ",") {
		t.Errorf("transmitFields = %v, but field.go marks %v as %s — every marked Field must appear in transmitFields, and nothing else may",
			declared, marked, transmitOnlyMarker)
	}
}

// declaredFieldConstants returns the value of every `Field` constant this
// package declares, read from the source for the same reason the marker test
// reads it: a hand-written list here would be one more copy to forget. This
// remains source-derived evidence independent of AllFields, whose own
// declaration audit uses the same package-wide source boundary.
func declaredFieldConstants(t *testing.T) []Field {
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
	var out []Field
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, s := range gen.Specs {
				vs, ok := s.(*ast.ValueSpec)
				if !ok {
					continue
				}
				if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != "Field" {
					continue
				}
				for _, v := range vs.Values {
					lit, ok := v.(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					value, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatalf("%v: unquoting %s: %v", fset.Position(lit.Pos()), lit.Value, err)
					}
					out = append(out, Field(value))
				}
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("no Field constants found, so a test walking them proves nothing")
	}
	return out
}

// TestIsTransmitField_AnswersExactlyTheDeclaredSet pins the predicate against
// the declaration it is derived from, in both directions: every Field in the
// set is transmit-only, and no other Field this package declares is. It walks
// every declared Field constant rather than naming a few receiver Fields, so a
// Field added later is covered without a test edit.
func TestIsTransmitField_AnswersExactlyTheDeclaredSet(t *testing.T) {
	transmit := map[Field]bool{}
	for _, f := range transmitFields {
		transmit[f] = true
		if !IsTransmitField(f) {
			t.Errorf("IsTransmitField(%s) = false for a Field in the declared set", f)
		}
	}
	declared := declaredFieldConstants(t)
	if len(declared) < len(transmitFields) {
		t.Fatalf("found %d Field constants, fewer than the %d in transmitFields", len(declared), len(transmitFields))
	}
	for _, f := range declared {
		if got := IsTransmitField(f); got != transmit[f] {
			t.Errorf("IsTransmitField(%s) = %v, want %v", f, got, transmit[f])
		}
	}
	if IsTransmitField("no-such-field") {
		t.Error("IsTransmitField reports an unknown Field as transmit-only")
	}
}

// TestSpecExportsNoPackageLevelVar is the closing review's fix, and the point
// is narrower than the name: this package's exported surface must offer no way
// to MUTATE its validation policy.
//
// TransmitFields was an exported slice, read by core/spec's own bank loop and
// by core/codeplug's unreachable-field wording. An exported slice is an
// exported mutable global — `spec.TransmitFields[0] = "nothing"` from anywhere
// in the process would have disabled the receive-only protection in both
// packages at once, silently, for every radio, and no test in either package
// would have noticed. The same is true of any exported map or slice var this
// package might grow later, so the pin is written against the SHAPE rather
// than against the one name that had it.
//
// Every other exported declaration here is a const, a type or a func, and a
// caller that needs an answer about policy asks a func for it: IsTransmitField
// is what replaced the slice. It reads the source for the same reason
// TestTransmitFields_MatchTheDeclaredMarker does — a test that restated the
// list of exported names would be written by the same hand as the export it
// was meant to catch.
func TestSpecExportsNoPackageLevelVar(t *testing.T) {
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
	var files int
	for _, file := range pkg.Files {
		files++
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, s := range gen.Specs {
				vs, ok := s.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					if !name.IsExported() {
						continue
					}
					t.Errorf("%v: core/spec exports the package-level var %s. A caller can assign to it, "+
						"or to an element of it, and change this package's validation policy for the whole "+
						"process — which is what the exported TransmitFields slice allowed. Keep the "+
						"declaration private and export a func that answers the question instead "+
						"(IsTransmitField is the precedent).", fset.Position(name.Pos()), name.Name)
				}
			}
		}
	}
	if files == 0 {
		t.Fatal("parsed no non-test files, so this test proved nothing")
	}
}

// TestValidate_ReceiveOnlyRefusesEveryTransmitField walks transmitFields
// itself, so a Field added to that set is refused on a ReceiveOnly radio
// from the moment it is added, with no test edit needed.
func TestValidate_ReceiveOnlyRefusesEveryTransmitField(t *testing.T) {
	for _, field := range transmitFields {
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
