// SPDX-License-Identifier: GPL-3.0-or-later

// Command observe derives core/cat/table2-observed.csv — the M8c hardware
// READ-observation artefact — from a private "rigprog read --settings"
// capture.
//
// PRIVACY: this program never writes a setting value. Per address it emits
// the observed P4 wire width and a shape class only. The capture itself
// (which carries MY CALL and PRESET NAME text) stays in
// docs/fixtures-private/ and is never committed. Error text names the
// offending address, never the value that provoked it.
//
// SCOPE: read direction only. Nothing here describes, implies or enables
// EX Set behaviour, which the M8c session did not probe.
//
// Usage:
//
//	go run ./internal/extable/observe \
//	  -capture docs/fixtures-private/m8c-settings-2026-07-24-run1.json \
//	  -csv core/cat/table2.csv \
//	  -out core/cat/table2-observed.csv
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/gm5dna/open-rig-programmer/internal/extable"
)

// header is the artefact's provenance block. It must stay byte-identical
// to observedHeader in core/cat/exobserved_test.go, which pins it.
const header = `# FT-710 EX read-characterisation observations (M8c, 24/07/2026).
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

// textWidth is the fixed wire width of an EX text item's raw P4 field.
//
// It equals the FT-710 profile's TextWidth today, and that is a COINCIDENCE
// of the FT-710, not a shared definition. The two belong to different
// categories: this constant is evidence-collection policy — what a text
// answer must LOOK like on the wire before this tool will record it — while
// Profile.TextWidth is manual schema — what the Digits column must SAY for a
// text row. core/cat/table2-corrections.csv is the standing proof that the
// two categories can disagree on one radio: TONE FREQ declares two digits in
// the manual and answered three on the wire.
//
// So this is not a missed parameterisation to fold into the profile later.
// Folding it would make the tool structurally incapable of recording a
// text-width correction, which is exactly the class of finding it exists to
// surface. If a future model's two numbers diverge, classify fails closed —
// it refuses the capture rather than summarising a width — and that refusal
// is the intended outcome, not a bug to route around.
const textWidth = 12

// capture is the subset of a codeplug file this tool reads: the menu
// snapshot's entries. Nothing else in the file is parsed, and the Value
// field never leaves this process.
type capture struct {
	Menus struct {
		Entries []struct {
			ID    string `json:"id"`
			Value string `json:"value"`
			State string `json:"state"`
		} `json:"entries"`
	} `json:"menus"`
}

// classify validates a captured raw P4 lexically and returns its shape
// class. Validation is deliberate rather than defensive: a capture that
// does not match one of the three known lexical forms is a finding to
// investigate at the radio, not something to quietly summarise into a
// width. Returned errors never quote the value.
func classify(raw string, isText bool) (string, error) {
	switch {
	case isText:
		if len(raw) != textWidth {
			return "", fmt.Errorf("text item answered %d bytes, want %d", len(raw), textWidth)
		}
		if strings.TrimRight(raw, " ") != strings.TrimSpace(raw) {
			return "", fmt.Errorf("text item is not right-space-padded")
		}
		for _, r := range raw {
			if r < 0x20 || r > 0x7e {
				return "", fmt.Errorf("text item contains a non-printable byte")
			}
		}
		return "text", nil
	case strings.HasPrefix(raw, "+"), strings.HasPrefix(raw, "-"):
		if !allDigits(raw[1:]) {
			return "", fmt.Errorf("signed item is not a sign followed by digits")
		}
		return "signed", nil
	default:
		if !allDigits(raw) {
			return "", fmt.Errorf("numeric item is not digits-only")
		}
		return "numeric", nil
	}
}

// allDigits reports whether s is one or more ASCII digits.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// observation is one address's committed evidence: never a value.
type observation struct {
	addr  string
	shape string
	width int
}

// derive turns a capture and the manual transcription into the artefact's
// bytes. Split out from main so the canary test can exercise it directly.
func derive(captureJSON, manualCSV []byte) ([]byte, error) {
	var cap capture
	if err := json.Unmarshal(captureJSON, &cap); err != nil {
		return nil, fmt.Errorf("parsing capture: %w", err)
	}

	rows, err := extable.ParseCSV(extable.FT710Profile(), manualCSV)
	if err != nil {
		return nil, fmt.Errorf("parsing manual CSV: %w", err)
	}
	isText := make(map[string]bool, len(rows))
	for _, r := range rows {
		isText[fmt.Sprintf("%02d%02d%02d", r.P1, r.P2, r.P3)] = r.Text
	}

	out := make([]observation, 0, len(cap.Menus.Entries))
	seen := make(map[string]bool, len(cap.Menus.Entries))
	for _, e := range cap.Menus.Entries {
		if e.State != "known" {
			return nil, fmt.Errorf("entry %s has state %q — widths are derived only from a complete, all-known sweep", e.ID, e.State)
		}
		text, member := isText[e.ID]
		if !member {
			return nil, fmt.Errorf("captured address %s is not a Table 2 member", e.ID)
		}
		if seen[e.ID] {
			return nil, fmt.Errorf("captured address %s appears twice", e.ID)
		}
		seen[e.ID] = true
		shape, err := classify(e.Value, text)
		if err != nil {
			return nil, fmt.Errorf("address %s: %w", e.ID, err)
		}
		out = append(out, observation{addr: e.ID, shape: shape, width: len(e.Value)})
	}
	if len(out) != len(rows) {
		return nil, fmt.Errorf("capture covers %d addresses, inventory has %d — the artefact must come from a complete sweep", len(out), len(rows))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].addr < out[j].addr })

	var buf bytes.Buffer
	buf.WriteString(header)
	for _, o := range out {
		fmt.Fprintf(&buf, "%s,%s,%s,%d,%s\n", o.addr[0:2], o.addr[2:4], o.addr[4:6], o.width, o.shape)
	}
	return buf.Bytes(), nil
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("extable/observe: ")

	capturePath := flag.String("capture", "", "path to a private 'rigprog read --settings' codeplug JSON")
	csvPath := flag.String("csv", "core/cat/table2.csv", "path to the Table 2 manual transcription")
	outPath := flag.String("out", "core/cat/table2-observed.csv", "path to write the observation CSV")
	flag.Parse()

	if *capturePath == "" {
		log.Fatal("-capture is required")
	}

	captureJSON, err := os.ReadFile(*capturePath)
	if err != nil {
		log.Fatalf("reading capture: %v", err)
	}
	manualCSV, err := os.ReadFile(*csvPath)
	if err != nil {
		log.Fatalf("reading %s: %v", *csvPath, err)
	}

	data, err := derive(captureJSON, manualCSV)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*outPath, data, 0o644); err != nil {
		log.Fatalf("writing %s: %v", *outPath, err)
	}
	log.Printf("wrote %d observations to %s", bytes.Count(data, []byte("\n"))-strings.Count(header, "\n"), *outPath)
}
