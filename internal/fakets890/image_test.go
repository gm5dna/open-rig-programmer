// SPDX-License-Identifier: GPL-3.0-or-later

package fakets890

import (
	"strings"
	"testing"
)

// printedFrequencies is the whole supply this book gives, re-derived here
// rather than taken from image.go: "00007000000" from the front matter's FA
// worked example (890:86, 890:118, 890:125) and "00014175000" from the AS0
// block (890:342). THE 890S PRINTS ONLY TWO. "00000000000" is not a third —
// it is the printed state of a single memory channel's split-transmission
// parameters (890:3217-3218) — and the eleven-space spelling is A6's reading
// of "blank" (890:3215-3216).
var printedFrequencies = map[string]bool{
	"00007000000": true,
	"00014175000": true,
	"00000000000": true,
	"           ": true,
}

// omNibbles is the OM P2 legend, all sixteen (890:3976-3992), plus the blank
// spelling.
const omNibbles = "0123456789ABCDEF "

// kyCharset is the character set this book prints, restricted to its
// unambiguously ASCII members: upper and lower case, digits, space, and the
// punctuation ' " ( ) * + , . / : = ? @ (890:2891-2900).
//
// THE PRINTED DASH IS DELIBERATELY EXCLUDED. It renders in the layout text as
// an EN DASH (U+2013), which is not an ASCII byte, and this book's own coding
// rule remaps 80h-FFh by Menu 9-01 (890:31-43) — so which byte a radio would
// accept for that glyph is unknown, and a name using it would be an invented
// byte rather than a printed one.
//
// The charset is used here ONLY as a source of characters that certainly exist
// in this radio's coding. It is NOT evidence for what a MEMORY NAME accepts,
// which is the design's A2 and is assumed, not printed.
const kyCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 '\"()*+,./:=?@"

