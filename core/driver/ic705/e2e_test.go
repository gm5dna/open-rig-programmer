// SPDX-License-Identifier: GPL-3.0-or-later

package ic705_test

import (
	"context"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic705"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic705"
)

// TWO INDEPENDENT READINGS OF ONE MANUAL, PAIRED.
//
// internal/fakeic705 was written by a different implementer from a brief,
// behind a wall that excluded this driver, core/civ's encoder,
// core/civ/ic705's profile and the golden vectors. It re-derived the
// record length from the transcripts on its own (115 diagram positions
// less 4 address bytes) and carried the transcripts' four unresolved STOPs
// forward rather than guessing past them. So a failure in this file is a
// genuine disagreement between two readings of the same document, and the
// way to resolve one is against the manual — never by making either side
// match the other by inspection.
//
// THE PACKAGE IS ic705_test, EXTERNAL, and that is load-bearing twice
// over. internal/guards' TestSimulatedProfileTokensConfinement constrains
// only NON-test files, so ic705.Simulated may be named here and nowhere
// else this wave; and an external package can reach only the driver's
// EXPORTED surface, so this file is also the proof that a real consumer
// needs nothing more than Open, ReadChannel, WriteChannel, Capabilities,
// Diagnostics and SessionInfo. The one exception is the pacing bridge
// (export_test.go), which removes a 20 ms sleep and no behaviour.
//
// WHAT THE FAKE'S FIXTURES ASSERT, AND WHAT THEY DO NOT: fakeic705's
// BlankRecord is 111 ZERO bytes, chosen because zero asserts nothing about
// a record this project has never read. It is therefore NOT a channel this
// driver can decode — record offset 7 is the filter, whose printed
// vocabulary is 1..3 and has no 0 — so a test that needs a READABLE
// channel seeds the manual's own worked example instead (goldenRecord
// below), and one that needs only an OCCUPIED slot uses the fake's blank.
// That division is a fact about what each fixture claims, not a defect in
// either side: the fake's records are opaque byte strings by design.

// display → wire, restated here BY HAND rather than borrowed from the
// driver's own unexported helper. The rule both implementations must agree
// on is wire = display − 1, for group and channel alike, in both banks;
// a test that asked the driver to convert would be asking the code under
// test to confirm itself.
func wireOf(t *testing.T, slot string) (group, channel int) {
	t.Helper()
	g, c, ok := spec.ParseSparseSlot(slot)
	if !ok {
		t.Fatalf("%q is not a canonical group-addressed slot", slot)
	}
	if g == 101 {
		return 100, c - 1 // the CALL group, printed 0100
	}
	return g - 1, c - 1
}

// goldenRecord returns the 111 record bytes of the manual's own
// `set-record-name-with-space` worked example, optionally with the three
// DV call-sign areas (and their TX-block copies) blanked to 0x20.
//
// THE MANUAL'S EVIDENCE, NOT EITHER SIDE'S ENCODER. Seeding the fake with
// bytes this driver produced would prove only that the driver round-trips
// itself; these bytes were transcribed from the CI-V Reference Guide by a
// leg that wrote no code, and they are frozen by a compiled-in SHA-256
// manifest in core/civ/ic705.
//
// AS TRANSCRIBED the record carries CQCQCQ in the UR call sign — an area
// no spec.Field claims and this tier's template fixes at 0x20 — so a write
// to a slot holding it is REFUSED under enabler E6. Blanked, it is an
// ordinary FM channel and the write path this tier actually ships.
func goldenRecord(t *testing.T, blankCallSigns bool) []byte {
	t.Helper()
	raw, err := os.ReadFile("../../civ/ic705/testdata/vectors.golden")
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
		rec := append([]byte(nil), frame[10:len(frame)-1]...)
		if len(rec) != fakeic705.RecordLen {
			t.Fatalf("the manual's record is %d bytes and the fake accepts %d — the two implementations disagree about the record length itself, which is a STOP for the orchestrator, not something to paper over here", len(rec), fakeic705.RecordLen)
		}
		if blankCallSigns {
			for _, r := range [][2]int{{24, 47}, {71, 94}} {
				for i := r[0]; i <= r[1]; i++ {
					rec[i] = 0x20
				}
			}
		}
		return rec
	}
	t.Fatal("the set-record-name-with-space vector is missing from the frozen file")
	return nil
}

