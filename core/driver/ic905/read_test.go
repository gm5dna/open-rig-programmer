// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// populate seeds img with the golden record at every listed wire address.
func populate(img *radioImage, addrs ...wireAddr) {
	if img.records == nil {
		img.records = map[wireAddr][]byte{}
	}
	for _, a := range addrs {
		img.records[a] = goldenRecord(144_500_000, 5).build()
	}
}

// memBankSlots returns a session's materialised MEM slots.
func memBankSlots(t *testing.T, s *Session) []string {
	t.Helper()
	mem, ok := s.Capabilities().Bank(spec.BankMemory)
	if !ok {
		t.Fatal("the session's capabilities have no MEM bank")
	}
	return mem.Slots
}

// TestOpen_BoundedWalkIsTheDefaultAndSaysSoInDiagnostics.
//
// The default walk reads group 0's hundred channels, then CHANNEL 00 ONLY
// of groups 1…99, descending into a group's remaining channels only where
// channel 00 answered with a record — and it reports InventoryComplete
// FALSE whatever it found, because it genuinely did not look at 9,801 of
// the 10,000 addresses. AN EARLY OR BOUNDED STOP MUST NEVER BE READABLE
// AS AN EMPTY RADIO.
func TestOpen_BoundedWalkIsTheDefaultAndSaysSoInDiagnostics(t *testing.T) {
	// PARALLEL because this package's tests are wire-paced, not
	// CPU-bound: transport.Engine applies a 20 ms settle after every
	// exchange, so an Open spends seconds asleep and several can overlap
	// at no cost. See TestOpen_FullWalkIsOptInAndReportsComplete for the
	// one that makes it worth doing.
	t.Parallel()
	var img radioImage
	img.idToken = testToken
	// G01-001 and G01-050 are in group 0, which the bounded walk reads in
	// full. G06-038 is in group 5, whose channel 00 is EMPTY, so the
	// bounded walk's one probe there answers FA and the whole group is
	// skipped — the exact hole ruling T3's write refusal exists for.
	populate(&img, wireAddr{0, 0}, wireAddr{0, 49}, wireAddr{5, 37})

	_, s := openFor(t, img)

	if got, want := memBankSlots(t, s), []string{"G01-001", "G01-050"}; !slices.Equal(got, want) {
		t.Errorf("materialised slots = %v, want %v — the bounded walk sees group 0 in full and misses a group whose channel 00 is empty", got, want)
	}
	if s.Diagnostics905().InventoryComplete {
		t.Error("InventoryComplete is true after a BOUNDED walk — the default walk covers 199 of 10,000 addresses and must say so")
	}
}

