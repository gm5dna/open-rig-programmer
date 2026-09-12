package fakeic9100

import (
	"net"
	"sync"
	"time"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// RecordLen is the record-only width the band-byte ruling derives: 60-byte
// data area less the 3-byte AddressFormBankChannel selector (doc.go, matrix
// §3.11, wave spec §7).
const RecordLen = 57

// controllerAddr is the printed controller default; radioAddrDefault is
// THIS radio's own printed default, 7Ch — the matrix's headline finding
// (doc.go, matrix §3.4), not the 88h the wave spec originally assumed.
const (
	controllerAddr   byte = 0xe0
	radioAddrDefault byte = 0x7c
)

// Radio is a fake IC-9100 on the far end of a pipe.
type Radio struct {
	// pipe is internal/fakepipe: the net.Pipe pair and the goroutines
	// servicing it, protocol-free (see doc.go).
	pipe          *fakepipe.Pipe
	addr          byte
	model         string
	recordLen     int
	emptyFF, echo bool
	mu            sync.Mutex
	slots         map[int][]byte
}

func New(opts ...Option) *Radio {
	c := defaultConfig()
	for _, o := range opts {
		o(&c)
	}
	r := &Radio{pipe: fakepipe.New(), addr: c.addr, model: c.model, recordLen: c.recordLen, emptyFF: c.emptyFF, echo: c.echo, slots: c.channels}
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

// SetSlot stores a raw record at addr ("<band>-<channel>", e.g. "HF-001"),
// panicking on an invalid channel or a record of the wrong length.
func (r *Radio) SetSlot(addr string, record []byte) {
	ch, ok := parseChannel(addr)
	if !ok || len(record) != r.recordLen {
		panic("fakeic9100: invalid slot")
	}
	r.mu.Lock()
	r.slots[ch] = append([]byte(nil), record...)
	r.mu.Unlock()
}

// SlotState returns the record held for addr, and whether it is occupied.
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

// ClearSlot marks one channel unoccupied.
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

func (r *Radio) dispatch(f wireFrame) {
	// Echo is a property of the link, not of the addressing: a linked
	// USB/REMOTE pair echoes whatever it carried, including frames this
	// radio will not answer. So it happens before both filters below.
	if r.echo {
		r.pipe.WriteNow(f.raw)
	}
	if f.to != r.addr {
		return
	}
	// A real transceiver answers the controller it is addressed by. Frames
	// from any other source are carried past in silence, never refused
	// with FA.
	if f.from != controllerAddr {
		return
	}
	if v := r.handle(f); v != nil {
		r.pipe.WriteNow(v)
	}
}

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
	return r.answer(f, 0xfa)
}

// memory handles a 1A 00 frame. p is everything after the 0x1A command
// byte: the 00 sub-command, the 3-byte selector, and (for a set) the
// 57-byte record.
func (r *Radio) memory(f wireFrame, p []byte) []byte {
	if len(p) < 4 || p[0] != 0 {
		return r.answer(f, 0xfa)
	}
	ch, ok := selector(p[1:4])
	if !ok {
		return r.answer(f, 0xfa)
	}
	rest := p[4:]
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
				return r.answer(f, append([]byte{0x1a, 0, p[1], p[2], p[3]}, b...)...)
			}
			return r.answer(f, 0xfa)
		}
		return r.answer(f, append([]byte{0x1a, 0, p[1], p[2], p[3]}, b...)...)
	}
	// A short set, and the printed one-byte clear form, are both refused —
	// this tier sends no clear (doc.go).
	if len(rest) == 1 && rest[0] == 0xff || len(rest) != r.recordLen {
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
