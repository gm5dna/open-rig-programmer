// SPDX-License-Identifier: GPL-3.0-or-later

package transport_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ts590"
	"github.com/gm5dna/open-rig-programmer/internal/fakets590"
)

// This file and its TS-480 sibling are the Kenwood half of the
// two-independent-transcriptions cross-check — ex_crosscheck_ft891_test.go's
// shape, on a family whose EX address is a SINGLE three-digit component and
// whose 590 row needs the check TWICE, over two disjoint charts printed in one
// book:
//
//	the CODEC's inventories (ts590.EXItemsS() and ts590.EXItemsSG()) are
//	generated from TRANSCRIPTION A (core/kw/ts590/menu590s.csv and
//	menu590sg.csv) by internal/extable;
//
//	the FAKE's inventories (fakets590.EXDefaults(row)) are generated from
//	TRANSCRIPTION B (internal/fakets590's own copies) by
//	internal/fakets590/gen, which imports nothing project-internal at all.
//
// A and B are two independent derivations of one printed chart: A
// layout-text-led and PDF-checked, B derived PDF-primary by a quarantined
// agent with no repository access and no sight of A, the page ledger or any
// row count. The two GENERATORS are independent too — that is what the fake's
// recursive no-imports fence enforces, gen/ included. So a mis-read row in
// either transcription, or a defect in either generator, surfaces HERE as a
// mismatch rather than as two tables quietly agreeing on the same wrong
// number.
//
// # Why this is package transport_test and its Yaesu siblings are not
//
// ex_crosscheck_ft891_test.go is `package transport`, because core/cat does
// not import core/transport and so an internal test file may import it.
// core/kw DOES import core/transport — it presents the Framing adapter this
// engine drives — so a `package transport` file importing core/kw would be an
// import cycle. The external test package is the language's own answer to
// exactly that, and it costs this file only the helpers defined in the
// internal one, which it does not need: the fakes here speak core/kw's
// accumulator, not core/cat's.
//
// It lives in core/transport for the reason its siblings do: this is the
// existing test home that already imports both a codec and a fake, which keeps
// the fakes' own test packages core-free. Nothing in this file is imported by
// production code.
//
// # What the two sides are compared ON, and what is deliberately not compared
//
// ADDRESS and WIDTH. Those are the datum: core/kw/ts590/crosscheck_test.go
// already binds A's digits against B's row for row, and this file binds A's
// generated inventory against B's generated projection and then drives every
// address over a wire.
//
// THE text FLAG IS NOT COMPARED, and that is a ruling rather than an omission.
// A and B were briefed with different definitions of "text row" and each
// applied its own consistently, so the flag records a CONVENTION; the one
// address on this pair where they differ is the SG's menu 000, and
// core/kw/ts590/crosscheck_test.go names it, requires the DIGITS to agree
// there, and treats any new divergence as a STOP. The fake's generator
// therefore does not project the flag at all
// (internal/fakets590/gen/main.go's widthToken), so every address it answers
// replies with its width in '0' bytes — and the SHAPE assertion below is
// written against that, with the codec's own text rows named as the deliberate
// state rather than left for a reader to notice.
//
// ON FAILURE: report the exact diff, and do NOT "fix" either table to make it
// pass. Which side is wrong — or whether the printed chart is — is an
// arbitration against the PDF. An edit that merely restores agreement destroys
// the evidence the agreement was worth.
//
// ONE CLASS NO LEG HERE CAN CATCH, stated so that the silence is a recorded
// limit: a defect PRINTED in the chart is read faithfully by both derivations
// and both tables agree on it. The TS-480's menu 034 is one, and it is pinned
// as a deliberate state by core/kw/ts480 and by internal/fakets480, not by
// anything below.

