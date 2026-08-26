// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// oneRecord builds an image holding exactly one populated channel, at the
// given wire address, carrying the golden record at the given frequency
// width.
func oneRecord(addr wireAddr, freqHz uint64, freqBytes int) radioImage {
	return radioImage{
		idToken: testToken,
		records: map[wireAddr][]byte{addr: goldenRecord(freqHz, freqBytes).build()},
	}
}

// TestProbe_BothDeclaredLengthsConfirm.
//
// A record of 64 bytes confirms. So does one of 65: THE ACCEPTED SET IS
// THE FINGERPRINT (spec D3.2), and this model's set has two members
// because its frequency field is documented at two widths (matrix §3.11
// Condition B, ASSUMED, lift ic905-R-06). The probe is TOLERANT OF EITHER
// until hardware settles it.
func TestProbe_BothDeclaredLengthsConfirm(t *testing.T) {
	for _, tt := range []struct {
		name      string
		freqHz    uint64
		freqBytes int
		wantLen   int
	}{
		{"the 64-byte shape the diagram draws", 144_500_000, 5, 64},
		{"the 65-byte 10 GHz shape", 10_250_000_000, 6, 65},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, s := openFor(t, oneRecord(wireAddr{0, 0}, tt.freqHz, tt.freqBytes))
			d := s.Diagnostics905()
			if !d.Fingerprinted {
				t.Fatal("Fingerprinted is false — a record at a DECLARED length is what confirms this radio")
			}
			if d.ObservedRecordLength != tt.wantLen {
				t.Errorf("ObservedRecordLength = %d, want %d", d.ObservedRecordLength, tt.wantLen)
			}
		})
	}
}

// TestProbe_AKnownSiblingLengthIsAWrongRadioWithProvisionalAttribution.
//
// TWO DIFFERENT BRANCHES, and REV 1 conflated them (Codex 8, Fable
// F11(c)). Spec D3.2 distinguishes:
//
//	(a) a length equal to a DIFFERENT REGISTERED MODEL's accepted set ->
//	    WrongRadioError with PROVISIONAL found-model attribution.
//	(b) any OTHER length -> refusal WITHOUT attribution.
//
// BRANCH (a) IS EXERCISED THROUGH A SEAM, and the seam is why a Wave-3
// driver can test it at all: this worktree does not know any other
// model's accepted set — cross-model distinctness is a Wave-4 tier check
// and this driver claims none — so the table is an Option, EMPTY BY
// DEFAULT, which Wave 4 populates from the registry in the same commit
// that registers the models.
//
// THE WORD "provisional" COMES FROM THIS DRIVER'S OWN WRAPPER, NOT FROM
// core/driver: WrongRadioError.Error()'s two formats are fixed there and
// the ID-only one is BASELINE-PINNED, so neither may be edited to add it.
// The test asserts errors.As on the wrapped chain AND the word in the
// outer message, so a future unwrapping breaks it.
func TestProbe_AKnownSiblingLengthIsAWrongRadioWithProvisionalAttribution(t *testing.T) {
	img := radioImage{
		idToken: testToken,
		// A 39-byte record: not this model's, and declared here as some
		// other model's. The number is the TEST's, not a claim.
		records: map[wireAddr][]byte{{0, 0}: make([]byte, 39)},
	}
	p := newRespondingPort(t, img)
	_, err := New(RealHardware, WithSiblingRecordLengths(SiblingLengths{39: "IC-7300"})).
		Open(context.Background(), p.Port(), driver.Identity{})
	if err == nil {
		t.Fatal("Open succeeded against a radio answering a foreign record length")
	}

	var wre *driver.WrongRadioError
	if !errors.As(err, &wre) {
		t.Fatalf("error = %v, want a *driver.WrongRadioError in the chain", err)
	}
	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Errorf("error = %v, does not satisfy errors.Is(err, driver.ErrWrongRadio)", err)
	}
	if wre.GotModel != "IC-7300" {
		t.Errorf("GotModel = %q, want %q — branch (a) ATTRIBUTES the length", wre.GotModel, "IC-7300")
	}
	if wre.WantModel != "IC-905" {
		t.Errorf("WantModel = %q, want %q — the named format renders only when BOTH are set", wre.WantModel, "IC-905")
	}
	if !strings.Contains(err.Error(), "PROVISIONAL") {
		t.Errorf("error = %q, want it to say the attribution is PROVISIONAL — the record lengths this tier compares are themselves ASSUMED derivations, never captured from a radio", err)
	}
}

