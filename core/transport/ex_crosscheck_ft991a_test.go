// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/ft991a"
	"github.com/gm5dna/open-rig-programmer/internal/fakeft991a"
)

// This file is the FT-991A's half of the two-independent-transcriptions
// cross-check — the shape of ex_crosscheck_ft891_test.go beside it, on a radio
// whose EX address is a SINGLE component:
//
//	the DIALECT's inventory (ft991a.Dialect().EXItems()) is generated from
//	TRANSCRIPTION A (core/cat/ft991a/table2.csv) by internal/extable;
//
//	the FAKE's inventory (fakeft991a.EXDefaults()) is generated from
//	TRANSCRIPTION B (internal/fakeft991a/transcription-b.csv, its own copy) by
//	internal/fakeft991a/gen, which imports nothing project-internal at all.
//
// A and B are two independent derivations of one printed chart (manual rev
// 1711-D, the MENU chart on folios 7-9): A layout-text-led and PDF-checked, B
// derived PDF-primary by a quarantined agent with no repository access, no
// sight of A and no row count. The two GENERATORS are independent too — that is
// what the fake's recursive no-imports fence enforces, gen/ included. So a
// mis-read row in either transcription, or a defect in either generator,
// surfaces HERE as a mismatch rather than as two tables quietly agreeing on the
// same wrong number.
//
// # What is DIFFERENT here, and it is not only the address width
//
// THE TWO SIDES OMIT THE SAME ROW FOR THE SAME REASON, HAVING READ TWO
// DIFFERENT SPELLINGS OF IT. 087 RADIO ID prints a single hyphen for its Digits
// and ten spaced hyphens for its parameter, so it names no field an EX frame
// could read or write. Transcription A keeps the raw '-' and internal/extable's
// ParameterlessExcluded omits the address; transcription B writes '?' — its
// brief's token for a non-integer cell — and internal/fakeft991a/gen's
// parameterlessAddrs omits the address. ONE GLYPH ON THE PAGE, two transcription
// conventions, two generators, one absence (this milestone's plan decision P18).
// So BOTH inventories are 152 for a 153-row chart, and
// TestEXInventoryCrossCheck_FT991ARow087IsAbsentFromBothSides asserts the
// absence on each side by name rather than leaving it to be inferred from the
// set comparison passing.
//
// The width half matters here as it does on the FT-891, and for the same
// asymmetric reason: this chart has no text row and B's three-column schema
// carries no cell from which one could be identified
// (internal/fakeft991a/gen/main.go's widthToken refuses rather than guessing),
// so THE DIALECT'S SIDE is where the text claim is checked. If A ever declares
// one, TestEXInventoryCrossCheck_FT991AWidthsAndShapesAgree fails on that item
// rather than silently accepting eight zeros where twelve spaces belong.
//
// It lives in core/transport for the reason its siblings do: this is the
// existing test home that already imports both core/cat and a fake, which keeps
// the fakes' own test packages core-free. Nothing in this file is imported by
// production code.
//
// ON FAILURE: report the exact diff, and do NOT "fix" either table to make it
// pass. Which side is wrong — or whether the printed chart is — is an
// arbitration against the PDF. An edit that merely restores agreement destroys
// the evidence the agreement was worth.
//
// ONE CLASS NO LEG HERE CAN CATCH, stated so that the silence is a recorded
// limit: a defect PRINTED in the chart is read faithfully by both derivations
// and both tables agree on it. This radio's known printing defects are recorded
// by core/cat/ft991a/testdata/ledger.md and transcription-b.md §4, not by
// anything below.

// Each test below fetches both inventories itself rather than sharing a
// package-level fixture: both APIs return fresh copies by contract
// (Dialect().EXItems() copies its slice, EXDefaults() its map), and a shared
// fixture would hide it if either stopped.

