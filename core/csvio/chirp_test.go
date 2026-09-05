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

// registeredRadioCapabilities returns one fixture per model registered in
// internal/wiring's driver tables, each mirroring that model's real
// scan-skip support. It is a table of FIXTURES rather than a walk of the
// registry for the layering reason ft710LikeCapabilities gives — core/csvio
// must not import core/driver, and internal/wiring imports every driver —
// so each entry cites the caps site it mirrors, and drift between the two
// is caught end-to-end by the CLI byte-identity baseline, which does use
// the real drivers.
//
// Every registered model's scan_skip is the zero FieldSupport today, which
// is exactly why the caps-aware branch (M9d-2 task 8, spec decision 5) is
// the one every real import takes: FT-710 (ft710/caps.go's bankFields —
// the 28-byte MR/MW layout has no scan-skip position), FTdx10
// (ftdx10/caps.go's bankFields), FTdx101D and FTdx101MP
// (ftdx101/caps.go's bankFields, one map for both siblings). The
// writable-radio branch has no registered radio at all and is pinned
// separately against writableCapabilities.
//
// THE LIST IS HAND-WRITTEN AND THIS TEST CANNOT NOTICE A FIFTH MODEL.
// Registering one adds no row here by itself, so this table would go on
// claiming to cover "every registered radio" while silently skipping it.
// The registry-walk pin lives where the registry does —
// internal/wiring.SupportedModels and
// TestSupportedModels_ContainsEveryRegisteredModel (internal/wiring/
// wiring_test.go), which walks the real tables — and it does not know
// about this file. Registering a model is therefore a two-place change:
// its row goes here as well, mirroring that driver's own scan-skip
// support, whichever branch it lands in. A model whose scan-skip is
// genuinely writable belongs in the literal-branch test instead, and the
// Unreachable precondition assertion below is what will say so.
func registeredRadioCapabilities() []spec.Capabilities {
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
	}
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

	for _, caps := range registeredRadioCapabilities() {
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
// "RTTY" to "RTTY-U" — the sideband-specific names three of the four
// registered Yaesu models print. The FT-891 prints "CW", "CW-R",
// "RTTY-LSB" and "RTTY-USB", so none of those three mapped names is in its
// caps.Modes and containsMode says no. Each such row therefore BLOCKS with
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
