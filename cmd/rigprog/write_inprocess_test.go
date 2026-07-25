// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/driver/ft710"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// This file drives runWrite directly against a session this file
// constructs itself (mirroring core/clone/helpers_test.go's
// openSimSession, unexported there so restated here), rather than going
// through cmdWrite/openFakeSession — see runWrite's doc comment in
// write.go. That is what gives these tests a handle on both the session
// AND the underlying *fakeradio.Radio at the same time, which is what
// makes the fault/drift/round-trip techniques below possible without any
// new production-only test seam (no equivalent of openFakeSessionOpts
// was needed for this file).

// testIdentity is the caller-side Identity every test session in this
// file opens with — mirrors core/clone/helpers_test.go's testIdentity.
var testIdentity = driver.Identity{Port: "rigprog-test-pipe", USBSerial: "SIM0001"}

// openWriteTestSession opens an ft710.Simulated session against a fresh
// fakeradio.Radio built with opts, registering cleanup for both, and
// returns BOTH the radio and the session — the radio handle is what
// lets a test inspect SlotState (image comparison), or close it
// mid-flow (simulating a disconnect), things cmdWrite's own
// openFakeSession deliberately never exposes to production code.
func openWriteTestSession(t *testing.T, opts ...fakeradio.Option) (*fakeradio.Radio, driver.Session) {
	t.Helper()
	r := fakeradio.New(opts...)
	t.Cleanup(func() { _ = r.Close() })

	sess, err := ft710.New(ft710.Simulated).Open(testCtx(t), r.Port(), testIdentity)
	if err != nil {
		t.Fatalf("Open (Simulated): unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return r, sess
}

// minimalFactoryImage is a factory image with ONLY M-01 populated,
// nothing else — mirrors core/clone/helpers_test.go's helper of the same
// name/shape (unexported there). Every OTHER MEM slot ("002".."099")
// stays empty, i.e. available as an Added-entry target. Every PMS slot
// is ALSO left empty (Codex M5b fix wave, Fix 3, adjudicated HIGH: real
// radios ship all-PMS-empty; PMS is no longer a NoBlank bank — see
// core/driver/ft710/caps.go) — none of this file's tests edit or assert
// on PMS content, so there is nothing to overlay back.
func minimalFactoryImage() map[string]fakeradio.MemState {
	return map[string]fakeradio.MemState{
		"001": {
			Freq: "007000000", ClarSign: '+', ClarMag: "0000",
			Mode: '1', Kind: '1', CTCSS: '0', Shift: '0',
			Populated: true,
		},
	}
}

// minimalFactoryPopulated mirrors, as codeplug.ChannelData, exactly what
// minimalFactoryImage's MemState values decode to via a real
// ReadChannel — see core/clone/helpers_test.go's helper of the same
// name/shape for the identical technique and its rationale (building a
// candidate that matches a given factory image WITHOUT ever reading the
// radio).
func minimalFactoryPopulated() map[string]*codeplug.ChannelData {
	m := map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 7_000_000, "").Data,
	}
	m["001"].Mode = "LSB"
	return m
}

// writableChannel returns a fully-writable populated channel for slot:
// every FieldState-carrying field Unknown (nothing inexpressible
// requested), everything else set to plain, codec-expressible values.
// Mirrors core/clone/helpers_test.go's helper of the same name/shape.
func writableChannel(slot string, freqHz uint32, tag string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz:     freqHz,
			Mode:       "USB",
			CTCSS:      "OFF",
			CTCSSTone:  codeplug.ToneField{State: codeplug.Unknown},
			Shift:      "SIMPLEX",
			Tag:        tag,
			TagDisplay: tag != "",
			ScanSkip:   codeplug.BoolField{State: codeplug.Unknown},
		},
	}
}

// matchingCandidateFile builds a *codeplug.Codeplug describing exactly
// populated's state for every slot caps lists (so codeplug.Diff reports
// every one of those slots Unchanged), except each slot named in edits,
// which carries that entry's Data instead (nil erases/leaves it empty).
// Mirrors core/clone/helpers_test.go's helper of the same name/shape.
// populated MUST match whichever factory image the session under test
// was opened with (see minimalFactoryImage/minimalFactoryPopulated).
func matchingCandidateFile(caps spec.Capabilities, populated map[string]*codeplug.ChannelData, edits map[string]*codeplug.ChannelData) *codeplug.Codeplug {
	var channels []codeplug.Channel
	for _, bank := range caps.Banks {
		for _, slot := range bank.Slots {
			if data, ok := edits[slot]; ok {
				channels = append(channels, codeplug.Channel{Slot: slot, Data: data})
				continue
			}
			if data, ok := populated[slot]; ok {
				channels = append(channels, codeplug.Channel{Slot: slot, Data: data})
				continue
			}
			channels = append(channels, codeplug.Channel{Slot: slot})
		}
	}
	return &codeplug.Codeplug{
		Schema:    codeplug.CurrentSchema,
		Generator: "rigprog-test",
		Radio:     codeplug.RadioInfo{Model: caps.Model, CATID: caps.CATID},
		Channels:  channels,
	}
}

