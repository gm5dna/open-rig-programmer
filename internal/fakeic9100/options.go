package fakeic9100

import (
	"fmt"
	"time"
)

// Option configures a fake radio at New time. Every option that changes an
// answer exists because the matrix leaves something open (doc.go's assumed
// register); a default is this package's reading of the manual, the option
// is the other admissible one.
type Option func(*config)

type config struct {
	addr             byte
	model            string
	recordLen        int
	emptyFF, echo    bool
	flood, addressed time.Duration
	channels         map[int][]byte
}

func defaultConfig() config {
	return config{addr: radioAddrDefault, model: "IC-9100", recordLen: RecordLen, channels: make(map[int][]byte)}
}

// WithModelName sets the string the 19 00 identity reply carries. Its
// value is undocumented (doc.go, ic9100-id-reply-value); this option lets a
// consumer pin a different one and prove its driver records whatever it
// gets rather than matching a particular value.
func WithModelName(name string) Option { return func(c *config) { c.model = name } }

// WithRadioAddress puts this radio at a CI-V address other than 7Ch. CI-V
// Address is a front-panel SET MODE item on a real IC-9100 (matrix §1 row
// 2, §3.4), so a radio answering elsewhere is a radio, not a fault.
func WithRadioAddress(addr byte) Option {
	return func(c *config) {
		if addr == 0xfe || addr == 0xfd {
			panic("fakeic9100: framing byte cannot be a radio address")
		}
		c.addr = addr
	}
}

// WithRecordLength changes the record length a memory SET must carry, away
// from the 57 bytes the band-byte ruling derives (doc.go). It exists for a
// driver's record-length fingerprint test to be tried against a radio of
// another length.
func WithRecordLength(n int) Option {
	return func(c *config) {
		if n < 1 {
			panic("fakeic9100: record length must be positive")
		}
		c.recordLen = n
	}
}

// WithEmptyReplyFA is the default: a read of an unoccupied channel is
// answered FA (doc.go, ic9100-empty-channel-fa). It exists to be named
// explicitly against WithAllFFEmpty.
func WithEmptyReplyFA() Option { return func(c *config) { c.emptyFF = false } }

// WithAllFFEmpty answers a read of an unoccupied channel with a full-length
// record of FF bytes instead of FA (doc.go, ic9100-all-ff-record).
func WithAllFFEmpty() Option { return func(c *config) { c.emptyFF = true } }

// WithUSBEcho echoes every received frame back before answering it (doc.go,
// ic9100-echo-default). The default is no echo.
func WithUSBEcho() Option { return func(c *config) { c.echo = true } }

// WithTransceiveFlood emits unsolicited identity frames addressed to 00
// every d (doc.go, ic9100-broadcast-address-form).
func WithTransceiveFlood(d time.Duration) Option { return func(c *config) { c.flood = d } }

// WithAddressedFlood emits the same identity content addressed to the
// controller (0xE0) every d — a separate option because a to=00 broadcast
// and a controller-addressed frame exercise different code in a consumer.
func WithAddressedFlood(d time.Duration) Option { return func(c *config) { c.addressed = d } }

// WithChannel seeds one memory channel before the radio starts answering.
// addr is the "<band>-<channel>" spelling parseChannel accepts, e.g.
// "HF-001", "1200-099". The record is OPAQUE beyond its length: nothing
// here decodes a frequency, a name or a D-STAR call sign out of it.
func WithChannel(addr string, record []byte) Option {
	return func(c *config) {
		ch, ok := parseChannel(addr)
		if !ok {
			panic(fmt.Sprintf("fakeic9100: invalid channel %q", addr))
		}
		if len(record) != c.recordLen {
			panic(fmt.Sprintf("fakeic9100: channel %q record has %d bytes, want %d", addr, len(record), c.recordLen))
		}
		c.channels[ch] = append([]byte(nil), record...)
	}
}
