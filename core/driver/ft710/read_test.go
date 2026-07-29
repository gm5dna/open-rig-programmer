// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// TestReadChannel_GoldenMappings drives ReadChannel against fakeradio's
// factory image (plus one overlaid tagged slot) and pins the full wire ->
// codeplug.ChannelData mapping, golden vector G4's M-01 first. Every
// expected value is written out literally, never computed by the code
// under test. CTCSSTone and ScanSkip must come back FieldState Unknown:
// the CAT protocol has no way to read them.
//
// Uses ImageUS, not the default ImageUK, specifically to keep 60m bank
// coverage (the "60m channel 501" case below): HW-CONFIRMED 2026-07-13
// (docs/hardware-notes.md §60m regional finding), the default ImageUK no
// longer synthesises a 5xx bank at all (real UK FT-710s have none) —
// ImageUS remains the STILL-ASSUMED fixture for exercising a populated
// 60m slot end-to-end through ReadChannel.
func TestReadChannel_GoldenMappings(t *testing.T) {
	tagged := fakeradio.MemState{
		Freq: "014250000", ClarSign: '-', ClarMag: "0150",
		RXClar: true, TXClar: false,
		Mode: '2', Kind: '1', CTCSS: '1', Shift: '1',
		Tag: "CALLING", TagDisplay: true,
		Populated: true,
	}
	_, sess := openSession(t, Simulated, fakeradio.WithFactoryImage(fakeradio.ImageUS), fakeradio.WithSlot("005", tagged))

	tests := []struct {
		name string
		slot string
		want codeplug.ChannelData
	}{
		{
			// Factory M-01: MR001007000000+000000110000; (golden vector G4).
			name: "M-01 golden 7.000000 MHz LSB",
			slot: "001",
			want: codeplug.ChannelData{
				FreqHz:     7_000_000,
				Mode:       "LSB",
				ClarHz:     0,
				RxClar:     false,
				TxClar:     false,
				CTCSS:      "OFF",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				Shift:      "SIMPLEX",
				Tag:        "",
				TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
		},
		{
			name: "populated with tag, clarifier, CTCSS, shift",
			slot: "005",
			want: codeplug.ChannelData{
				FreqHz:     14_250_000,
				Mode:       "USB",
				ClarHz:     -150,
				RxClar:     true,
				TxClar:     false,
				CTCSS:      "ENC-DEC",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				Shift:      "PLUS",
				Tag:        "CALLING",
				TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: true},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
		},
		{
			// Factory P1L: MRP1L001810000+000000150000; (golden vector G6).
			name: "PMS P1L golden 1.810000 MHz LSB (kind check passes for '5')",
			slot: "P1L",
			want: codeplug.ChannelData{
				FreqHz:     1_810_000,
				Mode:       "LSB",
				CTCSS:      "OFF",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				Shift:      "SIMPLEX",
				TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
		},
		{
			// Factory US 60m channel 501 (STILL-ASSUMED ImageUS
			// placeholder — HW-CONFIRMED 2026-07-13,
			// docs/hardware-notes.md §60m regional finding: the real UK
			// FT-710 has no 5xx bank at all, so this exercises ImageUS,
			// not a UK image): 5.260000 MHz USB, kind '1' (memory-like,
			// ASSUMED).
			name: "60m channel 501",
			slot: "501",
			want: codeplug.ChannelData{
				FreqHz:     5_260_000,
				Mode:       "USB",
				CTCSS:      "OFF",
				CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
				Shift:      "SIMPLEX",
				TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false},
				ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sess.ReadChannel(testCtx(t), tt.slot)
			if err != nil {
				t.Fatalf("ReadChannel(%q): unexpected error: %v", tt.slot, err)
			}
			if got.Slot != tt.slot {
				t.Errorf("Channel.Slot = %q, want %q", got.Slot, tt.slot)
			}
			if got.Empty() {
				t.Fatalf("ReadChannel(%q) = empty, want populated", tt.slot)
			}
			if *got.Data != tt.want {
				t.Errorf("ReadChannel(%q) data =\n%+v\nwant\n%+v", tt.slot, *got.Data, tt.want)
			}
		})
	}
}

// TestReadChannel_TagDisplayIsKnown is E1's read-direction PIN — a PIN, not
// a RED: it has been green since Task 1 flipped ChannelData.TagDisplay to a
// codeplug.BoolField and gave read.go's literal an explicit
// codeplug.Known, and it exists to keep it green.
//
// What it pins: the MT answer this channel is built from CARRIES the
// display flag (P1), so the value was genuinely READ from the radio, and
// ReadChannel must therefore report {Known, flag} — never Unknown, never
// Unavailable, and for BOTH flag values. The FALSE case is the one worth
// stating: Known-false and Unknown are exactly the pair a careless
// refactor conflates, since both render as "the tag is not displayed", yet
// only the first of them may be written back to a radio (buildWriteCommands
// refuses the other outright — see write_test.go's refusal pins). A
// regression here would not be loud: it would quietly turn every read
// channel into one the diff layer blocks.
//
// CTCSSTone and ScanSkip are asserted alongside as the deliberate CONTRAST:
// those two are genuinely unreadable over CAT and must stay Unknown, so the
// three fields together show that Unknown is a statement about the PROTOCOL
// here, not this driver's default.
func TestReadChannel_TagDisplayIsKnown(t *testing.T) {
	displayed := fakeradio.MemState{
		Freq: "014250000", ClarSign: '+', ClarMag: "0000",
		Mode: '2', Kind: '1', CTCSS: '0', Shift: '0',
		Tag: "SHOWN", TagDisplay: true,
		Populated: true,
	}
	hidden := displayed
	hidden.Tag, hidden.TagDisplay = "HIDDEN", false

	_, sess := openSession(t, Simulated,
		fakeradio.WithSlot("011", displayed),
		fakeradio.WithSlot("012", hidden),
	)

	for _, tt := range []struct {
		name string
		slot string
		want bool
	}{
		{"wire display flag set", "011", true},
		{"wire display flag clear", "012", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sess.ReadChannel(testCtx(t), tt.slot)
			if err != nil {
				t.Fatalf("ReadChannel(%q): unexpected error: %v", tt.slot, err)
			}
			if got.Empty() {
				t.Fatalf("ReadChannel(%q) = empty, want populated", tt.slot)
			}
			want := codeplug.BoolField{State: codeplug.Known, Value: tt.want}
			if got.Data.TagDisplay != want {
				t.Errorf("TagDisplay = %+v, want %+v (the MT answer carries P1, so the value is READ, not assumed)", got.Data.TagDisplay, want)
			}
			if got.Data.CTCSSTone.State != codeplug.Unknown {
				t.Errorf("CTCSSTone.State = %q, want %q (unreadable over CAT)", got.Data.CTCSSTone.State, codeplug.Unknown)
			}
			if got.Data.ScanSkip.State != codeplug.Unknown {
				t.Errorf("ScanSkip.State = %q, want %q (unreadable over CAT)", got.Data.ScanSkip.State, codeplug.Unknown)
			}
		})
	}
}