// freqDigits mirrors fakeradio's 9-digit zero-padded ASCII frequency
// encoding, for asserting against fakeradio.MemState.Freq directly —
// same technique as core/clone/execute_test.go's helper of the same
// name.
func freqDigits(hz uint32) string {
	s := ""
	for i := 0; i < 9; i++ {
		s = string(rune('0'+hz%10)) + s
		hz /= 10
	}
	return s
}

// captureImage snapshots radio's current MemState for every slot caps
// lists — a pure in-memory read (fakeradio.Radio.SlotState touches no
// wire), so calling this costs nothing beyond what the write/read under
// test already paid for.
func captureImage(caps spec.Capabilities, radio *fakeradio.Radio) map[string]fakeradio.MemState {
	out := make(map[string]fakeradio.MemState)
	for _, bank := range caps.Banks {
		for _, slot := range bank.Slots {
			st, ok := radio.SlotState(slot)
			if ok {
				out[slot] = st
			}
		}
	}
	return out
}

// triggerWriter wraps a bytes.Buffer, forwarding every Write call to it
// unchanged, and additionally invokes action exactly once, SYNCHRONOUSLY
// (before Write returns), the first time a chunk contains trigger.
//
// This is the mechanism the fault/drift tests below use in place of
// fakeradio's numbered-exchange faults (FaultDisconnect(afterN),
// FaultGarbleReply(n), the technique core/clone/execute_test.go uses,
// e.g. its magic "142"/"146" exchange numbers): progressPrinter
// (progress.go) writes exactly one Write() call per formatted progress
// line, synchronously, from inside clone.Service's own call chain
// (PrepareSend's "read" phase, Execute's "verify-read"/"write"/"verify"
// phases) — so wrapping stderr with this type gives a precise,
// deterministic synchronisation point with the CLI's own narration, with
// no exchange-count arithmetic to get right (and no risk of it silently
// drifting out of sync with a future change to Open's probe count or
// ReadAll's slot count). See this task's report for why this was chosen
// over reproducing core/clone's own numbered-fault technique here.
type triggerWriter struct {
	buf     bytes.Buffer
	trigger string
	action  func()
	fired   bool
}

func (w *triggerWriter) Write(b []byte) (int, error) {
	n, err := w.buf.Write(b)
	if !w.fired && strings.Contains(string(b), w.trigger) {
		w.fired = true
		w.action()
	}
	return n, err
}

func (w *triggerWriter) String() string { return w.buf.String() }