// ts590Rows pairs each registry row's codec inventory with the fake row that
// serves it. Both sides are fetched per call rather than shared: both APIs
// return fresh copies by contract, and a shared fixture would hide it if
// either stopped.
var ts590Rows = []struct {
	name   string
	row    fakets590.Row
	layout func() kw.Layout
	items  func() []kw.EXItem
	// lastMenu is this row's printed menu domain, "000 ~ 087: Menu number
	// (TS-590S)" and "000 ~ 099: Menu number (TS-590SG)" (590:543-544),
	// written as a literal: a bound taken from either table it bounds would
	// prove nothing.
	lastMenu int
	// textRows is the set of addresses the CODEC's inventory marks Text, from
	// the chart: the one "up to 8 ASCII characters" row each list prints —
	// menu 087 on the S and the same string renumbered to 001 on the SG
	// (590:741, 590:750). It is a literal for the same reason.
	textRows []int
}{
	{"TS-590S", fakets590.RowS, ts590.LayoutS, ts590.EXItemsS, 87, []int{87}},
	{"TS-590SG", fakets590.RowSG, ts590.LayoutSG, ts590.EXItemsSG, 99, []int{1}},
}

// exMismatch is one disagreement between the two inventories, rendered for a
// report.
type exMismatch struct {
	addr string
	what string
}

