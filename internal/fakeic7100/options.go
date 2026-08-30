// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7100

import (
	"fmt"
	"time"
)

// Option configures a fake radio at New time. Options are applied in the order
// they are given, so a later one that names the same channel as an earlier one
// wins: WithEmptySlot(1, 3) after WithSlot(1, 3, …) leaves the channel empty,
// and the two written the other way round leave it occupied.
//
// EVERY OPTION BELOW THAT CHANGES AN ANSWER EXISTS BECAUSE THE DOCUMENT LEAVES
// SOMETHING OPEN. Each names the doc.go register entry it belongs to. A default
// is this package's reading of the printed page; the option is the other
// admissible reading, so that a consumer can prove its driver survives either
// rather than only the one that was guessed.
type Option func(*config)

// seed is one channel an option asked for before the radio started answering.
type seed struct {
	addr     []byte
	record   []byte
	occupied bool
}

type config struct {
	radioAddress   byte
	identityToken  []byte
	seeds          []seed
	acceptedLength int

	allFFEmpty     bool
	unequalTXOK    bool
	shortSetsOK    bool
	echoBack       bool
	noSetAnswer    bool
	broadcasts     time.Duration
	addressedFlood time.Duration
}

// defaultIdentityToken is what a 19 00 request is answered with unless
// WithIdentityToken says otherwise.
//
// It is an INVENTION, chosen so that no reader could mistake it for a fact
// about an IC-7100. The command table's Data column for 19 00 is blank
// (PDF p.364, folio 20-5) and nothing anywhere in the document says what comes
// back. See doc.go, entry 4 (ic7100-id-reply-value).
var defaultIdentityToken = []byte{0xDE, 0xAD}

// WithRadioAddress puts this radio at a CI-V address other than the default.
//
// This is NOT a fault option: CI-V Address is a set-mode item on a real
// IC-7100 (PDF p.334, folio 17-25, "CI-V Address (Default: 88h)"), and the
// manual says a bus may carry up to four Icom CI-V devices, so a radio
// answering at 22h is a radio, not a misbehaviour.
//
// The preamble and end-of-message bytes are refused: neither may appear inside
// a frame, so no radio could be reached at either.
func WithRadioAddress(addr byte) Option {
	if addr == preamble || addr == endOfMessage {
		panic(fmt.Sprintf("fakeic7100: WithRadioAddress(%02X): %02X is the %s byte, which may never appear inside a frame, so no radio could be addressed at it", addr, addr, byteName(addr)))
	}
	return func(c *config) { c.radioAddress = addr }
}

// WithIdentityToken sets the data bytes a 19 00 request is answered with.
//
// The DEFAULT value is invented (defaultIdentityToken) because the reply is
// undocumented, and this option exists so a consumer can pin a DIFFERENT token
// and prove its driver RECORDS whatever it gets rather than matching a
// particular value. An empty token is honoured as an empty token: the radio
// then answers FE FE E0 88 19 00 FD, which is a legitimate thing to want a
// driver to cope with. See doc.go, entry 4.
func WithIdentityToken(tok []byte) Option {
	out := append([]byte(nil), tok...)
	for i, b := range out {
		if b == preamble || b == endOfMessage {
			panic(fmt.Sprintf("fakeic7100: WithIdentityToken(%s): byte %d is %02X, the %s byte, which may never appear inside a frame's data", hexBytes(tok), i, b, byteName(b)))
		}
	}
	return func(c *config) { c.identityToken = out }
}

