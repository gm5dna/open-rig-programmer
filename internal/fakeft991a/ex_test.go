// SPDX-License-Identifier: GPL-3.0-or-later

package fakeft991a

import (
	"strings"
	"testing"
)

// EX (MENU) behaviour, over the wire. Every expected reply here is written as a
// literal or assembled from the manual's own grammar — "EX" + three address
// digits + the raw P4 + ";" — and never by calling this package's own
// buildEXAnswer, for the reason fakeft991a_test.go's header states.
//
// THE REFUSAL CASES USE exchange RATHER THAN assertRejected, deliberately, and
// the trade is worth naming: assertRejected also proves that no SECOND frame
// follows the "?;", and it pays a 150 ms silence window for each case. That
// property is a property of the dispatch loop (fakeft991a.go's handleEvent,
// one reply per frame), not of handleEX, and it is already pinned across this
// suite's other commands. So it is pinned ONCE here — the last case of
// TestEX_OutOfInventoryAndMalformedAddressesAreRefused — and the remaining
// cases compare the answer alone, which keeps this file's contribution to the
// package's -race time at a fraction of a second rather than well over one.

// TestEX_ReadsAnswerTheInventorysOwnWidth walks a sample of the chart from both
// ends and both sides of the excluded row, and asserts the whole answer frame
// byte for byte. The widths are re-read here from the committed transcription
// (transcription-b.csv), not from EXDefaults(), so a projection that lost or
// shifted a width fails.
func TestEX_ReadsAnswerTheInventorysOwnWidth(t *testing.T) {
	_, conn := newTestRadio(t)

	for _, tc := range []struct {
		addr  string
		width int
		why   string
	}{
		{"001", 4, "the chart's first row, AGC FAST DELAY"},
		{"027", 5, "TIME ZONE, one of the four five-wide rows"},
		{"086", 1, "the row before the excluded one"},
		{"088", 1, "the row after the excluded one"},
		{"151", 8, "PRESET FREQUENCY, the chart's ONLY eight-wide row"},
		{"153", 2, "the chart's last row, WIRES DG-ID"},
	} {
		t.Run(tc.addr, func(t *testing.T) {
			want := "EX" + tc.addr + strings.Repeat("0", tc.width) + ";"
			if got := exchange(t, conn, "EX"+tc.addr+";"); got != want {
				t.Errorf("EX%s; -> %q, want %q (%s)", tc.addr, got, want, tc.why)
			}
		})
	}
}

// TestEX_Row087IsAbsentFromThisFakesInventoryToo is the fake's half of plan
// decision P18, and the reason it is its own test rather than a row in the
// table above: 087 RADIO ID is the one address the DIALECT's inventory omits
// (152 items for a 153-row chart), and the fake's inventory has to omit it for
// the SAME reason from the OTHER transcription — the chart prints no parameter
// there, so there is no field an EX frame could read.
//
// The two sides reach that conclusion independently: the dialect's from
// transcription A's raw '-' through internal/extable's ParameterlessExcluded,
// this fake's from transcription B's '?' through gen/main.go's
// parameterlessAddrs. If either side ever admitted the row, the transport
// cross-check's address-set leg would fail; this test is the local statement,
// so a reader of this package alone can see that the absence is designed.
func TestEX_Row087IsAbsentFromThisFakesInventoryToo(t *testing.T) {
	r, conn := newTestRadio(t)

	if _, ok := r.EXState("087"); ok {
		t.Error("087 is present in this fake's EX state — the chart prints no parameter for RADIO ID, so it names no field an EX frame could read or write")
	}
	if _, ok := EXDefaults()["087"]; ok {
		t.Error("EXDefaults() carries 087")
	}
	if got, want := exchange(t, conn, "EX087;"), "?;"; got != want {
		t.Errorf("EX087; -> %q, want %q", got, want)
	}
	// Its neighbours ARE present, so the absence is one row and not a hole
	// where the projection lost its place.
	for _, addr := range []string{"086", "088"} {
		if _, ok := r.EXState(addr); !ok {
			t.Errorf("%s is absent from this fake's EX state — the exclusion has taken more than the one row it declares", addr)
		}
	}
}

