// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	ic7300civ "github.com/gm5dna/open-rig-programmer/core/civ/ic7300"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic7300"
	"github.com/gm5dna/open-rig-programmer/core/transport"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7300"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7300mk2"
)

// THE TWO CASES, AND THE HONEST ONE COMES FIRST — because the synthetic one
// would otherwise read as a claim about the radios, and it is not one.
//
// In the FIELD the IC-7300 and the IC-7300MK2 cannot confuse each other: they
// answer at 94h and B6h, and a driver addressing one gets no answer at all
// from the other. Spec D3.2 says exactly that — "a wrong model at a DIFFERENT
// default address simply times out" — so the record-length fingerprint this
// driver carries protects against SAME-ADDRESS confusion only: a radio moved
// onto this address, or a bus mis-set. Case 2 manufactures that collision with
// the fake's WithRadioAddress, which is a SIMULATOR CONFIGURATION and not a
// claim that any radio ships so.

// mk2Record is a 45-byte record in the MK2's own shape, inside that fake's
// derived vocabularies so WithChannel will store it. Its CONTENT is
// irrelevant here — the length is the whole of the evidence — but it must be
// storable, or the fake would refuse the seed and the probe would meet an
// empty radio instead of a foreign record.
func mk2Record() []byte {
	rec := make([]byte, 45)
	rec[0] = 0x00                                     // ③
	copy(rec[1:6], []byte{0x00, 0x00, 0x10, 0x14, 0}) // ④ ~ ⑧ — 14.100 MHz
	rec[6] = 0x01                                     // ⑨ — USB
	rec[7] = 0x01                                     // ⑩ — FIL1
	rec[8] = 0x00                                     // ⑪
	copy(rec[9:12], []byte{0x00, 0x08, 0x85})         // ⑫ ~ ⑭
	copy(rec[12:15], []byte{0x00, 0x10, 0x00})        // ⑮ ~ ⑰
	copy(rec[15:20], []byte{0x00, 0x00, 0x10, 0x14, 0})
	rec[20] = 0x01
	rec[21] = 0x01
	rec[22] = 0x00
	copy(rec[23:26], []byte{0x00, 0x08, 0x85})
	copy(rec[26:29], []byte{0x00, 0x10, 0x00})
	for i := 29; i < 45; i++ {
		rec[i] = 0x20 // ⑱ ~ ㉝ — the printed space
	}
	return rec
}

// CASE 1, THE HONEST ONE. At their true addresses the two siblings cannot
// confuse each other: the IC-7300 driver talks to 94h and the MK2 fake answers
// only to B6h, so the probe times out. This test exists so that nobody reads
// Case 2 as a field behaviour.
func TestSibling_MK2AtItsOwnAddressSimplyTimesOut(t *testing.T) {
	// SEEDED, so the timeout is attributable to the ADDRESS and not merely to
	// an empty radio: this fake would answer a correctly addressed read.
	radio := fakeic7300mk2.New(fakeic7300mk2.WithChannel("001", mk2Record()))
	defer radio.Close()

	_, err := ic7300.New(ic7300.Simulated).
		Open(context.Background(), radio.Port(), driver.Identity{Port: "fake"})
	if err == nil {
		t.Fatal("Open succeeded against an IC-7300MK2 fake at B6h — the IC-7300 driver addresses 94h and must get no answer at all")
	}
	if errors.Is(err, driver.ErrWrongRadio) {
		t.Fatalf("Open returned ErrWrongRadio: %v — at DIFFERENT default addresses there is no answer to fingerprint, and claiming a model here would be a claim the evidence does not support", err)
	}
	if !errors.Is(err, transport.ErrTimeout) {
		t.Errorf("Open error = %v, want a timeout", err)
	}
}

