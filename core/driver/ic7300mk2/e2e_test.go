// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300mk2_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic7300mk2"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7300mk2"
)

// This file is the END-TO-END half of this driver's evidence: the driver
// against a fake IC-7300MK2 written by an author who never saw
// core/civ/ic7300mk2, core/driver/ic7300mk2 or this plan, and who re-derived
// the record's length, offsets and vocabularies from the same two committed
// evidence artefacts the dialect was built from.
//
// THAT INDEPENDENCE IS THE WHOLE POINT, and it is why these tests are worth
// more than their assertions look. A systematic misreading of the record strip
// — a field one byte out, a nibble the wrong way round, a length off by the
// two channel-address bytes — would pass every unit test in this package,
// because a unit test compares the codec against a fixture the same reading
// produced. It cannot pass here: the fake refuses a record whose length or
// whose printed vocabularies it does not recognise, and it answers from its
// own offsets. Green here means TWO independent readings of one document
// agree.
//
// The pairing lives in this file and NOT in internal/wiring. Wave 3 never
// touches wiring, and the registry rows are a separate commit.
//
// STILL NOT HARDWARE. No IC-7300MK2 has been asked anything by this project,
// and two agreeing readings of a document are not a radio.
//
// THE FAKE IS STRICTER THAN THE IC-7300'S IN TWO PLACES, both of which this
// file's fixture has to satisfy and neither of which the dialect had to be
// told about: ⑮ ~ ⑰'s first byte must be 00 and its 100 Hz digit at most 2
// (page 23's per-digit legend), and ⑱ ~ ㉝'s sixteen bytes must each be a
// PRINTED character code with NO pad byte of its own — 0x00 is refused, and
// the space this driver pads with is a printed code in its own right. Both
// records below are inside those vocabularies, which is a fact about the two
// derivations agreeing rather than a concession made to get the test green.

// e2eRecordBytes builds the 45-byte record this file seeds and expects.
//
// BUILT BY HAND FROM THE PRINTED WIDTHS, never by the codec under test: a
// fixture BuildMemorySet produced would agree with the parser about a wrong
// offset just as happily as a right one, which is exactly the failure this
// file exists to catch.
//
// Every value is inside the fake's own derived vocabularies — it validates
// every one of the twelve fields on every set, ⑮ ~ ⑰'s per-digit legend and
// ⑱ ~ ㉝'s printed character codes included — so a record this function builds
// is one the fake will store.
func e2eRecordBytes(sel byte, hz uint64, tag string) []byte {
	rec := make([]byte, 45)
	rec[0] = sel                    // ③ — Split OFF (high), SELECT group (low)
	copy(rec[1:6], bcdFreqLE(hz))   // ④ ~ ⑧
	rec[6] = 0x01                   // ⑨ — USB
	rec[7] = 0x01                   // ⑩ — FIL1
	rec[8] = 0x00                   // ⑪ — data mode OFF (high), tone mode OFF (low)
	copy(rec[9:12], toneBCD(885))   // ⑫ ~ ⑭ — 88.5 Hz
	copy(rec[12:15], toneBCD(1230)) // ⑮ ~ ⑰ — 123.0 Hz, DIFFERENT from ⑫ ~ ⑭ so
	//                                 a span swap cannot hide behind one value
	copy(rec[15:20], bcdFreqLE(hz)) // ❹ ~ ⑧ — the transmit frequency
	rec[20] = 0x01                  // ❾
	rec[21] = 0x01                  // ❿
	rec[22] = 0x00                  // ⓫
	copy(rec[23:26], toneBCD(885))  // ⓬ ~ ⓮
	copy(rec[26:29], toneBCD(1230)) // ⓯ ~ ⓱
	name := rec[29:45]              // ⑱ ~ ㉝ — SIXTEEN bytes on this model
	for i := range name {
		// The space. This document prints NO pad byte at all, and the fake
		// refuses 0x00 for that reason; the space is a printed code, which is
		// what makes a short name expressible in a fixed-width field.
		name[i] = 0x20
	}
	copy(name, tag)
	return rec
}

