// SPDX-License-Identifier: GPL-3.0-or-later

package ft1000mp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/bincat"
	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// buildDump returns a synthetic full U=00H dump: dumpHeaderLen zero
// bytes, then dumpRecordCount 16-byte records, every one masked
// (empty) by default, overridden per 0-based record index by set.
func buildDump(set map[int][]byte) []byte {
	dump := make([]byte, dumpHeaderLen+dumpRecordCount*recordLen)
	for i := 0; i < dumpRecordCount; i++ {
		start := dumpHeaderLen + i*recordLen
		rec, ok := set[i]
		if !ok {
			dump[start] = flagMemMask // masked/empty
			continue
		}
		copy(dump[start:start+recordLen], rec)
	}
	return dump
}

// simpleRecord builds one 16-byte record: unmasked, the manual's own
// 14.250.00 MHz worked example, mode CW, shift/clarifier per args.
func simpleRecord(shiftFlag byte) []byte {
	raw := make([]byte, recordLen)
	raw[offFreq+0], raw[offFreq+1], raw[offFreq+2], raw[offFreq+3] = 0x00, 0x05, 0x24, 0x10
	raw[offMode] = modeCW << 5 // family index 2 == modeCW's own code, both 2
	raw[offFlags] = shiftFlag
	return raw
}

func identityReply(match bool) []byte {
	if match {
		return []byte{0, 0, 0, 0x03, 0x93}
	}
	return []byte{0, 0, 0, 0x11, 0x22}
}

// newTestSession opens a Session against a scripted port whose FAH probe
// matches and whose full dump is dump — the shared setup every
// ReadChannel/WriteChannel test below starts from.
func newTestSession(t *testing.T, dump []byte, failOn func([]byte) bool) (*Session, *scriptedPort) {
	t.Helper()
	port := newScriptedPort(func(frame []byte) []byte {
		opcode, args, err := bincat.ParseFrame(frame)
		if err != nil {
			return nil
		}
		switch opcode {
		case bincat.OpIdentity:
			return identityReply(true)
		case bincat.OpStatusUpdate:
			if args[0] == bincat.UFullDump {
				return dump
			}
		}
		return nil // every write opcode: fire-and-forget, no reply
	})
	port.failOn = failOn

	d := New(RealHardware, WithConsentedUnverifiedWrites())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sess, err := d.Open(ctx, port, driver.Identity{Port: "test"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return sess.(*Session), port
}

func TestOpen_IdentityMatch(t *testing.T) {
	sess, _ := newTestSession(t, buildDump(nil), nil)
	defer sess.Close()
	if sess.Identity().CATID != catID {
		t.Errorf("CATID = %q, want %q", sess.Identity().CATID, catID)
	}
}

func TestOpen_IdentityMismatchIsWrongRadio(t *testing.T) {
	port := newScriptedPort(func(frame []byte) []byte { return identityReply(false) })
	d := New(RealHardware)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := d.Open(ctx, port, driver.Identity{})
	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Fatalf("Open error = %v, want errors.Is(_, driver.ErrWrongRadio)", err)
	}
}

func TestOpen_IdentitySilenceIsWrongRadio(t *testing.T) {
	port := newScriptedPort(func(frame []byte) []byte { return nil })
	d := New(RealHardware)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := d.Open(ctx, port, driver.Identity{})
	if !errors.Is(err, driver.ErrWrongRadio) {
		t.Fatalf("Open error = %v, want errors.Is(_, driver.ErrWrongRadio)", err)
	}
}

func TestReadChannel_Populated(t *testing.T) {
	dump := buildDump(map[int][]byte{3: simpleRecord(0)}) // channel "1" -> record index 3
	sess, _ := newTestSession(t, dump, nil)
	defer sess.Close()

	ch, err := sess.ReadChannel(context.Background(), "1")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if ch.Empty() {
		t.Fatal("ReadChannel returned an empty channel for a populated record")
	}
	if ch.Data.FreqHz != 14_250_000 {
		t.Errorf("FreqHz = %d, want 14250000", ch.Data.FreqHz)
	}
	if ch.Data.Mode != "CW" {
		t.Errorf("Mode = %q, want CW", ch.Data.Mode)
	}
	if ch.Data.Shift != "SIMPLEX" {
		t.Errorf("Shift = %q, want SIMPLEX", ch.Data.Shift)
	}
}

func TestReadChannel_MaskedIsEmpty(t *testing.T) {
	sess, _ := newTestSession(t, buildDump(nil), nil) // channel "2" left masked
	defer sess.Close()

	ch, err := sess.ReadChannel(context.Background(), "2")
	if err != nil {
		t.Fatalf("ReadChannel: %v", err)
	}
	if !ch.Empty() {
		t.Fatal("ReadChannel returned a populated channel for a masked record")
	}
}

func TestReadChannel_CachesTheDump(t *testing.T) {
	dumpReads := 0
	port := newScriptedPort(func(frame []byte) []byte {
		opcode, args, _ := bincat.ParseFrame(frame)
		switch opcode {
		case bincat.OpIdentity:
			return identityReply(true)
		case bincat.OpStatusUpdate:
			if args[0] == bincat.UFullDump {
				dumpReads++
				return buildDump(nil)
			}
		}
		return nil
	})
	d := New(RealHardware)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := d.Open(ctx, port, driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	sess := s.(*Session)
	defer sess.Close()

	if _, err := sess.ReadChannel(context.Background(), "1"); err != nil {
		t.Fatalf("ReadChannel #1: %v", err)
	}
	if _, err := sess.ReadChannel(context.Background(), "2"); err != nil {
		t.Fatalf("ReadChannel #2: %v", err)
	}
	if dumpReads != 1 {
		t.Fatalf("full dump fetched %d times, want 1 (cached)", dumpReads)
	}
}

func writableChannel(slot string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz: 14_250_000,
			Mode:   "CW",
			Shift:  "SIMPLEX",
		},
	}
}

