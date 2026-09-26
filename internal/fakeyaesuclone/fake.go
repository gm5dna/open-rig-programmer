// SPDX-License-Identifier: GPL-3.0-or-later

package fakeyaesuclone

import (
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/gm5dna/open-rig-programmer/core/clonewire"
	"github.com/gm5dna/open-rig-programmer/internal/fakepipe"
)

// ackByte mirrors core/clonewire's own ackByte constant (0x06) — restated
// here, not imported, since it is unexported in that package and this is a
// one-byte protocol fact CHIRP documents directly (yaesu_clone.py sends
// "\x06" after every accepted block).
const ackByte = 0x06

// ackWaitTimeout bounds how long Radio waits for the PC's ACK after writing
// an Ack-expecting block, before giving up on the transfer. Generous for a
// same-process in-memory pipe test, where the ACK round trip is
// microseconds; not a wire-timing claim.
const ackWaitTimeout = 5 * time.Second

// Radio plays the RADIO role of one clone-mode transfer (see doc.go): it
// drives clonewire.Profile's BlockSchedule over an in-memory duplex pipe,
// waiting for exactly one ACK byte per Ack-expecting block. Port() is the PC
// (core/clonewire) end; construct with New and read Err()/Wait() once the
// transfer has run to completion or failure.
type Radio struct {
	pipe    *fakepipe.Pipe
	profile clonewire.Profile

	done    chan struct{}
	err     atomic.Value // error
	ackHits atomic.Int32
}

// New builds profile's fabricated image (BuildImage) and starts serving it
// over a fresh in-memory pipe. The transfer runs on its own goroutine;
// nothing is sent until a reader starts pulling from Port().
func New(profile clonewire.Profile) *Radio {
	r := &Radio{
		pipe:    fakepipe.New(),
		profile: profile,
		done:    make(chan struct{}),
	}
	r.pipe.Go(r.serve)
	return r
}

// Port returns the PC-side end of the fake radio's connection — what
// clonewire.Arm/Receive are given as their transport.Port.
func (r *Radio) Port() io.ReadWriteCloser { return r.pipe.Host() }

// Wait is closed once the transfer has run to completion or failure.
func (r *Radio) Wait() <-chan struct{} { return r.done }

// Err reports the transfer's outcome; nil means every block was sent and
// every expected ACK arrived, byte-for-byte. Valid only after Wait closes.
func (r *Radio) Err() error {
	if v := r.err.Load(); v != nil {
		return v.(error)
	}
	return nil
}

// AckCount reports how many ACKs this Radio has received so far — the
// end-to-end tests assert this equals the schedule's Ack-expecting block
// count once the transfer succeeds.
func (r *Radio) AckCount() int { return int(r.ackHits.Load()) }

// Close tears the pipe down without waiting for a clean transfer finish —
// used by tests exercising a Receive-side failure that leaves Radio
// mid-schedule.
func (r *Radio) Close() error { return r.pipe.Close() }

// serve writes profile's fabricated image out over the schedule named by
// profile.BlockSchedule, waiting for one ACK per Ack-expecting block. It
// records its outcome in r.err and closes r.done exactly once.
func (r *Radio) serve() {
	defer close(r.done)

	image := BuildImage(r.profile)
	conn := r.pipe.Conn()

	cursor := 0
	for i, block := range r.profile.BlockSchedule {
		payloadLen := block.Len - block.HeaderBytes - block.TrailerBytes
		if payloadLen < 0 || cursor+payloadLen > len(image) {
			r.fail(fmt.Errorf("fakeyaesuclone: block %d wants %d payload byte(s), image has %d left", i, payloadLen, len(image)-cursor))
			return
		}
		payload := image[cursor : cursor+payloadLen]
		cursor += payloadLen

		chunk := buildWireChunk(i, payload, block)
		if !r.pipe.WriteNow(chunk) {
			r.fail(fmt.Errorf("fakeyaesuclone: writing block %d: peer gone", i))
			return
		}

		if !(r.profile.AckExpected && block.Ack) {
			continue
		}
		if err := r.awaitAck(conn); err != nil {
			r.fail(fmt.Errorf("fakeyaesuclone: block %d: %w", i, err))
			return
		}
		r.ackHits.Add(1)
	}
}

// awaitAck reads exactly one byte from conn, expecting ackByte, within
// ackWaitTimeout. Any other byte, any read error (including the timeout)
// and a Radio failure spec.md Decisions item 5 requires be caught, since
// core/clonewire's Receive is the only outbound-byte source this family's
// wire has at all.
func (r *Radio) awaitAck(conn net.Conn) error {
	if err := conn.SetReadDeadline(time.Now().Add(ackWaitTimeout)); err != nil {
		return fmt.Errorf("setting ACK read deadline: %w", err)
	}
	var b [1]byte
	n, err := conn.Read(b[:])
	if n == 0 || err != nil {
		return fmt.Errorf("no ACK received: %w", err)
	}
	if b[0] != ackByte {
		return fmt.Errorf("expected ACK 0x%02X, got 0x%02X", ackByte, b[0])
	}
	return nil
}

func (r *Radio) fail(err error) {
	r.err.Store(err)
	// A failed transfer stops driving the wire; closing the radio's own
	// end lets a blocked Receive see io.EOF rather than hang forever.
	_ = r.pipe.Shutdown()
}

// buildWireChunk frames one block independently of core/clonewire's own
// (unexported) framing code — see doc.go's quarantine note. header bytes
// are a plain incrementing block-number byte (its value is never checked by
// clonewire's Receive, only its presence/width); the trailer, when
// block.TrailerBytes == 1, is an 8-bit sum of payload bytes mod 256 —
// informed-by CHIRP's own YaesuChecksum, computed here from scratch.
func buildWireChunk(blockIndex int, payload []byte, block clonewire.Block) []byte {
	chunk := make([]byte, 0, block.Len)
	for i := 0; i < block.HeaderBytes; i++ {
		chunk = append(chunk, byte(blockIndex+i))
	}
	chunk = append(chunk, payload...)
	if block.TrailerBytes == 1 {
		var sum byte
		for _, b := range payload {
			sum += b
		}
		chunk = append(chunk, sum)
	} else {
		for i := 0; i < block.TrailerBytes; i++ {
			chunk = append(chunk, 0)
		}
	}
	return chunk
}
