// SPDX-License-Identifier: GPL-3.0-or-later

package csvio

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
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
//
// The MEM bank's Fields map is part of that mirroring since M9c-6
// (D-tagdisplay): ImportCHIRP now derives tag_display from the target
// bank's own spec.FieldTagDisplay support, so a fixture that declared no
// fields at all would describe a radio with NO display flag — the FTdx10's
// shape, not the FT-710's — and this fixture's job is to be an FT-710. The
// real driver's own MEM map is Read/Write Supported for tag_display in
// every profile that ships (ft710/caps.go's bankFields, rw), which is what
// the rw value below states.
func ft710LikeCapabilities() spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	slots := make([]string, 0, 99)
	for i := 1; i <= 99; i++ {
		slots = append(slots, fmt.Sprintf("%03d", i))
	}
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	return spec.Capabilities{
		Model: "FT-710",
		CATID: "0800",
		Banks: []spec.Bank{
			{ID: spec.BankMemory, Label: "Memories", Slots: slots, Fields: map[spec.Field]spec.FieldSupport{
				spec.FieldFrequency:  rw,
				spec.FieldMode:       rw,
				spec.FieldClarifier:  rw,
				spec.FieldCTCSSState: rw,
				spec.FieldShift:      rw,
				spec.FieldTag:        rw,
				spec.FieldTagDisplay: rw,
				// Tone and scan skip are the zero FieldSupport on the real
				// FT-710 too (its CAT protocol reaches neither); erase is
				// {Unsupported, Unverified} on MEM there, and THAT shape is
				// nothing ImportCHIRP consults. FieldScanSkip stopped being
				// decoration at M9d-2 task 8: ImportCHIRP now derives the
				// imported scan_skip from it, so this entry is load-bearing
				// exactly as FieldTagDisplay's already was.
				spec.FieldCTCSSTone: {},
				spec.FieldScanSkip:  {},
			}},
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

// noTagCapabilities is a NoTag model: no channel-name route over CAT at
// all (TagLen 0, NoTag true, no FieldTag/FieldTagDisplay in the memory
// bank's Fields map) — the design 2026-09-12-nameless-capability fixture
// shape, hand-built here for the same reason ft710LikeCapabilities is:
// core/csvio sits below core/driver and must not depend on it, even in
// tests.
func noTagCapabilities() spec.Capabilities {
	tones := spec.StandardCTCSSTones()
	return spec.Capabilities{
		Model: "FAKE-NN",
		CATID: "0000",
		NoTag: true,
		Banks: []spec.Bank{
			{ID: spec.BankMemory, Label: "Memories", Slots: []string{"001", "002", "003"}},
		},
		Modes:        []string{"USB"},
		TagLen:       0,
		CTCSSTones:   tones[:],
		ShiftOptions: spec.StandardShiftOptions(),
		CTCSSStates:  spec.StandardCTCSSStates(),
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

// TestImportCHIRP_NoTagDropsNamesWithOneWarning pins the
// nameless-capability CHIRP-import behaviour (design
// 2026-09-12-nameless-capability §2.2, §7 Q2): a NoTag model has no
// channel-name route at all, so a CHIRP file carrying Name values imports
// every row (tag stays empty, never a per-row truncation/sanitize entry)
// and the whole import produces exactly ONE non-blocking file-level
// warning that names were dropped — never a refusal, and never one
// warning per row.
func TestImportCHIRP_NoTagDropsNamesWithOneWarning(t *testing.T) {
	csv := "Location,Name,Frequency,Mode\n" +
		"1,Alpha,145.500000,USB\n" +
		"2,Bravo,146.500000,USB\n" +
		"3,,147.500000,USB\n"

	channels, report, err := ImportCHIRP(strings.NewReader(csv), noTagCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if len(channels) != 3 {
		t.Fatalf("len(channels) = %d, want 3 (a NoTag model never refuses a named row)", len(channels))
	}
	for _, ch := range channels {
		if ch.Data.Tag != "" {
			t.Errorf("slot %s: Tag = %q, want \"\" (a NoTag model has no field to hold it)", ch.Slot, ch.Data.Tag)
		}
	}
	if report.HasBlocking() {
		t.Fatalf("report.HasBlocking() = true, want false: %+v", report.Entries)
	}
	nameEntries := 0
	for _, e := range report.Entries {
		if e.Column == "Name" {
			nameEntries++
		}
	}
	if nameEntries != 1 {
		t.Fatalf("Name-column entries = %d, want exactly 1 (one warning per file, not per row)", nameEntries)
	}

	// A file with no Name values at all gets no warning: the whole point
	// is to flag names that were actually dropped, not to nag on every
	// NoTag import regardless of content.
	csvNoNames := "Location,Frequency,Mode\n1,145.500000,USB\n"
	_, report2, err := ImportCHIRP(strings.NewReader(csvNoNames), noTagCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	for _, e := range report2.Entries {
		if e.Column == "Name" {
			t.Errorf("unexpected Name-column entry with no Name column present: %+v", e)
		}
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
//
// Unchanged by M9c-6 (D-tagdisplay), and that is the half of that decision
// this test now also pins: Unknown remains the answer for a radio whose
// frame HAS the flag. The other half — a radio whose frame has none — is
// TestImportCHIRP_TagDisplayUnavailableWhenTheFrameHasNoFlag.
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

// ftdx10LikeCapabilities mirrors the FTdx10 fields ImportCHIRP consults
// (core/driver/ftdx10/caps.go), hand-built for the same reason
// ft710LikeCapabilities is: core/csvio sits below core/driver and must not
// import it, even in tests.
//
// The ONE difference that matters here is spec.FieldTagDisplay: the zero
// FieldSupport, on every bank and in every profile, because the FTdx10's
// combined MT record has no display flag at all — a MANUAL fact (that
// record's 41 positions are fully accounted for, and
// cat.Dialect.BuildMTSetCombined takes no display argument), not an
// assumption. Everything else is a 99-slot MEM bank with the same
// vocabularies, so a chirp.go that ignored capabilities would import
// identically against this fixture and the FT-710's.
func ftdx10LikeCapabilities() spec.Capabilities {
	caps := ft710LikeCapabilities()
	caps.Model = "FTdx10"
	caps.CATID = "0761"
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	banks := make([]spec.Bank, len(caps.Banks))
	copy(banks, caps.Banks)
	for i := range banks {
		banks[i].Fields = map[spec.Field]spec.FieldSupport{
			spec.FieldFrequency:  rw,
			spec.FieldMode:       rw,
			spec.FieldClarifier:  rw,
			spec.FieldCTCSSState: rw,
			spec.FieldShift:      rw,
			spec.FieldTag:        rw,
			// No display flag exists in this radio's memory frame.
			spec.FieldTagDisplay: {},
			spec.FieldCTCSSTone:  {},
			spec.FieldScanSkip:   {},
		}
	}
	caps.Banks = banks
	return caps
}

// TestImportCHIRP_TagDisplayUnavailableWhenTheFrameHasNoFlag is M9c-6
// D-tagdisplay's headline: the imported tag_display comes from the TARGET
// BANK's own support, so a radio whose memory frame carries no display
// flag imports Unavailable — "this radio has no such field" — rather than
// Unknown, which would say the answer is merely not yet known.
//
// The distinction is not cosmetic. Unknown is a question put to the user,
// and M9c-6 D5b opens an in-cell route for answering it (an Unknown
// tag-display cell toggles to Known-off on first press); an Unavailable
// one is refused by that cell AND by the paste path (M9c-5 review W2). An
// FTdx10 import that produced Unknown would therefore hand the user a way
// to manufacture a flag the radio cannot store, and a send plan carrying a
// Known tag_display for a radio whose Write support is Unsupported.
//
// Every row is asserted, including the bare minimal one: the derivation is
// per import, not per row, and a per-row divergence would mean the field
// had been reconstructed somewhere else.
func TestImportCHIRP_TagDisplayUnavailableWhenTheFrameHasNoFlag(t *testing.T) {
	csv := "Location,Name,Frequency,Duplex,Tone,rToneFreq,cToneFreq,Mode,Skip\n" +
		"1,MYCALL,145.500000,+,Tone,88.5,88.5,FM,S\n" +
		"2,,7.100000,,,,,USB,\n" +
		"3,GB3XX,430.925000,-,,,,FM,\n"

	channels, report, err := ImportCHIRP(strings.NewReader(csv), ftdx10LikeCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if report.HasBlocking() {
		t.Fatalf("unexpected blocking entries: %+v", report.Entries)
	}
	if len(channels) != 3 {
		t.Fatalf("len(channels) = %d, want 3", len(channels))
	}
	want := codeplug.BoolField{State: codeplug.Unavailable}
	for i, ch := range channels {
		if ch.Data == nil {
			t.Fatalf("channels[%d].Data = nil, want a populated channel", i)
		}
		if got := ch.Data.TagDisplay; got != want {
			t.Errorf("channels[%d].Data.TagDisplay = %+v, want %+v (this radio's memory frame has no display flag; Unknown would invite a value it cannot store)", i, got, want)
		}
	}
}

// TestImportCHIRP_TagDisplayFollowsTheTargetBank is the derivation itself,
// stated as a table over the support shapes a bank can declare, with the
// two real radios' own shapes named among them. Its job is to stop the
// rule collapsing back into a constant in either direction: the FT-710's
// side (Unknown) and the FTdx10's (Unavailable) both come out of ONE
// expression, and every intermediate shape — readable but unwritable, the
// merely unproven Unverified, the transmitted-but-ignored Inert — stays
// Unknown, because only "absent from the frame in BOTH directions"
// justifies Unavailable.
func TestImportCHIRP_TagDisplayFollowsTheTargetBank(t *testing.T) {
	const csv = "Location,Name,Frequency,Mode\n1,MYCALL,145.500000,FM\n"

	tests := []struct {
		name        string
		tagDisplay  spec.FieldSupport
		absentField bool
		want        codeplug.BoolField
	}{
		{name: "Read+Write Supported (the FT-710's own shape)", tagDisplay: spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}, want: codeplug.BoolField{State: codeplug.Unknown}},
		{name: "Read+Write Unverified (an unproven radio still has the flag)", tagDisplay: spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}, want: codeplug.BoolField{State: codeplug.Unknown}},
		{name: "readable, write Unsupported (the discovered 60M/EMG shape)", tagDisplay: spec.FieldSupport{Read: spec.Supported, Write: spec.Unsupported}, want: codeplug.BoolField{State: codeplug.Unknown}},
		{name: "writable, read Unsupported", tagDisplay: spec.FieldSupport{Read: spec.Unsupported, Write: spec.Supported}, want: codeplug.BoolField{State: codeplug.Unknown}},
		{name: "Inert write (transmitted-but-ignored is still a frame field)", tagDisplay: spec.FieldSupport{Read: spec.Supported, Write: spec.Inert}, want: codeplug.BoolField{State: codeplug.Unknown}},
		// The consented shape a session carries once the user has granted
		// unverified writes: Write ConsentedUnverified, Read left Unverified
		// (spec.ConsentUnverifiedWrites transforms the write side only). It
		// must read exactly as the plain-Unverified row above does — consent
		// is about authorising a write, and this derivation asks a different
		// question entirely, whether the frame HAS the flag.
		{name: "ConsentedUnverified write (consent does not make a flag appear or vanish)", tagDisplay: spec.FieldSupport{Read: spec.Unverified, Write: spec.ConsentedUnverified}, want: codeplug.BoolField{State: codeplug.Unknown}},
		{name: "both Unsupported (the FTdx10's own shape)", tagDisplay: spec.FieldSupport{}, want: codeplug.BoolField{State: codeplug.Unavailable}},
		{name: "field absent from the bank's map entirely", absentField: true, want: codeplug.BoolField{State: codeplug.Unavailable}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caps := ft710LikeCapabilities()
			banks := make([]spec.Bank, len(caps.Banks))
			copy(banks, caps.Banks)
			fields := make(map[spec.Field]spec.FieldSupport, len(banks[0].Fields))
			for f, fs := range banks[0].Fields {
				fields[f] = fs
			}
			if tc.absentField {
				delete(fields, spec.FieldTagDisplay)
			} else {
				fields[spec.FieldTagDisplay] = tc.tagDisplay
			}
			banks[0].Fields = fields
			caps.Banks = banks

			channels, _, err := ImportCHIRP(strings.NewReader(csv), caps)
			if err != nil {
				t.Fatalf("ImportCHIRP: unexpected error: %v", err)
			}
			if len(channels) != 1 {
				t.Fatalf("len(channels) = %d, want 1", len(channels))
			}
			if got := channels[0].Data.TagDisplay; got != tc.want {
				t.Errorf("TagDisplay = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestImportCHIRP_UnavailableTagDisplayDoesNotBlockTheDiff is the
// consequence of D-tagdisplay that makes it worth having, and the mirror
// image of TestImportCHIRP_UnknownTagDisplayBlocksTheDiff: an imported
// channel whose tag_display is Unavailable plans CLEANLY, because there is
// no question outstanding for the user to answer. Had the import produced
// Unknown against a radio with no display flag, every single imported
// channel would have been blocked at plan time by a gate the user could
// only clear by asserting a value the radio cannot store.
func TestImportCHIRP_UnavailableTagDisplayDoesNotBlockTheDiff(t *testing.T) {
	// writableCapabilities' permissive table with ONLY tag_display zeroed,
	// for the reason that fixture exists at all: every other field a
	// CHIRP-imported channel transmits stays write-Supported, so the only
	// thing that can block this entry is the tag_display gate.
	//
	// scan_skip is still one of those fields, and its permissiveness is
	// still load-bearing HERE: because this fixture declares scan_skip
	// writable, ImportCHIRP takes chirpScanSkip's LITERAL branch against
	// it and produces a Known one, which would meet the write gate and
	// mask the entry under test if the support were not permissive.
	//
	// What HAS changed (M9d-2 task 8) is the real-radio half of that
	// story. Against a real FT-710 or FTdx10 the masking no longer arises
	// at all: scan-skip is Unreachable on both, so a CHIRP import now
	// yields Unknown for it and the field never enters codeplug.Diff's
	// requestedFields. Before that fold it did, and this fixture was the
	// only thing keeping the tag_display gate visible.
	caps := writableCapabilities()
	banks := make([]spec.Bank, len(caps.Banks))
	copy(banks, caps.Banks)
	for i := range banks {
		fields := make(map[spec.Field]spec.FieldSupport, len(banks[i].Fields))
		for f, fs := range banks[i].Fields {
			fields[f] = fs
		}
		fields[spec.FieldTagDisplay] = spec.FieldSupport{}
		banks[i].Fields = fields
	}
	caps.Banks = banks
	csv := "Location,Name,Frequency,Mode\n2,MYCALL,145.500000,FM\n"

	imported, report, err := ImportCHIRP(strings.NewReader(csv), caps)
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if report.HasBlocking() {
		t.Fatalf("unexpected blocking loss entries: %+v", report.Entries)
	}
	if len(imported) != 1 {
		t.Fatalf("len(imported) = %d, want 1", len(imported))
	}

	// A baseline reading both slots as EMPTY, so the imported channel is an
	// Added delta — the shape a real "import into a fresh read" produces.
	// Same two-slot inventory on each side, per Diff's contract.
	newCodeplug := func(ch *codeplug.Channel) *codeplug.Codeplug {
		channels := []codeplug.Channel{{Slot: "001"}, {Slot: "002"}}
		if ch != nil {
			channels[1] = *ch
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
	if entry.Blocked {
		t.Errorf("slot 002 Blocked = true (%q), want false — an Unavailable tag_display asks the user nothing, so there is nothing for the plan to wait on", entry.BlockReason)
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

// ftdx101LikeCapabilities mirrors ONE thing about the FTdx101 faithfully:
// its per-bank field-support map (core/driver/ftdx101/caps.go's
// bankFields), which is the only part of that radio's capabilities these
// scan-skip and tag-display tests read. Everything ELSE is inherited
// unexamined from ft710LikeCapabilities and is NOT a claim about the real
// radio — the mode/tone/shift/CTCSS vocabularies and TagLen below are the
// FT-710's, and spec.FieldErase is omitted altogether. Nothing here asks
// this fixture an FTdx101-specific question about any of them, and a test
// that needed one would have to widen the fixture first rather than trust
// it. It is hand-built rather than taken from the driver for the same
// reason ft710LikeCapabilities and ftdx10LikeCapabilities are: core/csvio
// sits below core/driver and must not import it, even in tests.
//
// model/catID pick the sibling: "FTdx101D"/"0681" or "FTdx101MP"/"0682"
// (core/driver/ftdx101/ftdx101.go's modelD/modelMP). The two differ in
// NOTHING this package can see — ftdx101/caps.go's bankFields is one
// function serving both models, and its doc comment's matrix §2.5
// citation is why (the manual prints the memory-channel surface once, with
// no model qualifier) — so both fixtures are built from one constructor
// rather than two, and the tests still name them separately because the
// registry does.
//
// The field map is ftdx101/caps.go's bankFields shape: tag_display the
// zero FieldSupport (a manual-evidenced absence — the combined MT record
// has no display flag), and ctcss_tone/scan_skip the zero FieldSupport
// too, there on the weaker ASSUMED footing of that driver's register
// entry 6. This fixture's job is only to carry the scan_skip answer
// faithfully; the bank geometry ("001".."099", ftdx101/caps.go's memSlots)
// happens to match the other two radios' exactly, and no test here depends
// on that.
func ftdx101LikeCapabilities(model, catID string) spec.Capabilities {
	caps := ft710LikeCapabilities()
	caps.Model = model
	caps.CATID = catID
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	banks := make([]spec.Bank, len(caps.Banks))
	copy(banks, caps.Banks)
	for i := range banks {
		banks[i].Fields = map[spec.Field]spec.FieldSupport{
			spec.FieldFrequency:  rw,
			spec.FieldMode:       rw,
			spec.FieldClarifier:  rw,
			spec.FieldCTCSSState: rw,
			spec.FieldShift:      rw,
			spec.FieldTag:        rw,
			// No display flag exists in this radio's combined MT record.
			spec.FieldTagDisplay: {},
			// ASSUMED unreachable — that driver's register entry 6.
			spec.FieldCTCSSTone: {},
			spec.FieldScanSkip:  {},
		}
	}
	caps.Banks = banks
	return caps
}

// unreachableScanSkipCapabilities returns one fixture per registered model
// whose scan_skip is UNREACHABLE — the models the caps-aware branch (M9d-2
// task 8, spec decision 5) applies to. It is a table of FIXTURES rather than
// a walk of the registry for the layering reason ft710LikeCapabilities gives
// — core/csvio must not import core/driver, and internal/wiring imports every
// driver — so each entry cites the caps site it mirrors, and drift between
// the two is caught end-to-end by the CLI byte-identity baseline, which does
// use the real drivers.
//
// RENAMED AT TIER 6, AND THE OLD NAME WAS A CLAIM THIS FUNCTION NEVER MADE
// GOOD. It was registeredRadioCapabilities, documented as "one fixture per
// model registered in internal/wiring's driver tables"; it has in fact never
// held more than the Yaesu rows, and the eleven Icom models registered
// between M9d-2 and Tier 4b have no CHIRP fixture in this file at all.
// chirpFixtures below is what now carries the whole set this file DOES cover,
// and TestChirpFixtures_CoverEveryRegisteredModel is what measures it against
// the registry.
//
// THE MEMBERSHIP RULE IS THE UNREACHABLE ONE, and it is a precondition rather
// than a preference: TestImportCHIRP_ScanSkipIsCapabilityAware Fatals on a
// fixture whose scan_skip is reachable, because that radio takes the LITERAL
// branch and every assertion in that test is about the other one. The TS-590
// pair is the first registered family of which that is true — its 50-byte
// record carries a channel-lockout flag at a printed position — so those two
// fixtures live in chirpFixtures and NOT here, and
// TestImportCHIRP_TS590PairTakesTheLiteralScanSkipBranch is their coverage.
func unreachableScanSkipCapabilities() []spec.Capabilities {
	return []spec.Capabilities{
		ft710LikeCapabilities(),
		ftdx10LikeCapabilities(),
		ftdx101LikeCapabilities("FTdx101D", "0681"),
		ftdx101LikeCapabilities("FTdx101MP", "0682"),
		// The FT-891 (Tier 1), the two-place change this function's doc
		// comment asks for. Its scan_skip is the zero FieldSupport too
		// (ft891/caps.go's bankFields, capability matrix §2.3: the
		// 41-position combined record carries no skip flag anywhere), so
		// it lands in the caps-aware branch with the other four and the
		// Unreachable precondition below holds for it unchanged.
		ft891LikeCapabilities(),
		// The FT-991A (Tier 1), the same two-place change. Its scan_skip
		// is the zero FieldSupport as well (ft991a/caps.go's bankFields,
		// capability matrix §2.4: no position in the 41-position record
		// marks a channel for scan skip), so it lands in the caps-aware
		// branch with the other five. Its CTCSSStates do NOT match the
		// others — five members, the last two DCS — which is what
		// TestImportCHIRP_DTCSRefusalReasonFollowsTheRecord drives, and
		// that test fails outright if this row is dropped.
		ft991aLikeCapabilities(),
	}
}

// icXXXXLikeCapabilities (ic7800LikeCapabilities, ic7600LikeCapabilities,
// ic7410LikeCapabilities, ic7700LikeCapabilities, ic9100LikeCapabilities,
// ic7200LikeCapabilities) return the v1.7.0 Icom wave's own REGISTERED
// capabilities verbatim, via
// wiring.StaticCapabilities — unlike every fixture above, which is a
// hand-written literal shadowing its driver.
//
// THAT IS DELIBERATE, not a shortcut this file's own convention argues
// against: the debt-ledger comment on chirpFixtureExceptions above warns
// against "writing eleven radios' worth of UNEVIDENCED capability data",
// and a hand-invented literal here would be exactly that — a second,
// independently-typed claim about a radio this package has already
// registered capabilities for. Reading the real, registered value instead
// is the more evidenced choice, and it cannot drift from the driver the
// way a hand-copied literal could.
//
// NOT IN unreachableScanSkipCapabilities(), deliberately: that function's
// OTHER two callers (TestImportCHIRP_DTCSRefusalReasonFollowsTheRecord in
// particular) assume every fixture's ToneModes recognises DTCS/Cross as a
// tone TYPE even where it cannot write the DCS CODE — true of every
// hand-written fixture there, but not of these six, whose own narrower
// tone-type nibble does not admit a DTCS/Cross reading at all (their own
// capability reviews) — the IC-9100's own record maps DTCS CODE and
// POLARITY as tier fields (richer than every sibling in this wave) but
// still declares no DTCS/Cross TONE STATE, so it takes the same branch;
// the IC-7200 has no tone field of ANY kind at all, a stricter absence
// still, and takes the identical branch for the plainer reason. ImportCHIRP
// therefore refuses their DTCS/Cross cells on an earlier, differently-worded
// branch (chirp.go's "expresses no %s tone mode"), which that test does not
// expect. These six need only the ONE completeness property
// chirpFixtures() exists for — see
// its own call site below — not membership in a bucket whose other tests
// assume a tone vocabulary they do not have.
func ic7800LikeCapabilities() spec.Capabilities {
	caps, err := wiring.StaticCapabilities(wiring.IC7800Model)
	if err != nil {
		panic(fmt.Sprintf("chirp_test: wiring.StaticCapabilities(%q): %v", wiring.IC7800Model, err))
	}
	return caps
}

func ic7600LikeCapabilities() spec.Capabilities {
	caps, err := wiring.StaticCapabilities(wiring.IC7600Model)
	if err != nil {
		panic(fmt.Sprintf("chirp_test: wiring.StaticCapabilities(%q): %v", wiring.IC7600Model, err))
	}
	return caps
}

func ic7410LikeCapabilities() spec.Capabilities {
	caps, err := wiring.StaticCapabilities(wiring.IC7410Model)
	if err != nil {
		panic(fmt.Sprintf("chirp_test: wiring.StaticCapabilities(%q): %v", wiring.IC7410Model, err))
	}
	return caps
}

func ic7700LikeCapabilities() spec.Capabilities {
	caps, err := wiring.StaticCapabilities(wiring.IC7700Model)
	if err != nil {
		panic(fmt.Sprintf("chirp_test: wiring.StaticCapabilities(%q): %v", wiring.IC7700Model, err))
	}
	return caps
}

func ic9100LikeCapabilities() spec.Capabilities {
	caps, err := wiring.StaticCapabilities(wiring.IC9100Model)
	if err != nil {
		panic(fmt.Sprintf("chirp_test: wiring.StaticCapabilities(%q): %v", wiring.IC9100Model, err))
	}
	return caps
}

func ic7200LikeCapabilities() spec.Capabilities {
	caps, err := wiring.StaticCapabilities(wiring.IC7200Model)
	if err != nil {
		panic(fmt.Sprintf("chirp_test: wiring.StaticCapabilities(%q): %v", wiring.IC7200Model, err))
	}
	return caps
}

// ftdx5000LikeCapabilities returns the FTdx5000's own REGISTERED
// capabilities verbatim (v1.7.0 Kenwood/Yaesu wave, tenth row) — see
// ic7200LikeCapabilities' own doc comment for why this is a
// wiring.StaticCapabilities call rather than a hand-written fixture, and
// for why it goes straight into chirpFixtures rather than
// unreachableScanSkipCapabilities (that bucket is fixed to the
// hand-written fixtures TestImportCHIRP_ScanSkipIsCapabilityAware asserts
// against).
func ftdx5000LikeCapabilities() spec.Capabilities {
	caps, err := wiring.StaticCapabilities(wiring.FTdx5000Model)
	if err != nil {
		panic(fmt.Sprintf("chirp_test: wiring.StaticCapabilities(%q): %v", wiring.FTdx5000Model, err))
	}
	return caps
}

// ts2000LikeCapabilities returns the TS-2000's own REGISTERED capabilities
// verbatim (v1.7.0 Kenwood/Yaesu wave, first row) — see
// ic7200LikeCapabilities' own doc comment for why this is a
// wiring.StaticCapabilities call rather than a hand-written fixture. Its
// scan_skip IS reachable (byte 19, the family's channel-lockout flag), so
// this fixture goes straight into chirpFixtures rather than
// unreachableScanSkipCapabilities, on the TS-590 pair's footing.
func ts2000LikeCapabilities() spec.Capabilities {
	caps, err := wiring.StaticCapabilities(wiring.TS2000Model)
	if err != nil {
		panic(fmt.Sprintf("chirp_test: wiring.StaticCapabilities(%q): %v", wiring.TS2000Model, err))
	}
	return caps
}

// ts2000xLikeCapabilities is the TS-2000X's own REGISTERED capabilities
// (v1.7.0 Kenwood/Yaesu wave, second row) — same package, zero byte
// difference from the TS-2000's, so this fixture exists only because
// chirpFixtures needs one entry per registered model.
func ts2000xLikeCapabilities() spec.Capabilities {
	caps, err := wiring.StaticCapabilities(wiring.TS2000XModel)
	if err != nil {
		panic(fmt.Sprintf("chirp_test: wiring.StaticCapabilities(%q): %v", wiring.TS2000XModel, err))
	}
	return caps
}

// tsb2000LikeCapabilities is the TS-B2000's own REGISTERED capabilities
// (v1.7.0 Kenwood/Yaesu wave, third and last ts2000 row).
func tsb2000LikeCapabilities() spec.Capabilities {
	caps, err := wiring.StaticCapabilities(wiring.TSB2000Model)
	if err != nil {
		panic(fmt.Sprintf("chirp_test: wiring.StaticCapabilities(%q): %v", wiring.TSB2000Model, err))
	}
	return caps
}

// ts570dLikeCapabilities returns the TS-570D's own REGISTERED capabilities
// verbatim (v1.7.0 Kenwood/Yaesu wave, fourth row). Its scan_skip IS
// reachable (byte 19, this family's channel-lockout flag), so this
// fixture goes straight into chirpFixtures, on the TS-590 pair's footing.
func ts570dLikeCapabilities() spec.Capabilities {
	caps, err := wiring.StaticCapabilities(wiring.TS570DModel)
	if err != nil {
		panic(fmt.Sprintf("chirp_test: wiring.StaticCapabilities(%q): %v", wiring.TS570DModel, err))
	}
	return caps
}

// ts570sLikeCapabilities is the TS-570S's own REGISTERED capabilities
// (v1.7.0 Kenwood/Yaesu wave, fifth row).
func ts570sLikeCapabilities() spec.Capabilities {
	caps, err := wiring.StaticCapabilities(wiring.TS570SModel)
	if err != nil {
		panic(fmt.Sprintf("chirp_test: wiring.StaticCapabilities(%q): %v", wiring.TS570SModel, err))
	}
	return caps
}

// ts870sLikeCapabilities returns the TS-870S's own REGISTERED capabilities
// verbatim (v1.7.0 Kenwood/Yaesu wave, seventh row). Its scan_skip IS
// reachable (byte 18, this record's own channel-lockout flag), so this
// fixture goes straight into chirpFixtures on the TS-590 pair's footing.
func ts870sLikeCapabilities() spec.Capabilities {
	caps, err := wiring.StaticCapabilities(wiring.TS870SModel)
	if err != nil {
		panic(fmt.Sprintf("chirp_test: wiring.StaticCapabilities(%q): %v", wiring.TS870SModel, err))
	}
	return caps
}

// skipEntries returns every LossEntry the report holds for the Skip
// column, in order. The scan-skip tests assert on this slice alone: a row
// may legitimately produce OTHER columns' entries (an FTdx10/FTdx101
// import drops nothing extra here, but the assertion should not depend on
// that), and what is being pinned is the Skip rule.
func skipEntries(r LossReport) []LossEntry {
	var out []LossEntry
	for _, e := range r.Entries {
		if e.Column == "Skip" {
			out = append(out, e)
		}
	}
	return out
}

// TestImportCHIRP_ScanSkipIsCapabilityAware is M9d-2 task 8's headline
// (spec decision 5), driven over EVERY registered radio's real
// capabilities: where scan-skip is Unreachable — today, all four — a
// CHIRP file's Skip column can no longer produce a Known scan_skip,
// because a Known one is a CLAIM this radio's protocol cannot carry.
//
// The blank cell is the case that mattered: it used to import
// {Known,false}, which put spec.FieldScanSkip into codeplug.Diff's
// requestedFields for EVERY imported channel (diff.go's requestedFields),
// and the all-or-nothing write gate then blocked every one of them — a
// clean three-row import planned as "Blocked 3" (the M9c-6 manifest's A7
// finding). Unknown says the truthful thing (the file has told us nothing
// this radio can act on) and asks the user nothing, because there is
// nothing for the user to answer.
//
// The "S" cell is a real intent that cannot be honoured, so it is DROPPED
// with a non-blocking loss entry rather than silently discarded: the user
// asked for a scan skip, this radio has no way to store one, and the
// report says so per row. Blocking stays false — refusing to import a
// channel over a flag the radio does not have would be the same
// over-blocking A7 recorded, one layer up.
//
// The unrecognised-value arm is unchanged in both worlds and is pinned
// here as well so that the shared arm cannot drift under one branch only.
func TestImportCHIRP_ScanSkipIsCapabilityAware(t *testing.T) {
	const csv = "Location,Name,Frequency,Duplex,Tone,rToneFreq,cToneFreq,Mode,Skip\n" +
		"1,BLANK,145.500000,,,,,FM,\n" +
		"2,SKIPPED,145.525000,,,,,FM,S\n" +
		"3,ODD,145.550000,,,,,FM,P\n"

	for _, caps := range unreachableScanSkipCapabilities() {
		t.Run(caps.Model, func(t *testing.T) {
			fs := caps.FieldSupport(spec.BankMemory, spec.FieldScanSkip)
			if !fs.Unreachable() {
				t.Fatalf("fixture precondition: %s scan_skip = %+v, want Unreachable — this test is about the unreachable branch", caps.Model, fs)
			}

			channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
			if err != nil {
				t.Fatalf("ImportCHIRP() error = %v", err)
			}
			if len(channels) != 3 {
				t.Fatalf("imported %d channels, want 3", len(channels))
			}

			// Blank Skip: Unknown, and NOTHING reported — the file simply
			// said nothing, which is not a loss.
			for i, ch := range channels {
				if ch.Data == nil {
					t.Fatalf("channels[%d].Data = nil", i)
				}
			}
			if got := channels[0].Data.ScanSkip; got != (codeplug.BoolField{State: codeplug.Unknown}) {
				t.Errorf("blank Skip -> ScanSkip = %+v, want {Unknown} on %s: a Known false is a claim this radio's protocol cannot carry, and it blocked every imported channel", got, caps.Model)
			}
			for _, e := range skipEntries(report) {
				if e.Line == 2 {
					t.Errorf("blank Skip produced a loss entry on %s: %+v — an absent cell is not a loss", caps.Model, e)
				}
			}

			// "S": Unknown plus a NON-BLOCKING dropped entry naming the
			// radio.
			if got := channels[1].Data.ScanSkip; got != (codeplug.BoolField{State: codeplug.Unknown}) {
				t.Errorf("Skip=S -> ScanSkip = %+v, want {Unknown} on %s", got, caps.Model)
			}
			wantS := LossEntry{
				Line: 3, Column: "Skip", Value: "S", Action: ActionDropped, Blocking: false,
				Detail: fmt.Sprintf("CHIRP Skip \"S\" dropped: scan-skip is not reachable over CAT on %s; scan-skip left unresolved", caps.Model),
			}
			var gotS []LossEntry
			for _, e := range skipEntries(report) {
				if e.Line == 3 {
					gotS = append(gotS, e)
				}
			}
			if len(gotS) != 1 || gotS[0] != wantS {
				t.Errorf("Skip=S entries on %s = %+v, want exactly [%+v]", caps.Model, gotS, wantS)
			}

			// "P": today's unrecognised arm, byte-identical in both worlds.
			if got := channels[2].Data.ScanSkip; got != (codeplug.BoolField{State: codeplug.Unknown}) {
				t.Errorf("Skip=P -> ScanSkip = %+v, want {Unknown} on %s", got, caps.Model)
			}
			wantP := LossEntry{
				Line: 4, Column: "Skip", Value: "P", Action: ActionDropped, Blocking: false,
				Detail: fmt.Sprintf("CHIRP Skip value \"P\" has no %s equivalent; scan-skip left unresolved", caps.Model),
			}
			var gotP []LossEntry
			for _, e := range skipEntries(report) {
				if e.Line == 4 {
					gotP = append(gotP, e)
				}
			}
			if len(gotP) != 1 || gotP[0] != wantP {
				t.Errorf("Skip=P entries on %s = %+v, want exactly [%+v]", caps.Model, gotP, wantP)
			}
		})
	}
}

// TestImportCHIRP_ScanSkipLiteralOnAWritableRadio pins the OTHER branch —
// the one no registered radio takes today. On a radio whose scan-skip is
// genuinely reachable, the CHIRP file's Skip column means exactly what it
// says and the reading is the literal, pre-M9d-2 one: blank is a real
// "do not skip" ({Known,false}), "S" is a real "skip" ({Known,true}), and
// neither loses anything worth reporting.
//
// Without this the caps-aware fold would be indistinguishable from simply
// deleting the Known arm, and the first radio registered with a writable
// scan-skip would silently import as if it had none.
//
// TWO ROWS, because two different labels reach this branch and they are
// different KINDS of claim. spec.Supported is hardware evidence.
// spec.ConsentedUnverified is the label a session carries once the user has
// granted unverified writes (spec.ConsentUnverifiedWrites), and it must
// import IDENTICALLY: Unreachable asks whether both directions are
// Unsupported, which a consented write label is not, so the literal reading
// applies to a consented radio exactly as it does to a proven one. The row
// fails the moment the predicate is narrowed to test Supported.
func TestImportCHIRP_ScanSkipLiteralOnAWritableRadio(t *testing.T) {
	const csv = "Location,Name,Frequency,Duplex,Tone,rToneFreq,cToneFreq,Mode,Skip\n" +
		"1,BLANK,145.500000,,,,,FM,\n" +
		"2,SKIPPED,145.525000,,,,,FM,S\n"

	tests := []struct {
		name     string
		scanSkip spec.FieldSupport
	}{
		{"hardware-proven scan skip", spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}},
		{"consented scan skip (the user's grant, not hardware evidence)", spec.FieldSupport{Read: spec.Unverified, Write: spec.ConsentedUnverified}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caps := writableCapabilities()
			banks := make([]spec.Bank, len(caps.Banks))
			copy(banks, caps.Banks)
			for i := range banks {
				fields := make(map[spec.Field]spec.FieldSupport, len(banks[i].Fields))
				for f, fs := range banks[i].Fields {
					fields[f] = fs
				}
				fields[spec.FieldScanSkip] = tc.scanSkip
				banks[i].Fields = fields
			}
			caps.Banks = banks

			if fs := caps.FieldSupport(spec.BankMemory, spec.FieldScanSkip); fs.Unreachable() {
				t.Fatalf("fixture precondition: scan_skip = %+v, want reachable", fs)
			}

			channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
			if err != nil {
				t.Fatalf("ImportCHIRP() error = %v", err)
			}
			if len(channels) != 2 {
				t.Fatalf("imported %d channels, want 2", len(channels))
			}
			if got := channels[0].Data.ScanSkip; got != (codeplug.BoolField{State: codeplug.Known, Value: false}) {
				t.Errorf("blank Skip -> ScanSkip = %+v, want {Known,false} on a radio that can store it", got)
			}
			if got := channels[1].Data.ScanSkip; got != (codeplug.BoolField{State: codeplug.Known, Value: true}) {
				t.Errorf("Skip=S -> ScanSkip = %+v, want {Known,true} on a radio that can store it", got)
			}
			if got := skipEntries(report); len(got) != 0 {
				t.Errorf("Skip entries = %+v, want none: nothing is lost when the radio can store the answer", got)
			}
		})
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
				// Blank Skip on the FT-710 — whose scan-skip is
				// Unreachable — is Unknown, not Known/false: M9d-2 task 8
				// (spec decision 5). See
				// TestImportCHIRP_ScanSkipIsCapabilityAware for the rule and
				// the A7 over-blocking it fixes.
				if d.ScanSkip.State != codeplug.Unknown {
					t.Errorf("ScanSkip = %+v, want Unknown", d.ScanSkip)
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
				// Skip=S on a radio that cannot store one: Unknown plus the
				// non-blocking dropped entry below (M9d-2 task 8).
				if d.ScanSkip.State != codeplug.Unknown {
					t.Errorf("ScanSkip = %+v, want Unknown", d.ScanSkip)
				}
			},
			want: []wantEntry{{3, "Offset", "dropped", false}, {3, "Skip", "dropped", false}},
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
//
// The fixture moved from 5000 MHz to a value that overflows the
// multiplication itself, because the Icom tier widened this parser
// (design D4, adjudication 5) and 5 GHz is now perfectly representable
// — the case this test is about is an UNREPRESENTABLE frequency, which
// is a different thing from one this radio cannot store. That second
// question has not gone unanswered: codeplug.Validate refuses a 5 GHz
// channel against the FT-710's own MaxFreqHz, which is where a per-radio
// bound belongs.
func TestImportCHIRP_FrequencyOutOfRange(t *testing.T) {
	body := "Location,Name,Frequency,Mode\n1,TESTCH,18446744073710.500000,FM\n"
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
// two overflow paths (intPart too large for uint64; the total exceeding
// what a uint64 of hertz can hold).
//
// The RANGE cases moved with the Icom tier's widening of this parser
// (design D4, adjudication 5), and the move is the point rather than an
// incidental fixup: the ceiling here is what is REPRESENTABLE, and a
// 5 GHz cell — refused before, when the representable ceiling happened
// to be MaxUint32 — is now parsed, leaving "can THIS radio store it" to
// codeplug.Validate and that radio's own MaxFreqHz. The wrap-around case
// keeps its place unchanged: an unrepresentable value must still be
// refused rather than silently folded into a plausible small one.
func TestParseCHIRPFrequency(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    uint64
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
		{"5 GHz is representable now, and is the radio's question", "5000.000000", 5_000_000_000, nil},
		{"the old MaxUint32 boundary is no longer a boundary", "4294.967295", math.MaxUint32, nil},
		{"one Hz past the old boundary is accepted", "4294.967296", math.MaxUint32 + 1, nil},
		{"10 GHz, the IC-905's reach", "10000.000000", 10_000_000_000, nil},
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
// multi-byte rune). Both fall outside spec.Capabilities.TagByteOK's
// DEFAULT printable-ASCII range and must be replaced with a space,
// producing exactly one non-blocking ActionApproximated LossEntry on the
// Name column.
//
// The last two cases are why the predicate is caps' rather than this
// package's own: TagByteOK honours a PUBLISHED TagCharset, nine Icom
// drivers supply one, and three of those (IC-7760, IC-7851, IC-R8600)
// print a charset containing ';'. A literal here replaced a byte those
// radios accept and then described a rule they do not use.
func TestSanitizeCHIRPName(t *testing.T) {
	// withTagCharset is the per-model half of this test: a fixture that
	// differs from the default one in NOTHING but the charset it
	// publishes, so a case below can only be answering the charset
	// question.
	withTagCharset := func(set string) spec.Capabilities {
		caps := ft710LikeCapabilities()
		caps.TagCharset = set
		return caps
	}
	cases := []struct {
		name    string
		caps    spec.Capabilities
		in      string
		want    string
		entries int
		// detail, when non-empty, must appear in the single entry: the
		// rule that rejected the byte, as the message states it.
		detail string
	}{
		// The default arm, unchanged: a radio publishing no charset is
		// judged by printable ASCII 0x20-0x7E excluding ';', exactly as
		// before.
		{"non-printable control byte (0x07 BEL)", ft710LikeCapabilities(), "A\x07B", "A B", 1, "printable ASCII 0x20-0x7E, excluding ';'"},
		{"non-ASCII byte (0xC3)", ft710LikeCapabilities(), "A\xC3B", "A B", 1, "printable ASCII 0x20-0x7E, excluding ';'"},
		// A radio whose PUBLISHED charset contains ';' keeps it. The
		// default excludes ';' because it terminates a NEWCAT frame, but
		// the IC-7760, IC-7851 and IC-R8600 each print a name-charset
		// table that contains one — replacing it there sanitised a byte
		// the radio accepts and described a rule it does not use.
		{"a published charset containing ';' keeps it", withTagCharset("AB;"), "A;B", "A;B", 0, ""},
		// The converse, so the case above cannot pass by the charset
		// being ignored altogether: a byte the DEFAULT admits but this
		// charset omits is still replaced, and the Detail names the
		// charset that rejected it rather than the default wording.
		{"a published charset omitting a printable byte replaces it", withTagCharset("AB "), "AxB", "A B", 1, `this radio's tag charset "AB "`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, entries := sanitizeCHIRPName(1, tc.in, tc.caps)
			if got != tc.want {
				t.Errorf("sanitizeCHIRPName(%q) tag = %q, want %q", tc.in, got, tc.want)
			}
			if len(entries) != tc.entries {
				t.Fatalf("sanitizeCHIRPName(%q) = %d LossEntries, want %d: %+v", tc.in, len(entries), tc.entries, entries)
			}
			if tc.entries == 0 {
				return
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
			if !strings.Contains(e.Detail, tc.detail) {
				t.Errorf("LossEntry.Detail = %q, want it to state the rule that rejected the byte, %q", e.Detail, tc.detail)
			}
		})
	}

	// The default-charset sentence must be BYTE-IDENTICAL to v1.4.1's —
	// L2 widens sanitizeCHIRPName's PREDICATE to a radio's published
	// charset, it does not change what a default-charset radio (the
	// FT-710 among them) says when it rejects a byte.
	t.Run("default-charset Detail is byte-identical to v1.4.1", func(t *testing.T) {
		_, entries := sanitizeCHIRPName(1, "A\x07B", ft710LikeCapabilities())
		if len(entries) != 1 {
			t.Fatalf("%d LossEntries, want 1: %+v", len(entries), entries)
		}
		const want = `Name contained a byte outside the FT-710 tag charset (printable ASCII 0x20-0x7E, excluding ';'); replaced with a space`
		if got := entries[0].Detail; got != want {
			t.Errorf("Detail = %q, want %q", got, want)
		}
		t.Logf("Detail = %q", entries[0].Detail)
	})
}

// --- line endings and the byte-order mark (decision 8) ---

// TestImportCHIRP_CRLFAndBOM_ImportIdentically is the CHIRP twin of
// TestImport_CRLFAndBOM_ImportIdentically: a CHIRP CSV saved by a Windows
// spreadsheet — CRLF, and CRLF behind a UTF-8 BOM — must yield the same
// channels AND the same loss report as the LF original.
//
// The report matters as much as the channels here: a BOM stuck to
// "Location" did not merely rename a column, it made a core column look
// missing, so ImportCHIRP refused the file outright rather than reporting
// what it could not carry.
func TestImportCHIRP_CRLFAndBOM_ImportIdentically(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "chirp_sample.csv"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	lf, crlf, bomCRLF := csvTwins(t, fixture)

	wantChannels, wantReport, err := ImportCHIRP(bytes.NewReader(lf), ft710LikeCapabilities())
	if err != nil {
		t.Fatalf("ImportCHIRP(LF): unexpected error: %v", err)
	}
	if len(wantChannels) == 0 || len(wantReport.Entries) == 0 {
		t.Fatalf("fixture yielded %d channels and %d loss entries — it cannot pin anything", len(wantChannels), len(wantReport.Entries))
	}

	for _, tc := range []struct {
		name string
		in   []byte
	}{
		{"CRLF", crlf},
		{"BOM+CRLF", bomCRLF},
		{"BOM+LF", append([]byte("\xEF\xBB\xBF"), lf...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotChannels, gotReport, err := ImportCHIRP(bytes.NewReader(tc.in), ft710LikeCapabilities())
			if err != nil {
				t.Fatalf("ImportCHIRP(%s): unexpected error: %v", tc.name, err)
			}
			if !channelsEqual(t, gotChannels, wantChannels) {
				t.Errorf("ImportCHIRP(%s) produced different channels from the LF original", tc.name)
			}
			if !reflect.DeepEqual(gotReport, wantReport) {
				t.Errorf("ImportCHIRP(%s) loss report differs from the LF original:\n got  %+v\n want %+v", tc.name, gotReport.Entries, wantReport.Entries)
			}
		})
	}
}

// TestImportCHIRP_MissingCoreColumnStillRefused pins that stripping the BOM
// did not soften the core-column gate: a header genuinely missing Location
// is still refused, with the same text whether or not a BOM precedes it.
func TestImportCHIRP_MissingCoreColumnStillRefused(t *testing.T) {
	const wrong = "Frequency,Mode\n146.500000,FM\n"

	_, _, errBare := ImportCHIRP(strings.NewReader(wrong), ft710LikeCapabilities())
	_, _, errBOM := ImportCHIRP(strings.NewReader("\xEF\xBB\xBF"+wrong), ft710LikeCapabilities())

	if errBare == nil || errBOM == nil {
		t.Fatalf("want an error from both, got %v (bare) and %v (BOM)", errBare, errBOM)
	}
	if errBare.Error() != errBOM.Error() {
		t.Errorf("error text differs:\n bare = %q\n BOM  = %q", errBare, errBOM)
	}
	if !strings.Contains(errBare.Error(), "Location") {
		t.Errorf("error = %q, want it to name the missing core column", errBare)
	}
}

// TestImportCHIRP_EmptyAndBOMOnlyInput is the CHIRP twin of
// TestImport_EmptyAndBOMOnlyInput: the degenerate inputs the strip must
// survive without panicking or succeeding.
func TestImportCHIRP_EmptyAndBOMOnlyInput(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"BOM only", "\xEF\xBB\xBF"},
		{"truncated BOM", "\xEF\xBB"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := ImportCHIRP(strings.NewReader(tc.in), ft710LikeCapabilities())
			if err == nil {
				t.Fatalf("ImportCHIRP(%s): want an error, got %d channels", tc.name, len(got))
			}
			var pe *ParseError
			if !errors.As(err, &pe) {
				t.Fatalf("error is %T, want *ParseError", err)
			}
			if pe.Line != 1 {
				t.Errorf("ParseError.Line = %d, want 1", pe.Line)
			}
		})
	}
}

// ft891LikeCapabilities mirrors the FT-891 fields ImportCHIRP consults
// (core/driver/ft891/caps.go). Hand-built rather than the real driver's
// Capabilities for the layering reason ft710LikeCapabilities gives:
// core/csvio sits BELOW core/driver in the import graph and must not depend
// on it, even in tests.
//
// THE ONE FIELD THAT MATTERS HERE IS Modes, and it is the reason this
// fixture exists rather than a reuse of ftdx10LikeCapabilities. Every Yaesu
// fixture above lists the family display names "CW-U", "CW-L" and "RTTY-U";
// the FT-891's own mode legend, transcribed once into core/cat/ft891's
// dialect from three identical printings (MR's P6 at layout 972-974, MT's
// at 1007-1010, MW's at 1043-1046), prints "CW", "CW-R", "RTTY-LSB" and
// "RTTY-USB" instead — twelve names, with a printed HOLE at nibble 'A' and
// no 'E' or 'F' at all. The driver DERIVES caps.Modes from that dialect
// rather than transcribing a second list (core/driver/ft891/caps.go's
// modeNames), so these twelve are the radio's own names in the radio's own
// wire-code order.
//
// PMS IS ABSENT FROM THIS FIXTURE and that is deliberate, not an omission:
// ImportCHIRP writes into the MEMORY bank alone (memBank), so a second bank
// would change nothing any assertion below can see. The real driver has one.
//
// The two zeroed fields mirror the real driver's for the reason every Yaesu
// fixture's do: this radio's 41-position combined record carries no
// tone-NUMBER byte and no scan-skip flag (capability matrix §2.3), so a
// CHIRP Skip cell has nowhere to go. FieldTagDisplay is rw here, unlike the
// FTdx10 fixture's zero — byte 28 is a LIVE flag on this radio (§3.7).
func ft891LikeCapabilities() spec.Capabilities {
	caps := ft710LikeCapabilities()
	caps.Model = "FT-891"
	caps.CATID = "0650"
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	banks := make([]spec.Bank, len(caps.Banks))
	copy(banks, caps.Banks)
	for i := range banks {
		banks[i].Fields = map[spec.Field]spec.FieldSupport{
			spec.FieldFrequency:  rw,
			spec.FieldMode:       rw,
			spec.FieldClarifier:  rw,
			spec.FieldCTCSSState: rw,
			spec.FieldShift:      rw,
			spec.FieldTag:        rw,
			// A LIVE display flag, unlike the FTdx10's and FTdx101's.
			spec.FieldTagDisplay: rw,
			spec.FieldCTCSSTone:  {},
			spec.FieldScanSkip:   {},
		}
	}
	caps.Banks = banks
	caps.Modes = []string{
		"LSB", "USB", "CW", "FM", "AM", "RTTY-LSB",
		"CW-R", "DATA-LSB", "RTTY-USB", "FM-N", "DATA-USB", "AM-N",
	}
	return caps
}

// TestImportCHIRP_FT891BlocksCWAndRTTYRows is Tier 1's CHIRP pin, and it
// records a LIMITATION rather than celebrating a behaviour.
//
// chirpModeMap resolves CHIRP's "CW" to "CW-U", its "CWR" to "CW-L" and its
// "RTTY" to "RTTY-U" — the sideband-specific names FOUR OF THE SIX
// registered Yaesu models print (the FT-710's core/cat mode table, the
// FTdx10's, and the FTdx101 pair's shared one). The FT-891 prints "CW",
// "CW-R", "RTTY-LSB" and "RTTY-USB", so none of those three mapped names is
// in its caps.Modes and containsMode says no; the FT-991A's legend prints
// the same four names and blocks the same three rows for the same reason
// (core/cat/ft991a/dialect.go's modeNames), which is why this is a count
// of models rather than of makers. Each such row therefore BLOCKS with
// a Blocking ActionUnsupported entry naming the Mode column — exactly what
// every Icom model already does with the same three rows, and for the same
// reason: a mapped mode the radio does not list must be refused, never
// written as a mode the radio has never been shown to have.
//
// THE RESOLUTION IS DEFERRED, NOT DECIDED AGAINST THIS RADIO (plan decision
// P9, spec erratum S-E3). Teaching chirpModeMap to consult caps for a
// sideband-agnostic alternative would change ELEVEN Icom models' CHIRP
// outcome as well as this one, and every one of those models' byte-identity
// baselines with it, so it is a fleet question and a recorded roadmap
// follow-up. What this test does is make the FT-891's current answer
// EXPLICIT, so the day that question is settled the change shows up here as
// a deliberate edit rather than as a baseline that silently moved.
//
// THE FIVE ONE-NAME ROWS ARE THE OTHER HALF, and they are what stops this
// test passing because the import refuses everything: FM, NFM, AM, USB and
// LSB each map to a name this radio's legend does print, and every one of
// them imports cleanly.
func TestImportCHIRP_FT891BlocksCWAndRTTYRows(t *testing.T) {
	caps := ft891LikeCapabilities()

	// Precondition, stated rather than assumed: the three mapped names are
	// genuinely absent from this radio's mode list. If a later edit added
	// them, every blocking assertion below would become false and this test
	// would be pinning nothing.
	for _, absent := range []string{"CW-U", "CW-L", "RTTY-U"} {
		if containsMode(caps, absent) {
			t.Fatalf("fixture precondition: %q IS in the FT-891's Modes — this test is about the three names its legend does NOT print", absent)
		}
	}

	t.Run("CW, CWR and RTTY block", func(t *testing.T) {
		const csv = "Location,Name,Frequency,Mode\n" +
			"1,MORSE,7.030000,CW\n" +
			"2,MORSER,7.031000,CWR\n" +
			"3,TELETYPE,14.080000,RTTY\n"

		channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
		if err != nil {
			t.Fatalf("ImportCHIRP: unexpected error: %v", err)
		}
		if !report.HasBlocking() {
			t.Fatalf("HasBlocking() = false, want true: %+v", report.Entries)
		}
		for i, want := range []struct {
			line   int
			raw    string
			mapped string
		}{
			// LossEntry.Line counts the FILE's lines, so the header is 1
			// and the three data rows are 2, 3 and 4.
			{2, "CW", "CW-U"},
			{3, "CWR", "CW-L"},
			{4, "RTTY", "RTTY-U"},
		} {
			var modeEntries []LossEntry
			for _, e := range entriesForLine(report, want.line) {
				if e.Column == "Mode" {
					modeEntries = append(modeEntries, e)
				}
			}
			if len(modeEntries) != 1 {
				t.Errorf("row %d: %d Mode entries, want exactly 1: %+v", i+1, len(modeEntries), modeEntries)
				continue
			}
			e := modeEntries[0]
			if e.Action != ActionUnsupported || !e.Blocking {
				t.Errorf("row %d: Mode entry = %+v, want a Blocking ActionUnsupported one", i+1, e)
			}
			if e.Value != want.raw {
				t.Errorf("row %d: entry Value = %q, want the CHIRP cell %q", i+1, e.Value, want.raw)
			}
			// The detail must name BOTH names, so a user can see that the
			// refusal is about a NAME this radio's legend does not print
			// rather than about a mode it lacks.
			if !strings.Contains(e.Detail, want.raw) || !strings.Contains(e.Detail, want.mapped) {
				t.Errorf("row %d: Detail = %q, want it to name both the CHIRP mode %q and the mapped name %q", i+1, e.Detail, want.raw, want.mapped)
			}
		}
		for _, ch := range channels {
			if ch.Data != nil && ch.Data.Mode != "" {
				t.Errorf("channel %q imported Mode %q — a blocked row must not carry a mode at all", ch.Slot, ch.Data.Mode)
			}
		}
	})

	t.Run("the five one-name rows import", func(t *testing.T) {
		const csv = "Location,Name,Frequency,Mode\n" +
			"1,SIMPLEX,145.500000,FM\n" +
			"2,NARROW,145.525000,NFM\n" +
			"3,AIRBAND,118.000000,AM\n" +
			"4,UPPER,14.250000,USB\n" +
			"5,LOWER,7.100000,LSB\n"

		channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
		if err != nil {
			t.Fatalf("ImportCHIRP: unexpected error: %v", err)
		}
		if report.HasBlocking() {
			t.Fatalf("HasBlocking() = true, want false — these five CHIRP names all map to modes this radio's legend prints: %+v", report.Entries)
		}
		want := []string{"FM", "FM-N", "AM", "USB", "LSB"}
		if len(channels) != len(want) {
			t.Fatalf("len(channels) = %d, want %d", len(channels), len(want))
		}
		for i, w := range want {
			if channels[i].Data == nil {
				t.Errorf("channels[%d].Data is nil, want an imported channel", i)
				continue
			}
			if got := channels[i].Data.Mode; got != w {
				t.Errorf("channels[%d].Data.Mode = %q, want %q", i, got, w)
			}
		}
	})
}

// ft991aLikeCapabilities mirrors the FT-991A fields ImportCHIRP consults
// (core/driver/ft991a/caps.go). Hand-built rather than the real driver's
// Capabilities for the layering reason ft710LikeCapabilities gives:
// core/csvio sits BELOW core/driver in the import graph and must not
// depend on it, even in tests.
//
// TWO FIELDS ARE WHY THIS FIXTURE EXISTS rather than a reuse of the
// FT-891's, and both are load-bearing below.
//
// CTCSSStates HAS FIVE MEMBERS, and it is the first registered radio's
// that does. The record's P8 legend prints "0: CTCSS \"OFF\" 1: CTCSS
// ENC/DEC 2: CTCSS ENC 3: DCS ENC/DEC 4: DCS ENC" (capability matrix
// §1.17), so this radio's memory record can SAY a channel uses DCS where
// every sibling's can only say CTCSS. The DCS half is what
// TestImportCHIRP_DTCSRefusalReasonFollowsTheRecord's DCS branch is
// about; the first three members are byte-identical to
// spec.StandardCTCSSStates() because the radio's own legend is.
//
// FieldTagDisplay IS THE ZERO FieldSupport, the FTdx10's shape and the
// inversion of the FT-891's: MT position 28 prints "0: (Fixed)" on this
// radio (matrix §2.3), so there is no display flag for ImportCHIRP to
// derive a tag_display from.
//
// Modes are the fourteen the dialect transcribes ('1'..'9' then 'A'..'E',
// no hole and no 'F' — core/cat/ft991a/dialect.go's modeNames), and they
// include this radio's C4FM, which CHIRP has no name for at all. Like the
// FT-891's, this legend prints "CW", "CW-R", "RTTY-LSB" and "RTTY-USB"
// rather than the sideband-specific family names, so CHIRP's CW, CWR and
// RTTY rows block here for the same reason they block there.
//
// PMS IS ABSENT FROM THIS FIXTURE for the reason the FT-891's is:
// ImportCHIRP writes into the MEMORY bank alone. The real driver's PMS
// bank is the wire numbers "100".."117" (plan P20), and nothing
// ImportCHIRP consults can see it.
func ft991aLikeCapabilities() spec.Capabilities {
	caps := ft710LikeCapabilities()
	caps.Model = "FT-991A"
	caps.CATID = "0670"
	rw := spec.FieldSupport{Read: spec.Supported, Write: spec.Supported}
	banks := make([]spec.Bank, len(caps.Banks))
	copy(banks, caps.Banks)
	for i := range banks {
		banks[i].Fields = map[spec.Field]spec.FieldSupport{
			spec.FieldFrequency:  rw,
			spec.FieldMode:       rw,
			spec.FieldClarifier:  rw,
			spec.FieldCTCSSState: rw,
			spec.FieldShift:      rw,
			spec.FieldTag:        rw,
			// P11 is SCHEMA here, not a live flag: the FT-891's cell
			// inverted (matrix §2.3).
			spec.FieldTagDisplay: {},
			// No tone-NUMBER byte and no skip flag in the 41-position
			// record; both are ASSUMED-register entries (matrix §2.4).
			spec.FieldCTCSSTone: {},
			spec.FieldScanSkip:  {},
		}
	}
	caps.Banks = banks
	caps.Modes = []string{
		"LSB", "USB", "CW", "FM", "AM", "RTTY-LSB",
		"CW-R", "DATA-LSB", "RTTY-USB", "DATA-FM", "FM-N", "DATA-USB",
		"AM-N", "C4FM",
	}
	caps.CTCSSStates = []spec.ToneState{
		{Value: "OFF", Semantics: spec.ToneOff},
		{Value: "ENC-DEC", Semantics: spec.ToneEncodeDecode},
		{Value: "ENC", Semantics: spec.ToneEncode},
		{Value: "DCS-ENC-DEC", Semantics: spec.ToneDCSEncodeDecode},
		{Value: "DCS-ENC", Semantics: spec.ToneDCSEncode},
	}
	return caps
}

// declaresADCSState reports whether this radio's memory record can name a
// DCS state at all — either of the two spec.ToneSemantics members S0.4
// added. It is the question the DTCS/Cross refusal's REASON turns on, and
// it is asked of the capabilities rather than of the model name so that a
// second such radio needs no edit here.
func declaresADCSState(caps spec.Capabilities) bool {
	_, encDec := toneStateFor(caps, spec.ToneDCSEncodeDecode)
	_, enc := toneStateFor(caps, spec.ToneDCSEncode)
	return encDec || enc
}

// TestImportCHIRP_DTCSRefusalReasonFollowsTheRecord drives the DTCS/Cross
// refusal over EVERY fixture in unreachableScanSkipCapabilities and pins that
// the refusal's REASON is derived from the radio's own record while the
// BLOCKING is not.
//
// The blocking half never varies and never should: CHIRP's DTCS and Cross
// rows carry a DCS CODE, no registered radio's memory record has a field
// for one (the FT-991A's included — matrix §1.21 and §2.4: the chart
// exists, the state exists, the per-channel code field does not), so the
// row is refused rather than imported as something else.
//
// THE REASON WAS FALSE ON EXACTLY ONE RADIO. "%s CAT has no DCS memory
// write" is true of every model registered before the FT-991A, whose
// records carry a three-state CTCSS byte and nothing DCS-shaped at all.
// The FT-991A's record carries a five-state byte whose last two values
// ARE DCS states, and this driver writes them (matrix §2.4, and the
// dialect register's "THE DCS STATES' SET ACCEPTANCE"), so on that radio
// the sentence denied something the programme does. What cannot be
// carried is the CODE, and that is now what the entry says.
//
// THE SPLIT IS ON CAPABILITIES, NOT ON A MODEL NAME, which is what keeps
// the sixteen models registered before this one byte-identical: a fixture
// that declares neither DCS member takes the original sentence
// unchanged, and this test asserts that text in full rather than by
// substring. Fleet-wide byte identity is task 16's per-model CHIRP
// baseline legs; this is the per-branch pin underneath them.
//
// THE TWO PRECONDITION FATALS ARE THE COVERAGE ASSERTION the plan's M7
// asks for: dropping the FT-991A row from unreachableScanSkipCapabilities
// leaves no fixture declaring a DCS state, and this test goes red
// immediately rather than quietly pinning one branch. That is the only
// completeness pressure on that hand-written table from inside this
// package.
func TestImportCHIRP_DTCSRefusalReasonFollowsTheRecord(t *testing.T) {
	const csv = "Location,Name,Frequency,Tone,rToneFreq,cToneFreq,Mode\n" +
		"1,DCSCH,145.500000,DTCS,88.5,88.5,FM\n" +
		"2,CROSSCH,145.525000,Cross,88.5,88.5,FM\n"

	var withDCS, withoutDCS int
	for _, caps := range unreachableScanSkipCapabilities() {
		if declaresADCSState(caps) {
			withDCS++
		} else {
			withoutDCS++
		}
		t.Run(caps.Model, func(t *testing.T) {
			channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
			if err != nil {
				t.Fatalf("ImportCHIRP() error = %v", err)
			}
			if len(channels) != 2 {
				t.Fatalf("imported %d channels, want 2", len(channels))
			}
			for i, raw := range []string{"DTCS", "Cross"} {
				want := LossEntry{
					Line: i + 2, Column: "Tone", Value: raw,
					Action: ActionUnsupported, Blocking: true,
					Detail: fmt.Sprintf("%s CAT has no DCS memory write; %s tone squelch cannot be imported", caps.Model, raw),
				}
				if declaresADCSState(caps) {
					want.Detail = fmt.Sprintf("%s CAT writes the DCS state but not the DCS code; %s tone squelch cannot be imported", caps.Model, raw)
				}
				var got []LossEntry
				for _, e := range entriesForLine(report, want.Line) {
					if e.Column == "Tone" {
						got = append(got, e)
					}
				}
				if len(got) != 1 || got[0] != want {
					t.Errorf("%s Tone entries = %+v, want exactly [%+v]", raw, got, want)
				}
				// The channel is refused, not half-imported: neither the
				// state nor a tone survives a blocked row.
				if ch := channels[i]; ch.Data != nil {
					if ch.Data.CTCSS != "" {
						t.Errorf("%s row imported CTCSS %q — a refused row must carry no state", raw, ch.Data.CTCSS)
					}
					if ch.Data.CTCSSTone.State != codeplug.Unknown {
						t.Errorf("%s row imported CTCSSTone %+v, want {Unknown}", raw, ch.Data.CTCSSTone)
					}
				}
			}
		})
	}
	if withDCS == 0 {
		t.Fatal("no fixture in unreachableScanSkipCapabilities declares a DCS state — the FT-991A's row is what makes the DCS branch reachable from here; without it this test pins only half of what it claims")
	}
	if withoutDCS == 0 {
		t.Fatal("every fixture declares a DCS state — the original sentence's branch, and with it the byte identity of every model registered before the FT-991A, is no longer covered")
	}
}

// ts590LikeCapabilities mirrors the TS-590 pair's fields ImportCHIRP consults
// (core/driver/ts590/caps.go). Hand-built rather than the real driver's
// Capabilities for the layering reason ft710LikeCapabilities gives:
// core/csvio sits BELOW core/driver in the import graph and must not depend
// on it, even in tests.
//
// THREE THINGS ARE MIRRORED FAITHFULLY AND THE REST IS INHERITED UNEXAMINED,
// exactly as ftdx101LikeCapabilities' doc comment says of its own fixture.
// The three are:
//
//  1. Modes — this radio's own nine names, in its wire-code order, derived by
//     that driver from the layout's MD legend (590:1353-1363) rather than
//     transcribed twice. None of "CW-U", "CW-L" or "RTTY-U" is among them,
//     which is what TestImportCHIRP_TS590PairBlocksCWAndRTTYRows turns on.
//  2. The MEM bank's Fields, and in particular a REACHABLE scan_skip: byte 41
//     of the 50-byte record is a channel-lockout flag (590:1572-1574), so
//     this is the first registered family for which a CHIRP Skip cell has
//     somewhere to go.
//  3. ShiftOptions — nil, as core/driver/ts590/caps.go's own ShiftOptions is
//     (§1.16: the 50-byte record carries no duplex selector), and NOT the
//     FT-710's standard three. This one decides the whole family's CHIRP
//     import outcome, so inheriting it was a fixture that contradicted the
//     driver: a blank Duplex cell is CHIRP's ordinary simplex row, it asks
//     importCHIRPDuplexShift for a ShiftNone option, and with none published
//     that arm refuses BLOCKING. Every CHIRP row therefore blocks on these
//     two radios — the outcome
//     TestImportCHIRP_TS590PairBlocksCWAndRTTYRows' second subtest pins.
//  4. spec.FieldTxFrequency on the MEM bank — graded (bankFields' txFreq,
//     core/driver/ts590/caps.go:399) because it decides what a row this
//     branch says nothing about must leave TxFreqHz: this radio has the
//     field, so Unknown, never the zero value the map would otherwise leave
//     it at (see importCHIRPDuplexShift's own comment).
//  5. spec.Capabilities.SimplexTx — SimplexTxEqualsRx, as
//     core/driver/ts590/caps.go's is: MW/MR carries no transmit-frequency
//     field at all and P1 selects simplex (590:1521-1523), so a blank CHIRP
//     Duplex row's transmit disposition is the row's own receive frequency.
//     Read by importCHIRPDuplexShift's blank arm.
//  6. THE TONE VOCABULARY — the three fields FieldToneMode/FieldToneTx/
//     FieldToneRx, ToneModes, and the 43-entry Kenwood chart. Added
//     09/09/2026 for v1.5.x follow-up (l): grading none of the three sent
//     every pair-1 test down importCHIRPToneCTCSS whilst the registered rows
//     take importCHIRPToneIcom, so the fixture's evidence was about a branch
//     these radios never reach. CTCSSStates goes nil with it, as the real
//     driver's is — a row publishing the Icom half of the vocabulary pair
//     publishes no Yaesu half, and codeplug.Validate measures a channel's
//     ctcss_state against that list.
//
// Everything else — the tag charset — is ft710LikeCapabilities' and is NOT a
// claim about a Kenwood radio. No test here asks this fixture a question
// about it, and a test that needed one would have to widen the fixture first
// rather than trust it. TagLen IS corrected to 8 (590:1576) because the
// name-length path reads it.
//
// PMS/SCAN IS ABSENT FROM THIS FIXTURE and that is deliberate, not an
// omission: ImportCHIRP writes into the MEMORY bank alone (memBank), so a
// second bank would change nothing any assertion below can see. The real
// driver has one, and its per-bank field differences are asserted where they
// are visible — app/uispec_test.go's
// TestBankTierFields_RegisteredTS590Pair_PerBank.
//
// model/catID pick the sibling: "TS-590S"/"021" or "TS-590SG"/"023"
// (core/driver/ts590/caps.go's modelNameS/modelNameSG and catIDS/catIDSG).
// The two differ in NOTHING this package can see — the one memory-channel
// difference between the rows is the FILTER column, which ImportCHIRP does
// not consult — so both fixtures are built from one constructor rather than
// two, and the tests still name them separately because the registry does.
func ts590LikeCapabilities(model, catID string) spec.Capabilities {
	caps := ft710LikeCapabilities()
	caps.Model = model
	caps.CATID = catID
	caps.TagLen = 8
	caps.Modes = []string{"LSB", "USB", "CW", "FM", "FM-N", "AM", "FSK", "CW-R", "FSK-R"}
	caps.ShiftOptions = nil
	// Mirrored item 5: core/driver/ts590/caps.go's SimplexTxEqualsRx. MW/MR
	// has no transmit-frequency field at all and selects simplex with P1
	// (590:1521-1523), so a blank CHIRP Duplex row's transmit disposition is
	// the row's own receive frequency.
	caps.SimplexTx = spec.SimplexTxEqualsRx
	caps.Transmit = spec.HasTransmitter
	// Mirrored item 6. nil, as core/driver/ts590/caps.go's own is: this row
	// publishes the Icom half of the vocabulary pair and no Yaesu half.
	caps.CTCSSStates = nil
	caps.ToneModes = []spec.ToneMode{
		{Value: "OFF", Semantics: spec.ToneModeOff},
		{Value: "TONE", Semantics: spec.ToneModeCTCSS},
		{Value: "CTCSS", Semantics: spec.ToneModeCTCSSRxSquelch},
		{Value: "CROSS", Semantics: spec.ToneModeCross},
	}
	// THE 43-ENTRY CHART, not pair 2's 51: core/driver/ts590/caps.go's
	// kenwoodCTCSSTones literal, the project's shared forty-two plus
	// 1750.0 Hz. Transcribed here rather than shared because core/csvio sits
	// BELOW core/driver in the import graph.
	caps.CTCSSTones = []spec.Tone{
		670, 693, 719, 744, 770, 797, 825, 854, 885, 915, 948, 974,
		1000, 1035, 1072, 1109, 1148, 1188, 1230, 1273, 1318, 1365,
		1413, 1462, 1514, 1567, 1622, 1679, 1738, 1799, 1862, 1928,
		2035, 2065, 2107, 2181, 2257, 2291, 2336, 2418, 2503, 2541,
		17500,
	}
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	banks := make([]spec.Bank, len(caps.Banks))
	copy(banks, caps.Banks)
	for i := range banks {
		banks[i].Fields = map[spec.Field]spec.FieldSupport{
			spec.FieldFrequency: rw,
			spec.FieldMode:      rw,
			spec.FieldTag:       rw,
			// THE ONE THAT MATTERS HERE. Every registered radio before this
			// family grades scan_skip the zero FieldSupport; this record
			// carries a channel-lockout flag, so a CHIRP Skip cell imports
			// literally rather than being dropped with a loss entry.
			spec.FieldScanSkip: rw,
			// Graded on the MEM bank only, as the real driver's bankFields
			// does — see the doc comment's mirrored item 4.
			spec.FieldTxFrequency: rw,
			// Mirrored item 6: the Icom half of the vocabulary pair, all
			// three graded, which is what sends a CHIRP row down
			// importCHIRPToneIcom as the registered rows do.
			spec.FieldToneMode: rw,
			spec.FieldToneTx:   rw,
			spec.FieldToneRx:   rw,
			// No per-channel clarifier field, no shift selector and no
			// ctcss_state/ctcss_tone pair anywhere in the 47 accounted
			// parameter bytes: this record expresses tone as a mode selector
			// with two independent indices, which is the
			// tone_mode/tone_tx/tone_rx vocabulary, and repeater operation
			// as an independent transmit frequency rather than a shift.
			spec.FieldClarifier:  {},
			spec.FieldShift:      {},
			spec.FieldCTCSSState: {},
			spec.FieldCTCSSTone:  {},
			// No tag-display flag anywhere in either row's record.
			spec.FieldTagDisplay: {},
		}
	}
	caps.Banks = banks
	return caps
}

// ts890Modes and ts990Modes are Tier 6's second pair's own mode legends, each
// derived by its own driver from its own book's OM P2 chart
// (core/driver/ts890/caps.go and core/driver/ts990/caps.go, both via
// core/kw/ma's per-row ModeNames legend) rather than transcribed twice.
//
// SIXTEEN AND TWENTY-SIX, and the difference is entirely in how each radio
// spells its DATA modes: the 890S has one data set (LSB-D, USB-D, FM-D,
// FM-D-N, AM-D) and the 990S has three (D1, D2, D3), which is matrix erratum
// M-E3's consequence for CSV and CHIRP alike — a channel's data disposition
// travels in the mode column on these rows, because neither record has a
// separate byte for it.
//
// NEITHER LIST CONTAINS "CW-U", "CW-L" OR "RTTY-U", which is what
// TestImportCHIRP_TS890And990BlockCWAndRTTYRows turns on: Kenwood spells RTTY
// "FSK" and prints CW and CW-R without a sideband suffix.
var (
	ts890Modes = []string{
		"LSB", "USB", "CW", "FM", "FM-N", "AM", "FSK", "CW-R", "FSK-R",
		"PSK", "PSK-R", "LSB-D", "USB-D", "FM-D", "FM-D-N", "AM-D",
	}
	ts990Modes = []string{
		"LSB", "USB", "CW", "FM", "FM-N", "AM", "FSK", "CW-R", "FSK-R",
		"PSK", "PSK-R",
		"LSB-D1", "USB-D1", "FM-D1", "FM-D1-N", "AM-D1",
		"LSB-D2", "USB-D2", "FM-D2", "FM-D2-N", "AM-D2",
		"LSB-D3", "USB-D3", "FM-D3", "FM-D3-N", "AM-D3",
	}
)

// maLikeCapabilities mirrors Tier 6's SECOND pair's fields ImportCHIRP
// consults, on ts590LikeCapabilities' terms exactly: hand-built rather than
// the real drivers' Capabilities, because core/csvio sits BELOW core/driver
// in the import graph and must not depend on it even in tests.
//
// FIVE THINGS ARE MIRRORED FAITHFULLY AND THE REST IS INHERITED UNEXAMINED:
//
//  1. Modes — the caller's, one row's own legend per call (see ts890Modes and
//     ts990Modes above). This is the ONE parameter on which the two rows
//     genuinely differ here, which is why this constructor takes it rather
//     than picking it from the model name.
//  2. The MEM bank's Fields, and in particular a REACHABLE scan_skip: both
//     records carry a channel-lockout flag at a printed position
//     (890:3205-3207, 990:2952-2954), so a CHIRP Skip cell has somewhere to
//     go on these rows as it does on the 590 pair's.
//  3. ShiftOptions — nil, as both drivers' own are (§1.16 on each row:
//     neither record carries a duplex selector of any kind), and NOT the
//     FT-710's standard three. This decides the whole family's CHIRP import
//     outcome, and since the 07/09/2026 fleet ruling it decides it in the
//     PERMISSIVE direction: a blank Duplex cell asks for a ShiftNone option,
//     none is published, and that arm now reports nothing at all rather than
//     blocking.
//  4. spec.FieldTxFrequency on the MEM bank — GRADED (each driver's own
//     bankFields), and on these rows it is graded unconditionally rather than
//     per bank, because each row publishes one bank. It decides what a row
//     this branch says nothing about leaves TxFreqHz at: Unknown, never the
//     zero value the map would otherwise give.
//  5. THE TONE STORY, ALL OF IT — the three graded fields
//     spec.FieldToneMode/FieldToneTx/FieldToneRx (890:3180-3190 via
//     core/driver/ts890/caps.go:267-269; 990:2915-2926 via
//     core/driver/ts990/caps.go:246-248), the four-value ToneModes legend
//     both drivers publish (ts890/caps.go:589-594, ts990/caps.go:536-541) and
//     the fifty-one-entry CTCSS chart they share (ts890/caps.go:123-130,
//     ts990/caps.go:123-128). This was ITEM 5 ONLY AFTER THE MILESTONE-CLOSE
//     REVIEW: without it the fixture graded none of the three, took
//     importCHIRPToneCTCSS, and proved a branch NEITHER registered row can
//     reach (finding C-MED-1). It is not a nicety — it decides the whole
//     family's tone outcome, exactly as item 3 decides its duplex one, and
//     TestImportCHIRP_TS890And990TakeTheToneModeBranch is what holds it here.
//     The nil CTCSSStates travels with it — a row publishing the Icom half of
//     the vocabulary pair publishes no Yaesu half — and so does spec.Transmit,
//     because the TSQL arm asks the radio's anatomy which squelch semantic to
//     want.
//
// Everything else — the tag charset above all — is ft710LikeCapabilities' and
// is NOT a claim about a Kenwood radio. TagLen IS corrected to 10
// (890:3208-3209, 990:2955-2956) because the name-length path reads it.
//
// ONE CONSTRUCTOR FOR TWO ROWS, as the 590 pair has, and for a related but
// weaker reason: these two radios are driven by two different packages and
// their records are different shapes, but NOTHING ImportCHIRP consults
// differs between them except the mode legend — so the legend is a parameter
// and everything else is shared. A future divergence in anything else must
// widen this constructor rather than be absorbed by it.
func maLikeCapabilities(model, catID string, modes []string) spec.Capabilities {
	caps := ft710LikeCapabilities()
	caps.Model = model
	caps.CATID = catID
	caps.TagLen = 10
	caps.Modes = modes
	caps.ShiftOptions = nil
	caps.Transmit = spec.HasTransmitter
	// nil, as both drivers' own are (ts890/caps.go:552, ts990/caps.go:499),
	// and NOT the FT-710's standard three: a row that publishes the Icom half
	// of the vocabulary pair publishes no Yaesu half at all. It is load-bearing
	// rather than tidy — codeplug.Validate measures a channel's ctcss_state
	// against this list, and importCHIRPToneIcom never writes that field, so a
	// fixture that kept the inherited three would report an error on every
	// channel these two radios can import.
	caps.CTCSSStates = nil
	// Mirrored from both drivers' caps.go: SimplexTxZero. Each book prints
	// that a simplex channel's split parameters all read 0
	// (890:3217-3218, 990:2964-2965).
	caps.SimplexTx = spec.SimplexTxZero
	// The pair's own chart: the project's shared fifty plus 1750.0 Hz, which
	// is the one entry that separates them and the reason this is mirrored
	// rather than inherited.
	shared := spec.StandardCTCSSTones()
	caps.CTCSSTones = append(shared[:], 17500)
	caps.ToneModes = []spec.ToneMode{
		{Value: "OFF", Semantics: spec.ToneModeOff},
		{Value: "TONE", Semantics: spec.ToneModeCTCSS},
		{Value: "CTCSS", Semantics: spec.ToneModeCTCSSRxSquelch},
		{Value: "CROSS", Semantics: spec.ToneModeCross},
	}
	rw := spec.FieldSupport{Read: spec.Unverified, Write: spec.Unverified}
	banks := make([]spec.Bank, len(caps.Banks))
	copy(banks, caps.Banks)
	for i := range banks {
		banks[i].Fields = map[spec.Field]spec.FieldSupport{
			spec.FieldFrequency:   rw,
			spec.FieldMode:        rw,
			spec.FieldTag:         rw,
			spec.FieldScanSkip:    rw,
			spec.FieldTxFrequency: rw,
			// The Icom half of the vocabulary pair, all three graded, which
			// is what sends a CHIRP row down importCHIRPToneIcom.
			spec.FieldToneMode: rw,
			spec.FieldToneTx:   rw,
			spec.FieldToneRx:   rw,
			// No per-channel clarifier field, no shift selector and no
			// ctcss_state/ctcss_tone pair in either record: both express tone
			// as a mode selector with two independent indices, which is the
			// tone_mode/tone_tx/tone_rx vocabulary, and repeater operation as
			// an independent transmit frequency rather than a shift.
			spec.FieldClarifier:  {},
			spec.FieldShift:      {},
			spec.FieldCTCSSState: {},
			spec.FieldCTCSSTone:  {},
			// No tag-display flag anywhere in either record.
			spec.FieldTagDisplay: {},
		}
	}
	caps.Banks = banks
	return caps
}

// chirpFixtures is every capability fixture in this file that stands for a
// REGISTERED model — the unreachable-scan-skip set plus the TS-590 pair,
// which takes the other branch. It is the set
// TestChirpFixtures_CoverEveryRegisteredModel measures against the registry.
func chirpFixtures() []spec.Capabilities {
	out := unreachableScanSkipCapabilities()
	out = append(out,
		ts590LikeCapabilities("TS-590S", "021"),
		ts590LikeCapabilities("TS-590SG", "023"),
		maLikeCapabilities("TS-890S", "024", ts890Modes),
		maLikeCapabilities("TS-990S", "022", ts990Modes),
	)
	// The v1.7.0 Icom wave: registered capabilities, not fixtures of the
	// bucket above — see icXXXXLikeCapabilities's own doc comment for why.
	return append(out,
		ic7800LikeCapabilities(),
		ic7600LikeCapabilities(),
		ic7410LikeCapabilities(),
		ic7700LikeCapabilities(),
		ic9100LikeCapabilities(),
		ic7200LikeCapabilities(),
		// v1.7.0 Kenwood/Yaesu wave.
		ftdx5000LikeCapabilities(),
		ts2000LikeCapabilities(),
		ts2000xLikeCapabilities(),
		tsb2000LikeCapabilities(),
		ts570dLikeCapabilities(),
		ts570sLikeCapabilities(),
		ts870sLikeCapabilities(),
	)
}

// chirpFixtureExceptions names every registered model that has NO CHIRP
// capability fixture in this file, and it is a DEBT LEDGER rather than a
// policy: eleven Icom models were registered between M9d-2 and Tier 4b
// without one, and this milestone neither created that gap nor is the right
// place to close it.
//
// THE PLAN ASKED FOR AN EMPTY EXCEPTION SET (Codex re-review MED-1b) AND
// THAT IS NOT REACHABLE FROM THIS TASK. Closing the gap means inventing
// eleven Icom fixtures — a Modes list and a per-bank field map per radio,
// each a claim about a radio this milestone has read nothing about — and
// enrolling all eleven in TestImportCHIRP_ScanSkipIsCapabilityAware, whose
// per-row loss-entry assertions would then be running against fixture content
// nobody had checked against those drivers. Writing eleven radios' worth of
// unevidenced capability data to satisfy a completeness check would be the
// exact failure this project's evidence rules exist to prevent, so the check
// lands with the gap NAMED instead of hidden. The exception list is pinned to
// exactly these eleven and may only SHRINK.
//
// WHAT THE CHECK STILL BUYS, WHICH IS THE WHOLE POINT OF LANDING IT (plan
// decision P3, row 9 of the ten-edit list): the TS-480 is not on this list,
// so the day its row registers without a fixture here, this check FAILS —
// which is what makes edit 9 loud where it was silent. So does any future
// registration.
var chirpFixtureExceptions = []string{
	"IC-7610", "IC-7300", "IC-7300MK2", "IC-705", "IC-9700", "IC-905",
	"IC-7851", "IC-7850", "IC-7760", "IC-7100", "IC-R8600",
}

// TestChirpFixtures_CoverEveryRegisteredModel is the GENERAL CHIRP
// completeness check (Codex HIGH 3, tightened at Codex re-review MED-1b),
// and it lands HERE, in package csvio, next to the expectations themselves —
// it cannot be reached from internal/wiring, which is why revision 2's
// central leg was impossible: these fixtures are test-only symbols in this
// package's test binary and no import from internal/wiring reaches them.
//
// THE internal/wiring IMPORT IS TEST-ONLY, confined to this _test.go file
// exactly as the rest of chirp_test.go is, so production core/csvio
// (chirp.go, export.go, import.go, tone.go) gains no new dependency. The
// package's own layering rule — "core/csvio sits below core/driver and must
// not import it, even in tests" — is untouched, because this import is of
// internal/wiring, not core/driver.
//
// IT REPLACES A STALE CLAIM. unreachableScanSkipCapabilities' doc comment
// used to say that "the registry-walk pin … does not know about this file".
// It now does, deliberately, through this one test-only import, and that
// comment has been rewritten to say so.
//
// RED-PROVED (recorded, not re-run by CI): removing ft891LikeCapabilities()
// from unreachableScanSkipCapabilities fails here with `"FT-891" is
// registered but has no CHIRP capability fixture`. Adding "FT-891" to
// chirpFixtureExceptions to silence it fails the exception-list pin below
// instead.
func TestChirpFixtures_CoverEveryRegisteredModel(t *testing.T) {
	registered := wiring.SupportedModels()
	if len(registered) == 0 {
		t.Fatal("wiring.SupportedModels() is empty — this check would pass vacuously")
	}

	have := map[string]bool{}
	for _, caps := range chirpFixtures() {
		if caps.Model == "" {
			t.Fatal("a fixture in chirpFixtures has an empty Model — it stands for no registered radio and this check cannot see it")
		}
		if have[caps.Model] {
			t.Errorf("chirpFixtures holds two fixtures for %q", caps.Model)
		}
		have[caps.Model] = true
	}

	excepted := map[string]bool{}
	for _, m := range chirpFixtureExceptions {
		excepted[m] = true
	}

	for _, model := range registered {
		switch {
		case have[model] && excepted[model]:
			t.Errorf("%q has a CHIRP capability fixture AND is named in chirpFixtureExceptions — the exception list may only shrink, so delete its entry", model)
		case !have[model] && !excepted[model]:
			t.Errorf("%q is registered in internal/wiring but has no CHIRP capability fixture here — this file's per-row CHIRP expectations silently skip it. Add its fixture, mirroring that driver's own caps; a model whose scan-skip is reachable goes into chirpFixtures directly rather than into unreachableScanSkipCapabilities", model)
		}
	}
	for model := range have {
		if !slices.Contains(registered, model) {
			t.Errorf("chirpFixtures holds a fixture for %q, which internal/wiring does not register", model)
		}
	}
	for _, model := range chirpFixtureExceptions {
		if !slices.Contains(registered, model) {
			t.Errorf("chirpFixtureExceptions names %q, which is not registered — the list is a debt ledger of REGISTERED models with no fixture, so a name that no longer registers must be deleted rather than carried", model)
		}
	}

	// THE LEDGER MAY ONLY SHRINK, and the freeze is a COUNT here rather than
	// a second copy of the eleven names (Opus review of Tier 6 task 18,
	// LOW-2: the old wantExceptions literal sat twenty lines from the list it
	// claimed to pin, so the "frozen" assertion compared the list against a
	// copy of itself and both halves were one edit apart).
	//
	// A COUNT IS ENOUGH BECAUSE MEMBERSHIP IS ALREADY PINNED AGAINST
	// internal/wiring, by the two loops above and not by any literal here: a
	// name swapped INTO this list is either a model with a fixture, which
	// fails the have && excepted branch, or one without, in which case the
	// name it displaced fails the !have && !excepted branch. So substitution
	// is covered by the registry walk and only GROWTH needs freezing.
	//
	// RED-PROVED (recorded, not re-run by CI): prepending "FT-891" — which
	// has a fixture — fails both this cap and the have && excepted branch.
	const inheritedExceptions = 11
	if len(chirpFixtureExceptions) > inheritedExceptions {
		t.Errorf("chirpFixtureExceptions has %d entries, want at most the %d it inherited (%v) — a newly registered model earns a FIXTURE, never an exception", len(chirpFixtureExceptions), inheritedExceptions, chirpFixtureExceptions)
	}
}

// TestImportCHIRP_TS590PairBlocksCWAndRTTYRows is Tier 6's per-row CHIRP pin
// (plan decision P16), and it records a LIMITATION rather than celebrating a
// behaviour — the FT-891's own pin, applied to a third spelling family.
//
// chirpModeMap resolves CHIRP's "CW" to "CW-U", its "CWR" to "CW-L" and its
// "RTTY" to "RTTY-U" — the sideband-specific names three registered Yaesu
// models print. These radios print "CW", "CW-R", "FSK" and "FSK-R", so none
// of those three mapped names is in either row's caps.Modes and containsMode
// says no. Each such row therefore BLOCKS with a Blocking ActionUnsupported
// entry naming the Mode column — exactly what the eleven Icom models and the
// FT-891 already do with the same three rows, and for the same reason: a
// mapped mode the radio does not list must be refused, never written as a
// mode the radio has never been shown to have.
//
// KENWOOD SPELLS RTTY "FSK", which is a THIRD spelling family in this
// registry rather than a second, and the resolution is DEFERRED rather than
// decided against these radios (plan decision P16; the FT-891's spec erratum
// S-E3). Teaching chirpModeMap to consult caps for a sideband-agnostic
// alternative would change eleven Icom models' and the FT-891's CHIRP outcome
// as well as these two, and every one of those models' byte-identity
// baselines with it, so it is a fleet question and a recorded roadmap
// follow-up. What this test does is make the answer EXPLICIT, so the day that
// question is settled the change shows up here as a deliberate edit rather
// than as a baseline that silently moved.
//
// THE FIVE ONE-NAME ROWS ARE THE OTHER HALF, and since the 07/09/2026 fleet
// ruling they IMPORT: FM, NFM, AM, USB and LSB each map to a name both rows'
// legends do print, and their blank Duplex cells are no longer a refusal.
//
// BOTH ROWS, not one: the two share a mode legend today, so the second
// subtest is a coincidence of one book rather than a derived fact, and a
// future divergence must fail here rather than be hidden by a single-row
// test standing in for a pair.
func TestImportCHIRP_TS590PairBlocksCWAndRTTYRows(t *testing.T) {
	for _, caps := range []spec.Capabilities{
		ts590LikeCapabilities("TS-590S", "021"),
		ts590LikeCapabilities("TS-590SG", "023"),
	} {
		t.Run(caps.Model, func(t *testing.T) {
			// Precondition, stated rather than assumed: the three mapped
			// names are genuinely absent from this row's mode list. If a
			// later edit added them, every blocking assertion below would
			// become false and this test would be pinning nothing.
			for _, absent := range []string{"CW-U", "CW-L", "RTTY-U"} {
				if containsMode(caps, absent) {
					t.Fatalf("fixture precondition: %q IS in the %s's Modes — this test is about the three names its legend does NOT print", absent, caps.Model)
				}
			}

			t.Run("CW, CWR and RTTY block", func(t *testing.T) {
				const csv = "Location,Name,Frequency,Mode\n" +
					"1,MORSE,7.030000,CW\n" +
					"2,MORSER,7.031000,CWR\n" +
					"3,TELETYPE,14.080000,RTTY\n"

				channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
				if err != nil {
					t.Fatalf("ImportCHIRP: unexpected error: %v", err)
				}
				if !report.HasBlocking() {
					t.Fatalf("HasBlocking() = false, want true: %+v", report.Entries)
				}
				for i, want := range []struct {
					line   int
					raw    string
					mapped string
				}{
					// LossEntry.Line counts the FILE's lines, so the header
					// is 1 and the three data rows are 2, 3 and 4.
					{2, "CW", "CW-U"},
					{3, "CWR", "CW-L"},
					{4, "RTTY", "RTTY-U"},
				} {
					var modeEntries []LossEntry
					for _, e := range entriesForLine(report, want.line) {
						if e.Column == "Mode" {
							modeEntries = append(modeEntries, e)
						}
					}
					if len(modeEntries) != 1 {
						t.Errorf("row %d: %d Mode entries, want exactly 1: %+v", i+1, len(modeEntries), modeEntries)
						continue
					}
					e := modeEntries[0]
					if e.Action != ActionUnsupported || !e.Blocking {
						t.Errorf("row %d: Mode entry = %+v, want a Blocking ActionUnsupported one", i+1, e)
					}
					if e.Value != want.raw {
						t.Errorf("row %d: entry Value = %q, want the CHIRP cell %q", i+1, e.Value, want.raw)
					}
					// The detail must name BOTH names, so a user can see
					// that the refusal is about a NAME this radio's legend
					// does not print rather than about a mode it lacks.
					if !strings.Contains(e.Detail, want.raw) || !strings.Contains(e.Detail, want.mapped) {
						t.Errorf("row %d: Detail = %q, want it to name both the CHIRP mode %q and the mapped name %q", i+1, e.Detail, want.raw, want.mapped)
					}
				}
				for _, ch := range channels {
					if ch.Data != nil && ch.Data.Mode != "" {
						t.Errorf("channel %q imported Mode %q — a blocked row must not carry a mode at all", ch.Slot, ch.Data.Mode)
					}
				}
			})

			// THE FIVE ONE-NAME ROWS NOW IMPORT, and that is the whole of
			// the 07/09/2026 fleet ruling: a blank CHIRP Duplex cell on a
			// radio that publishes NO shift vocabulary at all is not a
			// loss, it is the radio's only state. This family's 50-byte
			// record carries no duplex selector (core/driver/ts590/caps.go's
			// ShiftOptions is nil), so importCHIRPDuplexShift's ShiftNone
			// arm now reports NOTHING rather than refusing: data.Shift stays
			// "", which is exactly what core/driver/ts590/read.go produces
			// on read and what write.go treats as "not requested".
			//
			// "off" STAYS BLOCKING on the same fixture — the third subtest
			// below is that pin, and the two together are the T20 mutation
			// M15b guard in its new shape: re-adding an entry to the blank
			// arm fails the zero-entry assertion here, and flipping the
			// "off" arm's Blocking to false fails the next subtest.
			t.Run("the five one-name rows import as simplex with no Duplex entry", func(t *testing.T) {
				const csv = "Location,Name,Frequency,Mode\n" +
					"1,SIMPLEX,145.500000,FM\n" +
					"2,NARROW,145.525000,NFM\n" +
					"3,AIRBAND,118.000000,AM\n" +
					"4,UPPER,14.250000,USB\n" +
					"5,LOWER,7.100000,LSB\n"

				channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
				if err != nil {
					t.Fatalf("ImportCHIRP: unexpected error: %v", err)
				}
				if report.HasBlocking() {
					t.Fatalf("HasBlocking() = true, want false — a blank Duplex cell on a radio with no shift vocabulary is simplex, not a loss: %+v", report.Entries)
				}
				if len(channels) != 5 {
					t.Fatalf("imported %d channels, want 5", len(channels))
				}
				// Lines 2-6: the header is line 1.
				for line := 2; line <= 6; line++ {
					for _, e := range entriesForLine(report, line) {
						if e.Column == "Duplex" || e.Column == "Mode" {
							t.Errorf("line %d: entry %+v — these five names ARE in this radio's legend, and its blank Duplex cell is its only state, so neither column may report anything", line, e)
						}
					}
				}
				for i, ch := range channels {
					if ch.Data == nil {
						t.Fatalf("channels[%d].Data = nil", i)
					}
					if ch.Data.Shift != "" {
						t.Errorf("channels[%d].Shift = %q, want \"\" — this radio publishes no shift value to store, and \"\" is what its own read produces", i, ch.Data.Shift)
					}
					// TS-590S/SG reach spec.FieldTxFrequency, and a BLANK
					// Duplex cell is decision 4's ordinary simplex row, so
					// the file DID state the transmit disposition: since
					// the 09/09/2026 design these rows carry Known at the
					// row's own frequency, this pair's printed encoding of
					// simplex (590:1521-1523). Never ABSENT — Absent is
					// what codeplug.Validate reports as an error.
					if ch.Data.TxFreqHz != (codeplug.FreqField{State: codeplug.Known, Value: ch.Data.FreqHz}) {
						t.Errorf("channels[%d].TxFreqHz = %+v, want Known at this row's own %d Hz", i, ch.Data.TxFreqHz, ch.Data.FreqHz)
					}
				}
				if issues := validateImported(t, channels, caps); codeplug.HasErrors(issues) {
					t.Errorf("codeplug.Validate reported an error on a row that never mentioned split: %+v", issues)
				}
			})

			// "off" is the OTHER side of the same arm and still blocks:
			// CHIRP's "off" asserts "no duplex configured" as distinct from
			// simplex, and that distinction is one this record cannot
			// carry, so refusing it is honest where agreeing with a blank
			// cell is not.
			t.Run("an off Duplex row still blocks", func(t *testing.T) {
				const csv = "Location,Name,Frequency,Duplex,Mode\n" +
					"1,OFFDUP,145.500000,off,FM\n"

				_, report, err := ImportCHIRP(strings.NewReader(csv), caps)
				if err != nil {
					t.Fatalf("ImportCHIRP: unexpected error: %v", err)
				}
				if !report.HasBlocking() {
					t.Fatalf("HasBlocking() = false, want true — CHIRP's \"off\" says something this radio cannot say: %+v", report.Entries)
				}
				var duplex []LossEntry
				for _, e := range entriesForLine(report, 2) {
					if e.Column == "Duplex" {
						duplex = append(duplex, e)
					}
				}
				if len(duplex) != 1 {
					t.Fatalf("%d Duplex entries, want exactly 1: %+v", len(duplex), duplex)
				}
				if e := duplex[0]; e.Action != ActionUnsupported || !e.Blocking || e.Value != "off" {
					t.Errorf("Duplex entry = %+v, want a Blocking ActionUnsupported one carrying the \"off\" cell", e)
				}
			})
		})
	}
}

// TestImportCHIRP_TS590PairTakesTheLiteralScanSkipBranch is the other half of
// the TS-590 pair's CHIRP posture, and it is the first time a REGISTERED
// model reaches this branch at all.
//
// TestImportCHIRP_ScanSkipLiteralOnAWritableRadio pins the same rule against
// a synthetic fixture, and has had to since M9d-2 task 8, because until Tier
// 6 every registered radio's scan_skip was the zero FieldSupport. These two
// rows' 50-byte record carries a channel-lockout flag at a printed position
// (590:1572-1574), so a CHIRP Skip cell means exactly what it says here:
// blank is a real "do not skip" and "S" is a real "skip", both {Known}, and
// nothing is dropped or reported.
//
// It is ALSO what keeps the fixture honest. ts590LikeCapabilities claims a
// reachable scan_skip; if that claim were quietly reverted to the zero
// FieldSupport — the shape every other fixture in this file has — the two
// rows would silently start importing Unknown with a loss entry, and the
// mode test above would not notice.
//
// BOTH ROWS NOW IMPORT. Their blank Duplex cells used to block, and since the
// 07/09/2026 fleet ruling they do not: a radio publishing no shift vocabulary
// reads a blank CHIRP Duplex cell as its own simplex state rather than as a
// loss (see the mode test's second subtest). The assertion below pins that
// nothing at all blocks here, so the day either arm widens again this test
// says so rather than quietly following.
func TestImportCHIRP_TS590PairTakesTheLiteralScanSkipBranch(t *testing.T) {
	const csv = "Location,Name,Frequency,Duplex,Tone,rToneFreq,cToneFreq,Mode,Skip\n" +
		"1,BLANK,145.500000,,,,,FM,\n" +
		"2,SKIPPED,145.525000,,,,,FM,S\n"

	for _, caps := range []spec.Capabilities{
		ts590LikeCapabilities("TS-590S", "021"),
		ts590LikeCapabilities("TS-590SG", "023"),
	} {
		t.Run(caps.Model, func(t *testing.T) {
			if fs := caps.FieldSupport(spec.BankMemory, spec.FieldScanSkip); fs.Unreachable() {
				t.Fatalf("fixture precondition: %s scan_skip = %+v, want REACHABLE — this record carries a channel-lockout flag, and the whole point of this test is that these are the first registered radios of which that is true", caps.Model, fs)
			}

			channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
			if err != nil {
				t.Fatalf("ImportCHIRP: unexpected error: %v", err)
			}
			if len(channels) != 2 {
				t.Fatalf("imported %d channels, want 2", len(channels))
			}
			for i, want := range []bool{false, true} {
				if channels[i].Data == nil {
					t.Fatalf("channels[%d].Data = nil", i)
				}
				if got := channels[i].Data.ScanSkip; got != (codeplug.BoolField{State: codeplug.Known, Value: want}) {
					t.Errorf("channels[%d].ScanSkip = %+v, want {Known,%v} — this radio's record has somewhere to put it", i, got, want)
				}
			}
			if entries := skipEntries(report); len(entries) != 0 {
				t.Errorf("Skip entries = %+v, want none — nothing is lost when the radio can carry the flag", entries)
			}
			if report.HasBlocking() {
				t.Errorf("HasBlocking() = true, want false — the Skip column is carried and the blank Duplex cells are this radio's own simplex state, so both rows reach a radio: %+v", report.Entries)
			}
		})
	}
}

// TestImportCHIRP_TS890And990BlockCWAndRTTYRows is Tier 6's SECOND pair's
// per-row CHIRP pin (plan decision P15), and it records the same LIMITATION
// the TS-590 pair's does for the same reason, one book family further on.
//
// chirpModeMap resolves CHIRP's "CW" to "CW-U", its "CWR" to "CW-L" and its
// "RTTY" to "RTTY-U" — the sideband-specific names three registered Yaesu
// models print. Kenwood spells RTTY "FSK" and prints CW and CW-R with no
// sideband suffix, so none of those three mapped names is in either row's
// caps.Modes and containsMode says no. Each such row therefore BLOCKS with a
// Blocking ActionUnsupported entry naming the Mode column.
//
// THE DEFERRAL IS UNCHANGED AND IS NOT A DECISION AGAINST THESE RADIOS.
// Teaching chirpModeMap to consult caps for a sideband-agnostic alternative
// would change eleven Icom models', the FT-891's and the TS-590 pair's CHIRP
// outcome as well as these two, and every one of those models' byte-identity
// baselines with it. It stays a fleet question and a recorded roadmap
// follow-up; this test makes the answer EXPLICIT, so the day it is settled the
// change shows up here as a deliberate edit rather than as a baseline that
// silently moved.
//
// BOTH ROWS, and here that is NOT a coincidence of one book: these two radios
// have two books and two mode legends of different lengths (sixteen against
// twenty-six), so the shared outcome is a shared FACT about how Kenwood spells
// those three modes rather than one legend standing in for two.
func TestImportCHIRP_TS890And990BlockCWAndRTTYRows(t *testing.T) {
	for _, caps := range []spec.Capabilities{
		maLikeCapabilities("TS-890S", "024", ts890Modes),
		maLikeCapabilities("TS-990S", "022", ts990Modes),
	} {
		t.Run(caps.Model, func(t *testing.T) {
			// Precondition, stated rather than assumed: the three mapped
			// names are genuinely absent from this row's mode list.
			for _, absent := range []string{"CW-U", "CW-L", "RTTY-U"} {
				if containsMode(caps, absent) {
					t.Fatalf("fixture precondition: %q IS in the %s's Modes — this test is about the three names its legend does NOT print", absent, caps.Model)
				}
			}
			// And the four names this radio DOES print for those two
			// families, so a reader can see the refusal is about a spelling
			// rather than about a missing capability.
			for _, present := range []string{"CW", "CW-R", "FSK", "FSK-R"} {
				if !containsMode(caps, present) {
					t.Fatalf("fixture precondition: %q is NOT in the %s's Modes — this radio's own legend prints it, so the fixture has drifted from the driver", present, caps.Model)
				}
			}

			t.Run("CW, CWR and RTTY block", func(t *testing.T) {
				const csv = "Location,Name,Frequency,Mode\n" +
					"1,MORSE,7.030000,CW\n" +
					"2,MORSER,7.031000,CWR\n" +
					"3,TELETYPE,14.080000,RTTY\n"

				channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
				if err != nil {
					t.Fatalf("ImportCHIRP: unexpected error: %v", err)
				}
				if !report.HasBlocking() {
					t.Fatalf("HasBlocking() = false, want true: %+v", report.Entries)
				}
				for i, want := range []struct {
					line   int
					raw    string
					mapped string
				}{
					// LossEntry.Line counts the FILE's lines, so the header
					// is 1 and the three data rows are 2, 3 and 4.
					{2, "CW", "CW-U"},
					{3, "CWR", "CW-L"},
					{4, "RTTY", "RTTY-U"},
				} {
					var modeEntries []LossEntry
					for _, e := range entriesForLine(report, want.line) {
						if e.Column == "Mode" {
							modeEntries = append(modeEntries, e)
						}
					}
					if len(modeEntries) != 1 {
						t.Errorf("row %d: %d Mode entries, want exactly 1: %+v", i+1, len(modeEntries), modeEntries)
						continue
					}
					e := modeEntries[0]
					if e.Action != ActionUnsupported || !e.Blocking {
						t.Errorf("row %d: Mode entry = %+v, want a Blocking ActionUnsupported one", i+1, e)
					}
					if e.Value != want.raw {
						t.Errorf("row %d: entry Value = %q, want the CHIRP cell %q", i+1, e.Value, want.raw)
					}
					// The detail must name BOTH names, so a user can see the
					// refusal is about a NAME this radio's legend does not
					// print rather than about a mode it lacks.
					if !strings.Contains(e.Detail, want.raw) || !strings.Contains(e.Detail, want.mapped) {
						t.Errorf("row %d: Detail = %q, want it to name both the CHIRP mode %q and the mapped name %q", i+1, e.Detail, want.raw, want.mapped)
					}
				}
				for _, ch := range channels {
					if ch.Data != nil && ch.Data.Mode != "" {
						t.Errorf("channel %q imported Mode %q — a blocked row must not carry a mode at all", ch.Slot, ch.Data.Mode)
					}
				}
			})

			// THE FIVE ONE-NAME ROWS IMPORT, and this pair is the FIRST
			// designed under lane L's arm rather than having it applied
			// retrospectively (plan decision P15): a blank CHIRP Duplex cell
			// on a radio that publishes NO shift vocabulary at all is not a
			// loss, it is the radio's only state. Neither record carries a
			// duplex selector, so importCHIRPDuplexShift's ShiftNone arm
			// reports NOTHING: data.Shift stays "", which is what each
			// driver's own read produces and what its write treats as "not
			// requested".
			t.Run("the five one-name rows import as simplex with no Duplex entry", func(t *testing.T) {
				const csv = "Location,Name,Frequency,Mode\n" +
					"1,SIMPLEX,145.500000,FM\n" +
					"2,NARROW,145.525000,NFM\n" +
					"3,AIRBAND,118.000000,AM\n" +
					"4,UPPER,14.250000,USB\n" +
					"5,LOWER,7.100000,LSB\n"

				channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
				if err != nil {
					t.Fatalf("ImportCHIRP: unexpected error: %v", err)
				}
				if report.HasBlocking() {
					t.Fatalf("HasBlocking() = true, want false — a blank Duplex cell on a radio with no shift vocabulary is simplex, not a loss: %+v", report.Entries)
				}
				if len(channels) != 5 {
					t.Fatalf("imported %d channels, want 5", len(channels))
				}
				// Lines 2-6: the header is line 1.
				for line := 2; line <= 6; line++ {
					for _, e := range entriesForLine(report, line) {
						if e.Column == "Duplex" || e.Column == "Mode" {
							t.Errorf("line %d: entry %+v — these five names ARE in this radio's legend, and its blank Duplex cell is its only state, so neither column may report anything", line, e)
						}
					}
				}
				for i, ch := range channels {
					if ch.Data == nil {
						t.Fatalf("channels[%d].Data = nil", i)
					}
					if ch.Data.Shift != "" {
						t.Errorf("channels[%d].Shift = %q, want \"\" — this radio publishes no shift value to store, and \"\" is what its own read produces", i, ch.Data.Shift)
					}
					// TxFreqHz IS REACHED ON THESE ROWS, and a BLANK Duplex
					// cell is decision 4's ordinary simplex row, so since the
					// 09/09/2026 design the importer states what each book
					// prints a simplex channel's split parameters hold:
					// Known 0 (890:3217-3218, 990:2964-2965). Never ABSENT —
					// Absent is what codeplug.Validate reports as an error.
					// That closes the v1.5.x follow-up this line used to
					// defer.
					//
					// THIS FIXTURE HAS NO Tone/rToneFreq/cToneFreq COLUMNS AT
					// ALL (its header above is Location,Name,Frequency,Mode),
					// so both tone indices stay Unknown here regardless of
					// ruling B2 or its 2026-09-12-chirp-b1 (symmetric B1)
					// supersession: an absent column is not a fill value to
					// carry, on either ruling. That is a DIFFERENT case from
					// TestImportCHIRP_TS890And990TakeTheToneModeBranch's
					// blank-Tone row, where rToneFreq/cToneFreq ARE present
					// (88.5 each) and B1 now carries both — see that test for
					// the current (post-B1) tone-index behaviour on a real
					// CHIRP export. IT DOES NOT MAKE THIS ROW WRITABLE either
					// way, which this pin records rather than hides.
					if ch.Data.TxFreqHz != (codeplug.FreqField{State: codeplug.Known, Value: 0}) {
						t.Errorf("channels[%d].TxFreqHz = %+v, want Known 0 (890:3217-3218, 990:2964-2965)", i, ch.Data.TxFreqHz)
					}
				}
				if issues := validateImported(t, channels, caps); codeplug.HasErrors(issues) {
					t.Errorf("codeplug.Validate reported an error on a row that never mentioned split: %+v", issues)
				}
			})

			// "off" is the OTHER side of the same arm and still blocks:
			// CHIRP's "off" asserts "no duplex configured" as distinct from
			// simplex, and that distinction is one neither record can carry,
			// so refusing it is honest where agreeing with a blank cell is
			// not. The two subtests together are the guard on the arm:
			// re-adding an entry to the blank arm fails the zero-entry
			// assertion above, and flipping this arm's Blocking to false
			// fails here.
			t.Run("an off Duplex row still blocks", func(t *testing.T) {
				const csv = "Location,Name,Frequency,Duplex,Mode\n" +
					"1,OFFDUP,145.500000,off,FM\n"

				_, report, err := ImportCHIRP(strings.NewReader(csv), caps)
				if err != nil {
					t.Fatalf("ImportCHIRP: unexpected error: %v", err)
				}
				if !report.HasBlocking() {
					t.Fatalf("HasBlocking() = false, want true — CHIRP's \"off\" says something this radio cannot say: %+v", report.Entries)
				}
				var duplex []LossEntry
				for _, e := range entriesForLine(report, 2) {
					if e.Column == "Duplex" {
						duplex = append(duplex, e)
					}
				}
				if len(duplex) != 1 {
					t.Fatalf("%d Duplex entries, want exactly 1: %+v", len(duplex), duplex)
				}
				if e := duplex[0]; e.Action != ActionUnsupported || !e.Blocking || e.Value != "off" {
					t.Errorf("Duplex entry = %+v, want a Blocking ActionUnsupported one carrying the \"off\" cell", e)
				}
			})

			// THE TAG WIDTH IS TEN ON THESE ROWS, not the 590 pair's eight,
			// and the truncation path reads caps.TagLen — so a name of
			// exactly ten survives whole where a longer one is truncated with
			// a reported loss. Pinned here because this fixture is the only
			// place in this package that carries a ten-character Kenwood tag.
			t.Run("a ten-character name survives and an eleven-character one truncates", func(t *testing.T) {
				const csv = "Location,Name,Frequency,Mode\n" +
					"1,TENCHARSXX,145.500000,FM\n" +
					"2,ELEVENCHARS,145.525000,FM\n"

				channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
				if err != nil {
					t.Fatalf("ImportCHIRP: unexpected error: %v", err)
				}
				if report.HasBlocking() {
					t.Fatalf("HasBlocking() = true, want false: %+v", report.Entries)
				}
				if len(channels) != 2 {
					t.Fatalf("imported %d channels, want 2", len(channels))
				}
				if got := channels[0].Data.Tag; got != "TENCHARSXX" {
					t.Errorf("a ten-character name imported as %q, want it whole — caps.TagLen is 10 on this row", got)
				}
				if got := channels[1].Data.Tag; got != "ELEVENCHAR" {
					t.Errorf("an eleven-character name imported as %q, want it truncated to ten", got)
				}
				var nameEntries []LossEntry
				for _, e := range entriesForLine(report, 3) {
					if e.Column == "Name" {
						nameEntries = append(nameEntries, e)
					}
				}
				if len(nameEntries) != 1 {
					t.Errorf("%d Name entries on the eleven-character row, want exactly 1: %+v", len(nameEntries), nameEntries)
				}
				if len(entriesForLine(report, 2)) != 0 {
					t.Errorf("the ten-character row reported %+v, want nothing at all", entriesForLine(report, 2))
				}
			})
		})
	}
}

// TestImportCHIRP_TS890And990TakeTheToneModeBranch pins the branch this pair
// actually takes and the field states it leaves behind, because the fixture
// took the WRONG one until the milestone-close review (C-MED-1) measured it:
// both real rows grade FieldToneMode/FieldToneTx/FieldToneRx
// (core/driver/ts890/caps.go:267-269, core/driver/ts990/caps.go:246-248), so
// ImportCHIRP dispatches to importCHIRPToneIcom (core/csvio/chirp.go:596-600)
// and NOT to importCHIRPToneCTCSS, which is what a fixture that graded none
// of the three silently proved instead.
//
// WHAT THE BRANCH LEAVES UNKNOWN IS THE POINT. importCHIRPToneIcom starts
// both reachable tone indices at Unknown and only the columns a row actually
// carries move them: a blank Tone cell sets the MODE off and says nothing
// about either index; a `Tone` cell speaks for the transmit index alone.
// Both drivers' write ladders require mode, transmit tone AND receive tone to
// be Known, because every MA0 Set carries all three
// (core/driver/ts890/write.go:629-640, core/driver/ts990/write.go:857-872) —
// so an ordinary CHIRP row imports cleanly here and is then refused at the
// write until the user supplies all of them. That is the whole of the
// recovery this milestone's prose has to name, and nothing in this importer
// may invent any of it (the standing rule: an omitted semantic is REFUSED,
// never defaulted).
func TestImportCHIRP_TS890And990TakeTheToneModeBranch(t *testing.T) {
	for _, caps := range []spec.Capabilities{
		maLikeCapabilities("TS-890S", "024", ts890Modes),
		maLikeCapabilities("TS-990S", "022", ts990Modes),
	} {
		t.Run(caps.Model, func(t *testing.T) {
			// Precondition, stated rather than assumed: this fixture must
			// take the Icom branch, which is exactly what C-MED-1 found it
			// did not do.
			for _, f := range []spec.Field{spec.FieldToneMode, spec.FieldToneTx, spec.FieldToneRx} {
				if !reaches(caps, spec.BankMemory, f) {
					t.Fatalf("fixture precondition: MEM does not reach %s — the real driver grades all three, so this fixture takes importCHIRPToneCTCSS where the registered row takes importCHIRPToneIcom", f)
				}
			}

			// A blank Tone cell is the ordinary CHIRP row, and it is the one
			// the release prose describes a recovery for.
			t.Run("a blank Tone row leaves both indices Unknown and the mode Known OFF", func(t *testing.T) {
				// rToneFreq/cToneFreq are populated (88.5) although Tone is
				// blank: an Unknown index beside a populated tone column is
				// ruling B2's statement, not B1's — a fixture with both
				// columns empty cannot tell the two apart.
				const csv = "Location,Name,Frequency,Mode,Tone,rToneFreq,cToneFreq\n" +
					"1,SIMPLEX,145.500000,FM,,88.5,88.5\n"

				channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
				if err != nil {
					t.Fatalf("ImportCHIRP: unexpected error: %v", err)
				}
				if report.HasBlocking() {
					t.Fatalf("HasBlocking() = true, want false — a blank Tone cell is this radio's OFF state: %+v", report.Entries)
				}
				if len(channels) != 1 {
					t.Fatalf("imported %d channels, want 1", len(channels))
				}
				d := channels[0].Data
				if d.ToneMode.State != codeplug.Known || d.ToneMode.Value != "OFF" {
					t.Errorf("ToneMode = %+v, want Known %q — importCHIRPToneIcom's blank arm asks caps.CanonicalToneMode(spec.ToneModeOff), and this row's P5 legend prints OFF (890:3180-3185, 990:2915-2919)", d.ToneMode, "OFF")
				}
				// Symmetric B1 (2026-09-12): the row's rToneFreq/cToneFreq
				// columns are a complete statement of the channel even
				// though Tone mode is off, so both indices are carried —
				// this SUPERSEDES ruling B2, which left them Unknown.
				if d.ToneTx != (codeplug.ToneField{State: codeplug.Known, Value: 885}) {
					t.Errorf("ToneTx = %+v, want Known 88.5 from rToneFreq (B1 rule 2, not B2)", d.ToneTx)
				}
				if d.ToneRx != (codeplug.ToneField{State: codeplug.Known, Value: 885}) {
					t.Errorf("ToneRx = %+v, want Known 88.5 from cToneFreq (B1 rule 2, not B2)", d.ToneRx)
				}
				// The Yaesu half of the vocabulary pair is UNAVAILABLE, not
				// Unknown: neither record has such a field for the imported
				// channel to hold (both drivers grade FieldCTCSSTone the zero
				// FieldSupport), which is also what a READ of these rows
				// reports.
				if d.CTCSSTone.State != codeplug.Unavailable {
					t.Errorf("CTCSSTone = %+v, want Unavailable — neither record carries a ctcss_tone field", d.CTCSSTone)
				}
				if issues := validateImported(t, channels, caps); codeplug.HasErrors(issues) {
					t.Errorf("codeplug.Validate reported an error on a blank-Tone row: %+v", issues)
				}
			})

			// `Tone` is CHIRP's encode-only row: it speaks for the TRANSMIT
			// index and for nothing else.
			t.Run("a Tone row sets the transmit index and leaves the receive index Unknown", func(t *testing.T) {
				const csv = "Location,Name,Frequency,Mode,Tone,rToneFreq,cToneFreq\n" +
					"1,REPEATER,145.500000,FM,Tone,100.0,88.5\n" +
					"2,KENWOOD,145.525000,FM,Tone,1750.0,88.5\n"

				channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
				if err != nil {
					t.Fatalf("ImportCHIRP: unexpected error: %v", err)
				}
				if report.HasBlocking() {
					t.Fatalf("HasBlocking() = true, want false: %+v", report.Entries)
				}
				if len(channels) != 2 {
					t.Fatalf("imported %d channels, want 2", len(channels))
				}
				// 1750.0 Hz is the fifty-first entry of THIS pair's chart and
				// is absent from the project's shared fifty
				// (core/driver/ts890/caps.go:123-130,
				// core/driver/ts990/caps.go:123-128 against
				// core/spec/tones.go:133-140) — so the second row passing is
				// what makes the fixture's chart load-bearing rather than
				// decorative.
				for i, wantTx := range []spec.Tone{1000, 17500} {
					d := channels[i].Data
					if d.ToneMode.State != codeplug.Known || d.ToneMode.Value != "TONE" {
						t.Errorf("channels[%d].ToneMode = %+v, want Known %q", i, d.ToneMode, "TONE")
					}
					if d.ToneTx.State != codeplug.Known || d.ToneTx.Value != wantTx {
						t.Errorf("channels[%d].ToneTx = %+v, want Known %v from rToneFreq", i, d.ToneTx, wantTx)
					}
					// B1 rule 2: cToneFreq is a reachable field the Tone
					// arm did not assign, so it is still carried (88.5).
					if d.ToneRx != (codeplug.ToneField{State: codeplug.Known, Value: 885}) {
						t.Errorf("channels[%d].ToneRx = %+v, want Known 88.5 from cToneFreq (B1 rule 2)", i, d.ToneRx)
					}
				}
			})

			// TSQL is REFUSED on both rows, and that is the pair's own
			// capability speaking rather than a gap in this importer: each
			// P5/P6 legend prints OFF, TONE, CTCSS and CROSS, and each driver
			// grades its CTCSS value spec.ToneModeCTCSSRxSquelch
			// (core/driver/ts890/caps.go:589-594,
			// core/driver/ts990/caps.go:536-541). CHIRP's TSQL asks a
			// transceiver for spec.ToneModeCTCSSSquelch, which neither row
			// publishes, so the importer refuses instead of substituting the
			// receive-only semantic.
			//
			// WHAT THIS PINS IS TODAY'S CAPABILITY VALUE, NOT A PROPERTY OF
			// THE RADIO. That CTCSS-to-ToneModeCTCSSRxSquelch mapping is
			// ASSUMED on this pair and carries register entry K-D1
			// (core/driver/ts890/caps.go's own "THE SEMANTICS ARE ASSUMED AND
			// THE REGISTER ENTRY IS K-D1"), so if K-D1 lifts the other way
			// this outcome changes and this subtest is what says so.
			t.Run("a TSQL row is refused and moves no field", func(t *testing.T) {
				const csv = "Location,Name,Frequency,Mode,Tone,rToneFreq,cToneFreq\n" +
					"1,SQUELCH,145.500000,FM,TSQL,88.5,88.5\n"

				channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
				if err != nil {
					t.Fatalf("ImportCHIRP: unexpected error: %v", err)
				}
				if !report.HasBlocking() {
					t.Fatalf("HasBlocking() = false, want true — this row publishes no encode+decode tone mode: %+v", report.Entries)
				}
				var tone []LossEntry
				for _, e := range entriesForLine(report, 2) {
					if e.Column == "Tone" {
						tone = append(tone, e)
					}
				}
				if len(tone) != 1 {
					t.Fatalf("%d Tone entries, want exactly 1: %+v", len(tone), tone)
				}
				if e := tone[0]; e.Action != ActionUnsupported || !e.Blocking || e.Value != "TSQL" {
					t.Errorf("Tone entry = %+v, want a Blocking ActionUnsupported one carrying the TSQL cell", e)
				}
				if len(channels) != 1 {
					t.Fatalf("imported %d channels, want 1", len(channels))
				}
				d := channels[0].Data
				if d.ToneMode.State != codeplug.Unknown {
					t.Errorf("ToneMode = %+v, want Unknown — a refused mode may not be recorded", d.ToneMode)
				}
				if d.ToneTx.State != codeplug.Unknown || d.ToneRx.State != codeplug.Unknown {
					t.Errorf("ToneTx = %+v, ToneRx = %+v, want both Unknown — the refusal happens before either index is read", d.ToneTx, d.ToneRx)
				}
			})

			// Cross assigns nothing active (only the mode itself), so B1
			// rule 2 carries both indices from their own columns — the
			// pair's own P5/P6/P7 legends have no DTCS field to grade.
			t.Run("a Cross row carries both tone indices from their own columns", func(t *testing.T) {
				const csv = "Location,Name,Frequency,Mode,Tone,rToneFreq,cToneFreq\n" +
					"1,XCH,145.500000,FM,Cross,100.0,67.0\n"
				d := importOneChannel(t, csv, caps)
				if d.ToneMode.State != codeplug.Known || d.ToneMode.Value != "CROSS" {
					t.Errorf("ToneMode = %+v, want Known CROSS", d.ToneMode)
				}
				if d.ToneTx != (codeplug.ToneField{State: codeplug.Known, Value: 1000}) {
					t.Errorf("ToneTx = %+v, want Known 100.0 from rToneFreq", d.ToneTx)
				}
				if d.ToneRx != (codeplug.ToneField{State: codeplug.Known, Value: 670}) {
					t.Errorf("ToneRx = %+v, want Known 67.0 from cToneFreq", d.ToneRx)
				}
			})

			// This pair has no DTCS field at all (K-D1's chart is
			// tone-frequency only), so a non-default DtcsCode still gets
			// the pre-existing dropped treatment via chirpExtraColumns —
			// B1 changes nothing here, and CHIRP's own fill values stay
			// silent.
			t.Run("DtcsCode has no field on this pair: fill silent, other values dropped", func(t *testing.T) {
				const head = "Location,Name,Frequency,Mode,Tone,rToneFreq,cToneFreq,DtcsCode,DtcsPolarity\n"
				t.Run("023/NN fill values are silent", func(t *testing.T) {
					csv := head + "1,SIMPLEX,145.500000,FM,,88.5,88.5,023,NN\n"
					_, report, err := ImportCHIRP(strings.NewReader(csv), caps)
					if err != nil {
						t.Fatalf("ImportCHIRP: %v", err)
					}
					for _, e := range report.Entries {
						if e.Column == "DtcsCode" || e.Column == "DtcsPolarity" {
							t.Errorf("report = %+v, want no entry for the fill values", report.Entries)
						}
					}
				})
				t.Run("a real code is dropped, non-blocking", func(t *testing.T) {
					csv := head + "1,SIMPLEX,145.500000,FM,,88.5,88.5,025,NN\n"
					_, report, err := ImportCHIRP(strings.NewReader(csv), caps)
					if err != nil {
						t.Fatalf("ImportCHIRP: %v", err)
					}
					if report.HasBlocking() {
						t.Fatalf("report has blocking entries: %+v", report.Entries)
					}
					found := false
					for _, e := range report.Entries {
						if e.Column == "DtcsCode" {
							found = true
							if e.Action != ActionDropped || e.Blocking {
								t.Errorf("DtcsCode entry = %+v, want non-blocking ActionDropped", e)
							}
						}
					}
					if !found {
						t.Errorf("report = %+v, want a dropped entry for DtcsCode — this pair has no such field", report.Entries)
					}
				})
			})
		})
	}
}

// TestImportCHIRP_BlankDuplexTakesTheRowsOwnSimplexStatement is decision A3
// of the 09/09/2026 CHIRP transmit-disposition design: a blank CHIRP Duplex
// cell is decision 4's ordinary simplex row, so on a bank that reaches
// spec.FieldTxFrequency the importer states the transmit disposition the
// radio's OWN record gives a simplex channel, instead of leaving an Unknown
// that says the file was silent when it was not.
//
// There is no fleet-wide value, which is the whole reason the datum exists:
// the TS-890S/TS-990S records print that every split parameter reads 0
// (SimplexTxZero), whilst the TS-590 pair and the IC-7300 pair have no split
// flag to read and express simplex as tx == rx (SimplexTxEqualsRx). A row
// declaring NOTHING keeps the old behaviour exactly.
//
// This does NOT re-introduce the substitution struck in
// core/driver/ic7300/write.go: that rule let a DRIVER invent a transmit
// frequency for a channel whose TxFreqHz was simply not Known, with no file
// behind it. Here the importer states what its file already said.
func TestImportCHIRP_BlankDuplexTakesTheRowsOwnSimplexStatement(t *testing.T) {
	const blank = "Location,Name,Frequency,Duplex,Mode\n1,SIMPLEX,29.600000,,FM\n"

	t.Run("SimplexTxZero imports Known 0", func(t *testing.T) {
		caps := maLikeCapabilities("TS-890S", "024", ts890Modes)
		if caps.SimplexTx != spec.SimplexTxZero {
			t.Fatalf("fixture SimplexTx = %v, want SimplexTxZero — the fixture must mirror core/driver/ts890/caps.go", caps.SimplexTx)
		}
		got := importOneChannel(t, blank, caps)
		if got.TxFreqHz != (codeplug.FreqField{State: codeplug.Known, Value: 0}) {
			t.Errorf("TxFreqHz = %+v, want Known 0 (890:3217-3218)", got.TxFreqHz)
		}
	})

	t.Run("SimplexTxEqualsRx imports Known at the row's own frequency", func(t *testing.T) {
		caps := ts590LikeCapabilities("TS-590SG", "023")
		if caps.SimplexTx != spec.SimplexTxEqualsRx {
			t.Fatalf("fixture SimplexTx = %v, want SimplexTxEqualsRx — the fixture must mirror core/driver/ts590/caps.go", caps.SimplexTx)
		}
		got := importOneChannel(t, blank, caps)
		if got.TxFreqHz != (codeplug.FreqField{State: codeplug.Known, Value: 29_600_000}) {
			t.Errorf("TxFreqHz = %+v, want Known 29600000, this row's own frequency (590:1521-1523)", got.TxFreqHz)
		}
	})

	// THE ZERO VALUE'S GUARD. An unregistered row that says nothing must
	// not be defaulted into an encoding nobody read, so it keeps the
	// Unknown it had before this datum existed — and the write keeps
	// refusing it.
	t.Run("SimplexTxUnstated is unchanged", func(t *testing.T) {
		caps := ts590LikeCapabilities("TEST-UNSTATED", "000")
		caps.SimplexTx = spec.SimplexTxUnstated
		got := importOneChannel(t, blank, caps)
		if got.TxFreqHz.State != codeplug.Unknown {
			t.Errorf("TxFreqHz = %+v, want Unknown — a row declaring nothing must behave exactly as it did before SimplexTx existed", got.TxFreqHz)
		}
	})

	// "off" IS UNTOUCHED. Decision 4 stands: CHIRP's "off" asserts "no
	// duplex configured" as distinct from simplex, which these records
	// cannot say, so it still blocks — and a blocked row states no
	// transmit disposition either.
	t.Run("an off Duplex row still blocks and states nothing", func(t *testing.T) {
		const csv = "Location,Name,Frequency,Duplex,Mode\n1,OFFDUP,29.600000,off,FM\n"
		caps := ts590LikeCapabilities("TS-590SG", "023")
		channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
		if err != nil {
			t.Fatalf("ImportCHIRP: unexpected error: %v", err)
		}
		if !report.HasBlocking() {
			t.Fatalf("HasBlocking() = false, want true: %+v", report.Entries)
		}
		for i, ch := range channels {
			if ch.Data != nil && ch.Data.TxFreqHz.State != codeplug.Unknown {
				t.Errorf("channels[%d].TxFreqHz = %+v, want Unknown — \"off\" is not a simplex statement", i, ch.Data.TxFreqHz)
			}
		}
	})
}

// importOneChannel imports a one-row CHIRP fixture and returns that row's
// channel data, failing the test if the row did not import cleanly.
func importOneChannel(t *testing.T, csv string, caps spec.Capabilities) *codeplug.ChannelData {
	t.Helper()
	channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
	if err != nil {
		t.Fatalf("ImportCHIRP: unexpected error: %v", err)
	}
	if report.HasBlocking() {
		t.Fatalf("HasBlocking() = true, want false: %+v", report.Entries)
	}
	if len(channels) != 1 || channels[0].Data == nil {
		t.Fatalf("imported %d channels, want exactly 1 with data", len(channels))
	}
	return channels[0].Data
}

// TestImportCHIRP_TS590PairTakesTheToneModeBranch is v1.5.x follow-up (l):
// until 09/09/2026 ts590LikeCapabilities graded NONE of
// FieldToneMode/FieldToneTx/FieldToneRx, so every pair-1 test took
// importCHIRPToneCTCSS whilst the registered rows take importCHIRPToneIcom
// (core/driver/ts590/caps.go's bankFields grades all three). The fixture's
// evidence was therefore about a branch these two radios never reach.
//
// The three shapes are §1.2 of the 09/09/2026 design, and the pair's answers
// are the TS-890S/TS-990S ones for the same reason: the 590 record spells
// tone as a mode selector with two independent indices, and its CTCSS legend
// value maps to spec.ToneModeCTCSSRxSquelch (ASSUMED, the same class of
// reading as register entry K-D1 on pair 2), which is receive squelch only —
// so CHIRP's TSQL, which asks for encode+decode, has no wire value here and
// is refused rather than substituted. That refusal is follow-up (k)'s
// question answered: the 590 pair does what pair 2 does, in the same words.
//
// A BLANK-Tone ROW CARRIES BOTH INDICES (design 2026-09-12-chirp-b1,
// symmetric B1): the file's rToneFreq/cToneFreq columns are a complete
// statement of the channel even though this row's Tone mode does not use
// them, so both are Known 88.5. This SUPERSEDES ruling B2, which held
// (09/09/2026) that the two columns were CHIRP's own defaults and not
// really data — B1 reopened that call.
func TestImportCHIRP_TS590PairTakesTheToneModeBranch(t *testing.T) {
	for _, caps := range []spec.Capabilities{
		ts590LikeCapabilities("TS-590S", "021"),
		ts590LikeCapabilities("TS-590SG", "023"),
	} {
		t.Run(caps.Model, func(t *testing.T) {
			for _, f := range []spec.Field{spec.FieldToneMode, spec.FieldToneTx, spec.FieldToneRx} {
				if !reaches(caps, spec.BankMemory, f) {
					t.Fatalf("fixture precondition: MEM does not reach %s — the real driver grades all three, so this fixture takes importCHIRPToneCTCSS where the registered row takes importCHIRPToneIcom", f)
				}
			}

			t.Run("a blank Tone row carries both indices from CHIRP's own fill values, mode Known OFF", func(t *testing.T) {
				d := importOneChannel(t, "Location,Name,Frequency,Mode,Tone,rToneFreq,cToneFreq\n1,SIMPLEX,145.500000,FM,,88.5,88.5\n", caps)
				if d.ToneMode.State != codeplug.Known || d.ToneMode.Value != "OFF" {
					t.Errorf("ToneMode = %+v, want Known %q", d.ToneMode, "OFF")
				}
				// Symmetric B1 (2026-09-12) SUPERSEDES ruling B2: the row's
				// rToneFreq/cToneFreq are a complete statement of the
				// channel even with Tone mode off, so both are carried.
				if d.ToneTx != (codeplug.ToneField{State: codeplug.Known, Value: 885}) {
					t.Errorf("ToneTx = %+v, want Known 88.5 from rToneFreq (B1 rule 2, not B2)", d.ToneTx)
				}
				if d.ToneRx != (codeplug.ToneField{State: codeplug.Known, Value: 885}) {
					t.Errorf("ToneRx = %+v, want Known 88.5 from cToneFreq (B1 rule 2, not B2)", d.ToneRx)
				}
			})

			t.Run("a Tone row sets the transmit index and carries the receive index from cToneFreq", func(t *testing.T) {
				d := importOneChannel(t, "Location,Name,Frequency,Mode,Tone,rToneFreq,cToneFreq\n1,REPEATER,145.500000,FM,Tone,100.0,88.5\n", caps)
				if d.ToneMode.State != codeplug.Known || d.ToneMode.Value != "TONE" {
					t.Errorf("ToneMode = %+v, want Known %q", d.ToneMode, "TONE")
				}
				if d.ToneTx.State != codeplug.Known || d.ToneTx.Value != spec.Tone(1000) {
					t.Errorf("ToneTx = %+v, want Known 1000 from rToneFreq", d.ToneTx)
				}
				// B1 rule 2: cToneFreq is a reachable field the Tone arm
				// did not assign, so it is still carried (88.5).
				if d.ToneRx != (codeplug.ToneField{State: codeplug.Known, Value: 885}) {
					t.Errorf("ToneRx = %+v, want Known 88.5 from cToneFreq (B1 rule 2)", d.ToneRx)
				}
			})

			t.Run("a TSQL row is refused and moves no field", func(t *testing.T) {
				const csv = "Location,Name,Frequency,Mode,Tone,rToneFreq,cToneFreq\n1,SQUELCH,145.500000,FM,TSQL,88.5,88.5\n"
				channels, report, err := ImportCHIRP(strings.NewReader(csv), caps)
				if err != nil {
					t.Fatalf("ImportCHIRP: unexpected error: %v", err)
				}
				if !report.HasBlocking() {
					t.Fatalf("HasBlocking() = false, want true — this row publishes no encode+decode tone mode: %+v", report.Entries)
				}
				var tone []LossEntry
				for _, e := range entriesForLine(report, 2) {
					if e.Column == "Tone" {
						tone = append(tone, e)
					}
				}
				if len(tone) != 1 {
					t.Fatalf("%d Tone entries, want exactly 1: %+v", len(tone), tone)
				}
				if e := tone[0]; e.Action != ActionUnsupported || !e.Blocking || e.Value != "TSQL" {
					t.Errorf("Tone entry = %+v, want a Blocking ActionUnsupported one carrying the TSQL cell", e)
				}
				d := channels[0].Data
				if d.ToneMode.State != codeplug.Unknown || d.ToneTx.State != codeplug.Unknown || d.ToneRx.State != codeplug.Unknown {
					t.Errorf("ToneMode = %+v, ToneTx = %+v, ToneRx = %+v, want all Unknown", d.ToneMode, d.ToneTx, d.ToneRx)
				}
			})

			t.Run("a Cross row carries both tone indices from their own columns", func(t *testing.T) {
				d := importOneChannel(t, "Location,Name,Frequency,Mode,Tone,rToneFreq,cToneFreq\n1,XCH,145.500000,FM,Cross,100.0,67.0\n", caps)
				if d.ToneMode.State != codeplug.Known || d.ToneMode.Value != "CROSS" {
					t.Errorf("ToneMode = %+v, want Known CROSS", d.ToneMode)
				}
				if d.ToneTx != (codeplug.ToneField{State: codeplug.Known, Value: 1000}) {
					t.Errorf("ToneTx = %+v, want Known 100.0 from rToneFreq", d.ToneTx)
				}
				if d.ToneRx != (codeplug.ToneField{State: codeplug.Known, Value: 670}) {
					t.Errorf("ToneRx = %+v, want Known 67.0 from cToneFreq", d.ToneRx)
				}
			})
		})
	}
}