// TestRunWrite_RoundTripAndUnchangedSlots_Interactive is this task's
// centrepiece proof, combining three of task-14 brief §2's requirements
// into a single radio-touching invocation (wall-time budget):
//
//   - the round-trip proof (a Modified + an Added entry land on the
//     radio with exactly the sent values);
//   - the plan-mandated M4 "unchanged-slot image comparison": a full
//     SlotState image captured before and after must be byte-for-byte
//     identical everywhere EXCEPT the two intended slots;
//   - the FULL interactive path (no --yes, no --firmware — isTTY=true,
//     stdin supplies "yes" to the confirmation prompt and "V01-10" to
//     the firmware prompt), proving both prompts are wired correctly
//     AND that runWrite's single shared *bufio.Reader correctly hands
//     each prompt its own line without one prompt's over-read stranding
//     the next prompt's answer (see readLine's doc comment).
func TestRunWrite_RoundTripAndUnchangedSlots_Interactive(t *testing.T) {
	radio, sess := openWriteTestSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	caps := sess.Capabilities()

	file := matchingCandidateFile(caps, minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 14_100_000, "MOD-ONE").Data,
		"010": writableChannel("010", 14_300_000, "ADDED").Data,
	})

	before := captureImage(caps, radio)

	snapshotDir := t.TempDir()
	stdin := strings.NewReader("yes\nV01-10\n")
	var stdout, stderr bytes.Buffer
	got := runWrite(testCtx(t), "FT-710", sess, snapshotDir, file, false, "", true, stdin, &stdout, &stderr)
	if got != exitSuccess {
		t.Fatalf("runWrite = %d, want exitSuccess (%d); stdout=%q stderr=%q", got, exitSuccess, stdout.String(), stderr.String())
	}

	// The interactive path was genuinely exercised (both prompts fired
	// and were answered from the SAME shared reader).
	if !strings.Contains(stderr.String(), "Type \"yes\"") {
		t.Errorf("stderr = %q, want the confirmation prompt", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Firmware version:") {
		t.Errorf("stderr = %q, want the firmware prompt", stderr.String())
	}
	if !strings.Contains(stdout.String(), "Written:        2") {
		t.Errorf("stdout = %q, want Written: 2", stdout.String())
	}

	// Round-trip proof: the two intended slots landed with exactly the
	// sent values.
	for _, tt := range []struct {
		slot   string
		freqHz uint32
		tag    string
	}{
		{"001", 14_100_000, "MOD-ONE"},
		{"010", 14_300_000, "ADDED"},
	} {
		st, ok := radio.SlotState(tt.slot)
		if !ok {
			t.Errorf("SlotState(%q): no state recorded", tt.slot)
			continue
		}
		if !st.Populated || st.Freq != freqDigits(tt.freqHz) || st.Tag != tt.tag {
			t.Errorf("SlotState(%q) = %+v, want Freq=%s Tag=%q Populated=true", tt.slot, st, freqDigits(tt.freqHz), tt.tag)
		}
	}

	// Plan-mandated M4 verification: capture the AFTER image and assert
	// byte-for-byte equality everywhere except the two intended slots.
	after := captureImage(caps, radio)
	changed := map[string]bool{"001": true, "010": true}
	if len(before) == 0 || len(after) == 0 {
		t.Fatal("captureImage returned nothing — sanity check failed, the comparison below would pass vacuously")
	}
	for slot, b := range before {
		if changed[slot] {
			continue
		}
		a, ok := after[slot]
		if !ok || a != b {
			t.Errorf("slot %q changed unexpectedly: before=%+v after=%+v (ok=%v)", slot, b, a, ok)
		}
	}
	for slot := range after {
		if _, ok := before[slot]; !ok && !changed[slot] {
			t.Errorf("slot %q appeared after the write but was absent before, and is not one of the intended changes", slot)
		}
	}
}

// TestRunWrite_VerifyMismatch_Aborts (task-14 brief §2's "verify-failure
// injection"): the SAME session used by runWrite is mutated out-of-band
// (mirroring core/clone/execute_test.go's TestExecute_VerifyReadPhase_
// BaselineDrift technique — "a second tool or a front-panel edit" — here
// timed via triggerWriter to land between the write and its own verify
// read-back, rather than before the pre-write verify-read) — so the
// write itself succeeds but its paired read-back disagrees. Execute
// aborts with a *clone.VerifyMismatchError wrapped in *clone.AbortedError;
// runWrite must render it: exit 5, the slot, the per-slot table, journal
// path, snapshot path, and the "snapshot" (never "backup") recovery
// wording.
func TestRunWrite_VerifyMismatch_Aborts(t *testing.T) {
	_, sess := openWriteTestSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	caps := sess.Capabilities()

	file := matchingCandidateFile(caps, minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 14_100_000, "MOD-ONE").Data,
	})

	target := "write 1/1 " + codeplug.DisplaySlot("001")
	stderr := &triggerWriter{trigger: target, action: func() {
		// Out-of-band mutation via the SAME session runWrite is using,
		// between the write Execute just performed and the verify
		// read-back it is about to perform — landing after WriteChannel
		// succeeded but before the paired ReadChannel, so the verify
		// itself disagrees with what was actually sent.
		if _, err := sess.WriteChannel(testCtx(t), writableChannel("001", 21_000_000, "CORRUPTED")); err != nil {
			t.Errorf("out-of-band WriteChannel (corrupting for verify-mismatch): %v", err)
		}
	}}

	snapshotDir := t.TempDir()
	var stdout bytes.Buffer
	got := runWrite(testCtx(t), "FT-710", sess, snapshotDir, file, true, "V01-10", false, strings.NewReader(""), &stdout, stderr)
	if got != exitAborted {
		t.Fatalf("runWrite = %d, want exitAborted (%d); stdout=%q stderr=%q", got, exitAborted, stdout.String(), stderr.String())
	}
	if !stderr.fired {
		t.Fatal("triggerWriter never fired — test setup assumption broken (progress line text changed?)")
	}
	out := stderr.String()
	for _, want := range []string{"M-01", "Journal:", "Snapshot:", "snapshot", "Slot results:"} {
		if !strings.Contains(out, want) {
			t.Errorf("abort output = %q, want it to contain %q", out, want)
		}
	}
	if strings.Contains(strings.ToLower(out), "backup") {
		t.Errorf("abort output = %q, want it NEVER to say \"backup\" (plan honesty rule)", out)
	}
}

