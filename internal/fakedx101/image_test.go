// SPDX-License-Identifier: GPL-3.0-or-later

package fakedx101

import (
	"sort"
	"testing"
	"time"
)

// TestDefaultImage_IsMinimalAndExactlyEnumerated pins the default image's whole
// membership, not a sample of it: M-01 plus the nine PMS pairs, and NOTHING
// else — no 5 MHz bank, no EMG channel, no other memory channel.
//
// The exact set matters in both directions. A fake that shipped more populated
// slots would make every "this slot is empty" assertion elsewhere a fixture
// accident, and a fake that shipped fewer would make core/driver/ftdx101's
// ordinary read path untestable against it.
func TestDefaultImage_IsMinimalAndExactlyEnumerated(t *testing.T) {
	want := []string{
		"001",
		"P1L", "P1U", "P2L", "P2U", "P3L", "P3U", "P4L", "P4U", "P5L", "P5U",
		"P6L", "P6U", "P7L", "P7U", "P8L", "P8U", "P9L", "P9U",
	}

	img := DefaultImage()
	got := make([]string, 0, len(img))
	for slot := range img {
		got = append(got, slot)
	}
	sort.Strings(got)
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("DefaultImage has %d slots %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("slot %d = %q, want %q (full set: %v)", i, got[i], want[i], got)
		}
	}

	// Every entry answers as a populated Memory-kind channel: the answer kind
	// is '1' for the PMS halves too, which is doc.go register entry 2 rather
	// than a manual fact.
	for slot, s := range img {
		if s.Kind != kindMemory {
			t.Errorf("%s: Kind = %q, want %q — register entry 2", slot, s.Kind, byte(kindMemory))
		}
		if len(s.Freq) != 9 {
			t.Errorf("%s: Freq = %q, want a 9-digit field", slot, s.Freq)
		}
		if s.Tag != "" {
			t.Errorf("%s: Tag = %q, want empty — the invented fixtures carry no tags", slot, s.Tag)
		}
	}
}

// TestDefaultImage_IsTheSameForBothModels pins the decision that there is no
// per-model image: the manual's slot legends are printed once for both radios
// (matrix §1.3, six legends, none model-qualified), so a fake FTDX101MP that
// shipped a different inventory from a fake FTDX101D would be simulating a
// distinction nothing documents.
//
// Asserted over the WIRE for the whole default membership, because that is where
// a divergence would actually be seen.
func TestDefaultImage_IsTheSameForBothModels(t *testing.T) {
	_, connD := newTestRadioFrom(t, NewD)
	_, connMP := newTestRadioFrom(t, NewMP)

	for slot := range DefaultImage() {
		gotD := exchange(t, connD, "MT"+slot+";")
		gotMP := exchange(t, connMP, "MT"+slot+";")
		if gotD != gotMP {
			t.Errorf("MT%s;: D answered %q, MP answered %q", slot, gotD, gotMP)
		}
		if gotD == "?;" {
			t.Errorf("MT%s; -> %q on both models, but the slot is in DefaultImage", slot, gotD)
		}
	}
}

// TestDefaultImage_EachCallIsIndependent pins the Image contract directly: two
// calls must not share the map.
func TestDefaultImage_EachCallIsIndependent(t *testing.T) {
	a, b := DefaultImage(), DefaultImage()

	mutated := a["001"]
	mutated.Freq = "000000001"
	a["001"] = mutated
	delete(a, "P1L")

	if got := b["001"].Freq; got != "007000000" {
		t.Errorf("mutating one DefaultImage() changed another: M-01 Freq = %q, want %q", got, "007000000")
	}
	if _, ok := b["P1L"]; !ok {
		t.Error("deleting from one DefaultImage() removed P1L from another — the two calls share a map")
	}
}

// TestTwoRadiosFromOneImageDoNotAlias is the same contract where it actually
// bites: two live radios, one image, a write to one, and nothing changed in the
// other. This is what a shared map would break in practice, and it is asserted
// over the WIRE, through the write path a caller really uses.
//
// The two radios are deliberately a D and an MP, since that is the pairing
// the model-comparing tests actually build — fakedx101_test.go's and
// ex_test.go's D-against-MP sweeps, and core/transport's EX cross-check —
// and a shared map would make one
// model's write appear in the other's inventory, which would look like a
// per-model difference rather than the aliasing bug it is.
func TestTwoRadiosFromOneImageDoNotAlias(t *testing.T) {
	var img Image = DefaultImage

	_, connA := newTestRadioFrom(t, NewD, WithFactoryImage(img))
	_, connB := newTestRadioFrom(t, NewMP, WithFactoryImage(img))

	writeFrame(t, connA, ordinaryChannel("001", mtSetKindFixed).frame())
	assertNoReply(t, connA)

	// A's M-01 is now the written channel; B's is still the image's.
	if got, want := exchange(t, connA, "MT001;"), ordinaryChannel("001", kindMemory).frame(); got != want {
		t.Errorf("radio A's M-01 after its own Set -> %q, want %q", got, want)
	}
	gotB := exchange(t, connB, "MT001;")
	if freq := gotB[5:14]; freq != "007000000" {
		t.Errorf("radio B's M-01 frequency = %q, want the image's %q — a write to A leaked into B", freq, "007000000")
	}
}

