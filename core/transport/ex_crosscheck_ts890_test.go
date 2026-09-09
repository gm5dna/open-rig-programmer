// SPDX-License-Identifier: GPL-3.0-or-later

package transport_test

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/kw"
	"github.com/gm5dna/open-rig-programmer/core/kw/ma"
	"github.com/gm5dna/open-rig-programmer/internal/fakets890"
)

// The TS-890S's half of the two-independent-transcriptions cross-check. The
// file comment on ex_crosscheck_ts590_test.go states the design in full, why
// these files are package transport_test, and what is deliberately not
// compared; the frame reader (newKWFrameReader), the mismatch renderer and the
// copy-not-move assertion (assertByteIdentical) live there and are reused here.
//
//	the CODEC's inventory (ma.EXItems890S) is generated from TRANSCRIPTION A
//	(core/kw/ma/menu890s.csv) by internal/extable;
//
//	the FAKE's inventory (fakets890.EXDefaults) is projected at init from its
//	own embedded copy of TRANSCRIPTION B by internal/fakets890/exinventory.go,
//	which imports nothing project-internal at all.
//
// # WHAT IS DIFFERENT ON THIS ROW, AND IT IS NOT COSMETIC
//
// THE ADDRESS IS A GROUPED TRIPLE. kw.EXAddress.Wire renders P1 alone as three
// digits and returns "" for a non-zero P2 or P3, which every address on this
// chart has; this family's rendering is ma.Layout.WireEXAddress, FIVE
// characters, P1 + P2P2 + P3P3 (890:1900). So the comparison below cannot reuse
// compareEXInventories — it keys the fake's map by Addr.Wire() — and a
// row-specific one is written instead. Reusing the 590's would compare every
// 890S address against "" and pass or fail for reasons that have nothing to do
// with the charts.
//
// THE FAKE'S PROJECTION APPLIES RULING R-B. Both legs read this chart's
// seventeen PF key rows differently: A carries four digits and B carries three,
// and the EX block's own P5 note settles it — "PF key settings use 4 digits
// (refer to the PF Key assignment ID lists)." (890:1918-1919). That
// arbitration is core/kw/ma/crosscheck_test.go's ruling R-B, which records THE
// LEG IS WRONG, NOT A and checks the divergence in both directions; the leg is
// frozen evidence, so the correction is applied in the fake's PROJECTION and
// never in the CSV. The fake reads the sentence itself and consults neither A
// nor internal/extable, so the two sides still meet here as two derivations.
// TestEXInventoryCrossCheck_TS890PFKeyWidthsAgreeUnderRulingRB pins that the
// correction is doing work rather than agreeing with what was already there.
//
// THE TWO SIDES ARE NOT THE SAME LENGTH AS TRANSCRIPTION B. The chart prints
// FOUR Advanced Menu rows with real addresses and the body "Does not correspond
// to a command" (890:2273-2280 — erratum E16). Transcription B carries all
// four, because they are printed and addressed and its ledger counts them; the
// PRODUCTION inventory excludes them by address under ParameterlessExcluded and
// then checks the arithmetic that follows. The FAKE therefore applies its OWN
// independently-written exclusion of the same four, and
// TestEXInventoryCrossCheck_TS890TheFourExcludedAddressesAreInNeitherInventory
// binds all three facts together: B has them, neither inventory does, and the
// two inventories are exactly four rows shorter than B.
//
// ON FAILURE: report the exact diff, and do NOT "fix" either table to make it
// pass. Which side is wrong — or whether the printed chart is — is an
// arbitration against the PDF. An edit that merely restores agreement destroys
// the evidence the agreement was worth.

// The two artefacts this file reads by path. The codec's copy is the one
// core/kw/ma/crosscheck_test.go binds and hashes by name; the fake's is the
// byte copy its projection embeds.
const (
	codecBCopy890 = "../kw/ma/testdata/transcription-b-890s.csv"
	fakeBCopy890  = "../../internal/fakets890/transcription-b-890s.csv"
)

// parameterlessAddresses890 is the four the production inventory excludes,
// written here as the FIVE-CHARACTER WIRE FORM and independently of both
// implementations — internal/extable's profile spells them as {1,0,23}… and
// the fake's projection as its own literals, and this is a third statement of
// the same four (890:2273-2280).
var parameterlessAddresses890 = []string{"10023", "10024", "10025", "10026"}