// WithSlot seeds one memory channel with a raw record.
//
// The record is OPAQUE to this package beyond its length and its transmit
// duplicate: it is stored and served back byte for byte, and nothing here
// decodes a frequency, a mode or a name out of it. Records of ANY length may be
// seeded — that is what a driver's record-length fingerprint test needs, since
// a radio that answers 104 bytes where 111 was expected is exactly the sibling
// confusion the fingerprint exists to catch.
//
// bank is 1 (A) to 5 (E) and channel is 1 to 99, the rectangle the field
// legends print. Anything else panics, including the special channels
// 0100-0109: the document names them but never says what bank byte they carry,
// so there is nothing for this fake to be a fake OF. See doc.go, entry 10.
//
// A record containing the preamble or end-of-message byte panics too: neither
// may appear inside a frame's data, so a record holding one could not be sent
// by any radio, and seeding one is a caller's mistake rather than a test case.
func WithSlot(bank, channel int, record []byte) Option {
	addr := mustChannelAddress("WithSlot", bank, channel)
	rec := append([]byte(nil), record...)
	for i, b := range rec {
		if b == preamble || b == endOfMessage {
			panic(fmt.Sprintf("fakeic7100: WithSlot(%d, %d, …) [%s]: record byte %d is %02X, the %s byte, which may never appear inside a frame's data", bank, channel, describeAddress(addr), i, b, byteName(b)))
		}
	}
	return func(c *config) {
		c.seeds = append(c.seeds, seed{addr: addr, record: rec, occupied: true})
	}
}

// WithEmptySlot marks one channel unoccupied.
//
// A channel that was never seeded is unoccupied anyway; this exists to say so
// on purpose, and to override an earlier WithSlot for the same channel.
func WithEmptySlot(bank, channel int) Option {
	addr := mustChannelAddress("WithEmptySlot", bank, channel)
	return func(c *config) {
		c.seeds = append(c.seeds, seed{addr: addr, occupied: false})
	}
}

// WithAcceptedRecordLength changes the record length a memory SET must carry,
// away from the 111 bytes this package derived (records.go).
//
// It exists for the near miss: taking the diagram bar's own (52)~(60) label at
// face value gives a 104-byte record, which is where a text-only reading of the
// page lands. A driver that fingerprints on 111 has to meet a radio that
// answers something else, and this is how a test builds one.
//
// It does NOT change what a seeded slot ANSWERS — a slot answers the bytes it
// was seeded with, whatever their length. Nor does it change the transmit-block
// equality rule's reach: that rule is derived from the 111-byte geometry and is
// applied only to records of that length, because this package knows where the
// blocks sit in a 111-byte record and nowhere else.
func WithAcceptedRecordLength(n int) Option {
	if n <= 0 {
		panic(fmt.Sprintf("fakeic7100: WithAcceptedRecordLength(%d): a record is at least one byte long", n))
	}
	return func(c *config) { c.acceptedLength = n }
}

// WithAllFFEmptyRecord answers a read of an unoccupied channel with a
// full-length record of FF bytes instead of the NG code.
//
// Both behaviours are ASSUMED and they are SEPARATE assumptions with separate
// register entries — doc.go entries 1 (ic7100-empty-channel-fa) and 2
// (ic7100-all-ff-record) — because one capture cannot establish both. The
// document says nothing at all about what a read of a cleared channel returns;
// its only use of FF as an emptiness marker is on the WRITE side, in the
// "About clearing operation" block.
func WithAllFFEmptyRecord() Option {
	return func(c *config) { c.allFFEmpty = true }
}

// WithUnequalTransmitBlockAccepted stops the radio refusing a set whose
// transmit duplicate differs from its receive payload.
//
// The default refuses, which is this package's reading of the printed NOTE.
// The NOTE's own words are advisory — "We recommend that you set the same data
// as (5)-(51)" — and the document never says what a radio does with a set whose
// blocks differ, so the refusal is an assumption and this is the other reading.
// See doc.go, entry 6 (ic7100-tx-block-mandatory).
func WithUnequalTransmitBlockAccepted() Option {
	return func(c *config) { c.unequalTXOK = true }
}

// WithShortSetsAccepted makes the radio store a set that stops before the
// record is complete, instead of refusing it.
//
// The document says nothing about a short set: it draws one record, of fixed
// index ranges, and the conditional-width notes elsewhere in the chapter belong
// to other commands. The default refuses; this is the other reading. See
// doc.go, entry 6.
//
// The printed CLEARING form is still refused even under this option — see
// doc.go, entry 12 — because it is a destructive form this tier never sends, and
// a fake that stored a one-byte "record" for it would be modelling nothing.
func WithShortSetsAccepted() Option {
	return func(c *config) { c.shortSetsOK = true }
}

