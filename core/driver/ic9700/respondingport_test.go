// SPDX-License-Identifier: GPL-3.0-or-later

package ic9700_test

import (
	"bytes"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/civ"
	civic9700 "github.com/gm5dna/open-rig-programmer/core/civ/ic9700"
	"github.com/gm5dna/open-rig-programmer/core/driver/ic9700"
	"github.com/gm5dna/open-rig-programmer/core/transport"
)

// broadcastAddress is the `to` byte of an Icom TRANSCEIVE broadcast: 00,
// meaning "everyone". Spec D5 entry 9, this model's own row (lift R5).
// core/civ declares no constant for it — the package never SENDS one, and
// its accumulator recognises a broadcast by the frame not being addressed
// to this controller rather than by this value — so the scripted radio
// spells it here, where the frames that carry it are built.
const broadcastAddress = 0x00

// recordingPort is this package's SCRIPTED radio: a net.Pipe whose remote
// end reassembles the CI-V frames the driver writes, answers each one from
// a table, and records every frame it received in order.
//
// IT IS DELIBERATELY NOT internal/fakeic9700 (Stage 3), and the difference
// is the whole reason it exists. A fake models a radio's STATE and is
// self-consistent by construction, which is exactly right for the
// end-to-end proof and exactly wrong here: the error paths need a radio
// that answers a 110-byte record, that replies from another station's
// address, that names the WRONG SLOT (T2), that never acknowledges a set,
// and that floods the bus in two different ways. A self-consistent fake
// will never produce any of those. core/driver/ftdx101's own
// respondingport_test.go is the house precedent.
//
// EVERYTHING IT SERVES IS AN ASSUMPTION APPLIED, NOT A RADIO OBSERVED. No
// IC-9700 has ever been connected to this project. That an empty channel
// answers `FA` is spec D5 entry 2(a) (lift R11); that a set is
// acknowledged with `FB` is the family's D5 form (lift W1); the record
// geometry is core/civ/ic9700's transcription. This file APPLIES those
// assumptions so the driver can be exercised against them; it is not
// evidence for any of them.

// answerKind names one of the radio's non-record answers, so a test can
// ask for it without building a frame.
//
// It exists because of T4: `Engine.Do` CONSUMES an `FA` and returns
// transport.ErrRejected with NO FRAME, so the driver never sees the
// rejection as a frame and a test must not hand it one. Naming the KIND
// keeps the test's intent ("this slot is empty") separate from the wire
// form that expresses it.
type answerKind int

const (
	// civFA is the radio's rejection: what an unwritten channel answers a
	// `1A 00` read with (spec D5 entry 2(a), lift R11).
	civFA answerKind = iota + 1
)

// radioImage is what a scripted radio "contains" and how it misbehaves.
//
// The fields divide into two groups, and the division matters. STORED
// STATE (records, fromAddr, idData, dataArea) is in force from the first
// byte, so the driver's Open probe sees it. ARMED FAULTS (misdirect,
// misdirectAllFF, rejectSets, silentSets) are honoured only after a test
// calls arm(), which the session helpers do immediately after Open — a
// fault in force during the probe would corrupt the very diagnostics the
// test is about (a global wrong-slot answer, for instance, would leave
// eight answer mismatches on the counter before the test's own read).
type radioImage struct {
	// fromAddr is the `from` byte of every answer. Zero means this
	// profile's own radio address, which is what a real IC-9700 sends.
	fromAddr byte
	// idData is the data of the `19 00` answer. Nil means the one byte a
	// radio at address A2 is assumed to send. The VALUE is never matched
	// by the driver (D5 entry 7, lift R6); it is here so the diagnostic
	// has something to record.
	idData []byte
	// silentID makes the radio ignore the `19 00` probe entirely.
	silentID bool
	// records maps an address to the RAW 111-byte record served for a
	// read of it. An address ABSENT from the map answers FA — which is
	// this radio's assumed empty-slot answer, and the same mechanism the
	// two are indistinguishable by on the wire.
	records map[civ.ChannelAddress][]byte
	// dataArea, when positive, overrides every read: the answer carries a
	// data area of exactly this many bytes (address INCLUDED). It is how
	// a test serves a record at a length this profile does not accept
	// without hand-writing the frame.
	dataArea int

	// misdirect, when set and armed, makes every read answer with THIS
	// address in the answer's address field, whatever was asked for. The
	// landed MemoryAnswerMatcher is envelope-only, so such a frame
	// reaches the driver and only the driver's own T2 comparison can
	// catch it.
	misdirect *civ.ChannelAddress
	// misdirectAllFF serves the misdirected answer as a full-length
	// record of 0xFF — the second empty-slot form (D5 entry 2(b)) at the
	// WRONG slot, which is the ordering trap T2 exists to close.
	misdirectAllFF bool
	// rejectSets answers every memory set with FA instead of FB.
	rejectSets bool
	// silentSets answers a memory set with nothing at all, which
	// ClassWriteWithAck must report as a failure WITHOUT resending.
	silentSets bool
}

