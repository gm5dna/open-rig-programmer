// SPDX-License-Identifier: GPL-3.0-or-later

package fakets990

import (
	"strings"
	"testing"
)

// printedFrequencies is the whole supply this book gives, re-derived here
// rather than taken from image.go: "00007000000" from the front matter's FA
// worked example (990:86-89, quoted again at 990:119 and 990:126), "00014175000"
// from the AS2 block ("for example, 14.175 MHz is displayed as 00014175000",
// 990:344-345) and "00014195000" from FA's own parameter note ("For example,
// enter 00014195000 for 14.195 MHz", 990:2352, and FB's at 990:2374). THE 990S
// PRINTS THREE where the 890S prints two. "00000000000" is not a fourth — it
// is the printed state of a single memory channel's frequency-2 parameters
// (990:2964-2965) — and the eleven-space spelling is A6's reading of "blank"
// (990:2962-2963).
var printedFrequencies = map[string]bool{
	"00007000000": true,
	"00014175000": true,
	"00014195000": true,
	"00000000000": true,
	"           ": true,
}

// omNibbles is the OM P2 legend, all TWENTY-FOUR (990:3707-3730), plus the
// blank spelling. The 890S's legend prints sixteen: 0-9 and A-F.
const omNibbles = "0123456789ABCDEFGHIJKLMN "

// kyCharset is the character set this book prints, restricted to its
// unambiguously ASCII members: upper and lower case, digits, space, and the
// punctuation ' " ( ) * + , . / : = ? @ (990:2770-2778).
//
// THE PRINTED DASH IS DELIBERATELY EXCLUDED. It renders in the layout text as
// an EN DASH (U+2013), which is not an ASCII byte, and this book's own coding
// rule remaps 80h-FFh by Menu 9-01 (990:32-39) — so which byte a radio would
// accept for that glyph is unknown, and a name using it would be an invented
// byte rather than a printed one.
//
// The charset is used here ONLY as a source of characters that certainly exist
// in this radio's coding. It is NOT evidence for what a MEMORY NAME accepts,
// which is the design's A2 and is assumed, not printed.
const kyCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 '\"()*+,./:=?@"

