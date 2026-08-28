// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700

import (
	"fmt"
	"strconv"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/spec"
)

// THE SLOT SPELLING, AND WHY IT CARRIES A BAND.
//
// This radio addresses a memory by (band, channel): ① is a FREQUENCY BAND
// CODE — 01 = 144 MHz, 02 = 430 MHz, 03 = 1.2 GHz, PDF p.16's
// `①Frequency band codes` table — and ②,③ are four printed decimal digits
// numbering the channel inside that band. Channel 1 of the 144 MHz band
// and channel 1 of the 430 MHz band are DIFFERENT memories, so a slot
// string carrying only the channel number would name three memories at
// once, and the neutral layers above this seam (which key everything on
// the slot string) would silently conflate them.
//
// The three forms follow the printed channel ranges (leg B's ②,③ row):
//
//	MEM   <band>-<nnn>       0001~0099   144-001, 430-099, 1200-042
//	SCAN  <band>-P<n><A|B>   0100~0105   144-P1A, 430-P3B
//	CALL  <band>-C<n>        0106~0107   1200-C1, 144-C2
//
// IT IS A CHOICE, and it is recorded as one: spec D4 defers per-model
// DisplaySlot cosmetics to the roadmap, and no radio-facing claim rides on
// the spelling — the WIRE only ever sees the address these functions
// produce. What does have to hold is that the mapping is a bijection over
// the 321 addressable slots and that no two slots share a name;
// slots_test.go walks the whole space to prove both, rather than sampling.
//
// SCAN's P<n><A|B> mirrors the front panel's own naming of the six program
// scan edges (1A/1B, 2A/2B, 3A/3B) and maps onto the printed numbering in
// its printed order: 0100 is 1A, 0101 is 1B, and so on in pairs.

// The three printed band names, in wire-index order. bandNames[i] is the
// name of wire band i+1, which under E4 is what civ.ChannelAddress.Group
// carries directly (GroupBase 1).
var bandNames = [3]string{"144", "430", "1200"}

// The channel-number boundaries of the three banks, from leg B's ②,③ row.
// Stated as named constants because three separate files ask about them
// and a repeated literal 99 is how an off-by-one gets in.
const (
	memChannelLo  = 1
	memChannelHi  = 99
	scanChannelLo = 100
	scanChannelHi = 105
	callChannelLo = 106
	callChannelHi = 107
)

// bandIndex maps a printed band name to its wire band code, or reports
// that this is not one of the three names this radio prints.
func bandIndex(name string) (int, bool) {
	for i, n := range bandNames {
		if n == name {
			return i + 1, true
		}
	}
	return 0, false
}

// bankForChannel reports which bank a channel number falls in.
func bankForChannel(channel int) (spec.BankID, bool) {
	switch {
	case channel >= memChannelLo && channel <= memChannelHi:
		return spec.BankMemory, true
	case channel >= scanChannelLo && channel <= scanChannelHi:
		return spec.BankScan, true
	case channel >= callChannelLo && channel <= callChannelHi:
		return spec.BankCall, true
	default:
		return "", false
	}
}

// slotAddress turns a canonical slot string into the wire address and the
// bank that owns it.
//
// IT REFUSES, NEVER GUESSES. Every rejected shape below is a slot this
// radio has no memory for, and the alternative to a refusal is a read or
// a write aimed at whichever channel the nearest interpretation landed
// on. In particular "144-100" is refused rather than read as the first
// scan edge: 0100 IS the first scan edge on the wire, but its canonical
// name here is 144-P1A, and admitting both spellings would give one
// memory two names — the collision slots_test.go exists to forbid.
func slotAddress(slot string) (civ.ChannelAddress, spec.BankID, error) {
	dash := -1
	for i := 0; i < len(slot); i++ {
		if slot[i] == '-' {
			dash = i
			break
		}
	}
	if dash <= 0 || dash == len(slot)-1 {
		return civ.ChannelAddress{}, "", fmt.Errorf("ic9700: slot %q is not <band>-<channel>", slot)
	}
	group, ok := bandIndex(slot[:dash])
	if !ok {
		return civ.ChannelAddress{}, "", fmt.Errorf("ic9700: slot %q names band %q; this radio prints %v", slot, slot[:dash], bandNames)
	}
	rest := slot[dash+1:]

	channel, bank, err := channelForSuffix(rest)
	if err != nil {
		return civ.ChannelAddress{}, "", fmt.Errorf("ic9700: slot %q: %w", slot, err)
	}
	return civ.ChannelAddress{Group: group, Channel: channel}, bank, nil
}

