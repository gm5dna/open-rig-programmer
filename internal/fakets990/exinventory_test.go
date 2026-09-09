// SPDX-License-Identifier: GPL-3.0-or-later

package fakets990

import (
	"strings"
	"testing"
)

// TestEXDefaults_CarriesEveryAddressTranscriptionBPrints. THIS CHART HAS NO
// EXCLUDED ADDRESS, and the absence is the 990S's own fact rather than an
// omission: the four "Does not correspond to a command" rows are the 890S's
// (890:2273-2280, erratum E16) and this book prints none, so the projection
// removes nothing and the fake's inventory is transcription B's addresses
// exactly.
//
// It is written as a set difference against transcription B's OWN addresses,
// re-parsed here, rather than as a count: a count-only assertion is satisfied
// by a projection that dropped one row and invented another.
func TestEXDefaults_CarriesEveryAddressTranscriptionBPrints(t *testing.T) {
	rows, err := parseB(transcriptionB990S)
	if err != nil {
		t.Fatalf("parseB(the committed transcription B): %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("transcription B parsed to zero rows — every assertion below would pass vacuously")
	}

	got := EXDefaults()
	inB := map[string]bool{}
	for _, r := range rows {
		inB[r.wire()] = true
		if _, ok := got[r.wire()]; !ok {
			t.Errorf("transcription B carries %s (line %d) and the projection does not — this chart prints no parameterless row for the projection to exclude", r.wire(), r.line)
		}
	}
	for addr := range got {
		if !inB[addr] {
			t.Errorf("the projection carries %q, which transcription B does not — the fake answers only what its own evidence leg prints", addr)
		}
	}
}

// TestEXDefaults_EveryValueIsItsWidthInZeroBytes pins the invented-placeholder
// convention (doc.go's register entry THE EX MENU VALUES ARE INVENTED) against
// transcription B's own digits cells, re-read here.
func TestEXDefaults_EveryValueIsItsWidthInZeroBytes(t *testing.T) {
	rows, err := parseB(transcriptionB990S)
	if err != nil {
		t.Fatalf("parseB: %v", err)
	}
	pf := map[string]bool{}
	for _, addr := range pfKeyAddresses {
		pf[addr] = true
	}
	got := EXDefaults()
	checked := 0
	for _, r := range rows {
		p5, ok := got[r.wire()]
		if !ok {
			t.Errorf("menu %s is not in the projection", r.wire())
			continue
		}
		width := r.width
		if pf[r.wire()] {
			// Ruling R-B: the leg reads three here and the book prints four
			// (990:1747-1748). TestEXDefaults_ThePFKeyRowsAnswerFourBytes is
			// where that correction is pinned; here it is only accounted
			// for, so this test stays about the placeholder convention.
			width = pfKeyWidth
		}
		if want := strings.Repeat("0", width); p5 != want {
			t.Errorf("menu %s (line %d): default P5 = %q, want %q", r.wire(), r.line, p5, want)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("checked zero addresses — this test would pass vacuously")
	}
}

// TestEXDefaults_ReturnsAFreshCopy pins the contract core/transport's
// cross-check and every *Radio depend on: mutating one caller's map must not
// reach another's, nor the package's own table.
func TestEXDefaults_ReturnsAFreshCopy(t *testing.T) {
	const probe = "00000"
	first := EXDefaults()
	if _, ok := first[probe]; !ok {
		t.Fatalf("menu %s is not in the projection — this test needs a real address to mutate", probe)
	}
	first[probe] = "mutated"
	if got := EXDefaults()[probe]; got == "mutated" {
		t.Errorf("EXDefaults()[%s] = %q after a caller mutated an earlier result — the two share a map", probe, got)
	}
}

// bWith renders a synthetic transcription B from a header and a set of data
// rows, for the refusal proofs below. The refusals cannot be driven from the
// committed CSV, which is well formed by construction.
func bWith(rows ...string) []byte {
	return []byte("p1,p2,p3,name,digits,text\n" + strings.Join(rows, "\n") + "\n")
}

// ruledRows are the rows projectWidths REQUIRES of any chart it folds: the
// eighteen ruling R-B corrects. A synthetic chart missing them is refused,
// which is the point — so the projection proofs that are not ABOUT that refusal
// supply them.
func ruledRows() []string {
	var out []string
	for _, addr := range pfKeyAddresses {
		out = append(out, addr[:1]+","+addr[1:3]+","+addr[3:]+",A PF Key Assignment,3,0")
	}
	return out
}

// bWithRuled renders a synthetic chart carrying rows plus every row the ruling
// needs.
func bWithRuled(rows ...string) []byte {
	return bWith(append(append([]string{}, rows...), ruledRows()...)...)
}

// TestParseB_Refusals pins every malformed input as a REFUSAL rather than a
// shorter table. The CSV is a committed, hash-frozen evidential artefact, so
// anything the projection cannot read is a finding — and a fake answering from
// a truncated inventory would be worse than one that refuses to start.
func TestParseB_Refusals(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			"a header that is not transcription B's",
			[]byte("menu_number,name,digits,text\n000,Display brightness,1,0\n"),
			"header",
		},
		{
			"a two-digit menu-type cell",
			bWith("00,00,00,Color Display Pattern (Main screen),3,0"),
			"p1 cell",
		},
		{
			"a menu type outside the two printed values",
			bWith("2,00,00,Color Display Pattern (Main screen),3,0"),
			"p1 cell",
		},
		{
			"a one-digit category cell — a lost leading zero",
			bWith("0,0,00,Color Display Pattern (Main screen),3,0"),
			"p2 cell",
		},
		{
			"a three-digit item cell",
			bWith("0,00,000,Color Display Pattern (Main screen),3,0"),
			"p3 cell",
		},
		{
			"an empty name cell — the signature of a misparsed row",
			bWith("0,00,00, ,3,0"),
			"empty name cell",
		},
		{
			"a digits cell that is not a number",
			bWith("0,00,00,Color Display Pattern (Main screen),three,0"),
			"digits cell",
		},
		{
			"a digits cell wider than any width this chart prints",
			bWith("0,00,00,Color Display Pattern (Main screen),16,0"),
			"outside 1-15",
		},
		{
			"a zero-width digits cell",
			bWith("0,00,00,Color Display Pattern (Main screen),0,0"),
			"outside 1-15",
		},
		{
			"a third value in the text column",
			bWith("0,00,00,Color Display Pattern (Main screen),3,2"),
			"text cell",
		},
		{
			"no data rows at all",
			bWith(),
			"no data rows",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseB(tt.data)
			if err == nil {
				t.Fatalf("parseB accepted %q", tt.data)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("parseB error = %q, want it to name %q", err, tt.want)
			}
		})
	}
}

