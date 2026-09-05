// SPDX-License-Identifier: GPL-3.0-or-later

package cat_test

import (
	"encoding/csv"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
)

// observedPath is the M8c hardware READ-observation artefact under test.
const observedPath = "table2-observed.csv"

// observedHeader is that artefact's provenance block, pinned byte-exactly.
// Pinning it is a PRIVACY control, not cosmetics: the values-free checks
// below validate DATA rows only, because encoding/csv skips '#' lines
// entirely — so without this pin, a comment line would be a place a
// captured setting value could hide unnoticed.
const observedHeader = `# FT-710 EX read-characterisation observations (M8c, 24/07/2026).
#
# HARDWARE EVIDENCE — NOT a manual transcription, and READ DIRECTION ONLY.
# The manual's own Table 2 transcription lives in table2.csv and is never
# edited from hardware. This file records what one radio ANSWERED to EX
# READ requests; it says nothing about EX Set behaviour, which was not
# probed and is M8e/M8f's question.
#
# Source: two successive full passive EX sweeps ("rigprog read --settings")
#   of one UK FT-710, CAT ID 0800, firmware V01-12, in one configuration,
#   24 July 2026, controller-driven, read-only. Both sweeps returned all
#   296 addresses "known" and were byte-identical to each other. Session
#   record: docs/hardware-notes.md.
#
# PRIVACY: this file carries NO setting values — only each address's
#   observed P4 wire width and shape class. Raw captures stay in
#   docs/fixtures-private/ (never committed).
#
# Regenerate with:
#   go run ./internal/extable/observe -capture <private capture.json> \
#     -csv core/cat/table2.csv -out core/cat/table2-observed.csv
#
# Columns: p1,p2,p3,observed_read_width,observed_read_shape
#   observed_read_width  bytes of raw P4 answered to a READ (a sign counts)
#   observed_read_shape  numeric | signed (leading '+'/'-') | text (12-byte free text)
`

// readObservedRows returns the artefact's data records with the field
// count enforced by the reader itself, so no test below can index a short
// or over-long record. Failure text names ADDRESSES only — never a whole
// record — so even a malformed artefact cannot spill content into a test
// log.
func readObservedRows(t *testing.T) [][]string {
	t.Helper()
	data, err := os.ReadFile(observedPath)
	if err != nil {
		t.Fatalf("reading %s: %v", observedPath, err)
	}
	r := csv.NewReader(strings.NewReader(string(data)))
	r.Comment = '#'
	r.FieldsPerRecord = 5
	recs, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parsing %s (every data row must have exactly 5 fields): %v", observedPath, err)
	}
	return recs
}

// TestObservedCSV_HeaderIsPinned fails on any edit to the provenance
// block, including one that smuggles content into a comment line.
func TestObservedCSV_HeaderIsPinned(t *testing.T) {
	data, err := os.ReadFile(observedPath)
	if err != nil {
		t.Fatalf("reading %s: %v", observedPath, err)
	}
	if !strings.HasPrefix(string(data), observedHeader) {
		t.Fatal("table2-observed.csv's comment header does not match the pinned provenance block byte-for-byte")
	}
	body := strings.TrimPrefix(string(data), observedHeader)
	for i, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "#") {
			t.Errorf("data line %d is a comment: comments outside the pinned header are not permitted", i+1)
		}
	}
}

// TestObservedCSV_CoversEveryInventoryAddressExactlyOnce proves the
// artefact is a complete, duplicate-free observation of the Table 2
// inventory: the M8c sweeps read all 296 addresses, so every inventory
// member must appear, and nothing else may.
func TestObservedCSV_CoversEveryInventoryAddressExactlyOnce(t *testing.T) {
	recs := readObservedRows(t)
	items := cat.FT710.EXItems()
	if len(recs) != len(items) {
		t.Fatalf("observed rows = %d, inventory items = %d — the artefact must cover the inventory exactly", len(recs), len(items))
	}
	seen := make(map[string]bool, len(recs))
	for _, rec := range recs {
		addr := rec[0] + rec[1] + rec[2]
		if seen[addr] {
			t.Errorf("address %s appears more than once", addr)
		}
		seen[addr] = true
	}
	for _, it := range items {
		if !seen[cat.FT710.EXWire(it.Addr)] {
			t.Errorf("inventory address %s has no observation row", cat.FT710.EXWire(it.Addr))
		}
	}
}

