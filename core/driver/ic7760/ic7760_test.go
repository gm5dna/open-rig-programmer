// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"bytes"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// goldenRecord is this package's 25-byte record fixture, laid out by hand
// from the frozen geometry witness (core/civ/ic7760/testdata/
// IC-7760-geometry-witness.csv) and written out in wire bytes rather than
// built by the codec: a fixture assembled by the encoder under test would
// agree with a wrong offset as happily as a right one.
//
// IT IS NOT THE FROZEN GOLDEN VECTOR, and stage-review finding F1 is why
// that is said out loud. An earlier comment here claimed these bytes WERE
// IC-7760-vectors.golden's `set-record-name-with-space` record; they are
// not — that vector carries 14 100 000 Hz, tone/tone OFF and "ALPHA BETA".
// The frozen vector is replayed byte-for-byte where it belongs, in
// core/civ/ic7760's TestGolden. These bytes exercise the DRIVER against a
// second, differently-valued record of the same documented shape.
//
//	offset  0      00            UNMAPPED (E6): SELECT-group nibble, OFF
//	offset  1-5    00 00 25 14 00  14 250 000 Hz, little-endian BCD
//	offset  6      01            USB
//	offset  7      01            FIL1
//	offset  8      01            hi = data mode OFF (E6), lo = TONE
//	offset  9-11   00 08 85      tone TX 885 deci-Hz (88.5 Hz)
//	offset  12-14  00 10 00      tone RX 1000 deci-Hz (100.0 Hz)
//	offset  15-24  "HOME QTH01"
var goldenRecord = []byte{
	0x00,
	0x00, 0x00, 0x25, 0x14, 0x00,
	0x01,
	0x01,
	0x01,
	0x00, 0x08, 0x85,
	0x00, 0x10, 0x00,
	0x48, 0x4F, 0x4D, 0x45, 0x20, 0x51, 0x54, 0x48, 0x30, 0x31,
}

// The two probe frames this package's tests compare byte for byte, from
// the plan's ONE TABLE and from the golden vectors' own `read-record` and
// `read-transceiver-id` lines.
var (
	idReadFrame = []byte{0xFE, 0xFE, 0xB2, 0xE0, 0x19, 0x00, 0xFD}
)

func memReadFrame(ch int) []byte {
	hi, lo := encodeChannel(ch)
	return []byte{0xFE, 0xFE, 0xB2, 0xE0, 0x1A, 0x00, hi, lo, 0xFD}
}

// openWith opens a session against p, failing the test on error.
func openWith(t *testing.T, p *scriptedPort, opts ...Option) *Session {
	t.Helper()
	d := New(Simulated, opts...)
	sess, err := d.Open(t.Context(), p.Port(), driver.Identity{Port: "/dev/scripted"})
	if err != nil {
		t.Fatalf("Open: %v\ntranscript:\n  %s", err, hexFrames(p.Transcript()))
	}
	s, ok := sess.(*Session)
	if !ok {
		t.Fatalf("Open returned %T, want *Session", sess)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// occupiedRadio is the ordinary image: an ID answer and a record in
// channel 1, so the probe fingerprints on its first read.
func occupiedRadio() radioImage {
	return radioImage{
		idToken: []byte{0xB2},
		records: map[int][]byte{1: goldenRecord},
	}
}

// TestOpen_AddressMatchedIDIsRequired pins spec D3.2's opening move: what
// identifies the radio is that an ADDRESS-MATCHED 19 00 reply arrived at
// all, so a reply from another station, a broadcast, or silence must not
// open a session.
//

func TestProbeScheduleIncludesP1AndP2AfterBoundedMEMSearch(t *testing.T) {
	p := newScriptedPort(t, radioImage{
		idToken: []byte{0x42},
		records: map[int][]byte{101: goldenRecord},
	})
	s := openWith(t, p)
	report := s.OpenDiagnostics()
	if !report.Fingerprinted || report.RecordLength != 25 || report.SlotsTried != 12 {
		t.Fatalf("OpenDiagnostics = %+v, want P2 fingerprint after 12 bounded slots", report)
	}

	want := [][]byte{idReadFrame}
	for ch := 1; ch <= 10; ch++ {
		want = append(want, memReadFrame(ch))
	}
	want = append(want, memReadFrame(100), memReadFrame(101))
	got := p.Transcript()
	if len(got) != len(want) {
		t.Fatalf("probe transcript has %d frames, want %d:\n  %s", len(got), len(want), hexFrames(got))
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("probe frame %d = % X, want % X", i, got[i], want[i])
		}
	}
}

func TestProbeScheduleOpensEmptyMEMAndSCANOnAddressEvidence(t *testing.T) {
	p := newScriptedPort(t, radioImage{idToken: []byte{0x42}, records: map[int][]byte{}})
	s := openWith(t, p)
	if report := s.OpenDiagnostics(); report.Fingerprinted || report.RecordLength != 0 || report.SlotsTried != 12 {
		t.Fatalf("OpenDiagnostics = %+v, want un-fingerprinted open after all 12 bounded slots", report)
	}
}