// TestWithFactoryImage_ReplacesAndWithSlotOverlays pins the two options'
// documented ordering rule, in both orders, because getting it backwards is
// silent: the image REPLACES the whole map, so a WithSlot before it is lost.
func TestWithFactoryImage_ReplacesAndWithSlotOverlays(t *testing.T) {
	empty := func() map[string]MemState { return map[string]MemState{} }

	t.Run("image then slot: the overlay survives", func(t *testing.T) {
		_, conn := newTestRadio(t, WithFactoryImage(empty), WithSlot("010", ordinaryState()))
		// The image is empty, so M-01 is gone and only the overlay answers.
		assertRejected(t, conn, "MT001;")
		if got, want := exchange(t, conn, "MT010;"), ordinaryChannel("010", kindMemory).frame(); got != want {
			t.Errorf("MT010; -> %q, want %q", got, want)
		}
	})

	t.Run("slot then image: the overlay is replaced away", func(t *testing.T) {
		_, conn := newTestRadio(t, WithSlot("010", ordinaryState()), WithFactoryImage(empty))
		assertRejected(t, conn, "MT010;")
		assertRejected(t, conn, "MT001;")
	})
}

// TestWithLatency_DelaysTheReplyWithoutChangingIt pins that the one non-image
// option this package keeps affects TIMING and nothing else. (Its interruptible
// teardown is TestClose_IsPromptDespiteAPendingLatency's subject.)
func TestWithLatency_DelaysTheReplyWithoutChangingIt(t *testing.T) {
	_, conn := newTestRadio(t, WithLatency(60*time.Millisecond))

	start := time.Now()
	if got, want := exchange(t, conn, "ID;"), "ID0681;"; got != want {
		t.Errorf("ID; -> %q, want %q", got, want)
	}
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Errorf("the reply arrived after %v, want at least the scripted 60ms (less a scheduling margin)", elapsed)
	}
}

// TestWith5xx_PopulatesASparseBankVisibleViaReads asserts the sparse set, and
// asserts the GAPS as hard as the members: the sparseness is the property
// core/driver/ftdx101's no-contiguity, no-early-termination discovery walk needs
// a fixture for (see sparseFiveMHzChannels' doc comment).
func TestWith5xx_PopulatesASparseBankVisibleViaReads(t *testing.T) {
	_, conn := newTestRadio(t, With5xx())

	for _, slot := range []string{"501", "503", "599"} {
		if got := exchange(t, conn, "MT"+slot+";"); got == "?;" {
			t.Errorf("MT%s; -> %q, want an answer — With5xx must populate it", slot, got)
		} else if got[2:5] != slot {
			t.Errorf("MT%s; answered for slot %q", slot, got[2:5])
		}
	}
	// The gaps, including the one immediately after the first channel: a walk
	// that stopped at the first "?;" would report a bank of one.
	for _, slot := range []string{"502", "504", "550", "598"} {
		assertRejected(t, conn, "MT"+slot+";")
	}
	// And EMG stays absent: the two options are independent.
	assertRejected(t, conn, "MTEMG;")
}

func TestWithEMG_PopulatesTheEmergencyChannelOnly(t *testing.T) {
	_, conn := newTestRadio(t, WithEMG())

	got := exchange(t, conn, "MTEMG;")
	if got == "?;" {
		t.Fatal("MTEMG; -> \"?;\", want an answer — WithEMG must populate it")
	}
	if slot := got[2:5]; slot != "EMG" {
		t.Errorf("MTEMG; answered for slot %q, want \"EMG\"", slot)
	}
	// 5.1675 MHz USB, the invented placeholder (doc.go register entry 14),
	// asserted on the wire bytes rather than through the state map.
	if freq := got[5:14]; freq != "005167500" {
		t.Errorf("EMG frequency field = %q, want %q", freq, "005167500")
	}
	if mode := got[21]; mode != modeUSB {
		t.Errorf("EMG mode nibble = %q, want %q (USB)", mode, byte(modeUSB))
	}
	// The 5 MHz bank stays absent.
	assertRejected(t, conn, "MT501;")
}

func TestWith5xxAndWithEMG_Compose(t *testing.T) {
	_, conn := newTestRadio(t, With5xx(), WithEMG())
	for _, slot := range []string{"501", "503", "599", "EMG"} {
		if got := exchange(t, conn, "MT"+slot+";"); got == "?;" {
			t.Errorf("MT%s; -> %q, want an answer", slot, got)
		}
	}
	// The default image's own members are untouched by either option: both
	// overlay, neither replaces.
	if got := exchange(t, conn, "MT001;"); got == "?;" {
		t.Error("MT001; -> \"?;\" — an overlay option replaced the image instead of adding to it")
	}
}

func TestFiveMHzSlotSpelling(t *testing.T) {
	// Wire numbers in, wire codes out. The 501..599 numbering itself is the
	// DIALECT's ASSUMED "SlotSpace.SixtyLo/SixtyHi" entry and is not re-derived
	// here.
	tests := []struct {
		n    int
		want string
	}{{501, "501"}, {503, "503"}, {510, "510"}, {599, "599"}}
	for _, tt := range tests {
		if got := fiveMHzSlot(tt.n); got != tt.want {
			t.Errorf("fiveMHzSlot(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
