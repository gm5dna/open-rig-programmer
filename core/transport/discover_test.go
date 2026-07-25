// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"sort"
	"strings"
	"testing"

	"go.bug.st/serial/enumerator"
)

// hasHintContaining reports whether any of info's Hints contains sub
// (case-insensitive) — tests check for a HINT's presence/content, not its
// exact wording, so rankPorts's hint text can evolve without churning every
// test.
func hasHintContaining(info PortInfo, sub string) bool {
	sub = strings.ToLower(sub)
	for _, h := range info.Hints {
		if strings.Contains(strings.ToLower(h), sub) {
			return true
		}
	}
	return false
}

func findByPath(infos []PortInfo, path string) (PortInfo, bool) {
	for _, i := range infos {
		if i.Path == path {
			return i, true
		}
	}
	return PortInfo{}, false
}

func TestRankPorts_NoCandidates_EmptyNotError(t *testing.T) {
	got := rankPorts(nil)
	if len(got) != 0 {
		t.Errorf("rankPorts(nil) = %v, want empty", got)
	}
	got = rankPorts([]*enumerator.PortDetails{})
	if len(got) != 0 {
		t.Errorf("rankPorts([]) = %v, want empty", got)
	}
}

func TestRankPorts_CP2105_EnhancedAndStandardPair_Darwin(t *testing.T) {
	// A real macOS CP2105 dual-UART bridge enumerates as FOUR port nodes:
	// a cu./tty. pair for each of its two independent UARTs (Enhanced =
	// the FT-710's CAT-1 interface; Standard = the other UART, not CAT).
	ports := []*enumerator.PortDetails{
		{Name: "/dev/cu.SLAB_USBtoUART", IsUSB: true, VID: "10C4", PID: "EA70", Product: "CP2105 USB to UART Bridge Controller - Enhanced Interface"},
		{Name: "/dev/tty.SLAB_USBtoUART", IsUSB: true, VID: "10C4", PID: "EA70", Product: "CP2105 USB to UART Bridge Controller - Enhanced Interface"},
		{Name: "/dev/cu.SLAB_USBtoUART2", IsUSB: true, VID: "10C4", PID: "EA70", Product: "CP2105 USB to UART Bridge Controller - Standard Interface"},
		{Name: "/dev/tty.SLAB_USBtoUART2", IsUSB: true, VID: "10C4", PID: "EA70", Product: "CP2105 USB to UART Bridge Controller - Standard Interface"},
	}

	got := rankPorts(ports)

	// The paired tty.* nodes must be dropped entirely (never filtered to
	// zero overall, but THIS specific darwin cu/tty duplicate rule does
	// drop the redundant node).
	if _, ok := findByPath(got, "/dev/tty.SLAB_USBtoUART"); ok {
		t.Error("/dev/tty.SLAB_USBtoUART (paired with a cu. node) was not dropped")
	}
	if _, ok := findByPath(got, "/dev/tty.SLAB_USBtoUART2"); ok {
		t.Error("/dev/tty.SLAB_USBtoUART2 (paired with a cu. node) was not dropped")
	}
	if len(got) != 2 {
		t.Fatalf("rankPorts returned %d entries, want 2 (the two cu. nodes): %+v", len(got), got)
	}

	enhanced, ok := findByPath(got, "/dev/cu.SLAB_USBtoUART")
	if !ok {
		t.Fatal("/dev/cu.SLAB_USBtoUART missing from results")
	}
	standard, ok := findByPath(got, "/dev/cu.SLAB_USBtoUART2")
	if !ok {
		t.Fatal("/dev/cu.SLAB_USBtoUART2 missing from results")
	}

	if enhanced.Score <= standard.Score {
		t.Errorf("Enhanced score %d must rank above Standard score %d (CAT-1 is the Enhanced interface)", enhanced.Score, standard.Score)
	}
	if enhanced.Score <= 0 || standard.Score <= 0 {
		t.Errorf("both CP2105 interfaces should score positively overall: enhanced=%d standard=%d", enhanced.Score, standard.Score)
	}
	if !hasHintContaining(enhanced, "CP2105") {
		t.Errorf("Enhanced hints = %v, want a CP2105 VID:PID hint", enhanced.Hints)
	}
	if !hasHintContaining(enhanced, "Enhanced") {
		t.Errorf("Enhanced hints = %v, want an Enhanced-interface hint", enhanced.Hints)
	}
	if !hasHintContaining(standard, "Standard") {
		t.Errorf("Standard hints = %v, want a Standard-interface hint", standard.Hints)
	}
	if enhanced.VID != "10C4" || enhanced.PID != "EA70" {
		t.Errorf("VID/PID = %s:%s, want 10C4:EA70", enhanced.VID, enhanced.PID)
	}
}

func TestRankPorts_UnpairedTTY_NotDropped(t *testing.T) {
	// A tty.* node with NO matching cu.* node must survive — the dedup
	// rule only drops a tty.* entry when its cu.* sibling is ALSO present.
	ports := []*enumerator.PortDetails{
		{Name: "/dev/tty.lonely", IsUSB: false},
	}
	got := rankPorts(ports)
	if _, ok := findByPath(got, "/dev/tty.lonely"); !ok {
		t.Error("unpaired /dev/tty.lonely was dropped; only PAIRED tty nodes should be")
	}
}

