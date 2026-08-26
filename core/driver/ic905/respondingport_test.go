// SPDX-License-Identifier: GPL-3.0-or-later

package ic905

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// respondingPort is this package's scripted radio: a net.Pipe whose
// remote end PARSES the CI-V frames the driver writes and answers each
// one per a configurable image, recording every frame it received in
// order.
//
// It is core/driver/ftdx101/respondingport_test.go's shape, COPIED rather
// than imported — it is a test file, and copying is what keeps the two
// driver packages independent — and re-cut for CI-V's binary framing:
// FE FE <to> <from> <cn> [<sc> …] FD instead of a ';'-terminated ASCII
// line.
//
// It is deliberately NOT internal/fakeic905, which is Stage 3's and is
// authored by a different implementer. A fake radio models a radio's
// STATE and is the right tool for round-trip and end-to-end tests; this
// answers per-frame from a table and can therefore serve deliberately
// WRONG answers — a record at an undeclared length, an answer naming a
// different channel, an all-0xFF record, an identity reply from the wrong
// address, a flood of transceive broadcasts — which is exactly what the
// error paths need and what a self-consistent fake will never produce.
//
// THE ACKNOWLEDGEMENT SEMANTICS ARE AN ASSUMED CONVENTION APPLIED, NOT AN
// OBSERVED RADIO TRANSCRIBED. No IC-905 has ever been connected to this
// project, so nothing here is evidence of what one does. What IS printed,
// and is the only fully concrete byte sequence in the whole document, is
// the pair PDF p.3 (folio 2) draws: the OK message FE FE E0 AC FB FD and
// the NG message FE FE E0 AC FA FD. That an empty channel answers FA is
// D5 entry 2(a), ASSUMED, lift ic905-R-14; that an all-0xFF record means
// empty is the SEPARATE D5 entry 2(b), lift ic905-R-15.
type respondingPort struct {
	host   net.Conn
	remote net.Conn

	mu       sync.Mutex
	received [][]byte
	image    radioImage

	floodMu   sync.Mutex
	floodStop chan struct{}
}

// wireAddr is a memory channel's address as the RADIO numbers it: the
// wire group (0…99 for MEM, 100 for CALL) and the channel within it.
// Comparable, so it keys the image's maps.
type wireAddr struct {
	group   int
	channel int
}

// radioImage is what a respondingPort's radio "contains".
//
// Every field is a deliberate hole a test can reach through, because the
// paths this driver has to get right are the ones a cooperative radio
// never exercises.
type radioImage struct {
	// idToken is the DATA bytes of the 19 00 answer. Empty means the
	// radio does not answer 19 00 at all — the "nothing is there" case,
	// which must surface as a TIMEOUT and never as a wrong radio.
	idToken []byte
	// idFrom overrides the address the 19 00 answer claims to come from.
	// Zero means this profile's own 0xAC. A different value is a
	// different radio on the bus, whose reply the codec's matcher must
	// not accept.
	idFrom byte
	// idOnce answers 19 00 exactly once and then goes silent, so a test
	// can drive a LATER read into its retry-and-quarantine path.
	idOnce bool
	idDone bool

	// records maps a wire address to the RAW record bytes served for a
	// 1A 00 read of it. An address ABSENT from the map is answered FA —
	// the assumed empty-channel reply, D5 entry 2(a).
	records map[wireAddr][]byte
	// answerAddr overrides the address the memory ANSWER carries, per
	// requested address. It exists because civ.MemoryAnswerMatcher is
	// deliberately ENVELOPE-ONLY (ruling T2): an answer for another
	// channel satisfies the matcher, and the DRIVER is what must catch
	// it.
	answerAddr map[wireAddr]wireAddr

	// setOutcome chooses what a 1A 00 SET draws.
	setOutcome setOutcome

	// floodOnOpen starts a flood of frames addressed to floodTo as soon as
	// the radio starts serving. It is a SEPARATE flag rather than
	// "floodTo != 0", and it has to be: the transceive BROADCAST address
	// IS 0x00, so a zero-means-absent test would silently never run
	// branch (a) — the one case whose whole point is that those frames
	// exist and are dropped.
	floodOnOpen bool
	// floodTo is the address that flood carries: 0x00 for the transceive
	// broadcast case (R9-SPLIT branch (a)), 0xE0 for the
	// controller-addressed case (branch (b)).
	floodTo byte
	// floodFor bounds that flood. It must exceed civ.DrainCap for branch
	// (b) to reach the cap at all; zero selects floodDefault.
	floodFor time.Duration
}

// setOutcome is what the scripted radio does with a 1A 00 memory set.
type setOutcome int

const (
	// setAcknowledged answers the printed OK message, FE FE E0 AC FB FD.
	setAcknowledged setOutcome = iota
	// setRejected answers the printed NG message, FE FE E0 AC FA FD.
	setRejected
	// setIgnored answers nothing at all, which for a ClassWriteWithAck
	// exchange is a TIMEOUT with an unattributable outcome — never a
	// retransmission.
	setIgnored
)