// TestOpen_FullWalkIsOptInAndReportsComplete: the same radio, opened
// WithFullInventoryWalk, finds the channel the bounded walk missed and is
// the only walk entitled to call itself complete.
func TestOpen_FullWalkIsOptInAndReportsComplete(t *testing.T) {
	t.Parallel()
	var img radioImage
	img.idToken = testToken
	populate(&img, wireAddr{0, 0}, wireAddr{5, 37})

	p := newRespondingPort(t, img)
	sess, err := New(RealHardware, WithFullInventoryWalk()).
		Open(context.Background(), p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	s := sess.(*Session)

	if got, want := memBankSlots(t, s), []string{"G01-001", "G06-038"}; !slices.Equal(got, want) {
		t.Errorf("materialised slots = %v, want %v — the full walk covers the whole 100 x 100 space", got, want)
	}
	if !s.Diagnostics905().InventoryComplete {
		t.Error("InventoryComplete is false after a FULL walk that ran to the end")
	}
}

// TestOpen_StopsAtBudgetAndSaysSo.
//
// The budget of 500 is ASSUMED — this document prints no capacity
// anywhere (register ic905.group_budget, lift ic905-R-09) — and the walk
// stops there rather than reading on. The flag is what keeps that stop
// from reading as the whole truth about the radio.
func TestOpen_StopsAtBudgetAndSaysSo(t *testing.T) {
	t.Parallel()
	var img radioImage
	img.idToken = testToken
	img.records = map[wireAddr][]byte{}
	rec := goldenRecord(144_500_000, 5).build()
	// Five full groups: exactly 500 occupied channels, then a sixth
	// group's channel 00 that the walk must never reach.
	for g := 0; g < 5; g++ {
		for c := 0; c < 100; c++ {
			img.records[wireAddr{g, c}] = rec
		}
	}
	img.records[wireAddr{5, 0}] = rec

	_, s := openFor(t, img)

	slots := memBankSlots(t, s)
	if len(slots) != 500 {
		t.Errorf("materialised %d slots, want exactly the budget of 500", len(slots))
	}
	if slices.Contains(slots, "G06-001") {
		t.Error("the walk read past the budget — G06-001 must never have been probed")
	}
	if s.Diagnostics905().InventoryComplete {
		t.Error("InventoryComplete is true after stopping at the budget")
	}
}

// TestOpen_RespectsContextCancellation.
//
// The ctx bound is one of the two load-bearing parts of discovery. A
// cancelled walk STOPS — it does not run to the end, and it does not fail
// the open — and the session it produces says InventoryComplete false, so
// the truncated inventory can never be mistaken for an empty radio by
// anything downstream.
func TestOpen_RespectsContextCancellation(t *testing.T) {
	t.Parallel()
	var img radioImage
	img.idToken = testToken
	populate(&img, wireAddr{0, 0})

	p := newRespondingPort(t, img)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel DETERMINISTICALLY, once the walk is demonstrably under way,
	// rather than after a wall-clock guess: the probe's bounded search is
	// at most 28 reads, so 60 puts us inside discovery on any machine.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if countMemoryReads(p) > 60 {
				cancel()
				return
			}
			select {
			case <-done:
				return
			case <-time.After(2 * time.Millisecond):
			}
		}
	}()

	sess, err := New(RealHardware).Open(ctx, p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v — a cancelled DISCOVERY stops the walk, it does not fail the open", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	s := sess.(*Session)

	if s.Diagnostics905().InventoryComplete {
		t.Error("InventoryComplete is true after a cancelled walk")
	}
	if n := countMemoryReads(p); n >= 211 {
		t.Errorf("the walk made %d memory reads — cancellation must STOP it, not merely be recorded", n)
	}
}

// TestOpen_MaterialisedSlotsAreWhatReadAllWalks is the whole reason
// discovery exists (ruling R12): core/clone.Service.ReadAll walks
// Capabilities().Banks[].Slots and calls ReadChannel per slot, so a
// sparse bank that published nothing would return no memories at all.
//
// It also pins the two namespaces staying apart: the MEM bank publishes
// only "G.." slots, and the CALL bank's twelve "C.." slots are static and
// present whether or not any of them holds anything.
func TestOpen_MaterialisedSlotsAreWhatReadAllWalks(t *testing.T) {
	t.Parallel()
	var img radioImage
	img.idToken = testToken
	populate(&img, wireAddr{0, 2}, wireAddr{callWireGroup, 4})

	_, s := openFor(t, img)
	caps := s.Capabilities()

	mem, _ := caps.Bank(spec.BankMemory)
	if !slices.Equal(mem.Slots, []string{"G01-003"}) {
		t.Errorf("MEM slots = %v, want [G01-003] — and NOT the occupied CALL channel, which belongs to the other bank", mem.Slots)
	}
	call, _ := caps.Bank(spec.BankCall)
	if len(call.Slots) != 12 {
		t.Errorf("CALL has %d slots, want its twelve static ones", len(call.Slots))
	}

	// Every published slot is readable through the seam ReadAll uses.
	for _, slot := range append(append([]string{}, mem.Slots...), call.Slots...) {
		ch, err := s.ReadChannel(context.Background(), slot)
		if err != nil {
			t.Errorf("ReadChannel(%q): %v", slot, err)
			continue
		}
		if ch.Slot != slot {
			t.Errorf("ReadChannel(%q) returned slot %q", slot, ch.Slot)
		}
	}
	// And the effective set still validates with the discovered bank in
	// it, which no Validate run over the static baseline would cover.
	if err := caps.Validate(); err != nil {
		t.Errorf("the effective capabilities do not validate: %v", err)
	}
}

