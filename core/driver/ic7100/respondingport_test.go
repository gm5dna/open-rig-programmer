// SPDX-License-Identifier: GPL-3.0-or-later

package ic7100

import (
	"encoding/hex"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

const (
	peerRadioAddress      = byte(0x88)
	peerControllerAddress = byte(0xE0)
)

type peerSlot struct{ bank, channel int }

type respondingPort struct {
	host, remote net.Conn
	closeOnce    sync.Once

	mu          sync.Mutex
	received    [][]byte
	records     map[peerSlot][]byte
	idToken     []byte
	idSource    byte
	noSetAnswer bool
	rejectSets  bool
}

type peerOption func(*respondingPort)

func newRespondingPort(t *testing.T, opts ...peerOption) *respondingPort {
	t.Helper()
	host, remote := net.Pipe()
	p := &respondingPort{
		host: host, remote: remote, records: map[peerSlot][]byte{},
		idToken: []byte{0x71, 0x00}, idSource: peerRadioAddress,
	}
	for _, opt := range opts {
		opt(p)
	}
	t.Cleanup(func() { _ = host.Close(); _ = remote.Close() })
	go p.serve()
	return p
}

func (p *respondingPort) Read(b []byte) (int, error)  { return p.host.Read(b) }
func (p *respondingPort) Write(b []byte) (int, error) { return p.host.Write(b) }
func (p *respondingPort) Close() error {
	var err error
	p.closeOnce.Do(func() { err = p.host.Close() })
	return err
}

func (p *respondingPort) Port() transport.Port { return p }

func (p *respondingPort) frames() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([][]byte, len(p.received))
	for i, f := range p.received {
		out[i] = append([]byte(nil), f...)
	}
	return out
}

func (p *respondingPort) recordAt(bank, channel int) []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]byte(nil), p.records[peerSlot{bank, channel}]...)
}

func withRecord(bank, channel int, record []byte) peerOption {
	return func(p *respondingPort) { p.records[peerSlot{bank, channel}] = append([]byte(nil), record...) }
}

func withRecordLength(bank, channel, length int) peerOption {
	return withRecord(bank, channel, make([]byte, length))
}

func withIDSource(source byte) peerOption { return func(p *respondingPort) { p.idSource = source } }
func withNoSetAnswer() peerOption         { return func(p *respondingPort) { p.noSetAnswer = true } }
func withRejectedSets() peerOption        { return func(p *respondingPort) { p.rejectSets = true } }

func occupiedRecord(t *testing.T) []byte {
	t.Helper()
	const frame = "FE FE 88 E0 1A 00 01 00 01 00 00 00 50 45 01 05 01 00 00 00 00 08 85 00 08 85 00 00 23 00 00 60 00 43 51 43 51 43 51 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 00 00 50 45 01 05 01 00 00 00 00 08 85 00 08 85 00 00 23 00 00 60 00 43 51 43 51 43 51 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 20 48 4F 4D 45 20 42 41 53 45 20 20 20 20 20 20 20 FD"
	b, err := hex.DecodeString(strings.ReplaceAll(frame, " ", ""))
	if err != nil {
		t.Fatal(err)
	}
	return append([]byte(nil), b[9:len(b)-1]...)
}

func (p *respondingPort) serve() {
	buf := make([]byte, 512)
	var acc []byte
	for {
		n, err := p.remote.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			for {
				i := byteIndex(acc, 0xFD)
				if i < 0 {
					break
				}
				frame := append([]byte(nil), acc[:i+1]...)
				acc = acc[i+1:]
				p.mu.Lock()
				p.received = append(p.received, frame)
				p.mu.Unlock()
				if answer := p.reply(frame); answer != nil {
					if _, werr := p.remote.Write(answer); werr != nil {
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

func byteIndex(b []byte, want byte) int {
	for i, got := range b {
		if got == want {
			return i
		}
	}
	return -1
}

func answerFrom(source byte, body ...byte) []byte {
	out := []byte{0xFE, 0xFE, peerControllerAddress, source}
	out = append(out, body...)
	return append(out, 0xFD)
}

func (p *respondingPort) reply(frame []byte) []byte {
	if len(frame) < 7 || frame[0] != 0xFE || frame[1] != 0xFE || frame[2] != peerRadioAddress || frame[3] != peerControllerAddress {
		return nil
	}
	cn, sc := frame[4], frame[5]
	switch {
	case cn == 0x19 && sc == 0x00 && len(frame) == 7:
		body := append([]byte{0x19, 0x00}, p.idToken...)
		return answerFrom(p.idSource, body...)
	case cn == 0x1A && sc == 0x00 && len(frame) == 10:
		bank := decodeBCD(frame[6])
		channel := decodeBCD(frame[7])*100 + decodeBCD(frame[8])
		p.mu.Lock()
		record, ok := p.records[peerSlot{bank, channel}]
		record = append([]byte(nil), record...)
		p.mu.Unlock()
		if !ok {
			return answerFrom(peerRadioAddress, 0xFA)
		}
		body := []byte{0x1A, 0x00, frame[6], frame[7], frame[8]}
		body = append(body, record...)
		return answerFrom(peerRadioAddress, body...)
	case cn == 0x1A && sc == 0x00:
		if p.noSetAnswer {
			return nil
		}
		if p.rejectSets {
			return answerFrom(peerRadioAddress, 0xFA)
		}
		if len(frame) == 10+111 {
			bank := decodeBCD(frame[6])
			channel := decodeBCD(frame[7])*100 + decodeBCD(frame[8])
			p.mu.Lock()
			p.records[peerSlot{bank, channel}] = append([]byte(nil), frame[9:len(frame)-1]...)
			p.mu.Unlock()
			return answerFrom(peerRadioAddress, 0xFB)
		}
		return answerFrom(peerRadioAddress, 0xFA)
	default:
		return answerFrom(peerRadioAddress, 0xFA)
	}
}

func decodeBCD(b byte) int { return int(b>>4)*10 + int(b&0x0F) }
