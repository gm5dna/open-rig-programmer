// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic9700

import (
	"io"
	"sync"
	"time"
)

// Radio is a fake IC-9700 on the far end of a pipe. It answers CI-V frames the
// way the printed wire facts say the transceiver does, out of a memory image
// that holds only what a test seeded into it.
type Radio struct {
	cfg config

	// toRadio carries what the controller wrote; toController carries what the
	// radio said.
	toRadio      *pipe
	toController *pipe
	port         *port

	// mu guards the image and the transcript. The image is reachable from the
	// serve goroutine only, but the transcript is read by whoever calls
	// Transcript, so both go under one lock rather than two.
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

// New returns a fake IC-9700 attached to a transport.Port-shaped pipe.
//
// The radio starts answering immediately and keeps two goroutines at most for
// the two flood species; Close stops all of them.
func New(opts ...Option) *Radio {
	var cfg config
	for _, opt := range opts {
		opt(&cfg)
	}

	img := newImage()
	for _, s := range cfg.seeds {
		img.seed(s.addr, s.record, s.occupied)
	}
	img.setServedLength(cfg.recordLength)

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

	// The two flood species differ in exactly one byte — the to address — and
	// that byte is the whole point of having both: a to=00 broadcast is dropped
	// by a controller's accumulator, while a frame addressed to the controller
	// reaches its engine. Both carry the transceiver ID, which is the one
	// answer this fake can emit without claiming anything about a memory
	// record.
	if d := cfg.broadcasts; d > 0 {
		r.wg.Add(1)
		go r.emitEvery(d, buildFrame(broadcastAddress, radioAddress, cmdTransceiverID, subTransceiverID, radioAddress))
	}
	if d := cfg.flood; d > 0 {
		r.wg.Add(1)
		go r.emitEvery(d, buildFrame(controllerAddress, radioAddress, cmdTransceiverID, subTransceiverID, radioAddress))
	}

	return r
}

// Port returns the reader/writer the driver opens.
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
// was said. Every received frame is recorded, including ones addressed to
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
	// to=00 broadcasts, which are addressed to everyone and answered by no one.
	if f.to != radioAddress {
		return
	}
	r.answer(f)
}

// answer dispatches one frame addressed to this radio.
func (r *Radio) answer(f frame) {
	switch {
	case len(f.data) == 2 && f.data[0] == cmdTransceiverID && f.data[1] == subTransceiverID:
		// The ID this radio reports is its own CI-V address, which is the one
		// identity the wire facts give it.
		r.send(buildFrame(f.from, radioAddress, cmdTransceiverID, subTransceiverID, radioAddress))

	case len(f.data) >= 2 && f.data[0] == cmdMemoryContent && f.data[1] == subMemoryContent:
		r.answerMemory(f, f.data[2:])

	default:
		r.send(ngFrame(f.from))
	}
}

// answerMemory handles a 1A 00 frame. The data block after the command and
// sub-command byte opens with the three bytes that name a channel; what follows
// them decides which of the three memory forms this is:
//
//   - nothing at all: a read;
//   - exactly one FF: the printed clearing form, which this fake refuses;
//   - anything else: a write of that record.
func (r *Radio) answerMemory(f frame, payload []byte) {
	if len(payload) < channelAddressLen {
		r.send(ngFrame(f.from))
		return
	}
	addr, rest := payload[:channelAddressLen], payload[channelAddressLen:]

	if isClearForm(payload) {
		// Refused. Clearing a channel is a destructive form that nothing this
		// fake exists to test needs to succeed, and a fake that answered OK to
		// it would let a driver bug that reaches for it pass unremarked.
		r.send(ngFrame(f.from))
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(rest) == 0 {
		record, occupied := r.image.read(addr)
		if !occupied {
			r.send(ngFrame(f.from))
			return
		}
		if n := r.image.servedLength(); n > 0 {
			record = fitRecord(record, n)
		}

		answerAddr := addr
		if r.cfg.answerAddress != nil {
			answerAddr = r.cfg.answerAddress
		}

		data := make([]byte, 0, 2+channelAddressLen+len(record))
		data = append(data, cmdMemoryContent, subMemoryContent)
		data = append(data, answerAddr...)
		data = append(data, record...)
		r.send(buildFrame(f.from, radioAddress, data...))
		return
	}

	if n := r.image.servedLength(); n > 0 && len(rest) != n {
		r.send(ngFrame(f.from))
		return
	}
	r.image.write(addr, rest)
	r.send(okFrame(f.from))
}
