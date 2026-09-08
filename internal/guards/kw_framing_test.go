// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// kwImportPath is core/kw, the Kenwood codec package.
const kwImportPath = modulePrefix + "core/kw"

// TestKenwoodDriversUseNewFramingFor pins the steering core/kw's prose asks
// for, mechanically: NO NON-TEST FILE UNDER core/driver MAY CALL
// kw.NewFraming OR kw.NewFramingWithGate — a driver builds its session's
// framing with kw.NewFramingFor(layout), or, for the MA family, with
// ma.NewFramingFor(layout).
//
// THE CONSTRUCTORS DIFFER IN THEIR OUTBOUND GATE, WHICH IS THE WHOLE
// POINT. kw.NewFraming(book) knows which document a session speaks, and so
// which cause sentence an "O;" carries (erratum E13), but it does not know
// which RADIO — the layout axes are per row — so its Allow can only be the
// envelope both books print: a terminator, exactly one, as the last byte;
// two upper-case name bytes; printable interior; no radio-to-host token; no
// frame past DefaultMaxFrame. That admits, among other things, the 42-byte
// erase shape of 590:1579-1581, a frame the book really prints and this
// programme really never builds. kw.NewFramingFor(layout) puts the eight
// per-row grammars in front of that envelope (core/kw/allowlist.go).
//
// A DRIVER REACHING FOR THE WEAKER ONE IS ONE IDENTIFIER'S DIFFERENCE, and
// NewFraming is the shorter name and the one a reader meets first in
// framing.go. Nothing about the resulting value would look wrong: it is a
// perfectly usable transport.Framing whose gate has silently fallen back to
// the envelope. core/kw says so in prose at three sites and the T11 brief
// repeats it; this is where the repository normally puts such a rule, and it
// lands before the first Kenwood driver rather than after it.
//
// kw.NewFramingWithGate(book, allow) is the SECOND back door, added to this
// guard at Kenwood pair 2 Stage 0 (spec decision 4). It is the gate hook
// core/kw/ma builds its own outbound gate through, and from a driver's seat
// it is worse than NewFraming, not better: the caller supplies the predicate,
// so a driver passing a permissive one gets the envelope back with a
// narrower-looking name. A guard that forbids one back door and not the one
// added beside it is worse than no guard, because it reads as coverage.
//
// SCOPE IS core/driver AND EVERYTHING BENEATH IT (inTree, not exact
// equality): the neutral seam package does not import core/kw at all, and it
// is the per-radio subpackages — core/driver/ts590, core/driver/ts480 — that
// will build sessions. NewFramingFor is deliberately NOT matched: the
// selector names are compared exactly, against the exact list
// isKWEnvelopeConstructor holds.
//
// TESTS ARE OUT OF SCOPE by parseRepo's own filter, and that is right here:
// a driver test may legitimately build an envelope-only framing to
// demonstrate what the narrower gate adds, which is the shape core/kw's own
// framing_test.go already has.
func TestKenwoodDriversUseNewFramingFor(t *testing.T) {
	const driverTree = "core/driver"

	files := parseRepo(t)

	seen := 0
	for _, pf := range files {
		if !inTree(pf.relDir, driverTree) {
			continue
		}
		seen++
		if callsKWNewFraming(pf.file) {
			t.Errorf("%s: calls kw.NewFraming or kw.NewFramingWithGate — the two constructors a driver MAY use are kw.NewFramingFor(layout) and ma.NewFramingFor(layout), whose outbound gates are their family's per-row grammars; the envelope-only pair gate on the shape both books print, which admits the 42-byte erase shape of 590:1579-1581 and every other row's frames, and NewFramingWithGate hands the predicate to the caller", pf.relPath)
		}
	}

	if seen == 0 {
		t.Fatalf("parseRepo found zero non-test Go files under %s — the walk or its filters are broken and the check above passed vacuously", driverTree)
	}
}

