// SPDX-License-Identifier: GPL-3.0-or-later

package ic905_test

import (
	"bytes"
	"slices"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/civ/ic905"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// The two addresses are the only two concrete bytes this document prints
// for a frame's endpoints: PDF p.3 (folio 2), "About the data format",
// the "Controller (PC) to IC-905" diagram, cell (2) printed AC and
// labelled "Transceiver's default address", cell (3) printed E0 and
// labelled "Controller's (PC's) default address". Both MANUAL-EVIDENCED
// (matrix section 3.4).
func TestProfile_AddressesAreTheDocumentedOnes(t *testing.T) {
	p := ic905.Profile()
	if !p.Configured() {
		t.Fatal("Profile() is not Configured — a zero profile builds, parses and admits nothing")
	}
	if got := p.Model(); got != "IC-905" {
		t.Errorf("Model() = %q, want %q", got, "IC-905")
	}
	if got := p.RadioAddress(); got != 0xAC {
		t.Errorf("RadioAddress() = %#02x, want 0xac (PDF p.3, cell 2)", got)
	}
	if got := p.ControllerAddress(); got != civ.ControllerAddressDefault {
		t.Errorf("ControllerAddress() = %#02x, want %#02x (PDF p.3, cell 3)", got, civ.ControllerAddressDefault)
	}
	if got := p.NameLength(); got != 16 {
		t.Errorf("NameLength() = %d, want 16 (PDF p.19: \"53~68: Memory name setting (16 characters, fixed)\")", got)
	}
	if got := p.NamePad(); got != 0x20 {
		// ASSUMED, and its entry is ic905.name_pad_byte with lift
		// ic905-R-17 — NOT D5 entry 3 / ic905-R-16, which covers only
		// the space-inside-a-name claim. Matrix section 3.9 grades them
		// separately: "a radio could accept 0x20 inside a name and
		// still pad with 0x00."
		t.Errorf("NamePad() = %#02x, want 0x20 — ASSUMED, ic905.name_pad_byte, lift ic905-R-17", got)
	}
}

// THE BUILD LENGTH IS 64, AND THE CHOICE IS THE PLAN'S OPEN QUESTION
// OQ-1 SETTLED.
//
// civ.ProfileConfig.BuildLength is a single static int and
// BuildMemorySet emits it and nothing else, so a profile cannot pick a
// width per record. 64 is the only record shape the memory-content
// diagram draws (MANUAL-EVIDENCED, matrix section 3.11 Condition A);
// 65 is ASSUMED. Building 64 means a 10 GHz record fails CLOSED inside
// the encoder rather than sending an assumed shape to a radio nobody
// has ever written to. Spec Erratum 2's deliberate gate width is what
// makes that coherent: AllowedCommand still admits a set at EITHER
// declared length, so the 65-byte record a radio answered with can be
// validated, and — once ic905-R-06 lands — written back with no gate
// change.
func TestBuildLengthIsTheShapeTheDiagramDraws(t *testing.T) {
	p := ic905.Profile()
	if got := p.BuildRecordLength(); got != ic905.RecordLengthShort {
		t.Errorf("BuildRecordLength() = %d, want %d", got, ic905.RecordLengthShort)
	}
	if !p.AcceptsRecordLength(ic905.RecordLengthWide) {
		t.Errorf("AcceptsRecordLength(%d) = false — the gate must admit the length the radio may answer with (spec Erratum 2)", ic905.RecordLengthWide)
	}
	if p.AcceptsRecordLength(ic905.RecordLengthShort-1) || p.AcceptsRecordLength(ic905.RecordLengthWide+1) {
		t.Error("a length outside {64, 65} is accepted — the fingerprint is the accepted set (spec D3.2)")
	}
}

// The address is FOUR bytes on this model: (1),(2) memory group and
// (3),(4) memory channel, each two packed-BCD bytes (PDF p.19, folio
// 18, left legend; ic905-field-ledger.csv rows D1/"1, 2" and D1/"3, 4";
// ic905-geometry-witness.csv the same rows at byte positions 1-2 and
// 3-4). The read frame below is G's read-record vector verbatim.
//
// THIS IS THE E4 GATE, and it is deliberately keyed to BYTES rather
// than to the address form's name: E4 lands a new four-byte form beside
// the existing three-byte one, and a test that asserted a constant's
// spelling would false-pass on the wrong form or false-STOP on a
// renamed right one.
func TestProfile_ReadFrameIsTheGoldenReadRecord(t *testing.T) {
	cmd, err := ic905.Profile().BuildMemoryRead(civ.ChannelAddress{Group: 0, Channel: 1})
	if err != nil {
		t.Fatalf("BuildMemoryRead: %v", err)
	}
	want := []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x1A, 0x00, 0x00, 0x00, 0x00, 0x01, 0xFD}
	if got := cmd.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("BuildMemoryRead(g0/ch1) =\n  got  % X\n  want % X", got, want)
	}
}

