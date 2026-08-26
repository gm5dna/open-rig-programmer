// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic9700 "github.com/gm5dna/open-rig-programmer/core/civ/ic9700"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic9700"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// The record-offset landmarks these tests reach for. They are the Task-2
// layout table's own numbers, restated here rather than recomputed,
// because a test that mutates the wrong byte fails in a way that looks
// like a driver bug.
const (
	offSelect     = 0  // ④ low nibble
	offMode       = 6  // ⑩
	offDuplexTone = 9  // ⑬ high nibble duplex, low nibble tone mode
	offToneTX     = 11 // ⑮~⑰, three bytes, big-endian tenths of a hertz
	offURCallSign = 24 // ㉘~㉟, eight bytes
	dupShift      = 47 // the filled block ❺~❺❶ repeats the primary at +47
)

// bcdBE renders v as n packed-BCD bytes, most significant pair first —
// the order this record's tone and DTCS fields use.
func bcdBE(v uint64, n int) []byte {
	out := make([]byte, n)
	for i := n - 1; i >= 0; i-- {
		out[i] = byte(v%10) | byte(v/10%10)<<4
		v /= 100
	}
	return out
}

// occupiedRecord is the frozen golden record at 144-001: the one channel
// state the E6 template guard admits.
func occupiedRecord(t *testing.T) []byte {
	t.Helper()
	return templateRecord(mustAddress("144-001"))
}

// recordWithToneDeciHz is the golden record with BOTH copies of ⑮~⑰ set
// to deci tenths of a hertz.
//
// BOTH COPIES, always. This profile maps the filled block's non-frequency
// fields as REPEATS of their primary ids, so civ.decodeRecord requires the
// two to AGREE; a helper that touched only one would produce a parse error
// and the test would read as a tone bug. Register entry
// `ic9700-duplicate-block-agrees-on-read`.
func recordWithToneDeciHz(t *testing.T, slot string, deci uint64) []byte {
	t.Helper()
	rec := templateRecord(mustAddress(slot))
	b := bcdBE(deci, 3)
	copy(rec[offToneTX:], b)
	copy(rec[offToneTX+dupShift:], b)
	return rec
}

// recordWithDuplexNibble sets ⑬'s HIGH nibble (duplex) in both copies,
// leaving the LOW nibble (tone mode) as it was.
func recordWithDuplexNibble(t *testing.T, slot string, nibble byte) []byte {
	t.Helper()
	rec := templateRecord(mustAddress(slot))
	for _, off := range []int{offDuplexTone, offDuplexTone + dupShift} {
		rec[off] = rec[off]&0x0F | nibble<<4
	}
	return rec
}

// recordWithSelectNibble sets ④'s LOW nibble. It has no copy in the
// filled block — ④ sits ahead of it.
func recordWithSelectNibble(t *testing.T, slot string, nibble byte) []byte {
	t.Helper()
	rec := templateRecord(mustAddress(slot))
	rec[offSelect] = rec[offSelect]&0xF0 | nibble&0x0F
	return rec
}

// recordWithCallSign writes an eight-byte UR call sign into both copies of
// ㉘~㉟ — a channel carrying D-STAR data this tier cannot name.
func recordWithCallSign(t *testing.T, slot, call string) []byte {
	t.Helper()
	if len(call) != 8 {
		t.Fatalf("call sign %q is %d bytes; the field is exactly 8", call, len(call))
	}
	rec := templateRecord(mustAddress(slot))
	copy(rec[offURCallSign:], call)
	copy(rec[offURCallSign+dupShift:], call)
	return rec
}

// recordWithDisagreeingDuplicate sets the FILLED block's ⑩ to a different
// mode from the primary's, which is the one thing the manual's grey NOTE
// asserts cannot happen and no capture confirms.
func recordWithDisagreeingDuplicate(t *testing.T, slot string) []byte {
	t.Helper()
	rec := templateRecord(mustAddress(slot))
	rec[offMode+dupShift] = 0x00 // LSB, where the primary says FM
	return rec
}

// allFFRecord is a full-length record of 0xFF: this radio's SECOND way of
// saying a channel is empty (spec D5 entry 2(b), lift R12).
func allFFRecord() []byte {
	return bytes.Repeat([]byte{0xFF}, civic9700.RecordLength)
}

