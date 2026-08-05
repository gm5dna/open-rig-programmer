// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx101_test

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/cat/ftdx101"
)

// This file binds task 2's frame-geometry witness to the dialect this task
// builds. It is a COMMITTED TEST rather than an orchestrator's eyeball over a
// CSV, because the thing being checked is a standing invariant: the witness
// records where the FTdx101's manual PUTS each field of the MR, MT and MW
// frames, core/cat's fixed-offset codec decides where this dialect READS AND
// WRITES each field, and any future change to either — a frame constant, a
// tag width, a new MT form — must re-fire this comparison rather than rely on
// somebody remembering that it was once done by hand.
//
// THE WITNESS IS INDEPENDENT EVIDENCE, and that is the whole point of
// comparing against it. testdata/geometry-witness.csv was produced by a
// quarantined agent from 300 dpi RASTER RENDERS of the PDF only — no text
// layer, no repository access, no sight of core/cat's offsets (the method,
// the crops and the enlargements are recorded in geometry-witness.md). So the
// positions on the left of every comparison below were counted off the
// printed page by someone who did not know what this codec expects, and the
// positions on the right come from the codec via the dialect. Their agreement
// is evidence; it would not be if the witness had been written from the code.
//
// HOW THE COMPARISON IS MADE. Not by restating the witness's numbers as
// constants and asserting them equal to themselves — that is the failure mode
// a "geometry test" naturally decays into. Instead each grid is ASSEMBLED
// FROM THE WITNESS: a byte slice of the witnessed length is filled by placing
// each token's expected content at the token's own witnessed positions, and
// the result is then put to the dialect. For the Set and Read grids the
// assembled frame must equal, byte for byte, what the dialect's own BUILDER
// produces from the same field values; for the Answer grids it must PARSE
// through the dialect's own parser back to those same field values. A witness
// position one byte out therefore fails as a frame mismatch or a parse error,
// not as an arithmetic disagreement between two hand-copied numbers.
//
// EVERY WITNESSED ROW IS CONSUMED. The row set is checked for exactly the six
// expected grids, each grid for contiguous single coverage of positions 1..N,
// and each grid's token set against the expectation table — an unknown token,
// a missing token, a gap, an overlap or an unexpected grid all fail. There is
// no skip path: a row this test does not understand is a failure, because a
// silently ignored row is a piece of evidence that stopped being evidence.
//
// THE WITNESS RECORDS SIX GRIDS, NOT NINE. MR Set, MW Read and MW Answer are
// printed in this manual as EMPTY form slots — the position ruler is drawn and
// nothing is placed on it — which matches the availability table (MR is
// X O O X, MW is O X X X) and matches core/cat, whose mr.go has no Set builder
// and whose mw.go has no Read or Answer. geometry-witness.md records the empty
// slots explicitly. Their absence here is therefore a fact about the radio and
// is asserted as one.
//
// ON DISAGREEMENT: STOP. If this test fails on a change to the witness or to
// core/cat's frame constants, the fix is arbitration against the PDF — one of
// the witness, the dialect and the spec is wrong, and which one is a question
// for the reviewer, not for whichever is easier to edit. The witness is
// never edited to satisfy the test.

const geometryWitnessPath = "testdata/geometry-witness.csv"

// witnessToken is one row of the witness: a named token and the 1-based
// inclusive character positions the manual's grid gives it.
type witnessToken struct {
	token string
	first int
	last  int
	line  int // CSV line, for error messages that point at the row
}

// gridKey identifies one of the manual's position charts.
type gridKey struct {
	command   string
	direction string
}

func (g gridKey) String() string { return g.command + " " + g.direction }

