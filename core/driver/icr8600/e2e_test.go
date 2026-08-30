// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civicr8600 "github.com/gm5dna/open-rig-programmer/core/civ/icr8600"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
	"github.com/gm5dna/open-rig-programmer/internal/fakeicr8600"
)

// THE TWO WITNESSES MEET, HAVING NEVER READ EACH OTHER.
//
// Every other test in this package answers from respondingport_test.go — a
// scripted responder THIS side wrote, which proves the driver consistent with
// itself and nothing more. internal/fakeicr8600 is the other witness: an
// independently authored, stdlib-only receiver whose implementer read the
// quarantined B/W/G artefacts and never core/civ/icr8600, never this driver,
// and never the plan. Where the two agree below, two readings of one printed
// guide landed in the same place. Where they disagreed, one of them would be
// wrong and the disagreement would be the finding.
//
// EVERY BYTE CROSSES A SERIAL PORT. The driver is handed nothing but
// Radio.Port(), an in-memory duplex connection, and the fake is asked nothing
// but what a CI-V controller may ask over it. Neither package imports the
// other: fakeicr8600 imports nothing of this repository at all (its own
// TestNoCoreImports enforces that), and this file's only use of the fake is
// its constructor, its Options, its Port, its Frames and its Record.
//
// WHAT THE FAKE'S DEFAULT IMAGE IS, AND WHY THE EXPECTATIONS BELOW ARE
// SPELLED OUT BY HAND. The fake ships eight occupied channels in group 0, one
// per declared layout with both NXDN wire codes present (its doc.go register
// entry 10). This file names the eight modes, the six record-only lengths and
// every neutral field value it expects from them LITERALLY, never by calling
// either side's tables, so that a shared mistake in the two codecs cannot
// cancel itself out here.
//
// NOTHING BELOW CLAIMS HARDWARE. No IC-R8600 has answered this project. Green
// means the two independent readings agree; it does not mean a receiver does.

// e2eImage is the fake's default image restated BY HAND: the slot, the mode
// this driver must decode from the record's mode byte, and the record-only
// length that mode's layout selects. FM and DCR are BOTH 44 bytes, which is
// exactly why the mode byte and not the length decides the layout.
var e2eImage = []struct {
	slot   string
	mode   string
	length int
}{
	{"G00-000", "AM", 37},
	{"G00-001", "FM", 44},
	{"G00-002", "P25", 41},
	{"G00-003", "D-STAR", 39},
	{"G00-004", "dPMR", 45},
	{"G00-005", "NXDN-VN", 43},
	{"G00-006", "NXDN-N", 43},
	{"G00-007", "DCR", 44},
}

// openFake opens one session against a real fake receiver over its port.
//
// fastTiming is the same short read timeout the scripted tests use: the
// bounded occupied-slot search makes 199 reads before a session exists, and
// production deliberately takes the transport defaults until a Stage R
// capture measures this radio.
func openFake(t *testing.T, profile Profile, consented bool, opts ...fakeicr8600.Option) (*fakeicr8600.Radio, *Session) {
	return openFakeWithDriverOptions(t, profile, consented, nil, opts...)
}

