package fakeic7851

import (
	"net"
	"sync"
	"time"
)

const RecordLen = 25
const NameLen = 10

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
	if r.echo {
		_, _ = r.fake.Write(f.raw)
	}
	if f.to != r.addr {
		return
	}
	if v := r.handle(f); v != nil {
		_, _ = r.fake.Write(v)
	}
}
func (r *Radio) handle(f wireFrame) []byte {
	if len(f.data) < 1 {
		return buildAnswer(0xfa)
	}
	switch f.data[0] {
	case 0x19:
		if len(f.data) == 2 && f.data[1] == 0 {
			return buildAnswer(append([]byte{0x19, 0}, []byte(r.model)...)...)
		}
	case 0x1a:
		return r.memory(f.data[1:])
	case 0x09, 0x0a, 0x0b:
		return buildAnswer(0xfa)
	}
	return buildAnswer(0xfa)
}
func selector(b []byte) (int, bool) {
	if len(b) != 2 || b[0] != 0 {
		return 0, false
	}
	if b[1] < 1 || b[1] > 0x99 || b[1]&15 > 9 || b[1]>>4 > 9 {
		return 0, false
	}
	return int(b[1]>>4)*10 + int(b[1]&15), true
}
func (r *Radio) memory(p []byte) []byte {
	if len(p) < 3 || p[0] != 0 {
		return buildAnswer(0xfa)
	}
	ch, ok := selector(p[1:3])
	if !ok {
		return buildAnswer(0xfa)
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
				return buildAnswer(append([]byte{0x1a, 0, p[1], p[2]}, b...)...)
			} else {
				return buildAnswer(0xfa)
			}
		}
		return buildAnswer(append([]byte{0x1a, 0, p[1], p[2]}, b...)...)
	}
	if len(rest) == 1 && rest[0] == 0xff || len(rest) != r.recordLen {
		if r.shortSetAck && len(rest) > 1 && !(len(rest) == 1 && rest[0] == 0xff) {
			return buildAnswer(0xfb)
		}
		return buildAnswer(0xfa)
	}
	r.mu.Lock()
	r.slots[ch] = append([]byte(nil), rest...)
	r.mu.Unlock()
	return buildAnswer(0xfb)
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
