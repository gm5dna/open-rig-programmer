// SPDX-License-Identifier: GPL-3.0-or-later

package ic7760

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

// scriptedPort is this package's scripted radio: a net.Pipe whose remote
// end splits the CI-V frames the driver writes, answers each one from a
// TABLE, and records every frame it received in order.
//
// IT IS A TABLE, NOT A RADIO, and that is the point (core/driver/ftdx101's
// respondingPort makes the same argument). internal/fakeic7760 arrives at
// Task 13 and models a radio's STATE, which is the right tool for
// round-trip and end-to-end work. This one answers per frame from a table
// and can therefore serve deliberately WRONG answers — a reply from
// another address, a broadcast, a record at a length this radio does not
// use, an answer naming a channel nobody asked for — which is exactly what
// the error paths need and what a self-consistent fake will never produce.
//
// IT ALSO FLOODS. The init-under-flood rule's two halves are distinguished by the flood's
// `to` byte, so the flood is a first-class feature here: startFlood emits
// frames continuously until stopFlood, and the two addresses produce
// genuinely different behaviour one layer down (the accumulator drops a
// to=00 broadcast before any engine event; a to=E0 frame reaches the
// engine and postpones its drain).
//
// EVERY ANSWER'S SEMANTICS ARE THIS DOCUMENT'S CONVENTION APPLIED, NOT AN
// OBSERVED RADIO TRANSCRIBED. No IC-7760 has ever been connected to this
// project, so nothing in this file is evidence about what one does.
type scriptedPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received [][]byte
	// rtsCalls and dtrCalls count control-line toggles. transport.Port is
	// an io.ReadWriteCloser and carries neither method, so a driver can
	// only reach them by type-asserting for them — which is precisely
	// what TestOpen_ControlLinesAreNeverToggled exists to catch.
	rtsCalls int
	dtrCalls int

	// writeMu serialises the two writers of remote: serve's answers and
	// flood's unsolicited frames.
	writeMu sync.Mutex

	floodStop chan struct{}
	floodOnce sync.Once
	floodDone sync.WaitGroup
}

// radioImage is what a scriptedPort's radio "contains".
type radioImage struct {
	// idToken is the DATA bytes of the 19 00 answer — what
	// Profile.ParseTransceiverID renders as a hex token. A nil idToken
	// means the radio answers 19 00 with SILENCE, which is the
	// wrong-address and wrong-baud failure mode.
	idToken []byte
	// idFrom overrides the address the 19 00 answer comes FROM. Zero
	// means the ordinary 0xB2.
	idFrom byte
	// idBroadcast addresses the 19 00 answer TO 0x00 — a transceive
	// broadcast rather than a reply to this controller. A separate flag
	// rather than an idTo byte, because 0x00 IS the interesting value and
	// a zero-means-default byte field could not express it.
	idBroadcast bool

	// records maps a channel number to the RAW record bytes served for a
	// 1A 00 read of it. A channel ABSENT from the map is answered FA,
	// which under tier ruling T4 the engine consumes into
	// transport.ErrRejected with no frame at all.
	records map[int][]byte
	// answerAddress, when non-nil, overrides the CHANNEL a memory answer
	// names — so a test can serve an answer for a channel nobody asked
	// for (tier ruling T2).
	answerAddress func(asked int) int
	// memSilent makes every 1A 00 read go unanswered.
	memSilent bool
	// ackSets makes a 1A 00 SET answer FB (accepted). rejectSets makes it
	// answer FA. Both false means SILENCE, which for an acknowledged
	// write is an ack timeout. Task 12's surface.
	ackSets    bool
	rejectSets bool
	// setFrom overrides the address a set's FA/FB answer comes FROM, so a
	// test can serve a rejection from a foreign station (E1's
	// source-address check). Zero means 0xB2.
	setFrom byte
}

// newScriptedPort starts a scripted radio serving img and registers its
// cleanup. The returned value's Port is what a driver Opens.
func newScriptedPort(t *testing.T, img radioImage) *scriptedPort {
	t.Helper()
	host, remote := net.Pipe()
	p := &scriptedPort{host: host, remote: remote, floodStop: make(chan struct{})}
	t.Cleanup(func() {
		p.stopFlood()
		p.floodDone.Wait()
		_ = host.Close()
		_ = remote.Close()
	})
	go p.serve(img)
	return p
}

// Port returns the end handed to the driver. The driver takes ownership of
// it (Open closes it on failure, Session.Close on success), so a test must
// not close it itself — newScriptedPort's cleanup covers the rest.
func (p *scriptedPort) Port() transport.Port { return p.host }

// SetRTS and SetDTR exist ONLY so a test can prove the driver never calls
// them. They are not part of transport.Port; a driver reaches them by
// type-asserting, and this driver must never do so — matrix §3.2 /
// ADDED-2: on this radio RTS and DTR on the USB serial ports are
// ASSIGNABLE TX-KEYING OUTPUTS, so a driver that asserts either when it
// opens the CI-V port can key the transmitter of a radio whose owner has
// set USB SEND to that line.
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

// Transcript returns a copy of every complete frame the port has received,
// in arrival order.
func (p *scriptedPort) Transcript() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([][]byte, len(p.received))
	for i, f := range p.received {
		out[i] = append([]byte(nil), f...)
	}
	return out
}

