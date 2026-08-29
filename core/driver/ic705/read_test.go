// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver/internal/drivertest"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// goldenRecord returns the 111 record bytes of the manual-derived
// `set-record-name-with-space` vector — the frame the G leg transcribed
// from the IC-705 CI-V Reference Guide's own worked example.
//
// IT IS THE EVIDENCE, NOT A BUILDER'S OUTPUT, and that is the whole point
// of using it here: a fixture produced by civ's encoder would agree with a
// decoder that shared its mistake just as happily as with a correct one.
// The file is core/civ/ic705's FROZEN testdata (a compiled-in SHA-256
// manifest guards it), read here read-only and never written.
func goldenRecord(t *testing.T) []byte {
	t.Helper()
	const path = "../../civ/ic705/testdata/vectors.golden"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the frozen vectors: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		name, hexBytes, ok := strings.Cut(line, "\t")
		if !ok || name != "set-record-name-with-space" {
			continue
		}
		frame, err := hex.DecodeString(strings.ReplaceAll(strings.TrimSpace(hexBytes), " ", ""))
		if err != nil {
			t.Fatalf("decoding the set vector: %v", err)
		}
		// FE FE A4 E0 1A 00 (6) + four address bytes, then the record,
		// then FD.
		return append([]byte(nil), frame[10:len(frame)-1]...)
	}
	t.Fatal("the set-record-name-with-space vector is missing from the frozen file")
	return nil
}

// radioHolding scripts a radio holding one record at one display slot.
func radioHolding(t *testing.T, slot string, record []byte) *scriptedRadio {
	t.Helper()
	return newScriptedRadio(t, radioImage{
		records: map[civ.ChannelAddress][]byte{addrOf(t, slot): record},
	})
}

func TestReadChannelDecodesTheGoldenRecord(t *testing.T) {
	// The Task 6 record, field by field, through the driver's own mapping.
	r := radioHolding(t, "G01-001", goldenRecord(t))
	sess := openSession(t, r)
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Slot != "G01-001" {
		t.Errorf("Slot = %q", ch.Slot)
	}
	if ch.Data == nil {
		t.Fatal("an occupied channel came back empty")
	}
	d := *ch.Data
	if d.FreqHz != 145500000 {
		t.Errorf("FreqHz = %d, want 145500000", d.FreqHz)
	}
	if d.Mode != "FM" {
		t.Errorf("Mode = %q, want \"FM\"", d.Mode)
	}
	if d.Tag != "MY REPEATER CH01" {
		t.Errorf("Tag = %q, want \"MY REPEATER CH01\" — including both interior spaces", d.Tag)
	}
	if d.TxFreqHz != (codeplug.FreqField{State: codeplug.Known, Value: 145500000}) {
		t.Errorf("TxFreqHz = %+v", d.TxFreqHz)
	}
	if d.OffsetHz != (codeplug.FreqField{State: codeplug.Known, Value: 600000}) {
		t.Errorf("OffsetHz = %+v, want 600 kHz", d.OffsetHz)
	}
	for _, tc := range []struct {
		name  string
		got   codeplug.StringField
		value string
	}{
		{"Duplex", d.Duplex, "DUP-"},
		{"ToneMode", d.ToneMode, "TONE"},
		{"Filter", d.Filter, "FIL1"},
		{"DTCSPolarity", d.DTCSPolarity, "NN"},
	} {
		if tc.got != (codeplug.StringField{State: codeplug.Known, Value: tc.value}) {
			t.Errorf("%s = %+v, want Known %q", tc.name, tc.got, tc.value)
		}
	}
	if d.ToneTx != (codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)}) {
		t.Errorf("ToneTx = %+v, want Known 88.5 Hz", d.ToneTx)
	}
	if d.ToneRx != (codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)}) {
		t.Errorf("ToneRx = %+v, want Known 88.5 Hz", d.ToneRx)
	}
	if d.DTCSCode != (codeplug.IntField{State: codeplug.Known, Value: 23}) {
		t.Errorf("DTCSCode = %+v, want Known 23", d.DTCSCode)
	}
	if d.DataMode != (codeplug.BoolField{State: codeplug.Known, Value: false}) {
		t.Errorf("DataMode = %+v, want Known false (the vector's 00 is OFF)", d.DataMode)
	}

	// The written-down zeros: this radio has no clarifier, and its
	// shift/tone-state vocabularies are the Yaesu family's, not its own.
	if d.ClarHz != 0 || d.RxClar || d.TxClar || d.CTCSS != "" || d.Shift != "" {
		t.Errorf("a Yaesu-shaped field carries a value: %+v", d)
	}
	for _, tc := range []struct {
		name  string
		state codeplug.FieldState
	}{
		{"CTCSSTone", d.CTCSSTone.State},
		{"TagDisplay", d.TagDisplay.State},
		{"ScanSkip", d.ScanSkip.State},
	} {
		if tc.state != codeplug.Unavailable {
			t.Errorf("%s.State = %q, want Unavailable — this record has no such field (ScanSkip per O-6)", tc.name, tc.state)
		}
	}
	drivertest.AssertFreshReadSaveLoad(t, ch, codeplug.Load)
}

