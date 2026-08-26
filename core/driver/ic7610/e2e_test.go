// SPDX-License-Identifier: GPL-3.0-or-later

package ic7610

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	civic7610 "github.com/gm5dna/open-rig-programmer/core/civ/ic7610"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
	"github.com/gm5dna/open-rig-programmer/internal/fakeic7610"
)

// THIS FILE IS WHERE TWO INDEPENDENT READINGS OF PDF P.12 MEET.
//
// internal/fakeic7610 was written by an implementer who read the committed
// evidence — the transcription and the geometry witness — and never opened
// core/civ/ic7610 or this package. Its RecordLen is DERIVED from the
// transcription's own width_bytes column (2+1+5+2+1+3+3+10 = 27, less the
// two selector bytes = 25); this driver's 25 comes from the plan's ONE
// TABLE, read off the same page by a different route. The two agree, and
// TestE2E_ProbeFingerprints is where that agreement is observed rather
// than assumed.
//
// A DISAGREEMENT HERE IS A STOP FOR ORCHESTRATOR ARBITRATION AGAINST THE
// PDF — either the fake or the codec misreads the page — and is NEVER
// fixed by editing the fake to match the codec. internal/fakeic7610 is
// frozen to this worktree exactly as core/civ/ic7610's testdata is.

// The ten printed mode codes and the three filter codes, in the wire form
// this file seeds records with. Written out here from PDF p.11's
// "①Receiving mode" and "Filter setting" columns — NOT taken from the
// codec, whose enum tables are what these records are used to exercise.
//
// The last two codes are RULING OQ1's: hexadecimal, so PSK is 0x12 and
// PSK-R is 0x13.
var (
	e2eModes = []struct {
		code byte
		name string
	}{
		{0x00, "LSB"}, {0x01, "USB"}, {0x02, "AM"}, {0x03, "CW"}, {0x04, "RTTY"},
		{0x05, "FM"}, {0x07, "CW-R"}, {0x08, "RTTY-R"}, {0x12, "PSK"}, {0x13, "PSK-R"},
	}
	e2eFilters = []struct {
		code byte
		name string
	}{{0x01, "FIL1"}, {0x02, "FIL2"}, {0x03, "FIL3"}}
	e2eToneModes = []struct {
		code byte
		name string
	}{{0x00, "OFF"}, {0x01, "TONE"}, {0x02, "TSQL"}}
)

// e2eFields is one seeded channel's content in NEUTRAL terms, so a test
// states what it put in the radio rather than what a builder produced.
type e2eFields struct {
	freqHz   uint64
	mode     string
	filter   string
	toneMode string
	toneTx   uint64 // deci-Hz
	toneRx   uint64 // deci-Hz
	name     string
}

// bcd5le renders a frequency in hertz as the record's five-byte
// LITTLE-endian packed BCD: PDF p.11's five-cell strip runs 10 Hz, 1 Hz,
// 1 kHz, 100 Hz, 100 kHz, 10 kHz, 10 MHz, 1 MHz, then a fixed "0 : 0" —
// least significant pair first. Written out here rather than taken from
// the encoder under test.
func bcd5le(hz uint64) []byte {
	out := make([]byte, 5)
	for i := 0; i < 5; i++ {
		lo := byte(hz % 10)
		hz /= 10
		hi := byte(hz % 10)
		hz /= 10
		out[i] = hi<<4 | lo
	}
	return out
}

// record assembles the 25-byte record for f, BY OFFSET, from the plan's
// ONE TABLE. The name is space-padded to ten bytes (register entries
// ic7610-name-space-character and ic7610-name-pad-byte, both ASSUMED).
func (f e2eFields) record(t *testing.T) []byte {
	t.Helper()
	rec := make([]byte, civic7610.RecordOnlyLength)
	// offset 0: UNMAPPED (E6). Zero, matching the profile's Fixed template.
	copy(rec[1:6], bcd5le(f.freqHz))
	rec[6] = codeFor(t, "mode", f.mode)
	rec[7] = codeFor(t, "filter", f.filter)
	// offset 8: high nibble UNMAPPED (E6, data mode OFF), low nibble tone mode.
	rec[8] = codeFor(t, "tone_mode", f.toneMode) & 0x0F
	copy(rec[9:12], bcd3(f.toneTx))
	copy(rec[12:15], bcd3(f.toneRx))
	for i := 15; i < 25; i++ {
		rec[i] = 0x20
	}
	copy(rec[15:25], f.name)
	return rec
}

func codeFor(t *testing.T, kind, name string) byte {
	t.Helper()
	var table []struct {
		code byte
		name string
	}
	switch kind {
	case "mode":
		table = e2eModes
	case "filter":
		table = e2eFilters
	case "tone_mode":
		table = e2eToneModes
	}
	for _, e := range table {
		if e.name == name {
			return e.code
		}
	}
	t.Fatalf("no %s code for %q", kind, name)
	return 0
}

// e2eSeed is the deterministic content of one slot, varied across every
// field the record maps so no two channels share a value by accident.
func e2eSeed(i int) e2eFields {
	m := e2eModes[i%len(e2eModes)]
	fl := e2eFilters[i%len(e2eFilters)]
	tm := e2eToneModes[i%len(e2eToneModes)]
	return e2eFields{
		freqHz:   1_800_000 + uint64(i)*137_000,
		mode:     m.name,
		filter:   fl.name,
		toneMode: tm.name,
		// Both tones stay INSIDE the declared {1, 2999, 1} domain, so
		// both come back Known; the out-of-domain arm is read_test.go's.
		toneTx: uint64(670 + i*7),
		toneRx: uint64(885 + i*11),
		name:   fmt.Sprintf("CH%03d TEST", i),
	}
}

