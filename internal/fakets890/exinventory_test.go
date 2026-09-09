// SPDX-License-Identifier: GPL-3.0-or-later

package fakets890

import (
	"strings"
	"testing"
)

// The four addresses the PRODUCTION inventory excludes and transcription B
// keeps, written here as literals so that this file and exinventory.go's own
// list are two statements of the same fact rather than one (890:2273-2280).
var excludedHere = []string{"10023", "10024", "10025", "10026"}

// TestEXDefaults_OmitsExactlyTheFourExcludedAddresses is the pin the plan asks
// for by name: the projection omits those four AND NOTHING ELSE.
//
// It is written as a set difference against transcription B's OWN addresses,
// re-parsed here, rather than against a count: a count-only assertion is
// satisfied by an inventory that dropped the wrong four rows, which is the
// falsification internal/extable's own profile comment names on the codec's
// side of this chart.
func TestEXDefaults_OmitsExactlyTheFourExcludedAddresses(t *testing.T) {
	rows, err := parseB(transcriptionB890S)
	if err != nil {
		t.Fatalf("parseB(the committed transcription B): %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("transcription B parsed to zero rows — every assertion below would pass vacuously")
	}

	got := EXDefaults()
	var missing []string
	for _, r := range rows {
		if _, ok := got[r.wire()]; !ok {
			missing = append(missing, r.wire())
		}
	}
	if strings.Join(missing, ",") != strings.Join(excludedHere, ",") {
		t.Errorf("transcription B's addresses absent from the projection are %v, want exactly %v (890:2273-2280) — "+
			"the production inventory excludes those four by address and the fake's own projection must exclude the same four, "+
			"neither more (the cross-check would report a missing address) nor fewer (it would report four extra)", missing, excludedHere)
	}

	// The other direction: nothing is in the projection that transcription B
	// does not carry.
	inB := map[string]bool{}
	for _, r := range rows {
		inB[r.wire()] = true
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
	rows, err := parseB(transcriptionB890S)
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
			continue // one of the four excluded addresses
		}
		width := r.width
		if pf[r.wire()] {
			// Ruling R-B: the leg reads three here and the book prints four
			// (890:1918-1919). TestEXDefaults_ThePFKeyRowsAnswerFourBytes is
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
// four the exclusion removes and the seventeen ruling R-B corrects. A
// synthetic chart missing them is refused, which is the point — so the
// projection proofs that are not ABOUT those refusals supply them.
func ruledRows() []string {
	var out []string
	for _, addr := range excludedHere {
		out = append(out, addr[:1]+","+addr[1:3]+","+addr[3:]+",Does not correspond to a command,3,0")
	}
	for _, addr := range pfKeyAddresses {
		out = append(out, addr[:1]+","+addr[1:3]+","+addr[3:]+",A PF Key Assignment,3,0")
	}
	return out
}

// bWithRuled renders a synthetic chart carrying rows plus every row the
// rulings need.
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
			bWith("00,00,00,Color Display Pattern,3,0"),
			"p1 cell",
		},
		{
			"a menu type outside the two printed values",
			bWith("2,00,00,Color Display Pattern,3,0"),
			"p1 cell",
		},
		{
			"a one-digit category cell — a lost leading zero",
			bWith("0,0,00,Color Display Pattern,3,0"),
			"p2 cell",
		},
		{
			"a three-digit item cell",
			bWith("0,00,000,Color Display Pattern,3,0"),
			"p3 cell",
		},
		{
			"an empty name cell — the signature of a misparsed row",
			bWith("0,00,00, ,3,0"),
			"empty name cell",
		},
		{
			"a digits cell that is not a number",
			bWith("0,00,00,Color Display Pattern,three,0"),
			"digits cell",
		},
		{
			"a digits cell wider than any width this chart prints",
			bWith("0,00,00,Color Display Pattern,16,0"),
			"outside 1-15",
		},
		{
			"a zero-width digits cell",
			bWith("0,00,00,Color Display Pattern,0,0"),
			"outside 1-15",
		},
		{
			"a third value in the text column",
			bWith("0,00,00,Color Display Pattern,3,2"),
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

// TestProjectWidths_RefusesADuplicateAddress. A repeated address would
// silently overwrite one row's width with another's, which is the one defect
// a width comparison against the codec could not localise — the table would be
// one row short and the missing address would be reported as absent rather
// than as duplicated.
func TestProjectWidths_RefusesADuplicateAddress(t *testing.T) {
	rows, err := parseB(bWithRuled(
		"0,00,00,Color Display Pattern,3,0",
		"0,00,00,Color Display Pattern,3,0",
	))
	if err != nil {
		t.Fatalf("parseB: %v", err)
	}
	if _, err := projectWidths(rows); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Errorf("projectWidths on a duplicated address returned %v, want a refusal naming the repeat", err)
	}
}

// TestProjectWidths_RefusesAMissingExcludedAddress. The exclusion is applied
// BY ADDRESS, and it is only meaningful if the row it removes was there: a
// transcription B that had lost one of the four is no longer the artefact the
// codec's side excludes from, and the cross-check would then agree for the
// wrong reason.
func TestProjectWidths_RefusesAMissingExcludedAddress(t *testing.T) {
	rows, err := parseB(bWith("0,00,00,Color Display Pattern,3,0"))
	if err != nil {
		t.Fatalf("parseB: %v", err)
	}
	_, err = projectWidths(rows)
	if err == nil || !strings.Contains(err.Error(), "10023") {
		t.Errorf("projectWidths on a B missing the excluded rows returned %v, want a refusal naming the first absent address", err)
	}
}

// TestProjectWidths_ExcludesTheFourItWasGiven is the exclusion's own positive
// proof on a synthetic chart, so that the arm is known to fire rather than
// merely to be present.
func TestProjectWidths_ExcludesTheFourItWasGiven(t *testing.T) {
	rows, err := parseB(bWithRuled("0,00,00,Color Display Pattern,3,0"))
	if err != nil {
		t.Fatalf("parseB: %v", err)
	}
	widths, err := projectWidths(rows)
	if err != nil {
		t.Fatalf("projectWidths: %v", err)
	}
	if want := 1 + len(pfKeyAddresses); len(widths) != want {
		t.Fatalf("projected %d addresses (%v), want %d — the four excluded rows must not survive", len(widths), widths, want)
	}
	if _, ok := widths["00000"]; !ok {
		t.Errorf("projected %v, want the one non-excluded ordinary address 00000", widths)
	}
	for _, addr := range excludedHere {
		if _, ok := widths[addr]; ok {
			t.Errorf("projected %s, which the exclusion must remove", addr)
		}
	}
}

// TestProjectWidths_AppliesRulingRBToThePFKeyRows is the second correction's
// positive proof: the seventeen PF rows arrive from the leg carrying three and
// leave the projection carrying the four the book prints — "PF key settings
// use 4 digits (refer to the PF Key assignment ID lists)." (890:1918-1919) —
// while every other row is untouched.
func TestProjectWidths_AppliesRulingRBToThePFKeyRows(t *testing.T) {
	rows, err := parseB(bWithRuled("0,00,00,Color Display Pattern,3,0"))
	if err != nil {
		t.Fatalf("parseB: %v", err)
	}
	widths, err := projectWidths(rows)
	if err != nil {
		t.Fatalf("projectWidths: %v", err)
	}
	for _, addr := range pfKeyAddresses {
		if got := widths[addr]; got != pfKeyWidth {
			t.Errorf("PF key %s projected width %d, want %d (890:1918-1919, ruling R-B)", addr, got, pfKeyWidth)
		}
	}
	if got := widths["00000"]; got != 3 {
		t.Errorf("the ordinary row 00000 projected width %d, want 3 — the correction must touch the PF rows and nothing else", got)
	}
}

// TestProjectWidths_RefusesAChangedOrMissingPFRow. The correction is
// meaningful only against the divergence the ruling records: a leg that no
// longer reads three there has MOVED, which is an arbitration rather than a
// correction to apply blind. This is ruling R-B's both-directions check,
// brought to this side.
func TestProjectWidths_RefusesAChangedOrMissingPFRow(t *testing.T) {
	tests := []struct {
		name string
		rows []string
		want string
	}{
		{
			"a PF row missing altogether",
			append([]string{"0,00,00,Color Display Pattern,3,0"}, ruledRows()[:len(excludedHere)+1]...),
			"is not in this transcription",
		},
		{
			"a PF row the leg no longer reads as three",
			func() []string {
				rows := append([]string{"0,00,00,Color Display Pattern,3,0"}, ruledRows()...)
				rows[1+len(excludedHere)] = "0,00,15,PF A: Key Assignment,4,0"
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
// answer side, and it is what core/transport's cross-check compares against
// the codec's own inventory.
func TestEXDefaults_ThePFKeyRowsAnswerFourBytes(t *testing.T) {
	defaults := EXDefaults()
	for _, addr := range pfKeyAddresses {
		if got, want := defaults[addr], strings.Repeat("0", pfKeyWidth); got != want {
			t.Errorf("menu %s answers %q, want %q (890:1918-1919)", addr, got, want)
		}
	}
	// And the leg really does read three there, so the correction is doing
	// work rather than agreeing with what was already in the CSV.
	rows, err := parseB(transcriptionB890S)
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
// carrying a newline. Several of this chart's own names carry commas
// ("Tuning Control :" does not, but "Frequency Rounding Off (Multi/ Channel
// Control)" is the shape that acquires one), so the reader must be able to
// name the line a bad row really sits on.
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
