// SPDX-License-Identifier: GPL-3.0-or-later

package ic7410

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// openSession opens a session against a scripted radio and returns the
// port's transcript length AT THE MOMENT Open returned — Open itself puts
// the 19 00 identity probe and the occupied-slot fingerprint search on the
// wire, so every test below measures NEW frames from this baseline rather
// than an absolute frame count.
func openSession(t *testing.T, img radioImage, opts ...Option) (sess *Session, sp *scriptedPort, afterOpen int) {
	t.Helper()
	sp = newScriptedPort(t, img)
	s, err := New(opts...).Open(t.Context(), sp.Port(), driver.Identity{Port: "/dev/fake"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s.(*Session), sp, len(sp.Transcript())
}

func TestE2E_OpenFingerprintsAndReads(t *testing.T) {
	img := radioImage{idToken: []byte{0x01}, records: map[int][]byte{1: vector1.record(t)}}
	sess, _, _ := openSession(t, img, WithSimulatedProfile())

	length, confirmed := sess.Fingerprint()
	if !confirmed || length != 40 {
		t.Fatalf("Fingerprint = (%d, %v), want (40, true)", length, confirmed)
	}

	ch, err := sess.ReadChannel(t.Context(), "0001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data == nil {
		t.Fatal("ReadChannel returned an empty channel")
	}
	d := *ch.Data
	if d.FreqHz != vector1.freqHz || d.Mode != vector1.mode || d.Filter.Value != vector1.filter {
		t.Errorf("channel = %+v", d)
	}
	if d.TxFreqHz.State != codeplug.Known || d.TxFreqHz.Value != vector1.txFreqHz {
		t.Errorf("TxFreqHz = %+v, want Known %d — the TX-duplicate block's frequency span", d.TxFreqHz, vector1.txFreqHz)
	}
	if d.DataMode.State != codeplug.Known || !d.DataMode.Value {
		t.Errorf("DataMode = %+v, want Known true", d.DataMode)
	}
	if d.ScanSkip.State != codeplug.Unavailable {
		t.Errorf("ScanSkip = %+v, want Unavailable — byte 0 has no neutral home on this model", d.ScanSkip)
	}
}

func TestE2E_ReadEmptySlot(t *testing.T) {
	img := radioImage{idToken: []byte{0x01}, records: map[int][]byte{}}
	sess, _, _ := openSession(t, img, WithSimulatedProfile())

	ch, err := sess.ReadChannel(t.Context(), "0002")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data != nil {
		t.Fatalf("expected an empty channel, got %+v", ch.Data)
	}
}

func TestE2E_MovedAddressTimesOutCleanly(t *testing.T) {
	img := radioImage{idToken: nil} // silence: no address-matched reply
	sp := newScriptedPort(t, img)
	_, err := New(WithSimulatedProfile()).Open(t.Context(), sp.Port(), driver.Identity{Port: "/dev/fake"})
	if err == nil {
		t.Fatal("Open succeeded against a radio that never answered 19 00")
	}
}

func TestE2E_AnswerMismatch(t *testing.T) {
	img := radioImage{
		idToken: []byte{0x01},
		// Channel 1 is Open's own probe target and answers CORRECTLY, so
		// the fingerprint succeeds; channel 5 is what THIS test reads, and
		// its answer is deliberately mis-attributed to channel 6 (tier
		// ruling T2).
		records: map[int][]byte{1: vector1.record(t), 5: vector1.record(t)},
		answerAddress: func(asked int) int {
			if asked == 5 {
				return 6
			}
			return asked
		},
	}
	sess, _, _ := openSession(t, img, WithSimulatedProfile())

	_, err := sess.ReadChannel(t.Context(), "0005")
	if err == nil {
		t.Fatal("ReadChannel succeeded against an answer naming the wrong channel")
	}
	var mismatch *AnswerMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("error = %v, want *AnswerMismatchError", err)
	}
	if sess.AnswerMismatches() != 1 {
		t.Errorf("AnswerMismatches() = %d, want 1", sess.AnswerMismatches())
	}
}

func TestE2E_WriteChannel_HappyPath(t *testing.T) {
	img := radioImage{idToken: []byte{0x01}, records: map[int][]byte{1: vector1.record(t)}, ackSets: true}
	sess, sp, before := openSession(t, img, WithSimulatedProfile())

	data := &codeplug.ChannelData{
		FreqHz: 7_100_000, Mode: "LSB",
		Filter:   codeplug.StringField{State: codeplug.Known, Value: "FIL2"},
		DataMode: codeplug.BoolField{State: codeplug.Known, Value: false},
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: 1000},
		ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: 1000},
		Tag:      "NEW CH",
	}
	res, err := sess.WriteChannel(t.Context(), codeplug.Channel{Slot: "0001", Data: data})
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Fatalf("WriteResult = %+v", res)
	}

	transcript := sp.Transcript()
	// ONE read (preservation) then ONE set: exactly two NEW frames since Open.
	newFrames := transcript[before:]
	if len(newFrames) != 2 {
		t.Fatalf("write put %d new frame(s) on the wire, want 2 (one preservation read, one set):\n  %s", len(newFrames), hexFrames(newFrames))
	}
	setFrame := newFrames[1]
	if len(setFrame) != memSetFrameLen {
		t.Fatalf("set frame is %d bytes, want %d", len(setFrame), memSetFrameLen)
	}
	rec := setFrame[8 : 8+40]
	// The TX frequency span must MIRROR the RX frequency: the caller set no
	// TxFreqHz, and spec.md's ruling 7 rules that the write path mirrors RX
	// into TX rather than refusing (a deliberate deviation from IC-7300).
	if got, want := rec[16:21], bcdLE(7_100_000, 5); string(got) != string(want) {
		t.Errorf("TX frequency span = % 02x, want % 02x (mirrored from RX)", got, want)
	}
	if rec[0] != 0x00 {
		t.Errorf("record[0] (Select-memory + Split) = %#02x, want 0x00", rec[0])
	}
	for i := 21; i < 31; i++ {
		if rec[i] != 0x00 {
			t.Errorf("record[%d] (TX-duplicate-block unmapped remainder) = %#02x, want 0x00", i, rec[i])
		}
	}
}

