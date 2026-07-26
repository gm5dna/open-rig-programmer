// SPDX-License-Identifier: GPL-3.0-or-later

package cat

import (
	"strings"
	"testing"
)

// kindPeerDialect is a fiction whose radio accepts a DIFFERENT P7 "kind"
// byte on every memory write than the FT-710 does.
//
// It exists because the FT-710's rule — P7 must be KindMemory on every MW
// write, for memory and PMS slots alike — is a HARDWARE finding about one
// radio (M5b write trials, 13/07/2026), not a property of the NEWCAT
// grammar. The manual does not document P7's meaning in a Set at all. It
// was nonetheless hardcoded in validateMWFields, which the outbound gate
// reaches through validMWCommand, so a second radio with a different rule
// would have had its legitimate writes refused by this program's own gate.
func kindPeerDialect(t *testing.T) Dialect {
	t.Helper()
	d, err := NewDialect(DialectConfig{
		CATID:     "4321",
		ModeNames: map[Mode]string{Mode('1'): "LSB", Mode('2'): "USB"},
		Slots: SlotSpace{
			MemoryLo: 1, MemoryHi: 99,
			PMSPairs: 9,
			NoneWire: "000",
		},
		MT:        MTPolicy{TagMaxBytes: 12, ClearTagByte: ' '},
		Clarifier: ClarifierPolicy{StepHz: 10, MaxAbsHz: 9990},
		// The whole point of this fixture.
		MWWriteKind: KindPMS,
	})
	if err != nil {
		t.Fatalf("building the kind peer: %v", err)
	}
	return d
}

func mwDataWithKind(t *testing.T, d Dialect, kind byte) MemoryData {
	t.Helper()
	slot, err := d.MemorySlot(10)
	if err != nil {
		t.Fatalf("MemorySlot(10): %v", err)
	}
	mode, err := d.ParseMode('2')
	if err != nil {
		t.Fatalf("ParseMode('2'): %v", err)
	}
	ctcss, _ := ParseCTCSSState('0')
	shift, _ := ParseShift('0')
	return MemoryData{
		Slot: slot, FreqHz: 7100000, Mode: mode,
		Kind: kind, CTCSS: ctcss, Shift: shift,
	}
}

// TestMWWriteKind_PeerAcceptsWhatFT710Rejects is the load-bearing test: the
// SAME MemoryData carrying KindPMS must be accepted by the peer whose
// policy says KindPMS, and rejected by the FT-710 whose hardware evidence
// says KindMemory.
//
// Both directions matter. A test asserting only that the peer accepts would
// pass for an implementation that accepts every Kind from everyone; a test
// asserting only that the FT-710 rejects would pass for the unchanged,
// hardcoded code.
func TestMWWriteKind_PeerAcceptsWhatFT710Rejects(t *testing.T) {
	peer := kindPeerDialect(t)

	// The peer's own policy byte, through the peer.
	peerCmd, err := peer.BuildMWSet(mwDataWithKind(t, peer, KindPMS))
	if err != nil {
		t.Fatalf("peer.BuildMWSet with its own KindPMS policy: %v — the peer refuses the kind it declares", err)
	}
	if !peer.AllowedCommand(peerCmd.Bytes()) {
		t.Errorf("the peer's own gate refused a frame its own builder produced: %q", peerCmd.Bytes())
	}

	// The FT-710 must refuse the identical kind byte.
	if _, err := FT710.BuildMWSet(mwDataWithKind(t, FT710, KindPMS)); err == nil {
		t.Error("FT710.BuildMWSet accepted KindPMS — the M5b hardware finding says the radio rejects it")
	}

	// And the FT-710's gate must refuse the peer's frame outright, so the
	// two dialects cannot launder each other's writes.
	if FT710.AllowedCommand(peerCmd.Bytes()) {
		t.Errorf("FT710's gate admitted the peer's KindPMS frame: %q", peerCmd.Bytes())
	}
}

// TestMWWriteKind_FT710StillRequiresKindMemory pins the FT-710's own
// behaviour is unchanged in BOTH directions, so the test above cannot pass
// by the receiver simply accepting anything.
func TestMWWriteKind_FT710StillRequiresKindMemory(t *testing.T) {
	if _, err := FT710.BuildMWSet(mwDataWithKind(t, FT710, KindMemory)); err != nil {
		t.Fatalf("FT710.BuildMWSet with KindMemory: %v, want success", err)
	}
	for _, k := range []byte{KindVFO, KindMemTune, KindQMB, KindUnset, KindPMS} {
		if _, err := FT710.BuildMWSet(mwDataWithKind(t, FT710, k)); err == nil {
			t.Errorf("FT710.BuildMWSet accepted Kind %#02x, want rejection", k)
		}
	}
}

// TestMWWriteKind_PeerRejectsTheFT710sKind is the mirror: the peer must
// refuse KindMemory, which is the FT-710's required value. Without this, an
// implementation reading the receiver for the ACCEPTED value while also
// still permitting KindMemory would pass everything above.
func TestMWWriteKind_PeerRejectsTheFT710sKind(t *testing.T) {
	peer := kindPeerDialect(t)
	if _, err := peer.BuildMWSet(mwDataWithKind(t, peer, KindMemory)); err == nil {
		t.Error("the peer accepted KindMemory, but its declared policy is KindPMS")
	}
}

// TestMWWriteKind_DiagnosticNamesTheDialectsOwnByte checks the ERROR TEXT
// follows the receiver too.
//
// Moving only the predicate yields correct acceptance with a false
// peer-facing message — a second radio's maintainer told "Kind must be
// KindMemory ('1')" while their dialect actually requires something else
// would reasonably conclude the codec is broken.
func TestMWWriteKind_DiagnosticNamesTheDialectsOwnByte(t *testing.T) {
	peer := kindPeerDialect(t)

	_, err := peer.BuildMWSet(mwDataWithKind(t, peer, KindMemory))
	if err == nil {
		t.Fatal("expected a rejection to inspect")
	}
	if !strings.Contains(err.Error(), "'5'") {
		t.Errorf("peer diagnostic = %q, want it to name the peer's own required byte '5'", err)
	}

	_, err = FT710.BuildMWSet(mwDataWithKind(t, FT710, KindPMS))
	if err == nil {
		t.Fatal("expected an FT-710 rejection to inspect")
	}
	if !strings.Contains(err.Error(), "'1'") {
		t.Errorf("FT710 diagnostic = %q, want it to name the FT-710's own required byte '1'", err)
	}
	// The hardware evidence must survive the rescoping, not be deleted with
	// the hardcoded value it justified.
	if !strings.Contains(err.Error(), "HW-CONFIRMED") {
		t.Errorf("FT710 diagnostic = %q, want it to keep the M5b hardware citation", err)
	}
}
