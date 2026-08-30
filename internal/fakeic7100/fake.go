// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic7100

import (
	"bytes"
	"io"
	"sync"
	"time"
)

// Radio is a fake IC-7100 on the far end of a pipe. It answers CI-V frames the
// way the printed page says the transceiver does, out of a memory image that
// holds only what a test seeded into it or wrote to it.
type Radio struct {
	cfg config

	// toRadio carries what the controller wrote; toController carries what the
	// radio said.
	toRadio      *pipe
	toController *pipe
	port         *port

	// mu guards the image and the transcript. The image is reachable from the
	// serve goroutine only, but both are read by whoever calls Slot or
	// Transcript, so they go under one lock rather than two.
	mu         sync.Mutex
	image      *image
	transcript [][]byte

	// writeMu serialises whole frames onto the wire, so that an answer and a
	// broadcast cannot interleave their bytes into one unparseable mess.
	writeMu sync.Mutex

	done      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// New returns a fake IC-7100 attached to a pipe-backed port.
//
// The radio starts answering immediately and keeps at most two further
// goroutines, one for each unsolicited-traffic species; Close stops all of
// them.
func New(opts ...Option) *Radio {
	cfg := config{
		radioAddress:   radioAddress,
		identityToken:  append([]byte(nil), defaultIdentityToken...),
		acceptedLength: recordLength,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	img := newImage()
	for _, s := range cfg.seeds {
		img.seed(s.addr, s.record, s.occupied)
	}

	r := &Radio{
		cfg:          cfg,
		toRadio:      newPipe(),
		toController: newPipe(),
		image:        img,
		done:         make(chan struct{}),
	}
	r.port = &port{radio: r}

	r.wg.Add(1)
	go r.serve()

	// The two species differ in exactly one byte — the to address — and that
	// byte is the whole point of having both. The CONTENT is the identity
	// answer, which is the one thing this fake can put on the wire without
	// claiming anything about a memory record; it asserts nothing, and neither
	// does its being the same in both.
	if d := cfg.broadcasts; d > 0 {
		r.wg.Add(1)
		go r.emitEvery(d, r.identityFrame(broadcastAddress))
	}
	if d := cfg.addressedFlood; d > 0 {
		r.wg.Add(1)
		go r.emitEvery(d, r.identityFrame(controllerAddress))
	}

	return r
}

// Port returns the reader/writer a driver opens onto this radio.
func (r *Radio) Port() io.ReadWriteCloser { return r.port }

// Close stops the radio and wakes anything blocked on its port. It is safe to
// call more than once, and safe to call from the port's own Close.
func (r *Radio) Close() error {
	r.closeOnce.Do(func() {
		close(r.done)
		r.toRadio.Close()
		r.toController.Close()
	})
	r.wg.Wait()
	return nil
}

// Transcript returns every frame the fake received, in order.
//
// Frames are recorded as normalised: exactly two preamble bytes, however many
// arrived, since the extra ones are a property of the line rate and not of what
// was said. EVERY received frame is recorded, including ones addressed to
// somebody else, which the radio then ignores — what a test wants to know is
// what reached the radio, not only what it chose to answer.
func (r *Radio) Transcript() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([][]byte, len(r.transcript))
	for i, f := range r.transcript {
		out[i] = append([]byte(nil), f...)
	}
	return out
}

// Slot returns the record the fake holds for one channel, and whether that
// channel is occupied at all. It is how a test asks what a write actually did.
//
// bank and channel are the printed rectangle's own numbers, 1-5 and 1-99; a
// pair outside it panics, exactly as WithSlot's does.
func (r *Radio) Slot(bank, channel int) ([]byte, bool) {
	addr := mustChannelAddress("Slot", bank, channel)
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.image.read(addr)
}

// serve reads the controller's bytes until the pipe closes, handing every whole
// frame to handle.
func (r *Radio) serve() {
	defer r.wg.Done()

	acc := newAccumulator()
	buf := make([]byte, 512)
	for {
		n, err := r.toRadio.Read(buf)
		if n > 0 {
			for _, body := range acc.feed(buf[:n]) {
				r.handle(body)
			}
		}
		if err != nil {
			return
		}
	}
}

// emitEvery sends the same frame every d until the radio closes.
func (r *Radio) emitEvery(d time.Duration, f []byte) {
	defer r.wg.Done()

	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-r.done:
			return
		case <-t.C:
			r.send(f)
		}
	}
}

// send writes one whole frame to the controller. A write to a closed pipe is
// ignored: a radio talking to a hung-up line is not an error the radio can do
// anything about.
func (r *Radio) send(f []byte) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	_, _ = r.toController.Write(f)
}

// identityFrame is the 19 00 answer, addressed to whoever is named.
func (r *Radio) identityFrame(to byte) []byte {
	data := make([]byte, 0, 2+len(r.cfg.identityToken))
	data = append(data, cmdTransceiverID, subTransceiverID)
	data = append(data, r.cfg.identityToken...)
	return buildFrame(to, r.cfg.radioAddress, data...)
}

// reject is the printed NG message, addressed back to whoever asked.
func (r *Radio) reject(to byte) []byte { return ngFrame(to, r.cfg.radioAddress) }

