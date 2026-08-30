// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7760"
)

// THIS FILE IS WHERE TWO INDEPENDENT READINGS OF THE SAME EVIDENCE MEET.
//
// internal/fakeic7760 was written by an implementer who read the frozen
// transcription and geometry witness in core/civ/ic7760/testdata and never
// opened core/civ/ic7760's profile or this package. Its RecordLen is
// derived from the transcription's own width_bytes column; this driver's
// 25 comes from the plan's table, read off the same page by a different
// route. The scripted port in respondingport_test.go is a TABLE and will
// answer whatever a test tells it to; this fake holds STATE and answers
// what it believes, which is why it is the right instrument for the
// end-to-end contract and the wrong one for the error paths.
//
// A DISAGREEMENT HERE IS A STOP FOR ARBITRATION AGAINST THE PDF — one of
// the two readings is wrong — and is never settled by editing the fake to
// agree with the codec.

// fakeSlot maps this driver's slot names onto the fake's own channel
// numbering. Written out rather than reached for through the driver's
// slotToAddress, so a seeding mistake in the driver cannot make a test
// agree with itself.
func fakeSlot(t *testing.T, slot string) int {
	t.Helper()
	switch slot {
	case "P1":
		return fakeic7760.ChanP1
	case "P2":
		return fakeic7760.ChanP2
	}
	var n int
	if _, err := fmt.Sscanf(slot, "%03d", &n); err != nil || n < 1 || n > 99 {
		t.Fatalf("not a memory slot on this radio: %q", slot)
	}
	return n
}

// everySlot is the whole addressable inventory in read order: 99 memories
// then the two programmed scan edges. There is no CALL bank.
func everySlot() []string {
	out := make([]string, 0, 101)
	for ch := 1; ch <= 99; ch++ {
		out = append(out, fmt.Sprintf("%03d", ch))
	}
	return append(out, "P1", "P2")
}

// e2eIndex is a slot's position in the two-byte selector's one contiguous
// space: 1..99 for the memories, 100 for P1 and 101 for P2. It is what the
// seeded records are keyed on, and it is deliberately NOT the fake's own
// channel numbering, which puts the scan edges on negative constants.
func e2eIndex(t *testing.T, slot string) int {
	t.Helper()
	switch slot {
	case "P1":
		return 100
	case "P2":
		return 101
	}
	var n int
	if _, err := fmt.Sscanf(slot, "%03d", &n); err != nil || n < 1 || n > 99 {
		t.Fatalf("not a memory slot on this radio: %q", slot)
	}
	return n
}

// e2eRecord is one 25-byte record whose values VARY WITH ch, so a read-all
// that returned the same record for every slot, or the right records in the
// wrong order, fails rather than passing on a constant.
//
// The bytes are laid out here from the frozen geometry witness, not built
// by the codec under test.
func e2eRecord(ch int) []byte {
	rec := append([]byte(nil), goldenRecord...)
	// Frequency, little-endian packed BCD: 14.2 MHz plus ch kHz, so the
	// 1 kHz and 100 Hz nibbles carry the channel number.
	rec[1] = 0x00
	rec[2] = byte((ch%10)<<4 | 0x00)
	rec[3] = byte(0x20 | (ch / 10 % 10))
	rec[4] = 0x14
	rec[5] = 0x00
	// A name that names the channel, space-padded to the printed ten.
	name := fmt.Sprintf("CH%03d", ch)
	copy(rec[15:25], bytes.Repeat([]byte{0x20}, 10))
	copy(rec[15:], name)
	return rec
}

// newFakeRadio builds a fake and registers its teardown.
func newFakeRadio(t *testing.T, opts ...fakeic7760.Option) *fakeic7760.Radio {
	t.Helper()
	r := fakeic7760.New(opts...)
	t.Cleanup(func() {
		r.StopFloods()
		_ = r.Close()
	})
	return r
}