func TestE2E_WriteChannel_ExplicitTxFrequencyOnMem(t *testing.T) {
	img := radioImage{idToken: []byte{0x01}, records: map[int][]byte{1: vector1.record(t)}, ackSets: true}
	sess, sp, _ := openSession(t, img, WithSimulatedProfile())

	data := &codeplug.ChannelData{
		FreqHz: 7_100_000, Mode: "LSB",
		Filter:   codeplug.StringField{State: codeplug.Known, Value: "FIL2"},
		DataMode: codeplug.BoolField{State: codeplug.Known, Value: false},
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: 1000},
		ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: 1000},
		TxFreqHz: codeplug.FreqField{State: codeplug.Known, Value: 7_200_000},
		Tag:      "SPLIT",
	}
	if _, err := sess.WriteChannel(t.Context(), codeplug.Channel{Slot: "0001", Data: data}); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	transcript := sp.Transcript()
	rec := transcript[len(transcript)-1][8 : 8+40]
	if got, want := rec[16:21], bcdLE(7_200_000, 5); string(got) != string(want) {
		t.Errorf("TX frequency span = % 02x, want % 02x (the caller's explicit value)", got, want)
	}
}

func TestE2E_WriteChannel_ExplicitTxFrequencyRefusedOnScan(t *testing.T) {
	img := radioImage{idToken: []byte{0x01}, records: map[int][]byte{100: vector1.record(t)}, ackSets: true}
	sess, sp, before := openSession(t, img, WithSimulatedProfile())

	data := &codeplug.ChannelData{
		FreqHz: 7_100_000, Mode: "LSB",
		Filter:   codeplug.StringField{State: codeplug.Known, Value: "FIL2"},
		DataMode: codeplug.BoolField{State: codeplug.Known, Value: false},
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: 1000},
		ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: 1000},
		TxFreqHz: codeplug.FreqField{State: codeplug.Known, Value: 7_200_000},
	}
	_, err := sess.WriteChannel(t.Context(), codeplug.Channel{Slot: "P1", Data: data})
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v, want *driver.WriteRefusedError", err)
	}
	if len(refused.Fields) != 1 || refused.Fields[0] != spec.FieldTxFrequency {
		t.Fatalf("refused fields = %v, want [tx_frequency]", refused.Fields)
	}
	// RUNG 2 refuses before any wire traffic: no NEW frame since Open.
	if got := sp.Transcript(); len(got) != before {
		t.Errorf("write put %d new frame(s) on the wire, want 0 (refused before all wire traffic): %s", len(got)-before, hexFrames(got[before:]))
	}
}

