// SPDX-License-Identifier: GPL-3.0-or-later

package fakets480

import (
	"testing"
)

// printedExampleFrequency is the worked frequency this book prints, "For
// example, 00014195000 for 14.195 MHz." (480:549, and again at 480:572 and
// 480:1746). Written here as a literal rather than taken from image.go, so a
// drift in the package constant fails this test rather than being agreed
// with.
const printedExampleFrequency = "00014195000"

// TestDefaultImage_EveryByteIsAPrintedValue is the evidence posture, made
// mechanical. Every byte of every default record must be one of exactly four
// things — a printed constant from the position table, the printed example
// frequency, a value printed in one of the referenced legends, or the
// eight-space name — and the SHIPPED SET of values is small enough to list
// here in full. See PROVENANCE.md for the claim this supports and for A27,
// which is what makes the composition itself an assumption.
func TestDefaultImage_EveryByteIsAPrintedValue(t *testing.T) {
	// The printed legend values, each with the line that prints it.
	modes := map[byte]bool{ // 480:843-854, the MD legend
		'0': true, '1': true, '2': true, '3': true, '4': true,
		'5': true, '6': true, '7': true, '8': true, '9': true,
	}
	lockouts := map[byte]bool{'0': true, '1': true} // 480:920
	toneModes := map[byte]bool{                     // 480:922 — THREE values
		'0': true, '1': true, '2': true,
	}
	toneIndices := map[string]bool{"00": true} // 480:1557, 480:337, the first row of each chart
	// ST's two mode-conditional legends, 00 ~ 04 for SSB/CW/FSK and
	// 00 ~ 09 for AM/FM (480:1494-1500). The shipped set is the indices
	// printed in BOTH, so no record's step depends on which class its own
	// mode nibble falls in — the ambiguity that is the design's A22.
	steps := map[string]bool{"00": true, "01": true, "02": true, "03": true, "04": true}

	img := DefaultImage()
	if len(img) == 0 {
		t.Fatal("DefaultImage is empty — every assertion below would pass vacuously")
	}
	for key, rec := range img {
		name := key.String()
		if rec.Freq != printedExampleFrequency {
			t.Errorf("%s: frequency %q is not the book's printed example %q", name, rec.Freq, printedExampleFrequency)
		}
		if !modes[rec.Mode] {
			t.Errorf("%s: mode nibble %q is not in the MD legend (480:843-854)", name, rec.Mode)
		}
		if !lockouts[rec.Lockout] {
			t.Errorf("%s: P6 %q is not in the lockout legend (480:920)", name, rec.Lockout)
		}
		if !toneModes[rec.ToneMode] {
			t.Errorf("%s: tone mode %q is not in P7's THREE-value legend (480:922)", name, rec.ToneMode)
		}
		if !toneIndices[rec.ToneNo] {
			t.Errorf("%s: tone index %q is not a shipped printed TN value", name, rec.ToneNo)
		}
		if !toneIndices[rec.CTCSSNo] {
			t.Errorf("%s: CTCSS index %q is not a shipped printed CN value", name, rec.CTCSSNo)
		}
		if !steps[rec.Step] {
			t.Errorf("%s: P14 %q is not an index printed in BOTH of ST's mode-conditional legends (480:1494-1500)", name, rec.Step)
		}
		if rec.Name != blankName {
			t.Errorf("%s: name %q is not the eight-space blank (the design's A3, unlifted on this row)", name, rec.Name)
		}
	}
}

// TestDefaultImage_CarriesEveryPrintedValueOfTheLiveLegends. An image whose
// every record answered the same byte would make a driver's handling of the
// other values untested and the agreement a fixture accident. The live
// non-frequency legends in a TS-480 record are the lockout's two values
// (480:920) and the tone mode's three (480:922) — where the 590 pair have a
// FILTER byte, a DATA-mode flag and an FM bandwidth flag instead.
func TestDefaultImage_CarriesEveryPrintedValueOfTheLiveLegends(t *testing.T) {
	img := DefaultImage()

	lockouts := map[byte]bool{}
	toneModes := map[byte]bool{}
	steps := map[string]bool{}
	modes := map[byte]bool{}
	for _, rec := range img {
		lockouts[rec.Lockout] = true
		toneModes[rec.ToneMode] = true
		steps[rec.Step] = true
		modes[rec.Mode] = true
	}

	for _, want := range []byte{'0', '1'} {
		if !lockouts[want] {
			t.Errorf("P6 (480:920): the default image never carries %q — both printed values must appear", want)
		}
	}
	for _, want := range []byte{'0', '1', '2'} {
		if !toneModes[want] {
			t.Errorf("P7 (480:922): the default image never carries %q — all THREE printed values must appear", want)
		}
	}
	if len(steps) < 2 {
		t.Errorf("P14 (480:979): the default image carries only %v — a single step index would make any step handling a fixture accident", steps)
	}
	if len(modes) < 2 {
		t.Errorf("P5 (480:843-854): the default image carries only %v — one mode nibble is not enough to tell a mode-dependent bug from a fixture", modes)
	}
}

