// SPDX-License-Identifier: GPL-3.0-or-later

package extable

import (
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestParseP4Domain_Fixtures pins the named cases from the milestone plan
// (.superpowers/sdd/2026-09-19-v1110-settings-write/reviews/plan.md,
// "(b1) P4 legend parser") — one per shape the parser has to tell apart.
func TestParseP4Domain_Fixtures(t *testing.T) {
	cases := []struct {
		name string
		p4   string
		want Domain
	}{
		{
			// table2.csv:57 USB MOD GAIN — pure range, no P4 parenthetical.
			// Not table2.csv:47 AGC MID DELAY (plan m1): that one is the
			// stepped-range fixture below, same shape as :46.
			name: "pure range, no parenthetical",
			p4:   "000 - 100",
			want: Domain{Lo: 0, Hi: 100, Step: 1},
		},
		{
			// table2.csv:46 AGC FAST DELAY — stepped range, wire form and
			// step given in the P4 parenthetical.
			name: "stepped range",
			p4:   "20 - 4000 msec (P4= 0020 - 4000, 20 msec/step)",
			want: Domain{Lo: 20, Hi: 4000, Step: 20},
		},
		{
			// table2.csv:201 PRMTRC EQ1 FREQ — hybrid: Codes={0} (OFF)
			// plus the range 1-7, whose bounds are the two CODES flanking
			// the hyphen (01, 07), not the labels after each colon
			// (100 Hz, 700 Hz) — the opposite reading from the stepped
			// fixture above, where the numbers flanking the hyphen ARE the
			// range.
			name: "hybrid: sentinel code plus range",
			p4:   "00 : OFF 01: 100 Hz - 07: 700 Hz (100 Hz steps)",
			want: Domain{Codes: []int{0}, Lo: 1, Hi: 7, Step: 1},
		},
		{
			// table2.csv:55 TX BPF SEL — enum with hyphenated labels, the
			// hardest case: each code's OWN label contains a hyphenated
			// range ("50 - 3050"), which carries no colon of its own and
			// so is never mistaken for a code boundary or a hybrid link.
			name: "enum with hyphenated labels",
			p4:   "0: 50 - 3050 1: 100 - 2900 2: 200 - 2800 3: 300 - 2700 4: 400 - 2600 (Hz)",
			want: Domain{Codes: []int{0, 1, 2, 3, 4}},
		},
		{
			// table2.csv:43 AF TREBLE GAIN — one of the 26 sign-bearing
			// addresses (spec §2): -00/+00 are the same wire value under
			// two spellings, so only the outer bounds are captured.
			name: "signed range",
			p4:   "-20 - -00 (or +00) - +10 (P4 = -20 - -00 or +00 - +10)",
			want: Domain{Lo: -20, Hi: 10, Step: 1, Signed: true},
		},
		{
			// table2.csv:187-192 MIC P1/P2/P3/P4/UP/DOWN — all six share
			// this legend (plan m3: the fixture is the whole 187-192
			// range, not just 190-192, three of six). A pure 21-code enum,
			// except the manual's own "17 ATT" (table2.csv:191) drops the
			// colon every other entry has — see parseEnumOrHybrid's doc
			// comment: 17 is swallowed into 16's label text rather than
			// recognised as its own code. Safe direction (Contains then
			// refuses a legal value rather than admitting an illegal one),
			// pinned here so a parser change doesn't move it unnoticed.
			name: "enum with a colonless manual typo (17 ATT)",
			p4:   "00:LOCK 01:QMB 02:A/B 03:V/M 04:TUNER 05:VOX/MOX 06:MODE 07:ZIN_SPOT 08:SPLIT 09:FINE 10:NAR 11:NB 12:DNR 13:FREQ UP 14:FREQ DOWN 15:BAND UP 16:BAND DOWN 17 ATT 18:IPO 19:DNF 20:AGC",
			want: Domain{Codes: []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 18, 19, 20}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseP4Domain(tc.p4)
			if !ok {
				t.Fatalf("ParseP4Domain(%q) ok = false, want true", tc.p4)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseP4Domain(%q) = %+v, want %+v", tc.p4, got, tc.want)
			}
		})
	}
}

// signedGolden is the exact 26 sign-bearing addresses (spec §2's ledger,
// grep -c "(or " core/cat/table2.csv == 26): every one must parse with
// Signed: true, so a parser regression that silently drops the sign never
// ships quietly.
var signedGolden = [][3]int{
	{1, 1, 1}, {1, 1, 2}, {1, 1, 3},
	{1, 2, 1}, {1, 2, 2}, {1, 2, 3},
	{1, 3, 1}, {1, 3, 2}, {1, 3, 3},
	{1, 4, 1}, {1, 4, 2}, {1, 4, 3},
	{1, 5, 1}, {1, 5, 2}, {1, 5, 3},
	{2, 1, 1}, {2, 1, 2}, {2, 1, 3},
	{3, 1, 18},
	{3, 2, 5},
	{3, 3, 3}, {3, 3, 6}, {3, 3, 9},
	{3, 3, 12}, {3, 3, 15}, {3, 3, 18},
}

