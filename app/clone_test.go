// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/clonewire"
	"github.com/gm5dna/open-rig-programmer/core/transport"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// useFakeClonePort substitutes openCloneRealPort with
// internal/wiring.OpenCloneFakePort for the duration of the test — no
// real hardware exists for any clone-mode radio (plan.md's Verdict), so
// every binding-layer test drives the fake instead of a serial port.
func useFakeClonePort(t *testing.T) {
	t.Helper()
	orig := openCloneRealPort
	openCloneRealPort = func(_ string, profile clonewire.Profile) (transport.Port, error) {
		port, _, err := wiring.OpenCloneFakePort(profile.Model)
		return port, err
	}
	t.Cleanup(func() { openCloneRealPort = orig })
}

// TestArmCloneRead_EmitsArmedBeforePromptIsPossible pins the
// arm-before-prompt ordering at the binding layer (spec.md §Read model
// point 1, §UI/CLI entry): ArmCloneRead must not return until
// core/clonewire.Arm's deadlines are live AND "clone:armed" has been
// emitted — so by the time this call returns, the frontend's prompt is
// already safe to show. ReceiveCloneImage is not even callable yet in
// the sense that matters here (no clone.Service.ReadAll-style pre-op
// exists to race), so "before the prompt is possible" is proved by
// asserting the ONE clone:armed event exists and precedes anything else
// this call could have emitted.
func TestArmCloneRead_EmitsArmedBeforePromptIsPossible(t *testing.T) {
	useFakeClonePort(t)
	a, rec := newTestApp(t)

	if err := a.ArmCloneRead("ignored-for-fake", clonewire.FT817.Model); err != nil {
		t.Fatalf("ArmCloneRead: %v", err)
	}

	armed := rec.named("clone:armed")
	if len(armed) != 1 {
		t.Fatalf("clone:armed events = %d, want exactly 1", len(armed))
	}
	ev, ok := armed[0].data.(CloneArmedEvent)
	if !ok {
		t.Fatalf("clone:armed payload type = %T, want CloneArmedEvent", armed[0].data)
	}
	if ev.Model != clonewire.FT817.Model {
		t.Errorf("clone:armed Model = %q, want %q", ev.Model, clonewire.FT817.Model)
	}

	// No transfer:done has fired — ReceiveCloneImage has not been called
	// yet, so nothing has completed or refused.
	if got := rec.named("transfer:done"); len(got) != 0 {
		t.Errorf("transfer:done events after ArmCloneRead alone = %d, want 0", len(got))
	}
}

// TestReceiveCloneImage_SavesWorkingCopyWithRawImage drives the whole
// Arm -> Receive sequence against the fake and pins the two things
// plan.md's Phase 4b test brief asks for: a successful save carries
// RawImage (Model/ProfileID/Bytes all populated, Bytes non-empty), and
// the working copy this becomes is reachable via GetCodeplug exactly
// like an ordinary ReadRadio result.
func TestReceiveCloneImage_SavesWorkingCopyWithRawImage(t *testing.T) {
	useFakeClonePort(t)
	a, rec := newTestApp(t)

	if err := a.ArmCloneRead("ignored-for-fake", clonewire.FT817.Model); err != nil {
		t.Fatalf("ArmCloneRead: %v", err)
	}

	view, err := a.ReceiveCloneImage()
	if err != nil {
		t.Fatalf("ReceiveCloneImage: %v", err)
	}
	if view.Radio.Model != clonewire.FT817.Model {
		t.Errorf("ReceiveCloneImage view.Radio.Model = %q, want %q", view.Radio.Model, clonewire.FT817.Model)
	}
	if len(view.Channels) == 0 {
		t.Fatal("ReceiveCloneImage: zero channels — sanity check failed")
	}

	a.mu.Lock()
	raw := a.working.RawImage
	a.mu.Unlock()
	if raw == nil {
		t.Fatal("ReceiveCloneImage: working copy has RawImage = nil, want it populated")
	}
	if raw.Model != clonewire.FT817.Model {
		t.Errorf("RawImage.Model = %q, want %q", raw.Model, clonewire.FT817.Model)
	}
	if raw.ProfileID != clonewire.FT817.ProfileID {
		t.Errorf("RawImage.ProfileID = %q, want %q", raw.ProfileID, clonewire.FT817.ProfileID)
	}
	if len(raw.Bytes) == 0 {
		t.Error("RawImage.Bytes is empty, want the whole received image")
	}

	doneEv := waitForTransferDone(t, rec, "clone", time.Second)
	if doneEv.Outcome != "ok" {
		t.Errorf("transfer:done Kind=clone Outcome = %q, want %q", doneEv.Outcome, "ok")
	}

	// The reservation is released: a and its opBusy field are back to
	// free, exactly like ReadRadio leaves it after completion.
	a.mu.Lock()
	busy := a.opBusy
	stillArmed := a.cloneState != nil
	a.mu.Unlock()
	if busy != "" {
		t.Errorf("a.opBusy after ReceiveCloneImage = %q, want \"\" (released)", busy)
	}
	if stillArmed {
		t.Error("a.cloneState after ReceiveCloneImage is still set, want nil")
	}
}

