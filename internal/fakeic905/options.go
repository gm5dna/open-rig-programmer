// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic905

import (
	"fmt"
	"time"
)

// Option configures a *Radio at construction time. See New.
//
// Options are applied in the order given, over whatever is already there, so a
// later one wins: WithEmpty(0, 3) after WithRecord(0, 3, …) leaves channel 3
// unoccupied, and the two written the other way round leave it occupied.
type Option func(*Radio)

// WithLatency makes every reply the fake sends wait d before being written to
// the port.
//
// THE WAIT DOES NOT BLOCK THE FAKE. The delay is scheduled, not slept through
// in the serve loop, so a scripted latency holds up the reply and NOTHING ELSE:
// either flood keeps emitting throughout it, further requests keep being read,
// and Close returns promptly rather than waiting the delay out. That is what
// TestTransceiveBroadcastsKeepArrivingWhileARequestIsOutstanding turns on, and
// it is the difference between "a radio that is slow to answer" and "a radio
// that has stopped".
//
// Replies are still written in the order they were produced.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) { r.latency = d }
}

// WithIdentityToken sets the data bytes the fake returns to 19 00.
//
// The DEFAULT is a fixed arbitrary token chosen by the constructor
// (defaultIdentityToken), because the reply VALUE is undocumented and a fake
// that implied one would be asserting a fact nobody has. This Option exists so
// a consumer can pin a DIFFERENT token and prove the driver RECORDS whatever it
// gets rather than matching a particular value.
//
// The bytes are copied, and an empty token is honoured as an empty token: the
// fake then answers FE FE E0 AC 19 00 FD, which is a legitimate thing for a
// consumer to want to see its driver cope with.
func WithIdentityToken(data []byte) Option {
	return func(r *Radio) {
		tok := make([]byte, len(data))
		copy(tok, data)
		r.identityToken = tok
	}
}

// WithRecord seeds one channel with RAW record bytes of ANY length — the fake
// never interprets them. This is the arbitrary-length ability the fingerprint
// and wrong-sibling tests need: a 64-byte record, a 65-byte one, or a 39-byte
// one that no IC-905 would ever send.
//
// group and channel are the two printed two-byte address fields, as numbers:
// group 0-99 are the memory channel groups and group 100 is the printed
// "01 00: Call channel group" (see bcd2, and doc.go register entry 6 — the page
// prints those values but states no encoding). Either outside 0-9999 panics.
//
// The bytes are copied. A record containing FE or FD is stored and returned
// exactly as given, unescaped, which will break the framing of the answer that
// carries it — deliberately: the fake holds no opinion about record content,
// and a consumer that seeds such a record is seeding one no radio could send.
func WithRecord(group, channel int, record []byte) Option {
	return func(r *Radio) {
		rec := make([]byte, len(record))
		copy(rec, record)
		r.records[addrOf(group, channel)] = MemState{Record: rec}
	}
}

// WithEmpty marks a channel unoccupied, so a read of it answers FA.
//
// A set to it is still accepted — there is no held length for a set to
// disagree with, so the channel is seeded at whatever length arrives.
func WithEmpty(group, channel int) Option {
	return func(r *Radio) {
		delete(r.records, addrOf(group, channel))
	}
}

// WithTransceiveBroadcasts makes the fake emit unsolicited frames addressed to
// 00 every interval, FOREVER, REGARDLESS of whether a request is pending — a
// radio that never goes quiet.
//
// frame is the COMPLETE frame to emit, byte for byte, and it must be addressed
// to 00: this Option's whole promise is which address the frames carry, so a
// frame that contradicts it PANICS at construction rather than quietly turning
// the test that used it into a test of something else.
//
// The broadcast form is itself ASSUMED — doc.go register entry 2, lift
// ic905-R-12. The reference prints four frames and none of them is a
// broadcast. The fake emits the form the tier's filter is designed for; no
// IC-905 has been observed emitting anything.
//
// A non-positive interval disables the flood.
func WithTransceiveBroadcasts(interval time.Duration, frame []byte) Option {
	f := floodFrame("WithTransceiveBroadcasts", broadcastAddr, frame)
	return func(r *Radio) {
		r.broadcastInterval = interval
		r.broadcastFrame = f
	}
}

// WithAddressedFlood emits frames addressed to the CONTROLLER (E0) every
// interval, forever, regardless of pending requests.
//
// IT IS A SEPARATE OPTION BECAUSE THE TWO FLOODS EXERCISE DIFFERENT CODE.
// Broadcast frames (to = 00) die at the controller's address filter and never
// reach the engine; controller-addressed frames do reach it, and are the only
// kind that can drive a drain to its cap. A single option with a caller-chosen
// address would let a test think it was exercising the second whilst exercising
// the first, which is exactly the confusion this pair exists to prevent — so
// the frame given here must be addressed to E0, and panics otherwise.
//
// This form is ASSUMED too — doc.go register entry 3. No IC-905 has been
// observed emitting anything at all.
//
// A non-positive interval disables the flood.
func WithAddressedFlood(interval time.Duration, frame []byte) Option {
	f := floodFrame("WithAddressedFlood", controllerAddr, frame)
	return func(r *Radio) {
		r.addressedInterval = interval
		r.addressedFrame = f
	}
}

// floodFrame validates and copies a flood frame.
//
// It PANICS on a frame that is not a frame, or that is addressed anywhere but
// to. Both are programming errors in the call that wrote them, both are visible
// at construction, and both would otherwise turn into a test that passes whilst
// proving something other than what it says. The alternative — silently
// rewriting the caller's address byte — is the sort of quiet fixing-up this
// package exists not to do.
func floodFrame(option string, to byte, frame []byte) []byte {
	if len(frame) < preambleLen+minBodyBytes+1 ||
		frame[0] != preambleByte || frame[1] != preambleByte ||
		frame[len(frame)-1] != endOfMessage {
		panic(fmt.Sprintf(
			"fakeic905: %s needs a COMPLETE frame — FE FE <to> <from> <cn> ... FD — and got % X",
			option, frame))
	}
	if frame[preambleLen] != to {
		panic(fmt.Sprintf(
			"fakeic905: %s emits frames addressed to %#02X, but the frame given is addressed to %#02X — the two floods are separate options precisely because that byte decides which of the consumer's code paths runs",
			option, to, frame[preambleLen]))
	}
	out := make([]byte, len(frame))
	copy(out, frame)
	return out
}
