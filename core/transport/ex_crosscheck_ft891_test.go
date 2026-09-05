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
	"github.com/gm5dna/open-rig-programmer/core/cat/ft891"
	"github.com/gm5dna/open-rig-programmer/internal/fakeft891"
)

// This file is the FT-891's half of the two-independent-transcriptions
// cross-check — the shape of ex_crosscheck_ftdx10_test.go beside it, on a radio
// whose EX address is a PAIR:
//
//	the DIALECT's inventory (ft891.Dialect().EXItems()) is generated from
//	TRANSCRIPTION A (core/cat/ft891/table2.csv) by internal/extable;
//
//	the FAKE's inventory (fakeft891.EXDefaults()) is generated from
//	TRANSCRIPTION B (internal/fakeft891/transcription-b.csv, its own copy) by
//	internal/fakeft891/gen, which imports nothing project-internal at all.
//
// A and B are two independent derivations of one printed chart (manual rev
// 1909-C, the MENU chart following the EX block): A layout-text-led and
// PDF-checked, B derived PDF-primary by a quarantined agent with no repository
// access and no sight of A, the group-boundary ledger or any row count. The two
// GENERATORS are independent too — that is what the fake's recursive no-imports
// fence enforces, gen/ included. So a mis-read row in either transcription, or
// a defect in either generator, surfaces HERE as a mismatch rather than as two
// tables quietly agreeing on the same wrong number.
//
// # What is DIFFERENT here, and it is not only the address width
//
// The FTdx10's cross-check has a SHAPE half as well as a width half: its chart
// carries one text item (MY CALL.), the fake answers twelve spaces for it and
// twelve zeros for every numeric item, and 12 spaces and 12 zeros are the same
// length — so a projection that lost the text discriminator would pass a
// width-only comparison. This chart has no text row, and B's three-column
// schema carries no cell from which one could be identified
// (internal/fakeft891/gen/main.go's widthToken refuses rather than guessing).
//
// That makes the shape half MORE important here, not less, and it is asserted
// in the direction the asymmetry demands: the fake's side cannot see a text
// item, so THE DIALECT'S SIDE is where the claim is checked. If A ever declares
// one, TestEXInventoryCrossCheck_FT891WidthsAndShapesAgree fails on that item
// rather than silently accepting five zeros where twelve spaces belong.
//
// It lives in core/transport for the reason its siblings do: this is the
// existing test home that already imports both core/cat and a fake (see
// engine_test.go), which keeps the fakes' own test packages core-free. Nothing
// in this file is imported by production code.
//
// ON FAILURE: report the exact diff, and do NOT "fix" either table to make it
// pass. Which side is wrong — or whether the printed chart is — is an
// arbitration against the PDF. An edit that merely restores agreement destroys
// the evidence the agreement was worth.
//
// ONE CLASS NO LEG HERE CAN CATCH, stated so that the silence is a recorded
// limit: a defect PRINTED in the chart is read faithfully by both derivations
// and both tables agree on it. 0905 RPT SHIFT 50MHz is one — Digits 1 against a
// legend needing four — and it is pinned as a deliberate state by
// core/cat/ft891/crosscheck_test.go and by internal/fakeft891/PROVENANCE.md,
// not by anything below.

// Each test below fetches both inventories itself rather than sharing a
// package-level fixture: both APIs return fresh copies by contract
// (Dialect().EXItems() copies its slice, EXDefaults() its map), and a shared
// fixture would hide it if either stopped.

