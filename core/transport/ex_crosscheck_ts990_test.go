// SPDX-License-Identifier: GPL-3.0-or-later

package transport_test

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
	"github.com/gm5dna/open-rig-programmer/internal/fakets990"
)

// The TS-990S's half of the two-independent-transcriptions cross-check. The
// file comment on ex_crosscheck_ts590_test.go states the design in full, why
// these files are package transport_test, and what is deliberately not
// compared; the frame reader (newKWFrameReader), the mismatch renderer and the
// copy-not-move assertion (assertByteIdentical) live there and are reused here,
// as is ex_crosscheck_ts890_test.go's compareMAEXInventories, which is this
// FAMILY's comparison and is keyed by the layout's own five-character address.
//
//	the CODEC's inventory (ma.EXItems990S) is generated from TRANSCRIPTION A
//	(core/kw/ma/menu990s.csv) by internal/extable;
//
//	the FAKE's inventory (fakets990.EXDefaults) is projected at init from its
//	own embedded copy of TRANSCRIPTION B by internal/fakets990/exinventory.go,
//	which imports nothing project-internal at all.
//
// # WHAT IS DIFFERENT ON THIS ROW, AND IT IS NOT COSMETIC
//
// THE TWO SIDES ARE THE SAME LENGTH AS TRANSCRIPTION B. The 890S's chart prints
// four Advanced Menu rows with real addresses and the body "Does not correspond
// to a command" (890:2273-2280, erratum E16), which the production inventory
// excludes by address and that fake's projection excludes again from its own
// list. THIS chart prints no such row: all 194 of B's addressed rows carry a
// width, internal/extable's ts990s profile is ParameterlessRefused, and neither
// side removes anything.
// TestEXInventoryCrossCheck_TS990CarriesEveryRowTranscriptionBPrints binds that
// in both directions, because "no exclusions" is exactly the kind of fact that
// is true until someone copies the sibling's list across.
//
// THE FAKE'S PROJECTION APPLIES RULING R-B, AND OVER EIGHTEEN ROWS. Both legs
// read this chart's PF key rows differently: A carries four digits and B
// carries three, and the EX block's own P5 note settles it — "PF key settings
// use 4 digits (refer to the PF Key assignment ID lists)." (990:1747-1748),
// with the PF Key Assignment Lists themselves (990:2287 onwards) printing every
// allotment ID as four digits. The leg is frozen evidence, so the correction is
// applied in the fake's PROJECTION and never in the CSV. The fake reads the
// sentence from ITS OWN book and consults neither A nor internal/extable, so
// the two sides still meet here as two derivations.
// TestEXInventoryCrossCheck_TS990PFKeyWidthsAgreeUnderRulingRB pins that the
// correction is doing work rather than agreeing with what was already there.
// It is the SAME CLASS AS THE 890S'S FOUR EXCLUDED ADDRESSES: a fact this book
// prints that the leg does not carry, applied at that side from an
// independently written list.
//
// THE RUN IS EIGHTEEN AND NOT SEVENTEEN. This radio inserts Voice (Main Band)
// and Voice (Sub Band) at items 17 and 18, so its PF rows are 0/00/15 to
// 0/00/32 (990:1793-1810) where the 890S's are 0/00/15 to 0/00/31. The
// addresses below are spelt from this chart's own run and not from the
// sibling's.
//
// THE ANSWER FRAME IS FIXED AT TWENTY-FOUR BYTES. This chart's Answer diagram
// holds P5 to fifteen cells and nails ';' to position 24 (990:1738-1747) where
// the 890S's terminator floats under a ruler head printed "x" — erratum E19 —
// so the wire leg below reads a padded P5 and takes the row's own printed width
// off the front of it, which is the obligation ma.ParseEXAnswer documents for
// this form.
//
// ON FAILURE: report the exact diff, and do NOT "fix" either table to make it
// pass. Which side is wrong — or whether the printed chart is — is an
// arbitration against the PDF. An edit that merely restores agreement destroys
// the evidence the agreement was worth.

// The two artefacts this file reads by path. The codec's copy is the one
// core/kw/ma/crosscheck_test.go binds and hashes by name; the fake's is the
// byte copy its projection embeds.
const (
	codecBCopy990 = "../kw/ma/testdata/transcription-b-990s.csv"
	fakeBCopy990  = "../../internal/fakets990/transcription-b-990s.csv"
)