func openFakeWithDriverOptions(t *testing.T, profile Profile, consented bool, driverOpts []Option, opts ...fakeicr8600.Option) (*fakeicr8600.Radio, *Session) {
	t.Helper()
	radio := fakeicr8600.New(opts...)
	t.Cleanup(func() { _ = radio.Close() })
	sess, err := newFakeDriver(profile, consented, driverOpts...).Open(context.Background(), radio.Port(), driver.Identity{Port: "fake"})
	if err != nil {
		t.Fatalf("Open against the independent fake: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return radio, sess.(*Session)
}

func newFakeDriver(profile Profile, consented bool, extra ...Option) driver.Driver {
	opts := append([]Option{fastTiming()}, extra...)
	if consented {
		opts = append(opts, WithConsentedUnverifiedWrites())
	}
	return New(profile, opts...)
}

// TestEndToEnd_InventoryCompletenessAndFullWalk pins the IC-905-shaped
// completeness contract against the independent fake. The hidden record is
// copied from the fake's own default image, so this test adds no evidence and
// changes none of the quarantined evidence tables.
func TestEndToEnd_InventoryCompletenessAndFullWalk(t *testing.T) {
	seed := fakeicr8600.New()
	hidden, ok := seed.Record(0, 0)
	if !ok {
		t.Fatal("the fake's default G00-000 record is missing")
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed fake: %v", err)
	}

	t.Run("bounded default is partial", func(t *testing.T) {
		_, s := openFake(t, RealHardware, false, fakeicr8600.WithRecord(5, 37, hidden))
		mem, ok := s.Capabilities().Bank(spec.BankMemory)
		if !ok {
			t.Fatal("the session has no MEM bank")
		}
		if slices.Contains(mem.Slots, "G05-037") {
			t.Fatal("the bounded walk found G05-037 although G05-000 is empty")
		}
		if d := s.OpenDiagnostics(); d.InventoryComplete || d.SlotsTried != 199 {
			t.Errorf("OpenDiagnostics = %+v, want InventoryComplete false after 199 reads", d)
		}
	})

	t.Run("full walk is complete", func(t *testing.T) {
		_, s := openFakeWithDriverOptions(t, RealHardware, false, []Option{WithFullInventoryWalk()}, fakeicr8600.WithRecord(5, 37, hidden))
		mem, ok := s.Capabilities().Bank(spec.BankMemory)
		if !ok {
			t.Fatal("the session has no MEM bank")
		}
		if !slices.Contains(mem.Slots, "G05-037") {
			t.Fatalf("full-walk slots = %v, want hidden G05-037", mem.Slots)
		}
		if d := s.OpenDiagnostics(); !d.InventoryComplete || d.SlotsTried != 10_000 {
			t.Errorf("OpenDiagnostics = %+v, want InventoryComplete true after 10,000 reads", d)
		}
	})
}

// TestEndToEnd_WriteToAnOccupiedSlotTheBoundedWalkMissedIsRefused pins the
// IC-905-shaped occupied-surprise guard. G05-000 is empty, so discovery never
// reads G05-037 into the inventory; the preservation read then finds a record,
// and the driver must refuse rather than overwrite a channel the codeplug never
// saw.
func TestEndToEnd_WriteToAnOccupiedSlotTheBoundedWalkMissedIsRefused(t *testing.T) {
	seed := fakeicr8600.New()
	hidden, ok := seed.Record(0, 0)
	if !ok {
		t.Fatal("the fake's default G00-000 record is missing")
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed fake: %v", err)
	}

	radio, s := openFake(t, RealHardware, true, fakeicr8600.WithRecord(5, 37, hidden))
	mem, ok := s.Capabilities().Bank(spec.BankMemory)
	if !ok {
		t.Fatal("the session has no MEM bank")
	}
	if slices.Contains(mem.Slots, "G05-037") {
		t.Fatal("the bounded walk found G05-037 although G05-000 is empty")
	}

	beforeSets := len(setFrames(radio.Frames()))
	res, err := s.WriteChannel(context.Background(), writableChannel("G05-037", "AM"))
	refused := requireWriteRefused(t, res, err)
	if refused.Slot != "G05-037" {
		t.Errorf("WriteRefusedError.Slot = %q, want G05-037", refused.Slot)
	}
	if !strings.Contains(refused.Reason, "discovery walk never saw it") {
		t.Errorf("WriteRefusedError.Reason = %q, want the undiscovered-slot cause", refused.Reason)
	}
	if got := len(setFrames(radio.Frames())); got != beforeSets {
		t.Errorf("refused write sent %d memory-set frames, want zero", got-beforeSets)
	}
	if got, ok := radio.Record(5, 37); !ok || !bytes.Equal(got, hidden) {
		t.Errorf("the refused write changed G05-037: present=%v record=% X", ok, got)
	}
}

// readFrames is Frames() reduced to the memory READ grammar: 1A 00 plus the
// four printed address bytes and nothing else.
func readFrames(frames [][]byte) [][]byte {
	var out [][]byte
	for _, f := range frames {
		if len(f) == 11 && f[4] == 0x1A && f[5] == 0x00 {
			out = append(out, f)
		}
	}
	return out
}

// assertOnlyAdmittedGrammars fails unless every frame the fake RECEIVED is one
// of the three admitted ones: the 19 00 identity read, the 1A 00 memory read,
// and the 1A 00 memory set. In particular no 1A 05 setting write, no
// transceive command, and no clear form (address followed by FF).
func assertOnlyAdmittedGrammars(t *testing.T, frames [][]byte) {
	t.Helper()
	for _, f := range frames {
		switch {
		case len(f) == 7 && f[4] == 0x19 && f[5] == 0x00:
		case len(f) == 11 && f[4] == 0x1A && f[5] == 0x00:
		case len(f) > 11 && f[4] == 0x1A && f[5] == 0x00:
			if len(f) == 12 && f[10] == 0xFF {
				t.Errorf("the printed clear form reached the wire: % X", f)
			}
		default:
			t.Errorf("a frame outside the three admitted grammars reached the wire: % X", f)
		}
	}
}

func TestEndToEnd_ProbeReadAllAndConsentedWriteBack(t *testing.T) {
	// The identity token's VALUE is printed nowhere in the guide (the
	// fake's register entry 1, the driver's icr8600-id-token), so this
	// pins a token of its own and requires the driver to RECORD whatever
	// it was told rather than match a value.
	radio, s := openFake(t, RealHardware, true, fakeicr8600.WithIDToken([]byte{0xA5, 0x5A}))

	if got, want := s.Identity().CATID, "96:A55A"; got != want {
		t.Errorf("CATID = %q, want %q: the address is matched, the token only recorded", got, want)
	}
	if got := newFakeDriver(RealHardware, false).Model(); got != "IC-R8600" {
		t.Errorf("Model = %q, want IC-R8600", got)
	}
	d := s.OpenDiagnostics()
	if !d.Fingerprinted || d.RecordLength != 37 || d.FirstOccupied != "G00-000" {
		t.Errorf("OpenDiagnostics = %+v, want the 37-byte AM record at G00-000 fingerprinted", d)
	}
	if !strings.Contains(d.AddressDiagnostic, "address 96 confirmed") || !strings.Contains(d.AddressDiagnostic, "record fingerprint 37") {
		t.Errorf("AddressDiagnostic = %q", d.AddressDiagnostic)
	}
	// Group zero in full plus one sample per later group: 100 + 99.
	if d.SlotsTried != 199 {
		t.Errorf("SlotsTried = %d, want the bounded 199-read sparse walk", d.SlotsTried)
	}
	assertOnlyAdmittedGrammars(t, radio.Frames())
	if got := len(setFrames(radio.Frames())); got != 0 {
		t.Errorf("opening the session put %d set frames on the wire; opening must mutate nothing", got)
	}

	mem, ok := s.Capabilities().Bank(spec.BankMemory)
	if !ok {
		t.Fatal("the session has no MEM bank")
	}
	var wantSlots []string
	for _, entry := range e2eImage {
		wantSlots = append(wantSlots, entry.slot)
	}
	if strings.Join(mem.Slots, ",") != strings.Join(wantSlots, ",") {
		t.Fatalf("discovered slots = %v, want %v", mem.Slots, wantSlots)
	}

	// READ ALL. Every field below is the fake's own default image read
	// back through this side's decoder, restated by hand.
	for _, entry := range e2eImage {
		ch, err := s.ReadChannel(context.Background(), entry.slot)
		if err != nil {
			t.Fatalf("ReadChannel(%s): %v", entry.slot, err)
		}
		if ch.Data == nil {
			t.Fatalf("ReadChannel(%s) came back empty", entry.slot)
		}
		got := *ch.Data
		if got.Mode != entry.mode {
			t.Errorf("%s mode = %q, want %q", entry.slot, got.Mode, entry.mode)
		}
		if got.Tag != entry.mode {
			t.Errorf("%s tag = %q, want the fake's own %q", entry.slot, got.Tag, entry.mode)
		}
		if got.FreqHz != 145_500_000 {
			t.Errorf("%s frequency = %d Hz, want 145500000", entry.slot, got.FreqHz)
		}
		if got.Duplex.Value != "OFF" || got.OffsetHz.Value != 0 {
			t.Errorf("%s duplex/offset = %v/%v", entry.slot, got.Duplex, got.OffsetHz)
		}
		// THE SEVEN D8 RECEIVER FIELDS (Erratum 6). They exist in the
		// record vocabulary only because that erratum added them, and
		// every one of them must survive the fake's wire.
		if !got.TuningStepEnabled.Value || got.TuningStepEnabled.State != codeplug.Known {
			t.Errorf("%s tuning_step_enabled = %+v, want Known ON", entry.slot, got.TuningStepEnabled)
		}
		if got.TuningStep.Value != "5 kHz" {
			t.Errorf("%s tuning_step = %q, want 5 kHz", entry.slot, got.TuningStep.Value)
		}
		if got.ProgramTuningStepHz.Value != 9_000 {
			t.Errorf("%s program_tuning_step = %d Hz, want 9000", entry.slot, got.ProgramTuningStepHz.Value)
		}
		if got.AttenuatorDB.Value != 0 || got.AttenuatorDB.State != codeplug.Known {
			t.Errorf("%s attenuator = %+v, want Known 0 dB", entry.slot, got.AttenuatorDB)
		}
		if got.Preamp.Value != "ON" {
			t.Errorf("%s preamp = %q, want ON", entry.slot, got.Preamp.Value)
		}
		if got.Antenna.Value != "ANT1" {
			t.Errorf("%s antenna = %q, want ANT1", entry.slot, got.Antenna.Value)
		}
		if got.IPPlus.Value || got.IPPlus.State != codeplug.Known {
			t.Errorf("%s ip_plus = %+v, want Known OFF", entry.slot, got.IPPlus)
		}
		// The receiver has no transmitter: neither field may ever be
		// anything but Unavailable, on any layout.
		if got.TxFreqHz.State != codeplug.Unavailable || got.ToneTx.State != codeplug.Unavailable {
			t.Errorf("%s receive-only fields = tx %v tone_tx %v", entry.slot, got.TxFreqHz.State, got.ToneTx.State)
		}
	}

	// ONE CONSENTED FULL-LAYOUT WRITE, then a read back over the same
	// port. The record the fake stores is the driver's own bytes; the
	// record the driver reads back is the fake's copy of them.
	before := len(setFrames(radio.Frames()))
	want := writableChannel("G00-001", "FM")
	res, err := s.WriteChannel(context.Background(), want)
	if err != nil {
		t.Fatalf("consented WriteChannel(G00-001): %v", err)
	}
	if len(res.Steps) != 1 || res.Steps[0].Command != "1A 00" || !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Fatalf("WriteResult = %+v, want one acknowledged 1A 00 step", res.Steps)
	}
	if got := len(setFrames(radio.Frames())) - before; got != 1 {
		t.Errorf("the write put %d set frames on the wire, want exactly one", got)
	}
	if stored, ok := radio.Record(0, 1); !ok || len(stored) != 44 {
		t.Errorf("the fake holds %d bytes at G00-001 (present %v), want the 44-byte FM layout", len(stored), ok)
	}

	back, err := s.ReadChannel(context.Background(), "G00-001")
	if err != nil {
		t.Fatalf("ReadChannel(G00-001) after the write: %v", err)
	}
	if back.Data == nil {
		t.Fatal("the written channel read back empty")
	}
	w, g := *want.Data, *back.Data
	if g.FreqHz != w.FreqHz || g.Mode != w.Mode || g.Tag != w.Tag {
		t.Errorf("round trip head = %d/%q/%q, want %d/%q/%q", g.FreqHz, g.Mode, g.Tag, w.FreqHz, w.Mode, w.Tag)
	}
	if g.Duplex.Value != w.Duplex.Value || g.OffsetHz.Value != w.OffsetHz.Value || g.Filter.Value != w.Filter.Value {
		t.Errorf("round trip duplex/offset/filter = %q/%d/%q", g.Duplex.Value, g.OffsetHz.Value, g.Filter.Value)
	}
	if g.TuningStepEnabled.Value != w.TuningStepEnabled.Value || g.TuningStep.Value != w.TuningStep.Value ||
		g.ProgramTuningStepHz.Value != w.ProgramTuningStepHz.Value || g.AttenuatorDB.Value != w.AttenuatorDB.Value ||
		g.Preamp.Value != w.Preamp.Value || g.Antenna.Value != w.Antenna.Value || g.IPPlus.Value != w.IPPlus.Value {
		t.Errorf("round trip D8 seven = ts_en %v ts %q pts %d att %d pre %q ant %q ip %v",
			g.TuningStepEnabled.Value, g.TuningStep.Value, g.ProgramTuningStepHz.Value,
			g.AttenuatorDB.Value, g.Preamp.Value, g.Antenna.Value, g.IPPlus.Value)
	}
	if g.ToneMode.Value != "TSQL" || g.ToneRx.Value != 885 || g.DTCSCode.Value != 23 || g.DTCSPolarity.Value != "Reverse" {
		t.Errorf("round trip FM receive squelch = %q/%v/%v/%q", g.ToneMode.Value, g.ToneRx.Value, g.DTCSCode.Value, g.DTCSPolarity.Value)
	}
	if g.TxFreqHz.State != codeplug.Unavailable || g.ToneTx.State != codeplug.Unavailable {
		t.Errorf("round trip re-introduced a transmit field: tx %v tone_tx %v", g.TxFreqHz.State, g.ToneTx.State)
	}
	assertOnlyAdmittedGrammars(t, radio.Frames())
}

func TestEndToEnd_UnconsentedRealHardwareWriteIsRefusedOverTheWire(t *testing.T) {
	radio, s := openFake(t, RealHardware, false)
	before := len(radio.Frames())
	res, err := s.WriteChannel(context.Background(), writableChannel("G00-001", "FM"))
	requireWriteRefused(t, res, err)
	if got := len(radio.Frames()) - before; got != 0 {
		t.Errorf("an unconsented refusal put %d frames on the wire, want none", got)
	}
}

func TestEndToEnd_EveryModeTailAndTheSharedLength44(t *testing.T) {
	radio, s := openFake(t, RealHardware, false)

	for _, entry := range e2eImage {
		addr, err := slotAddress(entry.slot)
		if err != nil {
			t.Fatalf("slotAddress(%s): %v", entry.slot, err)
		}
		stored, ok := radio.Record(addr.Group, addr.Channel)
		if !ok {
			t.Fatalf("the fake holds nothing at %s", entry.slot)
		}
		if len(stored) != entry.length {
			t.Errorf("%s record-only length = %d, want %d", entry.slot, len(stored), entry.length)
		}
		ch, err := s.ReadChannel(context.Background(), entry.slot)
		if err != nil || ch.Data == nil || ch.Data.Mode != entry.mode {
			t.Fatalf("ReadChannel(%s) = %+v, %v; want mode %q", entry.slot, ch, err, entry.mode)
		}
	}

	// FM AND DCR ARE BOTH 44 BYTES. Length names nothing on its own here;
	// the mode byte at record offset 6 is what picks the layout, and the
	// two packages agree on that byte without having read each other.
	fm, _ := radio.Record(0, 1)
	dcr, _ := radio.Record(0, 7)
	if len(fm) != 44 || len(dcr) != 44 {
		t.Fatalf("FM/DCR lengths = %d/%d, want 44 and 44", len(fm), len(dcr))
	}
	if fm[6] == dcr[6] {
		t.Fatalf("FM and DCR carry the same mode byte %#02x; nothing could tell them apart", fm[6])
	}
	if bytes.Equal(fm[37:], dcr[37:]) {
		t.Error("the two 44-byte tails are byte-identical; the length collision would be undetectable")
	}
}

func TestEndToEnd_WrongSiblingRecordLengthRefusesTheRadio(t *testing.T) {
	t.Run("a data-area length is not a record-only length", func(t *testing.T) {
		// 48 bytes is THIS radio's own FM/DCR data-area accounting —
		// the 44-byte record plus its four address bytes — and the
		// plan pins it as a third accounting that must never be
		// confused with a record-only length. A receiver answering 48
		// record bytes is not this dialect, and the probe must say so
		// rather than decode 48 bytes as something.
		radio := fakeicr8600.New(fakeicr8600.WithRecord(0, 0, make([]byte, 48)))
		defer radio.Close()
		_, err := newFakeDriver(RealHardware, false).Open(context.Background(), radio.Port(), driver.Identity{})
		if !errors.Is(err, driver.ErrWrongRadio) {
			t.Fatalf("Open error = %v, want ErrWrongRadio", err)
		}
		var wrong *driver.WrongRadioError
		if !errors.As(err, &wrong) {
			t.Fatalf("Open error type = %T, want *driver.WrongRadioError", err)
		}
		if !strings.HasPrefix(wrong.Want, "record ") || wrong.Got != "record 48" {
			t.Errorf("WrongRadioError = want %q got %q", wrong.Want, wrong.Got)
		}
	})

	t.Run("a longer sibling's record never becomes a frame", func(t *testing.T) {
		// The IC-9700's 111-byte record would need a 122-byte frame,
		// and this profile's MaxFrame is 64 — sized for the 55-byte G
		// witness and nothing more. So the wrong sibling is refused
		// EARLIER than the length fingerprint, by the reader that
		// declines to grow without bound on a contaminated line. The
		// refusal is still a refusal; what matters is that no part of
		// a foreign record is ever decoded, and that the failure is
		// not attributed to a different model.
		radio := fakeicr8600.New(fakeicr8600.WithRecord(0, 0, make([]byte, 111)))
		defer radio.Close()
		_, err := newFakeDriver(RealHardware, false).Open(context.Background(), radio.Port(), driver.Identity{})
		if err == nil {
			t.Fatal("Open succeeded against a receiver answering a 111-byte record")
		}
		if !strings.Contains(err.Error(), "frame exceeded maximum length") {
			t.Errorf("Open error = %q, want the frame-cap refusal", err)
		}
	})
}

func TestEndToEnd_ModeLengthMismatchIsRefusedOnRead(t *testing.T) {
	// The mismatch is seeded OUTSIDE group zero and off channel zero, so
	// the bounded sparse walk never reaches it and the session opens: the
	// refusal being proved is the CONTINUOUS one, on an ordinary read.
	fm := testRecord(t, testWireAddress{1, 5}, "FM")
	radio, s := openFake(t, RealHardware, false, fakeicr8600.WithRecord(1, 5, fm[:37]))

	_, err := s.ReadChannel(context.Background(), "G01-005")
	var lengthErr *civ.RecordLengthError
	if !errors.As(err, &lengthErr) {
		t.Fatalf("ReadChannel error = %v, want *civ.RecordLengthError", err)
	}
	if lengthErr.Mode != "FM" || lengthErr.Got != 37 {
		t.Errorf("RecordLengthError = %+v, want FM at 37 bytes", lengthErr)
	}
	if got := len(setFrames(radio.Frames())); got != 0 {
		t.Errorf("a refused read produced %d set frames", got)
	}
}

func TestEndToEnd_DigitalSquelchTailRefusesWithTheExactE6Reason(t *testing.T) {
	// The fake's own P25 tail and this profile's assumed template agree —
	// two independent readings of one diagram — so the refusal has to be
	// provoked by seeding a tail that differs, which is precisely the
	// state E6 exists for: bytes no neutral field carries, which a write
	// would silently replace.
	p25 := testRecord(t, testWireAddress{0, 2}, "P25")
	p25[39] ^= 0x07
	radio, s := openFake(t, RealHardware, true, fakeicr8600.WithRecord(0, 2, p25))

	before := len(setFrames(radio.Frames()))
	res, err := s.WriteChannel(context.Background(), writableChannel("G00-002", "P25"))
	refused := requireWriteRefused(t, res, err)
	if refused.Reason != civicr8600.DigitalTailRefusalReason {
		t.Errorf("reason = %q, want the exact E6 sentence %q", refused.Reason, civicr8600.DigitalTailRefusalReason)
	}
	if got := len(setFrames(radio.Frames())) - before; got != 0 {
		t.Errorf("the E6 refusal put %d set frames on the wire", got)
	}
	if stored, _ := radio.Record(0, 2); !bytes.Equal(stored, p25) {
		t.Error("the refused write changed what the fake holds")
	}
}

// TestEndToEnd_TheFakesDeclaredEvidenceBaseIsBWAndG guards the premise this
// whole file rests on. Every "the two witnesses agree" claim below is worth
// something only because of what the fake's implementer read: the
// quarantined manual-derived artefacts, and no source file of
// core/civ/icr8600 or core/driver/icr8600. That premise lives in the fake's
// own prose, so this side pins it rather than trusting it.
//
// The plan's Task 9 said "only the manual-derived B/W artefacts". The fake
// in fact read three — B, W AND the golden vectors' provenance — which is a
// WIDENING, and the packages must say so in as many words rather than leave
// a silent difference between plan and package.
func TestEndToEnd_TheFakesDeclaredEvidenceBaseIsBWAndG(t *testing.T) {
	for _, name := range []string{"doc.go", "image.go"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "internal", "fakeicr8600", name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			source := string(data)
			for _, want := range []string{
				"IC-R8600-transcription-b",
				"IC-R8600-geometry-witness",
				"IC-R8600-golden-provenance.md",
				"Task 9",
			} {
				if !strings.Contains(source, want) {
					t.Errorf("%s does not name %q; the evidence base must be stated as B, W AND G, and the widening of the plan's Task 9 recorded", name, want)
				}
			}
		})
	}

	// The widening is about EVIDENCE, never about code. Nothing in the
	// fake may depend on Stage 1/2, and its doc.go must keep saying so.
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "internal", "fakeicr8600", "doc.go"))
	if err != nil {
		t.Fatalf("read the fake's doc.go: %v", err)
	}
	if !strings.Contains(string(data), "NO SOURCE FILE OF core/civ/icr8600 OR") {
		t.Error("the fake's doc.go no longer states that it read no Stage 1/2 source file")
	}
}

