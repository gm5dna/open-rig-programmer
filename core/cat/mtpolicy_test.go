// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import "testing"

// mtPeerConfig is validBaselineConfig (dialectvalidate_test.go) with its MT
// policy overridden to TagMaxBytes: 6, ClearTagByte: '-' — narrower than
// the FT-710's 12/' ', and with a DIFFERENT clear byte, so every assertion
// in this file is blind to a helper still reading mt.go's old package
// constants (mtTagMaxBytes = 12, mtClearTag = 12 spaces) rather than this
// dialect's own receiver field. Built via NewDialect, in-package, per the
// task brief.
func mtPeerConfig() DialectConfig {
	cfg := validBaselineConfig()
	cfg.MT = MTPolicy{Form: MTFormShort, TagMaxBytes: 6, ClearTagByte: '-'}
	return cfg
}

// mustMTPeer constructs the peer dialect above, failing the test loudly if
// the config is somehow invalid — every assertion in this file depends on
// construction succeeding, so a silent zero Dialect would make them all
// pass vacuously (AllowedCommand refuses everything on an unconfigured
// dialect).
func mustMTPeer(t *testing.T) Dialect {
	t.Helper()
	d, err := NewDialect(mtPeerConfig())
	if err != nil {
		t.Fatalf("NewDialect(mtPeerConfig()): %v", err)
	}
	return d
}

// peerMemorySlot is validBaselineConfig's own memory range (10-40), used
// throughout this file so every case has a slot the peer's own mtSlotValid
// accepts.
func peerMemorySlot(t *testing.T, d Dialect, n int) Slot {
	t.Helper()
	s, err := d.MemorySlot(n)
	if err != nil {
		t.Fatalf("MemorySlot(%d) failed on the peer's own range (baseline config is 10-40): %v", n, err)
	}
	return s
}