func TestReadChannelOfARejectedSlotIsAnEmptyChannel(t *testing.T) {
	// An unwritten channel answers FA, which Engine.Do consumes into
	// transport.ErrRejected with no frame at all (ruling T4). It is an
	// EMPTY CHANNEL, never an error that aborts a whole-radio read.
	r := radioHolding(t, "G01-001", goldenRecord(t))
	sess := openSession(t, r)
	ch, err := sess.ReadChannel(context.Background(), "G01-050")
	if err != nil {
		t.Fatalf("ReadChannel of an unwritten slot: %v", err)
	}
	if ch.Slot != "G01-050" || ch.Data != nil {
		t.Errorf("got %+v, want an empty channel labelled G01-050", ch)
	}
}

func TestReadChannelOfAnAllFFRecordIsAnEmptyChannel(t *testing.T) {
	// The other unverified empty shape (D5 entry 2(b), lift L-EMPTY-FF).
	// It must be recognised on the RAW bytes: 0xFF fails the enum decode,
	// so testing for it after parsing would be too late.
	ff := make([]byte, 111)
	for i := range ff {
		ff[i] = 0xFF
	}
	r := radioHolding(t, "G01-001", ff)
	sess := openSession(t, r)
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data != nil {
		t.Errorf("an all-FF record was decoded into a channel: %+v", *ch.Data)
	}
}

func TestReadChannelOfAWrongLengthRecordFails(t *testing.T) {
	// Spec D4 adjudication 13: the read FAILS with ErrRecordLength — no
	// partial parse, no invented "Unavailable" channel, and NOT the empty
	// channel an FA produces. This is also the fingerprint being
	// CONTINUOUS: a session that was right about the radio at Open still
	// re-checks every record it reads.
	r := radioHolding(t, "G01-001", goldenRecord(t))
	sess := openSession(t, r)
	r.SetRecord(addrOf(t, "G01-002"), make([]byte, 39))
	ch, err := sess.ReadChannel(context.Background(), "G01-002")
	if !errors.Is(err, civ.ErrRecordLength) {
		t.Fatalf("ReadChannel returned (%+v, %v), want ErrRecordLength", ch, err)
	}
	if ch.Data != nil {
		t.Error("a wrong-length read returned channel data as well as an error")
	}
}

func TestToneOffChannelDecodesToUnknownNotKnownZero(t *testing.T) {
	// T1(3). The wire zero — what this radio holds for "no tone set" — is
	// OUTSIDE the declared domain, because AdmitsTone(0) is false under
	// any legal CTCSSToneRange (spec.Validate enforces MinDeciHz > 0). So
	// the read maps it to Unknown with a zero value, never to a Known zero
	// the codeplug layer would refuse. Writability is restored by T1(4)'s
	// preservation (write.go), not by widening the vocabulary.
	rec := fullRecord(addrOf(t, "G01-001"))
	rec.ToneTXDeciHz = civ.Available[uint64](0)
	rec.ToneRXDeciHz = civ.Available[uint64](0)
	rec.ToneMode = civ.Available("OFF")
	r := radioHolding(t, "G01-001", encodedRecord(t, rec))
	sess := openSession(t, r)
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	for _, tc := range []struct {
		name string
		got  codeplug.ToneField
	}{{"ToneTx", ch.Data.ToneTx}, {"ToneRx", ch.Data.ToneRx}} {
		if tc.got.State != codeplug.Unknown {
			t.Errorf("%s.State = %q, want Unknown for the wire zero", tc.name, tc.got.State)
		}
		if tc.got.Value != 0 {
			t.Errorf("%s.Value = %v, want 0 — a non-Known ToneField must carry a zero value", tc.name, tc.got.Value)
		}
		if err := tc.got.Valid(sess.Capabilities()); err != nil {
			t.Errorf("%s does not validate: %v", tc.name, err)
		}
	}
}

