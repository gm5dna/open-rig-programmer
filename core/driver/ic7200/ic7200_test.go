// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/driver"
)

// goldenRecord is the 17-byte record core/civ/ic7200/geometry_test.go's
// legalRecord() encodes: 14.250000 MHz, USB, Wide, data mode ON, the
// TX-duplicate block mirroring the RX span (matrix §3.11's NOTE).
//
//	offset  0      00            Split, UNMAPPED, OFF
//	offset  1-5    00 00 25 14 00  14 250 000 Hz, little-endian BCD
//	offset  6      01            USB
//	offset  7      01            Wide
//	offset  8      10            data mode ON (a full byte)
//	offset  9-13   00 00 25 14 00  TX frequency, same convention, mirrored
//	offset  14-16  00 00 00      TX mode/filter/data-mode mirror, UNMAPPED
var goldenRecord = []byte{
	0x00,
	0x00, 0x00, 0x25, 0x14, 0x00,
	0x01,
	0x01,
	0x10,
	0x00, 0x00, 0x25, 0x14, 0x00,
	0x00, 0x00, 0x00,
}

// idReadFrame and memReadFrame are the two probe frames this package's
// tests compare byte for byte.
var idReadFrame = []byte{0xFE, 0xFE, 0x76, 0xE0, 0x19, 0x00, 0xFD}

func memReadFrame(ch int) []byte {
	hi, lo := encodeChannel(ch)
	return []byte{0xFE, 0xFE, 0x76, 0xE0, 0x1A, 0x00, hi, lo, 0xFD}
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
		idToken: []byte{0x76},
		records: map[int][]byte{1: goldenRecord},
	}
}