// TestDefaultImage_EveryByteIsAPrintedValue holds the evidence posture
// mechanically: every byte of every shipped record is a printed constant, one
// of the three printed example frequencies, a printed legend value, a
// KY-charset name character, or A6's blank spelling. Nothing is invented,
// derived from another model, or padded to make a test pass.
//
// What it does NOT establish is the RECORD: no literal complete MA0 answer
// appears in this book, so the cross-field combination is this project's. That
// is the design's A22 and PROVENANCE.md carries it.
func TestDefaultImage_EveryByteIsAPrintedValue(t *testing.T) {
	img := DefaultImage()
	if len(img) == 0 {
		t.Fatal("DefaultImage is empty — every assertion below would pass vacuously")
	}
	for slot, s := range img {
		if !printedFrequencies[s.Freq] {
			t.Errorf("channel %03d: Freq %q is not one this book prints", slot, s.Freq)
		}
		if !printedFrequencies[s.Freq2] {
			t.Errorf("channel %03d: Freq2 %q is not one this book prints", slot, s.Freq2)
		}
		if !strings.ContainsRune(omNibbles, rune(s.Mode)) {
			t.Errorf("channel %03d: Mode %q is not in the OM legend (990:3707-3730)", slot, string(s.Mode))
		}
		if !strings.ContainsRune(omNibbles, rune(s.Mode2)) {
			t.Errorf("channel %03d: Mode2 %q is not in the OM legend (990:3707-3730)", slot, string(s.Mode2))
		}
		for _, f := range []struct {
			name string
			b    byte
			set  string
		}{
			{"Class (990:2898-2900)", s.Class, "012 "},
			{"FMNarrow (990:2913-2914)", s.FMNarrow, "01 "},
			{"FMNarrow2 (990:2933-2934)", s.FMNarrow2, "01 "},
			{"ToneType (990:2916-2919)", s.ToneType, "0123 "},
			{"ToneType2 (990:2936-2939)", s.ToneType2, "0123 "},
			{"Split (990:2947-2948)", s.Split, "01 "},
			{"DualRX (990:2950-2951)", s.DualRX, "01 "},
			// P17 IS 1/2 ON THIS RADIO, not 0/1 — erratum E8. A record
			// carrying the 890S's '0' here would be a byte this book prints
			// nowhere.
			{"Lockout (990:2953-2954, E8)", s.Lockout, "12 "},
		} {
			if !strings.ContainsRune(f.set, rune(f.b)) {
				t.Errorf("channel %03d: %s is %q, not a printed legend value", slot, f.name, string(f.b))
			}
		}
		for _, f := range []struct {
			name  string
			value string
			max   int
		}{
			{"ToneNo (TN 00 ~ 50, 990:4949-4974)", s.ToneNo, 50},
			{"CTCSSNo (CN 00 ~ 49, 990:1240-1267)", s.CTCSSNo, 49},
			{"ToneNo2 (TN 00 ~ 50, 990:4949-4974)", s.ToneNo2, 50},
			{"CTCSSNo2 (CN 00 ~ 49, 990:1240-1267)", s.CTCSSNo2, 49},
		} {
			if f.value == "  " {
				continue // A6's blank spelling
			}
			n, err := atoiTwo(f.value)
			if err != nil || n > f.max {
				t.Errorf("channel %03d: %s is %q, not an index the chart prints", slot, f.name, f.value)
			}
		}
		if len(s.Name) != maxNameLen {
			t.Errorf("channel %03d: Name %q is %d bytes, and the window is a FIXED ten (990:2955-2956)", slot, s.Name, len(s.Name))
		}
		for _, c := range s.Name {
			if !strings.ContainsRune(kyCharset, c) {
				t.Errorf("channel %03d: Name %q carries %q, which this book prints nowhere (990:2770-2778)", slot, s.Name, c)
			}
		}
	}
}

// atoiTwo parses a two-digit index cell.
func atoiTwo(s string) (int, error) {
	if len(s) != 2 || s[0] < '0' || s[0] > '9' || s[1] < '0' || s[1] > '9' {
		return 0, errNotTwoDigits
	}
	return int(s[0]-'0')*10 + int(s[1]-'0'), nil
}

var errNotTwoDigits = errString("not two ASCII digits")

type errString string

func (e errString) Error() string { return string(e) }

// TestDefaultImage_HasOneBlankShapeAndItIsPrinted. THIS ROW NEEDS NO SECOND
// BLANK FIXTURE, and the reason is a difference between the two books rather
// than a smaller effort here: this one's blank-channel note covers "P2 to P18"
// (990:2962-2963), the name window included, so the blank answer is MA0 + the
// slot + fifty spaces + ';' — printed, modulo A6. The 890S's note stops at P12
// (erratum E4), which is why that fake carries a second image with a residual
// name in its name window under A21, and why nothing of the sort may be
// invented here.
func TestDefaultImage_HasOneBlankShapeAndItIsPrinted(t *testing.T) {
	blank := BlankRecord()
	for _, s := range DefaultImage() {
		if s == blank {
			t.Error("the default image ships a wholly blank record; every unpopulated channel already answers the blank frame through handleMA0Read's one path")
		}
	}

	_, conn := newTestRadio(t)
	got := exchange(t, conn, "MA0050;")
	if want := blankFrame("050"); got != want {
		t.Errorf("an unwritten channel answered %q, want %q (990:2962-2963)", got, want)
	}
	if len(got) != recLen {
		t.Errorf("the blank answer is %d bytes, want %d — the terminator is nailed to position 57 (990:2915)", len(got), recLen)
	}
}

