// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// wantEntry is the subset of LossEntry the fixture-driven table asserts:
// line/column/action/blocking, per the brief. Detail is deliberately not
// asserted (free-text prose) and Value only where a case specifically
// cares about it.
type wantEntry struct {
	Line     int
	Column   string
	Action   string
	Blocking bool
}

// entryTuples reduces a LossReport to the (Line,Column,Action,Blocking)
// tuples wantEntry asserts, in report order.
func entryTuples(r LossReport) []wantEntry {
	out := make([]wantEntry, len(r.Entries))
	for i, e := range r.Entries {
		out[i] = wantEntry{Line: e.Line, Column: e.Column, Action: e.Action, Blocking: e.Blocking}
	}
	return out
}

// entriesForLine filters a LossReport's Entries down to one CSV line.
func entriesForLine(r LossReport, line int) []LossEntry {
	var out []LossEntry
	for _, e := range r.Entries {
		if e.Line == line {
			out = append(out, e)
		}
	}
	return out
}

// findChannel returns the Channel with the given slot, and true, or the
// zero Channel and false.
func findChannel(channels []codeplug.Channel, slot string) (codeplug.Channel, bool) {
	for _, c := range channels {
		if c.Slot == slot {
			return c, true
		}
	}
	return codeplug.Channel{}, false
}

// ft710LikeCapabilities mirrors the FT-710 fields ImportCHIRP consults
// (core/driver/ft710/caps.go). It is a hand-built fixture rather than the
// real driver's Capabilities because core/csvio sits BELOW core/driver in
// the import graph and must not depend on it, even in tests. Drift
// between this and the real driver is caught end-to-end by the CLI
// byte-identity baseline, which does use the real driver.
func ft710LikeCapabilities() spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	slots := make([]string, 0, 99)
	for i := 1; i <= 99; i++ {
		slots = append(slots, fmt.Sprintf("%03d", i))
	}
	return spec.Capabilities{
		Model: "FT-710",
		CATID: "0800",
		Banks: []spec.Bank{
			{ID: spec.BankMemory, Label: "Memories", Slots: slots},
		},
		Modes:        []string{"LSB", "USB", "CW-U", "CW-L", "FM", "AM", "RTTY-U", "FM-N"},
		TagLen:       12,
		CTCSSTones:   tones[:],
		ShiftOptions: spec.StandardShiftOptions(),
		CTCSSStates:  spec.StandardCTCSSStates(),
	}
}

// deviantCapabilities is a radio that agrees with the FT-710 about
// NOTHING ImportCHIRP consults: 4 memory slots named differently, a
// 6-byte tag, renamed shift and CTCSS vocabulary, one mode. A chirp.go
// that threaded its caps parameter through and then ignored it would
// still pass every ft710LikeCapabilities test; only this fixture can tell
// the difference.
func deviantCapabilities() spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	return spec.Capabilities{
		Model: "DEVIANT-1",
		CATID: "0001",
		Banks: []spec.Bank{
			{ID: spec.BankMemory, Label: "Memories", Slots: []string{"M1", "M2", "M3", "M4"}},
		},
		Modes:      []string{"USB"},
		TagLen:     6,
		CTCSSTones: tones[:],
		ShiftOptions: []spec.ShiftOption{
			{Value: "SPLIT-NONE", Direction: spec.ShiftNone},
			{Value: "SPLIT-PLUS", Direction: spec.ShiftUp},
			{Value: "SPLIT-MINUS", Direction: spec.ShiftDown},
		},
		CTCSSStates: []spec.ToneState{
			{Value: "DISABLED", Semantics: spec.ToneOff},
			{Value: "TONE-TX", Semantics: spec.ToneEncode},
			{Value: "TONE-BOTH", Semantics: spec.ToneEncodeDecode},
		},
	}
}