// imageHolding builds a factory image with the given record at each of the
// given DISPLAY slots.
func imageHolding(t *testing.T, record []byte, slots ...string) fakeic705.Image {
	t.Helper()
	img := fakeic705.EmptyImage()
	for _, slot := range slots {
		g, c := wireOf(t, slot)
		img = img.With(g, c, record)
	}
	return img
}

// openFake opens a session against a fake radio and registers both Closes.
func openFake(t *testing.T, radio *fakeic705.Radio, profile ic705.Profile, opts ...ic705.Option) *ic705.Session {
	t.Helper()
	sess, err := openFakeErr(t, radio, profile, opts...)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return sess
}

func openFakeErr(t *testing.T, radio *fakeic705.Radio, profile ic705.Profile, opts ...ic705.Option) (*ic705.Session, error) {
	t.Helper()
	t.Cleanup(func() { _ = radio.Close() })
	opts = append(opts, ic705.WithNoPacingForTest())
	d := ic705.New(profile, opts...)
	sess, err := d.Open(context.Background(), radio.Port(), driver.Identity{Port: "/dev/fake", USBSerial: "FAKE-1"})
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess.(*ic705.Session), nil
}

// memSlots returns a session's materialised memory-bank slot list.
func memSlots(t *testing.T, sess *ic705.Session) []string {
	t.Helper()
	b, ok := sess.Capabilities().Bank(spec.BankMemory)
	if !ok {
		t.Fatal("the session has no MEM bank")
	}
	return b.Slots
}