// TestEXInventoryCrossCheck_FT891AddressSetsIdentical compares the dialect's EX
// address set against the fake's, reporting BOTH diff directions so a report can
// quote the exact addresses without re-deriving the diff.
//
// This is the "every address the fake answers is in the dialect's inventory"
// direction and its converse, at table level; the round trip below proves the
// same thing over the wire.
func TestEXInventoryCrossCheck_FT891AddressSetsIdentical(t *testing.T) {
	dialectAddrs := make(map[string]bool)
	for _, item := range ft891.Dialect().EXItems() {
		dialectAddrs[ft891.Dialect().EXWire(item.Addr)] = true
	}
	fakeAddrs := fakeft891.EXDefaults()

	// Vacuity guard: two empty sets are trivially equal, and an inventory that
	// failed to generate would be exactly that.
	if len(dialectAddrs) == 0 || len(fakeAddrs) == 0 {
		t.Fatalf("empty inventory: dialect has %d addresses, fake has %d — one side failed to generate, and the comparison below would pass vacuously", len(dialectAddrs), len(fakeAddrs))
	}

	// The address is a PAIR on this radio, and the two sides have to agree
	// about that before they can meaningfully agree about membership: a
	// six-digit render on either side would make every address miss, which is
	// a true failure but an unhelpfully phrased one.
	if got := ft891.Dialect().EXAddressWidth(); got != 4 {
		t.Fatalf("ft891.Dialect().EXAddressWidth() = %d, want 4 — this radio's EX address is a pair (core/cat's EXAddressPair)", got)
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
		t.Errorf("FT-891 EX address inventories disagree between the two independent MENU chart transcriptions (dialect/A has %d addresses, fake/B has %d):\n"+
			"  in ft891.Dialect().EXItems() but NOT in fakeft891.EXDefaults() (%d): %v\n"+
			"  in fakeft891.EXDefaults() but NOT in ft891.Dialect().EXItems() (%d): %v\n"+
			"This is a genuine cross-check finding, not a bug in this test: do not modify either table to make it pass — report the diff and arbitrate against the PDF.",
			len(dialectAddrs), len(fakeAddrs), len(inDialectNotFake), inDialectNotFake, len(inFakeNotDialect), inFakeNotDialect)
	}
}