// TestWriteChannel_RefusedWithoutConsent pins the capability-gate defence
// in depth: a RealHardware session built WITHOUT
// WithConsentedUnverifiedWrites carries Write:Unverified (CanWrite()
// false), so WriteChannel must refuse before any wire traffic.
func TestWriteChannel_RefusedWithoutConsent(t *testing.T) {
	port := newScriptedPort(func(frame []byte) []byte {
		opcode, args, _ := bincat.ParseFrame(frame)
		if opcode == bincat.OpIdentity {
			return identityReply(true)
		}
		if opcode == bincat.OpStatusUpdate && args[0] == bincat.UFullDump {
			return buildDump(nil)
		}
		return nil
	})
	d := New(RealHardware) // no consent
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s, err := d.Open(ctx, port, driver.Identity{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	sess := s.(*Session)
	defer sess.Close()

	before := len(port.Writes()) // Open's own FAH probe already wrote one frame
	_, err = sess.WriteChannel(context.Background(), writableChannel("1"))
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel error = %v, want errors.Is(_, driver.ErrWriteRefused)", err)
	}
	if len(port.Writes()) != before {
		t.Fatalf("WriteChannel sent %d new frames before refusing, want 0", len(port.Writes())-before)
	}
}

func TestWriteChannel_Success(t *testing.T) {
	sess, port := newTestSession(t, buildDump(nil), nil)
	defer sess.Close()

	result, err := sess.WriteChannel(context.Background(), writableChannel("1"))
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	wantCommands := []string{"A/B", "SetFreq", "SetMode", "Clarifier", "Shift", "Store"}
	if len(result.Steps) != len(wantCommands) {
		t.Fatalf("Steps = %v, want %d steps for %v", result.Steps, len(wantCommands), wantCommands)
	}
	for i, step := range result.Steps {
		if step.Command != wantCommands[i] {
			t.Errorf("Steps[%d].Command = %q, want %q", i, step.Command, wantCommands[i])
		}
		if !step.Sent || !step.Confirmed {
			t.Errorf("Steps[%d] = %+v, want Sent/Confirmed both true", i, step)
		}
	}
	// Store is this radio's own opcode 0x03 with the channel in the 4th
	// argument byte (matrix §1.8) and K=00H ASSUMED in the 1st.
	last := port.Writes()[len(port.Writes())-1]
	if last[4] != bincat.OpStore || last[3] != 0x01 || last[0] != 0x00 {
		t.Errorf("Store frame = % x, want channel 0x01 in byte 4, 0x00 in byte 1 (K=Enter)", last)
	}
}

// TestWriteChannel_RefusesOutOfDomainFrequency pins the defence-in-depth
// wire-domain guard: a direct WriteChannel caller bypassing codeplug
// validation must still be refused before any frame is sent, not just
// callers that go through the 10 Hz alignment check.
func TestWriteChannel_RefusesOutOfDomainFrequency(t *testing.T) {
	for _, freqHz := range []uint64{99_990, 30_000_010} {
		sess, port := newTestSession(t, buildDump(nil), nil)
		before := len(port.Writes())
		ch := writableChannel("1")
		ch.Data.FreqHz = freqHz
		_, err := sess.WriteChannel(context.Background(), ch)
		if !errors.Is(err, driver.ErrWriteRefused) {
			t.Errorf("FreqHz=%d: WriteChannel error = %v, want errors.Is(_, driver.ErrWriteRefused)", freqHz, err)
		}
		if len(port.Writes()) != before {
			t.Errorf("FreqHz=%d: port.Writes() grew by %d, want no frames sent", freqHz, len(port.Writes())-before)
		}
		sess.Close()
	}
}

func TestWriteChannel_ShiftRequiresKnownOffset(t *testing.T) {
	sess, _ := newTestSession(t, buildDump(nil), nil)
	defer sess.Close()

	ch := writableChannel("1")
	ch.Data.Shift = "MINUS" // OffsetHz left at its zero value: State Absent, not Known
	_, err := sess.WriteChannel(context.Background(), ch)
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel error = %v, want errors.Is(_, driver.ErrWriteRefused)", err)
	}
}

