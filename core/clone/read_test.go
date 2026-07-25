// SPDX-License-Identifier: GPL-3.0-or-later

package clone

import (
	"context"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// TestReadAll_DefaultImage: a full ReadAll against fakeradio's default
// (ImageUK) image assembles a Codeplug with correct RadioInfo
// (obligation 1's fresh read) and a BaselineDigest matching
// codeplug.Digest of exactly the channels returned; the progress
// sequence is monotonic, 1-based, and ends at the total slot count.
// Renamed from TestReadAll_UKImage: HW-CONFIRMED 2026-07-13
// (docs/hardware-notes.md §60m regional finding), the default image no
// longer carries a synthetic 60m bank at all, matching Stuart's real UK
// FT-710 (99 MEM + 18 PMS = 117 slots total, no 60m/EMG — exactly the
// live baseline's shape).
func TestReadAll_DefaultImage(t *testing.T) {
	radio, sess := openSimSession(t) // fakeradio.New defaults to ImageUK

	var progressCalls []struct {
		phase string
		done  int
		total int
		slot  string
	}
	svc := NewService(sess, newStore(t), WithNow(func() time.Time { return fixedNow }), WithProgress(func(phase string, done, total int, slot string) {
		progressCalls = append(progressCalls, struct {
			phase string
			done  int
			total int
			slot  string
		}{phase, done, total, slot})
	}))

	cp, err := svc.ReadAll(testCtx(t))
	if err != nil {
		t.Fatalf("ReadAll: unexpected error: %v", err)
	}

	if cp.Schema != codeplug.CurrentSchema {
		t.Errorf("Schema = %d, want %d", cp.Schema, codeplug.CurrentSchema)
	}
	if cp.Generator != generatorID {
		t.Errorf("Generator = %q, want %q", cp.Generator, generatorID)
	}
	if cp.Radio.Model != "FT-710" || cp.Radio.CATID != "0800" {
		t.Errorf("Radio Model/CATID = %q/%q, want \"FT-710\"/\"0800\"", cp.Radio.Model, cp.Radio.CATID)
	}
	if !cp.Radio.ReadAt.Equal(fixedNow) {
		t.Errorf("Radio.ReadAt = %v, want the injected Now %v", cp.Radio.ReadAt, fixedNow)
	}
	if cp.Radio.Port != testIdentity.Port || cp.Radio.USBSerial != testIdentity.USBSerial {
		t.Errorf("Radio Port/USBSerial = %q/%q, want %q/%q", cp.Radio.Port, cp.Radio.USBSerial, testIdentity.Port, testIdentity.USBSerial)
	}
	if cp.Radio.Region != "no-60m" {
		t.Errorf("Radio.Region = %q, want \"no-60m\" (ft710.Session.Region via the regioner accessor; HW-CONFIRMED 2026-07-13 label for a zero-60m/zero-EMG inventory)", cp.Radio.Region)
	}
	wantDigest := codeplug.Digest(cp.Channels)
	if cp.Radio.BaselineDigest != wantDigest {
		t.Errorf("Radio.BaselineDigest = %q, want %q (Digest of the returned Channels)", cp.Radio.BaselineDigest, wantDigest)
	}

	// Slot count: MEM(99) + PMS(18) = 117, no 60M/EMG — matches the live
	// M5a baseline's shape exactly (docs/hardware-notes.md).
	wantSlots := 99 + 18
	if len(cp.Channels) != wantSlots {
		t.Errorf("len(Channels) = %d, want %d", len(cp.Channels), wantSlots)
	}

	// Spot-check M-01 against the factory image (golden vector G4).
	var m01 *codeplug.ChannelData
	for _, ch := range cp.Channels {
		if ch.Slot == "001" {
			m01 = ch.Data
		}
	}
	if m01 == nil {
		t.Fatal("M-01 (\"001\") missing or empty in the returned Channels")
	}
	if m01.FreqHz != 7_000_000 || m01.Mode != "LSB" {
		t.Errorf("M-01 = %+v, want 7.000000 MHz LSB", m01)
	}

	// Progress: monotonic 1..total within phase "read", every call names
	// a real slot, and the final call's done == total == len(Channels).
	if len(progressCalls) != wantSlots {
		t.Fatalf("progress called %d times, want %d (once per slot)", len(progressCalls), wantSlots)
	}
	for i, pc := range progressCalls {
		if pc.phase != "read" {
			t.Errorf("progress[%d].phase = %q, want \"read\"", i, pc.phase)
		}
		if pc.done != i+1 {
			t.Errorf("progress[%d].done = %d, want %d (1-based, monotonic)", i, pc.done, i+1)
		}
		if pc.total != wantSlots {
			t.Errorf("progress[%d].total = %d, want %d", i, pc.total, wantSlots)
		}
		if pc.slot == "" {
			t.Errorf("progress[%d].slot is empty", i)
		}
	}

	_ = radio // kept for future SlotState-based assertions; unused otherwise here.
}

// TestReadAll_FreshEveryCall: obligation 1's "fresh baseline" — two
// ReadAll calls against the same session both do a full re-read (no
// caching), and if the radio's content changed between them, the second
// call's digest reflects the CHANGE, not the first call's snapshot.
func TestReadAll_FreshEveryCall(t *testing.T) {
	_, sess := openSimSession(t)
	svc := NewService(sess, newStore(t), WithNow(func() time.Time { return fixedNow }))

	first, err := svc.ReadAll(testCtx(t))
	if err != nil {
		t.Fatalf("first ReadAll: %v", err)
	}

	// Mutate the radio directly through the same session (simulating "the
	// radio changed"), bypassing the clone Service entirely.
	changed := writableChannel("010", 14_300_000, "CHANGED")
	if _, err := sess.WriteChannel(testCtx(t), changed); err != nil {
		t.Fatalf("direct WriteChannel: %v", err)
	}

	second, err := svc.ReadAll(testCtx(t))
	if err != nil {
		t.Fatalf("second ReadAll: %v", err)
	}

	if first.Radio.BaselineDigest == second.Radio.BaselineDigest {
		t.Error("second ReadAll's digest equals the first's after a direct write — ReadAll must not be cached")
	}
	var got *codeplug.ChannelData
	for _, ch := range second.Channels {
		if ch.Slot == "010" {
			got = ch.Data
		}
	}
	if got == nil || got.FreqHz != 14_300_000 || got.Tag != "CHANGED" {
		t.Errorf("second ReadAll slot \"010\" = %+v, want the freshly written value", got)
	}
}

// TestReadAll_ContextCancelled: a context cancelled before ReadAll starts
// is refused immediately.
func TestReadAll_ContextCancelled(t *testing.T) {
	_, sess := openSimSession(t)
	svc := NewService(sess, newStore(t))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := svc.ReadAll(ctx); err == nil {
		t.Error("ReadAll with an already-cancelled context = nil error, want an error")
	}
}
