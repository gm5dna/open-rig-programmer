package fakeic7760

import (
	"fmt"
	"net"
	"sync"
	"time"
)

type Radio struct {
	host, device                 net.Conn
	addr                         byte
	id                           []byte
	echo                         bool
	fullRecord                   bool
	emptyReply                   byte
	emptyRecordFF                bool
	recordLen                    int
	scanEdgeLen                  int
	broadcastTo                  byte
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
	r := &Radio{host: h, device: d, addr: c.addr, id: c.id, echo: c.echo, fullRecord: c.fullRecord, emptyReply: c.emptyReply, emptyRecordFF: c.emptyRecordFF, recordLen: c.recordLen, scanEdgeLen: c.scanEdgeLen, broadcastTo: c.broadcastTo, latency: c.latency, slots: make(map[int]MemState), closed: make(chan struct{}), out: make(chan []byte, 128)}
	for ch, v := range c.channels {
		if n := c.recordLenFor(ch); len(v) != n {
			panic(fmt.Sprintf("fakeic7760: channel %d record has length %d, want %d", ch, len(v), n))
		}
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
	// The echo runs before the address filter and stays there: an echo is a
	// property of the line (a USB codec reflecting what was put on it),
	// answering is a property of the radio. That ordering is the modelled
	// assumption ic7760-echo-default, and TestAForeignControllerIsIgnored
	// pins it alongside the filter below.
	if r.echo {
		r.emit(raw)
	}
	f, ok := parseFrame(raw)
	// Both halves of the printed address pair are required: destination B2,
	// the transceiver's default address, AND source E0, the controller's.
	// A frame from any other source is on this radio's line but is not its
	// business, so it earns silence rather than NG.
	if !ok || f.to != r.addr || f.from != AddrController {
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
		if len(p) > 4 {
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
	// Reading a stored all-FF record back as "empty" is the ASSUMED register
	// entry ic7760-empty-reply-ff, and it is a different question from the
	// outbound clear form refused in write below.
	// TestTheInboundAllFFRecordInterpretationIsOptional pins both halves.
	if r.emptyRecordFF && allFF(m.Raw) {
		return reply(r.addr, r.emptyReply)
	}
	out := append([]byte{0x1A, 0, hi, lo}, m.Raw...)
	return reply(r.addr, out...)
}
func (r *Radio) write(hi, lo byte, v []byte) []byte {
	ch, ok := channelFor(hi, lo)
	if !ok {
		return reply(r.addr, CodeNG)
	}
	// The printed clear form — a single FF in the ③ select-memory byte and
	// nothing after it — is matched here explicitly so that the refusal is a
	// decision at a named place rather than a by-product of the length
	// arithmetic below, and so that it survives every length knob. Erase is
	// not admitted by this tier at all.
	// TestThePrintedClearFormIsRefusedIndependentlyOfShortSets pins it.
	if len(v) == 1 && v[0] == 0xFF {
		return reply(r.addr, CodeNG)
	}
	n := r.recordLenFor(ch)
	if len(v) > n || (len(v) != n && r.fullRecord) {
		return reply(r.addr, CodeNG)
	}
	r.mu.Lock()
	r.slots[ch] = MemState{Raw: append([]byte(nil), v...)}
	r.commands = append(r.commands, [2]byte{0x1A, 0})
	r.mu.Unlock()
	return reply(r.addr, CodeOK)
}

// wire builds one frame. The destination comes first on the line, as the
// data-format diagram draws it: FE FE <to> <from> <data> FD.
func wire(to, from byte, p ...byte) []byte {
	o := []byte{0xFE, 0xFE, to, from}
	o = append(o, p...)
	return append(o, 0xFD)
}

// reply is an answer to the controller: to=E0, from=this radio.
func reply(addr byte, p ...byte) []byte { return wire(AddrController, addr, p...) }
func (r *Radio) StartBroadcastFlood(d time.Duration) {
	r.startFlood(r.broadcastTo, d, &r.broadcastStop)
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
				// An unsolicited frame comes FROM the radio, so only the
				// destination varies between the two floods: the assumed
				// broadcast form (ic7760-broadcast-form, WithBroadcastForm)
				// and the synthetic controller-addressed one.
				// TestTheBroadcastFormIsConfigurable pins both.
				out := append([]byte{0x19, 0}, r.id...)
				r.emit(wire(to, r.addr, out...))
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

// recordLenFor is the accepted record length for one slot; see
// WithScanEdgeRecordShape.
func (r *Radio) recordLenFor(ch int) int {
	if (ch == ChanP1 || ch == ChanP2) && r.scanEdgeLen > 0 {
		return r.scanEdgeLen
	}
	return r.recordLen
}

// allFF reports whether a stored record is every-byte FF. Its meaning is the
// caller's question, not this helper's.
func allFF(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	for _, v := range b {
		if v != 0xFF {
			return false
		}
	}
	return true
}
