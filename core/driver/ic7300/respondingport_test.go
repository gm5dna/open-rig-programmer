// SPDX-License-Identifier: GPL-3.0-or-later

package ic7300

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/codeplug"
	"github.com/gm5dna/open-rig-programmer/core/driver"
	"github.com/gm5dna/open-rig-programmer/core/spec"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// respondingPort is this package's scripted radio: a net.Pipe whose remote
// end PARSES the CI-V frames the driver writes and answers each one from a
// small configuration, recording every complete frame it received.
//
// IT IS DELIBERATELY NOT internal/fakeic7300. A fake radio models a radio's
// STATE and is the right tool for round-trip and end-to-end tests; this
// answers per-frame from a table and can therefore serve deliberately WRONG
// answers — a record of the SIBLING'S length, a record of a length no model
// declares, an identity reply addressed to somebody else, an answer naming
// the wrong channel, a stream that never goes quiet — which is exactly what
// the error paths need and what a self-consistent fake will never produce.
//
// IT IS ALSO NOT SHARED WITH core/driver/ic7300mk2, which has its own copy.
// A shared peer would be the sibling-borrowing both matrices' §4 forbid, and
// it would hide a wrong address byte: this one answers from 0x94 and only
// from 0x94.
//
// THE ACKNOWLEDGEMENT SEMANTICS ARE AN ASSUMED CONVENTION APPLIED, NOT AN
// OBSERVED RADIO TRANSCRIBED — no IC-7300 has ever been connected to this
// project. That an accepted 1A 00 set draws the exact six-byte FB and a
// refused one draws FA is doc.go's `ic7300-write-ack-form`; that a read of
// an unwritten channel draws FA is D5 entry 2(a). Both are register entries
// with named lifts, and this file APPLIES them so the driver can be
// exercised, it does not evidence them.
type respondingPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received [][]byte

	// idToken is the data area of the 19 00 answer. Its VALUE is
	// undocumented on every model in this tier, so the driver records it
	// and matches nothing against it; a test that wants to prove that
	// serves two different tokens.
	idToken []byte
	// records maps a channel number (1..99 for M-CH, 100 for P1, 101 for
	// P2) to the RAW record bytes served for a read of it. A channel absent
	// from the map is answered FA — the empty-channel convention.
	records map[int][]byte

	// misaddress maps a REQUESTED channel to the channel the answer's
	// address field names instead. Per-channel rather than global, so a
	// test can leave the open probe's own slots answering honestly and
	// misdirect one read.
	misaddress map[int]int

	misaddressedID bool
	silentReads    bool
	noAnswerToSets bool
	rejectSets     bool

	broadcastPeriod  time.Duration
	floodPeriod      time.Duration
	floodAfterFrames int

	closeOnce sync.Once
}

// peerOption configures a scripted radio.
type peerOption func(*respondingPort)

// The two CI-V addresses this scripted radio uses. WRITTEN OUT, not taken
// from the profile under test: a fixture that derived its address from the
// code under test would answer at whatever address that code asked for,
// including a wrong one.
const (
	peerRadioAddr      = 0x94
	peerControllerAddr = 0xE0
)

// populatedRecord is the 39 record bytes of this model's golden set vector
// (core/civ/ic7300/testdata/ic7300-vectors.golden, line
// "set-record-name-with-space"): SELECT OFF and Split OFF in ③, 14.250 MHz,
// USB, FIL1, data mode OFF, tone mode OFF, 88.5 Hz on both tone spans, the
// same frequency in the transmit block, and the name "TEST CHAN1".
//
// TAKEN FROM THE FROZEN VECTOR, not built by the codec under test: a
// fixture a builder produced would pin the parser against the builder, and
// the two would agree about a wrong offset just as happily as a right one.
var populatedRecord = []byte{
	0x00,                         // ③ — SELECT OFF (low nibble), Split OFF (high nibble)
	0x00, 0x00, 0x25, 0x14, 0x00, // ④–⑧ — 14 250 000 Hz, least significant pair first
	0x01,             // ⑨ — USB
	0x01,             // ⑩ — FIL1
	0x00,             // ⑪ — data mode OFF (high), tone mode OFF (low)
	0x00, 0x08, 0x85, // ⑫–⑭ — 88.5 Hz
	0x00, 0x08, 0x85, // ⑮–⑰ — 88.5 Hz
	0x00, 0x00, 0x25, 0x14, 0x00, // ❹–⑧ — the transmit frequency
	0x01,             // ❾
	0x01,             // ❿
	0x00,             // ⓫
	0x00, 0x08, 0x85, // ⓬–⓮
	0x00, 0x08, 0x85, // ⓯–⓱
	'T', 'E', 'S', 'T', ' ', 'C', 'H', 'A', 'N', '1', // ⑱–㉗
}