// e2eSlots is every slot this radio declares, in capability order.
func e2eSlots() []string {
	var out []string
	for _, b := range capabilitiesUnverified().Banks {
		out = append(out, b.Slots...)
	}
	return out
}

// fakeChannel maps a canonical slot to the fake's own channel argument.
// The fake numbers the scan edges NEGATIVELY on purpose — ChanP1 = -1,
// ChanP2 = -2 — so that no arithmetic on a memory channel can land on a
// scan edge by accident. This driver addresses them as 100 and 101,
// because those are the BCD selectors the page prints. The two numbering
// schemes are independent and this function is the only place they meet.
func fakeChannel(t *testing.T, slot string) int {
	t.Helper()
	switch slot {
	case "P1":
		return fakeic7610.ChanP1
	case "P2":
		return fakeic7610.ChanP2
	}
	a, _, err := slotToAddress(slot)
	if err != nil {
		t.Fatalf("slotToAddress(%q): %v", slot, err)
	}
	return a.Channel
}

// openFake opens a driver session against r, registering cleanup.
func openFake(t *testing.T, r *fakeic7610.Radio, opts ...Option) *Session {
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

// newFake builds a fake and registers its cleanup.
func newFake(t *testing.T, opts ...fakeic7610.Option) *fakeic7610.Radio {
	t.Helper()
	r := fakeic7610.New(opts...)
	t.Cleanup(func() {
		r.StopFloods()
		_ = r.Close()
	})
	return r
}

// TestE2E_ProbeFingerprints is the independence check landing.
//
// The fake's RecordLen is 25, DERIVED by its author from the
// transcription's own width_bytes column without ever seeing this
// driver's ONE TABLE. This driver's probe measures whatever the radio
// answers with and confirms 25. Neither number was copied from the other,
// and a disagreement here would be a STOP for arbitration against PDF
// p.12 — never a fix to the fake.
//
// THE ID TOKEN IS RECORDED, NEVER MATCHED (D5 entry 7, matrix lift R7),
// and the fake's default proves it: 0xA5 is DELIBERATELY IMPLAUSIBLE, an
// alternating bit pattern chosen so that a driver which matched the token
// would fail loudly against a guess. This driver opens anyway, because
// what identifies the radio is that an ADDRESS-MATCHED reply arrived at
// all.
func TestE2E_ProbeFingerprints(t *testing.T) {
	t.Run("both record lengths are re-derived from the frozen artefact", func(t *testing.T) {
		record, dataArea, selector := recordLengthFromTranscription(t)

		// THE ARTEFACT IS THE AUTHORITY, and neither constant may agree
		// with the other while both disagree with it. Comparing the two
		// constants ALONE would leave a corrected transcription with two
		// green-and-wrong packages — which is the class Stage 1's own
		// 584bb02 ("the evidence that was merely stored is now consumed")
		// closed on the civ side, and it applies here for the same reason.
		if fakeic7610.RecordLen != record {
			t.Errorf("STOP — the fake's RecordLen is %d and the transcription's D1 widths derive %d. "+
				"The fake's author derived theirs from this same CSV without ever seeing this driver; "+
				"a disagreement is orchestrator arbitration AGAINST THE PDF, never an edit to either side",
				fakeic7610.RecordLen, record)
		}
		if civic7610.RecordOnlyLength != record {
			t.Errorf("STOP — this driver's RecordOnlyLength is %d and the transcription's D1 widths derive %d. "+
				"The plan's ONE TABLE and the B leg's own widths must agree; a disagreement is arbitration "+
				"against PDF p.12, never a constant moved to match",
				civic7610.RecordOnlyLength, record)
		}
		// The other two figures spec Erratum 1 requires stated TOGETHER,
		// with the address width named — pinned to the same artefact, so
		// the whole accounting stands or falls on one reading.
		if civic7610.DataAreaLength != dataArea {
			t.Errorf("DataAreaLength = %d, and the D1 widths sum to %d", civic7610.DataAreaLength, dataArea)
		}
		if civic7610.AddressBytes != selector {
			t.Errorf("AddressBytes = %d, and the transcription's selector row is %d bytes wide", civic7610.AddressBytes, selector)
		}
		if record != dataArea-selector {
			t.Errorf("the derivation is incoherent: %d != %d - %d", record, dataArea, selector)
		}
	})

	for _, tt := range []struct {
		name  string
		opts  []fakeic7610.Option
		token string
	}{
		{"the fake's deliberately implausible default token", nil, "98a5"},
		{"a supplied single-byte token", []fakeic7610.Option{fakeic7610.WithIDToken([]byte{0x98})}, "9898"},
		{"a supplied two-byte token", []fakeic7610.Option{fakeic7610.WithIDToken([]byte{0x00, 0x7c})}, "98007c"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := newFake(t, tt.opts...)
			seed := e2eSeed(1)
			r.SetSlot(1, fakeic7610.MemState{Raw: seed.record(t)})

			s := openFake(t, r)
			if got := s.Identity().CATID; got != tt.token {
				t.Errorf("Identity().CATID = %q, want %q", got, tt.token)
			}
			length, confirmed := s.Fingerprint()
			if !confirmed || length != 25 {
				t.Errorf("Fingerprint() = (%d, %v), want (25, true)", length, confirmed)
			}
		})
	}
}

