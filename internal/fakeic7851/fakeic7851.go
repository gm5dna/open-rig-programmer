package fakeic7851

import (
	"net"
	"sync"
	"time"
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
	host, fake                 net.Conn
	addr                       byte
	model                      string
	recordLen                  int
	emptyFF, echo, shortSetAck bool
	mu                         sync.Mutex
	slots                      map[int][]byte
	done                       chan struct{}
	once                       sync.Once
	wg                         sync.WaitGroup
}

func New(opts ...Option) *Radio {
	c := defaultConfig()
	for _, o := range opts {
		o(&c)
	}
	h, f := net.Pipe()
	r := &Radio{host: h, fake: f, addr: c.addr, model: c.model, recordLen: c.recordLen, emptyFF: c.emptyFF, echo: c.echo, shortSetAck: c.shortSetAck, slots: c.channels, done: make(chan struct{})}
	r.wg.Add(1)
	go r.serve()
	if c.flood > 0 {
		r.wg.Add(1)
		go r.floodLoop(0, c.flood)
	}
	if c.addressed > 0 {
		r.wg.Add(1)
		go r.floodLoop(0xe0, c.addressed)
	}
	return r
}
func (r *Radio) Port() net.Conn { return r.host }
func (r *Radio) Close() error {
	r.once.Do(func() { close(r.done); _ = r.fake.Close() })
	r.wg.Wait()
	return nil
}
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
	defer r.wg.Done()
	a := &reassembler{}
	b := make([]byte, 4096)
	for {
		n, e := r.fake.Read(b)
		if n > 0 {
			for _, f := range a.push(b[:n]) {
				r.dispatch(f)
			}
		}
		if e != nil {
			return
		}
	}
}
func (r *Radio) dispatch(f wireFrame) {
	// Echo is a property of the link, not of the addressing: a linked
	// USB/REMOTE pair echoes whatever it carried, including frames this radio
	// will not answer. So it happens before both filters below.
	if r.echo {
		_, _ = r.fake.Write(f.raw)
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
		_, _ = r.fake.Write(v)
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
	if len(rest) == 1 && rest[0] == 0xff || len(rest) != r.recordLen {
		if r.shortSetAck && len(rest) > 1 && !(len(rest) == 1 && rest[0] == 0xff) {
			return r.answer(f, 0xfb)
		}
		return r.answer(f, 0xfa)
	}
	r.mu.Lock()
	r.slots[ch] = append([]byte(nil), rest...)
	r.mu.Unlock()
	return r.answer(f, 0xfb)
}
func (r *Radio) floodLoop(to byte, d time.Duration) {
	defer r.wg.Done()
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			_, _ = r.fake.Write(buildFrame(to, r.addr, append([]byte{0x19, 0}, []byte(r.model)...)...))
		case <-r.done:
			return
		}
	}
}