// startFlood emits an unsolicited frame addressed to `to` every 2 ms until
// stopFlood, so a drain that re-arms on activity never finds its idle gap.
//
// TWO ADDRESSES, TWO BEHAVIOURS (the init-under-flood rule):
//   - to = 0x00 is a transceive BROADCAST. civ's accumulator counts it and
//     NEVER RETURNS it, so it never becomes an engine event, the idle
//     timer is never re-armed, and a drain completes normally.
//   - to = 0xE0 is addressed to THIS CONTROLLER. It passes the address
//     filter, becomes an engine event, re-arms the drain's idle timer, and
//     so drives the drain to its absolute cap.
func (p *scriptedPort) startFlood(to byte) {
	p.floodDone.Add(1)
	go func() {
		defer p.floodDone.Done()
		// A 19 00 answer shape: well-formed, six bytes of envelope plus
		// one data byte. Addressed to `to`, which is what decides its
		// fate one layer down.
		frame := []byte{civ.PreambleByte, civ.PreambleByte, to, 0xB2, 0x19, 0x00, 0xB2, civ.EndByte}
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

// stopFlood ends any flood. Idempotent, and called by the cleanup.
func (p *scriptedPort) stopFlood() { p.floodOnce.Do(func() { close(p.floodStop) }) }

// stopFloodAfter ends the flood d from now, so a test can have a flood
// that outlives Init's drain and stops before the probe needs an answer.
func (p *scriptedPort) stopFloodAfter(d time.Duration) {
	p.floodDone.Add(1)
	go func() {
		defer p.floodDone.Done()
		select {
		case <-p.floodStop:
		case <-time.After(d):
			p.stopFlood()
		}
	}()
}

// write puts one frame on the wire under writeMu, reporting whether the
// pipe is still alive.
func (p *scriptedPort) write(frame []byte) bool {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	_, err := p.remote.Write(frame)
	return err == nil
}

// serve reads the driver's bytes, splits them into FD-terminated frames,
// records each, and writes back whatever img says.
//
// Frame splitting rather than whole-read matching: the transport writes one
// frame per call today, but nothing in the Port contract promises that.
// 0xFD is a safe terminator to scan for because CI-V reserves it: every
// data byte this radio's records carry is packed BCD, an enum value below
// 0x14, or a printable ASCII name byte.
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

// The frame geometry this helper matches on, written out here from the
// plan's ONE TABLE rather than derived from the code under test: a fixture
// that asked the builder what shape to expect would answer whatever that
// builder produced, including a wrong shape.
//
//	1A 00 read : FE FE B2 E0 1A 00 <ch-hi> <ch-lo> FD          — 9 bytes
//	1A 00 set  : FE FE B2 E0 1A 00 <ch-hi> <ch-lo> <25> FD     — 34 bytes
//	19 00 read : FE FE B2 E0 19 00 FD                          — 7 bytes
const (
	memReadFrameLen = 9
	memSetFrameLen  = 34
	idReadFrameLen  = 7
)

// reply returns the frames this image answers frame with. An empty result
// is SILENCE.
func (img radioImage) reply(frame []byte) [][]byte {
	switch {
	case len(frame) == idReadFrameLen && frame[4] == civ.CmdTransceiverID && frame[5] == civ.SubTransceiverID:
		if img.idToken == nil {
			return nil
		}
		from := img.idFrom
		if from == 0 {
			from = 0xB2
		}
		to := byte(civ.ControllerAddressDefault)
		if img.idBroadcast {
			to = 0x00
		}
		out := []byte{civ.PreambleByte, civ.PreambleByte, to, from, civ.CmdTransceiverID, civ.SubTransceiverID}
		out = append(out, img.idToken...)
		return [][]byte{append(out, civ.EndByte)}

	case len(frame) == memReadFrameLen && frame[4] == civ.CmdMemory && frame[5] == civ.SubMemoryContents:
		if img.memSilent {
			return nil
		}
		asked := bcdChannel(frame[6], frame[7])
		rec, ok := img.records[asked]
		if !ok {
			return [][]byte{nakFrame(0xB2)}
		}
		named := asked
		if img.answerAddress != nil {
			named = img.answerAddress(asked)
		}
		return [][]byte{memAnswerFrame(named, rec)}

	case len(frame) >= memSetFrameLen && frame[4] == civ.CmdMemory && frame[5] == civ.SubMemoryContents:
		from := img.setFrom
		if from == 0 {
			from = 0xB2
		}
		switch {
		case img.rejectSets:
			return [][]byte{nakFrame(from)}
		case img.ackSets:
			return [][]byte{ackFrame(from)}
		default:
			return nil // silence: an acknowledged write's ack timeout
		}
	}
	return nil
}

// bcdChannel decodes the two-byte packed-BCD channel selector this radio's
// 1A 00 frames carry. Written out here for the reason the frame lengths
// are: a fixture must read the wire the way the DOCUMENT describes it, not
// the way the codec happens to.
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
	out := []byte{civ.PreambleByte, civ.PreambleByte, civ.ControllerAddressDefault, 0xB2, civ.CmdMemory, civ.SubMemoryContents, hi, lo}
	out = append(out, rec...)
	return append(out, civ.EndByte)
}

// nakFrame and ackFrame are the six-byte refusal and acknowledgement.
func nakFrame(from byte) []byte {
	return []byte{civ.PreambleByte, civ.PreambleByte, civ.ControllerAddressDefault, from, civ.NakByte, civ.EndByte}
}

func ackFrame(from byte) []byte {
	return []byte{civ.PreambleByte, civ.PreambleByte, civ.ControllerAddressDefault, from, civ.AckByte, civ.EndByte}
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