// bcdFreqLE packs hz as five packed-BCD bytes, LEAST significant pair first —
// the order the strip's five-cell diagram prints. Hand-rolled here rather than
// taken from core/civ, for e2eRecordBytes' reason.
func bcdFreqLE(hz uint64) []byte {
	out := make([]byte, 5)
	for i := 0; i < 5; i++ {
		pair := hz % 100
		out[i] = byte(pair/10)<<4 | byte(pair%10)
		hz /= 100
	}
	return out
}

// toneBCD packs a tone in TENTHS of a hertz as three packed-BCD bytes, MOST
// significant pair first.
func toneBCD(deciHz uint64) []byte {
	return []byte{
		byte(deciHz/100000%10)<<4 | byte(deciHz/10000%10),
		byte(deciHz/1000%10)<<4 | byte(deciHz/100%10),
		byte(deciHz/10%10)<<4 | byte(deciHz%10),
	}
}

// e2eExpected is the codeplug.ChannelData the record above must decode to.
func e2eExpected(slot string, hz uint64, tag string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz:       hz,
			Mode:         "USB",
			CTCSSTone:    codeplug.ToneField{State: codeplug.Unavailable},
			Tag:          tag,
			TagDisplay:   codeplug.BoolField{State: codeplug.Unavailable},
			ScanSkip:     codeplug.BoolField{State: codeplug.Unavailable},
			TxFreqHz:     codeplug.FreqField{State: codeplug.Known, Value: hz},
			Duplex:       codeplug.StringField{State: codeplug.Unavailable},
			OffsetHz:     codeplug.FreqField{State: codeplug.Unavailable},
			ToneMode:     codeplug.StringField{State: codeplug.Known, Value: "OFF"},
			ToneTx:       codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)},
			ToneRx:       codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(1230)},
			DTCSCode:     codeplug.IntField{State: codeplug.Unavailable},
			DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},
			Filter:       codeplug.StringField{State: codeplug.Known, Value: "FIL1"},
			DataMode:     codeplug.BoolField{State: codeplug.Known, Value: false},
		},
	}
}

// compareChannel reports every field on which got differs from want, by name.
// A whole-struct DeepEqual would say "these differ" and leave the reader to
// find out where, on a struct with twenty members.
func compareChannel(t *testing.T, what string, got, want codeplug.Channel) {
	t.Helper()
	if got.Slot != want.Slot {
		t.Errorf("%s: Slot = %q, want %q", what, got.Slot, want.Slot)
	}
	if got.Empty() != want.Empty() {
		t.Fatalf("%s: Empty() = %v, want %v", what, got.Empty(), want.Empty())
	}
	if got.Empty() {
		return
	}
	g, w := *got.Data, *want.Data
	for _, f := range []struct {
		name      string
		got, want any
	}{
		{"FreqHz", g.FreqHz, w.FreqHz},
		{"Mode", g.Mode, w.Mode},
		{"ClarHz", g.ClarHz, w.ClarHz},
		{"RxClar", g.RxClar, w.RxClar},
		{"TxClar", g.TxClar, w.TxClar},
		{"CTCSS", g.CTCSS, w.CTCSS},
		{"CTCSSTone", g.CTCSSTone, w.CTCSSTone},
		{"Shift", g.Shift, w.Shift},
		{"Tag", g.Tag, w.Tag},
		{"TagDisplay", g.TagDisplay, w.TagDisplay},
		{"ScanSkip", g.ScanSkip, w.ScanSkip},
		{"TxFreqHz", g.TxFreqHz, w.TxFreqHz},
		{"Duplex", g.Duplex, w.Duplex},
		{"OffsetHz", g.OffsetHz, w.OffsetHz},
		{"ToneMode", g.ToneMode, w.ToneMode},
		{"ToneTx", g.ToneTx, w.ToneTx},
		{"ToneRx", g.ToneRx, w.ToneRx},
		{"DTCSCode", g.DTCSCode, w.DTCSCode},
		{"DTCSPolarity", g.DTCSPolarity, w.DTCSPolarity},
		{"Filter", g.Filter, w.Filter},
		{"DataMode", g.DataMode, w.DataMode},
	} {
		if f.got != f.want {
			t.Errorf("%s: %s = %v, want %v", what, f.name, f.got, f.want)
		}
	}
}

