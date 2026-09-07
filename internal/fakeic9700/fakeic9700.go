// SPDX-License-Identifier: GPL-3.0-or-later

package fakeic9700

import (
	"io"
	"sync"
	"time"

	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// out is the radio's output queue, drained by one writer goroutine. Every
// byte the radio sends goes through it: answers and unsolicited frames alike.
// One writer means two frames can never interleave mid-frame; a queue means a
// flood can outrun a slow drain, which is the condition WithBroadcasts and
// WithAddressedFlood exist to create — net.Pipe itself is a rendezvous, so a
// direct write would wedge the emitter the moment the consumer stopped
// reading. It is deep rather than unbounded; send drops when it is full.
const outQueueDepth = 1024

// Radio is a fake IC-9700 on the far end of a pipe. It answers CI-V frames the
// way the printed wire facts say the transceiver does, out of a memory image
// that holds only what a test seeded into it.
type Radio struct {
	cfg config

	// pipe is the in-memory duplex connection and the goroutines servicing it:
	// internal/fakepipe, the one package the fakes share, and protocol-free by
	// construction (see doc.go). port wraps its host end so that a driver
	// closing its port closes the whole radio.
	pipe *fakepipe.Pipe
	port *port

	// out is the output queue; see outQueueDepth.
	out chan []byte

	// mu guards the image and the transcript. The image is reachable from the
	// serve goroutine only, but the transcript is read by whoever calls
	// Transcript, so both go under one lock rather than two.
	mu         sync.Mutex
	image      *image
	transcript [][]byte
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
		cfg:   cfg,
		pipe:  fakepipe.New(),
		out:   make(chan []byte, outQueueDepth),
		image: img,
	}
	r.port = &port{radio: r}

	r.serve()
	r.pipe.Go(r.writer)

	// The two flood species differ in exactly one byte — the to address — and
	// that byte is the whole point of having both: a to=00 broadcast is dropped
	// by a controller's accumulator, while a frame addressed to the controller
	// reaches its engine. Both carry the transceiver ID, which is the one
	// answer this fake can emit without claiming anything about a memory
	// record.
	if d := cfg.broadcasts; d > 0 {
		r.pipe.Go(func() {
			r.emitEvery(d, buildFrame(broadcastAddress, radioAddress, cmdTransceiverID, subTransceiverID, radioAddress))
		})
	}
	if d := cfg.flood; d > 0 {
		r.pipe.Go(func() {
			r.emitEvery(d, buildFrame(controllerAddress, radioAddress, cmdTransceiverID, subTransceiverID, radioAddress))
		})
	}

	return r
}

// Port returns the reader/writer the driver opens.
func (r *Radio) Port() io.ReadWriteCloser { return r.port }

// Close stops the radio and wakes anything blocked on its port. It is safe to
// call more than once, and safe to call from the port's own Close.
func (r *Radio) Close() error {
	return r.pipe.Close()
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

// serve starts the goroutine that reads the controller's bytes until the pipe
// closes, handing every whole frame to handle.
func (r *Radio) serve() {
	acc := newAccumulator()
	r.pipe.Go(func() {
		r.pipe.ReadLoop(func(b []byte) {
			for _, body := range acc.feed(b) {
				r.handle(body)
			}
		})
	})
}

// writer is the one goroutine that ever writes the radio's end of the pipe.
// A write that fails means the consumer has gone away, which ends it.
func (r *Radio) writer() {
	for {
		select {
		case b := <-r.out:
			if !r.pipe.WriteNow(b) {
				return
			}
		case <-r.pipe.Done():
			return
		}
	}
}

// emitEvery sends the same frame every d until the radio closes.
func (r *Radio) emitEvery(d time.Duration, f []byte) {
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-r.pipe.Done():
			return
		case <-t.C:
			r.send(f)
		}
	}
}

// send queues one whole frame for the writer goroutine. It never touches the
// wire itself: a radio talking to a hung-up line is not an error the radio can
// do anything about.
//
// IT NEVER BLOCKS, and that is the point. send runs on the goroutine that also
// READS the port, so blocking on a full queue would stop the fake reading, and
// the consumer's next write would then block on net.Pipe's rendezvous against
// a fake that had stopped listening — both ends waiting for each other. A full
// queue drops the frame instead, which is what a real line does with bytes
// nobody is draining.
func (r *Radio) send(f []byte) {
	select {
	case r.out <- f:
	case <-r.pipe.Done():
	default:
	}
}

// handle records one received frame and answers it.
//
// NOTHING IS EVER ECHOED. A line that echoes was a knob here until 06/09/2026;
// no driver was ever built against one, and modelling a line condition nothing
// meets is a second radio to keep true.
func (r *Radio) handle(body []byte) {
	received := canonicalFrame(body)

	r.mu.Lock()
	r.transcript = append(r.transcript, received)
	r.mu.Unlock()

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
