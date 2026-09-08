// SPDX-License-Identifier: GPL-3.0-or-later

package guards

import (
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
)

// THE CONSTRUCTION GUARD OVER core/kw/ma's FRAMINGS, and it is the one
// TestKenwoodDriversUseNewFramingFor (kw_framing_test.go) CANNOT EXPRESS.
//
// That guard is an AST walk over non-test files under core/driver, so it can
// see a driver reaching for kw.NewFraming or kw.NewFramingWithGate and it can
// see nothing at all inside core/kw/ma. If ma.NewFramingFor itself called
// kw.NewFraming — the envelope-only constructor — or handed
// kw.NewFramingWithGate the wrong predicate, every driver in the tree would
// still be calling the right-looking name and that guard would stay green
// while the outbound gate had silently fallen back to the shape both books
// print. THIS test takes the framing BOTH ma layouts ACTUALLY RETURN and
// drives frames through its Allow — the predicate the engine really holds
// (core/transport/engine.go:474 stores f.Allow at construction and :724 calls
// it before every write, both VERIFIED) — rather than reading source.
//
// IT DOES NOT SUBSTITUTE FOR core/kw/ma's OWN ROSTER TEST, AND THAT TEST DOES
// NOT SUBSTITUTE FOR THIS ONE. core/kw/ma's
// TestAllowedCommand_RefusesTheWholeNegativeRoster proves what the METHOD
// refuses; this proves that the method is what the engine will ask. A perfect
// roster wired to the wrong constructor passes the first and fails this; a
// miswired roster behind the right constructor passes neither. Both comments
// say so, in both files.
//
// IT DRIVES A POSITIVE CORPUS AS WELL AS THE NEGATIVE ROSTER, and without both
// halves it does not discriminate at all: a guard driving only refusals is
// satisfied by an ALWAYS-FALSE predicate, so a framing that refused every
// valid MA0 read, every EX read and the whole probe sequence would pass it
// green and the failure would first appear against a physical radio. That is
// the review finding this file exists to answer (C7). Each corpus is COUNTED
// and its count asserted non-empty, so an emptied table fails loudly rather
// than passing vacuously.
//
// THE NEGATIVE ROSTER HERE IS AN INDEPENDENT TRANSCRIPTION of the same printed
// set core/kw/ma's own roster carries, deliberately, and not a shared fixture:
// a guard that consumed its subject's own table would go green with it the day
// that table lost a row. The two are checked against each other by nothing but
// the books.

// maPositiveCorpus is every frame ONE ma layout's builders produce — the seven
// grammars of spec decision 5 — reached entirely through the package's
// EXPORTED surface, which is the surface a driver has.
//
// The EX address is taken from THAT ROW'S OWN inventory, because an EX read is
// per-radio: an address one chart prints and the other does not must be
// admitted here and refused by the sibling layout (core/kw/ma's tier 2).
func maPositiveCorpus(t *testing.T, l ma.Layout) map[string][]byte {
	t.Helper()
	must := func(cmd ma.Command, err error) []byte {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", l.Model(), err)
		}
		return cmd.Bytes()
	}
	slot, err := l.NewSlot(7)
	if err != nil {
		t.Fatalf("%s: NewSlot(7): %v", l.Model(), err)
	}
	addr, ok := maRowOnlyEXAddress(l)
	if !ok {
		t.Fatalf("%s: no address in its inventory is absent from the other row's, so the EX arm of this corpus would not be row-specific", l.Model())
	}
	return map[string][]byte{
		"ID read":    must(l.BuildIDRead()),
		"AI read":    must(l.BuildAIRead()),
		"AI set off": must(l.BuildAISetOff()),
		"FV read":    must(l.BuildFVRead()),
		"EX read of an address this row's chart prints": must(l.BuildEXRead(addr)),
		"MA0 read": must(l.BuildMA0Read(slot)),
		"MA0 set":  must(l.BuildMA0Set(ma.Record{Slot: slot, FreqHz: 14_250_000, Mode: '2', ToneType: '0', Name: "GB3IV"})),
	}
}

// maRowOnlyEXAddress returns an address in l's inventory that the OTHER row's
// chart does not print, and whether one exists.
//
// It is computed from the two inventories rather than transcribed: a literal
// address here would be a third copy of a generated artefact, and it would rot
// the first time either chart was re-transcribed.
func maRowOnlyEXAddress(l ma.Layout) (kw.EXAddress, bool) {
	other := ma.Layout890()
	if l.Book() == kw.Book890 {
		other = ma.Layout990()
	}
	for _, it := range l.EXItems() {
		if _, ok := other.EXItem(it.Addr); !ok {
			return it.Addr, true
		}
	}
	return kw.EXAddress{}, false
}