// TestReadChannel_AnFAIsAnEmptyChannel.
//
// An empty channel answers FA, which ReadChannel maps to an EMPTY
// codeplug.Channel (Data nil) and NOT to an error — core/driver's own
// contract, and an error here would abort core/clone's whole ReadAll
// walk. That the answer IS FA is ASSUMED: D5 entry 2(a), lift
// ic905-R-14.
//
// THE BRANCH KEYS ON errors.Is(err, transport.ErrRejected), NEVER ON "AN
// FA FRAME" (ruling T4). Engine.Do CONSUMES the FA and returns
// ErrRejected with NO frame, so a driver that expected a frame back and
// then asked civ.IsRejection about it could never fire at all, leaving
// FA-empty reads, discovery and the pre-write read with no valid branch.
func TestReadChannel_AnFAIsAnEmptyChannel(t *testing.T) {
	t.Parallel()
	var img radioImage
	img.idToken = testToken
	populate(&img, wireAddr{0, 0})

	_, s := openFor(t, img)
	ch, err := s.ReadChannel(context.Background(), "G01-002")
	if err != nil {
		t.Fatalf("ReadChannel of an empty slot returned an error: %v", err)
	}
	if !ch.Empty() {
		t.Errorf("ReadChannel returned %+v, want an EMPTY channel", ch)
	}
	if ch.Slot != "G01-002" {
		t.Errorf("the empty channel carries slot %q, want %q", ch.Slot, "G01-002")
	}
}

// TestReadChannel_AnAllFFRecordIsAnEmptyChannelNotAnError.
//
// AN ALL-FF RECORD IS RECOGNISED BEFORE THE RECORD PARSER, VIA THE
// CODEC'S PRE-PARSE HOOK (ruling R10). Routing it straight through
// ParseMemoryAnswer is IMPOSSIBLE: 0xFF is not a valid packed-BCD pair
// and not a member of any enum this layout declares, so the parse fails
// on the frequency field long before anything useful comes back.
//
// THIS IS A SECOND, SEPARATE ASSUMPTION from the FA one, with its own
// register entry and its own lift: D5 entry 2(b), ic905-R-15. Neither
// guess deletes a channel — an all-FF record carries no field values to
// lose.
func TestReadChannel_AnAllFFRecordIsAnEmptyChannelNotAnError(t *testing.T) {
	t.Parallel()
	var img radioImage
	img.idToken = testToken
	populate(&img, wireAddr{0, 0})
	ff := bytes.Repeat([]byte{0xFF}, 64)
	img.records[wireAddr{0, 1}] = ff

	_, s := openFor(t, img)
	ch, err := s.ReadChannel(context.Background(), "G01-002")
	if err != nil {
		t.Fatalf("ReadChannel of an all-FF record returned an error: %v — an error here aborts core/clone's whole ReadAll walk (R10)", err)
	}
	if !ch.Empty() {
		t.Errorf("ReadChannel returned %+v, want an EMPTY channel", ch)
	}
	// And discovery reads it the same way, so the two cannot disagree
	// about what the radio holds.
	if slices.Contains(memBankSlots(t, s), "G01-002") {
		t.Error("discovery materialised the all-FF slot as occupied — read and walk must recognise empty identically")
	}
}