// The CALL bank is group "01 00", which as a two-byte big-endian packed
// BCD group is index 100 — one past the hundred memory groups, and
// consecutive with them (PDF p.19, folio 18: "00 00 ~ 00 99: Memory
// channel group" / "01 00: Call channel group").
//
// Group carries the WIRE index, per E4's settled semantics: what the
// radio prints and sends, so 100 here and 0..99 for the memory groups.
func TestProfile_CallGroupIsAddressable(t *testing.T) {
	cmd, err := ic905.Profile().BuildMemoryRead(civ.ChannelAddress{Group: 100, Channel: 0})
	if err != nil {
		t.Fatalf("BuildMemoryRead(CALL): %v", err)
	}
	want := []byte{0xFE, 0xFE, 0xAC, 0xE0, 0x1A, 0x00, 0x01, 0x00, 0x00, 0x00, 0xFD}
	if got := cmd.Bytes(); !bytes.Equal(got, want) {
		t.Errorf("BuildMemoryRead(CALL c1) =\n  got  % X\n  want % X", got, want)
	}
}

// Both layouts, both totals, and the arithmetic that produces them.
func TestProfile_LayoutTotals(t *testing.T) {
	p := ic905.Profile()
	if got, want := p.RecordLengths(), []int{ic905.RecordLengthShort, ic905.RecordLengthWide}; !slices.Equal(got, want) {
		t.Fatalf("RecordLengths() = %v, want %v", got, want)
	}
	if p.Discriminator() != civ.DiscriminatorRecordLength {
		t.Errorf("Discriminator() = %v, want DiscriminatorRecordLength — this is the tier's only multi-length profile", p.Discriminator())
	}
	for _, n := range p.RecordLengths() {
		lay, ok := p.LayoutFor(n)
		if !ok {
			t.Fatalf("LayoutFor(%d) missing", n)
		}
		if lay.Length != n {
			t.Errorf("LayoutFor(%d).Length = %d", n, lay.Length)
		}
		if len(lay.Fixed) != n {
			t.Errorf("LayoutFor(%d).Fixed is %d bytes, want %d — every unmapped nibble must have a stated value, or a mutated record byte would re-encode identically and the gate would admit it", n, len(lay.Fixed), n)
		}
	}
}

// Ruling R4: the CALL bank's canonical slots must not collide with any
// address in MEM's 100 x 100 sparse space. This proves it over the
// WHOLE space rather than asserting it, because "they look different"
// is exactly the claim that quietly stops being true when one namespace
// grows.
func TestSlotNamespaces_CallCannotCollideWithAnySparseMEMAddress(t *testing.T) {
	mem := make(map[string]bool, 10_000)
	for g := 0; g < 100; g++ {
		for c := 0; c < 100; c++ {
			slot := spec.SparseSlot(g+1, c+1)
			if mem[slot] {
				t.Fatalf("MEM slot %q is produced by two addresses", slot)
			}
			mem[slot] = true
		}
	}
	if len(mem) != 10_000 {
		t.Fatalf("MEM space rendered %d distinct slots, want 10000", len(mem))
	}
	for n := 0; n < 12; n++ {
		call := ic905.CallSlot(n)
		if mem[call] {
			t.Errorf("CALL slot %q collides with a MEM address", call)
		}
		if _, _, ok := spec.ParseSparseSlot(call); ok {
			t.Errorf("CALL slot %q parses as a sparse MEM address — the namespaces are not structurally distinct", call)
		}
	}
	// And the inverse: no MEM slot may parse as a CALL slot.
	for slot := range mem {
		if _, ok := ic905.ParseCallSlot(slot); ok {
			t.Errorf("MEM slot %q parses as a CALL slot", slot)
		}
	}
}
