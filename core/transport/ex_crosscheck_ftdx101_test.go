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
	"github.com/gm5dna/open-rig-programmer/core/cat/ftdx101"
	"github.com/gm5dna/open-rig-programmer/internal/fakedx101"
)

// This file is the FTdx101's half of the two-independent-transcriptions
// cross-check — the FT-710 pattern in ex_crosscheck_test.go and the FTdx10's in
// ex_crosscheck_ftdx10_test.go beside it, applied to a family whose two sides
// are generated rather than hand-typed:
//
//	the DIALECT's inventory (ftdx101.DialectD().EXItems()) is generated from
//	TRANSCRIPTION A (core/cat/ftdx101/table2.csv) by internal/extable;
//
//	the FAKE's inventory (fakedx101.EXDefaults()) is generated from
//	TRANSCRIPTION B (internal/fakedx101/transcription-b.csv, its own copy) by
//	internal/fakedx101/gen, which imports nothing project-internal at all.
//
// A and B are two independent derivations of one printed chart (manual rev
// 2308-L, Table 2 "MENU Chart"): A layout-text-led and PDF-checked, B derived
// PDF-primary by a quarantined agent with no repository access, no sight of A or
// of the group-boundary ledger, and no row count or address given — B's own
// header block records that no text layer was consulted at all. The two
// GENERATORS are independent too — that is what the fake's recursive no-imports
// fence enforces, gen/ included. So a mis-read row in either transcription, or a
// defect in either generator, surfaces HERE as a mismatch rather than as two
// tables quietly agreeing on the same wrong number.
//
// core/cat/ftdx101/crosscheck_test.go binds A, B and the LEDGER to one another
// inside the dialect's own package. This file is the different question: it binds
// the dialect's generated inventory to the FAKE's, and then puts the whole thing
// on a wire — at BOTH models, which is a leg neither of the single-model
// siblings has.
//
// It lives in core/transport for the reason its two siblings do: this is the
// existing test home that already imports both core/cat and a fake (see
// engine_test.go), which keeps the fakes' own test packages core-free. Nothing in
// this file is imported by production code.
//
// ON FAILURE: report the exact diff, and do NOT "fix" either table to make it
// pass. Which side is wrong — or whether the printed chart is — is an
// arbitration against the PDF, exactly as it is for the FT-710 and the FTdx10,
// and exactly the STOP the dialect's own three-way cross-check declares. An edit
// that merely restores agreement destroys the evidence the agreement was worth.

// Each test below fetches both inventories itself rather than sharing a
// package-level fixture: both APIs return fresh copies by contract
// (DialectD().EXItems() copies its slice, EXDefaults() its map), and a shared
// fixture would hide it if either stopped.

// TestEXInventoryCrossCheck_FTdx101AddressSetsIdentical compares the dialect's
// EX address set against the fake's, reporting BOTH diff directions so a report
// can quote the exact addresses without re-deriving the diff.
//
// This is the "every address the fake answers is in the dialect's inventory"
// direction and its converse, at table level; the round trip below proves the
// same thing over the wire.
//
// DialectD() is the side taken here, and DialectMP() is not separately compared,
// because core/cat/ftdx101's own dialect_test.go already pins that the two
// dialects share one inventory (its EXItems and KnownEXAddress union tests). Two
// comparisons against the same slice would double the output of a failure without
// adding a fact. The MODEL question this file does own is on the wire, where the
// fake has two constructors — see the ID-divergence leg below.
func TestEXInventoryCrossCheck_FTdx101AddressSetsIdentical(t *testing.T) {
	dialectAddrs := make(map[string]bool)
	for _, item := range ftdx101.DialectD().EXItems() {
		dialectAddrs[ftdx101.DialectD().EXWire(item.Addr)] = true
	}
	fakeAddrs := fakedx101.EXDefaults()

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
		t.Errorf("FTdx101 EX address inventories disagree between the two independent Table 2 transcriptions (dialect/A has %d addresses, fake/B has %d):\n"+
			"  in ftdx101.DialectD().EXItems() but NOT in fakedx101.EXDefaults() (%d): %v\n"+
			"  in fakedx101.EXDefaults() but NOT in ftdx101.DialectD().EXItems() (%d): %v\n"+
			"This is a genuine cross-check finding, not a bug in this test: do not modify either table to make it pass — report the diff and arbitrate against the PDF.",
			len(dialectAddrs), len(fakeAddrs), len(inDialectNotFake), inDialectNotFake, len(inFakeNotDialect), inFakeNotDialect)
	}
}