// The CI-V frame's own bytes, from PDF p.3 (folio 2), "◇ About the data
// format". Written out here rather than taken from core/civ because a
// test fixture that derived its frame shape from the code under test
// would answer whatever that code asked for, including a wrong shape.
const (
	civPreamble   = 0xFE
	civTerminate  = 0xFD
	civRadio      = 0xAC // the IC-905's default address, PDF p.3 cell ②
	civController = 0xE0 // the controller's default address, cell ③
	civOK         = 0xFB // "OK code (fixed)"
	civNG         = 0xFA // "NG code (fixed)"
)

// floodDefault is how long a flood runs when the image does not say. It
// must comfortably exceed civ.DrainCap (3 × 200 ms) so a
// controller-addressed flood genuinely reaches the absolute cap, and must
// end so the rest of the test can proceed on a quiet line.
const floodDefault = 1200 * time.Millisecond

// newRespondingPort starts a scripted radio serving img and registers its
// cleanup. The returned value's Port is what a driver Opens.
func newRespondingPort(t *testing.T, img radioImage) *respondingPort {
	t.Helper()
	host, remote := net.Pipe()
	p := &respondingPort{host: host, remote: remote, image: img}
	t.Cleanup(func() {
		p.stopFlood()
		_ = host.Close()
		_ = remote.Close()
	})
	if img.floodOnOpen {
		d := img.floodFor
		if d == 0 {
			d = floodDefault
		}
		p.startFlood(img.floodTo, d)
	}
	go p.serve()
	return p
}

// Port returns the end handed to the driver. The driver takes ownership
// of it (Open closes it on failure; Session.Close on success), so a test
// must not close it itself — newRespondingPort's cleanup covers the rest.
func (p *respondingPort) Port() transport.Port { return p.host }

// Transcript returns a copy of every complete frame the port has
// received, in arrival order.
func (p *respondingPort) Transcript() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([][]byte, len(p.received))
	for i, f := range p.received {
		out[i] = bytes.Clone(f)
	}
	return out
}

// sets returns just the 1A 00 SET frames received — the frames a refusal
// test must find NONE of.
func (p *respondingPort) sets() [][]byte {
	var out [][]byte
	for _, f := range p.Transcript() {
		if len(f) > memoryReadFrameLen && len(f) >= 6 && f[4] == 0x1A && f[5] == 0x00 {
			out = append(out, f)
		}
	}
	return out
}

// memoryReadFrameLen is the 1A 00 READ request's length: FE FE AC E0
// 1A 00 + four address bytes + FD. Eleven bytes — the golden vector
// read-record is exactly that, and matrix Erratum 5 corrects an earlier
// slip that called it ten.
const memoryReadFrameLen = 11

// startFlood writes a frame addressed to `to` every few milliseconds for
// d, then stops.
//
// TWO ADDRESSES, TWO DIFFERENT OUTCOMES, and that is the whole of ruling
// R9-SPLIT. A frame addressed to 0x00 is a transceive broadcast: the
// accumulator's address filter drops it before any engine event, so the
// idle timer never re-arms and Init succeeds. A frame addressed to the
// CONTROLLER reaches the engine, re-arms the timer on every arrival, and
// drives the drain into its absolute cap.
func (p *respondingPort) startFlood(to byte, d time.Duration) {
	p.floodMu.Lock()
	stop := make(chan struct{})
	p.floodStop = stop
	p.floodMu.Unlock()

	// A frame the driver never asked for and no matcher accepts: an
	// unsolicited 1C 00 (transceiver status) push from the radio.
	frame := civFrame(to, civRadio, 0x1C, 0x00, 0x01)
	go func() {
		deadline := time.Now().Add(d)
		for time.Now().Before(deadline) {
			select {
			case <-stop:
				return
			default:
			}
			if _, err := p.remote.Write(frame); err != nil {
				return
			}
			time.Sleep(4 * time.Millisecond)
		}
	}()
}

// stopFlood ends any running flood. Idempotent.
func (p *respondingPort) stopFlood() {
	p.floodMu.Lock()
	defer p.floodMu.Unlock()
	if p.floodStop != nil {
		close(p.floodStop)
		p.floodStop = nil
	}
}

// misdirect makes the radio answer a read of requested with a record
// addressed to answered — the stale-reply shape the codec's ENVELOPE-ONLY
// matcher cannot see (ruling T2), and which the driver must catch.
func (p *respondingPort) misdirect(requested, answered wireAddr) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.image.answerAddr == nil {
		p.image.answerAddr = map[wireAddr]wireAddr{}
	}
	p.image.answerAddr[requested] = answered
}