// compareEXInventories is the comparison itself, factored out of the test that
// runs it over the real tables so that the RED PROOFS below can run it over a
// deliberately perturbed one. A comparison that is only ever run on data
// expected to agree has never been shown to be able to disagree.
//
// It reports, for every address in either side:
//
//	membership  the two address sets are equal
//	width       len(fake P5) == item.Digits
//	shape       the fake answers item.Digits x '0'
//
// The shape rule is UNCONDITIONAL, including on the codec's text rows: the
// fake's side cannot see textness (its generator does not project B's flag,
// and the flag is a convention rather than a datum), so answering an invented
// placeholder of the right WIDTH is the strongest honest claim this side can
// make. The text rows are pinned separately, by name.
func compareEXInventories(items []kw.EXItem, fake map[string]string) []exMismatch {
	var out []exMismatch
	seen := map[string]bool{}
	for _, item := range items {
		addr := item.Addr.Wire()
		seen[addr] = true
		p5, ok := fake[addr]
		if !ok {
			out = append(out, exMismatch{addr, fmt.Sprintf("in the codec's inventory (%q) and NOT in the fake's", item.Name)})
			continue
		}
		if len(p5) != item.Digits {
			out = append(out, exMismatch{addr, fmt.Sprintf("codec Digits=%d (%q), fake default is %d bytes (%q)", item.Digits, item.Name, len(p5), p5)})
			continue
		}
		if strings.Trim(p5, "0") != "" {
			out = append(out, exMismatch{addr, fmt.Sprintf("the fake's default is %q, want %d x '0' (the invented placeholder convention)", p5, item.Digits)})
		}
	}
	for addr := range fake {
		if !seen[addr] {
			out = append(out, exMismatch{addr, "in the fake's inventory and NOT in the codec's"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].addr < out[j].addr })
	return out
}

// renderMismatches formats a comparison result for a failure message.
func renderMismatches(ms []exMismatch) string {
	var b strings.Builder
	for _, m := range ms {
		fmt.Fprintf(&b, "  menu %s: %s\n", m.addr, m.what)
	}
	return b.String()
}

// TestEXInventoryCrossCheck_TS590AddressesAndWidthsAgree is the table-level
// leg, run on both rows.
func TestEXInventoryCrossCheck_TS590AddressesAndWidthsAgree(t *testing.T) {
	for _, tt := range ts590Rows {
		t.Run(tt.name, func(t *testing.T) {
			items, fake := tt.items(), fakets590.EXDefaults(tt.row)

			// Vacuity guard: two empty sets are trivially equal, and an
			// inventory that failed to generate would be exactly that.
			if len(items) == 0 || len(fake) == 0 {
				t.Fatalf("empty inventory: the codec has %d addresses, the fake has %d — one side failed to generate, and the comparison below would pass vacuously", len(items), len(fake))
			}
			if want := tt.lastMenu + 1; len(items) != want || len(fake) != want {
				t.Errorf("the codec has %d addresses and the fake %d; this row's printed domain is 000 ~ %03d, which is %d menus (590:543-544)", len(items), len(fake), tt.lastMenu, want)
			}

			if ms := compareEXInventories(items, fake); len(ms) > 0 {
				t.Errorf("%s EX inventories disagree between the two independent transcriptions of this row's parameter list — %d addresses:\n%s"+
					"This is a genuine cross-check finding, not a bug in this test: do not modify either table to make it pass — report the diff and arbitrate against the PDF.",
					tt.name, len(ms), renderMismatches(ms))
			}
		})
	}
}

// TestEXInventoryCrossCheck_TS590RedProofs runs the same comparison over
// DELIBERATELY perturbed copies of the fake's side, so that the green result
// above is known to be a result rather than a function that cannot fail.
//
// The two perturbations are the two defect classes the plan names: a dropped
// row, and a width-only change. The fake's generator refuses the first at
// projection (internal/fakets590/gen's projectWidths) and cannot see the
// second at all, which is exactly why the second has to be caught here.
func TestEXInventoryCrossCheck_TS590RedProofs(t *testing.T) {
	items := ts590.EXItemsSG()

	t.Run("a dropped address", func(t *testing.T) {
		fake := fakets590.EXDefaults(fakets590.RowSG)
		delete(fake, "043")
		ms := compareEXInventories(items, fake)
		if len(ms) != 1 || ms[0].addr != "043" {
			t.Errorf("dropping menu 043 from the fake's side reported %v, want exactly one mismatch at 043", ms)
		}
	})

	t.Run("a width-only perturbation", func(t *testing.T) {
		fake := fakets590.EXDefaults(fakets590.RowSG)
		if got := fake["043"]; got != "0" {
			t.Fatalf("menu 043's fake default is %q, and this proof assumes one byte", got)
		}
		fake["043"] = "00"
		ms := compareEXInventories(items, fake)
		if len(ms) != 1 || ms[0].addr != "043" {
			t.Errorf("widening menu 043's fake default to two bytes reported %v, want exactly one mismatch at 043", ms)
		}
	})

	// And the control: the comparison is not simply returning something for
	// everything.
	if ms := compareEXInventories(items, fakets590.EXDefaults(fakets590.RowSG)); len(ms) != 0 {
		t.Errorf("the unperturbed comparison reported %d mismatches — the proofs above prove nothing", len(ms))
	}
}

// TestEXInventoryCrossCheck_TS590TextRowsAreTheRecordedOnes pins the one thing
// the shape rule above deliberately does not assert: WHICH addresses the
// codec's side marks as text.
//
// The fake answers an invented placeholder of the right width everywhere,
// including at these two, and that is the recorded state rather than a defect:
// transcription B's text column is a CONVENTION the two legs disagree about
// (core/kw/ts590/crosscheck_test.go's ruling), so the fake's generator does not
// project it. What must not happen silently is transcription A acquiring a NEW
// text row — because that would be a change to the chart's reading that this
// file's shape rule would absorb without comment.
func TestEXInventoryCrossCheck_TS590TextRowsAreTheRecordedOnes(t *testing.T) {
	for _, tt := range ts590Rows {
		t.Run(tt.name, func(t *testing.T) {
			var got []int
			for _, item := range tt.items() {
				if item.Text {
					got = append(got, int(item.Addr.P1))
				}
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.textRows) {
				t.Errorf("the codec's inventory marks text rows %v, the chart prints %v (the one \"up to 8 ASCII characters\" row, 590:741, 590:750) — a new text row is a finding to arbitrate, not a pin to widen", got, tt.textRows)
			}
		})
	}
}

// --- The wire leg ---

// kwFrameReader reads whole ';'-terminated frames off a fake's port through
// core/kw's OWN accumulator — the one a Kenwood host uses — rather than
// core/cat's, so this leg exercises the same bound the driver will meet.
type kwFrameReader struct {
	t    *testing.T
	port interface {
		Read([]byte) (int, error)
		SetReadDeadline(time.Time) error
	}
	acc *kw.FrameAccumulator
	buf []byte
}

func newKWFrameReader(t *testing.T, port io.ReadWriteCloser) *kwFrameReader {
	t.Helper()
	deadliner, ok := port.(interface {
		Read([]byte) (int, error)
		SetReadDeadline(time.Time) error
	})
	if !ok {
		t.Fatalf("the fake's Port() (%T) does not support SetReadDeadline", port)
	}
	return &kwFrameReader{t: t, port: deadliner, acc: kw.NewFrameAccumulator(0), buf: make([]byte, 256)}
}

func (r *kwFrameReader) readOneFrame() []byte {
	r.t.Helper()
	for {
		if err := r.port.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			r.t.Fatalf("SetReadDeadline: unexpected error: %v", err)
		}
		n, rerr := r.port.Read(r.buf)
		if n > 0 {
			frames, ferr := r.acc.Push(r.buf[:n])
			if ferr != nil {
				r.t.Fatalf("frame accumulator error: %v", ferr)
			}
			if len(frames) > 0 {
				return frames[0]
			}
		}
		if rerr != nil {
			r.t.Fatalf("Read: unexpected error waiting for a frame: %v", rerr)
		}
	}
}