// TestReadChannel_EmptySlot: fakeradio answers "?;" for an MR read of an
// unpopulated slot (its ASSUMED register, item 2); the driver must map
// that — and ONLY that — to an EMPTY channel, never an error.
func TestReadChannel_EmptySlot(t *testing.T) {
	_, sess := openSession(t, Simulated) // ImageUK: "007" is not populated

	got, err := sess.ReadChannel(testCtx(t), "007")
	if err != nil {
		t.Fatalf("ReadChannel(empty slot): unexpected error: %v", err)
	}
	if !got.Empty() {
		t.Errorf("ReadChannel(empty slot) = populated (%+v), want empty", got.Data)
	}
	if got.Slot != "007" {
		t.Errorf("Channel.Slot = %q, want \"007\"", got.Slot)
	}
}

// TestReadChannel_KindMismatch: a memory slot whose MR answer claims PMS
// kind ('5') contradicts its own slot number — the driver's kind sanity
// check must refuse it with the typed error rather than mapping it. '5'
// is outside acceptedKinds' lenient memory-bank set ({'0','1','4'}), so
// this remains a genuine mismatch even after the M5b read-leniency fix.
func TestReadChannel_KindMismatch(t *testing.T) {
	lying := fakeradio.MemState{
		Freq: "014000000", ClarSign: '+', ClarMag: "0000",
		Mode: '2', Kind: '5', CTCSS: '0', Shift: '0',
		Populated: true,
	}
	_, sess := openSession(t, Simulated, fakeradio.WithSlot("020", lying))

	_, err := sess.ReadChannel(testCtx(t), "020")
	if !errors.Is(err, ErrKindMismatch) {
		t.Fatalf("ReadChannel(kind-lying slot) = %v, want errors.Is match against ErrKindMismatch", err)
	}
	var kme *KindMismatchError
	if !errors.As(err, &kme) {
		t.Fatalf("error %v is not a *KindMismatchError", err)
	}
	wantSet := []byte{'0', '1', '4'}
	if kme.Slot != "020" || kme.Got != '5' || !bytesEqual(kme.Want, wantSet) {
		t.Errorf("KindMismatchError = %+v, want Slot \"020\", Got '5', Want %q", kme, wantSet)
	}
}