// TestE2E_EmptyRadioOpensUnfingerprinted: a fake with no channel seeded
// answers NG to every probe read, so there is no record to measure. The
// session opens ON ADDRESS EVIDENCE ALONE (spec D3.2, D5 entry 2(a),
// matrix lift R2a).
//
// The FA reaches this driver as transport.ErrRejected with NO FRAME (tier
// ruling T4) — the engine consumes it — which is why the driver's
// keep-looking branch keys on the error and never on "an FA arrived".
func TestE2E_EmptyRadioOpensUnfingerprinted(t *testing.T) {
	r := newFake(t)
	s := openFake(t, r)
	if length, confirmed := s.Fingerprint(); confirmed || length != 0 {
		t.Errorf("Fingerprint() = (%d, %v), want (0, false)", length, confirmed)
	}
	rep := s.OpenDiagnostics()
	if rep.Fingerprinted || rep.SlotsTried != probeSlotCount {
		t.Errorf("OpenDiagnostics() = %+v, want UNFINGERPRINTED with the whole bounded search run", rep)
	}

	// AND EVERY SLOT READS BACK EMPTY, not as an error. This is T4 end to
	// end on a radio that produces the FA itself: an empty slot must never
	// be an error that aborts a caller's ReadAll, and the fake answers NG
	// for an unset memory AND for an unset scan edge (which is WIDER than
	// matrix lift R2a's scope — R2a excludes the scan edges, so P1/P2
	// emptiness rides lift R18, and R18's capture names P1 only, leaving
	// P2 uncovered even by that).
	for _, slot := range e2eSlots() {
		ch, err := s.ReadChannel(t.Context(), slot)
		if err != nil {
			t.Fatalf("ReadChannel %s on an empty radio: %v — an unset slot must come back empty, not as an error", slot, err)
		}
		if !ch.Empty() {
			t.Errorf("ReadChannel %s returned %+v, want an EMPTY channel (Data == nil is the sole discriminator)", slot, ch.Data)
		}
		if ch.Slot != slot {
			t.Errorf("ReadChannel %s carried slot %q", slot, ch.Slot)
		}
	}
}

// checkReadAll seeds every declared slot, reads each back through the
// driver, and compares against what the test put in. Shared by the plain
// run and the USB-echo run.
func checkReadAll(t *testing.T, opts ...fakeic7610.Option) {
	t.Helper()
	r := newFake(t, opts...)
	slots := e2eSlots()
	if len(slots) != 101 {
		t.Fatalf("this radio declares %d slots, want 101 (99 memories + 2 scan edges)", len(slots))
	}
	want := make(map[string]e2eFields, len(slots))
	for i, slot := range slots {
		f := e2eSeed(i)
		want[slot] = f
		r.SetSlot(fakeChannel(t, slot), fakeic7610.MemState{Raw: f.record(t)})
	}

	s := openFake(t, r)
	for _, slot := range slots {
		ch, err := s.ReadChannel(t.Context(), slot)
		if err != nil {
			t.Fatalf("ReadChannel %s: %v", slot, err)
		}
		if ch.Empty() {
			t.Fatalf("ReadChannel %s came back empty; the slot was seeded", slot)
		}
		f, d := want[slot], ch.Data

		// EVERY MAPPED FIELD MUST SURVIVE THE ROUND TRIP.
		if d.FreqHz != f.freqHz {
			t.Errorf("%s FreqHz = %d, want %d", slot, d.FreqHz, f.freqHz)
		}
		if d.Mode != f.mode {
			t.Errorf("%s Mode = %q, want %q", slot, d.Mode, f.mode)
		}
		if d.Filter != (codeplug.StringField{State: codeplug.Known, Value: f.filter}) {
			t.Errorf("%s Filter = %+v, want Known %q", slot, d.Filter, f.filter)
		}
		if d.ToneMode != (codeplug.StringField{State: codeplug.Known, Value: f.toneMode}) {
			t.Errorf("%s ToneMode = %+v, want Known %q", slot, d.ToneMode, f.toneMode)
		}
		if d.ToneTx != (codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(f.toneTx)}) {
			t.Errorf("%s ToneTx = %+v, want Known %d", slot, d.ToneTx, f.toneTx)
		}
		if d.ToneRx != (codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(f.toneRx)}) {
			t.Errorf("%s ToneRx = %+v, want Known %d", slot, d.ToneRx, f.toneRx)
		}
		if d.Tag != f.name {
			t.Errorf("%s Tag = %q, want %q", slot, d.Tag, f.name)
		}

		// E6: the two unmapped nibbles are Unavailable throughout.
		if d.ScanSkip.State != codeplug.Unavailable || d.DataMode.State != codeplug.Unavailable {
			t.Errorf("%s ScanSkip/DataMode = %+v/%+v, want Unavailable — an unmapped region is never decoded", slot, d.ScanSkip, d.DataMode)
		}
		// Every field the record does not carry is Unavailable.
		for name, state := range map[string]codeplug.FieldState{
			"TagDisplay":   d.TagDisplay.State,
			"CTCSSTone":    d.CTCSSTone.State,
			"TxFreqHz":     d.TxFreqHz.State,
			"Duplex":       d.Duplex.State,
			"OffsetHz":     d.OffsetHz.State,
			"DTCSCode":     d.DTCSCode.State,
			"DTCSPolarity": d.DTCSPolarity.State,
		} {
			if state != codeplug.Unavailable {
				t.Errorf("%s %s = %q, want Unavailable", slot, name, state)
			}
		}
	}
}

// TestE2E_ReadAll rounds every declared slot — all 99 memories and both
// scan edges — through the fake and back.
func TestE2E_ReadAll(t *testing.T) { checkReadAll(t) }