// TestEXTS590RoundTrip_AllAddressesRawPort is the wire-level leg: every
// address the CODEC's inventory declares is asked of a real fakets590.Radio
// through its Port(), and each answer is parsed back with the codec's own
// ParseEXAnswer against that inventory row.
//
// It is the strong form of "every address in the codec's inventory is answered
// by the fake": the table test above compares two data structures, while this
// one puts frames on a wire, through the fake's own parser, and decodes the
// replies with the codec. A projection that produced the right table but
// answered the wrong bytes — a handler bug, an off-by-one in the answer
// builder, a P2/P3/P4 in the wrong position — fails only here.
//
// IT ALSO EXERCISES BOTH SIDES OF THE ROW SPLIT. The two rows' domains differ
// by twelve addresses and their inventories are one identifier apart, so a
// codec or a fake that had crossed the pair answers a real menu with the wrong
// width somewhere in this sweep.
//
// Engine.Do is deliberately bypassed (direct Port() write plus an accumulator
// read), the design constraint the FT-710's own round trip records: 188
// exchanges at the engine's per-exchange Settle would pay seconds of pacing
// for nothing, and Engine's correlation behaviour is covered by
// engine_ex_test.go.
func TestEXTS590RoundTrip_AllAddressesRawPort(t *testing.T) {
	for _, tt := range ts590Rows {
		t.Run(tt.name, func(t *testing.T) {
			layout, items := tt.layout(), tt.items()
			if len(items) == 0 {
				t.Fatal("the codec's inventory is empty — this test would pass vacuously")
			}

			r := fakets590.New(tt.row)
			t.Cleanup(func() { _ = r.Close() })
			fake := fakets590.EXDefaults(tt.row)
			reader := newKWFrameReader(t, r.Port())

			answered := 0
			for _, item := range items {
				cmd, err := layout.BuildEXRead(item.Addr)
				if err != nil {
					t.Fatalf("BuildEXRead(%v): unexpected error: %v", item.Addr, err)
				}
				if got := len(cmd.Bytes()); got != kw.EXReadLen {
					t.Fatalf("BuildEXRead(%v) built a %d-byte frame (%q), want %d (590:552)", item.Addr, got, cmd.Bytes(), kw.EXReadLen)
				}
				if _, err := r.Port().Write(cmd.Bytes()); err != nil {
					t.Fatalf("Write(%v %q): unexpected error: %v", item.Addr, cmd.Bytes(), err)
				}

				frame := reader.readOneFrame()
				gotP5, err := layout.ParseEXAnswer(frame, item)
				if err != nil {
					t.Fatalf("ParseEXAnswer(%q) for %v: unexpected error: %v — the fake refused or malformed an address the codec's inventory declares", frame, item.Addr, err)
				}
				if len(gotP5) != item.Digits {
					t.Errorf("%v (%s): answered %d raw P5 bytes (%q), want %d per the codec's inventory", item.Addr, item.Name, len(gotP5), gotP5, item.Digits)
				}
				if want := fake[item.Addr.Wire()]; gotP5 != want {
					t.Errorf("%v: raw P5 = %q, want %q (the fake's own default)", item.Addr, gotP5, want)
				}
				answered++
			}
			if answered != len(items) {
				t.Errorf("%d of %d inventory addresses answered as expected", answered, len(items))
			}
		})
	}
}