// TestEXInventoryCrossCheck_FT891WidthsAndShapesAgree checks, for every address
// in BOTH inventories, that the fake's default raw P4 has exactly the width AND
// the shape the dialect's inventory declares:
//
//	width  len(P4) == item.Digits
//	shape  a numeric item answers all-'0'; a Text item would answer spaces
//
// THE TEXT BRANCH IS THE INTERESTING ONE ON THIS RADIO, and it is written the
// other way round from the FTdx10's. There, the fake CAN see textness (B has a
// P4 legend column) and the test asserts the two sides agree about which item
// is text. Here the fake CANNOT: B is three columns and carries no such cell,
// so every address it knows is numeric by construction. The dialect's side is
// therefore the only place the question can be asked, and if A ever declares a
// Text item this test reports it as a projection the fake cannot represent —
// which is a finding about the transcription pair, not a bug to paper over by
// teaching the fake to guess.
//
// Addresses missing from one side entirely are reported by the set test above
// and skipped here, to avoid a duplicate and less specific failure.
func TestEXInventoryCrossCheck_FT891WidthsAndShapesAgree(t *testing.T) {
	fakeAddrs := fakeft891.EXDefaults()

	var mismatches []string
	checked, widest := 0, 0
	for _, item := range ft891.Dialect().EXItems() {
		addr := ft891.Dialect().EXWire(item.Addr)
		p4, ok := fakeAddrs[addr]
		if !ok {
			continue // reported by TestEXInventoryCrossCheck_FT891AddressSetsIdentical
		}
		checked++
		if item.Digits > widest {
			widest = item.Digits
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
		t.Fatal("no address present in both inventories — TestEXInventoryCrossCheck_FT891AddressSetsIdentical should already have failed")
	}
	// The widest field on this chart is FIVE bytes, where every registered
	// sibling's numeric alphabet stops at four. Without this guard the whole
	// comparison could pass over a corpus that never exercised the token the
	// FT-891's generator had to be widened for.
	if widest != 5 {
		t.Errorf("the widest Digits checked was %d, want 5 — the '5' token (0803 OTHER DISP, 0804 OTHER SHIFT) is what this radio's width alphabet was extended for, and it was not exercised", widest)
	}
	if len(mismatches) > 0 {
		t.Errorf("FT-891 EX width/shape disagreement between the dialect (from transcription A) and the fake (from transcription B) — %d of %d shared addresses:\n%s\n"+
			"This is a genuine cross-check finding, not a bug in this test: do not modify either table to make it pass — report the diff and arbitrate against the PDF.",
			len(mismatches), checked, strings.Join(mismatches, "\n"))
	}
}

// newFT891EXFrameReader adapts a *fakeft891.Radio's port to the frame reader
// this package's FT-710 cross-check already uses (exFrameReader, defined in
// ex_crosscheck_test.go — the read loop is shared; only the fake's type
// differs).
func newFT891EXFrameReader(t *testing.T, r *fakeft891.Radio) *exFrameReader {
	t.Helper()
	port := r.Port()
	deadliner, ok := port.(interface {
		Read([]byte) (int, error)
		SetReadDeadline(time.Time) error
	})
	if !ok {
		t.Fatalf("fakeft891 Radio.Port() (%T) does not support SetReadDeadline", port)
	}
	return &exFrameReader{
		t:    t,
		port: deadliner,
		acc:  cat.NewFrameAccumulator(0), // DefaultMaxFrame: comfortably above the 12-byte widest EX answer
		buf:  make([]byte, 256),
	}
}

// TestEXFT891RoundTrip_AllAddressesRawPort is the wire-level leg: every address
// the DIALECT's inventory declares is asked of a real fakeft891.Radio through
// its Port(), and each answer must echo the address asked for and carry the
// width and shape that inventory declares.
//
// It is the strong form of "every address in the dialect's inventory is
// answered by the fake": the table tests above compare two data structures,
// while this one puts frames on a wire, through the fake's own parser, and
// parses the replies with the dialect's own codec. A projection that produced
// the right table but answered the wrong bytes — a handler bug, an off-by-one
// in the answer builder — fails only here.
//
// IT ALSO EXERCISES THE ONE SHARED FRAME LENGTH THAT MOVES. BuildEXRead emits
// SEVEN bytes for this dialect and ParseEXAnswer expects a four-digit address
// field, both derived from Dialect.EXAddressWidth() rather than from a
// constant; the fake independently rejects anything that is not four digits.
// So a codec that had forked the width, or a fake that had copied a sibling's
// six, fails here rather than in a comment.
//
// Engine.Do is deliberately bypassed (direct Port() write plus an accumulator
// read), the same design constraint the FT-710's round trip records: 159
// exchanges at the engine's per-exchange Settle would pay seconds of pacing for
// nothing, and Engine's own correlation behaviour is covered by
// engine_ex_test.go.
func TestEXFT891RoundTrip_AllAddressesRawPort(t *testing.T) {
	dialect := ft891.Dialect()
	items := dialect.EXItems() // sorted by (P1,P2,P3)
	if len(items) == 0 {
		t.Fatal("ft891.Dialect().EXItems() is empty — the dialect's inventory failed to generate, and this test would pass vacuously")
	}

	r := fakeft891.New()
	t.Cleanup(func() { _ = r.Close() })
	fakeDefaults := fakeft891.EXDefaults()
	reader := newFT891EXFrameReader(t, r)

	answered := 0
	for _, item := range items {
		cmd, err := dialect.BuildEXRead(item.Addr)
		if err != nil {
			t.Fatalf("BuildEXRead(%v): unexpected error: %v", item.Addr, err)
		}
		if got := len(cmd.Bytes()); got != 7 {
			t.Fatalf("BuildEXRead(%v) built a %d-byte frame (%q), want 7 — this radio's EX read is \"EX\" + four digits + \";\"", item.Addr, got, cmd.Bytes())
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
			t.Errorf("%v: answered over the wire but absent from fakeft891.EXDefaults() — inconsistent with handleEX's own membership check", item.Addr)
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

// TestEXFT891RoundTrip_OutOfInventoryAddressIsRefused is the round trip's
// negative control. Without it, a fake that answered EVERY four-digit address
// with something plausible would pass the test above completely — the loop only
// asks about addresses that ARE in the inventory.
//
// The addresses are the fake's own out-of-inventory cases: 0104 is past the end
// of a real group (P1=01 has three items), 1901 is a group prefix the chart has
// no rows for at all (its last is 18), and 0000 is the zero address, which no
// row can carry since P2 numbering starts at 01. All three draw "?;" — ASSUMED
// for this radio, internal/fakeft891/doc.go's register entry AN
// OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;", where the FT-710's equivalent was
// M8c-observed on its own grammar.
//
// THE SIX-DIGIT CASE IS THIS RADIO'S OWN, and has no counterpart in the
// siblings' files: a sibling's nine-byte read frame put to an FT-891 must not
// be answered. It is written by hand rather than built, because no dialect
// would build it for this radio.
func TestEXFT891RoundTrip_OutOfInventoryAddressIsRefused(t *testing.T) {
	dialect := ft891.Dialect()
	r := fakeft891.New()
	t.Cleanup(func() { _ = r.Close() })
	reader := newFT891EXFrameReader(t, r)

	for _, tc := range []struct {
		wire string
		why  string
	}{
		{"0104", "past the end of group 01, which has three items"},
		{"1901", "a group prefix the chart has no rows for; its last is 18"},
		{"0000", "the zero address; P2 numbering starts at 01"},
	} {
		t.Run(tc.wire, func(t *testing.T) {
			// The dialect refuses to BUILD a read for an address it does not
			// know, which is itself worth pinning: the two sides agree that
			// these are not members.
			addr := cat.EXAddress{
				P1: uint8(10*(tc.wire[0]-'0') + (tc.wire[1] - '0')),
				P2: uint8(10*(tc.wire[2]-'0') + (tc.wire[3] - '0')),
			}
			if dialect.KnownEXAddress(addr) {
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

	t.Run("a sibling's six-digit read frame", func(t *testing.T) {
		if _, err := r.Port().Write([]byte("EX010100;")); err != nil {
			t.Fatalf("Write: unexpected error: %v", err)
		}
		if got, want := string(reader.readOneFrame()), "?;"; got != want {
			t.Errorf("EX010100; -> %q, want %q — an FT-891 must not answer a six-digit EX read, whatever the first four digits name", got, want)
		}
	})
}

// TestFT891TranscriptionBCopy_ByteIdenticalToTheDialects pins the COPY-NOT-MOVE
// rule mechanically (internal/fakeft891/PROVENANCE.md).
//
// The fake's transcription-b.csv is a copy: the dialect's own copy does not
// move, because core/cat/ft891/crosscheck_test.go keeps reading it as one of
// the three artefacts it binds — and hashes it by name in its frozen-evidence
// map. The cross-check above is only "dialect from A vs fake from B" for as
// long as the fake's copy really is B: a drifted copy would still generate a
// table, and a table quietly derived from a private variant of B is exactly the
// kind of agreement this milestone is trying not to have.
//
// If the dialect's copy is ever corrected by an arbitration against the PDF,
// this test fails until the fake's copy is re-copied and its inventory
// regenerated. That is the intended workflow, not an obstacle to it.
func TestFT891TranscriptionBCopy_ByteIdenticalToTheDialects(t *testing.T) {
	const (
		dialectCopy = "../cat/ft891/testdata/transcription-b.csv"
		fakeCopy    = "../../internal/fakeft891/transcription-b.csv"
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
			"Re-copy the dialect's artefact and run `go generate ./internal/fakeft891` — or, if the divergence is a deliberate arbitration, record it in internal/fakeft891/PROVENANCE.md.",
			fakeCopy, dialectCopy, len(got), len(want))
	}
}
