// SPDX-License-Identifier: GPL-3.0-or-later

package clonewire

import (
	"fmt"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
)

// MapChannels maps img.Channels (this package's own lightweight
// DecodedChannel, see image.go) into codeplug.Channel/codeplug.ChannelData
// — the ONE place this milestone's transport-and-parse output turns into
// the neutral memory model (Phase 4, orchestrator note: "keep the mapping
// in one place").
//
// This family has NO registered core/driver capabilities (spec.md §Read
// model point 7: no driver.Session implementation exists for it), so
// there is no spec.Capabilities to validate CTCSS/Shift/tone vocabularies
// against, or to ask which tier fields this radio reaches. The minimal
// honest route, taken here:
//
//   - FreqHz, Mode and Tag (from DecodedChannel.Name) are the only fields
//     this family's record parse (record.go) actually decodes, so they are
//     the only fields carried across.
//   - CTCSS and Shift have NO tri-state form (ChannelData.CTCSS/Shift are
//     plain, always-present strings) and are not decoded by this family's
//     DecodeRecord at all — they are set to "OFF"/"SIMPLEX", the same
//     "nothing special" literal every other radio's own fixtures use for
//     an unremarkable channel (e.g. core/driver/ft2000's write_test.go).
//     This is a DEFAULT, not a decoded fact, and is recorded as such here
//     rather than invented per caller.
//   - TagDisplay, ScanSkip, CTCSSTone and DataMode are the pre-tier
//     tri-state fields this family cannot reach either; each is set
//     Unavailable directly (they predate, and are not part of,
//     codeplug.TierFields).
//   - Every one of the twenty TierFields entries is set Unavailable via
//     its own SetState accessor (core/codeplug/tierfields.go) — the
//     project's existing "field the record cannot reach" convention, not
//     a new state invented here.
func MapChannels(img Image) []codeplug.Channel {
	n := len(img.Channels)
	channels := make([]codeplug.Channel, n)
	for i, dc := range img.Channels {
		slot := fmt.Sprintf("%03d", i+1)
		if dc.Empty {
			channels[i] = codeplug.Channel{Slot: slot}
			continue
		}
		data := &codeplug.ChannelData{
			FreqHz: dc.FreqHz,
			Mode:   dc.Mode,
			// Not decoded by this family (see doc comment above) —
			// literal defaults, not read facts.
			CTCSS:      "OFF",
			Shift:      "SIMPLEX",
			Tag:        dc.Name,
			TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unavailable},
			CTCSSTone:  codeplug.ToneField{State: codeplug.Unavailable},
			DataMode:   codeplug.BoolField{State: codeplug.Unavailable},
		}
		for _, tf := range codeplug.TierFields {
			tf.SetState(data, codeplug.Unavailable)
		}
		channels[i] = codeplug.Channel{Slot: slot, Data: data}
	}
	return channels
}
