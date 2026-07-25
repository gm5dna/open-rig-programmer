// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"fmt"
	"sort"
	"strings"

	"go.bug.st/serial/enumerator"
)

// PortInfo describes one candidate serial port Discover found, along with
// rankPorts's judgement of how likely it is to be the FT-710's CAT
// interface.
type PortInfo struct {
	// Path is the OS device path (e.g. "/dev/cu.SLAB_USBtoUART",
	// "COM3").
	Path string
	// Description is a human-readable label for the device, when the OS
	// provides one (the USB iProduct string, or its USB configuration
	// string as a fallback) — empty if neither is available.
	Description string
	// VID and PID are the USB Vendor/Product ID as 4 upper-case hex
	// digits (e.g. "10C4", "EA70"), when the port is a USB serial device.
	VID, PID string
	// USBSerial is the USB serial number string, when available.
	USBSerial string
	// Score is rankPorts's combined ranking signal: higher means more
	// likely to be the FT-710's CAT-1 interface. A port that matched no
	// ranking signal at all still gets Score 0 and IS still listed — see
	// rankPorts's doc comment.
	Score int
	// Hints explains, in human-readable form, every ranking signal that
	// contributed to Score (or, for an unscored port, that none did).
	Hints []string
}

// Discover lists every serial port the OS currently exposes, ranked by how
// likely each is to be the FT-710's CAT interface (see rankPorts). It NEVER
// filters a candidate out for scoring zero — only rankPorts's own
// darwin cu/tty duplicate-node rule removes an entry, and only because it
// is a literal duplicate of another entry already in the list (see
// dedupDarwinCallout). The active ID; probe that finally confirms which
// port is actually attached to an FT-710 is a later layer's job (the CAT
// driver), not this package's.
func Discover() ([]PortInfo, error) {
	ports, err := enumerator.GetDetailedPortsList()
	if err != nil {
		return nil, fmt.Errorf("transport: discover: %w", err)
	}
	return rankPorts(ports), nil
}

// Score components. Each is a small, independently documented ranking
// signal; scoreCandidate sums whichever apply to a given port. None of
// these values has been validated against real FT-710 hardware — they
// encode plausible, documented reasoning (see each constant's comment) for
// M5a to confirm or correct once physical CAT interfaces are available to
// test against.
const (
	// scoreCP2105 rewards an exact VID:PID match for the Silicon Labs
	// CP2105 dual-UART USB-to-serial bridge (10C4:EA70) — the chipset
	// commonly used by FT-710 CAT-capable USB interfaces.
	scoreCP2105 = 50
	// scoreEnhanced further rewards a description mentioning "Enhanced":
	// the CP2105 exposes two independent UARTs per physical device, one
	// enumerated with "...Enhanced Interface" and one with "...Standard
	// Interface" in its USB product string. The FT-710's CAT-1 protocol
	// answers on the Enhanced interface. See scoreStandardPenalty.
	scoreEnhanced = 20
	// scoreStandardPenalty penalises a description mentioning "Standard"
	// — the CP2105's OTHER UART, not the CAT-1 interface. See
	// scoreEnhanced. A Standard-interface CP2105 node still nets positive
	// overall (scoreCP2105 dominates), it just ranks below its sibling
	// Enhanced node.
	scoreStandardPenalty = -20
	// scoreOtherUSBSerial weakly rewards a handful of OTHER common
	// USB-serial bridge chips that could plausibly be a generic
	// USB-serial CAT cable, without the CP2105-specific signal above:
	// FTDI's classic chips (VID 0403, any PID), the WCH CH340/CH341
	// family (VID 1A86), and Silicon Labs' single-UART CP2102/CP2109
	// (10C4:EA60 — same vendor as the CP2105, different, weaker-signal
	// product).
	scoreOtherUSBSerial = 5
	// scoreDarwinCu prefers macOS's "callout" (/dev/cu.*) device node,
	// which opens without waiting for carrier-detect — what a CAT session
	// needs — over its paired "callin" (/dev/tty.*) node for the SAME
	// physical serial line. When both nodes for one physical device are
	// present, dedupDarwinCallout drops the tty.* one outright rather
	// than merely scoring it lower; this small positive score remains for
	// the surviving cu.* entry (and for the rare case a cu.* node appears
	// with no tty.* pairing at all).
	scoreDarwinCu = 3
)

// rankPorts scores and ranks ports, returning one PortInfo per surviving
// candidate, sorted by Score descending (ties broken by Path ascending, for
// deterministic output). It is a pure function — no I/O, table-testable —
// factored out of Discover exactly so the ranking policy can be exercised
// without a real OS port list.
//
// rankPorts NEVER returns fewer candidates because of scoring: every input
// port, however unrecognised, appears in the output with Score 0 (still
// listed — see PortInfo.Score's doc comment). The ONE exception is
// dedupDarwinCallout's darwin cu/tty duplicate-node rule, which removes a
// tty.* entry only when an equivalent cu.* entry for the SAME physical
// device is also present in THIS SAME call's input — that is a genuine
// duplicate, not a filtered-out candidate, so it does not violate "never
// filters to zero": the device is still represented, once, by its cu.*
// node.
//
// The active ID; probe that finally decides which ranked candidate really
// is an FT-710 is deliberately NOT this function's job (see Discover's doc
// comment) — rankPorts only orders candidates for a caller (or a human) to
// try.
func rankPorts(ports []*enumerator.PortDetails) []PortInfo {
	infos := make([]PortInfo, 0, len(ports))
	for _, p := range ports {
		if p == nil {
			continue
		}
		infos = append(infos, scoreCandidate(p))
	}

	infos = dedupDarwinCallout(infos)

	sort.SliceStable(infos, func(i, j int) bool {
		if infos[i].Score != infos[j].Score {
			return infos[i].Score > infos[j].Score
		}
		return infos[i].Path < infos[j].Path
	})
	return infos
}

