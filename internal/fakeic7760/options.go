package fakeic7760

import (
	"fmt"
	"time"
)

// Every knob below that models an ASSUMED behaviour is named for the entry in
// the IC-7760 capability matrix's register that owns the assumption, and its
// comment names that entry. A reader of this list can therefore find the
// register entry, and a reader of the register can find the knob. The
// MANUAL-EVIDENCED facts — the B2/E0 address pair, the FB/FA codes, the
// selector codes, the ten-byte name — are not knobs and do not appear here.
//
// TestEveryModelledAssumptionIsReachableUnderItsRegisterName pins the mapping
// and the defaults.

type Option func(*config)
type config struct {
	id                            []byte
	addr                          byte
	echo                          bool
	emptyReply                    byte
	recordLen                     int
	latency, broadcast, addressed time.Duration
	channels                      map[int][]byte
}

func defaultConfig() config {
	return config{
		id:         []byte{0xA5},
		addr:       AddrRadio,
		recordLen:  RecordLen,
		emptyReply: CodeNG,
		channels:   make(map[int][]byte),
	}
}

// WithIDReply sets the data bytes a 19 00 request is answered with — register
// entry ic7760-id-reply. The guide prints the command with an empty Data cell
// and prints no reply value for it anywhere, so the default 0xA5 is invented,
// deliberately implausible, and lifts nothing.
func WithIDReply(v []byte) Option {
	return func(c *config) {
		if len(v) == 0 {
			panic("fakeic7760: empty identity token")
		}
		c.id = append([]byte(nil), v...)
	}
}

// WithIDToken is the older spelling of WithIDReply and does the same thing.
func WithIDToken(v []byte) Option { return WithIDReply(v) }

// WithRadioAddress moves the radio off B2. B2 itself is MANUAL-EVIDENCED (the
// data-format diagram captions it "Transceiver's default address"); what is
// ASSUMED is only that a Set-mode menu can change it — register entry
// ic7760-address-menu — which is why this knob exists at all.
func WithRadioAddress(v byte) Option {
	return func(c *config) {
		if v == 0xFE || v == 0xFD {
			panic(fmt.Sprintf("fakeic7760: reserved address %02X", v))
		}
		c.addr = v
	}
}

// WithEchoDefault turns the port's echo-back on or off — register entry
// ic7760-echo-default. That two per-port echo flags exist is
// MANUAL-EVIDENCED (1A 05 01 53 and 01 54); neither default is printed, and
// nothing printed says which side of the address filter an echo sits on.
// This fake reflects before filtering, which TestAForeignControllerIsIgnored
// pins.
func WithEchoDefault(on bool) Option { return func(c *config) { c.echo = on } }

// WithUSBEcho turns the echo on. Older spelling of WithEchoDefault(true).
func WithUSBEcho() Option { return WithEchoDefault(true) }

// WithEcho is the older spelling of WithEchoDefault.
func WithEcho(on bool) Option { return WithEchoDefault(on) }

// WithEmptyReplyFA sets what an unwritten channel is answered with —
// register entry ic7760-empty-reply-fa. The guide says nothing anywhere about
// reading a channel that has no contents; FA is the tier's assumption, and it
// is the only value this fake will model, so any other code panics rather
// than inventing a second answer.
func WithEmptyReplyFA(code byte) Option {
	return func(c *config) {
		if code != CodeNG {
			panic("fakeic7760: only FA is supported for empty channels")
		}
		c.emptyReply = code
	}
}

// WithRecordLength sets the memory record's length — register entry
// ic7760-record-length. Twenty-five is a derivation, not a printed total
// (TestRecordGeometryAndSelectors re-does the arithmetic), so a derivation
// that turns out to be wrong can be corrected here.
func WithRecordLength(n int) Option {
	return func(c *config) {
		if n < 1 {
			panic("fakeic7760: record length must be positive")
		}
		c.recordLen = n
	}
}

func WithLatency(d time.Duration) Option              { return func(c *config) { c.latency = d } }
func WithTransceiveFlood(d time.Duration) Option      { return func(c *config) { c.broadcast = d } }
func WithTransceiveBroadcasts(d time.Duration) Option { return WithTransceiveFlood(d) }
func WithAddressedFlood(d time.Duration) Option       { return func(c *config) { c.addressed = d } }

// WithChannel seeds a slot. The record's length is checked against the
// finished config in New, not here, so that this option and the two length
// options may be given in any order.
func WithChannel(s string, v []byte) Option {
	return func(c *config) {
		ch, ok := parseSlot(s)
		if !ok {
			panic("fakeic7760: invalid channel " + s)
		}
		c.channels[ch] = append([]byte(nil), v...)
	}
}
