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
	"github.com/gm5dna/open-rig-programmer/core/cat/ftdx10"
	"github.com/gm5dna/open-rig-programmer/internal/fakedx10"
)

// This file is the FTdx10's half of the two-independent-transcriptions
// cross-check — the FT-710 pattern in ex_crosscheck_test.go beside it, applied
// to a radio whose two sides are generated rather than hand-typed:
//
//	the DIALECT's inventory (ftdx10.Dialect().EXItems()) is generated from
//	TRANSCRIPTION A (core/cat/ftdx10/table2.csv) by internal/extable;
//
//	the FAKE's inventory (fakedx10.EXDefaults()) is generated from
//	TRANSCRIPTION B (internal/fakedx10/transcription-b.csv, its own copy) by
//	internal/fakedx10/gen, which imports nothing project-internal at all.
//
// A and B are two independent derivations of one printed chart (manual rev
// 2308-F, Table 2 "MENU Chart"): A layout-text-led and PDF-checked, B derived
// PDF-primary by a quarantined agent with no repository access and no sight of A,
// the group-boundary ledger or any row count. The two GENERATORS are independent
// too — that is what the fake's recursive no-imports fence enforces, gen/
// included. So a mis-read row in either transcription, or a defect in either
// generator, surfaces HERE as a mismatch rather than as two tables quietly
// agreeing on the same wrong number.
//
// It lives in core/transport for the reason its FT-710 sibling does: this is the
// existing test home that already imports both core/cat and a fake (see
// engine_test.go), which keeps the fakes' own test packages core-free. Nothing
// in this file is imported by production code.
//
// ON FAILURE: report the exact diff, and do NOT "fix" either table to make it
// pass. Which side is wrong — or whether the printed chart is — is an
// arbitration against the PDF, exactly as it is for the FT-710. An edit that
// merely restores agreement destroys the evidence the agreement was worth.

// Each test below fetches both inventories itself rather than sharing a
// package-level fixture: both APIs return fresh copies by contract
// (Dialect().EXItems() copies its slice, EXDefaults() its map), and a shared
// fixture would hide it if either stopped.

// TestEXInventoryCrossCheck_FTdx10AddressSetsIdentical compares the dialect's
// EX address set against the fake's, reporting BOTH diff directions so a report
// can quote the exact addresses without re-deriving the diff.
//
// This is the "every address the fake answers is in the dialect's inventory"
// direction and its converse, at table level; the round trip below proves the
// same thing over the wire.
func TestEXInventoryCrossCheck_FTdx10AddressSetsIdentical(t *testing.T) {
	dialectAddrs := make(map[string]bool)
	for _, item := range ftdx10.Dialect().EXItems() {
		dialectAddrs[ftdx10.Dialect().EXWire(item.Addr)] = true
	}
	fakeAddrs := fakedx10.EXDefaults()

	// Vacuity guard: two empty sets are trivially equal, and an inventory that
	// failed to generate would be exactly that.
	if len(dialectAddrs) == 0 || len(fakeAddrs) == 0 {
		t.Fatalf("empty inventory: dialect has %d addresses, fake has %d — one side failed to generate, and the comparison below would pass vacuously", len(dialectAddrs), len(fakeAddrs))
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
		t.Errorf("FTdx10 EX address inventories disagree between the two independent Table 2 transcriptions (dialect/A has %d addresses, fake/B has %d):\n"+
			"  in ftdx10.Dialect().EXItems() but NOT in fakedx10.EXDefaults() (%d): %v\n"+
			"  in fakedx10.EXDefaults() but NOT in ftdx10.Dialect().EXItems() (%d): %v\n"+
			"This is a genuine cross-check finding, not a bug in this test: do not modify either table to make it pass — report the diff and arbitrate against the PDF.",
			len(dialectAddrs), len(fakeAddrs), len(inDialectNotFake), inDialectNotFake, len(inFakeNotDialect), inFakeNotDialect)
	}
}