// TestDefaultImage_EveryLiveLegendValueAppears, so that no driver behaviour on
// one of them is a fixture accident. P5/P11, P15 and P16 are two-value bytes,
// P17 is the 1/2 pair this radio spends the datum on (E8), and P6's four tone
// types are checked as a set for the same reason.
//
// P2'S THIRD VALUE IS DELIBERATELY ABSENT. "2: Section defined Memory channel"
// (990:2900) is printed, but what such a channel's P9 holds is NOT — the
// section's end frequency is MA6's (990:3051), and the design's A8 records
// that P9 is assumed not to carry it — so composing one would put an
// unevidenced combination in an image. WithChannel is the seam for staging one
// when a driver's refusal needs it.
func TestDefaultImage_EveryLiveLegendValueAppears(t *testing.T) {
	seen := map[string]map[byte]bool{"P2": {}, "P5": {}, "P11": {}, "P6": {}, "P15": {}, "P16": {}, "P17": {}}
	for _, s := range DefaultImage() {
		seen["P2"][s.Class] = true
		seen["P5"][s.FMNarrow] = true
		seen["P11"][s.FMNarrow2] = true
		seen["P6"][s.ToneType] = true
		seen["P15"][s.Split] = true
		seen["P16"][s.DualRX] = true
		seen["P17"][s.Lockout] = true
	}
	for _, tt := range []struct {
		p    string
		want []byte
	}{
		{"P2", []byte{'0', '1'}},
		{"P5", []byte{'0', '1'}},
		{"P11", []byte{'0', '1'}},
		{"P6", []byte{'0', '1', '2', '3'}},
		{"P15", []byte{'0', '1'}},
		{"P16", []byte{'0', '1'}},
		{"P17", []byte{'1', '2'}},
	} {
		for _, want := range tt.want {
			if !seen[tt.p][want] {
				t.Errorf("no channel in the default image carries %s = %q", tt.p, string(want))
			}
		}
	}
	if seen["P2"]['2'] {
		t.Error("the default image composes a Section defined Memory channel; what its P9 holds is unprinted (A8, MA6 at 990:3051)")
	}
}

// TestDefaultImage_TheFourToneIndexWindowsAreDistinctAndNonZero. This grid has
// FOUR two-digit index windows — P7, P8, P13 and P14 at positions 22-23,
// 24-25, 40-41 and 42-43 — and the TN and CN charts print identical
// frequencies at indices 00-49 (990:4949-4974, 990:1240-1267), so an image
// leaving them all "00" reads the same whether a driver takes the right window
// or a neighbouring one, or drops the index entirely.
//
// This is the T15 review's M2, ruled ACCEPT for both fakes and made harder
// here because this radio has four windows where the 890S has two.
func TestDefaultImage_TheFourToneIndexWindowsAreDistinctAndNonZero(t *testing.T) {
	seen := map[string][]string{}
	for _, s := range DefaultImage() {
		for _, f := range []struct{ name, value string }{
			{"P7", s.ToneNo}, {"P8", s.CTCSSNo}, {"P13", s.ToneNo2}, {"P14", s.CTCSSNo2},
		} {
			if f.value != "00" && f.value != "  " {
				seen[f.name] = append(seen[f.name], f.value)
			}
		}
	}
	for _, p := range []string{"P7", "P8", "P13", "P14"} {
		if len(seen[p]) == 0 {
			t.Errorf("no channel in the default image carries a NON-ZERO %s: a P7/P8/P13/P14 offset error, or a dropped index, reads %q either way", p, "00")
		}
	}
	values := map[string]string{}
	for p, vs := range seen {
		for _, v := range vs {
			if other, clash := values[v]; clash {
				t.Errorf("%s and %s both carry index %q — two windows with one value cannot separate a transposition", p, other, v)
			}
			values[v] = p
		}
	}
}