func TestEndToEnd_AModeChangeIntoADigitalClassIsRefusedRatherThanInventTheTail(t *testing.T) {
	// The fake's G00-000 is AM: a 37-byte record with NO tail at all. A
	// DCR write would build a 44-byte record whose seven tail bytes come
	// entirely from the profile's ASSUMED icr8600-tail-templates entry —
	// D.SQL UC, UC code 123, encryption off — values G chose and no
	// evidence supplies, which the stored record cannot preserve because
	// it never had them.
	//
	// The FM path already refuses the mirror case ("the stored non-FM
	// record has no FM tail to preserve") and the empty-slot path refuses
	// "rather than inventing OFF". A digital target must refuse for the
	// same reason instead of putting invented bytes on a real receiver.
	radio, s := openFake(t, RealHardware, true)
	stored, ok := radio.Record(0, 0)
	if !ok || len(stored) != 37 {
		t.Fatalf("the fake holds a %d-byte record at G00-000 (present %v), want the 37-byte AM record", len(stored), ok)
	}

	before := len(setFrames(radio.Frames()))
	res, err := s.WriteChannel(context.Background(), writableChannel("G00-000", "DCR"))
	refused := requireWriteRefused(t, res, err)
	// The reason must NAME the bytes that would be invented, so an
	// operator reads which state the radio must be put into by hand.
	for _, want := range []string{"NONE", "DCR", "bytes 37-43", "icr8600-tail-templates", "01 01 23 00 00 00 00"} {
		if !strings.Contains(refused.Reason, want) {
			t.Errorf("refusal reason = %q, want it to name %q", refused.Reason, want)
		}
	}
	if got := len(setFrames(radio.Frames())) - before; got != 0 {
		t.Errorf("the refusal put %d set frames on the wire", got)
	}
	if now, _ := radio.Record(0, 0); !bytes.Equal(now, stored) {
		t.Error("the refused write changed what the fake holds")
	}
}

