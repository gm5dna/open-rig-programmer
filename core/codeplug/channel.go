// SPDX-License-Identifier: GPL-3.0-or-later

package codeplug

import "github.com/gm5dna/open-rig-programmer/core/spec"

// Channel is one memory-channel slot: either empty, or populated with
// ChannelData. Data == nil is the SOLE discriminator between empty and
// populated (see Empty) — there is no separate "in use" flag that could
// fall out of sync with the data, so a channel can never be both empty
// and populated at once.
type Channel struct {
	// Slot is the canonical wire-form slot identifier, e.g. "001", "P1L",
	// "501", "EMG".
	Slot string `json:"slot"`
	// Data is nil for an empty slot; a non-nil Data means the slot is
	// populated. omitempty means an empty channel's JSON has no "data"
	// key at all, not a "data": null.
	Data *ChannelData `json:"data,omitempty"`
}

// ChannelData is the contents of a populated memory channel.
type ChannelData struct {
	// FreqHz is the receive frequency in hertz.
	FreqHz uint32 `json:"freq_hz"`
	// Mode is the display-name mode, e.g. "USB", "DATA-FM-N".
	Mode string `json:"mode"`
	// ClarHz is the clarifier offset in hertz.
	ClarHz int `json:"clar_hz,omitempty"`
	// RxClar is whether the clarifier applies to receive.
	RxClar bool `json:"rx_clar,omitempty"`
	// TxClar is whether the clarifier applies to transmit.
	TxClar bool `json:"tx_clar,omitempty"`
	// CTCSS is the CTCSS mode: "OFF", "ENC-DEC", or "ENC".
	CTCSS string `json:"ctcss"`
	// CTCSSTone is the CTCSS tone, meaningful when CTCSS is not "OFF".
	CTCSSTone ToneField `json:"ctcss_tone"`
	// Shift is the repeater shift direction: "SIMPLEX", "PLUS", or
	// "MINUS".
	Shift string `json:"shift"`
	// Tag is the channel's display name/label.
	Tag string `json:"tag,omitempty"`
	// TagDisplay is whether the tag is shown in place of the frequency.
	TagDisplay bool `json:"tag_display,omitempty"`
	// ScanSkip is whether this channel is skipped during scanning.
	ScanSkip BoolField `json:"scan_skip"`
}

// Empty reports whether c is an empty slot. Data == nil is the sole test:
// see the Channel doc comment.
func (c Channel) Empty() bool {
	return c.Data == nil
}

// DisplaySlot returns the human-readable form of a canonical wire-form
// slot identifier, matching how the FT-710 front panel and manual label
// memory channels: ordinary memory channels "001".."099" become
// "M-01".."M-99", and 60 m channels "501".."599" become "5-01".."5-99".
// Every other form — PMS pairs ("P1L".."P9U"), the emergency channel
// ("EMG"), and anything not matching a recognised pattern — is returned
// unchanged: either it is already in its display form, or it is not a
// form this function recognises.
//
// This is the neutral default: the Yaesu 3-digit-family front-panel
// convention (ordinary/60 m memory channels, above), with an identity
// fallback for any other wire form — including a radio whose own slot
// forms are shaped differently, e.g. a longer, non-3-digit form such as
// the FTX-1's — which simply passes through unrecognised and unchanged.
// A per-model override of this mapping is deferred to the FTX-1
// milestone; nothing else in this project should hand-roll one today.
func DisplaySlot(slot string) string {
	if len(slot) != 3 {
		return slot
	}
	if slot[1] < '0' || slot[1] > '9' || slot[2] < '0' || slot[2] > '9' {
		return slot
	}
	switch slot[0] {
	case '0':
		return "M-" + slot[1:]
	case '5':
		return "5-" + slot[1:]
	default:
		return slot
	}
}

// bankForSlot reports which Bank in caps lists slot among its Slots, by
// linear scan. It returns the zero BankID and false if no bank in caps
// claims slot.
//
// Unexported for now: exported later if a caller outside this package
// needs it.
func bankForSlot(caps spec.Capabilities, slot string) (spec.BankID, bool) {
	for _, b := range caps.Banks {
		for _, s := range b.Slots {
			if s == slot {
				return b.ID, true
			}
		}
	}
	return "", false
}
