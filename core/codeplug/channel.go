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
	//
	// uint64 since the Icom tier (design D4, adjudication 5): the IC-905
	// reaches 10 GHz and a uint32 caps at 4.29 GHz. The JSON
	// representation is unchanged (a plain number), and the on-disk
	// schema is unchanged for any value that still fits a uint32 — the
	// file writer emits the lowest schema that can represent the content
	// (see schemaFor), so a >4.29 GHz frequency is exactly one of the two
	// things that forces schema 4, and the frozen uint32 schema-3 loader
	// therefore never meets one.
	FreqHz uint64 `json:"freq_hz"`
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
	// TagDisplay is whether the tag is shown in place of the frequency,
	// together with how confidently that is known. See FieldState for the
	// write rule: only a Known TagDisplay is ever sent to a radio.
	//
	// WHY that rule bites is PER RADIO, and this comment used to state one
	// radio's reason as though it were the seam's. On a radio whose memory
	// frame CARRIES a display flag — the FT-710, whose MT Set has one and
	// where the flag is MANDATORY, with no "leave it alone" encoding — a
	// non-Known TagDisplay cannot be transmitted at all without
	// manufacturing a value: Diff blocks such a channel at plan time and the
	// driver refuses it as defence in depth. On a radio whose frame has NO
	// display flag (the FTdx10's combined MT form takes none), there is
	// nothing to manufacture and nothing to send: every channel legitimately
	// reads back Unavailable, that radio's capabilities report the field
	// Unsupported both ways, and a Known value arriving from a file written
	// for a DIFFERENT radio is refused by the capability gate instead —
	// "tag_display not writable on this radio", pinned against the real
	// profile by core/driver/ftdx10's
	// TestDiff_KnownTagDisplayRefusedOnRealProfile. That sentence was
	// briefly false: the Icom tier's first cut of touchedFields filtered
	// the Known value out of the touched set instead, so the write went
	// out with the request silently missing (Wave-1c review 1, finding 1).
	// A Known value is a REQUEST, and this project refuses a request it
	// cannot honour rather than dropping it; see touchedFields.
	//
	// Both radios reach the same place — a non-Known value is never
	// transmitted — by different routes, and neither route is a property of
	// this struct. Unavailable in particular is a real, expected state here,
	// not an error condition.
	//
	// NO omitempty, deliberately: a BoolField is a struct, so the key is
	// always written. An Unknown TagDisplay is a real state that must be
	// VISIBLE in a saved file, not elided into indistinguishability from
	// Known-false — which is precisely the ambiguity the pre-schema-3
	// `bool` with omitempty created and this field exists to end.
	TagDisplay BoolField `json:"tag_display"`
	// ScanSkip is whether this channel is skipped during scanning.
	ScanSkip BoolField `json:"scan_skip"`

	// The ten fields the Icom tier added to the neutral memory model
	// (design D4). Every one is tri-state, and on every channel this
	// project produces for a Yaesu NEWCAT radio every one of them is
	// UNAVAILABLE — a read says so directly (the TagDisplay precedent),
	// a load of a schema-1/2/3 file migrates to it, and those radios'
	// banks list none of the corresponding spec.Fields, so the
	// capabilities agree.
	//
	// That is what keeps the pre-tier world byte-identical, in three
	// places at once: the file writer emits schema 3 while none of them
	// is Recorded (schemaFor — Unavailable says the same thing schema
	// 3 says by having no key), the send plan counts only a Known field
	// as touched (touchedFields), and Validate does not judge a field
	// this radio cannot reach. None of the three needs a per-field
	// exception list.
	//
	// NO omitempty, deliberately, for the reason TagDisplay gives above:
	// these are structs, so the key is always written — and in a schema-4
	// file a state must be VISIBLE rather than elided into
	// indistinguishability from one somebody chose.

	// TxFreqHz is an independent transmit ("split") frequency stored on
	// the channel itself, as opposed to a shift applied to FreqHz.
	TxFreqHz FreqField `json:"tx_frequency"`
	// Duplex is the Icom-family repeater duplex selector, drawn from
	// spec.Capabilities.DuplexOptions (e.g. "OFF", "DUP+", "DUP-"). It
	// and the Yaesu Shift field above never both apply to one radio.
	Duplex StringField `json:"duplex"`
	// OffsetHz is the per-channel repeater offset magnitude in hertz.
	OffsetHz FreqField `json:"offset"`
	// ToneMode is the Icom-family tone-squelch mode, drawn from
	// spec.Capabilities.ToneModes. It and the Yaesu CTCSS field above
	// never both apply to one radio.
	ToneMode StringField `json:"tone_mode"`
	// ToneTx is the transmitted CTCSS tone.
	ToneTx ToneField `json:"tone_tx"`
	// ToneRx is the CTCSS tone required to open squelch on receive.
	ToneRx ToneField `json:"tone_rx"`
	// DTCSCode is the DTCS/DCS code NUMBER (23 for "023"), drawn from
	// spec.Capabilities.DTCSCodes.
	DTCSCode IntField `json:"dtcs_code"`
	// DTCSPolarity is the DTCS polarity pair, drawn from
	// spec.Capabilities.DTCSPolarities (e.g. "NN", "NR", "RN", "RR").
	DTCSPolarity StringField `json:"dtcs_polarity"`
	// Filter is the per-channel IF filter selection, drawn from
	// spec.Capabilities.Filters (e.g. "FIL1").
	Filter StringField `json:"filter"`
	// DataMode is the per-channel data-mode flag, stored alongside — not
	// inside — the mode name.
	DataMode BoolField `json:"data_mode"`
}

// tierFieldsUnrecorded reports whether NONE of the ten fields the Icom
// tier added carries anything this channel needs a file to write down —
// i.e. every one of them is Absent or Unavailable (see
// FieldState.Recorded).
//
// It is the "no tier-added field is present" half of the file writer's
// lowest-schema rule (design D4), and it is deliberately a method on
// ChannelData rather than a loop in file.go, so that a later field added
// to this struct is one edit away from being accounted for here.
func (d ChannelData) tierFieldsUnrecorded() bool {
	return !d.TxFreqHz.State.Recorded() &&
		!d.Duplex.State.Recorded() &&
		!d.OffsetHz.State.Recorded() &&
		!d.ToneMode.State.Recorded() &&
		!d.ToneTx.State.Recorded() &&
		!d.ToneRx.State.Recorded() &&
		!d.DTCSCode.State.Recorded() &&
		!d.DTCSPolarity.State.Recorded() &&
		!d.Filter.State.Recorded() &&
		!d.DataMode.State.Recorded()
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

// bankForSlot reports which Bank in caps can hold slot, by linear scan.
// It returns the zero BankID and false if no bank in caps claims it.
//
// The question is spec.Bank.WithinSpace's, not bare membership of Slots:
// on a dense bank those are the same question, and on one of the Icom
// tier's SPARSE banks (design D4, adjudication 7) they are not — Slots
// there lists only what a read materialised, while the addressable space
// is Groups x PerGroup, and a slot the user is ADDING is legitimately in
// the space without being in the list. Asking membership alone would
// have made every such add "not part of any bank this radio supports".
//
// Unexported for now: exported later if a caller outside this package
// needs it.
func bankForSlot(caps spec.Capabilities, slot string) (spec.BankID, bool) {
	for _, b := range caps.Banks {
		if b.WithinSpace(slot) {
			return b.ID, true
		}
	}
	return "", false
}