// admittedUnparseableGolden is the known-hard remainder: P4 legends among
// the 234 admitted addresses that ParseP4Domain refuses. It is empty today
// — every admitted legend parses — and is asserted exactly empty below, so
// a parser change that starts refusing legends fails the build loudly
// (spec §10.3, plan risk 3) rather than quietly shrinking the write set.
var admittedUnparseableGolden = [][3]int{}

// TestParseP4Domain_AdmittedAddresses feeds every P4 legend for the 234
// admitted addresses (task (a)'s exdenylist.go predicates plus exHeld)
// through ParseP4Domain and checks the result against
// admittedUnparseableGolden.
func TestParseP4Domain_AdmittedAddresses(t *testing.T) {
	data, err := os.ReadFile("../../core/cat/table2.csv")
	if err != nil {
		t.Fatalf("reading table2.csv: %v", err)
	}
	rows, err := ParseCSV(FT710Profile(), data)
	if err != nil {
		t.Fatalf("ParseCSV(table2.csv): %v", err)
	}

	held := map[[3]int]bool{{1, 5, 16}: true, {3, 1, 12}: true, {3, 1, 13}: true, {3, 1, 14}: true}
	deniedNames := map[string]bool{
		"CAT-1 RATE": true, "CAT-1 TIME OUT TIMER": true, "CAT-1 CAT-3 STOP BIT": true,
		"CAT-2 RATE": true, "CAT-2 TIME OUT TIMER": true,
		"CAT-3 RATE": true, "CAT-3 TIME OUT TIMER": true,
		"TUN/LIN PORT SELECT": true, "TUNER TYPE SELECT": true,
		"TX TIME OUT TIMER": true, "MOD SOURCE": true,
	}

	var unparseable [][3]int
	var admittedCount, signedCount int
	signedSeen := map[[3]int]bool{}
	for _, r := range rows {
		addr := [3]int{r.P1, r.P2, r.P3}
		txSafetyDenied := r.P1 == 3 && r.P2 == 4 && (r.P3 == 1 || r.P3 == 2 || r.P3 == 3 || r.P3 == 4 || r.P3 == 6 || r.P3 == 7)
		if held[addr] || deniedNames[r.Name] || r.Text || txSafetyDenied || containsAllKeying(r.P4) {
			continue
		}
		admittedCount++

		d, ok := ParseP4Domain(r.P4)
		if !ok {
			unparseable = append(unparseable, addr)
			continue
		}
		if d.Signed {
			signedCount++
			signedSeen[addr] = true
		}
	}

	if admittedCount != 234 {
		t.Fatalf("admitted count = %d, want 234 — this test's own denylist mirror has drifted from core/cat/exdenylist.go", admittedCount)
	}

	sort.Slice(unparseable, func(i, j int) bool {
		if unparseable[i][0] != unparseable[j][0] {
			return unparseable[i][0] < unparseable[j][0]
		}
		if unparseable[i][1] != unparseable[j][1] {
			return unparseable[i][1] < unparseable[j][1]
		}
		return unparseable[i][2] < unparseable[j][2]
	})
	if len(unparseable) != len(admittedUnparseableGolden) {
		t.Fatalf("unparseable admitted addresses = %v (%d), want admittedUnparseableGolden %v (%d)",
			unparseable, len(unparseable), admittedUnparseableGolden, len(admittedUnparseableGolden))
	}
	for i, addr := range unparseable {
		if addr != admittedUnparseableGolden[i] {
			t.Errorf("unparseable[%d] = %v, want %v (admittedUnparseableGolden)", i, addr, admittedUnparseableGolden[i])
		}
	}

	if signedCount != len(signedGolden) {
		t.Errorf("signed count = %d, want %d", signedCount, len(signedGolden))
	}
	for _, addr := range signedGolden {
		if !signedSeen[addr] {
			t.Errorf("address %v is in signedGolden but did not parse Signed: true", addr)
		}
	}
}

// containsAllKeying mirrors exKeyingDenied (core/cat/exdenylist.go) without
// importing core/cat — internal/extable stays dependency-free of core/cat
// (extable.go's own doc comment; ex_crosscheck_test.go,
// internal/fakeradio/ex.go:161-168 protect the seam the other way).
func containsAllKeying(p4 string) bool {
	for _, sub := range []string{"RTS", "DTR", "DAKY"} {
		if !strings.Contains(p4, sub) {
			return false
		}
	}
	return true
}