// isKWEnvelopeConstructor reports whether name is one of core/kw's
// ENVELOPE-ONLY framing constructors — the ones whose gate is the shape both
// books print rather than a radio's own per-row grammars.
//
// THE LIST IS EXACT AND A NEW ONE MUST BE ADDED HERE BY NAME. NewFramingFor
// is deliberately absent: it is the constructor a driver SHOULD reach for.
//
// NewFramingWithGate joined at Kenwood pair 2 Stage 0 (spec decision 4,
// 08/09/2026), and it landed BEFORE core/kw gained the constructor — a name
// list widened for something that is not there yet is inert, and widening it
// afterwards is the edit everyone forgets. It is the gate hook core/kw/ma
// builds its outbound gate through, so from core/driver it is a second weaker
// constructor one identifier's difference from the right one.
func isKWEnvelopeConstructor(name string) bool {
	return name == "NewFraming" || name == "NewFramingWithGate"
}

// callsKWNewFraming reports whether f calls one of those constructors through
// its core/kw import, under whatever local name that import carries.
//
// IT RESOLVES THE IMPORT RATHER THAN MATCHING THE BARE NAME, because
// core/civ has a NewFraming of its own and the Icom drivers call it
// legitimately (core/driver/ic7300 and its siblings). A name-only match
// would fail every one of them, which is the shape of guard that gets
// deleted rather than fixed.
func callsKWNewFraming(f *ast.File) bool {
	local, ok := importsPath(f, kwImportPath)
	if !ok {
		return false
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if found {
			return false
		}
		switch x := n.(type) {
		case *ast.SelectorExpr:
			// kw.NewFraming or kw.NewFramingWithGate, under the import's
			// own local name.
			id, isIdent := x.X.(*ast.Ident)
			if isIdent && id.Name == local && isKWEnvelopeConstructor(x.Sel.Name) {
				found = true
			}
		case *ast.Ident:
			// A dot-imported core/kw makes the call a bare identifier.
			if local == "." && isKWEnvelopeConstructor(x.Name) {
				found = true
			}
		}
		return true
	})
	return found
}

// TestCallsKWNewFramingDetector is the guard's own red proof, and it is
// permanent rather than a one-off: the guard above walks a tree that
// contains no Kenwood driver yet, so without this its "no file calls it"
// result would be indistinguishable from a predicate that never says yes.
//
// The five sources are the five ways this has to come out: the plain call,
// the aliased call, the gate-hook constructor added at Kenwood pair 2 Stage 0
// (spec decision 4), the constructor a driver SHOULD use, and core/civ's
// same-named constructor that the Icom drivers really do call.
func TestCallsKWNewFramingDetector(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "the plain call",
			src: `package ts590
import "` + kwImportPath + `"
func open() { _, _ = kw.NewFraming(kw.Book590) }`,
			want: true,
		},
		{
			name: "the call under an alias",
			src: `package ts590
import kenwood "` + kwImportPath + `"
func open() { _, _ = kenwood.NewFraming(kenwood.Book590) }`,
			want: true,
		},
		{
			name: "the constructor a driver should use",
			src: `package ts590
import "` + kwImportPath + `"
func open(l kw.Layout) { _, _ = kw.NewFramingFor(l) }`,
			want: false,
		},
		{
			name: "the gate-hook constructor, the second back door",
			src: `package ts890
import "` + kwImportPath + `"
func open(allow func([]byte) bool) { _, _ = kw.NewFramingWithGate(kw.Book890, allow) }`,
			want: true,
		},
		{
			name: "core/civ's own NewFraming, which the Icom drivers call",
			src: `package ic7300
import "` + modulePrefix + `core/civ"
func open(p civ.Profile) { _, _ = civ.NewFraming(p) }`,
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := parser.ParseFile(token.NewFileSet(), "probe.go", tc.src, 0)
			if err != nil {
				t.Fatalf("parsing the probe source: %v", err)
			}
			if got := callsKWNewFraming(f); got != tc.want {
				t.Errorf("callsKWNewFraming = %v, want %v for:\n%s", got, tc.want, tc.src)
			}
		})
	}
}