func TestToneInsideTheDomainDecodesToKnown(t *testing.T) {
	// The boundaries as well as the ordinary case: 1 deciHz is the
	// declared floor and 2999 the declared (and encodable) ceiling.
	for _, tone := range []uint64{1, 885, 2999} {
		rec := fullRecord(addrOf(t, "G01-001"))
		rec.ToneTXDeciHz = civ.Available(tone)
		rec.ToneRXDeciHz = civ.Available(tone)
		r := radioHolding(t, "G01-001", encodedRecord(t, rec))
		sess := openSession(t, r)
		ch, err := sess.ReadChannel(context.Background(), "G01-001")
		if err != nil {
			t.Fatalf("tone %d: ReadChannel: %v", tone, err)
		}
		want := codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(tone)}
		if ch.Data.ToneTx != want || ch.Data.ToneRx != want {
			t.Errorf("tone %d decoded to %+v / %+v, want %+v", tone, ch.Data.ToneTx, ch.Data.ToneRx, want)
		}
	}
}

func TestRXFrequencyAtOrAbove500MHzFailsTheRead(t *testing.T) {
	// F1: RXFreqHz carries no FieldState, so it cannot fall back to
	// Unknown the way TxFreqHz/OffsetHz can — a read must never construct
	// a Known value write.go's rung 7 would refuse, so this is a decode
	// error naming the field and the value.
	rec := fullRecord(addrOf(t, "G01-001"))
	rec.RXFreqHz = civ.Available[uint64](500000000)
	r := radioHolding(t, "G01-001", encodedRecord(t, rec))
	sess := openSession(t, r)
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err == nil {
		t.Fatalf("ReadChannel accepted a 500 000 000 Hz rx frequency: %+v", ch)
	}
	if !strings.Contains(err.Error(), "rx frequency") || !strings.Contains(err.Error(), "500000000") {
		t.Errorf("the error %q does not name the field and value", err)
	}
	if ch.Data != nil {
		t.Error("channel data came back alongside the decode failure")
	}
}

func TestTXFrequencyAtOrAbove500MHzDecodesToUnknown(t *testing.T) {
	// F1's mirror for TxFreqHz, which DOES carry a FieldState: a read
	// never constructs a Known value write.go's rung 7 would refuse, so
	// this decodes to Unknown rather than failing the whole read.
	rec := fullRecord(addrOf(t, "G01-001"))
	rec.TXFreqHz = civ.Available[uint64](500000000)
	r := radioHolding(t, "G01-001", encodedRecord(t, rec))
	sess := openSession(t, r)
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data.TxFreqHz != (codeplug.FreqField{State: codeplug.Unknown}) {
		t.Errorf("TxFreqHz = %+v, want Unknown with a zero value", ch.Data.TxFreqHz)
	}
}

func TestOffsetAboveTheManualsCeilingDecodesToUnknown(t *testing.T) {
	// F1's mirror for OffsetHz: the field's three BCD bytes would happily
	// carry a sixth digit past the fixed 10 MHz leader write.go's rung 7
	// enforces, so this decodes to Unknown rather than a Known value the
	// write path could never accept back.
	rec := fullRecord(addrOf(t, "G01-001"))
	rec.OffsetHz = civ.Available[uint64](10000000)
	r := radioHolding(t, "G01-001", encodedRecord(t, rec))
	sess := openSession(t, r)
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data.OffsetHz != (codeplug.FreqField{State: codeplug.Unknown}) {
		t.Errorf("OffsetHz = %+v, want Unknown with a zero value", ch.Data.OffsetHz)
	}
}