// newRespondingPort starts a scripted radio and registers its cleanup. The
// returned value IS the transport.Port a driver Opens.
func newRespondingPort(t *testing.T, opts ...peerOption) *respondingPort {
	t.Helper()
	host, remote := net.Pipe()
	p := &respondingPort{
		host:       host,
		remote:     remote,
		idToken:    []byte{0x00},
		records:    map[int][]byte{},
		misaddress: map[int]int{},
	}
	for _, opt := range opts {
		opt(p)
	}
	t.Cleanup(func() {
		_ = host.Close()
		_ = remote.Close()
	})
	go p.serve()
	if p.floodPeriod > 0 && p.floodAfterFrames == 0 {
		go p.flood(peerControllerAddr, p.floodPeriod)
	}
	if p.broadcastPeriod > 0 {
		go p.flood(0x00, p.broadcastPeriod)
	}
	return p
}

// Read, Write and Close make this a transport.Port: the driver's end of the
// pipe. The driver takes ownership of it, so a test never closes it —
// newRespondingPort's cleanup does.
func (p *respondingPort) Read(b []byte) (int, error)  { return p.host.Read(b) }
func (p *respondingPort) Write(b []byte) (int, error) { return p.host.Write(b) }
func (p *respondingPort) Close() error {
	var err error
	p.closeOnce.Do(func() { err = p.host.Close() })
	return err
}

// Received returns DEFENSIVE COPIES of every complete frame this radio saw,
// in arrival order. Copies, because the caller counts and compares them
// while the serve goroutine is still appending.
func (p *respondingPort) Received() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([][]byte, len(p.received))
	for i, f := range p.received {
		out[i] = append([]byte(nil), f...)
	}
	return out
}

// withIDToken sets the data area of the 19 00 answer. Default {0x00}.
func withIDToken(token []byte) peerOption {
	return func(p *respondingPort) { p.idToken = append([]byte(nil), token...) }
}

// withRecord makes channel occupied, answering a read of it with rec.
func withRecord(channel int, rec []byte) peerOption {
	return func(p *respondingPort) { p.records[channel] = append([]byte(nil), rec...) }
}

// withRecordOfLength makes channel occupied by a record of n bytes whose
// CONTENT is irrelevant — the length is the whole point. Zero bytes, not
// 0xFF, so a length test cannot be confused with the empty-record question.
func withRecordOfLength(channel int, n int) peerOption {
	return func(p *respondingPort) { p.records[channel] = make([]byte, n) }
}

// withAnswerAddressedElsewhere answers a read of channel with a record
// whose ADDRESS FIELD names answerChannel: the wrong-slot reply the
// driver's own address check exists to catch, since civ's memory-answer
// matcher is envelope-only and deliberately does not look at the channel.
func withAnswerAddressedElsewhere(channel, answerChannel int) peerOption {
	return func(p *respondingPort) { p.misaddress[channel] = answerChannel }
}

// withMisaddressedIDAnswer answers the 19 00 read with a frame addressed to
// a DIFFERENT controller. The accumulator's address filter drops it, so the
// probe sees silence — which is the point: an identity reply for somebody
// else is not this radio identifying itself.
func withMisaddressedIDAnswer() peerOption {
	return func(p *respondingPort) { p.misaddressedID = true }
}

// withSilentReads answers no 1A 00 READ at all: neither a record nor an FA.
// The timeout case for a read, as distinct from the empty-channel case.
func withSilentReads() peerOption {
	return func(p *respondingPort) { p.silentReads = true }
}

// withNoAnswerToSets never answers a 1A 00 set: the write-timeout case.
func withNoAnswerToSets() peerOption {
	return func(p *respondingPort) { p.noAnswerToSets = true }
}