// TestEXInventoryCrossCheck_FTdx10WidthsAndShapesAgree checks, for every address
// in BOTH inventories, that the fake's default raw P4 has exactly the width AND
// the shape the dialect's inventory declares:
//
//	width  len(P4) == item.Digits
//	shape  a Text item answers 12 SPACES; a numeric item answers all-'0'
//
// The shape half matters as much as the width: 12 zeros and 12 spaces are the
// same length, and a projection that lost the text discriminator would put the
// wrong one in the call-sign field while the widths still matched.
//
// Addresses missing from one side entirely are reported by the set test above
// and skipped here, to avoid a duplicate and less specific failure.
func TestEXInventoryCrossCheck_FTdx10WidthsAndShapesAgree(t *testing.T) {
	fakeAddrs := fakedx10.EXDefaults()

	var mismatches []string
	checked, textChecked := 0, 0
	for _, item := range ftdx10.Dialect().EXItems() {
		addr := ftdx10.Dialect().EXWire(item.Addr)
		p4, ok := fakeAddrs[addr]
		if !ok {
			continue // reported by TestEXInventoryCrossCheck_FTdx10AddressSetsIdentical
		}
		checked++
		if len(p4) != item.Digits {
			mismatches = append(mismatches, fmt.Sprintf("%s (%s / %s): dialect Digits=%d, fake default is %d bytes (%q)",
				addr, item.P1Label, item.Name, item.Digits, len(p4), p4))
			continue
		}
		if item.Text {
			textChecked++
			if want := strings.Repeat(" ", item.Digits); p4 != want {
				mismatches = append(mismatches, fmt.Sprintf("%s (%s / %s): the dialect declares a TEXT item, so the fake must answer %d spaces; it answers %q",
					addr, item.P1Label, item.Name, item.Digits, p4))
			}
			continue
		}
		if strings.Trim(p4, "0") != "" {
			mismatches = append(mismatches, fmt.Sprintf("%s (%s / %s): the dialect declares a NUMERIC item, so the fake must answer %d x '0'; it answers %q",
				addr, item.P1Label, item.Name, item.Digits, p4))
		}
	}
	if checked == 0 {
		t.Fatal("no address present in both inventories — TestEXInventoryCrossCheck_FTdx10AddressSetsIdentical should already have failed")
	}
	if textChecked == 0 {
		t.Error("no TEXT item was checked — the FTdx10 chart has exactly one (040101, MY CALL.), so the shape half of this test passed without exercising its text branch")
	}
	if len(mismatches) > 0 {
		t.Errorf("FTdx10 EX width/shape disagreement between the dialect (from transcription A) and the fake (from transcription B) — %d of %d shared addresses:\n%s\n"+
			"This is a genuine cross-check finding, not a bug in this test: do not modify either table to make it pass — report the diff and arbitrate against the PDF.",
			len(mismatches), checked, strings.Join(mismatches, "\n"))
	}
}

// newFTdx10EXFrameReader adapts a *fakedx10.Radio's port to the frame reader
// this package's FT-710 cross-check already uses (exFrameReader, defined in
// ex_crosscheck_test.go — the read loop is shared; only the fake's type differs).
func newFTdx10EXFrameReader(t *testing.T, r *fakedx10.Radio) *exFrameReader {
	t.Helper()
	port := r.Port()
	deadliner, ok := port.(interface {
		Read([]byte) (int, error)
		SetReadDeadline(time.Time) error
	})
	if !ok {
		t.Fatalf("fakedx10 Radio.Port() (%T) does not support SetReadDeadline", port)
	}
	return &exFrameReader{
		t:    t,
		port: deadliner,
		acc:  cat.NewFrameAccumulator(0), // DefaultMaxFrame: comfortably above the 21-byte widest EX answer
		buf:  make([]byte, 256),
	}
}

