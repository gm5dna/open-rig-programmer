// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/cat"
	"github.com/gm5dna/open-rig-programmer/internal/fakeradio"
)

// This file is the cross-check the whole two-independent-transcriptions
// design exists for: core/cat's EXItems (transcribed from the manual's
// Table 2 by task 26) and internal/fakeradio's exGroups/EXDefaults
// (independently transcribed from the SAME manual pages by task 29,
// deliberately without consulting core/cat — see fakeradio/doc.go, THE
// HARD RULE) must agree on the address set and each address's field
// width. It lives here, in core/transport, because this is the existing
// test home that already imports both core/cat and internal/fakeradio
// (see engine_test.go) — fakeradio's own tests stay core-free.
//
// A failure in either of the first two tests below means one of the two
// transcriptions mis-read the manual — that IS the test doing its job.
// Per the task-30 brief: report NEEDS_CONTEXT with the exact diff: do NOT
// "fix" either table without a documented decision about which one was
// wrong, or whether the manual itself is wrong — as hardware showed it was
// for TONE FREQ's width at M8c, recorded in
// core/cat/table2-corrections.csv).

// TestEXInventoryCrossCheck_AddressSetsIdentical compares the set of
// core/cat's FT710 dialect (cat.FT710.EXItems()) wire addresses against the key set of
// fakeradio.EXDefaults(). On any disagreement it reports BOTH diff
// directions, so a NEEDS_CONTEXT report can quote the exact addresses
// without needing to re-derive the diff.
func TestEXInventoryCrossCheck_AddressSetsIdentical(t *testing.T) {
	catAddrs := make(map[string]bool)
	for _, item := range cat.FT710.EXItems() {
		catAddrs[item.Addr.Wire()] = true
	}
	fakeAddrs := fakeradio.EXDefaults()

	var inCatNotFake, inFakeNotCat []string
	for addr := range catAddrs {
		if _, ok := fakeAddrs[addr]; !ok {
			inCatNotFake = append(inCatNotFake, addr)
		}
	}
	for addr := range fakeAddrs {
		if !catAddrs[addr] {
			inFakeNotCat = append(inFakeNotCat, addr)
		}
	}
	sort.Strings(inCatNotFake)
	sort.Strings(inFakeNotCat)

	if len(inCatNotFake) > 0 || len(inFakeNotCat) > 0 {
		t.Errorf("EX address inventories disagree between the two independent Table 2 transcriptions (core/cat has %d addresses, fakeradio has %d):\n"+
			"  in cat.FT710.EXItems() but NOT in fakeradio.EXDefaults() (%d): %v\n"+
			"  in fakeradio.EXDefaults() but NOT in cat.FT710.EXItems() (%d): %v\n"+
			"This is a genuine cross-check finding, not a bug in this test: do not modify either table to make it pass — report NEEDS_CONTEXT with this diff.",
			len(catAddrs), len(fakeAddrs), len(inCatNotFake), inCatNotFake, len(inFakeNotCat), inFakeNotCat)
	}
}

// TestEXInventoryCrossCheck_WidthsAgree checks, for every address present
// in BOTH inventories, that fakeradio's default raw P4 length equals
// core/cat's Digits column (12 for the six Text items, per EXItem's doc
// comment). Addresses missing from one side entirely are already reported
// by TestEXInventoryCrossCheck_AddressSetsIdentical and are skipped here
// to avoid a duplicate, less specific failure.
func TestEXInventoryCrossCheck_WidthsAgree(t *testing.T) {
	fakeAddrs := fakeradio.EXDefaults()

	var mismatches []string
	checked := 0
	for _, item := range cat.FT710.EXItems() {
		addr := item.Addr.Wire()
		p4, ok := fakeAddrs[addr]
		if !ok {
			continue // reported by TestEXInventoryCrossCheck_AddressSetsIdentical
		}
		checked++
		if len(p4) != item.Digits {
			mismatches = append(mismatches, fmt.Sprintf("%s (%s / %s): cat.Digits=%d, fakeradio default len=%d (%q)",
				addr, item.P1Label, item.Name, item.Digits, len(p4), p4))
		}
	}
	if checked == 0 {
		t.Fatal("no address present in both inventories — TestEXInventoryCrossCheck_AddressSetsIdentical should already have failed")
	}
	if len(mismatches) > 0 {
		t.Errorf("EX field-width disagreement between core/cat and fakeradio (%d of %d shared addresses):\n%s\n"+
			"This is a genuine cross-check finding, not a bug in this test: do not modify either table to make it pass — report NEEDS_CONTEXT with this diff.",
			len(mismatches), checked, strings.Join(mismatches, "\n"))
	}
}

