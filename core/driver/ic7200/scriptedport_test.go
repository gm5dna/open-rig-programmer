// SPDX-License-Identifier: GPL-3.0-or-later

package ic7200

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
// IT IS A TABLE, NOT A RADIO (core/driver/ic7610/scriptedport_test.go
// makes the same argument, which this package copies rather than
// building internal/fakeic7200, per the driver brief): it answers per
// frame and can therefore serve deliberately WRONG answers — a reply
// from another address, a record at a length this radio does not use, an
// answer naming a channel nobody asked for — which is exactly what the
// error paths need.
//
// EVERY ANSWER'S SEMANTICS ARE THIS DOCUMENT'S CONVENTION APPLIED, NOT AN
// OBSERVED RADIO TRANSCRIBED. No IC-7200 has ever been connected to this
// project.
type scriptedPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received [][]byte

	writeMu sync.Mutex
}

// radioImage is what a scriptedPort's radio "contains".
type radioImage struct {
	// idToken is the DATA bytes of the 19 00 answer. Nil means the radio
	// answers 19 00 with SILENCE.
	idToken []byte
	// records maps a channel number to the RAW record bytes served for a
	// 1A 00 read of it. A channel ABSENT from the map is answered FA.
	records map[int][]byte
	// answerAddress, when non-nil, overrides the CHANNEL a memory answer
	// names — so a test can serve an answer for a channel nobody asked
	// for (tier ruling T2).
	answerAddress func(asked int) int
	// ackSets makes a 1A 00 SET answer FB (accepted). rejectSets makes it
	// answer FA. Both false means SILENCE (an ack timeout).
	ackSets    bool
	rejectSets bool
}

// newScriptedPort starts a scripted radio serving img and registers its
// cleanup.
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

// Port returns the end handed to the driver.
func (p *scriptedPort) Port() transport.Port { return p.host }

// Transcript returns a copy of every complete frame the port has received.
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

// serve reads the driver's bytes, splits them into FD-terminated frames,
// records each, and writes back whatever img says.
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

// The frame geometry this helper matches on, from the matrix's own §3.7,
// §3.10 and §3.12 arithmetic:
//
//	1A 00 read : FE FE 76 E0 1A 00 <ch-hi> <ch-lo> FD               — 9 bytes
//	1A 00 set  : FE FE 76 E0 1A 00 <ch-hi> <ch-lo> <17> FD          — 26 bytes
//	19 00 read : FE FE 76 E0 19 00 FD                               — 7 bytes
const (
	memReadFrameLen = 9
	memSetFrameLen  = 26
	idReadFrameLen  = 7
)

// reply returns the frames this image answers frame with. An empty
// result is SILENCE.
func (img radioImage) reply(frame []byte) [][]byte {
	switch {
	case len(frame) == idReadFrameLen && frame[4] == civ.CmdTransceiverID && frame[5] == civ.SubTransceiverID:
		if img.idToken == nil {
			return nil
		}
		out := []byte{civ.PreambleByte, civ.PreambleByte, byte(civ.ControllerAddressDefault), 0x76, civ.CmdTransceiverID, civ.SubTransceiverID}
		out = append(out, img.idToken...)
		return [][]byte{append(out, civ.EndByte)}

	case len(frame) == memReadFrameLen && frame[4] == civ.CmdMemory && frame[5] == civ.SubMemoryContents:
		asked := bcdChannel(frame[6], frame[7])
		rec, ok := img.records[asked]
		if !ok {
			return [][]byte{nakFrame()}
		}
		named := asked
		if img.answerAddress != nil {
			named = img.answerAddress(asked)
		}
		return [][]byte{memAnswerFrame(named, rec)}

	case len(frame) >= memSetFrameLen && frame[4] == civ.CmdMemory && frame[5] == civ.SubMemoryContents:
		switch {
		case img.rejectSets:
			return [][]byte{nakFrame()}
		case img.ackSets:
			return [][]byte{ackFrame()}
		default:
			return nil // silence: an acknowledged write's ack timeout
		}
	}
	return nil
}

// bcdChannel decodes the two-byte packed-BCD channel selector.
func bcdChannel(hi, lo byte) int {
	return int(hi>>4)*1000 + int(hi&0x0F)*100 + int(lo>>4)*10 + int(lo&0x0F)
}

// encodeChannel is bcdChannel's inverse.
func encodeChannel(ch int) (hi, lo byte) {
	hi = byte((ch/1000)<<4 | (ch/100)%10)
	lo = byte(((ch/10)%10)<<4 | ch%10)
	return hi, lo
}

// memAnswerFrame builds a 1A 00 ANSWER naming channel ch and carrying rec.
func memAnswerFrame(ch int, rec []byte) []byte {
	hi, lo := encodeChannel(ch)
	out := []byte{civ.PreambleByte, civ.PreambleByte, byte(civ.ControllerAddressDefault), 0x76, civ.CmdMemory, civ.SubMemoryContents, hi, lo}
	out = append(out, rec...)
	return append(out, civ.EndByte)
}

// nakFrame and ackFrame are the six-byte refusal and acknowledgement,
// from this radio's own address (matrix §3.10).
func nakFrame() []byte {
	return []byte{civ.PreambleByte, civ.PreambleByte, byte(civ.ControllerAddressDefault), 0x76, civ.NakByte, civ.EndByte}
}

func ackFrame() []byte {
	return []byte{civ.PreambleByte, civ.PreambleByte, byte(civ.ControllerAddressDefault), 0x76, civ.AckByte, civ.EndByte}
}

// hexFrames renders a transcript for a failure message.
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
