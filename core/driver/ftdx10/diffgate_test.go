// SPDX-License-Identifier: GPL-3.0-or-later

package ftdx10

import (
	"strings"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// ftdx10MemChannel is a plain MEM channel as a read of this radio leaves
// one: the six fields the combined MT form expresses carry values, and
// tag_display — which that form has NO room for — is Unavailable, the
// state this driver's read path writes on every channel.
func ftdx10MemChannel(freqHz uint64, tag string) *codeplug.ChannelData {
	return &codeplug.ChannelData{
		FreqHz:     freqHz,
		Mode:       "FM",
		CTCSS:      "OFF",
		Shift:      "SIMPLEX",
		Tag:        tag,
		CTCSSTone:  codeplug.ToneField{State: codeplug.Unavailable},
		TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
		ScanSkip:   codeplug.BoolField{State: codeplug.Unavailable},
	}
}

// TestDiff_KnownTagDisplayRefusedOnRealProfile is the profile-keyed pin
// for the case codeplug.ChannelData's TagDisplay doc names in so many
// words: this radio's combined MT form carries no display flag, so
// spec.FieldTagDisplay is the zero FieldSupport here (caps.go), and "a
// Known value arriving from a file written for a DIFFERENT radio is
// refused by the capability gate". Wave-1c review 1, finding 1, proved
// that refusal had been deleted; this test is what stops it going again.
//
// Consented capabilities, not Unverified ones: without consent the bank
// gate refuses the whole entry first ("bank MEM is read-only"), which
// would mask the per-field gate this test is about.
func TestDiff_KnownTagDisplayRefusedOnRealProfile(t *testing.T) {
	caps := spec.ConsentUnverifiedWrites(CapabilitiesUnverified())
	if fs := caps.FieldSupport(spec.BankMemory, spec.FieldTagDisplay); !fs.Unreachable() {
		t.Fatalf("FieldTagDisplay support = %+v, want the zero FieldSupport: this test's premise is that consent cannot reach a field the frame has no room for", fs)
	}

	baseline := &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Channels: []codeplug.Channel{{Slot: "001", Data: ftdx10MemChannel(145_500_000, "GB3TEST")}},
	}
	after := ftdx10MemChannel(145_500_000, "GB3TEST")
	after.TagDisplay = codeplug.BoolField{State: codeplug.Known, Value: true}
	file := &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Channels: []codeplug.Channel{{Slot: "001", Data: after}},
	}

	res, err := codeplug.Diff(baseline, file, caps)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	e := res.Entries[0]
	if e.Kind != codeplug.DiffModified {
		t.Fatalf("Kind = %q, want %q", e.Kind, codeplug.DiffModified)
	}
	if !e.Blocked {
		t.Fatal("Blocked = false, want true: this radio's frame carries no display flag, so a Known tag_display is a request it cannot honour")
	}
	if !strings.Contains(e.BlockReason, "tag_display not writable on this radio") {
		t.Errorf("BlockReason = %q, want it to name tag_display", e.BlockReason)
	}
}

// TestDiff_UnavailableTagDisplayFlowsOnRealProfile is the converse: an
// Unavailable display flag is what every read of this radio produces, it
// asks for nothing, and it must not block — neither through the per-field
// gate nor through Diff's dedicated tag-display-unknown gate, which is
// itself keyed on the frame carrying the flag at all.
func TestDiff_UnavailableTagDisplayFlowsOnRealProfile(t *testing.T) {
	caps := spec.ConsentUnverifiedWrites(CapabilitiesUnverified())
	baseline := &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Channels: []codeplug.Channel{{Slot: "001", Data: ftdx10MemChannel(145_500_000, "GB3TEST")}},
	}
	file := &codeplug.Codeplug{
		Schema:   codeplug.CurrentSchema,
		Channels: []codeplug.Channel{{Slot: "001", Data: ftdx10MemChannel(145_600_000, "GB3TEST")}},
	}

	res, err := codeplug.Diff(baseline, file, caps)
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if e := res.Entries[0]; e.Blocked {
		t.Errorf("Blocked = true (%q), want a plain frequency edit to flow", e.BlockReason)
	}
}