// TestReadChannel_AMalformedRecordIsStillAnError.
//
// A record that is neither all-FF nor decodable is an ERROR (spec D4): no
// partial parse, no fake Unavailable channel. The pre-parse hook narrows
// the empty case; it does not license swallowing a corrupt one.
func TestReadChannel_AMalformedRecordIsStillAnError(t *testing.T) {
	t.Parallel()
	var img radioImage
	img.idToken = testToken
	populate(&img, wireAddr{0, 0})
	bad := goldenRecord(144_500_000, 5)
	// 0x99 is not in the printed operating-mode table (PDF p.17, folio
	// 16), so the enum decode refuses it — a wrong-VOCABULARY byte rather
	// than a truncated frame, so the answer arrives, matches, and dies in
	// the parser, which is the path a radio saying something this
	// protocol does not define would take.
	bad.mode = 0x99
	img.records[wireAddr{0, 1}] = bad.build()

	_, s := openFor(t, img)
	if _, err := s.ReadChannel(context.Background(), "G01-002"); err == nil {
		t.Fatal("a record with an undefined mode byte was accepted")
	}
}

// TestReadChannel_AnAnswerForAnotherChannelIsAnErrorNotAStore.
//
// civ.MemoryAnswerMatcher is DELIBERATELY ENVELOPE-ONLY — it matches
// to/from/cn/sc and NOT the requested channel address (ruling T2) — so an
// answer for group 0 channel 5 satisfies the matcher for a read of group
// 0 channel 1, and a driver that trusted the matcher would have parsed it
// and stored it under the requested slot.
//
// The comparison happens BEFORE empty recognition, before record mapping,
// before the E6 template check and before any write merge, and the read
// fails with ErrAnswerMismatch while the mismatch is COUNTED.
func TestReadChannel_AnAnswerForAnotherChannelIsAnErrorNotAStore(t *testing.T) {
	t.Parallel()
	var img radioImage
	img.idToken = testToken
	populate(&img, wireAddr{0, 0}, wireAddr{0, 1})
	img.answerAddr = map[wireAddr]wireAddr{{0, 1}: {0, 5}}

	// Discovery would meet the mismatch and fail the open, so this test
	// opens against a clean image and then switches the answer.
	p := newRespondingPort(t, radioImage{idToken: testToken, records: map[wireAddr][]byte{
		{0, 0}: goldenRecord(144_500_000, 5).build(),
		{0, 1}: goldenRecord(144_500_000, 5).build(),
	}})
	sess, err := New(RealHardware).Open(context.Background(), p.Port(), driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	s := sess.(*Session)

	before := s.Diagnostics905().AnswerMismatches
	p.misdirect(wireAddr{0, 1}, wireAddr{0, 5})

	ch, err := s.ReadChannel(context.Background(), "G01-002")
	if err == nil {
		t.Fatalf("ReadChannel returned %+v for an answer about another channel — it must refuse rather than store it under the requested slot", ch)
	}
	if !errors.Is(err, ErrAnswerMismatch) {
		t.Errorf("error = %v, want an ErrAnswerMismatch", err)
	}
	var ame *AnswerMismatchError
	if errors.As(err, &ame) {
		if ame.Requested != (civ.ChannelAddress{Group: 0, Channel: 1}) || ame.Answered != (civ.ChannelAddress{Group: 0, Channel: 5}) {
			t.Errorf("mismatch = %+v, want requested {0 1} answered {0 5}", ame)
		}
	} else {
		t.Errorf("error = %v, want a *AnswerMismatchError carrying BOTH addresses", err)
	}
	if got := s.Diagnostics905().AnswerMismatches; got != before+1 {
		t.Errorf("AnswerMismatches = %d, want %d — the mismatch must be COUNTED, not merely refused", got, before+1)
	}
}

// TestReadChannel_TenGigahertzSurvivesTheCodeplugRoundTrip is the schema-4
// half of D4's byte-identity pin, on the model that forced the widening.
//
// The last step matters most: a second Save must be BYTE-IDENTICAL to the
// first, so a frequency that only schema 4 can hold survives a full
// save/load/save cycle without drifting.
func TestReadChannel_TenGigahertzSurvivesTheCodeplugRoundTrip(t *testing.T) {
	t.Parallel()
	var img radioImage
	img.idToken = testToken
	img.records = map[wireAddr][]byte{
		{0, 0}: goldenRecord(10_250_000_000, 6).build(),
	}

	_, s := openFor(t, img)

	// 1. and 2. the read.
	ch, err := s.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Empty() {
		t.Fatal("ReadChannel returned an empty channel for a populated 10 GHz slot")
	}
	if got := ch.Data.FreqHz; got != 10_250_000_000 {
		t.Fatalf("FreqHz = %d, want 10250000000 — about 2.4 times uint32's ceiling, which is what forced D4's widening", got)
	}

	cp := &codeplug.Codeplug{
		Generator: "ic905 test",
		Radio:     codeplug.RadioInfo{Model: s.Capabilities().Model, CATID: s.Capabilities().CATID},
		Channels:  []codeplug.Channel{ch},
	}

	// 3. and 4. save, and the file says schema 4.
	dir := t.TempDir()
	first := filepath.Join(dir, "first.json")
	if err := codeplug.Save(first, cp); err != nil {
		t.Fatalf("Save: %v", err)
	}
	saved, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("reading the saved file: %v", err)
	}
	if !bytes.Contains(saved, []byte(`"schema": 4`)) {
		t.Errorf("the saved file is not schema 4:\n%s", saved)
	}

	// 5. load, unchanged.
	back, err := codeplug.Load(first)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(back.Channels) != 1 || back.Channels[0].Data == nil {
		t.Fatalf("Load returned %+v", back.Channels)
	}
	if got := back.Channels[0].Data.FreqHz; got != 10_250_000_000 {
		t.Errorf("after a save and a load FreqHz = %d, want 10250000000", got)
	}

	// 6. and saving again is byte-identical.
	second := filepath.Join(dir, "second.json")
	if err := codeplug.Save(second, back); err != nil {
		t.Fatalf("second Save: %v", err)
	}
	again, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("reading the second file: %v", err)
	}
	if !bytes.Equal(saved, again) {
		t.Error("a second save is not byte-identical to the first — the schema-4 round trip is not lossless")
	}
}