// openFakeRadio opens a Simulated session against the fake.
func openFakeRadio(t *testing.T, r *fakeic7760.Radio, opts ...Option) *Session {
	t.Helper()
	sess, err := New(Simulated, opts...).Open(t.Context(), r.Port(), driver.Identity{Port: "/dev/fake"})
	if err != nil {
		t.Fatalf("Open against the fake: %v", err)
	}
	s, ok := sess.(*Session)
	if !ok {
		t.Fatalf("Open returned %T, want *Session", sess)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// seededRadio is a fake with every one of the 101 slots populated.
func seededRadio(t *testing.T, opts ...fakeic7760.Option) *fakeic7760.Radio {
	t.Helper()
	r := newFakeRadio(t, opts...)
	for _, slot := range everySlot() {
		r.SetSlot(fakeSlot(t, slot), fakeic7760.MemState{Raw: e2eRecord(e2eIndex(t, slot))})
	}
	return r
}

// TestE2E_ReadsEverySlotAndWritesOneBack is the acceptance contract: the
// driver against a radio that holds state rather than a table that answers
// per frame.
//
// ALL 101 SLOTS, each checked against the record THIS TEST seeded, so a
// read that returned the right shape for the wrong channel fails.
func TestE2E_ReadsEverySlotAndWritesOneBack(t *testing.T) {
	r := seededRadio(t)
	s := openFakeRadio(t, r, WithConsentedUnverifiedWrites())

	if n, confirmed := s.Fingerprint(); !confirmed || n != 25 {
		t.Fatalf("Fingerprint() = (%d, %v), want (25, true) — the fake derived its record length from the evidence independently", n, confirmed)
	}

	for _, slot := range everySlot() {
		ch, err := s.ReadChannel(t.Context(), slot)
		if err != nil {
			t.Fatalf("ReadChannel(%q): %v", slot, err)
		}
		if ch.Empty() {
			t.Fatalf("ReadChannel(%q) came back empty; the slot was seeded", slot)
		}
		if ch.Slot != slot {
			t.Errorf("ReadChannel(%q).Slot = %q", slot, ch.Slot)
		}
		if want := fmt.Sprintf("CH%03d", e2eIndex(t, slot)); ch.Data.Tag != want {
			t.Errorf("ReadChannel(%q).Tag = %q, want %q", slot, ch.Data.Tag, want)
		}
	}

	// ONE WRITE AND ITS READBACK, through the radio's own state: what comes
	// back is what the radio stored, not what this test remembered.
	target := "042"
	write := goodChannel(target)
	write.Data.Tag = "E2E WRITE"
	write.Data.FreqHz = 21_030_000
	res, err := s.WriteChannel(t.Context(), write)
	if err != nil {
		t.Fatalf("WriteChannel(%q): %v", target, err)
	}
	if len(res.Steps) != 1 || !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Fatalf("Steps = %+v, want one sent-and-confirmed step (the fake answers FB)", res.Steps)
	}
	back, err := s.ReadChannel(t.Context(), target)
	if err != nil {
		t.Fatalf("readback of %q: %v", target, err)
	}
	if back.Empty() || back.Data.Tag != "E2E WRITE" || back.Data.FreqHz != 21_030_000 {
		t.Errorf("readback = %+v, want the record just written", back.Data)
	}

	// THE RADIO SAW TWO COMMANDS AND NO OTHERS. No 1A 05, no transceive
	// set, no clear, no 0B — Init is a drain alone and nothing else in this
	// driver builds another grammar.
	for _, cmd := range r.CommandLog() {
		if cmd != [2]byte{0x1A, 0x00} && cmd != [2]byte{0x19, 0x00} {
			t.Errorf("the radio was sent command % X; only 19 00 and 1A 00 are admitted", cmd)
		}
	}
}

// TestE2E_SiblingRecordLengthIsRefused: a radio on this port whose memory
// records are not 25 bytes fails the open with a named reason.
//
// THE SIBLING CASE IS THE POINT. IC-7610, IC-7760 and IC-7851/IC-7850 share
// a flat two-byte address and a 25-byte record, so record length cannot
// tell them apart and this driver claims no such distinctness — that is a
// Wave-4 tier check. What it CAN say is that a record at a length this
// profile does not declare is not one it will parse, and the refusal says
// what was measured, what was expected, and that the expectation is itself
// ASSUMED under ic7760-record-length.
func TestE2E_SiblingRecordLengthIsRefused(t *testing.T) {
	for _, n := range []int{24, 26} {
		r := newFakeRadio(t, fakeic7760.WithRecordLength(n))
		r.SetSlot(1, fakeic7760.MemState{Raw: make([]byte, n)})
		sess, err := New(Simulated).Open(t.Context(), r.Port(), driver.Identity{Port: "/dev/fake"})
		if err == nil {
			_ = sess.Close()
			t.Fatalf("%d-byte records: Open succeeded", n)
		}
		var mismatch *RecordLengthMismatchError
		if !errors.As(err, &mismatch) || mismatch.Got != n || mismatch.Want != 25 {
			t.Errorf("%d-byte records: err = %v, want a RecordLengthMismatchError of %d/25", n, err, n)
		}
		if !errors.Is(err, driver.ErrWrongRadio) {
			t.Errorf("%d-byte records: err does not satisfy errors.Is(err, driver.ErrWrongRadio)", n)
		}
	}
}

// TestE2E_ShortSetIsRefusedByTheRadio pins the other half of the length
// contract, at the WRITE end and from the radio's side.
//
// The fake enforces ic7760-write-full-record: the guide prints one
// full-record set form and no statement permitting a short one, so a set
// that does not carry the whole layout is answered FA. This driver always
// sends all 25 record bytes, which is why the ordinary write above is
// accepted; the frame built by hand below is what the refusal looks like,
// and it never leaves this test.
func TestE2E_ShortSetIsRefusedByTheRadio(t *testing.T) {
	r := seededRadio(t)
	s := openFakeRadio(t, r, WithConsentedUnverifiedWrites())

	before, _ := r.Record(42)
	short := append([]byte{0xFE, 0xFE, fakeic7760.AddrRadio, fakeic7760.AddrController, 0x1A, 0x00, 0x00, 0x42},
		append(e2eRecord(42)[:24], 0xFD)...)
	if _, err := r.Port().Write(short); err != nil {
		t.Fatalf("writing the short set to the radio: %v", err)
	}
	// The session's next exchange is what proves the radio kept its record:
	// a refused set changes nothing.
	ch, err := s.ReadChannel(t.Context(), "042")
	if err != nil {
		t.Fatalf("ReadChannel after the refused short set: %v", err)
	}
	after, _ := r.Record(42)
	if !bytes.Equal(before, after) {
		t.Errorf("the radio's record changed under a short set:\n before % X\n after  % X", before, after)
	}
	if ch.Empty() || ch.Data.Tag != "CH042" {
		t.Errorf("the record read back is %+v, want the seeded CH042", ch.Data)
	}
}

// TestE2E_EraseIsRefusedBeforeAnyWire: erase is Channel.Data == nil, the
// sole discriminator between empty and populated, and FieldErase carries
// the zero FieldSupport in both profiles.
// spec.ConsentUnverifiedWrites structurally never consents it, so the
// refusal stands WITH consent applied and the radio is never asked.
func TestE2E_EraseIsRefusedBeforeAnyWire(t *testing.T) {
	r := seededRadio(t)
	s := openFakeRadio(t, r, WithConsentedUnverifiedWrites())
	commandsBefore := len(r.CommandLog())
	before, _ := r.Record(42)

	res, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: "042", Data: nil})
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("err = %v, want ErrWriteRefused", err)
	}
	var refusal *driver.WriteRefusedError
	if errors.As(err, &refusal) && !containsField(refusal.Fields, spec.FieldErase) {
		t.Errorf("the refusal names %v, want it to name erase", refusal.Fields)
	}
	if len(res.Steps) != 0 {
		t.Errorf("Steps = %+v, want empty — nothing may reach the wire", res.Steps)
	}
	if got := len(r.CommandLog()); got != commandsBefore {
		t.Errorf("the radio logged %d new commands, want 0", got-commandsBefore)
	}
	if after, _ := r.Record(42); !bytes.Equal(before, after) {
		t.Error("the radio's record changed during a refused erase")
	}
}

