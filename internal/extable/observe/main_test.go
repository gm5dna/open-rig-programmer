// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// canary is a value that must never survive derivation. A real capture's
// text items carry the operator's callsign and preset names; this stands
// in for them so the privacy guarantee is executable rather than merely
// reviewed by eye — a future diagnostic that printed a value would fail
// here instead of leaking on the next run.
const canary = "CANARY-SECRE" // 12 bytes: a text item's exact wire width

// manualCSV reads the real Table 2 transcription the tool joins against.
func manualCSV(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../../../core/cat/table2.csv")
	if err != nil {
		t.Fatalf("reading table2.csv: %v", err)
	}
	return data
}

// syntheticCapture builds a complete, all-known capture covering every
// inventory address: text items carry the canary, 3-byte numeric items
// carry an explicitly signed value (so signed classification is genuinely
// exercised, not merely assumed), and every other numeric item carries a
// digits-only value of the manual's own width.
func syntheticCapture(t *testing.T, csv []byte) []byte {
	t.Helper()
	rows, err := extable.ParseCSV(extable.FT710Profile(), csv)
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	type entry struct {
		ID    string `json:"id"`
		Value string `json:"value"`
		State string `json:"state"`
	}
	var entries []entry
	for _, r := range rows {
		id := fmt.Sprintf("%02d%02d%02d", r.P1, r.P2, r.P3)
		switch {
		case r.Text:
			entries = append(entries, entry{ID: id, Value: canary, State: "known"})
		case r.Digits == 3:
			entries = append(entries, entry{ID: id, Value: "+" + strings.Repeat("7", r.Digits-1), State: "known"})
		default:
			entries = append(entries, entry{ID: id, Value: strings.Repeat("7", r.Digits), State: "known"})
		}
	}
	doc := map[string]any{"menus": map[string]any{"entries": entries}}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshalling synthetic capture: %v", err)
	}
	return data
}

// TestDerive_NeverEmitsValues proves the artefact carries no captured
// content: a capture whose every text item is the canary derives to bytes
// that contain neither the canary nor any fragment of it.
func TestDerive_NeverEmitsValues(t *testing.T) {
	csv := manualCSV(t)
	got, err := derive(syntheticCapture(t, csv), csv)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if strings.Contains(string(got), "CANARY") {
		t.Error("derived artefact contains the canary — a captured value escaped into committed output")
	}
	for _, forbidden := range []string{"7777", canary, strings.TrimRight(canary, "E")} {
		if strings.Contains(string(got), forbidden) {
			t.Errorf("derived artefact contains %q, which came from a capture value", forbidden)
		}
	}
	if !strings.HasPrefix(string(got), header) {
		t.Error("derived artefact does not start with the pinned provenance header")
	}
}

// TestDerive_RejectsUnexpectedValueShapesWithoutQuotingThem proves the
// second half of the guarantee: a value that fails lexical validation
// aborts derivation, and the error names the address WITHOUT echoing the
// value — so a malformed capture cannot leak through an error path either.
func TestDerive_RejectsUnexpectedValueShapesWithoutQuotingThem(t *testing.T) {
	csv := manualCSV(t)
	var doc map[string]any
	if err := json.Unmarshal(syntheticCapture(t, csv), &doc); err != nil {
		t.Fatalf("unmarshalling synthetic capture: %v", err)
	}
	entries := doc["menus"].(map[string]any)["entries"].([]any)
	// 03 01 01 BEEP LEVEL is a plain numeric item; give it a value that is
	// not digits-only and carries the canary.
	for _, e := range entries {
		m := e.(map[string]any)
		if m["id"] == "030101" {
			m["value"] = canary
		}
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshalling mutated capture: %v", err)
	}

	_, err = derive(data, csv)
	if err == nil {
		t.Fatal("derive accepted a non-digits numeric value; want a refusal")
	}
	if !strings.Contains(err.Error(), "030101") {
		t.Errorf("error %q does not name the offending address", err)
	}
	if strings.Contains(err.Error(), "CANARY") {
		t.Errorf("error text quotes the offending value: %q", err)
	}
}

// TestDerive_RefusesIncompleteSweep pins the rule that widths are only
// ever derived from a complete, all-known sweep: a single unavailable
// entry aborts the whole derivation.
func TestDerive_RefusesIncompleteSweep(t *testing.T) {
	csv := manualCSV(t)
	var doc map[string]any
	if err := json.Unmarshal(syntheticCapture(t, csv), &doc); err != nil {
		t.Fatalf("unmarshalling synthetic capture: %v", err)
	}
	entries := doc["menus"].(map[string]any)["entries"].([]any)
	entries[0].(map[string]any)["state"] = "unavailable"
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshalling mutated capture: %v", err)
	}
	if _, err := derive(data, csv); err == nil {
		t.Fatal("derive accepted a capture containing an unavailable entry; want a refusal")
	}
}

// TestDerive_ProducesExactlyTheExpectedBytes is the stronger form of the
// privacy and correctness guarantee: rather than searching the output for
// things that must not appear, it builds the artefact the tool SHOULD
// produce for a known synthetic capture and byte-compares. Anything the
// tool emitted that the expectation does not contain — a stray diagnostic,
// a value, a reordered row — fails here.
func TestDerive_ProducesExactlyTheExpectedBytes(t *testing.T) {
	csv := manualCSV(t)
	rows, err := extable.ParseCSV(extable.FT710Profile(), csv)
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}

	type row struct{ addr, line string }
	var expected []row
	for _, r := range rows {
		addr := fmt.Sprintf("%02d%02d%02d", r.P1, r.P2, r.P3)
		width, shape := r.Digits, "numeric"
		switch {
		case r.Text:
			width, shape = len(canary), "text"
		case r.Digits == 3:
			shape = "signed" // syntheticCapture gives these an explicit sign
		}
		expected = append(expected, row{
			addr: addr,
			line: fmt.Sprintf("%s,%s,%s,%d,%s\n", addr[0:2], addr[2:4], addr[4:6], width, shape),
		})
	}
	sort.Slice(expected, func(i, j int) bool { return expected[i].addr < expected[j].addr })

	var want strings.Builder
	want.WriteString(header)
	for _, e := range expected {
		want.WriteString(e.line)
	}

	got, err := derive(syntheticCapture(t, csv), csv)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if string(got) != want.String() {
		t.Errorf("derived artefact is not byte-identical to the expected rendering (got %d bytes, want %d) — the tool emitted something the expectation does not account for", len(got), want.Len())
	}
}

// TestDerive_RejectsEveryNonDigitNumericWidth complements the canary test
// at each numeric width the manual uses: a value that is not digits-only is
// refused whatever its length, so no synthetic width slips past validation.
func TestDerive_RejectsEveryNonDigitNumericWidth(t *testing.T) {
	csv := manualCSV(t)
	for _, bad := range []string{"X", "1X", "12X", "123X"} {
		t.Run(bad, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal(syntheticCapture(t, csv), &doc); err != nil {
				t.Fatalf("unmarshalling synthetic capture: %v", err)
			}
			for _, e := range doc["menus"].(map[string]any)["entries"].([]any) {
				m := e.(map[string]any)
				if m["id"] == "030101" {
					m["value"] = bad
				}
			}
			data, err := json.Marshal(doc)
			if err != nil {
				t.Fatalf("marshalling mutated capture: %v", err)
			}
			if _, err := derive(data, csv); err == nil {
				t.Errorf("derive accepted the non-digit numeric value %q; want a refusal", bad)
			}
		})
	}
}