func TestEndToEnd_ReceiveOnlyTransmitFieldsAreRefusedBeforeTheWire(t *testing.T) {
	radio, s := openFake(t, RealHardware, true)

	for _, tc := range []struct {
		name  string
		mutot func(*codeplug.ChannelData)
		field spec.Field
	}{
		{"tx frequency", func(d *codeplug.ChannelData) {
			d.TxFreqHz = codeplug.FreqField{State: codeplug.Known, Value: 145_600_000}
		}, spec.FieldTxFrequency},
		{"tx tone", func(d *codeplug.ChannelData) {
			d.ToneTx = codeplug.ToneField{State: codeplug.Known, Value: 885}
		}, spec.FieldToneTx},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ch := writableChannel("G00-001", "FM")
			tc.mutot(ch.Data)
			before := len(radio.Frames())
			res, err := s.WriteChannel(context.Background(), ch)
			requireWriteRefused(t, res, err, tc.field)
			if got := len(radio.Frames()) - before; got != 0 {
				t.Errorf("a receive-only refusal put %d frames on the wire, want none", got)
			}
		})
	}
}

func TestEndToEnd_EraseIsRefusedAndNeverReachesTheWire(t *testing.T) {
	// The clear form IS printed in the guide, and the fake would refuse
	// it (its register entry 7) — but this tier admits no erase builder
	// at all, so the frame must never be built in the first place.
	radio, s := openFake(t, RealHardware, true)

	before := len(radio.Frames())
	res, err := s.WriteChannel(context.Background(), codeplug.Channel{Slot: "G00-000"})
	refused := requireWriteRefused(t, res, err, spec.FieldErase)
	if !strings.Contains(refused.Reason, "no erase path") {
		t.Errorf("reason = %q, want the tier's no-erase sentence", refused.Reason)
	}
	if got := len(radio.Frames()) - before; got != 0 {
		t.Errorf("the erase refusal put %d frames on the wire, want none", got)
	}
	for _, f := range radio.Frames() {
		if len(f) == 12 && f[4] == 0x1A && f[5] == 0x00 && f[10] == 0xFF {
			t.Errorf("the printed clear form reached the wire: % X", f)
		}
	}
	if _, ok := radio.Record(0, 0); !ok {
		t.Error("the fake no longer holds G00-000 after a refused erase")
	}
}

