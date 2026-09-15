// SPDX-License-Identifier: GPL-3.0-or-later

package ft1000mp

import (
	"errors"
	"sync"
)

// scriptedPort is a transport.Port whose replies are computed per write
// by a caller-supplied function — this driver's own binary frames need a
// content-keyed responder (a fixed reply script cannot express "reply
// differently to a Store than to a SetFreq"), unlike core/transport's
// line-oriented scriptedPort fixtures. Mirrors core/driver/ic7200's
// per-package scriptedPort convention (spec.md's own precedent note):
// each driver package owns a small one rather than sharing an exported
// helper.
type scriptedPort struct {
	mu      sync.Mutex
	respond func(frame []byte) []byte // nil reply means silence
	// failOn, when non-nil, makes Write return errSimulatedWriteFailure
	// for any frame it reports true for — this family's write choreography
	// can only fail this way (no NAK exists to synthesise instead).
	failOn  func(frame []byte) bool
	pending []byte
	wake    chan struct{}
	closeCh chan struct{}
	closed  bool
	writes  [][]byte
}

var errSimulatedWriteFailure = errors.New("ft1000mp: scriptedPort: simulated write failure")

var errPortClosed = errors.New("ft1000mp: scriptedPort: closed")

func newScriptedPort(respond func(frame []byte) []byte) *scriptedPort {
	return &scriptedPort{
		respond: respond,
		wake:    make(chan struct{}, 64),
		closeCh: make(chan struct{}),
	}
}

func (p *scriptedPort) Read(b []byte) (int, error) {
	for {
		p.mu.Lock()
		if len(p.pending) > 0 {
			n := copy(b, p.pending)
			p.pending = p.pending[n:]
			p.mu.Unlock()
			return n, nil
		}
		p.mu.Unlock()
		select {
		case <-p.wake:
		case <-p.closeCh:
			return 0, errPortClosed
		}
	}
}

func (p *scriptedPort) Write(b []byte) (int, error) {
	frame := append([]byte(nil), b...)
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return 0, errPortClosed
	}
	if p.failOn != nil && p.failOn(frame) {
		p.mu.Unlock()
		return 0, errSimulatedWriteFailure
	}
	p.writes = append(p.writes, frame)
	reply := p.respond(frame)
	if len(reply) > 0 {
		p.pending = append(p.pending, reply...)
	}
	p.mu.Unlock()
	select {
	case p.wake <- struct{}{}:
	default:
	}
	return len(b), nil
}

func (p *scriptedPort) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()
	close(p.closeCh)
	return nil
}

func (p *scriptedPort) Writes() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([][]byte(nil), p.writes...)
}