// sessionAnsweringSlot opens a session against a radio holding the golden
// record at slot, then ARMS the image's deliberate faults and forgets the
// probe's own traffic.
//
// The arming is what keeps a fault from corrupting the probe it is not
// about: a globally misdirected answer in force during Open would leave
// eight answer mismatches on the counter before the test's own read.
func sessionAnsweringSlot(t *testing.T, slot string, opts ...imageOption) (*ic9700.Session, *recordingPort) {
	t.Helper()
	port := newRecordingPort(t, baseImage(append([]imageOption{withTemplateStateAt(slot)}, opts...)...))
	sess := openWith(t, port)
	port.arm()
	port.clearTranscript()
	return sess, port
}

// withAnswerForAddress makes every read AFTER the probe answer with a
// well-formed record naming addr, whatever slot was asked for.
func withAnswerForAddress(addr civ.ChannelAddress) imageOption {
	return func(img *radioImage) {
		a := addr
		img.misdirect = &a
	}
}

// withAllFFAnswerForAddress is withAnswerForAddress serving the all-FF
// record — the second empty form at the WRONG slot.
func withAllFFAnswerForAddress(addr civ.ChannelAddress) imageOption {
	return func(img *radioImage) {
		a := addr
		img.misdirect = &a
		img.misdirectAllFF = true
	}
}

// readSlot reads one slot from a session over an image and returns both
// results.
func readSlot(t *testing.T, slot string, opts ...imageOption) (codeplug.Channel, error) {
	t.Helper()
	port := newRecordingPort(t, baseImage(opts...))
	sess := openWith(t, port)
	port.arm()
	return sess.ReadChannel(context.Background(), slot)
}

// readSlotAnswering reads a slot the radio answers with one of its
// non-record answers.
func readSlotAnswering(t *testing.T, slot string, kind answerKind) (codeplug.Channel, error) {
	t.Helper()
	if kind != civFA {
		t.Fatalf("unknown answer kind %v", kind)
	}
	// An absent record IS the FA answer: the scripted radio has no other
	// way to say "no such channel", which is exactly the position a real
	// one is in (D5 entry 2(a)).
	return readSlot(t, slot, withEmptySlot(slot))
}

// readSlotAnsweringRecord reads a slot holding raw bytes.
func readSlotAnsweringRecord(t *testing.T, slot string, record []byte) (codeplug.Channel, error) {
	t.Helper()
	return readSlot(t, slot, withStoredRecord(slot, record))
}

// readSlotWithDataArea reads a slot the radio answers with a data area of
// exactly n bytes.
//
// The override is applied AFTER the open, because a probe answered with an
// unacceptable length fails the open — which is Task 10's test, not this
// one's.
func readSlotWithDataArea(t *testing.T, slot string, n int) (codeplug.Channel, error) {
	t.Helper()
	port := newRecordingPort(t, factoryAnswers())
	sess := openWith(t, port)
	port.arm()
	port.setDataArea(n)
	return sess.ReadChannel(context.Background(), slot)
}

// readSlotWithDisagreeingDuplicate reads a slot whose filled block
// contradicts its primary.
func readSlotWithDisagreeingDuplicate(t *testing.T, slot string) (codeplug.Channel, error) {
	t.Helper()
	return readSlotAnsweringRecord(t, slot, recordWithDisagreeingDuplicate(t, slot))
}

// mustRead reads a slot and fails the test if the read did not succeed.
func mustRead(t *testing.T, slot string, opts ...imageOption) codeplug.Channel {
	t.Helper()
	ch, err := readSlot(t, slot, opts...)
	if err != nil {
		t.Fatalf("ReadChannel(%s): %v", slot, err)
	}
	if ch.Data == nil {
		t.Fatalf("ReadChannel(%s): empty channel, want a populated one", slot)
	}
	return ch
}

func readWithToneDeciHz(t *testing.T, deci uint64) codeplug.Channel {
	t.Helper()
	return mustRead(t, "144-001", withStoredRecord("144-001", recordWithToneDeciHz(t, "144-001", deci)))
}

func readWithDuplexNibble(t *testing.T, nibble byte) codeplug.Channel {
	t.Helper()
	return mustRead(t, "144-001", withStoredRecord("144-001", recordWithDuplexNibble(t, "144-001", nibble)))
}

func readWithSelectNibble(t *testing.T, nibble byte) codeplug.Channel {
	t.Helper()
	return mustRead(t, "144-001", withStoredRecord("144-001", recordWithSelectNibble(t, "144-001", nibble)))
}

// openThenFlood opens a healthy session and returns it with the port, so
// the caller can start flooding AFTER the open — the fail-closed half of
// R9-SPLIT, which is about a LATER drain and cannot be reached before one
// has succeeded.
func openThenFlood(t *testing.T) (*ic9700.Session, *recordingPort) {
	t.Helper()
	port := newRecordingPort(t, factoryAnswers())
	return openWith(t, port), port
}