// TestEXDefaults_IsTheWholeInventoryAndIsCopied pins the two properties the
// transport cross-check leans on: the map is the chart's 152 projected rows,
// and every call returns an independent copy so a mutating caller cannot reach
// another caller's map or any *Radio's stored state.
func TestEXDefaults_IsTheWholeInventoryAndIsCopied(t *testing.T) {
	first := EXDefaults()
	if got, want := len(first), 152; got != want {
		t.Errorf("EXDefaults() has %d addresses, want %d — 153 chart rows less the one parameterless row", got, want)
	}
	for addr, p4 := range first {
		if len(addr) != exAddrLen {
			t.Errorf("address %q is not %d digits wide", addr, exAddrLen)
		}
		if strings.Trim(p4, "0") != "" {
			t.Errorf("%s answers %q — every default is n x '0' (doc.go's register entry THE EX MENU VALUES ARE INVENTED)", addr, p4)
		}
		if len(p4) < 1 || len(p4) > exMaxWidth {
			t.Errorf("%s answers %d bytes, outside 1-%d", addr, len(p4), exMaxWidth)
		}
	}

	first["001"] = "MUTATED"
	if second := EXDefaults(); second["001"] == "MUTATED" {
		t.Error("EXDefaults() returned a shared map — a mutating caller reached the package's own table")
	}
	r, conn := newTestRadio(t)
	if got, want := exchange(t, conn, "EX001;"), "EX0010000;"; got != want {
		t.Errorf("EX001; after mutating a previous EXDefaults() result -> %q, want %q", got, want)
	}
	if v, _ := r.EXState("001"); v != "0000" {
		t.Errorf("EXState(\"001\") = %q after a mutated copy, want %q", v, "0000")
	}
}

// TestEX_OutOfInventoryAndMalformedAddressesAreRefused is the negative control.
// Without it a fake that answered EVERY three-digit address with something
// plausible would pass the tests above completely.
//
// THE SIX-BYTE FRAME IS THIS RADIO'S OWN and the wrong-width cases are the ones
// that matter across the family: a sibling's seven- or nine-byte EX read put to
// an FT-991A must not be answered, whatever its leading digits name. A length
// check written as "at least three digits" would answer an FT-891's frame with
// an FT-991A's menu value — the wrong answer given confidently.
func TestEX_OutOfInventoryAndMalformedAddressesAreRefused(t *testing.T) {
	_, conn := newTestRadio(t)

	for _, tc := range []struct {
		frame string
		why   string
	}{
		{"EX000;", "the zero address; the chart's numbering starts at 001"},
		{"EX154;", "one past the chart's last row, 153"},
		{"EX999;", "far past the chart, and the widest three-digit form"},
		{"EX087;", "transcribed and counted, but it names no field (plan P18)"},
		{"EX01;", "two digits — a lost leading zero"},
		{"EX0101;", "the FT-891's four-digit pair address"},
		{"EX010100;", "a sibling's six-digit triple address"},
		{"EX0O1;", "three bytes, one of them not a digit"},
		{"EX;", "no address at all"},
		{"EX0011;", "an EX Set shape: a known address followed by a P4 payload"},
	} {
		t.Run(strings.TrimSuffix(tc.frame, ";"), func(t *testing.T) {
			if got, want := exchange(t, conn, tc.frame), "?;"; got != want {
				t.Errorf("%s -> %q, want %q (%s)", tc.frame, got, want, tc.why)
			}
		})
	}

	// The one case that also proves no SECOND frame follows the rejection —
	// see this file's header for why it is pinned once rather than ten times.
	assertRejected(t, conn, "EX154;")
}

// TestEX_SetsAreNotModelled states the modelling gap in the direction a reader
// would otherwise have to infer from a refusal: this radio documents EX in both
// directions (availability 155), and this fake answers READS only. An EX Set is
// refused because its body is a known address followed by a payload, which is
// simply a too-long body to handleEX — not because a real FT-991A is claimed to
// refuse one — and the stored value is untouched, so a test cannot mistake the
// refusal for a write that half-happened.
func TestEX_SetsAreNotModelled(t *testing.T) {
	r, conn := newTestRadio(t)

	before, ok := r.EXState("001")
	if !ok {
		t.Fatal("001 is absent from this fake's EX state")
	}
	if got, want := exchange(t, conn, "EX0011234;"), "?;"; got != want {
		t.Errorf("EX0011234; -> %q, want %q", got, want)
	}
	after, _ := r.EXState("001")
	if after != before {
		t.Errorf("the refused Set moved 001 from %q to %q", before, after)
	}
}

// TestEX_CommandNameIsAcceptedInEitherCase is the MANUAL FACT of this radio
// applied to EX (ft991a_layout.txt:113-114), asserted here because the EX
// handler is reached through the same dispatch fold as every other command and
// a per-command exception would be an invented strictness.
func TestEX_CommandNameIsAcceptedInEitherCase(t *testing.T) {
	_, conn := newTestRadio(t)
	for _, frame := range []string{"ex001;", "Ex001;", "eX001;"} {
		if got, want := exchange(t, conn, frame), "EX0010000;"; got != want {
			t.Errorf("%s -> %q, want %q", frame, got, want)
		}
	}
}