// imageOption mutates a radioImage. The withXxx constructors below are the
// vocabulary the read and write tests describe a radio's state in.
type imageOption func(*radioImage)

// baseImage is a radio with nothing in it: no memories, the assumed ID
// answer, this profile's own address.
func baseImage(opts ...imageOption) radioImage {
	img := radioImage{records: map[civ.ChannelAddress][]byte{}}
	for _, opt := range opts {
		opt(&img)
	}
	return img
}

// factoryAnswers is a radio with a record in the first channel of the
// 144 MHz band, so Open's bounded occupied-slot search finds one on its
// first read and the session opens FINGERPRINTED.
func factoryAnswers(opts ...imageOption) radioImage {
	return baseImage(append([]imageOption{withTemplateStateAt("144-001")}, opts...)...)
}

// answersFrom is factoryAnswers from ANOTHER station's address: every
// answer is well-formed and none of it is ours.
func answersFrom(addr byte) radioImage {
	img := factoryAnswers()
	img.fromAddr = addr
	return img
}

// answersWithDataArea makes every `1A 00` read answer with a data area of
// exactly n bytes — the three address bytes plus n-3 record bytes.
//
// THE TWO NUMBERS ARE THE POINT (spec Erratum 1). The wire shows 114 bytes
// between `1A 00` and `FD` on an occupied channel; the profile declares a
// RECORD of 111. n = 114 is therefore the well-formed case and n = 111 is
// a 108-byte record — the characteristic bug of this model, dressed up as
// a plausible number.
func answersWithDataArea(n int) radioImage {
	img := factoryAnswers()
	img.dataArea = n
	return img
}

// allRejections is an EMPTY radio: every `1A 00` read answers FA, which is
// D5 entry 2(a) and is the state a factory radio is in.
func allRejections() radioImage {
	return baseImage()
}

// withTemplateStateAt stores, at slot, a record whose unmapped regions
// equal the profile's Fixed template and whose ④ reads OFF and ⑬ high
// nibble reads Duplex OFF — the ONE state the E6 template guard admits.
func withTemplateStateAt(slot string) imageOption {
	return func(img *radioImage) {
		addr := mustAddress(slot)
		img.records[addr] = templateRecord(addr)
	}
}

// withEmptySlot states positively that slot is unwritten. It is the
// absence of a record, spelt out, so a test reads as what it means rather
// than as an omission.
func withEmptySlot(slot string) imageOption {
	return func(img *radioImage) { delete(img.records, mustAddress(slot)) }
}

// withStoredRecord stores raw bytes at slot, for the cases that need a
// record no builder would produce.
func withStoredRecord(slot string, record []byte) imageOption {
	return func(img *radioImage) {
		img.records[mustAddress(slot)] = append([]byte(nil), record...)
	}
}

// mustAddress is slotAddress for a literal a test wrote. A failure is a
// typo in the test, not a condition to handle.
func mustAddress(slot string) civ.ChannelAddress {
	addr, _, err := ic9700.SlotAddress(slot)
	if err != nil {
		panic("ic9700 test: bad slot literal " + slot + ": " + err.Error())
	}
	return addr
}