// checkWriteOne writes one channel with consent and asserts THE FAKE'S
// STORED BYTES, not just the driver's WriteResult. Asserting the stored
// bytes is what makes this an independence check rather than a self-check:
// the driver's encoder and the test's own record builder are two separate
// readings of the same page, meeting in the fake's memory.
func checkWriteOne(t *testing.T, opts ...fakeic7610.Option) {
	t.Helper()
	r := newFake(t, opts...)
	// Seed the target so the E6 preservation read finds a record whose
	// unmapped regions are already the template's zeros.
	prior := e2eSeed(3)
	r.SetSlot(1, fakeic7610.MemState{Raw: e2eSeed(1).record(t)})
	r.SetSlot(42, fakeic7610.MemState{Raw: prior.record(t)})

	s := openFake(t, r, WithConsentedUnverifiedWrites())
	want := e2eFields{
		freqHz: 14_250_000, mode: "USB", filter: "FIL1", toneMode: "TONE",
		toneTx: 885, toneRx: 1000, name: "HOME QTH01",
	}
	res, err := s.WriteChannel(t.Context(), codeplug.Channel{
		Slot: "042",
		Data: &codeplug.ChannelData{
			FreqHz:   want.freqHz,
			Mode:     want.mode,
			Tag:      want.name,
			Filter:   codeplug.StringField{State: codeplug.Known, Value: want.filter},
			ToneMode: codeplug.StringField{State: codeplug.Known, Value: want.toneMode},
			ToneTx:   codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(want.toneTx)},
			ToneRx:   codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(want.toneRx)},
		},
	})
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	// The driver saw FB.
	if len(res.Steps) != 1 || !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want one sent, confirmed step", res.Steps)
	}

	// AND THE FAKE HOLDS THE BYTES THE TEST MEANT.
	got, ok := r.SlotState(42)
	if !ok {
		t.Fatal("the fake reports channel 42 unset after an acknowledged write")
	}
	if !bytes.Equal(got.Raw, want.record(t)) {
		t.Errorf("the fake stored\n  % x\nwant\n  % x", got.Raw, want.record(t))
	}
	if len(got.Raw) != fakeic7610.RecordLen {
		t.Errorf("the fake stored %d bytes, want the full %d — register entry ic7610-full-record-mandatory, matrix lift R15", len(got.Raw), fakeic7610.RecordLen)
	}
}

// TestE2E_WriteOne is the write half of the round trip.
func TestE2E_WriteOne(t *testing.T) { checkWriteOne(t) }

// TestE2E_WriteIsRefusedForASelectGroupChannel is E6's COST, proven end to
// end against an independently written radio.
//
// A CHANNEL IN A SELECT GROUP (★1/★2/★3), OR WHOSE DATA MODE IS
// DATA 1/2/3, CANNOT BE WRITTEN BY THIS PROGRAMME AT ALL. The assertion
// that matters is the second one: THE FAKE'S STORED BYTES ARE UNCHANGED.
// The channel is not downgraded to ★1, not cleared to OFF, and not
// rewritten in any other way.
func TestE2E_WriteIsRefusedForASelectGroupChannel(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func([]byte)
		offset int
		nibble string
	}{
		{"SELECT ★2 in byte 0's low nibble", func(b []byte) { b[0] = 0x02 }, civic7610.SelectNibbleOffset, "low"},
		{"SELECT ★3 in byte 0's low nibble", func(b []byte) { b[0] = 0x03 }, civic7610.SelectNibbleOffset, "low"},
		{"DATA 2 in byte 8's high nibble", func(b []byte) { b[8] = 0x20 | (b[8] & 0x0F) }, civic7610.DataModeNibbleOffset, "high"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := newFake(t)
			r.SetSlot(1, fakeic7610.MemState{Raw: e2eSeed(1).record(t)})
			prior := e2eSeed(5).record(t)
			tt.mutate(prior)
			r.SetSlot(42, fakeic7610.MemState{Raw: append([]byte(nil), prior...)})

			s := openFake(t, r, WithConsentedUnverifiedWrites())
			before := r.BytesWritten()

			res, err := s.WriteChannel(t.Context(), goodChannel("042"))
			var e *UnmappedRegionError
			if !errors.As(err, &e) {
				// Errorf, not Fatalf: the stored-bytes check below is
				// the one that shows what E6 is FOR, and it must run
				// even when the refusal has gone missing.
				t.Errorf("err = %v, want an *UnmappedRegionError", err)
			} else if e.Offset != tt.offset || e.Nibble != tt.nibble {
				t.Errorf("*UnmappedRegionError = %+v, want offset %d, nibble %q", e, tt.offset, tt.nibble)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty — no frame was ever built", res.Steps)
			}

			// THE RADIO IS UNTOUCHED.
			after, ok := r.SlotState(42)
			if !ok {
				t.Fatal("the fake reports channel 42 unset after a REFUSED write")
			}
			if !bytes.Equal(after.Raw, prior) {
				t.Errorf("the refused write changed the radio: stored\n  % x\nwas\n  % x", after.Raw, prior)
			}
			// Exactly one read's worth of new traffic — tier ruling T5's
			// single recorded exception — and no set frame.
			if grew := len(r.BytesWritten()) - len(before); grew != memReadFrameLen {
				t.Errorf("the refused write put %d new bytes on the wire, want %d (one 1A 00 read)", grew, memReadFrameLen)
			}
		})
	}
}