// pfKeyAddresses990 is ruling R-B's run, spelt from the chart's own rows
// (990:1793-1810) and independently of both implementations: PF A and PF B,
// Voice (Main Band) and Voice (Sub Band) — the two the 890S has not — External
// PF 1-8, Microphone PF 1-4, Microphone Down and Microphone Up.
func pfKeyAddresses990() []string {
	var out []string
	for item := 15; item <= 32; item++ {
		out = append(out, fmt.Sprintf("000%02d", item))
	}
	return out
}

// TestEXInventoryCrossCheck_TS990AddressesAndWidthsAgree is the table-level
// leg.
func TestEXInventoryCrossCheck_TS990AddressesAndWidthsAgree(t *testing.T) {
	layout, items, fake := ma.Layout990(), ma.EXItems990S(), fakets990.EXDefaults()

	// Vacuity guard: two empty sets are trivially equal, and an inventory
	// that failed to generate would be exactly that.
	if len(items) == 0 || len(fake) == 0 {
		t.Fatalf("empty inventory: the codec has %d addresses, the fake has %d — one side failed to generate, and the comparison below would pass vacuously", len(items), len(fake))
	}

	if ms := compareMAEXInventories(layout, items, fake); len(ms) > 0 {
		t.Errorf("TS-990S EX inventories disagree between the two independent transcriptions of this chart — %d addresses:\n%s"+
			"This is a genuine cross-check finding, not a bug in this test: do not modify either table to make it pass — report the diff and arbitrate against the PDF.",
			len(ms), renderMismatches(ms))
	}
}

// TestEXInventoryCrossCheck_TS990CarriesEveryRowTranscriptionBPrints is this
// row's counterpart to the 890S file's four-excluded-addresses test, and it
// asserts the OPPOSITE fact: that nothing is excluded on either side.
//
// It binds three things that only mean anything together:
//
//   - EVERY ADDRESS TRANSCRIPTION B CARRIES IS IN BOTH INVENTORIES. Neither the
//     generator nor the fake's projection removes a row of this chart.
//   - NEITHER INVENTORY CARRIES AN ADDRESS B DOES NOT. A row invented on one
//     side would otherwise show up only as a count.
//   - THE ARITHMETIC, taken from B's own row count rather than written as a
//     figure, so no number appears here to be carried forward stale — and it
//     binds A's count to B's count, which two independently derived tables of
//     one chart owe each other.
//
// If a later reader ever copies the 890S's ParameterlessExcluded list onto this
// row — the sibling's chart has four such rows and this one has none — this is
// the test that says so, rather than the comparison above reporting four
// addresses "missing from the fake" and pointing at the wrong document.
func TestEXInventoryCrossCheck_TS990CarriesEveryRowTranscriptionBPrints(t *testing.T) {
	bAddrs := transcriptionBAddresses990(t)
	layout, items, fake := ma.Layout990(), ma.EXItems990S(), fakets990.EXDefaults()

	codec := map[string]bool{}
	for _, item := range items {
		codec[layout.WireEXAddress(item.Addr)] = true
	}

	for addr := range bAddrs {
		if !codec[addr] {
			t.Errorf("the CODEC's inventory does not carry %s, which transcription B prints — this chart has no parameterless row for an exclusion to remove", addr)
		}
		if _, ok := fake[addr]; !ok {
			t.Errorf("the FAKE's projection does not carry %s, which transcription B prints — this chart has no parameterless row for an exclusion to remove", addr)
		}
	}
	for addr := range fake {
		if !bAddrs[addr] {
			t.Errorf("the FAKE's projection carries %s, which transcription B does not print", addr)
		}
	}
	for addr := range codec {
		if !bAddrs[addr] {
			t.Errorf("the CODEC's inventory carries %s, which transcription B does not print", addr)
		}
	}

	if want := len(bAddrs); len(items) != want || len(fake) != want {
		t.Errorf("transcription B has %d addressed rows and this chart excludes none, so each inventory should have %d; the codec has %d and the fake %d",
			len(bAddrs), want, len(items), len(fake))
	}
}