// CASE 2, THE SYNTHETIC ONE. WithRadioAddress manufactures the same-address
// collision the length fingerprint exists to catch: an MK2 answering at 94h
// with its own 45-byte records. A simulator configuration, NOT a claim that
// any radio behaves so — the two models ship at 94h and B6h.
func TestSibling_A45ByteRecordAt94hIsWrongRadioProvisionally(t *testing.T) {
	// SEED AN OCCUPIED SLOT. Open scans MEM 1..8, and an unpopulated fake
	// answers FA to every one of them, which opens the session UNFINGERPRINTED
	// — the test would fail to exercise the fingerprint at all. The seeded
	// record is this fake's own 45-byte shape, which is the whole point.
	radio := fakeic7300mk2.New(
		fakeic7300mk2.WithRadioAddress(0x94),
		fakeic7300mk2.WithChannel("001", mk2Record()),
	)
	defer radio.Close()

	_, err := ic7300.New(ic7300.Simulated).
		Open(context.Background(), radio.Port(), driver.Identity{Port: "fake"})
	var wrong *driver.WrongRadioError
	if !errors.As(err, &wrong) {
		t.Fatalf("Open error = %v, want *driver.WrongRadioError", err)
	}
	if wrong.WantModel != "IC-7300" {
		t.Errorf("WantModel = %q, want %q", wrong.WantModel, "IC-7300")
	}
	if !strings.Contains(wrong.GotModel, "IC-7300MK2") {
		t.Errorf("GotModel = %q, want it to name the IC-7300MK2", wrong.GotModel)
	}
	// THE WRAPPER'S OWN PROSE MUST SAY IT, not merely the GotModel string.
	// "IC-7300MK2 (provisional)" already contains the word, so a bare
	// Contains over the whole message would stay green if the driver's own
	// sentence were changed to claim a firm identification — which is
	// precisely the sentence D3.2 and D10 constrain. Stripping the wrapped
	// error off the end leaves exactly the prefix this driver added.
	prefix := strings.TrimSuffix(err.Error(), wrong.Error())
	if prefix == err.Error() {
		t.Fatalf("the *driver.WrongRadioError is not wrapped at the end of %q — the assertion below cannot isolate the driver's own sentence", err)
	}
	if !strings.Contains(prefix, "provisional") {
		t.Errorf("the driver's own sentence %q does not say the attribution is provisional — both record lengths are ASSUMED derivations from printed field widths, and the error must say so (spec D3.2)", prefix)
	}
	for _, want := range []string{"45", "39", "ASSUMED"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error text %q does not contain %q — the refusal names the length it saw, the length it wanted, and the grade of both", err, want)
		}
	}
}

// A length NEITHER model's set names is refused with NO attribution. Seed the
// slot with WithRawChannel, which bypasses the fake's own length check — that
// is the only way to put a 40-byte record on the wire.
//
// THE FAKE IS THIS DRIVER'S OWN, at its own address, which makes the statement
// sharper: even a radio that identifies correctly at 94h is refused when it
// answers a length nobody declares, and no model is named for it.
func TestSibling_AnUnknownLengthIsRefusedWithoutNamingAModel(t *testing.T) {
	radio := fakeic7300.New(fakeic7300.WithRawChannel("001", make([]byte, 40)))
	defer radio.Close()

	_, err := ic7300.New(ic7300.Simulated).
		Open(context.Background(), radio.Port(), driver.Identity{Port: "fake"})
	if err == nil {
		t.Fatal("Open succeeded against a radio answering a 40-byte record — no model in this tier declares that length")
	}
	var wrong *driver.WrongRadioError
	if errors.As(err, &wrong) {
		t.Fatalf("Open returned a *driver.WrongRadioError naming %q — 40 bytes is in no model's table, and guessing one from a number nobody has seen would be worse than saying nothing", wrong.GotModel)
	}
	if !errors.Is(err, civ.ErrRecordLength) {
		t.Errorf("Open error = %v, want it to wrap civ.ErrRecordLength", err)
	}
	if !strings.Contains(err.Error(), "40") {
		t.Errorf("error text %q does not name the observed length", err)
	}
}

// The civ layer refuses independently of the driver, so the property does not
// rest on the probe alone: a future driver that forgot its table would still
// meet a typed refusal from the codec.
func TestSibling_CIVRefusesTheForeignRecordLength(t *testing.T) {
	// A 1A 00 answer TO this controller FROM 94h carrying a 45-byte record:
	// the frame the synthetic collision above puts on the wire. Built by hand
	// so this test does not depend on the fake at all.
	frame := []byte{0xFE, 0xFE, 0xE0, 0x94, 0x1A, 0x00, 0x00, 0x01}
	frame = append(frame, mk2Record()...)
	frame = append(frame, 0xFD)

	_, err := ic7300civ.Profile().ParseMemoryAnswer(frame)
	var rle *civ.RecordLengthError
	if !errors.As(err, &rle) {
		t.Fatalf("ParseMemoryAnswer error = %v, want *civ.RecordLengthError", err)
	}
	if rle.Got != 45 || len(rle.Want) != 1 || rle.Want[0] != 39 {
		t.Errorf("RecordLengthError = {Want:%v Got:%d}, want {Want:[39] Got:45}", rle.Want, rle.Got)
	}
}