// TestRunWrite_MidTransferDisconnect_Aborts (task-14 brief §2's
// "Fault-point: FaultDisconnect mid-transfer"): closes the underlying
// fakeradio.Radio partway through a 3-delta write, deterministically
// synchronised via triggerWriter (see its doc comment for why this is
// used instead of fakeradio.WithFault(FaultDisconnect(afterN)) with a
// computed exchange number). The first delta must have fully completed
// before the disconnect; the third must never have been attempted.
func TestRunWrite_MidTransferDisconnect_Aborts(t *testing.T) {
	radio, sess := openWriteTestSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	caps := sess.Capabilities()

	file := matchingCandidateFile(caps, minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 14_100_000, "MOD-ONE").Data,
		"002": writableChannel("002", 14_200_000, "ADD-TWO").Data,
		"003": writableChannel("003", 14_300_000, "ADD-THREE").Data,
	})

	target := "write 2/3 " + codeplug.DisplaySlot("002")
	stderr := &triggerWriter{trigger: target, action: func() {
		_ = radio.Close() // simulate an abrupt mid-transfer disconnect
	}}

	snapshotDir := t.TempDir()
	var stdout bytes.Buffer
	got := runWrite(testCtx(t), "FT-710", sess, snapshotDir, file, true, "V01-10", false, strings.NewReader(""), &stdout, stderr)
	if got != exitAborted {
		t.Fatalf("runWrite = %d, want exitAborted (%d); stdout=%q stderr=%q", got, exitAborted, stdout.String(), stderr.String())
	}
	if !stderr.fired {
		t.Fatal("triggerWriter never fired — test setup assumption broken (progress line text changed?)")
	}

	// "001" (the first delta) must have completed its own write+verify
	// pair before the disconnect fired on "002"'s write phase.
	if st, ok := radio.SlotState("001"); !ok || st.Tag != "MOD-ONE" {
		t.Errorf("SlotState(\"001\") = %+v, ok=%v, want the WRITTEN value — it must have completed before \"002\"'s disconnect", st, ok)
	}
	// "003" (the third delta) must never have been attempted.
	if _, ok := radio.SlotState("003"); ok {
		t.Error("SlotState(\"003\") has recorded state, want none — never attempted")
	}

	out := stderr.String()
	for _, want := range []string{"Journal:", "Snapshot:", "snapshot", "Slot results:"} {
		if !strings.Contains(out, want) {
			t.Errorf("abort output = %q, want it to contain %q", out, want)
		}
	}
}

// failingWriter's every Write call returns err without touching
// anything else — the "unwritable/full stdout" Fix 1 (adjudicated HIGH,
// Codex M4 #1) is about: a caller that cannot even render the plan
// summary must never reach Execute.
type failingWriter struct{ err error }

func (f *failingWriter) Write([]byte) (int, error) { return 0, f.err }

// TestRunWrite_FailingStdout_AbortsBeforeExecute pins Fix 1 (adjudicated
// HIGH, Codex M4 #1): writePlanSummary's write to stdout is the FIRST
// thing runWrite does after PrepareSend, before the confirmation gate or
// Execute. With --yes set (so resolveConfirmation never even runs its
// own prompt), a failing stdout must still abort the whole run — exit 1
// — before Execute ever touches the radio. Reuses captureImage
// (defined above in this file) to prove that directly: the fakeradio
// image is byte-for-byte identical before and after the aborted run.
func TestRunWrite_FailingStdout_AbortsBeforeExecute(t *testing.T) {
	radio, sess := openWriteTestSession(t, fakeradio.WithFactoryImage(minimalFactoryImage))
	caps := sess.Capabilities()

	file := matchingCandidateFile(caps, minimalFactoryPopulated(), map[string]*codeplug.ChannelData{
		"001": writableChannel("001", 14_100_000, "MOD-ONE").Data,
	})

	before := captureImage(caps, radio)
	if len(before) == 0 {
		t.Fatal("captureImage returned nothing — sanity check failed, the comparison below would pass vacuously")
	}

	snapshotDir := t.TempDir()
	fw := &failingWriter{err: errors.New("simulated broken stdout")}
	var stderr bytes.Buffer
	got := runWrite(testCtx(t), "FT-710", sess, snapshotDir, file, true, "V01-10", false, strings.NewReader(""), fw, &stderr)
	if got != exitError {
		t.Fatalf("runWrite(failing stdout, --yes) = %d, want exitError (%d); stderr=%q", got, exitError, stderr.String())
	}

	after := captureImage(caps, radio)
	for slot, b := range before {
		a, ok := after[slot]
		if !ok || a != b {
			t.Errorf("slot %q changed even though the plan summary failed to render — Execute must never have run: before=%+v after=%+v ok=%v", slot, b, a, ok)
		}
	}
}
