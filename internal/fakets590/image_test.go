// SPDX-License-Identifier: GPL-3.0-or-later

package fakets590

import (
	"testing"
)

// printedExampleFrequency is the ONE frequency either book prints as a worked
// value: "For example, enter 00014195000 for 14.195 MHz." (590:965). Written
// here as a literal rather than taken from image.go, so a drift in the
// package constant fails this test rather than being agreed with.
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
	modes := map[byte]bool{ // 590:1353-1363, the MD legend
		'0': true, '1': true, '2': true, '3': true, '4': true,
		'5': true, '6': true, '7': true, '8': true, '9': true,
	}
	dataModes := map[byte]bool{'0': true, '1': true} // 590:447-449
	toneModes := map[byte]bool{                      // 590:1464-1467
		'0': true, '1': true, '2': true, '3': true,
	}
	toneIndices := map[string]bool{"00": true}         // 590:2296, the first TN row
	filters := map[byte]bool{'0': true, '1': true}     // 590:1476-1477
	fmFlags := map[string]bool{"00": true, "01": true} // 590:1484-1485
	lockouts := map[byte]bool{'0': true, '1': true}    // 590:1487-1488

	img := DefaultImage()
	if len(img) == 0 {
		t.Fatal("DefaultImage is empty — every assertion below would pass vacuously")
	}
	for key, rec := range img {
		name := key.String()
		if rec.Freq != printedExampleFrequency {
			t.Errorf("%s: frequency %q is not the book's printed example %q — no other frequency is printed anywhere in this document", name, rec.Freq, printedExampleFrequency)
		}
		if !modes[rec.Mode] {
			t.Errorf("%s: mode nibble %q is not in the MD legend (590:1353-1363)", name, rec.Mode)
		}
		if !dataModes[rec.DataMode] {
			t.Errorf("%s: data mode %q is not in the DA legend (590:447-449)", name, rec.DataMode)
		}
		if !toneModes[rec.ToneMode] {
			t.Errorf("%s: tone mode %q is not in P7's legend (590:1464-1467)", name, rec.ToneMode)
		}
		if !toneIndices[rec.ToneNo] {
			t.Errorf("%s: tone index %q is not a shipped printed TN value", name, rec.ToneNo)
		}
		if !toneIndices[rec.CTCSSNo] {
			t.Errorf("%s: CTCSS index %q is not a shipped printed CN value", name, rec.CTCSSNo)
		}
		if !filters[rec.Filter] {
			t.Errorf("%s: byte 28 %q is not in P11's legend (590:1476-1477)", name, rec.Filter)
		}
		if !fmFlags[rec.FMNarrow] {
			t.Errorf("%s: P14 %q is not in its legend (590:1484-1485)", name, rec.FMNarrow)
		}
		if !lockouts[rec.Lockout] {
			t.Errorf("%s: P15 %q is not in its legend (590:1487-1488)", name, rec.Lockout)
		}
		if rec.Name != blankName {
			t.Errorf("%s: name %q is not the eight-space blank the empty-channel sentence describes (590:1492-1493)", name, rec.Name)
		}
	}
}

// TestDefaultImage_CarriesBothValuesOfEveryLiveTwoValueByte. An image whose
// every record answered the same byte would make a driver's handling of the
// other value untested and the agreement a fixture accident. Byte 28 is the
// axis the two rows differ on, and P14 and P15 are the other two live
// two-value fields in the record.
func TestDefaultImage_CarriesBothValuesOfEveryLiveTwoValueByte(t *testing.T) {
	img := DefaultImage()
	seen := func(get func(MemState) byte) map[byte]bool {
		out := map[byte]bool{}
		for _, rec := range img {
			out[get(rec)] = true
		}
		return out
	}
	for _, tt := range []struct {
		field string
		get   func(MemState) byte
	}{
		{"byte 28, FILTER A/B (590:1476-1477)", func(m MemState) byte { return m.Filter }},
		{"P15, channel lockout (590:1487-1488)", func(m MemState) byte { return m.Lockout }},
		{"P6, DATA mode (590:447-449)", func(m MemState) byte { return m.DataMode }},
	} {
		got := seen(tt.get)
		if !got['0'] || !got['1'] {
			t.Errorf("%s: the default image carries only %v — both printed values must appear", tt.field, got)
		}
	}

	fm := map[string]bool{}
	for _, rec := range img {
		fm[rec.FMNarrow] = true
	}
	if !fm["00"] || !fm["01"] {
		t.Errorf("P14 (590:1484-1485): the default image carries only %v — both printed values must appear", fm)
	}
}

// TestDefaultImage_PopulatesBothHalvesOfOneSectionChannel. The driver reads a
// section-defined channel as a PAIR — P1=0 for the start frequency and P1=1
// for the end (590:1449-1451) — so a default image with only one half would
// leave half of that read path exercised against a silence.
func TestDefaultImage_PopulatesBothHalvesOfOneSectionChannel(t *testing.T) {
	img := DefaultImage()
	for _, half := range []Half{HalfRXOrStart, HalfTXOrEnd} {
		if _, ok := img[recordKey{channel: 100, half: half}]; !ok {
			t.Errorf("the default image has no record for channel 100 half %q", byte(half))
		}
	}
}

