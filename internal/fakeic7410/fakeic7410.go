// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7410

import (
	"net"
	"sync"
	"time"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// RecordLen is the one accepted length, in bytes, of the record that
// follows a 1A 00 channel selector. See doc.go, "The record: 40 bytes,
// not 25". WithRecordLength overrides it.
const RecordLen = 40

// NameLen is the width of the memory name field, in bytes: nine
// (matrix §1 row 7). Exported for a consumer building a record; nothing
// in this package writes it, since the record is opaque (see doc.go).
const NameLen = 9

// controllerAddr is the printed controller address (matrix §1 row 2);
// radioAddrDefault is the IC-7410's own printed default, 80h. scanEdgeP1
// and scanEdgeP2 are the slot keys parseChannel gives "P1" and "P2":
// negative, so that they cannot collide with memory channels 1 to 99.
const (
	controllerAddr   byte = 0xe0
	radioAddrDefault byte = 0x80
	scanEdgeP1            = -1
	scanEdgeP2            = -2
)

// MemState is one memory record, in wire order: exactly the bytes that
// follow the two channel-selector bytes in a 1A 00 frame. UNINTERPRETED —
// see doc.go.
type MemState struct{ Raw []byte }

type Radio struct {
	// pipe is internal/fakepipe: the net.Pipe pair and the goroutines
	// servicing it, protocol-free (see doc.go).
	pipe      *fakepipe.Pipe
	addr      byte
	model     string
	recordLen int
	emptyFF   bool
	mu        sync.Mutex
	slots     map[int][]byte
}

func New(opts ...Option) *Radio {
	c := defaultConfig()
	for _, o := range opts {
		o(&c)
	}
	r := &Radio{pipe: fakepipe.New(), addr: c.addr, model: c.model, recordLen: c.recordLen, emptyFF: c.emptyFF, slots: c.channels}
	r.serve()
	if c.flood > 0 {
		r.pipe.Go(func() { r.floodLoop(0, c.flood) })
	}
	if c.addressed > 0 {
		r.pipe.Go(func() { r.floodLoop(0xe0, c.addressed) })
	}
	return r
}

func (r *Radio) Port() net.Conn { return r.pipe.Host() }
func (r *Radio) Close() error   { return r.pipe.Close() }

// SetSlot seeds one channel with a record, as though it had been written
// over the wire. addr is "001".."099", "P1" or "P2" (parseChannel). It
// PANICS on an unaddressable channel or a record of the wrong length —
// both are programming errors in a test, not something this fake should
// quietly accept.
func (r *Radio) SetSlot(addr string, record []byte) {
	ch, ok := parseChannel(addr)
	if !ok || len(record) != r.recordLen {
		panic("fakeic7410: invalid slot")
	}
	r.mu.Lock()
	r.slots[ch] = append([]byte(nil), record...)
	r.mu.Unlock()
}

// SlotState returns the record stored for addr, and whether it is set at
// all. An unaddressable addr reports not-set.
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
// empty behaviour.
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

// dispatch applies the two address filters doc.go describes: this radio
// answers only a frame addressed to it (f.to == r.addr) from the
// controller (f.from == controllerAddr). Anything else is carried past in
// silence — TestOnlyTheControllerIsAnswered.
func (r *Radio) dispatch(f wireFrame) {
	if f.to != r.addr {
		return
	}
	if f.from != controllerAddr {
		return
	}
	if v := r.handle(f); v != nil {
		r.pipe.WriteNow(v)
	}
}

// handle answers this fake's tiny surface (doc.go, "What this radio
// answers, and what it refuses") and refuses everything else with NG.
func (r *Radio) handle(f wireFrame) []byte {
	if len(f.data) < 1 {
		return r.answer(f, 0xfa)
	}
	switch f.data[0] {
	case 0x19:
		if len(f.data) == 2 && f.data[1] == 0 {
			return r.answer(f, append([]byte{0x19, 0}, []byte(r.model)...)...)
		}
	case 0x1a:
		return r.memory(f, f.data[1:])
	}
	// 0x0b (Memory clear) and anything else this package does not model:
	// all refused, per doc.go.
	return r.answer(f, 0xfa)
}

// selector decodes the two packed-BCD channel bytes ①,② (matrix §1 row 5,
// §1b): 0001-0099 are memory channels 1 to 99, 0100 is programmed scan
// edge P1 and 0101 is P2. It returns the slot key parseChannel produces
// for the same channel, so a frame and a SetSlot call name one record.
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
	// A short set, and the printed one-byte clear form (③: FF, matrix
	// §3.13), are both refused — ASSUMED, matrix §3.10, and the tier's
	// FieldErase-is-never-consented convention respectively. Unlike a
	// short/long-but-wrong-length record, the clear form is length 1, so
	// it already fails the length check below; it is named here only to
	// document the decision, per doc.go.
	if len(rest) != r.recordLen {
		return r.answer(f, 0xfa)
	}
	r.mu.Lock()
	r.slots[ch] = append([]byte(nil), rest...)
	r.mu.Unlock()
	return r.answer(f, 0xfb)
}

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