// TestProbe_AnUnknownLengthIsRefusedWithoutAttribution is branch (b),
// which is what a Wave-3 driver with no sibling table takes for EVERY
// unrecognised length.
//
// WrongRadioError's model fields are OPTIONAL, and empty is the honest
// value for a driver that cannot name what it found.
func TestProbe_AnUnknownLengthIsRefusedWithoutAttribution(t *testing.T) {
	img := radioImage{
		idToken: testToken,
		records: map[wireAddr][]byte{{0, 0}: make([]byte, 63)},
	}
	p := newRespondingPort(t, img)
	_, err := New(RealHardware).Open(context.Background(), p.Port(), driver.Identity{})
	if err == nil {
		t.Fatal("Open succeeded against a radio answering a 63-byte record")
	}

	var wre *driver.WrongRadioError
	if !errors.As(err, &wre) {
		t.Fatalf("error = %v, want a *driver.WrongRadioError in the chain", err)
	}
	if wre.WantModel != "" || wre.GotModel != "" {
		t.Errorf("WantModel = %q, GotModel = %q — both must be EMPTY: this driver cannot name what answered", wre.WantModel, wre.GotModel)
	}
	if strings.Contains(err.Error(), "PROVISIONAL") {
		t.Errorf("error = %q — branch (b) attributes nothing, so it has no attribution to qualify", err)
	}
}

// TestProbe_TheSiblingTableIsEmptyInWaveThree, so nobody ships a table by
// accident. With no table, EVERY unrecognised length takes branch (b).
func TestProbe_TheSiblingTableIsEmptyInWaveThree(t *testing.T) {
	d, ok := New(RealHardware).(*ic905Driver)
	if !ok {
		t.Fatalf("New returned %T", New(RealHardware))
	}
	if len(d.siblingLengths) != 0 {
		t.Errorf("the default sibling table holds %d entries, want none — cross-model record-length distinctness is a Wave-4 tier check and this plan claims none", len(d.siblingLengths))
	}
}

// TestProbe_AnEmptyRadioOpensUnfingerprinted.
//
// All-FA over the bounded search is an EMPTY radio, not a wrong one: the
// session opens on ADDRESS EVIDENCE ALONE (spec D3.2, matrix §3.8(a)) and
// records an UNFINGERPRINTED diagnostic. That the FA answer itself means
// "empty" is ASSUMED — D5 entry 2(a), lift ic905-R-14.
func TestProbe_AnEmptyRadioOpensUnfingerprinted(t *testing.T) {
	_, s := openFor(t, radioImage{idToken: testToken})

	d := s.Diagnostics905()
	if d.Fingerprinted {
		t.Error("Fingerprinted is true on a radio that answered FA to every probed slot")
	}
	if d.ObservedRecordLength != 0 {
		t.Errorf("ObservedRecordLength = %d, want 0 when nothing was ever observed", d.ObservedRecordLength)
	}
	if s.Identity().CATID == "" {
		t.Error("the session did not open — an empty radio is not a wrong radio")
	}
}