func TestFakeSession_ProbeFingerprintsAndReadsAndWrites(t *testing.T) {
	record := goldenRecord(t, true)
	radio := fakeic705.New(fakeic705.WithFactoryImage(imageHolding(t, record, "G01-001")))
	d := ic705.New(ic705.RealHardware, ic705.WithConsentedUnverifiedWrites(), ic705.WithNoPacingForTest())
	t.Cleanup(func() { _ = radio.Close() })
	opened, err := d.Open(context.Background(), radio.Port(), driver.Identity{Port: "/dev/fake"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = opened.Close() })
	sess := opened.(*ic705.Session)

	// 1. THE PROBE FINGERPRINT. Identity().CATID begins with the DRIVER'S
	//    OWN Capabilities().CATID, read from the driver rather than
	//    written as a literal: a crossed table row fails here.
	want := d.Capabilities().CATID
	if !strings.HasPrefix(sess.Identity().CATID, want) {
		t.Errorf("Identity().CATID = %q, want a value beginning with the driver's own %q", sess.Identity().CATID, want)
	}
	if !sess.SessionInfo().Fingerprinted {
		t.Error("the session is not fingerprinted although the fake's first memory holds a 111-byte record")
	}
	if sess.SessionInfo().IDToken == "" {
		t.Error("no ID token was recorded from the fake's 19 00 answer")
	}

	// 2. A READ OF A POPULATED SLOT, in the field shape the read path
	//    documents: three fields Unavailable because this record has no
	//    such field, and every mapped tier field Known.
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data == nil {
		t.Fatal("a populated slot came back empty")
	}
	for _, f := range []struct {
		name  string
		state codeplug.FieldState
	}{
		{"CTCSSTone", ch.Data.CTCSSTone.State},
		{"TagDisplay", ch.Data.TagDisplay.State},
		{"ScanSkip", ch.Data.ScanSkip.State},
	} {
		if f.state != codeplug.Unavailable {
			t.Errorf("%s.State = %q, want Unavailable", f.name, f.state)
		}
	}
	for _, f := range []struct {
		name  string
		state codeplug.FieldState
	}{
		{"TxFreqHz", ch.Data.TxFreqHz.State},
		{"Duplex", ch.Data.Duplex.State},
		{"OffsetHz", ch.Data.OffsetHz.State},
		{"ToneMode", ch.Data.ToneMode.State},
		{"ToneTx", ch.Data.ToneTx.State},
		{"ToneRx", ch.Data.ToneRx.State},
		{"DTCSCode", ch.Data.DTCSCode.State},
		{"DTCSPolarity", ch.Data.DTCSPolarity.State},
		{"Filter", ch.Data.Filter.State},
		{"DataMode", ch.Data.DataMode.State},
	} {
		// Every mapped tier field is Known here because the manual's own
		// record carries a real value in each — 88.5 Hz in both tone
		// spans included. (The plan's REV 2 wording for this bullet said
		// "tones included, even at 0.0 Hz"; ruling T1 in REV 3 replaced
		// that: a wire zero is OUTSIDE the declared tone domain and reads
		// back Unknown, which TestFakeSession_ToneOffRoundTripsThroughThe
		// Fake below exercises against this same fake.)
		if f.state != codeplug.Known {
			t.Errorf("%s.State = %q, want Known", f.name, f.state)
		}
	}

	// 3. A READ OF AN EMPTY SLOT is an EMPTY CHANNEL, not an error: the
	//    fake answers NG for an unwritten channel, and clone.ReadAll must
	//    be able to walk past one.
	empty, err := sess.ReadChannel(context.Background(), "G01-050")
	if err != nil {
		t.Fatalf("ReadChannel of an unwritten slot: %v", err)
	}
	if empty.Slot != "G01-050" || empty.Data != nil {
		t.Errorf("got %+v, want an empty channel labelled G01-050", empty)
	}

	// 4. A WRITE INTO AN EMPTY SLOT — a CREATE, carrying explicit values
	//    for every mapped field INCLUDING both tones, because a create has
	//    no prior record to preserve a tone from and this radio's manual
	//    prints no default (O-11).
	setsBefore := radio.SetsSeen()
	create := codeplug.Channel{Slot: "G01-050", Data: new(codeplug.ChannelData)}
	*create.Data = *ch.Data
	create.Data.Tag = "MY REPEATER CH01"
	res, err := sess.WriteChannel(context.Background(), create)
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	if len(res.Steps) != 1 || res.Steps[0].Command != "1A 00" {
		t.Fatalf("Steps = %+v, want exactly one \"1A 00\"", res.Steps)
	}
	if !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("Steps[0] = %+v, want Sent and Confirmed — the fake answered FB", res.Steps[0])
	}
	if got := radio.SetsSeen() - setsBefore; got != 1 {
		t.Errorf("the fake saw %d memory sets, want exactly 1", got)
	}

	// 5. READ BACK, FIELD BY FIELD, including the name with its two
	//    interior spaces — the vector's own name, which is why it is the
	//    vector this project froze.
	back, err := sess.ReadChannel(context.Background(), "G01-050")
	if err != nil {
		t.Fatalf("ReadChannel after the write: %v", err)
	}
	if back.Data == nil {
		t.Fatal("the created channel reads back empty")
	}
	if back.Data.Tag != "MY REPEATER CH01" {
		t.Errorf("Tag = %q, want \"MY REPEATER CH01\" — both interior spaces included", back.Data.Tag)
	}
	if back.Data.FreqHz != ch.Data.FreqHz {
		t.Errorf("FreqHz = %d, want %d", back.Data.FreqHz, ch.Data.FreqHz)
	}
	if back.Data.Mode != ch.Data.Mode {
		t.Errorf("Mode = %q, want %q", back.Data.Mode, ch.Data.Mode)
	}
	if back.Data.ToneTx != ch.Data.ToneTx || back.Data.ToneRx != ch.Data.ToneRx {
		t.Errorf("tones came back %+v/%+v, want %+v/%+v", back.Data.ToneTx, back.Data.ToneRx, ch.Data.ToneTx, ch.Data.ToneRx)
	}
	if back.Data.OffsetHz != ch.Data.OffsetHz || back.Data.Duplex != ch.Data.Duplex {
		t.Errorf("offset/duplex came back %+v/%+v", back.Data.OffsetHz, back.Data.Duplex)
	}
	if back.Data.Filter != ch.Data.Filter || back.Data.DTCSCode != ch.Data.DTCSCode || back.Data.DTCSPolarity != ch.Data.DTCSPolarity {
		t.Errorf("filter/DTCS came back %+v/%+v/%+v", back.Data.Filter, back.Data.DTCSCode, back.Data.DTCSPolarity)
	}
	if back.Data.DataMode != ch.Data.DataMode {
		t.Errorf("DataMode = %+v, want %+v", back.Data.DataMode, ch.Data.DataMode)
	}
}

