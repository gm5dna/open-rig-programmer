// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/clonewire"
	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

func TestCmdCloneRead_NeitherPortNorFake(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "f.json")
	var stdout, stderr bytes.Buffer
	got := cmdRead(testCtx(t), []string{"--clone", "--model", "FT-817", "--out", out}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdRead(--clone, no --port/--fake) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

func TestCmdCloneRead_BothPortAndFake(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "f.json")
	var stdout, stderr bytes.Buffer
	got := cmdRead(testCtx(t), []string{"--clone", "--model", "FT-817", "--port", "/dev/cu.fake", "--fake", "--out", out}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdRead(--clone, both --port and --fake) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

func TestCmdCloneRead_MissingOut(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cmdRead(testCtx(t), []string{"--clone", "--model", "FT-817", "--fake"}, &stdout, &stderr)
	if got != exitUsage {
		t.Errorf("cmdRead(--clone --fake, no --out) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
}

// TestCmdCloneRead_UnknownModel pins that --clone validates against the
// SEPARATE wiring.CloneModels() list, not wiring.SupportedModels() — an
// ordinary radio model (the default, FT-710) is UNKNOWN on the clone
// path, exactly as a clone model would be unknown on the ordinary path
// (see TestCmdWrite_RefusesCloneModel in write_test.go).
func TestCmdCloneRead_UnknownModel(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "f.json")
	var stdout, stderr bytes.Buffer
	got := cmdRead(testCtx(t), []string{"--clone", "--fake", "--model", "FT-710", "--out", out}, &stdout, &stderr)
	if got != exitUsage {
		t.Fatalf("cmdRead(--clone --model FT-710) = %d, want exitUsage (%d); stderr=%q", got, exitUsage, stderr.String())
	}
	if !strings.Contains(stderr.String(), "FT-710") {
		t.Errorf("cmdRead(--clone --model FT-710) stderr = %q, want it to name the rejected model", stderr.String())
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("cmdRead(--clone, unknown model): --out was written despite the model being rejected")
	}
}

func TestCmdCloneRead_RefuseOverwrite(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "existing.json")
	const sentinel = "not a codeplug file"
	if err := os.WriteFile(out, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("seeding existing --out file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	got := cmdRead(testCtx(t), []string{"--clone", "--fake", "--model", "FT-817", "--out", out}, &stdout, &stderr)
	if got != exitError {
		t.Errorf("cmdCloneRead(existing --out, no --force) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Errorf("cmdCloneRead(existing --out) stderr = %q, want it to mention the file already exists", stderr.String())
	}
	contents, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading %s after refused read: %v", out, err)
	}
	if string(contents) != sentinel {
		t.Error("cmdCloneRead(existing --out, no --force) overwrote the file as a side effect, want it untouched")
	}
}

// TestCmdCloneRead_FakeEndToEnd is the required end-to-end proof (plan.md
// Phase 4): "rigprog read --clone" against the Phase 3 fake
// (internal/fakeyaesuclone, via wiring.OpenCloneFakePort) arms before
// prompting (the prompt is only ever printed after a successful Arm — see
// cmdCloneRead's unconditional ordering) and writes a codeplug file with
// RawImage populated with the WHOLE received image, one model per family.
func TestCmdCloneRead_FakeEndToEnd(t *testing.T) {
	for _, tc := range []struct {
		model   string
		profile clonewire.Profile
	}{
		{clonewire.FT817.Model, clonewire.FT817},
		{clonewire.FT857.Model, clonewire.FT857},
		{clonewire.FT897.Model, clonewire.FT897},
	} {
		t.Run(tc.model, func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "out.json")

			var stdout, stderr bytes.Buffer
			got := cmdRead(testCtx(t), []string{"--clone", "--fake", "--model", tc.model, "--out", out}, &stdout, &stderr)
			if got != exitSuccess {
				t.Fatalf("cmdRead(--clone --fake --model %s) = %d, want exitSuccess (%d); stderr=%q", tc.model, got, exitSuccess, stderr.String())
			}

			if !strings.Contains(stderr.String(), cloneArmedPrompt) {
				t.Errorf("stderr = %q, want the arm-before-prompt message", stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.model) {
				t.Errorf("stderr = %q, want the operator-asserted model name", stderr.String())
			}

			cp, err := codeplug.Load(out)
			if err != nil {
				t.Fatalf("loading saved codeplug: %v", err)
			}
			if cp.RawImage == nil {
				t.Fatal("saved codeplug has no RawImage — the whole-image blob was not populated")
			}
			if cp.RawImage.ProfileID != tc.profile.ProfileID {
				t.Errorf("RawImage.ProfileID = %q, want %q", cp.RawImage.ProfileID, tc.profile.ProfileID)
			}
			if len(cp.RawImage.Bytes) != tc.profile.ImageLen {
				t.Errorf("RawImage.Bytes has %d bytes, want %d (ImageLen) — the WHOLE image, never a trimmed subset", len(cp.RawImage.Bytes), tc.profile.ImageLen)
			}
			if len(cp.Channels) != tc.profile.ChannelCount {
				t.Errorf("saved %d channels, want %d (ChannelCount)", len(cp.Channels), tc.profile.ChannelCount)
			}
		})
	}
}

// TestClassifyCloneReceiveErr pins that all three of core/clonewire's
// typed refusal sentinels (spec.md §Read model point 5) are named
// distinctly in the CLI's own error text. The candidate set
// wiring.OpenCloneReader offers is always a SINGLETON (model identity is
// operator-asserted — see clonefake.go's own doc comment), so
// ErrImageAmbiguous can never actually arise via this CLI path in
// practice; core/clonewire and internal/fakeyaesuclone already exercise
// it directly (e.g. TestFT857Family_Ambiguous,
// TestEndToEnd_CombinedFT857FamilyAmbiguous) against a multi-candidate
// call the CLI itself never makes. This test only pins the CLI's own
// error-text mapping, for all three sentinels, without needing a live
// wire failure to reach it.
func TestClassifyCloneReceiveErr(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{clonewire.ErrImageIncomplete, "incomplete clone-mode image"},
		{clonewire.ErrImageIncompatible, "incompatible clone-mode image"},
		{clonewire.ErrImageAmbiguous, "ambiguous clone-mode image"},
		{errors.New("boom"), "boom"},
	} {
		if got := classifyCloneReceiveErr(tc.err); !strings.Contains(got, tc.want) {
			t.Errorf("classifyCloneReceiveErr(%v) = %q, want it to contain %q", tc.err, got, tc.want)
		}
	}
}