// compareMAEXInventories is this family's comparison: the codec's inventory
// against the fake's map, keyed by the layout's own five-character rendering.
//
// It reports, for every address in either side:
//
//	membership  the two address sets are equal
//	width       len(fake P5) == item.Digits
//	shape       the fake answers item.Digits x '0'
//
// The shape rule is UNCONDITIONAL, including on the codec's text rows: the
// fake's side cannot see textness (its projection does not read B's flag, and
// the flag is a convention rather than a datum — core/kw/ma/crosscheck_test.go's
// ruling), so answering an invented placeholder of the right WIDTH is the
// strongest honest claim this side can make. The text rows are pinned
// separately, by address.
//
// It is factored out of the test that runs it over the real tables so that the
// RED PROOFS below can run it over a deliberately perturbed one. A comparison
// that is only ever run on data expected to agree has never been shown to be
// able to disagree.
func compareMAEXInventories(layout ma.Layout, items []kw.EXItem, fake map[string]string) []exMismatch {
	var out []exMismatch
	seen := map[string]bool{}
	for _, item := range items {
		addr := layout.WireEXAddress(item.Addr)
		if addr == "" {
			out = append(out, exMismatch{item.Addr.String(), fmt.Sprintf("the layout renders no wire address for %q — a component outside the two printed menu types or the two-digit category/item domains", item.Name)})
			continue
		}
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

// TestEXInventoryCrossCheck_TS890AddressesAndWidthsAgree is the table-level
// leg.
func TestEXInventoryCrossCheck_TS890AddressesAndWidthsAgree(t *testing.T) {
	layout, items, fake := ma.Layout890(), ma.EXItems890S(), fakets890.EXDefaults()

	// Vacuity guard: two empty sets are trivially equal, and an inventory
	// that failed to generate would be exactly that.
	if len(items) == 0 || len(fake) == 0 {
		t.Fatalf("empty inventory: the codec has %d addresses, the fake has %d — one side failed to generate, and the comparison below would pass vacuously", len(items), len(fake))
	}

	if ms := compareMAEXInventories(layout, items, fake); len(ms) > 0 {
		t.Errorf("TS-890S EX inventories disagree between the two independent transcriptions of this chart — %d addresses:\n%s"+
			"This is a genuine cross-check finding, not a bug in this test: do not modify either table to make it pass — report the diff and arbitrate against the PDF.",
			len(ms), renderMismatches(ms))
	}
}

// TestEXInventoryCrossCheck_TS890TheFourExcludedAddressesAreInNeitherInventory
// is the 890S-only half of this file, and the one revision 1 omitted.
//
// It binds three facts that only mean anything together:
//
//   - TRANSCRIPTION B CARRIES ALL FOUR. They are printed, they are addressed,
//     and the evidence leg's ledger counts them (890:2273-2280). A B that had
//     lost one is no longer the artefact the codec's side excludes from.
//   - NEITHER INVENTORY CARRIES ANY OF THEM. The codec's is excluded by
//     internal/extable, by address; the fake's by its own independently written
//     list. If either exclusion were dropped, the comparison above would report
//     four addresses — but as "extra on one side", which points at the wrong
//     document.
//   - THE ARITHMETIC. Both inventories are exactly four rows shorter than B.
//     The bound is taken from B's own row count rather than written as a
//     figure, so no number appears here to be carried forward stale — and it
//     binds A's count to B's count, which two independently derived tables of
//     one chart owe each other.
func TestEXInventoryCrossCheck_TS890TheFourExcludedAddressesAreInNeitherInventory(t *testing.T) {
	bAddrs := transcriptionBAddresses890(t)
	layout, items, fake := ma.Layout890(), ma.EXItems890S(), fakets890.EXDefaults()

	codec := map[string]bool{}
	for _, item := range items {
		codec[layout.WireEXAddress(item.Addr)] = true
	}

	for _, addr := range parameterlessAddresses890 {
		if !bAddrs[addr] {
			t.Errorf("transcription B does not carry %s: the four \"Does not correspond to a command\" rows are PRINTED (890:2273-2280) and B counts them, so their absence means the artefact has drifted", addr)
		}
		if codec[addr] {
			t.Errorf("the CODEC's inventory carries %s, which internal/extable excludes by address under ParameterlessExcluded", addr)
		}
		if _, ok := fake[addr]; ok {
			t.Errorf("the FAKE's projection carries %s, which its own exclusion must remove — a literal projection of B is four entries longer than production and this cross-check fails on a correct implementation", addr)
		}
	}

	if want := len(bAddrs) - len(parameterlessAddresses890); len(items) != want || len(fake) != want {
		t.Errorf("transcription B has %d addressed rows and the exclusion names %d, so each inventory should have %d; the codec has %d and the fake %d",
			len(bAddrs), len(parameterlessAddresses890), want, len(items), len(fake))
	}
}

// transcriptionBAddresses890 reads the CODEC-side copy of transcription B and
// returns its addresses in the five-character wire form.
//
// It reads the codec's copy deliberately: the fake's is asserted byte-identical
// to it below, so using the original here means this test's B is the artefact
// core/kw/ma binds and hashes rather than a copy that might have drifted.
func transcriptionBAddresses890(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(codecBCopy890)
	if err != nil {
		t.Fatalf("reading %s: %v", codecBCopy890, err)
	}
	records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		t.Fatalf("parsing %s: %v", codecBCopy890, err)
	}
	if len(records) < 2 {
		t.Fatalf("%s has %d records — this test would pass vacuously", codecBCopy890, len(records))
	}
	if got, want := strings.Join(records[0], ","), "p1,p2,p3,name,digits,text"; got != want {
		t.Fatalf("%s's header is %q, want %q", codecBCopy890, got, want)
	}
	out := map[string]bool{}
	for _, rec := range records[1:] {
		addr := rec[0] + rec[1] + rec[2]
		if len(addr) != 5 {
			t.Fatalf("%s: address cells %q/%q/%q do not make a five-character wire address", codecBCopy890, rec[0], rec[1], rec[2])
		}
		if out[addr] {
			t.Fatalf("%s carries address %s twice", codecBCopy890, addr)
		}
		out[addr] = true
	}
	return out
}

// TestEXInventoryCrossCheck_TS890PFKeyWidthsAgreeUnderRulingRB pins the OTHER
// 890S-only fact of this file, and it is the one the plan did not anticipate.
//
// Transcription B reads THREE digits on all seventeen PF key rows and
// transcription A reads FOUR; the book's own P5 note settles it —
// "PF key settings use 4 digits (refer to the PF Key assignment ID lists)."
// (890:1918-1919) — and core/kw/ma/crosscheck_test.go's ruling R-B records the
// arbitration: THE LEG IS WRONG, NOT A. The leg is frozen evidence, so
// internal/fakets890's projection corrects those seventeen widths from the
// printed sentence rather than the CSV being edited.
//
// WITHOUT THIS TEST THE CORRECTION WOULD BE INVISIBLE. The comparison above
// would pass equally well if the fake had simply been given A's numbers, or if
// the leg were one day "tidied" to read four — and in either case the two
// sides would agree for a reason that is not two derivations meeting. So this
// asserts all three facts at once: both sides carry four, the LEG carries
// three, and the seventeen are exactly the rows the ruling names.
func TestEXInventoryCrossCheck_TS890PFKeyWidthsAgreeUnderRulingRB(t *testing.T) {
	layout, fake := ma.Layout890(), fakets890.EXDefaults()

	// The seventeen, spelt from the chart's own addresses rather than taken
	// from either implementation: PF A/B/C, External PF 1-8, Microphone PF
	// 1-4, Microphone DOWN and Microphone UP (0/00/15 … 0/00/31).
	var pf []string
	for item := 15; item <= 31; item++ {
		pf = append(pf, fmt.Sprintf("000%02d", item))
	}

	widths := map[string]int{}
	for _, item := range ma.EXItems890S() {
		widths[layout.WireEXAddress(item.Addr)] = item.Digits
	}
	legWidths := transcriptionBWidths890(t)

	for _, addr := range pf {
		if got, ok := widths[addr]; !ok || got != 4 {
			t.Errorf("the codec's inventory has %s at width %d (present=%v), want 4 (890:1918-1919)", addr, got, ok)
		}
		if got := fake[addr]; len(got) != 4 {
			t.Errorf("the fake answers %s with %q (%d bytes), want 4 — its projection must apply ruling R-B", addr, got, len(got))
		}
		if got := legWidths[addr]; got != 3 {
			t.Errorf("transcription B carries width %d at %s; ruling R-B records 3 there, and a leg that has moved is an arbitration rather than a correction to apply blind", got, addr)
		}
	}

	// The correction touches those seventeen and nothing else: every other
	// address the fake answers is its leg's own width.
	for addr, p5 := range fake {
		if slicesContains(pf, addr) {
			continue
		}
		if want := legWidths[addr]; len(p5) != want {
			t.Errorf("menu %s: the fake answers %d bytes where transcription B reads %d — the only correction this projection applies is ruling R-B's seventeen PF rows", addr, len(p5), want)
		}
	}
}

// transcriptionBWidths890 reads the codec-side copy of transcription B and
// returns its digits cells, keyed by wire address.
func transcriptionBWidths890(t *testing.T) map[string]int {
	t.Helper()
	data, err := os.ReadFile(codecBCopy890)
	if err != nil {
		t.Fatalf("reading %s: %v", codecBCopy890, err)
	}
	records, err := csv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil {
		t.Fatalf("parsing %s: %v", codecBCopy890, err)
	}
	out := map[string]int{}
	for _, rec := range records[1:] {
		n, err := strconv.Atoi(strings.TrimSpace(rec[4]))
		if err != nil {
			t.Fatalf("%s: digits cell %q on row %v: %v", codecBCopy890, rec[4], rec[:3], err)
		}
		out[rec[0]+rec[1]+rec[2]] = n
	}
	if len(out) == 0 {
		t.Fatalf("%s yielded no widths — this test would pass vacuously", codecBCopy890)
	}
	return out
}

// slicesContains is spelt out rather than imported so this file adds no import
// for one predicate.
func slicesContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// TestEXInventoryCrossCheck_TS890RedProofs runs the same comparison over
// DELIBERATELY perturbed copies of the fake's side, so that the green result
// above is known to be a result rather than a function that cannot fail.
//
// The two perturbations are the two defect classes the plan names: a dropped
// row, and a width-only change. The fake's projection refuses a DUPLICATE
// address at projection time and cannot see a width change at all, which is
// exactly why the second has to be caught here.
func TestEXInventoryCrossCheck_TS890RedProofs(t *testing.T) {
	layout, items := ma.Layout890(), ma.EXItems890S()
	const probe = "00300" // 0/03/00, Frequency Rounding Off — a three-digit row

	t.Run("a dropped address", func(t *testing.T) {
		fake := fakets890.EXDefaults()
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
		fake := fakets890.EXDefaults()
		if got := fake[probe]; got != "000" {
			t.Fatalf("menu %s's fake default is %q, and this proof assumes three bytes", probe, got)
		}
		fake[probe] = "0000"
		ms := compareMAEXInventories(layout, items, fake)
		if len(ms) != 1 || ms[0].addr != probe {
			t.Errorf("widening menu %s's fake default to four bytes reported %v, want exactly one mismatch at %s", probe, ms, probe)
		}
	})

	t.Run("a re-admitted excluded address", func(t *testing.T) {
		fake := fakets890.EXDefaults()
		fake["10023"] = "000"
		ms := compareMAEXInventories(layout, items, fake)
		if len(ms) != 1 || ms[0].addr != "10023" {
			t.Errorf("re-admitting the first excluded address reported %v, want exactly one mismatch at 10023", ms)
		}
	})

	// And the control: the comparison is not simply returning something for
	// everything.
	if ms := compareMAEXInventories(layout, items, fakets890.EXDefaults()); len(ms) != 0 {
		t.Errorf("the unperturbed comparison reported %d mismatches — the proofs above prove nothing", len(ms))
	}
}

// TestEXInventoryCrossCheck_TS890TextRowsAreTheRecordedOnes pins the one thing
// the shape rule above deliberately does not assert: WHICH addresses the
// codec's side marks as text.
//
// The fake answers an invented placeholder of the right width everywhere,
// including at these two, and that is the recorded state rather than a defect:
// transcription B's text column is a CONVENTION the two legs disagree about,
// and on this chart they differ at Contest Number (0/05/12) and Reference
// Oscillator Calibration (1/00/05) — both flagged text by B and not by A, where
// core/kw/ma/crosscheck_test.go's ruling is that only a row reading "Up to N
// alphanumeric characters" is a text row. What must not happen silently is
// transcription A acquiring a NEW text row, because that would be a change to
// the chart's reading that this file's shape rule would absorb without comment.
func TestEXInventoryCrossCheck_TS890TextRowsAreTheRecordedOnes(t *testing.T) {
	layout := ma.Layout890()
	// The two rows the chart prints as free text, written as literals: a set
	// taken from the table it bounds would prove nothing. They are the
	// Screen Saver Message ("0 to 10 characters", 890:1921) and the Power-on
	// Message ("0 to 15 characters", 890:1920).
	want := []string{"00005", "00006"}

	var got []string
	for _, item := range ma.EXItems890S() {
		if item.Text {
			got = append(got, layout.WireEXAddress(item.Addr))
		}
	}
	sort.Strings(got)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("the codec's inventory marks text rows %v, the chart prints %v (890:1920-1921) — a new text row is a finding to arbitrate, not a pin to widen", got, want)
	}
}