func TestReadChannelMapsEveryFieldTheRecordCarries(t *testing.T) {
	// Per the Task-2 layout table, against leg G's own transcribed
	// values: 145.500000 MHz FM, FIL1, data mode off, duplex off, tone
	// mode off, 88.5 Hz both ways, DTCS 023 NN, a 600 kHz offset, the
	// same transmit frequency, and the name INVERNESS GB3CFR.
	ch := mustRead(t, "144-001", withTemplateStateAt("144-001"))
	d := ch.Data

	if ch.Slot != "144-001" {
		t.Errorf("Slot = %q, want %q", ch.Slot, "144-001")
	}
	if d.FreqHz != 145_500_000 {
		t.Errorf("FreqHz = %d, want 145500000", d.FreqHz)
	}
	if d.Mode != "FM" {
		t.Errorf("Mode = %q, want FM", d.Mode)
	}
	if d.Tag != "INVERNESS GB3CFR" {
		t.Errorf("Tag = %q, want %q", d.Tag, "INVERNESS GB3CFR")
	}
	if d.TxFreqHz.State != codeplug.Known || d.TxFreqHz.Value != 145_500_000 {
		t.Errorf("TxFreqHz = %+v, want Known 145500000", d.TxFreqHz)
	}
	if d.OffsetHz.State != codeplug.Known || d.OffsetHz.Value != 600_000 {
		t.Errorf("OffsetHz = %+v, want Known 600000 — the wire carries 100 Hz units", d.OffsetHz)
	}
	for _, tc := range []struct {
		name  string
		field codeplug.StringField
		want  string
	}{
		{"Duplex", d.Duplex, "OFF"},
		{"ToneMode", d.ToneMode, "OFF"},
		{"DTCSPolarity", d.DTCSPolarity, "NN"},
		{"Filter", d.Filter, "FIL1"},
	} {
		if tc.field.State != codeplug.Known || tc.field.Value != tc.want {
			t.Errorf("%s = %+v, want Known %q", tc.name, tc.field, tc.want)
		}
	}
	for _, tc := range []struct {
		name  string
		field codeplug.ToneField
	}{{"ToneTx", d.ToneTx}, {"ToneRx", d.ToneRx}} {
		if tc.field.State != codeplug.Known || tc.field.Value != spec.Tone(885) {
			t.Errorf("%s = %+v, want Known 885 deciHz", tc.name, tc.field)
		}
	}
	if d.DTCSCode.State != codeplug.Known || d.DTCSCode.Value != 23 {
		t.Errorf("DTCSCode = %+v, want Known 23", d.DTCSCode)
	}
	if d.DataMode.State != codeplug.Known || d.DataMode.Value {
		t.Errorf("DataMode = %+v, want Known false", d.DataMode)
	}

	// The fields this record does not have, reported as absent rather
	// than invented.
	if d.CTCSSTone.State != codeplug.Unavailable {
		t.Errorf("CTCSSTone = %+v, want Unavailable — this family expresses tone_tx/tone_rx instead", d.CTCSSTone)
	}
	if d.TagDisplay.State != codeplug.Unavailable {
		t.Errorf("TagDisplay = %+v, want Unavailable", d.TagDisplay)
	}
	if d.CTCSS != "" || d.Shift != "" || d.ClarHz != 0 || d.RxClar || d.TxClar {
		t.Errorf("a Yaesu field carries a value: CTCSS=%q Shift=%q Clar=%d/%v/%v",
			d.CTCSS, d.Shift, d.ClarHz, d.RxClar, d.TxClar)
	}

	// And a channel this driver produced must survive the neutral
	// validator it will be handed to.
	if err := d.ToneTx.Valid(ic9700.CapabilitiesUnverified()); err != nil {
		t.Errorf("ToneTx.Valid: %v", err)
	}
	if err := d.Duplex.Valid(duplexValues()); err != nil {
		t.Errorf("Duplex.Valid: %v", err)
	}
}

// duplexValues is caps.DuplexOptions as the plain vocabulary
// StringField.Valid takes.
func duplexValues() []string {
	var out []string
	for _, o := range ic9700.CapabilitiesUnverified().DuplexOptions {
		out = append(out, o.Value)
	}
	return out
}

