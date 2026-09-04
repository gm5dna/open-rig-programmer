// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"fmt"
	"testing"
)

// M9c-0 task 65: cat.Dialect already carries clar ClarifierPolicy{StepHz;
// MaxAbsHz}, populated by NewDialect and set on FT710 to {10, 9990} (task
// 62). This file proves the clarifier code READS that field from its
// receiver rather than the package constants clarMaxAbsHz/clarStepHz that
// used to live in memdata.go.
//
// THE CENTRAL REQUIREMENT is testing BOTH 7 Hz and 9999 Hz on the peer. 7
// Hz alone distinguishes step 1 from the FT-710's step 10, but 7 sits
// INSIDE both dialects' magnitude ranges (-9990..9990 and -9999..9999
// alike), so an implementation that moves ONLY the step to the receiver
// while leaving the magnitude bound at the FT-710's global 9990 would
// still pass a 7 Hz-only test. 9999 Hz is the load-bearing RANGE mutation:
// it exceeds the FT-710's 9990 Hz bound but sits exactly at the peer's own
// 9999 Hz maximum (also the largest magnitude the wire format's 4-digit
// field can carry at all — dialectvalidate.go's clarFieldMaxHz). Missing
// it would leave the magnitude bound unconverted and undetected.

// clarPeerConfig describes a fictional radio whose clarifier policy
// differs from the FT-710's in BOTH dimensions: StepHz 1 (FT-710: 10) and
// MaxAbsHz 9999 (FT-710: 9990). Every OTHER attribute — mode set, slot
// space, MT policy, MW write kind — is shared with the FT-710 ON PURPOSE,
// so that ClarHz is the only field able to decide any assertion in this
// file: a slot or mode mismatch could otherwise explain a rejection just
// as well as the clarifier policy could, which would prove nothing. See
// seconddialect_test.go's testDialect/noneWireDialect/peerDialect doc
// comments for why each sibling policy task (64 MT, 65 this one, 66
// MWWriteKind) holds its own differing peer in its own new file rather
// than adding one more varying attribute to that shared fixture file.
func clarPeerConfig() DialectConfig {
	return DialectConfig{
		CATID: "6001",
		ModeNames: map[Mode]string{
			ModeUSB: "USB", // the FT-710's own wire byte and name for USB
		},
		Slots: SlotSpace{
			MemoryLo: 1, MemoryHi: 99, // identical to the FT-710's own range
			NoneWire: "000",
		},
		EXAddressForm: EXAddressTriple,
		MT:            MTPolicy{Form: MTFormShort, TagMaxBytes: 12, ClearTagByte: ' ', PadByte: ' '},
		Clarifier:     ClarifierPolicy{StepHz: 1, MaxAbsHz: 9999},
		MWWriteKind:   KindMemory,
	}
}

// newClarPeerDialect builds the peer via the exported constructor, per the
// brief ("Peer dialect ... via NewDialect"), failing loudly if the fixture
// itself is invalid rather than silently handing every test a zero
// Dialect.
func newClarPeerDialect(t *testing.T) Dialect {
	t.Helper()
	d, err := NewDialect(clarPeerConfig())
	if err != nil {
		t.Fatalf("NewDialect(clarPeerConfig()): %v — the fixture itself is invalid", err)
	}
	return d
}

// clarMemoryData returns a MemoryData fixed in every field except ClarHz,
// on slot s. Modelled on framecorpus_test.go's corpusMemoryData, but
// parameterised on the clarifier value under test rather than fixed at
// zero.
func clarMemoryData(s Slot, clarHz int16) MemoryData {
	return MemoryData{
		Slot:   s,
		FreqHz: 14_250_000,
		ClarHz: clarHz,
		RxClar: false,
		TxClar: false,
		Mode:   ModeUSB,
		Kind:   KindMemory,
		CTCSS:  CTCSSOff,
		Shift:  ShiftSimplex,
	}
}

// TestClarifierPolicy_SharedSlotAndModeBuildOnBothDialects is the negative
// control every rejection below depends on: with ClarHz held at a value
// legal under BOTH dialects' policies (0), the SAME slot and mode must
// build on both the peer and the FT-710. Without this, a peer/FT-710
// disagreement over slot 003 or mode USB — not the clarifier — could
// explain every "FT-710 rejects it" assertion below, and a builder that
// rejected everything would satisfy them just as well as a correct one.
func TestClarifierPolicy_SharedSlotAndModeBuildOnBothDialects(t *testing.T) {
	peer := newClarPeerDialect(t)

	peerSlot, err := peer.MemorySlot(3)
	if err != nil {
		t.Fatalf("peer.MemorySlot(3) failed: %v", err)
	}
	ft710Slot, err := FT710.MemorySlot(3)
	if err != nil {
		t.Fatalf("FT710.MemorySlot(3) failed: %v", err)
	}
	if peerSlot.Wire() != ft710Slot.Wire() {
		t.Fatalf("fixture broken: peer slot %q != FT710 slot %q for the same ordinal", peerSlot.Wire(), ft710Slot.Wire())
	}

	md := clarMemoryData(peerSlot, 0)
	if _, err := peer.BuildMWSet(md); err != nil {
		t.Errorf("peer.BuildMWSet(ClarHz=0) failed: %v", err)
	}
	if _, err := FT710.BuildMWSet(md); err != nil {
		t.Errorf("FT710.BuildMWSet(ClarHz=0) failed: %v", err)
	}
}

