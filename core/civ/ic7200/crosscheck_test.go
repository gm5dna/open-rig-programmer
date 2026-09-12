// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200_test

import (
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic7200"
)

// TestCrosscheck_ModeCodes cross-checks the codec's mode enum against
// matrix §1 row 5 / PDF p.120 (folio 11-6), "⑨ Operating mode": 00 LSB,
// 01 USB, 02 AM, 03 CW, 04 RTTY, 07 CW-R, 08 RTTY-R — seven codes, no
// FM/WFM/DV/PSK anywhere for this radio.
func TestCrosscheck_ModeCodes(t *testing.T) {
	want := map[byte]string{
		0x00: "LSB", 0x01: "USB", 0x02: "AM", 0x03: "CW", 0x04: "RTTY",
		0x07: "CW-R", 0x08: "RTTY-R",
	}
	crosscheckEnum(t, civ.FieldMode, want)
}

// TestCrosscheck_FilterCodes cross-checks matrix §1b, PDF p.120, "⑩
// Filter setting": 01 Wide, 02 Mid, 03 Narrow.
func TestCrosscheck_FilterCodes(t *testing.T) {
	crosscheckEnum(t, civ.FieldFilter, map[byte]string{0x01: "Wide", 0x02: "Mid", 0x03: "Narrow"})
}

// TestCrosscheck_DataModeCodes cross-checks matrix §1b, PDF p.120, "⑪
// Data mode setting": "1 byte data (XX)", 00 OFF, 10 ON — a full byte,
// not the 00/01 nibble every sibling model in this tier uses.
func TestCrosscheck_DataModeCodes(t *testing.T) {
	crosscheckEnum(t, civ.FieldDataMode, map[byte]string{0x00: "OFF", 0x10: "ON"})
}

func crosscheckEnum(t *testing.T, field civ.FieldID, want map[byte]string) {
	t.Helper()
	p := ic7200.Profile()
	var codec map[byte]string
	for _, sp := range p.Layouts()[0].Fields {
		if sp.Field == field {
			codec = sp.Enum
		}
	}
	if codec == nil {
		t.Fatalf("the layout maps no %s span", field)
	}
	if len(codec) != len(want) {
		t.Fatalf("%s codec has %d entries, matrix has %d: %v vs %v", field, len(codec), len(want), codec, want)
	}
	for code, name := range want {
		if codec[code] != name {
			t.Errorf("%s code %#02x = %q, matrix says %q", field, code, codec[code], name)
		}
	}
}

// TestCrosscheck_RecordArithmetic pins matrix §3.11's own addition, term
// by term, against the layout: ③ Split (1) + ④~⑧ RX frequency (5) +
// ⑨,⑩ mode+filter (2) + ⑪ data mode (1) + ❹~⓫ TX-duplicate block (8) = 17.
func TestCrosscheck_RecordArithmetic(t *testing.T) {
	const (
		split       = 1
		rxFrequency = 5
		modeFilter  = 2
		dataMode    = 1
		txDuplicate = 5 + 1 + 1 + 1 // tx_frequency + TX mode/filter/data-mode mirror
	)
	sum := split + rxFrequency + modeFilter + dataMode + txDuplicate
	if sum != 17 {
		t.Fatalf("matrix §3.11's own arithmetic sums to %d, want 17", sum)
	}
	if ic7200.RecordOnlyLength != sum {
		t.Errorf("RecordOnlyLength = %d, want %d (matrix §3.11's arithmetic, not spec.md §1's superseded 9 B figure)", ic7200.RecordOnlyLength, sum)
	}
}

// TestCrosscheck_ChannelSpace pins matrix §1 row 4 / §2's SCAN bank note:
// 199 memories (0001-0199) plus 2 scan edges (0200 P1, 0201 P2) share one
// flat two-byte selector, corroborated PDF p.84 (folio 7-2), "The
// transceiver has 201 memory channels, including 2 programmable scan
// edges."
func TestCrosscheck_ChannelSpace(t *testing.T) {
	p := ic7200.Profile()
	lo, hi := p.ChannelRange()
	const memories, scanEdges = 199, 2
	if hi-lo+1 != memories+scanEdges {
		t.Errorf("channel range %d..%d spans %d channels, want %d (199 memories + 2 scan edges, PDF p.84)", lo, hi, hi-lo+1, memories+scanEdges)
	}
}