// maNegativeRoster is every frame the TS-890S and TS-990S books print that
// this programme never sends: MA1-MA7, MI, MN, MV, QA, QD and QI (spec
// decision 15), plus MR, MW, MC and TY, which neither of those two books
// prints at all and which belong to the TS-590/TS-480 pair.
//
// Both books' spellings are driven against both rows, because the 990S gives
// MI, MN and MV a Main/Sub P1 that the 890S's forms have no cell for.
func maNegativeRoster() []struct{ what, frame string } {
	return []struct{ what, frame string }{
		{"MR read, another pair's book", "MR0007;"},
		{"MW set, another pair's book", "MW0007" + strings.Repeat("0", 43) + ";"},
		{"MC read, another pair's book", "MC;"},
		{"MC set, another pair's book", "MC007;"},
		{"TY read, another pair's book", "TY;"},

		{"890S MA1 Set", "MA1" + "00014250000" + "2" + "0" + ";"},
		{"890S MA2 Set", "MA2007 GB3IV;"},
		{"890S MA3 Set", "MA30071;"},
		{"890S MA4 Set", "MA4007008;"},
		{"890S MA5 Set, the printed erase", "MA5007;"},
		{"890S MA6 Set", "MA610000014250000;"},
		{"890S MA7 Set", "MA700014250000;"},
		{"890S MA7 Read", "MA70;"},
		{"890S MI Set", "MI007;"},
		{"890S MN Set", "MN007;"},
		{"890S MN Read", "MN;"},
		{"890S MV Set", "MV1;"},
		{"890S MV Read", "MV;"},
		{"890S QA Read", "QA0;"},
		{"890S QD Set", "QD;"},
		{"890S QI Set", "QI;"},

		{"990S MA1 Set", "MA1" + "00014250000" + "2" + "0" + ";"},
		{"990S MA2 Set", "MA2007 GB3       ;"},
		{"990S MA3 Set", "MA30071;"},
		{"990S MA4 Set", "MA4007008;"},
		{"990S MA5 Set, the printed erase", "MA5007;"},
		{"990S MA6 Set", "MA610000014250000;"},
		{"990S MI Set", "MI0007;"},
		{"990S MN Set", "MN0007;"},
		{"990S MN Read", "MN0;"},
		{"990S MV Set", "MV01;"},
		{"990S MV Read", "MV0;"},
		{"990S QA Read", "QA0;"},
		{"990S QD Set", "QD;"},
		{"990S QI Set", "QI;"},
	}
}

// maConstructorAdvice is the sentence every failure below ends with: the two
// constructors a driver MAY use, and what the other two do instead.
const maConstructorAdvice = "the two constructors a driver MAY use are kw.NewFramingFor(layout) and ma.NewFramingFor(layout), whose outbound gates are their family's per-row grammars; kw.NewFraming(book) gates on the ENVELOPE both books print and kw.NewFramingWithGate(book, allow) gates on whatever predicate its caller passed"

// TestMAFramingGatesTheRoster is the construction guard. See this file's
// header for what it proves and what it deliberately does not.
func TestMAFramingGatesTheRoster(t *testing.T) {
	roster := maNegativeRoster()
	if len(roster) == 0 {
		t.Fatal("the negative roster is empty, so every refusal below would pass vacuously")
	}

	admitted, refused := 0, 0
	for _, l := range []ma.Layout{ma.Layout890(), ma.Layout990()} {
		f, err := ma.NewFramingFor(l)
		if err != nil {
			t.Fatalf("%s: ma.NewFramingFor: %v", l.Model(), err)
		}

		// THE POSITIVE HALF. Without it an always-false predicate — a
		// framing that refuses everything, which is exactly what a
		// miswiring produces — satisfies the whole test.
		for what, frame := range maPositiveCorpus(t, l) {
			admitted++
			if !f.Allow(frame) {
				t.Errorf("%s: the framing ma.NewFramingFor returned REFUSED %q, this row's own %s — a gate that refuses what its own codec builds cannot read or write the radio at all; %s", l.Model(), frame, what, maConstructorAdvice)
			}
		}

		// THE NEGATIVE HALF. Each of these is a frame a physical radio
		// would act on.
		for _, tc := range roster {
			refused++
			if f.Allow([]byte(tc.frame)) {
				t.Errorf("%s: the framing ma.NewFramingFor returned ADMITTED %s (%q), which nothing in core/kw/ma builds — its gate has fallen back to the envelope or to a caller's predicate; %s", l.Model(), tc.what, tc.frame, maConstructorAdvice)
			}
		}
	}

	if admitted == 0 || refused == 0 {
		t.Fatalf("the guard drove %d admitted and %d refused frames — both corpora must be non-empty, or a miswired framing passes on an emptied table", admitted, refused)
	}
}

// TestMAFramingRefusesAnUnconfiguredLayout keeps the constructor's own door
// inside the guard's scope: a framing built for a zero ma.Layout would gate
// for no radio, and the engine cannot tell such a predicate from a real one
// because a zero Layout's AllowedCommand is a perfectly non-nil method value.
func TestMAFramingRefusesAnUnconfiguredLayout(t *testing.T) {
	f, err := ma.NewFramingFor(ma.Layout{})
	if err == nil {
		t.Fatalf("ma.NewFramingFor accepted an unconfigured layout and returned %v", f)
	}
	if f != nil {
		t.Errorf("ma.NewFramingFor returned a non-nil framing alongside its error: %v", f)
	}
	if !strings.Contains(err.Error(), "layout") {
		t.Errorf("the refusal reads %v, and it should name the layout", err)
	}
	// It is the family's own typed refusal, so ONE errors.Is arm in a driver
	// covers both constructors.
	if !errors.Is(err, kw.ErrLayoutInvalid) {
		t.Errorf("the refusal %v does not wrap kw.ErrLayoutInvalid", err)
	}
}
