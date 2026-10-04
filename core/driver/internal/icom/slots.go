// SPDX-License-Identifier: GPL-3.0-or-later

package icom

import (
	"fmt"
	"strconv"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// The two scan edges' wire channel numbers, and the last flat-addressed
// memory channel — shared by AddressToSlot and SlotToAddress below.
const (
	scanEdgeP1Channel = 100
	scanEdgeP2Channel = 101
	lastMemoryChannel = 99
)

// AddressToSlot is the addressToSlot body seven flat-addressed Icom
// packages (ic7600, ic7610, ic7700, ic7760, ic7800, ic7851 use "%03d";
// ic7410 uses "%04d") minted byte-identical copies of, apart from the
// model name in the two error strings and the memory pad width. ic7200
// (P1/P2 = 200/201) and ic705 (group-addressed) stay out: their P1/P2
// channel numbers, or their slot shape, differ.
//
// A package not yet on the engine keeps a one-line addressToSlot wrapper
// naming its own model and format; one already on the engine has the
// wrapper in helpers_test.go only, since nothing in production reads it.
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

// SlotToAddress maps a canonical wire-form slot to the channel address the
// codec addresses it by, and to the bank it belongs to. "001".."099" are
// the memories and "P1"/"P2" the scan edges; "000", "100", a bare "1", ""
// and any group form are errors. model prefixes the error text.
func SlotToAddress(model, slot string) (civ.ChannelAddress, spec.BankID, error) {
	switch slot {
	case "P1":
		return civ.ChannelAddress{Channel: scanEdgeP1Channel}, spec.BankScan, nil
	case "P2":
		return civ.ChannelAddress{Channel: scanEdgeP2Channel}, spec.BankScan, nil
	}
	if len(slot) != 3 {
		return civ.ChannelAddress{}, "", fmt.Errorf("%s: %q is not a slot on this radio: a memory is three digits (\"001\"..\"099\") and a scan edge is \"P1\" or \"P2\"", model, slot)
	}
	n, err := strconv.Atoi(slot)
	if err != nil {
		return civ.ChannelAddress{}, "", fmt.Errorf("%s: %q is not a slot on this radio: %w", model, slot, err)
	}
	if n < 1 || n > lastMemoryChannel {
		return civ.ChannelAddress{}, "", fmt.Errorf("%s: %q is outside this radio's memory range \"001\"..\"099\"", model, slot)
	}
	return civ.ChannelAddress{Channel: n}, spec.BankMemory, nil
}

// ChannelRange is the flat channel addresses lo..hi in order, for a
// package's occupied-slot search schedule.
func ChannelRange(lo, hi int) []civ.ChannelAddress {
	out := make([]civ.ChannelAddress, 0, hi-lo+1)
	for ch := lo; ch <= hi; ch++ {
		out = append(out, civ.ChannelAddress{Channel: ch})
	}
	return out
}