// TestE2E_EchoSuppressionIsByteIdentity runs the whole contract again over
// a port that reflects everything written to it, which is what a USB CI-V
// endpoint with echo-back on does.
//
// SUPPRESSION IS BY BYTE IDENTITY, never by position or count: the
// accumulator drops a frame because it byte-equals one this programme
// recorded sending, so a session over an echoing port behaves EXACTLY as it
// does over a silent one and the echo shows up only as a counter.
func TestE2E_EchoSuppressionIsByteIdentity(t *testing.T) {
	r := seededRadio(t, fakeic7760.WithEchoDefault(true))
	s := openFakeRadio(t, r, WithConsentedUnverifiedWrites())

	if n, confirmed := s.Fingerprint(); !confirmed || n != 25 {
		t.Fatalf("Fingerprint() = (%d, %v) over an echoing port, want (25, true)", n, confirmed)
	}
	for _, slot := range []string{"001", "042", "099", "P1", "P2"} {
		ch, err := s.ReadChannel(t.Context(), slot)
		if err != nil {
			t.Fatalf("ReadChannel(%q) over an echoing port: %v", slot, err)
		}
		if ch.Empty() || ch.Data.Tag != fmt.Sprintf("CH%03d", e2eIndex(t, slot)) {
			t.Errorf("ReadChannel(%q) = %+v over an echoing port", slot, ch.Data)
		}
	}
	if _, err := s.WriteChannel(t.Context(), goodChannel("042")); err != nil {
		t.Fatalf("WriteChannel over an echoing port: %v", err)
	}
	stats := s.WireStats()
	if stats.Echoes == 0 {
		t.Fatal("the adapter suppressed no echoes; the echoing port is not echoing and this test proves nothing")
	}
	if s.AnswerMismatches() != 0 {
		t.Errorf("AnswerMismatches() = %d over an echoing port, want 0", s.AnswerMismatches())
	}
}