// newRecordingPort starts a scripted radio serving img and registers its
// cleanup. The returned value's Port is what a driver Opens; the driver
// takes ownership of it, so a test never closes it itself.
func newRecordingPort(t *testing.T, img radioImage) *recordingPort {
	t.Helper()
	host, remote := net.Pipe()
	p := &recordingPort{host: host, remote: remote, img: img}
	t.Cleanup(func() {
		p.stop()
		_ = host.Close()
		_ = remote.Close()
	})
	go p.serve()
	return p
}

// newBroadcastFloodPort is a scripted radio that ALSO emits an unending
// stream of transceive broadcasts — `to = 00`, D5 entry 9's form.
//
// R9-SPLIT (a): the accumulator's address filter drops every one of them
// before the engine sees it, so the idle timer is never re-armed, Init
// returns nil, and the traffic appears ONLY in
// AccumulatorStats().Unexpected.
func newBroadcastFloodPort(t *testing.T, img radioImage) *recordingPort {
	t.Helper()
	p := newRecordingPort(t, img)
	p.startFlooding(broadcastAddress)
	return p
}

// newAddressedFloodPort is a scripted radio that emits an unending stream
// of frames addressed to the CONTROLLER.
//
// R9-SPLIT (b): those frames DO reach the engine, so they re-arm the idle
// timer and the drain fails its absolute cap — which is the only way to
// produce ErrDrainCapExceeded at all. Because they are returned to the
// engine they do NOT increment AccumulatorStats.Unexpected, which is why
// this port and the broadcast one cannot serve one test.
func newAddressedFloodPort(t *testing.T, img radioImage) *recordingPort {
	t.Helper()
	p := newRecordingPort(t, img)
	p.startFlooding(civ.ControllerAddressDefault)
	return p
}

// recordingPort is the scripted radio's host side plus its transcript.
type recordingPort struct {
	host   net.Conn
	remote net.Conn

	mu      sync.Mutex
	written [][]byte
	img     radioImage
	armed   bool

	floodOnce sync.Once
	stopOnce  sync.Once
	done      chan struct{}
	// answers serialises the serve goroutine's replies against the flood
	// goroutine's frames, both of which write to remote.
	answers sync.Mutex
}

// Port returns the end handed to the driver.
func (p *recordingPort) Port() transport.Port { return p.host }

// arm turns on the image's deliberate faults. The session helpers call it
// immediately after Open, so a fault cannot corrupt the probe it is not
// about.
func (p *recordingPort) arm() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.armed = true
}

// clearTranscript forgets every frame received so far.
//
// The session helpers call it after Open for one reason: Open legitimately
// writes a `19 00` and up to eight `1A 00` reads, and a write test asking
// "did a locally decidable refusal send any wire traffic?" (T5) means
// traffic THIS WRITE sent. Counting the probe's would make every such
// assertion vacuously false.
func (p *recordingPort) clearTranscript() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.written = nil
}

// frames returns a copy of every complete frame the port has received.
func (p *recordingPort) frames() [][]byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([][]byte, len(p.written))
	for i, f := range p.written {
		out[i] = append([]byte(nil), f...)
	}
	return out
}

// countReads counts the `1A 00` READ requests received: address, no
// record, ten bytes exactly.
func (p *recordingPort) countReads() int {
	n := 0
	for _, f := range p.frames() {
		if cn, sc, ok := civ.FrameCommand(f); ok && cn == 0x1A && sc == 0x00 && len(f) == 10 {
			n++
		}
	}
	return n
}

// countSets counts the `1A 00` SET frames received.
func (p *recordingPort) countSets() int {
	n := 0
	for _, f := range p.frames() {
		if cn, sc, ok := civ.FrameCommand(f); ok && cn == 0x1A && sc == 0x00 && len(f) > 10 {
			n++
		}
	}
	return n
}

// lastSet returns the most recent memory set frame received, or nil.
func (p *recordingPort) lastSet() []byte {
	var out []byte
	for _, f := range p.frames() {
		if cn, sc, ok := civ.FrameCommand(f); ok && cn == 0x1A && sc == 0x00 && len(f) > 10 {
			out = f
		}
	}
	return out
}

