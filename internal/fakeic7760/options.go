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
	fullRecord                    bool
	emptyReply                    byte
	emptyRecordFF                 bool
	recordLen                     int
	scanEdgeLen                   int
	broadcastTo                   byte
	latency, broadcast, addressed time.Duration
	channels                      map[int][]byte
}

func defaultConfig() config {
	return config{
		id:            []byte{0xA5},
		addr:          AddrRadio,
		recordLen:     RecordLen,
		emptyReply:    CodeNG,
		emptyRecordFF: true,
		fullRecord:    true,
		broadcastTo:   AddrBroadcast,
		channels:      make(map[int][]byte),
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

// WithBroadcastForm sets the to byte unsolicited frames carry — register
// entry ic7760-broadcast-form. The guide prints no broadcast frame at all;
// the only answer-direction skeleton it draws has to=E0, and the tier's
// assumption of to=00 is this fake's default and nothing more.
func WithBroadcastForm(to byte) Option { return func(c *config) { c.broadcastTo = to } }

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

// WithEmptyReply is the older spelling of WithEmptyReplyFA.
func WithEmptyReply(code byte) Option { return WithEmptyReplyFA(code) }

// WithEmptyReplyFF turns off the reading of a stored all-FF record as an
// empty channel — register entry ic7760-empty-reply-ff, and a SEPARATE
// question from the outbound clear form. The only FF the guide prints in the
// memory context is a value the controller SENDS to erase; nothing licenses
// reading it backwards, so a consumer that wants an all-FF record handed back
// unread says so here. Refusing the printed one-byte clear form is not this
// knob and is not switchable — see PROVENANCE.md and
// TestThePrintedClearFormIsRefusedIndependentlyOfShortSets.
func WithEmptyReplyFF(on bool) Option { return func(c *config) { c.emptyRecordFF = on } }

// WithWriteFullRecord sets whether a 1A 00 set must carry the whole layout —
// register entry ic7760-write-full-record. The guide prints no statement
// permitting a short set; the tier sends the full layout always, which is safe
// under either answer, so this fake insists by default and can be told not to.
func WithWriteFullRecord(on bool) Option { return func(c *config) { c.fullRecord = on } }

// WithAllowShortSet is the older, inverted spelling of WithWriteFullRecord.
func WithAllowShortSet(on bool) Option { return WithWriteFullRecord(!on) }

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

// WithScanEdgeRecordShape gives P1 and P2 a record length of their own —
// register entry ic7760-scan-edge-record-shape. That a 1A 00 read of 01 00 or
// 01 01 returns the same record-only shape as a memory channel is ASSUMED;
// unset, the scan edges follow WithRecordLength, which is that assumption.
func WithScanEdgeRecordShape(n int) Option {
	return func(c *config) {
		if n < 1 {
			panic("fakeic7760: scan-edge record length must be positive")
		}
		c.scanEdgeLen = n
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

// recordLenFor is the accepted length for one slot: the scan edges take their
// own shape when one has been set, every memory channel takes the record
// length.
func (c config) recordLenFor(ch int) int {
	if (ch == ChanP1 || ch == ChanP2) && c.scanEdgeLen > 0 {
		return c.scanEdgeLen
	}
	return c.recordLen
}