// TestE2E_WrongRecordLengthIsRefused: WithRecordLength(39) makes the fake
// answer 39-byte records, which this driver's CONTINUOUS length
// fingerprint refuses.
//
// THE REFUSAL NAMES NO FOUND MODEL. The IC-7610 has no registered sibling
// (matrix §4) and this package holds no table of other radios' record
// lengths, so this is a LENGTH DISAGREEMENT and not a cross-model claim;
// cross-model record-length distinctness is a Wave-4 tier check.
func TestE2E_WrongRecordLengthIsRefused(t *testing.T) {
	r := newFake(t, fakeic7610.WithRecordLength(39))
	r.SetSlot(1, fakeic7610.MemState{Raw: make([]byte, 39)})

	sess, err := New(Simulated).Open(t.Context(), r.Port(), driver.Identity{})
	if err == nil {
		_ = sess.Close()
		t.Fatal("Open accepted a 39-byte record")
	}
	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Errorf("err = %v, want one satisfying errors.Is(err, driver.ErrWrongRadio)", err)
	}
	var wrong *driver.WrongRadioError
	if errors.As(err, &wrong) && wrong.GotModel != "" {
		t.Errorf("the refusal names a found model (%q); it must not", wrong.GotModel)
	}
	var mismatch *RecordLengthMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("err = %v, want a *RecordLengthMismatchError", err)
	}
	if mismatch.Got != 39 || mismatch.Want != civic7610.RecordOnlyLength {
		t.Errorf("*RecordLengthMismatchError = %+v, want {Got: 39, Want: 25}", mismatch)
	}
}

// TestE2E_EraseIsRefused — ChannelData HAS NO Erase MEMBER. Erase is
// represented solely by Channel.Data == nil, the sole discriminator
// between empty and populated.
//
// BytesWritten IS CUMULATIVE and Open has already issued probe traffic, so
// the assertion is a BEFORE/AFTER comparison across the WriteChannel call,
// not len(after) == 0.
func TestE2E_EraseIsRefused(t *testing.T) {
	for _, opts := range [][]Option{nil, {WithConsentedUnverifiedWrites()}} {
		r := newFake(t)
		seeded := e2eSeed(1).record(t)
		r.SetSlot(1, fakeic7610.MemState{Raw: append([]byte(nil), seeded...)})

		s := openFake(t, r, opts...)
		before := r.BytesWritten()

		res, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: "001", Data: nil})
		if !errors.Is(err, driver.ErrWriteRefused) {
			t.Fatalf("err = %v, want ErrWriteRefused (spec D4 \"Erase\"; consent structurally never reaches the field)", err)
		}
		// NAMING THE FIELD MATTERS: a Data==nil channel that fell through
		// to the mandatory-field rung would also be refused, but for the
		// wrong reason — "mode is empty" rather than "this radio's clear
		// forms have never been asked of an IC-7610".
		var refusal *driver.WriteRefusedError
		if !errors.As(err, &refusal) || !containsField(refusal.Fields, spec.FieldErase) {
			t.Errorf("err = %v, want a *driver.WriteRefusedError naming erase", err)
		}
		if len(res.Steps) != 0 {
			t.Errorf("Steps = %+v, want empty", res.Steps)
		}
		if after := r.BytesWritten(); len(after) != len(before) {
			t.Errorf("the refused erase put %d new bytes on the wire, want none", len(after)-len(before))
		}
		// And the channel is still there.
		if got, ok := r.SlotState(1); !ok || !bytes.Equal(got.Raw, seeded) {
			t.Errorf("channel 1 was disturbed by a refused erase: %v / % x", ok, got.Raw)
		}
		_ = s.Close()
	}
}

// misaddressingPort sits BETWEEN the fake and the driver and rewrites the
// two channel-selector bytes of every 1A 00 ANSWER it carries, so the
// driver is offered a well-formed record about a channel it did not ask
// about.
//
// WHY A WIRE SHIM AND NOT A FAKE OPTION. The plan asks for the fake to be
// SEEDED to answer channel 5's read with channel 6's address.
// internal/fakeic7610 has no such lever and should not: its author never
// read this driver, so tier ruling T2 was not theirs to know about, and a
// fake that mis-addresses on request would be modelling a broken radio
// rather than an IC-7610. THE FAKE IS FROZEN — a gap in it is a STOP to
// the orchestrator, never an edit — so the misdirection is applied HERE,
// on the wire, which is also where it would happen in life: a bus
// neighbour's answer, or a corrupted selector, arrives at the host port
// exactly like this. Every assertion the plan names is made unchanged.
//
// Deviation recorded in the progress file for the orchestrator's ruling.
type misaddressingPort struct {
	net.Conn
	mu      sync.Mutex
	pending []byte
	to      byte
	lo      byte
	active  bool
}

// activate starts rewriting answers to name the given BCD selector. It is
// off until called, so Open's own probe sees honest answers.
func (p *misaddressingPort) activate(hi, lo byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.to, p.lo, p.active = hi, lo, true
}

func (p *misaddressingPort) Read(b []byte) (int, error) {
	p.mu.Lock()
	if len(p.pending) > 0 {
		n := copy(b, p.pending)
		p.pending = p.pending[n:]
		p.mu.Unlock()
		return n, nil
	}
	p.mu.Unlock()

	buf := make([]byte, len(b))
	n, err := p.Conn.Read(buf)
	if n > 0 {
		out := p.rewrite(buf[:n])
		p.mu.Lock()
		p.pending = append(p.pending, out...)
		n2 := copy(b, p.pending)
		p.pending = p.pending[n2:]
		p.mu.Unlock()
		return n2, err
	}
	return 0, err
}