func TestReadChannelRefusesAnAnswerNamingAnotherSlot(t *testing.T) {
	// T2, and the reason it cannot be left to the matcher: the landed
	// MemoryAnswerMatcher is envelope-only.
	sess, _ := sessionAnsweringSlot(t, "144-001", withAnswerForAddress(civ.ChannelAddress{Group: 1, Channel: 7}))
	_, err := sess.ReadChannel(context.Background(), "144-001")
	if !errors.Is(err, ic9700.ErrAnswerMismatch) {
		t.Fatalf("err = %v, want errors.Is match against ErrAnswerMismatch", err)
	}
	var mm *ic9700.AnswerMismatchError
	if !errors.As(err, &mm) {
		t.Fatalf("err = %v, want *ic9700.AnswerMismatchError", err)
	}
	if mm.Requested.Channel != 1 || mm.Answered.Channel != 7 {
		t.Errorf("mismatch = requested %v answered %v", mm.Requested, mm.Answered)
	}
	if sess.CIVDiagnostics().AnswerMismatches != 1 {
		t.Error("the mismatch was not counted in diagnostics")
	}
}

func TestAWrongSlotAnswerIsNotMistakenForAnEmptySlot(t *testing.T) {
	// The ordering matters: if empty recognition ran first, an all-FF
	// record for the WRONG slot would report the RIGHT slot as empty.
	sess, _ := sessionAnsweringSlot(t, "144-001",
		withAllFFAnswerForAddress(civ.ChannelAddress{Group: 1, Channel: 7}))
	_, err := sess.ReadChannel(context.Background(), "144-001")
	if !errors.Is(err, ic9700.ErrAnswerMismatch) {
		t.Fatalf("err = %v, want ErrAnswerMismatch — T2 precedes empty recognition", err)
	}
}

func TestReadChannelMapsAnOutOfDomainToneToUnknown(t *testing.T) {
	// T1(3): the civ layer is lossless and decodes tone spans as plain
	// BCD numbers, ZERO included; the capability domain starts at 1
	// deciHz. A read must never construct a Known value that
	// codeplug.ToneField.Valid would refuse.
	ch := readWithToneDeciHz(t, 0)
	if got := ch.Data.ToneTx.State; got != codeplug.Unknown {
		t.Errorf("ToneTx.State = %q for a wire zero, want %q", got, codeplug.Unknown)
	}
	if ch.Data.ToneTx.Value != 0 {
		t.Errorf("ToneTx.Value = %v under a non-Known state, want the zero value", ch.Data.ToneTx.Value)
	}
	inRange := readWithToneDeciHz(t, 885)
	if got := inRange.Data.ToneTx.State; got != codeplug.Known {
		t.Errorf("ToneTx.State = %q for 88.5 Hz, want %q", got, codeplug.Known)
	}
	if inRange.Data.ToneTx.Value != spec.Tone(885) {
		t.Errorf("ToneTx = %v, want 885 deciHz", inRange.Data.ToneTx.Value)
	}
}

func TestReadChannelRejectionMeansAnEmptyChannelNotAnError(t *testing.T) {
	// D5 entry 2(a), lift R11. driver.go — a rejected read is an EMPTY
	// CHANNEL, never an error, or core/clone/read.go aborts the whole
	// ReadAll on the first unwritten slot.
	ch, err := readSlotAnswering(t, "144-004", civFA)
	if err != nil {
		t.Fatalf("err = %v, want nil — an empty slot is not a failure", err)
	}
	if ch.Data != nil {
		t.Errorf("Data = %+v, want nil for an empty slot", ch.Data)
	}
	if ch.Slot != "144-004" {
		t.Errorf("Slot = %q, want %q — the empty channel still names its slot", ch.Slot, "144-004")
	}
}

func TestReadChannelAllFFRecordMeansAnEmptyChannelToo(t *testing.T) {
	// D5 entry 2(b), lift R12, recognised BEFORE the record parser (R10):
	// decodeRecord would reject 0xFF against this profile's enums and
	// report a parse error, which is not what an empty slot is.
	ch, err := readSlotAnsweringRecord(t, "144-005", allFFRecord())
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if ch.Data != nil {
		t.Errorf("Data = %+v, want nil for an all-FF record", ch.Data)
	}
}

