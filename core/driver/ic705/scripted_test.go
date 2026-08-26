// SPDX-License-Identifier: GPL-3.0-or-later

package ic705

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// scriptedRadio is this package's SCRIPTED CI-V peer: a net.Pipe whose
// remote end parses the frames the driver writes and answers each one from
// a slot image, recording everything it received.
//
// It is deliberately NOT internal/fakeic705 (Task 13's independent radio,
// which does not exist yet and which this package must not lean on for its
// own unit tests): a fake radio models a radio's STATE and is the right
// tool for end-to-end agreement, whereas this answers per frame from a
// table and can therefore MISBEHAVE ON PURPOSE — answer for the wrong
// channel, serve a foreign record length, go silent mid-session, or
// saturate the line with traffic — which is exactly what the error paths
// need and what a self-consistent fake will never produce.
//
// THE SEMANTICS ARE THE MANUAL'S, APPLIED, NOT AN OBSERVED RADIO
// TRANSCRIBED. No IC-705 has ever been connected to this project. FB means
// accepted and FA means refused (PDF p.3, folio 2, the OK/NG messages);
// that an unwritten memory answers FA rather than a record of some kind is
// spec D5 entry 2(a), ASSUMED, lift L-EMPTY-FA — this helper applies the
// convention the driver was written against, and a capture, not this file,
// will settle it.
type scriptedRadio struct {
	host, remote net.Conn

	mu         sync.Mutex
	received   [][]byte
	records    map[civ.ChannelAddress][]byte
	idPayload  []byte
	silent     bool
	silentID   bool
	answerNext *civ.ChannelAddress
	closed     bool

	floodStop chan struct{}
	floodWG   sync.WaitGroup
}

// radioImage is what a scriptedRadio "contains" at construction.
type radioImage struct {
	// records maps a WIRE address to the raw record bytes served for a
	// read of it. A record of ANY length may be seeded — the
	// wrong-sibling probe needs a foreign one — because the length rule
	// governs what arrives on the wire as a SET, not what an operator
	// injects into the image.
	records map[civ.ChannelAddress][]byte
	// idPayload is the `19 00` answer's data area. Empty means the radio
	// answers 19 00 with no data at all, which is a legal envelope and an
	// unusable token.
	idPayload []byte
	// silentID makes the radio ignore `19 00` entirely — the probe's
	// "no address-matched reply" path.
	silentID bool
	// silent makes the radio ignore EVERY frame.
	silent bool
}

func newScriptedRadio(t *testing.T, img radioImage) *scriptedRadio {
	t.Helper()
	host, remote := net.Pipe()
	r := &scriptedRadio{
		host:      host,
		remote:    remote,
		records:   map[civ.ChannelAddress][]byte{},
		idPayload: img.idPayload,
		silent:    img.silent,
		silentID:  img.silentID,
		floodStop: make(chan struct{}),
	}
	if r.idPayload == nil {
		// An arbitrary token. The probe records it and matches it against
		// NOTHING (spec D5 entry 7, lift L-IDTOKEN), so its value is the
		// fake's own business — which is precisely what the
		// two-different-tokens test asserts.
		r.idPayload = []byte{0x94}
	}
	for a, rec := range img.records {
		r.records[a] = append([]byte(nil), rec...)
	}
	t.Cleanup(func() {
		// THE PIPES CLOSE FIRST, then the flood is waited on. net.Pipe is
		// unbuffered, so a flood goroutine can be parked inside Write with
		// nobody reading; signalling it and waiting first would deadlock
		// on exactly the test that needed a flood most. A closed pipe
		// makes that Write return, and the goroutine exits.
		close(r.floodStop)
		_ = host.Close()
		_ = remote.Close()
		r.floodWG.Wait()
	})
	go r.serve()
	return r
}

// Port returns the end handed to the driver. The driver takes ownership of
// it (Open closes it on failure, Session.Close on success), so a test must
// not close it itself.
func (r *scriptedRadio) Port() transport.Port { return r.host }

// Transcript returns a copy of every complete frame the radio received.
func (r *scriptedRadio) Transcript() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([][]byte, len(r.received))
	copy(out, r.received)
	return out
}

// CountCommand returns how many received frames carried cn/sc.
func (r *scriptedRadio) CountCommand(cn, sc byte) int {
	n := 0
	for _, f := range r.Transcript() {
		if len(f) >= 6 && f[4] == cn && f[5] == sc {
			n++
		}
	}
	return n
}

// Reads and Sets count the two `1A 00` shapes: a read carries the address
// alone, a set carries a record after it.
func (r *scriptedRadio) Reads() int { return r.countMemory(false) }
func (r *scriptedRadio) Sets() int  { return r.countMemory(true) }

