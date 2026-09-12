// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7700

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
	channels         map[int][]byte
}
type Option func(*config)

func defaultConfig() config {
	return config{addr: radioAddrDefault, model: "IC-7700", recordLen: RecordLen, channels: make(map[int][]byte)}
}

// WithModelName sets the ASCII bytes a 19 00 request is answered with.
// Diagnostics only — see doc.go, register ic7700-id-token.
func WithModelName(name string) Option { return func(c *config) { c.model = name } }

// WithRadioAddress moves the address this radio answers to, and the `from`
// byte of every reply. Matrix §3.4: "the range is 01h to DFh".
func WithRadioAddress(addr byte) Option {
	return func(c *config) {
		if addr == 0xfe || addr == 0xfd {
			panic("fakeic7700: framing byte cannot be a radio address")
		}
		c.addr = addr
	}
}

// WithRecordLength overrides RecordLen (matrix §3.11's own derivation) for a
// consumer that needs to prove its own length handling without waiting on
// the matrix's arithmetic to settle further.
func WithRecordLength(n int) Option {
	return func(c *config) {
		if n < 1 {
			panic("fakeic7700: record length must be positive")
		}
		c.recordLen = n
	}
}

// WithEmptyReplyFA (the default) and WithAllFFEmpty select between the two
// separate ASSUMED empty-channel readings — matrix §3.8(a) and §3.8(b), and
// doc.go, "Empty channels".
func WithEmptyReplyFA() Option { return func(c *config) { c.emptyFF = false } }
func WithAllFFEmpty() Option   { return func(c *config) { c.emptyFF = true } }

// WithUSBEcho makes the radio echo every received frame back verbatim
// before any answer to it. See doc.go, register ic7700-no-echo-setting.
func WithUSBEcho() Option { return func(c *config) { c.echo = true } }

// WithTransceiveFlood starts a BROADCAST flood (`to` = 0x00) at
// construction. WithAddressedFlood starts a SYNTHETIC `to` = 0xE0 flood.
// See doc.go, "Echo and transceive broadcasts".
func WithTransceiveFlood(d time.Duration) Option { return func(c *config) { c.flood = d } }
func WithAddressedFlood(d time.Duration) Option  { return func(c *config) { c.addressed = d } }

// WithChannel seeds one channel at construction. addr is "001".."099", or
// "P1"/"P2". Panics on an invalid channel or a wrongly-sized record — see
// SetSlot.
func WithChannel(addr string, record []byte) Option {
	return func(c *config) {
		ch, ok := parseChannel(addr)
		if !ok {
			panic(fmt.Sprintf("fakeic7700: invalid channel %q", addr))
		}
		if len(record) != c.recordLen {
			panic(fmt.Sprintf("fakeic7700: channel %q record has %d bytes, want %d", addr, len(record), c.recordLen))
		}
		c.channels[ch] = append([]byte(nil), record...)
	}
}

// parseChannel maps a caller's channel name to the same slot key the wire
// selector decodes to, so SetSlot("P1") and a 1A 00 read of 0100 name one
// record. Memory channels are their three-digit decimal string ("001" ..
// "099"), matching the golden vector's own "001" read-record request.
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