// acknowledge is the printed OK message, addressed back to whoever asked.
func (r *Radio) acknowledge(to byte) []byte { return okFrame(to, r.cfg.radioAddress) }

// handle records one received frame, echoes it if asked, and answers it.
func (r *Radio) handle(body []byte) {
	received := canonicalFrame(body)

	r.mu.Lock()
	r.transcript = append(r.transcript, received)
	r.mu.Unlock()

	if r.cfg.echoBack {
		r.send(received)
	}

	f, ok := parseFrame(body)
	if !ok {
		return
	}
	// A radio answers only what is addressed to it, and says nothing at all to
	// anything else — no NG, no silence-breaking of any kind. That includes
	// to=00 broadcasts, which are addressed to everyone and answered by no one,
	// and frames addressed to the controller, which are somebody else's answer.
	// See doc.go, entry 11.
	if f.to != r.cfg.radioAddress {
		return
	}
	r.answer(f)
}

// answer dispatches one frame addressed to this radio.
//
// The answer goes back to the frame's OWN from address rather than to the
// controller default: index (3) of the printed frame is the "Controller's
// default address", a default on a bus the manual says may carry up to four
// CI-V devices.
func (r *Radio) answer(f frame) {
	switch {
	case len(f.data) == 2 && f.data[0] == cmdTransceiverID && f.data[1] == subTransceiverID:
		r.send(r.identityFrame(f.from))

	case len(f.data) >= 2 && f.data[0] == cmdMemoryContent && f.data[1] == subMemoryContent:
		r.answerMemory(f, f.data[2:])

	default:
		// Every other command in the chapter's table — 1A 01, 1A 05, 0B, 18 01
		// and the rest — is refused. Several of them are real commands a real
		// radio would honour; refusing them is this tier's policy, not a fact
		// about the radio. See doc.go, entry 13.
		r.send(r.reject(f.from))
	}
}

// answerMemory handles a 1A 00 frame. The data block after the command and
// sub-command bytes opens with the three bytes that name a channel; what
// follows them decides which form this is:
//
//   - nothing at all: a read;
//   - the printed clearing form: refused;
//   - anything else: a set of that record.
//
// THAT "NOTHING AT ALL IS A READ" IS ASSUMED. The document prints one layout
// for 1A 00 — the complete record — and introduces it without distinguishing a
// read from a write; the command-table row names both directions and points at
// that same single layout. No shortened read form is printed anywhere. See
// doc.go, entry 9 (ic7100-read-request-form).
func (r *Radio) answerMemory(f frame, payload []byte) {
	// The clearing form is tested for FIRST, because one of its two printed
	// readings is the same length as an address and would otherwise be mistaken
	// for a read of a malformed one. Both readings are refused either way; the
	// order only decides which reason a future reader sees.
	if isClearForm(payload) {
		r.send(r.reject(f.from))
		return
	}
	if len(payload) < addressLength {
		r.send(r.reject(f.from))
		return
	}
	addr, rest := payload[:addressLength], payload[addressLength:]
	if !addressIsInScope(addr) {
		r.send(r.reject(f.from))
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(rest) == 0 {
		r.answerRead(f, addr)
		return
	}
	r.answerSet(f, addr, rest)
}

// answerRead serves one channel. Called with mu held.
func (r *Radio) answerRead(f frame, addr []byte) {
	record, occupied := r.image.read(addr)
	if !occupied {
		if !r.cfg.allFFEmpty {
			r.send(r.reject(f.from))
			return
		}
		record = bytes.Repeat([]byte{0xFF}, r.cfg.acceptedLength)
	}

	data := make([]byte, 0, 2+addressLength+len(record))
	data = append(data, cmdMemoryContent, subMemoryContent)
	data = append(data, addr...)
	data = append(data, record...)
	r.send(buildFrame(f.from, r.cfg.radioAddress, data...))
}

// answerSet stores one channel. Called with mu held.
//
// Under WithNoSetAnswer the store happens exactly as it always does and the
// acknowledgement alone goes missing — the lost-acknowledgement lever, not a
// radio declining to answer. A REFUSAL is untouched by it: what disappears is
// the FB, and a set that was never acceptable never had one to lose.
func (r *Radio) answerSet(f frame, addr, record []byte) {
	if !r.setIsAcceptable(record) {
		r.send(r.reject(f.from))
		return
	}
	r.image.write(addr, record)
	if r.cfg.noSetAnswer {
		return
	}
	r.send(r.acknowledge(f.from))
}

// setIsAcceptable applies the two rules this fake has about a record it is
// asked to store: its length, and — for a record of the derived length — the
// equality of its transmit duplicate with its receive payload.
//
// The equality rule is applied ONLY to a record of the derived length, because
// that is the only length in which this package knows where the two blocks sit.
// A test that moved the accepted length with WithAcceptedRecordLength is
// building a radio whose geometry this package does not claim to know, and
// checking a block position inside it would be inventing one.
func (r *Radio) setIsAcceptable(record []byte) bool {
	switch {
	case len(record) == r.cfg.acceptedLength:
	case r.cfg.shortSetsOK && len(record) < r.cfg.acceptedLength:
	default:
		return false
	}
	if len(record) == recordLength && !r.cfg.unequalTXOK && !txBlockMatchesRX(record) {
		return false
	}
	return true
}