func (r *scriptedRadio) countMemory(withRecord bool) int {
	n := 0
	for _, f := range r.Transcript() {
		if len(f) < 7 || f[4] != 0x1A || f[5] != 0x00 {
			continue
		}
		body := f[6 : len(f)-1]
		if (len(body) > 4) == withRecord {
			n++
		}
	}
	return n
}

// SetRecord seeds (or removes, with a nil record) one slot.
func (r *scriptedRadio) SetRecord(a civ.ChannelAddress, rec []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec == nil {
		delete(r.records, a)
		return
	}
	r.records[a] = append([]byte(nil), rec...)
}

// SlotState reports what the radio holds at a, so a test can prove a
// refusal left the radio's own bytes alone.
func (r *scriptedRadio) SlotState(a civ.ChannelAddress) ([]byte, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.records[a]
	return append([]byte(nil), rec...), ok
}

// GoSilent stops the radio answering anything — the read-timeout path.
func (r *scriptedRadio) GoSilent() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.silent = true
}

// AnswerNextReadWithAddress arms ONE misbehaving answer: the next memory
// read is answered with a well-formed record whose channel address is a
// rather than the one requested. T2's regression needs it, and a peer that
// cannot misbehave cannot prove the driver catches misbehaviour.
func (r *scriptedRadio) AnswerNextReadWithAddress(a civ.ChannelAddress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	addr := a
	r.answerNext = &addr
}

// StartBroadcastFlood emits unsolicited frames addressed to 0x00 — the
// transceive broadcast a CI-V bus carries constantly. The adapter drops
// them before the engine sees one, so they can never trip a drain cap;
// what they DO is fill the accumulator's Unexpected counter.
func (r *scriptedRadio) StartBroadcastFlood(period time.Duration) {
	r.flood(period, []byte{0xFE, 0xFE, 0x00, 0xA4, 0x00, 0x00, 0x50, 0x45, 0x01, 0xFD})
}

// StartAddressedFlood emits unsolicited frames addressed to the CONTROLLER
// — a `1B 00` repeater-tone answer, which is well formed, ours by address,
// and matches no spec this driver ever puts in flight. This is the ONLY
// flood shape that reaches a drain and can exhaust its cap.
func (r *scriptedRadio) StartAddressedFlood(period time.Duration) {
	r.flood(period, []byte{0xFE, 0xFE, 0xE0, 0xA4, 0x1B, 0x00, 0x00, 0x08, 0x85, 0xFD})
}

func (r *scriptedRadio) flood(period time.Duration, frame []byte) {
	r.floodWG.Add(1)
	go func() {
		defer r.floodWG.Done()
		t := time.NewTicker(period)
		defer t.Stop()
		for {
			select {
			case <-r.floodStop:
				return
			case <-t.C:
				r.mu.Lock()
				closed := r.closed
				r.mu.Unlock()
				if closed {
					return
				}
				if _, err := r.remote.Write(frame); err != nil {
					return
				}
			}
		}
	}()
}

// EmitBroadcast writes n unsolicited transceive broadcasts (to = 0x00)
// and returns once the far end has taken the bytes. The CI-V accumulator
// drops them before the engine sees one, so they are countable ONLY
// through the framing adapter's own stats.
func (r *scriptedRadio) EmitBroadcast(n int) error {
	frame := []byte{0xFE, 0xFE, 0x00, 0xA4, 0x00, 0x00, 0x50, 0x45, 0x01, 0xFD}
	for i := 0; i < n; i++ {
		if _, err := r.remote.Write(frame); err != nil {
			return err
		}
	}
	return nil
}

// serve reads the driver's bytes, splits them into FD-terminated frames,
// records each, and answers per the image.
func (r *scriptedRadio) serve() {
	buf := make([]byte, 256)
	var acc []byte
	for {
		n, err := r.remote.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			for {
				i := bytes.IndexByte(acc, 0xFD)
				if i < 0 {
					break
				}
				frame := append([]byte(nil), acc[:i+1]...)
				acc = acc[i+1:]
				r.record(frame)
				if reply := r.reply(frame); reply != nil {
					if _, werr := r.remote.Write(reply); werr != nil {
						return
					}
				}
			}
		}
		if err != nil {
			r.mu.Lock()
			r.closed = true
			r.mu.Unlock()
			return
		}
	}
}

func (r *scriptedRadio) record(frame []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.received = append(r.received, frame)
}

// nak and ack are the two six-byte answers the manual prints.
func nak() []byte { return []byte{0xFE, 0xFE, 0xE0, 0xA4, 0xFA, 0xFD} }
func ack() []byte { return []byte{0xFE, 0xFE, 0xE0, 0xA4, 0xFB, 0xFD} }