func TestFakeSession_InventoryIsMaterialisedAndCloneReadsIt(t *testing.T) {
	// THE TEST WHOSE ABSENCE LET THE DEFECT THROUGH. core/clone's ReadAll
	// iterates Capabilities().Banks[i].Slots and nothing else, and a
	// sparse bank's Slots is what a read MATERIALISED — so without the
	// inventory walk this returns the four call channels and ZERO
	// memories.
	record := goldenRecord(t, true)
	img := imageHolding(t, record,
		"G01-001", "G01-050", "G07-013",
		"G101-001", "G101-002", "G101-003", "G101-004")
	radio := fakeic705.New(fakeic705.WithFactoryImage(img))
	sess := openFake(t, radio, ic705.Simulated)

	if got := strings.Join(memSlots(t, sess), ","); got != "G01-001,G01-050,G07-013" {
		t.Errorf("the session's MEM bank lists %q, want the three memories in ascending display order", got)
	}
	svc := clone.NewService(sess, clone.SnapshotStore{Dir: t.TempDir()})
	cp, err := svc.ReadAll(context.Background())
	if err != nil {
		t.Fatalf("clone.ReadAll: %v", err)
	}
	if len(cp.Channels) != 7 {
		var slots []string
		for _, c := range cp.Channels {
			slots = append(slots, c.Slot)
		}
		t.Fatalf("ReadAll returned %d channels (%v), want 7 — three memories and four call channels", len(cp.Channels), slots)
	}
	for _, c := range cp.Channels {
		if c.Data == nil {
			t.Errorf("slot %s came back empty from a radio that holds a record there", c.Slot)
		}
	}
	if issues := codeplug.Validate(cp, sess.Capabilities()); len(issues) != 0 {
		for _, i := range issues {
			t.Errorf("codeplug.Validate: [%v] slot %q field %q: %s", i.Severity, i.Slot, i.Field, i.Msg)
		}
	}
}

func TestFakeSession_WrongSiblingIsRefusedByRecordLength(t *testing.T) {
	// The 39-byte record is a FIXTURE and explicitly NOT a claim about any
	// other model — cross-model record-length distinctness is Wave 4's
	// check. The fake serves an operator-seeded record of any length
	// verbatim, which is the ability that makes this testable at all.
	radio := fakeic705.New(fakeic705.WithRecord(0, 0, make([]byte, 39)))
	sess, err := openFakeErr(t, radio, ic705.Simulated)
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open accepted a radio whose memory record is 39 bytes")
	}
	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Fatalf("Open failed with %v, want a wrong-radio refusal", err)
	}
	var wrong *driver.WrongRadioError
	if !errors.As(err, &wrong) {
		t.Fatalf("%v is not a *driver.WrongRadioError", err)
	}
	if wrong.GotModel != "" {
		t.Errorf("GotModel = %q, want EMPTY", wrong.GotModel)
	}
	if wrong.WantModel != "IC-705" || wrong.Want != "A4/111" || wrong.Got != "A4/39" {
		t.Errorf("the refusal reads %+v, want IC-705 / A4/111 / A4/39", wrong)
	}
}

func TestFakeSession_EraseIsRefused(t *testing.T) {
	radio := fakeic705.New(fakeic705.WithFactoryImage(imageHolding(t, goldenRecord(t, true), "G01-001")))
	sess := openFake(t, radio, ic705.Simulated)
	framesBefore := radio.FramesSeen()

	res, err := sess.WriteChannel(context.Background(), codeplug.Channel{Slot: "G01-001"})
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel of an empty channel returned %v, want a refusal", err)
	}
	var refused *driver.WriteRefusedError
	if !errors.As(err, &refused) || len(refused.Fields) != 1 || refused.Fields[0] != spec.FieldErase {
		t.Errorf("the refusal %v does not name exactly the erase field", err)
	}
	if len(res.Steps) != 0 {
		t.Errorf("Steps = %+v, want empty", res.Steps)
	}
	if radio.FramesSeen() != framesBefore {
		t.Error("the erase refusal reached the radio — it happens before any traffic")
	}
}