func TestImportCHIRP_ShiftVocabFromCaps(t *testing.T) {
	csv := "Location,Frequency,Mode,Duplex\n1,145.500000,USB,+\n2,145.500000,USB,-\n3,145.500000,USB,\n"

	channels, report, err := ImportCHIRP(strings.NewReader(csv), deviantCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if report.HasBlocking() {
		t.Fatalf("unexpected blocking entries: %+v", report.Entries)
	}
	want := []string{"SPLIT-PLUS", "SPLIT-MINUS", "SPLIT-NONE"}
	if len(channels) != 3 {
		t.Fatalf("len(channels) = %d, want 3", len(channels))
	}
	for i, w := range want {
		if got := channels[i].Data.Shift; got != w {
			t.Errorf("channels[%d].Data.Shift = %q, want %q (deviant vocabulary, not the FT-710's)", i, got, w)
		}
	}
}

func TestImportCHIRP_CTCSSVocabFromCaps(t *testing.T) {
	csv := "Location,Frequency,Mode,Tone,rToneFreq,cToneFreq\n" +
		"1,145.500000,USB,Tone,88.5,88.5\n" +
		"2,145.500000,USB,TSQL,88.5,88.5\n" +
		"3,145.500000,USB,,,\n"

	channels, report, err := ImportCHIRP(strings.NewReader(csv), deviantCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if report.HasBlocking() {
		t.Fatalf("unexpected blocking entries: %+v", report.Entries)
	}
	want := []string{"TONE-TX", "TONE-BOTH", "DISABLED"}
	if len(channels) != 3 {
		t.Fatalf("len(channels) = %d, want 3", len(channels))
	}
	for i, w := range want {
		if got := channels[i].Data.CTCSS; got != w {
			t.Errorf("channels[%d].Data.CTCSS = %q, want %q (deviant vocabulary, not the FT-710's)", i, got, w)
		}
	}
}

func TestImportCHIRP_ModeAbsentFromCapsBlocks(t *testing.T) {
	// FM maps to the display name "FM", which deviantCapabilities does
	// not list — a radio that cannot express the mapped mode must refuse
	// the row, not write a mode it has no equivalent for.
	csv := "Location,Frequency,Mode\n1,145.500000,FM\n"

	_, report, err := ImportCHIRP(strings.NewReader(csv), deviantCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if !report.HasBlocking() {
		t.Fatalf("HasBlocking() = false, want true: %+v", report.Entries)
	}
}

func TestImportCHIRP_MissingShiftDirectionBlocks(t *testing.T) {
	caps := deviantCapabilities()
	caps.ShiftOptions = []spec.ShiftOption{{Value: "SPLIT-NONE", Direction: spec.ShiftNone}}

	_, report, err := ImportCHIRP(strings.NewReader("Location,Frequency,Mode,Duplex\n1,145.500000,USB,+\n"), caps)
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if !report.HasBlocking() {
		t.Fatalf("HasBlocking() = false, want true: a radio with no up-shift option must refuse a \"+\" row: %+v", report.Entries)
	}
}

func TestImportCHIRP_ToneNotInCapsChartBlocks(t *testing.T) {
	caps := deviantCapabilities()
	caps.CTCSSTones = []spec.Tone{670} // 67.0 Hz only

	_, report, err := ImportCHIRP(strings.NewReader("Location,Frequency,Mode,Tone,rToneFreq\n1,145.500000,USB,Tone,88.5\n"), caps)
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if !report.HasBlocking() {
		t.Fatalf("HasBlocking() = false, want true: 88.5 is not in this radio's chart: %+v", report.Entries)
	}
}

// TestImportCHIRP_MissingOffStateBlocks covers chirp.go's Tone "" branch
// when caps has no (Encodes:false, Decodes:false) CTCSSStates entry: a
// radio that cannot express "CTCSS off" at all must refuse the row, and
// the refusal's Detail wording is pinned here (nothing else in the suite
// asserts it — the review that requested this test found the wording
// otherwise unverified).
func TestImportCHIRP_MissingOffStateBlocks(t *testing.T) {
	caps := deviantCapabilities()
	caps.CTCSSStates = []spec.ToneState{
		{Value: "TONE-TX", Semantics: spec.ToneEncode},
		{Value: "TONE-BOTH", Semantics: spec.ToneEncodeDecode},
	}

	_, report, err := ImportCHIRP(strings.NewReader("Location,Frequency,Mode\n1,145.500000,USB\n"), caps)
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	entries := entriesForLine(report, 2)
	want := "DEVIANT-1 expresses no off CTCSS state"
	if len(entries) != 1 || !entries[0].Blocking || entries[0].Detail != want {
		t.Fatalf("entries = %+v, want exactly one Blocking entry with Detail %q", entries, want)
	}
}

// TestImportCHIRP_MissingEncodeDecodeStateBlocks covers chirp.go's Tone
// "TSQL" branch when caps has no (Encodes:true, Decodes:true) CTCSSStates
// entry: pins the Detail wording for a radio that cannot express
// encode+decode CTCSS at all.
func TestImportCHIRP_MissingEncodeDecodeStateBlocks(t *testing.T) {
	caps := deviantCapabilities()
	caps.CTCSSStates = []spec.ToneState{
		{Value: "DISABLED", Semantics: spec.ToneOff},
		{Value: "TONE-TX", Semantics: spec.ToneEncode},
	}

	_, report, err := ImportCHIRP(strings.NewReader("Location,Frequency,Mode,Tone,cToneFreq\n1,145.500000,USB,TSQL,88.5\n"), caps)
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	entries := entriesForLine(report, 2)
	want := "DEVIANT-1 expresses no encode+decode CTCSS state"
	if len(entries) != 1 || !entries[0].Blocking || entries[0].Detail != want {
		t.Fatalf("entries = %+v, want exactly one Blocking entry with Detail %q", entries, want)
	}
}

// TestImportCHIRP_TSQLToneNotInCapsChartBlocks covers the cToneFreq/TSQL
// side of the tone-chart-failure branch (TestImportCHIRP_ToneNotInCapsChartBlocks
// above only exercises the rToneFreq/"Tone" side): pins the Detail
// wording when a TSQL row's cToneFreq value is not in caps' chart.
func TestImportCHIRP_TSQLToneNotInCapsChartBlocks(t *testing.T) {
	caps := deviantCapabilities()
	caps.CTCSSTones = []spec.Tone{670} // 67.0 Hz only

	_, report, err := ImportCHIRP(strings.NewReader("Location,Frequency,Mode,Tone,cToneFreq\n1,145.500000,USB,TSQL,88.5\n"), caps)
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	entries := entriesForLine(report, 2)
	want := "tone frequency is not in the DEVIANT-1's CTCSS chart"
	if len(entries) != 1 || !entries[0].Blocking || entries[0].Column != "cToneFreq" || entries[0].Detail != want {
		t.Fatalf("entries = %+v, want exactly one Blocking cToneFreq entry with Detail %q", entries, want)
	}
}

// TestImportCHIRP_MissingDownShiftDirectionBlocks covers the Duplex "-"
// side of the missing-shift-direction branch
// (TestImportCHIRP_MissingShiftDirectionBlocks above only exercises the
// "+"/up-shift side): pins the Detail wording for a radio with no
// down-shift option.
func TestImportCHIRP_MissingDownShiftDirectionBlocks(t *testing.T) {
	caps := deviantCapabilities()
	caps.ShiftOptions = []spec.ShiftOption{
		{Value: "SPLIT-NONE", Direction: spec.ShiftNone},
		{Value: "SPLIT-PLUS", Direction: spec.ShiftUp},
	}

	_, report, err := ImportCHIRP(strings.NewReader("Location,Frequency,Mode,Duplex\n1,145.500000,USB,-\n"), caps)
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	entries := entriesForLine(report, 2)
	want := "DEVIANT-1 expresses no down-shift option"
	if len(entries) != 1 || !entries[0].Blocking || entries[0].Detail != want {
		t.Fatalf("entries = %+v, want exactly one Blocking entry with Detail %q", entries, want)
	}
}

func TestImportCHIRP_SlotSpaceFromCaps(t *testing.T) {
	csv := "Location,Frequency,Mode\n2,145.500000,USB\n"

	channels, report, err := ImportCHIRP(strings.NewReader(csv), deviantCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if report.HasBlocking() {
		t.Fatalf("ImportCHIRP: unexpected blocking entries: %+v", report.Entries)
	}
	if len(channels) != 1 {
		t.Fatalf("len(channels) = %d, want 1", len(channels))
	}
	if channels[0].Slot != "M2" {
		t.Errorf("Slot = %q, want %q (deviant bank's second slot, NOT the FT-710's \"002\")", channels[0].Slot, "M2")
	}
}

func TestImportCHIRP_LocationBeyondBankBlocks(t *testing.T) {
	csv := "Location,Frequency,Mode\n5,145.500000,USB\n"

	channels, report, err := ImportCHIRP(strings.NewReader(csv), deviantCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if len(channels) != 0 {
		t.Errorf("len(channels) = %d, want 0: Location 5 is beyond the deviant radio's 4 slots", len(channels))
	}
	if !report.HasBlocking() {
		t.Fatal("HasBlocking() = false, want true for an out-of-range Location")
	}
}

func TestImportCHIRP_TagLenFromCaps(t *testing.T) {
	csv := "Location,Name,Frequency,Mode\n1,ABCDEFGHIJ,145.500000,USB\n"

	channels, _, err := ImportCHIRP(strings.NewReader(csv), deviantCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("len(channels) = %d, want 1", len(channels))
	}
	if got := channels[0].Data.Tag; got != "ABCDEF" {
		t.Errorf("Tag = %q, want %q (truncated to the deviant radio's TagLen 6, not the FT-710's 12)", got, "ABCDEF")
	}
}

// TestImportCHIRP_TagDisplayIsUnknown is M9c-5 E1d's headline CHIRP
// change, and the one place in this milestone where csvio stops
// manufacturing a value it was never told.
//
// CHIRP's schema has no display-flag column at all. Before E1 that had to
// become a plain false, because ChannelData.TagDisplay was a plain bool
// and there was no third answer to give. Now there is: every
// CHIRP-imported channel carries {State: Unknown}, whatever the row said,
// because nothing in the file speaks to it.
//
// The consequence is deliberate, not incidental — see
// TestImportCHIRP_UnknownTagDisplayBlocksTheDiff.
func TestImportCHIRP_TagDisplayIsUnknown(t *testing.T) {
	// Rows chosen to span the mapping paths that DO carry data (a named
	// channel with a tone and a scan skip, and a bare minimal row): none
	// of them says anything about the front-panel display, so all of them
	// must land on Unknown.
	csv := "Location,Name,Frequency,Duplex,Tone,rToneFreq,cToneFreq,Mode,Skip\n" +
		"1,MYCALL,145.500000,+,Tone,88.5,88.5,FM,S\n" +
		"2,,7.100000,,,,,USB,\n"

	channels, report, err := ImportCHIRP(strings.NewReader(csv), ft710LikeCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if report.HasBlocking() {
		t.Fatalf("unexpected blocking entries: %+v", report.Entries)
	}
	if len(channels) != 2 {
		t.Fatalf("len(channels) = %d, want 2", len(channels))
	}
	want := codeplug.BoolField{State: codeplug.Unknown}
	for i, ch := range channels {
		if ch.Data == nil {
			t.Fatalf("channels[%d].Data = nil, want a populated channel", i)
		}
		if got := ch.Data.TagDisplay; got != want {
			t.Errorf("channels[%d].Data.TagDisplay = %+v, want %+v (CHIRP says nothing about the display flag; inventing false would be a lie the diff cannot see through)", i, got, want)
		}
	}
}

// writableCapabilities is ft710LikeCapabilities plus a deliberately
// PERMISSIVE field-support table: every field a CHIRP-imported channel
// transmits is write-Supported. It is not a claim about any real radio —
// it exists so that the only thing capable of blocking a diff in
// TestImportCHIRP_UnknownTagDisplayBlocksTheDiff is the TagDisplay gate
// itself, rather than some unrelated unwritable field masking it.
func writableCapabilities() spec.Capabilities {
	caps := ft710LikeCapabilities()
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	fields := map[spec.Field]spec.FieldSupport{
		spec.FieldFrequency:  rw,
		spec.FieldMode:       rw,
		spec.FieldClarifier:  rw,
		spec.FieldCTCSSState: rw,
		spec.FieldCTCSSTone:  rw,
		spec.FieldShift:      rw,
		spec.FieldTag:        rw,
		spec.FieldTagDisplay: rw,
		spec.FieldScanSkip:   rw,
	}
	banks := make([]spec.Bank, len(caps.Banks))
	copy(banks, caps.Banks)
	for i := range banks {
		banks[i].Fields = fields
	}
	caps.Banks = banks
	return caps
}

// TestImportCHIRP_UnknownTagDisplayBlocksTheDiff pins E1d's whole point:
// the honest Unknown is not cosmetic — it reaches the plan-time gate E1b
// installed and stops the channel, per channel, with the one BlockReason
// that tells the user what to do about it.
//
// The reason string is spelled out as a literal here rather than
// referencing core/codeplug's unexported constant: this test's job is to
// prove the two halves of E1 meet, and a test that echoed the production
// value could not tell if the meeting point moved.
//
// The second half proves the friction is FINITE: once the user answers
// the question (Known, either way), the same channel plans cleanly.
func TestImportCHIRP_UnknownTagDisplayBlocksTheDiff(t *testing.T) {
	const wantTagDisplayUnknownReason = "tag display unknown — set On or Off before sending"

	caps := writableCapabilities()
	// Location 2 -> slot "002", empty in the baseline: a DiffAdded entry.
	csv := "Location,Name,Frequency,Duplex,Tone,rToneFreq,cToneFreq,Mode,Skip\n" +
		"2,MYCALL,7.100000,,,,,USB,\n"
	imported, report, err := ImportCHIRP(strings.NewReader(csv), caps)
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if report.HasBlocking() {
		t.Fatalf("unexpected blocking entries: %+v", report.Entries)
	}
	if len(imported) != 1 || imported[0].Slot != "002" {
		t.Fatalf("ImportCHIRP = %+v, want exactly slot 002", imported)
	}

	// A baseline (as read from the radio) and a candidate file sharing one
	// slot inventory, per Diff's contract. Both slots are empty in the
	// baseline; the candidate carries the CHIRP-imported channel at 002.
	newCodeplug := func(imported *codeplug.Channel) *codeplug.Codeplug {
		channels := []codeplug.Channel{{Slot: "001"}, {Slot: "002"}}
		if imported != nil {
			channels[1] = *imported
		}
		return &codeplug.Codeplug{Schema: codeplug.CurrentSchema, Channels: channels}
	}

	result, err := codeplug.Diff(newCodeplug(nil), newCodeplug(&imported[0]), caps)
	if err != nil {
		t.Fatalf("codeplug.Diff: unexpected error: %v", err)
	}
	var entry codeplug.DiffEntry
	for _, e := range result.Entries {
		if e.Slot == "002" {
			entry = e
		}
	}
	if entry.Kind != codeplug.DiffAdded {
		t.Fatalf("slot 002 Kind = %v, want %v", entry.Kind, codeplug.DiffAdded)
	}
	if !entry.Blocked {
		t.Fatal("slot 002 Blocked = false, want true: a CHIRP-imported channel's Unknown tag_display must not reach the wire")
	}
	if entry.BlockReason != wantTagDisplayUnknownReason {
		t.Errorf("slot 002 BlockReason = %q, want %q", entry.BlockReason, wantTagDisplayUnknownReason)
	}

	// The mitigation: the user answers the question, and the same channel
	// plans cleanly.
	resolved := imported[0]
	data := *resolved.Data
	data.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: false}
	resolved.Data = &data

	result, err = codeplug.Diff(newCodeplug(nil), newCodeplug(&resolved), caps)
	if err != nil {
		t.Fatalf("codeplug.Diff (resolved): unexpected error: %v", err)
	}
	for _, e := range result.Entries {
		if e.Slot == "002" {
			entry = e
		}
	}
	if entry.Blocked {
		t.Errorf("slot 002 with a Known tag_display is Blocked (%q), want it sendable", entry.BlockReason)
	}
}

// TestImportCHIRP_Fixture drives testdata/chirp_sample.csv — one row per
// mapping rule in the brief — against a table of expected channels and
// expected LossEntries (line/column/action/blocking all asserted, per
// the brief's test list).
func TestImportCHIRP_Fixture(t *testing.T) {
	f, err := os.Open("testdata/chirp_sample.csv")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	channels, report, err := ImportCHIRP(f, ft710LikeCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP() error = %v", err)
	}

	cases := []struct {
		name     string
		line     int
		wantSlot string // "" means no channel should exist for this row
		check    func(t *testing.T, d *codeplug.ChannelData)
		want     []wantEntry
	}{
		{
			name:     "Location 1: happy path FM simplex, tone off, rTone/cTone/Dtcs defaults ignored",
			line:     2,
			wantSlot: "001",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				if d.FreqHz != 146500000 {
					t.Errorf("FreqHz = %d, want 146500000", d.FreqHz)
				}
				if d.Mode != "FM" {
					t.Errorf("Mode = %q, want FM", d.Mode)
				}
				if d.Shift != "SIMPLEX" {
					t.Errorf("Shift = %q, want SIMPLEX", d.Shift)
				}
				if d.CTCSS != "OFF" {
					t.Errorf("CTCSS = %q, want OFF", d.CTCSS)
				}
				if d.Tag != "CALLING" {
					t.Errorf("Tag = %q, want CALLING", d.Tag)
				}
				if d.ScanSkip.State != codeplug.Known || d.ScanSkip.Value != false {
					t.Errorf("ScanSkip = %+v, want Known/false", d.ScanSkip)
				}
			},
			want: nil,
		},
		{
			name:     "Location 2: Duplex +, Offset non-zero dropped, Mode NFM->FM-N, Skip S",
			line:     3,
			wantSlot: "002",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				if d.Shift != "PLUS" {
					t.Errorf("Shift = %q, want PLUS", d.Shift)
				}
				if d.Mode != "FM-N" {
					t.Errorf("Mode = %q, want FM-N", d.Mode)
				}
				if d.ScanSkip.State != codeplug.Known || d.ScanSkip.Value != true {
					t.Errorf("ScanSkip = %+v, want Known/true", d.ScanSkip)
				}
			},
			want: []wantEntry{{3, "Offset", "dropped", false}},
		},
		{
			name:     "Location 3: Duplex -, Mode AM",
			line:     4,
			wantSlot: "003",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				if d.Shift != "MINUS" {
					t.Errorf("Shift = %q, want MINUS", d.Shift)
				}
				if d.Mode != "AM" {
					t.Errorf("Mode = %q, want AM", d.Mode)
				}
			},
			want: nil,
		},
		{
			name:     "Location 4: Mode USB unchanged",
			line:     5,
			wantSlot: "004",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				if d.Mode != "USB" {
					t.Errorf("Mode = %q, want USB", d.Mode)
				}
			},
			want: nil,
		},
		{
			name:     "Location 5: Mode LSB unchanged",
			line:     6,
			wantSlot: "005",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				if d.Mode != "LSB" {
					t.Errorf("Mode = %q, want LSB", d.Mode)
				}
			},
			want: nil,
		},
		{
			name:     "Location 6: Mode CW -> CW-U",
			line:     7,
			wantSlot: "006",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				if d.Mode != "CW-U" {
					t.Errorf("Mode = %q, want CW-U", d.Mode)
				}
			},
			want: nil,
		},
		{
			name:     "Location 7: Mode CWR -> CW-L",
			line:     8,
			wantSlot: "007",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				if d.Mode != "CW-L" {
					t.Errorf("Mode = %q, want CW-L", d.Mode)
				}
			},
			want: nil,
		},
		{
			name:     "Location 8: Mode RTTY -> RTTY-U",
			line:     9,
			wantSlot: "008",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				if d.Mode != "RTTY-U" {
					t.Errorf("Mode = %q, want RTTY-U", d.Mode)
				}
			},
			want: nil,
		},
		{
			name:     "Location 9: Tone=Tone -> ENC with rToneFreq",
			line:     10,
			wantSlot: "009",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				if d.CTCSS != "ENC" {
					t.Errorf("CTCSS = %q, want ENC", d.CTCSS)
				}
				if d.CTCSSTone.State != codeplug.Known || d.CTCSSTone.Value != spec.Tone(885) {
					t.Errorf("CTCSSTone = %+v, want Known/885 (88.5 Hz)", d.CTCSSTone)
				}
			},
			want: nil,
		},
		{
			name:     "Location 10: Tone=TSQL -> ENC-DEC with cToneFreq",
			line:     11,
			wantSlot: "010",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				if d.CTCSS != "ENC-DEC" {
					t.Errorf("CTCSS = %q, want ENC-DEC", d.CTCSS)
				}
				if d.CTCSSTone.State != codeplug.Known || d.CTCSSTone.Value != spec.Tone(1000) {
					t.Errorf("CTCSSTone = %+v, want Known/1000 (100.0 Hz)", d.CTCSSTone)
				}
			},
			want: nil,
		},
		{
			name:     "Location 11: tone frequency not in StandardCTCSSTones -> Blocking",
			line:     12,
			wantSlot: "011",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				if d.CTCSS != "ENC" {
					t.Errorf("CTCSS = %q, want ENC (tone TYPE was known)", d.CTCSS)
				}
				if d.CTCSSTone.State != codeplug.Unknown {
					t.Errorf("CTCSSTone.State = %v, want Unknown (value unresolved)", d.CTCSSTone.State)
				}
			},
			want: []wantEntry{{12, "rToneFreq", "unsupported", true}},
		},
		{
			name:     "Location 12: Tone=DTCS -> Blocking unsupported",
			line:     13,
			wantSlot: "012",
			check:    func(t *testing.T, d *codeplug.ChannelData) {},
			want:     []wantEntry{{13, "Tone", "unsupported", true}},
		},
		{
			name:     "Location 13: Tone=Cross -> Blocking unsupported",
			line:     14,
			wantSlot: "013",
			check:    func(t *testing.T, d *codeplug.ChannelData) {},
			want:     []wantEntry{{14, "Tone", "unsupported", true}},
		},
		{
			name:     "Location 14: Duplex=off -> SIMPLEX + dropped",
			line:     15,
			wantSlot: "014",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				if d.Shift != "SIMPLEX" {
					t.Errorf("Shift = %q, want SIMPLEX", d.Shift)
				}
			},
			want: []wantEntry{{15, "Duplex", "dropped", false}},
		},
		{
			name:     "Location 15: Duplex=split -> Blocking unsupported",
			line:     16,
			wantSlot: "015",
			check:    func(t *testing.T, d *codeplug.ChannelData) {},
			want:     []wantEntry{{16, "Duplex", "unsupported", true}},
		},
		{
			name:     "Location 16: Mode=DIG -> Blocking unsupported",
			line:     17,
			wantSlot: "016",
			check:    func(t *testing.T, d *codeplug.ChannelData) {},
			want:     []wantEntry{{17, "Mode", "unsupported", true}},
		},
		{
			name:     "Location 0: out of range -> Blocking, no channel",
			line:     18,
			wantSlot: "",
			want:     []wantEntry{{18, "Location", "unsupported", true}},
		},
		{
			name:     "Location 100: out of range -> Blocking, no channel",
			line:     19,
			wantSlot: "",
			want:     []wantEntry{{19, "Location", "unsupported", true}},
		},
		{
			name:     "Location 17: Name >12 bytes -> truncated + approximated",
			line:     20,
			wantSlot: "017",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				// "LONGNAMEEXCEEDS12" is 17 bytes; ft710LikeCapabilities'
				// TagLen is 12, so the only correct truncation is the
				// first 12 bytes, "LONGNAMEEXCE". This exact assertion
				// (FIX C2, m9c1 registration-gate dispatch C) replaces a
				// looser one that also accepted an 11-byte truncation,
				// which would have let an off-by-one bug through
				// undetected.
				if d.Tag != "LONGNAMEEXCE" {
					t.Errorf("Tag = %q, want exactly \"LONGNAMEEXCE\" (the first 12 bytes of LONGNAMEEXCEEDS12)", d.Tag)
				}
			},
			want: []wantEntry{{20, "Name", "approximated", false}},
		},
		{
			name:     "Location 18: Name charset violation -> sanitized + approximated",
			line:     21,
			wantSlot: "018",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				if strings.Contains(d.Tag, ";") {
					t.Errorf("Tag = %q, must not contain ';'", d.Tag)
				}
				if d.Tag != "BAD NAME" {
					t.Errorf("Tag = %q, want \"BAD NAME\" (';' replaced with space)", d.Tag)
				}
			},
			want: []wantEntry{{21, "Name", "approximated", false}},
		},
		{
			name:     "Location 19: fractional-Hz remainder -> Blocking unsupported",
			line:     22,
			wantSlot: "019",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				if d.FreqHz != 0 {
					t.Errorf("FreqHz = %d, want 0 (unresolved)", d.FreqHz)
				}
			},
			want: []wantEntry{{22, "Frequency", "unsupported", true}},
		},
		{
			name:     "Location 20: unparseable Frequency -> Blocking unsupported",
			line:     23,
			wantSlot: "020",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				if d.FreqHz != 0 {
					t.Errorf("FreqHz = %d, want 0 (unresolved)", d.FreqHz)
				}
			},
			want: []wantEntry{{23, "Frequency", "unsupported", true}},
		},
		{
			name:     "Location 21: TStep/DtcsCode/DtcsPolarity/Comment non-default -> 4x dropped",
			line:     24,
			wantSlot: "021",
			check:    func(t *testing.T, d *codeplug.ChannelData) {},
			want: []wantEntry{
				{24, "TStep", "dropped", false},
				{24, "DtcsCode", "dropped", false},
				{24, "DtcsPolarity", "dropped", false},
				{24, "Comment", "dropped", false},
			},
		},
		{
			name:     "Location 22: Skip=P -> non-blocking dropped, ScanSkip unresolved",
			line:     25,
			wantSlot: "022",
			check: func(t *testing.T, d *codeplug.ChannelData) {
				if d.ScanSkip.State != codeplug.Unknown {
					t.Errorf("ScanSkip.State = %v, want Unknown", d.ScanSkip.State)
				}
			},
			want: []wantEntry{{25, "Skip", "dropped", false}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ch, ok := findChannel(channels, tc.wantSlot)
			if tc.wantSlot == "" {
				if ok {
					t.Fatalf("channel found for slot %q, want none (Location out of range)", ch.Slot)
				}
			} else {
				if !ok {
					t.Fatalf("no channel found for slot %q", tc.wantSlot)
				}
				if ch.Empty() {
					t.Fatalf("channel %q is empty, want populated", tc.wantSlot)
				}
				if tc.check != nil {
					tc.check(t, ch.Data)
				}
			}

			gotEntries := entriesForLine(report, tc.line)
			if len(gotEntries) != len(tc.want) {
				t.Fatalf("line %d: %d LossEntries, want %d: got=%+v want=%+v", tc.line, len(gotEntries), len(tc.want), gotEntries, tc.want)
			}
			for i, w := range tc.want {
				g := gotEntries[i]
				if g.Line != w.Line || g.Column != w.Column || g.Action != w.Action || g.Blocking != w.Blocking {
					t.Errorf("line %d entry %d = {Line:%d Column:%q Action:%q Blocking:%v}, want %+v", tc.line, i, g.Line, g.Column, g.Action, g.Blocking, w)
				}
			}
		})
	}

	if !report.HasBlocking() {
		t.Error("report.HasBlocking() = false, want true (fixture has several Blocking entries)")
	}
}