// reply answers one frame, or nil for silence.
func (r *scriptedRadio) reply(frame []byte) []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.silent {
		return nil
	}
	if len(frame) < 6 || frame[0] != 0xFE || frame[1] != 0xFE {
		return nil
	}
	// A CI-V radio answers only what is addressed to IT.
	if frame[2] != 0xA4 {
		return nil
	}
	cn, sc := frame[4], frame[5]
	body := frame[6 : len(frame)-1]
	switch {
	case cn == 0x19 && sc == 0x00:
		if r.silentID {
			return nil
		}
		out := []byte{0xFE, 0xFE, 0xE0, 0xA4, 0x19, 0x00}
		out = append(out, r.idPayload...)
		return append(out, 0xFD)
	case cn == 0x1A && sc == 0x00 && len(body) == 4:
		addr, err := decodeWireAddress(body)
		if err != nil {
			return nak()
		}
		answerFor := addr
		if r.answerNext != nil {
			answerFor = *r.answerNext
			r.answerNext = nil
		}
		rec, ok := r.records[answerFor]
		if !ok {
			// An unwritten channel answers FA (D5 entry 2(a), ASSUMED).
			return nak()
		}
		out := []byte{0xFE, 0xFE, 0xE0, 0xA4, 0x1A, 0x00}
		out = append(out, encodeWireAddress(answerFor)...)
		out = append(out, rec...)
		return append(out, 0xFD)
	case cn == 0x1A && sc == 0x00 && len(body) > 4:
		addr, err := decodeWireAddress(body[:4])
		if err != nil {
			return nak()
		}
		rec := body[4:]
		if len(rec) != 111 {
			// A record at a length this radio does not use is refused —
			// never accepted, never truncated.
			return nak()
		}
		r.records[addr] = append([]byte(nil), rec...)
		return ack()
	default:
		return nak()
	}
}

// encodeWireAddress renders a as this radio's four address bytes: two
// packed-BCD bytes of group, most significant first, then two of channel.
//
// WRITTEN OUT BY HAND, not taken from civ's encoder. A scripted peer that
// derived its address geometry from the code under test would agree with a
// wrong geometry as readily as with a right one.
func encodeWireAddress(a civ.ChannelAddress) []byte {
	return []byte{
		byte(a.Group/1000%10)<<4 | byte(a.Group/100%10),
		byte(a.Group/10%10)<<4 | byte(a.Group%10),
		byte(a.Channel/1000%10)<<4 | byte(a.Channel/100%10),
		byte(a.Channel/10%10)<<4 | byte(a.Channel%10),
	}
}

// decodeWireAddress is encodeWireAddress's inverse, refusing a byte whose
// nibbles are not decimal digits.
func decodeWireAddress(b []byte) (civ.ChannelAddress, error) {
	if len(b) != 4 {
		return civ.ChannelAddress{}, errBadAddress
	}
	var digits [8]int
	for i, x := range b {
		hi, lo := int(x>>4), int(x&0x0F)
		if hi > 9 || lo > 9 {
			return civ.ChannelAddress{}, errBadAddress
		}
		digits[2*i], digits[2*i+1] = hi, lo
	}
	g := digits[0]*1000 + digits[1]*100 + digits[2]*10 + digits[3]
	c := digits[4]*1000 + digits[5]*100 + digits[6]*10 + digits[7]
	return civ.ChannelAddress{Group: g, Channel: c}, nil
}

// errBadAddress is the scripted radio's own refusal of a malformed address
// field; the driver never sees it as an error, only as the FA it becomes.
var errBadAddress = errBadAddressType{}

type errBadAddressType struct{}

func (errBadAddressType) Error() string { return "scripted radio: malformed address field" }

// noSettleClock is the engine clock these tests inject: real time
// everywhere it matters and NO SLEEP for the post-exchange pacing delay.
//
// IT REMOVES PACING AND NOTHING ELSE. transport.Engine's only Sleep call is
// its 20 ms inter-exchange settle; every timeout, idle gap and drain cap
// still runs on the real clock through Now and After, so the drain,
// quarantine and timeout behaviour these tests assert is the production
// behaviour. Without it the inventory walk's 1 000 exchanges would cost 20
// seconds of pure pacing per test — which is a fact about a real serial
// link and a waste of a test suite's time.
type noSettleClock struct{}

func (noSettleClock) Now() time.Time                         { return time.Now() }
func (noSettleClock) After(d time.Duration) <-chan time.Time { return time.After(d) }
func (noSettleClock) Sleep(time.Duration)                    {}

// bcd2 renders n as two packed-BCD bytes, most significant first — the
// address field's own encoding, for tests that build a frame by hand.
func bcd2(n int) []byte {
	return []byte{byte(n/1000%10)<<4 | byte(n/100%10), byte(n/10%10)<<4 | byte(n%10)}
}
