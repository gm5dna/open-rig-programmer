// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// goldenRecord is the 25-byte record the G leg derived by hand from PDF
// p.12 and committed as core/civ/ic7760/testdata/ic7760-vectors.golden's
// `set-record-name-with-space` vector. Written out here in wire bytes
// rather than built by the codec: a fixture assembled by the encoder under
// test would agree with a wrong offset as happily as a right one.
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