// --- The options ---

// TestWithEXSetting_OverlaysVerbatim pins the option's two properties: it
// overlays without validation, and it can make an address answerable that the
// generated inventory never produced — the seam that lets a test reach a wire
// behaviour the transcription does not describe WITHOUT editing the projection
// of transcription B that the cross-check depends on.
func TestWithEXSetting_OverlaysVerbatim(t *testing.T) {
	r, conn := newTestRadio(t,
		WithEXSetting("001", "1234"),
		WithEXSetting("087", "77"), // an address the inventory deliberately lacks
	)

	if got, want := exchange(t, conn, "EX001;"), "EX0011234;"; got != want {
		t.Errorf("EX001; -> %q, want %q", got, want)
	}
	if got, want := exchange(t, conn, "EX087;"), "EX08777;"; got != want {
		t.Errorf("EX087; with an overlay -> %q, want %q", got, want)
	}
	if v, _ := r.EXState("001"); v != "1234" {
		t.Errorf("EXState(\"001\") = %q, want %q", v, "1234")
	}
	// The generated table itself is untouched: the option writes to the
	// Radio's own map, which New seeded from a copy.
	if v := EXDefaults()["001"]; v != "0000" {
		t.Errorf("EXDefaults()[\"001\"] = %q after a WithEXSetting radio was built, want %q", v, "0000")
	}
}

// TestWithEXUnavailable_MakesAKnownAddressAnswerTheSameNAK pins the seam a
// settings reader's "unavailable" path needs: a KNOWN, otherwise-valid address
// forced to answer "?;". It introduces no new assumed behaviour — it removes a
// map entry, which reaches the fake's existing documented refusal (doc.go's
// register entry AN OUT-OF-INVENTORY EX ADDRESS ANSWERS "?;").
func TestWithEXUnavailable_MakesAKnownAddressAnswerTheSameNAK(t *testing.T) {
	r, conn := newTestRadio(t, WithEXUnavailable("001"))

	if _, ok := r.EXState("001"); ok {
		t.Error("001 is still in this fake's EX state after WithEXUnavailable")
	}
	if got, want := exchange(t, conn, "EX001;"), "?;"; got != want {
		t.Errorf("EX001; -> %q, want %q", got, want)
	}
	// Only that one address moved.
	if got, want := exchange(t, conn, "EX002;"), "EX0020000;"; got != want {
		t.Errorf("EX002; -> %q, want %q", got, want)
	}
}

// TestWithFactoryImage_LeavesTheMenuAlone pins the separation the two tables
// have: a factory image replaces the SLOT map and says nothing about the menu,
// so an EX read still answers the generated inventory's default.
func TestWithFactoryImage_LeavesTheMenuAlone(t *testing.T) {
	_, conn := newTestRadio(t, WithFactoryImage(func() map[string]MemState { return map[string]MemState{} }))

	if got, want := exchange(t, conn, "EX001;"), "EX0010000;"; got != want {
		t.Errorf("EX001; under an empty factory image -> %q, want %q", got, want)
	}
	// The image really was empty, or the assertion above proves nothing.
	if got, want := exchange(t, conn, "MT001;"), "?;"; got != want {
		t.Errorf("MT001; under an empty factory image -> %q, want %q", got, want)
	}
}

// TestBuildEXDefaults_PanicsOnAMalformedWidthToken pins the expander's own
// refusal. exItems is a GENERATED package-level table, so a token outside
// '1'..'8' is a defect in the generator or a hand-edit of its output — a
// programming error to catch at init, never a runtime input.
//
// IT IS PROVED ON A LOCAL TABLE, not by mutating the generated one: the panic
// must be reachable and demonstrably reached, and the real table must stay the
// one the rest of the suite runs against.
func TestBuildEXDefaults_PanicsOnAMalformedWidthToken(t *testing.T) {
	for _, tok := range []byte{'0', '9', 'T'} {
		t.Run(string(tok), func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("expandEXItems accepted the width token %q; want a panic", tok)
				}
			}()
			expandEXItems([]exItem{{addr: "001", width: tok}})
		})
	}
	// And the whole legal alphabet expands, so the guard above is a guard and
	// not a blanket refusal.
	for n := 1; n <= exMaxWidth; n++ {
		out := expandEXItems([]exItem{{addr: "001", width: byte('0' + n)}})
		if got := out["001"]; len(got) != n {
			t.Errorf("width token %q expanded to %d bytes (%q), want %d", byte('0'+n), len(got), got, n)
		}
	}
}