// TestObservedCSV_IsValuesFree is the privacy guard: every field is
// constrained to a closed vocabulary — two-digit address components, a
// width in 1..12, and one of three shape words — so no captured value can
// occupy a data row. Failure text names the offending address and field
// position only.
func TestObservedCSV_IsValuesFree(t *testing.T) {
	shapes := map[string]bool{"numeric": true, "signed": true, "text": true}
	for _, rec := range readObservedRows(t) {
		addr := rec[0] + rec[1] + rec[2]
		for i := 0; i < 3; i++ {
			if len(rec[i]) != 2 {
				t.Errorf("address %s: field %d is not a two-digit component", addr, i)
				continue
			}
			if _, err := strconv.Atoi(rec[i]); err != nil {
				t.Errorf("address %s: field %d is not numeric", addr, i)
			}
		}
		width, err := strconv.Atoi(rec[3])
		if err != nil || width < 1 || width > 12 {
			t.Errorf("address %s: observed_read_width is not an integer in 1..12", addr)
		}
		if !shapes[rec[4]] {
			t.Errorf("address %s: observed_read_shape is not one of numeric/signed/text", addr)
		}
	}
}

// TestObservedCSV_ShapeCounts pins the M8c session's own totals (264
// numeric, 26 signed, 6 text = 296), so a regenerated or hand-edited
// artefact cannot drift from the session record in docs/hardware-notes.md.
func TestObservedCSV_ShapeCounts(t *testing.T) {
	counts := map[string]int{}
	for _, rec := range readObservedRows(t) {
		counts[rec[4]]++
	}
	for shape, want := range map[string]int{"numeric": 264, "signed": 26, "text": 6} {
		if counts[shape] != want {
			t.Errorf("shape %q count = %d, want %d (M8c session record)", shape, counts[shape], want)
		}
	}
}

// --- M8c manual corrections artefact (task 48) ---

// correctionsPath is the machine-readable record of manual errors the M8c
// observations settled. It exists because table2-observed.csv can express
// only width and shape: a wrong enum code in the manual's P4 column has no
// home there.
const correctionsPath = "table2-corrections.csv"

// correctionsFile pins table2-corrections.csv byte-for-byte.
//
// Unlike the observation artefact, this file is hand-maintained and has
// two FREE-TEXT columns, so a vocabulary check cannot constrain it: a
// callsign, a preset name or any other captured value could be typed into
// manual_text, hardware_evidence or a comment line and satisfy every
// structural rule. Pinning the whole file is therefore the privacy
// control. It is two rows and a header — changing it deliberately means
// updating this constant in the same commit, which is exactly the review
// checkpoint the pin exists to force.
const correctionsFile = `# FT-710 Table 2 corrections settled by hardware observation (M8c, 24/07/2026).
#
# READ-DIRECTION EVIDENCE ONLY. This file records places where the MANUAL's
# printed chart is demonstrably wrong, as shown by what one radio ANSWERED
# to an EX READ. It is not a Set allowlist, it confers no write capability,
# and it says nothing about EX Set frames, which the M8c session did not
# probe (Set policy is M8e's to define and M8f's to verify).
#
# It exists because table2-observed.csv can only express per-address width
# and shape: a wrong ENUM CODE in the manual's P4 column has no home there,
# and would otherwise survive only as prose — where a later milestone could
# consume the known-bad chart text without ever seeing the correction.
#
# table2.csv itself is never edited from hardware: it is a transcription of
# the manual, typos included, and its provenance depends on staying that way.
#
# Scope: two successive full passive EX sweeps of one UK FT-710, CAT ID
#   0800, firmware V01-12, in one configuration, 24 July 2026,
#   controller-driven, read-only. Session record: docs/hardware-notes.md.
#
# PRIVACY: the operator consented (25/07/2026) to publishing the two
#   numeric values quoted below, both protocol-relevant and personally
#   innocuous. No other captured value appears in any committed file, and
#   no text-item value (MY CALL, PRESET NAME) ever will.
#
# Columns: p1,p2,p3,kind,manual_text,hardware_evidence
#   kind: enum_code | field_width
01,03,21,field_width,"Digits column prints 2","answered a 3-byte P4: EX010321012;"
01,05,16,enum_code,"1: 170 Hz 1: 200 Hz 2: 425 Hz 3: 850 Hz","answered 0; a 0 code therefore exists and the printed duplicate 1: is wrong. WHICH label maps to 0 is NOT established: no front-panel check of this item was made"
`

// TestCorrectionsCSV_IsPinnedByteForByte is the privacy and review gate
// for the hand-maintained corrections artefact — see correctionsFile.
func TestCorrectionsCSV_IsPinnedByteForByte(t *testing.T) {
	data, err := os.ReadFile(correctionsPath)
	if err != nil {
		t.Fatalf("reading %s: %v", correctionsPath, err)
	}
	if string(data) != correctionsFile {
		t.Errorf("table2-corrections.csv does not match its pinned content byte-for-byte.\n"+
			"If the change is intended, update correctionsFile in this test in the SAME commit — "+
			"and check the new content carries no captured setting value. (committed %d bytes, pinned %d bytes)",
			len(data), len(correctionsFile))
	}
}