// TestProjectWidths_RefusesADuplicateAddress. A repeated address would silently
// overwrite one row's width with another's, which is the one defect a width
// comparison against the codec could not localise — the table would be one row
// short and the missing address would be reported as absent rather than as
// duplicated.
func TestProjectWidths_RefusesADuplicateAddress(t *testing.T) {
	rows, err := parseB(bWithRuled(
		"0,00,00,Color Display Pattern (Main screen),3,0",
		"0,00,00,Color Display Pattern (Main screen),3,0",
	))
	if err != nil {
		t.Fatalf("parseB: %v", err)
	}
	if _, err := projectWidths(rows); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Errorf("projectWidths on a duplicated address returned %v, want a refusal naming the repeat", err)
	}
}

// TestProjectWidths_AppliesRulingRBToThePFKeyRows is the correction's positive
// proof: the eighteen PF rows arrive from the leg carrying three and leave the
// projection carrying the four the book prints — "PF key settings use 4 digits
// (refer to the PF Key assignment ID lists)." (990:1747-1748) — while every
// other row is untouched.
func TestProjectWidths_AppliesRulingRBToThePFKeyRows(t *testing.T) {
	rows, err := parseB(bWithRuled("0,00,00,Color Display Pattern (Main screen),3,0"))
	if err != nil {
		t.Fatalf("parseB: %v", err)
	}
	widths, err := projectWidths(rows)
	if err != nil {
		t.Fatalf("projectWidths: %v", err)
	}
	for _, addr := range pfKeyAddresses {
		if got := widths[addr]; got != pfKeyWidth {
			t.Errorf("PF key %s projected width %d, want %d (990:1747-1748, ruling R-B)", addr, got, pfKeyWidth)
		}
	}
	if got := widths["00000"]; got != 3 {
		t.Errorf("the ordinary row 00000 projected width %d, want 3 — the correction must touch the PF rows and nothing else", got)
	}
}

