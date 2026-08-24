// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700_test

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic9700"
)

func TestProfileConstructionInvariants(t *testing.T) {
	p := ic9700.Profile()
	if !p.Configured() {
		t.Fatal("profile is not configured")
	}
	if got, want := p.Model(), "IC-9700"; got != want {
		t.Errorf("Model() = %q, want %q", got, want)
	}
	if got, want := p.RadioAddress(), byte(0xA2); got != want {
		t.Errorf("RadioAddress() = %#02x, want %#02x", got, want)
	}
	if got, want := p.ControllerAddress(), byte(0xE0); got != want {
		t.Errorf("ControllerAddress() = %#02x, want %#02x", got, want)
	}
	if got, want := p.RecordLengths(), []int{ic9700.RecordLength}; !reflect.DeepEqual(got, want) {
		t.Errorf("RecordLengths() = %v, want %v (RECORD-ONLY; the wire shows %d)",
			got, want, ic9700.DataAreaLength)
	}
	if got, want := p.BuildRecordLength(), 111; got != want {
		t.Errorf("BuildRecordLength() = %d, want %d", got, want)
	}
	if got, want := p.Discriminator(), civ.DiscriminatorSingleLength; got != want {
		t.Errorf("Discriminator() = %v, want %v", got, want)
	}
	if got, want := p.AddressForm(), civ.AddressFormBandChannel; got != want {
		t.Errorf("AddressForm() = %v, want %v", got, want)
	}
	// E4: Group carries the WIRE index, so this profile's bands are 1..3.
	// The landed accessors are GroupBase() and Groups(); there is no
	// GroupRange().
	if got, want := p.GroupBase(), 1; got != want {
		t.Errorf("GroupBase() = %d, want %d — the legend prints 01/02/03 and admits no 00", got, want)
	}
	if got, want := p.Groups(), 3; got != want {
		t.Errorf("Groups() = %d, want %d", got, want)
	}
	if got, want := p.NameLength(), 16; got != want {
		t.Errorf("NameLength() = %d, want %d", got, want)
	}
	if got, want := p.NamePad(), byte(0x20); got != want {
		t.Errorf("NamePad() = %#02x, want %#02x", got, want)
	}
	lo, hi := p.ChannelRange()
	if lo != 1 || hi != 107 {
		t.Errorf("ChannelRange() = %d..%d, want 1..107", lo, hi)
	}
}

func TestBothLengthNumbersAreStatedAndDiffer(t *testing.T) {
	// spec Erratum 1: the profile carries the RECORD-ONLY length and the
	// wire carries the data-area length. Pinning only one is how they get
	// mixed up.
	if ic9700.RecordLength+ic9700.AddressBytes != ic9700.DataAreaLength {
		t.Fatalf("%d + %d != %d", ic9700.RecordLength, ic9700.AddressBytes, ic9700.DataAreaLength)
	}
	if ic9700.RecordLength == ic9700.DataAreaLength {
		t.Fatal("the two lengths must differ, or the convention is not being tested")
	}
}

func TestNameCharsetIsEveryPrintableASCII(t *testing.T) {
	p := ic9700.Profile()
	got := p.NameCharset()
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	var want []byte
	for b := byte(0x20); b <= 0x7E; b++ {
		want = append(want, b)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("NameCharset() has %d bytes, want the 95 printable ASCII", len(got))
	}
}

func TestFixedTemplateIsTheGoldensUnmappedState(t *testing.T) {
	// Fable finding 1 / R15: the template is derived from the frozen
	// golden, never assumed blank. If this fails, arbitrate against the
	// PDF — do not edit the vector.
	want := goldenRecordBytes(t) // record-only 111 bytes of the set vector
	got := ic9700.FixedTemplateForTest()
	for _, r := range []struct {
		name   string
		lo, hi int
	}{
		{"⑭", 10, 11}, {"㉔", 20, 21}, {"UR", 24, 32}, {"R1", 32, 40}, {"R2", 40, 48},
		{"dup ⑭", 57, 58}, {"dup ㉔", 67, 68}, {"dup UR", 71, 79},
		{"dup R1", 79, 87}, {"dup R2", 87, 95},
	} {
		if !bytes.Equal(got[r.lo:r.hi], want[r.lo:r.hi]) {
			t.Errorf("%s: template % X, golden % X", r.name, got[r.lo:r.hi], want[r.lo:r.hi])
		}
	}
}

// goldenVectors reads testdata/ic9700-vectors.golden — one
// `name<TAB>space-separated hex frame` per line — into an ordered slice.
//
// It is the ONE reader for the frozen vector file: Task 7's replays and
// this task's template check must agree byte for byte about what the file
// says, and two readers could not be made to.
//
// A MISSING testdata/ IS A FAILURE, NOT A SKIP. The frozen evidence is
// tracked beside this file and freeze_test.go hashes every byte of it, so
// there is no legitimate state in which it is absent — and a reader who
// has deleted it should be told, not quietly told nothing.
type goldenVector struct {
	name  string
	frame []byte
}

func goldenVectors(t *testing.T) []goldenVector {
	t.Helper()
	path := filepath.Join("testdata", "ic9700-vectors.golden")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out []goldenVector
	for i, line := range strings.Split(string(raw), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		name, hexBytes, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("%s line %d has no TAB: %q", path, i+1, line)
		}
		frame, err := hex.DecodeString(strings.ReplaceAll(strings.TrimSpace(hexBytes), " ", ""))
		if err != nil {
			t.Fatalf("%s line %d (%s): %v", path, i+1, name, err)
		}
		out = append(out, goldenVector{name: name, frame: frame})
	}
	return out
}

// goldenRecordBytes returns the RECORD-ONLY 111 bytes of the
// set-record-name-with-space vector: the 121-byte frame less its six-byte
// `FE FE A2 E0 1A 00` header, its three address bytes and its terminator.
func goldenRecordBytes(t *testing.T) []byte {
	t.Helper()
	const want = "set-record-name-with-space"
	for _, v := range goldenVectors(t) {
		if v.name != want {
			continue
		}
		const header = 6 // FE FE <to> <from> 1A 00
		rec := v.frame[header+ic9700.AddressBytes : len(v.frame)-1]
		if len(rec) != ic9700.RecordLength {
			t.Fatalf("%s carries a %d-byte record, want %d", want, len(rec), ic9700.RecordLength)
		}
		return rec
	}
	t.Fatalf("the frozen vector file has no %q", want)
	return nil
}