// startFlooding begins an unending stream of frames addressed to `to`,
// paced so that the stream never goes quiet for the framing's 200 ms idle
// gap while still leaving the scripted answers room to get out.
func (p *recordingPort) startFlooding(to byte) {
	p.floodOnce.Do(func() {
		p.done = make(chan struct{})
		go p.flood(to)
	})
}

// startAddressedFlooding is startFlooding's name at the one call site that
// needs it AFTER a session is open — the fail-closed half of R9-SPLIT.
func (p *recordingPort) startAddressedFlooding() {
	p.mu.Lock()
	p.img.silentID = true
	p.img.records = map[civ.ChannelAddress][]byte{}
	p.img.dataArea = 0
	p.mu.Unlock()
	p.startFlooding(civ.ControllerAddressDefault)
}

func (p *recordingPort) stop() {
	p.stopOnce.Do(func() {
		if p.done != nil {
			close(p.done)
		}
	})
}

// flood writes a transceive-shaped frequency broadcast every 20 ms. The
// SHAPE is D5 entry 9's (`00` = set frequency, five packed-BCD bytes);
// what matters to the two tests is only its `to` byte, which is what
// decides whether the accumulator drops it or the engine sees it.
func (p *recordingPort) flood(to byte) {
	frame := []byte{civ.PreambleByte, civ.PreambleByte, to, p.radioAddress(), 0x00,
		0x00, 0x00, 0x50, 0x45, 0x01, civ.EndByte}
	for {
		select {
		case <-p.done:
			return
		case <-time.After(20 * time.Millisecond):
		}
		p.answers.Lock()
		_, err := p.remote.Write(frame)
		p.answers.Unlock()
		if err != nil {
			return
		}
	}
}

// radioAddress is the `from` byte this image answers with.
func (p *recordingPort) radioAddress() byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.img.fromAddr != 0 {
		return p.img.fromAddr
	}
	return civic9700.Profile().RadioAddress()
}