// TestDefaultImage_EveryByteIsAPrintedValue holds the evidence posture
// mechanically: every byte of every shipped record is a printed constant, one
// of the two printed example frequencies, a printed legend value, a KY-charset
// name character, or A6's blank spelling. Nothing is invented, derived from
// another model, or padded to make a test pass.
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
		if !printedFrequencies[s.SplitFreq] {
			t.Errorf("channel %03d: SplitFreq %q is not one this book prints", slot, s.SplitFreq)
		}
		if !strings.ContainsRune(omNibbles, rune(s.Mode)) {
			t.Errorf("channel %03d: Mode %q is not in the OM legend (890:3976-3992)", slot, string(s.Mode))
		}
		if !strings.ContainsRune(omNibbles, rune(s.SplitMode)) {
			t.Errorf("channel %03d: SplitMode %q is not in the OM legend (890:3976-3992)", slot, string(s.SplitMode))
		}
		for _, f := range []struct {
			name string
			b    byte
			set  string
		}{
			{"FMNarrow (890:3177-3178)", s.FMNarrow, "01 "},
			{"SplitFMNarrow (890:3199-3200)", s.SplitFMNarrow, "01 "},
			{"Split (890:3202-3203)", s.Split, "01 "},
			{"Lockout (890:3206-3207)", s.Lockout, "01 "},
			{"ToneType (890:3181-3185)", s.ToneType, "0123 "},
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
			{"ToneNo (TN 00 ~ 50, 890:5149-5163)", s.ToneNo, 50},
			{"CTCSSNo (CN 00 ~ 49, 890:1354-1369)", s.CTCSSNo, 49},
		} {
			if f.value == "  " {
				continue // A6's blank spelling
			}
			n, err := atoiTwo(f.value)
			if err != nil || n > f.max {
				t.Errorf("channel %03d: %s is %q, not an index the chart prints", slot, f.name, f.value)
			}
		}
		if len(s.Name) > 10 {
			t.Errorf("channel %03d: Name %q is %d characters, and the field is \"Up to 10\" (890:3208-3209)", slot, s.Name, len(s.Name))
		}
		for _, c := range s.Name {
			if !strings.ContainsRune(kyCharset, c) {
				t.Errorf("channel %03d: Name %q carries %q, which this book prints nowhere (890:2891-2900)", slot, s.Name, c)
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

// TestDefaultImage_CarriesBothBlankFixtures is the plan's own requirement:
// TWO blank shapes, each bound to an explicit choice.
//
//   - the 40-byte frame with NO name window, which is A4's reading of a note
//     that stops at P12 (890:3215-3216, erratum E4) — reached by ANY channel
//     the image does not populate, through the same path every unwritten slot
//     takes;
//   - a second image carrying a RESIDUAL NAME in P13, which is A21: the
//     separate assumption that a residue, if one exists, is not channel
//     content. The empty predicate must be shown IGNORING it rather than
//     erroring on it, because core/clone's ReadAll abandons the whole read on
//     the first channel error.
func TestDefaultImage_CarriesBothBlankFixtures(t *testing.T) {
	img := DefaultImage()
	residual, ok := img[residualNameSlot]
	if !ok {
		t.Fatalf("the default image has no record at channel %03d — the A21 fixture is missing", residualNameSlot)
	}
	if residual.Name == "" {
		t.Errorf("channel %03d's Name is empty; the A21 fixture is a blank record CARRYING a residual name", residualNameSlot)
	}
	blank := BlankRecord()
	stripped := residual
	stripped.Name = ""
	if stripped != blank {
		t.Errorf("channel %03d is %+v; apart from its name it must be exactly BlankRecord() (%+v), or it is not a blank channel at all", residualNameSlot, residual, blank)
	}

	// And the two answer as different frames, one of which is the 40-byte
	// one A17 counts.
	_, conn := newTestRadio(t)
	unwritten := exchange(t, conn, "MA0050;")
	if len(unwritten) != 40 {
		t.Errorf("an unwritten channel answered %d bytes (%q), want the 40-byte blank frame (A4, A17)", len(unwritten), unwritten)
	}
	wire := slotWire(residualNameSlot)
	withResidue := exchange(t, conn, "MA0"+wire+";")
	if len(withResidue) <= 40 {
		t.Errorf("the residual-name fixture answered %q (%d bytes), want a longer frame carrying P13", withResidue, len(withResidue))
	}
	if !strings.HasPrefix(withResidue, "MA0"+wire+strings.Repeat(" ", 33)) {
		t.Errorf("the residual-name fixture answered %q, want P2 to P12 blank with the residue after them", withResidue)
	}
}

// TestDefaultImage_EveryLiveTwoValueByteAppearsWithBothPrintedValues, so that
// no driver behaviour on one of them is a fixture accident. The bytes are P4
// (890:3177-3178), P11 (890:3202-3203) and P12 (890:3206-3207); P5's four tone
// types are checked as a set for the same reason.
func TestDefaultImage_EveryLiveTwoValueByteAppearsWithBothPrintedValues(t *testing.T) {
	seen := map[string]map[byte]bool{"P4": {}, "P11": {}, "P12": {}, "P5": {}}
	for _, s := range DefaultImage() {
		if s == BlankRecord() || s.Freq == "           " {
			continue // the blank fixtures carry no legend value
		}
		seen["P4"][s.FMNarrow] = true
		seen["P11"][s.Split] = true
		seen["P12"][s.Lockout] = true
		seen["P5"][s.ToneType] = true
	}
	for _, p := range []string{"P4", "P11", "P12"} {
		for _, want := range []byte{'0', '1'} {
			if !seen[p][want] {
				t.Errorf("no channel in the default image carries %s = %q", p, string(want))
			}
		}
	}
	for _, want := range []byte{'0', '1', '2', '3'} {
		if !seen["P5"][want] {
			t.Errorf("no channel in the default image carries the tone type %q (890:3181-3185)", string(want))
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

// TestBlankRecord_IsEveryFieldBlank pins A6's reading in one place, since both
// the absent-channel answer and the A21 fixture are built from it.
func TestBlankRecord_IsEveryFieldBlank(t *testing.T) {
	b := BlankRecord()
	if b.Name != "" {
		t.Errorf("BlankRecord().Name = %q, want empty — the name window's absence is A4's reading", b.Name)
	}
	for _, f := range []struct {
		name  string
		value string
	}{
		{"Freq", b.Freq}, {"SplitFreq", b.SplitFreq},
		{"Mode", string(b.Mode)}, {"SplitMode", string(b.SplitMode)},
		{"FMNarrow", string(b.FMNarrow)}, {"SplitFMNarrow", string(b.SplitFMNarrow)},
		{"ToneType", string(b.ToneType)}, {"ToneNo", b.ToneNo}, {"CTCSSNo", b.CTCSSNo},
		{"Split", string(b.Split)}, {"Lockout", string(b.Lockout)},
	} {
		if strings.Trim(f.value, " ") != "" || f.value == "" {
			t.Errorf("BlankRecord().%s = %q, want its width in ASCII spaces (A6, 890:3215-3216)", f.name, f.value)
		}
	}
}
