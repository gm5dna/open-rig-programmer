// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7200

import (
	"fmt"
	"strconv"
	"time"
)

type config struct {
	addr             byte
	model            string
	recordLen        int
	emptyFF          bool
	flood, addressed time.Duration
	channels         map[int][]byte
}
type Option func(*config)

func defaultConfig() config {
	return config{addr: radioAddrDefault, model: "IC-7200", recordLen: RecordLen, channels: make(map[int][]byte)}
}

// WithModelName sets the bytes a 19 00 request is answered with. The
// reply value is UNDOCUMENTED (matrix §3.12(i)); "IC-7200" is INVENTED,
// not read off the manual.
func WithModelName(name string) Option { return func(c *config) { c.model = name } }

// WithRadioAddress moves the address this radio listens for and answers
// from. Matrix §3.4: MANUAL-EVIDENCED that the address is user-changeable
// via the front-panel DIAL, 01h-7Fh.
func WithRadioAddress(addr byte) Option {
	return func(c *config) {
		if addr == 0xfe || addr == 0xfd {
			panic("fakeic7200: framing byte cannot be a radio address")
		}
		c.addr = addr
	}
}

// WithRecordLength overrides RecordLen (17, the matrix's corrected
// figure) — see doc.go, "Record length, and proving the alternative".
func WithRecordLength(n int) Option {
	return func(c *config) {
		if n < 1 {
			panic("fakeic7200: record length must be positive")
		}
		c.recordLen = n
	}
}

// WithEmptyReplyFA (the default) makes an unwritten channel answer NG.
// ASSUMED, matrix §3.8(a).
func WithEmptyReplyFA() Option { return func(c *config) { c.emptyFF = false } }

// WithAllFFEmpty models the alternative empty convention the matrix
// leaves equally open: an unwritten channel answers an all-0xFF record.
// ASSUMED, matrix §3.8(b) — this manual is silent on the question, more
// so than IC-7610's (no clear-list form is printed at all).
func WithAllFFEmpty() Option { return func(c *config) { c.emptyFF = true } }

// WithTransceiveFlood starts a broadcast (to=00) flood: a frame every d,
// modelling the ASSUMED `to=00` broadcast form of the transceive traffic
// this radio's manual states is ON at the factory (matrix §3.5).
func WithTransceiveFlood(d time.Duration) Option { return func(c *config) { c.flood = d } }

// WithAddressedFlood starts a SYNTHETIC controller-addressed (to=E0)
// flood: a frame every d, as though the radio were answering
// continuously. No document describes a radio doing this; it exists so a
// consumer that must survive a jabbering peer can be shown to — the same
// tier-wide convention fakeic7610 and fakeic7851 both carry.
func WithAddressedFlood(d time.Duration) Option { return func(c *config) { c.addressed = d } }

// WithChannel seeds one channel at construction. addr is "001".."199",
// "P1" or "P2".
func WithChannel(addr string, record []byte) Option {
	return func(c *config) {
		ch, ok := parseChannel(addr)
		if !ok {
			panic(fmt.Sprintf("fakeic7200: invalid channel %q", addr))
		}
		if len(record) != c.recordLen {
			panic(fmt.Sprintf("fakeic7200: channel %q record has %d bytes, want %d", addr, len(record), c.recordLen))
		}
		c.channels[ch] = append([]byte(nil), record...)
	}
}

// parseChannel maps a caller's channel name to the same slot key the wire
// selector decodes to, so SetSlot("P1") and a 1A 00 read of 0200 name one
// record. addr is a three-digit memory channel "001".."199", or "P1"/"P2"
// for the two scan edges (matrix §1 row 4, §1b banks).
func parseChannel(s string) (int, bool) {
	if s == "P1" {
		return scanEdgeP1, true
	}
	if s == "P2" {
		return scanEdgeP2, true
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && len(s) == 3 && n >= 1 && n <= 199
}