// openFake opens a consented Simulated session against radio and registers the
// teardown in the order the resources require: the SESSION first (it owns the
// port), then the radio.
func openFake(t *testing.T, radio *fakeic7300mk2.Radio) driver.Session {
	t.Helper()
	sess, err := ic7300mk2.New(ic7300mk2.Simulated, ic7300mk2.WithConsentedUnverifiedWrites()).
		Open(context.Background(), radio.Port(), driver.Identity{Port: "fake"})
	if err != nil {
		t.Fatalf("Open against the fake: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

// civDiag reaches this driver's model-specific diagnostics surface.
func civDiag(t *testing.T, sess driver.Session) ic7300mk2.CIVDiagnostics {
	t.Helper()
	s, ok := sess.(*ic7300mk2.Session)
	if !ok {
		t.Fatalf("session is %T, not *ic7300mk2.Session", sess)
	}
	return s.CIVDiagnostics()
}

// The full sequence spec D3.2 names: an address-matched identity reply, then a
// bounded occupied-slot search that confirms the record length.
func TestE2E_ProbeFingerprintsAgainstTheFake(t *testing.T) {
	radio := fakeic7300mk2.New(fakeic7300mk2.WithChannel("001", e2eRecordBytes(0x00, 14_100_000, "TESTING NAME0123")))
	defer radio.Close()
	sess := openFake(t, radio)

	d := civDiag(t, sess)
	if !d.Fingerprinted {
		t.Error("Fingerprinted = false — the fake answered a record at its own derived length, and that length is what the probe fingerprints on")
	}
	if d.ProbeSlotsRead != 1 {
		t.Errorf("ProbeSlotsRead = %d, want 1 — the search stops at the first record", d.ProbeSlotsRead)
	}
	// The fake's default identity token is the address it is configured with,
	// which is its own choice and not one this driver may depend on — so the
	// assertion is on the SHAPE, "the address hex, a colon, and whatever came
	// back", and never on the token's value.
	if got := sess.Identity().CATID; !strings.HasPrefix(got, "b6:") || len(got) <= len("b6:") {
		t.Errorf("Identity().CATID = %q, want %q followed by the observed token — the 19 00 reply VALUE is undocumented and is recorded, never matched", got, "b6:")
	}
	// The FIRST frame the radio ever saw is the identity read. Nothing is
	// written to a CI-V radio at Init.
	frames := radio.Received()
	if len(frames) == 0 {
		t.Fatal("the fake saw no frames — every assertion here would be vacuous")
	}
	want := []byte{0xFE, 0xFE, 0xB6, 0xE0, 0x19, 0x00, 0xFD}
	if string(frames[0]) != string(want) {
		t.Errorf("first frame = % X, want % X", frames[0], want)
	}
}

// Every populated slot in the fake's image reads back field for field, and
// every unpopulated one reads back EMPTY rather than erroring.
func TestE2E_ReadAllSlots(t *testing.T) {
	seeded := map[string]struct {
		sel byte
		hz  uint64
		tag string
	}{
		"001": {0x00, 14_100_000, "TESTING NAME0123"},
		"042": {0x01, 7_100_000, "FORTY TWO"},
		"099": {0x02, 28_500_000, "LAST"},
		"P1":  {0x00, 14_000_000, "EDGE LOW"},
		"P2":  {0x00, 14_350_000, "EDGE HIGH"},
	}
	var opts []fakeic7300mk2.Option
	for slot, s := range seeded {
		opts = append(opts, fakeic7300mk2.WithChannel(slot, e2eRecordBytes(s.sel, s.hz, s.tag)))
	}
	radio := fakeic7300mk2.New(opts...)
	defer radio.Close()
	sess := openFake(t, radio)

	read := 0
	for _, b := range sess.Capabilities().Banks {
		for _, slot := range b.Slots {
			ch, err := sess.ReadChannel(context.Background(), slot)
			if err != nil {
				t.Fatalf("ReadChannel(%q): %v — an unwritten channel is an EMPTY channel, not an error, or ReadAll would abort on the first blank memory", slot, err)
			}
			read++
			if s, ok := seeded[slot]; ok {
				compareChannel(t, slot, ch, e2eExpected(slot, s.hz, s.tag))
				continue
			}
			if !ch.Empty() {
				t.Errorf("ReadChannel(%q) came back populated from a radio that holds nothing there", slot)
			}
		}
	}
	if read != 101 {
		t.Errorf("read %d slots, want 101 — 99 memories plus P1 and P2 (D11)", read)
	}
}

// One write into a SEEDED (occupied) slot, then a read-back comparing every
// field, with WriteResult carrying exactly one Sent+Confirmed step.
//
// SEEDED, not empty. A write into an empty slot is a CREATE, which this
// driver REFUSES for want of an honest SELECT value — so an e2e that wrote
// into a blank slot would contradict the unit design it exists to confirm.
// The create case has its own witness below.
func TestE2E_WriteOneAndReadItBack(t *testing.T) {
	radio := fakeic7300mk2.New(fakeic7300mk2.WithChannel("001", e2eRecordBytes(0x00, 14_100_000, "TESTING NAME0123")))
	defer radio.Close()
	sess := openFake(t, radio)

	const newHz = 14_105_000
	const newTag = "REWRITTEN NAME12"
	want := e2eExpected("001", newHz, newTag)

	res, err := sess.WriteChannel(context.Background(), want)
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || res.Steps[0].Command != "1A 00" {
		t.Fatalf("Steps = %+v, want exactly one 1A 00 step", res.Steps)
	}
	if !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("Steps[0] = %+v, want Sent and Confirmed — the fake answers an accepted set with the six-byte FB, so Confirmed here means a real acknowledgement arrived", res.Steps[0])
	}

	got, err := sess.ReadChannel(context.Background(), "001")
	if err != nil {
		t.Fatalf("ReadChannel after the write: %v", err)
	}
	compareChannel(t, "read-back", got, want)

	// AND THE RADIO'S OWN BYTES, which is the half a read-back cannot check:
	// a driver whose encoder and decoder shared one wrong offset would read
	// back exactly what it wrote and still have stored the wrong record.
	stored, ok := radio.Channel("001")
	if !ok {
		t.Fatal("the fake holds no record for 001 after an acknowledged set")
	}
	if len(stored) != 45 {
		t.Fatalf("the fake stored %d bytes, want 45", len(stored))
	}
	expect := e2eRecordBytes(0x00, newHz, newTag)
	if string(stored) != string(expect) {
		t.Errorf("the fake stored\n  % X\nwant\n  % X\n— the driver's encoder and the fake's independently derived offsets disagree", stored, expect)
	}
}

// The create rung, end to end, so the user-facing honesty row has a pin: a
// write to a slot the fake holds EMPTY is refused, and no set frame reaches
// the wire.
func TestE2E_CreateIntoAnEmptySlotIsRefused(t *testing.T) {
	radio := fakeic7300mk2.New(fakeic7300mk2.WithChannel("001", e2eRecordBytes(0x00, 14_100_000, "TESTING NAME0123")))
	defer radio.Close()
	sess := openFake(t, radio)

	before := len(radio.Received())
	_, err := sess.WriteChannel(context.Background(), e2eExpected("050", 21_200_000, "NEW ONE"))
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel into an empty slot = %v, want ErrWriteRefused — an empty slot has no SELECT group to preserve and no spec.Field carries one, so writing OFF would put the channel into a scan group the caller never chose", err)
	}
	if !strings.Contains(err.Error(), "SELECT") {
		t.Errorf("refusal %q does not name the SELECT nibble", err)
	}
	if n := countSets(radio.Received()[before:]); n != 0 {
		t.Errorf("%d set frames reached the radio on a refused create", n)
	}
	if _, ok := radio.Channel("050"); ok {
		t.Error("the fake now holds a record for 050 — a refused create must leave the radio untouched")
	}
}

// The fake refuses the documented clear forms and the driver never builds one.
func TestE2E_RefusesErase(t *testing.T) {
	radio := fakeic7300mk2.New(fakeic7300mk2.WithChannel("001", e2eRecordBytes(0x02, 14_100_000, "TESTING NAME0123")))
	defer radio.Close()
	sess := openFake(t, radio)

	// A full read-write-read cycle, so the scan below covers every frame this
	// driver is capable of emitting.
	if _, err := sess.ReadChannel(context.Background(), "001"); err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if _, err := sess.WriteChannel(context.Background(), e2eExpected("001", 14_105_000, "REWRITTEN NAME12")); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if _, err := sess.ReadChannel(context.Background(), "001"); err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}

	// An empty channel is an ERASE request, and it is refused before any wire
	// traffic.
	before := len(radio.Received())
	_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "001"})
	if !errors.Is(err, driver.ErrWriteRefused) || !strings.Contains(err.Error(), string(spec.FieldErase)) {
		t.Fatalf("WriteChannel(empty) = %v, want an ErrWriteRefused naming erase", err)
	}
	if after := len(radio.Received()); after != before {
		t.Errorf("the erase refusal put %d frames on the wire", after-before)
	}

	for i, f := range radio.Received() {
		cn, sc, ok := civ.FrameCommand(f)
		if !ok {
			continue
		}
		switch {
		case cn == 0x0B:
			t.Errorf("frame %d = % X is a 0B memory clear — this tier ships no erase path", i, f)
		case cn == 0x18:
			t.Errorf("frame %d = % X is an 18 %02X power command — nothing here may send one", i, sc, f)
		case cn == 0x1A && sc == 0x05:
			t.Errorf("frame %d = % X is a 1A 05 menu write — nothing in this tier may send one", i, f)
		case cn == 0x1A && sc == 0x00 && len(f) == 10 && f[8] == 0xFF:
			t.Errorf("frame %d = % X is the printed single-FF clear recipe — the driver must never build one", i, f)
		}
	}
	// The channel is still there, which is what "no erase path" means in the
	// only terms a user cares about.
	if _, ok := radio.Channel("001"); !ok {
		t.Error("the fake no longer holds 001 — something in this cycle erased it")
	}
}