// transcriptionBAddresses990 reads the CODEC-side copy of transcription B and
// returns its addresses in the five-character wire form.
//
// It reads the codec's copy deliberately: the fake's is asserted byte-identical
// to it below, so using the original here means this test's B is the artefact
// core/kw/ma binds and hashes rather than a copy that might have drifted.
func transcriptionBAddresses990(t *testing.T) map[string]bool {
	t.Helper()
	records := transcriptionBRecords990(t)
	out := map[string]bool{}
	for _, rec := range records {
		addr := rec[0] + rec[1] + rec[2]
		if len(addr) != 5 {
			t.Fatalf("%s: address cells %q/%q/%q do not make a five-character wire address", codecBCopy990, rec[0], rec[1], rec[2])
		}
		if out[addr] {
			t.Fatalf("%s carries address %s twice", codecBCopy990, addr)
		}
		out[addr] = true
	}
	return out
}

// transcriptionBRecords990 reads the codec-side copy of transcription B and
// returns its DATA rows, header checked.
func transcriptionBRecords990(t *testing.T) [][]string {
	t.Helper()
	data, err := os.ReadFile(codecBCopy990)
	if err != nil {
		t.Fatalf("reading %s: %v", codecBCopy990, err)
	}
	records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		t.Fatalf("parsing %s: %v", codecBCopy990, err)
	}
	if len(records) < 2 {
		t.Fatalf("%s has %d records — this test would pass vacuously", codecBCopy990, len(records))
	}
	if got, want := strings.Join(records[0], ","), "p1,p2,p3,name,digits,text"; got != want {
		t.Fatalf("%s's header is %q, want %q", codecBCopy990, got, want)
	}
	return records[1:]
}

// transcriptionBWidths990 reads the codec-side copy of transcription B and
// returns its digits cells, keyed by wire address.
func transcriptionBWidths990(t *testing.T) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, rec := range transcriptionBRecords990(t) {
		n, err := strconv.Atoi(strings.TrimSpace(rec[4]))
		if err != nil {
			t.Fatalf("%s: digits cell %q on row %v: %v", codecBCopy990, rec[4], rec[:3], err)
		}
		out[rec[0]+rec[1]+rec[2]] = n
	}
	if len(out) == 0 {
		t.Fatalf("%s yielded no widths — this test would pass vacuously", codecBCopy990)
	}
	return out
}

// TestEXInventoryCrossCheck_TS990PFKeyWidthsAgreeUnderRulingRB is this file's
// other row-specific fact, and it is the T15 review's ruling 1 applied
// SYMMETRICALLY to this radio.
//
// Transcription B reads THREE digits on all eighteen PF key rows and
// transcription A reads FOUR; this book's own P5 note settles it — "PF key
// settings use 4 digits (refer to the PF Key assignment ID lists)."
// (990:1747-1748) — and its PF Key Assignment Lists print four-digit allotment
// IDs (990:2287 onwards), so the book settles the width twice. The leg is
// frozen evidence, so internal/fakets990's projection corrects those eighteen
// widths from the printed sentence rather than the CSV being edited.
//
// WITHOUT THIS TEST THE CORRECTION WOULD BE INVISIBLE. The comparison above
// would pass equally well if the fake had simply been given A's numbers, or if
// the leg were one day "tidied" to read four — and in either case the two sides
// would agree for a reason that is not two derivations meeting. So this asserts
// all three facts at once: both sides carry four, the LEG carries three, and
// the eighteen are exactly the rows the ruling names.
func TestEXInventoryCrossCheck_TS990PFKeyWidthsAgreeUnderRulingRB(t *testing.T) {
	layout, fake := ma.Layout990(), fakets990.EXDefaults()
	pf := pfKeyAddresses990()
	if len(pf) != 18 {
		t.Fatalf("this test names %d PF addresses, want the eighteen rows 0/00/15 … 0/00/32 (990:1793-1810) — the 890S's run is seventeen and must not be copied here", len(pf))
	}

	widths := map[string]int{}
	for _, item := range ma.EXItems990S() {
		widths[layout.WireEXAddress(item.Addr)] = item.Digits
	}
	legWidths := transcriptionBWidths990(t)

	for _, addr := range pf {
		if got, ok := widths[addr]; !ok || got != 4 {
			t.Errorf("the codec's inventory has %s at width %d (present=%v), want 4 (990:1747-1748)", addr, got, ok)
		}
		if got := fake[addr]; len(got) != 4 {
			t.Errorf("the fake answers %s with %q (%d bytes), want 4 — its projection must apply ruling R-B", addr, got, len(got))
		}
		if got := legWidths[addr]; got != 3 {
			t.Errorf("transcription B carries width %d at %s; ruling R-B records 3 there, and a leg that has moved is an arbitration rather than a correction to apply blind", got, addr)
		}
	}

	// The correction touches those eighteen and nothing else: every other
	// address the fake answers is its leg's own width. That covers the two
	// OTHER rulings this chart carries — the twenty-nine text-flag
	// disagreements and the 0/03/01 name — which touch neither digits nor
	// width, and would be caught here if either ever did.
	for addr, p5 := range fake {
		if slices.Contains(pf, addr) {
			continue
		}
		if want := legWidths[addr]; len(p5) != want {
			t.Errorf("menu %s: the fake answers %d bytes where transcription B reads %d — the only correction this projection applies is ruling R-B's eighteen PF rows", addr, len(p5), want)
		}
	}
}