// channelForSuffix decodes the part of a slot string after the band.
func channelForSuffix(rest string) (int, spec.BankID, error) {
	switch {
	case len(rest) == 3 && rest[0] >= '0' && rest[0] <= '9':
		n, err := strconv.Atoi(rest)
		if err != nil {
			return 0, "", fmt.Errorf("memory channel %q is not three decimal digits", rest)
		}
		if n < memChannelLo || n > memChannelHi {
			return 0, "", fmt.Errorf("memory channel %q is outside the printed 001~099", rest)
		}
		return n, spec.BankMemory, nil

	case len(rest) == 3 && rest[0] == 'P':
		pair := int(rest[1] - '0')
		if pair < 1 || pair > (scanChannelHi-scanChannelLo+1)/2 {
			return 0, "", fmt.Errorf("scan edge pair %q is outside the printed three", rest)
		}
		var half int
		switch rest[2] {
		case 'A':
			half = 0
		case 'B':
			half = 1
		default:
			return 0, "", fmt.Errorf("scan edge %q ends in %q, not A or B", rest, rest[2:])
		}
		return scanChannelLo + (pair-1)*2 + half, spec.BankScan, nil

	case len(rest) == 2 && rest[0] == 'C':
		n := int(rest[1] - '0')
		if n < 1 || n > callChannelHi-callChannelLo+1 {
			return 0, "", fmt.Errorf("call channel %q is outside the printed C1 and C2", rest)
		}
		return callChannelLo + n - 1, spec.BankCall, nil

	default:
		return 0, "", fmt.Errorf("%q is none of this radio's three slot forms (<nnn>, P<n>A/B, C<n>)", rest)
	}
}

// addressSlot is slotAddress's inverse: the canonical slot string for a
// wire address, and the bank that owns it.
//
// IT REFUSES AN ADDRESS THIS RADIO HAS NO NAME FOR rather than rendering
// one. The addresses reaching it come off the wire (a decoded memory
// answer), and civ's own channel-space validation is not this function's
// to assume: an address outside 1..3 x 1..107 means the answer was not
// about a memory this driver can speak for, and inventing a name for it
// would let a later comparison believe two different things were the same
// slot.
func addressSlot(addr civ.ChannelAddress) (string, spec.BankID, error) {
	if addr.Group < 1 || addr.Group > len(bandNames) {
		return "", "", fmt.Errorf("ic9700: address %v names band %d; this radio's codes are 01, 02 and 03", addr, addr.Group)
	}
	band := bandNames[addr.Group-1]
	bank, ok := bankForChannel(addr.Channel)
	if !ok {
		return "", "", fmt.Errorf("ic9700: address %v names channel %d, outside the printed 0001~0107", addr, addr.Channel)
	}
	switch bank {
	case spec.BankMemory:
		return fmt.Sprintf("%s-%03d", band, addr.Channel), bank, nil
	case spec.BankScan:
		off := addr.Channel - scanChannelLo
		half := "A"
		if off%2 == 1 {
			half = "B"
		}
		return fmt.Sprintf("%s-P%d%s", band, off/2+1, half), bank, nil
	default:
		return fmt.Sprintf("%s-C%d", band, addr.Channel-callChannelLo+1), bank, nil
	}
}

// bankSlots renders every canonical slot string in one bank, in wire
// order: band by band, channel by channel.
//
// The capability tables are BUILT from this function rather than written
// out, so the slot list and the address mapping cannot disagree — a
// hand-written list of 297 strings is 297 chances to mistype one, and the
// mistyped one would simply never be readable.
func bankSlots(id spec.BankID) []string {
	lo, hi := memChannelLo, memChannelHi
	switch id {
	case spec.BankScan:
		lo, hi = scanChannelLo, scanChannelHi
	case spec.BankCall:
		lo, hi = callChannelLo, callChannelHi
	}
	out := make([]string, 0, len(bandNames)*(hi-lo+1))
	for group := 1; group <= len(bandNames); group++ {
		for ch := lo; ch <= hi; ch++ {
			slot, _, err := addressSlot(civ.ChannelAddress{Group: group, Channel: ch})
			if err != nil {
				// Unreachable: the bounds above are the same constants
				// addressSlot validates against. Asserted rather than
				// ignored, because a silently short slot list would read
				// as a radio with fewer memories than it has.
				panic(fmt.Sprintf("ic9700: bankSlots(%s): %v", id, err))
			}
			out = append(out, slot)
		}
	}
	return out
}