// withRejectSets answers every 1A 00 set with FA: the attributable refusal.
func withRejectSets() peerOption {
	return func(p *respondingPort) { p.rejectSets = true }
}

// withBroadcasts emits unsolicited `to = 00` transceive frames forever.
// They are filtered by civ.FrameAccumulator's address check BEFORE any
// engine event, so they can never reach the drain cap — which is exactly
// what the (a) half of the flood pair asserts.
func withBroadcasts(period time.Duration) peerOption {
	return func(p *respondingPort) { p.broadcastPeriod = period }
}

// withAddressedFlood emits never-quiet frames addressed to the CONTROLLER,
// forever. These are NOT filtered: they become engine events, and they are
// the only thing that can reach DrainPolicy.Cap.
func withAddressedFlood(period time.Duration) peerOption {
	return func(p *respondingPort) { p.floodPeriod = period }
}

// withAddressedFloodAfter is withAddressedFlood delayed until the radio has
// received n frames, so a test can let the line be QUIET through Init and
// then start the flood — which is how the fail-closed half of the drain
// rule is driven, Init's own nonfatal drain having already succeeded.
func withAddressedFloodAfter(frames int, period time.Duration) peerOption {
	return func(p *respondingPort) {
		p.floodPeriod = period
		p.floodAfterFrames = frames
	}
}

// serve reads the driver's bytes, splits them at FD, records each frame and
// writes back whatever the configuration says.
//
// Frame splitting rather than whole-read matching: the transport writes one
// frame per call today, but nothing in the Port contract promises that.
func (p *respondingPort) serve() {
	buf := make([]byte, 256)
	var acc []byte
	for {
		n, err := p.remote.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			for {
				i := indexByte(acc, 0xFD)
				if i < 0 {
					break
				}
				frame := append([]byte(nil), acc[:i+1]...)
				acc = acc[i+1:]
				count := p.record(frame)
				if p.floodAfterFrames > 0 && count == p.floodAfterFrames {
					go p.flood(peerControllerAddr, p.floodPeriod)
				}
				if reply := p.reply(frame); reply != nil {
					if _, werr := p.remote.Write(reply); werr != nil {
						return
					}
				}
			}
		}
		if err != nil {
			return
		}
	}
}

func indexByte(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}

// record appends one received frame and returns how many have arrived.
func (p *respondingPort) record(frame []byte) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.received = append(p.received, frame)
	return len(p.received)
}

// flood writes a well-formed frame addressed to `to` every period until the
// pipe goes away. `to = 0x00` is the transceive broadcast form; `to = 0xE0`
// is the controller-addressed traffic the accumulator does not filter.
func (p *respondingPort) flood(to byte, period time.Duration) {
	// A `00` "send frequency data" frame carrying five BCD bytes: the shape
	// a transceive radio emits when its VFO moves.
	frame := []byte{0xFE, 0xFE, to, peerRadioAddr, 0x00, 0x00, 0x00, 0x25, 0x14, 0x00, 0xFD}
	for {
		if _, err := p.remote.Write(frame); err != nil {
			return
		}
		time.Sleep(period)
	}
}

// answer wraps body in a frame FROM this radio TO the controller.
func answerFrame(body ...byte) []byte {
	out := []byte{0xFE, 0xFE, peerControllerAddr, peerRadioAddr}
	out = append(out, body...)
	return append(out, 0xFD)
}

// reply returns the bytes this radio answers frame with, or nil for
// silence. See respondingPort's doc comment for which register entry holds
// each convention.
func (p *respondingPort) reply(frame []byte) []byte {
	// Only frames addressed to THIS radio from the controller are answered
	// at all; anything else is somebody else's traffic.
	if len(frame) < 6 || frame[0] != 0xFE || frame[1] != 0xFE || frame[2] != peerRadioAddr || frame[3] != peerControllerAddr {
		return nil
	}
	cn, sc := frame[4], frame[5]
	switch {
	case cn == 0x19 && sc == 0x00:
		if p.misaddressedID {
			// Addressed to a controller that is not ours.
			out := []byte{0xFE, 0xFE, 0xE1, peerRadioAddr, 0x19, 0x00}
			out = append(out, p.idToken...)
			return append(out, 0xFD)
		}
		body := []byte{0x19, 0x00}
		body = append(body, p.idToken...)
		return answerFrame(body...)

	case cn == 0x1A && sc == 0x00 && len(frame) == 9:
		// A READ: FE FE 94 E0 1A 00 <hi> <lo> FD.
		if p.silentReads {
			return nil
		}
		ch := bcdChannel(frame[6], frame[7])
		rec, ok := p.records[ch]
		if !ok {
			return answerFrame(0xFA)
		}
		hi, lo := frame[6], frame[7]
		if other, misdirect := p.misaddress[ch]; misdirect {
			hi, lo = bcdBytes(other)
		}
		body := []byte{0x1A, 0x00, hi, lo}
		body = append(body, rec...)
		return answerFrame(body...)

	case cn == 0x1A && sc == 0x00:
		// A SET.
		if p.noAnswerToSets {
			return nil
		}
		if p.rejectSets {
			return answerFrame(0xFA)
		}
		return answerFrame(0xFB)

	default:
		return answerFrame(0xFA)
	}
}