// scoreCandidate builds the PortInfo for one enumerator.PortDetails entry,
// applying every ranking rule documented on the score* constants above and
// recording a human-readable Hint for each one that applied.
func scoreCandidate(p *enumerator.PortDetails) PortInfo {
	info := PortInfo{
		Path:        p.Name,
		Description: candidateDescription(p),
		VID:         p.VID,
		PID:         p.PID,
		USBSerial:   p.SerialNumber,
	}

	switch {
	case eqFold(p.VID, "10C4") && eqFold(p.PID, "EA70"):
		info.Score += scoreCP2105
		info.Hints = append(info.Hints, fmt.Sprintf("CP2105 VID:PID match (%s:%s) — the dual-UART bridge chip commonly used by FT-710 CAT interfaces", p.VID, p.PID))
	case eqFold(p.VID, "0403"):
		info.Score += scoreOtherUSBSerial
		info.Hints = append(info.Hints, "FTDI USB-serial chip (VID 0403) — could be a generic CAT cable")
	case eqFold(p.VID, "1A86"):
		info.Score += scoreOtherUSBSerial
		info.Hints = append(info.Hints, "WCH CH340/CH341 USB-serial chip (VID 1A86) — could be a generic CAT cable")
	case eqFold(p.VID, "10C4") && eqFold(p.PID, "EA60"):
		info.Score += scoreOtherUSBSerial
		info.Hints = append(info.Hints, "Silicon Labs CP2102/CP2109 single-UART USB-serial chip (10C4:EA60) — could be a generic CAT cable, but not the CP2105 dual-UART bridge")
	}

	lowerDesc := strings.ToLower(info.Description)
	switch {
	case strings.Contains(lowerDesc, "enhanced"):
		info.Score += scoreEnhanced
		info.Hints = append(info.Hints, `description contains "Enhanced" — CAT-1 is the CP2105's Enhanced interface`)
	case strings.Contains(lowerDesc, "standard"):
		info.Score += scoreStandardPenalty
		info.Hints = append(info.Hints, `description contains "Standard" — the CP2105's OTHER (non-CAT) interface`)
	}

	if strings.HasPrefix(info.Path, "/dev/cu.") {
		info.Score += scoreDarwinCu
		info.Hints = append(info.Hints, "/dev/cu.* — macOS callout device node, preferred over a paired /dev/tty. node")
	}

	if len(info.Hints) == 0 {
		info.Hints = append(info.Hints, "no ranking signal matched; listed for completeness")
	}

	return info
}

// candidateDescription picks PortInfo.Description's source field: the USB
// iProduct string when available, falling back to the USB configuration
// string, or "" if neither is present (e.g. a non-USB port).
func candidateDescription(p *enumerator.PortDetails) string {
	switch {
	case p.Product != "":
		return p.Product
	case p.Configuration != "":
		return p.Configuration
	default:
		return ""
	}
}

// eqFold reports whether a and b are equal, ignoring case — used for
// VID/PID comparison so a library implementation on some future platform
// that emits lower-case hex still matches (the darwin implementation this
// package has been developed against emits upper-case, per
// enumerator's usb_darwin.go, fmt.Sprintf("%04X", ...)).
func eqFold(a, b string) bool {
	return strings.EqualFold(a, b)
}

// dedupDarwinCallout drops any /dev/tty.SUFFIX entry from infos when a
// /dev/cu.SUFFIX entry with the identical SUFFIX is also present — macOS's
// well-known callout/callin device-node pairing for the SAME physical
// serial line (see scoreDarwinCu's doc comment). An entry with no matching
// pair — including every non-darwin port path, which never has this
// "/dev/cu."/"/dev/tty." prefix shape at all — passes through unchanged.
func dedupDarwinCallout(infos []PortInfo) []PortInfo {
	cuSuffixes := make(map[string]bool, len(infos))
	for _, info := range infos {
		if suffix, ok := strings.CutPrefix(info.Path, "/dev/cu."); ok {
			cuSuffixes[suffix] = true
		}
	}

	out := make([]PortInfo, 0, len(infos))
	for _, info := range infos {
		if suffix, ok := strings.CutPrefix(info.Path, "/dev/tty."); ok {
			if cuSuffixes[suffix] {
				continue // dropped: a matching /dev/cu.* entry represents the same device
			}
		}
		out = append(out, info)
	}
	return out
}