// TestProbe_IsBoundedAndStopsAtTheFirstRecord.
//
// The bound is stated so an EMPTY radio's open cannot take ten thousand
// reads: group 0's first sixteen channels, then the twelve CALL slots.
// The FULL inventory walk is a different question and is Task 13's.
func TestProbe_IsBoundedAndStopsAtTheFirstRecord(t *testing.T) {
	t.Run("an empty radio is probed exactly probeBound times", func(t *testing.T) {
		p, _ := openFor(t, radioImage{idToken: testToken})
		if got, want := countMemoryReads(p), probeSlotsInGroupZero+callSlotsProbed; got != want {
			t.Errorf("the probe made %d memory reads, want exactly %d (%d in group 0, then the %d CALL slots)", got, want, probeSlotsInGroupZero, callSlotsProbed)
		}
	})
	t.Run("the first record stops the search", func(t *testing.T) {
		p, _ := openFor(t, oneRecord(wireAddr{0, 0}, 144_500_000, 5))
		if got := countMemoryReads(p); got != 1 {
			t.Errorf("the probe made %d memory reads, want 1 — the search stops at the FIRST record", got)
		}
	})
	t.Run("a CALL channel confirms too", func(t *testing.T) {
		_, s := openFor(t, oneRecord(wireAddr{callWireGroup, 3}, 144_500_000, 5))
		if !s.Diagnostics905().Fingerprinted {
			t.Error("a record found in the CALL bank did not confirm — the CALL slots are part of the bounded search")
		}
	})
}

// countMemoryReads counts the 1A 00 READ frames in a transcript.
func countMemoryReads(p *respondingPort) int {
	n := 0
	for _, f := range p.Transcript() {
		if len(f) == memoryReadFrameLen && f[4] == 0x1A && f[5] == 0x00 {
			n++
		}
	}
	return n
}

// TestProbe_TheFingerprintIsContinuous.
//
// The fingerprint is not one-shot: EVERY later record read re-validates
// its length, because civ.MemoryAnswerRecord checks it and this driver
// has exactly one place a record enters it (recordAt). So a 63-byte
// answer AFTER a clean open still fails, with civ's own
// *RecordLengthError, rather than being decoded against a layout it does
// not fit.
func TestProbe_TheFingerprintIsContinuous(t *testing.T) {
	img := oneRecord(wireAddr{0, 0}, 144_500_000, 5)
	// A SECOND channel, whose record is a length this profile never
	// declared. The probe stops at channel 0 and never sees it.
	img.records[wireAddr{0, 1}] = make([]byte, 63)

	_, s := openFor(t, img)
	if !s.Diagnostics905().Fingerprinted {
		t.Fatal("the open did not fingerprint on channel 0's good record")
	}

	_, _, err := s.recordAt(context.Background(), civ.ChannelAddress{Group: 0, Channel: 1})
	if err == nil {
		t.Fatal("a 63-byte record was accepted after a clean open — the fingerprint must be CONTINUOUS")
	}
	var rle *civ.RecordLengthError
	if !errors.As(err, &rle) {
		t.Fatalf("error = %v, want a *civ.RecordLengthError", err)
	}
	if rle.Got != 63 {
		t.Errorf("RecordLengthError.Got = %d, want 63", rle.Got)
	}
}

// TestProbe_ARecordForAnotherChannelIsFatalToTheOpen.
//
// civ.MemoryAnswerMatcher is deliberately ENVELOPE-ONLY (ruling T2), so
// an answer about another channel satisfies it and the DRIVER is what
// must catch it. During the probe there is no honest way to continue: a
// radio answering about a channel nobody asked for is not one this driver
// can go on to read a codeplug from, so the open fails — the same
// decision core/driver/ftdx101 makes when discovery meets a wrong-slot
// echo.
func TestProbe_ARecordForAnotherChannelIsFatalToTheOpen(t *testing.T) {
	img := oneRecord(wireAddr{0, 0}, 144_500_000, 5)
	img.answerAddr = map[wireAddr]wireAddr{{0, 0}: {0, 5}}

	p := newRespondingPort(t, img)
	_, err := New(RealHardware).Open(context.Background(), p.Port(), driver.Identity{})
	if err == nil {
		t.Fatal("Open succeeded although the radio answered about a different channel")
	}
	if !errors.Is(err, ErrAnswerMismatch) {
		t.Errorf("error = %v, want an ErrAnswerMismatch", err)
	}
}