// TestE2E_AForeignControllerIsIgnored: the radio requires BOTH halves of
// the printed address pair — destination B2 and source E0 — before it will
// answer at all. A frame from another controller on the same bus is on this
// radio's line but is not its business.
//
// It matters here because this DRIVER never sends such a frame: the check
// exists so that a bus carrying another station's traffic cannot produce an
// answer this session might attribute to its own request.
func TestE2E_AForeignControllerIsIgnored(t *testing.T) {
	r := seededRadio(t)
	s := openFakeRadio(t, r)
	commandsBefore := len(r.CommandLog())

	foreign := []byte{0xFE, 0xFE, fakeic7760.AddrRadio, 0x94, 0x1A, 0x00, 0x00, 0x42, 0xFD}
	if _, err := r.Port().Write(foreign); err != nil {
		t.Fatalf("writing the foreign controller's read: %v", err)
	}
	// The session's own next exchange must be unaffected, and must get its
	// own answer rather than one raised by that frame.
	ch, err := s.ReadChannel(t.Context(), "001")
	if err != nil {
		t.Fatalf("ReadChannel after a foreign controller's frame: %v", err)
	}
	if ch.Empty() || ch.Data.Tag != "CH001" {
		t.Errorf("ReadChannel(\"001\") = %+v, want the seeded CH001", ch.Data)
	}
	if s.AnswerMismatches() != 0 {
		t.Errorf("AnswerMismatches() = %d, want 0", s.AnswerMismatches())
	}
	if got := len(r.CommandLog()) - commandsBefore; got != 0 {
		t.Errorf("the radio logged %d commands from a foreign controller's frame, want 0", got)
	}
}

// TestE2E_AnEmptyRadioOpensAndReadsEmpty: a radio with nothing in it opens
// on address evidence alone and every slot reads back EMPTY rather than as
// an error.
//
// The empty answer is FA — assumed under ic7760-empty-reply-fa, whose lift
// clears memory 99 and reads it. Refusing to open here would make a radio
// whose memories are all empty unprogrammable by this programme, which is
// precisely the radio a user most wants to programme.
func TestE2E_AnEmptyRadioOpensAndReadsEmpty(t *testing.T) {
	r := newFakeRadio(t)
	s := openFakeRadio(t, r)
	report := s.OpenDiagnostics()
	if report.Fingerprinted || report.SlotsTried != 12 {
		t.Fatalf("OpenDiagnostics = %+v, want an un-fingerprinted open after the whole bounded schedule", report)
	}
	for _, slot := range []string{"001", "099", "P1", "P2"} {
		ch, err := s.ReadChannel(t.Context(), slot)
		if err != nil {
			t.Fatalf("ReadChannel(%q) on an empty radio: %v", slot, err)
		}
		if !ch.Empty() {
			t.Errorf("ReadChannel(%q) = %+v, want an EMPTY channel", slot, ch.Data)
		}
	}
}