// TestReadChannel_TheGoldensToneMapsToKnown885 and
// TestReadChannel_AZeroToneMapsToUnknownNotKnown pin both directions of
// ruling T1(3).
func TestReadChannel_TheGoldensToneMapsToKnown885(t *testing.T) {
	t.Parallel()
	var img radioImage
	img.idToken = testToken
	populate(&img, wireAddr{0, 0})

	_, s := openFor(t, img)
	ch, err := s.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	for _, tt := range []struct {
		name  string
		field codeplug.ToneField
	}{
		{"tone_tx", ch.Data.ToneTx},
		{"tone_rx", ch.Data.ToneRx},
	} {
		if tt.field.State != codeplug.Known || tt.field.Value != 885 {
			t.Errorf("%s = %+v, want Known 885 — the 88.5 Hz both golden vectors carry", tt.name, tt.field)
		}
	}
}

// A civ-layer tone of 0 is a legal BCD number and the radio may well
// store one on a tone-OFF channel — but 0 Hz is NOT A TONE, the declared
// CTCSSToneRange starts at 1, and a Known 0 would fail ToneField.Valid on
// the very next validation. The READ MAPPING is what handles it, not the
// capability.
func TestReadChannel_AZeroToneMapsToUnknownNotKnown(t *testing.T) {
	t.Parallel()
	var img radioImage
	img.idToken = testToken
	zeroTone := goldenRecord(144_500_000, 5)
	zeroTone.toneTX, zeroTone.toneRX = 0, 0
	img.records = map[wireAddr][]byte{{0, 0}: zeroTone.build()}

	_, s := openFor(t, img)
	ch, err := s.ReadChannel(context.Background(), "G01-001")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	for _, tt := range []struct {
		name  string
		field codeplug.ToneField
	}{
		{"tone_tx", ch.Data.ToneTx},
		{"tone_rx", ch.Data.ToneRx},
	} {
		if tt.field.State != codeplug.Unknown {
			t.Errorf("%s = %+v, want Unknown — a read must never construct a Known value codeplug.Validate would refuse (T1(3))", tt.name, tt.field)
		}
		if tt.field.Value != 0 {
			t.Errorf("%s carries a value alongside a non-Known state: %+v", tt.name, tt.field)
		}
	}
}