func TestAPartlyFilledRadioReadsAllTheWayThrough(t *testing.T) {
	// The regression REV 1 would have shipped: one empty slot must not
	// abort the walk. 144-002 is absent and answers FA; 144-003 answers a
	// full-length record of 0xFF — the two empty forms, side by side.
	port := newRecordingPort(t, baseImage(
		withStoredRecord("144-001", occupiedRecord(t)),
		withStoredRecord("144-003", allFFRecord()),
		withStoredRecord("144-004", occupiedRecord(t)),
	))
	sess := openWith(t, port)

	got := 0
	for _, slot := range []string{"144-001", "144-002", "144-003", "144-004"} {
		ch, err := sess.ReadChannel(context.Background(), slot)
		if err != nil {
			t.Fatalf("%s: %v", slot, err)
		}
		if ch.Data != nil {
			got++
		}
	}
	if got != 2 {
		t.Errorf("read %d populated channels, want 2", got)
	}
}

func TestReadChannelPresentsRPSAsUnknownNotAsOff(t *testing.T) {
	// OQ-6: core/spec has three DuplexDirections and this radio's ⑬ high
	// nibble has four values. Flattening RPS onto OFF would be a lie
	// about the radio; Unavailable would be a lie about the field.
	ch := readWithDuplexNibble(t, 3)
	if got := ch.Data.Duplex.State; got != codeplug.Unknown {
		t.Errorf("Duplex.State = %q, want %q", got, codeplug.Unknown)
	}
	if ch.Data.Duplex.Value != "" {
		t.Errorf("Duplex.Value = %q, want empty under a non-Known state", ch.Data.Duplex.Value)
	}
	// The three values the vocabulary CAN name still come back Known, so
	// the rule above is a narrowing and not a blanket refusal.
	for nibble, want := range map[byte]string{0: "OFF", 1: "DUP-", 2: "DUP+"} {
		if got := readWithDuplexNibble(t, nibble).Data.Duplex; got.State != codeplug.Known || got.Value != want {
			t.Errorf("duplex nibble %d = %+v, want Known %q", nibble, got, want)
		}
	}
}

func TestReadChannelReportsScanSkipUnavailable(t *testing.T) {
	// The field is Unsupported in caps (OQ-4), so the read must say
	// Unavailable rather than invent a boolean out of ④'s four values.
	ch := readWithSelectNibble(t, 2) // ★2
	if got := ch.Data.ScanSkip.State; got != codeplug.Unavailable {
		t.Errorf("ScanSkip.State = %q, want %q", got, codeplug.Unavailable)
	}
	if ch.Data.ScanSkip.Value {
		t.Error("ScanSkip.Value is true under a non-Known state")
	}
}

func TestReadAllAbortsHonestlyOnAMalformedRecord(t *testing.T) {
	// spec D4, adjudication 13: a record whose length is not in the
	// accepted set is an ERROR — and, unlike an empty slot, it SHOULD
	// abort. The two cases are distinguished by the pre-parse hook, not
	// conflated by it.
	_, err := readSlotWithDataArea(t, "144-002", 113)
	var rl *civ.RecordLengthError
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v, want *civ.RecordLengthError", err)
	}
	if rl.Got != 110 {
		t.Errorf("RecordLengthError.Got = %d, want 110 — the RECORD, not the 113-byte data area", rl.Got)
	}
}

func TestReadChannelSurfacesADisagreeingDuplicateAsAParseError(t *testing.T) {
	// register entry ic9700-duplicate-block-agrees-on-read; lift W2.
	_, err := readSlotWithDisagreeingDuplicate(t, "144-003")
	if !errors.Is(err, civ.ErrParse) {
		t.Fatalf("err = %v, want a civ parse error naming the field that disagrees", err)
	}
}

func TestReadChannelRefusesASlotThisRadioHasNoNameFor(t *testing.T) {
	// The refusal precedes any wire traffic: a slot string this radio has
	// no memory for is not a read to attempt at whichever channel the
	// nearest interpretation lands on.
	port := newRecordingPort(t, factoryAnswers())
	sess := openWith(t, port)
	port.clearTranscript()
	if _, err := sess.ReadChannel(context.Background(), "144-100"); err == nil {
		t.Fatal("ReadChannel accepted a slot string this radio cannot name")
	}
	if got := port.countReads(); got != 0 {
		t.Errorf("the driver sent %d reads for an unnameable slot", got)
	}
}

func TestALaterQuarantineDrainFailureFailsClosed(t *testing.T) {
	// The other half of the R9 rule: only the INITIAL drain is forgiven,
	// and only a CONTROLLER-ADDRESSED flood can produce the failure at
	// all.
	sess, port := openThenFlood(t)
	port.startAddressedFlooding()
	_, err := sess.ReadChannel(context.Background(), "144-001")
	if err == nil {
		t.Fatal("a quarantine drain failure after open must fail closed")
	}
}
