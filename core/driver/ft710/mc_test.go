// SPDX-License-Identifier: GPL-3.0-or-later

package ft710

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// TestCurrentMemory_ReportsRadioSelection: CurrentMemory queries MC; and
// reports the radio's current memory selection as a canonical wire-form
// slot after an MW write moves it (HW-CONFIRMED 2026-07-13, M5b write
// trials, docs/hardware-notes.md), not merely after an MC-set recall.
// Before any write/recall, fakeradio's default selection is "000"
// (VFO/no-stored-memory) — see TestCurrentMemory_InvalidAnswerIsUnavailable
// for that case.
func TestCurrentMemory_ReportsRadioSelection(t *testing.T) {
	_, sess := openSession(t, Simulated, fakeradio.WithFactoryImage(minimalImage))

	if _, err := sess.WriteChannel(testCtx(t), writableChannel("010")); err != nil {
		t.Fatalf("WriteChannel: unexpected error: %v", err)
	}
	got, err := sess.CurrentMemory(testCtx(t))
	if err != nil {
		t.Fatalf("CurrentMemory (after write): unexpected error: %v", err)
	}
	if got != "010" {
		t.Errorf("CurrentMemory (after write) = %q, want \"010\" (MW moves selection)", got)
	}
}

// TestRecallMemory_MovesRadioSelection: RecallMemory issues an MC-set and
// the radio's selection (per CurrentMemory/fakeradio's CurrentChannel)
// moves to the recalled slot.
func TestRecallMemory_MovesRadioSelection(t *testing.T) {
	radio, sess := openSession(t, Simulated, fakeradio.WithFactoryImage(minimalImage))

	if err := sess.RecallMemory(testCtx(t), "001"); err != nil {
		t.Fatalf("RecallMemory(\"001\"): unexpected error: %v", err)
	}
	if got := radio.CurrentChannel(); got != "001" {
		t.Errorf("fakeradio CurrentChannel() after RecallMemory(\"001\") = %q, want \"001\"", got)
	}
	got, err := sess.CurrentMemory(testCtx(t))
	if err != nil {
		t.Fatalf("CurrentMemory: unexpected error: %v", err)
	}
	if got != "001" {
		t.Errorf("CurrentMemory() after RecallMemory(\"001\") = %q, want \"001\"", got)
	}
}

// TestRecallMemory_EmptySlotRejected: recalling an unpopulated slot is
// rejected — ASSUMED, by analogy with MR's identical empty-slot rule, not
// itself hardware-probed at M5b (docs/hardware-notes.md's "explicitly not
// probed" list).
func TestRecallMemory_EmptySlotRejected(t *testing.T) {
	_, sess := openSession(t, Simulated, fakeradio.WithFactoryImage(minimalImage))

	err := sess.RecallMemory(testCtx(t), "050") // never populated
	if err == nil {
		t.Fatal("RecallMemory(empty slot): want error, got nil")
	}
}

// TestCurrentMemory_InvalidAnswerIsUnavailable: an MC answer this codec
// cannot parse (e.g. "000", the VFO/no-stored-memory case whose semantics
// remain UNTESTED — core/cat/mc.go's doc comment) must never be guessed
// at by the caller; CurrentMemory reports a typed, matchable error rather
// than a slot string a caller might mistakenly treat as real.
func TestCurrentMemory_InvalidAnswerIsUnavailable(t *testing.T) {
	_, sess := openSession(t, Simulated, fakeradio.WithFactoryImage(minimalImage))

	// fakeradio's default currentChannel is "000" (VFO/no-stored-memory) —
	// exactly the case ParseMCAnswer refuses (mc.go's doc comment).
	_, err := sess.CurrentMemory(testCtx(t))
	if !errors.Is(err, ErrMCSnapshotUnavailable) {
		t.Fatalf("CurrentMemory (radio in VFO/\"000\" state) = %v, want errors.Is match against ErrMCSnapshotUnavailable", err)
	}
}