// TestEXInventoryCrossCheck_FT991AAddressSetsIdentical compares the dialect's
// EX address set against the fake's, reporting BOTH diff directions so a report
// can quote the exact addresses without re-deriving the diff.
func TestEXInventoryCrossCheck_FT991AAddressSetsIdentical(t *testing.T) {
	dialectAddrs := make(map[string]bool)
	for _, item := range ft991a.Dialect().EXItems() {
		dialectAddrs[ft991a.Dialect().EXWire(item.Addr)] = true
	}
	fakeAddrs := fakeft991a.EXDefaults()

	// Vacuity guard: two empty sets are trivially equal, and an inventory that
	// failed to generate would be exactly that.
	if len(dialectAddrs) == 0 || len(fakeAddrs) == 0 {
		t.Fatalf("empty inventory: dialect has %d addresses, fake has %d — one side failed to generate, and the comparison below would pass vacuously", len(dialectAddrs), len(fakeAddrs))
	}

	// The address is a SINGLE component on this radio, and the two sides have
	// to agree about that before they can meaningfully agree about membership:
	// a four- or six-digit render on either side would make every address miss,
	// which is a true failure but an unhelpfully phrased one.
	if got := ft991a.Dialect().EXAddressWidth(); got != 3 {
		t.Fatalf("ft991a.Dialect().EXAddressWidth() = %d, want 3 — this radio's EX address is the chart's whole three-digit MENU Number (core/cat's EXAddressSingle)", got)
	}

	// 152 for a 153-row chart, on both sides. Stated as a literal because the
	// arithmetic is the point: the set comparison below would be satisfied by
	// two inventories that had BOTH lost the same trailing stretch.
	const want = 152
	if len(dialectAddrs) != want || len(fakeAddrs) != want {
		t.Errorf("inventory sizes are dialect %d, fake %d, want %d each — 153 chart rows less the one row (087) that prints no parameter", len(dialectAddrs), len(fakeAddrs), want)
	}

	var inDialectNotFake, inFakeNotDialect []string
	for addr := range dialectAddrs {
		if _, ok := fakeAddrs[addr]; !ok {
			inDialectNotFake = append(inDialectNotFake, addr)
		}
	}
	for addr := range fakeAddrs {
		if !dialectAddrs[addr] {
			inFakeNotDialect = append(inFakeNotDialect, addr)
		}
	}
	sort.Strings(inDialectNotFake)
	sort.Strings(inFakeNotDialect)

	if len(inDialectNotFake) > 0 || len(inFakeNotDialect) > 0 {
		t.Errorf("FT-991A EX address inventories disagree between the two independent MENU chart transcriptions (dialect/A has %d addresses, fake/B has %d):\n"+
			"  in ft991a.Dialect().EXItems() but NOT in fakeft991a.EXDefaults() (%d): %v\n"+
			"  in fakeft991a.EXDefaults() but NOT in ft991a.Dialect().EXItems() (%d): %v\n"+
			"This is a genuine cross-check finding, not a bug in this test: do not modify either table to make it pass — report the diff and arbitrate against the PDF.",
			len(dialectAddrs), len(fakeAddrs), len(inDialectNotFake), inDialectNotFake, len(inFakeNotDialect), inFakeNotDialect)
	}
}

// TestEXInventoryCrossCheck_FT991ARow087IsAbsentFromBothSides states the ONE
// designed absence by name, on each side, from each side's own API — which the
// set comparison above cannot do, because two inventories that both wrongly
// CARRIED 087 would agree perfectly.
//
// The absence is reached independently: the dialect's from transcription A's
// raw '-' through internal/extable's ParameterlessExcluded, the fake's from
// transcription B's '?' through internal/fakeft991a/gen's parameterlessAddrs
// (plan decision P18 — one printed hyphen, two transcription conventions). Its
// neighbours are asserted present on both sides, so that an exclusion which
// took more than the row it declares fails here rather than shrinking both
// inventories in step.
func TestEXInventoryCrossCheck_FT991ARow087IsAbsentFromBothSides(t *testing.T) {
	dialect := ft991a.Dialect()
	fakeAddrs := fakeft991a.EXDefaults()

	const excluded = 87
	addr := cat.EXAddress{P1: excluded}
	if dialect.KnownEXAddress(addr) {
		t.Errorf("the dialect claims %s is a known EX address — 087 RADIO ID prints no parameter at all, so it names no field an EX frame could read or write", dialect.EXWire(addr))
	}
	if _, ok := fakeAddrs[dialect.EXWire(addr)]; ok {
		t.Errorf("fakeft991a.EXDefaults() carries %s", dialect.EXWire(addr))
	}
	for _, neighbour := range []uint16{86, 88} {
		a := cat.EXAddress{P1: neighbour}
		if !dialect.KnownEXAddress(a) {
			t.Errorf("the dialect does not know %s — the exclusion has taken more than the one row it declares", dialect.EXWire(a))
		}
		if _, ok := fakeAddrs[dialect.EXWire(a)]; !ok {
			t.Errorf("fakeft991a.EXDefaults() does not carry %s — the exclusion has taken more than the one row it declares", dialect.EXWire(a))
		}
	}
}