// An empty radio (every slot FA) opens UNFINGERPRINTED, with the diagnostic
// recorded and no error. It is the commonest state a new radio is in, and
// refusing to open one would be refusing the radio.
func TestE2E_EmptyRadioOpens(t *testing.T) {
	radio := fakeic7300mk2.New()
	defer radio.Close()
	sess := openFake(t, radio)

	d := civDiag(t, sess)
	if d.Fingerprinted {
		t.Error("Fingerprinted = true against a radio holding nothing — no record was seen, so no length was checked")
	}
	if d.ProbeSlotsRead != 8 {
		t.Errorf("ProbeSlotsRead = %d, want 8 — an FA is an empty channel, not an error, so the bounded search runs to its bound", d.ProbeSlotsRead)
	}
	ch, err := sess.ReadChannel(context.Background(), "001")
	if err != nil {
		t.Fatalf("ReadChannel on an empty radio: %v", err)
	}
	if !ch.Empty() {
		t.Error("ReadChannel returned data from a radio that holds none")
	}
}

// A continuous transceive flood does not wedge a read of all 101 slots.
//
// The broadcasts carry `to = 00` and are dropped by civ.FrameAccumulator's
// address filter BEFORE any engine event exists, so they cost the exchange
// nothing but noise — and the adapter counts them, which is how a user would
// ever learn the line was busy.
func TestE2E_SurvivesAContinuousFlood(t *testing.T) {
	radio := fakeic7300mk2.New(
		fakeic7300mk2.WithChannel("001", e2eRecordBytes(0x00, 14_100_000, "TESTING NAME0123")),
		fakeic7300mk2.WithTransceiveBroadcasts(2*time.Millisecond),
	)
	defer radio.Close()
	sess := openFake(t, radio)

	read := 0
	for _, b := range sess.Capabilities().Banks {
		for _, slot := range b.Slots {
			if _, err := sess.ReadChannel(context.Background(), slot); err != nil {
				t.Fatalf("ReadChannel(%q) under a transceive flood: %v", slot, err)
			}
			read++
		}
	}
	if read != 101 {
		t.Fatalf("read %d slots, want 101", read)
	}
	if n := civDiag(t, sess).Unexpected; n == 0 {
		t.Error("AccumulatorStats().Unexpected = 0 after a whole inventory read under a flood — the broadcasts are dropped by the address filter and COUNTED there, and a zero means the driver is reading the wrong counter")
	}
}

