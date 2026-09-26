// SPDX-License-Identifier: GPL-3.0-or-later

package icom

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/civ"
)

// The two scan edges' wire channel numbers, and the last flat-addressed
// memory channel — shared by every owning package's addressToSlot
// wrapper below, and each package's own slotToAddress (which keeps its
// own copies of these constants, since it stays outside this helper).
const (
	scanEdgeP1Channel = 100
	scanEdgeP2Channel = 101
	lastMemoryChannel = 99
)

// AddressToSlot is the addressToSlot body six flat-addressed Icom
// packages (ic7600, ic7610, ic7700, ic7760, ic7800, ic7851 use "%03d";
// ic7410 uses "%04d") minted byte-identical copies of, apart from the
// model name in the two error strings and the memory pad width. ic7200
// (P1/P2 = 200/201) and ic705 (group-addressed) stay out: their P1/P2
// channel numbers, or their slot shape, differ.
//
// Each owning package keeps a one-line addressToSlot(a civ.ChannelAddress)
// wrapper naming its own model and format, so callers and read_test.go
// are unaffected.
func AddressToSlot(model string, a civ.ChannelAddress, format string) (string, error) {
	if a.Group != 0 {
		return "", fmt.Errorf("%s: %s carries a group index; this radio's channel selector is a flat two-byte number", model, a)
	}
	switch a.Channel {
	case scanEdgeP1Channel:
		return "P1", nil
	case scanEdgeP2Channel:
		return "P2", nil
	}
	if a.Channel < 1 || a.Channel > lastMemoryChannel {
		return "", fmt.Errorf("%s: channel %d is outside this radio's addressable space (1..99, plus 100 and 101 for the scan edges)", model, a.Channel)
	}
	return fmt.Sprintf(format, a.Channel), nil
}
