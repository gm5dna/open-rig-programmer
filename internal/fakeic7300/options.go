// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7300

import (
	"fmt"
	"time"
)

// Option configures a *Radio at construction time. See New.
//
// Options are applied in the order given and each is applied exactly once,
// before any goroutine starts. An option that cannot be honoured PANICS rather
// than silently doing nothing: every one of them takes a value a caller writes
// as a literal in a test, so a bad one is a programming error that must stop
// the test loudly rather than hand back a radio quietly answering the wrong
// thing several layers from the typo.
type Option func(*Radio)

// WithLatency makes every REPLY the fake sends wait d before being written to
// the port — a per-exchange delay, applied once per answer. It does not delay
// echoes, transceive broadcasts or addressed floods, none of which is a reply
// to anything.
//
// The wait is interruptible: a Close during it abandons the reply and returns
// promptly, so a test may script a multi-second latency without a multi-second
// teardown.
func WithLatency(d time.Duration) Option {
	return func(r *Radio) { r.latency = d }
}

// WithIDToken fixes the data area of the answer to the identity read `19 00`.
// The answer is FE FE <controller> <this radio's address> 19 00 <token> FD.
//
// The token's VALUE is undocumented on this radio, so this fake has none of its
// own to offer and returns whatever it is told to (doc.go, ASSUMED entry 8).
// Without this option the token is one byte, the address the radio is
// configured with.
//
// It panics on an empty token: the identity answer must carry at least one data
// byte, and a bodyless one would be a different frame altogether.
func WithIDToken(token []byte) Option {
	return func(r *Radio) {
		if len(token) == 0 {
			panic("fakeic7300: WithIDToken needs at least one data byte — the identity answer is not a bodyless frame")
		}
		r.idToken = append([]byte(nil), token...)
	}
}

// WithEcho turns the bus echo on or off; it is off unless this option says
// otherwise. With it on, every COMPLETE frame the radio receives is written
// back to the port verbatim, before any answer to it — a CI-V bus is one wire,
// so a controller hears its own transmission come back.
//
// Every complete frame is echoed, including one addressed to some other radio:
// the echo is a property of the wire, not of who was being spoken to.
func WithEcho(on bool) Option {
	return func(r *Radio) { r.echo = on }
}

// WithTransceiveBroadcasts makes the radio emit an unsolicited frame addressed
// to 00 every period, forever, so that a caller can test that a continuous
// flood of traffic addressed to nobody does not wedge the radio. A period of
// zero or less emits nothing.
//
// Frames addressed to 00 are addressed to no particular station, so a host
// filtering on its own address discards them and they never reach anything
// waiting for an answer. That is the difference between this option and
// WithAddressedFlood.
func WithTransceiveBroadcasts(period time.Duration) Option {
	return func(r *Radio) { r.broadcastPeriod = period }
}

// WithAddressedFlood makes the radio emit an unsolicited frame addressed to the
// CONTROLLER (E0) every period, forever — never-quiet frames a host cannot
// dismiss as somebody else's, because they carry its own address. It is
// therefore the only traffic this fake produces that can reach a reading
// engine's drain cap, and the only way to exercise a nonfatal-at-Init path. A
// period of zero or less emits nothing.
//
// The frames are emitted whether or not anything has asked the radio for
// anything, and they continue during an exchange, so a host reading for an
// answer must expect them interleaved with it.
func WithAddressedFlood(period time.Duration) Option {
	return func(r *Radio) { r.floodPeriod = period }
}

// WithRadioAddress moves the address this radio is configured with away from
// its default, 94. Every byte of every answer that shows the radio's address
// follows it — the identity answer, the record answer, the OK frame and the NG
// frame alike — and the radio answers only frames whose `to` byte matches it.
//
// It is a simulator configuration for constructing a same-address collision in
// a test, and is NOT a claim that any radio behaves so.
//
// It panics on FE and FD, which are the preamble and end-of-message bytes: a
// radio addressed by either would put a reserved byte in the middle of every
// frame it sent, which no framer could recover.
func WithRadioAddress(addr byte) Option {
	return func(r *Radio) {
		if addr == preamble || addr == endOfMessage {
			panic(fmt.Sprintf("fakeic7300: WithRadioAddress(%02X): FE and FD are reserved framing bytes and cannot be a CI-V address", addr))
		}
		r.addr = addr
	}
}

// WithChannel seeds ONE slot with a record, as though it had already been
// written. addr is the canonical slot string — "001".."099", "P1" or "P2" — and
// record must be a record this radio would accept from the wire: exactly the
// length the diagram's field widths sum to, with every field whose values the
// page prints carrying one of them.
//
// It panics on anything else, so that a seeded fixture cannot quietly differ
// from what a set over the wire could have produced. Use WithRawChannel to seed
// deliberately foreign data.
//
// Several may be given; each overlays one slot on whatever is already stored.
func WithChannel(addr string, record []byte) Option {
	return func(r *Radio) {
		if _, _, ok := slotAddressBytes(addr); !ok {
			panic(fmt.Sprintf("fakeic7300: WithChannel(%q): not a channel address — the legend prints 001-099, P1 and P2, and no fourth form", addr))
		}
		if err := validateRecord(record); err != nil {
			panic(fmt.Sprintf("fakeic7300: WithChannel(%q): %v", addr, err))
		}
		r.channels[addr] = append([]byte(nil), record...)
	}
}

// WithRawChannel seeds one slot with a record of ANY length and ANY content,
// bypassing this package's own length and vocabulary checks, so that a caller
// can make the radio answer a read with a foreign-length record and see what
// the software under test does with it.
//
// addr is still validated — it is the map key a read has to reach, and a slot
// keyed by anything but one of the three printed forms could never be read
// back — so this option panics on an address WithChannel would have panicked
// on, and on that alone.
func WithRawChannel(addr string, record []byte) Option {
	return func(r *Radio) {
		if _, _, ok := slotAddressBytes(addr); !ok {
			panic(fmt.Sprintf("fakeic7300: WithRawChannel(%q): not a channel address — the legend prints 001-099, P1 and P2, and no fourth form", addr))
		}
		r.channels[addr] = append([]byte(nil), record...)
	}
}
