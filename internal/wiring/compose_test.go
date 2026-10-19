// SPDX-License-Identifier: GPL-3.0-or-later

package wiring

import (
	"errors"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/clone"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakeft450d"
)

// composeBaseline opens model's fake, reads the whole radio and returns the
// session, the service and the baseline.
func composeBaseline(t *testing.T, model string) (driver.Session, *clone.Service, *codeplug.Codeplug) {
	t.Helper()
	ctx := testCtx(t)
	sess, closeAll, err := OpenFakeSessionFor(ctx, model)
	if err != nil {
		t.Fatalf("OpenFakeSessionFor(%q): %v", model, err)
	}
	t.Cleanup(func() {
		if err := closeAll(); err != nil {
			t.Errorf("closeAll: %v", err)
		}
	})
	service := clone.NewService(sess, clone.SnapshotStore{Dir: t.TempDir()})
	baseline, err := service.ReadAll(ctx)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return sess, service, baseline
}

// TestCompose_ReadAllToReadBack_PerEngine runs ReadAll, PrepareSend, Execute
// and an independent session read-back for the FT-710 and FT-450D write
// engines. The FT-991A's own round trip is
// TestOpenFakeSessionFor_FT991ACloneWriteVerifyRoundTrip. The IC-7610 (its
// fake seeds no channel) and TS-590S (A9 refuses every MEM write) cannot
// compose from their default fakes; see the lane report.
func TestCompose_ReadAllToReadBack_PerEngine(t *testing.T) {
	for _, model := range []string{DefaultModel, FT450DModel} {
		t.Run(model, func(t *testing.T) {
			if model == FT450DModel {
				// The default image populates only PMS pair 1, which its own
				// NoBlank validation refuses; fill pair 2 so a send can pass.
				prev := FT450DFakeSessionOpts
				pair1 := fakeft450d.DefaultImage()["501"]
				FT450DFakeSessionOpts = []fakeft450d.Option{fakeft450d.WithSlot("503", pair1), fakeft450d.WithSlot("504", pair1)}
				t.Cleanup(func() { FT450DFakeSessionOpts = prev })
			}
			ctx := testCtx(t)
			sess, service, baseline := composeBaseline(t, model)
			caps := sess.Capabilities()

			var slot string
			var after codeplug.ChannelData
			for _, ch := range baseline.Channels {
				if b, _ := caps.BankOf(ch.Slot); ch.Data != nil && b == spec.BankMemory {
					slot, after = ch.Slot, *ch.Data
					break
				}
			}
			if slot == "" {
				t.Fatalf("%s: the default image has no occupied MEM slot", model)
			}
			// One field the radio grades writable: the tag where it has
			// one, else the frequency.
			if caps.TagLen > 0 {
				after.Tag = "ZQ1"
				if after.Tag == baselineTag(baseline, slot) {
					after.Tag = "ZQ2"
				}
			} else {
				after.FreqHz += 5_000
			}

			file := &codeplug.Codeplug{Schema: baseline.Schema, Radio: baseline.Radio}
			file.Channels = append(file.Channels, baseline.Channels...)
			for i := range file.Channels {
				if file.Channels[i].Slot == slot {
					file.Channels[i].Data = &after
				}
			}

			plan, err := service.PrepareSend(ctx, file)
			if err != nil {
				t.Fatalf("PrepareSend: %v", err)
			}
			var modified int
			for _, e := range plan.Diff().Entries {
				if e.Blocked {
					t.Fatalf("slot %q Blocked: %s", e.Slot, e.BlockReason)
				}
				if e.Kind == codeplug.DiffModified {
					modified++
				}
			}
			if modified != 1 {
				t.Fatalf("Modified entries = %d, want 1", modified)
			}

			report, err := service.Execute(ctx, plan, plan.ConfirmationDigest())
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if report.Aborted || report.Written != 1 || report.Verified != 1 {
				t.Fatalf("Aborted=%v (%s) Written=%d Verified=%d, want not aborted, 1, 1", report.Aborted, report.AbortReason, report.Written, report.Verified)
			}

			got, err := sess.ReadChannel(ctx, slot)
			if err != nil || got.Data == nil {
				t.Fatalf("ReadChannel(%q) after the send: %v, data %v", slot, err, got.Data)
			}
			if got.Data.Tag != after.Tag || got.Data.FreqHz != after.FreqHz {
				t.Errorf("read-back Tag %q, FreqHz %d, want %q, %d", got.Data.Tag, got.Data.FreqHz, after.Tag, after.FreqHz)
			}
		})
	}
}

func baselineTag(cp *codeplug.Codeplug, slot string) string {
	for _, ch := range cp.Channels {
		if ch.Slot == slot && ch.Data != nil {
			return ch.Data.Tag
		}
	}
	return ""
}

// TestCompose_CloneModeHasNoWriteSession pins the owner ruling of 25/09/2026
// (clone mode is read-only): no clone-mode model opens a driver session, so
// ReadAll, PrepareSend and Execute can never reach it. (The FT-890/900/1000MP
// are binary-CAT session models and write normally.)
func TestCompose_CloneModeHasNoWriteSession(t *testing.T) {
	for _, model := range CloneModels() {
		_, _, err := OpenFakeSessionFor(testCtx(t), model)
		var unk *UnknownModelError
		if !errors.As(err, &unk) {
			t.Errorf("OpenFakeSessionFor(%q) = %v, want *UnknownModelError", model, err)
		}
	}
}