// TestMTPolicy_PeerAcceptsSixRejectsSeven is the task brief's opening
// assertion: the peer's own TagMaxBytes (6) governs BuildMTSet, not the
// FT-710's 12. The 6-byte accept and 7-byte reject are paired with an
// FT-710 positive control at ITS OWN (wider) boundary, 12 bytes — which
// would fail if TagMaxBytes were ever hardwired to the narrower of the two
// dialects instead of read per-receiver.
func TestMTPolicy_PeerAcceptsSixRejectsSeven(t *testing.T) {
	peer := mustMTPeer(t)
	slot := peerMemorySlot(t, peer, 10)

	if _, err := peer.BuildMTSet(slot, true, "ABCDEF"); err != nil { // 6 bytes: the peer's own max
		t.Errorf("peer.BuildMTSet(6-byte tag) failed, though 6 is its own TagMaxBytes: %v", err)
	}
	if _, err := peer.BuildMTSet(slot, true, "ABCDEFG"); err == nil { // 7 bytes: one past
		t.Error("peer.BuildMTSet(7-byte tag) succeeded, though its TagMaxBytes is 6 — this is reading the FT-710's 12-byte bound")
	}

	ftSlot, err := FT710.MemorySlot(1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FT710.BuildMTSet(ftSlot, true, "ABCDEFGHIJKL"); err != nil { // 12 bytes: FT-710's own max
		t.Errorf("FT710.BuildMTSet(12-byte tag) failed, though 12 is its own TagMaxBytes: %v", err)
	}
}

// TestMTPolicy_EmptyTagClearsToSixDashes is the task brief's second
// assertion, checked at the WIRE-BYTE level rather than only by round-trip
// semantics: a round trip alone cannot distinguish "the peer's own clear
// form" from "the FT-710's 12-space clear form, which happens to still
// decode to empty because the old space-only trim eats it too" — exactly
// the gap TestMTPolicy_RoundTripSurvivesEmptyTag cannot see on its own.
func TestMTPolicy_EmptyTagClearsToSixDashes(t *testing.T) {
	peer := mustMTPeer(t)
	slot := peerMemorySlot(t, peer, 10)

	cmd, err := peer.BuildMTSet(slot, true, "")
	if err != nil {
		t.Fatalf("peer.BuildMTSet(empty tag): unexpected error: %v", err)
	}
	got := string(cmd.Bytes())
	want := "MT0101------;" // "MT" + "010" + '1' + six '-' + ';'
	if got != want {
		t.Errorf("peer.BuildMTSet(empty tag) = %q, want %q (six '-' bytes, this dialect's own clear form)", got, want)
	}
}

// TestMTPolicy_GateHonoursPeerTagLength is the outbound-gate half of the
// brief's point 4: AllowedCommand -> validMTCommand reaches validMTTag, and
// "the receiver matters" only if the GATE, not merely BuildMTSet, refuses a
// forged frame whose tag exceeds THIS dialect's own TagMaxBytes. A 7-byte
// tag is inside the FT-710's 12-byte bound and outside the peer's 6-byte
// one, so this frame is exactly what a gate still consulting a package
// constant (or the FT-710's own receiver) would wrongly admit.
func TestMTPolicy_GateHonoursPeerTagLength(t *testing.T) {
	peer := mustMTPeer(t)
	forged := []byte("MT0101ABCDEFG;") // slot 010, display on, 7-byte tag

	if peer.AllowedCommand(forged) {
		t.Errorf("peer.AllowedCommand(%q) = true, want false — its TagMaxBytes is 6, so a 7-byte tag must be refused at the gate", forged)
	}

	// Positive control: the SAME shape, at the peer's own 6-byte maximum,
	// must be admitted — otherwise the rejection above could just as well
	// be a gate that refuses every MT Set frame.
	withinBound := []byte("MT0101ABCDEF;") // 6-byte tag
	if !peer.AllowedCommand(withinBound) {
		t.Errorf("peer.AllowedCommand(%q) = false, want true — its own 6-byte tag is within TagMaxBytes", withinBound)
	}

	// And the FT-710 gate must still admit the 7-byte tag: it is well
	// inside ITS OWN 12-byte bound, so this pins the FT-710 side of the
	// receiver split rather than letting a "reject everything" gate
	// satisfy the peer assertion above by accident.
	ftForged := []byte("MT0011ABCDEFG;")
	if !FT710.AllowedCommand(ftForged) {
		t.Errorf("FT710.AllowedCommand(%q) = false, want true — a 7-byte tag is well within its own 12-byte TagMaxBytes", ftForged)
	}
}

// TestMTPolicy_RoundTripSurvivesEmptyTag is the brief's point 6: build ->
// gate -> parse, for BOTH the peer and the FT-710, asserting an empty tag
// survives as empty. This is the direct evidence for "MOVE BOTH
// DIRECTIONS" (mt.go's clear-form decode must recognise the RECEIVER's own
// ClearTagByte/TagMaxBytes, not just spaces) — without it, the peer's
// six-dash clear form would parse back as "------" rather than "".
func TestMTPolicy_RoundTripSurvivesEmptyTag(t *testing.T) {
	roundTrip := func(t *testing.T, name string, d Dialect, slot Slot) {
		t.Helper()
		cmd, err := d.BuildMTSet(slot, true, "")
		if err != nil {
			t.Fatalf("%s: BuildMTSet(empty tag): unexpected error: %v", name, err)
		}
		built := cmd.Bytes()

		if !d.AllowedCommand(built) {
			t.Fatalf("%s: AllowedCommand(%q) = false, want true — this dialect's own gate must admit its own builder's output", name, built)
		}

		_, _, tag, err := d.ParseMTAnswer(built)
		if err != nil {
			t.Fatalf("%s: ParseMTAnswer(%q): unexpected error: %v", name, built, err)
		}
		if tag != "" {
			t.Errorf("%s: round trip of an empty tag = %q, want \"\" (built %q)", name, tag, built)
		}
	}

	peer := mustMTPeer(t)
	roundTrip(t, "peer", peer, peerMemorySlot(t, peer, 10))

	ftSlot, err := FT710.MemorySlot(1)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip(t, "FT710", FT710, ftSlot)
}

// TestMTPolicy_ClearFormDecodeIsNotJustSpaces is TestMTPolicy_
// RoundTripSurvivesEmptyTag's more direct sibling: it feeds ParseMTAnswer
// the peer's clear form as a literal wire frame, rather than one produced
// by BuildMTSet, so a decode still hardwired to strings.TrimRight(tag, " ")
// is caught even if some future BuildMTSet change made the build side
// coincidentally agree with the decode side.
func TestMTPolicy_ClearFormDecodeIsNotJustSpaces(t *testing.T) {
	peer := mustMTPeer(t)
	frame := []byte("MT0100------;") // six '-' bytes: the peer's own clear form
	_, _, tag, err := peer.ParseMTAnswer(frame)
	if err != nil {
		t.Fatalf("peer.ParseMTAnswer(%q): unexpected error: %v", frame, err)
	}
	if tag != "" {
		t.Errorf("peer.ParseMTAnswer(%q) tag = %q, want \"\" — clear-form decoding is not receiver-aware", frame, tag)
	}
}