// --- The wire leg ---

// TestEXTS890RoundTrip_AllAddressesRawPort is the wire-level leg: every address
// the CODEC's inventory declares is asked of a real fakets890.Radio through its
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
// AND ON THIS ROW IT ALSO EXERCISES THE FRAME SHAPE ITSELF. The read is EIGHT
// bytes and carries NO P4 (890:1907), where core/kw's is ten with three printed
// constants; the answer splices P4 in before P5 and the terminator floats. Two
// sides that had read those two grids differently would agree on every width
// and still fail here.
//
// Engine.Do is deliberately bypassed (direct Port() write plus an accumulator
// read), the design constraint the FT-710's own round trip records: paying the
// engine's per-exchange Settle for every address in a chart this size would buy
// seconds of pacing for nothing, and Engine's correlation behaviour is covered
// by engine_ex_test.go.
func TestEXTS890RoundTrip_AllAddressesRawPort(t *testing.T) {
	layout, items := ma.Layout890(), ma.EXItems890S()
	if len(items) == 0 {
		t.Fatal("the codec's inventory is empty — this test would pass vacuously")
	}

	r := fakets890.New()
	t.Cleanup(func() { _ = r.Close() })
	fake := fakets890.EXDefaults()
	reader := newKWFrameReader(t, r.Port())

	answered := 0
	for _, item := range items {
		cmd, err := layout.BuildEXRead(item.Addr)
		if err != nil {
			t.Fatalf("BuildEXRead(%v): unexpected error: %v", item.Addr, err)
		}
		if got := len(cmd.Bytes()); got != ma.EXReadLen {
			t.Fatalf("BuildEXRead(%v) built a %d-byte frame (%q), want %d (890:1907)", item.Addr, got, cmd.Bytes(), ma.EXReadLen)
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
		if want := fake[layout.WireEXAddress(item.Addr)]; gotP5 != want {
			t.Errorf("%v: raw P5 = %q, want %q (the fake's own default)", item.Addr, gotP5, want)
		}
		answered++
	}
	if answered != len(items) {
		t.Errorf("%d of %d inventory addresses answered as expected", answered, len(items))
	}
}

// TestEXTS890RoundTrip_RefusedAddresses is the round trip's negative control.
// Without it, a fake that answered EVERY five-digit address with something
// plausible would pass the sweep above completely — the loop only asks about
// addresses that ARE in the inventory.
//
// THE FOUR EXCLUDED ADDRESSES ARE THE PAIR'S OWN CASE and have no counterpart
// in any other file here: they are PRINTED rows of this chart with real
// addresses, and both sides agree they are not members. The codec REFUSES TO
// BUILD a read for one — its builder is bounded by membership rather than by a
// scalar, precisely because this chart is sparse — so the frame is written by
// hand, exactly as the grammar prints it.
func TestEXTS890RoundTrip_RefusedAddresses(t *testing.T) {
	layout := ma.Layout890()
	r := fakets890.New()
	t.Cleanup(func() { _ = r.Close() })
	reader := newKWFrameReader(t, r.Port())

	t.Run("the four excluded addresses", func(t *testing.T) {
		for _, addr := range parameterlessAddresses890 {
			triple := kw.EXAddress{
				P1: uint8(addr[0] - '0'),
				P2: uint8((addr[1]-'0')*10 + (addr[2] - '0')),
				P3: uint8((addr[3]-'0')*10 + (addr[4] - '0')),
			}
			if _, err := layout.BuildEXRead(triple); err == nil {
				t.Fatalf("the codec built a read for %s, which its inventory excludes by address (890:2273-2280) — this negative control is testing the wrong thing", addr)
			}
			wire := "EX" + addr + ";"
			if _, err := r.Port().Write([]byte(wire)); err != nil {
				t.Fatalf("Write: unexpected error: %v", err)
			}
			if got, want := string(reader.readOneFrame()), "?;"; got != want {
				t.Errorf("%s -> %q, want %q (a printed row that corresponds to no command)", wire, got, want)
			}
		}
	})

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
		// seven-byte one. A TS-890S must not answer either, whatever its
		// digits name.
		for _, wire := range []string{"EX0000000;", "EX0101;"} {
			if _, err := r.Port().Write([]byte(wire)); err != nil {
				t.Fatalf("Write: unexpected error: %v", err)
			}
			if got, want := string(reader.readOneFrame()), "?;"; got != want {
				t.Errorf("%s -> %q, want %q — this row's read frame is eight bytes and carries no P4 (890:1907)", wire, got, want)
			}
		}
	})
}

// TestTS890TranscriptionBCopy_ByteIdenticalToTheCodecs pins the COPY-NOT-MOVE
// rule mechanically (internal/fakets890/PROVENANCE.md).
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
func TestTS890TranscriptionBCopy_ByteIdenticalToTheCodecs(t *testing.T) {
	assertByteIdentical(t, codecBCopy890, fakeBCopy890, "./internal/fakets890")
}