// TestLossReport_HasBlocking covers HasBlocking both ways directly.
func TestLossReport_HasBlocking(t *testing.T) {
	cases := []struct {
		name    string
		entries []LossEntry
		want    bool
	}{
		{"empty report", nil, false},
		{"only non-blocking entries", []LossEntry{{Action: "dropped", Blocking: false}, {Action: "approximated", Blocking: false}}, false},
		{"one blocking entry", []LossEntry{{Action: "dropped", Blocking: false}, {Action: "unsupported", Blocking: true}}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := LossReport{Entries: tc.entries}
			if got := r.HasBlocking(); got != tc.want {
				t.Errorf("HasBlocking() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestImportCHIRP_MissingCoreColumns covers the header-level error when
// Location, Frequency or Mode is absent.
func TestImportCHIRP_MissingCoreColumns(t *testing.T) {
	cases := []struct {
		name   string
		header string
	}{
		{"missing Location", "Name,Frequency,Duplex,Offset,Tone,rToneFreq,cToneFreq,DtcsCode,DtcsPolarity,Mode,TStep,Skip,Comment"},
		{"missing Frequency", "Location,Name,Duplex,Offset,Tone,rToneFreq,cToneFreq,DtcsCode,DtcsPolarity,Mode,TStep,Skip,Comment"},
		{"missing Mode", "Location,Name,Frequency,Duplex,Offset,Tone,rToneFreq,cToneFreq,DtcsCode,DtcsPolarity,TStep,Skip,Comment"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ImportCHIRP(strings.NewReader(tc.header+"\n"), ft710LikeCapabilities())
			if err == nil {
				t.Fatal("ImportCHIRP() error = nil, want error for missing core column")
			}
		})
	}
}

// TestImportCHIRP_UnknownColumnWithDataBlocks covers a column outside
// CHIRP's recognised set that carries a non-empty value on a row: such a
// value cannot be safely discarded, so it must produce a Blocking
// ActionUnsupported LossEntry naming the column — distinct from the
// recognised-but-unmapped columns (TStep etc, see chirpExtraColumns),
// which are non-blocking.
func TestImportCHIRP_UnknownColumnWithDataBlocks(t *testing.T) {
	body := "Location,Name,Frequency,Mode,SomeExtraColumn\n1,TESTCH,145.500000,FM,anything\n"
	channels, report, err := ImportCHIRP(strings.NewReader(body), ft710LikeCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP() error = %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("ImportCHIRP() = %d channels, want 1 (channel still built despite the blocking column)", len(channels))
	}
	entries := entriesForLine(report, 2)
	if len(entries) != 1 || entries[0].Column != "SomeExtraColumn" || entries[0].Action != ActionUnsupported || !entries[0].Blocking {
		t.Errorf("entries = %+v, want one Blocking ActionUnsupported entry naming SomeExtraColumn", entries)
	}
}

// TestImportCHIRP_UnknownColumnAllEmptySilent covers the converse: an
// unrecognised column present in the header but empty on every row
// produces no LossEntry at all — only a NON-empty value in it is a
// problem.
func TestImportCHIRP_UnknownColumnAllEmptySilent(t *testing.T) {
	body := "Location,Name,Frequency,Mode,SomeExtraColumn\n1,TESTCH,145.500000,FM,\n"
	channels, report, err := ImportCHIRP(strings.NewReader(body), ft710LikeCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP() error = %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("ImportCHIRP() = %d channels, want 1", len(channels))
	}
	if len(report.Entries) != 0 {
		t.Errorf("report.Entries = %+v, want none (unrecognised column empty on every row)", report.Entries)
	}
}

// TestImportCHIRP_DuplicateHeaderColumn covers the duplicate-header-
// column error directly: a typed *ParseError at line 1, naming the
// duplicate.
func TestImportCHIRP_DuplicateHeaderColumn(t *testing.T) {
	body := "Location,Location,Frequency,Mode\n1,1,145.500000,FM\n"
	_, _, err := ImportCHIRP(strings.NewReader(body), ft710LikeCapabilities())
	if err == nil {
		t.Fatal("ImportCHIRP() error = nil, want error for duplicate header column")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("ImportCHIRP() error = %T, want *ParseError", err)
	}
	if pe.Line != 1 {
		t.Errorf("ParseError.Line = %d, want 1", pe.Line)
	}
	if !strings.Contains(err.Error(), "Location") {
		t.Errorf("ImportCHIRP() error = %q, want it to name the duplicate column Location", err.Error())
	}
}

// TestImportCHIRP_PhysicalLineNumbers_QuotedMultilineField covers the
// physical-line-number requirement: row 1's Name cell is a quoted field
// containing an embedded newline (legal CSV), spanning TWO physical
// lines by itself. A naive per-RECORD line counter would report row 2's
// LossEntry at line 3 (header=1, row1=2, row2=3); the correct PHYSICAL
// line is 4 (header=1, row1 spans 2-3, row2=4).
func TestImportCHIRP_PhysicalLineNumbers_QuotedMultilineField(t *testing.T) {
	body := "Location,Name,Frequency,Mode\n" +
		"1,\"A\nB\",145.500000,FM\n" +
		"999,BADROW,145.500000,FM\n"
	_, report, err := ImportCHIRP(strings.NewReader(body), ft710LikeCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP() error = %v", err)
	}
	entries := entriesForLine(report, 4)
	if len(entries) != 1 || entries[0].Column != "Location" || !entries[0].Blocking {
		t.Errorf("entries for physical line 4 = %+v, want one Blocking Location entry", entries)
	}
	if got := entriesForLine(report, 3); len(got) != 0 {
		t.Errorf("entries for line 3 = %+v, want none (physical line numbering should place the entry at line 4, not the naive record-count line 3)", got)
	}
}

// TestImportCHIRP_UnparseableCSV covers a structurally malformed CSV
// stream.
func TestImportCHIRP_UnparseableCSV(t *testing.T) {
	body := "Location,Name,Frequency,Mode\n1,\"unterminated,145.5,FM\n"
	_, _, err := ImportCHIRP(strings.NewReader(body), ft710LikeCapabilities())
	if err == nil {
		t.Fatal("ImportCHIRP() error = nil, want error for malformed CSV")
	}
}

// TestImportCHIRP_RowLengthMismatch covers a data row with a different
// field count than the header: best-effort, so this is a Blocking
// LossEntry on that one row (no channel for it), not a fatal error —
// every other row still imports.
func TestImportCHIRP_RowLengthMismatch(t *testing.T) {
	body := "Location,Name,Frequency,Mode\n1,SHORT,145.500000\n2,GOOD,145.525000,FM\n"
	channels, report, err := ImportCHIRP(strings.NewReader(body), ft710LikeCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP() error = %v", err)
	}
	if _, ok := findChannel(channels, "001"); ok {
		t.Error("channel 001 present, want none (short row skipped)")
	}
	if _, ok := findChannel(channels, "002"); !ok {
		t.Error("channel 002 missing, want present (well-formed row still imported)")
	}
	entries := entriesForLine(report, 2)
	if len(entries) != 1 || !entries[0].Blocking || entries[0].Action != ActionUnsupported {
		t.Errorf("line 2 entries = %+v, want one Blocking unsupported entry", entries)
	}
}

// TestImportCHIRP_DuplexAndToneDefaultCases covers the catch-all
// "unrecognised value" default cases for Duplex and Tone (neither of
// which appears explicitly enumerated among the brief's named values),
// plus a TSQL row whose cToneFreq does not resolve, and Tone="Tone" when
// the rToneFreq column is entirely absent from the header (as opposed to
// present-but-empty).
func TestImportCHIRP_DuplexAndToneDefaultCases(t *testing.T) {
	t.Run("unrecognised Duplex value", func(t *testing.T) {
		body := "Location,Name,Frequency,Duplex,Mode\n1,TESTCH,145.500000,weird,FM\n"
		_, report, err := ImportCHIRP(strings.NewReader(body), ft710LikeCapabilities())
		if err != nil {
			t.Fatalf("ImportCHIRP() error = %v", err)
		}
		entries := entriesForLine(report, 2)
		if len(entries) != 1 || entries[0].Column != "Duplex" || !entries[0].Blocking {
			t.Errorf("entries = %+v, want one Blocking Duplex entry", entries)
		}
	})
	t.Run("unrecognised Tone value", func(t *testing.T) {
		body := "Location,Name,Frequency,Tone,Mode\n1,TESTCH,145.500000,Weird,FM\n"
		_, report, err := ImportCHIRP(strings.NewReader(body), ft710LikeCapabilities())
		if err != nil {
			t.Fatalf("ImportCHIRP() error = %v", err)
		}
		entries := entriesForLine(report, 2)
		if len(entries) != 1 || entries[0].Column != "Tone" || !entries[0].Blocking {
			t.Errorf("entries = %+v, want one Blocking Tone entry", entries)
		}
	})
	t.Run("TSQL cToneFreq not in standard chart", func(t *testing.T) {
		body := "Location,Name,Frequency,Tone,cToneFreq,Mode\n1,TESTCH,145.500000,TSQL,99.9,FM\n"
		channels, report, err := ImportCHIRP(strings.NewReader(body), ft710LikeCapabilities())
		if err != nil {
			t.Fatalf("ImportCHIRP() error = %v", err)
		}
		ch, ok := findChannel(channels, "001")
		if !ok {
			t.Fatal("channel 001 missing")
		}
		if ch.Data.CTCSS != "ENC-DEC" {
			t.Errorf("CTCSS = %q, want ENC-DEC", ch.Data.CTCSS)
		}
		entries := entriesForLine(report, 2)
		if len(entries) != 1 || entries[0].Column != "cToneFreq" || !entries[0].Blocking {
			t.Errorf("entries = %+v, want one Blocking cToneFreq entry", entries)
		}
	})
	t.Run("Tone=Tone with rToneFreq column entirely absent from header", func(t *testing.T) {
		body := "Location,Name,Frequency,Tone,Mode\n1,TESTCH,145.500000,Tone,FM\n"
		_, report, err := ImportCHIRP(strings.NewReader(body), ft710LikeCapabilities())
		if err != nil {
			t.Fatalf("ImportCHIRP() error = %v", err)
		}
		entries := entriesForLine(report, 2)
		if len(entries) != 1 || entries[0].Column != "rToneFreq" || !entries[0].Blocking {
			t.Errorf("entries = %+v, want one Blocking rToneFreq entry", entries)
		}
	})
}

// TestImportCHIRP_ToneExcessPrecisionBlocks covers the exact-decimal-
// precision rule through the full ImportCHIRP path: an rToneFreq cell
// with more than one decimal place (e.g. "88.54") must Block as
// unsupported rather than being silently rounded into the standard
// chart.
func TestImportCHIRP_ToneExcessPrecisionBlocks(t *testing.T) {
	body := "Location,Name,Frequency,Tone,rToneFreq,Mode\n1,TESTCH,145.500000,Tone,88.54,FM\n"
	channels, report, err := ImportCHIRP(strings.NewReader(body), ft710LikeCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP() error = %v", err)
	}
	ch, ok := findChannel(channels, "001")
	if !ok {
		t.Fatal("channel 001 missing")
	}
	if ch.Data.CTCSSTone.State != codeplug.Unknown {
		t.Errorf("CTCSSTone.State = %v, want Unknown (excess-precision value unresolved, not rounded)", ch.Data.CTCSSTone.State)
	}
	entries := entriesForLine(report, 2)
	if len(entries) != 1 || entries[0].Column != "rToneFreq" || !entries[0].Blocking {
		t.Errorf("entries = %+v, want one Blocking rToneFreq entry", entries)
	}
}

// TestImportCHIRP_FrequencyOutOfRange covers the Frequency-exceeds-range
// LossEntry detail path through ImportCHIRP itself (parseCHIRPFrequency
// is covered directly by TestParseCHIRPFrequency; this confirms
// importCHIRPRow selects the right Detail message for that error).
func TestImportCHIRP_FrequencyOutOfRange(t *testing.T) {
	body := "Location,Name,Frequency,Mode\n1,TESTCH,5000.000000,FM\n"
	channels, report, err := ImportCHIRP(strings.NewReader(body), ft710LikeCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP() error = %v", err)
	}
	ch, ok := findChannel(channels, "001")
	if !ok || ch.Data.FreqHz != 0 {
		t.Fatalf("channel 001 = %+v, ok=%v, want FreqHz 0", ch, ok)
	}
	entries := entriesForLine(report, 2)
	if len(entries) != 1 || entries[0].Column != "Frequency" || !entries[0].Blocking {
		t.Errorf("entries = %+v, want one Blocking Frequency entry", entries)
	}
}

// TestParseCHIRPFrequency covers parseCHIRPFrequency directly: exact
// whole-Hz conversion, short/absent fractional parts needing zero
// padding, format errors, the fractional-Hz-remainder rejection, and the
// two overflow paths (intPart too large for uint64; total exceeding
// uint32).
func TestParseCHIRPFrequency(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    uint32
		wantErr error
	}{
		{"whole MHz, no dot", "146", 146000000, nil},
		{"full 6-decimal precision", "146.500000", 146500000, nil},
		{"short fractional part is zero-padded", "14.2", 14200000, nil},
		{"empty fractional part after dot is zero-padded", "14.", 14000000, nil},
		{"more than 6 decimals but all-zero remainder is fine", "146.5000000", 146500000, nil},
		{"non-digit integer part", "abc.5", 0, errCHIRPFreqFormat},
		{"non-digit fractional part", "146.abc", 0, errCHIRPFreqFormat},
		{"empty string", "", 0, errCHIRPFreqFormat},
		{"fractional Hz remainder", "145.1234567", 0, errCHIRPFreqFractionalHz},
		{"integer part too large for uint64", "99999999999999999999.000000", 0, errCHIRPFreqFormat},
		{"total exceeds uint32 range", "5000.000000", 0, errCHIRPFreqRange},
		{"exactly MaxUint32 Hz accepted (boundary)", "4294.967295", math.MaxUint32, nil},
		{"one Hz over MaxUint32 rejected (boundary)", "4294.967296", 0, errCHIRPFreqRange},
		{"uint64 multiplication overflow must not wrap into range", "18446744073710.500000", 0, errCHIRPFreqRange},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCHIRPFrequency(tc.in)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("parseCHIRPFrequency(%q) error = %v, want %v", tc.in, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCHIRPFrequency(%q) unexpected error: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("parseCHIRPFrequency(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// TestParseCHIRPTone covers parseCHIRPTone directly: an unparseable cell
// and a well-formed value outside the standard chart both report
// (0, false).
func TestParseCHIRPTone(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		wantTone spec.Tone
		wantOK   bool
	}{
		{"standard tone", "88.5", spec.Tone(885), true},
		{"unparseable text", "not-a-number", 0, false},
		{"well-formed but not in chart", "99.9", 0, false},
		{"trailing zero beyond one place is exactly representable, accepted", "88.50", spec.Tone(885), true},
		{"more than one decimal place is rejected, not rounded", "88.54", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotTone, gotOK := parseCHIRPTone(tc.in, ft710LikeCapabilities())
			if gotTone != tc.wantTone || gotOK != tc.wantOK {
				t.Errorf("parseCHIRPTone(%q) = (%v, %v), want (%v, %v)", tc.in, gotTone, gotOK, tc.wantTone, tc.wantOK)
			}
		})
	}
}

// TestIsNonZeroCHIRPOffset covers isNonZeroCHIRPOffset directly,
// including its conservative treatment of unparseable input as
// "present" (non-zero).
func TestIsNonZeroCHIRPOffset(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"zero", "0.000000", false},
		{"non-zero", "1.600000", true},
		{"unparseable text treated as present", "garbage", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNonZeroCHIRPOffset(tc.in); got != tc.want {
				t.Errorf("isNonZeroCHIRPOffset(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestSanitizeCHIRPName covers sanitizeCHIRPName directly for two
// charset-violation shapes beyond the ';' case already exercised via
// TestImportCHIRP_Fixture (Location 18): a non-printable control byte
// (0x07, BEL) and a non-ASCII byte (0xC3, the lead byte of a UTF-8
// multi-byte rune). Both fall outside chirpTagByteOK's printable-ASCII
// range and must be replaced with a space, producing exactly one
// non-blocking ActionApproximated LossEntry on the Name column.
func TestSanitizeCHIRPName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"non-printable control byte (0x07 BEL)", "A\x07B", "A B"},
		{"non-ASCII byte (0xC3)", "A\xC3B", "A B"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, entries := sanitizeCHIRPName(1, tc.in, ft710LikeCapabilities())
			if got != tc.want {
				t.Errorf("sanitizeCHIRPName(%q) tag = %q, want %q", tc.in, got, tc.want)
			}
			if len(entries) != 1 {
				t.Fatalf("sanitizeCHIRPName(%q) = %d LossEntries, want 1: %+v", tc.in, len(entries), entries)
			}
			e := entries[0]
			if e.Column != "Name" {
				t.Errorf("LossEntry.Column = %q, want \"Name\"", e.Column)
			}
			if e.Action != ActionApproximated {
				t.Errorf("LossEntry.Action = %q, want %q", e.Action, ActionApproximated)
			}
			if e.Blocking {
				t.Errorf("LossEntry.Blocking = true, want false (non-blocking)")
			}
		})
	}
}