func TestRankPorts_SingleFTDI_WeaklyPositive(t *testing.T) {
	ports := []*enumerator.PortDetails{
		{Name: "/dev/cu.usbserial-FT1", IsUSB: true, VID: "0403", PID: "6001", Product: "FT232R USB UART"},
	}
	got := rankPorts(ports)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Score <= 0 {
		t.Errorf("FTDI score = %d, want weakly positive (>0)", got[0].Score)
	}
	cp2105, _ := findByPath(rankPorts([]*enumerator.PortDetails{
		{Name: "/dev/cu.cp", IsUSB: true, VID: "10C4", PID: "EA70"},
	}), "/dev/cu.cp")
	if got[0].Score >= cp2105.Score {
		t.Errorf("FTDI score %d should be weaker than a CP2105 match %d", got[0].Score, cp2105.Score)
	}
	if !hasHintContaining(got[0], "FTDI") && !hasHintContaining(got[0], "USB-serial") {
		t.Errorf("hints = %v, want a hint naming the chip family", got[0].Hints)
	}
}

func TestRankPorts_CH340_WeaklyPositive(t *testing.T) {
	ports := []*enumerator.PortDetails{
		{Name: "/dev/cu.wchusbserial", IsUSB: true, VID: "1A86", PID: "7523"},
	}
	got := rankPorts(ports)
	if got[0].Score <= 0 {
		t.Errorf("CH340 score = %d, want weakly positive", got[0].Score)
	}
}

func TestRankPorts_CP2102Single_WeaklyPositive_NotConfusedWithCP2105(t *testing.T) {
	// Same VID as CP2105 (Silicon Labs, 10C4) but the single-UART CP2102's
	// PID (EA60) must NOT get the strong CP2105-specific bonus.
	ports := []*enumerator.PortDetails{
		{Name: "/dev/cu.cp2102", IsUSB: true, VID: "10C4", PID: "EA60"},
	}
	got := rankPorts(ports)
	cp2105 := scoreCandidate(&enumerator.PortDetails{Name: "/dev/cu.x", VID: "10C4", PID: "EA70"})
	if got[0].Score >= cp2105.Score {
		t.Errorf("CP2102 (EA60) score %d must be weaker than CP2105 (EA70) score %d", got[0].Score, cp2105.Score)
	}
	if got[0].Score <= 0 {
		t.Errorf("CP2102 score = %d, want weakly positive", got[0].Score)
	}
}

func TestRankPorts_UnknownDevice_ZeroScoreButListed(t *testing.T) {
	// A Windows-style COM port name deliberately avoids the darwin
	// "/dev/cu." prefix bonus (scoreDarwinCu), so this genuinely tests a
	// port with NO matching ranking signal at all -> Score == 0, still
	// listed. TestRankPorts_DarwinCuBonus_AppliesEvenToUnknownChips below
	// covers the /dev/cu.* case, where an otherwise-unrecognised device
	// still nets a small positive score from that one signal.
	ports := []*enumerator.PortDetails{
		{Name: "COM7", IsUSB: false},
	}
	got := rankPorts(ports)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (never filter to zero)", len(got))
	}
	if got[0].Path != "COM7" {
		t.Errorf("Path = %q, want the unknown device's path preserved", got[0].Path)
	}
	if got[0].Score != 0 {
		t.Errorf("Score = %d, want 0 (no ranking signal matched)", got[0].Score)
	}
}

func TestRankPorts_DarwinCuBonus_AppliesEvenToUnknownChips(t *testing.T) {
	// Same unrecognised device as TestRankPorts_UnknownDevice_ZeroScoreButListed,
	// but enumerated via its darwin /dev/cu.* node: it still gets a small
	// positive score from scoreDarwinCu alone, distinct from (and smaller
	// than) any chip-identity signal.
	ports := []*enumerator.PortDetails{
		{Name: "/dev/cu.Bluetooth-Incoming-Port", IsUSB: false},
	}
	got := rankPorts(ports)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].Score != scoreDarwinCu {
		t.Errorf("Score = %d, want exactly scoreDarwinCu (%d) — no other signal should apply", got[0].Score, scoreDarwinCu)
	}
	if !hasHintContaining(got[0], "cu") {
		t.Errorf("hints = %v, want a hint mentioning the darwin cu preference", got[0].Hints)
	}
}

func TestRankPorts_SortedDescendingByScore(t *testing.T) {
	ports := []*enumerator.PortDetails{
		{Name: "/dev/cu.unknown", IsUSB: false},
		{Name: "/dev/cu.cp2105", IsUSB: true, VID: "10C4", PID: "EA70", Product: "Enhanced Interface"},
		{Name: "/dev/cu.ftdi", IsUSB: true, VID: "0403", PID: "6001"},
	}
	got := rankPorts(ports)
	for i := 1; i < len(got); i++ {
		if got[i-1].Score < got[i].Score {
			t.Fatalf("results not sorted descending by score: %+v", got)
		}
	}
	if !sort.SliceIsSorted(got, func(i, j int) bool { return got[i].Score > got[j].Score }) {
		t.Errorf("got = %+v, not sorted descending", got)
	}
}

func TestRankPorts_FieldsCopiedThrough(t *testing.T) {
	ports := []*enumerator.PortDetails{
		{Name: "/dev/cu.x", IsUSB: true, VID: "10C4", PID: "EA70", SerialNumber: "SN123", Product: "Widget"},
	}
	got := rankPorts(ports)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	info := got[0]
	if info.Path != "/dev/cu.x" || info.VID != "10C4" || info.PID != "EA70" || info.USBSerial != "SN123" {
		t.Errorf("got %+v, want Path/VID/PID/USBSerial copied through from PortDetails", info)
	}
	if info.Description != "Widget" {
		t.Errorf("Description = %q, want %q (from Product)", info.Description, "Widget")
	}
}

func TestRankPorts_NilEntry_Skipped(t *testing.T) {
	ports := []*enumerator.PortDetails{
		nil,
		{Name: "/dev/cu.real", IsUSB: false},
	}
	got := rankPorts(ports)
	if len(got) != 1 || got[0].Path != "/dev/cu.real" {
		t.Errorf("got %+v, want exactly the non-nil entry", got)
	}
}