// TestEXInventoryCrossCheck_FT991AWidthsAndShapesAgree checks, for every
// address in BOTH inventories, that the fake's default raw P4 has exactly the
// width AND the shape the dialect's inventory declares:
//
//	width  len(P4) == item.Digits
//	shape  a numeric item answers all-'0'; a Text item would answer spaces
//
// THE TEXT BRANCH IS ASSERTED FROM THE DIALECT'S SIDE, as it is on the FT-891
// and unlike the FTdx10's: the fake CANNOT see textness here (B is three
// columns and carries no such cell), so every address it knows is numeric by
// construction, and the dialect's side is the only place the question can be
// asked. If A ever declares a Text item this test reports it as a projection
// the fake cannot represent — a finding about the transcription pair, not a bug
// to paper over by teaching the fake to guess.
//
// Addresses missing from one side entirely are reported by the set test above
// and skipped here, to avoid a duplicate and less specific failure.
func TestEXInventoryCrossCheck_FT991AWidthsAndShapesAgree(t *testing.T) {
	fakeAddrs := fakeft991a.EXDefaults()

	var mismatches []string
	checked, widest := 0, 0
	widestAt := ""
	for _, item := range ft991a.Dialect().EXItems() {
		addr := ft991a.Dialect().EXWire(item.Addr)
		p4, ok := fakeAddrs[addr]
		if !ok {
			continue // reported by TestEXInventoryCrossCheck_FT991AAddressSetsIdentical
		}
		checked++
		if item.Digits > widest {
			widest, widestAt = item.Digits, addr
		}
		if len(p4) != item.Digits {
			mismatches = append(mismatches, fmt.Sprintf("%s (%s): dialect Digits=%d, fake default is %d bytes (%q)",
				addr, item.Name, item.Digits, len(p4), p4))
			continue
		}
		if item.Text {
			mismatches = append(mismatches, fmt.Sprintf("%s (%s): the dialect declares a TEXT item, which transcription B's three-column schema cannot describe — the fake answers %q. Arbitrate against the PDF: either A is wrong about this row, or B's schema is too narrow for this chart and the fake's generator needs a text column it does not have",
				addr, item.Name, p4))
			continue
		}
		if strings.Trim(p4, "0") != "" {
			mismatches = append(mismatches, fmt.Sprintf("%s (%s): the dialect declares a NUMERIC item, so the fake must answer %d x '0'; it answers %q",
				addr, item.Name, item.Digits, p4))
		}
	}
	if checked == 0 {
		t.Fatal("no address present in both inventories — TestEXInventoryCrossCheck_FT991AAddressSetsIdentical should already have failed")
	}
	// The widest field on this chart is EIGHT bytes, where the FT-891's
	// alphabet stops at five and every other registered sibling's at four, and
	// it comes from exactly one row. Without this guard the whole comparison
	// could pass over a corpus that never exercised the token this radio's
	// generator had to be widened for.
	if widest != 8 || widestAt != "151" {
		t.Errorf("the widest Digits checked was %d at %s, want 8 at 151 (PRESET FREQUENCY, \"00030000 ~ 47000000\") — the '8' token is what this radio's width alphabet was extended for, and it was not exercised", widest, widestAt)
	}
	if len(mismatches) > 0 {
		t.Errorf("FT-991A EX width/shape disagreement between the dialect (from transcription A) and the fake (from transcription B) — %d of %d shared addresses:\n%s\n"+
			"This is a genuine cross-check finding, not a bug in this test: do not modify either table to make it pass — report the diff and arbitrate against the PDF.",
			len(mismatches), checked, strings.Join(mismatches, "\n"))
	}
}

// newFT991AEXFrameReader adapts a *fakeft991a.Radio's port to the frame reader
// this package's FT-710 cross-check already uses (exFrameReader, defined in
// ex_crosscheck_test.go — the read loop is shared; only the fake's type
// differs).
func newFT991AEXFrameReader(t *testing.T, r *fakeft991a.Radio) *exFrameReader {
	t.Helper()
	port := r.Port()
	deadliner, ok := port.(interface {
		Read([]byte) (int, error)
		SetReadDeadline(time.Time) error
	})
	if !ok {
		t.Fatalf("fakeft991a Radio.Port() (%T) does not support SetReadDeadline", port)
	}
	return &exFrameReader{
		t:    t,
		port: deadliner,
		acc:  cat.NewFrameAccumulator(0), // DefaultMaxFrame: comfortably above the 14-byte widest EX answer
		buf:  make([]byte, 256),
	}
}

