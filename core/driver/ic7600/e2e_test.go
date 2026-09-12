// SPDX-License-Identifier: GPL-3.0-or-later

package ic7600

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic7600 "github.com/gm5dna/open-rig-programmer/core/civ/ic7600"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// THIS FILE USES THE IN-PACKAGE scriptedPort (scriptedport_test.go), NOT
// internal/fakeic7600 - the brief for this package OWNS only
// core/civ/ic7600 and core/driver/ic7600, and a stateful fake radio
// package is neither.
//
// scriptedPort ANSWERS PER FRAME FROM A TABLE (radioImage), built once at
// construction; it does not model persisted radio STATE the way a fake
// would. WHERE THE ORIGINAL IC-7610 e2e SUITE ASSERTED "the fake's stored
// bytes equal X" after a write, this file instead asserts "the SET FRAME
// this driver put on the wire equals X" via port.Transcript() - a direct
// wire-level check that needs no persisted state to compare against, and
// arguably a MORE direct independence check: the test's own record
// builder and the driver's own encoder are two separate readings of the
// same page, meeting on the wire rather than in a third party's memory.
//
// EVERY BYTE POSITION BELOW COMES FROM PDF p.178's RECORD, restated here
// independently of civ.RecordLayout's own tables (which this file's own
// tests exercise).

// The ten printed mode codes and the three filter codes, in the wire form
// this file seeds records with - NOT taken from the codec under test.
// RULING OQ1: PSK/PSK-R are the hex bytes 0x12/0x13.
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

// e2eFields is one seeded channel's content in NEUTRAL terms.
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
// LITTLE-endian packed BCD (PDF p.175's five-cell strip, least
// significant pair first).
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