// TestEXInventoryCrossCheck_TS990RedProofs runs the same comparison over
// DELIBERATELY perturbed copies of the fake's side, so that the green result
// above is known to be a result rather than a function that cannot fail.
//
// The two perturbations are the two defect classes the plan names: a dropped
// row, and a width-only change. The fake's projection refuses a DUPLICATE
// address at projection time and cannot see a width change at all, which is
// exactly why the second has to be caught here.
func TestEXInventoryCrossCheck_TS990RedProofs(t *testing.T) {
	layout, items := ma.Layout990(), ma.EXItems990S()
	const probe = "00300" // 0/03/00 — a three-digit row of this chart

	t.Run("a dropped address", func(t *testing.T) {
		fake := fakets990.EXDefaults()
		if _, ok := fake[probe]; !ok {
			t.Fatalf("menu %s is not in the fake's projection — this proof needs a real address", probe)
		}
		delete(fake, probe)
		ms := compareMAEXInventories(layout, items, fake)
		if len(ms) != 1 || ms[0].addr != probe {
			t.Errorf("dropping menu %s from the fake's side reported %v, want exactly one mismatch at %s", probe, ms, probe)
		}
	})

	t.Run("a width-only perturbation", func(t *testing.T) {
		fake := fakets990.EXDefaults()
		if got := fake[probe]; got != "000" {
			t.Fatalf("menu %s's fake default is %q, and this proof assumes three bytes", probe, got)
		}
		fake[probe] = "0000"
		ms := compareMAEXInventories(layout, items, fake)
		if len(ms) != 1 || ms[0].addr != probe {
			t.Errorf("widening menu %s's fake default to four bytes reported %v, want exactly one mismatch at %s", probe, ms, probe)
		}
	})

	t.Run("a PF row left at the leg's width", func(t *testing.T) {
		// What dropping ruling R-B at ONE address would look like here. The
		// projection applies it to all eighteen or refuses to start, so this
		// is the only level at which the single-address case can be shown.
		fake := fakets990.EXDefaults()
		const pf = "00017" // Voice (Main Band) — one of the two rows the 890S has not
		if got := fake[pf]; len(got) != 4 {
			t.Fatalf("menu %s's fake default is %q, and this proof assumes ruling R-B's four bytes", pf, got)
		}
		fake[pf] = "000"
		ms := compareMAEXInventories(layout, items, fake)
		if len(ms) != 1 || ms[0].addr != pf {
			t.Errorf("returning menu %s to the leg's three bytes reported %v, want exactly one mismatch at %s", pf, ms, pf)
		}
	})

	// And the control: the comparison is not simply returning something for
	// everything.
	if ms := compareMAEXInventories(layout, items, fakets990.EXDefaults()); len(ms) != 0 {
		t.Errorf("the unperturbed comparison reported %d mismatches — the proofs above prove nothing", len(ms))
	}
}