// TestEXFT991ARoundTrip_AllAddressesRawPort is the wire-level leg: every
// address the DIALECT's inventory declares is asked of a real fakeft991a.Radio
// through its Port(), and each answer must echo the address asked for and carry
// the width and shape that inventory declares.
//
// It is the strong form of "every address in the dialect's inventory is
// answered by the fake": the table tests above compare two data structures,
// while this one puts frames on a wire, through the fake's own parser, and
// parses the replies with the dialect's own codec. A projection that produced
// the right table but answered the wrong bytes — a handler bug, an off-by-one
// in the answer builder — fails only here.
//
// IT ALSO EXERCISES THE SHARED FRAME LENGTH THAT MOVES, at its NARROWEST value
// in the family. BuildEXRead emits SIX bytes for this dialect and ParseEXAnswer
// expects a three-digit address field, both derived from Dialect.EXAddressWidth()
// rather than from a constant; the fake independently rejects anything that is
// not three digits. So a codec that had forked the width, or a fake that had
// copied a sibling's four or six, fails here rather than in a comment.
//
// Engine.Do is deliberately bypassed (direct Port() write plus an accumulator
// read), the same design constraint the FT-710's round trip records: 152
// exchanges at the engine's per-exchange Settle would pay seconds of pacing for
// nothing, and Engine's own correlation behaviour is covered by
// engine_ex_test.go.
func TestEXFT991ARoundTrip_AllAddressesRawPort(t *testing.T) {
	dialect := ft991a.Dialect()
	items := dialect.EXItems() // sorted by (P1,P2,P3)
	if len(items) == 0 {
		t.Fatal("ft991a.Dialect().EXItems() is empty — the dialect's inventory failed to generate, and this test would pass vacuously")
	}

	r := fakeft991a.New()
	t.Cleanup(func() { _ = r.Close() })
	fakeDefaults := fakeft991a.EXDefaults()
	reader := newFT991AEXFrameReader(t, r)

	answered := 0
	for _, item := range items {
		cmd, err := dialect.BuildEXRead(item.Addr)
		if err != nil {
			t.Fatalf("BuildEXRead(%v): unexpected error: %v", item.Addr, err)
		}
		if got := len(cmd.Bytes()); got != 6 {
			t.Fatalf("BuildEXRead(%v) built a %d-byte frame (%q), want 6 — this radio's EX read is \"EX\" + three digits + \";\"", item.Addr, got, cmd.Bytes())
		}
		if _, err := r.Port().Write(cmd.Bytes()); err != nil {
			t.Fatalf("Write(%v %q): unexpected error: %v", item.Addr, cmd.Bytes(), err)
		}

		frame := reader.readOneFrame()

		gotAddr, gotRaw, err := dialect.ParseEXAnswer(frame)
		if err != nil {
			t.Fatalf("ParseEXAnswer(%q) for %v: unexpected error: %v — the fake refused or malformed an address the dialect's inventory declares", frame, item.Addr, err)
		}
		if gotAddr != item.Addr {
			t.Errorf("%v: answer echoed address %v, want %v", item.Addr, gotAddr, item.Addr)
		}
		if len(gotRaw) != item.Digits {
			t.Errorf("%v (%s): answered %d raw P4 bytes (%q), want %d per the dialect's inventory", item.Addr, item.Name, len(gotRaw), gotRaw, item.Digits)
		}
		wantRaw, ok := fakeDefaults[dialect.EXWire(item.Addr)]
		if !ok {
			t.Errorf("%v: answered over the wire but absent from fakeft991a.EXDefaults() — inconsistent with handleEX's own membership check", item.Addr)
			continue
		}
		if gotRaw != wantRaw {
			t.Errorf("%v: raw P4 = %q, want %q (the fake's own default)", item.Addr, gotRaw, wantRaw)
		}
		answered++
	}
	if answered != len(items) {
		t.Errorf("%d of %d inventory addresses answered as expected", answered, len(items))
	}
}