// serve reassembles frames from the driver's bytes and answers each one.
//
// Frame splitting rather than whole-read matching: the transport writes
// one frame per call today, but nothing in the Port contract promises it,
// and a helper that assumed it would break confusingly the first time two
// frames shared a read.
func (p *recordingPort) serve() {
	buf := make([]byte, 512)
	var acc []byte
	for {
		n, err := p.remote.Read(buf)
		if n > 0 {
			acc = append(acc, buf[:n]...)
			for {
				start := bytes.Index(acc, []byte{civ.PreambleByte, civ.PreambleByte})
				if start < 0 {
					break
				}
				end := bytes.IndexByte(acc[start:], civ.EndByte)
				if end < 0 {
					break
				}
				frame := append([]byte(nil), acc[start:start+end+1]...)
				acc = acc[start+end+1:]
				p.record(frame)
				if reply := p.reply(frame); len(reply) > 0 {
					p.answers.Lock()
					_, werr := p.remote.Write(reply)
					p.answers.Unlock()
					if werr != nil {
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

func (p *recordingPort) record(frame []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.written = append(p.written, frame)
}

// reply is the radio's answer to one frame, or nil for silence.
func (p *recordingPort) reply(frame []byte) []byte {
	p.mu.Lock()
	img := p.img
	armed := p.armed
	p.mu.Unlock()

	from := img.fromAddr
	if from == 0 {
		from = civic9700.Profile().RadioAddress()
	}
	cn, sc, ok := civ.FrameCommand(frame)
	if !ok {
		return nil
	}
	switch {
	case cn == 0x19 && sc == 0x00:
		if img.silentID {
			return nil
		}
		data := img.idData
		if data == nil {
			data = []byte{civic9700.Profile().RadioAddress()}
		}
		return answerFrame(from, []byte{0x19, 0x00}, data)

	case cn == 0x1A && sc == 0x00 && len(frame) == 10:
		return p.readReply(img, armed, frame, from)

	case cn == 0x1A && sc == 0x00 && len(frame) > 10:
		if !armed {
			return ackFrame(from)
		}
		switch {
		case img.silentSets:
			return nil
		case img.rejectSets:
			return nakFrame(from)
		default:
			return ackFrame(from)
		}
	}
	return nakFrame(from)
}

// readReply answers one `1A 00` read.
func (p *recordingPort) readReply(img radioImage, armed bool, frame []byte, from byte) []byte {
	addrBytes := append([]byte(nil), frame[6:9]...)
	if img.dataArea > 0 {
		if img.dataArea == civic9700.DataAreaLength {
			addr := decodeTestAddress(addrBytes)
			return answerFrame(from, []byte{0x1A, 0x00}, append(addrBytes, templateRecord(addr)...))
		}
		body := append(addrBytes, make([]byte, img.dataArea-len(addrBytes))...)
		return answerFrame(from, []byte{0x1A, 0x00}, body)
	}
	if armed && img.misdirect != nil {
		other := encodeTestAddress(*img.misdirect)
		record := templateRecord(*img.misdirect)
		if img.misdirectAllFF {
			record = bytes.Repeat([]byte{0xFF}, civic9700.RecordLength)
		}
		return answerFrame(from, []byte{0x1A, 0x00}, append(other, record...))
	}
	record, ok := img.records[decodeTestAddress(addrBytes)]
	if !ok {
		return nakFrame(from)
	}
	return answerFrame(from, []byte{0x1A, 0x00}, append(addrBytes, record...))
}

// answerFrame wraps a command and body as a frame FROM the radio TO the
// controller. The direction is the whole difference between a command and
// its answer, and getting it the wrong way round is how a test ends up
// asserting that the driver reads its own echo.
func answerFrame(from byte, cmd, body []byte) []byte {
	out := []byte{civ.PreambleByte, civ.PreambleByte, civ.ControllerAddressDefault, from}
	out = append(out, cmd...)
	out = append(out, body...)
	return append(out, civ.EndByte)
}

// nakFrame is the six-byte FA: the radio refusing.
func nakFrame(from byte) []byte {
	return []byte{civ.PreambleByte, civ.PreambleByte, civ.ControllerAddressDefault, from, civ.NakByte, civ.EndByte}
}

// ackFrame is the six-byte FB: the radio accepting a set.
func ackFrame(from byte) []byte {
	return []byte{civ.PreambleByte, civ.PreambleByte, civ.ControllerAddressDefault, from, civ.AckByte, civ.EndByte}
}

// encodeTestAddress renders a (band, channel) address as the three wire
// bytes: one packed-BCD band code, then two packed-BCD channel bytes, most
// significant pair first.
//
// It is written out HERE rather than reached for in core/civ because
// core/civ does not export it — and because a test that hand-builds a
// deliberately WRONG answer must build the address the way the radio would
// have, not the way the driver would have. decodeTestAddress is its
// inverse and the two are exercised against each other by every read the
// scripted radio serves.
func encodeTestAddress(addr civ.ChannelAddress) []byte {
	return []byte{
		byte(addr.Group/10<<4 | addr.Group%10),
		byte(addr.Channel/1000<<4 | addr.Channel/100%10),
		byte(addr.Channel/10%10<<4 | addr.Channel%10),
	}
}

func decodeTestAddress(b []byte) civ.ChannelAddress {
	digit := func(x byte) int { return int(x>>4)*10 + int(x&0x0F) }
	return civ.ChannelAddress{
		Group:   digit(b[0]),
		Channel: digit(b[1])*100 + digit(b[2]),
	}
}

// templateRecord is the 111 raw bytes of the frozen golden record,
// re-addressed — the ONE channel state the E6 guard admits, since the
// profile's Fixed template was derived from these very bytes.
//
// It is obtained by BUILDING the frame with the profile's own builder and
// slicing the record out, so the test's idea of what a record looks like
// can never drift from the codec's.
func templateRecord(addr civ.ChannelAddress) []byte {
	frame, err := civic9700.Profile().BuildMemorySet(goldenRecordAt(addr.Group, addr.Channel))
	if err != nil {
		panic("ic9700 test: BuildMemorySet: " + err.Error())
	}
	b := frame.Bytes()
	// FE FE <to> <from> 1A 00 <3 address bytes> = 9, then the record,
	// then FD.
	return append([]byte(nil), b[9:len(b)-1]...)
}
