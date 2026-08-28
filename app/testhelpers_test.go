// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
	"github.com/gm5dna/open-rig-programmer/internal/wiring"
)

// testAppCtx returns a fresh timeout-bounded context.Context, cleaned up
// at test end — used by helpers in this file that need a ctx before an
// *App (and its own a.ctx) exists yet.
func testAppCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), testAppCtxTimeout)
	t.Cleanup(cancel)
	return ctx
}

// This file's fakeradio session-construction helpers mirror
// core/clone/helpers_test.go's openSimSession/minimalFactoryImage
// (restated here — those are unexported in a package this one cannot
// import), and cmd/rigprog's own probe_test.go/settings_test.go pattern
// of driving a custom factory image through wiring.FakeSessionOpts +
// wiring.OpenFakeSessionFor rather than constructing a session directly
// (task 41, M9a-5: app/ imports no core/driver/ft710 at all any more,
// even in test files, so this can no longer build a session via
// ft710.New(ft710.Simulated).Open the way it did before that task).
// A test that needs to assert on the radio's OWN state after a write
// (rather than reading it back through the session, driver-neutrally)
// no longer has a raw *fakeradio.Radio handle available here — see
// send_test.go's connectDirect callers, which read back via
// driver.Session.ReadChannel instead.

// minimalFactoryImage is a factory image with ONLY M-01 populated, no
// other MEM slot, no 60m, no EMG, and (Codex M5b fix wave, Fix 3,
// adjudicated HIGH) EVERY PMS slot EMPTY — real radios ship
// all-PMS-empty; PMS is no longer a NoBlank bank (see
// core/driver/ft710/caps.go). Keeps a full ReadAll to the minimum slot
// count (117) fakeradio's default UK image would not (124, with 7 60m
// channels). Mirrors core/clone/helpers_test.go's minimalFactoryImage.
// Tests needing a populated PMS slot overlay one via fakeradio.WithSlot
// (see pmsModifiableSeed).
func minimalFactoryImage() map[string]fakeradio.MemState {
	return map[string]fakeradio.MemState{
		"001": {
			Freq: "007000000", ClarSign: '+', ClarMag: "0000",
			Mode: '1', Kind: '1', CTCSS: '0', Shift: '0',
			Populated: true,
		},
	}
}

// pmsModifiableSeed is a populated PMS MemState (14.000000 MHz, matching
// minimalFactoryPopulated's own P1L value before Fix 3) — overlaid onto
// "P1L" via fakeradio.WithSlot by tests that need a genuine, independently
// modifiable PMS channel (prepareTwoDeltaCandidate's "P1L" edit), since
// the default factory image now leaves every PMS slot empty.
var pmsModifiableSeed = fakeradio.MemState{
	Freq: "014000000", ClarSign: '+', ClarMag: "0000",
	Mode: '2', Kind: '5', CTCSS: '0', Shift: '0',
	Populated: true,
}