// TestEXFT991ARoundTrip_OutOfInventoryAddressIsRefused is the round trip's
// negative control. Without it, a fake that answered EVERY three-digit address
// with something plausible would pass the test above completely — the loop only
// asks about addresses that ARE in the inventory.
//
// THE 087 CASE IS THIS RADIO'S OWN and is the one no sibling has: an address
// the chart PRINTS, that both transcriptions carry, and that neither generator
// admits, because it names no field. Both sides agree it is not a member, and
// the fake answers the same unattributed NAK it gives an address the chart
// never carried — internal/fakeft991a/doc.go's register entry AN
// OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;", ASSUMED here where the FT-710's
// equivalent was M8c-observed on its own grammar.
//
// THE WRONG-WIDTH CASES ARE THE FAMILY'S: a sibling's seven- or nine-byte read
// frame put to an FT-991A must not be answered. They are written by hand rather
// than built, because no dialect would build one for this radio.
func TestEXFT991ARoundTrip_OutOfInventoryAddressIsRefused(t *testing.T) {
	dialect := ft991a.Dialect()
	r := fakeft991a.New()
	t.Cleanup(func() { _ = r.Close() })
	reader := newFT991AEXFrameReader(t, r)

	for _, tc := range []struct {
		wire string
		p1   uint16
		why  string
	}{
		{"087", 87, "printed and transcribed, but it names no field: a single hyphen for Digits and ten for its parameter (plan P18)"},
		{"154", 154, "one past the chart's last row, 153"},
		{"000", 0, "the zero address; the chart's numbering starts at 001"},
	} {
		t.Run(tc.wire, func(t *testing.T) {
			// The dialect refuses to BUILD a read for an address it does not
			// know, which is itself worth pinning: the two sides agree that
			// these are not members.
			if dialect.KnownEXAddress(cat.EXAddress{P1: tc.p1}) {
				t.Fatalf("the dialect claims %s is a known address (%s) — this negative control is testing the wrong thing", tc.wire, tc.why)
			}

			// So the frame is written by hand, exactly as the grammar prints it.
			if _, err := r.Port().Write([]byte("EX" + tc.wire + ";")); err != nil {
				t.Fatalf("Write: unexpected error: %v", err)
			}
			if got, want := string(reader.readOneFrame()), "?;"; got != want {
				t.Errorf("EX%s; -> %q, want %q (%s)", tc.wire, got, want, tc.why)
			}
		})
	}

	for _, tc := range []struct {
		frame string
		why   string
	}{
		{"EX0101;", "the FT-891's four-digit pair address"},
		{"EX010100;", "every other registered sibling's six-digit triple address"},
	} {
		t.Run(strings.TrimSuffix(tc.frame, ";"), func(t *testing.T) {
			if _, err := r.Port().Write([]byte(tc.frame)); err != nil {
				t.Fatalf("Write: unexpected error: %v", err)
			}
			if got, want := string(reader.readOneFrame()), "?;"; got != want {
				t.Errorf("%s -> %q, want %q — an FT-991A must not answer %s, whatever its first three digits name", tc.frame, got, want, tc.why)
			}
		})
	}
}

// TestFT991ATranscriptionBCopy_ByteIdenticalToTheDialects pins the
// COPY-NOT-MOVE rule mechanically (internal/fakeft991a/PROVENANCE.md).
//
// The fake's transcription-b.csv is a copy: the dialect's own copy does not
// move, because core/cat/ft991a/crosscheck_test.go keeps reading it as one of
// the artefacts it binds — and hashes it by name in its frozen-evidence map.
// The cross-check above is only "dialect from A vs fake from B" for as long as
// the fake's copy really is B: a drifted copy would still generate a table, and
// a table quietly derived from a private variant of B is exactly the kind of
// agreement this milestone is trying not to have.
//
// If the dialect's copy is ever corrected by an arbitration against the PDF,
// this test fails until the fake's copy is re-copied and its inventory
// regenerated. That is the intended workflow, not an obstacle to it.
func TestFT991ATranscriptionBCopy_ByteIdenticalToTheDialects(t *testing.T) {
	const (
		dialectCopy = "../cat/ft991a/testdata/transcription-b.csv"
		fakeCopy    = "../../internal/fakeft991a/transcription-b.csv"
	)
	want, err := os.ReadFile(dialectCopy)
	if err != nil {
		t.Fatalf("reading %s: %v", dialectCopy, err)
	}
	got, err := os.ReadFile(fakeCopy)
	if err != nil {
		t.Fatalf("reading %s: %v", fakeCopy, err)
	}
	if len(want) == 0 {
		t.Fatalf("%s is empty — this comparison would pass vacuously", dialectCopy)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is not byte-identical to %s (%d bytes vs %d): the fake's EX inventory is generated from its copy, so a drifted copy silently stops being transcription B.\n"+
			"Re-copy the dialect's artefact and run `go generate ./internal/fakeft991a` — or, if the divergence is a deliberate arbitration, record it in internal/fakeft991a/PROVENANCE.md.",
			fakeCopy, dialectCopy, len(got), len(want))
	}
}