func TestEndToEnd_ExactEchoIsSuppressedByByteIdentity(t *testing.T) {
	// The fake's bus echo repeats every complete frame VERBATIM. Byte
	// identity against what this side recorded sending is the only thing
	// that may suppress it, and nothing in a bus echo is unexpected.
	radio, s := openFake(t, RealHardware, false, fakeicr8600.WithEcho(true))

	// An echo is a property of the wire, not a reason to send anything
	// again: the bounded walk is still exactly one identity read and 199
	// memory reads.
	if got := len(readFrames(radio.Frames())); got != 199 {
		t.Errorf("the echoing session sent %d memory reads, want the same bounded 199", got)
	}
	stats := s.WireStats()
	if stats.Echoes == 0 {
		t.Errorf("wire stats = %+v, want byte-identical echoes suppressed", stats)
	}
	if stats.Unexpected != 0 {
		t.Errorf("wire stats = %+v, want no unexpected frames from an exact echo", stats)
	}
	ch, err := s.ReadChannel(context.Background(), "G00-001")
	if err != nil || ch.Data == nil || ch.Data.Mode != "FM" {
		t.Fatalf("ReadChannel through the echo = %+v, %v", ch, err)
	}
	if s.WireStats().Echoes <= stats.Echoes {
		t.Error("the read's own echo was not suppressed")
	}
}