func TestFakeSession_DVRoutedChannelIsRefusedNotCorrupted(t *testing.T) {
	// The record AS TRANSCRIBED carries CQCQCQ in the UR call sign. No
	// spec.Field claims that area and this tier's template fixes it at
	// 0x20, so enabler E6 rules: refuse with the reason named, never
	// rewrite.
	routed := goldenRecord(t, false)
	radio := fakeic705.New(fakeic705.WithFactoryImage(imageHolding(t, routed, "G01-001")))
	sess := openFake(t, radio, ic705.Simulated)

	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	ch.Data.Tag = "RENAME ME"
	setsBefore := radio.SetsSeen()
	if _, err := sess.WriteChannel(context.Background(), ch); !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel returned %v, want a preservation refusal", err)
	} else if !strings.Contains(err.Error(), "call sign") {
		t.Errorf("the refusal %q does not name the DV routing it is protecting", err)
	}
	if radio.SetsSeen() != setsBefore {
		t.Error("the refusal reached the radio as a set")
	}
	// THE RADIO'S OWN BYTES ARE STILL THERE, proven from the fake's state
	// rather than inferred from the refusal.
	g, c := wireOf(t, "G01-001")
	stored, occupied := radio.SlotState(g, c)
	if !occupied {
		t.Fatal("the slot is no longer occupied")
	}
	if string(stored[24:30]) != "CQCQCQ" {
		t.Errorf("the UR call sign now reads %q — it must be untouched", stored[24:30])
	}
}

func TestFakeSession_EmptyRadioOpensUnfingerprinted(t *testing.T) {
	// A radio nobody has ever programmed, plus ONE wrong-length record in
	// a group the bounded walk never reaches — so the session opens on
	// address evidence alone, and the length fingerprint is still applied
	// to every later read.
	radio := fakeic705.New(fakeic705.WithRecord(11, 0, make([]byte, 39)))
	sess := openFake(t, radio, ic705.Simulated)

	if sess.SessionInfo().Fingerprinted {
		t.Error("a radio with no reachable record reported itself fingerprinted")
	}
	if got := memSlots(t, sess); len(got) != 0 {
		t.Errorf("the materialised MEM slot list is %v, want empty", got)
	}
	ch, err := sess.ReadChannel(context.Background(), "G12-001")
	if err == nil {
		t.Fatalf("a 39-byte record read back as %+v — the fingerprint must be continuous", ch)
	}
	if !strings.Contains(err.Error(), "111") {
		t.Errorf("the read failed with %v, which does not name this profile's declared length", err)
	}
}

func TestFakeSession_ToneOffRoundTripsThroughTheFake(t *testing.T) {
	// T1(3) and T1(4) end to end, against the independent radio: the wire
	// zero this radio uses for "no tone set" is OUTSIDE the declared tone
	// domain, so it reads back Unknown — and the write does not refuse it,
	// it copies the radio's own number back out verbatim.
	// BOTH COPIES, because the record carries its RX fields twice: the
	// duplicated TX block mirrors offsets 1..47 forty-seven bytes further
	// on, and civ refuses a record whose copies disagree. Zeroing only the
	// first set produces a record the RADIO could never have written, and
	// the parse failure that follows is the layout being right rather than
	// the fake being wrong.
	record := goldenRecord(t, true)
	const txBlockDelta = 47
	for i := 11; i <= 16; i++ {
		record[i] = 0x00              // the two three-byte tone spans
		record[i+txBlockDelta] = 0x00 // and their copies
	}
	record[9] &= 0xF0              // tone mode OFF in the low nibble
	record[9+txBlockDelta] &= 0xF0 // and in its copy
	radio := fakeic705.New(fakeic705.WithFactoryImage(imageHolding(t, record, "G01-001")))
	sess := openFake(t, radio, ic705.Simulated)

	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Data.ToneTx.State != codeplug.Unknown || ch.Data.ToneRx.State != codeplug.Unknown {
		t.Fatalf("the tones read back %+v/%+v, want Unknown for the wire zero", ch.Data.ToneTx, ch.Data.ToneRx)
	}
	ch.Data.Tag = "TONE OFF"
	if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("a tone-OFF channel was not writable: %v", err)
	}
	g, c := wireOf(t, "G01-001")
	stored, _ := radio.SlotState(g, c)
	for i := 11; i <= 16; i++ {
		if stored[i] != 0x00 {
			t.Errorf("record offset %d went out as %#02x, want 0x00 — byte-identical to what the radio held", i, stored[i])
		}
	}
}