// TestDefaultImage_ShipsOnlyTheHalfTheRowPublishes. Decision 15 gives this
// row ONE FLAT MEM BANK of 00-99 and no scan bank: the P1=1 half of a channel
// is not a published slot on the TS-480, and P19 ships no image for a slot the
// row does not publish. The upper half of 90-99 — the printed END frequency
// (480:943-944) — is therefore served EMPTY, which is a posture and not a
// claim about the radio.
func TestDefaultImage_ShipsOnlyTheHalfTheRowPublishes(t *testing.T) {
	for key := range DefaultImage() {
		if key.half != HalfRXOrStart {
			t.Errorf("the default image carries %s — only the P1=0 half is a published slot on this row", key.String())
		}
	}
}

// TestDefaultImage_LeavesMostOfTheMemoryBankEmpty. A fake that shipped a
// hundred populated channels would turn every "this channel is empty"
// assertion into a fixture accident — and on this row that assertion is A4,
// the release gate itself, so the image must leave room for it.
func TestDefaultImage_LeavesMostOfTheMemoryBankEmpty(t *testing.T) {
	img := DefaultImage()
	if len(img) > 8 {
		t.Errorf("the default image holds %d records — it is meant to be minimal, and each one is a printed-value composition that A27 already carries", len(img))
	}
	if len(img) == 0 {
		t.Error("no channel is populated — the fleet's read-every-registered-model pins would be vacuous against this radio")
	}
}

// TestDefaultImage_EachCallIsIndependent: two radios built from one Image
// value must not share mutable state, or a write to one would show up in the
// other.
func TestDefaultImage_EachCallIsIndependent(t *testing.T) {
	a, b := DefaultImage(), DefaultImage()
	key := recordKey{channel: 0, half: HalfRXOrStart}
	rec := a[key]
	rec.Freq = "00000000001"
	a[key] = rec
	if b[key].Freq != printedExampleFrequency {
		t.Error("mutating one DefaultImage() result changed another — the maps are shared")
	}
}

// TestTwoRadiosFromOneImageDoNotAlias pins what a shared map would actually
// break.
func TestTwoRadiosFromOneImageDoNotAlias(t *testing.T) {
	img := Image(DefaultImage)
	r1, conn1 := newTestRadio(t, WithFactoryImage(img))
	r2, _ := newTestRadio(t, WithFactoryImage(img))

	f := newRecordFrame("MW", '0', "07")
	f.freq = "00007100000"
	writeFrame(t, conn1, f.String())
	assertNoReply(t, conn1)

	if _, ok := r1.ChannelState(7, HalfRXOrStart); !ok {
		t.Fatal("the write did not reach the first radio")
	}
	if _, ok := r2.ChannelState(7, HalfRXOrStart); ok {
		t.Error("the write reached the SECOND radio — the two share a record map")
	}
}

// --- The record-shaping options ---

// TestWithChannel_OverlaysOneRecord: overlay semantics, like every sibling
// fake's WithSlot. No validation is applied, so a test may craft a record
// whose ANSWER is deliberately malformed and drive a real driver's parse
// error through a real fake rather than through a scripted transcript.
func TestWithChannel_OverlaysOneRecord(t *testing.T) {
	rec := MemState{
		Freq: "00010100000", Mode: 'B', Lockout: '0', ToneMode: '0',
		ToneNo: "00", CTCSSNo: "00", Step: "00", Name: "TESTNAME",
	}
	r, conn := newTestRadio(t, WithChannel(9, rec))

	got, ok := r.ChannelState(9, HalfRXOrStart)
	if !ok || got != rec {
		t.Fatalf("ChannelState(9) = %+v, %v; want the overlaid record", got, ok)
	}
	// It reaches the wire verbatim, mode nibble 'B' and all — a nibble the
	// MD legend does not print, which is exactly what a parse-error test
	// needs and what a validating option could not provide.
	answer := exchange(t, conn, "MR0009;")
	if answer[17] != 'B' {
		t.Errorf("MR0009; -> byte 18 is %q, want the overlaid 'B'", answer[17])
	}
	if name := answer[41:49]; name != "TESTNAME" {
		t.Errorf("MR0009; -> name field %q, want %q", name, "TESTNAME")
	}
}

// TestWithEmptyChannel_RemovesBothHalves. It removes map entries, which
// triggers the fake's existing zero-record answer rather than introducing any
// new behaviour — the test-only seam for forcing a channel the default image
// populates to read back empty, so that a driver's empty-channel handling can
// be pinned against a channel a test names. What that answer PROVES about a
// TS-480 is A4's business, and A4 is unlifted.
func TestWithEmptyChannel_RemovesBothHalves(t *testing.T) {
	r, conn := newTestRadio(t, WithEmptyChannel(0))

	for _, half := range []Half{HalfRXOrStart, HalfTXOrEnd} {
		if _, ok := r.ChannelState(0, half); ok {
			t.Errorf("%s is still in the record map", recordKey{channel: 0, half: half}.String())
		}
	}
	for _, p1 := range []string{"0", "1"} {
		got := exchange(t, conn, "MR"+p1+"000;")
		if freq := got[6:17]; freq != "00000000000" {
			t.Errorf("MR%s000; -> frequency %q, want the zero record's zeros", p1, freq)
		}
	}
}