// TestEXFTdx10RoundTrip_AllAddressesRawPort is the wire-level leg: every address
// the DIALECT's inventory declares is asked of a real fakedx10.Radio through its
// Port(), and each answer must echo the address asked for and carry the width and
// shape that inventory declares.
//
// It is the strong form of "every address in the dialect's inventory is answered
// by the fake": the table tests above compare two data structures, while this one
// puts frames on a wire, through the fake's own parser, and parses the replies
// with the dialect's own codec. A projection that produced the right table but
// answered the wrong bytes — a handler bug, an off-by-one in the answer builder —
// fails only here.
//
// Engine.Do is deliberately bypassed (direct Port() write plus an accumulator
// read), the same design constraint the FT-710's round trip records: 197 exchanges
// at the engine's per-exchange Settle would pay seconds of pacing for nothing, and
// Engine's own correlation behaviour is covered by engine_ex_test.go.
func TestEXFTdx10RoundTrip_AllAddressesRawPort(t *testing.T) {
	dialect := ftdx10.Dialect()
	items := dialect.EXItems() // sorted by (P1,P2,P3)
	if len(items) == 0 {
		t.Fatal("ftdx10.Dialect().EXItems() is empty — the dialect's inventory failed to generate, and this test would pass vacuously")
	}

	r := fakedx10.New()
	t.Cleanup(func() { _ = r.Close() })
	fakeDefaults := fakedx10.EXDefaults()
	reader := newFTdx10EXFrameReader(t, r)

	answered := 0
	for _, item := range items {
		cmd, err := dialect.BuildEXRead(item.Addr)
		if err != nil {
			t.Fatalf("BuildEXRead(%v): unexpected error: %v", item.Addr, err)
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
			t.Errorf("%v (%s / %s): answered %d raw P4 bytes (%q), want %d per the dialect's inventory", item.Addr, item.P1Label, item.Name, len(gotRaw), gotRaw, item.Digits)
		}
		wantRaw, ok := fakeDefaults[ftdx10.Dialect().EXWire(item.Addr)]
		if !ok {
			t.Errorf("%v: answered over the wire but absent from fakedx10.EXDefaults() — inconsistent with handleEX's own membership check", item.Addr)
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

// TestEXFTdx10RoundTrip_OutOfInventoryAddressIsRefused is the round trip's
// negative control. Without it, a fake that answered EVERY six-digit address
// with something plausible would pass the test above completely — the loop only
// asks about addresses that ARE in the inventory.
//
// The addresses below are the fake's own out-of-inventory cases: no P1=05 group
// exists in this radio's chart (the EX grammar block says "P1 : 01 - 05" and the
// chart populates 01-04 — core/cat/ftdx10/doc.go records the anomaly
// UNRESOLVED), and 010199 is a P3 far beyond any group's item count. Both draw
// "?;" — ASSUMED for this radio, doc.go register entry 17.
func TestEXFTdx10RoundTrip_OutOfInventoryAddressIsRefused(t *testing.T) {
	dialect := ftdx10.Dialect()
	r := fakedx10.New()
	t.Cleanup(func() { _ = r.Close() })
	reader := newFTdx10EXFrameReader(t, r)

	for _, wire := range []string{"050101", "010199"} {
		t.Run(wire, func(t *testing.T) {
			// The dialect refuses to BUILD a read for an address it does not
			// know, which is itself worth pinning: the two sides agree that
			// these are not members.
			addr := cat.EXAddress{
				P1: uint8(10*(wire[0]-'0') + (wire[1] - '0')),
				P2: uint8(10*(wire[2]-'0') + (wire[3] - '0')),
				P3: uint8(10*(wire[4]-'0') + (wire[5] - '0')),
			}
			if dialect.KnownEXAddress(addr) {
				t.Fatalf("the dialect claims %s is a known address — this negative control is testing the wrong thing", wire)
			}

			// So the frame is written by hand, exactly as the grammar prints it.
			if _, err := r.Port().Write([]byte("EX" + wire + ";")); err != nil {
				t.Fatalf("Write: unexpected error: %v", err)
			}
			if got, want := string(reader.readOneFrame()), "?;"; got != want {
				t.Errorf("EX%s; -> %q, want %q", wire, got, want)
			}
		})
	}
}

// TestFTdx10TranscriptionBCopy_ByteIdenticalToTheDialects pins the COPY-NOT-MOVE
// rule mechanically (internal/fakedx10/PROVENANCE.md).
//
// The fake's transcription-b.csv is a copy: the dialect's own copy does not move,
// because core/cat/ftdx10/crosscheck_test.go keeps reading it as one of the three
// artefacts it binds. The cross-check above is only "dialect from A vs fake from
// B" for as long as the fake's copy really is B — a drifted copy would still
// generate a table, and a table quietly derived from a private variant of B is
// exactly the kind of agreement this milestone is trying not to have.
//
// If the dialect's copy is ever corrected by an arbitration against the PDF, this
// test fails until the fake's copy is re-copied and its inventory regenerated.
// That is the intended workflow, not an obstacle to it.
func TestFTdx10TranscriptionBCopy_ByteIdenticalToTheDialects(t *testing.T) {
	const (
		dialectCopy = "../cat/ftdx10/testdata/transcription-b.csv"
		fakeCopy    = "../../internal/fakedx10/transcription-b.csv"
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
			"Re-copy the dialect's artefact and run `go generate ./internal/fakedx10` — or, if the divergence is a deliberate arbitration, record it in internal/fakedx10/PROVENANCE.md.",
			fakeCopy, dialectCopy, len(got), len(want))
	}
}