// bcdChannel decodes the two packed-BCD channel-address bytes, most
// significant pair first. Written out here rather than taken from the codec
// for the reason peerRadioAddr is: a fixture must not learn the encoding
// from the code it is testing.
func bcdChannel(hi, lo byte) int {
	return int(hi>>4)*1000 + int(hi&0x0F)*100 + int(lo>>4)*10 + int(lo&0x0F)
}

// bcdBytes is bcdChannel's inverse, for the answers this radio addresses
// deliberately wrongly.
func bcdBytes(n int) (hi, lo byte) {
	return byte((n/1000)%10<<4 | (n/100)%10), byte((n/10)%10<<4 | n%10)
}

// openSession opens a session against p and registers its cleanup.
//
// THE PROFILE IS RealHardware, deliberately. That is the fail-safe profile a
// real IC-7300 gets — nothing writable — so a test that passes
// WithConsentedUnverifiedWrites is exercising the consent transform as the
// user's own grant, and a test that does not is exercising the guard. A
// Simulated-profile session would make both cases pass for the wrong
// reason.
func openSession(t *testing.T, p transport.Port, opts ...Option) *Session {
	t.Helper()
	sess, err := New(RealHardware, opts...).Open(context.Background(), p, driver.Identity{Port: "test"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return sess
}

// channelFor is a fully-populated channel for slot, every field this
// record carries Known and every field it does not carry Unavailable.
//
// The values match populatedRecord, so a write of this channel against a
// peer holding that record changes nothing but exercises the whole
// choreography. The nine Unavailable fields are what make an ORDINARY write
// request only the seven unconditional fields plus nothing else: a Known
// value in any of them would be a request this radio cannot honour, and
// would be refused by name.
func channelFor(slot string) codeplug.Channel {
	return codeplug.Channel{
		Slot: slot,
		Data: &codeplug.ChannelData{
			FreqHz: 14_250_000,
			Mode:   "USB",
			// The Yaesu scalars, left at their zero values: this record
			// carries no clarifier, no ctcss_state and no shift, and a
			// non-zero value here would REQUEST a field the radio lacks.
			ClarHz: 0,
			RxClar: false,
			TxClar: false,
			CTCSS:  "",
			Shift:  "",

			CTCSSTone:  codeplug.ToneField{State: codeplug.Unavailable},
			Tag:        "TEST CHAN1",
			TagDisplay: codeplug.BoolField{State: codeplug.Unavailable},
			ScanSkip:   codeplug.BoolField{State: codeplug.Unavailable},

			TxFreqHz:     codeplug.FreqField{State: codeplug.Known, Value: 14_250_000},
			Duplex:       codeplug.StringField{State: codeplug.Unavailable},
			OffsetHz:     codeplug.FreqField{State: codeplug.Unavailable},
			ToneMode:     codeplug.StringField{State: codeplug.Known, Value: "OFF"},
			ToneTx:       codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)},
			ToneRx:       codeplug.ToneField{State: codeplug.Known, Value: spec.Tone(885)},
			DTCSCode:     codeplug.IntField{State: codeplug.Unavailable},
			DTCSPolarity: codeplug.StringField{State: codeplug.Unavailable},
			Filter:       codeplug.StringField{State: codeplug.Known, Value: "FIL1"},
			DataMode:     codeplug.BoolField{State: codeplug.Known, Value: false},
		},
	}
}