// rewrite replaces the selector of every complete 1A 00 answer in chunk.
// The frames the fake emits are whole, so a byte-wise scan is enough here;
// a partial frame simply passes through and is rewritten on the next read
// if it completes there.
func (p *misaddressingPort) rewrite(chunk []byte) []byte {
	p.mu.Lock()
	active, hi, lo := p.active, p.to, p.lo
	p.mu.Unlock()
	if !active {
		return append([]byte(nil), chunk...)
	}
	out := append([]byte(nil), chunk...)
	for i := 0; i+8 <= len(out); i++ {
		if out[i] == 0xFE && out[i+1] == 0xFE && out[i+4] == 0x1A && out[i+5] == 0x00 {
			out[i+6], out[i+7] = hi, lo
		}
	}
	return out
}

// TestE2E_AnswerForAnotherChannelIsRefused — TIER RULING T2's MANDATORY
// PER-DRIVER MISMATCH REGRESSION TEST.
//
// The landed civ.Profile.MemoryAnswerMatcher is ENVELOPE-ONLY by design —
// it checks to/from/cn/sc and nothing else — so an answer for channel 6
// satisfies the spec for a read of channel 5 perfectly well. The address
// INSIDE the answer is therefore the DRIVER'S to check, and it is checked
// before any use of the answer.
func TestE2E_AnswerForAnotherChannelIsRefused(t *testing.T) {
	r := newFake(t)
	for i, slot := range []string{"001", "005", "006"} {
		f := e2eSeed(i + 1)
		r.SetSlot(fakeChannel(t, slot), fakeic7610.MemState{Raw: f.record(t)})
	}
	port := &misaddressingPort{Conn: r.Port()}

	sess, err := New(Simulated).Open(t.Context(), port, driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s := sess.(*Session)
	defer func() { _ = s.Close() }()

	// A clean read first, so the mismatch below is the only difference.
	if _, err := s.ReadChannel(t.Context(), "005"); err != nil {
		t.Fatalf("the honest read of 005 failed: %v", err)
	}
	before := s.AnswerMismatches()

	// Now every memory answer names channel 6, whoever asked. BCD "00 06".
	port.activate(0x00, 0x06)

	ch, err := s.ReadChannel(t.Context(), "005")
	if err == nil {
		t.Fatalf("ReadChannel 005 succeeded with %+v; the answer named channel 6", ch)
	}
	if !ch.Empty() {
		t.Error("a populated channel was produced alongside the error; nothing may be mapped from a mis-addressed answer")
	}
	var mismatch *AnswerMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("err = %v, want an *AnswerMismatchError naming both channels", err)
	}
	if mismatch.Want.Channel != 5 || mismatch.Got.Channel != 6 {
		t.Errorf("*AnswerMismatchError = {Want: %s, Got: %s}, want {ch5, ch6}", mismatch.Want, mismatch.Got)
	}
	if got := s.AnswerMismatches(); got != before+1 {
		t.Errorf("the mismatch diagnostic went %d -> %d, want one increment", before, got)
	}
}

// TestE2E_EchoOnChangesNothing proves the structural echo handling end to
// end (spec D3.4): the read-all and write-one suites run identically with
// the radio echoing every received frame back verbatim.
//
// NO ECHO PROBING ANYWHERE. This driver never asks whether echo is on and
// never counts frames to find out; civ's accumulator drops a frame that
// BYTE-EQUALS one NoteSent recorded, which is why the USB-echo-ON,
// USB-echo-OFF and REMOTE-bus cases are one case here. The fake puts its
// echo BEFORE the address filter (its assumption 7), which is the harder
// ordering for a consumer and the one this passes under.
func TestE2E_EchoOnChangesNothing(t *testing.T) {
	t.Run("read-all with echo", func(t *testing.T) { checkReadAll(t, fakeic7610.WithUSBEcho()) })
	t.Run("write-one with echo", func(t *testing.T) { checkWriteOne(t, fakeic7610.WithUSBEcho()) })

	t.Run("the echoes are counted and dropped, not answered", func(t *testing.T) {
		r := newFake(t, fakeic7610.WithUSBEcho())
		r.SetSlot(1, fakeic7610.MemState{Raw: e2eSeed(1).record(t)})
		s := openFake(t, r)
		if _, err := s.ReadChannel(t.Context(), "001"); err != nil {
			t.Fatalf("ReadChannel: %v", err)
		}
		if got := s.WireStats().Echoes; got == 0 {
			t.Error("WireStats().Echoes is zero with the radio echoing every frame — the accumulator's byte-identity suppression is what makes echo a non-event, and it should be visible in the counters")
		}
	})
}