// TestEXInventoryCrossCheck_FTdx101WidthsAndShapesAgree checks, for every address
// in BOTH inventories, that the fake's default raw P4 has exactly the width AND
// the shape the dialect's inventory declares:
//
//	width  len(P4) == item.Digits
//	shape  a Text item answers 12 SPACES; a numeric item answers all-'0'
//
// The shape half matters as much as the width: 12 zeros and 12 spaces are the
// same length, and a projection that lost the text discriminator would put the
// wrong one in the call-sign field while the widths still matched. On this radio
// the two sides reach that flag DIFFERENTLY — A carries a `text` column that
// extable stores, B carries its own independently transcribed `text` column that
// the fake's generator cross-checks against the digits cell — so the agreement
// below is two blind readings of one printed P4 cell agreeing, not one reading
// compared with itself.
//
// Addresses missing from one side entirely are reported by the set test above
// and skipped here, to avoid a duplicate and less specific failure.
func TestEXInventoryCrossCheck_FTdx101WidthsAndShapesAgree(t *testing.T) {
	fakeAddrs := fakedx101.EXDefaults()

	var mismatches []string
	checked, textChecked := 0, 0
	for _, item := range ftdx101.DialectD().EXItems() {
		addr := ftdx101.DialectD().EXWire(item.Addr)
		p4, ok := fakeAddrs[addr]
		if !ok {
			continue // reported by TestEXInventoryCrossCheck_FTdx101AddressSetsIdentical
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
		t.Fatal("no address present in both inventories — TestEXInventoryCrossCheck_FTdx101AddressSetsIdentical should already have failed")
	}
	if textChecked != 1 {
		t.Errorf("%d TEXT items were checked, want exactly 1 — the FTdx101 chart has one (040101, MY CALL.), so any other count means the shape half of this test is not exercising what it claims", textChecked)
	}
	if len(mismatches) > 0 {
		t.Errorf("FTdx101 EX width/shape disagreement between the dialect (from transcription A) and the fake (from transcription B) — %d of %d shared addresses:\n%s\n"+
			"This is a genuine cross-check finding, not a bug in this test: do not modify either table to make it pass — report the diff and arbitrate against the PDF.",
			len(mismatches), checked, strings.Join(mismatches, "\n"))
	}
}

// newFTdx101EXFrameReader adapts a *fakedx101.Radio's port to the frame reader
// this package's FT-710 cross-check already uses (exFrameReader, defined in
// ex_crosscheck_test.go — the read loop is shared; only the fake's type differs).
func newFTdx101EXFrameReader(t *testing.T, r *fakedx101.Radio) *exFrameReader {
	t.Helper()
	port := r.Port()
	deadliner, ok := port.(interface {
		Read([]byte) (int, error)
		SetReadDeadline(time.Time) error
	})
	if !ok {
		t.Fatalf("fakedx101 Radio.Port() (%T) does not support SetReadDeadline", port)
	}
	return &exFrameReader{
		t:    t,
		port: deadliner,
		acc:  cat.NewFrameAccumulator(0), // DefaultMaxFrame: comfortably above the 21-byte widest EX answer
		buf:  make([]byte, 256),
	}
}

// TestEXFTdx101RoundTrip_AllAddressesRawPort is the wire-level leg: every address
// the DIALECT's inventory declares is asked of a real fakedx101.Radio through its
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
// read), the same design constraint the FT-710's round trip records: 193
// exchanges at the engine's per-exchange Settle would pay seconds of pacing for
// nothing, and Engine's own correlation behaviour is covered by engine_ex_test.go.
func TestEXFTdx101RoundTrip_AllAddressesRawPort(t *testing.T) {
	dialect := ftdx101.DialectD()
	items := dialect.EXItems() // sorted by (P1,P2,P3)
	if len(items) == 0 {
		t.Fatal("ftdx101.DialectD().EXItems() is empty — the dialect's inventory failed to generate, and this test would pass vacuously")
	}

	r := fakedx101.NewD()
	t.Cleanup(func() { _ = r.Close() })
	fakeDefaults := fakedx101.EXDefaults()
	reader := newFTdx101EXFrameReader(t, r)

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
		wantRaw, ok := fakeDefaults[ftdx101.DialectD().EXWire(item.Addr)]
		if !ok {
			t.Errorf("%v: answered over the wire but absent from fakedx101.EXDefaults() — inconsistent with handleEX's own membership check", item.Addr)
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

// TestEXFTdx101RoundTrip_TheTwoModelsAnswerIdentically is this file's own leg,
// with no sibling in either single-model cross-check: the SAME EX read, put to a
// fake FTDX101D and to a fake FTDX101MP, must produce byte-identical answer
// frames for every address in the inventory.
//
// The claim it defends is the one this whole milestone rests on: the manual
// prints Table 2 ONCE for both models, and the capability matrix (§4) records
// that the manual's entire model-distinguishing surface is three places, none of
// them the MENU chart. So core/cat/ftdx101 carries ONE exItems for DialectD and
// DialectMP, core/driver/ftdx101 carries ONE settings descriptor for the pair,
// and internal/fakedx101 seeds both constructors from ONE EXDefaults(). If any of
// that ever stopped being true — a per-model overlay added without a manual
// citation, a shared map accidentally mutated, a second inventory introduced —
// this is where it becomes visible, and the failure names the address.
//
// Frames are compared RAW, before the dialect parses them, so a difference in
// padding or in the echoed address counts as a difference and not merely a
// difference in what the codec chose to extract.
func TestEXFTdx101RoundTrip_TheTwoModelsAnswerIdentically(t *testing.T) {
	dialect := ftdx101.DialectD()
	items := dialect.EXItems()
	if len(items) == 0 {
		t.Fatal("ftdx101.DialectD().EXItems() is empty — this comparison would pass vacuously")
	}

	rD := fakedx101.NewD()
	t.Cleanup(func() { _ = rD.Close() })
	rMP := fakedx101.NewMP()
	t.Cleanup(func() { _ = rMP.Close() })

	readerD := newFTdx101EXFrameReader(t, rD)
	readerMP := newFTdx101EXFrameReader(t, rMP)

	// The control: the two radios DO differ, and this is the one place they
	// may. Without it a pair of dead ports, or two radios accidentally built by
	// the same constructor, would make every comparison below trivially equal.
	for _, probe := range []struct {
		r    *fakedx101.Radio
		rd   *exFrameReader
		want string
	}{
		{rD, readerD, "ID0681;"},
		{rMP, readerMP, "ID0682;"},
	} {
		if _, err := probe.r.Port().Write([]byte("ID;")); err != nil {
			t.Fatalf("Write(ID;): unexpected error: %v", err)
		}
		if got := string(probe.rd.readOneFrame()); got != probe.want {
			t.Fatalf("ID; -> %q, want %q — the two radios under test are not the two models this test claims to compare", got, probe.want)
		}
	}

	compared := 0
	for _, item := range items {
		cmd, err := dialect.BuildEXRead(item.Addr)
		if err != nil {
			t.Fatalf("BuildEXRead(%v): unexpected error: %v", item.Addr, err)
		}
		if _, err := rD.Port().Write(cmd.Bytes()); err != nil {
			t.Fatalf("D Write(%v): unexpected error: %v", item.Addr, err)
		}
		if _, err := rMP.Port().Write(cmd.Bytes()); err != nil {
			t.Fatalf("MP Write(%v): unexpected error: %v", item.Addr, err)
		}
		gotD := string(readerD.readOneFrame())
		gotMP := string(readerMP.readOneFrame())
		if gotD != gotMP {
			t.Errorf("%v (%s / %s): the FTDX101D answered %q and the FTDX101MP answered %q — the two models share ONE MENU chart (matrix §4), so an EX difference is either an invented per-model behaviour or a shared inventory that stopped being shared",
				item.Addr, item.P1Label, item.Name, gotD, gotMP)
		}
		if gotD == "?;" {
			t.Errorf("%v: both models refused an address the dialect's inventory declares", item.Addr)
		}
		compared++
	}
	if compared != len(items) {
		t.Errorf("compared %d of %d inventory addresses", compared, len(items))
	}
}

// TestEXFTdx101RoundTrip_OutOfInventoryAddressIsRefused is the round trip's
// negative control. Without it, a fake that answered EVERY six-digit address with
// something plausible would pass the tests above completely — those loops only
// ask about addresses that ARE in the inventory.
//
// It is run against BOTH models for the same reason the positive sweep is: a
// refusal that held only on the D would be a per-model behaviour with no manual
// behind it.
//
// The addresses below are the fake's own out-of-inventory cases. "050101" is the
// brief's named case, and it carries the FTdx101's own header-vs-chart anomaly
// with it — the EX grammar block states "P1 : 01 - 05" (layout 700) while Table 2
// populates 01-04 only, which core/cat/ftdx101/doc.go records UNRESOLVED because,
// unlike the FT-710's, it cannot be put to hardware. Both this dialect and this
// fake follow the CHART, so both agree 050101 is not a member; what a real
// FTdx101 would answer is doubly unknown, and this test pins the two sides'
// agreement rather than the radio's behaviour. "010199" is a P3 far beyond any
// group's item count and carries no such question.
// Both draw "?;" — ASSUMED, fakedx101 doc.go register entry 17, and assumed twice
// over since these radios' manual never prints the rejection convention at all
// (entry 16).
func TestEXFTdx101RoundTrip_OutOfInventoryAddressIsRefused(t *testing.T) {
	models := []struct {
		name  string
		build func(...fakedx101.Option) *fakedx101.Radio
	}{
		{"FTDX101D", fakedx101.NewD},
		{"FTDX101MP", fakedx101.NewMP},
	}
	for _, m := range models {
		t.Run(m.name, func(t *testing.T) {
			dialect := ftdx101.DialectD()
			r := m.build()
			t.Cleanup(func() { _ = r.Close() })
			reader := newFTdx101EXFrameReader(t, r)

			for _, wire := range []string{"050101", "010199"} {
				t.Run(wire, func(t *testing.T) {
					// The dialect refuses to BUILD a read for an address it does
					// not know, which is itself worth pinning: the two sides
					// agree that these are not members.
					addr := cat.EXAddress{
						P1: uint8(10*(wire[0]-'0') + (wire[1] - '0')),
						P2: uint8(10*(wire[2]-'0') + (wire[3] - '0')),
						P3: uint8(10*(wire[4]-'0') + (wire[5] - '0')),
					}
					if dialect.KnownEXAddress(addr) {
						t.Fatalf("the dialect claims %s is a known address — this negative control is testing the wrong thing", wire)
					}

					// So the frame is written by hand, exactly as the grammar
					// prints it.
					if _, err := r.Port().Write([]byte("EX" + wire + ";")); err != nil {
						t.Fatalf("Write: unexpected error: %v", err)
					}
					if got, want := string(reader.readOneFrame()), "?;"; got != want {
						t.Errorf("EX%s; -> %q, want %q", wire, got, want)
					}
				})
			}
		})
	}
}

// TestFTdx101TranscriptionBCopy_ByteIdenticalToTheDialects pins the
// COPY-NOT-MOVE rule mechanically (internal/fakedx101/PROVENANCE.md).
//
// The fake's transcription-b.csv is a copy: the dialect's own copy does not move,
// because core/cat/ftdx101/crosscheck_test.go:106 keeps reading it as one of the
// THREE artefacts it binds — transcription A, transcription B and the
// group-boundary ledger. The cross-check above is only "dialect from A vs fake
// from B" for as long as the fake's copy really is B; a drifted copy would still
// generate a table, and a table quietly derived from a private variant of B is
// exactly the kind of agreement this milestone is trying not to have.
//
// If the dialect's copy is ever corrected by an arbitration against the PDF, this
// test fails until the fake's copy is re-copied and its inventory regenerated.
// That is the intended workflow, not an obstacle to it.
func TestFTdx101TranscriptionBCopy_ByteIdenticalToTheDialects(t *testing.T) {
	const (
		dialectCopy = "../cat/ftdx101/testdata/transcription-b.csv"
		fakeCopy    = "../../internal/fakedx101/transcription-b.csv"
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
			"Re-copy the dialect's artefact and run `go generate ./internal/fakedx101` — or, if the divergence is a deliberate arbitration, record it in internal/fakedx101/PROVENANCE.md.",
			fakeCopy, dialectCopy, len(got), len(want))
	}
}