// TestDefaultImage_ShipsNoExtensionChannel. Stuart ruled on 05/09/2026 that
// the TS-590SG's 110-119 are omitted from the published banks until A11 is
// lifted, and the fake ships no image for them: there is no slot ID left for
// such an image to represent. doc.go's register entry THE SG'S EXTENSION
// CHANNELS ARE NOT SERVED.
func TestDefaultImage_ShipsNoExtensionChannel(t *testing.T) {
	for key := range DefaultImage() {
		if key.channel >= 110 {
			t.Errorf("the default image carries channel %d — 110-119 are deliberately not served", key.channel)
		}
	}
}

// TestDefaultImage_LeavesMostOfTheMemoryBankEmpty. A fake that shipped a
// hundred populated channels would turn every "this channel is empty"
// assertion into a fixture accident, and nothing about an unwritten
// TS-590 channel's contents is printed beyond the empty-channel marker
// itself (590:1492-1493).
func TestDefaultImage_LeavesMostOfTheMemoryBankEmpty(t *testing.T) {
	img := DefaultImage()
	if len(img) > 8 {
		t.Errorf("the default image holds %d records — it is meant to be minimal, and each one is a printed-value composition that A27 already carries", len(img))
	}
	populated := 0
	for key := range img {
		if key.channel < 100 {
			populated++
		}
	}
	if populated == 0 {
		t.Error("no ordinary memory channel is populated — the fleet's read-every-registered-model pins would be vacuous against this radio")
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
	r1, conn1 := newTestRadio(t, RowSG, WithFactoryImage(img))
	r2, _ := newTestRadio(t, RowSG, WithFactoryImage(img))

	f := newRecordFrame("MW", '0', '0', "07")
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
		Freq: "00010100000", Mode: 'B', DataMode: '0', ToneMode: '0',
		ToneNo: "00", CTCSSNo: "00", Filter: '0', FMNarrow: "00",
		Lockout: '0', Name: "TESTNAME",
	}
	r, conn := newTestRadio(t, RowSG, WithChannel(9, rec))

	got, ok := r.ChannelState(9, HalfRXOrStart)
	if !ok || got != rec {
		t.Fatalf("ChannelState(9) = %+v, %v; want the overlaid record", got, ok)
	}
	// It reaches the wire verbatim, mode nibble 'B' and all — a nibble the
	// MD legend does not print, which is exactly what a parse-error test
	// needs and what a validating option could not provide.
	answer := exchange(t, conn, "MR0 09;")
	if answer[17] != 'B' {
		t.Errorf("MR0 09; -> byte 18 is %q, want the overlaid 'B'", answer[17])
	}
	if name := answer[41:49]; name != "TESTNAME" {
		t.Errorf("MR0 09; -> name field %q, want %q", name, "TESTNAME")
	}
}

// TestWithSplitChannel_StoresBothHalves. The two frames are the split
// channel's receive and transmit frequencies on an ordinary memory
// (590:1444-1447) and the section channel's start and end on 100-109
// (590:1449-1451) — one option, because it is one pair of frames.
func TestWithSplitChannel_StoresBothHalves(t *testing.T) {
	rx := defaultRecord()
	rx.Freq = "00007100000"
	tx := defaultRecord()
	tx.Freq = "00007200000"

	r, conn := newTestRadio(t, RowSG, WithSplitChannel(9, rx, tx))

	if got := exchange(t, conn, "MR0 09;")[6:17]; got != "00007100000" {
		t.Errorf("MR0 09; -> frequency %q, want the receive half", got)
	}
	if got := exchange(t, conn, "MR1 09;")[6:17]; got != "00007200000" {
		t.Errorf("MR1 09; -> frequency %q, want the transmit half", got)
	}
	if _, ok := r.ChannelState(9, HalfTXOrEnd); !ok {
		t.Error("the transmit half is not in the record map")
	}
}

// TestWithEmptyChannel_RemovesBothHalves. It removes map entries, which
// triggers the fake's existing documented empty-channel answer
// (590:1492-1493) rather than introducing any new behaviour — the
// WithEXUnavailable shape, one radio family over.
func TestWithEmptyChannel_RemovesBothHalves(t *testing.T) {
	r, conn := newTestRadio(t, RowSG, WithEmptyChannel(100))

	for _, key := range []recordKey{
		{channel: 100, half: HalfRXOrStart},
		{channel: 100, half: HalfTXOrEnd},
	} {
		if _, ok := r.ChannelState(key.channel, key.half); ok {
			t.Errorf("%s is still in the record map", key.String())
		}
	}
	for _, p1 := range []string{"0", "1"} {
		got := exchange(t, conn, "MR"+p1+"100;")
		if freq := got[6:17]; freq != "00000000000" {
			t.Errorf("MR%s100; -> frequency %q, want the empty record's zeros", p1, freq)
		}
	}
}
