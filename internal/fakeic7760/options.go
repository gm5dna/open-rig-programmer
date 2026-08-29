package fakeic7760

import (
	"fmt"
	"time"
)

type Option func(*config)
type config struct {
	id                            []byte
	addr                          byte
	echo                          bool
	allowShort                    bool
	emptyReply                    byte
	recordLen                     int
	latency, broadcast, addressed time.Duration
	channels                      map[int][]byte
}

func defaultConfig() config {
	return config{id: []byte{0xA5}, addr: AddrRadio, recordLen: RecordLen, emptyReply: CodeNG, channels: make(map[int][]byte)}
}
func WithIDToken(v []byte) Option {
	return func(c *config) {
		if len(v) == 0 {
			panic("fakeic7760: empty identity token")
		}
		c.id = append([]byte(nil), v...)
	}
}
func WithRadioAddress(v byte) Option {
	return func(c *config) {
		if v == 0xFE || v == 0xFD {
			panic(fmt.Sprintf("fakeic7760: reserved address %02X", v))
		}
		c.addr = v
	}
}
func WithUSBEcho() Option     { return func(c *config) { c.echo = true } }
func WithEcho(on bool) Option { return func(c *config) { c.echo = on } }
func WithEmptyReply(code byte) Option {
	return func(c *config) {
		if code != CodeNG {
			panic("fakeic7760: only FA is supported for empty channels")
		}
		c.emptyReply = code
	}
}
func WithAllowShortSet(on bool) Option { return func(c *config) { c.allowShort = on } }
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
func WithChannel(s string, v []byte) Option {
	return func(c *config) {
		ch, ok := parseSlot(s)
		if !ok {
			panic("fakeic7760: invalid channel " + s)
		}
		if len(v) != c.recordLen {
			panic("fakeic7760: channel record has wrong length")
		}
		c.channels[ch] = append([]byte(nil), v...)
	}
}