// TestEXTS590RoundTrip_OutOfDomainAddressIsRefused is the round trip's
// negative control. Without it, a fake that answered EVERY three-digit address
// with something plausible would pass the sweep above completely — the loop
// only asks about addresses that ARE in the inventory.
//
// THE S's 088 IS THE PAIR'S OWN CASE and has no counterpart in any Yaesu file:
// it is a REAL menu on the SG and one past the end of the S's printed domain
// (590:543-544). The codec REFUSES TO BUILD it for the S, which is itself
// worth pinning — the two sides agree it is not a member — so the frame is
// written by hand, exactly as the grammar prints it.
func TestEXTS590RoundTrip_OutOfDomainAddressIsRefused(t *testing.T) {
	for _, tt := range ts590Rows {
		t.Run(tt.name, func(t *testing.T) {
			layout := tt.layout()
			r := fakets590.New(tt.row)
			t.Cleanup(func() { _ = r.Close() })
			reader := newKWFrameReader(t, r.Port())

			beyond := kw.EXAddress{P1: uint8(tt.lastMenu + 1)}
			if _, err := layout.BuildEXRead(beyond); err == nil {
				t.Fatalf("the codec built a read for menu %v, which is outside this row's printed domain — this negative control is testing the wrong thing", beyond)
			}
			wire := fmt.Sprintf("EX%03d0000;", tt.lastMenu+1)
			if _, err := r.Port().Write([]byte(wire)); err != nil {
				t.Fatalf("Write: unexpected error: %v", err)
			}
			if got, want := string(reader.readOneFrame()), "?;"; got != want {
				t.Errorf("%s -> %q, want %q (one past this row's printed domain)", wire, got, want)
			}
		})
	}

	t.Run("a sibling's EX read frame", func(t *testing.T) {
		r := fakets590.New(fakets590.RowSG)
		t.Cleanup(func() { _ = r.Close() })
		reader := newKWFrameReader(t, r.Port())
		for _, wire := range []string{"EX0101;", "EX010100;"} {
			if _, err := r.Port().Write([]byte(wire)); err != nil {
				t.Fatalf("Write: unexpected error: %v", err)
			}
			if got, want := string(reader.readOneFrame()), "?;"; got != want {
				t.Errorf("%s -> %q, want %q — a TS-590 must not answer a Yaesu EX read frame, whatever its digits name", wire, got, want)
			}
		}
	})
}

// TestTS590TranscriptionBCopies_ByteIdenticalToTheCodecs pins the
// COPY-NOT-MOVE rule mechanically (internal/fakets590/PROVENANCE.md).
//
// The fake's copies are copies: the codec's own artefacts do not move, because
// core/kw/ts590/crosscheck_test.go keeps reading them as artefacts it binds
// and hashes by name. The cross-check above is only "codec from A versus fake
// from B" for as long as the fake's copies really are B: a drifted copy would
// still generate a table, and a table quietly derived from a private variant
// of B is exactly the kind of agreement this milestone is trying not to have.
//
// If a codec-side artefact is ever corrected by an arbitration against the
// PDF, this test fails until the fake's copy is re-copied and its inventory
// regenerated. That is the intended workflow, not an obstacle to it.
func TestTS590TranscriptionBCopies_ByteIdenticalToTheCodecs(t *testing.T) {
	for _, tt := range []struct{ codecCopy, fakeCopy string }{
		{"../kw/ts590/testdata/transcription-b-590s.csv", "../../internal/fakets590/transcription-b-590s.csv"},
		{"../kw/ts590/testdata/transcription-b-590sg.csv", "../../internal/fakets590/transcription-b-590sg.csv"},
	} {
		t.Run(tt.fakeCopy, func(t *testing.T) {
			assertByteIdentical(t, tt.codecCopy, tt.fakeCopy, "./internal/fakets590")
		})
	}
}

// assertByteIdentical is the copy-not-move comparison, shared with the TS-480
// file beside this one.
func assertByteIdentical(t *testing.T, codecCopy, fakeCopy, generatePkg string) {
	t.Helper()
	want, err := os.ReadFile(codecCopy)
	if err != nil {
		t.Fatalf("reading %s: %v", codecCopy, err)
	}
	got, err := os.ReadFile(fakeCopy)
	if err != nil {
		t.Fatalf("reading %s: %v", fakeCopy, err)
	}
	if len(want) == 0 {
		t.Fatalf("%s is empty — this comparison would pass vacuously", codecCopy)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s is not byte-identical to %s (%d bytes vs %d): the fake's EX inventory is generated from its copy, so a drifted copy silently stops being transcription B.\n"+
			"Re-copy the codec's artefact and run `go generate %s` — or, if the divergence is a deliberate arbitration, record it in the fake's PROVENANCE.md.",
			fakeCopy, codecCopy, len(got), len(want), generatePkg)
	}
}
