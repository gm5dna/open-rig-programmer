package fakeic7851

import (
	"net"
	"sync"
	"time"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

const RecordLen = 25
const NameLen = 10

// controllerAddr is the printed controller address; radioAddrDefault is the
// printed default shared by the IC-7851 and the IC-7850. scanEdgeP1 and
// scanEdgeP2 are the slot keys parseChannel gives "P1" and "P2": negative, so
// that they cannot collide with memory channels 1 to 99.
const (
	controllerAddr   byte = 0xe0
	radioAddrDefault byte = 0x8e
	scanEdgeP1            = -1
	scanEdgeP2            = -2
)

type MemState struct{ Raw []byte }

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
func (r *Radio) SetSlot(addr string, record []byte) {
	ch, ok := parseChannel(addr)
	if !ok || len(record) != r.recordLen {
		panic("fakeic7851: invalid slot")
	}
	r.mu.Lock()
	r.slots[ch] = append([]byte(nil), record...)
	r.mu.Unlock()
}
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
	// USB/REMOTE pair echoes whatever it carried, including frames this radio
	// will not answer. So it happens before both filters below.
	if r.echo {
		r.pipe.WriteNow(f.raw)
	}
	if f.to != r.addr {
		return
	}
	// A real transceiver answers the controller it is addressed by. Frames
	// from any other source — another radio's transceive traffic, a second
	// controller — are carried past in silence, never refused with FA.
	// TestOnlyTheControllerIsAnswered pins the silence and that the link
	// still serves 0xE0 afterwards.
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
	case 0x09, 0x0a, 0x0b:
		return r.answer(f, 0xfa)
	}
	return r.answer(f, 0xfa)
}

// selector decodes the two packed-BCD channel bytes ①,② printed on PDF p.263
// and transcribed as B row D1: 0001-0099 are memory channels 1 to 99, 0100 is
// programmed scan edge P1 and 0101 is P2. It returns the slot key that
// parseChannel produces for the same channel, so a frame and a SetSlot call
// name one record. TestScanEdgeSelectorsAddressP1AndP2 pins the whole space
// and TestSelectorsOutsideTheFlatSpaceAreRefused its edges.
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
			} else {
				return r.answer(f, 0xfa)
			}
		}
		return r.answer(f, append([]byte{0x1a, 0, p[1], p[2]}, b...)...)
	}
	// A short set, and the printed one-byte clear form, are both refused. The
	// open edge is registered as ic7851-write-ack-fb; a
	// WithShortSetAcknowledgement option modelled the other reading until
	// 06/09/2026 and nothing ever called it.
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
