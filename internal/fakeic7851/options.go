package fakeic7851

import (
	"fmt"
	"strconv"
	"time"
)

type config struct {
	addr             byte
	model            string
	recordLen        int
	emptyFF, echo    bool
	flood, addressed time.Duration
	shortSetAck      bool
	channels         map[int][]byte
}
type Option func(*config)

func defaultConfig() config {
	return config{addr: radioAddrDefault, model: "IC-7851", recordLen: RecordLen, channels: make(map[int][]byte)}
}
func WithModelName(name string) Option { return func(c *config) { c.model = name } }
func WithRadioAddress(addr byte) Option {
	return func(c *config) {
		if addr == 0xfe || addr == 0xfd {
			panic("fakeic7851: framing byte cannot be a radio address")
		}
		c.addr = addr
	}
}
func WithRecordLength(n int) Option {
	return func(c *config) {
		if n < 1 {
			panic("fakeic7851: record length must be positive")
		}
		c.recordLen = n
	}
}
func WithEmptyReplyFA() Option                   { return func(c *config) { c.emptyFF = false } }
func WithAllFFEmpty() Option                     { return func(c *config) { c.emptyFF = true } }
func WithUSBEcho() Option                        { return func(c *config) { c.echo = true } }
func WithTransceiveFlood(d time.Duration) Option { return func(c *config) { c.flood = d } }
func WithAddressedFlood(d time.Duration) Option  { return func(c *config) { c.addressed = d } }

// WithShortSetAcknowledgement makes the otherwise refused short-set edge
// explicit; the open behaviour is registered as ic7851-write-ack-fb.
func WithShortSetAcknowledgement() Option { return func(c *config) { c.shortSetAck = true } }
func WithChannel(addr string, record []byte) Option {
	return func(c *config) {
		ch, ok := parseChannel(addr)
		if !ok {
			panic(fmt.Sprintf("fakeic7851: invalid channel %q", addr))
		}
		if len(record) != c.recordLen {
			panic(fmt.Sprintf("fakeic7851: channel %q record has %d bytes, want %d", addr, len(record), c.recordLen))
		}
		c.channels[ch] = append([]byte(nil), record...)
	}
}

// parseChannel maps a caller's channel name to the same slot key that the wire
// selector decodes to, so SetSlot("P1") and a 1A 00 read of 0100 name one
// record. TestScanEdgeSelectorsAddressP1AndP2 pins that agreement.
func parseChannel(s string) (int, bool) {
	if s == "P1" {
		return scanEdgeP1, true
	}
	if s == "P2" {
		return scanEdgeP2, true
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && len(s) == 3 && n >= 1 && n <= 99
}