func TestFakeSession_BroadcastFloodOpensCleanlyAndIsCounted(t *testing.T) {
	// R9-SPLIT, half one. A BROADCAST flood is addressed to 00, so the
	// CI-V accumulator drops every frame before the engine sees one: Init
	// succeeds NORMALLY, and the traffic shows up only in the adapter's
	// own counter. Against an engine-only counter this reads zero, which
	// is the point.
	radio := fakeic705.New(
		fakeic705.WithFactoryImage(imageHolding(t, goldenRecord(t, true), "G01-001")),
		fakeic705.WithNeverQuiet(),
	)
	sess := openFake(t, radio, ic705.Simulated)

	if sess.SessionInfo().InitDrainCapExceeded {
		t.Error("a broadcast flood tripped the drain cap — those frames are supposed to be dropped on address before any drain sees them")
	}
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel under a broadcast flood: %v", err)
	}
	if ch.Data == nil || ch.Data.Tag != "MY REPEATER CH01" {
		t.Fatalf("the read returned %+v — a flood must not corrupt an answer", ch)
	}
	if got := sess.Diagnostics().UnexpectedFrames; got == 0 {
		t.Error("Diagnostics() reports zero unexpected frames on a line saturated with broadcasts")
	}
}

func TestFakeSession_AddressedFloodIsNonfatalAtOpen(t *testing.T) {
	// R9-SPLIT, half two, and the only flood shape that reaches a drain:
	// frames addressed to the CONTROLLER pass the accumulator's address
	// filter and postpone "quiet" until the cap.
	radio := fakeic705.New(
		fakeic705.WithFactoryImage(imageHolding(t, goldenRecord(t, true), "G01-001")),
		fakeic705.WithAddressedFlood(),
	)
	sess := openFake(t, radio, ic705.Simulated)

	// (a) NONFATAL AT OPEN, with the flood recorded. Transceive is
	// factory-ON on this radio and this tier ships no off-switch, so a
	// line that never goes quiet is a normal operating state at open.
	if !sess.SessionInfo().InitDrainCapExceeded {
		t.Fatal("Init did not report a drain-cap failure under a controller-addressed flood")
	}
	ch, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel under an addressed flood: %v", err)
	}

	// (b) A LATER QUARANTINE DRAIN STILL FAILS CLOSED. The write itself
	// succeeds — the FB arrives — but the post-write quarantine cannot
	// drain a line that never goes quiet, so the engine is left suspect
	// and the NEXT exchange refuses to transmit rather than pretending the
	// stream is clean.
	ch.Data.Tag = "FLOODED"
	if _, err := sess.WriteChannel(context.Background(), ch); err != nil {
		t.Fatalf("WriteChannel under an addressed flood: %v", err)
	}
	_, err = sess.ReadChannel(context.Background(), "G01-001")
	if !errors.Is(err, transport.ErrQuarantineFailed) {
		t.Errorf("the exchange after the write returned %v, want ErrQuarantineFailed — every quarantine after Init stays fail-closed", err)
	}
}

func TestFakeSession_WrongChannelAnswerIsCaught(t *testing.T) {
	// RULING T2, END TO END, AND THE MANDATORY PER-DRIVER REGRESSION. The
	// landed memory-answer matcher is ENVELOPE-ONLY by decision, so it
	// ACCEPTS this answer: the driver's own address check is the only
	// thing between another channel's contents and a caller who asked for
	// this one. Arming a fake to misbehave is the only way to prove the
	// check fires, which is why the fake has the hook at all.
	record := goldenRecord(t, true)
	img := imageHolding(t, record, "G01-012", "G01-100")
	radio := fakeic705.New(fakeic705.WithFactoryImage(img))
	sess := openFake(t, radio, ic705.Simulated)

	before := sess.SessionInfo().AnswerMismatches
	radio.AnswerNextReadWithAddress(0, 99) // display G01-100
	ch, err := sess.ReadChannel(context.Background(), "G01-012")
	if !errors.Is(err, ic705.ErrAnswerMismatch) {
		t.Fatalf("ReadChannel returned (%+v, %v), want ErrAnswerMismatch", ch, err)
	}
	var mismatch *ic705.AnswerMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("%v is not an *ic705.AnswerMismatchError", err)
	}
	if mismatch.Requested.Group != 0 || mismatch.Requested.Channel != 11 {
		t.Errorf("the error names requested %v, want wire group 0 channel 11", mismatch.Requested)
	}
	if mismatch.Answered.Group != 0 || mismatch.Answered.Channel != 99 {
		t.Errorf("the error names answered %v, want wire group 0 channel 99", mismatch.Answered)
	}
	if ch.Data != nil || ch.Slot != "" {
		t.Errorf("a channel came back alongside the mismatch: %+v", ch)
	}
	if got := sess.SessionInfo().AnswerMismatches; got != before+1 {
		t.Errorf("AnswerMismatches = %d, want %d", got, before+1)
	}
}