// openTestSimSession opens a driver.Session against a fresh in-process
// fakeradio.Radio built with opts (defaulting to minimalFactoryImage
// unless opts overrides WithFactoryImage), via
// internal/wiring.OpenFakeSessionFor + wiring.FakeSessionOpts — the same
// test-only seam cmd/rigprog's own probe_test.go/settings_test.go use
// (task 41, M9a-5) — registering cleanup for both the session and the
// fake rig behind it (OpenFakeSessionFor's own closeAll). Unlike the
// pre-task-41 version, this does not hand back a raw *fakeradio.Radio: a
// test that needs to inspect post-write state does so driver-neutrally,
// via the returned session's own ReadChannel (see send_test.go).
//
// wiring.FakeSessionOpts is process-global, unsynchronised state (see its
// own doc comment, internal/wiring/fake.go) — restored via t.Cleanup, and
// safe here because no test using it calls t.Parallel().
func openTestSimSession(t *testing.T, opts ...fakeradio.Option) driver.Session {
	t.Helper()
	allOpts := append([]fakeradio.Option{fakeradio.WithFactoryImage(minimalFactoryImage)}, opts...)
	prevOpts := wiring.FakeSessionOpts
	wiring.FakeSessionOpts = allOpts
	t.Cleanup(func() { wiring.FakeSessionOpts = prevOpts })

	sess, closeAll, err := wiring.OpenFakeSessionFor(testAppCtx(t), wiring.DefaultModel)
	if err != nil {
		t.Fatalf("OpenFakeSessionFor: unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = closeAll() })
	return sess
}

// writableChannel returns a codeplug.Channel for slot at freqHz, USB,
// CTCSS off, simplex, ScanSkip/CTCSSTone left Unknown (the CAT-honest
// state) — mirrors core/clone/helpers_test.go's writableChannel.
// yaesuTierUnavailable sets every one of the ten fields the Icom tier
// added to Unavailable and returns d.
//
// It is what a READ of any radio registered today reports for all ten
// (core/driver/*/read.go), what a load of a pre-tier codeplug migrates
// to, and what a version-1 CSV import produces. A test fixture that left
// them at the zero value would differ from a real read in ten fields,
// and codeplug.Diff compares ChannelData with ==, so an otherwise
// identical channel would plan as MODIFIED.
func yaesuTierUnavailable(d *codeplug.ChannelData) *codeplug.ChannelData {
	d.TxFreqHz = codeplug.FreqField{State: codeplug.Unavailable}
	d.Duplex = codeplug.StringField{State: codeplug.Unavailable}
	d.OffsetHz = codeplug.FreqField{State: codeplug.Unavailable}
	d.ToneMode = codeplug.StringField{State: codeplug.Unavailable}
	d.ToneTx = codeplug.ToneField{State: codeplug.Unavailable}
	d.ToneRx = codeplug.ToneField{State: codeplug.Unavailable}
	d.DTCSCode = codeplug.IntField{State: codeplug.Unavailable}
	d.DTCSPolarity = codeplug.StringField{State: codeplug.Unavailable}
	d.Filter = codeplug.StringField{State: codeplug.Unavailable}
	d.DataMode = codeplug.BoolField{State: codeplug.Unavailable}
	return d
}

func writableChannel(slot string, freqHz uint64, tag string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: yaesuTierUnavailable(&codeplug.ChannelData{
			FreqHz:     freqHz,
			Mode:       "USB",
			CTCSS:      "OFF",
			CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
			Shift:      "SIMPLEX",
			Tag:        tag,
			TagDisplay: codeplug.BoolField{State: codeplug.Known, Value: tag != ""},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
		}),
	}
}

// minimalFactoryPopulated mirrors, as codeplug.ChannelData, exactly what
// minimalFactoryImage's MemState values decode to via a real
// ReadChannel: "001" at 7.000000 MHz LSB, every PMS slot empty (real
// shape — see minimalFactoryImage's doc comment). Mirrors
// core/clone/helpers_test.go's function of the same name.
func minimalFactoryPopulated() map[string]*codeplug.ChannelData {
	m := map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 7_000_000, "").Data,
	}
	m["001"].Mode = "LSB"
	return m
}

// matchingCandidateFile builds a *codeplug.Codeplug describing exactly
// populated's state for every slot caps lists (so codeplug.Diff reports
// every one of those slots Unchanged), except each slot named in edits,
// which carries that entry's Data instead. A slot in neither map is left
// empty. Mirrors core/clone/helpers_test.go's function of the same name
// — used here so a test can drive PrepareSend/Execute against a session
// built by openTestSimSession WITHOUT first spending a real ReadRadio
// (~5s) just to obtain a matching baseline.
func matchingCandidateFile(caps spec.Capabilities, populated map[string]*codeplug.ChannelData, edits map[string]*codeplug.ChannelData) *codeplug.Codeplug {
	var channels []codeplug.Channel
	for _, bank := range caps.Banks {
		for _, slot := range bank.Slots {
			if data, ok := edits[slot]; ok {
				channels = append(channels, codeplug.Channel{Slot: slot, Data: copyChannelData(data)})
				continue
			}
			if data, ok := populated[slot]; ok {
				channels = append(channels, codeplug.Channel{Slot: slot, Data: copyChannelData(data)})
				continue
			}
			channels = append(channels, codeplug.Channel{Slot: slot})
		}
	}
	return &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Radio:    codeplug.RadioInfo{Model: caps.Model, CATID: caps.CATID},
		Channels: channels,
	}
}

// waitForEvent polls rec for the first event named event, up to timeout,
// returning it (or failing the test).
func waitForEvent(t *testing.T, rec *eventRecorder, event string, timeout time.Duration) recordedEvent {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if got := rec.named(event); len(got) > 0 {
			return got[0]
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for event %q", timeout, event)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitForTransferDone polls rec for a transfer:done event whose Kind
// equals kind, up to timeout. Filters by Kind (not just event name)
// because a busy ReadRadio/DiffAgainstRadio probe (see send_test.go's
// cancel-mid-transfer test, which deliberately calls both from inside
// Execute's own progress hook) ALSO emits a transfer:done event (Kind
// "read"/"diff") — waitForEvent's plain first-match would otherwise
// return the wrong one.
func waitForTransferDone(t *testing.T, rec *eventRecorder, kind string, timeout time.Duration) TransferDoneEvent {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		for _, e := range rec.named("transfer:done") {
			if ev, ok := e.data.(TransferDoneEvent); ok && ev.Kind == kind {
				return ev
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %s waiting for transfer:done (Kind=%q)", timeout, kind)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