// TestEXInventoryCrossCheck_TS990TextRowsAreTheRecordedOnes pins the one thing
// the shape rule deliberately does not assert: WHICH addresses the codec's side
// marks as text.
//
// The fake answers an invented placeholder of the right width everywhere, and
// that is the recorded state rather than a defect: transcription B's text
// column is a CONVENTION the two legs disagree about, and on this chart they
// differ at TWENTY-NINE addresses — the 28 Fixed Mode band-limit rows and
// Contest Number, all flagged text by B — where the ruling is that only a row
// reading "Up to N alphanumeric characters" is a text row. What must not happen
// silently is transcription A acquiring a NEW text row, because that would be a
// change to the chart's reading that this file's shape rule would absorb
// without comment.
func TestEXInventoryCrossCheck_TS990TextRowsAreTheRecordedOnes(t *testing.T) {
	layout := ma.Layout990()
	// The two rows the chart prints as free text, written as literals: a set
	// taken from the table it bounds would prove nothing. They are the Screen
	// Saver Message ("Up to 10 alphanumeric characters", 990:1778) and the
	// Power-on Message ("Up to 15 alphanumeric characters", 990:1779). THE
	// 890S'S TWO ARE AT 0/00/05 AND 0/00/06; this chart's are one row later,
	// which is what a list copied across would get wrong.
	want := []string{"00006", "00007"}

	var got []string
	for _, item := range ma.EXItems990S() {
		if item.Text {
			got = append(got, layout.WireEXAddress(item.Addr))
		}
	}
	sort.Strings(got)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the codec's inventory marks text rows %v, the chart prints %v (990:1778-1779) — a new text row is a finding to arbitrate, not a pin to widen", got, want)
	}
}

// --- The wire leg ---

// TestEXTS990RoundTrip_AllAddressesRawPort is the wire-level leg: every address
// the CODEC's inventory declares is asked of a real fakets990.Radio through its
// Port(), and each answer is parsed back with the codec's own ParseEXAnswer
// against that inventory row.
//
// It is the strong form of "every address in the codec's inventory is answered
// by the fake": the table test above compares two data structures, while this
// one puts frames on a wire, through the fake's own parser, and decodes the
// replies with the codec. A projection that produced the right table but
// answered the wrong bytes — a handler bug, an off-by-one in the answer
// builder, a P4 in the wrong position — fails only here.
//
// AND ON THIS ROW IT ALSO EXERCISES ERRATUM E19. The read is EIGHT bytes and
// carries NO P4 (990:1734-1736); the ANSWER is drawn to a fixed twenty-four,
// with P5 filling a fifteen-wide window and ';' at position 24
// (990:1738-1747). The fake prints that form, so ParseEXAnswer returns the
// whole window and THE CALLER TAKES THE ROW'S OWN item.Digits OFF THE FRONT —
// the obligation that function documents for this form, discharged here rather
// than assumed. Two sides that had read those two grids differently would agree
// on every width and still fail here.
//
// Engine.Do is deliberately bypassed (direct Port() write plus an accumulator
// read), the design constraint the FT-710's own round trip records: paying the
// engine's per-exchange Settle for every address in a chart this size would buy
// seconds of pacing for nothing, and Engine's correlation behaviour is covered
// by engine_ex_test.go.
func TestEXTS990RoundTrip_AllAddressesRawPort(t *testing.T) {
	layout, items := ma.Layout990(), ma.EXItems990S()
	if len(items) == 0 {
		t.Fatal("the codec's inventory is empty — this test would pass vacuously")
	}

	// The fifteen-wide P5 window the Answer diagram draws (990:1738-1747),
	// spelt here rather than taken from either side.
	const p5Window = 15

	r := fakets990.New()
	t.Cleanup(func() { _ = r.Close() })
	fake := fakets990.EXDefaults()
	reader := newKWFrameReader(t, r.Port())

	answered := 0
	for _, item := range items {
		cmd, err := layout.BuildEXRead(item.Addr)
		if err != nil {
			t.Fatalf("BuildEXRead(%v): unexpected error: %v", item.Addr, err)
		}
		if got := len(cmd.Bytes()); got != ma.EXReadLen {
			t.Fatalf("BuildEXRead(%v) built a %d-byte frame (%q), want %d (990:1734-1736)", item.Addr, got, cmd.Bytes(), ma.EXReadLen)
		}
		if _, err := r.Port().Write(cmd.Bytes()); err != nil {
			t.Fatalf("Write(%v %q): unexpected error: %v", item.Addr, cmd.Bytes(), err)
		}

		frame := reader.readOneFrame()
		if len(frame) != 24 {
			t.Fatalf("%v: the fake answered %d bytes (%q), want the twenty-four its own Answer diagram draws (990:1738-1747)", item.Addr, len(frame), frame)
		}
		gotP5, err := layout.ParseEXAnswer(frame, item)
		if err != nil {
			t.Fatalf("ParseEXAnswer(%q) for %v: unexpected error: %v — the fake refused or malformed an address the codec's inventory declares", frame, item.Addr, err)
		}
		if len(gotP5) != p5Window {
			t.Errorf("%v (%s): ParseEXAnswer returned %d bytes (%q), want the whole %d-wide window verbatim", item.Addr, item.Name, len(gotP5), gotP5, p5Window)
			continue
		}
		want := fake[layout.WireEXAddress(item.Addr)]
		if len(want) != item.Digits {
			t.Errorf("%v (%s): the fake's own default is %d bytes (%q) where the codec's inventory says %d", item.Addr, item.Name, len(want), want, item.Digits)
			continue
		}
		if got := gotP5[:item.Digits]; got != want {
			t.Errorf("%v: the answer's first %d P5 bytes are %q, want the fake's own default %q", item.Addr, item.Digits, got, want)
		}
		answered++
	}
	if answered != len(items) {
		t.Errorf("%d of %d inventory addresses answered as expected", answered, len(items))
	}
}