func TestADTCSCodeOutsideTheOctalTableDecodesToUnknown(t *testing.T) {
	// F2: the 512-code table (caps.go's dtcsCodes) is built from three
	// OCTAL digits, 0-7 each. This field's two packed-BCD bytes decode any
	// digit 0-9 without complaint — the comment this fixes claimed the
	// table "covers this field's whole printed domain", which is false: a
	// record whose packed digits are 8s decodes to 888, a real value civ
	// happily builds and decodes (fed here through BuildMemorySet/the
	// scripted port, not hand-poked bytes) that the octal table never
	// held.
	rec := fullRecord(addrOf(t, "G01-001"))
	rec.DTCSCode = civ.Available[uint64](888)
	r := radioHolding(t, "G01-001", encodedRecord(t, rec))
	sess := openSession(t, r)
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data.DTCSCode != (codeplug.IntField{State: codeplug.Unknown}) {
		t.Errorf("DTCSCode = %+v, want Unknown with a zero value", ch.Data.DTCSCode)
	}
}

func TestADTCSCodeInsideTheOctalTableStillDecodesToKnown(t *testing.T) {
	// The boundary case for F2's fix: a genuinely octal-digit code (023,
	// the golden vector's own value) must still decode to Known — the
	// membership check must not fail closed on the whole table.
	rec := fullRecord(addrOf(t, "G01-001"))
	rec.DTCSCode = civ.Available[uint64](23)
	r := radioHolding(t, "G01-001", encodedRecord(t, rec))
	sess := openSession(t, r)
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data.DTCSCode != (codeplug.IntField{State: codeplug.Known, Value: 23}) {
		t.Errorf("DTCSCode = %+v, want Known 23", ch.Data.DTCSCode)
	}
}

func TestAnswerForTheWrongChannelIsRefusedBeforeAnyUse(t *testing.T) {
	// T2, at the seam where it belongs. The landed MemoryAnswerMatcher is
	// envelope-only BY DESIGN, so it ACCEPTS this answer; the driver is
	// therefore the only thing standing between another channel's contents
	// and a caller who asked for this one.
	r := radioHolding(t, "G01-012", goldenRecord(t))
	other := addrOf(t, "G01-100")
	r.SetRecord(other, goldenRecord(t))
	sess := openSession(t, r)

	before := sess.SessionInfo().AnswerMismatches
	r.AnswerNextReadWithAddress(other)
	ch, err := sess.ReadChannel(context.Background(), "G01-012")
	if !errors.Is(err, ErrAnswerMismatch) {
		t.Fatalf("ReadChannel returned (%+v, %v), want ErrAnswerMismatch", ch, err)
	}
	var mismatch *AnswerMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("%v is not an *AnswerMismatchError", err)
	}
	if mismatch.Requested != addrOf(t, "G01-012") || mismatch.Answered != other {
		t.Errorf("the error names %v/%v, want %v/%v — both addresses, so a reader can see what happened", mismatch.Requested, mismatch.Answered, addrOf(t, "G01-012"), other)
	}
	if ch.Data != nil || ch.Slot != "" {
		t.Errorf("a channel came back alongside the mismatch: %+v — least of all one labelled with the requested slot and carrying another slot's contents", ch)
	}
	if got := sess.SessionInfo().AnswerMismatches; got != before+1 {
		t.Errorf("AnswerMismatches = %d, want %d — the model surface counts what the neutral one cannot", got, before+1)
	}
}