// readWitness parses the committed witness into grids, checking the header
// and every field as it goes. Any malformed row is fatal: this file's whole
// value rests on the witness being read exactly as it was written.
func readWitness(t *testing.T) map[gridKey][]witnessToken {
	t.Helper()

	f, err := os.Open(geometryWitnessPath)
	if err != nil {
		t.Fatalf("opening the committed geometry witness: %v", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = 5
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("reading %s: %v", geometryWitnessPath, err)
	}
	if len(records) < 2 {
		t.Fatalf("%s has %d records — an empty witness would make every comparison below vacuous", geometryWitnessPath, len(records))
	}

	wantHeader := []string{"command", "direction", "token", "first_pos", "last_pos"}
	for i, h := range wantHeader {
		if records[0][i] != h {
			t.Fatalf("%s header field %d is %q, want %q — the witness's own column order is what every index below assumes", geometryWitnessPath, i, records[0][i], h)
		}
	}

	grids := make(map[gridKey][]witnessToken)
	for i, rec := range records[1:] {
		line := i + 2
		first, err := strconv.Atoi(rec[3])
		if err != nil {
			t.Fatalf("%s line %d: first_pos %q is not an integer", geometryWitnessPath, line, rec[3])
		}
		last, err := strconv.Atoi(rec[4])
		if err != nil {
			t.Fatalf("%s line %d: last_pos %q is not an integer", geometryWitnessPath, line, rec[4])
		}
		if first < 1 || last < first {
			t.Fatalf("%s line %d: positions (%d, %d) are not a 1-based inclusive span", geometryWitnessPath, line, first, last)
		}
		k := gridKey{command: rec[0], direction: rec[1]}
		grids[k] = append(grids[k], witnessToken{token: rec[2], first: first, last: last, line: line})
	}
	return grids
}

// The field values every assembled frame carries. Chosen to be mutually
// distinguishable wherever the wire allows it — a nine-digit frequency that
// is not a run of one digit, a negative clarifier so the sign byte is not '+',
// a mode nibble that is neither the first nor the last of the legend, a CTCSS
// state and a shift that are not the zero value, and a full-width tag of
// twelve DIFFERENT characters so that every P12 position is individually
// pinned.
//
// Four positions cannot be made distinctive, and are named here rather than
// left to be noticed: P7 is fixed at '0' in the Set direction by this radio's
// own legends, P9 is the fixed "00", P11 is fixed '0', and P5 is '0' because
// TxClar is false. A transposition of two of those four would not be caught
// by the assembled-frame comparison. It is caught in the read direction
// instead, where P7 carries '1' against P5's '0' and P11's '0'.
const (
	geomSlotWire = "037"
	geomFreqHz   = 14195000
	geomFreqWire = "014195000"
	geomClarHz   = -1230
	geomClarWire = "-1230"
	geomTag      = "ABCDEFGHIJKL"
)

// geomMemoryData is the record placed into every assembled memory-shaped
// frame. kind varies with direction: the Set grids carry the byte this
// radio's legend fixes, the Answer grids carry '1' (Memory).
func geomMemoryData(t *testing.T, d cat.Dialect, kind byte) cat.MemoryData {
	t.Helper()

	s, err := d.ParseSlot(geomSlotWire)
	if err != nil {
		t.Fatalf("ParseSlot(%q) = %v — this test needs a slot the dialect accepts", geomSlotWire, err)
	}
	return cat.MemoryData{
		Slot:   s,
		FreqHz: geomFreqHz,
		ClarHz: geomClarHz,
		RxClar: true,
		TxClar: false,
		Mode:   cat.Mode('9'), // RTTY-U
		Kind:   kind,
		CTCSS:  cat.CTCSSEnc,
		Shift:  cat.ShiftMinus,
	}
}

// memoryBlockTokens is the expected content of the twenty-eight-position
// field block the MR answer, the MW Set and the MT Set/Answer all share —
// everything up to and including P10. kind is the P7 byte for the direction
// in hand.
func memoryBlockTokens(prefix string, kind byte) map[string]string {
	return map[string]string{
		prefix: prefix,
		"P1":   geomSlotWire,
		"P2":   geomFreqWire,
		"P3":   geomClarWire,
		"P4":   "1", // RxClar true
		"P5":   "0", // TxClar false
		"P6":   "9", // Mode RTTY-U
		"P7":   string(kind),
		"P8":   string(byte(cat.CTCSSEnc)),
		"P9":   "00",
		"P10":  string(byte(cat.ShiftMinus)),
	}
}

// assemble builds one grid's frame from the witness alone: a byte slice of
// the witnessed length with each token's expected content written at that
// token's own witnessed positions. It is the step that makes this a test OF
// the witness's numbers rather than a restatement of them.
//
// Coverage is checked as it goes, and every kind of gap is a failure: a
// position no row claims, a position two rows claim, a token the expectation
// table does not know, and a token the table knows that the witness does not
// carry.
func assemble(t *testing.T, k gridKey, rows []witnessToken, want map[string]string) []byte {
	t.Helper()

	if len(rows) == 0 {
		t.Fatalf("%s: the witness carries no rows for this grid", k)
	}

	n := 0
	for _, r := range rows {
		if r.last > n {
			n = r.last
		}
	}
	frame := make([]byte, n)
	claimed := make([]int, n) // CSV line that claimed each position, 0 = unclaimed

	seen := make(map[string]bool, len(rows))
	for _, r := range rows {
		content, ok := want[r.token]
		if !ok {
			t.Fatalf("%s line %d: witnessed token %q is not one this test expects in the %s grid — an unrecognised row must fail rather than be skipped, since a skipped row is evidence that stopped being evidence", geometryWitnessPath, r.line, r.token, k)
		}
		if seen[r.token] {
			t.Fatalf("%s line %d: token %q appears twice in the %s grid", geometryWitnessPath, r.line, r.token, k)
		}
		seen[r.token] = true

		width := r.last - r.first + 1
		if width != len(content) {
			t.Errorf("%s line %d: the %s grid gives %q positions %d-%d (%d characters), but this dialect's field is %d characters (%q) — the manual's chart and core/cat's frame layout disagree; ARBITRATE AGAINST THE PDF, do not edit either side to agree", geometryWitnessPath, r.line, k, r.token, r.first, r.last, width, len(content), content)
			continue
		}
		for i := r.first; i <= r.last; i++ {
			if claimed[i-1] != 0 {
				t.Fatalf("%s line %d: position %d of the %s grid is claimed by token %q and already by line %d — overlapping tokens", geometryWitnessPath, r.line, i, k, r.token, claimed[i-1])
			}
			claimed[i-1] = r.line
		}
		copy(frame[r.first-1:], content)
	}

	for tok := range want {
		if !seen[tok] {
			t.Errorf("the %s grid of the witness carries no %q row, but this dialect's frame has that field — a token missing from the witness is not a token this test may assume", k, tok)
		}
	}
	for i, line := range claimed {
		if line == 0 {
			t.Errorf("position %d of the %s grid is claimed by no witnessed token, yet the grid runs to %d — a hole in the chart is a disagreement, not a don't-care", i+1, k, n)
		}
	}
	return frame
}

// TestGeometryWitnessBindsDialect is the binding itself. It runs over BOTH
// models: the frame geometry is identical for the D and the MP (nothing in
// this manual's frame tables distinguishes them), and asserting that here
// costs one loop.
func TestGeometryWitnessBindsDialect(t *testing.T) {
	grids := readWitness(t)

	// EXACTLY the six grids the manual prints, no more and no fewer. The
	// three absent ones are the empty form slots recorded in
	// geometry-witness.md; a witness that acquired an "MR set" grid would be
	// describing a command this radio does not have.
	wantGrids := []gridKey{
		{"MR", "read"}, {"MR", "answer"},
		{"MT", "set"}, {"MT", "read"}, {"MT", "answer"},
		{"MW", "set"},
	}
	if len(grids) != len(wantGrids) {
		got := make([]string, 0, len(grids))
		for k := range grids {
			got = append(got, k.String())
		}
		sort.Strings(got)
		t.Fatalf("the witness carries %d grids %v, want the %d printed ones — MR Set, MW Read and MW Answer are EMPTY form slots in this manual (availability table: MR is X O O X, MW is O X X X), which is why they are absent", len(grids), got, len(wantGrids))
	}
	for _, k := range wantGrids {
		if _, ok := grids[k]; !ok {
			t.Fatalf("the witness carries no %s grid", k)
		}
	}

	for _, m := range bothModels() {
		t.Run(m.name, func(t *testing.T) {
			d := m.d

			// ----- the Read grids: assembled frame == what the builder emits.
			slot, err := d.ParseSlot(geomSlotWire)
			if err != nil {
				t.Fatalf("ParseSlot(%q) = %v", geomSlotWire, err)
			}

			mrRead, err := d.BuildMRRead(slot)
			if err != nil {
				t.Fatalf("BuildMRRead(%q) = %v", geomSlotWire, err)
			}
			got := assemble(t, gridKey{"MR", "read"}, grids[gridKey{"MR", "read"}], map[string]string{
				"MR": "MR", "P0": geomSlotWire, ";": ";",
			})
			if string(got) != string(mrRead.Bytes()) {
				t.Errorf("MR read: the frame assembled from the witness is %q, the dialect builds %q — the manual's chart and core/cat's mr.go disagree about this frame; ARBITRATE AGAINST THE PDF", got, mrRead.Bytes())
			}

			mtRead, err := d.BuildMTRead(slot)
			if err != nil {
				t.Fatalf("BuildMTRead(%q) = %v", geomSlotWire, err)
			}
			got = assemble(t, gridKey{"MT", "read"}, grids[gridKey{"MT", "read"}], map[string]string{
				"MT": "MT", "P0": geomSlotWire, ";": ";",
			})
			if string(got) != string(mtRead.Bytes()) {
				t.Errorf("MT read: the frame assembled from the witness is %q, the dialect builds %q", got, mtRead.Bytes())
			}

			// ----- MW Set: assembled frame == what BuildMWSet emits.
			mwTokens := memoryBlockTokens("MW", d.MWWriteKind())
			mwTokens[";"] = ";"
			mw, err := d.BuildMWSet(geomMemoryData(t, d, d.MWWriteKind()))
			if err != nil {
				t.Fatalf("BuildMWSet = %v", err)
			}
			got = assemble(t, gridKey{"MW", "set"}, grids[gridKey{"MW", "set"}], mwTokens)
			if string(got) != string(mw.Bytes()) {
				t.Errorf("MW set: the frame assembled from the witness is %q, the dialect builds %q", got, mw.Bytes())
			}

			// ----- MT Set: assembled frame == what BuildMTSetCombined emits.
			mtTokens := memoryBlockTokens("MT", cat.CombinedMTSetKind)
			mtTokens["P11"] = "0"
			mtTokens["P12"] = geomTag
			mtTokens[";"] = ";"
			mtSet, err := d.BuildMTSetCombined(geomMemoryData(t, d, cat.CombinedMTSetKind), geomTag)
			if err != nil {
				t.Fatalf("BuildMTSetCombined = %v", err)
			}
			got = assemble(t, gridKey{"MT", "set"}, grids[gridKey{"MT", "set"}], mtTokens)
			if string(got) != string(mtSet.Bytes()) {
				t.Errorf("MT set: the frame assembled from the witness is %q, the dialect builds %q", got, mtSet.Bytes())
			}

			// ----- the Answer grids: assembled frame PARSES back to the
			// values placed in it. There is no builder for an answer, so the
			// dialect is put on the reading side instead — which is the
			// direction the witness's Answer charts actually describe.
			//
			// P7 is '1' (Memory) here rather than the Set direction's '0':
			// MR's legend reads "0: VFO 1: Memory" and MT's reads
			// "Set: 0: (Fixed) / Read: 0: VFO 1: Memory", so the read
			// direction is where a non-zero P7 can be exercised at all.
			mrAnsTokens := memoryBlockTokens("MR", cat.KindMemory)
			mrAnsTokens[";"] = ";"
			got = assemble(t, gridKey{"MR", "answer"}, grids[gridKey{"MR", "answer"}], mrAnsTokens)
			mrData, err := d.ParseMRAnswer(got)
			if err != nil {
				t.Errorf("ParseMRAnswer(%q) = %v — the frame is assembled purely from the witness's positions, so a refusal means the chart and core/cat's memdata.go offsets disagree; ARBITRATE AGAINST THE PDF", got, err)
			} else {
				checkMemoryFields(t, "MR answer", mrData, cat.KindMemory)
			}

			mtAnsTokens := memoryBlockTokens("MT", cat.KindMemory)
			mtAnsTokens["P11"] = "0"
			mtAnsTokens["P12"] = geomTag
			mtAnsTokens[";"] = ";"
			got = assemble(t, gridKey{"MT", "answer"}, grids[gridKey{"MT", "answer"}], mtAnsTokens)
			mtData, gotTag, err := d.ParseMTAnswerCombined(got)
			if err != nil {
				t.Errorf("ParseMTAnswerCombined(%q) = %v", got, err)
			} else {
				checkMemoryFields(t, "MT answer", mtData, cat.KindMemory)
				if gotTag != geomTag {
					t.Errorf("MT answer: tag parsed as %q, want %q — the P12 field's witnessed span and this dialect's tag field are not the same twelve positions", gotTag, geomTag)
				}
			}
		})
	}
}

// checkMemoryFields holds a parsed record to the values the assembled frame
// placed at the witnessed positions. A field read from the wrong offset comes
// back wrong here even when the frame's LENGTH is right, which is the failure
// a length-only geometry check would miss.
func checkMemoryFields(t *testing.T, what string, m cat.MemoryData, kind byte) {
	t.Helper()

	if got := m.Slot.Wire(); got != geomSlotWire {
		t.Errorf("%s: P1 slot parsed as %q, want %q", what, got, geomSlotWire)
	}
	if m.FreqHz != geomFreqHz {
		t.Errorf("%s: P2 frequency parsed as %d, want %d", what, m.FreqHz, geomFreqHz)
	}
	if m.ClarHz != geomClarHz {
		t.Errorf("%s: P3 clarifier parsed as %d, want %d", what, m.ClarHz, geomClarHz)
	}
	if !m.RxClar {
		t.Errorf("%s: P4 RX clarifier parsed as false, want true", what)
	}
	if m.TxClar {
		t.Errorf("%s: P5 TX clarifier parsed as true, want false", what)
	}
	if m.Mode != cat.Mode('9') {
		t.Errorf("%s: P6 mode parsed as %q, want %q", what, byte(m.Mode), '9')
	}
	if m.Kind != kind {
		t.Errorf("%s: P7 kind parsed as %q, want %q", what, m.Kind, kind)
	}
	if m.CTCSS != cat.CTCSSEnc {
		t.Errorf("%s: P8 CTCSS parsed as %q, want %q", what, byte(m.CTCSS), byte(cat.CTCSSEnc))
	}
	if m.Shift != cat.ShiftMinus {
		t.Errorf("%s: P10 shift parsed as %q, want %q", what, byte(m.Shift), byte(cat.ShiftMinus))
	}
}

// TestGeometryWitnessTerminatorPositions states the four frame lengths the
// spec's hypothesis turns on, each taken from the witness on one side and
// DERIVED FROM THE DIALECT on the other — never written as a literal on both.
//
// This overlaps TestGeometryWitnessBindsDialect deliberately. That test would
// catch every one of these as a frame mismatch, but it would report them as
// "%q != %q" over forty-one bytes; these four assertions name the numbers the
// M9d spec actually claimed, so a failure says which claim fell.
func TestGeometryWitnessTerminatorPositions(t *testing.T) {
	grids := readWitness(t)
	d := ftdx101.DialectD()

	// The combined MT bound, from the dialect. 41 is not written here and is
	// not written in core/cat either: it is 29 + TagMaxBytes, so a family
	// with a different tag field gets a different answer from the same code.
	min, max, err := d.MTAnswerBounds()
	if err != nil {
		t.Fatalf("MTAnswerBounds() = %v", err)
	}
	if min != max {
		t.Fatalf("MTAnswerBounds() = (%d, %d) — the combined form's bounds are exact, and the checks below assume one number", min, max)
	}
	tagMax := max - 29 // the combined frame is 29 fixed positions plus the tag

	// The memory-frame and read-frame lengths, from the dialect's own
	// builders rather than from core/cat's unexported constants.
	slot, err := d.ParseSlot(geomSlotWire)
	if err != nil {
		t.Fatalf("ParseSlot(%q) = %v", geomSlotWire, err)
	}
	mw, err := d.BuildMWSet(geomMemoryData(t, d, d.MWWriteKind()))
	if err != nil {
		t.Fatalf("BuildMWSet = %v", err)
	}
	memLen := len(mw.Bytes())
	mrRead, err := d.BuildMRRead(slot)
	if err != nil {
		t.Fatalf("BuildMRRead = %v", err)
	}
	mtRead, err := d.BuildMTRead(slot)
	if err != nil {
		t.Fatalf("BuildMTRead = %v", err)
	}

	for _, c := range []struct {
		grid gridKey
		want int
		why  string
	}{
		{gridKey{"MT", "set"}, max, "the combined MT Set runs to MTAnswerBounds()'s exact length"},
		{gridKey{"MT", "answer"}, max, "the combined MT Answer runs to MTAnswerBounds()'s exact length"},
		{gridKey{"MR", "answer"}, memLen, "the MR Answer is the 28-byte memory frame BuildMWSet also emits"},
		{gridKey{"MW", "set"}, memLen, "the MW Set is that same 28-byte memory frame"},
		{gridKey{"MR", "read"}, len(mrRead.Bytes()), "the MR Read is what BuildMRRead emits"},
		{gridKey{"MT", "read"}, len(mtRead.Bytes()), "the MT Read is what BuildMTRead emits"},
	} {
		tok, ok := tokenIn(grids[c.grid], ";")
		if !ok {
			t.Errorf("the %s grid of the witness carries no %q row", c.grid, ";")
			continue
		}
		if tok.first != tok.last {
			t.Errorf("%s line %d: the %s terminator spans %d-%d — %q is one character", geometryWitnessPath, tok.line, c.grid, tok.first, tok.last, ";")
		}
		if tok.first != c.want {
			t.Errorf("%s line %d: the witness puts the %s terminator at position %d; the dialect's frame ends at %d (%s). ARBITRATE AGAINST THE PDF — the witness is never edited to satisfy this test", geometryWitnessPath, tok.line, c.grid, tok.first, c.want, c.why)
		}
	}

	// P12, the field that makes the combined form combined. Its span is
	// checked against the dialect's own tag width, and its START against the
	// twenty-nine fixed positions that precede it.
	for _, g := range []gridKey{{"MT", "set"}, {"MT", "answer"}} {
		tok, ok := tokenIn(grids[g], "P12")
		if !ok {
			t.Errorf("the %s grid of the witness carries no P12 row", g)
			continue
		}
		if tok.first != 29 {
			t.Errorf("%s line %d: the witness starts %s's P12 tag at position %d, want 29 — the combined record's first twenty-eight positions are the shared memory block and P11", geometryWitnessPath, tok.line, g, tok.first)
		}
		if got := tok.last - tok.first + 1; got != tagMax {
			t.Errorf("%s line %d: the witness gives %s's P12 tag %d characters (%d-%d); this dialect's TagMaxBytes, derived as MTAnswerBounds() - 29, is %d", geometryWitnessPath, tok.line, g, got, tok.first, tok.last, tagMax)
		}
		if tok.last != max-1 {
			t.Errorf("%s line %d: the witness ends %s's P12 tag at position %d, but the terminator is at %d — the tag field must run right up to it", geometryWitnessPath, tok.line, g, tok.last, max)
		}
	}

	// And the arithmetic itself, stated once: this is the number the M9d
	// spec hypothesised from the FTdx10 precedent before any raster was
	// counted, and it is what the witness independently produced.
	if want := 29 + tagMax; max != want {
		t.Errorf("MTAnswerBounds() = %d, want 29 + TagMaxBytes = %d", max, want)
	}
	if tagMax != 12 {
		t.Errorf("this dialect's tag field is %d bytes, want 12 — the manual's P12 legend reads \"TAG Characters (up to 12 characters) (ASCII)\" (layout 1330)", tagMax)
	}
	if fmt.Sprintf("%d", max) != "41" {
		t.Errorf("the combined frame length derives to %d, want 41 — the spec's hypothesis and the raster count both say 41", max)
	}
}

// tokenIn finds one named token in a grid's rows.
func tokenIn(rows []witnessToken, name string) (witnessToken, bool) {
	for _, r := range rows {
		if r.token == name {
			return r, true
		}
	}
	return witnessToken{}, false
}