// TestArmCloneRead_ReservationRefusesConcurrentReadRadio proves the
// clone-mode read reservation composes with the existing App-level
// exclusive-operation reservation exactly as ReadRadio/PrepareSend/
// ReadSettingsRadio already refuse each other (reservation.go): a
// ReadRadio call while ArmCloneRead's reservation is held must be
// refused with *OperationBusyError naming "ArmCloneRead", never allowed
// to interleave.
func TestArmCloneRead_ReservationRefusesConcurrentReadRadio(t *testing.T) {
	useFakeClonePort(t)
	a, _ := newTestApp(t)
	// ReadRadio refuses on ErrNotConnected before it ever reaches the
	// reservation check (app/codeplug.go, same order as PrepareSend/
	// ReadSettingsRadio) — a real connection has to be present for the
	// busy-refusal path this test targets to be reachable at all.
	connectDirect(t, a, openTestSimSession(t), nil)

	if err := a.ArmCloneRead("ignored-for-fake", clonewire.FT817.Model); err != nil {
		t.Fatalf("ArmCloneRead: %v", err)
	}

	_, err := a.ReadRadio()
	checkOperationBusy(t, "ReadRadio while ArmCloneRead is reserved", err, "ArmCloneRead")

	// Cleanly release, so t.Cleanup's port-close doesn't race a lingering
	// receive.
	if err := a.CancelCloneRead(); err != nil {
		t.Fatalf("CancelCloneRead: %v", err)
	}
}

// TestCancelCloneRead_ReleasesReservation pins the safety valve an armed
// read that is never followed through to ReceiveCloneImage needs: without
// it, a.opBusy would stay held for the rest of the session (app/clone.go's
// CancelCloneRead doc comment).
func TestCancelCloneRead_ReleasesReservation(t *testing.T) {
	useFakeClonePort(t)
	a, _ := newTestApp(t)

	if err := a.ArmCloneRead("ignored-for-fake", clonewire.FT817.Model); err != nil {
		t.Fatalf("ArmCloneRead: %v", err)
	}
	if err := a.CancelCloneRead(); err != nil {
		t.Fatalf("CancelCloneRead: %v", err)
	}

	a.mu.Lock()
	busy := a.opBusy
	a.mu.Unlock()
	if busy != "" {
		t.Errorf("a.opBusy after CancelCloneRead = %q, want \"\" (released)", busy)
	}

	if err := a.CancelCloneRead(); !errors.Is(err, ErrCloneNotArmed) {
		t.Errorf("CancelCloneRead (nothing armed) = %v, want ErrCloneNotArmed", err)
	}
}

// TestReceiveCloneImage_UnknownModelRefusesBeforeArm pins ArmCloneRead's
// own unknown-model refusal (wiring.OpenCloneReader), never opening a
// port or reserving anything for a name outside wiring.CloneModels().
func TestArmCloneRead_UnknownModelRefuses(t *testing.T) {
	useFakeClonePort(t)
	a, _ := newTestApp(t)

	err := a.ArmCloneRead("ignored-for-fake", "not-a-real-clone-model")
	if err == nil {
		t.Fatal("ArmCloneRead(unknown model): want an error, got nil")
	}

	a.mu.Lock()
	busy := a.opBusy
	a.mu.Unlock()
	if busy != "" {
		t.Errorf("a.opBusy after a refused ArmCloneRead = %q, want \"\" (never reserved)", busy)
	}
}