// bytesEqual reports whether a and b hold the same bytes in the same
// order.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestReadChannel_HWDerived_M5b_PMSKindLeniency reproduces the M5b live
// read bug and its fix (HW-CONFIRMED 2026-07-13, docs/hardware-notes.md):
// a populated PMS slot's MR answer carries kind '1' (KindMemory, as
// CAT-written), not kind '5' — the live failure frame is
// `MRP1L007100000+000000110000;` (decoded: slot P1L, 7.100000 MHz,
// simplex, LSB, kind '1', CTCSS off). Before this task's fix,
// ReadChannel aborted this exact exchange with *KindMismatchError
// ("carries kind '1', want '5' for this slot's bank") — reproduced live
// against Stuart's radio and, in this repo, by
// TestWriteChannel_HappyPath's PMS case and app/send_test.go's
// PrepareSend/ConfirmSend flows before this fix landed. It must now
// succeed and map the channel normally.
func TestReadChannel_HWDerived_M5b_PMSKindLeniency(t *testing.T) {
	liveP1L := fakeradio.MemState{
		Freq: "007100000", ClarSign: '+', ClarMag: "0000",
		RXClar: false, TXClar: false,
		Mode: '1', Kind: '1', CTCSS: '0', Shift: '0',
		Populated: true,
	}
	_, sess := openSession(t, Simulated, fakeradio.WithSlot("P1L", liveP1L))

	got, err := sess.ReadChannel(testCtx(t), "P1L")
	if err != nil {
		t.Fatalf("ReadChannel(P1L, live HW-derived kind '1'): unexpected error: %v", err)
	}
	if got.Empty() {
		t.Fatal("ReadChannel(P1L) = empty, want populated")
	}
	want := codeplug.ChannelData{
		FreqHz:     7_100_000,
		Mode:       "LSB",
		CTCSS:      "OFF",
		CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
		Shift:      "SIMPLEX",
		TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: false},
		ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
	}
	if *got.Data != want {
		t.Errorf("ReadChannel(P1L) data =\n%+v\nwant\n%+v", *got.Data, want)
	}
}

// TestReadChannel_60mEMG_AcceptsKindMemory is Codex M5b fix wave, Fix 5
// (adjudicated MEDIUM): a discovered 60m/EMG slot's MR answer carrying
// kind '1' (KindMemory — the only value ever actually observed for these
// banks; ASSUMED, doc.go register item 4, never HW-probed) must map
// normally, on both bank kinds.
func TestReadChannel_60mEMG_AcceptsKindMemory(t *testing.T) {
	for _, slot := range []string{"501", "EMG"} {
		t.Run(slot, func(t *testing.T) {
			st := fakeradio.MemState{
				Freq: "005166500", ClarSign: '+', ClarMag: "0000",
				Mode: '4', Kind: '1', CTCSS: '0', Shift: '0',
				Populated: true,
			}
			_, sess := openSession(t, Simulated, fakeradio.WithSlot(slot, st))

			got, err := sess.ReadChannel(testCtx(t), slot)
			if err != nil {
				t.Fatalf("ReadChannel(%q, kind '1'): unexpected error: %v", slot, err)
			}
			if got.Empty() {
				t.Fatalf("ReadChannel(%q) = empty, want populated", slot)
			}
		})
	}
}

// TestReadChannel_60mEMG_RejectsOtherKinds is Fix 5's main case: before
// this fix, acceptedKinds applied MEM's LENIENT {'0','1','4'} set to
// EVERY non-PMS slot, including discovered 60m/EMG banks — even though
// doc.go's own register (item 4) and fakeradio's own modelling
// (kind60mEMG) have always said these banks are expected to answer ONLY
// kind '1', never characterised beyond that. Kind '0' (VFO) and '4'
// (the documented "-"/unset placeholder) — both legitimately lenient on
// a MEM slot — must now be REJECTED on 501/EMG: a value outside {'1'}
// there means the radio and this driver disagree about what the slot
// IS, exactly as strict kind-checking would have caught before M5b's
// MEM/PMS leniency existed at all.
func TestReadChannel_60mEMG_RejectsOtherKinds(t *testing.T) {
	for _, slot := range []string{"501", "EMG"} {
		for _, kind := range []byte{'0', '4'} {
			t.Run(fmt.Sprintf("%s/kind_%c", slot, kind), func(t *testing.T) {
				st := fakeradio.MemState{
					Freq: "005166500", ClarSign: '+', ClarMag: "0000",
					Mode: '4', Kind: kind, CTCSS: '0', Shift: '0',
					Populated: true,
				}
				_, sess := openSession(t, Simulated, fakeradio.WithSlot(slot, st))

				_, err := sess.ReadChannel(testCtx(t), slot)
				if !errors.Is(err, ErrKindMismatch) {
					t.Fatalf("ReadChannel(%q, kind %q) = %v, want errors.Is match against ErrKindMismatch", slot, kind, err)
				}
				var kme *KindMismatchError
				if !errors.As(err, &kme) {
					t.Fatalf("error %v is not a *KindMismatchError", err)
				}
				wantSet := []byte{cat.KindMemory}
				if kme.Got != kind || !bytesEqual(kme.Want, wantSet) {
					t.Errorf("KindMismatchError = %+v, want Got %q, Want %q", kme, kind, wantSet)
				}
			})
		}
	}
}

// TestReadChannel_InvalidSlot: slot strings the wire grammar rejects (or
// that are answer-only) must fail before any wire traffic, with an error.
func TestReadChannel_InvalidSlot(t *testing.T) {
	cp, sess := openCountingSession(t, Simulated)

	baseline := cp.writes.Load()
	for _, slot := range []string{"", "XYZ", "100", "000", "p1l"} {
		if _, err := sess.ReadChannel(testCtx(t), slot); err == nil {
			t.Errorf("ReadChannel(%q) = nil error, want a rejection", slot)
		}
	}
	if got := cp.writes.Load(); got != baseline {
		t.Errorf("invalid-slot reads produced %d wire writes, want 0", got-baseline)
	}
}