// TestProjectWidths_TheRunIsEighteenRowsAndItIsThisBooksOwn. The 890S's
// equivalent run is SEVENTEEN; this one is eighteen because the 990S inserts
// Voice (Main Band) and Voice (Sub Band) at items 17 and 18 (990:1793-1810), so
// the run ends at 0/00/32 where the 890S's ends at 0/00/31. Copying the
// sibling's list would leave two of this chart's PF rows answering three bytes.
func TestProjectWidths_TheRunIsEighteenRowsAndItIsThisBooksOwn(t *testing.T) {
	var want []string
	for item := 15; item <= 32; item++ {
		want = append(want, "000"+string(rune('0'+item/10))+string(rune('0'+item%10)))
	}
	if len(want) != 18 {
		t.Fatalf("the test built %d addresses for 0/00/15 … 0/00/32, want 18", len(want))
	}
	if strings.Join(pfKeyAddresses, ",") != strings.Join(want, ",") {
		t.Errorf("pfKeyAddresses = %v, want the eighteen rows 0/00/15 … 0/00/32 (990:1793-1810)", pfKeyAddresses)
	}
}

// TestProjectWidths_RefusesAChangedOrMissingPFRow. The correction is meaningful
// only against the divergence the ruling records: a leg that no longer reads
// three there has MOVED, which is an arbitration rather than a correction to
// apply blind. This is ruling R-B's both-directions check, brought to this side.
func TestProjectWidths_RefusesAChangedOrMissingPFRow(t *testing.T) {
	tests := []struct {
		name string
		rows []string
		want string
	}{
		{
			"a PF row missing altogether",
			append([]string{"0,00,00,Color Display Pattern (Main screen),3,0"}, ruledRows()[1:]...),
			"is not in this transcription",
		},
		{
			"a PF row the leg no longer reads as three",
			func() []string {
				rows := append([]string{"0,00,00,Color Display Pattern (Main screen),3,0"}, ruledRows()...)
				rows[1] = "0,00,15,PF A: Key Assignment,4,0"
				return rows
			}(),
			"the divergence has changed rather than gone",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := parseB(bWith(tt.rows...))
			if err != nil {
				t.Fatalf("parseB: %v", err)
			}
			_, err = projectWidths(rows)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("projectWidths returned %v, want a refusal naming %q", err, tt.want)
			}
		})
	}
}

// TestEXDefaults_ThePFKeyRowsAnswerFourBytes is the same fact seen from the
// answer side, and it is what core/transport's cross-check compares against the
// codec's own inventory.
func TestEXDefaults_ThePFKeyRowsAnswerFourBytes(t *testing.T) {
	defaults := EXDefaults()
	for _, addr := range pfKeyAddresses {
		if got, want := defaults[addr], strings.Repeat("0", pfKeyWidth); got != want {
			t.Errorf("menu %s answers %q, want %q (990:1747-1748)", addr, got, want)
		}
	}
	// And the leg really does read three there, so the correction is doing
	// work rather than agreeing with what was already in the CSV.
	rows, err := parseB(transcriptionB990S)
	if err != nil {
		t.Fatalf("parseB: %v", err)
	}
	corrected := 0
	for _, r := range rows {
		for _, addr := range pfKeyAddresses {
			if r.wire() == addr {
				if r.width != pfKeyLegWidth {
					t.Errorf("transcription B carries width %d at %s; ruling R-B records %d there", r.width, addr, pfKeyLegWidth)
				}
				corrected++
			}
		}
	}
	if corrected != len(pfKeyAddresses) {
		t.Errorf("found %d of the %d PF rows in transcription B", corrected, len(pfKeyAddresses))
	}
}

// TestRecordLines pins the physical-line ledger the error messages depend on,
// against the case that makes the obvious arithmetic wrong: a quoted cell
// carrying a newline. This chart's names are long and parenthesised — "Operating
// Band (High/Low & Shift/Width Controls)" is the shape that acquires a comma in
// an editorial pass — so the reader must be able to name the line a bad row
// really sits on.
//
// It is internal/fakets890's recordLines and its test, COPIED. Neither fake may
// grow a third variant of a machinery both need and neither may share (doc.go,
// THE HARD RULE).
func TestRecordLines(t *testing.T) {
	data := []byte("p1,p2,p3,name,digits,text\n0,00,00,\"Two\nlines\",3,0\n0,00,01,Plain,3,0\n")
	got := recordLines(data)
	want := []int{1, 2, 4}
	if len(got) != len(want) {
		t.Fatalf("recordLines = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("recordLines = %v, want %v", got, want)
		}
	}
}
