// SPDX-License-Identifier: GPL-3.0-or-later

package ft890900

import (
	"net"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// scriptedPort is this package's scripted radio: a net.Pipe whose remote
// end answers each complete 5-byte frame it receives per a
// caller-configured table (setAnswer), recording every frame received in
// order — the same convention core/driver/ic7200/scriptedport_test.go and
// core/driver/ft450d/respondingport_test.go use for their own protocols,
// sized down for this family's fixed 5-byte-frame, no-delimiter shape.
type scriptedPort struct {
	host, remote net.Conn

	mu       sync.Mutex
	received [][5]byte
	answers  map[[5]byte][]byte
}

func newScriptedPort(t *testing.T) *scriptedPort {
	t.Helper()
	host, remote := net.Pipe()
	p := &scriptedPort{host: host, remote: remote, answers: map[[5]byte][]byte{}}
	t.Cleanup(func() {
		_ = host.Close()
		_ = remote.Close()
	})
	go p.serve()
	return p
}

// Port returns the end handed to the driver.
func (p *scriptedPort) Port() transport.Port { return p.host }

// setAnswer configures the reply for one exact 5-byte request: nil means
// no reply at all (this family's silent "do nothing" convention).
func (p *scriptedPort) setAnswer(frame [5]byte, reply []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.answers[frame] = reply
}

// Transcript returns a copy of every complete frame received, in order.
func (p *scriptedPort) Transcript() [][5]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([][5]byte(nil), p.received...)
}

// ResetTranscript clears the recorded frame history — used after Open so
// a test's own assertions see only the frames its own call under test
// sent, not Open's own two identity probes.
func (p *scriptedPort) ResetTranscript() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.received = nil
}

func (p *scriptedPort) serve() {
	buf := make([]byte, 256)
	var acc []byte
	for {
		n, err := p.remote.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			for len(acc) >= 5 {
				var frame [5]byte
				copy(frame[:], acc[:5])
				acc = acc[5:]

				p.mu.Lock()
				p.received = append(p.received, frame)
				reply, ok := p.answers[frame]
				p.mu.Unlock()

				if ok && reply != nil {
					if _, werr := p.remote.Write(reply); werr != nil {
						return
					}
				}
			}
		}
		if err != nil {
			return
		}
	}
}
