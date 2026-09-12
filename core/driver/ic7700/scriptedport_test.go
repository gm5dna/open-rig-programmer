// SPDX-License-Identifier: GPL-3.0-or-later

package ic7700

import (
	"bytes"
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// scriptedPort is this package's scripted radio: a net.Pipe whose remote
// end splits the CI-V frames the driver writes, answers each one from a
// TABLE, and records every frame it received in order.
//
// IT IS A TABLE, NOT A RADIO (core/driver/ic7610's scriptedPort makes the
// same argument, and this file follows its shape). This wave's brief funds
// no internal/fake<model> package, so this table-driven port is this
// package's ONLY test double — for round-trip coverage as well as for the
// wrong-answer error paths a self-consistent fake would never produce.
//
// EVERY ANSWER'S SEMANTICS ARE THIS DOCUMENT'S CONVENTION APPLIED, NOT AN
// OBSERVED RADIO TRANSCRIBED. No IC-7700 has ever been connected to this
// project, so nothing in this file is evidence about what one does.
type scriptedPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received [][]byte
	rtsCalls int
	dtrCalls int

	writeMu sync.Mutex
}

// radioImage is what a scriptedPort's radio "contains".
type radioImage struct {
	// idToken is the DATA bytes of the 19 00 answer. Nil means silence.
	idToken []byte
	// idFrom overrides the address the 19 00 answer comes FROM. Zero means
	// the ordinary 0x74.
	idFrom byte

	// records maps a channel number to the RAW record bytes served for a
	// 1A 00 read of it. A channel ABSENT from the map is answered FA
	// (tier ruling T4: transport.ErrRejected, no frame).
	records map[int][]byte
	// answerAddress, when non-nil, overrides the CHANNEL a memory answer
	// names — tier ruling T2.
	answerAddress func(asked int) int
	// ackSets makes a 1A 00 SET answer FB. rejectSets makes it answer FA.
	// Both false means silence (an ack timeout).
	ackSets    bool
	rejectSets bool
	setFrom    byte
}

func newScriptedPort(t *testing.T, img radioImage) *scriptedPort {
	t.Helper()
	host, remote := net.Pipe()
	p := &scriptedPort{host: host, remote: remote}
	t.Cleanup(func() {
		_ = host.Close()
		_ = remote.Close()
	})
	go p.serve(img)
	return p
}

func (p *scriptedPort) Port() transport.Port { return p.host }

// SetRTS and SetDTR exist ONLY so a test can prove the driver never calls
// them (matrix §3.2: this radio's [RS-232C] port is wired as a modem/DCE,
// and this project asserts no PTT/keying expectation onto either line).
func (p *scriptedPort) SetRTS(bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rtsCalls++
	return nil
}

func (p *scriptedPort) SetDTR(bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dtrCalls++
	return nil
}

func (p *scriptedPort) controlLineCalls() (rts, dtr int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.rtsCalls, p.dtrCalls
}

func (p *scriptedPort) Transcript() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([][]byte, len(p.received))
	for i, f := range p.received {
		out[i] = append([]byte(nil), f...)
	}
	return out
}

func (p *scriptedPort) write(frame []byte) bool {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	_, err := p.remote.Write(frame)
	return err == nil
}

func (p *scriptedPort) serve(img radioImage) {
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
				for _, reply := range img.reply(frame) {
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

func (p *scriptedPort) record(frame []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.received = append(p.received, frame)
}

// The frame geometry this helper matches on, from the matrix's own §3.10
// data-format citation rather than derived from the code under test.
//
//	1A 00 read : FE FE 74 E0 1A 00 <ch-hi> <ch-lo> FD          —  9 bytes
//	1A 00 set  : FE FE 74 E0 1A 00 <ch-hi> <ch-lo> <39> FD     — 48 bytes
//	19 00 read : FE FE 74 E0 19 00 FD                           —  7 bytes
const (
	memReadFrameLen = 9
	memSetFrameLen  = 48
	idReadFrameLen  = 7
	radioAddr       = 0x74
)

func (img radioImage) reply(frame []byte) [][]byte {
	switch {
	case len(frame) == idReadFrameLen && frame[4] == civ.CmdTransceiverID && frame[5] == civ.SubTransceiverID:
		if img.idToken == nil {
			return nil
		}
		from := img.idFrom
		if from == 0 {
			from = radioAddr
		}
		out := []byte{civ.PreambleByte, civ.PreambleByte, byte(civ.ControllerAddressDefault), from, civ.CmdTransceiverID, civ.SubTransceiverID}
		out = append(out, img.idToken...)
		return [][]byte{append(out, civ.EndByte)}

	case len(frame) == memReadFrameLen && frame[4] == civ.CmdMemory && frame[5] == civ.SubMemoryContents:
		asked := bcdChannel(frame[6], frame[7])
		rec, ok := img.records[asked]
		if !ok {
			return [][]byte{nakFrame(radioAddr)}
		}
		named := asked
		if img.answerAddress != nil {
			named = img.answerAddress(asked)
		}
		return [][]byte{memAnswerFrame(named, rec)}

	case len(frame) >= memSetFrameLen && frame[4] == civ.CmdMemory && frame[5] == civ.SubMemoryContents:
		from := img.setFrom
		if from == 0 {
			from = radioAddr
		}
		switch {
		case img.rejectSets:
			return [][]byte{nakFrame(from)}
		case img.ackSets:
			return [][]byte{ackFrame(from)}
		default:
			return nil
		}
	}
	return nil
}

func bcdChannel(hi, lo byte) int {
	return int(hi>>4)*1000 + int(hi&0x0F)*100 + int(lo>>4)*10 + int(lo&0x0F)
}

func encodeChannel(ch int) (hi, lo byte) {
	hi = byte((ch/1000)<<4 | (ch/100)%10)
	lo = byte(((ch/10)%10)<<4 | ch%10)
	return hi, lo
}

func memAnswerFrame(ch int, rec []byte) []byte {
	hi, lo := encodeChannel(ch)
	out := []byte{civ.PreambleByte, civ.PreambleByte, byte(civ.ControllerAddressDefault), radioAddr, civ.CmdMemory, civ.SubMemoryContents, hi, lo}
	out = append(out, rec...)
	return append(out, civ.EndByte)
}

func nakFrame(from byte) []byte {
	return []byte{civ.PreambleByte, civ.PreambleByte, byte(civ.ControllerAddressDefault), from, civ.NakByte, civ.EndByte}
}

func ackFrame(from byte) []byte {
	return []byte{civ.PreambleByte, civ.PreambleByte, byte(civ.ControllerAddressDefault), from, civ.AckByte, civ.EndByte}
}

func hexFrames(frames [][]byte) string {
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