// TestEXTS990RoundTrip_RefusedAddresses is the round trip's negative control.
// Without it, a fake that answered EVERY five-digit address with something
// plausible would pass the sweep above completely — the loop only asks about
// addresses that ARE in the inventory.
func TestEXTS990RoundTrip_RefusedAddresses(t *testing.T) {
	layout := ma.Layout990()
	r := fakets990.New()
	t.Cleanup(func() { _ = r.Close() })
	reader := newKWFrameReader(t, r.Port())

	t.Run("an address the chart does not print", func(t *testing.T) {
		const addr = "09999"
		if _, err := layout.BuildEXRead(kw.EXAddress{P1: 0, P2: 99, P3: 99}); err == nil {
			t.Fatal("the codec built a read for 0/99/99, which its inventory does not carry")
		}
		wire := "EX" + addr + ";"
		if _, err := r.Port().Write([]byte(wire)); err != nil {
			t.Fatalf("Write: unexpected error: %v", err)
		}
		if got, want := string(reader.readOneFrame()), "?;"; got != want {
			t.Errorf("%s -> %q, want %q", wire, got, want)
		}
	})

	t.Run("a sibling's EX read frame", func(t *testing.T) {
		// core/kw's ten-byte read (590:552, 480:410) and an FT-891's
		// seven-byte one. A TS-990S must not answer either, whatever its
		// digits name.
		for _, wire := range []string{"EX0000000;", "EX0101;"} {
			if _, err := r.Port().Write([]byte(wire)); err != nil {
				t.Fatalf("Write: unexpected error: %v", err)
			}
			if got, want := string(reader.readOneFrame()), "?;"; got != want {
				t.Errorf("%s -> %q, want %q — this row's read frame is eight bytes and carries no P4 (990:1734-1736)", wire, got, want)
			}
		}
	})
}

// TestTS990TranscriptionBCopy_ByteIdenticalToTheCodecs pins the COPY-NOT-MOVE
// rule mechanically (internal/fakets990/PROVENANCE.md).
//
// The fake's copy is a copy: the codec's own artefact does not move, because
// core/kw/ma/crosscheck_test.go keeps reading it as an artefact it binds and
// hashes by name. The cross-check above is only "codec from A versus fake from
// B" for as long as the fake's copy really is B: a drifted copy would still
// project a table, and a table quietly derived from a private variant of B is
// exactly the kind of agreement this milestone is trying not to have.
//
// If the codec-side artefact is ever corrected by an arbitration against the
// PDF, this test fails until the fake's copy is re-copied. That is the intended
// workflow, not an obstacle to it.
func TestTS990TranscriptionBCopy_ByteIdenticalToTheCodecs(t *testing.T) {
	assertByteIdentical(t, codecBCopy990, fakeBCopy990, "./internal/fakets990")
}
