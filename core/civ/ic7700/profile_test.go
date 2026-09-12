// SPDX-License-Identifier: GPL-3.0-or-later

package ic7700_test

import (
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ/ic7700"
)

func TestProfile_Identity(t *testing.T) {
	p := ic7700.Profile()
	if p.Model() != "IC-7700" {
		t.Errorf("Model() = %q, want IC-7700", p.Model())
	}
	if got := p.RadioAddress(); got != 0x74 {
		t.Errorf("RadioAddress() = %#02x, want 0x74 (matrix §1 row 2 / §3.4)", got)
	}
	if got := p.NameLength(); got != 10 {
		t.Errorf("NameLength() = %d, want 10 (matrix §1 row 7)", got)
	}
	if got := p.NamePad(); got != 0x20 {
		t.Errorf("NamePad() = %#02x, want 0x20 (ASSUMED, register entry ic7700-name-pad-byte)", got)
	}
	lo, hi := p.ChannelRange()
	if lo != 1 || hi != 101 {
		t.Errorf("ChannelRange() = (%d, %d), want (1, 101) (matrix §1 row 5: 99 memories + P1/P2)", lo, hi)
	}
}

func TestProfile_AcceptsRecordLength(t *testing.T) {
	p := ic7700.Profile()
	if !p.AcceptsRecordLength(ic7700.RecordOnlyLength) {
		t.Errorf("AcceptsRecordLength(%d) = false, want true", ic7700.RecordOnlyLength)
	}
	if ic7700.RecordOnlyLength != 39 {
		t.Errorf("RecordOnlyLength = %d, want 39 (matrix §3.11)", ic7700.RecordOnlyLength)
	}
	for _, n := range []int{25, 38, 40, 45, 60} {
		if p.AcceptsRecordLength(n) {
			t.Errorf("AcceptsRecordLength(%d) = true, want false — this model has no registered sibling and documents no conditional field width (matrix §4)", n)
		}
	}
}

// TestNameCharset_IsPrintableASCIIPlusSpace observes, rather than defines,
// that the transcribed charset lands inside printable ASCII — matrix §1
// row 30's "All characters are available" reading.
func TestNameCharset_IsPrintableASCIIPlusSpace(t *testing.T) {
	if len(ic7700.NameCharset) != 95 {
		t.Errorf("len(NameCharset) = %d, want 95 (26+26+10+32+1)", len(ic7700.NameCharset))
	}
	seen := map[byte]bool{}
	for i := 0; i < len(ic7700.NameCharset); i++ {
		b := ic7700.NameCharset[i]
		if b < 0x20 || b > 0x7E {
			t.Errorf("NameCharset contains %#02x, outside printable ASCII", b)
		}
		if seen[b] {
			t.Errorf("NameCharset repeats %#02x", b)
		}
		seen[b] = true
	}
	if !strings.Contains(ic7700.NameCharset, " ") {
		t.Error("NameCharset does not contain a space (matrix §1 row 30's ASSUMED space half)")
	}
}
