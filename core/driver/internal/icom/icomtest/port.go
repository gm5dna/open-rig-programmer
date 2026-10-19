// SPDX-License-Identifier: GPL-3.0-or-later

package icomtest

import (
	"bytes"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// ScriptedPort is a scripted radio: a net.Pipe whose remote end splits the CI-V
// frames the driver writes, answers each from a RadioImage table, and records
// every frame received. It can serve deliberately wrong answers and flood.
type ScriptedPort struct {
	addr   byte
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received [][]byte
	rtsCalls int
	dtrCalls int
	writeMu  sync.Mutex

	floodStop chan struct{}
	floodOnce sync.Once
	floodDone sync.WaitGroup
}

// RadioImage is what a ScriptedPort's radio contains.
type RadioImage struct {
	IDToken       []byte
	IDFrom        byte
	IDBroadcast   bool
	Records       map[int][]byte
	AnswerAddress func(asked int) int
	MemSilent     bool
	AckSets       bool
	RejectSets    bool
	SetFrom       byte
	LateSetAck    time.Duration
}

// NewPort starts a scripted radio at CI-V address addr serving img.
func NewPort(t *testing.T, addr byte, img RadioImage) *ScriptedPort {
	t.Helper()
	host, remote := net.Pipe()
	p := &ScriptedPort{addr: addr, host: host, remote: remote, floodStop: make(chan struct{})}
	t.Cleanup(func() {
		p.StopFlood()
		p.floodDone.Wait()
		_ = host.Close()
		_ = remote.Close()
	})
	go p.serve(img)
	return p
}
func (p *ScriptedPort) Port() transport.Port { return p.host }
func (p *ScriptedPort) SetRTS(bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rtsCalls++
	return nil
}

func (p *ScriptedPort) SetDTR(bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dtrCalls++
	return nil
}

func (p *ScriptedPort) ControlLineCalls() (rts, dtr int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.rtsCalls, p.dtrCalls
}
func (p *ScriptedPort) Transcript() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([][]byte, len(p.received))
	for i, f := range p.received {
		out[i] = append([]byte(nil), f...)
	}
	return out
}
func (p *ScriptedPort) StartFlood(to byte) {
	p.floodDone.Add(1)
	go func() {
		defer p.floodDone.Done()
		frame := []byte{civ.PreambleByte, civ.PreambleByte, to, p.addr, 0x19, 0x00, p.addr, civ.EndByte}
		tick := time.NewTicker(2 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-p.floodStop:
				return
			case <-tick.C:
				if !p.write(frame) {
					return
				}
			}
		}
	}()
}
func (p *ScriptedPort) StopFlood() { p.floodOnce.Do(func() { close(p.floodStop) }) }
func (p *ScriptedPort) StopFloodAfter(d time.Duration) {
	p.floodDone.Add(1)
	go func() {
		defer p.floodDone.Done()
		select {
		case <-p.floodStop:
		case <-time.After(d):
			p.StopFlood()
		}
	}()
}
func (p *ScriptedPort) writeAfter(d time.Duration, frame []byte) {
	p.floodDone.Add(1)
	go func() {
		defer p.floodDone.Done()
		select {
		case <-p.floodStop:
		case <-time.After(d):
			p.write(frame)
		}
	}()
}
func (p *ScriptedPort) write(frame []byte) bool {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	_, err := p.remote.Write(frame)
	return err == nil
}
func (p *ScriptedPort) serve(img RadioImage) {
	buf := make([]byte, 512)
	var acc []byte
	for {
		n, err := p.remote.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			for {
				i := bytes.IndexByte(acc, civ.EndByte)
				if i < 0 {
					break
				}
				frame := append([]byte(nil), acc[:i+1]...)
				acc = acc[i+1:]
				p.record(frame)
				if img.LateSetAck > 0 && len(frame) >= MemSetFrameLen &&
					frame[4] == civ.CmdMemory && frame[5] == civ.SubMemoryContents {
					p.writeAfter(img.LateSetAck, AckFrame(p.addr))
					continue
				}
				for _, reply := range img.reply(p.addr, frame) {
					if !p.write(reply) {
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

func (p *ScriptedPort) record(frame []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.received = append(p.received, frame)
}

const (
	MemReadFrameLen = 9
	MemSetFrameLen  = 34
	IDReadFrameLen  = 7
)

func (img RadioImage) reply(addr byte, frame []byte) [][]byte {
	switch {
	case len(frame) == IDReadFrameLen && frame[4] == civ.CmdTransceiverID && frame[5] == civ.SubTransceiverID:
		if img.IDToken == nil {
			return nil
		}
		from := img.IDFrom
		if from == 0 {
			from = addr
		}
		to := byte(civ.ControllerAddressDefault)
		if img.IDBroadcast {
			to = 0x00
		}
		out := []byte{civ.PreambleByte, civ.PreambleByte, to, from, civ.CmdTransceiverID, civ.SubTransceiverID}
		out = append(out, img.IDToken...)
		return [][]byte{append(out, civ.EndByte)}

	case len(frame) == MemReadFrameLen && frame[4] == civ.CmdMemory && frame[5] == civ.SubMemoryContents:
		if img.MemSilent {
			return nil
		}
		asked := BCDChannel(frame[6], frame[7])
		rec, ok := img.Records[asked]
		if !ok {
			return [][]byte{NakFrame(addr)}
		}
		named := asked
		if img.AnswerAddress != nil {
			named = img.AnswerAddress(asked)
		}
		return [][]byte{memAnswerFrame(addr, named, rec)}

	case len(frame) >= MemSetFrameLen && frame[4] == civ.CmdMemory && frame[5] == civ.SubMemoryContents:
		from := img.SetFrom
		if from == 0 {
			from = addr
		}
		switch {
		case img.RejectSets:
			return [][]byte{NakFrame(from)}
		case img.AckSets:
			return [][]byte{AckFrame(from)}
		default:
			return nil // silence: an acknowledged write's ack timeout
		}
	}
	return nil
}
func BCDChannel(hi, lo byte) int {
	return int(hi>>4)*1000 + int(hi&0x0F)*100 + int(lo>>4)*10 + int(lo&0x0F)
}
func EncodeChannel(ch int) (hi, lo byte) {
	hi = byte((ch/1000)<<4 | (ch/100)%10)
	lo = byte(((ch/10)%10)<<4 | ch%10)
	return hi, lo
}
func memAnswerFrame(addr byte, ch int, rec []byte) []byte {
	hi, lo := EncodeChannel(ch)
	out := []byte{civ.PreambleByte, civ.PreambleByte, civ.ControllerAddressDefault, addr, civ.CmdMemory, civ.SubMemoryContents, hi, lo}
	out = append(out, rec...)
	return append(out, civ.EndByte)
}
func NakFrame(from byte) []byte {
	return []byte{civ.PreambleByte, civ.PreambleByte, civ.ControllerAddressDefault, from, civ.NakByte, civ.EndByte}
}

func AckFrame(from byte) []byte {
	return []byte{civ.PreambleByte, civ.PreambleByte, civ.ControllerAddressDefault, from, civ.AckByte, civ.EndByte}
}
func HexFrames(frames [][]byte) string {
	var b bytes.Buffer
	for i, f := range frames {
		if i > 0 {
			b.WriteString("\n  ")
		}
		for j, x := range f {
			if j > 0 {
				b.WriteByte(' ')
			}
			fmt.Fprintf(&b, "%02x", x)
		}
	}
	return b.String()
}

// RestartFlood stops any flood, waits for it to exit, and starts a fresh one
// addressed to to.
func (p *ScriptedPort) RestartFlood(to byte) {
	p.StopFlood()
	p.floodDone.Wait()
	p.floodStop = make(chan struct{})
	p.floodOnce = sync.Once{}
	p.StartFlood(to)
}

// MemReadFrame is the 1A 00 read frame for channel ch at address addr.
func MemReadFrame(addr byte, ch int) []byte {
	hi, lo := EncodeChannel(ch)
	return []byte{0xFE, 0xFE, addr, 0xE0, 0x1A, 0x00, hi, lo, 0xFD}
}