func TestFakeSession_OccupiedSurpriseAddIsRefused(t *testing.T) {
	// RULING T3, with the fixture the ruling names. G11-001 is display
	// group 11 — outside the bounded walk's ten — so the materialised
	// inventory does not know it, and the radio holds a record there.
	record := goldenRecord(t, true)
	img := imageHolding(t, record, "G01-001", "G11-001")

	// The SAME radio image, twice over: a session takes ownership of the
	// port and closes it, so the second walk needs its own radio built
	// from a clone of the same fixture.
	bounded := fakeic705.New(fakeic705.WithFactoryImage(img.Clone()))
	sess := openFake(t, bounded, ic705.Simulated)

	seed, err := sess.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if got := memSlots(t, sess); strings.Join(got, ",") != "G01-001" {
		t.Fatalf("the bounded walk materialised %v — the fixture is not testing what it claims", got)
	}

	surprise := codeplug.Channel{Slot: "G11-001", Data: new(codeplug.ChannelData)}
	*surprise.Data = *seed.Data
	surprise.Data.Tag = "OVERWRITE"
	setsBefore := bounded.SetsSeen()
	res, err := sess.WriteChannel(context.Background(), surprise)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel returned %v, want an occupied-surprise refusal", err)
	}
	if !strings.Contains(err.Error(), "WithFullInventoryWalk") {
		t.Errorf("the refusal %q does not name the remedy", err)
	}
	if len(res.Steps) != 0 || bounded.SetsSeen() != setsBefore {
		t.Error("the refusal sent something")
	}
	g, c := wireOf(t, "G11-001")
	stored, occupied := bounded.SlotState(g, c)
	if !occupied || strings.TrimRight(string(stored[95:111]), " ") == "OVERWRITE" {
		t.Error("the seeded record at G11-001 was overwritten")
	}

	// THE REMEDY WORKS. With WithFullInventoryWalk() the same image
	// materialises the slot, and the write becomes an ordinary modify.
	full := fakeic705.New(fakeic705.WithFactoryImage(img.Clone()))
	fullSess := openFake(t, full, ic705.Simulated, ic705.WithFullInventoryWalk())
	if got := strings.Join(memSlots(t, fullSess), ","); got != "G01-001,G11-001" {
		t.Fatalf("the full walk materialised %q, want both records", got)
	}
	if _, err := fullSess.WriteChannel(context.Background(), surprise); err != nil {
		t.Fatalf("the write was still refused after a full walk: %v", err)
	}
	stored, _ = full.SlotState(g, c)
	if got := strings.TrimRight(string(stored[95:111]), " "); got != "OVERWRITE" {
		t.Errorf("the radio holds %q at G11-001, want the written name", got)
	}
}

func TestFakeSession_UnsolicitedFrameIsToldApartByItsAddressAlone(t *testing.T) {
	// The fake's unsolicited frame carries an INVENTED command — the brief
	// gave it no unsolicited form to copy, because the manual prints none
	// — so this driver may assert on its `to` byte and NOTHING else. It
	// does exactly that, structurally: the accumulator drops a frame whose
	// `to` is not this controller's address, and the engine never sees
	// one. This test pins that the DRIVER never grew an opinion about the
	// frame's contents, by proving a session works normally while a
	// stream of them arrives.
	radio := fakeic705.New(
		fakeic705.WithFactoryImage(imageHolding(t, goldenRecord(t, true), "G01-001")),
		fakeic705.WithBroadcastEvery(time.Millisecond),
	)
	sess := openFake(t, radio, ic705.Simulated)

	for i := 0; i < 3; i++ {
		ch, err := sess.ReadChannel(context.Background(), "G01-001")
		if err != nil {
			t.Fatalf("read %d under unsolicited traffic: %v", i, err)
		}
		if ch.Data == nil {
			t.Fatalf("read %d came back empty", i)
		}
	}
	if got := sess.Diagnostics().UnexpectedFrames; got == 0 {
		t.Error("the adapter counted none of the unsolicited frames")
	}
}