// record assembles the 25-byte record for f, BY OFFSET, from
// core/civ/ic7600's own table.
func (f e2eFields) record(t *testing.T) []byte {
	t.Helper()
	rec := make([]byte, civic7600.RecordOnlyLength)
	// offset 0: UNMAPPED (E6, whole byte). Zero, matching the Fixed template.
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
		toneTx:   uint64(670 + i*7),
		toneRx:   uint64(885 + i*11),
		name:     fmt.Sprintf("CH%03d TEST", i),
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

// slotChannel maps a canonical slot to its wire channel number: 1..99 for
// the memories, 100/101 for P1/P2.
func slotChannel(t *testing.T, slot string) int {
	t.Helper()
	a, _, err := slotToAddress(slot)
	if err != nil {
		t.Fatalf("slotToAddress(%q): %v", slot, err)
	}
	return a.Channel
}

// openScripted opens a driver session against p, registering cleanup.
func openScripted(t *testing.T, p *scriptedPort, opts ...Option) *Session {
	t.Helper()
	sess, err := New(Simulated, opts...).Open(t.Context(), p.Port(), driver.Identity{Port: "/dev/scripted"})
	if err != nil {
		t.Fatalf("Open against the scripted port: %v", err)
	}
	s, ok := sess.(*Session)
	if !ok {
		t.Fatalf("Open returned %T, want *Session", sess)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestE2E_ProbeFingerprints is the independence check landing: this
// package's own record-length constant is re-derived from the frozen
// transcription artefact by a SEPARATE arithmetic route, and the driver's
// probe measures whatever the scripted radio answers with.
//
// THE ID TOKEN IS RECORDED, NEVER MATCHED (D5 entry 7, matrix lift R7).
func TestE2E_ProbeFingerprints(t *testing.T) {
	t.Run("the record length is re-derived from the frozen artefact", func(t *testing.T) {
		record, dataArea, selector := recordLengthFromTranscription(t)
		if civic7600.RecordOnlyLength != record {
			t.Errorf("STOP - this driver's RecordOnlyLength is %d and the transcription's D1 widths derive %d. "+
				"A disagreement is arbitration against PDF p.178, never a constant moved to match",
				civic7600.RecordOnlyLength, record)
		}
		if civic7600.DataAreaLength != dataArea {
			t.Errorf("DataAreaLength = %d, and the D1 widths sum to %d", civic7600.DataAreaLength, dataArea)
		}
		if civic7600.AddressBytes != selector {
			t.Errorf("AddressBytes = %d, and the transcription's selector row is %d bytes wide", civic7600.AddressBytes, selector)
		}
		if record != dataArea-selector {
			t.Errorf("the derivation is incoherent: %d != %d - %d", record, dataArea, selector)
		}
	})

	for _, tt := range []struct {
		name  string
		token []byte
		want  string
	}{
		{"a deliberately implausible token", []byte{0xA5}, "7aa5"},
		{"a supplied single-byte token", []byte{0x7A}, "7a7a"},
		{"a supplied two-byte token", []byte{0x00, 0x7c}, "7a007c"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			seed := e2eSeed(1)
			img := radioImage{idToken: tt.token, records: map[int][]byte{1: seed.record(t)}}
			p := newScriptedPort(t, img)

			s := openScripted(t, p)
			if got := s.Identity().CATID; got != tt.want {
				t.Errorf("Identity().CATID = %q, want %q", got, tt.want)
			}
			length, confirmed := s.Fingerprint()
			if !confirmed || length != 25 {
				t.Errorf("Fingerprint() = (%d, %v), want (25, true)", length, confirmed)
			}
		})
	}
}

// TestE2E_EmptyRadioOpensUnfingerprinted: a scripted radio that answers
// every 1A 00 read FA (no records seeded) has no record to measure. The
// session opens ON ADDRESS EVIDENCE ALONE (spec D3.2, D5 entry 2(a),
// matrix lift R2a).
func TestE2E_EmptyRadioOpensUnfingerprinted(t *testing.T) {
	p := newScriptedPort(t, radioImage{idToken: []byte{0xA5}})
	s := openScripted(t, p)
	if length, confirmed := s.Fingerprint(); confirmed || length != 0 {
		t.Errorf("Fingerprint() = (%d, %v), want (0, false)", length, confirmed)
	}
	rep := s.OpenDiagnostics()
	if rep.Fingerprinted || rep.SlotsTried != probeSlotCount {
		t.Errorf("OpenDiagnostics() = %+v, want UNFINGERPRINTED with the whole bounded search run", rep)
	}

	for _, slot := range e2eSlots() {
		ch, err := s.ReadChannel(t.Context(), slot)
		if err != nil {
			t.Fatalf("ReadChannel %s on an empty radio: %v - an unset slot must come back empty, not as an error", slot, err)
		}
		if !ch.Empty() {
			t.Errorf("ReadChannel %s returned %+v, want an EMPTY channel", slot, ch.Data)
		}
		if ch.Slot != slot {
			t.Errorf("ReadChannel %s carried slot %q", slot, ch.Slot)
		}
	}
}

// checkReadAll seeds every declared slot, reads each back through the
// driver, and compares against what the test put in. Shared by the plain
// run and the USB-echo run.
func checkReadAll(t *testing.T, echo bool) {
	t.Helper()
	slots := e2eSlots()
	if len(slots) != 101 {
		t.Fatalf("this radio declares %d slots, want 101 (99 memories + 2 scan edges)", len(slots))
	}
	want := make(map[string]e2eFields, len(slots))
	records := map[int][]byte{}
	for i, slot := range slots {
		f := e2eSeed(i)
		want[slot] = f
		records[slotChannel(t, slot)] = f.record(t)
	}

	p := newScriptedPort(t, radioImage{idToken: []byte{0xA5}, records: records})
	port := p.Port()
	if echo {
		port = &echoingPort{Port: port}
	}
	sess, err := New(Simulated).Open(t.Context(), port, driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s := sess.(*Session)
	t.Cleanup(func() { _ = s.Close() })

	for _, slot := range slots {
		ch, err := s.ReadChannel(t.Context(), slot)
		if err != nil {
			t.Fatalf("ReadChannel %s: %v", slot, err)
		}
		if ch.Empty() {
			t.Fatalf("ReadChannel %s came back empty; the slot was seeded", slot)
		}
		f, d := want[slot], ch.Data

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
		if d.ScanSkip.State != codeplug.Unavailable || d.DataMode.State != codeplug.Unavailable {
			t.Errorf("%s ScanSkip/DataMode = %+v/%+v, want Unavailable - an unmapped region is never decoded", slot, d.ScanSkip, d.DataMode)
		}
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

// TestE2E_ReadAll rounds every declared slot through the scripted port and
// back.
func TestE2E_ReadAll(t *testing.T) { checkReadAll(t, false) }

// checkWriteOne writes one channel with consent and asserts the SET FRAME
// on the wire, via port.Transcript() - see the file header for why this
// replaces "the fake's stored bytes."
func checkWriteOne(t *testing.T, echo bool) {
	t.Helper()
	prior := e2eSeed(3)
	records := map[int][]byte{
		1:  e2eSeed(1).record(t),
		42: prior.record(t),
	}
	p := newScriptedPort(t, radioImage{idToken: []byte{0xA5}, records: records, ackSets: true})
	port := p.Port()
	if echo {
		port = &echoingPort{Port: port}
	}
	sess, err := New(Simulated, WithConsentedUnverifiedWrites()).Open(t.Context(), port, driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s := sess.(*Session)
	t.Cleanup(func() { _ = s.Close() })

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
	if len(res.Steps) != 1 || !res.Steps[0].Sent || !res.Steps[0].Confirmed {
		t.Errorf("Steps = %+v, want one sent, confirmed step", res.Steps)
	}

	// AND THE SET FRAME ON THE WIRE CARRIES THE BYTES THE TEST MEANT.
	set := lastSetFrame(t, p.Transcript())
	wantRecord := want.record(t)
	if !bytes.Equal(set, wantRecord) {
		t.Errorf("the set frame carried\n  % x\nwant\n  % x", set, wantRecord)
	}
	if len(set) != civic7600.RecordOnlyLength {
		t.Errorf("the set frame carried %d record bytes, want the full %d - register entry ic7600-full-record-mandatory, matrix lift R15", len(set), civic7600.RecordOnlyLength)
	}
}

// TestE2E_WriteOne is the write half of the round trip.
func TestE2E_WriteOne(t *testing.T) { checkWriteOne(t, false) }

// lastSetFrame returns the record bytes of the LAST 1A 00 SET frame in
// frames, or fails the test if there is none.
func lastSetFrame(t *testing.T, frames [][]byte) []byte {
	t.Helper()
	for i := len(frames) - 1; i >= 0; i-- {
		f := frames[i]
		if len(f) >= memSetFrameLen && f[4] == civ.CmdMemory && f[5] == civ.SubMemoryContents {
			return f[8 : len(f)-1]
		}
	}
	t.Fatalf("no 1A 00 set frame found in %s", hexFrames(frames))
	return nil
}

// TestE2E_WriteIsRefusedForASelectGroupChannel is E6's COST, proven end to
// end against an independently constructed radio image.
func TestE2E_WriteIsRefusedForASelectGroupChannel(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func([]byte)
		offset int
		nibble string
	}{
		{"SELECT star2 in byte 0 (whole byte)", func(b []byte) { b[0] = 0x02 }, civic7600.SelectByteOffset, "whole"},
		{"SELECT star3 in byte 0 (whole byte)", func(b []byte) { b[0] = 0x03 }, civic7600.SelectByteOffset, "whole"},
		{"DATA 2 in byte 8's high nibble", func(b []byte) { b[8] = 0x20 | (b[8] & 0x0F) }, civic7600.DataModeNibbleOffset, "high"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			prior := e2eSeed(5).record(t)
			tt.mutate(prior)
			records := map[int][]byte{
				1:  e2eSeed(1).record(t),
				42: append([]byte(nil), prior...),
			}
			p := newScriptedPort(t, radioImage{idToken: []byte{0xA5}, records: records, ackSets: true})
			s := openScripted(t, p, WithConsentedUnverifiedWrites())
			before := len(p.Transcript())

			res, err := s.WriteChannel(t.Context(), goodChannel("042"))
			var e *UnmappedRegionError
			if !errors.As(err, &e) {
				t.Errorf("err = %v, want an *UnmappedRegionError", err)
			} else if e.Offset != tt.offset || e.Nibble != tt.nibble {
				t.Errorf("*UnmappedRegionError = %+v, want offset %d, nibble %q", e, tt.offset, tt.nibble)
			}
			if len(res.Steps) != 0 {
				t.Errorf("Steps = %+v, want empty - no frame was ever built", res.Steps)
			}

			// Exactly one read's worth of new traffic - tier ruling T5's
			// single recorded exception - and no set frame.
			after := p.Transcript()
			if grew := len(after) - before; grew != 1 {
				t.Errorf("the refused write put %d new frames on the wire, want 1 (one 1A 00 read)", grew)
			}
			for _, f := range after[before:] {
				if len(f) >= memSetFrameLen && f[4] == civ.CmdMemory && f[5] == civ.SubMemoryContents {
					t.Errorf("a 1A 00 SET frame reached the wire on a REFUSED write: % X", f)
				}
			}
		})
	}
}

// TestE2E_WrongRecordLengthIsRefused: a scripted radio answering 39-byte
// records is refused by this driver's CONTINUOUS length fingerprint.
//
// THE REFUSAL NAMES NO FOUND MODEL. The IC-7600 has no registered sibling
// (matrix S4).
func TestE2E_WrongRecordLengthIsRefused(t *testing.T) {
	p := newScriptedPort(t, radioImage{idToken: []byte{0xA5}, records: map[int][]byte{1: make([]byte, 39)}})

	sess, err := New(Simulated).Open(t.Context(), p.Port(), driver.Identity{})
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
	if mismatch.Got != 39 || mismatch.Want != civic7600.RecordOnlyLength {
		t.Errorf("*RecordLengthMismatchError = %+v, want {Got: 39, Want: 25}", mismatch)
	}
}

// TestE2E_EraseIsRefused - ChannelData HAS NO Erase MEMBER; erase is
// represented solely by Channel.Data == nil.
func TestE2E_EraseIsRefused(t *testing.T) {
	for _, opts := range [][]Option{nil, {WithConsentedUnverifiedWrites()}} {
		seeded := e2eSeed(1).record(t)
		p := newScriptedPort(t, radioImage{idToken: []byte{0xA5}, records: map[int][]byte{1: append([]byte(nil), seeded...)}, ackSets: true})
		s := openScripted(t, p, opts...)
		before := len(p.Transcript())

		res, err := s.WriteChannel(t.Context(), codeplug.Channel{Slot: "001", Data: nil})
		if !errors.Is(err, driver.ErrWriteRefused) {
			t.Fatalf("err = %v, want ErrWriteRefused (spec D4 \"Erase\"; consent structurally never reaches the field)", err)
		}
		var refusal *driver.WriteRefusedError
		if !errors.As(err, &refusal) || !containsField(refusal.Fields, spec.FieldErase) {
			t.Errorf("err = %v, want a *driver.WriteRefusedError naming erase", err)
		}
		if len(res.Steps) != 0 {
			t.Errorf("Steps = %+v, want empty", res.Steps)
		}
		if after := len(p.Transcript()); after != before {
			t.Errorf("the refused erase put %d new frames on the wire, want none", after-before)
		}
	}
}

// misaddressingPort sits BETWEEN the scripted radio and the driver and
// rewrites the two channel-selector bytes of every 1A 00 ANSWER it
// carries, so the driver is offered a well-formed record about a channel
// it did not ask about (tier ruling T2's mandatory per-driver mismatch
// regression test).
type misaddressingPort struct {
	transport.Port
	mu      sync.Mutex
	pending []byte
	to      byte
	lo      byte
	active  bool
}

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
	n, err := p.Port.Read(buf)
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

func TestE2E_AnswerForAnotherChannelIsRefused(t *testing.T) {
	records := map[int][]byte{}
	for i, slot := range []string{"001", "005", "006"} {
		records[slotChannel(t, slot)] = e2eSeed(i + 1).record(t)
	}
	p := newScriptedPort(t, radioImage{idToken: []byte{0xA5}, records: records})
	port := &misaddressingPort{Port: p.Port()}

	sess, err := New(Simulated).Open(t.Context(), port, driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	s := sess.(*Session)
	defer func() { _ = s.Close() }()

	if _, err := s.ReadChannel(t.Context(), "005"); err != nil {
		t.Fatalf("the honest read of 005 failed: %v", err)
	}
	before := s.AnswerMismatches()

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
	if mismatch.Requested.Channel != 5 || mismatch.Answered.Channel != 6 {
		t.Errorf("*AnswerMismatchError = {Requested: %s, Answered: %s}, want {ch5, ch6}", mismatch.Requested, mismatch.Answered)
	}
	if got := s.AnswerMismatches(); got != before+1 {
		t.Errorf("the mismatch diagnostic went %d -> %d, want one increment", before, got)
	}
}

// echoingPort wraps a scripted port's host end so every frame the DRIVER
// writes is queued back as the next thing it reads, ahead of the genuine
// reply - modelling a CI-V link that echoes every transmitted frame. No
// scriptedport_test.go radioImage lever does this (its echo-adjacent
// fields are about the RADIO's own answers, not a wire-level echo), so
// this file supplies the wrapper directly.
type echoingPort struct {
	transport.Port
	mu      sync.Mutex
	pending []byte
}

func (p *echoingPort) Write(b []byte) (int, error) {
	n, err := p.Port.Write(b)
	if err == nil && n > 0 {
		p.mu.Lock()
		p.pending = append(p.pending, b[:n]...)
		p.mu.Unlock()
	}
	return n, err
}

func (p *echoingPort) Read(b []byte) (int, error) {
	p.mu.Lock()
	if len(p.pending) > 0 {
		n := copy(b, p.pending)
		p.pending = p.pending[n:]
		p.mu.Unlock()
		return n, nil
	}
	p.mu.Unlock()
	return p.Port.Read(b)
}

// TestE2E_EchoOnChangesNothing proves the structural echo handling end to
// end (spec D3.4): the read-all and write-one suites run identically with
// the radio's own outgoing frames echoed back to the driver verbatim.
//
// NO ECHO PROBING ANYWHERE: this driver never asks whether echo is on and
// never counts frames to find out; civ's accumulator drops a frame that
// BYTE-EQUALS one NoteSent recorded.
func TestE2E_EchoOnChangesNothing(t *testing.T) {
	t.Run("read-all with echo", func(t *testing.T) { checkReadAll(t, true) })
	t.Run("write-one with echo", func(t *testing.T) { checkWriteOne(t, true) })

	t.Run("the echoes are counted and dropped, not answered", func(t *testing.T) {
		p := newScriptedPort(t, radioImage{idToken: []byte{0xA5}, records: map[int][]byte{1: e2eSeed(1).record(t)}})
		port := &echoingPort{Port: p.Port()}
		sess, err := New(Simulated).Open(t.Context(), port, driver.Identity{})
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		s := sess.(*Session)
		defer func() { _ = s.Close() }()
		if _, err := s.ReadChannel(t.Context(), "001"); err != nil {
			t.Fatalf("ReadChannel: %v", err)
		}
		if got := s.WireStats().Echoes; got == 0 {
			t.Error("WireStats().Echoes is zero with the radio echoing every frame - the accumulator's byte-identity suppression is what makes echo a non-event, and it should be visible in the counters")
		}
	})
}

// TestE2E_BroadcastFloodNeverReachesTheEngine - R9-SPLIT half (a). civ's
// accumulator counts a to=00 frame and NEVER RETURNS it, so it never
// becomes an engine event and Engine.Init SUCCEEDS.
func TestE2E_BroadcastFloodNeverReachesTheEngine(t *testing.T) {
	records := map[int][]byte{}
	for i, slot := range e2eSlots() {
		records[slotChannel(t, slot)] = e2eSeed(i).record(t)
	}
	p := newScriptedPort(t, radioImage{idToken: []byte{0xA5}, records: records, ackSets: true})
	p.startFlood(0x00)

	s := openScripted(t, p, WithConsentedUnverifiedWrites())
	if s.OpenDiagnostics().InitDrainCapExceeded {
		t.Error("InitDrainCapExceeded is true under a BROADCAST flood - those frames never reach the engine")
	}
	if _, confirmed := s.Fingerprint(); !confirmed {
		t.Error("the probe did not fingerprint through a broadcast flood")
	}

	engineBefore := s.eng.UnexpectedFrames()
	wireBefore := s.WireStats().Unexpected
	if wireBefore == 0 {
		t.Fatal("WireStats().Unexpected is zero under a broadcast flood - the adapter's counter is the ONLY place this traffic is visible")
	}

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
		t.Errorf("Engine.UnexpectedFrames() moved %d -> %d during a BROADCAST flood", engineBefore, after)
	}
}

// TestE2E_AddressedFloodCapIsNonfatalThenLaterFailsClosed - R9-SPLIT
// half (b). A to=E0 frame passes the address filter, becomes an engine
// event, and re-arms the drain's idle timer, so Init's drain reaches its
// ABSOLUTE cap and returns ErrDrainCapExceeded - NONFATAL-WITH-DIAGNOSTIC.
// Every LATER drain failure is FATAL.
func TestE2E_AddressedFloodCapIsNonfatalThenLaterFailsClosed(t *testing.T) {
	records := map[int][]byte{
		1:  e2eSeed(1).record(t),
		42: e2eSeed(42).record(t),
	}
	p := newScriptedPort(t, radioImage{idToken: []byte{0xA5}, records: records, ackSets: true})
	p.startFlood(byte(civ.ControllerAddressDefault))

	s := openScripted(t, p, WithConsentedUnverifiedWrites())
	if !s.OpenDiagnostics().InitDrainCapExceeded {
		t.Error("InitDrainCapExceeded is false under a CONTROLLER-ADDRESSED flood - Init's drain cannot have found its idle gap")
	}

	p.stopFlood()
	time.Sleep(300 * time.Millisecond)
	if _, err := s.ReadChannel(t.Context(), "042"); err != nil {
		t.Fatalf("the clean read on a quiet line failed: %v", err)
	}

	p.restartFlood(byte(civ.ControllerAddressDefault))

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
		t.Skip("the addressed flood did not starve an exchange within twelve attempts on this machine")
	}
	t.Logf("the later failure the caller saw: %v", lastErr)
	if errors.Is(lastErr, transport.ErrDrainCapExceeded) || errors.Is(lastErr, transport.ErrQuarantineFailed) {
		return
	}
	if errors.Is(lastErr, transport.ErrTimeout) {
		return
	}
	t.Errorf("the later failure was %v; a drain that cannot find quiet must reach the caller", lastErr)
}

// TestE2E_TheDriverNeverMutatesTheRadiosSettings sweeps a representative
// workload and asserts the scripted radio SAW exactly two commands:
// 19 00 and 1A 00. E1's InitSequence is EMPTY.
func TestE2E_TheDriverNeverMutatesTheRadiosSettings(t *testing.T) {
	records := map[int][]byte{}
	for i, slot := range e2eSlots() {
		records[slotChannel(t, slot)] = e2eSeed(i).record(t)
	}
	p := newScriptedPort(t, radioImage{idToken: []byte{0xA5}, records: records, ackSets: true})
	s := openScripted(t, p, WithConsentedUnverifiedWrites())

	for _, slot := range []string{"001", "042", "099", "P1", "P2"} {
		if _, err := s.ReadChannel(t.Context(), slot); err != nil {
			t.Fatalf("ReadChannel %s: %v", slot, err)
		}
	}
	if _, err := s.WriteChannel(t.Context(), goodChannel("042")); err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	_, _ = s.WriteChannel(t.Context(), codeplug.Channel{Slot: "001", Data: nil})
	_, _ = s.WriteChannel(t.Context(), codeplug.Channel{Slot: "042", Data: &codeplug.ChannelData{}})
	bad := goodChannel("042")
	bad.Data.FreqHz = 70_000_000
	_, _ = s.WriteChannel(t.Context(), bad)

	want := map[[2]byte]bool{{0x19, 0x00}: true, {0x1A, 0x00}: true}
	seen := map[[2]byte]int{}
	for _, f := range p.Transcript() {
		if len(f) < 6 {
			continue
		}
		cs := [2]byte{f[4], f[5]}
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
	for _, f := range p.Transcript() {
		if bytes.Equal(f, []byte{0xFE, 0xFE, 0x7A, 0xE0, 0x0B, 0xFD}) {
			t.Error("a command 0B \"Memory clear\" frame reached the radio")
		}
		if len(f) == 10 && f[4] == 0x1A && f[5] == 0x00 && f[8] == 0xFF && f[9] == 0xFD {
			t.Error("a 1A 00 <ch> FF clear frame reached the radio")
		}
	}
}

// transcriptionPath is this radio's own committed transcription, reached
// from this package's own directory (go test's working directory). IT IS
// FROZEN EVIDENCE - core/civ/ic7600's TestEvidenceFrozen holds its SHA-256.
const transcriptionPath = "../../civ/ic7600/testdata/ic7600-transcription-b.csv"

// recordLengthFromTranscription re-derives this radio's three length
// figures FROM THE ARTEFACT:
//
//	dataArea = the sum of every D1 row's width_bytes          (27)
//	selector = the width of the "1,2" channel-selector row    (2)
//	record   = dataArea - selector                            (25)
//
// READ, NOT TRANSCRIBED, for the same reason the IC-7610 exemplar's own
// version of this function gives.
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
		if row[col["field_index"]] == "1,2" {
			if selector != 0 {
				t.Fatalf("the transcription carries more than one channel-selector row")
			}
			selector = w
		}
	}
	if d1 != 9 {
		t.Fatalf("the transcription has %d D1 rows, want 9 - the memory-content band was re-cut and this derivation no longer describes it", d1)
	}
	if selector == 0 {
		t.Fatalf("the transcription has no D1 row for the channel selector; the derivation has nothing to subtract")
	}
	return dataArea - selector, dataArea, selector
}