func TestE2E_WriteChannel_UnmappedRegionRefused(t *testing.T) {
	dirty := vector1.record(t)
	dirty[0] = 0x11 // a real radio's Select-memory/Split byte, non-zero
	img := radioImage{idToken: []byte{0x01}, records: map[int][]byte{1: dirty}, ackSets: true}
	sess, sp, before := openSession(t, img, WithSimulatedProfile())

	data := &codeplug.ChannelData{
		FreqHz: 7_100_000, Mode: "LSB",
		Filter:   codeplug.StringField{State: codeplug.Known, Value: "FIL2"},
		DataMode: codeplug.BoolField{State: codeplug.Known, Value: false},
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: 1000},
		ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: 1000},
	}
	_, err := sess.WriteChannel(t.Context(), codeplug.Channel{Slot: "0001", Data: data})
	var unmapped *UnmappedRegionError
	if !errors.As(err, &unmapped) {
		t.Fatalf("error = %v, want *UnmappedRegionError", err)
	}
	if unmapped.Offset != 0 {
		t.Errorf("Offset = %d, want 0", unmapped.Offset)
	}
	// Exactly ONE new frame (the preservation read) — no set frame follows
	// an unmapped-region refusal.
	if got := sp.Transcript(); len(got)-before != 1 {
		t.Errorf("write put %d new frame(s) on the wire, want 1 (the preservation read only): %s", len(got)-before, hexFrames(got[before:]))
	}
}

func TestE2E_WriteChannel_EraseRefused(t *testing.T) {
	img := radioImage{idToken: []byte{0x01}, records: map[int][]byte{}}
	sess, sp, before := openSession(t, img, WithSimulatedProfile())

	_, err := sess.WriteChannel(t.Context(), codeplug.Channel{Slot: "0001", Data: nil})
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v, want *driver.WriteRefusedError", err)
	}
	if len(refused.Fields) != 1 || refused.Fields[0] != spec.FieldErase {
		t.Fatalf("refused fields = %v, want [erase]", refused.Fields)
	}
	if got := sp.Transcript(); len(got) != before {
		t.Errorf("erase refusal put %d new frame(s) on the wire, want 0", len(got)-before)
	}
}

func TestE2E_WriteChannel_RealHardwareRefusedWithoutConsent(t *testing.T) {
	img := radioImage{idToken: []byte{0x01}, records: map[int][]byte{1: vector1.record(t)}, ackSets: true}
	sess, sp, before := openSession(t, img) // RealHardware, no consent

	data := &codeplug.ChannelData{
		FreqHz: 7_100_000, Mode: "LSB",
		Filter:   codeplug.StringField{State: codeplug.Known, Value: "FIL2"},
		DataMode: codeplug.BoolField{State: codeplug.Known, Value: false},
		ToneMode: codeplug.StringField{State: codeplug.Known, Value: "OFF"},
		ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: 1000},
		ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: 1000},
	}
	_, err := sess.WriteChannel(t.Context(), codeplug.Channel{Slot: "0001", Data: data})
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v, want *driver.WriteRefusedError — no IC-7410 has ever been written to, so nothing is writable without consent", err)
	}
	if got := sp.Transcript(); len(got) != before {
		t.Errorf("refused write put %d new frame(s) on the wire, want 0", len(got)-before)
	}
}