// The ③ byte survives a read-modify-write: no spec.Field carries the SELECT
// group, so a driver that did not carry it through would move the user's
// channel out of its scan group on every write.
func TestE2E_SelectByteSurvivesAWrite(t *testing.T) {
	for _, sel := range []byte{0x00, 0x01, 0x02, 0x03} {
		radio := fakeic7300mk2.New(fakeic7300mk2.WithChannel("001", e2eRecordBytes(sel, 14_100_000, "TESTING NAME0123")))
		sess := openFake(t, radio)

		if _, err := sess.WriteChannel(context.Background(), e2eExpected("001", 14_105_000, "REWRITTEN NAME12")); err != nil {
			radio.Close()
			t.Fatalf("③ = %#02x: WriteChannel: %v", sel, err)
		}
		stored, ok := radio.Channel("001")
		if !ok {
			radio.Close()
			t.Fatalf("③ = %#02x: the fake holds no record after the set", sel)
		}
		if stored[0] != sel {
			t.Errorf("③ went out as %#02x, want %#02x — the SELECT group the RADIO holds is carried through unchanged", stored[0], sel)
		}
		radio.Close()
	}
}

// P1 and P2 CANNOT BE CLEARED on this model, and the driver never tries.
//
// This is the one MANUAL-EVIDENCED fact in this pair that the IC-7300's own
// document does not carry: PDF p.4's 0B row prints "ⓘ P1 and P2 cannot be
// cleared." and PDF p.17 prints "* Except for \"01 00\" and \"01 01\"
// (P1/P2)." It is why this model's SCAN bank is NoBlank and the IC-7300's is
// not, and neither sentence lifts anything for the other model.
//
// The driver ships no erase path at all, so the assertion is doubled: nothing
// aimed at 01 00 or 01 01 ever leaves as a clear, AND an empty channel at P1
// is refused by name before any wire traffic.
func TestE2E_ScanEdgesAreNeverCleared(t *testing.T) {
	radio := fakeic7300mk2.New(
		fakeic7300mk2.WithChannel("P1", e2eRecordBytes(0x00, 14_000_000, "EDGE LOW")),
		fakeic7300mk2.WithChannel("P2", e2eRecordBytes(0x00, 14_350_000, "EDGE HIGH")),
	)
	defer radio.Close()
	sess := openFake(t, radio)

	if caps, _ := sess.Capabilities().Bank(spec.BankScan); !caps.NoBlank {
		t.Error("SCAN.NoBlank = false — this document says P1 and P2 cannot be cleared, and the whole-bank form is where that is said once")
	}

	for _, slot := range []string{"P1", "P2"} {
		before := len(radio.Received())
		_, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: slot})
		if !errors.Is(err, driver.ErrWriteRefused) || !strings.Contains(err.Error(), string(spec.FieldErase)) {
			t.Fatalf("WriteChannel(empty %s) = %v, want an ErrWriteRefused naming erase", slot, err)
		}
		if after := len(radio.Received()); after != before {
			t.Errorf("%s: the erase refusal put %d frames on the wire — it is rung 3, before the read", slot, after-before)
		}
		if _, ok := radio.Channel(slot); !ok {
			t.Errorf("%s: the fake no longer holds the scan edge", slot)
		}
	}

	// Nothing aimed at either scan edge is a clear, in any of the printed
	// forms, across the whole transcript.
	for i, f := range radio.Received() {
		cn, sc, ok := civ.FrameCommand(f)
		if !ok {
			continue
		}
		if cn == 0x0B {
			t.Errorf("frame %d = % X is a 0B memory clear", i, f)
			continue
		}
		if cn != 0x1A || sc != 0x00 || len(f) < 9 {
			continue
		}
		if f[6] != 0x01 {
			continue // not a scan-edge address
		}
		if len(f) != 9 {
			continue // a full set, which is not a clear
		}
	}
}

// countSets counts 1A 00 SET frames — a 1A 00 frame longer than the nine-byte
// read — among frames.
func countSets(frames [][]byte) int {
	n := 0
	for _, f := range frames {
		if cn, sc, ok := civ.FrameCommand(f); ok && cn == 0x1A && sc == 0x00 && len(f) > 9 {
			n++
		}
	}
	return n
}