// silence makes the radio stop answering 19 00, so a later read can be
// driven into its retry-and-quarantine path.
func (p *respondingPort) silence() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.image.idDone = true
}

// serve reads the driver's bytes, splits them into FE FE … FD frames,
// records each, and writes back whatever the image says.
//
// Frame splitting rather than whole-read matching: the transport writes
// one frame per call today, but nothing in the Port contract promises
// that, and a helper that assumed it would break confusingly the first
// time two frames shared a read.
func (p *respondingPort) serve() {
	buf := make([]byte, 512)
	var acc []byte
	for {
		n, err := p.remote.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			for {
				frame, rest, ok := cutCIVFrame(acc)
				if !ok {
					break
				}
				acc = rest
				p.record(frame)
				for _, reply := range p.reply(frame) {
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

// cutCIVFrame takes the first complete FE FE … FD frame off acc.
func cutCIVFrame(acc []byte) (frame, rest []byte, ok bool) {
	start := bytes.Index(acc, []byte{civPreamble, civPreamble})
	if start < 0 {
		return nil, acc, false
	}
	end := bytes.IndexByte(acc[start:], civTerminate)
	if end < 0 {
		return nil, acc, false
	}
	end += start
	return bytes.Clone(acc[start : end+1]), acc[end+1:], true
}

// record appends one received frame to the transcript.
func (p *respondingPort) record(frame []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.received = append(p.received, bytes.Clone(frame))
}

// reply returns the frames this image answers frame with — none for
// silence.
func (p *respondingPort) reply(frame []byte) [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	img := &p.image

	if len(frame) < 6 {
		return nil
	}
	cn, sc := frame[4], frame[5]

	switch {
	case cn == 0x19 && sc == 0x00:
		if len(img.idToken) == 0 || img.idDone {
			return nil
		}
		if img.idOnce {
			img.idDone = true
		}
		from := img.idFrom
		if from == 0 {
			from = civRadio
		}
		return [][]byte{civFrame(civController, from, append([]byte{0x19, 0x00}, img.idToken...)...)}

	case cn == 0x1A && sc == 0x00 && len(frame) == memoryReadFrameLen:
		addr := decodeWireAddr(frame[6:10])
		rec, populated := img.records[addr]
		if !populated {
			// The ASSUMED empty-channel reply, D5 entry 2(a), lift
			// ic905-R-14.
			return [][]byte{civFrame(civController, civRadio, civNG)}
		}
		answered := addr
		if alt, ok := img.answerAddr[addr]; ok {
			answered = alt
		}
		payload := append([]byte{0x1A, 0x00}, encodeWireAddr(answered)...)
		payload = append(payload, rec...)
		return [][]byte{civFrame(civController, civRadio, payload...)}

	case cn == 0x1A && sc == 0x00:
		// Anything longer than the read request is a memory SET.
		switch img.setOutcome {
		case setRejected:
			return [][]byte{civFrame(civController, civRadio, civNG)}
		case setIgnored:
			return nil
		default:
			return [][]byte{civFrame(civController, civRadio, civOK)}
		}
	}
	// Anything else: the printed NG message. A task that adds a command
	// class and forgets to teach this helper about it therefore sees its
	// frame REJECTED, loudly and immediately, rather than silently
	// succeeding.
	return [][]byte{civFrame(civController, civRadio, civNG)}
}

// civFrame assembles one CI-V frame: FE FE <to> <from> <payload…> FD
// (PDF p.3, folio 2, "◇ About the data format").
func civFrame(to, from byte, payload ...byte) []byte {
	f := make([]byte, 0, 5+len(payload))
	f = append(f, civPreamble, civPreamble, to, from)
	f = append(f, payload...)
	return append(f, civTerminate)
}

// bcdPair renders n as two packed-BCD bytes, most significant first —
// the form PDF p.19 (folio 18) prints for both the memory group (①,②)
// and the memory channel (③,④): group 0 is `00 00`, channel 1 is
// `00 01`, and the CALL group 100 is `01 00`.
func bcdPair(n int) []byte {
	return []byte{
		byte((n/1000%10)<<4 | (n / 100 % 10)),
		byte((n/10%10)<<4 | (n % 10)),
	}
}

// bcdPairValue reads two packed-BCD bytes back to a number.
func bcdPairValue(b []byte) int {
	return int(b[0]>>4)*1000 + int(b[0]&0x0F)*100 + int(b[1]>>4)*10 + int(b[1]&0x0F)
}

// encodeWireAddr renders a wireAddr as the four address bytes ①~④.
func encodeWireAddr(a wireAddr) []byte {
	return append(bcdPair(a.group), bcdPair(a.channel)...)
}

// decodeWireAddr reads the four address bytes back.
func decodeWireAddr(b []byte) wireAddr {
	return wireAddr{group: bcdPairValue(b[0:2]), channel: bcdPairValue(b[2:4])}
}
