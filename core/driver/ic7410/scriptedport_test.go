// SPDX-License-Identifier: GPL-3.0-or-later

package ic7410

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
// IT IS A TABLE, NOT A RADIO. do NOT write internal/fakeic7410 (brief):
// this in-package scripted port answers per frame from a table and can
// therefore serve deliberately WRONG answers — a reply from another
// address, a record at a length this radio does not use, an answer naming
// a channel nobody asked for — which is exactly what the error paths need.
//
// EVERY ANSWER'S SEMANTICS ARE THIS DOCUMENT'S CONVENTION APPLIED, NOT AN
// OBSERVED RADIO TRANSCRIBED. No IC-7410 has ever been connected to this
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
	// idToken is the DATA bytes of the 19 00 answer. A nil idToken means
	// SILENCE — the wrong-address/wrong-baud failure mode.
	idToken []byte
	// idFrom overrides the address the 19 00 answer comes FROM. Zero means
	// the ordinary 0x80.
	idFrom byte

	// records maps a channel number to the RAW record bytes served for a
	// 1A 00 read of it. A channel ABSENT from the map is answered FA
	// (tier ruling T4).
	records map[int][]byte
	// answerAddress, when non-nil, overrides the CHANNEL a memory answer
	// names, exercising tier ruling T2.
	answerAddress func(asked int) int
	// ackSets makes a 1A 00 SET answer FB. rejectSets makes it answer FA.
	// Both false means SILENCE, an ack timeout.
	ackSets    bool
	rejectSets bool
	setFrom    byte
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

// The frame geometry, written out from this model's own shape:
//
//	1A 00 read : FE FE 80 E0 1A 00 <ch-hi> <ch-lo> FD          — 9 bytes
//	1A 00 set  : FE FE 80 E0 1A 00 <ch-hi> <ch-lo> <40> FD     — 49 bytes
//	19 00 read : FE FE 80 E0 19 00 FD                          — 7 bytes
const (
	memReadFrameLen = 9
	memSetFrameLen  = 49
	idReadFrameLen  = 7
)

func (img radioImage) reply(frame []byte) [][]byte {
	switch {
	case len(frame) == idReadFrameLen && frame[4] == civ.CmdTransceiverID && frame[5] == civ.SubTransceiverID:
		if img.idToken == nil {
			return nil
		}
		from := img.idFrom
		if from == 0 {
			from = 0x80
		}
		out := []byte{civ.PreambleByte, civ.PreambleByte, byte(civ.ControllerAddressDefault), from, civ.CmdTransceiverID, civ.SubTransceiverID}
		out = append(out, img.idToken...)
		return [][]byte{append(out, civ.EndByte)}

	case len(frame) == memReadFrameLen && frame[4] == civ.CmdMemory && frame[5] == civ.SubMemoryContents:
		asked := bcdChannel(frame[6], frame[7])
		rec, ok := img.records[asked]
		if !ok {
			return [][]byte{nakFrame(0x80)}
		}
		named := asked
		if img.answerAddress != nil {
			named = img.answerAddress(asked)
		}
		return [][]byte{memAnswerFrame(named, rec)}

	case len(frame) >= memSetFrameLen && frame[4] == civ.CmdMemory && frame[5] == civ.SubMemoryContents:
		from := img.setFrom
		if from == 0 {
			from = 0x80
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
	out := []byte{civ.PreambleByte, civ.PreambleByte, byte(civ.ControllerAddressDefault), 0x80, civ.CmdMemory, civ.SubMemoryContents, hi, lo}
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

// e2eFields is one slot's content in NEUTRAL terms — what the scripted
// radio "contains" — so each test states what it put in the radio rather
// than what a builder produced.
type e2eFields struct {
	freqHz, txFreqHz uint64
	mode, filter     string
	dataMode         string // "OFF"/"ON"
	toneMode         string
	toneTx, toneRx   uint64 // deci-Hz
	name             string
}

func bcdLE(n uint64, width int) []byte {
	out := make([]byte, width)
	for i := 0; i < width; i++ {
		lo := byte(n % 10)
		n /= 10
		hi := byte(n % 10)
		n /= 10
		out[i] = hi<<4 | lo
	}
	return out
}

func bcdBE(n uint64, width int) []byte {
	out := bcdLE(n, width)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// record assembles the 40-byte record BY OFFSET, from this model's own
// offset table (matrix §1b). Nothing here calls the profile's builders.
func (f e2eFields) record(t *testing.T) []byte {
	t.Helper()
	rec := make([]byte, 40)
	// byte 0: Select-memory + Split, UNMAPPED, left 0x00.
	copy(rec[1:6], bcdLE(f.freqHz, 5))
	rec[6] = e2eCode(t, "mode", f.mode)
	rec[7] = e2eCode(t, "filter", f.filter)
	rec[8] = e2eCode(t, "data_mode", f.dataMode)
	rec[9] = e2eCode(t, "tone_mode", f.toneMode) << 4
	copy(rec[10:13], bcdBE(f.toneTx, 3))
	copy(rec[13:16], bcdBE(f.toneRx, 3))
	copy(rec[16:21], bcdLE(f.txFreqHz, 5))
	// bytes 21-30: TX-duplicate-block remainder, UNMAPPED, left 0x00.
	for i := 0; i < 9; i++ {
		rec[31+i] = 0x20
	}
	copy(rec[31:40], f.name)
	return rec
}

func e2eCode(t *testing.T, kind, name string) byte {
	t.Helper()
	tables := map[string]map[string]byte{
		"mode":      {"LSB": 0x00, "USB": 0x01, "AM": 0x02, "CW": 0x03, "RTTY": 0x04, "FM": 0x05, "CW-R": 0x07, "RTTY-R": 0x08},
		"filter":    {"FIL1": 0x01, "FIL2": 0x02, "FIL3": 0x03},
		"data_mode": {"OFF": 0x00, "ON": 0x01},
		"tone_mode": {"OFF": 0x0, "TONE": 0x1, "TSQL": 0x2},
	}
	code, ok := tables[kind][name]
	if !ok {
		t.Fatalf("no %s code for %q", kind, name)
	}
	return code
}

// vector1 is a representative populated MEM channel, reused across tests.
var vector1 = e2eFields{
	freqHz: 14_250_000, txFreqHz: 14_750_000,
	mode: "USB", filter: "FIL1", dataMode: "ON",
	toneMode: "TONE", toneTx: 885, toneRx: 1000,
	name: "TEST 1",
}
