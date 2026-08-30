// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7100

import (
	"io"
	"sync"
)

// This file holds the fake's runtime plumbing — the two byte queues that stand
// in for a serial line, the port the driver opens onto them, and the memory
// image. Nothing here knows a CI-V byte from any other byte, and nothing here
// interprets a record.

// pipe is a buffered one-way byte queue. Read blocks until bytes arrive or the
// pipe is closed; Write never blocks.
//
// It is buffered rather than an io.Pipe on purpose. io.Pipe is a rendezvous:
// each Write waits for a Read. A fake that emits unsolicited traffic —
// WithTransceiveBroadcasts, WithAddressedFlood — would then wedge its own
// emitter goroutine the moment the consumer under test stopped reading, which
// is precisely the condition those options exist to create. A drain cap is not
// reachable if the flood cannot outrun the drain.
type pipe struct {
	mu     sync.Mutex
	cond   *sync.Cond
	buf    []byte
	closed bool
}

func newPipe() *pipe {
	p := &pipe{}
	p.cond = sync.NewCond(&p.mu)
	return p
}

func (p *pipe) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return 0, io.ErrClosedPipe
	}
	p.buf = append(p.buf, b...)
	p.cond.Broadcast()
	return len(b), nil
}

func (p *pipe) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(p.buf) == 0 && !p.closed {
		p.cond.Wait()
	}
	if len(p.buf) == 0 {
		return 0, io.EOF
	}
	n := copy(b, p.buf)
	p.buf = append(p.buf[:0], p.buf[n:]...)
	return n, nil
}

// Close wakes every blocked reader. Bytes already queued stay readable, so a
// consumer that closes after a burst still sees the burst; only once the queue
// is drained does Read report io.EOF.
func (p *pipe) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.closed {
		p.closed = true
		p.cond.Broadcast()
	}
	return nil
}

// port is the reader/writer the driver opens: it reads what the radio said and
// writes what the controller says.
type port struct{ radio *Radio }

func (p *port) Read(b []byte) (int, error)  { return p.radio.toController.Read(b) }
func (p *port) Write(b []byte) (int, error) { return p.radio.toRadio.Write(b) }

// Close closes the whole fake, not just this end of it: a driver that closes
// its port has finished with the radio, and the radio's emitter goroutines must
// stop with it rather than outlive the test that made them.
func (p *port) Close() error { return p.radio.Close() }

var _ io.ReadWriteCloser = (*port)(nil)

// slot is one memory channel of the fake's image. A slot that is not occupied
// is one a read cannot satisfy — which is what WithEmptySlot asks for and what
// a channel that was never seeded gets anyway.
type slot struct {
	record   []byte
	occupied bool
}

// image is the fake's memory: channels keyed by their three address bytes.
//
// The key is the RAW address bytes as they arrived on the wire, not a decoded
// (bank, channel) pair. That is deliberate: a request naming a channel this
// package would refuse to seed simply misses the map, rather than being decoded
// into something it is not.
type image struct {
	slots map[string]slot
}

func newImage() *image { return &image{slots: make(map[string]slot)} }

// seed installs one slot before the radio starts answering.
func (i *image) seed(addr []byte, record []byte, occupied bool) {
	i.slots[string(addr)] = slot{record: append([]byte(nil), record...), occupied: occupied}
}

// read returns a copy of the record at addr, and whether that channel is
// occupied at all.
func (i *image) read(addr []byte) ([]byte, bool) {
	s, ok := i.slots[string(addr)]
	if !ok || !s.occupied {
		return nil, false
	}
	return append([]byte(nil), s.record...), true
}

// write stores a record at addr and marks the channel occupied.
func (i *image) write(addr []byte, record []byte) {
	i.slots[string(addr)] = slot{record: append([]byte(nil), record...), occupied: true}
}
