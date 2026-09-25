// SPDX-License-Identifier: GPL-3.0-or-later

package wiring

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/clonewire"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// TestCloneModelsDisjointFromSessionModels is the concrete proof (Phase 4
// brief, REQUIRED) that no write route can ever reach a clone-mode model:
// SupportedModels() (realDrivers) and CloneModels() (cloneModels) share no
// name.
func TestCloneModelsDisjointFromSessionModels(t *testing.T) {
	session := SupportedModels()
	clone := CloneModels()
	if len(session) == 0 || len(clone) == 0 {
		t.Fatalf("TestCloneModelsDisjointFromSessionModels: SupportedModels()=%d CloneModels()=%d, want both non-empty or the test passes vacuously", len(session), len(clone))
	}
	for _, m := range clone {
		if slices.Contains(session, m) {
			t.Errorf("model %q appears in BOTH SupportedModels() and CloneModels() — the disjoint-registry guarantee is broken", m)
		}
	}
}

func TestOpenCloneReader_UnknownModel(t *testing.T) {
	_, _, err := OpenCloneReader("NO-SUCH-CLONE-MODEL")
	var unk *UnknownModelError
	if !errors.As(err, &unk) {
		t.Fatalf("OpenCloneReader(unknown): got %v, want *UnknownModelError", err)
	}
	if !slices.Equal(unk.Supported, CloneModels()) {
		t.Errorf("OpenCloneReader(unknown).Supported = %v, want CloneModels()", unk.Supported)
	}
}

func TestOpenCloneReader_OneCandidatePerModel(t *testing.T) {
	for _, model := range CloneModels() {
		reader, profiles, err := OpenCloneReader(model)
		if err != nil {
			t.Fatalf("OpenCloneReader(%q): unexpected error: %v", model, err)
		}
		if reader == nil {
			t.Errorf("OpenCloneReader(%q): nil reader", model)
		}
		if len(profiles) != 1 || profiles[0].Model != model {
			t.Errorf("OpenCloneReader(%q) candidates = %+v, want exactly one Profile whose Model matches — identity is operator-asserted, never a family offered together", model, profiles)
		}
	}
}

func TestOpenCloneFakePort_UnknownModel(t *testing.T) {
	_, _, err := OpenCloneFakePort("NO-SUCH-CLONE-MODEL")
	var unk *UnknownModelError
	if !errors.As(err, &unk) {
		t.Fatalf("OpenCloneFakePort(unknown): got %v, want *UnknownModelError", err)
	}
}

// TestOpenRealSessionWith_RefusesCloneModel is test 3 of the Phase 4
// write-refusal proof (plan.md): OpenRealSessionWith is the exact
// function app.PrepareSend/app.ConfirmSend call
// (openRealSessionWith := wiring.OpenRealSessionWith, app/connection.go)
// — calling it directly with a clone-only model name proves the
// unknown-model boundary refuses it before any driver.Session is
// obtained, with ZERO bytes read or written to a port: the openSerial
// seam below is never even invoked, since realDriverFor fails first.
func TestOpenRealSessionWith_RefusesCloneModel(t *testing.T) {
	const model = "FT-817"
	if _, ok := cloneModels[model]; !ok {
		t.Fatalf("test fixture error: %q is not a registered clone model", model)
	}

	called := false
	prev := openSerial
	openSerial = func(string, transport.SerialConfig) (transport.Port, error) {
		called = true
		return nil, errSeamRefused
	}
	t.Cleanup(func() { openSerial = prev })

	sess, closeAll, err := OpenRealSessionWith(testCtx(t), model, "/dev/does-not-matter", SessionOptions{})
	var unk *UnknownModelError
	if !errors.As(err, &unk) {
		t.Fatalf("OpenRealSessionWith(%q): got %v, want *UnknownModelError", model, err)
	}
	if sess != nil || closeAll != nil {
		t.Errorf("OpenRealSessionWith(%q): want nil session/closeAll, got sess=%v closeAllIsNil=%v", model, sess, closeAll == nil)
	}
	if called {
		t.Error("OpenRealSessionWith: openSerial was called for a clone-only model — no port should ever be opened, let alone read from or written to")
	}
}

// TestOpenCloneFakePort_RoundTrips proves the fake port this milestone's
// CLI drives is actually wired to core/clonewire's own Arm/Receive, for
// one model per family (mirrors the BI script's own per-family spot
// check).
func TestOpenCloneFakePort_RoundTrips(t *testing.T) {
	for _, model := range []string{clonewire.FT817.Model, clonewire.FT857.Model, clonewire.FT897.Model} {
		t.Run(model, func(t *testing.T) {
			reader, profiles, err := OpenCloneReader(model)
			if err != nil {
				t.Fatalf("OpenCloneReader(%q): %v", model, err)
			}
			port, closePort, err := OpenCloneFakePort(model)
			if err != nil {
				t.Fatalf("OpenCloneFakePort(%q): %v", model, err)
			}
			defer func() { _ = closePort() }()

			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			reception, err := reader.Arm(ctx, port, profiles)
			if err != nil {
				t.Fatalf("Arm(%q): %v", model, err)
			}
			<-reception.Armed()
			img, err := reception.Receive(ctx)
			if err != nil {
				t.Fatalf("Receive(%q): %v", model, err)
			}
			if img.Profile.Model != model {
				t.Errorf("Receive(%q).Profile.Model = %q, want %q", model, img.Profile.Model, model)
			}
			if len(img.Raw) != profiles[0].ImageLen {
				t.Errorf("Receive(%q): got %d raw bytes, want %d (ImageLen)", model, len(img.Raw), profiles[0].ImageLen)
			}
		})
	}
}