// TestDefaultImage_EachCallIsIndependent and TestTwoRadiosFromOneImageDoNotAlias
// pin the Image contract: each call returns a freshly built map, so two
// *Radio instances never share mutable state.
func TestDefaultImage_EachCallIsIndependent(t *testing.T) {
	first := DefaultImage()
	first[0] = MemState{Freq: "mutated"}
	if got := DefaultImage()[0]; got.Freq == "mutated" {
		t.Error("DefaultImage returned a map a previous caller had mutated")
	}
}

func TestTwoRadiosFromOneImageDoNotAlias(t *testing.T) {
	img := Image(DefaultImage)
	a, connA := newTestRadio(t, WithFactoryImage(img))
	b, _ := newTestRadio(t, WithFactoryImage(img))

	writeFrame(t, connA, fieldsWithName("020", "A").frame())
	assertNoReply(t, connA)
	if _, ok := a.ChannelState(20); !ok {
		t.Fatal("the Set did not reach radio A")
	}
	if _, ok := b.ChannelState(20); ok {
		t.Error("a Set to radio A appeared in radio B — the two share one record map")
	}
}

// TestBlankRecord_IsEveryFieldBlank pins A6's reading in one place, the fixed
// name window included: on this radio "blank" covers P2 to P18
// (990:2962-2963), so there is no field of the grid it does not reach.
func TestBlankRecord_IsEveryFieldBlank(t *testing.T) {
	b := BlankRecord()
	for _, f := range []struct {
		name  string
		value string
	}{
		{"Class", string(b.Class)},
		{"Freq", b.Freq}, {"Freq2", b.Freq2},
		{"Mode", string(b.Mode)}, {"Mode2", string(b.Mode2)},
		{"FMNarrow", string(b.FMNarrow)}, {"FMNarrow2", string(b.FMNarrow2)},
		{"ToneType", string(b.ToneType)}, {"ToneType2", string(b.ToneType2)},
		{"ToneNo", b.ToneNo}, {"CTCSSNo", b.CTCSSNo},
		{"ToneNo2", b.ToneNo2}, {"CTCSSNo2", b.CTCSSNo2},
		{"Split", string(b.Split)}, {"DualRX", string(b.DualRX)},
		{"Lockout", string(b.Lockout)}, {"Name", b.Name},
	} {
		if strings.Trim(f.value, " ") != "" || f.value == "" {
			t.Errorf("BlankRecord().%s = %q, want its width in ASCII spaces (A6, 990:2962-2963)", f.name, f.value)
		}
	}
}

// TestWithEmptyChannel_MakesAPopulatedChannelAnswerBlank. It introduces no new
// assumed behaviour: it removes a map entry, which triggers the fake's
// existing documented blank-channel answer.
func TestWithEmptyChannel_MakesAPopulatedChannelAnswerBlank(t *testing.T) {
	if _, ok := DefaultImage()[0]; !ok {
		t.Fatal("channel 000 is not populated by the default image — this test needs one that is")
	}
	_, conn := newTestRadio(t, WithEmptyChannel(0))
	if got, want := exchange(t, conn, "MA0000;"), blankFrame("000"); got != want {
		t.Errorf("MA0000; -> %q, want %q", got, want)
	}
}

// TestWithChannel_StoresTheRecordVerbatim. No validation is applied, so a test
// may craft a channel whose ANSWER is deliberately malformed and drive a real
// driver's parse-error path through a real fake — and, on this row, may stage
// the Section defined channel (990:2900) the default image declines to
// compose.
func TestWithChannel_StoresTheRecordVerbatim(t *testing.T) {
	s := BlankRecord()
	s.Class = '2'
	_, conn := newTestRadio(t, WithChannel(30, s))
	got := exchange(t, conn, "MA0030;")
	if want := "MA0030" + "2" + strings.Repeat(" ", 49) + ";"; got != want {
		t.Errorf("MA0030; -> %q, want %q", got, want)
	}
}
