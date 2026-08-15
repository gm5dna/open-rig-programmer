// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

const testCtxTimeout = 30 * time.Second

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testCtxTimeout)
	t.Cleanup(cancel)
	return ctx
}

// unknownModelSentinel is the model name every unknown-model test in this
// package asks for — the shared sentinel for "a name internal/wiring can
// never resolve", used by validateModel, the six --model-bearing commands'
// rejection tests, the compiled-binary test and the radiotext-absence test.
//
// It is a NEVER-REGISTRABLE name on purpose (M9c-6 task 6). Until this
// milestone these tests spelt the sentinel "FTdx10", which read naturally
// while that radio had no driver — and INVERTED every one of them the
// moment the FTdx10 registered: a registered model is ACCEPTED, so each
// "exits 2 (usage)" assertion would have started failing, and the one
// radiotext test would have found the erase procedure it asserts is absent.
// A name no Yaesu radio will ever carry cannot be overtaken by the next
// driver the way a real model name was.
const unknownModelSentinel = "NO-SUCH-MODEL"

// TestOpenFakeSession exercises cmd/rigprog's openFakeSession alias
// end-to-end against the default (ImageUK) fakeradio image, confirming
// it yields a working driver.Session and that closeAll releases both the
// session and the fakeradio cleanly. The underlying construction is
// pinned directly (TestNewRealDriver_HWVerifiedWriteSet,
// TestSimulatedTokenSingleNonTestFile) in internal/wiring and
// internal/guards respectively (task-15's extraction) — this test's job
// is only to confirm cmd/rigprog's thin alias (wiring.go) still reaches
// it, now via the model-keyed OpenFakeSessionFor (task 40).
func TestOpenFakeSession(t *testing.T) {
	sess, closeAll, err := openFakeSession(testCtx(t), wiring.DefaultModel)
	if err != nil {
		t.Fatalf("openFakeSession: unexpected error: %v", err)
	}
	if sess == nil {
		t.Fatal("openFakeSession: nil session with nil error")
	}
	id := sess.Identity()
	if id.CATID != "0800" {
		t.Errorf("Identity().CATID = %q, want %q", id.CATID, "0800")
	}
	rr, ok := sess.(driver.RegionReporter)
	if !ok {
		t.Fatal("session does not implement driver.RegionReporter")
	}
	if rr.Region() != "no-60m" {
		t.Errorf("Region() = %q, want %q (default fakeradio image is ImageUK, HW-CONFIRMED 2026-07-13 to have no 5xx bank)", rr.Region(), "no-60m")
	}
	if err := closeAll(); err != nil {
		t.Errorf("closeAll: unexpected error: %v", err)
	}
}

// TestOpenRealSession_BadPort confirms cmd/rigprog's openRealSession
// alias surfaces a port-open failure as a plain error (not a panic), for
// a path that cannot possibly exist.
func TestOpenRealSession_BadPort(t *testing.T) {
	tempUserConfig(t) // openRealSession reads the consent store first
	sess, closeAll, err := openRealSession(testCtx(t), wiring.DefaultModel, "/dev/nonexistent-rigprog-test-port")
	if err == nil {
		t.Fatal("openRealSession: expected an error opening a nonexistent port, got nil")
	}
	if sess != nil || closeAll != nil {
		t.Errorf("openRealSession: expected nil session/closeAll on error, got sess=%v closeAllIsNil=%v", sess, closeAll == nil)
	}
}

// TestOpenRealSession_UnknownModel pins task 40's model-keyed wiring at
// the cmd/rigprog alias level: an unrecognised model fails BEFORE any
// port is touched, with the underlying *wiring.UnknownModelError still
// reachable (this alias only translates Register/Serial/Session errors,
// never UnknownModelError, since validateModel already catches that case
// earlier in every real caller — see probe.go/read.go/write.go/diff.go).
func TestOpenRealSession_UnknownModel(t *testing.T) {
	tempUserConfig(t) // openRealSession reads the consent store first
	_, _, err := openRealSession(testCtx(t), unknownModelSentinel, "/dev/nonexistent-rigprog-test-port")
	if err == nil {
		t.Fatal("openRealSession(unknown model): expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "FT-710") {
		t.Errorf("openRealSession(unknown model) error = %q, want it to name the supported model FT-710", err.Error())
	}
}

// TestOpenFakeSession_UnknownModel is TestOpenRealSession_UnknownModel's
// fake-session counterpart.
func TestOpenFakeSession_UnknownModel(t *testing.T) {
	_, _, err := openFakeSession(testCtx(t), unknownModelSentinel)
	if err == nil {
		t.Fatal("openFakeSession(unknown model): expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "FT-710") {
		t.Errorf("openFakeSession(unknown model) error = %q, want it to name the supported model FT-710", err.Error())
	}
}

// TestValidateModel pins validateModel's own contract (task 40 brief):
// a supported model passes silently (true, nothing written); an
// unsupported one writes a usage-style diagnostic naming the supported
// models, then the given usage text, and returns false.
func TestValidateModel(t *testing.T) {
	t.Run("supported", func(t *testing.T) {
		var stderr strings.Builder
		usageCalled := false
		ok := validateModel(&stderr, "probe", "FT-710", func(w io.Writer) {
			usageCalled = true
		})
		if !ok {
			t.Error("validateModel(FT-710) = false, want true")
		}
		if stderr.Len() != 0 {
			t.Errorf("validateModel(FT-710) wrote to stderr: %q, want nothing", stderr.String())
		}
		if usageCalled {
			t.Error("validateModel(FT-710) called printUsage, want it not to")
		}
	})

	t.Run("unsupported", func(t *testing.T) {
		var stderr strings.Builder
		usageCalled := false
		ok := validateModel(&stderr, "probe", unknownModelSentinel, func(w io.Writer) {
			usageCalled = true
		})
		if ok {
			t.Errorf("validateModel(%s) = true, want false", unknownModelSentinel)
		}
		if !usageCalled {
			t.Errorf("validateModel(%s) did not call printUsage, want it to", unknownModelSentinel)
		}
		out := stderr.String()
		if !strings.Contains(out, "rigprog probe: ") {
			t.Errorf("validateModel(%s) stderr = %q, want it prefixed \"rigprog probe: \"", unknownModelSentinel, out)
		}
		if !strings.Contains(out, unknownModelSentinel) || !strings.Contains(out, "FT-710") {
			t.Errorf("validateModel(%s) stderr = %q, want it to name both the rejected model and the supported one", unknownModelSentinel, out)
		}
	})
}