func TestReadChannelRoundTripsThroughCodeplugValidate(t *testing.T) {
	// Nothing in this tree had ever validated a SPARSE bank at codeplug
	// level. One read memory plus the four call channels, against the
	// SESSION's own capabilities, with ZERO issues.
	r := radioHolding(t, "G01-001", goldenRecord(t))
	for c := 1; c <= 4; c++ {
		r.SetRecord(addrOf(t, spec.SparseSlot(101, c)), goldenRecord(t))
	}
	sess := openSession(t, r)

	cp := &codeplug.Codeplug{
		Schema:    codeplug.CurrentSchema,
		Generator: "ic705-test",
		Radio: codeplug.RadioInfo{
			Model: sess.Capabilities().Model,
			CATID: sess.Capabilities().CATID,
		},
	}
	slots := []string{"G01-001", "G101-001", "G101-002", "G101-003", "G101-004"}
	for _, slot := range slots {
		ch, err := sess.ReadChannel(context.Background(), slot)
		if err != nil {
			t.Fatalf("ReadChannel(%q): %v", slot, err)
		}
		cp.Channels = append(cp.Channels, ch)
	}
	if issues := codeplug.Validate(cp, sess.Capabilities()); len(issues) != 0 {
		for _, i := range issues {
			t.Errorf("codeplug.Validate: [%v] slot %q field %q: %s", i.Severity, i.Slot, i.Field, i.Msg)
		}
	}
}

func TestCallChannelReadsThroughTheSameRecord(t *testing.T) {
	// G101-004 is wire group 100, channel 3 — the same 111-byte layout as
	// every memory. The CALL bank is its own namespace, not its own
	// format.
	r := radioHolding(t, "G101-004", goldenRecord(t))
	sess := openSession(t, r)
	ch, err := sess.ReadChannel(context.Background(), "G101-004")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data == nil || ch.Data.FreqHz != 145500000 {
		t.Fatalf("the call channel did not decode: %+v", ch)
	}
	var sawCallRead bool
	for _, f := range r.Transcript() {
		if len(f) < 11 || f[4] != 0x1A {
			continue
		}
		if a, err := decodeWireAddress(f[6:10]); err == nil && a.Group == 100 && a.Channel == 3 {
			sawCallRead = true
		}
	}
	if !sawCallRead {
		t.Error("no read went to wire group 100 channel 3 — the CALL display numbering must map to the manual's own 0100/0003")
	}
}

func TestASplitChannelWhoseTXBlockDisagreesFailsHonestly(t *testing.T) {
	// THE COST OF THE DUPLICATED-SPAN CHOICE, stated and tested rather
	// than left implicit. civ refuses a record whose duplicated copies
	// disagree — the right refusal, because silently preferring one copy
	// would be a guess about which the radio honours — and the consequence
	// is real: a channel the RADIO itself wrote with Split ON and a
	// TX-side mode differing from the RX side fails to parse, so
	// ReadChannel errors and clone.ReadAll ABORTS. One exotic split
	// channel makes the radio uncloneable, not merely unwritable.
	//
	// The error SHAPE is pinned here so that a future decision to soften
	// it — to an empty channel with a diagnostic, say — is a deliberate
	// change with a failing test to update, rather than a drift.
	rec := encodedRecord(t, fullRecord(addrOf(t, "G01-001")))
	// Record offset 53 is the TX block's copy of the mode byte (offset 6
	// plus the block's 47-byte delta). Make it disagree.
	rec[53] = 0x01 // USB in the block, FM in the RX area
	r := radioHolding(t, "G01-001", rec)
	sess := openSession(t, r)
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err == nil {
		t.Fatalf("ReadChannel accepted a record whose duplicated copies disagree: %+v", ch)
	}
	if errors.Is(err, civ.ErrRecordLength) {
		t.Errorf("the disagreement was reported as a length error: %v", err)
	}
	if ch.Data != nil {
		t.Error("channel data came back alongside the parse failure")
	}
	if !errors.Is(err, civ.ErrParse) {
		t.Errorf("ReadChannel returned %v, want a civ parse error naming the disagreement", err)
	}
}

func TestReadChannelRefusesAMalformedSlot(t *testing.T) {
	r := radioHolding(t, "G01-001", goldenRecord(t))
	sess := openSession(t, r)
	readsBefore := r.Reads()
	for _, slot := range []string{"", "G001-001", "G101-005", "G01-000"} {
		if ch, err := sess.ReadChannel(context.Background(), slot); err == nil {
			t.Errorf("ReadChannel(%q) = %+v, want an error", slot, ch)
		}
	}
	if r.Reads() != readsBefore {
		t.Error("a malformed slot put a frame on the wire — the slot is parsed before anything is built")
	}
}
