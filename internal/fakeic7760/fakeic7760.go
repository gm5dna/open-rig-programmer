package fakeic7760

import (
	"net"
	"sync"
	"time"
)

type Radio struct {
	host, device                 net.Conn
	addr                         byte
	id                           []byte
	echo                         bool
	allowShort                   bool
	emptyReply                   byte
	recordLen                    int
	latency                      time.Duration
	mu                           sync.Mutex
	slots                        map[int]MemState
	commands                     [][2]byte
	bytesWritten                 []byte
	closed                       chan struct{}
	out                          chan []byte
	once                         sync.Once
	wg                           sync.WaitGroup
	broadcastStop, addressedStop chan struct{}
}

func New(opts ...Option) *Radio {
	c := defaultConfig()
	for _, o := range opts {
		o(&c)
	}
	h, d := net.Pipe()
	r := &Radio{host: h, device: d, addr: c.addr, id: c.id, echo: c.echo, allowShort: c.allowShort, emptyReply: c.emptyReply, recordLen: c.recordLen, latency: c.latency, slots: make(map[int]MemState), closed: make(chan struct{}), out: make(chan []byte, 128)}
	for ch, v := range c.channels {
		r.slots[ch] = MemState{Raw: append([]byte(nil), v...)}
	}
	r.wg.Add(2)
	go r.serve()
	go r.writer()
	r.StartBroadcastFlood(c.broadcast)
	r.StartAddressedFlood(c.addressed)
	return r
}
func (r *Radio) Port() net.Conn { return r.host }
func (r *Radio) Close() error {
	r.StopFloods()
	r.once.Do(func() { close(r.closed); _ = r.device.Close() })
	r.wg.Wait()
	return nil
}
func (r *Radio) serve() {
	defer r.wg.Done()
	a := newReassembler(4096)
	b := make([]byte, 4096)
	for {
		n, e := r.device.Read(b)
		if n > 0 {
			r.mu.Lock()
			r.bytesWritten = append(r.bytesWritten, b[:n]...)
			r.mu.Unlock()
			for _, f := range a.push(b[:n]) {
				r.dispatch(f)
			}
		}
		if e != nil {
			return
		}
	}
}
func (r *Radio) writer() {
	defer r.wg.Done()
	for {
		select {
		case b := <-r.out:
			if _, err := r.device.Write(b); err != nil {
				return
			}
		case <-r.closed:
			return
		}
	}
}
func (r *Radio) emit(b []byte) {
	select {
	case r.out <- append([]byte(nil), b...):
	case <-r.closed:
	default:
	}
}
func (r *Radio) dispatch(raw []byte) {
	if r.echo {
		r.emit(raw)
	}
	f, ok := parseFrame(raw)
	if !ok || f.to != r.addr {
		return
	}
	p := r.handle(f.payload)
	if p == nil {
		return
	}
	if r.latency > 0 {
		select {
		case <-time.After(r.latency):
		case <-r.closed:
			return
		}
	}
	r.emit(p)
}
func (r *Radio) handle(p []byte) []byte {
	if len(p) == 0 {
		return reply(r.addr, CodeNG)
	}
	if len(p) >= 2 && p[0] == 0x19 && p[1] == 0 {
		if len(p) != 2 {
			return reply(r.addr, CodeNG)
		}
		out := append([]byte{0x19, 0}, r.id...)
		return reply(r.addr, out...)
	}
	if len(p) >= 2 && p[0] == 0x1A && p[1] == 0 {
		if len(p) == 4 {
			return r.read(p[2], p[3])
		}
		if len(p) == 4+r.recordLen {
			return r.write(p[2], p[3], p[4:])
		}
		if r.allowShort && len(p) > 4 && len(p) < 4+r.recordLen {
			return r.write(p[2], p[3], p[4:])
		}
		return reply(r.addr, CodeNG)
	}
	return reply(r.addr, CodeNG)
}
func (r *Radio) read(hi, lo byte) []byte {
	ch, ok := channelFor(hi, lo)
	if !ok {
		return reply(r.addr, CodeNG)
	}
	r.mu.Lock()
	m, set := r.slots[ch]
	r.mu.Unlock()
	if !set {
		return reply(r.addr, r.emptyReply)
	}
	allFF := len(m.Raw) > 0
	for _, b := range m.Raw { allFF = allFF && b == 0xFF }
	if allFF { return reply(r.addr, r.emptyReply) }
	out := append([]byte{0x1A, 0, hi, lo}, m.Raw...)
	return reply(r.addr, out...)
}
func (r *Radio) write(hi, lo byte, v []byte) []byte {
	ch, ok := channelFor(hi, lo)
	if !ok {
		return reply(r.addr, CodeNG)
	}
	r.mu.Lock()
	r.slots[ch] = MemState{Raw: append([]byte(nil), v...)}
	r.commands = append(r.commands, [2]byte{0x1A, 0})
	r.mu.Unlock()
	return reply(r.addr, CodeOK)
}
func reply(addr byte, p ...byte) []byte {
	o := []byte{0xFE, 0xFE, AddrController, addr}
	o = append(o, p...)
	return append(o, 0xFD)
}
func (r *Radio) StartBroadcastFlood(d time.Duration) {
	r.startFlood(AddrBroadcast, d, &r.broadcastStop)
}
func (r *Radio) StartAddressedFlood(d time.Duration) {
	r.startFlood(AddrController, d, &r.addressedStop)
}
func (r *Radio) startFlood(to byte, d time.Duration, slot *chan struct{}) {
	if d <= 0 {
		return
	}
	r.mu.Lock()
	if *slot != nil {
		close(*slot)
	}
	s := make(chan struct{})
	*slot = s
	r.wg.Add(1)
	r.mu.Unlock()
	go func() {
		defer r.wg.Done()
		t := time.NewTicker(d)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				out := append([]byte{0x19, 0}, r.id...)
				r.emit(reply(to, out...))
			case <-s:
				return
			case <-r.closed:
				return
			}
		}
	}()
}
func (r *Radio) StopFloods() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.broadcastStop != nil {
		close(r.broadcastStop)
		r.broadcastStop = nil
	}
	if r.addressedStop != nil {
		close(r.addressedStop)
		r.addressedStop = nil
	}
}
