// SPDX-License-Identifier: GPL-3.0-or-later

package ic905_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic905"
)

// answerFrame wraps a record body as an answer FROM the radio: the
// direction check in civ's parser requires to == controller and
// from == radio.
func answerFrame(addr, record []byte) []byte {
	f := []byte{0xFE, 0xFE, 0xE0, 0xAC, 0x1A, 0x00}
	f = append(f, addr...)
	f = append(f, record...)
	return append(f, 0xFD)
}

// assertNumeric requires a numeric neutral field to be PRESENT and to
// carry want. Present-ness is asserted separately from the value because
// "this radio has no such field" and "this channel's value is 0" are
// different facts, and civ.Optional exists to keep them apart.
func assertNumeric(t *testing.T, field string, got civ.Optional[uint64], want uint64) {
	t.Helper()
	v, ok := got.Get()
	if !ok {
		t.Errorf("%s is unavailable, want %d", field, want)
		return
	}
	if v != want {
		t.Errorf("%s = %d, want %d", field, v, want)
	}
}

// assertText is assertNumeric for the enum- and name-carrying fields.
func assertText(t *testing.T, field string, got civ.Optional[string], want string) {
	t.Helper()
	v, ok := got.Get()
	if !ok {
		t.Errorf("%s is unavailable, want %q", field, want)
		return
	}
	if v != want {
		t.Errorf("%s = %q, want %q", field, v, want)
	}
}

func TestParse_TheGolden68ByteRecordInNeutralTerms(t *testing.T) {
	record := []byte{
		0x00,                         // (5)  split/select, unmapped
		0x00, 0x00, 0x50, 0x44, 0x01, // (6)~(10) 144.500000 MHz
		0x05,             // (11) FM
		0x01,             // (12) FIL1
		0x00,             // (13) data mode OFF
		0x00,             // (14) duplex OFF / tone OFF
		0x00,             // (15) digital squelch, unmapped
		0x00, 0x08, 0x85, // (16)~(18) 88.5 Hz
		0x00, 0x08, 0x85, // (19)~(21) 88.5 Hz
		0x00,       // (22) polarity NN
		0x00, 0x23, // (23),(24) DTCS 023
		0x00,             // (25) DV code squelch, unmapped
		0x00, 0x00, 0x00, // (26)~(28) offset 0
	}
	for i := 0; i < 24; i++ {
		record = append(record, 0x20) // (29)~(52) three call signs
	}
	record = append(record, []byte("HIGHLAND BASE905")...) // 53~68
	if len(record) != ic905.RecordLengthShort {
		t.Fatalf("hand-assembled record is %d bytes, want %d", len(record), ic905.RecordLengthShort)
	}

	rec, err := ic905.Profile().ParseMemoryAnswer(answerFrame([]byte{0x00, 0x00, 0x00, 0x01}, record))
	if err != nil {
		t.Fatalf("ParseMemoryAnswer: %v", err)
	}
	assertNumeric(t, "rx_frequency", rec.RXFreqHz, 144_500_000)
	assertNumeric(t, "tone_tx", rec.ToneTXDeciHz, 885)
	assertNumeric(t, "tone_rx", rec.ToneRXDeciHz, 885)
	assertNumeric(t, "dtcs_code", rec.DTCSCode, 23)
	assertNumeric(t, "offset", rec.OffsetHz, 0)
	assertText(t, "mode", rec.Mode, "FM")
	assertText(t, "filter", rec.Filter, "FIL1")
	assertText(t, "data_mode", rec.DataMode, "OFF")
	assertText(t, "duplex", rec.Duplex, "OFF")
	assertText(t, "tone_mode", rec.ToneMode, "OFF")
	assertText(t, "dtcs_polarity", rec.DTCSPolarity, "NN")
	assertText(t, "name", rec.Name, "HIGHLAND BASE905")
	if got := rec.Address; got != (civ.ChannelAddress{Group: 0, Channel: 1}) {
		t.Errorf("Address = %v, want g0/ch1", got)
	}
	if !rec.TXFreqHz.Unavailable() {
		t.Error("tx_frequency is present — this record has no TX frequency field and no duplicated TX block (matrix section 1b)")
	}
	if !rec.Select.Unavailable() {
		t.Error("select is present — (5) is deliberately unmapped; scan_skip on an Icom is SELECT membership and is never mapped as skip")
	}
}

// The 10 GHz record is where uint64 stops being theoretical: 10.25 GHz
// is about 2.4 times uint32's ceiling.
func TestParse_TheGolden69ByteRecordCarriesTenGigahertz(t *testing.T) {
	record := []byte{
		0x00,
		0x00, 0x00, 0x00, 0x50, 0x02, 0x01, // 10250.000000 MHz, six bytes
		0x05, 0x01, 0x00, 0x00, 0x00,
		0x00, 0x08, 0x85,
		0x00, 0x08, 0x85,
		0x00, 0x00, 0x23,
		0x00,
		0x00, 0x00, 0x00,
	}
	for i := 0; i < 24; i++ {
		record = append(record, 0x20)
	}
	record = append(record, []byte("HIGHLAND BASE905")...)
	if len(record) != ic905.RecordLengthWide {
		t.Fatalf("hand-assembled record is %d bytes, want %d", len(record), ic905.RecordLengthWide)
	}

	rec, err := ic905.Profile().ParseMemoryAnswer(answerFrame([]byte{0x00, 0x00, 0x00, 0x01}, record))
	if err != nil {
		t.Fatalf("ParseMemoryAnswer: %v", err)
	}
	assertNumeric(t, "rx_frequency", rec.RXFreqHz, 10_250_000_000)
	// Every field after the frequency sits one byte later and still
	// carries its printed index (G's hazard (d), STOP 1).
	assertText(t, "mode", rec.Mode, "FM")
	assertText(t, "name", rec.Name, "HIGHLAND BASE905")
}

// A length outside the accepted set is an ERROR, not a partial parse
// (spec D4, malformed records). The fingerprint is continuous.
func TestParse_AnUndeclaredLengthIsARecordLengthError(t *testing.T) {
	body := make([]byte, 63)
	_, err := ic905.Profile().ParseMemoryAnswer(answerFrame([]byte{0x00, 0x00, 0x00, 0x01}, body))
	var rle *civ.RecordLengthError
	if !errors.As(err, &rle) {
		t.Fatalf("ParseMemoryAnswer(63-byte record) error = %v, want *civ.RecordLengthError", err)
	}
	if rle.Got != 63 || !slices.Equal(rle.Want, []int{64, 65}) {
		t.Errorf("RecordLengthError = %+v, want Got 63 Want [64 65]", rle)
	}
}