// TestReadChannel_EveryChannelPassesCodeplugValidate is the property the
// whole T1 layering exists to guarantee, and it is asserted over a
// COMPLETE codeplug — every slot the session publishes — because
// codeplug.Validate checks completeness and bank membership as well as
// field values.
func TestReadChannel_EveryChannelPassesCodeplugValidate(t *testing.T) {
	t.Parallel()
	var img radioImage
	img.idToken = testToken
	// One ordinary channel, one with a zero tone (which maps to Unknown),
	// and ALL TWELVE CALL channels — three different shapes through one
	// validator.
	//
	// All twelve, because the CALL bank is NoBlank: the clear form's own
	// block forbids targeting group "01 00" (PDF p.19, folio 18), so
	// codeplug.Validate refuses a codeplug whose call channels are empty.
	// A radio with an unpopulated call channel is not something this
	// document describes, and the capability data says so rather than
	// tolerating it.
	populate(&img, wireAddr{0, 0})
	for n := 0; n < 12; n++ {
		populate(&img, wireAddr{callWireGroup, n})
	}
	zeroTone := goldenRecord(144_500_000, 5)
	zeroTone.toneTX, zeroTone.toneRX = 0, 0
	img.records[wireAddr{0, 1}] = zeroTone.build()

	_, s := openFor(t, img)
	caps := s.Capabilities()

	var channels []codeplug.Channel
	for _, b := range caps.Banks {
		for _, slot := range b.Slots {
			ch, err := s.ReadChannel(context.Background(), slot)
			if err != nil {
				t.Fatalf("ReadChannel(%q): %v", slot, err)
			}
			channels = append(channels, ch)
		}
	}

	cp := &codeplug.Codeplug{
		Radio:    codeplug.RadioInfo{Model: caps.Model, CATID: caps.CATID},
		Channels: channels,
	}
	issues := codeplug.Validate(cp, caps)
	if codeplug.HasErrors(issues) {
		for _, i := range issues {
			if i.Severity == codeplug.SeverityError {
				t.Errorf("codeplug.Validate: %s", i.Msg)
			}
		}
	}
}

// TestSlotAddress_BothNamespacesAndTheirEdges pins the decoding both
// directions, including the OFF-BY-ONE that ruling R4's 1-based sparse
// contract makes easy to get wrong.
func TestSlotAddress_BothNamespacesAndTheirEdges(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		slot string
		want civ.ChannelAddress
	}{
		{"G01-001", civ.ChannelAddress{Group: 0, Channel: 0}},
		{"G01-100", civ.ChannelAddress{Group: 0, Channel: 99}},
		{"G100-001", civ.ChannelAddress{Group: 99, Channel: 0}},
		{"G100-100", civ.ChannelAddress{Group: 99, Channel: 99}},
		{"C01", civ.ChannelAddress{Group: callWireGroup, Channel: 0}},
		{"C12", civ.ChannelAddress{Group: callWireGroup, Channel: 11}},
	} {
		got, err := slotAddress(tt.slot)
		if err != nil {
			t.Errorf("slotAddress(%q): %v", tt.slot, err)
			continue
		}
		if got != tt.want {
			t.Errorf("slotAddress(%q) = %v, want %v", tt.slot, got, tt.want)
		}
	}

	for _, slot := range []string{
		"", "001", "P1L", "EMG",
		"G00-001",    // group 0 is not a 1-based group
		"G01-000",    // nor is channel 0
		"G101-001",   // past the hundred groups
		"G01-101",    // past the hundred channels
		"C00", "C13", // outside the twelve call channels
		"C1", "C001", // second spellings of one slot, refused by ParseCallSlot's strictness
		"G5-12", "G005-0012", // likewise, refused by ParseSparseSlot's
	} {
		if got, err := slotAddress(slot); err == nil {
			t.Errorf("slotAddress(%q) = %v, want an error", slot, got)
		}
	}
}