// exFrameReader reads one complete ';'-terminated frame off port,
// reassembling with a cat.FrameAccumulator exactly as a real transport
// consumer would (see core/cat/accumulator.go) — deliberately NOT
// Engine.Do: the brief's design constraint is that this 296-address
// round trip bypasses Engine entirely (direct Port() write + read) so it
// doesn't pay ~6s of Settle pacing for nothing; Engine's own correlation
// behaviour is covered by the targeted adversarial tests in
// engine_ex_test.go.
type exFrameReader struct {
	t    *testing.T
	port interface {
		Read([]byte) (int, error)
		SetReadDeadline(time.Time) error
	}
	acc *cat.FrameAccumulator
	buf []byte
}

func newEXFrameReader(t *testing.T, r *fakeradio.Radio) *exFrameReader {
	t.Helper()
	port := r.Port()
	deadliner, ok := port.(interface {
		Read([]byte) (int, error)
		SetReadDeadline(time.Time) error
	})
	if !ok {
		t.Fatalf("fakeradio Radio.Port() (%T) does not support SetReadDeadline", port)
	}
	return &exFrameReader{
		t:    t,
		port: deadliner,
		acc:  cat.NewFrameAccumulator(0), // DefaultMaxFrame: comfortably above exAnswerMaxLen (21)
		buf:  make([]byte, 256),
	}
}

// readOneFrame blocks (up to a generous per-frame deadline) until exactly
// one complete frame has been reassembled, returning it.
func (r *exFrameReader) readOneFrame() []byte {
	r.t.Helper()
	for {
		if err := r.port.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			r.t.Fatalf("SetReadDeadline: unexpected error: %v", err)
		}
		n, rerr := r.port.Read(r.buf)
		if n > 0 {
			frames, ferr := r.acc.Push(r.buf[:n])
			if ferr != nil {
				r.t.Fatalf("frame accumulator error: %v", ferr)
			}
			if len(frames) > 0 {
				return frames[0]
			}
		}
		if rerr != nil {
			r.t.Fatalf("Read: unexpected error waiting for a frame: %v", rerr)
		}
	}
}

