// SPDX-License-Identifier: GPL-3.0-or-later

package ic9100

import (
	"bytes"
	"net"
	"sync"
	"testing"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic9100 "github.com/gm5dna/open-rig-programmer/core/civ/ic9100"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// scriptedPort is this package's scripted radio: a net.Pipe whose remote
// end splits the CI-V frames the driver writes, answers each one from a
// TABLE, and records every frame it received in order.
//
// IT IS A TABLE, NOT A RADIO (core/driver/ic7760's respondingPort makes
// the same argument, and this file follows its shape). No internal/fake
// package exists for this brand-new model, and the brief deliberately
// does not want one — this in-package fixture is what lets the error
// paths (a reply from the wrong channel, a record at the wrong length, an
// unmapped region that disagrees) be exercised directly.
//
// EVERY ANSWER'S SEMANTICS ARE THIS DOCUMENT'S CONVENTION APPLIED, NOT AN
// OBSERVED RADIO TRANSCRIBED. No IC-9100 has ever been connected to this
// project.
type scriptedPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received [][]byte

	writeMu sync.Mutex
}

// bandChannel is a scriptedPort record key: band 0-2, channel 1-99.
type bandChannel struct{ band, channel int }

// radioImage is what a scriptedPort's radio "contains".
type radioImage struct {
	// idToken is the DATA bytes of the 19 00 answer. Nil means the radio
	// answers 19 00 with SILENCE (wrong-address/wrong-baud failure mode).
	idToken []byte
	// idFrom overrides the address the 19 00 answer comes FROM. Zero
	// means the ordinary 0x7C.
	idFrom byte

	// records maps a band+channel to the RAW record bytes served for a
	// 1A 00 read of it. Absent means the radio answers FA.
	records map[bandChannel][]byte
	// answerAddress, when non-nil, overrides the band+channel a memory
	// answer NAMES, so a test can serve an answer for an address nobody
	// asked for (tier ruling T2).
	answerAddress func(asked bandChannel) bandChannel
	// ackSets makes a 1A 00 SET answer FB. rejectSets makes it answer FA.
	// Both false means SILENCE — an acknowledgement timeout.
	ackSets    bool
	rejectSets bool
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

// Frame geometry, from the plan's own table (doc.go), never derived from
// the code under test:
//
//	19 00 read : FE FE 7C E0 19 00 FD                              — 7 bytes
//	1A 00 read : FE FE 7C E0 1A 00 <band><ch-hi><ch-lo> FD          — 10 bytes
//	1A 00 set  : FE FE 7C E0 1A 00 <band><ch-hi><ch-lo><57> FD      — 67 bytes
const (
	idReadFrameLen  = 7
	memReadFrameLen = 10
	memSetFrameLen  = 4 + 2 + 3 + civic9100.RecordLength + 1
)

func (img radioImage) reply(frame []byte) [][]byte {
	switch {
	case len(frame) == idReadFrameLen && frame[4] == civ.CmdTransceiverID && frame[5] == civ.SubTransceiverID:
		if img.idToken == nil {
			return nil
		}
		from := img.idFrom
		if from == 0 {
			from = 0x7C
		}
		out := []byte{civ.PreambleByte, civ.PreambleByte, civ.ControllerAddressDefault, from, civ.CmdTransceiverID, civ.SubTransceiverID}
		out = append(out, img.idToken...)
		return [][]byte{append(out, civ.EndByte)}

	case len(frame) == memReadFrameLen && frame[4] == civ.CmdMemory && frame[5] == civ.SubMemoryContents:
		asked := decodeBandChannel(frame[6], frame[7], frame[8])
		rec, ok := img.records[asked]
		if !ok {
			return [][]byte{nakFrame(0x7C)}
		}
		named := asked
		if img.answerAddress != nil {
			named = img.answerAddress(asked)
		}
		return [][]byte{memAnswerFrame(named, rec)}

	case len(frame) >= memSetFrameLen && frame[4] == civ.CmdMemory && frame[5] == civ.SubMemoryContents:
		switch {
		case img.rejectSets:
			return [][]byte{nakFrame(0x7C)}
		case img.ackSets:
			return [][]byte{ackFrame(0x7C)}
		default:
			return nil
		}
	}
	return nil
}

func decodeBandChannel(band, hi, lo byte) bandChannel {
	return bandChannel{band: int(band>>4)*10 + int(band&0x0F), channel: int(hi>>4)*1000 + int(hi&0x0F)*100 + int(lo>>4)*10 + int(lo&0x0F)}
}

func encodeBandChannel(a bandChannel) (band, hi, lo byte) {
	band = byte((a.band/10)<<4 | a.band%10)
	hi = byte((a.channel/1000)<<4 | (a.channel/100)%10)
	lo = byte(((a.channel/10)%10)<<4 | a.channel%10)
	return band, hi, lo
}

func memAnswerFrame(a bandChannel, rec []byte) []byte {
	band, hi, lo := encodeBandChannel(a)
	out := []byte{civ.PreambleByte, civ.PreambleByte, civ.ControllerAddressDefault, 0x7C, civ.CmdMemory, civ.SubMemoryContents, band, hi, lo}
	out = append(out, rec...)
	return append(out, civ.EndByte)
}

func nakFrame(from byte) []byte {
	return []byte{civ.PreambleByte, civ.PreambleByte, civ.ControllerAddressDefault, from, civ.NakByte, civ.EndByte}
}

func ackFrame(from byte) []byte {
	return []byte{civ.PreambleByte, civ.PreambleByte, civ.ControllerAddressDefault, from, civ.AckByte, civ.EndByte}
}