// TestE2E_BroadcastFloodNeverReachesTheEngine — R9-SPLIT half (a), and the
// test REV 2 had BACKWARDS.
//
// civ's accumulator counts a to=00 frame and NEVER RETURNS it, so it never
// becomes an engine event, the drain's idle timer is never re-armed, and
// Engine.Init SUCCEEDS. No drain cap is ever hit. What rises is the
// ADAPTER's own counter — and the engine's own UnexpectedFrames stays at
// zero for those frames, which is exactly why the DIAGNOSTICS CARRIER
// ruling forbids reaching past the adapter for it.
//
// A test expecting a drain-cap event from a to=00 flood is unsatisfiable.
func TestE2E_BroadcastFloodNeverReachesTheEngine(t *testing.T) {
	r := newFake(t, fakeic7610.WithTransceiveFlood(time.Millisecond))
	for i, slot := range e2eSlots() {
		r.SetSlot(fakeChannel(t, slot), fakeic7610.MemState{Raw: e2eSeed(i).record(t)})
	}

	s := openFake(t, r, WithConsentedUnverifiedWrites())
	if s.OpenDiagnostics().InitDrainCapExceeded {
		t.Error("InitDrainCapExceeded is true under a BROADCAST flood — those frames never reach the engine, so no drain can hit its cap on them")
	}
	if _, confirmed := s.Fingerprint(); !confirmed {
		t.Error("the probe did not fingerprint through a broadcast flood")
	}

	engineBefore := s.eng.UnexpectedFrames()
	wireBefore := s.WireStats().Unexpected
	if wireBefore == 0 {
		t.Fatal("WireStats().Unexpected is zero under a broadcast flood — the adapter's counter is the ONLY place this traffic is visible")
	}

	// The session still works, and within the test's own deadline.
	deadline := time.Now().Add(90 * time.Second)
	for _, slot := range e2eSlots() {
		if time.Now().After(deadline) {
			t.Fatal("the read-all did not complete within its deadline under a broadcast flood")
		}
		if _, err := s.ReadChannel(t.Context(), slot); err != nil {
			t.Fatalf("ReadChannel %s under a broadcast flood: %v", slot, err)
		}
	}
	if _, err := s.WriteChannel(t.Context(), goodChannel("042")); err != nil {
		t.Fatalf("WriteChannel under a broadcast flood: %v", err)
	}

	if after := s.WireStats().Unexpected; after <= wireBefore {
		t.Errorf("WireStats().Unexpected went %d -> %d, want it rising while the flood runs", wireBefore, after)
	}
	if after := s.eng.UnexpectedFrames(); after != engineBefore {
		t.Errorf("Engine.UnexpectedFrames() moved %d -> %d during a BROADCAST flood; the whole premise of the carrier ruling is that it never sees these frames", engineBefore, after)
	}
}

// TestE2E_AddressedFloodCapIsNonfatalThenLaterFailsClosed — R9-SPLIT
// half (b), and BOTH sides of the nonfatal/fail-closed rule with the lever
// that makes each reachable.
//
// A to=E0 frame passes the address filter, becomes an engine event, and
// re-arms the drain's idle timer, so Init's drain reaches its ABSOLUTE cap
// and returns ErrDrainCapExceeded. That INITIAL failure is
// NONFATAL-WITH-DIAGNOSTIC: E1's drain is bounded precisely so it cannot
// fail the open.
//
// EVERY LATER DRAIN FAILURE IS FATAL, and the asymmetry is the point: once
// the session is exchanging frames, a drain that cannot find quiet means
// this program can no longer tell its own answers from somebody else's.
func TestE2E_AddressedFloodCapIsNonfatalThenLaterFailsClosed(t *testing.T) {
	r := newFake(t, fakeic7610.WithAddressedFlood(time.Millisecond))
	r.SetSlot(1, fakeic7610.MemState{Raw: e2eSeed(1).record(t)})
	r.SetSlot(42, fakeic7610.MemState{Raw: e2eSeed(42).record(t)})

	s := openFake(t, r, WithConsentedUnverifiedWrites())
	if !s.OpenDiagnostics().InitDrainCapExceeded {
		t.Error("InitDrainCapExceeded is false under a CONTROLLER-ADDRESSED flood — Init's drain cannot have found its idle gap")
	}

	// The quiet line: a clean read proves the session is usable.
	r.StopFloods()
	time.Sleep(300 * time.Millisecond)
	if _, err := s.ReadChannel(t.Context(), "042"); err != nil {
		t.Fatalf("the clean read on a quiet line failed: %v", err)
	}

	// And now the line jabbers again, for good.
	r.StartAddressedFlood(time.Millisecond)
	t.Cleanup(r.StopFloods)

	// SOMETHING must eventually fail closed. Which exchange meets the
	// drain first depends on when an answer is starved past its timeout,
	// so the assertion is on the OUTCOME rather than on a chosen call:
	// no exchange may quietly succeed by treating a failed drain the way
	// Init's is treated.
	var lastErr error
	for i := 0; i < 12 && lastErr == nil; i++ {
		if _, err := s.ReadChannel(t.Context(), "042"); err != nil {
			lastErr = err
			break
		}
		if _, err := s.WriteChannel(t.Context(), goodChannel("042")); err != nil {
			lastErr = err
			break
		}
	}
	if lastErr == nil {
		t.Skip("the addressed flood did not starve an exchange within twelve attempts on this machine; " +
			"TestOpen_LaterQuarantineFailureFailsClosed pins the fail-closed half deterministically through the scripted port")
	}
	t.Logf("the later failure the caller saw: %v", lastErr)
	if errors.Is(lastErr, transport.ErrDrainCapExceeded) || errors.Is(lastErr, transport.ErrQuarantineFailed) {
		return // fail-closed, as required
	}
	if errors.Is(lastErr, transport.ErrTimeout) {
		return // starved and reported, which is also fail-closed
	}
	t.Errorf("the later failure was %v; a drain that cannot find quiet must reach the caller, never be stepped over the way Init's is", lastErr)
}