func TestWriteChannel_OffsetSentOnlyWhenShifted(t *testing.T) {
	sess, port := newTestSession(t, buildDump(nil), nil)
	defer sess.Close()

	ch := writableChannel("1")
	ch.Data.Shift = "PLUS"
	ch.Data.OffsetHz = codeplug.FreqField{State: codeplug.Known, Value: 600_000}
	result, err := sess.WriteChannel(context.Background(), ch)
	if err == nil {
		t.Fatalf("WriteChannel: want an error (600 kHz exceeds this driver's 299 kHz encodable range), got success: %+v", result)
	}
	if !errors.Is(err, driver.ErrWriteRefused) {
		t.Fatalf("WriteChannel error = %v, want errors.Is(_, driver.ErrWriteRefused) (out-of-range offset)", err)
	}

	ch.Data.OffsetHz.Value = 100_000
	result, err = sess.WriteChannel(context.Background(), ch)
	if err != nil {
		t.Fatalf("WriteChannel: %v", err)
	}
	var sawOffset bool
	for _, s := range result.Steps {
		if s.Command == "Offset" {
			sawOffset = true
		}
	}
	if !sawOffset {
		t.Fatalf("Steps = %v, want an Offset step for a non-SIMPLEX shift", result.Steps)
	}
	_ = port
}

// TestWriteChannel_StoreFailureIsUnresolved pins the residual-state rule
// (spec.md's Codex #8 correction, this package's ErrStoreUnresolved doc
// comment): a Store/Enter transport failure is reported as this radio's
// own UNRESOLVED outcome, distinct from every earlier step's failure
// (which leaves the memory simply UNCHANGED).
func TestWriteChannel_StoreFailureIsUnresolved(t *testing.T) {
	sess, _ := newTestSession(t, buildDump(nil), func(frame []byte) bool {
		opcode, _, _ := bincat.ParseFrame(frame)
		return opcode == bincat.OpStore
	})
	defer sess.Close()

	result, err := sess.WriteChannel(context.Background(), writableChannel("1"))
	if !errors.Is(err, ErrStoreUnresolved) {
		t.Fatalf("WriteChannel error = %v, want errors.Is(_, ErrStoreUnresolved)", err)
	}
	last := result.Steps[len(result.Steps)-1]
	if last.Command != "Store" || last.Sent {
		t.Errorf("last step = %+v, want Store with Sent=false", last)
	}
}

// TestWriteChannel_EarlyStepFailureLeavesMemoryUnchanged pins the OTHER
// half of the residual-state table: a failure before Store never reaches
// ErrStoreUnresolved.
func TestWriteChannel_EarlyStepFailureLeavesMemoryUnchanged(t *testing.T) {
	sess, _ := newTestSession(t, buildDump(nil), func(frame []byte) bool {
		opcode, _, _ := bincat.ParseFrame(frame)
		return opcode == bincat.OpSetFreq
	})
	defer sess.Close()

	_, err := sess.WriteChannel(context.Background(), writableChannel("1"))
	if err == nil {
		t.Fatal("WriteChannel: want an error")
	}
	if errors.Is(err, ErrStoreUnresolved) {
		t.Fatalf("WriteChannel error = %v, want NOT errors.Is(_, ErrStoreUnresolved) (SetFreq failed, Store never sent)", err)
	}
}

// TestVFOStateRestorer_RoundTrip exercises SnapshotVFOState then
// RestoreVFOState against a dump whose VFO-A record differs from VFO-B's
// and from current-op's, so ActiveVFO's inference is unambiguous.
func TestVFOStateRestorer_RoundTrip(t *testing.T) {
	vfoA := simpleRecord(0)
	dump := buildDump(map[int][]byte{
		dumpCurrentOpRecordIndex: vfoA, // current op == VFO-A -> ActiveVFO "A"
		dumpVFOARecordIndex:      vfoA,
		dumpVFOBRecordIndex:      simpleRecord(flagRptMinus),
	})
	sess, port := newTestSession(t, dump, nil)
	defer sess.Close()

	var vfoRestorer clone.VFOStateRestorer = sess
	snap, err := vfoRestorer.SnapshotVFOState(context.Background())
	if err != nil {
		t.Fatalf("SnapshotVFOState: %v", err)
	}
	if snap.ActiveVFO != "A" {
		t.Fatalf("ActiveVFO = %q, want A", snap.ActiveVFO)
	}
	if snap.Content.FreqHz != 14_250_000 {
		t.Fatalf("Content.FreqHz = %d, want 14250000", snap.Content.FreqHz)
	}

	before := len(port.Writes())
	if err := vfoRestorer.RestoreVFOState(context.Background(), snap); err != nil {
		t.Fatalf("RestoreVFOState: %v", err)
	}
	if len(port.Writes()) <= before {
		t.Fatal("RestoreVFOState sent no frames")
	}
}
