// SPDX-License-Identifier: GPL-3.0-or-later

package extable

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// ft710Table2Rows re-parses core/cat/table2.csv against the FT-710 profile,
// the shared fixture every test below joins WriteObserved maps against.
func ft710Table2Rows(t *testing.T) []Row {
	t.Helper()
	data, err := os.ReadFile("../../core/cat/table2.csv")
	if err != nil {
		t.Fatalf("reading table2.csv: %v", err)
	}
	rows, err := ParseCSV(FT710Profile(), data)
	if err != nil {
		t.Fatalf("ParseCSV(table2.csv): %v", err)
	}
	return rows
}

// TestRenderWriteGo_PartialJoinNeverRequiresSetEquality is the plan's own
// named test for (b2): unlike RenderGo's ObservationsRequired regime,
// RenderWriteGo must accept an EMPTY, a PARTIAL, and a FULL observation map
// over the same 296 rows — Session W characterises the 234 admitted
// addresses incrementally, over hours at the radio, and a partial sweep
// must never fail the whole generation.
func TestRenderWriteGo_PartialJoinNeverRequiresSetEquality(t *testing.T) {
	rows := ft710Table2Rows(t)

	full := make(map[string]WriteObserved, len(rows))
	for _, r := range rows {
		full[addrKey(r)] = WriteObserved{SetWidth: 3, SetShape: "numeric"}
	}
	partial := map[string]WriteObserved{
		addrKey(rows[0]):           {SetWidth: 3, SetShape: "numeric"},
		addrKey(rows[len(rows)-1]): {SetWidth: 4, SetShape: "signed"},
	}

	for _, tc := range []struct {
		name     string
		observed map[string]WriteObserved
	}{
		{"empty (nil)", nil},
		{"partial", partial},
		{"full", full},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := RenderWriteGo(rows, tc.observed)
			if err != nil {
				t.Fatalf("RenderWriteGo() with a %s observation map: %v", tc.name, err)
			}
			if got := strings.Count(string(out), "{Addr: EXAddress{"); got != len(rows) {
				t.Errorf("RenderWriteGo() emitted %d items, want %d (all of table2.csv, admitted or not)", got, len(rows))
			}
		})
	}
}

// addrKey renders r's address in ParseWriteObservedCSV's own six-digit
// AddressTriple key shape.
func addrKey(r Row) string {
	return fmt.Sprintf("%02d%02d%02d", r.P1, r.P2, r.P3)
}

// TestProfile_WriteObservedCSVEmptyForNonFT710 is the plan's own named test:
// WriteObservedCSV is the write-descriptor table's own optional input, and
// only "ft710write" — the one profile whose Renderer is RenderWrite — may
// carry it.
func TestProfile_WriteObservedCSVEmptyForNonFT710(t *testing.T) {
	for _, np := range RegisteredProfiles() {
		if np.Name == "ft710write" {
			if np.Profile.WriteObservedCSV == "" {
				t.Errorf("%s: WriteObservedCSV is empty, want table2-write-observed.csv", np.Name)
			}
			continue
		}
		if np.Profile.WriteObservedCSV != "" {
			t.Errorf("%s: WriteObservedCSV = %q, want empty — only ft710write declares a write-descriptor table", np.Name, np.Profile.WriteObservedCSV)
		}
	}
}