func TestEndToEnd_TransceiveBroadcastsAreSurvived(t *testing.T) {
	// A receiver that never goes quiet. The broadcasts are addressed to
	// the broadcast byte, never to this controller, so the address filter
	// must drop every one of them without disturbing a transaction.
	radio, s := openFake(t, RealHardware, true, fakeicr8600.WithTransceiveBroadcasts(200*time.Microsecond))

	for _, entry := range e2eImage {
		ch, err := s.ReadChannel(context.Background(), entry.slot)
		if err != nil || ch.Data == nil || ch.Data.Mode != entry.mode {
			t.Fatalf("ReadChannel(%s) under a broadcast flood = %+v, %v", entry.slot, ch, err)
		}
	}
	res, err := s.WriteChannel(context.Background(), writableChannel("G00-001", "FM"))
	if err != nil {
		t.Fatalf("WriteChannel under a broadcast flood: %v", err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Confirmed {
		t.Errorf("WriteResult under a broadcast flood = %+v", res.Steps)
	}
	if got := len(setFrames(radio.Frames())); got != 1 {
		t.Errorf("set frames = %d, want exactly one", got)
	}
}

func TestEndToEnd_BothPrintedEmptyAnswersReadAsEmpty(t *testing.T) {
	// The two empty answers are TWO SEPARATE ASSUMPTIONS — the fake
	// grades them apart (its entries 5 and 6) and so does the profile
	// (icr8600-empty-reply-fa, icr8600-empty-reply-ff). A driver must
	// cope with either, and the all-FF one must be decided on raw bytes
	// BEFORE any layout is selected: its mode byte, FF, names none.
	for _, tc := range []struct {
		name string
		opts []fakeicr8600.Option
	}{
		{"NG", nil},
		{"all-FF record", []fakeicr8600.Option{fakeicr8600.WithEmptyReplyAllFF()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			radio, s := openFake(t, RealHardware, false, tc.opts...)
			ch, err := s.ReadChannel(context.Background(), "G00-050")
			if err != nil {
				t.Fatalf("ReadChannel of an unoccupied channel: %v", err)
			}
			if !ch.Empty() || ch.Slot != "G00-050" {
				t.Errorf("ReadChannel = %+v, want an empty channel at G00-050", ch)
			}
			mem, _ := s.Capabilities().Bank(spec.BankMemory)
			if len(mem.Slots) != len(e2eImage) {
				t.Errorf("discovered %d occupied slots, want %d: an empty answer must not count as occupied", len(mem.Slots), len(e2eImage))
			}
			if got := len(setFrames(radio.Frames())); got != 0 {
				t.Errorf("reading empty channels produced %d set frames", got)
			}
		})
	}
}

// ackSwallowingPort is the whole of this file's wire plumbing: a filter that
// sits between the driver and the fake, forwards everything the driver sends,
// and drops the fake's positive acknowledgement — the six bytes FE FE E0 96
// FB FD — on its way back.
//
// It exists because the fake CANNOT be made to go silent on a frame addressed
// to it: an accepted set is acknowledged, and the only silence it offers is
// for a frame addressed to another radio, which the driver never sends and
// which would fail the identity probe long before a write. Swallowing the ack
// on the wire is therefore the only honest way to reach the branch this tier
// most needs proved: a set that WAS transmitted, WAS applied by the receiver,
// and was never acknowledged, whose outcome is unattributable and which must
// never be resent. The fake's own behaviour is untouched — Frames() and
// Record() below show it received and stored the set exactly once.
type ackSwallowingPort struct {
	radio io.ReadWriteCloser
	pr    *io.PipeReader
	pw    *io.PipeWriter
	once  sync.Once
}

func newAckSwallowingPort(t *testing.T, radio io.ReadWriteCloser) *ackSwallowingPort {
	t.Helper()
	pr, pw := io.Pipe()
	p := &ackSwallowingPort{radio: radio, pr: pr, pw: pw}
	go p.pump()
	t.Cleanup(func() { _ = p.Close() })
	return p
}

func (p *ackSwallowingPort) pump() {
	buf := make([]byte, 256)
	var pending []byte
	for {
		n, err := p.radio.Read(buf)
		if n > 0 {
			pending = append(pending, buf[:n]...)
			for {
				frame, rest, ok := cutFrame(pending)
				if !ok {
					break
				}
				pending = rest
				if len(frame) == 6 && frame[4] == 0xFB {
					continue
				}
				if _, werr := p.pw.Write(frame); werr != nil {
					return
				}
			}
		}
		if err != nil {
			_ = p.pw.CloseWithError(err)
			return
		}
	}
}

func (p *ackSwallowingPort) Read(b []byte) (int, error)  { return p.pr.Read(b) }
func (p *ackSwallowingPort) Write(b []byte) (int, error) { return p.radio.Write(b) }

func (p *ackSwallowingPort) Close() error {
	p.once.Do(func() {
		_ = p.radio.Close()
		_ = p.pw.Close()
		_ = p.pr.Close()
	})
	return nil
}

func TestEndToEnd_UnacknowledgedSetIsQuarantinedAndNeverResent(t *testing.T) {
	radio := fakeicr8600.New()
	defer radio.Close()
	port := newAckSwallowingPort(t, radio.Port())
	sess, err := newFakeDriver(RealHardware, true).Open(context.Background(), port, driver.Identity{Port: "fake"})
	if err != nil {
		t.Fatalf("Open through the ack filter: %v", err)
	}
	defer sess.Close()
	s := sess.(*Session)

	res, err := s.WriteChannel(context.Background(), writableChannel("G00-001", "FM"))
	if !errors.Is(err, transport.ErrTimeout) {
		t.Fatalf("WriteChannel error = %v, want ErrTimeout", err)
	}
	if len(res.Steps) != 1 || res.Steps[0].Sent || res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want one step whose outcome is not known-clean", res.Steps)
	}
	if !strings.Contains(err.Error(), "UNATTRIBUTABLE") || !strings.Contains(err.Error(), "will not be resent") {
		t.Errorf("timeout error = %q, want the operator-facing no-resend warning", err)
	}
	sent := setFrames(radio.Frames())
	if len(sent) != 1 {
		t.Fatalf("the fake received %d set frames, want EXACTLY ONE and no retransmission", len(sent))
	}
	// The receiver applied it. That is exactly why an unacknowledged set
	// is unattributable rather than failed, and why it must not be resent.
	if stored, ok := radio.Record(0, 1); !ok || len(stored) != 44 {
		t.Errorf("the fake holds %d bytes at G00-001 (present %v), want the applied 44-byte record", len(stored), ok)
	}

	start := time.Now()
	if _, err := s.ReadChannel(context.Background(), "G00-001"); err != nil {
		t.Fatalf("read after the timed-out write: %v", err)
	}
	if elapsed := time.Since(start); elapsed < civ.DrainIdleGap {
		t.Errorf("the read after a write timeout took %v, want at least the quarantine idle gap %v", elapsed, civ.DrainIdleGap)
	}
	if got := len(setFrames(radio.Frames())); got != 1 {
		t.Errorf("set frames after the quarantine = %d, want no retry", got)
	}
	assertOnlyAdmittedGrammars(t, radio.Frames())
}

func TestEndToEnd_ModelAndAddressDiagnostics(t *testing.T) {
	t.Run("unfingerprinted", func(t *testing.T) {
		// An empty receiver still identifies itself; what it cannot do
		// is confirm the record shape, and the diagnostic says so
		// rather than claiming a fingerprint it does not have.
		opts := []fakeicr8600.Option{}
		for i := range e2eImage {
			opts = append(opts, fakeicr8600.WithEmpty(0, i))
		}
		radio, s := openFake(t, RealHardware, false, opts...)
		d := s.OpenDiagnostics()
		if d.Fingerprinted || d.RecordLength != 0 || d.FirstOccupied != "" {
			t.Errorf("OpenDiagnostics = %+v, want no fingerprint", d)
		}
		if !strings.Contains(d.AddressDiagnostic, "address 96 confirmed") || !strings.Contains(d.AddressDiagnostic, "UNFINGERPRINTED") {
			t.Errorf("AddressDiagnostic = %q", d.AddressDiagnostic)
		}
		mem, _ := s.Capabilities().Bank(spec.BankMemory)
		if len(mem.Slots) != 0 {
			t.Errorf("discovered slots = %v, want none", mem.Slots)
		}
		if got := len(readFrames(radio.Frames())); got != 199 {
			t.Errorf("bounded walk sent %d reads, want 199", got)
		}
	})

	t.Run("model name and capability identity", func(t *testing.T) {
		_, s := openFake(t, Simulated, false)
		caps := s.Capabilities()
		if caps.Model != "IC-R8600" || caps.Transmit != spec.ReceiveOnly {
			t.Errorf("session capabilities = %q/%v, want IC-R8600 receive-only", caps.Model, caps.Transmit)
		}
		if !strings.HasPrefix(caps.CATID, "96:") {
			t.Errorf("session CATID = %q, want the 96 address prefix", caps.CATID)
		}
	})

	t.Run("a moved receiver times out rather than being misattributed", func(t *testing.T) {
		// This program ships no CI-V address flag, so a receiver moved
		// off 96h is unreachable. The failure must be a timeout on the
		// identity probe, never a claim about a different model.
		radio := fakeicr8600.New(fakeicr8600.WithRadioAddress(0x94))
		defer radio.Close()
		_, err := newFakeDriver(RealHardware, false).Open(context.Background(), radio.Port(), driver.Identity{})
		if !errors.Is(err, transport.ErrTimeout) {
			t.Fatalf("Open error = %v, want ErrTimeout", err)
		}
		if errors.Is(err, driver.ErrWrongRadio) {
			t.Error("a moved receiver was misattributed to a different model")
		}
		if !strings.Contains(err.Error(), "19 00 identity probe") {
			t.Errorf("Open error = %q, want the identity-probe context", err)
		}
		if got := len(radio.Frames()); got == 0 {
			t.Error("the fake saw no frames at all; the probe never reached the wire")
		}
	})
}