// readCorrectionRows returns the corrections artefact's data records with
// the field count enforced by the reader.
func readCorrectionRows(t *testing.T) [][]string {
	t.Helper()
	r := csv.NewReader(strings.NewReader(correctionsFile))
	r.Comment = '#'
	r.FieldsPerRecord = 6
	recs, err := r.ReadAll()
	if err != nil {
		t.Fatalf("parsing the pinned corrections content (every data row must have exactly 6 fields): %v", err)
	}
	return recs
}

// TestCorrectionsCSV_AddressesAreInventoryMembers proves every correction
// names a real Table 2 address: a correction for an address that does not
// exist would be evidence about nothing.
func TestCorrectionsCSV_AddressesAreInventoryMembers(t *testing.T) {
	members := map[string]bool{}
	for _, it := range cat.FT710.EXItems() {
		members[cat.FT710.EXWire(it.Addr)] = true
	}
	seen := map[string]bool{}
	for _, rec := range readCorrectionRows(t) {
		addr := rec[0] + rec[1] + rec[2]
		if !members[addr] {
			t.Errorf("correction names address %s, which is not in the Table 2 inventory", addr)
		}
		if seen[addr] {
			t.Errorf("address %s has more than one correction row", addr)
		}
		seen[addr] = true
	}
}

// TestCorrectionsCSV_KindVocabularyIsClosed keeps the artefact
// machine-readable: a future consumer (M8e's typed descriptors) must be
// able to switch on kind exhaustively. Failure text names the address
// only — never the offending field, which is untrusted free text.
func TestCorrectionsCSV_KindVocabularyIsClosed(t *testing.T) {
	kinds := map[string]bool{"enum_code": true, "field_width": true}
	for _, rec := range readCorrectionRows(t) {
		if !kinds[rec[3]] {
			t.Errorf("address %s: correction kind is not one of enum_code/field_width", rec[0]+rec[1]+rec[2])
		}
	}
}

// TestCorrectionsCSV_WidthCorrectionAgreesWithTheObservations proves the
// two artefacts cannot contradict each other: every field_width correction
// must name an address whose observed read width actually differs from the
// manual's Digits column, and every such deviation must be recorded here.
func TestCorrectionsCSV_WidthCorrectionAgreesWithTheObservations(t *testing.T) {
	recorded := map[string]bool{}
	for _, rec := range readCorrectionRows(t) {
		if rec[3] == "field_width" {
			recorded[rec[0]+rec[1]+rec[2]] = true
		}
	}
	for _, it := range cat.FT710.EXItems() {
		want := it.Digits
		if it.Text {
			want = 12
		}
		deviates := it.ObservedReadWidth != want
		if deviates && !recorded[cat.FT710.EXWire(it.Addr)] {
			t.Errorf("%s (%s) deviates from the manual (%d vs observed %d) but has no field_width correction", cat.FT710.EXWire(it.Addr), it.Name, want, it.ObservedReadWidth)
		}
		if !deviates && recorded[cat.FT710.EXWire(it.Addr)] {
			t.Errorf("%s (%s) has a field_width correction but its observed read width matches the manual", cat.FT710.EXWire(it.Addr), it.Name)
		}
	}
}

// TestObservedCSV_IsCanonicallyRendered pins the artefact's exact shape as
// the derivation tool emits it: the pinned header, then one data row per
// address in ascending address order, no blank lines, no trailing
// whitespace. Without this a reordered or whitespace-drifted file would
// still satisfy every semantic test while no longer being what the tool
// produces — and the difference between "derived" and "hand-edited" is the
// whole basis for trusting it.
func TestObservedCSV_IsCanonicallyRendered(t *testing.T) {
	data, err := os.ReadFile(observedPath)
	if err != nil {
		t.Fatalf("reading %s: %v", observedPath, err)
	}
	body := strings.TrimPrefix(string(data), observedHeader)
	if !strings.HasSuffix(body, "\n") {
		t.Error("artefact does not end with a newline")
	}
	lines := strings.Split(strings.TrimSuffix(body, "\n"), "\n")

	prev := ""
	for i, line := range lines {
		if line == "" {
			t.Errorf("data line %d is blank: the tool never emits blank lines", i+1)
			continue
		}
		if strings.TrimRight(line, " \t") != line {
			t.Errorf("data line %d has trailing whitespace", i+1)
		}
		fields := strings.Split(line, ",")
		if len(fields) != 5 {
			t.Errorf("data line %d does not have exactly 5 comma-separated fields", i+1)
			continue
		}
		addr := fields[0] + fields[1] + fields[2]
		if addr <= prev {
			t.Errorf("data line %d (address %s) is out of ascending order: the tool sorts by address", i+1, addr)
		}
		prev = addr
	}
}