// TestEXFakeradioRoundTrip_All296RawPort exhaustively reads every one of
// the 296 Table 2 addresses (per cat.FT710.EXItems, sorted by
// (P1,P2,P3)), one at a time, directly against fakeradio's Port() —
// bypassing Engine.Do entirely (see exFrameReader's doc comment). Each
// answer must echo the address it was asked for and carry the fake's own
// RUNTIME default raw P4 for that address (fakeradio.EXRuntimeDefaults —
// its manual transcription with the M8c hardware observations overlaid,
// which is what a *Radio actually answers). This is the exhaustive
// complement to engine_ex_test.go's targeted adversarial tests: it proves ordinary
// correlation holds over the WHOLE inventory, not just the handful of
// addresses those tests exercise.
func TestEXFakeradioRoundTrip_All296RawPort(t *testing.T) {
	r := fakeradio.New()
	t.Cleanup(func() { _ = r.Close() })

	fakeDefaults := fakeradio.EXRuntimeDefaults()
	items := cat.FT710.EXItems() // sorted by (P1,P2,P3); exactly 296 per EXItems' doc comment
	if len(items) != 296 {
		t.Fatalf("cat.FT710.EXItems() returned %d items, want 296", len(items))
	}

	reader := newEXFrameReader(t, r)

	for _, item := range items {
		cmd, err := cat.FT710.BuildEXRead(item.Addr)
		if err != nil {
			t.Fatalf("BuildEXRead(%v): unexpected error: %v", item.Addr, err)
		}
		if _, err := r.Port().Write(cmd.Bytes()); err != nil {
			t.Fatalf("Write(%v %q): unexpected error: %v", item.Addr, cmd.Bytes(), err)
		}

		frame := reader.readOneFrame()

		gotAddr, gotRaw, err := cat.FT710.ParseEXAnswer(frame)
		if err != nil {
			t.Fatalf("ParseEXAnswer(%q) for %v: unexpected error: %v", frame, item.Addr, err)
		}
		if gotAddr != item.Addr {
			t.Errorf("%v: answer echoed address %v, want %v", item.Addr, gotAddr, item.Addr)
		}
		wantRaw, ok := fakeDefaults[item.Addr.Wire()]
		if !ok {
			t.Errorf("%v: address answered over the wire but absent from fakeradio.EXRuntimeDefaults() — inconsistent with handleEX's own membership check", item.Addr)
			continue
		}
		if gotRaw != wantRaw {
			t.Errorf("%v: raw P4 = %q, want %q (the fake's runtime default)", item.Addr, gotRaw, wantRaw)
		}
	}
}

// TestEXCrossCheck_FakeRuntimeMatchesObservedReads is the third leg of the
// cross-check triangle. The two tests at the top of this file compare the
// project's two INDEPENDENT MANUAL transcriptions with each other, which
// only stays meaningful while neither has been corrected from hardware;
// this one instead compares the fake's RUNTIME answers
// (fakeradio.EXRuntimeDefaults) against the M8c hardware READ observations
// core/cat carries (EXItem.ObservedReadWidth/ObservedReadShape, from
// table2-observed.csv).
//
// So a fake that drifts from what the radio actually answered fails HERE,
// while the manual-vs-manual comparison stays undisturbed — which is why
// fakeradio's hardware corrections live in its own exHardwareOverrides
// table rather than being folded into exGroups.
//
// Scope: the observations are two successive sweeps of one UK FT-710,
// firmware V01-12, in one configuration, on 24/07/2026, in the READ
// direction only.
func TestEXCrossCheck_FakeRuntimeMatchesObservedReads(t *testing.T) {
	runtime := fakeradio.EXRuntimeDefaults()

	var mismatches []string
	checked := 0
	for _, item := range cat.FT710.EXItems() {
		addr := item.Addr.Wire()
		p4, ok := runtime[addr]
		if !ok {
			continue // reported by TestEXInventoryCrossCheck_AddressSetsIdentical
		}
		checked++
		if len(p4) != item.ObservedReadWidth {
			mismatches = append(mismatches, fmt.Sprintf("%s (%s / %s): fake runtime P4 is %d bytes, M8c observed %d",
				addr, item.P1Label, item.Name, len(p4), item.ObservedReadWidth))
		}
		signed := strings.HasPrefix(p4, "+") || strings.HasPrefix(p4, "-")
		if want := item.ObservedReadShape == "signed"; signed != want {
			mismatches = append(mismatches, fmt.Sprintf("%s (%s / %s): fake runtime signed=%v, M8c observed shape %q",
				addr, item.P1Label, item.Name, signed, item.ObservedReadShape))
		}
	}
	if checked == 0 {
		t.Fatal("no address present in both tables — TestEXInventoryCrossCheck_AddressSetsIdentical should already have failed")
	}
	if len(mismatches) > 0 {
		t.Errorf("fakeradio's runtime answers disagree with the M8c hardware observations (%d findings across %d addresses):\n%s\n"+
			"Fix the fake's exHardwareOverrides (or re-derive table2-observed.csv from the capture) — never by editing either manual transcription.",
			len(mismatches), checked, strings.Join(mismatches, "\n"))
	}
}