// TestE2E_TheDriverNeverMutatesTheRadiosSettings sweeps a representative
// workload and asserts the fake SAW exactly two commands: 19 00 and 1A 00.
//
// NO 1A 05, NO TRANSCEIVE SET, NO CLEAR, NO 0B, NO 18 01. E1's
// InitSequence is EMPTY, which is a safety property rather than an
// omission: transceive is tolerated STRUCTURALLY, by address filtering,
// instead of by writing a transceive-off setting, so opening a session
// touches nothing outside the consent regime.
//
// CommandLog logs REFUSED commands too — refusing is a thing the radio
// did — so a clear frame this driver built and the radio rejected would
// still show up here. That is what makes the absence meaningful.
func TestE2E_TheDriverNeverMutatesTheRadiosSettings(t *testing.T) {
	r := newFake(t)
	for i, slot := range e2eSlots() {
		r.SetSlot(fakeChannel(t, slot), fakeic7610.MemState{Raw: e2eSeed(i).record(t)})
	}
	s := openFake(t, r, WithConsentedUnverifiedWrites())

	for _, slot := range []string{"001", "042", "099", "P1", "P2"} {
		if _, err := s.ReadChannel(t.Context(), slot); err != nil {
			t.Fatalf("ReadChannel %s: %v", slot, err)
		}
	}
	if _, err := s.WriteChannel(t.Context(), goodChannel("042")); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	// Refusals too, so anything they might build would be logged.
	_, _ = s.WriteChannel(t.Context(), codeplug.Channel{Slot: "001", Data: nil})
	_, _ = s.WriteChannel(t.Context(), codeplug.Channel{Slot: "042", Data: &codeplug.ChannelData{}})
	bad := goodChannel("042")
	bad.Data.FreqHz = 70_000_000
	_, _ = s.WriteChannel(t.Context(), bad)

	want := map[[2]byte]bool{{0x19, 0x00}: true, {0x1A, 0x00}: true}
	seen := map[[2]byte]int{}
	for _, cs := range r.CommandLog() {
		seen[cs]++
		if !want[cs] {
			t.Errorf("the radio saw command %02x %02x, which this driver must never send", cs[0], cs[1])
		}
	}
	for cs := range want {
		if seen[cs] == 0 {
			t.Errorf("the radio never saw %02x %02x; the workload should have exercised it", cs[0], cs[1])
		}
	}
	// And no clear frame in the raw byte log either — the 0B form carries
	// no sub-command and the 1A 00 clear form is a short data area, so
	// neither is caught by the command sweep alone.
	log := r.BytesWritten()
	if bytes.Contains(log, []byte{0xFE, 0xFE, 0x98, 0xE0, 0x0B, 0xFD}) {
		t.Error("a command 0B \"Memory clear\" frame reached the radio")
	}
	for i := 0; i+10 <= len(log); i++ {
		if bytes.Equal(log[i:i+6], []byte{0xFE, 0xFE, 0x98, 0xE0, 0x1A, 0x00}) && log[i+8] == 0xFF && log[i+9] == 0xFD {
			t.Error("a 1A 00 <ch> FF clear frame reached the radio")
		}
	}
}

// transcriptionPath is the B leg's committed transcription, reached from
// this package's own directory (go test's working directory).
//
// IT IS FROZEN EVIDENCE. core/civ/ic7610's TestEvidenceFrozen holds its
// SHA-256, so a change to it is a deliberate, reviewable act — and this
// test is what makes that act reach the two RECORD-LENGTH CONSTANTS
// instead of leaving them behind.
const transcriptionPath = "../../civ/ic7610/testdata/ic7610-transcription-b.csv"

// recordLengthFromTranscription re-derives this radio's three length
// figures FROM THE ARTEFACT, by the arithmetic both packages claim to
// have done:
//
//	dataArea = the sum of every D1 row's width_bytes          (27)
//	selector = the width of the ①, ② channel-selector row     (2)
//	record   = dataArea - selector                            (25)
//
// READ, NOT TRANSCRIBED. A Go literal holding 2, 1, 5, 2, 1, 3, 3, 10
// would be a SECOND COPY of the CSV, and a correction to the CSV would
// leave the copy and the constants agreeing with each other and with
// nothing else. Reading the cells means a corrected artefact turns this
// test red, which is the only way the correction reaches the code.
func recordLengthFromTranscription(t *testing.T) (record, dataArea, selector int) {
	t.Helper()
	f, err := os.Open(transcriptionPath)
	if err != nil {
		t.Fatalf("opening the frozen transcription: %v", err)
	}
	defer func() { _ = f.Close() }()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("parsing %s: %v", transcriptionPath, err)
	}
	if len(rows) == 0 {
		t.Fatalf("%s is empty", transcriptionPath)
	}
	col := map[string]int{}
	for i, name := range rows[0] {
		col[name] = i
	}
	for _, want := range []string{"diagram_id", "field_index", "label_verbatim", "width_bytes"} {
		if _, ok := col[want]; !ok {
			t.Fatalf("%s has no %q column; its header is %v", transcriptionPath, want, rows[0])
		}
	}

	var d1 int
	for _, row := range rows[1:] {
		if row[col["diagram_id"]] != "D1" {
			continue
		}
		d1++
		w, err := strconv.Atoi(strings.TrimSpace(row[col["width_bytes"]]))
		if err != nil {
			t.Fatalf("D1 row %q has an unreadable width_bytes %q: %v",
				row[col["field_index"]], row[col["width_bytes"]], err)
		}
		dataArea += w
		// The selector row is identified by BOTH its printed index and
		// its label, so a renumbered or relabelled artefact fails loudly
		// here rather than silently subtracting the wrong field.
		if row[col["field_index"]] == "①, ②" && row[col["label_verbatim"]] == "Memory channel numbers" {
			if selector != 0 {
				t.Fatalf("the transcription carries more than one channel-selector row")
			}
			selector = w
		}
	}
	// Eight D1 rows is the band the page draws; a ninth or a seventh
	// means the transcription has been re-cut and every figure below it
	// needs re-reading rather than re-summing.
	if d1 != 8 {
		t.Fatalf("the transcription has %d D1 rows, want 8 — the memory-content band was re-cut and this derivation no longer describes it", d1)
	}
	if selector == 0 {
		t.Fatalf("the transcription has no D1 row for the ①, ② channel selector; the derivation has nothing to subtract")
	}
	return dataArea - selector, dataArea, selector
}