// TestClarifierPolicy_PeerAcceptsWhatFT710Rejects is the task's central
// proof. For EACH of 7 Hz and 9999 Hz — see the file-level doc comment for
// why both are required — it asserts all four legs the brief names: the
// peer's builder succeeds, the parser round-trips the value, the peer's
// OWN gate admits the frame, and the FT-710 rejects it, both at the gate
// and at its own builder.
func TestClarifierPolicy_PeerAcceptsWhatFT710Rejects(t *testing.T) {
	peer := newClarPeerDialect(t)

	slot, err := peer.MemorySlot(3)
	if err != nil {
		t.Fatalf("peer.MemorySlot(3) failed: %v", err)
	}
	// Premise: the FT-710 must know this exact slot too, so ClarHz is the
	// only field left able to decide the cases below.
	if _, err := FT710.MemorySlot(3); err != nil {
		t.Fatalf("premise broken: FT710.MemorySlot(3) failed: %v", err)
	}

	for _, v := range []int16{7, 9999} {
		t.Run(fmt.Sprintf("%dHz", v), func(t *testing.T) {
			md := clarMemoryData(slot, v)

			// Leg 1: the peer's builder succeeds.
			cmd, err := peer.BuildMWSet(md)
			if err != nil {
				t.Fatalf("peer.BuildMWSet(ClarHz=%d) failed, though the peer's own ClarifierPolicy is {StepHz:1, MaxAbsHz:9999}: %v", v, err)
			}

			// Leg 2: the parser round-trips it. parseMemoryFrame is the
			// decoder shared, unchanged, between ParseMRAnswer and
			// AllowedCommand's own MW grammar check (mr.go's doc comment on
			// parseMemoryFrame) — the same helper reached by leg 3 below.
			got, err := peer.parseMemoryFrame(cmd.Bytes(), "MW")
			if err != nil {
				t.Fatalf("peer.parseMemoryFrame round-trip of its own %d Hz clarifier failed: %v", v, err)
			}
			if got.ClarHz != v {
				t.Errorf("round-tripped ClarHz = %d, want %d", got.ClarHz, v)
			}

			// Leg 3: the peer's OWN gate admits its own builder's output.
			if !peer.AllowedCommand(cmd.Bytes()) {
				t.Errorf("peer REFUSED its own MW frame %q carrying ClarHz=%d Hz — the outbound gate is not reading this dialect's own clarifier policy", cmd.Bytes(), v)
			}

			// Leg 4: the FT-710 rejects it — both at the gate (the frame
			// the peer actually sent) and independently at its own builder
			// (the same MemoryData, isolating ClarHz — not the slot or
			// mode, both shared with the peer — as the deciding field).
			if FT710.AllowedCommand(cmd.Bytes()) {
				t.Errorf("FT710 ACCEPTED %q, whose ClarHz=%d Hz is outside its OWN {StepHz:10, MaxAbsHz:9990} policy — the gate is reading a global clarifier policy, not this dialect's own", cmd.Bytes(), v)
			}
			if _, err := FT710.BuildMWSet(md); err == nil {
				t.Errorf("FT710.BuildMWSet accepted ClarHz=%d Hz, outside its own 10 Hz-step/9990 Hz policy — validateMWFields' clarifier check is reading a global rather than this dialect's own ClarifierPolicy", v)
			}
		})
	}
}

// TestClarifierPolicy_PeerEnforcesItsOwnBound is the positive control that
// stops the rejections above passing for the wrong reason: a clarifier
// check that accepted EVERY value (e.g. a bypassed or short-circuited
// validClarHz) would let the peer admit 7 Hz and 9999 Hz too, but would
// also admit anything else. This proves the peer's policy is a genuine
// bound, not "accept everything": 10000 Hz exceeds even the peer's own
// MaxAbsHz of 9999.
func TestClarifierPolicy_PeerEnforcesItsOwnBound(t *testing.T) {
	peer := newClarPeerDialect(t)

	slot, err := peer.MemorySlot(3)
	if err != nil {
		t.Fatalf("peer.MemorySlot(3) failed: %v", err)
	}

	md := clarMemoryData(slot, 10000)
	if _, err := peer.BuildMWSet(md); err == nil {
		t.Error("peer.BuildMWSet accepted ClarHz=10000, outside its own MaxAbsHz=9999 — the clarifier check is not enforcing a bound at all")
	}
}
