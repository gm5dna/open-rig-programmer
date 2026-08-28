// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic9700

import (
	"fmt"
	"time"
)

// Option configures a fake radio at New time. Options are applied in the order
// they are given, so a later one that names the same channel as an earlier one
// wins.
type Option func(*config)

// seed is one channel an option asked for before the radio started answering.
type seed struct {
	addr     []byte
	record   []byte
	occupied bool
}

type config struct {
	seeds         []seed
	recordLength  int
	broadcasts    time.Duration
	flood         time.Duration
	answerAddress []byte
	echoBack      bool
}

// WithSlot seeds one memory channel with a raw record.
//
// The record is opaque to this package: it is stored and served back byte for
// byte, and nothing here interprets any part of it. The only constraint is a
// wire one — neither the preamble byte FE nor the end-of-message byte FD may
// appear inside a frame's data, so a record containing either could not be sent
// by any radio, and seeding one is a caller's mistake rather than a test case.
//
// band is 1 (144 MHz), 2 (430 MHz) or 3 (1.2 GHz); channel is 1 to 107, the
// printed extent of the memory channel number field. Anything else panics: the
// page prints no such channel, so there is nothing for the fake to be a fake OF.
func WithSlot(band, channel int, record []byte) Option {
	addr := mustChannelAddress("WithSlot", band, channel)
	rec := append([]byte(nil), record...)
	for i, c := range rec {
		if c == preamble || c == endOfMessage {
			panic(fmt.Sprintf("fakeic9700: WithSlot(%d, %d, …) [%s]: record byte %d is %02X, which is the %s byte and may never appear inside a frame's data", band, channel, describeAddress(addr), i, c, byteName(c)))
		}
	}
	return func(c *config) {
		c.seeds = append(c.seeds, seed{addr: addr, record: rec, occupied: true})
	}
}

// WithEmptySlot makes one channel answer the NG code.
//
// A channel that was never seeded answers NG anyway; this exists to say so on
// purpose, and to override an earlier WithSlot for the same channel.
func WithEmptySlot(band, channel int) Option {
	addr := mustChannelAddress("WithEmptySlot", band, channel)
	return func(c *config) {
		c.seeds = append(c.seeds, seed{addr: addr, occupied: false})
	}
}

// WithRecordLength makes every memory answer carry a record of n bytes,
// whatever was seeded — it changes the LENGTH of the answers seeded slots
// give; it does not by itself make any slot occupied, so a caller wanting a
// wrong-length ANSWER must also seed a slot the probe will reach (WithSlot).
//
// It also becomes the length a memory write must match: a write of any other
// length gets the NG code. Without it, and without any seeded slot to infer a
// length from, the fake enforces no length at all — this package has no opinion
// about how long an IC-9700 record is, and cannot acquire one (see doc.go, THE
// RECORD LENGTH STOP).
//
// Records shorter than n are padded with a byte this package invented (see
// recordPadByte); records longer than n are cut.
func WithRecordLength(n int) Option {
	if n < 0 {
		panic(fmt.Sprintf("fakeic9700: WithRecordLength(%d): a record cannot be a negative number of bytes", n))
	}
	return func(c *config) { c.recordLength = n }
}

// WithBroadcasts emits unsolicited to=00 frames every d. These are dropped by
// the controller's accumulator and never reach its engine.
func WithBroadcasts(d time.Duration) Option {
	mustPositiveInterval("WithBroadcasts", d)
	return func(c *config) { c.broadcasts = d }
}

// WithAddressedFlood emits frames addressed to the CONTROLLER every d. Unlike
// WithBroadcasts these DO reach the engine, which is what makes a drain cap
// reachable at all — the two species are not interchangeable.
func WithAddressedFlood(d time.Duration) Option {
	mustPositiveInterval("WithAddressedFlood", d)
	return func(c *config) { c.flood = d }
}

// WithAnswerAddress makes memory answers name a DIFFERENT channel address than
// the one requested, for the T2 mismatch regression test.
//
// The answer still carries the record the REQUESTED channel holds; only the
// three address bytes at the head of the answer's data block are replaced. That
// is the shape of the fault worth regressing against: an answer that looks
// perfectly well formed and belongs to the wrong channel.
func WithAnswerAddress(band, channel int) Option {
	addr := mustChannelAddress("WithAnswerAddress", band, channel)
	return func(c *config) { c.answerAddress = addr }
}

// WithEchoBack echoes every received frame before answering it.
//
// The echo is the frame as normalised on receipt — exactly two preamble bytes,
// however many arrived — because that is the frame the fake received, and it is
// what Transcript reports too.
func WithEchoBack() Option {
	return func(c *config) { c.echoBack = true }
}

func mustChannelAddress(option string, band, channel int) []byte {
	addr, err := channelAddress(band, channel)
	if err != nil {
		panic(fmt.Sprintf("fakeic9700: %s(%d, %d): %v", option, band, channel, err))
	}
	return addr
}

func mustPositiveInterval(option string, d time.Duration) {
	if d <= 0 {
		panic(fmt.Sprintf("fakeic9700: %s(%v): the interval must be positive, or nothing would ever be emitted", option, d))
	}
}

func byteName(c byte) string {
	if c == preamble {
		return "preamble"
	}
	return "end-of-message"
}
