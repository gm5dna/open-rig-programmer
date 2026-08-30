// SPDX-License-Identifier: GPL-3.0-or-later

package icr8600

import (
	"bytes"
	"net"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civicr8600 "github.com/gm5dna/open-rig-programmer/core/civ/icr8600"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

type testWireAddress struct{ group, channel int }

type testRadioImage struct {
	idToken     []byte
	idFrom      byte
	records     map[testWireAddress][]byte
	answerAddr  map[testWireAddress]testWireAddress
	echo        bool
	falseEcho   bool
	acknowledge bool
}

type respondingPort struct {
	host, remote net.Conn
	mu           sync.Mutex
	received     [][]byte
	image        testRadioImage
}

func newRespondingPort(t *testing.T, image testRadioImage) *respondingPort {
	t.Helper()
	host, remote := net.Pipe()
	p := &respondingPort{host: host, remote: remote, image: image}
	t.Cleanup(func() { _ = host.Close(); _ = remote.Close() })
	go p.serve()
	return p
}

func (p *respondingPort) Port() transport.Port { return p.host }

func (p *respondingPort) Transcript() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([][]byte, len(p.received))
	for i := range p.received {
		out[i] = bytes.Clone(p.received[i])
	}
	return out
}

func (p *respondingPort) setRecord(addr testWireAddress, record []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.image.records == nil {
		p.image.records = map[testWireAddress][]byte{}
	}
	p.image.records[addr] = bytes.Clone(record)
}

func (p *respondingPort) misdirect(requested, answered testWireAddress) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.image.answerAddr == nil {
		p.image.answerAddr = map[testWireAddress]testWireAddress{}
	}
	p.image.answerAddr[requested] = answered
}

func (p *respondingPort) serve() {
	buf := make([]byte, 1024)
	var pending []byte
	for {
		n, err := p.remote.Read(buf)
		if n > 0 {
			pending = append(pending, buf[:n]...)
			for {
				frame, rest, ok := cutFrame(pending)
				if !ok {
					break
				}
				pending = rest
				p.mu.Lock()
				p.received = append(p.received, bytes.Clone(frame))
				replies := p.repliesLocked(frame)
				p.mu.Unlock()
				for _, reply := range replies {
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

func cutFrame(in []byte) (frame, rest []byte, ok bool) {
	start := bytes.Index(in, []byte{0xFE, 0xFE})
	if start < 0 {
		return nil, in, false
	}
	end := bytes.IndexByte(in[start:], 0xFD)
	if end < 0 {
		return nil, in, false
	}
	end += start
	return bytes.Clone(in[start : end+1]), in[end+1:], true
}

func (p *respondingPort) repliesLocked(frame []byte) [][]byte {
	var replies [][]byte
	if p.image.falseEcho {
		wrong := bytes.Clone(frame)
		// Keep the position and length of an echo but make its command a
		// value this driver never sends, so it cannot accidentally equal a
		// different recorded request from the bounded walk.
		wrong[4] = 0x7F
		replies = append(replies, wrong)
	}
	if p.image.echo {
		replies = append(replies, bytes.Clone(frame))
	}
	if len(frame) == 7 && frame[4] == 0x19 && frame[5] == 0x00 {
		if p.image.idToken == nil {
			return replies
		}
		from := p.image.idFrom
		if from == 0 {
			from = civicr8600.RadioAddress
		}
		return append(replies, testFrame(civicr8600.ControllerAddress, from, append([]byte{0x19, 0x00}, p.image.idToken...)...))
	}
	if len(frame) >= 11 && frame[4] == 0x1A && frame[5] == 0x00 {
		addr := decodeTestAddress(frame[6:10])
		if len(frame) == 11 {
			record, ok := p.image.records[addr]
			if !ok {
				return append(replies, testFrame(civicr8600.ControllerAddress, civicr8600.RadioAddress, 0xFA))
			}
			answer := addr
			if a, ok := p.image.answerAddr[addr]; ok {
				answer = a
			}
			body := append([]byte{0x1A, 0x00}, encodeTestAddress(answer)...)
			body = append(body, record...)
			return append(replies, testFrame(civicr8600.ControllerAddress, civicr8600.RadioAddress, body...))
		}
		if p.image.acknowledge {
			return append(replies, testFrame(civicr8600.ControllerAddress, civicr8600.RadioAddress, 0xFB))
		}
	}
	return replies
}

func testFrame(to, from byte, body ...byte) []byte {
	out := []byte{0xFE, 0xFE, to, from}
	out = append(out, body...)
	return append(out, 0xFD)
}

func encodeTestAddress(a testWireAddress) []byte {
	return []byte{byte(a.group / 100), byte((a.group/10%10)<<4 | a.group%10), byte(a.channel / 100), byte((a.channel/10%10)<<4 | a.channel%10)}
}

func decodeTestAddress(b []byte) testWireAddress {
	return testWireAddress{int(b[0])*100 + int(b[1]>>4)*10 + int(b[1]&0x0F), int(b[2])*100 + int(b[3]>>4)*10 + int(b[3]&0x0F)}
}

func testRecord(t *testing.T, addr testWireAddress, mode string) []byte {
	t.Helper()
	rec := civ.MemoryRecord{
		Address: civ.ChannelAddress{Group: addr.group, Channel: addr.channel},
		Select:  civ.Available("OFF"), RXFreqHz: civ.Available(uint64(145_500_000)),
		Mode: civ.Available(mode), Filter: civ.Available("FIL1"),
		Duplex: civ.Available("OFF"), OffsetHz: civ.Available(uint64(0)),
		TuningStepEnabled: civ.Available("ON"), TuningStep: civ.Available("5 kHz"),
		ProgramTuningStepHz: civ.Available(uint64(9_000)), AttenuatorDB: civ.Available(uint64(10)),
		Preamp: civ.Available("ON"), Antenna: civ.Available("ANT2"), IPPlus: civ.Available("ON"),
		Name: civ.Available("DRIVER TEST"),
	}
	if mode == "FM" {
		rec.ToneMode = civ.Available("TSQL")
		rec.ToneRXDeciHz = civ.Available(uint64(885))
		rec.DTCSCode = civ.Available(uint64(23))
		rec.DTCSPolarity = civ.Available("Reverse")
	}
	cmd, err := civicr8600.Profile().BuildMemorySet(rec)
	if err != nil {
		t.Fatalf("BuildMemorySet(%s): %v", mode, err)
	}
	frame := cmd.Bytes()
	n := civicr8600.Profile().BuildRecordLengthFor(mode)
	return bytes.Clone(frame[len(frame)-1-n : len(frame)-1])
}

func fastTiming() Option {
	return func(d *icr8600Driver) {
		d.readTimeout = testShortTimeout
		d.settle = testShortSettle
	}
}

const (
	testShortTimeout = 20_000_000
	testShortSettle  = 1
)
