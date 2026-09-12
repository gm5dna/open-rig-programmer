// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7700

import (
	"net"
	"sync"
	"time"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// RecordLen is the one accepted length, in bytes, of the record that follows
// a 1A 00 channel selector. See doc.go, "Record length: 39 bytes".
const RecordLen = 39

// NameLen is the memory-name field's width; NamePad the padding byte this
// package assumes but does not enforce. See doc.go, "Record length".
const (
	NameLen      = 10
	NamePad byte = 0x20
)

// controllerAddr is the printed controller address; radioAddrDefault is the
// IC-7700's printed factory default (matrix §3.4). scanEdgeP1 and
// scanEdgeP2 are the slot keys parseChannel gives "P1" and "P2": negative,
// so that they cannot collide with a memory channel.
const (
	controllerAddr   byte = 0xe0
	radioAddrDefault byte = 0x74
	scanEdgeP1            = -1
	scanEdgeP2            = -2
)

// MemState is one memory record, in wire order — the RecordLen bytes that
// follow the two channel-selector bytes in a 1A 00 frame. Raw is
// UNINTERPRETED: this package parses no field of it. See doc.go.
type MemState struct{ Raw []byte }

// Radio is a simulated IC-7700: an in-memory duplex pipe presenting the host
// end via Port(), serviced from the Radio's own goroutines using this
// package's independent frame parser (parser.go).
type Radio struct {
	// pipe is internal/fakepipe: the net.Pipe pair and the goroutines
	// servicing it, protocol-free (see doc.go).
	pipe *fakepipe.Pipe

	addr          byte
	model         string
	recordLen     int
	emptyFF, echo bool

	mu    sync.Mutex
	slots map[int][]byte
}

// New constructs a simulated IC-7700 and starts its servicing goroutines.
// No channel is seeded: a read of any channel answers the configured empty
// behaviour until something sets it, over the wire or via SetSlot.
func New(opts ...Option) *Radio {
	c := defaultConfig()
	for _, o := range opts {
		o(&c)
	}
	r := &Radio{
		pipe:      fakepipe.New(),
		addr:      c.addr,
		model:     c.model,
		recordLen: c.recordLen,
		emptyFF:   c.emptyFF,
		echo:      c.echo,
		slots:     c.channels,
	}
	r.serve()
	if c.flood > 0 {
		r.pipe.Go(func() { r.floodLoop(0, c.flood) })
	}
	if c.addressed > 0 {
		r.pipe.Go(func() { r.floodLoop(0xe0, c.addressed) })
	}
	return r
}

// Port returns the host end of the fake's in-memory duplex connection.
func (r *Radio) Port() net.Conn { return r.pipe.Host() }

// Close shuts the fake radio down: stops any floods (via the pipe closing)
// and waits for every goroutine to exit. Safe to call more than once.
func (r *Radio) Close() error { return r.pipe.Close() }

// SetSlot seeds one channel with a record, as though it had been written
// over the wire. addr is a memory channel "001".."099", or "P1"/"P2".
//
// It PANICS on a channel this radio cannot address, or a record whose
// length is not RecordLen (or the WithRecordLength override): both are
// programming errors in a test that would otherwise surface several layers
// away as a puzzling NG.
func (r *Radio) SetSlot(addr string, record []byte) {
	ch, ok := parseChannel(addr)
	if !ok || len(record) != r.recordLen {
		panic("fakeic7700: invalid slot")
	}
	r.mu.Lock()
	r.slots[ch] = append([]byte(nil), record...)
	r.mu.Unlock()
}

// SlotState returns the record stored for addr, and whether that channel is
// set at all. An unaddressable addr reports not-set rather than panicking.
func (r *Radio) SlotState(addr string) (MemState, bool) {
	ch, ok := parseChannel(addr)
	if !ok {
		return MemState{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.slots[ch]
	return MemState{Raw: append([]byte(nil), b...)}, ok
}

// ClearSlot makes addr unset, so that a read of it answers the configured
// empty behaviour again. This is the Go-side control the wire deliberately
// does not offer: both printed clear forms (the inline-FF form and 0B) are
// REFUSED on this radio's wire, on purpose — see doc.go.
func (r *Radio) ClearSlot(addr string) {
	if ch, ok := parseChannel(addr); ok {
		r.mu.Lock()
		delete(r.slots, ch)
		r.mu.Unlock()
	}
}

func (r *Radio) serve() {
	a := &reassembler{}
	r.pipe.Go(func() {
		r.pipe.ReadLoop(func(b []byte) {
			for _, f := range a.push(b) {
				r.dispatch(f)
			}
		})
	})
}

// dispatch applies the echo and the address filter, in that order — an echo
// is a property of the line, not of the addressing, so it happens before
// both filters below. See doc.go, "Echo".
func (r *Radio) dispatch(f wireFrame) {
	if r.echo {
		r.pipe.WriteNow(f.raw)
	}
	if f.to != r.addr {
		return
	}
	// A real transceiver answers the controller it is addressed by. Frames
	// from any other source are carried past in silence, never refused with
	// NG — matching every sibling fake's own "only the controller is
	// answered" convention, which this matrix does not itself contradict.
	if f.from != controllerAddr {
		return
	}
	if v := r.handle(f); v != nil {
		r.pipe.WriteNow(v)
	}
}

// handle decides what one addressed, controller-sourced frame is answered
// with. A nil return is silence (never reached today: every path below
// answers something).
func (r *Radio) handle(f wireFrame) []byte {
	if len(f.data) < 1 {
		return r.answer(f, 0xfa)
	}
	switch f.data[0] {
	case 0x19:
		// The command is MANUAL-EVIDENCED (matrix §3.12); its reply value is
		// not printed anywhere. See doc.go, register ic7700-id-token.
		if len(f.data) == 2 && f.data[1] == 0 {
			return r.answer(f, append([]byte{0x19, 0}, []byte(r.model)...)...)
		}
	case 0x1a:
		return r.memory(f, f.data[1:])
	case 0x0b:
		// "Memory clear" (matrix §3.13). Refused deliberately — see doc.go.
		return r.answer(f, 0xfa)
	}
	return r.answer(f, 0xfa)
}

// selector decodes the two packed-BCD channel bytes printed on PDF p.213
// (matrix §1 row 5): 0001-0099 are memory channels 1 to 99, 0100 is
// programmed scan edge P1 and 0101 is P2. It returns the slot key
// parseChannel produces for the same channel, so a frame and a SetSlot call
// name one record.
func selector(b []byte) (int, bool) {
	if len(b) != 2 {
		return 0, false
	}
	n := 0
	for _, d := range []byte{b[0] >> 4, b[0] & 15, b[1] >> 4, b[1] & 15} {
		if d > 9 {
			return 0, false
		}
		n = n*10 + int(d)
	}
	switch {
	case n >= 1 && n <= 99:
		return n, true
	case n == 100:
		return scanEdgeP1, true
	case n == 101:
		return scanEdgeP2, true
	}
	return 0, false
}

// memory answers the 1A family. p is everything after the 1A command byte:
// the sub-command byte followed by the selector and any record.
//
// Only sub-command 00 is answered — 1A 05 (the menu surface this tier does
// not ship) and every other sub-command are refused, along with a
// selector that addresses nothing.
func (r *Radio) memory(f wireFrame, p []byte) []byte {
	if len(p) < 3 || p[0] != 0 {
		return r.answer(f, 0xfa)
	}
	ch, ok := selector(p[1:3])
	if !ok {
		return r.answer(f, 0xfa)
	}
	rest := p[3:]
	if len(rest) == 0 {
		r.mu.Lock()
		b, exists := r.slots[ch]
		b = append([]byte(nil), b...)
		r.mu.Unlock()
		if !exists {
			if r.emptyFF {
				b = make([]byte, r.recordLen)
				for i := range b {
					b[i] = 0xff
				}
				return r.answer(f, append([]byte{0x1a, 0, p[1], p[2]}, b...)...)
			}
			return r.answer(f, 0xfa)
		}
		return r.answer(f, append([]byte{0x1a, 0, p[1], p[2]}, b...)...)
	}
	// A short set, and the printed single-byte clear form ("...FF"), are
	// both refused — matrix §3.13 and §3.10; see doc.go.
	if len(rest) == 1 && rest[0] == 0xff || len(rest) != r.recordLen {
		return r.answer(f, 0xfa)
	}
	r.mu.Lock()
	r.slots[ch] = append([]byte(nil), rest...)
	r.mu.Unlock()
	return r.answer(f, 0xfb)
}

// floodLoop emits one ID-answer-shaped frame every d, addressed to `to`.
// See doc.go, "Echo and transceive broadcasts".
func (r *Radio) floodLoop(to byte, d time.Duration) {
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			r.pipe.WriteNow(buildFrame(to, r.addr, append([]byte{0x19, 0}, []byte(r.model)...)...))
		case <-r.pipe.Done():
			return
		}
	}
}