// WithNoSetAnswer makes the radio store an acceptable set and then say nothing
// at all about it: no FB, and no FA either, since nothing was refused.
//
// IT IS A TEST LEVER, NOT AN ASSUMPTION ABOUT AN IC-7100. Every other option
// above is the other admissible reading of an open page, with a register entry
// and a capture that would retire it. This one models a LOST ACKNOWLEDGEMENT ON
// THE LINK — the radio heard the frame, acted on it, and the reply did not
// arrive — so no capture from a radio could ever settle it and it has no
// register entry. See doc.go, WHAT IS NOT IN THAT REGISTER.
//
// It exists for the transport engine's write rule: a memory set is an
// acknowledged write, sent EXACTLY ONCE and never retransmitted when the
// acknowledgement fails to come back, with a post-write quarantine drain
// afterwards whatever the outcome (core/transport, "Command classes are stated,
// not inferred"). No radio that always answers can take a driver down that
// branch. This is the radio that hears one set frame and then says nothing —
// entry 11's silence is the read-path counterpart, and strands a read the same
// way.
//
// The set is STORED, so Slot and Transcript report it exactly as they always
// do, which is how a test tells "the write was lost" from "the write never
// happened". Everything else the radio does is unchanged: reads, 19 00 and
// every refusal are answered as before.

func WithNoSetAnswer() Option {
	return func(c *config) { c.noSetAnswer = true }
}

// WithEcho echoes every received frame back before answering it.
//
// The echo is the frame as normalised on receipt — exactly two preamble bytes,
// however many arrived — because that is the frame the fake received, and it is
// what Transcript reports too.
//
// Whether a real IC-7100 echoes is ASSUMED: the manual has no echo-back
// setting anywhere, and its [REMOTE] jack is a shared bus on which echo would
// be a property of the wiring rather than a setting. See doc.go, entry 3
// (ic7100-echo-default).
func WithEcho() Option {
	return func(c *config) { c.echoBack = true }
}

// WithTransceiveBroadcasts emits unsolicited frames addressed to 00 every d.
//
// The to=00 SPELLING IS ASSUMED — doc.go, entry 5
// (ic7100-broadcast-address-form). This document never prints 00 as an address
// value: its frame diagrams show only the point-to-point pair 88/E0, and the
// set-mode page describes CI-V Transceive without giving the frame. What the
// fake emits is the form a controller's address filter is built for; the frame
// CONTENT is arbitrary and asserts nothing.
func WithTransceiveBroadcasts(d time.Duration) Option {
	mustPositiveInterval("WithTransceiveBroadcasts", d)
	return func(c *config) { c.broadcasts = d }
}

// WithAddressedFlood emits frames addressed to the CONTROLLER every d.
//
// IT IS A SEPARATE OPTION BECAUSE THE TWO SPECIES EXERCISE DIFFERENT CODE. A
// to=00 broadcast dies at a controller's address filter and never reaches its
// engine; a controller-addressed frame does reach it, and is the only kind that
// can drive a drain to its cap. A test that wants to prove a cap needs this
// one; a test that wants to prove noise is ignored needs the other. Same
// standing as entry 5: nothing has been observed.
func WithAddressedFlood(d time.Duration) Option {
	mustPositiveInterval("WithAddressedFlood", d)
	return func(c *config) { c.addressedFlood = d }
}

func mustChannelAddress(option string, bank, channel int) []byte {
	addr, err := channelAddress(bank, channel)
	if err != nil {
		panic(fmt.Sprintf("fakeic7100: %s(%d, %d): %v", option, bank, channel, err))
	}
	return addr
}

func mustPositiveInterval(option string, d time.Duration) {
	if d <= 0 {
		panic(fmt.Sprintf("fakeic7100: %s(%v): the interval must be positive, or nothing would ever be emitted", option, d))
	}
}

func byteName(c byte) string {
	if c == preamble {
		return "preamble"
	}
	return "end-of-message"
}
